// Package scan inspects a project directory: which components exist
// (backend, web, mobile, ...) and which build/test commands the project
// really declares. Commands are never invented: Go modules get the standard
// go toolchain steps, Node packages only get scripts present in package.json.
package scan

import (
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
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
	Scripts         map[string]string `json:"scripts"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
	Workspaces      json.RawMessage   `json:"workspaces"`
}

// Result of scanning a directory.
type Result struct {
	Existing   bool
	Components []core.Component
	Commands   []core.Command
	Run        []core.Command // how to start the app (dev/start scripts), shown to the user

	// "dir|name" of package.json scripts that only fan out to workspace
	// packages (pnpm -r, npm --workspaces, turbo run, ...).
	delegating map[string]bool
}

// Scan walks dir up to depth 4.
func Scan(dir string) Result {
	r := Result{delegating: map[string]bool{}}
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
			r.Commands = append(r.Commands,
				core.Command{Name: "build", Dir: relDir, Argv: []string{"go", "build", "./..."}},
				core.Command{Name: "vet", Dir: relDir, Argv: []string{"go", "vet", "./..."}},
				core.Command{Name: "test", Dir: relDir, Argv: []string{"go", "test", "./..."}},
			)
		case name == "package.json":
			r.Existing = true
			scanNode(dir, relDir, path, add, &r)
		case name == "pubspec.yaml":
			r.Existing = true
			add(core.Component{Kind: "mobile", Path: relDir, Tech: "Flutter", Evidence: "pubspec.yaml"})
			if _, err := exec.LookPath("flutter"); err == nil {
				r.Commands = append(r.Commands,
					core.Command{Name: "lint", Dir: relDir, Argv: []string{"flutter", "analyze"}},
					core.Command{Name: "test", Dir: relDir, Argv: []string{"flutter", "test"}},
				)
			}
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
	r.Commands = dropDelegating(r.Commands, r.delegating)
	return r
}

// dropDelegating removes a workspace-root script when a package below it
// already runs the same command, so each package is built/tested once.
func dropDelegating(cmds []core.Command, deleg map[string]bool) []core.Command {
	var out []core.Command
	for _, c := range cmds {
		if deleg[c.Dir+"|"+c.Name] && coveredBelow(cmds, c) {
			continue
		}
		out = append(out, c)
	}
	return out
}

func coveredBelow(cmds []core.Command, root core.Command) bool {
	for _, c := range cmds {
		if c.Name == root.Name && c.Dir != root.Dir && (root.Dir == "." || strings.HasPrefix(c.Dir, root.Dir+string(filepath.Separator))) {
			return true
		}
	}
	return false
}

var delegateFlags = map[string]bool{"-r": true, "--recursive": true, "--filter": true, "--workspaces": true, "-ws": true}

func isDelegating(script string) bool {
	for _, phrase := range []string{"yarn workspaces", "turbo run", "lerna run", "nx run-many"} {
		if strings.Contains(script, phrase) {
			return true
		}
	}
	for _, f := range strings.Fields(script) {
		if flag, _, _ := strings.Cut(f, "="); delegateFlags[flag] {
			return true
		}
	}
	return false
}

func scanNode(root, relDir, path string, add func(core.Component), r *Result) {
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
	// Workspace roots usually delegate scripts to packages; only run what exists.
	pm := packageManager(root, relDir)
	for _, name := range []string{"typecheck", "lint", "build", "test", "test:e2e", "audit:perf", "smoke"} {
		if script, ok := p.Scripts[name]; ok {
			r.Commands = append(r.Commands, core.Command{Name: name, Dir: relDir, Argv: []string{pm, "run", name}})
			if isDelegating(script) {
				r.delegating[relDir+"|"+name] = true
			}
		}
	}
	for _, name := range []string{"dev", "start"} {
		if _, ok := p.Scripts[name]; ok {
			r.Run = append(r.Run, core.Command{Name: name, Dir: relDir, Argv: []string{pm, "run", name}})
			break
		}
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

// packageManager picks the manager by lockfile, searching up to the root.
func packageManager(root, relDir string) string {
	d := filepath.Join(root, relDir)
	for {
		for lock, pm := range map[string]string{"pnpm-lock.yaml": "pnpm", "yarn.lock": "yarn", "bun.lockb": "bun", "bun.lock": "bun"} {
			if _, err := os.Stat(filepath.Join(d, lock)); err == nil {
				return pm
			}
		}
		if d == root || len(d) <= len(root) {
			return "npm"
		}
		d = filepath.Dir(d)
	}
}
