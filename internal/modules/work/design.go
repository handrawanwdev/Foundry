package work

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/handrawanw/foundry/internal/core"
)

type designOut struct {
	System  *core.DesignSystem `json:"system"`
	Pages   []core.Page        `json:"pages"`
	Removed []string           `json:"removed"` // revision only: ids of deleted pages
	Patches []pagePatch        `json:"patches"` // revision only: edits to existing pages
	Flows   []struct {
		ID      string   `json:"id"`
		PageIDs []string `json:"page_ids"`
	} `json:"flows"`
}

var componentTypes = map[string]bool{
	"header": true, "nav": true, "text": true, "list": true, "table": true, "form": true,
	"button": true, "card": true, "stat": true, "input": true, "section": true, "steps": true,
}

// pageKinds are the layouts the wireframe knows; unknown kinds are dropped
// and the UI infers one from the components.
var pageKinds = map[string]bool{
	"auth": true, "dashboard": true, "list": true, "detail": true, "form": true, "wizard": true, "search": true,
	"settings": true, "payment": true, "profile": true, "analytics": true, "admin": true, "status": true,
}

var accessLevels = map[string]bool{"public": true, "auth": true}

var fieldTypes = map[string]bool{
	"text": true, "email": true, "password": true, "number": true, "tel": true, "date": true, "time": true,
	"datetime": true, "select": true, "textarea": true, "checkbox": true, "radio": true, "file": true, "currency": true,
}

// kindSubmit is the submit label a form gets from the page kind; the
// wireframe renders it (apps/web Wireframe.tsx), so it never needs a button.
var kindSubmit = map[string]string{"auth": "Masuk", "payment": "Bayar", "search": "Cari", "wizard": "Lanjut"}

// primarySubmit: kinds where the form is the page's main action, so its
// submit button carries the primary_action label.
var primarySubmit = map[string]bool{"auth": true, "form": true, "payment": true, "settings": true, "profile": true}

// FormSubmit is the label of the submit button a form renders on this page.
func FormSubmit(pg core.Page) string {
	if a := pg.PrimaryAction; a != nil && a.Label != "" && primarySubmit[pg.Kind] {
		return a.Label
	}
	if l := kindSubmit[pg.Kind]; l != "" {
		return l
	}
	return "Simpan"
}

var apiCall = regexp.MustCompile(`^(GET|POST|PUT|PATCH|DELETE) /\S*$`)

const pageShape = `{"id": "PAGE-001", "name": "", "platform": "web|android|ios|desktop", "role": "", "feature_id": "FEAT-001",
             "sub_feature_ids": ["FEAT-001.1"], "purpose": "", "kind": "list", "density": "comfortable",
             "hierarchy": ["", ""], "secondary_actions": [""], "responsive": "",
             "route": "/janji", "access": "auth", "primary_action": {"label": "", "target": "PAGE-003"},
             "data": {"required": [""], "optional": [], "dynamic": [""], "api": ["GET /appointments"]},
             "states": {"loading": "", "empty": "", "no_results": "", "error": ""},
             "layout": {"frame": "app", "nav": "side", "width": "full",
                        "root": {"id": "main", "type": "stack", "gap": "space.md", "children": [
                          {"id": "toolbar", "type": "row", "align": "between", "children": ["title", "create"]},
                          "appointments"]},
                        "devices": {"mobile": {"nav": "bottom", "nodes": {"toolbar": {"type": "stack"}}, "swap": {"appointments": "list"}}}},
             "components": [{"id": "menu", "type": "nav", "label": "", "items": [""]},
                            {"id": "title", "type": "header", "label": ""},
                            {"id": "create", "type": "button", "label": "", "target": "PAGE-003", "emphasis": "action.primary"},
                            {"id": "appointments", "type": "table", "label": "", "columns": [""], "items": [""], "target": "PAGE-002"}]}`

const systemShape = `{"direction": "Operations: dense tables, restrained neutrals, color only for status",
   "colors": [{"name": "action.primary", "value": "#1f6feb", "use": "primary action, links, focus"}],
   "typography": [{"name": "type.body", "value": "Inter 14px/20px 400", "use": "body text"}],
   "spacing": [{"name": "space.sm", "value": "8px"}], "radius": [{"name": "radius.md", "value": "6px"}],
   "shadow": [{"name": "elevation.overlay", "value": "0 8px 24px rgba(0,0,0,.12)", "use": "modal, dropdown only"}],
   "breakpoints": [{"name": "mobile", "value": "<640px"}],
   "components": [{"name": "Button", "value": "primary | secondary | ghost | danger; 40px high, 44px touch on mobile", "use": "one primary per context"}]}`

