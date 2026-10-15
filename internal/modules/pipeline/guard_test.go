package pipeline_test

import (
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/handrawanw/foundry/internal/core"
	"github.com/handrawanw/foundry/internal/modules/pipeline"
	"github.com/handrawanw/foundry/internal/modules/work"
)

// together runs fn from n goroutines released at once and returns their errors.
func together(n int, fn func() error) []error {
	errs := make([]error, n)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs[i] = fn()
		}()
	}
	close(start)
	wg.Wait()
	return errs
}

func prdProject(t *testing.T, s *pipeline.Service) {
	t.Helper()
	if _, err := s.Store.Update(func(p *core.Project) error {
		p.Stage = core.StageBuild
		p.Tasks = []core.Task{{ID: "TASK-001", Title: "Scaffold", Area: "scaffold", AcceptanceCriteria: []string{"ok"}, Status: core.TaskTodo}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// onlyOneStarted fails unless exactly one call succeeded and every other
// was refused as busy.
func onlyOneStarted(t *testing.T, errs []error) {
	t.Helper()
	started := 0
	for _, err := range errs {
		switch {
		case err == nil:
			started++
		case !errors.Is(err, pipeline.ErrBusy):
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if started != 1 {
		t.Fatalf("started = %d, want 1 (%v)", started, errs)
	}
}

func TestConcurrentGeneratePRDStartsOnce(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	runs := filepath.Join(t.TempDir(), "runs")
	t.Setenv("FAKE_RUNS", runs)
	prdProject(t, s)
	onlyOneStarted(t, together(4, s.GeneratePRD))
	s.Wait()
	if n := runsOf(t, runs, "prd"); n != 1 {
		t.Fatalf("prd agent runs = %d, want 1", n)
	}
	p := load(t, s)
	mustJob(t, p, work.KindPRD, core.JobDone) // not overwritten by a second run
	if p.PRD == nil {
		t.Fatal("prd missing")
	}
}

func TestConcurrentGeneratePRDWaitsOnceForHost(t *testing.T) {
	s := newService(t, fakeAI{mcp: true}, "")
	prdProject(t, s)
	before := load(t, s)
	onlyOneStarted(t, together(4, s.GeneratePRD))
	p := load(t, s)
	mustJob(t, p, work.KindPRD, core.JobWaiting)
	if len(p.Events) != len(before.Events) {
		t.Fatalf("refused requests changed the log: %d → %d events", len(before.Events), len(p.Events))
	}
	if err := s.GeneratePRD(); !errors.Is(err, pipeline.ErrBusy) {
		t.Fatalf("PRD while waiting for the host = %v, want ErrBusy", err)
	}
}

// verified sets up a project whose verification found a failing build,
// reported twice.
func verified(t *testing.T, s *pipeline.Service) {
	t.Helper()
	if _, err := s.Store.Update(func(p *core.Project) error {
		p.Stage = core.StageVerify
		p.Tasks = []core.Task{{ID: "TASK-001", Title: "Scaffold", Area: "scaffold", AcceptanceCriteria: []string{"ok"}, Status: core.TaskDone}}
		is := core.Issue{Kind: "build_failed", Message: "build gagal: go build ./..."}
		p.Verification = &core.Verification{Issues: []core.Issue{is, is}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func fixTasks(t *testing.T, s *pipeline.Service) []core.Task {
	t.Helper()
	var out []core.Task
	for _, tk := range load(t, s).Tasks {
		if strings.HasPrefix(tk.Title, "Perbaiki: ") {
			out = append(out, tk)
		}
	}
	return out
}

func TestFixAddsNoDuplicateTasks(t *testing.T) {
	s := newService(t, fakeAI{mcp: true}, "")
	verified(t, s)
	if err := s.Fix(); err != nil {
		t.Fatal(err)
	}
	if n := len(fixTasks(t, s)); n != 1 {
		t.Fatalf("fix tasks = %d, want 1 for one issue reported twice", n)
	}
	mustJob(t, load(t, s), "build", core.JobWaiting)

	// The build waits for the host: Fix again is refused and adds nothing.
	if err := s.Fix(); !errors.Is(err, pipeline.ErrBusy) {
		t.Fatalf("Fix while the build waits = %v, want ErrBusy", err)
	}
	if n := len(fixTasks(t, s)); n != 1 {
		t.Fatalf("fix tasks = %d after a refused Fix", n)
	}

	// Stopped before the task ran: a new Fix keeps the open task.
	if err := s.StopBuild(); err != nil {
		t.Fatal(err)
	}
	if err := s.Fix(); err != nil {
		t.Fatal(err)
	}
	if n := len(fixTasks(t, s)); n != 1 {
		t.Fatalf("fix tasks = %d after a second Fix, want 1", n)
	}

	// Once that task is DONE, the same issue found again gets a new one.
	if _, err := s.Store.Update(func(p *core.Project) error {
		p.Job = nil
		for i := range p.Tasks {
			p.Tasks[i].Status = core.TaskDone
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.Fix(); err != nil {
		t.Fatal(err)
	}
	if n := len(fixTasks(t, s)); n != 2 {
		t.Fatalf("fix tasks = %d, want 2 after the first was done", n)
	}
}

func TestConcurrentFixAddsOneTask(t *testing.T) {
	s := newService(t, fakeAI{mcp: true}, "")
	verified(t, s)
	for _, err := range together(4, s.Fix) {
		if err != nil && !errors.Is(err, pipeline.ErrBusy) {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if n := len(fixTasks(t, s)); n != 1 {
		t.Fatalf("fix tasks = %d, want 1", n)
	}
}

// A Fix whose build cannot start leaves no task behind.
func TestFixWithoutAIAddsNothing(t *testing.T) {
	s := newService(t, fakeAI{none: true}, "")
	verified(t, s)
	before := load(t, s)
	for range 2 {
		if err := s.Fix(); !errors.Is(err, pipeline.ErrNoAI) {
			t.Fatalf("Fix without AI = %v, want ErrNoAI", err)
		}
	}
	p := load(t, s)
	if len(p.Tasks) != len(before.Tasks) || p.Job != nil || p.Stage != core.StageVerify {
		t.Fatalf("state changed: %d tasks, job %+v, stage %s", len(p.Tasks), p.Job, p.Stage)
	}
}
