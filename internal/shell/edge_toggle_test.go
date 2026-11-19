package shell

import (
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/locales"
)

// The edge toggles are the document card's own panel switches: both sit
// inside its top corners whatever the panels do, and an accent glyph
// reads open where a muted one reads closed. The title bar holds no panel
// buttons anymore, so these are the only "Files panel" / "Outline panel"
// controls in the tree.

func openEdgeDoc(t *testing.T, s *Shell) *ui.Tester {
	t.Helper()
	if err := s.OpenFile(filepath.Join(t.TempDir(), "a.md")); err != nil {
		t.Fatal(err)
	}
	return nil
}

func TestEdgeTogglesPinToCardTopCorners(t *testing.T) {
	s, dir := newShell(t)
	if err := s.OpenFile(filepath.Join(dir, "a.md")); err != nil {
		t.Fatal(err)
	}
	s.Sidebar.Visible = true
	s.Outline.Visible = true
	tt := s.renderSize(t, 1200, 600)
	tt.Frame()
	tt.Frame()
	card, ok := tt.Find("Document card")
	if !ok {
		t.Fatalf("no document card: %v", tt.Texts())
	}
	files, okFiles := tt.Access("Files panel")
	outline, okOutline := tt.Access("Outline panel")
	if !okFiles || !okOutline {
		t.Fatalf("edge toggles missing: %v", tt.Texts())
	}
	cases := []struct {
		info  ui.AccessInfo
		wantX float32
	}{
		{files, card.X + edgeToggleInset},
		{outline, card.X + card.W - edgeToggleInset - edgeToggleSize},
	}
	for _, c := range cases {
		a := c.info
		if a.Role != ui.RoleButton {
			t.Fatalf("%q is role %v, want a button", a.Name, a.Role)
		}
		b := a.Bounds
		if b.W != edgeToggleSize || b.H != edgeToggleSize {
			t.Fatalf("%q is %vx%v, want %vx%v", a.Name, b.W, b.H, edgeToggleSize, edgeToggleSize)
		}
		if math.Abs(float64(b.X-c.wantX)) > 1 {
			t.Fatalf("%q starts at %v, want the card corner %v +/- 1", a.Name, b.X, c.wantX)
		}
		if math.Abs(float64(b.Y-(card.Y+edgeToggleTop))) > 1 {
			t.Fatalf("%q starts at y=%v, want the card's inner top %v +/- 1", a.Name, b.Y, card.Y+edgeToggleTop)
		}
		if b.X <= card.X || b.X+b.W >= card.X+card.W || b.Y < card.Y || b.Y+b.H > card.Y+card.H {
			t.Fatalf("%q bounds %v are not fully inside the card %v", a.Name, b, card)
		}
	}
}

func TestEdgeTogglesClearFrontmatterButtons(t *testing.T) {
	s, _ := newShell(t)
	if err := s.OpenFile(cardShotDoc(t)); err != nil {
		t.Fatal(err)
	}
	s.Sidebar.Visible = true
	s.Outline.Visible = true
	tt := s.renderSize(t, 1200, 600)
	tt.Frame()
	tt.Frame()
	for _, name := range []string{"Files panel", "Outline panel"} {
		toggle, ok := tt.Access(name)
		if !ok {
			t.Fatalf("no %q toggle", name)
		}
		for _, other := range []string{locales.ToggleFrontmatter(), locales.CopyYAML()} {
			info, ok := tt.Access(other)
			if !ok {
				t.Fatalf("no %q button in the frontmatter card", other)
			}
			if rectsOverlap(toggle.Bounds, info.Bounds) {
				t.Fatalf("%q bounds %v overlap the frontmatter %q bounds %v", name, toggle.Bounds, other, info.Bounds)
			}
		}
	}
}

func rectsOverlap(a, b ui.Rect) bool {
	return a.X < b.X+b.W && b.X < a.X+a.W && a.Y < b.Y+b.H && b.Y < a.Y+a.H
}

