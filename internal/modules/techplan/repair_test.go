package techplan

import (
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
)

func TestMergeScopeLocksAndDuplicates(t *testing.T) {
	draft := &core.TechPlan{Items: []core.TechItem{
		{Area: "backend", Technology: "Go"},
		{Area: "cache", Technology: "Redis"},
		{Area: "cache", Technology: "Valkey"},
		{Area: "database", Technology: "PostgreSQL"},
	}}
	issues := []core.TechIssue{
		{Area: "backend", Code: CodeAltIncomplete},
		{Area: "cache", Code: CodeDuplicateArea},
		{Area: "mobile", Code: CodePlatformMissing},
		{Code: CodeEvaluation},
	}
	sc := RepairScope(draft, issues)
	if !sc.Locked["backend"] || !sc.AltOnly["backend"] || sc.Locked["cache"] || len(sc.Missing) != 1 || !sc.Evaluation || sc.Consistency {
		t.Fatalf("scope = %+v", sc)
	}
	plan, res := Merge(draft, issues, Fix{
		Items:        []core.TechItem{{Area: "cache", Technology: "Valkey"}, {Area: "Mobile", Technology: "Flutter"}, {Area: "database", Technology: "MySQL"}},
		Alternatives: map[string][]core.TechAlternative{"backend": {{Technology: "Rust", Why: "w", Difference: "d", Pros: []string{"p"}, Cons: []string{"c"}}}},
		Evaluation:   &core.TechEvaluation{Criteria: []core.TechCriterion{{Name: "security", Score: 4}}},
		Consistency:  []core.TechCheck{{Area: "security"}},
	})
	if n := len(plan.Items); n != 4 || plan.Tech("cache") != "Valkey" || plan.Tech("mobile") != "Flutter" || plan.Tech("database") != "PostgreSQL" {
		t.Fatalf("items = %+v", plan.Items)
	}
	if len(plan.Item("backend").Alternatives) != 1 || len(plan.Evaluation.Criteria) != 1 || len(plan.Consistency) != 0 {
		t.Fatalf("plan = %+v", plan)
	}
	if len(res.Ignored) != 1 || len(draft.Items) != 4 || draft.Item("backend").Alternatives != nil {
		t.Fatalf("ignored=%v; the draft itself must stay untouched", res.Ignored)
	}
}
