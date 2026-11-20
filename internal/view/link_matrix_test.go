package view

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/doc"
)

// linkScenarios are the block shapes a link can sit in. Each one draws its
// link somewhere else: a paragraph, a heading at its level's size, a list
// item's leaf, a quote's, a table cell at the table's size, a check item's,
// and the real ticket's list item, whose inline code chip before the link
// is what drifted the rune offset from the drawn glyphs.
var linkScenarios = []struct {
	name string
	src  string
	want []string
}{
	{
		name: "paragraph",
		src:  "阅读 [第一份文档](docs/one.md) 与 [the second guide](https://example.com/guide) 以及 [the notes](notes.md) 三段。\n",
		want: []string{"docs/one.md", "https://example.com/guide", "notes.md"},
	},
	{
		name: "heading",
		src:  "## Read [the guide](https://example.com/h) before the [appendix](appendix.md)\n\nbody\n",
		want: []string{"https://example.com/h", "appendix.md"},
	},
	{
		name: "list first",
		src:  "- Read [the guide](https://example.com/l1) now.\n- another item\n",
		want: []string{"https://example.com/l1"},
	},
	{
		name: "list second",
		src:  "- first item\n- Read [the guide](https://example.com/l2) now.\n",
		want: []string{"https://example.com/l2"},
	},
	{
		name: "nested list",
		src:  "- outer\n  - Read [the guide](https://example.com/l3) now.\n    - deeper [the notes](notes.md)\n",
		want: []string{"https://example.com/l3", "notes.md"},
	},
	{
		name: "task list",
		src:  "- [ ] Read [the guide](https://example.com/tl) now.\n",
		want: []string{"https://example.com/tl"},
	},
	{
		name: "quote",
		src:  "> Read [the guide](https://example.com/q) and [the notes](notes.md) now.\n",
		want: []string{"https://example.com/q", "notes.md"},
	},
	{
		name: "table cell",
		src:  "| a | b |\n| --- | --- |\n| x | [the guide](https://example.com/t) |\n| [清单](notes.md) | y |\n",
		want: []string{"https://example.com/t", "notes.md"},
	},
	{
		name: "inline code before the link",
		src:  "# v0.3.0 验收矩阵\n\n- Go 基线：HEAD `95ef5a3`；Rust 对照源：[C:/Seafile/Works/rust/peekdown](C:/Seafile/Works/rust/peekdown)（README）。\n",
		want: []string{"C:/Seafile/Works/rust/peekdown"},
	},
	{
		name: "bold label",
		src:  "Read **[the guide](https://example.com/b)** now.\n",
		want: []string{"https://example.com/b"},
	},
	{
		name: "anchor and mailto",
		src:  "Jump [to details](#the-details), mail [the team](mailto:team@example.com).\n\n## The Details\n\nbody\n",
		want: []string{"#the-details", "mailto:team@example.com"},
	},
}

// LinkRects is the matrix's source of click points: the boxes the render
// itself recorded, so a point it hands back is a point the user sees.
func TestLinkMatrixCoversEveryScenario(t *testing.T) {
	for _, tc := range linkScenarios {
		t.Run(tc.name, func(t *testing.T) {
			_, tt, st, _, _ := localLinkDoc(t, tc.src, 420, 400)
			rects := st.LinkRects()
			if len(rects) == 0 {
				t.Fatalf("no link rects for %q", tc.src)
			}
			for _, dest := range tc.want {
				if !hasRect(rects, dest) {
					t.Fatalf("no rect for %q in %+v", dest, rects)
				}
			}
			// Every rect must sit in the box the engine laid the link's
			// inline element out in: the two geometries, shaped and laid,
			// have to agree or the click lands where nothing is drawn.
			for _, r := range rects {
				if !insideLinkBox(t, tt, st, r) {
					t.Fatalf("rect %+v is outside the engine's box for %q", r.Rect, r.Dest)
				}
			}
		})
	}
}

