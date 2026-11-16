package core

import (
	"strings"
	"testing"
)

// poLifecycle is a purchase order approved by a manager: submitted from a
// draft, approved or returned for revision, then sent and locked.
func poLifecycle() Lifecycle {
	return Lifecycle{Entity: "PurchaseOrder",
		States: []LifeState{{ID: "draft", Label: "Draf", Kind: StateInitial}, {ID: "pending_approval", Label: "Menunggu persetujuan"},
			{ID: "approved", Label: "Disetujui", Locked: true}, {ID: "sent", Label: "Terkirim", Kind: StateFinal, Locked: true},
			{ID: "cancelled", Label: "Dibatalkan", Kind: StateFinal, Locked: true}},
		Transitions: []LifeTransition{
			{From: []string{"draft"}, To: "pending_approval", Action: "Ajukan", Kind: TransSubmit, Actor: "Operator", Flow: "FLOW-002", Step: 1},
			{From: []string{"pending_approval"}, To: "approved", Action: "Setujui", Kind: TransApprove, Actor: "Manajer", Flow: "FLOW-002", Step: 2},
			{From: []string{"pending_approval"}, To: "draft", Action: "Kembalikan", Kind: TransReject, Actor: "Manajer", Flow: "FLOW-002", Step: 2},
			{From: []string{"approved"}, To: "sent", Action: "Kirim ke supplier", Kind: TransComplete, Actor: "Operator", Flow: "FLOW-002", Step: 3},
			{From: []string{"draft", "pending_approval"}, To: "cancelled", Action: "Batalkan", Kind: TransCancel, Actor: "Operator", Flow: "FLOW-002", Step: 1},
		}}
}

func poFlow() Flow {
	return Flow{ID: "FLOW-002", Name: "Persetujuan PO", FeatureID: "FEAT-001", Role: "Operator, Manajer",
		Steps:     []string{"Ajukan PO", "Manajer memeriksa PO", "Kirim PO ke supplier"},
		StepPages: []string{"po", "keputusan", "po"}}
}

func TestLifecycleIssues(t *testing.T) {
	p := uxProject()
	flows := append(p.Flows, poFlow())
	if issues := p.LifecycleIssues([]Lifecycle{poLifecycle()}, flows); len(issues) > 0 {
		t.Fatalf("valid lifecycle reported: %s", issueMessages(issues))
	}
	lc := poLifecycle()
	lc.Transitions = lc.Transitions[:2] // no rejection, approved and draft lose their way out
	lc.Transitions = append(lc.Transitions, LifeTransition{From: []string{"approved"}, To: "lost", Action: "Arsip", Actor: "Sistem"},
		LifeTransition{From: []string{"sent"}, To: "draft", Action: "Buka lagi", Actor: "Direksi", Flow: "FLOW-009"},
		LifeTransition{From: []string{"draft"}, To: "sent", Action: "Langsung", Actor: "Operator", Flow: "FLOW-002", Step: 7})
	msg := issueMessages(p.LifecycleIssues([]Lifecycle{lc}, flows))
	for _, want := range []string{
		`status tujuan "lost" tidak ada`, "FLOW-009 yang tidak ada", "langkah 7 dari FLOW-002",
		`aktor "Direksi" bukan peran`, `status akhir "sent" masih punya transisi keluar`,
		`"pending_approval" bisa disetujui (Setujui) tetapi tidak bisa ditolak`, `status "cancelled" tidak bisa dicapai`,
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("missing %q in %s", want, msg)
		}
	}
	if !strings.Contains(issueMessages(p.LifecycleIssues([]Lifecycle{lc, lc}, flows)), "lebih dari satu siklus") {
		t.Error("duplicate lifecycle not reported")
	}
}

func TestLifecyclePath(t *testing.T) {
	lc := poLifecycle()
	path := lc.PathTo("sent")
	if len(path) != 3 || path[0].Action != "Ajukan" || path[2].Action != "Kirim ke supplier" {
		t.Fatalf("path = %+v", path)
	}
	if lc.PathTo("draft") != nil || len(lc.Out("pending_approval")) != 3 {
		t.Fatal("initial status or its way out")
	}
	if got := lc.TransitionText(lc.Transitions[1]); got != "PurchaseOrder: Menunggu persetujuan → Disetujui" {
		t.Fatalf("text = %q", got)
	}
}

// The decision of a flow with a lifecycle comes from its transitions, not
// from its words; each step shows the status change it makes.
func TestLifecycleMarksSteps(t *testing.T) {
	p, _ := scenario(lifecyclePages(), poFlow())
	p.Lifecycles = []Lifecycle{poLifecycle()}
	w := walkOf(t, p, "FLOW-002")
	if !w.Steps[1].Decision || w.Steps[0].Decision || len(w.Steps[1].Status) != 2 {
		t.Fatalf("steps = %+v", w.Steps)
	}
	byPage := p.TransitionsByPage(p.UI.Layout)
	if len(byPage["keputusan"]) != 2 || len(byPage["po"]) != 3 {
		t.Fatalf("transitions by page = %+v", byPage)
	}
}

