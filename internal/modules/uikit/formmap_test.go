package uikit

import (
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
)

// formProject is an inventory with a shared create/edit form, a stock
// movement form and a wizard whose inputs are only page fields.
func formProject() *core.Project {
	return &core.Project{Name: "Inv",
		Blueprint: &core.Blueprint{Database: core.DatabaseArch{Entities: []core.Entity{
			{Name: "Item", Fields: []string{"id", "sku unique", "name", "category_id FK", "min_stock >= 0"}},
			{Name: "Category", Fields: []string{"id", "name unique"}},
			{Name: "StockMovement", Fields: []string{"id", "item_id FK", "type (in|out)", "qty > 0", "note"}},
			{Name: "Student", Fields: []string{"full_name", "nisn unique"}},
			{Name: "Guardian", Fields: []string{"phone"}},
		}}},
		UI: &core.UIDesign{Layout: &core.LayoutSpec{
			Shells: []core.LayoutShell{{ID: "app", Name: "App", Pattern: "sidebar"}},
			Navigation: []core.NavGroup{{Name: "Gudang", Items: []core.NavItem{
				{Label: "Barang", Page: "barang"}, {Label: "Mutasi", Page: "mutasi"}}}},
			Pages: []core.LayoutPage{
				{ID: "barang", Name: "Barang", Pattern: "table", Shell: "app", FeatureID: "F1",
					Sections: []core.Section{{ID: "tabel", Name: "Tabel barang", Content: []string{"Tabel: SKU, Nama, Stok", "Aksi baris: Ubah"}}}},
				{ID: "barang-form", Name: "Tambah atau Ubah Barang", Pattern: "form", Shell: "app", FeatureID: "F1",
					Fields: []core.FieldRule{{Field: "Item.sku", Label: "SKU", Presentation: "form-input"}, {Field: "Item.name", Label: "Nama barang", Presentation: "form-input"},
						{Field: "Category.name", Label: "Kategori", Presentation: "form-input"}, {Field: "Item.min_stock", Label: "Stok minimum", Presentation: "form-input"}},
					Sections: []core.Section{
						{ID: "data", Name: "Data barang", Content: []string{"SKU dan nama", "Kategori wajib dipilih", "Stok minimum"}, Components: []string{"Field"}},
						{ID: "aksi", Name: "Aksi form", Content: []string{"Tombol Simpan barang", "Tombol Batal kembali ke daftar"}, Components: []string{"Button"}},
					}},
				{ID: "mutasi", Name: "Mutasi", Pattern: "table", Shell: "app", FeatureID: "F2",
					Sections: []core.Section{{ID: "tabel", Name: "Tabel mutasi", Content: []string{"Tabel: Barang, Jumlah"}}}},
				{ID: "mutasi-form", Name: "Catat Mutasi", Pattern: "form", Shell: "app", FeatureID: "F2",
					Fields: []core.FieldRule{{Field: "StockMovement.type", Label: "Jenis mutasi", Presentation: "form-input"}, {Field: "Item.name", Label: "Barang", Presentation: "form-input"},
						{Field: "StockMovement.qty", Label: "Jumlah", Presentation: "form-input"}, {Field: "StockMovement.note", Label: "Catatan", Presentation: "form-input"},
						{Field: "Item.min_stock", Label: "Stok minimum", Presentation: "detail"}},
					Sections: []core.Section{
						{ID: "jenis", Name: "Jenis dan barang", Content: []string{"Pilihan jenis masuk/keluar sebagai segmented control berlabel", "SearchSelect barang dengan pencarian SKU atau nama"}, Components: []string{"Button", "SearchSelect"}},
						{ID: "konteks", Name: "Konteks stok", Content: []string{"Stok saat ini, dan stok minimum barang terpilih", "Peringatan 'melebihi stok tersedia' bila qty keluar lebih besar"}, Components: []string{"StatCard"}},
						{ID: "isi", Name: "Jumlah dan catatan", Content: []string{"Input jumlah dengan helper 'bilangan bulat lebih dari 0'", "Textarea catatan", "Tombol Simpan mutasi dan Batal"}, Components: []string{"Field", "Button"}},
					}},
				{ID: "registrasi", Name: "Registrasi Siswa", Pattern: "wizard", Shell: "app",
					Fields: []core.FieldRule{{Field: "Student.full_name", Label: "Nama lengkap", Presentation: "form-input"}, {Field: "Student.nisn", Label: "NISN", Presentation: "form-input"},
						{Field: "Guardian.phone", Label: "Telepon wali", Presentation: "form-input"}},
					Sections: []core.Section{{ID: "langkah", Name: "Langkah", Content: []string{"Biodata, Wali & relasi, Konfirmasi"}, Components: []string{"Wizard", "FormField"}}}},
			},
		}},
	}
}

