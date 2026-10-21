// Package pipeline is the Foundry orchestrator: it moves one project through
// Prompt → Blueprint (with design rules and layout spec when the product has
// screens) →
// Command Workflow → Finalize → Export. Foundry never runs the workflow. AI work is done
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
	"github.com/handrawanw/foundry/internal/modules/spec"
	"github.com/handrawanw/foundry/internal/modules/structure"
	"github.com/handrawanw/foundry/internal/modules/work"
)

var (
	ErrBusy        = errors.New("Foundry sedang mengerjakan proses lain")
	ErrNoAI        = errors.New("AI belum terhubung")
	ErrWrongStage  = errors.New("tahap project tidak sesuai")
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

	// ctx ends with Shutdown; every job this service runs derives from it.
	ctx  context.Context
	stop context.CancelFunc

	wg sync.WaitGroup
}

func New(store *core.Store, ai AI) *Service {
	ctx, stop := context.WithCancel(context.Background())
	return &Service{Store: store, AI: ai, ctx: ctx, stop: stop}
}

// context is the parent of every job: cancelled by Shutdown.
func (s *Service) context() context.Context {
	if s.ctx == nil {
		return context.Background()
	}
	return s.ctx
}

// Shutdown stops every job of this service: agents and the processes they
// started are killed through their cancelled context. It waits up to
// timeout for the jobs to return and record their state.
func (s *Service) Shutdown(timeout time.Duration) error {
	if s.stop != nil {
		s.stop()
	}
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("job belum berhenti setelah %s", timeout)
	}
}

// RecoverStale marks jobs left "running" by a dead process as failed. Only
// the process that runs agent jobs (the HTTP server) should call this.
func (s *Service) RecoverStale() error {
	_, err := s.Store.Update(func(p *core.Project) error {
		if p.Job != nil && p.Job.Status == core.JobRunning {
			p.Job.Status, p.Job.Error = core.JobFailed, "Proses terhenti sebelum selesai. Jalankan ulang."
		}
		return nil
	})
	return err
}

// Wait blocks until background jobs finish (tests, shutdown).
func (s *Service) Wait() { s.wg.Wait() }

// ---------- Tell / Discovery ----------

func (s *Service) SubmitStory(story string) (*core.Project, error) {
	return s.SubmitStoryWith(story, nil)
}

func (s *Service) SubmitStoryWith(story string, survey *core.Survey) (*core.Project, error) {
	story = strings.TrimSpace(story)
	if len(story) < 10 {
		return nil, errors.New("ceritakan idenya sedikit lebih lengkap")
	}
	p, err := s.Store.Update(func(p *core.Project) error {
		if p.Busy() {
			return ErrBusy
		}
		p.Story = story
		if survey != nil {
			survey.FillDefaults()
			if survey.What == "" {
				survey.What = story
			}
			p.Survey = survey
		} else if p.Survey == nil {
			s := &core.Survey{What: story}
			s.FillDefaults()
			p.Survey = s
		}
		unfinalize(p) // a new prompt reopens a finalized blueprint
		p.Stage = core.StageDiscovery
		attachStructure(p)
		p.Log("ui", "Cerita diterima")
		return nil
	})
	if err != nil {
		return nil, err
	}
	return p, s.startAI(work.KindDiscovery)
}

// DetectStructure records whether the folder already has a codebase, so
// Tell/Discovery copy and the discovery prompt can follow it.
func (s *Service) DetectStructure() (*core.Project, error) {
	return s.Store.Update(func(p *core.Project) error {
		attachStructure(p)
		return nil
	})
}

// attachStructure scans the project dir. An existing codebase wins; a new
// project's IR is kept if it is already there.
func attachStructure(p *core.Project) {
	d := structure.Detect(p.Dir, p.Platform)
	if d.Mode == core.StructureExisting || p.Structure == nil {
		p.Structure = d
	}
}

