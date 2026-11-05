# Markdown 语法差异清单：Rust 版 peekdown vs mygo 重写版

对应工单：GitHub issue #8。基线 commit `a9cc28e`，分支 `research/markdown-diff`。

## 0. 结论先行

1. 两边的 CommonMark 基础行为（标题、引用、列表、代码块、强调、删除线、表格对齐、autolink、引用式链接、CJK 强调）基本一致，差异集中在"扩展构造"和"HTML"两块。
2. 真正会造成用户可见回归的只有 4 类：
   - frontmatter：Rust 专门解析成键值卡片，mygo 没有 Frontmatter 块类型，`---` 被当成 thematic break + setext H2，文档开头会多一条分割线和一个大标题。
   - HTML 块 / 行内 HTML：Rust 用 html5ever 结构化解析（35 种块级元素、`<img>` 带 width、`<details>`、`<table>`、`<br>`、注释丢弃），mygo 的 HTMLBlock 默认按等宽文本原样显示源码，行内只认 17 个格式化标签。
   - 数学：Rust 的 `ParseOptions::gfm()` 只额外开了 frontmatter，没开 math，`$$`、`\[`、`\(` 都是原样文本；mygo 的 `parser.Default` 含 `ExtMath`，这些会变成数学块/行内数学，源码里的 `$$` 会"消失"。
   - 混排图片：Rust 允许段落里文字与图片混排；mygo 只在"整段只有图片"时才渲染图片，否则退化成 `[alt]` 文本，README 徽章（`[![b](x.svg)](url)`）会变成一行 `[b]` 链接。
3. 其余差异多为"mygo 更正确"或纯视觉：硬换行（Rust 丢 `Break`，两行直接拼接）、脚注（Rust 就地 `[id]:`，mygo 收到底部编号）、表格列宽（Rust 按字节数估，mygo 按实测像素）、标题 slug（`_`、重复标题去重）、代码块 info string（mygo 保留全串）。
4. 建议的落地策略：app 侧做一层 markdown 预处理 + 选项接线（见附录 A），不要把差异留到渲染期用 hack 兜。

## 1. 核对方法

- Rust 侧：读源码定论，未构建（`C:/Seafile/Works/rust/peekdown`，只读）。`markdown` crate 版本 1.0.0（`Cargo.toml:39`）。
- mygo 侧：读源码 + 跑探针 `C:/tmp/mdprobe`（`replace github.com/egoist/mygo => C:/src/go/mygo`），探针直接打印 `parser.Document` 的块树与行内树，以及 `markdown.Slug` 结果。探针用法见附录 B。
- 主表"证据"列给出 `文件:行号`，Rust 侧省略前缀 `C:/Seafile/Works/rust/peekdown/`，mygo 侧省略前缀 `C:/src/go/mygo/`。

## 2. 主表

### 2.1 块级构造

