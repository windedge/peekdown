package shell

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/settings"
	"github.com/windedge/peekdown/locales"
)

// The settings dialog's fixed geometry, in mojian's settings' measures:
// the panel is 560 DIPs wide, every section is one rounded card, and the
// two-line rows keep to the same paddings and type sizes.
const (
	settingsWidth    = 560
	settingsPadding  = 20
	settingsCardGap  = 12
	settingsCardPadX = 16
	settingsRowGap   = 16
	settingsDebounce = 300 * time.Millisecond
	settingsMarkersW = 200
)

// settingsDialog renders the settings modal: every control applies at
// once through persist (no Save button), the Done button just closes.
func (s *Shell) settingsDialog(c *ui.Context) {
	m := ui.Modal(c, &s.SettingsOpen, func() {
		// The font popovers draw over the dialog: mygo stacks overlays in
		// build order, so a layer built inside it sits on top, and the
		// anchor needs the trigger's frame, which only exists here.
		t := c.Theme()
		winW, winH := c.Size()
		fields := max(winH-settingsRoom, settingsMinFields)
		ui.Column(c).Width(min(settingsWidth, winW-48)).Padding(settingsPadding).Gap(14).Children(func() {
			ui.Text(c, locales.SettingsTitle()).FontSize(15).FontWeight(650).TextColor(t.Text)

			ui.Scroll(c).Label("Settings fields").MaxHeight(fields).Children(func() {
				ui.Column(c).Gap(settingsCardGap).PaddingEnd(12).Children(func() {
					s.groups(c)
				})
				if s.saveErr != "" {
					ui.Text(c, s.saveErr).FontSize(12).TextColor(t.Danger)
				}
			})
			if ui.PrimaryButton(c, locales.SettingsDone()).Clicked() {
				s.fontPickOpen, s.fontPickMonoOpen = false, false
				s.SettingsOpen = false
				s.saveErr = ""
			}
		})
	})
	if m.Dismissed() {
		// A popover's outside press lands here too; it closes both, and
		// the edits stand: they were applied the moment they were made.
		s.fontPickOpen, s.fontPickMonoOpen = false, false
		s.saveErr = ""
	}
}

// settingsRoom is the height the dialog spends outside its scrolling
// fields: the modal's margin and panel padding, plus this column's
// padding, title, button and gaps.
const settingsRoom = 150

// settingsMinFields keeps the scrolling fields usable in a window too
// short for the dialog's own chrome.
const settingsMinFields = 80

// groups renders the settings sections inside the dialog's scroll area:
// one card per section, rows separated by hairlines.
func (s *Shell) groups(c *ui.Context) {
	s.group(c, locales.SettingsSectionAppearance(), func() {
		s.settingRow(c, locales.SettingsTheme(), locales.SettingsThemeDetail(), func() {
			s.themeSegmented(c)
		})
		s.settingRow(c, locales.SettingsLayout(), "", func() {
			s.segmentedRow(c, "layout", &s.draftLayout, locales.SettingsLayout(),
				locales.LayoutCentered(), locales.LayoutFullWidth())
		})
		s.settingRow(c, locales.SettingsLanguage(), "", func() {
			s.languageSelect(c)
		})
	})

	s.group(c, locales.SettingsSectionEditor(), func() {
		s.settingRow(c, locales.SettingsContentFont(), "", func() {
			s.fontTrigger(c, locales.SettingsContentFont(), s.Saved.FontFamily, &s.fontPickOpen, fontContent)
			s.fontSize(c, locales.ContentFontSize(), &s.Saved.FontSize, 12, 24)
		})
		s.settingRow(c, locales.SettingsCodeFont(), "", func() {
			s.fontTrigger(c, locales.SettingsCodeFont(), s.Saved.MonoFontFamily, &s.fontPickMonoOpen, fontMono)
			s.fontSize(c, locales.CodeFontSize(), &s.Saved.MonoFontSize, 10, 18)
		})
		s.settingRow(c, locales.SettingsScrollSpeed(), "", func() {
			ui.NumberInput(c, &s.Saved.ScrollSpeed, 0.5, 3.0, 0.5).Label(locales.SettingsScrollSpeed())
		})
		s.settingRow(c, locales.SettingsInertiaScroll(), "", func() {
			s.switchRow(c, locales.SettingsInertiaScroll(), &s.Saved.InertiaScroll)
		})
	})

	s.group(c, locales.SettingsSectionFiles(), func() {
		s.settingRow(c, locales.SettingsExplorerRoot(), "", func() {
			s.segmentedRow(c, "explorer", &s.draftExplorer, locales.SettingsExplorerRoot(),
				locales.ExplorerCurrentDirectory(), locales.ExplorerProjectDirectory())
		})
		// The markers row exists only when the root is the project: with
		// the explorer rooted at the current directory there is no root to
		// mark. markersText keeps the text either way, so toggling the
		// segmented back and forth never loses what was typed.
		if s.Saved.ExplorerRootMode == "project_root" {
			s.settingRow(c, locales.SettingsProjectRootMarkers(), "", func() {
				s.markerInput(c)
			})
		}
	})

	s.group(c, locales.SettingsSectionSystem(), func() {
		s.settingRow(c, locales.SettingsAutoRefresh(), "", func() {
			s.switchRow(c, locales.SettingsAutoRefresh(), &s.Saved.AutoRefresh)
		})
		s.settingRow(c, locales.SettingsShowFPS(), "", func() {
			s.switchRow(c, locales.SettingsShowFPS(), &s.Saved.ShowFPS)
		})
	})
}

