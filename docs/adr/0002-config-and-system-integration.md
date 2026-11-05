# 配置与系统集成：用 mygo 原生机制，不迁移旧配置

**Status**: accepted（关闭工单 #10）

应用不再读取 Rust 版的 `config.toml`。配置、窗口状态与最近文件都改用 mygo 提供的机制，用户可见行为对齐，但存储格式与旧版不兼容，也不做一次性导入。

## Decision

- **设置**：JSON 文件 `settings.json`，位于 `mygo.App.Path(mygo.PathUserData)`（Windows 是 `%AppData%\Peekdown`，Linux 是 `~/.config/Peekdown`，macOS 是 `~/Library/Application Support/Peekdown`）。字段名沿用 Rust 配置里含义相同的键，启动时读取一次，缺失文件用默认值；写入用临时文件加原子替换；未知键忽略。
- **不迁移**：不读取、不解析、不导入旧 `config.toml`，也不删除它。旧用户从默认设置开始。
- **窗口状态**：`WindowOptions.StateKey: "main"`，位置、大小、最大化与全屏由 mygo 写入 `PathUserData/window-state.json`，多显示器与 DPI 由框架处理。旧配置里的 `window_x`、`window_y`、`window_width`、`window_height`、`is_maximized` 不再读取。
- **最近文件**：用 `mygo.App.AddRecentDocument` 与 `RecentDocuments`，上限 10，存 `PathUserData/recent-documents.json`，并同步到系统最近列表（Windows jump list、macOS Dock、Linux xbel）。旧配置顶层的 `recent_files` 不再读取。
- **单实例**：`App.RequestSingleInstanceLock` 取代命名管道。第二实例把命令行交给首实例后退出；首实例的 `OnOpenFile` 收到文件，`OnSecondInstance` 负责 `Restore` 与 `Focus`，Windows 的前台授权由 mygo 处理。
- **打开文件**：`App.OnOpenFile` 接收启动参数、第二实例转发的文件与 macOS Dock 拖入的文件。相对路径由 mygo 按工作目录解析成绝对路径。
- **文件关联**：由 NSIS 安装包与 macOS 应用包注册。删除 `--register`，应用运行时不再写注册表；Scoop 安装的 zip 不注册关联。
- **标题**：有文件时 `{绝对路径} - Peekdown`，否则 `Peekdown`。
- **外观**：`theme` 的 `auto`、`light`、`dark` 映射到 `mygo.ThemeSystem`、`ThemeLight`、`ThemeDark`。

## 与 Rust 版的行为差异

| 项目 | Rust 版 | Go 版 |
|---|---|---|
| 配置 | `config.toml`，TOML，手写窗口字段 | `settings.json`，JSON，mygo 路径；不迁移 |
| 窗口状态 | 配置里的 `window_*` 与 `is_maximized` | mygo `window-state.json`，处理多显示器与 DPI |
| 最近文件 | 配置里 15 条 | mygo 10 条，并同步系统列表 |
| 单实例 | 命名管道，自定义 JSON 消息 | mygo Unix socket 锁；Windows 用 AF_UNIX |
| 前台化 | 自写 Alt 键 hack + `AllowSetForegroundWindow` | 第二实例转交前台权，首实例 `Restore` + `Focus` |
| 文件关联 | `--register` 写 HKCU | 安装包/应用包注册；无 `--register` |
| 启动参数 | 任何参数都当文件路径 | 只接受已声明扩展名（`.md`、`.markdown`）且存在的文件 |
| 标题路径 | 参数原样 | mygo 解析后的绝对路径 |

## Consequences

- 正向：配置、窗口状态、最近文件、单实例、前台化都复用框架实现，代码少，跨平台行为一致。
- 负向：老用户的 `config.toml` 设置全部失效；最近文件从 15 条变 10 条；`--register` 脚本失效。
- 后续：设置对话框（#9）负责修改并保存 `settings.json`，保存要合并写入并去抖；欢迎页从 `App.RecentDocuments()` 取最近文件；文档视图（#5）落地后，`open` 从占位改为真正的标签页打开。

## References

- 本决策：[#10](https://github.com/windedge/peekdown/issues/10)
- 设置对话框：[#9](https://github.com/windedge/peekdown/issues/9)
- 文档视图：[#5](https://github.com/windedge/peekdown/issues/5)
- 垂直切片：[#15](https://github.com/windedge/peekdown/issues/15)
- 脚手架：[#12](https://github.com/windedge/peekdown/issues/12)
