package shell

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/settings"
)

// openOutline writes a three-level document, opens it and shows both
// panels: the right-hand outline and the file tree on the left. Emphasis
// in the headings keeps the outline's plain-text label distinct from the
// body's split spans, so Find measures the pane's copy.
func openOutline(t *testing.T, width int) (*Shell, *ui.Tester) {
	t.Helper()
	s, dir := newShell(t)
	p := filepath.Join(dir, "levels.md")
	src := "# H1 `one`\n\ntext\n\n## H2 `two`\n\ntext\n\n### H3 `three`\n\ntext\n"
	if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	s.OpenPaths([]string{p})
	s.SetFilesVisible(true)
	s.SetOutlineVisible(true)
	tt := s.renderSize(t, width, 600)
	tt.Frame()
	tt.Frame()
	// The nested split needs a committed body width before its pane
	// builds; a third frame with the content hook set forces that.
	s.ContentHook = func(*ui.Context) {}
	tt.Frame()
	return s, tt
}

// TestOutlineAllLevels checks every heading level reaches the pane: the
// Rust outline lists h1-h6, the model's Heading.Level drives the step.
func TestOutlineAllLevels(t *testing.T) {
	s, tt := openOutline(t, 1200)
	for _, text := range []string{"H1 one", "H2 two", "H3 three"} {
		if !tt.HasText(text) {
			t.Fatalf("outline missing %q: %v", text, tt.Texts())
		}
	}
	if got := len(s.ActiveTab().Model.Headings()); got != 3 {
		t.Fatalf("model headings = %d, want 3", got)
	}
}

// TestOutlineIndentPerLevel measures the rows: level n's text starts
// 12 DIPs right of level n-1's (Rust outline.rs's pl = 12 + (level-1)*12
// against the sidebar edge; the rows' equal padding cancels in the
// difference).
func TestOutlineIndentPerLevel(t *testing.T) {
	_, tt := openOutline(t, 1200)
	texts := []string{"H1 one", "H2 two", "H3 three"}
	x := make([]float32, len(texts))
	for i, text := range texts {
		r, ok := tt.Find(text)
		if !ok {
			t.Fatalf("outline row %q missing", text)
		}
		x[i] = r.X
	}
	for i := 1; i < len(x); i++ {
		if d := x[i] - x[i-1]; d < 11 || d > 13 {
			t.Fatalf("indent step %q -> %q = %.1f, want 12", texts[i-1], texts[i], d)
		}
	}
	if x[0]-x[1] == 0 {
		t.Fatal("levels did not indent at all")
	}
}

// TestOutlineRowsStack checks the rows sit one under another: a row that
// Fill()s the pane's height (instead of FillWidth()) pushes every later
// heading out of view and vertically centers its own text.
func TestOutlineRowsStack(t *testing.T) {
	s, tt := openOutline(t, 1200)
	texts := []string{"H1 one", "H2 two", "H3 three"}
	y := make([]float32, len(texts))
	for i, text := range texts {
		r, ok := tt.Find(text)
		if !ok {
			t.Fatalf("outline row %q missing", text)
		}
		y[i] = r.Y
	}
	for i := 1; i < len(y); i++ {
		if d := y[i] - y[i-1]; d < 17 || d > 34 {
			t.Fatalf("row step %q -> %q = %.1f, want one row height", texts[i-1], texts[i], d)
		}
	}
	// The first row keeps its 12 DIP start inset inside the right panel.
	if r, ok := tt.Find("H1 one"); ok {
		left := 1200 - s.Outline.Width
		if r.X < left || r.X > left+40 {
			t.Fatalf("h1 text x = %.1f, want its inset in the panel at %.0f", r.X, left)
		}
	}
}

// TestOutlineRowHeightAndPad checks the row geometry matches the Rust
// pane: a py_1 (4 DIPs) tall row block, 13px text, and the text
// clear of the pane's right edge where the overlay scroll bar paints.
func TestOutlineRowHeightAndPad(t *testing.T) {
	_, tt := openOutline(t, 1200)
	r, ok := tt.Find("H1 one")
	if !ok {
		t.Fatal("outline row missing")
	}
	// The row's text box follows the platform's 13px line height: ~20
	// DIPs with Segoe, ~15 with the macOS system font. Measure that line
	// instead of pinning one font's number; a clamp would shrink it.
	_, lineH := ui.ShapeText("H1 one", ui.Font{Size: 13}, 10000).Size()
	if r.H < lineH-1 || r.H > lineH+1 {
		t.Fatalf("row text height = %.1f, want the 13px line box %.1f", r.H, lineH)
	}
}

