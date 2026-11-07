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
	maxChanges        = 100
	maxActivity       = 40
	// updatingFor is how long a progress report shows as live work.
	updatingFor = 5 * time.Minute
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
	FinishedAt      *time.Time      `json:"finished_at"`
	UpdatedAt       *time.Time      `json:"updated_at"`
	Summary         string          `json:"summary"`
	Evidence        []core.Evidence `json:"evidence"`
	Tests           []core.TestRun  `json:"tests"`
	// Files and Changes are shortcuts for evidence kinds files and diff;
	// they make the hand-back shape obvious to coding agents.
	Files   []string `json:"files"`
	Changes []string `json:"changes"`
	Reason  string   `json:"reason"`
	Source  string   `json:"source"`
	// Session names the agent session; it ties reports of one run together.
	Session string `json:"session"`
	// Percent is the agent's own estimate, 0 to 100, for status progress.
	Percent *int `json:"percent"`
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
	// Warning tells the agent the report was stored but something is off.
	Warning string `json:"warning,omitempty"`
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
	WorkflowVersion int `json:"workflow_version"`
	// Percent, Weight and DoneWeight are the project roll-up of the actual
	// command status; the same commands are never counted twice.
	Percent       int               `json:"percent"`
	Weight        int               `json:"weight"`
	DoneWeight    int               `json:"done_weight"`
	Overall       Rollup            `json:"overall"`
	Workflow      WorkflowProgress  `json:"workflow"`
	Modules       []ModuleProgress  `json:"modules"`
	Features      []FeatureProgress `json:"features"`
	StaleAfterSec int               `json:"stale_after_sec"`
	UpdatedAt     time.Time         `json:"updated_at"`
	LastReportAt  *time.Time        `json:"last_report_at,omitempty"`
	LastSource    string            `json:"last_source,omitempty"`
	Complete      bool              `json:"complete"`
	Counts        Counts            `json:"counts"`
	Overview      Overview          `json:"overview"`
	// Activity is what agents reported lately, newest first.
	Activity []Activity `json:"activity"`
	// Sessions are the agent sessions seen in the kept reports, latest first.
	Sessions []Session           `json:"sessions"`
	Waves    []WaveProgress      `json:"waves"`
	Commands []CommandProgress   `json:"commands"`
	Next     []string            `json:"next"`
	Warnings []Warning           `json:"warnings"`
	Issues   []string            `json:"issues"`
	History  []core.StatusChange `json:"history"`
	// Visual is the latest measured UI quality; it never changes command
	// status, but completion does not imply a visually verified product.
	Visual      core.VisualSummary `json:"visual"`
	VisualReady bool               `json:"visual_ready"`
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
	Revision    int `json:"revision"`
}

// Overview is the execution dashboard. Each command is in exactly one bucket,
// so the buckets add up to Total.
type Overview struct {
	Total      int `json:"total"`
	Completed  int `json:"completed"`
	InProgress int `json:"in_progress"`
	Pending    int `json:"pending"`
	Blocked    int `json:"blocked"`
	Failed     int `json:"failed"`
	Review     int `json:"review"`
	Revision   int `json:"revision"`
	Skipped    int `json:"skipped"`
	Stale      int `json:"stale"`
	Percent    int `json:"percent"`
}

// Activity is one report as the timeline shows it.
type Activity struct {
	At      time.Time `json:"at"`
	Command string    `json:"command"`
	Name    string    `json:"name,omitempty"`
	Status  string    `json:"status"`
	Applied string    `json:"applied,omitempty"`
	Outcome string    `json:"outcome"`
	Summary string    `json:"summary,omitempty"`
	Reason  string    `json:"reason,omitempty"`
	Files   []string  `json:"files,omitempty"`
	Percent *int      `json:"percent,omitempty"`
	Source  string    `json:"source,omitempty"`
	Session string    `json:"session,omitempty"`
}

// Session is one agent session and the command it touched last.
type Session struct {
	ID      string    `json:"id"`
	Source  string    `json:"source,omitempty"`
	Command string    `json:"command"`
	LastAt  time.Time `json:"last_at"`
	Active  bool      `json:"active"`
}

type WaveProgress struct {
	Index    int      `json:"index"`
	Commands []string `json:"commands"`
	Verified int      `json:"verified"`
	Total    int      `json:"total"`
}

