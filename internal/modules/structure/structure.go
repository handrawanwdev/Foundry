// Package structure decides the project layout. Existing projects keep their
// own architecture (detected by scan); new projects get a Structure IR that
// Foundry recommends and the user edits and approves (decision.go).
package structure

import (
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
	Layout      []string `json:"layout"` // an example; the real one comes from the project
}

var Templates = []Template{
	{ID: core.StructSimple, Name: "Simple", Description: "Aplikasi kecil, kompleksitas rendah; tanpa batas modul",
		Layout: []string{"apps/web/", "apps/api/", "docs/"}},
	{ID: core.StructModular, Name: "Modular", Description: "Aplikasi bisnis dengan domain jelas; satu API, tiap domain jadi modul",
		Layout: []string{"apps/web/", "apps/api/modules/<domain>/", "packages/domain/", "packages/contracts/", "docs/"}},
	{ID: core.StructMonorepo, Name: "Monorepo", Description: "Beberapa aplikasi berbagi domain, kontrak, UI atau tooling",
		Layout: []string{"apps/web/", "apps/mobile/", "apps/api/modules/<domain>/", "packages/domain/", "packages/contracts/", "docs/"}},
	{ID: core.StructMicroservices, Name: "Microservices", Description: "Hanya bila perlu deploy/skala independen, ownership dan isolasi failure per domain",
		Layout: []string{"apps/web/", "services/<domain>/", "packages/domain/", "packages/contracts/", "docs/"}},
}

func TemplateByID(id string) (Template, bool) {
	for _, t := range Templates {
		if t.ID == id {
			return t, true
		}
	}
	return Template{}, false
}

// Detect scans dir. Existing code wins over any recommendation; a new
// project's IR is filled by Sync once the technology is decided.
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
	return &core.Structure{Mode: core.StructureNew}
}

// Scaffold creates the folders of the structure IR for a new project. It
// never touches an existing project and never overwrites files.
func Scaffold(dir string, s *core.Structure) error {
	if s.Mode != core.StructureNew {
		return nil
	}
	layout := s.Layout
	if s.IR != nil {
		layout = Layout(s.IR)
	}
	for _, l := range layout {
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
