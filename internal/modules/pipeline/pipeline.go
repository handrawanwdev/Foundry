// Package pipeline is the Foundry orchestrator: it moves one project through
// Prompt → Feature Discovery → Blueprint (with design rules and layout spec
// when the product has screens) →
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

	"github.com/handrawanwdev/foundry/internal/config"
	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/agent"
	"github.com/handrawanwdev/foundry/internal/modules/repoplan"
	"github.com/handrawanwdev/foundry/internal/modules/scan"
	"github.com/handrawanwdev/foundry/internal/modules/spec"
	"github.com/handrawanwdev/foundry/internal/modules/structure"
	"github.com/handrawanwdev/foundry/internal/modules/techplan"
	"github.com/handrawanwdev/foundry/internal/modules/usage"
	"github.com/handrawanwdev/foundry/internal/modules/work"
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

	// RetryWait is the pause before each new attempt when the AI server is
	// overloaded; its length bounds the attempts.
	RetryWait []time.Duration

	// Repair bounds the automatic repair of a refused technology plan and
	// names the model a stuck repair escalates to.
	Repair RepairPolicy

	// AreaParallel bounds the blueprint areas an agent writes at once.
	// 0 uses the default, 1 runs them one after another.
	AreaParallel int

	// ctx ends with Shutdown; every job this service runs derives from it.
	ctx  context.Context
	stop context.CancelFunc

	wg sync.WaitGroup
}

func New(store *core.Store, ai AI) *Service {
	ctx, stop := context.WithCancel(context.Background())
	return &Service{Store: store, AI: ai, RetryWait: []time.Duration{time.Minute, 2 * time.Minute}, ctx: ctx, stop: stop}
}

// Snapshot loads the project for a reader. Derived state that is cheap and
// useful in the UI (whether the AI structure analysis is still current) is
// refreshed on the copy; nothing is written.
func (s *Service) Snapshot() (*core.Project, error) {
	p, err := s.Store.Load()
	if err != nil {
		return nil, err
	}
	if p.Structure != nil {
		structure.RefreshPlan(p.Structure, p)
		repoplan.Refresh(p, p.Structure.Repo)
		if p.Structure.Mode == core.StructureExisting && p.Structure.Context != nil {
			scan.MarkStale(p.Dir, p.Structure.Context)
		}
	}
	if p.Stack != nil && p.Stack.Plan != nil {
		techplan.Refresh(p, p.Stack.Plan)
	}
	withETA(p)
	if p.BlueprintProgress != nil {
		p.BlueprintProgress.Events = nil // the timeline has its own endpoint and stream
	}
	return p, nil
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
			trackFail(p, p.Job.Kind, fmt.Errorf("%w: %s", context.Canceled, p.Job.Error))
		}
		recoverRun(p)
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
	return s.SubmitStoryIntent(story, survey, "", "")
}

