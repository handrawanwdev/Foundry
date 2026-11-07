package core

import (
	"strings"
	"testing"
	"time"
)

var bpAt = time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)

func twelveSections() []BlueprintSection {
	var out []BlueprintSection
	for i, id := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l"} {
		s := BlueprintSection{ID: id, Name: strings.ToUpper(id), Kind: "artifacts", Status: BPPending}
		if i >= 10 {
			s.Status, s.SkipKind, s.Reason = BPSkipped, BPSkipNotRequired, "tidak diminta"
		}
		out = append(out, s)
	}
	return out
}

func runTo(t *testing.T, r *BlueprintProgress, id string) string {
	t.Helper()
	exec := r.BeginExecution(id, BPTriggerInitial, "Mengerjakan "+id, "", bpAt)
	if exec == "" {
		t.Fatalf("no execution for %s", id)
	}
	r.BeginValidation(id, "Memvalidasi "+id, bpAt)
	return exec
}

func assertSummary(t *testing.T, r *BlueprintProgress) {
	t.Helper()
	s := r.Summary
	if s.Total != len(r.Sections) {
		t.Fatalf("total %d but list has %d", s.Total, len(r.Sections))
	}
	if s.Applicable != s.Completed+s.Active+s.Pending+s.Blocked+s.Failed {
		t.Fatalf("applicable does not add up: %+v", s)
	}
	if s.Total != s.Applicable+s.Skipped {
		t.Fatalf("total does not add up: %+v", s)
	}
}

// The reported bug: 3/10 above a list of 12 rows.
func TestSummaryMatchesTheList(t *testing.T) {
	r := NewBlueprintProgress("r", twelveSections(), 0, bpAt)
	for _, id := range []string{"a", "b", "c"} {
		runTo(t, r, id)
		r.CompleteSection(id, "", bpAt)
	}
	s := r.Summary
	if s.Completed != 3 || s.Applicable != 10 || s.Total != 12 || s.Skipped != 2 || s.Pending != 7 {
		t.Fatalf("summary %+v", s)
	}
	assertSummary(t, r)
}

func TestOnlyValidatedWorkCounts(t *testing.T) {
	r := NewBlueprintProgress("r", twelveSections(), 0, bpAt)
	r.BeginExecution("a", BPTriggerInitial, "x", "", bpAt)
	if r.Summary.Completed != 0 {
		t.Fatal("a started attempt counted")
	}
	r.BeginValidation("a", "x", bpAt)
	if r.Summary.Completed != 0 || r.Summary.Active != 1 {
		t.Fatalf("validating counted: %+v", r.Summary)
	}
	r.RejectOutput("a", BPFailValidation, "ditolak", bpAt)
	r.FailSection("a", BPFailValidation, "ditolak", bpAt)
	r.BeginExecution("b", BPTriggerInitial, "x", "", bpAt)
	r.SkipSection("c", BPSkipNotRequired, "alasan", bpAt)
	if got := r.Summary; got.Completed != 0 || got.Failed != 1 || got.Skipped != 3 || got.Applicable != 9 {
		t.Fatalf("summary %+v", got)
	}
	assertSummary(t, r)
}

func TestNeverHundredUntilTheRunCloses(t *testing.T) {
	r := NewBlueprintProgress("r", []BlueprintSection{{ID: "a", Name: "A", Status: BPPending}}, 0, bpAt)
	runTo(t, r, "a")
	r.CompleteSection("a", "", bpAt)
	if r.Summary.Percent != 99 {
		t.Fatalf("percent %d before the run closed", r.Summary.Percent)
	}
	r.CompleteRun(bpAt)
	if r.Status != BPRunCompleted || r.Summary.Percent != 100 {
		t.Fatalf("status %s percent %d", r.Status, r.Summary.Percent)
	}
}

func TestRunWithOpenSectionsIsIncomplete(t *testing.T) {
	r := NewBlueprintProgress("r", []BlueprintSection{
		{ID: "a", Name: "A", Status: BPPending}, {ID: "b", Name: "B", Status: BPPending},
	}, 0, bpAt)
	runTo(t, r, "a")
	r.CompleteSection("a", "", bpAt)
	r.CompleteRun(bpAt)
	if r.Status != BPRunIncomplete || r.Summary.Percent != 50 || r.Error == "" {
		t.Fatalf("status %s percent %d error %q", r.Status, r.Summary.Percent, r.Error)
	}
}

