package standard

import (
	"fmt"
	"strings"

	"github.com/handrawanw/foundry/internal/core"
)

// Profile is docs/foundry-profile.md: the technical decision for this project.
func Profile(p *core.Project) string {
	if p == nil {
		return "# Foundry Profile\n\nBelum ada project.\n"
	}
	var b strings.Builder
	b.WriteString("# Foundry Profile\n\n")
	fmt.Fprintf(&b, "Project **%s**. Keputusan teknis final. Coding agent mengikuti file ini dan FOUNDRY.md.\n\n", p.Name)
	if s := p.Survey; s != nil {
		s.FillDefaults()
		b.WriteString("## Survey\n\n")
		fmt.Fprintf(&b, "- Pengguna: %s\n- Tahap: %s\n- Latency: %s\n- Webhook: %s\n- Service: %s\n",
			s.Users, s.Stage, latencyLabel(p), s.Webhook, s.Services)
		if len(s.Kinds) > 0 {
			fmt.Fprintf(&b, "- Jenis: %s\n", strings.Join(s.Kinds, ", "))
		}
		if s.Stack != "" {
			fmt.Fprintf(&b, "- Preferensi stack: %s\n", s.Stack)
		}
		b.WriteString("\n")
	}
	if st := p.Stack; st != nil {
		fmt.Fprintf(&b, "## Stack\n\nProfile Foundry: **%s**. Kebutuhan: %s.\n\nStack dipilih deterministik dari profil Foundry, bukan riset ad-hoc. Alternatif tercatat per area.\n\n",
			st.Profile, strings.Join(st.Needs, ", "))
		for _, d := range st.Decisions {
			fmt.Fprintf(&b, "- **%s**: %s — %s\n", d.Area, d.Choice(), d.Reason)
			if len(d.Alternatives) > 0 {
				fmt.Fprintf(&b, "  Alternatif: %s\n", strings.Join(d.Alternatives, ", "))
			}
		}
		b.WriteString("\n")
	}
	if s := p.Structure; s != nil && s.IR != nil {
		b.WriteString("## Struktur folder aktual\n\n")
		fmt.Fprintf(&b, "Tipe: **%s**\n\n", s.IR.Type)
		for _, ps := range [][]core.StructurePart{s.IR.Apps, s.IR.Services, s.IR.Workers, s.IR.Packages} {
			for _, x := range ps {
				fmt.Fprintf(&b, "- `%s`", x.Path)
				if len(x.Modules) > 0 {
					fmt.Fprintf(&b, " modules: %s", strings.Join(x.Modules, ", "))
				}
				if x.Reason != "" {
					b.WriteString(" — " + x.Reason)
				}
				b.WriteString("\n")
			}
		}
		b.WriteString("\nPemetaan peran → lokasi:\n\n")
		b.WriteString("- Domain murni: `packages/domain`\n")
		b.WriteString("- Kontrak/event/DTO: `packages/contracts`\n")
		b.WriteString("- Token dan komponen UI: `packages/ui`\n")
		b.WriteString("- HTTP + webhook: `apps/api` (`src/modules/{fitur}/`)\n")
		b.WriteString("- Consumer event + job: `apps/worker`\n")
		b.WriteString("- ORM hanya di `{fitur}.repository`. Infra (DB, Redis, bus) di `src/infra`.\n\n")
	}
	if bp := p.Blueprint; bp != nil {
		if bp.Database.ORM != "" {
			fmt.Fprintf(&b, "## ORM\n\n**%s** pada **%s**. Akses database hanya di repository.\n\n", bp.Database.ORM, bp.Database.Engine)
		}
		if ev := bp.Events; ev.Durable != "" || ev.Envelope != "" {
			b.WriteString("## Event\n\n")
			if ev.Durable != "" {
				fmt.Fprintf(&b, "Durable: %s\n\n", ev.Durable)
			}
			if ev.Ephemeral != "" {
				fmt.Fprintf(&b, "Ephemeral: %s\n\n", ev.Ephemeral)
			}
			if ev.Outbox != "" {
				fmt.Fprintf(&b, "Outbox: %s\n\n", ev.Outbox)
			}
			if ev.Envelope != "" {
				fmt.Fprintf(&b, "Envelope: `%s`\n\n", ev.Envelope)
			}
			for _, e := range ev.Events {
				fmt.Fprintf(&b, "- `%s` producer %s consumer %s\n", e.Type, e.Producer, strings.Join(e.Consumers, ", "))
			}
			if len(ev.Events) > 0 {
				b.WriteString("\n")
			}
		}
		fmt.Fprintf(&b, "## Latency\n\nAnggaran %s. Health: %s\n\n", latencyLabel(p), bp.Performance.Health)
		if len(bp.Coding.Backend)+len(bp.Coding.Frontend) > 0 {
			b.WriteString("## Batas kode\n\n")
			for _, r := range bp.Coding.Backend {
				fmt.Fprintf(&b, "- %s\n", r)
			}
			for _, r := range bp.Coding.Frontend {
				fmt.Fprintf(&b, "- %s\n", r)
			}
			b.WriteString("\n")
		}
	}
	if p.Survey != nil && p.Survey.Stage != "" {
		fmt.Fprintf(&b, "## Asumsi\n\nTahap **%s**. Redis boleh mati tanpa kehilangan data. Database adalah sumber kebenaran.\n", p.Survey.Stage)
	}
	return b.String()
}