| # | 语法点 | Rust 行为 | mygo 行为 | 影响 | 建议处理 | 证据 |
|---|---|---|---|---|---|---|
| 1 | YAML frontmatter | `ParseOptions::gfm()` + `constructs.frontmatter = true`；`Node::Yaml` 交 `state::frontmatter::parse`（serde_yaml），成功则 `BlockNode::Frontmatter`，渲染成等宽键值卡片（空 frontmatter 渲染空）；解析失败降级为 `yml` 代码块 | 无 Frontmatter 块类型；`---` 解析为 thematic break，随后内容行 + 收尾 `---` 组成 setext H2。探针 `frontmatter`：块 1 = thematic break，块 2 = heading level 2 文本 `title: Hello` / `tags: [a, b]` | 高：所有带 frontmatter 的文档开头出现"一条分割线 + 一个大标题" | 必须补：app 侧在喂给 mygo 前剥离 frontmatter，自行渲染卡片（或用 `Options.Block` 自绘伪块） | Rust `src/text/format/markdown.rs:48-49`、`markdown.rs:389-405`、`src/state/frontmatter.rs:102-134`、`src/text/node.rs:1471-1479`；mygo `ui/markdown/parser/document.go:42-73`（Kind 列表无 Frontmatter） |
| 2 | TOML frontmatter（`+++`） | `Node::Toml` 转语言为 `toml` 的代码块 | 普通段落，`+++` 原样显示。探针 `frontmatter-toml` | 中：退化为一段文字 | 建议补：与 YAML 走同一条剥离路径 | Rust `markdown.rs:408-412`；探针 `frontmatter-toml` |
| 3 | HTML 块（结构元素） | `cleanup_html` 先用 html5minify 压缩空白再交 html5ever 解析；`h1-h6`、`ul/ol/li`、`table/tr/td`、`blockquote`、`img`、`br` 都转成结构化块；`style`/`script` 整块丢弃；注释、doctype、处理指令丢弃；35 个块级元素白名单，未列入的标签内容降级为行内文本 | `HTMLBlock` 是普通块类型，视图默认按等宽 muted 色原样显示源码；仅当 app 提供 `Options.HTML` 钩子时才交给 app 渲染。探针 `html-type6`、`details-html`、`comment` | 高：`<img width>`、`<details>`、`<p align>`、`<table>`、`<br>`、HTML 注释全部以源码形式出现在正文 | 必须补：app 侧预处理高频子集（`<img>`、`<br>`、`<details>/<summary>`、`<p>`/`<div>` 去壳、HTML 注释整块删除），或用 `Options.HTML` 接轻量 HTML 渲染器 | Rust `src/text/format/html.rs:46-82`、`html.rs:85-123`、`html.rs:417`、`html.rs:592`、`html.rs:640`；mygo `ui/markdown/parser/document.go:56`、`ui/markdown/view.go:357-364`、`ui/markdown/view.go:60-63` |
| 4 | 代码块 info string | mdast 只留 `lang`（首个词），meta（如 `{1,3} title=x`）丢弃，高亮按 lang | `Block.Info()` 保留完整 info 串（反转义），`Block.Language()` 取首词；`CodeBlockView.Language/Info/Open` 都暴露给 app。探针 `fence-info` 得 info=`js {1,3} title=x` | 低（mygo 信息更全） | 可接受差异；若要行高亮，app 可解析 `Info()` 里的 `{1,3}` | Rust `markdown.rs:337-341`；mygo `ui/markdown/parser/document.go:171-193`、`ui/markdown/code.go:13-27` |
| 5 | 缩进代码块 | mdast `Code` 无 lang，`CodeBlock` lang=None，无高亮 | `CodeBlock`，`Fenced=false`，`Language()` 为空。探针 `code-indented` | 无 | 无需处理 | Rust `markdown.rs:336-341`；mygo `ui/markdown/parser/document.go:102-104`、探针 `code-indented` |
| 6 | 未闭合围栏 | mdast 吃到文档结尾（无特殊标记） | `Block.Open=true`，内容到结尾；`CodeBlockView.Open` 让 app 知道"还在写"。探针 `unclosed-fence` | 无（mygo 更有信息） | 无需处理 | Rust `markdown.rs:336-341`；mygo `ui/markdown/parser/document.go:82-85`、`ui/markdown/code.go:18-20` |
| 7 | 表格列对齐 | `Table.column_aligns`（mdast `align`）转单元格 `justify_center`/`justify_end` | `Block.Align []Align`（None/Left/Center/Right）转单元格 `TextAlign`。探针 `table-align` 得 `[1 2 3 0]` | 无 | 无需处理 | Rust `markdown.rs:434-440`、`src/text/node.rs:1748-1769`；mygo `ui/markdown/parser/document.go:106-113`、`ui/markdown/table.go:103-113` |
| 8 | 单元格内行内代码 / 转义竖线 | 单元格走同一 `parse_paragraph`，行内代码正常；转义竖线由 markdown 解析器处理 | 单元格 seg 标记 `cell`，转义竖线解为单个竖线，行内代码照常。探针 `table-cells`：code span 内转义竖线得 `a\|b`，文本内转义竖线得 `x\|y` | 无 | 无需处理 | Rust `markdown.rs:60-75`；mygo `ui/markdown/parser/document.go:139-144`、探针 `table-cells` |
| 9 | 表格列宽计算 | 按单元格文本字节数 `text_len()` 作权重（默认最小 5，clamp 到 6..150，再加 3 padding），每字符估 9px 定整表 `max_w`，flex 比例分配 | 实测文本像素宽度（`MeasureText`）取列最大，上限 `Theme.TableMaxColumn`，超宽横向滚动；流式时列宽只增不减 | 低：CJK 表格 Rust 列宽估算偏差明显，mygo 更准；两者不会像素级一致 | 可接受差异；若需要"和 Rust 一样窄"，调小 `Theme.TableMaxColumn` | Rust `src/text/node.rs:493-534`、`node.rs:1688-1716`；mygo `ui/markdown/table.go:19-79`、`ui/markdown/theme.go:70-74` |
| 10 | 表格缺列 / 多列 | mdast 按书写生成 cell，多余列不渲染（按 `column_aligns` 取索引） | 多余列在视图层被丢弃（`k >= cols` 即 break），缺列留空单元格。探针 `table-cells`：3 列头 + 两列行得 3 个 cell（最后一个空），四列行只保留 3 个 | 无 | 无需处理 | mygo `ui/markdown/table.go:44-62`、`table.go:93-97`、探针 `table-cells` |

