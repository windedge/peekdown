package shell

import (
	"image"
	"image/draw"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/testshot"
)

// tabAccessRects reads the outer frame of both tabs from the accessibility
// tree: unlike Find, which gives the label's box, these frames are what
// moves when a tab's width changes.
func tabAccessRects(t *testing.T, tt *ui.Tester, names ...string) map[string]ui.Rect {
	t.Helper()
	want := make(map[string]bool, len(names))
	for _, name := range names {
		want[name] = true
	}
	rects := make(map[string]ui.Rect, len(names))
	for _, a := range tt.AccessAll() {
		if a.Role != ui.RoleTab {
			continue
		}
		for name := range want {
			if !strings.Contains(a.Name, name) {
				continue
			}
			if _, dup := rects[name]; dup {
				t.Fatalf("two tabs named %q", name)
			}
			rects[name] = a.Bounds
		}
	}
	for name := range want {
		if rects[name].W == 0 {
			t.Fatalf("no tab named %q in %v", name, tt.Texts())
		}
	}
	return rects
}

func assertTabRects(t *testing.T, stage string, got, want map[string]ui.Rect) {
	t.Helper()
	for name, before := range want {
		after := got[name]
		if math.Abs(float64(after.X-before.X)) > 0.5 || math.Abs(float64(after.W-before.W)) > 0.5 {
			t.Errorf("%s: %q tab moved from %v to %v", stage, name, before, after)
		}
	}
}

