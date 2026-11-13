package techplan

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/stack"
)

var (
	ErrNoPlan = errors.New("rencana teknologi belum ada")
	ErrStale  = errors.New("sumber rencana teknologi berubah; revisi bagian yang terdampak dulu")
)

// Decisions are the stack decisions of the plan's items in the areas
// Foundry knows. The user's own choices in old stay on top of them, and a
// technology the code already uses stays marked as existing.
func Decisions(plan *core.TechPlan, old *core.TechStack) []core.TechDecision {
	var out []core.TechDecision
	for _, it := range plan.Items {
		if !slices.Contains(core.TechAreas, it.Area) {
			continue
		}
		d := core.TechDecision{Area: it.Area, Recommended: it.Technology, Language: it.Language, Runtime: it.Runtime, Reason: it.Reasoning, Source: core.SourceAI}
		for _, a := range it.Alternatives {
			d.Alternatives = append(d.Alternatives, a.Technology)
		}
		if o := old.Decision(it.Area); o != nil {
			if o.Override != nil {
				ov := *o.Override
				d.Override = &ov
			}
			d.Review = o.Review
			if (o.Source == core.SourceExisting || o.Source == "" && old.Mode == core.StructureExisting) && strings.EqualFold(o.Recommended, it.Technology) {
				d.Source, d.Evidence, d.Reason = core.SourceExisting, o.Evidence, o.Reason
			}
		}
		out = append(out, d)
	}
	return out
}

// Project is the stack as it would be with the plan's decisions.
func Project(p *core.Project, plan *core.TechPlan) *core.TechStack {
	old := p.Stack
	if old == nil {
		old = &core.TechStack{}
	}
	return &core.TechStack{Mode: old.Mode, Needs: old.Needs, Platform: old.Platform,
		Decisions: Decisions(plan, old), Plan: plan}
}

// binding are the choices the plan may not change: what the user chose, and
// what the code of an existing project already uses.
func binding(old *core.TechStack, plan *core.TechPlan) []core.TechIssue {
	var out []core.TechIssue
	if old == nil {
		return nil
	}
	for _, d := range old.Decisions {
		it := plan.Item(d.Area)
		switch {
		case d.Override != nil:
			if it == nil || !strings.EqualFold(it.Technology, d.Override.Value) {
				out = append(out, core.TechIssue{Level: "error", Area: d.Area, Code: CodeBinding, Path: itemPath(d.Area, "technology"),
					Hint:    fmt.Sprintf("pakai tepat %q untuk %s", d.Override.Value, d.Area),
					Message: fmt.Sprintf("pilihan user untuk %s adalah %q dan wajib dipakai; jelaskan dan nilai pilihan itu di rencana, jangan menggantinya", d.Area, d.Override.Value)})
			}
		case d.Source == core.SourceExisting || d.Source == "" && old.Mode == core.StructureExisting:
			if it == nil || !strings.EqualFold(it.Technology, d.Recommended) {
				out = append(out, core.TechIssue{Level: "error", Area: d.Area, Code: CodeBinding, Path: itemPath(d.Area, "technology"),
					Hint:    fmt.Sprintf("pakai tepat %q untuk %s", d.Recommended, d.Area),
					Message: fmt.Sprintf("%s sudah dipakai di kode project (%q) dan harus tetap; mengganti teknologi yang sudah dipakai adalah migrasi yang diminta user, bukan keputusan rencana", d.Area, d.Recommended)})
			}
		}
	}
	return out
}

// Accept cleans a plan the AI wrote and refuses it, with the concrete
// reasons, when it cannot be the technology plan. A conflict between
// decisions is not a reason to refuse: it lands with the plan and blocks
// approval, so the user sees it.
func Accept(p *core.Project, plan *core.TechPlan) error {
	Normalize(plan)
	MarkCurrent(p, plan)
	cand := Project(p, plan)
	refresh(p, plan, cand)
	blocking := append(binding(p.Stack, plan), Blocking(p, plan, cand)...)
	if len(blocking) > 0 {
		return &RejectError{Plan: plan, Issues: blocking}
	}
	if p.Stack != nil {
		for _, d := range p.Stack.Decisions {
			if d.Override == nil {
				continue
			}
			if it := plan.Item(d.Area); it != nil {
				it.UserChosen = d.Override.Reason
				if it.UserChosen == "" {
					it.UserChosen = "dipilih user"
				}
			}
		}
	}
	return nil
}

