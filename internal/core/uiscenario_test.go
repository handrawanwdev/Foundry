package core

import (
	"slices"
	"strings"
	"testing"
)

// Business scenarios the screens must follow: simple CRUD, a multi-step
// procurement placed by the designer, multi-level approval, branching,
// rejection and repetition, hand-over to another use case, role-based
// access, operational monitoring and a mobile screen. Each builds on the
// valid clinic design (uxProject, uxDesign) and adds its pages and use
// case.

// scenario adds pages (each with a goal, a decision and the required
// states, so only what the scenario checks is reported) and use cases.
func scenario(pages []LayoutPage, flows ...Flow) (*Project, *UIDesign) {
	p, d := uxProject(), uxDesign()
	// The clinic's own use case is placed step by step on its page.
	p.Flows[0].StepPages = []string{"antrean", "antrean", "antrean"}
	base := &d.Layout.Pages[0]
	base.Role, base.States["permission_denied"] = "Operator", "Tanpa izin antrean"
	for _, pg := range pages {
		pg.Platform, pg.Goal = firstNonBlank(pg.Platform, PlatformWeb), "GOAL-1"
		if pg.Route == "" {
			pg.Route = "/" + pg.ID
		}
		if pg.Purpose == "" {
			pg.Purpose = pg.Name
		}
		if pg.States == nil {
			pg.States = map[string]string{}
		}
		for _, st := range RequiredStates[pg.Pattern] {
			if pg.States[st] == "" {
				pg.States[st] = "keterangan " + st
			}
		}
		d.Layout.Pages = append(d.Layout.Pages, pg)
		d.Layout.Decisions = append(d.Layout.Decisions, DesignDecision{Subject: pg.ID, Choice: pg.Pattern,
			Why: []string{"tugas pengguna"}, Basis: []string{"GOAL-1"}, Source: "fact"})
		d.Layout.Navigation[0].Items = append(d.Layout.Navigation[0].Items, NavItem{Label: pg.Name, Page: pg.ID})
	}
	p.Flows = append(p.Flows, flows...)
	p.UI = d
	return p, d
}

func sec(id string, content ...string) Section {
	return Section{ID: id, Name: id, Content: content}
}

func walkOf(t *testing.T, p *Project, id string) FlowWalk {
	t.Helper()
	for _, w := range FlowWalks(p, p.UI.Layout) {
		if w.ID == id {
			return w
		}
	}
	t.Fatalf("no walk %s", id)
	return FlowWalk{}
}

func checkOf(g DesignGate, id string) GateCheck {
	for _, c := range g.Checks {
		if c.ID == id {
			return c
		}
	}
	return GateCheck{}
}

func TestScenarioCRUD(t *testing.T) {
	p, _ := scenario([]LayoutPage{
		{ID: "produk", Name: "Produk", Pattern: "list", FeatureID: "FEAT-001", PrimaryAction: "Tambah produk",
			Sections: []Section{sec("daftar", "Daftar produk: nama, harga", "Filter: cari produk", "Paginasi: 20")},
			Links:    []PageLink{{To: "produk-baru", Via: "Tambah produk"}}, Data: []string{"Queue"}},
		{ID: "produk-baru", Name: "Produk baru", Pattern: "form", FeatureID: "FEAT-001", PrimaryAction: "Simpan produk",
			Sections: []Section{sec("isian", "Nama, harga", "Tombol Simpan produk")},
			States:   map[string]string{"error": "Harga wajib diisi"}},
	}, Flow{ID: "FLOW-002", Name: "Tambah produk", FeatureID: "FEAT-001", Role: "Operator",
		Steps:    []string{"Tambah produk", "Isi nama dan harga", "Simpan produk"},
		Failures: []string{"Langkah 2: harga kosong -> pesan wajib diisi"}, PageIDs: []string{"produk", "produk-baru"}})
	w := walkOf(t, p, "FLOW-002")
	for _, st := range w.Steps {
		if !st.Placed {
			t.Errorf("step %d not placed: %+v", st.N, st)
		}
	}
	if br := w.Branches[0]; br.From != 2 || br.Page != "produk-baru" || !br.Placed {
		t.Errorf("failure branch = %+v", br)
	}
	g := p.DesignGate()
	if !g.Ready || checkOf(g, "usecase-screen").Status != GatePass || checkOf(g, "exception-flow").Status != GatePass {
		t.Fatalf("gate = %s %+v", g.Summary, g.Checks)
	}
}

