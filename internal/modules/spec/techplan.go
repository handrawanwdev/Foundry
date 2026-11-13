package spec

import (
	"fmt"
	"slices"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/stack"
	"github.com/handrawanwdev/foundry/internal/modules/techplan"
)

// techPlanOf is the project's technology plan with its analysis freshly
// computed (the project is the caller's own copy), nil when there is none.
func techPlanOf(p *core.Project) *core.TechPlan {
	if p.Stack == nil || p.Stack.Plan == nil || len(p.Stack.Plan.Items) == 0 {
		return nil
	}
	techplan.Refresh(p, p.Stack.Plan)
	return p.Stack.Plan
}

var groupTitle = map[string]string{
	core.TechPlanStack: "Application Stack", core.TechPlanData: "Data Architecture", core.TechPlanAppInfra: "Application Infrastructure",
	core.TechPlanArchitecture: "Architecture & Repository", core.TechPlanOperations: "Infrastructure & Operations", core.TechPlanStandards: "Engineering Standards",
}

var criterionTitle = map[string]string{
	"requirement_coverage": "Requirement coverage", "compatibility": "Compatibility", "maintainability": "Maintainability", "performance": "Performance",
	"scalability": "Scalability", "security": "Security", "reliability": "Reliability", "developer_experience": "Developer experience",
	"operational_complexity": "Operational complexity (rendah lebih baik)", "total_cost": "Total cost of ownership (rendah lebih baik)", "sustainability": "Long-term sustainability",
}

func list(b *strings.Builder, label string, items []string) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(b, "- %s:\n", label)
	for _, i := range items {
		fmt.Fprintf(b, "  - %s\n", i)
	}
}

