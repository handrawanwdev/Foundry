package work

import (
	"testing"

	"github.com/handrawanw/foundry/internal/core"
)

func TestStrayFile(t *testing.T) {
	p := &core.Project{Structure: &core.Structure{IR: &core.StructureIR{
		Apps:     []core.StructurePart{{Path: "apps/web/"}},
		Services: []core.StructurePart{{Path: "services/api/", Modules: []string{"orders"}}},
		Docs:     "docs/",
	}}}
	base := map[string]bool{"apps": true, "README.md": true, "Makefile": true}
	for f, want := range map[string]bool{
		"apps/web/src/Login.tsx":         false,
		"services/api/orders/handler.go": false, // structure root, new in the tree
		"docs/api.md":                    false,
		"README.md":                      false, // already there
		".github/workflows/ci.yml":       false,
		"package.json":                   false,
		"IMPLEMENTATION_SUMMARY.md":      true,
		"FOUNDRY.md":                     false,
		"AGENTS.md":                      false,
		"notes.txt":                      true,
		"apps/web/debug.log":             true,
		"apps/web/src/App.tsx.orig":      true,
		"scripts/fix.sh":                 true, // new top-level folder
		"utils/format.ts":                true,
	} {
		if got := StrayFile(p, base, f); got != want {
			t.Errorf("StrayFile(%q) = %v, want %v", f, got, want)
		}
	}
	// Code at the repository root: new folders cannot be judged.
	root := &core.Project{Structure: &core.Structure{Components: []core.Component{{Path: "."}}}}
	if StrayFile(root, nil, "utils/format.go") {
		t.Error("root-level project: new folder flagged")
	}
}
