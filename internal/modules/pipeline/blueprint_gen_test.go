package pipeline_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/pipeline"
	"github.com/handrawanwdev/foundry/internal/modules/work"
)

// One click after discovery writes the remaining designs in order:
// technology, structure analysis (adopted), folder plan. Design and
// covered areas are skipped because discovery already produced them.
func TestGenerateBlueprintChains(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	designReady(t, s)
	if err := s.GenerateBlueprint(); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p := load(t, s)
	if p.BlueprintBatch != 0 || len(p.BlueprintQueue) != 0 {
		t.Fatalf("batch still open: batch=%d queue=%v job=%+v", p.BlueprintBatch, p.BlueprintQueue, p.Job)
	}
	if p.Stack == nil || p.Stack.Plan == nil {
		t.Fatalf("tech plan missing: %+v", p.Stack)
	}
	if p.Structure == nil || p.Structure.Plan == nil {
		t.Fatalf("structure plan missing: %+v", p.Structure)
	}
	if p.Structure.State() != core.TechApproved {
		t.Fatalf("structure status = %s", p.Structure.State())
	}
	if p.Structure.Repo == nil {
		t.Fatalf("repo plan missing: %+v", p.Structure)
	}
	if p.Job == nil || p.Job.Status != core.JobDone || p.Job.Kind != work.KindRepo {
		t.Fatalf("last job = %+v", p.Job)
	}
	if err := s.GenerateBlueprint(); !errors.Is(err, pipeline.ErrNothingToDo) {
		t.Fatalf("second generate err = %v", err)
	}
}

func TestGenerateBlueprintSkipsDone(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	designReady(t, s)
	if err := s.GenerateTechPlan("", ""); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	if err := s.GenerateBlueprint(); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p := load(t, s)
	if p.Structure == nil || p.Structure.Plan == nil || p.Structure.Repo == nil {
		t.Fatalf("structure/repo missing after skip: %+v", p.Structure)
	}
	if p.BlueprintBatch != 0 {
		t.Fatalf("batch=%d queue=%v", p.BlueprintBatch, p.BlueprintQueue)
	}
}

func TestGenerateBlueprintMCP(t *testing.T) {
	s := newService(t, fakeAI{mcp: true}, "")
	designReadyMCP(t, s)
	if err := s.GenerateBlueprint(); err != nil {
		t.Fatal(err)
	}
	w, err := s.NextWork()
	if err != nil || w.Kind != work.KindTech || !strings.Contains(w.Prompt, "technology architect") {
		t.Fatalf("first = %+v err=%v", w, err)
	}
	p := load(t, s)
	if p.BlueprintBatch < 2 || p.BlueprintQueue[0] != work.KindTech {
		t.Fatalf("batch=%d queue=%v", p.BlueprintBatch, p.BlueprintQueue)
	}
	if !strings.Contains(p.Job.Message, "/") {
		t.Fatalf("job message missing step index: %q", p.Job.Message)
	}
	if err := s.SubmitWork(work.KindTech, fakeTechAnswer(w.Prompt, false)); err != nil {
		t.Fatal(err)
	}
	w, err = s.NextWork()
	if err != nil || w.Kind != work.KindStructure || !strings.Contains(w.Prompt, "software architect") {
		t.Fatalf("second = %+v err=%v", w, err)
	}
	if err := s.SubmitWork(work.KindStructure, fakeStructurePlan(w.Prompt, false)); err != nil {
		t.Fatal(err)
	}
	w, err = s.NextWork()
	if err != nil || w.Kind != work.KindRepo || !strings.Contains(w.Prompt, "repository architect") {
		t.Fatalf("third = %+v err=%v", w, err)
	}
	if load(t, s).Structure.State() != core.TechApproved {
		t.Fatalf("structure not adopted before repo: %s", load(t, s).Structure.State())
	}
	if err := s.SubmitWork(work.KindRepo, fakeRepoAnswer(w.Prompt, false)); err != nil {
		t.Fatal(err)
	}
	p = load(t, s)
	if p.BlueprintBatch != 0 || p.Structure.Repo == nil {
		t.Fatalf("batch=%d repo=%v job=%+v", p.BlueprintBatch, p.Structure.Repo, p.Job)
	}
}

func TestGenerateBlueprintDismissClearsQueue(t *testing.T) {
	s := newService(t, fakeAI{mcp: true}, "")
	designReadyMCP(t, s)
	if err := s.GenerateBlueprint(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Store.Update(func(p *core.Project) error {
		p.Job.Status, p.Job.Error = core.JobFailed, "stopped"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.DismissJob(); err != nil {
		t.Fatal(err)
	}
	p := load(t, s)
	if p.BlueprintBatch != 0 || len(p.BlueprintQueue) != 0 || p.Job != nil {
		t.Fatalf("dismiss left batch: %+v", p)
	}
}

func designReadyMCP(t *testing.T, s *pipeline.Service) {
	t.Helper()
	if _, err := s.SubmitStory("Saya ingin membuat aplikasi klinik untuk janji temu pasien"); err != nil {
		t.Fatal(err)
	}
	if err := s.SubmitWork(work.KindFeatureMap, featureMapJSON); err != nil {
		t.Fatal(err)
	}
	if err := s.StartBlueprint(); err != nil {
		t.Fatal(err)
	}
	if err := s.SubmitWork(work.KindDiscovery, discoveryJSON); err != nil {
		t.Fatal(err)
	}
	if err := s.SubmitWork(work.KindDesign, designJSON); err != nil {
		t.Fatal(err)
	}
}
