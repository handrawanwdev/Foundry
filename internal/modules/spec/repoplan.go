package spec

import (
	"fmt"
	"slices"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/repoplan"
)

// repoPlanOf is the project's repository plan with its analysis freshly
// computed (the project is the caller's own copy), nil when there is none.
func repoPlanOf(p *core.Project) *core.RepoPlan {
	s := p.Structure
	if s == nil || s.Mode == core.StructureExisting || s.Repo == nil || len(s.Repo.Nodes) == 0 {
		return nil
	}
	repoplan.Refresh(p, s.Repo)
	return s.Repo
}

// repoTree draws the plan's folders and files as a tree.
func repoTree(plan *core.RepoPlan) string {
	type node struct {
		name string
		dir  bool
		kids []*node
	}
	root := &node{}
	at := map[string]*node{"": root}
	for _, n := range plan.Nodes { // Normalize keeps parents before children
		parent := at[n.Parent()]
		if parent == nil {
			parent = root
		}
		k := &node{name: n.BaseName(), dir: n.IsDir()}
		parent.kids = append(parent.kids, k)
		at[n.Path] = k
	}
	var b strings.Builder
	var walk func(n *node, prefix string)
	walk = func(n *node, prefix string) {
		for i, k := range n.kids {
			branch, next := "├── ", "│   "
			if i == len(n.kids)-1 {
				branch, next = "└── ", "    "
			}
			name := k.name
			if k.dir {
				name += "/"
			}
			b.WriteString(prefix + branch + name + "\n")
			walk(k, prefix+next)
		}
	}
	walk(root, "")
	return b.String()
}

func yesNo(v bool) string {
	if v {
		return "ya"
	}
	return "tidak"
}

func mermaidID(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return "u_" + b.String()
}

