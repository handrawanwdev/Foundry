package work

import (
	"fmt"
	"strings"

	"github.com/handrawanw/foundry/internal/core"
)

// existingUIBrief is the frontend an existing project already has, for the
// design, plan and task prompts: new pages fit into it, not beside it.
// forCode adds what the code must reuse.
func existingUIBrief(p *core.Project, forCode bool) string {
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
	if ui.ThemeID != "" {
		fmt.Fprintf(&b, "- Design tokens (theme %q) read from: %s\n", ui.ThemeID, strings.Join(ui.ThemeSources, ", "))
	}
	b.WriteString("Do not redesign or re-create existing pages; design only pages the features need. " +
		"New pages sit in the same layout shell and navigation: add them as items of the existing menu. " +
		"Follow existing route style; never give a new page an existing route.\n")
	if forCode {
		b.WriteString("Build new pages inside the existing layout shell, register their routes in the existing router and their menu items in the existing navigation, " +
			"and reuse the shared components above before creating new ones.\n")
	}
	return b.String()
}
