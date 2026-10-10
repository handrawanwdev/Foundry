package work

import (
	"path"
	"strings"

	"github.com/handrawanw/foundry/internal/core"
)

// junkExt are leftovers no task should land: logs, merge/backup and editor
// files.
var junkExt = map[string]bool{".log": true, ".orig": true, ".rej": true, ".bak": true, ".swp": true, ".tmp": true}

// StrayFile reports whether an isolated task's changed file falls outside
// the approved structure and is discarded instead of merged: junk files
// anywhere, notes/summary files new at the repository root, and new
// top-level folders the structure does not have. base holds the root names
// of the snapshot the task started from.
func StrayFile(p *core.Project, base map[string]bool, file string) bool {
	k := normPath(file)
	if k == "" {
		return false
	}
	if junkExt[path.Ext(k)] {
		return true
	}
	segs := strings.Split(k, "/")
	root := segs[0]
	// base names keep their case; k is lowered.
	exact := strings.SplitN(strings.TrimPrefix(path.Clean(strings.ReplaceAll(file, "\\", "/")), "./"), "/", 2)[0]
	if base[exact] || strings.HasPrefix(root, ".") || DependencyFile(k) {
		return false
	}
	if len(segs) == 1 {
		switch k {
		case "foundry.md", "agents.md", "claude.md", "docker-compose.yml", ".env.example", "readme.md":
			return false
		}
		ext := path.Ext(k)
		return ext == ".md" || ext == ".txt"
	}
	roots := structureRoots(p)
	return len(roots) > 0 && !roots[root]
}

// structureRoots are the top-level folders the project structure names;
// nil when code may live at the repository root itself.
func structureRoots(p *core.Project) map[string]bool {
	s := p.Structure
	if s == nil {
		return nil
	}
	roots := map[string]bool{}
	atRoot := false
	add := func(dir string) {
		if strings.TrimSpace(dir) == "" {
			return
		}
		if k := normPath(dir); k != "" {
			roots[strings.Split(k, "/")[0]] = true
		} else {
			atRoot = true
		}
	}
	for _, l := range s.Layout {
		add(l)
	}
	for _, c := range s.Components {
		add(c.Path)
	}
	if ir := s.IR; ir != nil {
		for _, ps := range [][]core.StructurePart{ir.Apps, ir.Services, ir.Workers, ir.Packages} {
			for _, x := range ps {
				add(x.Path)
			}
		}
		add(ir.Docs)
	}
	if atRoot {
		return nil
	}
	return roots
}
