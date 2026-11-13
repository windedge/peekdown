package view

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

// Ticket #23 F: a code block carries a copy button in its top-right
// corner, its long lines scroll sideways instead of wrapping, and the
// table columns honor the delimiter row's alignment.

// TestCodeBlockCopyButton clicks the copy button of a code block and
// expects the block's source, newlines and all, on the CopyText hook, the
// button to turn into "Copied" and to turn back after two seconds.
func TestCodeBlockCopyButton(t *testing.T) {
	m, tt, st := frame(t, "```\nx := 1\ny := 22\n```\n", 640, 300)
	var got string
	st.CopyText = func(s string) { got = s }
	tt.Frame()
	if !tt.HasText("Copy") {
		t.Fatalf("texts %v: no copy button", tt.Texts())
	}
	if err := tt.Click("Copy"); err != nil {
		t.Fatal(err)
	}
	if want := "x := 1\ny := 22"; got != want {
		t.Fatalf("CopyText got %q, want the block's source %q", got, want)
	}
	if want := m.Blocks()[0].Index; st.copiedBlock != want {
		t.Fatalf("copiedBlock = %d, want %d", st.copiedBlock, want)
	}
	if !tt.HasText("Copied") {
		t.Fatalf("texts %v: no copied feedback", tt.Texts())
	}
	tt.Advance(2 * time.Second)
	if tt.HasText("Copied") {
		t.Fatalf("texts %v: the copied feedback outlived its two seconds", tt.Texts())
	}
	if !tt.HasText("Copy") {
		t.Fatalf("texts %v: the copy button is gone", tt.Texts())
	}
}

// TestCopyButtonIsIconOnly checks the copy button draws as a square icon,
// no text: the user read the "Copy" word as noise next to the tooltip's
// "Copy code". The label keeps naming it for clicks and the screen
// reader, so the size is the only guard against the word growing back:
// a 12 icon, 4 padding and a 1 border put it at 22x22, and a text label
// would blow the width past 28.
func TestCopyButtonIsIconOnly(t *testing.T) {
	_, tt, st := frame(t, "```\nx := 1\n```\n", 640, 300)
	st.CopyText = func(string) {}
	tt.Frame()
	r, ok := tt.Find("Copy")
	if !ok {
		t.Fatalf("texts %v: no copy button", tt.Texts())
	}
	if d := r.W - r.H; d < -8 || d > 8 {
		t.Fatalf("copy button %v: not square within 8 DIPs", r)
	}
	if r.W > 28 {
		t.Fatalf("copy button %v wide, want the icon-only 28 cap", r)
	}
}

// TestCodeBlockCopyButtonHiddenWithoutHook checks the button only shows
// when the host wired the clipboard: an unwired view stays as it was.
func TestCodeBlockCopyButtonHiddenWithoutHook(t *testing.T) {
	_, tt, _ := frame(t, "```\nx := 1\n```\n", 640, 300)
	if tt.HasText("Copy") {
		t.Fatalf("texts %v: a copy button without a CopyText hook", tt.Texts())
	}
}

// TestCodeBlockLongLineScrolls checks a long line stays one line in the
// code panel: the block scrolls it sideways (mygo's ScrollHorizontal)
// instead of wrapping it, so the code keeps its shape.
func TestCodeBlockLongLineScrolls(t *testing.T) {
	line := "var s = \"" + strings.Repeat("x", 200) + "\" // tail"
	_, tt, _ := frame(t, "```go\n"+line+"\n```\n", 400, 300)
	if !tt.HasText(line) {
		t.Fatalf("texts %v: the long line is missing", tt.Texts())
	}
	// The label's box is the scroller's viewport: a wrapped line would be
	// several lines tall inside it, a whole one a single line it clips.
	if r := rectOf(t, tt, line); r.H > 30 {
		t.Fatalf("the long line draws %v tall: it wrapped onto several lines", r.H)
	}
	comment := editor.DefaultTheme(ui.LightTheme()).TokenColor(editor.TokenComment)
	if n := countColor(tt.Image(), comment); n != 0 {
		t.Fatalf("%d comment pixels with the block unscrolled: the tail already shows", n)
	}
}

// TestShortCodeLineStaysPut checks a short line needs no scrolling: the
// change must not indent or offset the code that fits.
func TestShortCodeLineStaysPut(t *testing.T) {
	_, tt, _ := frame(t, "Intro paragraph.\n\n```\nx := 1\n```\n", 640, 400)
	intro := rectOf(t, tt, "Intro paragraph")
	code := rectOf(t, tt, "x := 1")
	if got := code.X - intro.X; !near(got, codeBlockPadding) {
		t.Fatalf("code text at %v, %v from the block's left edge, want %v of padding", code.X, got, codeBlockPadding)
	}
}
