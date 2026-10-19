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

func TestDetectsComponents(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "go.mod"), "module x\n")
	write(t, filepath.Join(dir, "web", "package.json"), `{"dependencies":{"react":"18"}}`)
	r := Scan(dir)
	kinds := map[string]bool{}
	for _, c := range r.Components {
		kinds[c.Kind+" "+c.Tech] = true
	}
	if !r.Existing || !kinds["backend Go"] || !kinds["web React"] {
		t.Fatalf("scan = %+v", r)
	}
}
