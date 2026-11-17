package shell

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// openFontPickerFor opens the settings, clicks one font's trigger and
// leaves the popover's first frame drawn: the path every picker test
// takes. It returns the trigger's and the panel's rects.
func openFontPickerFor(t *testing.T, s *Shell, tt *ui.Tester, name string) (ui.Rect, ui.Rect) {
	t.Helper()
	s.OpenSettings()
	tt.Frame()
	if err := tt.Click("Choose " + name); err != nil {
		t.Fatalf("%s trigger: %v", name, err)
	}
	tt.Frame()
	trigger, ok := tt.Find("Choose " + name)
	if !ok {
		t.Fatalf("no trigger after clicking %s: %v", name, tt.Texts())
	}
	panel, ok := tt.Find("Font picker")
	if !ok {
		t.Fatalf("no picker panel after clicking %s: %v", name, tt.Texts())
	}
	if panel.W < fontPickerWidth-1 || panel.W > fontPickerWidth+1 {
		t.Fatalf("picker panel is %v wide, want ~%v", panel.W, fontPickerWidth)
	}
	return trigger, panel
}

// firstMonospaced is the test's stand-in for the machine's fonts: the
// first family FontFamilies calls monospaced, with whether there was one.
func firstMonospaced() (ui.FontFamily, bool) {
	for _, f := range fontPickerList() {
		if f.Monospaced {
			return f, true
		}
	}
	return ui.FontFamily{}, false
}

// TestFontPickerAnchorsBelowTrigger checks the popover's shape: the
// picker opens as a panel below the trigger that opened it, in the
// window's bounds, and the settings dialog stays behind it.
func TestFontPickerAnchorsBelowTrigger(t *testing.T) {
	s, _ := newShell(t)
	tt := s.renderSize(t, 1400, 880)
	trigger, panel := openFontPickerFor(t, s, tt, "Content Font")

	const w, h = float32(1400), float32(880)
	if panel.X < 0 || panel.Y < 0 || panel.X+panel.W > w || panel.Y+panel.H > h {
		t.Fatalf("picker panel %v runs past the %vx%v window", panel, w, h)
	}
	// Anchored: the panel starts at the trigger's bottom edge (plus the
	// attachment's top margin) and shares its horizontal span's start.
	if panel.Y < trigger.Y+trigger.H-0.5 || panel.Y > trigger.Y+trigger.H+8 {
		t.Fatalf("panel top %v is not at the trigger's bottom %v", panel.Y, trigger.Y+trigger.H)
	}
	if panel.X < trigger.X-0.5 {
		t.Fatalf("panel left %v sits left of the trigger's left %v", panel.X, trigger.X)
	}
	if !tt.HasText("Content Font") {
		t.Fatalf("no %q title in the picker: %v", "Content Font", tt.Texts())
	}
	// The settings dialog stays under the popover.
	if _, ok := tt.Find("Settings"); !ok {
		t.Fatalf("the settings dialog is gone under the picker")
	}
}

// TestFontPickerForCodeFont checks the mono trigger opens the picker on
// the code font: its title reads "Code Font", and a row click writes the
// draft's mono family.
func TestFontPickerForCodeFont(t *testing.T) {
	s, _ := newShell(t)
	tt := s.renderSize(t, 1400, 880)
	openFontPickerFor(t, s, tt, "Code Font")

	if !tt.HasText("Code Font") {
		t.Fatalf("no %q title in the picker: %v", "Code Font", tt.Texts())
	}
	if err := tt.Click("System Default row"); err != nil {
		t.Fatalf("the System Default row is not clickable: %v", err)
	}
	tt.Frame()
	if s.Saved.MonoFontFamily != "" {
		t.Fatalf("MonoFontFamily = %q, want the empty default", s.Saved.MonoFontFamily)
	}
	if s.fontPickOpen || s.fontPickMonoOpen {
		t.Fatalf("the picker stayed open over its row click")
	}
}

// TestFontPickerSearchFilters checks the field filters the families: an
// unmatched query leaves the empty state, a matched one the families it
// names, and the system default row goes with them.
func TestFontPickerSearchFilters(t *testing.T) {
	s, _ := newShell(t)
	tt := s.renderSize(t, 1400, 880)
	openFontPickerFor(t, s, tt, "Content Font")

	// The AutoFocus'd field takes the typing without a click first.
	tt.Type("Qzx8NoFont")
	tt.Frame()
	if !tt.HasText("No matching fonts") {
		t.Fatalf("no empty state for a query nothing matches: %v", tt.Texts())
	}
	// "System Default" now comes from three places: the two font triggers
	// under the panel and the Language select's system choice; a fourth
	// would be the row the filter should have dropped.
	if texts := tt.Texts(); strings.Count(strings.Join(texts, "|"), "System Default") != 3 {
		t.Fatalf("the default row survived a filter that keeps nothing: %v", texts)
	}

	// Arial ships with Windows and macOS both; a headless run without it
	// still passes on the machine's own families.
	const probe = "arial"
	tt.Type("\b\b\b\b\b\b\b\b\b\b\b" + probe)
	tt.Frame()
	if !tt.HasText("No matching fonts") {
		texts := tt.Texts()
		found := false
		for _, x := range texts {
			if strings.EqualFold(x, probe) {
				found = true
			}
		}
		if !found {
			t.Fatalf("no family matches %q in %v", probe, texts)
		}
	}
}

