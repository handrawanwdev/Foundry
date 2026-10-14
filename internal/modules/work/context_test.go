package work

import (
	"fmt"
	"strings"
	"testing"

	"github.com/handrawanw/foundry/internal/core"
)

func scopedProject() *core.Project {
	b := &core.Blueprint{}
	b.Architecture.Pattern = "modular"
	b.Auth.Method = "jwt"
	for _, m := range []string{"billing", "inventory", "shipping", "reports", "users"} {
		mod := core.Module{ID: m, Responsibility: "owns " + m}
		if m == "billing" {
			mod.DependsOn = []string{"users"}
		}
		b.Modules = append(b.Modules, mod)
		b.Database.Entities = append(b.Database.Entities, core.Entity{Name: m + "_row", Module: m, Fields: []string{"id"}})
	}
	b.Workers = []core.Worker{{Name: "invoice-mailer", Trigger: "queue", Job: "send"}, {Name: "stock-sync", Trigger: "cron", Job: "sync"}}
	return &core.Project{Name: "Demo", Blueprint: b}
}

func TestTaskBriefKeepsOnlyRelevantModules(t *testing.T) {
	p := scopedProject()
	tk := &core.Task{ID: "TASK-002", Title: "Billing API sends via invoice-mailer", Area: "api", Files: []string{"modules/billing/api.go"}}
	got := TaskArchitectureBrief(p, tk)
	for _, want := range []string{"- billing:", "- users:", "billing_row", "users_row", "invoice-mailer", "Auth: jwt", "Architecture: modular"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q:\n%s", want, got)
		}
	}
	for _, no := range []string{"inventory", "shipping", "reports", "stock-sync"} {
		if strings.Contains(got, no) {
			t.Fatalf("irrelevant %q kept:\n%s", no, got)
		}
	}
	if len(got) >= len(ArchitectureBrief(p)) {
		t.Fatal("scoped brief must be smaller than the full one")
	}
}

func TestTaskBriefFallsBackToFull(t *testing.T) {
	p := scopedProject()
	full := ArchitectureBrief(p)
	for name, tk := range map[string]*core.Task{
		"scaffold":    {Title: "Billing setup", Area: "scaffold"},
		"integration": {Title: "wire billing", Area: "integration"},
		"no match":    {Title: "Something unrelated", Area: "api"},
	} {
		if got := TaskArchitectureBrief(p, tk); got != full {
			t.Fatalf("%s: must keep full blueprint when scope is unknown", name)
		}
	}
	if TaskArchitectureBrief(&core.Project{}, &core.Task{}) != "" {
		t.Fatal("no blueprint, no brief")
	}
}

func TestTaskPromptSizeBaseline(t *testing.T) {
	p := scopedProject()
	for i := 0; i < 30; i++ {
		p.Blueprint.Modules = append(p.Blueprint.Modules, core.Module{ID: fmt.Sprintf("mod%02d", i), Responsibility: strings.Repeat("responsibility ", 8)})
		p.Blueprint.Database.Entities = append(p.Blueprint.Database.Entities, core.Entity{Name: fmt.Sprintf("ent%02d", i), Module: fmt.Sprintf("mod%02d", i), Fields: []string{"id", "name", "created_at"}})
	}
	tk := &core.Task{ID: "TASK-002", Title: "Billing invoice API", Area: "api", Files: []string{"modules/billing/api.go"}, AcceptanceCriteria: []string{"ok"}}
	before, after := len(ArchitectureBrief(p)), len(TaskArchitectureBrief(p, tk))
	t.Logf("architecture brief chars: full=%d scoped=%d", before, after)
	if after*2 > before {
		t.Fatalf("expected ≥50%% smaller on a 35-module blueprint: %d -> %d", before, after)
	}
}

func TestTaskBriefFollowsIndirectContracts(t *testing.T) {
	p := scopedProject()
	// shipping -> inventory -> reports (chain), billing unrelated to them.
	for i := range p.Blueprint.Modules {
		switch p.Blueprint.Modules[i].ID {
		case "shipping":
			p.Blueprint.Modules[i].DependsOn = []string{"inventory"}
		case "inventory":
			p.Blueprint.Modules[i].DependsOn = []string{"reports"}
		}
	}
	tk := &core.Task{ID: "TASK-009", Title: "Shipping labels", Area: "api"}
	got := TaskArchitectureBrief(p, tk)
	for _, want := range []string{"- shipping:", "- inventory:", "- reports:", "reports_row"} {
		if !strings.Contains(got, want) {
			t.Fatalf("transitive dependency %q dropped:\n%s", want, got)
		}
	}
	if strings.Contains(got, "billing") {
		t.Fatalf("unrelated module kept:\n%s", got)
	}
}

func TestTaskBriefUsesDependencyTasksAndRelations(t *testing.T) {
	p := scopedProject()
	p.Blueprint.Database.Entities[0].Relates = []string{"users_row"} // billing_row -> users_row
	p.Blueprint.Database.Entities = append(p.Blueprint.Database.Entities, core.Entity{Name: "audit_log", Module: "reports"})
	p.Tasks = []core.Task{{ID: "TASK-001", Title: "Inventory stock levels", Files: []string{"modules/inventory"}, Status: core.TaskDone}}
	// The task text never names inventory; its dependency does.
	tk := &core.Task{ID: "TASK-002", Title: "Expose stock endpoint", Area: "api", DependsOn: []string{"TASK-001"}}
	got := TaskArchitectureBrief(p, tk)
	if !strings.Contains(got, "- inventory:") || !strings.Contains(got, "inventory_row") {
		t.Fatalf("module of a dependency task dropped:\n%s", got)
	}
	tk2 := &core.Task{ID: "TASK-003", Title: "Billing totals", Area: "api"}
	if g := TaskArchitectureBrief(p, tk2); !strings.Contains(g, "users_row") || strings.Contains(g, "audit_log") {
		t.Fatalf("related entity must follow, unrelated must not:\n%s", g)
	}
}

