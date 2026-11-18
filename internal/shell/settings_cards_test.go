package shell

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/settings"
)

// settingsBareFrame is the same window without the dialog, so each
// translucent layer's expected color composites over the pixels it
// actually covers.
func settingsBareFrame(t *testing.T, w, h int) *ui.Tester {
	t.Helper()
	bareShell, _ := newShell(t)
	bare := bareShell.renderSize(t, w, h)
	bare.Frame()
	return bare
}

// TestSettingsDialogCardSurfaces checks the dialog's new card language:
// mojian's Backdrop dims the window, the panel is CardFocused inside a
// focused rim, and each section is a translucent Card on that panel.
func TestSettingsDialogCardSurfaces(t *testing.T) {
	const w, h = 1400, 880
	s, _ := newShell(t)
	tt := s.renderSize(t, w, h)
	bare := settingsBareFrame(t, w, h)
	s.OpenSettings()
	tt.Frame()

	k := accentThemeByID(settings.AccentThemeDefault).of(false)
	backdrop, ok := tt.Find("Settings backdrop")
	if !ok {
		t.Fatalf("no settings backdrop in %q", tt.Texts())
	}
	if backdrop.W != w || backdrop.H != h {
		t.Fatalf("backdrop %v does not cover the %dx%d window", backdrop, w, h)
	}
	panel, ok := tt.Find("Settings panel")
	if !ok {
		t.Fatalf("no settings panel in %q", tt.Texts())
	}
	if panel.W != settingsWidth {
		t.Fatalf("panel width %v, want %v", panel.W, settingsWidth)
	}

	probe := func(name string, x, y float32, want ui.Color) {
		t.Helper()
		if got := pixelAt(tt.Image(), x, y); !channelClose(got, want, 2) {
			t.Fatalf("%s pixel at %.0f,%.0f = %v, want %v", name, x, y, got, want)
		}
	}
	bx, by := float32(10), float32(h/2)
	dimmed := over(k.Backdrop, pixelAt(bare.Image(), bx, by))
	probe("backdrop", bx, by, dimmed)

	px, py := panel.X+10, panel.Y+panel.H/2
	panelFace := over(k.CardFocused, over(k.Backdrop, pixelAt(bare.Image(), px, py)))
	probe("panel", px, py, panelFace)

	card, ok := tt.Find("Appearance card")
	if !ok {
		t.Fatalf("no Appearance card in %q", tt.Texts())
	}
	cx, cy := card.X+4, card.Y+card.H/2
	probe("section card", cx, cy, over(k.Card, over(k.CardFocused, over(k.Backdrop, pixelAt(bare.Image(), cx, cy)))))

	done, ok := tt.Find("Settings Done")
	if !ok {
		t.Fatalf("no Done button box in %q", tt.Texts())
	}
	if got := pixelAt(tt.Image(), done.X+6, done.Y+done.H/2); !channelClose(got, windowLight().Accent, 1) {
		t.Fatalf("Done button face %v, want accent %v", got, windowLight().Accent)
	}
}

// TestSettingsBackdropClickCloses checks the DialogBase scrim is live:
// a press outside the panel dismisses the dialog and keeps the edits.
func TestSettingsBackdropClickCloses(t *testing.T) {
	s, _ := newShell(t)
	tt := s.renderSize(t, 1400, 880)
	s.OpenSettings()
	tt.Frame()
	panel, ok := tt.Find("Settings panel")
	if !ok {
		t.Fatalf("no settings panel in %q", tt.Texts())
	}
	tt.ClickAt(panel.X/2, panel.Y+panel.H/2)
	tt.Frame()
	if s.SettingsOpen {
		t.Fatal("the backdrop click left the settings dialog open")
	}
}
