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
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"time"
)

type Mode int

const (
	Think   Mode = iota // read-only reasoning, returns text
	Execute             // may edit files inside the project directory
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
	// PromptArg: prompt is passed as the last argument instead of stdin.
	PromptArg bool     `json:"-"`
	sub       []string // subcommand, before the model/effort flags
	think     []string // args after the model/effort flags
	execute   []string
	effortArg func(string) []string
}

var Presets = []Preset{
	{ID: "claude", Name: "Claude Code", Binary: "claude",
		Models:    []string{"fable", "opus", "sonnet", "haiku"},
		Efforts:   []string{"low", "medium", "high", "xhigh", "max"},
		effortArg: func(e string) []string { return []string{"--effort", e} },
		think:     []string{"-p"},
		execute:   []string{"-p", "--permission-mode", "acceptEdits"}},
	{ID: "codex", Name: "Codex", Binary: "codex", sub: []string{"exec"},
		Efforts:   []string{"low", "medium", "high", "xhigh"},
		effortArg: func(e string) []string { return []string{"-c", fmt.Sprintf("model_reasoning_effort=%q", e)} },
		think:     []string{"--skip-git-repo-check", "-s", "read-only"},
		execute:   []string{"--skip-git-repo-check", "-s", "workspace-write"}},
	{ID: "qwen", Name: "Qwen Code", Binary: "qwen",
		think:   []string{"--approval-mode", "plan"},
		execute: []string{"--approval-mode", "auto-edit"}},
	{ID: "grok", Name: "Grok", Binary: "grok", PromptArg: true,
		Models:    []string{"grok-4.6", "grok-4.5"},
		Efforts:   []string{"low", "medium", "high", "xhigh"},
		effortArg: func(e string) []string { return []string{"--effort", e} },
		think:     []string{"-p"},
		execute:   []string{"--always-approve", "-p"}},
	{ID: "gemini", Name: "Gemini", Binary: "gemini",
		think:   []string{},
		execute: []string{"--approval-mode", "auto_edit"}},
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
	Name      string
	argvThink []string
	argvExec  []string
	promptArg bool
	Timeout   time.Duration
}

// NewRunner builds a runner from a preset id, or from a custom command when
// custom is non-empty (prompt goes to stdin, same command for both modes).
func NewRunner(id string, custom []string, opt Options) (*Runner, error) {
	if len(custom) > 0 {
		return &Runner{Name: id, argvThink: custom, argvExec: custom, Timeout: 30 * time.Minute}, nil
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
		Name:      p.Name,
		argvThink: append(slices.Clone(head), p.think...),
		argvExec:  append(slices.Clone(head), p.execute...),
		promptArg: p.PromptArg,
		Timeout:   30 * time.Minute,
	}, nil
}

var ErrAgentFailed = errors.New("agent gagal")

// Run sends prompt to the agent with dir as working directory.
func (r *Runner) Run(ctx context.Context, mode Mode, dir, prompt string) (string, error) {
	argv := r.argvThink
	if mode == Execute {
		argv = r.argvExec
	}
	argv = append([]string{}, argv...)
	ctx, cancel := context.WithTimeout(ctx, r.Timeout)
	defer cancel()
	var cmd *exec.Cmd
	if r.promptArg {
		cmd = exec.CommandContext(ctx, argv[0], append(argv[1:], prompt)...)
	} else {
		cmd = exec.CommandContext(ctx, argv[0], argv[1:]...)
		cmd.Stdin = strings.NewReader(prompt)
	}
	cmd.Dir = dir
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return out.String(), fmt.Errorf("%w: timeout setelah %s", ErrAgentFailed, r.Timeout)
	}
	if err != nil {
		return out.String(), fmt.Errorf("%w: %v: %s", ErrAgentFailed, err, tail(errb.String(), 600))
	}
	return out.String(), nil
}

// Ping runs a minimal probe to prove the agent works headless.
func (r *Runner) Ping(ctx context.Context, dir string) error {
	saved := r.Timeout
	r.Timeout = 2 * time.Minute
	defer func() { r.Timeout = saved }()
	out, err := r.Run(ctx, Think, dir, "Balas hanya dengan satu kata: OK")
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
