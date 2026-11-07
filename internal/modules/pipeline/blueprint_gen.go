package pipeline

import (
	"errors"
	"fmt"
	"time"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/repoplan"
	"github.com/handrawanwdev/foundry/internal/modules/scan"
	"github.com/handrawanwdev/foundry/internal/modules/spec"
	"github.com/handrawanwdev/foundry/internal/modules/stack"
	"github.com/handrawanwdev/foundry/internal/modules/structure"
	"github.com/handrawanwdev/foundry/internal/modules/techplan"
	"github.com/handrawanwdev/foundry/internal/modules/work"
)

// continueInitialBlueprint runs the rest of the post-feature package:
// remaining artifacts, UI design, then Unified Blueprint Planning.
func (s *Service) continueInitialBlueprint() error {
	p, err := s.Store.Load()
	if err != nil {
		return err
	}
	steps := missingBlueprintSteps(p)
	if len(steps) == 0 {
		return s.clearBlueprintBatch()
	}
	return s.startBlueprintQueue(steps, 1+len(steps))
}

// GenerateBlueprint starts the missing blueprint designs: remaining
// areas, UI design, then Unified Blueprint Planning (technology,
// architecture and folder structure in one AI job). Steps that already
// exist and are current are skipped. Stale sections are redesigned with
// the others frozen.
func (s *Service) GenerateBlueprint() error {
	p, err := s.Store.Load()
	if err != nil {
		return err
	}
	if err := blueprintEditable(p); err != nil {
		return err
	}
	steps := missingBlueprintSteps(p)
	if len(steps) == 0 {
		return ErrNothingToDo
	}
	return s.startBlueprintQueue(steps, len(steps))
}

// missingBlueprintSteps is the generate-all order. Stale plans are
// regenerated so one click leaves a coherent set. Technology, architecture
// and folders are one unified job.
func missingBlueprintSteps(p *core.Project) []string {
	var steps []string
	if work.ScopeRequest(p, spec.Artifacts(p)) != "" {
		steps = append(steps, work.KindArtifacts)
	}
	if p.HasUI() && !p.UI.Written() {
		steps = append(steps, work.KindDesign)
	}
	if len(unifiedParts(p)) > 0 {
		steps = append(steps, work.KindPlan)
	}
	return steps
}

func unifiedParts(p *core.Project) []string {
	if !work.AppLike(p) {
		return nil
	}
	tech := needTechPlan(p)
	st := p.Structure
	existing := st != nil && st.Mode == core.StructureExisting
	if existing {
		if tech {
			return []string{work.KindTech}
		}
		return nil
	}
	structNeed := st == nil || structure.PlanNeeded(st, p)
	repoNeed := needRepoPlan(p)
	if tech {
		return []string{work.KindTech, work.KindStructure, work.KindRepo}
	}
	if structNeed {
		return []string{work.KindStructure, work.KindRepo}
	}
	if repoNeed {
		return []string{work.KindRepo}
	}
	return nil
}

func needTechPlan(p *core.Project) bool {
	if p.Stack == nil || p.Stack.Plan == nil {
		return true
	}
	techplan.Refresh(p, p.Stack.Plan)
	return p.Stack.Plan.Stale
}

func needRepoPlan(p *core.Project) bool {
	st := p.Structure
	if st == nil || st.Mode == core.StructureExisting {
		return false
	}
	if st.Repo == nil {
		return true
	}
	repoplan.Refresh(p, st.Repo)
	return st.Repo.Stale
}

func (s *Service) startBlueprintQueue(steps []string, total int) error {
	for len(steps) > 0 {
		if steps[0] == work.KindRepo {
			if err := s.ensureStructureAdopted(); err != nil {
				return s.failBlueprintAdopt(err)
			}
		}
		kind := steps[0]
		err := s.startAIIf(kind, func(p *core.Project) error {
			if err := blueprintEditable(p); err != nil {
				return err
			}
			if err := prepareBlueprintStep(p, kind); err != nil {
				return err
			}
			p.BlueprintQueue = steps
			if total > 0 && !continuesDiscovery(p) {
				now := time.Now().UTC()
				p.BlueprintStarted = &now
			}
			if total > 0 {
				p.BlueprintBatch = total
			} else if p.BlueprintBatch == 0 {
				p.BlueprintBatch = len(steps)
			}
			p.Log("ui", "Susun rancangan blueprint: "+kind)
			return nil
		})
		if err == nil {
			return nil
		}
		if !errors.Is(err, ErrNothingToDo) {
			return err
		}
		steps = steps[1:]
	}
	return s.clearBlueprintBatch()
}

