package view

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/doc"
	"github.com/windedge/peekdown/internal/selection"
)

// dragFromTo presses at (x0, y0), moves and releases at (x1, y1), the way a
// user drags a selection.
func dragFromTo(tt *ui.Tester, x0, y0, x1, y1 float32) {
	tt.Press(x0, y0)
	steps := 20
	for i := 1; i <= steps; i++ {
		tt.Move(x0+(x1-x0)*float32(i)/float32(steps), y0+(y1-y0)*float32(i)/float32(steps))
	}
	tt.Release(x1, y1)
}

func findText(t *testing.T, tt *ui.Tester, want string) (float32, float32) {
	t.Helper()
	for _, label := range tt.Texts() {
		if strings.Contains(label, want) {
			r, ok := tt.Find(label)
			if !ok {
				continue
			}
			return r.X + 2, r.Y + r.H/2
		}
	}
	t.Fatalf("texts %v: %q not found", tt.Texts(), want)
	return 0, 0
}

func TestPointerDragSelectsAcrossBlocks(t *testing.T) {
	src := "First paragraph.\n\nSecond paragraph.\n\nThird paragraph.\n"
	m := doc.New(src)
	var state State
	var tt *ui.Tester
	tt = ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		ui.List(c, &state.List, m.Len(), func(r int) {
			block := m.RowBlock(r).Index
			el := ui.Row(c)
			el.HandleInput(func(ev ui.InputEvent) bool {
				switch ev.Kind {
				case ui.InputPointerDown:
					if ev.Button == 0 {
						state.Press(m, block, ev)
						return true
					}
				case ui.InputPointerMove:
					if state.Dragging {
						state.Drag(m, block, ev)
						return true
					}
				case ui.InputPointerUp:
					if ev.Button == 0 && state.Dragging {
						state.Drag(m, block, ev)
						state.Dragging = false
						return true
					}
				}
				return false
			})
			el.Children(func() {
				Render(c, m, &state, block)
			})
		}).Grow(1)
	}, 400, 300)

	x0, y0 := findText(t, tt, "First")
	x1, y1 := findText(t, tt, "Third")
	dragFromTo(tt, x0, y0, x1, y1)

	got := state.SelectionText(m)
	want := "First paragraph.\nSecond paragraph.\nThird paragraph."
	if got != want {
		t.Fatalf("selected %q, want %q", got, want)
	}
	if !state.HasSelection() {
		t.Fatal("the selection was not kept")
	}
}

func TestClickCollapsesTheSelection(t *testing.T) {
	src := "One two three.\n\nFour five six.\n"
	m := doc.New(src)
	var state State
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		ui.List(c, &state.List, m.Len(), func(r int) {
			block := m.RowBlock(r).Index
			el := ui.Row(c)
			el.HandleInput(pointerInput(&state, m, block))
			el.Children(func() { Render(c, m, &state, block) })
		}).Grow(1)
	}, 400, 300)

	x0, y0 := findText(t, tt, "One")
	x1, y1 := findText(t, tt, "Four")
	dragFromTo(tt, x0, y0, x1, y1)
	if !state.HasSelection() {
		t.Fatal("no selection after the drag")
	}
	x, y := findText(t, tt, "five")
	tt.ClickAt(x, y)
	if state.HasSelection() {
		t.Fatalf("selection %q survived a plain click", state.SelectionText(m))
	}
}

func TestDoubleClickSelectsAWord(t *testing.T) {
	src := "Alpha beta gamma.\n"
	m := doc.New(src)
	var state State
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		ui.List(c, &state.List, m.Len(), func(r int) {
			block := m.RowBlock(r).Index
			el := ui.Row(c)
			el.HandleInput(pointerInput(&state, m, block))
			el.Children(func() { Render(c, m, &state, block) })
		}).Grow(1)
	}, 400, 200)

	x, y := findText(t, tt, "beta")
	tt.Press(x, y)
	tt.Release(x, y)
	tt.Press(x, y)
	tt.Release(x, y)
	if got := state.SelectionText(m); got != "beta" {
		t.Fatalf("double click selected %q", got)
	}
}

func TestSelectionText(t *testing.T) {
	m := doc.New("Alpha beta.\n\nGamma delta.\n")
	blocks := m.Blocks()
	plain := m.Plain(blocks[0].Index)
	if plain != "Alpha beta." {
		t.Fatalf("plain: %q", plain)
	}
	s := selection.Selection{
		Anchor: selection.Point{Block: blocks[0].Index, Offset: 6},
		Caret:  selection.Point{Block: blocks[1].Index, Offset: 5},
	}
	var st State
	st.SetSelection(s.Anchor, s.Caret)
	if got := st.SelectionText(m); got != "beta.\nGamma" {
		t.Fatalf("SelectionText: %q", got)
	}
}

func TestMatchesDrawn(t *testing.T) {
	m := doc.New("needle in one line\n\nanother needle here\n")
	var state State
	blocks := m.Blocks()
	state.Matches = []Match{
		{Block: blocks[0].Index, From: 0, To: 6},
		{Block: blocks[1].Index, From: 8, To: 14},
	}
	state.Current = 1
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		ui.Column(c).Children(func() {
			for _, b := range blocks {
				Render(c, m, &state, b.Index)
			}
		})
	}, 400, 200)
	joined := strings.Join(tt.Texts(), " ")
	if !strings.Contains(joined, "needle") {
		t.Fatalf("texts %v: the text is gone", tt.Texts())
	}
}

func TestMatchHelpers(t *testing.T) {
	m := doc.New("one two one three\n")
	blocks := m.Blocks()
	got := FindAll(m, blocks[0].Index, "one")
	if len(got) != 2 || got[0].From != 0 || got[1].From != 8 {
		t.Fatalf("FindAll: %+v", got)
	}
}

// pointerInput wires the pointer to the state, as the app will.
func pointerInput(state *State, m *doc.Model, block int) func(ui.InputEvent) bool {
	return func(ev ui.InputEvent) bool {
		switch ev.Kind {
		case ui.InputPointerDown:
			if ev.Button == 0 {
				state.Press(m, block, ev)
				return true
			}
		case ui.InputPointerMove:
			if state.IsDragging() {
				state.Drag(m, block, ev)
				return true
			}
		case ui.InputPointerUp:
			if ev.Button == 0 && state.IsDragging() {
				state.Drag(m, block, ev)
				state.Dragging = false
				return true
			}
		}
		return false
	}
}
