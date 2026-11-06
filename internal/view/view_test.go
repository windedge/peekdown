package view

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown/parser"

	"github.com/windedge/peekdown/internal/doc"
)

const sample = `# Title

Alpha beta gamma delta.

` + "```go" + `
x := 1
y := 22
` + "```" + `

- one
- two

| a | b |
|---|---|
| 1 | 2 |

> quoted

![img](picture.png)
`

func render(t *testing.T, src string) (*doc.Model, *ui.Tester, *State) {
	t.Helper()
	m := doc.New(src)
	state := &State{AnchorOf: anchorsOf(m)}
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		ui.List(c, &state.List, m.Len(), func(r int) {
			Render(c, m, state, m.RowBlock(r).Index)
		})
	}, 640, 400)
	return m, tt, state
}

func texts(tt *ui.Tester) []string { return tt.Texts() }

func has(tt *ui.Tester, want string) bool {
	for _, s := range tt.Texts() {
		if strings.Contains(s, want) {
			return true
		}
	}
	return false
}

func TestViewShowsEveryBlockKind(t *testing.T) {
	m, tt, _ := render(t, sample)
	for _, want := range []string{
		"Title",
		"Alpha beta gamma delta.",
		"x := 1",
		"y := 22",
		"one",
		"two",
	} {
		if !has(tt, want) {
			t.Fatalf("texts %v: want %q", texts(tt), want)
		}
	}
	if m.Len() != 7 {
		t.Fatalf("%d blocks, want 7 (title, text, code, one list, table, quote, image)", m.Len())
	}
	// The rest is below the window: the list builds it as it scrolls there.
	_, tt2, state2 := render(t, sample)
	state2.List.ScrollToEnd()
	tt2.Frame()
	for _, want := range []string{"a", "b", "quoted", "img"} {
		if !has(tt2, want) {
			t.Fatalf("texts %v after the end: want %q", texts(tt2), want)
		}
	}
	_ = m
}

func TestImageParagraphShowsAlt(t *testing.T) {
	_, tt, state := render(t, sample)
	state.List.ScrollToEnd()
	tt.Frame()
	if !has(tt, "img") {
		t.Fatalf("texts %v: want the image description", texts(tt))
	}
}

func TestHeadingRoleAndLevel(t *testing.T) {
	m := doc.New("# H1\n\n## H2\n\n### H3\n")
	var state State
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		ui.List(c, &state.List, m.Len(), func(r int) {
			Render(c, m, &state, m.RowBlock(r).Index)
		})
	}, 400, 300)
	if !tt.HasText("H1") || !tt.HasText("H2") {
		t.Fatalf("texts %v", tt.Texts())
	}
	state.List.ScrollToEnd()
	tt.Frame()
	if !tt.HasText("H3") {
		t.Fatalf("texts %v after the end", tt.Texts())
	}
}

func TestCodeTokensGetColors(t *testing.T) {
	m := doc.New("```go\n// hello\nx := 1\n```\n")
	b := m.Blocks()[0]
	if m.Language(b.Index) != "go" {
		t.Fatalf("language %q", m.Language(b.Index))
	}
	lang := editor.LanguageNamed("go")
	if lang == nil || lang.Tokenizer == nil {
		t.Skip("no go tokenizer")
	}
	toks, _ := lang.Tokenizer.Tokenize("// hello", 0, nil)
	if len(toks) == 0 {
		t.Fatal("no tokens")
	}
	ct := editor.DefaultTheme(ui.LightTheme())
	if ct.TokenColor(toks[0].Kind) == (ui.Color{}) {
		t.Fatalf("token color of %v", toks[0].Kind)
	}
	_ = parser.Default
}