### 2.2 行内构造

| # | 语法点 | Rust 行为 | mygo 行为 | 影响 | 建议处理 | 证据 |
|---|---|---|---|---|---|---|
| 11 | 行内 HTML | 与块级共用 html5ever：`b/strong/i/em/del/s/code` 加样式，`a` 变真链接，`img` 变真图片，`br` 变换行；其余标签整体丢弃（只 warn，不显示源码） | 只认白名单 `br/b/strong/i/em/u/ins/s/del/strike/code/kbd/samp/mark/sub/sup/small`；`<br>` 变换行，其余原样显示源码。探针 `html-inline`：`<b>bold</b>` 生效，`<span>sp</span>` 显示源码 | 中：`<span class=...>`、`<a href>`、`<img>` 表现不同（mygo 露源码 / Rust 丢标签或结构化） | 建议补：预处理把行内 `<a href>`/`<img>` 转成 Markdown 语法；`<span>` 之类可"剥标签保内容" | Rust `src/text/format/markdown.rs:183-200`、`src/text/format/html.rs:293-330`、`html.rs:417`；mygo `ui/markdown/spans.go:447-475`、`spans.go:318-321` |
| 12 | 段落内混排图片 / 徽章 | `Paragraph::push_image` 把图片直接塞进段落子节点，文字与图片可混排；链接包图片也能出图 | 只有"整段仅由图片和空白组成"才渲染成图片行（`imagesOnly`）；否则行内图片渲染成 `[alt]` 文本。探针 `badge`：`[![b](x.svg)](https://y)` 得 `link{image{b}}`，按规则退化为 `[b]` | 高：README 顶部徽章、图标行、图文混排全部退化 | 必须补：预处理把"只有链接包图片"的行拆成独立图片段落；或用 `Options.Block` 自绘混排块 | Rust `src/text/format/markdown.rs:158-192`、`src/text/node.rs:629-631`；mygo `ui/markdown/view.go:508-516`、`ui/markdown/spans.go:308-315`、`ui/markdown/image.go:71-110`、探针 `badge` |
| 13 | 图片相对路径 | `resolve_image_url`：`http(s)://`、`data:` 原样；否则 `document_path.parent().join(url)` 且必须 exists 才作为本地文件；否则保留原 url | 视图不解析路径，全靠 `Options.LoadImage`；`FileImages(dir)` 用固定 dir 拼相对路径（非绝对、非 `://`、非 `data:`），支持 SVG | 中：不接线就一张图都不显示；接错会全 404 | 必须补：`LoadImage = markdown.FileImages(filepath.Dir(docPath))`，切换文档同步换 dir（`Options.images` 按 url 缓存，跨文档同名相对路径会串图） | Rust `src/text/format/markdown.rs:20-45`、`src/text/format/html.rs:24-49`；mygo `ui/markdown/image.go:22-45`、`ui/markdown/view.go:64-70` |
| 14 | 链接相对路径 / 锚点 | 只特殊处理 `#anchor`（查 `heading_map` 滚动），其余一律 `cx.open_url`；相对路径不解析 | 优先 `Options.OnLink`；否则 `#anchor` 滚动、`ui.SafeToOpen` 允许的 scheme 打开、router 内无 scheme 路径跳转 | 低：两者都不解析相对 .md 路径；但 mygo 设了 `OnLink` 后 `#anchor` 必须由 app 自己处理 | 建议补：app 的 `OnLink` 里先判 `#`，再判相对路径（同目录 .md 直接切文档） | Rust `src/text/inline.rs:413-428`、`src/text/state.rs:280-288`；mygo `ui/markdown/view.go:485-505`、`view.go:520-531` |
| 15 | 脚注 | 引用处渲染斜体 `[id]`；定义处就地渲染成斜体 `[id]: ` 加内容的段落（不收集到底部，不重新编号） | 引用处渲染下标数字（按定义顺序编号）；定义块收进 `footnotes()` 在文档底部统一渲染成 `1.` 编号列表，前面加一条分隔线 | 中：位置和编号规则都不同（mygo 按定义顺序，GFM 按首次引用顺序）；Rust 的就地 `[id]:` 不会出现在 mygo 里 | 建议补：要贴近 GFM 需上游改编号顺序；否则按改进接受，验收基线里剔除脚注文档 | Rust `src/text/format/markdown.rs:207-215`、`markdown.rs:454-473`；mygo `ui/markdown/spans.go:333-345`、`ui/markdown/view.go:647-690` |
| 16 | 块级/行内数学（`$$`、`\[..\]`、`\(..\)`） | 未开启 math 构造（只开 frontmatter），`Node::Math`/`Node::InlineMath` 分支实际不可达；`$$y$$`、`\[z\]` 原样文本 | `parser.Default` 含 `ExtMath`，`$$y$$`/`\[z\]`/`\(w\)` 变行内数学，独占块的 `$$` 变 `MathBlock`；默认以等宽 muted 文本显示，可接 `Options.Math`/`InlineMath`。探针 `dollar-default`、`dollar-block` | 中：文档里的 `$$`、`\[`、`\(` 会被吃掉（显示为数学样式），与 Rust 可见性不同 | 必须决策：要一致就设 `Extensions: parser.GFM \| parser.ExtFootnotes`（去掉 `ExtMath`）；要增强就保留并接线 `Options.Math`/`InlineMath` | Rust `src/text/format/markdown.rs:48-49`、`markdown.rs:172-180`、`markdown.rs:366-370`；mygo `ui/markdown/parser/document.go:35-38`、`ui/markdown/view.go:366-377`、`ui/markdown/spans.go:322-332` |
| 17 | 单 `$` 数学歧义 | 无（未开 math） | `ExtDollarMath` 不在 `Default` 里，`$x$` 保持文本；即使开启，`$5 and $10` 也不误判。探针 `dollar-default`、`dollar-money` | 无（当前默认即与 Rust 一致） | 保持不开启 `ExtDollarMath` | mygo `ui/markdown/parser/document.go:28-32`、`document.go:38`、探针 `dollar-money` |
| 18 | 任务列表 | mdast `checked` 转自绘 checkbox（primary 边框/填充 + 勾图标），`spread` 时前面加空 div | `Item.Task`（Unchecked/Checked）转自绘圆角 checkbox，尺寸随字号。探针 `task-list` 得 task=1/2 | 无（视觉细节差异） | 无需处理 | Rust `src/text/node.rs:1639-1661`；mygo `ui/markdown/view.go:611-639`、探针 `task-list` |
| 19 | 嵌套列表 / 紧凑宽松 | `ListItem.spread`（mdast），列表渲染按 item 顺序编号 | `List.Tight` 在列表关闭时确定（探针 `tight-closed`/`loose-closed` 正确区分），间距用 `BlockSpacing`/`ItemSpacing`，嵌套深度决定 bullet 字形 | 无/极低 | 无需处理 | Rust `src/text/node.rs:1590-1600`、`node.rs:1884-1910`；mygo `ui/markdown/view.go:576-610`、`view.go:640-645`、探针 `nested-list` |
| 20 | GFM autolink（www./裸 URL/邮箱） | markdown-rs `gfm_autolink_literal` | `ExtAutolinks`，尾随标点正确剥离。探针 `autolink-punct`：`https://example.com.` 只链接域名，`(https://x.com/a_b)` 含下划线完整，`foo@bar.com.` 去掉句号；`www.example.com/a` 成链接 | 无 | 无需处理 | Rust `markdown.rs:48`；mygo `ui/markdown/parser/autolink.go`、`ui/markdown/parser/document.go:21-23`、探针 `autolink-punct`/`autolink-www` |
| 21 | 引用式链接 / 定义 | `Definition` 渲染为空；`LinkReference` 渲染时用 `identifier` 查 `node_cx.link_refs` 拿 url | 定义不是块（无 Definition kind），解析期即解析引用；未定义引用保持原文。探针 `link-ref` 全部成 link，`link-ref-missing` 全部保持文本 | 无 | 无需处理 | Rust `src/text/format/markdown.rs:217-236`、`markdown.rs:475-495`、`src/text/node.rs:1230-1235`、`node.rs:1916`；mygo `ui/markdown/parser/inline.go`、探针 `link-ref`/`link-ref-missing` |
| 22 | 删除线 / CJK 强调 | GFM `~~`；CommonMark 强调规则（`*a*b` 得 emphasis `a` 加文本 `b`） | 同规则。探针 `cjk-emphasis`：`中文*斜体*中文` 得 emphasis，`中文**粗**中文` 得 strong，`*a*b` 得 emphasis 加文本 | 无 | 无需处理 | mygo `ui/markdown/parser/cjk_emphasis_test.go`、探针 `cjk-emphasis` |
| 23 | 硬换行（行尾两空格 / 反斜杠） | `parse_paragraph` 里没有 `Node::Break` 分支，行内硬换行被丢弃，前后文本直接拼接（`a` + 反斜杠换行 + `b` 得 `ab`） | `LineBreak` 转 `\n`。探针 `hardbreak-backslash`/`hardbreak-spaces` 均得 `line break` 行内节点 | 中：Rust 是缺陷，mygo 更正确；做 Rust 回归对比时会看到差异 | 可接受差异（改进），无需补 | Rust `src/text/format/markdown.rs:77-235`（无 Break 分支）、`markdown.rs:331-334`（仅块级）；mygo `ui/markdown/spans.go:268-270`、探针 `hardbreak-*` |
| 24 | 软换行 | mdast 约定软换行保留为 Text 内的 `\n`，peekdown 未做替换，段落按源码折行渲染（本次未在 Rust 侧构建验证，属推断） | `SoftBreak` 转空格，段落按视口宽度重排。探针可见 `soft break` 与 `line break` 区分 | 低-中：段落换行位置不同，长文观感差异 | 可接受差异；若需一致再评估（不建议为此改 mygo） | Rust `src/text/format/markdown.rs:114-117`；mygo `ui/markdown/spans.go:266-267` |
| 25 | 标题 slug | `slugify`：小写 -> 空白转 `-` -> 只保留字母数字和 `-` -> `trim_matches('-')`；`heading_map` 对重复标题追加 `-1`/`-2`；锚点查找用原样 key | `markdown.Slug`：小写 + `TrimSpace` -> 空格转 `-` -> 保留 `-`、`_`、字母数字（不 trim、不去重）；`headingNamed` 把传入名 `ToLower` 后比较 | 中：`_` 保留与否、制表符等其它空白、首尾 `-`、重复标题 `#a-1` 行为不同 | 建议补：TOC/外部锚点统一用 `markdown.Slug` 且 app 侧自行去重；文档内 `#foo_bar` 在 mygo 能跳、Rust 不能（属改进） | Rust `src/text/node.rs:100-108`、`src/text/document.rs:49-88`、`src/text/state.rs:280-288`；mygo `ui/markdown/view.go:545-560`、`view.go:533-543`、探针 `SLUG *` |
| 26 | HTML 注释 | 注释节点整体丢弃 | 属 HTMLBlock（type 2），原样显示 `<!-- hi -->`。探针 `comment` | 中：注释露在正文里 | 必须补（归到第 3 项，预处理整块删除） | Rust `src/text/format/html.rs:640-642`；mygo `ui/markdown/parser/scan.go:198-214`、`ui/markdown/view.go:357-364`、探针 `comment` |

