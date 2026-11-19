package view

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/doc"
)

// linkDoc builds the state over a document with links and wires an
// OpenURL stub that records what it was asked to open.
func linkDoc(t *testing.T, src string, w, h int) (*doc.Model, *ui.Tester, *State, *[]string) {
	t.Helper()
	m := doc.New(src)
	var opened []string
	st := &State{AnchorOf: anchorsOf(m)}
	st.OpenURL = func(url string) {
		if urlSafe(url) {
			opened = append(opened, url)
			return
		}
		opened = append(opened, url)
	}
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		ScrollView(c, m, st)
	}, w, h)
	tt.Frame()
	return m, tt, st, &opened
}

// localLinkDoc is linkDoc with an OpenLocalLink stub too: local records
// the raw destinations the view hands over, opened the web ones.
func localLinkDoc(t *testing.T, src string, w, h int) (*doc.Model, *ui.Tester, *State, *[]string, *[]string) {
	t.Helper()
	m := doc.New(src)
	var opened, local []string
	st := &State{AnchorOf: anchorsOf(m)}
	st.OpenURL = func(url string) { opened = append(opened, url) }
	st.OpenLocalLink = func(dest string) { local = append(local, dest) }
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		ScrollView(c, m, st)
	}, w, h)
	tt.Frame()
	return m, tt, st, &opened, &local
}

// linkPoint returns a window point over the middle of the first text
// holding want, from the shaped line's caret boxes: the text element
// stretches across the column, so its rect's middle is past the glyphs.
func linkPoint(t *testing.T, tt *ui.Tester, want string) (float32, float32) {
	t.Helper()
	for _, label := range tt.Texts() {
		if !strings.Contains(label, want) {
			continue
		}
		r, ok := tt.Find(label)
		if !ok {
			continue
		}
		text := strings.SplitN(label, "\n", 2)[0]
		layout := ui.ShapeText(text, ui.Font{Size: defaultBodySize}, 10000)
		a, b := strings.Index(text, want), strings.Index(text, want)+len(want)
		mid := (layout.Caret(a).X + layout.Caret(b).X) / 2
		if mid <= 0 {
			t.Fatalf("no caret boxes for %q in %q", want, text)
		}
		return r.X + mid, r.Y + r.H/2
	}
	t.Fatalf("texts %v: %q not found", tt.Texts(), want)
	return 0, 0
}

func TestClickOnLinkOpensIt(t *testing.T) {
	m, tt, st, opened := linkDoc(t, "See [the guide](https://example.com/guide) now.\n", 400, 300)
	x, y := linkPoint(t, tt, "the guide")
	tt.ClickAt(x, y)
	if len(*opened) != 1 || (*opened)[0] != "https://example.com/guide" {
		t.Fatalf("opened %v, want the guide's URL", *opened)
	}
	// The click collapsed the selection; the drag the press began set a
	// caret at the link.
	if st.HasSelection() {
		t.Fatalf("the click left a selection %q", st.SelectionText(m))
	}
}

func TestClickOnLinkWithMailtoOpensIt(t *testing.T) {
	_, tt, _, opened := linkDoc(t, "Mail [the team](mailto:team@example.com) here.\n", 400, 300)
	x, y := linkPoint(t, tt, "the team")
	tt.ClickAt(x, y)
	if len(*opened) != 1 || (*opened)[0] != "mailto:team@example.com" {
		t.Fatalf("opened %v, want the mailto", *opened)
	}
}

func TestClickOnAnchorLinkScrollsToHeading(t *testing.T) {
	m, tt, st, opened := linkDoc(t, "Intro [to details](#the-details) text.\n\nDetails body here.\n\n## The Details\n\nDeep content.\n", 400, 400)
	x, y := linkPoint(t, tt, "to details")
	tt.ClickAt(x, y)
	if len(*opened) != 0 {
		t.Fatalf("opened %v, want no browser for an in-document anchor", *opened)
	}
	block, ok := m.AnchorBySlug("the-details")
	if !ok {
		t.Fatal("the anchor was not in the document")
	}
	if st.ScrollTargetBlock != block {
		t.Fatalf("ScrollTargetBlock = %d, want heading block %d", st.ScrollTargetBlock, block)
	}
}