// SubmitStoryIntent is SubmitStoryWith plus the work intent and the user's
// corrections of the repository scan.
func (s *Service) SubmitStoryIntent(story string, survey *core.Survey, intent core.WorkIntent, corrections string) (*core.Project, error) {
	story = strings.TrimSpace(story)
	if len(story) < 10 {
		return nil, errors.New("ceritakan idenya sedikit lebih lengkap")
	}
	survey.FillDefaults()
	if err := survey.Check(); err != nil {
		return nil, err
	}
	corrections = strings.TrimSpace(corrections)
	p, err := s.Store.Update(func(p *core.Project) error {
		if p.Busy() {
			return ErrBusy
		}
		p.Story = story
		if survey != nil {
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
		p.FeatureMap = &core.FeatureMap{Request: &core.FeatureMapRequest{Action: core.FeatureMapGenerate}}
		p.Change = nil
		attachStructure(p)
		p.Intent = core.ClampIntent(existingFolder(p), intent)
		if corrections != "" && p.Structure != nil && p.Structure.Mode == core.StructureExisting {
			if p.Structure.Context == nil {
				p.Structure.Context = &core.ProjectContext{}
			}
			p.Structure.Context.Corrections = corrections
		}
		p.Log("ui", "Cerita diterima")
		return nil
	})
	if err != nil {
		return nil, err
	}
	return p, s.startAI(work.KindFeatureMap)
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
// project's IR is kept if it is already there. User corrections of the
// scan are kept when the context is rebuilt.
func attachStructure(p *core.Project) {
	var prev *core.ProjectContext
	var corrections string
	if p.Structure != nil && p.Structure.Context != nil {
		prev = p.Structure.Context
		corrections = prev.Corrections
	}
	d := structure.Detect(p.Dir, p.Platform, prev)
	if d.Mode == core.StructureExisting {
		if d.Context != nil {
			d.Context.Corrections = corrections
		} else if corrections != "" {
			d.Context = &core.ProjectContext{Corrections: corrections}
		}
		p.Structure = d
	} else if p.Structure == nil {
		p.Structure = d
	}
	if p.Intent == "" {
		p.Intent = core.ClampIntent(existingFolder(p), "")
	}
}

func existingFolder(p *core.Project) bool {
	return p.Structure != nil && p.Structure.Mode == core.StructureExisting
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
	case work.KindFeatureMap:
		return work.FeatureMapPrompt(p)
	case work.KindDiscovery:
		return work.DiscoveryPrompt(p)
	case work.KindDesign:
		return work.DesignPrompt(p)
	case work.KindStack:
		return work.StackPrompt(p)
	case work.KindStructure:
		if p.Structure != nil && p.Structure.PlanRequest != nil {
			return work.StructurePlanPrompt(p)
		}
		return work.StructurePrompt(p)
	case work.KindRepo:
		if r := p.Structure.RepoRequest; r != nil && r.Action == core.RepoRevise && p.Structure.Repo != nil {
			return work.RepoRevisePrompt(p)
		}
		return work.RepoPlanPrompt(p)
	case work.KindTech:
		if r := p.Stack.PlanRequest; r != nil && r.Action == core.TechPlanRevise && p.Stack.Plan != nil {
			return work.TechRevisePrompt(p)
		}
		return work.TechPlanPrompt(p)
	case work.KindPlan:
		return work.UnifiedPlanPrompt(p)
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
	p, err := s.Store.Update(func(p *core.Project) error {
		if expect != nil {
			if err := expect(p); err != nil {
				return err
			}
		}
		msg := jobMessage(kind)
		if kind == work.KindPlan && planRepairing(p) {
			msg = "Memperbaiki output Blueprint"
		}
		if kind == work.KindWorkflow {
			msg = workflowRoundLabel(p, msg)
		}
		if isMCP {
			status, msg = core.JobWaiting, msg+" — menunggu AI Host (MCP)"
		}
		now := time.Now().UTC()
		p.Job = &core.Job{
			Kind: kind, Status: status, Message: planRoundLabel(p, kind, batchLabel(p, msg)), Started: now,
			ETAMS: core.EstimateJobMS(p.JobHistory, kind),
		}
		trackJobStart(p, kind, isMCP)
		return nil
	})
	if err != nil {
		return err
	}
	if isMCP {
		return nil
	}
	prompt := s.promptFor(p, kind)
	runner = s.techRunner(p, kind, runner)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if kind == work.KindWorkflow {
			s.runWorkflow(runner)
			return
		}
		out, err := s.runAgent(runner, kind, p.Dir, prompt)
		if err != nil {
			s.failJob(kind, err)
			return
		}
		saveAIRaw(p.Dir, kind, out)
		if err := s.applyAI(kind, out, "agent"); err != nil {
			if s.noteRejection(kind, err) {
				s.startAI(kind) // one retry with the rejection in the prompt
				return
			}
			if kind == work.KindStructure && s.structurePlanPending() {
				// The retry was refused too: close the analysis with the
				// concrete reason and let the user analyze again.
				s.dropStructureRequest(err)
				return
			}
			if kind == work.KindRepo && s.repoPending() {
				s.dropRepoRequest(err)
				return
			}
			if kind == work.KindTech && s.techPending() {
				s.dropTechRequest(err)
				return
			}
			if kind == work.KindPlan && s.planPending() {
				s.dropPlanRequest(err)
				return
			}
			s.failJob(kind, err)
		}
	}()
	return nil
}

// runAgent runs one prompt and records every attempt. An overloaded server
// answers the same prompt once it has room, so the job waits instead of
// failing. Any other error returns at once.
func (s *Service) runAgent(runner *agent.Runner, kind, dir, prompt string) (string, error) {
	return s.runAgentFor(runner, kind, runTag{}, dir, prompt)
}

// runTag says which part of a job a run is, for its progress and usage
// record: section is the job's section ("" the whole job), phase the step
// of a multi-round job, requestRetry the automatic retries of its request.
type runTag struct {
	section, phase string
	requestRetry   int
}

// runAgentFor is runAgent for a tagged part of the job.
func (s *Service) runAgentFor(runner *agent.Runner, kind string, tag runTag, dir, prompt string) (string, error) {
	section := tag.section
	static, body := work.SplitPrompt(work.WithPolicy(prompt))
	for attempt := 0; ; attempt++ {
		t0 := time.Now()
		res, err := runner.Exec(s.context(), dir, agent.Prompt{Static: static, Body: body})
		out := res.Text
		took := time.Since(t0)
		s.logAgentTiming(kind, took, len(static)+len(body), len(out), res.Usage)
		recordUsage(runner, dir, kind, tag, attempt, took, static, body, res, err)
		if !errors.Is(err, agent.ErrOverloaded) || attempt == len(s.RetryWait) {
			return out, err
		}
		wait := s.RetryWait[attempt]
		s.Store.Update(func(p *core.Project) error {
			p.Log("system", fmt.Sprintf("%s: server AI sibuk, coba lagi dalam %s (%d/%d)", kind, wait, attempt+1, len(s.RetryWait)))
			return nil
		})
		s.markBusyWait(kind, section, wait, attempt+1, len(s.RetryWait))
		select {
		case <-time.After(wait):
			s.markBusyRerun(kind, section)
		case <-s.context().Done():
			return out, err
		}
	}
}

func (s *Service) logAgentTiming(kind string, d time.Duration, promptBytes, outputBytes int, u *agent.Usage) {
	msg := fmt.Sprintf("%s agent %s prompt=%dB output=%dB", kind, d.Round(time.Millisecond), promptBytes, outputBytes)
	if u != nil && u.Total() != nil {
		msg += fmt.Sprintf(" tokens in=%d out=%d cache_write=%d cache_read=%d", *u.InputTokens, *u.OutputTokens, *u.CacheCreationTokens, *u.CacheReadTokens)
	}
	s.Store.Update(func(p *core.Project) error {
		p.Log("system", msg)
		return nil
	})
}

// recordUsage appends one run to the project's usage log. It never fails
// the job: a run whose usage cannot be stored is only missing from the log.
func recordUsage(runner *agent.Runner, dir, kind string, tag runTag, attempt int, took time.Duration, static, body string, res agent.Result, err error) {
	if dir == "" {
		return
	}
	size := len(static) + len(body)
	r := usage.Record{RunID: usage.NewRunID(), Project: usage.ProjectID(dir), Operation: kind, Agent: runner.ID,
		Time: time.Now().UTC(), WallMS: took.Milliseconds(), PromptBytes: size, StaticBytes: len(static),
		DynamicBytes: len(body), EstInputTokens: usage.EstimateTokens(size), Phase: tag.phase, RequestRetry: tag.requestRetry,
		OutputBytes: len(res.Text), Status: usage.StatusOK, Retry: attempt, CLICalls: res.Calls, Telemetry: usage.Unavailable}
	if v := runner.Version(context.Background()); v != "" {
		r.CLIVersion = &v
	}
	switch {
	case err == nil:
	case errors.Is(err, agent.ErrOverloaded):
		r.Status = usage.StatusOverloaded
	case errors.Is(err, agent.ErrLimit):
		r.Status = usage.StatusLimit
	default:
		r.Status = usage.StatusFailed
	}
	if u := res.Usage; u != nil {
		r.Model, r.InputTokens, r.OutputTokens = u.Model, u.InputTokens, u.OutputTokens
		r.CacheCreationTokens, r.CacheReadTokens, r.TotalReported = u.CacheCreationTokens, u.CacheReadTokens, u.Total()
		r.CacheWrite1h, r.CacheWrite5m = u.CacheWrite1h, u.CacheWrite5m
		r.NumTurns, r.DurationMS, r.ServiceTier = u.NumTurns, u.DurationMS, u.ServiceTier
		r.CostUSD = u.CostUSD
		if r.InputTokens != nil || r.OutputTokens != nil {
			r.Telemetry = usage.Reported
		}
	}
	_ = usage.Append(dir, r)
}

// runWorkflow runs a workflow job in rounds. A long answer can be cut at the
// model's output limit; ApplyWorkflow keeps the request when the answer
// revised only part of the stale commands, so the next round asks for the
// rest. Each round rebuilds the prompt from the saved project.
func (s *Service) runWorkflow(runner *agent.Runner) {
	kind := work.KindWorkflow
	p, err := s.Store.Load()
	if err != nil {
		s.failJob(kind, err)
		return
	}
	tag := runTag{phase: work.WorkflowPhase(p)}
	if p.Workflow != nil && p.Workflow.Request != nil {
		tag.requestRetry = p.Workflow.Request.Retries
	}
	out, err := s.runAgentFor(runner, kind, tag, p.Dir, s.promptFor(p, kind))
	if err != nil {
		s.failJob(kind, err)
		return
	}
	saveAIRaw(p.Dir, kind, out)
	if err := s.applyAI(kind, out, "agent"); err != nil {
		// The last refused answer stays for audit: the next round overwrites
		// the plain copy.
		saveAIRaw(p.Dir, kind+"-rejected", out)
		if s.noteRejection(kind, err) {
			s.startAI(kind) // retry with the rejection in the prompt
			return
		}
		s.failJob(kind, err)
		return
	}
	if s.context().Err() != nil {
		return
	}
	// The request survived: the workflow still needs another round. applyAI
	// already closed this job; the next one starts only if nothing else
	// (a continue, a cancel) acted in between.
	s.startNextWorkflowRound()
}

// startNextWorkflowRound starts the next round of a workflow job in the same
// update that checks the round is still open, so a round is never started
// twice nor after its request was cancelled.
func (s *Service) startNextWorkflowRound() error {
	err := s.startAIIf(work.KindWorkflow, workflowRoundOpen)
	switch {
	case errors.Is(err, ErrBusy), errors.Is(err, ErrNothingToDo):
		return nil // started by someone else, or nothing left
	case err != nil:
		s.failJob(work.KindWorkflow, err)
	}
	return err
}

func jobMessage(kind string) string {
	switch kind {
	case work.KindFeatureMap:
		return "Menyusun daftar fitur"
	case work.KindDiscovery:
		return "Merancang Blueprint"
	case work.KindDesign:
		return "Menyusun aturan desain & layout"
	case work.KindStack:
		return "Memahami permintaan perubahan teknologi"
	case work.KindStructure:
		return "Memproses struktur project"
	case work.KindRepo:
		return "Merancang struktur repository"
	case work.KindTech:
		return "Merancang teknologi"
	case work.KindPlan:
		return "Merancang Blueprint"
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
		trackFail(p, kind, err)
		return nil
	})
}

