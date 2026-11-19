package view

import (
	"image"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/doc"
)

// The inline emphasis style channel: the walk hands each run the style in
// force where it sits, and spans writes it onto the ui.Span. The channel
// used to stop at Strong's boundary (weight 700 never reached a leaf) and
// had no strikethrough case at all (the text vanished whole), so these
// tests are the sentinels for both.
//
// Each test asserts the span fields first, which catches the style being
// dropped on the way down, and the last test adds a pixel-ink comparison,
// which catches a style that arrives but never draws.

// inlineFrame renders src and returns the spans of the first block whose
// Plain text holds want, styled from the base the block's role would
// carry: the runs the walk styled, overlays included. The search walks
// the block tree depth first, since a table's cells and a list item's
// prose are their own blocks below the top level, and a leaf's spans
// carry the run split the emphasis nodes make.
func inlineFrame(t *testing.T, src, want string, base style) []ui.Span {
	t.Helper()
	m := doc.New(src)
	var found []ui.Span
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		st := &State{AnchorOf: anchorsOf(m)}
		var visit func(i int) bool
		visit = func(i int) bool {
			if strings.Contains(m.Plain(i), want) {
				found = st.spans(c, m, i, base)
				return true
			}
			hit := false
			m.Children(i)(func(j int) bool {
				if visit(j) {
					hit = true
					return false
				}
				return true
			})
			return hit
		}
		for _, blk := range m.Blocks() {
			if visit(blk.Index) {
				return
			}
		}
	}, 640, 400)
	tt.Frame()
	if found == nil {
		t.Fatalf("%q: no block draws %q", src, want)
	}
	return found
}

// exactSpan returns the span whose text is exactly want, one walk run, or
// nil: a run boundary proves the emphasis node ended where it should.
func exactSpan(spans []ui.Span, want string) *ui.Span {
	for k := range spans {
		if spans[k].Text == want {
			return &spans[k]
		}
	}
	return nil
}

// inkIn sums how far every pixel of the frame sits from the page's
// background: the same glyphs at weight 700 lay down more, and darker,
// ink than at 400. Alpha alone is no signal, an opaque page fills every
// pixel with it.
func inkIn(img *image.RGBA, bg ui.Color) int {
	n := 0
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := img.RGBAAt(x, y).RGBA()
			n += absDiff(uint8(r>>8), bg.R) + absDiff(uint8(g>>8), bg.G) + absDiff(uint8(bl>>8), bg.B)
		}
	}
	return n
}

func TestInlineBoldReachesTheSpans(t *testing.T) {
	got := inlineFrame(t, "say **bold** now\n", "bold", style{size: defaultBodySize, weight: 400})
	sp := exactSpan(got, "bold")
	if sp == nil {
		t.Fatalf("spans %v: no span whose text is exactly \"bold\"", got)
	}
	if sp.Weight != 700 {
		t.Fatalf("bold span weight %d, want 700: Strong's weight is not reaching the leaf", sp.Weight)
	}
	if plain := exactSpan(got, "say "); plain == nil || plain.Weight != 400 {
		t.Fatalf("spans %v: the run around the bold is not plain 400", got)
	}
}

func TestInlineItalicReachesTheSpans(t *testing.T) {
	got := inlineFrame(t, "say *italic* now\n", "italic", style{size: defaultBodySize, weight: 400})
	sp := exactSpan(got, "italic")
	if sp == nil {
		t.Fatalf("spans %v: no span whose text is exactly \"italic\"", got)
	}
	if !sp.Italic {
		t.Fatalf("italic span Italic=false: Emphasis is not reaching the leaf")
	}
	if plain := exactSpan(got, "say "); plain == nil || plain.Italic {
		t.Fatalf("spans %v: the run around the italic reads italic too", got)
	}
}

func TestStrikethroughKeepsItsTextAndStyle(t *testing.T) {
	// The page-level sentinel: the text used to vanish whole, not just
	// lose its line.
	m := doc.New("say ~~gone~~ now\n")
	st := &State{AnchorOf: anchorsOf(m)}
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		ScrollView(c, m, st)
	}, 640, 200)
	tt.Frame()
	if !tt.HasText("gone") {
		t.Fatalf("texts %v: the strikethrough text vanished from the page", tt.Texts())
	}
	got := inlineFrame(t, "say ~~gone~~ now\n", "gone", style{size: defaultBodySize, weight: 400})
	sp := exactSpan(got, "gone")
	if sp == nil {
		t.Fatalf("spans %v: no span whose text is exactly \"gone\"", got)
	}
	if !sp.Strikethrough {
		t.Fatalf("strikethrough span Strikethrough=false")
	}
}

