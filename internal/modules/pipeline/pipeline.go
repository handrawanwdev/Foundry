// Package pipeline is the Foundry orchestrator: it moves one project through
// Prompt → Blueprint (with UI design when the product has screens) →
// Command Workflow → Finalize → Export. Foundry never runs the workflow. AI work is done
// either by a connected agent CLI (Foundry drives it) or by an MCP host (the
// host pulls work with next_work and pushes answers with submit_work). Both
// paths go through the same apply functions and the same store.
package pipeline

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
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
	// Render takes browser screenshots for the visual check of a new
	// design and of a revision; nil skips the check.
	Render Renderer

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
		if p.Design == nil || p.Design.Status == core.DesignApproved {
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
	return s.reviseStep("Perubahan rancangan disimpan", "keep", work.KeepRevision)
}

// UndoDesignChange restores the design from before the last revision.
func (s *Service) UndoDesignChange() error {
	return s.reviseStep("Perubahan rancangan dibatalkan", "undo", work.UndoRevision)
}

func (s *Service) reviseStep(msg, source string, fn func(*core.Project) error) error {
	p, err := s.Store.Update(func(p *core.Project) error {
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
		measureQuality(p)
		recordVersion(p, source, "")
		p.Log("ui", msg)
		return nil
	})
	if err == nil {
		saveQuality(p)
	}
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
			case core.PlatformWeb, core.PlatformAndroid, core.PlatformIOS, core.PlatformDesktop, core.PlatformBackend,
				core.PlatformCLI, core.PlatformEmbedded, core.PlatformOther:
				ok = append(ok, t)
			}
		}
		if len(ok) == 0 {
			return errors.New("pilih minimal satu platform")
		}
		p.Platform.Targets = ok
		if p.Design == nil || p.Design.Status != core.DesignApproved {
			decideStack(p, "Platform diubah")
		}
		p.Log("ui", "Platform diubah")
		return nil
	})
}

// SetTheme picks the design's theme ("" lets Foundry select one from the
// requirement). A draft design follows it at once: its tokens and style
// change, its pages (Design IR) do not.
func (s *Service) SetTheme(id string) (*core.Project, error) {
	p, err := s.Store.Update(func(p *core.Project) error {
		if p.Busy() {
			return ErrBusy
		}
		if id != "" {
			themes, _ := work.Themes(p.Dir)
			if !slices.ContainsFunc(themes, func(t core.Theme) bool { return t.ID == id }) {
				return fmt.Errorf("theme %q tidak tersedia", id)
			}
		}
		d := p.Design
		designed := d != nil && len(d.Pages) > 0
		if designed && d.Status == core.DesignApproved {
			return ErrWrongStage
		}
		if designed && d.Revision != nil {
			return errors.New("simpan atau batalkan perubahan rancangan dulu")
		}
		p.Theme = id
		if designed {
			t, ctx := work.ResolveTheme(p)
			work.ApplyTheme(d, t, ctx)
			measureQuality(p)
			recordVersion(p, "theme", cmp.Or(id, "otomatis"))
		}
		p.Log("ui", "Theme: "+cmp.Or(id, "otomatis"))
		return nil
	})
	if err == nil {
		saveQuality(p)
	}
	return p, err
}

// ApproveDesign is Finalize, kept for the design workspace and older
// clients. ReopenDesign is Reopen.
func (s *Service) ApproveDesign() error { return s.Finalize() }

// ReopenDesign unlocks a finalized project so it can be revised again.
func (s *Service) ReopenDesign() error { return s.Reopen() }

// ---------- AI jobs (shared by agent and MCP) ----------