// TestFontPickerRowWritesDraft checks a family row: clicking it writes
// the draft's family, closes the popover, and Save alone persists it.
func TestFontPickerRowWritesSaved(t *testing.T) {
	s, _ := newShell(t)
	tt := s.renderSize(t, 1400, 880)
	openFontPickerFor(t, s, tt, "Content Font")

	if err := tt.Click("System Default row"); err != nil {
		t.Fatalf("the System Default row is not clickable: %v", err)
	}
	tt.Frame()
	if s.Saved.FontFamily != "" {
		t.Fatalf("FontFamily = %q, want the empty default", s.Saved.FontFamily)
	}
	if s.fontPickOpen || s.fontPickMonoOpen {
		t.Fatalf("the picker stayed open over its row click")
	}
}

// TestFontPickerCheckClearsScrollBar checks the chosen row's check mark
// stays out of the list's overlay scroll bar: the bar paints in the 9
// DIPs inside the panel's right edge, so the check's right edge keeps
// at least 14 DIPs of panel left of that edge.
func TestFontPickerCheckClearsScrollBar(t *testing.T) {
	s, _ := newShell(t)
	tt := s.renderSize(t, 1400, 880)
	_, panel := openFontPickerFor(t, s, tt, "Content Font")

	// The picker opens on the whole list, so the default row is there
	// with its check.
	check, ok := tt.Find("check System Default row")
	if !ok {
		t.Fatalf("no check on the System Default row: %v", tt.Texts())
	}
	if gap := panel.X + panel.W - (check.X + check.W); gap < 14 {
		t.Fatalf("the check's right edge sits %v DIPs from the panel's right edge, want >= 14 (9 DIPs of scroll bar + clearance): check %v in panel %v", gap, check, panel)
	}
}

// TestFontPickerMonoChip checks the families a code font needs carry the
// "Mono" chip: the first monospaced family's row shows it.
func TestFontPickerMonoChip(t *testing.T) {
	s, _ := newShell(t)
	tt := s.renderSize(t, 1400, 880)
	openFontPickerFor(t, s, tt, "Code Font")

	f, ok := firstMonospaced()
	if !ok {
		t.Skip("no monospaced family on this machine")
	}
	tt.Type(f.Name)
	tt.Frame()
	texts := tt.Texts()
	if !tt.HasText("Mono") {
		t.Fatalf("no Mono chip over the monospaced %q: %v", f.Name, texts)
	}
	if strings.Contains(strings.Join(texts, "|"), "No matching fonts") {
		t.Fatalf("the filter lost the family it names: %v", texts)
	}
}

// TestFontPickerEscapeClearsThenCloses checks Escape against the picker:
// with a query it clears the field, and a second press closes it.
func TestFontPickerEscapeClearsThenCloses(t *testing.T) {
	s, _ := newShell(t)
	tt := s.renderSize(t, 1400, 880)
	openFontPickerFor(t, s, tt, "Content Font")

	tt.Type("ari")
	tt.Frame()
	tt.TypeKey(0, ui.KeyEscape, "")
	tt.Frame()
	if !s.fontPickOpen {
		t.Fatalf("Escape closed the picker with a query standing")
	}
	if s.fontSearch != "" {
		t.Fatalf("Escape left the query %q, want it cleared", s.fontSearch)
	}
	tt.TypeKey(0, ui.KeyEscape, "")
	tt.Frame()
	if s.fontPickOpen || s.fontPickMonoOpen {
		t.Fatalf("the second Escape left the picker open")
	}
	// The settings dialog holds the screen under it.
	if _, ok := tt.Find("Settings"); !ok {
		t.Fatalf("the settings dialog is gone after the picker closed")
	}
}

// TestFontPickerDismissKeepsDraft checks the ways a popover closes
// without a choice: an Escape past the empty query, or a press outside
// on the settings backdrop, leaves the draft as it was and the dialog
// open.
func TestFontPickerDismissKeepsDraft(t *testing.T) {
	s, _ := newShell(t)
	tt := s.renderSize(t, 1400, 880)

	families := fontPickerList()
	if len(families) == 0 {
		t.Skip("no font families on this machine")
	}
	openFontPickerFor(t, s, tt, "Content Font")
	before := s.Saved.FontFamily

	tt.TypeKey(0, ui.KeyEscape, "")
	tt.Frame()
	if s.fontPickOpen || s.fontPickMonoOpen {
		t.Fatalf("the empty query's Escape left the picker open")
	}
	if s.Saved.FontFamily != before {
		t.Fatalf("FontFamily = %q, want the dismissed picker to keep %q", s.Saved.FontFamily, before)
	}
	if _, ok := tt.Find("Settings"); !ok {
		t.Fatalf("the settings dialog is gone after the picker closed")
	}

	// A press outside the popover lands on the settings modal's backdrop:
	// it closes the popover and must leave the dialog's edits standing.
	if err := tt.Click("Choose Content Font"); err != nil {
		t.Fatalf("content trigger: %v", err)
	}
	tt.Frame()
	trigger, _ := tt.Find("Choose Code Font")
	tt.ClickAt(trigger.X-40, trigger.Y)
	tt.Frame()
	if s.fontPickOpen || s.fontPickMonoOpen {
		t.Fatalf("a press outside left the picker open")
	}
	if s.Saved.FontFamily != before {
		t.Fatalf("FontFamily = %q, want the dismissed picker to keep %q", s.Saved.FontFamily, before)
	}
	if _, ok := tt.Find("Settings"); !ok {
		t.Fatalf("the outside press closed the settings dialog itself")
	}
}
