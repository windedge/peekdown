# Peekdown 主题色切换（HTML 原型）设计

目标：给 `docs/prototype/main-mojian.html` 增加第二视觉维度"主题色"（THEME），与现有"明暗模式"（Auto/Light/Dark）正交，共 10 主题 x 2 模式 = 20 组合。本文件只产出设计，不改 HTML；下一棒 coder 落盘。

## 0. TL;DR

- 10 主题 id/名称/顺序 = mojian `internal/theme/themes.go` 的 `Choices`：petal/mist/meadow/dusk/tide/apricot/aurora/dune/paper/plain。
- 每主题浅深两套颜色全部取自 mojian 真实管线（见第 1 节），无任何拍脑袋取色；所有 10 个主题都有深色变体，无缺失。
- 原型改法：颜色令牌从 `:root` / `.theme-dark` 两组迁移为 `html[data-theme="x"]`（浅）+ `html[data-theme="x"].theme-dark`（深）共 20 组；标题栏加调色板按钮打开 THEME Popover（2 行 x 5 列窗口缩影卡片网格），点击即时切 `data-theme` 不关浮层，Esc/外点关闭。
- 设置弹窗的 Theme 行（Auto/Light/Dark）不动，那是明暗维度。

## 1. 来源与复算方法（为什么这些数值可信）

mojian 的每主题颜色不是字面量表，而是运行时程序化生成：

1. 窗口渐变 4 stop 与 BgTint：`themes.go` `washes[id].light/.dark/.lightGlow/.darkGlow`（字面量，直接抄）。
2. Accent / 终端 16 色 / 正文文字：`tone.go` `toneTheme(tone, dark)`，输入是 `washes[id].tone{Hue,Tint,Vivid,Accent}`，内部用 oklch(l,c,h) 数学转 sRGB。
3. 卡面/面板/文字灰阶：`themes.go` `deriveColors`，对 toneTheme 的产物做 `ui.Color.Mix`（sRGB 线性插值 + 0.5 舍入）与 alpha 合成。

本次设计用一段一次性脚本精确复刻了上述 Go 管线（oklch->sRGB 的 mojian 原始矩阵、Mix 的 `(1-t)a+tb+0.5` 截断舍入、Over 的 alpha 合成、`withAlpha` 的 `a*255+0.5` 舍入），并用源码中仅有的两组字面量做回归校验：petal accent 浅 `#a84e7c`、深 `#e887b6`（`theme.go lightColors/darkColors`），复算结果逐字节命中，管线可信。

完备性检查：`washes` 每个主题都有 `light` 与 `dark` 两组 4 stop，`toneTheme` 有完整 dark 分支（oklch 明度整体上调）→ **10/10 主题均有官方深色变体，无缺失，无需自行推色**。

一个重要澄清：mojian 的 `plain` 不是"当前原型的紫灰色"。当前原型 `:root` 抄的其实是 mojian 默认主题 petal（粉调）。mojian 的 Plain 是 hue 260 的中性灰（浅底 `#eceef4`、深底 `#141418`、accent `#466fbd`/`#7daafd`），本文按 mojian 真实值采用；petal 单独成组。

## 2. 变量清单与来源映射（每变量一行）

原型现有变量名保持不变，另新增 `--code-*` 与 `--pv-*`。来源标注格式：mojian 文件 + 字段/公式。

| 变量 | 来源（浅色分支 / 深色分支同理） |
|---|---|
| --bg-top/mid/low/bottom | themes.go washes[id].light[0..3] / .dark[0..3] |
| --bg-tint | washes[id].lightGlow / .darkGlow（alpha 0 时写 transparent） |
| --card | deriveColors：Card（浅=白混termBg .10 @.78；深=#18181c混termBg .35 @.58） |
| --card-focused | deriveColors：CardFocused（浅=白混termBg .06 @.97；深=#242429混termBg .35 @.95） |
| --text | toneTheme 的 Foreground（= oklch tint(0.28/.92)） |
| --text-muted | deriveColors：Foreground.Mix(Background,.38) |
| --text-faint | deriveColors：Foreground.Mix(Background,.56) |
| --accent | toneTheme：浅 oklch(0.55,0.13,tone.Accent)；深 oklch(0.74,0.13,tone.Accent) |
| --panel-sel | deriveColors：withAlpha(accent, 浅.12 / 深.18) |
| --selection | themes.go UI()：accent alpha 浅.25 / 深.40 |
| --quote-bar | peekdown quoteBarTint（accent alpha 浅.45 / 深.50，沿用原型现值公式） |
| --danger / --attention | deriveColors：term.Palette[1] / Palette[3]（petal 例外，见下） |
| --panel | deriveColors：termBg.Mix(white, 浅.60 / 深.04) @.97 |
| --surface-hover | themes.go UI()：Panel.Mix(Text,.06)，取不透明近似值 |
| --code-inline-bg / --code-block-bg | mygo markdown：Text alpha .07 / .04（RGB 随主题 Text） |
| --code-kw/ty/fn/num/str/cm | themes.go EditorTheme：p[1]/p[3]/p[5]/p[4]/p[2]/TextMuted；**petal 例外**：DefaultID 走 editor.DefaultTheme 的 GitHub 色（= 原型现状 L509-514 的值） |
| --pv-g1..g4 | 主题卡片预览渐变 = 该主题窗口 bg 4 stop（对照 PaintPreview 的 PaintBackground） |
| --pv-card / --pv-text / --pv-accent | PaintPreview：CardFocused / Text / Accent |
| --pv-dots | PaintPreview 的 term.Palette[0..7] 八点（petal = mygo VS Code palette 字面量） |

petal 例外细节（mojian `buildLooks` 的 DefaultID 特判）：petal 的 Danger/Attention 保持 `theme.go` 字面量 `#d92d20/#f97066` 与 `#f59e0b/#fbbf24`；语法高亮保持 GitHub 色；仅 Accent 取自 tone、终端 8 点用 VS Code palette。非 petal 主题的 Danger/Attention/语法色全部来自 tone palette。

