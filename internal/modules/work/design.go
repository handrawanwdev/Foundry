package work

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/handrawanwdev/foundry/internal/core"
)

// The UI design is text and structured data only: design rules and a
// layout specification. The AI writes and revises them; Foundry validates
// every answer and refuses an invalid one as a whole.

// ErrNoChange means the AI found nothing to change for the request.
var ErrNoChange = errors.New("AI tidak menemukan perubahan rancangan UI untuk permintaan ini; tulis lebih spesifik")

type designOut struct {
	Rules        *core.DesignRules   `json:"rules"`
	Shells       []core.LayoutShell  `json:"shells"`
	Pages        []core.LayoutPage   `json:"pages"`
	RemoveShells []string            `json:"remove_shells"`
	RemovePages  []string            `json:"remove_pages"`
	Flows        map[string][]string `json:"flows"`
	// The UX layer: absent on a revision keeps the current value; an empty
	// array clears it.
	Users       *[]core.UIUser         `json:"users"`
	Goals       *[]core.UIGoal         `json:"goals"`
	Navigation  *[]core.NavGroup       `json:"navigation"`
	Decisions   *[]core.DesignDecision `json:"decisions"`
	Assumptions *[]string              `json:"assumptions"`
	Unknowns    *[]string              `json:"unknowns"`
}

// hasExperience reports whether an answer carries any UX-layer value.
func (o designOut) hasExperience() bool {
	return o.Users != nil || o.Goals != nil || o.Navigation != nil || o.Decisions != nil ||
		o.Assumptions != nil || o.Unknowns != nil
}

const designShape = `{
 "users": [{"role": "Operator", "platform": "web", "context": "kerja harian", "tasks": ["saring antrean"]}],
 "goals": [{"id": "GOAL-1", "role": "Operator", "text": "Saring antrean", "success": "Janji diproses", "priority": "must"}],
 "navigation": [{"name": "Operasional", "items": [{"label": "Antrean", "page": "antrean", "primary": true}]}],
 "decisions": [{"subject": "antrean", "choice": "table", "why": ["bandingkan banyak baris"], "basis": ["GOAL-1"], "source": "fact"}],
 "rules": {"source": "new", "reference": "", "direction": "", "principles": [""], "ux_guidelines": [""],
   "tokens": {"colors": [{"name": "action.primary", "value": "#1f6feb", "use": "aksi"}], "typography": [{"name": "type.body", "value": "Inter 14px/20px 400"}],
              "spacing": [{"name": "space.md", "value": "16px"}]},
   "components": [{"name": "Button", "use": "aksi", "variants": ["primary"], "rules": ["disabled saat kirim"]}],
   "accessibility": [""], "responsive": [""]},
 "shells": [{"id": "app", "name": "App", "platform": "web", "pattern": "sidebar",
   "regions": [{"id": "nav", "name": "Nav", "content": ["Menu"]}]}],
 "pages": [{"id": "antrean", "name": "Antrean", "platform": "web", "route": "/antrean", "access": "auth", "feature_id": "FEAT-001",
   "purpose": "Saring antrean", "goal": "GOAL-1", "shell": "app", "pattern": "table",
   "fields": [{"field": "Appointment.status", "label": "Status", "presentation": "badge"}],
   "sections": [{"id": "tabel", "name": "Tabel", "content": ["No, Pasien, Status, Aksi"], "components": ["Button"]}],
   "states": {"loading": "", "empty": "", "error": ""}}],
 "flows": {"FLOW-001": ["antrean"]}
}`

const designRules = `DESIGN RULES (rules)
- source: "new", or "existing" when the product already has a frontend: its tokens and components ARE the design system; keep them and add only what is missing.
- reference: when the DESIGN REFERENCE section names a template or product, put it in reference (the user's words). The rules are derived from it, not a restatement of it: never copy its logo, brand identity, proprietary assets or exact screens; adapt its visual characteristics to THIS product's tasks and data. When no reference is given, leave reference empty.
- principles: 3-7 principles for THIS product and its users, each one actionable and checkable ("operators compare many rows: dense tables, not cards"). Never generic ("modern and clean").
- ux_guidelines: concrete rules for navigation, forms and validation, feedback, errors, empty states, confirmation of destructive actions.
- tokens: colors, typography and spacing are required; radius, shadow, breakpoints when used. Semantic names (action.primary, text.secondary, surface.page, status.error, type.body, space.md, radius.md, breakpoint.tablet), concrete values (hex/rgb, px/rem, font family size/line-height weight), use = when to use it.
- components: every shared component the pages reuse: name, use, variants, rules (including hover, focus, active and disabled states). An existing component keeps its name and gets its file in source.
- visual_hierarchy: how importance is shown (size, weight, color, position). accessibility: WCAG 2.2 AA (contrast, focus, keyboard, labels, touch targets). responsive: the breakpoints and what changes at each. interaction: feedback, loading behavior, motion.
- platforms: rules that apply to one platform only (web, android, ios, desktop), e.g. Material 3 on android, Human Interface Guidelines on ios.
`

