package verify

import (
	"fmt"
	"strings"

	"github.com/handrawanw/foundry/internal/core"
)

// Markdown renders the project documentation from the single context:
// story, requirements, features, pages, tasks and traceability.
func Markdown(p *core.Project) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n%s\n\n", p.Name, p.Story)
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
