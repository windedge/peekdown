package shell

import (
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/locales"
)

// settingsDialog renders the modal settings form bound to s.Draft.
//
// Hidden from this build: an FPS switch, whose frame-rate readout this
// build has no counterpart for.
func (s *Shell) settingsDialog(c *ui.Context) {
	m := ui.Modal(c, &s.SettingsOpen, func() {
		// The font popovers draw over the dialog: mygo stacks overlays in
		// build order, so a layer built inside it sits on top, and the
		// anchor needs the trigger's frame, which only exists here.
		t := c.Theme()
		// Save stays in the window whatever its height: the fields scroll
		// in the room the modal panel leaves once its padding, this
		// column's chrome (title, buttons, gaps, padding) and the modal's
		// own margin are taken out, and the buttons sit below the scroll
		// rather than inside it.
		_, winH := c.Size()
		fields := max(winH-settingsRoom, settingsMinFields)
		ui.Column(c).Width(480).Padding(16).Gap(12).Children(func() {
			ui.Text(c, locales.SettingsTitle()).FontSize(18).Bold()

			ui.Scroll(c).Label("Settings fields").MaxHeight(fields).Children(func() {
				ui.Column(c).Gap(12).Children(func() {
					s.groups(c)
				})
			})

			if s.saveErr != "" {
				ui.Text(c, s.saveErr).FontSize(12).TextColor(t.Danger)
			}

			// The buttons hug the dialog's right edge, as a dialog's own
			// footer does. A row that keeps to its content would leave
			// them at the left, bunched under the title.
			ui.Row(c).Justify(ui.End).Gap(14).Children(func() {
				if ui.Button(c, locales.Cancel()).Clicked() {
					s.fontPickOpen = false
					s.fontPickMonoOpen = false
					s.SettingsOpen = false
					s.saveErr = ""
					s.Draft = s.Saved
				}
				if ui.PrimaryButton(c, locales.Save()).Clicked() {
					s.fontPickOpen = false
					s.fontPickMonoOpen = false
					s.Draft.ProjectRootMarkers = splitMarkers(s.markersText)
					if err := s.Apply(s.Draft); err != nil {
						s.saveErr = err.Error()
					} else {
						s.SettingsOpen = false
						s.saveErr = ""
					}
				}
			})
		})
	})
	if m.Dismissed() {
		// A press outside a popover goes on to what it went down on, so a
		// dismissal that carries a popover's press belongs to the popover:
		// hand the draft back instead of ending the settings' edits. The
		// modal itself has no way to say what its press hit, so the open
		// states stand in for the answer.
		popoverWas := s.fontPickOpen || s.fontPickMonoOpen
		if popoverWas {
			s.fontPickOpen, s.fontPickMonoOpen = false, false
			s.Draft = s.before
			s.saveErr = ""
		} else {
			s.Draft = s.Saved
			s.saveErr = ""
		}
	}
}

// settingsRoom is the height the dialog spends outside its scrolling
// fields: the modal's margin and panel padding, plus this column's
// padding, title, buttons and gaps.
const settingsRoom = 200

// settingsMinFields keeps the scrolling fields usable in a window too
// short for the dialog's own chrome.
const settingsMinFields = 80

