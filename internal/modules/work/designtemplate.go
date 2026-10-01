package work

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/handrawanw/foundry/internal/core"
)

// Template layer: a design template is a proven UX pattern (how a kind of
// product screen is structured), never markup or a screen to copy. Foundry
// recommends one from the requirement, the designer chooses and adapts it
// to the product's features and composes the Design IR; the theme still
// decides the look, the renderer still draws only the IR. Each template is
// a directory: intent.json, layout.json, rules.json, examples.json.

//go:embed design-templates
var builtinTemplates embed.FS

// minTemplateScore is the lowest match score a recommendation needs.
const minTemplateScore = 3

var loadTemplates = sync.OnceValues(func() ([]core.DesignTemplate, []ThemeProblem) {
	sub, _ := fs.Sub(builtinTemplates, "design-templates")
	entries, _ := fs.ReadDir(sub, ".")
	var out []core.DesignTemplate
	var bad []ThemeProblem
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		t, err := LoadDesignTemplate(sub, e.Name())
		if err != nil {
			bad = append(bad, ThemeProblem{ID: e.Name(), Error: err.Error()})
			continue
		}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, bad
})

// DesignTemplates is the template library, sorted by id, and the
// templates that failed to load.
func DesignTemplates() ([]core.DesignTemplate, []ThemeProblem) {
	return loadTemplates()
}

// FindDesignTemplate returns a template of the library, nil when unknown.
func FindDesignTemplate(id string) *core.DesignTemplate {
	all, _ := DesignTemplates()
	for i := range all {
		if all[i].ID == id {
			return &all[i]
		}
	}
	return nil
}

// LoadDesignTemplate reads one template directory and validates it.
func LoadDesignTemplate(fsys fs.FS, dir string) (core.DesignTemplate, error) {
	var t core.DesignTemplate
	for _, f := range []struct {
		name     string
		required bool
	}{{"intent.json", true}, {"layout.json", true}, {"rules.json", true}, {"examples.json", false}} {
		b, err := fs.ReadFile(fsys, dir+"/"+f.name)
		if errors.Is(err, fs.ErrNotExist) && !f.required {
			continue
		}
		if err != nil {
			return t, fmt.Errorf("%s tidak bisa dibaca", f.name)
		}
		if err := json.Unmarshal(b, &t); err != nil {
			return t, fmt.Errorf("%s: %w", f.name, err)
		}
	}
	if t.ID != dir {
		return t, fmt.Errorf("id %q harus sama dengan nama folder %q", t.ID, dir)
	}
	return t, ValidateDesignTemplate(t)
}

var regionRoles = []string{"primary", "secondary", "supporting", "action", "navigation"}
var regionAreas = []string{"top", "side", "main", "aside"}

