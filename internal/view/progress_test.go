package view

import (
	"strconv"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/doc"
)

// progressDoc builds a document tall enough to scroll in a 400x300 view.
func progressDoc(blocks int) *doc.Model {
	var b strings.Builder
	b.WriteString("# Top\n\n")
	for i := range blocks {
		b.WriteString("Paragraph number ")
		b.WriteString(strconv.Itoa(i))
		b.WriteString(" of the filler body.\n\n")
	}
	return doc.New(b.String())
}

// The scroll place updates as the list scrolls: 0 at the top, 1 at the
// end, between the two in the middle, and a document with no scroll room
// reads as both ends with 0 progress, so the shell draws no bar.
func TestScrollPlace(t *testing.T) {
	m := progressDoc(40)
	var state State
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		ScrollView(c, m, &state)
	}, 400, 300)
	tt.Frame()
	if !state.AtTop || state.AtBottom {
		t.Fatalf("at the top: top=%v bottom=%v progress=%v", state.AtTop, state.AtBottom, state.Progress)
	}
	if state.Progress != 0 {
		t.Fatalf("progress at the top = %v, want 0", state.Progress)
	}

	// Partway down: the fraction follows, bounded in 0..1. The track
	// writes the offset back as the frame lays out, after the build read
	// it, so the settled value shows one frame later.
	state.List.ScrollTo(20, ui.Start)
	tt.Frame()
	tt.Frame()
	if state.Progress <= 0 || state.Progress >= 1 {
		t.Fatalf("progress partway = %v, want inside 0..1", state.Progress)
	}
	if state.AtTop || state.AtBottom {
		t.Fatalf("partway: top=%v bottom=%v", state.AtTop, state.AtBottom)
	}

	// The end: 1, and the bottom flag up.
	state.List.ScrollToEnd()
	tt.Frame()
	tt.Frame()
	if !state.AtBottom || state.AtTop {
		t.Fatalf("at the end: top=%v bottom=%v", state.AtTop, state.AtBottom)
	}
	if state.Progress < 0.999 {
		t.Fatalf("progress at the end = %v, want 1", state.Progress)
	}
}

func TestScrollPlaceShortDocument(t *testing.T) {
	m := doc.New("One short paragraph.\n")
	var state State
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		ScrollView(c, m, &state)
	}, 400, 300)
	tt.Frame()
	if state.Progress != 0 || !state.AtTop || !state.AtBottom {
		t.Fatalf("short document: progress=%v top=%v bottom=%v, want 0, true, true",
			state.Progress, state.AtTop, state.AtBottom)
	}
	if st := state.Scroll; st.MaxY != 0 {
		t.Fatalf("short document MaxY = %v, want 0 (no scroll room)", st.MaxY)
	}
}

// A linked image opens its link, through the same dispatch a text link
// takes; a bare image has no pointer affordance to read in headless, so
// the test drives the dispatch and the model instead.
func TestImageLinkOpens(t *testing.T) {
	m := doc.New("[![alt](pic.png)](https://example.com/page)\n")
	var state State
	var opened []string
	state.OpenURL = func(url string) { opened = append(opened, url) }
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		Render(c, m, &state, m.Blocks()[0].Index)
	}, 300, 200)

	state.openImageLink("https://example.com/page")
	if len(opened) != 1 || opened[0] != "https://example.com/page" {
		t.Fatalf("OpenURL got %v", opened)
	}
	state.openImageLink("javascript:alert(1)")
	if len(opened) != 1 {
		t.Fatalf("unsafe scheme opened %v", opened)
	}

	// The image block's model carries the link, which the renderer reads
	// for the cursor and the tooltip.
	images := m.Images(m.Blocks()[0].Index)
	if len(images) != 1 || images[0].Link != "https://example.com/page" {
		t.Fatalf("model images: %+v", images)
	}
	if tt.HasText("pic.png") {
		t.Fatal("the linked image fell back to its description")
	}
}

// A local (non-scheme) image link goes to OpenLocalLink, as a text link
// does; an anchor scrolls to its heading.
func TestImageLinkLocalAndAnchor(t *testing.T) {
	m := doc.New("# Head\n\n[![alt](pic.png)](other.md#head)\n")
	var state State
	var local []string
	state.OpenLocalLink = func(dest string) { local = append(local, dest) }
	state.AnchorOf = anchorsOf(m)
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		ScrollView(c, m, &state)
	}, 300, 200)

	state.openImageLink("other.md#head")
	if len(local) != 1 || local[0] != "other.md#head" {
		t.Fatalf("OpenLocalLink got %v", local)
	}

	state.openImageLink("#head")
	if state.ScrollTargetBlock != m.Blocks()[0].Index {
		t.Fatalf("anchor target block = %d, want %d", state.ScrollTargetBlock, m.Blocks()[0].Index)
	}
	_ = tt
}
