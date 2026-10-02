package work

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/handrawanw/foundry/internal/core"
)

func tpl(t *testing.T, id string) core.DesignTemplate {
	t.Helper()
	tp := FindDesignTemplate(id)
	if tp == nil {
		t.Fatalf("template %s missing", id)
	}
	return *tp
}

func TestBuiltinTemplates(t *testing.T) {
	all, bad := DesignTemplates()
	if len(bad) > 0 {
		t.Fatalf("invalid templates: %+v", bad)
	}
	want := []string{"auth-clean", "dashboard-executive", "developer-tool", "finance-overview", "master-detail", "table-management", "trading-terminal", "workspace-agent"}
	var ids []string
	for _, tp := range all {
		ids = append(ids, tp.ID)
		// The preview is Design IR: valid for every domain it has an example for.
		for _, dom := range []string{"", "ticketing", "finance", "trading", "developer_tools", "admin_operations"} {
			pg := TemplatePreview(tp, dom)
			if probs := core.ValidatePage(pg, map[string]bool{pg.ID: true}); len(probs) > 0 {
				t.Errorf("%s/%s preview invalid: %v", tp.ID, dom, probs)
			}
			if pg.Plan == nil || pg.Plan.Primary == "" || !slices.Contains(tp.Layout.Strategies, pg.Plan.Layout) {
				t.Errorf("%s preview plan = %+v", tp.ID, pg.Plan)
			}
		}
	}
	if !slices.Equal(ids, want) {
		t.Fatalf("library = %v", ids)
	}
	for _, tp := range all {
		if len(tp.Wireframes) != 3 {
			t.Errorf("%s: %d wireframes", tp.ID, len(tp.Wireframes))
		}
	}
	bad1 := tpl(t, "workspace-agent")
	bad1.Layout.Strategies = []string{"carousel"}
	bad1.Quality.Avoid = []string{"blink"}
	if err := ValidateDesignTemplate(bad1); err == nil || !strings.Contains(err.Error(), "carousel") || !strings.Contains(err.Error(), "blink") {
		t.Fatalf("err = %v", err)
	}
	bad2 := tpl(t, "auth-clean")
	bad2.Wireframes = append([]core.TemplateWireframe(nil), bad2.Wireframes...)
	bad2.Wireframes[1].Layout = "dashboard"
	bad2.Wireframes[2].Regions = []string{"ghost"}
	if err := ValidateDesignTemplate(bad2); err == nil || !strings.Contains(err.Error(), "di luar layout_pattern") || !strings.Contains(err.Error(), `"ghost"`) {
		t.Fatalf("err = %v", err)
	}
	bad2.Wireframes = bad2.Wireframes[:2]
	if err := ValidateDesignTemplate(bad2); err == nil || !strings.Contains(err.Error(), "tepat 3 halaman") {
		t.Fatalf("err = %v", err)
	}
}

func TestTemplateSelection(t *testing.T) {
	cases := []struct{ story, want string }{
		{"Sistem tiket support untuk agen helpdesk dengan SLA", "workspace-agent"},
		{"Dompet digital untuk melihat saldo, transaksi dan anggaran", "finance-overview"},
		{"Platform trading saham dengan watchlist dan order book", "trading-terminal"},
		{"Platform developer untuk deploy API dan melihat logs", "developer-tool"},
		{"Back office inventori gudang untuk mengelola stok", "table-management"},
		{"CRM untuk mencatat pelanggan dan deal", "master-detail"},
		{"Dashboard manajemen untuk memantau KPI dan laporan kinerja cabang", "dashboard-executive"},
		{"Aplikasi resep masakan keluarga", ""},
	}
	for _, c := range cases {
		got := RecommendTemplate(&core.Project{Story: c.story})
		id := ""
		if got != nil {
			id = got.ID
			if got.Source != "auto" || len(got.Reasons) == 0 {
				t.Errorf("%q: choice = %+v", c.story, got)
			}
		}
		if id != c.want {
			t.Errorf("%q → %q, want %q", c.story, id, c.want)
		}
	}
	got := RecommendTemplate(&core.Project{Story: "Sistem tiket support untuk agen; login, registrasi dan lupa password"})
	if got == nil || got.ID != "workspace-agent" || got.Secondary != "auth-clean" {
		t.Fatalf("secondary = %+v", got)
	}
}

