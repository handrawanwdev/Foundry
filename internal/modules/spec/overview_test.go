package spec

import (
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
)

func clinicOverview() *core.Project {
	return &core.Project{
		Name:  "Klinik",
		Story: "Klinik ingin janji temu tanpa antrian kertas.",
		Discovery: &core.Discovery{
			Summary:  "Aplikasi janji temu untuk klinik pratama.",
			Business: "Pasien mengantri di kertas dan sering bentrok jadwal.",
			Users:    []string{"Pasien", "Petugas"},
			Roles: []core.Role{
				{Name: "Pasien", Description: "Membuat dan melihat janji temu.", Platform: "web"},
				{Name: "Petugas", Description: "Mengatur jadwal dokter.", Platform: "web"},
			},
			Integrations: []string{"WhatsApp untuk pengingat"},
			Workflows:    []string{"Buat janji"},
		},
		Blueprint: &core.Blueprint{
			Overview: core.Overview{
				Goal:         "Janji temu tanpa antrian kertas",
				Users:        []string{"Pasien", "Petugas"},
				Capabilities: []string{"Buat janji", "Lihat antrean"},
				CoreDomain:   "janji temu",
			},
			Modules: []core.Module{
				{ID: "janji", Responsibility: "Mencatat dan mengatur janji temu.", DependsOn: []string{"identitas"}},
				{ID: "identitas", Responsibility: "Mengenali pasien dan petugas."},
			},
			Events:  core.EventArch{Events: []core.EventSpec{{Type: "janji.dibuat", Producer: "janji", Consumers: []string{"pengingat"}}}},
			Workers: []core.Worker{{Name: "pengingat", Trigger: "event", Job: "kirim pengingat janji"}},
			Authz:   core.AuthzArch{Model: "RBAC"},
		},
		Platform: &core.PlatformPlan{Targets: []core.PlatformTarget{{Kind: core.PlatformWeb}}, Reason: "Dipakai petugas di meja dan pasien dari rumah."},
		Features: []core.Feature{{
			ID: "FEAT-001", Name: "Janji temu", Description: "Pasien memilih jadwal dan petugas mengonfirmasi.",
			SubFeatures: []core.SubFeature{{ID: "FEAT-001.1", Name: "Pilih slot"}, {ID: "FEAT-001.2", Name: "Konfirmasi petugas"}},
		}},
		Flows: []core.Flow{{
			ID: "FLOW-001", Name: "Buat janji", FeatureID: "FEAT-001", Role: "Pasien",
			Goal: "Dapat jadwal yang dikonfirmasi", Pre: []string{"Sudah masuk akun"},
			Steps: []string{"Pilih dokter", "Pilih waktu", "Kirim permintaan"},
			Post:  []string{"Janji tercatat sebagai menunggu"}, Failures: []string{"Jadwal penuh"},
			Rules: []string{"Satu pasien satu janji aktif per dokter"}, Permissions: []string{"janji:buat"},
		}},
		Decisions: []core.Decision{
			{ID: "DEC-001", Kind: core.DecisionQuestion, Text: "Pengingat lewat WhatsApp atau SMS?", Critical: true},
			{ID: "DEC-002", Kind: core.DecisionConflict, Text: "Jadwal dokter diisi petugas atau dokter sendiri?"},
			{ID: "DEC-003", Kind: core.DecisionQuestion, Text: "Pembayaran di klinik atau daring?", Resolution: "Pembayaran di klinik"},
		},
		Skips: []core.AreaSkip{{Area: core.AreaOperations, Reason: "uji coba di satu klinik"}},
	}
}

func overviewOf(t *testing.T, p *core.Project) core.Artifact {
	t.Helper()
	for _, a := range Artifacts(p) {
		if a.ID == core.ArtifactSystemOverview {
			if !a.Builtin {
				t.Fatal("system-overview must be builtin")
			}
			return a
		}
	}
	t.Fatal("system-overview missing")
	return core.Artifact{}
}

