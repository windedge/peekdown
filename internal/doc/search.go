package doc

import (
	"regexp"
	"unicode"
	"unicode/utf8"
)

// SearchOptions tunes FindAll: a plain scan or a regex, its case
// folding, and a cap on the matches.
type SearchOptions struct {
	Regex         bool
	CaseSensitive bool
	// Limit caps the matches; 0 or less takes the default, 10000.
	Limit int
}

// defaultLimit caps a search, as the Rust version does: enough to fill
// any view, few enough to stop a pathological document from stalling it.
const defaultLimit = 10000

// Match is one search hit: block i's run text, runes from Start to End.
// The offsets address the run table, the text the view draws, so the
// highlight lands on the found words.
type Match struct {
	Block      int
	Start, End int
}

// FindAll returns the matches of query over the document, in reading
// order, capped at the limit: the scan stops at the cap, so the caller
// tells truncation by comparing len to it. It scans the run table, the
// one authority on a block's text, the same text the view draws, so
// offsets from here highlight right. An invalid regex or an empty query
// matches nothing.
func FindAll(m *Model, query string, opts SearchOptions) []Match {
	if query == "" {
		return nil
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	var out []Match
	if opts.Regex {
		re, err := CompilePattern(query, opts.CaseSensitive)
		if err != nil {
			return nil
		}
		for _, run := range m.runs {
			// FindAllStringIndex reports byte offsets and never lets
			// matches overlap; an empty match highlights nothing, so
			// it drops.
			for _, loc := range re.FindAllStringIndex(run.Text, -1) {
				if loc[0] == loc[1] {
					continue
				}
				out = append(out, Match{
					Block: run.Block,
					Start: byteToRune(run.Text, loc[0]),
					End:   byteToRune(run.Text, loc[1]),
				})
				if len(out) == limit {
					return out
				}
			}
		}
		return out
	}
	needle := []rune(query)
	for _, run := range m.runs {
		r := []rune(run.Text)
		for i := 0; i+len(needle) <= len(r); {
			end := i + len(needle)
			if runesMatch(r[i:end], needle, opts.CaseSensitive) {
				out = append(out, Match{Block: run.Block, Start: i, End: end})
				if len(out) == limit {
					return out
				}
				// Matches never overlap: the scan resumes past this one.
				i = end
				continue
			}
			i++
		}
	}
	return out
}

// CompilePattern compiles query as a regexp the way FindAll does under
// Regex: one place for the case-folding flag, so a pattern the search
// accepts is one the caller can compile too, and the other way round.
func CompilePattern(query string, caseSensitive bool) (*regexp.Regexp, error) {
	pattern := query
	if !caseSensitive {
		pattern = "(?i)" + pattern
	}
	return regexp.Compile(pattern)
}

// runesMatch compares two equal-length rune slices, case-sensitively or
// under strings.EqualFold's folding: runes match when one orbit of
// simple case folding holds them both.
func runesMatch(a, b []rune, caseSensitive bool) bool {
	for k, br := range b {
		ar := a[k]
		if ar == br {
			continue
		}
		if caseSensitive {
			return false
		}
		if !foldsWith(ar, br) {
			return false
		}
	}
	return true
}

// foldsWith reports whether a and b are in one orbit of simple case
// folding, the test strings.EqualFold makes rune by rune.
func foldsWith(a, b rune) bool {
	for r := unicode.SimpleFold(a); r != a; r = unicode.SimpleFold(r) {
		if r == b {
			return true
		}
	}
	return false
}

// byteToRune converts a byte offset in s to a rune offset, as regexp
// reports bytes and a match addresses runes.
func byteToRune(s string, at int) int {
	return utf8.RuneCountInString(s[:at])
}
