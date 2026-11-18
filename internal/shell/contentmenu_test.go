package shell

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// rightClickBody opens the document area's context menu: the click lands on
// a paragraph of the open document, away from the tab bar and the sidebar.
func rightClickBody(t *testing.T, tt *ui.Tester, want string) {
	t.Helper()
	r, ok := paragraphRect(tt, want)
	if !ok {
		t.Fatalf("texts %v: %q not drawn", tt.Texts(), want)
	}
	tt.RightClickAt(r.X+r.W/2, r.Y+r.H/2)
}

// TestDocumentContextMenuListsTheActions opens the body's menu: it shows the
// document commands in the Rust version's order.
func TestDocumentContextMenuListsTheActions(t *testing.T) {
	s, dir := newShell(t)
	s.OpenFile(filepath.Join(dir, "a.md"))
	tt := s.render(t)
	tt.Frame()

	rightClickBody(t, tt, "first")
	want := []string{"Copy", "Select All", "-", "Find", "Refresh", "-", "Reveal in Sidebar", "Open in File Manager", "Copy Path"}
	got := tt.Menu()
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("menu %v, want %v", got, want)
	}
	tt.CloseMenu()
}

// TestDocumentContextMenuCopyDisabledWithoutSelection: with nothing
// selected the Copy item is greyed out and cannot be chosen.
func TestDocumentContextMenuCopyDisabledWithoutSelection(t *testing.T) {
	s, dir := newShell(t)
	s.OpenFile(filepath.Join(dir, "a.md"))
	tt := s.render(t)
	tt.Frame()

	rightClickBody(t, tt, "first")
	if err := tt.ChooseMenuItem("Copy"); err == nil {
		t.Fatal("Copy was choosable without a selection")
	}
	tt.CloseMenu()
}

// TestDocumentContextMenuCopyAndSelectAll walks the two editing commands:
// Select All fills the selection, Copy writes it to the clipboard.
func TestDocumentContextMenuCopyAndSelectAll(t *testing.T) {
	s, dir := newShell(t)
	s.OpenFile(filepath.Join(dir, "a.md"))
	tt := s.render(t)
	tt.Frame()

	rightClickBody(t, tt, "first")
	if err := tt.ChooseMenuItem("Select All"); err != nil {
		t.Fatal(err)
	}
	if !s.ActiveTab().State.HasSelection() {
		t.Fatal("Select All left the document unselected")
	}

	rightClickBody(t, tt, "first")
	if err := tt.ChooseMenuItem("Copy"); err != nil {
		t.Fatal(err)
	}
	if got := tt.Clipboard(); !strings.Contains(got, "first") {
		t.Fatalf("clipboard %q, want the selected document", got)
	}
}

// TestDocumentContextMenuFileCommands walks Copy Path, Reveal in Sidebar and
// Open in File Manager: each acts on the open document's path.
func TestDocumentContextMenuFileCommands(t *testing.T) {
	s, dir := newShell(t)
	path := filepath.Join(dir, "a.md")
	s.OpenFile(path)
	var revealed []string
	s.ShowItemInFolder = func(p string) { revealed = append(revealed, p) }
	tt := s.render(t)
	tt.Frame()

	rightClickBody(t, tt, "first")
	if err := tt.ChooseMenuItem("Copy Path"); err != nil {
		t.Fatal(err)
	}
	if got := tt.Clipboard(); got != path {
		t.Fatalf("clipboard %q, want %q", got, path)
	}

	rightClickBody(t, tt, "first")
	if err := tt.ChooseMenuItem("Reveal in Sidebar"); err != nil {
		t.Fatal(err)
	}
	if !s.Sidebar.Visible {
		t.Fatal("reveal left the file pane hidden")
	}
	if !tt.HasText("a.md") {
		t.Fatalf("texts %v: the file pane does not list the document", tt.Texts())
	}

	rightClickBody(t, tt, "first")
	if err := tt.ChooseMenuItem("Open in File Manager"); err != nil {
		t.Fatal(err)
	}
	if len(revealed) != 1 || revealed[0] != path {
		t.Fatalf("revealed %v, want [%s]", revealed, path)
	}
}

// TestDocumentContextMenuFindAndRefresh walks the view commands: Find opens
// the search bar, Refresh rereads the file from disk.
func TestDocumentContextMenuFindAndRefresh(t *testing.T) {
	s, dir := newShell(t)
	path := filepath.Join(dir, "a.md")
	s.OpenFile(path)
	tt := s.render(t)
	tt.Frame()

	rightClickBody(t, tt, "first")
	if err := tt.ChooseMenuItem("Find"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !s.SearchOpen {
		t.Fatal("Find did not open the search bar")
	}
	s.SearchOpen = false
	tt.Frame()

	if err := os.WriteFile(path, []byte("# A\n\nrewritten on disk\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rightClickBody(t, tt, "first")
	if err := tt.ChooseMenuItem("Refresh"); err != nil {
		t.Fatal(err)
	}
	if src := s.ActiveTab().Model.Source(); !strings.Contains(src, "rewritten on disk") {
		t.Fatalf("Refresh did not reread the file: %q", src)
	}
}

// TestTabContextMenuOpensTheFileManager: the tab menu carries Open in File
// Manager, as the Rust version's does.
func TestTabContextMenuOpensTheFileManager(t *testing.T) {
	s, dir := newShell(t)
	path := filepath.Join(dir, "a.md")
	s.OpenFile(path)
	var revealed []string
	s.ShowItemInFolder = func(p string) { revealed = append(revealed, p) }
	tt := s.render(t)
	tt.Frame()

	if err := tt.RightClick("a.md"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Open in File Manager"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(revealed) != 1 || revealed[0] != path {
		t.Fatalf("revealed %v, want [%s]", revealed, path)
	}
}
