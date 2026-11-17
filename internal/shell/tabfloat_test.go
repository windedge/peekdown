package shell

import (
	"image"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

// tabRect returns the frame of the tab holding label.
func tabRect(t *testing.T, tt *ui.Tester, label string) ui.Rect {
	t.Helper()
	r, ok := tt.Find(label)
	if !ok {
		t.Fatalf("texts %v: want the %q tab", tt.Texts(), label)
	}
	return r
}

// tabChipSample reads the tab's left padding, past the badge that now
// precedes the label: the chip's background shows there and no glyph or
// close button does.
func tabChipSample(img *image.RGBA, label ui.Rect) ui.Color {
	return pixelAt(img, label.X-tabBadgeGap-tabBadgeSize-4, label.Y+label.H/2)
}

// renderTheme builds the shell's frame under a fixed theme: the tester
// has no system appearance to follow, so dark needs SetTheme.
func renderTheme(t *testing.T, s *Shell, theme *ui.Theme, width, height int) *ui.Tester {
	t.Helper()
	return ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		c.SetTheme(theme)
		s.View(c)
	}, width, height)
}

// openTwoTabs gives the shell two tabs, the last one active: the load is
// async, so a frame after the open waits it out.
func openTwoTabs(t *testing.T, s *Shell) {
	t.Helper()
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a.md"), filepath.Join(dir, "b.md")
	for p, src := range map[string]string{a: "# a\n", b: "# b\n"} {
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	s.OpenPaths([]string{a, b})
}

// TestActiveTabFloatsAboveBar pins the chip look in pixels: the bar is a
// tray of Surface, the active tab a chip of the window's Background
// floating on it with idle tabs melted into the tray, so the active one
// reads against both.
func TestActiveTabFloatsAboveBar(t *testing.T) {
	s, _ := newShell(t)
	openTwoTabs(t, s)
	tt := s.renderSize(t, 900, 600)
	tt.SetTitleBar(ui.TitleBar{Height: TitleBarHeight})
	tt.Frame()

	light := windowLight()
	img := tt.Image()
	active := tabChipSample(img, tabRect(t, tt, "b.md"))
	if active != light.Background {
		t.Fatalf("the active tab is %v, want the window's %v it floats on", active, light.Background)
	}
	// The idle tab's fill lands composited (A=255), so a token match
	// means the channels.
	idle := tabChipSample(img, tabRect(t, tt, "a.md"))
	if !channelClose(idle, light.Surface, 1) {
		t.Fatalf("the idle tab is %v, want the tray's %v it melts into", idle, light.Surface)
	}

	// The tray away from the tabs and the shadows they cast: past the
	// strip's right edge is the toolbar's room, so the sample is 8 DIPs
	// above the strip's bottom, clear of the tabs' rows.
	strip := tabRect(t, tt, "Tab bar")
	tray := pixelAt(img, strip.X+strip.W-3, strip.Y+strip.H-8)
	if !channelClose(tray, light.Surface, 1) {
		t.Fatalf("the bar is %v, want the tray's %v", tray, light.Surface)
	}
	if active == tray {
		t.Fatal("the active tab does not read against the bar's tray")
	}
}

// TestActiveTabFloatsOnDarkBar is the dark half of the chip look: a
// heavier shadow lifts the light chip off the dark tray, so the two still
// read apart.
func TestActiveTabFloatsOnDarkBar(t *testing.T) {
	s, _ := newShell(t)
	openTwoTabs(t, s)
	tt := renderTheme(t, s, ui.DarkTheme(), 900, 600)
	tt.SetTitleBar(ui.TitleBar{Height: TitleBarHeight})
	tt.Frame()

	dark := windowDark()
	img := tt.Image()
	active := tabChipSample(img, tabRect(t, tt, "b.md"))
	if active != dark.Background {
		t.Fatalf("the active tab is %v, want the window's %v it floats on", active, dark.Background)
	}
	idle := tabChipSample(img, tabRect(t, tt, "a.md"))
	if !channelClose(idle, dark.Surface, 1) {
		t.Fatalf("the idle tab is %v, want the tray's %v it melts into", idle, dark.Surface)
	}
	strip := tabRect(t, tt, "Tab bar")
	tray := pixelAt(img, strip.X+strip.W-3, strip.Y+strip.H-8)
	if !channelClose(tray, dark.Surface, 1) {
		t.Fatalf("the bar is %v, want the tray's %v", tray, dark.Surface)
	}
	if active == tray {
		t.Fatal("the active tab does not read against the dark bar's tray")
	}
}