// The designer places every step: no word has to match, the route follows
// the steps, and a page that does not exist is an error.
func TestScenarioProcurementDeclared(t *testing.T) {
	flow := Flow{ID: "FLOW-002", Name: "Pengadaan", FeatureID: "FEAT-001", Role: "Operator",
		Steps:     []string{"Buat permintaan", "Sistem memberi nomor", "Bandingkan penawaran", "Terbitkan PO"},
		StepPages: []string{"pr-baru", "", "rfq", "po"}}
	p, d := scenario([]LayoutPage{
		{ID: "pr-baru", Name: "Permintaan baru", Pattern: "form", Sections: []Section{sec("isian", "Barang, jumlah", "Tombol Kirim")},
			Links: []PageLink{{To: "rfq", Via: "Lanjut"}}},
		{ID: "rfq", Name: "Perbandingan", Pattern: "table", PrimaryAction: "Pilih pemenang",
			Sections: []Section{sec("tabel", "Supplier, harga, termin", "Filter: supplier", "Paginasi: 20")}, Data: []string{"Queue"}},
		{ID: "po", Name: "Purchase order", Pattern: "detail", PrimaryAction: "Kirim ke supplier",
			Sections: []Section{sec("info", "Nomor, supplier, total")}, Data: []string{"Queue"}},
	}, flow)
	w := walkOf(t, p, "FLOW-002")
	if !w.Declared || strings.Join(w.Pages, ",") != "pr-baru,rfq,po" || !w.Steps[2].Placed || !w.Steps[1].System {
		t.Fatalf("walk = %+v", w)
	}
	issues := flowIssues(p, d.Layout, d.Rules, UIDesignFormat)
	if !strings.Contains(issuesAt(issues, "page:rfq"), `berlanjut ke "po"`) {
		t.Fatalf("missing link rfq -> po not reported: %s", issueMessages(issues))
	}
	p.Flows[1].StepPages[3] = "hilang"
	if !strings.Contains(issuesAt(flowIssues(p, d.Layout, d.Rules, UIDesignFormat), "flow:FLOW-002"), `halaman "hilang" yang tidak ada`) {
		t.Fatal("unknown step page not reported")
	}
}

// An approval needs a screen that approves and rejects; the decision step
// is placed there whatever its words.
func TestScenarioMultiLevelApproval(t *testing.T) {
	flow := Flow{ID: "FLOW-002", Name: "Persetujuan PO", FeatureID: "FEAT-001", Role: "Operator",
		Steps:    []string{"Ajukan PO untuk persetujuan", "Manajer memutuskan", "Direktur menyetujui PO di atas ambang"},
		Failures: []string{"Manajer menolak -> kembali ke langkah 1"}, PageIDs: []string{"po", "keputusan"}}
	pages := []LayoutPage{
		{ID: "po", Name: "PO", Pattern: "detail", PrimaryAction: "Ajukan", Sections: []Section{sec("info", "Nomor, total", "Aksi ajukan")},
			Links: []PageLink{{To: "keputusan", Via: "Ajukan PO"}}, Data: []string{"Queue"}},
		{ID: "keputusan", Name: "Keputusan", Pattern: "detail", PrimaryAction: "Lihat", Sections: []Section{sec("info", "Nomor, total")}, Data: []string{"Queue"}},
	}
	p, d := scenario(pages, flow)
	if !strings.Contains(issuesAt(flowIssues(p, d.Layout, d.Rules, UIDesignFormat), "flow:FLOW-002"), "butuh keputusan") {
		t.Fatal("approval without a decision screen not reported")
	}
	pages[1].Sections = []Section{sec("info", "Nomor, total", "Aksi setujui atau tolak")}
	p, d = scenario(pages, flow)
	if msg := issuesAt(flowIssues(p, d.Layout, d.Rules, UIDesignFormat), "flow:FLOW-002"); strings.Contains(msg, "butuh keputusan") {
		t.Fatalf("decision screen not recognized: %s", msg)
	}
	w := walkOf(t, p, "FLOW-002")
	if st := w.Steps[1]; !st.Decision || !st.Placed || st.Page != "keputusan" {
		t.Errorf("decision step = %+v", st)
	}
	if br := w.Branches[0]; br.From != 3 || br.Next != "step:1" {
		t.Errorf("rejection branch = %+v", br)
	}
}