// A form's inputs follow the page fields its lines name and the data
// behind them: two fields from one line, a search over another entity's
// records, bounds, choices, and a note left optional.
func TestFormMapsFields(t *testing.T) {
	k := kit(t)
	d := DocOf(formProject())
	d.Link, d.Self = func(id string) string { return "/m/" + id }, "/m/barang-form"
	out, _ := k.RenderPage(d, "barang-form", "")
	for _, want := range []string{`>SKU<span class="fk-req"`, `>Nama barang<span class="fk-req"`, `Harus unik, tidak boleh sama dengan data lain.`,
		`<div class="fk-combo">`, `list="f-data-2-list"`, `type="number" inputmode="numeric" class="fk-input" placeholder="0" min="0" step="1"`,
		`<h1>Tambah barang</h1>`, `<div class="fk-page fk-form-page"`,
		`<div class="fk-actions fk-form-bar"><span class="fk-form-note">`, `title="Batal kembali ke daftar">Batal</a><button class="fk-btn fk-btn-primary"`} {
		if !strings.Contains(out, want) {
			t.Errorf("form barang tanpa %s", want)
		}
	}
	if strings.Contains(out, "SKU dan nama") {
		t.Error("satu baris dua field tergambar sebagai satu field")
	}

	d.Self = "/m/mutasi-form"
	out, _ = k.RenderPage(d, "mutasi-form", "")
	for _, want := range []string{`<fieldset class="fk-field fk-group"`, `value="Masuk" required`, `value="Keluar"`,
		`min="1" step="1"`, `>Catatan <span class="fk-opt">(opsional)</span>`, `<textarea`, `<dt>Stok minimum</dt>`,
		`<div class="fk-callout" role="note">`, `<b>Melebihi stok tersedia</b>`} {
		if !strings.Contains(out, want) {
			t.Errorf("form mutasi tanpa %s", want)
		}
	}
	if strings.Contains(out, "Dan stok minimum") || strings.Contains(out, "<dt>Peringatan") {
		t.Error("konteks stok memakai kata desain mentah")
	}
}

// An edit action opens the shared form holding the record, titled by the
// edit.
func TestFormEditMode(t *testing.T) {
	k := kit(t)
	d := DocOf(formProject())
	d.Link = func(id string) string { return "/m/" + id }
	list, _ := k.RenderPage(d, "barang", "")
	if !strings.Contains(list, `href="/m/barang-form?fill=1"`) {
		t.Fatal("aksi Ubah tidak membuka form berisi record")
	}
	d.Sim = Sim{Fill: true}
	out, _ := k.RenderPage(d, "barang-form", "")
	for _, want := range []string{`<h1>Ubah barang</h1>`, `<title>Ubah barang`, `<span>Ubah barang</span>`, `value="Barang Alfa"`} {
		if !strings.Contains(out, want) {
			t.Errorf("mode ubah tanpa %s", want)
		}
	}
}

// A wizard spreads the page's inputs over its steps; the URL opens a step,
// and the last one without inputs reviews them.
func TestWizardSteps(t *testing.T) {
	k := kit(t)
	d := DocOf(formProject())
	d.Link, d.Self = func(id string) string { return "/m/" + id }, "/m/registrasi"
	out, _ := k.RenderPage(d, "registrasi", "")
	if !strings.Contains(out, `>Nama lengkap<span`) || !strings.Contains(out, `>NISN<span`) || strings.Contains(out, `>Telepon wali<span`) ||
		!strings.Contains(out, `name="view" value="step-2"`) || !strings.Contains(out, `Langkah 1 dari 3`) {
		t.Error("langkah 1 tidak memuat field biodata saja, atau Lanjut tidak ke langkah 2")
	}
	d.Sim = Sim{View: "step-2"}
	out, _ = k.RenderPage(d, "registrasi", "")
	if !strings.Contains(out, `>Telepon wali<span`) || !strings.Contains(out, `type="tel"`) || !strings.Contains(out, `<li class="done"><a href="/m/registrasi?view=step-1"`) {
		t.Error("langkah 2 tanpa field wali atau langkah selesai tanpa tautan")
	}
	d.Sim = Sim{View: "step-3"}
	out, _ = k.RenderPage(d, "registrasi", "")
	if !strings.Contains(out, `<dt>Biodata · Nama lengkap</dt>`) || !strings.Contains(out, `<input type="hidden" name="act" value="Simpan">`) {
		t.Error("langkah terakhir tidak meninjau isian atau tidak mengirim")
	}
}

// A section built from input components is a form's, whatever its words.
func TestInputSectionIsForm(t *testing.T) {
	k := kit(t)
	secs := k.AssignSlots("form", []core.Section{
		{ID: "konteks", Name: "Konteks", Content: []string{"Nama organisasi"}, Components: []string{"StatusBadge"}},
		{ID: "akun", Name: "Akun", Content: []string{"Field nama lengkap", "Tombol Terima undangan"}, Components: []string{"FormField"}},
	})
	if secs[1].Slot != "form" {
		t.Errorf("section input di slot %q", secs[1].Slot)
	}
}

func TestFontTokenNote(t *testing.T) {
	for _, v := range []string{"Inter 14px/20px 400 (mobile 16px/24px)", "Inter 14px/20px 500 tabular-nums"} {
		if f, ok := parseFont(v); !ok || f.family != "Inter" || f.size != "14px" {
			t.Errorf("%q: %+v %v", v, f, ok)
		}
	}
}
