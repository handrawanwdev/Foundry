package pipeline

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/handrawanw/foundry/internal/core"
	"github.com/handrawanw/foundry/internal/modules/work"
)

// WorkItem is what an MCP host must do next.
type WorkItem struct {
	Kind      string   `json:"kind"` // discovery | design | plan | task | wait | none
	TaskID    string   `json:"task_id,omitempty"`
	Prompt    string   `json:"prompt"`
	Submit    string   `json:"submit"`               // how to hand the result back
	AlsoReady []string `json:"also_ready,omitempty"` // other independent tasks another worker can claim
}

// NextWork hands the pending job to the MCP host. For builds it returns the
// task worker already holds, else claims the next one no other worker's task
// overlaps. Several hosts can build in parallel, each with its own worker
// name; a host whose tasks are all locked gets kind "wait".
func (s *Service) NextWork(worker string) (*WorkItem, error) {
	worker = strings.TrimSpace(worker)
	p, err := s.Store.Load()
	if err != nil {
		return nil, err
	}
	if p.Job == nil || p.Job.Status != core.JobWaiting {
		if m := work.PendingSupport(p); m != nil && m.Status == core.SupportWaiting {
			return &WorkItem{Kind: work.KindSupport, Prompt: work.SupportPrompt(p),
				Submit: fmt.Sprintf(`submit_work {"kind": %q, "result": "<the JSON object>"}`, work.KindSupport)}, nil
		}
		return &WorkItem{Kind: "none", Prompt: nothingMessage(p)}, nil
	}
	switch p.Job.Kind {
	case work.KindDiscovery, work.KindDesign, work.KindPlan, work.KindDesignQA, work.KindReference, work.KindStack, work.KindStructure, work.KindPRD:
		return &WorkItem{Kind: p.Job.Kind, Prompt: s.promptFor(p, p.Job.Kind),
			Submit: fmt.Sprintf(`submit_work {"kind": %q, "result": "<the JSON object>"}`, p.Job.Kind)}, nil
	case "build":
		now := time.Now().UTC()
		if np, err := s.Store.Update(func(p *core.Project) error {
			touched := false
			for i := range p.Tasks {
				if t := &p.Tasks[i]; t.Status == core.TaskInProgress && t.Owner == worker {
					t.Seen, touched = now, true
				}
			}
			if !reclaimSilent(p, now) && !touched {
				return errUnchanged
			}
			return nil
		}); err == nil {
			p = np
		} else if !errors.Is(err, errUnchanged) {
			return nil, err
		}
		for i := range p.Tasks {
			if t := &p.Tasks[i]; t.Status == core.TaskInProgress && t.Owner == worker {
				return taskItem(s, p, t), nil
			}
		}
		p, t, err := s.claimNext(worker, "mcp", nil)
		if err != nil {
			return nil, err
		}
		if t != nil {
			return taskItem(s, p, t), nil
		}
		if active := p.ActiveTasks(); len(active) > 0 {
			var ids []string
			for _, a := range active {
				ids = append(ids, a.ID)
			}
			return &WorkItem{Kind: "wait", Prompt: fmt.Sprintf(
				"Task independen sedang dikerjakan (%s). Tunggu sebentar lalu panggil next_work lagi.", strings.Join(ids, ", "))}, nil
		}
		if !s.finishBuild(context.Background()) {
			return &WorkItem{Kind: "none", Prompt: "Verifikasi sudah dijalankan host lain."}, nil
		}
		return &WorkItem{Kind: "none", Prompt: "Semua task yang bisa dikerjakan sudah selesai. Foundry menjalankan verifikasi."}, nil
	}
	return &WorkItem{Kind: "none", Prompt: nothingMessage(p)}, nil
}

// HostLease is how long an MCP host may stay silent (no next_work,
// update_task or submit_task) on a claimed task before another worker may
// take it: a host that died must not hold its files forever.
const HostLease = 30 * time.Minute

var errUnchanged = errors.New("tidak ada perubahan")

// reclaimSilent returns tasks of hosts silent past HostLease to TODO; the
// attempt does not count against the task.
func reclaimSilent(p *core.Project, now time.Time) bool {
	changed := false
	for i := range p.Tasks {
		t := &p.Tasks[i]
		if t.Status != core.TaskInProgress || now.Sub(t.Seen) < HostLease {
			continue
		}
		p.Log("system", fmt.Sprintf("%s dilepas: %s tidak aktif lebih dari %s", t.ID, cmp.Or(t.Owner, "host"), HostLease))
		t.Status, t.Owner, t.Progress = core.TaskTodo, "", ""
		if t.Attempts > 0 {
			t.Attempts--
		}
		markResume(p.Dir, t.ID)
		changed = true
	}
	return changed
}

func taskItem(s *Service, p *core.Project, t *core.Task) *WorkItem {
	slots := max(0, s.agents()-len(p.ActiveTasks()))
	var also []string
	for _, x := range p.IndependentTasks() {
		if x.ID == t.ID || slots == 0 {
			continue
		}
		also = append(also, x.ID)
		slots--
	}
	return &WorkItem{Kind: work.KindTask, TaskID: t.ID, Prompt: s.taskPrompt(p, t), AlsoReady: also,
		Submit: fmt.Sprintf(`submit_task {"task_id": %q, "criteria": [{"index": 1, "met": true, "evidence": "file + what it does"}], "patches": [{"file": "", "change": ""}]}`, t.ID)}
}

