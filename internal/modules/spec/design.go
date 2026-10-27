package spec

import (
	"fmt"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
)

// designRulesMarkdown is the design-rules artifact: every field of the
// rules, so any change changes the artifact's hash.
func designRulesMarkdown(p *core.Project) string {
	if p.UI == nil || p.UI.Rules == nil {
		return ""
	}
	r := p.UI.Rules
	var b strings.Builder
	if r.Source == core.RulesExisting {
		b.WriteString("Sumber: **design system yang sudah ada di project**. Pakai token dan komponennya; jangan ganti tanpa persetujuan.\n\n")
	} else {
		b.WriteString("Sumber: dirancang untuk produk ini.\n\n")
	}
	if len(r.Sources) > 0 {
		fmt.Fprintf(&b, "Dibaca dari: %s\n\n", codeList(r.Sources))
	}
	if r.Reference != "" {
		fmt.Fprintf(&b, "Referensi desain: **%s** — aturan di bawah diturunkan darinya dan disesuaikan dengan produk ini; bukan salinan merek, logo, atau aset proprietari.\n\n", r.Reference)
	}
	if r.Direction != "" {
		fmt.Fprintf(&b, "Arah visual: %s\n\n", r.Direction)
	}
	bullets(&b, "## Prinsip desain", r.Principles)
	bullets(&b, "## Panduan UX", r.UX)
	b.WriteString("## Design tokens\n\nSemua nilai visual berasal dari token ini; petakan ke sistem styling project. Dilarang warna, ukuran, atau spasi hardcode.\n\n")
	b.WriteString("| Grup | Token | Nilai | Pemakaian |\n|---|---|---|---|\n")
	for _, g := range r.Tokens.Groups() {
		for _, t := range *g.Tokens {
			fmt.Fprintf(&b, "| %s | `%s` | %s | %s |\n", g.Name, cell(t.Name), cell(t.Value), cell(t.Use))
		}
	}
	b.WriteString("\n")
	if len(r.Components) > 0 {
		b.WriteString("## Komponen\n\n")
		for _, c := range r.Components {
			fmt.Fprintf(&b, "### %s\n\n%s\n\n", c.Name, c.Use)
			if c.Source != "" {
				fmt.Fprintf(&b, "Sudah ada: `%s` (pakai ulang, jangan buat ulang)\n\n", c.Source)
			}
			if len(c.Variants) > 0 {
				fmt.Fprintf(&b, "Varian: %s\n\n", strings.Join(c.Variants, ", "))
			}
			for _, x := range c.Rules {
				fmt.Fprintf(&b, "- %s\n", x)
			}
			if len(c.Rules) > 0 {
				b.WriteString("\n")
			}
		}
	}
	bullets(&b, "## Hierarki visual", r.Hierarchy)
	bullets(&b, "## Aksesibilitas", r.Accessibility)
	bullets(&b, "## Responsif", r.Responsive)
	bullets(&b, "## Interaksi", r.Interaction)
	for _, pr := range r.Platforms {
		bullets(&b, "## Aturan platform "+pr.Platform, pr.Rules)
	}
	return b.String()
}

