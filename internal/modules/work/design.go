package work

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/handrawanw/foundry/internal/core"
)

type designOut struct {
	Pages   []core.Page `json:"pages"`
	Removed []string    `json:"removed"` // revision only: ids of deleted pages
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

var densities = map[string]bool{"low": true, "medium": true, "high": true}

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
             "sub_feature_ids": ["FEAT-001.1"], "purpose": "", "kind": "list", "density": "medium",
             "route": "/janji", "access": "auth", "primary_action": {"label": "", "target": "PAGE-003"},
             "data": {"required": [""], "optional": [], "dynamic": [""], "api": ["GET /appointments"]},
             "states": {"loading": "", "empty": "", "no_results": "", "error": ""},
             "components": [{"type": "header", "label": ""}, {"type": "table", "label": "", "columns": [""], "items": [""], "target": "PAGE-002"},
                            {"type": "button", "label": "", "target": "PAGE-003"},
                            {"type": "form", "label": "", "fields": [{"label": "", "type": "text", "required": true, "validation": "", "error": "", "group": ""}]}]}`

// isRevision reports a change request on an existing design: the AI then
// returns only the pages that change.
func isRevision(p *core.Project) bool {
	d := p.Design
	return d != nil && d.Status == core.DesignChanges && len(d.Pages) > 0 && len(d.Feedback) > 0
}

func DesignPrompt(p *core.Project) string {
	var b strings.Builder
	b.WriteString(`You are the product designer of Foundry. Create the UI pages for this product as simple wireframes.
Pages MUST come from the features, sub-features and user flows below. Do not invent features. Backend-only features need no page.
Write labels in the same language as the features. Do not write code. Do not modify files.

`)
	b.WriteString("PLATFORMS:\n" + mustJSON(p.Platform.Targets) + "\n\n")
	b.WriteString("FEATURES:\n" + mustJSON(p.Features) + "\n\n")
	b.WriteString("FLOWS:\n" + mustJSON(p.Flows) + "\n\n")
	if p.Discovery != nil {
		b.WriteString("ROLES:\n" + mustJSON(p.Discovery.Roles) + "\n\n")
	}
	revise := isRevision(p)
	if revise {
		fb := p.Design.Feedback
		b.WriteString("CURRENT DESIGN:\n" + mustJSON(p.Design.Pages) + "\n\n")
		if s := screenText(p.Design); s != "" {
			b.WriteString("WHAT THE USER SEES NOW (read from the rendered wireframe; [n] type is components[n] of that page, (otomatis) is drawn by the wireframe, not a component). " +
				"The change request describes this screen; map what it mentions to the components:\n" + s + "\n")
		}
		if len(fb) > 1 {
			b.WriteString("EARLIER REQUESTS (already applied in the current design, do not redo):\n- " + strings.Join(fb[:len(fb)-1], "\n- ") + "\n\n")
		}
		b.WriteString("CHANGE REQUEST (apply now):\n" + fb[len(fb)-1] + "\n\n")
	}
	b.WriteString(`Design every page for its job, not from one template. Components may repeat across pages; layouts should not.
Per page answer: what is the main user action, what must be seen first, what is only needed on demand.

"kind" picks the layout, one of:
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
"density": low (auth, payment, status), medium (most app pages), high (admin, data-heavy lists).
"states": copy for {"loading", "empty", "no_results", "error", "success"} only where the page can be in that state. Empty says why it is empty and what to do. no_results is for search/filter matching nothing (e.g. "Tidak ada janji yang cocok dengan filter."). Error says what failed and how to recover. Success states the concrete result. No "Oops", no "Congratulations", no emoji, no generic "No data found".

Every page is a build contract, all required:
- "route": URL path (e.g. "/janji", "/janji/:id"); unique per platform. Mobile screens use a route too.
- "access": "public" or "auth" (signed in). "role" is the role that uses the page.
- "primary_action": the one main action {"label": "Buat janji", "target": "PAGE-003"}; a read-only page gives {"reason": "Halaman baca saja"} instead of a label. Never invent an action to fill space.
- "data": {"required": [fields always shown], "optional": [], "dynamic": [values loaded or changing live], "api": ["GET /appointments"]}. api entries are "METHOD /path".
- form components need "fields": [{"label": "Email", "type": "email", "required": true, "validation": "format email", "error": "Masukkan email yang valid", "group": "Kontak"}].
  Field types: text, email, password, number, tel, date, time, datetime, select, textarea, checkbox, radio, file, currency.

Component types: header, nav, text, list, table, form, button, card, stat, input, section, steps.
- table: "columns" are the column names; "items" are example rows with cells separated by " | " (e.g. "Budi Santoso | Dr. Andi | Aktif"). Use realistic example data.
- stat: label "Name: value" (e.g. "Janji hari ini: 12").
- form: "fields" as above ("group" groups fields); "items" is not needed when "fields" is given.
- section: a separator heading that groups what follows.
- steps: "items" are the step names of a wizard.
- form already renders its own buttons under the fields: submit (label = "primary_action" label on auth, form, payment, settings and profile pages; otherwise Masuk on auth, Bayar on payment, Cari on search, Lanjut on wizard, else Simpan) and Batal on form, settings and profile pages. Never add a "button" that submits or cancels the form: it would show twice.
Use "target" to link to another page id so the prototype is clickable: on button and card (open), on list and table (a row opens the detail page), on form (submit goes there).
Navigation uses the user's business words (Jadwal, Pasien, Laporan), not database tables. Name nav items exactly like the pages they open.
How the wireframe renders a page (so you know what the user sees):
- Components show top to bottom in array order; to move something, change its position in "components".
- "nav" goes to the sidebar (desktop/tablet) or the bottom tab bar (mobile), not in the content.
- list/admin pages on desktop: header and buttons share one toolbar row above the data.
- consecutive "stat" components sit on one row.
- Added automatically, never as components: the top bar with app name and avatar, the app brand above the auth card (hidden when a header has the app name), a back link on detail pages, the form buttons above.
Mobile pages are designed for mobile: short content, the main nav becomes a bottom nav with at most 5 items.
Every UI feature needs at least one page. Every flow lists the pages it walks through, in order.

