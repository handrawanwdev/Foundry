package pipeline

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/handrawanw/foundry/internal/core"
	"github.com/handrawanw/foundry/internal/modules/agent"
	"github.com/handrawanw/foundry/internal/modules/checks"
	"github.com/handrawanw/foundry/internal/modules/gitwt"
	"github.com/handrawanw/foundry/internal/modules/scan"
	"github.com/handrawanw/foundry/internal/modules/verify"
	"github.com/handrawanw/foundry/internal/modules/work"
)

// ---------- Build (Kanban + agent) ----------

// GeneratePRD reconstructs the product PRD from current requirements,
// capabilities, flows, contracts and tasks. The kanban must already exist.
func (s *Service) GeneratePRD() error {
	p, err := s.Store.Load()
	if err != nil {
		return err
	}
	if p.Busy() || (p.Job != nil && p.Job.Status == core.JobWaiting) {
		return ErrBusy
	}
	if len(p.Tasks) == 0 {
		return ErrWrongStage
	}
	return s.startAI(work.KindPRD)
}

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
		logIndependent(p)
		return nil
	})
	if err != nil || isMCP {
		return err
	}
	if p, err := s.Store.Load(); err == nil && gitwt.Usable(p.Dir) {
		gitwt.Prune(p.Dir) // stale worktrees of a crashed run
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.cancel = cancel
	s.mu.Unlock()
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer cancel()
		s.buildLoop(ctx, cancel, runner)
	}()
	return nil
}

func (s *Service) StopBuild() error {
	s.mu.Lock()
	c := s.cancel
	s.mu.Unlock()
	if c != nil {
		c()
		return nil
	}
	return s.stopHostBuild()
}

// stopHostBuild ends a build that waits for an MCP host: no goroutine runs it,
// so the job is closed directly and tasks the host holds go back to TODO.
func (s *Service) stopHostBuild() error {
	var released []string
	_, err := s.Store.Update(func(p *core.Project) error {
		if p.Job == nil || p.Job.Kind != "build" || p.Job.Status != core.JobWaiting {
			return ErrNothingToDo
		}
		for i := range p.Tasks {
			t := &p.Tasks[i]
			if t.Status != core.TaskInProgress {
				continue
			}
			t.Status, t.Owner, t.Progress = core.TaskTodo, "", ""
			if t.Attempts > 0 {
				t.Attempts--
			}
			released = append(released, t.ID)
			p.Log("system", t.ID+" dihentikan, kembali ke TODO")
		}
		p.Job = &core.Job{Kind: "build", Status: core.JobDone, Message: "Build dihentikan", Started: time.Now().UTC()}
		return nil
	})
	for _, id := range released {
		markResume(s.Store.Dir, id)
	}
	return err
}

