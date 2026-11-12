package core

import (
	"fmt"
	"strings"
	"time"
)

// A blueprint run has three separate lifecycles, all persisted here and only
// here: a section is one part of the blueprint, a task is one step of a
// section, an execution is one real AI attempt behind a task. Events are the
// append-only timeline of those transitions. Counts are always derived from
// section and task state, never from events.

// Run statuses.
const (
	BPRunRunning    = "running"
	BPRunCompleted  = "completed"  // every applicable section is validated
	BPRunIncomplete = "incomplete" // the work ended with sections unfinished
	BPRunFailed     = "failed"
	BPRunStopped    = "stopped" // the user closed the run
)

// Section statuses.
const (
	BPPending    = "pending"
	BPInProgress = "in_progress"
	BPValidating = "validating"
	BPRepairing  = "repairing" // re-running with the refusal in the prompt
	BPRetrying   = "retrying"  // re-running the same work after a failure
	BPCompleted  = "completed"
	BPFailed     = "failed"
	BPBlocked    = "blocked" // a section this one depends on failed
	BPSkipped    = "skipped"
)

// Why a section is left out of the applicable set.
const (
	BPSkipNotApplicable    = "not_applicable"    // the product has no such concern
	BPSkipNotRequired      = "not_required"      // left out on purpose, with a reason
	BPSkipAlreadySatisfied = "already_satisfied" // valid output predates this run
	BPSkipDeferred         = "deferred"          // postponed by closing the run
)

// Task kinds and statuses.
const (
	BPTaskGenerate = "generate"
	BPTaskRetry    = "retry"
	BPTaskRepair   = "repair"
	BPTaskValidate = "validate"

	BPTaskPending   = "pending"
	BPTaskRunning   = "running"
	BPTaskCompleted = "completed"
	BPTaskFailed    = "failed"
	BPTaskCanceled  = "canceled"
)

// Execution triggers and statuses.
const (
	BPTriggerInitial      = "initial"
	BPTriggerContinuation = "continuation" // a further round of accepted work
	BPTriggerRepair       = "repair"       // the refusal goes back to the AI
	BPTriggerRetry        = "retry"        // the user or the pipeline runs it again
	BPTriggerBusyRetry    = "busy_retry"   // the AI server was overloaded

	BPExecRunning   = "running"
	BPExecSucceeded = "succeeded"
	BPExecRejected  = "rejected" // the AI answered; validation refused it
	BPExecFailed    = "failed"   // no usable answer came back
	BPExecCanceled  = "canceled"
)

// Failure kinds.
const (
	BPFailValidation  = "validation"
	BPFailOutput      = "invalid_output"
	BPFailOverloaded  = "overloaded"
	BPFailQuota       = "quota"
	BPFailAgent       = "agent"
	BPFailInterrupted = "interrupted"
)

// Validation statuses of a section's latest output.
const (
	BPValNone    = ""
	BPValRunning = "running"
	BPValPassed  = "passed"
	BPValFailed  = "failed"
)

// Activity types of the timeline.
const (
	EvRunStarted       = "run.started"
	EvRunResumed       = "run.resumed"
	EvRunCompleted     = "run.completed"
	EvRunIncomplete    = "run.incomplete"
	EvRunFailed        = "run.failed"
	EvRunStopped       = "run.stopped"
	EvPlanChanged      = "plan.changed"
	EvSectionStarted   = "section.started"
	EvSectionContinued = "section.continued"
	EvSectionRepairing = "section.repairing"
	EvSectionRetrying  = "section.retrying"
	EvSectionWaiting   = "section.waiting"
	EvSectionCompleted = "section.completed"
	EvSectionFailed    = "section.failed"
	EvSectionSkipped   = "section.skipped"
	EvSectionReopened  = "section.reopened"
	EvSectionBlocked   = "section.blocked"
	EvSectionUnblocked = "section.unblocked"
	EvValidationStart  = "validation.started"
	EvValidationPassed = "validation.passed"
	EvValidationFailed = "validation.failed"
)

const (
	maxBPEvents   = 512
	maxBPIssues   = 24
	maxCauseRunes = 140
)

