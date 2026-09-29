package work

import (
	"strings"
	"testing"

	"github.com/handrawanw/foundry/internal/core"
)

func revisionProject() *core.Project {
	return &core.Project{
		Platform: &core.PlatformPlan{},
		Features: []core.Feature{{ID: "FEAT-001", Name: "Janji"}},
		Flows:    []core.Flow{{ID: "FLOW-001", PageIDs: []string{"PAGE-001", "PAGE-002"}}},
		Design: &core.Design{Status: core.DesignChanges, Feedback: []string{"lama satu", "baru"}, Pages: []core.Page{
			{ID: "PAGE-001", Name: "Daftar", FeatureID: "FEAT-001", Components: []core.PageComponent{{Type: "button", Label: "Buka", Target: "PAGE-002"}}},
			{ID: "PAGE-002", Name: "Detail", FeatureID: "FEAT-001"},
		}},
	}
}

func TestDesignPromptRevision(t *testing.T) {
	got := DesignPrompt(revisionProject())
	for _, want := range []string{"CURRENT DESIGN:", "EARLIER REQUESTS (already applied in the current design, do not redo):\n- lama satu", "CHANGE REQUEST (apply now):\nbaru", `"removed"`} {
		if !strings.Contains(got, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
	if strings.Contains(got, "apply all") {
		t.Error("earlier requests must not be applied again")
	}
}

func TestApplyRevisionRemove(t *testing.T) {
	p := revisionProject()
	if err := ApplyDesign(p, "```json\n{\"removed\": [\"PAGE-002\"]}\n```"); err != nil {
		t.Fatal(err)
	}
	if len(p.Design.Pages) != 1 || p.Design.Pages[0].Components[0].Target != "" {
		t.Fatalf("pages = %+v", p.Design.Pages)
	}
	if got := p.Flows[0].PageIDs; len(got) != 1 || got[0] != "PAGE-001" {
		t.Fatalf("flow = %v", got)
	}
	if r := p.Design.Revision; r == nil || len(r.Changes) != 1 || r.Changes[0].Kind != "removed" {
		t.Fatalf("revision = %+v", r)
	}
	if err := ApplyDesign(revisionProject(), "```json\n{}\n```"); err == nil {
		t.Fatal("empty change set must fail")
	}
}