// ValidateDesignTemplate checks a template against Foundry's vocabulary:
// layout and responsive strategies, page kinds, component types, domains
// and the anti-patterns the quality check knows.
func ValidateDesignTemplate(t core.DesignTemplate) error {
	var out []string
	bad := func(format string, a ...any) { out = append(out, fmt.Sprintf(format, a...)) }
	oneOf := func(what, v string, set []string) {
		if v != "" && !slices.Contains(set, v) {
			bad("%s %q tidak dikenal (pakai: %s)", what, v, strings.Join(set, ", "))
		}
	}
	if strings.TrimSpace(t.Name) == "" || strings.TrimSpace(t.Intent) == "" {
		bad("name dan intent wajib diisi")
	}
	fit := t.SuitableFor
	for _, d := range fit.Domains {
		oneOf("suitable_for.domains", d, Domains)
	}
	for _, k := range fit.Tasks {
		oneOf("suitable_for.tasks", k, Tasks)
	}
	oneOf("suitable_for.frequency", fit.Frequency, Levels)
	oneOf("suitable_for.density", fit.Density, Levels)
	if len(fit.PageKinds) == 0 {
		bad("suitable_for.page_kinds kosong")
	}
	for _, k := range fit.PageKinds {
		if !pageKinds[k] {
			bad("page kind %q tidak dikenal", k)
		}
	}
	l := t.Layout
	if len(l.Strategies) == 0 {
		bad("layout_pattern.strategies kosong")
	}
	for _, s := range l.Strategies {
		oneOf("layout_pattern.strategies", s, LayoutStrategies)
	}
	oneOf("layout_pattern.navigation", l.Navigation, core.NavPlacements)
	oneOf("layout_pattern.density", l.Density, core.Densities)
	oneOf("responsive_strategy.strategy", t.Responsive.Strategy, ResponsiveStrategies)
	primary := 0
	seen := map[string]bool{}
	for _, r := range t.Composition {
		if r.Region == "" || seen[r.Region] {
			bad("region %q kosong atau dipakai lebih dari sekali", r.Region)
		}
		seen[r.Region] = true
		if !componentTypes[r.Type] {
			bad("region %q: tipe komponen %q tidak dikenal", r.Region, r.Type)
		}
		oneOf("region "+r.Region+" role", r.Role, regionRoles)
		oneOf("region "+r.Region+" area", r.Area, regionAreas)
		if r.Role == "primary" {
			primary++
		}
	}
	if primary != 1 {
		bad("component_composition butuh tepat satu region primary, ada %d", primary)
	}
	for _, a := range t.Quality.Avoid {
		oneOf("quality_rules.avoid", a, core.TemplateAvoid)
	}
	for _, ex := range t.Examples {
		oneOf("examples.domain", ex.Domain, Domains)
		for r := range ex.Regions {
			if !seen[r] {
				bad("contoh %q: region %q tidak ada", ex.Product, r)
			}
		}
	}
	if len(out) > 0 {
		return fmt.Errorf("template %s tidak valid:\n- %s", t.ID, strings.Join(out, "\n- "))
	}
	return nil
}

// ---------- Selection ----------

// TemplateInput is what template selection reads: the domain, the user
// roles and the product text (story, discovery, features).
type TemplateInput struct {
	Domain string
	Roles  []string
	Text   string
}

// TemplateMatch is how well one template fits, with the reasons.
type TemplateMatch struct {
	ID      string   `json:"id"`
	Score   int      `json:"score"`
	Reasons []string `json:"reasons"`
}

func templateInput(p *core.Project) TemplateInput {
	in := TemplateInput{Domain: DetectDomain(p), Text: productText(p)}
	if p.Design != nil && p.Design.Domain != "" {
		in.Domain = p.Design.Domain
	}
	if d := p.Discovery; d != nil {
		for _, r := range d.Roles {
			in.Roles = append(in.Roles, r.Name+" "+r.Description)
		}
	}
	return in
}

func wordHits(words []string, text string) []string {
	var out []string
	for _, w := range words {
		if regexp.MustCompile(`(?i)(^|[^\pL\pN])` + regexp.QuoteMeta(w) + `($|[^\pL\pN])`).MatchString(text) {
			out = append(out, w)
		}
	}
	return out
}