func lifecyclePages() []LayoutPage {
	return []LayoutPage{
		{ID: "po", Name: "PO", Pattern: "detail", PrimaryAction: "Ajukan", Data: []string{"Queue"},
			Sections: []Section{sec("info", "Nomor, total", "Status: Draf, Menunggu persetujuan, Terkirim")},
			Links:    []PageLink{{To: "keputusan", Via: "Ajukan"}}},
		{ID: "keputusan", Name: "Keputusan", Pattern: "detail", PrimaryAction: "Setujui", Data: []string{"Queue"},
			Sections: []Section{sec("info", "Nomor, total", "Tombol Setujui dan Tolak dengan konfirmasi")},
			Links:    []PageLink{{To: "po", Via: "Kirim"}}},
	}
}

// The screens follow the lifecycle: approval and rejection of a status on
// one screen, only the document's statuses shown, no form editing a locked
// document.
func TestLifecycleUIIssues(t *testing.T) {
	p, d := scenario(lifecyclePages(), poFlow())
	p.Lifecycles = []Lifecycle{poLifecycle()}
	if issues := lifecycleUIIssues(p, d.Layout); len(issues) > 0 {
		t.Fatalf("consistent screens reported: %s", issueMessages(issues))
	}
	d.Layout.Pages[1].Sections[0].Content = append(d.Layout.Pages[1].Sections[0].Content, "Status (Menunggu, Ditunda)")
	lc := &p.Lifecycles[0]
	lc.Transitions[2].Step = 1 // rejected on the PO page, approved on the decision page
	p.Flows[1].StepPages[2] = "kirim"
	d.Layout.Pages = append(d.Layout.Pages, LayoutPage{ID: "kirim", Name: "Kirim", Pattern: "form", Sections: []Section{sec("isi", "Catatan")}})
	msg := issueMessages(lifecycleUIIssues(p, d.Layout))
	for _, want := range []string{"ditolak di po", `status "Ditunda" tidak ada di siklus PurchaseOrder`, `menjalankan "Kirim ke supplier" pada PurchaseOrder yang terkunci`} {
		if !strings.Contains(msg, want) {
			t.Errorf("missing %q in %s", want, msg)
		}
	}
	if strings.Contains(msg, `"Menunggu"`) {
		t.Errorf("a status matching by its root reported: %s", msg)
	}
	if c := checkOf(p.DesignGate(), "lifecycle"); c.Status != GateWarn || len(c.Items) < 3 {
		t.Errorf("gate check = %+v", c)
	}
}

// Steps done at the same time are not forced into a sequence: the pages of
// parallel approvals need no link between them; the groups are validated.
func TestParallelSteps(t *testing.T) {
	flow := Flow{ID: "FLOW-002", Name: "Persetujuan ganda", FeatureID: "FEAT-001", Role: "Operator",
		Steps:     []string{"Ajukan anggaran", "Keuangan menyetujui", "Legal menyetujui", "Sistem menerbitkan kontrak"},
		StepPages: []string{"ajuan", "keuangan", "legal", ""},
		Parallel:  []ParallelSteps{{Steps: []int{2, 3}, Join: "all", Then: 4, Timeout: "2 hari", OnTimeout: "eskalasi ke Direktur"}}}
	p, d := scenario([]LayoutPage{
		{ID: "ajuan", Name: "Ajuan", Pattern: "form", Sections: []Section{sec("isi", "Nilai, alasan", "Tombol Ajukan")},
			Links: []PageLink{{To: "keuangan", Via: "Ajukan"}}},
		{ID: "keuangan", Name: "Keuangan", Pattern: "detail", Data: []string{"Queue"}, Sections: []Section{sec("info", "Setujui atau tolak")}},
		{ID: "legal", Name: "Legal", Pattern: "detail", Data: []string{"Queue"}, Sections: []Section{sec("info", "Setujui atau tolak")}},
	}, flow)
	if msg := issuesAt(flowIssues(p, d.Layout, d.Rules, UIDesignFormat), "page:keuangan"); strings.Contains(msg, "berlanjut") {
		t.Fatalf("parallel approvals forced into a sequence: %s", msg)
	}
	if w := walkOf(t, p, "FLOW-002"); w.Steps[1].Group != 1 || w.Steps[2].Group != 1 || w.Steps[0].Group != 0 || len(w.Parallel) != 1 {
		t.Fatalf("walk = %+v", w)
	}
	if c := checkOf(p.DesignGate(), "workflow-navigation"); strings.Contains(strings.Join(c.Items, " "), "legal") {
		t.Fatalf("parallel pages required to link: %+v", c)
	}
	flow.Parallel = []ParallelSteps{{Steps: []int{2, 9}, Join: "3", Then: 2}, {Steps: []int{2}}, {Steps: []int{3, 4}, OnTimeout: "eskalasi"}}
	msg := issueMessages(p.UseCaseIssues([]Flow{flow}))
	for _, want := range []string{"langkah 9 tidak ada", `join "3"`, "then 2", "paling sedikit dua", "sudah ada di kelompok paralel lain", "timeout) belum"} {
		if !strings.Contains(msg, want) {
			t.Errorf("missing %q in %s", want, msg)
		}
	}
}
