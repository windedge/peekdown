package doc

import (
	"reflect"
	"strings"
	"testing"
)

// The search addresses the run table: a match's Start and End are rune
// offsets into the text the view draws, so slicing that text at them
// returns the query, case apart.

func TestFindAllPlain(t *testing.T) {
	m := New("Alpha alpha ALPHA beta\n")
	block := m.Blocks()[0].Index
	tests := []struct {
		name  string
		query string
		cs    bool
		want  []Match
	}{
		{"insensitive", "alpha", false, []Match{{block, 0, 5}, {block, 6, 11}, {block, 12, 17}}},
		{"sensitive", "alpha", true, []Match{{block, 6, 11}}},
		{"sensitive upper", "ALPHA", true, []Match{{block, 12, 17}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FindAll(m, tt.query, SearchOptions{CaseSensitive: tt.cs})
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("FindAll %q: %+v, want %+v", tt.query, got, tt.want)
			}
		})
	}
}

// A match's offsets are runes, not bytes: slicing the run text at them
// must return the query.
func TestFindAllRuneOffsets(t *testing.T) {
	m := New("你好世界，你好\n")
	block := m.Blocks()[0].Index
	got := FindAll(m, "你好", SearchOptions{})
	want := []Match{{block, 0, 2}, {block, 5, 7}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FindAll: %+v, want %+v", got, want)
	}
	runes := []rune(m.Runs()[0].Text)
	if string(runes[got[1].Start:got[1].End]) != "你好" {
		t.Fatalf("slice at match: %q", string(runes[got[1].Start:got[1].End]))
	}
}

// strings.EqualFold semantics: runes in one simple fold orbit match, a
// Greek capital omega against a small one, say. A Latin O is not in that
// orbit and must not match.
func TestFindAllFoldsUnicode(t *testing.T) {
	m := New("Ωmega omega\n")
	got := FindAll(m, "ωmega", SearchOptions{})
	if len(got) != 1 || got[0].Start != 0 {
		t.Fatalf("FindAll: %+v", got)
	}
	if got := FindAll(m, "omega", SearchOptions{}); len(got) != 1 || got[0].Start != 6 {
		t.Fatalf("latin omega: %+v", got)
	}
}

func TestFindAllRegex(t *testing.T) {
	m := New("Go go GO go\n")
	got := FindAll(m, `\bgo\b`, SearchOptions{Regex: true})
	if len(got) != 4 {
		t.Fatalf("insensitive regex: %+v", got)
	}
	got = FindAll(m, `\bgo\b`, SearchOptions{Regex: true, CaseSensitive: true})
	if len(got) != 2 || got[0].Start != 3 || got[1].Start != 9 {
		t.Fatalf("sensitive regex: %+v", got)
	}
	got = FindAll(m, `\bGo\b`, SearchOptions{Regex: true, CaseSensitive: true})
	if len(got) != 1 || got[0].Start != 0 {
		t.Fatalf("sensitive Go: %+v", got)
	}
}

// An invalid pattern matches nothing, so the caller can show an error.
func TestFindAllInvalidRegex(t *testing.T) {
	m := New("text\n")
	if got := FindAll(m, "(", SearchOptions{Regex: true}); got != nil {
		t.Fatalf("invalid regex: %+v", got)
	}
}

// An empty match would loop the scan forever and highlights nothing.
func TestFindAllRegexSkipsEmptyMatches(t *testing.T) {
	m := New("azb\n")
	got := FindAll(m, "z*", SearchOptions{Regex: true})
	want := []Match{{m.Blocks()[0].Index, 1, 2}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FindAll: %+v, want %+v", got, want)
	}
}

func TestFindAllEmptyQuery(t *testing.T) {
	m := New("text\n")
	if got := FindAll(m, "", SearchOptions{}); got != nil {
		t.Fatalf("empty query: %+v", got)
	}
	if got := FindAll(m, "", SearchOptions{Regex: true}); got != nil {
		t.Fatalf("empty regex: %+v", got)
	}
}