func TestScrollViewScrolls(t *testing.T) {
	long := "# Long\n\n" + strings.Repeat("A paragraph of filler text.\n\n", 60)
	m := doc.New(long)
	var state State
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		ScrollView(c, m, &state)
	}, 400, 300)
	tt.Frame()
	// The list's row numbers do not depend on the fonts' metrics, the
	// heights the rows measure do, so the test reads the place, not the
	// texts: the first frame shows the top, scrolling to the end shows
	// the last row there.
	first, _ := state.List.Visible()
	if first != 0 {
		t.Fatalf("first row %d of a fresh view, want 0", first)
	}
	last := m.Len() - 1
	state.List.ScrollToEnd()
	tt.Frame()
	if got, end := state.List.Visible(); got <= first && end <= last {
		t.Fatalf("visible rows %d..%d after the end: the view did not scroll down", got, end)
	}
	if !state.List.AtEnd() {
		t.Fatalf("the end is out of view after ScrollToEnd")
	}
}

func TestScrollViewAnchor(t *testing.T) {
	var b strings.Builder
	b.WriteString("# Top\n\n")
	for i := range 40 {
		b.WriteString("Filler paragraph number ")
		b.WriteString(strings.Repeat("x", i%7))
		b.WriteString(".\n\n")
	}
	b.WriteString("# Target\n\nEnd text.\n")
	m := doc.New(b.String())
	state := &State{}
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		ScrollView(c, m, state)
	}, 400, 300)
	state.Anchor = "target"
	tt.Frame()
	if !has(tt, "Target") {
		t.Fatalf("texts %v: the target is not in view", texts(tt))
	}
	// The same anchor again without a version bump does not ask the list
	// again: the anchor was consumed the first time.
	state.Anchor = "target"
	tt.Frame()
	if !has(tt, "Target") {
		t.Fatalf("texts %v: the target left the view", texts(tt))
	}
	state.AnchorVersion++
	tt.Frame()
	if !has(tt, "Target") {
		t.Fatalf("texts %v: a new version did not scroll back", texts(tt))
	}
}

func TestAnchorOfModel(t *testing.T) {
	m := doc.New("# One\n\n## One\n")
	var state State
	ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		ScrollView(c, m, &state)
	}, 300, 200)
	if state.AnchorOf == nil {
		t.Fatal("AnchorOf was not set")
	}
	blocks := m.Blocks()
	if got := state.AnchorOf(blocks[0].Index); got != "one" {
		t.Fatalf("AnchorOf(first) = %q", got)
	}
	if got := state.AnchorOf(blocks[1].Index); got != "one-1" {
		t.Fatalf("AnchorOf(second) = %q", got)
	}
}

func TestSplitSpansKeepsStyle(t *testing.T) {
	spans := []ui.Span{
		{Text: "Big ", Size: 28, Weight: 700},
		{Text: "and bold", Size: 28, Weight: 700},
	}
	theme := ui.LightTheme()
	out := splitSpans(spans, []cut{{0, 2, cutSelection}}, theme)
	if len(out) <= len(spans) {
		t.Fatalf("%d spans after the cut, want more than %d", len(out), len(spans))
	}
	if out[0].Background != theme.Selection {
		t.Fatalf("first span's background %v, want the selection's %v", out[0].Background, theme.Selection)
	}
	for i, sp := range out {
		if sp.Weight != 700 || sp.Size != 28 {
			t.Fatalf("span %d (%q): Weight %d, Size %v", i, sp.Text, sp.Weight, sp.Size)
		}
	}
	var text strings.Builder
	for _, sp := range out {
		text.WriteString(sp.Text)
	}
	if text.String() != "Big and bold" {
		t.Fatalf("texts %q", text.String())
	}
}

func TestCodeBlockTokenColors(t *testing.T) {
	lang := editor.LanguageNamed("go")
	if lang == nil || lang.Tokenizer == nil {
		t.Skip("no go tokenizer")
	}
	toks, _ := lang.Tokenizer.Tokenize("// hello", 0, nil)
	if len(toks) == 0 {
		t.Fatal("no tokens")
	}
	ct := editor.DefaultTheme(ui.LightTheme())
	base := ui.LightTheme().Text
	spans := codeSpans("// hello", toks, ct, base)
	if len(spans) == 0 {
		t.Fatal("no spans")
	}
	want := ct.TokenColor(editor.TokenComment)
	var found bool
	for _, sp := range spans {
		if sp.Text == "" {
			t.Fatalf("empty span among %v", spans)
		}
		if sp.Text == "// hello" && sp.Color == want {
			found = true
		}
	}
	if !found {
		t.Fatalf("no span %q in %v with the comment color %v", "// hello", spans, want)
	}
}

