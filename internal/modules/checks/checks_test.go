package checks

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/handrawanw/foundry/internal/core"
)

// A background process left by a check keeps the output pipe open; the run
// must still return, and pass on the command's own exit code.
func TestRunNotHeldByBackgroundChild(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh tidak ada")
	}
	start := time.Now()
	b := Run(context.Background(), t.TempDir(), []core.Command{{Name: "test", Argv: []string{"sh", "-c", "echo ok; sleep 60 &"}}}, "")
	if d := time.Since(start); d > 20*time.Second {
		t.Fatalf("Run tertahan %s", d)
	}
	if !b.Passed || b.Steps[0].Output != "ok\n" {
		t.Fatalf("build = %+v", b)
	}
}

func TestInstallsWhenNodeModulesMissingOrStale(t *testing.T) {
	dir := t.TempDir()
	web := filepath.Join(dir, "web")
	os.MkdirAll(web, 0o755)
	os.WriteFile(filepath.Join(web, "package.json"), []byte(`{}`), 0o644)
	cmds := []core.Command{{Name: "typecheck", Dir: "web", Argv: []string{"npm", "run", "typecheck"}}, {Name: "test", Dir: "web", Argv: []string{"npm", "test"}}, {Name: "vet", Dir: ".", Argv: []string{"go", "vet"}}}

	got := installs(dir, cmds)
	if len(got) != 1 || got[0].Dir != "web" || strings.Join(got[0].Argv, " ") != "npm install --no-audit --no-fund" {
		t.Fatalf("missing node_modules: got %+v", got)
	}

	os.WriteFile(filepath.Join(web, "package-lock.json"), []byte(`{}`), 0o644)
	os.MkdirAll(filepath.Join(web, "node_modules"), 0o755)
	marker := filepath.Join(web, "node_modules", ".package-lock.json")
	os.WriteFile(marker, []byte(`{}`), 0o644)
	future := time.Now().Add(time.Hour)
	os.Chtimes(marker, future, future)
	if got := installs(dir, cmds); len(got) != 0 {
		t.Fatalf("fresh node_modules: got %+v", got)
	}

	later := future.Add(time.Minute)
	os.Chtimes(filepath.Join(web, "package-lock.json"), later, later)
	got = installs(dir, cmds)
	if len(got) != 1 || strings.Join(got[0].Argv, " ") != "npm ci --no-audit --no-fund" {
		t.Fatalf("stale lockfile: got %+v", got)
	}
}

