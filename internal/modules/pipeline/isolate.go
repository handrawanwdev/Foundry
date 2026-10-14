package pipeline

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/handrawanw/foundry/internal/core"
	"github.com/handrawanw/foundry/internal/modules/agent"
	"github.com/handrawanw/foundry/internal/modules/checks"
	"github.com/handrawanw/foundry/internal/modules/gitwt"
	"github.com/handrawanw/foundry/internal/modules/work"
)

// RepairRounds is how often one attempt may fix its own result in place
// (failed build/test, unmet criteria, merge conflict) before the task goes
// back to the queue. A repair keeps the agent's changes in the worktree, so
// it costs far less than a fresh attempt that redoes the whole task.
const RepairRounds = 2

// MaxFreeConflicts bounds the merge conflicts that do not count as a failed
// attempt, so a task that keeps clashing cannot loop forever.
const MaxFreeConflicts = 2

// isolated reports whether t runs in its own git worktree. Work holding the
// whole project runs alone anyway and keeps the shared tree, as does a
// project that is not a git repository.
func isolated(p *core.Project, t *core.Task) bool {
	for _, l := range t.Locks() {
		if l == core.LockAll {
			return false
		}
	}
	return gitwt.Usable(p.Dir)
}

// settled is where one round of an isolated attempt ended.
type settled struct {
	done     bool // concluded, rejected or released already; err is final
	err      error
	problem  string           // what keeps the work out of the project
	crit     []core.Criterion // judged criteria, when the build passed
	buildID  string
	conflict bool // problem is a clash with work merged meanwhile
	clean    bool // the clash merged cleanly: re-check, no agent needed
	final    bool // an agent cannot repair problem
}

// runIsolated runs a task in a worktree made from a snapshot of the current
// project tree. Its diff, minus edits outside the task's ownership and stray
// files, must pass build/test and criteria, and is then applied to the
// project tree — tested again first when the tree moved on meanwhile. A
// failing round is repaired in place by the agent up to RepairRounds times;
// a rejected attempt leaves the project tree untouched but saves the
// worktree diff so the next claim continues instead of starting over.
func (s *Service) runIsolated(ctx context.Context, runner *agent.Runner, p *core.Project, t *core.Task) error {
	id := t.ID
	base, err := gitwt.Snapshot(ctx, p.Dir, core.StateDir)
	if err != nil {
		return s.isolationFailed(ctx, runner, p, t, err)
	}
	tree, err := gitwt.Add(ctx, p.Dir, base, strings.ToLower(id))
	if err != nil {
		return s.isolationFailed(ctx, runner, p, t, err)
	}
	defer func() {
		s.persistDraft(context.WithoutCancel(ctx), id, tree)
		tree.Remove()
	}()

	prompt, out := work.TaskPrompt(p, t), ""
	if s.applyDraft(ctx, id, tree) {
		prompt = work.ContinuePrompt(p, t)
		s.setProgress(id, "Melanjutkan")
	}
	var crit []core.Criterion // met criteria kept across a clean re-merge
	for round := 0; ; round++ {
		if prompt != "" {
			out, err = s.runIn(ctx, runner, id, tree, p.Dir, prompt)
			if err != nil {
				if ctx.Err() != nil {
					return s.releaseTask(ctx, id)
				}
				s.rejectTask(id, "Agent gagal menjalankan task: "+err.Error())
				return err
			}
			s.handOffPatches(id, work.ParsePatches(out))
			crit = nil
		}
		r := s.settle(ctx, runner, id, p.Dir, tree, out, crit)
		if r.done {
			return r.err
		}
		if r.final || round >= RepairRounds {
			return s.giveUp(id, r)
		}
		s.Store.Update(func(p *core.Project) error {
			if t := p.Task(id); t != nil {
				t.Status = core.TaskInProgress // back from REVIEW while repairing
			}
			p.Log("system", fmt.Sprintf("%s diperbaiki di tempat (%d/%d): %s", id, round+1, RepairRounds, firstLine(r.problem)))
			return nil
		})
		if r.clean {
			prompt, crit = "", r.crit
			continue
		}
		s.setProgress(id, "Memperbaiki: "+r.problem)
		cur, err := s.Store.Load()
		if err != nil || cur.Task(id) == nil {
			return s.giveUp(id, r)
		}
		prompt = work.RepairPrompt(cur, cur.Task(id), r.problem)
	}
}

