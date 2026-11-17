# 惯性滚动设计（对齐 Rust v0.2.0）

状态：评审后修订（reviewer 有条件通过，必修项已并入）
作者：pi orchestrator
范围：`internal/view`（滚动状态与输入）+ `internal/shell`（设置注入），不改 mygo

## 修订记录

评审指出 4 个必修问题，本版已改：

1. 符号方向与乘子层级（原来会乘两次、可能取错负号）
2. 首帧 dt 未初始化（`last` 零值会被 cap 成 0.1s，首帧多跳 6 倍）
3. 丢了现有的 Shift 换轴与 `DX != 0` 放行保护（会吞掉代码块/宽表格的横向滚动）
4. 不该用 `MaxY` 自己 clamp（build 期 `MaxY` 是上一帧值、首帧为 0，会误杀）
5. 事实修正：`inertia_scroll` 已经接线（`shell.go` 的 `NoInertia`），开关也是可见的，不需要新增重复字段
6. 补齐中断规则（外部改 Y、List 命令、跳转、键盘、resize、切 tab、重载、pointer down、Precise/Momentum 到达都要清速度）

## 1. 问题

Rust 版在滚轮 flick 后继续滑行并按物理曲线减速。Go 版在 Windows 上现在的"惯性"只是 mygo 的 notch 级 150ms EaseOut 缓出（`ui/wheel.go:10`）：不累速、不滑行、松手即停。触控板的平台惯性只有 macOS 有（darwin 后端转发 `momentumPhase`），Windows 侧只靠 `delta%120` 判断精确设备，不产生 Momentum 事件。

`inertia_scroll` 当前已经接线为 `NoInertia`；瞬移分支的现有条件是 `NoInertia || speed != 1`（`view.go:297`），即关闭惯性、或速度不是 1 时都走"直接加位移、无动画"。`scroll_speed` 已经在 Handler 里做 dy 乘子。

本设计要**明确一个行为变更**：新分支只按 `NoInertia` 决定是否滑行，因此 `speed != 1` 时从"瞬移"变成"带乘子的滑行"。这是有意的（Rust 在 speed≠1 时同样走惯性路径），§8 记录。

## 2. Rust 参考实现（照抄的权威）

`src/text/state.rs:948-1051` `InertiaScrollState`：

| 常量 | 值 | 语义 |
| --- | --- | --- |
| `MIN_VELOCITY` | 10 | 低于此速度停止 |
| `FRICTION` | 0.06 | 每 1/60s 速度乘以 0.94 |
| `MAX_VELOCITY` | 8000 | 速度上限（clamp） |
| `VELOCITY_MULTIPLIER` | 8.0 | 滚轮 delta 到速度的换算 |
| `SMOOTH_THRESHOLD` | 8000 | 高于此值走指数衰减，否则线性 |
| `LINEAR_DECEL` | 8000 | 线性减速（单位/秒） |

`add_impulse(delta)`（`:977-989`）：同向累加 `v += delta * VELOCITY_MULTIPLIER * scroll_speed`；反向 `v = v*0.3 + delta*MULTIPLIER*scroll_speed`；clamp 到 ±MAX_VELOCITY。乘子在**这里乘一次**，调用处不再乘。

`update(dt)`（`:993-1043`）：
- `dt` 上限 0.1s
- `distance = v * dt`
- `|v| > SMOOTH_THRESHOLD * scroll_speed` 时指数衰减 `v *= 0.94^(dt*60)`，否则线性 `v -= sign(v) * LINEAR_DECEL * dt`
- `|v| < MIN_VELOCITY` 时归零停止

接线（`src/text/state.rs:660-700`）：滚轮事件调 `add_impulse`；每帧 `on_prepaint` 执行 `inertia.update` + `list_state.scroll_by(-distance)` + `request_animation_frame`。

**符号约定（关键，评审指出必须写清）**：Rust 的正 delta 表示"向上"，所以滚动时用 `scroll_by(-distance)`。mygo 的正 `DY` 表示"视图下移"（内容向上走），即与 Rust 的正 delta 同向。因此 Go 侧：
- impulse 用 `+ev.DY`（不再取负）
- 每帧位移用 `Scroll.Y + distance`（不再取负）

