package shell

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
)

// paragraphRect returns the window rect of the element drawn with want,
// for the layout assertions to read the body's width from.
func paragraphRect(tt *ui.Tester, want string) (ui.Rect, bool) {
	for _, l := range tt.Texts() {
		if strings.Contains(l, want) {
			return tt.Find(l)
		}
	}
	return ui.Rect{}, false
}

// TestLayoutAppliedToContent walks the saved layout: centered caps the
// body at 900 DIPs and centers it, full_width follows the window.
func TestLayoutAppliedToContent(t *testing.T) {
	s, dir := newShell(t)
	long := strings.Repeat("word ", 120)
	path := filepath.Join(dir, "long.md")
	if err := os.WriteFile(path, []byte(long+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	saved := s.Saved
	saved.Layout = "centered"
	if err := s.Apply(saved); err != nil {
		t.Fatal(err)
	}

	tt := s.renderSize(t, 1400, 400)
	tt.Frame()
	r, ok := paragraphRect(tt, "word")
	if !ok {
		t.Fatalf("texts %v: want the paragraph", tt.Texts())
	}
	if r.W > 900 || r.X < 200 {
		t.Fatalf("centered body at x=%v w=%v, want the 900 DIP centered column", r.X, r.W)
	}

	saved.Layout = "full_width"
	if err := s.Apply(saved); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	r2, ok := paragraphRect(tt, "word")
	if !ok {
		t.Fatalf("texts %v after full_width: want the paragraph", tt.Texts())
	}
	if r2.W <= r.W {
		t.Fatalf("full_width body w=%v, want wider than centered %v", r2.W, r.W)
	}
	if r2.X >= r.X {
		t.Fatalf("full_width body at x=%v, want it pulled left of centered %v", r2.X, r.X)
	}
}

// TestApplySidebarTab walks the saved tab: Apply selects it, an unknown
// name falls back to the explorer.
func TestApplySidebarTab(t *testing.T) {
	s, _ := newShell(t)
	saved := s.Saved
	saved.SidebarTab = "outline"
	if err := s.Apply(saved); err != nil {
		t.Fatal(err)
	}
	if s.Sidebar.Tab != 1 {
		t.Fatalf("tab = %d, want 1", s.Sidebar.Tab)
	}
	saved.SidebarTab = "nonsense"
	if err := s.Apply(saved); err != nil {
		t.Fatal(err)
	}
	if s.Sidebar.Tab != 0 {
		t.Fatalf("tab for a bad name = %d, want 0", s.Sidebar.Tab)
	}
}

// TestIsMarkdown keeps the set the open dialog, the file pane and the
// drop filter share.
func TestIsMarkdown(t *testing.T) {
	for name, want := range map[string]bool{
		"a.md": true, "a.markdown": true, "a.mdown": true, "a.mkd": true,
		"A.MD": true, "b.txt": false, "md": false, "": false,
	} {
		if got := IsMarkdown(name); got != want {
			t.Errorf("IsMarkdown(%q) = %v, want %v", name, got, want)
		}
	}
}

// TestOpenDropOpensMarkdownOnly checks OpenDrop: markdown paths open,
// the rest are skipped, a failure keeps the rest going.
func TestOpenDropOpensMarkdownOnly(t *testing.T) {
	s, dir := newShell(t)
	img := filepath.Join(dir, "pic.png")
	if err := os.WriteFile(img, []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(dir, "gone.md")
	s.OpenDrop([]string{img, missing, filepath.Join(dir, "b.md")})
	if len(s.Tabs) != 1 || filepath.Base(s.Tabs[0].Path) != "b.md" {
		t.Fatalf("drop opened %v, want only b.md", s.Tabs)
	}
	if s.Active != 0 || s.ActiveTab().Path != filepath.Join(dir, "b.md") {
		t.Fatalf("active = %d %v", s.Active, s.ActiveTab())
	}
}

// writePNG encodes a small valid PNG for the loader to decode.
func writePNG(t *testing.T, path string) {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestContentWiresLoadImage checks the image wiring: a fresh tab has no
// loader, the render asks the loader until the picture is cached, and
// the drawn picture replaces the alt description. A loader the frame
// cleared comes back pointed at the tab's directory.
func TestContentWiresLoadImage(t *testing.T) {
	s, dir := newShell(t)
	writePNG(t, filepath.Join(dir, "pic.png"))
	docPath := filepath.Join(dir, "pic.md")
	if err := os.WriteFile(docPath, []byte("![a pic](pic.png)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.OpenFile(docPath); err != nil {
		t.Fatal(err)
	}
	tab := s.ActiveTab()
	if tab.State.LoadImage != nil {
		t.Fatal("a fresh tab carries a loader already")
	}

	// The loader is the render's, per frame, so a counter must tick.
	calls := 0
	tab.State.LoadImage = func(url string) (ui.ImageSource, error) {
		calls++
		return markdown.FileImages(dir)(url)
	}
	tt := s.renderSize(t, 400, 300)
	tt.Frame()
	if calls == 0 {
		t.Fatal("the render never asked the loader")
	}
	// The load settles in another goroutine, fast enough to land within
	// the first frame; frames until the drawn picture replaces the alt.
	for i := 0; i < 100 && tt.HasText("a pic"); i++ {
		tt.Advance(50 * time.Millisecond)
		tt.Frame()
	}
	if tt.HasText("a pic") {
		t.Fatalf("texts %v: the loaded picture left the description up", tt.Texts())
	}

	// A failing loader keeps the description up: a fresh tab, so the
	// cache is empty, frame after frame.
	if err := s.OpenFile(filepath.Join(dir, "pic.md")); err != nil {
		t.Fatal(err)
	}
	fail := s.ActiveTab()
	fail.State.LoadImage = func(string) (ui.ImageSource, error) { return nil, os.ErrNotExist }
	for range 5 {
		tt.Frame()
	}
	if !tt.HasText("a pic") {
		t.Fatalf("texts %v: a failed load dropped the description", tt.Texts())
	}

	// A nil loader is topped up by the frame, pointed at the tab's dir:
	// the active tab's, so it resolves the sibling picture.
	fail.State.LoadImage = nil
	tt.Frame()
	if fail.State.LoadImage == nil {
		t.Fatal("the frame left LoadImage nil")
	}
	if _, err := fail.State.LoadImage("pic.png"); err != nil {
		t.Fatalf("the frame's loader missed pic.png: %v", err)
	}
}
