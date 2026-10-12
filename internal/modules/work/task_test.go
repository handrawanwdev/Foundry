package work

import (
	"strings"
	"testing"

	"github.com/handrawanw/foundry/internal/core"
)

func TestParseReviewSkipsProgressJSON(t *testing.T) {
	tk := &core.Task{AcceptanceCriteria: []string{"A", "B"}}
	raw := `{"task_id":"TASK-001","progress":"Menulis skema"}
` + "```json\n" + `{"criteria":[{"index":1,"met":true,"evidence":"a.go handler"},{"index":2,"met":true,"evidence":"b.go test"}]}` + "\n```"
	crit, err := ParseReview(tk, raw)
	if err != nil {
		t.Fatal(err)
	}
	if !AllMet(crit) || crit[0].Evidence != "a.go handler" {
		t.Fatalf("crit = %+v", crit)
	}
}

func TestSelfReviewNeedsCriteria(t *testing.T) {
	tk := &core.Task{AcceptanceCriteria: []string{"A"}}
	if _, ok := SelfReview(tk, "done"); ok {
		t.Fatal("plain output should not self-review")
	}
	crit, ok := SelfReview(tk, `{"criteria":[{"index":1,"met":true,"evidence":"x.go"}]}`)
	if !ok || !AllMet(crit) {
		t.Fatalf("self = %v %+v", ok, crit)
	}
	crit, err := ParseReview(tk, `{"criteria":[{"index":1,"met":true,"evidence":""}]}`)
	if err != nil || AllMet(crit) {
		t.Fatalf("empty evidence should not meet: %v %+v", err, crit)
	}
}

func TestContinuePromptKeepsPartial(t *testing.T) {
	p := &core.Project{Name: "Demo"}
	tk := &core.Task{ID: "TASK-001", Title: "x", Description: "d", Area: "api",
		AcceptanceCriteria: []string{"ok"}, Files: []string{"app/x.go"}, Notes: []string{"tes gagal"}}
	got := ContinuePrompt(p, tk)
	for _, want := range []string{"CONTINUE FROM PARTIAL", "Do not start over", "TASK-001", "app/x.go", "FEEDBACK", "tes gagal"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "PROBLEM TO FIX") || strings.Contains(got, "Implement ONLY this task") {
		t.Fatalf("continue prompt looks like a fresh start:\n%s", got)
	}
}

func TestTaskPromptAsksSelfReview(t *testing.T) {
	p := &core.Project{Name: "Demo"}
	tk := &core.Task{ID: "TASK-001", Title: "x", Description: "d", Area: "api", AcceptanceCriteria: []string{"ok"}}
	got := TaskPrompt(p, tk)
	if !strings.Contains(got, "skip a second review pass") || !strings.Contains(got, `"criteria"`) {
		t.Fatalf("task prompt missing self-review: %s", got)
	}
	if !strings.Contains(got, "coding agent") {
		t.Fatalf("task prompt missing coding agent: %s", got)
	}
}

func TestTaskPromptPointsToPRD(t *testing.T) {
	p := &core.Project{Name: "Demo", PRD: &core.PRD{Markdown: "# 1. Product Overview\n"}}
	tk := &core.Task{ID: "TASK-002", Title: "x", Description: "d", Area: "api", AcceptanceCriteria: []string{"ok"}}
	got := TaskPrompt(p, tk)
	if !strings.Contains(got, "docs/PRD.md") {
		t.Fatalf("task prompt missing PRD: %s", got)
	}
}

