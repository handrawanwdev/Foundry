package pipeline

import (
	"fmt"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/spec"
	"github.com/handrawanwdev/foundry/internal/modules/work"
	"github.com/handrawanwdev/foundry/internal/modules/workflow"
)

// Blueprint scope: the areas the blueprint covers or leaves out, use
// cases, acceptance criteria, the decision register and the review of
// artifacts whose references changed.

// stampRefs records the ref hashes of written artifacts that have none
// yet; call it after any blueprint change.
func stampRefs(p *core.Project) {
	work.StampRefs(p, workflow.Hashes(spec.Artifacts(p)))
}

// scopeBlockers lists what keeps the blueprint scope from being final:
// areas not written or invalid, critical decisions without the user's
// decision, an invalid decision register and artifacts to review. The UI
// design has its own blockers.
func scopeBlockers(p *core.Project) []string {
	var out []string
	docs := spec.Artifacts(p)
	for _, a := range core.Coverage(p, docs) {
		if a.ID == core.AreaUI {
			continue
		}
		switch a.Status {
		case core.AreaMissing:
			out = append(out, fmt.Sprintf("area %s belum ditulis atau dilewati dengan alasan", a.Title))
		case core.AreaInvalid:
			out = append(out, fmt.Sprintf("area %s: %d hal wajib diperbaiki", a.Title, a.Errors))
		}
	}
	if n := len(core.DecisionIssues(p.Decisions)); n > 0 {
		out = append(out, fmt.Sprintf("register keputusan: %d hal wajib diperbaiki", n))
	}
	for _, d := range p.OpenCritical() {
		out = append(out, fmt.Sprintf("keputusan kritis %s belum diputuskan: %s", d.ID, d.Text))
	}
	if len(p.DecisionSync) > 0 {
		out = append(out, fmt.Sprintf("keputusan %s berubah; artefak dan command belum disesuaikan", strings.Join(p.DecisionSync, ", ")))
	}
	review := 0
	for _, st := range core.ArtifactStates(p, docs) {
		if st.Status == core.StateNeedsReview {
			review++
		}
	}
	if review > 0 {
		out = append(out, fmt.Sprintf("%d artefak perlu ditinjau karena artefak acuannya berubah", review))
	}
	return out
}

// CompleteScope asks the AI to write or fix the blueprint areas that are
// missing or invalid.
func (s *Service) CompleteScope() error {
	return s.startAIIf(work.KindArtifacts, func(p *core.Project) error {
		if err := blueprintEditable(p); err != nil {
			return err
		}
		text := work.ScopeRequest(p, spec.Artifacts(p))
		if text == "" {
			return fmt.Errorf("%w: semua area blueprint sudah dicakup atau dilewati", ErrNothingToDo)
		}
		p.ArtifactRequest = &core.ChangeRequest{Text: text}
		p.Log("ui", "Minta AI melengkapi cakupan blueprint")
		return nil
	})
}

// SyncDecisions asks the AI to bring the artifacts, use cases, acceptance
// criteria and then the commands in line with the decisions the user
// changed.
func (s *Service) SyncDecisions() error {
	return s.startAIIf(work.KindArtifacts, func(p *core.Project) error {
		if err := blueprintEditable(p); err != nil {
			return err
		}
		text := work.DecisionSyncRequest(p)
		if text == "" {
			return fmt.Errorf("%w: blueprint sudah sesuai dengan keputusan", ErrNothingToDo)
		}
		p.ArtifactRequest = &core.ChangeRequest{Text: text, Sync: true}
		p.Log("ui", "Minta AI menyesuaikan blueprint dengan keputusan")
		return nil
	})
}

// scopeEdit runs a user's edit of the blueprint scope.
func (s *Service) scopeEdit(msg string, edit func(p *core.Project) error) (*core.Project, error) {
	return s.Store.Update(func(p *core.Project) error {
		if err := blueprintEditable(p); err != nil {
			return err
		}
		if err := edit(p); err != nil {
			return err
		}
		stampRefs(p)
		p.Log("ui", msg)
		return nil
	})
}

func (s *Service) EditUseCases(in []core.Flow) (*core.Project, error) {
	return s.scopeEdit("Use case diedit", func(p *core.Project) error { return work.EditUseCases(p, in) })
}

func (s *Service) EditAcceptance(in []work.AcceptanceIn, done []string) (*core.Project, error) {
	return s.scopeEdit("Acceptance criteria diedit", func(p *core.Project) error { return work.EditAcceptance(p, in, done) })
}

func (s *Service) ResolveDecision(id, resolution string) (*core.Project, error) {
	msg := "Keputusan " + id + " diputuskan"
	if resolution == "" {
		msg = "Keputusan " + id + " dibuka lagi"
	}
	return s.scopeEdit(msg, func(p *core.Project) error { return work.ResolveDecision(p, id, resolution) })
}

func (s *Service) SkipArea(area, reason string) (*core.Project, error) {
	return s.scopeEdit("Area "+area+" dilewati: "+reason, func(p *core.Project) error { return work.SkipArea(p, area, reason) })
}

func (s *Service) UnskipArea(area string) (*core.Project, error) {
	return s.scopeEdit("Area "+area+" dicakup lagi", func(p *core.Project) error { return work.UnskipArea(p, area) })
}

// ReviewArtifact records that a written artifact was checked against the
// artifacts it refers to.
func (s *Service) ReviewArtifact(id string) (*core.Project, error) {
	return s.scopeEdit("Artefak ditinjau: "+id, func(p *core.Project) error {
		return work.ReviewArtifact(p, id, workflow.Hashes(spec.Artifacts(p)))
	})
}