// applyAI lands an AI answer in the project and chains the next step.
func (s *Service) applyAI(kind, raw, source string) error {
	s.markValidating(kind, "")
	var next string
	_, err := s.Store.Update(func(p *core.Project) error {
		if p.Job == nil || p.Job.Kind != kind || p.Job.Status == core.JobDone {
			return fmt.Errorf("%w: tidak ada job %s yang aktif", ErrWrongStage, kind)
		}
		own := sectionForJob(p, kind)
		switch kind {
		case work.KindFeatureMap:
			if err := work.ApplyFeatureMap(p, raw); err != nil {
				return err
			}
		case work.KindDiscovery:
			if err := work.ApplyDiscovery(p, raw); err != nil {
				return err
			}
			defer stampRefs(p)
			var prev *core.ProjectContext
			if p.Structure != nil {
				prev = p.Structure.Context
			}
			p.Structure = structure.Detect(p.Dir, p.Platform, prev)
			readExistingUI(p)
			decideStack(p, "")
			p.Stage = core.StageDesign
		case work.KindDesign:
			if err := work.ApplyDesign(p, raw); err != nil {
				return err
			}
			if work.DesignNeedsDetails(p) {
				next = work.KindDesign
			}
		case work.KindStack:
			if err := work.ApplyStackChange(p, raw); err != nil {
				return err
			}
			p.Stack.Proposal.Affected = affected(p)
		case work.KindStructure:
			if p.Structure != nil && p.Structure.PlanRequest != nil {
				if err := work.ApplyStructurePlan(p, raw); err != nil {
					return err
				}
				break
			}
			if err := work.ApplyStructureChange(p, raw); err != nil {
				return err
			}
		case work.KindRepo:
			if p.Structure == nil || p.Structure.RepoRequest == nil {
				return errors.New("tidak ada permintaan struktur repository")
			}
			apply := work.ApplyRepoPlan
			if p.Structure.RepoRequest.Action == core.RepoRevise {
				apply = work.ApplyRepoRevision
			}
			if err := apply(p, raw); err != nil {
				return err
			}
			defer stampRefs(p)
		case work.KindTech:
			if p.Stack == nil || p.Stack.PlanRequest == nil {
				return errors.New("tidak ada permintaan rencana teknologi")
			}
			apply := work.ApplyTechPlan
			if p.Stack.PlanRequest.Action == core.TechPlanRevise {
				apply = work.ApplyTechRevision
			}
			v, err := apply(p, raw)
			if err != nil {
				return err
			}
			landTech(p, v)
			defer stampRefs(p)
		case work.KindPlan:
			if err := applyUnified(p, raw); err != nil {
				return err
			}
			defer stampRefs(p)
			if planRequestsOpen(p) {
				next = work.KindPlan
			}
		case work.KindArtifacts:
			area := ""
			if p.ArtifactRequest != nil {
				area = p.ArtifactRequest.Area
			}
			if err := work.ApplyArtifacts(p, raw); err != nil {
				return err
			}
			stampRefs(p)
			if err := checkAreaAnswered(p, area); err != nil {
				return err
			}
			if w := p.Workflow; w != nil && w.Request != nil && w.Request.Cascade {
				next = work.KindWorkflow // the cleanup of a deleted document continues with its commands
			}
		case work.KindWorkflow:
			if err := applyWorkflowDelta(p, raw); err != nil {
				return err
			}
			// Commands are written and revised one per round. The agent loop
			// starts the next round itself (runWorkflow); an MCP host gets it
			// queued.
			if w := p.Workflow; source == "mcp" && w != nil && w.Request != nil {
				next = work.KindWorkflow
			}
		default:
			return fmt.Errorf("jenis pekerjaan %q tidak dikenal", kind)
		}
		doneAt := time.Now().UTC()
		started := doneAt
		if p.Job != nil && !p.Job.Started.IsZero() {
			started = p.Job.Started
		}
		core.RecordJobTiming(p, kind, started, doneAt)
		doneMsg := jobMessage(kind)
		if p.BlueprintBatch > 0 {
			doneMsg = "Merancang Blueprint"
		}
		p.Job = &core.Job{Kind: kind, Status: core.JobDone, Message: doneMsg + " selesai", Started: started, Steps: finishJobSteps(p.Job, doneAt)}
		p.Log(source, doneMsg+" selesai")
		clearAIRaw(p.Dir, kind)
		syncCompleted(p, own)
		return nil
	})
	if kind == work.KindStack && err != nil {
		return s.dropStackRequest(err)
	}
	if kind == work.KindStructure && err != nil {
		// A refused analysis keeps its request so the AI gets the concrete
		// refusal in the prompt and can fix it; a refused structure change is
		// dropped like before.
		if s.structurePlanPending() {
			return err
		}
		return s.dropStructureRequest(err)
	}
	if kind == work.KindRepo && err != nil {
		// Like a structure analysis, a refused answer keeps its request so
		// the retry prompt carries the concrete refusal.
		if s.repoPending() {
			return err
		}
		return s.dropRepoRequest(err)
	}
	if kind == work.KindTech && err != nil {
		// A refused answer keeps its request so the retry prompt carries the
		// concrete refusal.
		if s.techPending() {
			return err
		}
		return s.dropTechRequest(err)
	}
	if kind == work.KindPlan && err != nil {
		if s.planPending() {
			return err
		}
		return s.dropPlanRequest(err)
	}
	if err != nil {
		return err
	}
	if next == work.KindWorkflow {
		return s.startNextWorkflowRound()
	}
	if next != "" {
		return s.startAI(next)
	}
	if kind == work.KindDiscovery {
		return s.continueInitialBlueprint()
	}
	return s.advanceBlueprint(kind)
}