` + "\n\n")
	if revise {
		b.WriteString(`This is a change to the current design. Return ONLY what changes:
- "pages": pages that change (keep their id, send the whole page) and new pages (id "NEW-1", "NEW-2", ...). Pages you do not send stay exactly as they are.
- "removed": ids of pages to delete.
- "flows": only flows whose page list changes, with the full new list.
Every returned page follows all the rules above. Keep targets to unchanged pages as they are.
Change only what the request asks. In a changed page keep every other component, its order, labels and data exactly as they are.
When the request talks about something visible (a duplicate button, a misplaced block), find which component or automatic element above produces it and change the JSON so the screen matches the request.

Reply with ONLY one JSON object in a ` + "```json" + ` block, shape:
{"pages": [` + pageShape + `], "removed": ["PAGE-009"], "flows": [{"id": "FLOW-001", "page_ids": ["PAGE-001", "NEW-1"]}]}`)
		return b.String()
	}
	b.WriteString(`Reply with ONLY one JSON object in a ` + "```json" + ` block, shape:
{
  "pages": [` + pageShape + `],
  "flows": [{"id": "FLOW-001", "page_ids": ["PAGE-001", "PAGE-002"]}]
}`)
	return b.String()
}

// screenText lists the user's screens in page order.
func screenText(d *core.Design) string {
	var b strings.Builder
	for _, pg := range d.Pages {
		if s := strings.TrimSpace(d.Screens[pg.ID]); s != "" {
			b.WriteString(pg.ID + " \"" + pg.Name + "\"\n" + s + "\n\n")
		}
	}
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
			pages = append(pages, np)
		}
	}
	if len(pages) == 0 {
		return fmt.Errorf("tidak ada page yang terhubung ke feature")
	}
	valid := clearDangling(pages)
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
	p.Design = &core.Design{Pages: pages, Status: core.DesignDraft, Feedback: feedback}
	p.Tasks, p.Verification = nil, nil
	return nil
}

// applyRevision merges a change set into the current design: returned pages
// replace pages with the same id, NEW-* pages get the next free ids, removed
// pages are dropped. Untouched pages keep their ids and content. The previous
// version is kept so the user can undo it.
func applyRevision(p *core.Project, out designOut) error {
	d := p.Design
	if len(out.Pages) == 0 && len(out.Removed) == 0 && len(out.Flows) == 0 {
		return fmt.Errorf("rancangan tidak berubah")
	}
	prevFlows := map[string][]string{}
	for _, fl := range p.Flows {
		prevFlows[fl.ID] = fl.PageIDs
	}
	rev := &core.DesignRevision{Request: d.Feedback[len(d.Feedback)-1], PreviousPages: d.Pages, PreviousFlows: prevFlows}

	idMap := map[string]string{}
	next := 0
	for _, pg := range d.Pages {
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
	changed := map[string]core.Page{}
	for _, pg := range out.Pages {
		if np, ok := normPage(p, pg, idMap); ok {
			changed[np.ID] = np
		}
	}
	var pages []core.Page
	for _, pg := range d.Pages {
		if _, ok := idMap[pg.ID]; !ok {
			continue // removed
		}
		if np, ok := changed[pg.ID]; ok {
			pg = np
		}
		pages = append(pages, pg)
	}
	for _, pg := range added {
		if np, ok := changed[idMap[pg.ID]]; ok {
			pages = append(pages, np)
		}
	}
	if len(pages) == 0 {
		return fmt.Errorf("rancangan tidak boleh kosong")
	}
	valid := clearDangling(pages)
	for i := range p.Flows {
		ids := p.Flows[i].PageIDs
		for _, fl := range out.Flows {
			if fl.ID == p.Flows[i].ID {
				ids = fl.PageIDs
			}
		}
		p.Flows[i].PageIDs = mapIDs(ids, idMap, valid)
	}
	rev.Changes = DiffPages(rev.PreviousPages, pages)
	p.Design = &core.Design{Pages: pages, Status: core.DesignDraft, Feedback: d.Feedback, Revision: rev}
	p.Tasks, p.Verification = nil, nil
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
		Route: normRoute(pg.Route), PrimaryAction: normAction(pg.PrimaryAction, idMap), Data: normData(pg.Data)}
	if np.ID == "" {
		return core.Page{}, false
	}
	if a := strings.ToLower(strings.TrimSpace(pg.Access)); accessLevels[a] {
		np.Access = a
	}
	if k := strings.ToLower(pg.Kind); pageKinds[k] {
		np.Kind = k
	}
	if d := strings.ToLower(pg.Density); densities[d] {
		np.Density = d
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
	return dropFormButtons(np), true
}

// dropFormButtons removes buttons that repeat the submit button a form on the
// same page already renders (e.g. a second "Masuk" on a login page).
func dropFormButtons(pg core.Page) core.Page {
	if !slices.ContainsFunc(pg.Components, func(c core.PageComponent) bool { return c.Type == "form" }) {
		return pg
	}
	submit := FormSubmit(pg)
	cancel := pg.Kind == "form" || pg.Kind == "settings" || pg.Kind == "profile"
	pg.Components = slices.DeleteFunc(pg.Components, func(c core.PageComponent) bool {
		l := strings.TrimSpace(c.Label)
		return c.Type == "button" && (strings.EqualFold(l, submit) || cancel && strings.EqualFold(l, "Batal"))
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
