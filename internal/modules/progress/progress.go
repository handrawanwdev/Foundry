// Package progress tracks execution of the command workflow by an external
// coding agent. Foundry records, validates and reconciles reports; it never
// runs the commands, writes project code, or executes the project's tests.
package progress

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/workflow"
)

const (
	DefaultStaleAfter = 30 * time.Minute
	maxReports        = 500
	maxHistory        = 400
)

var (
	ErrNoWorkflow = errors.New("workflow belum disusun")
	ErrInvalid    = errors.New("laporan tidak valid")
)

// ReportIn is the communication contract an external coding agent sends.
type ReportIn struct {
	ID              string          `json:"id"`
	Seq             int64           `json:"seq"`
	Project         string          `json:"project"`
	WorkflowVersion int             `json:"workflow_version"`
	Command         string          `json:"command"`
	Status          string          `json:"status"`
	StartedAt       *time.Time      `json:"started_at"`
	UpdatedAt       *time.Time      `json:"updated_at"`
	Summary         string          `json:"summary"`
	Evidence        []core.Evidence `json:"evidence"`
	Tests           []core.TestRun  `json:"tests"`
	Reason          string          `json:"reason"`
	Source          string          `json:"source"`
}

// ApplyResult is what the agent learns after Foundry stored the report.
type ApplyResult struct {
	Accepted      bool   `json:"accepted"`
	Duplicate     bool   `json:"duplicate,omitempty"`
	Ignored       bool   `json:"ignored,omitempty"`
	Reason        string `json:"reason,omitempty"`
	Command       string `json:"command"`
	Status        string `json:"status"`
	AppliedStatus string `json:"applied_status,omitempty"`
	ReportID      string `json:"report_id"`
	Seq           int64  `json:"seq"`
}

// Agent is the optional live connection of an MCP host. Empty means Foundry
// has no heartbeat; missing reports then age into stale, not unknown.
type Agent struct {
	Source  string
	At      time.Time
	Alive   bool
	Tracked bool
}

// Snapshot is the progress the UI and agents read. Derived fields (waiting,
// stale, unknown, next, complete) are computed; they are not stored.
type Snapshot struct {
	WorkflowVersion int                 `json:"workflow_version"`
	StaleAfterSec   int                 `json:"stale_after_sec"`
	UpdatedAt       time.Time           `json:"updated_at"`
	LastReportAt    *time.Time          `json:"last_report_at,omitempty"`
	LastSource      string              `json:"last_source,omitempty"`
	Complete        bool                `json:"complete"`
	Counts          Counts              `json:"counts"`
	Waves           []WaveProgress      `json:"waves"`
	Commands        []CommandProgress   `json:"commands"`
	Next            []string            `json:"next"`
	Warnings        []Warning           `json:"warnings"`
	Issues          []string            `json:"issues"`
	History         []core.StatusChange `json:"history"`
}

type Counts struct {
	Total       int `json:"total"`
	Unconfirmed int `json:"unconfirmed"`
	Waiting     int `json:"waiting"`
	Started     int `json:"started"`
	Blocked     int `json:"blocked"`
	Skipped     int `json:"skipped"`
	Failed      int `json:"failed"`
	Review      int `json:"review"`
	Done        int `json:"done"`
	Verified    int `json:"verified"`
	Stale       int `json:"stale"`
	Unknown     int `json:"unknown"`
}

type WaveProgress struct {
	Index    int      `json:"index"`
	Commands []string `json:"commands"`
	Verified int      `json:"verified"`
	Total    int      `json:"total"`
}

type CommandProgress struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	Status     string          `json:"status"`
	Stored     string          `json:"stored"`
	Wave       int             `json:"wave"`
	Waiting    []string        `json:"waiting,omitempty"`
	StartedAt  *time.Time      `json:"started_at,omitempty"`
	UpdatedAt  *time.Time      `json:"updated_at,omitempty"`
	Source     string          `json:"source,omitempty"`
	Summary    string          `json:"summary,omitempty"`
	Reason     string          `json:"reason,omitempty"`
	Evidence   []core.Evidence `json:"evidence,omitempty"`
	Tests      []core.TestRun  `json:"tests,omitempty"`
	EarlyStart bool            `json:"early_start,omitempty"`
	ReportID   string          `json:"report_id,omitempty"`
	Seq        int64           `json:"seq,omitempty"`
}

type Warning struct {
	Command string `json:"command,omitempty"`
	Kind    string `json:"kind"`
	Message string `json:"message"`
	Action  string `json:"action,omitempty"`
}

