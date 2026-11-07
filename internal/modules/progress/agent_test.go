package progress

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/handrawanwdev/foundry/internal/core"
)

func finishCommand(t *testing.T, p *core.Project, id string, now time.Time) {
	t.Helper()
	in := report(id, core.RunDone)
	in.Files = []string{"a.go"}
	mustApply(t, p, in, now)
}

func phase(s Snapshot, id string) string {
	for _, c := range s.Commands {
		if c.ID == id {
			return c.Phase
		}
	}
	return ""
}

func TestLifecycleFollowsReports(t *testing.T) {
	p := clinic()
	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	s := View(p, now, Agent{})
	if phase(s, "CMD-001") != core.PhaseReady || phase(s, "CMD-002") != core.PhaseCreated {
		t.Fatalf("before any report: %+v", s.Commands)
	}
	mustApply(t, p, report("CMD-001", core.RunStarted), now)
	if got := phase(View(p, now, Agent{}), "CMD-001"); got != core.PhaseInProgress {
		t.Fatalf("started phase = %s", got)
	}
	pct := 40
	in := report("CMD-001", core.RunProgress)
	in.Summary, in.Percent, in.Files = "tabel dibuat", &pct, []string{"db/a.sql"}
	mustApply(t, p, in, now.Add(time.Minute))
	s = View(p, now.Add(2*time.Minute), Agent{})
	c := s.Commands[0]
	if c.Phase != core.PhaseUpdating || c.Stored != core.RunStarted || c.Activity != "tabel dibuat" || *c.Percent != 40 {
		t.Fatalf("progress report = %+v", c)
	}
	if s.Percent != 0 || s.Overview.InProgress != 1 {
		t.Fatalf("an estimate must not count as verified: %d %+v", s.Percent, s.Overview)
	}
	if got := phase(View(p, now.Add(10*time.Minute), Agent{}), "CMD-001"); got != core.PhaseInProgress {
		t.Fatalf("old note still updating: %s", got)
	}

	in = report("CMD-001", core.RunDone)
	in.Summary = "selesai"
	out := mustApply(t, p, in, now.Add(11*time.Minute))
	if out.AppliedStatus != core.RunVerified {
		t.Fatalf("files of the attempt count as evidence: %+v", out)
	}
	s = View(p, now.Add(11*time.Minute), Agent{})
	if phase(s, "CMD-001") != core.PhaseCompleted || phase(s, "CMD-002") != core.PhaseReady || s.Commands[0].FinishedAt == nil {
		t.Fatalf("after done: %+v", s.Commands)
	}
	o := s.Overview
	if o.Total != 3 || o.Completed+o.InProgress+o.Pending+o.Blocked+o.Failed+o.Review+o.Revision+o.Skipped+o.Stale != o.Total {
		t.Fatalf("overview buckets = %+v", o)
	}
}

func TestProgressNoteDoesNotReopenFinishedCommand(t *testing.T) {
	p := clinic()
	now := time.Now().UTC()
	finishCommand(t, p, "CMD-001", now)
	finishedAt := *View(p, now, Agent{}).Commands[0].FinishedAt
	in := report("CMD-001", core.RunProgress)
	in.Summary = "catatan terlambat"
	mustApply(t, p, in, now.Add(time.Minute))
	c := View(p, now.Add(time.Minute), Agent{}).Commands[0]
	if c.Status != core.RunVerified || !c.FinishedAt.Equal(finishedAt) {
		t.Fatalf("late note moved the result: %+v", c)
	}
}

func TestNewAttemptStartsWithoutOldEvidence(t *testing.T) {
	p := clinic()
	now := time.Now().UTC()
	finishCommand(t, p, "CMD-001", now)
	mustApply(t, p, report("CMD-001", core.RunStarted), now.Add(time.Minute))
	out := mustApply(t, p, report("CMD-001", core.RunDone), now.Add(2*time.Minute))
	if out.AppliedStatus != core.RunReview {
		t.Fatalf("old evidence verified the new attempt: %+v", out)
	}
}

func TestProgressPercentOutOfRangeRefused(t *testing.T) {
	p := clinic()
	bad := 140
	in := report("CMD-001", core.RunProgress)
	in.Percent = &bad
	if _, err := Apply(p, in, time.Now().UTC()); !errors.Is(err, ErrInvalid) {
		t.Fatalf("err = %v", err)
	}
}

