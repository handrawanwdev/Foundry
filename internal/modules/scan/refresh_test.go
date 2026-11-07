package scan

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
)

func TestRefreshKeepsUnchangedAndRecordsDelta(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "go.mod"), "module example.com/app\n\ngo 1.22\n")
	write(t, filepath.Join(dir, "internal", "billing", "service.go"), "package billing\n")
	first := Context(dir, Scan(dir).Components)
	if first == nil || first.Fingerprint == "" || !hasModule(first.Modules, "internal/billing") {
		t.Fatalf("first %+v", first)
	}
	same := Refresh(dir, first, Scan(dir).Components)
	if same.Fingerprint != first.Fingerprint || len(same.Changes) != 0 || same.Stale {
		t.Fatalf("unchanged rescan %+v", same)
	}
	write(t, filepath.Join(dir, "internal", "auth", "jwt.go"), "package auth\n")
	second := Refresh(dir, first, Scan(dir).Components)
	if second.Fingerprint == first.Fingerprint {
		t.Fatal("fingerprint did not change")
	}
	if !hasModule(second.Modules, "internal/billing") || !hasModule(second.Modules, "internal/auth") {
		t.Fatalf("modules %+v", second.Modules)
	}
	if !hasChange(second.Changes, core.ContextAreaModules, "internal/auth", core.ChangeAdd) {
		t.Fatalf("changes %+v", second.Changes)
	}
	if second.Corrections != first.Corrections {
		t.Fatal("corrections dropped")
	}
}

func TestMarkStaleWhenRepoChanges(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "go.mod"), "module example.com/app\n")
	write(t, filepath.Join(dir, "internal", "billing", "service.go"), "package billing\n")
	c := Context(dir, Scan(dir).Components)
	MarkStale(dir, c)
	if c.Stale {
		t.Fatal("fresh context marked stale")
	}
	write(t, filepath.Join(dir, "README.md"), "# App\n")
	MarkStale(dir, c)
	if !c.Stale || !contains(c.StaleAreas, core.ContextAreaDocs) {
		t.Fatalf("stale=%v areas=%v", c.Stale, c.StaleAreas)
	}
	if !hasModule(c.Modules, "internal/billing") {
		t.Fatal("stale mark rewrote findings")
	}
}

func TestRefreshPreservesCorrections(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "go.mod"), "module x\n")
	c := Context(dir, Scan(dir).Components)
	c.Corrections = "modul billing sudah ada"
	write(t, filepath.Join(dir, "internal", "billing", "ok.go"), "package billing\n")
	got := Refresh(dir, c, Scan(dir).Components)
	if got.Corrections != "modul billing sudah ada" {
		t.Fatalf("corrections %q", got.Corrections)
	}
}

func TestIncompleteDocsStillEvidenceBased(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "go.mod"), "module example.com/app\n")
	write(t, filepath.Join(dir, "internal", "orders", "svc.go"), "package orders\n")
	c := Context(dir, Scan(dir).Components)
	if c == nil || hasFinding(c.Docs, "orders") {
		t.Fatalf("invented docs %+v", c)
	}
	if !hasModule(c.Modules, "internal/orders") {
		t.Fatalf("modules %+v", c.Modules)
	}
	if len(c.Features) != 0 {
		t.Fatalf("invented features %+v", c.Features)
	}
}

func hasChange(cs []core.ContextChange, area, path, action string) bool {
	for _, c := range cs {
		if c.Area == area && c.Path == path && c.Action == action {
			return true
		}
	}
	return false
}

func TestRefreshEmptyDirNil(t *testing.T) {
	if c := Refresh(t.TempDir(), nil, nil); c != nil {
		t.Fatalf("got %+v", c)
	}
}

func TestFingerprintIgnoresUnrelatedDotfiles(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "go.mod"), "module x\n")
	a := areaHashes(dir)
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("SECRET=1"), 0o644); err != nil {
		t.Fatal(err)
	}
	b := areaHashes(dir)
	if fingerprintOf(a) != fingerprintOf(b) {
		t.Fatal("dotfile changed fingerprint")
	}
}
