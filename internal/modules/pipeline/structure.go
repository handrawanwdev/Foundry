package pipeline

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/stack"
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
	return nil
}

// ProposeStructure records the user's edits as a proposal with its
// validation and impact.
func (s *Service) ProposeStructure(ops []core.StructureOp) (*core.Project, error) {
	return s.Store.Update(func(p *core.Project) error {
		if err := structureEditable(p); err != nil {
			return err
		}
		if p.Structure.Request != "" {
			return errors.New("AI masih memproses permintaan perubahan struktur")
		}
		if err := structure.Propose(p.Structure, p, ops, "user", "", ""); err != nil {
			return err
		}
		p.Log("ui", "Usulan perubahan struktur: "+strings.Join(p.Structure.Proposal.Changes, "; "))
		return nil
	})
}

// RecommendStructure proposes Foundry's recommendation for the current
// needs; the user applies it like any other change.
func (s *Service) RecommendStructure() (*core.Project, error) {
	return s.Store.Update(func(p *core.Project) error {
		if err := structureEditable(p); err != nil {
			return err
		}
		ir, why := structure.Recommend(p)
		return structure.ProposeIR(p.Structure, p, ir, nil, "recommend", "", why)
	})
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
		p.Structure.Request = text
		p.Log("ui", "Minta perubahan struktur: "+text)
		return nil
	})
	if err != nil {
		return err
	}
	return s.startAI(work.KindStructure)
}

func (s *Service) dropStructureRequest(cause error) error {
	_, err := s.Store.Update(func(p *core.Project) error {
		req := ""
		if p.Structure != nil {
			req, p.Structure.Request = p.Structure.Request, ""
		}
		if r := []rune(req); len(r) > 160 {
			req = string(r[:160]) + "…"
		}
		msg := fmt.Sprintf("Permintaan “%s” tidak mengubah struktur: %v. Struktur tidak diubah. "+
			"Tulis lebih spesifik, mis. “Reporting jangan jadi service sendiri, jadikan modul di operations” "+
			"atau “Tiap modul pakai layer domain, application, infrastructure, presentation”.", req, cause)
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
		st := p.Structure
		if st.Proposal == nil {
			return structure.ErrNoProposal
		}
		if len(st.Proposal.Tech) > 0 && p.Stack.Proposal != nil {
			return stack.ErrPending
		}
		before, pr, err := structure.Apply(st)
		if err != nil {
			return err
		}
		if len(pr.Tech) > 0 {
			old := core.CloneDecisions(p.Stack.Decisions)
			var areas []string
			for _, c := range pr.Tech {
				areas = append(areas, c.Area)
				value := c.To
				if d := p.Stack.Decision(c.Area); d != nil && value == d.Recommended {
					value = ""
				}
				if err := stack.Override(p.Stack, c.Area, value, c.Reason); err != nil {
					return err
				}
			}
			stack.Follow(p.Stack, areas...)
			stackChanged(p, old, "structure", "Struktur project: "+pr.IR.Type)
			work.ResetBlueprintTech(p, stack.Diff(old, p.Stack.Decisions))
			work.CompleteBlueprint(p)
		}
		structure.Refresh(st, p)
		v := structure.Record(st, before, pr.Source, pr.Request)
		p.Log("ui", fmt.Sprintf("Struktur v%d diterapkan: %s", v.Version, strings.Join(v.Changes, "; ")))
		return nil
	})
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

// syncStructure keeps the structure in step with the technology without
// overriding the user's structure (structure.Sync).
func syncStructure(p *core.Project, why string) {
	if p.Finalized() {
		return
	}
	structure.Sync(p, why)
}
