# 渲染层：自建渲染器与自建选区模型

**Status**: accepted

在只读依赖 mygo 的前提下（`ui/markdown` 没有搜索高亮 hook，`markdown.View` 整篇渲染，内置选区只覆盖已构建的元素），为了让 Peekdown 同时做到正则搜索高亮、按块虚拟化和跨屏选择，正文渲染采用“导出 parser + ui 原语自建渲染器”，选区由应用自己维护。

## Considered Options

- 复制 `ui/markdown` 到 app 内改造：可复用 inline 布局与选择，但要替换 `internal/l10n`、`internal/text` 导入，把整篇渲染改成按块，并把高亮注入最深的 inline 层，之后长期与上游分叉。
- 直接用 `markdown.View` + 源码预处理高亮：最省事，但正则命中跨强调或代码边界会破坏标记，虚拟化后的跨屏选择仍然缺失。

## Decision

- 解析：`github.com/egoist/mygo/ui/markdown/parser`，扩展为 `GFM | ExtFootnotes`；数学语法是否开启由后续决策决定。
- 视图：`ui.List` 按顶层块虚拟化；每个块由应用的块渲染器绘制，标题块同时是锚点目标。
- 行内：`ui.RichText` 的 `Span` 表达样式、代码与命中高亮；链接用 `ui.Link`；行内图片与任务框用内联对象。
- 选区：应用维护“锚点 + 焦点 = 块索引 + 文本偏移”的模型；命中测试用 `ShapeRichText` 的 `TextLayout`；高亮通过切分 span 的 `Background` 绘制；复制直接从文档源文本取值。选区不依赖元素树，因此与虚拟化解耦。
- 代码块：`RichText` 分块渲染，token 来自 `ui/editor` 的语言 tokenizer。
- 表格：自绘网格，单元格用 `RichText`。
- HTML：`golang.org/x/net/html` 解析后转成应用自己的块与行内节点。
- frontmatter：解析前剥离，用等长空白行保持源码偏移；由独立块渲染器呈现。
- Mermaid：独立块渲染器，接入《Mermaid 显示适配》的结论。

## Consequences

- 正向：搜索高亮、按块虚拟化、HTML、frontmatter、mermaid 都可控，不需要 fork mygo。
- 负向：选区、复制、表格、HTML 转换全部要自己实现；mygo 的 markdown 视图改进不会自动继承。
- 风险与验证：自建选区是最大风险。《文档视图》原型必须验证 1 万行文档的跨屏拖选、Shift 点击、Ctrl+A、复制全文、双击选词、锚点跳转和图片；不达标则回退到“复制 `ui/markdown` 并改造”，并重新评估。
- 下游：搜索复用同一套 span 切分；HTML、frontmatter、mermaid 各自接到本 ADR 定义的块渲染器上。
