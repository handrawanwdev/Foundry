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
	"slices"
	"strings"
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

// Live focused revision of a complete workflow on a copy of a project
// (BENCH_DIR; never the source project): BENCH_REVISE_TEXT for the
// commands in BENCH_REVISE_FOCUS (comma separated). It waits until the
// request is closed or the job fails, and checks the result like the
// workflow check does.
//
//	BENCH_DIR=/tmp/bench/parking BENCH_MODEL=claude-haiku-5-5 BENCH_REVISE_TEXT="..." BENCH_REVISE_FOCUS=CMD-009 \
//	  go test -tags foundrybench -run TestBenchWorkflowRevise -timeout 60m ./internal/modules/pipeline/
func TestBenchWorkflowRevise(t *testing.T) {
	dir, text := os.Getenv("BENCH_DIR"), os.Getenv("BENCH_REVISE_TEXT")
	if dir == "" || text == "" {
		t.Skip("BENCH_DIR or BENCH_REVISE_TEXT not set")
	}
	var focus []string
	for _, id := range strings.Split(os.Getenv("BENCH_REVISE_FOCUS"), ",") {
		if id = strings.TrimSpace(id); id != "" {
			focus = append(focus, id)
		}
	}
	st, err := core.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	before, err := st.Load()
	if err != nil {
		t.Fatal(err)
	}
	s := pipeline.New(st, benchAI{model: os.Getenv("BENCH_MODEL")})
	start := time.Now()
	if err := s.ReviseWorkflow(text, focus); err != nil {
		t.Fatal(err)
	}
	outcome := "timeout"
	for deadline := time.Now().Add(55 * time.Minute); time.Now().Before(deadline); time.Sleep(2 * time.Second) {
		p, err := st.Load()
		if err != nil || p.Job == nil || p.Job.Kind != work.KindWorkflow {
			continue
		}
		if p.Job.Status == core.JobFailed {
			outcome = "failed"
			break
		}
		if p.Job.Status != core.JobRunning && p.Workflow != nil && p.Workflow.Request == nil {
			outcome = "done"
			break
		}
	}
	wall := time.Since(start)
	s.Shutdown(10 * time.Second)
	p, err := st.Load()
	if err != nil {
		t.Fatal(err)
	}
	w := p.Workflow
	changed := map[string]string{}
	old := map[string]core.Command{}
	for _, c := range before.Workflow.Commands {
		old[c.ID] = c
	}
	for _, c := range w.Commands {
		o, ok := old[c.ID]
		switch {
		case !ok:
			changed[c.ID] = "added"
		case o.Instructions != c.Instructions || !slices.Equal(o.Acceptance, c.Acceptance) || !slices.Equal(o.Covers, c.Covers):
			changed[c.ID] = "changed"
		}
	}
	for id := range old {
		if w.Command(id) == nil {
			changed[id] = "removed"
		}
	}
	exists := map[string]bool{}
	for _, d := range spec.Artifacts(p) {
		exists[d.ID] = true
	}
	var msgs []string
	for _, is := range append(workflow.Validate(w.Commands, exists), workflow.ValidateFeatures(w.Commands, p.Features)...) {
		msgs = append(msgs, is.Command+" "+is.Message)
	}
	for _, is := range work.CheckUI(p, w.Commands) {
		msgs = append(msgs, is.Command+" "+is.Message)
	}
	unknown, uncovered := workflow.Covers(w.Commands, p.Requirements)
	out := map[string]any{"outcome": outcome, "wall_ms": wall.Milliseconds(), "job_error": jobError(p), "changed": changed,
		"commands": len(w.Commands), "uncovered_must": uncovered, "unknown_refs": len(unknown), "validate_issues": msgs}
	b, _ := json.MarshalIndent(out, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "bench-revise.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("%s after %s: %s", outcome, wall.Round(time.Second), b)
}
