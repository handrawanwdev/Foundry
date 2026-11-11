package agent

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"sync/atomic"
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
	out, err := r.Run(context.Background(), t.TempDir(), "hi")
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
	_, err := r.Run(context.Background(), t.TempDir(), "hi")
	if !errors.Is(err, ErrLimit) || time.Since(start) > 10*time.Second {
		t.Fatalf("err = %v after %s", err, time.Since(start))
	}
	r, _ = NewRunner("custom", []string{"sh", "-c", "cat >/dev/null; echo 'usage limit reached|123'"}, Options{})
	if _, err := r.Run(context.Background(), t.TempDir(), "hi"); !errors.Is(err, ErrLimit) {
		t.Fatalf("stdout notice: err = %v", err)
	}
}

func TestRunDetectsOverload(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh tidak ada")
	}
	r, _ := NewRunner("custom", []string{"sh", "-c", "cat >/dev/null; echo 'Error: Upstream request failed: [overloaded] the server is overloaded; retry shortly' >&2; exit 1"}, Options{})
	if _, err := r.Run(context.Background(), t.TempDir(), "hi"); !errors.Is(err, ErrOverloaded) || !errors.Is(err, ErrAgentFailed) {
		t.Fatalf("err = %v", err)
	}
	// A run that passed is an answer, whatever its log says.
	r, _ = NewRunner("custom", []string{"sh", "-c", "cat >/dev/null; echo 'overloaded, retrying' >&2; echo jawaban"}, Options{})
	if out, err := r.Run(context.Background(), t.TempDir(), "hi"); err != nil || out != "jawaban\n" {
		t.Fatalf("out=%q err=%v", out, err)
	}
	// The word inside a long answer is not the server's refusal.
	r, _ = NewRunner("custom", []string{"sh", "-c", "cat >/dev/null; printf '%0500d overloaded\n' 0; exit 1"}, Options{})
	if _, err := r.Run(context.Background(), t.TempDir(), "hi"); errors.Is(err, ErrOverloaded) || !errors.Is(err, ErrAgentFailed) {
		t.Fatalf("long stdout: err = %v", err)
	}
}

func TestLimitHitFunds(t *testing.T) {
	if !limitHit("Error: Upstream request failed: Insufficient account funds") {
		t.Fatal("funds notice not detected")
	}
}

// First listed model is dead; only "good" replies. SuggestModel must find it.
func TestSuggestModelFindsWorking(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script")
	}
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

// A default model with no balance must not fail the connection: Connect falls
// back to a listed model that answers and reports the model to store.
func TestConnectFallsBackToFreeModel(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script")
	}
	dir := t.TempDir()
	script := `#!/bin/sh
if [ "$1" = models ]; then printf 'p/paid\np/good-free\n'; exit 0; fi
cat >/dev/null
case "$*" in *good*) echo OK;; *) echo 'Error: Insufficient account funds' >&2; exit 1;; esac
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
	got, err := r.Connect(context.Background(), t.TempDir())
	if err != nil || got != "p/good-free" {
		t.Fatalf("Connect = %q, %v", got, err)
	}
}

// The claude preset runs lean: no built-in tools, MCP servers, skills or hooks.
func TestClaudePresetIsLean(t *testing.T) {
	c, _ := PresetByID("claude")
	for _, f := range []string{"--tools=", "--strict-mcp-config", "--disable-slash-commands"} {
		if !slices.Contains(c.lean, f) {
			t.Fatalf("lean %v misses %s", c.lean, f)
		}
	}
}

// A CLI that rejects a lean flag still answers: the runner drops lean once
// and keeps it dropped.
func TestRunDropsLeanOnUnknownFlag(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh tidak ada")
	}
	script := `cat >/dev/null; for a in "$@"; do [ "$a" = --lean ] && { echo "error: unknown option '--lean'" >&2; exit 1; }; done; echo ok`
	r := &Runner{argv: []string{"sh", "-c", script, "sh"}, lean: []string{"--lean"}, noLean: new(atomic.Bool), Timeout: time.Minute}
	out, err := r.Run(context.Background(), t.TempDir(), "hi")
	if err != nil || out != "ok\n" || !r.noLean.Load() {
		t.Fatalf("out=%q err=%v noLean=%v", out, err, r.noLean.Load())
	}
	r = &Runner{argv: []string{"sh", "-c", `cat >/dev/null; echo "$@"`, "sh"}, lean: []string{"--lean"}, noLean: new(atomic.Bool), Timeout: time.Minute}
	if out, _ := r.Run(context.Background(), t.TempDir(), "hi"); out != "--lean\n" {
		t.Fatalf("lean flag not passed: %q", out)
	}
}

// Only the CLI refusing a lean flag drops lean mode; an auth, network or API
// failure, or an unknown option that is not a lean flag, runs once.
func TestRunKeepsLeanOnOtherFailures(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh tidak ada")
	}
	for _, stderr := range []string{
		"Invalid API key · Please run /login",
		"API Error: getaddrinfo ENOTFOUND api.anthropic.com",
		"error: unknown option '--bogus'",
		"API Error: 500 Internal server error",
	} {
		count := filepath.Join(t.TempDir(), "count")
		script := `cat >/dev/null; echo x >> "$COUNT"; echo "$MSG" >&2; exit 1`
		r := &Runner{argv: []string{"sh", "-c", script, "sh"}, lean: []string{"--tools=", "--strict-mcp-config"}, noLean: new(atomic.Bool), Timeout: time.Minute}
		t.Setenv("COUNT", count)
		t.Setenv("MSG", stderr)
		if _, err := r.Run(context.Background(), t.TempDir(), "hi"); err == nil {
			t.Fatalf("%q: want an error", stderr)
		}
		runs, _ := os.ReadFile(count)
		if r.noLean.Load() || len(runs) != 2 {
			t.Fatalf("%q: noLean=%v runs=%q, want lean kept and one run", stderr, r.noLean.Load(), runs)
		}
	}
	if !rejectsLean("error: unknown option '--tools='", []string{"--tools=", "--settings"}) ||
		!rejectsLean("error: unknown option '--strict-mcp-config'", []string{"--strict-mcp-config"}) {
		t.Fatal("a refused lean flag must be recognised")
	}
}