// TestPanelHeaders checks that both panels use the same single title
// and close button: the file pane no longer carries an Outline tab.
func TestPanelHeaders(t *testing.T) {
	s, tt := openOutline(t, 1200)
	files, okF := tt.Find("Files")
	outline, okO := tt.Find("Outline")
	close, okC := tt.Find("Close sidebar Outline")
	if !okF || !okO || !okC {
		t.Fatalf("header parts missing: %v %v %v", okF, okO, okC)
	}
	// Both panes sit in the same 8 DIP gutter under the bar, so their
	// headers line up exactly.
	if d := outline.Y - files.Y; d < -1 || d > 1 {
		t.Fatalf("headers not aligned: Files y=%.1f Outline y=%.1f", files.Y, outline.Y)
	}
	if files.X < 0 || files.X > s.Sidebar.Width {
		t.Fatalf("Files title x=%.1f outside the left pane", files.X)
	}
	if outline.X < 1200-s.Outline.Width-24 {
		t.Fatalf("Outline title x=%.1f is not in the right panel", outline.X)
	}
	if close.X < outline.X+outline.W {
		t.Fatalf("close button overlaps the title: close x=%.1f, title end %.1f", close.X, outline.X+outline.W)
	}
	if close.X+20 > 1200-8 {
		t.Fatalf("close button at x=%.1f runs into the window edge", close.X)
	}
}

// TestPanelDefaultWidths pins both defaults and both drag ranges.
func TestPanelDefaultWidths(t *testing.T) {
	s, _ := newShell(t)
	if s.Sidebar.Width != 200 {
		t.Fatalf("default width = %v, want 200", s.Sidebar.Width)
	}
	s.SetSidebarWidth(10)
	if s.Sidebar.Width != 120 {
		t.Fatalf("min width = %v, want 120", s.Sidebar.Width)
	}
	s.SetSidebarWidth(1000)
	if s.Sidebar.Width != 400 {
		t.Fatalf("max width = %v, want 400", s.Sidebar.Width)
	}
	if s.Outline.Width != 232 {
		t.Fatalf("outline default = %v, want 232", s.Outline.Width)
	}
	s.SetOutlineWidth(10)
	if s.Outline.Width != 160 {
		t.Fatalf("outline min = %v, want 160", s.Outline.Width)
	}
	s.SetOutlineWidth(1000)
	if s.Outline.Width != 360 {
		t.Fatalf("outline max = %v, want 360", s.Outline.Width)
	}
}

// TestOutlineTextSmallerThanBody checks the pane draws its rows one size
// below the document. The pane's copy is emphasized in the source, so
// the body paints split spans and Find sees the outline's label alone.
func TestOutlineTextSmallerThanBody(t *testing.T) {
	s, dir := newShell(t)
	p := filepath.Join(dir, "sizes.md")
	if err := os.WriteFile(p, []byte("# Sizes **one**"), 0o644); err != nil {
		t.Fatal(err)
	}
	s.OpenPaths([]string{p})
	tt := s.renderSize(t, 1400, 900)
	tt.Frame()
	pane, ok := tt.Find("Sizes one")
	if !ok {
		t.Fatalf("outline row missing: %v", tt.Texts())
	}
	if pane.X < 1400-s.Outline.Width-24 {
		t.Fatalf("heading at x=%.1f is not the right panel's copy", pane.X)
	}

	// Plain text in the body, with the right panel closed.
	s.Outline.Visible = false
	if err := os.WriteFile(p, []byte("# Sizes one"), 0o644); err != nil {
		t.Fatal(err)
	}
	s.RefreshActiveTab()
	tt2 := s.renderSize(t, 1400, 900)
	tt2.Frame()
	body, ok := tt2.Find("Sizes one")
	if !ok {
		t.Fatalf("body heading missing: %v", tt2.Texts())
	}
	if pane.H <= 0 || body.H <= 0 {
		t.Fatalf("heading heights: pane %.1f body %.1f", pane.H, body.H)
	}
	if pane.H >= body.H {
		t.Fatalf("outline text height %.1f not smaller than the body's %.1f", pane.H, body.H)
	}
}

