package work

import (
	"slices"
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
)

// The designer's gaps in following the use cases are filled: an unmapped
// flow gets the pages its steps name, consecutive pages are linked, and a
// list page a flow starts on gets the create step as its primary action.
func TestCompleteFlows(t *testing.T) {
	p := &core.Project{Features: []core.Feature{{ID: "FEAT-1", Platforms: []string{core.PlatformWeb}}}}
	l := &core.LayoutSpec{Pages: []core.LayoutPage{
		{ID: "pesanan", Name: "Pesanan", Pattern: "table", FeatureID: "FEAT-1",
			Sections: []core.Section{{ID: "tabel", Content: []string{"Tabel pesanan: No, Pelanggan, Status"}}}},
		{ID: "kirim", Name: "Pengiriman", Pattern: "list",
			Sections: []core.Section{{ID: "daftar", Content: []string{"Surat jalan, kurir"}}}},
	}}
	flows := []core.Flow{
		{ID: "FLOW-001", FeatureID: "FEAT-1", Steps: []string{"Buat pesanan", "Sistem cek stok", "Cetak surat jalan"}},
		{ID: "FLOW-002", FeatureID: "FEAT-1", PageIDs: []string{"kirim", "pesanan"}, Steps: []string{"Pilih kurir", "Tandai pesanan"}},
	}
	completeFlows(p, l, flows)
	if !slices.Equal(flows[0].PageIDs, []string{"pesanan", "kirim"}) {
		t.Fatalf("pages = %v", flows[0].PageIDs)
	}
	if ln := l.Pages[0].Links; len(ln) != 1 || ln[0].To != "kirim" || ln[0].Via != "Cetak surat jalan" {
		t.Fatalf("pesanan links = %+v", ln)
	}
	if ln := l.Pages[1].Links; len(ln) != 1 || ln[0].To != "pesanan" {
		t.Fatalf("kirim links = %+v", ln)
	}
	if l.Pages[0].PrimaryAction != "Buat pesanan" || l.Pages[1].PrimaryAction != "" {
		t.Fatalf("primary actions = %q, %q", l.Pages[0].PrimaryAction, l.Pages[1].PrimaryAction)
	}
	check := *p
	check.Flows = flows
	for _, i := range check.DesignIssues(&core.UIDesign{Format: core.UIDesignFormat, Layout: l}) {
		if strings.Contains(i.Message, "berlanjut ke") {
			t.Fatalf("flow link still missing: %+v", i)
		}
	}
}

// The designer may place each step and scenario on its page; the older
// answer (a list of pages) still lands. A list that does not match the
// steps is dropped so the words place them instead.
func TestApplyFlowsOut(t *testing.T) {
	var out designOut
	raw := `{"flows": {"FLOW-001": ["A", "b"], "FLOW-002": {"steps": ["daftar", "", "Form"], "branches": ["form"]}, "FLOW-003": {"steps": ["x"]}}}`
	if err := ExtractJSON(raw, &out); err != nil {
		t.Fatal(err)
	}
	flows := []core.Flow{
		{ID: "FLOW-001", Steps: []string{"a"}},
		{ID: "FLOW-002", Steps: []string{"Buka daftar", "Sistem cek", "Isi form"}, Failures: []string{"Gagal simpan"}},
		{ID: "FLOW-003", Steps: []string{"a", "b"}, StepPages: []string{"x", "y"}},
	}
	if err := applyFlowsOut(flows, out.Flows); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(flows[0].PageIDs, []string{"a", "b"}) {
		t.Errorf("older form = %v", flows[0].PageIDs)
	}
	if f := flows[1]; !slices.Equal(f.StepPages, []string{"daftar", "", "form"}) || !slices.Equal(f.BranchPages, []string{"form"}) ||
		!slices.Equal(f.PageIDs, []string{"daftar", "form"}) {
		t.Errorf("step form = %+v", f)
	}
	if flows[2].StepPages != nil {
		t.Errorf("mismatched step list kept: %v", flows[2].StepPages)
	}
	if err := applyFlowsOut(flows, map[string]flowOut{"FLOW-404": {}}); err == nil {
		t.Error("unknown flow accepted")
	}
}

// A placement on a page that does not exist falls back to the words, and
// a scenario handing over to another use case links to where it starts.
func TestCompleteFlowsDeclaredAndHandOver(t *testing.T) {
	p := &core.Project{Features: []core.Feature{{ID: "FEAT-1", Platforms: []string{core.PlatformWeb}}}}
	l := &core.LayoutSpec{Pages: []core.LayoutPage{
		{ID: "keluar", Name: "Gerbang keluar", Pattern: "detail", Sections: []core.Section{{ID: "info", Content: []string{"Plat, status bayar"}}}},
		{ID: "kasir", Name: "Kasir", Pattern: "form", Sections: []core.Section{{ID: "bayar", Content: []string{"Jumlah", "Tombol Bayar"}}}},
	}}
	flows := []core.Flow{
		{ID: "FLOW-001", FeatureID: "FEAT-1", Steps: []string{"Periksa status bayar"}, StepPages: []string{"hilang"},
			PageIDs: []string{"keluar"}, Alternatives: []string{"Belum lunas: arahkan ke pembayaran (FLOW-002)"}},
		{ID: "FLOW-002", FeatureID: "FEAT-1", Steps: []string{"Bayar tagihan"}, StepPages: []string{"kasir"}},
	}
	completeFlows(p, l, flows)
	if flows[0].StepPages != nil || !slices.Equal(flows[1].PageIDs, []string{"kasir"}) {
		t.Fatalf("flows = %+v", flows)
	}
	if ln := l.Pages[0].Links; len(ln) != 1 || ln[0].To != "kasir" || ln[0].Via != "Arahkan ke pembayaran" {
		t.Fatalf("hand-over link = %+v", ln)
	}
}
