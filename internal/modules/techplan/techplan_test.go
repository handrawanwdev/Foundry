package techplan_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/stack"
	"github.com/handrawanwdev/foundry/internal/modules/techplan"
)

func project(kinds ...string) *core.Project {
	p := &core.Project{
		Name: "demo", Story: "Aplikasi klinik untuk janji temu pasien",
		Structure:    &core.Structure{Mode: core.StructureNew},
		Platform:     &core.PlatformPlan{},
		Requirements: []core.Requirement{{ID: "REQ-001", Text: "Pasien membuat janji temu", Priority: "MUST"}},
		Features:     []core.Feature{{ID: "FEAT-001", Name: "Janji temu"}},
	}
	for _, k := range kinds {
		p.Platform.Targets = append(p.Platform.Targets, core.PlatformTarget{Kind: k})
	}
	p.Stack = decided(p)
	stack.Record(p.Stack, nil, "recommend", "")
	return p
}

// decided is the stack an earlier AI plan decided for the platform. Foundry
// has no recommender of its own; the tests start from a decided stack.
func decided(p *core.Project) *core.TechStack {
	s := stack.Baseline(p)
	pl := s.Platform
	add := func(on bool, area, tech, lang string) {
		if on {
			s.Decisions = append(s.Decisions, core.TechDecision{Area: area, Recommended: tech, Language: lang, Reason: "rencana AI", Source: core.SourceAI})
		}
	}
	add(pl.Web, core.TechFrontend, "React + TypeScript + Vite", "TypeScript")
	add(pl.Web, core.TechUI, "Tailwind CSS", "")
	add(pl.Mobile, core.TechMobile, "Flutter", "Dart")
	add(pl.Backend, core.TechBackend, "NestJS + TypeScript", "TypeScript")
	add(pl.Backend, core.TechAPI, "REST + OpenAPI", "")
	add(pl.Backend, core.TechDatabase, "PostgreSQL", "SQL")
	for _, a := range []string{core.TechCache, core.TechMessaging, core.TechWorkers, core.TechStorage, core.TechSearch} {
		add(pl.Backend, a, core.TechNone, "")
	}
	add(true, core.TechArchitecture, "Modular Monolith", "")
	add(true, core.TechRepository, "Single repository", "")
	add(pl.Backend || pl.Web, core.TechDeployment, "Docker Compose (satu server)", "")
	add(pl.Backend, core.TechObservability, "Log terstruktur + health check", "")
	add(pl.Web || pl.Backend, core.TechTesting, "Vitest", "TypeScript")
	add(pl.Web || pl.Backend, core.TechQuality, "ESLint + Prettier", "TypeScript")
	stack.Refresh(s)
	return s
}

func filled(area, tech, lang string) core.TechItem {
	return core.TechItem{
		Area: area, Technology: tech, Language: lang,
		Purpose:     "Melayani kebutuhan " + area + " pada sistem ini",
		Fulfills:    []string{"REQ-001"},
		Driver:      core.DriverRequirement,
		Reasoning:   "Kebutuhan sistem ini terpenuhi oleh " + tech + " pada area " + area,
		Pros:        []string{"Cocok dengan kebutuhan yang disebutkan"},
		Cons:        []string{"Tim harus mempelajari ekosistemnya"},
		Tradeoffs:   []string{"Kompleksitas operasional bertambah sedikit"},
		Operational: "Perlu dioperasikan dan dipantau",
		Confidence:  "high",
	}
}

func noneItem(area string) core.TechItem {
	return core.TechItem{
		Area: area, Technology: core.TechNone, Driver: core.DriverEngineering, Confidence: "high",
		Reasoning: "Area ini tidak dibutuhkan sistem berdasarkan kebutuhan yang ada",
	}
}

