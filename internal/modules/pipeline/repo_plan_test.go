package pipeline_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/pipeline"
	"github.com/handrawanwdev/foundry/internal/modules/repoplan"
	"github.com/handrawanwdev/foundry/internal/modules/spec"
	"github.com/handrawanwdev/foundry/internal/modules/structure"
	"github.com/handrawanwdev/foundry/internal/modules/work"
)

// promptLines reads the "- first: rest" lines under a section header of a
// prompt, returning the part before the first ": " or " (".
func promptLines(prompt, header string) []string {
	var out []string
	in := false
	for _, line := range strings.Split(prompt, "\n") {
		switch {
		case strings.HasPrefix(line, header):
			in = true
		case in && strings.HasPrefix(line, "- "):
			out = append(out, strings.TrimPrefix(line, "- "))
		case in && strings.TrimSpace(line) == "":
			return out
		}
	}
	return out
}

func promptModules(prompt string) []string {
	var out []string
	for _, l := range promptLines(prompt, "PROJECT MODULES") {
		out = append(out, strings.SplitN(l, ":", 2)[0])
	}
	return out
}

// promptParts maps ref → folder from the STRUCTURE PARTS section.
func promptParts(prompt string) map[string]string {
	out := map[string]string{}
	for _, l := range promptLines(prompt, "STRUCTURE PARTS") {
		ref, rest, _ := strings.Cut(l, " (folder in the structure decision: ")
		out[ref] = strings.TrimSuffix(rest, ")")
	}
	return out
}

func fakeEvaluation() map[string]any {
	var crit []map[string]any
	for _, c := range core.RepoCriteria {
		crit = append(crit, map[string]any{"name": c, "score": 4, "reason": "Alasan konkret untuk " + c + " pada rencana ini"})
	}
	return map[string]any{"summary": "Struktur sederhana dan jelas", "criteria": crit, "findings": []any{}}
}

func fakeConsistency() []map[string]any {
	var out []map[string]any
	for _, a := range core.RepoConsistencyAreas {
		out = append(out, map[string]any{"area": a, "status": "consistent", "note": "Sesuai"})
	}
	return out
}