不随主题的变量（留在 `:root` / `.theme-dark` 兜底组）：`--hover/--pressed`（固定黑白 alpha）、`--track/--track-border/--tab-active/--tab-inactive`（白 alpha）、`--card-border/--shadow*/--panel-border/--backdrop`（deriveColors 不覆盖）、`--match-bg/--scrollbar`（mygo editor/ui 层）、全部字号/字重/圆角/节奏/尺寸。

## 3. 20 组窗口令牌 CSS

替换范围：原型 L74 `:root {` 与 L176 `.theme-dark {` 中**颜色类**变量迁出；`:root` 保留为兜底（值 = petal 浅）+ 非颜色令牌，`.theme-dark` 兜底 = petal 深。20 组新块插在第 0 节末尾（L234 `.theme-dark` 块结束之后）。选择器特异性：`html[data-theme="x"]` > `:root`，`html[data-theme="x"].theme-dark` > `.theme-dark`，覆盖成立。

html 标签（L52）改为：`<html lang="zh-CN" class="theme-light" data-theme="petal">`（petal = mojian DefaultID，即当前视觉不变）。

```css
/* Petal（mojian 默认；wash petal + tone 350/1/0.95/350） */
html[data-theme="petal"] {
  --bg-top:#f7f2f6; --bg-mid:#f4eff4; --bg-low:#efeaf1; --bg-bottom:#e9e5ed; --bg-tint:rgba(235,222,232,.5);
  --card:rgba(255,255,255,.72); --card-focused:rgba(255,255,255,.96);
  --text:#1f1b20; --text-muted:#66616a; --text-faint:#75707a;
  --accent:#a84e7c; --panel-sel:rgba(168,78,124,.12); --selection:rgba(168,78,124,.25); --quote-bar:rgba(168,78,124,.45);
  --danger:#d92d20; --attention:#f59e0b;
  --panel:rgba(251,249,251,.97); --surface-hover:#eeecee;
  --code-inline-bg:rgba(31,27,32,.07); --code-block-bg:rgba(31,27,32,.04);
  --code-kw:#cf222e; --code-ty:#953800; --code-fn:#8250df; --code-num:#0550ae; --code-str:#0a3069; --code-cm:#6e7781;
}
html[data-theme="petal"].theme-dark {
  --bg-top:#241d25; --bg-mid:#1f1a21; --bg-low:#1b181d; --bg-bottom:#18151a; --bg-tint:rgba(120,72,100,.18);
  --card:rgba(25,23,28,.58); --card-focused:rgba(37,35,41,.95);
  --text:#f3f0f4; --text-muted:#b1abb4; --text-faint:#928c96;
  --accent:#e887b6; --panel-sel:rgba(232,135,182,.18); --selection:rgba(232,135,182,.4); --quote-bar:rgba(232,135,182,.5);
  --danger:#f97066; --attention:#fbbf24;
  --panel:rgba(35,32,38,.97); --surface-hover:#2f2c32;
  --code-inline-bg:rgba(243,240,244,.07); --code-block-bg:rgba(243,240,244,.04);
  --code-kw:#ff7b72; --code-ty:#ffa657; --code-fn:#d2a8ff; --code-num:#79c0ff; --code-str:#a5d6ff; --code-cm:#8b949e;
}
/* Mist（tone 255/1/0.8/255） */
html[data-theme="mist"] {
  --bg-top:#e3e9f4; --bg-mid:#ebeff6; --bg-low:#f1f2f3; --bg-bottom:#f2f1ec; --bg-tint:rgba(210,225,248,.75);
  --card:rgba(254,254,255,.78); --card-focused:rgba(254,255,255,.97);
  --text:#1f2a38; --text-muted:#6f7884; --text-faint:#959da7;
  --accent:#3772bb; --panel-sel:rgba(55,114,187,.12); --selection:rgba(55,114,187,.25); --quote-bar:rgba(55,114,187,.45);
  --danger:#af545c; --attention:#9e833c;
  --panel:rgba(250,252,255,.97); --surface-hover:#edeff3;
  --code-inline-bg:rgba(31,42,56,.07); --code-block-bg:rgba(31,42,56,.04);
  --code-kw:#af545c; --code-ty:#9e833c; --code-fn:#835b9f; --code-num:#256ec4; --code-str:#30875f; --code-cm:#6f7884;
}
html[data-theme="mist"].theme-dark {
  --bg-top:#0c1628; --bg-mid:#0d1827; --bg-low:#111b23; --bg-bottom:#111b23; --bg-tint:rgba(46,91,166,.3);
  --card:rgba(25,28,34,.58); --card-focused:rgba(33,36,42,.95);
  --text:#dee5ef; --text-muted:#949ba5; --text-faint:#717882;
  --accent:#70adfb; --panel-sel:rgba(112,173,251,.18); --selection:rgba(112,173,251,.4); --quote-bar:rgba(112,173,251,.5);
  --danger:#db8087; --attention:#d7bb74;
  --panel:rgba(37,44,52,.97); --surface-hover:#30373f;
  --code-inline-bg:rgba(222,229,239,.07); --code-block-bg:rgba(222,229,239,.04);
  --code-kw:#db8087; --code-ty:#d7bb74; --code-fn:#b992d5; --code-num:#5fa1f3; --code-str:#6bbf94; --code-cm:#949ba5;
}
/* Meadow（tone 140/1/0.9/140） */
html[data-theme="meadow"] {
  --bg-top:#e2eee5; --bg-mid:#ebf2e7; --bg-low:#f3f2e2; --bg-bottom:#f5f0d6; --bg-tint:rgba(214,240,222,.75);
  --card:rgba(254,255,254,.78); --card-focused:rgba(254,255,254,.97);
  --text:#212d1e; --text-muted:#717b6f; --text-faint:#97a095;
  --accent:#448335; --panel-sel:rgba(68,131,53,.12); --selection:rgba(68,131,53,.25); --quote-bar:rgba(68,131,53,.45);
  --danger:#b4552b; --attention:#988629;
  --panel:rgba(251,253,250,.97); --surface-hover:#eef1ed;
  --code-inline-bg:rgba(33,45,30,.07); --code-block-bg:rgba(33,45,30,.04);
  --code-kw:#b4552b; --code-ty:#988629; --code-fn:#945295; --code-num:#1776ab; --code-str:#248b27; --code-cm:#717b6f;
}
html[data-theme="meadow"].theme-dark {
  --bg-top:#081c0d; --bg-mid:#101d0c; --bg-low:#1b1c0b; --bg-bottom:#1b1c0b; --bg-tint:rgba(6,113,50,.3);
  --card:rgba(26,29,28,.58); --card-focused:rgba(34,36,36,.95);
  --text:#dfe7de; --text-muted:#969d94; --text-faint:#737a71;
  --accent:#7dbf6e; --panel-sel:rgba(125,191,110,.18); --selection:rgba(125,191,110,.4); --quote-bar:rgba(125,191,110,.5);
  --danger:#df825c; --attention:#d0bf66;
  --panel:rgba(39,46,37,.97); --surface-hover:#323930;
  --code-inline-bg:rgba(223,231,222,.07); --code-block-bg:rgba(223,231,222,.04);
  --code-kw:#df825c; --code-ty:#d0bf66; --code-fn:#cb8acb; --code-num:#5aa7db; --code-str:#63c561; --code-cm:#969d94;
}
/* Dusk（tone 315/1/1.1/315） */
html[data-theme="dusk"] {
  --bg-top:#e4dbf3; --bg-mid:#efdeee; --bg-low:#f6e3dc; --bg-bottom:#f7e6cf; --bg-tint:rgba(246,206,222,.8);
  --card:rgba(255,254,255,.78); --card-focused:rgba(255,254,255,.97);
  --text:#2f2433; --text-muted:#7d7380; --text-faint:#a199a4;
  --accent:#8e57a5; --panel-sel:rgba(142,87,165,.12); --selection:rgba(142,87,165,.25); --quote-bar:rgba(142,87,165,.45);
  --danger:#c14059; --attention:#b67702;
  --panel:rgba(253,251,254,.97); --surface-hover:#f1eef2;
  --code-inline-bg:rgba(47,36,51,.07); --code-block-bg:rgba(47,36,51,.04);
  --code-kw:#c14059; --code-ty:#b67702; --code-fn:#a332b6; --code-num:#4d67c1; --code-str:#258a41; --code-cm:#7d7380;
}
html[data-theme="dusk"].theme-dark {
  --bg-top:#190f2e; --bg-mid:#260f24; --bg-low:#2d140b; --bg-bottom:#2d140b; --bg-tint:rgba(125,65,138,.3);
  --card:rgba(29,27,33,.58); --card-focused:rgba(37,35,41,.95);
  --text:#e8e2ec; --text-muted:#9e98a2; --text-faint:#7b757f;
  --accent:#ca91e3; --panel-sel:rgba(202,145,227,.18); --selection:rgba(202,145,227,.4); --quote-bar:rgba(202,145,227,.5);
  --danger:#ed7183; --attention:#f2af54;
  --panel:rgba(47,41,51,.97); --surface-hover:#3a343e;
  --code-inline-bg:rgba(232,226,236,.07); --code-block-bg:rgba(232,226,236,.04);
  --code-kw:#ed7183; --code-ty:#f2af54; --code-fn:#db74ed; --code-num:#7d9af0; --code-str:#64c376; --code-cm:#9e98a2;
}
/* Tide（tone 205/1/0.85/205） */
html[data-theme="tide"] {
  --bg-top:#dbebef; --bg-mid:#e3eff1; --bg-low:#ecf3f0; --bg-bottom:#f2f2e8; --bg-tint:rgba(194,232,238,.75);
  --card:rgba(253,255,255,.78); --card-focused:rgba(254,255,255,.97);
  --text:#162d30; --text-muted:#687b7e; --text-faint:#90a0a2;
  --accent:#008695; --panel-sel:rgba(0,134,149,.12); --selection:rgba(0,134,149,.25); --quote-bar:rgba(0,134,149,.45);
  --danger:#b35347; --attention:#968737;
  --panel:rgba(249,253,254,.97); --surface-hover:#ebf1f2;
  --code-inline-bg:rgba(22,45,48,.07); --code-block-bg:rgba(22,45,48,.04);
  --code-kw:#b35347; --code-ty:#968737; --code-fn:#8559a1; --code-num:#1b75ad; --code-str:#2b885c; --code-cm:#687b7e;
}
html[data-theme="tide"].theme-dark {
  --bg-top:#001b22; --bg-mid:#001e1f; --bg-low:#0b1e1b; --bg-bottom:#0b1e1b; --bg-tint:rgba(0,112,128,.3);
  --card:rgba(24,29,32,.58); --card-focused:rgba(31,37,40,.95);
  --text:#dae8e9; --text-muted:#909e9f; --text-faint:#6d7b7c;
  --accent:#00c2d2; --panel-sel:rgba(0,194,210,.18); --selection:rgba(0,194,210,.4); --quote-bar:rgba(0,194,210,.5);
  --danger:#df8073; --attention:#cebf70;
  --panel:rgba(32,47,48,.97); --surface-hover:#2b3a3b;
  --code-inline-bg:rgba(218,232,233,.07); --code-block-bg:rgba(218,232,233,.04);
  --code-kw:#df8073; --code-ty:#cebf70; --code-fn:#bb90d8; --code-num:#5ca6dd; --code-str:#67c091; --code-cm:#909e9f;
}
/* Apricot（tone 50/1/1.05/50） */
html[data-theme="apricot"] {
  --bg-top:#f5e1d6; --bg-mid:#f8e7dc; --bg-low:#f8eedf; --bg-bottom:#f6f0da; --bg-tint:rgba(252,212,194,.78);
  --card:rgba(255,254,254,.78); --card-focused:rgba(255,254,254,.97);
  --text:#35251b; --text-muted:#82746c; --text-faint:#a69a93;
  --accent:#ac571d; --panel-sel:rgba(172,87,29,.12); --selection:rgba(172,87,29,.25); --quote-bar:rgba(172,87,29,.45);
  --danger:#d02f00; --attention:#b97600;
  --panel:rgba(255,251,249,.97); --surface-hover:#f3eeec;
  --code-inline-bg:rgba(53,37,27,.07); --code-block-bg:rgba(53,37,27,.04);
  --code-kw:#d02f00; --code-ty:#b97600; --code-fn:#a44688; --code-num:#426bbd; --code-str:#5a831b; --code-cm:#82746c;
}
html[data-theme="apricot"].theme-dark {
  --bg-top:#2a1207; --bg-mid:#291507; --bg-low:#25190a; --bg-bottom:#25190a; --bg-tint:rgba(151,62,22,.3);
  --card:rgba(31,27,27,.58); --card-focused:rgba(38,35,36,.95);
  --text:#ede2dc; --text-muted:#a39892; --text-faint:#80756f;
  --accent:#ec915c; --panel-sel:rgba(236,145,92,.18); --selection:rgba(236,145,92,.4); --quote-bar:rgba(236,145,92,.5);
  --danger:#fd6945; --attention:#f5ae3a;
  --panel:rgba(51,41,35,.97); --surface-hover:#3e342e;
  --code-inline-bg:rgba(237,226,220,.07); --code-block-bg:rgba(237,226,220,.04);
  --code-kw:#fd6945; --code-ty:#f5ae3a; --code-fn:#dd80be; --code-num:#749ded; --code-str:#8fbb59; --code-cm:#a39892;
}
/* Aurora（tone 170/1/1.15/170） */
html[data-theme="aurora"] {
  --bg-top:#dbefe8; --bg-mid:#e1ebf2; --bg-low:#e8e4f4; --bg-bottom:#f0e6f1; --bg-tint:rgba(198,238,224,.75);
  --card:rgba(254,255,254,.78); --card-focused:rgba(254,255,254,.97);
  --text:#192e27; --text-muted:#6b7c76; --text-faint:#91a19b;
  --accent:#008966; --panel-sel:rgba(0,137,102,.12); --selection:rgba(0,137,102,.25); --quote-bar:rgba(0,137,102,.45);
  --danger:#c44412; --attention:#998700;
  --panel:rgba(249,253,251,.97); --surface-hover:#ecf1ee;
  --code-inline-bg:rgba(25,46,39,.07); --code-block-bg:rgba(25,46,39,.04);
  --code-kw:#c44412; --code-ty:#998700; --code-fn:#944aab; --code-num:#0077bb; --code-str:#009136; --code-cm:#6b7c76;
}
html[data-theme="aurora"].theme-dark {
  --bg-top:#001f13; --bg-mid:#06192b; --bg-low:#1e122a; --bg-bottom:#1e122a; --bg-tint:rgba(0,115,75,.3);
  --card:rgba(24,29,30,.58); --card-focused:rgba(32,37,38,.95);
  --text:#dbe8e3; --text-muted:#919e99; --text-faint:#6e7b76;
  --accent:#3bc59e; --panel-sel:rgba(59,197,158,.18); --selection:rgba(59,197,158,.4); --quote-bar:rgba(59,197,158,.5);
  --danger:#f0754e; --attention:#d2c049;
  --panel:rgba(34,47,42,.97); --surface-hover:#2d3a35;
  --code-inline-bg:rgba(219,232,227,.07); --code-block-bg:rgba(219,232,227,.04);
  --code-kw:#f0754e; --code-ty:#d2c049; --code-fn:#ca84e1; --code-num:#38a9ea; --code-str:#15cb6d; --code-cm:#919e99;
}
/* Dune（tone 80/0.7/0.55/80，低饱和沙色） */
html[data-theme="dune"] {
  --bg-top:#ede4d8; --bg-mid:#f1e9de; --bg-low:#f4eee4; --bg-bottom:#f3efe8; --bg-tint:rgba(242,222,194,.7);
  --card:rgba(255,254,254,.78); --card-focused:rgba(255,255,254,.97);
  --text:#2e281d; --text-muted:#7c776e; --text-faint:#a19c94;
  --accent:#996700; --panel-sel:rgba(153,103,0,.12); --selection:rgba(153,103,0,.25); --quote-bar:rgba(153,103,0,.45);
  --danger:#a0624c; --attention:#a1813f;
  --panel:rgba(253,252,250,.97); --surface-hover:#f1efed;
  --code-inline-bg:rgba(46,40,29,.07); --code-block-bg:rgba(46,40,29,.04);
  --code-kw:#a0624c; --code-ty:#a1813f; --code-fn:#8d5d7d; --code-num:#517198; --code-str:#667d50; --code-cm:#7c776e;
}
html[data-theme="dune"].theme-dark {
  --bg-top:#22190b; --bg-mid:#211a0e; --bg-low:#201b12; --bg-bottom:#201b12; --bg-tint:rgba(135,79,0,.3);
  --card:rgba(29,28,28,.58); --card-focused:rgba(37,35,36,.95);
  --text:#e8e4dd; --text-muted:#9e9a93; --text-faint:#7b7770;
  --accent:#d5a13c; --panel-sel:rgba(213,161,60,.18); --selection:rgba(213,161,60,.4); --quote-bar:rgba(213,161,60,.5);
  --danger:#cb8e77; --attention:#dbb976;
  --panel:rgba(47,43,36,.97); --surface-hover:#3a362f;
  --code-inline-bg:rgba(232,228,221,.07); --code-block-bg:rgba(232,228,221,.04);
  --code-kw:#cb8e77; --code-ty:#dbb976; --code-fn:#c493b3; --code-num:#82a1c7; --code-str:#9cb485; --code-cm:#9e9a93;
}
/* Paper（tone 260/0/0.9/262，无 tint 中性纸） */
html[data-theme="paper"] {
  --bg-top:#f6f6f7; --bg-mid:#f7f7f8; --bg-low:#f8f8f8; --bg-bottom:#f8f8f8; --bg-tint:transparent;
  --card:rgba(255,255,255,.78); --card-focused:rgba(255,255,255,.97);
  --text:#292929; --text-muted:#7a7a7a; --text-faint:#a1a1a1;
  --accent:#466fbd; --panel-sel:rgba(70,111,189,.12); --selection:rgba(70,111,189,.25); --quote-bar:rgba(70,111,189,.45);
  --danger:#b54e5a; --attention:#a2812d;
  --panel:rgba(255,255,255,.97); --surface-hover:#f2f2f2;
  --code-inline-bg:rgba(41,41,41,.07); --code-block-bg:rgba(41,41,41,.04);
  --code-kw:#b54e5a; --code-ty:#a2812d; --code-fn:#8557a4; --code-num:#216ccc; --code-str:#1f895b; --code-cm:#7a7a7a;
}
html[data-theme="paper"].theme-dark {
  --bg-top:#141416; --bg-mid:#151517; --bg-low:#161618; --bg-bottom:#161618; --bg-tint:rgba(90,120,200,.1);
  --card:rgba(28,28,30,.58); --card-focused:rgba(36,36,39,.95);
  --text:#e4e4e4; --text-muted:#9b9b9b; --text-faint:#787878;
  --accent:#7daafd; --panel-sel:rgba(125,170,253,.18); --selection:rgba(125,170,253,.4); --quote-bar:rgba(125,170,253,.5);
  --danger:#e17c85; --attention:#dcb968;
  --panel:rgba(44,44,44,.97); --surface-hover:#373737;
  --code-inline-bg:rgba(228,228,228,.07); --code-block-bg:rgba(228,228,228,.04);
  --code-kw:#e17c85; --code-ty:#dcb968; --code-fn:#bc8fdb; --code-num:#5c9ffb; --code-str:#61c191; --code-cm:#9b9b9b;
}
/* Plain（tone 260/0/0.9/262，纯灰无渐变差） */
html[data-theme="plain"] {
  --bg-top:#eceef4; --bg-mid:#eceef4; --bg-low:#eceef4; --bg-bottom:#eceef4; --bg-tint:transparent;
  --card:rgba(255,255,255,.78); --card-focused:rgba(255,255,255,.97);
  --text:#292929; --text-muted:#7a7a7a; --text-faint:#a1a1a1;
  --accent:#466fbd; --panel-sel:rgba(70,111,189,.12); --selection:rgba(70,111,189,.25); --quote-bar:rgba(70,111,189,.45);
  --danger:#b54e5a; --attention:#a2812d;
  --panel:rgba(255,255,255,.97); --surface-hover:#f2f2f2;
  --code-inline-bg:rgba(41,41,41,.07); --code-block-bg:rgba(41,41,41,.04);
  --code-kw:#b54e5a; --code-ty:#a2812d; --code-fn:#8557a4; --code-num:#216ccc; --code-str:#1f895b; --code-cm:#7a7a7a;
}
html[data-theme="plain"].theme-dark {
  --bg-top:#141418; --bg-mid:#141418; --bg-low:#141418; --bg-bottom:#141418; --bg-tint:transparent;
  --card:rgba(28,28,30,.58); --card-focused:rgba(36,36,39,.95);
  --text:#e4e4e4; --text-muted:#9b9b9b; --text-faint:#787878;
  --accent:#7daafd; --panel-sel:rgba(125,170,253,.18); --selection:rgba(125,170,253,.4); --quote-bar:rgba(125,170,253,.5);
  --danger:#e17c85; --attention:#dcb968;
  --panel:rgba(44,44,44,.97); --surface-hover:#373737;
  --code-inline-bg:rgba(228,228,228,.07); --code-block-bg:rgba(228,228,228,.04);
  --code-kw:#e17c85; --code-ty:#dcb968; --code-fn:#bc8fdb; --code-num:#5c9ffb; --code-str:#61c191; --code-cm:#9b9b9b;
}
```

