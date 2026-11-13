package work

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
)

// A large workflow keeps a focused revision small: the focus and its direct
// dependents always, deeper dependents only while they fit the budget.
func TestRevisionScopeBudget(t *testing.T) {
	big := strings.Repeat("x", revisionScopeBudget/3)
	cmd := func(id string, deps ...string) core.Command {
		return core.Command{ID: id, Instructions: big, DependsOn: deps}
	}
	w := &core.Workflow{Commands: []core.Command{
		cmd("A"), cmd("B", "A"), cmd("C", "A"), cmd("D", "B"), cmd("E", "D"), cmd("X"),
	}, Request: &core.ChangeRequest{Text: "x", Focus: []string{"A"}}}
	got := slices.Sorted(maps.Keys(revisionScope(w)))
	if !slices.Equal(got, []string{"A", "B", "C"}) {
		t.Fatalf("scope = %v, want the focus and its direct dependents", got)
	}
	// A round that changed B brings B's dependents into the core.
	w.Request.Handled = []string{"changed B"}
	got = slices.Sorted(maps.Keys(revisionScope(w)))
	if !slices.Equal(got, []string{"A", "B", "C", "D"}) {
		t.Fatalf("scope after round 1 = %v", got)
	}
}

func revisionFixture() (*core.Project, []core.Artifact) {
	art := []core.ArtifactRef{{ID: "requirements", Hash: "h"}}
	p := &core.Project{
		Requirements: []core.Requirement{{ID: "REQ-001", Priority: "MUST", Text: "login"}, {ID: "REQ-002", Priority: "MUST", Text: "laporan"}},
		Workflow: &core.Workflow{Version: 1, Commands: []core.Command{
			{ID: "CMD-001", Name: "a", Objective: "o", Instructions: "teks-a", Artifacts: art, Covers: []string{"REQ-001"}},
			{ID: "CMD-002", Name: "b", Objective: "o", Instructions: "teks-b", Artifacts: art, DependsOn: []string{"CMD-001"}},
			{ID: "CMD-003", Name: "c", Objective: "o", Instructions: "teks-c", Artifacts: art, Outputs: []string{"keluaran-c"}},
		}, Request: &core.ChangeRequest{Text: "ganti", Focus: []string{"CMD-001"}}},
	}
	return p, []core.Artifact{{ID: "requirements", Body: "isi-req"}}
}

// A revision round never takes a MUST requirement's last command away; a
// MUST the workflow already missed does not block it.
func TestRevisionKeepsMustCoverage(t *testing.T) {
	p, docs := revisionFixture()
	if err := ApplyWorkflow(p, `{"commands": [], "remove": ["CMD-001"]}`, docs); err == nil || !strings.Contains(err.Error(), uncoveredMust+"REQ-001") {
		t.Fatalf("err = %v", err)
	}
	if p.Workflow.Command("CMD-001") == nil {
		t.Fatal("refused round changed the workflow")
	}
	drop := `{"commands": [{"id": "CMD-001", "name": "a", "objective": "o", "instructions": "i", "artifacts": ["requirements"]}]}`
	if err := ApplyWorkflow(p, drop, docs); err == nil || !strings.Contains(err.Error(), "REQ-001") {
		t.Fatalf("covers dropped: err = %v", err)
	}
	keep := `{"commands": [{"id": "CMD-001", "name": "a2", "objective": "o", "instructions": "i", "artifacts": ["requirements"], "covers": ["REQ-001"]}], "done": true}`
	if err := ApplyWorkflow(p, keep, docs); err != nil {
		t.Fatalf("REQ-002 was already uncovered, the round must pass: %v", err)
	}
}