// isRevision reports a change request on an existing design: the AI then
// returns only the pages that change.
func isRevision(p *core.Project) bool {
	d := p.Design
	return d != nil && d.Status == core.DesignChanges && len(d.Pages) > 0 && len(d.Feedback) > 0
}

// Prompt parts. The full design rules go only into a new design and into
// revisions that may create pages; a scoped revision gets the component
// format and how the wireframe renders, nothing else.
const designerRules = `Work like a senior product designer: the product must be usable, clear, consistent, responsive and accessible, not just good-looking.
Source of truth, in order: requirements, business rules, user flows, existing structure, design system. Never invent features, flows, data, APIs or business rules.
When information is missing, pick the simplest UI that does not change business logic. Simple and intentional beats complex and decorative:
no extra dashboards, analytics, filters, menus, settings or decoration to make the UI look richer.
Priority on conflict: usability, clarity, consistency, accessibility, responsiveness, maintainability, visual polish.

Screen intent, per page:
- "purpose": the primary goal in one sentence (what the user understands or finishes on this page).
- "hierarchy": what the user must see, most important first (e.g. ["Judul halaman", "Status janji hari ini", "Daftar janji", "Filter"]). Not everything is equally important. The purpose must be clear within 2-3 seconds.
- "primary_action": at most ONE primary CTA per page. "secondary_actions": the other actions, visually lower; only actions the user needs. Never the same action twice, and never two buttons that look the same but do different things.
  Destructive actions (delete, cancel, revoke) use the danger style and ask for confirmation with what will be lost.
- "responsive": how the layout adapts on tablet and mobile (e.g. "Mobile: tabel jadi daftar kartu, filter di sheet, tombol utama menempel di bawah"). Mobile is not a shrunk desktop: no horizontal overflow, touch targets at least 44px, important information stays visible.
Accessibility is part of the design: labels on every field, readable contrast, clear focus, keyboard navigation, form errors per field that say how to recover, never color alone for meaning.
Beyond the happy path: long names and long lists must not break the layout (truncate, wrap or paginate), a disabled action says why,
a page limited to a role says in "error" what access is missing, partial data shows what loaded and what did not.

Every component must answer: why does it exist, which user problem does it solve, what happens on interaction, when data is missing, when the action fails. No clear answer: remove it.
Do not make it look like a generated mockup. Never: gradient hero with three cards, glassmorphism, purple/blue gradient by default, every section in a card,
identical repeated cards, big meaningless numbers, decorative icons, badges or illustrations, charts nobody asked for, oversized headings, arbitrary animation.
Prefer strong hierarchy, restrained color, intentional whitespace and domain-specific UI.

Design system: define it once in "system" and design every page with it. Tokens only, no arbitrary values.
"direction": ONE visual direction that fits the product (enterprise, financial, developer tool, productivity, consumer, healthcare, operations, trading, creative, industrial) and what it means for the tokens; never mix styles.
Token names are semantic; the pages reference them by name and the renderer resolves them from here:
colors: text.primary, text.secondary, surface.page, surface.primary, surface.secondary, border.default, action.primary, action.danger, status.success, status.warning, status.error, status.info
(every color has a semantic role, no random colors; text contrast at least WCAG AA), typography: type.display, type.heading, type.body, type.caption (a small scale),
spacing: space.xs, space.sm, space.md, space.lg, space.xl (one scale, e.g. 4/8/16/24/32), radius: radius.sm, radius.md, radius.lg, shadow: elevation.raised, elevation.overlay (only where it has a function), breakpoints, and components with usage rules
(Button, Input, Select, Search, Table, Card, Modal, Drawer, Tabs, Badge, Tooltip, Alert, Toast, Dropdown, Navigation, EmptyState, LoadingState, ErrorState, ConfirmDialog).
Keep the vocabulary small: reuse a component before adding a new one.

`

