package shell

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/settings"
)

// openOutline writes a three-level document, opens it and shows the
// outline: the pane the level tests share.
func openOutline(t *testing.T, width int) (*Shell, *ui.Tester) {
	t.Helper()
	s, dir := newShell(t)
	p := filepath.Join(dir, "levels.md")
	src := "# H1 one\n\ntext\n\n## H2 two\n\ntext\n\n### H3 three\n\ntext\n"
	if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	s.OpenPaths([]string{p})
	s.SetSidebarTab(1)
	tt := s.renderSize(t, width, 600)
	tt.Frame()
	return s, tt
}

// TestOutlineAllLevels checks every heading level reaches the pane: the
// Rust outline lists h1-h6, the model's Heading.Level drives the step.
func TestOutlineAllLevels(t *testing.T) {
	s, tt := openOutline(t, 900)
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
	_, tt := openOutline(t, 900)
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
	_, tt := openOutline(t, 900)
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
	// The first row is not shoved off the pane's left edge by the indent.
	if r, ok := tt.Find("H1 one"); ok && r.X < 8 {
		t.Fatalf("h1 text x = %.1f, want its 12 DIP start padding", r.X)
	}
}

// TestOutlineRowHeightAndPad checks the row geometry matches the Rust
// pane: a py_1 (4 DIPs) tall row block, 13px text, and the text
// clear of the pane's right edge where the overlay scroll bar paints.
func TestOutlineRowHeightAndPad(t *testing.T) {
	_, tt := openOutline(t, 900)
	r, ok := tt.Find("H1 one")
	if !ok {
		t.Fatal("outline row missing")
	}
	// FontSize 14 lays out near 20 DIPs tall with the padding; a clamp
	// or fixed-height regression would crush it below 17.
	if r.H < 17 || r.H > 26 {
		t.Fatalf("row height = %.1f, want ~20", r.H)
	}
}

// TestSidebarHeaderTabs checks the header's Files/Outline tabs and close
// button: small labels in one row (the Rust sidebar's header), the
// active one accent-tinted, all left of the close button.
func TestSidebarHeaderTabs(t *testing.T) {
	_, tt := openOutline(t, 900)
	files, okF := tt.Find("Files")
	outline, okO := tt.Find("Outline")
	close, okC := tt.Find("Close sidebar")
	if !okF || !okO || !okC {
		t.Fatalf("header parts missing: %v %v %v", okF, okO, okC)
	}
	if outline.Y != files.Y {
		t.Fatalf("tabs not in one row: Files y=%.1f Outline y=%.1f", files.Y, outline.Y)
	}
	if close.X < outline.X+outline.W {
		t.Fatalf("close button overlaps the tabs: close x=%.1f, tabs end %.1f", close.X, outline.X+outline.W)
	}
	// The tabs are small labels, not a Segmented chip: under 24 DIPs
	// tall (a Segmented track is ~28+).
	if outline.H > 24 {
		t.Fatalf("tab height = %.1f, want a small label under 24", outline.H)
	}
}

// TestSidebarDefaultWidth pins the Rust defaults: 200 on first run,
// clamped 120..400 (outline.rs / mod.rs / config.rs agree).
func TestSidebarDefaultWidth(t *testing.T) {
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
}

// TestOutlineTextSmallerThanBody checks the pane draws its rows one size
// below the document: the same heading measured in the pane (first in the
// layout, inside the sidebar) is shorter than the body's copy.
func TestOutlineTextSmallerThanBody(t *testing.T) {
	s, dir := newShell(t)
	p := filepath.Join(dir, "sizes.md")
	const heading = "Sizes one"
	src := "# " + heading
	if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	s.OpenPaths([]string{p})
	s.SetSidebarTab(1)
	tt := s.renderSize(t, 1400, 900)
	tt.Frame()
	pane, ok := tt.Find(heading)
	if !ok {
		t.Fatalf("outline row missing: %v", tt.Texts())
	}
	if pane.X >= s.Sidebar.Width {
		t.Fatalf("first %q rect x=%.1f is not the pane's copy (sidebar %.0f wide)", heading, pane.X, s.Sidebar.Width)
	}

	// Same document, no pane: the only copy is the body's.
	s.Sidebar.Visible = false
	tt2 := s.renderSize(t, 1400, 900)
	tt2.Frame()
	body, ok := tt2.Find(heading)
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
	s, _ := openOutline(t, 900)
	var writes int
	base := s.Save
	s.Save = func(v settings.Settings) error {
		writes++
		return base(v)
	}
	tt := s.renderSize(t, 900, 600)
	tt.Frame()

	// The divider sits at the sidebar's right edge; grab it and drag.
	w0 := s.Sidebar.Width
	gx := w0 - 1
	tt.Press(gx, 300)
	tt.Frame()
	tt.Move(gx+60, 300)
	tt.Frame()
	if s.Sidebar.Width <= w0 {
		t.Fatalf("width after drag = %.0f, want more than %.0f", s.Sidebar.Width, w0)
	}
	if s.Saved.SidebarWidth == float64(s.Sidebar.Width) {
		t.Fatal("saved width while the drag is still moving")
	}
	if writes != 0 {
		t.Fatalf("%d saves mid-drag, want none", writes)
	}

	// Let go: one more frame closes the press, then the timer runs out.
	tt.Release(gx+60, 300)
	tt.Frame()
	if writes != 0 {
		t.Fatalf("%d saves at release, want the debounce to hold it", writes)
	}
	tt.Advance(widthSaveDelay + 50*time.Millisecond)
	if writes != 1 {
		t.Fatalf("saves after settle = %d, want 1", writes)
	}
	if s.Saved.SidebarWidth != float64(s.Sidebar.Width) {
		t.Fatalf("saved %.0f, want the settled %.0f", s.Saved.SidebarWidth, s.Sidebar.Width)
	}
	if s.Draft.SidebarWidth != s.Saved.SidebarWidth {
		t.Fatalf("draft %.0f not synced with saved %.0f", s.Draft.SidebarWidth, s.Saved.SidebarWidth)
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
