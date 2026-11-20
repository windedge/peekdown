package shell

import (
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/settings"
)

// openSettingsSection opens the dialog and chooses one of the rail's
// pages by its name as the reader sees it.
func openSettingsSection(t *testing.T, tt *ui.Tester, s *Shell, name string) {
	t.Helper()
	s.OpenSettings()
	tt.Frame()
	if err := tt.Click("Settings nav " + name); err != nil {
		t.Fatalf("no %q in the settings rail: %v", name, err)
	}
	tt.Frame()
}

// TestSettingsDialogCards checks the two-column modal: its width follows
// the window up to 900, the rail and the page column sit side by side,
// and the single Close footer button still closes.
func TestSettingsDialogCards(t *testing.T) {
	for _, size := range []struct{ w, h int }{{1400, 880}, {760, 620}} {
		s, _ := newShell(t)
		tt := s.renderSize(t, size.w, size.h)
		s.OpenSettings()
		tt.Frame()

		panel, ok := tt.Find("Settings panel")
		if !ok {
			t.Fatalf("%dx%d: no settings panel in %q", size.w, size.h, tt.Texts())
		}
		wantW := float32(min(settingsWidth, size.w-48))
		if panel.W != wantW {
			t.Fatalf("%dx%d: panel is %v wide, want %v", size.w, size.h, panel.W, wantW)
		}
		if panel.X < 24 || panel.X+panel.W > float32(size.w-24) {
			t.Fatalf("%dx%d: panel %v does not keep the modal margin", size.w, size.h, panel)
		}
		if panel.H != float32(min(settingsMaxHeight, size.h-48)) {
			t.Fatalf("%dx%d: panel %v is not the fixed modal height", size.w, size.h, panel)
		}

		title, ok := tt.Find("Settings")
		if !ok {
			t.Fatalf("%dx%d: no dialog title in %q", size.w, size.h, tt.Texts())
		}
		rail, ok := tt.Find("Settings rail")
		if !ok {
			t.Fatalf("%dx%d: no settings rail in %q", size.w, size.h, tt.Texts())
		}
		if rail.W != settingsRailWidth {
			t.Fatalf("%dx%d: rail is %v wide, want %v", size.w, size.h, rail.W, settingsRailWidth)
		}
		if rail.X < title.X || rail.X+rail.W > panel.X+panel.W {
			t.Fatalf("%dx%d: rail %v does not sit inside the panel", size.w, size.h, rail)
		}
		header, ok := tt.Find("Settings header")
		if !ok {
			t.Fatalf("%dx%d: no section header in %q", size.w, size.h, tt.Texts())
		}
		if header.X <= rail.X+rail.W || header.X >= panel.X+panel.W {
			t.Fatalf("%dx%d: header %v does not sit right of the rail", size.w, size.h, header)
		}

		// The pane labels reach the rail, but only the chosen section's
		// card is in the detail column.
		for _, cap := range []string{"Appearance", "Reading", "About"} {
			if _, ok := tt.Find("Settings nav " + cap); !ok {
				t.Fatalf("%dx%d: no %q nav item in %q", size.w, size.h, cap, tt.Texts())
			}
			if _, ok := tt.Find(cap + " card"); ok && cap != "Appearance" {
				t.Fatalf("%dx%d: the %s card is drawn while Appearance is chosen", size.w, size.h, cap)
			}
		}
		if _, ok := tt.Find("Appearance card"); !ok {
			t.Fatalf("%dx%d: no Appearance card in %q", size.w, size.h, tt.Texts())
		}

		// No draft-era chrome.
		if _, ok := tt.Find("Cancel"); ok {
			t.Errorf("%dx%d: Cancel still drawn", size.w, size.h)
		}
		if _, ok := tt.Find("Save"); ok {
			t.Errorf("%dx%d: Save still drawn", size.w, size.h)
		}
		closeButton, ok := tt.Find("Settings Close")
		if !ok {
			t.Fatalf("%dx%d: no Close button in %q", size.w, size.h, tt.Texts())
		}
		if closeButton.Y < title.Y || closeButton.Y+closeButton.H > panel.Y+panel.H+1 {
			t.Fatalf("%dx%d: Close at %v is not the panel's footer", size.w, size.h, closeButton)
		}

		if err := tt.Click("Settings Close"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if s.SettingsOpen {
			t.Fatalf("%dx%d: the dialog stayed open after Close", size.w, size.h)
		}
	}
}

// rowLimit is where a card's controls must stop: the card's inner edge
// after its 16 DIP padding, clear of the right scroll bar's strip.
func rowLimit(t *testing.T, tt *ui.Tester, card string) float32 {
	t.Helper()
	r, ok := tt.Find(card)
	if !ok {
		t.Fatalf("no %q in %q", card, tt.Texts())
	}
	return r.X + r.W - settingsCardPadX
}

// TestSettingsRowsTwoLineLayout checks the rows' shape in the page
// column: the label's column grows so the control is pushed to the
// card's right edge, and a Theme row's detail line sits under its label.
func TestSettingsRowsTwoLineLayout(t *testing.T) {
	s, _ := newShell(t)
	tt := s.renderSize(t, 1400, 880)
	s.OpenSettings()
	tt.Frame()

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

	limit := rowLimit(t, tt, "Appearance card")
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

	// The Reading page keeps the font spinners and the inertia switch inside
	// its own card, which has the same right edge.
	if err := tt.Click("Settings nav Reading"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	limit = rowLimit(t, tt, "Reading card")
	for _, name := range []string{"Content Font size", "Code Font size"} {
		box, ok := tt.Find(name)
		if !ok {
			t.Fatalf("no %q box in %q", name, tt.Texts())
		}
		if box.X+box.W > limit {
			t.Errorf("%s ends at %v, past %v", name, box.X+box.W, limit)
		}
	}
	sw, ok := tt.AccessByRole(ui.RoleSwitch, "Inertia Scroll")
	if !ok {
		t.Fatalf("no Inertia Scroll switch in %q", tt.Texts())
	}
	if end := sw.Bounds.X + sw.Bounds.W; end > limit {
		t.Errorf("the inertia switch ends at %v, past %v", end, limit)
	}

	// Auto Refresh moved into this same Reading card: check its switch.
	auto, ok := tt.AccessByRole(ui.RoleSwitch, "Auto Refresh")
	if !ok {
		t.Fatalf("no Auto Refresh switch in %q", tt.Texts())
	}
	if end := auto.Bounds.X + auto.Bounds.W; end > limit {
		t.Errorf("the Auto Refresh switch ends at %v, past %v", end, limit)
	}

	if err := tt.Click("Settings nav Appearance"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	limit = rowLimit(t, tt, "Appearance card")
	fps, ok := tt.AccessByRole(ui.RoleSwitch, "Show FPS")
	if !ok {
		t.Fatalf("no Show FPS switch in %q", tt.Texts())
	}
	if end := fps.Bounds.X + fps.Bounds.W; end > limit {
		t.Errorf("the Show FPS switch ends at %v, past %v", end, limit)
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
// the disk write waits out the 300 ms quiet. The size field lives on
// the Reading page, so the test walks there first.
func TestSettingsFontSizeDebounce(t *testing.T) {
	s, _ := newShell(t)
	saves := 0
	s.Save = func(v settings.Settings) error { saves++; return nil }
	tt := s.renderSize(t, 1400, 880)
	openSettingsSection(t, tt, s, "Reading")

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
// directory. Both rows live at the end of the Reading page, so the test
// walks there first; toggling the segmented keeps the typed markers.
func TestSettingsMarkersRowFollowsExplorerRoot(t *testing.T) {
	s, _ := newShell(t)
	tt := s.renderSize(t, 1400, 880)
	openSettingsSection(t, tt, s, "Reading")

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
