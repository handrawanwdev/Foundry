//go:build foundrybench

package pipeline_test

// Live benchmark of Blueprint generation, technology section (Step 9), with
// the real agent CLI. It never touches the source project: the project state
// is copied into BENCH_DST and the run happens there. It stops as soon as
// the technology section is accepted or the job fails, so the later rounds
// are not paid for.
//
//	BENCH_SRC=~/Project/X BENCH_DST=/tmp/bench/x BENCH_MODEL=claude-haiku-5-5 \
//	  go test -tags foundrybench -run TestBenchTechPlan -timeout 60m ./internal/modules/pipeline/
//
// The result is BENCH_DST/bench.json; the raw evidence is BENCH_DST/.foundry
// (project.json with its events, usage.jsonl with the CLI-reported usage).

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/agent"
	"github.com/handrawanwdev/foundry/internal/modules/pipeline"
	"github.com/handrawanwdev/foundry/internal/modules/techplan"
	"github.com/handrawanwdev/foundry/internal/modules/work"
)

type benchAI struct {
	model string
	none  bool
}

func (b benchAI) Runner() (*agent.Runner, bool, error) {
	if b.none {
		return nil, false, pipeline.ErrNoAI
	}
	r, err := agent.NewRunner("claude", nil, agent.Options{Model: b.model})
	return r, false, err
}

var refusedCount = regexp.MustCompile(`rencana teknologi ditolak, (\d+) masalah`)

