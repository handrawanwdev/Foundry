package work

import (
	"strings"
	"testing"

	"github.com/handrawanw/foundry/internal/core"
)

func TestApplyArtifacts(t *testing.T) {
	p := &core.Project{}
	SetArtifacts(p, []core.Artifact{{Title: "Model Domain", Body: "x"}, {ID: "model-domain", Title: "Lagi", Body: "y"},
		{ID: "architecture", Title: "Bentrok", Body: "z"}, {ID: "kosong", Title: "Kosong"}})
	if len(p.Artifacts) != 2 || p.Artifacts[0].ID != "model-domain" || p.Artifacts[1].ID != "model-domain-2" || p.Artifacts[0].Category != core.ArtifactOther {
		t.Fatalf("set = %+v", p.Artifacts)
	}
	for raw, want := range map[string]string{
		`{"artifacts": [{"id": "layout-spec", "title": "x", "body": "y"}]}`: "bawaan",
		`{"remove": ["requirements"]}`:                                      "bawaan",
		`{"remove": ["ghost"]}`:                                             "tidak ada",
		`{"artifacts": []}`:                                                 "tidak menemukan perubahan",
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
