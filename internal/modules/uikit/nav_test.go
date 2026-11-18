package uikit

import (
	"regexp"
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
)

func navProject() *core.Project {
	return &core.Project{Name: "Trade", UI: &core.UIDesign{Layout: &core.LayoutSpec{
		Shells:     []core.LayoutShell{{ID: "app", Name: "App", Pattern: "sidebar"}},
		Navigation: []core.NavGroup{{Name: "Kelola", Items: []core.NavItem{{Label: "Sumber", Page: "sumber"}, {Label: "Log", Page: "log"}}}},
		Pages: []core.LayoutPage{
			{ID: "sumber", Name: "Sumber Chart dan Data", Pattern: "table", Shell: "app", FeatureID: "F1", PrimaryAction: "Daftarkan sumber",
				Fields: []core.FieldRule{{Field: "DataSource.name", Label: "Nama", Presentation: "table-column"}, {Field: "DataSource.id", Label: "Cek kelayakan", Presentation: "row-action"}},
				Sections: []core.Section{
					{ID: "tabel", Name: "Daftar sumber", Slot: "table", Content: []string{"Nama, Jenis, Kelayakan, Aksi (Cek kelayakan)"}, Components: []string{"Table"}},
					{ID: "pagination", Name: "Paginasi", Slot: "pagination", Content: []string{"Paginasi"}, Components: []string{"Pagination"}},
				},
				Links: []core.PageLink{{To: "sumber-form", Via: "Daftarkan sumber atau Cek kelayakan"}}},
			{ID: "sumber-form", Name: "Daftarkan Sumber", Pattern: "form", Shell: "app", FeatureID: "F1", PrimaryAction: "Simpan hasil cek kelayakan",
				Sections: []core.Section{
					{ID: "form", Name: "Data sumber", Slot: "form", Content: []string{"Nama, Jenis"}, Components: []string{"Input"}},
					{ID: "actions", Name: "Aksi", Slot: "actions", Content: []string{"Simpan hasil cek kelayakan", "Batal"}, Components: []string{"Button"}},
				},
				Links: []core.PageLink{{To: "sumber", Via: "Simpan atau Batal"}}},
			{ID: "log", Name: "Log", Pattern: "table", Shell: "app", FeatureID: "F2",
				Sections: []core.Section{
					{ID: "toolbar", Name: "Filter", Slot: "toolbar", Content: []string{"Tab: Notifikasi, Audit"}},
					{ID: "tabel", Name: "Tabel", Slot: "table", Content: []string{"Tab Notifikasi: Peristiwa, Waktu", "Tab Audit: Aksi audit, Aktor"}, Components: []string{"Table"}},
				}},
		},
	}}}
}

// deadButton finds a button that neither opens, submits nor is disabled:
// a click on it does nothing.
var buttonRe = regexp.MustCompile(`<button[^>]*>`)

func deadButtons(out string) []string {
	var dead []string
	for _, b := range buttonRe.FindAllString(out, -1) {
		if !strings.Contains(b, "popovertarget") && !strings.Contains(b, "disabled") && !strings.Contains(b, `type="submit"`) && !strings.Contains(b, " form=") {
			dead = append(dead, b)
		}
	}
	return dead
}

// Every control of a list page goes somewhere: the row action to the form
// its link names, holding the row; the primary action to the form; the
// pages of the collection to themselves.
func TestRowActionFollowsLinks(t *testing.T) {
	k := kit(t)
	d := DocOf(navProject())
	d.Link = func(id string) string { return "/m/" + id }
	d.Self = "/m/sumber"
	out, err := k.RenderPage(d, "sumber", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`href="/m/sumber-form?fill=1" data-foundry-link="sumber-form">Cek kelayakan</a>`,
		`href="/m/sumber-form" data-foundry-link="sumber-form">`, `href="/m/sumber?pg=2"`} {
		if !strings.Contains(out, want) {
			t.Errorf("halaman sumber tanpa %s", want)
		}
	}
	if dead := deadButtons(out); len(dead) > 0 {
		t.Errorf("tombol tanpa tujuan: %v", dead)
	}
	d.Sim = Sim{Page: 2}
	out, _ = k.RenderPage(d, "sumber", "")
	if !strings.Contains(out, `class="fk-pg on" aria-current="page" href="/m/sumber?pg=2"`) {
		t.Error("halaman 2 paginasi tidak tertandai")
	}
	d.Self, d.Sim = "/m/sumber-form", Sim{}
	form, _ := k.RenderPage(d, "sumber-form", "")
	if !strings.Contains(form, `href="/m/sumber" data-foundry-link="sumber">Batal</a>`) {
		t.Error("Batal di form tidak kembali ke sumber")
	}
}

// A tab opens on the page: the URL carries it, the open tab is marked and
// the table shows that tab's columns.
func TestTabsOpen(t *testing.T) {
	k := kit(t)
	d := DocOf(navProject())
	d.Link = func(id string) string { return "/m/" + id }
	d.Self = "/m/log"
	out, _ := k.RenderPage(d, "log", "")
	if !strings.Contains(out, `href="/m/log?tab=2"`) || !strings.Contains(out, ">Peristiwa</th>") {
		t.Fatal("tab Audit tidak bisa dibuka atau tab pertama tidak tampil")
	}
	d.Sim = Sim{Tab: 2}
	out, _ = k.RenderPage(d, "log", "")
	if !strings.Contains(out, `class="on" aria-current="page">Audit</a>`) || !strings.Contains(out, ">Aktor</th>") {
		t.Error("tab Audit terbuka tanpa tanda atau tanpa kolomnya")
	}
}
