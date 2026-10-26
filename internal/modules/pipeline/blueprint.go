package pipeline

import (
	"errors"
	"fmt"
	"time"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/spec"
	"github.com/handrawanwdev/foundry/internal/modules/stack"
	"github.com/handrawanwdev/foundry/internal/modules/work"
	"github.com/handrawanwdev/foundry/internal/modules/workflow"
)

// ErrFinalized means the blueprint is finalized; reopen it to change it.
var ErrFinalized = errors.New("blueprint sudah difinalisasi; buka kembali untuk mengubah")

// blueprintEditable is what every blueprint and workflow change needs: no job, a
// discovered project, not finalized.
func blueprintEditable(p *core.Project) error {
	switch {
	case p.Busy():
		return ErrBusy
	case p.Finalized():
		return ErrFinalized
	case p.Discovery == nil || p.Stage != core.StageDesign:
		return fmt.Errorf("%w: blueprint belum dibuat dari prompt", ErrWrongStage)
	}
	return nil
}

// ---------- Blueprint artifacts ----------

// ReviseArtifacts asks the AI to change the blueprint artifacts. focus
// names the artifacts the user is looking at.
func (s *Service) ReviseArtifacts(text string, focus []string) error {
	return s.startAIIf(work.KindArtifacts, func(p *core.Project) error {
		if err := blueprintEditable(p); err != nil {
			return err
		}
		if err := work.RequestArtifactChange(p, text, focus, spec.Artifacts(p)); err != nil {
			return err
		}
		p.Log("ui", "Minta perubahan blueprint: "+text)
		return nil
	})
}

// EditArtifact saves a user's edit of a written artifact.
func (s *Service) EditArtifact(id string, in core.Artifact) (*core.Project, error) {
	return s.Store.Update(func(p *core.Project) error {
		if err := blueprintEditable(p); err != nil {
			return err
		}
		if err := work.EditArtifact(p, id, in); err != nil {
			return err
		}
		stampRefs(p)
		p.Log("ui", "Artefak diedit: "+id)
		return nil
	})
}

// RemoveArtifact deletes a written artifact.
func (s *Service) RemoveArtifact(id string) (*core.Project, error) {
	return s.Store.Update(func(p *core.Project) error {
		if err := blueprintEditable(p); err != nil {
			return err
		}
		if err := work.RemoveArtifact(p, id); err != nil {
			return err
		}
		p.Log("ui", "Artefak dihapus: "+id)
		return nil
	})
}

// ---------- Command workflow ----------

// GenerateWorkflow asks the AI to write the workflow from the blueprint.
// It only starts while there are no commands; a workflow is changed with
// ReviseWorkflow so valid commands are not written again.
func (s *Service) GenerateWorkflow() error {
	return s.startAIIf(work.KindWorkflow, func(p *core.Project) error {
		if err := blueprintEditable(p); err != nil {
			return err
		}
		if p.Workflow != nil && len(p.Workflow.Commands) > 0 {
			return fmt.Errorf("%w: workflow sudah ada; minta perubahan lewat prompt", ErrWrongStage)
		}
		p.Log("ui", "Susun command workflow")
		return nil
	})
}

// ReviseWorkflow asks the AI to change the commands for a request; focus
// names the commands it is about.
func (s *Service) ReviseWorkflow(text string, focus []string) error {
	return s.startAIIf(work.KindWorkflow, func(p *core.Project) error {
		if err := blueprintEditable(p); err != nil {
			return err
		}
		if err := work.RequestWorkflowChange(p, text, focus); err != nil {
			return err
		}
		p.Log("ui", "Minta perubahan workflow: "+text)
		return nil
	})
}

// EditCommand saves a user's edit of one command.
func (s *Service) EditCommand(id string, in work.CommandEdit) (*core.Project, error) {
	return s.Store.Update(func(p *core.Project) error {
		if err := blueprintEditable(p); err != nil {
			return err
		}
		if err := work.EditCommand(p, id, in, spec.Artifacts(p)); err != nil {
			return err
		}
		p.Log("ui", "Command diedit: "+id)
		return nil
	})
}

// ReviewCommand marks a command for review, or as reviewed.
func (s *Service) ReviewCommand(id string, needs bool, reason string) (*core.Project, error) {
	return s.Store.Update(func(p *core.Project) error {
		if err := blueprintEditable(p); err != nil {
			return err
		}
		if p.Workflow == nil {
			return errors.New("workflow belum disusun")
		}
		if err := work.ReviewCommand(p, id, needs, reason, spec.Artifacts(p)); err != nil {
			return err
		}
		msg := "Command ditinjau: "
		if needs {
			msg = "Command perlu ditinjau: "
		}
		p.Log("ui", msg+id)
		return nil
	})
}

// CheckWorkflow reports the workflow against the current blueprint.
func CheckWorkflow(p *core.Project) workflow.Report {
	r := workflow.Check(p.Workflow, spec.Artifacts(p), p.Requirements)
	if p.Workflow != nil {
		if issues := work.CheckUI(p, p.Workflow.Commands); len(issues) > 0 {
			r.Issues = append(r.Issues, issues...)
			r.Ready = false
		}
	}
	return r
}

