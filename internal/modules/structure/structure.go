// Package structure holds the project layout. Existing projects keep their
// own architecture (detected by scan); a new project's Structure IR is
// designed by the AI for that system (plan.go), then edited and approved by
// the user (decision.go). Foundry keeps no structure template.
package structure

import (
	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/scan"
)

// Detect scans dir. Existing code wins over any design; a new project's IR
// comes from the AI structure analysis.
// prev is the last project context; unchanged areas are reused.
func Detect(dir string, platform *core.PlatformPlan, prev *core.ProjectContext) *core.Structure {
	r := scan.Scan(dir)
	if r.Existing {
		return &core.Structure{
			Mode:       core.StructureExisting,
			Components: r.Components,
			Reason:     "Foundry akan mengikuti struktur project yang sudah ada.",
			Context:    scan.Refresh(dir, prev, r.Components),
		}
	}
	return &core.Structure{Mode: core.StructureNew}
}