func ensure(p *core.Project) *core.Progress {
	if p.Progress == nil {
		p.Progress = &core.Progress{}
	}
	pr := p.Progress
	if pr.Commands == nil {
		pr.Commands = map[string]core.CommandRun{}
	}
	if pr.Retired == nil {
		pr.Retired = map[string]core.CommandRun{}
	}
	return pr
}

func staleAfter(pr *core.Progress) time.Duration {
	if pr != nil && pr.StaleAfterSec > 0 {
		return time.Duration(pr.StaleAfterSec) * time.Second
	}
	return DefaultStaleAfter
}

// Reconcile aligns stored runs with the current workflow: new commands start
// unconfirmed, removed commands keep their last run under Retired. It never
// invents a completed status.
func Reconcile(p *core.Project, now time.Time) {
	if p == nil {
		return
	}
	pr := ensure(p)
	pr.UpdatedAt = now.UTC()
	if p.Workflow == nil {
		return
	}
	pr.WorkflowVersion = p.Workflow.Version
	live := map[string]bool{}
	for _, c := range p.Workflow.Commands {
		live[c.ID] = true
		if _, ok := pr.Commands[c.ID]; ok {
			continue
		}
		if old, ok := pr.Retired[c.ID]; ok {
			pr.Commands[c.ID] = old
			delete(pr.Retired, c.ID)
			continue
		}
		pr.Commands[c.ID] = core.CommandRun{CommandID: c.ID, Status: core.RunUnconfirmed, UpdatedAt: now.UTC()}
	}
	for id, run := range pr.Commands {
		if !live[id] {
			pr.Retired[id] = run
			delete(pr.Commands, id)
		}
	}
}

// Apply stores a report then, if it is the newest for that command, updates
// the run. Duplicate ids and older seq/timestamps are kept but do not move
// status. Invalid reports are refused and not stored.
func Apply(p *core.Project, in ReportIn, now time.Time) (*ApplyResult, error) {
	if p == nil || p.Workflow == nil || len(p.Workflow.Commands) == 0 {
		return nil, ErrNoWorkflow
	}
	now = now.UTC()
	cmd := p.Workflow.Command(strings.TrimSpace(in.Command))
	if err := validate(p, in, cmd); err != nil {
		return nil, err
	}
	Reconcile(p, now)
	pr := ensure(p)
	id := strings.TrimSpace(in.ID)
	if id != "" {
		if prev := findReport(pr, id); prev != nil {
			return &ApplyResult{Accepted: true, Duplicate: true, Reason: "laporan dengan id ini sudah diterima",
				Command: in.Command, Status: pr.Commands[in.Command].Status, AppliedStatus: prev.AppliedStatus,
				ReportID: id, Seq: prev.Seq}, nil
		}
	}

	run := pr.Commands[cmd.ID]
	updated := now
	if in.UpdatedAt != nil && !in.UpdatedAt.IsZero() {
		updated = in.UpdatedAt.UTC()
	}
	late := false
	reason := ""
	if in.Seq > 0 && in.Seq <= run.Seq {
		late, reason = true, "laporan lebih lama dari status tersimpan (seq)"
	} else if in.Seq == 0 && !run.UpdatedAt.IsZero() && updated.Before(run.UpdatedAt) {
		late, reason = true, "laporan lebih lama dari status tersimpan (waktu)"
	}

	if id == "" {
		pr.Seq++
		id = fmt.Sprintf("rpt-%d", pr.Seq)
	} else if in.Seq > pr.Seq {
		pr.Seq = in.Seq
	} else if !late {
		pr.Seq++
	}
	seq := in.Seq
	if seq == 0 {
		seq = pr.Seq
	}

	rep := core.ProgressReport{
		ID: id, Seq: seq, Project: in.Project, WorkflowVersion: in.WorkflowVersion,
		Command: cmd.ID, Status: in.Status, StartedAt: utcPtr(in.StartedAt), UpdatedAt: updated,
		ReceivedAt: now, Summary: strings.TrimSpace(in.Summary), Reason: strings.TrimSpace(in.Reason),
		Evidence: slices.Clone(in.Evidence), Tests: slices.Clone(in.Tests), Source: strings.TrimSpace(in.Source),
		Outcome: "ignored",
	}
	out := &ApplyResult{Accepted: true, Command: cmd.ID, Status: run.Status, ReportID: id, Seq: seq}
	if late {
		rep.Outcome = "ignored"
		out.Ignored, out.Reason = true, reason
		appendReport(pr, rep)
		pr.UpdatedAt = now
		return out, nil
	}

	applied, why := decide(cmd, in, run)
	if in.StartedAt != nil {
		applied.StartedAt = utcPtr(in.StartedAt)
	} else if applied.Status == core.RunStarted && applied.StartedAt == nil {
		t := now
		applied.StartedAt = &t
	}
	if run.StartedAt != nil && applied.StartedAt == nil {
		applied.StartedAt = run.StartedAt
	}
	applied.CommandID = cmd.ID
	applied.UpdatedAt = updated
	applied.Source = strings.TrimSpace(in.Source)
	applied.Summary = strings.TrimSpace(in.Summary)
	applied.Seq = seq
	applied.ReportID = id
	if run.EarlyStart || (len(unmetDeps(p, cmd)) > 0 && (in.Status == core.RunStarted || in.Status == core.RunDone || in.Status == core.RunReview)) {
		applied.EarlyStart = true
	}

	rep.Outcome, rep.AppliedStatus = "applied", applied.Status
	appendReport(pr, rep)
	if run.Status != applied.Status {
		appendHistory(pr, core.StatusChange{At: now, Command: cmd.ID, From: run.Status, To: applied.Status,
			Reason: first(why, applied.Reason), ReportID: id, Source: applied.Source})
	}
	pr.Commands[cmd.ID] = applied
	pr.UpdatedAt = now
	out.Status, out.AppliedStatus, out.Reason = applied.Status, applied.Status, why
	return out, nil
}

