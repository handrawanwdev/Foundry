// Package structure decides the project layout. Existing projects keep their
// own architecture (detected by scan); new projects get a template.
package structure

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/handrawanw/foundry/internal/core"
	"github.com/handrawanw/foundry/internal/modules/scan"
)

type Template struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Layout      []string `json:"layout"`
}

var Templates = []Template{
	{ID: "simple", Name: "Simple", Description: "Cocok untuk aplikasi kecil",
		Layout: []string{"src/", "tests/", "docs/"}},
	{ID: "modular", Name: "Modular", Description: "Cocok untuk aplikasi bisnis: backend per modul + web",
		Layout: []string{"services/api/", "apps/web/", "docs/"}},
	{ID: "monorepo", Name: "Monorepo", Description: "Web + Mobile + Backend dengan kode bersama",
		Layout: []string{"apps/web/", "apps/mobile/", "services/api/", "packages/shared/", "packages/ui/", "docs/"}},
	{ID: "microservices", Name: "Microservices", Description: "Layanan terpisah per domain; hanya untuk skala dan deployment independen",
		Layout: []string{"apps/web/", "services/", "docs/"}},
}

func TemplateByID(id string) (Template, bool) {
	for _, t := range Templates {
		if t.ID == id {
			return t, true
		}
	}
	return Template{}, false
}

// Detect scans dir. Existing code wins over any template.
func Detect(dir string, platform *core.PlatformPlan) *core.Structure {
	r := scan.Scan(dir)
	if r.Existing {
		return &core.Structure{
			Mode:       core.StructureExisting,
			Components: r.Components,
			Commands:   r.Commands,
			Reason:     "Foundry akan mengikuti struktur project yang sudah ada.",
		}
	}
	t, reason := Suggest(platform)
	return &core.Structure{
		Mode:     core.StructureNew,
		Template: t.ID,
		Layout:   t.Layout,
		Reason:   reason,
	}
}

// Suggest picks a template from platform needs.
func Suggest(p *core.PlatformPlan) (Template, string) {
	kinds := map[string]bool{}
	if p != nil {
		for _, t := range p.Targets {
			kinds[t.Kind] = true
		}
	}
	mobile := kinds[core.PlatformAndroid] || kinds[core.PlatformIOS] || kinds[core.PlatformDesktop]
	switch {
	case mobile:
		t, _ := TemplateByID("monorepo")
		return t, "Struktur ini dipilih agar Web, Mobile, dan Backend tetap terpisah tetapi dapat berbagi kode yang diperlukan."
	case kinds[core.PlatformBackend] && kinds[core.PlatformWeb]:
		t, _ := TemplateByID("modular")
		return t, "Backend dan Web dipisah; backend dibagi per modul bisnis agar mudah dikembangkan."
	default:
		t, _ := TemplateByID("simple")
		return t, "Aplikasi kecil cukup satu folder sumber agar sederhana."
	}
}

// Apply switches a new-project structure to another template.
func Apply(s *core.Structure, templateID string) error {
	if s.Mode == core.StructureExisting {
		return fmt.Errorf("project existing: struktur tidak diganti template")
	}
	t, ok := TemplateByID(templateID)
	if !ok {
		return fmt.Errorf("template %q tidak dikenal", templateID)
	}
	s.Template, s.Layout = t.ID, t.Layout
	return nil
}

// Scaffold creates the template folders for a new project. It never touches
// an existing project and never overwrites files.
func Scaffold(dir string, s *core.Structure) error {
	if s.Mode != core.StructureNew {
		return nil
	}
	for _, l := range s.Layout {
		p := filepath.Join(dir, filepath.FromSlash(strings.TrimSuffix(l, "/")))
		if err := os.MkdirAll(p, 0o755); err != nil {
			return err
		}
		keep := filepath.Join(p, ".gitkeep")
		if _, err := os.Stat(keep); os.IsNotExist(err) {
			if err := os.WriteFile(keep, nil, 0o644); err != nil {
				return err
			}
		}
	}
	s.Scaffolded = true
	return nil
}
