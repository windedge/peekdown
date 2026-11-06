package shell

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

// settingsDialog renders the modal settings form bound to s.Draft.
func (s *Shell) settingsDialog(c *ui.Context) {
	m := ui.Modal(c, &s.SettingsOpen, func() {
		ui.Column(c).Width(480).Padding(16).Gap(10).Children(func() {
			ui.Text(c, "Settings").FontSize(18).Bold()

			ui.Text(c, "Theme")
			ui.Select(c, &s.Draft.Theme, []string{"auto", "light", "dark"})

			ui.Text(c, "Layout")
			s.draftLayout = layoutIndex(s.Draft.Layout)
			ui.Segmented(c, &s.draftLayout, "Centered", "Full Width")
			s.Draft.Layout = layoutName(s.draftLayout)

			ui.Text(c, "Explorer Root")
			s.draftExplorer = explorerIndex(s.Draft.ExplorerRootMode)
			ui.Segmented(c, &s.draftExplorer, "Current Dir", "Project Root")
			s.Draft.ExplorerRootMode = explorerName(s.draftExplorer)

			ui.Text(c, "Project Root Markers")
			if !s.markersLoaded {
				s.markersText = strings.Join(s.Draft.ProjectRootMarkers, ", ")
				s.markersLoaded = true
			}
			ui.TextInput(c, &s.markersText)

			ui.Text(c, "Scroll Speed")
			ui.Slider(c, &s.Draft.ScrollSpeed, 0.5, 3.0)

			ui.Switch(c, &s.Draft.InertiaScroll)
			ui.Text(c, "Smooth Scrolling")

			ui.Text(c, "Content Font")
			ui.NumberInput(c, &s.Draft.FontSize, 12, 24, 1)

			ui.Text(c, "Code Font")
			ui.NumberInput(c, &s.Draft.MonoFontSize, 10, 18, 1)

			ui.Row(c).Gap(8).Children(func() {
				ui.Box(c).Grow(1)
				if ui.Button(c, "Cancel").Clicked() {
					s.SettingsOpen = false
					s.Draft = s.Saved
				}
				if ui.PrimaryButton(c, "Save").Clicked() {
					draft := s.Draft
					draft.ProjectRootMarkers = splitMarkers(s.markersText)
					_ = s.Apply(draft)
					s.SettingsOpen = false
				}
			})
		})
	})
	if m.Dismissed() {
		s.Draft = s.Saved
		s.markersLoaded = false
	}
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
