package spec

import (
	"fmt"
	"slices"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
)

// useCasesMarkdown is the use-cases artifact: every flow with what a use
// case needs. Empty while no flow says more than its steps (older
// blueprints keep their flows in the requirements artifact only).
func useCasesMarkdown(p *core.Project) string {
	if !slices.ContainsFunc(p.Flows, func(f core.Flow) bool { return f.Goal != "" || len(f.Post) > 0 || len(f.Failures) > 0 }) {
		return ""
	}
	var b strings.Builder
	b.WriteString("Alur bisnis end-to-end. Aktor, tujuan, kondisi, alur utama, alternatif, kegagalan, aturan bisnis, otorisasi, serta efek ke data dan integrasi. Requirement setiap use case berasal dari fiturnya.\n\n")
	for _, f := range p.Flows {
		fmt.Fprintf(&b, "## %s %s\n\n", f.ID, f.Name)
		meta := []string{"Aktor: " + orDash(f.Role)}
		if ft := p.Feature(f.FeatureID); ft != nil {
			meta = append(meta, fmt.Sprintf("Fitur: %s %s", ft.ID, ft.Name))
			if len(ft.RequirementIDs) > 0 {
				meta = append(meta, "Requirement: "+strings.Join(ft.RequirementIDs, ", "))
			}
		}
		if len(f.PageIDs) > 0 {
			meta = append(meta, "Halaman: "+strings.Join(f.PageIDs, ", "))
		}
		b.WriteString(strings.Join(meta, " · ") + "\n\n")
		if f.Goal != "" {
			fmt.Fprintf(&b, "**Tujuan:** %s\n\n", f.Goal)
		}
		list := func(title string, items []string, numbered bool) {
			if len(items) == 0 {
				return
			}
			fmt.Fprintf(&b, "**%s**\n\n", title)
			for i, it := range items {
				if numbered {
					fmt.Fprintf(&b, "%d. %s\n", i+1, it)
				} else {
					fmt.Fprintf(&b, "- %s\n", it)
				}
			}
			b.WriteString("\n")
		}
		list("Preconditions", f.Pre, false)
		list("Alur utama", f.Steps, true)
		list("Postconditions (berhasil bila)", f.Post, false)
		list("Alternatif", f.Alternatives, false)
		list("Kegagalan", f.Failures, false)
		list("Aturan bisnis", f.Rules, false)
		list("Otorisasi", f.Permissions, false)
		list("Efek ke data", f.Data, false)
		list("Integrasi", f.Integrations, false)
	}
	return b.String()
}

// acceptanceMarkdown is the acceptance artifact: the criteria of every
// requirement and the Definition of Done. Criteria and expected evidence
// are requirements on the implementation, not results.
func acceptanceMarkdown(p *core.Project) string {
	has := len(p.Done) > 0 || slices.ContainsFunc(p.Requirements, func(r core.Requirement) bool { return len(r.Acceptance) > 0 || r.Untestable != "" })
	if !has {
		return ""
	}
	var b strings.Builder
	b.WriteString("Syarat penerimaan per requirement. Bukti yang diharapkan diserahkan coding agent saat implementasi; Foundry tidak menjalankan pengujian, jadi tidak ada kriteria yang dianggap sudah terverifikasi.\n\n")
	for _, r := range p.Requirements {
		fmt.Fprintf(&b, "## %s (%s)\n\n%s\n\n", r.ID, r.Priority, r.Text)
		if len(r.Acceptance) == 0 {
			if r.Untestable != "" {
				fmt.Fprintf(&b, "Tidak dapat diuji langsung: %s\n\n", r.Untestable)
			} else {
				b.WriteString("Belum ada kriteria.\n\n")
			}
			continue
		}
		b.WriteString("| ID | Jenis | Kriteria | Bukti yang diharapkan |\n|---|---|---|---|\n")
		for _, c := range r.Acceptance {
			fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", c.ID, c.Kind, cell(c.Text), cell(orDash(c.Evidence)))
		}
		b.WriteString("\n")
	}
	if len(p.Done) > 0 {
		b.WriteString("## Definition of Done\n\n")
		for _, d := range p.Done {
			fmt.Fprintf(&b, "- [ ] %s\n", d)
		}
		b.WriteString("\n")
	}
	return b.String()
}

