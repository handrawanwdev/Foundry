package core

import (
	"strconv"
	"testing"
)

func TestFeatureMapNormalizeAndFind(t *testing.T) {
	m := &FeatureMap{
		Summary: "  Exchange  ",
		Modules: []FeatureModule{
			{Name: " Trading ", Features: []FeatureNode{
				{Name: "Spot Trading", Description: " beli/jual ", Purpose: "eksekusi", Benefit: "likuid", Outcome: "order terisi",
					Children: []FeatureNode{{Name: "Market Order"}, {Name: ""}, {Name: "Limit Order"}}},
				{Name: ""},
			}},
			{Name: "  "},
		},
	}
	m.Normalize()
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	if m.Summary != "Exchange" || len(m.Modules) != 1 || m.Modules[0].ID != "trading" {
		t.Fatalf("modules = %+v", m.Modules)
	}
	spot := m.Find("spot-trading")
	if spot == nil || len(spot.Children) != 2 || spot.Children[0].ID != "market-order" || m.Count() != 3 {
		t.Fatalf("spot = %+v count=%d", spot, m.Count())
	}
	mod, node := m.Locate("trading")
	if mod == nil || node != nil || mod.Name != "Trading" {
		t.Fatalf("locate module = %+v %+v", mod, node)
	}
	mod, node = m.Locate("limit-order")
	if mod == nil || node == nil || node.Name != "Limit Order" {
		t.Fatalf("locate node = %+v %+v", mod, node)
	}
}

func TestFeatureMapCapsDepthAndCount(t *testing.T) {
	var nest FeatureNode
	cur := &nest
	cur.Name = "n1"
	for i := 2; i <= 6; i++ {
		cur.Children = []FeatureNode{{Name: "n" + strconv.Itoa(i)}}
		cur = &cur.Children[0]
	}
	m := &FeatureMap{Modules: []FeatureModule{{Name: "M", Features: []FeatureNode{nest}}}}
	m.Normalize()
	n := m.Find("n1")
	for d := 1; d < MaxFeatureDepth; d++ {
		if n == nil || len(n.Children) != 1 {
			t.Fatalf("depth %d: %+v", d, n)
		}
		n = &n.Children[0]
	}
	if n == nil || len(n.Children) != 0 {
		t.Fatalf("depth cap: %+v", n)
	}

	var feats []FeatureNode
	for i := 0; i < MaxFeatureNodes+10; i++ {
		feats = append(feats, FeatureNode{Name: "F" + strconv.Itoa(i)})
	}
	big := &FeatureMap{Modules: []FeatureModule{{Name: "M", Features: feats}}}
	big.Normalize()
	if big.Count() != MaxFeatureNodes {
		t.Fatalf("count = %d", big.Count())
	}
}

func TestFeatureMapFromFeatures(t *testing.T) {
	if FeatureMapFromFeatures(nil) != nil {
		t.Fatal("empty")
	}
	m := FeatureMapFromFeatures([]Feature{
		{ID: "FEAT-001", Name: "Janji", Description: "buat janji", SubFeatures: []SubFeature{{ID: "FEAT-001.1", Name: "Cari dokter"}}},
	})
	if m.Empty() || m.Modules[0].Name != "Fitur" || m.Find("feat-001") == nil || m.Find("feat-001-1") == nil || m.Count() != 2 {
		t.Fatalf("map = %+v", m)
	}
}

