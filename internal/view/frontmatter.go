package view

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/doc"
)

// The frontmatter renders above the list, outside the virtualization: it
// is usually under 20 rows, one extra build costs nothing against the
// virtual list it sits over, and the ScrollView's List would otherwise
// have to know about a zeroth non-document row.

// maxListRunes is where an inline list or map value is cut: past it the
// value shows with an ellipsis and keeps its full text for the tooltip.
// The Rust version folds long values instead; this reader simplifies to
// a cut and reads the rest through the copy button.
const maxListRunes = 120

// fmKeyGap is the gutter between the card's key and value columns; the
// alignment test reads it, so the spacing stays in one place.
const fmKeyGap = float32(16)

// copyLabel names the copy button for the card's format: the YAML table
// and the broken fallback copy YAML, a TOML block copies TOML.
func copyLabel(front *doc.Frontmatter) string {
	if front.Format == "toml" {
		return "Copy TOML"
	}
	return "Copy YAML"
}

// Frontmatter builds the frontmatter card the document opens with, when
// it has one: a bordered key-value table for a YAML mapping, a plain
// monospace code block for TOML or a YAML that failed to parse. The copy
// button runs st.CopyFrontmatter with the block's Raw text; the field
// nil hides the button. Selection over the card is not supported: the
// runs table holds the document body alone, so the card is outside the
// selection and the search, and copying goes through the button.
func Frontmatter(c *ui.Context, m *doc.Model, st *State) {
	FrontmatterElement(c, m.Front, st.CopyFrontmatter)
}

// FrontmatterElement builds the card of front, nil for none, so a test
// can drive it alone. copy runs on click, with Raw; nil copy hides the
// button. The callback answers the click inside the build, the way a
// button's click is answered anywhere, so the clipboard and the toast
// the host keeps never hold a stale element.
func FrontmatterElement(c *ui.Context, front *doc.Frontmatter, copy func(string)) {
	if front == nil || (front.Format != "toml" && !front.Broken && len(front.Order) == 0) {
		// No card for no frontmatter, and none for an empty or
		// comment-only block: it parses fine but has no rows to show,
		// and a bare frame over the body reads as a bug.
		return
	}
	t := c.Theme()
	table := ui.Column(c).Background(t.Surface).Radius(6).Border(1, t.Border).Padding(10).MarginY(6).
		AlignSelf(ui.Stretch).MinWidth(0)
	table.Children(func() {
		// The header: the format tag on the left, the copy button on the
		// right. The tag names the language a code block below would
		// carry; the Rust version labels its fallback the same way.
		head := ui.Row(c).Gap(8).AlignItems(ui.Center).AlignSelf(ui.Stretch)
		head.Children(func() {
			ui.Text(c, front.Format).FontSize(11).TextColor(t.TextMuted).Grow(1)
			if copy != nil && ui.Button(c, copyLabel(front)).FontSize(11).Clicked() {
				copy(front.Raw)
			}
		})
		if front.Broken || front.Format == "toml" {
			// No table for what has no key-value mapping: the raw lines
			// as a monospace block, no token highlight (the fallback is
			// a reader convenience, not an editor).
			text := strings.TrimRight(front.Raw, "\n")
			if strings.TrimSpace(text) == "" {
				text = " "
			}
			spans := []ui.Span{{Text: text, Color: t.TextMuted}}
			ui.RichText(c, spans...).Font(fontMono).FontSize(13).NoWrap().MinWidth(0)
			return
		}
		rows(c, front, t)
	})
}

// rows builds the key-value table: one row per key in source order, the
// key column as wide as the widest key, the values in a second column
// the rows share, so the values line up. A very wide key column is
// capped at half the card, where the capped keys ellipsize rather than
// crowd the values off the card.
func rows(c *ui.Context, front *doc.Frontmatter, t *ui.Theme) {
	keys := front.Order
	widths := make([]float32, len(keys))
	for i, key := range keys {
		w, _ := ui.ShapeText(key, ui.Font{Family: fontMono, Size: 13}, 10000).Size()
		widths[i] = w
	}
	keyWidth := slices.Max(widths)
	for _, key := range keys {
		val := front.Values[key]
		r := ui.Row(c).Gap(fmKeyGap).AlignItems(ui.Start).AlignSelf(ui.Stretch).MinWidth(0)
		r.Children(func() {
			// The key in muted mono, its own fixed column: the value of
			// every row starts at the same x.
			ui.Text(c, key).Font(fontMono).FontSize(13).Width(keyWidth).TextColor(t.TextMuted).
				Shrink(0).MaxWidthPercent(50).SingleLine()
			ui.Text(c, valueText(val)).FontSize(13).Grow(1).MinWidth(0).Tooltip(valueFull(val))
		})
	}
}

// valueText renders a value as the row shows it: strings, numbers and
// bools as themselves, null as ~, a list inline with commas, a map as
// k=v pairs, a nested one as {...}; past maxListRunes the text is cut
// with an ellipsis (the full value stays on the tooltip and in Raw, for
// the copy button).
func valueText(val any) string {
	s := inlineValue(val, 0)
	if runes := len([]rune(s)); runes > maxListRunes {
		r := []rune(s)
		return string(r[:maxListRunes]) + "..."
	}
	return s
}

// valueFull returns the value's whole text, for the tooltip and the copy
// button's title attribute; scalars need none.
func valueFull(val any) string {
	if _, isScalar := val.(string); isScalar {
		return ""
	}
	switch val.(type) {
	case float64, bool, nil:
		return ""
	}
	return inlineValue(val, 0)
}

// inlineValue flattens a value to its row text, depth counting the
// nesting: depth 1 opens a map, depth 2 and past it any container is
// {...}, so the row stays one line however deep the document goes.
func inlineValue(val any, depth int) string {
	switch v := val.(type) {
	case nil:
		return "~"
	case bool:
		return strconv.FormatBool(v)
	case float64:
		return numberText(v)
	case string:
		return v
	case []any:
		if depth >= 2 {
			return "{...}"
		}
		parts := make([]string, len(v))
		for i, item := range v {
			parts[i] = inlineValue(item, depth+1)
		}
		return strings.Join(parts, ", ")
	case map[string]any:
		if depth >= 1 {
			return "{...}"
		}
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		parts := make([]string, len(keys))
		for i, k := range keys {
			parts[i] = k + "=" + inlineValue(v[k], depth+1)
		}
		return strings.Join(parts, ", ")
	}
	// An unknown value still shows something: %v beats a silent blank.
	return fmt.Sprintf("%v", val)
}

// numberText prints a number the way YAML means it: whole floats
// without the decimal point, the rest as Go prints them.
func numberText(f float64) string {
	if f == float64(int64(f)) {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}