// BlueprintProgress is the single source of truth of one generation run.
type BlueprintProgress struct {
	RunID     string     `json:"run_id"`
	Status    string     `json:"status"`
	Activity  string     `json:"activity,omitempty"`
	StartedAt time.Time  `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
	// Seq never goes down, across runs too: event order is one line per project.
	Seq   int64  `json:"seq"`
	Error string `json:"error,omitempty"`
	// Parallel is how many sections of one group may run at once (0: one).
	Parallel int `json:"parallel,omitempty"`
	// NextExecution numbers executions; an id is never reused in a run.
	NextExecution int                   `json:"next_execution"`
	PlanVersion   int                   `json:"plan_version"`
	PlanChanges   []BlueprintPlanChange `json:"plan_changes,omitempty"`
	Summary       BlueprintSummary      `json:"summary"`
	// ETA is computed for readers from job history; it is not stored.
	ETA      *BlueprintETA      `json:"eta,omitempty"`
	Sections []BlueprintSection `json:"sections"`
	Events   []BlueprintEvent   `json:"events,omitempty"`
}

// BlueprintSummary is every number the UI shows. Applicable always equals
// Completed + Active + Pending + Blocked + Failed, and Total equals
// Applicable + Skipped.
type BlueprintSummary struct {
	Total      int `json:"total"`
	Applicable int `json:"applicable"`
	Completed  int `json:"completed"`
	Active     int `json:"active"`
	Pending    int `json:"pending"`
	Blocked    int `json:"blocked"`
	Failed     int `json:"failed"`
	Skipped    int `json:"skipped"`
	// Percent is Completed/Applicable, and is 100 only for a completed run.
	Percent int `json:"percent"`

	TasksTotal     int `json:"tasks_total"`
	TasksCompleted int `json:"tasks_completed"`
	TasksActive    int `json:"tasks_active"`
	TasksFailed    int `json:"tasks_failed"`
	Executions     int `json:"executions"`
	Retries        int `json:"retries"`
}

// BlueprintETA is an estimate with a stated basis. UnknownSections counts
// remaining sections that have no duration history; RemainingMS leaves them out.
type BlueprintETA struct {
	RemainingMS     int64 `json:"remaining_ms"`
	Samples         int   `json:"samples"`
	UnknownSections int   `json:"unknown_sections,omitempty"`
}

// BlueprintPlanChange records one change of the applicable denominator.
type BlueprintPlanChange struct {
	Version int       `json:"version"`
	From    int       `json:"from"`
	To      int       `json:"to"`
	Reason  string    `json:"reason"`
	At      time.Time `json:"at"`
}

// BlueprintSection is one part of the blueprint.
type BlueprintSection struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Kind      string   `json:"kind,omitempty"` // the AI job kind that writes it
	Status    string   `json:"status"`
	DependsOn []string `json:"depends_on,omitempty"`
	Activity  string   `json:"activity,omitempty"`
	// SkipKind and Reason explain a skipped section; Reason also explains
	// a blocked one and what a pending one waits for.
	SkipKind    string   `json:"skip_kind,omitempty"`
	Reason      string   `json:"reason,omitempty"`
	Error       string   `json:"error,omitempty"`
	FailureKind string   `json:"failure_kind,omitempty"`
	Attempt     int      `json:"attempt,omitempty"`
	Validation  string   `json:"validation,omitempty"`
	Issues      []string `json:"validation_issues,omitempty"`

	StartedAt  *time.Time           `json:"started_at,omitempty"`
	EndedAt    *time.Time           `json:"ended_at,omitempty"`
	Tasks      []BlueprintTask      `json:"tasks,omitempty"`
	Executions []BlueprintExecution `json:"executions,omitempty"`
}

// BlueprintTask is one step of a section. Tasks are created as the work
// needs them: a refused output adds a repair task and a new validation.
type BlueprintTask struct {
	ID          string     `json:"id"`
	Kind        string     `json:"kind"`
	Title       string     `json:"title"`
	Status      string     `json:"status"`
	Attempt     int        `json:"attempt"`
	ExecutionID string     `json:"execution_id,omitempty"`
	DependsOn   []string   `json:"depends_on,omitempty"`
	Error       string     `json:"error,omitempty"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	EndedAt     *time.Time `json:"ended_at,omitempty"`
}

// BlueprintExecution is one real AI attempt.
type BlueprintExecution struct {
	ID          string     `json:"id"`
	TaskID      string     `json:"task_id"`
	Attempt     int        `json:"attempt"`
	Trigger     string     `json:"trigger"`
	Status      string     `json:"status"`
	Cause       string     `json:"cause,omitempty"` // why this attempt was needed
	Error       string     `json:"error,omitempty"` // why it ended badly
	FailureKind string     `json:"failure_kind,omitempty"`
	StartedAt   time.Time  `json:"started_at"`
	EndedAt     *time.Time `json:"ended_at,omitempty"`
	DurationMS  int64      `json:"duration_ms,omitempty"`
}

