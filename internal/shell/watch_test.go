package shell

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/windedge/peekdown/internal/doc"
	"github.com/windedge/peekdown/internal/selection"
	"github.com/windedge/peekdown/internal/settings"
)

// reloadTab is testable without a watch: the watcher goroutine needs the
// mygo runtime, but the reload it performs is a plain shell call.
func TestReloadTabRereadsFile(t *testing.T) {
	s, dir := newShell(t)
	path := filepath.Join(dir, "a.md")
	if err := s.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	tab := s.ActiveTab()
	if tab.Model.Plain(1) != "A" {
		t.Fatalf("heading before reload: %q", tab.Model.Plain(1))
	}
	// An editor's atomic save: replace the file, then reload.
	newer := "# A rewritten\n"
	if err := os.WriteFile(path, []byte(newer), 0o644); err != nil {
		t.Fatal(err)
	}
	if !s.reloadTab(tab) {
		t.Fatal("reloadTab = false, want true")
	}
	if tab.Model.Source() != newer {
		t.Fatalf("model source after reload: %q", tab.Model.Source())
	}
	if got := tab.Model.Plain(1); got != "A rewritten" {
		t.Fatalf("heading after reload: %q", got)
	}
	// Search state drops on reload: the matches name text the new
	// document need not have.
	tab.Model.SetSearch("rewritten", doc.SearchOptions{})
	if len(tab.Model.Matches) == 0 {
		t.Fatal("search found nothing before the reload")
	}
	// The file changes again on disk; the reload brings the new content
	// and the search drops with it.
	final := "# A final\n"
	if err := os.WriteFile(path, []byte(final), 0o644); err != nil {
		t.Fatal(err)
	}
	if !s.reloadTab(tab) {
		t.Fatal("second reloadTab = false, want true (content changed)")
	}
	if len(tab.Model.Matches) != 0 {
		t.Fatalf("search survived reload: %d matches", len(tab.Model.Matches))
	}
}

