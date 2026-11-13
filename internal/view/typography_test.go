package view

import (
	"fmt"
	"image"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/doc"
)

// The typography baseline (ticket #18), compared against the Rust v0.2.0
// reader: the paragraph gap, the inline code chip, the code block's panel,
// the table's rules and the block widths.

// near reports whether a and b are the same layout measure, the tester's
// rectangles being laid out in fractional DIPs.
func near(a, b float32) bool { return a-b < 0.5 && b-a < 0.5 }

// frame renders src into a 640 wide window and returns the ids of the
// texts the tester drew, so a test can read their rectangles.
func frame(t *testing.T, src string, w, h int) (*doc.Model, *ui.Tester, *State) {
	t.Helper()
	m := doc.New(src)
	st := &State{AnchorOf: anchorsOf(m)}
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		ScrollView(c, m, st)
	}, w, h)
	tt.Frame()
	return m, tt, st
}

// countColor counts the image's pixels that are exactly c: a filled
// rectangle of a theme color draws them exactly, so a count of zero means
// nothing drew it.
func countColor(img *image.RGBA, c ui.Color) int {
	n := 0
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, a := img.RGBAAt(x, y).RGBA()
			if uint8(r>>8) == c.R && uint8(g>>8) == c.G && uint8(bl>>8) == c.B && uint8(a>>8) == c.A {
				n++
			}
		}
	}
	return n
}

// rightmostColorX returns the right edge of the last pixel of color c, or
// -1 when nothing drew it.
func rightmostColorX(img *image.RGBA, c ui.Color) int {
	right := -1
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Max.X - 1; x > right; x-- {
			r, g, bl, a := img.RGBAAt(x, y).RGBA()
			if uint8(r>>8) == c.R && uint8(g>>8) == c.G && uint8(bl>>8) == c.B && uint8(a>>8) == c.A {
				if x > right {
					right = x
				}
				break
			}
		}
	}
	return right
}

// pixelIs reports whether the pixel at x, y is exactly c.
func pixelIs(img *image.RGBA, x, y int, c ui.Color) bool {
	r, g, bl, a := img.RGBAAt(x, y).RGBA()
	return uint8(r>>8) == c.R && uint8(g>>8) == c.G && uint8(bl>>8) == c.B && uint8(a>>8) == c.A
}

// colorRuns returns the horizontal runs of the pixels of color c in row y:
// the left and the right edge of each, left to right.
func colorRuns(img *image.RGBA, y int, c ui.Color) [][2]int {
	var runs [][2]int
	b := img.Bounds()
	for x := b.Min.X; x < b.Max.X; x++ {
		if !pixelIs(img, x, y, c) {
			continue
		}
		if n := len(runs); n > 0 && runs[n-1][1] == x-1 {
			runs[n-1][1] = x
			continue
		}
		runs = append(runs, [2]int{x, x})
	}
	return runs
}

// surfaceRows returns the first and the last row of the page's surface
// fills, the widest of them, and -1 when nothing filled.
func surfaceRows(img *image.RGBA, surface ui.Color) (first, last, widest int) {
	first, last, widest = -1, -1, -1
	best := 0
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		n := 0
		for _, r := range colorRuns(img, y, surface) {
			n += r[1] - r[0] + 1
		}
		if n == 0 {
			continue
		}
		if first < 0 {
			first = y
		}
		last = y
		if n > best {
			best, widest = n, y
		}
	}
	return first, last, widest
}

// TestTableHeaderIsOneBand checks the header reads as a band across the
// row, not as a chip per cell: one surface fill, its only interruptions
// the column rules, and no page between the cells.
func TestTableHeaderIsOneBand(t *testing.T) {
	theme := ui.LightTheme()
	_, tt, _ := frame(t, "| aaa | bbb | ccc |\n|---|---|---|\n| 1 | 2 | 3 |\n| 4 | 5 | 6 |\n", 640, 400)
	img := tt.Image()
	_, _, band := surfaceRows(img, theme.Surface)
	if band < 0 {
		t.Fatal("the header's surface did not draw")
	}
	runs := colorRuns(img, band, theme.Surface)
	if len(runs) != 3 {
		t.Fatalf("y=%d: %v, want one surface run per column", band, runs)
	}
	for k := 1; k < len(runs); k++ {
		from, to := runs[k-1][1]+1, runs[k][0]-1
		if to < from {
			t.Fatalf("y=%d: %v, the cells at %d and %d share an edge", band, runs, k-1, k)
		}
		for x := from; x <= to; x++ {
			if !pixelIs(img, x, band, theme.Border) {
				t.Fatalf("y=%d: x=%d between the runs %v is not the column rule: the header is not one band", band, x, runs)
			}
		}
	}
	// The band's own fill never lets the page through between its cells.
	for x := runs[0][0]; x <= runs[len(runs)-1][1]; x++ {
		if pixelIs(img, x, band, theme.Background) {
			t.Fatalf("y=%d: x=%d in the header band is the page, want the surface or a rule (%v)", band, x, runs)
		}
	}
}