func evaluated(plan *core.TechPlan) {
	plan.Approach, plan.Summary = "Stack sesuai kebutuhan", "Teknologi dipilih dari kebutuhan aktual, bukan dari katalog."
	plan.Context = core.TechContext{
		Summary: "Sistem butuh aplikasi dengan data yang konsisten.",
		Facts:   []core.TechFact{{Category: "functional", Statement: "Pengguna membuat janji temu", Source: "requirements"}},
	}
	plan.Evaluation.Criteria = nil
	for _, c := range core.TechCriteria {
		plan.Evaluation.Criteria = append(plan.Evaluation.Criteria, core.TechCriterion{
			Name: c, Score: 4, Reason: "Alasan konkret dan cukup panjang untuk " + c, Evidence: []string{"backend", "REQ-001"},
		})
	}
	plan.Consistency = nil
	for _, a := range core.TechConsistencyAreas {
		plan.Consistency = append(plan.Consistency, core.TechCheck{Area: a, Status: core.CheckConsistent, Note: "Sesuai"})
	}
	plan.Compatibility = []core.TechCompat{{
		Area: "runtime", Status: core.CompatCompatible, Between: []string{"backend", "database"},
		Note: "Runtime backend dan database terhubung lewat driver resmi", Basis: "dokumentasi driver resmi",
	}}
}

// overlay copies every stack decision into a complete plan, then applies
// technology replacements and extra areas.
func overlay(p *core.Project, techs map[string]string, extra ...core.TechItem) *core.TechPlan {
	plan := &core.TechPlan{}
	evaluated(plan)
	for _, d := range p.Stack.Decisions {
		tech := d.Choice()
		if t, ok := techs[d.Area]; ok {
			tech = t
		}
		if tech == "" || tech == core.TechNone {
			plan.Items = append(plan.Items, noneItem(d.Area))
			continue
		}
		plan.Items = append(plan.Items, filled(d.Area, tech, d.Lang()))
	}
	plan.Items = append(plan.Items, extra...)
	return plan
}

func mustAccept(t *testing.T, p *core.Project, plan *core.TechPlan) {
	t.Helper()
	if err := techplan.Accept(p, plan); err != nil {
		t.Fatalf("accept: %v", err)
	}
}

func hasIssue(plan *core.TechPlan, part string) bool {
	return slices.ContainsFunc(plan.Analysis.Issues, func(i core.TechIssue) bool {
		return strings.Contains(i.Message, part)
	})
}

func hasError(plan *core.TechPlan, part string) bool {
	return slices.ContainsFunc(plan.Errors(), func(i core.TechIssue) bool {
		return strings.Contains(i.Message, part)
	})
}

func TestNormalizeCleansPlan(t *testing.T) {
	plan := &core.TechPlan{
		Approach: "  Elixir realtime  ", Summary: "Ringkasan",
		Items: []core.TechItem{
			{Area: "CI/CD", Technology: "GitHub Actions", Group: "wrong",
				Purpose: "pipeline", Fulfills: []string{"REQ-001"}, Driver: core.DriverOperational,
				Reasoning: "Build dan rilis harus otomatis untuk tim kecil ini",
				Pros:      []string{"sudah dipakai"}, Cons: []string{"vendor"}, Confidence: "high"},
			{Area: "architecture", Technology: "  Cell-based  modular monolith "},
			{Area: "repository", Technology: "Polyrepo per bounded context"},
			{Area: "backend", Technology: "Elixir + Phoenix", Language: "Elixir"},
		},
		Compatibility: []core.TechCompat{
			{Area: "runtime", Status: core.CompatCompatible, Between: []string{"backend"}},
			{Area: "API", Status: "maybe", Between: []string{"backend", "frontend"}},
		},
		Evaluation: core.TechEvaluation{Score: 99, Criteria: []core.TechCriterion{
			{Name: "requirement_coverage", Score: 5, Reason: "Cakupan kebutuhan lengkap untuk sistem ini", Evidence: []string{"backend"}},
			{Name: "operational_complexity", Score: 5, Reason: "Operasi cukup berat karena banyak layanan yang dijalankan", Evidence: []string{"backend"}},
			{Name: "total_cost", Score: 1, Reason: "Biaya rendah karena memakai layanan managed yang sudah ada", Evidence: []string{"backend"}},
		}},
	}
	techplan.Normalize(plan)

	ci := plan.Item("ci_cd")
	if ci == nil || ci.Group != core.TechPlanOperations {
		t.Fatalf("ci_cd = %+v", ci)
	}
	// Architecture and repository are free text: no closed list.
	if it := plan.Item(core.TechArchitecture); it == nil || it.Technology != "Cell-based modular monolith" || it.Group != core.TechPlanArchitecture {
		t.Fatalf("architecture = %+v", plan.Item(core.TechArchitecture))
	}
	if it := plan.Item(core.TechRepository); it == nil || it.Technology != "Polyrepo per bounded context" {
		t.Fatalf("repository = %+v", plan.Item(core.TechRepository))
	}
	be := plan.Item(core.TechBackend)
	if be == nil || be.Group != core.TechPlanStack || be.Language != "Elixir" {
		t.Fatalf("backend = %+v", be)
	}
	if plan.Compatibility[0].Status != core.CompatUnverified {
		t.Fatalf("compatible without basis = %s", plan.Compatibility[0].Status)
	}
	if plan.Compatibility[1].Status != core.CompatUnverified || plan.Compatibility[1].Area != "api" {
		t.Fatalf("unknown status = %+v", plan.Compatibility[1])
	}
	// Foundry computes the score; inverted criteria count the other way.
	if plan.Evaluation.Score != techplan.Score(plan.Evaluation.Criteria) || plan.Evaluation.Score == 99 {
		t.Fatalf("score = %d (AI value was kept)", plan.Evaluation.Score)
	}
}

