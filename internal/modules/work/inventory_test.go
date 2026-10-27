package work

import (
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/spec"
)

// inventoryDiscovery is the discovery answer the model is expected to write
// for the Sistem Manajemen Inventaris Gudang scenario: CRUD barang/kategori,
// transaksi masuk/keluar atomik, stok tidak negatif, permission per peran,
// riwayat + filter, peringatan stok minimum dan laporan periode. It is a
// fixture that proves the pipeline turns those needs into traceable,
// consistent, verifiable artifacts.
const inventoryDiscovery = `{
  "name": "Inventori Gudang",
  "discovery": {
    "summary": "Manajemen inventaris gudang",
    "business": "Gudang mencatat barang, stok, transaksi masuk/keluar, dan laporan persediaan",
    "users": ["Gudang", "Manajer"],
    "roles": [
      {"name": "Gudang", "description": "mencatat barang masuk/keluar", "platform": "web"},
      {"name": "Manajer", "description": "melihat laporan persediaan", "platform": "web"}
    ],
    "data": ["Barang: id, nama, kategori_id, stok_minimum", "Stock: barang_id, jumlah"],
    "integrations": [], "security": ["Login wajib", "Akses berdasarkan peran"],
    "workflows": ["Barang masuk", "Barang keluar", "Lihat laporan"],
    "tech_needs": ["transactions"]
  },
  "blueprint": {
    "overview": {"goal": "Stok akurat dan tidak negatif", "users": ["Gudang", "Manajer"],
      "capabilities": ["Barang", "Transaksi stok", "Laporan"], "core_domain": "stok"},
    "platform": {"reason": "Web untuk gudang dan manajer", "devices": ["web", "backend"], "offline": "none", "realtime": "none"},
    "architecture": {"pattern": "modular", "reason": "beberapa domain", "tradeoff": "satu deploy", "evolve_when": [], "scale": ""},
    "modules": [
      {"id": "identity", "responsibility": "Auth, user, role, permission", "data": ["User"], "interface": ["POST /auth/login", "GET /me"], "depends_on": []},
      {"id": "katalog", "responsibility": "Barang dan kategori", "data": ["Barang", "Kategori"], "interface": ["POST /catalog/items", "GET /catalog/items"], "depends_on": ["identity"]},
      {"id": "stok", "responsibility": "Transaksi stok atomik", "data": ["Stock"], "interface": ["POST /stock/adjust"], "depends_on": ["katalog"]},
      {"id": "riwayat", "responsibility": "Riwayat transaksi dan filter", "data": ["StockTransaction"], "interface": ["GET /transactions"], "depends_on": ["stok"]},
      {"id": "laporan", "responsibility": "Laporan persediaan", "data": ["LaporanPersediaan"], "interface": ["GET /reports/inventory"], "depends_on": ["riwayat"]}
    ],
    "database": {
      "entities": [
        {"name": "User", "fields": ["id", "email", "role"], "module": "identity"},
        {"name": "Barang", "fields": ["id", "nama", "kategori_id", "stok_minimum"], "module": "katalog"},
        {"name": "Kategori", "fields": ["id", "nama"], "module": "katalog"},
        {"name": "Stock", "fields": ["barang_id", "jumlah"], "module": "stok"},
        {"name": "StockTransaction", "fields": ["id", "barang_id", "jenis", "jumlah", "waktu"], "module": "riwayat"},
        {"name": "LaporanPersediaan", "fields": ["id", "periode", "barang_id", "stok_awal", "stok_akhir"], "module": "laporan"}
      ],
      "transactions": "Satu transaksi stok = satu transaksi DB; gagal tanpa perubahan parsial",
      "indexing": ["index barang_id pada transaksi"]
    },
    "storage": {"strategy": "none"},
    "auth": {"method": "email + password"},
    "authz": {"model": "RBAC", "convention": "resource:action",
      "permissions": ["catalog:manage", "stock:adjust", "stock:view", "report:read"]}
  },
  "requirements": [
    {"id": "R1", "text": "Perubahan stok atomik: masuk dan keluar tidak boleh meninggalkan perubahan parsial", "priority": "MUST",
     "acceptance": [{"id": "R1-AC1", "kind": "negative", "text": "Jika salah satu langkah transaksi gagal, seluruh transaksi dibatalkan tanpa perubahan parsial", "evidence": "test transaksi stok: simulasi kegagalan di tengah, assert stok kembali ke nilai awal"}]},
    {"id": "R2", "text": "Stok tidak boleh negatif", "priority": "MUST",
     "acceptance": [{"id": "R2-AC1", "kind": "edge", "text": "Keluar melebihi stok tersedia ditolak", "evidence": "test: attempt keluar dengan jumlah > stok, assert ditolak dan stok tetap"}]},
    {"id": "R3", "text": "Akses dibatasi berdasarkan peran", "priority": "MUST",
     "acceptance": [{"id": "R3-AC1", "kind": "security", "text": "Gudang boleh menyesuaikan stok, Manajer hanya membaca laporan", "evidence": "test otorisasi: actor tanpa permission ditolak"}]},
    {"id": "R4", "text": "CRUD barang dan kategori", "priority": "MUST",
     "acceptance": [{"id": "R4-AC1", "kind": "behavior", "text": "Barang dan kategori dapat dibuat, dibaca, diubah, dihapus", "evidence": "test CRUD + screenshot halaman barang"}]},
    {"id": "R5", "text": "Riwayat transaksi dengan pencarian dan filter", "priority": "MUST",
     "acceptance": [{"id": "R5-AC1", "kind": "behavior", "text": "Transaksi dapat dicari dan difilter berdasarkan barang dan periode", "evidence": "test filter transaksi + screenshot"}]},
    {"id": "R6", "text": "Peringatan stok minimum", "priority": "MUST",
     "acceptance": [{"id": "R6-AC1", "kind": "behavior", "text": "Barang di bawah stok minimum ditandai", "evidence": "test: barang dengan jumlah < stok_minimum tampil sebagai peringatan"}]},
    {"id": "R7", "text": "Laporan persediaan berdasarkan periode", "priority": "MUST",
     "acceptance": [{"id": "R7-AC1", "kind": "behavior", "text": "Laporan menunjukkan stok awal, masuk, keluar, stok akhir per periode", "evidence": "test laporan periode + screenshot"}]}
  ],
  "features": [
    {"id": "F1", "name": "Barang dan Kategori", "description": "CRUD barang dan kategori", "requirement_ids": ["R4"], "platforms": ["web", "backend"],
     "sub_features": [{"name": "Kelola barang"}, {"name": "Kelola kategori"}]},
    {"id": "F2", "name": "Transaksi Stok", "description": "Barang masuk dan keluar secara atomik", "requirement_ids": ["R1", "R2", "R3"], "platforms": ["web", "backend"],
     "sub_features": [{"name": "Barang masuk"}, {"name": "Barang keluar"}]},
    {"id": "F3", "name": "Riwayat dan Filter", "description": "Riwayat transaksi dengan pencarian dan filter", "requirement_ids": ["R5"], "platforms": ["web", "backend"],
     "sub_features": [{"name": "Daftar riwayat"}, {"name": "Filter periode"}]},
    {"id": "F4", "name": "Peringatan Stok", "description": "Tandai barang di bawah stok minimum", "requirement_ids": ["R6"], "platforms": ["web", "backend"],
     "sub_features": [{"name": "Indikator stok minimum"}]},
    {"id": "F5", "name": "Laporan Persediaan", "description": "Laporan stok per periode", "requirement_ids": ["R7"], "platforms": ["web", "backend"],
     "sub_features": [{"name": "Laporan periode"}]}
  ],
  "flows": [
    {"id": "FL1", "name": "Barang masuk", "feature_id": "F2", "role": "Gudang", "goal": "Stok bertambah",
     "steps": ["Pilih barang", "Masukkan jumlah masuk", "Konfirmasi"],
     "preconditions": ["Login sebagai Gudang"], "postconditions": ["Stock bertambah"],
     "alternatives": [], "failures": ["Jumlah tidak valid"],
     "business_rules": ["Perubahan stok atomik"], "permissions": ["stock:adjust"],
     "data_effects": ["Stock: bertambah", "StockTransaction: dicatat"], "integrations": []},
    {"id": "FL2", "name": "Barang keluar", "feature_id": "F2", "role": "Gudang", "goal": "Stok berkurang",
     "steps": ["Pilih barang", "Masukkan jumlah keluar", "Konfirmasi"],
     "preconditions": ["Stok cukup"], "postconditions": ["Stock berkurang"],
     "alternatives": [], "failures": ["Stok tidak cukup"],
     "business_rules": ["Stok tidak boleh negatif"], "permissions": ["stock:adjust"],
     "data_effects": ["Stock: berkurang", "StockTransaction: dicatat"], "integrations": []},
    {"id": "FL3", "name": "Lihat laporan", "feature_id": "F5", "role": "Manajer", "goal": "Lihat laporan persediaan",
     "steps": ["Pilih periode", "Lihat laporan"],
     "preconditions": ["Login sebagai Manajer"], "postconditions": ["Laporan tampil"],
     "alternatives": [], "failures": ["Belum ada data"],
     "business_rules": ["Laporan per periode"], "permissions": ["report:read"],
     "data_effects": [], "integrations": []}
  ],
  "platform": {"targets": [{"kind": "web", "rationale": "UI gudang dan manajer"}, {"kind": "backend", "rationale": "API"}],
    "reason": "Gudang dan manajer memakai web, API untuk backend"},
  "definition_of_done": ["Semua acceptance criteria terbukti dengan test dan bukti"],
  "decisions": [],
  "skip": [{"area": "migration", "reason": "project baru"}]
}`

