package pipeline_test

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/spec"
	"github.com/handrawanwdev/foundry/internal/modules/structure"
	"github.com/handrawanwdev/foundry/internal/modules/work"
)

// aiPlan builds a plan from the project itself, so the test follows the
// domains and platform discovery actually produced.
func aiPlan(p *core.Project) core.StructurePlan {
	domains := structure.Domains(p)
	reasons := map[string]string{}
	if p.Blueprint != nil {
		for _, m := range p.Blueprint.Modules {
			reasons[m.ID] = m.Responsibility
		}
	}
	moduleReasons := map[string]string{}
	for _, d := range domains {
		moduleReasons[d] = reasons[d]
		if moduleReasons[d] == "" {
			moduleReasons[d] = "Domain " + d
		}
	}
	apps := []core.StructurePart{}
	if p.Stack != nil && p.Stack.Platform.Web {
		apps = append(apps, core.StructurePart{Name: "web", Reason: "Admin dan pengguna memakai web"})
	}
	if p.Stack != nil && p.Stack.Platform.Mobile {
		apps = append(apps, core.StructurePart{Name: "mobile", Reason: "Pasien memakai aplikasi mobile"})
	}
	oneAPI := &core.StructureIR{Type: core.StructMonorepo, Apps: apps,
		Services: []core.StructurePart{{Name: "api", Reason: "Satu API melayani semua domain", Modules: domains, ModuleReasons: moduleReasons}},
		Packages: []core.StructurePart{
			{Name: "domain", Reason: "Entity dan aturan domain tanpa I/O"},
			{Name: "contracts", Reason: "Kontrak API dan event bersama"},
		},
		Rules: []string{"Modul hanya memanggil modul lain lewat public API."},
	}
	var services []core.StructurePart
	for _, d := range domains {
		services = append(services, core.StructurePart{Name: d, Reason: "Domain " + d + " dipisah"})
	}
	separate := &core.StructureIR{Type: core.StructMonorepo, Apps: apps, Services: services,
		Packages: []core.StructurePart{{Name: "contracts", Reason: "Kontrak API bersama"}}}
	planDomains := []core.StructureDomain{}
	for _, d := range domains {
		planDomains = append(planDomains, core.StructureDomain{Name: d, Responsibility: moduleReasons[d]})
	}
	return core.StructurePlan{
		Summary: "Sistem klinik dengan domain yang saling terkait dan volume awal kecil.",
		Drivers: []string{"Domain bisnis jelas", "Satu tim kecil", "Volume awal rendah"},
		Domains: planDomains,
		DependencyRules: []string{
			"Modul hanya boleh memanggil modul lain lewat public API atau event.",
			"Tidak ada import melingkar antarmodul.",
		},
		Recommendation: "recommended",
		Reason:         "Satu API dengan modul per domain paling sederhana dan tetap memisahkan tanggung jawab.",
		Alternatives: []core.StructureAlternative{
			{ID: "recommended", Name: "Modular dalam satu API", Summary: "Satu deploy, modul per domain.", Type: core.StructMonorepo, IR: oneAPI,
				Evaluation: core.StructureEvaluation{Maintainability: 5, Simplicity: 5, Extensibility: 3, Testability: 4, Separation: 4, Complexity: 2},
				Pros:       []string{"Sedikit bagian bergerak", "Batas domain tetap jelas"},
				Cons:       []string{"Satu deploy untuk semua domain"},
				Tradeoffs:  []string{"Skala per domain tidak independen"}},
			{ID: "alt-2", Name: "Service per domain", Summary: "Domain dipisah jadi service sendiri.", Type: core.StructMonorepo, IR: separate,
				Evaluation: core.StructureEvaluation{Maintainability: 3, Simplicity: 2, Extensibility: 5, Testability: 3, Separation: 5, Complexity: 5},
				Pros:       []string{"Isolasi domain"},
				Cons:       []string{"Biaya deploy dan observability lebih tinggi"},
				Tradeoffs:  []string{"Butuh kontrak jaringan dan tracing"}},
		},
	}
}

// TestStructurePlanRetriesMissingDomain covers the agent path: an answer that
// forgets a blueprint domain is refused once, the refusal goes back into the
// prompt, and the corrected answer is accepted.
func TestStructurePlanRetriesMissingDomain(t *testing.T) {
	s := newService(t, fakeAI{}, "structure_invalid_once")
	designReady(t, s)
	if err := s.AnalyzeStructure(""); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p := load(t, s)
	if p.Structure.Plan == nil || p.Structure.PlanRequest != nil {
		t.Fatalf("analysis was not retried: plan=%v request=%+v job=%+v", p.Structure.Plan, p.Structure.PlanRequest, p.Job)
	}
	if p.Job == nil || p.Job.Status != core.JobDone {
		t.Fatalf("job = %+v", p.Job)
	}
	rec := p.Structure.Plan.RecommendedAlternative()
	if rec == nil {
		t.Fatalf("plan = %+v", p.Structure.Plan)
	}
	var placed []string
	for _, svc := range rec.IR.Services {
		placed = append(placed, svc.Name)
		placed = append(placed, svc.Modules...)
	}
	for _, d := range structure.Domains(p) {
		if !slices.Contains(placed, d) {
			t.Fatalf("domain %s not placed: %v", d, placed)
		}
	}
}

