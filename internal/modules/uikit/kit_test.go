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
	} {
		if !strings.Contains(out, want) {
			t.Errorf("mockup tanpa %s", want)
		}
	}
	// A record's next actions are buttons, not fields; an irreversible one
	// is a destructive button that asks for confirmation.
	if out, _ := k.RenderPage(d, "kirim", ""); !strings.Contains(out, "fk-detail-actions") ||
		!strings.Contains(out, `fk-btn-primary">Kirim</button>`) ||
		!strings.Contains(out, `fk-btn-danger" data-foundry-confirm="true"`) || !strings.Contains(out, ">Batalkan</button>") {
		t.Error("aksi detail tidak jadi tombol")
	}
	if strings.Contains(out, "fk-flow") || strings.Contains(out, "Sistem cek stok") {
		t.Error("halaman mockup masih menampilkan langkah use case")
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

// Prototype simulation is explicit: a banner, HTML5 validation, a
// confirmation without script, and URL role/status that disable the
// wrong actions. Nothing is stored. Hash ignores the simulation.
func TestMockupSimulation(t *testing.T) {
	k := kit(t)
	p := &core.Project{Name: "PO",
		Lifecycles: []core.Lifecycle{{Entity: "PurchaseOrder",
			States: []core.LifeState{{ID: "draft", Kind: core.StateInitial}, {ID: "pending", Label: "Menunggu"},
				{ID: "approved", Locked: true, Kind: core.StateFinal}},
			Transitions: []core.LifeTransition{
				{From: []string{"draft"}, To: "pending", Action: "Ajukan", Kind: core.TransSubmit, Actor: "Operator", Flow: "FLOW-1", Step: 1},
				{From: []string{"pending"}, To: "approved", Action: "Setujui", Kind: core.TransApprove, Actor: "Manajer", Flow: "FLOW-1", Step: 2},
				{From: []string{"pending"}, To: "draft", Action: "Tolak", Kind: core.TransReject, Actor: "Manajer", Flow: "FLOW-1", Step: 2},
			}}},
		Flows: []core.Flow{{ID: "FLOW-1", Name: "PO", FeatureID: "FEAT-1", Role: "Operator",
			Steps: []string{"Ajukan PO", "Manajer memutuskan"}, StepPages: []string{"po", "keputusan"}}},
		Features: []core.Feature{{ID: "FEAT-1", Platforms: []string{core.PlatformWeb}}},
		UI: &core.UIDesign{Layout: &core.LayoutSpec{Pages: []core.LayoutPage{
			{ID: "po", Name: "PO", Pattern: "form", Role: "Operator", PrimaryAction: "Ajukan",
				Sections: []core.Section{{ID: "isi", Name: "Isi", Content: []string{"Nomor", "Catatan", "Tombol Ajukan"}}}},
			{ID: "keputusan", Name: "Keputusan", Pattern: "detail", Role: "Manajer", PrimaryAction: "Setujui",
				Sections: []core.Section{{ID: "info", Name: "Info", Content: []string{"Nomor", "Aksi Setujui atau Tolak"}}}},
		}}}}
	d := DocOf(p)
	d.Link = func(id string) string { return "/m/" + id }
	d.Self = "/m/po"
	d.Sim = Sim{Role: "Operator", Status: "draft"}
	out, err := k.RenderPage(d, "po", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`data-foundry-sim="banner"`, "Simulasi prototype — tidak tersimpan",
		`data-foundry-sim="form"`, `required`, `method="get"`,
		`data-foundry-sim="status"`, "Draf",
	} {
		if !strings.Contains(out, want) && !(want == "Draf" && !strings.Contains(out, "draft") && !strings.Contains(out, "Draf")) {
			if !strings.Contains(out, want) && want != "Draf" {
				t.Errorf("tanpa %s", want)
			}
		}
	}
	if !strings.Contains(out, "draft") && !strings.Contains(out, "Draf") {
		t.Error("status simulasi tidak tampil")
	}
	d.Sim = Sim{Role: "Operator", Status: "approved"}
	locked, _ := k.RenderPage(d, "po", "")
	if !strings.Contains(locked, " disabled") {
		t.Error("form dokumen terkunci masih bisa diisi")
	}
	d.Self, d.Sim = "/m/keputusan", Sim{Role: "Manajer", Status: "pending"}
	dec, _ := k.RenderPage(d, "keputusan", "")
	if !strings.Contains(dec, `data-foundry-confirm="true"`) || !strings.Contains(dec, `data-foundry-sim="confirm"`) {
		t.Error("setujui/tolak tanpa konfirmasi simulasi")
	}
	if !strings.Contains(dec, "Alasan penolakan") {
		t.Error("tolak tanpa alasan wajib")
	}
	d.Sim = Sim{Role: "Operator", Status: "pending"}
	wrong, _ := k.RenderPage(d, "keputusan", "")
	if !strings.Contains(wrong, `data-foundry-sim="role"`) {
		t.Error("peran salah tidak ditandai")
	}
	a, b := k.PageHash(d, "po"), k.PageHash(DocOf(p), "po")
	if a == "" || a != b {
		t.Errorf("hash mengikuti simulasi: %s vs %s", a, b)
	}
}

// A page's document actions follow the lifecycle whatever words the page
// uses for them (seen live on distra: "Aksi sesuai status: Ajukan (draft);
// Setujui atau Tolak (submitted)" rendered as text): every transition
// taken there becomes a control, enabled only in its status for its actor;
// a next-step link that is a transition of another page is gated too.
func TestMockupDocumentActionsFollowLifecycle(t *testing.T) {
	k := kit(t)
	p := &core.Project{Name: "PR",
		Lifecycles: []core.Lifecycle{{Entity: "PurchaseRequest",
			States: []core.LifeState{{ID: "draft", Label: "Draf", Kind: core.StateInitial}, {ID: "submitted", Label: "Diajukan", Locked: true},
				{ID: "approved", Label: "Disetujui", Locked: true}, {ID: "converted", Kind: core.StateFinal}},
			Transitions: []core.LifeTransition{
				{From: []string{"draft"}, To: "submitted", Action: "Ajukan", Kind: core.TransSubmit, Actor: "Purchasing", Flow: "FLOW-1", Step: 1},
				{From: []string{"submitted"}, To: "approved", Action: "Setujui", Kind: core.TransApprove, Actor: "Manajer", Flow: "FLOW-1", Step: 2},
				{From: []string{"submitted"}, To: "draft", Action: "Tolak", Kind: core.TransReject, Actor: "Manajer", Flow: "FLOW-1", Step: 2},
				{From: []string{"approved"}, To: "converted", Action: "Konversi menjadi RFQ", Kind: core.TransComplete, Actor: "Purchasing", Flow: "FLOW-1", Step: 3},
			}}},
		Flows: []core.Flow{{ID: "FLOW-1", Name: "PR", FeatureID: "FEAT-1", Role: "Purchasing",
			Steps: []string{"Ajukan PR", "Manajer memutuskan", "Konversi menjadi RFQ"}, StepPages: []string{"pr", "pr", "rfq"}}},
		Features: []core.Feature{{ID: "FEAT-1", Platforms: []string{core.PlatformWeb}}},
		UI: &core.UIDesign{Layout: &core.LayoutSpec{Pages: []core.LayoutPage{
			{ID: "pr", Name: "PR", Pattern: "detail", Role: "Purchasing, Manajer", Data: []string{"PurchaseRequest"},
				Links:    []core.PageLink{{To: "rfq", Via: "Konversi menjadi RFQ"}},
				Sections: []core.Section{{ID: "info", Name: "Info", Content: []string{"Nomor", "Aksi sesuai status: Ajukan (draft); Setujui atau Tolak (submitted)"}}}},
			{ID: "rfq", Name: "RFQ", Pattern: "detail", Role: "Purchasing", Sections: []core.Section{{ID: "info", Name: "Info", Content: []string{"Nomor"}}}},
		}}}}
	d := DocOf(p)
	d.Link = func(id string) string { return "/m/" + id }
	d.Self = "/m/pr"
	button := func(out, label string) string {
		i := strings.Index(out, ">"+label+"</button>")
		if i < 0 {
			return ""
		}
		return out[strings.LastIndex(out[:i], "<button"):i]
	}
	d.Sim = Sim{Role: "Manajer", Status: "submitted"}
	out, _ := k.RenderPage(d, "pr", "")
	if b := button(out, "Setujui"); b == "" || strings.Contains(b, "disabled") {
		t.Errorf("manajer cannot approve a submitted PR: %q", b)
	}
	if b := button(out, "Tolak"); b == "" || strings.Contains(b, "disabled") || !strings.Contains(b, "fk-btn-danger") {
		t.Errorf("rejection = %q", b)
	}
	if b := button(out, "Ajukan PR"); !strings.Contains(b, "disabled") {
		t.Errorf("submit offered in status submitted: %q", b)
	}
	if b := button(out, "Konversi menjadi RFQ"); !strings.Contains(b, "disabled") {
		t.Errorf("link to a later transition not gated: %q", b)
	}
	d.Sim = Sim{Role: "Purchasing", Status: "approved"}
	out, _ = k.RenderPage(d, "pr", "")
	if !strings.Contains(out, `data-foundry-link="rfq"`) {
		t.Error("link not taken in the status that allows it")
	}
	if b := button(out, "Setujui"); !strings.Contains(b, "disabled") {
		t.Errorf("approve offered after approval: %q", b)
	}
	// A plain mockup (no scenario) stays as designed: its hash is stable.
	if a, b := k.PageHash(d, "pr"), k.PageHash(DocOf(p), "pr"); a == "" || a != b {
		t.Errorf("hash follows the simulation: %s vs %s", a, b)
	}
}

// Seen live on distra: "Sales" enabled the approval of "Sales Manager"
// (one name contains the other), and "Batalkan sebelum konfirmasi" was
// enabled on a confirmed order because another cancel transition was.
func TestMockupSimRolesAndSameKindActions(t *testing.T) {
	k := kit(t)
	p := &core.Project{Name: "SO",
		Lifecycles: []core.Lifecycle{{Entity: "SalesOrder",
			States: []core.LifeState{{ID: "draft", Kind: core.StateInitial}, {ID: "pending"}, {ID: "confirmed", Locked: true},
				{ID: "cancelled", Kind: core.StateFinal}},
			Transitions: []core.LifeTransition{
				{From: []string{"draft"}, To: "pending", Action: "Ajukan", Kind: core.TransSubmit, Actor: "Sales", Flow: "FLOW-1", Step: 1},
				{From: []string{"pending"}, To: "confirmed", Action: "Setujui pesanan", Kind: core.TransApprove, Actor: "Sales Manager", Flow: "FLOW-1", Step: 2},
				{From: []string{"pending"}, To: "draft", Action: "Tolak pesanan", Kind: core.TransReject, Actor: "Sales Manager", Flow: "FLOW-1", Step: 2},
				{From: []string{"draft"}, To: "cancelled", Action: "Batalkan sebelum konfirmasi", Kind: core.TransCancel, Actor: "Sales", Flow: "FLOW-1", Step: 1},
				{From: []string{"confirmed"}, To: "cancelled", Action: "Batalkan pesanan terkonfirmasi", Kind: core.TransCancel, Actor: "Sales", Flow: "FLOW-1", Step: 1},
			}}},
		Flows: []core.Flow{{ID: "FLOW-1", Name: "SO", FeatureID: "FEAT-1", Role: "Sales",
			Steps: []string{"Ajukan pesanan", "Sales Manager memutuskan"}, StepPages: []string{"so", "so"}},
			{ID: "FLOW-2", Name: "Approve", FeatureID: "FEAT-1", Role: "Sales Manager", Steps: []string{"Lihat"}}},
		Features: []core.Feature{{ID: "FEAT-1", Platforms: []string{core.PlatformWeb}}},
		UI: &core.UIDesign{Layout: &core.LayoutSpec{Pages: []core.LayoutPage{
			{ID: "so", Name: "SO", Pattern: "detail", Role: "Sales, Sales Manager", Data: []string{"SalesOrder"},
				Sections: []core.Section{{ID: "info", Name: "Info", Content: []string{"Nomor"}}}},
		}}}}
	d := DocOf(p)
	d.Link = func(id string) string { return "/m/" + id }
	d.Self = "/m/so"
	enabled := func(out, label string) bool {
		i := strings.Index(out, ">"+label+"</button>")
		return i >= 0 && !strings.Contains(out[strings.LastIndex(out[:i], "<button"):i], "disabled")
	}
	d.Sim = Sim{Role: "Sales", Status: "pending"}
	out, _ := k.RenderPage(d, "so", "")
	if enabled(out, "Setujui pesanan") || enabled(out, "Tolak pesanan") {
		t.Error("Sales may decide for Sales Manager")
	}
	d.Sim = Sim{Role: "Sales Manager", Status: "pending"}
	if out, _ = k.RenderPage(d, "so", ""); !enabled(out, "Setujui pesanan") {
		t.Error("Sales Manager cannot approve")
	}
	d.Sim = Sim{Role: "Sales", Status: "confirmed"}
	out, _ = k.RenderPage(d, "so", "")
	if enabled(out, "Batalkan sebelum konfirmasi") || !enabled(out, "Batalkan pesanan terkonfirmasi") {
		t.Error("cancel transitions matched by kind instead of words")
	}
}

// A stepper numbers the step names itself; one step is no stepper; what
// names no step is a quality gate finding, not a step.
func TestStepsBlock(t *testing.T) {
	k := kit(t)
	d := design()
	pg := &d.Layout.Pages[0]
	pg.Pattern = "wizard"
	pg.Sections = []core.Section{{ID: "langkah", Name: "Langkah", Slot: "steps", Content: []string{"1 Data sekolah, 2 Akun admin, lalu 3D model"}}}
	out, _ := k.RenderPage(d, pg.ID, "")
	if !strings.Contains(out, `<b>1</b>Data sekolah</li>`) || !strings.Contains(out, `<b>3</b>3D model</li>`) {
		t.Error("stepper tidak memakai nomornya sendiri")
	}
	p := &core.Project{UI: &core.UIDesign{Rules: d.Rules, Layout: d.Layout}}
	if c := k.ContentCheck(p); c.Status != core.GatePass || !strings.HasPrefix(c.Detail, "1 stepper,") {
		t.Errorf("stepper rapi tidak lolos: %+v", c)
	}

	pg.Sections[0].Content = []string{"Langkah: pilih file, validasi checksum", "Gagal validasi: data lama tetap utuh"}
	out, _ = k.RenderPage(d, pg.ID, "")
	if !strings.Contains(out, `<b>2</b>Validasi checksum</li>`) || strings.Contains(out, "data lama tetap utuh") {
		t.Error("catatan tampil sebagai langkah")
	}
	if c := k.ContentCheck(p); c.Status != core.GateWarn || len(c.Items) != 1 || !strings.Contains(c.Items[0], `"Gagal validasi: data lama tetap utuh" bukan nama langkah`) {
		t.Errorf("catatan di stepper tidak dilaporkan: %+v", c)
	}

	pg.Sections[0].Content = []string{"Pilih rentang tanggal lalu simpan CSV"}
	if out, _ := k.RenderPage(d, pg.ID, ""); strings.Contains(out, `class="fk-steps"`) {
		t.Error("satu langkah tampil sebagai stepper")
	}
	if c := k.ContentCheck(p); c.Status != core.GateWarn || !strings.Contains(strings.Join(c.Items, " "), "minimal 2 nama langkah") {
		t.Errorf("stepper satu langkah tidak dilaporkan: %+v", c)
	}
	if g := p.DesignGate(); !slices.ContainsFunc(g.Checks, func(c core.GateCheck) bool { return c.ID == "component-content" }) {
		t.Error("pemeriksaan stepper tidak ada di quality gate")
	}
}

// A dialog the page opens is drawn after the page's content as a dialog,
// not in a slot of the page; a form drawn as a table is a gate finding.
func TestPageDialogs(t *testing.T) {
	k := kit(t)
	d := design()
	pg := &d.Layout.Pages[0]
	pg.Sections = append(pg.Sections, core.Section{ID: "dialog-pengguna", Name: "Dialog pengguna", Slot: "table",
		Content: []string{"Field Nama, Email, Kata sandi sementara dan Peran", "Dialog konfirmasi hapus pengguna"}})
	out, _ := k.RenderPage(d, pg.ID, "")
	i := strings.Index(out, `class="fk-dialogs"`)
	if i < 0 {
		t.Fatal("dialog tidak tergambar")
	}
	dlg := out[i:]
	for _, want := range []string{`role="dialog" aria-label="Dialog pengguna"`, `<span class="fk-badge">Dialog</span>`,
		`>Kata sandi sementara</label>`, `type="password"`, `Batal</button><button class="fk-btn fk-btn-primary" type="button">Simpan`,
		`role="alertdialog"`, `<p>Konfirmasi hapus pengguna</p>`, `fk-btn-danger" type="button">Hapus</button>`} {
		if !strings.Contains(dlg, want) {
			t.Errorf("dialog tanpa %s", want)
		}
	}
	if strings.Contains(out[:i], "dialog-pengguna") {
		t.Error("dialog ikut tergambar di slot halaman")
	}
	if out, _ := k.RenderPage(d, pg.ID, "loading"); strings.Contains(out, `class="fk-dialogs"`) {
		t.Error("dialog tampil di state loading")
	}
	p := &core.Project{UI: &core.UIDesign{Rules: d.Rules, Layout: d.Layout}}
	if c := k.ContentCheck(p); c.Status != core.GatePass {
		t.Errorf("dialog dilaporkan: %+v", c)
	}

	pg.Sections[len(pg.Sections)-1].ID, pg.Sections[len(pg.Sections)-1].Name = "pengguna-baru", "Pengguna baru"
	pg.Sections[len(pg.Sections)-1].Content = []string{"Field Nama, Email", "Tombol Simpan"}
	if c := k.ContentCheck(p); c.Status != core.GateWarn || !strings.Contains(strings.Join(c.Items, " "), "Pengguna baru: tergambar sebagai tabel padahal isinya field form") {
		t.Errorf("form yang jadi tabel tidak dilaporkan: %+v", c)
	}
}
