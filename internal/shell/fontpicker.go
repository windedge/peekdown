package shell

import (
	"strings"
	"sync"

	"github.com/egoist/mygo/ui"
)

// Which font setting the picker edits.
const (
	fontContent = 0
	fontMono    = 1
)

// The picker's fixed geometry: the panel's footprint and the row height
// the list virtualizes against.
const (
	fontPickerWidth  = 320
	fontPickerHeight = 360
	fontRowHeight    = float32(32)
)

var (
	fontFamiliesOnce sync.Once
	fontFamilies     []ui.FontFamily
)

// fontPickerList builds the family list once: registered families first,
// then the system's, each once, sorted regardless of case.
func fontPickerList() []ui.FontFamily {
	fontFamiliesOnce.Do(func() { fontFamilies = ui.FontFamilies() })
	return fontFamilies
}

// openFontPicker opens the popover anchored to the trigger pressed: it
// resets the search so the panel starts on the whole list, and closes
// the other trigger's popover.
func (s *Shell) openFontPicker(open *bool) {
	if open == &s.fontPickOpen {
		s.fontPickMonoOpen = false
	} else {
		s.fontPickOpen = false
	}
	*open = true
	s.fontSearch = ""
	s.fontList = ui.ListState{}
}

// fontPicker draws the searchable font picker as a popover anchored to
// the font trigger that opened it, built after the settings dialog so it
// stacks over it. Dismissing it only closes it: like every dialog field,
// the picker writes the draft, and Save persists it. Without a scrim, a
// press outside reaches what it went down on, as a select's popup does;
// a press on the settings backdrop therefore lands on the modal, whose
// dismissal the caller reverts when it carries this panel's.
func (s *Shell) fontPicker(c *ui.Context, anchor ui.Element, open *bool, which int) {
	if !*open || !anchor.Valid() {
		return
	}
	choices := s.fontChoices()
	// The query leaves the list as rows go, so it filters here, before
	// the count reaches the list and its virtualizer.
	q := s.fontSearch
	visible := make([]ui.FontFamily, 0, len(choices))
	for _, f := range choices {
		if fontFilter(f, q) {
			visible = append(visible, f)
		}
	}

	t := c.Theme()
	ui.PopoverBase(c, anchor, open, func(panel ui.Element) {
		panel.Label("Font picker").Width(fontPickerWidth).MaxHeight(fontPickerHeight).
			Radius(10).Background(t.Surface).BorderWidth(1).BorderColor(t.Border).
			Shadow(0, 8, 24, 0, t.Border).Clip().Margin(6, 0, 0, 0)
		panel.Children(func() {
			title := "Content Font"
			if which == fontMono {
				title = "Code Font"
			}
			ui.Column(c).FillWidth().Height(fontPickerHeight).Padding(12).Gap(8).Children(func() {
				ui.Text(c, title).FontSize(14).Bold()

				// The picker's own field writes the search text;
				// AutoFocus puts the keyboard here, as a font dialog's
				// field expects.
				ui.SearchField(c, &s.fontSearch).Placeholder("Search installed fonts...").AutoFocus()

				// The hairline between the field and the list keeps its
				// place while the list scrolls: the rows slide under the
				// rule, never over the field.
				ui.Box(c).FillWidth().Height(1).Background(t.Border)

				if len(visible) == 0 {
					ui.Text(c, "No matching fonts").FontSize(13).TextColor(t.TextMuted).Padding(24, 16)
					return
				}
				ui.List(c, &s.fontList, len(visible), func(i int) {
					s.fontPickerRow(c, visible[i], which)
				}).Grow(1)
			})
		})
	})
}

// fontChoices maps the cached families to the picker's rows: the system
// default first, drawn as a name and stored as the draft's empty family.
func (s *Shell) fontChoices() []ui.FontFamily {
	out := make([]ui.FontFamily, 0, len(fontPickerList())+1)
	out = append(out, ui.FontFamily{Name: ""})
	return append(out, fontPickerList()...)
}

// fontPickerRow draws one row of the picker's list.
func (s *Shell) fontPickerRow(c *ui.Context, f ui.FontFamily, which int) {
	t := c.Theme()
	// The empty family is the system default: drawn as a name, stored as
	// the draft's empty string.
	name := f.Name
	if name == "" {
		name = "System Default"
	}
	// The row's accessible name carries " row" past the text it shows:
	// a trigger shows the same family's name beside it, and a click on
	// the name must not reach one of them by accident.
	rowName := name + " row"
	current := s.fontPickValue(which) == f.Name
	// PaddingEnd clears the list's overlay scroll bar (9 DIPs in from
	// the row's right edge) with room for the check beside it, so the
	// bar never paints over the choice's mark.
	row := ui.ButtonBase(c.Key("font-row-"+name)).Label(rowName).
		Height(fontRowHeight).FillWidth().Padding(0, 10).PaddingEnd(18).
		Radius(8).AlignItems(ui.Center).Justify(ui.Start).Cursor(ui.CursorPointer)
	switch {
	case current:
		row.Background(t.Selection)
	case row.Hovered():
		row.Background(t.SurfaceHover)
	}
	row.Children(func() {
		// Grow with a zero floor keeps the name block inside the row's
		// padding, so a long family ends at the padding and clears the
		// list's overlay scroll bar.
		label := ui.Text(c, name).FontSize(13).SingleLine().Ellipsis("...").
			Grow(1).MinWidth(0)
		// Each family draws in itself, as a font dialog shows the face
		// next to its name; the system default keeps the dialog's font.
		if f.Name != "" {
			label.Font(f.Name)
		}
		// A compact "Mono" chip marks the families a code font needs.
		if f.Monospaced {
			ui.Box(c).Padding(2, 6).Radius(4).Background(t.Border).Children(func() {
				ui.Text(c, "Mono").FontSize(9).TextColor(t.TextMuted)
			})
		}
		// The draft's own value carries the check: the row the picker
		// would close on, as a select shows its choice. The labeled box
		// just lets the tests reach the mark's box.
		if current {
			ui.Box(c).Label("check " + rowName).Children(func() {
				ui.Icon(c, checkIcon).Size(14, 14).TextColor(t.Text)
			})
		}
	})
	if row.Clicked() {
		s.setFontPickValue(which, f.Name)
		s.fontPickOpen, s.fontPickMonoOpen = false, false
	}
}

// fontPickValue is the draft family the picker edits.
func (s *Shell) fontPickValue(which int) string {
	if which == fontMono {
		return s.Draft.MonoFontFamily
	}
	return s.Draft.FontFamily
}

// setFontPickValue writes the draft family the picker edits.
func (s *Shell) setFontPickValue(which int, v string) {
	if which == fontMono {
		s.Draft.MonoFontFamily = v
		return
	}
	s.Draft.FontFamily = v
}

// fontFilter passes a family whose name holds every word of the query,
// case folded: "se go ui" finds Segoe UI.
func fontFilter(f ui.FontFamily, q string) bool {
	for _, w := range strings.Fields(strings.ToLower(strings.TrimSpace(q))) {
		if !strings.Contains(strings.ToLower(f.Name), w) {
			return false
		}
	}
	return true
}