// layoutMarkdown is the layout-spec artifact: shells, then every page with
// its sections in visual order.
func layoutMarkdown(p *core.Project) string {
	if p.UI == nil || p.UI.Layout == nil || len(p.UI.Layout.Pages) == 0 {
		return ""
	}
	l := p.UI.Layout
	var b strings.Builder
	b.WriteString("## Peta halaman\n\n| Halaman | Platform | Route | Pattern | Shell | Akses | Fitur |\n|---|---|---|---|---|---|---|\n")
	for _, pg := range l.Pages {
		name := pg.Name
		if pg.Existing {
			name += " *(sudah ada)*"
		}
		fmt.Fprintf(&b, "| `%s` %s | %s | %s | %s | %s | %s | %s |\n", pg.ID, cell(name), pg.Platform, cell(dash(pg.Route)),
			pg.Pattern, dash(pg.Shell), dash(pg.Access), dash(pg.FeatureID))
	}
	b.WriteString("\n")
	if flows := pageFlows(p); flows != "" {
		b.WriteString(flows)
	}
	if ux := uxMarkdown(l); ux != "" {
		b.WriteString(ux)
	}
	if len(l.Shells) > 0 {
		b.WriteString("## Shell layout\n\n")
		for _, sh := range l.Shells {
			fmt.Fprintf(&b, "### `%s` %s\n\nPattern **%s**", sh.ID, sh.Name, sh.Pattern)
			if sh.Platform != "" {
				fmt.Fprintf(&b, " · platform %s", sh.Platform)
			}
			b.WriteString("\n\n")
			if sh.Existing {
				fmt.Fprintf(&b, "Sudah ada di kode%s; halaman baru masuk ke shell ini.\n\n", inFile(sh.Source))
			}
			field(&b, "Navigasi", sh.Navigation)
			field(&b, "Grid", sh.Grid)
			bullets(&b, "**Responsif**", sh.Responsive)
			sections(&b, sh.Regions)
		}
	}
	b.WriteString("## Halaman\n\n")
	for _, pg := range l.Pages {
		fmt.Fprintf(&b, "### `%s` %s\n\n%s\n\n", pg.ID, pg.Name, pg.Purpose)
		meta := []string{"Platform " + pg.Platform, "pattern **" + pg.Pattern + "**"}
		if pg.Route != "" {
			meta = append(meta, "route `"+pg.Route+"`")
		}
		if pg.Shell != "" {
			meta = append(meta, "shell `"+pg.Shell+"`")
		}
		if pg.Access != "" {
			meta = append(meta, "akses "+pg.Access)
		}
		if pg.Role != "" {
			meta = append(meta, "peran "+pg.Role)
		}
		if pg.FeatureID != "" {
			meta = append(meta, "fitur "+pg.FeatureID)
		}
		b.WriteString(strings.Join(meta, " · ") + "\n\n")
		if pg.Existing {
			b.WriteString("Halaman sudah ada di kode: ubah seperlunya, jangan buat ulang.\n\n")
		}
		if pg.Goal != "" {
			fmt.Fprintf(&b, "Tujuan pengguna: %s\n\n", pg.Goal)
		}
		field(&b, "Aksi utama", pg.PrimaryAction)
		field(&b, "Grid", pg.Grid)
		if len(pg.Hierarchy) > 0 {
			fmt.Fprintf(&b, "Hierarki: %s\n\n", strings.Join(pg.Hierarchy, " > "))
		}
		if len(pg.Fields) > 0 {
			b.WriteString("**Pemetaan field ke tampilan**\n\n| Field | Label | Tampilan | Format | Catatan |\n|---|---|---|---|---|\n")
			for _, f := range pg.Fields {
				fmt.Fprintf(&b, "| `%s` | %s | %s | %s | %s |\n", cell(f.Field), cell(f.Label), cell(f.Presentation), cell(f.Format), cell(f.Note))
			}
			b.WriteString("\n")
		}
		sections(&b, pg.Sections)
		states(&b, pg.States)
		bullets(&b, "**Responsif**", pg.Responsive)
		if len(pg.Links) > 0 {
			b.WriteString("**Navigasi keluar**\n\n")
			for _, ln := range pg.Links {
				fmt.Fprintf(&b, "- %s → `%s`\n", ln.Via, ln.To)
			}
			b.WriteString("\n")
		}
		bullets(&b, "**Data**", pg.Data)
	}
	b.WriteString(uiAcceptanceMarkdown(l))
	return b.String()
}

// uiAcceptanceMarkdown is the checkable acceptance of the UI design. It is
// derived from the spec only; it never claims a result and never reads the
// visual audit, so recording a review does not change the design artifact's
// hash.
func uiAcceptanceMarkdown(l *core.LayoutSpec) string {
	var b strings.Builder
	b.WriteString("## Kriteria penerimaan UI/UX\n\nBerlaku untuk setiap halaman yang disebut di atas:\n\n")
	bullets(&b, "", []string{
		"Tujuan pengguna halaman terlihat dari judul dan aksi utama, tanpa menebak dari nama tabel.",
		"Setiap halaman dapat dicapai dari navigasi atau link halaman lain; tidak ada halaman yatim.",
		"Setiap keputusan layout, komponen dan interaksi punya alasan yang tertelusur ke tujuan, alur, data, permission, platform atau token.",
		"Semua warna, tipografi, spasi, radius dan breakpoint berasal dari design token; tidak ada nilai hardcode.",
		"Setiap state yang diwajibkan pola halaman tampil dan dapat digunakan (bukan hanya teks di dokumen).",
		"Kontras WCAG AA, fokus keyboard, label kontrol, alt gambar dan target sentuh terpenuhi.",
		"Layout bertahan di breakpoint yang ditetapkan tanpa overflow horizontal dan tanpa informasi yang hilang.",
		"Field yang ditampilkan sesuai pemetaan field; tidak menampilkan seluruh entity tanpa alasan.",
	})
	b.WriteString("| Halaman | Aksi utama | State wajib | Field dipetakan | Keputusan |\n|---|---|---|---|---|\n")
	for _, pg := range l.Pages {
		states := core.RequiredStates[pg.Pattern]
		decisions := 0
		for _, d := range l.Decisions {
			if d.Subject == pg.ID {
				decisions++
			}
		}
		fmt.Fprintf(&b, "| `%s` %s | %s | %s | %d | %d |\n", pg.ID, cell(pg.Name), cell(dash(pg.PrimaryAction)),
			cell(dash(strings.Join(states, ", "))), len(pg.Fields), decisions)
	}
	b.WriteString("\nBukti visual diperiksa terpisah di `review/visual.md` setelah implementasi; halaman tanpa screenshot dan snapshot terukur belum diuji, dan build atau unit test bukan bukti kualitas UI.\n")
	return b.String()
}

