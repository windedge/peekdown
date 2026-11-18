package view

import (
	"testing"
	"time"
)

// A jump into a band of rows only estimated, then a flick up: as the rows
// above get measured the layout re-anchors the offset within a MaxY that
// grows with it, which the glide must follow rather than read as a foreign
// move and stop.
func TestInertiaFollowsEstimateDrift(t *testing.T) {
	m := heteroDoc()
	var st State
	tt := inertiaTester(t, &st, m)
	st.JumpToBlock(100)
	tt.Frame()
	tt.Advance(200 * time.Millisecond)
	st.Inertia.impulse(-1000, 1, st.Scroll.Y)
	tt.Frame()
	drifted, stopped := false, false
	for i := range 90 {
		tt.Advance(time.Second / 60)
		if !st.Inertia.active() {
			stopped = true
			if !drifted {
				t.Fatalf("glide stopped at frame %d, y=%v wroteY=%v, before any drift", i, st.Scroll.Y, st.Inertia.wroteY)
			}
			break
		}
		if !drifted && st.Scroll.Y != st.Inertia.wroteY {
			drifted = true
		}
	}
	if !drifted && !stopped {
		t.Skip("this run's frames measured nothing new; the drift path never ran")
	}
	if !stopped {
		t.Logf("glide ran to the natural decay, y=%v of %v", st.Scroll.Y, st.Scroll.MaxY)
	}
}

// A move that no layout reconciliation explains still ends the glide, as a
// jump or a thumb drag: the drift exemption must not swallow real takeovers.
func TestInertiaStillStopsOnUnexplainedMove(t *testing.T) {
	m := rowsDoc(200)
	var st State
	tt := inertiaTester(t, &st, m)
	st.Inertia.impulse(500, 1, st.Scroll.Y)
	tt.Frame()
	if !st.Inertia.active() {
		t.Fatal("the glide stopped before the move under test")
	}
	// 100 up while MaxY holds: no reconciliation explains it.
	st.Scroll.Y -= 100
	tt.Frame()
	if st.Inertia.Velocity != 0 {
		t.Fatalf("velocity after a foreign move = %v, want 0", st.Inertia.Velocity)
	}
}