type CommandProgress struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Status        string   `json:"status"`
	Phase         string   `json:"phase"`
	Stored        string   `json:"stored"`
	Revision      int      `json:"revision"`
	RunRevision   int      `json:"run_revision,omitempty"`
	Priority      string   `json:"priority,omitempty"`
	Complexity    string   `json:"complexity"`
	Files         []string `json:"files,omitempty"`
	Wave          int      `json:"wave"`
	Weight        int      `json:"weight"`
	DependsOn     []string `json:"depends_on"`
	Dependents    []string `json:"dependents"`
	FeatureIDs    []string `json:"feature_ids,omitempty"`
	SubFeatureIDs []string `json:"sub_feature_ids,omitempty"`
	Waiting       []string `json:"waiting,omitempty"`
	// Impact are the commands held back while this one is failed, blocked or
	// has to be redone: every direct and indirect dependent.
	Impact []string `json:"impact,omitempty"`
	// RevisionWhy says what changed since the work was done.
	RevisionWhy string `json:"revision_why,omitempty"`
	// StaleDocs are referenced artifacts that changed; the instruction may
	// no longer match the blueprint.
	StaleDocs  []string        `json:"stale_docs,omitempty"`
	Activity   string          `json:"activity,omitempty"`
	Percent    *int            `json:"percent,omitempty"`
	Session    string          `json:"session,omitempty"`
	StartedAt  *time.Time      `json:"started_at,omitempty"`
	FinishedAt *time.Time      `json:"finished_at,omitempty"`
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
	p.Workflow.EnsureID()
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

	evidence := mergeEvidence(in.Evidence, in.Files, in.Changes)
	rep := core.ProgressReport{
		ID: id, Seq: seq, Project: in.Project, WorkflowVersion: in.WorkflowVersion,
		Command: cmd.ID, Status: in.Status, StartedAt: utcPtr(in.StartedAt), FinishedAt: utcPtr(in.FinishedAt), UpdatedAt: updated,
		ReceivedAt: now, Summary: strings.TrimSpace(in.Summary), Reason: strings.TrimSpace(in.Reason),
		Evidence: evidence, Tests: slices.Clone(in.Tests), Source: strings.TrimSpace(in.Source),
		Session: strings.TrimSpace(in.Session), Percent: in.Percent, Outcome: "ignored",
	}
	out := &ApplyResult{Accepted: true, Command: cmd.ID, Status: run.Status, ReportID: id, Seq: seq}
	if late {
		rep.Outcome = "ignored"
		out.Ignored, out.Reason = true, reason
		appendReport(pr, rep)
		pr.UpdatedAt = now
		return out, nil
	}

	applied, why := decide(cmd, in, run, now)
	if in.Status == core.RunStarted && restarted(run.Status) {
		applied.StartedAt, applied.Revision = nil, 0 // another attempt, timed on its own
	}
	if in.StartedAt != nil {
		applied.StartedAt = utcPtr(in.StartedAt)
	} else if applied.Status == core.RunStarted && applied.StartedAt == nil {
		t := now
		applied.StartedAt = &t
	}
	if in.FinishedAt != nil {
		applied.FinishedAt = utcPtr(in.FinishedAt)
	} else if applied.Status == core.RunStarted {
		applied.FinishedAt = nil // a restarted command has no finish time yet
	} else if finished(applied.Status) && (applied.FinishedAt == nil || in.Status != core.RunProgress) {
		t := now
		applied.FinishedAt = &t
	}
	applied.CommandID = cmd.ID
	applied.UpdatedAt = updated
	applied.Source = strings.TrimSpace(in.Source)
	if sum := strings.TrimSpace(in.Summary); sum != "" || in.Status != core.RunProgress {
		applied.Summary = sum
	}
	if s := strings.TrimSpace(in.Session); s != "" {
		applied.Session = s
	}
	applied.Seq = seq
	applied.ReportID = id
	if applied.Revision == 0 {
		applied.Revision = core.CommandRevision(*cmd) // the revision this attempt works on
	}
	unmet := unmetDeps(p, cmd)
	if run.EarlyStart || (len(unmet) > 0 && (in.Status == core.RunStarted || in.Status == core.RunProgress || in.Status == core.RunDone || in.Status == core.RunReview)) {
		applied.EarlyStart = true
	}
	switch {
	case applied.Revision < core.CommandRevision(*cmd):
		out.Warning = fmt.Sprintf("%s sudah direvisi ke r%d; laporan ini berlaku untuk r%d. Baca ulang instruksi dan kerjakan ulang.", cmd.ID, core.CommandRevision(*cmd), applied.Revision)
	case len(unmet) > 0 && in.Status != core.RunFailed && in.Status != core.RunBlocked && in.Status != core.RunSkipped:
		out.Warning = cmd.ID + " dikerjakan sebelum dependency terverifikasi: " + strings.Join(unmet, ", ")
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
	if in.Percent != nil && (*in.Percent < 0 || *in.Percent > 100) {
		return fmt.Errorf("%w: percent harus 0 sampai 100", ErrInvalid)
	}
	if in.Status == core.RunSkipped && strings.TrimSpace(in.Reason) == "" {
		return fmt.Errorf("%w: command dilewati wajib menyertakan alasan", ErrInvalid)
	}
	if name := strings.TrimSpace(in.Project); name != "" && name != p.Name && name != p.Dir && name != filepath.Base(p.Dir) {
		return fmt.Errorf("%w: laporan untuk project %q, yang terbuka adalah %q", ErrInvalid, name, p.Name)
	}
	return nil
}

