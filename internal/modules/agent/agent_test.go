package agent

import (
	"context"
	"os/exec"
	"slices"
	"testing"
	"time"
)

func TestPresetArgs(t *testing.T) {
	claude, _ := PresetByID("claude")
	got, err := claude.args(Options{Model: "opus", Effort: "high"})
	if err != nil || !slices.Equal(got, []string{"--model", "opus", "--effort", "high"}) {
		t.Fatalf("claude args = %v, %v", got, err)
	}
	codex, _ := PresetByID("codex")
	got, _ = codex.args(Options{Effort: "xhigh"})
	if !slices.Equal(got, []string{"-c", `model_reasoning_effort="xhigh"`}) {
		t.Fatalf("codex args = %v", got)
	}
	if got, _ := claude.args(Options{}); len(got) != 0 {
		t.Fatalf("empty options should add nothing, got %v", got)
	}
	qwen, _ := PresetByID("qwen")
	if _, err := qwen.args(Options{Effort: "high"}); err == nil {
		t.Fatal("qwen has no effort flag")
	}
	for _, m := range []string{"--dangerous", "a b", ""} {
		if _, err := claude.args(Options{Model: m}); (err == nil) != (m == "") {
			t.Fatalf("model %q: err = %v", m, err)
		}
	}
}

// An agent that leaves a child holding stdout must not hang the run.
func TestRunNotHeldByBackgroundChild(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh tidak ada")
	}
	r, _ := NewRunner("custom", []string{"sh", "-c", "cat >/dev/null; echo done; sleep 60 &"}, Options{})
	start := time.Now()
	out, err := r.Run(context.Background(), Execute, t.TempDir(), "hi")
	if d := time.Since(start); d > 20*time.Second {
		t.Fatalf("Run tertahan %s", d)
	}
	if err != nil || out != "done\n" {
		t.Fatalf("out=%q err=%v", out, err)
	}
}
