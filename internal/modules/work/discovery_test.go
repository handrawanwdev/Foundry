package work

import (
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
)

const thinDiscovery = `{
  "name": "Klinik Sehat",
  "discovery": {"summary": "Aplikasi klinik", "business": "Klinik pratama", "users": ["Pasien", "Admin"],
    "roles": [{"name": "Pasien", "description": "membuat janji", "platform": "android"},
              {"name": "Admin", "description": "kelola jadwal", "platform": "web"}],
    "data": ["Appointment: id, pasien, dokter, waktu"], "integrations": [], "security": ["Login wajib"],
    "workflows": ["Pasien membuat janji"], "tech_needs": ["transactions"]},
  "requirements": [{"id": "R1", "text": "Pasien dapat membuat janji temu", "priority": "MUST"},
                   {"id": "R2", "text": "Admin dapat mengelola jadwal dokter", "priority": "MUST"}],
  "features": [{"id": "F1", "name": "Janji Temu", "description": "Pasien membuat janji", "requirement_ids": ["R1"],
                "platforms": ["android", "backend"], "sub_features": [{"name": "Cari dokter"}, {"name": "Buat janji"}]},
               {"id": "F2", "name": "Jadwal Dokter", "description": "Admin kelola jadwal", "requirement_ids": ["R2"],
                "platforms": ["web", "backend"], "sub_features": [{"name": "Tambah jadwal"}]}],
  "flows": [{"id": "FL1", "name": "Buat janji", "feature_id": "F1", "role": "Pasien", "steps": ["Pilih dokter", "Konfirmasi"]}],
  "platform": {"targets": [{"kind": "web", "audience": ["Admin"], "rationale": "dashboard"},
                           {"kind": "android", "audience": ["Pasien"], "rationale": "HP"},
                           {"kind": "backend", "audience": [], "rationale": "API"}],
               "reason": "Pasien memakai HP, admin memakai web."}
}`

const richDiscovery = `{
  "name": "Klinik Sehat",
  "discovery": {"summary": "Aplikasi klinik", "business": "Klinik pratama", "users": ["Pasien"],
    "roles": [{"name": "Pasien", "description": "janji", "platform": "android"}],
    "data": ["Appointment: id, pasien_id, waktu"], "integrations": [], "security": ["Login"],
    "workflows": ["Buat janji"], "tech_needs": ["transactions", "files"]},
  "blueprint": {
    "overview": {"goal": "Jadwal klinik tanpa antrian kertas", "users": ["Pasien"], "capabilities": ["Janji"], "core_domain": "appointment"},
    "platform": {"reason": "HP pasien", "devices": ["android", "backend"], "offline": "none", "realtime": "none"},
    "architecture": {"pattern": "toy", "reason": "AI salah", "tradeoff": "sederhana"},
    "modules": [
      {"id": "identity", "responsibility": "Auth", "data": ["User"], "interface": ["POST /auth/login"], "depends_on": ["janji-temu"]},
      {"id": "janji-temu", "responsibility": "Janji", "data": ["Appointment"], "interface": ["POST /appointments"], "depends_on": ["identity"]},
      {"id": "janji-temu", "responsibility": "duplikat"}
    ],
    "database": {"entities": [{"name": "Appointment", "fields": ["id", "waktu"], "module": "janji-temu"}]},
    "storage": {"strategy": "object", "types": ["attachment"]},
    "workers": [{"name": "email", "trigger": "queue", "job": "Kirim konfirmasi"}]
  },
  "requirements": [{"id": "R1", "text": "Pasien membuat janji", "priority": "MUST"}],
  "features": [{"id": "F1", "name": "Janji Temu", "description": "Janji", "requirement_ids": ["R1"],
                "platforms": ["android", "backend"], "sub_features": [{"name": "Buat janji"}, {"name": "Lihat janji"}]}],
  "flows": [{"id": "FL1", "name": "Buat janji", "feature_id": "F1", "role": "Pasien", "steps": ["Pilih", "Kirim"]}],
  "platform": {"targets": [{"kind": "android", "audience": ["Pasien"], "rationale": "HP"},
                           {"kind": "backend", "audience": [], "rationale": "API"}], "reason": "HP"}
}`

func TestApplyDiscoveryWithoutBlueprint(t *testing.T) {
	p := &core.Project{Story: "Aplikasi klinik"}
	if err := ApplyDiscovery(p, thinDiscovery); err != nil {
		t.Fatal(err)
	}
	if p.Name != "Klinik Sehat" || p.Blueprint != nil || len(p.Features) != 2 {
		t.Fatalf("name=%s blueprint=%v feats=%d", p.Name, p.Blueprint, len(p.Features))
	}
}

