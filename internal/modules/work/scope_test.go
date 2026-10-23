package work

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
)

func scoped() *core.Project {
	return &core.Project{
		Requirements: []core.Requirement{{ID: "REQ-001", Text: "Buat janji", Priority: "MUST"}, {ID: "REQ-002", Text: "Batal janji", Priority: "SHOULD"}},
		Features:     []core.Feature{{ID: "FEAT-001", Name: "Janji", RequirementIDs: []string{"REQ-001", "REQ-002"}}},
		// An older flow: steps only, with a page link from the design.
		Flows: []core.Flow{{ID: "FLOW-001", Name: "Buat janji", FeatureID: "FEAT-001", Role: "Pasien", Steps: []string{"Pilih jadwal"}, PageIDs: []string{"jadwal"}}},
	}
}

// A blueprint revision changes use cases, acceptance, decisions and skips
// besides artifacts. What it touches must be valid; problems it did not
// touch stay blockers, not refusals.
func TestApplyArtifactsScope(t *testing.T) {
	p := scoped()
	bad := `{"use_cases": [{"id": "FLOW-001", "name": "Buat janji", "feature_id": "FEAT-001", "role": "Pasien", "steps": ["a"]}]}`
	err := ApplyArtifacts(p, bad)
	var invalid *core.Invalid
	if !errors.As(err, &invalid) || !strings.Contains(err.Error(), "use case tidak valid") || p.Flows[0].Goal != "" {
		t.Fatalf("incomplete use case err = %v", err)
	}
	raw := `{"use_cases": [{"id": "FLOW-001", "name": "Buat janji", "feature_id": "FEAT-001", "role": "Pasien", "goal": "Dapat jadwal",
	   "steps": ["Pilih jadwal", "Konfirmasi"], "postconditions": ["Janji pending"], "failures": ["Jadwal penuh"]},
	  {"id": "", "name": "Batal janji", "feature_id": "FEAT-001", "role": "Pasien", "goal": "Batal", "steps": ["Batalkan"],
	   "postconditions": ["Janji batal"], "alternatives": ["Sudah lewat: ditolak"]}],
	 "acceptance": [{"requirement": "req-001", "criteria": [{"kind": "Behavior", "text": "Janji tersimpan"}, {"id": "REQ-009-AC1", "kind": "negative", "text": "Bentrok ditolak", "evidence": "test"}]}],
	 "definition_of_done": ["Test lulus", " "],
	 "decisions": [{"kind": "question", "text": "Gateway pembayaran?", "critical": true, "area": "integration"}],
	 "skip": [{"area": "migration", "reason": "project baru"}]}`
	if err := ApplyArtifacts(p, raw); err != nil {
		t.Fatal(err)
	}
	if len(p.Flows) != 2 || p.Flows[1].ID != "FLOW-002" || !slices.Equal(p.Flows[0].PageIDs, []string{"jadwal"}) {
		t.Fatalf("flows = %+v", p.Flows)
	}
	ac := p.Requirements[0].Acceptance
	if len(ac) != 2 || ac[0].ID != "REQ-001-AC1" || ac[0].Kind != "behavior" || ac[1].ID != "REQ-001-AC2" {
		t.Fatalf("acceptance = %+v", ac)
	}
	if !slices.Equal(p.Done, []string{"Test lulus"}) || p.Decisions[0].ID != "DEC-001" || p.Skip(core.AreaMigration) != "project baru" {
		t.Fatalf("done %v decisions %+v skips %+v", p.Done, p.Decisions, p.Skips)
	}
	// REQ-002 has no criteria: a blocker, but this revision did not touch it.
	if errs := core.Errors(core.AcceptanceIssues(p.Requirements, p.Done)); len(errs) != 1 || errs[0].Where != "REQ-002" {
		t.Fatalf("errors = %+v", errs)
	}
	for raw, want := range map[string]string{
		`{"acceptance": [{"requirement": "REQ-404", "criteria": [{"text": "x"}]}]}`: "REQ-404",
		`{"acceptance": [{"requirement": "REQ-002"}]}`:                              "acceptance criteria tidak valid",
		`{"skip": [{"area": "acceptance", "reason": "malas"}]}`:                     "tidak bisa dilewati",
		`{"skip": [{"area": "security", "reason": " "}]}`:                           "alasan",
		`{"decisions": [{"kind": "tebakan", "text": "x"}]}`:                         "keputusan tidak valid",
		`{"remove_use_cases": ["FLOW-404"]}`:                                        "FLOW-404",
	} {
		if err := ApplyArtifacts(p, raw); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err = %v, want %q", raw, err, want)
		}
	}
	// The user decides the question; the AI may update it by id.
	if err := ResolveDecision(p, "DEC-001", " Midtrans "); err != nil || p.Decisions[0].Resolution != "Midtrans" {
		t.Fatalf("resolve = %v %+v", err, p.Decisions)
	}
	if err := ApplyArtifacts(p, `{"decisions": [{"id": "DEC-001", "kind": "question", "text": "Gateway pembayaran?", "critical": true, "resolution": "Midtrans sandbox dulu"}], "unskip": ["migration"]}`); err != nil {
		t.Fatal(err)
	}
	if len(p.Decisions) != 1 || p.Decisions[0].Resolution != "Midtrans sandbox dulu" || p.Skips != nil {
		t.Fatalf("decisions %+v skips %+v", p.Decisions, p.Skips)
	}
}