func validate(p *core.Project, in ReportIn, cmd *core.Command) error {
	if strings.TrimSpace(in.Command) == "" {
		return fmt.Errorf("%w: command wajib diisi", ErrInvalid)
	}
	if cmd == nil {
		return fmt.Errorf("%w: command %s tidak ada di workflow", ErrInvalid, in.Command)
	}
	if !slices.Contains(core.AgentReportStatuses, in.Status) {
		return fmt.Errorf("%w: status %q tidak dikenal", ErrInvalid, in.Status)
	}
	if in.Status == core.RunSkipped && strings.TrimSpace(in.Reason) == "" {
		return fmt.Errorf("%w: command dilewati wajib menyertakan alasan", ErrInvalid)
	}
	if name := strings.TrimSpace(in.Project); name != "" && name != p.Name && name != p.Dir && name != filepath.Base(p.Dir) {
		return fmt.Errorf("%w: laporan untuk project %q, yang terbuka adalah %q", ErrInvalid, name, p.Name)
	}
	return nil
}

func decide(cmd *core.Command, in ReportIn, prev core.CommandRun) (core.CommandRun, string) {
	run := prev
	run.Reason = strings.TrimSpace(in.Reason)
	run.Evidence = compactEvidence(in.Evidence)
	run.Tests = slices.Clone(in.Tests)
	switch in.Status {
	case core.RunStarted:
		run.Status = core.RunStarted
		return run, ""
	case core.RunFailed:
		run.Status = core.RunFailed
		if run.Reason == "" {
			run.Reason = "agen melaporkan gagal"
		}
		return run, run.Reason
	case core.RunBlocked:
		run.Status = core.RunBlocked
		if run.Reason == "" {
			run.Reason = "agen melaporkan terhambat"
		}
		return run, run.Reason
	case core.RunSkipped:
		run.Status = core.RunSkipped
		return run, run.Reason
	case core.RunReview:
		run.Status = core.RunReview
		if run.Reason == "" {
			run.Reason = "agen menyerahkan untuk ditinjau"
		}
		return run, run.Reason
	case core.RunDone:
		return decideDone(cmd, run)
	}
	run.Status = in.Status
	return run, ""
}

func decideDone(cmd *core.Command, run core.CommandRun) (core.CommandRun, string) {
	need := len(cmd.Acceptance) > 0 || len(cmd.Outputs) > 0
	has := hasEvidence(run.Evidence)
	fail := hasFailedTest(run.Tests)
	switch {
	case need && !has:
		run.Status = core.RunReview
		run.Reason = "laporan selesai tanpa bukti yang dipersyaratkan"
		return run, run.Reason
	case fail:
		run.Status = core.RunReview
		run.Reason = "pengujian yang dilaporkan gagal"
		return run, run.Reason
	case has:
		run.Status = core.RunVerified
		return run, ""
	default:
		run.Status = core.RunDone
		return run, "selesai menurut agen, belum terverifikasi"
	}
}

func hasEvidence(ev []core.Evidence) bool {
	for _, e := range ev {
		if strings.TrimSpace(e.Ref) != "" {
			return true
		}
	}
	return false
}

