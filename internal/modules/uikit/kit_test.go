package uikit

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
)

func kit(t *testing.T) *Kit {
	t.Helper()
	return Load("")
}

func names(ts []core.Token) []string {
	var out []string
	for _, t := range ts {
		out = append(out, t.Name)
	}
	slices.Sort(out)
	return out
}

// Every preset carries the same token names, so a mockup and a brand work
// with any of them.
func TestPresetsShareTokenNames(t *testing.T) {
	k := kit(t)
	if len(k.Presets) < 4 {
		t.Fatalf("presets = %d", len(k.Presets))
	}
	ref := k.Presets[0].Tokens
	for _, p := range k.Presets[1:] {
		a, b := ref.Groups(), p.Tokens.Groups()
		for i := range a {
			if !slices.Equal(names(*a[i].Tokens), names(*b[i].Tokens)) {
				t.Errorf("preset %s: token %s berbeda dari %s", p.ID, a[i].Name, k.Presets[0].ID)
			}
		}
		for _, tok := range p.Tokens.Typography {
			if _, ok := parseFont(tok.Value); !ok {
				t.Errorf("preset %s: %s tidak terbaca: %q", p.ID, tok.Name, tok.Value)
			}
		}
	}
}

// Every template area names a slot of its pattern, every example fills its
// required slots and renders.
func TestPatternsAreConsistent(t *testing.T) {
	k := kit(t)
	for _, p := range k.Patterns {
		for _, row := range p.Areas {
			if len(row) != len(p.Areas[0]) {
				t.Errorf("%s: baris area tidak sama panjang", p.ID)
			}
			for _, c := range row {
				if _, ok := p.Slot(c); !ok {
					t.Errorf("%s: area %q bukan slot", p.ID, c)
				}
			}
		}
		if p.Shell {
			continue
		}
		if p.Example == nil {
			t.Errorf("%s: tanpa contoh", p.ID)
			continue
		}
		for _, id := range p.Panel {
			if _, ok := p.Slot(id); !ok {
				t.Errorf("%s: panel %q bukan slot", p.ID, id)
			}
		}
		filled := map[string]bool{}
		for _, s := range p.Example.Sections {
			if _, ok := p.Slot(s.Slot); !ok {
				t.Errorf("%s: contoh section %s slot %q tidak ada", p.ID, s.ID, s.Slot)
			}
			filled[s.Slot] = true
		}
		for _, s := range p.Slots {
			if s.Required && !filled[s.ID] {
				t.Errorf("%s: contoh tidak mengisi slot wajib %s", p.ID, s.ID)
			}
		}
		for _, c := range core.Collections(p.Example) {
			if c.Filter == "" || c.Pagination == "" {
				t.Errorf("%s: contoh %s tanpa filter/paginasi (%+v)", p.ID, c.Section, c)
			}
		}
		out, err := k.RenderReference(p.ID)
		if err != nil || !strings.Contains(out, `data-foundry-section="`) {
			t.Errorf("%s: render gagal: %v", p.ID, err)
		}
		if dir := os.Getenv("UIKIT_DUMP"); dir != "" {
			os.WriteFile(filepath.Join(dir, p.ID+".html"), []byte(out), 0o644)
		}
	}
}

