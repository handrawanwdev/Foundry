package work

import (
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
)

// The discovery prompt reasons across the seven planning domains and
// carries the reasoning, adaptive-survey, traceability, consistency and
// output-validation rules. These are the instructions that produce a
// complete, consistent blueprint.
func TestDiscoveryPromptSevenDomains(t *testing.T) {
	p := &core.Project{Story: "Sistem manajemen inventaris gudang"}
	got := DiscoveryPrompt(p)
	for _, want := range []string{
		"01 Product & Business", "02 Domain & Data", "03 Security & Access",
		"04 Experience & Platform", "05 Architecture & Infrastructure",
		"06 Quality & Delivery", "07 Planning Validation",
		"REASONING RULES", "ADAPTIVE SURVEY", "REQUIREMENT TRACEABILITY",
		"CONSISTENCY AND CONFLICTS", "OUTPUT VALIDATION",
		p.Story,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("discovery prompt missing %q", want)
		}
	}
}

// The domains encode the adaptive, evidence-based guarantees: scope is
// named, non-UI products are not forced screens, and no infrastructure is
// added without a need.
func TestDomainsStayAdaptive(t *testing.T) {
	got := analysisDomains + analysisRules + adaptiveSurveyRules
	for _, want := range []string{
		"in-scope, out-of-scope and deferred",
		"never forced screens",
		"no Redis, workers or microservices",
		"Do not invent requirements that were not given",
		"Never re-ask what the story",
		"never present an assumption as the user's decision",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("domains missing %q", want)
		}
	}
}

// The domain titles match the UI checklist (apps/web/src/tell.ts TELL_ANALYSIS)
// so the analysis the user sees and the analysis the model runs stay the same
// seven domains.
func TestDomainsMatchUI(t *testing.T) {
	got := analysisDomains
	for _, title := range []string{
		"Product & Business", "Domain & Data", "Security & Access",
		"Experience & Platform", "Architecture & Infrastructure",
		"Quality & Delivery", "Planning Validation",
	} {
		if !strings.Contains(got, title) {
			t.Errorf("analysisDomains missing domain title %q", title)
		}
	}
}

// The command rules translate data invariants and role-based access into
// concrete instructions and acceptance criteria, and forbid a generic
// "implement everything" command.
func TestCommandRulesCarryInvariantsAndAccess(t *testing.T) {
	got := commandRules
	for _, want := range []string{
		"a transaction must fail without partial change",
		`Never a generic "implement all features"`,
		"names the permission(s) from authz that gate the action",
		"an actor without the permission is refused",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("commandRules missing %q", want)
		}
	}
}

// The workflow prompt embeds the command rules for a fresh workflow, so the
// inventory scenario's consistency and access needs reach the planner.
func TestWorkflowPromptCarriesCommandRules(t *testing.T) {
	p := &core.Project{Story: "Inventaris gudang dengan pencatatan stok atomik dan akses per peran"}
	got := workflowFreshPrompt(p, nil)
	for _, want := range []string{
		"a transaction must fail without partial change",
		"names the permission(s) from authz that gate the action",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("workflow prompt missing %q", want)
		}
	}
}
