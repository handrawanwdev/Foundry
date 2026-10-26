package work

import (
	"slices"
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
)

func staleProject(t *testing.T) (*core.Project, []core.Artifact) {
	t.Helper()
	docs := []core.Artifact{{ID: "requirements", Title: "R", Body: "v1"}}
	p := &core.Project{}
	raw := `{"commands": [
		{"id": "C1", "name": "satu", "objective": "o", "instructions": "i", "artifacts": ["requirements"]},
		{"id": "C2", "name": "dua", "objective": "o", "instructions": "i", "depends_on": ["C1"], "artifacts": ["requirements"]}]}`
	if err := ApplyWorkflow(p, raw, docs); err != nil {
		t.Fatal(err)
	}
	docs[0] = core.Artifact{ID: "requirements", Title: "R", Body: "v2"}
	return p, docs
}

// A truncated answer must not decode to an empty workflow: the inner command
// object has no "commands" key, so it is not a workflow.
func TestExtractWorkflowRejectsInnerObject(t *testing.T) {
	var out workflowOut
	err := ExtractJSONFunc(`{"commands": [{"id": "C1", "name": "a"}, {"id": "C2"`,
		&out, func() bool { return out.Commands != nil || out.Remove != nil })
	if err == nil {
		t.Fatalf("truncated answer decoded as %+v", out)
	}
}

// OpenCode cuts long answers at the model's output limit. The commands that
// arrived whole are applied, the request stays for the next round, and the
// next answer finishes it.
func TestApplyWorkflowTruncatedMakesProgress(t *testing.T) {
	p, docs := staleProject(t)
	p.Workflow.Request = &core.ChangeRequest{
		Text:  "Perbarui command yang terdampak.",
		Focus: []string{"CMD-001", "CMD-002"},
	}
	truncated := "```json\n{\"commands\": [\n" +
		`{"id": "CMD-001", "name": "satu baru", "objective": "o", "instructions": "i", "artifacts": ["requirements"]},` + "\n" +
		`{"id": "CMD-002", "name": "dua`
	if err := ApplyWorkflow(p, truncated, docs); err != nil {
		t.Fatalf("truncated answer: %v", err)
	}
	if p.Workflow.Commands[0].Name != "satu baru" {
		t.Fatalf("salvaged command = %+v", p.Workflow.Commands[0])
	}
	if p.Workflow.Request == nil {
		t.Fatal("request cleared while a stale command was still untouched")
	}

	full := `{"commands": [{"id": "CMD-002", "name": "dua baru", "objective": "o",
		"instructions": "i", "depends_on": ["CMD-001"], "artifacts": ["requirements"]}]}`
	if err := ApplyWorkflow(p, full, docs); err != nil {
		t.Fatal(err)
	}
	if p.Workflow.Request != nil {
		t.Fatalf("request survived a complete answer: %+v", p.Workflow.Request)
	}
	if p.Workflow.Commands[1].Name != "dua baru" {
		t.Fatalf("command = %+v", p.Workflow.Commands[1])
	}
}

// An empty answer to a sync request means the text still matches the changed
// artifacts: their new versions are recorded instead of failing.
func TestApplyWorkflowSyncEmptyAcceptsStale(t *testing.T) {
	p, docs := staleProject(t)
	p.Workflow.Request = &core.ChangeRequest{
		Text:  "Perbarui command yang terdampak.",
		Focus: []string{"CMD-001"},
		Sync:  true,
	}
	before := p.Workflow.Version
	if err := ApplyWorkflow(p, `{"commands": []}`, docs); err != nil {
		t.Fatal(err)
	}
	if p.Workflow.Request != nil || p.Workflow.Version != before+1 {
		t.Fatalf("workflow = %+v", p.Workflow)
	}
	if h := docs[0].Hash(); !slices.ContainsFunc(p.Workflow.Commands[0].Artifacts, func(r core.ArtifactRef) bool {
		return r.ID == "requirements" && r.Hash == h
	}) {
		t.Fatalf("artifact hash not refreshed: %+v", p.Workflow.Commands[0].Artifacts)
	}
	// A command outside the focus keeps its old hash.
	if slices.ContainsFunc(p.Workflow.Commands[1].Artifacts, func(r core.ArtifactRef) bool {
		return r.Hash == docs[0].Hash()
	}) {
		t.Fatal("unfocused command was refreshed")
	}
}

// A real request that the AI leaves unanswered is still refused.
func TestApplyWorkflowEmptyChangeStillFails(t *testing.T) {
	p, docs := staleProject(t)
	p.Workflow.Request = &core.ChangeRequest{Text: "pecah CMD-001", Focus: []string{"CMD-001"}}
	if err := ApplyWorkflow(p, `{"commands": []}`, docs); err == nil || !strings.Contains(err.Error(), "tidak menemukan perubahan") {
		t.Fatalf("err = %v", err)
	}
}