func TestReviseSendsFinishedWorkBackWithoutLosingIt(t *testing.T) {
	p := clinic()
	now := time.Now().UTC()
	finishCommand(t, p, "CMD-001", now)
	mustApply(t, p, report("CMD-002", core.RunStarted), now)

	before := slices.Clone(p.Workflow.Commands)
	p.Workflow.Commands[0].Instructions = "tulis ulang skema"
	p.Workflow.Commands = append(p.Workflow.Commands, core.Command{ID: "CMD-004", Name: "Baru", Objective: "o", Instructions: "i"})
	p.Workflow.Version++
	ch := Track(p, before, "revise", "skema berubah", "ai", now.Add(time.Minute))
	if ch == nil || len(ch.Added) != 1 || len(ch.Modified) != 1 || ch.Reused != 2 || len(ch.Removed) != 0 {
		t.Fatalf("delta = %+v", ch)
	}
	if ch.Modified[0].Before.Instructions != "i" || !slices.Equal(ch.Modified[0].Fields, []string{"instructions"}) {
		t.Fatalf("old text lost: %+v", ch.Modified[0])
	}
	if p.Workflow.Commands[0].Revision != 2 || p.Workflow.Commands[1].Revision != 0 {
		t.Fatalf("revisions = %+v", p.Workflow.Commands)
	}

	s := View(p, now.Add(2*time.Minute), Agent{})
	c := s.Commands[0]
	if c.Status != core.RunRevision || c.Phase != core.PhaseRevision || c.Stored != core.RunVerified {
		t.Fatalf("revised = %+v", c)
	}
	if !slices.Contains(c.Impact, "CMD-002") || !slices.Contains(c.Impact, "CMD-003") {
		t.Fatalf("impact = %v", c.Impact)
	}
	if s.Commands[1].Stored != core.RunStarted || len(s.Commands[1].Waiting) != 1 {
		t.Fatalf("a dependent of redone work is not free to go: %+v", s.Commands[1])
	}
	if s.Percent != 0 || s.Complete || s.Overview.Revision != 1 {
		t.Fatalf("revised work still counted: %d %+v", s.Percent, s.Overview)
	}
	if len(s.Issues) != 0 {
		t.Fatalf("tracked revision reported as out of sync: %v", s.Issues)
	}

	mustApply(t, p, report("CMD-001", core.RunStarted), now.Add(3*time.Minute))
	out := func() *ApplyResult {
		in := report("CMD-001", core.RunDone)
		in.Files = []string{"b.go"}
		return mustApply(t, p, in, now.Add(4*time.Minute))
	}()
	if out.AppliedStatus != core.RunVerified || out.Warning != "" {
		t.Fatalf("redone = %+v", out)
	}
	if got := View(p, now.Add(4*time.Minute), Agent{}).Commands[0]; got.Status != core.RunVerified {
		t.Fatalf("redone work counts again: %+v", got)
	}

	d, err := Detail(p, "CMD-001")
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Changes) != 1 || d.Changes[0].Before == nil || d.Changes[0].Revision != 2 {
		t.Fatalf("changes = %+v", d.Changes)
	}
	if len(d.Reports) != 3 || d.Reports[0].Status != core.RunDone {
		t.Fatalf("reports = %+v", d.Reports)
	}
	if !slices.ContainsFunc(d.History, func(h core.StatusChange) bool { return h.To == core.RunRevision }) {
		t.Fatalf("history = %+v", d.History)
	}
}

func TestDoneOnOlderRevisionIsFlagged(t *testing.T) {
	p := clinic()
	now := time.Now().UTC()
	mustApply(t, p, report("CMD-001", core.RunStarted), now)
	before := slices.Clone(p.Workflow.Commands)
	p.Workflow.Commands[0].Acceptance = []string{"lebih ketat"}
	Track(p, before, "edit", "", "ui", now)
	if s := View(p, now, Agent{}); !slices.ContainsFunc(s.Warnings, func(w Warning) bool { return w.Kind == "revised" }) {
		t.Fatalf("no warning while running: %+v", s.Warnings)
	}
	in := report("CMD-001", core.RunDone)
	in.Files = []string{"a.go"}
	out := mustApply(t, p, in, now.Add(time.Minute))
	if out.Warning == "" {
		t.Fatalf("agent not told: %+v", out)
	}
	if got := View(p, now.Add(time.Minute), Agent{}).Commands[0].Status; got != core.RunRevision {
		t.Fatalf("work on r1 accepted for r2: %s", got)
	}
}