func TestEdgeTogglesFollowCardWhenPanesClose(t *testing.T) {
	s, dir := newShell(t)
	if err := s.OpenFile(filepath.Join(dir, "a.md")); err != nil {
		t.Fatal(err)
	}
	s.Sidebar.Visible = true
	s.Outline.Visible = true
	tt := s.renderSize(t, 1200, 600)
	tt.Frame()
	tt.Frame()
	card, ok := tt.Find("Document card")
	if !ok {
		t.Fatal("no document card with both panes open")
	}
	openLeft, ok := tt.Access("Files panel")
	if !ok {
		t.Fatal("no files toggle with both panes open")
	}
	openRight, ok := tt.Access("Outline panel")
	if !ok {
		t.Fatal("no outline toggle with both panes open")
	}

	// Closing the files pane moves the card's left edge; the right button
	// stays on the right edge, which the outline still anchors.
	filesWidth := s.Sidebar.Width
	s.Sidebar.Visible = false
	tt.Frame()
	tt.Frame()
	closedCard, ok := tt.Find("Document card")
	if !ok {
		t.Fatal("no document card with the files pane closed")
	}
	closedLeft, ok := tt.Access("Files panel")
	if !ok {
		t.Fatal("no files toggle with the files pane closed")
	}
	closedRight, ok := tt.Access("Outline panel")
	if !ok {
		t.Fatal("no outline toggle with the files pane closed")
	}
	leftShift := closedLeft.Bounds.X - openLeft.Bounds.X
	if math.Abs(float64(leftShift-(closedCard.X-card.X))) > 1 {
		t.Fatalf("files toggle moved %v, card's left edge moved %v", leftShift, closedCard.X-card.X)
	}
	if math.Abs(float64(-leftShift-filesWidth-1)) > 1 {
		t.Fatalf("files toggle moved %v when the %v-wide pane closed, want %v", leftShift, filesWidth, -filesWidth-1)
	}
	if math.Abs(float64(closedRight.Bounds.X-openRight.Bounds.X)) > 1 {
		t.Fatalf("outline toggle moved when only the files pane closed")
	}

	// Closing the outline moves the card's right edge, and its toggle with it.
	outlineWidth := s.Outline.Width
	s.Outline.Visible = false
	tt.Frame()
	tt.Frame()
	fullCard, ok := tt.Find("Document card")
	if !ok {
		t.Fatal("no document card with both panes closed")
	}
	fullRight, ok := tt.Access("Outline panel")
	if !ok {
		t.Fatal("no outline toggle with both panes closed")
	}
	rightShift := fullRight.Bounds.X - closedRight.Bounds.X
	if math.Abs(float64(rightShift-((fullCard.X+fullCard.W)-(closedCard.X+closedCard.W)))) > 1 {
		t.Fatalf("outline toggle moved %v, card's right edge moved %v", rightShift, (fullCard.X+fullCard.W)-(closedCard.X+closedCard.W))
	}
	if math.Abs(float64(rightShift-outlineWidth-1)) > 1 {
		t.Fatalf("outline toggle moved %v when the %v-wide pane closed, want %v", rightShift, outlineWidth, outlineWidth+1)
	}
}