func TestReloadTabUnchangedOrMissing(t *testing.T) {
	s, dir := newShell(t)
	path := filepath.Join(dir, "a.md")
	if err := s.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	tab := s.ActiveTab()
	if s.reloadTab(tab) {
		t.Fatal("reload with unchanged file = true, want false")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if s.reloadTab(tab) {
		t.Fatal("reload with file gone = true, want false")
	}
	if s.reloadTab(nil) {
		t.Fatal("reload of nil tab = true, want false")
	}
}

// TestSettingsSwitchAppliesAutoRefresh walks the dialog's auto refresh
// switch: the click flips the draft, Save reaches the saved settings and
// starts or stops the watcher as Apply does.
func TestSettingsSwitchAppliesAutoRefresh(t *testing.T) {
	s, dir := newShell(t)
	if err := s.OpenFile(filepath.Join(dir, "a.md")); err != nil {
		t.Fatal(err)
	}
	s.Saved.AutoRefresh = false
	s.applyAutoRefresh()
	tt := s.renderSize(t, 800, 1000)

	s.OpenSettings()
	tt.Frame()
	if !tt.HasText("Auto Refresh") {
		t.Fatalf("dialog missing the auto refresh switch: %v", tt.Texts())
	}
	sw, ok := tt.AccessByRole(ui.RoleSwitch, "Auto Refresh")
	if !ok {
		t.Fatal("no switch labeled Auto Refresh")
	}
	if sw.Checked {
		t.Fatal("switch on before the click, want off")
	}

	if err := tt.Click("Auto Refresh"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !s.Draft.AutoRefresh {
		t.Fatal("click did not flip the draft")
	}
	if err := tt.Click("Save"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if s.SettingsOpen || !s.Saved.AutoRefresh {
		t.Fatalf("after save: open %v, auto refresh %v", s.SettingsOpen, s.Saved.AutoRefresh)
	}
	if s.watchCancel == nil {
		t.Fatal("watcher not started after save")
	}

	// Turning it back off stops the watcher again.
	s.OpenSettings()
	tt.Frame()
	if err := tt.Click("Auto Refresh"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("Save"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if s.Saved.AutoRefresh || s.watchCancel != nil {
		t.Fatalf("after save: auto refresh %v, watcher running %v", s.Saved.AutoRefresh, s.watchCancel != nil)
	}
}

func TestApplyTogglesAutoRefresh(t *testing.T) {
	s, dir := newShell(t)
	if err := s.OpenFile(filepath.Join(dir, "a.md")); err != nil {
		t.Fatal(err)
	}
	s.Saved.AutoRefresh = true
	s.applyAutoRefresh()
	if s.watchCancel == nil {
		t.Fatal("watcher not started with auto refresh on")
	}
	v := settings.Default()
	v.AutoRefresh = false
	if err := s.Apply(v); err != nil {
		t.Fatal(err)
	}
	if s.watchCancel != nil {
		t.Fatal("watcher still running after Apply turned auto refresh off")
	}
	v.AutoRefresh = true
	if err := s.Apply(v); err != nil {
		t.Fatal(err)
	}
	if s.watchCancel == nil {
		t.Fatal("watcher not restarted by Apply")
	}
	s.stopWatch()
}

func TestCopySelectionWritesClipboard(t *testing.T) {
	s, dir := newShell(t)
	if err := s.OpenFile(filepath.Join(dir, "a.md")); err != nil {
		t.Fatal(err)
	}
	tab := s.ActiveTab()
	var got string
	// No selection: Copy reports nothing to write.
	if s.CopySelection(func(s string) { got = s }) {
		t.Fatal("CopySelection without a selection = true, want false")
	}
	// A selection across the first two run blocks: the heading block's
	// plain text is "A" ("#" is syntax), the paragraph's "first". The
	// run table joins them with a newline.
	tab.State.SetSelection(
		selection.Point{Block: 1, Offset: 0},
		selection.Point{Block: 2, Offset: 5},
	)
	if !s.CopySelection(func(s string) { got = s }) {
		t.Fatal("CopySelection = false, want true")
	}
	if got != "A\nfirst" {
		t.Fatalf("clipboard = %q, want %q", got, "A\nfirst")
	}
}

func TestSelectAllSelectsWholeDocument(t *testing.T) {
	s, dir := newShell(t)
	// a.md has two run blocks ("# A" -> "A" and "first"); b.md stands in
	// for a second file the pane could show.
	if err := s.OpenFile(filepath.Join(dir, "a.md")); err != nil {
		t.Fatal(err)
	}
	if !s.SelectAll() {
		t.Fatal("SelectAll = false, want true")
	}
	tab := s.ActiveTab()
	if !tab.State.HasSelection() {
		t.Fatal("no selection after SelectAll")
	}
	if got := tab.State.SelectionText(tab.Model); got != "A\nfirst\nA two" {
		t.Fatalf("SelectionText = %q, want %q", got, "A\nfirst\nA two")
	}
}

func TestSelectAllWithoutTab(t *testing.T) {
	s, _ := newShell(t)
	if s.SelectAll() {
		t.Fatal("SelectAll without tabs = true, want false")
	}
}

// Explorer root and sort: the pane's inputs are plain functions, so the
// tests check them directly; the rendering path is the same as before.

func TestFindProjectRoot(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "proj")
	deep := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	// The marker list trims and skips blanks; .git here comes from the
	// list itself, not any hardcoded VCS set.
	if got := findProjectRoot(deep, []string{"go.mod", " package.json ", ".git"}); got != root {
		t.Fatalf("findProjectRoot = %q, want %q", got, root)
	}
	if got := findProjectRoot(deep, nil); got != "" {
		t.Fatalf("findProjectRoot without markers = %q, want empty", got)
	}
}

func TestExplorerRootByMode(t *testing.T) {
	s, _ := newShell(t)
	saved := settings.Default()
	saved.ExplorerRootMode = "project_root"
	saved.ProjectRootMarkers = []string{".git"}
	s.Saved = saved

	dir := t.TempDir()
	root := filepath.Join(dir, "proj")
	sub := filepath.Join(root, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := s.explorerRoot(filepath.Join(sub, "doc.md")); got != root {
		t.Fatalf("explorerRoot = %q, want %q", got, root)
	}
	s.Saved.ExplorerRootMode = "current_dir"
	if got := s.explorerRoot(filepath.Join(sub, "doc.md")); got != sub {
		t.Fatalf("explorerRoot in current_dir mode = %q, want %q", got, sub)
	}
	// No marker above: fall back to the document's directory.
	plain := filepath.Join(dir, "plain")
	if err := os.MkdirAll(plain, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := s.explorerRoot(filepath.Join(plain, "doc.md")); got != plain {
		t.Fatalf("explorerRoot without a root = %q, want %q", got, plain)
	}
}

func TestReadSortedEntries(t *testing.T) {
	dir := t.TempDir()
	names := []string{"c.md", "a.md", "b.md", "sub"}
	for _, n := range names {
		if n == "sub" {
			if err := os.MkdirAll(filepath.Join(dir, n), 0o755); err != nil {
				t.Fatal(err)
			}
			// The directory predates every file, whatever the clock did
			// between the writes above.
			old := time.Now().Add(-time.Hour)
			if err := os.Chtimes(filepath.Join(dir, n), old, old); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.WriteFile(filepath.Join(dir, n), []byte(n), 0o644); err != nil {
			t.Fatal(err)
		}
		// Distinct modification times, oldest first.
		stamp := time.Now().Add(time.Duration(slices.Index(names, n)) * time.Minute)
		if err := os.Chtimes(filepath.Join(dir, n), stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	wantNames := func(entries []os.DirEntry) []string {
		out := make([]string, len(entries))
		for i, e := range entries {
			out[i] = e.Name()
		}
		return out
	}
	got, err := readSortedEntries(dir, "name_asc")
	if err != nil {
		t.Fatal(err)
	}
	if got := wantNames(got); got[0] != "sub" || got[1] != "a.md" || got[3] != "c.md" {
		t.Fatalf("name_asc = %v", got)
	}
	got, err = readSortedEntries(dir, "name_desc")
	if err != nil {
		t.Fatal(err)
	}
	if got := wantNames(got); got[0] != "sub" || got[1] != "c.md" || got[3] != "a.md" {
		t.Fatalf("name_desc = %v", got)
	}
	got, err = readSortedEntries(dir, "time_asc")
	if err != nil {
		t.Fatal(err)
	}
	if got := wantNames(got); got[0] != "sub" || got[1] != "c.md" || got[3] != "b.md" {
		t.Fatalf("time_asc = %v", got)
	}
	got, err = readSortedEntries(dir, "time_desc")
	if err != nil {
		t.Fatal(err)
	}
	// Directories first holds in the time sorts too: sub is the oldest
	// entry and still opens the list in time_desc.
	if got := wantNames(got); got[0] != "sub" || got[1] != "b.md" || got[2] != "a.md" || got[3] != "c.md" {
		t.Fatalf("time_desc = %v", got)
	}
	// An unknown mode falls back to sorting by name.
	got, err = readSortedEntries(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if got := wantNames(got); got[1] != "a.md" {
		t.Fatalf("default sort = %v", got)
	}
}