const designRules = designerRules + `Design every page for its job, not from one template. Components may repeat across pages; layouts should not.
Per page answer: what is the main user action, what must be seen first, what is only needed on demand.

"kind" is the page's job; build its "layout" for that job, one of:
- auth: login/sign up. Brand, the form, one primary action, recovery link. No marketing, no promo cards.
- dashboard: the user's work for today (e.g. today's schedule, items waiting). Use stat only for numbers that drive decisions, at most 3. Never a page of only stats and cards.
- list: data to scan: a table with columns (or a list for short simple items), search/filter only when the data needs it, a create button.
- detail: one record: header with name and status, then sections of information. Tabs only for really different groups.
- form: grouped fields, then Cancel/Save. Simple create/edit belongs in a drawer on the list page, not its own page.
- wizard: a complex multi-step create; a "steps" component then the current step. Only when a single form is too long.
- search: search input, recent searches, results. No stats.
- settings: sections (Account, Notifications, ...) separated by "section" components, fields in each.
- payment: amount and what is paid for, payment method, total, one pay button with the amount.
- profile: identity and account fields, save.
- analytics: a headline number with change vs previous period, then only charts that answer a question.
- admin: dense operational table with filters and bulk actions.
- status: empty/error/success result page: what happened, concrete details, the next action.
"density": compact (admin, operations, ticketing queues, data-heavy lists, trading, monitoring, developer tools), comfortable (most app pages, general SaaS), spacious (auth, payment, status, consumer onboarding). Pick it per page from the domain and the job, not one density for the whole app. Whitespace shows hierarchy; do not fill space because it is there.
"states": copy for {"loading", "empty", "no_results", "error", "success"} only where the page can be in that state. A page that loads data needs loading and error; a page with a list or table needs empty too. Empty says why it is empty and what to do. no_results is for search/filter matching nothing (e.g. "Tidak ada janji yang cocok dengan filter."). Error says what failed and how to recover. Success states the concrete result. No "Oops", no "Congratulations", no emoji, no generic "No data found".

Every page is a build contract, all required:
- "route": URL path (e.g. "/janji", "/janji/:id"); unique per platform. Mobile screens use a route too.
- "access": "public" or "auth" (signed in). "role" is the role that uses the page.
- "primary_action": the one main action {"label": "Buat janji", "target": "PAGE-003"}; a read-only page gives {"reason": "Halaman baca saja"} instead of a label. Never invent an action to fill space.
- "data": {"required": [fields always shown], "optional": [], "dynamic": [values loaded or changing live], "api": ["GET /appointments"]}. api entries are "METHOD /path".
- form components need "fields": [{"label": "Email", "type": "email", "required": true, "validation": "format email", "error": "Masukkan email yang valid", "group": "Kontak"}].
  Field types: text, email, password, number, tel, date, time, datetime, select, textarea, checkbox, radio, file, currency.

Domain decides the layout, never one generic dashboard for every product. Put the domain's working objects first, e.g.
ticketing: queue, SLA, priority, assignee, status; trading: market data, order book, chart, order entry, executions; finance: balance, transactions, category, trend;
developer tool: navigation, workspace, logs, code, actions. Use the words and objects of this product's features.

Design per platform, never a shrunk copy of another:
- web: persistent sidebar nav, header, page title with its actions, then content; tables for structured data, search/filter for large data, multi-column only when useful.
- android/ios: thumb reach, short vertical flow, one primary action, progressive disclosure, touch targets at least 44px; the main nav becomes a bottom nav with at most 5 items; no sidebar, no dense tables.
- desktop: app shell with nav, workspace and a context panel; high density; keyboard shortcuts where they speed up frequent work.
Every UI feature needs at least one page. Every flow lists the pages it walks through, in order.
`

