package stack

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
)

func project(story string, kinds ...string) *core.Project {
	p := &core.Project{Story: story, Structure: &core.Structure{Mode: core.StructureNew}, Platform: &core.PlatformPlan{}}
	for _, k := range kinds {
		p.Platform.Targets = append(p.Platform.Targets, core.PlatformTarget{Kind: k})
	}
	for _, n := range []string{"Order", "Portofolio", "Laporan", "Pengguna"} {
		p.Features = append(p.Features, core.Feature{Name: n})
	}
	return p
}

func TestProfiles(t *testing.T) {
	if len(Profiles) != 5 {
		t.Fatalf("profiles = %d", len(Profiles))
	}
	for _, p := range Profiles {
		for _, area := range []string{core.TechFrontend, core.TechMobile, core.TechDesktop, core.TechBackend, core.TechDatabase, core.TechArchitecture} {
			if p.Decisions[area].Value == "" || p.Decisions[area].Reason == "" {
				t.Errorf("%s: %s missing", p.ID, area)
			}
			if closedAreas[area] && option(area, p.Decisions[area].Value) == nil {
				t.Errorf("%s: %s %q not in catalog", p.ID, area, p.Decisions[area].Value)
			}
		}
	}
}

func TestAnalyze(t *testing.T) {
	p := project("Detail tagihan pasien dengan notifikasi live. Mendukung SSO.")
	p.Discovery = &core.Discovery{TechNeeds: []string{"offline", "unknown"}}
	// "detail" must not match "ai"; unknown needs are dropped.
	if got := Analyze(p); !slices.Equal(got, []string{"realtime", "offline", "transactions", "enterprise"}) {
		t.Fatalf("needs = %v", got)
	}
}

func TestRecommend(t *testing.T) {
	cases := []struct {
		story   string
		kinds   []string
		profile string
		want    map[string]string
	}{
		{"Platform trading kripto dengan order book", []string{"web", "backend"}, "trading",
			map[string]string{core.TechBackend: "Go + Gin", core.TechCache: "Redis", core.TechDatabase: "PostgreSQL", core.TechRepository: core.RepoSimple}},
		{"Asisten AI untuk merangkum dokumen", []string{"web", "backend"}, "ai-product",
			map[string]string{core.TechBackend: "Python + FastAPI", core.TechFrontend: "React + TypeScript + Vite"}},
		{"Toko online dengan landing page untuk SEO", []string{"web", "backend"}, "saas",
			map[string]string{core.TechFrontend: "Next.js + TypeScript", core.TechArchitecture: core.ArchModular}},
		{"Catatan olahraga harian", []string{"android", "ios", "backend"}, "mobile-app",
			map[string]string{core.TechMobile: "Flutter", core.TechFrontend: ""}},
		{"Form dinamis untuk survei", []string{"web", "android", "backend"}, "saas",
			map[string]string{core.TechDatabase: "MongoDB", core.TechRepository: core.RepoMonorepo, core.TechMobile: "React Native + TypeScript + Expo"}},
		{"Sistem pemesanan dengan microservices dan deploy terpisah", []string{"web", "backend"}, "saas",
			map[string]string{core.TechArchitecture: core.ArchMicroservices}},
	}
	for _, c := range cases {
		s := Recommend(project(c.story, c.kinds...))
		if s.Profile != c.profile || s.Mode != core.StructureNew {
			t.Errorf("%q: profile = %s", c.story, s.Profile)
		}
		for area, want := range c.want {
			if got := s.Choice(area); got != want {
				t.Errorf("%q: %s = %q, want %q", c.story, area, got, want)
			}
		}
		for _, d := range s.Decisions {
			if d.Reason == "" {
				t.Errorf("%q: %s has no reason", c.story, d.Area)
			}
		}
	}
	// Few features: simple architecture.
	p := project("Daftar tamu", "web")
	p.Features = p.Features[:2]
	if s := Recommend(p); s.Choice(core.TechArchitecture) != core.ArchSimple || s.Decision(core.TechBackend) != nil {
		t.Fatalf("stack = %+v", s)
	}
	// No need, no optional infrastructure; latency <100 ms picks Go.
	ticket := Recommend(project("Aplikasi tiket", "web", "backend"))
	for _, area := range []string{core.TechCache, core.TechMessaging, core.TechWorkers} {
		if ticket.Choice(area) != core.TechNone {
			t.Errorf("plain %s = %q", area, ticket.Choice(area))
		}
	}
	for _, d := range ticket.Decisions {
		if d.Source == "" || ticket.DecisionState(d) != core.StateRecommended {
			t.Errorf("%s: source %q state %s", d.Area, d.Source, ticket.DecisionState(d))
		}
	}
	fast := project("Aplikasi tiket", "web", "backend")
	fast.Survey = &core.Survey{Latency: core.SurveyLatency100}
	fast.Survey.FillDefaults()
	if s := Recommend(fast); s.Choice(core.TechBackend) != "Go + Gin" {
		t.Fatalf("latency 100 backend = %s", s.Choice(core.TechBackend))
	}
}

