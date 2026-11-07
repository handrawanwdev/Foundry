package pipeline

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/structure"
	"github.com/handrawanwdev/foundry/internal/modules/work"
	"github.com/handrawanwdev/foundry/internal/safefile"
)

func applyUnified(p *core.Project, raw string) error {
	ans, err := work.ParseUnified(raw)
	if err != nil {
		return err
	}
	applied := 0
	// Land in order. A truncated answer often keeps the first sections;
	// missing later sections stay requested and the next round fills them.
	if p.Stack != nil && p.Stack.PlanRequest != nil {
		if !ans.Has(work.KindTech) {
			if applied == 0 {
				return errors.New("jawaban unified tidak berisi bagian technology")
			}
			ans.LandAdaptation(p)
			return nil
		}
		v, err := ans.ApplyUnifiedTech(p)
		if err != nil {
			return err
		}
		landTech(p, v)
		applied++
		if p.Structure != nil && p.Structure.PlanRequest != nil {
			p.Structure.PlanRequest.SourceHash = structure.SourceHash(p)
		}
		if p.Structure != nil && p.Structure.RepoRequest != nil {
			p.Structure.RepoRequest.SourceHash = structure.SourceHash(p)
		}
	}
	if p.Structure != nil && p.Structure.PlanRequest != nil {
		if !ans.Has(work.KindStructure) {
			if applied == 0 {
				return errors.New("jawaban unified tidak berisi bagian architecture")
			}
			ans.LandAdaptation(p)
			return nil
		}
		if err := ans.ApplyUnifiedStructure(p); err != nil {
			return err
		}
		applied++
	}
	if p.Structure != nil && p.Structure.Plan != nil && p.Structure.RepoRequest != nil {
		if err := adoptRecommendedStructure(p); err != nil {
			return fmt.Errorf("menerapkan arsitektur: %w", err)
		}
		p.Structure.RepoRequest.SourceHash = structure.SourceHash(p)
	}
	if p.Structure != nil && p.Structure.RepoRequest != nil {
		if !ans.Has(work.KindRepo) {
			if applied == 0 {
				return errors.New("jawaban unified tidak berisi bagian repository")
			}
			ans.LandAdaptation(p)
			return nil
		}
		if err := ans.ApplyUnifiedRepo(p); err != nil {
			return err
		}
		applied++
	}
	ans.LandAdaptation(p)
	if planRequestsOpen(p) {
		return nil
	}
	work.SyncUnifiedSources(p)
	return nil
}

func finishJobSteps(j *core.Job, at time.Time) []core.JobStep {
	if j == nil || len(j.Steps) == 0 {
		return nil
	}
	out := append([]core.JobStep(nil), j.Steps...)
	for i := range out {
		switch out[i].Status {
		case core.StepPending, core.StepRunning:
			out[i].Status = core.StepDone
			if out[i].EndedAt == nil {
				t := at
				out[i].EndedAt = &t
			}
			if out[i].StartedAt == nil {
				t := at
				out[i].StartedAt = &t
			}
		}
	}
	return out
}

func (s *Service) planPending() bool {
	p, err := s.Store.Load()
	if err != nil {
		return false
	}
	return planRequestsOpen(p)
}

func planRequestsOpen(p *core.Project) bool {
	if p.Stack != nil && p.Stack.PlanRequest != nil {
		return true
	}
	if p.Structure == nil {
		return false
	}
	return p.Structure.PlanRequest != nil || p.Structure.RepoRequest != nil
}

func clearPlanRequests(p *core.Project) {
	if p.Stack != nil {
		p.Stack.PlanRequest = nil
	}
	if p.Structure != nil {
		p.Structure.PlanRequest = nil
		p.Structure.RepoRequest = nil
	}
}

// stampPlanRejection records the refusal on the section being designed; later
// sections have not been answered yet and keep their own retry.
func stampPlanRejection(p *core.Project, msg string) bool {
	switch work.CurrentPlanPart(p) {
	case work.KindTech:
		r := p.Stack.PlanRequest
		retry := r.Rejected == ""
		r.Rejected = msg
		return retry
	case work.KindStructure:
		r := p.Structure.PlanRequest
		retry := r.Rejected == ""
		r.Rejected = msg
		return retry
	case work.KindRepo:
		r := p.Structure.RepoRequest
		retry := r.Rejected == ""
		r.Rejected = msg
		return retry
	}
	return false
}

func (s *Service) dropPlanRequest(cause error) error {
	_, err := s.Store.Update(func(p *core.Project) error {
		if !planRequestsOpen(p) {
			return nil
		}
		msg := fmt.Sprintf("Perancangan Blueprint gagal: %v. Keputusan sebelumnya tidak diubah; tekan Coba lagi.", cause)
		p.Job = &core.Job{Kind: work.KindPlan, Status: core.JobFailed, Message: "Merancang Blueprint", Error: msg, Started: time.Now().UTC()}
		p.Log("system", msg)
		trackFail(p, work.KindPlan, cause)
		return nil
	})
	return err
}

func aiRawPath(dir, kind string) string {
	return filepath.Join(dir, core.StateDir, "ai-raw", kind+".txt")
}

func saveAIRaw(dir, kind, raw string) {
	if dir == "" || kind == "" {
		return
	}
	path := aiRawPath(dir, kind)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	_ = safefile.WriteFile(path, []byte(raw), 0o600)
}

func clearAIRaw(dir, kind string) {
	if dir == "" || kind == "" {
		return
	}
	_ = os.Remove(aiRawPath(dir, kind))
}

func planRepairing(p *core.Project) bool {
	if p == nil || !planRequestsOpen(p) {
		return false
	}
	switch work.CurrentPlanPart(p) {
	case work.KindTech:
		return p.Stack.PlanRequest.Rejected != ""
	case work.KindStructure:
		return p.Structure.PlanRequest.Rejected != ""
	case work.KindRepo:
		return p.Structure.RepoRequest.Rejected != ""
	}
	return false
}

// workflowRoundLabel says how far the filling of an outlined workflow got
// ("Mengisi detail command (5/18 selesai)"); each round is its own job.
func workflowRoundLabel(p *core.Project, msg string) string {
	w := p.Workflow
	if w.Unfilled() == 0 {
		return msg
	}
	return fmt.Sprintf("Mengisi detail command (%d/%d selesai)", len(w.Commands)-w.Unfilled(), len(w.Commands))
}

// planRoundLabel names the section the current plan round designs, so the
// progress line says what is being written ("Merancang Blueprint: Teknologi").
func planRoundLabel(p *core.Project, kind, msg string) string {
	if kind != work.KindPlan {
		return msg
	}
	name := map[string]string{
		work.KindTech:      "Teknologi",
		work.KindStructure: "Arsitektur",
		work.KindRepo:      "Struktur repository",
	}[work.CurrentPlanPart(p)]
	if name == "" {
		return msg
	}
	return msg + ": " + name
}
