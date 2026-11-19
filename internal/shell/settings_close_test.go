package shell

import (
	"os"
	"testing"

	"github.com/windedge/peekdown/locales"
)

// TestSettingsCloseEntryPoints checks the two new close controls: a
// secondary Close beside Done at the panel's foot, and the icon at its
// top-right corner. Both close the dialog, as Done still does.
func TestSettingsCloseEntryPoints(t *testing.T) {
	for _, size := range []struct{ w, h int }{{1400, 880}, {760, 620}} {
		s, _ := newShell(t)
		tt := s.renderSize(t, size.w, size.h)
		s.OpenSettings()
		tt.Frame()

		panel, ok := tt.Find("Settings panel")
		if !ok {
			t.Fatalf("%dx%d: no panel in %q", size.w, size.h, tt.Texts())
		}
		title, ok := tt.Find("Settings")
		if !ok {
			t.Fatalf("%dx%d: no title in %q", size.w, size.h, tt.Texts())
		}
		scroll, ok := tt.Find("Settings fields")
		if !ok {
			t.Fatalf("%dx%d: no fields scroller in %q", size.w, size.h, tt.Texts())
		}
		closeButton, ok := tt.Find("Settings Close")
		if !ok {
			t.Fatalf("%dx%d: no Close button in %q", size.w, size.h, tt.Texts())
		}
		done, ok := tt.Find("Settings Done")
		if !ok {
			t.Fatalf("%dx%d: no Done button in %q", size.w, size.h, tt.Texts())
		}
		if closeButton.X+closeButton.W > done.X {
			t.Fatalf("%dx%d: Close %v is not left of Done %v", size.w, size.h, closeButton, done)
		}
		if gap := done.X - closeButton.X - closeButton.W; gap < 0 || gap > 11 {
			t.Fatalf("%dx%d: Close and Done gap is %v, want the 10 DIP group", size.w, size.h, gap)
		}
		if dy := closeButton.Y + closeButton.H/2 - (done.Y + done.H/2); dy > 1 || dy < -1 {
			t.Fatalf("%dx%d: Close %v and Done %v are not on one row", size.w, size.h, closeButton, done)
		}
		if done.X+done.W > panel.X+panel.W-settingsPadding+2 {
			t.Fatalf("%dx%d: the button group %v runs past the panel %v", size.w, size.h, done, panel)
		}

		x, ok := tt.Find(locales.SettingsCloseIcon())
		if !ok {
			t.Fatalf("%dx%d: no close icon button in %q", size.w, size.h, tt.Texts())
		}
		if x.X < panel.X+panel.W-60 || x.X+x.W > panel.X+panel.W {
			t.Fatalf("%dx%d: close icon %v is not at the panel's right edge %v", size.w, size.h, x, panel)
		}
		if x.Y < panel.Y || x.Y < title.Y-4 || x.Y+x.H > title.Y+title.H+4 {
			t.Fatalf("%dx%d: close icon %v is not beside the title %v", size.w, size.h, x, title)
		}
		if x.Y+x.H > scroll.Y {
			t.Fatalf("%dx%d: close icon %v overlaps the fields scroller %v", size.w, size.h, x, scroll)
		}

		for _, entry := range []struct {
			name   string
			target string
		}{
			{"Close", "Settings Close"},
			{"close icon", locales.SettingsCloseIcon()},
			{"Done", "Settings Done"},
		} {
			s.saveErr = "pending probe"
			if err := tt.Click(entry.target); err != nil {
				t.Fatalf("%dx%d: %s: %v", size.w, size.h, entry.name, err)
			}
			tt.Frame()
			if s.SettingsOpen {
				t.Fatalf("%dx%d: %s left the settings dialog open", size.w, size.h, entry.name)
			}
			if s.saveErr != "" {
				t.Fatalf("%dx%d: %s left the save error %q", size.w, size.h, entry.name, s.saveErr)
			}
			s.OpenSettings()
			tt.Frame()
		}
	}
}

// TestSettingsCloseButtonsShot captures the footer group and the corner
// icon together for a visual check. Skipped outside PEEKDOWN_SHOT runs.
func TestSettingsCloseButtonsShot(t *testing.T) {
	if os.Getenv("PEEKDOWN_SHOT") == "" {
		t.Skip("set PEEKDOWN_SHOT=1 to write the settings close buttons screenshot")
	}
	s, _ := newShell(t)
	tt := s.renderSize(t, 1400, 880)
	s.OpenSettings()
	tt.Frame()
	writePng(t, tt.Image(), "C:/tmp/settings-close-buttons.png")
}
