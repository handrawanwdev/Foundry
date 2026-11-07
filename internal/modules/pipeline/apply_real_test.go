package pipeline_test

import (
	"os"
	"os/exec"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/pipeline"
	"github.com/handrawanwdev/foundry/internal/modules/structure"
	"github.com/handrawanwdev/foundry/internal/modules/work"
)

func TestApplyRealDistribusiOSJSON(t *testing.T) {
	raw, err := os.ReadFile("/tmp/foundry-plan-fail/bad1.txt")
	if err != nil {
		t.Skip(err)
	}
	src := "/home/handrawan/Project/distrub-project"
	if _, err := os.Stat(src + "/.foundry/project.json"); err != nil {
		t.Skip(err)
	}
	dir := os.Getenv("FOUNDRY_PROOF_DIR")
	if dir == "" {
		dir = t.TempDir()
		if err := exec.Command("cp", "-a", src+"/.foundry", dir+"/.foundry").Run(); err != nil {
			t.Fatal(err)
		}
	}
	st, err := core.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Update(func(p *core.Project) error {
		if p.Stack == nil {
			p.Stack = &core.TechStack{}
		}
		p.Stack.PlanRequest = &core.TechPlanRequest{Action: core.TechPlanGenerate}
		if p.Structure == nil {
			p.Structure = &core.Structure{Mode: core.StructureNew}
		}
		p.Structure.PlanRequest = &core.StructurePlanRequest{SourceHash: structure.SourceHash(p)}
		p.Structure.RepoRequest = &core.RepoRequest{Action: core.RepoGenerate, SourceHash: structure.SourceHash(p)}
		p.Job = &core.Job{Kind: work.KindPlan, Status: core.JobWaiting, Message: "Merancang Blueprint"}
		p.BlueprintQueue = []string{work.KindPlan}
		p.BlueprintBatch = 1
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	s := pipeline.New(st, fakeAI{mcp: true})
	if err := s.SubmitWork(work.KindPlan, string(raw)); err != nil {
		t.Fatalf("submit: %v", err)
	}
	p := load(t, s)
	if p.Stack == nil || p.Stack.Plan == nil {
		t.Fatalf("tech not landed job=%+v", p.Job)
	}
	if p.Structure == nil || p.Structure.Plan == nil || p.Structure.Repo == nil {
		t.Fatalf("arch/repo not landed: %+v", p.Structure)
	}
	t.Logf("landed items=%d alts=%d nodes=%d job=%s", len(p.Stack.Plan.Items), len(p.Structure.Plan.Alternatives), len(p.Structure.Repo.Nodes), p.Job.Status)
}
