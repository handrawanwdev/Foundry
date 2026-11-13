package pipeline_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/pipeline"
	"github.com/handrawanwdev/foundry/internal/modules/spec"
	"github.com/handrawanwdev/foundry/internal/modules/stack"
	"github.com/handrawanwdev/foundry/internal/modules/techplan"
	"github.com/handrawanwdev/foundry/internal/modules/work"
)

func techReady(t *testing.T, s *pipeline.Service, kinds ...string) *core.Project {
	t.Helper()
	designReady(t, s)
	if len(kinds) == 0 {
		return load(t, s)
	}
	var targets []core.PlatformTarget
	for _, k := range kinds {
		targets = append(targets, core.PlatformTarget{Kind: k, Rationale: k})
	}
	p, err := s.SetPlatform(targets)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func generateTech(t *testing.T, s *pipeline.Service, text string) *core.TechPlan {
	t.Helper()
	if err := s.GenerateTechPlan(text, ""); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p := load(t, s)
	mustJob(t, p, work.KindTech, core.JobDone)
	if p.Stack == nil || p.Stack.Plan == nil || p.Stack.PlanRequest != nil {
		t.Fatalf("plan=%v request=%+v job=%+v", p.Stack.Plan, p.Stack.PlanRequest, p.Job)
	}
	return p.Stack.Plan
}

func fakeTechEvaluation() map[string]any {
	var crit []map[string]any
	for _, c := range core.TechCriteria {
		crit = append(crit, map[string]any{
			"name": c, "score": 4,
			"reason":   "Alasan konkret dan cukup panjang untuk " + c,
			"evidence": []string{"REQ-001", "architecture"},
		})
	}
	return map[string]any{"summary": "Stack sederhana dan memenuhi kebutuhan", "criteria": crit, "findings": []any{}}
}

func fakeTechConsistency() []map[string]any {
	var out []map[string]any
	for _, a := range core.TechConsistencyAreas {
		out = append(out, map[string]any{"area": a, "status": "consistent", "note": "Sesuai"})
	}
	return out
}

func fakeTechCompat() []map[string]any {
	return []map[string]any{
		{"area": "runtime", "status": "compatible", "between": []string{"backend", "database"},
			"note": "Driver resmi menghubungkan keduanya", "basis": "dokumentasi driver resmi"},
		{"area": "language", "status": "compatible", "between": []string{"backend", "orm"},
			"note": "ORM berjalan di runtime backend", "basis": "satu bahasa dengan backend"},
	}
}

func fakeTechItem(area, tech, lang string) map[string]any {
	if tech == core.TechNone {
		return map[string]any{
			"area": area, "technology": tech, "driver": "engineering", "confidence": "high",
			"reasoning": "Sistem ini tidak membutuhkan teknologi di area " + area + " berdasarkan kebutuhan yang ada",
		}
	}
	item := map[string]any{
		"area": area, "technology": tech, "language": lang,
		"purpose": "Menyediakan " + area + " untuk sistem ini", "fulfills": []string{"REQ-001"},
		"driver": "requirement", "confidence": "high",
		"reasoning": "Pilihan " + tech + " memenuhi kebutuhan aktual pada area " + area + " tanpa mengikuti katalog",
		"pros":      []string{"Cocok dengan kebutuhan yang disebutkan"}, "cons": []string{"Tim perlu mempelajari ekosistem ini"},
		"tradeoffs": []string{"Bukan pilihan default katalog Foundry"}, "operational": "Perlu dioperasikan sesuai dokumentasi resmi",
		"alternatives": []map[string]any{{
			"technology": "Pilihan katalog", "why": "Sudah dikenal Foundry",
			"difference": "Lebih umum tetapi kurang pas untuk kebutuhan ini",
			"pros":       []string{"dokumentasi banyak"}, "cons": []string{"tidak pas kebutuhan"},
		}},
	}
	if lang == "" {
		delete(item, "language")
	}
	return item
}

func parseBoundTech(prompt string) map[string][2]string {
	out := map[string][2]string{}
	for _, header := range []string{"TECHNOLOGY ALREADY USED IN THE CODE", "CHOSEN BY THE USER"} {
		for _, l := range promptLines(prompt, header) {
			area, rest, ok := strings.Cut(l, ": ")
			if !ok {
				continue
			}
			tech := rest
			if before, _, ok := strings.Cut(rest, " (language "); ok {
				tech = before
			}
			if before, _, ok := strings.Cut(tech, " — "); ok {
				tech = before
			}
			lang := ""
			if _, after, ok := strings.Cut(rest, "(language "); ok {
				lang, _, _ = strings.Cut(after, ")")
			}
			out[strings.TrimSpace(area)] = [2]string{strings.TrimSpace(tech), strings.TrimSpace(lang)}
		}
	}
	return out
}

func techPlatformKinds(prompt string) []string {
	var out []string
	for _, l := range promptLines(prompt, "TARGET PLATFORM:") {
		kind, _, _ := strings.Cut(l, ":")
		kind = strings.TrimSpace(kind)
		if kind != "" && !slices.Contains(out, kind) {
			out = append(out, kind)
		}
	}
	return out
}

func pickFakeTech(area string, bound map[string][2]string, backendLang string) (tech, lang string) {
	if b, ok := bound[area]; ok {
		return b[0], b[1]
	}
	switch area {
	case core.TechFrontend:
		return "SvelteKit", "TypeScript"
	case core.TechUI:
		return "Tailwind CSS", ""
	case core.TechMobile:
		return "Flutter + Riverpod", "Dart"
	case core.TechDesktop:
		return "Iced + Rust", "Rust"
	case core.TechCLI:
		return "Go + urfave/cli", "Go"
	case core.TechBackend:
		return "Elixir + Phoenix", "Elixir"
	case core.TechAPI:
		return "REST + OpenAPI + Phoenix Channels", ""
	case core.TechDatabase:
		return "TimescaleDB", "SQL"
	case core.TechCache:
		return "Valkey", ""
	case core.TechArchitecture:
		return core.ArchModular, ""
	case core.TechRepository:
		return core.RepoMonorepo, ""
	case core.TechDeployment:
		return "Fly.io Machines", ""
	case core.TechObservability:
		return "Log terstruktur + health check", ""
	case core.TechTesting:
		if backendLang == "Go" {
			return "go test", "Go"
		}
		return "ExUnit", "Elixir"
	case core.TechQuality:
		if backendLang == "Go" {
			return "gofmt + go vet + golangci-lint", "Go"
		}
		return "Credo + mix format", "Elixir"
	case "authentication":
		if backendLang == "Elixir" {
			return "Phoenix.Session", "Elixir"
		}
		return "JWT session tokens", backendLang
	case "orm":
		if backendLang == "Elixir" {
			return "Ecto", "Elixir"
		}
		if backendLang == "Go" {
			return "sqlc", "Go"
		}
		return "Ecto", "Elixir"
	case "ci_cd":
		return "GitHub Actions", ""
	}
	return core.TechNone, ""
}

func fakeTechItems(prompt string) []map[string]any {
	bound := parseBoundTech(prompt)
	kinds := techPlatformKinds(prompt)
	has := func(k string) bool { return slices.Contains(kinds, k) }
	backendLang := "Elixir"
	if b, ok := bound[core.TechBackend]; ok && b[1] != "" {
		backendLang = b[1]
	}
	areas := []string{core.TechArchitecture, core.TechRepository}
	if has("web") {
		areas = append(areas, core.TechFrontend, core.TechUI)
	}
	if has("android") || has("ios") {
		areas = append(areas, core.TechMobile)
	}
	if has("desktop") {
		areas = append(areas, core.TechDesktop)
	}
	if has("cli") {
		areas = append(areas, core.TechCLI)
	}
	if has("backend") {
		areas = append([]string{core.TechBackend, core.TechAPI, core.TechDatabase, core.TechCache, core.TechDeployment, core.TechObservability, core.TechTesting, core.TechQuality}, areas...)
	}
	if !has("web") && !has("android") && !has("ios") && !has("desktop") && has("cli") {
		// CLI tool: simple architecture, no database.
		for i, a := range areas {
			if a == core.TechArchitecture {
				if _, ok := bound[a]; !ok {
					areas[i] = a
				}
			}
		}
	}
	seen := map[string]bool{}
	var items []map[string]any
	add := func(area string) {
		if seen[area] {
			return
		}
		seen[area] = true
		tech, lang := pickFakeTech(area, bound, backendLang)
		if area == core.TechArchitecture && !has("backend") && bound[area][0] == "" {
			tech = core.ArchSimple
		}
		if area == core.TechRepository && bound[area][0] == "" {
			if (has("web") && (has("android") || has("ios"))) || (has("web") && has("backend")) {
				tech = core.RepoMonorepo
			} else {
				tech = core.RepoSimple
			}
		}
		if area == core.TechDatabase && has("cli") && !has("backend") {
			tech = core.TechNone
		}
		if area == core.TechCache && !has("backend") {
			tech = core.TechNone
		}
		items = append(items, fakeTechItem(area, tech, lang))
	}
	for _, a := range areas {
		add(a)
	}
	for area := range bound {
		add(area)
	}
	if has("backend") {
		add("authentication")
		add("orm")
	}
	add("ci_cd")
	return items
}

func fakeTechPlanJSON(prompt string, invalid bool) string {
	items := fakeTechItems(prompt)
	if invalid {
		for _, it := range items {
			if it["area"] == core.TechBackend || it["area"] == core.TechFrontend || it["area"] == core.TechMobile || it["area"] == core.TechCLI {
				it["reasoning"] = ""
				it["pros"] = []any{}
			}
		}
	}
	plan := map[string]any{
		"approach":  "Stack sesuai kebutuhan aktual",
		"summary":   "Teknologi dipilih dari kebutuhan, termasuk yang di luar katalog Foundry.",
		"rationale": "Kebutuhan realtime, data, dan platform menuntun pilihan, bukan profil.",
		"context": map[string]any{
			"summary":     "Sistem butuh aplikasi yang sesuai platform target dan data yang konsisten.",
			"facts":       []map[string]any{{"category": "functional", "statement": "Pengguna memakai aplikasi sesuai platform yang dipilih", "source": "requirements"}},
			"unknowns":    []string{"Beban produksi belum diketahui"},
			"assumptions": []string{"Tim sanggup mengoperasikan stack yang dipilih"},
		},
		"items":         items,
		"compatibility": fakeTechCompat(),
		"evaluation":    fakeTechEvaluation(),
		"consistency":   fakeTechConsistency(),
	}
	raw, _ := json.Marshal(plan)
	return "```json\n" + string(raw) + "\n```"
}

func fakeTechRevisionJSON(prompt string, invalid bool) string {
	if invalid {
		raw, _ := json.Marshal(map[string]any{"set": []any{}, "remove": []any{}, "note": "tidak lengkap"})
		return "```json\n" + string(raw) + "\n```"
	}
	request := ""
	if _, after, ok := strings.Cut(prompt, "USER REQUEST:\n"); ok {
		request, _, _ = strings.Cut(after, "\n")
	}
	var set []map[string]any
	switch {
	case strings.Contains(request, "Redis"):
		set = append(set, fakeTechItem(core.TechCache, "Redis", ""))
	case strings.Contains(strings.ToLower(request), "valkey") || strings.Contains(strings.ToLower(request), "cache"):
		set = append(set, fakeTechItem(core.TechCache, "Valkey", ""))
	case strings.Contains(request, "NATS") || strings.Contains(strings.ToLower(request), "broker"):
		set = append(set, fakeTechItem(core.TechMessaging, "NATS JetStream", ""))
	}
	body := map[string]any{
		"set": set, "remove": []any{},
		"meta":          map[string]any{"summary": "Revisi menyesuaikan permintaan dan sumber yang berubah."},
		"compatibility": fakeTechCompat(),
		"evaluation":    fakeTechEvaluation(),
		"consistency":   fakeTechConsistency(),
		"note":          "Disesuaikan dengan permintaan",
	}
	raw, _ := json.Marshal(body)
	return "```json\n" + string(raw) + "\n```"
}

func fakeTechAnswer(prompt string, invalid bool) string {
	if strings.Contains(prompt, "Revise the CURRENT TECHNOLOGY PLAN") {
		return fakeTechRevisionJSON(prompt, invalid)
	}
	return fakeTechPlanJSON(prompt, invalid)
}

func assertOutsideCatalog(t *testing.T, p *core.Project) {
	t.Helper()
	plan := p.Stack.Plan
	if plan.Analysis.Outside == 0 {
		t.Fatal("expected technologies outside the catalog")
	}
	var outside bool
	for _, it := range plan.Items {
		if it.None() {
			continue
		}
		if !it.InCatalog || !stack.Known(it.Area, it.Technology) {
			outside = true
			break
		}
	}
	if !outside {
		t.Fatalf("every item is in the catalog: %+v", plan.Items)
	}
}

func TestTechPlanSaaSOutOfCatalog(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	techReady(t, s, core.PlatformWeb, core.PlatformBackend)
	plan := generateTech(t, s, "")
	p := load(t, s)
	if p.Stack.Choice(core.TechBackend) != "Elixir + Phoenix" || p.Stack.Decision(core.TechBackend).Source != core.SourceAI {
		t.Fatalf("backend = %s source=%s", p.Stack.Choice(core.TechBackend), p.Stack.Decision(core.TechBackend).Source)
	}
	if p.Stack.Choice(core.TechDatabase) != "TimescaleDB" || p.Stack.Choice(core.TechCache) != "Valkey" {
		t.Fatalf("data = db %s cache %s", p.Stack.Choice(core.TechDatabase), p.Stack.Choice(core.TechCache))
	}
	assertOutsideCatalog(t, p)
	if !strings.Contains(p.Stack.Brief(), "authentication:") || !strings.Contains(p.Stack.Brief(), "orm:") {
		t.Fatalf("brief missing extras:\n%s", p.Stack.Brief())
	}
	if err := techplan.Gate(p); err != nil {
		t.Fatalf("gate: %v issues=%v", err, plan.Analysis.Issues)
	}
	var hasPlanDoc bool
	for _, d := range spec.Artifacts(p) {
		if d.ID == core.ArtifactTechPlan && strings.Contains(d.Body, "Elixir + Phoenix") {
			hasPlanDoc = true
		}
	}
	if !hasPlanDoc {
		t.Fatal("technology-plan artifact missing")
	}
	if _, ok := spec.Files(p)["technology/plan.json"]; !ok {
		t.Fatal("technology/plan.json missing from export")
	}
	if _, err := s.RecommendStack(); err == nil || !strings.Contains(err.Error(), "rencana teknologi") {
		t.Fatalf("recommend with plan: %v", err)
	}
}

func TestTechPlanMobileOnly(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	techReady(t, s, core.PlatformAndroid)
	generateTech(t, s, "")
	p := load(t, s)
	if p.Stack.Decision(core.TechBackend) != nil && p.Stack.Choice(core.TechBackend) != core.TechNone && p.Stack.Choice(core.TechBackend) != "" {
		t.Fatalf("mobile-only has backend %s", p.Stack.Choice(core.TechBackend))
	}
	if p.Stack.Choice(core.TechMobile) != "Flutter + Riverpod" {
		t.Fatalf("mobile = %s", p.Stack.Choice(core.TechMobile))
	}
	assertOutsideCatalog(t, p)
	if p.Stack.Plan.Item(core.TechBackend) != nil && !p.Stack.Plan.Item(core.TechBackend).None() {
		t.Fatal("plan decided a backend the platform does not have")
	}
}

func TestTechPlanCLI(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	techReady(t, s, core.PlatformCLI)
	generateTech(t, s, "")
	p := load(t, s)
	if p.Stack.Choice(core.TechCLI) != "Go + urfave/cli" {
		t.Fatalf("cli = %s", p.Stack.Choice(core.TechCLI))
	}
	if db := p.Stack.Plan.Item(core.TechDatabase); db != nil && !db.None() {
		t.Fatalf("cli plan has a database: %+v", db)
	}
	assertOutsideCatalog(t, p)
}

func TestTechPlanExistingBinding(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	p := techReady(t, s, core.PlatformWeb, core.PlatformBackend)
	want := p.Stack.Choice(core.TechBackend)
	if _, err := s.Store.Update(func(p *core.Project) error {
		p.Structure.Mode = core.StructureExisting
		p.Stack.Mode = core.StructureExisting
		for i := range p.Stack.Decisions {
			p.Stack.Decisions[i].Source = core.SourceExisting
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	generateTech(t, s, "")
	p = load(t, s)
	if p.Stack.Choice(core.TechBackend) != want {
		t.Fatalf("existing backend replaced: got %s want %s", p.Stack.Choice(core.TechBackend), want)
	}
	d := p.Stack.Decision(core.TechBackend)
	if d.Source != core.SourceExisting {
		t.Fatalf("source = %s", d.Source)
	}
	assertOutsideCatalog(t, p)
}

func TestTechPlanLifecycleReviseApplyApprove(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	techReady(t, s, core.PlatformWeb, core.PlatformBackend)
	generateTech(t, s, "")
	p, err := s.ApproveStack()
	if err != nil {
		t.Fatal(err)
	}
	if p.Stack.State() != core.TechApproved {
		t.Fatalf("state = %s", p.Stack.State())
	}
	if err := s.ReviseTechPlan("Ganti cache ke Redis", ""); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p = load(t, s)
	mustJob(t, p, work.KindTech, core.JobDone)
	if p.Stack.Proposal == nil || p.Stack.Proposal.Plan == nil {
		t.Fatalf("revision of an approved stack must be a proposal: %+v", p.Stack.Proposal)
	}
	if p.Stack.Choice(core.TechCache) != "Valkey" {
		t.Fatalf("approved cache changed before apply: %s", p.Stack.Choice(core.TechCache))
	}
	if p.Stack.Proposal.Affected == nil || len(p.Stack.Proposal.Affected.Artifacts) == 0 {
		t.Fatalf("affected artifacts = %+v", p.Stack.Proposal.Affected)
	}
	p, err = s.ApplyStackProposal()
	if err != nil {
		t.Fatal(err)
	}
	if p.Stack.State() != core.TechChanged {
		t.Fatalf("applied revision of approved stack: state=%s", p.Stack.State())
	}
	if p.Stack.Choice(core.TechCache) != "Redis" {
		t.Fatalf("cache after apply = %s", p.Stack.Choice(core.TechCache))
	}
	if p.Stack.Plan == nil || p.Stack.Plan.Item(core.TechCache).Technology != "Redis" {
		t.Fatal("plan not installed from the proposal")
	}
	if _, err := s.ApproveStack(); err != nil {
		t.Fatal(err)
	}
}

func TestTechPlanRetriesRefusedAnswer(t *testing.T) {
	runs := filepath.Join(t.TempDir(), "runs")
	t.Setenv("FAKE_RUNS", runs)
	s := newService(t, fakeAI{}, "tech_invalid_once")
	techReady(t, s, core.PlatformWeb, core.PlatformBackend)
	if err := s.GenerateTechPlan("", ""); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p := load(t, s)
	if p.Stack.Plan == nil || p.Stack.PlanRequest != nil {
		t.Fatalf("not retried: plan=%v request=%+v job=%+v", p.Stack.Plan, p.Stack.PlanRequest, p.Job)
	}
	mustJob(t, p, work.KindTech, core.JobDone)
	raw, _ := os.ReadFile(runs)
	if strings.Count(string(raw), "tech") != 2 {
		t.Fatalf("runs = %q", raw)
	}
	var refused bool
	for _, l := range p.Events {
		refused = refused || strings.Contains(l.Message, "ditolak")
	}
	if !refused {
		t.Fatalf("the refusal was not logged: %+v", p.Events)
	}
}

func TestTechPlanDropsAfterTwoFailures(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	techReady(t, s, core.PlatformWeb, core.PlatformBackend)
	plan := generateTech(t, s, "")
	before := plan.Clone()
	t.Setenv("FAKE_MODE", "tech_invalid")
	if err := s.ReviseTechPlan("Ganti cache ke Valkey", ""); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p := load(t, s)
	if p.Stack.PlanRequest != nil || p.Job == nil || p.Job.Status != core.JobFailed {
		t.Fatalf("job=%+v request=%+v", p.Job, p.Stack.PlanRequest)
	}
	if p.Stack.Plan.Item(core.TechCache).Technology != before.Item(core.TechCache).Technology {
		t.Fatal("two refusals changed the existing plan")
	}
	if p.Stack.Proposal != nil {
		t.Fatal("a refused revision installed a proposal")
	}
	// The request is kept with the user's words; "Coba lagi" runs it again.
	if strings.Contains(p.Job.Error, "lebih spesifik") || p.Stack.PlanRetry == nil || p.Stack.PlanRetry.Text != "Ganti cache ke Valkey" {
		t.Fatalf("error=%q retry=%+v", p.Job.Error, p.Stack.PlanRetry)
	}
	t.Setenv("FAKE_MODE", "")
	if err := s.RetryJob(); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p = load(t, s)
	if p.Job == nil || p.Job.Status != core.JobDone || p.Stack.PlanRetry != nil || p.Stack.PlanRequest != nil {
		t.Fatalf("retry: job=%+v retry=%v", p.Job, p.Stack.PlanRetry != nil)
	}
}

func TestTechPlanUserOverrideNotReplaced(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	techReady(t, s, core.PlatformWeb, core.PlatformBackend)
	alt := "Vue + TypeScript + Vite"
	p, err := s.ProposeStack([]stack.Change{{Area: core.TechFrontend, Value: alt, Reason: "tim sudah Vue"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApplyStackProposal(); err != nil {
		t.Fatal(err)
	}
	if err := s.GenerateTechPlan("", ""); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p = load(t, s)
	mustJob(t, p, work.KindTech, core.JobDone)
	if p.Stack.Choice(core.TechFrontend) != alt {
		t.Fatalf("current frontend changed before apply: %s", p.Stack.Choice(core.TechFrontend))
	}
	if p.Stack.Proposal != nil {
		fe := p.Stack.Proposal.Plan.Item(core.TechFrontend)
		if fe == nil || fe.Technology != alt {
			t.Fatalf("proposal frontend = %+v", fe)
		}
		p, err = s.ApplyStackProposal()
		if err != nil {
			t.Fatal(err)
		}
	}
	if p.Stack.Plan == nil || p.Stack.Choice(core.TechFrontend) != alt {
		t.Fatalf("frontend after generate = %s plan=%v", p.Stack.Choice(core.TechFrontend), p.Stack.Plan != nil)
	}
}

func TestTechPlanRejectedInMCPPrompt(t *testing.T) {
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
	if _, err := s.SetPlatform([]core.PlatformTarget{{Kind: core.PlatformWeb, Rationale: "web"}, {Kind: core.PlatformBackend, Rationale: "api"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.GenerateTechPlan("", ""); err != nil {
		t.Fatal(err)
	}
	item, err := s.NextWork()
	if err != nil || item.Kind != work.KindTech || !strings.Contains(item.Prompt, "technology architect of Foundry") {
		t.Fatalf("work item = %+v, %v", item, err)
	}
	err = s.SubmitWork(work.KindTech, fakeTechAnswer(item.Prompt, true))
	if err == nil || !strings.Contains(err.Error(), "ditolak") {
		t.Fatalf("submit err = %v", err)
	}
	item, _ = s.NextWork()
	if item.Kind != work.KindTech || !strings.Contains(item.Prompt, "YOUR LAST ANSWER WAS REFUSED, fix this:") {
		t.Fatalf("the refusal is not in the next prompt: %s", item.Prompt[max(0, len(item.Prompt)-800):])
	}
	if err := s.SubmitWork(work.KindTech, fakeTechAnswer(item.Prompt, false)); err != nil {
		t.Fatal(err)
	}
	p := load(t, s)
	if p.Stack.Plan == nil || p.Stack.Choice(core.TechBackend) != "Elixir + Phoenix" {
		t.Fatalf("plan = %+v backend=%s", p.Stack.Plan, p.Stack.Choice(core.TechBackend))
	}
}

func TestTechPlanBriefInLaterPrompts(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	techReady(t, s, core.PlatformWeb, core.PlatformBackend)
	generateTech(t, s, "")
	p := load(t, s)
	docs := spec.Artifacts(p)
	wp := work.WorkflowPrompt(p, docs)
	ap := work.ArtifactsPrompt(p, docs)
	for name, got := range map[string]string{"workflow": wp, "blueprint": ap, "design": work.DesignPrompt(p)} {
		if !strings.Contains(got, "authentication") || !strings.Contains(got, "orm:") {
			t.Fatalf("%s prompt missing plan extras", name)
		}
	}
}

func TestRecommendStackRefusedWhenPlanExists(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	techReady(t, s, core.PlatformWeb, core.PlatformBackend)
	generateTech(t, s, "")
	if _, err := s.RecommendStack(); err == nil || !strings.Contains(err.Error(), "rencana teknologi") {
		t.Fatalf("err = %v", err)
	}
}