// fakeRepoAnswer is the architect's answer built from the prompt itself:
// every structure part gets its folder, every project module a place, and
// the dependency rules agree with the declared dependencies. dropLast leaves
// the last module unplaced, like an AI that overlooks one.
func fakeRepoAnswer(prompt string, dropLast bool) string {
	if strings.Contains(prompt, "Revise the CURRENT REPOSITORY PLAN") {
		return fakeRepoRevision(prompt)
	}
	modules := promptModules(prompt)
	parts := promptParts(prompt)
	owner := func(ref string) string {
		kind, name, _ := strings.Cut(ref, ":")
		if (kind == "module" || kind == "service" || kind == "worker") && slices.Contains(modules, name) {
			return name
		}
		if kind == "package" {
			return core.OwnerShared
		}
		return core.OwnerPlatform
	}
	node := func(path, own, part, resp string) map[string]any {
		n := map[string]any{"path": path, "type": "dir", "owner": own, "responsibility": resp, "reason": "Dibutuhkan oleh " + resp,
			"allowed": []string{"kode " + resp}, "forbidden": []string{"kode di luar " + resp}}
		if part != "" {
			n["part"] = part
		}
		return n
	}
	var nodes []map[string]any
	var contracts string
	refs := make([]string, 0, len(parts))
	for ref := range parts {
		refs = append(refs, ref)
	}
	slices.Sort(refs)
	for _, ref := range refs {
		if strings.HasPrefix(ref, "package:contracts") {
			contracts = parts[ref]
		}
	}
	placed := map[string]string{}
	for _, ref := range refs {
		n := node(parts[ref], owner(ref), ref, "bagian "+ref)
		if contracts != "" && owner(ref) != core.OwnerShared && owner(ref) != core.OwnerPlatform {
			n["depends_on"] = []string{contracts}
		}
		if own := owner(ref); slices.Contains(modules, own) && placed[own] == "" {
			placed[own] = parts[ref]
		}
		if strings.HasPrefix(ref, "package:") {
			n["shared_by"] = modules
		}
		nodes = append(nodes, n)
	}
	var mapped []map[string]any
	for i, m := range modules {
		path := placed[m]
		if path == "" {
			path = "src/" + m + "/"
			nodes = append(nodes, node(path, m, "", "modul "+m))
		}
		if dropLast && i == len(modules)-1 && len(modules) > 1 {
			continue
		}
		mapped = append(mapped, map[string]any{"module": m, "locations": []map[string]any{{"path": path, "role": "code"}},
			"frontend": i == 0 && len(promptLines(prompt, "TARGET PLATFORM:")) > 0, "backend": true})
	}
	nodes = append(nodes, node("docs/", core.OwnerPlatform, "", "dokumentasi"))
	forbidden := []map[string]any{}
	if len(modules) > 0 {
		forbidden = append(forbidden, map[string]any{"from": "shared", "to": modules[0], "reason": "Kode bersama tidak boleh memakai modul bisnis"})
	}
	plan := map[string]any{
		"approach": "Modul per domain", "topology": "Satu repository", "summary": "Kode dibagi per domain dengan bagian bersama.",
		"rationale": "Domain jelas dan tim kecil",
		"nodes":     nodes, "modules": mapped,
		"dependency": map[string]any{"direction": "Aplikasi → modul → shared", "forbidden": forbidden,
			"allowed": []map[string]any{{"from": "platform", "to": "shared", "reason": "tooling memakai shared"}}, "cycle_guard": "Tidak ada dependensi balik antarmodul"},
		"rules":       []string{"Kode modul hanya di folder modulnya"},
		"evaluation":  fakeEvaluation(),
		"consistency": fakeConsistency(),
		"decisions":   []map[string]any{{"title": "Struktur per domain", "choice": "modul per domain", "reason": "mengikuti batas domain", "alternatives": []string{"per lapisan"}}},
	}
	raw, _ := json.Marshal(plan)
	return "```json\n" + string(raw) + "\n```"
}

// fakeRepoRevision answers a revision: it places modules the plan lacks
// (a changed source) and follows simple requests.
func fakeRepoRevision(prompt string) string {
	var ops []map[string]any
	have := map[string]bool{}
	for _, l := range promptLines(prompt, "modules (module | locations") {
		have[strings.SplitN(l, " | ", 2)[0]] = true
	}
	for _, m := range promptModules(prompt) {
		if have[m] {
			continue
		}
		path := "src/" + m + "/"
		ops = append(ops,
			map[string]any{"op": "set_node", "node": map[string]any{"path": path, "type": "dir", "owner": m, "responsibility": "modul " + m, "reason": "domain baru",
				"allowed": []string{"kode " + m}, "forbidden": []string{"kode modul lain"}}},
			map[string]any{"op": "set_module", "module": map[string]any{"module": m, "locations": []map[string]any{{"path": path}}, "backend": true}})
	}
	request := ""
	if _, after, ok := strings.Cut(prompt, "USER REQUEST:\n"); ok {
		request, _, _ = strings.Cut(after, "\n")
	}
	switch {
	case strings.Contains(request, "ADR"):
		ops = append(ops, map[string]any{"op": "set_node", "node": map[string]any{"path": "docs/adr/", "type": "dir", "owner": "platform",
			"responsibility": "catatan keputusan arsitektur", "reason": "keputusan perlu jejak", "allowed": []string{"ADR"}, "forbidden": []string{"kode"}}})
	case strings.Contains(request, "pindahkan docs"):
		ops = append(ops, map[string]any{"op": "move_node", "path": "docs/", "to": "documentation/"})
	case strings.Contains(request, "ngawur"):
		ops = append(ops, map[string]any{"op": "remove_node", "path": "tidak-ada/"})
	}
	raw, _ := json.Marshal(map[string]any{"ops": ops, "note": "Disesuaikan", "evaluation": fakeEvaluation(), "consistency": fakeConsistency()})
	return "```json\n" + string(raw) + "\n```"
}

