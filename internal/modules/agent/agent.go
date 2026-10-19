// Package agent runs external AI coding CLIs. An agent receives a prompt and
// returns text (think mode) or edits files in the project (execute mode).
// Only CLIs whose headless flags were verified are listed as presets; any
// other tool can be connected through a custom command or through MCP.
package agent

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/handrawanw/foundry/internal/proc"
)

type Preset struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Binary string `json:"binary"`
	// Models are suggestions only; any model name the CLI accepts works.
	Models []string `json:"models"`
	// Efforts lists the accepted effort levels; empty means the CLI has no
	// effort flag.
	Efforts []string `json:"efforts"`
	// PromptFile: flag that reads the prompt from a file instead of stdin.
	// A file avoids the kernel's per-argument limit (E2BIG, ~128 KiB).
	PromptFile string   `json:"-"`
	sub        []string // subcommand, before the model/effort flags
	// think: args after the model/effort flags. Every run is read-only:
	// Foundry asks for designs, never for file edits.
	think     []string
	effortArg func(string) []string
	// listModels asks the CLI (or its API) which models exist; see ListModels.
	listModels func(context.Context, Preset) ([]string, error)
	// unwrap turns a CLI's structured output into the answer text. Only set
	// for CLIs whose format was verified.
	unwrap func(raw string) (string, bool)
}

var Presets = []Preset{
	{ID: "claude", Name: "Claude Code", Binary: "claude",
		Models:     []string{"fable", "opus", "sonnet", "haiku"},
		listModels: claudeModels,
		Efforts:    []string{"low", "medium", "high", "xhigh", "max"},
		effortArg:  func(e string) []string { return []string{"--effort", e} },
		unwrap:     claudeResult,
		think:      []string{"-p", "--output-format", "json"}},
	{ID: "codex", Name: "Codex", Binary: "codex", sub: []string{"exec"},
		listModels: codexModels,
		Efforts:    []string{"low", "medium", "high", "xhigh"},
		effortArg:  func(e string) []string { return []string{"-c", fmt.Sprintf("model_reasoning_effort=%q", e)} },
		think:      []string{"--skip-git-repo-check", "-s", "read-only"}},
	{ID: "qwen", Name: "Qwen Code", Binary: "qwen",
		think: []string{"--approval-mode", "plan"}},
	{ID: "grok", Name: "Grok", Binary: "grok", PromptFile: "--prompt-file",
		Models:     []string{"grok-4.6", "grok-4.5"},
		listModels: grokModels,
		Efforts:    []string{"low", "medium", "high", "xhigh"},
		effortArg:  func(e string) []string { return []string{"--effort", e} },
		think:      []string{}},
	{ID: "gemini", Name: "Gemini", Binary: "gemini",
		think: []string{}},
	// opencode run reads the prompt from stdin; the plan agent is read-only.
	{ID: "opencode", Name: "OpenCode", Binary: "opencode", sub: []string{"run"},
		listModels: opencodeModels,
		think:      []string{"--agent", "plan"}},
}

// Options picks the model and effort; empty values keep the CLI default.
type Options struct {
	Model  string `json:"model,omitempty"`
	Effort string `json:"effort,omitempty"`
}

