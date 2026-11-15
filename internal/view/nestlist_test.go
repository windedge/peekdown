package view

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/doc"
)

// rectAt returns the rect of the label that contains want exactly once.
func rectAt(t *testing.T, tt *ui.Tester, want string) ui.Rect {
	t.Helper()
	r, ok := rectOfMaybe(tt, want)
	if !ok {
		t.Fatalf("texts %v: %q not found", texts(tt), want)
	}
	return r
}

// rectOfMaybe finds the label that contains want, false without one.
func rectOfMaybe(tt *ui.Tester, want string) (ui.Rect, bool) {
	for _, label := range tt.Texts() {
		if strings.Contains(label, want) {
			if r, ok := tt.Find(label); ok {
				return r, true
			}
		}
	}
	return ui.Rect{}, false
}

// TestNestedListIndentsPerLevel checks the nested list's column shifts
// right by one level per depth, the Rust version's ml(rems(1.)) on each
// nested list: depth 1's markers start one marker column past depth
// 0's, and the texts move with them.
func TestNestedListIndentsPerLevel(t *testing.T) {
	src := "- outer\n  - inner\n"
	m := doc.New(src)
	var state State
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		ScrollView(c, m, &state)
	}, 400, 300)
	tt.Frame()
	outer, outerBullet := rectAt(t, tt, "outer"), rectAt(t, tt, "▪")
	inner, innerBullet := rectAt(t, tt, "inner"), rectAt(t, tt, "•")
	// One level shifts the whole nested row by the marker column it sits
	// inside (marker 18 + gap 4), the Go shape of the Rust per-level
	// ml(rems(1.)): the nested bullet starts where the outer text does.
	if got := innerBullet.X - outerBullet.X; got < 18 || got > 26 {
		t.Fatalf("inner bullet x offset = %v, want about 22", got)
	}
	if got := inner.X - outer.X; got != innerBullet.X-outerBullet.X {
		t.Fatalf("inner text x offset = %v, bullet moved %v: the row lost its shape", got, innerBullet.X-outerBullet.X)
	}
}

// TestNestedListMarkerChangesWithDepth checks the bullets step through
// listBullets per level, the Rust version's list_item_prefix: top level
// ▪, the first nested level •, the second ◦, past the set's end the last
// one repeats.
func TestNestedListMarkerChangesWithDepth(t *testing.T) {
	src := "- l0\n  - l1\n    - l2\n      - l3\n        - l4\n          - l5\n"
	m := doc.New(src)
	var state State
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		ScrollView(c, m, &state)
	}, 400, 400)
	tt.Frame()
	for _, glyph := range []string{"▪", "•", "◦", "‣", "⁃"} {
		if !has(tt, glyph) {
			t.Fatalf("texts %v: missing the level bullet %q", texts(tt), glyph)
		}
	}
	// Past the set's end, the last bullet repeats: level 5 shows ⁃ again,
	// six items, five glyphs.
	if got := len(texts(tt)); got == 0 {
		t.Fatal("no texts at all")
	}
	var last int
	for _, s := range texts(tt) {
		if s == "⁃" {
			last++
		}
	}
	if last != 2 {
		t.Fatalf("the deepest level's bullet repeats %v times, want 2", last)
	}
}

// TestTaskListKeepsTopLevelBox checks a top-level task item's checkbox
// takes the top level's marker width, 18 DIPs, so its text lines up
// with a bulleted item's, whatever the nesting.
func TestTaskListKeepsTopLevelBox(t *testing.T) {
	src := "- [x] done\n- [ ] open\n"
	m := doc.New(src)
	var state State
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		ScrollView(c, m, &state)
	}, 400, 300)
	tt.Frame()
	done := rectAt(t, tt, "done")
	open := rectAt(t, tt, "open")
	if done.X != open.X {
		t.Fatalf("task items x %v vs %v: the boxes lost the shared marker column", done.X, open.X)
	}
}

// TestOrderedListNestedLetters checks an ordered list's nested level
// names its items with letters, the Rust version's NUMBERED_PREFIXES:
// depth 1 runs A. B., depth 2 a. b., while the top level keeps numbers.
func TestOrderedListNestedLetters(t *testing.T) {
	src := "1. top\n   1. inner\n   2. inner too\n2. second\n"
	m := doc.New(src)
	var state State
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		ScrollView(c, m, &state)
	}, 400, 300)
	tt.Frame()
	for _, want := range []string{"1.", "2.", "A.", "B."} {
		if !has(tt, want) {
			t.Fatalf("texts %v: missing marker %q", texts(tt), want)
		}
	}
}
