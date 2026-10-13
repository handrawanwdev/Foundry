package agent

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
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

func TestOpenCodeArgv(t *testing.T) {
	oc, ok := PresetByID("opencode")
	if !ok {
		t.Fatal("opencode preset missing")
	}
	opts, err := oc.args(Options{Model: "anthropic/claude-sonnet-4"})
	if err != nil {
		t.Fatal(err)
	}
	head := append(append([]string{oc.Binary}, oc.sub...), opts...)
	got := append(slices.Clone(head), oc.think...)
	want := []string{"opencode", "run", "--model", "anthropic/claude-sonnet-4", "--agent", "plan"}
	if !slices.Equal(got, want) {
		t.Fatalf("think argv = %v", got)
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

func TestRunDetectsLimit(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh tidak ada")
	}
	r, _ := NewRunner("custom", []string{"sh", "-c", "cat >/dev/null; echo 'Claude usage limit reached' >&2; sleep 30"}, Options{})
	start := time.Now()
	_, err := r.Run(context.Background(), Execute, t.TempDir(), "hi")
	if !errors.Is(err, ErrLimit) || time.Since(start) > 10*time.Second {
		t.Fatalf("err = %v after %s", err, time.Since(start))
	}
	r, _ = NewRunner("custom", []string{"sh", "-c", "cat >/dev/null; echo 'usage limit reached|123'"}, Options{})
	if _, err := r.Run(context.Background(), Execute, t.TempDir(), "hi"); !errors.Is(err, ErrLimit) {
		t.Fatalf("stdout notice: err = %v", err)
	}
}

func TestRunKillsIdleAgent(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh tidak ada")
	}
	r, _ := NewRunner("custom", []string{"sh", "-c", "cat >/dev/null; sleep 30"}, Options{})
	r.Idle = 2 * time.Second
	start := time.Now()
	_, err := r.Run(context.Background(), Execute, t.TempDir(), "hi")
	if !errors.Is(err, ErrAgentFailed) || errors.Is(err, ErrLimit) || time.Since(start) > 15*time.Second {
		t.Fatalf("err = %v after %s", err, time.Since(start))
	}
}

func TestPreflightFailsFastOnLimit(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh tidak ada")
	}
	r, _ := NewRunner("custom", []string{"sh", "-c", "cat >/dev/null; echo 'usage limit reached'"}, Options{})
	if err := r.Preflight(context.Background(), t.TempDir()); !errors.Is(err, ErrLimit) {
		t.Fatalf("err = %v", err)
	}
	ok, _ := NewRunner("custom", []string{"sh", "-c", "cat >/dev/null; echo OK"}, Options{})
	if err := ok.Preflight(context.Background(), t.TempDir()); err != nil {
		t.Fatal(err)
	}
}

func TestLimitHitFunds(t *testing.T) {
	if !limitHit("Error: Upstream request failed: Insufficient account funds") {
		t.Fatal("funds notice not detected")
	}
}

// First listed model is dead; only "good" replies. SuggestModel must find it.
func TestSuggestModelFindsWorking(t *testing.T) {
	dir := t.TempDir()
	script := `#!/bin/sh
if [ "$1" = models ]; then printf 'p/dead\np/good-free\n'; exit 0; fi
cat >/dev/null
case "$*" in *good*) echo OK;; *) echo 'Error: Insufficient account funds';; esac
`
	os.WriteFile(filepath.Join(dir, "opencode"), []byte(script), 0o755)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	modelCache.Lock()
	delete(modelCache.m, "opencode")
	modelCache.Unlock()
	r, err := NewRunner("opencode", nil, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got := r.SuggestModel(context.Background(), t.TempDir()); got != "p/good-free" {
		t.Fatalf("suggested %q", got)
	}
}
