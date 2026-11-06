package doc

import (
	"reflect"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui/markdown/parser"
)

const sample = `# Title

First paragraph.

## Second heading

A paragraph with [a link](https://example.com) and ` + "`code`" + ` and **bold**.

- one
- two
  - nested

1. first
2. second

> quoted text

` + "```go" + `
x := 1
y := 2
` + "```" + `

| a | b |
|---|---|
| 1 | 2 |

---

![alt](img.png)

<div>
<p>HTML here</p>
</div>

- [ ] todo
- [x] done
`

func parseSample(t *testing.T) *Model {
	t.Helper()
	return New(sample)
}

func TestTopLevelBlocks(t *testing.T) {
	m := parseSample(t)
	blocks := m.Blocks()
	kinds := []parser.Kind{}
	for _, b := range blocks {
		kinds = append(kinds, b.Kind)
	}
	want := []parser.Kind{
		parser.Heading, parser.Paragraph, parser.Heading, parser.Paragraph,
		parser.List, parser.List, parser.Quote, parser.CodeBlock,
		parser.Table, parser.ThematicBreak, parser.Paragraph,
		parser.HTMLBlock, parser.List,
	}
	if len(kinds) != len(want) {
		t.Fatalf("%d blocks %v, want %d", len(kinds), kinds, len(want))
	}
	for i, k := range want {
		if kinds[i] != k {
			t.Fatalf("block %d: got %v, want %v (all: %v)", i, kinds[i], k, kinds)
		}
	}
}

func TestHeadings(t *testing.T) {
	m := parseSample(t)
	hs := m.Headings()
	if len(hs) != 2 {
		t.Fatalf("%d headings, want 2", len(hs))
	}
	if hs[0].Level != 1 || hs[0].Text != "Title" {
		t.Fatalf("heading 0: %+v", hs[0])
	}
	if hs[1].Level != 2 || hs[1].Text != "Second heading" {
		t.Fatalf("heading 1: %+v", hs[1])
	}
	if hs[0].BlockIndex >= hs[1].BlockIndex {
		t.Fatalf("headings out of order: %+v", hs)
	}
}