// previewShape is a preview without its words: component types and the tree.
func previewShape(pg core.Page) string {
	var types []string
	for _, c := range pg.Components {
		types = append(types, c.ID+":"+c.Type)
	}
	b, _ := json.Marshal(pg.Layout)
	return strings.Join(types, ",") + string(b)
}

func TestTemplateAdaptation(t *testing.T) {
	tp := tpl(t, "workspace-agent")
	ticket, ops := TemplatePreview(tp, "ticketing"), TemplatePreview(tp, "admin_operations")
	if reflect.DeepEqual(ticket, ops) {
		t.Fatal("same template in another domain must adapt its content")
	}
	if previewShape(ticket) != previewShape(ops) {
		t.Fatal("adaptation must keep the template's structure")
	}
	if ticket.Components[3].Label != "Percakapan tiket #1042" || ops.Components[3].Label != "Permintaan PO-2210" {
		t.Fatalf("detail = %q / %q", ticket.Components[3].Label, ops.Components[3].Label)
	}
	// A domain without its own example falls back to the generic one.
	if a := AdaptTemplate(tpl(t, "table-management"), "trading"); a[1].Content != "Pengguna" {
		t.Fatalf("fallback = %+v", a[1])
	}
}

func TestTemplateWithTheme(t *testing.T) {
	pg := TemplatePreview(tpl(t, "trading-terminal"), "trading")
	themes, _ := Themes("")
	var systems []*core.DesignSystem
	for _, id := range []string{"trading-terminal", "sakura-light"} {
		d := &core.Design{Pages: []core.Page{pg}}
		th, ctx := SelectTheme(themes, id, "")
		ApplyTheme(d, th, ctx)
		if !reflect.DeepEqual(d.Pages[0], pg) {
			t.Fatal("the theme must not change the template's IR")
		}
		systems = append(systems, d.System)
	}
	if reflect.DeepEqual(systems[0].Colors, systems[1].Colors) {
		t.Fatal("same template with another theme must look different")
	}
}

func TestTemplateChoice(t *testing.T) {
	p := irProject()
	p.Story = "Sistem tiket support untuk agen helpdesk"
	withTpl := strings.Replace(irDesign, `{"system": null,`, `{"template": {"primary": "table-management", "reason": "agen mengelola banyak tiket"}, "system": null,`, 1)
	if err := ApplyDesign(p, withTpl); err != nil {
		t.Fatal(err)
	}
	c := p.Design.Template
	if c == nil || c.ID != "table-management" || c.Source != "ai" || c.Reasons[0] != "agen mengelola banyak tiket" || !strings.Contains(c.Reasons[1], "Workspace Agent") {
		t.Fatalf("choice = %+v", c)
	}

	unknown := strings.Replace(irDesign, `{"system": null,`, `{"template": {"primary": "landing-page"}, "system": null,`, 1)
	if err := ApplyDesign(p, unknown); err != nil {
		t.Fatal(err)
	}
	if c := p.Design.Template; c == nil || c.ID != "workspace-agent" || c.Source != "auto" || !strings.Contains(strings.Join(c.Reasons, ";"), "landing-page") {
		t.Fatalf("fallback = %+v", c)
	}

	// The user's pick wins over the AI's; a blank canvas means no template.
	p.DesignTemplate = "master-detail"
	if err := ApplyDesign(p, withTpl); err != nil {
		t.Fatal(err)
	}
	if c := p.Design.Template; c == nil || c.ID != "master-detail" || c.Source != "user" || c.Reasons[1] != "agen mengelola banyak tiket" {
		t.Fatalf("user choice = %+v", c)
	}
	p.DesignTemplate = core.TemplateBlank
	if err := ApplyDesign(p, withTpl); err != nil {
		t.Fatal(err)
	}
	if p.Design.Template != nil {
		t.Fatalf("blank = %+v", p.Design.Template)
	}

	// No signal and no choice: a design without a template, as before.
	plain := irProject()
	plain.Story, plain.Features[0].Name = "Aplikasi resep masakan", "Resep"
	if err := ApplyDesign(plain, irDesign); err != nil {
		t.Fatal(err)
	}
	if plain.Design.Template != nil {
		t.Fatalf("template = %+v", plain.Design.Template)
	}
}

