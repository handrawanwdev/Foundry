package work

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/handrawanw/foundry/internal/core"
)

var testSystem = &core.DesignSystem{Colors: []core.Token{{Name: "primary", Value: "#1f6feb"}},
	Typography: []core.Token{{Name: "body", Value: "Inter 14px"}}, Spacing: []core.Token{{Name: "space-2", Value: "8px"}}}

func blockerTexts(p *core.Project) []string {
	var out []string
	for _, b := range Blockers(p) {
		out = append(out, b.PageID+" "+b.Text)
	}
	return out
}

func TestBlockersScreenIntent(t *testing.T) {
	p := &core.Project{Design: &core.Design{Pages: []core.Page{
		{ID: "PAGE-001", Name: "Masuk", Kind: "auth", Route: "/masuk", Access: "public", Hierarchy: []string{"Form"},
			PrimaryAction: &core.PageAction{Label: "Masuk"},
			Components:    []core.PageComponent{{Type: "form", Label: "Login"}, {Type: "button", Label: "masuk "}}},
		{ID: "PAGE-002", Name: "Janji", Kind: "list", Route: "/janji", Access: "auth", Hierarchy: []string{"Tabel"},
			PrimaryAction: &core.PageAction{Label: "Buat"}, States: &core.PageStates{Empty: "Belum ada janji."},
			Components: []core.PageComponent{{Type: "table", Label: "Janji", Columns: []string{"Pasien"}}}},
		{ID: "PAGE-003", Name: "Detail", Kind: "detail", Route: "/janji/:id", Access: "auth",
			PrimaryAction: &core.PageAction{Reason: "baca saja"},
			Components:    []core.PageComponent{{Type: "list", Label: "Info"}}},
	}}}
	got := blockerTexts(p)
	want := []string{
		" design system (warna, tipografi, spacing) belum ditentukan",
		"PAGE-001 aksi “Masuk” tampil lebih dari sekali",
		"PAGE-002 kondisi memuat, error belum ditentukan",
		"PAGE-003 urutan informasi (apa yang dilihat user lebih dulu) belum ditentukan",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("blockers =\n%s", strings.Join(got, "\n"))
	}
	p.Design.System = testSystem
	if got := blockerTexts(p); slices.Contains(got, want[0]) {
		t.Fatal("system blocker must go once a system exists")
	}
}

func TestNormSystemNeedsCoreTokens(t *testing.T) {
	if normSystem(&core.DesignSystem{Colors: testSystem.Colors, Typography: testSystem.Typography}) != nil {
		t.Fatal("system without spacing must be dropped")
	}
	s := normSystem(&core.DesignSystem{Colors: append([]core.Token{{Name: " ", Value: "#000"}}, testSystem.Colors...),
		Typography: testSystem.Typography, Spacing: testSystem.Spacing})
	if s == nil || len(s.Colors) != 1 {
		t.Fatalf("system = %+v", s)
	}
}

func TestRevisionSystemScopeAndUndo(t *testing.T) {
	sys := `{"system": {"colors": [{"name": "primary", "value": "#0a7"}], "typography": [{"name": "body", "value": "Inter"}], "spacing": [{"name": "s", "value": "8px"}]}}`

	p := revisionProject()
	p.Design.System = testSystem
	p.Design.Scope = &core.ChangeScope{Pages: []core.ScopePage{{ID: "PAGE-001"}}}
	var rej *PatchRejected
	if err := ApplyDesign(p, sys); !errors.As(err, &rej) {
		t.Fatalf("scoped system change err = %v", err)
	}

	p = revisionProject()
	p.Design.System = testSystem
	if err := ApplyDesign(p, sys); err != nil {
		t.Fatal(err)
	}
	if p.Design.System.Colors[0].Value != "#0a7" || !slices.Contains(p.Design.Revision.Patch, "UPDATE design system") {
		t.Fatalf("design = %+v", p.Design)
	}
	if err := UndoRevision(p); err != nil {
		t.Fatal(err)
	}
	if p.Design.System != testSystem {
		t.Fatalf("undo system = %+v", p.Design.System)
	}
}

func TestApplyQAIssues(t *testing.T) {
	p := revisionProject()
	p.Design.Revision = &core.DesignRevision{QA: &core.DesignQA{Status: "running", Round: 1}}
	again, err := ApplyQA(p, `{"status": "pass", "issues": [
		{"page": "PAGE-001", "severity": "MAJOR", "problem": "Dua tombol utama", "why": "maks. satu CTA", "impact": "user ragu", "fix": "jadikan sekunder"},
		{"page": "PAGE-404", "severity": "?", "problem": "Teks terpotong"}, {"problem": " "}]}`)
	if err != nil || again {
		t.Fatalf("again=%v err=%v", again, err)
	}
	is := p.Design.Revision.QA.Issues
	if len(is) != 2 || is[0].Severity != "major" || is[0].PageID != "PAGE-001" || is[1].Severity != "minor" || is[1].PageID != "" {
		t.Fatalf("issues = %+v", is)
	}
}

func TestTaskPromptUIRules(t *testing.T) {
	p := opsProject()
	p.Design.System = testSystem
	p.Design.Pages[0].Hierarchy = []string{"Tabel janji"}
	ui := &core.Task{ID: "TASK-002", Area: "ui", PageIDs: []string{"PAGE-001"}}
	got := TaskPrompt(p, ui)
	for _, want := range []string{"UI RULES:", "DESIGN SYSTEM:", `"hierarchy":[`, "Tabel janji", "At most one primary CTA", "primary=#1f6feb"} {
		if !strings.Contains(got, want) {
			t.Errorf("ui prompt missing %q", want)
		}
	}
	if strings.Contains(got, `"label":`) || strings.Contains(got, `"colors":[`) {
		t.Error("ui prompt dumped raw page components or design-system JSON")
	}
	if got := TaskPrompt(p, &core.Task{ID: "TASK-001", Area: "api"}); strings.Contains(got, "UI RULES:") {
		t.Error("api task must not get UI rules")
	}
}
