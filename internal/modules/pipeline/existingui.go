package pipeline

import (
	"fmt"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/uiscan"
)

// readExistingUI reads the frontend of an existing project: routes, layout
// shell, navigation, shared components and the design tokens its code
// declares. The design rules of the project follow them.
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
	tokens := "tidak ada token desain yang cukup"
	if t, sources := uiscan.Tokens(p.Dir, ui); t != nil {
		ui.Tokens, ui.TokenSources = t, sources
		tokens = "token desain dari " + strings.Join(sources, ", ")
	}
	p.Log("system", fmt.Sprintf("UI yang ada dibaca: %d route, %d komponen, navigasi %q; %s",
		len(ui.Routes), len(ui.Components), ui.Navigation, tokens))
}
