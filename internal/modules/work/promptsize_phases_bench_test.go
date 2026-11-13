//go:build foundrybench

package work

// Offline prompt size benchmark of the workflow sketch and revision phases
// (BENCH_PROJECT: a project.json with a complete workflow). Each scenario
// prints the bytes of the static part (system prompt, cached by the
// provider) and of the dynamic part (sent fresh every round), first try and
// retry, and checks what the round must carry.
//
//	BENCH_PROJECT=/path/.foundry/project.json \
//	  go test -tags foundrybench -run 'TestBench(Sketch|Revision)PromptSize' -v ./internal/modules/work/

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/spec"
)

func benchProject(t *testing.T) *core.Project {
	t.Helper()
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
	return &p
}

// measure logs one prompt and writes it to BENCH_OUT when set.
func measure(t *testing.T, name string, p *core.Project, docs []core.Artifact) (static, body string) {
	t.Helper()
	static, body = SplitPrompt(WithPolicy(WorkflowPrompt(p, docs)))
	if out := os.Getenv("BENCH_OUT"); out != "" {
		os.WriteFile(filepath.Join(out, name+".txt"), []byte(static+"\n<<<CACHE BREAK>>>\n"+body), 0o600)
	}
	t.Logf("%-22s prompt=%7d static=%7d dynamic=%7d est_tokens=%6d", name, len(static)+len(body), len(static), len(body), estTokens(static+body))
	return static, body
}

func mustCarry(t *testing.T, name, text string, want []string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(text, w) {
			t.Errorf("%s misses %q", name, w)
		}
	}
}

func TestBenchSketchPromptSize(t *testing.T) {
	p := benchProject(t)
	docs := spec.Artifacts(p)
	var must []string
	for _, r := range p.Requirements {
		if r.Priority == "MUST" {
			must = append(must, r.ID)
		}
	}
	for _, f := range p.Features {
		must = append(must, f.ID)
	}
	p.Workflow = &core.Workflow{ID: p.Workflow.ID}
	s1, b1 := measure(t, "sketch", p, docs)
	mustCarry(t, "sketch", s1+b1, must)
	p.Workflow.Request = &core.ChangeRequest{Rejected: "workflow tidak valid: " + uncoveredMust + "REQ-025, REQ-031"}
	s2, b2 := measure(t, "sketch-retry", p, docs)
	mustCarry(t, "sketch-retry", s2+b2, append(must, "YOUR LAST ANSWER WAS REFUSED", "REQ-025", "REQ-031"))
	if s1 != s2 {
		t.Log("static part differs between try and retry: retry is not served from cache")
	}
}

func TestBenchRevisionPromptSize(t *testing.T) {
	p := benchProject(t)
	docs := spec.Artifacts(p)
	cmds := p.Workflow.Commands
	focus := cmds[len(cmds)/3].ID
	scenarios := []struct {
		name string
		req  core.ChangeRequest
	}{
		{"revision-focus", core.ChangeRequest{Text: "Perjelas langkah uji pada command ini.", Focus: []string{focus}}},
		{"revision-focus-round2", core.ChangeRequest{Text: "Perjelas langkah uji pada command ini.", Focus: []string{focus}, Handled: []string{"changed " + focus}}},
		{"revision-focus-retry", core.ChangeRequest{Text: "Perjelas langkah uji pada command ini.", Focus: []string{focus}, Rejected: "revisi harus satu command per jawaban"}},
		{"revision-all", core.ChangeRequest{Text: "Tambahkan langkah pemeriksaan keamanan di setiap command yang menyentuh API."}},
		{"revision-all-retry", core.ChangeRequest{Text: "Tambahkan langkah pemeriksaan keamanan di setiap command yang menyentuh API.", Rejected: "revisi harus satu command per jawaban"}},
		{"revision-sync", core.ChangeRequest{Text: "Ikuti artefak blueprint yang berubah.", Focus: []string{focus}, Sync: true}},
	}
	statics := map[string]string{}
	for _, sc := range scenarios {
		r := sc.req
		p.Workflow.Request = &r
		s, b := measure(t, sc.name, p, docs)
		statics[sc.name] = s
		// The commands it may change are carried whole; the request, the
		// refusal and the rules always.
		want := []string{"USER REQUEST", r.Text, "Revision rules", "Command rules"}
		if r.Rejected != "" {
			want = append(want, "YOUR LAST ANSWER WAS REFUSED, fix this: "+r.Rejected)
		}
		scope := revisionScope(p.Workflow)
		for _, c := range cmds {
			if scope == nil || scope[c.ID] {
				want = append(want, mustJSON(c.Instructions))
			} else {
				want = append(want, c.ID)
			}
		}
		mustCarry(t, sc.name, s+b, want)
	}
	for _, pair := range [][2]string{{"revision-focus", "revision-focus-round2"}, {"revision-focus", "revision-focus-retry"}, {"revision-all", "revision-all-retry"}} {
		if statics[pair[0]] != statics[pair[1]] {
			t.Logf("static part differs: %s vs %s", pair[0], pair[1])
		}
	}
}