// TestLayoutCacheInvalidatedBySet renders doc A, fills the layout cache
// through a pointer hit, then Sets doc B: the handler's press sees the
// version and drops the cache and the selection, which named blocks of
// the older document.
func TestLayoutCacheInvalidatedBySet(t *testing.T) {
	m := doc.New("Alpha beta gamma.")
	var state State
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		list := ui.List(c, &state.List, m.Len(), func(r int) {
			state.Row(c, m, r)
		}).Grow(1)
		list.HandleInput(state.Handler(m))
	}, 400, 300)
	tt.Frame()
	x, y := findText(t, tt, "Alpha", false)
	tt.Press(x, y)
	tt.Release(x, y)
	if len(state.layouts) == 0 || !state.Selection.set {
		t.Fatalf("layouts %v, selection %v: the hit filled neither", state.layouts, state.Selection.set)
	}
	old := state.layouts[layoutKey{block: m.Blocks()[0].Index, width: 400, size: 15}]
	m.Set("Completely different text here.")
	// The next press drops the stale cache and the old selection, then
	// refills the cache from the new document in the same event.
	state.Handler(m)(ui.InputEvent{Kind: ui.InputPointerDown, Button: 0})
	for key, e := range state.layouts {
		if e == old {
			t.Fatalf("stale entry %v survived the new document", key)
		}
	}
	if p := state.Selection.start; state.Selection.set && p.Block != m.Blocks()[0].Index {
		t.Fatalf("selection %+v points outside the new document", state.Selection)
	}
	// The refilled entry shapes the new text: a layout carries no text,
	// so compare the width it reports against a fresh shaping.
	var entry *textLayout
	for key, e := range state.layouts {
		if key.block == m.Blocks()[0].Index {
			entry = e
		}
	}
	if entry == nil {
		t.Fatalf("no entry for block %d in %v", m.Blocks()[0].Index, state.layouts)
	}
	w, _ := ui.ShapeText(m.Plain(m.Blocks()[0].Index), ui.Font{Size: 15}, 10000).Size()
	if cw, _ := entry.layout.Size(); cw != w {
		t.Fatalf("cached layout width %v, want %v for the new text", cw, w)
	}
}

// TestLayoutUsesHeadingSize checks the cache keys by font size: a
// heading's shaped layout takes its level's size, a paragraph's the
// body's 15, so hit testing reshapes at what the row draws.
func TestLayoutUsesHeadingSize(t *testing.T) {
	m := doc.New("# Title\n\nBody text here.\n")
	var state State
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		list := ui.List(c, &state.List, m.Len(), func(r int) {
			state.Row(c, m, r)
		}).Grow(1)
		list.HandleInput(state.Handler(m))
	}, 400, 300)
	tt.Frame()
	x, y := findText(t, tt, "Title", false)
	tt.Press(x, y)
	tt.Release(x, y)
	x2, y2 := findText(t, tt, "Body", false)
	tt.Press(x2, y2)
	tt.Release(x2, y2)
	b1, b2 := m.Blocks()[0].Index, m.Blocks()[1].Index
	var heading, body *textLayout
	for key, e := range state.layouts {
		switch {
		case key.block == b1 && key.size == 28:
			heading = e
		case key.block == b2 && key.size == 15:
			body = e
		}
	}
	if heading == nil || body == nil {
		t.Fatalf("layouts %v: want a heading entry at 28 and a body entry at 15", state.layouts)
	}
}

