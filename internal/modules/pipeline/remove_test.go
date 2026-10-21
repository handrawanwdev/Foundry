package pipeline

import (
	"slices"
	"testing"

	"github.com/handrawanw/foundry/internal/core"
)

func deleteFixture() *core.Project {
	return &core.Project{
		Requirements: []core.Requirement{{ID: "REQ-001", Priority: "must", Text: "login"}},
		Artifacts: []core.Artifact{
			{ID: "threat-model", Title: "Threat Model", Category: core.ArtifactQuality, Area: core.AreaSecurity, Body: "x", Version: 1},
			{ID: "ops-notes", Title: "Ops Notes", Category: core.ArtifactOther, Body: "y", Refs: []string{"threat-model"}, Version: 1},
		},
		Workflow: &core.Workflow{Status: core.CommandDraft, Commands: []core.Command{
			{ID: "CMD-001", Name: "Hardening", Objective: "o", Instructions: "i", Artifacts: []core.ArtifactRef{{ID: "threat-model"}}, Status: core.CommandDraft},
			{ID: "CMD-002", Name: "Login", Objective: "o", Instructions: "i", Artifacts: []core.ArtifactRef{{ID: "threat-model"}, {ID: "requirements"}},
				DependsOn: []string{"CMD-001"}, Covers: []string{"REQ-001"}, Status: core.CommandDraft},
			{ID: "CMD-003", Name: "Setup", Objective: "o", Instructions: "i", Artifacts: []core.ArtifactRef{{ID: "requirements"}}, DependsOn: []string{"CMD-001"}, Status: core.CommandDraft},
		}},
	}
}

func TestDeleteDocReleasesWhatDependsOnIt(t *testing.T) {
	p := deleteFixture()
	im, err := deleteDoc(p, "threat-model")
	if err != nil {
		t.Fatal(err)
	}
	if slices.ContainsFunc(p.Artifacts, func(a core.Artifact) bool { return a.ID == "threat-model" }) {
		t.Fatal("document still there")
	}
	if got := p.Artifacts[0]; got.ID != "ops-notes" || got.Refs != nil {
		t.Fatalf("referrer = %+v", got)
	}
	if len(p.Workflow.Commands) != 2 || p.Workflow.Command("CMD-001") != nil {
		t.Fatalf("commands = %+v", p.Workflow.Commands)
	}
	c2 := p.Workflow.Command("CMD-002")
	if c2.Status != core.CommandNeedsReview || len(c2.Artifacts) != 1 || len(c2.DependsOn) != 0 {
		t.Fatalf("CMD-002 = %+v", c2)
	}
	if c3 := p.Workflow.Command("CMD-003"); c3.Status != core.CommandNeedsReview || len(c3.DependsOn) != 0 {
		t.Fatalf("CMD-003 = %+v", c3)
	}
	if !slices.Equal(im.review, []string{"CMD-002", "CMD-003"}) || !slices.Equal(im.referrers, []string{"ops-notes"}) || !im.AI {
		t.Fatalf("impact = %+v", im)
	}
	if !slices.Equal(im.Areas, []string{"Keamanan & Threat Model"}) {
		t.Fatalf("areas = %v", im.Areas)
	}
	if r := CheckWorkflow(p); len(r.Issues) > 0 {
		t.Fatalf("workflow issues after delete: %+v", r.Issues)
	}
}

func TestDeleteDocPreviewMatchesAndRefusesComputedDocs(t *testing.T) {
	p := deleteFixture()
	if _, err := deleteDoc(p, "requirements"); err == nil {
		t.Fatal("requirements must not be deletable")
	}
	if _, err := deleteDoc(p, "nope"); err == nil {
		t.Fatal("unknown document must fail")
	}
	p = deleteFixture()
	im, err := deleteDoc(p, "threat-model")
	if err != nil {
		t.Fatal(err)
	}
	if got := cleanupRequests(p, im); got != "blueprint" || p.ArtifactRequest == nil || !p.ArtifactRequest.Cascade ||
		p.Workflow.Request == nil || !p.Workflow.Request.Cascade {
		t.Fatalf("cleanup = %q %+v %+v", got, p.ArtifactRequest, p.Workflow.Request)
	}
}
