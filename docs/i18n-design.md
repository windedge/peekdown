# Peekdown 国际化（i18n）方案设计

状态：设计稿（只读调查产出，未改代码）
参照实现：`C:/src/go/nexttodo`（mygo i18n 的应用样例）
调查日期：2026-02

## 1. mygo i18n API 清单与 generate 机制结论

### 1.1 运行时 API（mygo@v0.7.5-0.20261105040656 / i18n 包）

| API | 位置 | 说明 |
|---|---|---|
| `i18n.NewCatalog(fsys fs.FS, source string)` | `i18n/catalog.go` | 从目录根的 `<tag>.json` 建 catalog；必须有 source 语言的文件；并发安全；无参消息按 locale+key 缓存（零分配） |
| `i18n.Register(c)` | `i18n/catalog.go` | 注册 app 的 catalog：其语言成为 app 的语言（`i18n.Languages()`），键 `"<ns>.<key>"` 可覆盖库（mygo 控件）的消息 |
| `i18n.Languages()` | `i18n/languages.go` | 已注册 catalog 的语言表（source 在前）；系统 locale 匹配池就来自它 |
| `i18n.Locale()` | `i18n/catalog.go` | 返回 app 当前显示语言；由 `mygo` 包 `init()` 里 `SetLocaleFunc(currentLocale)` 接到 `mygo.App.Locale()` |
| `Catalog.Format(locale, key, args)` | `i18n/catalog.go` | 按 CLDR 距离匹配（zh-TW 得 zh-Hant、zh-CN 得 zh-Hans），沿父链回退，最终回退到 source 语言；缺 key 返回 key 本身 |
| ICU MessageFormat | `i18n/message.go` | `{name}`、`{n, plural, one {# file} other {# files}}`、`=0` 精确值、`select`/`selectordinal`、`{d, date, short}`、撇号转义 |
| 伪语言 en-XA / ar-XB | catalog.go 文档 | 恒可用于测试翻译布局（拉长/RTL 变形），无 JSON 文件 |
| `mygo.App.SetLocale(tag)` | `locale.go:147` | `""` = 恢复跟随系统；非法 tag panic；**由 mygo 自己持久化**（Windows 下 PathUserData 的 locale 文件），改动即时生效并回调 `OnLocaleChanged`；`MYGO_LOCALE` 环境变量单次覆盖 |
| `mygo.App.OnLocaleChanged(fn)` | `locale.go:186` | 语言变化回调（重建菜单、窗口标题等） |
| `ui.Tester.SetLocale(tag)` | `ui/headless.go:440` | **只设置 headless 的 `rt.lang`**，即视图内 `ui.Context.Locale()`；不影响包级 `i18n.Locale()`（headless 下无 mygo app，走 `MYGO_LOCALE` 或注册语言表，无注册时回退 "en"） |

关键推论（测试语言）：headless 测试里若 `locales` 包被导入（init 里 `Register`），`i18n.Locale()` 的解析顺序是：`MYGO_LOCALE` 环境变量 > `i18n.Languages()` 与系统语言匹配（测试进程的系统是开发机，可能是 zh-CN）> source "en"。**因此包级 `locales.Xxx()` 在 CI 上不确定**，测试必须显式用 `locales.In("en")` 或设置 `MYGO_LOCALE=en`。

### 1.2 generate 机制

- 配置：`mygo.json` 增加 `"i18n": { "dir": "locales", "source": "en" }`（nexttodo 的 mygo.json 即如此）。
- 命令：`mygo generate`（`cmd/mygo/generate.go` + `cmd/mygo/i18n.go`）：
  - 读取 `locales/*.json`，校验消息解析、参数与复数一致性（错误则失败，警告照常打印）；
  - 生成 `locales/catalog_gen.go`（`internal/i18ngen.GoFile` 常量），内容为：`//go:embed *.json` + `NewCatalog(files, source)` + `Register` + 每个键一个类型安全方法（无参 `Xxx()` / 带参 `Xxx(args...)`），以及 `In(locale)` / `Messages`；
  - 有前端绑定时还生成 TS 模块（peekdown 无前端，不涉及）。
- `mygo i18n check [dir]` 可单独校验目录。
- `mygo.exe` 已在 `C:/Users/xujl/go/bin/mygo`（go 1.27 toolchain 可直接 `go run github.com/egoist/mygo/cmd/mygo generate` 作为替代）。
- 包名推断：`i18ngen.PackageName` 取目录里非 `_test.go`、非 `catalog_gen.go` 的 .go 文件的包名，否则按目录名合成。**给 locales 目录放一个手写 `doc.go`（package locales）即可固定包名。**

