package shell

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui/markdown/parser"

	"github.com/windedge/peekdown/internal/doc"
)

// OpenFile parses without the math extensions (ticket #17): math syntax
// stays plain text, its characters in the run table for search.
func TestOpenFileParsesMathAsPlainText(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "math.md")
	src := "# Math\n\n$$E=mc^2$$\n\nPrice $x+y$ stays.\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	s := New()
	if err := s.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	m := s.Tabs[s.Active].Model
	for _, b := range m.Blocks() {
		if b.Kind == parser.MathBlock {
			t.Fatalf("block %d is a math block; math must stay plain text", b.Index)
		}
	}
	m.SetSearch("E=mc^2", doc.SearchOptions{})
	if len(m.Matches) == 0 {
		t.Fatalf("search misses E=mc^2 in runs %v", m.Runs())
	}
}
