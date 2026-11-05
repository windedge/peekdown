# 性能与体积:测量方法与基线(issue #13)

- 分支:`research/perf-baseline`,测量日期:2026-11-05
- 机器:Windows 11 Pro 10.0.26200,Intel Core Ultra 9 185H(22 逻辑核),31.5 GB,本地桌面会话
- 工具链:go1.27.3 windows/amd64(与 mygo、本仓库 go.mod 的 1.27.3 一致,**未触发 toolchain 下载**),GOTOOLCHAIN=auto,powershell 5.1.26100.9444
- mygo 参考:`C:/src/go/mygo`(只读,本次未改动)

## 1. 结论先行

1. mygo 的现成工具可直接用,但分工不同:`MYGO_FRAME_STATS`(逐帧日志)、`Window.OnReadyToShow`(首帧钩子)、`internal/idlemem`(稳态内存)在 Windows 上开箱可用;`scripts/bench.ts` 测的是 mygo 自己的 examples,不测我们的 app;`internal/inputlatency` 只有 macOS。
2. 没有 `Window.OnFirstFrame`,但 `Window.OnReadyToShow` 就是等价的现成钩子:Content 窗口"画完第一帧"后回调一次(`window.go:656` 起 `showLater`/`settleShow`,`ui/window.go:504` 的 `ready()`)。本次已用它实测(4.4)。
3. 外部黑盒也可行:PowerShell 轮询"该 PID 第一个可见、未被 DWM cloak、客户区非空的顶层窗口",分辨率约 1 ms;counter-native 5 次 p50 340.9 ms、p90 341.0 ms,抖动 0.1 ms 级。
4. **最大风险是"整篇一次渲染"而不是列表**:同一份 1 万行文档,`ui.List` 虚拟化滚动 55.5 fps(帧中位 1.1 ms);整篇塞进一个 `ui.Scroll` 只有 23.5 fps(中位 39 ms);5 万行整篇掉到 4.5 fps(中位 226 ms),首帧 2.0 到 3.8 s。**rewrite 必须块级虚拟化**,否则"1 万行流畅 / 5 万行不卡死"都过不了。
5. 体积基线(release 参数,单 exe、无 webview、无子进程):counter-native 10,329,600 B(9.85 MiB)、markdown-chat 11,348,480 B(10.82 MiB)、gallery 14,991,872 B(14.30 MiB)。
6. Rust 对照脚本已给,但本机**无法实测**:没有 cargo/rustc(`cargo --version` 失败,无 `~/.cargo`)。

## 2. mygo 现成测量工具

