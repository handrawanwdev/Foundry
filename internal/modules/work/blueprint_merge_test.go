package work

import (
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
)

func TestMergeBlueprintKeepsUntouchedModules(t *testing.T) {
	prev := &core.Blueprint{
		Architecture: core.ArchDecision{Pattern: "modular monolith", Reason: "satu proses"},
		Modules: []core.Module{
			{ID: "identity", Responsibility: "Akun", Interface: []string{"POST /login"}},
			{ID: "billing", Responsibility: "Tagihan"},
		},
		Database: core.DatabaseArch{Entities: []core.Entity{{Name: "User", Module: "identity", Fields: []string{"id"}}}},
		Auth:     core.AuthArch{Method: "session"},
	}
	next := &core.Blueprint{
		Architecture: core.ArchDecision{Pattern: "microservices", Reason: "skala"},
		Modules: []core.Module{
			{ID: "identity", Responsibility: "DIGANTI"},
			{ID: "notify", Responsibility: "Notifikasi"},
		},
		Database: core.DatabaseArch{Entities: []core.Entity{{Name: "Notice", Module: "notify"}}},
		Auth:     core.AuthArch{Method: "jwt"},
	}
	change := &core.ChangePlan{
		Scope: core.BlueprintScopeChange,
		Items: []core.ChangeItem{{Action: core.ChangeAdd, Target: "Notification", Module: "notify"}},
	}
	got, kept := MergeBlueprint(prev, next, change, core.IntentNewFeature)
	if got == nil {
		t.Fatal("nil")
	}
	ids := map[string]bool{}
	for _, m := range got.Modules {
		ids[m.ID] = true
		if m.ID == "identity" && m.Responsibility == "DIGANTI" {
			t.Fatal("identity overwritten")
		}
	}
	if !ids["identity"] || !ids["billing"] || !ids["notify"] {
		t.Fatalf("modules %+v", got.Modules)
	}
	if got.Architecture.Pattern != "modular monolith" {
		t.Fatalf("architecture %q kept=%v", got.Architecture.Pattern, kept)
	}
	if got.Auth.Method != "session" {
		t.Fatalf("auth %q", got.Auth.Method)
	}
	if len(kept) == 0 {
		t.Fatal("expected kept reasons")
	}
}

func TestMergeBlueprintFullScopeReplaces(t *testing.T) {
	prev := &core.Blueprint{Modules: []core.Module{{ID: "old", Responsibility: "x"}}}
	next := &core.Blueprint{Modules: []core.Module{{ID: "new", Responsibility: "y"}}}
	got, _ := MergeBlueprint(prev, next, &core.ChangePlan{Scope: core.BlueprintScopeFull}, core.IntentExisting)
	if len(got.Modules) != 1 || got.Modules[0].ID != "new" {
		t.Fatalf("%+v", got.Modules)
	}
}

func TestApplyDiscoveryPreservesDecisionsOnChange(t *testing.T) {
	p := existingCtxProject()
	p.Blueprint = &core.Blueprint{
		Architecture: core.ArchDecision{Pattern: "modular monolith", Reason: "ada"},
		Modules:      []core.Module{{ID: "billing", Responsibility: "Tagihan"}},
	}
	p.Decisions = []core.Decision{{
		ID: "DEC-001", Kind: core.DecisionAssumption, Area: core.AreaSecurity,
		Text: "Auth memakai session", Resolution: "session, bukan JWT",
	}}
	raw := strings.TrimSuffix(thinDiscovery, "}") + `,
  "change": {
    "scope": "change",
    "summary": "Notifikasi",
    "items": [{"action": "add", "target": "Notification", "module": "notify", "files": ["internal/notify/service.go"]}],
    "files": {"add": [{"path": "internal/notify/service.go"}]}
  },
  "decisions": [{"kind": "assumption", "area": "security", "text": "Pakai JWT", "resolution": "jwt"}]
}`
	if err := ApplyDiscovery(p, raw); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range p.Decisions {
		if d.ID == "DEC-001" && d.Resolution == "session, bukan JWT" {
			found = true
		}
	}
	if !found {
		t.Fatalf("old decision dropped %+v", p.Decisions)
	}
	if p.Change == nil || len(p.Change.Impact) == 0 {
		t.Fatalf("observed impact missing %+v", p.Change)
	}
}

func TestGreenfieldDiscoveryUnchanged(t *testing.T) {
	p := &core.Project{Story: "Aplikasi klinik", Intent: core.IntentNewProject,
		Structure: &core.Structure{Mode: core.StructureNew}}
	if err := ApplyDiscovery(p, thinDiscovery); err != nil {
		t.Fatal(err)
	}
	if p.Change != nil {
		t.Fatalf("greenfield has change %+v", p.Change)
	}
	if p.Structure.Mode != core.StructureNew {
		t.Fatal("greenfield structure flipped")
	}
}
