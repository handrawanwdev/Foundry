// Package pipeline is the Foundry orchestrator: it moves one project through
// Tell → Discovery → Design → Plan → Build → Verify → Ready. AI work is done
// either by a connected agent CLI (Foundry drives it) or by an MCP host (the
// host pulls work with next_work and pushes answers with submit_work). Both
// paths go through the same apply functions and the same store.
package pipeline

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/handrawanw/foundry/internal/config"
	"github.com/handrawanw/foundry/internal/core"
	"github.com/handrawanw/foundry/internal/modules/agent"
	"github.com/handrawanw/foundry/internal/modules/structure"
	"github.com/handrawanw/foundry/internal/modules/work"
)

// MaxAttempts before a task becomes BLOCKED.
const MaxAttempts = 3

var (
	ErrBusy        = errors.New("Foundry sedang mengerjakan proses lain")
	ErrNoAI        = errors.New("AI belum terhubung")
	ErrWrongStage  = errors.New("tahap project tidak sesuai")
	ErrGate        = errors.New("task hanya boleh DONE setelah build/test lulus dan acceptance criteria terpenuhi")
	ErrNothingToDo = errors.New("tidak ada pekerjaan untuk AI saat ini")
	ErrIncomplete  = errors.New("rancangan belum lengkap")
)

// AI abstracts where AI work goes. Nil runner means MCP host mode.
type AI interface {
	Runner() (*agent.Runner, bool, error) // runner, isMCP, err
}

// ConfigAI resolves the connection from user config.
type ConfigAI struct{}

func (ConfigAI) Runner() (*agent.Runner, bool, error) {
	c, err := config.Load()
	if err != nil {
		return nil, false, err
	}
	if c.Connection == nil {
		return nil, false, ErrNoAI
	}
	if c.Connection.Mode == config.ModeMCP {
		return nil, true, nil
	}
	r, err := agent.NewRunner(c.Connection.AgentID, c.Connection.Command,
		agent.Options{Model: c.Connection.Model, Effort: c.Connection.Effort})
	return r, false, err
}

