package core

import (
	"strings"
	"testing"
)

// Every table or list must be searchable/filterable and paged: the page's
// own toolbar and pagination sections serve its first collection, any
// other collection names its controls in its content.
func TestCollectionsNeedSearchAndPagination(t *testing.T) {
	pg := &LayoutPage{ID: "antrean", Pattern: "table", Sections: []Section{
		{ID: "saring", Name: "Filter", Content: []string{"Cari pasien"}},
		{ID: "tabel", Name: "Tabel antrean", Content: []string{"Tabel: No, Pasien, Status"}},
		{ID: "riwayat", Name: "Riwayat", Pattern: "list", Content: []string{"Panggilan terakhir"}},
		{ID: "tab", Name: "Tab", Content: []string{"Tab Hari ini, Besok"}},
	}}
	issues := collectionIssues("antrean", pg, UIDesignFormat)
	got := issueMessages(issues)
	if len(issues) != 2 || Errors(issues) == nil ||
		!strings.Contains(got, "antrean/tabel") || strings.Contains(issueText(issues, "antrean/tabel"), "filter") ||
		!strings.Contains(issueText(issues, "antrean/riwayat"), "pencarian/filter dan paginasi") {
		t.Fatalf("issues = %s", got)
	}
	pg.Sections[1].Content = append(pg.Sections[1].Content, "Paginasi: 20 per halaman")
	pg.Sections[2].Content = append(pg.Sections[2].Content, "Filter: cari, status", "Muat lebih banyak saat digulir")
	if issues := collectionIssues("antrean", pg, UIDesignFormat); len(issues) != 0 {
		t.Fatalf("controls not recognized: %s", issueMessages(issues))
	}
	pg.Sections[2].Content = pg.Sections[2].Content[:1]
	if issues := collectionIssues("antrean", pg, UIUXFormat); len(issues) != 1 || Errors(issues) != nil {
		t.Fatalf("older design must only warn: %s", issueMessages(issues))
	}
}

func issueText(issues []Issue, where string) string {
	for _, i := range issues {
		if i.Where == where {
			return i.Message
		}
	}
	return ""
}

func TestDialogSection(t *testing.T) {
	dlg := Section{ID: "dialog-kategori", Name: "Dialog kategori", Content: []string{"Tabel: Nama, Deskripsi"}}
	if !DialogSection("table", dlg) || IsCollection("table", dlg) || ControlSection("table", dlg) != "" {
		t.Error("dialog dianggap bagian tata letak halaman")
	}
	if DialogSection("modal", Section{ID: "aksi", Name: "Aksi dialog"}) {
		t.Error("isi halaman modal dianggap dialog terpisah")
	}
	if DialogSection("table", Section{ID: "dialogue-log", Name: "Riwayat"}) {
		t.Error("kata yang hanya memuat dialog dianggap dialog")
	}
	if !DialogSection("detail", Section{ID: "x", Name: "Hapus", Pattern: "modal"}) {
		t.Error("section pattern modal bukan dialog")
	}
}