两处都按这个方向写，不自洽地把符号翻两次。

## 3. Go 侧设计

### 3.1 新增状态（`internal/view/view.go`）

```go
// InertiaScroll is the wheel's velocity, in DIPs per second, decaying as
// the Rust viewer's does: a flick keeps the page moving after the wheel
// stops, which mygo's per-notch ease does not.
type InertiaScroll struct {
    Velocity float32
    last     time.Time // zero until the first step
    wroteY   float32   // the Y the inertia last wrote, to spot foreign moves
}
```

`State` 增加字段 `Inertia InertiaScroll`。**不新增 `InertiaScroll` 开关字段**：复用现有的 `NoInertia`（`shell.go` 已注入）。

常量（包级，注释引用 Rust 常量名）：

```go
const (
    inertiaMinVelocity     float32 = 10
    inertiaFriction        float64 = 0.94 // 1 - FRICTION, per 1/60s
    inertiaMaxVelocity     float32 = 8000
    inertiaVelocityScale   float32 = 8
    inertiaSmoothThreshold float32 = 8000
    inertiaLinearDecel     float32 = 8000
    inertiaMaxStep                 = 100 * time.Millisecond
    inertiaFirstStep               = time.Second / 60
)
```

### 3.2 输入：Handler 里截获滚轮（保留现有保护）

`internal/view/view.go:264-315` 的 `Handler` 增加 InputScroll 分支。**必须先保留现有两条保护**（评审红项）：

```go
case ui.InputScroll:
    // A wheel that moves nothing keeps its route: the strip handlers
    // and the list see a zero notch as they always did.
    if ev.DY == 0 {
        return false
    }
    // The body's own shifts and sideways wheels keep their routes: the
    // code block and wide table scroll sideways on DX, and Shift turns
    // the wheel sideways.
    if ev.DX != 0 || ev.Mods&ui.Shift != 0 {
        return false
    }
    if ev.Precise || ev.Momentum || st.NoInertia {
        return false // touchpads follow the fingers; off means instant moves
    }
    st.Inertia.impulse(ev.DY, st.ScrollSpeed)
    return true
```

要点：
- `NoInertia` 时不拦截，保持现状（瞬移，非 mygo 缓出）
- Precise 与 Momentum 放行（macOS 的 Momentum 交给 mygo）
- 消费事件后 mygo 的 notch glide 与 `snapNotch` 不再生效。本应用无 snap 容器，notch glide 由衰减曲线替代

### 3.3 每帧推进（clamp 与判停在一处，不拆两段代码）

滚动容器（`ScrollView`，`internal/view/view.go:580` 附近）在惯性激活时：

```go
if st.Inertia.active() {
    // A foreign move wins: anything else that scrolled the view since
    // our last write (List commands, jumps, keys, a thumb drag, a
    // reload) ends the glide before it fights it.
    if st.Scroll.Y != st.Inertia.wroteY {
        st.Inertia.stop()
    } else {
        now := c.Now()
        dt := now.Sub(st.Inertia.last)
        if st.Inertia.last.IsZero() || dt <= 0 {
            dt = inertiaFirstStep // a first step is one frame, not the zero-time gap
        }
        if dt > inertiaMaxStep {
            dt = inertiaMaxStep
        }
        st.Inertia.last = now
        if distance := st.Inertia.step(dt, st.ScrollSpeed); distance != 0 {
            next := st.Scroll.Y + distance
            if st.Scroll.MaxY > 0 {
                next = clamp(next, 0, st.Scroll.MaxY)
            }
            if next == st.Scroll.Y {
                st.Inertia.stop() // the edge holds us: nothing more lands
            } else {
                st.Scroll.Y, st.Inertia.wroteY = next, next
            }
        }
    }
    c.AnimationFrame()
}
```

### 3.4 边界：clamp 到 `MaxY`，用"是否被夹"判停（代码见 §3.3）

