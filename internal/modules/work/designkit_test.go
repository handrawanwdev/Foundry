package work

import (
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
)

const kitDesign = `{"rules": {"source": "new", "preset": "ops-dense", "brand": {"primary": "#0E7490", "font": "IBM Plex Sans"},
  "principles": ["Padat"], "ux_guidelines": ["Filter di atas tabel"], "accessibility": ["AA"], "responsive": ["Tabel scroll"],
  "tokens": {"colors": [{"name": "status.overdue", "value": "#9f1239", "use": "terlambat"}], "spacing": [{"name": "space.md", "value": "15px"}]}},
 "users": [{"role": "Admin", "platform": "web", "context": "harian", "tasks": ["saring antrean"]}],
 "goals": [{"id": "GOAL-1", "role": "Admin", "text": "Saring antrean"}],
 "shells": [{"id": "app", "name": "App", "platform": "web", "pattern": "sidebar", "regions": [{"id": "menu", "name": "Menu", "content": ["Menu"]}]}],
 "pages": [{"id": "antrean", "name": "Antrean", "platform": "web", "route": "/antrean", "purpose": "Saring antrean", "goal": "GOAL-1", "shell": "app", "pattern": "table",
   "sections": [{"id": "saring", "name": "Filter", "content": ["Cari pasien, Status"]},
                {"id": "data", "name": "Data antrean", "content": ["No, Pasien, Status"], "components": ["Table", "Badge"]}],
   "states": {"loading": "Memuat", "empty": "Kosong", "error": "Gagal"}},
  {"id": "detail", "name": "Detail", "platform": "web", "route": "/antrean/1", "purpose": "Lihat janji", "goal": "GOAL-1", "shell": "app", "pattern": "detail",
   "sections": [{"id": "status", "name": "Status", "slot": "summary", "content": ["Status, Jam"]}],
   "links": [{"to": "antrean", "via": "Kembali"}], "states": {"loading": "Memuat", "error": "Gagal"}}]}`

// The AI picks a preset, a brand and pages; Foundry fills the tokens from
// the preset, places every section in a template slot (a required slot is
// never left empty) and adds the kit components the pages reference.
func TestApplyDesignUsesUIKit(t *testing.T) {
	p := &core.Project{Platform: &core.PlatformPlan{Targets: []core.PlatformTarget{{Kind: core.PlatformWeb}}}}
	if err := ApplyDesign(p, kitDesign); err != nil {
		t.Fatal(err)
	}
	r := p.UI.Rules
	if r.Preset != "ops-dense" || p.UI.Format != core.UIDesignFormat {
		t.Fatalf("preset=%q format=%d", r.Preset, p.UI.Format)
	}
	checks := map[string]string{
		tokenValue(r.Tokens.Colors, "action.primary"):     "#0e7490",
		tokenValue(r.Tokens.Colors, "status.overdue"):     "#9f1239",
		tokenValue(r.Tokens.Spacing, "space.md"):          "12px",
		tokenValue(r.Tokens.Spacing, "size.control"):      "32px",
		tokenValue(r.Tokens.Typography, "type.body")[:13]: "IBM Plex Sans",
	}
	for got, want := range checks {
		if got != want {
			t.Errorf("token %q, mau %q", got, want)
		}
	}
	slots := map[string]string{}
	for _, pg := range p.UI.Layout.Pages {
		for _, s := range pg.Sections {
			slots[pg.ID+"/"+s.ID] = s.Slot
		}
	}
	if slots["antrean/saring"] != "toolbar" || slots["antrean/data"] != "table" || slots["detail/status"] != "main" {
		t.Fatalf("slots = %v", slots)
	}
	// The table has the page's filter but no pagination: Foundry adds it.
	data := strings.Join(p.UI.Layout.Pages[0].Sections[1].Content, " | ")
	if !strings.Contains(data, "Paginasi:") || strings.Contains(data, "Filter:") {
		t.Fatalf("table content = %s", data)
	}
	if p.UI.Layout.Shells[0].Regions[0].Slot != "nav" {
		t.Fatalf("shell region slot = %q", p.UI.Layout.Shells[0].Regions[0].Slot)
	}
	var names []string
	for _, c := range r.Components {
		names = append(names, c.Name)
	}
	if !strings.Contains(strings.Join(names, ","), "Table") || !strings.Contains(strings.Join(names, ","), "Badge") {
		t.Fatalf("components = %v", names)
	}
	if errs := core.Errors(p.DesignIssues(nil)); len(errs) > 0 {
		t.Fatalf("errors = %+v", errs)
	}
}

// An unknown preset falls back to the default; a slot that is not in the
// template is an error for a kit design and a warning for an older one.
func TestKitValidation(t *testing.T) {
	p := &core.Project{Platform: &core.PlatformPlan{Targets: []core.PlatformTarget{{Kind: core.PlatformWeb}}}}
	if err := ApplyDesign(p, strings.Replace(kitDesign, `"ops-dense"`, `"neon"`, 1)); err != nil {
		t.Fatal(err)
	}
	if p.UI.Rules.Preset != "saas-neutral" {
		t.Fatalf("preset = %q", p.UI.Rules.Preset)
	}
	d := cloneDesign(p.UI)
	d.Layout.Pages[0].Sections[1].Slot = "sidebar"
	issue := func(d *core.UIDesign) string {
		for _, is := range p.DesignIssues(d) {
			if strings.Contains(is.Message, `slot "sidebar"`) {
				return is.Level
			}
		}
		return ""
	}
	if lv := issue(d); lv != core.IssueError {
		t.Fatalf("slot asing = %q", lv)
	}
	d.Format = core.UIUXFormat
	if lv := issue(d); lv != core.IssueWarning {
		t.Fatalf("slot asing di format lama = %q", lv)
	}
	d = cloneDesign(p.UI)
	d.Rules.Preset = "neon"
	if !strings.Contains(issuesText(p.DesignIssues(d)), `preset "neon" tidak ada`) {
		t.Fatal("preset tak dikenal lolos")
	}
	d = cloneDesign(p.UI)
	d.Rules.Tokens.Spacing = append(d.Rules.Tokens.Spacing, core.Token{Name: "space.odd", Value: "13px"})
	if !strings.Contains(issuesText(p.DesignIssues(d)), "di luar skala preset") {
		t.Fatal("spacing di luar skala tidak diperingatkan")
	}
}

// An existing project keeps its own design system: the preset is recorded
// for the mockup, its tokens are not added to the rules.
func TestKitKeepsExistingDesignSystem(t *testing.T) {
	p := &core.Project{Platform: &core.PlatformPlan{Targets: []core.PlatformTarget{{Kind: core.PlatformWeb}}},
		Structure: &core.Structure{Mode: core.StructureExisting, UI: &core.ExistingUI{
			Tokens: &core.DesignTokens{Colors: []core.Token{{Name: "brand.500", Value: "#7c3aed"}}}}}}
	if err := ApplyDesign(p, kitDesign); err != nil {
		t.Fatal(err)
	}
	if tokenValue(p.UI.Rules.Tokens.Spacing, "size.control") != "" || tokenValue(p.UI.Rules.Tokens.Colors, "brand.500") != "#7c3aed" {
		t.Fatalf("tokens = %+v", p.UI.Rules.Tokens)
	}
}

func issuesText(is []core.Issue) string {
	var b strings.Builder
	for _, i := range is {
		b.WriteString(i.Level + " " + i.Message + "\n")
	}
	return b.String()
}
