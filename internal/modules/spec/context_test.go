package spec

import (
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
)

func TestProjectContextAndChangeArtifacts(t *testing.T) {
	p := &core.Project{
		Intent: core.IntentNewFeature,
		Structure: &core.Structure{
			Mode: core.StructureExisting,
			Context: &core.ProjectContext{
				Modules: []core.ContextModule{{Name: "billing", Path: "internal/billing", Evidence: "go×2"}},
				Data:    []core.ContextEntity{{Name: "Invoice", Path: "schema.prisma", Evidence: "model"}},
			},
		},
		Change: &core.ChangePlan{
			Scope:   core.BlueprintScopeChange,
			Summary: "Notifikasi invoice",
			Items:   []core.ChangeItem{{Action: core.ChangeAdd, Target: "Notification"}},
			Files:   core.ChangeFiles{Add: []core.ChangeFile{{Path: "internal/notify/service.go"}}},
		},
	}
	docs := Artifacts(p)
	var ctx, change *core.Artifact
	for i := range docs {
		switch docs[i].ID {
		case core.ArtifactProjectContext:
			ctx = &docs[i]
		case core.ArtifactChangePlan:
			change = &docs[i]
		}
	}
	if ctx == nil || !strings.Contains(ctx.Body, "internal/billing") || !strings.Contains(ctx.Body, "Invoice") {
		t.Fatalf("context artifact %+v", ctx)
	}
	if change == nil || !strings.Contains(change.Body, "Notifikasi invoice") || !strings.Contains(change.Body, "internal/notify/service.go") {
		t.Fatalf("change artifact %+v", change)
	}
	s := Export(p)
	if s.Intent != core.IntentNewFeature || s.Change == nil || s.Change.Summary != "Notifikasi invoice" {
		t.Fatalf("export %+v", s)
	}
}

func TestNoContextArtifactsOnNewProject(t *testing.T) {
	p := &core.Project{Intent: core.IntentNewProject, Structure: &core.Structure{Mode: core.StructureNew}}
	for _, a := range Artifacts(p) {
		if a.ID == core.ArtifactProjectContext || a.ID == core.ArtifactChangePlan {
			t.Fatalf("unexpected %s", a.ID)
		}
	}
}
