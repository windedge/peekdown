package view

import (
	"strconv"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/doc"
)

// jumpDoc builds a document of 300 paragraphs with a heading every 100,
// long enough that a heading more than a viewport down needs a real jump.
func jumpDoc() *doc.Model {
	var b strings.Builder
	b.WriteString("# Top\n\n")
	for i := range 300 {
		if i%100 == 0 {
			b.WriteString("## Section ")
			b.WriteString(strconv.Itoa(i / 100))
			b.WriteString("\n\n")
		}
		b.WriteString("Paragraph number ")
		b.WriteString(strconv.Itoa(i))
		b.WriteString(" of the filler body.\n\n")
	}
	return doc.New(b.String())
}

// jumpView renders the document in a labelled viewport box: the label's
// committed bounds are the top the rows jump to.
func jumpView(t *testing.T, m *doc.Model, st *State) *ui.Tester {
	t.Helper()
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		ui.Box(c).Fill().Label("viewport").Children(func() {
			ScrollView(c, m, st)
		})
	}, 400, 300)
	tt.Frame()
	tt.Frame()
	return tt
}

// rowTop returns how far the block's window rect starts below the
// viewport's top.
func rowTop(t *testing.T, tt *ui.Tester, st *State, block int) float32 {
	t.Helper()
	viewport, ok := tt.Find("viewport")
	if !ok {
		t.Fatalf("no viewport box in %q", tt.Texts())
	}
	r, ok := st.RowRect(block)
	if !ok {
		t.Fatalf("block %d has no committed rect: it is not in view", block)
	}
	return r.Y - viewport.Y
}

// TestOutlineJumpLandsAtTop walks the outline's jump: a heading far
// below the viewport lands at the view's top, as a link to it should,
// rather than at the bottom edge a least-scroll reveal would leave it on.
func TestOutlineJumpLandsAtTop(t *testing.T) {
	m := jumpDoc()
	var st State
	tt := jumpView(t, m, &st)

	// The third section heading sits well past the first viewport.
	headings := m.Headings()
	if len(headings) < 4 {
		t.Fatalf("headings %v: the document lost its sections", headings)
	}
	target := headings[3].BlockIndex
	st.JumpToBlock(target)
	tt.Frame()
	tt.Frame()

	if top := rowTop(t, tt, &st, target); top < -8 || top > 24 {
		t.Fatalf("heading %d sits %v DIPs below the viewport top, want it at the top", target, top)
	}
	// The viewport is 300 DIPs tall, so a least-scroll reveal would have
	// parked the heading within one row of the bottom edge.
	if top := rowTop(t, tt, &st, target); top > 300-40 {
		t.Fatalf("heading %d sits %v DIPs below the top: the jump revealed it at the bottom", target, top)
	}
	// The section's body follows it, so the jump is not past the content.
	if !strings.Contains(strings.Join(tt.Texts(), " "), "Paragraph number 200") {
		t.Fatalf("texts %v: the section body did not come into view", tt.Texts())
	}
}

// TestAnchorJumpLandsAtTop covers the anchor path: a cross-document
// heading lands at the view's top too, not at the bottom edge.
func TestAnchorJumpLandsAtTop(t *testing.T) {
	m := jumpDoc()
	st := State{}
	tt := jumpView(t, m, &st)
	st.ResetPendingAnchor("section-2")
	tt.Frame()
	tt.Frame()

	block, ok := m.AnchorBySlug("section-2")
	if !ok {
		t.Fatal("the anchor was not in the document")
	}
	if st.Anchor != "section-2" {
		t.Fatalf("anchor %q, want the pending slug promoted", st.Anchor)
	}
	if top := rowTop(t, tt, &st, block); top < -8 || top > 24 {
		t.Fatalf("anchored heading %d sits %v DIPs below the viewport top, want it at the top", block, top)
	}
}

// TestSearchHitOnlyNeedsToBeInView pins the search path: a hit already
// on screen does not move the list, and a hit below it is revealed at the
// bottom edge, both the least-scroll the heading jumps replaced.
func TestSearchHitOnlyNeedsToBeInView(t *testing.T) {
	m := jumpDoc()
	var st State
	tt := jumpView(t, m, &st)
	st.List.ScrollTo(3, ui.Start)
	tt.Frame()
	tt.Frame()

	// A hit on screen leaves the view where it is, a few DIPs down: a
	// top alignment would have parked it on the viewport's edge.
	inView := m.Blocks()[4].Index
	st.ScrollTargetBlock = inView
	tt.Frame()
	tt.Frame()
	if top := rowTop(t, tt, &st, inView); top < 20 {
		t.Fatalf("an in-view hit sits %v DIPs below the viewport top: the search aligned it", top)
	}

	// A hit far below comes into view at the bottom edge.
	far := m.Blocks()[150].Index
	st.ScrollTargetBlock = far
	tt.Frame()
	tt.Frame()
	top := rowTop(t, tt, &st, far)
	if top < 0 || top > 300 {
		t.Fatalf("the far hit sits %v DIPs below the viewport top, want it in view", top)
	}
	if top < 300-80 {
		t.Fatalf("the far hit sits %v DIPs below the top: the search aligned it", top)
	}
}
