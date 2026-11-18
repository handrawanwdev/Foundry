package work

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/uikit"
)

// The UI design is text and structured data only: design rules and a
// layout specification. The AI writes and revises them; Foundry validates
// every answer, repairs recoverable gaps, and lands a truncated inventory
// as a sketch instead of refusing the whole result.

// ErrNoChange means the AI found nothing to change for the request.
var ErrNoChange = errors.New("AI tidak menemukan perubahan rancangan UI untuk permintaan ini")

type designOut struct {
	Rules        *core.DesignRules  `json:"rules"`
	Shells       []core.LayoutShell `json:"shells"`
	Pages        []core.LayoutPage  `json:"pages"`
	RemoveShells []string           `json:"remove_shells"`
	RemovePages  []string           `json:"remove_pages"`
	Flows        map[string]flowOut `json:"flows"`
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
 "rules": {"source": "new", "preset": "ops-dense", "brand": {"primary": "#0e7490"}, "reference": "", "direction": "", "principles": [""], "ux_guidelines": [""],
   "tokens": {"colors": [{"name": "status.overdue", "value": "#9f1239", "use": "janji terlambat"}]},
   "components": [{"name": "SlotPicker", "use": "pilih jam praktik", "variants": ["default"], "rules": ["slot penuh disabled"]}],
   "accessibility": [""], "responsive": [""]},
 "shells": [{"id": "app", "name": "App", "platform": "web", "pattern": "sidebar",
   "regions": [{"id": "nav", "name": "Nav", "slot": "nav", "content": ["Menu"]}]}],
 "pages": [{"id": "antrean", "name": "Antrean", "platform": "web", "route": "/antrean", "access": "auth", "role": "Operator", "feature_id": "FEAT-001",
   "purpose": "Saring antrean", "goal": "GOAL-1", "shell": "app", "pattern": "table", "primary_action": "Panggil berikutnya",
   "fields": [{"field": "Appointment.status", "label": "Status", "presentation": "badge", "examples": ["Menunggu", "Selesai"]}],
   "sections": [{"id": "filter", "name": "Filter", "slot": "toolbar", "content": ["Cari pasien, Status"]}, {"id": "tabel", "name": "Tabel", "slot": "table", "content": ["No, Pasien, Status, Aksi"], "components": ["Table", "Badge"]}],
   "states": {"loading": "", "empty": "", "error": ""}}],
 "users": [{"role": "Operator", "platform": "web", "context": "kerja harian", "tasks": ["saring antrean"]}],
 "goals": [{"id": "GOAL-1", "role": "Operator", "text": "Saring antrean", "success": "Janji diproses", "priority": "must"}],
 "navigation": [{"name": "Operasional", "items": [{"label": "Antrean", "page": "antrean", "primary": true}]}],
 "decisions": [{"subject": "antrean", "choice": "table", "why": ["bandingkan banyak baris"], "basis": ["GOAL-1"], "source": "fact"}],
 "flows": {"FLOW-001": {"steps": ["antrean", ""], "branches": ["antrean"]}}
}`

const designRules = `DESIGN RULES (rules)
- source: "new", or "existing" when the product already has a frontend: its tokens and components ARE the design system; keep them and add only what is missing.
- reference: when the DESIGN REFERENCE section names a template or product, put it in reference (the user's words). The rules are derived from it, not a restatement of it: never copy its logo, brand identity, proprietary assets or exact screens; adapt its visual characteristics to THIS product's tasks and data. When no reference is given, leave reference empty.
- principles: 3-7 principles for THIS product and its users, each one actionable and checkable ("operators compare many rows: dense tables, not cards"). Never generic ("modern and clean").
- ux_guidelines: concrete rules for navigation, forms and validation, feedback, errors, empty states, confirmation of destructive actions.
- preset: required when source is "new" — one id from UI KIT PRESETS; Foundry fills colors, typography, spacing, radius, shadow and breakpoints from it. brand only to adapt accent color or font. tokens: only extra semantic colors the preset lacks. With source "existing" the code's tokens win; still pick the closest preset so missing tokens are filled consistently.
- components: the kit already defines Button, Input, Table, Badge, Card, Dialog, Toast, EmptyState, Tabs, Pagination — reference them by name in sections[].components and list them in rules.components only to change their rules. List every product-specific shared component: name, use, variants, rules (including hover, focus, active and disabled states). An existing component keeps its name and gets its file in source.
- visual_hierarchy: how importance is shown (size, weight, color, position). accessibility: WCAG 2.2 AA (contrast, focus, keyboard, labels, touch targets). responsive: the breakpoints and what changes at each. interaction: feedback, loading behavior, motion.
- platforms: omit unless a named platform in ALL PLATFORMS FOR UX RULES needs rules the others do not (e.g. Material 3 on android). Never emit a platform that was not selected.
`

const uxRules = `UX INTELLIGENCE (users, goals, navigation, decisions)
Work like a senior product designer: understand the users and their tasks BEFORE choosing a layout. A table, entity or field name is data, never a layout instruction.
1. Users: from the roles and the story, name each kind of user, the platform they use, the context (frequency, pressure, device, environment) and the tasks they must complete.
2. Goals: one measurable goal per important task (id GOAL-1, ...), the role it belongs to, and what success looks like. Goals come from the use cases, not from the data model.
3. Information architecture: group destinations by the user's job, not by database tables and not one group per page. Each group has a name and items[{label, page}]; a group without items is invalid. Primary nav lists home destinations only. Auth, create/edit forms, detail and modal pages are reached by links, not the top-level menu. Keep the top level small (usually one group).
4. Page inventory: one page per user task or decision point, never per database entity and never one page per feature. Reuse a page when the task is the same; split when the user's goal, permission or data scope differs. Typical 4-12 pages.
   Not a page: a field or status, a filter/search on a list, responsive/theming, automated tests, logout, confirm-delete. Those live on the page or shell that already serves the task.
   Do not invent landing, settings, profile, OAuth, admin, team or sharing unless the story or features name them.
5. Patterns: choose by task, data volume and platform, not by habit.
   table: compare many records with several attributes; list: scan with one dominant attribute; cards: small set with visual identity; detail: one record; form: create or edit; wizard: long/dependent sequence; dashboard: only when KPIs are a real task; search: large/unbounded set; modal/drawer: short side task; tabs: facets of one record; settings: configuration; master-detail: rapid switching.
   On a small screen prefer list/cards over wide tables. Do not make every page a dashboard or a card grid.
6. Density: operators who repeat a task need compact rows and few clicks; occasional users need more guidance; touch users need larger targets. State the choice.
7. Every page must have at least one decision whose subject is the page id itself, stating its pattern and why; a form page also names its structure, input validation, submit state and error feedback. No page is left without a decision. One decision per page is enough unless it is a form. why: one concrete sentence. basis: 1-2 ids from the inputs. source: fact|assumption|constraint. Do not write a decision that cannot name its basis.
8. Facts, assumptions and unknowns are separate. Never invent volume, permissions, compliance or integrations that the blueprint does not state; record the gap as an assumption or unknown.
9. Data-to-UI mapping: for each page with records, map only the fields shown (table-column, list-item, card-title, card-meta, card-value, delta, detail, form-input, filter, summary, chart, badge, row-action, hidden). Do not show every field of an entity. Omit storage secrets (password_hash, token_hash) and internal foreign keys, or mark them hidden. card-value is a prominent value; delta is a change indicator; row-action is a per-row action (note says what it does). Record identity (symbol, name) is card-title.
10. States, validation, permissions and feedback are part of the page design: loading, empty, no_results, error, success, disabled, permission_denied, and what a user without the permission sees.
11. For a CLI/terminal platform (no screen layout), put its UX in rules.platforms with platform "cli": command and subcommand grammar, flags and defaults, help and examples, output shape and columns, error messages and exit codes, confirmation for destructive actions, and non-interactive/pipe behavior. Do not create layout pages for the terminal.
`

const layoutRules = `LAYOUT SPECIFICATION (shells, pages)
- Text and structured data only. No images, mockups, HTML, CSS or code.
- shells: the layouts pages share (app frame with sidebar or navbar, auth frame, ...): id, name, platform, pattern, regions (sections), navigation, grid, responsive.
- pages: every screen the user tasks need, per platform with screens. route: web starts with /, mobile/desktop is the screen name. access: public or auth. role: the users[].role(s) the page is for, comma-separated (the actors of its use cases); a page a permission limits says in permission_denied what the others see. shell: a shell id, or empty for a page with its own frame.
- pages[].goal names the goal id the page serves; pages[].fields maps data fields to their presentation; examples: 3-5 real domain values a layperson knows ("EURUSD", "H1"), never "Item 1"; note: plain words for jargon.
- primary_action: the page's main action, a verb phrase from the use case step done there (table, list, detail, dashboard, form, wizard). Never a reason to add a page. On a decision page it is the approval, with the rejection beside it.
- pattern (page, shell and optional for a section) is one of: %s.
- sections[].slot: the slot of the page pattern's template (UI KIT PAGE TEMPLATES) the section fills; every required slot gets a section; no header section (the page header comes from name, purpose and primary_action). Shell regions take the shell template's slots.
- sections: in visual order (top to bottom, start to end). content lists the elements in order, concretely ("Tabel antrean: No, Pasien, Dokter, Status, Aksi"). components are names from rules.components. grid, alignment, spacing, sizing and responsive use the tokens (space.md, breakpoint.tablet), never arbitrary values.
- Every table or list of records has search + filter and pagination: toolbar/pagination sections, or content items "Filter: …" and "Paginasi: …".
- hierarchy: section ids from the most important.
- states (page or section): keys %s; the value says what the user sees and can do. Required per page pattern: %s.
- links: {to: page id, via: what the user activates}; together they are the navigation between pages.
- navigation: groups named by the user's job, each with items[{label, page}]. Empty groups are invalid. Every page is reachable from navigation or from a link.
- flows: every use case with screens -> {"steps": the page id of each step in order, "" for a system step (feedback on the current page, not a page); "branches": exactly one entry per b1..bn, in order: the page that shows it, "" when none}. Each step and branch has a section, field, state or action on its page; consecutive pages are linked (via = the step); a decision step's page offers approve and reject/return; an irreversible step (delete, cancel, post) names its confirmation; a long document flow (draft, submit, approve) gets form/detail pages or a wizard; a branch that hands over to another use case links to that one's first page; steps done at the same time (parallel) are on their own pages without links between them; a page of a document with a lifecycle shows its status and the actions its transitions allow in that status.
- ids: lowercase letters, digits and dashes. Page ids are unique; section ids are unique within their page.
`

const outputDiscipline = `OUTPUT DISCIPLINE
Emit compact JSON. Omit empty keys, empty arrays and empty strings. Do not copy placeholder empty values from the shape. The shape shows keys and types only; do not copy its product, roles, pages or fields.
- When DESIGN REFERENCE is present, rules.reference is required (the user's words); never omit it.
- Every pages[].shell must name a shells[].id emitted in this same answer.
- principles 3-5 short clauses; ux_guidelines 4-6; accessibility, responsive, interaction 2-4 each. Do not repeat the same sentence across those lists.
- rules.preset always; tokens only for extra semantic colors (0-4). Never restate preset tokens.
- components: only product-specific ones and kit components whose rules change (typically 0-6). One short variants list and one short rules line each.
- pages: sections 2-4; content 1-3 concrete items; fields only those shown.
- navigation: one or few groups; every group has items that name existing page ids. Never a group without items. Never one group per page.
- rules.platforms: omit unless a selected platform needs distinct rules; never emit an unselected platform.
- decisions: exactly one per page (pattern + why). Form pages may add one for validation/submit. why: one short sentence. basis: 1-2 existing ids from the UX inputs (GOAL-*, FLOW-*, FEAT-*, REQ-*, DEC-*, entity, permission, platform, token, page id). Never invent a basis.
- states: one short sentence per required key only. Foundry fills users, goals, navigation, decisions and required states when omitted; omit them rather than emit invalid ids.
- Emit keys in this order so a cut-off answer still has the required parts: rules, shells, pages, then users, goals, navigation, decisions, flows.
- Prefer a complete answer. If output would be truncated, keep rules and every page id (name, platform, route, pattern, purpose, goal, shell, links); omit sections/fields/states on later pages rather than dropping pages. Foundry asks for those pages next.
- Reply with JSON only. No preamble.
`

// DesignPrompt asks for the design rules and layout spec, or for a
// revision of them (the pending request). Pages that landed without
// sections get a focused detail prompt instead of a full rewrite.
func DesignPrompt(p *core.Project) string {
	if DesignNeedsDetails(p) {
		return designDetailPrompt(p)
	}
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
		b.WriteString("FEATURES (pages.feature_id): attach to a page that already serves the task; do not add a page per feature.\n")
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
	// system still wins. The UI kit turns it into choices: a curated token
	// preset and page templates whose slots the pages fill.
	b.WriteString(designTemplate + "\n")
	kit := uikit.Default()
	b.WriteString(kit.PresetBrief() + "\n")
	b.WriteString(kit.PatternBrief() + "\n")

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
		b.WriteString("Write the design rules first, then shells and every page, then the UX layer (users, goals, navigation, decisions).\n\nReply with ONLY one JSON object in a ```json block, shape:\n" + designShape)
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
` + `{"rules": {...}, "users": [...], "goals": [...], "navigation": [...], "decisions": [...], "assumptions": [""], "unknowns": [""], "shells": [...], "pages": [...], "remove_shells": ["id"], "remove_pages": ["id"], "flows": {"FLOW-001": {"steps": ["page-id", ""], "branches": ["page-id"]}}}`)
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
			if fl.Trigger != "" {
				fmt.Fprintf(&b, "; trigger %s", fl.Trigger)
			}
			if len(fl.Steps) > 0 {
				steps := make([]string, len(fl.Steps))
				for i, st := range fl.Steps {
					steps[i] = fmt.Sprintf("%d. %s", i+1, st)
				}
				fmt.Fprintf(&b, "; steps %s", strings.Join(steps, " "))
			}
			// Branches are numbered b1..bn, alternatives then failures: the
			// order of flows[].branches, so the designer can count them.
			n := 0
			numbered := func(in []string) string {
				out := make([]string, len(in))
				for i, s := range in {
					n++
					out[i] = fmt.Sprintf("b%d) %s", n, s)
				}
				return strings.Join(out, " | ")
			}
			if len(fl.Alternatives) > 0 {
				fmt.Fprintf(&b, "; alt %s", numbered(fl.Alternatives))
			}
			if len(fl.Failures) > 0 {
				fmt.Fprintf(&b, "; fail %s", numbered(fl.Failures))
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
			for _, g := range fl.Parallel {
				fmt.Fprintf(&b, "; parallel steps %v join %s", g.Steps, firstNonEmpty(g.Join, "all"))
			}
			b.WriteString("\n")
		}
	}
	if len(p.Lifecycles) > 0 {
		b.WriteString("- Document lifecycles (a page showing the document shows its status, offers only the transitions available in that status, keeps a locked status read-only, and shows no status not listed):\n")
		for _, lc := range p.Lifecycles {
			states := make([]string, len(lc.States))
			for i, st := range lc.States {
				states[i] = st.ID
				if st.Kind != "" || st.Locked {
					var tags []string
					if st.Kind != "" {
						tags = append(tags, st.Kind)
					}
					if st.Locked {
						tags = append(tags, "locked")
					}
					states[i] += " (" + strings.Join(tags, ", ") + ")"
				}
			}
			trans := make([]string, len(lc.Transitions))
			for i, t := range lc.Transitions {
				trans[i] = fmt.Sprintf("%s->%s %q by %s", strings.Join(t.From, "/"), t.To, t.Action, t.Actor)
				if t.Flow != "" && t.Step > 0 {
					trans[i] += fmt.Sprintf(" at %s step %d", t.Flow, t.Step)
				}
			}
			fmt.Fprintf(&b, "  - %s: %s; transitions %s\n", lc.Entity, strings.Join(states, ", "), strings.Join(trans, " | "))
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

// ApplyDesign lands the AI's design or revision. Repairable gaps are filled
// in-process. A truncated or inventory-only answer lands as a sketch and
// Foundry asks for the missing page details instead of refusing the whole
// result.
func ApplyDesign(p *core.Project, raw string) error {
	fresh := !p.UI.Written()
	sync := p.UI != nil && p.UI.Request != nil && p.UI.Request.Sync
	err := applyDesign(p, raw)
	if sync && errors.Is(err, ErrNoChange) {
		// The design already follows its sources.
		p.UI.Request, err = nil, nil
	}
	if err == nil && (fresh || sync) && p.UI != nil {
		p.UI.Basis = core.DesignBasis(p)
	}
	return err
}

// SyncDesign asks the AI to make the design follow what changed in the
// features, use cases and blueprint since it was last written or synced.
func SyncDesign(p *core.Project) error {
	if !p.UI.Written() {
		return errors.New("aturan desain dan layout belum disusun")
	}
	text := p.DesignDrift().SyncRequest()
	if missing := fieldsWithoutExamples(p.UI.Layout); missing != "" {
		text += "\nLengkapi examples (3-5 contoh data nyata dari domain ini yang dimengerti orang awam, bukan \"Item 1\") dan note berisi arti dengan kata sederhana untuk label istilah teknis, pada field: " + missing
	}
	p.UI.Request = &core.ChangeRequest{Text: text, Sync: true}
	return nil
}

// fieldsWithoutExamples lists the shown page fields the mockup has no
// example data for, by page: "pantau (Instrumen, Timeframe); log (Alasan)".
func fieldsWithoutExamples(l *core.LayoutSpec) string {
	if l == nil {
		return ""
	}
	var pages []string
	for _, pg := range l.Pages {
		var labels []string
		for _, f := range pg.Fields {
			if f.Presentation == "row-action" || f.Presentation == "hidden" || len(f.Examples) > 0 {
				continue
			}
			lb := f.Label
			if lb == "" {
				lb = f.Field
			}
			labels = append(labels, lb)
		}
		if len(labels) > 0 {
			pages = append(pages, pg.ID+" ("+strings.Join(labels, ", ")+")")
		}
	}
	return strings.Join(pages, "; ")
}

func applyDesign(p *core.Project, raw string) error {
	var out designOut
	hasDesign := func() bool { return out.Rules != nil || len(out.Pages) > 0 || len(out.Shells) > 0 }
	if err := ExtractJSONFunc(raw, &out, hasDesign); err != nil {
		salvaged := salvageDesign(raw)
		if salvaged == nil {
			if p.UI.Written() {
				var any map[string]json.RawMessage
				if ExtractJSON(raw, &any) == nil {
					return ErrNoChange
				}
			}
			if !p.UI.Written() {
				return errors.New("rancangan UI tidak lengkap: rules dan pages wajib diisi")
			}
			return err
		}
		out = *salvaged
	} else if !hasDesign() {
		if salvaged := salvageDesign(raw); salvaged != nil {
			out = *salvaged
		}
	}
	if DesignNeedsDetails(p) {
		return applyDesignDetails(p, out)
	}
	fresh := !p.UI.Written()
	switch {
	case fresh && out.Rules == nil && len(out.Pages) == 0 && len(out.Shells) == 0:
		return errors.New("rancangan UI tidak lengkap: rules dan pages wajib diisi")
	case fresh && len(out.Pages) == 0 && len(out.Shells) == 0:
		// Rules without pages still land; the next round asks for pages.
	case !fresh && out.Rules == nil && len(out.Shells)+len(out.Pages)+len(out.RemoveShells)+len(out.RemovePages)+len(out.Flows) == 0 && !out.hasExperience():
		return ErrNoChange
	}
	next := cloneDesign(p.UI)
	if out.Rules != nil {
		next.Rules = completeRules(p, kitRules(p, normRules(*out.Rules)))
	} else if fresh {
		next.Rules = defaultRules(p)
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
	if err := applyFlowsOut(flows, out.Flows); err != nil {
		return err
	}
	dropPages(l, flows, gone)
	applyExisting(p, next)
	completeDesign(p, next)
	completeFlows(p, next.Layout, flows)
	if fresh || len(l.Users) > 0 && len(l.Goals) > 0 && len(l.Navigation) > 0 && len(l.Decisions) > 0 {
		next.Format = core.UIDesignFormat
	}
	if pagesNeedDetails(next) {
		if err := commitDesignSketch(p, next, flows); err != nil {
			return err
		}
	} else if err := commitDesign(p, next, flows); err != nil {
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
		// A step or scenario on a removed page is placed again by its words.
		if slices.ContainsFunc(flows[i].StepPages, func(id string) bool { return gone[id] }) {
			flows[i].StepPages = nil
		}
		if slices.ContainsFunc(flows[i].BranchPages, func(id string) bool { return gone[id] }) {
			flows[i].BranchPages = nil
		}
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
	assignSlots(l)
	addCollectionControls(l)
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
	"identity": "card-title", "title": "card-title", "heading": "card-title", "name": "card-title",
	"subtitle": "card-meta", "meta": "card-meta", "caption": "card-meta",
	"column": "table-column", "cell": "table-column", "col": "table-column",
	"kpi": "card-value", "metric": "card-value", "value": "card-value", "price": "card-value",
	"change": "delta", "percent": "delta", "pct": "delta", "trend": "delta",
	"input": "form-input", "text": "form-input", "field": "form-input",
	"action": "row-action", "button": "row-action", "link": "row-action",
	"search": "filter", "query": "filter",
	"status": "badge", "tag": "badge", "chip": "badge",
	"sparkline": "chart", "graph": "chart",
	"total": "summary", "aggregate": "summary",
}

var patternAliases = map[string]string{
	"data-table": "table", "datatable": "table", "list-view": "list", "listview": "list",
	"card": "cards", "card-grid": "cards", "grid": "cards",
	"login": "auth", "signin": "auth", "sign-in": "auth", "signup": "auth", "sign-up": "auth", "register": "auth",
	"kanban-board": "kanban", "board": "kanban",
	"map-view": "map", "chat-view": "chat", "messenger": "chat",
	"landing-page": "landing", "settings-page": "settings",
	"popup": "modal", "dialog": "modal", "drawer": "modal",
	"stepper": "wizard", "steps": "wizard",
}

var decisionSubjectAliases = map[string]string{
	"navigation": "*", "nav": "*", "ia": "*", "app": "*", "layout": "*",
	"shell": "*", "global": "*", "product": "*",
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
	r.Preset = normID(r.Preset)
	if b := r.Brand; b != nil {
		nb := core.DesignBrand{Primary: strings.ToLower(strings.TrimSpace(b.Primary)), Font: strings.TrimSpace(b.Font)}
		r.Brand = nil
		if nb.Primary != "" || nb.Font != "" {
			r.Brand = &nb
		}
	}
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
		u.Platform, u.Context = userPlatform(u.Platform, u.Context)
		if u.Role != "" {
			out = append(out, u)
		}
	}
	return out
}

// userPlatform keeps one platform with screens for a user. A user written
// with several ("web, android, ios", seen live) keeps the first; one
// without screens ("backend") keeps none. The words the designer wrote stay
// in the context, so the design is not refused for them.
func userPlatform(platform, context string) (string, string) {
	if platform == "" || slices.Contains(core.UIPlatform, platform) {
		return platform, context
	}
	first := ""
	for _, part := range regexp.MustCompile(`,|/|;| dan | and | atau | or `).Split(platform, -1) {
		if f := strings.Fields(part); len(f) > 0 && slices.Contains(core.UIPlatform, f[0]) {
			first = f[0]
			break
		}
	}
	note := "platform: " + platform
	if context != "" {
		note = context + "; " + note
	}
	return first, note
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
	if a, ok := patternAliases[sh.Pattern]; ok {
		sh.Pattern = a
	}
	sh.Name, sh.Navigation, sh.Grid, sh.Source = strings.TrimSpace(sh.Name), strings.TrimSpace(sh.Navigation), strings.TrimSpace(sh.Grid), strings.TrimSpace(sh.Source)
	sh.Responsive = nonEmpty(sh.Responsive)
	sh.Regions = normSections(sh.Regions)
	return sh
}

func normPage(pg core.LayoutPage) core.LayoutPage {
	pg.ID, pg.Platform, pg.Pattern, pg.Access, pg.Shell = normID(pg.ID), normID(pg.Platform), normID(pg.Pattern), normID(pg.Access), normID(pg.Shell)
	if a, ok := patternAliases[pg.Pattern]; ok {
		pg.Pattern = a
	}
	pg.FeatureID = strings.ToUpper(strings.TrimSpace(pg.FeatureID))
	// A page has one feature; a list ("FEAT-029, FEAT-005", seen live)
	// keeps the first instead of refusing the design.
	if f := strings.FieldsFunc(pg.FeatureID, func(r rune) bool { return r == ',' || r == ';' || r == '/' || r == ' ' }); len(f) > 1 {
		pg.FeatureID = f[0]
	}
	for _, f := range []*string{&pg.Name, &pg.Route, &pg.Role, &pg.Purpose, &pg.Goal, &pg.PrimaryAction} {
		*f = strings.TrimSpace(*f)
	}
	var fields []core.FieldRule
	for _, f := range pg.Fields {
		f = core.FieldRule{Field: strings.TrimSpace(f.Field), Label: strings.TrimSpace(f.Label),
			Presentation: normPresentation(f.Presentation), Format: strings.TrimSpace(f.Format), Note: strings.TrimSpace(f.Note),
			Examples: nonEmpty(f.Examples)}
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
		s.ID, s.Pattern, s.Slot = normID(s.ID), normID(s.Pattern), normID(s.Slot)
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
	if d == nil {
		return
	}
	if d.Rules != nil {
		d.Rules = completeRules(p, d.Rules)
	}
	if d.Layout == nil {
		return
	}
	l := d.Layout
	repairLayout(p, d)
	assignSlots(l)
	addCollectionControls(l)
	addKitComponents(d)
	completeUsers(p, l)
	completeGoals(p, l)
	completePageGoals(l)
	completeNavigation(l)
	completePageStates(l.Pages)
	completePageData(l.Pages)
	completePageDecisions(l)
	sanitizeDecisions(p, l, d.Rules)
	repairDecisions(p, l, d.Rules)
}

// completePageData names the entities a page shows from its field
// mapping ("Payment.amount" → Payment) when the designer left data empty.
func completePageData(pages []core.LayoutPage) {
	for i := range pages {
		pg := &pages[i]
		if len(pg.Data) > 0 || pg.Existing {
			continue
		}
		for _, f := range pg.Fields {
			if e, _, ok := strings.Cut(f.Field, "."); ok && strings.TrimSpace(e) != "" && !slices.Contains(pg.Data, strings.TrimSpace(e)) {
				pg.Data = append(pg.Data, strings.TrimSpace(e))
			}
		}
	}
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
	if l == nil || len(l.Pages) == 0 {
		return
	}
	pages := map[string]core.LayoutPage{}
	for _, pg := range l.Pages {
		pages[pg.ID] = pg
	}
	var groups []core.NavGroup
	for _, g := range l.Navigation {
		var items []core.NavItem
		for _, it := range g.Items {
			pg, ok := pages[it.Page]
			if !ok {
				continue
			}
			if it.Label == "" {
				it.Label = firstNonEmpty(pg.Name, pg.ID)
			}
			items = append(items, it)
		}
		if len(items) == 0 {
			continue
		}
		if g.Name == "" {
			g.Name = "Utama"
		}
		g.Items = items
		groups = append(groups, g)
	}
	if len(groups) > 0 {
		l.Navigation = groups
		return
	}
	nested := map[string]bool{"detail": true, "form": true, "wizard": true, "modal": true, "auth": true}
	var items []core.NavItem
	for _, pg := range l.Pages {
		if nested[pg.Pattern] || pg.AccessIsPublic() {
			continue
		}
		items = append(items, core.NavItem{Label: firstNonEmpty(pg.Name, pg.ID), Page: pg.ID, Primary: true})
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
	for _, d := range p.Decisions {
		add(d.ID)
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
