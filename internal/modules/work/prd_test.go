package work

import (
	"strings"
	"testing"

	"github.com/handrawanw/foundry/internal/core"
)

func prdProject() *core.Project {
	return &core.Project{
		Name: "Klinik", Story: "Aplikasi janji temu klinik",
		Discovery: &core.Discovery{Summary: "Klinik", Business: "Janji temu",
			Users: []string{"Pasien", "Admin"},
			Roles: []core.Role{{Name: "Pasien", Description: "membuat janji", Platform: "web"}},
			Security: []string{"Login wajib"}},
		Requirements: []core.Requirement{
			{ID: "REQ-001", Text: "Pasien dapat membuat janji", Priority: "MUST"},
			{ID: "REQ-002", Text: "Admin dapat melihat daftar janji", Priority: "MUST"},
			{ID: "REQ-003", Text: "Pasien menerima pengingat", Priority: "SHOULD"},
		},
		Features: []core.Feature{
			{ID: "FEAT-001", Name: "Janji Temu", Description: "Pasien membuat janji",
				RequirementIDs: []string{"REQ-001"}, SubFeatures: []core.SubFeature{{Name: "Buat janji"}}},
			{ID: "FEAT-002", Name: "Daftar Janji", Description: "Admin melihat janji",
				RequirementIDs: []string{"REQ-002"}, SubFeatures: []core.SubFeature{{Name: "Lihat daftar"}}},
		},
		Flows: []core.Flow{
			{ID: "FLOW-001", Name: "Buat janji", FeatureID: "FEAT-001", Role: "Pasien",
				Steps: []string{"Pilih dokter", "Konfirmasi"}, PageIDs: []string{"PAGE-001"}},
		},
		Design: &core.Design{Pages: []core.Page{
			{ID: "PAGE-001", Name: "Buat Janji", Role: "Pasien", FeatureID: "FEAT-001",
				Route: "/janji/baru", Access: "auth", Data: &core.PageData{API: []string{"POST /appointments"}}},
			{ID: "PAGE-002", Name: "Daftar Janji", Role: "Admin", FeatureID: "FEAT-002",
				Route: "/janji", Access: "auth", Data: &core.PageData{API: []string{"GET /appointments"}}},
		}},
		Tasks: []core.Task{
			{ID: "TASK-001", Title: "Scaffold", Area: "scaffold", AcceptanceCriteria: []string{"build lulus"}, Status: core.TaskTodo},
			{ID: "TASK-002", Title: "API janji", Area: "api", FeatureID: "FEAT-001", RequirementIDs: []string{"REQ-001"},
				DependsOn: []string{"TASK-001"}, AcceptanceCriteria: []string{"POST /appointments"}, Files: []string{"backend/janji/api.go"},
				TestRequirements: []string{"unit test"}, Status: core.TaskTodo},
			{ID: "TASK-003", Title: "UI buat janji", Area: "ui", FeatureID: "FEAT-001", RequirementIDs: []string{"REQ-001"},
				PageIDs: []string{"PAGE-001"}, DependsOn: []string{"TASK-002"}, AcceptanceCriteria: []string{"halaman tampil"},
				Files: []string{"apps/web/src/pages/Janji.tsx"}, Status: core.TaskTodo},
		},
	}
}

func TestAnalyzePRDFindsGaps(t *testing.T) {
	p := prdProject()
	a := AnalyzePRD(p)
	has := func(kind, ref string) bool {
		for _, f := range a.Findings {
			if f.Kind == kind && f.Ref == ref {
				return true
			}
		}
		return false
	}
	if !has(FindMissing, "REQ-003") {
		t.Fatalf("REQ-003 should be MISSING_TASK: %+v", a.Findings)
	}
	if !has(FindMissing, "REQ-002") {
		t.Fatalf("REQ-002 should be MISSING_TASK: %+v", a.Findings)
	}
	if has(FindMissing, "FEAT-002") {
		t.Fatal("FEAT-002 is covered by REQ-002 MISSING_TASK")
	}
	if has(FindMissing, "PAGE-002") {
		t.Fatal("PAGE-002 is covered by REQ-002 MISSING_TASK")
	}
	if has(FindGAP, "TASK-001") {
		t.Fatal("scaffold must not be GAP")
	}
	if has(FindMissing, "REQ-001") {
		t.Fatal("REQ-001 is implemented")
	}
	if has(FindMissing, "TASK-001") {
		t.Fatal("existing task is not MISSING_TASK")
	}
	var linked bool
	for _, l := range a.Links {
		if strings.Contains(l.Requirement, "REQ-001") && l.TaskID == "TASK-002" && l.Capability == "Janji Temu" {
			linked = true
		}
	}
	if !linked {
		t.Fatalf("REQ-001 not traced to TASK-002: %+v", a.Links)
	}
}

