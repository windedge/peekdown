package shell

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/settings"
)

// fpsPattern matches the readout's text: the rate.
var fpsPattern = regexp.MustCompile(`^\d+ fps$`)

// feed samples n frames interval apart, the first a second and a half in the
// past so the window is full, and returns the last frame's time.
func feed(m *fpsMeter, interval time.Duration, n int) time.Time {
	now := time.Now().Add(-1500 * time.Millisecond)
	for range n {
		m.sample(now)
		now = now.Add(interval)
	}
	return now.Add(-interval)
}

// gap samples the frame at now+d, forcing the readout to update.
func gap(m *fpsMeter, now time.Time, d time.Duration) {
	m.published = time.Time{}
	m.sample(now.Add(d))
}

// TestFPSMeterSteady checks a steady 16ms run reads as the panel's rate and
// is not flagged.
func TestFPSMeterSteady(t *testing.T) {
	var m fpsMeter
	feed(&m, 16*time.Millisecond, 70)
	if m.stalled {
		t.Fatalf("worst %v flagged a steady 16ms run as stalled", m.worst)
	}
	if m.fps < 61 || m.fps > 64 {
		t.Fatalf("label %q, want 16ms read as about 62 fps", m.label())
	}
	if m.worst != 16*time.Millisecond {
		t.Fatalf("worst %v, want 16ms", m.worst)
	}
}

// TestFPSMeterStall checks one long gap in an otherwise steady run is
// flagged and printed with its length.
func TestFPSMeterStall(t *testing.T) {
	var m fpsMeter
	now := feed(&m, 16*time.Millisecond, 61)
	gap(&m, now, 100*time.Millisecond)
	if !m.stalled {
		t.Fatalf("worst %v: want the 100ms gap flagged", m.worst)
	}
	if m.worst != 100*time.Millisecond {
		t.Fatalf("worst %v, want the 100ms gap", m.worst)
	}
	if got, want := m.label(), "57 fps"; got != want {
		t.Fatalf("label %q, want %q", got, want)
	}
}

// TestFPSMeterStallThresholdFollowsPanel checks the threshold rides the
// window's median gap: 18ms is a dropped frame on a 144Hz panel and the
// frame's own interval, half of it, on a 50Hz one.
func TestFPSMeterStallThresholdFollowsPanel(t *testing.T) {
	var fast fpsMeter
	now := feed(&fast, 7*time.Millisecond, 200)
	gap(&fast, now, 18*time.Millisecond)
	if !fast.stalled {
		t.Fatalf("worst %v on a 7ms run: want 18ms flagged", fast.worst)
	}

	var slow fpsMeter
	now = feed(&slow, 20*time.Millisecond, 70)
	gap(&slow, now, 18*time.Millisecond)
	if slow.stalled {
		t.Fatalf("worst %v on a 20ms run: want 18ms unflagged", slow.worst)
	}
}

// TestFPSMeterResetForgetsWindow checks the window goes when the readout
// hides, so the time it was away is not a stall when it returns.
func TestFPSMeterResetForgetsWindow(t *testing.T) {
	var m fpsMeter
	now := feed(&m, 16*time.Millisecond, 61)
	gap(&m, now, 100*time.Millisecond)
	m.reset()
	if m.label() != "" || len(m.frames) != 0 || m.stalled {
		t.Fatalf("after reset %+v, want an empty meter", m)
	}
}

// TestFPSReadoutOff checks the status bar carries no readout with the switch
// off, and that nothing pulls frames for it.
func TestFPSReadoutOff(t *testing.T) {
	s, dir := newShell(t)
	if err := s.OpenFile(filepath.Join(dir, "a.md")); err != nil {
		t.Fatal(err)
	}
	tt := s.renderSize(t, 900, 600)
	tt.Frame()
	tt.Advance(2 * time.Second)
	if tt.FramePending() {
		t.Fatal("the switch is off but frames are still pulled")
	}
	if text, _, ok := findFPS(tt); ok {
		t.Fatalf("texts %v hold %q with the switch off", tt.Texts(), text)
	}
}