func TestSystemOverviewExplainsProduct(t *testing.T) {
	p := clinicOverview()
	a := overviewOf(t, p)
	if a.Title != "Penjelasan Sistem" || a.Category != core.ArtifactRequirements {
		t.Fatalf("meta = %+v", a)
	}
	body := a.Body
	for _, want := range []string{
		"## Tujuan", "## Fitur utama", "## Pengguna dan wewenang", "## Alur kerja",
		"## Aturan yang berlaku", "## Hubungan antarbagian", "## Contoh penggunaan",
		"## Manfaat", "## Yang belum ditentukan",
		"Aplikasi janji temu untuk klinik pratama.",
		"Pasien mengantri di kertas",
		"Janji temu tanpa antrian kertas",
		"### Janji temu", "Pilih slot", "Konfirmasi petugas",
		"### Pasien", "Membuat dan melihat janji temu", "situs web",
		"wewenang mengikuti peran", "Pasien: janji:buat",
		"### Buat janji", "Pilih dokter", "Jadwal penuh",
		"Satu pasien satu janji aktif per dokter", "Pembayaran di klinik",
		"### janji", "Bekerja bersama: identitas",
		"Ketika janji.dibuat terjadi", "kirim pengingat janji",
		"WhatsApp untuk pengingat",
		"Pasien ingin Dapat jadwal yang dikonfirmasi",
		"Pilih dokter, lalu Pilih waktu, lalu Kirim permintaan",
		"Dengan sistem ini, pengguna dapat", "Lihat antrean",
		"Pengingat lewat WhatsApp atau SMS? (perlu diputuskan sebelum rancangan dianggap lengkap)",
		"Jadwal dokter diisi petugas atau dokter sendiri?",
		"uji coba di satu klinik",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q\n%s", want, body)
		}
	}
	for _, no := range []string{
		"Foundry", "blueprint", "coding agent", "Argon2id", "REQ-001", "Project Overview",
		"JWT", "Modular Monolith",
	} {
		if strings.Contains(body, no) {
			t.Errorf("layperson text contains %q", no)
		}
	}
}

func TestSystemOverviewDoesNotInvent(t *testing.T) {
	p := &core.Project{Name: "Klinik", Story: "Klinik ingin janji temu tanpa kertas."}
	body := overviewOf(t, p).Body
	if !strings.Contains(body, undetermined) {
		t.Fatalf("empty fields not marked:\n%s", body)
	}
	for _, invented := range []string{"Pasien", "WhatsApp", "admin", "lebih cepat", "lebih mudah", "Petugas"} {
		if strings.Contains(body, invented) {
			t.Errorf("invented %q\n%s", invented, body)
		}
	}
	if strings.Contains(body, "Tidak ada informasi terbuka") {
		t.Error("claimed nothing is open while most sections are empty")
	}
}

func TestSystemOverviewTracksBlueprint(t *testing.T) {
	p := clinicOverview()
	p.UI = sampleUI()
	before := overviewOf(t, p)
	p.Features = append(p.Features, core.Feature{ID: "FEAT-002", Name: "Rekam medis", Description: "Catat hasil periksa."})
	after := overviewOf(t, p)
	if after.Hash() == before.Hash() || !strings.Contains(after.Body, "Rekam medis") {
		t.Fatalf("feature change did not update overview: %s", after.Body)
	}
	p.UI.Layout.Pages[0].Sections[0].Spacing = "space.lg"
	if overviewOf(t, p).Hash() != after.Hash() {
		t.Fatal("layout token change must not rewrite the overview")
	}
}

func TestSystemOverviewDualSourcesStayOpen(t *testing.T) {
	p := clinicOverview()
	p.Blueprint.Overview.Users = []string{"Dokter"}
	body := overviewOf(t, p).Body
	if !strings.Contains(body, "Belum ada keputusan mana yang diikuti") {
		t.Fatalf("conflicting users not flagged:\n%s", body)
	}
	if !strings.Contains(body, "Dokter") || !strings.Contains(body, "Pasien") {
		t.Fatalf("dropped a source:\n%s", body)
	}
}

func TestSystemOverviewFirstArtifact(t *testing.T) {
	docs := Artifacts(clinicOverview())
	if len(docs) == 0 || docs[0].ID != core.ArtifactSystemOverview {
		t.Fatalf("first = %+v", docs)
	}
}

func TestSystemOverviewAbsentWithoutSource(t *testing.T) {
	for _, a := range Artifacts(&core.Project{Name: "X"}) {
		if a.ID == core.ArtifactSystemOverview {
			t.Fatalf("unexpected overview: %s", a.Body)
		}
	}
}
