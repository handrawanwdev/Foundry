package work

import (
	"os"
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
)

func unifiedProject() *core.Project {
	p := planProject()
	p.Stack = &core.TechStack{
		Mode:        core.StructureNew,
		PlanRequest: &core.TechPlanRequest{Action: core.TechPlanGenerate},
	}
	p.Structure.RepoRequest = &core.RepoRequest{Action: core.RepoGenerate}
	return p
}

func TestUnifiedPlanPromptGreenfield(t *testing.T) {
	p := unifiedProject()
	got := UnifiedPlanPrompt(p)
	for _, want := range []string{
		"unified blueprint architect",
		"SECTIONS TO PRODUCE: technology\n",
		`"technology"`, "sequential rounds", "This round: technology only",
		"booking", "TARGET PLATFORM",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("prompt misses %q", want)
		}
	}
	if strings.Contains(got, `"architecture": {`) || strings.Contains(got, `"repository": {`) {
		t.Fatal("first round asked for later sections")
	}
	if strings.Contains(got, "technology architect of Foundry") || strings.Contains(got, "repository architect of Foundry") {
		t.Fatal("unified prompt still looks like a standalone architect job")
	}
}

func TestUnifiedPlanPromptRoundsInOrder(t *testing.T) {
	p := unifiedProject()
	p.Stack.PlanRequest = nil
	p.Stack.Plan = &core.TechPlan{Approach: "Landed", Items: []core.TechItem{{Area: core.TechBackend, Technology: "Go"}}}
	got := UnifiedPlanPrompt(p)
	for _, want := range []string{"SECTIONS TO PRODUCE: architecture\n", "FROZEN TECHNOLOGY", `"architecture": {`} {
		if !strings.Contains(got, want) {
			t.Fatalf("architecture round misses %q", want)
		}
	}
	if strings.Contains(got, `"repository": {`) || strings.Contains(got, `"technology": {`) {
		t.Fatal("architecture round carries other sections")
	}
	p.Structure.PlanRequest = nil
	p.Structure.Plan = &core.StructurePlan{}
	got = UnifiedPlanPrompt(p)
	if !strings.Contains(got, "SECTIONS TO PRODUCE: repository\n") || strings.Contains(got, "sequential rounds") {
		t.Fatal("last round must ask repository only")
	}
}

func TestUnifiedPlanPromptExisting(t *testing.T) {
	p := unifiedProject()
	p.Intent = core.IntentExisting
	p.Structure.Mode = core.StructureExisting
	p.Structure.PlanRequest = nil
	p.Structure.RepoRequest = nil
	p.Structure.Context = &core.ProjectContext{
		Layout: []core.ContextFinding{{Kind: "layout", Text: "apps/", Path: "apps", Confidence: core.ConfidenceObserved}},
	}
	got := UnifiedPlanPrompt(p)
	for _, want := range []string{"EXISTING PROJECT", "reuse", "adaptation", "SECTIONS TO PRODUCE: technology"} {
		if !strings.Contains(got, want) {
			t.Fatalf("existing prompt misses %q", want)
		}
	}
	if strings.Contains(got, `"architecture"`) || strings.Contains(got, `"repository"`) {
		t.Fatal("existing unified prompt asked for greenfield architecture/repository")
	}
}

func TestUnifiedPlanPromptOmitsFrozen(t *testing.T) {
	p := unifiedProject()
	p.Stack.PlanRequest = nil
	p.Stack.Plan = &core.TechPlan{Approach: "Frozen stack", Summary: "Sudah diputuskan", Items: []core.TechItem{
		{Area: core.TechBackend, Technology: "Go"},
	}}
	got := UnifiedPlanPrompt(p)
	if !strings.Contains(got, "FROZEN TECHNOLOGY") || !strings.Contains(got, "Go") {
		t.Fatal("missing frozen technology")
	}
	if strings.Contains(got, "SECTIONS TO PRODUCE: technology") {
		t.Fatal("asked to reproduce frozen technology")
	}
}

