package pipeline_test

import (
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/pipeline"
	"github.com/handrawanwdev/foundry/internal/modules/progress"
)

func seedWorkflow(t *testing.T, s *pipeline.Service) {
	t.Helper()
	if _, err := s.Store.Update(func(p *core.Project) error {
		p.Workflow = &core.Workflow{Version: 1, Commands: []core.Command{
			{ID: "CMD-001", Name: "Skema", Objective: "o", Instructions: "i", Acceptance: []string{"migrasi jalan"}},
			{ID: "CMD-002", Name: "API", Objective: "o", Instructions: "i", DependsOn: []string{"CMD-001"}},
		}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestProgressPersistsAcrossReload(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	seedWorkflow(t, s)
	in := progress.ReportIn{ID: "r1", Command: "CMD-001", Status: core.RunDone, Source: "codex",
		Evidence: []core.Evidence{{Kind: "files", Ref: "db/schema.sql"}}}
	out, err := s.ReportProgress(in, "mcp")
	if err != nil || out.AppliedStatus != core.RunVerified {
		t.Fatalf("report = %+v err=%v", out, err)
	}
	s2 := pipeline.New(s.Store, fakeAI{})
	snap, err := s2.ProgressView()
	if err != nil {
		t.Fatal(err)
	}
	if snap.Commands[0].Status != core.RunVerified || snap.Commands[0].ReportID != "r1" {
		t.Fatalf("reload lost progress: %+v", snap.Commands)
	}
	if snap.Complete {
		t.Fatal("workflow marked complete with CMD-002 unverified")
	}
	p := load(t, s2)
	if len(p.Events) == 0 || !strings.Contains(p.Events[len(p.Events)-1].Message, "CMD-001") {
		t.Fatalf("events = %+v", p.Events)
	}
}

func TestProgressDoesNotRunWorkflow(t *testing.T) {
	s := newService(t, fakeAI{}, "boom")
	seedWorkflow(t, s)
	if _, err := s.ReportProgress(progress.ReportIn{Command: "CMD-001", Status: core.RunStarted, Source: "qwen"}, "agent"); err != nil {
		t.Fatal(err)
	}
	p := load(t, s)
	if p.Job != nil && p.Job.Status == core.JobRunning {
		t.Fatalf("progress started a job: %+v", p.Job)
	}
}
