package safefile

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteFileReplaces(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.json")
	for _, body := range []string{"one", "two"} {
		if err := WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if got, _ := os.ReadFile(p); string(got) != body {
			t.Fatalf("got %q want %q", got, body)
		}
	}
	if m, _ := filepath.Glob(p + ".*.tmp"); len(m) != 0 {
		t.Fatalf("temp files left: %v", m)
	}
}

func TestCleanTemp(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "project.json")
	old := p + ".111.tmp"
	fresh := p + ".222.tmp"
	other := filepath.Join(dir, "other.json.333.tmp")
	for _, f := range []string{old, fresh, other} {
		os.WriteFile(f, []byte("x"), 0o644)
	}
	past := time.Now().Add(-2 * time.Hour)
	os.Chtimes(old, past, past)
	os.Chtimes(other, past, past)
	CleanTemp(p, time.Hour)
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatal("old temp file should be removed")
	}
	for _, f := range []string{fresh, other} {
		if _, err := os.Stat(f); err != nil {
			t.Fatalf("%s must stay: %v", f, err)
		}
	}
}
