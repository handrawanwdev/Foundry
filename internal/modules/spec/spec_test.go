package spec

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/handrawanw/foundry/internal/core"
	"github.com/handrawanw/foundry/internal/modules/work"
)

func TestMarkdownIncludesBlueprint(t *testing.T) {
	p := &core.Project{Name: "Klinik", Story: "Aplikasi klinik",
		Discovery: &core.Discovery{Summary: "Klinik", Business: "Pratama"},
		Blueprint: &core.Blueprint{
			Overview:     core.Overview{Goal: "Tanpa antrian kertas", CoreDomain: "appointment"},
			Architecture: core.ArchDecision{Pattern: "Modular Monolith", Reason: "beberapa domain"},
			Modules:      []core.Module{{ID: "identity", Responsibility: "Auth"}},
			Auth:         core.AuthArch{Method: "email + password", Hashing: "Argon2id"},
		},
	}
	md := Markdown(p)
	for _, want := range []string{"Project Overview", "Tanpa antrian kertas", "Modular Monolith", "`identity`", "Argon2id"} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q", want)
		}
	}
}

func TestWriteDocs(t *testing.T) {
	dir := t.TempDir()
	p := &core.Project{Name: "Klinik", Story: "Aplikasi klinik", Dir: dir, Stage: core.StageReady,
		Discovery:    &core.Discovery{Summary: "Klinik"},
		Requirements: []core.Requirement{{ID: "REQ-001", Text: "Buat janji", Priority: "MUST"}},
		Stack: &core.TechStack{Profile: "saas", Decisions: []core.TechDecision{
			{Area: core.TechBackend, Recommended: "NestJS + TypeScript", Reason: "API"},
		}},
		Artifacts: []core.Artifact{{ID: "threat-model", Title: "Threat Model", Category: core.ArtifactQuality, Body: "## Aset"}},
		Workflow: &core.Workflow{Version: 2, Commands: []core.Command{
			{ID: "CMD-001", Name: "Kontrak", Objective: "o", Instructions: "i", Artifacts: []core.ArtifactRef{{ID: "requirements"}}},
			{ID: "CMD-002", Name: "API", Objective: "o", Instructions: "i", DependsOn: []string{"CMD-001"}},
		}},
	}
	work.CompleteBlueprint(p)
	// A file of an earlier export that is no longer part of the package.
	old := filepath.Join(dir, "docs", "foundry", "workflow", "CMD-009.md")
	os.MkdirAll(filepath.Dir(old), 0o755)
	os.WriteFile(old, []byte("lama"), 0o644)
	if err := WriteDocs(dir, p); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(dir, "docs", "foundry")
	for _, f := range []string{"README.md", "manifest.json", "spec.json", "adr/0001-stack.md", "blueprint/requirements.md",
		"blueprint/architecture.md", "blueprint/technology.md", "blueprint/threat-model.md", "workflow/workflow.json",
		"workflow/CMD-001.md", "workflow/CMD-002.md"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(f))); err != nil {
			t.Errorf("%s: %v", f, err)
		}
	}
	for _, f := range []string{"blueprint/ui-design.md", "design-brief.md", "workflow/CMD-009.md"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(f))); !os.IsNotExist(err) {
			t.Errorf("%s should not exist: %v", f, err)
		}
	}
	readme, _ := os.ReadFile(filepath.Join(root, "README.md"))
	if !strings.Contains(string(readme), "**Gelombang 2**") || !strings.Contains(string(readme), "coding agent eksternal") {
		t.Errorf("README = %s", readme)
	}
	if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); err != nil {
		t.Error(err)
	}
	// An existing AGENTS.md is the project's own: it is never replaced.
	os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("milik user"), 0o644)
	if err := WriteDocs(dir, p); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md")); string(b) != "milik user" {
		t.Errorf("AGENTS.md replaced: %s", b)
	}
}

func TestWriteDocsIncludesPagesAndSpecJSON(t *testing.T) {
	dir := t.TempDir()
	p := &core.Project{Name: "Klinik", Dir: dir, Design: &core.Design{Pages: []core.Page{
		{ID: "PAGE-001", Name: "Antrean", Route: "/antrean", Platform: "web", Purpose: "Lihat antrean"}}}}
	if err := WriteDocs(dir, p); err != nil {
		t.Fatal(err)
	}
	pages, err := os.ReadFile(filepath.Join(dir, "docs", "foundry", "blueprint", "ui-design.md"))
	if err != nil || !strings.Contains(string(pages), "/antrean") {
		t.Fatalf("ui-design.md = %s, %v", pages, err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "docs", "foundry", "spec.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got Spec
	if err := json.Unmarshal(raw, &got); err != nil || len(got.Pages) != 1 || got.Pages[0].PageID != "PAGE-001" {
		t.Fatalf("spec.json = %s, %v", raw, err)
	}
}

// Artifact hashes only change with their content, so commands are not
// reported stale by rendering alone.
func TestArtifactsStable(t *testing.T) {
	p := &core.Project{Name: "Klinik", Story: "x", Discovery: &core.Discovery{Summary: "Klinik"},
		Requirements: []core.Requirement{{ID: "REQ-001", Text: "Buat janji", Priority: "MUST"}},
		Design:       &core.Design{Pages: []core.Page{{ID: "PAGE-001", Name: "Antrean", Route: "/antrean", Platform: "web"}}}}
	work.CompleteBlueprint(p)
	a, b := Artifacts(p), Artifacts(p)
	if len(a) != 3 || BlueprintHash(a) != BlueprintHash(b) {
		t.Fatalf("artifacts = %d, hashes %s %s", len(a), BlueprintHash(a), BlueprintHash(b))
	}
	p.Design.Pages[0].Name = "Antrian"
	if BlueprintHash(Artifacts(p)) == BlueprintHash(a) {
		t.Fatal("design change did not change the blueprint hash")
	}
}