var decisionLabel = map[string]string{
	core.DecisionAssumption: "Asumsi", core.DecisionQuestion: "Pertanyaan", core.DecisionConflict: "Konflik", core.DecisionRisk: "Risiko",
}

// decisionsMarkdown is the decisions artifact: the areas left out and the
// decision register.
func decisionsMarkdown(p *core.Project) string {
	if len(p.Decisions) == 0 && len(p.Skips) == 0 {
		return ""
	}
	var b strings.Builder
	if len(p.Skips) > 0 {
		b.WriteString("## Area yang tidak dicakup\n\n")
		for _, s := range p.Skips {
			title := s.Area
			if a := core.AreaByID(s.Area); a != nil {
				title = a.Title
			}
			fmt.Fprintf(&b, "- **%s**: %s\n", title, s.Reason)
		}
		b.WriteString("\n")
	}
	for _, group := range []struct {
		title string
		open  bool
	}{{"Terbuka", true}, {"Diputuskan", false}} {
		var rows []core.Decision
		for _, d := range p.Decisions {
			if d.Open() == group.open {
				rows = append(rows, d)
			}
		}
		if len(rows) == 0 {
			continue
		}
		fmt.Fprintf(&b, "## %s\n\n", group.title)
		if group.open {
			b.WriteString("Jangan mengimplementasikan bagian yang bergantung pada keputusan kritis yang masih terbuka; tanyakan ke user.\n\n")
		}
		for _, d := range rows {
			tags := []string{decisionLabel[d.Kind]}
			if d.Critical {
				tags = append(tags, "kritis")
			}
			if d.Area != "" {
				tags = append(tags, "area "+d.Area)
			}
			fmt.Fprintf(&b, "- **%s** (%s): %s\n", d.ID, strings.Join(tags, ", "), d.Text)
			if !d.Open() {
				fmt.Fprintf(&b, "  - Keputusan: %s\n", d.Resolution)
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

// traceabilityMarkdown links every requirement to its features, use
// cases, acceptance criteria and the commands that cover it. It is part of
// the export, not an artifact: commands change it, and no command may
// depend on it.
func traceabilityMarkdown(p *core.Project) string {
	var b strings.Builder
	b.WriteString("# Traceability\n\nRequirement → fitur → use case → acceptance criteria → command. Dibuat ulang setiap ekspor.\n\n")
	b.WriteString("| Requirement | Prioritas | Fitur | Use case | Kriteria | Command |\n|---|---|---|---|---|---|\n")
	var uncovered []string
	for _, r := range p.Requirements {
		var feats, flows, crit, cmds []string
		for _, f := range p.Features {
			if slices.Contains(f.RequirementIDs, r.ID) {
				feats = append(feats, f.ID)
				for _, fl := range p.Flows {
					if fl.FeatureID == f.ID {
						flows = append(flows, fl.ID)
					}
				}
			}
		}
		for _, c := range r.Acceptance {
			crit = append(crit, c.ID)
		}
		if r.Untestable != "" && len(crit) == 0 {
			crit = append(crit, "tidak dapat diuji")
		}
		if w := p.Workflow; w != nil {
			for _, c := range w.Commands {
				if slices.Contains(c.Covers, r.ID) {
					cmds = append(cmds, fmt.Sprintf("[%s](%s)", c.ID, commandFile(c.ID)))
				}
			}
		}
		if len(cmds) == 0 && r.Priority == "MUST" {
			uncovered = append(uncovered, r.ID)
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s |\n", r.ID, r.Priority, orDash(strings.Join(feats, ", ")),
			orDash(strings.Join(flows, ", ")), orDash(strings.Join(crit, ", ")), orDash(strings.Join(cmds, ", ")))
	}
	if len(uncovered) > 0 {
		fmt.Fprintf(&b, "\nRequirement MUST tanpa command: %s.\n", strings.Join(uncovered, ", "))
	}
	return b.String()
}
