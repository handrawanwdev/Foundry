package progress

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/handrawanwdev/foundry/internal/core"
)

func clinic() *core.Project {
	return &core.Project{Name: "Klinik", Dir: "/tmp/klinik",
		Workflow: &core.Workflow{Version: 1, Commands: []core.Command{
			{ID: "CMD-001", Name: "Skema", Objective: "o", Instructions: "i",
				Acceptance: []string{"migrasi jalan"}, Outputs: []string{"schema.sql"}},
			{ID: "CMD-002", Name: "API", Objective: "o", Instructions: "i",
				DependsOn: []string{"CMD-001"}, Acceptance: []string{"POST /janji 201"}},
			{ID: "CMD-003", Name: "UI", Objective: "o", Instructions: "i",
				DependsOn: []string{"CMD-002"}},
		}}}
}

func report(cmd, status string) ReportIn {
	return ReportIn{Command: cmd, Status: status, Source: "claude-code"}
}

func mustApply(t *testing.T, p *core.Project, in ReportIn, now time.Time) *ApplyResult {
	t.Helper()
	out, err := Apply(p, in, now)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	return out
}

func TestUnreportedCommandsStayUnconfirmed(t *testing.T) {
	p := clinic()
	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	s := View(p, now, Agent{})
	if s.Counts.Total != 3 || s.Counts.Unconfirmed != 1 || s.Counts.Waiting != 2 {
		t.Fatalf("counts = %+v", s.Counts)
	}
	if s.Complete || s.Commands[0].Status != core.RunUnconfirmed || s.Commands[1].Status != core.RunWaiting {
		t.Fatalf("commands = %+v", s.Commands)
	}
	if len(s.Next) != 1 || s.Next[0] != "CMD-001" {
		t.Fatalf("next = %v", s.Next)
	}
}

func TestStartedThenVerifiedWithEvidence(t *testing.T) {
	p := clinic()
	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	mustApply(t, p, report("CMD-001", core.RunStarted), now)
	if got := View(p, now.Add(time.Minute), Agent{}).Commands[0].Status; got != core.RunStarted {
		t.Fatalf("started = %s", got)
	}
	in := report("CMD-001", core.RunDone)
	in.Summary = "migrasi ditulis"
	in.Evidence = []core.Evidence{{Kind: "files", Ref: "db/schema.sql"}}
	in.Tests = []core.TestRun{{Name: "migrate", Result: "pass"}}
	out := mustApply(t, p, in, now.Add(2*time.Minute))
	if out.AppliedStatus != core.RunVerified {
		t.Fatalf("applied = %+v", out)
	}
	s := View(p, now.Add(2*time.Minute), Agent{})
	if s.Commands[0].Status != core.RunVerified || s.Counts.Verified != 1 {
		t.Fatalf("view = %+v", s.Commands[0])
	}
	if s.Complete || len(s.Next) == 0 || s.Next[0] != "CMD-002" {
		t.Fatalf("next after verify = %v complete=%v", s.Next, s.Complete)
	}
}

func TestDoneWithoutEvidenceBecomesReview(t *testing.T) {
	p := clinic()
	now := time.Now().UTC()
	out := mustApply(t, p, report("CMD-001", core.RunDone), now)
	if out.AppliedStatus != core.RunReview || !strings.Contains(out.Reason, "bukti") {
		t.Fatalf("out = %+v", out)
	}
	s := View(p, now, Agent{})
	if s.Commands[0].Status != core.RunReview || s.Complete {
		t.Fatalf("status = %s complete=%v", s.Commands[0].Status, s.Complete)
	}
}

func TestFailedTestsBlockVerified(t *testing.T) {
	p := clinic()
	now := time.Now().UTC()
	in := report("CMD-001", core.RunDone)
	in.Evidence = []core.Evidence{{Ref: "db/schema.sql"}}
	in.Tests = []core.TestRun{{Name: "migrate", Result: "fail"}}
	out := mustApply(t, p, in, now)
	if out.AppliedStatus != core.RunReview {
		t.Fatalf("out = %+v", out)
	}
}

func TestSkipIsNotDone(t *testing.T) {
	p := clinic()
	now := time.Now().UTC()
	if _, err := Apply(p, report("CMD-003", core.RunSkipped), now); err == nil {
		t.Fatal("skip without reason accepted")
	}
	in := report("CMD-003", core.RunSkipped)
	in.Reason = "UI ditunda"
	mustApply(t, p, in, now)
	s := View(p, now, Agent{})
	if s.Commands[2].Status != core.RunSkipped || s.Complete || s.Counts.Verified != 0 {
		t.Fatalf("skip counted as complete: %+v", s.Counts)
	}
}