const componentRules = `Component types: header, nav, text, list, table, form, button, card, stat, input, section, steps.
Every component has an "id" unique on its page (short, readable: "title", "ticket-table"); the layout places components by id.
Visual intent is semantic tokens only, never raw values (no hex colors, no px): "emphasis" on button and card: action.primary (at most one per page: the primary action),
action.secondary (default), action.ghost, action.danger (destructive); "tone" on text, stat, card, section: text.primary, text.secondary, status.success, status.warning, status.error, status.info.
- table: "columns" are the column names; "items" are example rows with cells separated by " | " (e.g. "Budi Santoso | Dr. Andi | Aktif"). Use realistic domain data (real-looking names, IDs, dates, amounts); never "Lorem ipsum", "John Doe", "Example User" or "$1234".
- stat: label "Name: value" (e.g. "Janji hari ini: 12").
- form: "fields" as above ("group" groups fields); "items" is not needed when "fields" is given.
- section: a separator heading that groups what follows.
- steps: "items" are the step names of a wizard.
- form already renders its own buttons under the fields: submit (label = "primary_action" label on auth, form, payment, settings and profile pages; otherwise Masuk on auth, Bayar on payment, Cari on search, Lanjut on wizard, else Simpan) and Batal on form, settings and profile pages. Never add a "button" that submits or cancels the form: it would show twice.
Use "target" to link to another page id so the prototype is clickable: on button and card (open), on list and table (a row opens the detail page), on form (submit goes there).
Navigation uses the user's business words (Jadwal, Pasien, Laporan), not database tables. Name nav items exactly like the pages they open.
`

const renderRules = `Every page has an explicit "layout" (the Design IR). The renderer draws exactly the layout and never picks one itself:
- "frame": app (top bar with page name and avatar, plus the nav slot), center (one centered panel, e.g. sign-in; "brand": true shows the app name above it), full (content only).
- "nav": where the page's nav components go in the app frame: side (sidebar), rail (icon rail), top (in the top bar), bottom (tab bar), none. Nav components are never placed in the tree.
- "width": content width: narrow (sign-in, payment, status), medium (forms), wide (detail, search), full (tables, workspaces).
- "back": page id of a back link above the content (detail pages); omit when there is none.
- "root": the component tree. A node is {"id", "type", "children"} and children are component ids (strings) or nested nodes. Primitives:
  stack (vertical), row (horizontal; "align": start|center|end|between), grid ("columns": 1-4), split (exactly 2 children, "ratio": 1:1|1:2|2:1|1:3|3:1),
  sidebar (exactly 2 children, the narrow one is "side": start|end), center (centered column), full (fills the width), overlay ("placement": drawer|modal|sheet, drawn over the page).
  Node tokens: "gap" space.none|space.xs|space.sm|space.md|space.lg|space.xl, "surface" surface.primary|surface.secondary (a panel; never a surface inside a surface), "elevation" elevation.none|elevation.raised|elevation.overlay.
  Every non-nav component appears in the tree exactly once; order in the tree is the order on screen.
- "devices": what changes on tablet or mobile, explicitly: {"mobile": {"frame", "nav", "width", "hide": [ids], "nodes": {"node-id": {"type": "stack", "columns": 1, "order": [child ids]}}, "swap": {"table-id": "list"}}}.
  Mobile is not a shrunk desktop: on mobile no split or sidebar node, grids at most 2 columns, nav bottom, top or none; swap dense tables to a list where rows are long.
- Drawn by the renderer from the components themselves: a form draws its submit and cancel buttons; a table or list draws the empty state copy when there is no data; loading, error and success show the page's header and the state copy.
`

func DesignPrompt(p *core.Project) string {
	if isRevision(p) {
		return revisionPrompt(p)
	}
	var b strings.Builder
	b.WriteString(`You are the product designer of Foundry. Create the UI pages for this product as simple wireframes.
Pages MUST come from the features, sub-features and user flows below. Do not invent features. Backend-only features need no page.
Write labels in the same language as the features. Do not write code. Do not modify files.

`)
	if d := p.Design; d != nil && d.Rejected != "" {
		b.WriteString("YOUR PREVIOUS DESIGN WAS REJECTED, send a corrected design:\n" + d.Rejected + "\n\n")
	}
	b.WriteString("PLATFORMS:\n" + mustJSON(p.Platform.Targets) + "\n\n")
	b.WriteString("FEATURES:\n" + mustJSON(p.Features) + "\n\n")
	b.WriteString("FLOWS:\n" + mustJSON(p.Flows) + "\n\n")
	if p.Discovery != nil {
		b.WriteString("ROLES:\n" + mustJSON(p.Discovery.Roles) + "\n\n")
	}
	b.WriteString(designRules + "\n" + componentRules + renderRules + "\n")
	b.WriteString(`Reply with ONLY one JSON object in a ` + "```json" + ` block, shape:
{
  "system": ` + systemShape + `,
  "pages": [` + pageShape + `],
  "flows": [{"id": "FLOW-001", "page_ids": ["PAGE-001", "PAGE-002"]}]
}`)
	return b.String()
}