func TestApplyDiscoveryNormalizesBlueprint(t *testing.T) {
	p := &core.Project{Story: "Aplikasi klinik"}
	if err := ApplyDiscovery(p, richDiscovery); err != nil {
		t.Fatal(err)
	}
	b := p.Blueprint
	if b == nil || len(b.Modules) != 2 {
		t.Fatalf("modules = %+v", b)
	}
	id := b.Module("identity")
	jt := b.Module("janji-temu")
	if id == nil || jt == nil {
		t.Fatalf("missing module: %+v", b.Modules)
	}
	// Cycle identity ↔ janji-temu: one edge may stay, the closing edge is dropped.
	if containsStr(id.DependsOn, "janji-temu") && containsStr(jt.DependsOn, "identity") {
		t.Fatalf("cycle kept: identity=%v janji=%v", id.DependsOn, jt.DependsOn)
	}
	if b.Overview.CoreDomain != "appointment" || b.Storage.Strategy != "object" {
		t.Fatalf("overview/storage = %+v %+v", b.Overview, b.Storage)
	}
}

func TestCompleteBlueprintFillsStackAndRules(t *testing.T) {
	p := &core.Project{Story: "Aplikasi klinik", Name: "Klinik"}
	if err := ApplyDiscovery(p, thinDiscovery); err != nil {
		t.Fatal(err)
	}
	p.Stack = &core.TechStack{
		Needs: []string{"transactions"},
		Decisions: []core.TechDecision{
			{Area: core.TechBackend, Recommended: "NestJS + TypeScript"},
			{Area: core.TechDatabase, Recommended: "PostgreSQL"},
			{Area: core.TechCache, Recommended: core.CacheNone},
			{Area: core.TechArchitecture, Recommended: core.ArchModular, Reason: "beberapa domain"},
			{Area: core.TechFrontend, Recommended: "React + TypeScript + Vite"},
		},
	}
	CompleteBlueprint(p)
	b := p.Blueprint
	if b == nil {
		t.Fatal("nil blueprint")
	}
	if b.Architecture.Pattern != core.ArchModular {
		t.Fatalf("pattern = %s", b.Architecture.Pattern)
	}
	if b.Database.Engine != "PostgreSQL" || b.Database.ORM != "Prisma" || b.Database.PK == "" {
		t.Fatalf("db = %+v", b.Database)
	}
	if b.Module("identity") == nil || b.Module("janji-temu") == nil {
		t.Fatalf("modules = %+v", b.Modules)
	}
	if b.Auth.Hashing != "Argon2id" || b.Authz.Model != "RBAC" {
		t.Fatalf("auth = %+v %+v", b.Auth, b.Authz)
	}
	if b.Frontend.State != "Zustand" || b.Coding.API.Style != "REST" {
		t.Fatalf("frontend/api = %+v %+v", b.Frontend, b.Coding.API)
	}
	if len(b.AIGuidelines) == 0 || len(b.Coding.Backend) == 0 {
		t.Fatal("missing rules")
	}
	if b.Storage.Strategy != "none" {
		t.Fatalf("storage = %s", b.Storage.Strategy)
	}
}

func TestCompleteBlueprintKeepsAIAndOverridesPattern(t *testing.T) {
	p := &core.Project{Story: "klinik"}
	if err := ApplyDiscovery(p, richDiscovery); err != nil {
		t.Fatal(err)
	}
	p.Stack = &core.TechStack{Decisions: []core.TechDecision{
		{Area: core.TechArchitecture, Recommended: core.ArchModular, Reason: "Foundry"},
		{Area: core.TechDatabase, Recommended: "PostgreSQL"},
		{Area: core.TechBackend, Recommended: "Go + Gin"},
	}}
	CompleteBlueprint(p)
	if p.Blueprint.Architecture.Pattern != core.ArchModular {
		t.Fatalf("pattern = %s", p.Blueprint.Architecture.Pattern)
	}
	if p.Blueprint.Overview.Goal != "Jadwal klinik tanpa antrian kertas" {
		t.Fatalf("goal overwritten: %s", p.Blueprint.Overview.Goal)
	}
	if p.Blueprint.Database.ORM != "GORM" { // empty ORM filled from Go stack
		t.Fatalf("orm = %s", p.Blueprint.Database.ORM)
	}
	if p.Blueprint.Storage.Strategy != "object" {
		t.Fatalf("storage dropped: %s", p.Blueprint.Storage.Strategy)
	}
}

func TestDiscoveryPromptAsksForBlueprint(t *testing.T) {
	p := &core.Project{Story: "Aplikasi klinik pasien buat janji"}
	got := DiscoveryPrompt(p)
	for _, want := range []string{"discovery analyst", "blueprint", "modules", "tech_needs", "files", "FOUNDRY DESIGN PRINCIPLES", "embedded", "artifacts", p.Story} {
		if !strings.Contains(got, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
}