func compactEvidence(ev []core.Evidence) []core.Evidence {
	var out []core.Evidence
	for _, e := range ev {
		ref := strings.TrimSpace(e.Ref)
		if ref == "" {
			continue
		}
		out = append(out, core.Evidence{Kind: strings.TrimSpace(e.Kind), Ref: ref})
	}
	return out
}

func hasFailedTest(tests []core.TestRun) bool {
	for _, t := range tests {
		if strings.EqualFold(strings.TrimSpace(t.Result), "fail") {
			return true
		}
	}
	return false
}

func unmetDeps(p *core.Project, cmd *core.Command) []string {
	var out []string
	pr := p.Progress
	for _, d := range cmd.DependsOn {
		st := core.RunUnconfirmed
		if pr != nil {
			if run, ok := pr.Commands[d]; ok && run.Status != "" {
				st = run.Status
			}
		}
		if st != core.RunVerified {
			out = append(out, d)
		}
	}
	return out
}

func findReport(pr *core.Progress, id string) *core.ProgressReport {
	for i := range pr.Reports {
		if pr.Reports[i].ID == id {
			return &pr.Reports[i]
		}
	}
	return nil
}

func appendReport(pr *core.Progress, r core.ProgressReport) {
	pr.Reports = append(pr.Reports, r)
	if n := len(pr.Reports); n > maxReports {
		pr.Reports = pr.Reports[n-maxReports:]
	}
}

func appendHistory(pr *core.Progress, h core.StatusChange) {
	pr.History = append(pr.History, h)
	if n := len(pr.History); n > maxHistory {
		pr.History = pr.History[n-maxHistory:]
	}
}

func utcPtr(t *time.Time) *time.Time {
	if t == nil || t.IsZero() {
		return nil
	}
	u := t.UTC()
	return &u
}

