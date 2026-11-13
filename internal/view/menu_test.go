package view

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/doc"
	"github.com/windedge/peekdown/internal/selection"
)

// menuState renders the document with the shell's menu wired to a recorder:
// every action appends its name, HasSelection reads the view's own model.
func menuState(t *testing.T, src string, ran *[]string) (*doc.Model, *ui.Tester, *State) {
	t.Helper()
	m := doc.New(src)
	st := &State{AnchorOf: anchorsOf(m)}
	st.Menu = &MenuActions{
		HasSelection:      st.HasSelection,
		Copy:              func() { *ran = append(*ran, "copy") },
		SelectAll:         func() { *ran = append(*ran, "select-all") },
		Find:              func() { *ran = append(*ran, "find") },
		Refresh:           func() { *ran = append(*ran, "refresh") },
		Reveal:            func() { *ran = append(*ran, "reveal") },
		OpenInFileManager: func() { *ran = append(*ran, "open-in-fm") },
		CopyPath:          func() { *ran = append(*ran, "copy-path") },
	}
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		ScrollView(c, m, st)
	}, 400, 300)
	tt.Frame()
	return m, tt, st
}

// rightClickText opens the document menu over the drawn text want.
func rightClickText(t *testing.T, tt *ui.Tester, want string) {
	t.Helper()
	r, ok := tt.Find(want)
	if !ok {
		t.Fatalf("texts %v: %q not drawn", tt.Texts(), want)
	}
	tt.RightClickAt(r.X+r.W/2, r.Y+r.H/2)
}

// TestDocumentMenuItemsAndCallbacks walks the document's context menu: it
// lists the commands, Copy greys out without a selection, and every item
// reaches the callback the shell injected.
func TestDocumentMenuItemsAndCallbacks(t *testing.T) {
	var ran []string
	_, tt, st := menuState(t, "First paragraph.\n\nSecond paragraph.\n", &ran)

	rightClickText(t, tt, "First paragraph.")
	want := []string{"Copy", "Select All", "-", "Find", "Refresh", "-", "Reveal in Sidebar", "Open in File Manager", "Copy Path"}
	if got := tt.Menu(); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("menu %v, want %v", got, want)
	}
	if err := tt.ChooseMenuItem("Copy"); err == nil {
		t.Fatal("Copy was choosable without a selection")
	}
	tt.CloseMenu()

	// With a selection the item works and reaches Copy.
	st.SetSelection(selection.Point{Block: 0}, selection.Point{Block: 0, Offset: 5})
	rightClickText(t, tt, "First paragraph.")
	if err := tt.ChooseMenuItem("Copy"); err != nil {
		t.Fatal(err)
	}

	for _, label := range []string{"Select All", "Find", "Refresh", "Reveal in Sidebar", "Open in File Manager", "Copy Path"} {
		rightClickText(t, tt, "First paragraph.")
		if err := tt.ChooseMenuItem(label); err != nil {
			t.Fatal(err)
		}
	}

	wantRan := []string{"copy", "select-all", "find", "refresh", "reveal", "open-in-fm", "copy-path"}
	if strings.Join(ran, "|") != strings.Join(wantRan, "|") {
		t.Fatalf("callbacks %v, want %v", ran, wantRan)
	}
}

// TestDocumentMenuAbsentWithoutActions: a view the shell gave no actions has
// no context menu, as the frontmatter and code copy buttons hide without
// their callbacks.
func TestDocumentMenuAbsentWithoutActions(t *testing.T) {
	m := doc.New("First paragraph.\n")
	var st State
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		ScrollView(c, m, &st)
	}, 400, 300)
	tt.Frame()

	rightClickText(t, tt, "First paragraph.")
	if items := tt.Menu(); len(items) != 0 {
		t.Fatalf("menu %v, want none", items)
	}
}
