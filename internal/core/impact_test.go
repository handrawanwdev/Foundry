package core

import "testing"

func impactProject() *Project {
	return &Project{
		Intent: IntentNewFeature,
		Structure: &Structure{
			Mode: StructureExisting,
			Context: &ProjectContext{
				Modules: []ContextModule{
					{Name: "billing", Path: "internal/billing"},
					{Name: "notify", Path: "internal/notify"},
					{Name: "http", Path: "internal/http"},
				},
				Deps: []ContextDep{
					{From: "internal/http", To: "internal/billing", Kind: DepKindModule, Path: "internal/http/router.go", Evidence: "import", Confidence: ConfidenceObserved},
					{From: "internal/notify", To: "internal/billing", Kind: DepKindModule, Path: "internal/notify/svc.go", Evidence: "import", Confidence: ConfidenceObserved},
				},
				Features: []ContextFinding{{Text: "Tagihan", Path: "README.md", Confidence: ConfidenceDocumented}},
				APIs:     []ContextAPI{{Kind: "route", Path: "internal/http/router.go", Detail: "POST /invoices"}},
			},
		},
		Blueprint: &Blueprint{
			Modules:      []Module{{ID: "billing", Responsibility: "Tagihan"}, {ID: "identity", Responsibility: "Akun"}},
			Architecture: ArchDecision{Pattern: "modular monolith", Reason: "satu proses"},
		},
		Change: &ChangePlan{
			Scope: BlueprintScopeChange,
			Items: []ChangeItem{
				{Action: ChangeModify, Target: "Invoice", Module: "billing", Files: []string{"internal/billing/service.go", "internal/http/router.go"}},
				{Action: ChangeAdd, Target: "Tagihan", Module: "billing"},
			},
			Files: ChangeFiles{
				Modify: []ChangeFile{{Path: "internal/billing/service.go"}, {Path: "internal/http/router.go"}},
			},
		},
	}
}

func TestObservedImpactDirectIndirectAndAPI(t *testing.T) {
	p := impactProject()
	got := ObservedImpact(p)
	has := func(kind string) bool {
		for _, im := range got {
			if im.Kind == kind && im.Source == ConfidenceObserved {
				return true
			}
		}
		return false
	}
	if !has(ImpactDirect) || !has(ImpactIndirect) || !has(ImpactAPI) || !has(ImpactRegression) {
		t.Fatalf("impact %+v", got)
	}
	for _, im := range got {
		if im.Source != ConfidenceObserved {
			t.Fatalf("estimated in observed set: %+v", im)
		}
	}
}

func TestChangeConflictsDuplicateAndDependents(t *testing.T) {
	p := impactProject()
	p.Change.Items = append(p.Change.Items, ChangeItem{Action: ChangeRemove, Target: "billing", Module: "billing"})
	cfs := ChangeConflicts(p)
	dup, dep := false, false
	for _, c := range cfs {
		if c.Kind == "duplicate" {
			dup = true
		}
		if c.Kind == "dependents" {
			dep = true
		}
	}
	if !dup || !dep {
		t.Fatalf("conflicts %+v", cfs)
	}
}

func TestEnrichChangeMergesObserved(t *testing.T) {
	p := impactProject()
	p.Change.Impact = []ChangeImpact{{Area: "ui", Message: "tidak ada layar baru", Risk: "low", Source: ConfidenceEstimated}}
	EnrichChange(p)
	if p.Change == nil || len(p.Change.Impact) < 2 {
		t.Fatalf("not merged %+v", p.Change)
	}
	est, obs := 0, 0
	for _, im := range p.Change.Impact {
		switch im.Source {
		case ConfidenceEstimated:
			est++
		case ConfidenceObserved:
			obs++
		}
	}
	if est == 0 || obs == 0 {
		t.Fatalf("sources est=%d obs=%d %+v", est, obs, p.Change.Impact)
	}
}

func TestSchemaChangeAddsMigrationAndRollback(t *testing.T) {
	p := impactProject()
	p.Change.Files.Modify = []ChangeFile{{Path: "schema.prisma"}}
	p.Change.Items = []ChangeItem{{Action: ChangeModify, Target: "User", Module: "billing", Files: []string{"schema.prisma"}}}
	got := ObservedImpact(p)
	mig, rb := false, false
	for _, im := range got {
		if im.Kind == ImpactMigration {
			mig = true
		}
		if im.Kind == ImpactRollback {
			rb = true
		}
	}
	if !mig || !rb {
		t.Fatalf("schema impact %+v", got)
	}
}
