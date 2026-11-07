package progress

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/handrawanwdev/foundry/internal/core"
)

// ErrDepsUnmet refuses to hand out a command before its dependencies are
// verified. Foundry cannot stop an agent that is already working, so a plain
// report stays truthful and is flagged; handing out work is what it controls.
var ErrDepsUnmet = errors.New("dependency belum terverifikasi")

// ErrClaimed refuses a command another session is working on.
var ErrClaimed = errors.New("command sedang dikerjakan session lain")

// claimablePhases are the commands an agent may pick up by itself. Failed,
// blocked and review wait for a person or an explicit start by id: handing
// them out again would repeat the same failure.
var claimablePhases = []string{core.PhaseReady, core.PhaseRevision, core.PhaseStale}

// claimable holds for a command an agent may pick up by itself. One whose
// blueprint changed waits for its instruction to be brought in line first.
func claimable(c CommandProgress) bool {
	if c.Phase == core.PhaseStale && c.Stored != core.RunStarted {
		return false // an old blocker is still a blocker
	}
	return slices.Contains(claimablePhases, c.Phase) && len(c.StaleDocs) == 0
}

// Pick is the outcome of asking for work.
type Pick struct {
	// Command is the id handed out, "" when nothing can start now.
	Command string `json:"command"`
	// Resumed means the session already held this command.
	Resumed bool `json:"resumed,omitempty"`
	// Reason says why nothing was handed out.
	Reason string `json:"reason,omitempty"`
	// Attention are the commands only a person or a retry by id can move on.
	Attention []string `json:"attention,omitempty"`
}

// Next picks what a session should do now: the command it already holds,
// else the first claimable command. Read-only.
func Next(s Snapshot, session string) Pick {
	if session != "" {
		for _, c := range s.Commands {
			if c.Session == session && (c.Phase == core.PhaseInProgress || c.Phase == core.PhaseUpdating) {
				return Pick{Command: c.ID, Resumed: true}
			}
		}
	}
	byID := make(map[string]CommandProgress, len(s.Commands))
	for _, c := range s.Commands {
		byID[c.ID] = c
	}
	var pick Pick
	for _, id := range s.Next {
		c := byID[id]
		switch {
		case !claimable(c):
			pick.Attention = append(pick.Attention, id)
		case pick.Command == "":
			pick.Command = id
		}
	}
	if pick.Command != "" {
		return pick
	}
	switch {
	case s.Counts.Total == 0:
		pick.Reason = "workflow belum punya command"
	case s.Complete:
		pick.Reason = "semua command sudah terverifikasi"
	case len(pick.Attention) > 0:
		pick.Reason = "tidak ada command baru; yang tersisa perlu tindakan: " + strings.Join(pick.Attention, ", ")
	case s.Overview.InProgress > 0:
		pick.Reason = "command yang siap sudah dikerjakan session lain; tunggu dependency selesai"
	default:
		pick.Reason = "belum ada command yang siap"
	}
	return pick
}

// Claim marks a command started for a session. id "" takes what Next picks.
// Foundry holds the store lock around it, so two agents never get the same
// command.
func Claim(p *core.Project, id, session, source string, now time.Time, agent Agent, stale map[string][]string) (Pick, error) {
	s := ViewStale(p, now, agent, stale)
	pick := Pick{Command: id}
	if id == "" {
		pick = Next(s, session)
		if pick.Command == "" || pick.Resumed {
			return pick, nil
		}
	} else {
		i := slices.IndexFunc(s.Commands, func(c CommandProgress) bool { return c.ID == id })
		if i < 0 {
			return pick, fmt.Errorf("%w: command %s tidak ada di workflow", ErrInvalid, id)
		}
		c := s.Commands[i]
		if len(c.Waiting) > 0 {
			return pick, fmt.Errorf("%w: %s menunggu %s", ErrDepsUnmet, id, strings.Join(c.Waiting, ", "))
		}
		if c.Phase == core.PhaseInProgress || c.Phase == core.PhaseUpdating {
			if session != "" && c.Session == session {
				pick.Resumed = true
				return pick, nil
			}
			if c.Session != "" {
				return pick, fmt.Errorf("%w: %s dipegang %s", ErrClaimed, id, c.Session)
			}
		}
	}
	pr := ensure(p)
	_, err := Apply(p, ReportIn{ID: fmt.Sprintf("claim-%s-%d", pick.Command, pr.Seq+1), Command: pick.Command, Status: core.RunStarted,
		Session: session, Source: source, Summary: "Mulai dikerjakan"}, now)
	return pick, err
}

// CommandDetail is everything recorded about one command.
type CommandDetail struct {
	Command string              `json:"command"`
	Reports []Activity          `json:"reports"`
	History []core.StatusChange `json:"history"`
	Changes []CommandRevision   `json:"changes"`
	// Evidence and Tests of the current attempt.
	Evidence []core.Evidence `json:"evidence"`
	Tests    []core.TestRun  `json:"tests"`
}

