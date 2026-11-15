package shell

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/doc"
	"github.com/windedge/peekdown/internal/testshot"
	"github.com/windedge/peekdown/internal/view"
)

// observeShotPath is where the observation screenshots land, "" with the
// env unset: the test skips, the regular runs stay headless-clean.
var observeShotPath = func() string {
	if os.Getenv("PEEKDOWN_SHOT") == "" {
		return ""
	}
	return "C:/tmp/observe-"
}()

// testPNG encodes a solid 600x400 picture, the bitmap the stub loader
// hands out for the linked image's shot.
func testPNG(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{255, 0, 0, 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		panic(err)
	}
	return b.Bytes()
}

// TestObserveShot writes the observation items' screenshots for a human
// to look at: the progress bar and the jump buttons over one
// scrolled-to-the-middle frame, and a linked image drawn with a real
// picture behind the loader. Skipped unless PEEKDOWN_SHOT is set, the
// copy button shot's gating.
func TestObserveShot(t *testing.T) {
	if observeShotPath == "" {
		t.Skip("set PEEKDOWN_SHOT=1 to write the observation screenshots")
	}

	// A long document on disk, opened through the shell's own path so
	// the tab's model, view state and loader wire up as in production.
	dir := t.TempDir()
	path := filepath.Join(dir, "long.md")
	var b []byte
	b = append(b, "# Top\n\n"...)
	for i := range 60 {
		b = append(b, "Paragraph of the filler body, number "...)
		b = append(b, []byte{byte('0' + i/10), byte('0' + i%10)}...)
		b = append(b, ". It is long enough to scroll.\n\n"...)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	s, _ := newShell(t)
	s.OpenPaths([]string{path})
	tt := s.renderSize(t, 900, 600)
	tt.Frame()

	// Scrolled to the middle: the bar's fill stops partway across the
	// body's top edge and both jump buttons stand in the bottom right.
	// Tab.State is a value: the scroll request must land on the tab's own
	// state, not on a copy of it, or the list never scrolls.
	tab := s.ActiveTab()
	tab.State.List.ScrollTo(30, ui.Start)
	tt.Frame()
	tt.Frame()
	writePng(t, tt.Image(), observeShotPath+"progress-mid.png")
	img := tt.Image()
	writePng(t, testshot.Crop(img, img.Bounds().Max.X-280, img.Bounds().Max.Y-160, 280, 160), observeShotPath+"jump-buttons.png")

	// A linked image with a real picture behind the loader: it draws
	// instead of falling back to its alt text.
	tab.Model = doc.New("[![pic](https://e.x/p.png)](https://example.com/page)\n")
	tab.State = view.State{LoadImage: func(url string) (ui.ImageSource, error) {
		return ui.DecodeImage(testPNG(600, 400))
	}}
	tt = s.renderSize(t, 640, 400)
	tt.Frame()
	writePng(t, tt.Image(), observeShotPath+"img-link.png")
}

// writePng saves img for the human eye.
func writePng(t *testing.T, img image.Image, path string) {
	testshot.WritePng(t, img, path)
}