// Changing a decision queues a command rewrite; appending to an existing
// request still marks it as a sync so an empty answer is valid.
func TestRequestCommandSync(t *testing.T) {
	p := &core.Project{
		Decisions: []core.Decision{{ID: "DEC-001", Kind: core.DecisionQuestion, Text: "Gateway?", Resolution: "Midtrans"}},
		Workflow:  &core.Workflow{Commands: []core.Command{{ID: "CMD-001", Name: "x", Objective: "o", Instructions: "i"}}},
	}
	requestCommandSync(p, []string{"DEC-001"})
	r := p.Workflow.Request
	if r == nil || !r.Sync || !r.Cascade || !strings.Contains(r.Text, "Keputusan: Midtrans") {
		t.Fatalf("request = %+v", r)
	}
	requestCommandSync(p, []string{"DEC-001"})
	if !p.Workflow.Request.Sync || strings.Count(p.Workflow.Request.Text, "Keputusan: Midtrans") != 2 {
		t.Fatalf("append = %+v", p.Workflow.Request)
	}
}

func TestEditUseCasesAndAcceptance(t *testing.T) {
	p := scoped()
	// Unchanged older flows are kept as they are; a changed one must be valid.
	if err := EditUseCases(p, slices.Clone(p.Flows)); err != nil {
		t.Fatal(err)
	}
	edit := slices.Clone(p.Flows)
	edit[0].Name = "Ubah nama saja"
	edit[0].PageIDs = nil
	if err := EditUseCases(p, edit); err == nil || !strings.Contains(err.Error(), "tujuan belum diisi") {
		t.Fatalf("incomplete edit err = %v", err)
	}
	edit[0].Goal, edit[0].Post, edit[0].Alternatives = "Dapat jadwal", []string{"Janji pending"}, []string{"Jadwal penuh"}
	if err := EditUseCases(p, edit); err != nil {
		t.Fatal(err)
	}
	if p.Flows[0].Name != "Ubah nama saja" || !slices.Equal(p.Flows[0].PageIDs, []string{"jadwal"}) {
		t.Fatalf("flows = %+v", p.Flows)
	}
	err := EditAcceptance(p, []AcceptanceIn{{Requirement: "REQ-001", Criteria: []core.Criterion{{Kind: "edge", Text: "Jam lewat ditolak"}}},
		{Requirement: "REQ-002", Untestable: "kebijakan klinik"}}, []string{"Bukti test dilampirkan"})
	if err != nil {
		t.Fatal(err)
	}
	if errs := core.Errors(core.AcceptanceIssues(p.Requirements, p.Done)); len(errs) != 0 {
		t.Fatalf("errors = %+v", errs)
	}
	if err := EditAcceptance(p, nil, nil); err == nil || !strings.Contains(err.Error(), "Definition of Done") {
		t.Fatalf("empty DoD err = %v", err)
	}
}

func TestStampAndReviewArtifact(t *testing.T) {
	p := &core.Project{Artifacts: []core.Artifact{{ID: "threat-model", Title: "T", Body: "x", Refs: []string{"requirements"}}, {ID: "lain", Title: "L", Body: "y"}}}
	StampRefs(p, map[string]string{"requirements": "h1"})
	if p.Artifacts[0].Seen["requirements"] != "h1" || p.Artifacts[1].Seen != nil {
		t.Fatalf("stamped = %+v", p.Artifacts)
	}
	changed := map[string]string{"requirements": "h2"}
	if err := ReviewArtifact(p, "lain", changed); err == nil {
		t.Fatal("artifact without changed refs reviewed")
	}
	if err := ReviewArtifact(p, "threat-model", changed); err != nil || p.Artifacts[0].Seen["requirements"] != "h2" {
		t.Fatalf("review = %v %+v", err, p.Artifacts[0])
	}
	// A changed artifact records its refs again.
	if err := EditArtifact(p, "threat-model", core.Artifact{Title: "T", Body: "x2", Refs: []string{"requirements"}}); err != nil || p.Artifacts[0].Seen != nil {
		t.Fatalf("edit = %v %+v", err, p.Artifacts[0])
	}
	if p.Artifacts[0].Area != core.AreaSecurity {
		t.Fatalf("area of threat-model = %q", p.Artifacts[0].Area)
	}
}

