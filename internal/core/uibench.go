package core

import (
	"fmt"
	"strings"
)

// Design quality benchmark: each dimension is reported on its own. A
// passing visual check does not hide a missing MUST screen or a broken
// document status. Nothing here is a single product score.

// Benchmark kinds: what a dimension judges.
const (
	BenchStructural  = "structural"
	BenchSemantic    = "semantic"
	BenchVisual      = "visual"
	BenchInteraction = "interaction"
	BenchBusiness    = "business"
)

// BenchDim is one measured dimension of a design. Covered/Total are the
// countable items of that dimension alone (requirement traces, steps,
// branches, …). Blocking means a fail here keeps the design unready
// regardless of every other dimension.
type BenchDim struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Kind     string   `json:"kind"`
	Status   string   `json:"status"`
	Covered  int      `json:"covered"`
	Total    int      `json:"total"`
	Blocking bool     `json:"blocking,omitempty"`
	Detail   string   `json:"detail"`
	Items    []string `json:"items,omitempty"`
}

// DesignBenchmark is the per-dimension report of a design. Ready is false
// when a blocking dimension fails; it is not an average.
type DesignBenchmark struct {
	Dimensions []BenchDim `json:"dimensions"`
	Blocking   []string   `json:"blocking,omitempty"`
	Ready      bool       `json:"ready"`
	Summary    string     `json:"summary"`
}

// DesignBenchmark measures the current design on separate dimensions.
func (p *Project) DesignBenchmark() DesignBenchmark {
	g := p.DesignGate()
	check := map[string]GateCheck{}
	for _, c := range g.Checks {
		check[c.ID] = c
	}
	reqCov, reqTot := 0, 0
	for _, r := range g.Requirements {
		if r.Status == TraceNoScreen || r.Status == TraceUnlinked {
			continue
		}
		reqTot++
		if r.Status == TraceCovered {
			reqCov++
		}
	}
	stepCov, stepTot, declared, mapped := 0, 0, 0, 0
	brCov, brTot := 0, 0
	for _, f := range g.Flows {
		stepCov, stepTot = stepCov+f.Placed, stepTot+f.Steps
		brCov, brTot = brCov+f.Shown, brTot+f.Branches
		mapped++
		if f.Declared && !f.Inferred {
			declared++
		}
	}
	b := DesignBenchmark{Dimensions: []BenchDim{
		fromCheck("requirement-coverage", "Cakupan requirement", BenchSemantic, check["requirement-screen"], reqCov, reqTot, true),
		fromCheck("workflow-coverage", "Cakupan workflow", BenchSemantic, check["usecase-screen"], stepCov, stepTot, true),
		fromCheck("branch-coverage", "Cakupan cabang", BenchSemantic, check["exception-flow"], brCov, brTot, false),
		fromCheck("screen-mapping", "Akurasi pemetaan layar", BenchStructural, screenMapCheck(g.Flows), declared, mapped, false),
		fromCheck("action-coverage", "Cakupan aksi", BenchInteraction, check["primary-action"], countHave(check["primary-action"]), countTot(check["primary-action"]), false),
		fromCheck("state-consistency", "Konsistensi status", BenchBusiness, check["lifecycle"], lifecycleTraced(p), lifecycleTotal(p), true),
		fromCheck("navigation-integrity", "Integritas navigasi", BenchStructural, check["workflow-navigation"], 0, 0, true),
		fromCheck("role-consistency", "Konsistensi peran", BenchBusiness, check["role-permission"], 0, 0, false),
		fromCheck("exception-handling", "Penanganan pengecualian", BenchBusiness, check["exception-flow"], brCov, brTot, false),
		uxQuality(check),
	}}
	fillCounts(&b)
	for _, d := range b.Dimensions {
		if d.Blocking && d.Status == GateFail {
			b.Blocking = append(b.Blocking, d.Name)
		}
	}
	b.Ready = len(b.Blocking) == 0 && g.Errors == 0
	switch {
	case len(b.Dimensions) == 0:
		b.Summary = "Belum ada rancangan halaman."
	case len(b.Blocking) > 0:
		b.Summary = "Dimensi yang memblokir: " + strings.Join(b.Blocking, "; ") + "."
	case !b.Ready:
		b.Summary = fmt.Sprintf("Desain masih punya %d error rancangan.", g.Errors)
	default:
		warn := 0
		for _, d := range b.Dimensions {
			if d.Status == GateWarn {
				warn++
			}
		}
		if warn == 0 {
			b.Summary = "Setiap dimensi lolos. Tidak digabung menjadi satu skor."
		} else {
			b.Summary = fmt.Sprintf("Tidak ada dimensi yang memblokir; %d dimensi perlu ditinjau. Tidak digabung menjadi satu skor.", warn)
		}
	}
	return b
}

