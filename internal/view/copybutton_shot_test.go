package view

import (
	"os"
	"testing"
	"time"

	"github.com/windedge/peekdown/internal/testshot"
)

// TestCopyButtonShot writes the code block's copy button screenshots for
// a human to look at; skipped unless PEEKDOWN_SHOT is set, so the regular
// test runs stay headless-clean. The hover pass rests the pointer on the
// button past mygo's tooltip delay, so the icon-only button and its
// "Copy code" tooltip land in the same frame.
func TestCopyButtonShot(t *testing.T) {
	if os.Getenv("PEEKDOWN_SHOT") == "" {
		t.Skip("set PEEKDOWN_SHOT=1 to write the copy button screenshots")
	}
	src := "Intro paragraph.\n\n```go\nfunc main() {\n\tfmt.Println(\"hello peekdown\")\n}\n```\n"
	_, tt, st := frame(t, src, 640, 400)
	st.CopyText = func(string) {}
	tt.Frame()
	r, ok := tt.Find("Copy")
	if !ok {
		t.Fatalf("texts %v: no copy button", tt.Texts())
	}
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	tt.Advance(650 * time.Millisecond)
	tt.Frame()
	testshot.WritePng(t, tt.Image(), "C:/tmp/copyblock-full.png")

	// Crop the block's top-right corner around the button and blow it up
	// 3x nearest-neighbor, so the icon's shape is easy to judge.
	cx, cy := int(r.X+r.W/2), int(r.Y+r.H/2)
	src3x := testshot.Scale3x(testshot.Crop(tt.Image(), cx-70, cy-70, 140, 140))
	testshot.WritePng(t, src3x, "C:/tmp/copy-icon-only.png")
}