const uxRules = `UX INTELLIGENCE (users, goals, navigation, decisions)
Work like a senior product designer: understand the users and their tasks BEFORE choosing a layout. A table, entity or field name is data, never a layout instruction.
1. Users: from the roles and the story, name each kind of user, the platform they use, the context (frequency, pressure, device, environment) and the tasks they must complete.
2. Goals: one measurable goal per important task (id GOAL-1, ...), the role it belongs to, and what success looks like. Goals come from the use cases, not from the data model.
3. Information architecture: group destinations by the user's job, not by database tables. Label them with the user's words. A primary destination belongs to the menu; a detail page is reached from its list. Keep the top level small.
4. Page inventory: one page per user task or decision point, never per database entity. Reuse a page when the task is the same; split when the user's goal, permission or data scope differs. Typical 4-12 pages.
5. Patterns: choose by task, data volume and platform, not by habit.
   table: compare many records with several attributes; list: scan with one dominant attribute; cards: small set with visual identity; detail: one record; form: create or edit; wizard: long/dependent sequence; dashboard: only when KPIs are a real task; search: large/unbounded set; modal/drawer: short side task; tabs: facets of one record; settings: configuration; master-detail: rapid switching.
   On a small screen prefer list/cards over wide tables. Do not make every page a dashboard or a card grid.
6. Density: operators who repeat a task need compact rows and few clicks; occasional users need more guidance; touch users need larger targets. State the choice.
7. Every page must have at least one decision whose subject is the page id itself, stating its pattern and why; a form page also names its structure, input validation, submit state and error feedback. No page is left without a decision. One decision per page is enough unless it is a form. why: one concrete sentence. basis: 1-2 ids from the inputs. source: fact|assumption|constraint. Do not write a decision that cannot name its basis.
8. Facts, assumptions and unknowns are separate. Never invent volume, permissions, compliance or integrations that the blueprint does not state; record the gap as an assumption or unknown.
9. Data-to-UI mapping: for each page with records, map only the fields shown (table-column, list-item, card-title, card-meta, card-value, delta, detail, form-input, filter, summary, chart, badge, row-action, hidden). Do not show every field of an entity. card-value is a prominent value; delta is a change indicator; row-action is a per-row action (note says what it does). Record identity (symbol, name) is card-title.
10. States, validation, permissions and feedback are part of the page design: loading, empty, no_results, error, success, disabled, permission_denied, and what a user without the permission sees.
11. For a CLI/terminal platform (no screen layout), put its UX in rules.platforms with platform "cli": command and subcommand grammar, flags and defaults, help and examples, output shape and columns, error messages and exit codes, confirmation for destructive actions, and non-interactive/pipe behavior. Do not create layout pages for the terminal.
`

const layoutRules = `LAYOUT SPECIFICATION (shells, pages)
- Text and structured data only. No images, mockups, HTML, CSS or code.
- shells: the layouts pages share (app frame with sidebar or navbar, auth frame, ...): id, name, platform, pattern, regions (sections), navigation, grid, responsive.
- pages: every screen the features need, per platform with screens — one page per user task, not per entity. route: web starts with /, mobile/desktop is the screen name. access: public or auth. shell: a shell id, or empty for a page with its own frame.
- pages[].goal names the goal id the page serves; pages[].fields maps data fields to their presentation.
- pattern (page, shell and optional for a section) is one of: %s.
- sections: in visual order (top to bottom, start to end). content lists the elements in order, concretely ("Tabel antrean: No, Pasien, Dokter, Status, Aksi"). components are names from rules.components. grid, alignment, spacing, sizing and responsive use the tokens (space.md, breakpoint.tablet), never arbitrary values.
- hierarchy: section ids from the most important.
- states (page or section): keys %s; the value says what the user sees and can do. Required per page pattern: %s.
- links: {to: page id, via: what the user activates}; together they are the navigation between pages.
- navigation groups pages by the user's job; every page is reachable from it or from a link.
- flows: for each flow id, the page ids it passes through, in order.
- ids: lowercase letters, digits and dashes. Page ids are unique; section ids are unique within their page.
`

const outputDiscipline = `OUTPUT DISCIPLINE
Emit compact JSON. Omit empty keys, empty arrays and empty strings. Do not copy placeholder empty values from the shape.
- principles 3-5 short clauses; ux_guidelines 4-6; accessibility, responsive, interaction 2-4 each. Do not repeat the same sentence across those lists.
- tokens: 6-10 colors, 3-5 typography, 4-6 spacing, 2-3 radius, 1-2 shadow, 2-3 breakpoints. Short use.
- components: only those pages reuse (typically 4-10). One short variants list and one short rules line each.
- pages: one per user task. Typical 4-12. sections 2-4; content 1-3 concrete items; fields only those shown.
- decisions: exactly one per page (pattern + why). Form pages may add one for validation/submit. why: one short sentence. basis: 1-2 existing ids.
- states: one short sentence per required key only.
- Reply with JSON only. No preamble.
`

