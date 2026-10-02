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

// An edit to a file another pending task owns is rejected and never reaches
// the project tree.
func TestIsolatedRejectsForeignFile(t *testing.T) {
	t.Setenv("FOUNDRY_AGENTS", "1")
	s := newService(t, fakeAI{}, "cross")
	dir := s.Store.Dir
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/k\n\ngo 1.22\n"), 0o644)
	gitRepo(t, dir)
	if _, err := s.Store.Update(func(p *core.Project) error {
		p.Stage = core.StageBuild
		p.Tasks = []core.Task{
			{ID: "TASK-001", Title: "a", Area: "api", Files: []string{"app/task_001.go", "app/task_001_test.go"}, AcceptanceCriteria: []string{"ok"}, Status: core.TaskTodo},
			{ID: "TASK-002", Title: "b", Area: "api", Files: []string{"b/y.go"}, AcceptanceCriteria: []string{"ok"}, Status: core.TaskTodo},
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
	t1 := p.Task("TASK-001")
	if t1.Status != core.TaskBlocked || !strings.Contains(strings.Join(t1.Notes, "\n"), "b/y.go (TASK-002)") {
		t.Fatalf("TASK-001 = %s %v", t1.Status, t1.Notes)
	}
	if _, err := os.Stat(filepath.Join(dir, "app", "task_001.go")); err == nil {
		t.Fatal("rejected work reached the project tree")
	}
	if p.Task("TASK-002").Status != core.TaskDone {
		t.Fatalf("TASK-002 = %s %v", p.Task("TASK-002").Status, p.Task("TASK-002").Notes)
	}
}
