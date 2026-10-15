package pipeline

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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
