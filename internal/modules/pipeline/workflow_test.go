package pipeline_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/handrawanw/foundry/internal/core"
	"github.com/handrawanw/foundry/internal/modules/pipeline"
	"github.com/handrawanw/foundry/internal/modules/spec"
	"github.com/handrawanw/foundry/internal/modules/work"
)

// Prompt → blueprint → workflow → revision → impact → finalize → export →
// reopen, with the agent.
func TestWorkflowLifecycle(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	designReady(t, s)
	if _, err := s.ApproveStack(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApproveStructure(); err != nil {
		t.Fatal(err)
	}
	if err := s.Finalize(); !errors.Is(err, pipeline.ErrIncomplete) {
		t.Fatalf("finalize without workflow err = %v", err)
	}
	if err := s.ReviseWorkflow("tambah test", nil); err == nil {
		t.Fatal("revising an empty workflow must fail")
	}
	if err := workflowReady(s); err != nil {
		t.Fatal(err)
	}
	p := load(t, s)
	w := p.Workflow
	if len(w.Commands) != 3 || w.Commands[1].ID != "CMD-002" || !slices.Equal(w.Commands[1].DependsOn, []string{"CMD-001"}) ||
		w.Commands[0].Artifacts[0].Hash == "" || w.Version != 1 {
		t.Fatalf("workflow = %+v", w)
	}
	r := pipeline.CheckWorkflow(p)
	if !r.Ready || len(r.Waves) != 2 || !slices.Equal(r.Waves[1], []string{"CMD-002", "CMD-003"}) {
		t.Fatalf("report = %+v", r)
	}
	if err := s.GenerateWorkflow(); !errors.Is(err, pipeline.ErrWrongStage) {
		t.Fatalf("generate twice err = %v", err)
	}

	// A revision changes only what it returns; new commands get new ids.
	if err := s.ReviseWorkflow("tambahkan test integrasi", []string{"CMD-002"}); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p = load(t, s)
	w = p.Workflow
	if len(w.Commands) != 4 || w.Commands[1].Name != "Backend janji temu + notifikasi" || w.Commands[0].Name != "Kontrak API janji temu" ||
		w.Commands[3].ID != "CMD-004" || !slices.Equal(w.Commands[3].DependsOn, []string{"CMD-002", "CMD-003"}) || w.Request != nil || w.Version != 2 {
		t.Fatalf("revised = %+v", w.Commands)
	}

	// A manual edit that makes a cycle is refused as a whole.
	edit := work.CommandEdit{Name: "Kontrak", Objective: "o", Instructions: "i", DependsOn: []string{"CMD-004"}, Artifacts: []string{"requirements"}}
	if _, err := s.EditCommand("CMD-001", edit); err == nil || !strings.Contains(err.Error(), "melingkar") {
		t.Fatalf("cycle edit err = %v", err)
	}
	edit.DependsOn = []string{"CMD-009"}
	if _, err := s.EditCommand("CMD-001", edit); err == nil {
		t.Fatal("unknown dependency accepted")
	}
	if load(t, s).Workflow.Commands[0].Name != "Kontrak API janji temu" {
		t.Fatal("refused edit changed the command")
	}

	// A design change makes the commands that follow the layout stale.
	l := *load(t, s).UI.Layout
	l.Pages = slices.Clone(l.Pages)
	l.Pages[0].Name = "Pilih Dokter"
	if _, err := s.EditLayout(l); err != nil {
		t.Fatal(err)
	}
	r = pipeline.CheckWorkflow(load(t, s))
	if r.Ready || !slices.Equal(r.Stale["CMD-003"], []string{core.ArtifactLayoutSpec}) || len(r.Stale["CMD-001"]) != 0 {
		t.Fatalf("stale = %+v", r.Stale)
	}
	if err := s.Finalize(); !errors.Is(err, pipeline.ErrIncomplete) {
		t.Fatalf("finalize with stale command err = %v", err)
	}
	if _, err := s.ReviewCommand("CMD-003", false, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReviewCommand("CMD-001", true, "cek ulang kontrak"); err != nil {
		t.Fatal(err)
	}
	if r = pipeline.CheckWorkflow(load(t, s)); r.Review["CMD-001"] != "cek ulang kontrak" || r.Ready {
		t.Fatalf("review = %+v", r.Review)
	}
	if _, err := s.ReviewCommand("CMD-001", false, ""); err != nil {
		t.Fatal(err)
	}

	// Blueprint revision through the AI adds a project-specific artifact.
	if err := s.ReviseArtifacts("tambahkan threat model", nil); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p = load(t, s)
	if len(p.Artifacts) != 2 || p.Artifacts[0].ID != "threat-model" || p.Artifacts[0].Version != 2 || p.Artifacts[0].Area != core.AreaSecurity ||
		!slices.Equal(p.Artifacts[0].Refs, []string{"requirements"}) || p.ArtifactRequest != nil {
		t.Fatalf("artifacts = %+v", p.Artifacts)
	}
	if _, err := s.EditArtifact("requirements", core.Artifact{Title: "x", Body: "y"}); err == nil {
		t.Fatal("built-in artifact edited")
	}

	if err := s.Finalize(); err != nil {
		t.Fatal(err)
	}
	p = load(t, s)
	if p.Stage != core.StageReady || p.Workflow.Status != core.CommandFinalized || p.Workflow.Commands[0].Status != core.CommandFinalized {
		t.Fatalf("finalized = %s %+v", p.Stage, p.Workflow)
	}
	root := filepath.Join(p.Dir, "docs", "foundry")
	var m spec.Manifest
	raw, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	if err != nil || json.Unmarshal(raw, &m) != nil {
		t.Fatalf("manifest = %s, %v", raw, err)
	}
	if m.Status != "finalized" || len(m.Commands) != 4 || m.Commands[3].Wave != 3 || m.FinalizedAt == "" ||
		!slices.ContainsFunc(m.Artifacts, func(a spec.ManifestArtifact) bool { return a.ID == "threat-model" }) {
		t.Fatalf("manifest = %+v", m)
	}
	cmd, err := os.ReadFile(filepath.Join(root, "workflow", "CMD-004.md"))
	if err != nil || !strings.Contains(string(cmd), "Bergantung pada: CMD-002, CMD-003") || !strings.Contains(string(cmd), "- [ ] Alur buat janji lulus") {
		t.Fatalf("CMD-004.md = %s, %v", cmd, err)
	}
	if _, err := os.Stat(filepath.Join(p.Dir, "AGENTS.md")); err != nil {
		t.Fatalf("AGENTS.md: %v", err)
	}

	// Finalized: no change until reopened.
	if _, err := s.ReviewCommand("CMD-001", true, ""); !errors.Is(err, pipeline.ErrFinalized) {
		t.Fatalf("edit after finalize err = %v", err)
	}
	if err := s.DeleteDocument("threat-model"); !errors.Is(err, pipeline.ErrFinalized) {
		t.Fatalf("delete after finalize err = %v", err)
	}
	if !p.Platform.Approved {
		t.Fatal("finalize did not approve the platform")
	}
	targets := p.Platform.Targets
	if _, err := s.SetPlatform([]core.PlatformTarget{{Kind: core.PlatformWeb}}); !errors.Is(err, pipeline.ErrWrongStage) {
		t.Fatalf("platform change after finalize err = %v", err)
	}
	if !reflect.DeepEqual(load(t, s).Platform.Targets, targets) {
		t.Fatal("refused platform change was saved")
	}
	if err := s.Reopen(); err != nil {
		t.Fatal(err)
	}
	if p = load(t, s); p.Stage != core.StageDesign || p.Workflow.Status != core.CommandDraft || p.Workflow.Commands[0].Status != core.CommandDraft {
		t.Fatalf("reopened = %s %+v", p.Stage, p.Workflow)
	}
	// Reopening unlocks the platform for another revision.
	if p.Platform.Approved {
		t.Fatal("platform still approved after reopen")
	}
	// Removing an artifact a command never used leaves the workflow valid;
	// the next finalize drops its exported file.
	if _, err := s.RemoveArtifact("threat-model"); err != nil {
		t.Fatal(err)
	}
	// Without it the security area is missing: it must be written or
	// skipped with a reason.
	if err := s.Finalize(); err == nil || !strings.Contains(err.Error(), "Keamanan") {
		t.Fatalf("finalize without security err = %v", err)
	}
	if _, err := s.SkipArea(core.AreaSecurity, ""); err == nil {
		t.Fatal("skip without reason accepted")
	}
	if _, err := s.SkipArea(core.AreaSecurity, "Aplikasi demo tanpa data nyata"); err != nil {
		t.Fatal(err)
	}
	if err := s.Finalize(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "blueprint", "threat-model.md")); !os.IsNotExist(err) {
		t.Fatalf("stale export file kept: %v", err)
	}
	dec, err := os.ReadFile(filepath.Join(root, "blueprint", "decisions.md"))
	if err != nil || !strings.Contains(string(dec), "Aplikasi demo tanpa data nyata") {
		t.Fatalf("decisions.md = %s, %v", dec, err)
	}
}

// A refused answer (cycle) is retried once with the reason in the prompt.
func TestWorkflowCycleRetried(t *testing.T) {
	runs := filepath.Join(t.TempDir(), "runs")
	t.Setenv("FAKE_RUNS", runs)
	s := newService(t, fakeAI{}, "cycle_once")
	designReady(t, s)
	if err := s.GenerateWorkflow(); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p := load(t, s)
	mustJob(t, p, work.KindWorkflow, core.JobDone)
	raw, _ := os.ReadFile(runs)
	if n := strings.Count(string(raw), "workflow"); n != 2 || len(p.Workflow.Commands) != 3 || p.Workflow.Request != nil {
		t.Fatalf("runs=%d workflow=%+v", n, p.Workflow)
	}
}

// Software without screens or a server: no UI design, no web stack gate;
// the blueprint carries the project's own artifacts.
func TestEmbeddedProject(t *testing.T) {
	s := newService(t, fakeAI{}, "embedded")
	if _, err := s.SubmitStory("Firmware sensor suhu gudang yang mengirim data lewat MQTT"); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p := load(t, s)
	mustJob(t, p, work.KindDiscovery, core.JobDone)
	if p.HasUI() || p.UI != nil || p.Stage != core.StageDesign {
		t.Fatalf("stage=%s design=%+v", p.Stage, p.UI)
	}
	if err := s.GenerateDesign(); err == nil {
		t.Fatal("design generated for a product without screens")
	}
	if len(p.Artifacts) != 2 || p.Artifacts[0].ID != "firmware-architecture" || p.Artifacts[1].Area != core.AreaIntegration {
		t.Fatalf("artifacts = %+v", p.Artifacts)
	}
	if bp := p.Blueprint; bp != nil && (bp.Coding.API.Style != "" || len(bp.Modules) > 0) {
		t.Fatalf("web defaults on firmware: %+v", bp)
	}
	if err := workflowReady(s); err != nil {
		t.Fatal(err)
	}
	r := pipeline.CheckWorkflow(load(t, s))
	if !r.Ready || len(r.Waves) != 2 || len(r.Waves[0]) != 2 {
		t.Fatalf("report = %+v", r)
	}
	if b := pipeline.Blockers(load(t, s)); len(b) != 0 {
		t.Fatalf("blockers = %v", b)
	}
	if err := s.Finalize(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(s.Store.Dir, "docs", "foundry", "blueprint", "firmware-architecture.md")); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"design-brief.md", "blueprint/design-rules.md", "blueprint/layout-spec.md", "design"} {
		if _, err := os.Stat(filepath.Join(s.Store.Dir, "docs", "foundry", filepath.FromSlash(f))); !os.IsNotExist(err) {
			t.Fatalf("%s written without UI: %v", f, err)
		}
	}
}