// A dependency added to package.json after the lockfile must not use npm ci,
// which refuses an out-of-sync lockfile.
func TestInstallUpdatesLockfileWhenManifestNewer(t *testing.T) {
	dir := t.TempDir()
	lock := filepath.Join(dir, "package-lock.json")
	os.WriteFile(lock, []byte(`{}`), 0o644)
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{}`), 0o644)
	old := time.Now().Add(-time.Hour)
	os.Chtimes(lock, old, old)
	got := installs(dir, []core.Command{{Name: "test", Argv: []string{"npm", "test"}}})
	if len(got) != 1 || strings.Join(got[0].Argv, " ") != "npm install --no-audit --no-fund" {
		t.Fatalf("got %+v", got)
	}
}

func TestInstallsBrowserForPlaywrightE2E(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"devDependencies":{"@playwright/test":"^1"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cmds := []core.Command{{Name: "test:e2e", Argv: []string{"npm", "run", "test:e2e"}}}
	var names []string
	for _, c := range installs(dir, cmds) {
		names = append(names, c.Name)
	}
	if strings.Join(names, ",") != "install,install-browser" {
		t.Fatalf("installs = %v", names)
	}
	if got := installs(dir, []core.Command{{Name: "test", Argv: []string{"npm", "test"}}}); len(got) != 1 {
		t.Fatalf("no e2e script must not install a browser: %v", got)
	}
}

// A Go module whose go.sum is missing or behind go.mod / the sources gets one
// `go mod tidy`: agents cannot fetch modules, and go vet fails on every
// attempt otherwise.
func TestInstallsGoModules(t *testing.T) {
	dir := t.TempDir()
	cmds := []core.Command{{Name: "vet", Dir: ".", Argv: []string{"go", "vet", "./..."}}, {Name: "test", Dir: ".", Argv: []string{"go", "test", "./..."}}}
	names := func() string {
		var n []string
		for _, c := range installs(dir, cmds) {
			n = append(n, strings.Join(c.Argv, " "))
		}
		return strings.Join(n, ",")
	}
	mod := filepath.Join(dir, "go.mod")
	os.WriteFile(mod, []byte("module x\n\ngo 1.22\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println() }\n"), 0o644)
	if got := names(); got != "" {
		t.Fatalf("stdlib-only module must not tidy: %q", got)
	}
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nimport (\n\t\"github.com/gin-gonic/gin\"\n)\n\nfunc main() { _ = gin.New }\n"), 0o644)
	if got := names(); got != "go mod tidy -go=1.22" {
		t.Fatalf("missing go.sum with third-party import: %q", got)
	}
	sum := filepath.Join(dir, "go.sum")
	os.WriteFile(sum, []byte(""), 0o644)
	future := time.Now().Add(time.Hour)
	os.Chtimes(sum, future, future)
	if got := names(); got != "" {
		t.Fatalf("fresh go.sum: %q", got)
	}
	later := future.Add(time.Minute)
	os.Chtimes(filepath.Join(dir, "main.go"), later, later)
	if got := names(); got != "go mod tidy -go=1.22" {
		t.Fatalf("source newer than go.sum: %q", got)
	}
	if got := installs(dir, cmds[:1]); len(got) != 1 {
		t.Fatalf("one tidy per module dir: %+v", got)
	}
}

// Imports of the module's own packages need no fetch, and a commented
// "require" is not a directive: neither schedules a tidy on every run.
func TestGoInstallsSkipsOwnModuleImports(t *testing.T) {
	dir := t.TempDir()
	cmds := []core.Command{{Name: "vet", Dir: ".", Argv: []string{"go", "vet", "./..."}}}
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module github.com/me/app\n\n// require nothing yet\ngo 1.22\n"), 0o644)
	os.MkdirAll(filepath.Join(dir, "internal", "x"), 0o755)
	os.WriteFile(filepath.Join(dir, "internal", "x", "x.go"), []byte("package x\n\nfunc F() {}\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nimport (\n\t\"fmt\"\n\n\t\"github.com/me/app/internal/x\"\n)\n\nfunc main() { fmt.Println(); x.F() }\n"), 0o644)
	if got := installs(dir, cmds); len(got) != 0 {
		t.Fatalf("own-module imports scheduled %+v", got)
	}
	os.WriteFile(filepath.Join(dir, "dep.go"), []byte("package main\n\nimport _ \"github.com/me/appx/y\"\n"), 0o644)
	if got := installs(dir, cmds); len(got) != 1 {
		t.Fatalf("a sibling module with a shared prefix is third-party: %+v", got)
	}
}

// tidy is pinned to the declared go version so it never raises it; without a
// go directive it runs plain.
func TestGoInstallsPinsGoDirective(t *testing.T) {
	dir := t.TempDir()
	cmds := []core.Command{{Name: "vet", Dir: ".", Argv: []string{"go", "vet", "./..."}}}
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n\ngo 1.22.5\n\nrequire github.com/a/b v1.0.0\n"), 0o644)
	if got := installs(dir, cmds); len(got) != 1 || strings.Join(got[0].Argv, " ") != "go mod tidy -go=1.22.5" {
		t.Fatalf("pinned: %+v", got)
	}
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n\nrequire github.com/a/b v1.0.0\n"), 0o644)
	if got := installs(dir, cmds); len(got) != 1 || strings.Join(got[0].Argv, " ") != "go mod tidy" {
		t.Fatalf("no directive: %+v", got)
	}
}
