package doc

import (
	"testing"
)

func TestModelWithFrontmatter(t *testing.T) {
	src := "---\ntitle: Note\ntags:\n  - a\n---\n\n# Real Title\n\nBody text with title here.\n"
	m := New(src)
	if m.Front == nil {
		t.Fatalf("Front nil")
	}
	if m.Front.Format != "yaml" || m.Front.Broken {
		t.Fatalf("Front %+v", m.Front)
	}
	if len(m.Headings()) != 1 || m.Headings()[0].Text != "Real Title" {
		t.Fatalf("headings %v, want [Real Title]", m.Headings())
	}
	blocks := m.Blocks()
	if len(blocks) == 0 || blocks[0].Kind.String() != "heading" {
		t.Fatalf("block 0 %v, want a heading", blocks[0])
	}
}

func TestModelWithoutFrontmatter(t *testing.T) {
	m := New("# Title\n\nBody.\n")
	if m.Front != nil {
		t.Fatalf("Front %v, want nil", m.Front)
	}
}

func TestSearchExcludesFrontmatter(t *testing.T) {
	// The frontmatter is not document text: the search runs over the body
	// only, its own title does not match. The two hits are the heading and
	// the paragraph of the body.
	src := "---\ntitle: Note\n---\n\n# Real Title\n\nBody text with title here.\n"
	m := New(src)
	m.SetSearch("title", SearchOptions{})
	if len(m.Matches) != 2 {
		matches := []string{}
		for _, match := range m.Matches {
			matches = append(matches, m.RunText(*m.RunOf(match.Block), match.Start, match.End))
		}
		t.Fatalf("%d matches %v, want 2 in the body", len(m.Matches), matches)
	}
}

func TestSearchExcludesBrokenFrontmatter(t *testing.T) {
	// The broken block is shown as a code block by the view, but like any
	// frontmatter block it is not document text: the search skips it.
	src := "---\n: :\n---\n\n# Title\n\nbody with needle here\n"
	m := New(src)
	m.SetSearch("needle", SearchOptions{})
	if len(m.Matches) != 1 {
		t.Fatalf("%d matches, want 1 in the body", len(m.Matches))
	}
}

func TestFrontmatterOffsetContract(t *testing.T) {
	// Line contract: the body's line 1 is the source's line
	// Front.LineCount+1; the body block offsets are relative to the body.
	src := "---\ntitle: Note\nsecond line\n---\n\n# H\n\nbody\n"
	m := New(src)
	if m.Front == nil || m.Front.LineCount != 4 {
		t.Fatalf("Front %+v, want LineCount 4", m.Front)
	}
	b := m.Block(1)
	if b.Kind.String() != "heading" {
		t.Fatalf("block 1 %v, want a heading", b)
	}
	if m.Source()[b.Start:b.End] != "# H" {
		t.Fatalf("block 0 source %q, want \"# H\"", m.Source()[b.Start:b.End])
	}
}
