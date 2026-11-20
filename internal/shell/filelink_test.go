package shell

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/settings"
	"github.com/windedge/peekdown/internal/view"
)

// linkFiles writes a.md linking to b.md plain, with an anchor, a missing
// file, an image, a subdirectory file and an upper-case one; b.md with a
// second heading; sub/c.md under a directory.
func linkFiles(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write := func(name, src string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a.md", "# A\n\nLinks: [bee](b.md), [bee anchor](b.md#sec), [missing](nope.md), [img](pic.png), [dir](sub), [sub](sub/c.md), [upper](B.MD).\n")
	write("b.md", "# B\n\nbody text\n\n## Sec\n\nsec body\n")
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(filepath.Join("sub", "c.md"), "# C\n\nsub body\n")
	return dir
}

// clickLink clicks the middle of a link's text, the way view.linkPoint
// does: the text element spans the column, so its rect's middle sits
// past the glyphs and the shaped carets give the glyphs' x range.
func clickLink(t *testing.T, tt *ui.Tester, want string, size float32) {
	t.Helper()
	for _, label := range tt.Texts() {
		if !strings.Contains(label, want) {
			continue
		}
		r, ok := tt.Find(label)
		if !ok {
			t.Fatalf("%q found as text but has no rect", want)
		}
		text := strings.SplitN(label, "\n", 2)[0]
		layout := ui.ShapeText(text, ui.Font{Size: size}, 10000)
		a := strings.Index(text, want)
		mid := (layout.Caret(a).X + layout.Caret(a+len(want)).X) / 2
		if mid <= 0 {
			t.Fatalf("no caret boxes for %q in %q", want, text)
		}
		tt.ClickAt(r.X+mid, r.Y+r.H/2)
		tt.Frame()
		return
	}
	t.Fatalf("texts %v: %q not found", tt.Texts(), want)
}

func TestShellLocalLinkOpensTarget(t *testing.T) {
	dir := linkFiles(t)
	s, _ := newShell(t)
	tt := s.render(t)
	if err := s.OpenFile(filepath.Join(dir, "a.md")); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	clickLink(t, tt, "bee,", float32(settings.Default().FontSize))
	if tab := s.ActiveTab(); tab == nil || tab.Path != filepath.Join(dir, "b.md") {
		t.Fatalf("active %v, want b.md", tab)
	}
	if len(s.Tabs) != 2 {
		t.Fatalf("tabs = %d, want 2", len(s.Tabs))
	}

	// The same link again, from a.md's own tab: the open tab activates,
	// no third tab. Back on a.md first, or the link text is not in the
	// frame to click.
	tt.Click("a.md")
	tt.Frame()
	clickLink(t, tt, "bee,", float32(settings.Default().FontSize))
	if len(s.Tabs) != 2 || s.Active != 1 || s.ActiveTab().Path != filepath.Join(dir, "b.md") {
		t.Fatalf("reopen: %d tabs, active %d (%v)", len(s.Tabs), s.Active, s.ActiveTab())
	}
}

func TestShellLocalLinkWithAnchorScrolls(t *testing.T) {
	dir := linkFiles(t)
	s, _ := newShell(t)
	tt := s.render(t)
	if err := s.OpenFile(filepath.Join(dir, "a.md")); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	clickLink(t, tt, "bee anchor", float32(settings.Default().FontSize))
	tab := s.ActiveTab()
	if tab == nil || tab.Path != filepath.Join(dir, "b.md") {
		t.Fatalf("active %v, want b.md", tab)
	}
	block, ok := tab.Model.AnchorBySlug("sec")
	if !ok {
		t.Fatal("b.md lost its Sec heading")
	}
	// Either the shell resolved the heading at once (ScrollTargetBlock)
	// or the view consumed the pending slug into its anchor scroll; both
	// land the heading in view and clear the pending state.
	if tab.State.PendingAnchor != "" {
		t.Fatalf("pending anchor %q left over", tab.State.PendingAnchor)
	}
	if tab.State.Anchor != "sec" && tab.State.ScrollTargetBlock != block {
		t.Fatalf("anchor %q target %d, want sec or block %d", tab.State.Anchor, tab.State.ScrollTargetBlock, block)
	}
	if !tt.HasText("Sec") {
		t.Fatalf("texts %v: the Sec heading is not in view", tt.Texts())
	}
}

func TestShellLocalLinkDropsMissingTarget(t *testing.T) {
	dir := linkFiles(t)
	s, _ := newShell(t)
	tt := s.render(t)
	if err := s.OpenFile(filepath.Join(dir, "a.md")); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	clickLink(t, tt, "missing", float32(settings.Default().FontSize))
	if len(s.Tabs) != 1 {
		t.Fatalf("tabs = %d, want 1", len(s.Tabs))
	}
	if tab := s.ActiveTab(); tab == nil || tab.Path != filepath.Join(dir, "a.md") {
		t.Fatalf("active %v, want a.md untouched", tab)
	}
}

func TestShellLocalLinkRevealsDirectoryAndNonMarkdown(t *testing.T) {
	dir := linkFiles(t)
	s, _ := newShell(t)
	var revealed []string
	s.ShowItemInFolder = func(path string) { revealed = append(revealed, path) }
	tt := s.render(t)
	if err := s.OpenFile(filepath.Join(dir, "a.md")); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	for _, label := range []string{"dir", "img"} {
		clickLink(t, tt, label, float32(settings.Default().FontSize))
	}
	want := []string{filepath.Join(dir, "sub"), filepath.Join(dir, "pic.png")}
	if !reflect.DeepEqual(revealed, want) {
		t.Fatalf("revealed %v, want %v", revealed, want)
	}
	if len(s.Tabs) != 1 {
		t.Fatalf("tabs = %d, want 1: a revealed target must not open a tab", len(s.Tabs))
	}
	if tab := s.ActiveTab(); tab == nil || tab.Path != filepath.Join(dir, "a.md") {
		t.Fatalf("active %v, want a.md untouched", tab)
	}
}

func TestShellLocalLinkResolvesAgainstDocDir(t *testing.T) {
	dir := linkFiles(t)
	s, _ := newShell(t)
	tt := s.render(t)
	if err := s.OpenFile(filepath.Join(dir, "a.md")); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	clickLink(t, tt, "sub", float32(settings.Default().FontSize))
	want := filepath.Join(dir, "sub", "c.md")
	if tab := s.ActiveTab(); tab == nil || tab.Path != want {
		t.Fatalf("active %v, want %s", tab, want)
	}
	if len(s.Tabs) != 2 {
		t.Fatalf("tabs = %d, want 2", len(s.Tabs))
	}
}

func TestShellLocalLinkCaseInsensitiveSuffix(t *testing.T) {
	dir := linkFiles(t)
	s, _ := newShell(t)
	tt := s.render(t)
	if err := s.OpenFile(filepath.Join(dir, "a.md")); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	clickLink(t, tt, "upper", float32(settings.Default().FontSize))
	if _, err := os.Stat(filepath.Join(dir, "B.MD")); err != nil {
		// A case-only alias is another file on a case-sensitive disk and
		// does not exist here: the link silently does nothing.
		if len(s.Tabs) != 1 {
			t.Fatalf("tabs = %d, want 1 on a case-sensitive disk", len(s.Tabs))
		}
		return
	}
	// A case-insensitive disk names the file under any case, so the link
	// opens the disk file: a second open of b.md, the same file as tab 1,
	// must land on that tab, never add another.
	if err := s.OpenFile(filepath.Join(dir, "b.md")); err != nil {
		t.Fatal(err)
	}
	if len(s.Tabs) != 2 || s.Active != 1 || !sameFile(s.ActiveTab().Path, filepath.Join(dir, "b.md")) {
		t.Fatalf("case-insensitive open: %d tabs, active %d (%v)", len(s.Tabs), s.Active, s.ActiveTab().Path)
	}
}

// The view's pending-anchor state the shell arms must be reachable from
// the shell package: compile-time probe of the exported surface.
var _ = view.State{PendingAnchor: ""}
