package shell

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

// settingsDialog renders the modal settings form bound to s.Draft.
//
// Hidden from this build: Scroll Speed and Smooth Scrolling, whose mygo
// APIs do not exist yet (the shell keeps the settings, the UI omits
// them), and an FPS switch, whose frame-rate readout this build has no
// counterpart for.
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
			ui.Text(c, "Settings").FontSize(18).Bold()

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
				if ui.Button(c, "Cancel").Clicked() {
					s.fontPickOpen = false
					s.fontPickMonoOpen = false
					s.SettingsOpen = false
					s.saveErr = ""
					s.Draft = s.Saved
				}
				if ui.PrimaryButton(c, "Save").Clicked() {
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
	s.group(c, "Appearance", func() {
		s.field(c, "Theme", func() {
			ui.Select(c, &s.Draft.Theme, []string{"auto", "light", "dark"})
		})
		s.field(c, "Layout", func() {
			s.draftLayout = layoutIndex(s.Draft.Layout)
			ui.Segmented(c, &s.draftLayout, "Centered", "Full Width").Label("Layout")
			s.Draft.Layout = layoutName(s.draftLayout)
		})
	})

	s.group(c, "Editor", func() {
		s.field(c, "Content Font", func() {
			s.fontTrigger(c, "Content Font", s.Draft.FontFamily, &s.fontPickOpen, fontContent)
			s.fontSize(c, "Content Font size", &s.Draft.FontSize, 12, 24)
		})
		s.field(c, "Code Font", func() {
			s.fontTrigger(c, "Code Font", s.Draft.MonoFontFamily, &s.fontPickMonoOpen, fontMono)
			s.fontSize(c, "Code Font size", &s.Draft.MonoFontSize, 10, 18)
		})
	})

	s.group(c, "Files", func() {
		s.field(c, "Explorer Root", func() {
			s.draftExplorer = explorerIndex(s.Draft.ExplorerRootMode)
			ui.Segmented(c, &s.draftExplorer, "Current Directory", "Project Directory").Label("Explorer Root")
			s.Draft.ExplorerRootMode = explorerName(s.draftExplorer)
		})
		s.field(c, "Project Root Markers", func() {
			// Constrained like the font dropdown above: left free the input
			// fills the row and its text runs under the fields' scroll bar.
			// Label names it, here and for assistive technology.
			ui.TextInput(c, &s.markersText).Grow(1).MaxWidth(292).Label("Project Root Markers")
		})
	})

	s.group(c, "System", func() {
		// mygo's Switch draws no text of its own, so the row carries the
		// visible label; the switch keeps the same name for assistive
		// technology.
		s.field(c, "Auto Refresh", func() {
			ui.Switch(c, &s.Draft.AutoRefresh).Label("Auto Refresh")
		})
	})
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
		display = "System Default"
	}
	b := ui.ButtonBase(c.Key("font-"+name)).Label("Choose "+name).
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
