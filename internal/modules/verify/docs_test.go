package verify

import (
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
		"storage.md", "performance.md", "workflow.md", "deployment.md", "adr/0001-stack.md"} {
		if _, err := os.Stat(filepath.Join(dir, "docs", f)); err != nil {
			t.Errorf("%s: %v", f, err)
		}
	}
}

func TestWriteDocsIncludesPRD(t *testing.T) {
	dir := t.TempDir()
	p := &core.Project{Name: "Klinik", Dir: dir, PRD: &core.PRD{Markdown: "# 1. Product Overview\nKlinik\n"}}
	if err := WriteDocs(dir, p); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "docs", "PRD.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "Product Overview") {
		t.Fatalf("PRD.md = %s", b)
	}
}
