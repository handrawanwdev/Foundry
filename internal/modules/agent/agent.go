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
	"sync/atomic"
	"time"

	"github.com/handrawanwdev/foundry/internal/proc"
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
	think []string
	// lean: args that keep the CLI's own context out of a run (built-in
	// tools, MCP servers, skills, hooks). Foundry prompts carry everything
	// the answer needs, so a run never needs them. A CLI too old to know a
	// flag rejects it; the runner then drops lean for good and retries.
	lean []string
	// systemFile is the lean flag that takes a file as extra system prompt;
	// see Prompt. Empty: the CLI gets the prompt as one text.
	systemFile string
	// usage reads the token usage from the CLI's raw output; nil when the
	// CLI reports none.
	usage     func(raw string) *Usage
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
		usage:      claudeUsage,
		systemFile: "--append-system-prompt-file",
		think:      []string{"-p", "--output-format", "json"},
		lean:       []string{"--tools=", "--strict-mcp-config", "--disable-slash-commands", "--settings", `{"disableAllHooks":true}`}},
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
	// The listed models are free ones, used when the default model has no
	// balance.
	{ID: "opencode", Name: "OpenCode", Binary: "opencode", sub: []string{"run"},
		Models:     []string{"opencode/big-pickle", "opencode/deepseek-v4.1-flash-free"},
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
	lean       []string
	noLean     *atomic.Bool // set once the CLI rejected a lean flag
	promptFile string
	systemFile string // flag that reads the static prompt part as system prompt
	Timeout    time.Duration
	unwrap     func(raw string) (string, bool)
	usage      func(raw string) *Usage
	opt        Options
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
		lean:       p.lean,
		noLean:     new(atomic.Bool),
		promptFile: p.PromptFile,
		systemFile: p.systemFile,
		unwrap:     p.unwrap,
		usage:      p.usage,
		Timeout:    30 * time.Minute,
		opt:        opt,
	}, nil
}

var ErrAgentFailed = errors.New("agent gagal")

// ErrLimit means the AI account ran out of quota or hit a rate limit; retrying
// cannot help until it resets.
var ErrLimit = fmt.Errorf("%w: limit/kuota AI habis", ErrAgentFailed)

// ErrOverloaded means the AI server had no room for this run; the same prompt
// can pass once it has.
var ErrOverloaded = fmt.Errorf("%w: server AI sedang sibuk", ErrAgentFailed)

// WaitDelay bounds how long Run waits for output after the agent exited or
// was stopped.
const WaitDelay = 5 * time.Second

var limitText = regexp.MustCompile(`(?i)(usage limit|rate[ _-]?limit|limit reached|reached (your|the) .{0,20}limit|quota (exceeded|exhausted)|insufficient[ _]quota|insufficient (account )?funds|out of (credits|quota)|exceeded your (current )?quota|resource[_ ]exhausted|too many requests|\b429\b|credit balance is too low|limit will reset)`)

var overloadText = regexp.MustCompile(`(?i)\boverloaded\b`)

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

var unknownFlag = regexp.MustCompile(`(?i)unknown (option|flag|argument)|unexpected argument`)

// rejectsLean reports whether a failed run's stderr is the CLI refusing one
// of the lean flags: an unknown-option message that names that flag. Any
// other failure (auth, network, API, a bad answer) is not, so it never
// costs a second run.
func rejectsLean(stderr string, lean []string) bool {
	if !unknownFlag.MatchString(stderr) {
		return false
	}
	for _, f := range lean {
		if name, _, _ := strings.Cut(f, "="); strings.HasPrefix(name, "--") && strings.Contains(stderr, name) {
			return true
		}
	}
	return false
}

// Prompt is one run's prompt. Static is the part that stays the same from
// run to run (rules, project briefs, blueprint); a CLI that can take it as
// its system prompt keeps it in the provider's prompt cache, so the next run
// reads it from there instead of paying for it again. Body is the rest. A
// CLI without that option gets Static + Body as one prompt.
type Prompt struct {
	Static string
	Body   string
}

// Text is the whole prompt as one text.
func (p Prompt) Text() string { return p.Static + p.Body }

// Result is one run's answer and what the CLI reported about it.
type Result struct {
	Text string
	// Usage is nil when the CLI reports no usage or its output could not
	// be read; it is never estimated.
	Usage *Usage
	// Calls counts the CLI processes started: 2 when it refused a lean flag
	// and the run was repeated without lean.
	Calls int
}

// Run sends prompt to the agent with dir as working directory and returns
// its answer.
func (r *Runner) Run(ctx context.Context, dir, prompt string) (string, error) {
	res, err := r.Exec(ctx, dir, Prompt{Body: prompt})
	return res.Text, err
}

