package view

import (
	"os"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/doc"
)

// TestTableRuleShot writes the acceptance matrix's five column table, the
// one whose column rules broke into short segments, for the human eye.
// Skipped unless PEEKDOWN_SHOT is set.
func TestTableRuleShot(t *testing.T) {
	if os.Getenv("PEEKDOWN_SHOT") == "" {
		t.Skip("set PEEKDOWN_SHOT=1 to write the table rule screenshot")
	}
	src, err := os.ReadFile("../../docs/v0.3.0-acceptance.md")
	if err != nil {
		t.Fatalf("read the acceptance matrix: %v", err)
	}
	out := os.Getenv("PEEKDOWN_SHOT_OUT")
	if out == "" {
		out = "C:/tmp/table-borders-after.png"
	}
	m := doc.New(string(src))
	st := &State{AnchorOf: anchorsOf(m), ContentWidth: 1200}
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		ScrollView(c, m, st)
	}, 1500, 900)
	tt.Frame()
	writeShot(t, tt.Image(), out)
}
