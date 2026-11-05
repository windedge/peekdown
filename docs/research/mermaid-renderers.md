# Mermaid 纯 Go 渲染器选型（issue #3）

调研目的：给 Peekdown 的 Go 重写（mygo 原生 UI，无 webview）选一个把 ```mermaid 代码块渲染成 SVG 的方案。
约束前提：`mygo` 的 `ui.SVG` 绘制路径、基本图形、`g`/`use`/`symbol`、transform、颜色、渐变、描边、opacity、clipPath、mask、简单选择器的 `<style>`，
**明确不支持 `<text>`、`<image>`、`<pattern>`、`<marker>`、`<filter>`**（来源：`C:/src/go/mygo/ui/svg.go:13-25` 与 `C:/src/go/mygo/internal/svg/svg.go:1-13`）。

## 结论先行

| 角色 | 选择 | 一句话理由 |
| --- | --- | --- |
| 首选 | `github.com/zkrebbekx/go-mermaid` v0.1.6 | 纯 Go、无 cgo、20 种图类型全覆盖、输出 SVG **不使用 `<style>`/`<foreignObject>`/CSS**，不受支持特性只剩 `<text>` 和单个三角 `<marker>`，适配层最小；二进制增量 +1.60 MiB，冷启动约 110 ms |
| 备选（视觉一致性优先时） | `github.com/xo/mermaid` v0.1.1 | 用 wazero 跑 `mermaid-rs-renderer` 0.3.1 的 WASM，与 Rust 版同一渲染器家族（Rust 版为 0.2.2），视觉最接近现有 Peekdown；代价是 2.9 MB WASM 内嵌、二进制 +6.64 MiB、**每次进程冷启动约 1.9 s** |
| 淘汰 | `bvolpato/mermaid-go-renderer`（mmdg） | 主要图类型用 `<foreignObject>`+HTML `<div>` 放标签，非 web 渲染器无法还原；且仓库**没有 LICENSE 文件** |
| 淘汰 | `Luo-Studio/go-mermaid` | 只支持 5 种图（无 pie/gantt/journey/...）；SVG 输出每张图 **540 KB 起**（`<style>` 内嵌 Inter 字体 base64） |
| 淘汰 | `yashikota/mermaigo`（现 `a-kaibu/mermaigo`） | 只支持 5 种图；`<style>` 用 `@import` 拉 Google Fonts + CSS 变量 + `color-mix()`，在 mygo 里等于样式全丢 |
| 不存在 | `rickil/mmdr-go` | `gh api repos/rickil/mmdr-go` 返回 404，无此仓库；cgo 对照项无从对比 |

选型的核心判断：**五个候选没有一个能绕过 `<text>`**，所以无论选谁都要写一层 "文本转路径" 的后处理（或改为在 Go 侧自绘文本）。
因此真正的区分度变成"除文本外还要额外处理多少 mygo 不支持的 SVG 特性"，`zkrebbekx/go-mermaid` 在这一维度上明显最优。

## 候选核实总表

数据来源：`gh api repos/<repo>`（star/许可证/推送时间）、各仓库 `go.mod`/`README`/`LICENSE`、以及在 `C:/tmp/pk-mermaid-probe` 里实际 `go run` 渲染 20 种图的实测结果。

| 候选 | 版本 | 许可证 | 纯 Go / cgo | 直接依赖数 | 图类型数 | star | 最后推送 | 二进制(win/amd64, -s -w) | 冷启动 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `zkrebbekx/go-mermaid` | v0.1.6 | MIT (c) Zac Krebbekx | 纯 Go，无 cgo | 4 (+7 间接) | 20 | 12 | 2026-10-17 | 2,891,264 B（+1,675,264） | ~110 ms |
| `xo/mermaid` | v0.1.1 | MIT (c) Kenneth Shaw + `LICENSE.mermaid-rs-renderer` MIT | 纯 Go，无 cgo，内嵌 2.9 MB `mermaid.wasm`，跑在 wazero 上 | 1 (+1 间接) | 23 / 31 文档类型 | 0 | 2026-09-14 | 8,182,784 B（+6,966,784） | ~1.9 s（WASM 编译） |
| `bvolpato/mermaid-go-renderer` | v0.11.1 | **无 LICENSE 文件**（`license: null`） | 纯 Go，无 cgo | 3 (+3 间接) | 20 | 4 | 2026-10-21 | 4,482,048 B（+3,266,048） | ~140 ms |
| `Luo-Studio/go-mermaid` | 伪版本 `v0.0.0-20260508212710` | MIT (c) Luo Studio Oy | 纯 Go，无 cgo（`nulab/autog` 布局） | 3 (+30 间接，含 `tdewolff/canvas`、`x/image`） | 5 | 0 | 2026-05-08 | 11,506,176 B（+10,290,176） | ~130 ms |
| `yashikota/mermaigo` | 伪版本 `v0.0.0-20260204123623` | MIT (c) 2026 kota | 纯 Go，无 cgo，布局走 `goccy/go-graphviz`（Graphviz WASM / wazero） | 1 (+7 间接) | 5 | 0 | 2026-02-04 | 9,044,992 B（+7,828,992） | ~150 ms |
| `rickil/mmdr-go` | 不存在 | - | - | - | - | - | - | - | - |

基线：同样条件下的空 `main` 二进制 = 1,216,000 B（Windows/amd64，Go 1.27.2，`-trimpath -ldflags="-s -w"`）。

## 逐项核实

### 1. 图类型支持与降级方式

实测方法：在 `C:/tmp/pk-mermaid-probe/samples.txt` 写 20 段最小示例（flowchart / sequence / class / state / er / pie / gantt / journey / gitGraph / mindmap / timeline / quadrant / requirement / C4 / sankey / xychart / block / kanban / packet / radar），逐个候选跑一遍。

| 图类型 | zkrebbekx | xo (mmdr) | bvolpato | luo | mermaigo |
| --- | --- | --- | --- | --- | --- |
| flowchart/graph | OK | OK | OK | OK | OK |
| sequenceDiagram | OK | OK | OK | OK | OK |
| classDiagram | OK | OK | OK | OK | OK |
| stateDiagram-v2 | OK | OK | OK | OK | OK |
| erDiagram | OK | OK | OK | OK | OK |
| pie | OK | OK | OK | **不识别** | **不识别** |
| gantt | OK | OK | OK | **不识别** | **不识别** |
| journey / gitGraph / mindmap / timeline / quadrant / requirement / C4 / sankey / xychart / block / kanban / packet / radar | 全部 OK | 全部 OK | 全部 OK | 不识别 | 不识别 |

不支持时的降级行为差异很大，这点对"用户随手写错语法"的 Markdown 阅读器很关键：

- `zkrebbekx`：未知类型返回 `ErrUnsupported`（`mermaid.go` 包注释与 `errors.go`），调用方可以据此回退成代码块。解析失败包成 `*ParseError`（含 line/col），可用 `errors.As` 取位置。
- `xo/mermaid`：**7 种 `-beta` 图会被静默当 flowchart 渲染**，返回一个语法合法但内容荒谬的 SVG（README 的 Fidelity 一节明确列出 `venn-beta`/`ishikawa-beta` 等）；只有 `eventmodeling` 会报错。要避免展示垃圾输出，调用方必须自己维护一份拒绝清单。
- `bvolpato`：`api.go:ensureHighFidelityOrAllowApproximate` 对非原生类型直接报错，除非开了 `AllowApproximate`。
- `luo`：`ErrUnknownDiagram`。
- `mermaigo`：`invalid mermaid header: ...`。

### 2. 许可证

- `zkrebbekx/go-mermaid`：MIT，`LICENSE` 版权人 Zac Krebbekx。只需随二进制附带 MIT 声明。
- `xo/mermaid`：MIT（Kenneth Shaw）+ 内嵌 WASM 对应 `mermaid-rs-renderer` 的 MIT（`LICENSE.mermaid-rs-renderer`）；README 明确要求再分发时**两份许可都要带**，也就意味着要写进 About/许可页。
- `Luo-Studio/go-mermaid`：MIT（Luo Studio Oy）；另需遵守内嵌 Inter 字体的 SIL OFL 1.1（`fonts/LICENSE-Inter.txt`）。
- `yashikota/mermaigo`：MIT（kota）。
- `bvolpato/mermaid-go-renderer`：**仓库内无 LICENSE 文件，GitHub API `license: null`**。README 只写 "inspired by mermaid-rs-renderer"，而后者是 MIT，但"inspired"不构成派生声明，代码也有大量同名结构。默认版权保留，分发前无法确认授权，这是硬阻塞项。

### 3. 依赖与 cgo

| 候选 | 是否 cgo | 内嵌大二进制 | go.mod 直接 / 间接依赖 |
| --- | --- | --- | --- |
| zkrebbekx | 否 | 无（无 embed 资源） | 4 / 7：`goconvey`、`oksvg`、`rasterx`、`x/image`；后三者只在 `raster` 子包（PNG）用到，只出 SVG 时不会链接进二进制 |
| xo | 否 | **2.9 MB `mermaid.wasm`**（`mermaid.wasm` 2910110 B，`//go:embed`） | 1 / 1：`tetratelabs/wazero`、`x/sys` |
| bvolpato | 否 | 无 | 3 / 3：`oksvg`、`rasterx`、`x/image` |
| luo | 否 | 无 embed，但经 `tdewolff/canvas` 拉进 `go-text/typesetting`、`x/image`、`fpdf`、`goldmark`、`gonum/plot`、`star-tex` 等 | 3 / 30 |
| mermaigo | 否 | Graphviz WASM（经 `goccy/go-graphviz` + wazero） | 1 / 7 |

