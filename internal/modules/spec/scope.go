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
		if f.Trigger != "" {
			fmt.Fprintf(&b, "**Pemicu:** %s\n\n", f.Trigger)
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
		list("Langkah paralel", parallelLines(f), false)
		list("Postconditions (berhasil bila)", f.Post, false)
		list("Alternatif", f.Alternatives, false)
		list("Kegagalan", f.Failures, false)
		list("Aturan bisnis", f.Rules, false)
		list("Otorisasi", f.Permissions, false)
		list("Efek ke data", f.Data, false)
		list("Integrasi", f.Integrations, false)
	}
	b.WriteString(lifecyclesMarkdown(p))
	return b.String()
}

// parallelLines are the step groups of a use case done at the same time.
func parallelLines(f core.Flow) []string {
	var out []string
	for _, g := range f.Parallel {
		steps := make([]string, len(g.Steps))
		for i, n := range g.Steps {
			steps[i] = fmt.Sprint(n)
		}
		line := fmt.Sprintf("Langkah %s berjalan bersamaan (%s)", strings.Join(steps, ", "), core.JoinText(g.Join))
		if g.Then > 0 {
			line += fmt.Sprintf(", lalu langkah %d", g.Then)
		}
		if g.Timeout != "" {
			line += "; batas waktu " + g.Timeout
			if g.OnTimeout != "" {
				line += ": " + g.OnTimeout
			}
		}
		out = append(out, line)
	}
	return out
}

// lifecyclesMarkdown is the status model of each business document: its
// statuses, and the transitions with the action, actor, guard and the use
// case step that performs them.
func lifecyclesMarkdown(p *core.Project) string {
	if len(p.Lifecycles) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("## Siklus status dokumen\n\nStatus yang dimiliki setiap dokumen bisnis dan perpindahannya. Layar hanya menampilkan status ini, dan aksi sebuah halaman adalah transisi yang tersedia pada status dokumen saat itu.\n\n")
	for _, lc := range p.Lifecycles {
		fmt.Fprintf(&b, "### %s\n\n| Status | Label | Jenis | Terkunci |\n|---|---|---|---|\n", lc.Entity)
		for _, st := range lc.States {
			kind := map[string]string{core.StateInitial: "awal", core.StateFinal: "akhir"}[st.Kind]
			locked := ""
			if st.Locked {
				locked = "ya"
			}
			fmt.Fprintf(&b, "| `%s` | %s | %s | %s |\n", st.ID, cell(st.Label), orDash(kind), orDash(locked))
		}
		b.WriteString("\n| Dari | Ke | Aksi | Jenis | Aktor | Use case | Syarat |\n|---|---|---|---|---|---|---|\n")
		for _, t := range lc.Transitions {
			trace := t.Flow
			if t.Step > 0 {
				trace = fmt.Sprintf("%s langkah %d", t.Flow, t.Step)
			}
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s | %s |\n", strings.Join(t.From, ", "), t.To, cell(t.Action),
				orDash(t.Kind), cell(t.Actor), orDash(trace), orDash(cell(t.Guard)))
		}
		b.WriteString("\n```mermaid\nstateDiagram-v2\n")
		if s := lc.Initial(); s != "" {
			fmt.Fprintf(&b, "  [*] --> %s\n", s)
		}
		for _, t := range lc.Transitions {
			for _, f := range t.From {
				fmt.Fprintf(&b, "  %s --> %s: %s\n", f, t.To, strings.ReplaceAll(t.Action, ":", " "))
			}
		}
		for _, st := range lc.States {
			if st.Kind == core.StateFinal {
				fmt.Fprintf(&b, "  %s --> [*]\n", st.ID)
			}
		}
		b.WriteString("```\n\n")
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
		} else {
			b.WriteString("Keputusan di bawah mengikat. Bila artefak, use case, aturan desain, layout, atau command bertentangan, ikuti keputusan ini — jangan mengikuti dokumen yang masih memakai keputusan lama.\n\n")
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
	screens := map[string]core.ReqTrace{}
	if p.UI != nil && p.UI.Layout != nil && len(p.UI.Layout.Pages) > 0 {
		for _, t := range p.DesignGate().Requirements {
			screens[t.ID] = t
		}
	}
	if len(screens) > 0 {
		b.WriteString("# Traceability\n\nRequirement → fitur → use case → layar dan aksi utama → acceptance criteria → command. Dibuat ulang setiap ekspor.\n\n")
		b.WriteString("| Requirement | Prioritas | Fitur | Use case | Layar | Aksi utama | Kriteria | Command |\n|---|---|---|---|---|---|---|---|\n")
	} else {
		b.WriteString("# Traceability\n\nRequirement → fitur → use case → acceptance criteria → command. Dibuat ulang setiap ekspor.\n\n")
		b.WriteString("| Requirement | Prioritas | Fitur | Use case | Kriteria | Command |\n|---|---|---|---|---|---|\n")
	}
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
		if len(screens) > 0 {
			t := screens[r.ID]
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s | %s | %s |\n", r.ID, r.Priority, orDash(strings.Join(feats, ", ")),
				orDash(strings.Join(flows, ", ")), orDash(strings.Join(t.Pages, ", ")), orDash(strings.Join(t.Actions, "; ")),
				orDash(strings.Join(crit, ", ")), orDash(strings.Join(cmds, ", ")))
			continue
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s |\n", r.ID, r.Priority, orDash(strings.Join(feats, ", ")),
			orDash(strings.Join(flows, ", ")), orDash(strings.Join(crit, ", ")), orDash(strings.Join(cmds, ", ")))
	}
	if len(uncovered) > 0 {
		fmt.Fprintf(&b, "\nRequirement MUST tanpa command: %s.\n", strings.Join(uncovered, ", "))
	}
	return b.String()
}