// groups renders the settings sections inside the dialog's scroll area.
func (s *Shell) groups(c *ui.Context) {
	s.group(c, locales.SettingsSectionAppearance(), func() {
		s.field(c, locales.SettingsTheme(), func() {
			ui.Select(c, &s.Draft.Theme, []string{"auto", "light", "dark"})
		})
		s.field(c, locales.SettingsLayout(), func() {
			s.draftLayout = layoutIndex(s.Draft.Layout)
			ui.Segmented(c, &s.draftLayout, locales.LayoutCentered(), locales.LayoutFullWidth()).Label(locales.SettingsLayout())
			s.Draft.Layout = layoutName(s.draftLayout)
		})
		s.field(c, locales.SettingsLanguage(), func() {
			s.languageSelect(c)
		})
	})

	s.group(c, locales.SettingsSectionEditor(), func() {
		s.field(c, locales.SettingsContentFont(), func() {
			s.fontTrigger(c, locales.SettingsContentFont(), s.Draft.FontFamily, &s.fontPickOpen, fontContent)
			s.fontSize(c, locales.ContentFontSize(), &s.Draft.FontSize, 12, 24)
		})
		s.field(c, locales.SettingsCodeFont(), func() {
			s.fontTrigger(c, locales.SettingsCodeFont(), s.Draft.MonoFontFamily, &s.fontPickMonoOpen, fontMono)
			s.fontSize(c, locales.CodeFontSize(), &s.Draft.MonoFontSize, 10, 18)
		})
		// A stepper rather than a slider: the field row shares the font
		// sizes' control, and half steps read better than a drag here.
		s.field(c, locales.SettingsScrollSpeed(), func() {
			ui.NumberInput(c, &s.Draft.ScrollSpeed, 0.5, 3.0, 0.5).Label(locales.SettingsScrollSpeed())
		})
		s.field(c, locales.SettingsInertiaScroll(), func() {
			ui.Switch(c, &s.Draft.InertiaScroll).Label(locales.SettingsInertiaScroll())
		})
	})

	s.group(c, locales.SettingsSectionFiles(), func() {
		s.field(c, locales.SettingsExplorerRoot(), func() {
			s.draftExplorer = explorerIndex(s.Draft.ExplorerRootMode)
			ui.Segmented(c, &s.draftExplorer, locales.ExplorerCurrentDirectory(), locales.ExplorerProjectDirectory()).Label(locales.SettingsExplorerRoot())
			s.Draft.ExplorerRootMode = explorerName(s.draftExplorer)
		})
		// The markers row exists only when the root is the project: with
		// the explorer rooted at the current directory there is no root to
		// mark. The draft keeps the text either way, so toggling the
		// segmented back and forth never loses what was typed.
		if s.Draft.ExplorerRootMode == "project_root" {
			s.field(c, locales.SettingsProjectRootMarkers(), func() {
				// Constrained like the font dropdown above: left free the input
				// fills the row and its text runs under the fields' scroll bar.
				// Label names it, here and for assistive technology.
				ui.TextInput(c, &s.markersText).Grow(1).MaxWidth(292).Label(locales.SettingsProjectRootMarkers())
			})
		}
	})

	s.group(c, locales.SettingsSectionSystem(), func() {
		// mygo's Switch draws no text of its own, so the row carries the
		// visible label; the switch keeps the same name for assistive
		// technology.
		s.field(c, locales.SettingsAutoRefresh(), func() {
			ui.Switch(c, &s.Draft.AutoRefresh).Label(locales.SettingsAutoRefresh())
		})
	})
}

// languageTags are the language select's choices: "" follows the system,
// as mygo's locale resolution does without an override.
var languageTags = []string{"", "en", "zh-Hans"}

// languageName shows a tag as its chooser reads it: the system choice by
// its meaning, the others by their own name, in any language.
func languageName(tag string) string {
	switch tag {
	case "en":
		return locales.LanguageEnglish()
	case "zh-Hans":
		return locales.LanguageSimplifiedChinese()
	}
	return locales.LanguageSystem()
}