// BlueprintEvent is one transition of the timeline. Two events with the same
// text are different events: identity is EventID, order is Seq.
type BlueprintEvent struct {
	EventID      string    `json:"event_id"`
	RunID        string    `json:"run_id"`
	SectionID    string    `json:"section_id,omitempty"`
	SectionName  string    `json:"section_name,omitempty"`
	TaskID       string    `json:"task_id"`
	ExecutionID  string    `json:"execution_id"`
	Seq          int64     `json:"sequence_number"`
	Timestamp    time.Time `json:"timestamp"`
	ActivityType string    `json:"activity_type"`
	Status       string    `json:"status"`
	Attempt      int       `json:"attempt_number"`
	Message      string    `json:"message,omitempty"`
	Detail       string    `json:"detail,omitempty"`
}

// NewBlueprintProgress starts a run. baseSeq is the last seq of the project's
// previous run, so a reader's cursor stays valid across runs.
func NewBlueprintProgress(id string, sections []BlueprintSection, baseSeq int64, at time.Time) *BlueprintProgress {
	t := &BlueprintProgress{RunID: id, Status: BPRunRunning, StartedAt: at, Seq: baseSeq, PlanVersion: 1, Sections: sections}
	t.emit(bpNote{typ: EvRunStarted, status: BPRunRunning, msg: "Merancang Blueprint"}, at)
	return t
}

// After returns events with seq greater than after, in order.
func (t *BlueprintProgress) After(after int64) []BlueprintEvent {
	if t == nil {
		return nil
	}
	var out []BlueprintEvent
	for _, e := range t.Events {
		if e.Seq > after {
			out = append(out, e)
		}
	}
	return out
}

// Section returns the named section, or nil.
func (t *BlueprintProgress) Section(id string) *BlueprintSection {
	if t == nil || id == "" {
		return nil
	}
	for i := range t.Sections {
		if t.Sections[i].ID == id {
			return &t.Sections[i]
		}
	}
	return nil
}

// Active reports work in flight on the section.
func (s *BlueprintSection) Active() bool {
	switch s.Status {
	case BPInProgress, BPValidating, BPRepairing, BPRetrying:
		return true
	}
	return false
}

// Running reports whether any section has work in flight.
func (t *BlueprintProgress) Running() bool {
	if t == nil {
		return false
	}
	for i := range t.Sections {
		if t.Sections[i].Active() {
			return true
		}
	}
	return false
}

// ResumeRun reopens an ended run without dropping its finished work.
func (t *BlueprintProgress) ResumeRun(at time.Time) {
	if t == nil || t.Status == BPRunRunning {
		return
	}
	t.Status, t.Error, t.EndedAt = BPRunRunning, "", nil
	t.emit(bpNote{typ: EvRunResumed, status: BPRunRunning, msg: "Melanjutkan bagian yang belum selesai"}, at)
}

// ReconcilePlan folds the current project truth into the section list. It is
// the only place the applicable denominator may change, and every change is
// recorded. Items carry pending, completed or skipped (with kind and reason).
func (t *BlueprintProgress) ReconcilePlan(items []BlueprintSection, at time.Time) {
	if t == nil {
		return
	}
	before := t.Summary.Applicable
	prev := make(map[string]BlueprintSection, len(t.Sections))
	source := ""
	for _, s := range t.Sections {
		prev[s.ID] = s
		if s.Status == BPValidating && source == "" {
			source = s.Name
		}
	}
	var changes []string
	next := make([]BlueprintSection, 0, len(items))
	known := make(map[string]bool, len(items))
	for _, it := range items {
		known[it.ID] = true
		s, ok := prev[it.ID]
		if !ok {
			s = it
			s.Status = BPPending
			s.SkipKind = ""
			changes = append(changes, it.Name+" ditambahkan")
		}
		s.Name, s.Kind, s.DependsOn = it.Name, it.Kind, it.DependsOn
		if s.Status == BPPending {
			s.Reason = it.Reason
		}
		next = append(next, s)
	}
	for _, s := range t.Sections {
		if !known[s.ID] {
			changes = append(changes, s.Name+" dihapus dari rencana")
		}
	}
	t.Sections = next

	for _, it := range items {
		s := t.Section(it.ID)
		switch it.Status {
		case BPSkipped:
			if s.Status == BPInProgress || s.Status == BPRepairing || s.Status == BPRetrying {
				continue // an attempt in flight decides this section itself
			}
			if s.Status == BPSkipped && s.SkipKind == it.SkipKind {
				s.Reason = it.Reason
				continue
			}
			t.SkipSection(it.ID, it.SkipKind, it.Reason, at)
			changes = append(changes, it.Name+" dilewati: "+it.Reason)
		case BPCompleted:
			if s.Status == BPCompleted || s.SkipKind == BPSkipAlreadySatisfied ||
				s.Status == BPInProgress || s.Status == BPRepairing || s.Status == BPRetrying {
				continue
			}
			if s.Status == BPSkipped {
				t.ReopenSection(it.ID, "Bagian ini ternyata sudah terisi", at)
				changes = append(changes, it.Name+" kembali relevan")
			}
			t.CompleteSection(it.ID, source, at)
		default:
			if s.Status == BPCompleted || s.Status == BPSkipped {
				wasSkipped := s.Status == BPSkipped
				t.ReopenSection(it.ID, "Isinya tidak lagi berlaku dan perlu dikerjakan lagi", at)
				if wasSkipped {
					changes = append(changes, it.Name+" kembali relevan")
				}
			}
		}
	}
	t.refreshBlocked(at)
	t.recordPlanChange(before, strings.Join(changes, "; "), at)
}