// runIn runs the agent in the worktree. The agent's MCP tools (update_task)
// find the project from the cwd, so .foundry is linked in for the run only.
func (s *Service) runIn(ctx context.Context, runner *agent.Runner, id string, tree *gitwt.Tree, project, prompt string) (string, error) {
	state := filepath.Join(tree.Dir, core.StateDir)
	os.Symlink(filepath.Join(project, core.StateDir), state)
	defer os.Remove(state)
	stop := s.watchProgress(ctx, id, tree.Dir, checks.Take(tree.Dir))
	defer stop()
	return s.measured(id, prompt, func() (agent.Result, error) { return runner.RunDetailed(ctx, agent.Execute, tree.Dir, prompt) })
}

// settle checks one round of work: collect the diff, build/test, criteria
// (reviewed only when crit is nil), then merge into the project.
func (s *Service) settle(ctx context.Context, runner *agent.Runner, id, dir string, tree *gitwt.Tree, out string, crit []core.Criterion) settled {
	patch, changed, err := s.collect(ctx, id, tree)
	if err != nil {
		s.rejectTask(id, "Perubahan tidak dapat dibaca: "+err.Error())
		return settled{done: true}
	}
	if len(changed) == 0 {
		return settled{problem: "Tidak ada file yang berubah; implementasi belum dibuat"}
	}
	s.setProgress(id, "Menjalankan build & tes")
	build := s.runChecks(ctx, id, tree.Dir)
	if ctx.Err() != nil {
		return settled{done: true, err: s.releaseTask(ctx, id)}
	}
	if !build.Passed {
		return settled{problem: "Verifikasi gagal: " + build.Reason + "\n" + lastOutput(build)}
	}
	if crit == nil {
		if crit, err = s.review(ctx, runner, id, out, tree.Dir, changed, patch, &build); crit == nil {
			return settled{done: true, err: err}
		}
	}
	if !work.AllMet(crit) {
		return settled{problem: missingCriteria(crit), crit: crit, buildID: build.ID}
	}
	s.setProgress(id, "Menggabungkan ke project")
	buildID, note, kind := s.merge(ctx, id, dir, tree, patch, build.ID)
	if kind == merged { // in the project now, even when Stop came meanwhile
		return settled{done: true, err: s.concludeTask(id, crit, buildID, "agent")}
	}
	if ctx.Err() != nil {
		return settled{done: true, err: s.releaseTask(ctx, id)}
	}
	switch kind {
	case mergeConflict:
		files, err := s.remerge(ctx, dir, tree, patch)
		switch {
		case err != nil:
			return settled{problem: note, conflict: true, final: true}
		case len(files) == 0:
			return settled{problem: note, conflict: true, clean: true, crit: crit}
		}
		return settled{conflict: true, problem: "Konflik dengan perubahan task lain yang sudah digabung. File berisi conflict marker " +
			"(<<<<<<< ======= >>>>>>>): " + strings.Join(files, ", ") + ". Selesaikan dengan mempertahankan maksud kedua sisi, lalu pastikan build/test lulus."}
	case mergeBroken: // the worktree holds the rebased work: repairable there
		return settled{problem: note}
	}
	return settled{problem: note, final: true}
}

// giveUp ends the attempt: the task goes back to TODO (or BLOCKED) with the
// problem as feedback. Unmet criteria are stored with the rejection; a
// conflict with work merged meanwhile does not count, up to
// MaxFreeConflicts times.
func (s *Service) giveUp(id string, r settled) error {
	if r.crit != nil && !r.conflict {
		return s.concludeTask(id, r.crit, r.buildID, "agent") // rejects with the missing criteria
	}
	if r.conflict {
		s.Store.Update(func(p *core.Project) error {
			if t := p.Task(id); t != nil && t.Attempts > 0 && t.Conflicts < MaxFreeConflicts {
				t.Conflicts++
				t.Attempts--
			}
			return nil
		})
	}
	s.rejectTask(id, r.problem)
	return nil
}

