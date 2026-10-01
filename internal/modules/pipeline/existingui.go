package pipeline

import (
	"fmt"

	"github.com/handrawanw/foundry/internal/core"
	"github.com/handrawanw/foundry/internal/modules/uiscan"
	"github.com/handrawanw/foundry/internal/modules/work"
)

// readExistingUI reads the frontend of an existing project: routes, layout
// shell, navigation and shared components for the prompts, and its design
// tokens as the project theme "existing-ui". That theme becomes the
// design's theme unless the user already picked one.
func readExistingUI(p *core.Project) {
	s := p.Structure
	if s == nil || s.Mode != core.StructureExisting {
		return
	}
	ui := uiscan.Scan(p.Dir, s.Components)
	s.UI = ui
	if ui == nil {
		return
	}
	builtin, _ := work.Themes("")
	theme := "tidak ada token desain yang cukup; theme dipilih seperti biasa"
	if t, sources := uiscan.Theme(p.Dir, ui, builtin); t != nil {
		if err := work.SaveTheme(p.Dir, *t); err != nil {
			theme = "theme dari kode tidak bisa disimpan: " + err.Error()
		} else {
			ui.ThemeID, ui.ThemeSources = t.ID, sources
			theme = "theme " + t.ID
			if p.Theme == "" {
				p.Theme = t.ID
			}
		}
	}
	p.Log("system", fmt.Sprintf("UI yang ada dibaca: %d route, %d komponen, navigasi %q; %s",
		len(ui.Routes), len(ui.Components), ui.Navigation, theme))
}