## 4. 10 组卡片预览令牌 CSS

预览卡的色属于"卡片自己的主题"，不随窗口主题变、只随窗口深浅变，所以按卡片选择器定义（与第 3 组分离）。插在同一区域。`--pv-dots` 用 8 色 hard-stop 渐变一次给全（原型等价表达；Go 落地按 mojian `PaintPreview` 画 8 个独立 5px 点，gap 3px）。

```css
.thmcard[data-theme="petal"]   { --pv-g1:#f7f2f6; --pv-g2:#f4eff4; --pv-g3:#efeaf1; --pv-g4:#e9e5ed; --pv-card:rgba(255,255,255,.96); --pv-text:#1f1b20; --pv-accent:#a84e7c;
  --pv-dots:linear-gradient(90deg,#000 0 12.5%,#cd3131 0 25%,#00bc00 0 37.5%,#949800 0 50%,#0451a5 0 62.5%,#bc05bc 0 75%,#0598bc 0 87.5%,#555555 0); }
.theme-dark .thmcard[data-theme="petal"]   { --pv-g1:#241d25; --pv-g2:#1f1a21; --pv-g3:#1b181d; --pv-g4:#18151a; --pv-card:rgba(37,35,41,.95); --pv-text:#f3f0f4; --pv-accent:#e887b6;
  --pv-dots:linear-gradient(90deg,#000 0 12.5%,#cd3131 0 25%,#0dbc79 0 37.5%,#e5e510 0 50%,#2472c8 0 62.5%,#bc3fbc 0 75%,#11a8cd 0 87.5%,#e5e5e5 0); }
.thmcard[data-theme="mist"]    { --pv-g1:#e3e9f4; --pv-g2:#ebeff6; --pv-g3:#f1f2f3; --pv-g4:#f2f1ec; --pv-card:rgba(254,255,255,.97); --pv-text:#1f2a38; --pv-accent:#3772bb;
  --pv-dots:linear-gradient(90deg,#1f2a38 0 12.5%,#af545c 0 25%,#30875f 0 37.5%,#9e833c 0 50%,#256ec4 0 62.5%,#835b9f 0 75%,#3a808b 0 87.5%,#969fab 0); }
.theme-dark .thmcard[data-theme="mist"]    { --pv-g1:#0c1628; --pv-g2:#0d1827; --pv-g3:#111b23; --pv-g4:#111b23; --pv-card:rgba(33,36,42,.95); --pv-text:#dee5ef; --pv-accent:#70adfb;
  --pv-dots:linear-gradient(90deg,#363e48 0 12.5%,#db8087 0 25%,#6bbf94 0 37.5%,#d7bb74 0 50%,#5fa1f3 0 62.5%,#b992d5 0 75%,#70bfcc 0 87.5%,#bfc5cc 0); }
.thmcard[data-theme="meadow"]  { --pv-g1:#e2eee5; --pv-g2:#ebf2e7; --pv-g3:#f3f2e2; --pv-g4:#f5f0d6; --pv-card:rgba(254,255,254,.97); --pv-text:#212d1e; --pv-accent:#448335;
  --pv-dots:linear-gradient(90deg,#212d1e 0 12.5%,#b4552b 0 25%,#248b27 0 37.5%,#988629 0 50%,#1776ab 0 62.5%,#945295 0 75%,#2e8478 0 87.5%,#98a296 0); }
.theme-dark .thmcard[data-theme="meadow"]  { --pv-g1:#081c0d; --pv-g2:#101d0c; --pv-g3:#1b1c0b; --pv-g4:#1b1c0b; --pv-card:rgba(34,36,36,.95); --pv-text:#dfe7de; --pv-accent:#7dbf6e;
  --pv-dots:linear-gradient(90deg,#384036 0 12.5%,#df825c 0 25%,#63c561 0 37.5%,#d0bf66 0 50%,#5aa7db 0 62.5%,#cb8acb 0 75%,#66c5b5 0 87.5%,#c0c6bf 0); }
.thmcard[data-theme="dusk"]    { --pv-g1:#e4dbf3; --pv-g2:#efdeee; --pv-g3:#f6e3dc; --pv-g4:#f7e6cf; --pv-card:rgba(255,254,255,.97); --pv-text:#2f2433; --pv-accent:#8e57a5;
  --pv-dots:linear-gradient(90deg,#2f2433 0 12.5%,#c14059 0 25%,#258a41 0 37.5%,#b67702 0 50%,#4d67c1 0 62.5%,#a332b6 0 75%,#008494 0 87.5%,#a39ba7 0); }
.theme-dark .thmcard[data-theme="dusk"]    { --pv-g1:#190f2e; --pv-g2:#260f24; --pv-g3:#2d140b; --pv-g4:#2d140b; --pv-card:rgba(37,35,41,.95); --pv-text:#e8e2ec; --pv-accent:#ca91e3;
  --pv-dots:linear-gradient(90deg,#413a44 0 12.5%,#ed7183 0 25%,#64c376 0 37.5%,#f2af54 0 50%,#7d9af0 0 62.5%,#db74ed 0 75%,#49c4d6 0 87.5%,#c7c2ca 0); }
.thmcard[data-theme="tide"]    { --pv-g1:#dbebef; --pv-g2:#e3eff1; --pv-g3:#ecf3f0; --pv-g4:#f2f2e8; --pv-card:rgba(254,255,255,.97); --pv-text:#162d30; --pv-accent:#008695;
  --pv-dots:linear-gradient(90deg,#162d30 0 12.5%,#b35347 0 25%,#2b885c 0 37.5%,#968737 0 50%,#1b75ad 0 62.5%,#8559a1 0 75%,#00878b 0 87.5%,#91a2a4 0); }
.theme-dark .thmcard[data-theme="tide"]    { --pv-g1:#001b22; --pv-g2:#001e1f; --pv-g3:#0b1e1b; --pv-g4:#0b1e1b; --pv-card:rgba(31,37,40,.95); --pv-text:#dae8e9; --pv-accent:#00c2d2;
  --pv-dots:linear-gradient(90deg,#314042 0 12.5%,#df8073 0 25%,#67c091 0 37.5%,#cebf70 0 50%,#5ca6dd 0 62.5%,#bb90d8 0 75%,#38c7cc 0 87.5%,#bcc6c8 0); }
.thmcard[data-theme="apricot"] { --pv-g1:#f5e1d6; --pv-g2:#f8e7dc; --pv-g3:#f8eedf; --pv-g4:#f6f0da; --pv-card:rgba(255,254,254,.97); --pv-text:#35251b; --pv-accent:#ac571d;
  --pv-dots:linear-gradient(90deg,#35251b 0 12.5%,#d02f00 0 25%,#5a831b 0 37.5%,#b97600 0 50%,#426bbd 0 62.5%,#a44688 0 75%,#14867c 0 87.5%,#a99b94 0); }
.theme-dark .thmcard[data-theme="apricot"] { --pv-g1:#2a1207; --pv-g2:#291507; --pv-g3:#25190a; --pv-g4:#25190a; --pv-card:rgba(38,35,36,.95); --pv-text:#ede2dc; --pv-accent:#ec915c;
  --pv-dots:linear-gradient(90deg,#463a34 0 12.5%,#fd6945 0 25%,#8fbb59 0 37.5%,#f5ae3a 0 50%,#749ded 0 62.5%,#dd80be 0 75%,#55c7ba 0 87.5%,#cbc2bd 0); }
.thmcard[data-theme="aurora"]  { --pv-g1:#dbefe8; --pv-g2:#e1ebf2; --pv-g3:#e8e4f4; --pv-g4:#f0e6f1; --pv-card:rgba(254,255,254,.97); --pv-text:#192e27; --pv-accent:#008966;
  --pv-dots:linear-gradient(90deg,#192e27 0 12.5%,#c44412 0 25%,#009136 0 37.5%,#998700 0 50%,#0077bb 0 62.5%,#944aab 0 75%,#008c82 0 87.5%,#93a39c 0); }
.theme-dark .thmcard[data-theme="aurora"]  { --pv-g1:#001f13; --pv-g2:#06192b; --pv-g3:#1e122a; --pv-g4:#1e122a; --pv-card:rgba(32,37,38,.95); --pv-text:#dbe8e3; --pv-accent:#3bc59e;
  --pv-dots:linear-gradient(90deg,#33413b 0 12.5%,#f0754e 0 25%,#15cb6d 0 37.5%,#d2c049 0 50%,#38a9ea 0 62.5%,#ca84e1 0 75%,#00cec1 0 87.5%,#bdc7c3 0); }
.thmcard[data-theme="dune"]    { --pv-g1:#ede4d8; --pv-g2:#f1e9de; --pv-g3:#f4eee4; --pv-g4:#f3efe8; --pv-card:rgba(255,255,254,.97); --pv-text:#2e281d; --pv-accent:#996700;
  --pv-dots:linear-gradient(90deg,#2e281d 0 12.5%,#a0624c 0 25%,#667d50 0 37.5%,#a1813f 0 50%,#517198 0 62.5%,#8d5d7d 0 75%,#537e76 0 87.5%,#a39e95 0); }
.theme-dark .thmcard[data-theme="dune"]    { --pv-g1:#22190b; --pv-g2:#211a0e; --pv-g3:#201b12; --pv-g4:#201b12; --pv-card:rgba(37,35,36,.95); --pv-text:#e8e4dd; --pv-accent:#d5a13c;
  --pv-dots:linear-gradient(90deg,#413c35 0 12.5%,#cb8e77 0 25%,#9cb485 0 37.5%,#dbb976 0 50%,#82a1c7 0 62.5%,#c493b3 0 75%,#8bbdb3 0 87.5%,#c7c4be 0); }
.thmcard[data-theme="paper"]   { --pv-g1:#f6f6f7; --pv-g2:#f7f7f8; --pv-g3:#f8f8f8; --pv-g4:#f8f8f8; --pv-card:rgba(255,255,255,.97); --pv-text:#292929; --pv-accent:#466fbd;
  --pv-dots:linear-gradient(90deg,#292929 0 12.5%,#b54e5a 0 25%,#1f895b 0 37.5%,#a2812d 0 50%,#216ccc 0 62.5%,#8557a4 0 75%,#2d818e 0 87.5%,#9e9e9e 0); }
.theme-dark .thmcard[data-theme="paper"]   { --pv-g1:#141416; --pv-g2:#151517; --pv-g3:#161618; --pv-g4:#161618; --pv-card:rgba(36,36,39,.95); --pv-text:#e4e4e4; --pv-accent:#7daafd;
  --pv-dots:linear-gradient(90deg,#3d3d3d 0 12.5%,#e17c85 0 25%,#61c191 0 37.5%,#dcb968 0 50%,#5c9ffb 0 62.5%,#bc8fdb 0 75%,#65c1d0 0 87.5%,#c4c4c4 0); }
.thmcard[data-theme="plain"]   { --pv-g1:#eceef4; --pv-g2:#eceef4; --pv-g3:#eceef4; --pv-g4:#eceef4; --pv-card:rgba(255,255,255,.97); --pv-text:#292929; --pv-accent:#466fbd;
  --pv-dots:linear-gradient(90deg,#292929 0 12.5%,#b54e5a 0 25%,#1f895b 0 37.5%,#a2812d 0 50%,#216ccc 0 62.5%,#8557a4 0 75%,#2d818e 0 87.5%,#9e9e9e 0); }
.theme-dark .thmcard[data-theme="plain"]   { --pv-g1:#141418; --pv-g2:#141418; --pv-g3:#141418; --pv-g4:#141418; --pv-card:rgba(36,36,39,.95); --pv-text:#e4e4e4; --pv-accent:#7daafd;
  --pv-dots:linear-gradient(90deg,#3d3d3d 0 12.5%,#e17c85 0 25%,#61c191 0 37.5%,#dcb968 0 50%,#5c9ffb 0 62.5%,#bc8fdb 0 75%,#65c1d0 0 87.5%,#c4c4c4 0); }
```