func repoPlanOf(t *testing.T, s *pipeline.Service) *core.RepoPlan {
	t.Helper()
	p := load(t, s)
	if p.Structure == nil || p.Structure.Repo == nil {
		t.Fatalf("no repository plan; job = %+v, request = %+v", p.Job, p.Structure.RepoRequest)
	}
	return p.Structure.Repo
}

func TestRepoPlanLifecycle(t *testing.T) {
	runs := filepath.Join(t.TempDir(), "runs")
	t.Setenv("FAKE_RUNS", runs)
	s := newService(t, fakeAI{}, "")
	designReady(t, s)
	p := load(t, s)
	if len(p.Blueprint.Modules) == 0 {
		t.Fatal("fixture has no project modules")
	}

	if err := s.GenerateRepoPlan("", ""); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p = load(t, s)
	mustJob(t, p, work.KindRepo, core.JobDone)
	plan := p.Structure.Repo
	if plan == nil || p.Structure.RepoRequest != nil {
		t.Fatalf("plan = %v request = %+v", plan, p.Structure.RepoRequest)
	}
	if errs := plan.Errors(); len(errs) != 0 {
		t.Fatalf("errors = %v", errs)
	}
	if plan.State() != core.TechRecommended || plan.Version() != 1 || plan.Stale || plan.Approach != "Modul per domain" {
		t.Fatalf("state=%s version=%d stale=%v approach=%q", plan.State(), plan.Version(), plan.Stale, plan.Approach)
	}
	if plan.Evaluation.Score != 69 { // every criterion 4, complexity 4: (8*4 + 2 - 9) * 100 / 36
		t.Fatalf("score = %d", plan.Evaluation.Score)
	}
	// Every project module is placed, and every part of the structure decision.
	for _, m := range repoplan.Modules(p) {
		if !slices.ContainsFunc(plan.Modules, func(x core.RepoModuleMap) bool { return x.Module == m.ID }) {
			t.Fatalf("module %s unplaced: %+v", m.ID, plan.Modules)
		}
	}
	// A second design is refused while nothing changed.
	if err := s.GenerateRepoPlan("", ""); err == nil || !strings.Contains(err.Error(), "masih berlaku") {
		t.Fatalf("regenerate err = %v", err)
	}
	// Approval needs the structure decision approved first.
	if _, err := s.ApproveRepoPlan(); !errors.Is(err, repoplan.ErrNeedStructure) {
		t.Fatalf("approve err = %v", err)
	}
	if _, err := s.ApproveStructure(); err != nil {
		t.Fatal(err)
	}
	if p, err := s.ApproveRepoPlan(); err != nil || p.Structure.Repo.State() != core.TechApproved {
		t.Fatalf("approve: %v", err)
	}

	// A manual edit needs a reason, becomes a new version, and puts the plan
	// back under review with the AI evaluation marked outdated.
	op := core.RepoOp{Op: repoplan.OpSetNode, Node: &core.RepoNode{Path: "docs/adr/", Type: "dir", Owner: "platform", Responsibility: "ADR", Reason: "jejak keputusan",
		Allowed: []string{"ADR"}, Forbidden: []string{"kode"}}}
	if _, err := s.EditRepoPlan([]core.RepoOp{op}, " "); err == nil {
		t.Fatal("an edit without a reason was accepted")
	}
	p, err := s.EditRepoPlan([]core.RepoOp{op}, "Keputusan arsitektur harus tercatat")
	if err != nil {
		t.Fatal(err)
	}
	plan = p.Structure.Repo
	if plan.State() != core.TechChanged || !plan.Evaluation.Outdated || plan.Version() != 2 || plan.Node("docs/adr/") == nil {
		t.Fatalf("state=%s outdated=%v version=%d", plan.State(), plan.Evaluation.Outdated, plan.Version())
	}
	last := plan.History[len(plan.History)-1]
	if last.Source != "user" || last.Reason != "Keputusan arsitektur harus tercatat" || len(last.Changes) != 1 {
		t.Fatalf("history = %+v", last)
	}
	// A broken edit is refused and leaves the plan alone.
	if _, err := s.EditRepoPlan([]core.RepoOp{{Op: repoplan.OpRemoveNode, Path: "tidak-ada/"}}, "x"); err == nil {
		t.Fatal("removing a missing path was accepted")
	}
	if repoPlanOf(t, s).Version() != 2 {
		t.Fatal("a refused edit changed the plan")
	}
	if _, err := s.ApproveRepoPlan(); err != nil {
		t.Fatal(err)
	}

	// Only one agent run per request.
	raw, _ := os.ReadFile(runs)
	if strings.Count(string(raw), "repo") != 1 {
		t.Fatalf("runs = %q", raw)
	}
}