| 工具 | 测什么 | 怎么跑 | 输出 | 平台限制 / 注意 |
|---|---|---|---|---|
| `scripts/bench.ts`(Bun) | mygo 仓库自身的 Go benchmark;`examples/hello`、`examples/counter-native` 的 release 体积;`--e2e` 时两者稳态内存(idlemem) | `bun scripts/bench.ts run [--e2e] [--count 6] [--benchtime 500ms] [--out f] [pkgs...]`;`merge` 维护历史 | JSON:`sha/os/arch/cpu/go/count/results`,按 pkg 与 bench 给 `ns/op`、`B/op`、`allocs/op`,另有 `size.bytes`、`memory` | 需 bun(本机 1.3.13 可用);只测 mygo 自己的 examples;`-p 1` 串行;`--e2e` 需桌面会话 |
| `internal/idlemem` | app 空闲后的内存:app + 后代进程 | `go run ./internal/idlemem [-count 3] [-idle 6s] app...` | JSON `{"app":{"app":bytes,"total":bytes}}` 取中位 | 需桌面会话;Windows 用 `K32GetProcessMemoryInfo` 的 PrivateWorkingSetSize(任务管理器口径),后代走 Toolhelp32;原生 UI 后代为 0,webview app 含 WebView2;属 internal 包,只能在 mygo 仓库内跑 |
| `internal/inputlatency` | macOS 端到端输入延迟:queue/handle/到 present/到屏幕,每帧等 drawable、新帧比例 | `go run ./internal/inputlatency [-mode type,scroll,drag,anim,heavy] [-runs 6] [-events 300]` | 每 mode 每次的 mean/p50/p90/p99 与分项,再给多次运行中位数 | **仅 macOS**:`measure_other.go` 在非 darwin 直接返回 `measures macOS only` |
| `ui.Tester` benchmark(`docs/ui/performance.md`) | 无窗口 CPU 渲染的稳定帧:`BenchmarkValueFrame`、`BenchmarkValueList`(百万行列表可见行)、`BenchmarkDiffSteady`、`ui/markdown` 的 `BenchmarkViewSteady/Stream`;还有 retained heap | `CGO_ENABLED=0 GOMAXPROCS=1 go test ./ui -run '^$' -bench 'BenchmarkValueFrame|BenchmarkValueList|BenchmarkDiffSteady' -benchmem -count=6 -benchtime=300ms` | `ns/op`、`B/op`、`allocs/op` | 只有 CPU 侧(构建+布局+绘制),无 GPU/合成器/显示节奏;固定 fixture,不是真实文档;是帧预算下界的好代理 |
| `BenchmarkStressFrame` | 900x640@2x 每帧全变:`boxes`、`labels`、`translated_column`、**`list`(10 万行 List 每帧滚 3 行)**、`dashboard` | `GOMAXPROCS=1 go test ./ui -run '^$' -bench 'BenchmarkStressFrame/list' -benchmem -count 6` | `ns/op` 一帧成本 | GPU 版仅 Metal;`MYGO_GPU=0` 可强制 CPU 渲染器 |
| `MYGO_FRAME_STATS`(`ui/framestats.go`) | **真实 app 逐帧**:总耗时 + build/layout/paint/present、绘制路径、期间分配、是否 GC | 启动前设 `MYGO_FRAME_STATS=1`(只记 >8 ms)、`=4`、`=all`(每帧),日志走 stderr | `mygo: frame 3 took 2.1 ms: build 0.5, layout 0.0, paint 0.5, present 1.1 (drawn on the GPU: ...), other 0.0; meanwhile ...; no GC` | 只有画出来的帧才有日志(静止不画);**时间戳只到秒**,帧间隔要靠会话时长换算 |
| `Window.OnReadyToShow` | 首帧时机(Content 窗口画完第一帧) | app 内注册(见 3.1) | 自己打点 | `WhenReady` 是"app 启动完成可建窗口",不是首帧;`Hidden` 窗口也会先画首帧 |
| `plugins/automation` + `internal/automationcli` | 驱动真实 app:快照、click/wheel/type、`--settle`、截图 | app 需 `//go:build e2e` + 空白导入 plugin,且设 `MYGO_AUTOMATION`;客户端 `go install .../cmd/mygo-automation` | 文本快照/diff、退出码 | **release(`-tags mygo_noinspector`)不含 automation**,只打印忽略提示;适合 CI 精确驱动,不适合测 release 包 |
| `docs/distribution.md` + `docs/architecture.md` 的 Binary size 节 | release 参数、体积归因 | 见 3.5;`go tool nm -size -sort size`(去掉 `-s -w`)、`-ldflags=-dumpdep` | 字节数、符号大小 | `mygo build` 默认 `CGO_ENABLED=0`;Windows 加 `-H=windowsgui` |

## 3. 可重复的测量方法(Windows 优先)

原则:测量期间不开浏览器/IDE 编译,电源固定插电;每轮之间等 300 到 500 ms 让进程树退干净(webview app 要等子进程退出,否则抢用户数据目录);区分**热启动**(文件缓存命中)与**冷启动**(重启后首次或清空 standby list),两个都记并标注;每份结果都带机器配置模板(OS、CPU、核数、内存、电源、go 版本、构建 sha、日期、刷新率)。

### 3.1 冷启动到首屏

(a) **app 内打点(最准,改动最小)**:用 `GetProcessTimes` 拿本进程创建时刻,用 `OnReadyToShow` 拿首帧,一个数就覆盖"进程创建 -> 首帧",不含外壳开销:

```go
created := processCreated() // GetProcessTimes;非 Windows 返回 time.Now(),口径降级为"进入 main -> 首帧"
mygo.App.WhenReady(func() {
    w := mygo.NewWindow(mygo.WindowOptions{ /* ... */ })
    if os.Getenv("MYGO_STARTUP_TRACE") != "" {
        w.OnReadyToShow(func() {
            log.Printf("startup: first frame %s after the process was created",
                time.Since(created).Round(time.Millisecond))
        })
    }
})
```

Windows 侧 `processCreated()`:`kernel32!GetProcessTimes` + `time.Unix(0, creation.Nanoseconds())`(本次在夹具里验证可用)。开关不打点时零开销,可以留在代码里。

(b) **外部黑盒(不改 app,CI 用)**:`scripts/perf/Measure-ColdStart.ps1`。`Stopwatch` 起表 -> `Start-Process` -> 自旋轮询该 PID 的第一个可见、未 cloak、客户区非空的顶层窗口。

语义差异必须写明:mygo 的 Content 窗口在 `NewWindow` 返回后的主线程轮次里 show,首帧随后画,所以**外部"窗口可见"通常比"首帧"早几毫秒到几十毫秒**;重文档上这个差是几百毫秒到 2 s(4.4 实测)。因此"冷启动到首屏 < 1 s"以 (a) 为准,(b) 作回归哨兵。

### 3.2 1 万行文档滚动(帧时间)

1. **CPU 侧下界(CI 友好)**:`BenchmarkStressFrame/list`(10 万行每帧滚 3 行)+ `ui/markdown` 的 `BenchmarkViewSteady`。单帧 `ns/op` 远小于 16.7 ms 才值得上真机。
2. **真实 app(验收用)**:`MYGO_FRAME_STATS=all` + `scripts/perf/Invoke-ScrollTest.ps1`(指针居中,固定频率发滚轮 N 秒)-> `Measure-FrameStats.ps1` 解析。判据:中位 < 16.7 ms 且 p99 < 16.7 ms;会话内帧数/秒数 >= 50(60 Hz 上限约 60);超预算帧 < 1%,且没有 > 100 ms 的帧。
   mygo 只在有变化时画帧,滚不动的内容不出帧,所以文档必须足够长;每次先看"帧数/fps"是否接近 55 到 60,否则先怀疑驱动方式(本次 gallery 就是这样拿到无效数据)。
3. **端到端延迟(macOS 可选)**:`internal/inputlatency -mode scroll -runs 6`,是"跟手感"的唯一直接指标,Windows 没有等价工具。

需要精确滚动量(而不是按时间发 notches)时,用 `plugins/automation` 的 `wheel` 驱动 e2e 构建,再解析同一份日志。

### 3.3 打开到首屏、搜索

- 打开到首屏:与冷启动同一套打点,`OnReadyToShow` 换成"第 N 个文档首次画完",日志区分"进程首帧"和"文档首帧";外部黑盒测不到换文档。解析开销先用 `BenchmarkViewSteady` 判断,再用 `MYGO_FRAME_STATS` 看第一帧的 build/layout 分项。
- 搜索 1 万条命中:搜索是同步逻辑,不属于帧时间,先做成 benchmark:`for b.Loop() { if n := len(search(doc, q)); n < 10000 { b.Fatalf("%d hits", n) } }`,判据 `ns/op` < 16.7 ms(一次搜索不破一帧);真机再补"输入一个字符到命中高亮出现"的 framestats,确认无 > 50 ms 的帧。若必须异步,验收改成"输入到结果首次可见 < 100 ms"。

### 3.4 稳态与峰值内存

- `scripts/perf/Measure-Memory.ps1`:启动、空闲 `IdleSec`(默认 6 s)后采样 **private working set**(任务管理器"内存"口径,与 `internal/idlemem` 一致),同时给后代进程之和与 lifetime peak working set。
- 交叉校验:在 mygo 仓库 `go run ./internal/idlemem -count 3 ./app.exe`。
- **webview 差异**:原生 UI 的 mygo app 没有子进程(本次实测 descendants = 0);带 webview 会拉 WebView2 的 browser/renderer/GPU 进程,`total` 才是用户感受的量级,两项都要报。`linux.webview: false` 只影响 Linux 包依赖,Windows 上 WebView2 进程照样在。
- 峰值:打开大文档、滚到底、搜索一次之后再读,能抓到缓存与渲染资源高点。

