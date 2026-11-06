package view

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/doc"
	"github.com/windedge/peekdown/internal/selection"
)

// dragFromTo presses at (x0, y0), moves and releases at (x1, y1), the way
// a user drags a selection.
func dragFromTo(tt *ui.Tester, x0, y0, x1, y1 float32) {
	tt.Press(x0, y0)
	steps := 20
	for i := 1; i <= steps; i++ {
		tt.Move(x0+(x1-x0)*float32(i)/float32(steps), y0+(y1-y0)*float32(i)/float32(steps))
	}
	tt.Release(x1, y1)
}

// findText returns the window point of the text drawn with want: over the
// line's last glyphs at its right edge when over is set, else past its
// first glyph's left half.
func findText(t *testing.T, tt *ui.Tester, want string, over bool) (float32, float32) {
	t.Helper()
	for _, label := range tt.Texts() {
		if strings.Contains(label, want) {
			r, ok := tt.Find(label)
			if !ok {
				continue
			}
			x := r.X + 2
			if over {
				x = r.X + r.W - 1
			}
			return x, r.Y + r.H/2
		}
	}
	t.Fatalf("texts %v: %q not found", tt.Texts(), want)
	return 0, 0
}

// wordMid returns the window x over the middle of the first occurrence of
// want in the line's drawn text, from the shaped text's caret boxes: a
// word double-clicked has to be pressed between its glyphs.
func wordMid(t *testing.T, tt *ui.Tester, want string) float32 {
	t.Helper()
	for _, label := range tt.Texts() {
		if !strings.Contains(label, want) {
			continue
		}
		r, ok := tt.Find(label)
		if !ok {
			continue
		}
		text := strings.SplitN(label, "\n", 2)[0]
		layout := ui.ShapeText(text, ui.Font{Size: 15}, 10000)
		a, b := strings.Index(text, want), strings.Index(text, want)+len(want)
		mid := (layout.Caret(a).X + layout.Caret(b).X) / 2
		if mid <= 0 {
			t.Fatalf("no caret boxes for %q in %q", want, text)
		}
		return r.X + mid
	}
	t.Fatalf("texts %v: %q not found", tt.Texts(), want)
	return 0
}

func TestPointerDragSelectsAcrossBlocks(t *testing.T) {
	src := "First paragraph.\n\nSecond paragraph.\n\nThird paragraph.\n"
	m := doc.New(src)
	var state State
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		list := ui.List(c, &state.List, m.Len(), func(r int) {
			state.Row(c, m, r)
		}).Grow(1)
		list.HandleInput(state.Handler(m))
	}, 400, 300)

	x0, y0 := findText(t, tt, "First", false)
	x1, y1 := findText(t, tt, "Third", true)
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
		list := ui.List(c, &state.List, m.Len(), func(r int) {
			state.Row(c, m, r)
		}).Grow(1)
		list.HandleInput(state.Handler(m))
	}, 400, 300)

	x0, y0 := findText(t, tt, "One", false)
	x1, y1 := findText(t, tt, "Four", true)
	dragFromTo(tt, x0, y0, x1, y1)
	if !state.HasSelection() {
		t.Fatal("no selection after the drag")
	}
	x, y := findText(t, tt, "five", false)
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
		list := ui.List(c, &state.List, m.Len(), func(r int) {
			state.Row(c, m, r)
		}).Grow(1)
		list.HandleInput(state.Handler(m))
	}, 400, 200)

	// Over the glyphs of the second word: the box the text is drawn in
	// starts at the row's edge, before "Alpha"'s first glyph, and a
	// point on a glyph's left half reads the boundary before it.
	_, y := findText(t, tt, "gamma", false)
	x := wordMid(t, tt, "beta")
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
		{Block: blocks[0].Index, Start: 0, End: 6},
		{Block: blocks[1].Index, Start: 8, End: 14},
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
	got := doc.FindAll(m, "one", doc.SearchOptions{})
	if len(got) != 2 || got[0].Start != 0 || got[1].Start != 8 || got[0].Block != blocks[0].Index {
		t.Fatalf("FindAll: %+v", got)
	}
}