func TestScoreInvertsCostAndComplexity(t *testing.T) {
	all := func(good, bad int) []core.TechCriterion {
		var cs []core.TechCriterion
		for _, n := range core.TechCriteria {
			s := good
			if slices.Contains(core.TechLowerIsBetter, n) {
				s = bad
			}
			cs = append(cs, core.TechCriterion{Name: n, Score: s})
		}
		return cs
	}
	if got := techplan.Score(all(5, 1)); got != 100 {
		t.Fatalf("best = %d", got)
	}
	if got := techplan.Score(all(3, 3)); got != 50 {
		t.Fatalf("mid = %d", got)
	}
	highCost := techplan.Score(all(5, 5))
	if highCost >= 100 || highCost <= 50 {
		t.Fatalf("inverted high cost should land between, got %d", highCost)
	}
	if techplan.Score(all(4, 4)) == techplan.Score(all(4, 2)) {
		t.Fatal("inverting operational_complexity/total_cost did not change the score")
	}
}

func TestAcceptValidPlan(t *testing.T) {
	p := project(core.PlatformWeb, core.PlatformBackend)
	plan := overlay(p, map[string]string{
		core.TechBackend:  "Elixir + Phoenix",
		core.TechDatabase: "TimescaleDB",
		core.TechCache:    "Valkey",
	}, filled("authentication", "Phoenix.Session", "Elixir"), filled("orm", "Ecto", "Elixir"))
	mustAccept(t, p, plan)
	if hasIssue(plan, "katalog") {
		t.Fatalf("a technology was judged against a catalog: %+v", plan.Analysis.Issues)
	}
}

// Evidence written as a sentence counts when it names an area or a
// requirement.
func TestAcceptEvidenceSentence(t *testing.T) {
	p := project(core.PlatformWeb, core.PlatformBackend)
	plan := overlay(p, nil)
	plan.Evaluation.Criteria[0].Evidence = []string{"REQ-001..REQ-034 dipetakan pada areas database, orm."}
	plan.Evaluation.Criteria[1].Evidence = []string{"Areas: backend; module:iam."}
	mustAccept(t, p, plan)
}