// TestSidebarDragPersistsWidth drags the split's real divider and checks
// the width settles into the settings only after the drag stops moving,
// clamped, and the open settings draft rides along.
func TestSidebarDragPersistsWidth(t *testing.T) {
	s, _ := openOutline(t, 1200)
	var writes int
	base := s.Save
	s.Save = func(v settings.Settings) error {
		writes++
		return base(v)
	}
	tt := s.renderSize(t, 1200, 600)
	tt.Frame()

	// The outline divider sits at the window's right edge: dragging it
	// left widens the panel, right narrows it.
	w0 := s.Outline.Width
	gx := 1200 - w0
	tt.Press(gx, 300)
	tt.Frame()
	tt.Move(gx-60, 300)
	tt.Frame()
	if s.Outline.Width <= w0 {
		t.Fatalf("width after left drag = %.0f, want more than %.0f", s.Outline.Width, w0)
	}
	if s.Saved.OutlineWidth == float64(s.Outline.Width) {
		t.Fatal("saved width while the drag is still moving")
	}
	if writes != 0 {
		t.Fatalf("%d saves mid-drag, want none", writes)
	}

	// Let go: one more frame closes the press, then the timer runs out.
	tt.Release(gx-60, 300)
	tt.Frame()
	if writes != 0 {
		t.Fatalf("%d saves at release, want the debounce to hold it", writes)
	}
	tt.Advance(widthSaveDelay + 50*time.Millisecond)
	if writes != 1 {
		t.Fatalf("saves after settle = %d, want 1", writes)
	}
	if s.Saved.OutlineWidth != float64(s.Outline.Width) {
		t.Fatalf("saved %.0f, want the settled %.0f", s.Saved.OutlineWidth, s.Outline.Width)
	}
	if s.Saved.SidebarWidth != float64(s.Sidebar.Width) {
		t.Fatalf("file pane width %.0f was not saved with it", s.Saved.SidebarWidth)
	}

	// A right drag narrows the panel again, through the same debounce.
	w1 := s.Outline.Width
	gx = 1200 - w1
	tt.Press(gx, 300)
	tt.Frame()
	tt.Move(gx+40, 300)
	tt.Frame()
	if s.Outline.Width >= w1 {
		t.Fatalf("width after right drag = %.0f, want less than %.0f", s.Outline.Width, w1)
	}
	tt.Release(gx+40, 300)
	tt.Frame()
	tt.Advance(widthSaveDelay + 50*time.Millisecond)
	if writes != 2 {
		t.Fatalf("saves after narrowing = %d, want 2", writes)
	}
	if s.Saved.OutlineWidth != float64(s.Outline.Width) {
		t.Fatalf("saved %.0f, want the narrowed %.0f", s.Saved.OutlineWidth, s.Outline.Width)
	}
}

// TestRestoreClampsSidebarWidth checks the saved width lands through
// SetSidebarWidth, so an out-of-range file is clamped back into 120-400.
func TestRestoreClampsSidebarWidth(t *testing.T) {
	s := New()
	var saved settings.Settings
	saved.SidebarWidth = 5000
	saved.SidebarVisible = true
	s.RestoreFromSettings(saved)
	if s.Sidebar.Width != 400 {
		t.Fatalf("width = %v, want the max 400", s.Sidebar.Width)
	}
	if s.Saved.SidebarWidth != 5000 {
		t.Fatalf("saved width = %v, restore must not rewrite the file", s.Saved.SidebarWidth)
	}

	s2 := New()
	var low settings.Settings
	low.SidebarWidth = 10
	low.SidebarVisible = true
	s2.RestoreFromSettings(low)
	if s2.Sidebar.Width != 120 {
		t.Fatalf("width = %v, want the min 120", s2.Sidebar.Width)
	}
}