// DesignPrompt asks for the design rules and layout spec, or for a
// revision of them (the pending request).
func DesignPrompt(p *core.Project) string {
	var b strings.Builder
	b.WriteString(`You are the UI/UX designer of Foundry. Foundry designs software and writes instructions for an EXTERNAL coding agent; it never implements, renders or draws. Write the UI design as text and structured data: design rules and a layout specification. Do not write code. Do not modify files.
Write every human-readable text in the same language as the story.

PROJECT: ` + p.Name + "\nSTORY:\n" + p.Story + "\n\n")
	writeBinding(&b, p)
	b.WriteString("PLATFORMS WITH SCREENS: " + strings.Join(p.UIPlatforms(), ", ") + "\n")
	if ux := p.UXPlatforms(); len(ux) > 0 {
		b.WriteString("ALL PLATFORMS FOR UX RULES: " + strings.Join(ux, ", ") + "\n")
	}
	if len(p.Features) > 0 {
		b.WriteString("FEATURES (pages.feature_id):\n")
		for _, f := range p.Features {
			fmt.Fprintf(&b, "- %s %s [%s]\n", f.ID, f.Name, strings.Join(f.Platforms, ", "))
		}
	}
	if brief := p.Stack.Brief(); brief != "" {
		b.WriteString("\n" + brief)
	}
	if brief := uxBrief(p); brief != "" {
		b.WriteString("\n" + brief)
	}
	if brief := existingUIBrief(p); brief != "" {
		b.WriteString("\n" + brief)
	}
	b.WriteString("\n")
	d := p.UI
	var rejected string
	if d != nil && d.Request != nil {
		rejected = d.Request.Rejected
	}
	b.WriteString(uxRules + "\n")

	// The design language is seeded from the UI template (ui-template-ai-friendly):
	// one framework-agnostic guidance for every product; the existing design
	// system still wins.
	b.WriteString(designTemplate + "\n")

	// A user-provided design reference (Duolingo, YouTube, a custom template)
	// becomes concrete rules, adapted to this product, never cloned.
	if ref := designReference(p); ref != "" {
		b.WriteString(ref + "\n")
	}

	// Add learned patterns from previous projects
	if learnedRules := GetLearnedDesignRules(p, ""); learnedRules != "" {
		b.WriteString(learnedRules + "\n")
	}

	// Add learned recommendations from project history
	if recommendations := GetLearnedRecommendations(p, ""); recommendations != "" {
		b.WriteString(recommendations + "\n")
	}

	b.WriteString(designRules + "\n")
	fmt.Fprintf(&b, layoutRules+"\n", strings.Join(core.LayoutPatterns, ", "), strings.Join(core.UIStates, ", "), requiredStates())
	b.WriteString(outputDiscipline + "\n")
	if !d.Written() {
		if rejected != "" {
			b.WriteString("YOUR LAST ANSWER WAS REFUSED, fix this: " + rejected + "\n\n")
		}
		b.WriteString("Write the UX layer (users, goals, navigation, decisions), the design rules and the layout specification of every page.\n\nReply with ONLY one JSON object in a ```json block, shape:\n" + designShape)
		return b.String()
	}
	b.WriteString("CURRENT DESIGN RULES:\n" + mustJSON(d.Rules) + "\n\nCURRENT LAYOUT SPECIFICATION:\n" + mustJSON(d.Layout) + "\n\n")
	if r := d.Request; r != nil && r.Text != "" {
		b.WriteString("USER REQUEST:\n" + r.Text + "\n\n")
		if len(r.Focus) > 0 {
			b.WriteString("The request is about: " + strings.Join(r.Focus, ", ") + ".\n\n")
		}
	}
	if rejected != "" {
		b.WriteString("YOUR LAST ANSWER WAS REFUSED, fix this: " + rejected + "\n\n")
	}
	b.WriteString(`Revision rules:
- decided decisions override the current design: do not keep a page, role, permission, copy or access rule that contradicts them.
- change only what the request affects. Return rules only when they change, then in full. Return only the shells and pages you add or change, each in full; keep their ids.
- return users, goals, navigation or decisions only when they change, then in full; a decision removed with a page is dropped automatically.
- remove_pages and remove_shells list the ids to delete; links, navigation items and flow steps to removed pages are dropped.
- return flows only for flows whose pages change.
- what is not returned stays as it is.

Reply with ONLY one JSON object in a ` + "```json" + ` block, shape (every key optional):
` + `{"rules": {...}, "users": [...], "goals": [...], "navigation": [...], "decisions": [...], "assumptions": [""], "unknowns": [""], "shells": [...], "pages": [...], "remove_shells": ["id"], "remove_pages": ["id"], "flows": {"FLOW-001": ["page-id"]}}`)
	return b.String()
}

func requiredStates() string {
	var out []string
	for _, pat := range core.LayoutPatterns {
		if st := core.RequiredStates[pat]; len(st) > 0 {
			out = append(out, pat+" → "+strings.Join(st, "+"))
		}
	}
	return strings.Join(out, "; ")
}

// designReference turns the user's design reference into a design-stage
// instruction: extract the characteristics that matter, adapt them to this
// product, never clone the reference's brand or assets, and record the
// reference so the rules stay traceable.
func designReference(p *core.Project) string {
	if p == nil || p.Survey == nil {
		return ""
	}
	ref := strings.TrimSpace(p.Survey.DesignReference)
	if ref == "" {
		return ""
	}
	return `DESIGN REFERENCE (the user asked the UI to follow this template or product):
` + ref + `
Translate it into THIS product's design rules, adapted to its function, users and data:
- Identify the characteristics that matter: color palette and mood, typography (family, scale, weight), spacing and radius, elevation, layout and navigation structure, component patterns, interaction and motion, content tone.
- Convert them into concrete values in the structured output: tokens (colors, typography, spacing, radius, shadow, breakpoints), components with use and variants, principles, interaction and responsive rules. Do not answer with the reference name alone; derive the rules.
- Adapt, never clone: keep the feel but fit THIS product's tasks and data volume. Do not copy logos, brand identity, proprietary assets or exact screens, and do not reproduce copyrighted visual material.
- The existing design system (if any) still wins over the reference; the reference fills only what the product needs.
- Put the reference in rules.reference (the user's words). If the reference is too thin or ambiguous to derive from, choose the most fitting pattern and record the assumption in assumptions, naming the reference.
`
}