### 2.3 探针实测的 slug 对照（Rust 列为按 `slugify` 代码推演）

| 输入 | Rust `slugify` | mygo `Slug` | 是否一致 |
|---|---|---|---|
| `Hello, World!` | `hello-world` | `hello-world` | 是 |
| `API v2 --- notes` | `api-v2-----notes` | `api-v2-----notes` | 是 |
| `foo_bar-baz` | `foobarbaz` | `foo_bar-baz` | 否（`_`） |
| `a  b` | `a--b` | `a--b` | 是 |
| `  pad  ` | `pad` | `pad` | 是 |
| `中文标题` | `中文标题` | `中文标题` | 是 |
| `C++ guide` | `c-guide` | `c-guide` | 是 |
| `a.b.c` | `abc` | `abc` | 是 |
| `100%` | `100` | `100` | 是 |
| `a` + 制表符 + `b` | `a-b` | `ab` | 否（空白类别） |
| `Ünïcode` | `ünïcode` | `ünïcode` | 是 |
| 重复标题 `# A` 两次 | `a`、`a-1` | `a`、`a`（不去重） | 否（去重） |

## 3. 附录 A：app 侧预处理 / 补丁清单

### A.1 必须补（不补就是功能回归）

1. frontmatter 剥离与渲染（主表 1、2）
   - 交给 `markdown.View` 之前检测首行 `---`（YAML）或 `+++`（TOML），剥离到闭合行；用占位块（配合 `Options.Block`）或视图外的独立元素渲染键值卡片。
   - 关键约束：`Block.Start/End` 是源码字节偏移，选中、复制、滚动定位都依赖它。建议把 frontmatter 区域替换成等长空白行而不是删除，再用 `Options.Block` 在该偏移区间画卡片。
   - 参考 Rust 语义：YAML 顶层非 mapping（如 `[a,b]`）视为解析失败，退化成 `yml` 代码块；空 frontmatter（`---` 紧跟 `---`）不渲染。
