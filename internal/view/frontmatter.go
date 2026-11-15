package view

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/doc"
	"github.com/windedge/peekdown/locales"
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
		return locales.CopyTOML()
	}
	return locales.CopyYAML()
}

// Frontmatter builds the frontmatter card the document opens with, when
// it has one: a bordered key-value table for a YAML mapping, a plain
// monospace code block for TOML or a YAML that failed to parse. The copy
// button runs st.CopyFrontmatter with the block's Raw text; the field
// nil hides the button. Selection over the card is not supported: the
// runs table holds the document body alone, so the card is outside the
// selection and the search, and copying goes through the button.
func Frontmatter(c *ui.Context, m *doc.Model, st *State) {
	front := m.Front
	if front == nil || (front.Format != "toml" && !front.Broken && len(front.Order) == 0) {
		// No card for no frontmatter, and none for an empty or
		// comment-only block: it parses fine but has no rows to show,
		// and a bare frame over the body reads as a bug.
		return
	}
	tk := st.tokens(c)
	if st.ContentWidth > 0 {
		// The card sits above the list, outside its rows, so it takes the
		// same cap and centering the rows do; otherwise it would span the
		// window over a column capped at ContentWidth.
		ui.Row(c).FillWidth().Justify(ui.Center).Children(func() {
			ui.Column(c).Grow(1).MaxWidth(st.ContentWidth).Children(func() {
				frontmatterCard(c, front, st.CopyFrontmatter, tk, st)
			})
		})
		return
	}
	frontmatterCard(c, front, st.CopyFrontmatter, tk, st)
}

// frontmatterCard builds the card: a bordered key-value table for a YAML
// mapping, a plain monospace code block for TOML or a YAML that failed to
// parse. The copy button runs copy with the block's Raw text; nil copy
// hides the button.
func frontmatterCard(c *ui.Context, front *doc.Frontmatter, copy func(string), tk docTokens, st *State) {
	// The right margin is the rows' scrollbar clearance: the card sits
	// above the list, outside the rows that carry it, so it takes the
	// same room and its frame ends where their text does. The inner 26 is
	// the card's own padding, keeping the copy button off the frame. The
	// bottom margin is the block gap the rows take, so the card clears
	// the first block below it.
	table := ui.Column(c).Background(tk.panel).Radius(tk.radius).Border(1, tk.border).Padding(10, 26, 10, 10).
		Margin(0, rowClearance, blockGap, 0).AlignSelf(ui.Stretch).MinWidth(0).
		Label("Frontmatter card")
	table.Children(func() {
		// The header: the format tag on the left, the fold and copy buttons
		// on the right. The tag names the language a code block below would
		// carry; the Rust version labels its fallback the same way.
		head := ui.Row(c).Gap(8).AlignItems(ui.Center).AlignSelf(ui.Stretch)
		head.Children(func() {
			ui.Text(c, front.Format).FontSize(11).TextColor(tk.muted).Grow(1)
			// The fold toggle shows the state the card is in: an up
			// chevron while open, a down one while folded. The click only
			// flips a bool, but the flip must reach this build for the
			// frame to change, and mygo rebuilds a pass when a click was
			// taken, so the fold applies the same frame.
			fold := ui.ButtonBase(c.Key("frontmatter-fold")).Label(locales.ToggleFrontmatter()).Padding(4).Radius(5).
				AlignItems(ui.Center).Tooltip(locales.ToggleFrontmatter())
			if fold.Hovered() {
				fold.Background(c.Theme().SurfaceHover)
			}
			icon := chevronUpIcon
			if st.FrontCollapsed {
				icon = chevronDownIcon
			}
			fold.Children(func() {
				ui.Icon(c, icon).Size(12, 12).TextColor(tk.muted)
			})
			if fold.Clicked() {
				st.FrontCollapsed = !st.FrontCollapsed
			}
			if copy == nil {
				return
			}
			// The icon-only button matches the code block's copy button:
			// the label keeps naming it for clicks and the screen reader,
			// the tooltip says the action the icon alone does not. No
			// copied feedback; the card sits outside the blocks the
			// copied() bookkeeping indexes by. The copy works folded too:
			// Raw is the block's text, nothing the fold hides.
			label := copyLabel(front)
			b := ui.ButtonBase(c).Label(label).Padding(4).Radius(5).
				AlignItems(ui.Center).Tooltip(label)
			if b.Hovered() {
				b.Background(c.Theme().SurfaceHover)
			}
			b.Children(func() {
				ui.Icon(c, copyIcon).Size(12, 12).TextColor(tk.muted)
			})
			if b.Clicked() {
				copy(front.Raw)
			}
		})
		if st.FrontCollapsed {
			// Folded: the header alone. Copy still copies Raw, so nothing
			// else survives the fold.
			return
		}
		if front.Broken || front.Format == "toml" {
			// No table for what has no key-value mapping: the raw lines
			// as monospaced text at the code's size, no token highlight
			// (the fallback is a reader convenience, not an editor).
			text := strings.TrimRight(front.Raw, "\n")
			if strings.TrimSpace(text) == "" {
				text = " "
			}
			spans := []ui.Span{{Text: text, Color: tk.muted}}
			ui.RichText(c, spans...).Font(tk.monoFamily()).FontSize(tk.monoSize).MinWidth(0)
			return
		}
		rows(c, front, tk)
	})
}

// rows builds the key-value table: one row per key in source order, the
// key column as wide as the widest key, the values in a second column
// the rows share, so the values line up. A very wide key column is
// capped at half the card, where the capped keys ellipsize rather than
// crowd the values off the card.
func rows(c *ui.Context, front *doc.Frontmatter, tk docTokens) {
	keys := front.Order
	widths := make([]float32, len(keys))
	for i, key := range keys {
		w, _ := ui.ShapeText(key, ui.Font{Family: tk.monoFamily(), Size: tk.monoSize}, 10000).Size()
		widths[i] = w
	}
	keyWidth := slices.Max(widths)
	for _, key := range keys {
		val := front.Values[key]
		r := ui.Row(c).Gap(fmKeyGap).AlignItems(ui.Start).AlignSelf(ui.Stretch).MinWidth(0)
		r.Children(func() {
			// The key in muted mono, its own fixed column: the value of
			// every row starts at the same x. The value keeps the mono
			// face too, as the Rust version draws the card in mono.
			ui.Text(c, key).Font(tk.monoFamily()).FontSize(tk.monoSize).Width(keyWidth).TextColor(tk.muted).
				Shrink(0).MaxWidthPercent(50).SingleLine()
			ui.Text(c, valueText(val)).Font(tk.monoFamily()).FontSize(tk.monoSize).TextColor(tk.text).
				Grow(1).MinWidth(0).Tooltip(valueFull(val))
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