全部候选都不是 cgo，`CGO_ENABLED=0` 交叉编译没问题（本报告所有二进制都是默认 cgo 关闭状态下 `go build` 的 Windows/amd64 产物）。

值得注意：`zkrebbekx` 的 SVG 路径**不依赖** `oksvg`/`rasterx`（那两个是 `raster` 子包做 PNG 用的），所以只出 SVG 时依赖面很干净；实测体积增量 1.6 MiB 主要来自它自己的 20 套 parser/layout/render 代码。

### 4. 输出形态与 API

```go
// zkrebbekx/go-mermaid（SVG 用 []byte，另有 raster.PNG 子包）
svg, err := mermaid.Render(src string, opts ...Option) ([]byte, error)
//   Option: WithTheme/WithFont/WithPadding/WithSpacing/WithCustomTheme/WithCurvedEdges...

// xo/mermaid（string，含 Renderer 复用与字体注册）
r, err := mermaid.New(ctx, opts...) (*Renderer, error)
svg, err := r.Render(ctx, src string) (string, error)
svg, err := r.RenderWith(ctx, src string, opts Options) (string, error)
err = r.RegisterFont(ctx, ttf []byte) error
v := r.Version() string

// bvolpato/mermaid-go-renderer（string）
svg, err := mermaid.Render(input string) (string, error)
svg, err := mermaid.RenderWithOptions(input string, options RenderOptions) (string, error)
res, err := mermaid.RenderWithDetailedTiming(input string, options RenderOptions) (RenderDetailedResult, error)

// luo：DisplayList IR + 三个 emitter
dl, err := mermaid.ParseAndLayout(src string, opts mermaid.LayoutOptions) (*displaylist.DisplayList, error)
svg, err := mermaidcanvasr.RenderSVG(src string, opts mermaidcanvasr.RenderOptions) ([]byte, error)
png, err := mermaidcanvasr.RenderPNG(src string, opts mermaidcanvasr.RenderOptions) ([]byte, error)
err := mermaidpdf.DrawMermaid(pdf *fpdf.Fpdf, src string, x, y float64, opts ...)  // PDF

// yashikota/mermaigo
svg, err := mermaid.Render(src string, opts *mermaid.RenderOptions) (string, error)
text, err := mermaid.RenderText(src string, opts *mermaid.RenderOptions) (string, error) // ASCII/Unicode
```