func TestTaskBriefKeepsGlobalRulesAndSecurity(t *testing.T) {
	p := scopedProject()
	p.Blueprint.Authz = core.AuthzArch{Model: "rbac", Convention: "resource:action"}
	p.Blueprint.Coding.Backend = []string{"validate all input"}
	p.Blueprint.AIGuidelines = []string{"never log secrets"}
	tk := &core.Task{ID: "TASK-002", Title: "Billing", Area: "api"}
	got := TaskArchitectureBrief(p, tk)
	for _, want := range []string{"Auth: jwt", "Authz: rbac", "validate all input", "never log secrets"} {
		if !strings.Contains(got, want) {
			t.Fatalf("global/security rule %q dropped:\n%s", want, got)
		}
	}
	if full := TaskPrompt(p, tk); !strings.Contains(full, "FOUNDRY RULES") || !strings.Contains(full, "ORM only in repository") {
		t.Fatal("task prompt lost mandatory rules")
	}
}

func TestTaskBriefGatesFrontendAndBackendRulesByArea(t *testing.T) {
	p := scopedProject()
	p.Blueprint.Coding.Backend = []string{"repo only"}
	p.Blueprint.Coding.Frontend = []string{"tokens only"}
	p.Blueprint.Frontend.State = "zustand"
	p.Blueprint.Backend.Pattern = "layered"
	has := func(tk *core.Task, s string) bool { return strings.Contains(TaskArchitectureBrief(p, tk), s) }
	api := &core.Task{Title: "Billing", Area: "api"}
	ui := &core.Task{Title: "Billing screen", Area: "ui"}
	test := &core.Task{Title: "Billing tests", Area: "test"}
	if !has(api, "repo only") || has(api, "tokens only") || has(api, "zustand") {
		t.Fatal("api task: backend rules yes, frontend no")
	}
	if has(ui, "repo only") || has(ui, "layered") || !has(ui, "tokens only") || !has(ui, "zustand") {
		t.Fatal("ui task: frontend rules yes, backend no")
	}
	if !has(test, "repo only") || !has(test, "tokens only") {
		t.Fatal("other areas keep both")
	}
	if full := ArchitectureBrief(p); !strings.Contains(full, "tokens only") || !strings.Contains(full, "repo only") {
		t.Fatal("unscoped brief must keep everything")
	}
}

func TestTaskPromptRulesFollowArea(t *testing.T) {
	p := scopedProject()
	prompt := func(area string, pages ...string) string {
		return TaskPrompt(p, &core.Task{ID: "T", Title: "Billing", Area: area, PageIDs: pages, AcceptanceCriteria: []string{"ok"}})
	}
	const (
		frontend = "7. Frontend is not AI slop"
		redis    = "4. Redis required"
		outbox   = "Outbox: write the event"
		webhook  = "Inbound webhook"
	)
	global := []string{"1. Modular + Clean", "3. ORM required", "5. Cross-module side effects", "8. Structure is the contract", "Stop and ask", "WORK RULES", "ORM only in repository"}
	cases := []struct {
		area     string
		has, not []string
	}{
		{"api", []string{redis, outbox}, []string{frontend, webhook}},
		{"data", []string{redis, outbox}, []string{frontend, webhook}},
		{"ui", []string{frontend, "UI RULES"}, []string{redis, outbox, webhook}},
		{"scaffold", []string{frontend, redis, outbox, webhook}, nil},
		{"test", []string{frontend, redis, outbox, webhook}, nil},
		{"", []string{frontend, redis, outbox, webhook}, nil}, // unknown area: everything
	}
	for _, c := range cases {
		got := prompt(c.area)
		for _, w := range append(append([]string{}, global...), c.has...) {
			if !strings.Contains(got, w) {
				t.Fatalf("area %q lost %q", c.area, w)
			}
		}
		for _, w := range c.not {
			if strings.Contains(got, w) {
				t.Fatalf("area %q should not carry %q", c.area, w)
			}
		}
		if !strings.Contains(got, "ACCEPTANCE CRITERIA") {
			t.Fatalf("area %q lost acceptance criteria", c.area)
		}
	}
	// A page makes an api task a UI task too; a webhook task gets webhook rules.
	if got := prompt("api", "PAGE-1"); !strings.Contains(got, frontend) {
		t.Fatal("task with a page must carry the frontend rule")
	}
	wh := TaskPrompt(p, &core.Task{ID: "T", Title: "Receive payment webhook", Area: "api", AcceptanceCriteria: []string{"ok"}})
	if !strings.Contains(wh, webhook) {
		t.Fatal("webhook task must carry webhook rules")
	}
}

func TestRulesVersionInvalidatesMemory(t *testing.T) {
	p, dep, _ := memProject(t)
	if rulesVersion() == "" {
		t.Fatal("no version")
	}
	if contractHashWith(p, dep, "rules-v1") == contractHashWith(p, dep, "rules-v2") {
		t.Fatal("a rule change must change the contract hash")
	}
	if len(MemoryStale(p, dep)) != 0 {
		t.Fatalf("unchanged rules must keep memory valid: %v", MemoryStale(p, dep))
	}
}
