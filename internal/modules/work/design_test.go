package work

import (
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
)

const existingDesign = `{"rules": {"source": "new", "principles": ["Ikuti UI yang ada"], "ux_guidelines": ["Menu tetap di kiri"],
  "accessibility": ["AA"], "responsive": ["Sidebar jadi drawer di mobile"],
  "tokens": {"colors": [{"name": "action.primary", "value": "#000000"}, {"name": "status.info", "value": "#0ea5e9"}],
             "typography": [{"name": "type.body", "value": "Inter 14px"}], "spacing": [{"name": "space.md", "value": "16px"}]},
  "components": [{"name": "sidebar", "use": "navigasi"}]},
 "pages": [
  {"id": "orders", "name": "Pesanan", "platform": "web", "route": "/orders", "purpose": "Ubah filter pesanan", "pattern": "detail",
   "sections": [{"id": "filter", "name": "Filter", "content": ["Status"]}], "states": {"loading": "Memuat", "error": "Gagal"}},
  {"id": "budget", "name": "Anggaran", "platform": "web", "route": "/budget", "purpose": "Atur anggaran", "pattern": "detail",
   "sections": [{"id": "isi", "name": "Isi", "content": ["Limit bulanan"]}], "states": {"loading": "Memuat", "error": "Gagal"}}]}`

// An existing project's design follows its code: the code's tokens win,
// its components are listed for reuse and pages on its routes are marked
// existing.
func TestApplyDesignExisting(t *testing.T) {
	p := &core.Project{
		Platform: &core.PlatformPlan{Targets: []core.PlatformTarget{{Kind: core.PlatformWeb}}},
		Structure: &core.Structure{Mode: core.StructureExisting, UI: &core.ExistingUI{
			Routes:       []string{"/orders"},
			Components:   []string{"Sidebar (web/src/components/Sidebar.tsx)", "DataTable (web/src/components/DataTable.tsx)"},
			Tokens:       &core.DesignTokens{Colors: []core.Token{{Name: "action.primary", Value: "#7c3aed", Use: "dari web/src/index.css"}}},
			TokenSources: []string{"web/src/index.css"},
		}},
	}
	if err := ApplyDesign(p, existingDesign); err != nil {
		t.Fatal(err)
	}
	r := p.UI.Rules
	if r.Source != core.RulesExisting || len(r.Tokens.Colors) != 2 || r.Tokens.Colors[0].Value != "#7c3aed" || r.Tokens.Colors[1].Name != "status.info" {
		t.Fatalf("tokens = %+v", r.Tokens.Colors)
	}
	if len(r.Components) != 2 || r.Components[0].Source != "web/src/components/Sidebar.tsx" || r.Components[1].Name != "DataTable" {
		t.Fatalf("components = %+v", r.Components)
	}
	if pg := p.UI.Layout.Pages; !pg[0].Existing || pg[1].Existing {
		t.Fatalf("existing = %v %v", pg[0].Existing, pg[1].Existing)
	}
	if !strings.Contains(DesignPrompt(p), "CURRENT LAYOUT SPECIFICATION") {
		t.Fatal("a written design is revised, not rewritten")
	}
}

func TestApplyDesignRefusesIncomplete(t *testing.T) {
	p := &core.Project{Platform: &core.PlatformPlan{Targets: []core.PlatformTarget{{Kind: core.PlatformWeb}}}}
	if err := ApplyDesign(p, `{"pages": []}`); err == nil || p.UI != nil {
		t.Fatalf("incomplete design err = %v ui = %+v", err, p.UI)
	}
	bad := strings.Replace(existingDesign, `"pattern": "detail"`, `"pattern": "hero"`, 1)
	err := ApplyDesign(p, bad)
	if err == nil || !strings.Contains(err.Error(), `pattern "hero" tidak dikenal`) || p.UI != nil {
		t.Fatalf("unknown pattern err = %v", err)
	}
}