评审指出的坑：不能"完全不 clamp"。`ui/layout.go:146-158` 只把 `MaxY` 写回 `ScrollState`，`adoptScroll`（`ui/scroll.go:172-194`）是单向采纳 app 写的 `Y`，**引擎不会回写 `Y`**。所以不 clamp 时 `Scroll.Y` 会无限累积：视觉上被 `place` 夹住看着对，但用户反向滚要先"还债"，判停也永远失效。

- `MaxY > 0` 守卫回避"首帧 `MaxY` 为 0 时误杀"（首帧只可能不 clamp 地落一步，不会把 Y 压成 0，也不会误停）
- "被夹住"作为边界信号，比"回读 Y 前后比对"可靠
- 不回弹（与 Rust 一致）

### 3.5 中断规则（差分式，清单只作补充）

评审指出：清单式 hook 会漏 - `List.ScrollTo`/`ScrollToEnd`/`ScrollIntoView`、`ScrollTargetBlock`（`searchbar.go:137`）、shell 的跳转按钮**都不写 `Scroll.Y`**，要在每个入口调 `stopInertia` 就得跨包导出，必漏。

改为**差分式**（与 §3.3 复用同一字段，比对放在 `step` 之前）：

```go
// InertiaScroll 增加
wroteY float32 // the Y the loop last wrote, to spot foreign moves
```

每帧推进前先比对（就是 §3.3 代码块的第一条 `if`）：

```go
if st.Scroll.Y != st.Inertia.wroteY {
    st.Inertia.stop() // something else moved the view: yield to it
}
```

这样任何第三方改动（List 命令、跳转、键盘、拖动 thumb、resize、切 tab、重载）都自动中断，无需在每个入口埋 hook。

### 3.5.1 起停时的字段复位（评审第三轮指出的必改点）

差分判定的前提是 `wroteY` 与当前 `Y` 在静止时一致，否则**第一次 flick 就会被误判为"外来改动"直接停机**：

- `impulse()` 从静止起速时（`Velocity == 0` 且 `!active()`）：`wroteY = st.Scroll.Y`、`last = time.Time{}`
  - 只设 `wroteY` 不同步 `last` 会让新 flick 的首帧 dt 取到上一轮残留的旧时间戳，被 cap 成 0.1s，正是修订记录第 2 条要消灭的 6 倍跳
- `stop()`：`Velocity = 0`、`last = time.Time{}`（`wroteY` 留着也无害，下次 `impulse` 会重新同步）

对应测试 `TestInertiaFirstFlickFromNonZeroOffset`：文档滚到 Y=2400 后第一次 flick 不被误停，且首帧步长等于 1/60s。

清单式 hook 只在**不写 Y 的入口**补充，这里只有一类：收到 `Precise` 或 `Momentum` 事件时清速度（§3.2 的放行分支里加一行 `st.Inertia.stop()`），因为放行后平台自己接手，继续滑行会打架。

### 3.6 帧驱动与设置注入

- 消费滚轮的那一帧也必须请求下一帧：调用 `c.AnimationFrame()`（mygo 在事件被 handler 消费后不再标记 `animating`，否则滑行的起始帧可能不重绘）。实现上把 `AnimationFrame()` 放在"有速度就拉帧"的统一位置（`ScrollView` 的惯性块），不要分成两处。
- `scroll_speed`：只在 `impulse` 内乘一次。
- `inertia_scroll`：复用现有 `NoInertia` 注入路径，不新增开关字段。

## 4. 测试

新增 `internal/view/inertia_test.go`：