func TestSlug(t *testing.T) {
	// The Rust rules: lowercase, each whitespace rune to one dash (no
	// collapsing), keep Unicode letters, digits and dashes, trim dashes.
	cases := map[string]string{
		"Hello World":     "hello-world",
		"Hello  World!":   "hello--world",
		"  trim me  ":     "trim-me",
		"C++ and C#":      "c-and-c",
		"Ünïcode Tïtle":   "ünïcode-tïtle",
		"foo_bar":         "foobar",
		"A -- B":          "a----b",
		"Tabs\tHere":      "tabs-here",
		"already-slug":    "already-slug",
		"CAPS and MiXeD":  "caps-and-mixed",
		"trailing dash -": "trailing-dash",
		"!!!":             "",
		"日本語タイトル":         "日本語タイトル",
		"one/two:three":   "onetwothree",
		"one  two":        "one--two",
		"one-two":         "one-two",
	}
	for in, want := range cases {
		if got := Slug(in); got != want {
			t.Errorf("Slug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSlugDeduplicates(t *testing.T) {
	m := New("# A\n\n## A\n\n### A\n\n#### A\n")
	hs := m.Headings()
	got := []string{}
	for _, h := range hs {
		got = append(got, m.Anchor(h.BlockIndex))
	}
	want := []string{"a", "a-1", "a-2", "a-3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("anchors %v, want %v", got, want)
	}
}

func TestSlugDedupesAgainstLiteralSuffixes(t *testing.T) {
	// GitHub numbering: walk slug-1, slug-2, ... to the first free
	// anchor, whether the collision is a repeat or a literal slug.
	// # a        -> a
	// ## a-1     -> a-1
	// ### a      -> a taken, a-1 taken, a-2 free
	// #### a-1   -> a-1 taken, a-1-1 free
	// #### a     -> a, a-1, a-2 taken, a-3 free
	m := New("# a\n## a-1\n### a\n#### a-1\n#### a\n")
	hs := m.Headings()
	got := []string{}
	for _, h := range hs {
		got = append(got, m.Anchor(h.BlockIndex))
	}
	want := []string{"a", "a-1", "a-2", "a-1-1", "a-3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("anchors %v, want %v", got, want)
	}
}

func TestAnchor(t *testing.T) {
	m := parseSample(t)
	if got := m.Anchor(m.Blocks()[0].Index); got != "title" {
		t.Fatalf("anchor of the title: %q", got)
	}
	if got := m.Anchor(m.Blocks()[2].Index); got != "second-heading" {
		t.Fatalf("anchor of the second heading: %q", got)
	}
	if _, ok := m.AnchorBySlug("title"); !ok {
		t.Fatal("AnchorBySlug missed the title")
	}
}

func TestHeadingBySlug(t *testing.T) {
	m := parseSample(t)
	i, ok := m.HeadingBySlug("second-heading")
	if !ok || m.Block(i).Kind != parser.Heading {
		t.Fatalf("HeadingBySlug: %v %v", i, ok)
	}
	if _, ok := m.HeadingBySlug("nope"); ok {
		t.Fatal("unknown slug found")
	}
}

func TestBlockSourceAndText(t *testing.T) {
	m := parseSample(t)
	i := m.Blocks()[3].Index // the paragraph with the link
	src := m.Source()[m.Block(i).Start:m.Block(i).End]
	if !strings.Contains(src, "a link") {
		t.Fatalf("block source %q", src)
	}
	if got := m.Plain(i); !strings.Contains(got, "a link") || strings.Contains(got, "](") {
		t.Fatalf("plain text %q", got)
	}
}

func TestImageParagraph(t *testing.T) {
	m := parseSample(t)
	i := m.Blocks()[10].Index
	images := m.Images(i)
	if len(images) != 1 || images[0].Dest != "img.png" || images[0].Alt != "alt" {
		t.Fatalf("images: %+v", images)
	}
}

func TestMixedTextAndImage(t *testing.T) {
	m := New("Text before ![alt](x.png) text after.\n")
	i := m.Blocks()[0].Index
	if got := m.Images(i); len(got) != 0 {
		t.Fatalf("mixed paragraph: %+v", got)
	}
}

func TestSetReparsesIncrementally(t *testing.T) {
	m := parseSample(t)
	n := len(m.Blocks())
	m.Set(sample + "\nAppended paragraph.\n")
	if len(m.Blocks()) <= n {
		t.Fatalf("append did not add a block: %d -> %d", n, len(m.Blocks()))
	}
	last := m.Blocks()[len(m.Blocks())-1]
	if last.Kind != parser.Paragraph {
		t.Fatalf("last block: %v", last.Kind)
	}
}

func TestUpdateInPlace(t *testing.T) {
	m := New("First.\n")
	m.Set("Second.\n")
	if got := m.Plain(m.Blocks()[0].Index); got != "Second." {
		t.Fatalf("plain after edit: %q", got)
	}
	hs := m.Headings()
	if len(hs) != 0 {
		t.Fatalf("headings after edit: %+v", hs)
	}
}

func TestTaskItems(t *testing.T) {
	m := New("- [ ] todo\n- [x] done\n")
	list := m.Blocks()[0]
	items := m.ListItems(list.Index)
	if len(items) != 2 {
		t.Fatalf("%d items", len(items))
	}
	if items[0].Task != parser.Unchecked || items[1].Task != parser.Checked {
		t.Fatalf("tasks: %+v", items)
	}
}

func TestOrderedMarkers(t *testing.T) {
	m := New("1. first\n2. second\n\n- bullet\n")
	ordered, bullets := m.Blocks()[0], m.Blocks()[1]
	if !ordered.Ordered || ordered.Number != 1 {
		t.Fatalf("ordered: %+v", ordered)
	}
	if bullets.Ordered {
		t.Fatalf("bullets ordered: %+v", bullets)
	}
}

func TestNewlineOutsideParagraphIsSpace(t *testing.T) {
	m := New("line one\nline two\n")
	if got := m.Plain(m.Blocks()[0].Index); got != "line one line two" {
		t.Fatalf("plain: %q", got)
	}
}