## 5. 原型修改清单（main-mojian.html，精确到锚点）

1. L52 `<html lang="zh-CN" class="theme-light">`：加 `data-theme="petal"`。
2. 第 0 节令牌（L74 `:root {` 至 L234）：`:root` 删去颜色变量（bg-*/card*/text*/hover/pressed/accent/track*/danger/attention/panel*/surface-hover/selection/code-*/rule/quote-bar），只留字号/字重/圆角/节奏/尺寸与 `--match-bg/--scrollbar`；`.theme-dark` 同样只留非主题色差（本例仅 `--match-bg/--scrollbar` 与 body 桌面底色 L236 相关量）。颜色兜底：在 20 组之前加注释说明"颜色由 html[data-theme] 提供"。
3. 语法高亮（L509-514）：`.kw/.ty/.fn/.num/.str/.cm` 改为 `color: var(--code-kw)` 等 6 个变量，删除全部 `.theme-dark .kw {...}` 硬编码对。petal 的变量值 = 原 GitHub 色字面量，视觉零回归。
4. 其他 `.theme-dark X` 规则（L236 body、L355-356 sun/moon）保留不动，它们属于明暗维度。
5. region: titlebar / `.titlebar-tools`（L693）：在 `#themeToggle` 按钮之前插入调色板按钮 + Popover 包裹层（结构见第 6/7 节）。palette 按钮 `.ibtn.lg` 规格，SVG 24x24 stroke 2：`<path d="M12 3a9 9 0 1 0 .4 18H14a2.5 2.5 0 0 0 0-5h-1a2 2 0 0 1 0-4h6.2A3.8 3.8 0 0 0 23 8.2C22.2 5 17.6 3 12 3z"/>` + 三个 `fill="currentColor" stroke="none"` 小圆 `<circle cx="7.5" cy="10.5" r="1"/><circle cx="11.5" cy="7" r="1"/><circle cx="15.5" cy="8.5" r="1"/>`。
6. 新增 CSS 第 7 节（`.sf-hit` 规则之后、`@media` 之前）：`.palette-wrap/.thmcard/.thmgrid/.theme-popover` 全部样式（见第 6/7 节）。
7. region: window 内、`.searchfloat`（L819）之后：不加静态 popover DOM，popover 放在 5 的包裹层里随按钮定位。
8. `<script>`（L874-881）：保留现有 themeToggle 明暗切换；追加 THEME Popover 逻辑（见第 8 节）。
9. 文件头注释（L14-52 实现对照表）追加一条：标题栏 palette 按钮 + THEME popover -> internal/shell/titlebar.go（Go 侧为 theme.Choices 网格，颜色源 mojian themes.go Window/Terminal）。

