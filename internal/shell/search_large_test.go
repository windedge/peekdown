package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/windedge/peekdown/internal/doc"
)

// TestSearchLargeDocument runs the bar over a document of about two
// megabytes: the count is exact, a query past the cap stops at the limit
// of 10000, and one pass stays far inside the interactive budget.
func TestSearchLargeDocument(t *testing.T) {
	s, dir := newShell(t)
	// About 300000 words at ten bytes and a space each, near 3 MB with
	// the breaks, whose half the words-per-line fills.
	src, needles := doc.NeedleWords(300000, 500)
	path := filepath.Join(dir, "large.md")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tt := s.renderSize(t, 800, 600)
	if err := s.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	s.OpenSearch()
	tab := s.ActiveTab()
	if tab == nil {
		t.Fatal("no tab for the large document")
	}

	start := time.Now()
	s.search.Query = "needle"
	s.runSearch(tab)
	elapsed := time.Since(start)
	tt.Frame()
	if len(tab.Model.Matches) != needles {
		t.Fatalf("matches = %d, want %d", len(tab.Model.Matches), needles)
	}
	if tab.Model.Current != 0 || s.search.Count != fmt.Sprintf("1/%d", needles) {
		t.Fatalf("current %d count %q, want 0 and 1/%d", tab.Model.Current, s.search.Count, needles)
	}
	if elapsed > 10*time.Second {
		t.Fatalf("one pass took %v, far past any interactive budget", elapsed)
	}

	// A query past the cap: the scan stops at the limit, and the bar
	// shows a full 10000 rather than a total.
	s.search.Query = "e"
	s.runSearch(tab)
	tt.Frame()
	if len(tab.Model.Matches) != 10000 {
		t.Fatalf("truncated matches = %d, want the cap 10000", len(tab.Model.Matches))
	}
	if !tt.HasText("1/10000") {
		t.Fatalf("count %q, want 1/10000: %v", s.search.Count, tt.Texts())
	}

	tab.Model.NextMatch()
	if tab.Model.Current != 1 {
		t.Fatalf("next in %d matches went to %d", len(tab.Model.Matches), tab.Model.Current)
	}

	// Closing drops the 10000 highlights with the bar.
	s.CloseSearch()
	if len(tab.Model.Matches) != 0 || tab.Model.Query() != "" {
		t.Fatalf("close kept the search: %d matches %q", len(tab.Model.Matches), tab.Model.Query())
	}
}