// uxBrief is the UX input: who the users are, what they must accomplish,
// which data and operations exist, and the constraints. It is quoted from
// the blueprint, never inferred from table names.
func uxBrief(p *core.Project) string {
	var b strings.Builder
	b.WriteString("UX INPUTS (facts from the blueprint; derive goals, pages and decisions from them):\n")
	if ux := p.UXPlatforms(); len(ux) > 0 {
		b.WriteString("- Platform targets: " + strings.Join(ux, ", ") + "\n")
	}
	if d := p.Discovery; d != nil {
		if len(d.Users) > 0 {
			b.WriteString("- Users named in discovery: " + strings.Join(d.Users, ", ") + "\n")
		}
		if len(d.Roles) > 0 {
			b.WriteString("- Roles: ")
			for i, r := range d.Roles {
				if i > 0 {
					b.WriteString("; ")
				}
				fmt.Fprintf(&b, "%s (%s): %s", r.Name, r.Platform, r.Description)
			}
			b.WriteString("\n")
		}
		if len(d.Workflows) > 0 {
			b.WriteString("- Workflows: " + strings.Join(d.Workflows, " | ") + "\n")
		}
		if len(d.Security) > 0 {
			b.WriteString("- Security rules: " + strings.Join(d.Security, " | ") + "\n")
		}
		if len(d.Integrations) > 0 {
			b.WriteString("- Integrations: " + strings.Join(d.Integrations, " | ") + "\n")
		}
	}
	if len(p.Flows) > 0 {
		b.WriteString("- Use cases (tasks):\n")
		for _, fl := range p.Flows {
			fmt.Fprintf(&b, "  - %s %s; role %s; goal: %s", fl.ID, fl.Name, fl.Role, firstNonEmpty(fl.Goal, fl.Name))
			if len(fl.Steps) > 0 {
				fmt.Fprintf(&b, "; steps %s", strings.Join(fl.Steps, " -> "))
			}
			if len(fl.Rules) > 0 {
				fmt.Fprintf(&b, "; rules %s", strings.Join(fl.Rules, " | "))
			}
			if len(fl.Permissions) > 0 {
				fmt.Fprintf(&b, "; perm %s", strings.Join(fl.Permissions, ", "))
			}
			if len(fl.Data) > 0 {
				fmt.Fprintf(&b, "; data %s", strings.Join(fl.Data, " | "))
			}
			if len(fl.Failures) > 0 {
				fmt.Fprintf(&b, "; fail %s", strings.Join(fl.Failures, " | "))
			}
			b.WriteString("\n")
		}
	}
	if bp := p.Blueprint; bp != nil {
		if len(bp.Database.Entities) > 0 {
			b.WriteString("- Data model (shape only; do not turn every entity into a page):\n")
			for _, e := range bp.Database.Entities {
				fmt.Fprintf(&b, "  - %s", e.Name)
				if e.Module != "" {
					fmt.Fprintf(&b, " [%s]", e.Module)
				}
				if n := len(e.Fields); n > 0 {
					fields := e.Fields
					if n > 8 {
						fields = append(append([]string{}, e.Fields[:8]...), "…")
					}
					fmt.Fprintf(&b, ": %s", strings.Join(fields, ", "))
				}
				if len(e.Relates) > 0 {
					fmt.Fprintf(&b, "; relates to %s", strings.Join(e.Relates, ", "))
				}
				b.WriteString("\n")
			}
		}
		if len(bp.Modules) > 0 {
			b.WriteString("- Operations (module interfaces):\n")
			for _, m := range bp.Modules {
				if len(m.Interface) > 0 {
					fmt.Fprintf(&b, "  - %s: %s\n", m.ID, strings.Join(m.Interface, ", "))
				}
			}
		}
		if len(bp.Authz.Permissions) > 0 {
			b.WriteString("- Permissions: " + strings.Join(bp.Authz.Permissions, ", ") + "\n")
		}
		if bp.Platform.Offline != "" || bp.Platform.Realtime != "" {
			fmt.Fprintf(&b, "- Platform constraints: offline %s, realtime %s, deployment %s\n",
				dashText(bp.Platform.Offline), dashText(bp.Platform.Realtime), dashText(bp.Platform.Deployment))
		}
	}
	if d := p.Discovery; d != nil && len(d.Data) > 0 {
		b.WriteString("- Discovery data notes: " + strings.Join(d.Data, " | ") + "\n")
	}
	if p.Survey != nil && strings.TrimSpace(p.Survey.Constraints) != "" {
		b.WriteString("- Mandatory constraints: " + strings.TrimSpace(p.Survey.Constraints) + "\n")
	}
	b.WriteString("- Data volume: not stated in the blueprint unless the story says so; do not assume few or many records. " +
		"If volume matters to the pattern, record the assumption in assumptions and cite it in the decision.\n")
	return b.String()
}

// ApplyDesign lands the AI's design or revision. The result must pass the
// design check or nothing changes.
func ApplyDesign(p *core.Project, raw string) error {
	var out designOut
	if err := ExtractJSON(raw, &out); err != nil {
		return err
	}
	fresh := !p.UI.Written()
	switch {
	case fresh && (out.Rules == nil || len(out.Pages) == 0):
		return errors.New("rancangan UI tidak lengkap: rules dan pages wajib diisi")
	case !fresh && out.Rules == nil && len(out.Shells)+len(out.Pages)+len(out.RemoveShells)+len(out.RemovePages)+len(out.Flows) == 0 && !out.hasExperience():
		return ErrNoChange
	}
	next := cloneDesign(p.UI)
	if out.Rules != nil {
		next.Rules = normRules(*out.Rules)
	}
	l := next.Layout
	if l == nil {
		l = &core.LayoutSpec{}
		next.Layout = l
	}
	if out.Users != nil {
		l.Users = normUsers(*out.Users)
	}
	if out.Goals != nil {
		l.Goals = normGoals(*out.Goals)
	}
	if out.Navigation != nil {
		l.Navigation = normNavigation(*out.Navigation)
	}
	if out.Decisions != nil {
		l.Decisions = mergeDesignDecisions(l.Decisions, *out.Decisions)
	}
	if out.Assumptions != nil {
		l.Assumptions = nonEmpty(*out.Assumptions)
	}
	if out.Unknowns != nil {
		l.Unknowns = nonEmpty(*out.Unknowns)
	}
	for _, id := range out.RemoveShells {
		i := slices.IndexFunc(l.Shells, func(s core.LayoutShell) bool { return s.ID == normID(id) })
		if i < 0 {
			return fmt.Errorf("shell %q yang dihapus tidak ada", id)
		}
		l.Shells = slices.Delete(l.Shells, i, i+1)
	}
	gone := map[string]bool{}
	for _, id := range out.RemovePages {
		i := slices.IndexFunc(l.Pages, func(pg core.LayoutPage) bool { return pg.ID == normID(id) })
		if i < 0 {
			return fmt.Errorf("halaman %q yang dihapus tidak ada", id)
		}
		gone[l.Pages[i].ID] = true
		l.Pages = slices.Delete(l.Pages, i, i+1)
	}
	for _, sh := range out.Shells {
		sh = normShell(sh)
		if i := slices.IndexFunc(l.Shells, func(x core.LayoutShell) bool { return x.ID == sh.ID }); i >= 0 {
			l.Shells[i] = sh
		} else {
			l.Shells = append(l.Shells, sh)
		}
	}
	for _, pg := range out.Pages {
		pg = normPage(pg)
		if i := slices.IndexFunc(l.Pages, func(x core.LayoutPage) bool { return x.ID == pg.ID }); i >= 0 {
			l.Pages[i] = pg
		} else {
			l.Pages = append(l.Pages, pg)
		}
	}
	flows := slices.Clone(p.Flows)
	for id, pages := range out.Flows {
		i := slices.IndexFunc(flows, func(f core.Flow) bool { return f.ID == id })
		if i < 0 {
			return fmt.Errorf("alur %q tidak ada", id)
		}
		flows[i].PageIDs = nil
		for _, pg := range pages {
			flows[i].PageIDs = append(flows[i].PageIDs, normID(pg))
		}
	}
	dropPages(l, flows, gone)
	applyExisting(p, next)
	completeDesign(p, next)
	if fresh || len(l.Users) > 0 && len(l.Goals) > 0 && len(l.Navigation) > 0 && len(l.Decisions) > 0 {
		next.Format = core.UIDesignFormat
	}
	if err := commitDesign(p, next, flows); err != nil {
		return err
	}
	if fresh {
		p.UI.TechVersion = p.Stack.Version() // written with this decision
	}
	return nil
}