func TestAnalyzePRDFlagsOverlapAndInvalidDep(t *testing.T) {
	p := prdProject()
	p.Tasks = append(p.Tasks,
		core.Task{ID: "TASK-004", Title: "API janji copy", Area: "api", FeatureID: "FEAT-001", RequirementIDs: []string{"REQ-001"},
			AcceptanceCriteria: []string{"ok"}, Files: []string{"backend/janji/api.go"}, Status: core.TaskTodo},
		core.Task{ID: "TASK-005", Title: "Orphan", Area: "api", DependsOn: []string{"TASK-999"},
			AcceptanceCriteria: []string{"ok"}, Status: core.TaskTodo},
		core.Task{ID: "TASK-006", Title: "Cycle A", Area: "api", FeatureID: "FEAT-001", RequirementIDs: []string{"REQ-001"},
			DependsOn: []string{"TASK-007"}, AcceptanceCriteria: []string{"ok"}, Status: core.TaskTodo},
		core.Task{ID: "TASK-007", Title: "Cycle B", Area: "api", FeatureID: "FEAT-001", RequirementIDs: []string{"REQ-001"},
			DependsOn: []string{"TASK-006"}, AcceptanceCriteria: []string{"ok"}, Status: core.TaskTodo},
	)
	a := AnalyzePRD(p)
	has := func(kind, ref string) bool {
		for _, f := range a.Findings {
			if f.Kind == kind && (ref == "" || f.Ref == ref) {
				return true
			}
		}
		return false
	}
	if !has(FindOverlap, "TASK-004") {
		t.Fatalf("shared file without dep: %+v", a.Findings)
	}
	if !has(FindInvalidDep, "TASK-005") {
		t.Fatalf("missing dep: %+v", a.Findings)
	}
	if !has(FindGAP, "TASK-005") {
		t.Fatalf("orphan task: %+v", a.Findings)
	}
	if !has(FindInvalidDep, "") && !has(FindInvalidDep, "TASK-006") && !has(FindInvalidDep, "TASK-007") {
		t.Fatalf("cycle: %+v", a.Findings)
	}
}

func TestAnalyzePRDMissingUIWhenAPIExists(t *testing.T) {
	p := prdProject()
	p.Tasks = p.Tasks[:2]
	a := AnalyzePRD(p)
	has := func(kind, ref string) bool {
		for _, f := range a.Findings {
			if f.Kind == kind && f.Ref == ref {
				return true
			}
		}
		return false
	}
	if has(FindMissing, "REQ-001") {
		t.Fatal("REQ-001 still has API task")
	}
	if !has(FindMissing, "PAGE-001") {
		t.Fatalf("PAGE-001 needs UI task: %+v", a.Findings)
	}
}

func TestAnalyzePRDEmptyDependsOnIsNotInvalid(t *testing.T) {
	p := prdProject()
	p.Tasks[0].DependsOn = nil
	a := AnalyzePRD(p)
	for _, f := range a.Findings {
		if f.Kind == FindInvalidDep && f.Ref == "TASK-001" {
			t.Fatalf("null depends_on flagged: %+v", a.Findings)
		}
	}
}

func TestPRDPromptUsesAnalysisNotTaskFirst(t *testing.T) {
	got := PRDPrompt(prdProject())
	for _, s := range []string{
		"product analyst of Foundry", "TRACEABILITY ANALYSIS", "Product → Capability",
		"TASK-001 (area scaffold) is technical foundation", "1. Product Overview", "18. Task Breakdown",
		"Do not invent", "REQ-003", "UNASSIGNED", "never MISSING_TASK",
		"Empty or null depends_on is not INVALID_DEPENDENCY",
	} {
		if !strings.Contains(got, s) {
			t.Fatalf("prompt missing %q", s)
		}
	}
}

func TestApplyPRDMergesFindingsAndRejectsGaps(t *testing.T) {
	p := prdProject()
	if err := ApplyPRD(p, `{"markdown":"# x"}`); err == nil {
		t.Fatal("short markdown accepted")
	}
	md := prdMarkdown()
	raw := "```json\n" + `{"markdown":` + jsonString(md) + `,"findings":[{"kind":"GAP","ref":"TASK-999","message":"invented"},{"kind":"GAP","ref":"TASK-002","message":"API harus validasi jadwal bentrok"},{"kind":"MISSING_TASK","ref":"TASK-001","message":"wire.go task komposisi"},{"kind":"UNASSIGNED","ref":"TASK-001","message":"wire.go bagian scaffold"}]}` + "\n```"
	if err := ApplyPRD(p, raw); err != nil {
		t.Fatal(err)
	}
	if p.PRD == nil || !strings.Contains(p.PRD.Markdown, "1. Product Overview") {
		t.Fatalf("prd = %+v", p.PRD)
	}
	var sawMissing, sawInvented, sawAI, sawWireMissing, sawUnassigned bool
	for _, f := range p.PRD.Findings {
		if f.Kind == FindMissing && f.Ref == "REQ-003" {
			sawMissing = true
		}
		if f.Ref == "TASK-999" {
			sawInvented = true
		}
		if f.Ref == "TASK-002" && strings.Contains(f.Message, "jadwal") {
			sawAI = true
		}
		if f.Kind == FindMissing && f.Ref == "TASK-001" {
			sawWireMissing = true
		}
		if f.Kind == FindUnassigned && f.Ref == "TASK-001" && strings.Contains(f.Message, "wire.go") {
			sawUnassigned = true
		}
	}
	if !sawMissing {
		t.Fatalf("deterministic finding dropped: %+v", p.PRD.Findings)
	}
	if sawInvented {
		t.Fatal("unknown ref kept")
	}
	if !sawAI {
		t.Fatalf("valid AI finding dropped: %+v", p.PRD.Findings)
	}
	if sawWireMissing {
		t.Fatal("MISSING_TASK on existing TASK-ID kept")
	}
	if !sawUnassigned {
		t.Fatalf("UNASSIGNED wire.go dropped: %+v", p.PRD.Findings)
	}
}

func prdMarkdown() string {
	var b strings.Builder
	for _, s := range prdSections {
		b.WriteString("# " + s + "\nisi\n\n")
	}
	return b.String()
}

func jsonString(s string) string { return mustJSON(s) }