// An answer that leaves a module unplaced is refused once with the concrete
// reason, and the corrected answer is accepted.
func TestRepoPlanRetriesRefusedAnswer(t *testing.T) {
	runs := filepath.Join(t.TempDir(), "runs")
	t.Setenv("FAKE_RUNS", runs)
	s := newService(t, fakeAI{}, "repo_invalid_once")
	designReady(t, s)
	if err := s.GenerateRepoPlan("", ""); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p := load(t, s)
	if p.Structure.Repo == nil || p.Structure.RepoRequest != nil {
		t.Fatalf("not retried: plan=%v request=%+v job=%+v", p.Structure.Repo, p.Structure.RepoRequest, p.Job)
	}
	mustJob(t, p, work.KindRepo, core.JobDone)
	raw, _ := os.ReadFile(runs)
	if strings.Count(string(raw), "repo") != 2 {
		t.Fatalf("runs = %q", raw)
	}
	var refused bool
	for _, l := range p.Events {
		refused = refused || strings.Contains(l.Message, "belum punya lokasi di repository")
	}
	if !refused {
		t.Fatalf("the refusal was not logged: %+v", p.Events)
	}
}

// Sources change after the plan: the plan is stale in the affected areas
// only, cannot be approved, and the AI revision places what is new.
func TestRepoPlanStaleAndRevision(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	designReady(t, s)
	if err := s.GenerateRepoPlan("", ""); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	if _, err := s.ApproveStructure(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApproveRepoPlan(); err != nil {
		t.Fatal(err)
	}
	if err := s.ReviseRepoPlan("", ""); err == nil {
		t.Fatal("nothing changed, yet a revision without text was accepted")
	}

	// A new module appears in the blueprint.
	if _, err := s.Store.Update(func(p *core.Project) error {
		p.Blueprint.Modules = append(p.Blueprint.Modules, core.Module{ID: "reporting", Responsibility: "Laporan bulanan"})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	snap, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	plan := snap.Structure.Repo
	if !plan.Stale || !slices.Equal(plan.Analysis.StaleAreas, []string{"modules"}) || !slices.Contains(plan.Analysis.Affected, "dependency") {
		t.Fatalf("stale=%v areas=%v affected=%v", plan.Stale, plan.Analysis.StaleAreas, plan.Analysis.Affected)
	}
	if !slices.ContainsFunc(plan.Errors(), func(i core.TechIssue) bool { return strings.Contains(i.Message, "reporting") }) {
		t.Fatalf("the unplaced module is not an error: %v", plan.Errors())
	}
	if _, err := s.ApproveRepoPlan(); err == nil {
		t.Fatal("a stale plan was approved")
	}
	if b := pipeline.Blockers(load(t, s)); !slices.ContainsFunc(b, func(m string) bool { return strings.Contains(m, "repository") }) {
		t.Fatalf("blockers = %v", b)
	}

	if err := s.ReviseRepoPlan("", ""); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	plan = repoPlanOf(t, s)
	if plan.Stale || len(plan.Errors()) != 0 || plan.Node("src/reporting/") == nil || plan.Version() != 2 {
		t.Fatalf("stale=%v errors=%v version=%d nodes=%d", plan.Stale, plan.Errors(), plan.Version(), len(plan.Nodes))
	}
	if plan.State() != core.TechChanged {
		t.Fatalf("a revised approved plan must be reviewed again, state = %s", plan.State())
	}
	if last := plan.History[len(plan.History)-1]; last.Source != "ai" || !strings.Contains(strings.Join(last.Changes, ";"), "src/reporting/") {
		t.Fatalf("history = %+v", last)
	}

	// A request in words with a reason, and one that cannot apply.
	if err := s.ReviseRepoPlan("Tambahkan folder ADR untuk keputusan arsitektur", "Tim butuh jejak keputusan"); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	plan = repoPlanOf(t, s)
	if plan.Node("docs/adr/") == nil || plan.History[len(plan.History)-1].Reason != "Tim butuh jejak keputusan" || plan.Evaluation.Outdated {
		t.Fatalf("revision not applied: %+v", plan.History[len(plan.History)-1])
	}
	v := plan.Version()
	if err := s.ReviseRepoPlan("ngawur", ""); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p := load(t, s)
	if p.Structure.RepoRequest != nil || p.Job == nil || p.Job.Status != core.JobFailed || p.Structure.Repo.Version() != v {
		t.Fatalf("refused revision: job=%+v request=%+v version=%d", p.Job, p.Structure.RepoRequest, p.Structure.Repo.Version())
	}
	if err := s.DismissJob(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DiscardRepoPlan(); err != nil || load(t, s).Structure.Repo != nil {
		t.Fatalf("discard: %v", err)
	}
}

// The MCP host path: the refusal travels back in the next prompt.
func TestRepoPlanThroughMCPHost(t *testing.T) {
	s := newService(t, fakeAI{mcp: true}, "")
	if _, err := s.SubmitStory("Aplikasi klinik untuk janji temu pasien"); err != nil {
		t.Fatal(err)
	}
	mcpAfterStory(t, s)
	if err := s.SubmitWork(work.KindDiscovery, discoveryJSON); err != nil {
		t.Fatal(err)
	}
	if err := s.SubmitWork(work.KindDesign, designJSON); err != nil {
		t.Fatal(err)
	}
	holdAutoPlan(t, s)
	if err := s.GenerateRepoPlan("", ""); err != nil {
		t.Fatal(err)
	}
	item, err := s.NextWork()
	if err != nil || item.Kind != work.KindRepo || !strings.Contains(item.Prompt, "repository architect of Foundry") {
		t.Fatalf("work item = %+v, %v", item, err)
	}
	for _, want := range []string{"PROJECT MODULES", "STRUCTURE PARTS", "You are free to choose the organization approach"} {
		if !strings.Contains(item.Prompt, want) {
			t.Fatalf("prompt lacks %q", want)
		}
	}
	err = s.SubmitWork(work.KindRepo, fakeRepoAnswer(item.Prompt, true))
	if err == nil || !strings.Contains(err.Error(), "belum punya lokasi di repository") {
		t.Fatalf("submit err = %v", err)
	}
	item, _ = s.NextWork()
	if item.Kind != work.KindRepo || !strings.Contains(item.Prompt, "YOUR LAST ANSWER WAS REFUSED, fix this: rencana struktur repository ditolak") {
		t.Fatalf("the refusal is not in the next prompt: %s", item.Prompt[len(item.Prompt)-600:])
	}
	if err := s.SubmitWork(work.KindRepo, fakeRepoAnswer(item.Prompt, false)); err != nil {
		t.Fatal(err)
	}
	if plan := repoPlanOf(t, s); len(plan.Errors()) != 0 {
		t.Fatalf("errors = %v", plan.Errors())
	}
}

// The plan reaches the export as documentation an external agent follows,
// and the blueprint it replaces is not shown twice.
func TestRepoPlanExport(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	designReady(t, s)
	before := load(t, s)
	techBefore := ""
	for _, d := range spec.Artifacts(before) {
		if d.ID == "technology" {
			techBefore = d.Body
		}
		if d.ID == core.ArtifactRepoStructure {
			t.Fatal("repository artifact exists before a plan")
		}
	}
	if _, ok := spec.Files(before)["structure/repository.json"]; ok {
		t.Fatal("repository.json exported without a plan")
	}
	if err := s.GenerateRepoPlan("", ""); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p := load(t, s)
	var repoDoc, techAfter string
	for _, d := range spec.Artifacts(p) {
		switch d.ID {
		case core.ArtifactRepoStructure:
			repoDoc = d.Body
		case "technology":
			techAfter = d.Body
		}
	}
	for _, want := range []string{"## Pohon folder", "## Spesifikasi folder dan file", "## Pemetaan modul", "## Arsitektur dependency", "```mermaid",
		"## Evaluasi kualitas struktur", "### Konsistensi", "## Keputusan desain", "## Riwayat revisi", "Tidak boleh ditempatkan", "Pendekatan:** Modul per domain"} {
		if !strings.Contains(repoDoc, want) {
			t.Errorf("repository document lacks %q", want)
		}
	}
	for _, m := range repoplan.Modules(p) {
		if !strings.Contains(repoDoc, "| "+m.ID+" |") {
			t.Errorf("module %s missing from the mapping table", m.ID)
		}
	}
	if techAfter == techBefore || !strings.Contains(techAfter, core.ArtifactRepoStructure) {
		t.Fatal("the technology document does not point at the repository plan")
	}
	files := spec.Files(p)
	raw, ok := files["structure/repository.json"]
	var back core.RepoPlan
	if !ok || json.Unmarshal([]byte(raw), &back) != nil || len(back.Nodes) != len(p.Structure.Repo.Nodes) {
		t.Fatalf("repository.json = %v %q", ok, raw[:min(len(raw), 80)])
	}
	if _, ok := files["blueprint/"+core.ArtifactRepoStructure+".md"]; !ok {
		t.Fatal("repository document missing from the package")
	}
	// The workflow and blueprint prompts carry the plan as source of truth.
	if prompt := work.WorkflowPrompt(p, spec.Artifacts(p)); !strings.Contains(prompt, "REPOSITORY PLAN") {
		t.Fatal("workflow prompt does not carry the repository plan")
	}
}

// Architectures that differ from the clinic fixture: the AI is free to place
// the same project as a microservice system, and the plan follows the
// structure decision it is given.
func TestRepoPlanForOtherStructureDecisions(t *testing.T) {
	cases := map[string]*core.StructureIR{
		"microservices with a worker": {Type: "microservices",
			Apps:     []core.StructurePart{{Name: "web", Reason: "web"}},
			Services: []core.StructurePart{{Name: "alpha", Reason: "a"}, {Name: "beta", Reason: "b"}},
			Workers:  []core.StructurePart{{Name: "worker", Reason: "job"}},
			Packages: []core.StructurePart{{Name: "contracts", Reason: "kontrak"}}},
		"no backend": {Type: "simple", Apps: []core.StructurePart{{Name: "web", Reason: "web"}}},
	}
	for name, ir := range cases {
		t.Run(name, func(t *testing.T) {
			s := newService(t, fakeAI{}, "")
			designReady(t, s)
			if _, err := s.Store.Update(func(p *core.Project) error {
				p.Blueprint.Modules = []core.Module{{ID: "alpha", Responsibility: "Domain alpha"}, {ID: "beta", Responsibility: "Domain beta"}}
				structure.Normalize(ir)
				p.Structure.IR = ir
				p.Structure.SetStatus(core.TechApproved)
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if err := s.GenerateRepoPlan("", ""); err != nil {
				t.Fatal(err)
			}
			s.Wait()
			p := load(t, s)
			plan := p.Structure.Repo
			if plan == nil {
				t.Fatalf("no plan: job = %+v", p.Job)
			}
			if errs := plan.Errors(); len(errs) != 0 {
				t.Fatalf("errors = %v", errs)
			}
			for _, part := range repoplan.IRParts(ir) {
				if !slices.ContainsFunc(plan.Nodes, func(n core.RepoNode) bool { return n.Part == part.Ref }) {
					t.Errorf("part %s is not mapped", part.Ref)
				}
			}
			if _, err := s.ApproveRepoPlan(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// A plan that exists must be approved and current before the blueprint is
// finalized; without a plan nothing changes.
func TestRepoPlanGatesFinalize(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	designReady(t, s)
	if _, err := s.ApproveStack(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApproveStructure(); err != nil {
		t.Fatal(err)
	}
	if err := workflowReady(s); err != nil {
		t.Fatal(err)
	}
	if b := pipeline.Blockers(load(t, s)); len(b) != 0 {
		t.Fatalf("no plan, yet blockers = %v", b)
	}
	if err := s.GenerateRepoPlan("", ""); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	if err := s.Finalize(); err == nil || !strings.Contains(err.Error(), "rencana struktur repository belum disetujui") {
		t.Fatalf("finalize with an unapproved plan: %v", err)
	}
	if _, err := s.ApproveRepoPlan(); err != nil {
		t.Fatal(err)
	}
	// The plan changed the technology and structure documents, so the
	// commands that were written against them are flagged for review: the
	// workflow follows the plan.
	review := pipeline.CheckWorkflow(load(t, s)).Review
	if len(review) == 0 {
		t.Fatal("no command was flagged after the repository plan changed the documents")
	}
	if err := s.Finalize(); !errors.Is(err, pipeline.ErrIncomplete) {
		t.Fatalf("finalize with commands to review: %v", err)
	}
	for id := range review {
		if _, err := s.ReviewCommand(id, false, ""); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Finalize(); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(s.Store.Dir, "docs", "foundry")
	for _, f := range []string{"blueprint/repository-structure.md", "structure/repository.json"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(f))); err != nil {
			t.Errorf("export lacks %s: %v", f, err)
		}
	}
	// A finalized project cannot change its plan.
	if _, err := s.EditRepoPlan([]core.RepoOp{{Op: repoplan.OpAddRule, Text: "aturan baru"}}, "x"); !errors.Is(err, pipeline.ErrWrongStage) {
		t.Fatalf("edit after finalize: %v", err)
	}
}

// A revision the AI keeps failing is kept with the user's words: the failure
// does not ask for a "more specific" request, and "Coba lagi" runs the same
// request again instead of a fresh design without it. The request has no
// length limit, like a chat message.
func TestRepoRevisionFailureKeepsTheRequestForRetry(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	designReady(t, s)
	if err := s.GenerateRepoPlan("", ""); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	text := "Pindahkan dokumentasi ke folder handbook." + strings.Repeat(" Detail tambahan permintaan.", 1000)
	t.Setenv("FAKE_MODE", "repo_invalid")
	if err := s.ReviseRepoPlan(text, ""); err != nil {
		t.Fatalf("long request refused: %v", err)
	}
	s.Wait()
	p := load(t, s)
	mustJob(t, p, work.KindRepo, core.JobFailed)
	if strings.Contains(p.Job.Error, "lebih spesifik") || !strings.Contains(p.Job.Error, "Coba lagi") {
		t.Fatalf("error = %q", p.Job.Error)
	}
	if p.Structure.RepoRequest != nil || p.Structure.RepoRetry == nil || p.Structure.RepoRetry.Text != text {
		t.Fatalf("request not kept: req=%v retry=%v", p.Structure.RepoRequest != nil, p.Structure.RepoRetry != nil)
	}

	t.Setenv("FAKE_MODE", "")
	if err := s.RetryJob(); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p = load(t, s)
	mustJob(t, p, work.KindRepo, core.JobDone)
	h := p.Structure.Repo.History
	if p.Structure.RepoRetry != nil || len(h) < 2 || h[len(h)-1].Request != text || h[len(h)-1].Source != "ai" {
		t.Fatalf("retry did not run the request: retry=%v history=%d source=%s len=%d", p.Structure.RepoRetry != nil, len(h), h[len(h)-1].Source, len(h[len(h)-1].Request))
	}
}

// Closing the failed job drops the kept request.
func TestDismissDropsKeptRepoRequest(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	designReady(t, s)
	if err := s.GenerateRepoPlan("", ""); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	t.Setenv("FAKE_MODE", "repo_invalid")
	if err := s.ReviseRepoPlan("gabungkan docs", ""); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	if err := s.DismissJob(); err != nil {
		t.Fatal(err)
	}
	if p := load(t, s); p.Structure.RepoRetry != nil || p.Job != nil {
		t.Fatalf("kept after dismiss: %+v %+v", p.Structure.RepoRetry, p.Job)
	}
}
