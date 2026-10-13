package verify

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/handrawanw/foundry/internal/core"
)

func runnableProject(t *testing.T) *core.Project {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"scripts":{"start":"node x.js","test":"true"}}`), 0o644)
	return &core.Project{Dir: dir, Tasks: []core.Task{{ID: "TASK-001", Status: core.TaskDone}}}
}

func kinds(v *core.Verification) map[string]bool {
	m := map[string]bool{}
	for _, i := range v.Issues {
		m[i.Kind] = true
	}
	return m
}

func TestRunnableProjectNeedsSmoke(t *testing.T) {
	p := runnableProject(t)
	b := &core.BuildRun{ID: "B1", Passed: true, Steps: []core.StepResult{{Name: "build"}, {Name: "test"}}}
	v := Report(p, b)
	if !kinds(v)["no_smoke"] || v.Ready {
		t.Fatalf("issues = %+v ready=%v", v.Issues, v.Ready)
	}
	b.Steps = append(b.Steps, core.StepResult{Name: "smoke", Command: "npm run smoke", ExitCode: 1})
	v = Report(p, b)
	if !kinds(v)["smoke_failed"] || kinds(v)["no_smoke"] || v.Ready {
		t.Fatalf("issues = %+v", v.Issues)
	}
	b.Steps[2].ExitCode = 0
	v = Report(p, b)
	if kinds(v)["no_smoke"] || kinds(v)["smoke_failed"] {
		t.Fatalf("issues = %+v", v.Issues)
	}
}
