package view

import (
	"os"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/doc"
)

// TestListFontShot renders the top of this repo's AGENTS.md, its nested
// bullets over paragraphs, for a human to judge the list's font size
// against the prose. Skipped unless PEEKDOWN_SHOT is set.
func TestListFontShot(t *testing.T) {
	if os.Getenv("PEEKDOWN_SHOT") == "" {
		t.Skip("set PEEKDOWN_SHOT=1 to write the list font screenshot")
	}
	src, err := os.ReadFile("../../AGENTS.md")
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	out := os.Getenv("PEEKDOWN_SHOT_OUT")
	if out == "" {
		out = "C:/tmp/list-font-after.png"
	}
	m := doc.New(string(src))
	st := &State{AnchorOf: anchorsOf(m), ContentWidth: 1200}
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		ScrollView(c, m, st)
	}, 1440, 900)
	tt.Frame()
	writeShot(t, tt.Image(), out)
}
