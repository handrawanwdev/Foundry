package work

import (
	"fmt"
	"slices"
	"strings"

	"github.com/handrawanw/foundry/internal/core"
)

const patchFormat = `OUTPUT:
Patch operations only. Do not return the full page.
Reply with ONLY one JSON object in a ` + "```json" + ` block, shape:
{"patches": [{"page": "PAGE-001", "operations": [
  {"op": "move", "target": 3, "after": 2},
  {"op": "remove", "target": 4},
  {"op": "update", "target": 0, "component": {the whole new component}},
  {"op": "add", "after": 1, "component": {"type": "button", "label": "", "target": "PAGE-003"}},
  {"op": "update", "page_fields": {"kind": "list"}}
]}],
 "pages": [], "removed": [], "flows": []}
- "target" and "after" are the [n] indexes of CURRENT PAGE, before the patch; earlier operations never shift them. "after": -1 = top, no "after" = bottom.
- One patch per page, with all its operations.
- "update" with "page_fields" (no target) only where PRESERVE allows page fields. Allowed page fields: name, role, purpose, hierarchy, secondary_actions, responsive, kind, density, route, access, primary_action, data, states, sub_feature_ids.
- "pages": only brand-new pages (id "NEW-1", "NEW-2", ...) in full. Never send an existing page.
- "removed": ids of pages to delete; "flows": only flows whose page list changes, with the full new list.
- "system": only when the request changes the design system and it is listed above as changeable; then the whole new system.
Foundry applies the patch and compares before/after. Any change outside the targets, or a target outside the scope, rejects the whole patch.`

// revisionPrompt asks for a patch: per requested page the current
// components with their [n], what the user sees, the target, the change and
// what must be preserved.
func revisionPrompt(p *core.Project) string {
	d := p.Design
	scope := d.Scope
	fb := d.Feedback
	var b strings.Builder
	b.WriteString(`You are the product designer of Foundry. The user asks for a change to the current wireframe design.
Turn the request into specific patch operations. Change only what is asked; do not improve, tidy or restyle anything else.
Write labels in the same language as the design. Do not write code. Do not modify files.

`)
	if d.Rejected != "" {
		b.WriteString("YOUR PREVIOUS PATCH WAS REJECTED, send a corrected patch:\n" + d.Rejected + "\n\n")
	}
	if len(fb) > 1 {
		b.WriteString("EARLIER REQUESTS (already applied, do not redo):\n- " + strings.Join(fb[:len(fb)-1], "\n- ") + "\n\n")
	}
	b.WriteString("PAGES (for targets and navigation):\n")
	for _, pg := range d.Pages {
		fmt.Fprintf(&b, "- %s %q %s %s %s\n", pg.ID, pg.Name, pg.Platform, pg.Kind, pg.Route)
	}
	b.WriteString("\n")

	open := scope == nil || scope.Open
	for _, pg := range d.Pages {
		sp := scope.Page(pg.ID)
		if sp == nil && !open {
			continue
		}
		writePage(&b, pg, sp, d.Screens[pg.ID])
		switch {
		case sp != nil && len(sp.Components) > 0:
			b.WriteString("TARGET:\n")
			for _, i := range sp.Components {
				if i >= 0 && i < len(pg.Components) {
					c := pg.Components[i]
					fmt.Fprintf(&b, "[%d] %s %q %s\n", i, c.Type, c.Label, mustJSON(c))
				}
			}
		case sp != nil:
			b.WriteString("TARGET:\nthe components of this page the change needs\n")
		}
		if sp != nil && sp.Request != "" {
			b.WriteString("CHANGE:\n" + sp.Request + "\n")
		}
		b.WriteString("PRESERVE:\n" + preserve(pg, scope, sp) + "\n\n")
	}
	switch {
	case scope != nil && scope.General != "":
		b.WriteString("CHANGE (any page above):\n" + scope.General + "\n\n")
	case scope == nil:
		b.WriteString("CHANGE:\n" + fb[len(fb)-1] + "\n\n")
	}

	b.WriteString(componentRules + renderRules + "\n")
	if open {
		// A general request may add pages or fill page contracts.
		b.WriteString("DESIGN RULES (only for new pages and for components or fields you add or update):\n" + designRules + "\n")
		b.WriteString("FEATURES:\n" + mustJSON(p.Features) + "\n\nFLOWS:\n" + mustJSON(p.Flows) + "\n\n")
		b.WriteString("A new page has this shape:\n" + pageShape + "\n\n")
		b.WriteString("DESIGN SYSTEM (changeable, same shape):\n" + mustJSON(d.System) + "\n\n")
	} else if d.System != nil {
		b.WriteString("DESIGN SYSTEM (use its tokens and component rules; not changeable in this request):\n" + mustJSON(d.System) + "\n\n")
	}
	b.WriteString(patchFormat)
	return b.String()
}

func writePage(b *strings.Builder, pg core.Page, sp *core.ScopePage, screen string) {
	seen := ""
	if sp != nil && sp.Device != "" {
		seen = ", user sees it on " + sp.Device
	}
	fmt.Fprintf(b, "CURRENT PAGE:\n%s %q (%s, kind %s%s)\n", pg.ID, pg.Name, pg.Platform, pg.Kind, seen)
	b.WriteString("COMPONENTS:\n")
	for i, c := range pg.Components {
		fmt.Fprintf(b, "[%d] %s\n", i, mustJSON(c))
	}
	if len(pg.Components) == 0 {
		b.WriteString("(none)\n")
	}
	fields := pg
	fields.Components = nil
	b.WriteString("PAGE FIELDS:\n" + mustJSON(fields) + "\n")
	if s := strings.TrimSpace(screen); s != "" {
		b.WriteString("WHAT THE USER SEES (read from the rendered wireframe; [n] type is components[n], (otomatis) is drawn by the wireframe, not a component):\n" + s + "\n")
	}
}

func preserve(pg core.Page, scope *core.ChangeScope, sp *core.ScopePage) string {
	var keep []string
	if sp != nil && len(sp.Components) > 0 {
		var others []string
		for i := range pg.Components {
			if !slices.Contains(sp.Components, i) {
				others = append(others, fmt.Sprintf("[%d]", i))
			}
		}
		if len(others) > 0 {
			keep = append(keep, "components "+strings.Join(others, " ")+" (content and order); you may add new components")
		}
	} else {
		keep = append(keep, "every component the change does not need, with its label, content and order")
	}
	if scope == nil || scope.Fields || sp != nil && sp.Fields {
		keep = append(keep, "page fields the change does not name")
	} else {
		keep = append(keep, "all page fields: kind, route, access, data, states, primary_action, density, name")
	}
	keep = append(keep, "automatic elements (top bar, brand, back link, form buttons) — change them only through the component that produces them")
	return "Everything else exactly as-is:\n- " + strings.Join(keep, "\n- ")
}