// Branches say where they leave and where they go: back to a step, end,
// cancel or another use case. A hand-over needs a link to that use case.
func TestScenarioBranchingAndHandOver(t *testing.T) {
	p, d := scenario([]LayoutPage{
		{ID: "keluar", Name: "Gerbang keluar", Pattern: "detail", PrimaryAction: "Buka barrier",
			Sections: []Section{sec("info", "Plat, tarif, status bayar")}, Data: []string{"Queue"}},
		{ID: "kasir", Name: "Kasir", Pattern: "form", Sections: []Section{sec("bayar", "Jumlah, metode", "Tombol Bayar")}},
	}, Flow{ID: "FLOW-002", Name: "Keluar", FeatureID: "FEAT-001", Role: "Operator", PageIDs: []string{"keluar"},
		Steps:        []string{"Periksa status bayar", "Buka barrier"},
		Alternatives: []string{"Langkah 1: belum lunas -> arahkan ke pembayaran (FLOW-003)", "Langkah 9: tidak ada", "Langkah 2: barrier macet, ulangi langkah 2"},
		Failures:     []string{"Pelanggan membatalkan: batal"}},
		Flow{ID: "FLOW-003", Name: "Bayar", FeatureID: "FEAT-001", Role: "Operator", PageIDs: []string{"kasir"}, Steps: []string{"Bayar tagihan"}})
	w := walkOf(t, p, "FLOW-002")
	if br := w.Branches[0]; br.From != 1 || !slices.Equal(br.Flows, []string{"FLOW-003"}) {
		t.Errorf("hand-over = %+v", br)
	}
	if w.Branches[2].Next != "step:2" || w.Branches[3].Next != "cancel" {
		t.Errorf("next = %q, %q", w.Branches[2].Next, w.Branches[3].Next)
	}
	issues := flowIssues(p, d.Layout, d.Rules, UIDesignFormat)
	if msg := issueMessages(issues); !strings.Contains(msg, "langkah 9 yang tidak ada") || !strings.Contains(msg, "use case FLOW-003 (kasir)") {
		t.Fatalf("branch issues = %s", msg)
	}
	if c := checkOf(p.DesignGate(), "workflow-navigation"); c.Status != GateWarn {
		t.Errorf("hand-over without link: %+v", c)
	}
}

func TestScenarioRoleBasedAndRisky(t *testing.T) {
	p, d := scenario([]LayoutPage{
		{ID: "pengguna", Name: "Pengguna", Pattern: "table", PrimaryAction: "Undang", Access: "auth",
			Sections: []Section{sec("tabel", "Nama, peran", "Filter: peran", "Paginasi: 20")}, Data: []string{"Queue"}},
	}, Flow{ID: "FLOW-002", Name: "Hapus pengguna", FeatureID: "FEAT-001", Role: "Operator", PageIDs: []string{"pengguna"},
		Steps: []string{"Pilih pengguna", "Hapus pengguna"}, Permissions: []string{"user:delete"}})
	if !strings.Contains(issuesAt(flowIssues(p, d.Layout, d.Rules, UIDesignFormat), "page:pengguna"), "konfirmasi") {
		t.Fatal("irreversible step without confirmation not reported")
	}
	if c := checkOf(p.DesignGate(), "role-permission"); c.Status != GateWarn {
		t.Fatalf("role check = %+v", c)
	}
	pg := &d.Layout.Pages[len(d.Layout.Pages)-1]
	pg.Role, pg.States["permission_denied"] = "Operator", "Hanya admin"
	d.Rules.UX = append(d.Rules.UX, "Konfirmasi sebelum menghapus")
	if strings.Contains(issuesAt(flowIssues(p, d.Layout, d.Rules, UIDesignFormat), "page:pengguna"), "konfirmasi") {
		t.Fatal("a confirmation guideline covers every page")
	}
	if c := checkOf(p.DesignGate(), "role-permission"); c.Status != GatePass {
		t.Fatalf("role check = %+v", c)
	}
}