func decide(cmd *core.Command, in ReportIn, prev core.CommandRun, now time.Time) (core.CommandRun, string) {
	run := prev
	run.Reason = strings.TrimSpace(in.Reason)
	// One attempt collects evidence report by report; a new attempt starts
	// clean, so old evidence cannot verify work done again.
	if in.Status == core.RunStarted && restarted(prev.Status) {
		run.Evidence, run.Tests, run.Activity, run.Percent, run.ActivityAt = nil, nil, "", nil, nil
	}
	run.Evidence = mergeEvidence(append(slices.Clone(run.Evidence), in.Evidence...), in.Files, in.Changes)
	run.Tests = append(slices.Clone(run.Tests), in.Tests...)
	switch in.Status {
	case core.RunStarted:
		run.Status = core.RunStarted
		run.Reason = ""
		return run, ""
	case core.RunProgress:
		return reportActivity(run, prev, in, now)
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

// reportActivity keeps the note of a progress report. Work reported on an
// open command means it runs, whatever was said before; a finished command
// keeps its result, a late note does not reopen it.
func reportActivity(run, prev core.CommandRun, in ReportIn, now time.Time) (core.CommandRun, string) {
	if text := strings.TrimSpace(in.Summary); text != "" {
		run.Activity = text
	}
	if in.Percent != nil {
		pct := *in.Percent
		run.Percent = &pct
	}
	at := now
	run.ActivityAt = &at
	if finished(prev.Status) {
		run.Reason, run.Evidence, run.Tests = prev.Reason, prev.Evidence, prev.Tests
		return run, ""
	}
	run.Status, run.Reason = core.RunStarted, ""
	return run, ""
}

// restarted holds for a status a started report begins a new attempt from.
func restarted(st string) bool {
	return finished(st) || st == core.RunReview
}

func finished(st string) bool {
	switch st {
	case core.RunDone, core.RunVerified, core.RunFailed, core.RunSkipped:
		return true
	}
	return false
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

func mergeEvidence(ev []core.Evidence, files, changes []string) []core.Evidence {
	out := compactEvidence(ev)
	seen := map[string]bool{}
	for _, e := range out {
		seen[e.Kind+"\x00"+e.Ref] = true
	}
	add := func(kind string, refs []string) {
		for _, ref := range refs {
			ref = strings.TrimSpace(ref)
			if ref == "" || seen[kind+"\x00"+ref] {
				continue
			}
			seen[kind+"\x00"+ref] = true
			out = append(out, core.Evidence{Kind: kind, Ref: ref})
		}
	}
	add("files", files)
	add("diff", changes)
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
		var run core.CommandRun
		if pr != nil {
			if r, ok := pr.Commands[d]; ok && r.Status != "" {
				run, st = r, r.Status
			}
		}
		dep := p.Workflow.Command(d)
		if st != core.RunVerified || (dep != nil && outdated(run, *dep)) {
			out = append(out, d)
		}
	}
	return out
}

// outdated holds for a run done on an older revision of the command.
func outdated(run core.CommandRun, c core.Command) bool {
	return max(run.Revision, 1) < core.CommandRevision(c)
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
	return ViewStale(p, now, agent, nil)
}

// ViewStale is View knowing which commands have changed blueprint artifacts
// (workflow.Stale): finished work on those has to be looked at again.
func ViewStale(p *core.Project, now time.Time, agent Agent, stale map[string][]string) Snapshot {
	now = now.UTC()
	s := Snapshot{Waves: []WaveProgress{}, Commands: []CommandProgress{}, Next: []string{},
		Warnings: []Warning{}, Issues: []string{}, History: []core.StatusChange{},
		Activity: []Activity{}, Sessions: []Session{},
		StaleAfterSec: int(DefaultStaleAfter / time.Second)}
	if p == nil {
		return s
	}
	if p.Workflow == nil {
		s.Overall = rollupCommands(nil, nil)
		s.Percent, s.Weight, s.DoneWeight = s.Overall.Percent, s.Overall.Weight, s.Overall.DoneWeight
		s.Workflow = workflowProgress(p, nil, s.Overall)
		s.Modules, s.Features = featureRollups(p, nil)
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
		s.Activity, s.Sessions = activityOf(p, staleAfter(pr), now)
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
		cp := commandView(p, c, now, window, agent, waveOf[c.ID], stale[c.ID])
		s.Commands = append(s.Commands, cp)
		byID[c.ID] = cp
		addCount(&s.Counts, cp.Status)
		addOverview(&s.Overview, cp.Phase)
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
		case core.RunRevision:
			s.Warnings = append(s.Warnings, Warning{Command: c.ID, Kind: "revision",
				Message: "Perlu dikerjakan ulang: " + cp.RevisionWhy + ".",
				Action:  "Minta agen membaca ulang instruksi dan melaporkan hasil baru."})
		}
		if run := p.Progress; run != nil && cp.Stored == core.RunStarted && outdated(run.Commands[c.ID], c) {
			s.Warnings = append(s.Warnings, Warning{Command: c.ID, Kind: "revised",
				Message: fmt.Sprintf("%s direvisi ke r%d saat sedang dikerjakan pada r%d.", c.ID, cp.Revision, cp.RunRevision),
				Action:  "Minta agen membaca ulang instruksi sebelum melanjutkan."})
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
	impacts(s.Commands)
	for i := range s.Commands {
		c := &s.Commands[i]
		byID[c.ID] = *c
		if len(c.Impact) > 0 && c.Status != core.RunRevision {
			s.Warnings = append(s.Warnings, Warning{Command: c.ID, Kind: "impact",
				Message: fmt.Sprintf("Menahan %d command: %s.", len(c.Impact), idList(c.Impact)),
				Action:  "Selesaikan penyebabnya; command itu belum bisa dimulai."})
		}
	}
	sortNext(s.Next, byID)
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
	if p.HasUI() && p.UI != nil && p.UI.Layout != nil && len(p.UI.Layout.Pages) > 0 {
		s.Visual = p.Visual.Summary(p.UI.Layout.Pages)
		s.VisualReady = s.Visual.Pages > 0 && s.Visual.Fail == 0 && s.Visual.Untested == 0 && s.Visual.OpenHigh == 0
		implemented := s.Counts.Verified+s.Counts.Done > 0
		switch {
		case !implemented:
		case s.Visual.CheckedAt.IsZero():
			s.Warnings = append(s.Warnings, Warning{Kind: "visual_missing",
				Message: "UI diimplementasikan tetapi belum dievaluasi secara visual; kualitas tampilan belum terbukti.",
				Action:  "Kirim snapshot browser + screenshot untuk setiap halaman melalui evaluasi visual."})
		case s.Visual.OpenHigh > 0:
			s.Warnings = append(s.Warnings, Warning{Kind: "visual",
				Message: fmt.Sprintf("%d temuan visual berat pada %d halaman yang diuji.", s.Visual.OpenHigh, s.Visual.Pages-s.Visual.Untested),
				Action:  "Buat command perbaikan dari temuan, lalu kirim ulang snapshot setelah diperbaiki."})
		}
		if implemented && len(s.Visual.NotChecked) > 0 && !s.Visual.CheckedAt.IsZero() {
			s.Warnings = append(s.Warnings, Warning{Kind: "visual_unchecked",
				Message: "Halaman belum diuji secara visual: " + strings.Join(s.Visual.NotChecked, ", ") + ".",
				Action:  "Kirim snapshot halaman tersebut; halaman tanpa bukti tidak dianggap lulus."})
		}
	}
	s.Overall = rollupCommands(s.Commands, nil)
	s.Percent, s.Weight, s.DoneWeight = s.Overall.Percent, s.Overall.Weight, s.Overall.DoneWeight
	s.Overview.Percent = s.Percent
	s.Workflow = workflowProgress(p, s.Commands, s.Overall)
	s.Modules, s.Features = featureRollups(p, s.Commands)
	return s
}

func addOverview(o *Overview, phase string) {
	o.Total++
	switch phase {
	case core.PhaseCompleted:
		o.Completed++
	case core.PhaseInProgress, core.PhaseUpdating:
		o.InProgress++
	case core.PhaseBlocked:
		o.Blocked++
	case core.PhaseFailed:
		o.Failed++
	case core.PhaseReview:
		o.Review++
	case core.PhaseRevision:
		o.Revision++
	case core.PhaseSkipped:
		o.Skipped++
	case core.PhaseStale:
		o.Stale++
	default:
		o.Pending++
	}
}

// phaseOf is the lifecycle step of a command from its displayed status.
func phaseOf(status string, run core.CommandRun, now time.Time) string {
	switch status {
	case core.RunWaiting:
		return core.PhaseCreated
	case core.RunUnconfirmed:
		return core.PhaseReady
	case core.RunStarted:
		if run.ActivityAt != nil && now.Sub(*run.ActivityAt) <= updatingFor {
			return core.PhaseUpdating
		}
		return core.PhaseInProgress
	case core.RunVerified:
		return core.PhaseCompleted
	case core.RunDone, core.RunReview:
		return core.PhaseReview
	case core.RunBlocked:
		return core.PhaseBlocked
	case core.RunFailed:
		return core.PhaseFailed
	case core.RunRevision:
		return core.PhaseRevision
	case core.RunSkipped:
		return core.PhaseSkipped
	}
	return core.PhaseStale
}

// impacts names, on each command that holds work back, every command that
// cannot go on because of it.
func impacts(cmds []CommandProgress) {
	byID := make(map[string]CommandProgress, len(cmds))
	for _, c := range cmds {
		byID[c.ID] = c
	}
	for i := range cmds {
		c := &cmds[i]
		if c.Status != core.RunFailed && c.Status != core.RunBlocked && c.Status != core.RunRevision {
			continue
		}
		seen := map[string]bool{}
		var walk func(ids []string)
		walk = func(ids []string) {
			for _, id := range ids {
				if !seen[id] {
					seen[id] = true
					walk(byID[id].Dependents)
				}
			}
		}
		walk(c.Dependents)
		for _, other := range cmds { // keep workflow order
			if seen[other.ID] {
				c.Impact = append(c.Impact, other.ID)
			}
		}
	}
}

// sortNext orders the commands that may go on: what an agent may take by
// itself first, then by priority; the workflow order stays otherwise.
func sortNext(ids []string, byID map[string]CommandProgress) {
	rank := func(id string) [2]int {
		c := byID[id]
		first := 1
		if claimable(c) {
			first = 0
		}
		return [2]int{first, priorityRank(c.Priority)}
	}
	slices.SortStableFunc(ids, func(a, b string) int {
		x, y := rank(a), rank(b)
		for i := range x {
			if x[i] != y[i] {
				return x[i] - y[i]
			}
		}
		return 0
	})
}

func priorityRank(p string) int {
	switch p {
	case core.LevelHigh:
		return 0
	case core.LevelLow:
		return 2
	}
	return 1 // medium, or a command written before priorities existed
}

// activityOf is the recent reports, newest first, and the sessions they name.
func activityOf(p *core.Project, window time.Duration, now time.Time) ([]Activity, []Session) {
	acts, sessions := []Activity{}, []Session{}
	seen := map[string]bool{}
	reports := p.Progress.Reports
	for i := len(reports) - 1; i >= 0; i-- {
		r := reports[i]
		if id := r.Session; id != "" && !seen[id] {
			seen[id] = true
			sessions = append(sessions, Session{ID: id, Source: r.Source, Command: r.Command, LastAt: r.ReceivedAt, Active: now.Sub(r.ReceivedAt) <= window})
		}
		if len(acts) < maxActivity {
			acts = append(acts, activityFrom(p, r))
		}
	}
	return acts, sessions
}

func activityFrom(p *core.Project, r core.ProgressReport) Activity {
	a := Activity{At: r.ReceivedAt, Command: r.Command, Status: r.Status, Applied: r.AppliedStatus, Outcome: r.Outcome,
		Summary: r.Summary, Reason: r.Reason, Percent: r.Percent, Source: r.Source, Session: r.Session}
	if c := p.Workflow.Command(r.Command); c != nil {
		a.Name = c.Name
	}
	for _, e := range r.Evidence {
		if e.Kind == "files" {
			a.Files = append(a.Files, e.Ref)
		}
	}
	return a
}

func commandView(p *core.Project, c core.Command, now time.Time, window time.Duration, agent Agent, wave int, staleDocs []string) CommandProgress {
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
	why := ""
	if run.Status == core.RunVerified || run.Status == core.RunDone {
		switch {
		case outdated(run, c):
			status, why = core.RunRevision, fmt.Sprintf("instruksi berubah dari r%d ke r%d", max(run.Revision, 1), core.CommandRevision(c))
		case len(staleDocs) > 0:
			status, why = core.RunRevision, "artefak blueprint berubah: "+strings.Join(staleDocs, ", ")
		}
	}
	cp := CommandProgress{ID: c.ID, Name: c.Name, Status: status, Phase: phaseOf(status, run, now), Stored: stored, Wave: wave,
		Revision: core.CommandRevision(c), RunRevision: run.Revision, Priority: c.Priority, Complexity: core.CommandComplexity(c),
		Files: c.Files, RevisionWhy: why, StaleDocs: staleDocs, Activity: run.Activity, Percent: run.Percent, Session: run.Session,
		Weight: core.CommandWeight(c), DependsOn: append([]string{}, c.DependsOn...), Dependents: dependentsOf(p.Workflow, c.ID),
		FeatureIDs: core.ResolveCommandFeatureIDs(c, p.Features), SubFeatureIDs: core.ResolveCommandSubFeatureIDs(c, p.Features),
		Waiting: unmet, StartedAt: run.StartedAt, FinishedAt: run.FinishedAt, Source: run.Source, Summary: run.Summary,
		Reason: run.Reason, Evidence: run.Evidence, Tests: run.Tests, EarlyStart: run.EarlyStart,
		ReportID: run.ReportID, Seq: run.Seq}
	if cp.DependsOn == nil {
		cp.DependsOn = []string{}
	}
	if cp.Dependents == nil {
		cp.Dependents = []string{}
	}
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
	// The heartbeat is the MCP host's: it says nothing of an agent that
	// reports another way (CLI, API, file).
	if st == core.RunStarted && agent.Tracked && !agent.Alive && run.Source == agent.Source {
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
	case core.RunUnconfirmed, core.RunWaiting, core.RunFailed, core.RunBlocked, core.RunReview, core.RunStale, core.RunUnknown, core.RunDone, core.RunRevision:
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
	case core.RunRevision:
		c.Revision++
	}
}

// idList names a few commands; the rest of a long list only counts.
func idList(ids []string) string {
	const shown = 5
	if len(ids) <= shown {
		return strings.Join(ids, ", ")
	}
	return fmt.Sprintf("%s dan %d lagi", strings.Join(ids[:shown], ", "), len(ids)-shown)
}

func reasonSuffix(r string) string {
	if r == "" {
		return ""
	}
	return ": " + r
}

func dependentsOf(w *core.Workflow, id string) []string {
	if w == nil {
		return nil
	}
	var out []string
	for _, c := range w.Commands {
		if slices.Contains(c.DependsOn, id) {
			out = append(out, c.ID)
		}
	}
	return out
}