2. HTML 块处理（主表 3、26）
   - 必做最小集：`<!-- ... -->` 整块删除；`<img ...>` 转成 Markdown 图片（或用 `Options.Block` 自绘，保留 width/height/style 百分比）；`<br>`、`<br/>` 转硬换行；`<p>`、`<div>`、`<center>`、`<p align="...">` 去壳保留内容。
   - 次最小集：`<details>`/`<summary>`（可折叠）、`<table>`（Rust 会渲染成真表格）、`<h1>`-`<h6>`（转 ATX 标题，slug 需一致）。
   - `<style>`/`<script>` 整块丢弃（Rust 也丢）。
3. 行内 `<img>` / `<a>`（主表 11）
   - `<a href="x">text</a>` 转 `[text](x)`；`<img src="x" alt="y" width="200">` 单独成行时按块级图片处理。
4. 混排图片 / 徽章行（主表 12）
   - 识别"整行只由链接包图片或图片组成"的段落，拆成独立图片段落，否则 README 徽章全部变 `[alt]`。
5. 数学开关决策（主表 16）
   - 方案 A（严格对齐 Rust）：`Extensions: parser.GFM | parser.ExtFootnotes`，`$$` 保持文本。
   - 方案 B（保留增强）：用默认 `Extensions`，同时必须接线 `Options.Math`（块级）和 `Options.InlineMath`（行内），否则数学以等宽 TeX 出现，比 Rust 更难看。
   - 无论哪个方案，都不要开 `parser.ExtDollarMath`（金额误判风险）。
