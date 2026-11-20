package shell

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/locales"
)

func TestPaneSlideAdvancesToTarget(t *testing.T) {
	var a paneSlide
	a.init(0, time.Unix(0, 0))
	a.wants = true
	a.retarget(200)

	prev := a.value
	for _, dt := range []time.Duration{16 * time.Millisecond, 16 * time.Millisecond, 32 * time.Millisecond, 64 * time.Millisecond, 100 * time.Millisecond} {
		a.advance(dt)
		if a.value < prev || a.value > 200 {
			t.Fatalf("after %v: width %.3f, previous %.3f, want monotonic toward 200", dt, a.value, prev)
		}
		prev = a.value
	}
	if a.value != 200 || a.moving() {
		t.Fatalf("settled at %v, moving %v, want exactly 200 and at rest", a.value, a.moving())
	}
}

func TestPaneSlideReversesFromCurrentWidth(t *testing.T) {
	var a paneSlide
	a.init(0, time.Unix(0, 0))
	a.wants = true
	a.retarget(200)
	a.advance(70 * time.Millisecond)
	mid := a.value
	if mid <= 0 || mid >= 200 {
		t.Fatalf("mid-flight width = %v, want between 0 and 200", mid)
	}

	a.wants = false
	a.retarget(0)
	if a.value != mid || a.moving() != true {
		t.Fatalf("reverse start = %v moving %v, want %v and true", a.value, a.moving(), mid)
	}
	a.advance(20 * time.Millisecond)
	if a.value >= mid || a.value <= 0 {
		t.Fatalf("after reversing, width = %v, want below %v but above 0", a.value, mid)
	}
	a.advance(250 * time.Millisecond)
	if a.value != 0 || a.moving() {
		t.Fatalf("reverse end = %v moving %v, want 0 and at rest", a.value, a.moving())
	}

	a.wants = true
	a.retarget(200)
	a.advance(10 * time.Millisecond)
	if a.value <= 0 || a.value >= 200 {
		t.Fatalf("reopen width = %v, want between 0 and 200", a.value)
	}
}

// animatedShell keeps mygo's animations on: the regular renderSize helper
// calls DisableAnimations, which would settle the panel in the first frame.
func animatedShell(t *testing.T, w, h int, files, outline bool) (*Shell, *ui.Tester) {
	t.Helper()
	s, dir := newShell(t)
	if err := s.OpenFile(filepath.Join(dir, "a.md")); err != nil {
		t.Fatal(err)
	}
	s.Sidebar.Visible = files
	s.Outline.Visible = outline
	s.Saved.SidebarVisible = files
	s.Saved.OutlineVisible = outline
	tt := ui.NewTester(func(c *ui.Context) { s.View(c) }, w, h)
	tt.SetTitleBar(ui.TitleBar{Height: TitleBarHeight, Right: 138})
	tt.Frame()
	tt.Frame()
	return s, tt
}

func TestPanelOpeningSlidesAndSaves(t *testing.T) {
	s, tt := animatedShell(t, 1200, 700, false, false)
	if _, ok := tt.Find("Files card"); ok {
		t.Fatal("the file pane is present before opening")
	}
	if err := tt.Click(locales.BarFilesPanel()); err != nil {
		t.Fatal(err)
	}
	if !s.Sidebar.Visible || !s.Saved.SidebarVisible {
		t.Fatalf("visibility is not immediate: visible %v saved %v", s.Sidebar.Visible, s.Saved.SidebarVisible)
	}
	if s.Sidebar.Width != 200 {
		t.Fatalf("the persisted width changed on toggle: %v", s.Sidebar.Width)
	}

	var widths []float32
	for i := 0; i < 3; i++ {
		tt.Advance(40 * time.Millisecond)
		card, ok := tt.Find("Files card")
		if !ok {
			t.Fatalf("frame %d: file pane missing while opening", i+1)
		}
		if len(widths) > 0 && card.W <= widths[len(widths)-1] {
			t.Fatalf("frame %d width %v did not grow past %v", i+1, card.W, widths[len(widths)-1])
		}
		widths = append(widths, card.W)
	}
	tt.Advance(250 * time.Millisecond)
	final, ok := tt.Find("Files card")
	if !ok {
		t.Fatal("file pane missing after the animation settled")
	}
	if final.W <= widths[len(widths)-1] {
		t.Fatalf("final width %v did not grow past %v", final.W, widths[len(widths)-1])
	}
	if s.Sidebar.slide.value != s.Sidebar.Width || s.Sidebar.slide.moving() {
		t.Fatalf("slide width %v moving %v, want %v at rest", s.Sidebar.slide.value, s.Sidebar.slide.moving(), s.Sidebar.Width)
	}
	if s.Saved.SidebarWidth != float64(s.Sidebar.Width) {
		t.Fatalf("saved width %v, want %v", s.Saved.SidebarWidth, s.Sidebar.Width)
	}
}