func TestParseUnified(t *testing.T) {
	raw := "```json\n{\"technology\":{\"items\":[{\"area\":\"backend\"}]},\"architecture\":{\"alternatives\":[{}]},\"adaptation\":[{\"action\":\"reuse\",\"target\":\"apps/\"}]}\n```"
	ans, err := ParseUnified(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !ans.has(KindTech) || !ans.has(KindStructure) || len(ans.Adaptation) != 1 {
		t.Fatalf("parsed = %+v", ans)
	}
}

func TestParseUnifiedTruncatedWrapper(t *testing.T) {
	raw := "```json\n{\"technology\":{\"items\":[{\"area\":\"backend\",\"technology\":\"Go\"}],\"approach\":\"Modul\""
	ans, err := ParseUnified(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !ans.has(KindTech) {
		t.Fatalf("truncated wrapper not repaired: %+v", ans)
	}
}

func TestParseUnifiedSeparateBlocks(t *testing.T) {
	raw := "```json\n{\"items\":[{\"area\":\"backend\",\"technology\":\"Go\"}],\"approach\":\"Modul\"}\n```\n" +
		"```json\n{\"alternatives\":[{\"id\":\"recommended\",\"name\":\"Modular\"}]}\n```\n" +
		"```json\n{\"nodes\":[{\"path\":\"apps/\",\"type\":\"dir\"}],\"approach\":\"Modul per domain\"}\n```"
	ans, err := ParseUnified(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !ans.has(KindTech) || !ans.has(KindStructure) || !ans.has(KindRepo) {
		t.Fatalf("separate blocks = tech=%t arch=%t repo=%t", ans.has(KindTech), ans.has(KindStructure), ans.has(KindRepo))
	}
}

func TestUnifiedPlanPromptAcceptsSplitJSON(t *testing.T) {
	got := UnifiedPlanPrompt(unifiedProject())
	if !strings.Contains(got, "one complete json block per section") {
		t.Fatal("prompt does not tell the model split JSON is accepted")
	}
}

func TestParseUnifiedRepairsLLMSyntax(t *testing.T) {
	raw := "```json\n" + `{
  "technology": {"approach":"Modul","summary":"s","items":[{"area":"backend","technology":"Go","impact":"x.")"}]},
  "architecture": {"summary":"s","alternatives":[{"id":"recommended","name":"Modular","ir":{"type":"modular"}}]},
  "repository": {"approach":"Modul","nodes":[{"path":"apps/","type":"dir"}],"evaluation":{"summary":"ok"}}
}` + "\n```"
	ans, err := ParseUnified(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !ans.Has(KindTech) || !ans.Has(KindStructure) || !ans.Has(KindRepo) {
		t.Fatalf("repaired = tech=%t arch=%t repo=%t", ans.Has(KindTech), ans.Has(KindStructure), ans.Has(KindRepo))
	}
}

func TestParseUnifiedDiagnosesEmpty(t *testing.T) {
	_, err := ParseUnified("   ")
	if err == nil || !strings.Contains(err.Error(), "kosong") {
		t.Fatalf("err = %v", err)
	}
}

func TestUnifiedRepairPromptUsesPreviousOutput(t *testing.T) {
	p := unifiedProject()
	p.Dir = t.TempDir()
	rawDir := p.Dir + "/.foundry/ai-raw"
	if err := os.MkdirAll(rawDir, 0o700); err != nil {
		t.Fatal(err)
	}
	broken := `{"technology":{"items":[{"area":"backend"}]}}`
	if err := os.WriteFile(rawDir+"/plan.txt", []byte(broken), 0o600); err != nil {
		t.Fatal(err)
	}
	p.Stack.PlanRequest.Rejected = "sintaks rusak di baris 1"
	got := UnifiedPlanPrompt(p)
	if !strings.Contains(got, "Repair JSON syntax only") || !strings.Contains(got, broken) {
		t.Fatalf("repair prompt missing previous output: %s", got[:min(len(got), 400)])
	}
	if strings.Contains(got, "SECTIONS TO PRODUCE: technology, architecture, repository") && strings.Contains(got, `"technology": { "approach"`) {
		t.Fatal("repair prompt still embeds the full schema example")
	}
}