### 1.3 nexttodo 的既有约定（直接沿用）

- 源语言 en（camelCase 键 + `@@locale` 元数据 + `@key` 描述/占位符元数据），翻译 `zh-Hans.json`。
- 使用模式三种：
  1. 包级 `locales.Xxx()`（跟随 app 语言）；
  2. `a.tr := locales.In(a.lang)`，后台 goroutine 先 `a.useAppLanguage()` 快照语言再格式化（避免竞态）；
  3. `locales.In(mygo.App.Locale())` 临时取。
- 语言设置：`languageTags = ["", "en", "zh-Hans"]`，下拉显示 autonym（English / 简体中文 / System Default）；选择后调 `mygo.App.SetLocale(tag)`（持久化、即时生效是 mygo 的事，app 不自己存）。
- 测试：`newTester` 里 `tt.SetLocale(lang)`（固定视图语言），逻辑测试用 `a.useTestLanguage("en")` 快照 `tr`；另有双语言截图测试（`TestSnapshots` 遍历语言 x 明暗）。

## 2. peekdown 用户可见字符串盘点

统计口径：`rg` 检索非测试代码里 `Label:`、`Tooltip:`、`Placeholder`、`ui.Text(c, "...")`、`ui.Button(...)`、`m.Item("...")`、错误文案字面量。

### 2.1 按文件计数（非测试代码）

| 文件 | 条数（约） | 代表例 |
|---|---|---|
| `internal/shell/menu.go` | 27 | File / Edit / View / Help 菜单：Open...、Close Tab、Find Next、Zoom In、Toggle Outline、Settings...、Exit、Peekdown on GitHub、Keyboard Shortcuts |
| `internal/shell/settings.go` | 35 | Settings 标题、分区 Appearance/Editor/Files/System、Theme、Layout、Content Font、Scroll Speed、Inertia Scroll、Explorer Root、Project Root Markers、Auto Refresh、Cancel、Save、"Choose "+字体名 |
| `internal/shell/shell.go` | 25 | Welcome to Peekdown、Open a Markdown file to get started、Open File、Or drag and drop files...、Recent Files、Clear、No document open、状态栏 Files/Outline/Settings/Search 按钮 + tooltip、Zoom tooltip、窗口标题 `"Could not read this document: %s - Peekdown"`、`"%s - Peekdown"` |
| `internal/view/view.go` | 15 | 右键菜单 Copy/Select All/Find/Refresh/Reveal in Sidebar/Open in File Manager/Copy Path、代码块 Copy/Copied 按钮 + tooltip、"rows %d-%d of %d" |
| `internal/shell/searchbar.go` | 6 | Find placeholder、Match case、Regular expression、Previous/Next match、Close search、"bad pattern"、"no results"、"%d/%d" |
| `internal/shell/filetree.go` | 4 | No file open、No Markdown files |
| `internal/shell/sidebar.go` | 3 | Files、Outline tab、No headings |
| `internal/shell/titlebar.go` | 8 | Open file、Find in document、Files、Outline、Settings 图标按钮 tooltip（含快捷键提示）、Home、Peekdown 品牌字 |
| `internal/shell/tabs.go` | 7 | "Close "+文件名 tooltip、右键菜单 Reveal in Sidebar / Open in File Manager / Copy File Path / Close Tab / Close Other Tabs / Close All Tabs |
| `internal/shell/fontpicker.go` | 4 | Choose Content Font / Choose Mono Font 标题、Search installed fonts...、No matching fonts |
| `internal/view/frontmatter.go` | 5 | Copy TOML / Copy YAML、Toggle frontmatter、Copied frontmatter |
| `internal/view/mermaid.go` | 3 | Diagram rendered. Open SVG for full view. / Diagram failed to render |
| `main.go` | 2 | Open Dialog 标题 "Open Markdown"、filter 名 "Markdown" |
| 合计 | 约 135 | 去重后（Copy/Settings 等跨处复用）约 110-120 个键 |

另有 mygo 控件自带字符串（日历、color picker 等），已随 `_ "github.com/egoist/mygo/locales/zh-Hans"` 导入获得中文。

### 2.2 测试里按 label 查找的断言