func TestPanelClosingUnmountsOnlyAtTheEnd(t *testing.T) {
	s, tt := animatedShell(t, 1200, 700, true, false)
	start, ok := tt.Find("Files card")
	if !ok {
		t.Fatal("setup: file pane missing")
	}
	if err := tt.Click(locales.SidebarClose()); err != nil {
		t.Fatal(err)
	}
	if s.Sidebar.Visible || s.Saved.SidebarVisible {
		t.Fatalf("close is not immediate: visible %v saved %v", s.Sidebar.Visible, s.Saved.SidebarVisible)
	}

	tt.Advance(50 * time.Millisecond)
	card, ok := tt.Find("Files card")
	if !ok {
		t.Fatal("pane vanished before the closing animation ended")
	}
	if card.W >= start.W {
		t.Fatalf("closing width %v did not shrink from %v", card.W, start.W)
	}
	tt.Advance(250 * time.Millisecond)
	if _, ok := tt.Find("Files card"); ok {
		t.Fatal("pane is still rendered after the closing animation ended")
	}
	if s.Sidebar.slide.value != 0 || s.Sidebar.slide.moving() {
		t.Fatalf("closed slide = %v moving %v, want 0 at rest", s.Sidebar.slide.value, s.Sidebar.slide.moving())
	}
	if s.Saved.SidebarWidth != float64(s.Sidebar.Width) {
		t.Fatalf("saved width %v, want %v", s.Saved.SidebarWidth, s.Sidebar.Width)
	}
}

func TestPanelToggleReversesMidFlight(t *testing.T) {
	s, tt := animatedShell(t, 1200, 700, false, false)
	s.ToggleFiles()
	tt.Frame()
	tt.Advance(70 * time.Millisecond)
	mid := s.Sidebar.slide.value
	if mid <= 0 || mid >= s.Sidebar.Width {
		t.Fatalf("mid-flight width %v, want between 0 and %v", mid, s.Sidebar.Width)
	}

	s.ToggleFiles()
	tt.Frame()
	tt.Advance(20 * time.Millisecond)
	if got := s.Sidebar.slide.value; got >= mid || got <= 0 {
		t.Fatalf("reversed width %v, want below %v but above 0", got, mid)
	}
	tt.Advance(250 * time.Millisecond)
	if got := s.Sidebar.slide.value; got != 0 {
		t.Fatalf("reversed slide ended at %v, want 0", got)
	}

	s.ToggleFiles()
	tt.Frame()
	tt.Advance(250 * time.Millisecond)
	if got := s.Sidebar.slide.value; got != s.Sidebar.Width {
		t.Fatalf("reopened slide ended at %v, want %v", got, s.Sidebar.Width)
	}
}

func TestPanelsAnimateIndependently(t *testing.T) {
	s, tt := animatedShell(t, 1200, 700, false, false)
	s.ToggleFiles()
	s.ToggleOutline()
	tt.Frame()
	tt.Advance(50 * time.Millisecond)
	if got := s.Sidebar.slide.value; got <= 0 || got >= s.Sidebar.Width {
		t.Fatalf("file slide %v is not between 0 and %v", got, s.Sidebar.Width)
	}
	if got := s.Outline.slide.value; got <= 0 || got >= s.Outline.Width {
		t.Fatalf("outline slide %v is not between 0 and %v", got, s.Outline.Width)
	}
	if !s.Saved.SidebarVisible || !s.Saved.OutlineVisible {
		t.Fatal("both panels must save their open state")
	}

	s.ToggleOutline()
	tt.Frame()
	tt.Advance(250 * time.Millisecond)
	if s.Outline.slide.value != 0 {
		t.Fatalf("outline slide ended at %v after closing", s.Outline.slide.value)
	}
	if s.Sidebar.slide.value != s.Sidebar.Width {
		t.Fatalf("closing the outline disturbed the file slide: %v", s.Sidebar.slide.value)
	}
	if _, ok := tt.Find("Files card"); !ok {
		t.Fatal("file pane disappeared while its own state stayed open")
	}
}

func TestDragWaitsForPanelAnimation(t *testing.T) {
	s, tt := animatedShell(t, 1200, 700, true, false)
	start := s.Sidebar.Width
	s.ToggleFiles()
	tt.Frame()
	tt.Advance(50 * time.Millisecond)
	if !s.Sidebar.slide.moving() {
		t.Fatal("setup: closing animation is not running")
	}
	x := s.Sidebar.slide.value
	tt.Press(x, 300)
	tt.Frame()
	tt.Move(x+40, 300)
	tt.Frame()
	tt.Release(x+40, 300)
	tt.Frame()
	if s.Sidebar.Width != start {
		t.Fatalf("drag during animation changed width to %v, want %v", s.Sidebar.Width, start)
	}

	s.ToggleFiles()
	tt.Frame()
	tt.Advance(250 * time.Millisecond)
	tt.Press(start, 300)
	tt.Frame()
	tt.Move(start+40, 300)
	tt.Frame()
	tt.Release(start+40, 300)
	tt.Frame()
	if s.Sidebar.Width <= start {
		t.Fatalf("drag after settling did not widen the pane: %v, want above %v", s.Sidebar.Width, start)
	}
}

func TestPanelAnimationFramesShot(t *testing.T) {
	if os.Getenv("PEEKDOWN_SHOT") == "" {
		t.Skip("set PEEKDOWN_SHOT=1 to write the panel animation frames")
	}
	_, tt := animatedShell(t, 1200, 700, false, false)
	if err := tt.Click(locales.BarFilesPanel()); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 4; i++ {
		tt.Advance(40 * time.Millisecond)
		writePng(t, tt.Image(), "C:/tmp/anim-f"+string(rune('0'+i))+".png")
	}
}
