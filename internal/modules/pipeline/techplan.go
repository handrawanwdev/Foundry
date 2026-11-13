package pipeline

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/stack"
	"github.com/handrawanwdev/foundry/internal/modules/techplan"
	"github.com/handrawanwdev/foundry/internal/modules/work"
)

// Technology plan workspace: the AI analyzes what the system needs and
// designs its technology (decisions with reasoning, alternatives,
// compatibility, evaluation); the user asks for revisions in words or
// changes a decision by hand; a plan that replaces something the user
// approved or chose is a proposal with its impact, never a silent change.
// Approval is the technology approval, extended with the plan's own checks.

var errTechBusy = errors.New("AI masih memproses rencana teknologi")

// techEditable is stackEditable plus the checks every plan change shares: no
// proposal waits and no AI request is open.
func techEditable(p *core.Project) error {
	if err := stackEditable(p); err != nil {
		return err
	}
	st := p.Stack
	if st.Proposal != nil {
		return stack.ErrPending
	}
	if st.Request != "" || st.PlanRequest != nil {
		return errTechBusy
	}
	if p.Job != nil && p.Job.Status == core.JobWaiting {
		return ErrBusy
	}
	return nil
}

// GenerateTechPlan asks the AI to design the technology. With a plan already
// there, text asks for a new design that builds on it; without text the
// design is only repeated once its sources changed.
func (s *Service) GenerateTechPlan(text, reason string) error {
	text, err := work.CleanTechRequest(text)
	if err != nil {
		return err
	}
	_, err = s.Store.Update(func(p *core.Project) error {
		if err := techEditable(p); err != nil {
			return err
		}
		if plan := p.Stack.Plan; plan != nil {
			techplan.Refresh(p, plan)
			if text == "" && !plan.Stale {
				return errors.New("rencana teknologi masih berlaku: sumbernya belum berubah; tulis permintaan untuk merancang ulang")
			}
		}
		p.Stack.PlanRetry = nil
		p.Stack.PlanRequest = &core.TechPlanRequest{Action: core.TechPlanGenerate, Text: text, Reason: strings.TrimSpace(reason)}
		if text == "" {
			p.Log("ui", "Minta AI merancang teknologi")
		} else {
			p.Log("ui", "Minta AI merancang ulang teknologi: "+text)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return s.startAI(work.KindTech)
}

// ReviseTechPlan asks the AI to revise the current plan. Without text it
// reconciles the plan with the sources that changed and evaluates it again.
func (s *Service) ReviseTechPlan(text, reason string) error {
	text, err := work.CleanTechRequest(text)
	if err != nil {
		return err
	}
	_, err = s.Store.Update(func(p *core.Project) error {
		if err := techEditable(p); err != nil {
			return err
		}
		plan := p.Stack.Plan
		if plan == nil {
			return techplan.ErrNoPlan
		}
		techplan.Refresh(p, plan)
		if text == "" && !plan.Stale && !plan.Evaluation.Outdated && len(plan.Errors()) == 0 {
			return errors.New("tulis permintaan revisi; sumber tidak berubah dan evaluasi masih berlaku")
		}
		p.Stack.PlanRetry = nil
		p.Stack.PlanRequest = &core.TechPlanRequest{Action: core.TechPlanRevise, Text: text, Reason: strings.TrimSpace(reason)}
		if text == "" {
			p.Log("ui", "Minta AI menyesuaikan rencana teknologi dengan perubahan sumber")
		} else {
			p.Log("ui", "Minta revisi rencana teknologi: "+text)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return s.startAI(work.KindTech)
}

// DiscardTechPlan deletes the plan. The decisions stay as they are; Foundry's
// recommender governs them again.
func (s *Service) DiscardTechPlan() (*core.Project, error) {
	return s.Store.Update(func(p *core.Project) error {
		if err := techEditable(p); err != nil {
			return err
		}
		if p.Stack.Plan == nil {
			return techplan.ErrNoPlan
		}
		p.Stack.Plan = nil
		stack.Refresh(p.Stack)
		stampRefs(p)
		p.Log("ui", "Rencana teknologi dihapus; keputusan teknologi tetap")
		return nil
	})
}

// techPending reports whether an AI technology request still waits for a
// usable answer.
func (s *Service) techPending() bool {
	p, err := s.Store.Load()
	return err == nil && p.Stack != nil && p.Stack.PlanRequest != nil
}

// dropTechRequest closes a request whose answer was refused twice. The plan
// and decisions before it stay as they were.
func (s *Service) dropTechRequest(cause error) error {
	_, err := s.Store.Update(func(p *core.Project) error {
		if p.Stack == nil || p.Stack.PlanRequest == nil {
			return nil
		}
		req := p.Stack.PlanRequest
		report := techDiagnosis(p, work.KindTech, cause, true)
		p.Stack.PlanRequest = nil
		p.Stack.PlanRetry = req
		what := "Perancangan teknologi"
		if req.Action == core.TechPlanRevise {
			what = "Revisi rencana teknologi"
		}
		msg := fmt.Sprintf("%s gagal: %v. Rencana dan keputusan teknologi tidak diubah; permintaanmu dan draft yang valid disimpan, Coba lagi melanjutkannya.", what, cause)
		p.Job = &core.Job{Kind: work.KindTech, Status: core.JobFailed, Message: what, Error: msg, Started: time.Now().UTC()}
		p.Log("system", msg)
		trackFailMsg(p, work.KindTech, cause, report)
		return nil
	})
	return err
}

// landTech follows an accepted answer into the rest of the project: decisions
// that changed at once reach the blueprint and the structure; a proposal gets
// its impact.
func landTech(p *core.Project, v *core.TechVersion) {
	switch {
	case v != nil:
		followStack(p, v)
		p.Log("ui", fmt.Sprintf("Teknologi v%d dirancang AI: %s", v.Version, changeText(v.Changes)))
	case p.Stack != nil && p.Stack.Proposal != nil:
		p.Stack.Proposal.Affected = affected(p)
	}
}
