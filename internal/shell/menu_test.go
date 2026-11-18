package shell

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// menuItems flattens a menu's whole tree, submenus included.
func menuItems(items []*mygo.MenuItem) []*mygo.MenuItem {
	var out []*mygo.MenuItem
	for _, it := range items {
		out = append(out, it)
		out = append(out, menuItems(it.Submenu)...)
	}
	return out
}

// findMenuItem returns the item labeled label that answers to accel, ""
// for any accelerator: the alias entries share a label and differ in
// their chord only.
func findMenuItem(t *testing.T, m *mygo.Menu, label, accel string) *mygo.MenuItem {
	t.Helper()
	for _, it := range menuItems(m.Items()) {
		if it.Label != label {
			continue
		}
		if accel == "" || it.Accelerator == accel {
			return it
		}
	}
	var have []string
	for _, it := range menuItems(m.Items()) {
		if it.Type == mygo.MenuItemSeparator || it.Label == "" {
			continue
		}
		have = append(have, it.Label+" ("+it.Accelerator+")")
	}
	t.Fatalf("menu has no %q with %q; it has %v", label, accel, have)
	return nil
}

// chooseMenuItem runs an item's Click the way the native menu does, on the
// main thread and with the window the menu belongs to, nil here: the
// handlers repaint through the tester's next frame.
func chooseMenuItem(t *testing.T, m *mygo.Menu, label, accel string) {
	t.Helper()
	it := findMenuItem(t, m, label, accel)
	if it.Click == nil {
		t.Fatalf("%q has no Click", label)
	}
	it.Click(it, nil)
}

