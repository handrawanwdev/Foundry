// Package pipeline is the Foundry orchestrator: it moves one project through
// Tell → Discovery → Design → Plan → Build → Verify → Ready. AI work is done
// either by a connected agent CLI (Foundry drives it) or by an MCP host (the
// host pulls work with next_work and pushes answers with submit_work). Both
// paths go through the same apply functions and the same store.
package pipeline

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/handrawanw/foundry/internal/config"
	"github.com/handrawanw/foundry/internal/core"
	"github.com/handrawanw/foundry/internal/modules/agent"
	"github.com/handrawanw/foundry/internal/modules/stack"
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

// Parallel is optional on AI: how many coding agents may build at once.
type Parallel interface {
	Agents() int
}

// DefaultAgents build in parallel unless FOUNDRY_AGENTS or the connection
// says otherwise; MaxAgents caps it.
const (
	DefaultAgents = 3
	MaxAgents     = 8
)

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

func (ConfigAI) Agents() int {
	c, err := config.Load()
	if err != nil || c.Connection == nil {
		return 0
	}
	return c.Connection.Agents
}

type Service struct {
	Store *core.Store
	AI    AI
	// Render takes browser screenshots for the visual check of a new
	// design and of a revision; nil skips the check.
	Render Renderer

	mu     sync.Mutex
	cancel context.CancelFunc
	wg     sync.WaitGroup
	// checkMu runs one build/test at a time; parallel runs of the same
	// toolchain would fight over its outputs.
	checkMu sync.Mutex
}

// agents is how many coding agents build in parallel.
func (s *Service) agents() int {
	n := 0
	if v, err := strconv.Atoi(os.Getenv("FOUNDRY_AGENTS")); err == nil {
		n = v
	} else if pa, ok := s.AI.(Parallel); ok {
		n = pa.Agents()
	}
	if n <= 0 {
		n = DefaultAgents
	}
	return min(n, MaxAgents)
}

func New(store *core.Store, ai AI) *Service {
	return &Service{Store: store, AI: ai}
}

