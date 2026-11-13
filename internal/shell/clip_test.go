package shell

import (
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// The composed page clears the scrollbar on the right at every level: the
// body rows take the clearance as padding, and the frontmatter card, which
// sits above the list rather than in a row, takes the same room as a right
// margin. Without it the card's frame ran to the window's edge, under the
// bar, and the reader saw the card's right side cut.

// rightmostSurfaceX returns the right edge of the last pixel of color c in
// rows y0..y1, or -1 when nothing painted it.
func rightmostSurfaceX(img *image.RGBA, c ui.Color, y0, y1 int) int {
	right := -1
	b := img.Bounds()
	for y := max(y0, b.Min.Y); y < min(y1, b.Max.Y); y++ {
		for x := b.Max.X - 1; x > right; x-- {
			r, g, bl, a := img.RGBAAt(x, y).RGBA()
			if uint8(r>>8) == c.R && uint8(g>>8) == c.G && uint8(bl>>8) == c.B && uint8(a>>8) == c.A {
				if x > right {
					right = x
				}
				break
			}
		}
	}
	return right
}

func TestFrontmatterAndBodyClearTheScrollbar(t *testing.T) {
	s, dir := newShell(t)
	path := filepath.Join(dir, "clip.md")
	src := "---\ntitle: Clip probe\n---\n\n" + strings.Repeat("word ", 200) + "\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	saved := s.Saved
	saved.Layout = "full_width"
	if err := s.Apply(saved); err != nil {
		t.Fatal(err)
	}

	const width = 1400
	tt := s.renderSize(t, width, 600)
	tt.Frame()
	// The window is DIPs in the tester, so the clearance is the same 16.
	clearance := 16 // view.rowClearance, the rows' scrollbar clearance

	// The card is the only surface on the page, and it sits above the
	// body: its frame ends where the rows' text does. The scan starts
	// under the title bar, whose tray is the same surface color now.
	card := rightmostSurfaceX(tt.Image(), ui.LightTheme().Surface, TitleBarHeight, 400)
	if card < 0 {
		t.Skip("the card did not paint")
	}
	if got := width - 1 - card; got < clearance {
		t.Fatalf("the frontmatter card ends %d DIPs from the window's edge, want %d", got, clearance)
	}

	// The body's line box clears it too.
	r, ok := paragraphRect(tt, "word")
	if !ok {
		t.Fatalf("texts %v: want the paragraph", tt.Texts())
	}
	if got := width - (r.X + r.W); got < float32(clearance) {
		t.Fatalf("the body ends %v DIPs from the window's edge, want %d (rect %v)", got, clearance, r)
	}
}

// TestCodeBlockCopyWritesClipboard clicks a code block's copy button: the
// shell wires the view's CopyText to the clipboard and a toast.
func TestCodeBlockCopyWritesClipboard(t *testing.T) {
	s, dir := newShell(t)
	path := filepath.Join(dir, "code.md")
	if err := os.WriteFile(path, []byte("```go\nx := 1\ny := 22\n```\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	tt := s.render(t)
	tt.Frame()
	if err := tt.Click("Copy"); err != nil {
		t.Fatal(err)
	}
	if got, want := tt.Clipboard(), "x := 1\ny := 22"; got != want {
		t.Fatalf("clipboard %q, want %q", got, want)
	}
	if !tt.HasText("Copied code") {
		t.Fatalf("texts %v: no toast after the copy", tt.Texts())
	}
}