func TestTemplatePrompt(t *testing.T) {
	p := &core.Project{Story: "Sistem tiket support untuk agen helpdesk", Platform: &core.PlatformPlan{}}
	s := DesignPrompt(p)
	for _, want := range []string{"DESIGN TEMPLATES", "- trading-terminal (Trading Terminal)", "Foundry recommends: workspace-agent", "Template workspace-agent:", "Wireframes (the pattern on 3 pages", `Page "Monitoring antrean"`,
		"queue [secondary, list, side]", "(e.g. Antrean tiket)", "Avoid: hero_section", `"template": {"primary"`} {
		if !strings.Contains(s, want) {
			t.Errorf("prompt misses %q", want)
		}
	}
	p.DesignTemplate = "finance-overview"
	if s := DesignPrompt(p); !strings.Contains(s, "the user picked finance-overview") || !strings.Contains(s, "Template finance-overview:") || strings.Contains(s, "Foundry recommends") {
		t.Error("prompt must follow the user's template")
	}
	p.DesignTemplate = core.TemplateBlank
	if s := DesignPrompt(p); !strings.Contains(s, "blank canvas") || strings.Contains(s, "Template workspace-agent:") {
		t.Error("prompt must design without a template")
	}
}

func TestQualityWarnings(t *testing.T) {
	before := &core.DesignQuality{Decision: "pass", Dimensions: map[string]int{"responsive": 100, "hierarchy": 80}}
	after := &core.DesignQuality{Decision: "revision_required", Dimensions: map[string]int{"responsive": 70, "hierarchy": 90}}
	got := QualityWarnings(before, after)
	if !slices.Equal(got, []string{"Rancangan tidak lagi lolos gate kualitas", "Perubahan ini menurunkan kegunaan di mobile (100 → 70)"}) {
		t.Fatalf("warnings = %v", got)
	}
	if QualityWarnings(after, after) != nil || QualityWarnings(nil, after) != nil {
		t.Fatal("no drop, no warning")
	}
}

func TestTemplateQuality(t *testing.T) {
	pg := TemplatePreview(tpl(t, "workspace-agent"), "ticketing")
	pg.ID = "P1"
	p := qualityProject(pg)
	p.Design.Template = &core.TemplateChoice{ID: "workspace-agent", Name: "Workspace Agent", Source: "auto"}
	if is := TemplateLint(pg, currentTemplates(p)); len(is) != 0 {
		t.Fatalf("the template's own preview must follow it: %+v", is)
	}
	before := MeasureDesign(p)

	off := pg
	plan := *pg.Plan
	plan.Layout = "sections"
	off.Plan, off.Density = &plan, "spacious"
	issues := TemplateLint(off, currentTemplates(p))
	var rules []string
	for _, is := range issues {
		rules = append(rules, is.Rule)
		if is.Category != "template" || is.Source != "lint" {
			t.Fatalf("issue = %+v", is)
		}
	}
	if !slices.Equal(rules, []string{"template_layout", "template_density"}) {
		t.Fatalf("rules = %v", rules)
	}
	p.Design.Pages = []core.Page{off}
	if after := MeasureDesign(p); after.Dimensions["composition"] >= before.Dimensions["composition"] {
		t.Fatalf("composition %d → %d", before.Dimensions["composition"], after.Dimensions["composition"])
	}

	// A template that structures no page is reported for the whole design.
	p.Design.Template = &core.TemplateChoice{ID: "auth-clean", Name: "Auth Clean"}
	q := MeasureDesign(p)
	if !slices.ContainsFunc(q.Findings, func(is core.QAIssue) bool { return is.Rule == "template_unused" }) {
		t.Fatalf("findings = %+v", q.Findings)
	}
}

func TestDesignWithoutTemplate(t *testing.T) {
	// A project saved before templates: no template field, same quality.
	raw := `{"status": "draft", "domain": "ticketing", "pages": []}`
	var d core.Design
	if err := json.Unmarshal([]byte(raw), &d); err != nil || d.Template != nil {
		t.Fatalf("design = %+v, %v", d, err)
	}
	pg := TemplatePreview(tpl(t, "workspace-agent"), "ticketing")
	p := qualityProject(pg)
	if len(currentTemplates(p)) != 0 || TemplateLint(pg, nil) != nil || templateUnused(p) != nil {
		t.Fatal("a design without template has no template findings")
	}
	if q := MeasureDesign(p); q == nil || q.Overall == 0 {
		t.Fatalf("quality = %+v", q)
	}
	p.Design.Template = &core.TemplateChoice{ID: "gone"} // removed from the library
	if len(currentTemplates(p)) != 0 {
		t.Fatal("an unknown template is ignored")
	}
}
