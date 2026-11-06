package work

import (
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/repoplan"
	"github.com/handrawanwdev/foundry/internal/modules/structure"
)

func repoProject() *core.Project {
	p := &core.Project{Name: "demo", Story: "cerita", Blueprint: &core.Blueprint{Modules: []core.Module{{ID: "booking", Responsibility: "pemesanan"}}},
		Structure: &core.Structure{Mode: core.StructureNew, IR: &core.StructureIR{Type: core.StructModular,
			Services: []core.StructurePart{{Name: "api", Path: "apps/api/", Modules: []string{"booking"}}}}}}
	return p
}

func TestRepoPlanPromptLeavesTheApproachToTheAI(t *testing.T) {
	prompt := RepoPlanPrompt(repoProject())
	for _, want := range []string{"You are free to choose the organization approach", "references, not a menu", "PROJECT MODULES", "- booking: pemesanan",
		"STRUCTURE PARTS", "service:api (default folder apps/api/)", "module:booking", "EVERY project module", "consistency", "evaluation",
		"FOLDERS, never files", "not a second part mapping", `"role": "ui"`} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt lacks %q", want)
		}
	}
	if strings.Contains(prompt, "choose one of") || strings.Contains(prompt, "must be one of: feature-based") {
		t.Error("the prompt restricts the approach to a catalog")
	}
}

func TestRepoRevisePromptCarriesChangedSourcesAndRefusal(t *testing.T) {
	p := repoProject()
	p.Structure.Repo = &core.RepoPlan{Approach: "A", Nodes: []core.RepoNode{{Path: "apps/", Type: core.NodeDir, Owner: "platform", Responsibility: "aplikasi"}},
		Analysis:    core.RepoAnalysis{StaleAreas: []string{"modules"}, Affected: []string{"modules", "tree", "evaluation"}},
		Evaluation:  core.RepoEvaluation{Summary: "Jelas", Criteria: []core.RepoCriterion{{Name: "cohesion", Score: 4, Reason: "domain terpisah"}}},
		Consistency: []core.RepoCheck{{Area: "project_modules", Status: core.CheckConsistent, Note: "semua ada"}}}
	p.Structure.RepoRequest = &core.RepoRequest{Action: core.RepoRevise, Text: "pisahkan billing", Reason: "deploy terpisah", Rejected: "modul x tanpa lokasi"}
	prompt := RepoRevisePrompt(p)
	for _, want := range []string{"SOURCES THAT CHANGED since the plan was written: modules", "Plan sections affected: modules, tree, evaluation",
		"USER REQUEST:\npisahkan billing", "REASON GIVEN BY THE USER:\ndeploy terpisah", "YOUR LAST ANSWER WAS REFUSED, fix this: modul x tanpa lokasi",
		"- apps/ | platform", `"op":"move_node"`, "CURRENT EVALUATION", "- cohesion: 4 — domain terpisah",
		"CURRENT CONSISTENCY", "evaluation and consistency are mandatory JSON keys"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt lacks %q", want)
		}
	}
}

func dirNode(path, owner, resp, part string) core.RepoNode {
	n := core.RepoNode{Path: path, Type: core.NodeDir, Owner: owner, Responsibility: resp, Reason: "Dibutuhkan untuk " + resp,
		Allowed: []string{"kode " + resp}, Forbidden: []string{"kode di luar " + resp}, Part: part}
	return n
}

func reviewed(plan *core.RepoPlan) {
	plan.Approach, plan.Topology, plan.Summary = "Modular per domain", "Satu repository", "Kode per domain"
	plan.Decisions = []core.RepoDecision{{Title: "Satu repo", Choice: "monorepo", Reason: "tim kecil"}}
	plan.Dependency.Direction = "aplikasi → modul → shared"
	plan.Evaluation = core.RepoEvaluation{Summary: "Sederhana", Criteria: make([]core.RepoCriterion, 0, len(core.RepoCriteria))}
	for _, c := range core.RepoCriteria {
		plan.Evaluation.Criteria = append(plan.Evaluation.Criteria, core.RepoCriterion{Name: c, Score: 4, Reason: "Alasan konkret " + c})
	}
	plan.Consistency = nil
	for _, a := range core.RepoConsistencyAreas {
		plan.Consistency = append(plan.Consistency, core.RepoCheck{Area: a, Status: core.CheckConsistent, Note: "Sesuai"})
	}
}

func reviseReady() *core.Project {
	p := repoProject()
	structure.Normalize(p.Structure.IR)
	api := dirNode("apps/api/", core.OwnerPlatform, "API", "service:api")
	booking := dirNode("apps/api/modules/booking/", "booking", "domain pemesanan", "module:booking")
	plan := &core.RepoPlan{
		Nodes: []core.RepoNode{
			dirNode("apps/", core.OwnerPlatform, "semua aplikasi", ""),
			api,
			dirNode("apps/api/modules/", core.OwnerPlatform, "modul bisnis", ""),
			booking,
			dirNode("docs/", core.OwnerPlatform, "dokumentasi", ""),
		},
		Modules: []core.RepoModuleMap{
			{Module: "booking", Locations: []core.RepoLocation{{Path: "apps/api/modules/booking/", Role: "api"}}, Backend: true},
		},
		Dependency: core.RepoDependency{
			Forbidden: []core.RepoDepRule{{From: "shared", To: "booking", Reason: "shared tidak memakai modul bisnis"}},
		},
		Status: core.TechRecommended,
	}
	reviewed(plan)
	if err := repoplan.Accept(p, plan); err != nil {
		panic(err)
	}
	p.Structure.Repo = plan
	p.Structure.RepoRequest = &core.RepoRequest{Action: core.RepoRevise, Text: "Tambahkan folder ADR", Reason: "jejak keputusan"}
	return p
}

