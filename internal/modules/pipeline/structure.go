package pipeline

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/structure"
	"github.com/handrawanwdev/foundry/internal/modules/work"
)

// Project structure workspace: recommendation → user edit (by hand or in
// words) → validation and impact → apply → approval. The approved Structure
// IR is what the scaffold and the build follow.

var ErrStructureNotApproved = errors.New("setujui struktur project dulu")

// structureEditable: the structure may change until the blueprint is
// finalized.
func structureEditable(p *core.Project) error {
	if p.Busy() {
		return ErrBusy
	}
	if p.Platform == nil || p.Structure == nil || p.Finalized() {
		return ErrWrongStage
	}
	if p.Structure.Mode == core.StructureExisting {
		return structure.ErrExisting
	}
	if p.Stack == nil {
		decideStack(p, "")
	}
	if p.Structure.IR == nil {
		structure.Sync(p, "")
	}
	structure.Refresh(p.Structure, p)
	return nil
}

// ProposeStructure records the user's edits as a proposal with its
// validation and impact.
func (s *Service) ProposeStructure(ops []core.StructureOp) (*core.Project, error) {
	return s.Store.Update(func(p *core.Project) error {
		if err := structureEditable(p); err != nil {
			return err
		}
		if p.Structure.Request != "" || p.Structure.PlanRequest != nil {
			return errors.New("AI masih memproses permintaan struktur")
		}
		if err := structure.Propose(p.Structure, p, ops, "user", "", ""); err != nil {
			return err
		}
		p.Log("ui", "Usulan perubahan struktur: "+strings.Join(p.Structure.Proposal.Changes, "; "))
		return nil
	})
}

// RecommendStructure adopts the alternative the AI analysis recommends.
// Foundry has no structure of its own to recommend.
func (s *Service) RecommendStructure() (*core.Project, error) {
	return s.UseStructureAlternative("")
}

// RequestStructureChange asks the AI to turn a request in words into a
// proposal.
func (s *Service) RequestStructureChange(text string) error {
	text, err := work.CleanStructureRequest(text)
	if err != nil {
		return err
	}
	_, err = s.Store.Update(func(p *core.Project) error {
		if err := structureEditable(p); err != nil {
			return err
		}
		if p.Job != nil && p.Job.Status == core.JobWaiting {
			return ErrBusy
		}
		if p.Structure.PlanRequest != nil {
			return errors.New("AI masih menganalisis struktur")
		}
		p.Structure.Request = text
		p.Log("ui", "Minta perubahan struktur: "+text)
		return nil
	})
	if err != nil {
		return err
	}
	return s.startAI(work.KindStructure)
}