func TestBoldItalicNestComposes(t *testing.T) {
	got := inlineFrame(t, "say **both *part* ends** now\n", "part", style{size: defaultBodySize, weight: 400})
	sp := exactSpan(got, "part")
	if sp == nil {
		t.Fatalf("spans %v: no span whose text is exactly \"part\"", got)
	}
	if sp.Weight != 700 || !sp.Italic {
		t.Fatalf("nested span weight %d italic %v, want 700 and true", sp.Weight, sp.Italic)
	}
	if outer := exactSpan(got, "both "); outer == nil || outer.Weight != 700 || outer.Italic {
		t.Fatalf("spans %v: the bold around the italic is not bold 700 upright", got)
	}
}

func TestStrikethroughWrapsBold(t *testing.T) {
	got := inlineFrame(t, "say ~~struck **deep** more~~ now\n", "deep", style{size: defaultBodySize, weight: 400})
	sp := exactSpan(got, "deep")
	if sp == nil {
		t.Fatalf("spans %v: no span whose text is exactly \"deep\"", got)
	}
	if sp.Weight != 700 || !sp.Strikethrough {
		t.Fatalf("nested span weight %d strike %v, want 700 and true", sp.Weight, sp.Strikethrough)
	}
	if outer := exactSpan(got, "struck "); outer == nil || !outer.Strikethrough {
		t.Fatalf("spans %v: the strikethrough around the bold lost its own line", got)
	}
}

func TestBoldInALinkKeepsItsWeight(t *testing.T) {
	got := inlineFrame(t, "see [**anchor** text](https://example.com/a) now\n", "anchor", style{size: defaultBodySize, weight: 400})
	sp := exactSpan(got, "anchor")
	if sp == nil {
		t.Fatalf("spans %v: no span whose text is exactly \"anchor\"", got)
	}
	if sp.Weight != 700 {
		t.Fatalf("link-bold span weight %d, want 700", sp.Weight)
	}
	if !sp.Underline {
		t.Fatalf("link-bold span Underline=false, want the link's underline")
	}
}

func TestBoldInATableCellIsStyled(t *testing.T) {
	got := inlineFrame(t, "| k | v |\n|---|---|\n| a | **cell bold** |\n", "cell bold", style{size: defaultBodySize, weight: 400})
	sp := exactSpan(got, "cell bold")
	if sp == nil {
		t.Fatalf("spans %v: no span whose text is exactly \"cell bold\"", got)
	}
	if sp.Weight != 700 {
		t.Fatalf("table cell bold span weight %d, want 700", sp.Weight)
	}
}

func TestBoldInAHeadingOverridesItsWeight(t *testing.T) {
	// An h2 is semibold 600; its inline bold must read 700 on top of it,
	// and the runs around it stay at the heading's own 600.
	got := inlineFrame(t, "## Head **bold** more\n", "bold", style{size: 22, weight: 600})
	sp := exactSpan(got, "bold")
	if sp == nil {
		t.Fatalf("spans %v: no span whose text is exactly \"bold\"", got)
	}
	if sp.Weight != 700 {
		t.Fatalf("heading bold span weight %d, want 700", sp.Weight)
	}
	if around := exactSpan(got, "Head "); around == nil || around.Weight != 600 {
		t.Fatalf("spans %v: the heading's own runs are not 600", got)
	}
}

func TestBoldInAListItemIsStyled(t *testing.T) {
	got := inlineFrame(t, "- item **bold** rest\n", "bold", style{size: defaultBodySize, weight: 400})
	sp := exactSpan(got, "bold")
	if sp == nil {
		t.Fatalf("spans %v: no span whose text is exactly \"bold\"", got)
	}
	if sp.Weight != 700 {
		t.Fatalf("list item bold span weight %d, want 700", sp.Weight)
	}
	if plain := exactSpan(got, "item "); plain == nil || plain.Weight != 400 {
		t.Fatalf("spans %v: the item's plain runs are not 400", got)
	}
}

func TestBoldDrawsHeavierInk(t *testing.T) {
	// The pixel check over the span fields: a style that arrives in the
	// spans but never draws leaves both pages at the same ink.
	page := func(t *testing.T, src string) *image.RGBA {
		m := doc.New(src)
		st := &State{AnchorOf: anchorsOf(m)}
		tt := ui.NewTester(func(c *ui.Context) {
			c.DisableAnimations()
			ScrollView(c, m, st)
		}, 400, 120)
		tt.Frame()
		return tt.Image()
	}
	bold := page(t, "tail **bold** tail\n")
	plain := page(t, "tail bold tail\n")
	bg := ui.LightTheme().Background
	bInk, pInk := inkIn(bold, bg), inkIn(plain, bg)
	if bInk <= pInk {
		t.Fatalf("bold ink %d, plain ink %d: weight 700 drew no extra ink", bInk, pInk)
	}
}