// CommandRevision is one change of the command with its earlier text.
type CommandRevision struct {
	Version  int           `json:"version"`
	At       time.Time     `json:"at"`
	Kind     string        `json:"kind"`
	Reason   string        `json:"reason,omitempty"`
	Source   string        `json:"source,omitempty"`
	Action   string        `json:"action"` // add | modify
	Revision int           `json:"revision"`
	Fields   []string      `json:"fields,omitempty"`
	Before   *core.Command `json:"before,omitempty"`
}

// Detail collects the reports, status changes and revisions of a command,
// newest first. It reads only what is stored.
func Detail(p *core.Project, id string) (*CommandDetail, error) {
	if p == nil || p.Workflow == nil || p.Workflow.Command(id) == nil {
		return nil, fmt.Errorf("%w: command %s tidak ada di workflow", ErrInvalid, id)
	}
	d := &CommandDetail{Command: id, Reports: []Activity{}, History: []core.StatusChange{}, Changes: []CommandRevision{},
		Evidence: []core.Evidence{}, Tests: []core.TestRun{}}
	if pr := p.Progress; pr != nil {
		for i := len(pr.Reports) - 1; i >= 0; i-- {
			if pr.Reports[i].Command == id {
				d.Reports = append(d.Reports, activityFrom(p, pr.Reports[i]))
			}
		}
		for i := len(pr.History) - 1; i >= 0; i-- {
			if pr.History[i].Command == id {
				d.History = append(d.History, pr.History[i])
			}
		}
		if run, ok := pr.Commands[id]; ok {
			d.Evidence = append(d.Evidence, run.Evidence...)
			d.Tests = append(d.Tests, run.Tests...)
		}
	}
	changes := p.Workflow.Changes
	for i := len(changes) - 1; i >= 0; i-- {
		c := changes[i]
		rev := CommandRevision{Version: c.Version, At: c.At, Kind: c.Kind, Reason: c.Reason, Source: c.Source}
		if slices.Contains(c.Added, id) {
			rev.Action, rev.Revision = "add", 1
			d.Changes = append(d.Changes, rev)
			continue
		}
		for _, m := range c.Modified {
			if m.ID == id {
				before := m.Before
				rev.Action, rev.Revision, rev.Fields, rev.Before = "modify", m.Revision, m.Fields, &before
				d.Changes = append(d.Changes, rev)
			}
		}
	}
	return d, nil
}

// Track records a workflow change as a delta against before, the commands as
// they were. Changed commands get the next revision and keep their old text;
// finished work on them stops counting until the agent reports it again. A
// skeleton getting its first details is the same revision: nobody could have
// worked on it. It returns nil when no command changed.
func Track(p *core.Project, before []core.Command, kind, reason, source string, now time.Time) *core.WorkflowChange {
	w := p.Workflow
	if w == nil {
		return nil
	}
	now = now.UTC()
	prev := make(map[string]core.Command, len(before))
	for _, c := range before {
		prev[c.ID] = c
	}
	ch := core.WorkflowChange{Version: w.Version, At: now, Kind: kind, Reason: reason, Source: source}
	for i := range w.Commands {
		c := &w.Commands[i]
		old, ok := prev[c.ID]
		if !ok {
			ch.Added = append(ch.Added, c.ID)
			continue
		}
		delete(prev, c.ID)
		c.Revision = old.Revision
		fields := core.CommandDiff(old, *c)
		switch {
		case len(fields) == 0:
			ch.Reused++
		case old.Instructions != "":
			c.Revision = core.CommandRevision(old) + 1
			ch.Modified = append(ch.Modified, core.CommandChange{ID: c.ID, Revision: c.Revision, Fields: fields, Before: old})
		}
	}
	for _, c := range before {
		if _, gone := prev[c.ID]; gone {
			ch.Removed = append(ch.Removed, core.CommandChange{ID: c.ID, Before: c})
		}
	}
	Reconcile(p, now)
	if len(ch.Added)+len(ch.Modified)+len(ch.Removed) == 0 {
		return nil
	}
	w.Changes = append(w.Changes, ch)
	if n := len(w.Changes); n > maxChanges {
		w.Changes = w.Changes[n-maxChanges:]
	}
	pr := ensure(p)
	for _, m := range ch.Modified {
		if run := pr.Commands[m.ID]; run.Status == core.RunVerified || run.Status == core.RunDone {
			appendHistory(pr, core.StatusChange{At: now, Command: m.ID, From: run.Status, To: core.RunRevision, Source: source,
				Reason: fmt.Sprintf("command direvisi ke r%d: %s", m.Revision, strings.Join(m.Fields, ", "))})
		}
	}
	return &w.Changes[len(w.Changes)-1]
}
