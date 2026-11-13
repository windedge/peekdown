package view

import (
	"strings"
	"testing"
)

// TestCodeBlockClearsScrollbar keeps a scrolling code block's last line
// off the overlay scroll bar: the bar paints over the scroller's last 9
// DIPs, so the block's bottom padding sits inside the scroller and the
// text ends at least 12 DIPs above its bottom edge.
func TestCodeBlockClearsScrollbar(t *testing.T) {
	long := "func main() { println(\"" + strings.Repeat("wide ", 40) + "\") }"
	_, tt, _ := frame(t, "```go\n"+long+"\n```\n", 600, 300)

	scroll, ok := tt.Find("Code block")
	if !ok {
		t.Fatalf("texts %v: want the code block's scroller", tt.Texts())
	}
	line := rectOf(t, tt, long)
	if got := scroll.Y + scroll.H - (line.Y + line.H); got < 12 {
		t.Fatalf("the code line ends %v DIPs above the scroller's bottom, want >= 12 (the bar's strip)", got)
	}
}
