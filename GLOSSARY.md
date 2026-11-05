# Peekdown

一个用来阅读 Markdown 文档的桌面应用。

## Language

**文档 (Document)**:
用户打开的一个 Markdown 文件，及其内容与元数据。
_Avoid_: 文件、页面

**块 (Block)**:
文档结构中的一个顶层单元，如段落、标题、代码块、表格、引用、列表。
_Avoid_: 节点、元素

**渲染层 (Renderer)**:
把解析后的文档呈现为可读内容的组件。
_Avoid_: 视图层、Markdown 视图

**虚拟化 (Virtualization)**:
只呈现当前可见的文档部分，其余部分在滚动时再构建。
_Avoid_: 懒加载、按需渲染

**选区 (Selection)**:
用户在文档中选定的一段连续文本，由锚点与焦点描述。
_Avoid_: 选择状态、光标

**锚点 (Anchor)**:
选区开始的一端。
_Avoid_: 起点、光标

**焦点 (Caret)**:
选区结束的一端；没有选区时是文本插入位置。
_Avoid_: 光标、游标

**命中 (Match)**:
搜索在文档中找到的一处文本。
_Avoid_: 结果、关键字

**命中高亮 (Match highlight)**:
命中在文档中的标记。
_Avoid_: 选中、搜索结果

**frontmatter**:
文档开头用 YAML 或 TOML 写的元数据块。
_Avoid_: 头部、元信息