func TestChangedBlueprintAsksToLookAgain(t *testing.T) {
	p := clinic()
	now := time.Now().UTC()
	finishCommand(t, p, "CMD-001", now)
	stale := map[string][]string{"CMD-001": {"architecture"}, "CMD-002": {"architecture"}}
	s := ViewStale(p, now, Agent{}, stale)
	if s.Commands[0].Status != core.RunRevision || !strings.Contains(s.Commands[0].RevisionWhy, "architecture") {
		t.Fatalf("stale finished work = %+v", s.Commands[0])
	}
	if pick := Next(s, ""); pick.Command != "" || len(pick.Attention) == 0 {
		t.Fatalf("instructions that may be wrong were handed out: %+v", pick)
	}
}

func TestSkeletonDetailsAreNotARevision(t *testing.T) {
	p := clinic()
	p.Workflow.Commands[0].Instructions = ""
	before := slices.Clone(p.Workflow.Commands)
	p.Workflow.Commands[0].Instructions = "isi"
	if ch := Track(p, before, "revise", "", "ai", time.Now().UTC()); ch != nil {
		t.Fatalf("change = %+v", ch)
	}
	if p.Workflow.Commands[0].Revision != 0 {
		t.Fatalf("revision = %d", p.Workflow.Commands[0].Revision)
	}
}

func TestRemovedCommandIsKept(t *testing.T) {
	p := clinic()
	now := time.Now().UTC()
	before := slices.Clone(p.Workflow.Commands)
	p.Workflow.Commands = p.Workflow.Commands[:2]
	ch := Track(p, before, "revise", "", "ai", now)
	if ch == nil || len(ch.Removed) != 1 || ch.Removed[0].Before.Name != "UI" {
		t.Fatalf("removed = %+v", ch)
	}
}

func TestChangesAreBounded(t *testing.T) {
	p := clinic()
	now := time.Now().UTC()
	for i := range maxChanges + 5 {
		before := slices.Clone(p.Workflow.Commands)
		p.Workflow.Commands[2].Context = strings.Repeat("x", i+1)
		Track(p, before, "edit", "", "ui", now)
	}
	if len(p.Workflow.Changes) != maxChanges {
		t.Fatalf("changes = %d", len(p.Workflow.Changes))
	}
}

func TestClaimHandsEachCommandOnce(t *testing.T) {
	p := clinic()
	p.Workflow.Commands = append(p.Workflow.Commands, core.Command{ID: "CMD-004", Name: "Lain", Objective: "o", Instructions: "i", Priority: core.LevelHigh})
	now := time.Now().UTC()
	a, err := Claim(p, "", "a", "cli", now, Agent{}, nil)
	if err != nil || a.Command != "CMD-004" {
		t.Fatalf("high priority first: %+v %v", a, err)
	}
	b, err := Claim(p, "", "b", "cli", now, Agent{}, nil)
	if err != nil || b.Command != "CMD-001" {
		t.Fatalf("second agent: %+v %v", b, err)
	}
	if again, err := Claim(p, "", "a", "cli", now, Agent{}, nil); err != nil || again.Command != "CMD-004" || !again.Resumed {
		t.Fatalf("same session resumes: %+v %v", again, err)
	}
	if _, err := Claim(p, "CMD-001", "a", "cli", now, Agent{}, nil); !errors.Is(err, ErrClaimed) {
		t.Fatalf("taken from another session: %v", err)
	}
	none, err := Claim(p, "", "c", "cli", now, Agent{}, nil)
	if err != nil || none.Command != "" || none.Reason == "" {
		t.Fatalf("nothing left: %+v %v", none, err)
	}
}