`Click("...")/HasText("...")/Find("...")` 合计约 202 处引用（含文件名等无关字符串）。**受 i18n 影响的核心 label**（改中文后会破）：

- `Click("Save")` x5、`Click("Cancel")`、`Click("Copy")` x2、`Click("Copy YAML")` x4、`Click("Copy TOML")`
- `Click("Toggle frontmatter")` x5、`Click("Match case")` x2、`Click("Regular expression")` x2
- `Click("System Default row")`（未来语言行）、`Click("Choose Content Font")` x2、`Click("Increase")`
- `HasText/Find`：`"Welcome to Peekdown"` x8、`"Settings"`、`"Outline"`、`"Files"`（sidebar/shell 测试多处）、`"No matching fonts"` x2、`"Copied"` x2、`"100%"`、`"Main menu"`
- `chooseMenuItem(t, m, "Refresh", accel)`、`"Scroll to Top/Bottom"` 等 menu 测试约 10 处
- 窗口标题：`shell_test.go:180/504/511` 断言 `"path - Peekdown"`、`"Peekdown"`

 peekdown 测试没有 `t.Parallel()`，进程级环境变量方案安全。

## 3. 方案

### 3.1 推荐路径：引入 mygo generate（决策：A）

**用 mygo generate，不手写 catalog_gen.go。**理由：

1. peekdown 已有 mygo.json，加一行 `"i18n": {...}` 即接入；`mygo.exe` 已安装，一条命令出全部类型安全访问器；
2. `mygo i18n check` 免费得到翻译一致性校验（参数/复数/缺失键），手写没有；
3. 手写 110+ 键的访问器（约 1300 行样板）是一次性纯手工成本，且以后每加一个键都要手写两处（json + 方法）；生成器把这个成本降为零；
4. nexttodo 同款流程已验证可行。

保留点：locales 目录放手写 `doc.go` 固定 `package locales`；生成产物 `catalog_gen.go` 提交进仓库（CI 不需要装 mygo，构建是纯 `go build`）。

**备选 B（手写 catalog）**：仅当不想引入生成命令时选。做法与 nexttodo 产物逐字等价（`//go:embed *.json` + `NewCatalog` + `Register` + 手写方法集）。缺点：样板多、无校验工具。不推荐，除非团队决定永远离线维护。

**备选 C（极简 go-i18n 类库）**：放弃。mygo i18n 与 ui.Context 的语言、CLDR 复数、mygo 控件字符串联动是一体的，引入第三方库反而丢失控件翻译与系统 locale 跟随。

### 3.2 架构与分层

新增 `locales/` 包（根目录，同 nexttodo）：

```
locales/
  doc.go            // package locales（固定包名，手写）
  catalog_gen.go    // mygo generate 产物，提交
  en.json           // 源语言（camelCase 键 + @ 元数据）
  zh-Hans.json      // 简体中文
```

main.go 变化：

```go
import (
    "github.com/windedge/peekdown/locales"
    _ "github.com/egoist/mygo/locales/zh-Hans" // mygo 控件的中文字符串
)
// 启动后与 OnLocaleChanged 里：
mygo.App.OnLocaleChanged(func(string) {
    mygo.App.SetMenu(shell.BuildMenu(s))
    a.win.SetTitle(a.title())
})
```

字符串改造分组（每处 `locales.Xxx()`，带参数的用生成器签名）：

| 分组 | 文件 | 说明 |
|---|---|---|
| 菜单 | `internal/shell/menu.go` | `MenuSpecs` 构造时即取 `locales.MenuFile()` 等；`BuildMenu` 与 `drawMenu` 共用 specs，无需两份 |
| 设置弹窗 | `internal/shell/settings.go` | 标题、4 个分区名、每个 field 行、Cancel/Save、"Choose %s"（占位符键 `chooseFont`）、字体选择器标题 |
| 欢迎页/状态栏/标题栏 | `internal/shell/shell.go`、`titlebar.go` | Welcome 三行、Recent Files、Clear、状态栏按钮 tooltip、`"Could not read this document: %s"`（窗口标题拼接） |
| 搜索浮窗 | `internal/shell/searchbar.go` | placeholder、两个 toggle、prev/next/close tooltip；`searchStatus` 的 "bad pattern"/"no results"；"%d/%d" 用 `searchCount` 键 |
| 标签栏 | `internal/shell/tabs.go` | "Close %s"（参数键）、右键菜单 6 项 |
| 文件树/侧栏 | `filetree.go`、`sidebar.go` | 空态文案、Files/Outline tab |
| 字体选择器 | `fontpicker.go` | 标题、placeholder、空态 |
| 正文视图 | `internal/view/view.go`、`frontmatter.go`、`mermaid.go` | 右键菜单 7 项、Copy/Copied、Copy TOML/YAML、Toggle frontmatter、"rows %d-%d of %d"、mermaid 两条状态 |
| 宿主 | `main.go` | 打开对话框标题与 filter 名 |