### 3.5 二进制体积

```sh
go build -trimpath -tags mygo_noinspector \
  -ldflags "-s -w -H=windowsgui -X github.com/egoist/mygo.production=1" \
  -o out/app.exe ./cmd/app
```

报 `stat` 字节数(别用资源管理器的 MiB 四舍五入)。体积回归:去掉 `-s -w` 后 `go tool nm -size -sort size` 找变大符号;`-ldflags=-dumpdep` 找是谁把包拖进来(常见:`reflect.Type.Method` 保留全部导出方法、`net/http`、`text/template`)。发行包另外报 NSIS 安装器与多平台产物。

## 4. 已测数字(2026-11-05,本机)

### 4.1 构建与体积

参数:`go build -trimpath -tags mygo_noinspector -ldflags "-s -w -H=windowsgui -X github.com/egoist/mygo.production=1"`,go1.27.3 windows/amd64,产物在 `C:/tmp`(不入库)。

| 程序 | 来源 | 字节 | MiB |
|---|---|---:|---:|
| counter-native | `C:/src/go/mygo/examples/counter-native` | 10,329,600 | 9.85 |
| markdown-chat | `C:/src/go/mygo/examples/markdown-chat` | 11,348,480 | 10.82 |
| gallery | `C:/src/go/mygo/examples/gallery` | 14,991,872 | 14.30 |
| perfharness(1 万行文档夹具) | `C:/tmp/pk-perf/harness` | 11,350,528 | 10.82 |

构建耗时:counter-native 首次 16.0 s(含 mygo 全量编译),markdown-chat 缓存热后 2.4 s。

### 4.2 冷启动(外部黑盒,窗口可见;1 次预热丢弃 + 5 次,热文件缓存)

| app | p50 | p90 | min | max | 首帧时 peak WS |
|---|---:|---:|---:|---:|---:|
| counter-native | 340.9 ms | 341.0 | 319.7 | 341.0 | 71.3 MB |
| markdown-chat | 598.9 ms | 622.6 | 408.1 | 782.2 | 78.3 MB |
| perfharness(list 1 万行) | 448.8 ms | 494.6 | 429.8 | 655.0 | 88.4 MB |

markdown-chat 抖动大(408 到 782 ms),主要是首次字体/文本系统初始化与解析;counter-native 稳定在 341 ms。都在 1 s 内。

### 4.3 稳态内存(private working set,6 s 空闲,3 次中位)

| app | app | 含后代 | 后代数 | lifetime peak WS |
|---|---:|---:|---:|---:|
| counter-native | 32.2 MB | 32.2 MB | 0 | 71.3 MB |
| markdown-chat | 36.5 MB | 36.5 MB | 0 | 78.9 MB |

### 4.4 首帧(app 内 `OnReadyToShow` + `GetProcessTimes`,3 次)

夹具 `C:/tmp/pk-perf/harness`:用 mygo 现成 API,`-scene doc` 为整篇 `ui.Scroll` + `markdown.View`,`-scene list` 为 `ui.List` + 每行 markdown;`-lines` 控制行数。

| 场景 | 3 次读数 |
|---|---|
| list 1 万行(虚拟化) | 296 / 273 / 438 ms |
| doc 1 万行(整篇) | 1.509 / 1.048 / 1.05 s |
| doc 5 万行(整篇) | 2.032 / 2.695 / 3.759 s |

对照 4.2:perfharness list 的"窗口可见"是 448.8 ms,而"进程创建到首帧"是 273 到 438 ms,量级一致;doc 5 万行时窗口很快出现但首帧要 2 到 3.8 s。**只用外部黑盒会严重低估,这就是必须用 `OnReadyToShow` 的理由。**

### 4.5 滚动帧时间(`MYGO_FRAME_STATS=all`,滚轮 30 notches/s,4 s 会话,60 Hz 预算 16.7 ms)

