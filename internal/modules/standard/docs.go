package standard

import (
	"fmt"
	"strings"

	"github.com/handrawanw/foundry/internal/core"
)

// DesignBrief is design-brief.md of the export. Without UI → short note.
func DesignBrief(p *core.Project) string {
	if p == nil || !p.HasUI() {
		return "# Design Brief\n\nTidak ada frontend. File ini sengaja pendek.\n"
	}
	var b strings.Builder
	b.WriteString("# Design Brief\n\n")
	b.WriteString("Ditulis sebelum layar diimplementasikan. Coding agent UI mengikuti brief ini dan token di bawah; artefak blueprint/ui-design.md memuat spesifikasi per halaman.\n\n")
	b.WriteString("## Pengguna dan tugas per layar\n\n")
	if p.Design != nil {
		for _, pg := range p.Design.Pages {
			task := pg.Purpose
			if pg.Intent != nil && pg.Intent.PrimaryTask != "" {
				task = pg.Intent.PrimaryTask
			}
			fmt.Fprintf(&b, "- **%s** (%s, %s): %s\n", pg.Name, pg.Role, pg.Route, task)
		}
		b.WriteString("\n")
	}
	b.WriteString("## Riset referensi\n\n")
	if p.Reference != nil && p.Reference.URL != "" {
		fmt.Fprintf(&b, "Referensi user: %s. Hanya Theme DNA yang dipakai; HTML/CSS/konten tidak disalin.\n\n", p.Reference.URL)
	} else {
		b.WriteString("Tidak ada URL referensi. Arah visual mengikuti theme dan domain product, bukan template generik (gradient ungu, hero klise, kartu seragam, emoji sebagai ikon, lorem ipsum).\n\n")
	}
	if p.Design != nil && p.Design.System != nil && p.Design.System.Direction != "" {
		fmt.Fprintf(&b, "## Prinsip desain\n\n%s\n\n", p.Design.System.Direction)
	}
	if p.Design != nil && p.Design.System != nil {
		s := p.Design.System
		b.WriteString("## Design tokens\n\nSemua nilai visual berasal dari token ini. Dilarang warna, ukuran, atau spasi hardcode di komponen.\n\n")
		writeTokens := func(title string, xs []core.Token) {
			if len(xs) == 0 {
				return
			}
			fmt.Fprintf(&b, "### %s\n\n", title)
			for _, t := range xs {
				fmt.Fprintf(&b, "- `%s` = %s", t.Name, t.Value)
				if t.Use != "" {
					fmt.Fprintf(&b, " — %s", t.Use)
				}
				b.WriteByte('\n')
			}
			b.WriteString("\n")
		}
		writeTokens("Warna", s.Colors)
		writeTokens("Tipografi", s.Typography)
		writeTokens("Spasi", s.Spacing)
		writeTokens("Radius", s.Radius)
		writeTokens("Bayangan", s.Shadow)
	}
	b.WriteString("## Pemeriksaan anti-slop\n\n")
	b.WriteString("Setiap layar punya state loading, kosong, error, sukses. Satu aksi utama. Konten realistis. Ikon dari satu keluarga, bukan emoji. Kontras WCAG AA. Jika layar bisa tertukar dengan template generik, kembali ke brief ini.\n")
	return b.String()
}