// languageSelect renders the language drop-down. Unlike the dialog's
// other fields it does not wait for Save: mygo persists the choice
// itself and applies it at once, redrawing the whole window through
// OnLocaleChanged, so a draft round-trip could never show it.
func (s *Shell) languageSelect(c *ui.Context) {
	chosen := ""
	if s.AppLocale != nil {
		if v := s.AppLocale(); slices.Contains(languageTags, v) {
			chosen = v
		}
	}
	sel := ui.SelectBase(c.Key("language"), &chosen)
	sel.Trigger.Height(30).MaxWidth(132).Justify(ui.SpaceBetween).AlignItems(ui.Center).
		Padding(6, 10).Radius(6).
		BorderWidth(1).BorderColor(c.Theme().Border).Background(c.Theme().Background)
	sel.Trigger.Children(func() {
		ui.Text(c, languageName(chosen)).FontSize(13).SingleLine().Ellipsis("...").Grow(1).MinWidth(0)
		ui.Icon(c, chevronDownIcon).Size(14, 14).TextColor(c.Theme().TextMuted)
	})
	sel.Popup(func(panel ui.Element) {
		panel.MinWidth(160).Padding(4).Radius(6).Background(c.Theme().Surface).
			BorderWidth(1).BorderColor(c.Theme().Border).Shadow(0, 4, 12, 0, c.Theme().Border)
		for _, tag := range languageTags {
			item := sel.Item(tag).Height(28).Padding(0, 10).Radius(4).AlignItems(ui.Center)
			if item.Highlighted() {
				item.Background(c.Theme().Selection)
			}
			item.Children(func() {
				if tag == chosen {
					ui.Icon(c, checkIcon).Size(12, 12).TextColor(c.Theme().Accent)
				} else {
					ui.Box(c).Size(12, 12)
				}
				ui.Text(c, languageName(tag)).FontSize(13).SingleLine().Grow(1)
			})
		}
	})
	if sel.Trigger.Changed() && s.SetAppLocale != nil {
		s.SetAppLocale(chosen)
	}
}

// group lays out one settings section: a muted caption over a rule,
// then the fields.
func (s *Shell) group(c *ui.Context, title string, fields func()) {
	t := c.Theme()
	ui.Column(c).Gap(8).Children(func() {
		ui.Column(c).Gap(4).Children(func() {
			ui.Text(c, title).FontSize(11).Bold().TextColor(t.TextMuted)
			ui.Box(c).Fill().Height(1).Background(t.Border)
		})
		fields()
	})
}

// field lays out one setting: the label in a fixed-width column, the
// control next to it, so every control lines up. The label column fits
// "Project Root Markers", the longest (129 DIPs), with room to spare,
// and keeps the widest row, a font's dropdown and size spinner, 12 DIPs
// off the dialog's right edge: the scroll bar's strip is the last 9, so
// the spinner's "+" stays clear of it. The gap stays at 8 so the two fit.
func (s *Shell) field(c *ui.Context, label string, control func()) {
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, label).Width(136).Shrink(0)
		control()
	})
}

// fontSize renders a font's size spinner, named so a reader tells the
// two font fields apart and the dialog's layout test finds its place.
func (s *Shell) fontSize(c *ui.Context, name string, value *float64, lo, hi float64) {
	ui.Box(c).Label(name).Children(func() {
		ui.NumberInput(c, value, lo, hi, 1)
	})
}

// fontTrigger draws the font field's button: the draft's family, or
// "System Default" where none is set, and the dropdown's chevron. It
// opens the searchable picker for that field; the draft keeps the old
// value until the picker's own row click replaces it. Capped like the
// select it replaced, 132 DIPs: grown to the row it would push the size
// spinner's "+" into the fields' scroll bar, and the spinner is
// shrink-proof, so the row would overflow instead.
func (s *Shell) fontTrigger(c *ui.Context, name, family string, open *bool, which int) {
	t := c.Theme()
	display := family
	if display == "" {
		display = locales.SystemDefault()
	}
	b := ui.ButtonBase(c.Key("font-"+name)).Label(locales.ChooseFont(name)).
		Height(30).MaxWidth(132).Justify(ui.SpaceBetween).AlignItems(ui.Center).
		Padding(6, 10).Radius(6).
		BorderWidth(1).BorderColor(t.Border).Background(t.Background)
	if b.Hovered() || *open {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		ui.Text(c, display).FontSize(13).SingleLine().Ellipsis("...").Grow(1).MinWidth(0)
		// The chevron points up while the popover is open, as a select's
		// trigger does.
		icon := chevronDownIcon
		if *open {
			icon = chevronUpIcon
		}
		ui.Icon(c, icon).Size(14, 14).TextColor(t.TextMuted)
	})
	if b.Clicked() {
		s.openFontPicker(open)
	}
	s.fontPicker(c, b, open, which)
}

func splitMarkers(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