## 6. 主题卡片：单张模板与网格 CSS

10 张按 Choices 顺序生成，仅 `data-theme` 与名字不同：

```html
<button class="thmcard on" data-theme="petal" role="radio" aria-checked="true" title="Petal">
  <span class="thm-prev" aria-hidden="true">
    <span class="thm-win">
      <span class="thm-aa">Aa</span>
      <span class="thm-line"><i class="thm-caret"></i></span>
      <span class="thm-dots"></span>
    </span>
  </span>
  <span class="thm-name">Petal</span>
</button>
```

```css
.thmgrid { display: grid; grid-template-columns: repeat(5, 1fr); gap: 8px; }  /* 2 行 x 5 列 */
.thmcard {
  display: flex; flex-direction: column; align-items: stretch; gap: 6px;
  padding: 6px 6px 5px; border: none; background: transparent; cursor: pointer;
  border-radius: var(--radius); color: var(--text-muted); font: inherit;
  transition: transform .15s, box-shadow .15s;
}
.thmcard:hover { transform: translateY(-1px); }
.thmcard:hover .thm-prev { box-shadow: 0 4px 12px rgba(0,0,0,.18); }
.thm-prev {
  height: 64px; border-radius: 7px; overflow: hidden; display: block; padding: 7px;
  background:
    linear-gradient(90deg, transparent 35%, var(--pv-tint, transparent)),
    linear-gradient(180deg, var(--pv-g1) 0%, var(--pv-g2) 38%, var(--pv-g3) 70%, var(--pv-g4) 100%);
}
.thm-win {
  display: block; height: 100%; padding: 5px 6px; border-radius: 5px;
  background: var(--pv-card); box-shadow: 0 1px 2px rgba(0,0,0,.10);
  position: relative;
}
.thm-aa  { font-size: 10px; font-weight: 600; color: var(--pv-text); line-height: 1; display: block; }
.thm-line { display: block; height: 4px; margin-top: 5px; width: 62%; border-radius: 2px;
  background: var(--pv-text); opacity: .22; position: relative; }   /* 一行假文 */
.thm-caret { position: absolute; left: 100%; top: -2px; width: 2px; height: 8px;
  border-radius: 1px; background: var(--pv-accent); opacity: 1; }   /* 光标竖条，对照 PaintPreview */
.thm-dots { display: block; height: 5px; border-radius: 2.5px; margin-top: 8px;
  background-image: var(--pv-dots); }
.thm-name { font-size: 11px; font-weight: 400; text-align: center; line-height: 1; }
.thmcard.on { color: var(--text); }
.thmcard.on .thm-prev { outline: 2px solid var(--accent); outline-offset: -1px; }  /* 选中描边 = 当前主题 accent */
.thmcard.on .thm-name { font-weight: 600; }
.thmcard.on::after { content: ""; position: absolute; }  /* 可选：不加勾，描边 + 名字加重已足够 */
```

