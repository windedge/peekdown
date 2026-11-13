# 窗口顶部：隐藏系统标题栏，工具栏与标题栏合并

**Status**: proposed

依据 mygo 快照 `v0.7.5-0.20261105040656-977ea57e0baf`（`C:/Users/xujl/go/pkg/mod/github.com/egoist/mygo@v0.7.5-0.20261105040656-977ea57e0baf`），参考实现 NextTodo（`C:/src/go/nexttodo`，mygo `v0.6.4`）。

目标：去掉标准标题栏与顶部原生菜单栏，让自绘 bar 同时承担标题栏、工具栏、窗口拖动区，窗口控制按钮仍交给系统绘制（与 NextTodo 一致）。

## 参考实现：NextTodo 怎么做的

- 窗口：`main.go:110-126`，`mygo.NewWindow(mygo.WindowOptions{... TitleBarStyle: mygo.TitleBarHiddenInset, TitleBarHeight: 50, TrafficLightPosition: &mygo.Point{X: 18, Y: 18}})`。**不是** `Frameless`，**不设** `HideWindowButtons`：最小化/最大化/关闭由 mygo 画（Windows 上为 DirectComposition 画的 Win11 风格 caption buttons，macOS 上是红绿灯，Linux 上是 GTK 的 title buttons），应用只负责让位。
- bar：`app.go:471-525`，`bar := c.TitleBar()` 后
  `ui.Row(c).Height(max(bar.Height, 50)).Shrink(0).Padding(0, 12+bar.Right, 0, max(bar.Left+8, 16)).AlignItems(ui.Center).Gap(4).DragWindow()`。
  左：logo `Icon(17x17)` + 应用名（13px/600）；中：`pageSwitch` 用 `Attach(ui.AnchorCenter, ui.AnchorCenter)` 绝对居中；右：`iconBtn`（`style.go:282-289`，26x26、Radius 7、hover 才出背景）+ 一个 `ui.PopoverBase` 的 "+" 弹出菜单。
- pill 切换器：`style.go:341-388` 自绘 segmented control。先用 `c.MeasureText` 按 12.5px/500 量每段宽（+22 内边距），track `Size(x+3, 28)`；白色 thumb 为 `Box.Absolute().Top(2).Left(2+thumbX).Size(widths[chosen], 24).Radius(6).Shadow(...).Transition(thumbMotion)`（200ms + scale 0.92）；段按钮是 `Absolute().Top(2).Left(2)` 的 Row，段高 24、Radius 6、选中文本 500 权重。
- 菜单：**保留** `mygo.App.SetMenu(a.menu())`（`main.go:57`、`main.go:83`；菜单定义 `main.go:327-370`，含 `RoleAppMenu/RoleFileMenu/RoleEditMenu/RoleWindowMenu`）。截图里看不到菜单栏是因为 mygo 对无标题栏窗口做了 barless 处理：`internal/windows/menu.go:235-239`（`barless()` = captionless/fullScreen/noRedirect）让菜单栏不挂到窗口上，`internal/windows/menu.go:264+`（`popupMenuBar`）改成 Alt/F10 时在自绘标题栏下方弹出。加速键照旧生效。
- 平台分支：只有 `main.go:90` 一处 `runtime.GOOS != "darwin"`，管的是"关窗即退出"；窗口按钮没有 GOOS 分支，全靠 mygo（`TitleBarHiddenInset` 在 Windows 上等同 `hidden`，`TrafficLightPosition` 只在 macOS 生效）。

## mygo 提供的 API（v0.7.5 快照）

| API | 要点 | 平台 |
| --- | --- | --- |
| `WindowOptions.TitleBarStyle` | `TitleBarDefault` / `TitleBarHidden` / `TitleBarHiddenInset`；保留系统控制按钮，页面铺满窗口 | 全平台 |
| `WindowOptions.TitleBarHeight` | 自绘标题栏高度；控制按钮在 Windows 上填满它，Linux 上居中；0 = Windows 32、Linux 桌面 header bar 高 | Win/Linux |
| `WindowOptions.TrafficLightPosition` | 红绿灯位置（close 按钮左上角距窗口左上角） | macOS |
| `WindowOptions.HideWindowButtons` | 连系统按钮也不要；保留阴影、圆角、resize 边（含 Windows 顶边），`c.TitleBar()` 变 0 | 全平台 |
| `ui.Context.TitleBar() ui.TitleBar` | `struct{Height, Left, Right float32}`，控制按钮占的宽度/高度；全屏为 0 | 全平台 |
| `ui.Element.DragWindow()` | 拖动窗口；`clicks==2` 时调 `TitleBarDoubleClicked`（Windows max/unmax，macOS zoom）。命中只看最深的 interactive 元素（`ui/input.go:309,330-381`），所以 Box/Spacer/Text/Icon 会落到父级 drag 元素，Button/输入框/菜单按钮仍可点 | 全平台 |
| `ui.Element.MaximizeButton()` | 只用于 `HideWindowButtons` 自绘场景，让 Win11 snap layouts 跟到自绘按钮上 | Windows |
| `ui.Element.Menu(build func(*ui.Menu))` | menu button：按下即开原生弹出菜单，Enter/Space/Down 也可开；`ui.PopoverMenuButton` 是窗口内自绘版 | 全平台 |
| `mygo.Window.Minimize/Maximize/Unmaximize/ToggleMaximize/IsMaximized/Close` | 自绘控制按钮时的动作 | 全平台 |
| `mygo.Window.SetWindowButtonsTheme(ThemeDark/ThemeLight/ThemeSystem)` | 让 Windows caption buttons 跟随深色顶栏 | Windows |
| `ui.Tester.SetTitleBar(ui.TitleBar{...})` | headless 测试注入控制按钮占位（`ui/headless.go:397`）；fake 平台默认 `Right: 138`（`internal/fake/fake.go:236`） | 测试 |

