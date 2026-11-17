package core

import (
	"strings"
	"testing"
)

// Dimensions stay separate: a passing visual check does not hide a
// blocking business failure, and no combined score is produced.
func TestDesignBenchmarkSeparateDimensions(t *testing.T) {
	p, _ := scenario(lifecyclePages(), poFlow())
	p.Lifecycles = []Lifecycle{poLifecycle()}
	b := p.DesignBenchmark()
	if len(b.Dimensions) != 10 {
		t.Fatalf("dimensions = %d", len(b.Dimensions))
	}
	seen := map[string]bool{}
	kinds := map[string]bool{}
	for _, d := range b.Dimensions {
		if d.ID == "" || d.Kind == "" || d.Status == "" {
			t.Fatalf("empty dimension %+v", d)
		}
		if seen[d.ID] {
			t.Fatalf("duplicate %s", d.ID)
		}
		seen[d.ID], kinds[d.Kind] = true, true
	}
	for _, k := range []string{BenchStructural, BenchSemantic, BenchVisual, BenchInteraction, BenchBusiness} {
		if !kinds[k] {
			t.Errorf("missing kind %s", k)
		}
	}
	if b.Summary == "" || strings.Contains(strings.ToLower(b.Summary), "skor keseluruhan") {
		t.Errorf("summary = %q", b.Summary)
	}
	st := dimOf(b, "state-consistency")
	if st.Kind != BenchBusiness || st.Total != 5 || st.Covered != 5 {
		t.Errorf("state-consistency = %+v", st)
	}
}

func TestDesignBenchmarkBlockingNotAveraged(t *testing.T) {
	p, d := scenario(lifecyclePages(), poFlow())
	p.Lifecycles = []Lifecycle{poLifecycle()}
	// A MUST requirement with a use case and no screen is a blocking
	// semantic failure; other dimensions may still pass.
	p.Requirements = append(p.Requirements, Requirement{ID: "REQ-MUST", Text: "Wajib layar", Priority: "MUST"})
	p.Features = append(p.Features, Feature{ID: "FEAT-MUST", Name: "Wajib", RequirementIDs: []string{"REQ-MUST"}, Platforms: []string{PlatformWeb}})
	p.Flows = append(p.Flows, Flow{ID: "FLOW-MUST", Name: "Wajib", FeatureID: "FEAT-MUST", Role: "Operator",
		Steps: []string{"Lihat yang tidak ada"}, PageIDs: []string{"ghost"}})
	_ = d
	b := p.DesignBenchmark()
	req := dimOf(b, "requirement-coverage")
	if req.Status != GateFail || !req.Blocking {
		t.Fatalf("requirement-coverage = %+v", req)
	}
	if b.Ready || !containsStr(b.Blocking, req.Name) {
		t.Fatalf("blocking hidden: ready=%v blocking=%v", b.Ready, b.Blocking)
	}
	if vis := dimOf(b, "ux-quality"); vis.Status == GateFail && vis.ID == "" {
		t.Fatal("ux-quality missing")
	}
}

func TestDesignBenchmarkParallelNotForcedLinear(t *testing.T) {
	flow := Flow{ID: "FLOW-002", Name: "Persetujuan ganda", FeatureID: "FEAT-001", Role: "Operator",
		Steps:     []string{"Ajukan anggaran", "Keuangan menyetujui", "Legal menyetujui", "Sistem menerbitkan kontrak"},
		StepPages: []string{"ajuan", "keuangan", "legal", ""},
		Parallel:  []ParallelSteps{{Steps: []int{2, 3}, Join: "all", Then: 4}}}
	p, _ := scenario([]LayoutPage{
		{ID: "ajuan", Name: "Ajuan", Pattern: "form", Sections: []Section{sec("isi", "Nilai, alasan", "Tombol Ajukan")},
			Links: []PageLink{{To: "keuangan", Via: "Ajukan"}}},
		{ID: "keuangan", Name: "Keuangan", Pattern: "detail", Data: []string{"Queue"}, Sections: []Section{sec("info", "Setujui atau tolak")}},
		{ID: "legal", Name: "Legal", Pattern: "detail", Data: []string{"Queue"}, Sections: []Section{sec("info", "Setujui atau tolak")}},
	}, flow)
	nav := dimOf(p.DesignBenchmark(), "navigation-integrity")
	if strings.Contains(strings.Join(nav.Items, " "), "legal") {
		t.Fatalf("parallel pages forced to link: %+v", nav)
	}
}

func dimOf(b DesignBenchmark, id string) BenchDim {
	for _, d := range b.Dimensions {
		if d.ID == id {
			return d
		}
	}
	return BenchDim{}
}

func containsStr(in []string, s string) bool {
	for _, v := range in {
		if v == s {
			return true
		}
	}
	return false
}

// Nothing measured is not a pass. Seen live on the Parking-System and
// distra designs: flows whose pages are guessed made navigation "skip",
// use cases that change statuses without a lifecycle made the state
// dimension "skip", and page lists without a page per step counted as
// mapped by the designer.
func TestDesignBenchmarkUnmeasuredIsNotPass(t *testing.T) {
	flow := poFlow()
	flow.StepPages = nil
	flow.Data = []string{"PurchaseOrder: status menunggu persetujuan"}
	p, _ := scenario(lifecyclePages(), flow)
	b := p.DesignBenchmark()
	if st := dimOf(b, "state-consistency"); st.Status != GateWarn || !strings.Contains(strings.Join(st.Items, " "), "FLOW-002") {
		t.Errorf("status changes without a lifecycle = %+v", st)
	}
	if sm := dimOf(b, "screen-mapping"); sm.Status != GateWarn {
		t.Errorf("words matched as designer mapping = %+v", sm)
	}
	for i := range p.Flows {
		p.Flows[i].PageIDs = nil
	}
	if nav := dimOf(p.DesignBenchmark(), "navigation-integrity"); nav.Status != GateWarn {
		t.Errorf("guessed pages leave navigation = %+v", nav)
	}
}

// A flow placed step by step whose branches are matched by words is not
// fully mapped by the designer.
func TestDesignBenchmarkBranchesDeclared(t *testing.T) {
	flow := poFlow()
	flow.Alternatives = []string{"Langkah 2: Manajer menolak -> kembali ke langkah 1"}
	p, _ := scenario(lifecyclePages(), flow)
	p.Lifecycles = []Lifecycle{poLifecycle()}
	if sm := dimOf(p.DesignBenchmark(), "screen-mapping"); sm.Status != GateWarn || !strings.Contains(strings.Join(sm.Items, " "), "cabang") {
		t.Fatalf("words-matched branches counted as mapped: %+v", sm)
	}
	p.Flows[len(p.Flows)-1].BranchPages = []string{"keputusan"}
	if sm := dimOf(p.DesignBenchmark(), "screen-mapping"); strings.Contains(strings.Join(sm.Items, " "), "FLOW-002") {
		t.Fatalf("declared branches still reported: %+v", sm)
	}
}