// ---------- Finalize ----------

// Blockers lists what keeps the project from being finalized. Empty means
// Finalize can run.
func Blockers(p *core.Project) []string {
	if p.Discovery == nil || p.Platform == nil {
		return []string{"blueprint belum dibuat dari prompt"}
	}
	out := append(designBlockers(p), scopeBlockers(p)...)
	if work.AppLike(p) {
		switch {
		case p.Stack == nil:
			out = append(out, "keputusan teknologi belum ada")
		case stack.NeedsApproval(p.Stack) && p.Stack.State() != core.TechApproved:
			out = append(out, "teknologi belum disetujui")
		case len(p.TechReview()) > 0:
			out = append(out, "rancangan UI belum ditinjau terhadap perubahan teknologi")
		}
		if st := p.Structure; st == nil || st.Mode != core.StructureExisting && st.State() != core.TechApproved {
			out = append(out, "struktur project belum disetujui")
		}
	}
	return append(out, workflowBlockers(p)...)
}

func workflowBlockers(p *core.Project) []string {
	r := CheckWorkflow(p)
	switch {
	case p.Workflow == nil || len(p.Workflow.Commands) == 0:
		return []string{"command workflow belum disusun"}
	case len(r.Issues) > 0:
		return []string{fmt.Sprintf("workflow: %d masalah dependency/isi", len(r.Issues))}
	case len(r.Review) > 0:
		return []string{fmt.Sprintf("workflow: %d command perlu ditinjau", len(r.Review))}
	}
	return nil
}

// Finalize approves the blueprint and the workflow as one consistent
// version and writes the export package into the project (docs/foundry).
// It approves a specification, not an implementation.
func (s *Service) Finalize() error {
	_, err := s.Store.Update(func(p *core.Project) error {
		if p.Busy() {
			return ErrBusy
		}
		if p.Finalized() {
			return ErrFinalized
		}
		if p.Stage != core.StageDesign {
			return ErrWrongStage
		}
		if p.Discovery == nil || p.Platform == nil {
			return ErrWrongStage
		}
		if b := append(designBlockers(p), scopeBlockers(p)...); len(b) > 0 {
			return fmt.Errorf("%w: %s", ErrIncomplete, b[0])
		}
		if work.AppLike(p) {
			if p.Stack == nil {
				decideStack(p, "")
			}
			if p.Stack == nil {
				return errors.New("keputusan teknologi belum ada")
			}
			if err := stackReady(p); err != nil {
				return err
			}
			if p.Structure == nil {
				return errors.New("struktur project belum ada")
			}
			if err := structureReady(p); err != nil {
				return err
			}
		}
		if b := workflowBlockers(p); len(b) > 0 {
			return fmt.Errorf("%w: %s", ErrIncomplete, b[0])
		}
		if p.Platform != nil {
			p.Platform.Approved = true
		}
		if p.Structure != nil {
			p.Structure.Approved = true
		}
		work.CompleteBlueprint(p)
		// Completing the blueprint may change an artifact a command
		// recorded; the workflow must still match what is exported.
		if r := CheckWorkflow(p); !r.Ready {
			return fmt.Errorf("%w: workflow perlu ditinjau setelah blueprint dilengkapi", ErrIncomplete)
		}
		if b := scopeBlockers(p); len(b) > 0 {
			return fmt.Errorf("%w: %s setelah blueprint dilengkapi", ErrIncomplete, b[0])
		}
		now := time.Now().UTC()
		w := p.Workflow
		w.Status, w.FinalizedAt = core.CommandFinalized, &now
		for i := range w.Commands {
			w.Commands[i].Status = core.CommandFinalized
		}
		p.Stage = core.StageReady
		if err := spec.WriteDocs(p.Dir, p); err != nil {
			return fmt.Errorf("gagal menulis paket ekspor: %w", err)
		}
		p.Log("ui", fmt.Sprintf("Blueprint dan workflow v%d difinalisasi", w.Version))
		return nil
	})
	return err
}

// Reopen unlocks a finalized project for another revision. The exported
// package stays on disk until the next finalize replaces it.
func (s *Service) Reopen() error {
	_, err := s.Store.Update(func(p *core.Project) error {
		if p.Busy() {
			return ErrBusy
		}
		if !p.Finalized() {
			return ErrWrongStage
		}
		unfinalize(p)
		p.Stage = core.StageDesign
		p.Log("ui", "Blueprint dibuka untuk revisi")
		return nil
	})
	return err
}

// unfinalize returns the workflow to draft.
func unfinalize(p *core.Project) {
	if p.Platform != nil {
		p.Platform.Approved = false
	}
	if w := p.Workflow; w != nil {
		w.Status, w.FinalizedAt = core.CommandDraft, nil
		for i := range w.Commands {
			if w.Commands[i].Status == core.CommandFinalized {
				w.Commands[i].Status = core.CommandDraft
			}
		}
	}
}