说明：选中描边用窗口当前 `--accent`（mojian 语义：选中态永远用当前窗口的 accent），不是卡片自己的 `--pv-accent`，这样 10 张卡里选中的那张描边色与其余 UI 一致。

## 7. Popover 结构与 CSS

锚定方案：按钮和浮层包一层，自动右对齐，无需手算偏移：

```html
<div class="palette-wrap">
  <button class="ibtn lg" id="paletteBtn" title="主题色" aria-haspopup="true" aria-expanded="false" aria-controls="themePopover">
    <!-- 第 5 节的调色板 SVG -->
  </button>
  <div class="theme-popover" id="themePopover" role="listbox" aria-label="THEME">
    <div class="tree-group" style="padding:10px 12px 4px;">THEME</div>  <!-- 10px/650 大写 TextFaint，与侧栏分组标签同规格 -->
    <div class="thmgrid">
      <!-- 第 6 节卡片 x10 -->
    </div>
  </div>
</div>
```

```css
.palette-wrap { position: relative; flex: none; }
.theme-popover {
  position: absolute; top: calc(100% + 8px); right: 0; z-index: 60;  /* 右缘对齐按钮右缘 */
  width: 536px;                                                      /* 5x96 + 4x8 + padding 2x12 */
  padding: 2px 12px 12px;
  background: var(--panel); border: 1px solid var(--panel-border);
  border-radius: var(--radius); box-shadow: var(--shadow-dialog);
  display: none;
}
.theme-popover.open { display: block; }
.theme-popover .thmgrid { margin-top: 6px; }
```