func fromCheck(id, name, kind string, c GateCheck, covered, total int, blocking bool) BenchDim {
	d := BenchDim{ID: id, Name: name, Kind: kind, Status: GateSkip, Covered: covered, Total: total, Blocking: blocking}
	if c.ID != "" {
		d.Status, d.Detail, d.Items = c.Status, c.Detail, append([]string{}, c.Items...)
	}
	if d.Status == GateSkip && (covered > 0 || total > 0) {
		d.Status = GatePass
	}
	if d.Detail == "" && total > 0 {
		d.Detail = fmt.Sprintf("%d dari %d", covered, total)
	}
	return d
}

func screenMapCheck(flows []FlowTrace) GateCheck {
	c := check("screen-mapping", "Akurasi pemetaan layar")
	n := 0
	for _, f := range flows {
		if f.Inferred {
			n++
			c.note(GateWarn, "%s %s: halaman ditebak, belum dipetakan desainer", f.ID, f.Name)
		}
	}
	if len(flows) == 0 {
		c.Status = GateSkip
	}
	c.Detail = fmt.Sprintf("%d dari %d use case dipetakan desainer", len(flows)-n, len(flows))
	return c
}

func uxQuality(check map[string]GateCheck) BenchDim {
	d := BenchDim{ID: "ux-quality", Name: "Kualitas UX", Kind: BenchVisual, Status: GatePass,
		Detail: "visual, responsif, dan integritas prototype — masing-masing tetap di quality gate"}
	for _, id := range []string{"visual-consistency", "responsive", "prototype-integrity"} {
		c := check[id]
		if c.ID == "" {
			continue
		}
		if c.Status == GateFail || c.Status == GateWarn && d.Status == GatePass {
			d.Status = c.Status
		}
		d.Items = append(d.Items, c.Items...)
		if id == "prototype-integrity" && c.Status == GateFail {
			d.Blocking = true
		}
	}
	if d.Status == GatePass && len(d.Items) == 0 && check["visual-consistency"].ID == "" {
		d.Status = GateSkip
	}
	return d
}

func lifecycleTotal(p *Project) int {
	n := 0
	for _, lc := range p.Lifecycles {
		n += len(lc.Transitions)
	}
	return n
}

func lifecycleTraced(p *Project) int {
	n := 0
	for _, lc := range p.Lifecycles {
		for _, t := range lc.Transitions {
			if t.Flow != "" && t.Step > 0 {
				n++
			}
		}
	}
	return n
}

func countHave(c GateCheck) int { return pair(c.Detail) }
func countTot(c GateCheck) int {
	_, t := parsePair(c.Detail)
	return t
}

func pair(detail string) int {
	h, _ := parsePair(detail)
	return h
}

func parsePair(detail string) (int, int) {
	var have, tot int
	if _, err := fmt.Sscanf(detail, "%d dari %d", &have, &tot); err != nil {
		return 0, 0
	}
	return have, tot
}

func fillCounts(b *DesignBenchmark) {
	for i, d := range b.Dimensions {
		if d.Total > 0 || d.Covered > 0 {
			continue
		}
		if h, t := parsePair(d.Detail); t > 0 {
			b.Dimensions[i].Covered, b.Dimensions[i].Total = h, t
		}
	}
}