func TestExisting(t *testing.T) {
	p := project("Tambah fitur laporan", "web", "backend")
	p.Structure = &core.Structure{Mode: core.StructureExisting, Components: []core.Component{
		{Kind: "backend", Path: "api", Tech: "Go + Gin", Language: "Go", Evidence: "go.mod"},
		{Kind: "web", Path: "web", Tech: "React", Language: "TypeScript", Evidence: "package.json"},
		{Kind: "database", Path: "api", Tech: "PostgreSQL", Evidence: "go.mod"},
		{Kind: "testing", Path: ".", Tech: "go test", Language: "Go", Evidence: "*_test.go"},
		{Kind: "infra", Path: ".", Tech: "Docker"},
	}}
	s := Recommend(p)
	for area, want := range map[string]string{core.TechFrontend: "React", core.TechBackend: "Go + Gin", core.TechDatabase: "PostgreSQL",
		core.TechTesting: "go test", core.TechRepository: core.RepoSimple, core.TechArchitecture: "Struktur yang ada"} {
		d := s.Decision(area)
		if d == nil || d.Choice() != want || d.Source != core.SourceExisting || s.DecisionState(*d) != core.StateExisting {
			t.Errorf("%s = %+v, want %q existing", area, d, want)
		}
	}
	if d := s.Decision(core.TechBackend); d.Lang() != "Go" || len(d.Evidence) != 1 || d.Evidence[0] != "api/go.mod" || len(d.Alternatives) != 0 {
		t.Errorf("backend = %+v", d)
	}
	// Nothing the code does not show is added without a need: no UI
	// library, cache, deployment or lint tool.
	for _, area := range []string{core.TechUI, core.TechCache, core.TechMessaging, core.TechDeployment, core.TechQuality, core.TechObservability} {
		if d := s.Decision(area); d != nil {
			t.Errorf("%s added: %+v", area, d)
		}
	}
	if s.Decisions[0].Area != core.TechFrontend || NeedsApproval(s) {
		t.Fatalf("stack = %+v", s)
	}
	// Replacing existing code technology is a migration: it needs a reason.
	if err := Override(s, core.TechBackend, "Python + FastAPI", ""); err != ErrExistingReason {
		t.Fatalf("err = %v", err)
	}
	if err := Propose(s, []Change{{Area: core.TechBackend, Value: "Python + FastAPI", Reason: "tim data"}}, "user", "", ""); err != nil {
		t.Fatal(err)
	}
	pr := s.Proposal
	if !slices.ContainsFunc(pr.Issues, func(i core.TechIssue) bool {
		return i.Area == core.TechBackend && strings.Contains(i.Message, "migrasi")
	}) {
		t.Errorf("issues = %+v", pr.Issues)
	}
	// go test was read from the code: it is not replaced, it is reviewed.
	if !slices.ContainsFunc(pr.Review, func(i core.TechIssue) bool { return i.Area == core.TechTesting }) {
		t.Errorf("review = %+v", pr.Review)
	}
	if _, err := Apply(s); err != nil || !NeedsApproval(s) || s.Choice(core.TechTesting) != "go test" {
		t.Fatalf("apply: %v %+v", err, s)
	}
}