// TestTableHeaderFillsInsideTheFrame checks the band's corners follow the
// frame's radius: a square fill would poke out of the rounded frame.
func TestTableHeaderFillsInsideTheFrame(t *testing.T) {
	theme := ui.LightTheme()
	_, tt, _ := frame(t, "| aaa | bbb | ccc |\n|---|---|---|\n| 1 | 2 | 3 |\n| 4 | 5 | 6 |\n", 640, 400)
	img := tt.Image()
	first, last, band := surfaceRows(img, theme.Surface)
	if first < 0 {
		t.Fatal("the header's surface did not draw")
	}
	top := colorRuns(img, first, theme.Surface)
	body := colorRuns(img, band, theme.Surface)
	if len(top) == 0 || len(body) == 0 {
		t.Fatalf("no surface on y=%d or y=%d", first, band)
	}
	if top[0][0] < body[0][0]+2 {
		t.Fatalf("the band starts at x=%d on its first row y=%d and at x=%d on y=%d: its corner is square, the frame's is not",
			top[0][0], first, body[0][0], band)
	}
	if got := top[len(top)-1][1]; got > body[len(body)-1][1]-2 {
		t.Fatalf("the band ends at x=%d on its first row y=%d and at x=%d on y=%d: its corner is square, the frame's is not",
			got, first, body[len(body)-1][1], band)
	}
	// The band is taller than the radius, and its last row is the row rule.
	if last-first < 12 {
		t.Fatalf("the band spans y %d..%d: too thin for the header's padding", first, last)
	}
}

// TestParagraphGapIsTheBlockGap checks the space between two paragraphs:
// the Rust version gives every block 1rem below it, and the rows now carry
// that gap themselves.
func TestParagraphGapIsTheBlockGap(t *testing.T) {
	_, tt, _ := frame(t, "Alpha beta.\n\nSecond paragraph here.\n\nThird one.\n", 640, 400)
	first := rectOf(t, tt, "Alpha beta.")
	second := rectOf(t, tt, "Second paragraph")
	third := rectOf(t, tt, "Third one")
	for _, pair := range []struct {
		from, to ui.Rect
		name     string
	}{{first, second, "first to second"}, {second, third, "second to third"}} {
		gap := pair.to.Y - (pair.from.Y + pair.from.H)
		if !near(gap, blockGap) {
			t.Fatalf("%s paragraph gap %v, want %v (from %v to %v)", pair.name, gap, blockGap, pair.from, pair.to)
		}
	}
}

// TestHeadingGapIsTheBlockGap checks a heading takes the same gap: the
// Rust version gives headings a smaller one, the ticket asks for one gap
// under every block.
func TestHeadingGapIsTheBlockGap(t *testing.T) {
	_, tt, _ := frame(t, "# Title\n\nBody text.\n", 640, 400)
	head := rectOf(t, tt, "Title")
	body := rectOf(t, tt, "Body text")
	if gap := body.Y - (head.Y + head.H); !near(gap, blockGap) {
		t.Fatalf("gap under the heading %v, want %v (heading %v, body %v)", gap, blockGap, head, body)
	}
}

// TestInlineCodeSpanHasChip checks the span itself: code within text is a
// monospaced run on a rounded background with side padding, the Rust
// version's accent highlight, where it used to be only a muted color.
func TestInlineCodeSpanHasChip(t *testing.T) {
	m := doc.New("a `code` and `more` here\n")
	b := m.Blocks()[0]
	var got []ui.Span
	_ = ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		got = (&State{}).spans(c, m, b.Index, style{})
	}, 640, 400)
	theme := ui.LightTheme()
	want := theme.Background.Mix(theme.Accent, inlineCodeTint)
	found := 0
	for _, sp := range got {
		switch sp.Text {
		case "code", "more":
			found++
			if sp.Background != want {
				t.Fatalf("code span %q background %v, want the accent tint %v", sp.Text, sp.Background, want)
			}
			if sp.Radius <= 0 || sp.PaddingX <= 0 {
				t.Fatalf("code span %q: radius %v, padding %v, want both", sp.Text, sp.Radius, sp.PaddingX)
			}
			if sp.Font != fontMono {
				t.Fatalf("code span %q font %q, want mono", sp.Text, sp.Font)
			}
			if sp.Color != theme.Text {
				t.Fatalf("code span %q color %v, want the body text %v", sp.Text, sp.Color, theme.Text)
			}
		}
	}
	if found != 2 {
		t.Fatalf("%d code spans in %v, want 2", found, got)
	}
}