// SetPlatform lets the user override the suggested platforms.
func (s *Service) SetPlatform(targets []core.PlatformTarget) (*core.Project, error) {
	return s.Store.Update(func(p *core.Project) error {
		if p.Platform == nil || p.Busy() || p.Finalized() {
			return ErrWrongStage
		}
		var ok []core.PlatformTarget
		for _, t := range targets {
			switch t.Kind {
			case core.PlatformWeb, core.PlatformAndroid, core.PlatformIOS, core.PlatformDesktop, core.PlatformBackend,
				core.PlatformCLI, core.PlatformEmbedded, core.PlatformOther:
				ok = append(ok, t)
			}
		}
		if len(ok) == 0 {
			return errors.New("pilih minimal satu platform")
		}
		p.Platform.Targets = ok
		decideStack(p, "Platform diubah")
		p.Log("ui", "Platform diubah")
		return nil
	})
}

// ---------- AI jobs (shared by agent and MCP) ----------

func (s *Service) promptFor(p *core.Project, kind string) string {
	switch kind {
	case work.KindDiscovery:
		return work.DiscoveryPrompt(p)
	case work.KindDesign:
		return work.DesignPrompt(p)
	case work.KindStack:
		return work.StackPrompt(p)
	case work.KindStructure:
		return work.StructurePrompt(p)
	case work.KindArtifacts:
		return work.ArtifactsPrompt(p, spec.Artifacts(p))
	case work.KindWorkflow:
		return work.WorkflowPrompt(p, spec.Artifacts(p))
	}
	return ""
}

// startAI either runs the job with the agent in the background or parks it
// for the MCP host.
func (s *Service) startAI(kind string) error { return s.startAIIf(kind, nil) }

// startAIIf is startAI with a precondition checked in the same store update
// that starts the job, so two callers can never both start it. A failed
// precondition starts nothing and writes nothing.
func (s *Service) startAIIf(kind string, expect func(*core.Project) error) error {
	runner, isMCP, err := s.AI.Runner()
	if err != nil {
		s.failJobIf(kind, err, expect)
		return err
	}
	status := core.JobRunning
	msg := jobMessage(kind)
	if isMCP {
		status, msg = core.JobWaiting, msg+" — menunggu AI Host (MCP)"
	}
	p, err := s.Store.Update(func(p *core.Project) error {
		if expect != nil {
			if err := expect(p); err != nil {
				return err
			}
		}
		p.Job = &core.Job{Kind: kind, Status: status, Message: msg, Started: time.Now().UTC()}
		return nil
	})
	if err != nil {
		return err
	}
	if isMCP {
		return nil
	}
	prompt := s.promptFor(p, kind)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		out, err := runner.Run(s.context(), p.Dir, prompt)
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
		return "Menyusun aturan desain dan spesifikasi layout"
	case work.KindStack:
		return "Memahami permintaan perubahan teknologi"
	case work.KindStructure:
		return "Memahami permintaan perubahan struktur"
	case work.KindArtifacts:
		return "Merevisi blueprint"
	case work.KindWorkflow:
		return "Menyusun command workflow"
	}
	return kind
}

func (s *Service) failJob(kind string, err error) { s.failJobIf(kind, err, nil) }

// failJobIf records a failed job only while expect holds (nil: always), so
// a late failure never overwrites a job that started after it.
func (s *Service) failJobIf(kind string, err error, expect func(*core.Project) error) {
	s.Store.Update(func(p *core.Project) error {
		if expect != nil {
			if err := expect(p); err != nil {
				return err
			}
		}
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
			defer stampRefs(p)
			p.Structure = structure.Detect(p.Dir, p.Platform)
			readExistingUI(p)
			decideStack(p, "")
			p.Stage = core.StageDesign
			if p.HasUI() {
				next = work.KindDesign
			}
		case work.KindDesign:
			if err := work.ApplyDesign(p, raw); err != nil {
				return err
			}
		case work.KindStack:
			if err := work.ApplyStackChange(p, raw); err != nil {
				return err
			}
			p.Stack.Proposal.Affected = affected(p)
		case work.KindStructure:
			if err := work.ApplyStructureChange(p, raw); err != nil {
				return err
			}
		case work.KindArtifacts:
			if err := work.ApplyArtifacts(p, raw); err != nil {
				return err
			}
			stampRefs(p)
			if w := p.Workflow; w != nil && w.Request != nil && w.Request.Cascade {
				next = work.KindWorkflow // the cleanup of a deleted document continues with its commands
			}
		case work.KindWorkflow:
			if err := work.ApplyWorkflow(p, raw, spec.Artifacts(p)); err != nil {
				return err
			}
		default:
			return fmt.Errorf("jenis pekerjaan %q tidak dikenal", kind)
		}
		p.Job = &core.Job{Kind: kind, Status: core.JobDone, Message: jobMessage(kind) + " selesai", Started: time.Now().UTC()}
		p.Log(source, jobMessage(kind)+" selesai")
		return nil
	})
	if kind == work.KindStack && err != nil {
		return s.dropStackRequest(err)
	}
	if kind == work.KindStructure && err != nil {
		return s.dropStructureRequest(err)
	}
	if err != nil {
		return err
	}
	if next != "" {
		return s.startAI(next)
	}
	return nil
}