## Peekdown 改造规格

### 1. WindowOptions（`main.go:198-207`）

```go
w := mygo.NewWindow(mygo.WindowOptions{
    Title:          a.title(),
    Width:          1024,
    Height:         768,
    MinWidth:       480,
    MinHeight:      320,
    StateKey:       "main",
    TitleBarStyle:  mygo.TitleBarHidden,
    TitleBarHeight: shell.TitleBarHeight, // 44，见下文
    Content:        ui.View(a.view),
})
```

- 不设 `Frameless`：Windows 上 `Frameless` 会丢顶边 resize 和阴影，`TitleBarHidden` 两者都保留（`window.go:119-126`、`internal/windows/window.go:225-240`、`newTopEdge`）。
- 不设 `HideWindowButtons`：省掉自绘 min/max/close 与 macOS 红绿灯适配，正是 NextTodo 的做法。
- 深色主题时补 `a.win.SetWindowButtonsTheme(mygo.ThemeDark)`（浅色 `ThemeLight`），接线点在 `main.go` 现有的主题设置路径（`shell.go:427`、`shell.go:517` 调 `s.SetTheme` 处），或在 `a.adopt` 后按 `s.Load().Theme` 设一次、每次 `Apply` 再设。
- `SetTitle` 保留（任务栏、Mission Control、窗口菜单仍需要），但 bar 上不必重复完整路径。

### 2. 顶部 bar（改 `internal/shell/tabs.go` 的 `tabBar`，新增 `internal/shell/titlebar.go`）

先修 `shell.go:556 View` 的结构问题：现在 `if len(s.Tabs)==0 { s.welcome(c); return }` 在 `s.tabBar(c)` 之前，**欢迎页没有 bar**。改造后 bar 必须无条件绘制（否则无边框窗口既拖不动、也没有最大化入口，只剩状态栏）。

尺寸与排布：

```
Row  Height = max(c.TitleBar().Height, 44)   Shrink(0)  AlignItems(Center)  Gap(6)
     Padding(0, 8 + bar.Right, 0, 8 + bar.Left)   Background(t.Background)
     BorderWidth(0,0,1,0) BorderColor(t.Border)   DragWindow()
```

- 高度取 44 而不是沿用 tab bar 的 40：AGENTS.md 的滚动条 overlay 规则要求横向滚动容器底部 12px 内不能有文字；40 高配 34 高 tab 只剩 6px 余量，横向滚动条会盖住 tab 下沿。44 + 34 tab + 底部 1px 边框 = 9px，仍偏紧，实现时按 44 高、tab 32 高、`AlignItems(ui.Center)` 处理，并截图核对。
- 左：`Icon(logo, 16x16)` + `Text("Peekdown", 13px, 600)`，无 tab 时显示；有 tab 时品牌块收窄（只留 24px 的 logo）或整个让给 tab strip。
- 主：现有 `ui.ScrollHorizontal` tab strip，`Grow(1).MinWidth(0)`。tab 是 `ButtonBase`（clickable），在 `DragWindow` 行内仍可点击、可右键；strip 的空白处不是 interactive 元素，指针会回落到 bar 行，因此空白处可以拖窗，无需额外 spacer。
- 右：全部复用 `internal/shell/icons.go:46` 的 `iconButton`（24x24、Radius 5、hover 才出背景）：Open(`folderOpenIcon`)、Refresh、Find(`searchIcon`)、Toggle Files、Toggle Outline、Settings(`gearIcon`)，再加一个汉堡菜单按钮（见下）。所有按钮右缘停在 `bar.Right` 之前，不进入控制按钮区域。
- `c.TitleBar()` 在首帧就是真值（`internal/bridge/bridge.go:74` 首帧注入），不需要延迟重建；全屏时为 0，bar 自然贴边，不用写分支。

### 3. 菜单迁移

