package pipeline_test

import (
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

// runs counts the agent runs per label recorded in $FAKE_RUNS.
func runs(t *testing.T, path string) map[string]int {
	t.Helper()
	raw, _ := os.ReadFile(path)
	out := map[string]int{}
	for _, l := range strings.Fields(string(raw)) {
		out[l]++
	}
	return out
}

func hasDoc(p *core.Project, id string) bool {
	return slices.ContainsFunc(spec.Artifacts(p), func(a core.Artifact) bool { return a.ID == id })
}

// dataReferenced makes the threat model and CMD-002 refer to the data
// lifecycle artifact written in discovery.
func dataReferenced(t *testing.T, s *pipeline.Service) {
	t.Helper()
	if _, err := s.EditArtifact("threat-model", core.Artifact{Title: "Threat Model", Category: core.ArtifactQuality, Area: core.AreaSecurity,
		Body: "## Aset\n\n- Data pasien\n- Lihat siklus data", Refs: []string{"requirements", "siklus-data"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.EditCommand("CMD-002", work.CommandEdit{Name: "Backend janji temu", Objective: "Endpoint janji",
		Instructions: "Implementasikan endpoint; simpan sesuai siklus data.", DependsOn: []string{"CMD-001"},
		Artifacts: []string{"architecture", "siklus-data"}}); err != nil {
		t.Fatal(err)
	}
}

// Deleting the layout takes the design rules with it; the screen command
// keeps its other document, is marked for review and rewritten by the AI.
func TestDeleteDesignReleasesAndCleansCommands(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runs")
	t.Setenv("FAKE_RUNS", path)
	s := newService(t, fakeAI{}, "")
	designReady(t, s)
	if err := workflowReady(s); err != nil {
		t.Fatal(err)
	}
	before := load(t, s)

	im, err := s.DeleteImpact(core.ArtifactLayoutSpec)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(im.Docs, []string{core.ArtifactDesignRules, core.ArtifactLayoutSpec}) || !im.AI || len(im.Artifacts) != 0 ||
		len(im.Commands) != 1 || im.Commands[0].ID != "CMD-003" || im.Commands[0].Fate != pipeline.FateReview ||
		!slices.Contains(im.Areas, "Aturan Desain & Spesifikasi Layout") || len(im.Uncovered) != 0 || !strings.Contains(im.Erases, "3 halaman") {
		t.Fatalf("impact = %+v", im)
	}
	// The preview changes nothing.
	if after := load(t, s); after.UI == nil || !reflect.DeepEqual(after.Workflow, before.Workflow) || !reflect.DeepEqual(after.Artifacts, before.Artifacts) {
		t.Fatal("preview changed the project")
	}

	if err := s.DeleteDocument(core.ArtifactLayoutSpec); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p := load(t, s)
	mustJob(t, p, work.KindWorkflow, core.JobDone)
	if p.UI != nil || hasDoc(p, core.ArtifactDesignRules) || hasDoc(p, core.ArtifactLayoutSpec) {
		t.Fatalf("design still there: %+v", p.UI)
	}
	c := p.Workflow.Command("CMD-003")
	if c == nil || c.Status != core.CommandDraft || c.Review != "" || len(c.Artifacts) != 1 || c.Artifacts[0].ID != "requirements" || c.Artifacts[0].Hash == "" {
		t.Fatalf("CMD-003 = %+v", c)
	}
	if !reflect.DeepEqual(p.Workflow.Command("CMD-001"), before.Workflow.Command("CMD-001")) {
		t.Fatal("an unaffected command changed")
	}
	if r := pipeline.CheckWorkflow(p); len(r.Issues) > 0 || len(r.Review) > 0 || p.Workflow.Request != nil {
		t.Fatalf("workflow after cleanup = %+v, request %+v", r, p.Workflow.Request)
	}
	if n := runs(t, path); n["workflow"] != 2 || n["blueprint"] != 0 {
		t.Fatalf("runs = %v", n)
	}
}

// A document other documents refer to: the artifact cleanup runs first and
// hands over to the workflow cleanup.
func TestDeleteArtifactChainsArtifactThenWorkflowCleanup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runs")
	t.Setenv("FAKE_RUNS", path)
	s := newService(t, fakeAI{}, "")
	designReady(t, s)
	if err := workflowReady(s); err != nil {
		t.Fatal(err)
	}
	dataReferenced(t, s)

	im, err := s.DeleteImpact("siklus-data")
	if err != nil {
		t.Fatal(err)
	}
	if len(im.Artifacts) != 1 || im.Artifacts[0].ID != "threat-model" || len(im.Commands) != 1 || im.Commands[0].ID != "CMD-002" ||
		im.Commands[0].Fate != pipeline.FateReview || !slices.Contains(im.Areas, "Arsitektur & Siklus Hidup Data") {
		t.Fatalf("impact = %+v", im)
	}
	if err := s.DeleteDocument("siklus-data"); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p := load(t, s)
	mustJob(t, p, work.KindWorkflow, core.JobDone)
	if hasDoc(p, "siklus-data") || p.ArtifactRequest != nil || p.Workflow.Request != nil {
		t.Fatalf("after delete: request %+v / %+v", p.ArtifactRequest, p.Workflow.Request)
	}
	tm := p.Artifacts[slices.IndexFunc(p.Artifacts, func(a core.Artifact) bool { return a.ID == "threat-model" })]
	if !slices.Equal(tm.Refs, []string{"requirements"}) || tm.Summary != "Ancaman utama tanpa siklus data" {
		t.Fatalf("threat model = %+v", tm)
	}
	if c := p.Workflow.Command("CMD-002"); c.Status != core.CommandDraft || len(c.Artifacts) != 1 || c.Artifacts[0].ID != "architecture" {
		t.Fatalf("CMD-002 = %+v", c)
	}
	if n := runs(t, path); n["blueprint"] != 1 || n["workflow"] != 2 {
		t.Fatalf("runs = %v", n)
	}
	// The data area is now missing: finalize asks to write or skip it.
	if b := pipeline.Blockers(p); !slices.ContainsFunc(b, func(s string) bool { return strings.Contains(s, "Siklus Hidup Data") }) {
		t.Fatalf("blockers = %v", b)
	}
}

// A failed cleanup keeps the deletion; dismissing the failure drops both
// cleanup requests so no stale workflow request is left behind.
func TestDeleteCleanupFailureKeepsDeletion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runs")
	t.Setenv("FAKE_RUNS", path)
	s := newService(t, fakeAI{}, "cleanup_fail")
	designReady(t, s)
	if err := workflowReady(s); err != nil {
		t.Fatal(err)
	}
	dataReferenced(t, s)
	if err := s.DeleteDocument("siklus-data"); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p := load(t, s)
	mustJob(t, p, work.KindArtifacts, core.JobFailed)
	if hasDoc(p, "siklus-data") || p.Workflow.Command("CMD-002").Status != core.CommandNeedsReview {
		t.Fatal("deletion not kept after a failed cleanup")
	}
	if p.ArtifactRequest == nil || !p.ArtifactRequest.Cascade || p.Workflow.Request == nil || !p.Workflow.Request.Cascade {
		t.Fatalf("requests = %+v / %+v", p.ArtifactRequest, p.Workflow.Request)
	}
	if r := pipeline.CheckWorkflow(p); len(r.Issues) > 0 {
		t.Fatalf("workflow invalid after a failed cleanup: %+v", r.Issues)
	}
	if n := runs(t, path); n["blueprint"] != 2 || n["workflow"] != 1 { // one retry, no workflow cleanup
		t.Fatalf("runs = %v", n)
	}
	if err := s.DismissJob(); err != nil {
		t.Fatal(err)
	}
	if p = load(t, s); p.Job != nil || p.ArtifactRequest != nil || p.Workflow.Request != nil {
		t.Fatalf("after dismiss: job %+v, requests %+v / %+v", p.Job, p.ArtifactRequest, p.Workflow.Request)
	}
}

// With an MCP host the cleanup jobs wait for it and still chain.
func TestDeleteCleanupThroughMCP(t *testing.T) {
	s := newService(t, fakeAI{mcp: true}, "")
	if _, err := s.SubmitStory("Saya ingin membuat aplikasi klinik untuk janji temu pasien"); err != nil {
		t.Fatal(err)
	}
	for _, w := range []struct{ kind, raw string }{{work.KindDiscovery, discoveryJSON}, {work.KindDesign, designJSON}} {
		if err := s.SubmitWork(w.kind, w.raw); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.GenerateWorkflow(); err != nil {
		t.Fatal(err)
	}
	if err := s.SubmitWork(work.KindWorkflow, workflowJSON); err != nil {
		t.Fatal(err)
	}
	dataReferenced(t, s)
	if err := s.DeleteDocument("siklus-data"); err != nil {
		t.Fatal(err)
	}
	item, err := s.NextWork()
	if err != nil || item.Kind != work.KindArtifacts || !strings.Contains(item.Prompt, "dihapus dari blueprint") || strings.Contains(item.Prompt, "=== siklus-data") {
		t.Fatalf("next = %+v, %v", item.Kind, err)
	}
	if err := s.SubmitWork(work.KindArtifacts, cleanupArtifactsJSON); err != nil {
		t.Fatal(err)
	}
	if item, err = s.NextWork(); err != nil || item.Kind != work.KindWorkflow || !strings.Contains(item.Prompt, "Commands the request is about: CMD-002.") {
		t.Fatalf("next = %+v, %v", item.Kind, err)
	}
	if err := s.SubmitWork(work.KindWorkflow, cleanupWorkflowJSON(item.Prompt)); err != nil {
		t.Fatal(err)
	}
	if p := load(t, s); p.Workflow.Command("CMD-002").Status != core.CommandDraft || p.Workflow.Request != nil || p.ArtifactRequest != nil {
		t.Fatalf("workflow = %+v", p.Workflow)
	}
}

func TestDeleteDocumentRefused(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	if _, err := s.DeleteImpact("threat-model"); !errors.Is(err, pipeline.ErrWrongStage) {
		t.Fatalf("impact before discovery err = %v", err)
	}
	if err := s.DeleteDocument("threat-model"); !errors.Is(err, pipeline.ErrWrongStage) {
		t.Fatalf("delete before discovery err = %v", err)
	}
	designReady(t, s)
	if err := workflowReady(s); err != nil {
		t.Fatal(err)
	}
	before := load(t, s)
	if !hasDoc(before, core.ArtifactSurvey) {
		t.Fatal("survey document missing")
	}
	for _, id := range []string{"requirements", "architecture", "technology", core.ArtifactDecisions, core.ArtifactSurvey, "tidak-ada", ""} {
		if err := s.DeleteDocument(id); err == nil {
			t.Fatalf("%q deleted", id)
		}
		if _, err := s.DeleteImpact(id); err == nil {
			t.Fatalf("%q previewed", id)
		}
	}
	if p := load(t, s); !reflect.DeepEqual(spec.Artifacts(p), spec.Artifacts(before)) || !reflect.DeepEqual(p.Workflow, before.Workflow) {
		t.Fatal("a refused deletion changed the project")
	}
}

// A job waiting for the MCP host owns the project: a deletion must not
// replace it.
func TestDeleteDocumentWaitsForPendingJob(t *testing.T) {
	s := newService(t, fakeAI{mcp: true}, "")
	if _, err := s.SubmitStory("Saya ingin membuat aplikasi klinik untuk janji temu pasien"); err != nil {
		t.Fatal(err)
	}
	for _, w := range []struct{ kind, raw string }{{work.KindDiscovery, discoveryJSON}, {work.KindDesign, designJSON}} {
		if err := s.SubmitWork(w.kind, w.raw); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.GenerateWorkflow(); err != nil { // waits for the host
		t.Fatal(err)
	}
	if err := s.DeleteDocument("siklus-data"); !errors.Is(err, pipeline.ErrBusy) {
		t.Fatalf("delete while a job waits err = %v", err)
	}
	p := load(t, s)
	mustJob(t, p, work.KindWorkflow, core.JobWaiting)
	if !hasDoc(p, "siklus-data") {
		t.Fatal("document deleted while another job waits")
	}
}

func TestDeleteUseCasesAndAcceptance(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	designReady(t, s)
	im, err := s.DeleteImpact(core.ArtifactUseCases)
	if err != nil || im.Erases != "1 use case dihapus." || im.AI || !slices.Contains(im.Areas, "Alur Bisnis & Use Case") {
		t.Fatalf("use cases impact = %+v, %v", im, err)
	}
	if err := s.DeleteDocument(core.ArtifactUseCases); err != nil {
		t.Fatal(err)
	}
	if p := load(t, s); len(p.Flows) != 0 || hasDoc(p, core.ArtifactUseCases) || p.Job.Kind != work.KindDesign {
		t.Fatalf("flows = %+v, job %+v", p.Flows, p.Job)
	}
	if err := s.DeleteDocument(core.ArtifactAcceptance); err != nil {
		t.Fatal(err)
	}
	p := load(t, s)
	if hasDoc(p, core.ArtifactAcceptance) || len(p.Done) != 0 || len(p.Requirements) != 2 {
		t.Fatalf("acceptance left: done %v, reqs %d", p.Done, len(p.Requirements))
	}
	for _, r := range p.Requirements {
		if len(r.Acceptance) != 0 || r.Untestable != "" {
			t.Fatalf("requirement %s keeps acceptance", r.ID)
		}
	}
	if err := s.DeleteDocument(core.ArtifactAcceptance); err == nil {
		t.Fatal("deleting an empty document twice succeeded")
	}
}
