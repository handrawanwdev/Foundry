package work

import (
	"fmt"
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
)

// bigDocs is a blueprint above workflowSketchFullBody: the outline prompt
// carries summaries only, as for the real ParkOS blueprint.
func bigDocs() []core.Artifact {
	return []core.Artifact{
		{ID: core.ArtifactRequirements, Title: "Requirements", Summary: "ringkasan", Body: strings.Repeat("isi-requirements ", 4000)},
		{ID: "threat-model", Title: "Threat model", Summary: "ringkasan", Body: "x"},
	}
}

// The ParkOS outline was refused twice: the blueprint is read by summaries,
// so the planner never saw REQ-025 and REQ-031, and the retry named only
// their ids. The registry and the retry now carry what they mean.
func TestSketchPromptCarriesRequirementRegistryWhenDocsAreSummarized(t *testing.T) {
	p, _ := parkOS(t)
	p.Workflow = nil
	prompt := WorkflowPrompt(p, bigDocs())
	if strings.Contains(prompt, "isi-requirements") {
		t.Fatal("blueprint bodies in a summaries-only outline")
	}
	for _, want := range []string{
		"REQUIREMENTS (registry",
		"- REQ-025 [MUST] Data lokal tidak hilang saat aplikasi atau perangkat restart.",
		"REQ-025-AC1 (behavior) Given transaksi tersimpan lokal",
		"- REQ-031 [MUST] Keamanan dasar: enkripsi koneksi, batas permintaan, validasi input, pengelolaan secret.",
		"REQ-031-AC3 (security)",
		"FEAT-015 Edge Controller dan Mode Offline (requirements REQ-024, REQ-025, REQ-026)",
		"every MUST id appears in the covers of a command that builds what its acceptance criteria check",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt lacks %q", want)
		}
	}
	if strings.Contains(prompt, "LEFT UNCOVERED") {
		t.Fatal("repair section without a refusal")
	}
	// Only MUST requirements carry their criteria.
	for _, r := range p.Requirements {
		if r.Priority != "MUST" && len(r.Acceptance) > 0 && strings.Contains(prompt, r.Acceptance[0].ID+" (") {
			t.Fatalf("%s %s criteria in the registry", r.Priority, r.ID)
		}
	}
}

func TestSketchRetrySpellsOutTheUncoveredMustRequirements(t *testing.T) {
	p, _ := parkOS(t)
	refused := "workflow tidak valid: requirement MUST belum dicakup command mana pun (covers): REQ-025, REQ-031"
	p.Workflow = &core.Workflow{Request: &core.ChangeRequest{Rejected: refused}}
	prompt := WorkflowPrompt(p, bigDocs())
	i := strings.Index(prompt, "MUST REQUIREMENTS YOUR LAST ANSWER LEFT UNCOVERED")
	if i < 0 {
		t.Fatal("no repair section")
	}
	repair := prompt[i:strings.Index(prompt, "SKETCH PHASE")]
	for _, want := range []string{
		"REQ-025 [MUST] Data lokal tidak hilang",
		"features: FEAT-015 Edge Controller dan Mode Offline",
		"[evidence: Test crash recovery]",
		"REQ-031 [MUST] Keamanan dasar",
		"features: FEAT-016 Keamanan Operasi dan Keandalan",
		"REQ-031-AC1 (security) Given koneksi tanpa TLS",
		"REQ-031-AC2 (negative)",
		"REQ-031-AC3 (security) Given pemindaian repositori",
		"Never add an id to a command that only mentions it",
		"Keep every other command",
	} {
		if !strings.Contains(repair, want) {
			t.Fatalf("repair section lacks %q:\n%s", want, repair)
		}
	}
	if strings.Contains(repair, "REQ-024 [") {
		t.Fatal("a covered requirement in the repair section")
	}

	// Any other refusal gets no coverage section.
	p.Workflow.Request.Rejected = "workflow tidak valid: C3 dependency C9 tidak ada"
	if strings.Contains(WorkflowPrompt(p, bigDocs()), "LEFT UNCOVERED") {
		t.Fatal("coverage section for a refusal that is not about coverage")
	}
}

func TestUncoveredRequirementsReadsOnlyKnownIDsAfterThePrefix(t *testing.T) {
	p := &core.Project{Requirements: []core.Requirement{{ID: "REQ-001"}, {ID: "REQ-002"}, {ID: "REQ-003"}}}
	got := uncoveredRequirements(p, "C1 requirement REQ-001 tidak ada; "+uncoveredMust+"REQ-003, REQ-002, REQ-099")
	if len(got) != 2 || got[0].ID != "REQ-002" || got[1].ID != "REQ-003" {
		t.Fatalf("got %+v", got)
	}
	if uncoveredRequirements(p, "output AI tidak berisi JSON yang valid") != nil {
		t.Fatal("ids read from an unrelated refusal")
	}
}

// The validator itself is unchanged: an outline that leaves a MUST
// requirement uncovered is refused with the ids, one that covers it lands.
func TestSketchStillRefusesUncoveredMustAndAcceptsCoverage(t *testing.T) {
	p := &core.Project{Requirements: []core.Requirement{
		{ID: "REQ-025", Priority: "MUST"}, {ID: "REQ-031", Priority: "MUST"}, {ID: "REQ-033", Priority: "COULD"}}}
	docs := []core.Artifact{{ID: "requirements", Title: "R", Body: "v"}}
	outline := func(covers string) string {
		return fmt.Sprintf(`{"commands": [{"id": "C1", "name": "Edge", "objective": "o", "artifacts": ["requirements"], "covers": [%s]}]}`, covers)
	}
	err := ApplyWorkflow(p, outline(`"REQ-033"`), docs)
	if err == nil || !strings.Contains(err.Error(), "requirement MUST belum dicakup command mana pun (covers): REQ-025, REQ-031") {
		t.Fatalf("err = %v", err)
	}
	if got := uncoveredRequirements(p, err.Error()); len(got) != 2 {
		t.Fatalf("the refusal does not round-trip to its requirements: %+v", got)
	}
	if err := ApplyWorkflow(p, outline(`"REQ-025", "REQ-031"`), docs); err != nil {
		t.Fatal(err)
	}
}

// The second refusal of the ParkOS retry: a subfeature whose feature was not
// in feature_ids. The validator checks it, so the prompts state it.
func TestWorkflowPromptsStateTheSubfeatureParentRule(t *testing.T) {
	p, _ := parkOS(t)
	p.Workflow = nil
	if !strings.Contains(WorkflowPrompt(p, bigDocs()), "Every subfeature belongs to a feature in the same command's feature_ids (FEAT-007.1 needs FEAT-007)") {
		t.Fatal("outline prompt lacks the subfeature rule")
	}
	if !strings.Contains(commandRules, "Every subfeature id belongs to a feature listed in the same command's feature_ids") {
		t.Fatal("command rules lack the subfeature rule")
	}
}