func TestAcceptRefusals(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*core.Project, *core.TechPlan)
		want   string
	}{
		{"no reasoning", func(_ *core.Project, pl *core.TechPlan) { pl.Item(core.TechBackend).Reasoning = "" }, "reasoning"},
		{"no fulfills", func(_ *core.Project, pl *core.TechPlan) { pl.Item(core.TechBackend).Fulfills = nil }, "fulfills"},
		{"no pros", func(_ *core.Project, pl *core.TechPlan) { pl.Item(core.TechBackend).Pros = nil }, "pros"},
		{"no cons", func(_ *core.Project, pl *core.TechPlan) { pl.Item(core.TechBackend).Cons = nil }, "cons"},
		{"web without frontend", func(p *core.Project, pl *core.TechPlan) {
			pl.Items = slices.DeleteFunc(pl.Items, func(it core.TechItem) bool { return it.Area == core.TechFrontend })
			_ = p
		}, "tidak menentukan teknologi frontend"},
		{"backend without backend", func(p *core.Project, pl *core.TechPlan) {
			pl.Items = slices.DeleteFunc(pl.Items, func(it core.TechItem) bool { return it.Area == core.TechBackend })
			_ = p
		}, "tidak menentukan teknologi backend"},
		{"architecture missing", func(_ *core.Project, pl *core.TechPlan) {
			pl.Items = slices.DeleteFunc(pl.Items, func(it core.TechItem) bool { return it.Area == core.TechArchitecture })
		}, "rencana harus menentukan architecture"},
		{"duplicate area", func(_ *core.Project, pl *core.TechPlan) {
			pl.Items = append(pl.Items, filled(core.TechBackend, "Go + Gin", "Go"))
		}, "muncul lebih dari sekali"},
		{"evaluation missing criterion", func(_ *core.Project, pl *core.TechPlan) {
			pl.Evaluation.Criteria = pl.Evaluation.Criteria[1:]
		}, "evaluasi belum menilai"},
		{"score out of range", func(_ *core.Project, pl *core.TechPlan) {
			pl.Evaluation.Criteria[0].Score = 9
		}, "skor evaluasi harus 1-5"},
		{"short reason", func(_ *core.Project, pl *core.TechPlan) {
			pl.Evaluation.Criteria[0].Reason = "pendek"
		}, "alasan skor terlalu singkat"},
		{"unresolved evidence", func(_ *core.Project, pl *core.TechPlan) {
			pl.Evaluation.Criteria[0].Evidence = []string{"tidak-ada-di-mana-pun"}
		}, "skor tanpa evidence"},
		{"evidence sentence naming nothing", func(_ *core.Project, pl *core.TechPlan) {
			pl.Evaluation.Criteria[0].Evidence = []string{"Bagian compatibility: 21 entri, 0 conflict."}
		}, "skor tanpa evidence"},
		{"consistency missing area", func(_ *core.Project, pl *core.TechPlan) {
			pl.Consistency = pl.Consistency[1:]
		}, "pemeriksaan konsistensi belum lengkap"},
		{"empty context", func(_ *core.Project, pl *core.TechPlan) {
			pl.Context = core.TechContext{}
		}, "context teknis wajib diisi"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := project(core.PlatformWeb, core.PlatformBackend)
			plan := overlay(p, nil)
			c.mutate(p, plan)
			err := techplan.Accept(p, plan)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want %q", err, c.want)
			}
		})
	}
}

func TestAcceptRefusesUserOverrideAndExistingCode(t *testing.T) {
	p := project(core.PlatformWeb, core.PlatformBackend)
	if err := stack.Override(p.Stack, core.TechBackend, "Rust + Axum", "tim sudah Rust"); err != nil {
		t.Fatal(err)
	}
	plan := overlay(p, map[string]string{core.TechBackend: "Elixir + Phoenix"})
	err := techplan.Accept(p, plan)
	if err == nil || !strings.Contains(err.Error(), "pilihan user") {
		t.Fatalf("override err = %v", err)
	}
	plan = overlay(p, map[string]string{core.TechBackend: "Rust + Axum"})
	mustAccept(t, p, plan)
	if plan.Item(core.TechBackend).UserChosen == "" {
		t.Fatal("user choice was not recorded on the item")
	}

	p = project(core.PlatformWeb, core.PlatformBackend)
	p.Stack.Mode = core.StructureExisting
	d := p.Stack.Decision(core.TechBackend)
	d.Source, d.Recommended, d.Override = core.SourceExisting, "Go + Gin", nil
	plan = overlay(p, map[string]string{core.TechBackend: "Elixir + Phoenix"})
	err = techplan.Accept(p, plan)
	if err == nil || !strings.Contains(err.Error(), "sudah dipakai di kode") {
		t.Fatalf("existing err = %v", err)
	}
	plan = overlay(p, map[string]string{core.TechBackend: "Go + Gin"})
	mustAccept(t, p, plan)
}