// TestLinkHitEveryX is the drift's sentinel: a link's every drawn column
// must answer with that link's destination, and never with another's.
func TestLinkHitEveryX(t *testing.T) {
	for _, tc := range linkScenarios {
		t.Run(tc.name, func(t *testing.T) {
			_, tt, st, _, _ := localLinkDoc(t, tc.src, 420, 400)
			for _, r := range st.LinkRects() {
				// Every column of the words: a rune's width is a few
				// DIPs, so a DIP is finer than the glyphs.
				for x := r.Rect.X + 0.5; x < r.Rect.X+r.Rect.W; x++ {
					for _, fy := range []float32{0.2, 0.5, 0.8} {
						y := r.Rect.Y + r.Rect.H*fy
						got, how := st.linkHitAt(ui.Point{X: x, Y: y})
						if got != r.Dest {
							t.Fatalf("point %.1f,%.1f (%s) = %q, want %q", x, y, how, got, r.Dest)
						}
					}
				}
			}
			_ = tt
		})
	}
}

// TestLinkClickEveryRect drives a whole press and release at each rect's
// middle through the tester: the handler's own path, from the pointer to
// the shell's callback.
func TestLinkClickEveryRect(t *testing.T) {
	for _, tc := range linkScenarios {
		t.Run(tc.name, func(t *testing.T) {
			_, tt, st, opened, local := localLinkDoc(t, tc.src, 420, 400)
			for _, r := range st.LinkRects() {
				if strings.HasPrefix(r.Dest, "#") {
					// An anchor scrolls its heading in, which the view's own
					// test covers; the shell is never handed it.
					continue
				}
				*opened, *local = nil, nil
				tt.ClickAt(r.Rect.X+r.Rect.W/2, r.Rect.Y+r.Rect.H/2)
				if got := lastDest(*opened, *local); got != r.Dest {
					t.Fatalf("click %.1f,%.1f opened %q, want %q (web %v local %v)",
						r.Rect.X+r.Rect.W/2, r.Rect.Y+r.Rect.H/2, got, r.Dest, *opened, *local)
				}
			}
		})
	}
}

// A press, a small move and the release still open the link the press
// landed on: the drag stays under the click's threshold.
func TestLinkClickWithSmallMoveOpens(t *testing.T) {
	_, tt, st, opened, local := localLinkDoc(t, "See [the guide](https://example.com/guide) now.\n", 400, 300)
	r := firstRect(t, st, "https://example.com/guide")
	x, y := r.Rect.X+r.Rect.W/2, r.Rect.Y+r.Rect.H/2
	tt.Press(x, y)
	tt.Move(x+2, y)
	tt.Frame()
	tt.Release(x+2, y)
	tt.Frame()
	if got := lastDest(*opened, *local); got != "https://example.com/guide" {
		t.Fatalf("opened %v local %v, want the link", *opened, *local)
	}
}

// A press that moves past the threshold selects instead of opening: the
// rects must not turn a drag into a click.
func TestLinkDragPastThresholdDoesNotOpen(t *testing.T) {
	_, tt, st, opened, local := localLinkDoc(t, "See [the guide](https://example.com/guide) now.\n\nSecond paragraph.\n", 400, 300)
	r := firstRect(t, st, "https://example.com/guide")
	x, y := r.Rect.X+r.Rect.W/2, r.Rect.Y+r.Rect.H/2
	tt.Press(x, y)
	tt.Move(x, y+20)
	tt.Frame()
	tt.Release(x, y+20)
	tt.Frame()
	if len(*opened) != 0 || len(*local) != 0 {
		t.Fatalf("opened %v local %v, a drag must not open a link", *opened, *local)
	}
}

// The pointer takes the hand over a link's words and the I-beam over the
// text around them: the cursor is the engine's read of the inline
// elements the render built, not a shape guessed from a rune offset.
func TestLinkCursorFollowsTheWords(t *testing.T) {
	_, tt, st, _, _ := localLinkDoc(t, "Read [the guide](https://example.com/guide) here.\n", 400, 300)
	r := firstRect(t, st, "https://example.com/guide")
	tt.Move(r.Rect.X+r.Rect.W/2, r.Rect.Y+r.Rect.H/2)
	if got := tt.Cursor(); got != ui.CursorPointer {
		t.Fatalf("cursor over the link = %v, want the hand", got)
	}
	// The row starts at the column's left edge: a point past the link's
	// words is the prose after it.
	tt.Move(r.Rect.X+r.Rect.W+8, r.Rect.Y+r.Rect.H/2)
	if got := tt.Cursor(); got != ui.CursorText {
		t.Fatalf("cursor over the prose = %v, want the I-beam", got)
	}
	tt.Move(r.Rect.X+r.Rect.W/2, r.Rect.Y+r.Rect.H/2)
	if got := tt.Cursor(); got != ui.CursorPointer {
		t.Fatalf("cursor back over the link = %v, want the hand", got)
	}
}

