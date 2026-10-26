package spec

import (
	"fmt"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
)

// visualMarkdown is the measured UI quality report written by the visual
// review. It states what passed, what failed and what could not be tested;
// absence of evidence is never reported as a pass.
func visualMarkdown(p *core.Project) string {
	if p == nil || p.Visual == nil || p.UI == nil || p.UI.Layout == nil {
		return ""
	}
	s := p.Visual.Summary(p.UI.Layout.Pages)
	var b strings.Builder
	b.WriteString("# Evaluasi Visual\n\n")
	fmt.Fprintf(&b, "Halaman: %d lulus, %d sebagian, %d gagal, %d belum diuji. Temuan terbuka: %d (%d berat).\n\n",
		s.Pass, s.Partial, s.Fail, s.Untested, s.OpenAll, s.OpenHigh)
	if !s.CheckedAt.IsZero() {
		fmt.Fprintf(&b, "Pemeriksaan terakhir: %s\n\n", s.CheckedAt.UTC().Format("2006-01-02T15:04:05Z"))
	}
	if len(s.NotChecked) > 0 {
		fmt.Fprintf(&b, "Belum diuji: %s. Halaman tanpa screenshot tidak dianggap lulus.\n\n", strings.Join(s.NotChecked, ", "))
	}
	b.WriteString("## Status halaman\n\n| Halaman | Status | Sebelumnya | Temuan | Kriteria belum diuji |\n|---|---|---|---|---|\n")
	for _, pg := range p.UI.Layout.Pages {
		a, ok := p.Visual.Pages[pg.ID]
		if !ok {
			fmt.Fprintf(&b, "| `%s` | belum diuji | - | - | semua |\n", pg.ID)
			continue
		}
		var untested []string
		for _, c := range a.Criteria {
			if c.Status == core.VisualUntested {
				untested = append(untested, c.ID)
			}
		}
		fmt.Fprintf(&b, "| `%s` | %s | %s | %d | %s |\n", pg.ID, core.StatusLabel(a.Status),
			dash(a.Previous), len(a.Findings), dash(strings.Join(untested, ", ")))
	}
	b.WriteString("\n")
	for _, pg := range p.UI.Layout.Pages {
		a, ok := p.Visual.Pages[pg.ID]
		if !ok {
			continue
		}
		fmt.Fprintf(&b, "## `%s` %s\n\n", pg.ID, pg.Name)
		if a.URL != "" {
			fmt.Fprintf(&b, "URL: %s\n\n", a.URL)
		}
		if len(a.Screenshots) > 0 {
			fmt.Fprintf(&b, "Bukti: %s\n\n", codeList(a.Screenshots))
		}
		if len(a.Resolved) > 0 {
			b.WriteString("**Temuan yang sudah hilang sejak pemeriksaan sebelumnya**\n\n")
			for _, r := range a.Resolved {
				fmt.Fprintf(&b, "- %s\n", r)
			}
			b.WriteString("\n")
		}
		b.WriteString("| Kriteria | Status | Keterangan |\n|---|---|---|\n")
		for _, c := range a.Criteria {
			fmt.Fprintf(&b, "| %s | %s | %s |\n", c.Title, core.StatusLabel(c.Status), cell(c.Detail))
		}
		b.WriteString("\n")
		if len(a.Findings) > 0 {
			b.WriteString("**Temuan**\n\n| Keparahan | Kriteria | Bukti | Masalah | Perbaikan |\n|---|---|---|---|---|\n")
			for _, f := range a.Findings {
				fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", f.Severity, f.Criterion, cell(f.Evidence), cell(f.Message), cell(f.Fix))
			}
			b.WriteString("\n")
		}
	}
	b.WriteString("## Batasan\n\n")
	b.WriteString("Pemeriksaan ini memakai snapshot DOM terukur dan screenshot yang dikirim oleh alat browser atau agen. ")
	b.WriteString("Foundry tidak membuka browser, tidak menghasilkan screenshot, dan tidak menilai keindahan visual. ")
	b.WriteString("Kriteria tanpa bukti dilaporkan belum diuji, bukan lulus.\n")
	return b.String()
}
