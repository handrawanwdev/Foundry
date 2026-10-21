package spec

import (
	"fmt"
	"strings"

	"github.com/handrawanw/foundry/internal/core"
	"github.com/handrawanw/foundry/internal/modules/stack"
	"github.com/handrawanw/foundry/internal/modules/structure"
	"github.com/handrawanw/foundry/internal/modules/workflow"
)

// Markdown renders the whole spec as one document: story, blueprint
// artifacts and the command workflow.
func Markdown(p *core.Project) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n%s\n\n", p.Name, p.Story)
	b.WriteString(overviewMarkdown(p))
	for _, a := range Artifacts(p) {
		fmt.Fprintf(&b, "## %s\n\n", a.Title)
		if a.Summary != "" {
			fmt.Fprintf(&b, "%s\n\n", a.Summary)
		}
		b.WriteString(demote(a.Body))
		b.WriteString("\n")
	}
	if w := p.Workflow; w != nil && len(w.Commands) > 0 {
		b.WriteString("## Command Workflow\n\n")
		for i, wave := range workflow.Waves(w.Commands) {
			fmt.Fprintf(&b, "%d. %s\n", i+1, strings.Join(wave, ", "))
		}
		b.WriteString("\n")
		for _, c := range w.Commands {
			b.WriteString(demote(CommandMarkdown(c, nil)))
			b.WriteString("\n")
		}
	}
	return b.String()
}

// demote moves every Markdown heading one level down, so a document can
// sit under a "##" heading. Code blocks are left alone.
func demote(md string) string {
	lines := strings.Split(md, "\n")
	code := false
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "```") {
			code = !code
		}
		if !code && strings.HasPrefix(l, "#") && len(l) < 200 {
			lines[i] = "##" + l
		}
	}
	return strings.Join(lines, "\n")
}

// requirementsMarkdown is the requirements artifact: problem, users,
// requirements, features and flows.
func requirementsMarkdown(p *core.Project) string {
	if len(p.Requirements) == 0 && p.Discovery == nil {
		return ""
	}
	var b strings.Builder
	if d := p.Discovery; d != nil {
		if d.Summary != "" {
			fmt.Fprintf(&b, "%s\n\n", d.Summary)
		}
		if d.Business != "" {
			fmt.Fprintf(&b, "**Tujuan bisnis:** %s\n\n", d.Business)
		}
		if len(d.Roles) > 0 {
			b.WriteString("## Pengguna dan peran\n\n")
			for _, r := range d.Roles {
				fmt.Fprintf(&b, "- **%s** (%s): %s\n", r.Name, r.Platform, r.Description)
			}
			b.WriteString("\n")
		}
	}
	if p.Platform != nil && len(p.Platform.Targets) > 0 {
		b.WriteString("## Platform\n\n")
		for _, t := range p.Platform.Targets {
			fmt.Fprintf(&b, "- %s", t.Kind)
			if t.Rationale != "" {
				fmt.Fprintf(&b, ": %s", t.Rationale)
			}
			b.WriteString("\n")
		}
		if p.Platform.Reason != "" {
			fmt.Fprintf(&b, "\n%s\n", p.Platform.Reason)
		}
		b.WriteString("\n")
	}
	if len(p.Requirements) > 0 {
		b.WriteString("## Requirements\n\n| ID | Prioritas | Kebutuhan |\n|---|---|---|\n")
		for _, r := range p.Requirements {
			fmt.Fprintf(&b, "| %s | %s | %s |\n", r.ID, r.Priority, cell(r.Text))
		}
		b.WriteString("\n")
	}
	if len(p.Features) > 0 {
		b.WriteString("## Fitur\n\n")
		for _, f := range p.Features {
			fmt.Fprintf(&b, "### %s %s\n\n%s\n\nRequirement: %s · Platform: %s\n\n", f.ID, f.Name, f.Description,
				strings.Join(f.RequirementIDs, ", "), strings.Join(f.Platforms, ", "))
			for _, s := range f.SubFeatures {
				fmt.Fprintf(&b, "- %s %s\n", s.ID, s.Name)
			}
			if len(f.SubFeatures) > 0 {
				b.WriteString("\n")
			}
		}
	}
	if len(p.Flows) > 0 {
		b.WriteString("## Alur pengguna\n\n")
		for _, fl := range p.Flows {
			fmt.Fprintf(&b, "- **%s %s** (%s, %s): %s\n", fl.ID, fl.Name, fl.Role, fl.FeatureID, strings.Join(fl.Steps, " → "))
		}
		b.WriteString("\n")
	}
	if d := p.Discovery; d != nil {
		list := func(title string, items []string) {
			if len(items) == 0 {
				return
			}
			fmt.Fprintf(&b, "## %s\n\n", title)
			for _, it := range items {
				fmt.Fprintf(&b, "- %s\n", it)
			}
			b.WriteString("\n")
		}
		list("Data utama", d.Data)
		list("Integrasi", d.Integrations)
		list("Keamanan", d.Security)
		list("Kebutuhan teknis", d.TechNeeds)
	}
	return b.String()
}