func TestScenarioMonitoringAndMobile(t *testing.T) {
	p, _ := scenario([]LayoutPage{
		{ID: "pantau", Name: "Pantau gerbang", Pattern: "dashboard", Sections: []Section{sec("kpi", "Okupansi, antrean, alert")}, Data: []string{"Queue"}},
		{ID: "tugas", Name: "Tugas", Platform: PlatformAndroid, Pattern: "table", PrimaryAction: "Ambil tugas",
			Sections: []Section{sec("tabel", "No, Lokasi, Barang, Jumlah, Status, Petugas", "Filter: status", "Paginasi: 20")}, Data: []string{"Queue"}},
	}, Flow{ID: "FLOW-002", Name: "Pantau", FeatureID: "FEAT-001", Role: "Operator", PageIDs: []string{"pantau"},
		Steps: []string{"Pantau okupansi", "Tindak lanjuti alert"}})
	g := p.DesignGate()
	if c := checkOf(g, "primary-action"); c.Status != GateWarn || !strings.Contains(strings.Join(c.Items, " "), "pantau") {
		t.Errorf("monitoring page without action: %+v", c)
	}
	if c := checkOf(g, "responsive"); c.Status != GateWarn {
		t.Errorf("wide mobile table: %+v", c)
	}
}

// A MUST requirement with a use case and no screen blocks the gate; one
// without a use case (non-functional) is only reviewed.
func TestDesignGateRequirementTrace(t *testing.T) {
	p, _ := scenario(nil)
	p.Requirements = []Requirement{{ID: "REQ-001", Text: "Cek antrean", Priority: "MUST"},
		{ID: "REQ-002", Text: "Atur shift", Priority: "MUST"}, {ID: "REQ-003", Text: "Backup harian", Priority: "MUST"}}
	p.Features = append(p.Features,
		Feature{ID: "FEAT-002", Name: "Shift", Platforms: []string{PlatformWeb}, RequirementIDs: []string{"REQ-002"}},
		Feature{ID: "FEAT-003", Name: "Backup", Platforms: []string{PlatformWeb}, RequirementIDs: []string{"REQ-003"}})
	p.Flows = append(p.Flows, Flow{ID: "FLOW-002", Name: "Atur shift", FeatureID: "FEAT-002", Role: "Operator", Steps: []string{"Atur shift"}})
	g := p.DesignGate()
	got := map[string]string{}
	for _, r := range g.Requirements {
		got[r.ID] = r.Status
	}
	if got["REQ-001"] != TraceCovered || got["REQ-002"] != TraceMissing || got["REQ-003"] != TraceMissing {
		t.Fatalf("traces = %v", got)
	}
	if g.Ready || g.Verdict != VerdictBlocked || checkOf(g, "requirement-screen").Status != GateFail {
		t.Fatalf("gate = %s %+v", g.Verdict, checkOf(g, "requirement-screen"))
	}
	if tr := g.Requirements[0]; !slices.Contains(tr.Pages, "antrean") || !slices.Contains(tr.Actions, "Panggil") {
		t.Errorf("REQ-001 trace = %+v", tr)
	}
}

// issuesAt joins every message reported at where.
func issuesAt(issues []Issue, where string) string {
	var out []string
	for _, i := range issues {
		if i.Where == where {
			out = append(out, i.Message)
		}
	}
	return strings.Join(out, "\n")
}
