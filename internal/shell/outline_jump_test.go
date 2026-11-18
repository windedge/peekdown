package shell

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/testshot"
)

// jumpShotPath is where the outline jump's screenshot lands, "" with the
// env unset: the shot is extra evidence, the assertions run headless
// either way.
var jumpShotPath = func() string {
	if os.Getenv("PEEKDOWN_SHOT") == "" {
		return ""
	}
	return "C:/tmp/outline-jump-top.png"
}()

// openOutlineJumpPage opens a document whose fourth section heading sits
// several viewports down, with both panes shown, and returns the block
// the outline row names.
func openOutlineJumpPage(t *testing.T) (*Shell, *ui.Tester, int) {
	t.Helper()
	s, dir := newShell(t)
	p := filepath.Join(dir, "sections.md")
	var b []byte
	b = append(b, "# Top\n\n"...)
	for i := range 400 {
		if i%100 == 0 {
			b = append(b, "## Section "...)
			b = append(b, strconv.Itoa(i/100)...)
			b = append(b, "\n\n"...)
		}
		b = append(b, "Paragraph number "...)
		b = append(b, strconv.Itoa(i)...)
		b = append(b, " of the filler body.\n\n"...)
	}
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
	s.OpenPaths([]string{p})
	s.SetFilesVisible(true)
	s.SetOutlineVisible(true)
	tt := s.renderSize(t, 1400, 700)
	tt.Frame()
	tt.Frame()
	// The nested split needs a committed body width before its pane
	// builds; a third frame with the content hook set forces that.
	s.ContentHook = func(*ui.Context) {}
	tt.Frame()
	target := -1
	for _, h := range s.ActiveTab().Model.Headings() {
		if h.Text == "Section 3" {
			target = h.BlockIndex
		}
	}
	if target < 0 {
		t.Fatalf("headings %v: the sections did not parse", s.ActiveTab().Model.Headings())
	}
	return s, tt, target
}

// outlineJumpTop clicks the outline row and returns how far the heading's
// row starts below the content area's top.
func outlineJumpTop(t *testing.T, s *Shell, tt *ui.Tester, target int) (area ui.Rect, top float32) {
	t.Helper()
	if err := tt.Click("Section 3"); err != nil {
		t.Fatalf("clicking the outline row: %v", err)
	}
	tt.Frame()
	tt.Frame()
	if got := s.ActiveTab().State.ScrollTargetBlock; got != target {
		t.Fatalf("ScrollTargetBlock = %d, want the heading's block %d", got, target)
	}
	area, ok := tt.Find("Content area")
	if !ok {
		t.Fatalf("no content area in %q", tt.Texts())
	}
	r, ok := s.ActiveTab().State.RowRect(target)
	if !ok {
		t.Fatalf("the heading's block %d has no committed rect", target)
	}
	return area, r.Y - area.Y
}

// TestOutlineClickLandsAtTop checks the outline jump's landing: the
// clicked section's heading comes to the top of the view, the way a
// link to it should, instead of being revealed at the bottom edge.
func TestOutlineClickLandsAtTop(t *testing.T) {
	s, tt, target := openOutlineJumpPage(t)
	area, top := outlineJumpTop(t, s, tt, target)
	if top < -8 || top > 40 {
		t.Fatalf("the heading sits %v DIPs below the content area's top %v, want it at the top", top, area.Y)
	}
	// Half the viewport is the tell: a least-scroll reveal parks the
	// heading at the bottom, a top-aligned jump brings its body in below.
	if top > area.H/2 {
		t.Fatalf("the heading sits %v DIPs down a %v DIP tall area: the jump revealed it at the bottom", top, area.H)
	}
}

// TestOutlineJumpTopShot captures the landing, so the heading's place in
// the viewport can be inspected.
func TestOutlineJumpTopShot(t *testing.T) {
	if jumpShotPath == "" {
		t.Skip("set PEEKDOWN_SHOT=1 to write the outline jump screenshot")
	}
	s, tt, target := openOutlineJumpPage(t)
	outlineJumpTop(t, s, tt, target)
	testshot.WritePng(t, tt.Image(), jumpShotPath)
}
