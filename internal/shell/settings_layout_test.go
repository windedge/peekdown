package shell

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestSettingsSegmentedFitsContent checks the settings' segmented tracks
// keep to their content's width. Grow(1) used to stretch the track across
// the row's remaining width, leaving dead surface right of the choices;
// the choices do not stretch with the track, so the track's own bounds
// are what can see the bug.
func TestSettingsSegmentedFitsContent(t *testing.T) {
	s, _ := newShell(t)
	tt := s.renderSize(t, 1400, 880)
	s.OpenSettings()
	tt.Frame()

	title, ok := tt.Find("Settings")
	if !ok {
		t.Fatalf("no dialog title in %q", tt.Texts())
	}
	// The dialog's column is 480 wide with 16 of padding either side, and
	// the fields' scroll bar lies over its last 9 DIPs.
	const content, gutter = 480 - 2*16, barClearance
	limit := title.X + content - gutter

	for _, tc := range []struct {
		track, left, right string
	}{
		{"Layout", "Centered", "Full Width"},
		{"Explorer Root", "Current Directory", "Project Directory"},
	} {
		track, ok := tt.AccessByRole(ui.RoleRadioGroup, tc.track)
		if !ok {
			t.Fatalf("no segmented track named %q", tc.track)
		}
		l, ok := tt.Find(tc.left)
		if !ok {
			t.Fatalf("no %q label in %q", tc.left, tt.Texts())
		}
		r, ok := tt.Find(tc.right)
		if !ok {
			t.Fatalf("no %q label in %q", tc.right, tt.Texts())
		}
		// A stretched track runs to the row limit, far past its content;
		// a natural one hugs it, only its own padding wider.
		contentW := union(l, r).W
		if track.Bounds.W > contentW+30 {
			t.Errorf("%s track is %v wide for %v of content: stretched past its choices",
				tc.track, track.Bounds.W, contentW)
		}
		if got := track.Bounds.X + track.Bounds.W; got > limit {
			t.Errorf("%s track ends at %v, past the row limit %v", tc.track, got, limit)
		}
	}
}

// TestSettingsMarkersInputClearsScrollbar checks the Project Root Markers
// input against the dialog's content column. Left free it filled the row
// and its text ran under the fields' scroll bar, cut off mid-marker.
func TestSettingsMarkersInputClearsScrollbar(t *testing.T) {
	s, _ := newShell(t)
	s.markersText = "Cargo.toml, package.json, pyproject.toml, go.mod"
	tt := s.renderSize(t, 1400, 880)
	s.OpenSettings()
	tt.Frame()

	title, ok := tt.Find("Settings")
	if !ok {
		t.Fatalf("no dialog title in %q", tt.Texts())
	}
	// The dialog's column is 480 wide with 16 of padding either side, and
	// the fields' scroll bar lies over its last 9 DIPs: the input must keep
	// 12 DIPs clear of the edge, or the bar covers its text's tail.
	const content, gutter = 480 - 2*16, barClearance
	limit := title.X + content - gutter

	// The field's label names the input, so the accessibility tree carries
	// the input's own bounds, not the label text's.
	in, ok := tt.AccessByRole(ui.RoleTextField, "Project Root Markers")
	if !ok {
		t.Fatalf("no markers input in %q", tt.Texts())
	}
	if in.Bounds.X+in.Bounds.W > limit {
		t.Errorf("the markers input ends at %v, past the row limit %v", in.Bounds.X+in.Bounds.W, limit)
	}
	// Grow(1) takes the row it can, MaxWidth(292) caps it: the cap must
	// hold so the row keeps the bar's strip clear at every window width.
	if in.Bounds.W > 292.5 {
		t.Errorf("the markers input is %v wide, past its 292 cap", in.Bounds.W)
	}
}

// union covers the rects a and b with one rect.
func union(a, b ui.Rect) ui.Rect {
	x2 := max(a.X+a.W, b.X+b.W)
	y2 := max(a.Y+a.H, b.Y+b.H)
	a.X = min(a.X, b.X)
	a.Y = min(a.Y, b.Y)
	a.W = x2 - a.X
	a.H = y2 - a.H
	return a
}

