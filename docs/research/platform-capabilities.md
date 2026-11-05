# GitHub issue #11 系统能力核对(文件树 / 项目根 / 图片 / 自动刷新 / 字体)

- 日期: 2026-11-05
- 参考实现: `C:/Seafile/Works/rust/peekdown`(Rust + GPUI, v0.2.0, 只读)
- 目标框架: `C:/src/go/mygo`(commit `977ea57e`, 只读, 只使用导出 API)
- 本文只做研究, 不含产品源码改动

## 1 结论先行

5 项能力里, **mygo 直接现成 2 项半, 图片与字体基本现成但缺 4 个 app 侧环节, 文件树需要"app 侧实现 + 1 个第三方 .gitignore 库", 自动刷新完全现成且语义强于 Rust**。

一句话结论:

1. **文件树**: 树控件(`ui.Outline`, 虚拟化只建可见行)与懒加载所需原语(`os.ReadDir` 按目录读)都已具备; `.gitignore` 是唯一必须引入第三方库的环节; 排序/隐藏点文件/只显示 `.md`/展开状态记忆全部是 app 侧逻辑(约 200 行)。
2. **项目根**: mygo 没有任何 VCS/标记探测 API, 100% app 侧(约 60 行)。Windows 上不需要手写大小写不敏感比较: NTFS 存在性检查本身不区分大小写, 而路径"是否在根下"可用 `filepath.Rel`(`os` 在 Windows 上按 `strings.EqualFold` 比较路径分量)。
3. **图片**: `ui.Image` / `ui.Bitmap` / `ui.SVG` 现成, 本地相对路径解析(`markdown.FileImages`)与 SVG 现成; **http(s)、data URL、失败态、`width`/`height`/`style` 四件事必须 app 侧补**。mygo 的 markdown 解析器不给图片任何尺寸字段, 内联 `<img>` 甚至不当作图片(按原文文本显示)。
4. **自动刷新**: `mygo.WatchFiles` 完全现成, 且比 Rust 的方案强: 给出 `create/write/remove/rename` 的**差异级事件**(Rust 只给"某个根变了"), 默认 50ms 静默去抖 + 500ms 上限(Rust 固定 200ms), Windows 后端就是 `ReadDirectoryChangesW` + IOCP。差异只在限额(`MaxEntries`, 非 macOS 8192)与"消费端不取批次就不再扫描"的合作式节奏。
5. **字体**: `ui.FontFamilies()` + `ui.RegisterFont` + `Theme.Font` 完全够用, 空 family 就是系统默认字体(与 Rust 的 `""` 语义一致)。**中文字体置顶纯 app 侧排序**(mygo 只给 `Name/Monospaced/Registered`, 没有语言/字符集标记)。唯一注意点: 首次 `FontFamilies()` 会枚举系统字体(几十毫秒), 要在非主线程取一次并缓存。

### 1.1 能力矩阵

图例: 现成 = mygo 已提供; app = 需在 peekdown 写业务代码; 库 = 需引入第三方 Go 库。