// A platform the code has no app for yet is a new part: it is recommended.
func TestExistingNewPlatform(t *testing.T) {
	p := project("Tambah aplikasi mobile", "web", "android", "ios", "backend")
	p.Structure = &core.Structure{Mode: core.StructureExisting, Components: []core.Component{
		{Kind: "backend", Path: ".", Tech: "Laravel + PHP", Language: "PHP", Evidence: "composer.json"},
		{Kind: "web", Path: "resources", Tech: "Vue", Language: "TypeScript", Evidence: "package.json"},
	}}
	s := Recommend(p)
	if d := s.Decision(core.TechMobile); d == nil || d.Source == core.SourceExisting || s.DecisionState(*d) != core.StateRecommended {
		t.Fatalf("mobile = %+v", d)
	}
	if d := s.Decision(core.TechTesting); d == nil || !strings.Contains(d.Choice(), "PHPUnit") {
		t.Errorf("testing = %+v", d)
	}
	if s.Decision(core.TechDatabase) != nil || !NeedsApproval(s) {
		t.Errorf("database guessed behind an existing backend: %+v", s.Decision(core.TechDatabase))
	}
}

func TestPlatformCombinations(t *testing.T) {
	cases := []struct {
		kinds   []string
		want    map[string]string
		missing []string
	}{
		{[]string{"cli"}, map[string]string{core.TechCLI: "Go + Cobra", core.TechTesting: "go test", core.TechQuality: "gofmt + go vet + golangci-lint"},
			[]string{core.TechBackend, core.TechFrontend, core.TechDatabase, core.TechUI, core.TechDeployment, core.TechCache}},
		{[]string{"desktop"}, map[string]string{core.TechDesktop: "Tauri + React + TypeScript", core.TechUI: "Tailwind CSS + shadcn/ui"},
			[]string{core.TechBackend, core.TechDeployment, core.TechAPI}},
		{[]string{"web"}, map[string]string{core.TechDeployment: "Static hosting + CDN", core.TechTesting: "Vitest"},
			[]string{core.TechBackend, core.TechDatabase, core.TechObservability}},
		{[]string{"android", "backend"}, map[string]string{core.TechMobile: "Flutter", core.TechAPI: "REST + OpenAPI"},
			[]string{core.TechFrontend, core.TechUI}},
		{[]string{"web", "backend", "cli"}, map[string]string{core.TechCLI: "Node.js + TypeScript + Commander", core.TechBackend: "NestJS + TypeScript"}, nil},
	}
	for _, c := range cases {
		s := Recommend(project("Aplikasi catatan", c.kinds...))
		for area, want := range c.want {
			if got := s.Choice(area); got != want {
				t.Errorf("%v: %s = %q, want %q", c.kinds, area, got, want)
			}
		}
		for _, area := range c.missing {
			if d := s.Decision(area); d != nil {
				t.Errorf("%v: %s = %q, want none", c.kinds, area, d.Choice())
			}
		}
		if errs := s.Errors(); len(Validate(s)) > 0 && len(errs) > 0 {
			t.Errorf("%v: errors %+v", c.kinds, errs)
		}
	}
	// A native app runs on one platform only.
	both := Recommend(project("Aplikasi catatan", "android", "ios", "backend"))
	if fits(both, core.TechMobile, "Swift + SwiftUI") || slices.Contains(both.Decision(core.TechMobile).Alternatives, "Swift + SwiftUI") {
		t.Error("iOS-only app fits Android and iOS")
	}
	ios := Recommend(project("Aplikasi catatan", "ios", "backend"))
	if !fits(ios, core.TechMobile, "Swift + SwiftUI") {
		t.Error("Swift must fit iOS only")
	}
}

