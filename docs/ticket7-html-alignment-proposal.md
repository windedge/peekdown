# 工单 #7 调研：内嵌 HTML 的语义对齐范围提案

## 一、现状证据

- mygo parser：`HTMLBlock` 块持有原始行（document.go:55，view 走 `d.Lines(i)`）；行内 `HTML` Inline 持原始标签文本（inline.go:30,639）。不解析标签。
- mygo 行内白名单：spans.go:447-497 `inlineHTML` 字符串匹配 `br,b,strong,i,em,u,ins,s,del,strike,code,kbd,samp,mark,sub,sup,small`，维护开标签栈后由 `htmlStyle` 应用；其余标签按原文显示。工单的 8 标签已全部生效。
- ui.Span 词汇（richtext.go:15-70）：Italic、Underline、Strikethrough、Background+Radius+PaddingX、BaselineShift、Font，足够映射 em/strong/del/u/code/kbd/mark/sub/sup/br。
- peekdown：view.go:365-377 HTMLBlock 渲染为等宽灰文本；view.go:487-528 自有 `walk`（parser.HTML 按原文 code 样式）；style 结构仅 size/weight/color/underline（view.go:390）。doc/model.go:479,519 HTMLBlock 是一个 run，搜索/选区作用于该原始文本。
- 依赖：peekdown 与 mygo 的 go.mod 均含 `golang.org/x/net v0.59.0`（peekdown 为 indirect），`x/net/html`、`x/net/html/charset` 已在模块缓存中。
- 折叠组件：mygo 有 `ui.Collapsible`（api_gen.go:1052）。

## 二、分层支持矩阵（提案）

**块级**（x/net/html 解析 HTMLBlock 内容后按元素分派，复用现有渲染器）：

| 类别 | 标签 | 处理 |
|---|---|---|
| 映射现有块 | table/thead/tbody/tr/td/th | 复用 `st.table`（实现备注：HTML 表无对齐语义，v0.3.0 列宽保持均分，不做测量定宽） |
| | img（块内独占） | 复用 `st.images`/doc.Image |
| | hr | ThematicBreak 渲染 |
| 容器透明展开 | div/section/article/aside/header/footer/main/nav/figure/figcaption/blockquote/ul/ol/li/dl/dt/dd/details/summary/p/h1-h6 | 递归展开子节点：p/hN→Paragraph 渲染，ul/ol/li→list，其余仅作容器，无样式 |
| 保留原样 | pre | CodeBlock 渲染（无高亮） |
| 忽略 | script/style/head/meta/link/title | 不产出内容 |
| 降级 | 其余未知标签（iframe/form/input/button 等） | 显示子文本节点，剥标签 |

Rust 版 35 种块级容器中，阅读器场景不值得对齐：form 控件、video/audio、canvas、svg。以上矩阵覆盖 ~22 种，即"常见文档 HTML"。

**行内**：复用 mygo 语义映射到扩展后的 style：`b/strong`→weight700、`i/em`→italic、`u/ins`→underline、`s/del/strike`→strikethrough、`code/kbd/samp`→等宽+Background+Radius（实现备注：code 采用与 markdown 行内一致的 muted 样式，不做背景块）、`mark`→Background 高亮、`sub/sup`→size*0.75+BaselineShift、`br`→换行、`a`→链接样式（沿用 walk 的 Link 处理，取安全 href）、行内 `img`→`[alt]` 文本（v1）。未知行内标签降级为其文本。

## 三、解析管线

在 `internal/view`（或新 `internal/htmlblk`）新增转换器：HTMLBlock 的 `Lines(i)` → `x/net/html.ParseFragment`（body 上下文）→ 遍历语义树 → 产出"虚拟块列表"（类型+子行内节点），行内文本节点需经 mygo 行内 parser 二次解析（段落内可再含 `**` 等）或按纯文本+行内标签扫描处理。**复用现有渲染器**（paragraph/table/codeBlock/images/list），不新建 HTML 块渲染器；映射结果缓存于 Model（随 doc revision 失效）。run 表改为从语义树叶子的 Plain 文本构建（model.go:465 appendRuns 增加对语义块的分支），搜索/选区随之天然工作。编码：文件级 UTF-8 假定不变，charset 包不引入（v1）。

## 四、安全白名单

- `a[href]`：仅 http/https/mailto/#fragment，其余剥为纯文本；`javascript:` 一律拒绝。
- `img[src]`：仅 http/https/data:image/(png|jpeg|gif|webp)；data:image/svg+xml 拒绝（可含脚本）。
- 全局剥：`on*` 属性、`style` 属性、`class`/`id`、事件与任意 URL 属性（srcset 等）。
- 无脚本执行环境（无 JS 引擎），风险面主要是外链请求与 data: URI 体量；img 解码复用现有图片管道的大小限制。
- 拒绝清单直接不渲染：script/style/iframe/object/embed/form。

## 五、明确不做（v0.3.0 out of scope）

- `<details>/<summary>` 折叠交互（mygo 有 `ui.Collapsible` 可后补，但需 run 表与滚动定位配套，推后）
- `mark` 高亮、`sub/sup` BaselineShift（透明降级，不产出样式）
- `ul/ol/li` 列表 marker（容器透明分段，不出编号/圆点）
- Model 级 HTML 节点缓存（选 RunText 方案：doc 侧构建 run 时用 `htmlx.RunText` 一次解析，view 渲染树每帧重建、每帧每块 Parse 一次，HTML 块少，可接受）
- CSS/style 属性、class 主题映射
- svg/video/audio/canvas、表单控件
- charset 编码探测
- 行内 img 实际渲染
- mathml

## 六、工作量与文件影响

预估 **24-30 人时**：
- 语义树转换器 + 块级映射：10h（新 `internal/view/htmlblk.go` + 测试）
- 行内标签映射（style 扩展 italic/strike/mark/baseline + walk 改造）：6h（view.go:390,487）
- run 表/搜索/选区适配（model.go:465）：4h
- 安全白名单 + 测试：3h
- 回归测试与样例文档：3h

文件：`internal/view/view.go`、`internal/view/htmlblk.go`(新)、`internal/doc/model.go`、`internal/view/view_test.go`、`go.mod`（x/net 转 direct）。
