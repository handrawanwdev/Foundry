package work

import (
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
)

func existingCtxProject() *core.Project {
	return &core.Project{
		Story:  "Tambahkan sistem notifikasi ke aplikasi tagihan",
		Intent: core.IntentNewFeature,
		Structure: &core.Structure{
			Mode: core.StructureExisting,
			Components: []core.Component{
				{Kind: "backend", Path: ".", Tech: "Go", Language: "Go", Evidence: "go.mod"},
			},
			Context: &core.ProjectContext{
				Languages: []core.ContextFinding{{Text: "Go", Path: ".", Evidence: "go.mod", Confidence: core.ConfidenceObserved}},
				Modules:   []core.ContextModule{{Name: "billing", Path: "internal/billing", Evidence: "source files: go×2"}},
				Data:      []core.ContextEntity{{Name: "Invoice", Fields: []string{"id", "amount"}, Path: "schema.prisma", Evidence: "schema.prisma model"}},
				APIs:      []core.ContextAPI{{Kind: "route", Path: "internal/http/router.go", Detail: "POST /invoices", Evidence: "router.go"}},
				Features:  []core.ContextFinding{{Text: "Tagihan", Path: "README.md", Confidence: core.ConfidenceDocumented, Evidence: "README.md ## Features"}},
				Corrections: "Modul billing sudah handle invoice.",
			},
		},
	}
}

func TestDiscoveryPromptExistingContextAndChange(t *testing.T) {
	p := existingCtxProject()
	got := DiscoveryPrompt(p)
	for _, want := range []string{
		"WORK INTENT: new_feature",
		"EXISTING PROJECT COMPONENTS",
		"EXISTING PROJECT ADAPTATION",
		"FEATURE CHANGE BLUEPRINT",
		"EXISTING PROJECT CONTEXT",
		"internal/billing",
		"Invoice",
		"POST /invoices",
		"Tagihan",
		"USER CORRECTIONS",
		"Modul billing sudah handle invoice",
		`"scope": "full|change"`,
		`"action": "add|modify|reuse|refactor|remove"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("discovery prompt missing %q", want)
		}
	}
	if strings.Contains(got, "Greenfield") {
		t.Error("existing treated as greenfield")
	}
}

func TestFeatureMapPromptSkipsDuplicateFeatures(t *testing.T) {
	p := existingCtxProject()
	got := FeatureMapPrompt(p)
	for _, want := range []string{
		"Do not add a feature the repository already has",
		"Tagihan",
		"internal/billing",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("feature map prompt missing %q", want)
		}
	}
}

func TestWorkflowPromptCarriesChangePlan(t *testing.T) {
	p := existingCtxProject()
	p.Change = &core.ChangePlan{
		Scope:   core.BlueprintScopeChange,
		Summary: "Notifikasi invoice",
		Existing: []string{"internal/billing"},
		Items: []core.ChangeItem{
			{Action: core.ChangeAdd, Target: "Notification", Module: "notify", Files: []string{"internal/notify/service.go"}},
			{Action: core.ChangeReuse, Target: "Invoice", Module: "billing"},
		},
		Files: core.ChangeFiles{Add: []core.ChangeFile{{Path: "internal/notify/service.go", Responsibility: "kirim"}}},
	}
	got := workflowFreshPrompt(p, nil)
	for _, want := range []string{
		"EXISTING CODEBASE: yes",
		"EXISTING PROJECT COMMANDS",
		"inspect the named files first",
		"CHANGE PLAN",
		"internal/notify/service.go",
		"Notifikasi invoice",
		"reuse Invoice",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("workflow prompt missing %q", want)
		}
	}
}

func TestApplyDiscoveryLandsChangePlan(t *testing.T) {
	p := existingCtxProject()
	raw := strings.TrimSuffix(thinDiscovery, "}") + `,
  "change": {
    "scope": "change",
    "summary": "Notifikasi",
    "existing": ["internal/billing"],
    "items": [{"action": "add", "target": "Notification", "module": "notify", "files": ["internal/notify/service.go"]}],
    "impact": [{"area": "api", "message": "endpoint baru", "risk": "medium"}],
    "files": {"add": [{"path": "internal/notify/service.go", "responsibility": "kirim"}]}
  }
}`
	if err := ApplyDiscovery(p, raw); err != nil {
		t.Fatal(err)
	}
	c := p.Change
	if c == nil || c.Scope != core.BlueprintScopeChange || c.Summary != "Notifikasi" {
		t.Fatalf("change %+v", c)
	}
	if len(c.Items) != 1 || c.Items[0].Action != core.ChangeAdd || len(c.Files.Add) != 1 {
		t.Fatalf("items %+v files %+v", c.Items, c.Files)
	}
}