func TestFeatureMapDuplicateIDs(t *testing.T) {
	m := &FeatureMap{Modules: []FeatureModule{
		{ID: "same", Name: "A", Features: []FeatureNode{{ID: "same", Name: "B"}}},
	}}
	m.Normalize()
	if m.Modules[0].ID == m.Modules[0].Features[0].ID {
		t.Fatalf("ids not unique: %+v", m)
	}
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestLinkFeatureMapSetsSourceAndParent(t *testing.T) {
	m := &FeatureMap{Modules: []FeatureModule{{Name: "Antrean", Features: []FeatureNode{
		{Name: "Janji", Children: []FeatureNode{{Name: "Form janji"}}},
	}}}}
	m.Normalize()
	feats := []Feature{{ID: "FEAT-001", Name: "Janji", SubFeatures: []SubFeature{{ID: "FEAT-001.1", Name: "Form janji"}}}}
	LinkFeatureMap(m, feats)
	if feats[0].SourceID != m.Modules[0].Features[0].ID {
		t.Fatalf("feature source = %q", feats[0].SourceID)
	}
	if feats[0].SubFeatures[0].SourceID != m.Modules[0].Features[0].Children[0].ID {
		t.Fatalf("subfeature source = %q", feats[0].SubFeatures[0].SourceID)
	}
	if feats[0].SubFeatures[0].FeatureID != "FEAT-001" {
		t.Fatalf("subfeature parent = %q", feats[0].SubFeatures[0].FeatureID)
	}
	if parent := m.Modules[0].Features[0].Children[0].ParentID; parent != m.Modules[0].Features[0].ID {
		t.Fatalf("node parent = %q", parent)
	}
}

func TestMapFeatureMapLegacyNames(t *testing.T) {
	feats := []Feature{
		{ID: "FEAT-002", Name: "Ringkasan tugas", Description: "Dasbor menampilkan jumlah tugas total dan per status milik pengguna."},
		{ID: "FEAT-003", Name: "Kelola tugas", Description: "Buat, baca, ubah, hapus tugas milik sendiri; data tersimpan di server."},
		{ID: "FEAT-004", Name: "Status dan prioritas", Description: "Tugas memakai status dan prioritas terbatas; prioritas tampak visual di daftar."},
		{ID: "FEAT-005", Name: "Deadline", Description: "Deadline tanggal opsional pada tugas, dapat diisi, diubah, atau dikosongkan."},
		{ID: "FEAT-006", Name: "Cari dan filter", Description: "Cari judul dan filter status/prioritas pada tugas milik sendiri."},
		{ID: "FEAT-007", Name: "Pengalaman web responsif", Description: "Dasbor modern terinspirasi Todoist dengan identitas sendiri, mudah dipakai di desktop dan seluler."},
	}
	m := &FeatureMap{Modules: []FeatureModule{
		{Name: "Ringkasan Tugas", Features: []FeatureNode{{Name: "Ringkasan Jumlah Tugas", Description: "Halaman awal menampilkan berapa banyak tugas yang ada."}}},
		{Name: "Kelola Tugas", Features: []FeatureNode{
			{Name: "Daftar Tugas", Description: "Semua tugas muncul di satu halaman."},
			{Name: "Tambah Tugas", Description: "Isi judul tugas baru lalu simpan."},
			{Name: "Ubah Tugas", Description: "Ganti judul atau isi tugas yang sudah ada."},
			{Name: "Hapus Tugas", Description: "Buang tugas yang sudah tidak perlu."},
		}},
		{Name: "Status Tugas", Features: []FeatureNode{{Name: "Ubah Status Tugas", Description: "Ganti status tugas sesuai posisi kerjaannya."}}},
		{Name: "Prioritas", Features: []FeatureNode{{Name: "Atur Prioritas", Description: "Pilih tingkat penting tiap tugas."}}},
		{Name: "Deadline", Features: []FeatureNode{{Name: "Tentukan Deadline", Description: "Isi tanggal batas waktu tiap tugas."}}},
		{Name: "Cari dan Filter", Features: []FeatureNode{
			{Name: "Cari Tugas", Description: "Ketik kata kunci buat menemukan tugas."},
			{Name: "Filter Tugas", Description: "Saring daftar tugas berdasar kriteria tertentu."},
		}},
		{Name: "Tampilan", Features: []FeatureNode{
			{Name: "Tampilan Modern", Description: "Halaman dibuat bersih dan enak dilihat."},
			{Name: "Tampilan Responsif", Description: "Layar menyesuaikan ukuran HP, tablet, dan laptop."},
		}},
		{Name: "Simpan Data", Features: []FeatureNode{
			{Name: "Simpan Perubahan", Description: "Setiap tugas yang dibuat atau diubah langsung tersimpan."},
			{Name: "Data Tetap Ada", Description: "Tugas yang sudah dicatat masih ada saat aplikasi dibuka lagi."},
		}},
	}}
	m.Normalize()
	MapFeatureMap(m, feats)
	want := map[string]string{
		"ringkasan-jumlah-tugas": "FEAT-002",
		"daftar-tugas":           "FEAT-003",
		"tambah-tugas":           "FEAT-003",
		"ubah-status-tugas":      "FEAT-004",
		"atur-prioritas":         "FEAT-004",
		"tentukan-deadline":      "FEAT-005",
		"cari-tugas":             "FEAT-006",
		"filter-tugas":           "FEAT-006",
		"tampilan-modern":        "FEAT-007",
		"tampilan-responsif":     "FEAT-007",
		"simpan-perubahan":       "FEAT-003",
		"data-tetap-ada":         "FEAT-003",
	}
	for id, wantID := range want {
		n := m.Find(id)
		if n == nil || n.FeatureID != wantID {
			t.Errorf("%s feature = %+v, want %s", id, n, wantID)
		}
	}
}

func TestBackfillCommandFeaturesFromCovers(t *testing.T) {
	p := &Project{
		Features: []Feature{{ID: "FEAT-001", Name: "Janji", RequirementIDs: []string{"REQ-001"}}},
		Workflow: &Workflow{ID: "WF-001", Commands: []Command{{ID: "CMD-001", Covers: []string{"REQ-001"}}}},
	}
	p.NormalizeLinks()
	if got := p.Workflow.Commands[0].FeatureIDs; len(got) != 1 || got[0] != "FEAT-001" {
		t.Fatalf("backfilled features = %v", got)
	}
}