// noteRejection keeps why a design, workflow, blueprint or structure
// analysis answer was refused on its request, for the retry prompt. It
// reports whether the job retries automatically.
func (s *Service) noteRejection(kind string, err error) bool {
	if err == nil {
		return false
	}
	if kind == work.KindStructure {
		retry := false
		s.Store.Update(func(p *core.Project) error {
			if p.Structure == nil || p.Structure.PlanRequest == nil {
				return nil
			}
			retry = p.Structure.PlanRequest.Rejected == ""
			p.Structure.PlanRequest.Rejected = err.Error()
			p.Log("system", err.Error())
			trackRejection(p, kind, err)
			return nil
		})
		return retry
	}
	if kind == work.KindRepo {
		retry := false
		s.Store.Update(func(p *core.Project) error {
			if p.Structure == nil || p.Structure.RepoRequest == nil {
				return nil
			}
			retry = p.Structure.RepoRequest.Rejected == ""
			p.Structure.RepoRequest.Rejected = err.Error()
			p.Log("system", err.Error())
			trackRejection(p, kind, err)
			return nil
		})
		return retry
	}
	if kind == work.KindTech || kind == work.KindPlan {
		escFrom, escTo := s.escalation()
		retry := false
		s.Store.Update(func(p *core.Project) error {
			if techRequestOf(p, kind) == nil {
				if kind == work.KindPlan {
					retry = stampPlanRejection(p, err.Error())
					p.Log("system", err.Error())
					trackRejection(p, kind, err)
				}
				return nil
			}
			retry = recordTechAttempt(p, kind, err, escFrom, escTo, s.maxTechAttempts())
			p.Log("system", err.Error())
			trackRejectionMsg(p, kind, err, techDiagnosis(p, kind, err, false))
			return nil
		})
		return retry
	}
	if kind == work.KindFeatureMap {
		retry := false
		s.Store.Update(func(p *core.Project) error {
			if p.FeatureMap == nil {
				p.FeatureMap = &core.FeatureMap{}
			}
			if p.FeatureMap.Request == nil {
				p.FeatureMap.Request = &core.FeatureMapRequest{Action: core.FeatureMapGenerate}
			}
			retry = p.FeatureMap.Request.Rejected == ""
			p.FeatureMap.Request.Rejected = err.Error()
			p.Log("system", err.Error())
			return nil
		})
		return retry
	}
	if kind != work.KindWorkflow && kind != work.KindArtifacts && kind != work.KindDesign {
		s.Store.Update(func(p *core.Project) error { // no request to retry on, but the refusal is real
			trackRejection(p, kind, err)
			return nil
		})
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
				p.Workflow = &core.Workflow{ID: "WF-001", Status: core.CommandDraft}
			}
			r = &p.Workflow.Request
		}
		if *r == nil {
			*r = &core.ChangeRequest{}
		}
		retry = autoRetry(*r, err.Error())
		(*r).Rejected = err.Error()
		p.Log("system", err.Error())
		trackRejection(p, kind, err)
		return nil
	})
	return retry
}