func TestMaterializeLocksScaleAndAppliesBrand(t *testing.T) {
	k := kit(t)
	r := &core.DesignRules{Preset: "ops-dense", Brand: &core.DesignBrand{Primary: "#0e7490", Font: "IBM Plex Sans"},
		Tokens: core.DesignTokens{
			Spacing: []core.Token{{Name: "space.md", Value: "17px"}, {Name: "space.gutter", Value: "20px"}},
			Colors:  []core.Token{{Name: "status.overdue", Value: "#9f1239"}, {Name: "surface.page", Value: "#fafafa"}, {Name: "action.primary", Value: "#ff0000"}},
		}}
	k.Materialize(r)
	get := func(ts []core.Token, n string) string {
		if i := indexToken(ts, n); i >= 0 {
			return ts[i].Value
		}
		return ""
	}
	if v := get(r.Tokens.Spacing, "space.md"); v != "12px" {
		t.Errorf("space.md = %s, preset terkunci 12px", v)
	}
	if v := get(r.Tokens.Spacing, "space.gutter"); v != "20px" {
		t.Errorf("token tambahan hilang: %q", v)
	}
	if v := get(r.Tokens.Colors, "action.primary"); v != "#0e7490" {
		t.Errorf("brand kalah: %s", v)
	}
	if v := get(r.Tokens.Colors, "surface.page"); v != "#fafafa" {
		t.Errorf("warna rules tidak menimpa preset: %s", v)
	}
	if v := get(r.Tokens.Colors, "status.overdue"); v == "" {
		t.Error("warna domain hilang")
	}
	if v := get(r.Tokens.Typography, "type.body"); !strings.HasPrefix(v, "IBM Plex Sans 13px") {
		t.Errorf("font brand: %s", v)
	}
	if v := get(r.Tokens.Colors, "action.primary.text"); v != "#ffffff" {
		t.Errorf("teks di atas primary: %s", v)
	}
}

func TestAssignSlots(t *testing.T) {
	k := kit(t)
	secs := []core.Section{
		{ID: "saring", Name: "Filter status", Content: []string{"Status"}},
		{ID: "daftar-pesanan", Name: "Daftar pesanan", Pattern: "table", Content: []string{"Tabel: No, Nama"}},
		{ID: "lain", Name: "Lain-lain", Content: []string{"x"}},
		{ID: "x", Name: "X", Slot: "pagination", Content: []string{"20"}},
	}
	got := k.AssignSlots("table", secs)
	want := []string{"toolbar", "table", "table", "pagination"}
	for i := range got {
		if got[i].Slot != want[i] {
			t.Errorf("%s → %s, mau %s", got[i].ID, got[i].Slot, want[i])
		}
	}
	if secs[0].Slot != "" {
		t.Error("input berubah")
	}
	if got := k.AssignSlots("kanban", secs); got[3].Slot != "" {
		t.Error("pattern tanpa template tidak boleh punya slot")
	}
}

func design() Doc {
	return Doc{Name: "Toko <Maju>", Rules: &core.DesignRules{Preset: "saas-neutral"}, Layout: &core.LayoutSpec{
		Shells: []core.LayoutShell{{ID: "app", Name: "App", Pattern: "sidebar", Regions: []core.Section{{ID: "nav", Name: "Nav", Content: []string{"Menu"}}}}},
		Pages: []core.LayoutPage{{ID: "pesanan", Name: "Pesanan <script>alert(1)</script>", Platform: "web", Pattern: "table", Shell: "app",
			Purpose: "Saring pesanan", PrimaryAction: "Buat pesanan",
			Sections: []core.Section{{ID: "tabel", Name: "Tabel", Content: []string{"Tabel: No, Pelanggan, Total, Status"}}},
			States:   map[string]string{"empty": "Belum ada pesanan"}}},
		Navigation: []core.NavGroup{{Name: "Utama", Items: []core.NavItem{{Label: "Pesanan", Page: "pesanan"}}}},
	}}
}

func TestRenderPageEscapesAndMarksSections(t *testing.T) {
	k := kit(t)
	out, err := k.RenderPage(design(), "pesanan", "")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "<script>") {
		t.Error("teks desain tidak di-escape")
	}
	for _, want := range []string{`data-foundry-section="tabel"`, `data-slot="table"`, `--action-primary:#4f46e5`, `class="fk-side"`, "Buat pesanan", `class="fk-num"`} {
		if !strings.Contains(out, want) {
			t.Errorf("mockup tanpa %s", want)
		}
	}
	empty, _ := k.RenderPage(design(), "pesanan", "empty")
	if !strings.Contains(empty, "Belum ada pesanan") {
		t.Error("state empty tidak tampil")
	}
	if _, err := k.RenderPage(design(), "pesanan", "rusak"); err == nil {
		t.Error("state tak dikenal diterima")
	}
	if _, err := k.RenderPage(design(), "tidak-ada", ""); err == nil {
		t.Error("halaman tak dikenal diterima")
	}
}

