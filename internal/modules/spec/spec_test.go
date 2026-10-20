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

func sampleUI() *core.UIDesign {
	return &core.UIDesign{
		Rules: &core.DesignRules{Source: core.RulesNew, Principles: []string{"Tabel padat untuk operator"},
			UX: []string{"Konfirmasi aksi hapus"}, Accessibility: []string{"Kontras AA"}, Responsive: []string{"Tabel jadi list di mobile"},
			Tokens: core.DesignTokens{Colors: []core.Token{{Name: "action.primary", Value: "#1f6feb"}},
				Typography: []core.Token{{Name: "type.body", Value: "Inter 14px"}}, Spacing: []core.Token{{Name: "space.md", Value: "16px"}}},
			Components: []core.ComponentRule{{Name: "DataTable", Use: "daftar data"}}},
		Layout: &core.LayoutSpec{Pages: []core.LayoutPage{{ID: "antrean", Name: "Antrean", Platform: "web", Route: "/antrean",
			Purpose: "Lihat antrean", Pattern: "table", States: map[string]string{"loading": "skeleton", "empty": "belum ada", "error": "gagal"},
			Sections: []core.Section{{ID: "tabel", Name: "Tabel", Content: []string{"No, Pasien, Status"}, Components: []string{"DataTable"}}}}}},
	}
}

func TestWriteDocsIncludesDesignAndSpecJSON(t *testing.T) {
	dir := t.TempDir()
	p := &core.Project{Name: "Klinik", Dir: dir, UI: sampleUI(), Flows: []core.Flow{{ID: "FLOW-001", Name: "Cek", PageIDs: []string{"antrean"}}}}
	root := filepath.Join(dir, "docs", "foundry")
	// Files of an older export with the visual design workspace.
	for _, f := range []string{"design-brief.md", "blueprint/ui-design.md"} {
		os.MkdirAll(filepath.Dir(filepath.Join(root, f)), 0o755)
		os.WriteFile(filepath.Join(root, f), []byte("lama"), 0o644)
	}
	if err := WriteDocs(dir, p); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"design-brief.md", "blueprint/ui-design.md"} {
		if _, err := os.Stat(filepath.Join(root, f)); !os.IsNotExist(err) {
			t.Errorf("%s should be removed: %v", f, err)
		}
	}
	layout, err := os.ReadFile(filepath.Join(root, "blueprint", "layout-spec.md"))
	if err != nil || !strings.Contains(string(layout), "/antrean") || !strings.Contains(string(layout), "Komponen: DataTable") ||
		!strings.Contains(string(layout), "- empty: belum ada") || !strings.Contains(string(layout), "FLOW-001 Cek") {
		t.Fatalf("layout-spec.md = %s, %v", layout, err)
	}
	rules, err := os.ReadFile(filepath.Join(root, "blueprint", "design-rules.md"))
	if err != nil || !strings.Contains(string(rules), "`action.primary`") || !strings.Contains(string(rules), "### DataTable") {
		t.Fatalf("design-rules.md = %s, %v", rules, err)
	}
	var lj struct {
		Pages []core.LayoutPage `json:"pages"`
		Flows []core.Flow       `json:"flows"`
	}
	raw, _ := os.ReadFile(filepath.Join(root, "design", "layout-spec.json"))
	if err := json.Unmarshal(raw, &lj); err != nil || len(lj.Pages) != 1 || len(lj.Flows) != 1 {
		t.Fatalf("layout-spec.json = %s, %v", raw, err)
	}
	if _, err := os.Stat(filepath.Join(root, "design", "design-rules.json")); err != nil {
		t.Error(err)
	}
	raw, err = os.ReadFile(filepath.Join(root, "spec.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got Spec
	if err := json.Unmarshal(raw, &got); err != nil || got.Layout == nil || got.Layout.Pages[0].ID != "antrean" || got.DesignRules == nil {
		t.Fatalf("spec.json = %s, %v", raw, err)
	}
	// The design is gone: its JSON files go with it.
	p.UI = nil
	if err := WriteDocs(dir, p); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "design", "layout-spec.json")); !os.IsNotExist(err) {
		t.Errorf("layout-spec.json kept: %v", err)
	}
}

// Artifact hashes only change with their content, so commands are not
// reported stale by rendering alone.
func TestArtifactsStable(t *testing.T) {
	p := &core.Project{Name: "Klinik", Story: "x", Discovery: &core.Discovery{Summary: "Klinik"},
		Requirements: []core.Requirement{{ID: "REQ-001", Text: "Buat janji", Priority: "MUST"}}, UI: sampleUI()}
	work.CompleteBlueprint(p)
	a, b := Artifacts(p), Artifacts(p)
	if len(a) != 4 || BlueprintHash(a) != BlueprintHash(b) {
		t.Fatalf("artifacts = %d, hashes %s %s", len(a), BlueprintHash(a), BlueprintHash(b))
	}
	hashes := map[string]string{}
	for _, d := range a {
		hashes[d.ID] = d.Hash()
	}
	p.UI.Layout.Pages[0].Sections[0].Spacing = "space.lg"
	for _, d := range Artifacts(p) {
		if changed := d.Hash() != hashes[d.ID]; changed != (d.ID == core.ArtifactLayoutSpec) {
			t.Errorf("%s changed = %v", d.ID, changed)
		}
	}
	p.UI.Rules.Tokens.Spacing[0].Value = "12px"
	for _, d := range Artifacts(p) {
		if d.ID == core.ArtifactDesignRules && d.Hash() == hashes[d.ID] {
			t.Error("rules change did not change design-rules hash")
		}
	}
}