`luo` 的 `displaylist` 是唯一提供"与输出格式无关 IR"的候选，`Item` 只有 `Shape`/`Edge`/`Text`/`Cluster`/`Marker` 五种，带 `Role` 语义标签（`displaylist/displaylist.go`）。
这在架构上最贴近"直接往 mygo 场景里画"的思路（见下文方案 B），但它只覆盖 5 种图。

### 5. 输出 SVG 实际使用了哪些特性（实测，最重要的一节）

方法：把 5 个候选渲染的 20（mermaigo/luo 为 5）张 SVG 存到 `C:/tmp/pk-mermaid-probe/out/all_*/`，用 `grep -o "<tag[ >]"` 逐个统计。

| 特性 | zkrebbekx | xo | bvolpato | luo | mermaigo |
| --- | --- | --- | --- | --- | --- |
| `<text>` | 有（每图 3-5 个） | 有 | 仅 sequence/pie | 有 | 有 |
| `<tspan>` | 有（仅 flowchart/多行标签，只有 `x`/`y`） | 有（带 `dy`、`alignment-baseline`、`font-weight`、`x`） | 有 | 有（每行一对 `x`/`y`） | 无 |
| `<foreignObject>` | **无** | **无** | **有**（flowchart/class/state/er 的标签全是 `<div><span>`） | 无 | 无 |
| `<marker>` | 有（只有 1 种 `id="arrow"` 三角） | 有（`arrow-N`/`arrow-start-N`，但 flowchart 同时用显式 `<polygon>`+`rotate` 画箭头） | 有（多种） | **无** | 有 |
| `<style>` / CSS | **无**（全部写在 presentation attribute 上） | 无 `<style>`，只有无害的 `class="edgePath"` 等属性 | 有：`#my-svg{...}` 选择器、`@keyframes dash`、`!important` | 有：`@font-face` 内嵌 Inter base64 | 有：`@import url(https://fonts.googleapis.com/...)`、CSS 变量、`color-mix()` |
| `<pattern>` / `<filter>` / `<image>` / 渐变 / `clipPath` / `mask` | 均无 | 均无 | 均无 | 均无 | 均无 |
| `width="100%"` | 无（有明确 width/height） | **有**（pie 等 `width="100%"` + `style="max-width"`） | **有**（全部） | 无（用 mm 单位） | 无 |
| 平均元素数/图 | 12 | 25 | 40 | 15 | 22 |
| 单图字节数（flowchart） | 1,357 B | 3,297 B | 8,998 B | **544,197 B** | 2,950 B |

