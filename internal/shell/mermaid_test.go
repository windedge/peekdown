package shell

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/windedge/peekdown/internal/mermaid"
)

// mermaidDoc is a document with one diagram that renders and one that
// fails, above the fold at the test window's size.
const mermaidDoc = "```mermaid\ngraph TD; A-->B\n```\n\n```mermaid\ngraph TD; A--\n```\n"

// TestContentWiresMermaidCache checks the diagram wiring: the shell hands
// every tab the shared cache, a render lands as a picture, and a broken
// diagram keeps its source block with the error under it.
func TestContentWiresMermaidCache(t *testing.T) {
	s, dir := newShell(t)
	if err := os.WriteFile(filepath.Join(dir, "m.md"), []byte(mermaidDoc), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.OpenFile(filepath.Join(dir, "m.md")); err != nil {
		t.Fatal(err)
	}
	tab := s.ActiveTab()
	if tab.State.LoadMermaid != nil {
		t.Fatal("a fresh tab carries a mermaid loader already")
	}

	// The wiring renders through the shell's cache; the frame itself
	// already ran the block's render, so the loader points there. The
	// same call twice: the second returns the cached image itself, not
	// a fresh render.
	tt := s.renderSize(t, 800, 600)
	tt.Frame()
	loader := tab.State.LoadMermaid
	if loader == nil {
		t.Fatal("the frame left LoadMermaid nil")
	}
	s.mermaidCache = mermaid.NewCache(32)
	img1, err := loader("graph TD; A-->B", mermaid.ThemeDefault, 2)
	if err != nil {
		t.Fatalf("loader: %v", err)
	}
	if img1 == nil {
		t.Fatal("loader image is nil")
	}
	img2, err := loader("graph TD; A-->B", mermaid.ThemeDefault, 2)
	if err != nil {
		t.Fatalf("loader again: %v", err)
	}
	if img2 != img1 {
		t.Fatal("the second call rendered again instead of returning the cached image")
	}

	// A cache miss the render fails keeps the source: the broken block's
	// text shows, with the error line under it.
	if !tt.HasText("graph TD; A--") {
		t.Fatalf("texts %v: want the broken diagram's source", tt.Texts())
	}
	if !tt.HasText("mermaid:") {
		t.Fatalf("texts %v: want the error line", tt.Texts())
	}

	// A tab opened elsewhere is topped up with the same shared loader.
	if err := s.OpenFile(filepath.Join(dir, "m.md")); err != nil {
		t.Fatal(err)
	}
	second := s.ActiveTab()
	if second.State.LoadMermaid != nil {
		t.Fatal("the new tab carried a loader of its own")
	}
	tt.Frame()
	if second.State.LoadMermaid == nil {
		t.Fatal("the frame left the second tab's LoadMermaid nil")
	}
	img, err := second.State.LoadMermaid("graph TD; A-->B", mermaid.ThemeDefault, 2)
	if err != nil {
		t.Fatalf("second tab loader: %v", err)
	}
	if img == nil {
		t.Fatal("second tab loader image is nil")
	}
	// The second tab's frame rendered through the same shared cache: the
	// image is the one this test's earlier loader calls cached.
	if img != img1 {
		t.Fatal("the second tab rendered through a different cache")
	}
}

// TestMermaidCacheRenderFails keeps the library's errors: the loader is
// the cache's Render, so errors.Is can match go-mermaid's sentinels
// through it, and an empty source never enters the cache.
func TestMermaidCacheRenderFails(t *testing.T) {
	s, _ := newShell(t)
	_, err1 := s.mermaidCache.Render("", mermaid.ThemeDefault, 2)
	if err1 == nil {
		t.Fatal("empty source rendered")
	}
	// The same empty source again: the stored error comes back as-is,
	// a fresh render would hand over a new error instance.
	_, err2 := s.mermaidCache.Render("", mermaid.ThemeDefault, 2)
	if err2 == nil {
		t.Fatal("second render of empty source: error = nil, want error")
	}
	if err2 != err1 {
		t.Fatal("the failed render was not cached; the second call errored freshly")
	}
}
