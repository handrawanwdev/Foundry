package pipeline

import (
	"testing"
	"time"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/work"
)

// A plan round that landed part of the plan leaves its section in progress
// for the next round. When that round cannot come (the technology waits for
// the user's decision on a proposal) and no job is left, the run must end
// with the section waiting, not spin as "running" forever.
func TestIdleRunEndsWithSectionWaiting(t *testing.T) {
	p := &core.Project{Stack: &core.TechStack{Proposal: &core.TechProposal{}}}
	ensureBlueprintProgress(p)
	run := p.BlueprintProgress
	now := time.Now().UTC()
	run.BeginExecution(secRepo, core.BPTriggerInitial, "Merancang Struktur Repository", "", now)
	run.BeginValidation(secRepo, "Memvalidasi", now)
	run.AcceptPartial(secRepo, "Struktur Repository valid, masih ada bagian yang perlu dilengkapi", now)
	if !run.Running() {
		t.Fatal("setup: repo not in progress")
	}
	p.Job = &core.Job{Kind: work.KindPlan, Status: core.JobDone}

	finishRunIfDone(p)
	if run.Status != core.BPRunIncomplete {
		t.Fatalf("run %s, want incomplete", run.Status)
	}
	s := run.Section(secRepo)
	if s.Active() || s.Status != core.BPPending || s.Reason != reasonTechProposal {
		t.Fatalf("repo %s %q", s.Status, s.Reason)
	}
	for _, e := range s.Executions {
		if e.Status == core.BPExecRunning {
			t.Fatalf("execution left open: %+v", e)
		}
	}
}

// The stop on a run with no job behind it closes the run.
func TestStopClosesIdleRun(t *testing.T) {
	st, err := core.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Init("test"); err != nil {
		t.Fatal(err)
	}
	st.Update(func(p *core.Project) error {
		ensureBlueprintProgress(p)
		now := time.Now().UTC()
		p.BlueprintProgress.BeginExecution(secRepo, core.BPTriggerInitial, "x", "", now)
		p.Job = &core.Job{Kind: work.KindPlan, Status: core.JobDone}
		return nil
	})
	s := New(st, nil)
	if err := s.StopJob(); err != nil {
		t.Fatal(err)
	}
	p, _ := st.Load()
	if r := p.BlueprintProgress; r.Status == core.BPRunRunning || r.Running() {
		t.Fatalf("run still open: %s", r.Status)
	}
	if err := s.StopJob(); err != ErrNothingToDo {
		t.Fatalf("second stop: %v", err)
	}
}