func TestEdgeTogglesFlipAndReadTheirState(t *testing.T) {
	s, dir := newShell(t)
	if err := s.OpenFile(filepath.Join(dir, "a.md")); err != nil {
		t.Fatal(err)
	}
	// The flip test pins the start state: a fresh shell restores the
	// saved visibility (off), so without this the first click opens.
	s.Sidebar.Visible = true
	tt := s.renderSize(t, 1200, 600)
	tt.Frame()
	tt.Frame()

	if err := tt.Click("Files panel"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if s.Sidebar.Visible {
		t.Fatal("the files toggle must close the pane")
	}
	if a, ok := tt.Access("Files panel"); !ok || a.Expanded {
		t.Fatal("a closed files toggle must read unexpanded and wear a muted glyph")
	}
	if err := tt.Click("Files panel"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if !s.Sidebar.Visible {
		t.Fatal("the files toggle must reopen the pane")
	}
	if a, ok := tt.Access("Files panel"); !ok || !a.Expanded {
		t.Fatal("an open files toggle must read expanded and wear an accent glyph")
	}

	if err := tt.Click("Outline panel"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if s.Outline.Visible {
		t.Fatal("the outline toggle must close the pane")
	}
	if err := tt.Click("Outline panel"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if !s.Outline.Visible || !s.Sidebar.Visible {
		t.Fatal("the two toggles must work independently")
	}
}

// The tester cannot read a button child's SVG pointer, so the icon
// pairing is checked at its source: the two panelEdgeButton call sites
// must pass the folder tree and outline glyphs with one icon argument.
func TestEdgeToggleIconWiring(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "edgetoggle.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var calls []*ast.CallExpr
	ast.Inspect(f, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "panelEdgeButton" {
				calls = append(calls, call)
			}
		}
		return true
	})
	want := []struct{ key, icon string }{
		{"pane-files-toggle", "folderTreeIcon"},
		{"pane-outline-toggle", "outlineIcon"},
	}
	if len(calls) != len(want) {
		t.Fatalf("panelEdgeButton call sites = %d, want %d", len(calls), len(want))
	}
	for i, call := range calls {
		if len(call.Args) != 7 {
			t.Fatalf("%s passes %d arguments, want one icon argument in a 7-argument call", want[i].key, len(call.Args))
		}
		lit, ok := call.Args[1].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			t.Fatalf("%s key is not a string literal", want[i].key)
		}
		key, err := strconv.Unquote(lit.Value)
		if err != nil || key != want[i].key {
			t.Fatalf("call %d key = %q, want %q", i, lit.Value, want[i].key)
		}
		icon, ok := call.Args[4].(*ast.Ident)
		if !ok || icon.Name != want[i].icon {
			t.Fatalf("%s icon = %v, want %s", key, call.Args[4], want[i].icon)
		}
	}
}

// The glyph color carries the open/closed state now that the icon no
// longer turns. Count pixels of the two widget colors inside the glyph;
// the card and gradient behind it are neither color.
func TestEdgeToggleGlyphColorTracksState(t *testing.T) {
	s, dir := newShell(t)
	if err := s.OpenFile(filepath.Join(dir, "a.md")); err != nil {
		t.Fatal(err)
	}
	s.Sidebar.Visible = true
	tt := s.renderSize(t, 1200, 600)
	tt.Frame()
	tt.Frame()
	btn, ok := tt.Find("Files panel")
	if !ok {
		t.Fatal("files edge button missing")
	}
	th := windowLight()
	count := func(want ui.Color) int {
		n := 0
		for y := int(btn.Y) + 4; y < int(btn.Y)+20; y++ {
			for x := int(btn.X) + 4; x < int(btn.X)+20; x++ {
				if channelClose(pixelAt(tt.Image(), float32(x), float32(y)), want, 1) {
					n++
				}
			}
		}
		return n
	}
	if n := count(th.Accent); n < 3 {
		t.Fatalf("open glyph has %d accent pixels, want at least 3", n)
	}
	if err := tt.Click("Files panel"); err != nil {
		t.Fatal(err)
	}
	tt.Move(600, 300)
	tt.Frame()
	tt.Frame()
	// Closing the pane widens the card, so the toggle follows it.
	btn, ok = tt.Find("Files panel")
	if !ok {
		t.Fatal("files edge button missing after the toggle")
	}
	if n := count(th.TextMuted); n < 3 {
		t.Fatalf("closed glyph has %d muted pixels, want at least 3", n)
	}
	if n := count(th.Accent); n != 0 {
		t.Fatalf("closed glyph still has %d accent pixels", n)
	}
}

func TestEdgeTogglesShowClosedAndSkipWelcome(t *testing.T) {
	s, dir := newShell(t)
	if err := s.OpenFile(filepath.Join(dir, "a.md")); err != nil {
		t.Fatal(err)
	}
	s.Sidebar.Visible = false
	s.Outline.Visible = false
	tt := s.renderSize(t, 1200, 600)
	tt.Frame()
	tt.Frame()
	for _, label := range []string{"Files panel", "Outline panel"} {
		if _, ok := tt.Access(label); !ok {
			t.Fatalf("with the panes closed the %q toggle must still show, over the gradient", label)
		}
	}

	// The welcome page has no content area and no toggles.
	w, _ := newShell(t)
	tw := w.renderSize(t, 1200, 600)
	tw.Frame()
	if _, ok := tw.Find("Content area"); ok {
		t.Fatal("the welcome page must not draw a content area")
	}
	for _, label := range []string{"Files panel", "Outline panel"} {
		if _, ok := tw.Access(label); ok {
			t.Fatalf("the welcome page draws a %q toggle", label)
		}
	}
}
