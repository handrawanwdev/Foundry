// Package scan inspects a project directory: which components exist
// (backend, web, mobile, ...), so the design follows an existing project.
package scan

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/handrawanw/foundry/internal/core"
)

var skipDirs = map[string]bool{
	"node_modules": true, ".git": true, core.StateDir: true, "dist": true, "build": true,
	"vendor": true, ".next": true, "target": true, ".dart_tool": true, "Pods": true, ".venv": true,
}

type pkgJSON struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
	Workspaces      json.RawMessage   `json:"workspaces"`
}

// Result of scanning a directory.
type Result struct {
	Existing   bool
	Components []core.Component

	// "dir|name" of package.json scripts that only fan out to workspace
	// packages (pnpm -r, npm --workspaces, turbo run, ...).
}

// Scan walks dir up to depth 4.
func Scan(dir string) Result {
	r := Result{}
	seen := map[string]bool{}
	add := func(c core.Component) {
		key := c.Kind + "|" + c.Path
		if !seen[key] {
			seen[key] = true
			r.Components = append(r.Components, c)
		}
	}
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		if d.IsDir() {
			if path != dir && (skipDirs[d.Name()] || strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			if strings.Count(rel, string(filepath.Separator)) >= 4 {
				return filepath.SkipDir
			}
			switch d.Name() {
			case "migrations", "prisma":
				add(core.Component{Kind: "database", Path: rel, Evidence: d.Name() + "/"})
			case "packages", "libs", "shared":
				add(core.Component{Kind: "shared", Path: rel, Evidence: d.Name() + "/"})
			case "android":
				add(core.Component{Kind: "mobile", Path: rel, Tech: "Android", Evidence: "android/"})
			case "ios":
				add(core.Component{Kind: "mobile", Path: rel, Tech: "iOS", Evidence: "ios/"})
			case "k8s", "helm", "terraform", "deploy", "infra":
				add(core.Component{Kind: "infra", Path: rel, Evidence: d.Name() + "/"})
			}
			return nil
		}
		relDir := filepath.Dir(rel)
		switch name := d.Name(); {
		case name == "go.mod":
			r.Existing = true
			add(core.Component{Kind: "backend", Path: relDir, Tech: "Go", Evidence: "go.mod"})
		case name == "package.json":
			r.Existing = true
			scanNode(relDir, path, add)
		case name == "pubspec.yaml":
			r.Existing = true
			add(core.Component{Kind: "mobile", Path: relDir, Tech: "Flutter", Evidence: "pubspec.yaml"})
		case name == "Dockerfile" || name == "docker-compose.yml" || name == "compose.yaml":
			add(core.Component{Kind: "infra", Path: relDir, Tech: "Docker", Evidence: name})
		case strings.HasSuffix(name, ".sql"):
			add(core.Component{Kind: "database", Path: relDir, Tech: "SQL", Evidence: name})
		case strings.HasSuffix(name, "_test.go") || strings.Contains(name, ".test.") || strings.Contains(name, ".spec."):
			add(core.Component{Kind: "testing", Path: relDir, Evidence: name})
		}
		return nil
	})
	sort.SliceStable(r.Components, func(i, j int) bool { return r.Components[i].Kind < r.Components[j].Kind })
	return r
}

func scanNode(relDir, path string, add func(core.Component)) {
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var p pkgJSON
	if json.Unmarshal(b, &p) != nil {
		return
	}
	has := func(dep string) bool {
		_, a := p.Dependencies[dep]
		_, b := p.DevDependencies[dep]
		return a || b
	}
	switch {
	case has("expo") || has("react-native"):
		add(core.Component{Kind: "mobile", Path: relDir, Tech: "React Native", Evidence: "package.json"})
	case has("electron") || has("@tauri-apps/api"):
		add(core.Component{Kind: "desktop", Path: relDir, Tech: "Electron/Tauri", Evidence: "package.json"})
	case has("react") || has("vue") || has("svelte") || has("next") || has("vite"):
		add(core.Component{Kind: "web", Path: relDir, Tech: webTech(has), Evidence: "package.json"})
	case has("express") || has("fastify") || has("@nestjs/core") || has("hono"):
		add(core.Component{Kind: "backend", Path: relDir, Tech: "Node", Evidence: "package.json"})
	case len(p.Workspaces) > 0:
		add(core.Component{Kind: "shared", Path: relDir, Tech: "Workspace root", Evidence: "package.json workspaces"})
	}
}

func webTech(has func(string) bool) string {
	switch {
	case has("next"):
		return "Next.js"
	case has("vue"):
		return "Vue"
	case has("svelte"):
		return "Svelte"
	default:
		return "React"
	}
}
