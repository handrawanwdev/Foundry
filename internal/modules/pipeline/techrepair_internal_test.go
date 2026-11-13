package pipeline

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/techplan"
	"github.com/handrawanwdev/foundry/internal/modules/work"
)

// refusal is a validator refusal with n issues on n areas.
func refusal(n int) error {
	plan := &core.TechPlan{}
	var issues []core.TechIssue
	for i := range n {
		area := fmt.Sprintf("area_%d", i)
		plan.Items = append(plan.Items, core.TechItem{Area: area, Technology: "T"})
		issues = append(issues, core.TechIssue{Level: "error", Area: area, Code: techplan.CodeAltIncomplete, Message: "alternatif belum lengkap"})
	}
	return &techplan.RejectError{Plan: plan, Issues: issues}
}

func techProject() *core.Project {
	return &core.Project{Stack: &core.TechStack{PlanRequest: &core.TechPlanRequest{Action: core.TechPlanGenerate}}}
}

func TestRetryContinuesWhileErrorsDrop(t *testing.T) {
	p := techProject()
	for i, n := range []int{41, 12, 2} {
		if !recordTechAttempt(p, work.KindTech, refusal(n), "haiku", "sonnet", 5) {
			t.Fatalf("attempt %d (%d errors) stopped the chain", i+1, n)
		}
	}
	r := p.Stack.PlanRequest
	if r.Model != "" {
		t.Fatalf("escalated while making progress: %q", r.Model)
	}
	want := []string{core.TechProgressFirst, core.TechProgressImproved, core.TechProgressImproved}
	for i, a := range r.Attempts {
		if a.Progress != want[i] {
			t.Fatalf("attempt %d progress = %s", i+1, a.Progress)
		}
	}
	if len(r.Issues) != 2 || r.Draft == nil || len(r.Draft.Items) != 2 {
		t.Fatalf("draft not the latest best: issues=%d", len(r.Issues))
	}
}

func TestRetryWithoutProgressEscalatesOnceThenStops(t *testing.T) {
	p := techProject()
	r := p.Stack.PlanRequest
	recordTechAttempt(p, work.KindTech, refusal(41), "haiku", "sonnet", 5)
	if !recordTechAttempt(p, work.KindTech, refusal(41), "haiku", "sonnet", 5) {
		t.Fatal("a stalled chain with a stronger model available stopped")
	}
	if r.Model != "sonnet" || r.EscalatedFrom != "haiku" || r.Escalation == "" {
		t.Fatalf("escalation = %q from %q (%q)", r.Model, r.EscalatedFrom, r.Escalation)
	}
	if recordTechAttempt(p, work.KindTech, refusal(41), "haiku", "sonnet", 5) {
		t.Fatal("the escalated model stalled too, but the chain went on")
	}
	if got := r.Attempts[2].Model; got != "sonnet" {
		t.Fatalf("attempt 3 model = %q", got)
	}
}

func TestRetryNoEscalationForOneFieldOnFirstRefusal(t *testing.T) {
	p := techProject()
	if !recordTechAttempt(p, work.KindTech, refusal(1), "haiku", "sonnet", 5) || p.Stack.PlanRequest.Model != "" {
		t.Fatal("the first refusal of one empty field escalated or stopped")
	}
}

func TestRetryRegressionKeepsBetterDraft(t *testing.T) {
	p := techProject()
	r := p.Stack.PlanRequest
	recordTechAttempt(p, work.KindTech, refusal(3), "", "", 5)
	best := r.Draft
	if recordTechAttempt(p, work.KindTech, refusal(9), "", "", 5) {
		t.Fatal("a regression without escalation continued")
	}
	if r.Draft != best || len(r.Issues) != 3 || r.Attempts[1].Progress != core.TechProgressRegressed {
		t.Fatalf("draft replaced by a worse one: issues=%d progress=%s", len(r.Issues), r.Attempts[1].Progress)
	}
}