// commitDesign checks next and lands it with flows. Warnings do not block.
func commitDesign(p *core.Project, next *core.UIDesign, flows []core.Flow) error {
	check := *p
	check.Flows = flows
	if errs := core.Errors(check.DesignIssues(next)); len(errs) > 0 {
		return &core.Invalid{Issues: errs}
	}
	next.Request = nil
	next.Version++
	next.UpdatedAt = time.Now().UTC()
	p.UI, p.Flows = next, flows
	return nil
}

// dropPages removes links, navigation items, decisions and flow steps to
// pages that are gone.
func dropPages(l *core.LayoutSpec, flows []core.Flow, gone map[string]bool) {
	if len(gone) == 0 {
		return
	}
	for i := range l.Pages {
		l.Pages[i].Links = slices.DeleteFunc(l.Pages[i].Links, func(ln core.PageLink) bool { return gone[ln.To] })
	}
	for i := range l.Navigation {
		l.Navigation[i].Items = slices.DeleteFunc(l.Navigation[i].Items, func(it core.NavItem) bool { return gone[it.Page] })
	}
	l.Navigation = slices.DeleteFunc(l.Navigation, func(g core.NavGroup) bool { return len(g.Items) == 0 })
	l.Decisions = slices.DeleteFunc(l.Decisions, func(d core.DesignDecision) bool { return gone[d.Subject] })
	for i := range flows {
		flows[i].PageIDs = slices.DeleteFunc(slices.Clone(flows[i].PageIDs), func(id string) bool { return gone[id] })
	}
}

// EditDesignRules saves the user's design rules.
func EditDesignRules(p *core.Project, in core.DesignRules) error {
	next := cloneDesign(p.UI)
	next.Rules = normRules(in)
	return commitDesign(p, next, p.Flows)
}

// EditLayout saves the user's layout spec. Links and flow steps to pages
// the user removed are dropped.
func EditLayout(p *core.Project, in core.LayoutSpec) error {
	next := cloneDesign(p.UI)
	l := &core.LayoutSpec{}
	for _, sh := range in.Shells {
		l.Shells = append(l.Shells, normShell(sh))
	}
	for _, pg := range in.Pages {
		l.Pages = append(l.Pages, normPage(pg))
	}
	l.Users = normUsers(in.Users)
	l.Goals = normGoals(in.Goals)
	l.Navigation = normNavigation(in.Navigation)
	l.Decisions = normDecisions(in.Decisions)
	l.Assumptions = nonEmpty(in.Assumptions)
	l.Unknowns = nonEmpty(in.Unknowns)
	if len(l.Users) > 0 && len(l.Goals) > 0 && len(l.Navigation) > 0 && len(l.Decisions) > 0 {
		next.Format = core.UIDesignFormat
	}
	gone := map[string]bool{}
	if next.Layout != nil {
		for _, pg := range next.Layout.Pages {
			if !slices.ContainsFunc(l.Pages, func(x core.LayoutPage) bool { return x.ID == pg.ID }) {
				gone[pg.ID] = true
			}
		}
	}
	next.Layout = l
	flows := slices.Clone(p.Flows)
	dropPages(l, flows, gone)
	return commitDesign(p, next, flows)
}

// RequestDesignChange records a change request for the AI. focus are the
// design artifacts, shells or pages it is about.
func RequestDesignChange(p *core.Project, text string, focus []string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return errors.New("tuliskan perubahan yang diinginkan")
	}
	if !p.UI.Written() {
		return errors.New("aturan desain dan layout belum disusun")
	}
	known := map[string]bool{core.ArtifactDesignRules: true, core.ArtifactLayoutSpec: true}
	if l := p.UI.Layout; l != nil {
		for _, sh := range l.Shells {
			known[sh.ID] = true
		}
		for _, pg := range l.Pages {
			known[pg.ID] = true
		}
	}
	for _, f := range focus {
		if !known[f] {
			return fmt.Errorf("%q bukan artefak desain, shell, atau halaman", f)
		}
	}
	p.UI.Request = &core.ChangeRequest{Text: text, Focus: focus}
	return nil
}

func cloneDesign(d *core.UIDesign) *core.UIDesign {
	if d == nil {
		return &core.UIDesign{}
	}
	var out core.UIDesign
	b, _ := json.Marshal(d)
	json.Unmarshal(b, &out)
	return &out
}

