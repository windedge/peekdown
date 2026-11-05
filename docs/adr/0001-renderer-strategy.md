# 渲染层：自建渲染器与自建选区模型

**Status**: accepted（关闭工单 #2）

依据 mygo 源码快照 `v0.7.4-164-g977ea57e`（`C:/src/go/mygo`，2026-11-05）；依赖版本在脚手架工单 #12 固定。

在只读依赖 mygo 的前提下（`ui/markdown` 没有搜索高亮 hook，`markdown.View` 整篇渲染，内置选区只覆盖已构建的元素），为了让 Peekdown 同时做到正则搜索高亮、按块虚拟化和跨屏选择，正文渲染采用“导出 parser + ui 原语自建渲染器”，选区由应用自己维护。

## Considered Options

- 复制 `ui/markdown` 到 app 内改造：可复用 inline 布局、表格与代码块渲染，但选择仍需重写（内置选区随元素树，与按块虚拟化互斥），且要替换 `internal/l10n`、`internal/text` 导入，把整篇渲染改成按块，并把高亮注入最深的 inline 层，之后长期与上游分叉。
- 直接用 `markdown.View` 加 `Options.Block`/`Options.HTML`/`Options.Code` 定制，再加源码预处理高亮：`Options.Block` 能按块接管，但拿不到默认块渲染函数，无法包裹；`View` 内部仍是整篇构建，没有虚拟化，跨屏选区仍缺失。正则命中跨强调或代码边界时，往源码插 `<mark>` 会破坏标记。

## Decision

- 解析：`github.com/egoist/mygo/ui/markdown/parser`，扩展为 `GFM | ExtFootnotes`；数学语法开关见 #17。
- 视图：`ui.List` 按顶层块虚拟化；每个块由应用的块渲染器绘制，标题块同时是锚点目标。单块超大（长代码块、巨型表格）内部不再拆分，横向滚动由块自己处理。
- 行内：`ui.RichText` 的 `Span` 表达样式、代码与命中高亮；链接用 `ui.Link`；行内图片与任务框用内联对象。块级图片由独立块渲染器负责，加载器按文档目录设置，缓存按 URL 与文档作用域隔离。
- 选区：应用维护“锚点 + 焦点 = 块索引 + 文本偏移”的模型；命中测试用 `ShapeRichText` 的 `TextLayout`；高亮通过切分 span 的 `Background` 绘制。复制语义分开：纯文本从渲染文本取值（按块与行内偏移映射剥掉标记），复制 Markdown 取源文本切片，复制 HTML 由后续决策决定。选区不依赖元素树，因此与虚拟化解耦。
- 代码块：`RichText` 分块渲染，token 来自 `ui/editor` 的语言 tokenizer。
- 表格：自绘网格，单元格用 `RichText`。
- HTML：`golang.org/x/net/html` 解析后转成应用自己的块与行内节点。
- frontmatter：解析前剥离，用等长空白行保持源码偏移；由独立块渲染器呈现。
- 锚点：slug 由应用统一生成并去重（Rust 的规则是丢 `_`、空白转 `-`、重复加 `-1`；mygo 的 `Slug` 保留 `_` 且不去重），outline 与锚点共用同一套结果。
- Mermaid：独立块渲染器，接入工单 #4 的结论。
- 数学：语法未开启时按原文显示；开启后也只用等宽文本显示 TeX，由 #17 决定。

## Consequences

- 正向：搜索高亮、按块虚拟化、HTML、frontmatter、mermaid 都可控，不需要 fork mygo。
- 负向：选区、复制、表格、HTML 转换全部要自己实现；mygo 的 markdown 视图改进不会自动继承。
- 风险与验证：自建选区是最大风险。工单 #5 必须验证 1 万行文档的跨屏拖选、Shift 点击、Ctrl+A、复制全文、双击选词、锚点跳转、图片，以及单块超大（5000 行代码块、宽表格）与横向滚动。不达标时回退路线 a 只能换回 inline 布局、表格与代码块渲染；选区在两条路线都要自建，回退不解决跨屏选择。
- 下游：搜索复用同一套 span 切分；HTML、frontmatter、mermaid 各自接到本 ADR 定义的块渲染器上。

## References

- 本决策：[#2](https://github.com/windedge/peekdown/issues/2)
- Mermaid 显示适配：[#4](https://github.com/windedge/peekdown/issues/4)
- 文档视图原型：[#5](https://github.com/windedge/peekdown/issues/5)
- Markdown 语法差异：[#8](https://github.com/windedge/peekdown/issues/8)
- frontmatter：[#14](https://github.com/windedge/peekdown/issues/14)
- 数学公式开关：[#17](https://github.com/windedge/peekdown/issues/17)