// TestSettingsFontRowsFit checks the editor's font fields against the
// dialog's content column. The size spinner used to hang past its right
// edge: the dropdown took the whole row, the "+" was clipped, and the
// scroll bar covered what was left of it.
func TestSettingsFontRowsFit(t *testing.T) {
	s, _ := newShell(t)
	tt := s.renderSize(t, 1400, 880)
	s.OpenSettings()
	tt.Frame()

	title, ok := tt.Find("Settings")
	if !ok {
		t.Fatalf("no dialog title in %q", tt.Texts())
	}
	// The dialog's column is 480 wide with 16 of padding either side, and
	// the fields' scroll bar lies over its last 9 DIPs: the row must keep
	// 12 DIPs clear of the edge, or the bar covers the spinner's "+" while
	// it shows.
	const content, gutter = 480 - 2*16, barClearance
	limit := title.X + content - gutter

	// Each font's size spinner names its box: both rows must fit, not
	// just the one whose buttons the tester finds first.
	for _, name := range []string{"Content Font size", "Code Font size"} {
		r, ok := tt.Find(name)
		if !ok {
			t.Fatalf("no %q box in %q", name, tt.Texts())
		}
		if r.X+r.W > limit {
			t.Errorf("%s ends at %v, past %v", name, r.X+r.W, limit)
		}
		// A clipped box reports the room left for it, not its own width.
		if r.W < 130 {
			t.Errorf("%s is %v wide, want the spinner's full width", name, r.W)
		}
	}

	// The stepper's buttons sit at the row's end: the "+" must be whole
	// and clickable, not half drawn under the scroll bar.
	minus, ok := tt.Find("Decrease")
	if !ok {
		t.Fatalf("no stepper buttons in %q", tt.Texts())
	}
	plus, ok := tt.Find("Increase")
	if !ok {
		t.Fatalf("no stepper buttons in %q", tt.Texts())
	}
	for _, b := range []struct {
		name string
		r    ui.Rect
	}{{"Decrease", minus}, {"Increase", plus}} {
		if b.r.X+b.r.W > limit {
			t.Errorf("%s button ends at %v, past %v", b.name, b.r.X+b.r.W, limit)
		}
		if b.r.W < 20 {
			t.Errorf("%s button is %v wide, too narrow to be whole", b.name, b.r.W)
		}
	}

	before := s.Draft.FontSize
	if err := tt.Click("Increase"); err != nil {
		t.Fatalf("the size spinner's + is not clickable: %v", err)
	}
	tt.Frame()
	if got := s.Draft.FontSize; got != before+1 {
		t.Fatalf("FontSize = %v after +, want %v", got, before+1)
	}
}

// TestSettingsFooterButtonsRightAligned checks the dialog's footer: Cancel
// and Save hug the right edge, spaced apart, on one line, and stay inside
// the dialog's padded content. A row that kept to its content left them
// bunched under the title, at the dialog's left; a FillWidth row ran over
// the panel's right padding once a narrow window clamped the column below
// its 480.
func TestSettingsFooterButtonsRightAligned(t *testing.T) {
	for _, size := range []struct{ w, h int }{{1400, 880}, {760, 620}} {
		s, _ := newShell(t)
		tt := s.renderSize(t, size.w, size.h)
		s.OpenSettings()
		tt.Frame()

		title, ok := tt.Find("Settings")
		if !ok {
			t.Fatalf("%dx%d: no dialog title in %q", size.w, size.h, tt.Texts())
		}
		// The title stretches across the column's content box, so its own
		// width is that box's: the padded column, less the panel's edge.
		contentRight := title.X + title.W
		centerX := title.X + title.W/2

		cancel, ok := tt.Find("Cancel")
		if !ok {
			t.Fatalf("%dx%d: no Cancel button in %q", size.w, size.h, tt.Texts())
		}
		save, ok := tt.Find("Save")
		if !ok {
			t.Fatalf("%dx%d: no Save button in %q", size.w, size.h, tt.Texts())
		}

		for _, b := range []struct {
			name string
			r    ui.Rect
		}{{"Cancel", cancel}, {"Save", save}} {
			if b.r.X <= centerX {
				t.Errorf("%dx%d: %s starts at %v, not past the dialog's center %v",
					size.w, size.h, b.name, b.r.X, centerX)
			}
			if b.r.X+b.r.W > contentRight {
				t.Errorf("%dx%d: %s ends at %v, past the dialog's content edge %v",
					size.w, size.h, b.name, b.r.X+b.r.W, contentRight)
			}
		}
		if gap := save.X - (cancel.X + cancel.W); gap < 12 {
			t.Errorf("%dx%d: Cancel and Save are %v apart, want the footer's 14 DIP gap",
				size.w, size.h, gap)
		}
		if inset := contentRight - (save.X + save.W); inset < 8 || inset > 32 {
			t.Errorf("%dx%d: Save ends %v from the dialog's content edge, want it near the right",
				size.w, size.h, inset)
		}
		if d := save.Y - cancel.Y; d < -2 || d > 2 {
			t.Errorf("%dx%d: Cancel and Save are %v apart vertically, want one footer line",
				size.w, size.h, d)
		}
		if cancel.X >= save.X {
			t.Errorf("%dx%d: Cancel starts at %v, not left of Save at %v",
				size.w, size.h, cancel.X, save.X)
		}
	}
}
