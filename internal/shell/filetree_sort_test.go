package shell

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/settings"
	"github.com/windedge/peekdown/internal/testshot"
	"github.com/windedge/peekdown/locales"
)

// openSortMenu checks the menu Rust's explorer shows: a title, the four
// modes, a separator between the name and time groups, and one check.
func openSortMenu(t *testing.T, tt *ui.Tester, mode string) {
	t.Helper()
	checked := explorerSortLabel(mode)
	tip := locales.SidebarSortTooltip(checked)
	if err := tt.Click(tip); err != nil {
		t.Fatalf("sort button %q: %v", tip, err)
	}
	tt.Frame()
	want := []string{
		locales.SidebarSortBy(), "-",
		locales.SidebarSortNameAsc(), locales.SidebarSortNameDesc(),
		locales.SidebarSortTimeNewest(), locales.SidebarSortTimeOldest(),
	}
	if got := tt.Menu(); !slices.Equal(got, want) {
		t.Fatalf("sort menu = %v, want %v", got, want)
	}
	for _, label := range want[2:] {
		item, ok := tt.AccessByRole(ui.RoleMenuItemCheckBox, label)
		if !ok {
			t.Fatalf("sort item %q has no check state", label)
		}
		if got := item.Checked; got != (label == checked) {
			t.Fatalf("sort item %q checked = %v, want %v", label, got, label == checked)
		}
	}
}

// saveToDisk makes persist write the real settings file, as the host does.
func saveToDisk(s *Shell, path string) {
	s.Save = func(v settings.Settings) error {
		data, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return err
		}
		return os.WriteFile(path, data, 0o644)
	}
}

func savedSortMode(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var v settings.Settings
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatal(err)
	}
	return v.ExplorerSortMode
}

// TestFileTreeSortMenuChoosesMode walks the menu from the header: clicking
// Name (Z-A) changes the saved mode, reverses the tree and lands on disk.
func TestFileTreeSortMenuChoosesMode(t *testing.T) {
	s, dir := newShell(t)
	proj := treeFixture(t, dir)
	if err := os.WriteFile(filepath.Join(proj, "b.md"), []byte("# B\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	settingsPath := filepath.Join(dir, "settings.json")
	saveToDisk(s, settingsPath)
	tt := openTree(t, s, proj)
	wantRows(t, proj, s.tree.rows, "a", "b.md", "root.md")
	openSortMenu(t, tt, "name_asc")

	if err := tt.ChooseMenuItem(locales.SidebarSortNameDesc()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := s.Saved.ExplorerSortMode; got != "name_desc" {
		t.Fatalf("mode = %q, want name_desc", got)
	}
	wantRows(t, proj, s.tree.rows, "a", "root.md", "b.md")
	if got := savedSortMode(t, settingsPath); got != "name_desc" {
		t.Fatalf("settings.json mode = %q, want name_desc", got)
	}
}

// TestFileTreeSortMenuTimeModes checks that choosing the time menu item
// rebuilds the tree in mtime order, newest first, and persists the value.
func TestFileTreeSortMenuTimeModes(t *testing.T) {
	s, dir := newShell(t)
	proj := treeFixture(t, dir)
	settingsPath := filepath.Join(dir, "settings.json")
	saveToDisk(s, settingsPath)
	base := time.Now().Add(-48 * time.Hour)
	for _, st := range []struct {
		name string
		at   time.Time
	}{
		{"old.md", base},
		{"root.md", base.Add(24 * time.Hour)},
		{"new.md", base.Add(47 * time.Hour)},
	} {
		path := filepath.Join(proj, st.name)
		if st.name != "root.md" {
			if err := os.WriteFile(path, []byte("# "+st.name+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.Chtimes(path, st.at, st.at); err != nil {
			t.Fatal(err)
		}
	}
	tt := openTree(t, s, proj)
	openSortMenu(t, tt, "name_asc")

	if err := tt.ChooseMenuItem(locales.SidebarSortTimeNewest()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := s.Saved.ExplorerSortMode; got != "time_desc" {
		t.Fatalf("mode = %q, want time_desc", got)
	}
	wantRows(t, proj, s.tree.rows, "a", "new.md", "root.md", "old.md")
	if got := savedSortMode(t, settingsPath); got != "time_desc" {
		t.Fatalf("settings.json mode = %q, want time_desc", got)
	}

	openSortMenu(t, tt, "time_desc")
	if err := tt.ChooseMenuItem(locales.SidebarSortTimeOldest()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := s.Saved.ExplorerSortMode; got != "time_asc" {
		t.Fatalf("mode = %q, want time_asc", got)
	}
	wantRows(t, proj, s.tree.rows, "a", "old.md", "root.md", "new.md")
}

// TestFileTreeSortCaseInsensitive pins Rust's lowercase comparison: Z.md
// must not sort ahead of a.md just because its first letter is capital.
func TestFileTreeSortCaseInsensitive(t *testing.T) {
	s, dir := newShell(t)
	proj := treeFixture(t, dir)
	for _, name := range []string{"Z.md", "a.md"} {
		if err := os.WriteFile(filepath.Join(proj, name), []byte("# "+name+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	_ = openTree(t, s, proj)
	wantRows(t, proj, s.tree.rows, "a", "a.md", "root.md", "Z.md")

	entries, err := readSortedEntries(proj, "name_desc")
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if IsMarkdown(e.Name()) {
			names = append(names, e.Name())
		}
	}
	if want := []string{"Z.md", "root.md", "a.md"}; !slices.Equal(names, want) {
		t.Fatalf("name_desc = %v, want %v", names, want)
	}
}

// TestFileTreeSortMenuScreenshot writes the menu's expanded state for a
// human to check the header button and the popup's place in the pane.
func TestFileTreeSortMenuScreenshot(t *testing.T) {
	if os.Getenv("PEEKDOWN_SHOT") == "" {
		t.Skip("set PEEKDOWN_SHOT=1 to write the sort menu screenshot")
	}
	s, dir := newShell(t)
	proj := treeFixture(t, dir)
	saved := s.Saved
	saved.ExplorerRootMode = "project_root"
	saved.ProjectRootMarkers = []string{"go.mod"}
	saved.ExplorerSortMode = "name_asc"
	saved.ExpandedDirs = nil
	s.Saved = saved
	if err := s.OpenFile(filepath.Join(proj, "root.md")); err != nil {
		t.Fatal(err)
	}
	s.Sidebar.Visible = true
	s.SetFilesVisible(true)
	tt := s.renderSize(t, 1000, 700)
	tt.Frame()
	openSortMenu(t, tt, "name_asc")
	testshot.WritePng(t, tt.Image(), "C:/tmp/filetree-sort-menu.png")
}