func TestDuplicateAndLateReportsDoNotMoveStatus(t *testing.T) {
	p := clinic()
	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	first := report("CMD-001", core.RunStarted)
	first.ID = "r1"
	mustApply(t, p, first, now)
	dup := mustApply(t, p, first, now.Add(time.Minute))
	if !dup.Duplicate || p.Progress.Commands["CMD-001"].Status != core.RunStarted {
		t.Fatalf("dup = %+v run=%+v", dup, p.Progress.Commands["CMD-001"])
	}
	done := report("CMD-001", core.RunDone)
	done.ID, done.Seq = "r2", 2
	done.Evidence = []core.Evidence{{Ref: "a.sql"}}
	mustApply(t, p, done, now.Add(2*time.Minute))
	late := report("CMD-001", core.RunFailed)
	late.ID, late.Seq = "r0", 1
	late.Reason = "lama"
	out := mustApply(t, p, late, now.Add(3*time.Minute))
	if !out.Ignored || p.Progress.Commands["CMD-001"].Status != core.RunVerified {
		t.Fatalf("late overwrote: %+v run=%+v", out, p.Progress.Commands["CMD-001"])
	}
}

func TestStaleIsNotFailedOrDone(t *testing.T) {
	p := clinic()
	p.Progress = &core.Progress{StaleAfterSec: 60}
	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	mustApply(t, p, report("CMD-001", core.RunStarted), now)
	later := now.Add(2 * time.Minute)
	s := View(p, later, Agent{})
	if s.Commands[0].Status != core.RunStale || s.Commands[0].Stored != core.RunStarted {
		t.Fatalf("stale = %+v", s.Commands[0])
	}
	if s.Counts.Failed != 0 || s.Counts.Verified != 0 || s.Counts.Done != 0 || s.Complete {
		t.Fatalf("time mutated terminal status: %+v", s.Counts)
	}
}

func TestUnknownWhenAgentDisconnected(t *testing.T) {
	p := clinic()
	now := time.Now().UTC()
	mustApply(t, p, report("CMD-001", core.RunStarted), now)
	s := View(p, now.Add(time.Minute), Agent{Tracked: true, Alive: false, Source: "claude-code"})
	if s.Commands[0].Status != core.RunUnknown || s.Commands[0].Stored != core.RunStarted {
		t.Fatalf("unknown = %+v", s.Commands[0])
	}
	if s.Counts.Started != 0 {
		t.Fatalf("still counted as running: %+v", s.Counts)
	}
}

func TestEarlyStartDetected(t *testing.T) {
	p := clinic()
	now := time.Now().UTC()
	mustApply(t, p, report("CMD-002", core.RunStarted), now)
	s := View(p, now, Agent{})
	if !s.Commands[1].EarlyStart || len(s.Commands[1].Waiting) != 1 {
		t.Fatalf("early = %+v", s.Commands[1])
	}
}

func TestInvalidReportsRejected(t *testing.T) {
	p := clinic()
	now := time.Now().UTC()
	if _, err := Apply(p, report("CMD-009", core.RunStarted), now); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unknown command err = %v", err)
	}
	if _, err := Apply(p, report("CMD-001", "finished"), now); !errors.Is(err, ErrInvalid) {
		t.Fatalf("bad status err = %v", err)
	}
	bad := report("CMD-001", core.RunStarted)
	bad.Project = "Lain"
	if _, err := Apply(p, bad, now); !errors.Is(err, ErrInvalid) {
		t.Fatalf("wrong project err = %v", err)
	}
	if p.Progress != nil && len(p.Progress.Reports) != 0 {
		t.Fatalf("invalid report stored: %+v", p.Progress.Reports)
	}
}

func TestWorkflowChangeKeepsHistory(t *testing.T) {
	p := clinic()
	now := time.Now().UTC()
	in := report("CMD-001", core.RunDone)
	in.Evidence = []core.Evidence{{Ref: "a.sql"}}
	mustApply(t, p, in, now)
	p.Workflow.Version = 2
	p.Workflow.Commands = []core.Command{
		p.Workflow.Commands[1],
		{ID: "CMD-004", Name: "Baru", Objective: "o", Instructions: "i"},
	}
	Reconcile(p, now.Add(time.Minute))
	if _, ok := p.Progress.Retired["CMD-001"]; !ok {
		t.Fatalf("retired missing: %+v", p.Progress.Retired)
	}
	if p.Progress.Retired["CMD-001"].Status != core.RunVerified {
		t.Fatalf("lost verified history: %+v", p.Progress.Retired["CMD-001"])
	}
	s := View(p, now.Add(time.Minute), Agent{})
	if len(s.Commands) != 2 || s.Commands[1].ID != "CMD-004" || s.Commands[1].Status != core.RunUnconfirmed {
		t.Fatalf("new command = %+v", s.Commands)
	}
	if s.Complete {
		t.Fatal("invented complete after structural change")
	}
}

