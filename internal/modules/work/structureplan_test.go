package work

import (
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
)

func planProject() *core.Project {
	p := &core.Project{
		Name:  "Klinik",
		Story: "Aplikasi klinik untuk janji temu",
		Platform: &core.PlatformPlan{Targets: []core.PlatformTarget{
			{Kind: core.PlatformWeb, Rationale: "admin"},
			{Kind: core.PlatformBackend, Rationale: "API"},
		}},
		Discovery: &core.Discovery{Summary: "Klinik pratama", Business: "Janji temu pasien"},
		Blueprint: &core.Blueprint{Modules: []core.Module{
			{ID: "booking", Responsibility: "Janji temu pasien"},
			{ID: "billing", Responsibility: "Tagihan pasien"},
		}},
		Features: []core.Feature{{ID: "FEAT-001", Name: "Janji Temu", RequirementIDs: []string{"REQ-001"}}},
		Structure: &core.Structure{Mode: core.StructureNew,
			IR: &core.StructureIR{Type: core.StructModular,
				Apps: []core.StructurePart{{Name: "web", Reason: "Admin memakai web"}},
				Services: []core.StructurePart{{Name: "api", Reason: "API tunggal",
					Modules:       []string{"booking", "billing"},
					ModuleReasons: map[string]string{"booking": "Janji", "billing": "Tagihan"}}}},
			PlanRequest: &core.StructurePlanRequest{SourceHash: "abc"},
		},
	}
	return p
}

const structurePlanJSON = `{
 "summary": "Klinik dengan dua domain yang saling terkait.",
 "drivers": ["Dua domain bisnis", "Volume awal kecil"],
 "domains": [{"name": "booking", "responsibility": "Janji temu pasien"}, {"name": "billing", "responsibility": "Tagihan pasien"}],
 "dependency_rules": ["Modul hanya memanggil modul lain lewat public API."],
 "recommendation": "recommended",
 "reason": "Satu API modular paling sederhana dan tetap memisahkan domain.",
 "alternatives": [
  {"id": "recommended", "name": "Modular per domain", "summary": "Satu API dengan dua modul.", "type": "modular",
   "ir": {"type": "modular", "apps": [{"name": "web", "reason": "Admin memakai web"}],
          "services": [{"name": "api", "reason": "Satu API", "modules": ["booking", "billing"],
                        "module_reasons": {"booking": "Janji temu pasien", "billing": "Tagihan pasien"}}],
          "docs": "docs/", "rules": ["Setiap modul punya public API."]},
   "evaluation": {"maintainability": 5, "simplicity": 5, "extensibility": 3, "testability": 4, "separation_of_concerns": 4, "complexity": 2},
   "pros": ["Sedikit bagian bergerak"], "cons": ["Satu deploy"], "tradeoffs": ["Skala per domain tidak independen"]},
  {"id": "alt-2", "name": "Dua service", "summary": "Booking dan billing dipisah.", "type": "microservices",
   "ir": {"type": "microservices", "apps": [{"name": "web", "reason": "Admin memakai web"}],
          "services": [{"name": "booking", "reason": "Janji temu"}, {"name": "billing", "reason": "Tagihan"}],
          "docs": "docs/"},
   "evaluation": {"maintainability": 3, "simplicity": 2, "extensibility": 5, "testability": 3, "separation_of_concerns": 5, "complexity": 5},
   "pros": ["Isolasi"], "cons": ["Operasional lebih mahal"], "tradeoffs": ["Butuh antrean dan tracing"]}
 ]
}`

func TestStructurePlanPromptAndApply(t *testing.T) {
	p := planProject()
	prompt := StructurePlanPrompt(p)
	for _, want := range []string{"software architect", "2-3", "booking", "TARGET PLATFORM", "DOMAIN MODULES", "CURRENT STRUCTURE"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt misses %q:\n%s", want, prompt)
		}
	}
	if err := ApplyStructurePlan(p, structurePlanJSON); err != nil {
		t.Fatal(err)
	}
	plan := p.Structure.Plan
	if plan == nil || plan.SourceHash != "abc" || len(plan.Alternatives) != 2 || plan.Recommendation != "recommended" {
		t.Fatalf("plan = %+v", plan)
	}
	if p.Structure.PlanRequest != nil || p.Structure.Recommended != core.StructModular {
		t.Fatalf("structure = %+v", p.Structure)
	}
	if plan.Selected != "" || plan.GeneratedAt.IsZero() {
		t.Fatalf("plan selection/time = %+v", plan)
	}
}

func TestStructurePlanRefusesIncompleteAlternative(t *testing.T) {
	p := planProject()
	bad := strings.Replace(structurePlanJSON, `"domains": [{"name": "booking", "responsibility": "Janji temu pasien"}, {"name": "billing", "responsibility": "Tagihan pasien"}],`, `"domains": [],`, 1)
	bad = strings.Replace(bad, `"module_reasons": {"booking": "Janji temu pasien", "billing": "Tagihan pasien"}`, `"module_reasons": {}`, 1)
	if err := ApplyStructurePlan(p, bad); err == nil {
		t.Fatal("alternative without module responsibility accepted")
	}
	if p.Structure.Plan != nil || p.Structure.PlanRequest == nil {
		t.Fatalf("failed answer changed the structure: %+v", p.Structure)
	}
}

func TestWriteStructureBriefCarriesRules(t *testing.T) {
	p := planProject()
	p.Structure.IR.Rules = []string{"apps/web hanya memakai packages/contracts."}
	var b strings.Builder
	writeStructureBrief(&b, p, "STRUCTURE")
	got := b.String()
	if !strings.Contains(got, "apps/web/") || !strings.Contains(got, "apps/web hanya memakai") || !strings.Contains(got, "Janji") {
		t.Fatalf("brief = %s", got)
	}
}