| 场景 | 画出帧数 | fps | 中位 | p90 | p99 | max | 超 16.7 ms | 分项中位 build/layout/paint/present |
|---|---:|---:|---:|---:|---:|---:|---:|---|
| list 1 万行 | 222 | 55.5 | 1.10 ms | 1.60 | 2.20 | 178.8 | 0.5%(只首帧) | 0.00 / 0.50 / 0.60 / 0.00 |
| list 5 万行 | 222 | 55.5 | 1.10 ms | 1.60 | 1.90 | 139.2 | 0.5%(只首帧) | 0.00 / 0.50 / 0.50 / 0.00 |
| doc 1 万行(整篇) | 94 | 23.5 | 39.00 ms | 50.50 | 128.70 | 425.1 | 100% | 14.3 / 20.3 / 1.00 / 0.50 |
| doc 5 万行(整篇) | 18 | 4.5 | 226.00 ms | 251.60 | 260.50 | 1758.6 | 100% | 91.1 / 107.5 / 4.50 / 0.80 |
| gallery(对照,指针未必在其列表上,数据无效) | 29 | 9.7 | 1.00 ms | 34.20 | 110.00 | 1993.5 | 13.8% | 0.00 / 0.00 / 0.00 / 0.00 |

解读:

- 虚拟化列表已达 60 Hz 上限:1 万行与 5 万行几乎同价(只布局可见行),中位 1.1 ms、55.5 fps,唯一超预算的是启动帧。
- 整篇渲染是硬伤:1 万行时 layout 中位 20 ms、build 14 ms;5 万行时 build 91 ms、layout 107 ms。**文档模型必须"块列表 + 可见块渲染"**,不能沿用 `ui.Scroll` + 一次性 `markdown.View`。
- `present` 基本都 < 0.5 ms,瓶颈在 CPU 侧(构建与布局),所以用 `go test` 的 CPU benchmark 做回归有效。

## 5. 与 Rust v0.2.0 对照

本机**没有 Rust 工具链**:`cargo --version` / `rustc --version` 均失败,`where cargo` 无结果,无 `~/.cargo`。只有方法,没有数字。

在装了 Rust stable(edition 2024)的 Windows 机器上:

```powershell
cd C:/Seafile/Works/rust/peekdown
cargo build --release            # [profile.release] lto="thin", codegen-units=16, incremental=true, strip=true
(Get-Item target/release/peekdown.exe).Length
./scripts/perf/Measure-ColdStart.ps1 -Exe C:/Seafile/Works/rust/peekdown/target/release/peekdown.exe -Runs 10
./scripts/perf/Measure-Memory.ps1  -Exe C:/Seafile/Works/rust/peekdown/target/release/peekdown.exe -Runs 3 -IdleSec 6
```

注意事项:peekdown 有单实例 IPC,用同一个 `.md` 反复启动会把参数发给已有实例,测量前确保没有残留进程;GPUI 首帧之后内容才出现,外部"窗口可见"可能早于内容,需用 `RUST_LOG`/自带 span 交叉校验;当前 Cargo.toml 无 WebKit/Tauri 依赖,是纯 GPUI 单进程,内存可直接对比;1 万行滚动对比用同一份测试文档、同一份 `Measure-FrameStats` 口径,Rust 侧没有等价 env 时退化为外部轮询 + CPU 占用采样,或临时在 render 循环打点(只在测量分支)。

## 6. 验收命令(可直接照抄)

