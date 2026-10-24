package work

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/handrawanwdev/foundry/internal/core"
)

func TestApplyArtifacts(t *testing.T) {
	p := &core.Project{}
	SetArtifacts(p, []core.Artifact{{Title: "Model Domain", Body: "x"}, {ID: "model-domain", Title: "Lagi", Body: "y"},
		{ID: "architecture", Title: "Bentrok", Body: "z"}, {ID: "kosong", Title: "Kosong"}})
	if len(p.Artifacts) != 2 || p.Artifacts[0].ID != "model-domain" || p.Artifacts[1].ID != "model-domain-2" || p.Artifacts[0].Category != core.ArtifactOther {
		t.Fatalf("set = %+v", p.Artifacts)
	}
	for raw, want := range map[string]string{
		`{"artifacts": [{"id": "layout-spec", "title": "x", "body": "y"}]}`:     "bawaan",
		`{"artifacts": [{"id": "system-overview", "title": "x", "body": "y"}]}`: "bawaan",
		`{"remove": ["requirements"]}`:                                          "bawaan",
		`{"remove": ["ghost"]}`:                                                 "tidak ada",
		`{"artifacts": []}`:                                                     "tidak menemukan perubahan",
	} {
		if err := ApplyArtifacts(p, raw); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err = %v, want %q", raw, err, want)
		}
	}
	if err := ApplyArtifacts(p, `{"artifacts": [{"id": "model-domain", "title": "Model Domain", "category": "domain", "body": "x2"}], "remove": ["model-domain-2"]}`); err != nil {
		t.Fatal(err)
	}
	if len(p.Artifacts) != 1 || p.Artifacts[0].Version != 2 || p.Artifacts[0].Body != "x2" || p.Artifacts[0].Category != core.ArtifactDomain {
		t.Fatalf("revised = %+v", p.Artifacts)
	}
}

// Every command must point at a blueprint artifact that exists, with the
// hash it was written against.
func TestApplyWorkflowRefs(t *testing.T) {
	docs := []core.Artifact{{ID: "requirements", Title: "R", Body: "b"}}
	p := &core.Project{}
	err := ApplyWorkflow(p, `{"commands": [{"id": "C1", "name": "a", "objective": "o", "instructions": "i", "artifacts": ["ghost"]}]}`, docs)
	if err == nil || !strings.Contains(err.Error(), "ghost") || p.Workflow != nil {
		t.Fatalf("err = %v workflow = %+v", err, p.Workflow)
	}
	if err := ApplyWorkflow(p, `{"commands": [{"id": "C1", "name": "a", "objective": "o", "instructions": "i", "artifacts": ["requirements"]},
		{"id": "C2", "name": "b", "objective": "o", "instructions": "i", "depends_on": ["C1"], "artifacts": ["requirements"]}]}`, docs); err != nil {
		t.Fatal(err)
	}
	c := p.Workflow.Commands[1]
	if c.ID != "CMD-002" || c.DependsOn[0] != "CMD-001" || c.Artifacts[0].Hash != docs[0].Hash() || c.Status != core.CommandDraft {
		t.Fatalf("command = %+v", c)
	}
}

// A decision the user made late in a long decisions artifact still
// reaches the planner whole, while other long artifacts are cut.
func TestWorkflowPromptKeepsDecisions(t *testing.T) {
	long := strings.Repeat("- **DEC-002** (Pertanyaan): terbuka ééé\n", 200)
	docs := []core.Artifact{
		{ID: core.ArtifactDecisions, Title: "Keputusan", Body: "## Terbuka\n\n" + long + "## Diputuskan\n\n- **DEC-001**\n  - Keputusan: Akses DB: Squirrel + pgx\n"},
		{ID: "open-decisions", Title: "OD", Body: "Rekomendasi: sqlc\n" + long + "EKOR"},
	}
	got := WorkflowPrompt(&core.Project{
		Name: "RBAC", Story: "s",
		Decisions: []core.Decision{{ID: "DEC-001", Kind: core.DecisionQuestion, Text: "Akses DB?", Resolution: "Squirrel + pgx"}},
	}, docs)
	if !strings.Contains(got, "BINDING DECISIONS") || !strings.Contains(got, "Keputusan: Akses DB: Squirrel + pgx") || !strings.Contains(got, "1. decided decisions") {
		t.Error("decided decision or precedence missing from workflow prompt")
	}
	if strings.Contains(got, "EKOR") || !utf8.ValidString(got) {
		t.Error("non-binding artifact not cut on a rune boundary")
	}
}

// A pending decision sync puts the binding digest first, keeps contradicting
// artifacts whole, and forbids a sidecar document that restates the decision.
func TestArtifactsPromptBindsDecisions(t *testing.T) {
	p := &core.Project{
		Name: "Futures", Story: "pasar futures",
		Decisions: []core.Decision{{
			ID: "DEC-020", Kind: core.DecisionConflict, Text: "Trader transfer Spot-Futures?",
			Resolution: "Trader tidak boleh transfer; admin maker-checker.",
		}},
		DecisionSync:    []string{"DEC-020"},
		ArtifactRequest: &core.ChangeRequest{Text: "Sesuaikan", Sync: true},
	}
	long := strings.Repeat("Trader boleh transfer.\n", 300)
	got := ArtifactsPrompt(p, []core.Artifact{
		{ID: "threat-model", Title: "TM", Body: long + "EKOR-TRANSFER"},
		{ID: core.ArtifactDecisions, Title: "Keputusan", Body: "Keputusan: admin"},
	})
	for _, want := range []string{
		"BINDING DECISIONS", "DEC-020", "Trader tidak boleh transfer",
		"EKOR-TRANSFER", "Do not create a new artifact whose only job is to restate",
		"DECISIONS CHANGED",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
}

// An empty sync answer is valid: nothing contradicted the changed decisions.
func TestApplyArtifactsDecisionSyncEmpty(t *testing.T) {
	p := &core.Project{
		Decisions:       []core.Decision{{ID: "DEC-001", Kind: core.DecisionQuestion, Text: "Gateway?", Resolution: "Midtrans"}},
		DecisionSync:    []string{"DEC-001"},
		ArtifactRequest: &core.ChangeRequest{Text: "Sesuaikan", Sync: true},
	}
	if err := ApplyArtifacts(p, `{}`); err != nil {
		t.Fatal(err)
	}
	if p.DecisionSync != nil || p.ArtifactRequest != nil {
		t.Fatalf("after empty sync: %+v %+v", p.DecisionSync, p.ArtifactRequest)
	}
}