// applyExisting makes an existing project's design follow its code: the
// tokens the code declares win over the AI's, its shared components are
// listed for reuse and pages on existing routes are marked existing.
func applyExisting(p *core.Project, d *core.UIDesign) {
	if p.Structure == nil || p.Structure.Mode != core.StructureExisting || p.Structure.UI == nil {
		return
	}
	ui := p.Structure.UI
	if r := d.Rules; r != nil {
		r.Source = core.RulesExisting
		if ui.Tokens != nil {
			code := *ui.Tokens
			groups := r.Tokens.Groups()
			for i, g := range code.Groups() {
				*groups[i].Tokens = mergeTokens(*g.Tokens, *groups[i].Tokens)
			}
			for _, s := range ui.TokenSources {
				if !slices.Contains(r.Sources, s) {
					r.Sources = append(r.Sources, s)
				}
			}
		}
		for _, c := range ui.Components {
			name, src := splitComponent(c)
			i := slices.IndexFunc(r.Components, func(x core.ComponentRule) bool { return strings.EqualFold(x.Name, name) })
			switch {
			case i < 0:
				r.Components = append(r.Components, core.ComponentRule{Name: name, Use: "komponen yang sudah ada; pakai ulang, jangan buat ulang", Source: src})
			case r.Components[i].Source == "":
				r.Components[i].Source = src
			}
		}
	}
	if l := d.Layout; l != nil {
		for i := range l.Pages {
			if l.Pages[i].Platform == core.PlatformWeb && slices.Contains(ui.Routes, l.Pages[i].Route) {
				l.Pages[i].Existing = true
			}
		}
	}
}

// mergeTokens keeps the code's tokens and adds the AI's tokens of other
// names.
func mergeTokens(code, ai []core.Token) []core.Token {
	out := slices.Clone(code)
	for _, t := range ai {
		if !slices.ContainsFunc(out, func(x core.Token) bool { return x.Name == t.Name }) {
			out = append(out, t)
		}
	}
	return out
}

// splitComponent parses "Button (src/components/ui/button.tsx)".
func splitComponent(s string) (name, src string) {
	if i := strings.Index(s, " ("); i > 0 && strings.HasSuffix(s, ")") {
		return s[:i], s[i+2 : len(s)-1]
	}
	return s, ""
}

