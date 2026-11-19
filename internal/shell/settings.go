package shell

import (
	"fmt"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/settings"
	"github.com/windedge/peekdown/locales"
)

// The settings dialog follows mojian's two-column measures: a rail of
// pages on the left, one section's card on the right, and the same
// two-line rows inside that card.
const (
	settingsWidth          = 900
	settingsMaxHeight      = 640
	settingsWindowMargin   = 48
	settingsPadding        = 20
	settingsGap            = 14
	settingsRailWidth      = 216
	settingsPaneGap        = 12
	settingsScrollPadStart = 24
	settingsScrollPadEnd   = 12
	settingsCardGap        = 12
	settingsCardRim        = 1
	settingsCardPadX       = 16
	settingsRowGap         = 16
	settingsDebounce       = 300 * time.Millisecond
	settingsMarkersW       = 200
)

// settingsThemeGridWidth is the space the Appearance grid has inside the
// page's card: the panel's content, less the rail, the column gaps, the
// scroller's insets, the card's rim and its horizontal padding.
func settingsThemeGridWidth(winW float32) float32 {
	return min(settingsWidth, winW-settingsWindowMargin) - 2*settingsPadding -
		settingsGap - settingsRailWidth - settingsPaneGap - settingsScrollPadStart -
		settingsScrollPadEnd - 2*settingsCardRim - 2*settingsCardPadX
}

// settingsNavItem is one page of the rail: its name, its page subtitle
// and the glyph beside the name.
type settingsNavItem struct {
	title  func() string
	detail func() string
	icon   *ui.SVG
}

var settingsNavItems = []settingsNavItem{
	{locales.SettingsSectionAppearance, locales.SettingsSectionAppearanceDetail, paletteIcon},
	{locales.SettingsSectionReading, locales.SettingsSectionReadingDetail, docIcon},
	{locales.SettingsSectionAbout, locales.SettingsSectionAboutDetail, gearIcon},
}

// settingsDialog renders the settings modal: every control applies at
// once through persist (no Save button), the Done button just closes.
func (s *Shell) settingsDialog(c *ui.Context) {
	winW, winH := c.Size()
	m := ui.DialogBase(c, &s.SettingsOpen, func(backdrop, panel ui.Element) {
		// The font popovers draw over the dialog: mygo stacks overlays in
		// build order, so a layer built inside it sits on top, and the
		// anchor needs the trigger's frame, which only exists here.
		t := c.Theme()
		k := s.accentColors(c)
		backdrop.Background(k.Backdrop).Label("Settings backdrop")
		panel.Width(min(settingsWidth, winW-settingsWindowMargin)).
			Height(min(settingsMaxHeight, winH-settingsWindowMargin)).
			Padding(settingsPadding).Gap(settingsGap).Radius(10).
			Background(k.CardFocused).Border(1, cardRim(t, true)).
			Shadow(0, 1, 3, 0, cardShadow(t, true)).
			Shadow(0, 6, 22, -2, cardShadow(t, true)).
			Clip().Label("Settings panel")
		ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
			ui.Text(c, locales.SettingsTitle()).FontSize(15).FontWeight(650).
				TextColor(t.Text).Grow(1).MinWidth(0)
			iconButton(c, "settings-close-icon", closeIcon, locales.SettingsCloseIcon(),
				24, 10, s.CloseSettings)
		})

		ui.Row(c).Grow(1).MinHeight(0).Gap(settingsPaneGap).AlignItems(ui.Stretch).Children(func() {
			s.settingsRail(c, t, k)
			s.settingsContent(c, t)
		})
		ui.Row(c).FillWidth().Justify(ui.End).AlignItems(ui.Center).Gap(10).Children(func() {
			if ui.Button(c, locales.SettingsClose()).Label("Settings Close").Clicked() {
				s.CloseSettings()
			}
			if ui.PrimaryButton(c, locales.SettingsDone()).Label("Settings Done").Clicked() {
				s.CloseSettings()
			}
		})
	})
	if m.Dismissed() {
		// A popover's outside press lands here too; it closes both, and
		// the edits stand: they were applied the moment they were made.
		s.CloseSettings()
	}
}