// techPlanMarkdown is the technology plan artifact: the whole plan as a
// technical specification an external coding agent can follow.
func techPlanMarkdown(p *core.Project) string {
	plan := techPlanOf(p)
	if plan == nil {
		return ""
	}
	a := plan.Analysis
	st := p.Stack
	var b strings.Builder
	fmt.Fprintf(&b, "Status: **%s** · v%d · %d keputusan · skor evaluasi %d/100", st.State(), plan.Version(), a.Items, plan.Evaluation.Score)
	if plan.Stale {
		fmt.Fprintf(&b, " · **KEDALUWARSA** (sumber berubah: %s; bagian terdampak: %s)", strings.Join(a.StaleAreas, ", "), strings.Join(a.Affected, ", "))
	}
	if len(a.Diverged) > 0 {
		fmt.Fprintf(&b, " · **dipilih ulang user**: %s; evaluasi belum mencerminkannya", strings.Join(a.Diverged, ", "))
	}
	b.WriteString("\n\n")
	b.WriteString("Rencana ini adalah sumber kebenaran untuk teknologi. Coding agent memakai teknologi di bawah dan tidak menggantinya; " +
		"versi yang ditandai belum terverifikasi harus diperiksa sebelum dipakai.\n\n")

	fmt.Fprintf(&b, "## Gambaran\n\n**Pendekatan:** %s\n\n%s\n\n", plan.Approach, plan.Summary)
	if plan.Rationale != "" {
		fmt.Fprintf(&b, "**Alasan:** %s\n\n", plan.Rationale)
	}

	c := plan.Context
	b.WriteString("## Konteks teknis\n\n")
	fmt.Fprintf(&b, "%s\n\n", orDash(c.Summary))
	for _, cat := range core.TechFactCategories {
		var xs []string
		for _, f := range c.Facts {
			if f.Category == cat {
				s := f.Statement
				if f.Source != "" {
					s += " _(" + f.Source + ")_"
				}
				xs = append(xs, s)
			}
		}
		list(&b, cat, xs)
	}
	if len(c.Unknowns) > 0 || len(c.Assumptions) > 0 {
		b.WriteString("\n")
		list(&b, "Belum jelas (tidak diasumsikan sebagai fakta)", c.Unknowns)
		list(&b, "Asumsi", c.Assumptions)
	}
	b.WriteString("\n")

	for _, g := range core.TechPlanGroups {
		var items []core.TechItem
		for _, it := range plan.Items {
			if it.Group == g {
				items = append(items, it)
			}
		}
		if len(items) == 0 {
			continue
		}
		fmt.Fprintf(&b, "## %s\n\n", groupTitle[g])
		for _, it := range items {
			writeTechItem(&b, p, it)
		}
	}

	if len(plan.Constraints) > 0 {
		b.WriteString("## Constraint kompatibilitas\n\n")
		for _, k := range plan.Constraints {
			fmt.Fprintf(&b, "- [%s] %s", strings.Join(k.Areas, ", "), k.Rule)
			if k.Reason != "" {
				fmt.Fprintf(&b, " — %s", k.Reason)
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("## Kompatibilitas teknologi\n\n| Area | Status | Antar | Catatan | Dasar |\n|---|---|---|---|---|\n")
	for _, x := range plan.Compatibility {
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", x.Area, x.Status, strings.Join(x.Between, ", "), cell(x.Note), cell(x.Basis))
	}
	for _, x := range a.Compat {
		fmt.Fprintf(&b, "| %s | %s (dihitung Foundry) | %s | %s | — |\n", x.Area, x.Status, strings.Join(x.Between, ", "), cell(x.Note))
	}
	b.WriteString("\n")

	ev := plan.Evaluation
	b.WriteString("## Evaluasi teknologi\n\n")
	if ev.Summary != "" {
		fmt.Fprintf(&b, "%s\n\n", ev.Summary)
	}
	b.WriteString("| Kriteria | Skor | Alasan | Evidence |\n|---|---|---|---|\n")
	for _, k := range ev.Criteria {
		fmt.Fprintf(&b, "| %s | %d/5 | %s | %s |\n", criterionTitle[k.Name], k.Score, cell(k.Reason), cell(strings.Join(k.Evidence, ", ")))
	}
	b.WriteString("\n")
	fs := slices.Concat(ev.Findings, a.Findings)
	if len(fs) > 0 {
		b.WriteString("### Temuan\n\n")
		for _, f := range fs {
			fmt.Fprintf(&b, "- **%s** (%s, %s)", f.Category, f.Severity, orDash(f.Source))
			if f.Area != "" {
				fmt.Fprintf(&b, " `%s`", f.Area)
			}
			fmt.Fprintf(&b, ": %s", f.Message)
			if f.Resolution != "" {
				fmt.Fprintf(&b, " → %s", f.Resolution)
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("## Konsistensi dengan artefak lain\n\n")
	checks := slices.Concat(plan.Consistency, a.Checks)
	for _, k := range checks {
		fmt.Fprintf(&b, "- `%s`: %s (%s)", k.Area, k.Status, orDash(k.Source))
		if k.Note != "" {
			fmt.Fprintf(&b, " — %s", k.Note)
		}
		b.WriteString("\n")
	}
	if len(a.Issues) > 0 {
		b.WriteString("\n### Masalah yang harus diselesaikan\n\n")
		for _, i := range a.Issues {
			fmt.Fprintf(&b, "- %s", i.Level)
			if i.Area != "" {
				fmt.Fprintf(&b, " (%s)", i.Area)
			}
			fmt.Fprintf(&b, ": %s\n", i.Message)
		}
	}
	b.WriteString("\n")

	if len(plan.History) > 0 {
		b.WriteString("## Riwayat revisi\n\n")
		for i := len(plan.History) - 1; i >= 0; i-- {
			v := plan.History[i]
			fmt.Fprintf(&b, "- **v%d** · %s · %s · %s", v.Version, v.At.Format("2006-01-02 15:04"), v.Source, v.Status)
			if v.Request != "" {
				fmt.Fprintf(&b, " · “%s”", v.Request)
			}
			b.WriteString("\n")
			for _, c := range v.Changes {
				fmt.Fprintf(&b, "  - %s\n", c)
			}
		}
	}
	return b.String()
}

func writeTechItem(b *strings.Builder, p *core.Project, it core.TechItem) {
	fmt.Fprintf(b, "### %s — %s\n\n", stack.AreaName(it.Area), it.Technology)
	if it.None() {
		fmt.Fprintf(b, "Tidak memakai teknologi untuk area ini. %s\n\n", it.Reasoning)
		return
	}
	var attrs []string
	if it.Version != "" {
		v := "versi " + it.Version
		if !it.VersionVerified {
			v += " (belum terverifikasi)"
		}
		attrs = append(attrs, v)
	}
	if it.Language != "" {
		attrs = append(attrs, "bahasa "+it.Language)
	}
	if it.Runtime != "" {
		attrs = append(attrs, "runtime "+it.Runtime)
	}
	attrs = append(attrs, "driver: "+it.Driver, "kepercayaan: "+it.Confidence)
	fmt.Fprintf(b, "%s\n\n", strings.Join(attrs, " · "))
	if it.UserChosen != "" {
		fmt.Fprintf(b, "**Dipilih user** (%s); rencana menjelaskan dan menilai pilihan itu.\n\n", it.UserChosen)
	}
	if d := p.Stack.Decision(it.Area); d != nil && d.Override != nil && !strings.EqualFold(d.Choice(), it.Technology) {
		fmt.Fprintf(b, "**Pilihan user sekarang: %s** (rencana AI: %s) — evaluasi belum mencerminkannya.\n\n", d.Choice(), it.Technology)
	}
	fmt.Fprintf(b, "- Tujuan: %s\n- Alasan: %s\n", orDash(it.Purpose), orDash(it.Reasoning))
	list(b, "Memenuhi", it.Fulfills)
	list(b, "Kelebihan", it.Pros)
	list(b, "Kekurangan", it.Cons)
	list(b, "Trade-off", it.Tradeoffs)
	if len(it.DependsOn) > 0 {
		fmt.Fprintf(b, "- Terintegrasi dengan: %s\n", strings.Join(it.DependsOn, ", "))
	}
	if it.Integration != "" {
		fmt.Fprintf(b, "- Integrasi: %s\n", it.Integration)
	}
	if it.Operational != "" {
		fmt.Fprintf(b, "- Konsekuensi operasional: %s\n", it.Operational)
	}
	list(b, "Asumsi", it.Assumptions)
	if len(it.Alternatives) > 0 {
		b.WriteString("- Alternatif:\n")
		for _, a := range it.Alternatives {
			fmt.Fprintf(b, "  - **%s** — %s Beda: %s", a.Technology, a.Why, a.Difference)
			if len(a.Pros) > 0 {
				fmt.Fprintf(b, " Kelebihan: %s.", strings.Join(a.Pros, "; "))
			}
			if len(a.Cons) > 0 {
				fmt.Fprintf(b, " Kekurangan: %s.", strings.Join(a.Cons, "; "))
			}
			for _, x := range []struct{ label, v string }{{"arsitektur", a.ArchitectureImpact}, {"maintenance", a.MaintenanceImpact}, {"biaya operasional", a.CostImpact}} {
				if x.v != "" {
					fmt.Fprintf(b, " Dampak %s: %s.", x.label, x.v)
				}
			}
			b.WriteString("\n")
		}
	}
	b.WriteString("\n")
}