func normID(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// presentationAliases maps a presentation the designer names onto the
// supported one that means the same thing, so a valid choice is never
// dropped or hidden just to pass validation. Record identity (symbol, name)
// is the record's title, not a separate presentation type.
var presentationAliases = map[string]string{
	"identity": "card-title",
}

func normPresentation(s string) string {
	s = normID(s)
	if a, ok := presentationAliases[s]; ok {
		return a
	}
	return s
}

func normRules(r core.DesignRules) *core.DesignRules {
	r.Source = normID(r.Source)
	if r.Source == "" {
		r.Source = core.RulesNew
	}
	r.Reference = strings.TrimSpace(r.Reference)
	r.Direction = strings.TrimSpace(r.Direction)
	for _, l := range []*[]string{&r.Principles, &r.UX, &r.Hierarchy, &r.Accessibility, &r.Responsive, &r.Interaction, &r.Sources} {
		*l = nonEmpty(*l)
	}
	for _, g := range r.Tokens.Groups() {
		var toks []core.Token
		for _, t := range *g.Tokens {
			t = core.Token{Name: strings.TrimSpace(t.Name), Value: strings.TrimSpace(t.Value), Use: strings.TrimSpace(t.Use)}
			if t.Name != "" || t.Value != "" {
				toks = append(toks, t)
			}
		}
		*g.Tokens = toks
	}
	var comps []core.ComponentRule
	for _, c := range r.Components {
		c = core.ComponentRule{Name: strings.TrimSpace(c.Name), Use: strings.TrimSpace(c.Use), Variants: nonEmpty(c.Variants),
			Rules: nonEmpty(c.Rules), Source: strings.TrimSpace(c.Source)}
		if c.Name != "" || c.Use != "" {
			comps = append(comps, c)
		}
	}
	r.Components = comps
	var plats []core.PlatformRules
	for _, pr := range r.Platforms {
		pr = core.PlatformRules{Platform: normID(pr.Platform), Rules: nonEmpty(pr.Rules)}
		if pr.Platform != "" || len(pr.Rules) > 0 {
			plats = append(plats, pr)
		}
	}
	r.Platforms = plats
	return &r
}

func normUsers(in []core.UIUser) []core.UIUser {
	var out []core.UIUser
	for _, u := range in {
		u = core.UIUser{Role: strings.TrimSpace(u.Role), Platform: normID(u.Platform), Context: strings.TrimSpace(u.Context),
			Goals: nonEmpty(u.Goals), Tasks: nonEmpty(u.Tasks)}
		if u.Role != "" {
			out = append(out, u)
		}
	}
	return out
}

func normGoals(in []core.UIGoal) []core.UIGoal {
	var out []core.UIGoal
	for _, g := range in {
		g = core.UIGoal{ID: strings.ToUpper(strings.TrimSpace(g.ID)), Role: strings.TrimSpace(g.Role),
			Text: strings.TrimSpace(g.Text), Success: strings.TrimSpace(g.Success), Priority: normID(g.Priority)}
		if g.ID != "" || g.Text != "" {
			out = append(out, g)
		}
	}
	return out
}

func normNavigation(in []core.NavGroup) []core.NavGroup {
	var out []core.NavGroup
	for _, in := range in {
		g := core.NavGroup{Name: strings.TrimSpace(in.Name)}
		for _, it := range in.Items {
			it = core.NavItem{Label: strings.TrimSpace(it.Label), Page: normID(it.Page), Primary: it.Primary}
			if it.Label != "" || it.Page != "" {
				g.Items = append(g.Items, it)
			}
		}
		if g.Name != "" || len(g.Items) > 0 {
			out = append(out, g)
		}
	}
	return out
}

func normDecisions(in []core.DesignDecision) []core.DesignDecision {
	var out []core.DesignDecision
	for _, d := range in {
		d = core.DesignDecision{Subject: strings.TrimSpace(d.Subject), Topic: strings.TrimSpace(d.Topic), Choice: strings.TrimSpace(d.Choice),
			Why: nonEmpty(d.Why), Basis: nonEmpty(d.Basis), Source: normID(d.Source), Alternative: strings.TrimSpace(d.Alternative)}
		if d.Subject != "" || d.Choice != "" {
			out = append(out, d)
		}
	}
	return out
}

// mergeDesignDecisions replaces the decisions of the subjects and topics an
// answer mentions and keeps the others: a revision of one page does not need
// to resend the whole decision register.
func mergeDesignDecisions(existing, in []core.DesignDecision) []core.DesignDecision {
	in = normDecisions(in)
	touched := map[string]bool{}
	for _, d := range in {
		touched[d.Subject+"\x00"+d.Topic] = true
	}
	var out []core.DesignDecision
	for _, d := range existing {
		if !touched[d.Subject+"\x00"+d.Topic] {
			out = append(out, d)
		}
	}
	return append(out, in...)
}

func dashText(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

func normShell(sh core.LayoutShell) core.LayoutShell {
	sh.ID, sh.Platform, sh.Pattern = normID(sh.ID), normID(sh.Platform), normID(sh.Pattern)
	sh.Name, sh.Navigation, sh.Grid, sh.Source = strings.TrimSpace(sh.Name), strings.TrimSpace(sh.Navigation), strings.TrimSpace(sh.Grid), strings.TrimSpace(sh.Source)
	sh.Responsive = nonEmpty(sh.Responsive)
	sh.Regions = normSections(sh.Regions)
	return sh
}

func normPage(pg core.LayoutPage) core.LayoutPage {
	pg.ID, pg.Platform, pg.Pattern, pg.Access, pg.Shell = normID(pg.ID), normID(pg.Platform), normID(pg.Pattern), normID(pg.Access), normID(pg.Shell)
	pg.FeatureID = strings.ToUpper(strings.TrimSpace(pg.FeatureID))
	for _, f := range []*string{&pg.Name, &pg.Route, &pg.Role, &pg.Purpose, &pg.Goal, &pg.PrimaryAction} {
		*f = strings.TrimSpace(*f)
	}
	var fields []core.FieldRule
	for _, f := range pg.Fields {
		f = core.FieldRule{Field: strings.TrimSpace(f.Field), Label: strings.TrimSpace(f.Label),
			Presentation: normPresentation(f.Presentation), Format: strings.TrimSpace(f.Format), Note: strings.TrimSpace(f.Note)}
		if f.Field != "" {
			fields = append(fields, f)
		}
	}
	pg.Fields = fields
	pg.Sections = normSections(pg.Sections)
	pg.Hierarchy = nonEmpty(pg.Hierarchy)
	for i := range pg.Hierarchy {
		pg.Hierarchy[i] = normID(pg.Hierarchy[i])
	}
	pg.Responsive, pg.Data = nonEmpty(pg.Responsive), nonEmpty(pg.Data)
	var links []core.PageLink
	for _, ln := range pg.Links {
		ln = core.PageLink{To: normID(ln.To), Via: strings.TrimSpace(ln.Via)}
		if ln.To != "" || ln.Via != "" {
			links = append(links, ln)
		}
	}
	pg.Links = links
	pg.States = normStates(pg.States)
	return pg
}

func normSections(in []core.Section) []core.Section {
	var out []core.Section
	for _, s := range in {
		s.ID, s.Pattern = normID(s.ID), normID(s.Pattern)
		for _, f := range []*string{&s.Name, &s.Purpose, &s.Grid, &s.Alignment, &s.Spacing, &s.Sizing, &s.Responsive} {
			*f = strings.TrimSpace(*f)
		}
		s.Content, s.Components = nonEmpty(s.Content), nonEmpty(s.Components)
		s.States = normStates(s.States)
		out = append(out, s)
	}
	return out
}

func normStates(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := map[string]string{}
	for k, v := range in {
		if v = strings.TrimSpace(v); v != "" {
			out[normID(k)] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// defaultStateText is the fallback description for a state the designer did
// not describe. The generator fills the states every pattern requires so the
// blueprint never fails validation on an omitted loading/empty/error state;
// the designer's text always wins.
var defaultStateText = map[string]string{
	"loading":           "Menampilkan indikator loading saat data dimuat",
	"empty":             "Menampilkan bahwa belum ada data dan langkah selanjutnya",
	"no_results":        "Menampilkan bahwa pencarian tidak menemukan hasil",
	"error":             "Menampilkan pesan kesalahan dan cara memuat ulang",
	"success":           "Menampilkan konfirmasi bahwa aksi berhasil",
	"disabled":          "Menampilkan kondisi saat aksi belum tersedia",
	"permission_denied": "Menampilkan penolakan akses bagi pengguna tanpa izin",
	"interactive":       "Menampilkan interaksi yang tersedia",
}

// completeDesign fills structural gaps the designer omitted so a nearly-valid
// answer lands instead of a full LLM retry. Text the designer wrote always wins.
func completeDesign(p *core.Project, d *core.UIDesign) {
	if d == nil || d.Layout == nil {
		return
	}
	l := d.Layout
	completeUsers(p, l)
	completeGoals(p, l)
	completePageGoals(l)
	completeNavigation(l)
	completePageStates(l.Pages)
	completePageDecisions(l)
	sanitizeDecisions(p, l, d.Rules)
}

func completeUsers(p *core.Project, l *core.LayoutSpec) {
	if len(l.Users) > 0 || p == nil || p.Discovery == nil {
		return
	}
	seen := map[string]bool{}
	for _, r := range p.Discovery.Roles {
		role := strings.TrimSpace(r.Name)
		if role == "" || seen[strings.ToLower(role)] {
			continue
		}
		seen[strings.ToLower(role)] = true
		u := core.UIUser{Role: role, Platform: normID(r.Platform), Context: strings.TrimSpace(r.Description)}
		if u.Context != "" {
			u.Tasks = []string{u.Context}
		}
		l.Users = append(l.Users, u)
	}
}

func completeGoals(p *core.Project, l *core.LayoutSpec) {
	if len(l.Goals) > 0 || p == nil {
		return
	}
	for i, fl := range p.Flows {
		text := firstNonEmpty(fl.Goal, fl.Name)
		if text == "" {
			continue
		}
		l.Goals = append(l.Goals, core.UIGoal{
			ID:       fmt.Sprintf("GOAL-%d", i+1),
			Role:     strings.TrimSpace(fl.Role),
			Text:     text,
			Priority: "must",
		})
	}
}

func completePageGoals(l *core.LayoutSpec) {
	if len(l.Goals) == 0 {
		return
	}
	first := l.Goals[0].ID
	known := map[string]bool{}
	byRole := map[string]string{}
	for _, g := range l.Goals {
		known[strings.ToLower(g.ID)] = true
		if r := strings.ToLower(strings.TrimSpace(g.Role)); r != "" && byRole[r] == "" {
			byRole[r] = g.ID
		}
	}
	for i := range l.Pages {
		g := strings.TrimSpace(l.Pages[i].Goal)
		if g != "" && known[strings.ToLower(g)] {
			continue
		}
		if id := byRole[strings.ToLower(l.Pages[i].Role)]; id != "" {
			l.Pages[i].Goal = id
			continue
		}
		l.Pages[i].Goal = first
	}
}

func completeNavigation(l *core.LayoutSpec) {
	if len(l.Navigation) > 0 || len(l.Pages) == 0 {
		return
	}
	nested := map[string]bool{"detail": true, "form": true, "wizard": true, "modal": true, "auth": true}
	var items []core.NavItem
	for _, pg := range l.Pages {
		if nested[pg.Pattern] {
			continue
		}
		label := firstNonEmpty(pg.Name, pg.ID)
		items = append(items, core.NavItem{Label: label, Page: pg.ID, Primary: true})
	}
	if len(items) == 0 {
		for _, pg := range l.Pages {
			items = append(items, core.NavItem{Label: firstNonEmpty(pg.Name, pg.ID), Page: pg.ID})
		}
	}
	if len(items) > 0 {
		l.Navigation = []core.NavGroup{{Name: "Utama", Items: items}}
	}
}

func completePageDecisions(l *core.LayoutSpec) {
	have := map[string]bool{}
	for _, d := range l.Decisions {
		have[strings.TrimSpace(d.Subject)] = true
	}
	for _, pg := range l.Pages {
		if have[pg.ID] {
			continue
		}
		why := firstNonEmpty(pg.Purpose, pg.Name, pg.Pattern)
		var basis []string
		if pg.Goal != "" {
			basis = append(basis, pg.Goal)
		}
		if pg.FeatureID != "" {
			basis = append(basis, pg.FeatureID)
		}
		src := "assumption"
		if len(basis) > 0 {
			src = "fact"
		}
		l.Decisions = append(l.Decisions, core.DesignDecision{
			Subject: pg.ID,
			Choice:  firstNonEmpty(pg.Pattern, "custom"),
			Why:     []string{why},
			Basis:   basis,
			Source:  src,
		})
	}
}

func sanitizeDecisions(p *core.Project, l *core.LayoutSpec, rules *core.DesignRules) {
	known := knownDesignBasis(p, l, rules)
	for i := range l.Decisions {
		d := &l.Decisions[i]
		if strings.TrimSpace(d.Choice) == "" {
			d.Choice = "custom"
		}
		if len(nonEmpty(d.Why)) == 0 {
			d.Why = []string{firstNonEmpty(d.Choice, "pola halaman")}
		} else {
			d.Why = nonEmpty(d.Why)
		}
		if !slices.Contains(core.DecisionSources, d.Source) {
			d.Source = "assumption"
		}
		var basis []string
		for _, b := range d.Basis {
			b = strings.TrimSpace(b)
			if b != "" && known[strings.ToLower(b)] {
				basis = append(basis, b)
			}
		}
		if len(basis) == 0 {
			if sub := strings.TrimSpace(d.Subject); known[strings.ToLower(sub)] {
				basis = []string{sub}
			}
		}
		d.Basis = basis
	}
}

func knownDesignBasis(p *core.Project, l *core.LayoutSpec, rules *core.DesignRules) map[string]bool {
	out := map[string]bool{}
	add := func(v string) {
		if v = strings.ToLower(strings.TrimSpace(v)); v != "" {
			out[v] = true
		}
	}
	if l != nil {
		for _, g := range l.Goals {
			add(g.ID)
		}
		for _, u := range l.Users {
			add(u.Role)
		}
		for _, pg := range l.Pages {
			add(pg.ID)
			add(pg.FeatureID)
			add(pg.Goal)
		}
	}
	if p == nil {
		return out
	}
	if p.Discovery != nil {
		for _, r := range p.Discovery.Roles {
			add(r.Name)
		}
	}
	for _, fl := range p.Flows {
		add(fl.ID)
		for _, perm := range fl.Permissions {
			add(perm)
		}
	}
	for _, f := range p.Features {
		add(f.ID)
	}
	for _, r := range p.Requirements {
		add(r.ID)
	}
	for _, t := range p.UIPlatforms() {
		add(t)
	}
	if p.Blueprint != nil {
		for _, e := range p.Blueprint.Database.Entities {
			add(e.Name)
			for _, f := range e.Fields {
				add(e.Name + "." + f)
			}
		}
		for _, perm := range p.Blueprint.Authz.Permissions {
			add(perm)
		}
	}
	if rules != nil {
		for _, g := range rules.Tokens.Groups() {
			for _, t := range *g.Tokens {
				add(t.Name)
			}
		}
		for _, c := range rules.Components {
			add(c.Name)
		}
	}
	return out
}

// completePageStates guarantees every generated page specifies the states its
// pattern requires. A missing required state gets a default description; a
// state the designer already described is kept.
func completePageStates(pages []core.LayoutPage) {
	for i := range pages {
		pg := &pages[i]
		if pg.States == nil {
			pg.States = map[string]string{}
		}
		for _, st := range core.RequiredStates[pg.Pattern] {
			if strings.TrimSpace(pg.States[st]) == "" {
				pg.States[st] = defaultStateText[st]
			}
		}
	}
}

// nonEmpty trims items and drops the empty ones.
func nonEmpty(in []string) []string {
	var out []string
	for _, s := range in {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}