// noteRejection keeps why a design, workflow or blueprint answer was
// refused on its request, for the retry prompt. Only the first refusal
// retries automatically.
func (s *Service) noteRejection(kind string, err error) bool {
	if err == nil || kind != work.KindWorkflow && kind != work.KindArtifacts && kind != work.KindDesign {
		return false
	}
	retry := false
	s.Store.Update(func(p *core.Project) error {
		var r **core.ChangeRequest
		switch kind {
		case work.KindArtifacts:
			r = &p.ArtifactRequest
		case work.KindDesign:
			if p.UI == nil {
				p.UI = &core.UIDesign{}
			}
			r = &p.UI.Request
		default:
			if p.Workflow == nil {
				p.Workflow = &core.Workflow{Status: core.CommandDraft}
			}
			r = &p.Workflow.Request
		}
		if *r == nil {
			*r = &core.ChangeRequest{}
		}
		retry = (*r).Rejected == ""
		(*r).Rejected = err.Error()
		p.Log("system", err.Error())
		return nil
	})
	return retry
}

// RetryJob reruns the last failed AI job.
// DismissJob closes a failed job without retrying it; the error stays in
// the log.
func (s *Service) DismissJob() error {
	_, err := s.Store.Update(func(p *core.Project) error {
		if p.Job == nil || p.Job.Status != core.JobFailed {
			return ErrNothingToDo
		}
		switch p.Job.Kind {
		case work.KindArtifacts:
			p.ArtifactRequest = nil
			if p.Workflow != nil && p.Workflow.Request != nil && p.Workflow.Request.Cascade {
				p.Workflow.Request = nil
			}
		case work.KindDesign:
			if p.UI != nil {
				p.UI.Request = nil
				if !p.UI.Written() {
					p.UI = nil
				}
			}
		case work.KindWorkflow:
			if p.Workflow != nil {
				p.Workflow.Request = nil
			}
		}
		p.Job = nil
		p.Log("ui", "Pesan gagal ditutup")
		return nil
	})
	return err
}

func (s *Service) RetryJob() error {
	p, err := s.Store.Load()
	if err != nil {
		return err
	}
	if p.Job == nil || p.Job.Status != core.JobFailed {
		return ErrNothingToDo
	}
	// The check above is an early answer only; failedJob repeats it inside
	// the update that starts the job, so concurrent retries start it once.
	kind := p.Job.Kind
	switch kind {
	case work.KindDiscovery, work.KindDesign:
		return s.startAIIf(kind, failedJob(kind))
	case work.KindStack:
		if p.Stack != nil && p.Stack.Request != "" {
			return s.startAIIf(kind, failedJob(kind))
		}
	case work.KindStructure:
		if p.Structure != nil && p.Structure.Request != "" {
			return s.startAIIf(kind, failedJob(kind))
		}
	case work.KindArtifacts:
		if p.ArtifactRequest != nil {
			return s.startAIIf(kind, failedJob(kind))
		}
	case work.KindWorkflow:
		return s.startAIIf(kind, failedJob(kind))
	}
	return ErrNothingToDo
}

// failedJob holds while the project's job is still the failed job of kind.
func failedJob(kind string) func(*core.Project) error {
	return func(p *core.Project) error {
		if p.Busy() {
			return ErrBusy
		}
		if p.Job == nil || p.Job.Status != core.JobFailed || p.Job.Kind != kind {
			return ErrNothingToDo
		}
		return nil
	}
}
