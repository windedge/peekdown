# Mermaid CJK 豆腐块调研（只读）

## 关键发现（与预期不同）

raster/text.go **不是位图字体**，已经用 opentype 矢量栈：`opentype.Parse(goregular.TTF)` 装两个包级变量 `regularFnt/boldFnt`（text.go:37-40），`font.Drawer.DrawString` 绘制（text.go:151-157）。豆腐块原因单纯是 goregular 无 CJK 字形。度量与绘制同 face：`buildFace`（text.go:89-108）用同一 face 量宽并按布局预留宽度收缩字号。且布局侧 CJK 度量早已正确：`svgutil.runeWidth` 对 wideRanges 计 1.0em（svgutil.go:107-116），测试含 `TextWidth("日本語")`（svgutil_test.go:58）。**换 CJK face 后中文宽度正好等于预留，无需改布局。**

注入点：无。`regularFnt/boldFnt` 是私有包变量，`PNG/RasterizeSVG` 签名不收字体（raster.go:20,32），也无关闭文本层的开关（drawText 无条件调用，raster.go:50）。mygo 的 `ui.Shape`+`Painter.Glyphs` 走系统文本引擎（docs/ui/drawing.md:215-239），但只能画 glyph 进帧，拿不到"画进 image.Image"的 API。

## 路线矩阵

| 路线 | 工作量 | 风险 | 中文 | 英文回归 |
|---|---|---|---|---|
| a fork+注入系统字体 | 4-8h | 低 | 好 | 低（shrink 只缩不放） |
| a' vendor 替换 | 同 a | 低但脏（整树入库） | 好 | 低 |
| a'' 上游 PR | 同 a | 周期不可控 | 好 | 低 |
| b mygo 叠画文本 | 2-3天 | 高：需复刻 text.go 全部坐标逻辑+帧内叠画破坏 bitmap 缓存 | 好 | 中 |
| c 换渲染器 | 1周+ | oksvg 本身不画 text（raster.go:45 注释），纯 Go 无现成替代，等于重写 | - | - |
| d 降级源码块 | 1h | 无 | 无 | 无 |

## 推荐：路线 a

三步：fork 加 `raster.SetFonts(r, b *opentype.Font)`（改 init 为缺省+可覆盖，text.go:37-40 一处）+ go.mod replace；peekdown 加字体加载：`opentype.ParseCollection` 原生支持 ttc（opentype.go:37），Windows 读 `msyh.ttc`/`msyhbd.ttc`，miss 则回退 goregular；跑 raster_test.go 英文 golden 回归+中文手工用例（含 rotate 轴标签）。vendor 弃（污染整棵依赖树）；上游 CHANGELOG 显示字体相关 PR 被接受过（#32，CHANGELOG.md:17），fork 后顺手提 PR，merge 撤 replace。b/c 不值得：b 要复刻 anchor/dy/rotate/translate 换算且每帧重画；c 无生态支撑。d 仅作字体全 miss 时的兜底。

风险注记：x/image sfnt 无 hinting，但 scale>=2（mermaidScale）下足够清晰；雅黑 ASCII 略窄于 Helvetica 度量，盒子只会偏松不会截断。

## Alternatives without forking（不改 go-mermaid 的解法盘点）

结论先行：**上游无新版**（`go list -m -versions` 止于 v0.1.6，CHANGELOG 无字体 API）；**唯一真正修复 CJK 且自包含的路线是拷贝 raster/ 做自有适配层**，其余降级或否决。

1. **上游更新**：v0.1.0-v0.1.6，v0.1.6 即最新（CHANGELOG.md，2026-09-10）。#32 只改 svgutil 度量，未动 raster 字体。不可行。
2. **自有适配层（推荐）**：go-mermaid 只出 SVG（公开 API `mermaid.Render`），peekdown 拷 raster/ 三文件共 413 行（raster.go:73、text.go:232、arrows.go:108，另 raster_test.go:57）进 internal/，text.go:37-40 的 init 改为注入 font.go 已加载的 msyh face。MIT LICENSE（LICENSE.md:1-4）允许拷贝改造，保留版权声明即可。Render 签名不变则 Cache/Key/bitmapCache/MaxScale 零改动（view.go:82、mermaid.go:38）。成本 3-4h。风险：上游改 SVG 输出格式时本地正则需同步（fork 方案同样有此耦合，只是 merge 时可见）。
3. **mygo 矢量叠加**：ui.Shape+Painter.Glyphs 走系统 shaping（drawing.md:215-239），但 Glyphs 只能画进帧，LoadMermaid 须改返回"位图+glyph 集"，且 anchor/dy/tspan/rotate 全套坐标（text.go:151-206）要按 mygo 布局复刻、Shape 无缓存需自制（drawing.md:230）。2-3 天起，破坏 bitmap 缓存。否决。
4. **换库 xo/mermaid**：模块缓存实锤走 wazero 跑 Rust wasm（mermaid.go:1-10），只输出 SVG 不光栅化（无 PNG API），mermaid.wasm 2.9MB，冷启动约百毫秒级编译。字体问题原样回到自己手里，成本=路线 2+2.9MiB。否决。
5. **降级兜底**：`含 CJK && !cjkActive()` 时显示源码块+提示行。检测用 unicode.Han 等扫 src，face 缺字形可逐 rune 查 opentype Face.Index(r)==0。1h，零风险，可作字体全 miss 兜底，非修复。
6. **预处理替换标签**：改写标签破坏图语义，无低成本变体。否决。

**排序**：路线 2 > 路线 5（兜底）> 其余否决。

**fork vs 路线 2**：fork diff 最小（上游约 3 行）、上游演进可 merge，但 `replace ../go-mermaid` 要求每台构建机有该目录，CI/协作破裂（go.mod:12-15 已自记此痛）；路线 2 自包含、CI 无感，代价是本地维护 413 行。若计划提 PR（上游收过 #32 字体相关 PR），fork 是撤 replace 前的过渡最优；若 PR 无望，路线 2 是唯一自包含真解。

决定：路线 2 已实施（internal/mermaid/raster，附 internal/mermaid/svgutil 因其被 raster 引用且为上游 internal 包），replace 已撤，fork 已删。
