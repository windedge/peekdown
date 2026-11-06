package shell

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/doc"
)

// searchBar is the state of the search bar over the content: the query
// as the field edits it, the toggles, and the count line. The shell's
// one per app, so the field's text survives switching tabs.
type searchBar struct {
	// Query is what the field edits; running it is SetSearch's job on
	// the active tab's model.
	Query string
	// Regex and CaseSensitive mirror the toggles' state.
	Regex         bool
	CaseSensitive bool

	// Count is the bar's status text: nothing without a query, "bad
	// pattern" under an invalid regex, "no results" with none, else
	// "current/total", one-based.
	Count string

	// focus wants the field's keyboard focus in the frame it comes, the
	// way Ctrl+F asks it; the field's Focus resets it when it took.
	focus bool
}

// searchStatus is the count line for a model's search state: nothing
// without a query, "bad pattern" under an invalid regex, "no results"
// with none, else "current/total", one-based.
func searchStatus(m *doc.Model) string {
	if m.Query() == "" {
		return ""
	}
	if m.SearchOptions().Regex {
		if _, err := doc.CompilePattern(m.Query(), m.SearchOptions().CaseSensitive); err != nil {
			return "bad pattern"
		}
	}
	if len(m.Matches) == 0 {
		return "no results"
	}
	return fmt.Sprintf("%d/%d", m.Current+1, len(m.Matches))
}

// runSearch runs the bar's query and options over the tab's model and
// updates the count line; a query with hits starts at its first match
// and scrolls the block into view, and moves after that track it.
func (s *Shell) runSearch(tab *Tab) {
	m := tab.Model
	m.SetSearch(s.search.Query, doc.SearchOptions{
		Regex:         s.search.Regex,
		CaseSensitive: s.search.CaseSensitive,
	})
	s.trackMatch(tab)
}

// OpenSearch shows the bar, focusing its field: the query starts as the
// document's selection, as editors do, else it stays what it was.
func (s *Shell) OpenSearch() {
	if s.Active < 0 || s.Active >= len(s.Tabs) {
		return
	}
	s.SearchOpen = true
	tab := &s.Tabs[s.Active]
	if st := &tab.State; !s.search.focus && st.HasSelection() {
		if sel := st.SelectionText(tab.Model); sel != "" {
			s.search.Query = sel
			s.runSearch(tab)
		}
	}
	s.search.focus = true
}

// CloseSearch drops the bar and the highlights on every tab: the matches
// name text the next search's model need not share.
func (s *Shell) CloseSearch() {
	s.SearchOpen = false
	s.search.Query = ""
	s.search.Regex = false
	s.search.CaseSensitive = false
	s.search.Count = ""
	s.search.focus = false
	for i := range s.Tabs {
		s.Tabs[i].Model.ClearSearch()
	}
}

// NextSearchMatch moves to the next match and scrolls to its block; the
// bar shows the new current.
func (s *Shell) NextSearchMatch() {
	if !s.SearchOpen {
		return
	}
	tab := s.ActiveTab()
	if tab == nil {
		return
	}
	tab.Model.NextMatch()
	s.trackMatch(tab)
}

// PrevSearchMatch moves to the previous match and scrolls to its block.
func (s *Shell) PrevSearchMatch() {
	if !s.SearchOpen {
		return
	}
	tab := s.ActiveTab()
	if tab == nil {
		return
	}
	tab.Model.PrevMatch()
	s.trackMatch(tab)
}

// tabSwitched retargets the bar at the tab's own search: the field takes
// the tab's query and the count reads the tab's matches, so the bar
// never shows a stale tab's count. It runs only while the bar shows.
func (s *Shell) tabSwitched(tab *Tab) {
	if !s.SearchOpen {
		return
	}
	s.search.Query = tab.Model.Query()
	s.search.Count = searchStatus(tab.Model)
}

// trackMatch hands the block of the current match to the view, which
// scrolls it into view once.
func (s *Shell) trackMatch(tab *Tab) {
	m := tab.Model
	if m.Current >= 0 && m.Current < len(m.Matches) {
		tab.State.ScrollTargetBlock = m.Matches[m.Current].Block
	}
	s.search.Count = searchStatus(m)
}

// searchView builds the bar: the field, the count, the toggles, the
// arrows and the close, above the document, under the tabs.
func (s *Shell) searchView(c *ui.Context) {
	t := c.Theme()
	bar := ui.Row(c).Height(32).AlignSelf(ui.Stretch).Padding(2, 8).Gap(6).
		Background(t.Surface).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).AlignItems(ui.Center)
	bar.Children(func() {
		in := ui.TextInput(c, &s.search.Query).Grow(1).Height(24).Placeholder("Find")
		// The keys that edit nothing go to the search: Enter and
		// Shift+Enter move, Escape closes. A taken key does not reach
		// the input, so Enter submits nothing and Escape clears not.
		in.OnTextKey(func(k ui.TextKey) bool {
			switch {
			case k.Key == ui.KeyEnter && k.Mods == ui.Shift:
				s.PrevSearchMatch()
				return true
			case k.Key == ui.KeyEnter && k.Mods == 0:
				s.NextSearchMatch()
				return true
			case k.Key == ui.KeyEscape && k.Mods == 0:
				s.CloseSearch()
				return true
			}
			return false
		})
		if s.search.focus {
			s.search.focus = in.Focused()
			in.Focus()
		}
		if in.Changed() {
			s.runSearch(s.ActiveTab())
		}
		ui.Text(c, s.search.Count).FontSize(12).TextColor(t.TextMuted).Shrink(0)

		toggle(c, "search-case", "Match case", "Aa", s.search.CaseSensitive, s.flipCase)
		toggle(c, "search-regex", "Regular expression", ".*", s.search.Regex, s.flipRegex)

		iconButton(c, "search-prev", chevronUpIcon, "Previous match", 24, 14, s.PrevSearchMatch)
		iconButton(c, "search-next", chevronDownIcon, "Next match", 24, 14, s.NextSearchMatch)
		iconButton(c, "search-close", closeIcon, "Close search", 24, 14, s.CloseSearch)
	})
}

// flipCase re-runs the search with case folding turned over.
func (s *Shell) flipCase() {
	s.search.CaseSensitive = !s.search.CaseSensitive
	s.rerun()
}

// flipRegex re-runs the search over regexes turned over.
func (s *Shell) flipRegex() {
	s.search.Regex = !s.search.Regex
	s.rerun()
}

// rerun runs the query again with the toggles' state, on the active tab.
func (s *Shell) rerun() {
	if tab := s.ActiveTab(); tab != nil && tab.Model != nil {
		s.runSearch(tab)
	}
}

// toggle is a pressed-state icon button: it paints the theme's
// InactiveAccent while the state it toggles is on.
func toggle(c *ui.Context, key any, label, glyph string, on bool, flip func()) {
	t := c.Theme()
	b := ui.ButtonBase(c.Key(key)).Label(label).Size(24, 24).Padding(0).Radius(5)
	b.AlignItems(ui.Center).Justify(ui.Center).FocusRing(false)
	if on {
		b.Background(t.InactiveAccent)
	} else if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() { ui.Text(c, glyph).FontSize(12) })
	if b.Clicked() {
		flip()
	}
}