func TestCrossArtifactConflictsLandAsIssues(t *testing.T) {
	cases := []struct {
		name    string
		setup   func(*core.Project, *core.TechPlan)
		want    string
		warning bool
	}{
		{"transactions without database", func(p *core.Project, pl *core.TechPlan) {
			if !slices.Contains(p.Stack.Needs, "transactions") {
				p.Stack.Needs = append(p.Stack.Needs, "transactions")
			}
			for i := range pl.Items {
				if pl.Items[i].Area == core.TechDatabase {
					pl.Items[i] = noneItem(core.TechDatabase)
				}
			}
		}, "kebutuhan transaksi", false},
		{"files without storage", func(p *core.Project, pl *core.TechPlan) {
			p.Stack.Needs = append(p.Stack.Needs, "files")
			pl.Items = slices.DeleteFunc(pl.Items, func(it core.TechItem) bool { return it.Area == core.TechStorage })
		}, "upload file", false},
		{"backend without deployment", func(_ *core.Project, pl *core.TechPlan) {
			pl.Items = slices.DeleteFunc(pl.Items, func(it core.TechItem) bool { return it.Area == core.TechDeployment })
		}, "tidak menentukan deployment", false},
		{"orm language mismatch", func(_ *core.Project, pl *core.TechPlan) {
			pl.Item(core.TechBackend).Technology = "Elixir + Phoenix"
			pl.Item(core.TechBackend).Language = "Elixir"
			pl.Items = append(pl.Items, filled("orm", "Prisma", "TypeScript"))
		}, "tidak kompatibel dengan backend", false},
		{"job_queue without worker", func(_ *core.Project, pl *core.TechPlan) {
			for i := range pl.Items {
				if pl.Items[i].Area == core.TechWorkers {
					pl.Items[i] = noneItem(core.TechWorkers)
				}
			}
			pl.Items = append(pl.Items, filled("job_queue", "Oban", "Elixir"))
		}, "tidak ada worker", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := project(core.PlatformWeb, core.PlatformBackend)
			plan := overlay(p, map[string]string{core.TechBackend: "Elixir + Phoenix"})
			c.setup(p, plan)
			if err := techplan.Accept(p, plan); err != nil {
				t.Fatalf("conflict must land, not refuse: %v", err)
			}
			if _, err := techplan.Land(p, plan, "generate", "", "", ""); err != nil {
				t.Fatalf("land: %v", err)
			}
			if c.warning {
				if !hasIssue(p.Stack.Plan, c.want) || hasError(p.Stack.Plan, c.want) {
					t.Fatalf("want warning %q, issues = %v", c.want, p.Stack.Plan.Analysis.Issues)
				}
				if err := techplan.Gate(p); err != nil {
					t.Fatalf("warning must not block approve: %v", err)
				}
				return
			}
			if !hasError(p.Stack.Plan, c.want) {
				t.Fatalf("want error %q, issues = %v", c.want, p.Stack.Plan.Analysis.Issues)
			}
			if err := techplan.Gate(p); err == nil {
				t.Fatal("gate allowed a plan with conflicts")
			}
		})
	}
}

