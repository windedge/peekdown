package shell

import (
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/settings"
)

// TestSettingsDialogCards checks the modal's shape: 560 wide (clamped in
// a narrow window), a title line over section cards, and no Cancel or
// Save - the footer is Done alone, the changes apply as they are made.
func TestSettingsDialogCards(t *testing.T) {
	for _, size := range []struct{ w, h int }{{1400, 880}, {700, 620}} {
		s, _ := newShell(t)
		tt := s.renderSize(t, size.w, size.h)
		s.OpenSettings()
		tt.Frame()

		title, ok := tt.Find("Settings")
		if !ok {
			t.Fatalf("%dx%d: no dialog title in %q", size.w, size.h, tt.Texts())
		}
		wantW := float32(min(560, size.w-48))
		if got := title.X - 20; got < 0 {
			t.Fatalf("%dx%d: the dialog's column starts at %v, off the window", size.w, size.h, got)
		}
		// The panel's width shows in the section captions' common left
		// edge and the window's margins: the title sits 20 DIPs in from
		// the panel's left edge, and the panel is wantW wide, centered.
		panelLeft := title.X - 20
		panelRight := panelLeft + wantW
		if panelRight > float32(size.w) {
			t.Fatalf("%dx%d: the panel ends at %v, past the window", size.w, size.h, panelRight)
		}

		// One caption per section, all on the panel's left edge.
		for _, cap := range []string{"Appearance", "Editor", "Files", "System"} {
			r, ok := tt.Find(cap)
			if !ok {
				t.Fatalf("%dx%d: no %q caption in %q", size.w, size.h, cap, tt.Texts())
			}
			if d := r.X - title.X; d < -1 || d > 1 {
				t.Errorf("%dx%d: %q caption at %v is not on the title's left edge %v", size.w, size.h, cap, r.X, title.X)
			}
		}

		// No draft-era chrome.
		if _, ok := tt.Find("Cancel"); ok {
			t.Errorf("%dx%d: Cancel still drawn", size.w, size.h)
		}
		if _, ok := tt.Find("Save"); ok {
			t.Errorf("%dx%d: Save still drawn", size.w, size.h)
		}
		done, ok := tt.Find("Done")
		if !ok {
			t.Fatalf("%dx%d: no Done button in %q", size.w, size.h, tt.Texts())
		}
		if done.Y < title.Y {
			t.Errorf("%dx%d: Done at %v above the title at %v", size.w, size.h, done, title)
		}

		// Done closes.
		if err := tt.Click("Done"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if s.SettingsOpen {
			t.Fatalf("%dx%d: the dialog stayed open after Done", size.w, size.h)
		}
	}
}

// TestSettingsRowsTwoLineLayout checks the rows' shape: the label's
// column grows (so the control is pushed to the card's right edge), the
// control clears the scroll bar's strip, and a Theme row's detail line
// sits under its label.
func TestSettingsRowsTwoLineLayout(t *testing.T) {
	s, _ := newShell(t)
	tt := s.renderSize(t, 1400, 880)
	s.OpenSettings()
	tt.Frame()

	title, _ := tt.Find("Settings")
	// The fields' scroll bar paints over the content column's last 9
	// DIPs; every control must end 12 DIPs before the column's edge.
	content := float32(min(560, 1400-48)) - 2*20 - 12
	limit := title.X - 20 + 20 + content

	label, ok := tt.Find("Theme")
	if !ok {
		t.Fatalf("no Theme row in %q", tt.Texts())
	}
	detail, ok := tt.Find("Light and dark follow the system when Auto.")
	if !ok {
		t.Fatalf("no Theme detail line in %q", tt.Texts())
	}
	if detail.Y <= label.Y {
		t.Errorf("the detail line at %v is not under the label at %v", detail, label)
	}

	// The control column: the theme segmented sits right of the label,
	// before the scroll bar's strip.
	seg, ok := tt.AccessByRole(ui.RoleRadioGroup, "Theme")
	if !ok {
		t.Fatalf("no theme segmented in %q", tt.Texts())
	}
	if seg.Bounds.X <= label.X+label.W {
		t.Errorf("the theme segmented at %v is not right of the label ending at %v", seg.Bounds, label.X+label.W)
	}
	if end := seg.Bounds.X + seg.Bounds.W; end > limit {
		t.Errorf("the theme segmented ends at %v, past the row limit %v", end, limit)
	}

	// Every switch's box clears the strip too.
	for _, name := range []string{"Inertia Scroll", "Auto Refresh", "Show FPS"} {
		sw, ok := tt.AccessByRole(ui.RoleSwitch, name)
		if !ok {
			t.Fatalf("no %q switch in %q", name, tt.Texts())
		}
		if end := sw.Bounds.X + sw.Bounds.W; end > limit {
			t.Errorf("the %s switch ends at %v, past %v", name, end, limit)
		}
	}

	// The font rows' spinners keep their place at the row's end.
	for _, name := range []string{"Content Font size", "Code Font size"} {
		box, ok := tt.Find(name)
		if !ok {
			t.Fatalf("no %q box in %q", name, tt.Texts())
		}
		if box.X+box.W > limit {
			t.Errorf("%s ends at %v, past %v", name, box.X+box.W, limit)
		}
	}
}

// TestSettingsThemeAppliesImmediately checks the spec's headline change:
// choosing Dark in the dialog applies the theme at once, through Saved
// and the host theme, without any Save press.
func TestSettingsThemeAppliesImmediately(t *testing.T) {
	s, _ := newShell(t)
	var applied string
	s.SetTheme = func(src mygo.ThemeSource) { applied = string(src) }
	if s.SetTheme == nil {
		t.Fatal("no theme sink")
	}
	tt := s.renderSize(t, 1400, 880)
	s.OpenSettings()
	tt.Frame()

	if err := tt.Click("Dark"); err != nil {
		t.Fatalf("no Dark segment: %v", err)
	}
	tt.Frame()
	if s.Saved.Theme != "Dark" {
		t.Fatalf("theme = %q right after the click, want Dark applied at once", s.Saved.Theme)
	}
	if applied != "dark" {
		t.Fatalf("host theme = %q right after the click, want dark", applied)
	}
}

// TestSettingsFontSizeDebounce checks the spinner's write is debounced:
// the value lands in Saved at once (the view reads it next frame), but
// the disk write waits out the 300 ms quiet.
func TestSettingsFontSizeDebounce(t *testing.T) {
	s, _ := newShell(t)
	saves := 0
	s.Save = func(v settings.Settings) error { saves++; return nil }
	tt := s.renderSize(t, 1400, 880)
	s.OpenSettings()
	tt.Frame()

	before := s.Saved.FontSize
	if err := tt.Click("Increase"); err != nil {
		t.Fatalf("the size spinner's + is not clickable: %v", err)
	}
	tt.Frame()
	if got := s.Saved.FontSize; got != before+1 {
		t.Fatalf("FontSize = %v after +, want %v", got, before+1)
	}
	if saves != 0 {
		t.Fatalf("the write landed %d times before the debounce, want it held", saves)
	}

	tt.Advance(settingsDebounce + 50*time.Millisecond)
	tt.Frame()
	if saves < 1 {
		t.Fatal("the debounced write never landed")
	}

	// Two changes inside the window collapse into one write.
	if err := tt.Click("Increase"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("Increase"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	writes := saves
	tt.Advance(settingsDebounce + 50*time.Millisecond)
	tt.Frame()
	if saves != writes+1 {
		t.Fatalf("%d writes after two changes inside the window, want 1", saves-writes)
	}
	if got := s.Saved.FontSize; got != before+3 {
		t.Fatalf("FontSize = %v, want %v", got, before+3)
	}
}

// TestSettingsMarkersRowFollowsExplorerRoot checks the Project Root
// Markers row shows only while the explorer's root is the project
// directory. Toggling the segmented shows and hides the row, and the
// input keeps the typed markers throughout, so a round trip does not
// lose what was in it.
func TestSettingsMarkersRowFollowsExplorerRoot(t *testing.T) {
	s, _ := newShell(t)
	tt := s.renderSize(t, 1400, 880)
	s.OpenSettings()
	tt.Frame()

	// The default root is the project directory: the markers row shows,
	// seeded with the saved markers.
	if _, ok := tt.AccessByRole(ui.RoleTextField, "Project Root Markers"); !ok {
		t.Fatalf("no markers input at the default project root, in %q", tt.Texts())
	}

	// Switch the root to the current directory: the row goes, and the
	// mode applies at once.
	if err := tt.Click("Current Directory"); err != nil {
		t.Fatalf("no Current Directory choice in %q", tt.Texts())
	}
	tt.Frame()
	if s.Saved.ExplorerRootMode != "current_dir" {
		t.Fatalf("ExplorerRootMode = %q after switching to the current directory, want current_dir applied at once", s.Saved.ExplorerRootMode)
	}
	if _, ok := tt.AccessByRole(ui.RoleTextField, "Project Root Markers"); ok {
		t.Fatal("markers input still shown while the root is the current directory")
	}
	// Plant the text the way a keystroke would: the input's Changed
	// keeps the saved list in step, Apply resets the text from it.
	const typed = "Cargo.toml, package.json"
	s.markersText = typed
	s.Saved.ProjectRootMarkers = splitMarkers(typed)

	// Switch back to the project directory: the row returns, and the
	// input's text survives for the next hiding.
	if err := tt.Click("Project Directory"); err != nil {
		t.Fatalf("no Project Directory choice in %q", tt.Texts())
	}
	tt.Frame()
	if s.Saved.ExplorerRootMode != "project_root" {
		t.Fatalf("ExplorerRootMode = %q after switching back, want project_root", s.Saved.ExplorerRootMode)
	}
	if _, ok := tt.AccessByRole(ui.RoleTextField, "Project Root Markers"); !ok {
		t.Fatal("markers input still hidden after switching back to the project directory")
	}
	if s.markersText != typed {
		t.Fatalf("markersText = %q, the dialog lost the typed markers", s.markersText)
	}

	// Once more to the current directory: the row is gone with the typed
	// text kept for the next showing.
	if err := tt.Click("Current Directory"); err != nil {
		t.Fatalf("no Current Directory choice in %q", tt.Texts())
	}
	tt.Frame()
	if _, ok := tt.AccessByRole(ui.RoleTextField, "Project Root Markers"); ok {
		t.Fatal("markers input still shown after the second switch to the current directory")
	}
}
