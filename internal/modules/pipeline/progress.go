package pipeline

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/handrawanwdev/foundry/internal/config"
	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/progress"
	"github.com/handrawanwdev/foundry/internal/modules/spec"
	"github.com/handrawanwdev/foundry/internal/modules/work"
	"github.com/handrawanwdev/foundry/internal/modules/workflow"
)

// ProgressView is the current execution snapshot. Derived; does not write.
func (s *Service) ProgressView() (progress.Snapshot, error) {
	p, err := s.Store.Load()
	if err != nil {
		return progress.Snapshot{}, err
	}
	return s.ProgressOf(p), nil
}

// ProgressOf is View with this service's MCP heartbeat and the commands whose
// blueprint artifacts changed.
func (s *Service) ProgressOf(p *core.Project) progress.Snapshot {
	return progress.ViewStale(p, time.Now().UTC(), s.progressAgent(), staleCommands(p))
}

func staleCommands(p *core.Project) map[string][]string {
	if p == nil || p.Workflow == nil {
		return nil
	}
	return workflow.Stale(p.Workflow.Commands, workflow.Hashes(spec.Artifacts(p)))
}

// applyWorkflowDelta lands an AI answer and records what it changed, so a
// revision keeps every earlier text and the progress of untouched commands.
func applyWorkflowDelta(p *core.Project, raw string) error {
	var before []core.Command
	kind, reason := "generate", "Disusun dari blueprint"
	if w := p.Workflow; w != nil && len(w.Commands) > 0 {
		before, kind = slices.Clone(w.Commands), "revise"
		if w.Request != nil {
			reason = w.Request.Text
		}
	}
	if err := work.ApplyWorkflow(p, raw, spec.Artifacts(p)); err != nil {
		return err
	}
	progress.Track(p, before, kind, reason, "ai", time.Now().UTC())
	return nil
}

// CommandBrief is everything an agent needs to carry out one command.
type CommandBrief struct {
	Command  core.Command             `json:"command"`
	Progress progress.CommandProgress `json:"progress"`
	// Markdown is the command as the exported workflow file has it.
	Markdown string `json:"markdown"`
	// Dependencies are what the finished commands before this one left.
	Dependencies     []DependencyResult `json:"dependencies"`
	Finalized        bool               `json:"finalized"`
	WorkflowVersion  int                `json:"workflow_version"`
	BindingDecisions []core.Decision    `json:"binding_decisions"`
	Report           string             `json:"report"`
}

// DependencyResult is the outcome of one dependency.
type DependencyResult struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Status   string          `json:"status"`
	Summary  string          `json:"summary,omitempty"`
	Evidence []core.Evidence `json:"evidence,omitempty"`
}

// NextResult is the answer to "what do I do now".
type NextResult struct {
	progress.Pick
	Brief    *CommandBrief     `json:"brief,omitempty"`
	Overview progress.Overview `json:"overview"`
}

const reportHint = "Laporkan dengan report_progress, `foundry command`, POST /api/v1/project/progress/reports, atau file JSON di .foundry/inbox: started, progress (ringkasan dan percent), lalu done dengan bukti, failed, blocked, review, atau skipped."

func (s *Service) brief(p *core.Project, id string) *CommandBrief {
	c := p.Workflow.Command(id)
	if c == nil {
		return nil
	}
	snap := s.ProgressOf(p)
	view := map[string]progress.CommandProgress{}
	for _, cp := range snap.Commands {
		view[cp.ID] = cp
	}
	b := &CommandBrief{Command: *c, Progress: view[id], Markdown: spec.CommandMarkdown(*c, workflow.Waves(p.Workflow.Commands)),
		Dependencies: []DependencyResult{}, Finalized: p.Finalized(), WorkflowVersion: p.Workflow.Version,
		BindingDecisions: p.BindingDecisions(), Report: reportHint}
	if b.BindingDecisions == nil {
		b.BindingDecisions = []core.Decision{}
	}
	for _, d := range c.DependsOn {
		dep := view[d]
		b.Dependencies = append(b.Dependencies, DependencyResult{ID: d, Name: dep.Name, Status: dep.Status, Summary: dep.Summary, Evidence: dep.Evidence})
	}
	return b
}

// Command is the brief of one command.
func (s *Service) Command(id string) (*CommandBrief, error) {
	p, err := s.Store.Load()
	if err != nil {
		return nil, err
	}
	if p.Workflow == nil {
		return nil, progress.ErrNoWorkflow
	}
	b := s.brief(p, id)
	if b == nil {
		return nil, fmt.Errorf("%w: command %s tidak ada di workflow", progress.ErrInvalid, id)
	}
	return b, nil
}

// CommandDetail is the reports, status changes and revisions of a command.
func (s *Service) CommandDetail(id string) (*progress.CommandDetail, error) {
	p, err := s.Store.Load()
	if err != nil {
		return nil, err
	}
	return progress.Detail(p, id)
}

// NextCommand says what the session should do now without changing anything.
func (s *Service) NextCommand(session string) (*NextResult, error) {
	p, err := s.Store.Load()
	if err != nil {
		return nil, err
	}
	if p.Workflow == nil || len(p.Workflow.Commands) == 0 {
		return nil, progress.ErrNoWorkflow
	}
	snap := s.ProgressOf(p)
	out := &NextResult{Pick: progress.Next(snap, session), Overview: snap.Overview}
	if out.Command != "" {
		out.Brief = s.brief(p, out.Command)
	}
	return out, nil
}

