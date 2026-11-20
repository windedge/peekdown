package shell

import "github.com/egoist/mygo/ui"

// paneGutter 是一条分栏缝隙在分隔线两侧各留的空隙: mygo 的 Split 把
// 1px 常显分隔线画在 pane 边界上, 两侧 pane 各留这么多 padding, 缝隙
// 就是 8 | 1 | 8, 分隔线落在正中。
const paneGutter = float32(8)

// gripHotspotHalf 是悬停热区在线两侧各伸展的宽度, 拼出 9px 的热区,
// 与 mygo 原生拖拽把手的大部分可达范围重合。
const gripHotspotHalf = float32(4)

// coverNativeDivider 在 x 处重画一列 1px 宽的窗口背景, 盖掉 mygo Split
// 画在 pane 边界上的常显分隔线。root 是根背景列的分段与范围: 三段竖向
// 渐变用它自己的 Y 和 H 分段, 一列宽的渐变在每个 y 上与整窗的取值相同;
// 横向 tint 在固定 x 上只有 alpha 一个变量, 取该处的常量。
func coverNativeDivider(p *ui.Painter, x float32, root ui.Rect, k AccentColors) {
	h1, h2 := root.H*0.38, root.H*0.32
	p.FillGradient(ui.Rect{X: x - 1, Y: root.Y, W: 3, H: h1 + 1},
		ui.LinearGradient{From: k.BgTop, To: k.BgMid, Angle: 180}, 0)
	p.FillGradient(ui.Rect{X: x - 1, Y: root.Y + h1, W: 3, H: h2 + 1},
		ui.LinearGradient{From: k.BgMid, To: k.BgLow, Angle: 180}, 0)
	p.FillGradient(ui.Rect{X: x - 1, Y: root.Y + h1 + h2, W: 3, H: root.H - h1 - h2},
		ui.LinearGradient{From: k.BgLow, To: k.BgBottom, Angle: 180}, 0)
	coverTintColumn(p, x, root, k)
}

// coverTintColumn 涂横向 tint 在遮盖列上的取值: tint 是从窗口 35% 宽到
// 右缘的横向渐变, 色相固定, 只有 alpha 沿 x 走, 所以一列内是一个常数
// alpha 的纯色, 与 paintAccentBackground 里 tint 的 Start/End 对应。
// 遮盖带 coverNativeDivider 画的是 x-1..x+1 三列, tint 要逐列还原:
// 只涂中间列会把两侧列留在无 tint 的浅渐变上, 在 tint 有色区 (右缝隙)
// 露出两条过亮的竖线。
func coverTintColumn(p *ui.Painter, x float32, root ui.Rect, k AccentColors) {
	if k.BgTint.A == 0 {
		return
	}
	for cx := x - 1; cx <= x+1; cx++ {
		t := float32(0)
		if root.W > 0 {
			t = (cx + 0.5 - root.X) / root.W
		}
		tint := k.BgTint
		tint.A = uint8(float32(k.BgTint.A) * min(max((t-0.35)/0.65, 0), 1))
		if tint.A <= 0 {
			continue
		}
		p.Fill(ui.Rect{X: cx, Y: root.Y, W: 1, H: root.H}, tint, 0)
	}
}

// paneGrip 挂在 Split 自己的 DrawOver 上: 子 pane (连同原生分隔线) 画完
// 后, 先用一列渐变盖掉贴边的常显线, 悬停时再在缝隙正中画 1px 强调线。
// side 0 是左栏 (文件 pane 右边界), side 1 是右大纲 (RTL split, 第一 pane
// 左边界在 split 右缘往左一个 pane 宽处)。
func (s *Shell) paneGrip(c *ui.Context, p *ui.Painter, r ui.Rect, side int, filesW, outlineW float32) {
	k := s.accentColors(c)
	var x float32
	if side == 0 {
		x = r.X + filesW
	} else {
		x = r.X + r.W - outlineW
	}
	coverNativeDivider(p, x, s.bgRect, k)
	if side == 0 && outlineW > 0 {
		// The outer split paints its Border line after the inner split's
		// DrawOver already ran, so the outline boundary needs a second
		// cover from this outer pass.
		coverNativeDivider(p, r.X+r.W-outlineW, s.bgRect, k)
	}
	if s.gripHover[side] {
		// Centered in the gutter: the gutter runs paneGutter wide from the
		// pane boundary, so its middle sits paneGutter/2 into it.
		cx := x + paneGutter/2
		if side != 0 {
			cx = x - paneGutter/2
		}
		p.Fill(ui.Rect{X: cx, Y: r.Y, W: 1, H: r.H}, k.Accent, 0)
	}
}

// paneGripHotspots 放两条不可见的绝对定位热区盖住缝隙: Hovered 让悬停
// 变化触发重建, 重建里读到的悬停状态存进 gripHover 供 paneGrips 画悬停
// 线。热区盖住了 mygo 的拖拽把手 (hitChain 不穿透), 所以拖拽也由热区自
// 持: 左线拖动宽窄同向, 右线在大纲上反向 (RTL split), 宽度都走现有
// Set*Width 与防抖保存路径。返回这一帧是否有拖动。
func (s *Shell) paneGripHotspots(c *ui.Context, areaW, filesW, outlineW float32, filesMoving, outlineMoving bool) bool {
	s.gripHover[0], s.gripHover[1] = false, false
	moved := false
	if filesW > 0 && !filesMoving {
		e := ui.Box(c).Absolute().Left(filesW - gripHotspotHalf).
			Width(2*gripHotspotHalf + 1).Top(0).Bottom(0).Cursor(ui.CursorResizeEW)
		s.gripHover[0] = e.Hovered()
		if dx, _, ok := e.Dragged(); ok && dx != 0 {
			s.SetSidebarWidth(s.Sidebar.Width + dx)
			s.Sidebar.slide.snap(s.Sidebar.Width)
			s.gripHover[0] = true
			moved = true
		}
	}
	winW, _ := c.Size()
	if outlineW > 0 && !outlineMoving && winW >= outlineMinWindowWidth {
		x := areaW - outlineW - 1
		e := ui.Box(c).Absolute().Left(x - gripHotspotHalf).
			Width(2*gripHotspotHalf + 1).Top(0).Bottom(0).Cursor(ui.CursorResizeEW)
		s.gripHover[1] = e.Hovered()
		if dx, _, ok := e.Dragged(); ok && dx != 0 {
			// RTL split: 手柄左移 (dx<0) 加宽大纲。
			s.SetOutlineWidth(s.Outline.Width - dx)
			s.Outline.slide.snap(s.Outline.Width)
			s.gripHover[1] = true
			moved = true
		}
	}
	return moved
}
