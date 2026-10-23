package core

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func scopeProject() *Project {
	return &Project{
		Discovery:    &Discovery{Data: []string{"Appointment: id"}},
		Platform:     &PlatformPlan{Targets: []PlatformTarget{{Kind: PlatformBackend}}},
		Requirements: []Requirement{{ID: "REQ-001", Text: "Buat janji", Priority: "MUST"}},
		Features:     []Feature{{ID: "FEAT-001", Name: "Janji", RequirementIDs: []string{"REQ-001"}}},
		Flows:        []Flow{{ID: "FLOW-001", Name: "Buat janji", FeatureID: "FEAT-001", Role: "Pasien", Steps: []string{"Pilih jadwal"}}},
	}
}

func statusOf(areas []AreaStatus, id string) AreaStatus {
	i := slices.IndexFunc(areas, func(a AreaStatus) bool { return a.ID == id })
	return areas[i]
}

// Areas are adaptive: what does not apply needs nothing, what applies is
// covered, skipped with a reason, or still missing.
func TestCoverage(t *testing.T) {
	p := scopeProject()
	p.Skips = []AreaSkip{{Area: AreaOperations, Reason: "prototipe"}, {Area: AreaAcceptance, Reason: "tidak bisa dilewati"}}
	docs := []Artifact{{ID: ArtifactUseCases, Builtin: true}, {ID: "keamanan", Area: AreaSecurity}}
	got := Coverage(p, docs)
	want := map[string]string{
		AreaUI: AreaNotApplicable, AreaUseCases: AreaInvalid, AreaData: AreaMissing, AreaSecurity: AreaCovered,
		AreaIntegration: AreaNotApplicable, AreaOperations: AreaSkipped, AreaMigration: AreaNotApplicable, AreaAcceptance: AreaMissing,
	}
	for id, st := range want {
		if a := statusOf(got, id); a.Status != st {
			t.Errorf("%s = %s (%+v), want %s", id, a.Status, a, st)
		}
	}
	if a := statusOf(got, AreaOperations); a.Reason != "prototipe" {
		t.Errorf("skip reason = %q", a.Reason)
	}
	// A complete use case and acceptance criteria cover their areas.
	f := &p.Flows[0]
	f.Goal, f.Post, f.Failures = "Dapat jadwal", []string{"Janji tersimpan"}, []string{"Jadwal penuh"}
	p.Requirements[0].Acceptance = []Criterion{{ID: "REQ-001-AC1", Kind: "behavior", Text: "Janji tersimpan"}}
	p.Done = []string{"Test lulus"}
	docs = append(docs, Artifact{ID: ArtifactAcceptance, Builtin: true})
	got = Coverage(p, docs)
	for _, id := range []string{AreaUseCases, AreaAcceptance} {
		if a := statusOf(got, id); a.Status != AreaCovered {
			t.Errorf("%s = %+v", id, a)
		}
	}
	p.Structure = &Structure{Mode: StructureExisting}
	if a := statusOf(Coverage(p, docs), AreaMigration); a.Status != AreaMissing {
		t.Errorf("existing project migration = %+v", a)
	}
}

func TestUseCaseIssues(t *testing.T) {
	p := scopeProject()
	flows := append(p.Flows, Flow{ID: "FLOW-001", Name: "Dobel", FeatureID: "FEAT-404"})
	issues := p.UseCaseIssues(flows)
	for _, msg := range []string{"tujuan belum diisi", "postcondition (kondisi berhasil) belum diisi", "skenario alternatif atau kegagalan belum diisi",
		"id dipakai lebih dari satu use case", `fitur "FEAT-404" tidak ada`, "aktor belum diisi", "alur utama belum diisi"} {
		if !slices.ContainsFunc(issues, func(i Issue) bool { return i.Message == msg && i.Where == "FLOW-001" }) {
			t.Errorf("missing %q in %+v", msg, issues)
		}
	}
}

func TestAcceptanceIssues(t *testing.T) {
	reqs := []Requirement{
		{ID: "REQ-001", Priority: "MUST", Acceptance: []Criterion{{ID: "REQ-001-AC1", Kind: "behavior", Text: "ok"}}},
		{ID: "REQ-002", Priority: "SHOULD"},
		{ID: "REQ-003", Priority: "COULD", Untestable: "keputusan bisnis, bukan perilaku"},
		{ID: "REQ-004", Priority: "MUST", Acceptance: []Criterion{{ID: "REQ-004-AC1", Kind: "gaya", Text: "x"}, {ID: "REQ-004-AC1", Kind: "edge", Text: " "}}},
	}
	issues := AcceptanceIssues(reqs, nil)
	has := func(level, where, msg string) bool {
		return slices.ContainsFunc(issues, func(i Issue) bool { return i.Level == level && i.Where == where && i.Message == msg })
	}
	switch {
	case !has(IssueWarning, "REQ-001", "belum ada negative atau edge case"):
		t.Error("MUST without negative/edge case not warned")
	case !has(IssueError, "REQ-002", "belum punya acceptance criteria (atau alasan tidak dapat diuji)"):
		t.Error("requirement without criteria accepted")
	case slices.ContainsFunc(issues, func(i Issue) bool { return i.Where == "REQ-003" }):
		t.Error("untestable requirement with a reason refused")
	case !has(IssueError, "REQ-004", `id kriteria "REQ-004-AC1" kosong atau ganda`), !has(IssueError, "REQ-004", "kriteria REQ-004-AC1 kosong"):
		t.Errorf("criteria problems = %+v", issues)
	case !has(IssueError, ArtifactAcceptance, "Definition of Done belum diisi"):
		t.Error("missing Definition of Done accepted")
	}
}

