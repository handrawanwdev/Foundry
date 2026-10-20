package pipeline

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/handrawanw/foundry/internal/core"
	"github.com/handrawanw/foundry/internal/modules/spec"
	"github.com/handrawanw/foundry/internal/modules/stack"
	"github.com/handrawanw/foundry/internal/modules/work"
	"github.com/handrawanw/foundry/internal/modules/workflow"
)

// Technology decision workspace: AI recommendation → user change (by hand
// or in words) → validation and impact → apply → approval. Nothing the
// user chose is replaced without the user applying it.

var (
	ErrStackNotApproved = errors.New("setujui keputusan teknologi dulu")
	ErrTechReview       = errors.New("teknologi berubah setelah rancangan UI dibuat: periksa dampaknya ke aturan desain dan layout dulu")
)

// stackEditable: the decision may change until the blueprint is
// finalized.
func stackEditable(p *core.Project) error {
	if p.Busy() {
		return ErrBusy
	}
	if p.Platform == nil || p.Structure == nil || p.Finalized() {
		return ErrWrongStage
	}
	if p.Stack == nil {
		decideStack(p, "")
	}
	return nil
}

// ProposeStack records the user's changes (an empty value returns an area
// to the recommendation) as a proposal with its validation and impact.
func (s *Service) ProposeStack(changes []stack.Change) (*core.Project, error) {
	return s.Store.Update(func(p *core.Project) error {
		if err := stackEditable(p); err != nil {
			return err
		}
		if p.Stack.Request != "" {
			return errors.New("AI masih memproses permintaan perubahan teknologi")
		}
		if err := stack.Propose(p.Stack, changes, "user", "", ""); err != nil {
			return err
		}
		p.Stack.Proposal.Affected = affected(p)
		p.Log("ui", "Usulan perubahan teknologi: "+changeText(p.Stack.Proposal.Changes))
		return nil
	})
}

// RequestStackChange asks the AI to turn a request in words into a
// proposal.
func (s *Service) RequestStackChange(text string) error {
	text, err := work.CleanStackRequest(text)
	if err != nil {
		return err
	}
	_, err = s.Store.Update(func(p *core.Project) error {
		if err := stackEditable(p); err != nil {
			return err
		}
		if p.Job != nil && p.Job.Status == core.JobWaiting {
			return ErrBusy // the AI host still owes another answer
		}
		p.Stack.Request = text
		p.Log("ui", "Minta perubahan teknologi: "+text)
		return nil
	})
	if err != nil {
		return err
	}
	return s.startAI(work.KindStack)
}

// dropStackRequest ends a request the AI could not turn into a change: the
// decision stays as it was and the user is told why.
func (s *Service) dropStackRequest(cause error) error {
	_, err := s.Store.Update(func(p *core.Project) error {
		req := ""
		if p.Stack != nil {
			req, p.Stack.Request = p.Stack.Request, ""
		}
		msg := fmt.Sprintf("Permintaan “%s” tidak mengubah teknologi: %v. Keputusan teknologi tidak diubah. "+
			"Tulis lebih spesifik, mis. “Ganti backend ke Rust + Axum, tetap PostgreSQL”.", req, cause)
		p.Job = &core.Job{Kind: work.KindStack, Status: core.JobFailed, Message: "Permintaan perubahan teknologi", Error: msg, Started: time.Now().UTC()}
		p.Log("system", msg)
		return nil
	})
	return err
}

// ApplyStackProposal makes the waiting proposal the decision: a new
// version, the structure and blueprint follow it. A change after approval
// asks for approval again.
func (s *Service) ApplyStackProposal() (*core.Project, error) {
	return s.Store.Update(func(p *core.Project) error {
		if err := stackEditable(p); err != nil {
			return err
		}
		v, err := stack.Apply(p.Stack)
		if err != nil {
			return err
		}
		followStack(p, v)
		p.Log("ui", fmt.Sprintf("Teknologi v%d diterapkan: %s", v.Version, changeText(v.Changes)))
		return nil
	})
}

// followStack brings the blueprint and the structure to the applied
// version v. Commands that refer to an artifact whose content changed
// become needs_review through their recorded hashes.
func followStack(p *core.Project, v *core.TechVersion) {
	work.ResetBlueprintTech(p, v.Changes)
	work.CompleteBlueprint(p)
	syncStructure(p, "Teknologi diubah: "+changeText(v.Changes))
}

// affected previews the waiting proposal on a copy of p: the artifacts
// whose content would change and the commands that refer to them. nil
// when it cannot be computed.
func affected(p *core.Project) *core.TechAffected {
	b, err := json.Marshal(p)
	if err != nil {
		return nil
	}
	var c core.Project
	if json.Unmarshal(b, &c) != nil {
		return nil
	}
	before := workflow.Hashes(spec.Artifacts(&c))
	v, err := stack.Apply(c.Stack)
	if err != nil {
		return nil
	}
	followStack(&c, v)
	after := workflow.Hashes(spec.Artifacts(&c))
	out := &core.TechAffected{}
	for id, h := range before {
		if after[id] != h {
			out.Artifacts = append(out.Artifacts, id)
		}
	}
	for id := range after {
		if _, ok := before[id]; !ok {
			out.Artifacts = append(out.Artifacts, id)
		}
	}
	sort.Strings(out.Artifacts)
	if p.Workflow != nil {
		for _, cmd := range p.Workflow.Commands {
			if slices.ContainsFunc(cmd.Artifacts, func(r core.ArtifactRef) bool { return slices.Contains(out.Artifacts, r.ID) }) {
				out.Commands = append(out.Commands, cmd.ID)
			}
		}
	}
	return out
}

