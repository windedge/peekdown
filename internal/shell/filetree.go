package shell

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"
)

// treeRow is one line of the explorer: a directory or a Markdown file.
// Rel is the slash path under the root, the row's label and the key the
// list follows; Name is the file's own name.
type treeRow struct {
	Path  string
	Name  string
	Rel   string
	Dir   bool
	Depth int
}

// cachedDir keeps one directory's listing: the pane walks the tree every
// frame and must not read the disk again for an unchanged directory.
type cachedDir struct {
	entries []os.DirEntry
	err     error
}

// treeState is the explorer's cache and the rows the last walk produced.
// reveal names a path the next frame scrolls into view.
type treeState struct {
	dirs   map[string]cachedDir
	sort   string
	rows   []treeRow
	list   ui.ListState
	reads  int
	reveal string
}

// loadDir lists dir through the cache, reading it again when it is new or
// when the sort mode changed under the cached order.
func (s *Shell) loadDir(dir, mode string) ([]os.DirEntry, error) {
	if s.tree.dirs == nil {
		s.tree.dirs = map[string]cachedDir{}
	}
	if c, ok := s.tree.dirs[dir]; ok && s.tree.sort == mode {
		return c.entries, c.err
	}
	entries, err := readSortedEntries(dir, mode)
	s.tree.dirs[dir] = cachedDir{entries: entries, err: err}
	s.tree.sort = mode
	s.tree.reads++
	return entries, err
}

// invalidateTree drops every listing: the next frame reads the disk again.
func (s *Shell) invalidateTree() {
	s.tree.dirs = nil
	s.tree.sort = ""
}

// invalidateTreeDir drops one directory's listing, as the watcher does when
// a file under it changed.
func (s *Shell) invalidateTreeDir(dir string) {
	if s.tree.dirs == nil {
		return
	}
	delete(s.tree.dirs, dir)
}

// visibleRows flattens the tree into the rows the pane draws: every
// directory, and the Markdown files, with the opening ones' children
// under them. Hidden entries and other files stay out.
func (s *Shell) visibleRows(root string) []treeRow {
	rows := make([]treeRow, 0, len(s.tree.rows))
	var walk func(dir, rel string, depth int)
	walk = func(dir, rel string, depth int) {
		entries, err := s.loadDir(dir, s.Saved.ExplorerSortMode)
		if err != nil {
			return
		}
		for _, e := range entries {
			name := e.Name()
			if strings.HasPrefix(name, ".") {
				continue
			}
			path := filepath.Join(dir, name)
			rowRel := name
			if rel != "" {
				rowRel = rel + "/" + name
			}
			if e.IsDir() {
				rows = append(rows, treeRow{Path: path, Name: name, Rel: rowRel, Dir: true, Depth: depth})
				if s.expandedDir(path) {
					walk(path, rowRel, depth+1)
				}
				continue
			}
			if !IsMarkdown(name) {
				continue
			}
			rows = append(rows, treeRow{Path: path, Name: name, Rel: rowRel, Depth: depth})
		}
	}
	walk(root, "", 0)
	s.tree.rows = rows
	return rows
}

// sameDir compares two paths as the platform does: case-insensitively on
// Windows, where the settings and the tree may spell a directory either way.
func sameDir(a, b string) bool {
	return sameFile(filepath.Clean(a), filepath.Clean(b))
}

// expandedDir reports whether the settings keep dir open.
func (s *Shell) expandedDir(dir string) bool {
	for _, p := range s.Saved.ExpandedDirs {
		if sameDir(p, dir) {
			return true
		}
	}
	return false
}

// persistExpanded writes the expanded set through the settings, which is
// where it survives a restart.
func (s *Shell) persistExpanded() {
	s.Draft.ExpandedDirs = append([]string(nil), s.Saved.ExpandedDirs...)
	if s.Save != nil {
		_ = s.Save(s.Saved)
	}
}

// toggleDir opens or closes a directory row, persisting the expanded set.
func (s *Shell) toggleDir(r treeRow) {
	if !r.Dir {
		return
	}
	if s.expandedDir(r.Path) {
		s.Saved.ExpandedDirs = slices.DeleteFunc(s.Saved.ExpandedDirs,
			func(p string) bool { return sameDir(p, r.Path) })
	} else {
		s.Saved.ExpandedDirs = append(s.Saved.ExpandedDirs, r.Path)
	}
	// One entry per directory: a repeated open must not stack duplicates.
	slices.Sort(s.Saved.ExpandedDirs)
	s.Saved.ExpandedDirs = slices.Compact(s.Saved.ExpandedDirs)
	s.persistExpanded()
}

