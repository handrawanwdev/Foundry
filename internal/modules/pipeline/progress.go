package pipeline

import (
	"errors"
	"time"

	"github.com/handrawanwdev/foundry/internal/config"
	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/progress"
)

// ProgressView is the current execution snapshot. Derived; does not write.
func (s *Service) ProgressView() (progress.Snapshot, error) {
	p, err := s.Store.Load()
	if err != nil {
		return progress.Snapshot{}, err
	}
	return s.ProgressOf(p), nil
}

// ProgressOf is View with this service's MCP heartbeat.
func (s *Service) ProgressOf(p *core.Project) progress.Snapshot {
	return progress.View(p, time.Now().UTC(), s.progressAgent())
}

// ReportProgress stores an agent report and updates the command run.
func (s *Service) ReportProgress(in progress.ReportIn, logSrc string) (*progress.ApplyResult, error) {
	var out *progress.ApplyResult
	_, err := s.Store.Update(func(p *core.Project) error {
		res, err := progress.Apply(p, in, time.Now().UTC())
		if err != nil {
			return err
		}
		out = res
		src := logSrc
		if src == "" {
			src = "agent"
		}
		msg := "Laporan progres " + in.Command + ": " + res.Status
		if res.Duplicate {
			msg = "Laporan progres " + in.Command + " (duplikat)"
		} else if res.Ignored {
			msg = "Laporan progres " + in.Command + " diabaikan (lama)"
		}
		p.Log(src, msg)
		return nil
	})
	return out, err
}

// ReconcileProgress aligns stored runs with the current workflow and
// optionally records a new stale window. It does not invent statuses.
func (s *Service) ReconcileProgress(staleAfterSec int) (progress.Snapshot, error) {
	p, err := s.Store.Update(func(p *core.Project) error {
		if p.Workflow == nil || len(p.Workflow.Commands) == 0 {
			return errors.New("workflow belum disusun")
		}
		now := time.Now().UTC()
		progress.Reconcile(p, now)
		if staleAfterSec > 0 {
			p.Progress.StaleAfterSec = staleAfterSec
		}
		p.Log("system", "Rekonsiliasi progres workflow")
		return nil
	})
	if err != nil {
		return progress.Snapshot{}, err
	}
	return progress.View(p, time.Now().UTC(), s.progressAgent()), nil
}

func (s *Service) progressAgent() progress.Agent {
	c, err := config.Load()
	if err != nil || c == nil || c.MCP == nil || c.MCP.Dir != s.Store.Dir {
		return progress.Agent{}
	}
	return progress.Agent{Source: c.MCP.Client, At: c.MCP.At, Alive: c.MCPAlive(), Tracked: true}
}