// A link longer than the column wraps: each line of it is a rect of its
// own, on its own line, and every column of every line opens it.
func TestWrappedLinkHasOneRectPerLine(t *testing.T) {
	src := "Lorem ipsum dolor sit amet consectetur adipiscing elit sed do eiusmod tempor incididunt ut labore. Read [the guide to everything that follows here](https://example.com/wrapped) and more text follows to wrap.\n"
	_, tt, st, _, _ := localLinkDoc(t, src, 420, 400)
	rects := linkRectsOf(st, "https://example.com/wrapped")
	if len(rects) < 2 {
		t.Fatalf("wrapped link has %d rects, want one per line: %+v", len(rects), rects)
	}
	lines := map[float32]bool{}
	for _, r := range rects {
		lines[r.Rect.Y] = true
	}
	if len(lines) != len(rects) {
		t.Fatalf("rects %+v share a line", rects)
	}
	for _, r := range rects {
		for x := r.Rect.X + 0.5; x < r.Rect.X+r.Rect.W; x++ {
			if got, _ := st.linkHitAt(ui.Point{X: x, Y: r.Rect.Y + r.Rect.H/2}); got != "https://example.com/wrapped" {
				t.Fatalf("point %.1f,%.1f = %q, want the wrapped link", x, r.Rect.Y+r.Rect.H/2, got)
			}
		}
	}
	_ = tt
}

// The ticket's own layout: a sidebar and an outline take the window's
// edges, so the document's list starts 331 DIPs in, the place the field
// log reported, and the links must still open from their own columns.
func TestLinkHitWithSidebarAndOutline(t *testing.T) {
	src := "# v0.3.0 验收矩阵\n\n- Go 基线：HEAD `95ef5a3`；Rust 对照源：[C:/Seafile/Works/rust/peekdown](C:/Seafile/Works/rust/peekdown)（README）。\n"
	m := doc.New(src)
	var opened, local []string
	st := &State{AnchorOf: anchorsOf(m)}
	st.OpenURL = func(url string) { opened = append(opened, url) }
	st.OpenLocalLink = func(dest string) { local = append(local, dest) }
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		ui.Row(c).Grow(1).Children(func() {
			ui.Column(c).Width(331).Children(func() { ui.Text(c, "Files") })
			ui.Row(c).Grow(1).Children(func() { ScrollView(c, m, st) })
			ui.Column(c).Width(288).Children(func() { ui.Text(c, "Outline") })
		})
	}, 1400, 900)
	tt.Frame()
	rects := st.LinkRects()
	r := firstRect(t, st, "C:/Seafile/Works/rust/peekdown")
	if r.Rect.X < 331 {
		t.Fatalf("link starts at %.1f, want it inside the document column past the sidebar", r.Rect.X)
	}
	for _, r := range rects {
		if !insideLinkBox(t, tt, st, r) {
			t.Fatalf("rect %+v is outside the engine's box for %q", r.Rect, r.Dest)
		}
		for x := r.Rect.X + 0.5; x < r.Rect.X+r.Rect.W; x++ {
			if got, how := st.linkHitAt(ui.Point{X: x, Y: r.Rect.Y + r.Rect.H/2}); got != r.Dest {
				t.Fatalf("point %.1f,%.1f (%s) = %q, want %q", x, r.Rect.Y+r.Rect.H/2, how, got, r.Dest)
			}
		}
		opened, local = nil, nil
		tt.ClickAt(r.Rect.X+r.Rect.W/2, r.Rect.Y+r.Rect.H/2)
		if got := lastDest(opened, local); got != r.Dest {
			t.Fatalf("click at %.1f,%.1f opened %q, want %q", r.Rect.X+r.Rect.W/2, r.Rect.Y+r.Rect.H/2, got, r.Dest)
		}
	}
}

