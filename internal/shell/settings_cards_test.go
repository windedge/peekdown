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
	if panel.H != settingsMaxHeight {
		t.Fatalf("panel height %v, want the fixed %v", panel.H, settingsMaxHeight)
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

	px, py := panel.X+10, panel.Y+10
	panelFace := over(k.CardFocused, over(k.Backdrop, pixelAt(bare.Image(), px, py)))
	probe("panel", px, py, panelFace)

	rail, ok := tt.Find("Settings rail")
	if !ok {
		t.Fatalf("no settings rail in %q", tt.Texts())
	}
	rx, ry := rail.X+rail.W/2, rail.Y+rail.H-8
	railFace := over(k.Card, over(k.CardFocused, over(k.Backdrop, pixelAt(bare.Image(), rx, ry))))
	probe("rail", rx, ry, railFace)

	card, ok := tt.Find("Appearance card")
	if !ok {
		t.Fatalf("no Appearance card in %q", tt.Texts())
	}
	cx, cy := card.X+4, card.Y+card.H/2
	probe("section card", cx, cy, over(k.Card, over(k.CardFocused, over(k.Backdrop, pixelAt(bare.Image(), cx, cy)))))

	closeButton, ok := tt.Find("Settings Close")
	if !ok {
		t.Fatalf("no Close button box in %q", tt.Texts())
	}
	if got := pixelAt(tt.Image(), closeButton.X+6, closeButton.Y+closeButton.H/2); !channelClose(got, windowLight().Accent, 1) {
		t.Fatalf("Close button face %v, want accent %v", got, windowLight().Accent)
	}
}

// TestSettingsBackdropClickCloses checks the DialogBase scrim is live:
// a press outside the panel dismisses the dialog and keeps the edits.
func TestSettingsBackdropClickCloses(t *testing.T) {
	s, _ := newShell(t)
	tt := s.renderSize(t, 1400, 880)
	s.OpenSettings()
	tt.Frame()
	if _, ok := tt.Find("Settings panel"); !ok {
		t.Fatalf("no settings panel in %q", tt.Texts())
	}
	tt.ClickAt(10, 10)
	tt.Frame()
	if s.SettingsOpen {
		t.Fatal("the backdrop click left the settings dialog open")
	}
}