func TestClaimRefusesUnmetDependency(t *testing.T) {
	p := clinic()
	if _, err := Claim(p, "CMD-002", "a", "cli", time.Now().UTC(), Agent{}, nil); !errors.Is(err, ErrDepsUnmet) {
		t.Fatalf("err = %v", err)
	}
	if run := p.Progress; run != nil && run.Commands["CMD-002"].Status == core.RunStarted {
		t.Fatal("started anyway")
	}
}

func TestFailedWaitsForAPerson(t *testing.T) {
	p := clinic()
	now := time.Now().UTC()
	mustApply(t, p, report("CMD-001", core.RunFailed), now)
	s := View(p, now, Agent{})
	pick := Next(s, "")
	if pick.Command != "" || !slices.Contains(pick.Attention, "CMD-001") {
		t.Fatalf("pick = %+v", pick)
	}
	if !slices.Equal(s.Commands[0].Impact, []string{"CMD-002", "CMD-003"}) {
		t.Fatalf("impact = %v", s.Commands[0].Impact)
	}
	if got, err := Claim(p, "CMD-001", "a", "cli", now, Agent{}, nil); err != nil || got.Command != "CMD-001" {
		t.Fatalf("retry by id: %+v %v", got, err)
	}
}

func TestSessionsAndActivityComeFromReports(t *testing.T) {
	p := clinic()
	now := time.Now().UTC()
	in := report("CMD-001", core.RunStarted)
	in.Session, in.Files = "s1", []string{"a.go"}
	mustApply(t, p, in, now)
	s := View(p, now.Add(time.Minute), Agent{})
	if len(s.Sessions) != 1 || s.Sessions[0].ID != "s1" || !s.Sessions[0].Active || s.Sessions[0].Command != "CMD-001" {
		t.Fatalf("sessions = %+v", s.Sessions)
	}
	if len(s.Activity) != 1 || s.Activity[0].Name != "Skema" || s.Activity[0].Files[0] != "a.go" {
		t.Fatalf("activity = %+v", s.Activity)
	}
	if got := View(p, now.Add(2*time.Hour), Agent{}).Sessions[0]; got.Active {
		t.Fatalf("old session active: %+v", got)
	}
}

func TestHeartbeatOnlySpeaksForItsOwnSource(t *testing.T) {
	p := clinic()
	now := time.Now().UTC()
	in := report("CMD-001", core.RunStarted)
	in.Source = "cli"
	mustApply(t, p, in, now)
	s := View(p, now.Add(time.Minute), Agent{Tracked: true, Alive: false, Source: "claude-code"})
	if s.Commands[0].Status != core.RunStarted {
		t.Fatalf("status = %s", s.Commands[0].Status)
	}
}

func TestLateNoteKeepsFailureReason(t *testing.T) {
	p := clinic()
	now := time.Now().UTC()
	in := report("CMD-001", core.RunFailed)
	in.Reason = "migrasi gagal"
	mustApply(t, p, in, now)
	note := report("CMD-001", core.RunProgress)
	note.Summary = "catatan"
	mustApply(t, p, note, now.Add(time.Minute))
	if c := View(p, now.Add(time.Minute), Agent{}).Commands[0]; c.Status != core.RunFailed || c.Reason != "migrasi gagal" {
		t.Fatalf("command = %+v", c)
	}
}

func TestOldBlockerIsNotHandedOut(t *testing.T) {
	p := clinic()
	now := time.Now().UTC()
	mustApply(t, p, report("CMD-001", core.RunBlocked), now)
	later := now.Add(2 * DefaultStaleAfter)
	if pick := Next(View(p, later, Agent{}), ""); pick.Command != "" {
		t.Fatalf("blocked command handed out: %+v", pick)
	}
}

func TestAbandonedCommandCanBeTakenOver(t *testing.T) {
	p := clinic()
	now := time.Now().UTC()
	in := report("CMD-001", core.RunStarted)
	in.Session = "mati"
	mustApply(t, p, in, now)
	later := now.Add(2 * DefaultStaleAfter)
	pick, err := Claim(p, "", "baru", "cli", later, Agent{}, nil)
	if err != nil || pick.Command != "CMD-001" {
		t.Fatalf("pick = %+v %v", pick, err)
	}
	if got := p.Progress.Commands["CMD-001"].Session; got != "baru" {
		t.Fatalf("session = %s", got)
	}
}