func missingCriteria(crit []core.Criterion) string {
	var miss []string
	for _, c := range crit {
		if !c.Met {
			miss = append(miss, c.Text)
		}
	}
	return "Acceptance criteria belum terpenuhi: " + strings.Join(miss, "; ")
}

// isolationFailed falls back to the shared tree when git cannot give the
// task a worktree; the build keeps going.
func (s *Service) isolationFailed(ctx context.Context, runner *agent.Runner, p *core.Project, t *core.Task, err error) error {
	s.Store.Update(func(p *core.Project) error {
		p.Log("system", fmt.Sprintf("%s tanpa worktree terpisah: %v", t.ID, err))
		return nil
	})
	return s.runShared(ctx, runner, p, t)
}

// maxHandoffDiff bounds the diff excerpt handed to a file's owner.
const maxHandoffDiff = 1500

// collect returns the worktree's diff after dropping what must not land:
// edits to files another pending task owns (handed to that owner as a
// patch with the diff, instead of throwing the whole attempt away) and
// stray files outside the approved structure.
func (s *Service) collect(ctx context.Context, id string, tree *gitwt.Tree) ([]byte, []string, error) {
	patch, changed, err := tree.Diff(ctx)
	if err != nil || len(changed) == 0 {
		return patch, changed, err
	}
	p, err := s.Store.Load()
	if err != nil || p.Task(id) == nil {
		return patch, changed, nil
	}
	t := p.Task(id)
	top := tree.TopLevel(ctx)
	var drop, stray []string
	var handoff []work.Patch
	for _, f := range changed {
		if o := work.ForeignOwner(p, t, f); o != nil {
			diff := tree.FileDiff(ctx, f)
			if len(diff) > maxHandoffDiff {
				diff = diff[:maxHandoffDiff] + "\n…"
			}
			handoff = append(handoff, work.Patch{File: f, Change: "edit " + id + " tidak digabung (file milikmu); terapkan bila dibutuhkan:\n" + diff})
			drop = append(drop, f)
		} else if work.StrayFile(p, top, f) {
			stray = append(stray, f)
			drop = append(drop, f)
		}
	}
	if len(drop) == 0 {
		return patch, changed, nil
	}
	if err := tree.Drop(ctx, drop); err != nil {
		return nil, nil, err
	}
	s.handOffPatches(id, handoff)
	if len(stray) > 0 {
		s.Store.Update(func(p *core.Project) error {
			p.Log("system", fmt.Sprintf("%s: file di luar struktur dibuang: %s", id, strings.Join(stray, ", ")))
			return nil
		})
	}
	return tree.Diff(ctx)
}

type mergeResult int

const (
	merged        mergeResult = iota
	mergeConflict             // clashes with work merged meanwhile
	mergeBroken               // rebased cleanly but build/test failed
	mergeFailed               // git could not read or apply; not repairable
)

// merge applies a task's patch to the project tree under checkMu, so merges
// and checks never interleave. When the tree changed since the worktree's
// snapshot, the patch is replayed on a fresh snapshot and tested there
// first, so what lands is exactly what passed. It returns the build that
// gates DONE, or a note and why the work could not land.
func (s *Service) merge(ctx context.Context, id, dir string, tree *gitwt.Tree, patch []byte, buildID string) (string, string, mergeResult) {
	s.checkMu.Lock()
	defer s.checkMu.Unlock()
	now, err := gitwt.Snapshot(ctx, dir, core.StateDir)
	if err != nil {
		return "", "Gagal membaca project untuk digabung: " + err.Error(), mergeFailed
	}
	if !sameTree(ctx, dir, now, tree.Base) {
		if err := tree.Rebase(ctx, now, patch); err != nil {
			return "", conflictNote(err), conflictKind(err)
		}
		build := s.runChecksLocked(ctx, id, tree.Dir)
		if !build.Passed {
			return "", "Verifikasi gagal setelah digabung dengan perubahan task lain: " + build.Reason + "\n" + lastOutput(build), mergeBroken
		}
		buildID = build.ID
	}
	// Not cancellable: Stop must not leave the project half patched.
	if err := gitwt.Apply(context.WithoutCancel(ctx), dir, patch); err != nil {
		return "", conflictNote(err), conflictKind(err)
	}
	s.Store.Update(func(p *core.Project) error {
		p.Log("system", id+" digabung ke project")
		return nil
	})
	return buildID, "", merged
}

