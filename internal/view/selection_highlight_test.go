package view

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/doc"
	"github.com/windedge/peekdown/internal/selection"
)

// selectAllState renders src with selection.All's selection and returns
// the spans of the block whose text holds want, with the theme's
// selection color, so a test can check the highlight the overlay paints
// per block. The lookup follows the view's two draw paths: a nested
// leaf's index hits RunAt (a quote's paragraph, a table cell) and its
// run holds the host block's text; a top-level paragraph's leaf is the
// block itself and RunAt misses, so the block's own text decides. Both
// go through st.spans, the real render path.
func selectAllState(t *testing.T, src, want string) ([]ui.Span, ui.Color) {
	t.Helper()
	m := doc.New(src)
	st := &State{AnchorOf: anchorsOf(m)}
	sel := selection.All(m)
	st.SetSelection(sel.Anchor, sel.Caret)
	if !st.HasSelection() {
		t.Fatalf("selection.All on %q left nothing selected", src)
	}
	var spans []ui.Span
	var theme ui.Color
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		theme = c.Theme().Selection
		// Nested leaves push the block indices past Len, so scan past it
		// and let misses fall through to the block's own text.
		for b := 0; b < m.Len()+64; b++ {
			if run, _ := m.RunAt(b); run != nil && strings.Contains(run.Text, want) {
				spans = st.spans(c, m, b, style{})
				return
			}
			if strings.Contains(blockText(m, b), want) {
				spans = st.spans(c, m, b, style{})
				return
			}
		}
	}, 640, 400)
	tt.Frame()
	if len(spans) == 0 {
		t.Fatalf("%q: no block holds %q", src, want)
	}
	n := 0
	for _, sp := range spans {
		if sp.Background == theme {
			n++
		}
	}
	if n == 0 {
		t.Fatalf("Select All over %q: no span of %q carries the selection's background %v (spans %v)", src, want, theme, spans)
	}
	return spans, theme
}

// blockText returns the text the view typesets for block b: a
// paragraph's or heading's plain text, or a source-like block's lines.
func blockText(m *doc.Model, b int) string {
	if run := m.Plain(b); strings.TrimSpace(run) != "" {
		return run
	}
	return strings.Trim(m.Lines(b), "\n")
}

// Select All must light every run between the endpoints, not only the one
// block that holds both: the highlight is per block's run, and a doc-wide
// selection spans many.
func TestSelectAllHighlightsEveryBlock(t *testing.T) {
	spans, _ := selectAllState(t, "# Head\n\nFirst para alpha\n\nSecond para beta\n\n```\ncode line\n```\n", "Second para beta")
	if got := spansText(spans); got != "Second para beta" {
		t.Fatalf("second paragraph split %q, want whole", got)
	}
}

// An endpoint inside a nested block (a quote's paragraph) still colors
// the middle blocks' runs whole.
func TestSelectAllHighlightsQuoteRuns(t *testing.T) {
	selectAllState(t, "# Head\n\n> quoted alpha\n\nSecond para beta\n", "quoted alpha")
}

// A middle block sits wholly in the selection: every rune of its text
// carries the selection's background, not only the first stretch the
// endpoints' offsets once covered.
func TestSelectAllCoversMiddleBlocksWholly(t *testing.T) {
	src := "# Head\n\nFirst para alpha\n\nSecond para beta\n\n```\ncode line\n```\n"
	spans, theme := selectAllState(t, src, "Second para beta")
	covered := 0
	total := 0
	for _, sp := range spans {
		n := runeLen(sp.Text)
		total += n
		if sp.Background == theme {
			covered += n
		}
	}
	if want := runeLen("Second para beta"); total != want {
		t.Fatalf("middle block's spans hold %d runes, want %d", total, want)
	}
	if covered != total {
		t.Fatalf("only %d of %d runes of the middle block highlighted, want all", covered, total)
	}
}

func spansText(spans []ui.Span) string {
	var b strings.Builder
	for _, sp := range spans {
		b.WriteString(sp.Text)
	}
	return b.String()
}
