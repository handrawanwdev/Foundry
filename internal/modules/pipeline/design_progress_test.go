package pipeline_test

import (
	"strings"
	"testing"
	"time"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/pipeline"
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

// assertRun holds the numbers against the list and the timeline.
func assertRun(t *testing.T, p *core.Project) {
	t.Helper()
	r := p.BlueprintProgress
	s := r.Summary
	if s.Total != len(r.Sections) {
		t.Fatalf("total %d but %d sections", s.Total, len(r.Sections))
	}
	count := map[string]int{}
	for _, sec := range r.Sections {
		count[sec.Status]++
		if sec.Status == core.BPSkipped && (sec.SkipKind == "" || sec.Reason == "") {
			t.Fatalf("skipped without a reason: %+v", sec)
		}
		if sec.Status == core.BPCompleted && sec.Validation != core.BPValPassed {
			t.Fatalf("completed without validation: %+v", sec)
		}
	}
	if count[core.BPCompleted] != s.Completed || count[core.BPSkipped] != s.Skipped {
		t.Fatalf("summary %+v, list %v", s, count)
	}
	if s.Applicable != s.Completed+s.Active+s.Pending+s.Blocked+s.Failed {
		t.Fatalf("applicable does not add up: %+v", s)
	}
	seen := map[string]bool{}
	var last int64
	for _, e := range r.Events {
		if seen[e.EventID] || e.Seq <= last || e.RunID != r.RunID {
			t.Fatalf("event out of line: %+v after %d", e, last)
		}
		seen[e.EventID], last = true, e.Seq
	}
	if from := s.Applicable; len(r.PlanChanges) > 0 && r.PlanChanges[len(r.PlanChanges)-1].To != from {
		t.Fatalf("denominator %d moved without a record: %+v", from, r.PlanChanges)
	}
}

func eventTypes(p *core.Project) []string {
	var out []string
	for _, e := range p.BlueprintProgress.Events {
		out = append(out, e.ActivityType)
	}
	return out
}

func storyToBlueprint(t *testing.T, s *pipeline.Service) {
	t.Helper()
	if _, err := s.SubmitStory("Aplikasi klinik untuk janji temu pasien"); err != nil {
		t.Fatal(err)
	}
}

func TestProgressFullRunOfAnAgent(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	p := designReadyKeepPlan(t, s)
	assertRun(t, p)
	r := p.BlueprintProgress
	if r.Status != core.BPRunCompleted || r.Summary.Percent != 100 {
		t.Fatalf("status %s %+v\n%v", r.Status, r.Summary, eventTypes(p))
	}
	if r.Summary.Completed != r.Summary.Applicable {
		t.Fatalf("%+v", r.Summary)
	}
	if r.Summary.Total != 12 {
		t.Fatalf("the blueprint has 12 sections, list has %d", r.Summary.Total)
	}
	if r.EndedAt == nil {
		t.Fatal("run has no end")
	}
}

func TestProgressListIsFixedFromTheFirstEvent(t *testing.T) {
	s := newService(t, fakeAI{mcp: true}, "")
	storyToBlueprint(t, s)
	mcpAfterStory(t, s)
	p := load(t, s)
	r := p.BlueprintProgress
	if r.Summary.Total != 12 {
		t.Fatalf("list %d before discovery", r.Summary.Total)
	}
	if got := r.Section(core.AreaData); got.Status != core.BPPending || got.Reason == "" {
		t.Fatalf("data before discovery: %+v", got)
	}
	before := r.Summary.Applicable
	if err := s.SubmitWork(work.KindDiscovery, discoveryJSON); err != nil {
		t.Fatal(err)
	}
	p = load(t, s)
	assertRun(t, p)
	r = p.BlueprintProgress
	if r.Summary.Total != 12 {
		t.Fatalf("list changed length: %d", r.Summary.Total)
	}
	if r.Summary.Applicable != before && len(r.PlanChanges) == 0 {
		t.Fatalf("denominator %d to %d with no record", before, r.Summary.Applicable)
	}
}

func TestProgressFailedValidationKeepsTheSectionOpen(t *testing.T) {
	s := newService(t, fakeAI{mcp: true}, "")
	storyToBlueprint(t, s)
	mcpAfterStory(t, s)
	if err := s.SubmitWork(work.KindDiscovery, "bukan json"); err == nil {
		t.Fatal("invalid json accepted")
	}
	p := load(t, s)
	assertRun(t, p)
	sec := p.BlueprintProgress.Section("discovery")
	if sec.Status == core.BPCompleted || sec.Attempt != 2 || sec.Status != core.BPRepairing {
		t.Fatalf("discovery after refusal: %+v", sec)
	}
	first, second := sec.Executions[0], sec.Executions[1]
	if first.Status != core.BPExecRejected || first.FailureKind != core.BPFailOutput || second.ID == first.ID || second.Trigger != core.BPTriggerRepair {
		t.Fatalf("executions %+v", sec.Executions)
	}
	if err := s.SubmitWork(work.KindDiscovery, discoveryJSON); err != nil {
		t.Fatal(err)
	}
	p = load(t, s)
	assertRun(t, p)
	sec = p.BlueprintProgress.Section("discovery")
	if sec.Status != core.BPCompleted || sec.Attempt != 2 || sec.Executions[1].Status != core.BPExecSucceeded {
		t.Fatalf("discovery after repair: %+v", sec)
	}
}

func TestProgressAgentRepairOnlyTouchesTheRefusedSection(t *testing.T) {
	s := newService(t, fakeAI{}, "design_invalid_once")
	p := designReadyKeepPlan(t, s)
	assertRun(t, p)
	ui := p.BlueprintProgress.Section(core.AreaUI)
	if ui.Status != core.BPCompleted || ui.Attempt < 2 {
		t.Fatalf("ui %+v", ui)
	}
	if ui.Executions[0].Status != core.BPExecRejected || ui.Executions[1].Trigger != core.BPTriggerRepair {
		t.Fatalf("executions %+v", ui.Executions)
	}
	var repair *core.BlueprintTask
	for i := range ui.Tasks {
		if ui.Tasks[i].Kind == core.BPTaskRepair {
			repair = &ui.Tasks[i]
		}
	}
	if repair == nil || !strings.HasPrefix(repair.Title, "Memperbaiki output: ") || repair.Title == "Memperbaiki output: " {
		t.Fatalf("no specific repair task: %+v", ui.Tasks)
	}
	if got := p.BlueprintProgress.Section("discovery").Attempt; got != 1 {
		t.Fatalf("discovery rerun: %d", got)
	}
	if p.BlueprintProgress.Status != core.BPRunCompleted {
		t.Fatalf("run %s", p.BlueprintProgress.Status)
	}
}

func TestProgressAreaThatDoesNotAnswerItsOwnAreaIsRefused(t *testing.T) {
	s := newService(t, fakeAI{}, "area_empty_once")
	p := designReadyKeepPlan(t, s)
	assertRun(t, p)
	r := p.BlueprintProgress
	found := false
	for _, sec := range r.Sections {
		for _, e := range sec.Executions {
			if e.Status == core.BPExecRejected && strings.Contains(e.Error, "tidak menulis area") {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("no refusal recorded\n%v", eventTypes(p))
	}
	if r.Status != core.BPRunCompleted {
		t.Fatalf("run %s %+v", r.Status, r.Summary)
	}
}

func TestProgressParallelAreasOverlapAndLandSeparately(t *testing.T) {
	t.Setenv("FAKE_AREA_MS", "150")
	s := newService(t, fakeAI{}, "areas_open")
	p := designReadyKeepPlan(t, s)
	assertRun(t, p)
	r := p.BlueprintProgress
	if r.Parallel < 2 {
		t.Fatalf("parallel %d", r.Parallel)
	}
	active, peak := 0, 0
	areas := map[string]bool{}
	for _, e := range r.Events {
		a := core.AreaByID(e.SectionID)
		if a == nil || a.Builtin {
			continue
		}
		switch e.ActivityType {
		case core.EvSectionStarted:
			areas[e.SectionID] = true
			active++
		case core.EvSectionCompleted, core.EvSectionSkipped, core.EvSectionFailed:
			active--
		}
		peak = max(peak, active)
	}
	if peak < 2 {
		t.Fatalf("no overlap across %v: peak %d\n%v", areas, peak, eventTypes(p))
	}
	if peak > r.Parallel {
		t.Fatalf("peak %d above limit %d", peak, r.Parallel)
	}
	execs := map[string]bool{}
	for _, sec := range r.Sections {
		for _, e := range sec.Executions {
			if execs[e.ID] {
				t.Fatalf("execution id reused: %s", e.ID)
			}
			execs[e.ID] = true
		}
	}
}

func TestProgressFailedAreaKeepsSiblingsAndRetriesOnlyItself(t *testing.T) {
	s := newService(t, fakeAI{}, "area_data_fail")
	if _, err := s.SubmitStory("Saya ingin membuat aplikasi klinik untuk janji temu pasien"); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	if err := s.StartBlueprint(); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p := load(t, s)
	assertRun(t, p)
	r := p.BlueprintProgress
	if r.Status != core.BPRunFailed || p.Job == nil || p.Job.Status != core.JobFailed {
		t.Fatalf("run %s job %+v\n%v", r.Status, p.Job, eventTypes(p))
	}
	data := r.Section(core.AreaData)
	if data.Status != core.BPFailed || data.Error == "" {
		t.Fatalf("data %+v", data)
	}
	var kept []string
	attempts := map[string]int{}
	for _, sec := range r.Sections {
		attempts[sec.ID] = sec.Attempt
		if a := core.AreaByID(sec.ID); a != nil && !a.Builtin && sec.Status == core.BPCompleted {
			kept = append(kept, sec.ID)
		}
	}
	if len(kept) == 0 {
		t.Fatalf("no sibling landed: %+v", r.Sections)
	}
	if st := r.Section("tech").Status; st != core.BPBlocked {
		t.Fatalf("tech %s, want blocked behind the failed area", st)
	}

	t.Setenv("FAKE_MODE", "")
	if err := s.RetryJob(); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p = load(t, s)
	assertRun(t, p)
	r = p.BlueprintProgress
	if r.Status != core.BPRunCompleted {
		t.Fatalf("after retry: %s %+v\n%v", r.Status, r.Summary, eventTypes(p))
	}
	for _, id := range kept {
		if got := r.Section(id).Attempt; got != attempts[id] {
			t.Fatalf("%s regenerated: %d attempts, was %d", id, got, attempts[id])
		}
	}
	data = r.Section(core.AreaData)
	last := data.Executions[len(data.Executions)-1]
	if data.Status != core.BPCompleted || last.Trigger == core.BPTriggerInitial {
		t.Fatalf("data %+v", data)
	}
}

func TestProgressDismissDefersWithReason(t *testing.T) {
	s := newService(t, fakeAI{}, "area_data_fail")
	if _, err := s.SubmitStory("Saya ingin membuat aplikasi klinik untuk janji temu pasien"); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	if err := s.StartBlueprint(); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	if err := s.DismissJob(); err != nil {
		t.Fatal(err)
	}
	p := load(t, s)
	assertRun(t, p)
	r := p.BlueprintProgress
	if r.Status != core.BPRunStopped {
		t.Fatalf("run %s", r.Status)
	}
	sec := r.Section("repo")
	if sec.Status != core.BPSkipped || sec.SkipKind != core.BPSkipDeferred || sec.Reason == "" {
		t.Fatalf("repo %+v", sec)
	}
}

func TestProgressBusyServerGivesEveryAttemptItsOwnExecution(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	s.RetryWait = []time.Duration{time.Millisecond, time.Millisecond}
	if _, err := s.SubmitStory("Saya ingin membuat aplikasi klinik untuk janji temu pasien"); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	t.Setenv("FAKE_MODE", "overloaded_once")
	t.Setenv("FAKE_RUNS", t.TempDir()+"/runs")
	if err := s.StartBlueprint(); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p := load(t, s)
	assertRun(t, p)
	sec := p.BlueprintProgress.Section("discovery")
	if sec.Status != core.BPCompleted || len(sec.Executions) != 2 {
		t.Fatalf("discovery %+v", sec)
	}
	first, second := sec.Executions[0], sec.Executions[1]
	if first.FailureKind != core.BPFailOverloaded || first.Status != core.BPExecFailed ||
		second.Trigger != core.BPTriggerBusyRetry || second.Attempt != 2 || second.ID == first.ID {
		t.Fatalf("executions %+v", sec.Executions)
	}
	waits := 0
	for _, e := range p.BlueprintProgress.Events {
		if e.ActivityType == core.EvSectionWaiting {
			waits++
		}
	}
	if waits != 1 {
		t.Fatalf("waits %d", waits)
	}
}

func TestProgressCrashIsVisibleAndRetryable(t *testing.T) {
	s := newService(t, fakeAI{mcp: true}, "")
	storyToBlueprint(t, s)
	mcpAfterStory(t, s)
	if _, err := s.Store.Update(func(p *core.Project) error {
		p.Job.Status = core.JobRunning
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecoverStale(); err != nil {
		t.Fatal(err)
	}
	p := load(t, s)
	assertRun(t, p)
	r := p.BlueprintProgress
	sec := r.Section("discovery")
	if r.Status != core.BPRunFailed || sec.Status != core.BPFailed || sec.FailureKind != core.BPFailInterrupted {
		t.Fatalf("run %s section %+v", r.Status, sec)
	}
}

func TestProgressRunLeftWithoutAJobDoesNotSpinForever(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	p := designReadyKeepPlan(t, s)
	if _, err := s.Store.Update(func(p *core.Project) error {
		p.BlueprintProgress.Status = core.BPRunRunning
		p.BlueprintProgress.EndedAt = nil
		p.Job = nil
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecoverStale(); err != nil {
		t.Fatal(err)
	}
	p = load(t, s)
	if p.BlueprintProgress.Status == core.BPRunRunning {
		t.Fatalf("still running")
	}
}

func TestProgressRevisionAfterARunLeavesItAlone(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	p := designReadyKeepPlan(t, s)
	seq := p.BlueprintProgress.Seq
	if err := s.ReviseArtifacts("Tambahkan catatan retensi", nil); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p = load(t, s)
	if p.BlueprintProgress.Seq != seq || p.BlueprintProgress.Status != core.BPRunCompleted {
		t.Fatalf("a revision moved the run: seq %d to %d, %s", seq, p.BlueprintProgress.Seq, p.BlueprintProgress.Status)
	}
}

func TestProgressSecondRunCountsOnlyItsOwnWork(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	p := designReadyKeepPlan(t, s)
	seq := p.BlueprintProgress.Seq
	if _, err := s.Store.Update(func(p *core.Project) error {
		p.Stack.Plan = nil
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.GenerateBlueprint(); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p = load(t, s)
	assertRun(t, p)
	r := p.BlueprintProgress
	if r.Seq <= seq || r.Events[0].Seq != seq+1 {
		t.Fatalf("seq restarted: %d after %d", r.Events[0].Seq, seq)
	}
	if sec := r.Section("discovery"); sec.SkipKind != core.BPSkipAlreadySatisfied || sec.Reason == "" {
		t.Fatalf("discovery %+v", sec)
	}
	if r.Status != core.BPRunCompleted || r.Section("tech").Status != core.BPCompleted {
		t.Fatalf("run %s tech %s", r.Status, r.Section("tech").Status)
	}
	if r.Summary.Applicable >= r.Summary.Total {
		t.Fatalf("satisfied work counted: %+v", r.Summary)
	}
}

func TestProgressSnapshotOmitsTheTimelineAndStreamHasIt(t *testing.T) {
	s := newService(t, fakeAI{mcp: true}, "")
	storyToBlueprint(t, s)
	mcpAfterStory(t, s)
	p, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(p.BlueprintProgress.Events) != 0 {
		t.Fatal("project snapshot carries the timeline")
	}
	bp, events, err := s.BlueprintProgress(0)
	if err != nil || bp == nil || len(events) == 0 {
		t.Fatalf("progress %v events %d err %v", bp, len(events), err)
	}
	later, _, _ := s.BlueprintProgress(bp.Seq)
	if later.Seq != bp.Seq {
		t.Fatal("cursor moved the state")
	}
	if _, rest, _ := s.BlueprintProgress(bp.Seq); len(rest) != 0 {
		t.Fatalf("replayed %d", len(rest))
	}
}