// settingsRail is the page list: a card of named items on the left, with
// the chosen page filled by PanelSel and accent-colored, and a hovered
// page tinted like mygo's sidebar rows.
func (s *Shell) settingsRail(c *ui.Context, t *ui.Theme, k AccentColors) {
	ui.Column(c).Width(settingsRailWidth).Shrink(0).FillHeight().
		Background(k.Card).Border(1, cardRim(t, false)).Radius(10).
		Shadow(0, 8, 24, 0, cardShadow(t, false)).Clip().
		Label("Settings rail").Children(func() {
		ui.Column(c).FillWidth().Padding(6).Gap(2).Children(func() {
			for i, item := range settingsNavItems {
				c.Key("settings-nav-" + strconv.Itoa(i))
				chosen := i == s.settingsSection
				nav := ui.Row(c).FillWidth().MinHeight(36).Padding(6, 8).Gap(8).
					Radius(10).AlignItems(ui.Center).Cursor(ui.CursorPointer).
					Focusable().Role(ui.RoleTab).AccessSelected(chosen).
					Label("Settings nav " + item.title())
				col := t.TextMuted
				if chosen {
					col = k.Accent
					nav.Background(k.PanelSel)
				} else if nav.Hovered() {
					nav.Background(k.SurfaceHover)
				}
				nav.Children(func() {
					ui.Icon(c, item.icon).Size(16, 16).TextColor(col)
					ui.Text(c, item.title()).FontSize(14).FontWeight(550).TextColor(col).Grow(1).MinWidth(0)
				})
				if nav.Clicked() && s.settingsSection != i {
					s.settingsSection = i
					s.settingsScroll.Y = 0
				}
			}
		})
	})
}

// settingsContent scrolls only the chosen page. Its ScrollState lets the
// rail put the page back at the top when the choice changes.
func (s *Shell) settingsContent(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Grow(1).MinWidth(0).MinHeight(0).Children(func() {
		ui.Scroll(c).Label("Settings fields").Grow(1).MinHeight(0).
			TrackScroll(&s.settingsScroll).Children(func() {
			ui.Column(c).FillWidth().Padding(2, 0, 2, settingsScrollPadStart).
				Gap(settingsGap).PaddingEnd(settingsScrollPadEnd).
				Children(func() {
					item := settingsNavItems[s.settingsSection]
					ui.Column(c).Gap(3).Children(func() {
						ui.Text(c, item.title()).Label("Settings header").
							Role(ui.RoleHeading).FontSize(20).FontWeight(650).TextColor(t.Text)
						if detail := item.detail(); detail != "" {
							ui.Text(c, detail).FontSize(12.5).TextColor(t.TextMuted).LineHeight(1.35)
						}
					})
					if s.saveErr != "" {
						ui.Text(c, s.saveErr).FontSize(12).TextColor(t.Danger)
					}
					s.currentGroup(c)
				})
		})
	})
}

// currentGroup draws the card for the chosen page alone; switching pages
// rebuilds the scroll's children, so no other page's rows stay behind.
func (s *Shell) currentGroup(c *ui.Context) {
	switch s.settingsSection {
	case 1:
		s.readingGroup(c)
	case 2:
		s.aboutGroup(c)
	default:
		s.appearanceGroup(c)
	}
}

func (s *Shell) appearanceGroup(c *ui.Context) {
	s.group(c, locales.SettingsSectionAppearance(), func() {
		s.settingRow(c, locales.SettingsTheme(), locales.SettingsThemeDetail(), func() {
			s.themeSegmented(c)
		})
		s.themeColorBlock(c)
		s.settingRow(c, locales.SettingsLayout(), "", func() {
			s.segmentedRow(c, "layout", &s.draftLayout, locales.SettingsLayout(),
				locales.LayoutCentered(), locales.LayoutFullWidth())
		})
		s.settingRow(c, locales.SettingsLanguage(), "", func() {
			s.languageSelect(c)
		})
		s.settingRow(c, locales.SettingsShowFPS(), "", func() {
			s.switchRow(c, locales.SettingsShowFPS(), &s.Saved.ShowFPS)
		})
	})
}

// themeColorBlock embeds the card grid in the Appearance page. It draws
// its own rules, so the grid is not counted as a setting row.
func (s *Shell) themeColorBlock(c *ui.Context) {
	ui.Divider(c).Margin(0, settingsCardPadX)
	winW, _ := c.Size()
	ui.Column(c).FillWidth().Padding(0, settingsCardPadX, settingsCardPadX).
		Label("Theme Color section").Children(func() {
		paletteHeading(c, locales.SettingsThemeColor(), 11)
		s.themeGrid(c, settingsThemeGridWidth(winW))
	})
	s.rows = 1
}

