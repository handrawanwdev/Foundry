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

// runIsolated runs a task in a worktree made from a snapshot of the current
// project tree. Its diff must stay inside the task's ownership, pass
// build/test and criteria, and is then applied to the project tree — tested
// again first when the tree moved on meanwhile. A rejected attempt leaves the
// project tree untouched.
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
	// The agent's MCP tools (update_task) find the project from the cwd.
	state := filepath.Join(tree.Dir, core.StateDir)
	os.Symlink(filepath.Join(p.Dir, core.StateDir), state)
	defer func() {
		os.Remove(state)
		tree.Remove()
	}()

	stopWatch := s.watchProgress(ctx, id, tree.Dir, checks.Take(tree.Dir))
	out, err := runner.Run(ctx, agent.Execute, tree.Dir, work.TaskPrompt(p, t))
	stopWatch()
	if err != nil {
		if ctx.Err() != nil {
			return s.releaseTask(ctx, id)
		}
		s.rejectTask(id, "Agent gagal menjalankan task: "+err.Error())
		return err
	}
	s.handOffPatches(id, work.ParsePatches(out))
	os.Remove(state)
	patch, changed, err := tree.Diff(ctx)
	if err != nil {
		s.rejectTask(id, "Perubahan tidak dapat dibaca: "+err.Error())
		return nil
	}
	if len(changed) == 0 {
		s.rejectTask(id, "Tidak ada file yang berubah; implementasi belum dibuat")
		return nil
	}
	if note := s.crossedOwnership(id, changed); note != "" {
		s.rejectTask(id, note)
		return nil
	}
	s.setProgress(id, "Menjalankan build & tes")
	build := s.runChecks(ctx, id, tree.Dir)
	if ctx.Err() != nil {
		return s.releaseTask(ctx, id)
	}
	if !build.Passed {
		s.rejectTask(id, "Verifikasi gagal: "+build.Reason+"\n"+lastOutput(build))
		return nil
	}
	crit, err := s.review(ctx, runner, id, out, tree.Dir, changed, &build)
	if crit == nil {
		return err
	}
	if !work.AllMet(crit) {
		return s.concludeTask(id, crit, build.ID, "agent") // rejects with the missing criteria
	}
	s.setProgress(id, "Menggabungkan ke project")
	buildID, note, conflict := s.merge(ctx, id, p.Dir, tree, patch, build.ID)
	if note == "" { // in the project now, even when Stop came meanwhile
		return s.concludeTask(id, crit, buildID, "agent")
	}
	if ctx.Err() != nil {
		return s.releaseTask(ctx, id)
	}
	if conflict {
		// Another task landed first; the work was fine. The next attempt
		// starts from the newer tree and does not count.
		s.Store.Update(func(p *core.Project) error {
			if t := p.Task(id); t != nil && t.Attempts > 0 {
				t.Attempts--
			}
			return nil
		})
	}
	s.rejectTask(id, note)
	return nil
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

// crossedOwnership names changed files owned by another pending task.
func (s *Service) crossedOwnership(id string, changed []string) string {
	p, err := s.Store.Load()
	if err != nil {
		return ""
	}
	t := p.Task(id)
	var bad []string
	for _, f := range changed {
		if o := work.ForeignOwner(p, t, f); o != nil {
			bad = append(bad, fmt.Sprintf("%s (%s)", f, o.ID))
		}
	}
	if len(bad) == 0 {
		return ""
	}
	return "Mengubah file milik task lain: " + strings.Join(bad, ", ") +
		". Jangan edit file itu; kirim perubahan yang dibutuhkan lewat \"patches\"."
}

// merge applies a task's patch to the project tree under checkMu, so merges
// and checks never interleave. When the tree changed since the worktree's
// snapshot, the patch is replayed on a fresh snapshot and tested there
// first, so what lands is exactly what passed. It returns the build that
// gates DONE, or a note when the task must try again; conflict tells a
// clash with changes merged meanwhile from a failure of the task itself.
func (s *Service) merge(ctx context.Context, id, dir string, tree *gitwt.Tree, patch []byte, buildID string) (string, string, bool) {
	s.checkMu.Lock()
	defer s.checkMu.Unlock()
	now, err := gitwt.Snapshot(ctx, dir, core.StateDir)
	if err != nil {
		return "", "Gagal membaca project untuk digabung: " + err.Error(), false
	}
	if !sameTree(ctx, dir, now, tree.Base) {
		if err := tree.Rebase(ctx, now, patch); err != nil {
			return "", conflictNote(err), errors.Is(err, gitwt.ErrConflict)
		}
		build := s.runChecksLocked(ctx, id, tree.Dir)
		if !build.Passed {
			return "", "Verifikasi gagal setelah digabung dengan perubahan task lain: " + build.Reason + "\n" + lastOutput(build), false
		}
		buildID = build.ID
	}
	// Not cancellable: Stop must not leave the project half patched.
	if err := gitwt.Apply(context.WithoutCancel(ctx), dir, patch); err != nil {
		return "", conflictNote(err), errors.Is(err, gitwt.ErrConflict)
	}
	s.Store.Update(func(p *core.Project) error {
		p.Log("system", id+" digabung ke project")
		return nil
	})
	return buildID, "", false
}

func sameTree(ctx context.Context, dir, a, b string) bool {
	ta, err1 := gitwt.TreeOf(ctx, dir, a)
	tb, err2 := gitwt.TreeOf(ctx, dir, b)
	return err1 == nil && err2 == nil && ta == tb
}

func conflictNote(err error) string {
	if errors.Is(err, gitwt.ErrConflict) {
		return "Konflik saat digabung dengan perubahan lain; ulangi dari kondisi project terbaru: " + firstLine(err.Error())
	}
	return "Gagal menggabungkan: " + err.Error()
}
