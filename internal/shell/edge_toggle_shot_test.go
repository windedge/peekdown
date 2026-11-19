package shell

import (
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

// edgeToggleShotPath gates the headless screenshots on PEEKDOWN_SHOT.
var edgeToggleShotPath = func() string {
	if os.Getenv("PEEKDOWN_SHOT") == "" {
		return ""
	}
	return "C:/tmp/edge-toggle-"
}()

func edgeToggleShotShell(t *testing.T, width, height int, files, outline bool) (*Shell, *ui.Tester) {
	t.Helper()
	s, _ := newShell(t)
	if err := s.OpenFile(cardShotDoc(t)); err != nil {
		t.Fatal(err)
	}
	s.Sidebar.Visible = files
	s.Outline.Visible = outline
	return s, s.renderSize(t, width, height)
}

func TestEdgeToggleShot(t *testing.T) {
	if edgeToggleShotPath == "" {
		t.Skip("set PEEKDOWN_SHOT=1 to write the edge toggle screenshots")
	}
	_, open := edgeToggleShotShell(t, 1440, 900, true, true)
	open.Frame()
	open.Frame()
	writePng(t, open.Image(), edgeToggleShotPath+"both-open.png")

	_, closed := edgeToggleShotShell(t, 1440, 900, false, false)
	closed.Frame()
	closed.Frame()
	writePng(t, closed.Image(), edgeToggleShotPath+"closed.png")

	_, hover := edgeToggleShotShell(t, 1440, 900, true, true)
	hover.Frame()
	hover.Frame()
	b, ok := hover.Find("Files panel")
	if !ok {
		t.Fatal("files edge button missing")
	}
	hover.Move(b.X+b.W/2, b.Y+b.H/2)
	hover.Frame()
	hover.Frame()
	writePng(t, hover.Image(), edgeToggleShotPath+"hover.png")

	_, narrow := edgeToggleShotShell(t, 760, 700, true, true)
	narrow.Frame()
	narrow.Frame()
	writePng(t, narrow.Image(), edgeToggleShotPath+"narrow.png")
}