// Sources of a survey decision.
const (
	fromUser    = "Pilihan pengguna"
	fromFoundry = "Rekomendasi Foundry"
	fromDefault = "Default Foundry"
	unknown     = "Belum diketahui"
)

// surveyMarkdown is the survey as decisions, each with where it came from:
// the user's answer, Foundry's recommendation, a default, or still unknown.
func surveyMarkdown(p *core.Project) string {
	s := p.Survey
	if s == nil || p.Discovery == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("| Bagian | Keputusan | Sumber |\n|---|---|---|\n")
	row := func(part, decision, source string) {
		fmt.Fprintf(&b, "| %s | %s | %s |\n", part, cell(decision), source)
	}
	pick := func(cs []core.Choice, ids []string) string {
		var out []string
		for _, id := range ids {
			out = append(out, core.ChoiceLabel(cs, id))
		}
		return strings.Join(out, ", ")
	}
	switch {
	case len(s.Kinds) > 0:
		row("Platform", pick(core.SurveyKinds, s.Kinds), fromUser)
	case p.Platform != nil && len(p.Platform.Targets) > 0:
		var kinds []string
		for _, t := range p.Platform.Targets {
			kinds = append(kinds, t.Kind)
		}
		row("Platform", strings.Join(kinds, ", ")+" (dari cerita)", fromFoundry)
	default:
		row("Platform", "Dibaca dari cerita saat discovery", unknown)
	}
	switch {
	case s.Users != "":
		row("Pengguna", core.ChoiceLabel(core.SurveyUsers, s.Users), fromUser)
	case p.Discovery != nil && len(p.Discovery.Roles) > 0:
		var roles []string
		for _, r := range p.Discovery.Roles {
			roles = append(roles, r.Name)
		}
		row("Pengguna", strings.Join(roles, ", ")+" (dari cerita)", fromFoundry)
	default:
		row("Pengguna", "Dibaca dari cerita saat discovery", unknown)
	}
	source := fromUser
	if s.Stage == "" {
		source = fromDefault
	}
	row("Target delivery", core.ChoiceLabel(core.SurveyStages, s.Delivery())+": "+core.DeliveryCriteria[s.Delivery()], source)
	if s.Architecture != "" {
		row("Arsitektur", core.ChoiceLabel(core.SurveyArchs, s.Architecture), fromUser)
	} else {
		row("Arsitektur", "Dipilih dari kebutuhan; lihat Teknologi & Struktur", fromFoundry)
	}
	if len(s.Priorities) > 0 {
		row("Prioritas kualitas", pick(core.SurveyPriorities, s.Priorities), fromUser)
	} else {
		row("Prioritas kualitas", "Tidak ada prioritas khusus", fromDefault)
	}
	if s.Constraints != "" {
		row("Batasan wajib", s.Constraints, fromUser)
	} else {
		row("Batasan wajib", "Tidak ada yang disebutkan", fromDefault)
	}
	b.WriteString("\nKualitas dasar (validasi input, kontrol akses sesuai pengguna, test alur utama) tetap berlaku walau tidak dipilih sebagai prioritas. " +
		"Konflik antarjawaban tercatat di Keputusan, Asumsi & Risiko.\n")
	return b.String()
}

