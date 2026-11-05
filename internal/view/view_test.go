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
	before := len(tt.Texts())
	state.List.ScrollToEnd()
	tt.Frame()
	after := len(tt.Texts())
	if after <= before {
		t.Fatalf("texts before %d, after %d: the end did not build more rows", before, after)
	}
	_ = m
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