type Service struct {
	Store *core.Store
	AI    AI
	// Render takes browser screenshots for the visual check of a design
	// revision; nil skips the check.
	Render Renderer

	mu     sync.Mutex
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func New(store *core.Store, ai AI) *Service {
	return &Service{Store: store, AI: ai}
}

// RecoverStale marks jobs left "running" by a dead process as failed. Only
// the process that runs agent jobs (the HTTP server) should call this.
func (s *Service) RecoverStale() {
	s.Store.Update(func(p *core.Project) error {
		if p.Job != nil && p.Job.Status == core.JobRunning {
			p.Job.Status, p.Job.Error = core.JobFailed, "Proses terhenti sebelum selesai. Jalankan ulang."
			for i := range p.Tasks {
				if p.Tasks[i].Status == core.TaskInProgress {
					p.Tasks[i].Status = core.TaskTodo
				}
			}
		}
		return nil
	})
}

// Wait blocks until background jobs finish (tests, shutdown).
func (s *Service) Wait() { s.wg.Wait() }

// ---------- Tell / Discovery ----------

func (s *Service) SubmitStory(story string) (*core.Project, error) {
	story = strings.TrimSpace(story)
	if len(story) < 10 {
		return nil, errors.New("ceritakan idenya sedikit lebih lengkap")
	}
	p, err := s.Store.Update(func(p *core.Project) error {
		if p.Busy() {
			return ErrBusy
		}
		p.Story = story
		p.Stage = core.StageDiscovery
		p.Log("ui", "Cerita diterima")
		return nil
	})
	if err != nil {
		return nil, err
	}
	return p, s.startAI(work.KindDiscovery)
}

// ---------- Design ----------

// ReviseDesign asks the AI for a change as a patch. screens are the page
// outlines the user saw; scope is what the change may touch (nil: any page).
func (s *Service) ReviseDesign(feedback string, screens map[string]string, scope *core.ChangeScope) error {
	feedback = strings.TrimSpace(feedback)
	if feedback == "" {
		return errors.New("tuliskan perubahan yang diinginkan")
	}
	_, err := s.Store.Update(func(p *core.Project) error {
		if p.Busy() {
			return ErrBusy
		}
		if p.Design == nil {
			return ErrWrongStage
		}
		p.Design.Feedback = append(p.Design.Feedback, feedback)
		p.Design.Screens = screens
		p.Design.Scope = cleanScope(p, scope)
		p.Design.Rejected = ""
		p.Design.Status = core.DesignChanges
		p.Stage = core.StageDesign
		p.Log("ui", "Minta perubahan rancangan: "+feedback)
		return nil
	})
	if err != nil {
		return err
	}
	return s.startAI(work.KindDesign)
}

// KeepDesignChange accepts the last design revision.
func (s *Service) KeepDesignChange() error {
	return s.reviseStep("Perubahan rancangan disimpan", work.KeepRevision)
}

// UndoDesignChange restores the design from before the last revision.
func (s *Service) UndoDesignChange() error {
	return s.reviseStep("Perubahan rancangan dibatalkan", work.UndoRevision)
}

func (s *Service) reviseStep(msg string, fn func(*core.Project) error) error {
	_, err := s.Store.Update(func(p *core.Project) error {
		if p.Busy() {
			return ErrBusy
		}
		if p.Design == nil || p.Design.Status == core.DesignApproved {
			return ErrWrongStage
		}
		if err := fn(p); err != nil {
			return err
		}
		if j := p.Job; j != nil && j.Kind == work.KindDesignQA && j.Status == core.JobWaiting {
			j.Status, j.Message = core.JobDone, "Pemeriksaan visual dilewati"
		}
		p.Log("ui", msg)
		return nil
	})
	return err
}

// SetPlatform lets the user override the suggested platforms.
func (s *Service) SetPlatform(targets []core.PlatformTarget) (*core.Project, error) {
	return s.Store.Update(func(p *core.Project) error {
		if p.Platform == nil || p.Busy() {
			return ErrWrongStage
		}
		var ok []core.PlatformTarget
		for _, t := range targets {
			switch t.Kind {
			case core.PlatformWeb, core.PlatformAndroid, core.PlatformIOS, core.PlatformDesktop, core.PlatformBackend:
				ok = append(ok, t)
			}
		}
		if len(ok) == 0 {
			return errors.New("pilih minimal satu platform")
		}
		p.Platform.Targets = ok
		if p.Structure != nil && p.Structure.Mode == core.StructureNew && !p.Structure.Approved {
			t, reason := structure.Suggest(p.Platform)
			p.Structure.Template, p.Structure.Layout, p.Structure.Reason = t.ID, t.Layout, reason
		}
		p.Log("ui", "Platform diubah")
		return nil
	})
}

func (s *Service) SetTemplate(id string) (*core.Project, error) {
	return s.Store.Update(func(p *core.Project) error {
		if p.Structure == nil || p.Busy() {
			return ErrWrongStage
		}
		if err := structure.Apply(p.Structure, id); err != nil {
			return err
		}
		p.Structure.Reason = "Dipilih oleh user."
		p.Log("ui", "Struktur project: "+id)
		return nil
	})
}

// ApproveDesign locks platform, structure and design, then generates tasks.
func (s *Service) ApproveDesign() error {
	_, err := s.Store.Update(func(p *core.Project) error {
		if p.Busy() {
			return ErrBusy
		}
		if p.Design == nil || len(p.Design.Pages) == 0 || p.Platform == nil || p.Structure == nil {
			return ErrWrongStage
		}
		if b := work.Blockers(p); len(b) > 0 {
			return fmt.Errorf("%w: %d hal wajib dilengkapi", ErrIncomplete, len(b))
		}
		p.Design.Status = core.DesignApproved
		p.Design.Revision = nil // approving keeps any pending change
		p.Platform.Approved = true
		p.Structure.Approved = true
		if err := structure.Scaffold(p.Dir, p.Structure); err != nil {
			return fmt.Errorf("gagal menyiapkan struktur: %w", err)
		}
		p.Stage = core.StagePlan
		p.Log("ui", "Rancangan disetujui")
		return nil
	})
	if err != nil {
		return err
	}
	return s.startAI(work.KindPlan)
}

// ---------- AI jobs (shared by agent and MCP) ----------

func (s *Service) promptFor(p *core.Project, kind string) string {
	switch kind {
	case work.KindDiscovery:
		return work.DiscoveryPrompt(p)
	case work.KindDesign:
		return work.DesignPrompt(p)
	case work.KindPlan:
		return work.PlanPrompt(p)
	case work.KindDesignQA:
		return work.QAPrompt(p)
	}
	return ""
}

// startAI either runs the job with the agent in the background or parks it
// for the MCP host.
func (s *Service) startAI(kind string) error {
	runner, isMCP, err := s.AI.Runner()
	if err != nil {
		s.failJob(kind, err)
		return err
	}
	status := core.JobRunning
	msg := jobMessage(kind)
	if isMCP {
		status, msg = core.JobWaiting, msg+" — menunggu AI Host (MCP)"
	}
	p, err := s.Store.Update(func(p *core.Project) error {
		p.Job = &core.Job{Kind: kind, Status: status, Message: msg, Started: time.Now().UTC()}
		return nil
	})
	if err != nil {
		return err
	}
	if kind == work.KindDesignQA {
		if p, err = s.prepareQA(p); err != nil || p == nil {
			return err // nil project: check skipped, job closed
		}
	}
	if isMCP {
		return nil
	}
	prompt := s.promptFor(p, kind)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		out, err := runner.Run(context.Background(), agent.Think, p.Dir, prompt)
		if err != nil {
			s.failJob(kind, err)
			return
		}
		if err := s.applyAI(kind, out, "agent"); err != nil {
			if s.noteRejection(kind, err) {
				s.startAI(kind) // one retry with the rejection in the prompt
				return
			}
			s.failJob(kind, err)
		}
	}()
	return nil
}