// expandAncestors opens every directory between root and path, so a
// reveal shows the file without opening the whole tree.
func (s *Shell) expandAncestors(root, path string) {
	root = filepath.Clean(root)
	added := false
	for dir := filepath.Dir(filepath.Clean(path)); ; dir = filepath.Dir(dir) {
		if sameDir(dir, root) || !s.underRoot(root, dir) {
			break
		}
		if !s.expandedDir(dir) {
			s.Saved.ExpandedDirs = append(s.Saved.ExpandedDirs, dir)
			added = true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
	if added {
		slices.Sort(s.Saved.ExpandedDirs)
		s.Saved.ExpandedDirs = slices.Compact(s.Saved.ExpandedDirs)
		s.persistExpanded()
	}
}

// underRoot reports whether dir sits inside root, not beside it: the walk
// up from a file stops at the root instead of climbing past it.
func (s *Shell) underRoot(root, dir string) bool {
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// RevealFile shows the explorer with the path's directories open and
// scrolls its row into view; the next frame does the scrolling.
func (s *Shell) RevealFile(path string) {
	root := s.explorerRoot(path)
	s.SetSidebarTab(0)
	s.expandAncestors(root, path)
	s.tree.reveal = path
}

// treeActive reports whether the row is the open document, which the pane
// tints: the tab's path and the row's are the same file, spelled either way.
func (s *Shell) treeActive(r treeRow) bool {
	tab := s.ActiveTab()
	return tab != nil && tab.Path != "" && sameFile(tab.Path, r.Path)
}

// treeIcon is the glyph of a row: an open or closed folder, the document
// mark, or the generic file for anything else.
func treeIcon(r treeRow, expanded bool) *ui.SVG {
	if r.Dir {
		if expanded {
			return folderOpenIcon
		}
		return folderIcon
	}
	if IsMarkdown(r.Name) {
		return docIcon
	}
	return fileIcon
}

// explorerRootChars is how much of the root path a pane of the given width
// shows: the row's padding plus the icon and label sizes, at the font's
// average advance.
func explorerRootChars(w float32) int {
	return int((w - 74) / 7)
}

// truncatePathMiddle shortens a path to maxChars runes, keeping both ends
// and the ellipsis in the middle: a tail stays legible where the head is
// context, and a separator picks the cut so no name is split in half.
func truncatePathMiddle(path string, maxChars int) string {
	if maxChars < 5 {
		maxChars = 5
	}
	r := []rune(path)
	if len(r) <= maxChars {
		return path
	}
	sep := filepath.Separator
	tail := 10
	if tail > maxChars-4 {
		tail = maxChars - 4
	}
	cut := len(r) - tail
	for i := cut; i > 0; i-- {
		if r[i] != sep && r[i] != '/' {
			continue
		}
		keep := maxChars - tail - 4
		if keep > i {
			keep = i
		}
		if keep < 2 {
			break
		}
		return string(r[:keep]) + "..." + string(sep) + string(r[cut:])
	}
	return string(r[:maxChars-3]) + "..."
}

// filesPane renders the explorer: the root path, then one row per visible
// directory and Markdown file.
func (s *Shell) filesPane(c *ui.Context) {
	t := c.Theme()
	tab := s.ActiveTab()
	var root string
	if tab != nil && tab.Path != "" {
		root = s.explorerRoot(tab.Path)
	}
	ui.Column(c).Fill().Children(func() {
		if root == "" {
			ui.Text(c, "No file open").FontSize(12).TextColor(t.TextMuted).
				Padding(4, 12)
			return
		}
		// The root row names the pane's directory, cut in the middle so
		// both ends survive a narrow sidebar; the tooltip holds it whole.
		ui.Text(c, truncatePathMiddle(root, explorerRootChars(s.Sidebar.Width))).
			FontSize(12).FontWeight(550).TextColor(t.TextMuted).Tooltip(root).
			SingleLine().Ellipsis("...").Padding(4, 12).PaddingEnd(14)
		rows := s.visibleRows(root)
		if len(rows) == 0 {
			ui.Text(c, "No Markdown files").FontSize(12).TextColor(t.TextMuted).
				Padding(4, 12)
			return
		}
		ui.List(c, &s.tree.list, len(rows), func(i int) {
			s.treeRowElement(c, rows[i])
		}).ItemKey(func(i int) any { return rows[i].Path }).Grow(1)
		if reveal := s.tree.reveal; reveal != "" {
			s.tree.reveal = ""
			for i, r := range rows {
				if sameDir(r.Path, reveal) {
					s.tree.list.ScrollIntoView(i)
					break
				}
			}
		}
	})
}

// treeRowElement draws one row: its indent, the folder chevron, the icon
// and the name, with the active document tinted.
func (s *Shell) treeRowElement(c *ui.Context, r treeRow) {
	t := c.Theme()
	expanded := s.expandedDir(r.Path)
	indent := 12 + 12*float32(r.Depth)
	row := ui.ButtonBase(c.Key(r.Path)).FillWidth().Padding(4, 12).PaddingStart(indent).
		Radius(4).Cursor(ui.CursorPointer).AlignItems(ui.Center).Justify(ui.Start)
	active := s.treeActive(r)
	hover := row.Hovered()
	switch {
	case active:
		row.Background(t.Accent.Mix(t.Surface, 0.85))
	case hover:
		row.Background(t.SurfaceHover)
	}
	row.Children(func() {
		ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
			// Files get an empty chevron slot so names line up per depth.
			if r.Dir {
				icon := chevronRightIcon
				if expanded {
					icon = chevronDownIcon
				}
				ui.Icon(c, icon).Size(16, 16)
			} else {
				ui.Box(c).Width(16).Height(16)
			}
			size := float32(12)
			if r.Dir {
				size = 16
			}
			// One 16 wide icon slot for both kinds: a file's smaller
			// glyph must not shift its name off the depth's column.
			ui.Box(c).Width(16).Height(16).AlignItems(ui.Center).Justify(ui.Center).
				Children(func() { ui.Icon(c, treeIcon(r, expanded)).Size(size, size) })
			name := ui.Text(c, r.Rel).FontSize(14).SingleLine().Ellipsis("...").
				Grow(1).MinWidth(0).PaddingEnd(14)
			if active {
				name.TextColor(t.Accent)
			} else {
				name.TextColor(t.Text)
			}
		})
	})
	if row.Clicked() {
		if r.Dir {
			s.toggleDir(r)
		} else if IsMarkdown(r.Name) {
			_ = s.OpenFile(r.Path)
		}
	}
}