// maxAutoRetries bounds the automatic retries of one request.
const maxAutoRetries = 3

// autoRetry reports whether a refusal retries on its own, and counts it. The
// first refusal retries; a later one retries only when it names a different
// problem than the last, so a fixed problem followed by a new one (coverage
// passes, a subfeature rule fails) is not left to the user, while the same
// problem twice stops.
func autoRetry(r *core.ChangeRequest, msg string) bool {
	if r.Retries >= maxAutoRetries || (r.Rejected != "" && r.Rejected == msg) {
		return false
	}
	r.Retries++
	return true
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
		case work.KindFeatureMap:
			if p.FeatureMap != nil {
				p.FeatureMap.Request = nil
			}
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
			// A revision with rounds already saved stays open: the user
			// continues or cancels it, it is not half applied silently.
			if w := p.Workflow; w.Revising() {
				w.Request.Rejected = ""
			} else if w != nil {
				w.Request = nil
			}
		case work.KindStructure:
			if p.Structure != nil {
				p.Structure.PlanRequest = nil
				p.Structure.Request = ""
			}
		case work.KindRepo:
			if p.Structure != nil {
				p.Structure.RepoRequest = nil
			}
		case work.KindTech:
			if p.Stack != nil {
				p.Stack.PlanRequest = nil
			}
		case work.KindPlan:
			clearPlanRequests(p)
		}
		p.BlueprintQueue = nil
		p.BlueprintBatch = 0
		p.Job = nil
		stopRun(p)
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
	case work.KindFeatureMap, work.KindDiscovery, work.KindDesign:
		return s.startAIIf(kind, failedJob(kind))
	case work.KindStack:
		if p.Stack != nil && p.Stack.Request != "" {
			return s.startAIIf(kind, failedJob(kind))
		}
	case work.KindStructure:
		if p.Structure != nil && (p.Structure.Request != "" || p.Structure.PlanRequest != nil) {
			return s.startAIIf(kind, failedJob(kind))
		}
	case work.KindRepo:
		if p.Structure != nil && p.Structure.RepoRequest != nil {
			return s.startAIIf(kind, failedJob(kind))
		}
		if p.Structure != nil && p.Structure.IR != nil && p.Structure.Mode != core.StructureExisting {
			return s.GenerateRepoPlan("", "")
		}
	case work.KindTech:
		if p.Stack != nil && p.Stack.PlanRequest != nil {
			return s.startAIIf(kind, retryTech(kind))
		}
	case work.KindPlan:
		if s.planPending() {
			return s.startAIIf(kind, retryTech(kind))
		}
	case work.KindArtifacts:
		if p.ArtifactRequest != nil {
			return s.startAIIf(kind, failedJob(kind))
		}
	case work.KindWorkflow:
		return s.startAIIf(kind, failedJob(kind))
	}
	if p.BlueprintBatch > 0 {
		return s.GenerateBlueprint()
	}
	return ErrNothingToDo
}

// retryTech is failedJob for a technology job: a retry the user asked for
// starts a new automatic repair chain on the draft kept so far.
func retryTech(kind string) func(*core.Project) error {
	check := failedJob(kind)
	return func(p *core.Project) error {
		if err := check(p); err != nil {
			return err
		}
		if techRequestOf(p, kind) != nil {
			newTechChain(p)
		}
		return nil
	}
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
		// "Coba lagi" gives the request a fresh retry budget; its last
		// refusal stays, so the same problem again still stops.
		if r := changeRequestOf(p, kind); r != nil {
			r.Retries = 0
		}
		return nil
	}
}

// changeRequestOf is the change request a design, artifacts or workflow job
// answers; nil for any other job.
func changeRequestOf(p *core.Project, kind string) *core.ChangeRequest {
	switch {
	case kind == work.KindWorkflow && p.Workflow != nil:
		return p.Workflow.Request
	case kind == work.KindArtifacts:
		return p.ArtifactRequest
	case kind == work.KindDesign && p.UI != nil:
		return p.UI.Request
	}
	return nil
}