// recordPlanChange logs a move of the denominator. Every path that can move
// it ends here, so the number never changes without a recorded reason.
func (t *BlueprintProgress) recordPlanChange(before int, reason string, at time.Time) {
	t.summarize()
	after := t.Summary.Applicable
	if after == before {
		return
	}
	t.PlanVersion++
	t.PlanChanges = append(t.PlanChanges, BlueprintPlanChange{Version: t.PlanVersion, From: before, To: after, Reason: reason, At: at})
	t.emit(bpNote{typ: EvPlanChanged, status: t.Status,
		msg: fmt.Sprintf("Rencana berubah: %d menjadi %d bagian relevan", before, after), detail: reason}, at)
}

// BeginExecution opens a new AI attempt on the section and returns its id.
// Every attempt gets its own execution id and task. cause says why it runs.
func (t *BlueprintProgress) BeginExecution(id, trigger, activity, cause string, at time.Time) string {
	s := t.Section(id)
	if s == nil || s.Status == BPCompleted || s.Status == BPSkipped {
		return ""
	}
	t.closeExecution(s, BPExecCanceled, "", "", at)
	s.Attempt++
	t.NextExecution++
	exec := fmt.Sprintf("exec-%d", t.NextExecution)

	kind, title, status, typ := BPTaskGenerate, "Menghasilkan "+s.Name, BPInProgress, EvSectionStarted
	switch trigger {
	case BPTriggerContinuation:
		title, typ = "Melengkapi "+s.Name, EvSectionContinued
	case BPTriggerRepair:
		kind, title, status, typ = BPTaskRepair, "Memperbaiki output: "+ShortCause(cause), BPRepairing, EvSectionRepairing
	case BPTriggerRetry, BPTriggerBusyRetry:
		kind, title, status, typ = BPTaskRetry, "Mengulang pembuatan "+s.Name, BPRetrying, EvSectionRetrying
	}
	s.Status = status
	s.SkipKind, s.Reason = "", ""
	s.Error, s.FailureKind = "", ""
	s.Validation, s.Issues = BPValNone, nil
	s.EndedAt = nil
	if s.StartedAt == nil {
		s.StartedAt = bpTime(at)
	}
	workID := fmt.Sprintf("%s.%s.%d", id, kind, s.Attempt)
	validateID := fmt.Sprintf("%s.%s.%d", id, BPTaskValidate, s.Attempt)
	s.Tasks = append(s.Tasks,
		BlueprintTask{ID: workID, Kind: kind, Title: title, Status: BPTaskRunning, Attempt: s.Attempt, ExecutionID: exec, StartedAt: bpTime(at)},
		BlueprintTask{ID: validateID, Kind: BPTaskValidate, Title: "Memvalidasi hasil", Status: BPTaskPending, Attempt: s.Attempt,
			ExecutionID: exec, DependsOn: []string{workID}})
	s.Executions = append(s.Executions, BlueprintExecution{
		ID: exec, TaskID: workID, Attempt: s.Attempt, Trigger: trigger, Status: BPExecRunning, Cause: cause, StartedAt: at,
	})
	t.refreshBlocked(at)
	t.emit(bpNote{typ: typ, section: id, task: workID, exec: exec, status: status, attempt: s.Attempt, msg: activity, detail: cause}, at)
	return exec
}

