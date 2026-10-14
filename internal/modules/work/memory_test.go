package work

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/handrawanw/foundry/internal/core"
	"github.com/handrawanw/foundry/internal/modules/standard"
)

func memProject(t *testing.T) (*core.Project, *core.Task, *core.Task) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "modules/users"), 0o755)
	os.WriteFile(filepath.Join(dir, "modules/users/repo.go"), []byte("package users\n"+strings.Repeat("// real source line\n", 200)), 0o644)
	dep := core.Task{ID: "TASK-001", Title: "Users repo", Status: core.TaskDone, Files: []string{"modules/users"},
		AcceptanceCriteria: []string{"repo exists"}, Criteria: []core.Criterion{{Text: "repo exists", Met: true, Evidence: "modules/users/repo.go Repo interface"}}}
	cur := core.Task{ID: "TASK-002", Title: "Billing", Area: "api", DependsOn: []string{"TASK-001"}}
	p := &core.Project{Dir: dir, Tasks: []core.Task{dep, cur}}
	p.Tasks[0].Memory = RememberTask(p, &p.Tasks[0])
	return p, &p.Tasks[0], &p.Tasks[1]
}

func TestMemoryReusedWhileValid(t *testing.T) {
	p, _, cur := memProject(t)
	got := DependencyMemory(p, cur)
	if !strings.Contains(got, "Repo interface") || strings.Contains(got, "changed since") {
		t.Fatalf("valid memory should be reused:\n%s", got)
	}
	if !strings.Contains(TaskPrompt(p, cur), "Repo interface") {
		t.Fatal("task prompt must carry dependency memory")
	}
}

func TestMemoryInvalidatedBySourceChange(t *testing.T) {
	p, dep, cur := memProject(t)
	os.WriteFile(filepath.Join(p.Dir, "modules/users/repo.go"), []byte("package users // edited"), 0o644)
	got := DependencyMemory(p, cur)
	if !strings.Contains(got, "changed since") || strings.Contains(got, "Repo interface") || !strings.Contains(got, "repo.go") {
		t.Fatalf("edited source must drop the summary:\n%s", got)
	}
	if len(MemoryStale(p, dep)) == 0 {
		t.Fatal("stale expected")
	}
}

func TestMemoryInvalidatedByNewFileAndContract(t *testing.T) {
	p, dep, _ := memProject(t)
	os.WriteFile(filepath.Join(p.Dir, "modules/users/extra.go"), []byte("package users"), 0o644)
	if len(MemoryStale(p, dep)) == 0 {
		t.Fatal("new file in owned dir must invalidate")
	}
	p, dep, _ = memProject(t)
	dep.AcceptanceCriteria = append(dep.AcceptanceCriteria, "new rule")
	if why := MemoryStale(p, dep); len(why) != 1 || why[0] != "contract" {
		t.Fatalf("criteria change must invalidate via contract: %v", why)
	}
	p, dep, _ = memProject(t)
	p.Blueprint = &core.Blueprint{Architecture: core.ArchDecision{Pattern: "changed"}}
	if len(MemoryStale(p, dep)) == 0 {
		t.Fatal("blueprint change must invalidate")
	}
}

func TestMemorySkipsUnfinishedDependency(t *testing.T) {
	p, dep, cur := memProject(t)
	dep.Status = core.TaskTodo
	if DependencyMemory(p, cur) != "" {
		t.Fatal("only DONE dependencies are remembered")
	}
}

func TestMemorySkippedWhenNotWorthIt(t *testing.T) {
	p, dep, cur := memProject(t)
	// Tiny source: reading it is cheaper than a summary.
	os.WriteFile(filepath.Join(p.Dir, "modules/users/repo.go"), []byte("package users"), 0o644)
	dep.Memory = RememberTask(p, dep)
	if got := DependencyMemory(p, cur); got != "" {
		t.Fatalf("tiny source must not get a memory block:\n%s", got)
	}
	// Empty summary: nothing to say.
	p, dep, cur = memProject(t)
	dep.Memory.Summary = ""
	if DependencyMemory(p, cur) != "" {
		t.Fatal("empty summary adds nothing")
	}
	// Summary not much smaller than the source it replaces.
	p, dep, cur = memProject(t)
	dep.Memory.Summary = strings.Repeat("x", dep.Memory.SourceBytes/2)
	if DependencyMemory(p, cur) != "" {
		t.Fatal("summary must be well below source size")
	}
}

func TestMemoryContributionIsMeasured(t *testing.T) {
	p, dep, cur := memProject(t)
	sizes := MeasurePrompts(p)
	var s PromptSize
	for _, x := range sizes {
		if x.TaskID == cur.ID {
			s = x
		}
	}
	if s.Memory == 0 || s.MemorySource != dep.Memory.SourceBytes || s.MemorySource <= 4*s.Memory {
		t.Fatalf("memory metric = %+v (source %d)", s, dep.Memory.SourceBytes)
	}
	if s.Baseline != s.Current-s.Memory-len(TaskArchitectureBrief(p, cur))+len(ArchitectureBrief(p))-len(standard.RulesFor(ruleScope(cur)))+len(standard.Rules) {
		t.Fatal("baseline must drop the memory block")
	}
}

func TestMemoryBlockIsCapped(t *testing.T) {
	p, dep, cur := memProject(t)
	var ids []string
	for i := 0; i < 6; i++ {
		d := *dep
		d.ID = "TASK-1" + string(rune('0'+i))
		d.Memory = &core.TaskMemory{Summary: strings.Repeat("s", 400), SourceBytes: 100000, Files: dep.Memory.Files, Contract: dep.Memory.Contract}
		p.Tasks = append(p.Tasks, d)
		ids = append(ids, d.ID)
	}
	cur = p.Task("TASK-002") // appends moved the slice
	cur.DependsOn = append(cur.DependsOn, ids...)
	got := DependencyMemory(p, cur)
	if len(got) < 800 || len(got) > maxMemoryBlock+200 {
		t.Fatalf("block %d chars: want non-trivial but capped at %d", len(got), maxMemoryBlock)
	}
}
