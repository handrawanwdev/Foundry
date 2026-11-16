package core

import (
	"strings"
	"testing"
)

func flowLayout() *LayoutSpec {
	return &LayoutSpec{Pages: []LayoutPage{
		{ID: "pesanan", Name: "Pesanan", Pattern: "table", FeatureID: "FEAT-1",
			Sections: []Section{{ID: "tabel", Name: "Tabel pesanan", Content: []string{"Tabel: No, Pelanggan, Status"}}},
			Links:    []PageLink{{To: "pesanan-baru", Via: "Buat pesanan"}}},
		{ID: "pesanan-baru", Name: "Pesanan baru", Pattern: "form", FeatureID: "FEAT-1",
			Sections: []Section{{ID: "isian", Name: "Isian", Content: []string{"Pilih pelanggan, produk, kuantitas", "Tombol Ajukan"}}}},
		{ID: "kirim", Name: "Pengiriman", Pattern: "list", FeatureID: "FEAT-2",
			Sections: []Section{{ID: "daftar", Name: "Daftar pengiriman", Content: []string{"Surat jalan, kurir"}}}},
	}}
}

func flowProject() *Project {
	return &Project{
		Features: []Feature{{ID: "FEAT-1", Platforms: []string{PlatformWeb}}, {ID: "FEAT-2", Platforms: []string{PlatformWeb}}},
		Flows: []Flow{{ID: "FLOW-001", Name: "Pesan", FeatureID: "FEAT-1", PageIDs: []string{"pesanan", "pesanan-baru", "kirim"},
			Steps: []string{"Buat pesanan", "Pilih pelanggan dan produk", "Sistem cek stok", "Ajukan pesanan", "Bandingkan harga pesaing", "Cetak surat jalan"}}},
	}
}

// The steps of a use case land on its pages in order: a system step is
// feedback where the user is, a step no page names is flagged.
func TestMapFlowSteps(t *testing.T) {
	p, l := flowProject(), flowLayout()
	got := MapFlowSteps(l, p.Flows[0])
	want := []struct {
		page           string
		system, placed bool
	}{{"pesanan", false, true}, {"pesanan-baru", false, true}, {"pesanan-baru", true, true},
		{"pesanan-baru", false, true}, {"pesanan-baru", false, false}, {"kirim", false, true}}
	for i, w := range want {
		if st := got[i]; st.Page != w.page || st.System != w.system || st.Placed != w.placed {
			t.Errorf("langkah %d = %+v; want %+v", i+1, st, w)
		}
	}
}

func TestFlowIssues(t *testing.T) {
	p, l := flowProject(), flowLayout()
	issues := flowIssues(p, l, nil, UIDesignFormat)
	if Errors(issues) == nil || !strings.Contains(issueText(issues, "page:pesanan-baru"), `berlanjut ke "kirim"`) ||
		!strings.Contains(issueText(issues, "flow:FLOW-001"), "Bandingkan harga pesaing") {
		t.Fatalf("issues = %s", issueMessages(issues))
	}
	l.Pages[1].Links = []PageLink{{To: "kirim", Via: "Kirim"}}
	if errs := Errors(flowIssues(p, l, nil, UIDesignFormat)); errs != nil {
		t.Fatalf("linked flow still fails: %s", issueMessages(errs))
	}
	l.Pages[1].Links = nil
	if errs := Errors(flowIssues(p, l, nil, UIUXFormat)); errs != nil {
		t.Fatal("older design must only warn")
	}
	// A flow without pages is reported; its pages can be guessed.
	p.Flows[0].PageIDs = nil
	if !strings.Contains(issueText(flowIssues(p, l, nil, UIDesignFormat), "flow:FLOW-001"), "belum dipetakan") {
		t.Fatal("unmapped flow not reported")
	}
	if got := InferFlowPages(l, p.Flows[0]); strings.Join(got, ",") != "pesanan,pesanan-baru,kirim" {
		t.Fatalf("inferred pages = %v", got)
	}
}

func TestStepWords(t *testing.T) {
	for text, system := range map[string]bool{
		"Sistem verifikasi kredensial": true, "Buat ParkingSession": true, "Emit payment.completed": true,
		"Simpan dengan idempotency_key": true, "Sesi dibuat": true, "Masukkan kuantitas diterima": false,
		"Isi email dan password": false, "Pelanggan membayar": false,
	} {
		if SystemStep(text) != system {
			t.Errorf("SystemStep(%q) = %v", text, !system)
		}
	}
	if got := StepAction("buat penawaran atau langsung pesanan"); got != "Buat penawaran" {
		t.Errorf("StepAction = %q", got)
	}
	if got := StepAction("Buat DeviceCommand dengan idempotency_key"); got != "Buat DeviceCommand" {
		t.Errorf("cut label = %q", got)
	}
	if !CreateStep("Ajukan cuti") || CreateStep("Tinjau selisih") {
		t.Error("CreateStep")
	}
}

// Decision, approval request and irreversible steps are recognized through
// the Indonesian affixes (menyetujui ← setuju, memutuskan ← putus).
func TestStepKinds(t *testing.T) {
	if DecisionStep("Approver langkah pertama menerima notifikasi") || !DecisionStep("Approver memutuskan") {
		t.Error("an approver is not a decision")
	}
	for text, want := range map[string][3]bool{ // decision, approval request, risky
		"Supervisor menyetujui":       {true, false, false},
		"Approver memutuskan":         {true, false, false},
		"Manajer menolak permintaan":  {true, false, false},
		"Ajukan PO untuk persetujuan": {false, true, false},
		"Hapus pengguna":              {false, false, true},
		"Posting penerimaan":          {false, false, true},
		"Buat pesanan":                {false, false, false},
	} {
		if got := [3]bool{DecisionStep(text), ApprovalRequest(text), RiskyStep(text)}; got != want {
			t.Errorf("%q = %v; want %v", text, got, want)
		}
	}
	if !DangerAction("Tolak") || !DangerAction("Hapus") || DangerAction("Setujui") {
		t.Error("DangerAction")
	}
	if !stemMatch(stem("pengiriman"), stem("kirim")) || !stemMatch(stem("penerimaan"), stem("terima")) || stemMatch(stem("direktur"), stem("rektor")) {
		t.Error("candidate roots")
	}
}

// A rewritten use case keeps its screen mapping while the steps and
// branches it describes are unchanged.
func TestKeepScreens(t *testing.T) {
	old := Flow{Steps: []string{"a", "b"}, Failures: []string{"x"}, PageIDs: []string{"p"}, StepPages: []string{"p", ""}, BranchPages: []string{"p"}}
	same := Flow{Steps: []string{"a", "b"}, Failures: []string{"x"}}
	same.KeepScreens(old)
	if len(same.PageIDs) != 1 || len(same.StepPages) != 2 || len(same.BranchPages) != 1 {
		t.Fatalf("kept = %+v", same)
	}
	changed := Flow{Steps: []string{"a", "c"}, Failures: []string{"y"}}
	changed.KeepScreens(old)
	if len(changed.PageIDs) != 1 || changed.StepPages != nil || changed.BranchPages != nil {
		t.Fatalf("stale mapping kept = %+v", changed)
	}
}
