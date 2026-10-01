package work

import (
	"slices"
	"strings"
	"testing"

	"github.com/handrawanw/foundry/internal/core"
)

func TestPlanPromptAsksParallel(t *testing.T) {
	p := &core.Project{Platform: &core.PlatformPlan{}}
	got := PlanPrompt(p)
	for _, s := range []string{"one page per task", "Never a whole tree", "true prerequisites"} {
		if !strings.Contains(got, s) {
			t.Fatalf("plan prompt missing %q", s)
		}
	}
}

func TestApplyPlanNormalizesFilesAndPages(t *testing.T) {
	p := &core.Project{
		Requirements: []core.Requirement{{ID: "REQ-001", Text: "x"}},
		Features: []core.Feature{{ID: "FEAT-001", Name: "A", Description: "d", RequirementIDs: []string{"REQ-001"},
			SubFeatures: []core.SubFeature{{Name: "s"}}}},
		Design: &core.Design{Pages: []core.Page{
			{ID: "PAGE-001", Name: "Home", FeatureID: "FEAT-001", Purpose: "home"},
			{ID: "PAGE-002", Name: "About", FeatureID: "FEAT-001", Purpose: "about"},
		}},
	}
	raw := `{"tasks":[
	  {"id":"T1","title":"UI both","area":"ui","feature_id":"FEAT-001","page_ids":["PAGE-001","PAGE-002"],
	   "acceptance_criteria":["ok"],"files":["src","packages/ui","apps/web/src/pages/Home.tsx","./Foo.go"]},
	  {"id":"T2","title":"API","area":"api","feature_id":"FEAT-001","acceptance_criteria":["ok"],"files":["services/api"]}
	]}`
	if err := ApplyPlan(p, raw); err != nil {
		t.Fatal(err)
	}
	ui := p.Task("TASK-001")
	if ui == nil || !slices.Equal(ui.PageIDs, []string{"PAGE-001"}) {
		t.Fatalf("ui pages = %+v", ui)
	}
	if !slices.Equal(ui.Files, []string{"apps/web/src/pages/Home.tsx", "Foo.go"}) {
		t.Fatalf("ui files = %v", ui.Files)
	}
	api := p.Task("TASK-002")
	if api == nil || len(api.Files) != 0 {
		t.Fatalf("wide api files kept: %+v", api)
	}
	var extraUI bool
	for _, tk := range p.Tasks {
		if tk.Area == "ui" && slices.Equal(tk.PageIDs, []string{"PAGE-002"}) {
			extraUI = true
		}
	}
	if !extraUI {
		t.Fatal("PAGE-002 not filled")
	}
}