// The inventory scenario lands as a complete, traceable, verifiable
// blueprint: every MUST requirement has a checkable acceptance criterion
// with evidence, every requirement reaches a feature, and the cross-artifact
// validator reports no error.
func TestInventoryScenarioDiscovery(t *testing.T) {
	p := &core.Project{Story: "Sistem manajemen inventaris gudang"}
	if err := ApplyDiscovery(p, inventoryDiscovery); err != nil {
		t.Fatalf("ApplyDiscovery: %v", err)
	}
	if len(p.Requirements) != 7 {
		t.Fatalf("requirements = %d, want 7", len(p.Requirements))
	}
	if len(p.Features) != 5 || len(p.Flows) != 3 {
		t.Fatalf("features=%d flows=%d", len(p.Features), len(p.Flows))
	}

	// Verifiable acceptance criteria: every MUST requirement has criteria
	// with evidence, or an untestable reason.
	reqByText := map[string]core.Requirement{}
	for _, r := range p.Requirements {
		reqByText[r.Text] = r
		if r.Priority == "MUST" && len(r.Acceptance) == 0 && r.Untestable == "" {
			t.Fatalf("MUST requirement %s has no acceptance criteria", r.ID)
		}
		for _, c := range r.Acceptance {
			if strings.TrimSpace(c.Text) == "" || strings.TrimSpace(c.Evidence) == "" {
				t.Fatalf("requirement %s criterion %s not verifiable: %+v", r.ID, c.ID, c)
			}
		}
	}

	// The scenario's rules become data + verifiable criteria.
	atomic := findReq(reqByText, "atomik")
	if atomic == nil || !hasCriterionKind(atomic, "negative") {
		t.Fatalf("atomicity not a verifiable rule: %+v", atomic)
	}
	nonNeg := findReq(reqByText, "negatif")
	if nonNeg == nil || !hasCriterionKind(nonNeg, "edge") {
		t.Fatalf("non-negative stock not an edge criterion: %+v", nonNeg)
	}
	access := findReq(reqByText, "peran")
	if access == nil || !hasCriterionKind(access, "security") {
		t.Fatalf("role-based access not a security criterion: %+v", access)
	}

	// Traceability: every requirement reaches a feature; flows reference a
	// feature and name the permission that gates the action.
	covered := map[string]bool{}
	for _, f := range p.Features {
		for _, r := range f.RequirementIDs {
			covered[r] = true
		}
	}
	for _, r := range p.Requirements {
		if !covered[r.ID] {
			t.Fatalf("requirement %s not covered by a feature", r.ID)
		}
	}
	for _, fl := range p.Flows {
		if fl.FeatureID == "" || len(fl.Permissions) == 0 {
			t.Fatalf("flow %s lacks feature or permission: %+v", fl.ID, fl)
		}
	}

	// Cross-artifact consistency: no error, and the fixture is clean enough
	// that the deterministic validator raises nothing at all.
	if issues := core.ConsistencyIssues(p); len(issues) != 0 {
		t.Fatalf("consistency issues: %+v", issues)
	}
}

// The inventory blueprint yields a fresh workflow prompt that carries the
// command rules for invariants and access, and a non-empty artifact set.
func TestInventoryScenarioWorkflowPrompt(t *testing.T) {
	p := &core.Project{Story: "Sistem manajemen inventaris gudang"}
	if err := ApplyDiscovery(p, inventoryDiscovery); err != nil {
		t.Fatalf("ApplyDiscovery: %v", err)
	}
	docs := spec.Artifacts(p)
	if len(docs) == 0 {
		t.Fatal("no blueprint artifacts rendered")
	}
	got := workflowFreshPrompt(p, docs)
	for _, want := range []string{
		"a transaction must fail without partial change",
		"names the permission(s) from authz that gate the action",
		"REQ-001",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("inventory workflow prompt missing %q", want)
		}
	}
}

func findReq(m map[string]core.Requirement, sub string) *core.Requirement {
	for text, r := range m {
		if strings.Contains(strings.ToLower(text), sub) {
			rc := r
			return &rc
		}
	}
	return nil
}

func hasCriterionKind(r *core.Requirement, kind string) bool {
	for _, c := range r.Acceptance {
		if c.Kind == kind {
			return true
		}
	}
	return false
}
