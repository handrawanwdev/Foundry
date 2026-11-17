//go:build unix

package pipeline_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/pipeline"
	"github.com/handrawanwdev/foundry/internal/modules/work"
)

// hangAgents makes every agent run whose prompt holds marker block, and
// returns a function that waits for n of them and gives their pids.
func hangAgents(t *testing.T, marker string) func(n int) []int {
	t.Helper()
	pids := filepath.Join(t.TempDir(), "pids")
	t.Setenv("FAKE_HANG", marker)
	t.Setenv("FAKE_PIDS", pids)
	return func(n int) []int {
		t.Helper()
		for deadline := time.Now().Add(20 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
			raw, _ := os.ReadFile(pids)
			var out []int
			for _, f := range strings.Fields(string(raw)) {
				if pid, err := strconv.Atoi(f); err == nil {
					out = append(out, pid)
				}
			}
			if len(out) >= n {
				return out
			}
		}
		t.Fatalf("fewer than %d agents started", n)
		return nil
	}
}

// agentGone reports whether pid ended (or is a zombie not reaped yet).
func agentGone(pid int) bool {
	if syscall.Kill(pid, 0) == syscall.ESRCH {
		return true
	}
	out, _ := exec.Command("ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output()
	st := strings.TrimSpace(string(out))
	return st == "" || strings.HasPrefix(st, "Z")
}

func mustStopped(t *testing.T, s *pipeline.Service, kind string, pids []int) {
	t.Helper()
	for _, pid := range pids {
		if !agentGone(pid) {
			t.Fatalf("agent %d still runs after stop", pid)
		}
	}
	s.Wait() // nothing chained after the stop
	p := load(t, s)
	mustJob(t, p, kind, core.JobFailed)
	if !p.Job.Stopped || !strings.Contains(p.Job.Error, "Dihentikan") {
		t.Fatalf("job not marked stopped: %+v", p.Job)
	}
	if r := p.BlueprintProgress; r != nil && r.Status == core.BPRunRunning {
		t.Fatalf("run still running after stop: %+v", r.Summary)
	}
}

// Stopping "Merancang Blueprint" kills the agent at once; a retry runs it
// again from where it stopped.
func TestStopJobKillsAgent(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	if _, err := s.SubmitStory("Saya ingin membuat aplikasi klinik untuk janji temu pasien"); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	started := hangAgents(t, "discovery analyst")
	if err := s.StartBlueprint(); err != nil {
		t.Fatal(err)
	}
	pids := started(1)
	t0 := time.Now()
	if err := s.StopJob(); err != nil {
		t.Fatal(err)
	}
	if took := time.Since(t0); took > 10*time.Second {
		t.Fatalf("stop took %s", took)
	}
	mustStopped(t, s, work.KindDiscovery, pids)
	if !errors.Is(s.StopJob(), pipeline.ErrNothingToDo) {
		t.Fatal("a stopped job stopped twice")
	}

	t.Setenv("FAKE_HANG", "")
	if err := s.RetryJob(); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	if p := load(t, s); p.Job == nil || p.Job.Status == core.JobFailed || p.Job.Stopped || p.Stage == core.StageDiscovery {
		t.Fatalf("retry after stop did not continue: stage=%s job=%+v", p.Stage, p.Job)
	}
}

// The blueprint areas run side by side: a stop kills every one of their
// agents and starts nothing after them.
func TestStopJobKillsParallelAreas(t *testing.T) {
	s := newService(t, fakeAI{}, "areas_open")
	if _, err := s.SubmitStory("Saya ingin membuat aplikasi klinik untuk janji temu pasien"); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	started := hangAgents(t, "Lengkapi blueprint: tulis area")
	if err := s.StartBlueprint(); err != nil {
		t.Fatal(err)
	}
	pids := started(2)
	if err := s.StopJob(); err != nil {
		t.Fatal(err)
	}
	mustStopped(t, s, work.KindArtifacts, pids)

	t.Setenv("FAKE_HANG", "")
	if err := s.RetryJob(); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	if p := load(t, s); p.Job == nil || p.Job.Status != core.JobDone {
		t.Fatalf("retry after stop did not finish the blueprint: %+v", p.Job)
	}
}

// A job waiting for the MCP host stops too, and the host's late answer is
// refused.
func TestStopJobWaitingForHost(t *testing.T) {
	s := newService(t, fakeAI{mcp: true}, "")
	if _, err := s.SubmitStory("Saya ingin membuat aplikasi klinik untuk janji temu pasien"); err != nil {
		t.Fatal(err)
	}
	mustJob(t, load(t, s), work.KindFeatureMap, core.JobWaiting)
	if err := s.StopJob(); err != nil {
		t.Fatal(err)
	}
	mustStopped(t, s, work.KindFeatureMap, nil)
	if err := s.SubmitWork(work.KindFeatureMap, featureMapJSON); err == nil {
		t.Fatal("answer of a stopped job accepted")
	}
	if !load(t, s).FeatureMap.Empty() {
		t.Fatal("stopped job's answer landed")
	}
}

func TestStopJobNothingRunning(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	if !errors.Is(s.StopJob(), pipeline.ErrNothingToDo) {
		t.Fatal("stop without a job")
	}
}
