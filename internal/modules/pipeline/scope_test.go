package pipeline_test

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/pipeline"
	"github.com/handrawanwdev/foundry/internal/modules/work"
)

// A critical question blocks finalize until the user decides it; a
// reopened blueprint keeps the decision, and opening the decision again
// blocks again.
func TestCriticalDecisionBlocksFinalize(t *testing.T) {
	s := newService(t, fakeAI{}, "critical")
	p := designReady(t, s)
	var id string
	for _, d := range p.Decisions {
		if d.Critical {
			id = d.ID
		}
	}
	if id == "" || !slices.ContainsFunc(pipeline.Blockers(p), func(b string) bool { return strings.Contains(b, "keputusan kritis "+id) }) {
		t.Fatalf("decisions %+v blockers %v", p.Decisions, pipeline.Blockers(p))
	}
	if err := approve(s); !errors.Is(err, pipeline.ErrIncomplete) || !strings.Contains(err.Error(), id) {
		t.Fatalf("finalize with open critical decision err = %v", err)
	}
	if _, err := s.ResolveDecision(id, "WhatsApp lewat penyedia resmi"); err != nil {
		t.Fatal(err)
	}
	syncDecisions(t, s)
	if err := s.Finalize(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ResolveDecision(id, ""); !errors.Is(err, pipeline.ErrFinalized) {
		t.Fatalf("change after finalize err = %v", err)
	}
	if err := s.Reopen(); err != nil {
		t.Fatal(err)
	}
	if p = load(t, s); p.Decisions[slices.IndexFunc(p.Decisions, func(d core.Decision) bool { return d.ID == id })].Resolution == "" {
		t.Fatal("reopen dropped the decision")
	}
	if _, err := s.ResolveDecision(id, ""); err != nil {
		t.Fatal(err)
	}
	if err := s.Finalize(); !errors.Is(err, pipeline.ErrIncomplete) {
		t.Fatalf("finalize after reopening the decision err = %v", err)
	}
}

// A requirement with no feature blocks finalize: it can never be built or
// accepted, so the blueprint is not complete until it is mapped.
func TestOrphanedRequirementBlocksFinalize(t *testing.T) {
	s := newService(t, fakeAI{mcp: true}, "")
	mcpDesigned(t, s)
	if _, err := s.Store.Update(func(p *core.Project) error {
		p.Requirements = append(p.Requirements, core.Requirement{
			ID: "REQ-010", Text: "Performa & latency: data pasar disajikan dari cache", Priority: "MUST",
			Acceptance: []core.Criterion{{ID: "REQ-010-AC1", Kind: "behavior", Text: "Harga dari cache"}},
		})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	p := load(t, s)
	if b := pipeline.Blockers(p); !slices.ContainsFunc(b, func(s string) bool { return strings.Contains(s, "keterlacakan requirement: 1") }) {
		t.Fatalf("blockers = %v", b)
	}
	if err := s.Finalize(); !errors.Is(err, pipeline.ErrIncomplete) {
		t.Fatalf("finalize err = %v", err)
	}
}

func syncDecisions(t *testing.T, s *pipeline.Service) {
	t.Helper()
	if err := s.SyncDecisions(); err != nil {
		t.Fatal(err)
	}
	s.Wait()
}

// A decision the user changes waits until the AI brought the artifacts,
// then the commands, in line with it; until then finalize is blocked. The
// AI may find nothing contradicting it.
func TestDecisionChangeSyncsBlueprintAndCommands(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runs")
	t.Setenv("FAKE_RUNS", path)
	s := newService(t, fakeAI{}, "critical")
	p := designReady(t, s)
	if err := workflowReady(s); err != nil {
		t.Fatal(err)
	}
	id := p.OpenCritical()[0].ID
	if err := s.SyncDecisions(); !errors.Is(err, pipeline.ErrNothingToDo) {
		t.Fatalf("sync without a changed decision err = %v", err)
	}
	if _, err := s.ResolveDecision(id, "WhatsApp lewat penyedia resmi"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ResolveDecision(id, " WhatsApp lewat penyedia resmi "); err != nil { // same decision: queued once
		t.Fatal(err)
	}
	p = load(t, s)
	if !slices.Equal(p.DecisionSync, []string{id}) ||
		!slices.ContainsFunc(pipeline.Blockers(p), func(b string) bool { return strings.Contains(b, "keputusan "+id+" berubah") }) {
		t.Fatalf("sync %v blockers %v", p.DecisionSync, pipeline.Blockers(p))
	}
	syncDecisions(t, s)
	p = load(t, s)
	mustJob(t, p, work.KindWorkflow, core.JobDone)
	if p.DecisionSync != nil || p.ArtifactRequest != nil || p.Workflow.Request != nil {
		t.Fatalf("after sync: %v %+v %+v", p.DecisionSync, p.ArtifactRequest, p.Workflow.Request)
	}
	if n := runs(t, path); n["blueprint-sync"] != 1 || n["workflow-sync"] != 1 {
		t.Fatalf("runs = %v", n)
	}
	if _, err := s.ApproveStack(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApproveStructure(); err != nil {
		t.Fatal(err)
	}
	if err := s.Finalize(); err != nil {
		t.Fatal(err)
	}
}

// The AI completes the areas that are missing; nothing to complete is
// refused.
func TestCompleteScope(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	designReady(t, s)
	if err := s.CompleteScope(); !errors.Is(err, pipeline.ErrNothingToDo) {
		t.Fatalf("complete with everything covered err = %v", err)
	}
	if _, err := s.RemoveArtifact("siklus-data"); err != nil {
		t.Fatal(err)
	}
	if b := pipeline.Blockers(load(t, s)); !slices.Contains(b, "area Arsitektur & Siklus Hidup Data belum ditulis atau dilewati dengan alasan") {
		t.Fatalf("blockers = %v", b)
	}
	if err := s.CompleteScope(); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p := load(t, s)
	mustJob(t, p, "blueprint", core.JobDone)
	if !slices.ContainsFunc(p.Artifacts, func(a core.Artifact) bool { return a.ID == "data-lifecycle" && a.Area == core.AreaData }) || p.ArtifactRequest != nil {
		t.Fatalf("artifacts = %+v", p.Artifacts)
	}
	if err := approve(s); err != nil {
		t.Fatal(err)
	}
}