// openWidthTabs opens two differently named documents with the second
// active, ready for the state-change checks.
func openWidthTabs(t *testing.T) (*Shell, string, string) {
	t.Helper()
	s, dir := newShell(t)
	first, second := filepath.Join(dir, "a-longer-name.md"), filepath.Join(dir, "b.md")
	for p, src := range map[string]string{first: "# a\n", second: "# b\n"} {
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	s.OpenPaths([]string{first, second})
	return s, first, second
}

// TestTabWidthStableAcrossStates keeps both tabs' outer frames fixed while
// the choice and the pointer move: a wider active label used to shove the
// whole strip sideways on every click.
func TestTabWidthStableAcrossStates(t *testing.T) {
	s, first, second := openWidthTabs(t)
	tt := s.renderSize(t, 900, 600)
	tt.SetTitleBar(ui.TitleBar{Height: TitleBarHeight})
	tt.Frame()
	tt.Move(880, 580)
	tt.Frame()

	names := []string{filepath.Base(first), filepath.Base(second)}
	base := tabAccessRects(t, tt, names...)
	img := tt.Image()
	labelA, labelB := tabRect(t, tt, filepath.Base(first)), tabRect(t, tt, filepath.Base(second))
	baseActive := tabChipSample(img, labelB)
	baseIdle := tabChipSample(img, labelA)
	if baseActive == baseIdle {
		t.Fatal("the active tab has no background highlight to distinguish it")
	}

	if err := tt.Click(filepath.Base(first)); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Move(880, 580)
	tt.Frame()
	assertTabRects(t, "switch to first", tabAccessRects(t, tt, names...), base)
	if got := tabChipSample(tt.Image(), labelA); got != baseActive {
		t.Errorf("after switching the active chip is %v, want %v", got, baseActive)
	}

	// Hovering the active tab must not move it either.
	activeTab := tabAccessRects(t, tt, names...)[filepath.Base(first)]
	tt.Move(activeTab.X+activeTab.W/2, activeTab.Y+activeTab.H/2)
	tt.Frame()
	assertTabRects(t, "hover active tab", tabAccessRects(t, tt, names...), base)

	// Hovering an idle tab must only repaint it.
	tt.Move(880, 580)
	tt.Frame()
	hoverTab := tabAccessRects(t, tt, names...)[filepath.Base(second)]
	tt.Move(hoverTab.X+hoverTab.W/2, hoverTab.Y+hoverTab.H/2)
	tt.Frame()
	assertTabRects(t, "hover idle tab", tabAccessRects(t, tt, names...), base)
	hovered := tabChipSample(tt.Image(), labelB)
	if hovered == baseIdle {
		t.Error("hovering an idle tab does not change its background")
	}

	if err := tt.Click(filepath.Base(second)); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Move(880, 580)
	tt.Frame()
	assertTabRects(t, "switch back", tabAccessRects(t, tt, names...), base)
	if got := tabChipSample(tt.Image(), labelB); got != baseActive {
		t.Errorf("after switching back the active chip is %v, want %v", got, baseActive)
	}
}

// tabInk counts the dark pixels inside a tab's label. A heavier weight
// lays down more glyph coverage, so the same name in the same box has
// more ink when its tab is active.
func tabInk(t *testing.T, tt *ui.Tester, label string, threshold int) int {
	t.Helper()
	r := tabRect(t, tt, label)
	img := tt.Image()
	n := 0
	for y := int(r.Y); y < int(r.Y+r.H); y++ {
		for x := int(r.X); x < int(r.X+r.W); x++ {
			c := pixelAt(img, float32(x), float32(y))
			if int(c.R)+int(c.G)+int(c.B) < 3*threshold {
				n++
			}
		}
	}
	return n
}

// TestActiveTabLabelIsBolder checks the weight itself, not just the chip:
// with the label's box reserved at the active advance, switching state
// must put visibly more glyph ink in that same box.
func TestActiveTabLabelIsBolder(t *testing.T) {
	s, first, _ := openWidthTabs(t)
	// Keep both states the same color so the ink change can only come
	// from the label's weight, not the active state's text token.
	th := windowLight()
	th.Text = th.TextMuted
	tt := renderTheme(t, s, th, 900, 600)
	tt.SetTitleBar(ui.TitleBar{Height: TitleBarHeight})
	tt.Frame()
	tt.Move(880, 580)
	tt.Frame()

	name := filepath.Base(first)
	const threshold = 190
	idle := tabInk(t, tt, name, threshold)
	if err := tt.Click(name); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Move(880, 580)
	tt.Frame()
	active := tabInk(t, tt, name, threshold)
	t.Logf("label ink: idle=%d active=%d", idle, active)
	if idle == 0 {
		t.Fatal("the idle label has no ink to compare")
	}
	if active*100 < idle*105 {
		t.Errorf("active label is not bolder: idle=%d active=%d", idle, active)
	}
}

// TestTabWidthShot draws the bar with each tab active for a visual check
// that their edges line up. Skipped unless PEEKDOWN_SHOT is set.
func TestTabWidthShot(t *testing.T) {
	if os.Getenv("PEEKDOWN_SHOT") == "" {
		t.Skip("set PEEKDOWN_SHOT=1 to write the tab width screenshots")
	}
	s, first, second := openWidthTabs(t)
	tt := s.renderSize(t, 1440, 880)
	tt.SetTitleBar(ui.TitleBar{Height: TitleBarHeight, Right: 138})
	tt.Frame()
	tt.Move(1420, 860)
	tt.Frame()

	names := []string{filepath.Base(first), filepath.Base(second)}
	rects := tabAccessRects(t, tt, names...)
	bar, ok := tt.Find("Title bar")
	if !ok {
		t.Fatalf("texts %v: want the title bar", tt.Texts())
	}
	x := int(rects[names[0]].X) - 12
	w := int(rects[names[1]].X+rects[names[1]].W) - x + 12
	h := int(bar.H)

	writeBar := func(path string) image.Image {
		t.Helper()
		testshot.WritePng(t, tt.Image(), path)
		return testshot.Crop(tt.Image(), x, 0, w, h)
	}
	secondBar := writeBar("C:/tmp/tabs-active-2.png")
	if err := tt.Click(filepath.Base(first)); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Move(1420, 860)
	tt.Frame()
	firstBar := writeBar("C:/tmp/tabs-active-1.png")

	stack := image.NewRGBA(image.Rect(0, 0, w, 2*h))
	draw.Draw(stack, image.Rect(0, 0, w, h), firstBar, image.Pt(0, 0), draw.Src)
	draw.Draw(stack, image.Rect(0, h, w, 2*h), secondBar, image.Pt(0, 0), draw.Src)
	testshot.WritePng(t, testshot.Scale3x(stack), "C:/tmp/tabs-active-zoom.png")
}
