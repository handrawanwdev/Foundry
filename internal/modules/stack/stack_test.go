package stack

import (
	"slices"
	"testing"

	"github.com/handrawanw/foundry/internal/core"
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
		for _, area := range []string{core.TechFrontend, core.TechMobile, core.TechDesktop, core.TechBackend, core.TechDatabase, core.TechCache, core.TechArchitecture} {
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
}

func TestExisting(t *testing.T) {
	p := project("Tambah fitur laporan", "web", "backend")
	p.Structure = &core.Structure{Mode: core.StructureExisting, Components: []core.Component{
		{Kind: "backend", Path: "api", Tech: "Go"}, {Kind: "web", Path: "web", Tech: "React"}, {Kind: "infra", Path: ".", Tech: "Docker"},
	}}
	s := Recommend(p)
	if s.Mode != core.StructureExisting || s.Choice(core.TechFrontend) != "React" || s.Choice(core.TechBackend) != "Go" ||
		s.Decisions[0].Area != core.TechFrontend || s.Decision(core.TechRepository) != nil {
		t.Fatalf("stack = %+v", s)
	}
	if err := Override(s, core.TechBackend, "Python + FastAPI", ""); err != ErrExisting {
		t.Fatalf("err = %v", err)
	}
}

func TestOverride(t *testing.T) {
	s := Recommend(project("Aplikasi tiket", "web", "backend"))
	if err := Override(s, core.TechBackend, "Laravel", "tim PHP"); err != nil {
		t.Fatal(err)
	}
	if d := s.Decision(core.TechBackend); d.Choice() != "Laravel" || d.Lang() != "" || d.Recommended != "NestJS + TypeScript" {
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

func TestStructure(t *testing.T) {
	feats := []core.Feature{{Name: "Tiket"}, {Name: "Pembayaran & Invoice"}, {Name: "Tiket"}}
	s := Recommend(project("Aplikasi tiket", "web", "android", "backend"))
	tpl, layout, _ := Structure(s, feats)
	if tpl != "monorepo" || !slices.Equal(layout, []string{"apps/web/", "apps/mobile/", "services/api/", "services/api/modules/tiket/",
		"services/api/modules/pembayaran-invoice/", "packages/ui/", "packages/types/", "docs/"}) {
		t.Fatalf("%s %v", tpl, layout)
	}
	_ = Override(s, core.TechArchitecture, core.ArchSimple, "")
	if _, layout, _ = Structure(s, feats); slices.Contains(layout, "services/api/modules/tiket/") {
		t.Fatalf("modules = %v", layout)
	}
	_ = Override(s, core.TechArchitecture, core.ArchModular, "")
	_ = Override(s, core.TechRepository, core.RepoSimple, "")
	if tpl, layout, _ = Structure(s, feats); tpl != "modular" || layout[2] != "apps/api/" {
		t.Fatalf("%s %v", tpl, layout)
	}
	_ = Override(s, core.TechArchitecture, core.ArchMicroservices, "")
	if tpl, layout, _ = Structure(s, feats); tpl != "microservices" || !slices.Contains(layout, "services/tiket/") {
		t.Fatalf("%s %v", tpl, layout)
	}
	calc := project("Kalkulator", "web")
	calc.Features = nil
	web := Recommend(calc)
	if _, layout, _ = Structure(web, nil); !slices.Equal(layout, []string{"src/", "tests/", "docs/"}) {
		t.Fatalf("%v", layout)
	}
	_ = Override(s, core.TechArchitecture, core.ArchModular, "")
	_ = Override(s, core.TechRepository, core.RepoMonorepo, "")
	bp := &core.Blueprint{Modules: []core.Module{{ID: "identity"}, {ID: "appointment"}}}
	if _, layout, _ = StructureFrom(s, feats, bp); !slices.Contains(layout, "services/api/modules/identity/") ||
		!slices.Contains(layout, "services/api/modules/appointment/") || slices.Contains(layout, "services/api/modules/tiket/") {
		t.Fatalf("blueprint modules = %v", layout)
	}
}
