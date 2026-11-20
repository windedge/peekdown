package view

import (
	"testing"
	"time"
)

// The flick up from mid-document rides its decay out: whatever stops it
// before the top is the bug.
func TestInertiaUpFromMiddle(t *testing.T) {
	m := rowsDoc(200)
	var st State
	tt := inertiaTester(t, &st, m)
	st.Scroll.Y = st.Scroll.MaxY / 2
	tt.Frame()
	st.Inertia.impulse(-1000, 1, st.Scroll.Y)
	tt.Frame()
	for i := range 30 {
		tt.Advance(time.Second / 60)
		if !st.Inertia.active() {
			t.Fatalf("the up glide died at frame %d, y=%v of %v, before the top", i, st.Scroll.Y, st.Scroll.MaxY)
		}
	}
}

// The flick up from the very bottom, over rows only estimated: their real
// heights come and go as they enter and leave the view, and each wave of
// them re-anchors the offset within a MaxY that moves with it. The glide
// must ride every re-anchor out, not read the reconciled place as a
// foreign move and stop mid-document.
func TestInertiaUpFromBottom(t *testing.T) {
	m := heteroDoc()
	var st State
	tt := inertiaTester(t, &st, m)
	st.List.ScrollToEnd()
	tt.Frame()
	if st.Scroll.MaxY <= 0 {
		t.Fatal("the document does not scroll")
	}
	st.Inertia.impulse(-1000, 1, st.Scroll.Y)
	tt.Frame()
	if !st.Inertia.active() {
		t.Fatal("the glide died before its first full frame")
	}
	for i := range 40 {
		tt.Advance(time.Second / 60)
		if !st.Inertia.active() {
			t.Fatalf("the up glide from the bottom died at frame %d, y=%v of %v", i, st.Scroll.Y, st.Scroll.MaxY)
		}
	}
	t.Logf("rode out: y=%v of %v", st.Scroll.Y, st.Scroll.MaxY)
}

// The exemption's diff is a subtraction of two float32s that the re-anchor
// made mathematically equal; rounding may leave the offset's diff a hair
// over the MaxY's. A slack of one DIP forgives that and nothing wider:
// a real move past the content's own move still ends the glide.
func TestInertiaDriftExemptionSlack(t *testing.T) {
	var st State
	st.Scroll.Y, st.Scroll.MaxY = 5545.7344, 8890.18
	st.Inertia.wroteY, st.Inertia.wroteMaxY = 5545.7344, 8890.18
	st.Inertia.Velocity = -2000
	// The re-anchor: both move by 767.45, the sum rounding differently.
	st.Scroll.Y, st.Scroll.MaxY = st.Scroll.Y+767.45, st.Scroll.MaxY+767.45
	st.glide(nil)
	if !st.Inertia.active() {
		t.Fatal("the glide stopped on a re-anchor its own slack explains")
	}
}

// The sign matrix: a re-anchor moves the offset and the content's end the
// same way (the end's move carries the offset with it, by clamping or by
// the rows' own places), so the same-sign pairs at every size ride, and
// the opposite-sign pairs, which only a foreign writer produces, stop.
func TestInertiaDriftExemptionSignMatrix(t *testing.T) {
	rides := []struct{ dy, dm float32 }{
		{40, 60}, {40, 40}, {-40, -60}, {-40, -40},
	}
	for _, c := range rides {
		var st State
		st.Scroll.Y, st.Scroll.MaxY = 500, 1000
		st.Inertia.wroteY, st.Inertia.wroteMaxY = 500, 1000
		st.Inertia.Velocity = -1000
		st.Scroll.Y += c.dy
		st.Scroll.MaxY += c.dm
		st.glide(nil)
		if !st.Inertia.active() {
			t.Errorf("dy=%+v dm=%+v: the glide stopped on a reconciled move", c.dy, c.dm)
		}
	}
	stops := []struct{ dy, dm float32 }{
		{40, -60}, {-40, 40}, {60, 40}, {-60, -40},
	}
	for _, c := range stops {
		var st State
		st.Scroll.Y, st.Scroll.MaxY = 500, 1000
		st.Inertia.wroteY, st.Inertia.wroteMaxY = 500, 1000
		st.Inertia.Velocity = -1000
		st.Scroll.Y += c.dy
		st.Scroll.MaxY += c.dm
		st.glide(nil)
		if st.Inertia.active() {
			t.Errorf("dy=%+v dm=%+v: the glide rode a foreign move", c.dy, c.dm)
		}
	}
}

// A move beyond what the MaxY's own move explains is still foreign: the
// thumb drag, the jump, the keys. The slack may not open the door to them.
func TestInertiaDriftExemptionRejectsOversizedDrift(t *testing.T) {
	cells := []struct{ dy, dm float32 }{
		{60, 40}, {60, -40}, {-60, 40}, {-60, -40}, {100, 0}, {-100, 0},
	}
	for _, c := range cells {
		var st State
		st.Scroll.Y, st.Scroll.MaxY = 500, 1000
		st.Inertia.wroteY, st.Inertia.wroteMaxY = 500, 1000
		st.Inertia.Velocity = -1000
		st.Scroll.Y += c.dy
		st.Scroll.MaxY += c.dm
		st.glide(nil)
		if st.Inertia.active() {
			t.Errorf("dy=%+v dm=%+v: the glide rode a foreign move", c.dy, c.dm)
		}
	}
}