// Blocks never join: a query that only a seam could match finds nothing.
func TestFindAllStaysInBlocks(t *testing.T) {
	m := New("xa\n\nax\n")
	if got := FindAll(m, "aa", SearchOptions{}); len(got) != 0 {
		t.Fatalf("cross-block match: %+v", got)
	}
}

func TestFindAllOrdersAcrossBlocks(t *testing.T) {
	m := New("hit here\n\nno match\n\nhit again\n")
	blocks := m.Blocks()
	got := FindAll(m, "hit", SearchOptions{})
	want := []Match{{blocks[0].Index, 0, 3}, {blocks[2].Index, 0, 3}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FindAll: %+v, want %+v", got, want)
	}
}

func TestFindAllLimitTruncates(t *testing.T) {
	m := New("x x x x x x x x x x\n")
	got := FindAll(m, "x", SearchOptions{Limit: 5})
	if len(got) != 5 {
		t.Fatalf("%d matches, want 5", len(got))
	}
	block := m.Blocks()[0].Index
	if got[4] != (Match{block, 8, 9}) {
		t.Fatalf("fifth match %+v, want the first five not the last", got[4])
	}
}

func TestFindAllDefaultLimit(t *testing.T) {
	m := New(strings.Repeat("a", 30) + "\n")
	if got := FindAll(m, "a", SearchOptions{}); len(got) != 30 {
		t.Fatalf("%d matches, want 30 (the default limit not reached)", len(got))
	}
}

// A hard line break contributes a space to the run text, a footnote
// reference its "[^label]": both are searchable there.
func TestFindAllHitsLineBreakSpace(t *testing.T) {
	m := New("alpha  \nbeta\n")
	got := FindAll(m, "alpha beta", SearchOptions{})
	if len(got) != 1 || got[0].Start != 0 || got[0].End != 10 {
		t.Fatalf("FindAll: %+v", got)
	}
}

func TestFindAllHitsFootnoteRef(t *testing.T) {
	m := New("see[^1] now\n\n[^1]: the note\n")
	got := FindAll(m, "[^1]", SearchOptions{})
	if len(got) != 1 || got[0].Start != 3 || got[0].End != 7 {
		t.Fatalf("FindAll: %+v", got)
	}
	if got[0].Block != m.Blocks()[0].Index {
		t.Fatalf("match block %d, want %d", got[0].Block, m.Blocks()[0].Index)
	}
}

func TestSetSearchAndNavigate(t *testing.T) {
	m := New("alpha beta\n\nalpha again\n")
	v := m.Version()

	m.SetSearch("alpha", SearchOptions{})
	if len(m.Matches) != 2 || m.Current != 0 {
		t.Fatalf("after SetSearch: %d matches, current %d", len(m.Matches), m.Current)
	}
	if m.Query() != "alpha" {
		t.Fatalf("Query: %q", m.Query())
	}

	m.NextMatch()
	if m.Current != 1 {
		t.Fatalf("next: current %d", m.Current)
	}
	m.NextMatch()
	if m.Current != 0 {
		t.Fatalf("next past the last: current %d", m.Current)
	}
	m.PrevMatch()
	if m.Current != 1 {
		t.Fatalf("prev: current %d", m.Current)
	}
	m.PrevMatch()
	if m.Current != 0 {
		t.Fatalf("prev: current %d", m.Current)
	}
	m.PrevMatch()
	if m.Current != 1 {
		t.Fatalf("prev past the first: current %d", m.Current)
	}

	if m.Version() != v {
		t.Fatalf("version %d, want it unchanged: searching is not editing", m.Version())
	}

	m.SetSearch("nope", SearchOptions{})
	if len(m.Matches) != 0 || m.Current != -1 {
		t.Fatalf("no hits: %d matches, current %d", len(m.Matches), m.Current)
	}
	m.NextMatch()
	m.PrevMatch()
	if m.Current != -1 {
		t.Fatalf("navigating with no hits: current %d", m.Current)
	}

	m.ClearSearch()
	if m.Matches != nil || m.Current != -1 || m.Query() != "" {
		t.Fatalf("after ClearSearch: %v, current %d, query %q", m.Matches, m.Current, m.Query())
	}
}