func TestTaskPromptOwnership(t *testing.T) {
	p := &core.Project{Name: "Demo", Tasks: []core.Task{
		{ID: "TASK-001", Area: "api", Status: core.TaskInProgress, Files: []string{"api/users.go"}},
		{ID: "TASK-002", Area: "ui", Status: core.TaskInProgress, Files: []string{"apps/web/src/App.tsx"}, PageIDs: []string{"PAGE-002"}},
		{ID: "TASK-003", Area: "api", Status: core.TaskDone, Files: []string{"api/done.go"}},
		{ID: "TASK-004", Area: "api", Status: core.TaskTodo, Files: []string{"api/orders.go", "api/users.go/x.go"}},
	}}
	got := TaskPrompt(p, &p.Tasks[0])
	for _, want := range []string{
		"You own:\n- api/users.go\n",
		"Running now (do NOT edit, another agent is changing them):",
		"- apps/web/src/App.tsx (TASK-002)",
		"- page PAGE-002 (TASK-002)",
		"Owned by other tasks (do not edit):\n- api/orders.go (TASK-004)\n",
		`"patches"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
	for _, bad := range []string{"(TASK-001)", "api/done.go", "x.go (TASK-004)", "LIKELY FILES"} {
		if strings.Contains(got, bad) {
			t.Fatalf("unexpected %q in:\n%s", bad, got)
		}
	}
}

func TestParsePatchesAndOwner(t *testing.T) {
	raw := "done\n```json\n{\"criteria\": [{\"index\": 1, \"met\": true, \"evidence\": \"x\"}], \"patches\": [{\"file\": \"./Web/App.tsx\", \"change\": \"add route /orders\"}, {\"file\": \"\", \"change\": \"x\"}]}\n```"
	ps := ParsePatches(raw)
	if len(ps) != 1 || ps[0].Change != "add route /orders" {
		t.Fatalf("patches = %+v", ps)
	}
	p := &core.Project{Tasks: []core.Task{
		{ID: "TASK-001", Status: core.TaskDone, Files: []string{"web/App.tsx"}},
		{ID: "TASK-002", Status: core.TaskTodo, Files: []string{"web"}},
		{ID: "TASK-003", Status: core.TaskInProgress, Files: []string{"api/x.go"}},
	}}
	if o := PatchOwner(p, "TASK-003", ps[0].File); o == nil || o.ID != "TASK-002" {
		t.Fatalf("owner = %+v", o)
	}
	if o := PatchOwner(p, "TASK-002", "web/App.tsx"); o != nil {
		t.Fatalf("own file must not resolve to another owner, got %s", o.ID)
	}
}

// Dependency manifests and lockfiles are never owned: adding a dependency
// is not an edit of another task's file.
func TestForeignOwnerSkipsDependencyFiles(t *testing.T) {
	p := &core.Project{Tasks: []core.Task{
		{ID: "TASK-001", Status: core.TaskInProgress},
		{ID: "TASK-002", Status: core.TaskTodo, Files: []string{"package.json", "apps/web/package-lock.json", "src/db.ts"}},
	}}
	me := &p.Tasks[0]
	for _, f := range []string{"package.json", "apps/web/package-lock.json", "go.sum"} {
		if o := ForeignOwner(p, me, f); o != nil {
			t.Errorf("%s owned by %s", f, o.ID)
		}
	}
	if o := ForeignOwner(p, me, "src/db.ts"); o == nil || o.ID != "TASK-002" {
		t.Errorf("src/db.ts owner = %v", o)
	}
}

func TestBlockedPromptCarriesFailureHistory(t *testing.T) {
	p := &core.Project{Name: "x", Builds: []core.BuildRun{{ID: "B1", Reason: "go test gagal", Steps: []core.StepResult{{Command: "go test ./...", ExitCode: 1, Output: "FAIL pkg TestA"}}}}}
	p.Tasks = []core.Task{
		{ID: "T1", Title: "dep", Status: core.TaskDone},
		{ID: "T2", Title: "x", AcceptanceCriteria: []string{"a", "b"}, DependsOn: []string{"T1"}, Files: []string{"a.go"},
			Notes: []string{"Verifikasi gagal: boom"}, LastBuildID: "B1", Criteria: []core.Criterion{{Met: true}, {Met: false, Evidence: "missing"}}},
	}
	got := BlockedPrompt(p, p.Task("T2"), true)
	for _, want := range []string{"ROOT CAUSE", "Verifikasi gagal: boom", "FAIL pkg TestA", "[NOT MET: missing]", "T1 dep", "previous attempt"} {
		if !strings.Contains(got, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
}