// errNoClaim leaves the state alone when nothing was handed out.
var errNoClaim = errors.New("tidak ada command diambil")

// ClaimCommand hands one command to a session and marks it started. id ""
// takes the next one. A command with unmet dependencies is refused.
func (s *Service) ClaimCommand(id, session, source string) (*NextResult, error) {
	var out *NextResult
	_, err := s.Store.Update(func(p *core.Project) error {
		if p.Workflow == nil || len(p.Workflow.Commands) == 0 {
			return progress.ErrNoWorkflow
		}
		pick, err := progress.Claim(p, id, session, source, time.Now().UTC(), s.progressAgent(), staleCommands(p))
		if err != nil {
			return err
		}
		out = &NextResult{Pick: pick, Overview: s.ProgressOf(p).Overview}
		if pick.Command != "" {
			out.Brief = s.brief(p, pick.Command)
		}
		if pick.Command == "" || pick.Resumed {
			return errNoClaim
		}
		p.Log(logSource(source), "Command diambil: "+pick.Command)
		return nil
	})
	if errors.Is(err, errNoClaim) {
		err = nil
	}
	return out, err
}

func logSource(src string) string {
	if src == "" {
		return "agent"
	}
	return src
}

// inboxDir is where an agent without MCP, HTTP or the CLI drops reports.
const inboxDir = "inbox"

const maxInboxFile = 1 << 20

// IngestInbox applies the report files in .foundry/inbox. Each file is one
// ReportIn. Its name is the report id unless it carries one, so a file that
// was applied but not moved away cannot count twice. Applied files go to
// inbox/done, refused ones to inbox/rejected with the reason beside them.
func (s *Service) IngestInbox() (applied, rejected int, err error) {
	dir := filepath.Join(s.Store.Dir, core.StateDir, inboxDir)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return 0, 0, nil
	}
	if err != nil {
		return 0, 0, err
	}
	for _, e := range entries {
		if !e.Type().IsRegular() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		if cause := s.ingestFile(path, e.Name()); cause != nil {
			if errors.Is(cause, core.ErrLocked) {
				return applied, rejected, cause // try this file again next time
			}
			rejected++
			moveReport(path, filepath.Join(dir, "rejected"), cause.Error())
			continue
		}
		applied++
		moveReport(path, filepath.Join(dir, "done"), "")
	}
	return applied, rejected, nil
}

func (s *Service) ingestFile(path, name string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Size() > maxInboxFile {
		return fmt.Errorf("%w: file laporan lebih dari %d byte", progress.ErrInvalid, maxInboxFile)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var in progress.ReportIn
	if err := json.Unmarshal(b, &in); err != nil {
		return fmt.Errorf("%w: %v", progress.ErrInvalid, err)
	}
	if in.ID == "" {
		in.ID = "file-" + strings.TrimSuffix(name, ".json")
	}
	if in.Source == "" {
		in.Source = "file"
	}
	_, err = s.ReportProgress(in, "agent")
	return err
}

func moveReport(path, dir, reason string) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	dest := filepath.Join(dir, filepath.Base(path))
	if reason != "" {
		_ = os.WriteFile(dest+".error", []byte(reason+"\n"), 0o644)
	}
	_ = os.Rename(path, dest)
}

// ReportProgress stores an agent report and updates the command run.
func (s *Service) ReportProgress(in progress.ReportIn, logSrc string) (*progress.ApplyResult, error) {
	var out *progress.ApplyResult
	_, err := s.Store.Update(func(p *core.Project) error {
		res, err := progress.Apply(p, in, time.Now().UTC())
		if err != nil {
			return err
		}
		out = res
		src := logSrc
		if src == "" {
			src = "agent"
		}
		if in.Status == core.RunProgress {
			return nil // a note of ongoing work, not an event for the project log
		}
		msg := "Laporan progres " + in.Command + ": " + res.Status
		if res.Duplicate {
			msg = "Laporan progres " + in.Command + " (duplikat)"
		} else if res.Ignored {
			msg = "Laporan progres " + in.Command + " diabaikan (lama)"
		}
		p.Log(src, msg)
		return nil
	})
	return out, err
}

// ReconcileProgress aligns stored runs with the current workflow and
// optionally records a new stale window. It does not invent statuses.
func (s *Service) ReconcileProgress(staleAfterSec int) (progress.Snapshot, error) {
	p, err := s.Store.Update(func(p *core.Project) error {
		if p.Workflow == nil || len(p.Workflow.Commands) == 0 {
			return errors.New("workflow belum disusun")
		}
		now := time.Now().UTC()
		progress.Reconcile(p, now)
		if staleAfterSec > 0 {
			p.Progress.StaleAfterSec = staleAfterSec
		}
		p.Log("system", "Rekonsiliasi progres workflow")
		return nil
	})
	if err != nil {
		return progress.Snapshot{}, err
	}
	return progress.View(p, time.Now().UTC(), s.progressAgent()), nil
}

func (s *Service) progressAgent() progress.Agent {
	c, err := config.Load()
	if err != nil || c == nil || c.MCP == nil || c.MCP.Dir != s.Store.Dir {
		return progress.Agent{}
	}
	return progress.Agent{Source: c.MCP.Client, At: c.MCP.At, Alive: c.MCPAlive(), Tracked: true}
}
