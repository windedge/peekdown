package doc

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui/markdown/parser"
)

// Math stays plain text (ticket #17): parsed without the math extensions,
// the run table holds the literal characters, so search finds them.
func TestMathLiteralStaysInRunsAndSearch(t *testing.T) {
	m := NewWith("before\n\n$$E=mc^2$$\n\nPrice $x+y$ today.\n", parser.GFM|parser.ExtFootnotes)
	found := false
	for _, r := range m.Runs() {
		if strings.Contains(r.Text, "$$E=mc^2$$") {
			found = true
		}
	}
	if !found {
		t.Fatalf("runs %v: want $$E=mc^2$$ in a run", m.Runs())
	}
	m.SetSearch("E=mc^2", SearchOptions{})
	if len(m.Matches) == 0 {
		t.Fatalf("no matches for E=mc^2 in runs %v", m.Runs())
	}
	m.SetSearch("$x+y$", SearchOptions{})
	if len(m.Matches) == 0 {
		t.Fatalf("no matches for $x+y$ in runs %v", m.Runs())
	}
}

// A footnote definition contributes its note text: the block's own Lines
// are empty, the note living in the blocks nested in it, so the run is
// the leaves joined, and the search finds the note where it renders.
func TestRunsOfFootnoteDefinition(t *testing.T) {
	m := NewWith("Alpha[^1] beta.\n\n[^1]: the note text\n", parser.GFM|parser.ExtFootnotes)
	var def *Block
	for k := range m.Blocks() {
		if m.Blocks()[k].Kind == parser.FootnoteDefinition {
			def = &m.Blocks()[k]
		}
	}
	if def == nil {
		t.Fatalf("blocks %v: want a footnote definition", m.Blocks())
	}
	run := m.RunOf(def.Index)
	if run == nil {
		t.Fatalf("footnote definition %d has no run; runs %v", def.Index, m.Runs())
	}
	if run.Text != "the note text" {
		t.Fatalf("run %q, want the note text", run.Text)
	}
	m.SetSearch("note text", SearchOptions{})
	if len(m.Matches) == 0 {
		t.Fatalf("search misses the note in runs %v", m.Runs())
	}
}
