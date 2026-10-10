package stack

import (
	"errors"
	"slices"
	"testing"

	"github.com/handrawanw/foundry/internal/core"
)

// Infrastructure is only recommended for a need; otherwise "Tidak perlu".
func TestRecommendInfra(t *testing.T) {
	plain := Recommend(project("Aplikasi tiket", "web", "backend"))
	for area, want := range map[string]string{
		core.TechMessaging: "Redis Streams", core.TechStorage: core.TechNone, core.TechSearch: core.TechNone,
		core.TechWorkers: "Job in-process", core.TechDeployment: "Docker Compose (satu server)",
		core.TechObservability: "Log terstruktur + health check",
	} {
		if got := plain.Choice(area); got != want {
			t.Errorf("plain %s = %q, want %q", area, got, want)
		}
	}
	if issues := Validate(plain); len(issues) != 0 {
		t.Errorf("plain issues = %+v", issues)
	}
	trading := Recommend(project("Platform trading kripto realtime dengan upload dokumen KYC dan pencarian teks", "web", "backend"))
	for area, want := range map[string]string{
		core.TechMessaging: "Redis Streams", core.TechStorage: "S3-compatible (S3 / MinIO)", core.TechSearch: "PostgreSQL full-text search",
		core.TechWorkers: "Worker terpisah + antrean", core.TechObservability: "OpenTelemetry + Prometheus + Grafana",
	} {
		if got := trading.Choice(area); got != want {
			t.Errorf("trading %s = %q, want %q", area, got, want)
		}
	}
	web := Recommend(project("Profil perusahaan", "web"))
	if web.Choice(core.TechDeployment) != "Static hosting + CDN" || web.Decision(core.TechObservability) != nil {
		t.Errorf("web only = %+v", web.Decisions)
	}
	if d := plain.Decision(core.TechBackend); len(d.Alternatives) != 2 || slices.Contains(d.Alternatives, d.Recommended) {
		t.Errorf("alternatives = %v", d.Alternatives)
	}
}

func TestValidate(t *testing.T) {
	s := Recommend(project("Aplikasi tiket", "web", "backend"))
	must := func(area, value string) {
		t.Helper()
		if err := Override(s, area, value, ""); err != nil {
			t.Fatal(err)
		}
	}
	must(core.TechSearch, "PostgreSQL full-text search")
	must(core.TechDatabase, "MySQL")
	must(core.TechDeployment, "Static hosting + CDN")
	must(core.TechCache, core.CacheNone)
	must(core.TechMessaging, core.TechNone)
	must(core.TechWorkers, "Worker terpisah + antrean")
	must(core.TechBackend, "Rust + Axum")
	var errs, unknown []string
	for _, i := range Validate(s) {
		if i.Level == "error" {
			errs = append(errs, i.Area)
		}
		if i.Area == core.TechBackend {
			unknown = append(unknown, i.Message)
		}
	}
	if !slices.Equal(errs, []string{core.TechSearch, core.TechDeployment, core.TechWorkers}) || len(unknown) != 1 {
		t.Fatalf("errors = %v, backend = %v", errs, unknown)
	}
	Refresh(s)
	if err := Approve(s); err == nil {
		t.Fatal("approve with errors must fail")
	}
}

func TestProposeApplyHistory(t *testing.T) {
	s := Recommend(project("Aplikasi tiket", "web", "backend"))
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
	// A stack without history gets its old state as v1 first.
	if v.Version != 2 || len(s.History) != 2 || s.History[0].Decisions[1].Choice() != "NestJS + TypeScript" ||
		len(v.Changes) != 1 || v.Changes[0].From != "NestJS + TypeScript" || v.Changes[0].To != "Go + Gin" || !v.Design {
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
