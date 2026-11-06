package pipeline

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/repoplan"
	"github.com/handrawanwdev/foundry/internal/modules/structure"
	"github.com/handrawanwdev/foundry/internal/modules/work"
)

// Repository plan workspace: the AI designs the repository structure
// (folders, files, module mapping, dependency rules, quality review), the
// user asks for revisions in words or edits by hand, each with a reason, and
// approves. The approved plan is exported for the external coding agent.

var errRepoBusy = errors.New("AI masih memproses rencana repository")

// repoEditable is structureEditable plus the checks every repository
// change shares: no structure proposal waits, and no AI request is open.
func repoEditable(p *core.Project) error {
	if err := structureEditable(p); err != nil {
		return err
	}
	st := p.Structure
	if st.Proposal != nil {
		return structure.ErrPending
	}
	if st.Request != "" || st.PlanRequest != nil || st.RepoRequest != nil {
		return errRepoBusy
	}
	if p.Job != nil && p.Job.Status == core.JobWaiting {
		return ErrBusy
	}
	return nil
}

func checkRepoText(text string) (string, error) { return work.CleanRepoRequest(text) }

// GenerateRepoPlan asks the AI to design the repository structure. With a
// plan already there, text asks for a new design that builds on it; without
// text the design is only repeated once its sources changed.
func (s *Service) GenerateRepoPlan(text, reason string) error {
	text, err := checkRepoText(text)
	if err != nil {
		return err
	}
	_, err = s.Store.Update(func(p *core.Project) error {
		if err := repoEditable(p); err != nil {
			return err
		}
		if plan := p.Structure.Repo; plan != nil {
			repoplan.Refresh(p, plan)
			if text == "" && !plan.Stale {
				return errors.New("rencana repository masih berlaku: sumbernya belum berubah; tulis permintaan untuk merancang ulang")
			}
		}
		p.Structure.RepoRequest = &core.RepoRequest{Action: core.RepoGenerate, Text: text, Reason: strings.TrimSpace(reason), SourceHash: structure.SourceHash(p)}
		if text == "" {
			p.Log("ui", "Minta AI merancang struktur repository")
		} else {
			p.Log("ui", "Minta AI merancang ulang struktur repository: "+text)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return s.startAI(work.KindRepo)
}

// ReviseRepoPlan asks the AI to revise the current plan. Without text it
// reconciles the plan with the sources that changed and evaluates it again.
func (s *Service) ReviseRepoPlan(text, reason string) error {
	text, err := checkRepoText(text)
	if err != nil {
		return err
	}
	_, err = s.Store.Update(func(p *core.Project) error {
		if err := repoEditable(p); err != nil {
			return err
		}
		plan := p.Structure.Repo
		if plan == nil {
			return repoplan.ErrNoPlan
		}
		repoplan.Refresh(p, plan)
		if text == "" && !plan.Stale && !plan.Evaluation.Outdated && len(plan.Errors()) == 0 {
			return errors.New("tulis permintaan revisi; sumber tidak berubah dan evaluasi masih berlaku")
		}
		p.Structure.RepoRequest = &core.RepoRequest{Action: core.RepoRevise, Text: text, Reason: strings.TrimSpace(reason)}
		if text == "" {
			p.Log("ui", "Minta AI menyesuaikan struktur repository dengan perubahan sumber")
		} else {
			p.Log("ui", "Minta revisi struktur repository: "+text)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return s.startAI(work.KindRepo)
}

// EditRepoPlan applies the user's own edits (add, remove, move, redesign)
// as a new version. The reason is required; the AI evaluation is marked
// outdated until the AI evaluates the plan again.
func (s *Service) EditRepoPlan(ops []core.RepoOp, reason string) (*core.Project, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, errors.New("tulis alasan perubahan struktur")
	}
	if len(reason) > work.MaxRepoRequest {
		return nil, fmt.Errorf("alasan terlalu panjang (maks. %d karakter)", work.MaxRepoRequest)
	}
	return s.Store.Update(func(p *core.Project) error {
		if err := repoEditable(p); err != nil {
			return err
		}
		if p.Structure.Repo == nil {
			return repoplan.ErrNoPlan
		}
		plan := p.Structure.Repo.Clone()
		changes, err := repoplan.Apply(plan, ops)
		if err != nil {
			return err
		}
		plan.Evaluation.Outdated = true
		repoplan.Changed(plan)
		repoplan.Refresh(p, plan)
		v := repoplan.Record(plan, "user", "", reason, changes)
		p.Structure.Repo = plan
		stampRefs(p)
		p.Log("ui", fmt.Sprintf("Struktur repository v%d: %s", v.Version, strings.Join(changes, "; ")))
		return nil
	})
}

// ApproveRepoPlan approves the plan once nothing blocks it.
func (s *Service) ApproveRepoPlan() (*core.Project, error) {
	return s.Store.Update(func(p *core.Project) error {
		if err := repoEditable(p); err != nil {
			return err
		}
		if err := repoplan.Approve(p, p.Structure.Repo); err != nil {
			return err
		}
		stampRefs(p)
		p.Log("ui", fmt.Sprintf("Struktur repository v%d disetujui", p.Structure.Repo.Version()))
		return nil
	})
}

// DiscardRepoPlan deletes the plan; the coarse structure decision stays.
func (s *Service) DiscardRepoPlan() (*core.Project, error) {
	return s.Store.Update(func(p *core.Project) error {
		if err := repoEditable(p); err != nil {
			return err
		}
		if p.Structure.Repo == nil {
			return repoplan.ErrNoPlan
		}
		p.Structure.Repo = nil
		stampRefs(p)
		p.Log("ui", "Rencana struktur repository dihapus")
		return nil
	})
}

// repoPending reports whether an AI repository request still waits for a
// usable answer.
func (s *Service) repoPending() bool {
	p, err := s.Store.Load()
	return err == nil && p.Structure != nil && p.Structure.RepoRequest != nil
}

// dropRepoRequest closes a request whose answer was refused twice. The plan
// before it stays as it was.
func (s *Service) dropRepoRequest(cause error) error {
	_, err := s.Store.Update(func(p *core.Project) error {
		if p.Structure == nil || p.Structure.RepoRequest == nil {
			return nil
		}
		req := p.Structure.RepoRequest
		p.Structure.RepoRequest = nil
		what := "Perancangan struktur repository"
		if req.Action == core.RepoRevise {
			what = "Revisi struktur repository"
		}
		msg := fmt.Sprintf("%s gagal: %v. Rencana tidak diubah; tulis permintaan yang lebih spesifik atau coba lagi.", what, cause)
		p.Job = &core.Job{Kind: work.KindRepo, Status: core.JobFailed, Message: what, Error: msg, Started: time.Now().UTC()}
		p.Log("system", msg)
		return nil
	})
	return err
}