// TestStructurePlanRejectionIsFedBack covers the MCP path: the host receives
// the concrete error, and the next prompt carries the refusal so it can fix
// the answer instead of starting a fresh analysis.
func TestStructurePlanRejectionIsFedBack(t *testing.T) {
	s := newService(t, fakeAI{mcp: true}, "")
	if _, err := s.SubmitStory("Aplikasi klinik untuk janji temu pasien"); err != nil {
		t.Fatal(err)
	}
	mcpAfterStory(t, s)
	if err := s.SubmitWork(work.KindDiscovery, discoveryJSON); err != nil {
		t.Fatal(err)
	}
	if err := s.SubmitWork(work.KindDesign, designJSON); err != nil {
		t.Fatal(err)
	}
	holdAutoPlan(t, s)
	p := load(t, s)
	plan := aiPlan(p)
	domains := structure.Domains(p)
	drop := domains[len(domains)-1]
	alt := &plan.Alternatives[0]
	for i := range alt.IR.Services {
		alt.IR.Services[i].Modules = slices.DeleteFunc(alt.IR.Services[i].Modules, func(m string) bool { return m == drop })
		delete(alt.IR.Services[i].ModuleReasons, drop)
	}
	raw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AnalyzeStructure(""); err != nil {
		t.Fatal(err)
	}
	err = s.SubmitWork(work.KindStructure, string(raw))
	if err == nil || !strings.Contains(err.Error(), "tidak menempatkan domain") || !strings.Contains(err.Error(), drop) {
		t.Fatalf("err = %v", err)
	}
	p = load(t, s)
	if p.Structure.Plan != nil || p.Structure.PlanRequest == nil ||
		!strings.Contains(p.Structure.PlanRequest.Rejected, "tidak menempatkan domain") {
		t.Fatalf("rejection not kept: plan=%v request=%+v", p.Structure.Plan, p.Structure.PlanRequest)
	}
	w, err := s.NextWork()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(w.Prompt, "YOUR LAST ANSWER WAS REFUSED") || !strings.Contains(w.Prompt, drop) {
		t.Fatalf("retry prompt does not carry the refusal:\n%s", w.Prompt)
	}
	raw, err = json.Marshal(aiPlan(p))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SubmitWork(work.KindStructure, string(raw)); err != nil {
		t.Fatal(err)
	}
	p = load(t, s)
	if p.Structure.Plan == nil || p.Structure.PlanRequest != nil {
		t.Fatalf("corrected answer not accepted: %+v", p.Structure)
	}
}

func TestStructurePlanEndToEnd(t *testing.T) {
	s := newService(t, fakeAI{mcp: true}, "")
	if _, err := s.SubmitStory("Aplikasi klinik untuk janji temu pasien"); err != nil {
		t.Fatal(err)
	}
	mcpAfterStory(t, s)
	if err := s.SubmitWork(work.KindDiscovery, discoveryJSON); err != nil {
		t.Fatal(err)
	}
	if err := s.SubmitWork(work.KindDesign, designJSON); err != nil {
		t.Fatal(err)
	}
	holdAutoPlan(t, s)
	p := load(t, s)
	plan := aiPlan(p)
	raw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}

	// A fresh analysis is handed to the MCP host as a structure job.
	if err := s.AnalyzeStructure(""); err != nil {
		t.Fatal(err)
	}
	mustJob(t, load(t, s), work.KindStructure, core.JobWaiting)
	w, err := s.NextWork()
	if err != nil || w.Kind != work.KindStructure || !strings.Contains(w.Prompt, "software architect") ||
		!strings.Contains(w.Prompt, "2-3") || !strings.Contains(w.Prompt, "TECHNOLOGY DECISION") {
		t.Fatalf("work = %+v err=%v", w, err)
	}
	if err := s.SubmitWork(work.KindStructure, string(raw)); err != nil {
		t.Fatal(err)
	}
	p = load(t, s)
	if p.Structure.Plan == nil || p.Structure.PlanRequest != nil || p.Structure.Plan.SourceHash == "" {
		t.Fatalf("plan = %+v", p.Structure)
	}
	if p.Structure.Plan.RecommendedAlternative() == nil || len(p.Structure.Plan.Alternatives) != 2 {
		t.Fatalf("alternatives = %+v", p.Structure.Plan)
	}

	// The source did not change: the analysis is not repeated.
	if err := s.AnalyzeStructure(""); !errors.Is(err, structure.ErrPlanCurrent) {
		t.Fatalf("repeat analysis err = %v", err)
	}

	// The user picks the AI recommendation; it becomes the approved final
	// structure, with the dependency rules saved on the IR.
	p, err = s.UseStructureAlternative("")
	if err != nil {
		t.Fatal(err)
	}
	if p.Structure.State() != core.TechApproved || p.Structure.Plan.Selected != "recommended" {
		t.Fatalf("structure = %+v", p.Structure)
	}
	if p.Structure.IR.Type != core.StructMonorepo || !strings.Contains(strings.Join(p.Structure.IR.Rules, "; "), "import melingkar") {
		t.Fatalf("ir = %+v", p.Structure.IR)
	}
	if p.Structure.IR.Services[0].ModuleReasons["identity"] == "" {
		t.Fatalf("module reasons = %+v", p.Structure.IR.Services[0].ModuleReasons)
	}
	var technology core.Artifact
	for _, a := range spec.Artifacts(p) {
		if a.ID == "technology" {
			technology = a
		}
	}
	for _, want := range []string{"Rekomendasi AI", "Ringkasan analisis sistem", "Aturan dependency antarbagian", "import melingkar"} {
		if !strings.Contains(technology.Body, want) {
			t.Fatalf("technology artifact misses %q:\n%s", want, technology.Body)
		}
	}
}
