package pipeline_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/usage"
	"github.com/handrawanwdev/foundry/internal/modules/work"
)

// Every attempt is one usage record, retries counted; a CLI that reports no
// usage leaves the token fields null.
func TestUsageRecordedPerAttempt(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	s.RetryWait = []time.Duration{time.Millisecond, time.Millisecond}
	designReady(t, s)
	t.Setenv("FAKE_RUNS", filepath.Join(t.TempDir(), "runs"))
	t.Setenv("FAKE_MODE", "overloaded_once")
	if err := s.GenerateWorkflow(); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p := load(t, s)
	mustJob(t, p, work.KindWorkflow, core.JobDone)
	all, bad, err := usage.Read(p.Dir)
	var recs []usage.Record
	for _, r := range all {
		if r.Operation == work.KindWorkflow {
			recs = append(recs, r)
		}
	}
	if err != nil || bad != 0 || len(recs) != 5 { // 2 outline attempts + 3 detail rounds
		t.Fatalf("records=%d bad=%d err=%v", len(recs), bad, err)
	}
	if r := recs[0]; r.Status != usage.StatusOverloaded || r.Retry != 0 || recs[1].Retry != 1 || recs[1].Status != usage.StatusOK {
		t.Fatalf("attempts: %+v / %+v", recs[0], recs[1])
	}
	for _, r := range recs {
		if r.Operation != work.KindWorkflow || r.Telemetry != usage.Unavailable || r.InputTokens != nil || r.Model != nil ||
			r.PromptBytes == 0 || r.Project != usage.ProjectID(p.Dir) {
			t.Fatalf("record %+v", r)
		}
	}
}

// A usage log that cannot be written never fails the job.
func TestUsageStorageFailureKeepsJob(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	designReady(t, s)
	p := load(t, s)
	log := filepath.Join(p.Dir, core.StateDir, usage.File)
	os.Remove(log)
	if err := os.MkdirAll(log, 0o700); err != nil { // a folder where the file goes
		t.Fatal(err)
	}
	if err := s.GenerateWorkflow(); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	mustJob(t, load(t, s), work.KindWorkflow, core.JobDone)
}