- **保留** `mygo.App.SetMenu(shell.BuildMenu(s))`（`main.go:208`）。理由：它是全部加速键的唯一来源（`internal/shell/menu_test.go` 逐条断言 label+accelerator），macOS 需要系统菜单栏，Windows/Linux 上 mygo 会自动隐藏菜单栏并保留 Alt/F10 弹出。
- **新增** bar 上的菜单按钮：`ui.ButtonBase(c.Key("app-menu")).Size(24,24).Menu(func(m *ui.Menu){ ... })`，内容为 File/Edit/View/Peekdown 的合并列表。为免两份定义漂移，把 `internal/shell/menu.go` 抽成平台无关描述：

  ```go
  type MenuSpec struct { Label, Accel string; Run func(*mygo.Window); Sub []MenuSpec; Sep bool }
  func MenuSpecs(s *Shell) []MenuSpec              // 唯一来源
  func BuildMenu(s *Shell) *mygo.Menu              // 渲染成原生菜单（加速键、macOS 菜单栏、Alt/F10）
  func (s *Shell) appMenu(c *ui.Context) *ui.Menu  // 渲染成 bar 菜单按钮的内容
  ```

  测试：现有 `menu_test.go` 不动（继续断言加速键）；新增一条断言"`MenuSpecs` 的顶层 label 集合 == BuildMenu 顶层 label 集合"，保证两条渲染路径同源。
- **Linux 例外**：hidden titlebar 的窗口仍保留 GTK 菜单栏（`docs/frontend.md:324`），会出现在自绘 bar 上方。第一阶段接受这个差异并记入已知问题；若要消掉，需 `w.SetMenu(mygo.NewMenu(nil))` 再用 `c.OnWindowShortcut` 逐条补回加速键，工作量和回归面都明显更大，建议单开一单。

### 4. 拖动 / 双击 / 控制按钮接线

- 拖动：bar 行 `DragWindow()`，不额外写代码。tab strip 不设 `flagTrackPointer`，空白处可拖。
- 双击最大化：mygo 内部处理（`ui/input.go:377-383` 的 `clicks == 2` 分支），**不要**自己注册双击，否则会双触发。
- 控制按钮：系统绘制，应用零代码。仅当将来要做完全自绘时才用 `HideWindowButtons: true` + `ui.ButtonBase(...).MaximizeButton()` + `a.win.Minimize/ToggleMaximize/Close`。
- macOS：`TitleBarHidden` 已保留红绿灯；若要与 NextTodo 对齐，可加 `TrafficLightPosition: &mygo.Point{X: 14, Y: 12}`，并按 `bar.Left` 让位（macOS 的 `bar.Left` 约 78）。bar 上的菜单按钮在 macOS 可省略（系统菜单栏已有），但建议保留以保持一致；不写 `runtime.GOOS` 分支也能工作。

## 风险与需真机验证

1. Windows 无边框窗口的阴影、圆角、**顶边 resize 热区**（`internal/windows/window.go:427,824` 里 Win11 特判 `r.Top--`）只能真机拖一下确认，headless 测不到。
2. `bar.Right` 随 DPI、最大化、全屏变化（全屏为 0，Win11 约 138，macOS hiddenInset 约 78）。测试必须用 `tt.SetTitleBar(ui.TitleBar{Height: 44, Right: 138})` 与 `{0,0,0}` 两种值，断言最右侧按钮右缘 `<= w - 138` 且不重叠。
3. 深色主题下 Windows caption buttons 默认跟系统，必须调 `SetWindowButtonsTheme`，否则深色 bar 上出现浅色按钮（截图才看得出）。
4. 欢迎页（无 tab）必须有 bar，否则窗口拖不动、无最大化入口。
5. 横向滚动条 overlay：bar 高度、tab 高度、底部边框三者要一起定，改完必须真机截图（AGENTS.md 明确要求）。
6. Linux：GTK 菜单栏与自绘 bar 叠两层；Wayland 合成器（KWin/Hyprland/Sway）自绘装饰时不显示 title buttons，`bar.Right` 为 0 或整条 bar 都由合成器接管。
7. 现有 headless 测试默认 `bar` 为 0，改造后新增的几何断言必须显式 `SetTitleBar`，否则永远测不到"让位"这条规则。
8. `Window.SetTitle` 在无标题栏后不再可见，但仍是任务栏/窗口菜单/无障碍的名字，不要删。

## 工作量估算

约 7-9 人时：

- WindowOptions + bar 重排 + 欢迎页路径（2h）
- 菜单同源重构（`MenuSpec` + 两条渲染路径 + 测试）（2-3h）
- bar 右侧图标按钮与菜单按钮、新图标（1-2h）
- headless 几何测试（bar 让位、两档 `TitleBar`、状态栏不被挤）（2h）
- Windows / macOS / Linux 真机截图核对（1-2h）