// RankTemplates scores every template against the input, best first.
// Signals: the domain (its home domain counts most), the user roles, the
// words of the requirement and, for dense domains, a template made for
// high frequency, high density work. Templates without a signal are left
// out; ties keep the library order.
func RankTemplates(tpls []core.DesignTemplate, in TemplateInput) []TemplateMatch {
	var out []TemplateMatch
	roles := strings.Join(in.Roles, " ")
	for _, t := range tpls {
		m := TemplateMatch{ID: t.ID}
		fit := t.SuitableFor
		if i := slices.Index(fit.Domains, in.Domain); in.Domain != "" && i >= 0 {
			m.Score += 5
			if i > 0 {
				m.Score -= 2
			}
			m.Reasons = append(m.Reasons, "domain "+in.Domain)
		}
		if hits := wordHits(fit.Roles, roles); len(hits) > 0 {
			m.Score += 2
			m.Reasons = append(m.Reasons, "peran: "+strings.Join(hits, ", "))
		}
		if hits := wordHits(fit.Keywords, in.Text); len(hits) > 0 {
			m.Score += min(len(hits), 4)
			m.Reasons = append(m.Reasons, "kebutuhan menyebut: "+strings.Join(hits, ", "))
		}
		if m.Score > 0 && denseDomains[in.Domain] && fit.Frequency == "high" && fit.Density == "high" {
			m.Score++
			m.Reasons = append(m.Reasons, "pekerjaan sering dan padat informasi")
		}
		if m.Score > 0 {
			out = append(out, m)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	return out
}

// RecommendTemplate is Foundry's pick for the project: the best match as
// the primary template and the next one that structures other pages as
// the secondary pattern. Nil when no template fits.
func RecommendTemplate(p *core.Project) *core.TemplateChoice {
	tpls, _ := DesignTemplates()
	ranked := RankTemplates(tpls, templateInput(p))
	if len(ranked) == 0 || ranked[0].Score < minTemplateScore {
		return nil
	}
	best := FindDesignTemplate(ranked[0].ID)
	c := &core.TemplateChoice{ID: best.ID, Name: best.Name, Source: "auto", Reasons: ranked[0].Reasons}
	for _, m := range ranked[1:] {
		t := FindDesignTemplate(m.ID)
		if m.Score >= minTemplateScore && t.Category != best.Category {
			c.Secondary = t.ID
			break
		}
	}
	return c
}

// templateOut is the designer's template decision in the design reply.
type templateOut struct {
	Primary   string `json:"primary"`
	Secondary string `json:"secondary"`
	Reason    string `json:"reason"`
}

// ChooseTemplate records the template a new design follows: the
// designer's choice when it names a template of the library, else
// Foundry's recommendation.
func ChooseTemplate(p *core.Project, out *templateOut) *core.TemplateChoice {
	if c, ok := pickedTemplate(p, out); ok {
		return c
	}
	rec := RecommendTemplate(p)
	if out == nil || strings.TrimSpace(out.Primary) == "" || out.Primary == "none" {
		return rec
	}
	t := FindDesignTemplate(strings.TrimSpace(out.Primary))
	if t == nil {
		if rec != nil {
			rec.Reasons = append(rec.Reasons, fmt.Sprintf("template %q pilihan AI tidak ada di library", out.Primary))
		}
		return rec
	}
	c := &core.TemplateChoice{ID: t.ID, Name: t.Name, Source: "ai"}
	if s := strings.TrimSpace(out.Secondary); s != t.ID && FindDesignTemplate(s) != nil {
		c.Secondary = s
	}
	if r := strings.TrimSpace(out.Reason); r != "" {
		c.Reasons = append(c.Reasons, r)
	}
	if rec != nil && rec.ID != t.ID {
		c.Reasons = append(c.Reasons, "rekomendasi Foundry: "+rec.Name)
	}
	return c
}

// pickedTemplate is the user's own pick, which the designer follows: ok is
// false when the user picked nothing (or a template no longer in the
// library), leaving the choice to the designer.
func pickedTemplate(p *core.Project, out *templateOut) (*core.TemplateChoice, bool) {
	if p.DesignTemplate == core.TemplateBlank {
		return nil, true
	}
	t := FindDesignTemplate(p.DesignTemplate)
	if t == nil {
		return nil, false
	}
	c := &core.TemplateChoice{ID: t.ID, Name: t.Name, Source: "user", Reasons: []string{"dipilih user sebagai struktur awal"}}
	if out != nil {
		if s := strings.TrimSpace(out.Secondary); s != t.ID && FindDesignTemplate(s) != nil {
			c.Secondary = s
		}
		if r := strings.TrimSpace(out.Reason); r != "" {
			c.Reasons = append(c.Reasons, r)
		}
	}
	return c, true
}

// currentTemplates are the templates the design follows: primary first.
func currentTemplates(p *core.Project) []*core.DesignTemplate {
	if p.Design == nil || p.Design.Template == nil {
		return nil
	}
	var out []*core.DesignTemplate
	for _, id := range []string{p.Design.Template.ID, p.Design.Template.Secondary} {
		if t := FindDesignTemplate(id); t != nil {
			out = append(out, t)
		}
	}
	return out
}

// ---------- Adaptation ----------

// AdaptedRegion is one template region filled for a domain.
type AdaptedRegion struct {
	core.TemplateRegion
	Content string   `json:"content"`
	Items   []string `json:"items,omitempty"`
}

// templateExample is the template's example for the domain, else its
// generic one, else its first; nil without examples.
func templateExample(t core.DesignTemplate, domain string) *core.TemplateExample {
	for _, want := range []string{domain, ""} {
		for i := range t.Examples {
			if t.Examples[i].Domain == want {
				return &t.Examples[i]
			}
		}
	}
	if len(t.Examples) > 0 {
		return &t.Examples[0]
	}
	return nil
}

// AdaptTemplate fills the template's regions with what they hold in the
// domain. The same template gives a different composition per domain;
// the structure (roles, types, areas) stays.
func AdaptTemplate(t core.DesignTemplate, domain string) []AdaptedRegion {
	ex := templateExample(t, domain)
	out := make([]AdaptedRegion, 0, len(t.Composition))
	for _, r := range t.Composition {
		a := AdaptedRegion{TemplateRegion: r, Content: r.Purpose}
		if ex != nil {
			if c := ex.Regions[r.Region]; c != "" {
				a.Content = c
			}
			a.Items = ex.Items[r.Region]
		}
		out = append(out, a)
	}
	return out
}

// TemplatePreview is the template drawn for a domain as one Design IR page
// with sample data, for the template gallery. It goes through the same IR
// validation and renderer as a designed page; the theme styles it.
func TemplatePreview(t core.DesignTemplate, domain string) core.Page {
	strategy := t.Layout.Strategies[0]
	pg := core.Page{ID: "PREVIEW", Name: t.Name, Platform: core.PlatformWeb, Purpose: t.Intent,
		Kind: t.SuitableFor.PageKinds[0], Density: t.Layout.Density, Access: "auth"}
	if ex := templateExample(t, domain); ex != nil {
		pg.Name = ex.Product
	}
	pg.Intent = &core.PageIntent{Frequency: t.SuitableFor.Frequency, InformationDensity: t.SuitableFor.Density}
	if len(t.SuitableFor.Tasks) > 0 {
		pg.Intent.PrimaryTask = t.SuitableFor.Tasks[0]
	}
	plan := &core.PagePlan{Layout: strategy, Responsive: t.Responsive.Strategy, Reasons: map[string]string{}}
	areas := map[string][]core.LayoutNode{}
	var grids []string
	hasNav, hasPrimary := false, false
	for _, r := range AdaptTemplate(t, domain) {
		c := core.PageComponent{ID: r.Region, Type: r.Type, Label: r.Content}
		switch r.Type {
		case "nav":
			c.Items = r.Items
			hasNav = true
			pg.Components = append(pg.Components, c)
			continue
		case "table":
			c.Columns = r.Items
		case "form":
			for _, it := range r.Items {
				c.Fields = append(c.Fields, core.FormField{Label: it, Type: "text", Required: true})
			}
		case "button":
			switch {
			case r.Role == "action" && !hasPrimary:
				c.Emphasis, hasPrimary = "action.primary", true
			case r.Role == "action":
				c.Emphasis = "action.secondary"
			default:
				c.Emphasis = "action.ghost"
			}
		default:
			c.Items = r.Items
		}
		node := core.LayoutNode{Component: c.ID}
		ids := []string{c.ID}
		if r.Type == "stat" {
			// One stat per sample value, side by side.
			labels := r.Items
			if len(labels) == 0 {
				labels = []string{r.Content}
			}
			ids = nil
			node = core.LayoutNode{ID: "n-" + r.Region, Type: "grid", Columns: min(len(labels), 4)}
			for i, l := range labels {
				id := fmt.Sprintf("%s-%d", r.Region, i+1)
				pg.Components = append(pg.Components, core.PageComponent{ID: id, Type: "stat", Label: l})
				node.Children = append(node.Children, core.LayoutNode{Component: id})
				ids = append(ids, id)
			}
			if node.Columns > 2 {
				grids = append(grids, node.ID)
			}
		} else {
			pg.Components = append(pg.Components, c)
		}
		for _, id := range ids {
			plan.Reasons[id] = r.Purpose
			switch r.Role {
			case "primary":
				if plan.Primary == "" {
					plan.Primary = id
				}
			case "secondary", "action":
				plan.Secondary = append(plan.Secondary, id)
			default:
				plan.Supporting = append(plan.Supporting, id)
			}
		}
		area := r.Area
		if strategy == "focused" {
			area = "main"
		}
		areas[area] = append(areas[area], node)
	}
	pg.Plan = plan

	l := &core.PageLayout{Frame: "app", Nav: "none", Width: "full"}
	mobile := core.DeviceLayout{Nodes: map[string]core.NodeChange{}, Swap: map[string]string{}}
	switch strategy {
	case "focused":
		l.Frame, l.Width, l.Brand = "center", "narrow", true
	case "dashboard", "sections":
		l.Width = "wide"
	}
	if hasNav && l.Frame == "app" {
		l.Nav = orDefault(t.Layout.Navigation, "side")
		if l.Nav == "side" || l.Nav == "rail" {
			mobile.Nav = "bottom"
		}
	}
	stack := func(id string, kids []core.LayoutNode) core.LayoutNode {
		return core.LayoutNode{ID: id, Type: "stack", Gap: "space.md", Children: kids}
	}
	body := stack("n-main", areas["main"])
	side, aside := areas["side"], areas["aside"]
	sidebar := func(id, at string, a, b core.LayoutNode) core.LayoutNode {
		mobile.Nodes[id] = core.NodeChange{Type: "stack"}
		return core.LayoutNode{ID: id, Type: "sidebar", Side: at, Gap: "space.md", Children: []core.LayoutNode{a, b}}
	}
	if len(aside) > 0 {
		body = sidebar("n-work", "end", body, stack("n-aside", aside))
	}
	if len(side) > 0 {
		body = sidebar("n-body", "start", stack("n-side", side), body)
	}
	var root []core.LayoutNode
	if top := areas["top"]; len(top) == 1 {
		root = append(root, top[0])
	} else if len(top) > 1 {
		root = append(root, core.LayoutNode{ID: "n-top", Type: "row", Align: "between", Children: top})
	}
	l.Root = stack("n-root", append(root, body))
	for _, id := range grids {
		mobile.Nodes[id] = core.NodeChange{Columns: 2}
	}
	for _, c := range pg.Components {
		if c.Type == "table" {
			mobile.Swap[c.ID] = "list"
		}
	}
	if mobile.Nav != "" || len(mobile.Nodes) > 0 || len(mobile.Swap) > 0 {
		l.Devices = map[string]core.DeviceLayout{"mobile": mobile}
	}
	pg.Layout = l
	return pg
}

// ---------- Knowledge for the designer ----------

// templateKnowledge is the template section of the design prompt: the
// library in short, Foundry's recommendation and the full pattern of the
// recommended templates, adapted to the domain.
func templateKnowledge(p *core.Project) string {
	tpls, _ := DesignTemplates()
	if len(tpls) == 0 {
		return ""
	}
	in := templateInput(p)
	var b strings.Builder
	if p.DesignTemplate == core.TemplateBlank {
		return "DESIGN TEMPLATES: the user chose a blank canvas. Use no template; design every page from its own job, and omit \"template\".\n"
	}
	if t := FindDesignTemplate(p.DesignTemplate); t != nil {
		fmt.Fprintf(&b, "DESIGN TEMPLATE: the user picked %s (%s) as the starting structure: use it as the primary template, adapted to this product. "+
			"A secondary pattern only for pages of another kind (e.g. sign-in). Explain the adaptation in \"template.reason\".\n", t.ID, t.Name)
		b.WriteString(templateDetail(*t, in.Domain))
		return b.String()
	}
	b.WriteString("DESIGN TEMPLATES (proven UX patterns: a structural and composition reference, never a page to copy; the theme decides the look, the template the structure):\n")
	for _, t := range tpls {
		fmt.Fprintf(&b, "- %s (%s): %s For pages of kind %s. Layout %s; %s. Characteristics: %s.\n", t.ID, t.Name, t.Intent,
			strings.Join(t.SuitableFor.PageKinds, "/"), strings.Join(t.Layout.Strategies, "/"), t.Layout.Structure, strings.Join(t.Characteristics, ", "))
	}
	rec := RecommendTemplate(p)
	if rec == nil {
		b.WriteString("Foundry found no clear match: choose the template whose user, task, frequency and density fit, or none.\n")
	} else {
		fmt.Fprintf(&b, "Foundry recommends: %s (%s)", rec.ID, strings.Join(rec.Reasons, "; "))
		if rec.Secondary != "" {
			b.WriteString(", secondary pattern " + rec.Secondary)
		}
		b.WriteString(". Choose differently only with a reason.\n")
		for _, id := range []string{rec.ID, rec.Secondary} {
			if t := FindDesignTemplate(id); t != nil {
				b.WriteString(templateDetail(*t, in.Domain))
			}
		}
	}
	b.WriteString(`Select one primary template and an optional secondary pattern (for pages of another kind, e.g. sign-in). Adapt it: map its regions to THIS product's features, data and flows; ` +
		`keep its hierarchy, composition and responsive strategy where the page's job allows; never copy example labels. Pages the template does not cover are designed from their own job. ` +
		`Explain the choice in "template.reason" (user, task, frequency, density).` + "\n")
	return b.String()
}

// templateDetail is one template's full pattern for the prompt.
func templateDetail(t core.DesignTemplate, domain string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Template %s:\n  Structure: %s. Navigation %s, density %s.\n  Information hierarchy: %s.\n  Composition (adapted to this domain):\n",
		t.ID, t.Layout.Structure, t.Layout.Navigation, t.Layout.Density, strings.Join(t.Hierarchy, " > "))
	for _, r := range AdaptTemplate(t, domain) {
		fmt.Fprintf(&b, "    - %s [%s, %s, %s]: %s", r.Region, r.Role, r.Type, r.Area, r.Purpose)
		if r.Content != r.Purpose {
			b.WriteString(" (e.g. " + r.Content + ")")
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "  Responsive: %s. Desktop: %s. Mobile: %s.\n", t.Responsive.Strategy, t.Responsive.Desktop, t.Responsive.Mobile)
	fmt.Fprintf(&b, "  Expected: %s. Avoid: %s.\n", strings.Join(t.Quality.Expected, "; "), strings.Join(t.Quality.Avoid, ", "))
	if len(t.Quality.Checks) > 0 {
		b.WriteString("  Checks: " + strings.Join(t.Quality.Checks, "; ") + ".\n")
	}
	return b.String()
}

// ---------- Template consistency ----------

// templateFor is the template that structures a page: the first of the
// design's templates made for the page's kind.
func templateFor(pg core.Page, tpls []*core.DesignTemplate) *core.DesignTemplate {
	kind := core.PageKind(pg)
	for _, t := range tpls {
		if slices.Contains(t.SuitableFor.PageKinds, kind) {
			return t
		}
	}
	return nil
}

// TemplateLint lists where a page departs from its template: a layout
// strategy outside the pattern, a looser density, or an anti-pattern the
// template avoids. Category "template".
func TemplateLint(pg core.Page, tpls []*core.DesignTemplate) []core.QAIssue {
	t := templateFor(pg, tpls)
	if t == nil || pg.Layout == nil {
		return nil
	}
	var out []core.QAIssue
	add := func(rule, sev, comp, problem, fix string) {
		out = append(out, core.QAIssue{PageID: pg.ID, Severity: sev, Category: "template", Component: comp, Problem: problem,
			Why: "template " + t.Name + ": " + strings.Join(t.Quality.Expected, ", "), Fix: fix, Source: "lint", Rule: rule})
	}
	if pl := pg.Plan; pl != nil && pl.Layout != "" && !slices.Contains(t.Layout.Strategies, pl.Layout) {
		add("template_layout", "minor", "", fmt.Sprintf("Layout %s di luar pola template %s (%s)", pl.Layout, t.Name, strings.Join(t.Layout.Strategies, ", ")),
			"Pakai "+t.Layout.Strategies[0]+" bila tugas halaman mengizinkan.")
	}
	if want, ok := densityRank[t.Layout.Density]; ok && pg.Density != "" && densityRank[pg.Density] > want {
		add("template_density", "minor", "", fmt.Sprintf("Density %s lebih longgar dari template (%s)", pg.Density, t.Layout.Density),
			"Pakai density "+t.Layout.Density+".")
	}
	avoid := func(a string) bool { return slices.Contains(t.Quality.Avoid, a) }
	count := map[string]int{}
	for _, c := range pg.Components {
		count[c.Type]++
	}
	if avoid("hero_section") {
		if id, ok := heroNode(pg.Layout.Root, pg); ok {
			add("template_hero", "major", id, "Bagian pembuka berbentuk hero", "Mulai langsung dengan "+t.Hierarchy[0]+".")
		}
	}
	if avoid("large_cards") && count["card"] >= 3 {
		add("template_cards", "major", "", fmt.Sprintf("%d kartu terpisah", count["card"]), "Gabungkan jadi daftar atau tabel.")
	}
	if avoid("excessive_stats") && count["stat"] > 3 {
		add("template_stats", "minor", "", fmt.Sprintf("%d statistik; template membatasi 3", count["stat"]), "Sisakan angka yang mendorong keputusan.")
	}
	return out
}

// templateUnused reports a design whose template structures none of its
// pages: the choice was recorded but not applied.
func templateUnused(p *core.Project) *core.QAIssue {
	tpls := currentTemplates(p)
	if len(tpls) == 0 {
		return nil
	}
	for _, pg := range p.Design.Pages {
		if templateFor(pg, tpls) != nil {
			return nil
		}
	}
	return &core.QAIssue{Severity: "minor", Category: "template", Source: "lint", Rule: "template_unused",
		Problem: "Tidak ada halaman yang memakai pola template " + tpls[0].Name,
		Why:     "template dipilih tetapi tidak diterapkan", Fix: "Susun halaman kerja utama dengan pola " + tpls[0].Layout.Structure + "."}
}

// ---------- Gallery ----------

// TemplateGallery is the template library drawn for the gallery: every
// template's preview page (Design IR with sample data) for one domain,
// styled by one theme.
type TemplateGallery struct {
	Templates   []core.DesignTemplate `json:"templates"`
	Previews    map[string]core.Page  `json:"previews"`
	Domain      string                `json:"domain,omitempty"`
	System      *core.DesignSystem    `json:"system,omitempty"`
	Theme       *core.ThemeContext    `json:"theme,omitempty"`
	Recommended *core.TemplateChoice  `json:"recommended,omitempty"`
	Invalid     []ThemeProblem        `json:"invalid,omitempty"`
}

// Gallery builds the gallery. The theme is the given one, else the
// project's; the domain the given one, else the project's. p may be nil.
func Gallery(p *core.Project, theme, domain string) TemplateGallery {
	tpls, bad := DesignTemplates()
	g := TemplateGallery{Templates: tpls, Previews: map[string]core.Page{}, Domain: domain, Invalid: bad}
	dir := ""
	if p != nil {
		dir = p.Dir
		if g.Domain == "" {
			g.Domain = templateInput(p).Domain
		}
		if theme == "" {
			if t := currentTheme(p); t != nil {
				theme = t.ID
			} else if t, _ := ResolveTheme(p); t != nil {
				theme = t.ID
			}
		}
		g.Recommended = RecommendTemplate(p)
	}
	themes, _ := Themes(dir)
	if t, ctx := SelectTheme(themes, theme, g.Domain); t != nil {
		g.System, g.Theme = ThemeSystem(*t, nil), &ctx
	}
	for _, t := range tpls {
		g.Previews[t.ID] = TemplatePreview(t, g.Domain)
	}
	return g
}
