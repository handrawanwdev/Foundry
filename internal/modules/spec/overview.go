package spec

import (
	"fmt"
	"slices"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
)

const undetermined = "Belum ditentukan dalam rancangan."

// systemOverviewMarkdown is the layperson explanation of the product.
// It uses only the current blueprint. Empty fields and open decisions are
// named; nothing is invented.
func systemOverviewMarkdown(p *core.Project) string {
	if p == nil || !hasOverviewSource(p) {
		return ""
	}
	var b strings.Builder
	if open := overviewOpening(p); open != "" {
		b.WriteString(open)
		b.WriteString("\n\n")
	}
	b.WriteString("Isi ini mengikuti rancangan yang ada. Bagian yang belum diisi atau masih terbuka dicatat di akhir.\n\n")
	writeOverviewSection(&b, "Tujuan", overviewPurpose(p))
	writeOverviewSection(&b, "Fitur utama", overviewFeatures(p))
	writeOverviewSection(&b, "Pengguna dan wewenang", overviewRoles(p))
	writeOverviewSection(&b, "Alur kerja", overviewFlows(p))
	writeOverviewSection(&b, "Aturan yang berlaku", overviewRules(p))
	writeOverviewSection(&b, "Hubungan antarbagian", overviewRelations(p))
	writeOverviewSection(&b, "Contoh penggunaan", overviewExamples(p))
	writeOverviewSection(&b, "Manfaat", overviewBenefits(p))
	writeOverviewSection(&b, "Yang belum ditentukan", overviewUnknowns(p))
	return strings.TrimSpace(b.String()) + "\n"
}

func hasOverviewSource(p *core.Project) bool {
	if strings.TrimSpace(p.Story) != "" || len(p.Features) > 0 || len(p.Flows) > 0 || len(p.Requirements) > 0 {
		return true
	}
	if d := p.Discovery; d != nil && (d.Summary != "" || d.Business != "" || len(d.Roles) > 0 || len(d.Users) > 0) {
		return true
	}
	return p.Blueprint != nil && (p.Blueprint.Overview.Goal != "" || len(p.Blueprint.Overview.Capabilities) > 0)
}

func overviewOpening(p *core.Project) string {
	if d := p.Discovery; d != nil {
		if s := strings.TrimSpace(d.Summary); s != "" {
			return s
		}
	}
	if p.Blueprint != nil {
		if g := strings.TrimSpace(p.Blueprint.Overview.Goal); g != "" {
			return g
		}
	}
	if s := firstParagraph(p.Story); s != "" {
		return s
	}
	if n := strings.TrimSpace(p.Name); n != "" {
		return "Dokumen ini menjelaskan " + n + "."
	}
	return ""
}

func writeOverviewSection(b *strings.Builder, title, body string) {
	fmt.Fprintf(b, "## %s\n\n", title)
	body = strings.TrimSpace(body)
	if body == "" {
		b.WriteString(undetermined)
	} else {
		b.WriteString(body)
	}
	b.WriteString("\n\n")
}

func overviewPurpose(p *core.Project) string {
	var b strings.Builder
	goal, business, domain := "", "", ""
	if p.Blueprint != nil {
		goal = strings.TrimSpace(p.Blueprint.Overview.Goal)
		domain = strings.TrimSpace(p.Blueprint.Overview.CoreDomain)
	}
	if d := p.Discovery; d != nil {
		business = strings.TrimSpace(d.Business)
	}
	if business != "" {
		fmt.Fprintf(&b, "Masalah yang diselesaikan: %s\n\n", business)
	}
	if goal != "" && !equalFoldSpace(goal, business) {
		fmt.Fprintf(&b, "Tujuan sistem: %s\n\n", goal)
	}
	if domain != "" && !equalFoldSpace(domain, goal) && !equalFoldSpace(domain, business) {
		fmt.Fprintf(&b, "Lingkup utama: %s\n\n", domain)
	}
	if p.Platform != nil && strings.TrimSpace(p.Platform.Reason) != "" {
		fmt.Fprintf(&b, "Tempat dipakai: %s\n\n", strings.TrimSpace(p.Platform.Reason))
	} else if p.Platform != nil && len(p.Platform.Targets) > 0 {
		var places []string
		for _, t := range p.Platform.Targets {
			places = append(places, platformPlain(t.Kind))
		}
		fmt.Fprintf(&b, "Tempat dipakai: %s.\n\n", joinID(places))
	}
	return b.String()
}

