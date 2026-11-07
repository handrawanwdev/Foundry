package spec

import (
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
)

func planProject() *core.Project {
	return &core.Project{Name: "x", Stack: &core.TechStack{Plan: &core.TechPlan{
		Approach: "monolith", Items: []core.TechItem{{Area: "backend", Technology: "Go"}}}}}
}

func techPlanDoc(t *testing.T, p *core.Project) core.Artifact {
	t.Helper()
	for _, d := range Artifacts(p) {
		if d.ID == core.ArtifactTechPlan {
			return d
		}
	}
	t.Fatal("technology plan artifact not rendered")
	return core.Artifact{}
}

// Approving the stack changes only the status line of the plan artifact;
// the commands that recorded it must not need review for that.
func TestPlanStatusIsNotContent(t *testing.T) {
	p := planProject()
	p.Stack.SetStatus(core.TechRecommended)
	before := techPlanDoc(t, p)
	if before.Status != "Status: **recommended**" {
		t.Fatalf("status = %q", before.Status)
	}
	p.Stack.SetStatus(core.TechApproved)
	after := techPlanDoc(t, p)
	if before.Body == after.Body {
		t.Fatal("status line must still be shown in the body")
	}
	if before.Hash() != after.Hash() {
		t.Error("approval changed the artifact hash")
	}
	p.Stack.Plan.Approach = "microservices"
	if techPlanDoc(t, p).Hash() == after.Hash() {
		t.Error("content change must change the hash")
	}
}

// Refs recorded with the hash that counted the status move to the current
// hash once; refs that were already stale stay stale.
func TestMigrateStatusHashes(t *testing.T) {
	p := planProject()
	d := techPlanDoc(t, p)
	p.Workflow = &core.Workflow{Commands: []core.Command{
		{ID: "CMD-001", Artifacts: []core.ArtifactRef{{ID: d.ID, Hash: d.LegacyHash()}}},
		{ID: "CMD-002", Artifacts: []core.ArtifactRef{{ID: d.ID, Hash: "stale"}}},
	}}
	p.Artifacts = []core.Artifact{{ID: "notes", Refs: []string{d.ID}, Seen: map[string]string{d.ID: d.LegacyHash()}}}
	migrateStatusHashes(p)
	if got := p.Workflow.Commands[0].Artifacts[0].Hash; got != d.Hash() {
		t.Errorf("legacy ref not migrated: %s", got)
	}
	if got := p.Workflow.Commands[1].Artifacts[0].Hash; got != "stale" {
		t.Errorf("stale ref changed: %s", got)
	}
	if got := p.Artifacts[0].Seen[d.ID]; got != d.Hash() {
		t.Errorf("seen hash not migrated: %s", got)
	}
	if p.HashVersion != core.HashVersion {
		t.Error("hash version not recorded")
	}
	p.Workflow.Commands[1].Artifacts[0].Hash = d.LegacyHash()
	migrateStatusHashes(p) // runs once
	if p.Workflow.Commands[1].Artifacts[0].Hash != d.LegacyHash() {
		t.Error("migration ran twice")
	}
}
