package view

import "testing"

// TestListTextKeepsBodySize checks a list item's prose draws at the body
// size, as a paragraph does. The item used to hand RichText spans with no
// size, and a span without a size inherits the window's font, 14, not the
// body's: every bullet and every nested bullet came out a size down from
// the paragraphs around it. Nesting only indents, it does not shrink.
func TestListTextKeepsBodySize(t *testing.T) {
	src := "HangA\n\n- HangB\n  - HangC\n\nHangD\n"
	_, tt, _ := frame(t, src, 640, 400)
	labels := []string{"HangA", "HangB", "HangC", "HangD"}
	want := rectOf(t, tt, labels[0]).H
	for _, label := range labels[1:] {
		if got := rectOf(t, tt, label).H; !near(got, want) {
			t.Fatalf("%s line height %v, the paragraph's %v: the list item drew a size down", label, got, want)
		}
	}
}

// TestListRunsKeepBodySize checks a list item's inline runs scale from the
// body too: code, bold and a link each keep the line height the same run
// takes in a paragraph. With the item's spans unsized, the code chip
// multiplied zero and every run came out at the window's font.
func TestListRunsKeepBodySize(t *testing.T) {
	src := "`HangA`\n\n- `HangB`\n\n**HangC**\n\n- **HangD**\n\n[HangE](x)\n\n- [HangF](y)\n"
	_, tt, _ := frame(t, src, 640, 700)
	pairs := [][2]string{
		{"HangA", "HangB"},
		{"HangC", "HangD"},
		{"HangE", "HangF"},
	}
	for _, p := range pairs {
		para, item := rectOf(t, tt, p[0]).H, rectOf(t, tt, p[1]).H
		if !near(para, item) {
			t.Fatalf("%s in a list draws at line height %v, the paragraph's %v", p[1], item, para)
		}
	}
}