func prepareBlueprintStep(p *core.Project, kind string) error {
	switch kind {
	case work.KindArtifacts:
		text := work.ScopeRequest(p, spec.Artifacts(p))
		if text == "" {
			return ErrNothingToDo
		}
		p.ArtifactRequest = &core.ChangeRequest{Text: text}
		return nil
	case work.KindDesign:
		if err := designEditable(p); err != nil {
			return err
		}
		if p.UI.Written() {
			return ErrNothingToDo
		}
		p.UI = nil
		return nil
	case work.KindTech:
		if err := stackEditable(p); err != nil {
			return err
		}
		if p.Stack.Proposal != nil {
			return stack.ErrPending
		}
		action := core.TechPlanGenerate
		if p.Stack.Plan != nil {
			action = core.TechPlanRevise
		}
		p.Stack.PlanRequest = &core.TechPlanRequest{Action: action}
		return nil
	case work.KindStructure:
		if err := structureEditable(p); err != nil {
			return err
		}
		if p.Structure.Proposal != nil {
			return structure.ErrPending
		}
		p.Structure.Request = ""
		p.Structure.PlanRequest = &core.StructurePlanRequest{SourceHash: structure.SourceHash(p)}
		return nil
	case work.KindRepo:
		if err := structureEditable(p); err != nil {
			return err
		}
		if p.Structure.Proposal != nil {
			return structure.ErrPending
		}
		action := core.RepoGenerate
		if p.Structure.Repo != nil {
			action = core.RepoRevise
		}
		p.Structure.RepoRequest = &core.RepoRequest{Action: action, SourceHash: structure.SourceHash(p)}
		return nil
	case work.KindPlan:
		return prepareUnifiedPlan(p)
	}
	return fmt.Errorf("langkah rancangan %q tidak dikenal", kind)
}

// prepareUnifiedPlan sets the section requests of one unified job and
// refreshes the repository inventory once when it is stale.
func prepareUnifiedPlan(p *core.Project) error {
	parts := unifiedParts(p)
	if len(parts) == 0 {
		return ErrNothingToDo
	}
	refreshPlanningScan(p)
	for _, part := range parts {
		switch part {
		case work.KindTech:
			if err := stackEditable(p); err != nil {
				return err
			}
			if p.Stack.Proposal != nil {
				return stack.ErrPending
			}
			p.Stack.PlanRequest = &core.TechPlanRequest{Action: core.TechPlanGenerate}
		case work.KindStructure:
			if err := structureEditable(p); err != nil {
				return err
			}
			if p.Structure.Proposal != nil {
				return structure.ErrPending
			}
			p.Structure.Request = ""
			p.Structure.PlanRequest = &core.StructurePlanRequest{SourceHash: structure.SourceHash(p)}
		case work.KindRepo:
			if err := structureEditable(p); err != nil {
				return err
			}
			if p.Structure.Proposal != nil {
				return structure.ErrPending
			}
			p.Structure.RepoRequest = &core.RepoRequest{Action: core.RepoGenerate, SourceHash: structure.SourceHash(p)}
		}
	}
	return nil
}

func refreshPlanningScan(p *core.Project) {
	st := p.Structure
	if st == nil || st.Mode != core.StructureExisting {
		return
	}
	c := st.Context
	if c != nil && !c.Stale && !c.Empty() {
		return
	}
	st.Context = scan.Refresh(p.Dir, c, st.Components)
}

// advanceBlueprint starts the next missing design after a batch step
// landed. A job that was not part of the batch (prompt revise, discovery
// chain) leaves the queue alone.
func (s *Service) advanceBlueprint(kind string) error {
	p, err := s.Store.Load()
	if err != nil || p.BlueprintBatch == 0 {
		return err
	}
	q := p.BlueprintQueue
	if len(q) == 0 || q[0] != kind {
		return nil
	}
	rest := q[1:]
	if len(rest) == 0 {
		return s.clearBlueprintBatch()
	}
	return s.startBlueprintQueue(rest, 0)
}

func (s *Service) clearBlueprintBatch() error {
	_, err := s.Store.Update(func(p *core.Project) error {
		p.BlueprintQueue = nil
		p.BlueprintBatch = 0
		return nil
	})
	return err
}

// ensureStructureAdopted takes the AI recommendation as the structure
// before the folder plan runs. An already-approved structure is left
// alone. A proposal with errors stops the batch so the user can fix it.
func (s *Service) ensureStructureAdopted() error {
	p, err := s.Store.Load()
	if err != nil {
		return err
	}
	st := p.Structure
	if st == nil || st.Mode == core.StructureExisting || st.State() == core.TechApproved {
		return nil
	}
	if st.Plan == nil || !structure.PlanCurrent(st, p) {
		return nil
	}
	p, err = s.UseStructureAlternative("")
	if err != nil {
		return err
	}
	if p.Structure == nil || p.Structure.State() != core.TechApproved {
		return errors.New("rekomendasi punya masalah validasi; perbaiki usulan struktur dulu")
	}
	return nil
}

func (s *Service) failBlueprintAdopt(cause error) error {
	_, err := s.Store.Update(func(p *core.Project) error {
		p.BlueprintQueue = nil
		p.BlueprintBatch = 0
		msg := fmt.Sprintf("Rekomendasi struktur belum bisa diterapkan: %v. Perbaiki di Struktur repository, lalu tekan Susun rancangan lagi.", cause)
		p.Job = &core.Job{Kind: work.KindStructure, Status: core.JobFailed, Message: "Menerapkan rekomendasi struktur", Error: msg, Started: time.Now().UTC()}
		p.Log("system", msg)
		return nil
	})
	if err != nil {
		return err
	}
	return cause
}

func batchLabel(p *core.Project, msg string) string {
	if p.BlueprintBatch > 0 {
		return "Merancang Blueprint"
	}
	return msg
}

// continuesDiscovery: the batch is the rest of the first blueprint run that
// discovery began, so it keeps that run's start time.
func continuesDiscovery(p *core.Project) bool {
	return p.BlueprintStarted != nil && p.Job != nil && p.Job.Kind == work.KindDiscovery && p.Job.Status == core.JobDone
}
