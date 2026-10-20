package work

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/handrawanw/foundry/internal/core"
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
}

const designShape = `{
 "rules": {"source": "new|existing", "direction": "", "principles": [""], "ux_guidelines": [""],
   "tokens": {"colors": [{"name": "action.primary", "value": "#1f6feb", "use": ""}], "typography": [{"name": "type.body", "value": "Inter 14px/20px 400", "use": ""}],
              "spacing": [{"name": "space.md", "value": "16px", "use": ""}], "radius": [], "shadow": [], "breakpoints": [{"name": "breakpoint.tablet", "value": "768px", "use": ""}]},
   "components": [{"name": "Button", "use": "", "variants": ["primary", "secondary"], "rules": ["hover/focus/active/disabled: ..."], "source": ""}],
   "visual_hierarchy": [""], "accessibility": [""], "responsive": [""], "interaction": [""],
   "platforms": [{"platform": "android", "rules": [""]}]},
 "shells": [{"id": "app", "name": "", "platform": "web", "pattern": "sidebar", "navigation": "", "grid": "", "responsive": [""],
   "regions": [{"id": "nav", "name": "", "content": [""], "components": ["Sidebar"], "sizing": "", "responsive": ""}]}],
 "pages": [{"id": "antrean", "name": "", "platform": "web", "route": "/antrean", "role": "", "access": "auth", "feature_id": "FEAT-001",
   "purpose": "", "shell": "app", "pattern": "table", "primary_action": "",
   "sections": [{"id": "filter", "name": "", "pattern": "", "purpose": "", "content": [""], "components": [""],
                 "grid": "", "alignment": "", "spacing": "space.md", "sizing": "", "responsive": "", "states": {}}],
   "hierarchy": ["filter"], "grid": "", "responsive": [""], "links": [{"to": "detail-antrean", "via": ""}],
   "states": {"loading": "", "empty": "", "error": ""}, "data": [""]}],
 "flows": {"FLOW-001": ["antrean", "detail-antrean"]}
}`

const designRules = `DESIGN RULES (rules)
- source: "new", or "existing" when the product already has a frontend: its tokens and components ARE the design system; keep them and add only what is missing.
- principles: 3-7 principles for THIS product and its users, each one actionable and checkable ("operators compare many rows: dense tables, not cards"). Never generic ("modern and clean").
- ux_guidelines: concrete rules for navigation, forms and validation, feedback, errors, empty states, confirmation of destructive actions.
- tokens: colors, typography and spacing are required; radius, shadow, breakpoints when used. Semantic names (action.primary, text.secondary, surface.page, status.error, type.body, space.md, radius.md, breakpoint.tablet), concrete values (hex/rgb, px/rem, font family size/line-height weight), use = when to use it.
- components: every shared component the pages reuse: name, use, variants, rules (including hover, focus, active and disabled states). An existing component keeps its name and gets its file in source.
- visual_hierarchy: how importance is shown (size, weight, color, position). accessibility: WCAG 2.2 AA (contrast, focus, keyboard, labels, touch targets). responsive: the breakpoints and what changes at each. interaction: feedback, loading behavior, motion.
- platforms: rules that apply to one platform only (web, android, ios, desktop), e.g. Material 3 on android, Human Interface Guidelines on ios.
`

const layoutRules = `LAYOUT SPECIFICATION (shells, pages)
- Text and structured data only. No images, mockups, HTML, CSS or code.
- shells: the layouts pages share (app frame with sidebar or navbar, auth frame, ...): id, name, platform, pattern, regions (sections), navigation, grid, responsive.
- pages: every screen the features need, per platform with screens. route: web starts with /, mobile/desktop is the screen name. access: public or auth. shell: a shell id, or empty for a page with its own frame.
- pattern (page, shell and optional for a section) is one of: %s.
- sections: in visual order (top to bottom, start to end). content lists the elements in order, concretely ("Tabel antrean: No, Pasien, Dokter, Status, Aksi"). components are names from rules.components. grid, alignment, spacing, sizing and responsive use the tokens (space.md, breakpoint.tablet), never arbitrary values.
- hierarchy: section ids from the most important.
- states (page or section): keys %s; the value says what the user sees and can do. Required per page pattern: %s.
- links: {to: page id, via: what the user activates}; together they are the navigation between pages.
- flows: for each flow id, the page ids it passes through, in order.
- ids: lowercase letters, digits and dashes. Page ids are unique; section ids are unique within their page.
`

