package spec

import (
	"fmt"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
)

// gateStatus is the label of a gate check status.
var gateStatus = map[string]string{core.GatePass: "lolos", core.GateWarn: "tinjau", core.GateFail: "gagal", core.GateSkip: "tidak berlaku"}

// traceStatus is the label of a requirement trace status.
var traceStatus = map[string]string{core.TraceCovered: "terwakili", core.TracePartial: "sebagian", core.TraceMissing: "belum ada layar",
	core.TraceNoScreen: "tanpa layar", core.TraceUnlinked: "tanpa fitur"}

// designGateMarkdown is the quality gate report of the UI design: the
// checks, requirement and use case coverage, and the assumptions to
// confirm. A design is not complete because every page exists.
func designGateMarkdown(p *core.Project) string {
	g := p.DesignGate()
	var b strings.Builder
	b.WriteString("# Quality gate desain UI\n\nDibuat ulang setiap ekspor dari rancangan dan use case; tidak ada angka yang ditaksir.\n\n")
	verdict := map[string]string{core.VerdictReady: "**Siap.**", core.VerdictReview: "**Perlu ditinjau.**"}[g.Verdict]
	if verdict == "" {
		verdict = "**Belum siap.**"
	}
	fmt.Fprintf(&b, "%s %s Error %d, peringatan %d, catatan %d.\n\n", verdict, g.Summary, g.Errors, g.Warnings, g.Notes)
	b.WriteString("| Pemeriksaan | Status | Ringkasan |\n|---|---|---|\n")
	for _, c := range g.Checks {
		fmt.Fprintf(&b, "| %s | %s | %s |\n", c.Name, gateStatus[c.Status], c.Detail)
	}
	for _, c := range g.Checks {
		if len(c.Items) == 0 {
			continue
		}
		fmt.Fprintf(&b, "\n### %s\n\n", c.Name)
		for _, it := range c.Items {
			fmt.Fprintf(&b, "- %s\n", it)
		}
	}
	if len(g.Requirements) > 0 {
		b.WriteString("\n## Requirement → layar\n\n| Requirement | Prioritas | Status | Use case | Layar | Aksi utama |\n|---|---|---|---|---|---|\n")
		for _, r := range g.Requirements {
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s |\n", r.ID, orDash(r.Priority), traceStatus[r.Status],
				orDash(strings.Join(r.Flows, ", ")), orDash(strings.Join(r.Pages, ", ")), orDash(strings.Join(r.Actions, "; ")))
		}
	}
	if len(g.Flows) > 0 {
		b.WriteString("\n## Use case → layar\n\n| Use case | Halaman | Langkah tergambar | Skenario terlihat | Pemetaan |\n|---|---|---|---|---|\n")
		for _, f := range g.Flows {
			mapping := "dicocokkan dari kata"
			switch {
			case f.Inferred:
				mapping = "halaman ditebak"
			case f.Declared:
				mapping = "ditetapkan desainer"
			}
			fmt.Fprintf(&b, "| %s %s | %s | %d/%d | %d/%d | %s |\n", f.ID, f.Name, orDash(strings.Join(f.Pages, " → ")),
				f.Placed, f.Steps, f.Shown, f.Branches, mapping)
		}
	}
	bullets(&b, "\n**Asumsi desain yang perlu dikonfirmasi**", g.Assumptions)
	bullets(&b, "**Belum diketahui**", g.Unknowns)
	return b.String()
}

var benchKind = map[string]string{
	core.BenchStructural: "struktural", core.BenchSemantic: "semantik", core.BenchVisual: "visual",
	core.BenchInteraction: "interaksi", core.BenchBusiness: "bisnis",
}

// designBenchmarkMarkdown is the per-dimension quality report. Dimensions
// stay separate so a passing visual check cannot hide a blocking failure.
func designBenchmarkMarkdown(p *core.Project) string {
	bch := p.DesignBenchmark()
	var b strings.Builder
	b.WriteString("# Benchmark kualitas desain\n\nSetiap dimensi diukur sendiri. Tidak ada skor gabungan. Dimensi yang memblokir (gagal) membuat desain belum siap, apa pun hasil dimensi lain.\n\n")
	fmt.Fprintf(&b, "%s\n\n", bch.Summary)
	if len(bch.Blocking) > 0 {
		b.WriteString("**Memblokir:** " + strings.Join(bch.Blocking, "; ") + "\n\n")
	}
	b.WriteString("| Dimensi | Jenis | Status | Terukur | Ringkasan |\n|---|---|---|---|---|\n")
	for _, d := range bch.Dimensions {
		cov := "—"
		if d.Total > 0 {
			cov = fmt.Sprintf("%d/%d", d.Covered, d.Total)
		}
		block := ""
		if d.Blocking {
			block = " (memblokir bila gagal)"
		}
		fmt.Fprintf(&b, "| %s%s | %s | %s | %s | %s |\n", d.Name, block, benchKind[d.Kind], gateStatus[d.Status], cov, d.Detail)
	}
	for _, d := range bch.Dimensions {
		if len(d.Items) == 0 {
			continue
		}
		fmt.Fprintf(&b, "\n### %s\n\n", d.Name)
		for _, it := range d.Items {
			fmt.Fprintf(&b, "- %s\n", it)
		}
	}
	return b.String()
}
