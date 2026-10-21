// Package structure decides the project layout. Existing projects keep their
// own architecture (detected by scan); new projects get a Structure IR that
// Foundry recommends and the user edits and approves (decision.go).
package structure

import (
	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/scan"
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
			Reason:     "Foundry akan mengikuti struktur project yang sudah ada.",
		}
	}
	return &core.Structure{Mode: core.StructureNew}
}
