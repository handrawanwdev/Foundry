package work

import (
	"slices"
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/spec"
	"github.com/handrawanwdev/foundry/internal/modules/workflow"
)

// mockupProject has two features with a page each, plus a page without a
// feature; its ids are common words ("analisis") that instructions use.
func mockupProject() *core.Project {
	p := uiProject()
	p.Features = append(p.Features, core.Feature{ID: "FEAT-002", Name: "Analisis", Platforms: []string{"web"}})
	p.UI.Layout.Pages = []core.LayoutPage{
		{ID: "antrean", Name: "Antrean", Platform: "web", Pattern: "table", FeatureID: "FEAT-001",
			Sections: []core.Section{{ID: "tabel", Name: "Tabel antrean", Content: []string{"Nomor", "Status"}, Components: []string{"DataTable"}}}},
		{ID: "analisis", Name: "Analisis", Platform: "web", Pattern: "table", FeatureID: "FEAT-002",
			Sections: []core.Section{{ID: "grafik", Name: "Grafik harian", Content: []string{"Jumlah pasien per jam"}, Components: []string{"LineChart"}}}},
		{ID: "pengaturan", Name: "Pengaturan", Platform: "web", Pattern: "form"},
	}
	return p
}

func TestSettleSketchPagesGivesEveryPageACommand(t *testing.T) {
	p := mockupProject()
	hashes := workflow.Hashes(spec.Artifacts(p))
	cmds := []core.Command{
		{ID: "CMD-001", Name: "API", Artifacts: []core.ArtifactRef{{ID: core.ArtifactRequirements}}},
		// A screen command without pages gets the pages of its feature.
		{ID: "CMD-002", Name: "Layar antrean", FeatureIDs: []string{"FEAT-001"}, Artifacts: design()},
		// Pages without the design artifacts get them, and the page's feature.
		{ID: "CMD-003", Name: "Layar analisis", Pages: []string{"analisis"}, Artifacts: []core.ArtifactRef{{ID: core.ArtifactRequirements}}},
	}
	if err := settleSketchPages(p, cmds, hashes); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(cmds[1].Pages, []string{"antrean"}) {
		t.Fatalf("CMD-002 pages = %v", cmds[1].Pages)
	}
	// The page nobody took goes to the last screen command.
	if !slices.Equal(cmds[2].Pages, []string{"analisis", "pengaturan"}) || !listsLayout(cmds[2]) ||
		!slices.Contains(cmds[2].FeatureIDs, "FEAT-002") {
		t.Fatalf("CMD-003 = %+v", cmds[2])
	}
	if len(cmds[0].Pages) != 0 {
		t.Fatalf("API command got pages %v", cmds[0].Pages)
	}
	// An unknown page id is refused.
	bad := []core.Command{{ID: "CMD-001", Pages: []string{"dasbor"}, Artifacts: design()}}
	if err := settleSketchPages(p, bad, hashes); err == nil || !strings.Contains(err.Error(), "dasbor") {
		t.Fatalf("err = %v", err)
	}
}

// A page id that is also a common word does not count as built because an
// instruction uses the word: the recorded pages decide.
func TestCheckUIReadsRecordedPages(t *testing.T) {
	p := mockupProject()
	cmds := []core.Command{
		{ID: "CMD-001", Instructions: "Bangun layar antrean dengan analisis per jam.", Pages: []string{"antrean"}, Artifacts: design()},
		{ID: "CMD-002", Instructions: "Bangun pengaturan.", Pages: []string{"pengaturan"}, Artifacts: design()},
	}
	issues := CheckUI(p, cmds)
	if len(issues) != 1 || !strings.Contains(issues[0].Message, "analisis") {
		t.Fatalf("issues = %+v", issues)
	}
	if got := UnbuiltPages(p, cmds); !slices.Equal(got, []string{"analisis"}) {
		t.Fatalf("unbuilt = %v", got)
	}
	// A page removed from the design is a sync issue of its command.
	cmds[1].Pages = []string{"pengaturan", "laporan"}
	if issues := CheckUI(p, cmds); !slices.ContainsFunc(issues, func(is workflow.Issue) bool {
		return is.Command == "CMD-002" && strings.Contains(is.Message, "laporan")
	}) {
		t.Fatalf("issues = %+v", issues)
	}
}

// The detail round of a screen command reads the whole mockup spec of its
// pages, however long layout-spec is.
func TestDetailPromptCarriesTheMockupOfItsPages(t *testing.T) {
	p := mockupProject()
	// A long design: the page sections would be cut away without the specs.
	for i := range p.UI.Layout.Pages {
		pg := &p.UI.Layout.Pages[i]
		for j := 0; j < 30; j++ {
			pg.Data = append(pg.Data, strings.Repeat("Catatan data panjang ", 8))
		}
	}
	p.Workflow = &core.Workflow{Commands: []core.Command{
		{ID: "CMD-001", Name: "Layar analisis", Objective: "o", Pages: []string{"analisis"}, Artifacts: design()},
		{ID: "CMD-002", Name: "Layar lain", Objective: "o", Pages: []string{"antrean", "pengaturan"}, Artifacts: design()},
	}}
	docs := spec.Artifacts(p)
	prompt := WorkflowPrompt(p, docs)
	at := strings.Index(prompt, "MOCKUP SPECS")
	if at < 0 || !strings.Contains(prompt[at:], "Grafik harian") || !strings.Contains(prompt[at:], "LineChart") {
		t.Fatalf("prompt misses the mockup of its page:\n%s", prompt)
	}
	// Other pages' mockups are not this round's.
	if strings.Contains(prompt, "Tabel antrean") {
		t.Fatal("prompt carries the mockup of another command's page")
	}
}

func TestSettlePagesKeepsTheOutlinePages(t *testing.T) {
	p := mockupProject()
	cmds := []core.Command{
		{ID: "CMD-001", Instructions: "Bangun layar.", Pages: []string{"analisis"}, Artifacts: design()},
		{ID: "CMD-002", Pages: []string{"antrean", "pengaturan"}, Artifacts: design()},
	}
	// The answer has no "pages": the outline's hold.
	if err := settlePages(p, cmds, &cmds[0], nil); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(cmds[0].Pages, []string{"analisis"}) || !strings.Contains(cmds[0].Instructions, "`analisis`") ||
		!slices.Contains(cmds[0].FeatureIDs, "FEAT-002") {
		t.Fatalf("command = %+v", cmds[0])
	}
}
