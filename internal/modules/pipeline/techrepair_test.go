package pipeline_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/agent"
	"github.com/handrawanwdev/foundry/internal/modules/pipeline"
	"github.com/handrawanwdev/foundry/internal/modules/techplan"
	"github.com/handrawanwdev/foundry/internal/modules/work"
)

func section(t *testing.T, p *core.Project, id string) *core.BlueprintSection {
	t.Helper()
	s := p.BlueprintProgress.Section(id)
	if s == nil {
		t.Fatalf("no section %s", id)
	}
	return s
}

func logged(p *core.Project, text string) bool {
	for _, e := range p.Events {
		if strings.Contains(e.Message, text) {
			return true
		}
	}
	return false
}

// escalateTo makes the service escalate to a runner whose fake agent
// answers correctly.
func escalateTo(s *pipeline.Service) {
	s.Repair.Escalation = func(*agent.Runner) string { return "sonnet" }
	s.Repair.WithModel = func(_ *agent.Runner, model string) (*agent.Runner, error) {
		if model != "sonnet" {
			return nil, errors.New("unexpected model " + model)
		}
		return agent.NewRunner("fake", []string{os.Args[0], "escalated"}, agent.Options{})
	}
}

// The real failure: alternatives without pros and cons. One targeted repair
// completes them, then architecture and repository follow without anything
// before them running again.
func TestBlueprintTechAlternativesRepairedThenDependentsContinue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runs")
	t.Setenv("FAKE_RUNS", path)
	s := newService(t, fakeAI{}, "tech_alts_once")
	p := designReadyKeepPlan(t, s)
	n := runs(t, path)
	if n["plan-repair"] != 1 || n["discovery"] != 1 {
		t.Fatalf("runs = %v", n)
	}
	if p.Stack.Plan == nil || p.Stack.PlanRequest != nil || p.Structure.Plan == nil || p.Structure.Repo == nil {
		t.Fatalf("tech=%v request=%+v arch=%v repo=%v job=%+v", p.Stack.Plan != nil, p.Stack.PlanRequest, p.Structure.Plan != nil, p.Structure.Repo != nil, p.Job)
	}
	for _, it := range p.Stack.Plan.Items {
		for _, a := range it.Alternatives {
			if missing := techplan.MissingAlternative(a); len(missing) > 0 {
				t.Fatalf("%s alternative %s misses %v", it.Area, a.Technology, missing)
			}
		}
	}
	tech := section(t, p, "tech")
	if tech.Status != core.BPCompleted || tech.Attempt != 2 || tech.Executions[1].Trigger != core.BPTriggerRepair {
		t.Fatalf("tech section = %s attempt %d executions %+v", tech.Status, tech.Attempt, tech.Executions)
	}
	if !strings.Contains(tech.Executions[0].Error, "alternatif belum lengkap") {
		t.Fatalf("refusal not diagnosed: %q", tech.Executions[0].Error)
	}
	for _, id := range []string{"arch", "repo"} {
		if st := section(t, p, id).Status; st != core.BPCompleted {
			t.Fatalf("%s = %s", id, st)
		}
	}
	if !logged(p, "Perbaikan terarah rencana teknologi lolos validasi") {
		t.Fatal("the repair is not reported")
	}
	if p.BlueprintProgress.Status != core.BPRunCompleted {
		t.Fatalf("run = %s", p.BlueprintProgress.Status)
	}
}

// Identical refusals: the second shows no progress, so the third attempt
// runs on the stronger model.
func TestBlueprintTechStalledRepairEscalates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runs")
	t.Setenv("FAKE_RUNS", path)
	s := newService(t, fakeAI{}, "tech_invalid")
	escalateTo(s)
	p := designReadyKeepPlan(t, s)
	n := runs(t, path)
	if n["plan-repair"] != 2 || n["plan-escalated"] < 1 {
		t.Fatalf("runs = %v", n)
	}
	if p.Stack.Plan == nil || p.Stack.PlanRequest != nil || p.Structure.Repo == nil {
		t.Fatalf("not landed after escalation: job=%+v", p.Job)
	}
	if !logged(p, "Eskalasi model default → sonnet") {
		t.Fatal("the escalation is not logged")
	}
	tech := section(t, p, "tech")
	if tech.Attempt != 3 || !strings.Contains(tech.Tasks[len(tech.Tasks)-2].Title, "Memperbaiki output") {
		t.Fatalf("tech attempts = %d tasks %+v", tech.Attempt, tech.Tasks)
	}
}

// Without a stronger model the chain stops after the first attempt that
// fixes nothing. The draft stays; architecture and repository wait as
// blocked; a retry continues the repair and the dependents follow.
func TestBlueprintTechStallStopsKeepsDraftAndResumes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runs")
	t.Setenv("FAKE_RUNS", path)
	s := newService(t, fakeAI{}, "tech_invalid")
	p := designReadyKeepPlan(t, s)
	if n := runs(t, path); n["plan-repair"] != 1 || n["plan"] != 2 {
		t.Fatalf("runs = %v", n)
	}
	mustJob(t, p, work.KindPlan, core.JobFailed)
	r := p.Stack.PlanRequest
	if r == nil || r.Draft == nil || len(r.Attempts) != 2 || r.Attempts[1].Progress != core.TechProgressStalled {
		t.Fatalf("request = %+v", r)
	}
	if !strings.Contains(p.Job.Error, "keputusan teknologi yang sudah valid disimpan") {
		t.Fatalf("job error = %q", p.Job.Error)
	}
	tech := section(t, p, "tech")
	if tech.Status != core.BPFailed || !strings.Contains(strings.Join(tech.Issues, "\n"), "tidak ada progres") {
		t.Fatalf("tech = %s issues %v", tech.Status, tech.Issues)
	}
	for _, id := range []string{"arch", "repo"} {
		if st := section(t, p, id).Status; st != core.BPBlocked {
			t.Fatalf("%s = %s, want blocked", id, st)
		}
	}

	t.Setenv("FAKE_MODE", "")
	if err := s.RetryJob(); err != nil {
		t.Fatal(err)
	}
	if err := s.RetryJob(); err == nil {
		t.Fatal("a second retry started the job again")
	}
	s.Wait()
	p = load(t, s)
	n := runs(t, path)
	if n["plan-repair"] != 2 || n["discovery"] != 1 {
		t.Fatalf("runs after retry = %v", n)
	}
	if p.Stack.Plan == nil || p.Stack.PlanRequest != nil || p.Structure.Plan == nil || p.Structure.Repo == nil {
		t.Fatalf("not resumed: job=%+v", p.Job)
	}
	for _, id := range []string{"tech", "arch", "repo"} {
		if st := section(t, p, id).Status; st != core.BPCompleted {
			t.Fatalf("%s = %s", id, st)
		}
	}
}

func TestBlueprintTechAttemptLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runs")
	t.Setenv("FAKE_RUNS", path)
	s := newService(t, fakeAI{}, "tech_invalid")
	escalateTo(s)
	s.Repair.MaxAttempts = 2
	p := designReadyKeepPlan(t, s)
	if n := runs(t, path); n["plan-repair"] != 1 || n["plan-escalated"] != 0 {
		t.Fatalf("runs = %v", n)
	}
	mustJob(t, p, work.KindPlan, core.JobFailed)
}