// TestTableColumnsConsistent sizes a table whose second column is much
// longer: per-column widths, each capped at 300, so every row reads the
// same widths.
func TestTableColumnsConsistent(t *testing.T) {
	m := doc.New("| a | bbbb... |\n|---|---|\n| 1 | 2 |\n")
	widths := tableColumnWidths(m, m.Blocks()[0].Index)
	if len(widths) != 2 {
		t.Fatalf("%d widths, want 2", len(widths))
	}
	if widths[1] <= widths[0] {
		t.Fatalf("widths %v: the long column is not wider", widths)
	}
	for k, w := range widths {
		if w > 300 {
			t.Fatalf("width %d = %v, over the cap", k, w)
		}
	}
}

func TestAnchorScrollRequestBeforeLayout(t *testing.T) {
	m := doc.New("# Top\n\nMiddle.\n\n# Bottom\n\nEnd.\n")
	var state State
	state.Anchor = "bottom"
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		ScrollView(c, m, &state)
	}, 300, 200)
	_ = tt
	if state.Anchor != "bottom" {
		t.Fatal("anchor consumed")
	}
}

// TestMergeCutsPriority checks the overlap rule: the current match wins
// over the selection, the selection over a plain match, and the loser
// keeps the runes the winner does not cover instead of vanishing.
func TestMergeCutsPriority(t *testing.T) {
	theme := ui.LightTheme()
	out := mergeCuts([]cut{{0, 10, cutMatch}, {2, 4, cutCurrent}}, 10)
	want := []segment{{0, 2, cutMatch}, {2, 4, cutCurrent}, {4, 10, cutMatch}}
	if len(out) != len(want) {
		t.Fatalf("segments %v, want %v", out, want)
	}
	for k, s := range out {
		if s != want[k] {
			t.Fatalf("segment %d = %v, want %v", k, s, want[k])
		}
	}
	out = mergeCuts([]cut{{0, 10, cutSelection}, {0, 5, cutCurrent}}, 10)
	want = []segment{{0, 5, cutCurrent}, {5, 10, cutSelection}}
	if len(out) != len(want) {
		t.Fatalf("segments %v, want %v", out, want)
	}
	for k, s := range out {
		if s != want[k] {
			t.Fatalf("segment %d = %v, want %v", k, s, want[k])
		}
	}
	_ = theme
}

// TestQuoteTextMuted checks the quote's paragraphs draw in the muted
// color: spans used to overwrite the style's color with the plain text
// color, swallowing what the quote passed in.
func TestQuoteTextMuted(t *testing.T) {
	m := doc.New("> quoted wisdom\n")
	quote := m.Blocks()[0]
	leaf := m.QuoteBlocks(quote.Index)[0]
	var got []ui.Span
	_ = ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		t := c.Theme()
		var st State
		got = st.spans(c, m, leaf, style{color: t.TextMuted})
	}, 400, 300)
	if len(got) == 0 {
		t.Fatal("no spans")
	}
	theme := ui.LightTheme()
	for _, sp := range got {
		if sp.Color == theme.Text {
			t.Fatalf("span %q draws in Text, want TextMuted", sp.Text)
		}
		if sp.Color != theme.TextMuted {
			t.Fatalf("span %q color %v, want %v", sp.Text, sp.Color, theme.TextMuted)
		}
	}
}

// TestLinkSpanStyled checks the link's text draws as a link does: the
// accent color, underlined.
func TestLinkSpanStyled(t *testing.T) {
	m := doc.New("a [link](x) here\n")
	b := m.Blocks()[0]
	var got []ui.Span
	_ = ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		var st State
		got = st.spans(c, m, b.Index, style{})
	}, 400, 300)
	theme := ui.LightTheme()
	for _, sp := range got {
		if sp.Text == "link" {
			if !sp.Underline || sp.Color != theme.Accent {
				t.Fatalf("link span %v: want Underline and Accent %v", sp, theme.Accent)
			}
			return
		}
	}
	t.Fatalf("no span %q in %v", "link", got)
}