func TestClickOnUnsafeLinkDoesNothing(t *testing.T) {
	_, tt, _, opened := linkDoc(t, "A [javascript link](javascript:alert(1)) here.\n", 400, 300)
	x, y := linkPoint(t, tt, "javascript link")
	tt.ClickAt(x, y)
	if len(*opened) != 0 {
		t.Fatalf("opened %v, want nothing for javascript:", *opened)
	}
}

func TestDragAcrossLinkDoesNotOpenIt(t *testing.T) {
	m, tt, st, opened := linkDoc(t, "First [a link](https://example.com/x) here.\n\nSecond paragraph after.\n", 400, 300)
	x0, y0 := linkPoint(t, tt, "a link")
	// Drag from inside the link's text across to the next paragraph: the
	// press-release is no click, and must not open the link. The drag
	// starts mid-word, so the selection opens there too.
	x1, y1 := linkPoint(t, tt, "Second paragraph")
	dragFromTo(tt, x0, y0, x1, y1)
	if len(*opened) != 0 {
		t.Fatalf("opened %v, a drag must not open links", *opened)
	}
	if !st.HasSelection() {
		t.Fatal("the drag made no selection")
	}
	if got := st.SelectionText(m); !strings.Contains(got, "Second p") {
		t.Fatalf("selected %q, want the second paragraph in it", st.SelectionText(m))
	}
	// The copy is the markdown source now: the selection from inside the
	// link's label carries the link's markup.
	if !strings.Contains(st.SelectionText(m), "](htt") {
		t.Fatalf("selected %q, want the link's source in it", st.SelectionText(m))
	}
	_ = m
}

func TestDragStartingOnLinkSelectsInstead(t *testing.T) {
	_, tt, _, opened := linkDoc(t, "Words [a link](https://example.com/x) more words here.\n", 500, 300)
	x, y := linkPoint(t, tt, "a link")
	// A drag wholly inside the link's text: still a selection, no open.
	dragFromTo(tt, x-15, y, x+15, y)
	if len(*opened) != 0 {
		t.Fatalf("opened %v, a drag within the link must not open it", *opened)
	}
}

func TestImageWrappedInLinkOpensTheOuterURL(t *testing.T) {
	// The walk marks the alt text with the image's own dest; the click
	// must open the outer link's.
	src := "[![Badge](https://img.example.com/badge.svg)](https://ci.example.com/pipeline)\n"
	m, tt, _, opened := linkDoc(t, src, 400, 300)
	if m.LinkTargetOf(m.Blocks()[0].Index) != "https://ci.example.com/pipeline" {
		t.Fatalf("LinkTargetOf = %q, want the outer pipeline URL", m.LinkTargetOf(m.Blocks()[0].Index))
	}
	x, y := linkPoint(t, tt, "Badge")
	tt.ClickAt(x, y)
	if len(*opened) != 1 || (*opened)[0] != "https://ci.example.com/pipeline" {
		t.Fatalf("opened %v, want the outer pipeline URL", *opened)
	}
}

func TestURLSafe(t *testing.T) {
	for _, ok := range []string{"http://a.b", "https://a.b/c", "mailto:a@b.c", "MAILTO:a@b.c", "HTTPS://A.B"} {
		if !urlSafe(ok) {
			t.Fatalf("urlSafe(%q) = false", ok)
		}
	}
	for _, bad := range []string{"javascript:alert(1)", "file:///C:/x", "data:text/html,x", "#frag", "", "ftp://x"} {
		if urlSafe(bad) {
			t.Fatalf("urlSafe(%q) = true", bad)
		}
	}
}

func TestClickOnFileLinkHandsRawDestToShell(t *testing.T) {
	// The anchor tail rides along raw: the view does not split it, the
	// shell resolves the path and the heading.
	_, tt, _, opened, local := localLinkDoc(t, "Read [the other](./other.md#sec) too.\n", 400, 300)
	x, y := linkPoint(t, tt, "the other")
	tt.ClickAt(x, y)
	if len(*opened) != 0 {
		t.Fatalf("opened %v, a file link must not reach the browser", *opened)
	}
	if len(*local) != 1 || (*local)[0] != "./other.md#sec" {
		t.Fatalf("local %v, want the raw destination with its anchor", *local)
	}
}

