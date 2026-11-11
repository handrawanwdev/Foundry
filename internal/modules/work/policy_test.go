package work

import (
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
)

func TestSplitPrompt(t *testing.T) {
	static, body := SplitPrompt(WithPolicy("tugas"))
	if static != ExecutionPolicy || body != "tugas" {
		t.Fatalf("policy only: %q / %q", static, body)
	}
	static, body = SplitPrompt(WithPolicy("aturan" + CacheBreak + "tugas" + CacheBreak + "lagi"))
	if static != ExecutionPolicy+"aturan" || body != "tugaslagi" {
		t.Fatalf("own break: %q / %q", static, body)
	}
	if s, b := SplitPrompt("tanpa"); s != "" || b != "tanpa" {
		t.Fatalf("no break: %q / %q", s, b)
	}
	if WithPolicy("") != "" {
		t.Fatal("empty prompt must stay empty")
	}
	if j := JoinPrompt(WithPolicy("a" + CacheBreak + "b")); j != ExecutionPolicy+"ab" || strings.Contains(j, "\x1e") {
		t.Fatalf("join = %q", j)
	}
}

// The part before the break is what stays the same from round to round: the
// briefs and the blueprint, never the commands or the request.
func TestWorkflowPromptCacheBreak(t *testing.T) {
	p := &core.Project{Name: "Klinik", Story: "cerita", Workflow: &core.Workflow{Commands: []core.Command{
		{ID: "C1", Name: "a", Instructions: "teks-c1", Artifacts: []core.ArtifactRef{{ID: "api"}}},
		{ID: "C2", Name: "b", Instructions: "teks-c2", DependsOn: []string{"C1"}},
		{ID: "C3", Name: "c", Instructions: "teks-c3"},
	}, Request: &core.ChangeRequest{Text: "permintaan-user", Focus: []string{"C1"}}}}
	docs := []core.Artifact{{ID: "api", Body: "isi-api"}, {ID: "requirements", Body: "isi-req"}}
	check := func(name, prompt string) {
		t.Helper()
		if strings.Count(prompt, CacheBreak) != 1 {
			t.Fatalf("%s: %d breaks", name, strings.Count(prompt, CacheBreak))
		}
		static, body := SplitPrompt(prompt)
		if !strings.Contains(static, "cerita") || strings.Contains(static, "teks-c") || strings.Contains(static, "permintaan-user") {
			t.Fatalf("%s static:\n%s", name, static)
		}
		if !strings.Contains(body, "teks-c1") || !strings.Contains(body, "Reply with ONLY one JSON object") {
			t.Fatalf("%s body:\n%s", name, body)
		}
	}
	focused := WorkflowPrompt(p, docs)
	check("focused", focused)
	if s, _ := SplitPrompt(focused); !strings.Contains(s, "isi-api") {
		t.Fatal("focused: blueprint must be in the static part")
	}
	p.Workflow.Request.Focus = nil
	check("unfocused", WorkflowPrompt(p, docs))

	// Rounds of the same request share the static part byte for byte.
	p.Workflow.Request.Focus = []string{"C1"}
	a, _ := SplitPrompt(WorkflowPrompt(p, docs))
	p.Workflow.Commands[0].Instructions = "teks-c1-baru"
	p.Workflow.Request.Handled = []string{"changed C1"}
	b, _ := SplitPrompt(WorkflowPrompt(p, docs))
	if a != b {
		t.Fatal("static part changed between rounds")
	}
}
