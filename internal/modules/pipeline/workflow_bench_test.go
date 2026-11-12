//go:build foundrybench

package pipeline_test

// Live retry of a failed workflow job on a copy of a project (BENCH_DIR, a
// directory that already holds .foundry/project.json; never the source
// project). It presses "Coba lagi" the way the UI does and waits until the
// outline lands (BENCH_FULL: until every command is filled) or the job fails.
//
//	BENCH_DIR=/tmp/bench/parking BENCH_MODEL=claude-haiku-5-5 \
//	  go test -tags foundrybench -run TestBenchWorkflowRetry -timeout 90m ./internal/modules/pipeline/

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/pipeline"
	"github.com/handrawanwdev/foundry/internal/modules/spec"
	"github.com/handrawanwdev/foundry/internal/modules/work"
	"github.com/handrawanwdev/foundry/internal/modules/workflow"
)

func TestBenchWorkflowRetry(t *testing.T) {
	dir := os.Getenv("BENCH_DIR")
	if dir == "" {
		t.Skip("BENCH_DIR not set")
	}
	st, err := core.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	full := os.Getenv("BENCH_FULL") != ""
	s := pipeline.New(st, benchAI{model: os.Getenv("BENCH_MODEL")})
	start := time.Now()
	if err := s.RetryJob(); err != nil {
		t.Fatal(err)
	}
	outcome := "timeout"
	for deadline := time.Now().Add(85 * time.Minute); time.Now().Before(deadline); time.Sleep(2 * time.Second) {
		p, err := st.Load()
		if err != nil {
			continue
		}
		if p.Job != nil && p.Job.Kind == work.KindWorkflow && p.Job.Status == core.JobFailed {
			outcome = "failed"
			break
		}
		if w := p.Workflow; w != nil && len(w.Commands) > 0 && (!full || (w.Unfilled() == 0 && (p.Job == nil || p.Job.Status != core.JobRunning))) {
			outcome = "accepted"
			break
		}
	}
	wall := time.Since(start)
	s.Shutdown(10 * time.Second)
	p, err := st.Load()
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]any{"outcome": outcome, "wall_ms": wall.Milliseconds(), "job_error": jobError(p)}
	if w := p.Workflow; w != nil && len(w.Commands) > 0 {
		unknown, uncovered := workflow.Covers(w.Commands, p.Requirements)
		out["commands"] = len(w.Commands)
		out["unfilled"] = w.Unfilled()
		out["uncovered_must"] = uncovered
		out["unknown_refs"] = len(unknown)
		if w.Unfilled() == 0 {
			exists := map[string]bool{}
			for _, d := range spec.Artifacts(p) {
				exists[d.ID] = true
			}
			var msgs []string
			for _, is := range workflow.Validate(w.Commands, exists) {
				msgs = append(msgs, is.Command+" "+is.Message)
			}
			for _, is := range workflow.ValidateFeatures(w.Commands, p.Features) {
				msgs = append(msgs, is.Command+" "+is.Message)
			}
			out["validate_issues"] = msgs
		}
	}
	b, _ := json.MarshalIndent(out, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "bench-workflow.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("%s after %s", outcome, wall.Round(time.Second))
}
