package shell

import (
	"os"
	"testing"

	"github.com/windedge/peekdown/locales"
)

// TestSettingsCloseEntryPoints checks the panel's single primary Close
// button and the icon at its top-right corner. Both only close the dialog.
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
		if _, ok := tt.Find("Settings Done"); ok {
			t.Fatalf("%dx%d: the removed Done button is still drawn in %q", size.w, size.h, tt.Texts())
		}
		if _, ok := tt.Find("Done"); ok {
			t.Fatalf("%dx%d: Done text is still drawn in %q", size.w, size.h, tt.Texts())
		}
		if got := pixelAt(tt.Image(), closeButton.X+6, closeButton.Y+closeButton.H/2); !channelClose(got, windowLight().Accent, 1) {
			t.Fatalf("%dx%d: Close button face %v, want accent %v", size.w, size.h, got, windowLight().Accent)
		}
		if closeButton.Y < title.Y || closeButton.Y+closeButton.H > panel.Y+panel.H+1 {
			t.Fatalf("%dx%d: Close %v is not the panel's footer %v", size.w, size.h, closeButton, panel)
		}
		if closeButton.X+closeButton.W > panel.X+panel.W-settingsPadding+2 {
			t.Fatalf("%dx%d: the Close button %v runs past the panel %v", size.w, size.h, closeButton, panel)
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

// TestSettingsCloseButtonsShot captures the footer button and the corner
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

// TestSettingsOneCloseShot captures the Appearance page with its single
// primary Close button at the panel's foot.
func TestSettingsOneCloseShot(t *testing.T) {
	if os.Getenv("PEEKDOWN_SHOT") == "" {
		t.Skip("set PEEKDOWN_SHOT=1 to write the settings one-close screenshot")
	}
	s, _ := newShell(t)
	tt := s.renderSize(t, 1400, 880)
	s.OpenSettings()
	tt.Frame()
	writePng(t, tt.Image(), "C:/tmp/settings-one-close.png")
}

// TestSettingsNavRegularShot captures the rail after its page names lost
// their bold weight, for a visual check of the selected row.
func TestSettingsNavRegularShot(t *testing.T) {
	if os.Getenv("PEEKDOWN_SHOT") == "" {
		t.Skip("set PEEKDOWN_SHOT=1 to write the settings nav screenshot")
	}
	s, _ := newShell(t)
	tt := s.renderSize(t, 1400, 880)
	s.OpenSettings()
	tt.Frame()
	writePng(t, tt.Image(), "C:/tmp/settings-nav-regular.png")
}
