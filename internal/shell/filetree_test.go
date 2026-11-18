package shell

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// treeFixture writes a small project under dir/proj and returns its root:
//
//	proj/
//	  go.mod        the project root marker
//	  root.md
//	  a/
//	    y.md
//	    b/
//	      x.md
//	      pic.png
//	  .hidden/
//	    .secret.md
func treeFixture(t *testing.T, dir string) string {
	t.Helper()
	proj := filepath.Join(dir, "proj")
	for _, d := range []string{"a/b", ".hidden"} {
		if err := os.MkdirAll(filepath.Join(proj, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// Non-markdown files stay out of the tree, but the fixture keeps
	// go.mod: the project root marker the settings match against.
	files := map[string]string{
		"go.mod":             "module proj\n",
		"root.md":            "# Root\n",
		"a/y.md":             "# Y\n",
		"a/b/x.md":           "# X\n",
		"a/b/pic.png":        "png",
		"a/b/data.ps1":       "echo",
		"a/b/build.log":      "log",
		".hidden/.secret.md": "# hidden\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(proj, filepath.FromSlash(name)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(proj, "a", "b", "pic.png"), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	return proj
}

// openTree opens proj/root.md with the project root mode on and the
// sidebar showing the explorer, so the tree hangs from proj.
func openTree(t *testing.T, s *Shell, proj string) *ui.Tester {
	t.Helper()
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
	tt := s.render(t)
	tt.Frame()
	return tt
}

// treeRows names the rows relative to root, so a Windows path separator
// never reaches the expectations.
func treeRows(root string, rows []treeRow) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		rel, err := filepath.Rel(root, r.Path)
		if err != nil {
			rel = r.Path
		}
		out[i] = filepath.ToSlash(rel)
	}
	return out
}

func wantRows(t *testing.T, root string, rows []treeRow, want ...string) {
	t.Helper()
	got := treeRows(root, rows)
	if !slices.Equal(got, want) {
		t.Fatalf("rows = %v, want %v", got, want)
	}
}

// TestFileTreeStartsCollapsedAndExpands walks a click on a directory:
// the root's entries show first, the directory's own once clicked, and
// the expanded set reaches the settings.
func TestFileTreeStartsCollapsedAndExpands(t *testing.T) {
	s, dir := newShell(t)
	proj := treeFixture(t, dir)
	tt := openTree(t, s, proj)

	// treeFixture files a go.mod the filter leaves out: it is the root
	// marker, not a document the pane lists.
	wantRows(t, proj, s.tree.rows, "a", "root.md")
	if len(s.Saved.ExpandedDirs) != 0 {
		t.Fatalf("expanded = %v, want none", s.Saved.ExpandedDirs)
	}

	if err := tt.Click("a"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	wantRows(t, proj, s.tree.rows, "a", "a/b", "a/y.md", "root.md")
	if s.tree.rows[1].Depth != 1 || s.tree.rows[2].Depth != 1 {
		t.Fatalf("child depths = %d, %d, want 1", s.tree.rows[1].Depth, s.tree.rows[2].Depth)
	}
	if want := []string{filepath.Join(proj, "a")}; !slices.Equal(s.Saved.ExpandedDirs, want) {
		t.Fatalf("expanded = %v, want %v", s.Saved.ExpandedDirs, want)
	}

	// Hidden entries stay out of the tree at every level.
	if err := tt.Click("a/b"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if slices.Contains(treeRows(proj, s.tree.rows), ".hidden") {
		t.Fatalf("hidden directory listed: %v", treeRows(proj, s.tree.rows))
	}

	// Clicking the open directory again collapses it, and its children
	// leave the tree with the expanded set.
	if err := tt.Click("a"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	wantRows(t, proj, s.tree.rows, "a", "root.md")
	if want := []string{filepath.Join(proj, "a", "b")}; !slices.Equal(s.Saved.ExpandedDirs, want) {
		t.Fatalf("expanded after collapse = %v, want %v", s.Saved.ExpandedDirs, want)
	}
}

// TestFileTreeRestoresExpandedFromSettings rebuilds the shell over the
// settings the pane wrote: the directories open again on startup.
func TestFileTreeRestoresExpandedFromSettings(t *testing.T) {
	s, dir := newShell(t)
	proj := treeFixture(t, dir)
	tt := openTree(t, s, proj)
	if err := tt.Click("a"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	saved := s.Saved

	s2, _ := newShell(t)
	s2.Saved = saved
	if err := s2.OpenFile(filepath.Join(proj, "root.md")); err != nil {
		t.Fatal(err)
	}
	s2.Sidebar.Visible = true
	s2.SetFilesVisible(true)
	tt2 := s2.render(t)
	tt2.Frame()
	wantRows(t, proj, s2.tree.rows, "a", "a/b", "a/y.md", "root.md")
}

// TestFileTreeSortModes runs the four modes through the tree: the order
// follows the setting, directories first in every one of them.
func TestFileTreeSortModes(t *testing.T) {
	s, dir := newShell(t)
	proj := treeFixture(t, dir)
	// Distinct modification times: the directories oldest, then the
	// files from oldest to newest, so no mode can pass by accident.
	base := time.Now().Add(-48 * time.Hour)
	stamps := []struct {
		name string
		at   time.Duration
	}{
		{"a", 0},
		{"root.md", 2 * time.Hour},
		{"c.md", 1 * time.Hour},
		{"d", 3 * time.Hour},
	}
	for _, st := range stamps {
		path := filepath.Join(proj, st.name)
		if st.name == "c.md" {
			if err := os.WriteFile(path, []byte("# C\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		} else if st.name == "d" {
			if err := os.MkdirAll(path, 0o755); err != nil {
				t.Fatal(err)
			}
		}
		at := base.Add(st.at)
		if err := os.Chtimes(path, at, at); err != nil {
			t.Fatal(err)
		}
	}
	tt := openTree(t, s, proj)

	rows := func() []string { return treeRows(proj, s.tree.rows) }
	// name_asc: directories first by name, then the files.
	if got := rows(); !slices.Equal(got, []string{"a", "d", "c.md", "root.md"}) {
		t.Fatalf("name_asc = %v", got)
	}
	s.Saved.ExplorerSortMode = "name_desc"
	tt.Frame()
	if got := rows(); !slices.Equal(got, []string{"d", "a", "root.md", "c.md"}) {
		t.Fatalf("name_desc = %v", got)
	}
	s.Saved.ExplorerSortMode = "time_asc"
	tt.Frame()
	if got := rows(); !slices.Equal(got, []string{"a", "d", "c.md", "root.md"}) {
		t.Fatalf("time_asc = %v", got)
	}
	s.Saved.ExplorerSortMode = "time_desc"
	tt.Frame()
	if got := rows(); !slices.Equal(got, []string{"d", "a", "root.md", "c.md"}) {
		t.Fatalf("time_desc = %v", got)
	}
}

// TestFileTreeFiltersNonMarkdown checks the pane's filter: only the
// directories and the Markdown files make the rows, whatever else the
// directory holds.
func TestFileTreeFiltersNonMarkdown(t *testing.T) {
	s, dir := newShell(t)
	proj := treeFixture(t, dir)
	tt := openTree(t, s, proj)
	if err := tt.Click("a"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("a/b"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	wantRows(t, proj, s.tree.rows, "a", "a/b", "a/b/x.md", "a/y.md", "root.md")
	got := treeRows(proj, s.tree.rows)
	for _, gone := range []string{"a/b/pic.png", "a/b/data.ps1", "a/b/build.log"} {
		if slices.Contains(got, gone) {
			t.Fatalf("non-markdown %s listed: %v", gone, got)
		}
	}
}

// TestFileTreeIndentColumns checks the row layout through the rendered
// boxes: every row starts at the pane's 12 DIP padding, and one depth
// more indents 12 DIPs more, so the names of depth 0 and 1 sit 12 apart.
func TestFileTreeIndentColumns(t *testing.T) {
	s, dir := newShell(t)
	proj := treeFixture(t, dir)
	tt := openTree(t, s, proj)
	if err := tt.Click("a"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	a, okA := tt.Find("a")
	y, okY := tt.Find("a/y.md")
	if !okA || !okY {
		t.Fatalf("rows not rendered: %v", tt.Texts())
	}
	if got := y.X - a.X; got != 12 {
		t.Fatalf("indent between depth 0 and 1 = %v, want 12", got)
	}
}

// TestTruncatePathMiddle checks the root row's middle truncation: the
// result never passes maxChars, keeps both ends and shows the ellipsis
// in the middle. A short path, and a separator too close to the tail,
// pass through whole.
func TestTruncatePathMiddle(t *testing.T) {
	long := `C:\Users\someone\Documents\Deep\Nest\Reports\proj`
	got := truncatePathMiddle(long, 20)
	if len([]rune(got)) > 20 {
		t.Fatalf("truncate = %q, %d runes over the 20 cap", got, len([]rune(got)))
	}
	if !strings.HasPrefix(got, "C:") || !strings.HasSuffix(got, "proj") {
		t.Fatalf("truncate = %q, want the ends kept", got)
	}
	if !strings.Contains(got, "...") {
		t.Fatalf("truncate = %q, want the ellipsis", got)
	}
	if again := truncatePathMiddle(long, 20); again != got {
		t.Fatalf("truncate unstable: %q then %q", got, again)
	}
	if whole := truncatePathMiddle(`C:\p\proj`, 20); whole != `C:\p\proj` {
		t.Fatalf("truncate = %q, want a short path whole", whole)
	}
	if blank := truncatePathMiddle("", 20); blank != "" {
		t.Fatalf("truncate = %q, want the empty path whole", blank)
	}
}

// TestFileTreeRootRow shows the root path, truncated in the middle once
// the pane is narrow, with the full path on the tooltip.
func TestFileTreeRootRow(t *testing.T) {
	s, dir := newShell(t)
	proj := treeFixture(t, dir)
	tt := openTree(t, s, proj)
	s.SetSidebarWidth(120)
	tt.Frame()
	want := truncatePathMiddle(proj, explorerRootChars(120))
	if len([]rune(want)) >= len(proj) {
		t.Skipf("temp dir %q does not exceed a 120 pane", proj)
	}
	texts := tt.Texts()
	if !slices.Contains(texts, want) {
		t.Fatalf("root row = %v, want %q among them", texts, want)
	}
	for _, line := range texts {
		if line == proj {
			t.Fatalf("the whole root path leaked to the pane: %v", texts)
		}
	}
}

// TestFileTreeHighlightsActiveFile checks the row the pane marks as the
// open document, and that a tab switch moves the mark.
func TestFileTreeHighlightsActiveFile(t *testing.T) {
	s, dir := newShell(t)
	proj := treeFixture(t, dir)
	openTree(t, s, proj)
	x := filepath.Join(proj, "a", "b", "x.md")
	if err := s.OpenFile(x); err != nil {
		t.Fatal(err)
	}
	s.expandAncestors(s.explorerRoot(x), x)
	tt := s.render(t)
	tt.Frame()

	var marked []string
	for _, r := range s.tree.rows {
		if s.treeActive(r) {
			marked = append(marked, filepath.ToSlash(r.Rel))
		}
	}
	if !slices.Equal(marked, []string{"a/b/x.md"}) {
		t.Fatalf("highlighted = %v, want a/b/x.md", marked)
	}
	if r, ok := tt.Find("a/b/x.md"); !ok || r.Y <= 0 {
		t.Fatalf("highlighted row not rendered: %v %v", r, ok)
	}

	// The root document's tab is still open: switching to it moves the
	// highlight to its own row.
	for i := range s.Tabs {
		if s.Tabs[i].Path == filepath.Join(proj, "root.md") {
			s.Active = i
		}
	}
	tt.Frame()
	marked = nil
	for _, r := range s.tree.rows {
		if s.treeActive(r) {
			marked = append(marked, filepath.ToSlash(r.Rel))
		}
	}
	if !slices.Equal(marked, []string{"root.md"}) {
		t.Fatalf("highlighted after the switch = %v, want root.md", marked)
	}
}

// TestRevealFileExpandsAndScrolls is the tab menu's Reveal: every
// directory between the root and the file opens, and the file's row
// shows in the window.
func TestRevealFileExpandsAndScrolls(t *testing.T) {
	s, dir := newShell(t)
	proj := treeFixture(t, dir)
	tt := openTree(t, s, proj)
	x := filepath.Join(proj, "a", "b", "x.md")
	if err := s.OpenFile(x); err != nil {
		t.Fatal(err)
	}
	s.Saved.ExpandedDirs = nil
	s.invalidateTree()
	tt.Frame()
	if slices.Contains(treeRows(proj, s.tree.rows), "a/b/x.md") {
		t.Fatal("the collapsed tree already shows the file")
	}

	s.RevealFile(x)
	tt.Frame()
	got := treeRows(proj, s.tree.rows)
	if !slices.Contains(got, "a/b/x.md") {
		t.Fatalf("revealed rows = %v, want a/b/x.md among them", got)
	}
	want := []string{filepath.Join(proj, "a"), filepath.Join(proj, "a", "b")}
	for _, w := range want {
		if !slices.Contains(s.Saved.ExpandedDirs, w) {
			t.Fatalf("expanded = %v, want %s", s.Saved.ExpandedDirs, w)
		}
	}
	r, ok := tt.Find("a/b/x.md")
	if !ok {
		t.Fatalf("revealed row not rendered: %v", tt.Texts())
	}
	if r.Y < 0 || r.Y+r.H > 600 { // the test window is 800x600
		t.Fatalf("revealed row at %v is outside the 600 high window", r)
	}
	if !s.Sidebar.Visible {
		t.Fatalf("sidebar after reveal: %+v, want the file pane shown", s.Sidebar)
	}
}

// TestFileTreeCachesDirReads counts the directory reads: a frame over an
// unchanged tree reads nothing, expanding reads the new directory once,
// and a collapse plus expand again finds it cached.
func TestFileTreeCachesDirReads(t *testing.T) {
	s, dir := newShell(t)
	proj := treeFixture(t, dir)
	tt := openTree(t, s, proj)

	reads := s.tree.reads
	if reads == 0 {
		t.Fatal("the first frame read no directory")
	}
	tt.Frame()
	tt.Frame()
	if s.tree.reads != reads {
		t.Fatalf("reads after idle frames = %d, want %d", s.tree.reads, reads)
	}

	if err := tt.Click("a"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if s.tree.reads != reads+1 {
		t.Fatalf("reads after expanding = %d, want %d", s.tree.reads, reads+1)
	}
	reads = s.tree.reads
	for range 2 {
		if err := tt.Click("a"); err != nil { // collapse
			t.Fatal(err)
		}
		tt.Frame()
		if err := tt.Click("a"); err != nil { // expand again
			t.Fatal(err)
		}
		tt.Frame()
	}
	if s.tree.reads != reads {
		t.Fatalf("reads after collapse and expand = %d, want the cached %d", s.tree.reads, reads)
	}

	s.invalidateTreeDir(proj)
	tt.Frame()
	if s.tree.reads != reads+1 {
		t.Fatalf("reads after the directory changed = %d, want %d", s.tree.reads, reads+1)
	}
}

// TestFileTreeIcons maps the row kinds to their glyphs: folders open and
// closed, the markdown document, and the generic file for the rest.
func TestFileTreeIcons(t *testing.T) {
	s, dir := newShell(t)
	proj := treeFixture(t, dir)
	tt := openTree(t, s, proj)
	for _, name := range []string{"a", "a/b"} {
		if err := tt.Click(name); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
	}

	byName := map[string]treeRow{}
	for _, r := range s.tree.rows {
		byName[filepath.ToSlash(r.Rel)] = r
	}
	cases := []struct {
		rel      string
		want     *ui.SVG
		expanded bool
	}{
		{"a", folderOpenIcon, true},
		{"a/b", folderOpenIcon, true},
		{"a/b/x.md", docIcon, false},
	}
	for _, c := range cases {
		r, ok := byName[c.rel]
		if !ok {
			t.Fatalf("row %s missing from %v", c.rel, treeRows(proj, s.tree.rows))
		}
		if got := treeIcon(r, c.expanded); got != c.want {
			t.Fatalf("icon of %s = %p, want %p", c.rel, got, c.want)
		}
	}
	if got := treeIcon(byName["a"], false); got != folderIcon {
		t.Fatalf("closed folder icon = %p, want %p", got, folderIcon)
	}
}

// TestFileTreeOpensMarkdownClick clicks a nested document: the row opens
// it as a tab.
func TestFileTreeOpensMarkdownClick(t *testing.T) {
	s, dir := newShell(t)
	proj := treeFixture(t, dir)
	tt := openTree(t, s, proj)
	if err := tt.Click("a"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("a/y.md"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	y := filepath.Join(proj, "a", "y.md")
	if len(s.Tabs) != 2 || s.ActiveTab().Path != y {
		t.Fatalf("after the click: %d tabs, active %q, want %q", len(s.Tabs), s.ActiveTab().Path, y)
	}
}
