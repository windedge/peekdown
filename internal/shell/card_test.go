package shell

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// cardFrame opens the same document the screenshots use, so the card's
// pixels and the bars around it come from the production render path.
func cardFrame(t *testing.T, theme string, w, h int) *ui.Tester {
	t.Helper()
	s, _ := newShell(t)
	s.Saved.AccentTheme = theme
	s.Sidebar.Visible = true
	if err := s.OpenFile(cardShotDoc(t)); err != nil {
		t.Fatal(err)
	}
	tt := s.renderSize(t, w, h)
	tt.Frame()
	return tt
}

// bareFrame renders the same window with no document: the welcome page
// paints the gradient with no card over it, which gives each card
// assertion the gradient underneath that point.
func bareFrame(t *testing.T, theme string, w, h int) *ui.Tester {
	t.Helper()
	s, _ := newShell(t)
	s.Saved.AccentTheme = theme
	tt := s.renderSize(t, w, h)
	tt.Frame()
	if _, ok := tt.Find("Document card"); ok {
		t.Fatal("the welcome page must not draw a document card")
	}
	return tt
}

// TestDocumentCardOverGradient pins the layer order: the document card
// fills with CardFocused over the gradient, while the 8 DIP gutter still
// shows the gradient (darkened a little by the card's shadow).
func TestDocumentCardOverGradient(t *testing.T) {
	const theme = "apricot"
	tt := cardFrame(t, theme, 1400, 800)
	bare := bareFrame(t, theme, 1400, 800)
	card, ok := tt.Find("Document card")
	if !ok {
		t.Fatalf("no document card: %v", tt.Texts())
	}
	k := accentThemeByID(theme).Light
	x, y := card.X+40, card.Y+card.H*0.75
	want := over(k.CardFocused, pixelAt(bare.Image(), x, y))
	if got := pixelAt(tt.Image(), x, y); !channelClose(got, want, 2) {
		t.Fatalf("card pixel %v, want CardFocused over the gradient %v", got, want)
	}
	// Four DIPs outside the card the shadow has not erased the hue: the
	// pixel tracks the bare gradient far closer than the card's fill.
	gx := card.X - 4
	gap, ref := pixelAt(tt.Image(), gx, y), pixelAt(bare.Image(), gx, y)
	// The 22 DIP shadow reaches into the gutter, so it is a few channels
	// darker than the bare gradient rather than exactly equal to it.
	if channelSum(gap, ref) > 32 || channelSum(gap, ref) >= channelSum(gap, pixelAt(tt.Image(), x, y)) {
		t.Fatalf("gap pixel %v, want the gradient %v under the shadow, not card fill", gap, ref)
	}
}

// TestPanelCardsMatch checks both side panels use the translucent card
// token and sit in the same gutters as the document card.
func TestPanelCardsMatch(t *testing.T) {
	const theme = "apricot"
	tt := cardFrame(t, theme, 1400, 800)
	bare := bareFrame(t, theme, 1400, 800)
	doc, okD := tt.Find("Document card")
	files, okF := tt.Find("Files card")
	outline, okO := tt.Find("Outline card")
	if !okD || !okF || !okO {
		t.Fatalf("card labels missing: doc=%v files=%v outline=%v", okD, okF, okO)
	}
	for _, r := range []ui.Rect{doc, files, outline} {
		if r.Y != TitleBarHeight+8 {
			t.Fatalf("card starts at y=%v, want the 8 DIP gap under the bar", r.Y)
		}
		if r.Y+r.H > 800-8-28 {
			t.Fatalf("card ends at y=%v, want the 8 DIP gap over the status bar", r.Y+r.H)
		}
	}
	if files.X != 8 || 1400-(outline.X+outline.W) != 8 {
		t.Fatalf("side cards at x=%.0f and right=%.0f, want 8 DIP gutters", files.X, outline.X+outline.W)
	}
	if doc.X-(files.X+files.W) < 8 || outline.X-(doc.X+doc.W) < 8 {
		t.Fatalf("cards touch: files->%v doc->%v outline->%v", files.X+files.W, doc.X+doc.W, outline.X)
	}
	k := accentThemeByID(theme).Light
	for _, tc := range []struct {
		name string
		r    ui.Rect
	}{{"files", files}, {"outline", outline}} {
		x, y := tc.r.X+tc.r.W/2, tc.r.Y+tc.r.H/2
		want := over(k.Card, pixelAt(bare.Image(), x, y))
		if got := pixelAt(tt.Image(), x, y); !channelClose(got, want, 2) {
			t.Fatalf("%s card pixel %v, want Card over the gradient %v", tc.name, got, want)
		}
	}
}
