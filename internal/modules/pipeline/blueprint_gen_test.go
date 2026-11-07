package pipeline_test

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/pipeline"
	"github.com/handrawanwdev/foundry/internal/modules/work"
)

// One click after discovery writes remaining designs: Unified Blueprint
// Planning lands technology, architecture and folder structure in one AI
// job. Design and covered areas are skipped because discovery produced them.
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
	if p.Job == nil || p.Job.Status != core.JobDone || p.Job.Kind != work.KindPlan {
		t.Fatalf("last job = %+v", p.Job)
	}
	if p.Stack.Plan.Stale || p.Structure.Plan.Stale || p.Structure.Repo.Stale {
		t.Fatalf("co-designed plans marked stale: tech=%v struct=%v repo=%v", p.Stack.Plan.Stale, p.Structure.Plan.Stale, p.Structure.Repo.Stale)
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

// answerPlanRounds answers the sequential plan rounds, one section each.
func answerPlanRounds(t *testing.T, s *pipeline.Service, w *pipeline.WorkItem, rounds int) {
	t.Helper()
	for i := 0; i < rounds; i++ {
		if w.Kind != work.KindPlan {
			t.Fatalf("round %d: work = %+v", i, w)
		}
		if got := strings.Count(w.Prompt, "SECTIONS TO PRODUCE:"); got != 1 {
			t.Fatalf("round %d: prompt lists sections %d times", i, got)
		}
		if err := s.SubmitWork(work.KindPlan, fakeUnifiedAnswer(w.Prompt, false)); err != nil {
			t.Fatalf("round %d: %v", i, err)
		}
		if i == rounds-1 {
			return
		}
		next, err := s.NextWork()
		if err != nil {
			t.Fatal(err)
		}
		w = next
	}
}

func TestGenerateBlueprintMCP(t *testing.T) {
	s := newService(t, fakeAI{mcp: true}, "")
	designReadyMCP(t, s)
	if err := s.GenerateBlueprint(); err != nil {
		t.Fatal(err)
	}
	w, err := s.NextWork()
	if err != nil || w.Kind != work.KindPlan || !strings.Contains(w.Prompt, "unified blueprint architect") {
		t.Fatalf("first = %+v err=%v", w, err)
	}
	p := load(t, s)
	if p.BlueprintBatch != 1 || p.BlueprintQueue[0] != work.KindPlan {
		t.Fatalf("batch=%d queue=%v", p.BlueprintBatch, p.BlueprintQueue)
	}
	if !strings.Contains(p.Job.Message, "Merancang Blueprint") {
		t.Fatalf("job message = %q", p.Job.Message)
	}
	if len(p.Job.Steps) != 0 {
		t.Fatalf("unified job must not list sub-steps: %+v", p.Job.Steps)
	}
	answerPlanRounds(t, s, w, 3)
	p = load(t, s)
	if p.BlueprintBatch != 0 || p.Structure == nil || p.Structure.Repo == nil || p.Stack == nil || p.Stack.Plan == nil {
		t.Fatalf("batch=%d repo=%v plan=%v job=%+v", p.BlueprintBatch, p.Structure.Repo, p.Stack.Plan, p.Job)
	}
	if p.Structure.State() != core.TechApproved {
		t.Fatalf("structure not adopted: %s", p.Structure.State())
	}
}

func TestGenerateBlueprintRepairsJSON(t *testing.T) {
	s := newService(t, fakeAI{mcp: true}, "")
	designReadyMCP(t, s)
	if err := s.GenerateBlueprint(); err != nil {
		t.Fatal(err)
	}
	w, err := s.NextWork()
	if err != nil {
		t.Fatal(err)
	}
	good := fakeUnifiedAnswer(w.Prompt, false)
	broken := strings.Replace(good, "\n```", ")\n```", 1)
	if err := s.SubmitWork(work.KindPlan, broken); err != nil {
		t.Fatal(err)
	}
	if w, err = s.NextWork(); err != nil {
		t.Fatal(err)
	}
	answerPlanRounds(t, s, w, 2)
	p := load(t, s)
	if p.Stack == nil || p.Stack.Plan == nil || p.Structure == nil || p.Structure.Repo == nil {
		t.Fatalf("repaired JSON did not land: job=%+v", p.Job)
	}
}

func TestGenerateBlueprintEmptyKeepsDecisions(t *testing.T) {
	s := newService(t, fakeAI{mcp: true}, "")
	designReadyMCP(t, s)
	if err := s.GenerateBlueprint(); err != nil {
		t.Fatal(err)
	}
	if err := s.SubmitWork(work.KindPlan, "   "); err == nil {
		t.Fatal("empty output should fail")
	}
	p := load(t, s)
	if p.Stack == nil || p.Stack.PlanRequest == nil {
		t.Fatal("plan request dropped after empty output")
	}
	if p.Stack.Plan != nil {
		t.Fatal("empty output overwrote technology plan")
	}
	if p.Job == nil || p.Job.Status != core.JobWaiting {
		t.Fatalf("mcp job = %+v", p.Job)
	}
}

func TestGenerateBlueprintOneAICall(t *testing.T) {
	runs := t.TempDir() + "/runs"
	t.Setenv("FAKE_RUNS", runs)
	s := newService(t, fakeAI{}, "")
	designReady(t, s)
	if err := os.WriteFile(runs, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.GenerateBlueprint(); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	raw, err := os.ReadFile(runs)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if strings.Count(text, "plan\n") != 3 {
		t.Fatalf("want one plan call per section (tech, architecture, repository), got %q", text)
	}
	for _, old := range []string{"tech\n", "structure\n", "repo\n"} {
		if strings.Contains(text, old) {
			t.Fatalf("standalone %s still ran: %q", strings.TrimSpace(old), text)
		}
	}
}

func TestStartBlueprintChainsUnifiedPlan(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	if _, err := s.SubmitStory("Saya ingin membuat aplikasi klinik untuk janji temu pasien"); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	if err := s.StartBlueprint(); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p := load(t, s)
	if p.Stack == nil || p.Stack.Plan == nil {
		t.Fatalf("tech plan missing after features: %+v", p.Stack)
	}
	if p.Structure == nil || p.Structure.Plan == nil || p.Structure.Repo == nil {
		t.Fatalf("architecture/repo missing after features: %+v", p.Structure)
	}
	if p.Job == nil || p.Job.Kind != work.KindPlan || p.Job.Status != core.JobDone {
		t.Fatalf("last job = %+v", p.Job)
	}
	if err := s.GenerateBlueprint(); !errors.Is(err, pipeline.ErrNothingToDo) {
		t.Fatalf("second generate err = %v", err)
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
	// What was left open can still be built: the same button starts it again.
	if err := s.GenerateBlueprint(); err != nil {
		t.Fatalf("resume after dismiss: %v", err)
	}
	if p := load(t, s); p.Job == nil || p.BlueprintBatch == 0 {
		t.Fatalf("resume started nothing: job=%+v batch=%d", p.Job, p.BlueprintBatch)
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
	holdAutoPlan(t, s)
}

// The progress line names the plan section being designed, round by round.
func TestPlanRoundLabelNamesSection(t *testing.T) {
	s := newService(t, fakeAI{mcp: true}, "")
	designReadyMCP(t, s)
	if err := s.GenerateBlueprint(); err != nil {
		t.Fatal(err)
	}
	w, err := s.NextWork()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Teknologi", "Arsitektur", "Struktur repository"}
	for i, name := range want {
		if msg := load(t, s).Job.Message; msg != "Merancang Blueprint: "+name {
			t.Fatalf("round %d message = %q, want section %q", i, msg, name)
		}
		if err := s.SubmitWork(work.KindPlan, fakeUnifiedAnswer(w.Prompt, false)); err != nil {
			t.Fatal(err)
		}
		if i == len(want)-1 {
			break
		}
		if w, err = s.NextWork(); err != nil {
			t.Fatal(err)
		}
	}
}