// DesignPrompt asks for the design rules and layout spec, or for a
// revision of them (the pending request).
func DesignPrompt(p *core.Project) string {
	var b strings.Builder
	b.WriteString(`You are the UI/UX designer of Foundry. Foundry designs software and writes instructions for an EXTERNAL coding agent; it never implements, renders or draws. Write the UI design as text and structured data: design rules and a layout specification. Do not write code. Do not modify files.
Write every human-readable text in the same language as the story.

PROJECT: ` + p.Name + "\nSTORY:\n" + p.Story + "\n\n")
	b.WriteString("PLATFORMS WITH SCREENS: " + strings.Join(p.UIPlatforms(), ", ") + "\n")
	if d := p.Discovery; d != nil && len(d.Roles) > 0 {
		b.WriteString("ROLES:\n")
		for _, r := range d.Roles {
			fmt.Fprintf(&b, "- %s (%s): %s\n", r.Name, r.Platform, r.Description)
		}
	}
	b.WriteString("FEATURES:\n")
	for _, f := range p.Features {
		var subs []string
		for _, s := range f.SubFeatures {
			subs = append(subs, s.Name)
		}
		fmt.Fprintf(&b, "- %s %s [%s]: %s. Sub: %s\n", f.ID, f.Name, strings.Join(f.Platforms, ", "), f.Description, strings.Join(subs, "; "))
	}
	if len(p.Flows) > 0 {
		b.WriteString("FLOWS:\n")
		for _, fl := range p.Flows {
			fmt.Fprintf(&b, "- %s %s (%s, %s): %s\n", fl.ID, fl.Name, fl.Role, fl.FeatureID, strings.Join(fl.Steps, " → "))
		}
	}
	b.WriteString("\n" + p.Stack.Brief())
	if brief := existingUIBrief(p); brief != "" {
		b.WriteString("\n" + brief)
	}
	b.WriteString("\n")
	d := p.UI
	var rejected string
	if d != nil && d.Request != nil {
		rejected = d.Request.Rejected
	}
	b.WriteString(designRules + "\n")
	fmt.Fprintf(&b, layoutRules+"\n", strings.Join(core.LayoutPatterns, ", "), strings.Join(core.UIStates, ", "), requiredStates())
	if !d.Written() {
		if rejected != "" {
			b.WriteString("YOUR LAST ANSWER WAS REFUSED, fix this: " + rejected + "\n\n")
		}
		b.WriteString("Write the design rules and the layout specification of every page.\n\nReply with ONLY one JSON object in a ```json block, shape:\n" + designShape)
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
- change only what the request affects. Return rules only when they change, then in full. Return only the shells and pages you add or change, each in full; keep their ids.
- remove_pages and remove_shells list the ids to delete; links and flows to removed pages are dropped.
- return flows only for flows whose pages change.
- what is not returned stays as it is.

Reply with ONLY one JSON object in a ` + "```json" + ` block, shape (every key optional):
` + `{"rules": {...}, "shells": [...], "pages": [...], "remove_shells": ["id"], "remove_pages": ["id"], "flows": {"FLOW-001": ["page-id"]}}`)
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
	case !fresh && out.Rules == nil && len(out.Shells)+len(out.Pages)+len(out.RemoveShells)+len(out.RemovePages)+len(out.Flows) == 0:
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
		return &core.DesignInvalid{Issues: errs}
	}
	next.Request = nil
	next.Version++
	next.UpdatedAt = time.Now().UTC()
	p.UI, p.Flows = next, flows
	return nil
}

// dropPages removes links and flow steps to pages that are gone.
func dropPages(l *core.LayoutSpec, flows []core.Flow, gone map[string]bool) {
	if len(gone) == 0 {
		return
	}
	for i := range l.Pages {
		l.Pages[i].Links = slices.DeleteFunc(l.Pages[i].Links, func(ln core.PageLink) bool { return gone[ln.To] })
	}
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

func normRules(r core.DesignRules) *core.DesignRules {
	r.Source = normID(r.Source)
	if r.Source == "" {
		r.Source = core.RulesNew
	}
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
	for _, f := range []*string{&pg.Name, &pg.Route, &pg.Role, &pg.Purpose, &pg.PrimaryAction} {
		*f = strings.TrimSpace(*f)
	}
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