// persist writes the settings and applies them at once: theme, tree,
// watcher and fonts pick the change up when the frame rebuilds. A failing
// write keeps its message for the dialog's error line; the in-memory
// state still holds what the user chose.
func (s *Shell) persist() {
	if err := s.Apply(s.Saved); err != nil {
		s.saveErr = err.Error()
	} else {
		s.saveErr = ""
	}
}

// persistSoon debounces a persist while the user keeps editing: a size
// spinner or the markers field changes every keystroke, but the file
// needs to land once the editing settles. settlePersist, at the head of
// each frame, fires it when its time is up.
func (s *Shell) persistSoon(c *ui.Context) {
	s.persistTimer = c.Now().Add(settingsDebounce)
	c.After(settingsDebounce)
}

// settlePersist runs the debounced persist once its time is up. It sits
// at the head of the frame, where the width save already settles.
func (s *Shell) settlePersist(c *ui.Context) {
	if s.persistTimer.IsZero() {
		return
	}
	if now := c.Now(); now.Before(s.persistTimer) {
		c.After(s.persistTimer.Sub(now))
		return
	}
	s.persistTimer = time.Time{}
	s.persist()
}

// settingRow lays out one setting mojian's way: the label (13/500) over
// an optional detail line (11.5 muted) in a growing column, the control
// shrink-wrapped at the row's end, both centered. A hairline divides the
// rows but never leads the card.
func (s *Shell) settingRow(c *ui.Context, title, detail string, control func()) {
	s.rows++
	if s.rows > 1 {
		ui.Divider(c).Margin(0, settingsCardPadX)
	}
	t := c.Theme()
	ui.Row(c).MinHeight(52).Padding(0, settingsCardPadX).Gap(settingsRowGap).
		AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, title).FontSize(13).FontWeight(500).TextColor(t.Text)
			if detail != "" {
				ui.Text(c, detail).FontSize(11.5).TextColor(t.TextMuted).LineHeight(1.35)
			}
		})
		ui.Row(c).Shrink(0).Gap(10).AlignItems(ui.Center).Children(control)
	})
}

// group draws one section: a muted caption with mojian's letter spacing
// over one rounded card of rows.
func (s *Shell) group(c *ui.Context, title string, fields func()) {
	t := c.Theme()
	s.rows = 0
	ui.Column(c).Gap(8).FillWidth().Children(func() {
		ui.Row(c).MinHeight(24).Padding(2, 0).AlignItems(ui.Center).Children(func() {
			ui.Text(c, title).FontSize(11).FontWeight(650).TextColor(t.TextMuted).LetterSpacing(0.5)
		})
		ui.Column(c).FillWidth().Background(t.Surface).BorderWidth(1).BorderColor(t.Border).
			Radius(10).Shadow(0, 1, 3, 0, t.Border).Clip().Children(fields)
	})
}

// themeLabels name the three appearances in the spellings the setting
// stores, as the Rust build's dialog read them.
var themeLabels = []string{settings.ThemeAuto, settings.ThemeLight, settings.ThemeDark}

// themeSegmented is the theme's three-way segmented control (Auto, Light,
// Dark); a click applies the theme through persist at once.
func (s *Shell) themeSegmented(c *ui.Context) {
	choice := 0
	switch s.Saved.Theme {
	case settings.ThemeLight:
		choice = 1
	case settings.ThemeDark:
		choice = 2
	}
	seg := ui.SegmentedBase(c, &choice, len(themeLabels))
	s.styleSegmented(c, seg.Track, locales.SettingsTheme())
	seg.Track.Children(func() {
		for n, label := range themeLabels {
			c.Key("theme-" + label)
			s.segment(c, seg.Segment(n), n, choice, label)
		}
	})
	// OnChange commits the local the bound segments chose once the input
	// has updated it, which Changed in the same pass cannot promise.
	seg.Track.OnChange(func() {
		s.Saved.Theme = themeLabels[choice]
		s.persist()
	})
}