// ApplyDesign replaces pages with a new draft; approval is reset.
func ApplyDesign(p *core.Project, raw string) error {
	var out designOut
	if err := ExtractJSON(raw, &out); err != nil {
		return err
	}
	if isRevision(p) {
		return applyRevision(p, out)
	}
	if len(out.Pages) == 0 {
		return fmt.Errorf("rancangan tidak berisi page")
	}
	idMap := map[string]string{}
	for i, pg := range out.Pages {
		idMap[pg.ID] = fmt.Sprintf("PAGE-%03d", i+1)
	}
	var pages []core.Page
	for _, pg := range out.Pages {
		if np, ok := normPage(p, pg, idMap); ok {
			pages = append(pages, dropFormButtons(np))
		}
	}
	if len(pages) == 0 {
		return fmt.Errorf("tidak ada page yang terhubung ke feature")
	}
	core.EnsureIR(pages, p.Name)
	valid := clearDangling(pages)
	if err := core.ValidateDesign(pages, pages); err != nil {
		return err
	}
	for i := range p.Flows {
		p.Flows[i].PageIDs = nil
		for _, fl := range out.Flows {
			if fl.ID == p.Flows[i].ID {
				p.Flows[i].PageIDs = mapIDs(fl.PageIDs, idMap, valid)
			}
		}
	}
	var feedback []string
	if p.Design != nil {
		feedback = p.Design.Feedback
	}
	p.Design = &core.Design{System: normSystem(out.System), Pages: pages, Status: core.DesignDraft, Feedback: feedback}
	p.Tasks, p.Verification = nil, nil
	return nil
}

// applyRevision merges a patch into the current design: patches edit
// existing pages, NEW-* pages get the next free ids, removed pages are
// dropped. Untouched pages keep their ids and content. The previous version
// is kept so the user can undo it.
func applyRevision(p *core.Project, out designOut) error {
	d := p.Design
	system := normSystem(out.System)
	if len(out.Pages) == 0 && len(out.Removed) == 0 && len(out.Flows) == 0 && len(out.Patches) == 0 && system == nil {
		return fmt.Errorf("rancangan tidak berubah")
	}
	if system != nil && d.Scope != nil && !d.Scope.Open {
		return &PatchRejected{Unexpected: []string{"design system diubah, padahal permintaan hanya untuk halaman tertentu"}}
	}
	prevFlows := map[string][]string{}
	for _, fl := range p.Flows {
		prevFlows[fl.ID] = fl.PageIDs
	}
	pages, idMap, requested, scope, err := mergePatch(p, d.Pages, out, d.Scope)
	if err != nil {
		return err
	}
	valid := map[string]bool{}
	for _, pg := range pages {
		valid[pg.ID] = true
	}
	for i := range p.Flows {
		ids := p.Flows[i].PageIDs
		for _, fl := range out.Flows {
			if fl.ID == p.Flows[i].ID {
				ids = fl.PageIDs
			}
		}
		p.Flows[i].PageIDs = mapIDs(ids, idMap, valid)
	}
	for _, pg := range out.Pages {
		requested = append(requested, "ADD halaman "+pg.Name)
	}
	for _, id := range out.Removed {
		requested = append(requested, "REMOVE halaman "+id)
	}
	rev := &core.DesignRevision{Request: d.Feedback[len(d.Feedback)-1], PreviousPages: d.Pages, PreviousFlows: prevFlows,
		PreviousSystem: d.System, Scope: scope, Patch: requested}
	rev.Changes = DiffPages(rev.PreviousPages, pages)
	next := d.System
	if system != nil {
		next = system
		rev.Patch = append(rev.Patch, "UPDATE design system")
		rev.Changes = append(rev.Changes, core.DesignChange{Kind: "changed", Page: "Design system"})
	}
	p.Design = &core.Design{System: next, Pages: pages, Status: core.DesignDraft, Feedback: d.Feedback, Revision: rev}
	p.Tasks, p.Verification = nil, nil
	return nil
}