// repoMarkdown is the repository structure artifact: the whole plan as
// documentation an external coding agent can follow.
func repoMarkdown(p *core.Project) string {
	plan := repoPlanOf(p)
	if plan == nil {
		return ""
	}
	a := plan.Analysis
	var b strings.Builder
	state := string(plan.State())
	fmt.Fprintf(&b, "Status: **%s** · v%d · %d folder, %d file · skor kualitas %d/100", state, plan.Version(), a.Dirs, a.Files, plan.Evaluation.Score)
	if plan.Stale {
		fmt.Fprintf(&b, " · **KEDALUWARSA** (sumber berubah: %s; bagian terdampak: %s)", strings.Join(a.StaleAreas, ", "), strings.Join(a.Affected, ", "))
	}
	b.WriteString("\n\n")
	b.WriteString("Rencana ini adalah sumber kebenaran untuk letak kode. Coding agent mengikuti folder, kepemilikan, dan aturan dependency di bawah; " +
		"jika sesuatu belum tercantum, tambahkan di folder milik modul yang bertanggung jawab dan jangan membuat tata letak lain.\n\n")

	fmt.Fprintf(&b, "## Gambaran\n\n**Pendekatan:** %s\n\n**Topologi repository:** %s\n\n%s\n\n", plan.Approach, plan.Topology, plan.Summary)
	if plan.Rationale != "" {
		fmt.Fprintf(&b, "**Alasan:** %s\n\n", plan.Rationale)
	}

	b.WriteString("## Pohon folder\n\n```text\n" + repoTree(plan) + "```\n\n")

	b.WriteString("## Spesifikasi folder dan file\n\n")
	for _, n := range plan.Nodes {
		kind := "file"
		if n.IsDir() {
			kind = "folder"
		}
		fmt.Fprintf(&b, "### `%s`\n\n", n.Path)
		fmt.Fprintf(&b, "- Jenis: %s · owner: `%s`", kind, n.Owner)
		if n.Role != "" {
			fmt.Fprintf(&b, " · peran: %s", n.Role)
		}
		if n.Part != "" {
			fmt.Fprintf(&b, " · bagian struktur: `%s`", n.Part)
		}
		b.WriteString("\n")
		fmt.Fprintf(&b, "- Tanggung jawab: %s\n- Alasan ada: %s\n", orDash(n.Responsibility), orDash(n.Reason))
		if len(n.Allowed) > 0 {
			fmt.Fprintf(&b, "- Boleh ditempatkan: %s\n", strings.Join(n.Allowed, "; "))
		}
		if len(n.Forbidden) > 0 {
			fmt.Fprintf(&b, "- Tidak boleh ditempatkan: %s\n", strings.Join(n.Forbidden, "; "))
		}
		if n.Relations != "" {
			fmt.Fprintf(&b, "- Hubungan: %s\n", n.Relations)
		}
		if n.Boundary != "" {
			fmt.Fprintf(&b, "- Batas kepemilikan: %s\n", n.Boundary)
		}
		if len(n.SharedBy) > 0 {
			fmt.Fprintf(&b, "- Dipakai bersama oleh: %s\n", strings.Join(n.SharedBy, ", "))
		}
		if len(n.DependsOn) > 0 {
			fmt.Fprintf(&b, "- Boleh bergantung pada: %s\n", strings.Join(n.DependsOn, ", "))
		}
		b.WriteString("\n")
	}

	b.WriteString("## Pemetaan modul\n\n| Modul | Lokasi | Frontend | Backend | Catatan |\n|---|---|---|---|---|\n")
	for _, m := range plan.Modules {
		var locs []string
		for _, l := range m.Locations {
			s := "`" + l.Path + "`"
			if l.Role != "" {
				s += " (" + l.Role + ")"
			}
			locs = append(locs, s)
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", cell(m.Module), orDash(strings.Join(locs, "<br>")), yesNo(m.Frontend), yesNo(m.Backend), cell(orDash(m.Note)))
	}
	b.WriteString("\n")
	var none, shared []string
	for _, m := range plan.Modules {
		if !m.Frontend && !m.Backend {
			none = append(none, m.Module)
		}
	}
	for _, n := range plan.Nodes {
		if n.IsDir() && (n.Owner == core.OwnerShared || len(n.SharedBy) > 1) {
			shared = append(shared, fmt.Sprintf("`%s` — %s", n.Path, orDash(n.Reason)))
		}
	}
	if len(none) > 0 {
		fmt.Fprintf(&b, "Modul tanpa implementasi frontend maupun backend: %s.\n\n", strings.Join(none, ", "))
	}
	if len(shared) > 0 {
		b.WriteString("Folder bersama (dipakai beberapa modul):\n")
		for _, s := range shared {
			b.WriteString("- " + s + "\n")
		}
		b.WriteString("\n")
	}

	writeRepoDependency(&b, plan)

	if len(plan.Rules) > 0 {
		b.WriteString("## Aturan arsitektur\n\n")
		for i, r := range plan.Rules {
			fmt.Fprintf(&b, "%d. %s\n", i+1, r)
		}
		b.WriteString("\n")
	}

	writeRepoEvaluation(&b, plan)

	if len(plan.Decisions) > 0 {
		b.WriteString("## Keputusan desain\n\n")
		for _, d := range plan.Decisions {
			fmt.Fprintf(&b, "### %s\n\nPilihan: %s\n\nAlasan: %s\n\n", d.Title, d.Choice, d.Reason)
			if len(d.Alternatives) > 0 {
				fmt.Fprintf(&b, "Alternatif yang ditolak: %s\n\n", strings.Join(d.Alternatives, "; "))
			}
			if d.Tradeoff != "" {
				fmt.Fprintf(&b, "Trade-off: %s\n\n", d.Tradeoff)
			}
		}
	}

	if len(plan.History) > 0 {
		b.WriteString("## Riwayat revisi\n\n| Versi | Waktu | Sumber | Status | Perubahan |\n|---|---|---|---|---|\n")
		for _, v := range plan.History {
			what := strings.Join(v.Changes, "; ")
			if v.Request != "" {
				what = "“" + v.Request + "” — " + what
			}
			if v.Reason != "" {
				what += " (alasan: " + v.Reason + ")"
			}
			fmt.Fprintf(&b, "| v%d | %s | %s | %s | %s |\n", v.Version, v.At.Format("2006-01-02 15:04"), v.Source, v.Status, cell(what))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func writeRepoDependency(b *strings.Builder, plan *core.RepoPlan) {
	d := plan.Dependency
	b.WriteString("## Arsitektur dependency\n\n")
	fmt.Fprintf(b, "Arah dependency: %s\n\n", orDash(d.Direction))
	if len(plan.Analysis.Edges) > 0 {
		b.WriteString("```mermaid\nflowchart LR\n")
		for _, u := range plan.Analysis.Units {
			if u.Nodes > 0 || u.FanIn+u.FanOut > 0 {
				fmt.Fprintf(b, "  %s[\"%s\"]\n", mermaidID(u.Name), u.Name)
			}
		}
		for _, e := range plan.Analysis.Edges {
			arrow := "-->"
			switch {
			case e.Kind == "forbidden":
				arrow = "-. dilarang .-x"
			case e.Mode == core.DepEvent:
				arrow = "-. event .->"
			case e.Kind == "allowed":
				arrow = "-- boleh -->"
			}
			fmt.Fprintf(b, "  %s %s %s\n", mermaidID(e.From), arrow, mermaidID(e.To))
		}
		b.WriteString("```\n\n")
	}
	rules := func(title string, rs []core.RepoDepRule) {
		if len(rs) == 0 {
			return
		}
		fmt.Fprintf(b, "**%s**\n\n| Dari | Ke | Lewat | Mode | Alasan |\n|---|---|---|---|---|\n", title)
		for _, r := range rs {
			fmt.Fprintf(b, "| `%s` | `%s` | %s | %s | %s |\n", r.From, r.To, cell(orDash(r.Via)), orDash(r.Mode), cell(orDash(r.Reason)))
		}
		b.WriteString("\n")
	}
	rules("Dependency yang diizinkan", d.Allowed)
	rules("Dependency yang dilarang", d.Forbidden)
	rules("Komunikasi antarmodul", d.Communication)
	if len(d.Interfaces) > 0 {
		b.WriteString("**Public interface** (satu-satunya pintu masuk ke modul)\n\n")
		for _, i := range d.Interfaces {
			fmt.Fprintf(b, "- %s: `%s` — %s\n", i.Module, i.Path, orDash(i.Reason))
		}
		b.WriteString("\n")
	}
	if len(d.SharedRules) > 0 {
		b.WriteString("**Aturan shared code**\n\n")
		for _, r := range d.SharedRules {
			fmt.Fprintf(b, "- %s\n", r)
		}
		b.WriteString("\n")
	}
	if d.CycleGuard != "" {
		fmt.Fprintf(b, "**Pencegahan dependensi melingkar:** %s\n\n", d.CycleGuard)
	}
}

func writeRepoEvaluation(b *strings.Builder, plan *core.RepoPlan) {
	e := plan.Evaluation
	b.WriteString("## Evaluasi kualitas struktur\n\n")
	if e.Outdated {
		b.WriteString("_Struktur diubah manual setelah evaluasi AI terakhir; evaluasi belum diperbarui._\n\n")
	}
	if e.Summary != "" {
		b.WriteString(e.Summary + "\n\n")
	}
	b.WriteString("| Kriteria | Skor | Alasan |\n|---|---|---|\n")
	for _, c := range e.Criteria {
		fmt.Fprintf(b, "| %s | %d | %s |\n", c.Name, c.Score, cell(c.Reason))
	}
	b.WriteString("\n(1 = buruk, 5 = sangat baik; untuk complexity 1 = sederhana, 5 = kompleks.)\n\n")
	if len(plan.Analysis.Units) > 0 {
		b.WriteString("Ukuran dependency per unit:\n\n| Unit | Node | Fan-out | Fan-in |\n|---|---|---|---|\n")
		for _, u := range plan.Analysis.Units {
			fmt.Fprintf(b, "| %s | %d | %d | %d |\n", u.Name, u.Nodes, u.FanOut, u.FanIn)
		}
		b.WriteString("\n")
	}
	all := slices.Concat(e.Findings, plan.Analysis.Findings)
	if len(all) == 0 {
		b.WriteString("Temuan: tidak ada kelemahan struktur yang terdeteksi.\n\n")
	} else {
		b.WriteString("Temuan:\n\n| Kategori | Tingkat | Lokasi | Temuan | Perbaikan | Sumber |\n|---|---|---|---|---|---|\n")
		for _, f := range all {
			fmt.Fprintf(b, "| %s | %s | %s | %s | %s | %s |\n", f.Category, f.Severity, cell(orDash(f.Path)), cell(f.Message), cell(orDash(f.Resolution)), f.Source)
		}
		b.WriteString("\n")
	}
	b.WriteString("### Konsistensi\n\n| Area | Status | Catatan | Sumber |\n|---|---|---|---|\n")
	for _, c := range plan.Analysis.Checks {
		fmt.Fprintf(b, "| %s | %s | %s | %s |\n", c.Area, c.Status, cell(orDash(c.Note)), c.Source)
	}
	b.WriteString("\n")
	if len(plan.Analysis.Issues) > 0 {
		b.WriteString("### Temuan validasi\n\n")
		for _, i := range plan.Analysis.Issues {
			fmt.Fprintf(b, "- %s (%s): %s\n", i.Level, i.Area, i.Message)
		}
		b.WriteString("\n")
	}
}
