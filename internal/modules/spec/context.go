package spec

import (
	"fmt"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
)

func projectContextMarkdown(p *core.Project) string {
	if p.Structure == nil || p.Structure.Mode != core.StructureExisting {
		return ""
	}
	c := p.Structure.Context
	if c == nil || (c.Empty() && strings.TrimSpace(c.Corrections) == "") {
		return ""
	}
	var b strings.Builder
	b.WriteString("Kondisi project dibaca dari repository. `observed` berasal dari kode atau manifest; `documented` dari dokumentasi dan belum diverifikasi di kode; `estimated` hanya dari AI, bukan scanner. Path bukan tanggung jawab kecuali evidence mengatakannya.\n\n")
	if c.Stale {
		b.WriteString("**Konteks kedaluwarsa.** Repository berubah setelah pemindaian ini")
		if len(c.StaleAreas) > 0 {
			fmt.Fprintf(&b, " (area: %s)", strings.Join(c.StaleAreas, ", "))
		}
		b.WriteString(". Jangan memakai temuan ini sebagai fakta terkini sebelum dipindai ulang.\n\n")
	}
	if !c.ScannedAt.IsZero() {
		fmt.Fprintf(&b, "Dipindai: %s", c.ScannedAt.UTC().Format("2006-01-02T15:04:05Z"))
		if c.Fingerprint != "" {
			fmt.Fprintf(&b, " · sidik %s", c.Fingerprint)
		}
		b.WriteString("\n\n")
	}
	writeFindingSection(&b, "Layout", c.Layout)
	writeFindingSection(&b, "Bahasa", c.Languages)
	writeFindingSection(&b, "Framework", c.Frameworks)
	if len(c.Modules) > 0 {
		b.WriteString("## Modul\n\n")
		for _, m := range c.Modules {
			fmt.Fprintf(&b, "- `%s` — %s", m.Path, m.Evidence)
			if len(m.Depends) > 0 {
				fmt.Fprintf(&b, " → %s", strings.Join(m.Depends, ", "))
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	if len(c.Deps) > 0 {
		b.WriteString("## Dependensi teramati\n\n| Dari | Ke | Jenis | Bukti |\n|---|---|---|---|\n")
		for _, d := range c.Deps {
			fmt.Fprintf(&b, "| `%s` | `%s` | %s | %s `%s` |\n", d.From, d.To, d.Kind, d.Evidence, d.Path)
		}
		b.WriteString("\n")
	}
	if len(c.Changes) > 0 {
		b.WriteString("## Perubahan sejak pemindaian sebelumnya\n\n")
		for _, ch := range c.Changes {
			fmt.Fprintf(&b, "- %s `%s` (%s)\n", ch.Action, ch.Path, ch.Area)
		}
		b.WriteString("\n")
	}
	if len(c.Data) > 0 {
		b.WriteString("## Data model\n\n")
		for _, e := range c.Data {
			fields := ""
			if len(e.Fields) > 0 {
				fields = " (" + strings.Join(e.Fields, ", ") + ")"
			}
			fmt.Fprintf(&b, "- **%s**%s — `%s` (%s)\n", e.Name, fields, e.Path, e.Evidence)
		}
		b.WriteString("\n")
	}
	if len(c.APIs) > 0 {
		b.WriteString("## API dan rute\n\n")
		for _, a := range c.APIs {
			detail := a.Detail
			if detail == "" {
				detail = a.Kind
			}
			fmt.Fprintf(&b, "- %s — `%s` (%s)\n", detail, a.Path, a.Evidence)
		}
		b.WriteString("\n")
	}
	writeFindingSection(&b, "Autentikasi", c.Auth)
	writeFindingSection(&b, "Testing", c.Tests)
	writeFindingSection(&b, "Dokumentasi", c.Docs)
	writeFindingSection(&b, "Konvensi", c.Conventions)
	writeFindingSection(&b, "Fitur terdokumentasi", c.Features)
	if s := strings.TrimSpace(c.Corrections); s != "" {
		b.WriteString("## Koreksi pengguna\n\n")
		b.WriteString(s)
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String()) + "\n"
}

func writeFindingSection(b *strings.Builder, title string, fs []core.ContextFinding) {
	if len(fs) == 0 {
		return
	}
	fmt.Fprintf(b, "## %s\n\n", title)
	for _, f := range fs {
		conf := f.Confidence
		if conf == "" {
			conf = core.ConfidenceObserved
		}
		path := ""
		if f.Path != "" {
			path = " `" + f.Path + "`"
		}
		fmt.Fprintf(b, "- %s%s [%s] — %s\n", f.Text, path, conf, f.Evidence)
	}
	b.WriteString("\n")
}

func changePlanMarkdown(p *core.Project) string {
	c := p.Change
	if c == nil || c.Empty() {
		return ""
	}
	var b strings.Builder
	intent := p.IntentOrDefault()
	fmt.Fprintf(&b, "Jenis pekerjaan: **%s**. Scope blueprint: **%s**.\n\n",
		core.ChoiceLabel(core.WorkIntents, string(intent)), or(c.Scope, core.BlueprintScopeChange))
	if c.Summary != "" {
		fmt.Fprintf(&b, "%s\n\n", c.Summary)
	}
	if len(c.Existing) > 0 {
		b.WriteString("## Dipakai ulang\n\n")
		for _, s := range c.Existing {
			fmt.Fprintf(&b, "- %s\n", s)
		}
		b.WriteString("\n")
	}
	if len(c.Items) > 0 {
		b.WriteString("## Perubahan\n\n| Aksi | Target | Modul | Alasan | File |\n|---|---|---|---|---|\n")
		for _, it := range c.Items {
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", it.Action, it.Target, it.Module, it.Reason, strings.Join(it.Files, ", "))
		}
		b.WriteString("\n")
	}
	if len(c.Impact) > 0 {
		b.WriteString("## Dampak\n\n| Area | Jenis | Sumber | Risiko | Dampak |\n|---|---|---|---|---|\n")
		for _, im := range c.Impact {
			src := im.Source
			if src == "" {
				src = core.ConfidenceEstimated
			}
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", im.Area, im.Kind, src, im.Risk, im.Message)
		}
		b.WriteString("\n")
	}
	if len(c.Kept) > 0 {
		b.WriteString("## Dipertahankan dari blueprint utama\n\n")
		for _, s := range c.Kept {
			fmt.Fprintf(&b, "- %s\n", s)
		}
		b.WriteString("\n")
	}
	if len(c.Conflicts) > 0 {
		b.WriteString("## Konflik dengan blueprint utama\n\n")
		for _, cf := range c.Conflicts {
			fmt.Fprintf(&b, "- **%s**: %s", cf.Kind, cf.Message)
			if cf.Keep != "" {
				fmt.Fprintf(&b, " (pertahankan %s)", cf.Keep)
			}
			b.WriteByte('\n')
		}
		b.WriteString("\n")
	}
	if traces := core.Traceability(p); len(traces) > 0 && (c.Scope == core.BlueprintScopeChange || p.IntentOrDefault().ChangeScope()) {
		b.WriteString("## Keterlacakan requirement\n\n| Requirement | Fitur | Domain | Modul | File | Acceptance | Tes | Command |\n|---|---|---|---|---|---|---|---|\n")
		for _, t := range traces {
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s | %s | %s |\n",
				t.RequirementID, t.FeatureID, t.Domain, t.Module,
				strings.Join(t.Files, ", "), strings.Join(t.Acceptance, ", "),
				strings.Join(t.Tests, ", "), strings.Join(t.Commands, ", "))
		}
		b.WriteString("\n")
	}
	writeChangeFileSection(&b, "File baru", c.Files.Add)
	writeChangeFileSection(&b, "File diubah", c.Files.Modify)
	writeChangeFileSection(&b, "File dihapus", c.Files.Remove)
	b.WriteString("Rencana ini mengikuti arsitektur utama. Command workflow merujuk file dan modul di sini.\n")
	return b.String()
}

func writeChangeFileSection(b *strings.Builder, title string, files []core.ChangeFile) {
	if len(files) == 0 {
		return
	}
	fmt.Fprintf(b, "## %s\n\n", title)
	for _, f := range files {
		fmt.Fprintf(b, "- `%s`", f.Path)
		if f.Responsibility != "" {
			fmt.Fprintf(b, " — %s", f.Responsibility)
		}
		if f.Reason != "" {
			fmt.Fprintf(b, " (%s)", f.Reason)
		}
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
}

func or(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}