动态拼串（`"Close "+filepath.Base(path)`、`"Choose "+name`）一律改成参数化键：`"closeTab": "Close {name}"` / `"closeTab": "关闭 {name}"`。

**语言切换的即时刷新**：peekdown 是单窗口直绘 UI，`OnLocaleChanged` 里重建 native 菜单（nexttodo 同款）+ `win.Invalidate()` 即可；设置弹窗内切换语言时顺带重绘当前帧。

### 3.3 语言支持决策

**建议：en（源）+ zh-Hans 起步，不做 ja。**理由：

- 目标用户与作者语言覆盖即够；mygo 自带 ja 只是"控件"翻译，app 自己的消息没有 ja 文案，补全成本不值起步投入；
- catalog 的匹配链免费支持以后加语言：新增 `ja.json` + regenerate 即可，架构不用动。

**语言跟随 vs 设置项：两者都做，跟随系统为默认值。**

- 默认：`mygo.App.Locale()` 已内置"系统语言与 app 语言表（Register 决定）按 CLDR 匹配"，zh-Hans 系统、zh-CN 系统自动得 zh-Hans，其余得 en。零代码。
- 设置项：settings.json 增加 `language` 字段（`""` / `"en"` / `"zh-Hans"`，与 nexttodo 一致），设置弹窗加一个下拉（System Default / English / 简体中文，autonym 显示）。选中非空值调 `mygo.App.SetLocale(tag)`；选回 `""` 调 `mygo.App.SetLocale("")` 恢复跟随。持久化由 mygo 负责（PathUserData 的 locale 文件），**不把生效值写进 peekdown 自己的 settings.json**，避免两份真相；`settings.json` 里的 `language` 只存用户显式选择（或干脆不存，完全交给 mygo 的持久化--推荐后者，少一个字段、少一处同步逻辑）。
- 注意：`SetLocale` 的持久化文件与 peekdown 的 settings.json 是两处，卸载清理语义不同；在设置 UI 上不做"导出/同步语言"的承诺即可。

### 3.4 测试策略

问题：peekdown 现有 200 处 label 断言默认拿到什么语言？

- headless 无 mygo app，`i18n.Locale()` 走 `MYGO_LOCALE` > `i18n.Languages()` 匹配系统语言 > "en"。开发机是 zh-CN 系统时，包级 `locales.Xxx()` 会返回中文，断言 `"Save"` 会破。
- `ui.Tester.SetLocale("en")` **只管视图的 `ui.Context.Locale()`**，救不了包级访问器。

策略（三选一，推荐 1+2 组合）：

1. **测试辅助统一入口（必做）**：在 `internal/shell`、`internal/view` 的测试 helper 里加 `TestMain` 或 helper 函数 `os.Setenv("MYGO_LOCALE", "en")`（在首次 `i18n.Locale()` 调用前）。进程级、一次设置，全部包级访问器回到 en；peekdown 无 `t.Parallel()`，安全。对 window title 断言（`"%s - Peekdown"`）同样生效。
2. **专项多语言测试（可选）**：新增 `i18n_test.go`：`MYGO_LOCALE=zh-Hans` 下渲染欢迎页/设置，`HasText("欢迎使用 Peekdown")` 断言键真的翻译了（防 zh-Hans.json 漏键退化成英文）。
3. **按 In("en") 断言（不推荐大面积用）**：把断言改成 `locales.In("en").Save()`。防语言漂移最好，但把 200 处断言全改一遍成本高、可读性差；只对个别语义敏感的断言用。

menu 测试（`chooseMenuItem(t, m, "Refresh", ...)`）：`BuildMenu` 在 MYGO_LOCALE=en 下产出英文，断言不变；多语言菜单正确性交给专项测试或截图。