// AnalyzeStructure asks the AI to analyze the whole system and propose
// alternatives. An empty text is a fresh analysis; a text revises the
// current plan. The analysis is not repeated while its source is unchanged.
func (s *Service) AnalyzeStructure(text string) error {
	text = strings.TrimSpace(text)
	_, err := s.Store.Update(func(p *core.Project) error {
		if err := structureEditable(p); err != nil {
			return err
		}
		if p.Structure.Proposal != nil {
			return structure.ErrPending
		}
		if p.Structure.Request != "" || p.Structure.PlanRequest != nil {
			return errors.New("AI masih memproses permintaan struktur")
		}
		if p.Job != nil && p.Job.Status == core.JobWaiting {
			return ErrBusy
		}
		if text == "" && structure.PlanCurrent(p.Structure, p) {
			return structure.ErrPlanCurrent
		}
		p.Structure.PlanRequest = &core.StructurePlanRequest{Text: text, SourceHash: structure.SourceHash(p)}
		if text == "" {
			p.Log("ui", "Minta analisis struktur dengan AI")
		} else {
			p.Log("ui", "Minta revisi analisis struktur: "+text)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return s.startAI(work.KindStructure)
}

// adoptRecommendedStructure takes the AI recommendation as the structure
// and approves it, in the caller's store update. Used by unified planning
// so the folder plan can land against the adopted IR.
func adoptRecommendedStructure(p *core.Project) error {
	st := p.Structure
	if st == nil || st.Mode == core.StructureExisting {
		return nil
	}
	if st.State() == core.TechApproved && (st.Plan == nil || st.Plan.Selected != "") {
		return nil
	}
	if st.Plan == nil {
		return nil
	}
	st.Plan.SourceHash = structure.SourceHash(p)
	st.Plan.Stale = false
	alt := st.Plan.RecommendedAlternative()
	if alt == nil {
		return errors.New("rekomendasi struktur tidak ada")
	}
	ir := work.PlanAlternativeIR(st.Plan, alt)
	decision := alt.Summary
	if alt.ID == st.Plan.Recommendation {
		decision = st.Plan.Reason
	}
	err := structure.ProposeIR(st, p, ir, nil, "ai", st.Plan.Reason, alt.Summary)
	if errors.Is(err, structure.ErrNoChange) {
		if hasStructureError(structure.Validate(ir, p, p.Stack)) {
			return err
		}
		st.Plan.Selected, st.Plan.Decision, st.Plan.DecidedAt = alt.ID, decision, time.Now().UTC()
		return structure.Approve(st, p)
	}
	if err != nil {
		return err
	}
	if hasStructureError(st.Proposal.Issues) {
		return errors.New("rekomendasi punya masalah validasi; perbaiki usulan struktur dulu")
	}
	st.Plan.Selected, st.Plan.Decision, st.Plan.DecidedAt = alt.ID, decision, time.Now().UTC()
	if err := applyStructureProposalLocked(p); err != nil {
		return err
	}
	return structure.Approve(st, p)
}

// UseStructureAlternative adopts one AI alternative as the structure and
// approves it. When the alternative has validation errors the proposal is
// left waiting, so the user can fix it with the existing editor.
func (s *Service) UseStructureAlternative(alternative string) (*core.Project, error) {
	alternative = strings.TrimSpace(alternative)
	var adopted bool
	p, err := s.Store.Update(func(p *core.Project) error {
		if err := structureEditable(p); err != nil {
			return err
		}
		if p.Structure.Proposal != nil {
			return structure.ErrPending
		}
		if p.Structure.Request != "" || p.Structure.PlanRequest != nil {
			return errors.New("AI masih memproses permintaan struktur")
		}
		plan := p.Structure.Plan
		if plan == nil {
			return structure.ErrNoPlan
		}
		if !structure.PlanCurrent(p.Structure, p) {
			return structure.ErrPlanStale
		}
		alt := plan.Alternative(alternative)
		if alternative == "" {
			alt = plan.RecommendedAlternative()
		}
		if alt == nil {
			return fmt.Errorf("alternatif struktur %q tidak ada", alternative)
		}
		ir := work.PlanAlternativeIR(plan, alt)
		decision := alt.Summary
		if alt.ID == plan.Recommendation {
			decision = plan.Reason
		}
		err := structure.ProposeIR(p.Structure, p, ir, nil, "ai", plan.Reason, alt.Summary)
		if errors.Is(err, structure.ErrNoChange) {
			// The current structure is already this alternative; only the
			// decision needs to be recorded.
			if hasStructureError(structure.Validate(ir, p, p.Stack)) {
				return err
			}
			plan.Selected, plan.Decision, plan.DecidedAt = alt.ID, decision, time.Now().UTC()
			adopted = true
			return nil
		}
		if err != nil {
			return err
		}
		p.Structure.Proposal.Alternative = alt.ID
		if hasStructureError(p.Structure.Proposal.Issues) {
			// Leave the proposal for the existing review/editor flow.
			return nil
		}
		plan.Selected, plan.Decision, plan.DecidedAt = alt.ID, decision, time.Now().UTC()
		adopted = true
		p.Log("ui", "Struktur AI dipilih: "+alt.Name)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if !adopted {
		return p, nil
	}
	if p.Structure.Proposal == nil {
		return s.ApproveStructure()
	}
	if _, err := s.ApplyStructureProposal(); err != nil {
		return nil, err
	}
	return s.ApproveStructure()
}

func hasStructureError(issues []core.TechIssue) bool {
	for _, i := range issues {
		if i.Level == "error" {
			return true
		}
	}
	return false
}

// structurePlanPending reports whether an AI structure analysis is still
// waiting for a usable answer.
func (s *Service) structurePlanPending() bool {
	p, err := s.Store.Load()
	return err == nil && p.Structure != nil && p.Structure.PlanRequest != nil
}

func (s *Service) dropStructureRequest(cause error) error {
	_, err := s.Store.Update(func(p *core.Project) error {
		if p.Structure != nil && p.Structure.PlanRequest != nil {
			req := p.Structure.PlanRequest.Text
			p.Structure.PlanRequest = nil
			if r := []rune(req); len(r) > 160 {
				req = string(r[:160]) + "…"
			}
			if req == "" {
				req = "analisis seluruh sistem"
			}
			msg := fmt.Sprintf("Analisis struktur “%s” gagal: %v. Struktur tidak diubah; analisis sebelumnya (bila ada) tetap dipakai sampai kamu menganalisis ulang.", req, cause)
			p.Job = &core.Job{Kind: work.KindStructure, Status: core.JobFailed, Message: "Analisis struktur", Error: msg, Started: time.Now().UTC()}
			p.Log("system", msg)
			trackFail(p, work.KindStructure, cause)
			return nil
		}
		req := ""
		if p.Structure != nil {
			req, p.Structure.Request = p.Structure.Request, ""
		}
		if r := []rune(req); len(r) > 160 {
			req = string(r[:160]) + "…"
		}
		msg := fmt.Sprintf("Permintaan “%s” tidak mengubah struktur: %v. Struktur tidak diubah.", req, cause)
		p.Job = &core.Job{Kind: work.KindStructure, Status: core.JobFailed, Message: "Permintaan perubahan struktur", Error: msg, Started: time.Now().UTC()}
		p.Log("system", msg)
		return nil
	})
	return err
}

// ApplyStructureProposal makes the waiting proposal the structure: a new
// version. The architecture/repository changes it needs, shown in the
// proposal, are applied to the technology with it.
func (s *Service) ApplyStructureProposal() (*core.Project, error) {
	return s.Store.Update(func(p *core.Project) error {
		if err := structureEditable(p); err != nil {
			return err
		}
		return applyStructureProposalLocked(p)
	})
}

func applyStructureProposalLocked(p *core.Project) error {
	st := p.Structure
	if st.Proposal == nil {
		return structure.ErrNoProposal
	}
	before, pr, err := structure.Apply(st)
	if err != nil {
		return err
	}
	work.CompleteBlueprint(p)
	stampRefs(p)
	structure.Refresh(st, p)
	v := structure.Record(st, before, pr.Source, pr.Request)
	p.Log("ui", fmt.Sprintf("Struktur v%d diterapkan: %s", v.Version, strings.Join(v.Changes, "; ")))
	return nil
}

func (s *Service) DiscardStructureProposal() (*core.Project, error) {
	return s.Store.Update(func(p *core.Project) error {
		if err := structureEditable(p); err != nil {
			return err
		}
		if err := structure.Discard(p.Structure); err != nil {
			return err
		}
		p.Log("ui", "Usulan perubahan struktur dibatalkan")
		return nil
	})
}

func (s *Service) ApproveStructure() (*core.Project, error) {
	return s.Store.Update(func(p *core.Project) error {
		if err := structureEditable(p); err != nil {
			return err
		}
		if p.Structure.PlanRequest != nil {
			return ErrBusy
		}
		if err := structure.Approve(p.Structure, p); err != nil {
			return err
		}
		p.Log("ui", fmt.Sprintf("Struktur v%d disetujui", p.Structure.Version()))
		return nil
	})
}

// structureReady is what finalizing needs from the structure: an
// approved IR without errors. An existing project keeps its own structure.
func structureReady(p *core.Project) error {
	st := p.Structure
	if st.Mode == core.StructureExisting {
		return nil
	}
	if st.IR == nil {
		structure.Sync(p, "")
	}
	structure.Refresh(st, p)
	if st.State() != core.TechApproved {
		return ErrStructureNotApproved
	}
	if errs := st.Errors(); len(errs) > 0 {
		return fmt.Errorf("%d masalah struktur harus diselesaikan dulu: %s", len(errs), errs[0].Message)
	}
	return nil
}

// syncStructure revalidates the structure against the technology without
// changing it (structure.Sync).
func syncStructure(p *core.Project, why string) {
	if p.Finalized() {
		return
	}
	structure.Sync(p, why)
}