// PlanChanges lists what differs between two plans, by decision.
func PlanChanges(old, plan *core.TechPlan) []string {
	var out []string
	if old == nil {
		for _, it := range plan.Items {
			out = append(out, fmt.Sprintf("%s: %s", it.Area, it.Technology))
		}
		return append([]string{fmt.Sprintf("Rancangan awal oleh AI: %s (%d keputusan)", plan.Approach, len(plan.Items))}, out...)
	}
	for _, it := range plan.Items {
		o := old.Item(it.Area)
		switch {
		case o == nil:
			out = append(out, fmt.Sprintf("%s: + %s", it.Area, it.Technology))
		case !strings.EqualFold(o.Technology, it.Technology):
			out = append(out, fmt.Sprintf("%s: %s → %s", it.Area, o.Technology, it.Technology))
		case o.Version != it.Version:
			out = append(out, fmt.Sprintf("%s: versi %s → %s", it.Area, orDash(o.Version), orDash(it.Version)))
		}
	}
	for _, o := range old.Items {
		if plan.Item(o.Area) == nil {
			out = append(out, fmt.Sprintf("%s: − %s", o.Area, o.Technology))
		}
	}
	return out
}

// Record appends a revision to the plan's history and returns it.
func Record(plan *core.TechPlan, source, request, reason string, changes []string, status core.TechStatus) core.TechPlanVersion {
	v := core.TechPlanVersion{
		Version: plan.Version() + 1, At: time.Now().UTC(), Source: source, Request: request, Reason: reason,
		Changes: changes, Status: status, Items: len(plan.Items), Score: plan.Evaluation.Score,
	}
	plan.History = append(plan.History, v)
	return v
}

// Land puts an accepted plan into the project's stack.
//
// A plan that replaces a recommendation nobody chose or approved becomes the
// decision at once. Anything else (a revision, an approved or changed stack,
// a stack with the user's own choices) becomes a proposal with its impact:
// the user applies it, and nothing the user approved changes silently. A plan
// that decides the same technologies only replaces the reasoning. v is the
// stack version when the decisions changed at once.
func Land(p *core.Project, plan *core.TechPlan, source, request, reason, note string) (v *core.TechVersion, err error) {
	s := p.Stack
	if s == nil {
		return nil, errors.New("teknologi belum direkomendasikan")
	}
	if s.Proposal != nil {
		return nil, stack.ErrPending
	}
	old := s.Plan
	if old != nil {
		plan.History = slices.Clone(old.History)
	}
	changes := PlanChanges(old, plan)
	next := Decisions(plan, s)
	diff := stack.Diff(s.Decisions, next)
	custom := slices.ContainsFunc(s.Decisions, func(d core.TechDecision) bool { return d.Override != nil })
	status := s.State()
	switch {
	case len(diff) == 0:
		Record(plan, source, request, reason, changes, status)
		s.Decisions, s.Plan = next, plan
		stack.Refresh(s)
		return nil, nil
	case old == nil && source == "generate" && status == core.TechRecommended && !custom:
		Record(plan, source, request, reason, changes, status)
		return stack.ApplyDecisions(s, next, plan, core.SourceAI, request), nil
	default:
		Record(plan, source, request, reason, changes, core.TechReview)
		return nil, stack.ProposeDecisions(s, next, plan, core.SourceAI, request, note)
	}
}

// Gate is what approving the technology needs from a plan that exists: it is
// current and free of conflicts. No plan, no requirement.
func Gate(p *core.Project) error {
	s := p.Stack
	if s == nil || s.Plan == nil {
		return nil
	}
	Refresh(p, s.Plan)
	if s.Plan.Stale {
		return fmt.Errorf("%w (%s)", ErrStale, strings.Join(s.Plan.Analysis.StaleAreas, ", "))
	}
	if errs := s.Plan.Errors(); len(errs) > 0 {
		return fmt.Errorf("%d masalah rencana teknologi harus diselesaikan dulu: %s", len(errs), errs[0].Message)
	}
	return nil
}