```powershell
cd <repo>
# 0) release 构建 + 体积
go build -trimpath -tags mygo_noinspector `
  -ldflags "-s -w -H=windowsgui -X github.com/egoist/mygo.production=1" `
  -o C:/tmp/perf/peekdown.exe ./cmd/peekdown
(Get-Item C:/tmp/perf/peekdown.exe).Length

# 1) 冷启动到首屏(外部):p50 与 p90 都 < 1000 ms
./scripts/perf/Measure-ColdStart.ps1 -Exe C:/tmp/perf/peekdown.exe -Runs 10

# 2) 冷启动到首帧(app 内,更准)
$env:MYGO_STARTUP_TRACE = "1"
./scripts/perf/Measure-ColdStart.ps1 -Exe C:/tmp/perf/peekdown.exe -Runs 5   # 同时看 stderr 的 startup: 行

# 3) 1 万行滚动:中位与 p99 都 < 16.7 ms,fps >= 50
./scripts/perf/Invoke-ScrollTest.ps1 -Exe C:/tmp/perf/peekdown.exe -AppArgs C:/tmp/perf/doc-10k.md -Seconds 4

# 4) 5 万行 / 5 MB:fps >= 30,且不出现 > 100 ms 的帧
./scripts/perf/Invoke-ScrollTest.ps1 -Exe C:/tmp/perf/peekdown.exe -AppArgs C:/tmp/perf/doc-50k.md -Seconds 4

# 5) 搜索:一次 < 16.7 ms
go test ./internal/search -run '^$' -bench 'Search10k' -benchmem -count 6

# 6) 稳态与峰值内存
./scripts/perf/Measure-Memory.ps1 -Exe C:/tmp/perf/peekdown.exe -AppArgs C:/tmp/perf/doc-50k.md -Runs 3 -IdleSec 6
```

CI(无 GPU/无桌面)退化为 CPU 侧:

```sh
GOMAXPROCS=1 go test ./ui -run '^$' -bench 'BenchmarkStressFrame/list' -benchmem -count 6
GOMAXPROCS=1 go test ./ui/markdown -run '^$' -bench '^BenchmarkView' -benchmem -count 6
```

## 7. 脚本清单(`scripts/perf/`,PowerShell 5.1,无外部依赖)

| 脚本 | 用途 |
|---|---|
| `PerfCommon.ps1` | 共享 Win32 封装:`PerfWin32.WaitFirstWindow`(首个可见未 cloak 窗口)、`PrivateWorkingSet`、`PeakWorkingSet`、`Descendants`、`Start-AppTree`/`Stop-AppTree`、百分位/中位、机器信息。其它脚本点源它 |
| `Measure-ColdStart.ps1` | 外部冷启动:预热丢弃 + N 次,p50/p90/min/max、首帧 peak WS,`-Json` 出结构化结果 |
| `Measure-Memory.ps1` | 稳态内存:app + 后代(private working set)+ lifetime peak,`-IdleSec` 控制空闲时长 |
| `Measure-FrameStats.ps1` | 解析 `MYGO_FRAME_STATS` 日志:帧数、p50/p90/p99/max、超预算比例、build/layout/paint/present 中位、`-DurationSeconds` 换算 fps |
| `Invoke-ScrollTest.ps1` | 起 app(`MYGO_FRAME_STATS=all`)-> 指针居中 -> 定频滚轮 -> 关 app -> 调 `Measure-FrameStats` |

用法:

```powershell
./scripts/perf/Invoke-ScrollTest.ps1 -Exe C:/tmp/pk-perf/bin/perfharness.exe -AppArgs "-scene list -lines 10000" -Seconds 4
./scripts/perf/Measure-Memory.ps1 -Exe C:/tmp/pk-perf/bin/markdown-chat.exe -Runs 3
```

已知边界:`Invoke-ScrollTest` 用系统滚轮,不是精确滚动量(要精确请用 `mygo-automation` 的 e2e 构建,日志交给同一个解析脚本);外部冷启动测不到换文档与首帧内容,复杂文档会明显低估;脚本需要桌面会话,RDP 断开或服务会话里窗口不会正常出现。

## 8. 下一步

1. 在 rewrite 的 app 落地两个打点:`MYGO_STARTUP_TRACE`(进程创建到首帧)与"文档打开到首帧",都是零开销开关。
2. 先定文档模型(块级虚拟化)再写渲染层:本次数据说明整篇 `markdown.View` 在 1 万行就 23.5 fps,块列表形态 55.5 fps。
3. 5 万行 / 5 MB 的超阈值降级:用"块数/字节数 + 预估布局时间"做阈值,并用 `Measure-FrameStats` 复现降级后的帧分布。
4. 搜索尚未实现,先补 benchmark 再谈 UI。
5. macOS 上补 `internal/inputlatency -mode scroll` 的端到端延迟数据。