6. 图片加载接线（主表 13）
   - `Options.LoadImage = markdown.FileImages(filepath.Dir(docPath))`；切换文档要换 `LoadImage`。`Options.images` 是 `sync.Map` 且按 url 键缓存，跨文档同名相对路径会串图，建议按文档路径加前缀或切换文档时重建 `Options`。
7. 锚点 / TOC slug 统一（主表 25）
   - 内部锚点、侧边栏 TOC、`Options.Anchor` 一律用 `markdown.Slug`；重复标题由 app 侧加 `-1`/`-2` 去重（mygo 内部不去重，`headingNamed` 命中的是第一个同名标题）。

### A.2 建议补（体验对齐，不做也能用）

1. 脚注编号顺序（主表 15）：若验收要求与 GFM 一致，按"首次引用顺序"重排；当前 mygo 按定义顺序。
2. 行内未知 HTML 标签（主表 11）：`<span class="x">text</span>` 建议剥标签保内容，避免正文出现尖括号源码。
3. `Options.OnLink` 内的 `#anchor`（主表 14）：设了 `OnLink` 后 mygo 不再自动滚动锚点，必须自己处理 `strings.HasPrefix(dest, "#")` 并设置 `Options.Anchor`。
4. 代码块 info string 的额外信息（主表 4）：`{1,3}` 行高亮、`title=` 可做增强。
5. 表格列宽手感（主表 9）：Rust 表格整体偏窄（`ESTIMATED_CHAR_WIDTH_PX = 9.0` 加 `max_w`），mygo 按内容实测且可横向滚动；如需贴近 Rust，调小 `Theme.TableMaxColumn`。
6. 软换行渲染（主表 24）：若希望保留源码折行（Rust 观感），需上游提供选项，不建议 app 侧 hack。

