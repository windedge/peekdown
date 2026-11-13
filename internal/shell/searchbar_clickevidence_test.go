package shell

import (
	"testing"
)

// TestSearchToggleClickAtPoint clicks the regex toggle at its real
// coordinates, the way a physical click reaches it: move, press, release,
// while the field holds the focus with a query typed, instead of the
// label-dispatched tt.Click.
func TestSearchToggleClickAtPoint(t *testing.T) {
	s, tt, tab := searchShell(t)

	s.search.Query = "fi.*"
	s.runSearch(tab)
	tt.Frame()

	r, ok := tt.Find("Regular expression")
	if !ok {
		t.Fatalf("regex toggle not rendered: %v", tt.Texts())
	}
	cx, cy := r.X+r.W/2, r.Y+r.H/2
	tt.Move(cx, cy)
	tt.Press(cx, cy)
	tt.Frame()
	tt.Release(cx, cy)
	tt.Frame()

	if !s.search.Regex {
		t.Fatalf("pointer click at (%.0f, %.0f) did not turn the toggle on", cx, cy)
	}
	if tab.Model.Query() != "fi.*" {
		t.Fatalf("query changed to %q", tab.Model.Query())
	}
}
