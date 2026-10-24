package core

import "time"

// Execution statuses reported by an external coding agent or derived by
// Foundry while tracking that agent. Distinct from Command.Status, which
// is a design state of the instruction itself.
const (
	RunUnconfirmed = "unconfirmed" // no report yet; never treated as done
	RunWaiting     = "waiting"     // view only: unmet dependencies
	RunStarted     = "started"
	RunBlocked     = "blocked"
	RunSkipped     = "skipped" // recorded with a reason; not done
	RunFailed      = "failed"
	RunReview      = "review"   // needs review (missing evidence, failed tests, or agent asked)
	RunDone        = "done"     // agent claims done; not yet verified
	RunVerified    = "verified" // Foundry accepted evidence against the command
	RunStale       = "stale"    // view only: last report older than the configured window
	RunUnknown     = "unknown"  // view only: agent disconnected; last status kept
)

// AgentReportStatuses are the values an external agent may send.
var AgentReportStatuses = []string{RunStarted, RunDone, RunFailed, RunBlocked, RunReview, RunSkipped}

// Progress is the persistent execution record of the command workflow.
// Foundry stores and validates it; it never runs the commands.
type Progress struct {
	WorkflowVersion int                   `json:"workflow_version"`
	StaleAfterSec   int                   `json:"stale_after_sec,omitempty"` // 0 = package default
	Seq             int64                 `json:"seq"`
	Commands        map[string]CommandRun `json:"commands"`
	// Retired keeps the last run of commands removed from the workflow.
	Retired   map[string]CommandRun `json:"retired,omitempty"`
	Reports   []ProgressReport      `json:"reports"`
	History   []StatusChange        `json:"history"`
	UpdatedAt time.Time             `json:"updated_at"`
}

// CommandRun is the current execution state of one command.
type CommandRun struct {
	CommandID  string     `json:"command"`
	Status     string     `json:"status"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	UpdatedAt  time.Time  `json:"updated_at"`
	Source     string     `json:"source,omitempty"`
	Summary    string     `json:"summary,omitempty"`
	Reason     string     `json:"reason,omitempty"`
	Evidence   []Evidence `json:"evidence,omitempty"`
	Tests      []TestRun  `json:"tests,omitempty"`
	Seq        int64      `json:"seq"`
	ReportID   string     `json:"report_id,omitempty"`
	EarlyStart bool       `json:"early_start,omitempty"`
}

// ProgressReport is one agent report, kept for idempotency and audit.
type ProgressReport struct {
	ID              string     `json:"id"`
	Seq             int64      `json:"seq"`
	Project         string     `json:"project,omitempty"`
	WorkflowVersion int        `json:"workflow_version,omitempty"`
	Command         string     `json:"command"`
	Status          string     `json:"status"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	UpdatedAt       time.Time  `json:"updated_at"`
	ReceivedAt      time.Time  `json:"received_at"`
	Summary         string     `json:"summary,omitempty"`
	Reason          string     `json:"reason,omitempty"`
	Evidence        []Evidence `json:"evidence,omitempty"`
	Tests           []TestRun  `json:"tests,omitempty"`
	Source          string     `json:"source,omitempty"`
	Outcome         string     `json:"outcome"` // applied | duplicate | ignored
	AppliedStatus   string     `json:"applied_status,omitempty"`
}

// Evidence is a pointer to work the agent claims (files, commit, log, test).
type Evidence struct {
	Kind string `json:"kind,omitempty"` // files | commit | log | test | diff | other
	Ref  string `json:"ref"`
}

// TestRun is one test the agent reported.
type TestRun struct {
	Name   string `json:"name"`
	Result string `json:"result"` // pass | fail | skip
	Detail string `json:"detail,omitempty"`
}

// StatusChange is one recorded transition of a command's execution status.
type StatusChange struct {
	At       time.Time `json:"at"`
	Command  string    `json:"command"`
	From     string    `json:"from"`
	To       string    `json:"to"`
	Reason   string    `json:"reason,omitempty"`
	ReportID string    `json:"report_id,omitempty"`
	Source   string    `json:"source,omitempty"`
}
