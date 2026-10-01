package pipeline

import (
	"context"
	"fmt"

	"github.com/handrawanw/foundry/internal/core"
	"github.com/handrawanw/foundry/internal/modules/agent"
	"github.com/handrawanw/foundry/internal/modules/work"
)

// The support assistant runs beside the pipeline: it does not take the job
// slot, so users can ask questions while the project is being designed or
// built. Only one question is answered at a time.

// AskSupport records a question and starts answering it.
func (s *Service) AskSupport(question string) error {
	return s.support(func(p *core.Project, status string) error { return work.Ask(p, question, status) })
}

// RetrySupport answers the last failed question again.
func (s *Service) RetrySupport() error {
	return s.support(work.RetrySupport)
}

// ClearSupport starts a new conversation.
func (s *Service) ClearSupport() error {
	_, err := s.Store.Update(work.ClearSupport)
	return err
}

// Escalate stores the summary for the technical team.
func (s *Service) Escalate(e core.Escalation) (*core.Escalation, error) {
	var out *core.Escalation
	_, err := s.Store.Update(func(p *core.Project) error {
		var err error
		if out, err = work.Escalate(p, e); err != nil {
			return err
		}
		p.Log("ui", fmt.Sprintf("Eskalasi %s: %s", out.ID, out.Issue))
		return nil
	})
	return out, err
}

func (s *Service) support(start func(*core.Project, string) error) error {
	runner, isMCP, err := s.AI.Runner()
	if err != nil {
		return err
	}
	status := core.SupportRunning
	if isMCP {
		status = core.SupportWaiting
	}
	p, err := s.Store.Update(func(p *core.Project) error { return start(p, status) })
	if err != nil || isMCP {
		return err
	}
	prompt := work.SupportPrompt(p)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		out, err := runner.Run(context.Background(), agent.Think, p.Dir, prompt)
		if err == nil {
			err = s.applySupport(out)
		}
		if err != nil {
			s.Store.Update(func(p *core.Project) error {
				work.FailSupport(p, err)
				return nil
			})
		}
	}()
	return nil
}

func (s *Service) applySupport(raw string) error {
	_, err := s.Store.Update(func(p *core.Project) error { return work.ApplySupport(p, raw) })
	return err
}

// recoverSupport fails an answer left running by a dead process.
func recoverSupport(p *core.Project) {
	if m := work.PendingSupport(p); m != nil && m.Status == core.SupportRunning {
		m.Status, m.Error = core.SupportFailed, "Proses terhenti sebelum selesai. Coba lagi."
	}
}