// A table or list always renders with its search/filter and pagination:
// the template's toolbar and pagination sections join the table in one
// panel; a table without them gets them drawn in.
func TestCollectionsRenderControls(t *testing.T) {
	k := kit(t)
	d := design()
	out, _ := k.RenderPage(d, "pesanan", "")
	for _, want := range []string{`class="fk-panel"`, `class="fk-search"`, `aria-label="Paginasi"`, `class="fk-sec-head"`} {
		if !strings.Contains(out, want) {
			t.Errorf("tabel tanpa kontrol bawaan: %s", want)
		}
	}
	pg := &d.Layout.Pages[0]
	pg.Sections = append([]core.Section{{ID: "saring", Name: "Saring", Content: []string{"Cari no, Status, Tanggal"}}}, pg.Sections...)
	pg.Sections = append(pg.Sections, core.Section{ID: "halaman", Name: "Halaman", Slot: "pagination", Content: []string{"50 per halaman"}})
	out, _ = k.RenderPage(d, "pesanan", "")
	panel := out[strings.Index(out, `class="fk-panel"`):]
	a, b, c := strings.Index(panel, `data-foundry-section="saring"`), strings.Index(panel, `data-foundry-section="tabel"`), strings.Index(panel, `data-foundry-section="halaman"`)
	if a < 0 || b < a || c < b || strings.Count(out, `class="fk-search"`) != 1 || !strings.Contains(out, "1–50 dari") {
		t.Errorf("panel tidak menyatukan toolbar, tabel, paginasi (%d %d %d)", a, b, c)
	}
	d = design()
	d.Layout.Pages[0].Pattern = "dashboard"
	d.Layout.Pages[0].Sections = []core.Section{{ID: "kpi", Name: "KPI", Content: []string{"Omzet, Pesanan"}},
		{ID: "terbaru", Name: "Pesanan terbaru", Content: []string{"Tabel: No, Pelanggan, Status"}}}
	out, _ = k.RenderPage(d, "pesanan", "")
	if !strings.Contains(out, `class="fk-search"`) || !strings.Contains(out, `aria-label="Paginasi"`) {
		t.Error("tabel di dashboard tanpa pencarian/paginasi")
	}
}

func TestPageHashFollowsDesign(t *testing.T) {
	k := kit(t)
	d := design()
	h := k.PageHash(d, "pesanan")
	if h == "" || h != k.PageHash(design(), "pesanan") {
		t.Fatal("hash tidak stabil")
	}
	d.Layout.Pages[0].Sections[0].Content = []string{"Tabel: No, Total"}
	if k.PageHash(d, "pesanan") == h {
		t.Error("hash tidak berubah saat desain berubah")
	}
}

func TestGridCollapsesEmptySlots(t *testing.T) {
	k := kit(t)
	p, _ := k.Pattern("dashboard")
	css := gridCSS(p, true, []core.Section{{Slot: "metrics"}, {Slot: "main"}}, "", 768)
	if !strings.Contains(css, `grid-template-areas:"metrics" "main"`) || !strings.Contains(css, "grid-template-columns:minmax(0,1fr)") {
		t.Errorf("grid: %s", css)
	}
	css = gridCSS(p, true, []core.Section{{Slot: "main"}, {Slot: "aside"}}, "", 768)
	if !strings.Contains(css, `"main aside"`) {
		t.Errorf("grid dua kolom: %s", css)
	}
	// The data panel takes its toolbar's area; an empty slot never leaves
	// a non-rectangular area.
	md, _ := k.Pattern("master-detail")
	css = gridCSS(md, true, []core.Section{{Slot: "toolbar"}, {Slot: "list"}, {Slot: "detail"}}, "list", 768)
	if !strings.Contains(css, `grid-template-areas:"list detail"}`) {
		t.Errorf("grid master-detail: %s", css)
	}
	st, _ := k.Pattern("settings")
	css = gridCSS(st, true, []core.Section{{Slot: "nav"}, {Slot: "main"}}, "", 768)
	if !strings.Contains(css, `grid-template-areas:"nav main"}`) {
		t.Errorf("grid settings tanpa actions: %s", css)
	}
}