样例文件：`docs/research/fixtures/{zkrebbekx,xo,bvolpato,mermaigo}/{flowchart,sequence,class,state,er,pie,gantt,mindmap}.svg`。
`luo` 的样例没有入库：单文件 544 KB 到 1.1 MB（`<style>` 里塞了 base64 的 Inter 字体），超过 200 KB 上限；这也正是它被淘汰的直接原因。

**用 mygo 自己的解析器实测**：把 `C:/src/go/mygo/internal/{svg,vec}` 拷到临时模块 `C:/tmp/pk-mermaid-probe/mygoprobe`，对全部输出调用 `svg.Parse`：

- 五个候选的 SVG **全部解析成功，没有 XML 层面的报错**（`internal/svg/svg.go:32`）。
- 解析出的尺寸基本等于 viewBox；`width="100%"` 的 bvolpato flowchart 被解析成 108x284（viewBox 是 `0 0 290 284`），说明百分比宽度会让尺寸估计失真，需要渲染前把 width/height 规范化。
- 但"解析成功"不等于"画得出来"：`<text>`、`<marker>`、`<foreignObject>` 会被静默丢弃（`internal/svg/svg.go:1-13` 的包注释即声明），结果是**框线都在、文字和箭头头部都不在**。

结论：所有候选都需要一层文本处理，差别在于附加成本：

