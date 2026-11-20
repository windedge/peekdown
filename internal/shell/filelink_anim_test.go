package shell

import (
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/settings"
)

// Regression for the reported dead links: with animations on, the pane
// slide runs on the first frames; the split must not stay disabled past
// it, or the document's clicks land on nothing. This test runs the same
// render path without DisableAnimations, so the slide actually moves.
func TestLinkClickSurvivesPaneSlideAnimation(t *testing.T) {
	dir := linkFiles(t)
	s, _ := newShell(t)
	tt := ui.NewTester(func(c *ui.Context) {
		s.View(c)
	}, 800, 600)
	if err := s.OpenFile(filepath.Join(dir, "a.md")); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	clickLink(t, tt, "bee,", float32(settings.Default().FontSize))
	if tab := s.ActiveTab(); tab == nil || tab.Path != filepath.Join(dir, "b.md") {
		t.Fatalf("active %v, want b.md (link click swallowed)", tab)
	}
	if len(s.Tabs) != 2 {
		t.Fatalf("tabs = %d, want 2", len(s.Tabs))
	}
}