var modelName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/@\[\]-]*$`)

// args returns the model/effort flags, rejecting values the CLI would
// misread (a model starting with "-" would become a flag).
func (p Preset) args(o Options) ([]string, error) {
	var out []string
	if o.Model != "" {
		if !modelName.MatchString(o.Model) || len(o.Model) > 100 {
			return nil, fmt.Errorf("nama model %q tidak valid", o.Model)
		}
		out = append(out, "--model", o.Model)
	}
	if o.Effort != "" {
		if !slices.Contains(p.Efforts, o.Effort) {
			return nil, fmt.Errorf("%s tidak mendukung effort %q", p.Name, o.Effort)
		}
		out = append(out, p.effortArg(o.Effort)...)
	}
	return out, nil
}

func PresetByID(id string) (Preset, bool) {
	for _, p := range Presets {
		if p.ID == id {
			return p, true
		}
	}
	return Preset{}, false
}

// Detected reports whether the preset binary exists on PATH.
func (p Preset) Detected() (string, bool) {
	path, err := exec.LookPath(p.Binary)
	return path, err == nil
}

// Runner executes one configured agent.
type Runner struct {
	ID         string // preset id; empty for a custom command
	Name       string
	argv       []string
	promptFile string
	Timeout    time.Duration
	unwrap     func(raw string) (string, bool)
}

// NewRunner builds a runner from a preset id, or from a custom command when
// custom is non-empty (prompt goes to stdin).
func NewRunner(id string, custom []string, opt Options) (*Runner, error) {
	if len(custom) > 0 {
		return &Runner{Name: id, argv: custom, Timeout: 30 * time.Minute}, nil
	}
	p, ok := PresetByID(id)
	if !ok {
		return nil, fmt.Errorf("agent %q tidak dikenal", id)
	}
	if _, ok := p.Detected(); !ok {
		return nil, fmt.Errorf("%s tidak ditemukan di PATH (%s)", p.Name, p.Binary)
	}
	opts, err := p.args(opt)
	if err != nil {
		return nil, err
	}
	head := append(append([]string{p.Binary}, p.sub...), opts...)
	return &Runner{
		ID:         p.ID,
		Name:       p.Name,
		argv:       append(slices.Clone(head), p.think...),
		promptFile: p.PromptFile,
		unwrap:     p.unwrap,
		Timeout:    30 * time.Minute,
	}, nil
}

var ErrAgentFailed = errors.New("agent gagal")

// ErrLimit means the AI account ran out of quota or hit a rate limit; retrying
// cannot help until it resets.
var ErrLimit = fmt.Errorf("%w: limit/kuota AI habis", ErrAgentFailed)

// WaitDelay bounds how long Run waits for output after the agent exited or
// was stopped.
const WaitDelay = 5 * time.Second

var limitText = regexp.MustCompile(`(?i)(usage limit|rate[ _-]?limit|limit reached|reached (your|the) .{0,20}limit|quota (exceeded|exhausted)|insufficient[ _]quota|insufficient (account )?funds|out of (credits|quota)|exceeded your (current )?quota|resource[_ ]exhausted|too many requests|\b429\b|credit balance is too low|limit will reset)`)

// limitHit reports whether text reads like a quota or rate-limit message.
func limitHit(text string) bool { return limitText.MatchString(text) }

// watchBuf is an output buffer that checks for limit messages as they arrive (stderr only; stdout is the agent's prose
// and may legitimately talk about rate limits).
type watchBuf struct {
	mu       sync.Mutex
	buf      bytes.Buffer
	onLimit  func()
	tailText string
}

func (w *watchBuf) Write(p []byte) (int, error) {
	w.mu.Lock()
	w.buf.Write(p)
	hit := false
	if w.onLimit != nil {
		w.tailText = tail(w.tailText+string(p), 400)
		hit = limitHit(w.tailText)
	}
	w.mu.Unlock()
	if hit {
		w.onLimit()
	}
	return len(p), nil
}

func (w *watchBuf) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

// Run sends prompt to the agent with dir as working directory and returns
// its answer.
func (r *Runner) Run(ctx context.Context, dir, prompt string) (string, error) {
	argv := append([]string{}, r.argv...)
	ctx, cancel := context.WithTimeout(ctx, r.Timeout)
	defer cancel()
	var cmd *exec.Cmd
	if r.promptFile != "" {
		f, err := os.CreateTemp("", "foundry-prompt-*.txt")
		if err != nil {
			return "", fmt.Errorf("%w: %v", ErrAgentFailed, err)
		}
		defer os.Remove(f.Name())
		_, err = f.WriteString(prompt)
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return "", fmt.Errorf("%w: %v", ErrAgentFailed, err)
		}
		cmd = exec.CommandContext(ctx, argv[0], append(argv[1:], r.promptFile, f.Name())...)
	} else {
		cmd = exec.CommandContext(ctx, argv[0], argv[1:]...)
		cmd.Stdin = strings.NewReader(prompt)
	}
	cmd.Dir = dir
	// A child the agent left running (dev server, MCP server) keeps the
	// output pipes open; without a bound, Run would wait for it forever.
	cmd.WaitDelay = WaitDelay
	// Stopping the agent stops what it started; a server it left behind
	// is killed once it returns.
	proc.Group(cmd)
	defer proc.KillGroup(cmd)
	// cancelRun records why the watchdog stopped the run.
	var (
		reasonMu sync.Mutex
		reason   error
	)
	stop := func(err error) {
		reasonMu.Lock()
		if reason == nil {
			reason = err
		}
		reasonMu.Unlock()
		cancel()
	}
	out := &watchBuf{}
	errb := &watchBuf{onLimit: func() { stop(ErrLimit) }}
	cmd.Stdout, cmd.Stderr = out, errb
	err := cmd.Run()
	res := r.text(out.String())
	reasonMu.Lock()
	why := reason
	reasonMu.Unlock()
	if why != nil {
		return res, why
	}
	if ctx.Err() == context.DeadlineExceeded {
		return res, fmt.Errorf("%w: timeout setelah %s", ErrAgentFailed, r.Timeout)
	}
	failed := err != nil && !errors.Is(err, exec.ErrWaitDelay) // ErrWaitDelay: the agent itself exited 0
	// A CLI that hit its quota prints the notice and exits; stdout is only
	// trusted when the run failed or produced almost nothing.
	if limitHit(errb.String()) || ((failed || len(strings.TrimSpace(res)) < 400) && limitHit(res)) {
		return res, fmt.Errorf("%w: %s", ErrLimit, tail(strings.TrimSpace(errb.String()+"\n"+res), 300))
	}
	if failed {
		return res, fmt.Errorf("%w: %v: %s", ErrAgentFailed, err, tail(errb.String(), 600))
	}
	return res, nil
}

// Ping runs a minimal probe to prove the agent works headless.
func (r *Runner) Ping(ctx context.Context, dir string) error {
	return r.probe(ctx, dir, 2*time.Minute)
}

func (r *Runner) probe(ctx context.Context, dir string, timeout time.Duration) error {
	probe := *r
	probe.Timeout = timeout
	out, err := probe.Run(ctx, dir, "Balas hanya dengan satu kata: OK")
	if err != nil {
		return err
	}
	if strings.TrimSpace(out) == "" {
		return fmt.Errorf("%w: tidak ada output", ErrAgentFailed)
	}
	return nil
}

func tail(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return "…" + s[len(s)-n:]
	}
	return s
}

// maxCandidates caps how many models SuggestModel probes at once.
const maxCandidates = 8

// SuggestTimeout bounds the whole SuggestModel race.
const SuggestTimeout = 20 * time.Second

// SuggestModel looks for a model that answers when the current one does not
// (empty quota, silent hang). It probes a few listed models in parallel and
// returns the first that replies; free models go first because an empty
// balance is the usual cause. Empty means none replied or the agent is custom.
func (r *Runner) SuggestModel(ctx context.Context, dir string) string {
	p, ok := PresetByID(r.ID)
	if !ok || p.listModels == nil {
		return ""
	}
	models := slices.Clone(p.ListModels(ctx))
	slices.SortStableFunc(models, func(a, b string) int {
		return boolInt(strings.Contains(b, "free")) - boolInt(strings.Contains(a, "free"))
	})
	if len(models) > maxCandidates {
		models = models[:maxCandidates]
	}
	ctx, cancel := context.WithTimeout(ctx, SuggestTimeout)
	defer cancel()
	won := make(chan string, len(models))
	var wg sync.WaitGroup
	for _, m := range models {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cand, err := NewRunner(r.ID, nil, Options{Model: m})
			if err != nil {
				return
			}
			if cand.probe(ctx, dir, SuggestTimeout) == nil {
				won <- m
			}
		}()
	}
	go func() { wg.Wait(); close(won) }()
	return <-won // "" once every probe failed
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
