package shell

import (
	"os"
	"path/filepath"
	"testing"
)

// TestContentShowsFrontmatter opens a document with frontmatter: the card
// shows above the body, and its copy button puts Raw on the clipboard and
// tells the user with a toast.
func TestContentShowsFrontmatter(t *testing.T) {
	s, dir := newShell(t)
	fm := filepath.Join(dir, "front.md")
	src := "---\ntitle: Note\ntags:\n  - a\n  - b\n---\n\n# Real Title\n\nbody\n"
	if err := os.WriteFile(fm, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tt := s.render(t)
	if err := s.OpenFile(fm); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	for _, want := range []string{"yaml", "title", "Note", "tags", "a, b", "Copy YAML"} {
		if !tt.HasText(want) {
			t.Fatalf("texts %v: want %q", tt.Texts(), want)
		}
	}
	// The body renders below the card: the frontmatter is not document
	// text, the heading keeps its anchor and the search.
	if !tt.HasText("Real Title") {
		t.Fatalf("texts %v: the body is gone", tt.Texts())
	}

	if err := tt.Click("Copy YAML"); err != nil {
		t.Fatal(err)
	}
	if got := tt.Clipboard(); got != "title: Note\ntags:\n  - a\n  - b\n" {
		t.Fatalf("clipboard %q, want the frontmatter's Raw", got)
	}
	if !tt.HasText("Copied frontmatter") {
		t.Fatalf("texts %v: no toast after the copy", tt.Texts())
	}

	// A document without frontmatter shows no card.
	s.OpenFile(filepath.Join(dir, "a.md"))
	tt.Frame()
	if tt.HasText("Copy YAML") {
		t.Fatalf("texts %v: the card outlived its document", tt.Texts())
	}
}