func first(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// View builds the read model from the workflow plus stored progress. It does
// not write. Missing reports stay unconfirmed; time alone never marks done
// or failed.
func View(p *core.Project, now time.Time, agent Agent) Snapshot {
	now = now.UTC()
	s := Snapshot{Waves: []WaveProgress{}, Commands: []CommandProgress{}, Next: []string{},
		Warnings: []Warning{}, Issues: []string{}, History: []core.StatusChange{},
		StaleAfterSec: int(DefaultStaleAfter / time.Second)}
	if p == nil || p.Workflow == nil {
		return s
	}
	s.WorkflowVersion = p.Workflow.Version
	pr := p.Progress
	if pr != nil {
		s.StaleAfterSec = int(staleAfter(pr) / time.Second)
		s.UpdatedAt = pr.UpdatedAt
		s.History = pr.History
		if s.History == nil {
			s.History = []core.StatusChange{}
		}
		if n := len(pr.Reports); n > 0 {
			last := pr.Reports[n-1]
			t := last.ReceivedAt
			s.LastReportAt, s.LastSource = &t, last.Source
		}
		if pr.WorkflowVersion != 0 && pr.WorkflowVersion != p.Workflow.Version {
			s.Issues = append(s.Issues, fmt.Sprintf("versi workflow tersimpan v%d, definisi sekarang v%d; status lama dipertahankan, command baru belum terkonfirmasi",
				pr.WorkflowVersion, p.Workflow.Version))
		}
		for id := range pr.Commands {
			if p.Workflow.Command(id) == nil {
				s.Issues = append(s.Issues, "command "+id+" ada di status tersimpan tetapi tidak ada di workflow")
			}
		}
	}
	waves := workflow.Waves(p.Workflow.Commands)
	waveOf := map[string]int{}
	for i, w := range waves {
		for _, id := range w {
			waveOf[id] = i + 1
		}
	}
	window := staleAfter(pr)
	byID := map[string]CommandProgress{}
	for _, c := range p.Workflow.Commands {
		cp := commandView(p, c, now, window, agent, waveOf[c.ID])
		s.Commands = append(s.Commands, cp)
		byID[c.ID] = cp
		addCount(&s.Counts, cp.Status)
		s.Counts.Total++
		switch cp.Status {
		case core.RunStale:
			s.Warnings = append(s.Warnings, Warning{Command: c.ID, Kind: "stale",
				Message: "Laporan " + c.ID + " kedaluwarsa. Pekerjaan tidak dianggap masih berjalan.",
				Action:  "Minta agen mengirim ulang laporan untuk command ini."})
		case core.RunUnknown:
			s.Warnings = append(s.Warnings, Warning{Command: c.ID, Kind: "unknown",
				Message: "Koneksi agen terputus. Status terakhir " + c.ID + " dipertahankan; pekerjaan tidak dianggap masih berjalan.",
				Action:  "Pulihkan koneksi agen, lalu minta laporan terbaru."})
		case core.RunSkipped:
			s.Warnings = append(s.Warnings, Warning{Command: c.ID, Kind: "skipped",
				Message: c.ID + " dilewati" + reasonSuffix(cp.Reason) + ". Tidak dihitung selesai.",
				Action:  "Kerjakan command ini atau biarkan terlewat dengan alasan yang sudah dicatat."})
		}
		if cp.EarlyStart && len(cp.Waiting) > 0 {
			s.Warnings = append(s.Warnings, Warning{Command: c.ID, Kind: "early",
				Message: c.ID + " dimulai sebelum prasyarat terpenuhi: " + strings.Join(cp.Waiting, ", ") + ".",
				Action:  "Selesaikan dan verifikasi dependency sebelum melanjutkan."})
		}
		if eligible(cp.Status) && len(cp.Waiting) == 0 {
			s.Next = append(s.Next, c.ID)
		}
	}
	for i, w := range waves {
		wp := WaveProgress{Index: i + 1, Commands: w, Total: len(w)}
		for _, id := range w {
			if byID[id].Status == core.RunVerified {
				wp.Verified++
			}
		}
		s.Waves = append(s.Waves, wp)
	}
	s.Complete = s.Counts.Total > 0 && s.Counts.Verified == s.Counts.Total
	if s.Counts.Total > 0 && !s.Complete {
		left := s.Counts.Total - s.Counts.Verified
		s.Warnings = append(s.Warnings, Warning{Kind: "incomplete",
			Message: fmt.Sprintf("Workflow belum selesai: %d command wajib belum terverifikasi.", left),
			Action:  "Lanjutkan command yang siap, atau pulihkan laporan yang hilang."})
	}
	return s
}

func commandView(p *core.Project, c core.Command, now time.Time, window time.Duration, agent Agent, wave int) CommandProgress {
	run := core.CommandRun{CommandID: c.ID, Status: core.RunUnconfirmed}
	if p.Progress != nil {
		if stored, ok := p.Progress.Commands[c.ID]; ok {
			run = stored
		}
	}
	stored := run.Status
	if stored == "" {
		stored = core.RunUnconfirmed
	}
	unmet := unmetDeps(p, &c)
	status := display(run, unmet, now, window, agent)
	cp := CommandProgress{ID: c.ID, Name: c.Name, Status: status, Stored: stored, Wave: wave,
		Waiting: unmet, StartedAt: run.StartedAt, Source: run.Source, Summary: run.Summary,
		Reason: run.Reason, Evidence: run.Evidence, Tests: run.Tests, EarlyStart: run.EarlyStart,
		ReportID: run.ReportID, Seq: run.Seq}
	if !run.UpdatedAt.IsZero() {
		t := run.UpdatedAt
		cp.UpdatedAt = &t
	}
	return cp
}

func display(run core.CommandRun, unmet []string, now time.Time, window time.Duration, agent Agent) string {
	st := run.Status
	if st == "" || st == core.RunUnconfirmed {
		if len(unmet) > 0 {
			return core.RunWaiting
		}
		return core.RunUnconfirmed
	}
	if st == core.RunStarted && agent.Tracked && !agent.Alive {
		return core.RunUnknown
	}
	if open(st) && !run.UpdatedAt.IsZero() && now.Sub(run.UpdatedAt) > window {
		return core.RunStale
	}
	return st
}

func open(st string) bool {
	return st == core.RunStarted || st == core.RunBlocked || st == core.RunReview
}

func eligible(st string) bool {
	switch st {
	case core.RunUnconfirmed, core.RunWaiting, core.RunFailed, core.RunBlocked, core.RunReview, core.RunStale, core.RunUnknown, core.RunDone:
		return true
	}
	return false
}

func addCount(c *Counts, st string) {
	switch st {
	case core.RunUnconfirmed:
		c.Unconfirmed++
	case core.RunWaiting:
		c.Waiting++
	case core.RunStarted:
		c.Started++
	case core.RunBlocked:
		c.Blocked++
	case core.RunSkipped:
		c.Skipped++
	case core.RunFailed:
		c.Failed++
	case core.RunReview:
		c.Review++
	case core.RunDone:
		c.Done++
	case core.RunVerified:
		c.Verified++
	case core.RunStale:
		c.Stale++
	case core.RunUnknown:
		c.Unknown++
	}
}

func reasonSuffix(r string) string {
	if r == "" {
		return ""
	}
	return ": " + r
}