func benchCopy(t *testing.T, src, dst string) *core.Store {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(src, core.StateDir, "project.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(dst); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dst, core.StateDir), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dst, core.StateDir, "project.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	st, err := core.NewStore(dst)
	if err != nil {
		t.Fatal(err)
	}
	reset := func(p *core.Project) error {
		p.Job, p.BlueprintQueue, p.BlueprintBatch, p.BlueprintProgress, p.BlueprintStarted = nil, nil, 0, nil, nil
		p.Events = nil
		if s := p.Stack; s != nil {
			s.Plan, s.PlanRequest, s.Proposal = nil, nil, nil
		}
		if s := p.Structure; s != nil {
			s.Plan, s.Repo, s.PlanRequest, s.RepoRequest, s.Proposal = nil, nil, nil, nil, nil
		}
		return nil
	}
	if _, err := st.Update(reset); err != nil {
		t.Fatal(err)
	}
	// Only the unified plan may run: anything before it would be paid for
	// and is not what is measured.
	dry := pipeline.New(st, benchAI{none: true})
	_ = dry.GenerateBlueprint()
	p, err := st.Load()
	if err != nil {
		t.Fatal(err)
	}
	if p.Job == nil || p.Job.Kind != work.KindPlan {
		t.Fatalf("first blueprint step is %+v, not the unified plan", p.Job)
	}
	if _, err := st.Update(reset); err != nil {
		t.Fatal(err)
	}
	return st
}

func TestBenchTechPlan(t *testing.T) {
	src, dst := os.Getenv("BENCH_SRC"), os.Getenv("BENCH_DST")
	if src == "" || dst == "" {
		t.Skip("BENCH_SRC and BENCH_DST not set")
	}
	st := benchCopy(t, src, dst)
	if os.Getenv("BENCH_DRY") != "" {
		return // the copy is ready and the plan is the first step; nothing paid
	}
	seed := benchSeed(t, st)
	s := pipeline.New(st, benchAI{model: os.Getenv("BENCH_MODEL")})
	start := time.Now()
	if err := s.GenerateBlueprint(); err != nil {
		t.Fatal(err)
	}
	outcome := "timeout"
	var landed time.Time
	for deadline := time.Now().Add(55 * time.Minute); time.Now().Before(deadline); time.Sleep(time.Second) {
		p, err := st.Load()
		if err != nil {
			continue
		}
		// Accepted: landed as the decision, or as a proposal when the stack
		// already had choices the plan may not replace silently.
		if p.Stack != nil && (p.Stack.Plan != nil || p.Stack.Proposal != nil) && p.Stack.PlanRequest == nil {
			outcome, landed = "accepted", time.Now()
			break
		}
		if p.Job != nil && p.Job.Status == core.JobFailed {
			outcome, landed = "failed", time.Now()
			break
		}
	}
	s.Shutdown(10 * time.Second)

	p, err := st.Load()
	if err != nil {
		t.Fatal(err)
	}
	var refusals []int
	for _, e := range p.Events {
		if m := refusedCount.FindStringSubmatch(e.Message); m != nil && e.Source == "system" {
			n, _ := strconv.Atoi(m[1])
			refusals = append(refusals, n)
		}
	}
	var runs []map[string]any
	if f, err := os.Open(filepath.Join(dst, core.StateDir, "usage.jsonl")); err == nil {
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		for sc.Scan() {
			var r map[string]any
			if json.Unmarshal(sc.Bytes(), &r) != nil || r["operation"] != work.KindPlan {
				continue
			}
			if at, err := time.Parse(time.RFC3339Nano, r["time"].(string)); err == nil && !landed.IsZero() && at.After(landed) {
				continue // a later round cut by the shutdown
			}
			runs = append(runs, r)
		}
		f.Close()
	}
	out := map[string]any{
		"project": p.Name, "outcome": outcome, "wall_ms": landed.Sub(start).Milliseconds(),
		"refusals": refusals, "runs": runs, "job_error": jobError(p),
	}
	if plan := benchPlan(p); plan != nil {
		out["items"] = len(plan.Items)
		out["plan_errors"] = len(plan.Errors())
	}
	var notes []string
	for _, e := range p.Events {
		if strings.Contains(e.Message, "Perbaikan terarah") || strings.Contains(e.Message, "Eskalasi") {
			notes = append(notes, e.Message)
		}
	}
	out["repair_notes"] = notes
	if seed != nil {
		out["seed_issues"] = len(seed.Issues)
		out["seed_codes"] = techplan.CodeCounts(seed.Issues)
		out["forced_model"] = seed.Model
		if plan := benchPlan(p); plan != nil {
			// Decisions the seed's issues did not name must come out unchanged.
			bad := map[string]bool{}
			for _, is := range seed.Issues {
				bad[is.Area] = true
			}
			kept, changed := 0, []string{}
			for _, it := range seed.Draft.Items {
				if bad[it.Area] {
					continue
				}
				if got := plan.Item(it.Area); got != nil && got.Technology == it.Technology && got.Reasoning == it.Reasoning {
					kept++
				} else {
					changed = append(changed, it.Area)
				}
			}
			out["valid_kept"], out["valid_changed"] = kept, changed
		}
		if r := p.Stack.PlanRequest; r != nil {
			out["attempts"] = r.Attempts
		}
	}
	b, _ := json.MarshalIndent(out, "", "  ")
	if err := os.WriteFile(filepath.Join(dst, "bench.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("%s: %s after %d runs, refusals %v", p.Name, outcome, len(runs), refusals)
}

// benchSeed starts the run from a refused answer (BENCH_SEED, a saved
// unified output), the way a retry continues a failed Step 9: the next run
// is a targeted repair. BENCH_FORCE_MODEL starts the repair on that model,
// as after an escalation.
func benchSeed(t *testing.T, st *core.Store) *core.TechPlanRequest {
	t.Helper()
	path := os.Getenv("BENCH_SEED")
	if path == "" {
		return nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var seed *core.TechPlanRequest
	_, err = st.Update(func(p *core.Project) error {
		r := &core.TechPlanRequest{Action: core.TechPlanGenerate}
		p.Stack.PlanRequest = r
		_, aerr := work.ApplyTechPlan(p, work.TechSection(string(raw)))
		var rej *techplan.RejectError
		if !errors.As(aerr, &rej) {
			return fmt.Errorf("seed is not a validator refusal: %v", aerr)
		}
		p.Stack.PlanRequest = r // ApplyTechPlan never clears it on refusal
		r.Draft, r.Issues, r.Rejected = rej.Plan, rej.Issues, rej.Error()
		r.Attempts = []core.TechAttempt{{N: 1, At: time.Now().UTC(), Strategy: core.TechAttemptGenerate, Errors: len(rej.Issues),
			Codes: techplan.CodeCounts(rej.Issues), Progress: core.TechProgressFirst}}
		if m := os.Getenv("BENCH_FORCE_MODEL"); m != "" {
			r.Model, r.EscalatedFrom, r.Escalation = m, os.Getenv("BENCH_MODEL"), "benchmark: repair dijalankan langsung pada model eskalasi"
		}
		p.Log("system", aerr.Error())
		copy := *r
		seed = &copy
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return seed
}

func jobError(p *core.Project) string {
	if p.Job != nil {
		return p.Job.Error
	}
	return ""
}

func benchPlan(p *core.Project) *core.TechPlan {
	if p.Stack == nil {
		return nil
	}
	if p.Stack.Plan != nil {
		return p.Stack.Plan
	}
	if p.Stack.Proposal != nil {
		return p.Stack.Proposal.Plan
	}
	return nil
}
