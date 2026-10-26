package work

import (
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
)

func uiProject() *core.Project {
	return &core.Project{Name: "Antrean", Story: "Aplikasi antrean klinik", Platform: &core.PlatformPlan{Targets: []core.PlatformTarget{{Kind: core.PlatformWeb}}},
		Discovery: &core.Discovery{Users: []string{"Operator", "Supervisor"},
			Roles: []core.Role{{Name: "Operator", Platform: "web", Description: "memanggil antrean"}},
			Data:  []string{"Queue: id, nomor, status"}},
		Features: []core.Feature{{ID: "FEAT-001", Name: "Antrean", Platforms: []string{"web"}, RequirementIDs: []string{"REQ-001"}}},
		Flows:    []core.Flow{{ID: "FLOW-001", Name: "Panggil", FeatureID: "FEAT-001", Role: "Operator", Goal: "Memanggil pasien", Steps: []string{"Lihat", "Panggil"}, Rules: []string{"Hanya nomor berikutnya"}, Permissions: []string{"queue:call"}, Data: []string{"Queue: status berubah"}}},
		Blueprint: &core.Blueprint{Database: core.DatabaseArch{Entities: []core.Entity{{Name: "Queue", Fields: []string{"id", "nomor", "status"}}}},
			Authz: core.AuthzArch{Permissions: []string{"queue:call"}}},
		UI: &core.UIDesign{Format: core.UIDesignFormat, Rules: &core.DesignRules{Source: core.RulesNew},
			Layout: &core.LayoutSpec{Pages: []core.LayoutPage{{ID: "antrean", Name: "Antrean", Platform: "web", FeatureID: "FEAT-001",
				Purpose: "Panggil", Goal: "GOAL-1", Pattern: "table",
				Sections: []core.Section{{ID: "tabel", Name: "Tabel", Content: []string{"No"}}}}}}},
	}
}

func TestCheckUIRequiresEveryPageInACommand(t *testing.T) {
	p := uiProject()
	cmds := []core.Command{{ID: "CMD-001", Name: "API", Objective: "o", Instructions: "Bangun API antrean.",
		Artifacts: []core.ArtifactRef{{ID: core.ArtifactRequirements}}}}
	if issues := CheckUI(p, cmds); len(issues) != 1 || !strings.Contains(issues[0].Message, "antrean") {
		t.Fatalf("issues = %+v", issues)
	}
	cmds = append(cmds, core.Command{ID: "CMD-002", Name: "Layar", Objective: "o", Instructions: "Bangun layar antrean.",
		Artifacts: []core.ArtifactRef{{ID: core.ArtifactDesignRules}, {ID: core.ArtifactLayoutSpec}}})
	if issues := CheckUI(p, cmds); len(issues) != 0 {
		t.Fatalf("issues = %+v", issues)
	}
	// A design artifact without its pair is a broken reference.
	cmds[1].Artifacts = []core.ArtifactRef{{ID: core.ArtifactLayoutSpec}}
	if issues := CheckUI(p, cmds); len(issues) != 1 || !strings.Contains(issues[0].Message, "tidak keduanya") {
		t.Fatalf("issues = %+v", issues)
	}
}

func TestDesignPromptCarriesUXInputsAndMethod(t *testing.T) {
	p := uiProject()
	p.Stack = nil
	prompt := DesignPrompt(p)
	for _, want := range []string{"UX INPUTS", "UX INTELLIGENCE", "Queue", "queue:call", "Hanya nomor berikutnya",
		"Memanggil pasien", "not stated", "decisions", "navigation"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
}

func TestApplyDesignMergesPageDecisions(t *testing.T) {
	p := uiProject()
	p.UI = nil
	p.Features = append(p.Features, core.Feature{ID: "FEAT-002", Name: "Laporan", Platforms: []string{"web"}})
	first := `{"rules": {"source": "new", "principles": ["Padat"], "ux_guidelines": ["Filter"], "accessibility": ["AA"], "responsive": ["List"],
 "tokens": {"colors": [{"name": "action.primary", "value": "#111111"}], "typography": [{"name": "type.body", "value": "14px"}], "spacing": [{"name": "space.md", "value": "16px"}]}},
 "users": [{"role": "Operator", "platform": "web"}], "goals": [{"id": "GOAL-1", "role": "Operator", "text": "Memanggil pasien"}],
 "navigation": [{"name": "Utama", "items": [{"label": "Antrean", "page": "antrean"}, {"label": "Laporan", "page": "laporan"}]}],
 "decisions": [{"subject": "antrean", "choice": "table", "why": ["banyak baris"], "basis": ["GOAL-1"], "source": "fact"},
               {"subject": "laporan", "choice": "report", "why": ["ringkasan"], "basis": ["GOAL-1"], "source": "fact"}],
 "pages": [{"id": "antrean", "name": "Antrean", "platform": "web", "route": "/antrean", "purpose": "Panggil", "goal": "GOAL-1", "pattern": "table", "primary_action": "Panggil",
   "sections": [{"id": "tabel", "name": "Tabel", "content": ["No"]}], "states": {"loading": "x", "empty": "x", "error": "x"}},
  {"id": "laporan", "name": "Laporan", "platform": "web", "route": "/laporan", "purpose": "Lihat", "goal": "GOAL-1", "pattern": "report", "primary_action": "Ekspor",
   "sections": [{"id": "isi", "name": "Isi", "content": ["Ringkasan"]}], "states": {"loading": "x", "empty": "x", "error": "x"}}]}`
	if err := ApplyDesign(p, first); err != nil {
		t.Fatal(err)
	}
	revision := `{"pages": [{"id": "antrean", "name": "Antrean", "platform": "web", "route": "/antrean", "purpose": "Panggil", "goal": "GOAL-1", "pattern": "table", "primary_action": "Panggil",
 "sections": [{"id": "tabel", "name": "Tabel", "content": ["No", "Status"]}], "states": {"loading": "x", "empty": "x", "error": "x"}}],
 "decisions": [{"subject": "antrean", "choice": "table", "why": ["banyak baris dan status"], "basis": ["GOAL-1"], "source": "fact"}]}`
	if err := ApplyDesign(p, revision); err != nil {
		t.Fatal(err)
	}
	var subjects []string
	for _, d := range p.UI.Layout.Decisions {
		subjects = append(subjects, d.Subject)
	}
	if !strings.Contains(strings.Join(subjects, ","), "laporan") || len(p.UI.Layout.Decisions) != 2 {
		t.Fatalf("decisions = %+v", p.UI.Layout.Decisions)
	}
}