// Rules and blueprint are the static part; a retry changes only the
// dynamic part, which carries the commands without artifact hashes and the
// outline with what the scope builds on.
func TestRevisionPromptSplit(t *testing.T) {
	p, docs := revisionFixture()
	p.Workflow.Commands[0].DependsOn = []string{"CMD-003"}
	static1, body1 := SplitPrompt(WithPolicy(WorkflowPrompt(p, docs)))
	for _, want := range []string{"Command rules", "Revision rules", "a round never leaves a MUST requirement without a command", "isi-req"} {
		if !strings.Contains(static1, want) {
			t.Errorf("static part misses %q", want)
		}
	}
	for _, want := range []string{"CURRENT COMMANDS", "teks-a", "teks-b", "- CMD-003 c: o", "output: keluaran-c", "USER REQUEST:\nganti"} {
		if !strings.Contains(body1, want) {
			t.Errorf("dynamic part misses %q", want)
		}
	}
	if strings.Contains(body1, `"hash"`) || strings.Contains(body1, "teks-c") || strings.Contains(body1, "workflow_id") {
		t.Errorf("dynamic part carries hashes or out-of-scope text:\n%s", body1)
	}
	p.Workflow.Request.Rejected = "workflow tidak valid: " + uncoveredMust + "REQ-001"
	static2, body2 := SplitPrompt(WithPolicy(WorkflowPrompt(p, docs)))
	if static1 != static2 {
		t.Fatal("static part changed on retry")
	}
	if !strings.Contains(body2, "YOUR LAST ANSWER WAS REFUSED") || !strings.Contains(body2, "MUST REQUIREMENTS YOUR LAST ANSWER LEFT UNCOVERED") {
		t.Fatalf("retry body:\n%s", body2)
	}
}

// A sync revision reads whole only the artifacts that changed; a long
// unchanged one is cut to its sections about the request.
func TestSyncRevisionReadsChangedArtifactsWhole(t *testing.T) {
	filler := strings.Repeat("kalimat pengisi umum. ", 120)
	long := "## Antrian\nRetry webhook pembayaran. " + filler + "\n## Laporan\nGrafik okupansi. " + filler + "\n"
	p, _ := revisionFixture()
	for i := range p.Workflow.Commands {
		p.Workflow.Commands[i].Artifacts = []core.ArtifactRef{{ID: "changed", Hash: "old"}, {ID: "same"}}
	}
	docs := []core.Artifact{{ID: "changed", Body: long}, {ID: "same", Body: long}, {ID: "requirements", Body: "isi-req"}}
	for i := range p.Workflow.Commands {
		p.Workflow.Commands[i].Artifacts[1].Hash = docs[1].Hash()
	}
	p.Workflow.Request = &core.ChangeRequest{Text: "Ikuti perubahan retry webhook pembayaran.", Sync: true}
	prompt := WorkflowPrompt(p, docs)
	i, j := strings.Index(prompt, "=== changed"), strings.Index(prompt, "=== same")
	if i < 0 || j < 0 {
		t.Fatal("artifact missing")
	}
	if !strings.Contains(prompt[i:j], "Grafik okupansi") {
		t.Error("changed artifact not whole")
	}
	same := prompt[j:strings.Index(prompt, "=== requirements")]
	if !strings.Contains(same, "Retry webhook pembayaran") || strings.Contains(same, "Grafik okupansi") {
		t.Errorf("unchanged artifact not cut to the request (%d bytes)", len(same))
	}
}

// The sketch sends only its refusal fresh: the rest is the same on retry.
func TestSketchRetryReusesStaticPart(t *testing.T) {
	p := detailProject()
	p.Workflow = nil
	static1, body1 := SplitPrompt(WithPolicy(WorkflowPrompt(p, nil)))
	p.Workflow = &core.Workflow{Request: &core.ChangeRequest{Rejected: "workflow tidak valid: " + uncoveredMust + "REQ-002"}}
	static2, body2 := SplitPrompt(WithPolicy(WorkflowPrompt(p, nil)))
	if static1 != static2 {
		t.Fatal("static part changed on retry")
	}
	if !strings.Contains(static1, "SKETCH PHASE") || !strings.Contains(static1, "REQ-002 [MUST] laporan") {
		t.Error("rules or registry missing from the static part")
	}
	if strings.Contains(body1, "REFUSED") || !strings.Contains(body2, "REQ-002-AC1 (behavior) laporan tampil") {
		t.Errorf("bodies:\n%s\n---\n%s", body1, body2)
	}
}
