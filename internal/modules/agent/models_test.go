package agent

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

// fakeCLI writes a script that prints out and returns a preset running it.
func fakeCLI(t *testing.T, id, out string) Preset {
	if runtime.GOOS == "windows" {
		t.Skip("shell script")
	}
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "out"), []byte(out), 0o644)
	bin := filepath.Join(dir, "cli")
	os.WriteFile(bin, []byte("#!/bin/sh\ncat "+filepath.Join(dir, "out")+"\n"), 0o755)
	return Preset{ID: id, Binary: bin}
}

func TestGrokModels(t *testing.T) {
	p := fakeCLI(t, "grok-test", "You are not authenticated.\n\nDefault model: grok-9\n\nAvailable models:\n  * grok-9 (default)\n  - grok-8\n")
	got, err := grokModels(context.Background(), p)
	if err != nil || !slices.Equal(got, []string{"grok-9", "grok-8"}) {
		t.Fatalf("got %v %v", got, err)
	}
}

func TestCodexModelsKeepsListedByPriority(t *testing.T) {
	p := fakeCLI(t, "codex-test", `{"models":[{"slug":"b","visibility":"list","priority":5},{"slug":"hidden","visibility":"hide","priority":1},{"slug":"a","visibility":"list","priority":2}]}`)
	got, err := codexModels(context.Background(), p)
	if err != nil || !slices.Equal(got, []string{"a", "b"}) {
		t.Fatalf("got %v %v", got, err)
	}
}

func TestListModelsMergesSuggestionsAndFallsBack(t *testing.T) {
	p := fakeCLI(t, "merge-test", "  * new-1\n  - old\n")
	p.Models, p.listModels = []string{"old", "alias"}, grokModels
	if got := p.ListModels(context.Background()); !slices.Equal(got, []string{"new-1", "old", "alias"}) {
		t.Fatalf("got %v", got)
	}
	broken := Preset{ID: "broken-test", Binary: "/nonexistent", Models: []string{"x"}, listModels: grokModels}
	if got := broken.ListModels(context.Background()); !slices.Equal(got, []string{"x"}) {
		t.Fatalf("fallback got %v", got)
	}
}

func TestClaudeCLIModelsReadsVersionedIDs(t *testing.T) {
	p := fakeCLI(t, "claude-test", "")
	os.WriteFile(p.Binary, []byte(`x"claude-opus-4-8" "claude-opus-5-5"; claude-sonnet-5-5,`+
		`"claude-haiku-4-5-20251001" "claude-opus-4-6-v1" claude-haiku-3-55 "claude-opus-5" "claude-opus-5-5"`), 0o755)
	got, err := claudeCLIModels(context.Background(), p)
	if err != nil || !slices.Equal(got, []string{"claude-opus-5-5", "claude-sonnet-5-5", "claude-opus-4-8"}) {
		t.Fatalf("got %v %v", got, err)
	}
}

func TestOpenCodeModelsParsesList(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "opencode")
	os.WriteFile(bin, []byte("#!/bin/sh\nprintf 'opencode/a-free\\n\\nopencode/b\\n'\n"), 0o755)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	p, _ := PresetByID("opencode")
	got, err := opencodeModels(context.Background(), p)
	if err != nil || !slices.Equal(got, []string{"opencode/a-free", "opencode/b"}) {
		t.Fatalf("got %v, %v", got, err)
	}
}
