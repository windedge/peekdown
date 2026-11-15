package shell

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/locales"
)

// sidebar renders the file list or the outline of the active tab. The
// header copies the Rust sidebar's (mod.rs around line 2070): small
// text tabs on the left, a ghost close on the right, a hairline under
// both; no chip track like a Segmented control builds.
func (s *Shell) sidebar(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Background(t.Surface).Children(func() {
		ui.Row(c).Padding(4, 8).Gap(2).AlignItems(ui.Center).
			BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			s.sidebarTab(c, "sidebar-files-tab", 0, locales.SidebarFiles())
			s.sidebarTab(c, "sidebar-outline-tab", 1, locales.SidebarOutline())
			ui.Box(c).Grow(1)
			iconButton(c, "close-sidebar", closeIcon, locales.SidebarClose(), 20, 10, func() { s.Sidebar.Visible = false })
		})
		if s.Sidebar.Tab == 0 {
			s.filesPane(c)
			return
		}
		s.outlinePane(c)
	})
}

// sidebarTab is one header tab, the Rust one's plain small label: muted
// text that turns accent on the active side with a faint accent wash,
// radius 4, nothing else.
func (s *Shell) sidebarTab(c *ui.Context, key string, index int, label string) {
	t := c.Theme()
	b := ui.ButtonBase(c.Key(key)).Label(label).Padding(2, 8).Radius(4).
		AlignItems(ui.Center).Justify(ui.Center).FocusRing(false).
		FontSize(11).FontWeight(500)
	// Hover is read before the branch so the active tab tracks it too,
	// even though its look stays as it is.
	hover := b.Hovered()
	if s.Sidebar.Tab == index {
		b.Background(t.Accent.Alpha(0.15)).TextColor(t.Accent)
	} else if hover {
		b.Background(t.SurfaceHover).TextColor(t.Text)
	} else {
		b.TextColor(t.TextMuted)
	}
	b.Children(func() { ui.Text(c, label).FontSize(11).FontWeight(500) })
	if b.Clicked() {
		s.SetSidebarTab(index)
	}
}

// explorerRoot is the directory the file pane shows: the document's own
// directory, or the project root found upward from it when the settings
// ask for one. Without a root it falls back to the document's directory.
func (s *Shell) explorerRoot(path string) string {
	dir := filepath.Dir(path)
	if s.Saved.ExplorerRootMode != "project_root" {
		return dir
	}
	if root := findProjectRoot(dir, s.Saved.ProjectRootMarkers); root != "" {
		return root
	}
	return dir
}

// bottomUpVCS and recurringVCS mirror Rust's workspace module: a VCS
// marker claims the whole repo, so the closest ancestor wins, while .svn
// and CVS repeat per subdirectory and resolve to the topmost contiguous
// run instead.
var (
	bottomUpVCS  = []string{".git", ".hg", ".pijul", "_darcs", ".bzr", ".jj"}
	recurringVCS = []string{".svn", "CVS"}
)

