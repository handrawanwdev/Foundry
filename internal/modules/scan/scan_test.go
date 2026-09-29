package scan

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDelegatingRootScriptSkipped(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "package.json"), `{"scripts":{"build":"pnpm -r --filter \"@x/*\" run build","test":"pnpm -r run test","lint":"eslint ."}}`)
	write(t, filepath.Join(dir, "pnpm-lock.yaml"), "")
	write(t, filepath.Join(dir, "apps", "web", "package.json"), `{"scripts":{"build":"vite build","test":"vitest run"}}`)

	got := map[string]bool{}
	for _, c := range Scan(dir).Commands {
		got[c.Dir+" "+c.Name] = true
	}
	want := map[string]bool{
		". lint":         true, // own work at root, kept
		"apps/web build": true,
		"apps/web test":  true,
	}
	if len(got) != len(want) {
		t.Fatalf("commands = %v", got)
	}
	for k := range want {
		if !got[filepath.FromSlash(k)] {
			t.Fatalf("missing %q in %v", k, got)
		}
	}
}

func TestRootScriptKeptWithoutPackages(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "package.json"), `{"scripts":{"test":"turbo run test"}}`)
	if cmds := Scan(dir).Commands; len(cmds) != 1 {
		t.Fatalf("commands = %v", cmds)
	}
}

func TestIsDelegating(t *testing.T) {
	for script, want := range map[string]bool{
		"pnpm -r run build":          true,
		"npm run build --workspaces": true,
		"pnpm --filter=api test":     true,
		"turbo run test":             true,
		"vite build":                 false,
		"node scripts/run-r.js":      false,
	} {
		if got := isDelegating(script); got != want {
			t.Errorf("isDelegating(%q) = %v", script, got)
		}
	}
}
