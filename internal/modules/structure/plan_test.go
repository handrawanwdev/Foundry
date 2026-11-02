package structure

import (
	"slices"
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
)

func planProject() *core.Project {
	p := project("Booking", "Pembayaran")
	p.Discovery = &core.Discovery{Summary: "Aplikasi booking", TechNeeds: []string{"transactions"}}
	p.Blueprint = &core.Blueprint{Modules: []core.Module{
		{ID: "booking", Responsibility: "Mengelola janji temu"},
		{ID: "pembayaran", Responsibility: "Mengelola pembayaran"},
	}}
	return p
}

func TestSourceHashTracksSystemNotStructure(t *testing.T) {
	p := planProject()
	h := SourceHash(p)
	p.Structure.IR = &core.StructureIR{Type: core.StructModular}
	if SourceHash(p) != h {
		t.Fatal("the structure itself must not change its source hash")
	}
	p.Blueprint.Modules[0].Responsibility = "Mengelola janji dan jadwal"
	if SourceHash(p) == h {
		t.Fatal("a blueprint change must change the source hash")
	}
}

func TestNormalizePlanComparesAlternatives(t *testing.T) {
	p := planProject()
	modular := &core.StructureIR{Type: core.StructModular,
		Apps: []core.StructurePart{{Name: "web", Reason: "Pasien dan admin memakai web"}},
		Services: []core.StructurePart{{Name: "api", Reason: "Satu API untuk kedua domain",
			Modules: []string{"booking", "pembayaran"},
			ModuleReasons: map[string]string{
				"booking":    "Janji temu dan jadwal",
				"pembayaran": "Tagihan dan status bayar",
			}}},
		Rules: []string{"Modul hanya memanggil modul lain lewat public API."},
	}
	services := &core.StructureIR{Type: core.StructMicroservices,
		Apps: []core.StructurePart{{Name: "web", Reason: "Pasien dan admin memakai web"}},
		Services: []core.StructurePart{
			{Name: "booking", Reason: "Janji temu diskalakan sendiri"},
			{Name: "pembayaran", Reason: "Pembayaran diisolasi"},
		}}
	plan := &core.StructurePlan{
		Summary: "Booking dan pembayaran saling terkait, volume awalnya kecil.",
		Domains: []core.StructureDomain{{Name: "booking", Responsibility: "Janji temu"},
			{Name: "pembayaran", Responsibility: "Pembayaran"}},
		DependencyRules: []string{"Tidak ada import melingkar antarmodul."},
		Recommendation:  "recommended",
		Alternatives: []core.StructureAlternative{
			{ID: "recommended", Name: "Modular", Summary: "Satu API, dua modul", Type: core.StructModular, IR: modular,
				Evaluation: core.StructureEvaluation{Maintainability: 5, Simplicity: 5, Extensibility: 3, Testability: 4, Separation: 4, Complexity: 2}},
			{ID: "alt-2", Name: "Microservices", Summary: "Satu service per domain", Type: core.StructMicroservices, IR: services,
				Evaluation: core.StructureEvaluation{Maintainability: 3, Simplicity: 2, Extensibility: 5, Testability: 3, Separation: 5, Complexity: 5}},
		},
	}
	if err := NormalizePlan(p, plan); err != nil {
		t.Fatal(err)
	}
	rec := plan.RecommendedAlternative()
	if rec == nil || rec.ID != "recommended" || !rec.Recommended || plan.Reason == "" {
		t.Fatalf("recommendation = %+v reason=%q", rec, plan.Reason)
	}
	if rec.Evaluation.Score <= plan.Alternatives[1].Evaluation.Score {
		t.Fatalf("scores: %+v", plan.Alternatives)
	}
	if !strings.Contains(rec.IR.Services[0].ModuleReasons["booking"], "Janji") {
		t.Fatalf("module reasons = %+v", rec.IR.Services[0].ModuleReasons)
	}
	ir := AlternativeIR(plan, rec)
	if !slices.Contains(ir.Rules, "Tidak ada import melingkar antarmodul.") {
		t.Fatalf("dependency rules not merged: %v", ir.Rules)
	}
}

func TestNormalizePlanRefusesFolderWithoutResponsibility(t *testing.T) {
	p := planProject()
	ir := &core.StructureIR{Type: core.StructModular,
		Apps:     []core.StructurePart{{Name: "web", Reason: "Web"}},
		Services: []core.StructurePart{{Name: "api", Reason: "API", Modules: []string{"booking", "pembayaran"}}}}
	// Module reasons are filled from the plan's domains, so remove those to
	// test the refusal.
	plan := &core.StructurePlan{Summary: "Sistem kecil", DependencyRules: []string{"Tidak ada import melingkar."},
		Alternatives: []core.StructureAlternative{
			{ID: "a", Name: "A", Type: core.StructModular, IR: ir},
			{ID: "b", Name: "B", Type: core.StructModular, IR: ir.Clone()},
		}}
	err := NormalizePlan(p, plan)
	if err == nil || !strings.Contains(err.Error(), "tanpa alasan") {
		t.Fatalf("err = %v", err)
	}
}

func TestPlanCurrent(t *testing.T) {
	p := planProject()
	p.Structure.Plan = &core.StructurePlan{SourceHash: SourceHash(p)}
	if !PlanCurrent(p.Structure, p) || PlanNeeded(p.Structure, p) {
		t.Fatal("a plan with the current hash is current")
	}
	p.Features = append(p.Features, core.Feature{Name: "Laporan"})
	if PlanCurrent(p.Structure, p) || !PlanNeeded(p.Structure, p) {
		t.Fatal("a changed system makes the plan stale")
	}
}
