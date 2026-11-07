package pipeline_test

import (
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/work"
)

func sectionStatus(p *core.Project, id string) string {
	if p.BlueprintProgress == nil {
		return ""
	}
	s := p.BlueprintProgress.Section(id)
	if s == nil {
		return "missing"
	}
	return s.Status
}

func TestBlueprintProgressFollowsAILifecycle(t *testing.T) {
	s := newService(t, fakeAI{mcp: true}, "")
	if _, err := s.SubmitStory("Aplikasi klinik untuk janji temu pasien"); err != nil {
		t.Fatal(err)
	}
	if err := s.SubmitWork(work.KindFeatureMap, featureMapJSON); err != nil {
		t.Fatal(err)
	}
	if err := s.StartBlueprint(); err != nil {
		t.Fatal(err)
	}
	p := load(t, s)
	if p.BlueprintProgress == nil || p.BlueprintProgress.Status != core.BPTaskRunning {
		t.Fatalf("progress = %+v", p.BlueprintProgress)
	}
	if sectionStatus(p, "discovery") != core.BPInProgress {
		t.Fatalf("discovery = %s", sectionStatus(p, "discovery"))
	}
	if p.BlueprintProgress.Activity == "" {
		t.Fatal("empty activity")
	}
	if n := len(p.BlueprintProgress.Sections); n < 2 {
		t.Fatalf("sections %d, want dynamic > 1", n)
	}
	done, total := p.BlueprintProgress.Counts()
	if done != 0 || total == 0 {
		t.Fatalf("counts %d/%d", done, total)
	}

	if err := s.SubmitWork(work.KindDiscovery, "bukan json"); err == nil {
		t.Fatal("invalid json accepted")
	}
	p = load(t, s)
	if sectionStatus(p, "discovery") == core.BPCompleted {
		t.Fatal("discovery completed on invalid output")
	}

	if err := s.SubmitWork(work.KindDiscovery, discoveryJSON); err != nil {
		t.Fatal(err)
	}
	p = load(t, s)
	if sectionStatus(p, "discovery") != core.BPCompleted {
		t.Fatalf("discovery after apply = %s", sectionStatus(p, "discovery"))
	}
	if p.BlueprintProgress.Seq < 2 {
		t.Fatalf("seq %d", p.BlueprintProgress.Seq)
	}
	seen := map[int64]bool{}
	for _, e := range p.BlueprintProgress.Events {
		if seen[e.Seq] {
			t.Fatalf("duplicate seq %d", e.Seq)
		}
		seen[e.Seq] = true
	}
	if sectionStatus(p, "discovery") != core.BPCompleted {
		t.Fatal("lost discovery after later work")
	}
}

func TestBlueprintProgressRetryKeepsCompleted(t *testing.T) {
	s := newService(t, fakeAI{mcp: true}, "")
	if _, err := s.SubmitStory("Aplikasi klinik untuk janji temu pasien"); err != nil {
		t.Fatal(err)
	}
	mcpAfterStory(t, s)
	if err := s.SubmitWork(work.KindDiscovery, discoveryJSON); err != nil {
		t.Fatal(err)
	}
	p := load(t, s)
	if sectionStatus(p, "discovery") != core.BPCompleted {
		t.Fatalf("discovery = %s", sectionStatus(p, "discovery"))
	}
	w, err := s.NextWork()
	if err != nil {
		t.Fatal(err)
	}
	if w.Kind == "none" {
		t.Fatal("expected remaining blueprint work")
	}
	if err := s.SubmitWork(w.Kind, "bukan json"); err == nil {
		t.Fatal("invalid json accepted")
	}
	p = load(t, s)
	if sectionStatus(p, "discovery") != core.BPCompleted {
		t.Fatal("retry dropped completed discovery")
	}
	cur := sectionStatus(p, sectionForCurrent(p, w.Kind))
	if cur != core.BPRetrying && cur != core.BPInProgress && cur != core.BPFailed {
		t.Fatalf("current section %s", cur)
	}
}

func sectionForCurrent(p *core.Project, kind string) string {
	switch kind {
	case work.KindDiscovery:
		return "discovery"
	case work.KindDesign:
		return core.AreaUI
	case work.KindArtifacts:
		if p.ArtifactRequest != nil {
			return p.ArtifactRequest.Area
		}
	case work.KindTech:
		return "tech"
	case work.KindStructure:
		return "arch"
	case work.KindRepo:
		return "repo"
	case work.KindPlan:
		switch work.CurrentPlanPart(p) {
		case work.KindTech:
			return "tech"
		case work.KindStructure:
			return "arch"
		case work.KindRepo:
			return "repo"
		}
	}
	return ""
}

func TestBlueprintProgressNoFakePercent(t *testing.T) {
	s := newService(t, fakeAI{mcp: true}, "")
	if _, err := s.SubmitStory("Aplikasi klinik untuk janji temu pasien"); err != nil {
		t.Fatal(err)
	}
	mcpAfterStory(t, s)
	p := load(t, s)
	done, total := p.BlueprintProgress.Counts()
	if done != 0 {
		t.Fatalf("started job counted as done: %d/%d", done, total)
	}
	if err := s.SubmitWork(work.KindDiscovery, discoveryJSON); err != nil {
		t.Fatal(err)
	}
	p = load(t, s)
	done2, total2 := p.BlueprintProgress.Counts()
	if done2 <= done {
		t.Fatalf("completed did not rise: %d -> %d", done, done2)
	}
	if total2 == 0 {
		t.Fatal("total hardcoded empty")
	}
}

func TestGenerateBlueprintProgressTask(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	designReady(t, s)
	if err := s.GenerateBlueprint(); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p := load(t, s)
	if p.BlueprintProgress == nil {
		t.Fatal("no progress")
	}
	if p.BlueprintProgress.Status != core.BPTaskCompleted && p.Job != nil && p.Job.Status == core.JobDone {
		// queue finished; task should complete
		if p.BlueprintBatch != 0 {
			t.Fatalf("batch still open: %+v", p.BlueprintProgress)
		}
	}
	if st := sectionStatus(p, "tech"); st != core.BPCompleted && p.Stack != nil && p.Stack.Plan != nil {
		t.Fatalf("tech landed but status=%s", st)
	}
	if strings.Contains(p.BlueprintProgress.Activity, "fake") {
		t.Fatalf("activity looks simulated: %s", p.BlueprintProgress.Activity)
	}
}

func TestBlueprintProgressEventsHaveTaskId(t *testing.T) {
	s := newService(t, fakeAI{mcp: true}, "")
	if _, err := s.SubmitStory("Aplikasi klinik untuk janji temu pasien"); err != nil {
		t.Fatal(err)
	}
	mcpAfterStory(t, s)
	p := load(t, s)
	if p.BlueprintProgress.TaskID == "" {
		t.Fatal("empty task id")
	}
	for _, e := range p.BlueprintProgress.Events {
		if e.TaskID != p.BlueprintProgress.TaskID {
			t.Fatalf("event taskId %s want %s", e.TaskID, p.BlueprintProgress.TaskID)
		}
		if e.Event == "" || e.Seq == 0 {
			t.Fatalf("event %+v", e)
		}
	}
}
