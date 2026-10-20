package work

import (
	"fmt"
	"strings"

	"github.com/handrawanw/foundry/internal/core"
)

// existingUIBrief is the frontend an existing project already has, for the
// design prompt: new pages fit into it, not beside it.
func existingUIBrief(p *core.Project) string {
	if p.Structure == nil || p.Structure.UI == nil {
		return ""
	}
	ui := p.Structure.UI
	var b strings.Builder
	b.WriteString("EXISTING UI (product already has this frontend; new pages fit into it, not beside it):\n")
	if len(ui.Routes) > 0 {
		b.WriteString("- Existing routes: " + strings.Join(ui.Routes, ", ") + "\n")
	}
	if len(ui.Shell) > 0 {
		b.WriteString("- Layout shell and navigation files: " + strings.Join(ui.Shell, ", ") + "\n")
	}
	if ui.Navigation != "" {
		nav := map[string]string{"side": "a side menu", "top": "a top bar"}[ui.Navigation]
		if len(ui.NavItems) > 0 {
			nav += " with the items: " + strings.Join(ui.NavItems, ", ")
		}
		b.WriteString("- Navigation: " + nav + "\n")
	}
	if len(ui.Components) > 0 {
		b.WriteString("- Shared components: " + strings.Join(ui.Components, ", ") + "\n")
	}
	if t := ui.Tokens; t != nil {
		fmt.Fprintf(&b, "- Design tokens declared in %s (Foundry keeps these; add only missing ones):\n", strings.Join(ui.TokenSources, ", "))
		for _, g := range t.Groups() {
			for _, tok := range *g.Tokens {
				fmt.Fprintf(&b, "  - %s %s = %s\n", g.Name, tok.Name, tok.Value)
			}
		}
	}
	b.WriteString("rules.source is \"existing\": the existing design system and components are the rules; do not replace them. " +
		"Do not redesign or re-create existing pages; a page on an existing route is marked existing and only describes the change. " +
		"Specify only the pages the features need. New pages sit in the existing layout shell and navigation (mark that shell existing, " +
		"source = its file) and are added as items of the existing menu. Follow the existing route style; never give a new page an existing route.\n")
	return b.String()
}