// mergePatch builds the next page list from the current one and a patch.
func mergePatch(p *core.Project, current []core.Page, out designOut, scope *core.ChangeScope) ([]core.Page, map[string]string, []string, *core.ChangeScope, error) {
	core.EnsureIR(current, p.Name)
	idMap := map[string]string{}
	next := 0
	for _, pg := range current {
		idMap[pg.ID] = pg.ID
		var n int
		if _, err := fmt.Sscanf(pg.ID, "PAGE-%d", &n); err == nil && n > next {
			next = n
		}
	}
	for _, id := range out.Removed {
		delete(idMap, id)
	}
	var added []core.Page
	for _, pg := range out.Pages {
		if _, ok := idMap[pg.ID]; !ok && !slices.Contains(out.Removed, pg.ID) {
			next++
			idMap[pg.ID] = fmt.Sprintf("PAGE-%03d", next)
			added = append(added, pg)
		}
	}
	changed, requested, origins, err := patchPages(p, current, out, idMap, scope)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	var pages []core.Page
	touched := map[string]bool{}
	for _, pg := range current {
		if _, ok := idMap[pg.ID]; !ok {
			continue // removed
		}
		if np, ok := changed[pg.ID]; ok {
			pg = np
			touched[pg.ID] = true
		}
		pages = append(pages, pg)
	}
	for _, pg := range added {
		if np, ok := normPage(p, pg, idMap); ok {
			pages = append(pages, dropFormButtons(np))
			touched[np.ID] = true
		}
	}
	if len(pages) == 0 {
		return nil, nil, nil, nil, fmt.Errorf("rancangan tidak boleh kosong")
	}
	core.EnsureIR(pages, p.Name)
	clearDangling(pages)
	var check []core.Page
	for _, pg := range pages {
		if touched[pg.ID] {
			check = append(check, pg)
		}
	}
	if err := core.ValidateDesign(check, pages); err != nil {
		return nil, nil, nil, nil, err
	}
	return pages, idMap, requested, remapScope(scope, origins), nil
}

// ApplyQAFix applies the extra patch from a failed visual check to the
// revision under review. Only patches are allowed, within the same scope;
// undo still restores the design from before the request.
func ApplyQAFix(p *core.Project, raw string) error {
	d := p.Design
	if d == nil || d.Revision == nil {
		return fmt.Errorf("tidak ada perubahan untuk diperbaiki")
	}
	var out designOut
	if err := ExtractJSON(raw, &out); err != nil {
		return err
	}
	return applyFix(p, out)
}

func applyFix(p *core.Project, out designOut) error {
	d := p.Design
	if len(out.Pages) > 0 || len(out.Removed) > 0 || len(out.Flows) > 0 || out.System != nil {
		return &PatchRejected{Unexpected: []string{"perbaikan visual hanya boleh berupa patch, bukan halaman/alur/design system"}}
	}
	if len(out.Patches) == 0 {
		return fmt.Errorf("perbaikan visual tanpa patch")
	}
	pages, _, requested, scope, err := mergePatch(p, d.Pages, out, d.Revision.Scope)
	if err != nil {
		return err
	}
	d.Pages = pages
	d.Revision.Scope = scope
	d.Revision.Patch = append(d.Revision.Patch, requested...)
	d.Revision.Changes = DiffPages(d.Revision.PreviousPages, pages)
	return nil
}