// DiscardStackProposal cancels the waiting proposal.
func (s *Service) DiscardStackProposal() (*core.Project, error) {
	return s.Store.Update(func(p *core.Project) error {
		if err := stackEditable(p); err != nil {
			return err
		}
		if err := stack.Discard(p.Stack); err != nil {
			return err
		}
		p.Log("ui", "Usulan perubahan teknologi dibatalkan")
		return nil
	})
}

// ConfirmStackReview keeps a choice that was marked for review after a
// decision it depends on changed.
func (s *Service) ConfirmStackReview(area string) (*core.Project, error) {
	return s.Store.Update(func(p *core.Project) error {
		if err := stackEditable(p); err != nil {
			return err
		}
		if p.Stack.Proposal != nil {
			return stack.ErrPending
		}
		if err := stack.ConfirmReview(p.Stack, area); err != nil {
			return err
		}
		p.Log("ui", "Pilihan teknologi "+area+" tetap dipakai: "+p.Stack.Choice(area))
		return nil
	})
}

// ApproveStack approves the current decision; errors block it.
func (s *Service) ApproveStack() (*core.Project, error) {
	return s.Store.Update(func(p *core.Project) error {
		if err := stackEditable(p); err != nil {
			return err
		}
		if err := stack.Approve(p.Stack); err != nil {
			return err
		}
		p.Log("ui", fmt.Sprintf("Teknologi v%d disetujui", p.Stack.Version()))
		return nil
	})
}

// RecommendStack analyzes the requirement again; the user's choices stay.
func (s *Service) RecommendStack() (*core.Project, error) {
	return s.Store.Update(func(p *core.Project) error {
		if err := stackEditable(p); err != nil {
			return err
		}
		if p.Stack.Proposal != nil {
			return stack.ErrPending
		}
		decideStack(p, "Analisa ulang kebutuhan")
		p.Log("ui", "Rekomendasi teknologi diperbarui")
		return nil
	})
}

// AckTechReview marks the UI design as checked against the current
// technology; the design itself is not changed.
func (s *Service) AckTechReview() (*core.Project, error) {
	return s.Store.Update(func(p *core.Project) error {
		if p.Busy() {
			return ErrBusy
		}
		if len(p.TechReview()) == 0 {
			return ErrNothingToDo
		}
		p.UI.TechVersion = p.Stack.Version()
		p.Log("ui", fmt.Sprintf("Rancangan diperiksa terhadap teknologi v%d", p.Stack.Version()))
		return nil
	})
}

// stackReady is what finalizing needs from the technology: an
// approved decision and no unreviewed change that affects the UI design. An
// existing project's stack that is only what its code uses is approved
// with it; anything Foundry recommended or the user changed there is
// approved explicitly.
func stackReady(p *core.Project) error {
	st := p.Stack
	if !stack.NeedsApproval(st) {
		if st.State() != core.TechApproved {
			if err := stack.Approve(st); err != nil {
				return err
			}
		}
		return nil
	}
	if st.State() != core.TechApproved {
		return ErrStackNotApproved
	}
	if len(p.TechReview()) > 0 {
		return ErrTechReview
	}
	return nil
}

// decideStack recommends the stack from the requirement and platform,
// keeps the user's choices, saves a version when the result differs and
// derives a new project's structure from it. why names the trigger.
func decideStack(p *core.Project, why string) {
	old := p.Stack
	next := stack.Recommend(p)
	stack.Carry(old, next)
	stack.Refresh(next)
	p.Stack = next
	switch {
	case old == nil || len(old.History) == 0 && len(old.Decisions) == 0:
		next.SetStatus(core.TechRecommended)
		stack.Record(next, nil, "recommend", why)
	default:
		before := old.Decisions
		if len(stack.Diff(before, next.Decisions)) > 0 {
			stackChanged(p, before, "recommend", why)
			work.ResetBlueprintTech(p, stack.Diff(before, next.Decisions))
		} else if len(next.History) == 0 {
			stack.Record(next, nil, "recommend", why)
		}
	}
	work.CompleteBlueprint(p)
	syncStructure(p, why)
}

// stackChanged saves a version after the decisions changed from before;
// a change after approval asks for approval again.
func stackChanged(p *core.Project, before []core.TechDecision, source, request string) {
	st := p.Stack
	switch st.State() {
	case core.TechApproved, core.TechChanged:
		st.SetStatus(core.TechChanged)
	case core.TechRecommended:
		if source != "recommend" {
			st.SetStatus(core.TechImpactChecked)
		}
	}
	stack.Refresh(st)
	stack.Record(st, before, source, request)
}

func changeText(cs []core.TechChange) string {
	var out []string
	for _, c := range cs {
		out = append(out, fmt.Sprintf("%s %s → %s", c.Area, orNone(c.From), orNone(c.To)))
	}
	return strings.Join(out, ", ")
}

func orNone(v string) string {
	if v == "" {
		return "—"
	}
	return v
}
