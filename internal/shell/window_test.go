package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// longDoc writes a document tall enough that its natural height runs past
// any window these tests use, so only the layout keeps the chrome in view.
func longDoc(t *testing.T, dir string) string {
	t.Helper()
	var b strings.Builder
	for i := range 120 {
		fmt.Fprintf(&b, "Paragraph %d with enough words to wrap across a couple of lines of the column.\n\n", i)
	}
	path := filepath.Join(dir, "long.md")
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// statusBarLabels are the strip's controls, the rows the viewport must hold.
var statusBarLabels = []string{"Files", "Outline", "Search", "Settings"}

// TestStatusBarStaysInTheWindow pins the root column's height budget: a
// long document must not push the status bar past the window's bottom
// edge, with the sidebar hidden or shown (the two body paths).
func TestStatusBarStaysInTheWindow(t *testing.T) {
	const w, h = 1400, 880
	for _, sidebar := range []bool{false, true} {
		s, dir := newShell(t)
		if err := s.OpenFile(longDoc(t, dir)); err != nil {
			t.Fatal(err)
		}
		s.Sidebar.Visible = sidebar
		tt := s.renderSize(t, w, h)
		tt.Frame()

		for _, label := range statusBarLabels {
			r, ok := tt.Find(label)
			if !ok {
				t.Fatalf("sidebar %v: texts %v: want the %q control", sidebar, tt.Texts(), label)
			}
			if r.H <= 0 || r.W <= 0 {
				t.Fatalf("sidebar %v: %q collapsed to %v", sidebar, label, r)
			}
			if r.Y < 0 || r.Y+r.H > h {
				t.Fatalf("sidebar %v: %q at %v sits outside the %d DIP window", sidebar, label, r, h)
			}
		}

		// The list is bounded by the body, not by the document: its box
		// ends above the status bar, which is what lets it clip and
		// scroll instead of pushing the strip off the window.
		var viewCtx *ui.Context
		s.ContentHook = func(ctx *ui.Context) { viewCtx = ctx }
		tt.Frame()
		list := s.ActiveTab().State.List.Handle.Bounds(viewCtx)
		if list.H <= 0 {
			t.Fatalf("sidebar %v: the list has no committed bounds", sidebar)
		}
		if list.Y+list.H > h-28 {
			t.Fatalf("sidebar %v: the list at %v runs past the status bar's top edge", sidebar, list)
		}
		// And it does scroll: a jump to the last row moves the text up.
		text, ok := paragraphRect(tt, "Paragraph 0 ")
		if !ok {
			t.Fatalf("sidebar %v: texts %v: want the first paragraph", sidebar, tt.Texts())
		}
		s.ActiveTab().State.List.ScrollIntoView(s.ActiveTab().Model.Len() - 1)
		tt.Frame()
		if moved, ok := paragraphRect(tt, "Paragraph 0 "); ok && moved.Y >= text.Y {
			t.Fatalf("sidebar %v: the text stayed at %v: want the list to scroll", sidebar, text)
		}
		if after, _ := tt.Find("Settings"); after.Y+after.H > h {
			t.Fatalf("sidebar %v: the status bar left the window on a scroll: %v", sidebar, after)
		}
	}
}

// TestStatusBarFitsAShortDocumentToo keeps the ordinary case whole: a
// three line document leaves the status bar at the bottom edge and the
// body takes the room between the bars.
func TestStatusBarFitsAShortDocumentToo(t *testing.T) {
	const w, h = 1400, 880
	s, dir := newShell(t)
	path := filepath.Join(dir, "tiny.md")
	if err := os.WriteFile(path, []byte("# T\n\nx\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	tt := s.renderSize(t, w, h)
	tt.Frame()

	r, ok := tt.Find("Settings")
	if !ok {
		t.Fatalf("texts %v: want the settings control", tt.Texts())
	}
	// The bar hugs the bottom: its 28 DIP row ends within a DIP of the edge.
	if got := float32(h) - (r.Y + r.H); got < 0 || got > 2 {
		t.Fatalf("status bar ends %v DIPs from the bottom edge: %v", got, r)
	}
}

// TestWelcomeRecentListScrolls proves the recent list renders its rows
// and scrolls: ten entries cannot all fit the 300 DIP cap, so the first
// shows at a real height and the scroll container keeps them reachable.
func TestWelcomeRecentListScrolls(t *testing.T) {
	for _, h := range []int{600, 503} {
		welcomeRecent(t, 800, h)
	}
}

func welcomeRecent(t *testing.T, w, h int) {
	t.Helper()
	s, dir := newShell(t)
	var paths []string
	for i := range 10 {
		p := filepath.Join(dir, fmt.Sprintf("doc%02d.md", i))
		if err := os.WriteFile(p, []byte("# x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}
	s.Recent = func() []string { return append([]string(nil), paths...) }
	tt := s.renderSize(t, w, h)
	tt.Frame()

	first, ok := tt.Find("doc00.md")
	if !ok {
		t.Fatalf("texts %v: want the first recent entry", tt.Texts())
	}
	if first.H < 10 {
		t.Fatalf("the first recent row is %v: want a drawn row, not a collapsed one", first)
	}
	if first.Y < 0 || first.Y+first.H > float32(h) {
		t.Fatalf("the first recent row at %v sits outside the window", first)
	}

	// The rows stack at a row's own pitch: a collapsed row would spread
	// them over the height it took instead.
	second, ok := tt.Find("doc01.md")
	if !ok {
		t.Fatalf("texts %v: want the second recent entry", tt.Texts())
	}
	if pitch := second.Y - first.Y; pitch > 60 {
		t.Fatalf("the rows sit %v DIPs apart: want a row's own pitch", pitch)
	}

	// The 300 DIP cap scrolls the overflow: a scroll over the list brings
	// the last entry into the window.
	tt.Scroll(first.X+10, first.Y+5, 0, 600)
	tt.Frame()
	last, ok := tt.Find("doc09.md")
	if !ok {
		t.Fatalf("texts %v: want the last recent entry", tt.Texts())
	}
	if last.Y+last.H > float32(h) || last.H < 10 {
		t.Fatalf("the last recent row at %v did not scroll into the %d DIP window", last, h)
	}
	// The list's own box ends inside the window too, so no row starts below
	// it: the cap follows the room the greeting leaves.
	tt.Scroll(last.X+10, last.Y-20, 0, -2000)
	tt.Frame()
	if top, ok := tt.Find("doc00.md"); !ok || top.Y+top.H > float32(h) {
		t.Fatalf("height %d: the first row at %v runs past the bottom edge", h, top)
	}
}

// TestSettingsDialogReachesSave pins the dialog's height budget in a
// window too short for the whole form: Cancel and Save stay in the
// window, and the fields scroll above them.
func TestSettingsDialogReachesSave(t *testing.T) {
	for _, h := range []int{503, 400} {
		s, _ := newShell(t)
		tt := s.renderSize(t, 800, h)
		s.OpenSettings()
		tt.Frame()

		for _, label := range []string{"Cancel", "Save"} {
			r, ok := tt.Find(label)
			if !ok {
				t.Fatalf("height %d: texts %v: want the %q button", h, tt.Texts(), label)
			}
			if r.H < 16 {
				t.Fatalf("height %d: %q is %v: want a drawn button", h, label, r)
			}
			if r.Y < 0 || r.Y+r.H > float32(h) {
				t.Fatalf("height %d: %q at %v sits outside the window", h, label, r)
			}
		}
		// The fields scroll: the last group is out of view at the top of
		// the list and comes in on a scroll over it.
		if r, _ := tt.Find("Auto Refresh"); r.H > 0 {
			t.Fatalf("height %d: the last group is already drawn at %v: want it below the scroll", h, r)
		}
		theme, ok := tt.Find("Theme")
		if !ok {
			t.Fatalf("height %d: texts %v: want the first field", h, tt.Texts())
		}
		tt.Scroll(theme.X+20, theme.Y+8, 0, 2000)
		tt.Frame()
		if r, _ := tt.Find("Auto Refresh"); r.H < 16 || r.Y+r.H > float32(h) {
			t.Fatalf("height %d: the last group at %v did not scroll into view", h, r)
		}

		if err := tt.Click("Save"); err != nil {
			t.Fatalf("height %d: %v", h, err)
		}
		tt.Frame()
		if s.SettingsOpen {
			t.Fatalf("height %d: the dialog stayed open after Save", h)
		}
	}
}
