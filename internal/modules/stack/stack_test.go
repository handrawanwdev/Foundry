package stack

import (
	"encoding/json"
	"errors"
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

// withDecisions is a new project's stack after the AI plan decided it.
func withDecisions(p *core.Project, choices map[string]string) *core.TechStack {
	s := Baseline(p)
	for area, v := range choices {
		s.Decisions = append(s.Decisions, core.TechDecision{Area: area, Recommended: v, Reason: "rencana AI", Source: core.SourceAI})
	}
	sortDecisions(s)
	return s
}

// Foundry recommends no technology of its own: a new project starts empty
// whatever the story says.
func TestBaselineHasNoDefaults(t *testing.T) {
	for _, story := range []string{"Aplikasi tiket", "Platform trading kripto realtime dengan upload dokumen", "Profil perusahaan dengan SEO"} {
		s := Baseline(project(story, "web", "backend"))
		if len(s.Decisions) != 0 || s.Mode != core.StructureNew {
			t.Fatalf("%q: decisions = %+v", story, s.Decisions)
		}
		if !s.Platform.Web || !s.Platform.Backend {
			t.Fatalf("platform = %+v", s.Platform)
		}
	}
}

// Needs come from the discovery analyst, not from keywords in the story.
func TestAnalyzeReadsDiscoveryOnly(t *testing.T) {
	p := project("Platform trading realtime dengan pembayaran", "web", "backend")
	if got := Analyze(p); len(got) != 0 {
		t.Fatalf("needs guessed from keywords: %v", got)
	}
	p.Discovery = &core.Discovery{TechNeeds: []string{" Realtime ", "geo_tracking", "realtime"}}
	if got := Analyze(p); !slices.Equal(got, []string{"realtime", "geo_tracking"}) {
		t.Fatalf("needs = %v", got)
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
	s := Baseline(p)
	for area, want := range map[string]string{core.TechFrontend: "React", core.TechBackend: "Go + Gin", core.TechDatabase: "PostgreSQL",
		core.TechTesting: "go test", core.TechArchitecture: "Struktur yang ada"} {
		d := s.Decision(area)
		if d == nil || d.Choice() != want || d.Source != core.SourceExisting || s.DecisionState(*d) != core.StateExisting {
			t.Errorf("%s = %+v, want %q existing", area, d, want)
		}
	}
	if d := s.Decision(core.TechBackend); d.Lang() != "Go" || len(d.Evidence) != 1 || d.Evidence[0] != "api/go.mod" {
		t.Errorf("backend = %+v", d)
	}
	// Nothing the code does not show is added.
	for _, area := range []string{core.TechUI, core.TechCache, core.TechRepository, core.TechDeployment, core.TechQuality} {
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
	if !slices.ContainsFunc(s.Proposal.Issues, func(i core.TechIssue) bool {
		return i.Area == core.TechBackend && strings.Contains(i.Message, "migrasi")
	}) {
		t.Errorf("issues = %+v", s.Proposal.Issues)
	}
	// Foundry derives no other area from the change.
	if len(s.Proposal.Changes) != 1 {
		t.Errorf("changes = %+v", s.Proposal.Changes)
	}
	if _, err := Apply(s); err != nil || !NeedsApproval(s) || s.Choice(core.TechTesting) != "go test" {
		t.Fatalf("apply: %v %+v", err, s)
	}
}

func TestOldStack(t *testing.T) {
	var s core.TechStack
	old := `{"mode":"new","profile":"saas","platform":{"web":true,"mobile":true,"backend":true},"approved":true,
		"decisions":[{"area":"frontend","recommended":"React + TypeScript + Vite","language":"TypeScript","reason":"x","source":"profile"},
		{"area":"mobile","recommended":"Flutter","language":"Dart","reason":"x"},
		{"area":"backend","recommended":"NestJS + TypeScript","language":"TypeScript","reason":"x","override":{"value":"Go + Gin","language":"Go"}},
		{"area":"database","recommended":"PostgreSQL","reason":"x"},{"area":"cache","recommended":"Redis","reason":"x"},
		{"area":"architecture","recommended":"Modular Monolith","reason":"x"}]}`
	if err := json.Unmarshal([]byte(old), &s); err != nil {
		t.Fatal(err)
	}
	for _, d := range s.Decisions {
		if st := s.DecisionState(d); st != core.StateApproved {
			t.Errorf("%s = %s", d.Area, st)
		}
	}
	if issues := Validate(&s); len(issues) != 0 {
		t.Errorf("issues = %+v", issues)
	}
	if err := Propose(&s, []Change{{Area: core.TechDatabase, Value: "MySQL"}}, "user", "", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(&s); err != nil || s.Choice(core.TechDatabase) != "MySQL" || s.Choice(core.TechCache) != "Redis" || s.Version() != 2 {
		t.Fatalf("apply on old stack: %v %+v", err, s)
	}
	// Reading the facts again keeps the user's choices.
	next := Baseline(project("Aplikasi tiket", "web", "android", "backend"))
	Carry(&s, next)
	if next.Choice(core.TechBackend) != "Go + Gin" || next.Choice(core.TechDatabase) != "MySQL" {
		t.Fatalf("carry = %+v", next.Decisions)
	}
}

// Any technology and any area are accepted: Foundry has no allowed list.
func TestOverride(t *testing.T) {
	s := withDecisions(project("Aplikasi tiket", "web", "backend"), map[string]string{
		core.TechBackend: "NestJS + TypeScript", core.TechArchitecture: "Hexagonal modular monolith"})
	for area, v := range map[string]string{
		core.TechBackend: "Gleam + Wisp", core.TechCache: "Memcached", core.TechArchitecture: "Event-sourced cells",
		core.TechRepository: "Polyrepo per bounded context", "state_management": "XState"} {
		if err := Override(s, area, v, "pilihan tim"); err != nil {
			t.Fatalf("%s: %v", area, err)
		}
		if s.Choice(area) != v {
			t.Fatalf("%s = %q", area, s.Choice(area))
		}
	}
	if err := Override(s, "Bad Area!", "x", ""); err == nil {
		t.Fatal("invalid area name accepted")
	}
	// The recommended value clears the override.
	if err := Override(s, core.TechBackend, "nestjs + typescript", ""); err != nil || s.Decision(core.TechBackend).Override != nil {
		t.Fatalf("err = %v, override = %+v", err, s.Decision(core.TechBackend).Override)
	}
	next := Baseline(project("Aplikasi tiket", "web", "backend"))
	_ = Override(s, core.TechDatabase, "MySQL", "ekosistem lama")
	Carry(s, next)
	if d := next.Decision(core.TechDatabase); d == nil || d.Choice() != "MySQL" || d.Override.Reason != "ekosistem lama" {
		t.Fatalf("carry = %+v", d)
	}
}

// Validation checks no technology compatibility list: only a decision
// without a choice and a migration of the code's technology.
func TestValidate(t *testing.T) {
	s := withDecisions(project("Aplikasi tiket", "web", "backend"), map[string]string{
		core.TechFrontend: "Elm", core.TechUI: "Tailwind CSS + shadcn/ui", core.TechAPI: "tRPC", core.TechBackend: "Go + Gin",
		core.TechDeployment: "Static hosting + CDN", core.TechArchitecture: "Cell-based", core.TechRepository: "Polyrepo"})
	if issues := Validate(s); len(issues) != 0 {
		t.Fatalf("Foundry judged technologies itself: %+v", issues)
	}
	s.Decisions = append(s.Decisions, core.TechDecision{Area: core.TechSearch})
	if issues := Validate(s); len(issues) != 1 || issues[0].Level != "error" || issues[0].Area != core.TechSearch {
		t.Fatalf("empty choice = %+v", issues)
	}
}

func TestProposeApplyHistory(t *testing.T) {
	s := withDecisions(project("Aplikasi tiket", "web", "backend"), map[string]string{
		core.TechBackend: "NestJS + TypeScript", core.TechTesting: "Vitest"})
	if err := Propose(s, []Change{{Area: core.TechBackend, Value: "nestjs + typescript"}}, "user", "", ""); !errors.Is(err, ErrNoChange) {
		t.Fatalf("same value err = %v", err)
	}
	if err := Propose(s, []Change{{Area: core.TechBackend, Value: "Go + Gin", Reason: "latency"}}, "user", "", ""); err != nil {
		t.Fatal(err)
	}
	if s.Choice(core.TechBackend) != "NestJS + TypeScript" || s.State() != core.TechReview {
		t.Fatalf("proposal applied early: %+v", s)
	}
	v, err := Apply(s)
	if err != nil {
		t.Fatal(err)
	}
	// A stack without history gets its old state as v1 first; no area
	// follows the change on its own.
	if v.Version != 2 || len(s.History) != 2 || len(v.Changes) != 1 || v.Changes[0].From != "NestJS + TypeScript" ||
		v.Changes[0].To != "Go + Gin" || s.Choice(core.TechTesting) != "Vitest" {
		t.Fatalf("version = %+v", v)
	}
	if err := Approve(s); err != nil || s.History[1].Status != core.TechApproved || !s.Approved {
		t.Fatalf("approve: %v %+v", err, s.History[1])
	}
	// Back to the recommendation is a change too; after approval it is "changed".
	if err := Propose(s, []Change{{Area: core.TechBackend}}, "user", "", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(s); err != nil || s.State() != core.TechChanged || s.Approved || s.Decision(core.TechBackend).Override != nil {
		t.Fatalf("revert: %v %+v", err, s)
	}
}

// A new project's technology is not approved before anyone designed it.
func TestApproveNeedsADesignedStack(t *testing.T) {
	s := Baseline(project("Aplikasi tiket", "web", "backend"))
	if err := Approve(s); !errors.Is(err, ErrNotDesigned) {
		t.Fatalf("err = %v", err)
	}
}