// findProjectRoot walks from dir up to the drive root once, collecting
// three candidates: the closest bottom-up VCS root, the bottom of the
// topmost contiguous recurring-VCS run, and the outermost configured
// marker. The VCS roots win over configured markers because a repo
// boundary is stronger than a stray build file; configured markers stay
// outermost because that is where a whole project starts. Returns ""
// when nothing matches, and the caller falls back to the document dir.
func findProjectRoot(dir string, markers []string) string {
	var bottomUp, topDown string
	var recurringHits []string

	for {
		// First hit wins: it is the closest ancestor as we walk up.
		for _, vcs := range bottomUpVCS {
			if exists(filepath.Join(dir, vcs)) {
				if bottomUp == "" {
					bottomUp = dir
				}
				break
			}
		}
		for _, vcs := range recurringVCS {
			if exists(filepath.Join(dir, vcs)) {
				recurringHits = append(recurringHits, dir)
				break
			}
		}
		// Outermost level wins: keep overwriting while walking up.
		for _, m := range markers {
			m = strings.TrimSpace(m)
			if m == "" {
				continue
			}
			if exists(filepath.Join(dir, m)) {
				topDown = dir
				break
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	if bottomUp != "" {
		return bottomUp
	}
	if len(recurringHits) > 0 {
		slices.Reverse(recurringHits)
		// After reversing, walk down while each hit is a child of the
		// previous one; the first gap ends the topmost run.
		result := recurringHits[0]
		for i := 1; i < len(recurringHits); i++ {
			if filepath.Dir(recurringHits[i]) == recurringHits[i-1] {
				result = recurringHits[i]
			} else {
				break
			}
		}
		return result
	}
	return topDown
}

// exists is a plain stat, matching Rust's Path::exists: directories and
// files both count as a marker hit.
func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// readSortedEntries lists dir and orders it as the sort setting asks:
// directories first, then by name or modification time. The file tree
// reads a directory through loadDir, which caches the result; the sort
// itself stays here.
func readSortedEntries(dir, mode string) ([]os.DirEntry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	byTime := mode == "time_asc" || mode == "time_desc"
	if byTime {
		type stamp struct {
			entry os.DirEntry
			mod   time.Time
		}
		rows := make([]stamp, 0, len(entries))
		for _, e := range entries {
			mod := time.Time{}
			if info, err := e.Info(); err == nil {
				mod = info.ModTime()
			}
			rows = append(rows, stamp{e, mod})
		}
		slices.SortFunc(rows, func(a, b stamp) int {
			if a.entry.IsDir() != b.entry.IsDir() {
				if a.entry.IsDir() {
					return -1
				}
				return 1
			}
			if c := a.mod.Compare(b.mod); c != 0 {
				if mode == "time_desc" {
					return -c
				}
				return c
			}
			return strings.Compare(a.entry.Name(), b.entry.Name())
		})
		out := make([]os.DirEntry, len(rows))
		for i, v := range rows {
			out[i] = v.entry
		}
		return out, nil
	}
	slices.SortFunc(entries, func(a, b os.DirEntry) int {
		if a.IsDir() != b.IsDir() {
			if a.IsDir() {
				return -1
			}
			return 1
		}
		c := strings.Compare(a.Name(), b.Name())
		if mode == "name_desc" {
			return -c
		}
		return c
	})
	return entries, nil
}

func (s *Shell) outlinePane(c *ui.Context) {
	t := c.Theme()
	tab := s.ActiveTab()
	ui.Column(c).Fill().Children(func() {
		if tab == nil || tab.Model == nil {
			ui.Text(c, locales.NoHeadings()).FontSize(13).TextColor(t.TextMuted).
				Padding(4, 12)
			return
		}
		headings := tab.Model.Headings()
		if len(headings) == 0 {
			ui.Text(c, locales.NoHeadings()).FontSize(13).TextColor(t.TextMuted).
				Padding(4, 12)
			return
		}
		// The entry the document is scrolled to shows on a surface, the
		// section the reader is in; -1 for none. LinksWired is the shell's
		// "this tab is drawn through the shell" flag: the outline skips
		// the follow work for a view nobody wired (a bare test build).
		current := -1
		if s.Sidebar.Tab == 1 && tab.State.LinksWired {
			current = currentHeadingBlock(tab)
		}
		// A long outline scrolls: the rows stack in a Scroll, each one
		// as tall as its text (Fill would claim the pane's whole height).
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Gap(1).Padding(2, 0).Children(func() {
				for _, h := range headings {
					// Rust outline.rs: the whole row indents per level, 12 DIPs
					// a step off a 12 DIP base, not just the text inside it.
					// PaddingStart overrides the row's own start padding.
					indent := 12 + 12*float32(h.Level-1)
					row := ui.ButtonBase(c).FillWidth().Padding(4, 12).PaddingStart(indent).Radius(4).Cursor(ui.CursorPointer).
						AlignItems(ui.Center).Justify(ui.Start)
					// Hover lights a row with the same surface the active
					// section holds; the highlight wins and does not stack.
					hover := row.Hovered()
					if block, ok := tab.Model.AnchorBySlug(tab.Model.Anchor(h.BlockIndex)); (ok && block == current) || hover {
						row.Background(t.SurfaceHover)
					}
					row.Children(func() {
						ui.Text(c, h.Text).FontSize(13).SingleLine().Ellipsis("...").
							Grow(1).MinWidth(0).PaddingEnd(14)
					})
					if row.Clicked() {
						tab.State.ScrollTargetBlock = h.BlockIndex
						// The Rust version keeps the sidebar open after a jump.
					}
				}
			})
		})
	})
}

// currentHeadingBlock returns the block of the first heading at or above
// the top of the view: the section the document is scrolled into, which
// the outline highlights. The rows' handles give each block's window
// rectangle, so the first visible one wins; no rows, no highlight. The
// view's list is virtual, so a heading two viewports down has no
// committed rectangle and misses on its own; the 3-viewports band below
// the top covers the gap between row commits and scroll jumps.
func currentHeadingBlock(tab *Tab) int {
	best := -1
	for _, h := range tab.Model.Headings() {
		r, ok := tab.State.RowRect(h.BlockIndex)
		if !ok || r.Y < -r.H || r.Y > headingLookaheadY {
			continue
		}
		if best < 0 || h.BlockIndex < best {
			best = h.BlockIndex
		}
	}
	return best
}

// MarkdownExtensions is the extension set IsMarkdown accepts, for the
// host's open dialog filters to agree with it.
func MarkdownExtensions() []string {
	return []string{"md", "markdown", "mdown", "mkd"}
}

// headingLookaheadY is how far below the window's top the outline looks
// for the current section's heading, in window DIPs: about three
// viewports of a 600 DIP window. The list commits rows as it draws
// them, so a heading far below the viewport has no rectangle; the band
// keeps the highlight on the section the reader is entering through
// scroll jumps.
const headingLookaheadY = float32(2000)

// IsMarkdown reports whether name looks like a Markdown document, by
// its extension: the open dialog, the file pane and the drop filter
// agree on the set.
func IsMarkdown(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	for _, e := range MarkdownExtensions() {
		if ext == "."+e {
			return true
		}
	}
	return false
}
