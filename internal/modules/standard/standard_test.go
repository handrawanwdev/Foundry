package standard

import (
	"strings"
	"testing"

	"github.com/handrawanw/foundry/internal/core"
)

func TestDocumentAndProfile(t *testing.T) {
	p := &core.Project{
		Name: "Klinik", Story: "Aplikasi klinik",
		Survey: &core.Survey{Users: core.SurveyUsersSelf, Stage: core.SurveyStageMVP, Latency: core.SurveyLatency500, Webhook: core.SurveyWebhookNone, Services: core.SurveyServicesOne},
		Stack: &core.TechStack{Profile: "saas", Decisions: []core.TechDecision{
			{Area: core.TechBackend, Recommended: "NestJS + TypeScript", Reason: "API"},
			{Area: core.TechCache, Recommended: "Redis", Reason: "wajib"},
		}},
		Structure: &core.Structure{IR: &core.StructureIR{Type: core.StructModular,
			Apps:     []core.StructurePart{{Name: "web", Path: "apps/web/"}},
			Services: []core.StructurePart{{Name: "api", Path: "apps/api/", Modules: []string{"identity"}}},
		}},
		Features: []core.Feature{{ID: "FEAT-001", Name: "Janji", Description: "Buat janji"}},
	}
	doc := Document(p)
	for _, w := range []string{"Delapan aturan wajib", "Redis required", "apps/api", "Definition of Done"} {
		if !strings.Contains(doc, w) {
			t.Errorf("FOUNDRY.md missing %q", w)
		}
	}
	prof := Profile(p)
	for _, w := range []string{"Foundry Profile", "saas", "Redis", "apps/api"} {
		if !strings.Contains(prof, w) {
			t.Errorf("profile missing %q", w)
		}
	}
	req := RequirementsDoc(p)
	if !strings.Contains(req, "Janji") || !strings.Contains(req, "Aplikasi klinik") {
		t.Errorf("requirements = %s", req)
	}
	if !strings.Contains(AgentsMD, "FOUNDRY.md") {
		t.Fatal("AGENTS.md")
	}
}