// normPage cleans one page from the AI; false when it traces to no feature.
func normPage(p *core.Project, pg core.Page, idMap map[string]string) (core.Page, bool) {
	f := p.Feature(pg.FeatureID)
	if f == nil {
		return core.Page{}, false // design must trace to a feature
	}
	np := core.Page{ID: idMap[pg.ID], Name: pg.Name, Platform: strings.ToLower(pg.Platform), Role: pg.Role,
		FeatureID: pg.FeatureID, Purpose: pg.Purpose, States: pg.States,
		Hierarchy: nonEmpty(pg.Hierarchy), Responsive: strings.TrimSpace(pg.Responsive),
		Route: normRoute(pg.Route), PrimaryAction: normAction(pg.PrimaryAction, idMap), Data: normData(pg.Data),
		Density: core.NormDensity(pg.Density), Layout: pg.Layout.Clone()}
	for _, a := range nonEmpty(pg.SecondaryActions) {
		dup := slices.ContainsFunc(np.SecondaryActions, func(x string) bool { return strings.EqualFold(x, a) })
		if !dup && (np.PrimaryAction == nil || !strings.EqualFold(np.PrimaryAction.Label, a)) {
			np.SecondaryActions = append(np.SecondaryActions, a)
		}
	}
	if np.ID == "" {
		return core.Page{}, false
	}
	if a := strings.ToLower(strings.TrimSpace(pg.Access)); accessLevels[a] {
		np.Access = a
	}
	if k := strings.ToLower(pg.Kind); pageKinds[k] {
		np.Kind = k
	}
	if s := np.States; s != nil && *s == (core.PageStates{}) {
		np.States = nil
	}
	for _, sid := range pg.SubFeatureIDs {
		for _, s := range f.SubFeatures {
			if s.ID == sid {
				np.SubFeatureIDs = append(np.SubFeatureIDs, sid)
			}
		}
	}
	for _, c := range pg.Components {
		c.ID = strings.TrimSpace(c.ID)
		c.Emphasis = strings.ToLower(strings.TrimSpace(c.Emphasis))
		c.Tone = strings.ToLower(strings.TrimSpace(c.Tone))
		c.Type = strings.ToLower(c.Type)
		if !componentTypes[c.Type] {
			c.Type = "text"
		}
		if c.Type != "table" {
			c.Columns = nil
		}
		if c.Type == "form" {
			c.Fields = normFields(c.Fields)
			if len(c.Fields) > 0 {
				c.Items = fieldItems(c.Fields)
			}
		} else {
			c.Fields = nil
		}
		if c.Target != "" {
			c.Target = idMap[c.Target] // unknown targets become "" (dead link removed)
		}
		np.Components = append(np.Components, c)
	}
	return np, true
}

// dropFormButtons removes buttons that repeat the submit button a form on the
// same page already renders (e.g. a second "Masuk" on a login page).
func dropFormButtons(pg core.Page) core.Page {
	if !slices.ContainsFunc(pg.Components, func(c core.PageComponent) bool { return c.Type == "form" }) {
		return pg
	}
	submit := FormSubmit(pg)
	cancel := pg.Kind == "form" || pg.Kind == "settings" || pg.Kind == "profile"
	pg.Layout = pg.Layout.Clone()
	pg.Components = slices.DeleteFunc(pg.Components, func(c core.PageComponent) bool {
		l := strings.TrimSpace(c.Label)
		drop := c.Type == "button" && (strings.EqualFold(l, submit) || cancel && strings.EqualFold(l, "Batal"))
		if drop && pg.Layout != nil && c.ID != "" {
			pg.Layout.Detach(c.ID)
		}
		return drop
	})
	return pg
}

// clearDangling clears targets to pages that no longer exist and returns the
// set of page ids.
func clearDangling(pages []core.Page) map[string]bool {
	valid := map[string]bool{}
	for _, pg := range pages {
		valid[pg.ID] = true
	}
	for i := range pages {
		if a := pages[i].PrimaryAction; a != nil && !valid[a.Target] {
			a.Target = ""
		}
		if l := pages[i].Layout; l != nil && l.Back != "" && !valid[l.Back] {
			l = l.Clone()
			l.Back = ""
			pages[i].Layout = l
		}
		for j := range pages[i].Components {
			if !valid[pages[i].Components[j].Target] {
				pages[i].Components[j].Target = ""
			}
		}
	}
	return valid
}

func mapIDs(ids []string, idMap map[string]string, valid map[string]bool) []string {
	var out []string
	for _, id := range ids {
		if m := idMap[id]; valid[m] {
			out = append(out, m)
		}
	}
	return out
}

// normSystem drops empty tokens; nil when colors, typography or spacing is
// missing, so the design is not approvable without a real system.
func normSystem(s *core.DesignSystem) *core.DesignSystem {
	if s == nil {
		return nil
	}
	out := core.DesignSystem{Direction: strings.TrimSpace(s.Direction), Colors: normTokens(s.Colors), Typography: normTokens(s.Typography), Spacing: normTokens(s.Spacing),
		Radius: normTokens(s.Radius), Shadow: normTokens(s.Shadow), Breakpoints: normTokens(s.Breakpoints), Components: normTokens(s.Components)}
	if len(out.Colors) == 0 || len(out.Typography) == 0 || len(out.Spacing) == 0 {
		return nil
	}
	return &out
}