func testEvaluation() map[string]any {
	var crit []map[string]any
	for _, c := range core.RepoCriteria {
		crit = append(crit, map[string]any{"name": c, "score": 5, "reason": "Dinilai ulang untuk " + c})
	}
	return map[string]any{"summary": "Tetap jelas", "criteria": crit, "findings": []any{}}
}

func testConsistency() []map[string]any {
	var out []map[string]any
	for _, a := range core.RepoConsistencyAreas {
		out = append(out, map[string]any{"area": a, "status": "consistent", "note": "Masih sesuai"})
	}
	return out
}

func TestApplyRepoRevisionLandsOpsAndReview(t *testing.T) {
	p := reviseReady()
	raw := mustJSON(map[string]any{
		"ops": []map[string]any{{"op": "set_node", "node": map[string]any{
			"path": "docs/adr/", "type": "dir", "owner": "platform", "responsibility": "catatan keputusan",
			"reason": "jejak arsitektur", "allowed": []string{"ADR"}, "forbidden": []string{"kode"},
		}}},
		"note":        "Folder ADR ditambahkan",
		"evaluation":  testEvaluation(),
		"consistency": testConsistency(),
	})
	if err := ApplyRepoRevision(p, raw); err != nil {
		t.Fatal(err)
	}
	plan := p.Structure.Repo
	if plan.Node("docs/adr/") == nil || p.Structure.RepoRequest != nil || plan.Evaluation.Outdated {
		t.Fatalf("plan = %+v request = %+v", plan.Node("docs/adr/"), p.Structure.RepoRequest)
	}
	if plan.Evaluation.Summary != "Tetap jelas" || plan.Evaluation.Criteria[0].Score != 5 {
		t.Fatalf("evaluation = %+v", plan.Evaluation)
	}
}

func TestApplyRepoRevisionKeepsReviewWhenAIOmitsIt(t *testing.T) {
	p := reviseReady()
	before := p.Structure.Repo.Evaluation.Summary
	raw := mustJSON(map[string]any{
		"ops": []map[string]any{{"op": "set_node", "node": map[string]any{
			"path": "docs/adr/", "type": "dir", "owner": "platform", "responsibility": "catatan keputusan",
			"reason": "jejak arsitektur", "allowed": []string{"ADR"}, "forbidden": []string{"kode"},
		}}},
		"note": "Folder ADR ditambahkan",
	})
	if err := ApplyRepoRevision(p, raw); err != nil {
		t.Fatal(err)
	}
	plan := p.Structure.Repo
	if plan.Node("docs/adr/") == nil {
		t.Fatal("ops were dropped because evaluation was missing")
	}
	if !plan.Evaluation.Outdated || plan.Evaluation.Summary != before {
		t.Fatalf("expected previous evaluation marked outdated, got %+v", plan.Evaluation)
	}
	if p.Structure.RepoRequest != nil {
		t.Fatal("request still open")
	}
}

func TestApplyRepoRevisionAcceptsFullPlan(t *testing.T) {
	p := reviseReady()
	full := p.Structure.Repo.Clone()
	full.Nodes = append(full.Nodes, dirNode("docs/adr/", core.OwnerPlatform, "catatan keputusan", ""))
	full.Evaluation.Summary = "Rancangan ulang"
	raw := mustJSON(full)
	if err := ApplyRepoRevision(p, raw); err != nil {
		t.Fatal(err)
	}
	plan := p.Structure.Repo
	if plan.Node("docs/adr/") == nil || p.Structure.RepoRequest != nil {
		t.Fatalf("full plan not landed: node=%v request=%+v", plan.Node("docs/adr/"), p.Structure.RepoRequest)
	}
	if plan.Evaluation.Summary != "Rancangan ulang" {
		t.Fatalf("evaluation = %+v", plan.Evaluation)
	}
}

func TestApplyRepoRevisionLandsTruncatedOps(t *testing.T) {
	p := reviseReady()
	raw := "```json\n" + `{"ops":[{"op":"set_node","node":{"path":"docs/adr/","type":"dir","owner":"platform","responsibility":"catatan keputusan","reason":"jejak arsitektur","allowed":["ADR"],"forbidden":["kode"]}}],"note":"Folder ADR"`
	if err := ApplyRepoRevision(p, raw); err != nil {
		t.Fatal(err)
	}
	if p.Structure.Repo.Node("docs/adr/") == nil {
		t.Fatal("truncated revision dropped the ops")
	}
}

func TestApplyRepoRevisionRejectsEmptyAnswerWithoutReview(t *testing.T) {
	p := reviseReady()
	p.Structure.Repo.Evaluation = core.RepoEvaluation{}
	p.Structure.Repo.Consistency = nil
	if err := ApplyRepoRevision(p, mustJSON(map[string]any{"note": "tidak berubah"})); err == nil ||
		!strings.Contains(err.Error(), "evaluation dan consistency") {
		t.Fatalf("err = %v", err)
	}
	if p.Structure.Repo.Node("docs/adr/") != nil {
		t.Fatal("empty answer changed the plan")
	}
}