// BeginValidation marks the AI answer as received and under validation.
func (t *BlueprintProgress) BeginValidation(id, activity string, at time.Time) {
	s := t.Section(id)
	if s == nil {
		return
	}
	e := s.openExecution()
	if e == nil {
		return
	}
	s.endTask(e.TaskID, BPTaskCompleted, "", at)
	v := s.validateTask(e.ID)
	if v != nil {
		v.Status, v.StartedAt = BPTaskRunning, bpTime(at)
	}
	s.Status, s.Validation = BPValidating, BPValRunning
	t.emit(bpNote{typ: EvValidationStart, section: id, task: taskID(v), exec: e.ID, status: BPValidating, attempt: e.Attempt, msg: activity}, at)
}

// AcceptPartial closes a valid attempt whose section still needs another
// round (a design without page details). Nothing counts as completed.
func (t *BlueprintProgress) AcceptPartial(id, activity string, at time.Time) {
	s := t.Section(id)
	if s == nil || s.Status != BPValidating {
		return
	}
	e := s.openExecution()
	if e == nil {
		return
	}
	execID, attempt, tid := e.ID, e.Attempt, e.TaskID
	t.closeExecution(s, BPExecSucceeded, "", "", at)
	s.Status, s.Validation = BPInProgress, BPValNone
	t.emit(bpNote{typ: EvValidationPassed, section: id, task: tid, exec: execID, status: BPInProgress, attempt: attempt, msg: activity}, at)
}

// RejectOutput records that validation refused the AI answer. The section's
// earlier valid output, and every other section, stays as it is. It waits as
// repairing: the next attempt opens its own execution, and FailSection ends
// it when no attempt is left, so a refusal alone blocks nothing downstream.
func (t *BlueprintProgress) RejectOutput(id, failureKind, msg string, at time.Time) {
	s := t.Section(id)
	if s == nil || s.Status == BPCompleted || s.Status == BPSkipped {
		return
	}
	e := s.openExecution()
	if e == nil {
		return
	}
	execID, attempt := e.ID, e.Attempt
	v := s.validateTask(execID)
	vid := taskID(v)
	t.closeExecution(s, BPExecRejected, failureKind, msg, at)
	s.Status, s.Validation, s.Issues = BPRepairing, BPValFailed, issueLines(msg)
	s.Error, s.FailureKind = msg, failureKind
	t.emit(bpNote{typ: EvValidationFailed, section: id, task: vid, exec: execID, status: BPRepairing, attempt: attempt,
		msg: fmt.Sprintf("Output %s ditolak validasi (percobaan %d): %s", s.Name, attempt, ShortCause(msg)), detail: msg}, at)
}

// WaitRetry ends an attempt the AI server could not serve; the section waits
// for the pipeline's next attempt, which opens its own execution.
func (t *BlueprintProgress) WaitRetry(id, msg string, at time.Time) {
	s := t.Section(id)
	if s == nil || s.Status == BPCompleted || s.Status == BPSkipped {
		return
	}
	e := s.openExecution()
	if e == nil {
		return
	}
	execID, attempt, tid := e.ID, e.Attempt, e.TaskID
	t.closeExecution(s, BPExecFailed, BPFailOverloaded, msg, at)
	s.Status, s.Error, s.FailureKind = BPRetrying, msg, BPFailOverloaded
	t.emit(bpNote{typ: EvSectionWaiting, section: id, task: tid, exec: execID, status: BPRetrying, attempt: attempt, msg: msg}, at)
}

// CompleteSection is the only way into completed, and is reached only after
// the output landed in the project and passed validation. source names the
// section whose answer also produced this one, when it had no attempt itself.
func (t *BlueprintProgress) CompleteSection(id, source string, at time.Time) {
	s := t.Section(id)
	if s == nil || s.Status == BPCompleted {
		return
	}
	execID, tid := "", ""
	if e := s.openExecution(); e != nil {
		execID, tid = e.ID, taskID(s.validateTask(e.ID))
		t.closeExecution(s, BPExecSucceeded, "", "", at)
	} else {
		title := "Tervalidasi dari data project"
		if source != "" {
			title = "Tervalidasi dari output " + source
		}
		s.Attempt = max(s.Attempt, 1)
		tid = fmt.Sprintf("%s.%s.%d", id, BPTaskValidate, s.Attempt)
		s.Tasks = append(s.Tasks, BlueprintTask{ID: tid, Kind: BPTaskValidate, Title: title,
			Status: BPTaskCompleted, Attempt: s.Attempt, StartedAt: bpTime(at), EndedAt: bpTime(at)})
	}
	s.Status, s.Validation, s.Issues = BPCompleted, BPValPassed, nil
	s.Error, s.FailureKind, s.Reason, s.SkipKind = "", "", "", ""
	s.EndedAt = bpTime(at)
	if s.StartedAt == nil {
		s.StartedAt = bpTime(at)
	}
	t.refreshBlocked(at)
	t.emit(bpNote{typ: EvSectionCompleted, section: id, task: tid, exec: execID, status: BPCompleted, attempt: s.Attempt, msg: s.Name + " selesai dan tervalidasi"}, at)
}