// The shapes hold at the scales a window reports: the rects are window
// DIPs, so the same document must answer the same way at 1.5.
func TestLinkHitAtScale(t *testing.T) {
	for _, scale := range []float32{1, 1.5} {
		t.Run(strings.ReplaceAll(scaleString(scale), ".", "_"), func(t *testing.T) {
			src := "阅读 [第一份文档](docs/one.md) 与 [the guide](https://example.com/guide) 两段。\n"
			m := doc.New(src)
			var opened, local []string
			st := &State{AnchorOf: anchorsOf(m)}
			st.OpenURL = func(url string) { opened = append(opened, url) }
			st.OpenLocalLink = func(dest string) { local = append(local, dest) }
			tt := ui.NewTester(func(c *ui.Context) {
				c.DisableAnimations()
				ScrollView(c, m, st)
			}, 420, 400)
			tt.SetScale(scale)
			tt.Frame()
			rects := st.LinkRects()
			if len(rects) < 2 {
				t.Fatalf("scale %v: rects %+v", scale, rects)
			}
			for _, r := range rects {
				for x := r.Rect.X + 0.5; x < r.Rect.X+r.Rect.W; x++ {
					if got, _ := st.linkHitAt(ui.Point{X: x, Y: r.Rect.Y + r.Rect.H/2}); got != r.Dest {
						t.Fatalf("scale %v: point %.1f,%.1f = %q, want %q", scale, x, r.Rect.Y+r.Rect.H/2, got, r.Dest)
					}
				}
				opened, local = nil, nil
				tt.ClickAt(r.Rect.X+r.Rect.W/2, r.Rect.Y+r.Rect.H/2)
				if got := lastDest(opened, local); got != r.Dest {
					t.Fatalf("scale %v: click opened %q, want %q", scale, got, r.Dest)
				}
			}
		})
	}
}

func scaleString(s float32) string {
	if s == float32(int(s)) {
		return string(rune('0' + int(s)))
	}
	return string(rune('0'+int(s))) + "." + string(rune('0'+int(s*10)%10))
}

// hasRect reports whether the frame drew a rect for dest.
func hasRect(rects []LinkRect, dest string) bool {
	for _, r := range rects {
		if r.Dest == dest {
			return true
		}
	}
	return false
}

// linkRectsOf returns the frame's rects for one destination.
func linkRectsOf(st *State, dest string) []LinkRect {
	var out []LinkRect
	for _, r := range st.LinkRects() {
		if r.Dest == dest {
			out = append(out, r)
		}
	}
	return out
}

// firstRect returns the frame's first rect for dest.
func firstRect(t *testing.T, st *State, dest string) LinkRect {
	t.Helper()
	rects := linkRectsOf(st, dest)
	if len(rects) == 0 {
		t.Fatalf("no rect for %q in %+v", dest, st.LinkRects())
	}
	return rects[0]
}

// insideLinkBox reports whether a shaped rect sits in the box the engine
// laid the link's inline element out in, give or take the antialiasing of
// a glyph's edge: the shaped boxes come from the same spans, so the two
// have to agree.
func insideLinkBox(t *testing.T, tt *ui.Tester, st *State, r LinkRect) bool {
	t.Helper()
	const slack = 2
	for _, para := range st.linkParas {
		for _, seg := range para.segs {
			if seg.dest != r.Dest {
				continue
			}
			b := seg.h.Bounds(st.ctx)
			if b.W <= 0 || b.H <= 0 {
				continue
			}
			if r.Rect.X >= b.X-slack && r.Rect.X+r.Rect.W <= b.X+b.W+slack &&
				r.Rect.Y >= b.Y-slack && r.Rect.Y+r.Rect.H <= b.Y+b.H+slack {
				return true
			}
			t.Logf("rect %+v vs engine box %+v for %q", r.Rect, b, r.Dest)
		}
	}
	return false
}

// lastDest returns the destination the shell was handed last.
func lastDest(opened, local []string) string {
	if len(local) > 0 {
		return local[len(local)-1]
	}
	if len(opened) > 0 {
		return opened[len(opened)-1]
	}
	return ""
}
