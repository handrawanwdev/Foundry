package pipeline_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/pipeline"
	"github.com/handrawanwdev/foundry/internal/modules/progress"
	"github.com/handrawanwdev/foundry/internal/modules/spec"
	"github.com/handrawanwdev/foundry/internal/modules/work"
)

func commandOf(t *testing.T, snap progress.Snapshot, id string) progress.CommandProgress {
	t.Helper()
	for _, c := range snap.Commands {
		if c.ID == id {
			return c
		}
	}
	t.Fatalf("command %s not in snapshot", id)
	return progress.CommandProgress{}
}

func TestAIRevisionKeepsProgressAndHistory(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	designReady(t, s)
	if _, err := s.ApproveStack(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApproveStructure(); err != nil {
		t.Fatal(err)
	}
	if err := workflowReady(s); err != nil {
		t.Fatal(err)
	}
	p := load(t, s)
	if n := len(p.Workflow.Changes); n != 1 || p.Workflow.Changes[0].Kind != "generate" || len(p.Workflow.Changes[0].Added) != 3 {
		t.Fatalf("generation not recorded: %+v", p.Workflow.Changes)
	}
	for _, id := range []string{"CMD-001", "CMD-002"} {
		if _, err := s.ReportProgress(progress.ReportIn{Command: id, Status: core.RunDone, Source: "cli", Files: []string{"a.go"}}, "agent"); err != nil {
			t.Fatal(err)
		}
	}
	before := p.Workflow.Command("CMD-002").Instructions

	if err := s.ReviseWorkflow("tambahkan test integrasi", []string{"CMD-002"}); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p = load(t, s)
	last := p.Workflow.Changes[len(p.Workflow.Changes)-1]
	if last.Kind != "revise" || last.Reason != "tambahkan test integrasi" || len(last.Added) != 1 || len(last.Modified) != 1 || last.Reused != 2 {
		t.Fatalf("delta = %+v", last)
	}
	if last.Modified[0].Before.Instructions != before {
		t.Fatalf("earlier text lost: %+v", last.Modified[0])
	}
	snap, err := s.ProgressView()
	if err != nil {
		t.Fatal(err)
	}
	if got := commandOf(t, snap, "CMD-001"); got.Status != core.RunVerified {
		t.Fatalf("untouched command lost progress: %+v", got)
	}
	if got := commandOf(t, snap, "CMD-002"); got.Status != core.RunRevision || got.Revision != 2 {
		t.Fatalf("revised command = %+v", got)
	}
	if len(snap.Issues) != 0 {
		t.Fatalf("issues = %v", snap.Issues)
	}
	if p.Workflow.Request = (&core.ChangeRequest{Text: "x"}); !strings.Contains(work.WorkflowPrompt(p, spec.Artifacts(p)), "EXECUTION STATE") {
		t.Fatal("revision prompt does not know what the agent finished")
	}
}

func TestEditWithoutChangeIsNoRevision(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	designReady(t, s)
	if err := workflowReady(s); err != nil {
		t.Fatal(err)
	}
	c := *load(t, s).Workflow.Command("CMD-001")
	refs := make([]string, len(c.Artifacts))
	for i, r := range c.Artifacts {
		refs[i] = r.ID
	}
	edit := work.CommandEdit{Name: c.Name, Objective: c.Objective, Context: c.Context, Inputs: c.Inputs, Instructions: c.Instructions,
		DependsOn: c.DependsOn, Artifacts: refs, Covers: c.Covers, FeatureIDs: c.FeatureIDs, SubFeatureIDs: c.SubFeatureIDs,
		Outputs: c.Outputs, Acceptance: c.Acceptance, Constraints: c.Constraints, Files: c.Files, Priority: c.Priority, Complexity: c.Complexity}
	if _, err := s.EditCommand("CMD-001", edit); err != nil {
		t.Fatal(err)
	}
	p := load(t, s)
	if p.Workflow.Command("CMD-001").Revision != 0 || len(p.Workflow.Changes) != 1 {
		t.Fatalf("no-op edit counted: %+v", p.Workflow.Changes)
	}
	edit.Priority, edit.Files = "HIGH", []string{"api/"}
	if _, err := s.EditCommand("CMD-001", edit); err != nil {
		t.Fatal(err)
	}
	got := load(t, s).Workflow.Command("CMD-001")
	if got.Revision != 2 || got.Priority != core.LevelHigh || !slices.Equal(got.Files, []string{"api/"}) {
		t.Fatalf("edit = %+v", got)
	}
}

func TestClaimGivesTheBrief(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	seedWorkflow(t, s)
	first, err := s.ClaimCommand("", "a", "cli")
	if err != nil || first.Command != "CMD-001" || first.Brief == nil || !strings.Contains(first.Brief.Markdown, "CMD-001") {
		t.Fatalf("claim = %+v %v", first, err)
	}
	version := load(t, s).Version
	if again, err := s.ClaimCommand("", "a", "cli"); err != nil || !again.Resumed {
		t.Fatalf("resume = %+v %v", again, err)
	}
	if load(t, s).Version != version {
		t.Fatal("resuming wrote the state")
	}
	if _, err := s.ClaimCommand("CMD-002", "b", "cli"); !errors.Is(err, progress.ErrDepsUnmet) {
		t.Fatalf("unmet dependency err = %v", err)
	}
	if _, err := s.ReportProgress(progress.ReportIn{Command: "CMD-001", Status: core.RunDone, Summary: "skema jadi", Files: []string{"db.sql"}, Session: "a", Source: "cli"}, "agent"); err != nil {
		t.Fatal(err)
	}
	second, err := s.NextCommand("b")
	if err != nil || second.Command != "CMD-002" {
		t.Fatalf("next = %+v %v", second, err)
	}
	if len(second.Brief.Dependencies) != 1 || second.Brief.Dependencies[0].Summary != "skema jadi" {
		t.Fatalf("dependency result = %+v", second.Brief.Dependencies)
	}
	if commandOf(t, must(s.ProgressView()), "CMD-002").Stored != core.RunUnconfirmed {
		t.Fatal("next without claim changed the status")
	}
}

func must(snap progress.Snapshot, err error) progress.Snapshot {
	if err != nil {
		panic(err)
	}
	return snap
}

func TestInboxIngest(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	seedWorkflow(t, s)
	dir := filepath.Join(s.Store.Dir, core.StateDir, "inbox")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a.json", `{"command":"CMD-001","status":"started","session":"s1"}`)
	write("b.json", `{"command":"CMD-009","status":"started"}`)
	write("c.json", `not json`)
	write("note.txt", `dibiarkan`)
	if err := os.Symlink(filepath.Join(dir, "a.json"), filepath.Join(dir, "link.json")); err != nil {
		t.Fatal(err)
	}
	applied, rejected, err := s.IngestInbox()
	if err != nil || applied != 1 || rejected != 2 {
		t.Fatalf("ingest = %d %d %v", applied, rejected, err)
	}
	snap := must(s.ProgressView())
	if c := commandOf(t, snap, "CMD-001"); c.Stored != core.RunStarted || c.Source != "file" || c.Session != "s1" {
		t.Fatalf("report = %+v", c)
	}
	if _, err := os.Stat(filepath.Join(dir, "done", "a.json")); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "rejected", "b.json.error")); err != nil || !strings.Contains(string(b), "CMD-009") {
		t.Fatalf("rejection = %q %v", b, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "note.txt")); err != nil {
		t.Fatal("other files stay")
	}

	// The same file again counts once: its name is the report id.
	write("a.json", `{"command":"CMD-001","status":"started"}`)
	if _, _, err := s.IngestInbox(); err != nil {
		t.Fatal(err)
	}
	if n := len(load(t, s).Progress.Reports); n != 1 {
		t.Fatalf("reports = %d", n)
	}
}