func nothingMessage(p *core.Project) string {
	switch p.Stage {
	case core.StageTell:
		return "User belum menceritakan idenya di UI Foundry."
	case core.StageDesign:
		return "Rancangan menunggu persetujuan user di UI Foundry."
	case core.StageBuild:
		return "Task siap. User perlu menekan Mulai Bangun di UI Foundry."
	case core.StageReady:
		return "Project sudah siap diperiksa."
	}
	return "Tidak ada pekerjaan saat ini."
}

// SubmitWork applies a discovery/design/plan/support answer from the MCP host.
func (s *Service) SubmitWork(kind, result string) error {
	if kind == work.KindSupport {
		return s.applySupport(result)
	}
	p, err := s.Store.Load()
	if err != nil {
		return err
	}
	if p.Job == nil || p.Job.Status != core.JobWaiting || p.Job.Kind != kind {
		return fmt.Errorf("%w: Foundry tidak sedang menunggu %s", ErrWrongStage, kind)
	}
	err = s.applyAI(kind, result, "mcp")
	s.noteRejection(kind, err) // the host gets the error; next_work shows it too
	return err
}

// SubmitTask is called by the MCP host after implementing a task. Foundry
// runs build/test itself; DONE requires passing checks and every criterion
// met with evidence. patches are changes the host needed in files other
// tasks own; they go to the owner whatever the outcome. A non-empty worker
// must be the task's owner: a task reclaimed from a silent host is not
// submitted by it.
func (s *Service) SubmitTask(id, worker string, verdicts []work.Verdict, patches []work.Patch) (*core.Task, *core.BuildRun, error) {
	worker = strings.TrimSpace(worker)
	p, err := s.Store.Update(func(p *core.Project) error {
		t := p.Task(id)
		if t == nil || t.Status != core.TaskInProgress {
			return fmt.Errorf("task %s tidak sedang IN_PROGRESS", id)
		}
		if worker != "" && t.Owner != worker {
			return fmt.Errorf("task %s dipegang %s, bukan %s", id, cmp.Or(t.Owner, "host lain"), worker)
		}
		t.Seen = time.Now().UTC()
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	t := p.Task(id)
	s.setProgress(id, "Menjalankan build & tes")
	stop := s.keepSeen(id)
	build, err := s.runChecks(context.Background(), id, p.Dir)
	stop()
	if err != nil { // nothing ran: the task stays with its host to submit again
		s.setProgress(id, "")
		return nil, nil, err
	}
	s.handOffPatches(id, work.CleanPatches(patches))
	if !build.Passed {
		s.rejectTask(id, "Verifikasi gagal: "+build.Reason+"\n"+lastOutput(build))
	} else {
		s.setProgress(id, "Meninjau hasil")
		s.setStatus(id, core.TaskReview, "mcp", "Build + test lulus")
		for i := range verdicts {
			if verdicts[i].Evidence == "" {
				verdicts[i].Met = false // a claim without evidence is not accepted
			}
		}
		if err := s.concludeTask(id, work.MergeCriteria(t, verdicts), build.ID, "mcp"); err != nil {
			return nil, nil, err
		}
	}
	p, err = s.Store.Load()
	if err != nil {
		return nil, nil, err
	}
	return p.Task(id), p.Build(build.ID), nil
}

// keepSeen refreshes the task's Seen while Foundry itself works on it, so
// long checks never look like a silent host.
func (s *Service) keepSeen(id string) func() {
	done := make(chan struct{})
	go func() {
		tick := time.NewTicker(time.Minute)
		defer tick.Stop()
		for {
			select {
			case <-done:
				return
			case <-tick.C:
				s.Store.Update(func(p *core.Project) error {
					if t := p.Task(id); t != nil {
						t.Seen = time.Now().UTC()
					}
					return nil
				})
			}
		}
	}()
	return func() { close(done) }
}

// UpdateTaskNote lets the host record a short live progress line, lasting
// feedback, or a blocker. progress replaces the Kanban line; note is kept.
func (s *Service) UpdateTaskNote(id, note, progress string, blocked bool) (*core.Task, error) {
	p, err := s.Store.Update(func(p *core.Project) error {
		t := p.Task(id)
		if t == nil {
			return fmt.Errorf("task %s tidak ada", id)
		}
		t.Seen = time.Now().UTC()
		if progress != "" {
			t.Progress = clipProgress(progress)
		} else if note != "" && t.Active() && !blocked {
			t.Progress = clipProgress(note)
		}
		if blocked {
			t.Status = core.TaskBlocked
			t.Progress = ""
			if note != "" {
				t.Notes = append(t.Notes, note)
			}
		} else if note != "" && progress != "" {
			t.Notes = append(t.Notes, note)
		}
		msg := progress
		if msg == "" {
			msg = note
		}
		if strings.TrimSpace(msg) != "" {
			p.Log("mcp", fmt.Sprintf("%s: %s", id, firstLine(msg)))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return p.Task(id), nil
}