// scrollDoc writes a document of blocks paragraphs and opens it.
func scrollDoc(t *testing.T, s *Shell, dir string, blocks int) string {
	t.Helper()
	var b strings.Builder
	for i := 0; i < blocks; i++ {
		b.WriteString("## Heading ")
		b.WriteString(strconv.Itoa(i))
		b.WriteString("\n\nParagraph ")
		b.WriteString(strconv.Itoa(i))
		b.WriteString(" of the long document.\n\n")
	}
	path := filepath.Join(dir, "long.md")
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestMenuAcceleratorsUnique guards the accelerators against a collision:
// a chord the menu registers twice would fire one item only, and which one
// depends on the platform's tables. Items of a role are mygo's own, whose
// chords (macOS's Cmd+Q, for one) are not this app's to keep apart.
func TestMenuAcceleratorsUnique(t *testing.T) {
	s, _ := newShell(t)
	m := BuildMenu(s)
	seen := map[string]string{}
	for _, it := range menuItems(m.Items()) {
		if it.Accelerator == "" || it.Type == mygo.MenuItemSeparator || it.Role != "" {
			continue
		}
		key := it.Accelerator
		// Ctrl and CmdOrCtrl name the same chord away from macOS.
		if runtime.GOOS != "darwin" {
			key = strings.ReplaceAll(key, "CmdOrCtrl+", "Ctrl+")
		}
		if prev, ok := seen[key]; ok {
			t.Fatalf("%s is bound twice: %q and %q", key, prev, it.Label)
		}
		seen[key] = it.Label
	}
}

// TestMenuScrollShortcuts walks Ctrl+Home and Ctrl+End: the first shows the
// document's top, the last its end.
func TestMenuScrollShortcuts(t *testing.T) {
	s, dir := newShell(t)
	scrollDoc(t, s, dir, 120)
	tt := s.renderSize(t, 800, 400)
	tt.Frame()
	m := BuildMenu(s)

	chooseMenuItem(t, m, "Scroll to Bottom", "CmdOrCtrl+End")
	tt.Frame()
	if !s.ActiveTab().State.List.AtEnd() {
		t.Fatal("Ctrl+End did not scroll the document to its end")
	}

	chooseMenuItem(t, m, "Scroll to Top", "CmdOrCtrl+Home")
	tt.Frame()
	if first, _ := s.ActiveTab().State.List.Visible(); first != 0 {
		t.Fatalf("Ctrl+Home left the first visible row at %d, want 0", first)
	}
}

// TestMenuRefreshRereadsTheFile walks F5 and Ctrl+R: both reread the open
// file, and both work with auto refresh off.
func TestMenuRefreshRereadsTheFile(t *testing.T) {
	for _, accel := range []string{"F5"} { // Ctrl+R rides the window shortcuts, walked in shell_test
		s, dir := newShell(t)
		path := filepath.Join(dir, "a.md")
		if err := s.OpenFile(path); err != nil {
			t.Fatal(err)
		}
		tt := s.render(t)
		tt.Frame()
		m := BuildMenu(s)

		if err := os.WriteFile(path, []byte("# A\n\nrewritten on disk\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		chooseMenuItem(t, m, "Refresh", accel)
		tt.Frame()
		if src := s.ActiveTab().Model.Source(); !strings.Contains(src, "rewritten on disk") {
			t.Fatalf("%s did not reread the file: %q", accel, src)
		}
	}
}

// TestMenuCloseTabShortcuts walks both chords that close a tab: Ctrl+W
// closes one, Ctrl+F4 the window shortcut the next.
func TestMenuCloseTabShortcuts(t *testing.T) {
	s, dir := newShell(t)
	s.OpenPaths([]string{filepath.Join(dir, "a.md"), filepath.Join(dir, "b.md")})
	tt := s.render(t)
	tt.Frame()
	m := BuildMenu(s)

	chooseMenuItem(t, m, "Close Tab", "CmdOrCtrl+W")
	tt.Frame()
	if len(s.Tabs) != 1 || filepath.Base(s.Tabs[0].Path) != "a.md" {
		t.Fatalf("Ctrl+W left %d tabs: %v", len(s.Tabs), s.Tabs)
	}

	tt.TypeKey(ui.Ctrl, ui.KeyF4, "")
	tt.Frame()
	if len(s.Tabs) != 0 {
		t.Fatalf("Ctrl+F4 left %d tabs", len(s.Tabs))
	}
}

// TestMenuSidebarToggles walks Ctrl+B and Ctrl+Shift+E: each flips its own
// panel and leaves the other one alone.
func TestMenuSidebarToggles(t *testing.T) {
	s, dir := newShell(t)
	if err := s.OpenFile(filepath.Join(dir, "a.md")); err != nil {
		t.Fatal(err)
	}
	tt := s.render(t)
	tt.Frame()
	m := BuildMenu(s)

	s.Outline.Visible = false
	s.Sidebar.Visible = false
	chooseMenuItem(t, m, "Toggle Outline", "CmdOrCtrl+B")
	tt.Frame()
	if !s.Outline.Visible || s.Sidebar.Visible {
		t.Fatalf("Ctrl+B: outline %v files %v, want only the outline", s.Outline.Visible, s.Sidebar.Visible)
	}
	chooseMenuItem(t, m, "Toggle Outline", "CmdOrCtrl+B")
	tt.Frame()
	if s.Outline.Visible {
		t.Fatal("Ctrl+B again did not hide the outline")
	}

	chooseMenuItem(t, m, "Toggle Files", "CmdOrCtrl+Shift+E")
	tt.Frame()
	if !s.Sidebar.Visible || s.Outline.Visible {
		t.Fatalf("Ctrl+Shift+E: files %v outline %v, want only the files", s.Sidebar.Visible, s.Outline.Visible)
	}
	chooseMenuItem(t, m, "Toggle Files", "CmdOrCtrl+Shift+E")
	tt.Frame()
	if s.Sidebar.Visible {
		t.Fatal("Ctrl+Shift+E again did not hide the files")
	}
}

// TestMenuFindStaysOnCtrlF guards the chord the search bar already had.
func TestMenuFindStaysOnCtrlF(t *testing.T) {
	s, dir := newShell(t)
	if err := s.OpenFile(filepath.Join(dir, "a.md")); err != nil {
		t.Fatal(err)
	}
	tt := s.render(t)
	m := BuildMenu(s)
	chooseMenuItem(t, m, "Find", "CmdOrCtrl+F")
	tt.Frame()
	if !s.SearchOpen {
		t.Fatal("Find did not open the search bar")
	}
}

// visibleItems drops the separators of a native menu's list.
func visibleItems(items []*mygo.MenuItem) []*mygo.MenuItem {
	out := make([]*mygo.MenuItem, 0, len(items))
	for _, it := range items {
		if it.Type != mygo.MenuItemSeparator {
			out = append(out, it)
		}
	}
	return out
}

// TestMenuSpecsAreTheOnlySource guards the two renderings of the app menu
// against drifting apart: the native menu bar (BuildMenu) and the title
// bar's menu button (drawMenu) are the same specs, so their labels,
// orders and chords have to match item for item.
func TestMenuSpecsAreTheOnlySource(t *testing.T) {
	s, _ := newShell(t)
	native := BuildMenu(s)

	var walk func(specs []MenuSpec, items []*mygo.MenuItem, path string)
	walk = func(specs []MenuSpec, items []*mygo.MenuItem, path string) {
		t.Helper()
		shown := visibleItems(items)
		var want []MenuSpec
		for _, sp := range specs {
			if !sp.Sep {
				want = append(want, sp)
			}
		}
		if len(shown) != len(want) {
			t.Fatalf("%s: %d items in the menu, %d in the specs", path, len(shown), len(want))
		}
		for i, sp := range want {
			it := shown[i]
			where := path + " > " + sp.Label
			if it.Label != sp.Label {
				t.Fatalf("%s: the menu shows %q at %d", where, it.Label, i)
			}
			if it.Accelerator != sp.Accel {
				t.Fatalf("%s: the menu registers %q, the specs %q", where, it.Accelerator, sp.Accel)
			}
			if len(sp.Sub) > 0 {
				walk(sp.Sub, it.Submenu, where)
				continue
			}
			if it.Click == nil {
				t.Fatalf("%s: the menu has no Click", where)
			}
			if sp.Run == nil {
				t.Fatalf("%s: the specs have no Run", where)
			}
			// The drawn entry prints the chord from the same string, so a
			// chord only the native menu understands would go missing.
			if sp.Accel == "" {
				continue
			}
			if _, _, ok := shortcut(sp.Accel); !ok {
				t.Fatalf("%s: the drawn menu cannot show the chord %q", where, sp.Accel)
			}
		}
	}
	walk(MenuSpecs(s), topItems(native.Items()), "menu")
}

// topItems drops the role items the native bar carries and the specs do
// not: macOS's application menu, which mygo builds from the role.
func topItems(items []*mygo.MenuItem) []*mygo.MenuItem {
	out := make([]*mygo.MenuItem, 0, len(items))
	for _, it := range items {
		if it.Role == "" {
			out = append(out, it)
		}
	}
	return out
}

// TestMenuTopLevelStructure pins the bar's shape: three menus, then
// Settings and Exit as items of their own, as the reference menu shows them.
func TestMenuTopLevelStructure(t *testing.T) {
	s, _ := newShell(t)
	var got []string
	for _, sp := range MenuSpecs(s) {
		if sp.Sep {
			got = append(got, "-")
			continue
		}
		got = append(got, sp.Label)
		if sp.Label == "Peekdown" {
			t.Fatal("the bar has a Peekdown submenu again")
		}
	}
	want := []string{"File", "Edit", "View", "Help", "-", "Settings...", "-", "Exit"}
	if !slices.Equal(got, want) {
		t.Fatalf("the bar shows %v, want %v", got, want)
	}
	// Settings is a top-level item, and the drawn menu prints its chord
	// from the same string the native one registers.
	for _, sp := range MenuSpecs(s) {
		if sp.Sep {
			continue
		}
		if len(sp.Sub) > 0 {
			continue
		}
		switch sp.Label {
		case "Settings...":
			if sp.Accel != "CmdOrCtrl+," {
				t.Fatalf("Settings... has %q, want CmdOrCtrl+,", sp.Accel)
			}
		case "Exit":
			if sp.Accel != "CmdOrCtrl+Q" {
				t.Fatalf("Exit has %q, want CmdOrCtrl+Q", sp.Accel)
			}
		}
		if sp.Run == nil {
			t.Fatalf("%q has no Run", sp.Label)
		}
	}
}

// TestMenuExitClosesTheWindow walks the Exit item: the app is one window,
// so the command is the host's close.
func TestMenuExitClosesTheWindow(t *testing.T) {
	s, _ := newShell(t)
	closed := false
	s.CloseWindow = func() { closed = true }
	chooseMenuItem(t, BuildMenu(s), "Exit", "CmdOrCtrl+Q")
	if !closed {
		t.Fatal("Exit did not close the window")
	}
}

// TestMenuHelpOpensTheProjectPages walks the Help menu: both items hand
// the browser the project's page.
func TestMenuHelpOpensTheProjectPages(t *testing.T) {
	s, _ := newShell(t)
	var opened []string
	s.OpenURL = func(url string) { opened = append(opened, url) }
	m := BuildMenu(s)
	chooseMenuItem(t, m, "Peekdown on GitHub", "")
	chooseMenuItem(t, m, "Keyboard Shortcuts", "")
	want := []string{repoURL, shortcutsURL}
	if !slices.Equal(opened, want) {
		t.Fatalf("the Help menu opened %v, want %v", opened, want)
	}
}

// TestMenuButtonDrawsTheAppMenu walks the title bar's entry into the same
// commands: a menu button named Main menu sits in the bar, it opens the
// native menu's tree, and choosing an item runs the command its chord
// would.
func TestMenuButtonDrawsTheAppMenu(t *testing.T) {
	s, dir := newShell(t)
	s.Outline.Visible = false
	if err := s.OpenFile(filepath.Join(dir, "a.md")); err != nil {
		t.Fatal(err)
	}
	tt := s.renderSize(t, 900, 600)
	tt.SetTitleBar(ui.TitleBar{Height: TitleBarHeight, Right: 138})
	tt.Frame()
	if _, ok := tt.AccessByRole(ui.RoleMenuButton, "Main menu"); !ok {
		t.Fatalf("texts %v: want the bar's menu button", tt.Texts())
	}

	if err := tt.Click("Main menu"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	var want []string
	for _, it := range visibleItems(BuildMenu(s).Items()) {
		want = append(want, it.Label)
	}
	var got []string
	for _, label := range tt.Menu() {
		if label != "-" {
			got = append(got, label)
		}
	}
	if !slices.Equal(got, want) {
		t.Fatalf("the bar's menu shows %v, the native one %v", got, want)
	}

	if err := tt.ChooseMenuItem("View", "Toggle Outline"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !s.Outline.Visible {
		t.Fatal("Toggle Outline from the bar did not show the right panel")
	}
}

// TestMenuButtonOpensSettings walks the bar's menu down to Settings: the
// item is on the bar's top level, and choosing it opens the dialog its
// chord opens.
func TestMenuButtonOpensSettings(t *testing.T) {
	s, dir := newShell(t)
	if err := s.OpenFile(filepath.Join(dir, "a.md")); err != nil {
		t.Fatal(err)
	}
	tt := s.renderSize(t, 900, 600)
	tt.SetTitleBar(ui.TitleBar{Height: TitleBarHeight, Right: 138})
	tt.Frame()
	if err := tt.Click("Main menu"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	var want []string
	for _, sp := range MenuSpecs(s) {
		if sp.Sep {
			want = append(want, "-")
			continue
		}
		want = append(want, sp.Label)
	}
	// The separators are the tree's too: Settings and Exit sit on the
	// menu, not in a submenu of the app's name.
	if got := tt.Menu(); !slices.Equal(got, want) {
		t.Fatalf("the bar's menu shows %v, the specs %v", got, want)
	}
	if err := tt.ChooseMenuItem("Settings..."); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !s.SettingsOpen {
		t.Fatal("Settings... from the bar did not open the dialog")
	}
}

// TestShortcutParsesTheMenuChords pins the translation the drawn menu
// needs: each chord of the app menu becomes the modifiers and key the
// native menu and the window shortcuts register for it.
func TestShortcutParsesTheMenuChords(t *testing.T) {
	for _, tc := range []struct {
		accel string
		mods  ui.Modifiers
		key   ui.Key
		ok    bool
	}{
		{"CmdOrCtrl+O", ui.Cmd, ui.KeyO, true},
		{"CmdOrCtrl+Shift+Tab", ui.Cmd | ui.Shift, ui.KeyTab, true},
		{"Shift+F3", ui.Shift, ui.KeyF3, true},
		{"F5", 0, ui.KeyF5, true},
		{"CmdOrCtrl+=", ui.Cmd, ui.KeyEqual, true},
		{"CmdOrCtrl+-", ui.Cmd, ui.KeyMinus, true},
		{"CmdOrCtrl+,", ui.Cmd, ui.KeyComma, true},
		{"CmdOrCtrl+0", ui.Cmd, ui.Key0, true},
		{"CmdOrCtrl+Home", ui.Cmd, ui.KeyHome, true},
		{"Ctrl+Alt+Delete", ui.Ctrl | ui.Alt, ui.KeyDelete, true},
		{"Hyper+O", 0, 0, false},
		{"CmdOrCtrl+Nonesuch", 0, 0, false},
		{"", 0, 0, false},
	} {
		mods, key, ok := shortcut(tc.accel)
		if ok != tc.ok || mods != tc.mods || key != tc.key {
			t.Fatalf("%q: got %v %v %v, want %v %v %v", tc.accel, mods, key, ok, tc.mods, tc.key, tc.ok)
		}
	}
}