- `zkrebbekx`：只需处理 `<text>`/`<tspan>` 与单一三角 marker。marker 只有 `id="arrow"` 一个形状，且 `refX/refY/markerWidth` 固定，展开成路径是最简单的一类；`<tspan>` 只带绝对 `x`/`y`，不带 `dy`/`alignment-baseline`。
- `xo`：`<text>` 带 `textLength`+`lengthAdjust`（需要按字距缩放），tspan 带 `dy`/`alignment-baseline`/`font-weight`，还有 `translate(...) rotate(...)` 组合变换；marker 有 2 种 × 每边编号，但 flowchart 里箭头已用显式 polygon 画，marker 损失相对小。
- `bvolpato`：直接出局。`<foreignObject>` 里是 HTML `div`/`span` 且靠 CSS 换行（`display: table-cell`、`max-width`），纯 Go 无法还原，等于所有节点标签都没有。
- `luo`：无 transform、无 marker，几何最简单（每个 `<text>` 里带 `style="fill:...;font-family:Inter;font-size:..."`），如果能接受体积，反而是最容易解析的一类；但每张图 540 KB 的字体 base64 在阅读器里不可接受。
- `mermaigo`：文字几何简单，但颜色靠 `var(--fg)` / `color-mix()` 计算，mygo 的"简单选择器"样式表不支持 CSS 变量与函数，等于配色全丢；并且 `<style>` 里带 `@import` 外部字体，离线阅读器还得自己剥掉。

### 6. Windows / amd64

- 五个候选在 Windows/amd64 + Go 1.27.2 下都 `go build` 通过、`go run` 渲染成功，没有平台特化代码路径（无 cgo、无 `syscall` 依赖）。
- `xo/mermaid` 是唯一有实际 Windows 风险的：wazero 的编译型后端需要可执行内存（`VirtualAlloc`），在部分受限环境/杀软策略下会被拦；本次实测正常（`New` 1.8-2.6 s，`Render` 0.5-27 ms）。
- `zkrebbekx` README 声明有"Linux/macOS/Windows (amd64/arm64)" 的 GoReleaser 预编译产物，`.goreleaser.yaml` 在仓库内，说明 Windows 是 CI 覆盖的目标平台。
- 已知渲染问题（影响可用性，非平台相关）：`xo/mermaid` README 自述 sankey 布局有 bug、YAML front matter 被丢弃、`%%{init}%%` 无效；`bvolpato` README 自述 23 个 fixture 里只有 8 个与 mmdc 达到标签级一致。

### 7. 维护活跃度与风险

| 候选 | star | fork | open issues | 最近 release | 最近 commit | 风险判断 |
| --- | --- | --- | --- | --- | --- | --- |
| zkrebbekx | 12 | 4 | 22 | v0.1.6 (2026-09-10) | 2026-10-17 | 活跃开发中，release-please + GoReleaser + CI/golden 测试；22 个 open issue 说明还有不少边角未修 |
| xo | 0 | 0 | 0 | 无 GitHub release | 2026-09-14 | 单人一次提交式产出（"Claude generated Go bindings"），代码质量高、测试详尽，但 bus factor = 1 且无 release 流程 |
| bvolpato | 4 | 1 | 1 | v0.11.1 (2026-03-01) | 2026-10-21 | 活跃，但许可证缺失是硬阻塞 |
| luo | 0 | 0 | 0 | 无 release（只有伪版本） | 2026-05-08 | 无 tag、无 release，文档齐全但停在 5 种图 |
| mermaigo | 0 | 0 | 0 | 无 release | 2026-02-04 | 仓库已从 `yashikota` 改名到 `a-kaibu`，但 go.mod module path 仍是 `github.com/yashikota/mermaigo`；近 9 个月无提交 |

