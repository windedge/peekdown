package view

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// The link's inline element carries the destination as written: a web URL,
// a relative path, a Windows path and an anchor all keep their own text.
func TestLinkTooltipShowsDestinationAsWritten(t *testing.T) {
	for _, tc := range linkScenarios {
		t.Run(tc.name, func(t *testing.T) {
			for _, dest := range tc.want {
				_, tt, st, _, _ := localLinkDoc(t, tc.src, 420, 400)
				r := firstRect(t, st, dest)
				tt.Move(r.Rect.X+r.Rect.W/2, r.Rect.Y+r.Rect.H/2)
				tt.Advance(time.Second)
				if _, ok := tt.Find(dest); !ok {
					t.Fatalf("no tooltip %q for %q: texts %q", dest, dest, tt.Texts())
				}
				// The tip is the link's own, not the whole paragraph's.
				for _, a := range tt.AccessAll() {
					if a.Role == ui.RoleTooltip && a.Name != dest {
						t.Fatalf("tooltip %+v, want only %q", a, dest)
					}
				}
			}
		})
	}
}

// Prose around a link has no tip, and moving off the link takes its tip
// away: the tip belongs to the inline element, not to the row.
func TestProseHasNoLinkTooltip(t *testing.T) {
	src := "Read [the guide](https://example.com/guide) now.\n"
	_, tt, st, _, _ := localLinkDoc(t, src, 420, 300)
	r := firstRect(t, st, "https://example.com/guide")
	x, y := r.Rect.X+r.Rect.W/2, r.Rect.Y+r.Rect.H/2
	tt.Move(x, y)
	tt.Advance(time.Second)
	if _, ok := tt.Find("https://example.com/guide"); !ok {
		t.Fatalf("no tooltip over the link: texts %q", tt.Texts())
	}
	tt.Move(r.Rect.X+r.Rect.W+12, y)
	tt.Advance(time.Second)
	if _, ok := tt.Find("https://example.com/guide"); ok {
		t.Fatalf("the tip stayed over the prose: texts %q", tt.Texts())
	}
	for _, a := range tt.AccessAll() {
		if a.Role == ui.RoleTooltip {
			t.Fatalf("a tooltip over prose: %+v", a)
		}
	}
}

// A long destination keeps its head and tail, with an ellipsis between:
// the middle is the least useful part of both a URL and a local path.
func TestLinkTooltipTruncatesLongDestination(t *testing.T) {
	dest := "https://example.com/very/long/path/with/a/filename/that/keeps-going/" + strings.Repeat("x", 90) + "/tail.md"
	want := linkTip(dest)
	if want == dest {
		t.Fatal("a long destination was not cut")
	}
	if !strings.HasPrefix(want, "https://example.com/very/") || !strings.HasSuffix(want, "/tail.md") {
		t.Fatalf("tip %q lost the head or tail", want)
	}
	if !strings.Contains(want, "...") || len([]rune(want)) != 120 {
		t.Fatalf("tip %q is not a 120-rune middle cut", want)
	}
	_, tt, st, _, _ := localLinkDoc(t, "Read [the guide]("+dest+") now.\n", 420, 300)
	r := firstRect(t, st, dest)
	tt.Move(r.Rect.X+r.Rect.W/2, r.Rect.Y+r.Rect.H/2)
	tt.Advance(time.Second)
	if _, ok := tt.Find(want); !ok {
		t.Fatalf("no truncated tooltip %q: texts %q", want, tt.Texts())
	}
	if _, ok := tt.Find(dest); ok {
		t.Fatalf("the full long URL showed in a tooltip: %q", dest)
	}
}

func TestLinkTipLeavesShortDestinationsAlone(t *testing.T) {
	for _, dest := range []string{
		"notes.md",
		"C:/Seafile/Works/rust/peekdown",
		"#the-details",
		"mailto:team@example.com",
		strings.Repeat("x", 120),
	} {
		if got := linkTip(dest); got != dest {
			t.Fatalf("linkTip(%q) = %q", dest, got)
		}
	}
}