func TestWorkflowCompleteOnlyWhenAllVerified(t *testing.T) {
	p := clinic()
	now := time.Now().UTC()
	for _, id := range []string{"CMD-001", "CMD-002"} {
		in := report(id, core.RunDone)
		in.Evidence = []core.Evidence{{Ref: id + ".go"}}
		mustApply(t, p, in, now)
		now = now.Add(time.Minute)
	}
	s := View(p, now, Agent{})
	if s.Complete || s.Counts.Verified != 2 {
		t.Fatalf("early complete: %+v", s.Counts)
	}
	in := report("CMD-003", core.RunDone)
	in.Evidence = []core.Evidence{{Ref: "ui.tsx"}}
	mustApply(t, p, in, now)
	s = View(p, now, Agent{})
	if !s.Complete || s.Counts.Verified != 3 {
		t.Fatalf("not complete: %+v", s.Counts)
	}
}

func TestRestartRoundTrip(t *testing.T) {
	p := clinic()
	now := time.Now().UTC()
	in := report("CMD-001", core.RunDone)
	in.ID = "persist-1"
	in.Evidence = []core.Evidence{{Ref: "a.sql"}}
	mustApply(t, p, in, now)
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var loaded core.Project
	if err := json.Unmarshal(b, &loaded); err != nil {
		t.Fatal(err)
	}
	s := View(&loaded, now.Add(time.Second), Agent{})
	if s.Commands[0].Status != core.RunVerified || s.Commands[0].ReportID != "persist-1" {
		t.Fatalf("lost after reload: %+v", s.Commands[0])
	}
	if len(loaded.Progress.History) == 0 || loaded.Progress.History[0].To != core.RunVerified {
		t.Fatalf("history = %+v", loaded.Progress.History)
	}
}

func TestNoWorkflow(t *testing.T) {
	if _, err := Apply(&core.Project{Name: "x"}, report("CMD-001", core.RunStarted), time.Now()); !errors.Is(err, ErrNoWorkflow) {
		t.Fatalf("err = %v", err)
	}
}

func TestVisualQualityIsPartOfProgress(t *testing.T) {
	p := clinic()
	p.Platform = &core.PlatformPlan{Targets: []core.PlatformTarget{{Kind: core.PlatformWeb}}}
	p.UI = &core.UIDesign{Format: core.UIDesignFormat, Rules: &core.DesignRules{Source: core.RulesNew},
		Layout: &core.LayoutSpec{Pages: []core.LayoutPage{{ID: "antrean", Name: "Antrean", Platform: "web", Pattern: "detail",
			Purpose: "Lihat", Goal: "GOAL-1", Sections: []core.Section{{ID: "isi", Name: "Isi", Content: []string{"Data"}}}}}}}
	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	s := View(p, now, Agent{})
	if hasKind(s.Warnings, "visual_missing") {
		t.Fatalf("warned before any implementation: %+v", s.Warnings)
	}
	mustApply(t, p, ReportIn{Command: "CMD-001", Status: core.RunDone, Source: "agent",
		Evidence: []core.Evidence{{Kind: "files", Ref: "db/schema.sql"}}}, now)
	s = View(p, now, Agent{})
	if !hasKind(s.Warnings, "visual_missing") || s.Visual.Pages != 1 || s.Visual.Untested != 1 || s.VisualReady {
		t.Fatalf("visual = %+v warnings = %+v", s.Visual, s.Warnings)
	}
	p.Visual = &core.VisualAudit{Pages: map[string]core.PageAudit{"antrean": {Page: "antrean", Status: core.VisualFail,
		CheckedAt: now, Findings: []core.VisualFinding{{Page: "antrean", Criterion: "contrast", Severity: core.SeverityHigh}}}}}
	s = View(p, now, Agent{})
	if !hasKind(s.Warnings, "visual") || s.Visual.OpenHigh != 1 || s.VisualReady {
		t.Fatalf("visual = %+v warnings = %+v", s.Visual, s.Warnings)
	}
}

func hasKind(ws []Warning, kind string) bool {
	for _, w := range ws {
		if w.Kind == kind {
			return true
		}
	}
	return false
}
