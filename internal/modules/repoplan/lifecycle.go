package repoplan

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/handrawanwdev/foundry/internal/core"
)

var (
	ErrNotApplicable = errors.New("project yang sudah ada mengikuti struktur kodenya; rencana folder hanya untuk project baru")
	ErrNoPlan        = errors.New("rencana struktur repository belum ada")
	ErrNeedStructure = errors.New("setujui struktur project dulu; rencana repository merinci struktur yang disetujui")
	ErrStale         = errors.New("sumber rencana berubah; revisi bagian yang terdampak dulu")
)

// Applicable reports whether the project gets a repository plan: a new
// project that has a structure decision.
func Applicable(p *core.Project) error {
	if p.Structure == nil || p.Structure.Mode == core.StructureExisting {
		return ErrNotApplicable
	}
	return nil
}

// Accept cleans a plan the AI wrote and refuses it with the concrete
// reasons when it cannot be the repository plan. A conflict the AI reported
// itself is kept: it blocks approval, not the answer.
func Accept(p *core.Project, plan *core.RepoPlan) error {
	Normalize(plan)
	MarkCurrent(p, plan)
	Refresh(p, plan)
	blocking := Blocking(plan)
	if len(blocking) == 0 {
		return nil
	}
	var msgs []string
	for i, is := range blocking {
		if i == 6 {
			msgs = append(msgs, fmt.Sprintf("dan %d masalah lain", len(blocking)-6))
			break
		}
		msgs = append(msgs, is.Message)
	}
	return fmt.Errorf("rencana struktur repository ditolak, %d masalah: %s", len(blocking), strings.Join(msgs, "; "))
}

// Record appends a revision to the history and returns it.
func Record(plan *core.RepoPlan, source, request, reason string, changes []string) core.RepoVersion {
	v := core.RepoVersion{
		Version: plan.Version() + 1, At: time.Now().UTC(), Source: source, Request: request, Reason: reason,
		Changes: changes, Status: plan.Status, Nodes: len(plan.Nodes), Score: plan.Evaluation.Score,
	}
	plan.History = append(plan.History, v)
	return v
}

// Changed puts an approved plan back under review after an edit.
func Changed(plan *core.RepoPlan) {
	if plan.State() == core.TechApproved {
		plan.Status = core.TechChanged
	} else if plan.Status == "" {
		plan.Status = core.TechRecommended
	}
}

// Approve accepts the plan: the structure decision it details is approved,
// nothing blocks it, and its sources have not changed.
func Approve(p *core.Project, plan *core.RepoPlan) error {
	if plan == nil {
		return ErrNoPlan
	}
	if p.Structure == nil || p.Structure.State() != core.TechApproved {
		return ErrNeedStructure
	}
	Refresh(p, plan)
	if errs := plan.Errors(); len(errs) > 0 {
		return fmt.Errorf("%d masalah harus diselesaikan dulu: %s", len(errs), errs[0].Message)
	}
	if plan.Stale {
		return fmt.Errorf("%w (%s)", ErrStale, strings.Join(plan.Analysis.StaleAreas, ", "))
	}
	plan.Status = core.TechApproved
	if n := len(plan.History); n > 0 {
		plan.History[n-1].Status = core.TechApproved
	}
	return nil
}

// Ready is what finalizing needs from a repository plan that exists: it is
// approved, current and free of errors. No plan, no requirement.
func Ready(p *core.Project) error {
	if p.Structure == nil || p.Structure.Repo == nil || p.Structure.Mode == core.StructureExisting {
		return nil
	}
	plan := p.Structure.Repo
	Refresh(p, plan)
	switch {
	case plan.State() != core.TechApproved:
		return errors.New("rencana struktur repository belum disetujui")
	case plan.Stale:
		return fmt.Errorf("rencana struktur repository kedaluwarsa (%s)", strings.Join(plan.Analysis.StaleAreas, ", "))
	}
	if errs := plan.Errors(); len(errs) > 0 {
		return fmt.Errorf("rencana struktur repository punya %d masalah: %s", len(errs), errs[0].Message)
	}
	return nil
}
