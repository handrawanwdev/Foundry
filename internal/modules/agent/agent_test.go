package agent

import (
	"slices"
	"testing"
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