// RecoverStale marks jobs left "running" by a dead process as failed. Only
// the process that runs agent jobs (the HTTP server) should call this.
func (s *Service) RecoverStale() {
	s.Store.Update(func(p *core.Project) error {
		recoverSupport(p)
		if p.Job != nil && p.Job.Status == core.JobRunning {
			p.Job.Status, p.Job.Error = core.JobFailed, "Proses terhenti sebelum selesai. Jalankan ulang."
			for i := range p.Tasks {
				if p.Tasks[i].Active() { // frees its page/file locks
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
			case core.PlatformWeb, core.PlatformAndroid, core.PlatformIOS, core.PlatformDesktop, core.PlatformBackend:
				ok = append(ok, t)
			}
		}
		if len(ok) == 0 {
			return errors.New("pilih minimal satu platform")
		}
		p.Platform.Targets = ok
		if p.Stack == nil || !p.Stack.Approved {
			decideStack(p)
		}
		p.Log("ui", "Platform diubah")
		return nil
	})
}

// SetTemplate picks the project structure template; for a new project it
// is recorded as the architecture and repository decision it stands for,
// so the stack and the structure never disagree.
func (s *Service) SetTemplate(id string) (*core.Project, error) {
	return s.Store.Update(func(p *core.Project) error {
		if p.Structure == nil || p.Busy() || p.Structure.Approved {
			return ErrWrongStage
		}
		if err := structure.Apply(p.Structure, id); err != nil {
			return err
		}
		if p.Stack == nil {
			decideStack(p)
		}
		arch, repo := core.ArchModular, core.RepoSimple
		switch id {
		case "simple":
			arch = core.ArchSimple
		case "monorepo":
			repo = core.RepoMonorepo
		case "microservices":
			arch = core.ArchMicroservices
		}
		const why = "Struktur dipilih oleh user."
		if err := stack.Override(p.Stack, core.TechArchitecture, arch, why); err != nil {
			return err
		}
		if err := stack.Override(p.Stack, core.TechRepository, repo, why); err != nil {
			return err
		}
		syncStructure(p)
		p.Log("ui", "Struktur project: "+id)
		return nil
	})
}

// SetStack overrides the recommended technology for one area (value ""
// returns to the recommendation). The reason is kept with the decision.
func (s *Service) SetStack(area, value, reason string) (*core.Project, error) {
	return s.Store.Update(func(p *core.Project) error {
		if p.Busy() {
			return ErrBusy
		}
		if p.Platform == nil || p.Structure == nil || (p.Stack != nil && p.Stack.Approved) {
			return ErrWrongStage
		}
		if p.Stack == nil {
			decideStack(p)
		}
		if err := stack.Override(p.Stack, area, value, reason); err != nil {
			return err
		}
		syncStructure(p)
		p.Log("ui", fmt.Sprintf("Teknologi %s: %s", area, p.Stack.Choice(area)))
		return nil
	})
}

// RecommendStack analyzes the requirement again (projects made before the
// technology decision get theirs here); the user's overrides stay.
func (s *Service) RecommendStack() (*core.Project, error) {
	return s.Store.Update(func(p *core.Project) error {
		if p.Busy() {
			return ErrBusy
		}
		if p.Platform == nil || p.Structure == nil || (p.Stack != nil && p.Stack.Approved) {
			return ErrWrongStage
		}
		decideStack(p)
		p.Log("ui", "Rekomendasi teknologi diperbarui")
		return nil
	})
}

// decideStack recommends the stack from the requirement and platform, keeps
// the user's overrides and derives a new project's structure from it.
func decideStack(p *core.Project) {
	next := stack.Recommend(p)
	stack.Carry(p.Stack, next)
	p.Stack = next
	syncStructure(p)
}

func syncStructure(p *core.Project) {
	if st := p.Structure; st != nil && st.Mode == core.StructureNew && !st.Approved && p.Stack != nil {
		st.Template, st.Layout, st.Reason = stack.Structure(p.Stack, p.Features)
	}
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

// ApproveDesign locks platform, structure and design, then generates tasks.
// The approval is saved first: a later scaffold or plan error does not
// undo it. RetryJob re-runs those steps.
func (s *Service) ApproveDesign() error {
	_, err := s.Store.Update(func(p *core.Project) error {
		if p.Busy() {
			return ErrBusy
		}
		if p.Design == nil || len(p.Design.Pages) == 0 || p.Platform == nil || p.Structure == nil {
			return ErrWrongStage
		}
		if p.Design.Status == core.DesignApproved {
			return ErrWrongStage
		}
		if b := work.Blockers(p); len(b) > 0 {
			return fmt.Errorf("%w: %d hal wajib dilengkapi", ErrIncomplete, len(b))
		}
		p.Design.Status = core.DesignApproved
		p.Design.Revision = nil // approving keeps any pending change
		if p.Stack == nil {
			decideStack(p)
		}
		if p.Stack == nil {
			return errors.New("keputusan teknologi belum ada")
		}
		p.Platform.Approved = true
		p.Structure.Approved = true
		p.Stack.Approved = true
		p.Stage = core.StagePlan
		p.Log("ui", "Rancangan disetujui")
		return nil
	})
	if err != nil {
		return err
	}
	return s.continueApproval()
}

// continueApproval scaffolds the project (if needed) and starts the plan
// job. Called after a fresh approval and when retrying a failed plan.
func (s *Service) continueApproval() error {
	if err := s.scaffoldApproved(); err != nil {
		s.failJob(work.KindPlan, err)
		return err
	}
	return s.startAI(work.KindPlan)
}

func (s *Service) scaffoldApproved() error {
	_, err := s.Store.Update(func(p *core.Project) error {
		if p.Design == nil || p.Design.Status != core.DesignApproved {
			return ErrWrongStage
		}
		if p.Structure == nil || p.Structure.Scaffolded {
			return nil
		}
		if err := structure.Scaffold(p.Dir, p.Structure); err != nil {
			return fmt.Errorf("gagal menyiapkan struktur: %w", err)
		}
		return nil
	})
	return err
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
	case work.KindReference:
		return work.ReferencePrompt(p)
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
	case work.KindReference:
		return "Menganalisis referensi desain"
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
			decideStack(p)
			p.Stage = core.StageDesign
			next = work.KindDesign
			if r := p.Reference; r != nil && r.Status == "pending" {
				next = work.KindReference // the design follows the reference's theme
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
	switch p.Job.Kind {
	case work.KindDiscovery, work.KindDesign, work.KindDesignQA, work.KindReference:
		return s.startAI(p.Job.Kind)
	case work.KindPlan:
		return s.continueApproval()
	case "build":
		return s.StartBuild()
	case "verify":
		return s.Verify()
	}
	return ErrNothingToDo
}