// remerge three-way merges the task's patch onto the current project tree
// in the worktree, returning the files left with conflict markers.
func (s *Service) remerge(ctx context.Context, dir string, tree *gitwt.Tree, patch []byte) ([]string, error) {
	s.checkMu.Lock()
	defer s.checkMu.Unlock()
	now, err := gitwt.Snapshot(ctx, dir, core.StateDir)
	if err != nil {
		return nil, err
	}
	return tree.Merge(ctx, now, patch)
}

func sameTree(ctx context.Context, dir, a, b string) bool {
	ta, err1 := gitwt.TreeOf(ctx, dir, a)
	tb, err2 := gitwt.TreeOf(ctx, dir, b)
	return err1 == nil && err2 == nil && ta == tb
}

func conflictKind(err error) mergeResult {
	if errors.Is(err, gitwt.ErrConflict) {
		return mergeConflict
	}
	return mergeFailed
}

func conflictNote(err error) string {
	if errors.Is(err, gitwt.ErrConflict) {
		return "Konflik saat digabung dengan perubahan lain; ulangi dari kondisi project terbaru: " + firstLine(err.Error())
	}
	return "Gagal menggabungkan: " + err.Error()
}

func draftPath(dir, id string) string {
	id = strings.NewReplacer("/", "_", "\\", "_").Replace(id)
	return filepath.Join(dir, core.StateDir, "drafts", id+".patch")
}

func hasDraft(dir, id string) bool {
	_, err := os.Stat(draftPath(dir, id))
	return err == nil
}

func clearDraft(dir, id string) {
	os.Remove(draftPath(dir, id))
}

// markResume records that id left IN_PROGRESS and should continue next claim.
// An existing patch is kept.
func markResume(dir, id string) {
	p := draftPath(dir, id)
	if hasDraft(dir, id) {
		return
	}
	os.MkdirAll(filepath.Dir(p), 0o755)
	os.WriteFile(p, nil, 0o644)
}

func (s *Service) persistDraft(ctx context.Context, id string, tree *gitwt.Tree) {
	p, err := s.Store.Load()
	if err != nil || p.Task(id) == nil || p.Task(id).Status == core.TaskDone {
		clearDraft(s.Store.Dir, id)
		return
	}
	patch, _, err := tree.Diff(ctx)
	if err != nil {
		markResume(s.Store.Dir, id)
		return
	}
	path := draftPath(s.Store.Dir, id)
	os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, patch, 0o644); err != nil {
		markResume(s.Store.Dir, id)
	}
}

func (s *Service) applyDraft(ctx context.Context, id string, tree *gitwt.Tree) bool {
	b, err := os.ReadFile(draftPath(s.Store.Dir, id))
	if err != nil {
		return false
	}
	if len(b) == 0 {
		return true
	}
	if err := tree.ApplyPatch(ctx, b); err != nil {
		s.Store.Update(func(p *core.Project) error {
			p.Log("system", fmt.Sprintf("%s draf tidak bersih diterapkan: %s", id, firstLine(err.Error())))
			return nil
		})
		return true
	}
	s.Store.Update(func(p *core.Project) error {
		p.Log("system", id+" melanjutkan draf")
		return nil
	})
	return true
}

func (s *Service) taskPrompt(p *core.Project, t *core.Task) string {
	if t.Repair {
		return work.BlockedPrompt(p, t, hasDraft(p.Dir, t.ID))
	}
	if hasDraft(p.Dir, t.ID) {
		return work.ContinuePrompt(p, t)
	}
	return work.TaskPrompt(p, t)
}
