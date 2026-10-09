package pipeline_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/handrawanw/foundry/internal/core"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t"}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func gitRepo(t *testing.T, dir string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(".foundry/\n"), 0o644)
	git(t, dir, "init", "-q")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-q", "-m", "init")
}

// In a git project every non-scaffold task builds in its own worktree and
// lands in the project tree only after passing; Foundry never commits.
func TestIsolatedBuildMergesIntoProject(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	dir := s.Store.Dir
	gitRepo(t, dir)
	head := git(t, dir, "rev-parse", "HEAD")
	approveAndPlan(t, s)
	if err := s.StartBuild(); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p := load(t, s)
	for _, tk := range p.Tasks {
		if tk.Status != core.TaskDone {
			t.Fatalf("%s status %s notes %v", tk.ID, tk.Status, tk.Notes)
		}
		if tk.Area == "scaffold" {
			continue
		}
		f := filepath.Join(dir, "app", strings.ToLower(strings.ReplaceAll(tk.ID, "-", "_"))+".go")
		if _, err := os.Stat(f); err != nil {
			t.Fatalf("%s not merged: %v", tk.ID, err)
		}
	}
	if !strings.Contains(strings.Join(logTexts(p), "\n"), "digabung ke project") {
		t.Fatalf("log = %v", logTexts(p))
	}
	if got := git(t, dir, "rev-parse", "HEAD"); got != head {
		t.Fatalf("HEAD moved: %s", got)
	}
	if wt := git(t, dir, "worktree", "list"); strings.Count(wt, "\n") != 0 {
		t.Fatalf("worktrees left:\n%s", wt)
	}
}

// buildIsolated runs tasks in a git project with the fake agent in mode.
func buildIsolated(t *testing.T, mode, agents string, tasks []core.Task) (*core.Project, string) {
	t.Helper()
	t.Setenv("FOUNDRY_AGENTS", agents)
	s := newService(t, fakeAI{}, mode)
	dir := s.Store.Dir
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/k\n\ngo 1.22\n"), 0o644)
	gitRepo(t, dir)
	if _, err := s.Store.Update(func(p *core.Project) error {
		p.Stage = core.StageBuild
		p.Tasks = tasks
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.StartBuild(); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	return load(t, s), dir
}

func apiTask(id string, files ...string) core.Task {
	return core.Task{ID: id, Title: id, Area: "api", Files: files, AcceptanceCriteria: []string{"ok"}, Status: core.TaskTodo}
}

// An edit to a file another pending task owns is dropped from the attempt
// and handed to the owner; the rest of the work still lands.
func TestIsolatedDropsForeignFile(t *testing.T) {
	p, dir := buildIsolated(t, "cross", "1", []core.Task{
		apiTask("TASK-001", "app/task_001.go", "app/task_001_test.go"),
		apiTask("TASK-002", "b/y.go"),
	})
	for _, tk := range p.Tasks {
		if tk.Status != core.TaskDone || tk.Attempts != 1 {
			t.Fatalf("%s = %s attempts %d notes %v", tk.ID, tk.Status, tk.Attempts, tk.Notes)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "app", "task_001.go")); err != nil {
		t.Fatal("TASK-001 work did not land")
	}
	if _, err := os.Stat(filepath.Join(dir, "b", "y.go")); err == nil {
		t.Fatal("foreign edit reached the project tree")
	}
	if !strings.Contains(strings.Join(logTexts(p), "\n"), "b/y.go, diteruskan ke TASK-002") {
		t.Fatalf("log = %v", logTexts(p))
	}
}

// A task that returns to the queue keeps its worktree diff and continues
// on the next claim instead of starting over.
func TestIsolatedResumesDraft(t *testing.T) {
	p, dir := buildIsolated(t, "fail_until_resume", "1", []core.Task{apiTask("TASK-001", "app/task_001.go", "app/task_001_test.go")})
	tk := p.Task("TASK-001")
	if tk.Status != core.TaskDone || tk.Attempts != 2 {
		t.Fatalf("TASK-001 = %s attempts %d notes %v", tk.Status, tk.Attempts, tk.Notes)
	}
	if !strings.Contains(strings.Join(logTexts(p), "\n"), "melanjutkan draf") {
		t.Fatalf("log = %v", logTexts(p))
	}
	if _, err := os.Stat(filepath.Join(dir, "app", "restarted.txt")); err == nil {
		t.Fatal("second claim started over")
	}
	if _, err := os.Stat(filepath.Join(dir, "app", "task_001.go")); err != nil {
		t.Fatal("task work did not land")
	}
}

// A failing build is repaired in place by the same attempt instead of
// redoing the task from scratch.
func TestIsolatedRepairsInPlace(t *testing.T) {
	p, _ := buildIsolated(t, "fail_once", "1", []core.Task{apiTask("TASK-001", "app/task_001.go", "app/task_001_test.go")})
	tk := p.Task("TASK-001")
	if tk.Status != core.TaskDone || tk.Attempts != 1 {
		t.Fatalf("TASK-001 = %s attempts %d notes %v", tk.Status, tk.Attempts, tk.Notes)
	}
	if !strings.Contains(strings.Join(logTexts(p), "\n"), "diperbaiki di tempat (1/") {
		t.Fatalf("log = %v", logTexts(p))
	}
}

// Notes at the root and junk files never land in the project.
func TestIsolatedDropsStrayFiles(t *testing.T) {
	p, dir := buildIsolated(t, "stray", "1", []core.Task{apiTask("TASK-001", "app/task_001.go", "app/task_001_test.go")})
	if tk := p.Task("TASK-001"); tk.Status != core.TaskDone {
		t.Fatalf("TASK-001 = %s notes %v", tk.Status, tk.Notes)
	}
	for _, f := range []string{"NOTES.md", filepath.Join("app", "debug.log")} {
		if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
			t.Fatalf("%s reached the project tree", f)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "app", "task_001.go")); err != nil {
		t.Fatal("task work did not land")
	}
}

// Two parallel tasks add the same file: the later merge conflicts, retries
// from the newer tree, and the conflict does not count as a failed attempt.
func TestIsolatedConflictRetriesFree(t *testing.T) {
	t.Setenv("FOUNDRY_AGENTS", "2")
	s := newService(t, fakeAI{}, "clash")
	dir := s.Store.Dir
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/k\n\ngo 1.22\n"), 0o644)
	gitRepo(t, dir)
	if _, err := s.Store.Update(func(p *core.Project) error {
		p.Stage = core.StageBuild
		p.Tasks = []core.Task{
			{ID: "TASK-001", Title: "a", Area: "api", Files: []string{"app/task_001.go", "app/task_001_test.go"}, AcceptanceCriteria: []string{"ok"}, Status: core.TaskTodo},
			{ID: "TASK-002", Title: "b", Area: "api", Files: []string{"app/task_002.go", "app/task_002_test.go"}, AcceptanceCriteria: []string{"ok"}, Status: core.TaskTodo},
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.StartBuild(); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p := load(t, s)
	if !strings.Contains(strings.Join(logTexts(p), "\n"), "Konflik") {
		t.Fatalf("no conflict happened: %v", logTexts(p))
	}
	for _, tk := range p.Tasks {
		if tk.Status != core.TaskDone || tk.Attempts != 1 {
			t.Fatalf("%s = %s attempts %d notes %v", tk.ID, tk.Status, tk.Attempts, tk.Notes)
		}
	}
}