// techMarkdown is the technology artifact: the stack decision and the
// project structure.
func techMarkdown(p *core.Project) string {
	var b strings.Builder
	if st := p.Stack; st != nil && len(st.Decisions) > 0 {
		b.WriteString("## Teknologi\n\n")
		for _, d := range st.Decisions {
			fmt.Fprintf(&b, "- **%s**: %s — %s", d.Area, d.Choice(), d.Reason)
			if o := d.Override; o != nil {
				fmt.Fprintf(&b, " (diganti user dari %s: %s)", d.Recommended, o.Reason)
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	if s := p.Structure; s != nil {
		switch {
		case s.Mode == core.StructureExisting && len(s.Components) > 0:
			b.WriteString("## Struktur project (existing)\n\n")
			for _, c := range s.Components {
				fmt.Fprintf(&b, "- %s: `%s` %s\n", c.Kind, c.Path, c.Tech)
			}
			b.WriteString("\n")
		case s.IR != nil:
			fmt.Fprintf(&b, "## Struktur project\n\nTipe: %s (v%d)\n\n```text\n%s```\n\n", s.IR.Type, s.Version(), structure.Tree(s.IR))
			for _, ps := range [][]core.StructurePart{s.IR.Apps, s.IR.Services, s.IR.Workers, s.IR.Packages} {
				for _, x := range ps {
					fmt.Fprintf(&b, "- `%s` %s\n", x.Path, x.Reason)
				}
			}
			if len(s.IR.Rules) > 0 {
				b.WriteString("\n### Aturan struktur\n\n")
				for i, r := range s.IR.Rules {
					fmt.Fprintf(&b, "%d. %s\n", i+1, r)
				}
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}

// cell makes text safe inside a Markdown table cell.
func cell(s string) string { return strings.NewReplacer("|", "\\|", "\n", " ").Replace(s) }

func overviewMarkdown(p *core.Project) string {
	b := p.Blueprint
	if b == nil || b.Overview.Goal == "" {
		return ""
	}
	var s strings.Builder
	s.WriteString("## Project Overview\n\n")
	fmt.Fprintf(&s, "- **Business Goal:** %s\n", b.Overview.Goal)
	if b.Overview.CoreDomain != "" {
		fmt.Fprintf(&s, "- **Core Domain:** %s\n", b.Overview.CoreDomain)
	}
	if len(b.Overview.Users) > 0 {
		fmt.Fprintf(&s, "- **Target User:** %s\n", strings.Join(b.Overview.Users, ", "))
	}
	if len(b.Overview.Capabilities) > 0 {
		fmt.Fprintf(&s, "- **Main Capability:** %s\n", strings.Join(b.Overview.Capabilities, ", "))
	}
	s.WriteString("\n")
	return s.String()
}

func architectureMarkdown(p *core.Project) string {
	bp := p.Blueprint
	if bp == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("## Architecture\n\n")
	if bp.Architecture.Pattern != "" {
		fmt.Fprintf(&b, "**%s** — %s\n\n", bp.Architecture.Pattern, bp.Architecture.Reason)
		if bp.Architecture.Tradeoff != "" {
			fmt.Fprintf(&b, "Trade-off: %s\n\n", bp.Architecture.Tradeoff)
		}
		if bp.Architecture.Scale != "" {
			fmt.Fprintf(&b, "Scale: %s\n\n", bp.Architecture.Scale)
		}
		if len(bp.Architecture.EvolveWhen) > 0 {
			b.WriteString("Pecah arsitektur ketika:\n")
			for _, e := range bp.Architecture.EvolveWhen {
				fmt.Fprintf(&b, "- %s\n", e)
			}
			b.WriteString("\n")
		}
	}
	if pl := bp.Platform; pl.Reason != "" || len(pl.Devices) > 0 {
		b.WriteString("### Platform\n\n")
		if len(pl.Devices) > 0 {
			fmt.Fprintf(&b, "Device: %s. Offline: %s. Realtime: %s.\n\n", strings.Join(pl.Devices, ", "), pl.Offline, pl.Realtime)
		}
		if pl.Deployment != "" {
			fmt.Fprintf(&b, "Deploy: %s\n\n", pl.Deployment)
		}
	}
	if len(bp.Modules) > 0 {
		b.WriteString("### Modules\n\n| Module | Tanggung jawab | Data | Interface | Depends |\n|---|---|---|---|---|\n")
		for _, m := range bp.Modules {
			fmt.Fprintf(&b, "| `%s` | %s | %s | %s | %s |\n", m.ID, m.Responsibility,
				strings.Join(m.Data, ", "), strings.Join(m.Interface, ", "), strings.Join(m.DependsOn, ", "))
		}
		b.WriteString("\n")
	}
	b.WriteString(databaseMarkdown(bp))
	if bp.Cache.Use != "" {
		fmt.Fprintf(&b, "### Cache\n\n%s. Simpan: %s. Jangan: %s. Invalidation: %s\n\n",
			bp.Cache.Use, strings.Join(bp.Cache.Store, ", "), strings.Join(bp.Cache.Never, ", "), bp.Cache.Invalidation)
	}
	if bp.Storage.Strategy != "" {
		fmt.Fprintf(&b, "### File Storage\n\nStrategy **%s** %s. Upload: %s\n\n", bp.Storage.Strategy, bp.Storage.Provider, bp.Storage.Upload)
		if len(bp.Storage.Security) > 0 {
			for _, x := range bp.Storage.Security {
				fmt.Fprintf(&b, "- %s\n", x)
			}
			b.WriteString("\n")
		}
	}
	if bp.Auth.Method != "" {
		fmt.Fprintf(&b, "### Authentication\n\n%s. Hash %s. %s. Refresh: %s. Session: %s\n\n",
			bp.Auth.Method, bp.Auth.Hashing, bp.Auth.Tokens, bp.Auth.Refresh, bp.Auth.Session)
	}
	if bp.Authz.Model != "" {
		fmt.Fprintf(&b, "### Authorization\n\n%s (`%s`). Permissions: %s\n\n",
			bp.Authz.Model, bp.Authz.Convention, strings.Join(bp.Authz.Permissions, ", "))
	}
	if bp.Realtime.Needed {
		fmt.Fprintf(&b, "### Realtime\n\nBroker %s. Retry: %s\n\n", bp.Realtime.Broker, bp.Realtime.Retry)
		for _, e := range bp.Realtime.Events {
			fmt.Fprintf(&b, "- `%s`\n", e)
		}
		b.WriteString("\n")
	}
	if ev := bp.Events; ev.Envelope != "" || ev.Durable != "" {
		b.WriteString("### Event internal\n\n")
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
			fmt.Fprintf(&b, "- `%s` producer %s\n", e.Type, e.Producer)
		}
		if len(ev.Events) > 0 {
			b.WriteString("\n")
		}
	}
	if len(bp.Workers) > 0 {
		b.WriteString("### Workers\n\n")
		for _, w := range bp.Workers {
			fmt.Fprintf(&b, "- **%s** [%s]: %s\n", w.Name, w.Trigger, w.Job)
		}
		b.WriteString("\n")
	}
	if bp.Aggregation.Needed {
		fmt.Fprintf(&b, "### Aggregation\n\n%s\n\nReads: %s\n\n", bp.Aggregation.Approach, strings.Join(bp.Aggregation.Reads, ", "))
	}
	if bp.Performance.Pagination != "" {
		fmt.Fprintf(&b, "### Performance\n\n%s %s\n\nHindari:\n", bp.Performance.Pagination, bp.Performance.Indexing)
		for _, a := range bp.Performance.Avoid {
			fmt.Fprintf(&b, "- %s\n", a)
		}
		b.WriteString("\n")
	}
	if len(bp.Coding.Backend)+len(bp.Coding.Frontend) > 0 {
		b.WriteString("### Coding Rules\n\n**Backend**\n")
		for _, r := range bp.Coding.Backend {
			fmt.Fprintf(&b, "- %s\n", r)
		}
		b.WriteString("\n**Frontend**\n")
		for _, r := range bp.Coding.Frontend {
			fmt.Fprintf(&b, "- %s\n", r)
		}
		if a := bp.Coding.API; a.Style != "" {
			fmt.Fprintf(&b, "\n**API** %s %s\n\nError: `%s`\n\nPagination: `%s`\n\n", a.Style, a.Versioning, a.Errors, a.Pagination)
		}
	}
	if len(bp.Workflow) > 0 {
		b.WriteString("### Development Workflow\n\n" + strings.Join(bp.Workflow, " → ") + "\n\n")
	}
	if len(bp.ChangeFlow) > 0 {
		b.WriteString("### Change Request\n\n" + strings.Join(bp.ChangeFlow, " → ") + "\n\n")
	}
	if len(bp.AIGuidelines) > 0 {
		b.WriteString("### AI Coding Agent Guidelines\n\n")
		for _, r := range bp.AIGuidelines {
			fmt.Fprintf(&b, "- %s\n", r)
		}
		b.WriteString("\n")
	}
	if fe := bp.Frontend; fe.State != "" {
		fmt.Fprintf(&b, "### Frontend stack detail\n\nState %s · server state %s · forms %s · validation %s · routing %s · API %s\n\n",
			fe.State, fe.ServerState, fe.Forms, fe.Validation, fe.Routing, fe.API)
	}
	if be := bp.Backend; be.Pattern != "" {
		fmt.Fprintf(&b, "### Backend stack detail\n\n%s · DI %s · validation %s · background %s\n\n",
			be.Pattern, be.DI, be.Validation, be.Background)
	}
	return b.String()
}

func databaseMarkdown(bp *core.Blueprint) string {
	d := bp.Database
	if d.Engine == "" && len(d.Entities) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("### Database\n\n")
	fmt.Fprintf(&b, "Engine **%s**, ORM **%s**.\n\n%s\n\nPK: %s. Timestamps: %s. Soft delete: %s. Audit: %s\n\n%s\n\nTransaksi: %s\n\n",
		d.Engine, d.ORM, d.Naming, d.PK, d.Timestamps, d.SoftDelete, d.Audit, d.Migrations, d.Transactions)
	if len(d.Entities) > 0 {
		b.WriteString("| Entity | Module | Fields | Relasi |\n|---|---|---|---|\n")
		for _, e := range d.Entities {
			fmt.Fprintf(&b, "| %s | `%s` | %s | %s |\n", e.Name, e.Module, strings.Join(e.Fields, ", "), strings.Join(e.Relates, ", "))
		}
		b.WriteString("\n")
	}
	if len(d.Indexing) > 0 {
		for _, i := range d.Indexing {
			fmt.Fprintf(&b, "- %s\n", i)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// adrStack records the technology decision per group. It is accepted
// only when the user approved it; until then every choice is proposed.
func adrStack(p *core.Project) string {
	var b strings.Builder
	st := p.Stack
	status := "proposed"
	if st != nil && st.State() == core.TechApproved && len(st.Reviews()) == 0 && len(st.Errors()) == 0 {
		status = "accepted"
	}
	fmt.Fprintf(&b, "# ADR 0001 — Technology stack\n\nStatus: %s\n\n", status)
	if st == nil {
		return b.String() + "Stack belum diputuskan.\n"
	}
	if st.Version() > 0 {
		fmt.Fprintf(&b, "Versi: v%d\n\n", st.Version())
	}
	fmt.Fprintf(&b, "Profil rekomendasi (kebijakan, bukan aturan): %s\n\nKebutuhan teknis: %s\n\n", st.Profile, orDash(strings.Join(st.Needs, ", ")))
	for _, g := range core.TechGroups {
		var ds []core.TechDecision
		for _, a := range g.Areas {
			if d := st.Decision(a); d != nil {
				ds = append(ds, *d)
			}
		}
		if len(ds) == 0 {
			continue
		}
		fmt.Fprintf(&b, "## %s\n\n", g.Title)
		for _, d := range ds {
			fmt.Fprintf(&b, "### %s\n\nPilih **%s**", stack.AreaName(d.Area), d.Choice())
			var attrs []string
			if l := d.Lang(); l != "" {
				attrs = append(attrs, "bahasa "+l)
			}
			if r := d.Run(); r != "" {
				attrs = append(attrs, "runtime "+r)
			}
			if len(attrs) > 0 {
				fmt.Fprintf(&b, " (%s)", strings.Join(attrs, ", "))
			}
			fmt.Fprintf(&b, " — %s.\n\n%s\n", stateLabel[st.DecisionState(d)], d.Reason)
			if o := d.Override; o != nil {
				fmt.Fprintf(&b, "\nDiganti user dari %s", d.Recommended)
				if o.Reason != "" {
					fmt.Fprintf(&b, ": %s", o.Reason)
				}
				b.WriteString(".\n")
			}
			if len(d.Evidence) > 0 {
				fmt.Fprintf(&b, "\nBukti di kode: %s\n", strings.Join(d.Evidence, ", "))
			}
			if d.Review != "" {
				fmt.Fprintf(&b, "\nPerlu ditinjau: %s\n", d.Review)
			}
			if len(d.Alternatives) > 0 {
				fmt.Fprintf(&b, "\nAlternatif: %s\n", strings.Join(d.Alternatives, ", "))
			}
			b.WriteString("\n")
		}
	}
	if len(st.Issues) > 0 {
		b.WriteString("## Temuan validasi\n\n")
		for _, i := range st.Issues {
			fmt.Fprintf(&b, "- %s (%s): %s\n", i.Level, i.Area, i.Message)
		}
		b.WriteString("\n")
	}
	if p.Blueprint != nil && p.Blueprint.Architecture.Pattern != "" {
		fmt.Fprintf(&b, "## Architecture pattern\n\nPilih **%s**. %s\n\n%s\n",
			p.Blueprint.Architecture.Pattern, p.Blueprint.Architecture.Reason, p.Blueprint.Architecture.Tradeoff)
	}
	return b.String()
}

var stateLabel = map[string]string{
	core.StateRecommended: "rekomendasi Foundry, belum disetujui", core.StateExisting: "sudah dipakai di kode",
	core.StateSelected: "dipilih user, belum disetujui", core.StateApproved: "disetujui",
	core.StateNeedsReview: "perlu ditinjau", core.StateConflict: "konflik yang belum diselesaikan",
}

// Spec is the machine-readable spec an external coding agent consumes:
// the structured blueprint, the design rules, the layout spec and the
// command workflow.
type Spec struct {
	Name         string             `json:"name"`
	Story        string             `json:"story"`
	Requirements []core.Requirement `json:"requirements"`
	Features     []core.Feature     `json:"features"`
	Flows        []core.Flow        `json:"flows"`
	Platform     *core.PlatformPlan `json:"platform,omitempty"`
	Stack        *core.TechStack    `json:"stack,omitempty"`
	Structure    *core.Structure    `json:"structure,omitempty"`
	Blueprint    *core.Blueprint    `json:"blueprint,omitempty"`
	DesignRules  *core.DesignRules  `json:"design_rules,omitempty"`
	Layout       *core.LayoutSpec   `json:"layout,omitempty"`
	// Artifacts are the blueprint documents written for this project
	// (the built-in ones are the structured fields above).
	Artifacts []core.Artifact `json:"artifacts"`
	// Decisions, Skips and Done are the decision register, the areas
	// left out and the Definition of Done.
	Decisions []core.Decision `json:"decisions,omitempty"`
	Skips     []core.AreaSkip `json:"area_skips,omitempty"`
	Done      []string        `json:"definition_of_done,omitempty"`
	Workflow  *core.Workflow  `json:"workflow,omitempty"`
}

// Export builds the Spec from the project.
func Export(p *core.Project) Spec {
	s := Spec{Name: p.Name, Story: p.Story, Requirements: p.Requirements, Features: p.Features, Flows: p.Flows,
		Platform: p.Platform, Stack: p.Stack, Structure: p.Structure, Blueprint: p.Blueprint,
		Artifacts: p.Artifacts, Decisions: p.Decisions, Skips: p.Skips, Done: p.Done, Workflow: p.Workflow}
	if s.Artifacts == nil {
		s.Artifacts = []core.Artifact{}
	}
	if p.UI != nil {
		s.DesignRules, s.Layout = p.UI.Rules, p.UI.Layout
	}
	return s
}