func TestStaleAreasAndAffected(t *testing.T) {
	p := project(core.PlatformWeb, core.PlatformBackend)
	plan := overlay(p, nil)
	mustAccept(t, p, plan)
	if plan.Stale || len(plan.Analysis.StaleAreas) != 0 {
		t.Fatalf("fresh plan is stale: %v", plan.Analysis.StaleAreas)
	}

	p.Story = "Aplikasi klinik plus antrian realtime"
	if got := techplan.StaleAreas(p, plan); !slices.Equal(got, []string{"requirements"}) {
		t.Fatalf("story stale = %v", got)
	}
	if got := techplan.Affected([]string{"requirements"}); !slices.Contains(got, "context") || !slices.Contains(got, "stack") || !slices.Contains(got, "evaluation") {
		t.Fatalf("requirements affected = %v", got)
	}

	p = project(core.PlatformWeb, core.PlatformBackend)
	plan = overlay(p, nil)
	mustAccept(t, p, plan)
	p.Platform.Targets = append(p.Platform.Targets, core.PlatformTarget{Kind: core.PlatformCLI})
	if got := techplan.StaleAreas(p, plan); !slices.Equal(got, []string{"platform"}) {
		t.Fatalf("platform stale = %v", got)
	}
	aff := techplan.Affected([]string{"platform"})
	for _, s := range []string{"context", "stack", "architecture", "infra", "evaluation"} {
		if !slices.Contains(aff, s) {
			t.Fatalf("platform affected missing %s: %v", s, aff)
		}
	}

	p = project(core.PlatformWeb, core.PlatformBackend)
	p.Blueprint = &core.Blueprint{Modules: []core.Module{{ID: "booking", Responsibility: "janji"}}}
	plan = overlay(p, nil)
	mustAccept(t, p, plan)
	p.Blueprint.Modules = append(p.Blueprint.Modules, core.Module{ID: "billing", Responsibility: "tagihan"})
	if got := techplan.StaleAreas(p, plan); !slices.Equal(got, []string{"modules"}) {
		t.Fatalf("modules stale = %v", got)
	}
	if got := techplan.Affected([]string{"modules"}); !slices.Contains(got, "architecture") || !slices.Contains(got, "evaluation") {
		t.Fatalf("modules affected = %v", got)
	}

	techplan.Refresh(p, plan)
	if !plan.Stale || !slices.Equal(plan.Analysis.StaleAreas, []string{"modules"}) {
		t.Fatalf("refresh stale = %v", plan.Analysis.StaleAreas)
	}
}

// The folder plan is written from the technology: rewriting it must not mark
// the technology plan stale, or each revision would invalidate the other.
func TestFolderPlanIsNotASource(t *testing.T) {
	p := project(core.PlatformWeb, core.PlatformBackend)
	p.Structure = &core.Structure{Mode: core.StructureNew, Repo: &core.RepoPlan{Approach: "a", Topology: "t"}}
	plan := overlay(p, nil)
	mustAccept(t, p, plan)
	p.Structure.Repo.Approach, p.Structure.Repo.Topology = "rewritten by AI", "other wording"
	if got := techplan.StaleAreas(p, plan); len(got) != 0 {
		t.Fatalf("folder rewrite made the plan stale: %v", got)
	}
	plan.Sources["folder"] = "old-hash" // plans written before folder stopped being a source
	if got := techplan.StaleAreas(p, plan); len(got) != 0 {
		t.Fatalf("legacy folder hash made the plan stale: %v", got)
	}
}

func TestLandGenerateAppliesOnRecommended(t *testing.T) {
	p := project(core.PlatformWeb, core.PlatformBackend)
	plan := overlay(p, map[string]string{core.TechBackend: "Elixir + Phoenix", core.TechDatabase: "TimescaleDB"})
	mustAccept(t, p, plan)
	v, err := techplan.Land(p, plan, "generate", "", "", "")
	if err != nil || v == nil {
		t.Fatalf("land = %v, %v", v, err)
	}
	if p.Stack.Plan != plan || p.Stack.Proposal != nil {
		t.Fatal("generate on a recommended stack must become the decision")
	}
	d := p.Stack.Decision(core.TechBackend)
	if d == nil || d.Source != core.SourceAI || d.Recommended != "Elixir + Phoenix" {
		t.Fatalf("backend decision = %+v", d)
	}
	if p.Stack.Choice(core.TechDatabase) != "TimescaleDB" {
		t.Fatalf("database = %s", p.Stack.Choice(core.TechDatabase))
	}
}

