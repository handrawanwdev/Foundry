package pipeline

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/handrawanw/foundry/internal/core"
)

func newStateService(t *testing.T) *Service {
	t.Helper()
	t.Setenv("FOUNDRY_HOME", t.TempDir())
	st, err := core.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Init("test"); err != nil {
		t.Fatal(err)
	}
	return New(st, nil)
}

func breakState(t *testing.T, s *Service) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(s.Store.Dir, core.StateDir, "project.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestEndBuildKeepsNewerCancel(t *testing.T) {
	s := &Service{}
	old := s.beginBuild(func() {})
	newer := s.beginBuild(func() {})
	s.endBuild(old) // the old build's cleanup runs after the new one started
	if s.cancel == nil {
		t.Fatal("old build cleared the cancel func of the newer build")
	}
	s.endBuild(newer)
	if s.cancel != nil {
		t.Fatal("finished build left its cancel func behind")
	}
}

func TestVerifyNowWithBrokenState(t *testing.T) {
	s := newStateService(t)
	breakState(t, s)
	s.verifyNow(context.Background()) // must return, not panic
}

func TestLateFailureKeepsNewerJob(t *testing.T) {
	s := newStateService(t)
	if _, err := s.Store.Update(func(p *core.Project) error {
		p.Job = &core.Job{Kind: "build", Status: core.JobRunning}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	s.failJobIf("verify", errors.New("boom"), runningJob("verify"))
	p, err := s.Store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if p.Job.Kind != "build" || p.Job.Status != core.JobRunning {
		t.Fatalf("job overwritten: %+v", p.Job)
	}
}

func TestReviewMissingTaskOrState(t *testing.T) {
	s := newStateService(t)
	crit, err := s.review(context.Background(), nil, "GONE", "", s.Store.Dir, nil, nil, &core.BuildRun{})
	if crit != nil || err != nil {
		t.Fatalf("missing task: crit=%v err=%v", crit, err)
	}
	breakState(t, s)
	crit, err = s.review(context.Background(), nil, "GONE", "", s.Store.Dir, nil, nil, &core.BuildRun{})
	if crit != nil || err == nil {
		t.Fatalf("broken state: crit=%v err=%v", crit, err)
	}
}

// BLOCKED must carry a traceable cause: attempt, last error, recovery. The
// evidence (notes) stays, and leaving BLOCKED clears the reason.
func TestRejectTaskRecordsBlockedReason(t *testing.T) {
	s := newStateService(t)
	vet := "Verifikasi gagal: vet gagal (go vet ./...)\napps/api/main.go:18:2: no required module provides package github.com/gin-gonic/gin"
	s.Store.Update(func(p *core.Project) error {
		p.Tasks = []core.Task{{ID: "T1", Status: core.TaskInProgress, Attempts: 1}}
		return nil
	})
	s.rejectTask("T1", vet)
	tk := mustLoad(t, s.Store).Task("T1")
	if tk.Status != core.TaskTodo || tk.BlockedReason != "" {
		t.Fatalf("first failure retries: %s %q", tk.Status, tk.BlockedReason)
	}
	s.Store.Update(func(p *core.Project) error { p.Task("T1").Attempts = 2; return nil })
	s.rejectTask("T1", vet)
	tk = mustLoad(t, s.Store).Task("T1")
	if tk.Status != core.TaskBlocked || len(tk.Notes) != 2 {
		t.Fatalf("identical failure blocks, keeps evidence: %s notes %d", tk.Status, len(tk.Notes))
	}
	for _, want := range []string{"Percobaan ke-2", "no required module provides", "Perbaiki Terblokir"} {
		if !strings.Contains(tk.BlockedReason, want) {
			t.Fatalf("reason %q lacks %q", tk.BlockedReason, want)
		}
	}
	// A running build picks the reopened task up; none is started here.
	s.Store.Update(func(p *core.Project) error {
		p.Job = &core.Job{Kind: "build", Status: core.JobRunning}
		return nil
	})
	if err := s.FixBlocked(); err != nil {
		t.Fatal(err)
	}
	tk = mustLoad(t, s.Store).Task("T1")
	if tk.BlockedReason != "" || tk.Status == core.TaskBlocked || tk.Attempts != 0 || len(tk.Notes) != 2 {
		t.Fatalf("after FixBlocked: %+v", tk)
	}
}

// A transient error keeps its attempts: it is retried until MaxAttempts, and
// only then BLOCKED with the attempt count in the reason.
func TestTransientFailureRetriesUntilLimit(t *testing.T) {
	s := newStateService(t)
	note := "Verifikasi gagal: test gagal (npm test)\nError: connect ECONNREFUSED 127.0.0.1:5432"
	s.Store.Update(func(p *core.Project) error {
		p.Tasks = []core.Task{{ID: "T1", Status: core.TaskInProgress, Attempts: 1}}
		return nil
	})
	for n := 1; n < MaxAttempts; n++ {
		s.Store.Update(func(p *core.Project) error { p.Task("T1").Attempts = n; return nil })
		s.rejectTask("T1", note)
		if st := mustLoad(t, s.Store).Task("T1").Status; st != core.TaskTodo {
			t.Fatalf("attempt %d: %s, want TODO", n, st)
		}
	}
	s.Store.Update(func(p *core.Project) error { p.Task("T1").Attempts = MaxAttempts; return nil })
	s.rejectTask("T1", note)
	tk := mustLoad(t, s.Store).Task("T1")
	if tk.Status != core.TaskBlocked || !strings.Contains(tk.BlockedReason, "batas 3 percobaan") || !strings.Contains(tk.BlockedReason, "ECONNREFUSED") {
		t.Fatalf("%s %q", tk.Status, tk.BlockedReason)
	}
}

// A dependency fetch that fails on the network is an environment problem:
// retried up to MaxAttempts, then BLOCKED with an environment recovery step.
func TestDependencyFetchFailureIsEnvironment(t *testing.T) {
	s := newStateService(t)
	note := "Verifikasi gagal: install-go gagal (go mod tidy)\ndial tcp: lookup proxy.golang.org: no such host"
	s.Store.Update(func(p *core.Project) error {
		p.Tasks = []core.Task{{ID: "T1", Status: core.TaskInProgress, Attempts: 1, Notes: []string{note}}}
		return nil
	})
	s.rejectTask("T1", note)
	if st := mustLoad(t, s.Store).Task("T1").Status; st != core.TaskTodo {
		t.Fatalf("identical network failure at attempt 1 = %s, want TODO", st)
	}
	s.Store.Update(func(p *core.Project) error { p.Task("T1").Attempts = MaxAttempts; return nil })
	s.rejectTask("T1", note)
	tk := mustLoad(t, s.Store).Task("T1")
	if tk.Status != core.TaskBlocked || !strings.Contains(tk.BlockedReason, "lingkungan") {
		t.Fatalf("%s %q", tk.Status, tk.BlockedReason)
	}
}
