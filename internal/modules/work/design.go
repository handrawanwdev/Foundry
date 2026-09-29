package work

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/handrawanw/foundry/internal/core"
)

type designOut struct {
	Pages []core.Page `json:"pages"`
	Flows []struct {
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

var apiCall = regexp.MustCompile(`^(GET|POST|PUT|PATCH|DELETE) /\S*$`)

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
	if p.Design != nil && len(p.Design.Feedback) > 0 {
		b.WriteString("PREVIOUS DESIGN:\n" + mustJSON(p.Design.Pages) + "\n\n")
		b.WriteString("USER REQUESTED CHANGES (apply all):\n- " + strings.Join(p.Design.Feedback, "\n- ") + "\n\n")
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
Use "target" to link to another page id so the prototype is clickable: on button and card (open), on list and table (a row opens the detail page), on form (submit goes there).
Navigation uses the user's business words (Jadwal, Pasien, Laporan), not database tables. Name nav items exactly like the pages they open.
Mobile pages are designed for mobile: short content, the main nav becomes a bottom nav with at most 5 items.
Every UI feature needs at least one page. Every flow lists the pages it walks through, in order.

Reply with ONLY one JSON object in a ` + "```json" + ` block, shape:
{
  "pages": [{"id": "PAGE-001", "name": "", "platform": "web|android|ios|desktop", "role": "", "feature_id": "FEAT-001",
             "sub_feature_ids": ["FEAT-001.1"], "purpose": "", "kind": "list", "density": "medium",
             "route": "/janji", "access": "auth", "primary_action": {"label": "", "target": "PAGE-003"},
             "data": {"required": [""], "optional": [], "dynamic": [""], "api": ["GET /appointments"]},
             "states": {"loading": "", "empty": "", "no_results": "", "error": ""},
             "components": [{"type": "header", "label": ""}, {"type": "table", "label": "", "columns": [""], "items": [""], "target": "PAGE-002"},
                            {"type": "button", "label": "", "target": "PAGE-003"},
                            {"type": "form", "label": "", "fields": [{"label": "", "type": "text", "required": true, "validation": "", "error": "", "group": ""}]}]}],
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
	if len(out.Pages) == 0 {
		return fmt.Errorf("rancangan tidak berisi page")
	}
	idMap := map[string]string{}
	for i, pg := range out.Pages {
		idMap[pg.ID] = fmt.Sprintf("PAGE-%03d", i+1)
	}
	var pages []core.Page
	for _, pg := range out.Pages {
		if p.Feature(pg.FeatureID) == nil {
			continue // design must trace to a feature
		}
		np := core.Page{ID: idMap[pg.ID], Name: pg.Name, Platform: strings.ToLower(pg.Platform), Role: pg.Role,
			FeatureID: pg.FeatureID, Purpose: pg.Purpose, States: pg.States,
			Route: normRoute(pg.Route), PrimaryAction: normAction(pg.PrimaryAction, idMap), Data: normData(pg.Data)}
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
		f := p.Feature(pg.FeatureID)
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
		pages = append(pages, np)
	}
	if len(pages) == 0 {
		return fmt.Errorf("tidak ada page yang terhubung ke feature")
	}
	// A revision (change request on an existing design) keeps the previous
	// version so the user can undo it.
	var rev *core.DesignRevision
	if d := p.Design; d != nil && d.Status == core.DesignChanges && len(d.Pages) > 0 && len(d.Feedback) > 0 {
		prevFlows := map[string][]string{}
		for _, fl := range p.Flows {
			prevFlows[fl.ID] = fl.PageIDs
		}
		rev = &core.DesignRevision{Request: d.Feedback[len(d.Feedback)-1], PreviousPages: d.Pages, PreviousFlows: prevFlows}
	}
	// Pages that were dropped leave dangling targets; clear them.
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
	for i := range p.Flows {
		p.Flows[i].PageIDs = nil
		for _, fl := range out.Flows {
			if fl.ID != p.Flows[i].ID {
				continue
			}
			for _, id := range fl.PageIDs {
				if m := idMap[id]; valid[m] {
					p.Flows[i].PageIDs = append(p.Flows[i].PageIDs, m)
				}
			}
		}
	}
	var feedback []string
	if p.Design != nil {
		feedback = p.Design.Feedback
	}
	if rev != nil {
		rev.Changes = DiffPages(rev.PreviousPages, pages)
	}
	p.Design = &core.Design{Pages: pages, Status: core.DesignDraft, Feedback: feedback, Revision: rev}
	p.Tasks, p.Verification = nil, nil
	return nil
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