// A new workflow must cover every MUST requirement; unknown requirement
// ids are refused.
func TestApplyWorkflowCovers(t *testing.T) {
	docs := []core.Artifact{{ID: "requirements", Title: "R", Body: "b"}}
	p := scoped()
	cmd := `{"id": "C1", "name": "a", "objective": "o", "instructions": "i", "artifacts": ["requirements"], "covers": [%s]}`
	for covers, want := range map[string]string{`"REQ-002"`: "REQ-001", `"REQ-001", "REQ-404"`: "REQ-404 tidak ada"} {
		err := ApplyWorkflow(p, `{"commands": [`+strings.Replace(cmd, "%s", covers, 1)+`]}`, docs)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("covers %s: err = %v", covers, err)
		}
	}
	if err := ApplyWorkflow(p, `{"commands": [`+strings.Replace(cmd, "%s", `"req-001"`, 1)+`]}`, docs); err != nil {
		t.Fatal(err)
	}
	if c := p.Workflow.Commands[0]; !slices.Equal(c.Covers, []string{"REQ-001"}) {
		t.Fatalf("covers = %v", c.Covers)
	}
}

func TestDiscoveryScope(t *testing.T) {
	p := &core.Project{Story: "x"}
	raw := `{"name": "Klinik", "discovery": {"summary": "s", "data": ["Appointment"]},
	 "requirements": [{"id": "R1", "text": "Buat janji", "priority": "MUST", "acceptance": [{"kind": "behavior", "text": "Tersimpan"}]}],
	 "features": [{"id": "F1", "name": "Janji", "requirement_ids": ["R1"], "platforms": ["backend"]}],
	 "flows": [{"id": "FL1", "name": "Buat", "feature_id": "F1", "role": "Pasien", "goal": "Jadwal", "steps": ["a"], "postconditions": ["b"], "failures": ["c"], "page_ids": ["x"]}],
	 "platform": {"targets": [{"kind": "backend"}]},
	 "artifacts": [{"id": "operations", "title": "Operasional", "body": "## Health"}],
	 "definition_of_done": ["Test lulus"],
	 "decisions": [{"id": "D9", "kind": "aneh", "text": "Satu zona waktu", "area": "planet"}, {"kind": "risk", "text": " "}],
	 "skip": [{"area": "integration", "reason": "tidak ada sistem luar"}, {"area": "ui", "reason": "x"}]}`
	if err := ApplyDiscovery(p, raw); err != nil {
		t.Fatal(err)
	}
	r := p.Requirements[0]
	if r.ID != "REQ-001" || len(r.Acceptance) != 1 || r.Acceptance[0].ID != "REQ-001-AC1" {
		t.Fatalf("requirement = %+v", r)
	}
	if f := p.Flows[0]; f.ID != "FLOW-001" || f.FeatureID != "FEAT-001" || f.Goal != "Jadwal" || f.PageIDs != nil {
		t.Fatalf("flow = %+v", f)
	}
	if len(p.Decisions) != 1 || p.Decisions[0].ID != "DEC-001" || p.Decisions[0].Kind != core.DecisionAssumption || p.Decisions[0].Area != "" {
		t.Fatalf("decisions = %+v", p.Decisions)
	}
	if len(p.Skips) != 1 || p.Skips[0].Area != core.AreaIntegration || p.Artifacts[0].Area != core.AreaOperations || !slices.Equal(p.Done, []string{"Test lulus"}) {
		t.Fatalf("skips %+v artifacts %+v done %v", p.Skips, p.Artifacts, p.Done)
	}
	if prompt := DiscoveryPrompt(p); !strings.Contains(prompt, "threat-model, area security") || !strings.Contains(prompt, `"definition_of_done"`) ||
		!strings.Contains(prompt, `"postconditions"`) {
		t.Fatal("discovery prompt lacks the blueprint areas")
	}
}
