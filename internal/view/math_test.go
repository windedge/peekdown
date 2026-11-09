package view

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown/parser"

	"github.com/windedge/peekdown/internal/doc"
)

// Math syntax stays literal text (ticket #17): no math extensions, so
// the math spellings survive as ordinary characters, typesetting later.
const mathSample = "# Math samples\n\n$$E=mc^2$$\n\nInline \\(a+b\\) here.\n\n\\[c\\]\n\nPrice $x+y$ stays.\n"

func renderExt(t *testing.T, src string, ext parser.Extensions) (*doc.Model, *ui.Tester, *State) {
	t.Helper()
	m := doc.NewWith(src, ext)
	state := &State{AnchorOf: anchorsOf(m)}
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		ui.List(c, &state.List, m.Len(), func(r int) {
			Render(c, m, state, m.RowBlock(r).Index)
		})
	}, 640, 400)
	tt.Frame()
	return m, tt, state
}

// TestMathStaysLiteral checks the four math spellings render with their
// characters visible and no math block in the document: content must not
// vanish into a blank row.
func TestMathStaysLiteral(t *testing.T) {
	m, tt, _ := renderExt(t, mathSample, parser.GFM|parser.ExtFootnotes)
	for _, b := range m.Blocks() {
		if b.Kind == parser.MathBlock {
			t.Fatalf("block %d is a math block; math must stay plain text", b.Index)
		}
	}
	for _, want := range []string{
		"Math samples",
		"$$E=mc^2$$",
		"(a+b)",
		"[c]",
		"Price $x+y$ stays.",
	} {
		if !has(tt, want) {
			t.Fatalf("texts %v: want %q", texts(tt), want)
		}
	}
}

// TestRenderUnknownBlockFallsBackToSource builds a math block directly
// (extensions on) to exercise Render's MathBlock branch: its TeX shows as
// source lines, no delimiters, mygo having stripped them into the block.
func TestRenderUnknownBlockFallsBackToSource(t *testing.T) {
	const src = "before\n\n$$\nE=mc^2\n$$\n\nafter\n"
	ext := parser.GFM | parser.ExtFootnotes | parser.ExtMath
	m := doc.NewWith(src, ext)
	math := -1
	for _, b := range m.Blocks() {
		if b.Kind == parser.MathBlock {
			math = b.Index
		}
	}
	if math < 0 {
		t.Fatalf("blocks %v: want a math block", m.Blocks())
	}
	if got := strings.TrimSpace(m.Lines(math)); got != "E=mc^2" {
		t.Fatalf("math block lines %q, want E=mc^2", got)
	}
	_, tt, _ := renderExt(t, src, ext)
	if !has(tt, "E=mc^2") {
		t.Fatalf("texts %v: math block content missing", texts(tt))
	}
	if !has(tt, "before") || !has(tt, "after") {
		t.Fatalf("texts %v: neighbours lost", texts(tt))
	}
}

// A footnote definition renders its note (muted, not a gray source box)
// and its run feeds the overlays: a search hit in the note text colors
// the spans the run table addresses.
func TestFootnoteDefinitionRendersNoteAndHighlights(t *testing.T) {
	src := "Alpha[^1] beta.\n\n[^1]: the note text\n"
	m, tt, _ := renderExt(t, src, parser.GFM|parser.ExtFootnotes)
	if !has(tt, "the note text") {
		t.Fatalf("texts %v: the note is not rendered", texts(tt))
	}
	var def int = -1
	for _, b := range m.Blocks() {
		if b.Kind == parser.FootnoteDefinition {
			def = b.Index
		}
	}
	if def < 0 {
		t.Fatalf("blocks %v: want a footnote definition", m.Blocks())
	}
	var st State
	leaf := -1
	for j := range m.Children(def) {
		if m.Block(j).Kind == parser.Paragraph {
			leaf = j
		}
	}
	if leaf < 0 {
		t.Fatalf("footnote definition %d has no paragraph", def)
	}
	joined := spanTextOf(m, &st, leaf)
	if plain := m.Plain(leaf); joined != plain {
		t.Fatalf("note spans %q != plain %q", joined, plain)
	}
	st.Matches = []Match{{Block: def, Start: 4, End: 13}}
	st.Current = 0
	var highlighted int
	_ = ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		spans := st.overlayRun(c, def, 0, []ui.Span{{Text: m.RunOf(def).Text}})
		for _, sp := range spans {
			if sp.Background != (ui.Color{}) {
				highlighted++
			}
		}
	}, 400, 300)
	if highlighted != 1 {
		t.Fatalf("%d highlighted spans over run %q, want 1", highlighted, m.RunOf(def).Text)
	}
}