// buildLoop runs up to s.agents() coding agents at once and re-reads that
// cap every spawn, so a settings change applies to the next free slot.
// Each claim takes the first ready task whose pages and files no running
// task holds; a task that overlaps one waits in the queue until that task
// finishes.
func (s *Service) buildLoop(ctx context.Context, cancel context.CancelFunc, runner *agent.Runner) {
	type result struct {
		worker string
		err    error
	}
	done := make(chan result)
	queued := map[string]string{} // task → blocker already logged
	nextID, running, agentFailures := 0, 0, 0
	var fatal error
	for {
		for fatal == nil && ctx.Err() == nil && running < s.agents() {
			nextID++
			worker := fmt.Sprintf("agent-%d", nextID)
			p, t, err := s.claimNext(worker, "agent", queued)
			if err != nil {
				fatal = err
				cancel()
				break
			}
			if t == nil {
				break
			}
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
		switch {
		case r.err == nil:
			agentFailures = 0
		case ctx.Err() != nil:
			// stopped by the user or by a fatal error; handled once all return
		case errors.Is(r.err, agent.ErrLimit):
			if fatal == nil {
				fatal = fmt.Errorf("limit/kuota AI habis, tunggu reset atau ganti koneksi AI lalu lanjutkan build: %w", r.err)
				cancel()
			}
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
	limit := s.agents()
	p, err := s.Store.Update(func(p *core.Project) error {
		if len(p.ActiveTasks()) >= limit {
			return nil
		}
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
	now := time.Now().UTC()
	resume := hasDraft(p.Dir, t.ID) || !t.StartedAt.IsZero()
	t.Status = core.TaskInProgress
	t.Attempts++
	t.Owner = owner
	t.Seen = now
	if t.StartedAt.IsZero() {
		t.StartedAt = now
	}
	switch {
	case resume:
		t.Progress = "Melanjutkan"
	case source == "mcp":
		t.Progress = "Host mengerjakan"
	default:
		t.Progress = "Menulis kode"
	}
	if p.Job != nil {
		var ids []string
		for _, a := range p.ActiveTasks() {
			ids = append(ids, a.ID)
		}
		p.Job.TaskID = t.ID
		p.Job.TaskIDs = ids
		p.Job.Message = jobMessage("build") + ": " + t.Title
		if n := len(ids); n > 1 {
			p.Job.Message = fmt.Sprintf("%s: %d task paralel (%s)", jobMessage("build"), n, strings.Join(ids, ", "))
		}
	}
	p.Log(source, fmt.Sprintf("%s diambil %s (percobaan %d)", t.ID, owner, t.Attempts))
}

func logIndependent(p *core.Project) {
	s := p.Schedule()
	switch {
	case len(s.Independent) > 0:
		p.Log("system", fmt.Sprintf("Analisa: %d task independen siap paralel (%s)", len(s.Independent), strings.Join(s.Independent, ", ")))
	case len(p.ReadyTasks()) > 0:
		p.Log("system", "Analisa: task siap menunggu file atau halaman yang sedang dikerjakan")
	default:
		p.Log("system", "Analisa: belum ada task independen, masih menunggu dependensi")
	}
}

func lockName(lock string) string {
	if lock == core.LockAll {
		return "seluruh project"
	}
	return strings.Replace(lock, ":", " ", 1)
}

const maxProgress = 72

func clipProgress(s string) string {
	s = strings.TrimSpace(firstLine(s))
	r := []rune(s)
	if len(r) > maxProgress {
		return string(r[:maxProgress-1]) + "…"
	}
	return s
}

func progressFromFiles(files []string) string {
	switch len(files) {
	case 0:
		return "Menulis kode"
	case 1:
		return "Mengubah " + filepath.Base(files[0])
	default:
		return fmt.Sprintf("Mengubah %d file", len(files))
	}
}

func (s *Service) setProgress(id, msg string) {
	s.Store.Update(func(p *core.Project) error {
		if t := p.Task(id); t != nil {
			t.Progress = clipProgress(msg)
		}
		return nil
	})
}

// watchProgress refreshes the Kanban line from files the agent touches.
func (s *Service) watchProgress(ctx context.Context, id, dir string, before checks.Snapshot) func() {
	done := make(chan struct{})
	var once sync.Once
	go func() {
		tick := time.NewTicker(2 * time.Second)
		defer tick.Stop()
		last := ""
		for {
			select {
			case <-ctx.Done():
				return
			case <-done:
				return
			case <-tick.C:
				msg := progressFromFiles(checks.Changed(dir, before))
				if msg == last {
					continue
				}
				last = msg
				s.setProgress(id, msg)
			}
		}
	}()
	return func() { once.Do(func() { close(done) }) }
}

func (s *Service) runTaskWithAgent(ctx context.Context, runner *agent.Runner, p *core.Project, t *core.Task) error {
	if isolated(p, t) {
		return s.runIsolated(ctx, runner, p, t)
	}
	return s.runShared(ctx, runner, p, t)
}

// runShared runs a task in the project tree itself: work that holds the
// whole project, or a project that is not a git repository.
func (s *Service) runShared(ctx context.Context, runner *agent.Runner, p *core.Project, t *core.Task) error {
	id := t.ID
	// Changes by agents running in parallel land in the same tree, so the
	// diff below may include their files too; the reviewer judges criteria.
	before := checks.Take(p.Dir)
	stopWatch := s.watchProgress(ctx, id, p.Dir, before)
	out, err := runner.Run(ctx, agent.Execute, p.Dir, s.taskPrompt(p, t))
	if err != nil {
		stopWatch()
		if ctx.Err() != nil {
			return s.releaseTask(ctx, id)
		}
		s.rejectTask(id, "Agent gagal menjalankan task: "+err.Error())
		return err
	}
	stopWatch()
	s.handOffPatches(id, work.ParsePatches(out))
	changed := checks.Changed(p.Dir, before)
	if len(changed) == 0 {
		s.rejectTask(id, "Tidak ada file yang berubah; implementasi belum dibuat")
		return nil
	}
	s.setProgress(id, "Menjalankan build & tes")
	build := s.runChecks(ctx, id, p.Dir)
	if ctx.Err() != nil {
		return s.releaseTask(ctx, id)
	}
	if !build.Passed {
		s.rejectTask(id, "Verifikasi gagal: "+build.Reason+"\n"+lastOutput(build))
		return nil
	}
	crit, err := s.review(ctx, runner, id, out, p.Dir, changed, &build)
	if crit == nil {
		return err
	}
	return s.concludeTask(id, crit, build.ID, "agent")
}

// review judges acceptance criteria: the coding agent's own JSON when it
// sent one, else a reviewer run in dir. A nil result means the task was
// already released or rejected.
func (s *Service) review(ctx context.Context, runner *agent.Runner, id, out, dir string, changed []string, build *core.BuildRun) ([]core.Criterion, error) {
	s.setProgress(id, "Meninjau hasil")
	p, _ := s.Store.Load()
	t := p.Task(id)
	if crit, ok := work.SelfReview(t, out); ok {
		return crit, nil
	}
	s.setStatus(id, core.TaskReview, "agent", "Build + test lulus, review acceptance criteria")
	out, err := s.reviewRunner(runner).Run(ctx, agent.Think, dir, work.ReviewPrompt(p, t, checks.Excerpt(dir, changed, 60000), build))
	if err != nil {
		if ctx.Err() != nil {
			return nil, s.releaseTask(ctx, id)
		}
		s.rejectTask(id, "Review gagal: "+err.Error())
		return nil, err
	}
	crit, err := work.ParseReview(t, out)
	if err != nil {
		s.rejectTask(id, "Review tidak dapat dibaca: "+err.Error())
		return nil, nil
	}
	return crit, nil
}

// handOffPatches gives changes an agent needed outside its ownership to the
// task that owns the file, as feedback for its next run. A file no pending
// task owns is only logged: the user decides.
func (s *Service) handOffPatches(from string, patches []work.Patch) {
	if len(patches) == 0 {
		return
	}
	s.Store.Update(func(p *core.Project) error {
		for _, pt := range patches {
			o := work.PatchOwner(p, from, pt.File)
			if o == nil {
				p.Log("system", fmt.Sprintf("%s minta perubahan di %s (tanpa pemilik): %s", from, pt.File, firstLine(pt.Change)))
				continue
			}
			o.Notes = append(o.Notes, fmt.Sprintf("Patch dari %s untuk %s: %s", from, pt.File, pt.Change))
			if len(o.Notes) > 5 {
				o.Notes = o.Notes[len(o.Notes)-5:]
			}
			p.Log("system", fmt.Sprintf("%s minta perubahan di %s, diteruskan ke %s", from, pt.File, o.ID))
		}
		return nil
	})
}

// runChecks re-detects commands in dir (a scaffold task may have created
// tooling) and runs them, storing the result.
func (s *Service) runChecks(ctx context.Context, taskID, dir string) core.BuildRun {
	s.checkMu.Lock()
	defer s.checkMu.Unlock()
	return s.runChecksLocked(ctx, taskID, dir)
}

// runChecksLocked is runChecks for a caller already holding checkMu.
func (s *Service) runChecksLocked(ctx context.Context, taskID, dir string) core.BuildRun {
	cmds := scan.Scan(dir).Commands
	build := checks.Run(ctx, dir, cmds, taskID)
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
			t.Progress = ""
			if t.Attempts > 0 {
				t.Attempts--
			}
			p.Log("system", id+" dihentikan, kembali ke TODO")
		}
		return nil
	})
	markResume(s.Store.Dir, id)
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
		t.Progress = ""
		if t.Attempts >= MaxAttempts {
			t.Status = core.TaskBlocked
			p.Log("system", fmt.Sprintf("%s BLOCKED setelah %d percobaan", id, t.Attempts))
		} else {
			t.Status = core.TaskTodo
			p.Log("system", fmt.Sprintf("%s kembali ke TODO: %s", id, firstLine(note)))
		}
		return nil
	})
	markResume(s.Store.Dir, id)
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
		t.Notes, t.Repair = nil, false
		t.Progress = ""
		stampFinished(t)
		clearDraft(p.Dir, id)
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
			t.Attempts, t.Conflicts = 0, 0
			t.Progress = ""
			t.StartedAt = time.Time{}
			clearDraft(p.Dir, id)
		case core.TaskBlocked:
			if t.Status == core.TaskInProgress && p.Busy() {
				return ErrBusy
			}
			t.Progress = ""
		case core.TaskDone:
			b := p.Build(t.LastBuildID)
			if t.Status != core.TaskReview || b == nil || !b.Passed {
				return ErrGate
			}
			t.Progress = ""
			stampFinished(t)
		default:
			return fmt.Errorf("status %s tidak bisa dipindah manual", to)
		}
		t.Status = to
		p.Log("ui", fmt.Sprintf("%s dipindah ke %s", id, to))
		return nil
	})
}

