package core

import "testing"

func TestTraceabilityChain(t *testing.T) {
	p := &Project{
		Requirements: []Requirement{
			{ID: "REQ-001", Text: "Kirim notifikasi", Priority: "MUST",
				Acceptance: []Criterion{{ID: "REQ-001-AC1", Kind: "behavior", Text: "Email terkirim"}}},
		},
		Features: []Feature{{ID: "FEAT-001", Name: "Notifikasi", RequirementIDs: []string{"REQ-001"}}},
		FeatureMap: &FeatureMap{Modules: []FeatureModule{{
			ID: "mod-notify", Name: "Notify",
			Features: []FeatureNode{{ID: "n1", Name: "Notifikasi", FeatureID: "FEAT-001"}},
		}}},
		Blueprint: &Blueprint{Modules: []Module{{ID: "notify", Responsibility: "Notifikasi"}}},
		Change: &ChangePlan{
			Scope: BlueprintScopeChange,
			Items: []ChangeItem{{Action: ChangeAdd, Target: "Notifikasi", Module: "notify",
				Files: []string{"internal/notify/service.go"}}},
			Files: ChangeFiles{Add: []ChangeFile{{Path: "internal/notify/service.go"}}},
		},
		Workflow: &Workflow{Commands: []Command{{
			ID: "CMD-001", Covers: []string{"REQ-001"},
			Outputs:    []string{"test notifikasi"},
			Acceptance: []string{"REQ-001-AC1"},
		}}},
		Structure: &Structure{Mode: StructureExisting, Context: &ProjectContext{
			Modules: []ContextModule{{Name: "notify", Path: "internal/notify"}},
		}},
	}
	got := Traceability(p)
	if len(got) != 1 {
		t.Fatalf("got %+v", got)
	}
	tr := got[0]
	if tr.RequirementID != "REQ-001" || tr.FeatureID != "FEAT-001" || tr.Domain != "Notify" ||
		tr.Module != "notify" || len(tr.Files) == 0 || len(tr.Acceptance) == 0 ||
		len(tr.Commands) != 1 || len(tr.Tests) == 0 {
		t.Fatalf("incomplete chain %+v", tr)
	}
}

func TestTraceConsistencyBlocksUncoveredMUST(t *testing.T) {
	p := consistencyProject()
	p.Workflow = &Workflow{Commands: []Command{{ID: "CMD-001", Covers: []string{"REQ-999"}}}}
	issues := ConsistencyIssues(p)
	if Errors(issues) == nil || !containsIssue(issues, "REQ-001") {
		t.Fatalf("missing uncovered MUST: %+v", issues)
	}
}

func containsIssue(issues []Issue, where string) bool {
	for _, i := range issues {
		if i.Where == where {
			return true
		}
	}
	return false
}