// TestInlineCodePaintsBackground checks the chip reaches the page: the
// tint covers pixels that plain text leaves the page's background.
func TestInlineCodePaintsBackground(t *testing.T) {
	theme := ui.LightTheme()
	want := theme.Background.Mix(theme.Accent, inlineCodeTint)
	_, plain, _ := frame(t, "a code span here\n", 640, 400)
	if n := countColor(plain.Image(), want); n != 0 {
		t.Fatalf("%d tinted pixels without inline code", n)
	}
	_, coded, _ := frame(t, "a `code` span here\n", 640, 400)
	if n := countColor(coded.Image(), want); n < 20 {
		t.Fatalf("%d tinted pixels for the chip, want the background painted", n)
	}
}

// TestInlineCodeChipWidensTheLine checks the chip's side padding takes
// room around the glyphs: the painted chip is wider than its text.
func TestInlineCodeChipWidensTheLine(t *testing.T) {
	theme := ui.LightTheme()
	want := theme.Background.Mix(theme.Accent, inlineCodeTint)
	_, tt, _ := frame(t, "a `code` here\n", 640, 400)
	left, right, ok := colorBounds(tt.Image(), want)
	if !ok {
		t.Fatal("the chip did not paint")
	}
	tw, _ := ui.ShapeText("code", ui.Font{Family: fontMono, Size: 15 * inlineCodeScale}, 10000).Size()
	chip := float32(right - left + 1)
	if chip < tw+2*inlineCodePadding-1.5 {
		t.Fatalf("chip %v wide for %v of text: no padding around it", chip, tw)
	}
}

// colorBounds returns the left and right edges of the pixels of color c.
func colorBounds(img *image.RGBA, c ui.Color) (left, right int, ok bool) {
	left, right = -1, -1
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, a := img.RGBAAt(x, y).RGBA()
			if uint8(r>>8) != c.R || uint8(g>>8) != c.G || uint8(bl>>8) != c.B || uint8(a>>8) != c.A {
				continue
			}
			if left < 0 || x < left {
				left = x
			}
			if x > right {
				right = x
			}
		}
	}
	return left, right, left >= 0
}

// TestCodeBlockPanel checks the panel's parameters: 12 of padding inside,
// the mono size, and one block gap under the block above, not two.
func TestCodeBlockPanel(t *testing.T) {
	_, tt, _ := frame(t, "Intro paragraph.\n\n```go\nx := 1\n```\n", 640, 400)
	intro := rectOf(t, tt, "Intro paragraph")
	code := rectOf(t, tt, "x := 1")
	if got := code.X - intro.X; !near(got, codeBlockPadding) {
		t.Fatalf("code text at %v, %v from the block's left edge, want %v of padding", code.X, got, codeBlockPadding)
	}
	if got := code.Y - (intro.Y + intro.H); !near(got, blockGap+codeBlockPadding) {
		t.Fatalf("code text %v under the paragraph, want the block gap %v plus the panel's %v", got, blockGap, codeBlockPadding)
	}
	if code.H > 16 {
		t.Fatalf("code line %v tall, want the mono size, not the body's", code.H)
	}
}

// TestCodeBlockLongLineKeepsOneLine checks a long line stays one line
// wide, the code block scrolling it sideways (ticket #23 F) instead of
// wrapping it: the panel clips nothing the reader cannot scroll to.
func TestCodeBlockLongLineKeepsOneLine(t *testing.T) {
	long := strings.Repeat("x", 200)
	_, tt, _ := frame(t, "```\n"+long+"\n```\n", 400, 300)
	if !tt.HasText(long) {
		t.Fatalf("texts %v: the long line is missing", tt.Texts())
	}
	line := rectOf(t, tt, long)
	if line.H > 30 {
		t.Fatalf("the long line draws %v tall: it wrapped onto several lines", line.H)
	}
	if line.X+line.W > 400-rowClearance {
		t.Fatalf("the long line reaches x %v in a 400 wide window: it runs past the panel", line.X+line.W)
	}
}