// SetTaskClosed is the Open|Closed gate on a Kanban card. A closed task is
// skipped by the build queue; a running attempt is not aborted.
func (s *Service) SetTaskClosed(id string, closed bool) (*core.Project, error) {
	return s.Store.Update(func(p *core.Project) error {
		t := p.Task(id)
		if t == nil {
			return fmt.Errorf("task %s tidak ada", id)
		}
		t.Closed = closed
		if closed {
			p.Log("ui", id+" ditutup")
		} else {
			p.Log("ui", id+" dibuka")
		}
		return nil
	})
}

// SetQueueClosed closes (or reopens) every TODO task at once, so the build
// queue is empty without moving cards. Running, review, blocked and done
// tasks are untouched.
func (s *Service) SetQueueClosed(closed bool) (*core.Project, error) {
	return s.Store.Update(func(p *core.Project) error {
		n := 0
		for i := range p.Tasks {
			t := &p.Tasks[i]
			if t.Status == core.TaskTodo && t.Closed != closed {
				t.Closed = closed
				n++
			}
		}
		if closed {
			p.Log("ui", fmt.Sprintf("Antrian ditutup: %d task", n))
		} else {
			p.Log("ui", fmt.Sprintf("Antrian dibuka: %d task", n))
		}
		return nil
	})
}

