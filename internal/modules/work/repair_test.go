package work

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/handrawanw/foundry/internal/core"
)

func TestRepairDesignAddsMissingPagesAndLinksFlows(t *testing.T) {
	p := &core.Project{Name: "app",
		Features: []core.Feature{
			{ID: "FEAT-001", Name: "Login", Platforms: []string{"web"}},
			{ID: "FEAT-010", Name: "Konsol RBAC", Description: "Atur peran", Platforms: []string{"web"},
				SubFeatures: []core.SubFeature{{ID: "FEAT-010.1", Name: "Daftar peran"}}},
			{ID: "FEAT-020", Name: "Worker", Platforms: []string{"backend"}},
		},
		Flows: []core.Flow{
			{ID: "FLOW-001", Name: "Masuk", FeatureID: "FEAT-001", PageIDs: []string{"PAGE-001"}},
			{ID: "FLOW-010", Name: "Kelola peran", FeatureID: "FEAT-010", Role: "Admin"},
		},
		Design: &core.Design{System: &core.DesignSystem{}, Pages: []core.Page{{ID: "PAGE-001", Name: "Login", Platform: "web", FeatureID: "FEAT-001", Route: "/login"}}},
	}
	pages, flows := RepairDesign(p)
	if pages != 1 || flows != 1 {
		t.Fatalf("pages=%d flows=%d", pages, flows)
	}
	pg := p.Design.Pages[1]
	if pg.FeatureID != "FEAT-010" || pg.ID != "PAGE-002" || pg.Role != "Admin" || pg.Route != "/konsol-rbac" {
		t.Fatalf("page = %+v", pg)
	}
	if got := p.Flows[1].PageIDs; len(got) != 1 || got[0] != "PAGE-002" {
		t.Fatalf("flow pages = %v", got)
	}
	if err := core.ValidateDesign(p.Design.Pages[1:], p.Design.Pages); err != nil {
		t.Fatal(err)
	}
	for _, b := range Blockers(p) {
		if b.PageID == "PAGE-002" {
			t.Errorf("blocker: %s", b.Text)
		}
	}
	if pages, flows = RepairDesign(p); pages+flows != 0 {
		t.Fatalf("not idempotent: %d %d", pages, flows)
	}
}

func TestFeatureCarriedByFlowPageNeedsNoOwnPage(t *testing.T) {
	p := &core.Project{Name: "app",
		Features: []core.Feature{
			{ID: "FEAT-001", Name: "Tampilan", Platforms: []string{"web"}},
			{ID: "FEAT-002", Name: "Aksi", Platforms: []string{"web"}},
			{ID: "FEAT-003", Name: "Gate CI", Platforms: []string{"web"}},
		},
		Flows: []core.Flow{
			{ID: "FLOW-001", FeatureID: "FEAT-001", PageIDs: []string{"PAGE-001"}},
			{ID: "FLOW-002", FeatureID: "FEAT-002", PageIDs: []string{"PAGE-001"}},
		},
		Design: &core.Design{System: &core.DesignSystem{}, Pages: []core.Page{{ID: "PAGE-001", Name: "Counter", Platform: "web", FeatureID: "FEAT-001", Route: "/"}}},
	}
	if got := PageIDsOf(p, "FEAT-002"); len(got) != 1 || got[0] != "PAGE-001" {
		t.Fatalf("PageIDsOf = %v", got)
	}
	for _, f := range UIFeatures(p) {
		if f.ID == "FEAT-003" {
			t.Fatal("feature without a flow must not require a page")
		}
	}
	if pages, flows := RepairDesign(p); pages+flows != 0 {
		t.Fatalf("repair added %d pages, %d flows", pages, flows)
	}
}

func TestSmokeCriteriaOnlyForExistingApps(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"dependencies":{"react":"1"},"scripts":{"dev":"vite"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, c := range SmokeCriteriaFor(dir) {
		if strings.Contains(c, "API") || strings.Contains(c, "Worker") {
			t.Fatalf("client-only app must not require %q", c)
		}
	}
}
