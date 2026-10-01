package pipeline

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/handrawanw/foundry/internal/core"
	"github.com/handrawanw/foundry/internal/modules/agent"
	"github.com/handrawanw/foundry/internal/modules/checks"
	"github.com/handrawanw/foundry/internal/modules/scan"
	"github.com/handrawanw/foundry/internal/modules/verify"
	"github.com/handrawanw/foundry/internal/modules/work"
)

// ---------- Build (Kanban + agent) ----------

// StartBuild runs ready tasks until none is left, then verifies.
func (s *Service) StartBuild() error {
	runner, isMCP, err := s.AI.Runner()
	if err != nil {
		return err
	}
	status, msg := core.JobRunning, jobMessage("build")
	if isMCP {
		status, msg = core.JobWaiting, msg+" — AI Host mengambil task lewat MCP"
	}
	_, err = s.Store.Update(func(p *core.Project) error {
		if p.Busy() {
			return ErrBusy
		}
		if len(p.Tasks) == 0 {
			return ErrWrongStage
		}
		p.Stage = core.StageBuild
		p.Job = &core.Job{Kind: "build", Status: status, Message: msg, Started: time.Now().UTC()}
		p.Log("ui", "Build dimulai")
		return nil
	})
	if err != nil || isMCP {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.cancel = cancel
	s.mu.Unlock()
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer cancel()
		s.buildLoop(ctx, cancel, runner, s.agents())
	}()
	return nil
}

func (s *Service) StopBuild() error {
	s.mu.Lock()
	c := s.cancel
	s.mu.Unlock()
	if c == nil {
		return ErrNothingToDo
	}
	c()
	return nil
}

// buildLoop runs up to workers agents at once. Each claim takes the first
// ready task whose pages and files no running task holds; a task that
// overlaps one waits in the queue until that task finishes.
func (s *Service) buildLoop(ctx context.Context, cancel context.CancelFunc, runner *agent.Runner, workers int) {
	type result struct {
		worker string
		err    error
	}
	done := make(chan result)
	var free []string
	for i := workers; i >= 1; i-- {
		free = append(free, fmt.Sprintf("agent-%d", i))
	}
	queued := map[string]string{} // task → blocker already logged
	running, agentFailures := 0, 0
	var fatal error
	for {
		for fatal == nil && ctx.Err() == nil && len(free) > 0 {
			worker := free[len(free)-1]
			p, t, err := s.claimNext(worker, "agent", queued)
			if err != nil {
				fatal = err
				cancel()
				break
			}
			if t == nil {
				break
			}
			free = free[:len(free)-1]
			running++
			go func() { done <- result{worker, s.runTaskWithAgent(ctx, runner, p, t)} }()
		}
		if running == 0 {
			switch {
			case fatal != nil:
				s.failJob("build", fatal)
			case ctx.Err() != nil:
				s.Store.Update(func(p *core.Project) error {
					p.Job = &core.Job{Kind: "build", Status: core.JobDone, Message: "Build dihentikan", Started: time.Now().UTC()}
					return nil
				})
			default:
				s.finishBuild(ctx)
			}
			return
		}
		r := <-done
		running--
		free = append(free, r.worker)
		switch {
		case r.err == nil:
			agentFailures = 0
		case ctx.Err() != nil:
			// stopped by the user or by a fatal error; handled once all return
		case errors.Is(r.err, agent.ErrAgentFailed):
			if agentFailures++; agentFailures >= 2 && fatal == nil {
				fatal = fmt.Errorf("agent gagal berulang kali, periksa koneksi AI: %w", r.err)
				cancel()
			}
		case fatal == nil:
			fatal = r.err
			cancel()
		}
	}
}

// claimNext atomically moves the first claimable task to IN_PROGRESS for
// owner. It returns a nil task when every ready task is locked by a running
// one (or none is ready). Waits are logged once per blocker when queued is
// non-nil. The check runs inside the store lock, so agents in other
// processes (MCP hosts) can never claim overlapping tasks either.
func (s *Service) claimNext(owner, source string, queued map[string]string) (*core.Project, *core.Task, error) {
	var id string
	p, err := s.Store.Update(func(p *core.Project) error {
		for _, t := range p.ReadyTasks() {
			o, lock := p.Blocker(t)
			if o == nil {
				id = t.ID
				claim(p, t, owner, source)
				return nil
			}
			if queued != nil && queued[t.ID] != o.ID {
				queued[t.ID] = o.ID
				p.Log("system", fmt.Sprintf("%s antri: %s sedang dipakai %s", t.ID, lockName(lock), o.ID))
			}
		}
		return nil
	})
	if err != nil || id == "" {
		return p, nil, err
	}
	return p, p.Task(id), nil
}

func claim(p *core.Project, t *core.Task, owner, source string) {
	t.Status = core.TaskInProgress
	t.Attempts++
	t.Owner = owner
	if p.Job != nil {
		p.Job.TaskID = t.ID
		p.Job.Message = jobMessage("build") + ": " + t.Title
		if n := len(p.ActiveTasks()); n > 1 {
			p.Job.Message = fmt.Sprintf("%s: %d task paralel", jobMessage("build"), n)
		}
	}
	p.Log(source, fmt.Sprintf("%s diambil %s (percobaan %d)", t.ID, owner, t.Attempts))
}