func jobMessage(kind string) string {
	switch kind {
	case work.KindDiscovery:
		return "Memahami kebutuhan"
	case work.KindDesign:
		return "Membuat rancangan aplikasi"
	case work.KindPlan:
		return "Membuat fitur, sub-fitur, dan task"
	case "build":
		return "AI Agent mengerjakan task"
	case "verify":
		return "Build, test, dan verifikasi"
	case work.KindDesignQA:
		return "Memeriksa hasil visual di browser"
	}
	return kind
}

func (s *Service) failJob(kind string, err error) {
	s.Store.Update(func(p *core.Project) error {
		p.Job = &core.Job{Kind: kind, Status: core.JobFailed, Message: jobMessage(kind), Error: err.Error(), Started: time.Now().UTC()}
		p.Log("system", fmt.Sprintf("%s gagal: %v", kind, err))
		return nil
	})
}

// applyAI lands an AI answer in the project and chains the next step.
func (s *Service) applyAI(kind, raw, source string) error {
	var next string
	_, err := s.Store.Update(func(p *core.Project) error {
		if p.Job == nil || p.Job.Kind != kind || p.Job.Status == core.JobDone {
			return fmt.Errorf("%w: tidak ada job %s yang aktif", ErrWrongStage, kind)
		}
		switch kind {
		case work.KindDiscovery:
			if err := work.ApplyDiscovery(p, raw); err != nil {
				return err
			}
			p.Structure = structure.Detect(p.Dir, p.Platform)
			p.Stage = core.StageDesign
			next = work.KindDesign
		case work.KindDesign:
			revising := p.Design != nil && p.Design.Status == core.DesignChanges && len(p.Design.Pages) > 0
			if err := work.ApplyDesign(p, raw); err != nil {
				return err
			}
			if revising && s.Render != nil && p.Design.Revision != nil {
				p.Design.Revision.QA = &core.DesignQA{Status: "running", Round: 1}
				next = work.KindDesignQA
			}
			p.Stage = core.StageDesign
		case work.KindPlan:
			if err := work.ApplyPlan(p, raw); err != nil {
				return err
			}
			p.Stage = core.StageBuild
		case work.KindDesignQA:
			again, err := work.ApplyQA(p, raw)
			if err != nil {
				return err
			}
			if again {
				next = work.KindDesignQA
			}
		default:
			return fmt.Errorf("jenis pekerjaan %q tidak dikenal", kind)
		}
		p.Job = &core.Job{Kind: kind, Status: core.JobDone, Message: jobMessage(kind) + " selesai", Started: time.Now().UTC()}
		p.Log(source, jobMessage(kind)+" selesai")
		return nil
	})
	if err != nil {
		return err
	}
	if next != "" {
		return s.startAI(next)
	}
	return nil
}

// RetryJob reruns the last failed AI job.
func (s *Service) RetryJob() error {
	p, err := s.Store.Load()
	if err != nil {
		return err
	}
	if p.Job == nil || p.Job.Status != core.JobFailed {
		return ErrNothingToDo
	}
	switch p.Job.Kind {
	case work.KindDiscovery, work.KindDesign, work.KindPlan, work.KindDesignQA:
		return s.startAI(p.Job.Kind)
	case "build":
		return s.StartBuild()
	case "verify":
		return s.Verify()
	}
	return ErrNothingToDo
}