// SkipSection leaves a section out. A reason is required: the user must be
// able to read why.
func (t *BlueprintProgress) SkipSection(id, kind, reason string, at time.Time) {
	s := t.Section(id)
	if s == nil || (s.Status == BPSkipped && s.SkipKind == kind) {
		return
	}
	if s.openExecution() != nil {
		t.closeExecution(s, BPExecSucceeded, "", "", at)
	}
	s.Status, s.SkipKind, s.Reason = BPSkipped, kind, reason
	s.Error, s.FailureKind = "", ""
	s.Validation, s.Issues = BPValNone, nil
	s.EndedAt = bpTime(at)
	t.refreshBlocked(at)
	t.emit(bpNote{typ: EvSectionSkipped, section: id, status: BPSkipped, msg: s.Name + " dilewati: " + reason}, at)
}

// ReopenSection returns a finished section to pending: its output no longer
// holds. It is out of the completed count from here on.
func (t *BlueprintProgress) ReopenSection(id, reason string, at time.Time) {
	s := t.Section(id)
	if s == nil || (s.Status != BPCompleted && s.Status != BPSkipped) {
		return
	}
	s.Status, s.SkipKind, s.Reason = BPPending, "", ""
	s.Validation, s.Issues = BPValNone, nil
	s.EndedAt = nil
	t.emit(bpNote{typ: EvSectionReopened, section: id, status: BPPending, msg: s.Name + " dibuka kembali: " + reason}, at)
}

// ReworkSection brings a finished section back because the pipeline is about
// to work on it again.
func (t *BlueprintProgress) ReworkSection(id, reason string, at time.Time) {
	if t.Section(id) == nil {
		return
	}
	before := t.Summary.Applicable
	t.ReopenSection(id, reason, at)
	t.recordPlanChange(before, t.Section(id).Name+" dikerjakan ulang", at)
}

// FailSection records why the section stopped. Finished siblings stay.
func (t *BlueprintProgress) FailSection(id, failureKind, msg string, at time.Time) {
	s := t.Section(id)
	if s == nil || s.Status == BPCompleted || s.Status == BPSkipped {
		return
	}
	if s.Status == BPFailed {
		return
	}
	execID, tid, attempt := "", "", s.Attempt
	if e := s.openExecution(); e != nil {
		execID, tid, attempt = e.ID, e.TaskID, e.Attempt
		t.closeExecution(s, BPExecFailed, failureKind, msg, at)
	}
	s.Status, s.Error, s.FailureKind = BPFailed, msg, failureKind
	s.EndedAt = bpTime(at)
	t.refreshBlocked(at)
	t.emit(bpNote{typ: EvSectionFailed, section: id, task: tid, exec: execID, status: BPFailed, attempt: attempt,
		msg: s.Name + " gagal: " + ShortCause(msg), detail: msg}, at)
}

// CompleteRun ends the run. Only a run whose every applicable section is
// completed is completed; anything else is incomplete and says so.
func (t *BlueprintProgress) CompleteRun(at time.Time) {
	if t == nil || t.Status != BPRunRunning {
		return
	}
	t.refreshBlocked(at)
	t.summarize()
	sum := t.Summary
	t.EndedAt = bpTime(at)
	if sum.Completed == sum.Applicable {
		t.Status = BPRunCompleted
		t.emit(bpNote{typ: EvRunCompleted, status: BPRunCompleted,
			msg: fmt.Sprintf("Blueprint selesai: %d/%d bagian tervalidasi", sum.Completed, sum.Applicable)}, at)
		return
	}
	t.Status = BPRunIncomplete
	t.Error = fmt.Sprintf("%d dari %d bagian belum selesai", sum.Applicable-sum.Completed, sum.Applicable)
	t.emit(bpNote{typ: EvRunIncomplete, status: BPRunIncomplete, msg: "Blueprint belum lengkap: " + t.Error}, at)
}

// FailRun ends the run as failed. Finished sections are kept; work still in
// flight has no owner any more and is closed as interrupted.
func (t *BlueprintProgress) FailRun(msg string, at time.Time) {
	if t == nil || t.Status != BPRunRunning {
		return
	}
	for i := range t.Sections {
		if s := &t.Sections[i]; s.Active() {
			t.FailSection(s.ID, BPFailInterrupted, msg, at)
		}
	}
	t.Status, t.Error, t.EndedAt = BPRunFailed, msg, bpTime(at)
	t.emit(bpNote{typ: EvRunFailed, status: BPRunFailed, msg: "Perancangan Blueprint berhenti: " + ShortCause(msg), detail: msg}, at)
}