func lockName(lock string) string {
	if lock == core.LockAll {
		return "seluruh project"
	}
	return strings.Replace(lock, ":", " ", 1)
}

func (s *Service) runTaskWithAgent(ctx context.Context, runner *agent.Runner, p *core.Project, t *core.Task) error {
	id := t.ID
	// Changes by agents running in parallel land in the same tree, so the
	// diff below may include their files too; the reviewer judges criteria.
	before := checks.Take(p.Dir)
	if _, err := runner.Run(ctx, agent.Execute, p.Dir, work.TaskPrompt(p, t)); err != nil {
		if ctx.Err() != nil {
			return s.releaseTask(ctx, id)
		}
		s.rejectTask(id, "Agent gagal menjalankan task: "+err.Error())
		return err
	}
	changed := checks.Changed(p.Dir, before)
	if len(changed) == 0 {
		s.rejectTask(id, "Tidak ada file yang berubah; implementasi belum dibuat")
		return nil
	}
	build := s.runChecks(ctx, id)
	if ctx.Err() != nil {
		return s.releaseTask(ctx, id)
	}
	if !build.Passed {
		s.rejectTask(id, "Verifikasi gagal: "+build.Reason+"\n"+lastOutput(build))
		return nil
	}
	s.setStatus(id, core.TaskReview, "agent", "Build + test lulus, review acceptance criteria")
	p, _ = s.Store.Load()
	out, err := runner.Run(ctx, agent.Think, p.Dir, work.ReviewPrompt(p, p.Task(id), checks.Excerpt(p.Dir, changed, 60000), &build))
	if err != nil {
		if ctx.Err() != nil {
			return s.releaseTask(ctx, id)
		}
		s.rejectTask(id, "Review gagal: "+err.Error())
		return err
	}
	crit, err := work.ParseReview(p.Task(id), out)
	if err != nil {
		s.rejectTask(id, "Review tidak dapat dibaca: "+err.Error())
		return nil
	}
	return s.concludeTask(id, crit, build.ID, "agent")
}

// runChecks re-detects commands (a scaffold task may have created tooling)
// and runs them, storing the result.
func (s *Service) runChecks(ctx context.Context, taskID string) core.BuildRun {
	p, _ := s.Store.Load()
	cmds := scan.Scan(p.Dir).Commands
	s.checkMu.Lock()
	build := checks.Run(ctx, p.Dir, cmds, taskID)
	s.checkMu.Unlock()
	s.Store.Update(func(p *core.Project) error {
		if p.Structure != nil {
			p.Structure.Commands = cmds
		}
		p.Builds = append(p.Builds, build)
		if n := len(p.Builds); n > 50 {
			p.Builds = p.Builds[n-50:]
		}
		if t := p.Task(taskID); t != nil {
			t.LastBuildID = build.ID
		}
		return nil
	})
	return build
}

func lastOutput(b core.BuildRun) string {
	if len(b.Steps) == 0 {
		return ""
	}
	out := b.Steps[len(b.Steps)-1].Output
	if len(out) > 1500 {
		out = "…" + out[len(out)-1500:]
	}
	return out
}

// releaseTask returns a task interrupted by "Hentikan" to TODO. The attempt
// is not counted and no failure note is added: the agent was stopped, it did
// not fail.
func (s *Service) releaseTask(ctx context.Context, id string) error {
	s.Store.Update(func(p *core.Project) error {
		if t := p.Task(id); t != nil {
			t.Status = core.TaskTodo
			if t.Attempts > 0 {
				t.Attempts--
			}
			p.Log("system", id+" dihentikan, kembali ke TODO")
		}
		return nil
	})
	return ctx.Err()
}

// rejectTask sends a task back to TODO with feedback, or BLOCKED after
// MaxAttempts.
func (s *Service) rejectTask(id, note string) {
	s.Store.Update(func(p *core.Project) error {
		t := p.Task(id)
		if t == nil {
			return nil
		}
		t.Notes = append(t.Notes, note)
		if len(t.Notes) > 5 {
			t.Notes = t.Notes[len(t.Notes)-5:]
		}
		if t.Attempts >= MaxAttempts {
			t.Status = core.TaskBlocked
			p.Log("system", fmt.Sprintf("%s BLOCKED setelah %d percobaan", id, t.Attempts))
		} else {
			t.Status = core.TaskTodo
			p.Log("system", fmt.Sprintf("%s kembali ke TODO: %s", id, firstLine(note)))
		}
		return nil
	})
}

// concludeTask applies the DONE gate: passing build + all criteria met.
func (s *Service) concludeTask(id string, crit []core.Criterion, buildID, source string) error {
	var failedNote string
	_, err := s.Store.Update(func(p *core.Project) error {
		t := p.Task(id)
		if t == nil {
			return fmt.Errorf("task %s tidak ada", id)
		}
		t.Criteria = crit
		b := p.Build(buildID)
		if b == nil || !b.Passed {
			return ErrGate
		}
		if !work.AllMet(crit) {
			var miss []string
			for _, c := range crit {
				if !c.Met {
					miss = append(miss, c.Text)
				}
			}
			failedNote = "Acceptance criteria belum terpenuhi: " + strings.Join(miss, "; ")
			return nil
		}
		t.Status = core.TaskDone
		t.Notes = nil
		p.Log(source, id+" DONE")
		return nil
	})
	if err == nil && failedNote != "" {
		s.rejectTask(id, failedNote)
	}
	return err
}