func TestDenominatorChangeIsRecorded(t *testing.T) {
	sections := []BlueprintSection{
		{ID: "a", Name: "A", Status: BPPending}, {ID: "b", Name: "B", Status: BPPending},
		{ID: "c", Name: "C", Status: BPPending},
	}
	r := NewBlueprintProgress("r", sections, 0, bpAt)
	next := []BlueprintSection{
		{ID: "a", Name: "A", Status: BPPending},
		{ID: "b", Name: "B", Status: BPSkipped, SkipKind: BPSkipNotApplicable, Reason: "tidak ada integrasi"},
		{ID: "c", Name: "C", Status: BPPending},
	}
	r.ReconcilePlan(next, bpAt)
	if len(r.PlanChanges) != 1 {
		t.Fatalf("plan changes %+v", r.PlanChanges)
	}
	c := r.PlanChanges[0]
	if c.From != 3 || c.To != 2 || !strings.Contains(c.Reason, "tidak ada integrasi") || r.PlanVersion != 2 {
		t.Fatalf("change %+v v%d", c, r.PlanVersion)
	}
	if last := r.Events[len(r.Events)-1]; last.ActivityType != EvPlanChanged {
		t.Fatalf("last event %s", last.ActivityType)
	}
	r.ReconcilePlan(next, bpAt)
	if len(r.PlanChanges) != 1 {
		t.Fatal("same plan recorded twice")
	}
	assertSummary(t, r)
}

func TestSectionsCompletedBySiblingAnswerAreValidatedNotInvented(t *testing.T) {
	r := NewBlueprintProgress("r", []BlueprintSection{
		{ID: "a", Name: "Discovery", Status: BPPending}, {ID: "b", Name: "Use case", Status: BPPending},
	}, 0, bpAt)
	runTo(t, r, "a")
	r.ReconcilePlan([]BlueprintSection{
		{ID: "a", Name: "Discovery", Status: BPCompleted}, {ID: "b", Name: "Use case", Status: BPCompleted},
	}, bpAt)
	b := r.Section("b")
	if b.Status != BPCompleted || len(b.Executions) != 0 || !strings.Contains(b.Tasks[0].Title, "Discovery") {
		t.Fatalf("b %+v", b)
	}
}

func TestRetryIsTraceable(t *testing.T) {
	r := NewBlueprintProgress("r", []BlueprintSection{
		{ID: "ok", Name: "Selesai", Status: BPPending}, {ID: "arch", Name: "Arsitektur", Status: BPPending},
	}, 0, bpAt)
	runTo(t, r, "ok")
	r.CompleteSection("ok", "", bpAt)

	first := runTo(t, r, "arch")
	r.RejectOutput("arch", BPFailValidation, "relasi tidak valid\nmodul ganda", bpAt)
	s := r.Section("arch")
	if s.Status != BPRepairing || s.Validation != BPValFailed || len(s.Issues) != 2 {
		t.Fatalf("after reject %+v", s)
	}
	second := r.BeginExecution("arch", BPTriggerRepair, "Memperbaiki", "relasi tidak valid\nmodul ganda", bpAt)
	s = r.Section("arch")
	if second == "" || second == first {
		t.Fatalf("retry must have a new execution id: %q vs %q", first, second)
	}
	if s.Status != BPRepairing || s.Attempt != 2 {
		t.Fatalf("status %s attempt %d", s.Status, s.Attempt)
	}
	if len(s.Tasks) != 4 || s.Tasks[2].Kind != BPTaskRepair || !strings.Contains(s.Tasks[2].Title, "relasi tidak valid") ||
		strings.Contains(s.Tasks[2].Title, "modul ganda") {
		t.Fatalf("tasks %+v", s.Tasks)
	}
	if s.Tasks[0].Status != BPTaskCompleted || s.Tasks[1].Status != BPTaskFailed {
		t.Fatalf("first attempt tasks %+v", s.Tasks[:2])
	}
	r.BeginValidation("arch", "Memvalidasi ulang", bpAt)
	if r.Section("arch").Status != BPValidating {
		t.Fatal("not validating")
	}
	r.CompleteSection("arch", "", bpAt)
	s = r.Section("arch")
	if s.Executions[0].Status != BPExecRejected || s.Executions[1].Status != BPExecSucceeded || s.Executions[1].Trigger != BPTriggerRepair {
		t.Fatalf("executions %+v", s.Executions)
	}
	if r.Section("ok").Attempt != 1 || r.Section("ok").Status != BPCompleted {
		t.Fatalf("sibling touched: %+v", r.Section("ok"))
	}
	if r.Summary.Retries != 1 || r.Summary.Executions != 3 {
		t.Fatalf("summary %+v", r.Summary)
	}
}

