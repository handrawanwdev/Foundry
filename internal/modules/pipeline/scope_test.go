package pipeline_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/pipeline"
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