// StopRun closes the run on the user's request. Sections not reached are
// deferred, with that reason; failed ones stay failed.
func (t *BlueprintProgress) StopRun(reason string, at time.Time) {
	if t == nil || t.Status == BPRunCompleted || t.Status == BPRunStopped {
		return
	}
	before := t.Summary.Applicable
	for i := range t.Sections {
		s := &t.Sections[i]
		if s.Status == BPPending || s.Status == BPBlocked {
			t.SkipSection(s.ID, BPSkipDeferred, reason, at)
		}
	}
	t.recordPlanChange(before, reason, at)
	t.Status, t.EndedAt = BPRunStopped, bpTime(at)
	t.emit(bpNote{typ: EvRunStopped, status: BPRunStopped, msg: "Perancangan Blueprint ditutup: " + reason}, at)
}

// Estimate is the time left, from the median real duration of each remaining
// section's job kind. nil when no remaining section has a basis, the run is
// not running, or a person drives the pace (MCP host).
func (t *BlueprintProgress) Estimate(history []JobTiming, hostDriven bool, now time.Time) *BlueprintETA {
	if t == nil || t.Status != BPRunRunning || hostDriven {
		return nil
	}
	used := map[string]bool{}
	var serial, grouped int64
	eta := BlueprintETA{}
	for i := range t.Sections {
		s := &t.Sections[i]
		if !s.Active() && s.Status != BPPending && s.Status != BPBlocked {
			continue
		}
		est := EstimateJobMS(history, s.Kind)
		if est == 0 {
			eta.UnknownSections++
			continue
		}
		used[s.Kind] = true
		rem := est
		if e := s.openExecution(); e != nil {
			rem = max(est-now.Sub(e.StartedAt).Milliseconds(), 0)
		}
		if s.parallelKind() && t.Parallel > 1 {
			grouped += rem
		} else {
			serial += rem
		}
	}
	if len(used) == 0 {
		return nil
	}
	if t.Parallel > 1 {
		grouped /= int64(t.Parallel)
	}
	eta.RemainingMS = serial + grouped
	if eta.RemainingMS == 0 {
		return nil // the work outran its history: there is no honest number
	}
	for _, h := range history {
		if used[h.Kind] {
			eta.Samples++
		}
	}
	return &eta
}

func (s *BlueprintSection) parallelKind() bool {
	a := AreaByID(s.ID)
	return a != nil && !a.Builtin
}

func (s *BlueprintSection) openExecution() *BlueprintExecution {
	for i := len(s.Executions) - 1; i >= 0; i-- {
		if s.Executions[i].Status == BPExecRunning {
			return &s.Executions[i]
		}
	}
	return nil
}

func (s *BlueprintSection) validateTask(execID string) *BlueprintTask {
	for i := len(s.Tasks) - 1; i >= 0; i-- {
		if s.Tasks[i].Kind == BPTaskValidate && s.Tasks[i].ExecutionID == execID {
			return &s.Tasks[i]
		}
	}
	return nil
}

func (s *BlueprintSection) endTask(id, status, errMsg string, at time.Time) {
	for i := range s.Tasks {
		task := &s.Tasks[i]
		if task.ID != id || (task.Status != BPTaskRunning && task.Status != BPTaskPending) {
			continue
		}
		task.Status, task.Error, task.EndedAt = status, errMsg, bpTime(at)
		return
	}
}

// closeExecution ends the open attempt and settles its tasks to match.
func (t *BlueprintProgress) closeExecution(s *BlueprintSection, status, failureKind, errMsg string, at time.Time) {
	e := s.openExecution()
	if e == nil {
		return
	}
	e.Status, e.FailureKind, e.Error = status, failureKind, errMsg
	e.EndedAt = bpTime(at)
	e.DurationMS = max(at.Sub(e.StartedAt).Milliseconds(), 0)
	v := s.validateTask(e.ID)
	switch status {
	case BPExecSucceeded:
		s.endTask(e.TaskID, BPTaskCompleted, "", at)
		if v != nil {
			s.endTask(v.ID, BPTaskCompleted, "", at)
		}
	case BPExecRejected:
		s.endTask(e.TaskID, BPTaskCompleted, "", at)
		if v != nil {
			s.endTask(v.ID, BPTaskFailed, errMsg, at)
		}
	case BPExecFailed:
		s.endTask(e.TaskID, BPTaskFailed, errMsg, at)
		if v != nil {
			s.endTask(v.ID, BPTaskCanceled, "", at)
		}
	default:
		s.endTask(e.TaskID, BPTaskCanceled, "", at)
		if v != nil {
			s.endTask(v.ID, BPTaskCanceled, "", at)
		}
	}
}