// uxMarkdown is the UX layer: who the users are, what they must achieve,
// how the information is organized and why the design choices were made.
func uxMarkdown(l *core.LayoutSpec) string {
	if len(l.Users) == 0 && len(l.Goals) == 0 && len(l.Navigation) == 0 && len(l.Decisions) == 0 &&
		len(l.Assumptions) == 0 && len(l.Unknowns) == 0 {
		return ""
	}
	var b strings.Builder
	if len(l.Users) > 0 {
		b.WriteString("## Pengguna dan konteks\n\n| Peran | Platform | Konteks | Tujuan | Tugas |\n|---|---|---|---|---|\n")
		for _, u := range l.Users {
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", cell(u.Role), cell(u.Platform), cell(u.Context),
				cell(strings.Join(u.Goals, "; ")), cell(strings.Join(u.Tasks, "; ")))
		}
		b.WriteString("\n")
	}
	if len(l.Goals) > 0 {
		b.WriteString("## Tujuan pengguna\n\n| ID | Peran | Tujuan | Keberhasilan | Prioritas |\n|---|---|---|---|---|\n")
		for _, g := range l.Goals {
			fmt.Fprintf(&b, "| `%s` | %s | %s | %s | %s |\n", cell(g.ID), cell(g.Role), cell(g.Text), cell(g.Success), cell(g.Priority))
		}
		b.WriteString("\n")
	}
	if len(l.Navigation) > 0 {
		b.WriteString("## Information architecture\n\n")
		for _, g := range l.Navigation {
			fmt.Fprintf(&b, "**%s**\n\n", g.Name)
			for _, it := range g.Items {
				primary := ""
				if it.Primary {
					primary = " (utama)"
				}
				fmt.Fprintf(&b, "- %s → `%s`%s\n", it.Label, it.Page, primary)
			}
			b.WriteString("\n")
		}
	}
	if len(l.Decisions) > 0 {
		b.WriteString("## Keputusan desain\n\nSetiap pilihan ditelusuri ke tujuan, alur, data, permission, platform atau token.\n\n")
		b.WriteString("| Subjek | Aspek | Pilihan | Alasan | Dasar | Sumber | Alternatif |\n|---|---|---|---|---|---|---|\n")
		for _, d := range l.Decisions {
			fmt.Fprintf(&b, "| `%s` | %s | %s | %s | %s | %s | %s |\n", cell(d.Subject), cell(dash(d.Topic)), cell(d.Choice),
				cell(strings.Join(d.Why, "; ")), cell(strings.Join(d.Basis, ", ")), cell(d.Source), cell(d.Alternative))
		}
		b.WriteString("\n")
	}
	bullets(&b, "**Asumsi desain**", l.Assumptions)
	bullets(&b, "**Belum diketahui**", l.Unknowns)
	return b.String()
}

func pageFlows(p *core.Project) string {
	var b strings.Builder
	for _, fl := range p.Flows {
		if len(fl.PageIDs) > 0 {
			fmt.Fprintf(&b, "- **%s %s**: %s\n", fl.ID, fl.Name, strings.Join(fl.PageIDs, " → "))
		}
	}
	if b.Len() == 0 {
		return ""
	}
	return "## Alur antarhalaman\n\n" + b.String() + "\n"
}

// sections lists each section as a bold line with its fields as bullets
// (no nested lists: every Markdown reader shows them the same).
func sections(b *strings.Builder, secs []core.Section) {
	for i, s := range secs {
		fmt.Fprintf(b, "**%d. %s** `%s`", i+1, s.Name, s.ID)
		if s.Pattern != "" {
			fmt.Fprintf(b, " (%s)", s.Pattern)
		}
		if s.Purpose != "" {
			fmt.Fprintf(b, ": %s", s.Purpose)
		}
		b.WriteString("\n\n")
		sub := func(name, v string) {
			if v != "" {
				fmt.Fprintf(b, "- %s: %s\n", name, v)
			}
		}
		sub("Isi", strings.Join(s.Content, " · "))
		sub("Komponen", strings.Join(s.Components, ", "))
		sub("Grid", s.Grid)
		sub("Perataan", s.Alignment)
		sub("Spasi", s.Spacing)
		sub("Ukuran", s.Sizing)
		sub("Responsif", s.Responsive)
		for _, k := range core.UIStates {
			sub("State "+k, s.States[k])
		}
		b.WriteString("\n")
	}
}

func states(b *strings.Builder, st map[string]string) {
	if len(st) == 0 {
		return
	}
	b.WriteString("**State**\n\n")
	for _, k := range core.UIStates {
		if v := st[k]; v != "" {
			fmt.Fprintf(b, "- %s: %s\n", k, v)
		}
	}
	b.WriteString("\n")
}

func bullets(b *strings.Builder, title string, items []string) {
	if len(items) == 0 {
		return
	}
	b.WriteString(title + "\n\n")
	for _, it := range items {
		fmt.Fprintf(b, "- %s\n", it)
	}
	b.WriteString("\n")
}

func field(b *strings.Builder, name, v string) {
	if v != "" {
		fmt.Fprintf(b, "%s: %s\n\n", name, v)
	}
}

func codeList(xs []string) string {
	out := make([]string, len(xs))
	for i, x := range xs {
		out[i] = "`" + x + "`"
	}
	return strings.Join(out, ", ")
}

func inFile(src string) string {
	if src == "" {
		return ""
	}
	return " (`" + src + "`)"
}

func dash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