func (s *Service) setStatus(id string, st core.TaskStatus, source, msg string) {
	s.Store.Update(func(p *core.Project) error {
		if t := p.Task(id); t != nil {
			t.Status = st
			p.Log(source, fmt.Sprintf("%s → %s: %s", id, st, msg))
		}
		return nil
	})
}

// MoveTask is a manual Kanban move by the user. DONE stays behind the gate.
func (s *Service) MoveTask(id string, to core.TaskStatus) (*core.Project, error) {
	return s.Store.Update(func(p *core.Project) error {
		t := p.Task(id)
		if t == nil {
			return fmt.Errorf("task %s tidak ada", id)
		}
		switch to {
		case core.TaskTodo:
			if t.Status == core.TaskInProgress && p.Busy() {
				return ErrBusy
			}
			t.Attempts = 0
		case core.TaskBlocked:
			if t.Status == core.TaskInProgress && p.Busy() {
				return ErrBusy
			}
		case core.TaskDone:
			b := p.Build(t.LastBuildID)
			if t.Status != core.TaskReview || b == nil || !b.Passed {
				return ErrGate
			}
		default:
			return fmt.Errorf("status %s tidak bisa dipindah manual", to)
		}
		t.Status = to
		p.Log("ui", fmt.Sprintf("%s dipindah ke %s", id, to))
		return nil
	})
}

// finishBuild runs the full verification once the queue is empty.
func (s *Service) finishBuild(ctx context.Context) {
	s.Store.Update(func(p *core.Project) error {
		p.Stage = core.StageVerify
		p.Job = &core.Job{Kind: "verify", Status: core.JobRunning, Message: jobMessage("verify"), Started: time.Now().UTC()}
		return nil
	})
	s.verifyNow(ctx)
}

// ---------- Verify / Fix ----------

func (s *Service) Verify() error {
	_, err := s.Store.Update(func(p *core.Project) error {
		if p.Busy() {
			return ErrBusy
		}
		if len(p.Tasks) == 0 {
			return ErrWrongStage
		}
		p.Stage = core.StageVerify
		p.Job = &core.Job{Kind: "verify", Status: core.JobRunning, Message: jobMessage("verify"), Started: time.Now().UTC()}
		return nil
	})
	if err != nil {
		return err
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.verifyNow(context.Background())
	}()
	return nil
}

func (s *Service) verifyNow(ctx context.Context) {
	build := s.runChecks(ctx, "")
	s.Store.Update(func(p *core.Project) error {
		v := verify.Report(p, p.Build(build.ID))
		p.Verification = v
		if v.Ready {
			p.Stage = core.StageReady
			p.Log("system", "Project siap diperiksa")
		} else {
			p.Stage = core.StageVerify
			p.Log("system", fmt.Sprintf("Verifikasi: %d masalah", len(v.Issues)))
		}
		p.Job = &core.Job{Kind: "verify", Status: core.JobDone, Message: "Verifikasi selesai", Started: time.Now().UTC()}
		return nil
	})
}

// Fix turns verification issues into work and restarts the build.
func (s *Service) Fix() error {
	_, err := s.Store.Update(func(p *core.Project) error {
		if p.Busy() {
			return ErrBusy
		}
		if p.Verification == nil {
			return ErrWrongStage
		}
		for i := range p.Tasks {
			if p.Tasks[i].Status == core.TaskBlocked || p.Tasks[i].Status == core.TaskReview {
				p.Tasks[i].Status, p.Tasks[i].Attempts = core.TaskTodo, 0
			}
		}
		added := work.FillCoverage(p)
		for _, is := range p.Verification.Issues {
			tooling := is.Kind == "missing_test" && is.Ref == ""
			if is.Kind != "build_failed" && is.Kind != "test_failed" && is.Kind != "no_commands" && !tooling {
				continue
			}
			var out string
			if b := p.Build(is.Ref); b != nil {
				out = lastOutput(*b)
			}
			p.Tasks = append(p.Tasks, core.Task{ID: work.NextTaskID(p), Title: "Perbaiki: " + is.Message, Area: "integration",
				Description:        "Verifikasi project gagal. Perbaiki penyebabnya tanpa mengubah perilaku yang sudah benar.\n\n" + out,
				AcceptanceCriteria: []string{"Semua command build/test project lulus", "Tidak ada test yang dihapus atau dilewati untuk membuatnya lulus"},
				TestRequirements:   []string{"Command test project lulus"}, Status: core.TaskTodo})
			added++
		}
		p.Log("ui", fmt.Sprintf("Perbaiki otomatis: %d task baru", added))
		return nil
	})
	if err != nil {
		return err
	}
	return s.StartBuild()
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
