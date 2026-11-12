//go:build foundrybench

package work

// Offline prompt size benchmark of the workflow detail phase: it replays
// every detail round of a finished workflow and measures the prompt each
// round sends, without calling an agent.
//
//	BENCH_PROJECT=/path/.foundry/project.json \
//	  go test -tags foundrybench -run TestBenchDetailPromptSize -v ./internal/modules/work/

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/spec"
)

func TestBenchDetailPromptSize(t *testing.T) {
	path := os.Getenv("BENCH_PROJECT")
	if path == "" {
		t.Skip("BENCH_PROJECT kosong")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var p core.Project
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatal(err)
	}
	if p.Workflow == nil || len(p.Workflow.Commands) == 0 {
		t.Fatal("workflow kosong")
	}
	final := map[string]core.Command{}
	for _, c := range p.Workflow.Commands {
		final[c.ID] = c
	}
	for i := range p.Workflow.Commands {
		c := &p.Workflow.Commands[i]
		c.Context, c.Instructions, c.Inputs, c.Outputs, c.Acceptance, c.Constraints, c.Files = "", "", nil, nil, nil, nil, nil
	}
	docs := spec.Artifacts(&p)
	var total, totalStatic, maxBytes, n int
	for {
		next := nextFillCommand(p.Workflow.Commands)
		if next == nil {
			break
		}
		static, body := SplitPrompt(WithPolicy(WorkflowPrompt(&p, docs)))
		size := len(static) + len(body)
		if out := os.Getenv("BENCH_OUT"); out != "" {
			os.WriteFile(filepath.Join(out, next.ID+".txt"), []byte(static+CacheBreak+body), 0o600)
		}
		t.Logf("%s prompt=%d static=%d dynamic=%d est_tokens=%d", next.ID, size, len(static), len(body), estTokens(static+body))
		// What the round must carry whatever the scoping: every criterion
		// of the requirements it covers, what its dependencies hand over,
		// its own links.
		var must []string
		for _, r := range p.Requirements {
			if slices.Contains(next.Covers, r.ID) {
				for _, c := range r.Acceptance {
					must = append(must, c.ID+" ("+c.Kind+"): "+oneLine(c.Text))
				}
			}
		}
		for _, c := range p.Workflow.Commands {
			if slices.Contains(next.DependsOn, c.ID) {
				for _, o := range c.Outputs {
					must = append(must, "output: "+oneLine(o))
				}
			}
		}
		must = append(must, next.FeatureIDs...)
		must = append(must, next.SubFeatureIDs...)
		for _, m := range must {
			if !strings.Contains(body, m) {
				t.Errorf("%s: prompt misses %q", next.ID, m)
			}
		}
		total += size
		totalStatic += len(static)
		maxBytes = max(maxBytes, size)
		n++
		*next = final[next.ID]
		if strings.TrimSpace(next.Instructions) == "" {
			next.Instructions = "-"
		}
	}
	t.Logf("rounds=%d avg_prompt=%d avg_static=%d avg_dynamic=%d max=%d total=%d", n, total/n, totalStatic/n, (total-totalStatic)/n, maxBytes, total)
}

// estTokens is a rough token count (4 bytes per token) for comparing runs;
// the CLI-reported counts are in usage.jsonl.
func estTokens(s string) int { return (len(s) + 3) / 4 }

// TestBenchRankSections prints how the sections of one artifact rank for
// one command (BENCH_CMD, BENCH_ART).
func TestBenchRankSections(t *testing.T) {
	path, id, art := os.Getenv("BENCH_PROJECT"), os.Getenv("BENCH_CMD"), os.Getenv("BENCH_ART")
	if path == "" || id == "" || art == "" {
		t.Skip("BENCH_PROJECT/BENCH_CMD/BENCH_ART kosong")
	}
	raw, _ := os.ReadFile(path)
	var p core.Project
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatal(err)
	}
	var cmd core.Command
	for _, c := range p.Workflow.Commands {
		if c.ID == id {
			cmd = c
		}
	}
	q := commandQuery(&p, []core.Command{cmd})
	for _, d := range spec.Artifacts(&p) {
		if d.ID != art {
			continue
		}
		secs := splitSections(d.Body)
		scores := rank(secs, q)
		for i, s := range secs {
			t.Logf("%6.2f %6d %s", scores[i], len(s.text), s.heading)
		}
	}
}