func TestReportQualityAndUncoveredFeatures(t *testing.T) {
	p := &core.Project{Features: []core.Feature{{ID: "FEAT-001", Name: "Janji"}, {ID: "FEAT-002", Name: "Jadwal"}},
		Workflow: &core.Workflow{Commands: []core.Command{{ID: "CMD-001", Instructions: "singkat", FeatureIDs: []string{"FEAT-001"}}}}}
	r := pipeline.CheckWorkflow(p)
	if !slices.Equal(r.UncoveredFeatures, []string{"FEAT-002"}) || len(r.Quality) == 0 {
		t.Fatalf("report = %+v", r)
	}
}

func TestBlueprintChangeReachesFinishedWork(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	designReady(t, s)
	if err := workflowReady(s); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReportProgress(progress.ReportIn{Command: "CMD-001", Status: core.RunDone, Source: "cli", Files: []string{"a.go"}}, "agent"); err != nil {
		t.Fatal(err)
	}
	if got := commandOf(t, must(s.ProgressView()), "CMD-001"); got.Status != core.RunVerified {
		t.Fatalf("before = %+v", got)
	}
	if _, err := s.Store.Update(func(p *core.Project) error {
		p.Requirements[0].Text += " (berubah)"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	got := commandOf(t, must(s.ProgressView()), "CMD-001")
	if got.Status != core.RunRevision || !strings.Contains(got.RevisionWhy, "requirements") {
		t.Fatalf("after = %+v", got)
	}
}