### A.3 可接受差异（无需处理，验收时标注）

1. 硬换行（主表 23）：mygo 正确，Rust 丢 `Break`。
2. 缩进代码块 / 未闭合围栏（主表 5、6）：行为等价，mygo 额外提供 `Open` 信号。
3. 表格对齐、单元格转义、缺列多列（主表 7、8、10）：一致。
4. 任务列表、嵌套列表、紧凑宽松列表（主表 18、19）：语义一致，仅间距与图标细节不同。
5. GFM autolink、引用式链接、删除线、CJK 强调（主表 20、21、22）：一致。
6. 单 `$` 数学（主表 17）：默认都不启用，一致。
7. 行内格式化标签子集（主表 11 的 `b/i/em/strong/code/del/s/mark/sub/sup/kbd/samp`）：两边都能生效（Rust 走 html5ever，mygo 走白名单）。
8. 标题 slug 中的 CJK：两边都保留 CJK 字符。

## 4. 附录 B：探针

位置 `C:/tmp/mdprobe`（`go.mod` 用 `replace github.com/egoist/mygo => C:/src/go/mygo`）。核心是把 `parser.Document` 打成块树加行内树：

- `show(name, src, ext)` 逐块打印 `Kind/Parent/Open/Task/Tight/Fenced/Info/Align/Number/Marker/Label/Lines/Inlines`。
- `sub(in, from, to)` 递归打印行内树。注意必须传原始切片加区间，不能传子切片，否则子节点的 `Next` 是绝对下标，会越界 panic。
- 覆盖样例：`frontmatter`、`frontmatter-no-blank`、`frontmatter-toml`、`img-html-block`、`details-html`、`comment`、`html-type6`、`html-inline`、`task-list`、`loose-closed`、`tight-closed`、`nested-list`、`table-cells`、`table-align`、`table-img`、`img-in-text`、`badge`、`footnote-order`、`cjk-emphasis`、`fence-info`、`code-indented`、`unclosed-fence`、`autolink-punct`、`autolink-www`、`link-ref`、`link-ref-missing`、`dollar-default`、`dollar-block`、`dollar-money`、`hardbreak-*`、`SLUG *`。

复跑：`cd C:/tmp/mdprobe && go run .`

## 5. 附录 C：待实测项（本地无法定论）

1. 软换行（主表 24）：Rust 侧是否真的把换行留在 Text 里并按源码折行，需要在 Rust 版构建后目视验证一次（`markdown` crate 1.0.0，mdast 约定如此，但 peekdown 既未替换也未加测试）。
2. Rust 表格 `max_w` 上限下的溢出行为：`ESTIMATED_CHAR_WIDTH_PX` 估算偏小可能截断长单元格，需目视确认（mygo 是横向滚动）。
3. `<details>` 在 Rust 的可折叠性：Rust 把 `details/summary` 当块级容器渲染子节点，是否真可折叠需实测；mygo 侧反正要预处理。