func (s *Service) promptFor(p *core.Project, kind string) string {
	switch kind {
	case work.KindDiscovery:
		return work.DiscoveryPrompt(p)
	case work.KindDesign:
		return work.DesignPrompt(p)
	case work.KindDesignQA:
		return work.QAPrompt(p)
	case work.KindReference:
		return work.ReferencePrompt(p)
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
	if kind == work.KindDesignQA {
		if p, err = s.prepareQA(p); err != nil || p == nil {
			return err // nil project: check skipped, job closed
		}
	}
	if kind == work.KindReference {
		if p, err = s.prepareReference(p); err != nil {
			s.failJob(kind, err)
			return err
		}
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
		return "Membuat rancangan aplikasi"
	case work.KindDesignQA:
		return "Memeriksa hasil visual di browser"
	case work.KindReference:
		return "Menganalisis referensi desain"
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

// dropUnclearRequest ends a change request the AI found nothing to change
// for: the request is removed and the design is back to draft, unchanged,
// so a retry does not ask the same thing again. The user is told to write
// a clearer request.
func (s *Service) dropUnclearRequest() error {
	_, err := s.Store.Update(func(p *core.Project) error {
		req := ""
		if d := p.Design; d != nil && d.Status == core.DesignChanges {
			if n := len(d.Feedback); n > 0 {
				req, d.Feedback = d.Feedback[n-1], d.Feedback[:n-1]
			}
			d.Status, d.Scope, d.Screens, d.Rejected = core.DesignDraft, nil, nil, ""
		}
		msg := fmt.Sprintf("AI tidak menemukan perubahan untuk permintaan “%s”. Rancangan tidak diubah. "+
			"Tulis lebih spesifik: halaman, bagian, dan hasil yang diinginkan (mis. “Halaman Antrean: tabel lebih compact, filter di atas”).", req)
		p.Job = &core.Job{Kind: work.KindUnclear, Status: core.JobFailed, Message: "Permintaan perubahan", Error: msg, Started: time.Now().UTC()}
		p.Log("system", "Permintaan perubahan tidak jelas: "+req)
		return nil
	})
	return err
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
	measured := false
	p, err := s.Store.Update(func(p *core.Project) error {
		if p.Job == nil || p.Job.Kind != kind || p.Job.Status == core.JobDone {
			return fmt.Errorf("%w: tidak ada job %s yang aktif", ErrWrongStage, kind)
		}
		switch kind {
		case work.KindDiscovery:
			if err := work.ApplyDiscovery(p, raw); err != nil {
				return err
			}
			p.Structure = structure.Detect(p.Dir, p.Platform)
			readExistingUI(p)
			decideStack(p, "")
			p.Stage = core.StageDesign
			if p.HasUI() {
				next = work.KindDesign
				if r := p.Reference; r != nil && r.Status == "pending" {
					next = work.KindReference // the design follows the reference's theme
				}
			}
		case work.KindReference:
			if err := applyReference(p, raw); err != nil {
				return err
			}
			measured = p.Design != nil && p.Design.Quality != nil
			if p.Design == nil || len(p.Design.Pages) == 0 {
				next = work.KindDesign
			}
		case work.KindDesign:
			revising := p.Design != nil && p.Design.Status == core.DesignChanges && len(p.Design.Pages) > 0
			if err := work.ApplyDesign(p, raw); err != nil {
				return err
			}
			if !revising {
				p.Design.TechVersion = p.Stack.Version() // made with this decision
			}
			switch {
			case revising && s.Render != nil && p.Design.Revision != nil:
				p.Design.Revision.QA = &core.DesignQA{Status: "running", Round: 1}
				next = work.KindDesignQA
			case !revising && s.Render != nil:
				// A new design is critiqued before the user sees it.
				p.Design.QA = &core.DesignQA{Status: "running", Round: 1}
				next = work.KindDesignQA
			case !revising:
				// No browser: only the design rules are checked.
				if lint := work.LintDesign(p, nil); len(lint) > 0 {
					p.Design.QA = &core.DesignQA{Status: "skipped", Notes: "browser tidak tersedia; hanya aturan desain yang diperiksa", Issues: lint}
				}
			}
			p.Stage = core.StageDesign
			measureQuality(p)
			measured = true
		case work.KindStack:
			if err := work.ApplyStackChange(p, raw); err != nil {
				return err
			}
		case work.KindStructure:
			if err := work.ApplyStructureChange(p, raw); err != nil {
				return err
			}
		case work.KindArtifacts:
			if err := work.ApplyArtifacts(p, raw); err != nil {
				return err
			}
		case work.KindWorkflow:
			if err := work.ApplyWorkflow(p, raw, spec.Artifacts(p)); err != nil {
				return err
			}
		case work.KindDesignQA:
			again, err := work.ApplyQA(p, raw)
			if err != nil {
				return err
			}
			measureQuality(p) // re-measure after the critic round
			measured = true
			if again {
				next = work.KindDesignQA
			}
		default:
			return fmt.Errorf("jenis pekerjaan %q tidak dikenal", kind)
		}
		recordAIVersion(p, kind)
		p.Job = &core.Job{Kind: kind, Status: core.JobDone, Message: jobMessage(kind) + " selesai", Started: time.Now().UTC()}
		p.Log(source, jobMessage(kind)+" selesai")
		return nil
	})
	if kind == work.KindDesign && errors.Is(err, work.ErrNoChange) {
		return s.dropUnclearRequest()
	}
	if kind == work.KindStack && err != nil {
		return s.dropStackRequest(err)
	}
	if kind == work.KindStructure && err != nil {
		return s.dropStructureRequest(err)
	}
	if err != nil {
		return err
	}
	if measured {
		saveQuality(p)
	}
	if next != "" {
		return s.startAI(next)
	}
	return nil
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
	case work.KindDiscovery, work.KindDesign, work.KindDesignQA, work.KindReference:
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
