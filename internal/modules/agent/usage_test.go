package agent

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// testdata/claude_result.json is a real `claude -p --output-format json`
// answer of Claude Code 2.1.322, with the session ids taken out.
func TestClaudeUsage(t *testing.T) {
	raw, err := os.ReadFile("testdata/claude_result.json")
	if err != nil {
		t.Fatal(err)
	}
	u := claudeUsage(string(raw))
	if u == nil || *u.InputTokens != 2 || *u.OutputTokens != 4 || *u.CacheCreationTokens != 1235 || *u.CacheReadTokens != 3173 ||
		*u.NumTurns != 1 || *u.DurationMS != 1007 || *u.Model != "claude-haiku-5-5" || *u.ServiceTier != "standard" ||
		*u.CacheWrite1h != 1235 || *u.CacheWrite5m != 0 {
		t.Fatalf("usage = %+v", u)
	}
	if *u.Total() != 2+4+1235+3173 {
		t.Fatalf("total = %d", *u.Total())
	}
	if text, ok := claudeResult(string(raw)); !ok || text != "ok" {
		t.Fatalf("result = %q %v", text, ok)
	}
}

func TestClaudeUsageGaps(t *testing.T) {
	cases := map[string]func(*Usage) bool{
		// fields missing: nil, not zero
		`{"type":"result","result":"x"}`: func(u *Usage) bool {
			return u != nil && u.InputTokens == nil && u.Model == nil && u.Total() == nil
		},
		// null fields
		`{"type":"result","result":"x","num_turns":null,"usage":{"input_tokens":null,"output_tokens":7}}`: func(u *Usage) bool {
			return u != nil && u.InputTokens == nil && *u.OutputTokens == 7 && u.NumTurns == nil && u.Total() == nil
		},
		// more than one model in one run
		`{"type":"result","result":"x","modelUsage":{"claude-opus-5-5":{},"claude-haiku-5-5":{}},"usage":{"input_tokens":1,"output_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}`: func(u *Usage) bool {
			return u != nil && *u.Model == "claude-haiku-5-5,claude-opus-5-5" && *u.Total() == 2
		},
		`not json`:                 func(u *Usage) bool { return u == nil },
		`{"type":"assistant"}`:     func(u *Usage) bool { return u == nil },
		"":                         func(u *Usage) bool { return u == nil },
		`{"type":"result","usage"`: func(u *Usage) bool { return u == nil },
	}
	for raw, ok := range cases {
		if u := claudeUsage(raw); !ok(u) {
			t.Errorf("%q: usage = %+v", raw, u)
		}
	}
}

// A failed run still hands back the usage it reported, and a CLI without
// usage support reports none.
func TestExecUsage(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh tidak ada")
	}
	out := `{"type":"result","result":"err","is_error":true,"usage":{"input_tokens":3,"output_tokens":0,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}`
	r := &Runner{argv: []string{"sh", "-c", `cat >/dev/null; echo "$OUT"; exit 1`, "sh"}, unwrap: claudeResult, usage: claudeUsage, Timeout: time.Minute}
	t.Setenv("OUT", out)
	res, err := r.Exec(context.Background(), t.TempDir(), Prompt{Body: "hi"})
	if err == nil || res.Usage == nil || *res.Usage.InputTokens != 3 || res.Calls != 1 {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	r.usage = nil
	if res, _ := r.Exec(context.Background(), t.TempDir(), Prompt{Body: "hi"}); res.Usage != nil {
		t.Fatal("usage without a reader")
	}
}

// With lean mode the static part goes to the system prompt file and only
// the body to stdin; without it, or for a CLI without the flag, the agent
// gets the whole prompt as one text.
func TestExecSplitsPrompt(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh tidak ada")
	}
	script := `in=$(cat); f=""; prev=""; for a in "$@"; do [ "$prev" = --sys ] && f="$a"; prev="$a"; done; if [ -n "$f" ]; then printf 'SYS[%s]' "$(cat "$f")"; fi; printf 'IN[%s]' "$in"`
	mk := func(sys string) *Runner {
		return &Runner{argv: []string{"sh", "-c", script, "sh"}, lean: []string{"--lean"}, noLean: new(atomic.Bool), systemFile: sys, Timeout: time.Minute}
	}
	p := Prompt{Static: "rules", Body: "task"}
	if res, err := mk("--sys").Exec(context.Background(), t.TempDir(), p); err != nil || res.Text != "SYS[rules]IN[task]" {
		t.Fatalf("split: %q %v", res.Text, err)
	}
	if res, _ := mk("").Exec(context.Background(), t.TempDir(), p); res.Text != "IN[rulestask]" {
		t.Fatalf("no system flag: %q", res.Text)
	}
	r := mk("--sys")
	r.noLean.Store(true)
	if res, _ := r.Exec(context.Background(), t.TempDir(), p); res.Text != "IN[rulestask]" {
		t.Fatalf("lean off: %q", res.Text)
	}
	if res, _ := mk("--sys").Exec(context.Background(), t.TempDir(), Prompt{Body: "only"}); res.Text != "IN[only]" {
		t.Fatalf("no static: %q", res.Text)
	}
	// The temp file is gone after the run.
	left, _ := filepath.Glob(filepath.Join(os.TempDir(), "foundry-system-*.txt"))
	for _, f := range left {
		if b, _ := os.ReadFile(f); strings.Contains(string(b), "rules") {
			t.Fatalf("system prompt file left behind: %s", f)
		}
	}
}

// A CLI that refuses the system prompt flag drops lean once and still gets
// the whole prompt.
func TestExecDropsRefusedSystemFlag(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh tidak ada")
	}
	script := `in=$(cat); for a in "$@"; do [ "$a" = --sys ] && { echo "error: unknown option '--sys'" >&2; exit 1; }; done; printf '%s' "$in"`
	r := &Runner{argv: []string{"sh", "-c", script, "sh"}, lean: []string{"--lean"}, noLean: new(atomic.Bool), systemFile: "--sys", Timeout: time.Minute}
	res, err := r.Exec(context.Background(), t.TempDir(), Prompt{Static: "a", Body: "b"})
	if err != nil || res.Text != "ab" || res.Calls != 2 || !r.noLean.Load() {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}
