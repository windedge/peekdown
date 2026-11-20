package shell

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/windedge/peekdown/internal/testshot"
)

// TestLinkTooltipShot writes a frame with the pointer resting on a link:
// the tip must name the destination the document wrote. Skipped unless
// PEEKDOWN_SHOT is set, as the other picture tests are.
func TestLinkTooltipShot(t *testing.T) {
	if os.Getenv("PEEKDOWN_SHOT") == "" {
		t.Skip("set PEEKDOWN_SHOT=1 to write the link tooltip screenshot")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "note.md")
	const long = "https://example.com/very/long/path/with/a/filename/that/keeps-going/" +
		"xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx/tail.md"
	src := "# Links\n\nRead [the guide](https://example.com/guide) and [the notes](notes.md) here.\n\n" +
		"A [local path](C:/Seafile/Works/rust/peekdown) and [a section](#links).\n\n" +
		"A [long link](" + long + ") here.\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	s, _ := newShell(t)
	s.OpenPaths([]string{path})
	tt := s.renderSize(t, 900, 600)
	tt.Frame()

	const dest = "https://example.com/guide"
	rects := s.ActiveTab().State.LinkRects()
	var x, y float32
	for _, r := range rects {
		if r.Dest == dest {
			x, y = r.Rect.X+r.Rect.W/2, r.Rect.Y+r.Rect.H/2
			break
		}
	}
	if x == 0 {
		t.Fatalf("no rect for %q in %+v", dest, rects)
	}
	tt.Move(x, y)
	tt.Advance(time.Second)
	tt.Frame()
	if _, ok := tt.Find(dest); !ok {
		t.Fatalf("no link tooltip; texts %q", tt.Texts())
	}
	testshot.WritePng(t, tt.Image(), "C:/tmp/link-tooltip.png")
	tip, ok := tt.Find(dest)
	if !ok {
		t.Fatalf("no tooltip box for %q", dest)
	}
	crop := testshot.Crop(tt.Image(),
		int(tip.X)-30, int(tip.Y)-30, int(tip.W)+60, int(tip.H)+60)
	testshot.WritePng(t, testshot.Scale3x(crop), "C:/tmp/link-tooltip-crop.png")

	// The same frame with a long destination: the tip cuts its middle,
	// so it stays readable instead of stretching across the window.
	tt.Move(0, 500)
	for _, r := range s.ActiveTab().State.LinkRects() {
		if r.Dest == long {
			tt.Move(r.Rect.X+r.Rect.W/2, r.Rect.Y+r.Rect.H/2)
			break
		}
	}
	tt.Advance(time.Second)
	tt.Frame()
	testshot.WritePng(t, tt.Image(), "C:/tmp/link-tooltip-long.png")
}
