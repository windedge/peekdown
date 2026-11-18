package shell

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo"
)

// cardShotPath is where the card-layer screenshots land, "" unless the
// env asks for them: the regular runs stay headless-clean.
var cardShotPath = func() string {
	if os.Getenv("PEEKDOWN_SHOT") == "" {
		return ""
	}
	return "C:/tmp/card-"
}()

// cardShotDoc is an open document with the surfaces the card layers
// have to sit under: frontmatter, a code panel and headings for the
// outline.
func cardShotDoc(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "card.md")
	src := "---\ntitle: Card layers\ntags:\n  - layout\n---\n\n" +
		"# Heading one\n\nA paragraph with **bold** text and a [link](https://example.com).\n\n" +
		"```go\nfunc main() { println(\"hello\") }\n```\n\n" +
		"## Heading two\n\nAnother paragraph to fill the card.\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestCardShot writes the document, welcome and dark card-layer
// screenshots: the white document card, the translucent side cards and
// the gradient gaps between them. Skipped unless PEEKDOWN_SHOT is set.
func TestCardShot(t *testing.T) {
	if cardShotPath == "" {
		t.Skip("set PEEKDOWN_SHOT=1 to write the card screenshots")
	}
	s, _ := newShell(t)
	s.Saved.AccentTheme = "apricot"
	s.Sidebar.Visible = true
	if err := s.OpenFile(cardShotDoc(t)); err != nil {
		t.Fatal(err)
	}
	tt := s.renderSize(t, 1440, 900)
	tt.Frame()
	writePng(t, tt.Image(), cardShotPath+"content.png")

	// The welcome page stays centered on the bare gradient.
	w, _ := newShell(t)
	w.Saved.AccentTheme = "apricot"
	tw := w.renderSize(t, 1440, 900)
	tw.Frame()
	writePng(t, tw.Image(), cardShotPath+"welcome.png")

	// The dark host needs its setter wired to the tester's dark flag.
	d, _ := newShell(t)
	d.Saved.AccentTheme = "apricot"
	d.Sidebar.Visible = true
	if err := d.OpenFile(cardShotDoc(t)); err != nil {
		t.Fatal(err)
	}
	td := d.renderSize(t, 1440, 900)
	d.SetTheme = func(src mygo.ThemeSource) { td.SetDark(src == mygo.ThemeDark) }
	td.SetDark(true)
	td.Frame()
	writePng(t, td.Image(), cardShotPath+"dark.png")
}
