package core

import "testing"

func entryLayout() *LayoutSpec {
	return &LayoutSpec{
		Navigation: []NavGroup{{Name: "Gudang", Items: []NavItem{{Label: "Barang", Page: "barang"}, {Label: "Mutasi", Page: "mutasi"}, {Label: "Pengguna", Page: "pengguna"}}}},
		Pages: []LayoutPage{
			{ID: "barang", Name: "Daftar Barang", Pattern: "table", FeatureID: "F3"},
			{ID: "barang-detail", Name: "Detail Barang", Pattern: "detail", FeatureID: "F3"},
			{ID: "barang-form", Name: "Tambah atau Ubah Barang", Pattern: "form", FeatureID: "F3"},
			{ID: "mutasi", Name: "Riwayat Mutasi", Pattern: "table", FeatureID: "F4"},
			{ID: "mutasi-form", Name: "Catat Mutasi", Pattern: "form", FeatureID: "F4"},
			{ID: "pengguna", Name: "Pengguna", Pattern: "table", FeatureID: "F1"},
			{ID: "lepas", Name: "Ubah Profil", Pattern: "form", FeatureID: "F9"},
		},
	}
}

// A form or detail page outside the menu is entered from the list page of
// its feature: its create action, its rows.
func TestPageEntries(t *testing.T) {
	l := entryLayout()
	if id, lb := CreateForm(l, "barang"); id != "barang-form" || lb != "Tambah barang" {
		t.Errorf("form tambah barang: %q %q", id, lb)
	}
	if id, lb := CreateForm(l, "mutasi"); id != "mutasi-form" || lb != "Catat mutasi" {
		t.Errorf("form catat mutasi: %q %q", id, lb)
	}
	if id, _ := CreateForm(l, "pengguna"); id != "" {
		t.Errorf("pengguna tanpa form: %q", id)
	}
	if got := DetailPage(l, "barang"); got != "barang-detail" {
		t.Errorf("detail barang: %q", got)
	}
	for id, want := range map[string]string{"barang-form": "barang", "barang-detail": "barang", "barang": "", "lepas": ""} {
		if got := ParentPage(l, id); got != want {
			t.Errorf("induk %s: %q, mau %q", id, got, want)
		}
	}
	if got := UnreachablePages(l); len(got) != 1 || got[0] != "lepas" {
		t.Errorf("tidak terjangkau: %v", got)
	}
	if got := LinkTarget(l, "ke riwayat mutasi", "dashboard"); got != "mutasi" {
		t.Errorf("tautan riwayat mutasi: %q", got)
	}
	if got := LinkTarget(l, "lihat semua barang menipis", "dashboard"); got != "barang" {
		t.Errorf("tautan barang menipis: %q", got)
	}
}

// A use case of the feature that makes what the page lists asks for its
// create action.
func TestCreateIntent(t *testing.T) {
	l := entryLayout()
	pg := &l.Pages[5]
	p := &Project{Flows: []Flow{{ID: "FLOW-1", FeatureID: "F1", Name: "Login", Steps: []string{"Isi email"}}}}
	if p.CreateIntent(pg) {
		t.Error("login dianggap membuat pengguna")
	}
	p.Flows = append(p.Flows, Flow{ID: "FLOW-2", FeatureID: "F1", Name: "Admin membuat akun pengguna", Steps: []string{"Form user"}})
	if !p.CreateIntent(pg) {
		t.Error("membuat akun pengguna tidak terbaca")
	}
	if CreateLabel("Ubah Profil") != "" || CreateLabel("Tambah atau Ubah Barang") != "Tambah barang" {
		t.Error("label create salah")
	}
}