// TestFPSReadoutOn checks the switch draws a live readout in the status bar,
// at its left edge with the fixed room it keeps.
func TestFPSReadoutOn(t *testing.T) {
	s, dir := newShell(t)
	s.Saved.ShowFPS = true

	if err := s.OpenFile(filepath.Join(dir, "a.md")); err != nil {
		t.Fatal(err)
	}
	tt := s.renderSize(t, 900, 600)
	tt.Frame()
	// The box holds the readout's room before its first window fills: the
	// zoom label already sits where it stays.
	zoomBefore, ok := tt.Find("100%")
	if !ok {
		t.Fatalf("no zoom label in %v", tt.Texts())
	}
	tt.Advance(2 * time.Second)

	text, rect, ok := findFPS(tt)
	if !ok {
		t.Fatalf("texts %v: want a readout after two seconds", tt.Texts())
	}
	if s.fps.fps < 50 || s.fps.fps > 70 {
		t.Fatalf("readout %q measured %d fps at the tester's 60Hz", text, s.fps.fps)
	}
	if rect.W != fpsWidth {
		t.Fatalf("readout %q is %v wide, want the fixed %v", text, rect.W, fpsWidth)
	}
	zoom, ok := tt.Find("100%")
	if !ok {
		t.Fatalf("no zoom label in %v", tt.Texts())
	}
	if zoom.X != zoomBefore.X || zoom.Y != zoomBefore.Y {
		t.Fatalf("zoom label moved from %v to %v when the readout arrived", zoomBefore, zoom)
	}
	// The bar reads fps then zoom: the readout sits at the left edge, the
	// zoom readout at the right, with the empty room between them.
	if rect.X >= zoom.X {
		t.Fatalf("readout at %v is not left of the zoom label at %v", rect, zoom)
	}
	if rect.X > 20 {
		t.Fatalf("readout at %v is not at the bar's left edge", rect)
	}
	if zoom.X+zoom.W < 900-20 {
		t.Fatalf("zoom label at %v is not at the bar's right edge", zoom)
	}
}

// TestFPSReadoutStallColor checks a stalled reading keeps the bar's muted
// gray, no warning color.
func TestFPSReadoutStallColor(t *testing.T) {
	s, dir := newShell(t)
	s.Saved.ShowFPS = true

	if err := s.OpenFile(filepath.Join(dir, "a.md")); err != nil {
		t.Fatal(err)
	}
	tt := s.renderSize(t, 900, 600)
	tt.Frame()
	tt.Advance(2 * time.Second)
	_, rect, ok := findFPS(tt)
	if !ok {
		t.Fatalf("no readout in %v", tt.Texts())
	}
	if orange := orangePixels(tt, rect); orange != 0 {
		t.Fatalf("%d orange pixels in the steady readout at %v: want the muted gray", orange, rect)
	}

	// The last window found a stall, as one the tester's clock cannot show
	// would: the text stays until the next measurement is due, and the color
	// does not change.
	s.fps.text = "60 fps"
	s.fps.stalled = true
	s.fps.published = tt.Now()
	tt.Frame()
	_, rect, ok = findFPS(tt)
	if !ok {
		t.Fatalf("no stalled readout in %v", tt.Texts())
	}
	if orange := orangePixels(tt, rect); orange != 0 {
		t.Fatalf("%d orange pixels in the stalled readout at %v: want the muted gray", orange, rect)
	}
}

// TestShowFPSSettingsRoundTrip checks the switch survives settings.json, and
// that a first run has it off as the Rust build did.
func TestShowFPSSettingsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), settings.FileName)
	if settings.Default().ShowFPS {
		t.Fatal("a first run shows FPS: want it off")
	}
	on := settings.Default()
	on.ShowFPS = true
	if err := settings.Save(path, on); err != nil {
		t.Fatal(err)
	}
	loaded, err := settings.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.ShowFPS {
		t.Fatalf("%s lost show_fps", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`"show_fps": true`).Match(data) {
		t.Fatalf("%s has no show_fps key: %s", path, data)
	}
}

// findFPS returns the readout's text and its bounds.
func findFPS(tt *ui.Tester) (string, ui.Rect, bool) {
	for _, text := range tt.Texts() {
		if !fpsPattern.MatchString(text) {
			continue
		}
		if rect, ok := tt.Find(text); ok {
			return text, rect, true
		}
	}
	return "", ui.Rect{}, false
}

// orangePixels counts the pixels in rect that the warning color leaves: text
// drawn in it has orange cores, the muted gray text none.
func orangePixels(tt *ui.Tester, rect ui.Rect) int {
	img := tt.Image()
	b := img.Bounds()
	x0, y0 := max(int(rect.X), b.Min.X), max(int(rect.Y), b.Min.Y)
	x1, y1 := min(int(rect.X+rect.W), b.Max.X), min(int(rect.Y+rect.H), b.Max.Y)
	n := 0
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			c := img.RGBAAt(x, y)
			if int(c.R)-int(c.B) > 120 && int(c.R)-int(c.G) > 60 && c.B < 96 {
				n++
			}
		}
	}
	return n
}