// MCP host: the workflow job is handed out by next_work.
func TestWorkflowMCP(t *testing.T) {
	s := newService(t, fakeAI{mcp: true}, "")
	if _, err := s.SubmitStory("Saya ingin membuat aplikasi klinik untuk janji temu pasien"); err != nil {
		t.Fatal(err)
	}
	if err := s.SubmitWork(work.KindDiscovery, discoveryJSON); err != nil {
		t.Fatal(err)
	}
	if err := s.SubmitWork(work.KindDesign, designJSON); err != nil {
		t.Fatal(err)
	}
	if err := s.GenerateWorkflow(); err != nil {
		t.Fatal(err)
	}
	item, err := s.NextWork()
	if err != nil || item.Kind != work.KindWorkflow || !strings.Contains(item.Prompt, "workflow planner") || !strings.Contains(item.Prompt, "=== layout-spec") {
		t.Fatalf("next = %+v, %v", item, err)
	}
	if err := s.SubmitWork(work.KindWorkflow, cyclicWorkflowJSON); err == nil {
		t.Fatal("cyclic workflow accepted")
	}
	if p := load(t, s); p.Workflow == nil || p.Workflow.Request == nil || !strings.Contains(p.Workflow.Request.Rejected, "melingkar") {
		t.Fatalf("rejection not kept: %+v", p.Workflow)
	}
	if item, _ = s.NextWork(); !strings.Contains(item.Prompt, "YOUR LAST ANSWER WAS REFUSED") {
		t.Fatal("retry prompt misses the rejection")
	}
	if err := s.SubmitWork(work.KindWorkflow, workflowJSON); err != nil {
		t.Fatal(err)
	}
	if p := load(t, s); len(p.Workflow.Commands) != 3 {
		t.Fatalf("workflow = %+v", p.Workflow)
	}
}
