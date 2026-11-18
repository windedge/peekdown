package shell

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The composed page clears the scrollbar on the right at every level: the
// body rows take the clearance as padding, and the frontmatter card, which
// sits above the list rather than in a row, takes the same room as a right
// margin. Both clear the document card's right edge, where the overlay
// bar now paints, rather than the window's edge.

func TestFrontmatterAndBodyClearTheScrollbar(t *testing.T) {
	s, dir := newShell(t)
	path := filepath.Join(dir, "clip.md")
	src := "---\ntitle: Clip probe\n---\n\n" + strings.Repeat("word ", 200) + "\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	saved := s.Saved
	saved.Layout = "full_width"
	if err := s.Apply(saved); err != nil {
		t.Fatal(err)
	}

	tt := s.renderSize(t, 1400, 600)
	tt.Frame()
	// The window is DIPs in the tester, so the clearance is the same 16.
	clearance := float32(16) // view.rowClearance, the rows' scrollbar clearance
	doc, ok := tt.Find("Document card")
	if !ok {
		t.Fatalf("no document card: %v", tt.Texts())
	}
	card, ok := tt.Find("Frontmatter card")
	if !ok {
		t.Fatalf("no frontmatter card: %v", tt.Texts())
	}
	if got := doc.X + doc.W - (card.X + card.W); got < clearance {
		t.Fatalf("the frontmatter card ends %v DIPs from the document card's edge, want %v", got, clearance)
	}

	// The body's line box clears it too.
	r, ok := paragraphRect(tt, "word")
	if !ok {
		t.Fatalf("texts %v: want the paragraph", tt.Texts())
	}
	if got := doc.X + doc.W - (r.X + r.W); got < 12 {
		t.Fatalf("the body ends %v DIPs from the document card's edge, want >= 12 (rect %v)", got, r)
	}
}

// TestCodeBlockCopyWritesClipboard clicks a code block's copy button: the
// shell wires the view's CopyText to the clipboard and a toast.
func TestCodeBlockCopyWritesClipboard(t *testing.T) {
	s, dir := newShell(t)
	path := filepath.Join(dir, "code.md")
	if err := os.WriteFile(path, []byte("```go\nx := 1\ny := 22\n```\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	tt := s.render(t)
	tt.Frame()
	if err := tt.Click("Copy"); err != nil {
		t.Fatal(err)
	}
	if got, want := tt.Clipboard(), "x := 1\ny := 22"; got != want {
		t.Fatalf("clipboard %q, want %q", got, want)
	}
	if !tt.HasText("Copied code") {
		t.Fatalf("texts %v: no toast after the copy", tt.Texts())
	}
}