// FixBlocked puts only BLOCKED tasks back in the queue, with a fresh attempt
// budget and their failure notes as feedback. Nothing else is touched. The
// tasks are not started directly: they go through claimNext like any other, so
// a task whose files or pages another running task holds waits for it, and a
// task that is free runs in parallel. If a build is already running or waits
// for an MCP host it picks them up; otherwise a build is started.
func (s *Service) FixBlocked() error {
	var picked bool
	_, err := s.Store.Update(func(p *core.Project) error {
		building := buildActive(p)
		if p.Busy() && !building {
			return ErrBusy
		}
		var ids []string
		for i := range p.Tasks {
			t := &p.Tasks[i]
			if t.Status != core.TaskBlocked {
				continue
			}
			t.Status, t.Attempts, t.Conflicts, t.Closed, t.Progress, t.Repair = core.TaskTodo, 0, 0, false, "", true
			ids = append(ids, t.ID)
		}
		if len(ids) == 0 {
			return ErrNothingToDo
		}
		picked = building
		p.Log("ui", fmt.Sprintf("Perbaiki terblokir: %d task kembali ke antrian (%s)", len(ids), strings.Join(ids, ", ")))
		return nil
	})
	if err != nil || picked {
		return err
	}
	return s.StartBuild()
}

func stampFinished(t *core.Task) {
	now := time.Now().UTC()
	t.FinishedAt = now
	if t.StartedAt.IsZero() {
		t.StartedAt = now
	}
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
	p, _ := s.Store.Load()
	build := s.runChecks(ctx, "", p.Dir)
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
				p.Tasks[i].Status, p.Tasks[i].Attempts, p.Tasks[i].Conflicts = core.TaskTodo, 0, 0
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
