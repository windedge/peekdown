package view

import (
	"fmt"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown/parser"

	"github.com/windedge/peekdown/internal/doc"
	"github.com/windedge/peekdown/internal/htmlx"
)

// An HTML block must be blank-line separated in markdown, as CommonMark
// reads the block until the blank line.
const htmlSample = `before

<div>
<h2>Title</h2>
<p>Hello <b>bold</b> <a href="https://e.x">link</a></p>
<table><tr><th>a</th><th>b</th></tr><tr><td>1</td><td>2</td></tr></table>
<pre>x := 1</pre>
<hr>
<img src="https://e.x/p.png" alt="pic">
<foobar>custom</foobar>
</div>

after
`

// TestHTMLBlockRendersSemantics checks the block draws its content the
// way the markdown blocks around it do: heading, paragraph with styled
// inlines, table, pre, rule and a pruned tag's text, while the tags
// themselves stay out of the reader's texts. The run table holds the
// block's visible text, so the search addresses what is drawn.
func TestHTMLBlockRendersSemantics(t *testing.T) {
	var loaded []string
	m := doc.New(htmlSample)
	var state State
	state.LoadImage = func(url string) (ui.ImageSource, error) {
		loaded = append(loaded, url)
		return nil, fmt.Errorf("no loader in the test")
	}
	state.AnchorOf = anchorsOf(m)
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		ui.List(c, &state.List, m.Len(), func(r int) {
			Render(c, m, &state, m.RowBlock(r).Index)
		})
	}, 640, 400)
	tt.Frame()
	for _, want := range []string{
		"Title", "Hello", "bold", "link",
		"a", "b", "1", "2",
		"x := 1", "custom", "pic",
	} {
		if !has(tt, want) {
			t.Fatalf("texts %v: want %q", texts(tt), want)
		}
	}
	for _, gone := range []string{"<div>", "<p>", "<b>", "href", "<table>"} {
		if has(tt, gone) {
			t.Fatalf("texts %v: the tag %q leaks", texts(tt), gone)
		}
	}
	// The visible text agrees with the run table the search addresses.
	var run *doc.Run
	for _, r := range m.Runs() {
		if strings.Contains(r.Text, "Title") {
			run = &r
		}
	}
	if run == nil {
		t.Fatalf("no run of the HTML block in %v", m.Runs())
	}
	if want := "Title\nHello bold link\na b 1 2\nx := 1\npic\ncustom"; run.Text != want {
		t.Fatalf("run %q, want %q", run.Text, want)
	}
	// The image asked its loader for the src, or its label showed trying.
	if len(loaded) != 1 || loaded[0] != "https://e.x/p.png" {
		t.Fatalf("loaded %v, want the img's src", loaded)
	}
	// The markdown around the block stays put.
	if texts(tt)[0] != "before" || !has(tt, "after") {
		t.Fatalf("the markdown around the block moved: texts %v", texts(tt))
	}
	// Scroll to the end so the rows below the window build too.
	state.List.ScrollToEnd()
	tt.Frame()
	if !has(tt, "custom") {
		t.Fatalf("texts %v after the end: the block's tail is missing", texts(tt))
	}
}

// TestHTMLBlockSearchHighlight checks a hit inside the block lights up:
// the run table holds the visible text, so the model's search finds it
// there, and overlayRun cuts the paragraph's spans at the match with the
// match's background over the covered runes.
func TestHTMLBlockSearchHighlight(t *testing.T) {
	m := doc.New("<div>\n<p>Alpha searchable</p>\n</div>")
	m.SetSearch("search", doc.SearchOptions{})
	if len(m.Matches) == 0 {
		t.Fatalf("no matches in the HTML block's visible text %q", m.Runs()[0].Text)
	}
	blocks := m.Blocks()
	var state State
	state.Matches = m.Matches
	state.Current = -1 // a plain match, not the current one
	var out []ui.Span
	var focus ui.Color
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		focus = c.Theme().Focus
		// The paragraph's spans as htmlNode builds them, before the
		// overlay, then cut at the match.
		nodes := htmlx.Parse(m.Lines(blocks[0].Index))
		var spans []ui.Span
		for _, seg := range nodes[0].Segs {
			spans = append(spans, state.htmlSeg(c, seg, nodes[0].Level))
		}
		out = state.overlayRun(c, blocks[0].Index, 0, spans)
	}, 400, 300)
	tt.Frame()
	if len(out) <= 1 {
		t.Fatalf("%d spans after the cut, want the match's piece split out", len(out))
	}
	var covered bool
	for _, sp := range out {
		if sp.Background != (ui.Color{}) && sp.Text == "search" {
			covered = true
			if sp.Background != focus {
				t.Fatalf("match's background %v, want the theme's focus %v", sp.Background, focus)
			}
		}
	}
	if !covered {
		t.Fatalf("spans %v: no piece carries the match's background", out)
	}
}