// TestTableRules checks the grid's lines reach the page: rows and columns
// rule in the border color, where the table used to carry only its frame.
func TestTableRules(t *testing.T) {
	theme := ui.LightTheme()
	_, plain, _ := frame(t, "plain text\n", 640, 400)
	if n := countColor(plain.Image(), theme.Border); n != 0 {
		t.Fatalf("%d border pixels without a table", n)
	}
	_, tt, _ := frame(t, "| a | b |\n|---|---|\n| 1 | 2 |\n\n", 640, 400)
	// The frame alone is about 2*(w+h) pixels; the row and column rules
	// draw more than the frame can.
	if n := countColor(tt.Image(), theme.Border); n < 300 {
		t.Fatalf("%d border pixels, want the table's grid ruled", n)
	}
}

// TestTableKeepsItsContentWidth checks the table draws at the width its
// columns measure, not stretched across the page: the Rust version leaves
// the page's right side empty rather than pulling short cells apart.
func TestTableKeepsItsContentWidth(t *testing.T) {
	theme := ui.LightTheme()
	_, tt, _ := frame(t, "| a | b |\n|---|---|\n| 1 | 2 |\n\n", 640, 400)
	// The header's surface fill ends where the table ends.
	right := rightmostColorX(tt.Image(), theme.Surface)
	if right < 0 {
		t.Fatal("the header's surface did not draw")
	}
	if right > 320 {
		t.Fatalf("the table's header fill reaches x %d in a 640 wide window: the table is stretched", right)
	}
	// The second column still sits right of the first, with a rule between.
	first := rectOf(t, tt, "a")
	second := rectOf(t, tt, "b")
	if second.X <= first.X+first.W {
		t.Fatalf("columns at %v and %v: the second does not clear the first", first, second)
	}
}

// TestSplitTableRulesEverySlice checks a split table's later slices stay
// ruled and do not repeat the header: the note closes the slice.
func TestSplitTableRulesEverySlice(t *testing.T) {
	theme := ui.LightTheme()
	m, tt, st := frame(t, giantTableDoc(200), 800, 600)
	if len(m.Blocks()) < 2 {
		t.Skipf("%d slices, want a split table", len(m.Blocks()))
	}
	// A later slice: the second block.
	st.List.ScrollIntoView(1)
	tt.Frame()
	if n := countColor(tt.Image(), theme.Border); n < 300 {
		t.Fatalf("%d border pixels on the second slice, want it ruled too", n)
	}
	for _, s := range tt.Texts() {
		if strings.Contains(s, "rows ") {
			return
		}
	}
	t.Fatalf("texts %v: the slice's note is missing", tt.Texts())
}

// TestListItemsKeepTheListGap checks the block gap stays outside lists: the
// Rust version zeroes it inside a list, and the items keep the list's own
// tight gap.
func TestListItemsKeepTheListGap(t *testing.T) {
	_, tt, _ := frame(t, "- one\n- two\n- three\n", 640, 400)
	one := rectOf(t, tt, "one")
	two := rectOf(t, tt, "two")
	gap := two.Y - (one.Y + one.H)
	if gap >= blockGap {
		t.Fatalf("list items %v apart, want the list's own gap, under the block's %v", gap, blockGap)
	}
	if !strings.Contains(fmt.Sprint(gap), ".") && gap > 8 {
		t.Fatalf("list items %v apart, want them tight", gap)
	}
}

// TestBodyFontSizeFollowsSetting checks the body setting reaches the
// paragraphs: the same line draws taller and wider at a bigger size.
func TestBodyFontSizeFollowsSetting(t *testing.T) {
	m := doc.New("Alpha beta gamma delta echoes foxtrot gradient.")
	line := func(size float32) ui.Rect {
		var st State
		st.FontSize = size
		tt := ui.NewTester(func(c *ui.Context) {
			c.DisableAnimations()
			ScrollView(c, m, &st)
		}, 640, 400)
		tt.Frame()
		return rectOf(t, tt, "Alpha beta")
	}
	small, big := line(12), line(24)
	if big.H <= small.H {
		t.Fatalf("line %v tall at 24, %v at 12: the body size does not reach the text", big.H, small.H)
	}
}