// segmentedRow is the two-way segmented control (layout, explorer root)
// mojian's choice rows use, reading and writing the index idx holds and
// mapping the choice through onSegmented.
func (s *Shell) segmentedRow(c *ui.Context, key string, idx *int, label string, choices ...string) {
	seg := ui.SegmentedBase(c.Key(key), idx, len(choices))
	s.styleSegmented(c, seg.Track, label)
	seg.Track.Children(func() {
		for n, text := range choices {
			c.Key(key + "-" + strconv.Itoa(n))
			s.segment(c, seg.Segment(n), n, *idx, text)
		}
	})
	seg.Track.OnChange(func() { s.onSegmented(key, idx) })
}

// styleSegmented dresses a segmented track as mojian's choice rows do:
// a hover-tinted pill with a hairline, its segments inset 3 DIPs.
func (s *Shell) styleSegmented(c *ui.Context, track ui.Element, label string) {
	track.Padding(3).Radius(8).Background(c.Theme().SurfaceHover).BorderWidth(1).
		BorderColor(c.Theme().Border).Gap(2).Label(label)
}

// segment dresses one choice mojian's way: the chosen one lifts onto the
// surface color, the rest tint on hover.
func (s *Shell) segment(c *ui.Context, b ui.Element, n, chosen int, label string) {
	t := c.Theme()
	b.Padding(4, 11).Radius(6).Cursor(ui.CursorPointer)
	switch {
	case n == chosen:
		b.Background(t.Surface)
	case b.Hovered():
		b.Background(t.SurfacePressed)
	}
	b.Children(func() {
		col := t.TextMuted
		if n == chosen {
			col = t.Text
		}
		ui.Text(c, label).FontSize(12).FontWeight(550).TextColor(col)
	})
}

// onSegmented maps the layout and explorer segmented's index back onto
// the saved settings and persists: both apply on the next frame.
func (s *Shell) onSegmented(key string, idx *int) {
	switch key {
	case "layout":
		s.Saved.Layout = layoutName(*idx)
	case "explorer":
		s.Saved.ExplorerRootMode = explorerName(*idx)
	}
	s.persist()
}

// switchRow is a switch writing its value straight into the saved
// settings, named so assistive technology and the tests reach the box.
func (s *Shell) switchRow(c *ui.Context, label string, value *bool) {
	on := *value
	sw := ui.Switch(c, &on).Label(label)
	if sw.Changed() {
		*value = on
		s.persist()
	}
}

// markerInput is the markers field, debounced like the font sizes: it
// writes the split list into the saved settings as the user types.
func (s *Shell) markerInput(c *ui.Context) {
	in := ui.TextInput(c, &s.markersText).Width(settingsMarkersW).Label(locales.SettingsProjectRootMarkers())
	if in.Changed() {
		s.Saved.ProjectRootMarkers = splitMarkers(s.markersText)
		s.persistSoon(c)
	}
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

// languageSelect renders the language drop-down. Like the dialog's other
// fields it applies at once: mygo persists the choice itself and applies
// it immediately, redrawing the whole window through OnLocaleChanged.
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
		BorderWidth(1).BorderColor(c.Theme().Border).Background(c.Theme().Surface)
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

// fontSize renders a font's size spinner, named so a reader tells the
// two font fields apart and the dialog's layout test finds its place.
// The spinner writes the saved size and debounces the persist.
func (s *Shell) fontSize(c *ui.Context, name string, value *float64, lo, hi float64) {
	ui.Box(c).Label(name).Children(func() {
		in := ui.NumberInput(c, value, lo, hi, 1)
		if in.Changed() {
			s.persistSoon(c)
		}
	})
}

// fontTrigger draws the font field's button: the saved family, or
// "System Default" where none is set, and the dropdown's chevron. It
// opens the searchable picker for that field; the picker's own row click
// applies the family at once.
func (s *Shell) fontTrigger(c *ui.Context, name, family string, open *bool, which int) {
	t := c.Theme()
	display := family
	if display == "" {
		display = locales.SystemDefault()
	}
	b := ui.ButtonBase(c.Key("font-"+name)).Label(locales.ChooseFont(name)).
		Height(30).MaxWidth(132).Justify(ui.SpaceBetween).AlignItems(ui.Center).
		Padding(6, 10).Radius(6).
		BorderWidth(1).BorderColor(t.Border).Background(t.Surface)
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