func TestPreferences(t *testing.T) {
	p := project("Aplikasi tiket", "web", "android", "backend")
	p.Survey = &core.Survey{Constraints: "Vue, Laravel, MySQL, React Native"}
	s := Recommend(p)
	for area, want := range map[string]string{core.TechFrontend: "Vue + TypeScript + Vite", core.TechBackend: "Laravel + PHP",
		core.TechDatabase: "MySQL", core.TechMobile: "React Native + TypeScript + Expo"} {
		if d := s.Decision(area); d.Choice() != want || d.Source != core.SourcePreference {
			t.Errorf("%s = %+v, want %q from preference", area, d, want)
		}
	}
	// The UI library follows the preferred framework.
	if d := s.Decision(core.TechUI); d.Choice() != "PrimeVue" || d.Source == core.SourcePreference {
		t.Errorf("ui = %+v", d)
	}
	if got := s.Choice(core.TechTesting); got != "Vitest, PHPUnit" {
		t.Errorf("testing = %q", got)
	}
	// "react native" is mobile, not a React web frontend.
	p.Survey.Constraints = "react native"
	if d := Recommend(p).Decision(core.TechFrontend); d.Source == core.SourcePreference {
		t.Errorf("frontend from %q: %+v", p.Survey.Constraints, d)
	}
	// A prohibition is no preference; the required part still binds.
	p.Survey.Constraints = "Backend wajib Laravel.\nJangan pakai MySQL, tidak boleh Vue"
	s = Recommend(p)
	if d := s.Decision(core.TechBackend); d.Choice() != "Laravel + PHP" || d.Source != core.SourcePreference {
		t.Errorf("backend = %+v", d)
	}
	for _, area := range []string{core.TechDatabase, core.TechFrontend} {
		if d := s.Decision(area); d.Source == core.SourcePreference {
			t.Errorf("%s from a prohibition: %+v", area, d)
		}
	}
}

// The architecture the user picks wins over the one the needs suggest.
func TestSurveyArchitecture(t *testing.T) {
	p := project("Sistem pemesanan dengan microservices dan deploy terpisah", "web", "backend")
	p.Survey = &core.Survey{Architecture: core.SurveyArchModular}
	if d := Recommend(p).Decision(core.TechArchitecture); d.Choice() != core.ArchModular || d.Source != core.SourcePreference {
		t.Errorf("modular = %+v", d)
	}
	p = project("Aplikasi tiket", "web", "backend")
	p.Survey = &core.Survey{Architecture: core.SurveyArchServices, Stage: core.SurveyStagePrototype}
	if d := Recommend(p).Decision(core.TechArchitecture); d.Choice() != core.ArchMicroservices || d.Source != core.SourcePreference {
		t.Errorf("services = %+v", d)
	}
	// Left to Foundry: a prototype stays simple.
	p.Survey = &core.Survey{Stage: core.SurveyStagePrototype}
	if d := Recommend(p).Decision(core.TechArchitecture); d.Choice() != core.ArchSimple || d.Source == core.SourcePreference {
		t.Errorf("auto = %+v", d)
	}
}

// Optional components appear only for a need.
func TestConditional(t *testing.T) {
	cases := []struct {
		story string
		want  map[string]string
	}{
		{"Aplikasi tiket", map[string]string{core.TechCache: core.TechNone, core.TechMessaging: core.TechNone, core.TechWorkers: core.TechNone,
			core.TechAPI: "REST + OpenAPI", core.TechObservability: "Log terstruktur + health check"}},
		{"Chat tim dengan notifikasi live", map[string]string{core.TechCache: "Redis", core.TechMessaging: "Redis Pub/Sub", core.TechAPI: "REST + OpenAPI + WebSocket"}},
		{"Asisten AI untuk merangkum dokumen", map[string]string{core.TechWorkers: "Worker terpisah + antrean", core.TechCache: "Redis", core.TechMessaging: core.TechNone}},
		{"Sistem pemesanan dengan microservices dan deploy terpisah", map[string]string{core.TechMessaging: "NATS", core.TechDeployment: "Kubernetes",
			core.TechObservability: "OpenTelemetry + Prometheus + Grafana"}},
	}
	for _, c := range cases {
		s := Recommend(project(c.story, "web", "backend"))
		for area, want := range c.want {
			if got := s.Choice(area); got != want {
				t.Errorf("%q: %s = %q, want %q", c.story, area, got, want)
			}
		}
		if errs := s.Errors(); len(errs) > 0 {
			t.Errorf("%q: %+v", c.story, errs)
		}
	}
}

