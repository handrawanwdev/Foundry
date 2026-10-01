package verify

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/handrawanw/foundry/internal/core"
)

// Markdown renders the project documentation from the single context:
// story, requirements, features, pages, tasks and traceability.
func Markdown(p *core.Project) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n%s\n\n", p.Name, p.Story)
	b.WriteString(overviewMarkdown(p))
	if d := p.Discovery; d != nil {
		fmt.Fprintf(&b, "## Ringkasan\n\n%s\n\n**Bisnis:** %s\n\n", d.Summary, d.Business)
		if len(d.Roles) > 0 {
			b.WriteString("**Peran:**\n")
			for _, r := range d.Roles {
				fmt.Fprintf(&b, "- %s (%s): %s\n", r.Name, r.Platform, r.Description)
			}
			b.WriteString("\n")
		}
	}
	if p.Platform != nil {
		b.WriteString("## Platform\n\n")
		for _, t := range p.Platform.Targets {
			fmt.Fprintf(&b, "- %s: %s\n", t.Kind, strings.Join(t.Audience, ", "))
		}
		fmt.Fprintf(&b, "\n%s\n\n", p.Platform.Reason)
	}
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
		b.WriteString("## Struktur Project\n\n")
		if s.Mode == core.StructureExisting {
			for _, c := range s.Components {
				fmt.Fprintf(&b, "- %s: `%s` %s\n", c.Kind, c.Path, c.Tech)
			}
		} else {
			fmt.Fprintf(&b, "Template: %s\n\n", s.Template)
			for _, l := range s.Layout {
				fmt.Fprintf(&b, "- `%s`\n", l)
			}
		}
		b.WriteString("\n")
	}
	b.WriteString(architectureMarkdown(p))
	b.WriteString("## Requirements\n\n")
	for _, r := range p.Requirements {
		fmt.Fprintf(&b, "- **%s** [%s] %s\n", r.ID, r.Priority, r.Text)
	}
	b.WriteString("\n## Features\n\n")
	for _, f := range p.Features {
		fmt.Fprintf(&b, "### %s %s\n\n%s\n\nRequirement: %s · Platform: %s\n\n", f.ID, f.Name, f.Description,
			strings.Join(f.RequirementIDs, ", "), strings.Join(f.Platforms, ", "))
		for _, s := range f.SubFeatures {
			fmt.Fprintf(&b, "- %s %s\n", s.ID, s.Name)
		}
		b.WriteString("\n")
	}
	if p.Design != nil {
		b.WriteString("## Halaman\n\n")
		for _, pg := range p.Design.Pages {
			fmt.Fprintf(&b, "- **%s** %s (%s, %s): %s\n", pg.ID, pg.Name, pg.Platform, pg.FeatureID, pg.Purpose)
		}
		b.WriteString("\n")
		if s := p.Design.System; s != nil {
			b.WriteString("## Design system\n\n")
			if s.Direction != "" {
				b.WriteString("Arah visual: " + s.Direction + "\n\n")
			}
			b.WriteString("| Grup | Token | Nilai | Pemakaian |\n|---|---|---|---|\n")
			for _, g := range []struct {
				name   string
				tokens []core.Token
			}{{"Warna", s.Colors}, {"Tipografi", s.Typography}, {"Spacing", s.Spacing}, {"Radius", s.Radius},
				{"Shadow", s.Shadow}, {"Breakpoint", s.Breakpoints}, {"Komponen", s.Components}} {
				for _, t := range g.tokens {
					cell := strings.NewReplacer("|", "\\|", "\n", " ")
					fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", g.name, cell.Replace(t.Name), cell.Replace(t.Value), cell.Replace(t.Use))
				}
			}
			b.WriteString("\n")
		}
	}
	b.WriteString("## Task\n\n| ID | Task | Feature | Status |\n|---|---|---|---|\n")
	for _, t := range p.Tasks {
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", t.ID, t.Title, t.FeatureID, t.Status)
	}
	if v := p.Verification; v != nil {
		b.WriteString("\n## Traceability\n\n| Requirement | Feature | Page | Task | Test | Selesai |\n|---|---|---|---|---|---|\n")
		for _, t := range v.Trace {
			done := "belum"
			if t.Done {
				done = "ya"
			}
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s |\n", t.RequirementID, strings.Join(t.FeatureIDs, ", "),
				strings.Join(t.PageIDs, ", "), strings.Join(t.TaskIDs, ", "), strings.Join(t.TestTaskIDs, ", "), done)
		}
	}
	return b.String()
}

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