func (s *Shell) readingGroup(c *ui.Context) {
	s.group(c, locales.SettingsSectionReading(), func() {
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
		s.settingRow(c, locales.SettingsAutoRefresh(), "", func() {
			s.switchRow(c, locales.SettingsAutoRefresh(), &s.Saved.AutoRefresh)
		})
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
}

const (
	aboutIssuesURL    = repoURL + "/issues"
	aboutChangelogURL = repoURL + "/blob/main/CHANGELOG.md"
)

func (s *Shell) aboutGroup(c *ui.Context) {
	// Keep the long URLs from squeezing the labels in a narrow window.
	winW, _ := c.Size()
	panelW := min(settingsWidth, winW-settingsWindowMargin)
	rowW := panelW - 2*settingsPadding - settingsGap - settingsRailWidth -
		settingsPaneGap - 2*settingsCardRim - settingsScrollPadStart -
		settingsScrollPadEnd - 2*settingsCardPadX - settingsRowGap
	linkW := max(180, min(380, rowW-110))
	s.group(c, locales.SettingsSectionAbout(), func() {
		s.settingRow(c, locales.AppTitle(), locales.SettingsAboutIntro(), func() {
			version := s.appVersion()
			if version != "" {
				version = "v" + version
			}
			ui.Text(c, version).FontSize(14).FontWeight(600).TextColor(c.Theme().Text)
		})
		s.aboutLinkRow(c, locales.SettingsAboutRepository(), repoURL, linkW)
		s.aboutLinkRow(c, locales.SettingsAboutIssues(), aboutIssuesURL, linkW)
		s.aboutLinkRow(c, locales.SettingsAboutChangelog(), aboutChangelogURL, linkW)
		s.settingRow(c, locales.SettingsAboutLicense(), "", func() {
			ui.Text(c, "MIT").FontSize(13).TextColor(c.Theme().Text)
		})
		s.settingRow(c, locales.SettingsAboutRuntime(), "", func() {
			ui.Text(c, fmt.Sprintf("Go %s / mygo %s", runtime.Version(), mygo.Version)).
				FontSize(12).TextColor(c.Theme().TextMuted)
		})
	})
}

func (s *Shell) aboutLinkRow(c *ui.Context, label, url string, width float32) {
	s.settingRow(c, label, "", func() {
		link := ui.Row(c).Width(width).Shrink(0).
			Label("About link " + label).Role(ui.RoleLink).Focusable().
			Cursor(ui.CursorPointer).AlignItems(ui.Center)
		link.Children(func() {
			ui.Text(c, url).FontSize(12).TextColor(c.Theme().Accent).
				SingleLine().Ellipsis("...").Grow(1).MinWidth(0)
		})
		if link.Clicked() {
			s.OpenLink(url)
		}
	})
}

// appVersion reads the version mygo injected at build time; the hook lets
// headless tests use the manifest's version when no build info exists.
func (s *Shell) appVersion() string {
	if s.AppVersion != nil {
		return s.AppVersion()
	}
	return mygo.App.Version()
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
			ui.Text(c, title).FontSize(13).FontWeight(500).TextColor(t.Text).
				SingleLine().Ellipsis("...")
			if detail != "" {
				ui.Text(c, detail).FontSize(11.5).TextColor(t.TextMuted).LineHeight(1.35)
			}
		})
		ui.Row(c).Shrink(0).Gap(10).AlignItems(ui.Center).Children(control)
	})
}

// group draws one section as a rounded card of rows. The page header
// above the card names the section, so the card needs no second caption.
func (s *Shell) group(c *ui.Context, title string, fields func()) {
	t := c.Theme()
	k := s.accentColors(c)
	s.rows = 0
	ui.Column(c).FillWidth().Background(k.Card).Border(1, cardRim(t, false)).
		Radius(10).Shadow(0, 1, 3, 0, cardShadow(t, false)).Clip().
		Label(title + " card").Children(fields)
}

// themeLabels name the three appearances in the spellings the setting
// stores, as the Rust build's dialog read them.
var themeLabels = []string{settings.ThemeAuto, settings.ThemeLight, settings.ThemeDark}

// themeChoiceLabels are the three appearances as the reader sees them.
// The spellings themeLabels keeps are stored, never shown.
func themeChoiceLabels() []string {
	return []string{locales.AppearanceSystem(), locales.AppearanceLight(), locales.AppearanceDark()}
}

// themeSegmented is the theme's three-way segmented control; a click
// applies the theme through persist at once.
func (s *Shell) themeSegmented(c *ui.Context) {
	choice := 0
	switch s.Saved.Theme {
	case settings.ThemeLight:
		choice = 1
	case settings.ThemeDark:
		choice = 2
	}
	labels := themeChoiceLabels()
	seg := ui.SegmentedBase(c, &choice, len(themeLabels))
	s.styleSegmented(c, seg.Track, locales.SettingsTheme())
	seg.Track.Children(func() {
		for n, label := range themeLabels {
			c.Key("theme-" + label)
			s.segment(c, seg.Segment(n), n, choice, labels[n])
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