布局测试注意：中文文案宽度与英文不同，逐条 rect 断言的测试（tabbar/settings_layout 等）在 en 下全部保持原值，不受影响；若 CI 未来跑 zh 矩阵，需单独修期望值。

## 4. 分批实施计划（可拆给 1-2 个 coder）

### 批次 1：骨架 + 生成流程（约 0.5 人时，coder A）

1. `mygo.json` 加 `"i18n": {"dir": "locales", "source": "en"}`；
2. 建 `locales/doc.go`（package locales）+ 从字符串清单起草 `en.json`（先只放第一批键）；
3. 跑 `mygo generate` 生成 `catalog_gen.go`，`go build ./...` 验证；
4. main.go：导入 locales、`_ "github.com/egoist/mygo/locales/zh-Hans"`、`OnLocaleChanged` 重建菜单 + SetTitle；
5. 测试 helper 设 `MYGO_LOCALE=en`（TestMain）。

验收：`go build ./... && go test ./... -count=1` 全绿（此时业务代码还没换，只是架子）。

### 批次 2：zh-Hans.json 全量 + 低风险界面切换（约 2 人时，coder A）

按分组替换 + 补两份 JSON（每替换一批 regenerate 一次）：

1. 欢迎页/状态栏/标题栏（shell.go、titlebar.go）；
2. 搜索浮窗（searchbar.go，含 searchStatus 三个状态 + 计数键）；
3. 文件树/侧栏/字体选择器（filetree、sidebar、fontpicker）；
4. 标签栏（tabs.go，"Close {name}" 参数化）；
5. frontmatter/mermaid/代码块 Copy（view 局部）。

验收：每完成一组 `go test ./... -count=1`；全部完成后真机截图（无参启动 + 打开文档两条路径，AGENTS.md 规则）确认无溢出/遮挡（中文更宽）。

### 批次 3：设置弹窗 + 语言选择 + 菜单（约 1.5 人时，coder B 可并行批次 2 的后半）

1. settings.go 全量（分区名、field 行、Cancel/Save、"Choose {name}"）；
2. settings.json/Shell 增加 `Language` 字段 + 下拉行（System Default/English/简体中文），Changed 时 `mygo.App.SetLocale`；RestoreFromSettings 时应用一次；
3. menu.go：MenuSpecs 26 个 label 换 locales（menu_test 的 label 断言在 MYGO_LOCALE=en 下不变）；
4. view.go 右键菜单 7 项 + "rows %d-%d of %d"。

验收：`mygo i18n check` 零错误零警告；切语言到 zh-Hans 真机看菜单/设置/搜索/欢迎页四张截图；`go vet && gofmt -l` 干净。

### 批次 4：专项测试与收尾（约 0.5 人时）

1. `i18n_test.go`：zh-Hans 下渲染欢迎页 + 设置弹窗，断言中文出现（防漏翻）；
2. 伪语言截图一次（`MYGO_LOCALE=en-XA` 或 ar-XB）看有没有硬编码漏网；
3. CHANGELOG + docs 记录"新增语言只需 locales/<tag>.json + mygo generate"。

### 总量

- 新增/改动文件：约 16 个（locales 4 个新文件、shell 9 个、view 3 个、main.go、mygo.json、若干测试文件小调）；
- 翻译量：en.json 约 115 键（作者写，即源文案整理），zh-Hans.json 约 115 条翻译；
- 预估合计 **4.5-5 人时**（单人 1 个工作日内；双人并行约 3-4 小时墙钟）。

## 5. 风险与备注

- **窗口标题拼接**（`"Could not read this document: %s - Peekdown"`）：title 测试断言含 "Peekdown"，品牌名建议不翻译（en.json 里 appName 保持 "Peekdown"），仅翻译前半句，测试不用改。
- **生成产物的 diff 噪音**：catalog_gen.go 每次全量重写，提交前 `mygo generate` 一次即可，避免半新半旧。
- **mygo 版本锁定**：catalog_gen.go 由特定 mygo 版本生成，升级 mygo 后需 regenerate；在 locales/doc.go 注一句。
- **"System Default row" 测试**（settings_layout_test）：未来语言下拉的行 label 若本地化（"System Default" -> "跟随系统"），该测试断言会破；建议该测试改为按 key 查找（`c.Key("language-system")`）或归入批次 3 的测试适配。
- mygo 控件自身的字符串（下拉、日历等）由 `_ "mygo/locales/zh-Hans"` 覆盖，无需自建键。
