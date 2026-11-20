package view

import (
	"strings"
	"testing"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/doc"
)

// linkDestAtCases covers the runes the hit tests address: a table, a
// list or a quote flattens its leaves into one run, so the offset a
// click reports has to find the leaf whose inlines hold the link.
func TestLinkDestAtFallsBackToLeafInlines(t *testing.T) {
	cases := []struct {
		name, src, label, dest string
	}{
		{"list", "- Go 基线：[C:/xxx](C:/xxx)\n", "C:/xxx", "C:/xxx"},
		{"nested list", "- Outer\n  - [Inner](https://example.com/inner)\n", "Inner", "https://example.com/inner"},
		{"quote", "> Read [the guide](https://example.com/guide) now.\n", "the guide", "https://example.com/guide"},
		{"footnote", "Text[^n].\n\n[^n]: See [the note](https://example.com/note).\n", "the note", "https://example.com/note"},
		{"paragraph", "Read [the guide](https://example.com/guide) now.\n", "the guide", "https://example.com/guide"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := doc.New(tc.src)
			off := -1
			for r := 0; r < m.Len(); r++ {
				block := m.RowBlock(r).Index
				run := m.RunOf(block)
				if run == nil {
					continue
				}
				if byteOff := strings.Index(run.Text, tc.label); byteOff >= 0 {
					off = utf8.RuneCountInString(run.Text[:byteOff])
					if got := linkDestAt(m, block, off); got != tc.dest {
						t.Fatalf("linkDestAt(block %d, %d) = %q, want %q (run %q)", block, off, got, tc.dest, run.Text)
					}
					break
				}
			}
			if off < 0 {
				t.Fatalf("no run holds %q in %q", tc.label, tc.src)
			}
		})
	}
}

func TestLeafAtMapsContainerOffsetToOneLeaf(t *testing.T) {
	m := doc.New("- First leaf\n- Second [leaf](https://example.com/second)\n")
	list := m.RowBlock(0).Index
	run := m.RunOf(list)
	if run == nil {
		t.Fatal("the list has no run")
	}
	off := runeLen(run.Text[:strings.Index(run.Text, "Second")])
	leaf, leafOff := m.LeafAt(list, off+7)
	if leaf == list {
		t.Fatalf("LeafAt(%d, %d) did not leave the list", list, off+7)
	}
	if got := m.Plain(leaf); !strings.Contains(got, "Second") {
		t.Fatalf("leaf %d holds %q, want the second item", leaf, got)
	}
	if leafOff != 7 {
		t.Fatalf("leaf offset = %d, want 7", leafOff)
	}
}

// The regression sentinel for the reported ticket: the link lives in
// a list item's leaf paragraph, and the click must reach the shell's
// local-link callback with the raw destination.
func TestClickOnLinkInsideListItemOpensLocalPath(t *testing.T) {
	src := "# v0.3.0 验收矩阵\n\n- Go 基线：HEAD `95ef5a3`；Rust 对照源：[C:/Seafile/Works/rust/peekdown](C:/Seafile/Works/rust/peekdown)（README）。\n"
	_, tt, _, opened, local := localLinkDoc(t, src, 900, 700)
	x, y := linkPointUTF16(t, tt, "C:/Seafile/Works/rust/peekdown")
	tt.ClickAt(x, y)
	if len(*opened) != 0 {
		t.Fatalf("opened %v, a local path must not reach the browser", *opened)
	}
	if len(*local) != 1 || (*local)[0] != "C:/Seafile/Works/rust/peekdown" {
		t.Fatalf("local %v, want the list item's directory link", *local)
	}
}

// linkPointUTF16 is linkPoint with the UTF-16 index ShapeText's caret
// expects: the root case's Chinese prefix makes a byte index land well
// past the link's glyphs.
func linkPointUTF16(t *testing.T, tt *ui.Tester, want string) (float32, float32) {
	t.Helper()
	for _, label := range tt.Texts() {
		if !strings.Contains(label, want) {
			continue
		}
		r, ok := tt.Find(label)
		if !ok {
			continue
		}
		text := strings.SplitN(label, "\n", 2)[0]
		a := strings.Index(text, want)
		from := len(utf16.Encode([]rune(text[:a])))
		to := len(utf16.Encode([]rune(text[:a+len(want)])))
		layout := ui.ShapeText(text, ui.Font{Size: defaultBodySize}, 10000)
		mid := (layout.Caret(from).X + layout.Caret(to).X) / 2
		if mid <= 0 {
			t.Fatalf("no caret boxes for %q in %q", want, text)
		}
		return r.X + mid, r.Y + r.H/2
	}
	t.Fatalf("texts %v: %q not found", tt.Texts(), want)
	return 0, 0
}

func TestClickOnLinkInsideQuoteOpensURL(t *testing.T) {
	_, tt, _, opened := linkDoc(t, "> Read [the guide](https://example.com/guide) now.\n", 500, 300)
	x, y := linkPoint(t, tt, "the guide")
	tt.ClickAt(x, y)
	if len(*opened) != 1 || (*opened)[0] != "https://example.com/guide" {
		t.Fatalf("opened %v, want the quote's URL", *opened)
	}
}
