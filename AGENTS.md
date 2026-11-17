# AGENTS.md

本文件写给在这个仓库里改代码的 AI 代理和人。

## 项目概览

Peekdown 是 Markdown 阅读器，Go + [mygo](https://github.com/egoist/mygo) UI，目标是复刻 Rust 版 peeq 的阅读体验。

- `internal/shell`：窗口外壳，标签栏、侧栏、状态栏、搜索浮窗、设置弹窗、欢迎页
- `internal/view`：文档正文的渲染与交互（段落、代码块、表格、mermaid、frontmatter）
- `internal/doc`：Markdown 文档模型
- `internal/parser`：GFM 解析
- `cmd`、`main.go`：入口与宿主接线
- `docs/`：验收、差距分析等过程文档

## 代码与提交约定

- 注释只解释代码本身说不清的"为什么"，不写复述函数名、参数、返回值的注释
- 测试用 headless `ui.Tester`（`renderSize` / `frame`），逐条断言几何（rect）而不是只断言"有这段文字"
- 提交信息用 Conventional Commits，不加 `Co-authored-by`
- 除非明确要求，不要自动执行 git 提交

## UI 布局规则（mygo）

- **滚动条是 overlay，文字必须让位**：mygo 的滚动条画在滚动容器内部右缘（内缩 3px、自己宽 6px，共占最外侧 9px），只在鼠标悬停/拖动/`.KeepScrollbars()` 时才画，但一画就盖住紧贴右缘的内容。任何放进滚动容器的文字行，右侧必须预留 >= 12px 的内边距（`PaddingEnd`，或给行内容 `Padding(8, 14)` 这类右内边距），让文字右缘停在滚动条左侧。横向滚动容器同理：文字底缘要离容器底边 >= 12px，否则横向滚动条盖住最后一行的下半截。
- **滚动容器里的文字块要定宽**：行是 `FillWidth()`、里面的文字块只按内容收缩（shrink-to-fit）时，`MaxLines(1)` 会按自然宽度排版，文字会溢出行的 padding 一直顶到容器右缘，正好落在滚动条下面。给文字块 `Grow(1)`（或显式宽度）让它在行内被约束，截断点才会落在 padding 上。
- **固定高度的 TextInput 会裁字**：mygo 的 TextInput 在 `Height(h)` 下内容区 = h - padding - border，小于文字行高时文字/光标溢出被裁且垂直居中失准。不要给 TextInput 设固定 Height，让内容撑起，或保证 h >= 字号行高 + padding + border。
- **布局容器 Fill 与 Grow 的区别**：Column 里 `Fill()` 按 100% 父高占位会把兄弟（状态栏等）挤出视口；滚动区用 `Grow(1)` 让它受视口约束。
- **宽度要用 `Width`，不要只写 `MaxWidth`**：`MaxWidth` 只是上限，子列仍按内容收缩，光把 `MaxWidth(440)` 改成 560 不会让列表变宽。要跟随容器又设上限，就用 `Width(min(上限, 窗口宽 - 两侧边距))` 这样的显式宽度。
- **修改布局后必须截图验证，但只用 headless**：headless 测试通过不代表视觉正确（历史案例：状态栏被挤出、滚动条盖字、placeholder 错位、主题名回落成 i18n key、网格换行、彩点被裁、文字画成实心块，都是截图暴露的）。截图的唯一方式是下面的 headless 流程，**不要重建 exe、不要启动实机窗口、不要鼠标注入**。

## 验证

```bash
go build ./... && go vet ./... && go test ./... -count=1
gofmt -l .      # 应为空
```

改动涉及布局时，额外用无参数启动（欢迎页）和打开文档两条路径各截一张图核对。

### 截图只用 headless

视觉证据一律走 `ui.Tester`：`tt.Click` / `tt.Frame` / `tt.Image() *image.RGBA`，再用 `internal/testshot` 写盘或裁切放大。这就是 mygo 自己的做法（`C:/src/go/mygo/ui/ui_test.go` 的 `savePNG`，env 门控）。截图测试挂在 `PEEKDOWN_SHOT=1` 下，常规 `go test` 自动跳过。

布局改动就是跑截图测试 + 看 PNG，**不重建 exe、不启动应用、不用 PowerShell / PrintWindow / 鼠标注入**：坐标算不准、hover 事件不保证送达、锁屏就失效，而且它看不到 headless 能立刻暴露的缺陷。

`win.CapturePage()`（mygo `window.go`）是唯一可用的真窗口取像素途径，CPU 重画 last frame、不受遮挡影响；但只留给"必须验证系统级合成（窗口阴影、DPI、caption 按钮）"的场合，日常布局检查不需要。