func TestLandProposesWhenApprovedOrOverriddenOrRevised(t *testing.T) {
	p := project(core.PlatformWeb, core.PlatformBackend)
	if err := stack.Approve(p.Stack); err != nil {
		t.Fatal(err)
	}
	plan := overlay(p, map[string]string{core.TechBackend: "Elixir + Phoenix"})
	mustAccept(t, p, plan)
	v, err := techplan.Land(p, plan, "generate", "pakai Elixir", "", "ganti backend")
	if err != nil || v != nil || p.Stack.Proposal == nil || p.Stack.Proposal.Plan != plan {
		t.Fatalf("approved: v=%v err=%v proposal=%v", v, err, p.Stack.Proposal)
	}
	if p.Stack.Choice(core.TechBackend) == "Elixir + Phoenix" {
		t.Fatal("approved stack changed before the user applied the proposal")
	}

	p = project(core.PlatformWeb, core.PlatformBackend)
	alt := "Vue + TypeScript + Vite"
	if p.Stack.Choice(core.TechFrontend) == alt {
		alt = "React + TypeScript + Vite"
	}
	if err := stack.Override(p.Stack, core.TechFrontend, alt, "tim Vue"); err != nil {
		t.Fatal(err)
	}
	plan = overlay(p, map[string]string{core.TechFrontend: alt, core.TechBackend: "Elixir + Phoenix"})
	mustAccept(t, p, plan)
	v, err = techplan.Land(p, plan, "generate", "", "", "")
	if err != nil || v != nil || p.Stack.Proposal == nil {
		t.Fatalf("override: v=%v err=%v proposal=%v", v, err, p.Stack.Proposal)
	}

	p = project(core.PlatformWeb, core.PlatformBackend)
	first := overlay(p, map[string]string{core.TechBackend: "Elixir + Phoenix"})
	mustAccept(t, p, first)
	if _, err := techplan.Land(p, first, "generate", "", "", ""); err != nil {
		t.Fatal(err)
	}
	rev := overlay(p, map[string]string{core.TechBackend: "Elixir + Phoenix", core.TechCache: "Valkey"})
	mustAccept(t, p, rev)
	v, err = techplan.Land(p, rev, "ai", "ganti cache", "", "Valkey")
	if err != nil || v != nil || p.Stack.Proposal == nil {
		t.Fatalf("revision: v=%v err=%v proposal=%v", v, err, p.Stack.Proposal)
	}
	if p.Stack.Choice(core.TechCache) == "Valkey" {
		t.Fatal("revision applied without the user")
	}
}

func TestLandReasoningOnlyHasEmptyDiff(t *testing.T) {
	p := project(core.PlatformWeb, core.PlatformBackend)
	plan := overlay(p, nil)
	mustAccept(t, p, plan)
	before := p.Stack.Version()
	v, err := techplan.Land(p, plan, "generate", "", "", "")
	if err != nil || v != nil {
		t.Fatalf("same technologies: v=%v err=%v", v, err)
	}
	if p.Stack.Version() != before {
		t.Fatalf("version moved from %d to %d", before, p.Stack.Version())
	}
	if p.Stack.Plan != plan || plan.Version() == 0 {
		t.Fatalf("plan was not recorded: %+v", plan.History)
	}
}

func TestGateBlocksStaleAndErrors(t *testing.T) {
	p := project(core.PlatformWeb, core.PlatformBackend)
	if err := techplan.Gate(p); err != nil {
		t.Fatalf("no plan: %v", err)
	}
	plan := overlay(p, map[string]string{core.TechBackend: "Elixir + Phoenix"})
	mustAccept(t, p, plan)
	if _, err := techplan.Land(p, plan, "generate", "", "", ""); err != nil {
		t.Fatal(err)
	}
	if err := techplan.Gate(p); err != nil {
		t.Fatalf("fresh plan: %v", err)
	}

	p.Story = "Cerita baru yang mengubah hash sumber"
	if err := techplan.Gate(p); err == nil || !strings.Contains(err.Error(), "sumber rencana teknologi berubah") {
		t.Fatalf("stale gate = %v", err)
	}

	p = project(core.PlatformWeb, core.PlatformBackend)
	if !slices.Contains(p.Stack.Needs, "transactions") {
		p.Stack.Needs = append(p.Stack.Needs, "transactions")
	}
	plan = overlay(p, map[string]string{core.TechDatabase: core.TechNone})
	mustAccept(t, p, plan)
	if _, err := techplan.Land(p, plan, "generate", "", "", ""); err != nil {
		t.Fatal(err)
	}
	if err := techplan.Gate(p); err == nil || !strings.Contains(err.Error(), "masalah rencana teknologi") {
		t.Fatalf("conflict gate = %v", err)
	}
}

func TestAreaID(t *testing.T) {
	if got := techplan.AreaID("CI/CD"); got != "ci_cd" {
		t.Fatalf("CI/CD = %s", got)
	}
	if got := techplan.AreaID("  State Management "); got != "state_management" {
		t.Fatalf("State Management = %s", got)
	}
}
