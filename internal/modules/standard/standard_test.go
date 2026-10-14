package standard

import (
	"crypto/sha256"
	"encoding/hex"
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

// Rules is pinned: editing any rule changes Version(), which invalidates task
// memory built under the old text. Update this hash on purpose when rules change.
func TestRulesPinnedAndComposable(t *testing.T) {
	sum := sha256.Sum256([]byte(Rules))
	if got := hex.EncodeToString(sum[:8]); got != "b9feb3fc7ba3dbc5" {
		t.Fatalf("Rules changed (hash %s): update this test and expect memory to be invalidated", got)
	}
	if RulesFor(AllScope) != Rules {
		t.Fatal("full scope must equal the full contract")
	}
	if Version() == "" || len(Version()) != 12 {
		t.Fatalf("version = %q", Version())
	}
}

func TestRulesForKeepsGlobalRules(t *testing.T) {
	global := []string{"1. Modular + Clean", "2. Readable code", "3. ORM required", "5. Cross-module side effects", "8. Structure is the contract", "Stop and ask"}
	for name, s := range map[string]Scope{"none": {}, "ui": {Frontend: true}, "backend": {Backend: true}, "all": AllScope} {
		got := RulesFor(s)
		for _, want := range global {
			if !strings.Contains(got, want) {
				t.Fatalf("%s scope dropped global rule %q", name, want)
			}
		}
	}
	ui, be := RulesFor(Scope{Frontend: true}), RulesFor(Scope{Backend: true})
	if strings.Contains(ui, "Redis required") || strings.Contains(ui, "Event envelope") || !strings.Contains(ui, "7. Frontend") {
		t.Fatal("ui scope: no backend details, keep rule 7")
	}
	if strings.Contains(be, "7. Frontend") || !strings.Contains(be, "4. Redis required") || !strings.Contains(be, "Outbox:") || strings.Contains(be, "Inbound webhook") {
		t.Fatal("backend scope: keep Redis/latency/outbox, no rule 7, webhooks only on request")
	}
}
