package view

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/doc"
)

// rowsDoc is n numbered paragraphs: long enough to glide through a 400x300
// window, and its rows named so a test can see where one landed.
func rowsDoc(n int) *doc.Model {
	var b strings.Builder
	for i := range n {
		fmt.Fprintf(&b, "Row number %03d of the filler text.\n\n", i)
	}
	return doc.New(b.String())
}

// inertiaTester renders a view of m and settles it, so the scroll state
// carries a laid-out MaxY before a test starts a glide.
func inertiaTester(t *testing.T, st *State, m *doc.Model) *ui.Tester {
	t.Helper()
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		ScrollView(c, m, st)
	}, 400, 300)
	tt.Frame()
	return tt
}

// TestWheelImpulseAccumulates checks the add_impulse arithmetic against the
// Rust constants: the same direction adds, the opposite one keeps 30% of
// the old velocity, the sum clamps at the maximum, and the multiplier rides
// the impulse alone.
func TestWheelImpulseAccumulates(t *testing.T) {
	var in InertiaScroll
	in.impulse(120, 1, 0)
	if want := float32(120 * 8); !near(in.Velocity, want) {
		t.Fatalf("velocity after one notch = %v, want %v", in.Velocity, want)
	}
	in.impulse(120, 1, 0)
	if want := float32(2 * 120 * 8); !near(in.Velocity, want) {
		t.Fatalf("velocity after two notches = %v, want %v", in.Velocity, want)
	}
	// The reversal keeps 30% of the 1920 it had, then takes the -960.
	in.impulse(-120, 1, 0)
	if want := float32(1920*0.3 - 960); !near(in.Velocity, want) {
		t.Fatalf("velocity after a reverse notch = %v, want %v", in.Velocity, want)
	}
	var fast InertiaScroll
	for range 10 {
		fast.impulse(200, 1, 0)
	}
	if !near(fast.Velocity, inertiaMaxVelocity) {
		t.Fatalf("velocity after ten fast notches = %v, want the clamp %v", fast.Velocity, inertiaMaxVelocity)
	}
	var faster InertiaScroll
	faster.impulse(120, 2, 0)
	if want := float32(120 * 8 * 2); !near(faster.Velocity, want) {
		t.Fatalf("velocity at speed 2 = %v, want %v", faster.Velocity, want)
	}
}

// TestInertiaFirstStepIsOneFrame is the regression for the first step's
// clock: a zero last time is one frame, not the zero time gap capped to
// 0.1s, which would land six frames' travel on the first step.
func TestInertiaFirstStepIsOneFrame(t *testing.T) {
	m := rowsDoc(200)
	var st State
	tt := inertiaTester(t, &st, m)
	// 160 DIP/s decays past the 8000 DIP/s linear rate after this step, so
	// the glide lands exactly one step and stops: the offset is its travel.
	st.Inertia.impulse(20, 1, st.Scroll.Y)
	if !st.Inertia.last.IsZero() {
		t.Fatal("an impulse from rest kept a clock from an earlier glide")
	}
	tt.Frame()
	if want := float32(160.0 / 60); !near(st.Scroll.Y, want) {
		t.Fatalf("scroll y after the first step = %v, want one frame's %v", st.Scroll.Y, want)
	}
}

// TestInertiaDecaysExponentially drives the smooth branch, reachable only
// below speed 1: the threshold is SMOOTH_THRESHOLD times the speed, so at a
// speed of 1 or more an impulse at MAX_VELOCITY stays under it.
func TestInertiaDecaysExponentially(t *testing.T) {
	in := InertiaScroll{Velocity: inertiaMaxVelocity}
	dt := time.Second / 60
	in.step(dt, 0.5)
	want := float64(inertiaMaxVelocity) * math.Pow(0.94, dt.Seconds()*60)
	if diff := math.Abs(float64(in.Velocity) - want); diff/math.Abs(want) > 0.01 {
		t.Fatalf("velocity after one step = %v, want 0.94^1 of it %v", in.Velocity, want)
	}
}