// Exec is Run with a split prompt and the usage the CLI reported.
func (r *Runner) Exec(ctx context.Context, dir string, p Prompt) (Result, error) {
	lean := len(r.lean) > 0 && r.noLean != nil && !r.noLean.Load()
	out, err := r.run(ctx, dir, p, lean)
	calls := 1
	if lean && err != nil && rejectsLean(out.stderr, r.leanFlags()) {
		r.noLean.Store(true)
		out, err = r.run(ctx, dir, p, false)
		calls++
	}
	res := Result{Text: out.text, Calls: calls}
	if r.usage != nil {
		res.Usage = r.usage(out.raw)
	}
	return res, err
}

// leanFlags are the flags lean mode adds, the system prompt one included.
func (r *Runner) leanFlags() []string {
	if r.systemFile == "" {
		return r.lean
	}
	return append(slices.Clone(r.lean), r.systemFile)
}

type runOut struct{ raw, text, stderr string }

func (r *Runner) run(ctx context.Context, dir string, p Prompt, lean bool) (runOut, error) {
	argv := append([]string{}, r.argv...)
	if lean {
		argv = append(argv, r.lean...)
	}
	prompt := p.Text()
	if lean && r.systemFile != "" && p.Static != "" {
		f, err := writeTemp("foundry-system-*.txt", p.Static)
		if err != nil {
			return runOut{}, fmt.Errorf("%w: %v", ErrAgentFailed, err)
		}
		defer os.Remove(f)
		argv = append(argv, r.systemFile, f)
		prompt = p.Body
	}
	ctx, cancel := context.WithTimeout(ctx, r.Timeout)
	defer cancel()
	var cmd *exec.Cmd
	if r.promptFile != "" {
		f, err := writeTemp("foundry-prompt-*.txt", prompt)
		if err != nil {
			return runOut{}, fmt.Errorf("%w: %v", ErrAgentFailed, err)
		}
		defer os.Remove(f)
		cmd = exec.CommandContext(ctx, argv[0], append(argv[1:], r.promptFile, f)...)
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
	raw := out.String()
	res := r.text(raw)
	o := runOut{raw: raw, text: res, stderr: errb.String()}
	reasonMu.Lock()
	why := reason
	reasonMu.Unlock()
	if why != nil {
		return o, why
	}
	if ctx.Err() == context.DeadlineExceeded {
		return o, fmt.Errorf("%w: timeout setelah %s", ErrAgentFailed, r.Timeout)
	}
	failed := err != nil && !errors.Is(err, exec.ErrWaitDelay) // ErrWaitDelay: the agent itself exited 0
	// A CLI that hit its quota prints the notice and exits; stdout is only
	// trusted when the run failed or produced almost nothing.
	if limitHit(o.stderr) || ((failed || len(strings.TrimSpace(res)) < 400) && limitHit(res)) {
		return o, fmt.Errorf("%w: %s", ErrLimit, tail(strings.TrimSpace(o.stderr+"\n"+res), 300))
	}
	if failed && (overloadText.MatchString(o.stderr) || (len(strings.TrimSpace(res)) < 400 && overloadText.MatchString(res))) {
		return o, fmt.Errorf("%w: %s", ErrOverloaded, tail(strings.TrimSpace(o.stderr+"\n"+res), 300))
	}
	if failed {
		return o, fmt.Errorf("%w: %v: %s", ErrAgentFailed, err, tail(o.stderr, 600))
	}
	return o, nil
}

// writeTemp writes text to a new private temp file and returns its path.
func writeTemp(pattern, text string) (string, error) {
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", err
	}
	_, err = f.WriteString(text)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
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

// Connect probes the agent and, when the chosen model has no quota or balance,
// retries with a listed model that answers. It returns the model to store, or
// "" when the configured model already works.
func (r *Runner) Connect(ctx context.Context, dir string) (string, error) {
	err := r.Ping(ctx, dir)
	if err == nil {
		return "", nil
	}
	if !errors.Is(err, ErrLimit) || r.ID == "" {
		return "", err
	}
	model := r.SuggestModel(ctx, dir)
	if model == "" {
		return "", err
	}
	retry, err := NewRunner(r.ID, nil, Options{Model: model, Effort: r.opt.Effort})
	if err != nil {
		return "", err
	}
	if err := retry.Ping(ctx, dir); err != nil {
		return "", err
	}
	return model, nil
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

var (
	versionMu sync.Mutex
	versions  = map[string]string{}
)

// Version is the first line of `<binary> --version` of a preset CLI, asked
// once per process; "" for a custom command or a CLI that does not answer.
func (r *Runner) Version(ctx context.Context) string {
	if r.ID == "" || len(r.argv) == 0 {
		return ""
	}
	bin := r.argv[0]
	versionMu.Lock()
	defer versionMu.Unlock()
	if v, ok := versions[bin]; ok {
		return v
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, _ := exec.CommandContext(ctx, bin, "--version").Output()
	v, _, _ := strings.Cut(strings.TrimSpace(string(out)), "\n")
	if len(v) > 60 {
		v = v[:60]
	}
	versions[bin] = v
	return v
}