// RequirementsDoc is docs/requirements.md.
func RequirementsDoc(p *core.Project) string {
	if p == nil {
		return "# Requirements\n\nBelum ada kebutuhan.\n"
	}
	var b strings.Builder
	b.WriteString("# Requirements\n\n")
	if p.Story != "" {
		fmt.Fprintf(&b, "## Masalah\n\n%s\n\n", p.Story)
	}
	if d := p.Discovery; d != nil {
		if d.Business != "" {
			fmt.Fprintf(&b, "%s\n\n", d.Business)
		}
		if len(d.Users) > 0 {
			fmt.Fprintf(&b, "Pengguna: %s\n\n", strings.Join(d.Users, ", "))
		}
	}
	b.WriteString("## Fitur MVP\n\n")
	n := 0
	for _, f := range p.Features {
		n++
		if n > 5 {
			break
		}
		fmt.Fprintf(&b, "- **%s %s**: %s\n", f.ID, f.Name, f.Description)
	}
	if n == 0 {
		b.WriteString("Belum ada fitur.\n")
	}
	b.WriteString("\n## Non-goals\n\n")
	b.WriteString("- Fitur di luar MVP di atas\n- Mengganti stack atau struktur yang sudah disetujui tanpa ADR\n")
	if p.Survey != nil && p.Survey.Stage == core.SurveyStageMVP {
		b.WriteString("- Skala enterprise (SSO, multi-region) sebelum ada bukti kebutuhan\n")
	}
	b.WriteString("\n## Skenario pengguna\n\n")
	for _, fl := range p.Flows {
		fmt.Fprintf(&b, "- **%s** (%s): %s\n", fl.Name, fl.Role, strings.Join(fl.Steps, " → "))
	}
	b.WriteString("\n## Ukuran sukses\n\n")
	fmt.Fprintf(&b, "- Setiap requirement MUST punya task DONE dengan bukti\n- Latency operasi utama %s\n- Build dan test project lulus\n", latencyLabel(p))
	b.WriteString("\n## Risiko\n\n")
	b.WriteString("- Redis down: aplikasi tetap benar, cache/event tertunda\n- Event duplikat: consumer idempotent by eventId\n")
	return b.String()
}

// DesignBrief is docs/design-brief.md. Empty of frontend → short skip note.
func DesignBrief(p *core.Project) string {
	hasUI := false
	if p != nil && p.Platform != nil {
		for _, t := range p.Platform.Targets {
			switch t.Kind {
			case core.PlatformWeb, core.PlatformAndroid, core.PlatformIOS, core.PlatformDesktop:
				hasUI = true
			}
		}
	}
	if p != nil && p.Design != nil && len(p.Design.Pages) > 0 {
		hasUI = true
	}
	if !hasUI {
		return "# Design Brief\n\nTidak ada frontend. File ini sengaja pendek.\n"
	}
	var b strings.Builder
	b.WriteString("# Design Brief\n\n")
	b.WriteString("Ditulis sebelum layar diimplementasikan. Coding agent UI wajib mengikuti brief ini dan token di packages/ui.\n\n")
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
