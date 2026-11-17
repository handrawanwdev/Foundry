package uikit

import (
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
)

// A detail page names its record by the page's fields: its name as title,
// its status as a badge, its key figure large, its actions beside the
// title, and a history as records with their kind in the design's words.
func TestRecordPage(t *testing.T) {
	k := kit(t)
	p := formProject()
	p.Blueprint.Database.Entities[0].Fields = append(p.Blueprint.Database.Entities[0].Fields, "is_active", "current_stock >= 0")
	l := p.UI.Layout
	l.Pages = append(l.Pages, core.LayoutPage{ID: "barang-detail", Name: "Detail Barang", Pattern: "detail", Shell: "app", FeatureID: "F1",
		Fields: []core.FieldRule{{Field: "Item.name", Label: "Nama barang", Presentation: "detail"}, {Field: "Item.sku", Label: "SKU", Presentation: "detail"},
			{Field: "Item.current_stock", Label: "Stok saat ini", Presentation: "card-value"}, {Field: "Item.is_active", Label: "Status", Presentation: "badge"},
			{Field: "StockMovement.type", Label: "Tipe", Presentation: "badge"}, {Field: "StockMovement.qty", Label: "Qty", Presentation: "table-column"}},
		Sections: []core.Section{
			{ID: "identitas", Name: "Identitas dan stok", Content: []string{"PageHeader dengan breadcrumb dan badge status", "Stok saat ini sebagai angka dominan", "Metadata SKU"}, Components: []string{"PageHeader", "StatCard"}},
			{ID: "riwayat", Name: "Riwayat mutasi", Content: []string{"Sepuluh mutasi terakhir: Waktu, Tipe, Qty, Oleh"}, Components: []string{"DataTable"}},
			{ID: "aksi", Name: "Aksi barang", Content: []string{"Tombol Ubah data", "Tombol Nonaktifkan barang untuk Admin dengan konfirmasi"}, Components: []string{"Button"}},
		}})
	d := DocOf(p)
	d.Link = func(id string) string { return "/m/" + id }
	out, _ := k.RenderPage(d, "barang-detail", "")
	for _, want := range []string{`<h3>Barang Alfa</h3>`, `fk-badge fk-badge-success" title="Status">Aktif`, `<p class="fk-meta">SKU #`,
		`<div class="fk-figure"><span>Stok saat ini</span><strong>`, `<div class="fk-actions fk-hero-acts">`, `href="/m/barang-form?fill=1"`,
		`fk-btn fk-btn-danger`, `<ul class="fk-feed">`, `>Masuk</span>`, `oleh `} {
		if !strings.Contains(out, want) {
			t.Errorf("detail tanpa %s", want)
		}
	}
	for _, bad := range []string{"PageHeader", "breadcrumb", "Metadata", "<dt>Waktu", `data-foundry-section="aksi"`} {
		if strings.Contains(out, bad) {
			t.Errorf("detail memuat %s", bad)
		}
	}
}