func TestOverlayReplacesPreset(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "presets"), 0o755)
	os.WriteFile(filepath.Join(dir, "presets", "brand.json"), []byte(`{"id":"brand-x","name":"Brand X","tokens":{"colors":[{"name":"action.primary","value":"#123456"}]}}`), 0o644)
	k := Load(dir)
	if _, ok := k.Preset("brand-x"); !ok {
		t.Fatal("preset pengguna tidak dimuat")
	}
	if k.Version == Load("").Version {
		t.Error("versi kit tidak berubah")
	}
}

// A page leads on to the next page of its use cases, shows the steps it
// hosts in order, and the index walks every use case step by step.
func TestMockupsFollowFlows(t *testing.T) {
	k := kit(t)
	d := design()
	d.Link = func(id string) string { return id + ".html" }
	d.Layout.Pages[0].PrimaryAction = ""
	d.Layout.Pages = append(d.Layout.Pages, core.LayoutPage{ID: "kirim", Name: "Pengiriman", Platform: "web", Pattern: "detail", Shell: "app",
		Sections: []core.Section{{ID: "info", Name: "Info", Content: []string{"Surat jalan, Kurir", "Aksi kirim atau batalkan"}}}})
	d.Layout.Pages[0].Links = []core.PageLink{{To: "kirim", Via: "tombol Kirim pesanan"}}
	d.Walks = []core.FlowWalk{{ID: "FLOW-001", Name: "Pesan", Pages: []string{"pesanan", "kirim"}, Steps: []core.FlowStep{
		{N: 1, Text: "Buat pesanan", Page: "pesanan", Placed: true},
		{N: 2, Text: "Sistem cek stok", System: true, Page: "pesanan", Placed: true},
		{N: 3, Text: "Ajukan pesanan", Page: "pesanan", Placed: true},
		{N: 4, Text: "Cetak surat jalan", Page: "kirim", Placed: true},
	}}}
	out, _ := k.RenderPage(d, "pesanan", "")
	for _, want := range []string{
		`<a class="fk-btn" href="kirim.html" data-foundry-link="kirim">Kirim pesanan`, // next page, via label
		`fk-btn-primary"><svg`, ">Buat pesanan</button>", // primary action from the flow's create step
		`data-foundry-flow="FLOW-001"`, `<li class="here on"`, `href="kirim.html"><span class="fk-flow-n">3</span>Cetak surat jalan`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("mockup tanpa %s", want)
		}
	}
	// A record's next actions are buttons, not fields; an irreversible one
	// is a destructive button that asks for confirmation.
	if out, _ := k.RenderPage(d, "kirim", ""); !strings.Contains(out, "fk-detail-actions") ||
		!strings.Contains(out, `fk-btn-primary">Kirim</button>`) ||
		!strings.Contains(out, `<button class="fk-btn fk-btn-danger" data-foundry-confirm="true">Batalkan</button>`) {
		t.Error("aksi detail tidak jadi tombol")
	}
	if strings.Contains(out, "Sistem cek stok") {
		t.Error("langkah sistem tampil sebagai langkah pengguna")
	}
	d.Walks[0].Inferred = true
	if out, _ := k.RenderPage(d, "pesanan", ""); strings.Contains(out, `data-foundry-link="kirim"`) {
		t.Error("halaman tebakan dijadikan link")
	}
	d.Walks[0].Trigger = "Pelanggan menelepon"
	d.Walks[0].Steps[2].Decision = true
	d.Walks[0].Branches = []core.FlowBranch{{Kind: core.BranchFailure, Text: "Stok habis", From: 2, Page: "pesanan", Placed: true},
		{Kind: core.BranchAlternative, Text: "Belum bayar", From: 3, Flows: []string{"FLOW-002"}}}
	idx := k.RenderIndex(d, nil)
	for _, want := range []string{"Alur use case", "Otomatis · Pesanan", "Halaman ditebak", "<b>Pemicu</b> Pelanggan menelepon",
		">Keputusan</span>", ">Gagal</span> <span class=\"fk-meta\">di langkah 2</span> Stok habis", "Belum terlihat", "→ FLOW-002"} {
		if !strings.Contains(idx, want) {
			t.Errorf("indeks tanpa %s", want)
		}
	}
}
