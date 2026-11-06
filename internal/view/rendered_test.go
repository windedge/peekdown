package view

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown/parser"

	"github.com/windedge/peekdown/internal/doc"
)

// The renderer's spans must agree with doc's run table rune for rune: the
// selection, the cuts and the search all address the offsets of that
// joined text.

// alignSample carries one of each: a paragraph with a link, code, emphasis
// and a hard break; a heading; a code block; a table; a quote; a task
// list.
const alignSample = "Lead paragraph with a [link](https://example.com), `code`, *emphasis* and a hard break.\\\nstill one block.\n\n" +
	"## Heading text\n\n" +
	"```go\nx := 1\ny := 22\n```\n\n" +
	"| Left | Right |\n|---|---|\n| 1 | 2 |\n\n" +
	"> quoted wisdom\n\n" +
	"- [ ] task one\n- [x] task two\n"

func spanText(spans []ui.Span) string {
	var b strings.Builder
	for _, sp := range spans {
		b.WriteString(sp.Text)
	}
	return b.String()
}

// spanTextOf joins the spans the renderer builds for block i. The theme
// lives in the frame, so the call runs inside the tester's frame.
func spanTextOf(m *doc.Model, st *State, i int) string {
	var out string
	_ = ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		out = spanText(st.spans(c, m, i, style{}))
	}, 400, 300)
	return out
}

// firstRun returns block i's run text, or its Plain when the block has no
// run of its own.
func firstRun(m *doc.Model, i int) string {
	if r := m.RunOf(i); r != nil {
		return r.Text
	}
	return m.Plain(i)
}

func TestSpansMatchRunsOnTopLevelParagraphs(t *testing.T) {
	m := doc.New(alignSample)
	var st State
	checked := 0
	for _, b := range m.Blocks() {
		if b.Kind != parser.Paragraph && b.Kind != parser.Heading {
			continue
		}
		if m.RunOf(b.Index) == nil {
			t.Fatalf("block %d (%v) has no run", b.Index, b.Kind)
		}
		joined, run := spanTextOf(m, &st, b.Index), firstRun(m, b.Index)
		if joined != run {
			t.Errorf("block %d (%v): spans %q != run %q", b.Index, b.Kind, joined, run)
		}
		checked++
	}
	if checked < 2 {
		t.Fatalf("%d top-level paragraph or heading blocks, want at least 2", checked)
	}
}

// A quote's or a list's paragraph shares the whole block's one run with its
// siblings, so the view checks its spans against the leaf's own Plain.
func TestSpansMatchPlainInsideQuoteAndList(t *testing.T) {
	m := doc.New(alignSample)
	var st State

	quote := m.Blocks()[4]
	if quote.Kind != parser.Quote {
		t.Fatalf("block 4 is %v, want a quote", quote.Kind)
	}
	leaves := m.QuoteBlocks(quote.Index)
	if len(leaves) == 0 {
		t.Fatalf("quote %d has no blocks", quote.Index)
	}
	if m.Block(leaves[0]).Kind != parser.Paragraph {
		t.Fatalf("quote's first block is %v, want a paragraph", m.Block(leaves[0]).Kind)
	}
	if joined, plain := spanTextOf(m, &st, leaves[0]), m.Plain(leaves[0]); joined != plain {
		t.Errorf("quote paragraph: spans %q != plain %q", joined, plain)
	}

	list := m.Blocks()[5]
	if list.Kind != parser.List {
		t.Fatalf("block 5 is %v, want a list", list.Kind)
	}
	items := m.ListItems(list.Index)
	if len(items) < 2 {
		t.Fatalf("%d items, want 2", len(items))
	}
	item := items[0]
	leaf := -1
	for j := range m.Children(item.Index) {
		if m.Block(j).Kind == parser.Paragraph {
			leaf = j
			break
		}
	}
	if leaf < 0 {
		t.Fatalf("item %d has no paragraph", item.Index)
	}
	joined := spanTextOf(m, &st, leaf)
	if plain := m.Plain(leaf); joined != plain {
		t.Errorf("list paragraph: spans %q != plain %q", joined, plain)
	}
	// The task box is a prefix span outside the run text.
	if !strings.HasPrefix(joined, "task one") {
		t.Errorf("list paragraph %q does not hold the item's text", joined)
	}
}

// Semantic corners where walk used to drift from parser.PlainText: hard
// breaks are spaces and a footnote reference shows as [^label].
func TestWalkFollowsPlainTextSemantics(t *testing.T) {
	m := doc.NewWith("Alpha.  \nBeta[^1] gamma.\n\n[^1]: the note\n", parser.ExtFootnotes)
	blocks := m.Blocks()
	if len(blocks) == 0 || blocks[0].Kind != parser.Paragraph {
		t.Fatalf("blocks %+v, want a paragraph first", blocks)
	}

	joined := spanTextOf(m, &State{}, blocks[0].Index)
	plain := m.Plain(blocks[0].Index)
	if joined != plain {
		t.Fatalf("walk %q != PlainText %q", joined, plain)
	}
	for _, bad := range []string{"\n", "Beta1 "} {
		if strings.Contains(joined, bad) {
			t.Fatalf("walk %q holds %q", joined, bad)
		}
	}
	if !strings.Contains(joined, "Alpha. Beta[^1] gamma.") {
		t.Fatalf("walk %q loses the break space or the footnote mark", joined)
	}
}

// An image's description stays in the text: PlainText's flat walk over the
// inline array emits the alt text too, though it drops link and image
// destinations.
func TestWalkDropsLinkDestinationButKeepsText(t *testing.T) {
	m := doc.New("See [the guide](https://example.com/guide) now.\n")
	b := m.Blocks()[0]
	joined := spanTextOf(m, &State{}, b.Index)
	if joined != m.Plain(b.Index) {
		t.Fatalf("walk %q != PlainText %q", joined, m.Plain(b.Index))
	}
	if !strings.Contains(joined, "the guide") {
		t.Fatalf("walk %q loses the link's text", joined)
	}
	if strings.Contains(joined, "https://example.com/guide") {
		t.Fatalf("walk %q holds the destination", joined)
	}
}