| # | 能力 | 需要 | mygo 现成 | 需 app 实现 | 需第三方库 |
|---|------|------|-----------|-------------|------------|
| 1.1 | 树控件(缩进/箭头/展开收起/键盘/虚拟滚动) | 是 | `ui.Outline` / `ui.Tree` | 行内容(图标+文件名) | - |
| 1.2 | 目录懒加载 | 是 | `os.ReadDir`(标准库) | 按需读 + 缓存 + 失效 | - |
| 1.3 | 排序(目录优先 / 名称 / 时间) | 是 | `slices.SortFunc` | 比较函数 | - |
| 1.4 | 隐藏点文件 / 只显示 `.md` | 是 | `strings.HasPrefix` / `filepath.Ext` | 过滤规则 | - |
| 1.5 | 尊重 `.gitignore`(嵌套 / 取反 / 目录模式) | 是 | 无 | 逐目录规则栈 | **是**(见 §4) |
| 1.6 | 展开状态记忆 | 是 | `ui.Selection[K]` + `All()` | 序列化到配置 | - |
| 2.1 | VCS 标记探测(`.git/.hg/.svn/CVS` 等) | 是 | 无 | 向上遍历 + `os.Stat` | - |
| 2.2 | 用户可配置标记 | 是 | 无 | 同 2.1 | - |
| 2.3 | Windows 大小写不敏感前缀匹配 | 是 | `filepath.Rel`(分量级 `EqualFold`) | 薄封装 | - |
| 2.4 | UNC / `\\?\` 路径规范化 | 是 | `filepath.Clean` | 前缀剥离(展示用) | - |
| 3.1 | 本地相对路径图片 | 是 | `markdown.FileImages(dir)` | 选目录、per-tab 绑定 | - |
| 3.2 | 位图格式(PNG/JPEG/GIF/WebP/BMP) | 是 | `ui.DecodeBitmap`(+EXIF 方向) | 解码结果缓存 | - |
| 3.3 | SVG 文件 | 是 | `ui.ParseSVG`(不含 text/pattern/filter) | 扩展名分派 | - |
| 3.4 | http(s) 远程图片 | 是 | **无** | 抓取 + `ui.DecodeImage` + `netproxy` | -(`net/http`) |
| 3.5 | data URL | 是 | **无** | base64 解码 + `ui.DecodeImage` | - |
| 3.6 | 失败态 / 重试 | 是 | 只显示 alt 占位, 不重试 | 维护错误状态, 重建 Options | - |
| 3.7 | `width`/`height`/`style` 属性 | 是 | `Width`/`WidthPercent`/`Height`/`AspectRatio` | HTML 属性解析 + `Options.Block` 覆盖 | - |
| 4.1 | 目录级递归监听 | 是 | `WatchFiles{Recursive:true}` | 根注册/注销 | - |
| 4.2 | 文件级监听(引用计数) | 是 | 文件经其目录监听 | 计数表 | - |
| 4.3 | 去抖/合并 | 是 | `Debounce` 50ms / `MaxDelay` 500ms | 调参 | - |
| 4.4 | 差异提示(哪个文件怎么变) | 是 | `Op/Path/OldPath/IsDir` | 事件到 tab/目录的映射 | - |
| 4.5 | Windows 后端 | 是 | `ReadDirectoryChangesW` + IOCP | - | - |
| 5.1 | 系统字体列表 | 是 | `ui.FontFamilies()` | 非主线程取一次 + 缓存 | - |
| 5.2 | 中文字体置顶 | 是 | 无(仅 `Name`) | 置顶名单 + 存在性过滤 | - |
| 5.3 | 嵌入字体 | 可选 | `ui.RegisterFont(data, family)` | `go:embed` | - |
| 5.4 | 空 family = 系统默认 | 是 | 是(`""` 即系统 UI 字体) | 下拉首项显示 "System Default" | - |
| 5.5 | 字号设置 | 是 | `Theme.FontSize` / `FontSize()` | 滑块 + 持久化 | - |

### 1.2 缺口清单

| ID | 缺口 | 影响 | 归属 | 工作量估计 |
|----|------|------|------|-----------|
| G1 | `.gitignore` 匹配(嵌套/取反/目录模式) | 文件树显示范围与 Rust 不一致 | 第三方库 + app | 库接入 1 天, 规则栈 0.5 天 |
| G2 | 图片 http(s) 加载 | 远程图片只剩 alt 文本 | app | 0.5 天(含 3 处缓存/超时) |
| G3 | 图片 data URL 加载 | 内联 base64 图片不显示 | app | 0.5 小时 |
| G4 | 图片失败态与重试 | 一次失败永久占位, 无提示 | app | 0.5 天(需 per-doc Options) |
| G5 | `width`/`height`/`style` 属性 | 图片尺寸控制丢失 | app(`Options.Block`) | 1 天 |
| G6 | 内联 `<img ...>` 完全不被识别 | 原文按文本显示 | app(`Options.Block`) | 与 G5 同一处 |
| G7 | 项目根探测 | 无 VCS/标记感知 | app | 0.5 天 |
| G8 | `WatchFiles` 8192 条目上限 | 大仓库 watch 直接失败 | app(`Ignore` 剪枝) | 0.5 天 |
| G9 | 中文字体置顶 + 字体列表开销 | 首次打开设置可能卡顿; 顺序不理想 | app | 0.5 天 |
| G10 | 多文档相对图片路径缓存串味 | 不同目录的同名相对路径共享缓存 | app(per-tab Options) | 0.5 天 |

## 2 逐项分析

### 2.1 文件树

#### Rust 现状(证据)

- 懒加载: `build_tree_lazy(dir, depth, expanded_dirs, sort_mode)` 每次只读一层, 只对 `expanded` 目录递归; 在 `smol::unblock` 后台线程执行, 回到主线程前用 `refresh_seq` 丢弃过期结果 (`src/workspace/file_explorer.rs:316`, `:899-1004`)。子树展开走 `refresh_subtree`, 用 `subtree_refresh_seq` 做逐目录防抖 (`:380-427`)。
- `.gitignore`: 用 `ignore::WalkBuilder::new(dir).max_depth(Some(1)).standard_filters(true).hidden(false)` (`:909-912`, `:1009-1012`)。即标准过滤全开: 逐目录 `.gitignore` + `.ignore` + 全局 `core.excludesFile` + `.git/info/exclude`; `hidden(false)` 关掉 ignore crate 自带的隐藏文件过滤, 由 app 自己判 `name.starts_with('.')` (`:966`, `:1020`)。
- 排序: 目录优先, 同类型内按 `ExplorerSortMode`(NameAsc/NameDesc 用 `to_lowercase()`, TimeAsc/TimeDesc 用 `modified()`, None 排后) (`:926-960`)。
- 只显示 `.md`: `path.extension() == Some("md")`, **区分大小写**, 只认小写 `md` (`:991`); 而 watcher 侧的 `is_markdown` 却是 `eq_ignore_ascii_case` 且认 `.markdown` (`src/file_watcher.rs:352-358`) - 两处不一致, 重写时应统一。
- `has_relevant_children`: 为决定箭头是否显示, 又对该目录整体扫描一次 (`:1008-1035`), 每个目录每帧刷新都要扫两遍, 这是 Rust 侧的性能热点。
- 展开状态: `expanded_dirs: HashSet<PathBuf>` (`:107`), 折叠时直接从 `entries` 里 `drain` 掉子行 (`:355-368`), 变更通过 `on_expanded_change` 上报, 以"相对根"的形式存进配置 `appearance.expanded_dirs` (`src/workspace/mod.rs:1135-1141`, `src/state/config.rs:124-126`)。
- 依赖版本: `ignore = "0.4"`(锁到 0.4.25), `notify = "6.1"`(锁到 6.1.1) (`Cargo.toml:47`, `:25`)。

#### Go 侧怎么实现懒加载

- `os.ReadDir(dir)` 足够: 每次只读一层, 天然懒加载; `filepath.WalkDir` 是**全树**遍历(且每个目录都要先整层读完再下降), 只适合一次性扫描(搜索、大纲), 不适合展开式文件树。需要剪枝时用 `fs.SkipDir`, 但依然无法"展开到哪读到哪"。
- 每个目录一个节点缓存: `map[string]*dirNode{ entries []entry, err error, loaded bool }`。`ui.Outline` 的 `children` 回调在**主线程构建视图时**被调用, 只对被打开(可见)的节点调用, 因此直接在里面做一次 `os.ReadDir` + 排序即可; 单目录读取在本地磁盘是微秒到毫秒级。
- 若担心网络盘/大目录卡顿(与 Rust 的 `smol::unblock` 对齐), 就在这里返回"加载中"占位行, 起 goroutine 读目录, 完成后 `win.Update(...)` 重建节点并 `Invalidate`; 配合每目录一个 `gen uint64` 丢弃过期结果(等价于 Rust 的 `refresh_seq`)。
- 排序与过滤: `slices.SortFunc` + `strings.ToLower` 做名称比较; 目录优先; 时间用 `DirEntry.Info().ModTime()`。Windows 上 `os.ReadDir` 的 `DirEntry` 内部持有 `Win32finddata`, `Info()` 通常不再发起系统调用(实现时应加一条基准测试确认)。
- 遍历安全: `DirEntry.Type()&fs.ModeSymlink != 0` 时当叶子处理(不下降), 与 Rust(ignore crate 默认不跟随符号链接)一致; 目录再解析(junction)风险由 `os.Root` 之外的判断承担, 至少不要让手工输入路径越过根。
- `filepath.WalkDir` 仍有用: 供"搜索整个项目"(Rust `search_bar` + outline)使用, 配 `fs.SkipDir` 剪 `.git`、忽略目录。
- 长路径/UNC: Go 的 `os` 在 Windows 上自动给绝对路径加 `\\?\` 长路径前缀, 展示与持久化时要剥掉(`strings.TrimPrefix(p, "\\?\")`), 对应 Rust 的 `normalize_unc_path` (`src/workspace/mod.rs:4-35`)。

#### `.gitignore` 库选型

详见 §4。语义要求(来自 git 文档, 也是 Rust `ignore` crate 的行为):

1. 逐目录生效: 每个 `.gitignore` 的规则相对它所在目录(domain)。
2. 优先级: 越深的文件优先级越高; 同一文件内后行覆盖前行; `!` 取反可以重新包含。
3. 目录模式: 以 `/` 结尾只匹配目录, 一旦目录被忽略, 其子孙不得被重新包含(git 不会进入被忽略目录)。
4. `**` 语义: `**/foo`、`abc/**`、`a/**/b`。
5. 附带来源: `.git/info/exclude`、全局 `core.excludesFile`(Rust 通过 `standard_filters(true)` 都启用了)。

"是否要调 `git check-ignore`": **不要放进热路径**。理由: 每次进程启动在 Windows 上是 10-30ms 级; 需要 git 已安装; 只在工作树内有效(用户可能打开非仓库目录); 目录列表要按批喂 `--stdin -z` 才勉强可用, 但仍比内存匹配慢一到两个数量级。**建议只用于测试对拍**(见 §6)。

#### Go 实现要点(文件树)

- 每个目录条目只需三种状态行: `dir(open, hasChildren)` / `file`; `hasChildren` 由"读到该目录时是否至少有一个可见子项"决定, 展开时再真正读 - 这样天然消除 Rust 的 `has_relevant_children` 双扫热点(代价是未展开的空目录仍显示箭头, 可用"读一次并缓存的浅探测"折中)。
- 展开状态持久化: `ui.OutlineState.Open`(`Selection[string]`)本身就是展开集合, 用 `All()` 取出相对根路径写入配置, 启动时 `Add` 回去; 与 Rust 的 `expanded_dirs` 语义等价, 且 Outline 的 key 是路径字符串, 天然在刷新后保留(`ui/outline.go:19-37`, `ui/selection.go:51`)。

### 2.2 项目根

#### Rust 现状(证据)

`WorkspaceView::find_project_root` (`src/workspace/mod.rs:1493-1556`):

- 硬编码 bottom-up VCS: `[".git", ".hg", ".pijul", "_darcs", ".bzr", ".jj"]`, 取**最近的一个**;
- 硬编码 recurring VCS: `[".svn", "CVS"]`, 语义特殊: 从顶向下看连续命中的最底一个(即"最外层仓库的子树根");
- 用户标记(默认 `Cargo.toml, package.json, pyproject.toml, go.mod, pom.xml, build.gradle, build.gradle.kts, CMakeLists.txt`, `src/state/config.rs:197-211`): 自底向上遍历中**最后**命中的那个, 即最外层(top-down outermost);
- 优先级: bottom-up VCS > recurring VCS > 用户标记 (`:1550`)。
- 标记存在性用 `current.join(marker).exists()`, 因此在 Windows 上大小写不敏感由文件系统自动满足; 手写的大小写不敏感比较只用在**路径前缀**判断(`path_starts_with_ci`, `src/file_watcher.rs:381-407`)。

#### Go 实现要点

- 向上遍历: `for d := start; ; { ...; parent := filepath.Dir(d); if parent == d { break }; d = parent }`(驱动器/UNC 根处 `Dir(d)==d`)。
- 存在性: `os.Stat(filepath.Join(d, marker))`, 错误一律视为不存在。`.git` 可能是**文件**(worktree/submodule, 内容是 `gitdir: ...`), `os.Stat` 已覆盖两种形态, 无需区分。
- 大小写: Windows 上无需额外处理(NTFS 查找不区分大小写); Linux/macOS 保持大小写敏感, 与 Rust 一致。
- "路径是否在根之下": `rel, err := filepath.Rel(root, p)`; `err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))`。Windows 上 `filepath.Rel` 用 `sameWord` = `strings.EqualFold` 比较每个路径分量(Go 1.27 源码 `src/path/filepath/path_windows.go:116`), 所以大小写不敏感是免费的 - 这就是 Rust 手写 `path_starts_with_ci` 的对应物。
- 规范化: 用户配置 `expanded_dirs` 是相对根存的, 根变化时要重算; 路径比较一律先 `filepath.Clean`, 需要真实路径时再 `filepath.EvalSymlinks`(注意 `EvalSymlinks` 对不存在的路径会报错, 与 mygo `WatchFiles` 内部行为一致: `watch.go:198-201`)。
- 展示: 剥 `\\?\` / `\\?\UNC\` 前缀后再显示(对应 Rust `normalize_unc_pathbuf`)。

### 2.3 图片

#### mygo 现成的部分(证据)

- `ui.ImageSource` 只有两种实现: `*ui.Bitmap` 与 `*ui.SVG` (`ui/widgets.go:533-537`); `ui.ImageSize(src)` 取固有尺寸 (`:541-549`)。
- `ui.Bitmap`: `ui.DecodeBitmap` 支持 PNG / JPEG / GIF(首帧)/ WebP / BMP, 并按 EXIF 摆正 JPEG (`ui/bitmap.go:121`, `docs/ui/image.md:20-30`); `ui.NewBitmap(image.Image)` 接任意 Go 图像。
- `ui.SVG`: `ui.ParseSVG` 解析 vector; **不含 text、嵌入图片、pattern、marker、filter** (`ui/svg.go:14-28`, `docs/ui/image.md:122-143`)。`ui.MustParseSVG` 用于 `go:embed`。
- 未知格式: `ui.DecodeImage(data)` 先试位图, 再试 SVG (`ui/widgets.go:551-562`)。
- 尺寸控制: `Width/Height/Size/WidthPercent/HeightPercent/MaxWidthPercent/AspectRatio/Fit/ObjectPosition/Radius/Label/Tooltip` (`ui/element.go:965-1003`, `:1250`, `docs/ui/image.md:97-135`)。

#### 缺口(必须 app 侧补)

1. **http(s)**: `markdown.FileImages(dir)` 明确拒绝带 `://` 的地址与 `data:` 前缀, 直接返回 `os.ErrNotExist` (`ui/markdown/image.go:22-40`); `Options.LoadImage` 是唯一入口, 且 mygo "**不会自己从网络加载任何东西**" (`docs/ui/markdown.md:192-200`)。
2. **data URL**: 同上被拒绝。
3. **失败态**: 加载失败只写进 `imageEntry.err`, 视图侧继续显示 alt 占位; 且 `LoadOrStore` 保证**只尝试一次**, 失败后没有重试路径 (`ui/markdown/image.go:44-70`, `:74-95`)。
4. **缓存键**: 缓存是 `Options.images sync.Map`, 键是**写在文档里的原样 URL**; 多个文档共用一个 `Options` 时, 不同目录下的同名相对路径(`img/a.png`)会互相串味。而且 `Options` 内含 `sync.Map`, 按值复制已用过的 `Options` 是错误用法。**结论: 每个文档(标签页)各新建一个 `Options`(结构体字面量新造, 不是复制)**。
5. **`width`/`height`/`style`**: mygo 的 inline 节点没有任何尺寸字段(`ui/markdown/parser/inline.go:49-68`), 渲染时固定为 `MaxWidthPercent(100)` + 由固有尺寸推 `AspectRatio` (`ui/markdown/image.go:88-95`)。
6. **内联 HTML `<img>`**: 解析器把它当 `HTML` inline, 而视图的 `inlineHTML` 只认 `<br>/<b>/<i>/<code>/<mark>/<sub>/<sup>...`, 其它标签**按文本原样输出** (`ui/markdown/spans.go:450-475`, `:308-320`)。也就是说 `<img src="x.png" width="120">` 会显示成一行尖括号文本。

#### Rust 侧对照(要还原的行为)

- 相对路径: 只在"目标存在"时才转绝对路径, 否则保留原 URL (`src/text/format/markdown.rs:17-45`, `src/text/format/html.rs:20-45`)。
- 属性解析: `width`/`height` 属性优先, 缺的从 `style` 的 `width`/`height` 取; 值以 `%` 结尾按百分比(相对长度), 否则按 px (`src/text/format/html.rs:157-202`)。还可解析 `style` 的 `img` 背景等(`:241-244`)。
- 应用: `img(path 或 url)` + `object_fit(Contain)` + `max_w(relative(1.))` + `max_h(px(2000.))` + 有宽度就 `w(width)`、有高度就 `h(height)` (`src/text/node.rs:1169-1181`)。
- 远程: 无本地路径时直接把 URL 交给 gpui 的 `img(url)`; **app 侧没有缓存、没有失败态**(`src/text/node.rs:1169-1173`)。本次未核实 gpui 0.2 的 `img` 是否真的发起 HTTP 请求(只读仓库内没有相关配置代码), 因此不宣称 Rust 的远程图片行为, 重写时以"mygo 需 app 自己抓取"为准。

#### `width`/`height`/`style` 在 app 侧解析成什么

| Markdown/HTML 输入 | mygo 调用 |
|---|---|
| `width="120"` / `style="width:120px"` | `Width(120)` |
| `style="width:80%"` / `width="80%"` | `WidthPercent(80)` |
| `height="90"` | `Height(90)`; 仅这时不设 `AspectRatio` |
| 只有一边有值 | 设该边 + `AspectRatio(固有 w/h)` |
| 都没有 | `MaxWidthPercent(100)` + `AspectRatio(固有 w/h)`(mygo 默认行为) |
| 任何情况 | 加 `MaxHeight(2000)` 与 `Label(alt)`、有 `title` 加 `Tooltip(title)` |

实现位置:**`Options.Block` 覆盖**(`ui/markdown/view.go:59-61`, `:320-328`)。判断条件与做法:

```go
o.Block = func(c *ui.Context, d *parser.Document, i int) bool {
    b := d.Block(i)
    if b.Kind != parser.Paragraph && b.Kind != parser.HTMLBlock {
        return false
    }
    src := d.Source()[b.Start:b.End]
    if !strings.Contains(src, "<img") {
        return false
    }
    app.renderImageParagraph(c, src) // 自己解析 <img>, 建 ui.Image
    return true
}
```

`d.Block(i).Start/End` 与 `d.Source()` 都是导出 API (`ui/markdown/parser/document.go:106-115`, `:310`), 足以切出原始 HTML 片段。代价: 该段落不走 mygo 的文本选择/复制路径, 需要在 app 侧自己保证可访问性(`.Label(alt)`)与复制行为。

如果不想承担 `Block` 覆盖的成本, 折中方案是**预处理 markdown 源码**: 把 `<img ...>` 改写成等价的 HTML `<div>` + 记录尺寸的旁表, 但这会破坏行号映射(搜索/大纲依赖行号), **不推荐**。

#### 推荐 loader 结构(app 侧)

```
resolve(url, docDir) -> source:
  1. data: URL  -> base64 解码 -> ui.DecodeImage
  2. http(s):// -> netproxy 客户端 GET(超时 10s, 限 8MB) -> ui.DecodeImage -> 内存缓存(按绝对 URL, LRU 64)
  3. 其它       -> 展开为 docDir 下的绝对路径 -> markdown.FileImages(docDir) 或直接 os.ReadFile + 扩展名分派
  4. 失败       -> 返回错误 + 记录失败状态, 视图侧显示"加载失败 + 重试"
```

网络一律用 `&http.Client{Transport: netproxy.Transport(), Timeout: ...}`, 这样才跟随系统代理(WinHTTP/PAC), 与页面行为一致 (`docs/native.md:491-520`, `netproxy/netproxy.go:82-88`)。不要用裸 `http.DefaultClient`。

### 2.4 自动刷新

#### mygo 现成能力(证据)

- API: `mygo.WatchFiles(ctx, path, mygo.FileWatchOptions{...})` 返回 `*mygo.FileWatcher`, 用 `Next(ctx)` 逐批取事件, `Close()` / `Done()` / `Err()` 收尾 (`watch.go:105-360`)。
- 选项与默认值: `Recursive`, `Ignore(path,isDir)`, `Debounce`(0 -> 50ms, 负数 -> 立即), `MaxDelay`(0 -> 500ms, 且不小于 `Debounce`), `MaxEntries`(0 -> 8192; macOS 65536), `HintsOnly` (`watch.go:58-96`, `:176-190`, `watch_entries_other.go`)。取值逻辑在 `:182-189`。
- 事件: `FileCreate/FileWrite/FileRemove/FileRename/FileHint` + `Path`(相对监听根, 斜杠分隔, `.` 表示自身)+ `OldPath` + `IsDir` (`watch.go:16-56`)。
- **语义是差异而不是流水**: 内核维护快照(identity/size/mtime/mode), 收到系统通知后重扫被点名的目录, 用前后差异生成事件; 原子保存(临时文件 rename 覆盖)= 一次 `write`; 元数据 touch、目录 write 不上报 (`docs/native.md:630-650`, `docs/architecture.md:2156-2200`)。
- 合作式节奏: 上一批没被 `Next` 取走之前不再扫描, 所以不会丢事件也不会堆积 (`docs/native.md:645-648`, `watch.go:340-348` 的 `take()`)。
- 文件经其目录监听: 文件被替换 = `write` 的 `.`; 目录被删/改名 = 最后一批有 `.` 的 `remove` 且 `Next` 返回 `io.EOF` (`docs/native.md:653-660`)。
- Windows 后端: `ReadDirectoryChangesW` + IO completion port, 递归监听用 1 个 handle(`Subtree`), 并额外监听父目录以感知自身被改名; 缓冲区溢出(`ERROR_NOTIFY_ENUM_DIR`)时全量重扫而不是停 (`internal/windows/watch.go:27-70`, `:262`, `docs/architecture.md:2196-2200`, `docs/plugins/watch.md:99-105`)。
- 运行约束: 需要 app 已启动(`needsApp`), 不占用主线程, app 退出时统一停止 (`watch.go:168-176`, `:419-436`)。
- 限额: 超过 `MaxEntries` 时以 `mygo.ErrWatchLimit` 停止 (`watch.go:100`)。

#### 与 Rust 对比

| 维度 | Rust (`src/file_watcher.rs`) | mygo (`watch.go`) |
|---|---|---|
| 后端 | `notify::RecommendedWatcher`(Windows 上同为 `ReadDirectoryChangesW`) | 平台自带实现, Windows 同上 |
| 实例数 | 1 个 watcher 多路复用 (`:41-56`) | 每次 `WatchFiles` 一个 watcher(`per watch 一个 goroutine`), 注册几个就几个 |
| 合并窗口 | 固定 200ms 延迟窗 + Condvar 工作线程 (`:23`, `:96-170`) | 50ms 静默去抖 + 500ms 上限, 内核侧自动 |
| 粒度 | 只有 `RootChanged(root)` / `FileModified(path)` 两种 (`:28-36`) | `create/write/remove/rename` + 路径 + `OldPath` + `IsDir` |
| 差异提示 | 无: 只知道"这个根变了", explorer 整树重建 | 有: 可以只失效受影响的目录/tab |
| 目录级 | `RecursiveMode::Recursive` (`:214`) | `Recursive: true` |
| 文件级 | 引用计数 `file_watch_counts` + `NonRecursive` (`:43`, `:237-247`) | 每次 watch 一个对象, 引用计数由 app 做(相同做法) |
| 过滤 | 扫描期用 ignore crate, 事件不过滤; 未匹配根则全刷 (`:300-330`) | `Ignore(path,isDir)`, 相对路径 + 斜杠, 被忽略目录**永不打开**且不计入 `MaxEntries` (`watch_scan.go:255-280`) |
| Access 事件 | 主动丢弃 `EventKind::Access` (`:311-314`) | 内核层已过滤(元数据变化不报) |
| 限额 | 无 | 8192 条目(非 macOS), 超限报错 |
| 生命周期 | `Drop` 时 join 工作线程 (`:79-88`) | `Close()` / context / app 退出 (`:350-360`, `:419-436`) |
| 消费节奏 | 无背压, 队列无界 | 有背压: 不取批次就暂停扫描 |

结论: mygo 的 watch 在语义上**更强**, Rust 的 200ms 合并与"按根刷新"在 Go 侧可以省掉 - 直接按事件路径精确失效即可。需要额外做的只有三件事: 引用计数、把事件搬回主线程、以及用 `Ignore` 规避 8192 上限。

#### Go 侧实现方案

```
type watcher struct {
    mu      sync.Mutex
    root    string                    // explorer 根
    rootW   *mygo.FileWatcher         // 递归 watch
    docs    map[string]*docWatch      // 绝对路径 -> {w, refs}
    dirty   map[string]struct{}       // 待失效的目录(相对根)
}

注册:
  setRoot(root):  关掉旧 rootW; mygo.WatchFiles(ctx, root, {Recursive:true, Ignore: gitignoreMatcher, Debounce: 150ms, MaxDelay: 600ms})
  watchDoc(path): refs++; refs==1 时 mygo.WatchFiles(ctx, path, {Debounce: 100ms})
  unwatchDoc(path): refs--; refs==0 时 Close()

消费(每个 watcher 一个 goroutine):
  for { evs, err := w.Next(ctx); if err != nil { return }
        win.Update(func(){ apply(evs) }) }

apply(evs):
  for ev in evs:
    if ev.Op in {create, remove, rename} or ev.IsDir: markDirty(filepath.Dir(ev.Path)) + 该目录缓存失效
    if ev.Op == write && 是某个 tab 的路径: 重载该 tab(保滚动位置, 失败保留旧内容)
```

要点:

- `Ignore` 里同时剪掉 `.git`、`node_modules` 和 `.gitignore` 命中的目录, 这是避免 `ErrWatchLimit` 的唯一手段; `Ignore` 在 watcher 的 goroutine 上跑, 规则栈必须缓存(`watch.go:64-68`)。
- 事件路径是相对根、斜杠分隔: 直接 `strings.Split(ev.Path, "/")` 喂给 gitignore 匹配器即可(`watch_scan.go:270`)。
- 主线程应用: `win.Update(func(){...})` 或 `ui.Context.Invalidate`; 不要在其他 goroutine 里直接改 UI 状态。
- 关闭顺序: 先 `Close()` rootW 与所有 docWatch, 等 `Done()`, 再丢弃状态, 避免 `win.Update` 撞上已销毁的窗口。
- 如果只是"省电/省 fd"的极端场景, 可以用 `HintsOnly`, 但它只给 `FileHint`(某个目录下"有变化"), 拿不到具体文件, 文档明确不建议用在要紧的地方 (`docs/native.md:670-695`)。文件树+文档重载属于"要紧", 用完整模式。

### 2.5 字体

#### mygo 现成能力(证据)

- `ui.FontFamilies() []ui.FontFamily`, 每项 `{Name string, Monospaced bool, Registered bool}`; 顺序是"app 注册的在前, 系统其后", 组内按小写名排序, 同名去重 (`ui/window.go:41-72`, `internal/text/layout.go:660-712`)。
- 首次调用会真的去枚举系统字体, "有几百个字体时几十毫秒", 之后缓存直到 `RegisterFont`; 任何 goroutine 可调 (`ui/window.go:54-66`, `internal/text/layout.go:665-669`)。
- Windows 实现: DirectWrite 系统集合逐 family 取英文名, 用 `IDWriteFont1` 判断等宽 (`internal/text/dwrite_windows.go:1562-1610`)。
- 空 family 就是系统 UI 字体, 与 Rust 的 `""` 语义一致 (`internal/text/layout.go:18-21` 的 `Style.Family` 注释; `ui/theme.go:73-75` "Font its family (\"\" is the system's)")。
- 多字体回退: `Font("Inter, Noto Sans JP")` 是逗号分隔列表, 前一个缺字用后一个, 仍缺才落到系统选择 (`ui/element.go:1532-1536`, `docs/ui/text.md:270-276`) - 这就是"中文字体裁剪"的正规手段。
- 嵌入字体: `ui.RegisterFont(data []byte, family string)`: TTF/OTF/TTC; family 为空则用字体自带名; 会清空 family 缓存与排版缓存 (`ui/window.go:35-38`, `internal/text/layout.go:636-658`)。
- 语言/字形: `text.System.SetLanguage` 由 UI 宿主按 app 语言自动设置 (`ui/window.go:899-904`), 中文 Han 字形选择不需要 app 干预。
- 设置对话框构件: `ui.Combobox`(输入即过滤的长列表)、`ui.Select`、`ui.Slider`、`ui.Dialog`/`ui.Form` 都在 (`docs/ui/combobox.md`, `docs/ui/select.md`, `docs/ui/slider.md`, `docs/ui/dialog.md`, `docs/ui/form.md`)。

#### Rust 侧做法(要还原的部分)

- 字体列表来源: `cx.text_system().all_font_names()`, 即系统字体名数组 (`src/workspace/settings_dialog.rs:186-188`)。
- 置顶名单: 内容字体 `Microsoft YaHei, Source Han Sans SC, PingFang SC, Noto Sans SC, SimSun, SimHei`; 等宽 `Consolas, Cascadia Code, JetBrains Mono, Fira Code, Source Code Pro, Menlo` (`:18-38`)。
- 列表构造: 首项 `""`(显示为 "System Default")-> 置顶名单中**系统确实存在**的 -> 其余字体(排除已置顶与 `.SystemUIFont`) (`:40-60`, `:62-88`)。
- 保存与应用: 只有非空才覆盖 `theme.font_family` / `theme.mono_font_family`, 字号总是覆盖 (`src/state/config.rs:260-272`); 默认字号 16 / 代码 13 (`:181-192`)。

#### Go 侧做法

- 字体列表: 启动后台 goroutine 调一次 `ui.FontFamilies()` 缓存到 app 状态(`sync.OnceValue` 或 `once.Do`), 打开设置时直接读缓存; 不在视图构建里调用。按需在"设置里点刷新"时再调(对应"新装字体要重启才可见"的现状)。
- 置顶: 用 `slices.IndexFunc` + `strings.EqualFold` 在列表里查置顶名(Windows 上 DirectWrite 给的是英文名, 与 Rust 名单完全对得上), 命中才插入; 首项固定 `""`。
- 中文字体真正生效的两种做法:
  1. 用户只选一个 family(如 `Microsoft YaHei`), 缺字交给系统回退(与 Rust 相同);
  2. app 侧把默认值写成逗号列表 `Microsoft YaHei, Segoe UI`, 或在用户选择后附加 `, system-ui`, 用于"中文优先但西文仍用系统界面字体"的诉求。**建议 1 为默认, 2 作为可选开关**, 避免与 Rust 行为不一致。
- 等宽字体选择: 用 `f.Monospaced` 过滤, 比 Rust 的"手工置顶名单"更准; 名单仍保留在最前面。
- 空 family 落盘: 持久化 `""`, 应用时 `if fam != "" { theme.Font = fam }`, 与 Rust 完全一致。
- 嵌入字体: 只有确实要"自带字体"时才 `go:embed` + `ui.RegisterFont`; 注意它会使字体列表缓存失效(下次 `FontFamilies()` 会重新枚举系统字体), 应集中在启动早期做一次。

## 3 推荐库(带版本 / 许可证 / 维护状态)

数据取自 `proxy.golang.org/@latest` 与 GitHub API, 采集于 2026-11-05。

| 库 | 版本 | 许可证 | 维护状态 | 语义覆盖 | 结论 |
|---|---|---|---|---|---|
| `github.com/go-git/go-git/v5` 的 `plumbing/format/gitignore` | v5.19.3 (2026-10-04) | Apache-2.0 | 活跃 (最近提交 2026-11-04, 7.8k star) | `ParsePattern(p, domain)` + `NewMatcher` 按 domain 区分优先级; 支持 `!`、目录模式(`dirOnly`)、`**`、注释/转义/尾随空格; 嵌套靠 app 传入 domain | **首选** |
| `github.com/denormal/go-gitignore` | v0.0.0-20180930084346 (无 tag, 无 go.mod; 最近提交 2023-11-10) | MIT | 半停更 | 提供 `NewRepository(base)` 直接处理嵌套 `.gitignore` + `.git/info/exclude`, 惰性缓存; 自定义 fnmatch | **备选**(省 app 代码, 但依赖停更且无版本约束) |
| `github.com/bmatcuk/doublestar/v4` | v4.10.4 (2026-10-24) | MIT | 活跃 (722 star) | 只提供 glob(含 `**`), 需自写约 100 行 gitignore 规则编译 | **轻量方案**(零 gitignore 语义, 完全可控) |
| `github.com/sabhiram/go-gitignore` | v0.0.0-20210923224102 (2021-09-23; 最近提交 2024-02-16) | MIT | 半停更 | 正则实现, `MatchesPath` **没有 isDir 参数**(`foo/` 会把同名文件也忽略), 没有 domain 概念(嵌套后相对路径语义错), 取反只在"已经匹配过"时生效 | 不推荐 |
| `github.com/monochromegane/go-gitignore` | v0.0.0-20200626010858 (2020-06-26; 最近提交 2023-05-29) | MIT | 半停更 | 支持 `!`、目录模式、`Match(path, isDir)`; README 明确**不支持 `**`** (用 `filepath.Match`) | 不推荐 |
| `github.com/gobwas/glob` | v1.1.0 (2026-10-26) | MIT | 活跃 | 纯 glob | 与 doublestar 同类, 二选一 |
| `git check-ignore` | 系统 git | - | - | 权威但需 git 已装、仅工作树内有效、进程启动开销大 | 只用于测试对拍 |

### 3.1 为什么首选 go-git 的 gitignore 子包

- 语义最接近 git: 规则以"路径分量数组 + domain"表达, 与"逐目录生效"的模型一一对应; `Matcher.Match` 从新到旧找第一条命中(后者覆盖前者), `dirOnly` 只在末端判断(`plumbing/format/gitignore/pattern.go` 的 `Match`, `matcher.go` 的 `matcher.Match`)。这正是嵌套 `.gitignore` + 取反 + 目录模式需要的三件套。
- `**` 支持: v5 用 `filepath.Match` 逐段匹配(对 `a/**/b`、`**/foo`、`abc/**` 都覆盖, 但含 `**` 的中间段会被判为非法; 与 git 的 wildmatch 有细微差别); **v6 (beta) 已把 git 2.54 的 `wildmatch.c` 移植进来**, 语义几乎完全一致。若将来允许升级, 换 v6 即可, API 形状一致。
- 成本: 引入 `go-git` 模块, 但只编译 `plumbing/format/gitignore` 及其依赖(`go-billy`、`plumbing/format/config`、`internal/pathutil`), 不拉起整个 go-git。
- 我们不用它的 `ReadPatterns`(它要求 `billy.Filesystem` 且是全树遍历, 与懒加载冲突), 只用 `ParsePattern` + `NewMatcher` 两个导出符号, 自己维护逐目录规则栈。

### 3.2 关于 `git check-ignore`

不用在运行时, 但**强烈建议做对拍测试**: 用固定 fixture 仓库, 把同一批路径分别喂给我们的匹配器和 `git check-ignore --stdin -z --no-index`, 断言一致。这样即使库有细微偏差也能在 CI 中暴露, 而不是等用户发现漏文件。

## 4 可直接采用的 Go 代码骨架

以下代码只作为实现蓝本, 不落在产品目录。

### 4.1 懒加载文件树节点 + 排序 + 过滤 + gitignore

```go
package workspace

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
)

type sortMode int

const (
	sortNameAsc sortMode = iota
	sortNameDesc
	sortTimeDesc
	sortTimeAsc
)

// entry 的 rel 是相对根的斜杠路径, 同时作为 ui.Outline 的 item 键。
type entry struct {
	rel   string
	name  string
	isDir bool
	mod   int64 // UnixNano, 0 表示取不到
}

// dirCache 保存每个目录一次读取的结果, 供 ui.Outline 的 children 回调使用。
type dirCache struct {
	root    string
	entries map[string][]entry // 相对根的斜杠路径 -> 子项
	ignore  map[string]gitignore.Matcher
	sort    sortMode
	showAll bool // false: 只显示 .md
}

func (d *dirCache) invalidate(rel string) {
	for p := range d.entries {
		if p == rel || strings.HasPrefix(p, rel+"/") {
			delete(d.entries, p)
		}
	}
	for p := range d.ignore {
		if p == rel || strings.HasPrefix(p, rel+"/") {
			delete(d.ignore, p)
		}
	}
}

// matcher 返回作用于 rel 目录(相对根)的规则集合: 根到 rel 的各级 .gitignore, 由浅到深。
func (d *dirCache) matcher(rel string) gitignore.Matcher {
	if m, ok := d.ignore[rel]; ok {
		return m
	}
	var pats []gitignore.Pattern
	var domain []string
	// .git/info/exclude 相对仓库根, domain 为空; 对应 Rust 的 standard_filters(true)。
	d.addPatterns(&pats, nil, filepath.Join(d.root, ".git", "info", "exclude"))
	if rel != "." && rel != "" {
		d.addPatterns(&pats, nil, filepath.Join(d.root, ".gitignore"))
		cur := ""
		for _, part := range strings.Split(rel, "/") {
			cur = strings.TrimPrefix(cur+"/"+part, "/")
			domain = append(domain, part)
			d.addPatterns(&pats, domain, filepath.Join(d.root, filepath.FromSlash(cur), ".gitignore"))
		}
	} else {
		d.addPatterns(&pats, nil, filepath.Join(d.root, ".gitignore"))
	}
	m := gitignore.NewMatcher(pats)
	if d.ignore == nil {
		d.ignore = map[string]gitignore.Matcher{}
	}
	d.ignore[rel] = m
	return m
}

func (d *dirCache) addPatterns(pats *[]gitignore.Pattern, domain []string, file string) {
	b, err := os.ReadFile(file)
	if err != nil {
		return
	}
	for line := range strings.SplitSeq(string(b), "\n") {
		line = strings.TrimSuffix(line, "\r")
		if s := strings.TrimSpace(line); s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		*pats = append(*pats, gitignore.ParsePattern(line, domain))
	}
}

// children 是 ui.Outline 的 children 回调: 只读被展开的目录, 返回子项的相对路径。
func (d *dirCache) children(rel string) []string {
	if got, ok := d.entries[rel]; ok {
		return keys(got)
	}
	dir := d.root
	base := rel
	if rel == "." || rel == "" {
		base = "."
	} else {
		dir = filepath.Join(d.root, filepath.FromSlash(rel))
	}
	des, err := os.ReadDir(dir)
	if err != nil {
		d.entries[rel] = nil
		return nil
	}
	m := d.matcher(rel)
	out := make([]entry, 0, len(des))
	for _, de := range des {
		name := de.Name()
		if strings.HasPrefix(name, ".") { // 与 Rust 一致: 跳过点文件
			continue
		}
		if de.Type()&fs.ModeSymlink != 0 { // 不跟随符号链接
			continue
		}
		isDir := de.IsDir()
		childRel := name
		if base != "." {
			childRel = base + "/" + name
		}
		var parts []string
		if base != "." {
			parts = append(strings.Split(base, "/"), name)
		} else {
			parts = []string{name}
		}
		if m.Match(parts, isDir) {
			continue
		}
		if !isDir && !d.showAll && !strings.EqualFold(filepath.Ext(name), ".md") {
			continue
		}
		e := entry{rel: childRel, name: name, isDir: isDir}
		if d.sort != sortNameAsc && d.sort != sortNameDesc {
			if info, err := de.Info(); err == nil {
				e.mod = info.ModTime().UnixNano()
			}
		}
		out = append(out, e)
	}
	slices.SortFunc(out, func(a, b entry) int {
		if a.isDir != b.isDir {
			if a.isDir {
				return -1
			}
			return 1
		}
		switch d.sort {
		case sortTimeDesc:
			return cmp.Compare(b.mod, a.mod)
		case sortTimeAsc:
			return cmp.Compare(a.mod, b.mod)
		case sortNameDesc:
			return cmp.Or(strings.Compare(strings.ToLower(b.name), strings.ToLower(a.name)), strings.Compare(b.name, a.name))
		}
		return cmp.Or(strings.Compare(strings.ToLower(a.name), strings.ToLower(b.name)), strings.Compare(a.name, b.name))
	})
	d.entries[rel] = out
	return keys(out)
}

func keys(in []entry) []string {
	out := make([]string, len(in))
	for i, e := range in {
		out[i] = e.rel
	}
	return out
}
```

`ui.Outline` 侧(展开状态记忆):

```go
ui.Outline(c, &app.files, []string{"."}, func(rel string) []string {
	return app.tree.children(rel)
}, func(rel string) {
	ui.Text(c, filepath.Base(rel)).SingleLine()
}).Grow(1)
// 持久化: app.files.Open.All() 里换成相对根的字符串写进配置
```

### 4.2 项目根

```go
var (
	vcsBottomUp = []string{".git", ".hg", ".pijul", "_darcs", ".bzr", ".jj"}
	vcsRecur    = []string{".svn", "CVS"}
)

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

// findProjectRoot 复刻 Rust: bottom-up VCS > 连续 VCS > 用户标记(最外层)。
func findProjectRoot(start string, markers []string) (string, bool) {
	var bottomUp, topDown string
	var recur []string
	for d := filepath.Clean(start); ; {
		for _, m := range vcsBottomUp {
			if bottomUp == "" && exists(filepath.Join(d, m)) {
				bottomUp = d
			}
		}
		for _, m := range vcsRecur {
			if exists(filepath.Join(d, m)) {
				recur = append(recur, d)
				break
			}
		}
		for _, m := range markers {
			if m = strings.TrimSpace(m); m != "" && exists(filepath.Join(d, m)) {
				topDown = d
				break
			}
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	if bottomUp != "" {
		return bottomUp, true
	}
	if n := len(recur); n > 0 {
		slices.Reverse(recur) // 自顶向下
		best := recur[0]
		for i := 1; i < n; i++ {
			if filepath.Dir(recur[i]) != recur[i-1] {
				break
			}
			best = recur[i]
		}
		return best, true
	}
	return topDown, topDown != ""
}

// underRoot 判断 p 是否在 root 之下; Windows 上 filepath.Rel 逐分量 EqualFold。
func underRoot(root, p string) (string, bool) {
	rel, err := filepath.Rel(root, p)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return rel, true
}
```

### 4.3 图片 loader(本地 / http(s) / data URL)

```go
type imageLoader struct {
	dir    string // 文档所在目录
	client *http.Client
	cache  map[string]ui.ImageSource // 仅远程/data; 本地交给 markdown.FileImages 自己的去重
	files  func(string) (ui.ImageSource, error)
}

func newImageLoader(docDir string) *imageLoader {
	return &imageLoader{
		dir:    docDir,
		client: &http.Client{Transport: netproxy.Transport(), Timeout: 10 * time.Second},
		files:  markdown.FileImages(docDir),
	}
}

func (l *imageLoader) load(url string) (ui.ImageSource, error) {
	switch {
	case strings.HasPrefix(url, "data:"):
		return decodeDataURL(url)
	case strings.HasPrefix(url, "http://"), strings.HasPrefix(url, "https://"):
		if v, ok := l.cache.Get(url); ok {
			return v, nil
		}
		src, err := l.fetch(url)
		if err != nil {
			return nil, err
		}
		l.cache.Add(url, src)
		return src, nil
	default:
		return l.files(url)
	}
}

func (l *imageLoader) fetch(url string) (ui.ImageSource, error) {
	resp, err := l.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("image %s: %s", url, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	return ui.DecodeImage(data) // 位图或 SVG 都能吃
}

func decodeDataURL(u string) (ui.ImageSource, error) {
	i := strings.IndexByte(u, ',')
	if i < 0 {
		return nil, errors.New("bad data URL")
	}
	meta, payload := u[5:i], u[i+1:]
	var data []byte
	if strings.Contains(meta, ";base64") {
		b, err := base64.StdEncoding.DecodeString(payload)
		if err != nil {
			return nil, err
		}
		data = b
	} else {
		s, err := url.QueryUnescape(payload)
		if err != nil {
			return nil, err
		}
		data = []byte(s)
	}
	return ui.DecodeImage(data)
}
```

每个标签页新建 `markdown.Options`(不能复制已使用的结构体, 内含 `sync.Map`):

```go
app.md = &markdown.Options{
	Theme:     app.mdTheme,
	LoadImage: loader.load,
	Block:     app.imageBlock, // §2.3 的 <img width/height/style> 覆盖
	OnLink:    app.openLink,
	HTML:      app.rawHTML,
}
```

### 4.4 watcher

```go
type docWatch struct {
	w    *mygo.FileWatcher
	refs int
}

type watcher struct {
	mu   sync.Mutex
	root string
	rootW *mygo.FileWatcher
	docs map[string]*docWatch
	ctx  context.Context
}

func (m *watcher) setRoot(root string, ignore func(string, bool) bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.rootW != nil {
		m.rootW.Close()
		<-m.rootW.Done()
		m.rootW = nil
	}
	w, err := mygo.WatchFiles(m.ctx, root, mygo.FileWatchOptions{
		Recursive: true,
		Ignore:    ignore, // 剪 .git / node_modules / .gitignore 命中项
		Debounce:  150 * time.Millisecond,
		MaxDelay:  600 * time.Millisecond,
	})
	if err != nil {
		return err
	}
	m.root, m.rootW = root, w
	go m.pump(w, false, "")
	return nil
}

func (m *watcher) watchDoc(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if d := m.docs[path]; d != nil {
		d.refs++
		return nil
	}
	w, err := mygo.WatchFiles(m.ctx, path, mygo.FileWatchOptions{Debounce: 100 * time.Millisecond})
	if err != nil {
		return err
	}
	m.docs[path] = &docWatch{w: w, refs: 1}
	go m.pump(w, true, path)
	return nil
}

func (m *watcher) pump(w *mygo.FileWatcher, isDoc bool, path string) {
	for {
		evs, err := w.Next(m.ctx)
		if err != nil {
			return
		}
		evs := evs
		m.win.Update(func() {
			if isDoc {
				m.onDocChange(path, evs)
			} else {
				m.onRootChange(evs)
			}
		})
	}
}
```

### 4.5 字体列表

```go
var fontList = sync.OnceValue(func() []ui.FontFamily { return ui.FontFamilies() })

var (
	pinnedContent = []string{"Microsoft YaHei", "Source Han Sans SC", "PingFang SC", "Noto Sans SC", "SimSun", "SimHei"}
	pinnedMono    = []string{"Consolas", "Cascadia Code", "JetBrains Mono", "Fira Code", "Source Code Pro", "Menlo"}
)

// buildFontList: "" 作为 "System Default" 首项, 置顶名单中系统确实存在的其次, 其余按原顺序。
func buildFontList(all []ui.FontFamily, pinned []string, mono bool) []string {
	out := []string{""}
	used := map[string]bool{}
	for _, name := range pinned {
		for _, f := range all {
			if strings.EqualFold(f.Name, name) && (!mono || f.Monospaced) {
				out = append(out, f.Name)
				used[strings.ToLower(f.Name)] = true
				break
			}
		}
	}
	for _, f := range all {
		if used[strings.ToLower(f.Name)] {
			continue
		}
		if mono && !f.Monospaced {
			continue
		}
		out = append(out, f.Name)
	}
	return out
}
```

界面上用 `ui.Combobox(c, &app.fontFamily, options).Label("Content Font")`(可输入过滤), `""` 项显示为文本 "System Default"。

## 5 验证方法

1. **gitignore 对拍**: fixture 仓库(嵌套 `.gitignore` + `!` 取反 + `dir/` + `a/**/b` + `**/foo`), 用 `git check-ignore --stdin -z --no-index` 的出结论与 `dirCache.matcher` 的结论逐条比对, 作为单测。
2. **watch 端到端**: mygo 自带 `internal/e2e/watch_test.go` 可作为行为参照(事件 op/去抖/EOF 语义); 自己的测试只需覆盖"事件到 UI 失效"的映射: 建/删/改名文件 -> 只失效对应目录; 写入打开的文档 -> 恰好重载一次。
3. **字体**: 在 Windows 上断言 `FontFamilies()` 含 `Microsoft YaHei` 与 `Consolas` 且 `Monospaced` 为 true; 断言 `RegisterFont` 之后列表首部出现注册名。
4. **图片**: 四类 URL(相对 / 绝对路径 / data / http)各一张, 加一张坏图, 校验: 显示、尺寸属性生效、失败态可重试、切换两个目录下同名 `img/a.png` 不串图。

## 6 风险与取舍

- **`Options.Block` 覆盖图片段落**的代价是丢掉 mygo 的文本选择/复制/行内排版。若产品接受"HTML 内联图片不支持尺寸属性", 可以只做 G2/G3/G4 而跳过 G5/G6, 成本减半。
- **`Options` per-tab** 会让"一份全局 markdown 主题与选项"变成"每 tab 一份", 改主题时要逐 tab 更新(或让 `Theme` 指针共享, 只让 `Options` 结构体各自独立)。
- **`MaxEntries` 8192** 是硬约束: 打开一个 node_modules 齐全的前端仓库, 不做 `Ignore` 剪枝必然失败; 剪枝后仍超限时应降级为"不监听 explorer 根, 只监听打开的文档"(功能退化但可用)。
- **go-git v5 的 `**` 与 git wildmatch 有细微差异**(v5 用 `filepath.Match` 逐段; v6 beta 才移植 wildmatch)。若对拍测试暴露问题, 选择: 升 v6(beta) / 换 `denormal` / 自写规则编译 + `doublestar`。
- **不支持 v6 语义的降级**: `filepath.Match` 在 Windows 上对 `\` 的处理与 POSIX 不同, 含反斜杠的模式要自行规范化(库内部用 `path` 而非 `filepath` 的部分依赖导出行为, 需在测试中确认)。
- **字体首次枚举开销**: 一定要在后台取, 且不要在每次打开设置时重复取(mygo 自己会缓存到 `RegisterFont` 为止, 所以取一次即可)。