func TestSameTextDifferentExecutionStaysTwoEvents(t *testing.T) {
	r := NewBlueprintProgress("r", []BlueprintSection{{ID: "a", Name: "A", Status: BPPending}}, 0, bpAt)
	for range 3 {
		r.BeginExecution("a", BPTriggerRetry, "Mengulang A", "", bpAt)
		r.WaitRetry("a", "Server AI sibuk", bpAt)
	}
	seen := map[string]bool{}
	ids := map[string]bool{}
	waits := 0
	for _, e := range r.Events {
		if seen[e.EventID] {
			t.Fatalf("duplicate event id %s", e.EventID)
		}
		seen[e.EventID] = true
		if e.ActivityType == EvSectionWaiting {
			waits++
			ids[e.ExecutionID] = true
		}
	}
	if waits != 3 || len(ids) != 3 {
		t.Fatalf("waits %d executions %v", waits, ids)
	}
}

func TestEventsCarryIdentity(t *testing.T) {
	r := NewBlueprintProgress("r", []BlueprintSection{{ID: "a", Name: "A", Status: BPPending}}, 0, bpAt)
	runTo(t, r, "a")
	var last int64
	for _, e := range r.Events {
		if e.EventID == "" || e.RunID != "r" || e.ActivityType == "" || e.Timestamp.IsZero() || e.Seq <= last {
			t.Fatalf("event %+v after %d", e, last)
		}
		last = e.Seq
	}
	v := r.Events[len(r.Events)-1]
	if v.ActivityType != EvValidationStart || v.TaskID == "" || v.ExecutionID == "" || v.Attempt != 1 {
		t.Fatalf("validation event %+v", v)
	}
}

func TestSeqContinuesAcrossRuns(t *testing.T) {
	old := NewBlueprintProgress("r1", []BlueprintSection{{ID: "a", Name: "A", Status: BPPending}}, 0, bpAt)
	runTo(t, old, "a")
	next := NewBlueprintProgress("r2", []BlueprintSection{{ID: "a", Name: "A", Status: BPPending}}, old.Seq, bpAt)
	if next.Seq <= old.Seq || len(next.After(old.Seq)) != 1 {
		t.Fatalf("seq %d after %d", next.Seq, old.Seq)
	}
}

func TestFailedDependencyBlocksDownstreamUntilRetried(t *testing.T) {
	r := NewBlueprintProgress("r", []BlueprintSection{
		{ID: "a", Name: "Data", Status: BPPending},
		{ID: "b", Name: "Keamanan", Status: BPPending},
		{ID: "c", Name: "Teknologi", Status: BPPending, DependsOn: []string{"a", "b"}},
		{ID: "d", Name: "Arsitektur", Status: BPPending, DependsOn: []string{"c"}},
	}, 0, bpAt)
	runTo(t, r, "a")
	r.FailSection("a", BPFailAgent, "agent mati", bpAt)
	for _, id := range []string{"c", "d"} {
		if s := r.Section(id); s.Status != BPBlocked || !strings.Contains(s.Reason, "Menunggu") {
			t.Fatalf("%s %+v", id, s)
		}
	}
	if r.Section("b").Status != BPPending {
		t.Fatal("independent section blocked")
	}
	r.BeginExecution("a", BPTriggerRetry, "Mengulang", "", bpAt)
	if s := r.Section("c"); s.Status != BPPending || s.Reason != "" {
		t.Fatalf("c %+v", s)
	}
	if r.Section("d").Status != BPPending {
		t.Fatalf("d %s", r.Section("d").Status)
	}
	assertSummary(t, r)
}

func TestParallelSectionsKeepIndependentState(t *testing.T) {
	r := NewBlueprintProgress("r", []BlueprintSection{
		{ID: "a", Name: "A", Status: BPPending}, {ID: "b", Name: "B", Status: BPPending},
	}, 0, bpAt)
	ea := r.BeginExecution("a", BPTriggerInitial, "x", "", bpAt)
	eb := r.BeginExecution("b", BPTriggerInitial, "y", "", bpAt)
	if ea == eb || r.Summary.Active != 2 {
		t.Fatalf("%s %s %+v", ea, eb, r.Summary)
	}
	r.BeginValidation("b", "v", bpAt)
	r.RejectOutput("b", BPFailValidation, "buruk", bpAt)
	r.BeginValidation("a", "v", bpAt)
	r.CompleteSection("a", "", bpAt)
	if r.Section("a").Status != BPCompleted || r.Section("b").Status != BPRepairing {
		t.Fatalf("a=%s b=%s", r.Section("a").Status, r.Section("b").Status)
	}
}

