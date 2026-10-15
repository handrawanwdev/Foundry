package pipeline_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/handrawanw/foundry/internal/core"
	"github.com/handrawanw/foundry/internal/modules/agent"
	"github.com/handrawanw/foundry/internal/modules/pipeline"
	"github.com/handrawanw/foundry/internal/modules/work"
)

// retryTwice calls RetryJob from two goroutines released together.
func retryTwice(s *pipeline.Service) []error {
	errs := make([]error, 2)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs[i] = s.RetryJob()
		}()
	}
	close(start)
	wg.Wait()
	return errs
}

// oneAccepted fails unless exactly one retry started the job and the other
// was refused as busy or as having nothing to retry.
func oneAccepted(t *testing.T, errs []error) {
	t.Helper()
	accepted := 0
	for _, err := range errs {
		switch {
		case err == nil:
			accepted++
		case !errors.Is(err, pipeline.ErrBusy) && !errors.Is(err, pipeline.ErrNothingToDo):
			t.Fatalf("unexpected retry error: %v", err)
		}
	}
	if accepted != 1 {
		t.Fatalf("accepted retries = %d, want 1 (%v)", accepted, errs)
	}
}

func runsOf(t *testing.T, path, label string) int {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	n := 0
	for _, l := range strings.Split(string(b), "\n") {
		if l == label {
			n++
		}
	}
	return n
}

func countLog(p *core.Project, msg string) int {
	n := 0
	for _, e := range p.Events {
		if e.Message == msg {
			n++
		}
	}
	return n
}

func TestConcurrentRetryStartsJobOnce(t *testing.T) {
	s := newService(t, fakeAI{}, "bad_json")
	if _, err := s.SubmitStory("Aplikasi keuangan pribadi untuk mencatat"); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	mustJob(t, load(t, s), work.KindDiscovery, core.JobFailed)

	runs := filepath.Join(t.TempDir(), "runs")
	t.Setenv("FAKE_MODE", "")
	t.Setenv("FAKE_RUNS", runs)
	oneAccepted(t, retryTwice(s))
	s.Wait()

	if n := runsOf(t, runs, "discovery"); n != 1 {
		t.Fatalf("discovery agent runs = %d, want 1", n)
	}
	p := load(t, s)
	mustJob(t, p, work.KindDesign, core.JobDone) // the applyAI chain still runs
	if n := countLog(p, "Memahami kebutuhan selesai"); n != 1 {
		t.Fatalf("discovery applied %d times", n)
	}
}

func TestConcurrentPlanRetryStartsJobOnce(t *testing.T) {
	s := newService(t, fakeAI{}, "plan_fail")
	designReady(t, s)
	if err := approve(s); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	mustJob(t, load(t, s), work.KindPlan, core.JobFailed)

	runs := filepath.Join(t.TempDir(), "runs")
	t.Setenv("FAKE_MODE", "")
	t.Setenv("FAKE_RUNS", runs)
	oneAccepted(t, retryTwice(s))
	s.Wait()

	if n := runsOf(t, runs, "plan"); n != 1 {
		t.Fatalf("plan agent runs = %d, want 1", n)
	}
	mustJob(t, load(t, s), work.KindPlan, core.JobDone)
}

// switchAI is an agent connection that can turn into an MCP host one.
type switchAI struct{ mcp *atomic.Bool }

func (a switchAI) Runner() (*agent.Runner, bool, error) {
	return fakeAI{mcp: a.mcp.Load()}.Runner()
}

func resetTask(t *testing.T, s *pipeline.Service) {
	t.Helper()
	if _, err := s.Store.Update(func(p *core.Project) error {
		p.Stage = core.StageBuild
		p.Job = nil
		p.Tasks = []core.Task{{ID: "T1", Title: "a", Area: "api", Files: []string{"a/x.go"}, AcceptanceCriteria: []string{"ok"}, Status: core.TaskTodo}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// A finished agent build must leave neither a stale stop for the next agent
// build nor one that hides a host build from StopBuild.
func TestStopAfterFinishedAgentBuild(t *testing.T) {
	mcp := &atomic.Bool{}
	s := newService(t, switchAI{mcp}, "crash")
	resetTask(t, s)
	if err := s.StartBuild(); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	mustJob(t, load(t, s), "build", core.JobFailed)

	// A second agent build is stopped by its own cancel func.
	resetTask(t, s)
	t.Setenv("FAKE_MODE", "hang")
	if err := s.StartBuild(); err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(10 * time.Second); load(t, s).Task("T1").Status != core.TaskInProgress; time.Sleep(20 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("task not started")
		}
	}
	if err := s.StopBuild(); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	if p := load(t, s); p.Job.Message != "Build dihentikan" || p.Task("T1").Status != core.TaskTodo {
		t.Fatalf("agent build not stopped: job=%+v task=%s", p.Job, p.Task("T1").Status)
	}

	// The same service now waits for an MCP host; Stop must end that build.
	mcp.Store(true)
	resetTask(t, s)
	if err := s.StartBuild(); err != nil {
		t.Fatal(err)
	}
	if w, err := s.NextWork("w1"); err != nil || w.TaskID != "T1" {
		t.Fatalf("claim = %+v %v", w, err)
	}
	if err := s.StopBuild(); err != nil {
		t.Fatal(err)
	}
	p := load(t, s)
	mustJob(t, p, "build", core.JobDone)
	if tk := p.Task("T1"); tk.Status != core.TaskTodo || tk.Owner != "" {
		t.Fatalf("host task not released: %s owner %q", tk.Status, tk.Owner)
	}
	if err := s.StopBuild(); !errors.Is(err, pipeline.ErrNothingToDo) {
		t.Fatalf("stop with nothing running = %v, want ErrNothingToDo", err)
	}
}