// A change re-evaluates what depends on it: recommendations follow, the
// user's own choices are marked for review.
func TestDependencies(t *testing.T) {
	s := Recommend(project("Aplikasi tiket", "web", "backend"))
	if err := Propose(s, []Change{{Area: core.TechFrontend, Value: "Vue + TypeScript + Vite"}}, "user", "", ""); err != nil {
		t.Fatal(err)
	}
	pr := s.Proposal
	ui := slices.IndexFunc(pr.Changes, func(c core.TechChange) bool { return c.Area == core.TechUI })
	if ui < 0 || !pr.Changes[ui].Follow || pr.Changes[ui].To != "PrimeVue" || s.Choice(core.TechUI) != "Tailwind CSS + shadcn/ui" {
		t.Fatalf("proposal = %+v", pr.Changes)
	}
	v, err := Apply(s)
	if err != nil || s.Choice(core.TechUI) != "PrimeVue" || !slices.ContainsFunc(v.Changes, func(c core.TechChange) bool { return c.Area == core.TechUI && c.Follow }) {
		t.Fatalf("apply: %v %+v", err, v)
	}
	if d := s.Decision(core.TechFrontend); s.DecisionState(*d) != core.StateSelected {
		t.Errorf("frontend state = %s", s.DecisionState(*d))
	}
	if err := Approve(s); err != nil {
		t.Fatal(err)
	}
	if d := s.Decision(core.TechFrontend); s.DecisionState(*d) != core.StateApproved {
		t.Errorf("frontend state = %s", s.DecisionState(*d))
	}
	// The user's UI choice is kept but reviewed when the frontend changes.
	if err := Propose(s, []Change{{Area: core.TechUI, Value: "Vuetify"}}, "user", "", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(s); err != nil {
		t.Fatal(err)
	}
	if err := Propose(s, []Change{{Area: core.TechFrontend, Value: "React + TypeScript + Vite"}}, "user", "", ""); err != nil {
		t.Fatal(err)
	}
	if len(s.Proposal.Review) != 1 || s.Proposal.Review[0].Area != core.TechUI || len(s.Proposal.Changes) != 1 {
		t.Fatalf("proposal = %+v", s.Proposal)
	}
	if !slices.ContainsFunc(s.Proposal.Issues, func(i core.TechIssue) bool { return i.Area == core.TechUI && i.Level == "error" }) {
		t.Fatalf("Vuetify on React: issues %+v", s.Proposal.Issues)
	}
	if _, err := Apply(s); err != nil {
		t.Fatal(err)
	}
	d := s.Decision(core.TechUI)
	if d.Choice() != "Vuetify" || d.Review == "" || s.DecisionState(*d) != core.StateConflict {
		t.Fatalf("ui = %+v (%s)", d, s.DecisionState(*d))
	}
	if err := Approve(s); err == nil || !strings.Contains(err.Error(), "ditinjau") {
		t.Fatalf("approve with review err = %v", err)
	}
	if err := ConfirmReview(s, core.TechUI); err != nil || s.Decision(core.TechUI).Review != "" {
		t.Fatal(err)
	}
	if err := ConfirmReview(s, core.TechUI); err == nil {
		t.Fatal("nothing to confirm")
	}
	if err := Approve(s); err == nil { // the conflict remains
		t.Fatal("approve with conflict")
	}
	if err := Propose(s, []Change{{Area: core.TechUI, Value: ""}}, "user", "", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(s); err != nil || Approve(s) != nil {
		t.Fatalf("back to the recommendation: %+v", s.Issues)
	}
}

// Stacks saved before sources, states and the new areas still load and work.
func TestOldStack(t *testing.T) {
	var s core.TechStack
	old := `{"mode":"new","profile":"saas","platform":{"web":true,"mobile":true,"backend":true},"approved":true,
		"decisions":[{"area":"frontend","recommended":"React + TypeScript + Vite","language":"TypeScript","reason":"x"},
		{"area":"mobile","recommended":"Flutter","language":"Dart","reason":"x"},
		{"area":"backend","recommended":"NestJS + TypeScript","language":"TypeScript","reason":"x","override":{"value":"Go + Gin","language":"Go"}},
		{"area":"database","recommended":"PostgreSQL","reason":"x"},{"area":"cache","recommended":"Redis","reason":"x"},
		{"area":"messaging","recommended":"Redis Streams","reason":"x"},{"area":"architecture","recommended":"Modular Monolith","reason":"x"}]}`
	if err := json.Unmarshal([]byte(old), &s); err != nil {
		t.Fatal(err)
	}
	for _, d := range s.Decisions {
		if st := s.DecisionState(d); st != core.StateApproved {
			t.Errorf("%s = %s", d.Area, st)
		}
	}
	if errs := s.Errors(); len(Validate(&s)) > 0 && len(errs) > 0 {
		t.Errorf("errors = %+v", errs)
	}
	if err := Propose(&s, []Change{{Area: core.TechDatabase, Value: "MySQL"}}, "user", "", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(&s); err != nil || s.Choice(core.TechDatabase) != "MySQL" || s.Choice(core.TechCache) != "Redis" || s.Version() != 2 {
		t.Fatalf("apply on old stack: %v %+v", err, s)
	}
	// A re-analysis adds the new areas and keeps the user's choices.
	next := Recommend(project("Aplikasi tiket", "web", "android", "backend"))
	Carry(&s, next)
	if next.Choice(core.TechBackend) != "Go + Gin" || next.Choice(core.TechDatabase) != "MySQL" || next.Decision(core.TechTesting) == nil {
		t.Fatalf("carry = %+v", next.Decisions)
	}
	if got := next.Choice(core.TechTesting); got != "Vitest, go test, flutter_test" && !strings.Contains(got, "go test") {
		t.Errorf("testing follows the carried backend: %q", got)
	}
}

func TestOverride(t *testing.T) {
	s := Recommend(project("Aplikasi tiket", "web", "backend"))
	if err := Override(s, core.TechBackend, "Laravel", "tim PHP"); err != nil {
		t.Fatal(err)
	}
	if d := s.Decision(core.TechBackend); d.Choice() != "Laravel" || d.Lang() != "PHP" || d.Recommended != "NestJS + TypeScript" {
		t.Fatalf("backend = %+v", d)
	}
	if err := Override(s, core.TechCache, "Memcached", ""); err == nil {
		t.Fatal("cache must be from the catalog")
	}
	if err := Override(s, core.TechMobile, "Flutter", ""); err == nil {
		t.Fatal("mobile is not part of the product")
	}
	// The recommended value clears the override.
	if err := Override(s, core.TechBackend, "nestjs + typescript", ""); err != nil || s.Decision(core.TechBackend).Override != nil {
		t.Fatalf("err = %v, override = %+v", err, s.Decision(core.TechBackend).Override)
	}
	next := Recommend(project("Aplikasi tiket", "web", "backend"))
	_ = Override(s, core.TechDatabase, "MySQL", "ekosistem lama")
	Carry(s, next)
	if next.Choice(core.TechDatabase) != "MySQL" || next.Decision(core.TechDatabase).Override.Reason != "ekosistem lama" {
		t.Fatalf("carry = %+v", next.Decision(core.TechDatabase))
	}
}