func TestStopDefersWhatWasNotReachedAndRecordsIt(t *testing.T) {
	r := NewBlueprintProgress("r", []BlueprintSection{
		{ID: "a", Name: "A", Status: BPPending}, {ID: "b", Name: "B", Status: BPPending},
		{ID: "c", Name: "C", Status: BPPending, DependsOn: []string{"a"}},
	}, 0, bpAt)
	runTo(t, r, "a")
	r.FailSection("a", BPFailAgent, "gagal", bpAt)
	r.FailRun("gagal", bpAt)
	r.StopRun("ditutup pengguna", bpAt)
	if r.Status != BPRunStopped || r.Section("a").Status != BPFailed {
		t.Fatalf("status %s a=%s", r.Status, r.Section("a").Status)
	}
	for _, id := range []string{"b", "c"} {
		if s := r.Section(id); s.SkipKind != BPSkipDeferred || s.Reason == "" {
			t.Fatalf("%s %+v", id, s)
		}
	}
	if len(r.PlanChanges) != 1 || r.PlanChanges[0].To != 1 {
		t.Fatalf("plan changes %+v", r.PlanChanges)
	}
	assertSummary(t, r)
}

func TestRefusalBlocksNothingUntilAttemptsRunOut(t *testing.T) {
	r := NewBlueprintProgress("r", []BlueprintSection{
		{ID: "a", Name: "A", Status: BPPending},
		{ID: "b", Name: "B", Status: BPPending, DependsOn: []string{"a"}},
	}, 0, bpAt)
	runTo(t, r, "a")
	r.RejectOutput("a", BPFailValidation, "buruk", bpAt)
	r.ReconcilePlan([]BlueprintSection{
		{ID: "a", Name: "A", Status: BPPending},
		{ID: "b", Name: "B", Status: BPPending, DependsOn: []string{"a"}},
	}, bpAt)
	if got := r.Section("b").Status; got != BPPending {
		t.Fatalf("b %s while a is being repaired", got)
	}
	r.FailSection("a", BPFailValidation, "buruk", bpAt)
	if got := r.Section("b").Status; got != BPBlocked {
		t.Fatalf("b %s after a failed", got)
	}
}

func TestFailRunClosesWorkInFlight(t *testing.T) {
	r := NewBlueprintProgress("r", []BlueprintSection{{ID: "a", Name: "A", Status: BPPending}}, 0, bpAt)
	r.BeginExecution("a", BPTriggerInitial, "x", "", bpAt)
	r.FailRun("server mati", bpAt)
	s := r.Section("a")
	if s.Status != BPFailed || s.FailureKind != BPFailInterrupted || s.Executions[0].Status != BPExecFailed || s.Executions[0].EndedAt == nil {
		t.Fatalf("%+v", s)
	}
}

func TestCompletedSectionIsNotReopenedByLateFailure(t *testing.T) {
	r := NewBlueprintProgress("t", []BlueprintSection{{ID: "a", Name: "A", Status: BPPending}}, 0, bpAt)
	runTo(t, r, "a")
	r.CompleteSection("a", "", bpAt)
	r.BeginExecution("a", BPTriggerInitial, "again", "", bpAt)
	r.FailSection("a", BPFailAgent, "x", bpAt)
	r.RejectOutput("a", BPFailValidation, "x", bpAt)
	if r.Section("a").Status != BPCompleted {
		t.Fatalf("status %s", r.Section("a").Status)
	}
}

func TestEstimateNeedsRealDurations(t *testing.T) {
	r := NewBlueprintProgress("r", []BlueprintSection{
		{ID: "a", Name: "A", Kind: "artifacts", Status: BPPending},
		{ID: "b", Name: "B", Kind: "plan", Status: BPPending},
	}, 0, bpAt)
	if eta := r.Estimate(nil, false, bpAt); eta != nil {
		t.Fatalf("eta from nothing: %+v", eta)
	}
	one := []JobTiming{{Kind: "artifacts", MS: 60000, At: bpAt}}
	if eta := r.Estimate(one, false, bpAt); eta != nil {
		t.Fatalf("eta from one sample: %+v", eta)
	}
	two := append(one, JobTiming{Kind: "artifacts", MS: 60000, At: bpAt})
	eta := r.Estimate(two, false, bpAt)
	if eta == nil || eta.RemainingMS != 60000 || eta.Samples != 2 || eta.UnknownSections != 1 {
		t.Fatalf("eta %+v", eta)
	}
	if got := r.Estimate(two, true, bpAt); got != nil {
		t.Fatalf("eta while a host drives: %+v", got)
	}
	r.BeginExecution("a", BPTriggerInitial, "x", "", bpAt)
	if got := r.Estimate(two, false, bpAt.Add(45*time.Second)); got == nil || got.RemainingMS != 15000 {
		t.Fatalf("elapsed not subtracted: %+v", got)
	}
	if got := r.Estimate(two, false, bpAt.Add(2*time.Minute)); got != nil {
		t.Fatalf("eta after the work outran its history: %+v", got)
	}
	r.FailRun("x", bpAt)
	if got := r.Estimate(two, false, bpAt); got != nil {
		t.Fatalf("eta on a failed run: %+v", got)
	}
}