面板/描边/阴影全部走 `--panel/--panel-border/--shadow-dialog`，Popover 内深浅自动跟随当前模式（同一套变量在深色下翻转），卡片预览色由第 4 节的 `.theme-dark .thmcard[...]` 覆盖组跟随。

## 8. 交互细节

```js
var palBtn = document.getElementById('paletteBtn');
var pop = document.getElementById('themePopover');
palBtn.addEventListener('click', function (e) {
  e.stopPropagation();
  var open = pop.classList.toggle('open');
  palBtn.setAttribute('aria-expanded', open);
});
pop.addEventListener('click', function (e) {
  var c = e.target.closest('.thmcard');
  if (!c) return;
  document.documentElement.setAttribute('data-theme', c.dataset.theme);  // 即时切换，不关浮层
  pop.querySelectorAll('.thmcard').forEach(function (x) {
    var on = x === c;
    x.classList.toggle('on', on);
    x.setAttribute('aria-checked', on);
  });
});
document.addEventListener('keydown', function (e) {
  if (e.key === 'Escape' && pop.classList.contains('open')) {
    pop.classList.remove('open');
    palBtn.setAttribute('aria-expanded', 'false');
  }
});
document.addEventListener('click', function (e) {
  if (pop.classList.contains('open') && !pop.contains(e.target) && !palBtn.contains(e.target)) {
    pop.classList.remove('open');
    palBtn.setAttribute('aria-expanded', 'false');
  }
});
```

要点：点击卡片只换 `data-theme` 并更新 `.on`，浮层不关（用户要连续试色）；Esc 与浮层外点击关闭；选中标识 = 2px accent 描边 + 卡名 600（不再画勾，描边即 mojian 选中语义，勾反而和 Go 落地的 PanelSel 行为不一致）；hover = 卡片上浮 1px + 预览投影加深。

## 9. 正交关系与守恒项

- 明暗 = `html.theme-dark` class（现有 sun/moon 按钮、设置弹窗 Theme 行不变）；主题色 = `html[data-theme]` 属性。两者任意组合有效，20 组 CSS 已覆盖全部组合。
- 初始态 = `theme-light` + `data-theme="petal"`，与当前原型渲染逐像素等价（petal 就是原型现状的颜色），本次改动对默认视觉零回归。
- 桌面底色（`.theme-dark body` L236）属于明暗维度，不随主题色变。