// TestInertiaStopsBelowMinVelocity checks the floor: a velocity under
// MIN_VELOCITY is zeroed and the step lands nothing, so the page does not
// creep for frames.
func TestInertiaStopsBelowMinVelocity(t *testing.T) {
	in := InertiaScroll{Velocity: 20}
	if got := in.step(time.Second/60, 1); got != 0 {
		t.Fatalf("step distance = %v, want none below the floor", got)
	}
	if in.Velocity != 0 || in.active() {
		t.Fatalf("velocity = %v, want it stopped", in.Velocity)
	}
}

// TestInertiaStopsAtBounds flicks hard to the end and checks the glide
// lands there and ends: the edge holds the view, so nothing more of the
// glide may land.
func TestInertiaStopsAtBounds(t *testing.T) {
	m := rowsDoc(60)
	var st State
	tt := inertiaTester(t, &st, m)
	if st.Scroll.MaxY <= 0 {
		t.Fatal("the document does not scroll")
	}
	st.Inertia.impulse(1000, 1, st.Scroll.Y)
	tt.Frame()
	tt.Advance(2 * time.Second)
	if !near(st.Scroll.Y, st.Scroll.MaxY) {
		t.Fatalf("scroll y = %v, want the end %v", st.Scroll.Y, st.Scroll.MaxY)
	}
	if st.Inertia.Velocity != 0 {
		t.Fatalf("velocity at the end = %v, want 0", st.Inertia.Velocity)
	}
}

// TestInertiaDisabledFallsThrough checks the setting off: the handler takes
// the notch and lands it at once, the instant ride the setting asks for, and
// no velocity builds up to glide with. Design §3.2 passes NoInertia through
// to the engine, which glides every notch it is handed: that would be the
// glide the setting turns off, so the instant branch takes it instead.
func TestInertiaDisabledFallsThrough(t *testing.T) {
	var st State
	st.NoInertia = true
	st.Scroll.Y, st.Scroll.MaxY = 100, 1000
	h := st.Handler(doc.New("Body\n"))
	if !h(ui.InputEvent{Kind: ui.InputScroll, DY: 120}) {
		t.Fatal("the handler left the notch to the engine, the setting asks for an instant move")
	}
	if !near(st.Scroll.Y, 220) {
		t.Fatalf("scroll y = %v, want the notch's 220", st.Scroll.Y)
	}
	if st.Inertia.Velocity != 0 {
		t.Fatalf("velocity = %v, want none with inertia off", st.Inertia.Velocity)
	}
}

// TestPreciseScrollBypassesInertia checks the touchpad's own drags: the
// engine glides them, so the handler leaves them alone and lands no
// velocity.
func TestPreciseScrollBypassesInertia(t *testing.T) {
	var st State
	st.Scroll.Y, st.Scroll.MaxY = 100, 1000
	h := st.Handler(doc.New("Body\n"))
	for _, ev := range []ui.InputEvent{
		{Kind: ui.InputScroll, DY: 12, Precise: true},
		{Kind: ui.InputScroll, DY: 12, Momentum: true},
	} {
		if h(ev) {
			t.Fatalf("the handler took a %+v event the platform should glide", ev)
		}
		if st.Inertia.Velocity != 0 {
			t.Fatalf("velocity = %v, want none for a touchpad drag", st.Inertia.Velocity)
		}
		if st.Scroll.Y != 100 {
			t.Fatalf("scroll y = %v, want the drag untouched at 100", st.Scroll.Y)
		}
	}
}

// TestShiftWheelAndSidewaysPassThrough keeps the body's own sideways
// scrolls: a code block and a wide table scroll on DX, and Shift turns the
// wheel sideways, so neither may become vertical velocity.
func TestShiftWheelAndSidewaysPassThrough(t *testing.T) {
	var st State
	st.Scroll.Y, st.Scroll.MaxY = 100, 1000
	h := st.Handler(doc.New("Body\n"))
	for _, ev := range []ui.InputEvent{
		{Kind: ui.InputScroll, DY: 120, DX: 60},
		{Kind: ui.InputScroll, DY: 120, Mods: ui.Shift},
	} {
		if h(ev) {
			t.Fatalf("the handler took a sideways %+v event", ev)
		}
		if st.Inertia.Velocity != 0 {
			t.Fatalf("velocity = %v, want none for a sideways notch", st.Inertia.Velocity)
		}
	}
}