func overviewFeatures(p *core.Project) string {
	if len(p.Features) == 0 {
		if p.Blueprint != nil && len(p.Blueprint.Overview.Capabilities) > 0 {
			var b strings.Builder
			b.WriteString("Kemampuan yang tercatat:\n\n")
			for _, c := range p.Blueprint.Overview.Capabilities {
				if s := strings.TrimSpace(c); s != "" {
					fmt.Fprintf(&b, "- %s\n", s)
				}
			}
			return b.String()
		}
		return ""
	}
	var b strings.Builder
	for _, f := range p.Features {
		name := strings.TrimSpace(f.Name)
		if name == "" {
			name = f.ID
		}
		fmt.Fprintf(&b, "### %s\n\n", name)
		if d := strings.TrimSpace(f.Description); d != "" {
			fmt.Fprintf(&b, "%s\n\n", d)
		}
		if len(f.SubFeatures) > 0 {
			b.WriteString("Yang termasuk dalam fitur ini:\n\n")
			for _, s := range f.SubFeatures {
				n := strings.TrimSpace(s.Name)
				if n == "" {
					n = s.ID
				}
				fmt.Fprintf(&b, "- %s\n", n)
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}

func overviewRoles(p *core.Project) string {
	var b strings.Builder
	seen := map[string]bool{}
	if d := p.Discovery; d != nil {
		for _, r := range d.Roles {
			name := strings.TrimSpace(r.Name)
			if name == "" || seen[strings.ToLower(name)] {
				continue
			}
			seen[strings.ToLower(name)] = true
			fmt.Fprintf(&b, "### %s\n\n", name)
			if desc := strings.TrimSpace(r.Description); desc != "" {
				fmt.Fprintf(&b, "%s\n\n", desc)
			}
			if pl := strings.TrimSpace(r.Platform); pl != "" {
				fmt.Fprintf(&b, "Memakai sistem lewat %s.\n\n", platformPlain(pl))
			}
		}
		if len(d.Roles) == 0 {
			for _, u := range d.Users {
				if s := strings.TrimSpace(u); s != "" && !seen[strings.ToLower(s)] {
					seen[strings.ToLower(s)] = true
					fmt.Fprintf(&b, "- %s\n", s)
				}
			}
			if len(d.Users) > 0 {
				b.WriteString("\n")
			}
		}
	}
	if p.Blueprint != nil {
		for _, u := range p.Blueprint.Overview.Users {
			if s := strings.TrimSpace(u); s != "" && !seen[strings.ToLower(s)] {
				seen[strings.ToLower(s)] = true
				fmt.Fprintf(&b, "- %s\n", s)
			}
		}
	}
	for _, f := range p.Flows {
		if r := strings.TrimSpace(f.Role); r != "" && !seen[strings.ToLower(r)] {
			seen[strings.ToLower(r)] = true
			fmt.Fprintf(&b, "- %s\n", r)
		}
	}
	if perms := rolePermissions(p); perms != "" {
		b.WriteString("Batas wewenang yang tercatat:\n\n")
		b.WriteString(perms)
	} else if b.Len() > 0 {
		b.WriteString("Batas wewenang per peran belum diisi.\n")
	}
	if d := p.Discovery; d != nil && p.Blueprint != nil && len(d.Users) > 0 && len(p.Blueprint.Overview.Users) > 0 &&
		!sameSet(d.Users, p.Blueprint.Overview.Users) {
		b.WriteString("\nDaftar pengguna di ringkasan dan di gambaran sistem berbeda. Belum ada keputusan mana yang diikuti.\n")
	}
	return b.String()
}

func rolePermissions(p *core.Project) string {
	var b strings.Builder
	if p.Blueprint != nil && strings.TrimSpace(p.Blueprint.Authz.Model) != "" {
		fmt.Fprintf(&b, "- Aturan akses: %s.\n", authzPlain(p.Blueprint.Authz.Model))
	}
	type row struct{ role, text string }
	var rows []row
	seen := map[string]bool{}
	for _, f := range p.Flows {
		role := strings.TrimSpace(f.Role)
		if role == "" || len(f.Permissions) == 0 {
			continue
		}
		text := joinID(trimList(f.Permissions))
		if text == "" {
			continue
		}
		key := strings.ToLower(role + "\x00" + text)
		if seen[key] {
			continue
		}
		seen[key] = true
		rows = append(rows, row{role, text})
	}
	for _, r := range rows {
		fmt.Fprintf(&b, "- %s: %s.\n", r.role, r.text)
	}
	return b.String()
}

func overviewFlows(p *core.Project) string {
	if len(p.Flows) == 0 {
		if d := p.Discovery; d != nil && len(d.Workflows) > 0 {
			var b strings.Builder
			b.WriteString("Alur yang tercatat:\n\n")
			for _, w := range d.Workflows {
				if s := strings.TrimSpace(w); s != "" {
					fmt.Fprintf(&b, "- %s\n", s)
				}
			}
			return b.String()
		}
		return ""
	}
	var b strings.Builder
	for _, f := range p.Flows {
		name := strings.TrimSpace(f.Name)
		if name == "" {
			name = f.ID
		}
		fmt.Fprintf(&b, "### %s\n\n", name)
		if r := strings.TrimSpace(f.Role); r != "" {
			fmt.Fprintf(&b, "Siapa yang mengerjakan: %s.\n\n", r)
		}
		if g := strings.TrimSpace(f.Goal); g != "" {
			fmt.Fprintf(&b, "Tujuan: %s.\n\n", g)
		}
		listBlock(&b, "Sebelum mulai", f.Pre, false)
		listBlock(&b, "Langkah", f.Steps, true)
		listBlock(&b, "Berhasil bila", f.Post, false)
		listBlock(&b, "Jalan lain", f.Alternatives, false)
		listBlock(&b, "Bila gagal", f.Failures, false)
	}
	return b.String()
}

func overviewRules(p *core.Project) string {
	var b strings.Builder
	seen := map[string]bool{}
	for _, f := range p.Flows {
		for _, r := range f.Rules {
			s := strings.TrimSpace(r)
			if s == "" || seen[strings.ToLower(s)] {
				continue
			}
			seen[strings.ToLower(s)] = true
			fmt.Fprintf(&b, "- %s\n", s)
		}
	}
	for _, d := range p.Decisions {
		if d.Open() {
			continue
		}
		res := strings.TrimSpace(d.Resolution)
		if res == "" {
			continue
		}
		fmt.Fprintf(&b, "- %s\n", res)
	}
	return b.String()
}

func overviewRelations(p *core.Project) string {
	var b strings.Builder
	if p.Blueprint != nil {
		for _, m := range p.Blueprint.Modules {
			name := strings.TrimSpace(m.ID)
			if name == "" {
				continue
			}
			fmt.Fprintf(&b, "### %s\n\n", name)
			if r := strings.TrimSpace(m.Responsibility); r != "" {
				fmt.Fprintf(&b, "%s\n\n", r)
			}
			if len(m.DependsOn) > 0 {
				fmt.Fprintf(&b, "Bekerja bersama: %s.\n\n", joinID(trimList(m.DependsOn)))
			}
		}
		for _, e := range p.Blueprint.Events.Events {
			if strings.TrimSpace(e.Type) == "" {
				continue
			}
			line := "Ketika " + e.Type + " terjadi"
			if pr := strings.TrimSpace(e.Producer); pr != "" {
				line += ", " + pr + " memberi tahu"
			}
			if cs := joinID(trimList(e.Consumers)); cs != "" {
				line += " " + cs
			}
			fmt.Fprintf(&b, "- %s.\n", line)
		}
		if len(p.Blueprint.Events.Events) > 0 {
			b.WriteString("\n")
		}
		for _, w := range p.Blueprint.Workers {
			name := strings.TrimSpace(w.Name)
			job := strings.TrimSpace(w.Job)
			if name == "" && job == "" {
				continue
			}
			line := name
			if job != "" {
				if line != "" {
					line += ": " + job
				} else {
					line = job
				}
			}
			if t := workerTriggerPlain(w.Trigger); t != "" {
				line += " (" + t + ")"
			}
			fmt.Fprintf(&b, "- %s.\n", line)
		}
		if len(p.Blueprint.Workers) > 0 {
			b.WriteString("\n")
		}
	}
	if d := p.Discovery; d != nil && len(d.Integrations) > 0 {
		b.WriteString("Layanan di luar sistem:\n\n")
		for _, it := range d.Integrations {
			if s := strings.TrimSpace(it); s != "" {
				fmt.Fprintf(&b, "- %s\n", s)
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

func overviewExamples(p *core.Project) string {
	n := 0
	var b strings.Builder
	for _, f := range p.Flows {
		if len(trimList(f.Steps)) == 0 {
			continue
		}
		n++
		if n > 3 {
			break
		}
		who := strings.TrimSpace(f.Role)
		if who == "" {
			who = "Pengguna"
		}
		want := strings.TrimSpace(f.Goal)
		if want == "" {
			want = strings.TrimSpace(f.Name)
		}
		fmt.Fprintf(&b, "%s ingin %s.\n\n", who, want)
		fmt.Fprintf(&b, "Langkahnya: %s.\n\n", joinThen(trimList(f.Steps)))
		if post := joinID(trimList(f.Post)); post != "" {
			fmt.Fprintf(&b, "Bila berhasil: %s.\n\n", post)
		}
		if fail := joinID(trimList(f.Failures)); fail != "" {
			fmt.Fprintf(&b, "Bila gagal: %s.\n\n", fail)
		}
	}
	return b.String()
}

func overviewBenefits(p *core.Project) string {
	var caps []string
	if p.Blueprint != nil {
		caps = trimList(p.Blueprint.Overview.Capabilities)
	}
	if len(caps) == 0 {
		for _, f := range p.Features {
			if n := strings.TrimSpace(f.Name); n != "" {
				caps = append(caps, n)
			}
		}
	}
	if len(caps) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Dengan sistem ini, pengguna dapat:\n\n")
	for _, c := range caps {
		fmt.Fprintf(&b, "- %s\n", c)
	}
	return b.String()
}

func overviewUnknowns(p *core.Project) string {
	var b strings.Builder
	var conflicts, open []core.Decision
	for _, d := range p.Decisions {
		if !d.Open() {
			continue
		}
		if d.Kind == core.DecisionConflict {
			conflicts = append(conflicts, d)
		} else {
			open = append(open, d)
		}
	}
	if len(conflicts) > 0 {
		b.WriteString("Konflik yang tercatat, belum diputuskan:\n\n")
		for _, d := range conflicts {
			fmt.Fprintf(&b, "- %s\n", strings.TrimSpace(d.Text))
		}
		b.WriteString("\n")
	}
	if len(open) > 0 {
		b.WriteString("Pertanyaan, asumsi, atau risiko yang masih terbuka:\n\n")
		for _, d := range open {
			line := strings.TrimSpace(d.Text)
			if d.Critical {
				line += " (perlu diputuskan sebelum rancangan dianggap lengkap)"
			}
			fmt.Fprintf(&b, "- %s\n", line)
		}
		b.WriteString("\n")
	}
	if len(p.Skips) > 0 {
		b.WriteString("Bagian yang sengaja tidak dicakup:\n\n")
		for _, s := range p.Skips {
			title := s.Area
			if a := core.AreaByID(s.Area); a != nil {
				title = a.Title
			}
			fmt.Fprintf(&b, "- %s: %s\n", title, s.Reason)
		}
		b.WriteString("\n")
	}
	for _, title := range missingPlainAreas(p) {
		fmt.Fprintf(&b, "- %s belum diuraikan.\n", title)
	}
	if b.Len() == 0 {
		return "Tidak ada informasi terbuka yang tercatat. Rancangan yang ada dipakai apa adanya."
	}
	return b.String()
}

var areaPlain = map[string]string{
	core.AreaUI:          "Tampilan layar",
	core.AreaData:        "Data yang disimpan",
	core.AreaSecurity:    "Keamanan",
	core.AreaIntegration: "Hubungan dengan layanan lain",
	core.AreaOperations:  "Pemantauan dan keandalan",
	core.AreaMigration:   "Pindah dari sistem lama",
}

func missingPlainAreas(p *core.Project) []string {
	covered := map[string]bool{}
	for _, a := range p.Artifacts {
		if a.Area != "" {
			covered[a.Area] = true
		}
		for _, area := range core.Areas {
			if slices.Contains(area.Artifacts, a.ID) {
				covered[area.ID] = true
			}
		}
	}
	if p.UI != nil && p.UI.Rules != nil && p.UI.Layout != nil && len(p.UI.Layout.Pages) > 0 {
		covered[core.AreaUI] = true
	}
	var out []string
	for _, id := range []string{core.AreaUI, core.AreaData, core.AreaSecurity, core.AreaIntegration, core.AreaOperations, core.AreaMigration} {
		if p.Skip(id) != "" || covered[id] {
			continue
		}
		if applies, _ := p.AreaApplies(id); !applies {
			continue
		}
		out = append(out, areaPlain[id])
	}
	return out
}

func listBlock(b *strings.Builder, title string, items []string, numbered bool) {
	items = trimList(items)
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(b, "%s:\n\n", title)
	for i, it := range items {
		if numbered {
			fmt.Fprintf(b, "%d. %s\n", i+1, it)
		} else {
			fmt.Fprintf(b, "- %s\n", it)
		}
	}
	b.WriteString("\n")
}

func trimList(in []string) []string {
	var out []string
	for _, s := range in {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func joinID(in []string) string {
	switch len(in) {
	case 0:
		return ""
	case 1:
		return in[0]
	case 2:
		return in[0] + " dan " + in[1]
	}
	return strings.Join(in[:len(in)-1], ", ") + ", dan " + in[len(in)-1]
}

func joinThen(in []string) string {
	switch len(in) {
	case 0:
		return ""
	case 1:
		return in[0]
	}
	return strings.Join(in[:len(in)-1], ", lalu ") + ", lalu " + in[len(in)-1]
}

func firstParagraph(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if i := strings.Index(s, "\n\n"); i >= 0 {
		s = s[:i]
	}
	return strings.Join(strings.Fields(s), " ")
}

func equalFoldSpace(a, b string) bool {
	return strings.EqualFold(strings.Join(strings.Fields(a), " "), strings.Join(strings.Fields(b), " "))
}

func sameSet(a, b []string) bool {
	norm := func(in []string) map[string]bool {
		m := map[string]bool{}
		for _, s := range in {
			s = strings.ToLower(strings.Join(strings.Fields(s), " "))
			if s != "" {
				m[s] = true
			}
		}
		return m
	}
	x, y := norm(a), norm(b)
	if len(x) != len(y) {
		return false
	}
	for k := range x {
		if !y[k] {
			return false
		}
	}
	return true
}

func platformPlain(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case core.PlatformWeb:
		return "situs web"
	case core.PlatformAndroid:
		return "aplikasi Android"
	case core.PlatformIOS:
		return "aplikasi iPhone"
	case core.PlatformDesktop:
		return "aplikasi komputer"
	case core.PlatformBackend:
		return "layanan di belakang layar"
	case core.PlatformCLI:
		return "perintah teks"
	case core.PlatformEmbedded:
		return "perangkat atau firmware"
	case core.PlatformOther:
		return "sistem tanpa layar sendiri"
	}
	return kind
}

func authzPlain(model string) string {
	switch strings.ToLower(strings.TrimSpace(model)) {
	case "rbac":
		return "wewenang mengikuti peran"
	case "acl":
		return "wewenang mengikuti daftar izin"
	case "rebac":
		return "wewenang mengikuti hubungan antar pengguna"
	}
	return model
}

func workerTriggerPlain(t string) string {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "queue":
		return "saat ada antrean kerja"
	case "cron":
		return "pada jadwal berkala"
	case "event":
		return "saat ada kejadian tertentu"
	}
	return strings.TrimSpace(t)
}