// refreshBlocked derives blocked from dependencies. Sections are ordered so a
// dependency comes first, and blocked travels down the chain in one pass.
func (t *BlueprintProgress) refreshBlocked(at time.Time) {
	for i := range t.Sections {
		s := &t.Sections[i]
		if s.Status != BPPending && s.Status != BPBlocked {
			continue
		}
		by := ""
		for _, id := range s.DependsOn {
			if d := t.Section(id); d != nil && (d.Status == BPFailed || d.Status == BPBlocked) {
				by = d.Name
				break
			}
		}
		switch {
		case by != "" && (s.Status != BPBlocked || s.Reason != "Menunggu "+by):
			s.Status, s.Reason = BPBlocked, "Menunggu "+by
			t.emit(bpNote{typ: EvSectionBlocked, section: s.ID, status: BPBlocked, msg: s.Name + " terhambat: menunggu " + by}, at)
		case by == "" && s.Status == BPBlocked:
			s.Status, s.Reason = BPPending, ""
			t.emit(bpNote{typ: EvSectionUnblocked, section: s.ID, status: BPPending, msg: s.Name + " tidak lagi terhambat"}, at)
		}
	}
}

func (t *BlueprintProgress) summarize() {
	var sum BlueprintSummary
	for i := range t.Sections {
		s := &t.Sections[i]
		sum.Total++
		switch {
		case s.Status == BPSkipped:
			sum.Skipped++
		case s.Status == BPCompleted:
			sum.Completed++
		case s.Status == BPFailed:
			sum.Failed++
		case s.Status == BPBlocked:
			sum.Blocked++
		case s.Active():
			sum.Active++
		default:
			sum.Pending++
		}
		for _, task := range s.Tasks {
			sum.TasksTotal++
			switch task.Status {
			case BPTaskCompleted:
				sum.TasksCompleted++
			case BPTaskRunning:
				sum.TasksActive++
			case BPTaskFailed:
				sum.TasksFailed++
			}
		}
		sum.Executions += len(s.Executions)
		for _, e := range s.Executions {
			if e.Attempt > 1 {
				sum.Retries++
			}
		}
	}
	sum.Applicable = sum.Total - sum.Skipped
	if sum.Applicable > 0 {
		sum.Percent = sum.Completed * 100 / sum.Applicable
	}
	if sum.Percent == 100 && t.Status != BPRunCompleted {
		sum.Percent = 99 // validated sections, but the run has not closed
	}
	t.Summary = sum
}

type bpNote struct {
	typ, section, task, exec, status, msg, detail string
	attempt                                       int
}

func (t *BlueprintProgress) emit(n bpNote, at time.Time) {
	if t == nil {
		return
	}
	t.Seq++
	name := ""
	if s := t.Section(n.section); s != nil {
		name = s.Name
		if n.msg != "" {
			s.Activity = n.msg
		}
	}
	if n.msg != "" {
		t.Activity = n.msg
	}
	t.summarize()
	t.Events = append(t.Events, BlueprintEvent{
		EventID: fmt.Sprintf("%s-%d", t.RunID, t.Seq), RunID: t.RunID, SectionID: n.section, SectionName: name,
		TaskID: n.task, ExecutionID: n.exec, Seq: t.Seq, Timestamp: at, ActivityType: n.typ,
		Status: n.status, Attempt: n.attempt, Message: n.msg, Detail: n.detail,
	})
	if len(t.Events) > maxBPEvents {
		t.Events = t.Events[len(t.Events)-maxBPEvents:]
	}
}

func bpTime(at time.Time) *time.Time { return &at }

func taskID(t *BlueprintTask) string {
	if t == nil {
		return ""
	}
	return t.ID
}

// ShortCause is the first line of a failure, for one-line labels.
func ShortCause(msg string) string {
	line := strings.TrimSpace(msg)
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = strings.TrimSpace(line[:i])
	}
	if r := []rune(line); len(r) > maxCauseRunes {
		return string(r[:maxCauseRunes]) + "…"
	}
	return line
}

func issueLines(msg string) []string {
	var out []string
	for _, l := range strings.Split(msg, "\n") {
		if l = strings.TrimSpace(l); l != "" && len(out) < maxBPIssues {
			out = append(out, l)
		}
	}
	return out
}
