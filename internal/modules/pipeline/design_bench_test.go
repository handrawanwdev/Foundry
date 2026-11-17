//go:build foundrybench

package pipeline_test

// Live design evaluation on a copy of a project (BENCH_DIR, a directory that
// already holds .foundry/project.json; never the source project). Every run
// goes through the normal pipeline with a real agent; nothing is injected.
//
//	BENCH_DIR=... BENCH_LABEL=before go test -tags foundrybench -run TestBenchDesignReport ./internal/modules/pipeline/
//	BENCH_DIR=... BENCH_MODEL=claude-haiku-5-5 BENCH_TEXT="..." BENCH_FOCUS=use-cases \
//	  go test -tags foundrybench -run TestBenchScopeRevise -timeout 60m ./internal/modules/pipeline/
//	BENCH_DIR=... BENCH_MODEL=claude-haiku-5-5 [BENCH_FRESH=1] \
//	  go test -tags foundrybench -run TestBenchDesignGenerate -timeout 90m ./internal/modules/pipeline/
//
// Each test writes bench-<name>.json next to .foundry with the measured
// result; the design report keeps every dimension apart.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/pipeline"
	"github.com/handrawanwdev/foundry/internal/modules/work"
)

func benchEnvList(key string) []string {
	var out []string
	for _, s := range strings.Split(os.Getenv(key), ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func benchWrite(t *testing.T, dir, name string, v any) {
	t.Helper()
	b, _ := json.MarshalIndent(v, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "bench-"+name+".json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// designReport is the measured state of a project's design and use cases.
func designReport(p *core.Project) map[string]any {
	out := map[string]any{}
	pages := 0
	if p.UI != nil && p.UI.Layout != nil {
		pages = len(p.UI.Layout.Pages)
	}
	trans, parallel, triggers := 0, 0, 0
	for _, lc := range p.Lifecycles {
		trans += len(lc.Transitions)
	}
	for _, f := range p.Flows {
		parallel += len(f.Parallel)
		if f.Trigger != "" {
			triggers++
		}
	}
	out["counts"] = map[string]int{"requirements": len(p.Requirements), "flows": len(p.Flows), "pages": pages,
		"lifecycles": len(p.Lifecycles), "transitions": trans, "parallel_groups": parallel, "triggers": triggers}
	var errs, warns []string
	for _, is := range p.DesignIssues(nil) {
		line := is.Where + ": " + is.Message
		switch is.Level {
		case core.IssueError:
			errs = append(errs, line)
		case core.IssueWarning:
			warns = append(warns, line)
		}
	}
	out["design_errors"], out["design_warnings"] = errs, warns
	var lcIssues []string
	for _, is := range p.LifecycleIssues(p.Lifecycles, p.Flows) {
		if is.Level != core.IssueInfo {
			lcIssues = append(lcIssues, is.Level+" "+is.Where+": "+is.Message)
		}
	}
	out["lifecycle_issues"] = lcIssues
	var ucIssues []string
	for _, is := range p.UseCaseIssues(p.Flows) {
		if is.Level != core.IssueInfo {
			ucIssues = append(ucIssues, is.Level+" "+is.Where+": "+is.Message)
		}
	}
	out["usecase_issues"] = ucIssues
	g := p.DesignGate()
	g.Requirements = nil // long; the per-requirement status is summarised below
	req := map[string]int{}
	for _, r := range p.DesignGate().Requirements {
		req[r.Status]++
	}
	out["gate"], out["requirement_status"] = g, req
	out["benchmark"] = p.DesignBenchmark()
	return out
}

func benchUsage(dir string, since time.Time) []json.RawMessage {
	raw, err := os.ReadFile(filepath.Join(dir, core.StateDir, "usage.jsonl"))
	if err != nil {
		return nil
	}
	var out []json.RawMessage
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		var r struct {
			At time.Time `json:"at"`
		}
		if json.Unmarshal([]byte(line), &r) == nil && (r.At.IsZero() || !r.At.Before(since)) {
			out = append(out, json.RawMessage(line))
		}
	}
	return out
}

// waitJob waits until a job of kind ends (done with nothing left, or
// failed). left reports work the pipeline still has queued for the kind.
func waitJob(st *core.Store, kind string, limit time.Duration, left func(*core.Project) bool) string {
	for deadline := time.Now().Add(limit); time.Now().Before(deadline); time.Sleep(2 * time.Second) {
		p, err := st.Load()
		if err != nil || p.Job == nil {
			continue
		}
		// A job of kind may chain another (a use case change syncs the
		// workflow); the run ends when nothing is running.
		if p.Job.Kind != kind && p.Job.Status == core.JobRunning {
			continue
		}
		switch p.Job.Status {
		case core.JobFailed:
			return "failed"
		case core.JobDone:
			if left == nil || !left(p) {
				return "done"
			}
		}
	}
	return "timeout"
}

func TestBenchDesignReport(t *testing.T) {
	dir := os.Getenv("BENCH_DIR")
	if dir == "" {
		t.Skip("BENCH_DIR not set")
	}
	st, err := core.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	p, err := st.Load()
	if err != nil {
		t.Fatal(err)
	}
	label := os.Getenv("BENCH_LABEL")
	if label == "" {
		label = "report"
	}
	benchWrite(t, dir, label, designReport(p))
}

func TestBenchScopeRevise(t *testing.T) {
	dir, text := os.Getenv("BENCH_DIR"), os.Getenv("BENCH_TEXT")
	if dir == "" || text == "" {
		t.Skip("BENCH_DIR or BENCH_TEXT not set")
	}
	st, err := core.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	s := pipeline.New(st, benchAI{model: os.Getenv("BENCH_MODEL")})
	start := time.Now().UTC()
	if err := s.ReviseArtifacts(text, benchEnvList("BENCH_FOCUS")); err != nil {
		t.Fatal(err)
	}
	outcome := waitJob(st, work.KindArtifacts, 55*time.Minute, func(p *core.Project) bool { return p.ArtifactRequest != nil })
	wall := time.Since(start)
	s.Shutdown(10 * time.Second)
	p, err := st.Load()
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]any{"outcome": outcome, "wall_ms": wall.Milliseconds(), "job_error": jobError(p),
		"report": designReport(p), "usage": benchUsage(dir, start)}
	benchWrite(t, dir, "scope", out)
	t.Logf("%s after %s", outcome, wall.Round(time.Second))
}

func TestBenchDesignGenerate(t *testing.T) {
	dir := os.Getenv("BENCH_DIR")
	if dir == "" {
		t.Skip("BENCH_DIR not set")
	}
	st, err := core.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv("BENCH_FRESH") != "" {
		// A copy without a design: the state of a project before its first
		// design job. Use cases, lifecycles and everything else stay.
		if _, err := st.Update(func(p *core.Project) error {
			p.UI, p.Job = nil, nil
			for i := range p.Flows {
				p.Flows[i].PageIDs, p.Flows[i].StepPages, p.Flows[i].BranchPages = nil, nil, nil
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	s := pipeline.New(st, benchAI{model: os.Getenv("BENCH_MODEL")})
	start := time.Now().UTC()
	var startErr error
	if text := os.Getenv("BENCH_TEXT"); text != "" {
		startErr = s.ReviseDesign(text, benchEnvList("BENCH_FOCUS"))
	} else {
		startErr = s.GenerateDesign()
	}
	if startErr != nil {
		t.Fatal(startErr)
	}
	outcome := waitJob(st, work.KindDesign, 85*time.Minute, func(p *core.Project) bool {
		return work.DesignNeedsDetails(p) || p.UI != nil && p.UI.Request != nil
	})
	wall := time.Since(start)
	s.Shutdown(10 * time.Second)
	p, err := st.Load()
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]any{"outcome": outcome, "wall_ms": wall.Milliseconds(), "job_error": jobError(p),
		"report": designReport(p), "usage": benchUsage(dir, start)}
	name := "design"
	if l := os.Getenv("BENCH_LABEL"); l != "" {
		name = l
	}
	benchWrite(t, dir, name, out)
	t.Logf("%s after %s", outcome, wall.Round(time.Second))
}