func TestClickOnPlainFileLinkHandsItOver(t *testing.T) {
	_, tt, _, opened, local := localLinkDoc(t, "Open [the notes](notes.md) later.\n", 400, 300)
	x, y := linkPoint(t, tt, "the notes")
	tt.ClickAt(x, y)
	if len(*opened) != 0 {
		t.Fatalf("opened %v, a file link must not reach the browser", *opened)
	}
	if len(*local) != 1 || (*local)[0] != "notes.md" {
		t.Fatalf("local %v, want the plain destination", *local)
	}
}

func TestClickOnNonMarkdownFileStillHandsItOver(t *testing.T) {
	// The view knows no document directory and no open policy, so even a
	// png rides to the shell, which silently drops it; the view must not
	// swallow the click on its own judgement.
	_, tt, _, opened, local := localLinkDoc(t, "See [the picture](pic.png) here.\n", 400, 300)
	x, y := linkPoint(t, tt, "the picture")
	tt.ClickAt(x, y)
	if len(*opened) != 0 {
		t.Fatalf("opened %v, a file link must not reach the browser", *opened)
	}
	if len(*local) != 1 || (*local)[0] != "pic.png" {
		t.Fatalf("local %v, want pic.png handed to the shell", *local)
	}
}

func TestClickOnFileLinkWithoutCallbackDoesNothing(t *testing.T) {
	m := doc.New("See [the other](other.md) too.\n")
	st := &State{}
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		ScrollView(c, m, st)
	}, 400, 300)
	tt.Frame()
	for _, label := range tt.Texts() {
		if strings.Contains(label, "the other") {
			r, ok := tt.Find(label)
			if !ok {
				t.Fatal("label found but no rect")
			}
			tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
			tt.Frame()
			if st.HasSelection() {
				t.Fatal("the click made a selection")
			}
			return
		}
	}
	t.Fatalf("texts %v: the link text is missing", tt.Texts())
}

func TestPendingAnchorPromotesWhenHeadingAppears(t *testing.T) {
	m := doc.New("# Top\n\nIntro text.\n")
	st := &State{AnchorOf: anchorsOf(m)}
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		ScrollView(c, m, st)
	}, 400, 300)
	tt.Frame()
	st.ResetPendingAnchor("the-details")
	// Still pending: the document holds no such heading yet.
	tt.Frame()
	if st.PendingAnchor != "the-details" || st.Anchor != "" {
		t.Fatalf("pending %q anchor %q, want the slug still pending", st.PendingAnchor, st.Anchor)
	}
	if has(tt, "The Details") {
		t.Fatalf("texts %v: the missing heading scrolled into view", tt.Texts())
	}
	// The document gains the heading, as a reload would: the next frame
	// promotes the slug to Anchor, whose scrollPendingAnchor asks the
	// list to bring the heading's row in.
	if err := m.Set("# Top\n\nIntro text.\n\n## The Details\n\nBody.\n"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if st.PendingAnchor != "" {
		t.Fatalf("pending %q left, want it consumed", st.PendingAnchor)
	}
	if st.Anchor != "the-details" {
		t.Fatalf("anchor %q, want the-details", st.Anchor)
	}
	if !has(tt, "The Details") {
		t.Fatalf("texts %v: the heading did not scroll into view", tt.Texts())
	}
}

func TestPendingAnchorDropsAfterTimeout(t *testing.T) {
	m := doc.New("# Top\n\nJust text, no such heading.\n")
	st := &State{AnchorOf: anchorsOf(m)}
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		ScrollView(c, m, st)
	}, 400, 300)
	tt.Frame()
	st.ResetPendingAnchor("never-there")
	for range pendingAnchorTries + 1 {
		tt.Frame()
	}
	if st.PendingAnchor != "" {
		t.Fatalf("pending %q survived the timeout", st.PendingAnchor)
	}
	if st.Anchor != "" {
		t.Fatalf("anchor %q set from a dead slug", st.Anchor)
	}
}
