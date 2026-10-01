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

func TestTaskPromptAsksSelfReview(t *testing.T) {
	p := &core.Project{Name: "Demo"}
	tk := &core.Task{ID: "TASK-001", Title: "x", Description: "d", Area: "api", AcceptanceCriteria: []string{"ok"}}
	got := TaskPrompt(p, tk)
	if !strings.Contains(got, "skip a second review pass") || !strings.Contains(got, `"criteria"`) {
		t.Fatalf("task prompt missing self-review: %s", got)
	}
}