// TestHTMLBlockRuleAndBareImageRender checks a zero-text node no longer
// skips its draw: the hr's line and an image without alt render among
// their siblings, the text around them stays put, and the loader sees
// the bare img's src.
func TestHTMLBlockRuleAndBareImageRender(t *testing.T) {
	var loaded []string
	m := doc.New("top\n\n<div>\n<hr>\n<p>after the rule</p>\n<img src=\"https://e.x/p.png\">\n</div>\n\nbottom\n")
	var state State
	state.LoadImage = func(url string) (ui.ImageSource, error) {
		loaded = append(loaded, url)
		return nil, fmt.Errorf("no loader in the test")
	}
	state.AnchorOf = anchorsOf(m)
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		ui.List(c, &state.List, m.Len(), func(r int) {
			Render(c, m, &state, m.RowBlock(r).Index)
		})
	}, 640, 400)
	tt.Frame()
	for _, want := range []string{"top", "after the rule", "bottom"} {
		if !has(tt, want) {
			t.Fatalf("texts %v: want %q", texts(tt), want)
		}
	}
	if len(loaded) != 1 || loaded[0] != "https://e.x/p.png" {
		t.Fatalf("loaded %v, want the bare img's src", loaded)
	}
	// The rule takes no slot in the run: it stays the paragraph's text
	// alone, and the blocks around the HTML block keep their places.
	var run *doc.Run
	for _, r := range m.Runs() {
		if strings.Contains(r.Text, "after the rule") {
			run = &r
		}
	}
	if run == nil || run.Text != "after the rule" {
		t.Fatalf("runs %v, want \"after the rule\" alone", m.Runs())
	}
	if texts(tt)[0] != "top" || !has(tt, "bottom") {
		t.Fatalf("the markdown around the block moved: texts %v", texts(tt))
	}
}

// TestHTMLBlockSearchHighlightInQuote checks a hit inside an HTML block
// nested in a quote lights up at the right runes: the block's piece of
// the quote's one run starts after the quote's earlier leaves, so the
// overlays must address it at the leaf's base offset, not the run's.
func TestHTMLBlockSearchHighlightInQuote(t *testing.T) {
	m := doc.New("> before\n>\n> <div>\n> <p>Quoted searchable</p>\n> </div>\n")
	m.SetSearch("search", doc.SearchOptions{})
	if len(m.Matches) == 0 {
		t.Fatalf("no matches in %v", m.Runs())
	}
	var quote, html int
	for _, b := range m.Blocks() {
		if b.Kind == parser.Quote {
			quote = b.Index
			leaves := m.QuoteBlocks(quote)
			html = leaves[len(leaves)-1]
		}
	}
	if m.Block(html).Kind != parser.HTMLBlock {
		t.Fatalf("quote %d's last leaf %d is no HTML block", quote, html)
	}
	run, base := m.RunAt(html)
	if run == nil || base <= 0 {
		t.Fatalf("RunAt(%d) = %v, %d, want a run and a past-the-quote base", html, run, base)
	}
	match := m.Matches[0]
	if match.Start <= base {
		t.Fatalf("match at %d..%d, want past the base %d", match.Start, match.End, base)
	}
	var state State
	state.Matches = m.Matches
	state.Current = -1
	// htmlBlock must overlay the nested block's piece at its base in the
	// quote's run, not at the run's start; assert the seam directly.
	leaf, hbase := htmlLeaf(m, html)
	if leaf != quote || hbase != 7 {
		t.Fatalf("htmlLeaf(%d) = %d, %d, want the quote %d at 7", html, leaf, hbase, quote)
	}
	// The paragraph's spans as htmlNode builds them, cut at the match
	// through that base: the highlight lands on "search", not shifted
	// into the text.
	var out []ui.Span
	var focus ui.Color
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		focus = c.Theme().Focus
		nodes := htmlx.Parse(m.Lines(html))
		var spans []ui.Span
		for _, seg := range nodes[0].Segs {
			spans = append(spans, state.htmlSeg(c, seg, nodes[0].Level))
		}
		out = state.overlayRun(c, leaf, hbase, spans)
	}, 400, 300)
	tt.Frame()
	var covered bool
	for _, sp := range out {
		if sp.Background != (ui.Color{}) && sp.Text == "search" {
			covered = true
			if sp.Background != focus {
				t.Fatalf("match's background %v, want the theme's focus %v", sp.Background, focus)
			}
		}
	}
	if !covered {
		t.Fatalf("spans %v: no piece carries the match's background at base %d", out, hbase)
	}
}