// TestInertiaStopsOnExternalScroll checks the diff against the offset: a
// List command that moves the view ends the glide rather than fighting it.
func TestInertiaStopsOnExternalScroll(t *testing.T) {
	m := rowsDoc(200)
	var st State
	tt := inertiaTester(t, &st, m)
	st.Inertia.impulse(500, 1, st.Scroll.Y)
	tt.Frame()
	if !st.Inertia.active() {
		t.Fatal("the glide stopped before the jump under test")
	}
	st.List.ScrollToEnd()
	tt.Frame()
	if st.Inertia.Velocity != 0 {
		t.Fatalf("velocity after the jump = %v, want 0", st.Inertia.Velocity)
	}
}

// TestInertiaFirstFrameKeepsMaxYZero is the regression for a frame whose
// MaxY is still 0, as the first one is before the container laid the
// content out: clamping to it would pin the offset at the top and stop the
// glide dead.
func TestInertiaFirstFrameKeepsMaxYZero(t *testing.T) {
	m := rowsDoc(200)
	var st State
	tt := inertiaTester(t, &st, m)
	st.Scroll.MaxY = 0
	st.Inertia.impulse(120, 1, st.Scroll.Y)
	tt.Frame()
	if st.Scroll.Y <= 0 {
		t.Fatalf("scroll y = %v: the zero MaxY clamped and stopped the glide", st.Scroll.Y)
	}
	tt.Advance(50 * time.Millisecond)
	if st.Scroll.Y <= 16 {
		t.Fatalf("scroll y after the next frames = %v, want the glide to go on", st.Scroll.Y)
	}
}

// TestInertiaStopsOnForeignKeyMove is the diff's other half: a place the
// glide did not write ends it, whoever wrote it.
func TestInertiaStopsOnForeignKeyMove(t *testing.T) {
	m := rowsDoc(200)
	var st State
	tt := inertiaTester(t, &st, m)
	st.Inertia.impulse(500, 1, st.Scroll.Y)
	tt.Frame()
	if !st.Inertia.active() {
		t.Fatal("the glide stopped before the key move under test")
	}
	st.Scroll.Y += 100
	tt.Frame()
	if st.Inertia.Velocity != 0 {
		t.Fatalf("velocity after a foreign move = %v, want 0", st.Inertia.Velocity)
	}
}

// TestInertiaStopOnPreciseArrival checks a touchpad taking over mid-flick:
// the platform glides from there, so the flick ends when the drag arrives
// rather than keep writing the offset under it.
func TestInertiaStopOnPreciseArrival(t *testing.T) {
	m := rowsDoc(200)
	var st State
	tt := inertiaTester(t, &st, m)
	st.Inertia.impulse(500, 1, st.Scroll.Y)
	tt.Frame()
	if !st.Inertia.active() {
		t.Fatal("the glide stopped before the drag under test")
	}
	if st.Handler(m)(ui.InputEvent{Kind: ui.InputScroll, DY: 8, Precise: true}) {
		t.Fatal("the handler took the touchpad's drag")
	}
	if st.Inertia.Velocity != 0 {
		t.Fatalf("velocity after the drag arrived = %v, want 0", st.Inertia.Velocity)
	}
}

// TestInertiaFirstFlickFromNonZeroOffset is the regression for the fields a
// glide leaves behind: a document read at a place the inertia never wrote
// must not have its first flick read as a foreign move, and its first step
// is one frame, as the reset clock has it.
func TestInertiaFirstFlickFromNonZeroOffset(t *testing.T) {
	m := rowsDoc(200)
	var st State
	tt := inertiaTester(t, &st, m)
	st.Scroll.Y = 600
	tt.Frame()
	if !near(st.Scroll.Y, 600) {
		t.Fatalf("scroll y = %v, want the 600 set", st.Scroll.Y)
	}
	if st.Inertia.wroteY == 600 {
		t.Fatal("the inertia wrote a place it never stepped")
	}
	st.Inertia.impulse(120, 1, st.Scroll.Y)
	tt.Frame()
	if st.Scroll.Y <= 600 {
		t.Fatalf("scroll y = %v: the flick read the place it found as a foreign move", st.Scroll.Y)
	}
	// One frame's travel is 16 DIPs at 960 DIP/s; the 0.1s cap would land
	// 96 on the first step alone.
	if moved := st.Scroll.Y - 600; moved > 30 {
		t.Fatalf("the first step moved %v, more than the first clock's 16", moved)
	}
}
