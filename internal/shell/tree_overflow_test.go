package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/settings"
)

// deepTreeFixture writes a directory with count subdirectories, the
// last ones carrying names long enough to fill a 200 DIP sidebar at
// depth 2: the shape the overflow reports came from.
func deepTreeFixture(t *testing.T, dir string, count int) string {
	t.Helper()
	proj := filepath.Join(dir, "proj")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proj, "go.mod"), []byte("module proj\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proj, "root.md"), []byte("# Root\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for i := range count {
		name := fmt.Sprintf("directory-with-a-really-long-name-number-%03d-for-the-tree", i)
		if i >= count-4 {
			// The last four stack a suffix too, so a couple sit past the
			// default sidebar's 200 DIPs at depth 2.
			name += "-and-its-name-keeps-going-overflow-candidate"
		}
		sub := filepath.Join(proj, name, "nested")
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(sub, "deep.md"), []byte("# Deep\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	expanded := make([]string, 0, count)
	for i := range count {
		expanded = append(expanded, filepath.Join(proj, fmt.Sprintf("directory-with-a-really-long-name-number-%03d-for-the-tree", i)))
	}
	saved := settings.Default()
	saved.AutoRefresh = false
	saved.ExplorerRootMode = "project_root"
	saved.ProjectRootMarkers = []string{"go.mod"}
	saved.ExplorerSortMode = "name_asc"
	saved.ExpandedDirs = expanded
	return proj
}

// openDeepTree opens the deep fixture's root.md with the sidebar showing.
func openDeepTree(t *testing.T, s *Shell, proj string, w, h int) *ui.Tester {
	t.Helper()
	if err := s.OpenFile(filepath.Join(proj, "root.md")); err != nil {
		t.Fatal(err)
	}
	s.Sidebar.Visible = true
	s.SetFilesVisible(true)
	tt := s.renderSize(t, w, h)
	tt.Frame()
	return tt
}

// TestFileTreeStaysInsideCard is the overflow regression: a deep tree
// with long names must keep its rows inside the files card. The row
// text's right edge stops before the card's right edge (the overlay
// scrollbar's 12 DIP clearance), the card ends above the status bar,
// and the tree's List clips inside the card's rounded corners.
func TestFileTreeStaysInsideCard(t *testing.T) {
	s, dir := newShell(t)
	proj := deepTreeFixture(t, dir, 30)
	if len(s.tree.rows) != 0 {
		t.Fatal("a fresh shell must not carry rows")
	}
	tt := openDeepTree(t, s, proj, 1440, 880)

	if len(s.tree.rows) < 30 {
		t.Fatalf("rows = %d, want the deep tree's 30+", len(s.tree.rows))
	}
	card, ok := tt.Find("Files card")
	if !ok {
		t.Fatalf("files card missing: %v", tt.Texts())
	}

	// Every committed row's text stays inside the card: the name block's
	// Find box ends at the row's PaddingEnd, at least the scrollbar
	// clearance short of the card's right edge. Only the visible rows
	// carry a label; a long tree scrolls, so unseen rows have none.
	lastEnd := float32(0)
	for _, r := range s.tree.rows {
		rn, ok := tt.Find(r.Rel)
		if !ok {
			continue
		}
		if rn.X < card.X || rn.X+rn.W > card.X+card.W {
			t.Fatalf("row %q spans x %.1f..%.1f, outside the card %.1f..%.1f",
				r.Rel, rn.X, rn.X+rn.W, card.X, card.X+card.W)
		}
		lastEnd = max(lastEnd, rn.Y+rn.H)
	}
	if lastEnd == 0 {
		t.Fatal("no tree row was committed; the pane drew nothing")
	} // The virtual list commits one straddling row past its floor; its
	// paint is clipped by the list, so the geometry may overhang but no
	// pixel may: sample the gutter between the card and the status bar
	// for row fill (near-white) or glyphs leaking out of the card.
	gx, gy := card.X+card.W/2, card.Y+card.H+4
	if sum := channelSum(pixelAt(tt.Image(), gx, gy), ui.Color{}); sum > 748 {
		t.Fatalf("pixel below the card at %.0f,%.0f too bright (%v): a row paints past the card", gx, gy, pixelAt(tt.Image(), gx, gy))
	}
	// The card holds inside the gutters: above the status bar, below the
	// title bar, no pane overruns the frame's pane bands.
	const statusBarTop = float32(880 - 28)
	if card.Y < TitleBarHeight || card.Y+card.H > statusBarTop {
		t.Fatalf("card spans y %.0f..%.0f, want inside the pane band %d..%.0f",
			card.Y, card.Y+card.H, TitleBarHeight, statusBarTop)
	}
}

// TestFileTreeCardClipsOverflow drives the clip specifically: with the
// List's committed geometry under a clip the rows' boxes stop at the
// card, and a pixel sampled past the card's right edge stays gradient,
// not card or row fill. This pins regression in the direction the
// overflow report came from, a row escaping the card sideways.
func TestFileTreeCardClipsOverflow(t *testing.T) {
	s, dir := newShell(t)
	proj := deepTreeFixture(t, dir, 30)
	tt := openDeepTree(t, s, proj, 1440, 880)
	card, ok := tt.Find("Files card")
	if !ok {
		t.Fatalf("files card missing: %v", tt.Texts())
	}
	// Right at the card's edge plus a hair: past Clip() this is card
	// fill or a row's white; with it, the 8 DIP gutter's gradient.
	k := accentThemeByID(settings.AccentThemeDefault).Light
	x, y := card.X+card.W+3, card.Y+card.H/2
	if got, want := pixelAt(tt.Image(), x, y), over(k.BgTop, k.BgTop); channelClose(got, want, 2) {
		t.Fatalf("pixel just past the card edge %v reads card fill; the gutter must show the gradient", got)
	}
}

// TestEdgeButtonsAreToggles asserts both edge buttons flip their pane:
// Files must close on the second click (it only opened before). The idle
// button has no surface of its own, so its pixel matches the card's.
func TestEdgeButtonsAreToggles(t *testing.T) {
	s, dir := newShell(t)
	if err := s.OpenFile(filepath.Join(dir, "a.md")); err != nil {
		t.Fatal(err)
	}
	s.Sidebar.Visible = true
	tt := s.renderSize(t, 1200, 600)
	tt.Frame()
	tt.Frame()

	btn, ok := tt.Find("Files panel")
	if !ok {
		t.Fatalf("files edge button missing: %v", tt.Texts())
	}
	// The probe takes the button's inner top-left corner: above the
	// centered icon and left of the heading's first glyph, which reaches
	// under this corner button when the document opens with no
	// frontmatter to push the H1 down. An idle transparent button shows
	// the card's own face there.
	x, y := btn.X+3, btn.Y+3
	card, ok := tt.Find("Document card")
	if !ok {
		t.Fatal("document card missing")
	}
	wantOpen := pixelAt(tt.Image(), card.X+40, btn.Y-4)
	if got := pixelAt(tt.Image(), x, y); !channelClose(got, wantOpen, 3) {
		t.Fatalf("open button pixel %v, want the card surface %v", got, wantOpen)
	}

	if err := tt.Click("Files panel"); err != nil {
		t.Fatal(err)
	}
	// The click leaves the pointer on the button; move it off so the
	// closed state reads idle paint, not hover.
	tt.Move(600, 300)
	tt.Frame()
	tt.Frame()
	if s.Sidebar.Visible {
		t.Fatal("the Files button must close the pane")
	}
	if _, ok := tt.Find("Files card"); ok {
		t.Fatal("files pane still drawn after the toggle closed it")
	}
	// Closed: still the card's face, with the glyph as the only state cue.
	card, ok = tt.Find("Document card")
	if !ok {
		t.Fatal("document card missing after close")
	}
	btn, ok = tt.Find("Files panel")
	if !ok {
		t.Fatal("files edge button missing after close")
	}
	wantClosed := pixelAt(tt.Image(), card.X+40, btn.Y-4)
	if got := pixelAt(tt.Image(), btn.X+3, btn.Y+3); !channelClose(got, wantClosed, 3) {
		t.Fatalf("closed button pixel %v, want the card surface %v", got, wantClosed)
	}
	if err := tt.Click("Files panel"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if !s.Sidebar.Visible {
		t.Fatal("the Files button must reopen the pane")
	}

	// Outline already toggles: the same walk holds there, and the two
	// panes stay independent.
	if err := tt.Click("Outline panel"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if s.Outline.Visible {
		t.Fatal("the Outline button must close the pane")
	}
	if err := tt.Click("Outline panel"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if !s.Outline.Visible {
		t.Fatal("the Outline button must reopen the pane")
	}
	if !s.Sidebar.Visible {
		t.Fatal("outline toggling must leave the files pane alone")
	}
}

// TestEdgeButtonsToggleOnlyOpensIsRed is the reverse check: a Files
// button wired only-open must fail the toggle test's first flip. Kept as
// a mirror of the wiring so the regression reads without history.
func TestEdgeButtonsToggleOnlyOpensIsRed(t *testing.T) {
	s, dir := newShell(t)
	if err := s.OpenFile(filepath.Join(dir, "a.md")); err != nil {
		t.Fatal(err)
	}
	s.Sidebar.Visible = true
	tt := s.renderSize(t, 1200, 600)
	tt.Frame()
	tt.Frame()

	// The wiring under test: the edge hands ToggleFiles, not
	// SetFilesVisible(true). Click once and the pane must be gone.
	if err := tt.Click("Files panel"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if s.Sidebar.Visible {
		t.Fatal("the edge button opens only; wire it to ToggleFiles so a click closes")
	}
}

// TestEdgeButtonPixelProbe documents the probe pixel the toggle test
// reads, so a background change upstream moves the assertion with it.
func TestEdgeButtonPixelProbe(t *testing.T) {
	s, dir := newShell(t)
	if err := s.OpenFile(filepath.Join(dir, "a.md")); err != nil {
		t.Fatal(err)
	}
	s.Sidebar.Visible = true
	tt := s.renderSize(t, 1200, 600)
	tt.Frame()
	tt.Frame()
	btn, ok := tt.Find("Files panel")
	if !ok {
		t.Fatal("files edge button missing")
	}
	card, ok := tt.Find("Document card")
	if !ok {
		t.Fatal("document card missing")
	}
	x, y := btn.X+3, btn.Y+3
	want := pixelAt(tt.Image(), card.X+40, btn.Y-4)
	if got := pixelAt(tt.Image(), x, y); !channelClose(got, want, 3) {
		t.Fatalf("probe %v, want the card surface %v; retune the toggle test's sample point", got, want)
	}
}

// TestTreeOverflowShot writes the deep-tree screenshot the overflow
// fix is verified on: rows inside the card, nothing past the card or
// the status bar. Skipped unless PEEKDOWN_SHOT is set.
func TestTreeOverflowShot(t *testing.T) {
	if cardShotPath == "" {
		t.Skip("set PEEKDOWN_SHOT=1 to write the tree screenshots")
	}
	s, dir := newShell(t)
	proj := deepTreeFixture(t, dir, 30)
	tt := openDeepTree(t, s, proj, 1440, 880)
	writePng(t, tt.Image(), "C:/tmp/tree-overflow-fixed.png")
}