func TestRetryStopsAtAttemptLimitAndUserRetryStartsNewChain(t *testing.T) {
	p := techProject()
	r := p.Stack.PlanRequest
	recordTechAttempt(p, work.KindTech, refusal(10), "", "", 2)
	if recordTechAttempt(p, work.KindTech, refusal(5), "", "", 2) {
		t.Fatal("the attempt limit was passed")
	}
	newTechChain(p)
	if !recordTechAttempt(p, work.KindTech, refusal(5), "", "", 2) || r.Attempts[2].Progress != core.TechProgressFirst {
		t.Fatalf("a user retry did not start a new chain: %+v", r.Attempts[2])
	}
	if len(r.Attempts) != 3 {
		t.Fatal("history lost")
	}
}

func TestUnreadableAnswersStall(t *testing.T) {
	p := techProject()
	parse := errors.New("output AI tidak berisi JSON yang valid (tidak ada objek JSON)")
	recordTechAttempt(p, work.KindTech, parse, "", "", 5)
	if recordTechAttempt(p, work.KindTech, parse, "", "", 5) {
		t.Fatal("two unreadable answers in a row continued without a new strategy")
	}
}

func TestDiagnosisReportsAttemptsModelAndIssues(t *testing.T) {
	p := techProject()
	recordTechAttempt(p, work.KindTech, refusal(41), "haiku", "sonnet", 5)
	err := refusal(41)
	recordTechAttempt(p, work.KindTech, err, "haiku", "sonnet", 5)
	got := techDiagnosis(p, work.KindTech, err, true)
	for _, want := range []string{
		"Rencana teknologi ditolak validator: 41 masalah (alternatif belum lengkap 41 di 41 area)",
		"Percobaan 2", "tidak ada progres", "error per percobaan: 41 → 41",
		"Eskalasi haiku → sonnet", "Disimpan: 0 keputusan tervalidasi", "Status: gagal",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("diagnosis misses %q:\n%s", want, got)
		}
	}
}

// A refusal retries on its own while it names a new problem, up to the
// bound; the same problem twice stops.
func TestAutoRetryWhileRefusalChanges(t *testing.T) {
	r := &core.ChangeRequest{}
	for i, c := range []struct {
		msg  string
		want bool
	}{{"coverage", true}, {"coverage", false}, {"subfitur", true}, {"cycle", true}, {"lain", false}} {
		if got := autoRetry(r, c.msg); got != c.want {
			t.Fatalf("%d %q: retry=%v", i, c.msg, got)
		}
		r.Rejected = c.msg
	}
	// After "Coba lagi" the request keeps its last refusal: a new one retries.
	r = &core.ChangeRequest{Rejected: "coverage"}
	if !autoRetry(r, "subfitur") || autoRetry(&core.ChangeRequest{Rejected: "coverage"}, "coverage") {
		t.Fatal("retry after Coba lagi")
	}
}

// Repository, structure and feature map requests follow the same retry rule
// as the change requests, and "Coba lagi" resets their budget too.
func TestEveryRequestSharesTheRetryRule(t *testing.T) {
	repo := &core.RepoRequest{Action: core.RepoRevise}
	for i, c := range []struct {
		msg  string
		want bool
	}{{"path a", true}, {"path a", false}, {"path b", true}, {"cycle", true}, {"lain", false}} {
		if got := retryBudget(&repo.Retries, repo.Rejected, c.msg); got != c.want {
			t.Fatalf("%d %q: retry=%v", i, c.msg, got)
		}
		repo.Rejected = c.msg
	}
	p := &core.Project{
		Job:        &core.Job{Kind: work.KindRepo, Status: core.JobFailed},
		Structure:  &core.Structure{RepoRequest: repo, PlanRequest: &core.StructurePlanRequest{Retries: 2}},
		FeatureMap: &core.FeatureMap{Request: &core.FeatureMapRequest{Retries: 3}},
	}
	if err := failedJob(work.KindRepo)(p); err != nil || repo.Retries != 0 {
		t.Fatalf("repo reset: %v retries=%d", err, repo.Retries)
	}
	for kind, n := range map[string]*int{work.KindStructure: &p.Structure.PlanRequest.Retries, work.KindFeatureMap: &p.FeatureMap.Request.Retries} {
		p.Job.Kind = kind
		if err := failedJob(kind)(p); err != nil || *n != 0 {
			t.Fatalf("%s reset: %v retries=%d", kind, err, *n)
		}
	}
}