版本可获取性（重要）：只有 `zkrebbekx`（v0.1.6）和 `bvolpato`（v0.11.1）有正式 tag；`luo` 与 `mermaigo` 只能拿到伪版本，意味着**上游一改代码，`go get` 的结果就变**，需要 `go.sum` + vendor 或自己 fork 打 tag 才能锁死。

## 推荐方案

### 首选：`zkrebbekx/go-mermaid` + 自写 "SVG 降级层"

理由（按权重）：

1. **适配层成本最低**：输出里没有 `<style>`、没有 `<foreignObject>`、没有 CSS、没有渐变/pattern/filter，不受支持的特性只有 `<text>`/`<tspan>` 和单一三角 `<marker>`，全部写在 presentation attribute 上，解析后可以直接映射到 mygo 的 `internal/svg` 能力。
2. **图类型覆盖最广且降级可控**：20 种图全部实测通过，未知类型返回 `ErrUnsupported`，解析错误带行列号，可以直接回退成"语法高亮代码块 + 错误提示"。
3. **成本最低**：+1.6 MiB、冷启动 ~110 ms、单图 1-3 ms、输出 1-3 KB。
4. **依赖面干净**：只出 SVG 时不会链接 oksvg/rasterx；无 embed 资源、无 WASM、无 cgo。
5. 有正式 tag、CI、golden 测试、Windows 预编译产物，维护节奏可见。

已知代价与应对：

- **视觉与 Rust 版不一致**：`zkrebbekx` 是独立重写（README 自述 "an independent, compatible renderer"），布局与配色跟 `mermaid-rs-renderer` 0.2.2 有肉眼可见差异。Peekdown 重写本来就不要求像素级对齐，但如果验收标准里有"和 Rust 版截图对比"，需要提前和产品确认放宽。
- **22 个 open issue**：接入前应把 CI 的 golden SVG 全部跑一遍，重点看 flowchart 子图、序列图 `alt/par` 帧、类图关系符。
- **CJK 字体度量**：它用内置 Helvetica advance width 估宽，中文标签实测节点框宽 101.46 px 容纳 "开始 Start"，尚可，但长中文标签有溢出风险，需要用 `WithFont`/自定义度量或后续 issue 跟踪。

### 备选：`xo/mermaid`（仅当"视觉与 Rust 版一致"是硬需求）

它是本次唯一与 Rust 版共用同一渲染器内核的选项（`mermaid-rs-renderer` 0.3.1 vs Rust 版 0.2.2），23/31 种文档图类型可用，README 对每个已知缺陷都有测试钉住，工程严谨度最高。代价：

- 二进制 +6.64 MiB（2.9 MB WASM + wazero）；
- 首次 `New(ctx)` **1.8 s 起步**（本机实测 1.75-2.59 s），对一个"双击打开 Markdown"的阅读器是明显的启动税，必须放到后台 goroutine 预热或用 `wazero.NewCompilationCacheWithDir` 落盘编译缓存；
- 7 种 `-beta` 图会被静默降级成 flowchart，需要调用方维护拒绝清单；
- `<text>` 带 `textLength`/`lengthAdjust`/`dy`，降级层比 `zkrebbekx` 复杂一档；
- 0 star、无 release、单人项目，长期维护风险高。

### 淘汰理由汇总

