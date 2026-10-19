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
	p := &core.Project{Name: "Klinik", Story: "Aplikasi klinik", Dir: dir,
		Stack: &core.TechStack{Profile: "saas", Decisions: []core.TechDecision{
			{Area: core.TechBackend, Recommended: "NestJS + TypeScript", Reason: "API"},
		}},
	}
	work.CompleteBlueprint(p)
	if err := WriteDocs(dir, p); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"README.md", "architecture.md", "database.md", "api.md", "security.md",
		"storage.md", "performance.md", "workflow.md", "deployment.md", "adr/0001-stack.md",
		"foundry-profile.md", "requirements.md", "design-brief.md"} {
		if _, err := os.Stat(filepath.Join(dir, "docs", f)); err != nil {
			t.Errorf("%s: %v", f, err)
		}
	}
	for _, f := range []string{"FOUNDRY.md", "AGENTS.md", "CLAUDE.md"} {
		b, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			t.Errorf("%s: %v", f, err)
			continue
		}
		if !strings.Contains(string(b), "FOUNDRY") {
			t.Errorf("%s missing FOUNDRY", f)
		}
	}
}

func TestWriteDocsIncludesPagesAndSpecJSON(t *testing.T) {
	dir := t.TempDir()
	p := &core.Project{Name: "Klinik", Dir: dir, Design: &core.Design{Pages: []core.Page{
		{ID: "PAGE-001", Name: "Antrean", Route: "/antrean", Platform: "web", Purpose: "Lihat antrean"}}}}
	if err := WriteDocs(dir, p); err != nil {
		t.Fatal(err)
	}
	pages, err := os.ReadFile(filepath.Join(dir, "docs", "pages.md"))
	if err != nil || !strings.Contains(string(pages), "/antrean") {
		t.Fatalf("pages.md = %s, %v", pages, err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "docs", "spec.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got Spec
	if err := json.Unmarshal(raw, &got); err != nil || len(got.Pages) != 1 || got.Pages[0].PageID != "PAGE-001" {
		t.Fatalf("spec.json = %s, %v", raw, err)
	}
}