// A critical open decision is what the user must decide; deciding it
// clears it.
func TestDecisions(t *testing.T) {
	p := &Project{Decisions: []Decision{
		{ID: "DEC-001", Kind: DecisionQuestion, Text: "Gateway pembayaran?", Critical: true},
		{ID: "DEC-002", Kind: DecisionAssumption, Text: "Satu zona waktu"},
		{ID: "DEC-002", Kind: "tebakan", Text: " ", Area: "x"},
	}}
	if got := p.OpenCritical(); len(got) != 1 || got[0].ID != "DEC-001" {
		t.Fatalf("open critical = %+v", got)
	}
	if issues := DecisionIssues(p.Decisions); len(issues) != 4 {
		t.Fatalf("issues = %+v", issues)
	}
	p.Decisions[0].Resolution = "Midtrans"
	if len(p.OpenCritical()) != 0 {
		t.Fatal("decided question still open")
	}
	got := p.BindingDecisions()
	if len(got) != 1 || got[0].ID != "DEC-001" {
		t.Fatalf("binding = %+v", got)
	}
	text := BindingText(p.Decisions)
	if !strings.Contains(text, "DEC-001") || !strings.Contains(text, "Keputusan: Midtrans") || strings.Contains(text, "DEC-002") {
		t.Fatalf("binding text = %s", text)
	}
}

// A written artifact is marked for review when an artifact it refers to
// changes after it recorded the hash; refs it never recorded are not.
func TestArtifactStates(t *testing.T) {
	req := Artifact{ID: "requirements", Title: "R", Body: "v1", Builtin: true}
	tm := Artifact{ID: "threat-model", Title: "T", Body: "x", Refs: []string{"requirements", "api"}, Seen: map[string]string{"requirements": req.Hash()}}
	p := scopeProject()
	if st := ArtifactStates(p, []Artifact{req, tm})["threat-model"]; st.Status != StateValid {
		t.Fatalf("unchanged = %+v", st)
	}
	req.Body = "v2"
	st := ArtifactStates(p, []Artifact{req, tm})["threat-model"]
	if st.Status != StateNeedsReview || st.Reason != "artefak acuan berubah: requirements" {
		t.Fatalf("changed = %+v", st)
	}
	if st := ArtifactStates(p, []Artifact{{ID: ArtifactUseCases, Builtin: true}})[ArtifactUseCases]; st.Status != StateDraft {
		t.Fatalf("invalid use cases = %+v", st)
	}
	p.Stage = StageReady
	if st := ArtifactStates(p, []Artifact{req, tm})["threat-model"]; st.Status != StateFinalized {
		t.Fatalf("finalized = %+v", st)
	}
}

// Written artifacts that took an id now used by a built-in artifact are
// renamed on load; command refs follow, so nothing is lost or stale.
func TestLoadRenamesArtifactsOfBuiltinIDs(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, StateDir), 0o755); err != nil {
		t.Fatal(err)
	}
	old := `{"name":"klinik","stage":"DESIGN","story":"x",
		"requirements":[{"id":"REQ-001","text":"Buat janji","priority":"MUST"}],
		"flows":[{"id":"FLOW-001","name":"Janji","feature_id":"FEAT-001","role":"Pasien","steps":["a"],"page_ids":null}],
		"artifacts":[{"id":"use-cases","title":"Use Case","category":"domain","body":"isi","version":1},
		             {"id":"api","title":"API","category":"data","body":"b","refs":["use-cases"],"version":1}],
		"workflow":{"status":"draft","version":1,"commands":[{"id":"CMD-001","name":"n","objective":"o","instructions":"i","status":"draft",
		  "artifacts":[{"id":"use-cases","hash":"h1"}]}]}}`
	if err := os.WriteFile(filepath.Join(dir, StateDir, "project.json"), []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := (&Store{Dir: dir}).Load()
	if err != nil {
		t.Fatal(err)
	}
	if p.Artifacts[0].ID != "use-cases-doc" || p.Artifacts[1].Refs[0] != "use-cases-doc" {
		t.Fatalf("artifacts = %+v", p.Artifacts)
	}
	if r := p.Workflow.Commands[0].Artifacts[0]; r.ID != "use-cases-doc" || r.Hash != "h1" {
		t.Fatalf("command ref = %+v", r)
	}
	// Older requirements and flows load without the new fields; they show
	// up as what is still to write, nothing is invented.
	if len(p.Requirements[0].Acceptance) != 0 || p.Flows[0].Goal != "" || p.Decisions != nil {
		t.Fatalf("invented data: %+v %+v", p.Requirements, p.Flows)
	}
}
