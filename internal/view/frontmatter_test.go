package view

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/doc"
)

// frontCard builds a card for src's frontmatter alone, no document list.
func frontCard(t *testing.T, src string) (*doc.Model, *ui.Tester, *State) {
	t.Helper()
	m := doc.New(src)
	if m.Front == nil {
		t.Fatalf("%q has no frontmatter for the card", src)
	}
	var state State
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		Frontmatter(c, m, &state)
	}, 480, 400)
	return m, tt, &state
}

func TestFrontmatterRendersKeyValues(t *testing.T) {
	_, tt, _ := frontCard(t, "---\ntitle: Note\ndraft: true\nrating: 4.5\nempty:\ntags:\n  - a\n  - b\nauthor:\n  name: Alice\nroles:\n  - admin: true\n---\n\n# Body\n")
	for _, want := range []string{
		"title",
		"Note",
		"draft",
		"true",
		"rating",
		"4.5",
		"empty",
		"~",
		"tags",
		"a, b",
		"author",
		"name=Alice",
		// A map nested inside a list stays collapsed, one {...} marker;
		// only a top-level map renders its pairs.
		"roles",
		"{...}",
	} {
		if !has(tt, want) {
			t.Fatalf("texts %v: want %q", texts(tt), want)
		}
	}
}

func TestFrontmatterLongListTruncated(t *testing.T) {
	// A list past 120 runes shows cut with ..., and its full text rides
	// the tooltip.
	long := strings.Repeat("word ", 40)
	_, tt, _ := frontCard(t, "---\ntags:\n  - "+long+"\n---\n\nbody\n")
	if !has(tt, "...") {
		t.Fatalf("texts %v: want the ellipsis", texts(tt))
	}
	full := long + " " // the last item keeps its trailing space
	if has(tt, full) {
		t.Fatalf("texts %v: the full value should be cut, not shown", texts(tt))
	}
}

func TestFrontmatterKeyColumnAligned(t *testing.T) {
	_, tt, _ := frontCard(t, "---\nshort: a\na-much-longer-key: b\nmid: c\n---\n\nbody\n")
	x, ok := tt.Find("short")
	if !ok {
		t.Fatalf("texts %v: no short", texts(tt))
	}
	for _, key := range []string{"a-much-longer-key", "mid"} {
		kx, ok := tt.Find(key)
		if !ok {
			t.Fatalf("texts %v: no %q", texts(tt), key)
		}
		if kx.X != x.X {
			t.Fatalf("key %q at x %v, %q at %v: the key column does not line up", key, kx.X, "short", x.X)
		}
	}
	// The values start one widest-key-plus-gap to the right of the keys,
	// the same x for every row.
	wide, _ := ui.ShapeText("a-much-longer-key", ui.Font{Family: fontMono, Size: 13}, 10000).Size()
	for _, v := range []string{"a", "b", "c"} {
		vx, ok := tt.Find(v)
		if !ok {
			t.Fatalf("texts %v: no value %q", texts(tt), v)
		}
		if vx.X != x.X+wide+fmKeyGap {
			t.Fatalf("value %q at x %v, want %v (keys %v + gap)", v, vx.X, x.X+wide+fmKeyGap, x.X)
		}
	}
}

func TestFrontmatterBrokenFallsBackToCodeBlock(t *testing.T) {
	_, tt, _ := frontCard(t, "---\n: :\n---\n\n# Body\n")
	if !has(tt, "yaml") {
		t.Fatalf("texts %v: want the yaml tag", texts(tt))
	}
	if !has(tt, ": :") {
		t.Fatalf("texts %v: want the raw text", texts(tt))
	}
}

func TestFrontmatterTomlShownAsToml(t *testing.T) {
	m := doc.New("+++\ntitle = \"TOML Note\"\n+++\n\n# Body\n")
	var state State
	state.CopyFrontmatter = func(string) {}
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		Frontmatter(c, m, &state)
	}, 480, 400)
	if !has(tt, "toml") {
		t.Fatalf("texts %v: want the toml tag", texts(tt))
	}
	if !has(tt, "title = \"TOML Note\"") {
		t.Fatalf("texts %v: want the raw TOML", texts(tt))
	}
	// The TOML is never a table: its key shows only inside the raw text.
	for _, s := range texts(tt) {
		if s == "title" {
			t.Fatalf("texts %v: TOML leaked into the key-value table", texts(tt))
		}
	}
	// The button names what the click copies.
	if err := tt.Click("Copy TOML"); err != nil {
		t.Fatalf("texts %v: no Copy TOML button: %v", texts(tt), err)
	}
}

func TestFrontmatterEmptyRendersNothing(t *testing.T) {
	// An empty block (or one of comments only) is a valid frontmatter
	// with no rows: no card at all, and never the panic of a table over
	// zero keys.
	for _, src := range []string{"---\n---\n\n# Body\n", "---\n# just a comment\n---\n\n# Body\n"} {
		_, tt, _ := frontCard(t, src)
		if len(texts(tt)) != 0 {
			t.Fatalf("texts %v: the empty card drew something for %q", texts(tt), src)
		}
	}
}

func TestFrontmatterLongKeyKeepsValue(t *testing.T) {
	// A very wide key column is capped: the key ellipsizes but the row
	// and its value survive.
	long := strings.Repeat("key-", 20)
	_, tt, _ := frontCard(t, "---\n"+long+": value-that-must-show\n---\n\nbody\n")
	if !has(tt, long) {
		t.Fatalf("texts %v: want the long key", texts(tt))
	}
	if !has(tt, "value-that-must-show") {
		t.Fatalf("texts %v: the value is crowded out by the key", texts(tt))
	}
}

func TestFrontmatterCopyButton(t *testing.T) {
	src := "---\ntitle: Note\n---\n\n# Body\n"
	m := doc.New(src)
	var got string
	state := &State{CopyFrontmatter: func(s string) { got = s }}
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		Frontmatter(c, m, state)
	}, 480, 200)
	if err := tt.Click("Copy YAML"); err != nil {
		t.Fatalf("no copy button: %v in %v", err, texts(tt))
	}
	if got != m.Front.Raw {
		t.Fatalf("callback got %q, want Raw %q", got, m.Front.Raw)
	}

	// nil CopyFrontmatter hides the button.
	m3 := doc.New(src)
	var state3 State
	tt3 := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		Frontmatter(c, m3, &state3)
	}, 480, 200)
	if err := tt3.Click("Copy YAML"); err == nil {
		t.Fatalf("the button shows without CopyFrontmatter: %v", texts(tt3))
	}
}

// TestFrontmatterHiddenWithoutFront keeps the card out of documents that
// open plainly.
func TestFrontmatterHiddenWithoutFront(t *testing.T) {
	m := doc.New("# Plain\n")
	var state State
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		Frontmatter(c, m, &state)
	}, 480, 200)
	if len(texts(tt)) != 0 {
		t.Fatalf("texts %v: the empty card drew something", texts(tt))
	}
}