func normTokens(in []core.Token) []core.Token {
	var out []core.Token
	for _, t := range in {
		t = core.Token{Name: strings.TrimSpace(t.Name), Value: strings.TrimSpace(t.Value), Use: strings.TrimSpace(t.Use)}
		if t.Name != "" && t.Value != "" {
			out = append(out, t)
		}
	}
	return out
}

func normRoute(r string) string {
	r = strings.ToLower(strings.Join(strings.Fields(r), "-"))
	if r != "" && !strings.HasPrefix(r, "/") {
		r = "/" + r
	}
	return r
}

func normAction(a *core.PageAction, idMap map[string]string) *core.PageAction {
	if a == nil {
		return nil
	}
	out := core.PageAction{Label: strings.TrimSpace(a.Label), Reason: strings.TrimSpace(a.Reason)}
	if a.Target != "" {
		out.Target = idMap[a.Target]
	}
	if out.Label == "" && out.Reason == "" {
		return nil
	}
	if out.Label == "" {
		out.Target = ""
	}
	return &out
}

func normData(d *core.PageData) *core.PageData {
	if d == nil {
		return nil
	}
	out := core.PageData{Required: nonEmpty(d.Required), Optional: nonEmpty(d.Optional), Dynamic: nonEmpty(d.Dynamic)}
	for _, a := range d.API {
		f := strings.Fields(a)
		if len(f) == 2 {
			if c := strings.ToUpper(f[0]) + " " + f[1]; apiCall.MatchString(c) {
				out.API = append(out.API, c)
			}
		}
	}
	if len(out.Required)+len(out.Optional)+len(out.Dynamic)+len(out.API) == 0 {
		return nil
	}
	return &out
}

func nonEmpty(in []string) []string {
	var out []string
	for _, s := range in {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func normFields(in []core.FormField) []core.FormField {
	var out []core.FormField
	for _, f := range in {
		f.Label = strings.TrimSpace(f.Label)
		if f.Label == "" {
			continue
		}
		f.Type = strings.ToLower(strings.TrimSpace(f.Type))
		if !fieldTypes[f.Type] {
			f.Type = "text"
		}
		f.Group = strings.TrimSpace(f.Group)
		out = append(out, f)
	}
	return out
}

// fieldItems rebuilds the wireframe items ("# Group" then labels) from the
// field spec so both never disagree.
func fieldItems(fields []core.FormField) []string {
	var items []string
	group := ""
	for _, f := range fields {
		if f.Group != group {
			group = f.Group
			if group != "" {
				items = append(items, "# "+group)
			}
		}
		items = append(items, f.Label)
	}
	return items
}

// KeepRevision accepts the last change request.
func KeepRevision(p *core.Project) error {
	if p.Design == nil || p.Design.Revision == nil {
		return fmt.Errorf("tidak ada perubahan untuk ditinjau")
	}
	p.Design.Revision = nil
	return nil
}

// UndoRevision restores the design from before the last change request and
// drops that request so later revisions do not apply it again.
func UndoRevision(p *core.Project) error {
	if p.Design == nil || p.Design.Revision == nil {
		return fmt.Errorf("tidak ada perubahan untuk dibatalkan")
	}
	rev := p.Design.Revision
	p.Design.Pages = rev.PreviousPages
	p.Design.System = rev.PreviousSystem
	for i := range p.Flows {
		p.Flows[i].PageIDs = rev.PreviousFlows[p.Flows[i].ID]
	}
	if n := len(p.Design.Feedback); n > 0 {
		p.Design.Feedback = p.Design.Feedback[:n-1]
	}
	p.Design.Status = core.DesignDraft
	p.Design.Revision = nil
	p.Design.Screens = nil
	return nil
}

// UIFeatures returns features that must have at least one page.
func UIFeatures(p *core.Project) []core.Feature {
	var out []core.Feature
	for _, f := range p.Features {
		for _, pl := range f.Platforms {
			if pl != core.PlatformBackend {
				out = append(out, f)
				break
			}
		}
	}
	return out
}