1. `TestWheelImpulseAccumulates`：同向两次滚轮 → `v = (d1+d2)*8*speed`；反向 → `v*0.3 + impulse`；超上限 clamp 到 8000
2. `TestInertiaFirstStepIsOneFrame`：`last` 零值时第一步等于 1/60 秒的位移（回归评审的首帧 bug）
3. `TestInertiaDecaysExponentially`：**speed=0.5** 让指数分支可达（`MAX_VELOCITY=8000` 且阈值 `8000*speed`，speed>=1 时指数分支不可达，评审指出），按 16ms 步进断言速度等于 `0.94^(dt*60)` 的解析值（容差 1%）
4. `TestInertiaStopsBelowMinVelocity`
5. `TestInertiaStopsAtBounds`：滚到底后速度归零（用"被夹住"判停，断言 `Scroll.Y == MaxY` 且速度 0）
6. `TestInertiaDisabledFallsThrough`：`NoInertia=true` 时 Handler 返回 false 且走现状瞬移（`ScrollState.Y` 由现有路径写）
7. `TestPreciseScrollBypassesInertia`：`ev.Precise=true` 与 `ev.Momentum=true` 都返回 false、速度不变
8. `TestShiftWheelAndSidewaysPassThrough`：`Mods&Shift` 与 `DX != 0` 都返回 false（横向滚动不被吞）
9. `TestInertiaStopsOnExternalScroll`：`List.ScrollToEnd()` 后速度归零
10. `TestInertiaFirstFrameKeepsMaxYZero`：首帧 `MaxY == 0` 时**不压缩 Y、也不误停速**（`MaxY > 0` 守卫），下一步照常推进
11. `TestInertiaStopsOnForeignKeyMove`：脚踏 `st.Scroll.Y`（模拟 `List.ScrollToEnd` / 跳转按钮）后，下一帧速度归零（`wroteY` 差分式中断）
12. `TestInertiaStopOnPreciseArrival`：滑行中收到 `Precise` 事件 → 速度归零并放行
13. `TestInertiaFirstFlickFromNonZeroOffset`：文档停在 Y=2400（`wroteY` 残留旧值 0）时第一次 flick **不被误停**，且首帧步长等于 1/60s（覆盖评审第三轮的字段复位必改点）

headless 驱动：`tt.Wheel(x, y, 0, dy)`（`ui/headless.go`）+ `tt.Advance(d)`（只在请求帧时推进，所以每帧必须 `c.AnimationFrame()`）。

回归：`scrollview_interaction_test.go`（滚动乘子与瞬移语义）、进度条/跳转按钮（依赖 `Scroll.Y`）、超大文档性能。

## 5. 风险与取舍

| 风险 | 评估 |
| --- | --- |
| 截获滚轮后 mygo notch glide/snap 失效 | 本应用无 snap 容器；notch glide 由衰减曲线替代 |
| 每帧写 `Scroll.Y` 与 List 虚拟化的请求竞争 | 中断规则（§3.5）保证外部请求优先；`TrackScroll` 语义本来就是"app 写即接管" |
| 高 DPI / 大 dt 跳变 | `dt` 上限 0.1s，首帧固定 1/60s，速度上限 8000 |
| 触控板手感 | Precise 与 Momentum 一律放行 |
| 与乘子叠加 | 乘子只在 `impulse` 内乘一次 |
| Windows 无 Momentum | 正是本改动要解决的 |

## 6. 不做

- 不改 mygo（上游无先例，`docs/ui/scroll.md` 明确 notch 语义）
- 不做边界回弹（Rust 也没有）
- 不改 List 虚拟化

## 7. 工作量

约 3-4 小时：状态与算法 1h、Handler/帧驱动/中断规则 1.5h、测试 1h、真机调参 0.5h。

## 8. 已定的取舍（评审建议已采纳）

1. 接受"截获滚轮后失去 mygo notch glide"（应用无 snap 容器）
2. `inertia_scroll=false` 保持现状的瞬移语义；**`scroll_speed != 1` 从"瞬移"变为"带乘子的滑行"**，这是有意的行为变更（与 Rust 一致），CHANGELOG 记录
3. 常量逐字照抄 Rust，包括 `VELOCITY_MULTIPLIER=8`；注释记录来源，真机调参再微调
4. `scroll_speed` 与衰减阈值照抄 Rust（`|v| > 8000*speed`），并在测试注释里写明 speed>=1 时指数分支不可达
5. `ReduceMotion`：不接管滚轮（走 mygo 瞬移），与 mygo 自身对该偏好的尊重一致