// dragWire is the test's wiring of the view: the single Handler on the
// list that wraps the rows, the way the app wires it.
func dragWire(c *ui.Context, m *doc.Model, state *State) {
	list := ui.List(c, &state.List, m.Len(), func(r int) {
		state.Row(c, m, r)
	}).Grow(1)
	list.HandleInput(state.Handler(m))
}

func TestWheelScrollStillWorks(t *testing.T) {
	long := strings.Repeat("Paragraph of filler text to scroll past.\n\n", 60)
	m := doc.New(long)
	// Without the handler, the wheel scrolls the list.
	var state State
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		ui.List(c, &state.List, m.Len(), func(r int) {
			state.Row(c, m, r)
		}).Grow(1)
	}, 400, 300)
	first := tt.Texts()
	tt.Wheel(200, 150, 0, 120)
	tt.Advance(200 * time.Millisecond)
	if strings.Join(tt.Texts(), " ") == strings.Join(first, " ") {
		t.Fatal("the wheel scrolled nothing without the handler")
	}
	// With the handler, it still does: the handler leaves the wheel be.
	var state2 State
	tt2 := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		dragWire(c, m, &state2)
	}, 400, 300)
	first2 := tt2.Texts()
	tt2.Wheel(200, 150, 0, 120)
	tt2.Advance(200 * time.Millisecond)
	if strings.Join(tt2.Texts(), " ") == strings.Join(first2, " ") {
		t.Fatal("the handler swallowed the wheel")
	}
}

func TestDragSelectAfterScroll(t *testing.T) {
	var b strings.Builder
	for i := range 40 {
		b.WriteString("Paragraph number ")
		b.WriteString(itoa(i))
		b.WriteString(" of the long document.\n\n")
	}
	m := doc.New(b.String())
	var state State
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		dragWire(c, m, &state)
	}, 400, 300)
	state.List.ScrollTo(30, ui.Start)
	tt.Frame()
	x0, y0 := findText(t, tt, "Paragraph number 30", false)
	x1, y1 := findText(t, tt, "Paragraph number 32", true)
	dragFromTo(tt, x0, y0, x1, y1)
	got := state.SelectionText(m)
	want := "Paragraph number 30 of the long document.\nParagraph number 31 of the long document.\nParagraph number 32 of the long document."
	if got != want {
		t.Fatalf("selected %q, want %q", got, want)
	}
}

func TestSingleHandlerCoordinates(t *testing.T) {
	m := doc.New("Alpha beta.\n\nGamma delta.\n")
	var state State
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		dragWire(c, m, &state)
	}, 400, 300)
	// A press above the rows but inside the window, a move below the
	// rows: the press clamps to the first row's start, the move to the
	// last row's end, still in document. (A press above the window's
	// top comes as no press at all: the engine lets it go where the
	// window does not take it.)
	x, rowY := findText(t, tt, "Alpha", false)
	tt.Press(x, 5)
	tt.Move(x, rowY+50)
	tt.Release(x, rowY+50)
	got := state.SelectionText(m)
	want := "Alpha beta.\nGamma delta."
	if got != want {
		t.Fatalf("selected %q, want %q", got, want)
	}
	if !state.HasSelection() || !state.HasCaret() {
		t.Fatalf("HasSelection %v, HasCaret %v", state.HasSelection(), state.HasCaret())
	}
	// A collapsed selection is no selection, but still a caret.
	state.SetSelection(selection.Point{Block: 0}, selection.Point{Block: 0})
	if state.HasSelection() || !state.HasCaret() {
		t.Fatalf("collapsed: HasSelection %v, HasCaret %v", state.HasSelection(), state.HasCaret())
	}
}