// WriteDocs writes the architecture documentation into docs/, the contract
// coding agents read on disk.
func WriteDocs(dir string, p *core.Project) error {
	if dir == "" || p == nil {
		return nil
	}
	root := filepath.Join(dir, "docs")
	if err := os.MkdirAll(filepath.Join(root, "adr"), 0o755); err != nil {
		return err
	}
	for name, body := range DocsFiles(p) {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// DocsFiles is the documentation set derived from the project context.
func DocsFiles(p *core.Project) map[string]string {
	out := map[string]string{
		"README.md":         Markdown(p),
		"architecture.md":   "# Architecture\n\n" + strings.TrimPrefix(architectureMarkdown(p), "## Architecture\n\n"),
		"database.md":       "# Database\n\n" + databaseDoc(p),
		"api.md":            "# API\n\n" + apiDoc(p),
		"security.md":       "# Security\n\n" + securityDoc(p),
		"storage.md":        "# Storage\n\n" + storageDoc(p),
		"performance.md":    "# Performance\n\n" + performanceDoc(p),
		"workflow.md":       "# Workflow\n\n" + workflowDoc(p),
		"deployment.md":     "# Deployment\n\n" + deploymentDoc(p),
		"adr/0001-stack.md": adrStack(p),
	}
	return out
}

func databaseDoc(p *core.Project) string {
	if p.Blueprint == nil {
		return "Belum ada blueprint.\n"
	}
	return databaseMarkdown(p.Blueprint)
}

func apiDoc(p *core.Project) string {
	if p.Blueprint == nil {
		return "Belum ada blueprint.\n"
	}
	a := p.Blueprint.Coding.API
	var b strings.Builder
	fmt.Fprintf(&b, "Style: %s\n\nVersion: `%s`\n\nError: `%s`\n\nPagination: `%s`\n\n", a.Style, a.Versioning, a.Errors, a.Pagination)
	b.WriteString("Public interface per module:\n\n")
	for _, m := range p.Blueprint.Modules {
		if len(m.Interface) == 0 {
			continue
		}
		fmt.Fprintf(&b, "### %s\n\n", m.ID)
		for _, i := range m.Interface {
			fmt.Fprintf(&b, "- `%s`\n", i)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func securityDoc(p *core.Project) string {
	if p.Blueprint == nil {
		return "Belum ada blueprint.\n"
	}
	var b strings.Builder
	au, az := p.Blueprint.Auth, p.Blueprint.Authz
	fmt.Fprintf(&b, "## Authentication\n\n%s\n\nHash: %s\n\n%s\n\nRefresh: %s\n\nSession: %s\n\n",
		au.Method, au.Hashing, au.Tokens, au.Refresh, au.Session)
	fmt.Fprintf(&b, "## Authorization\n\n%s `%s`\n\n", az.Model, az.Convention)
	for _, perm := range az.Permissions {
		fmt.Fprintf(&b, "- `%s`\n", perm)
	}
	if d := p.Discovery; d != nil && len(d.Security) > 0 {
		b.WriteString("\n## Requirements\n\n")
		for _, s := range d.Security {
			fmt.Fprintf(&b, "- %s\n", s)
		}
	}
	return b.String()
}

func storageDoc(p *core.Project) string {
	if p.Blueprint == nil {
		return "Belum ada blueprint.\n"
	}
	s := p.Blueprint.Storage
	if s.Strategy == "" || s.Strategy == "none" {
		return "Tidak ada penyimpanan file terpisah; file besar dilarang masuk database.\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Strategy: **%s** (%s)\n\nUpload: %s\n\nCDN: %s\n\nLifecycle: %s\n\n",
		s.Strategy, s.Provider, s.Upload, s.CDN, s.Lifecycle)
	if len(s.Types) > 0 {
		fmt.Fprintf(&b, "Jenis: %s\n\n", strings.Join(s.Types, ", "))
	}
	if len(s.Security) > 0 {
		b.WriteString("## Security\n\n")
		for _, x := range s.Security {
			fmt.Fprintf(&b, "- %s\n", x)
		}
	}
	if len(s.Processing) > 0 {
		b.WriteString("\n## Worker\n\n")
		for _, x := range s.Processing {
			fmt.Fprintf(&b, "- %s\n", x)
		}
	}
	b.WriteString("\n## Metadata File\n\n`id, original_name, storage_key, bucket, mime_type, size, checksum, uploaded_by, created_at`\n")
	return b.String()
}

func performanceDoc(p *core.Project) string {
	if p.Blueprint == nil {
		return "Belum ada blueprint.\n"
	}
	pf := p.Blueprint.Performance
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\nCache: %s\n\nIndex: %s\n\n", pf.Pagination, pf.Caching, pf.Indexing)
	if p.Blueprint.Aggregation.Needed {
		fmt.Fprintf(&b, "\n## Read model\n\n%s\n\n%s\n", p.Blueprint.Aggregation.Approach, strings.Join(p.Blueprint.Aggregation.Reads, ", "))
	}
	if len(pf.Avoid) > 0 {
		b.WriteString("\n## Hindari\n\n")
		for _, a := range pf.Avoid {
			fmt.Fprintf(&b, "- %s\n", a)
		}
	}
	return b.String()
}

func workflowDoc(p *core.Project) string {
	if p.Blueprint == nil {
		return "Belum ada blueprint.\n"
	}
	var b strings.Builder
	b.WriteString("## Development\n\n" + strings.Join(p.Blueprint.Workflow, " → ") + "\n\n")
	b.WriteString("## Change request\n\n" + strings.Join(p.Blueprint.ChangeFlow, " → ") + "\n\n")
	if len(p.Blueprint.AIGuidelines) > 0 {
		b.WriteString("## AI coding agent\n\n")
		for _, r := range p.Blueprint.AIGuidelines {
			fmt.Fprintf(&b, "- %s\n", r)
		}
	}
	return b.String()
}

func deploymentDoc(p *core.Project) string {
	if p.Blueprint != nil && p.Blueprint.Platform.Deployment != "" {
		return p.Blueprint.Platform.Deployment + "\n"
	}
	return "Container untuk API; klien web/mobile di-build terpisah.\n"
}

func adrStack(p *core.Project) string {
	var b strings.Builder
	b.WriteString("# ADR 0001 — Technology stack\n\nStatus: accepted\n\n")
	if p.Stack == nil {
		return b.String() + "Stack belum diputuskan.\n"
	}
	fmt.Fprintf(&b, "Profile: %s\n\nNeeds: %s\n\n", p.Stack.Profile, strings.Join(p.Stack.Needs, ", "))
	for _, d := range p.Stack.Decisions {
		fmt.Fprintf(&b, "## %s\n\nPilih **%s**. %s\n\n", d.Area, d.Choice(), d.Reason)
	}
	if p.Blueprint != nil && p.Blueprint.Architecture.Pattern != "" {
		fmt.Fprintf(&b, "## architecture pattern\n\nPilih **%s**. %s\n\n%s\n",
			p.Blueprint.Architecture.Pattern, p.Blueprint.Architecture.Reason, p.Blueprint.Architecture.Tradeoff)
	}
	return b.String()
}
