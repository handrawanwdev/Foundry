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