- `bvolpato/mermaid-go-renderer`：**没有 LICENSE**（分发不可行）+ 主流图类型用 `<foreignObject>` 输出 HTML 标签（非 web 渲染器无法还原，等于标签全丢）+ `width="100%"` 需要额外规范化。
- `Luo-Studio/go-mermaid`：只支持 5 种图（无 pie/gantt/journey/mindmap/...，Markdown 场景里 pie 和 gantt 很常见）；每张 SVG 540 KB - 1.1 MB（`@font-face` 内嵌整个 Inter 字体 base64）；30 个间接依赖。它的 `displaylist` IR 思路值得借鉴（见方案 B），但当前覆盖度不够。
- `yashikota/mermaigo`：只支持 5 种图；`<style>` 依赖 `@import` 外链字体 + CSS 变量 + `color-mix()`，在 mygo 里样式基本全丢；仓库改名后 module path 未同步，9 个月无提交。
- `rickil/mmdr-go`：仓库不存在（GitHub API 404）。

## 两个实现方向（供 issue #3 拆任务参考）

### 方案 A（推荐）：SVG 后处理降级层

在 `zkrebbekx/go-mermaid` 输出之后、`ui.ParseSVG` 之前插一个纯 Go 的 SVG 规整步骤：

1. **文本转路径**：遍历 `<text>`/`<tspan>`，按 `font-family`/`font-size`/`font-weight`/`text-anchor` 选字体（内嵌一份 Inter 或 Noto Sans CJK，只取需要的子集），用 `golang.org/x/image/font/sfnt` + `opentype` 取字形轮廓，生成 `<path>` 替换原节点；多行 `<tspan>` 按各自 `x`/`y` 定位。这是最大的一块工作量。
2. **marker 内联**：`<marker>` 只有一种（三角），把引用处按 `refX/refY`/`markerWidth`/`orient` 计算旋转平移，替换成内联 `<path>`。
3. **规范化尺寸**：把 `width="100%"` 换成 viewBox 宽高（`zkrebbekx` 目前不产生这种写法，但为鲁棒性保留）。
4. 结果交给 `ui.ParseSVG`，缓存 `*ui.SVG`（按 mermaid 源码 hash），滚动重绘不重复解析。

### 方案 B（备选，长期更优）：自建 DisplayList 直绘

借 `Luo-Studio/go-mermaid` 的 `displaylist` 形态（`Shape`/`Edge`/`Text`/`Cluster`/`Marker` + `Role`），让渲染器输出结构化图元，再由一个 mygo emitter 直接画到 scene，文本用 mygo 原生文本 API 而非 SVG 里的 `<text>`。
优点：完全绕开 SVG 解析与文本转路径，主题/高亮可以跟着应用走。
缺点：需要上游支持或自己维护一层适配（`zkrebbekx` 目前不暴露 IR），且 20 种图的 emitter 工作量远大于方案 A。
建议：先做方案 A 落地，把 IR 作为后续优化项记录在 issue 里。

## 复现方式

```bash
# 候选源码
git clone --depth 200 https://github.com/zkrebbekx/go-mermaid
git clone --depth 200 https://github.com/xo/mermaid
git clone --depth 200 https://github.com/bvolpato/mermaid-go-renderer
git clone --depth 200 https://github.com/Luo-Studio/go-mermaid
git clone --depth 200 https://github.com/a-kaibu/mermaigo

# 实验目录（不属于仓库）
#   C:/tmp/pk-mermaid-probe/p_zk|p_xo|p_bvo|p_luo|p_aka   20 种图逐个渲染
#   C:/tmp/pk-mermaid-probe/out/all_*                     渲染产物，用于特性统计
#   C:/tmp/pk-mermaid-probe/mygoprobe                     拷入 mygo internal/svg + internal/vec，验证可解析性
#   C:/tmp/pk-mermaid-probe/size/*                        最小示例二进制体积与冷启动计时

# 体积与冷启动
go build -trimpath -ldflags="-s -w" -o bin.exe .   # 每个 size/<candidate>
```

环境：Go 1.27.2（`go version`）在临时模块里未触发 toolchain 切换；worktree 的 `go.mod` 声明 `go 1.27.3`，本机 `C:/src/go/mygo` 也要求 1.27.3，`go version` 在 worktree 里会显示 1.27.3，本次调研未遇到 toolchain 下载问题。