// TestHeadingScalesWithBody checks the headings ride the body size: an
// h1 of a bigger body draws taller, and stays taller than the body line
// at the same setting.
func TestHeadingScalesWithBody(t *testing.T) {
	m := doc.New("# Heading One\n")
	head := func(size float32) ui.Rect {
		var st State
		st.FontSize = size
		tt := ui.NewTester(func(c *ui.Context) {
			c.DisableAnimations()
			ScrollView(c, m, &st)
		}, 640, 400)
		tt.Frame()
		return rectOf(t, tt, "Heading One")
	}
	small, big := head(14), head(24)
	if big.H <= small.H {
		t.Fatalf("h1 %v tall at a 24 body, %v at a 14 body: it does not scale", big.H, small.H)
	}
}

// TestMonoFontSizeFollowsSetting checks the code setting reaches the code
// block: the same line draws taller at a bigger mono size.
func TestMonoFontSizeFollowsSetting(t *testing.T) {
	m := doc.New("```go\nx := 1\n```")
	line := func(size float32) ui.Rect {
		var st State
		st.MonoFontSize = size
		tt := ui.NewTester(func(c *ui.Context) {
			c.DisableAnimations()
			ScrollView(c, m, &st)
		}, 640, 400)
		tt.Frame()
		return rectOf(t, tt, "x := 1")
	}
	small, big := line(9), line(18)
	if big.H <= small.H {
		t.Fatalf("code line %vx%v at 18, %vx%v at 9: the mono size does not reach the text", big.W, big.H, small.W, small.H)
	}
}

// TestFontFamiliesFollowSettings checks the family settings map onto the
// tokens: empty names leave the defaults (the page's sans, monospace),
// set ones ride through to the mono spans and the text elements.
func TestFontFamiliesFollowSettings(t *testing.T) {
	var def, set docTokens
	dflt := &State{}
	custom := &State{FontFamily: "Georgia", MonoFontFamily: "Courier New"}
	tt := ui.NewTester(func(c *ui.Context) {
		def = dflt.tokens(c)
		set = custom.tokens(c)
	}, 200, 100)
	tt.Frame()
	if def.sans != "" || def.mono != "" {
		t.Fatalf("default tokens sans %q mono %q, want both empty, the defaults", def.sans, def.mono)
	}
	if def.monoFamily() != fontMono || set.monoFamily() != "Courier New" {
		t.Fatalf("resolved mono %q / %q, want the default and the setting", def.monoFamily(), set.monoFamily())
	}
	if set.sans != "Georgia" || set.mono != "Courier New" {
		t.Fatalf("set tokens sans %q mono %q, want the settings", set.sans, set.mono)
	}
	if def.body != defaultBodySize || set.body != defaultBodySize {
		t.Fatalf("body size %v then %v, want the default %v", def.body, set.body, defaultBodySize)
	}
}

// TestInlineCodeCarriesTheMonoFamily checks the chip's span takes the
// mono setting's family, not the hardcoded default.
func TestInlineCodeCarriesTheMonoFamily(t *testing.T) {
	m := doc.New("a `code` here\n")
	var st State
	st.MonoFontFamily = "Courier New"
	var got []ui.Span
	_ = ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		got = st.spans(c, m, m.Blocks()[0].Index, style{})
	}, 640, 400)
	for _, sp := range got {
		if sp.Text == "code" {
			if sp.Font != "Courier New" {
				t.Fatalf("code span font %q, want the mono setting", sp.Font)
			}
			return
		}
	}
	t.Fatalf("spans %v: no code span", got)
}

// absDiff is the distance between two color channels.
func absDiff(a, b uint8) int {
	if a > b {
		return int(a) - int(b)
	}
	return int(b) - int(a)
}

// TestInlineCodeChipReadableInBothThemes checks the chip stands off the
// page in either appearance: a tint that faded into the background would
// leave the code as bare text again.
func TestInlineCodeChipReadableInBothThemes(t *testing.T) {
	for _, theme := range []*ui.Theme{ui.LightTheme(), ui.DarkTheme()} {
		chip := theme.Background.Mix(theme.Accent, inlineCodeTint)
		diff := absDiff(chip.R, theme.Background.R) + absDiff(chip.G, theme.Background.G) +
			absDiff(chip.B, theme.Background.B)
		if diff < 12 {
			t.Fatalf("chip %v is %d off the page %v (dark %v): too faint", chip, diff, theme.Background, theme.Dark)
		}
		if chip == theme.Background || chip == theme.Text {
			t.Fatalf("chip %v against the page %v (dark %v)", chip, theme.Background, theme.Dark)
		}
	}
}
