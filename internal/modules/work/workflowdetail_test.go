package work

import (
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
)

func TestSplitSectionsIgnoresFencedHeadings(t *testing.T) {
	secs := splitSections("intro\n## A\na\n```\n## not a heading\n```\n### A.1\nx\n## B\nb\n")
	var heads []string
	for _, s := range secs {
		heads = append(heads, s.heading)
	}
	if got := strings.Join(heads, "|"); got != "|## A|### A.1|## B" {
		t.Fatalf("headings = %q", got)
	}
	if secs[2].parent != 1 || secs[3].parent != -1 {
		t.Fatalf("parents = %d %d", secs[2].parent, secs[3].parent)
	}
	if !strings.Contains(secs[1].text, "## not a heading") {
		t.Fatal("fenced line lost")
	}
}

func TestScopeArtifact(t *testing.T) {
	filler := strings.Repeat("kalimat pengisi umum tentang sistem. ", 40)
	body := "Pengantar artefak.\n" +
		"## Cache Redis\nRedis menyimpan counter rate limit. " + filler + "\n" +
		"## Penyimpanan durable\n### SQLite Edge\nSQLite WAL synchronous FULL agar data lokal tidak hilang saat restart (REQ-025). " + filler + "\n" +
		"## Riwayat revisi\n" + filler + "\n"
	var q scopeQuery
	q.add("Penyimpanan durable Edge SQLite restart", 2)
	q.add("REQ-025", 1)

	if got := scopeArtifact(body, q, len(body)); got != body {
		t.Fatal("an artifact that fits is not read whole")
	}
	got := scopeArtifact(body, q, 2200)
	for _, want := range []string{"Pengantar artefak.", "## Penyimpanan durable\n### SQLite Edge", "synchronous FULL", "left out as not about this command: Cache Redis; Riwayat revisi"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "counter rate limit") {
		t.Errorf("unrelated section kept:\n%s", got)
	}
	if len(got) > 2200+300 {
		t.Errorf("over budget: %d", len(got))
	}
	// The best section too long for the room is cut, not lost.
	got = scopeArtifact(body, q, 1500)
	if !strings.Contains(got, "synchronous FULL") || !strings.Contains(got, "…(dipotong)") {
		t.Errorf("best section lost:\n%s", got)
	}
}

func detailProject() *core.Project {
	return &core.Project{
		Name:  "P",
		Story: "cerita",
		Requirements: []core.Requirement{
			{ID: "REQ-001", Priority: "MUST", Text: "login", Acceptance: []core.Criterion{{ID: "REQ-001-AC1", Kind: "behavior", Text: "sesi dibuat", Evidence: "test login"}}},
			{ID: "REQ-002", Priority: "MUST", Text: "laporan", Acceptance: []core.Criterion{{ID: "REQ-002-AC1", Kind: "behavior", Text: "laporan tampil"}}},
		},
		Features: []core.Feature{{ID: "FEAT-001", Name: "Akses", RequirementIDs: []string{"REQ-001"}}, {ID: "FEAT-002", Name: "Laporan", RequirementIDs: []string{"REQ-002"}}},
		Workflow: &core.Workflow{Commands: []core.Command{
			{ID: "CMD-001", Name: "Fondasi", Objective: "siapkan repo", Covers: []string{"REQ-001"}, FeatureIDs: []string{"FEAT-001"}},
			{ID: "CMD-002", Name: "Laporan", Objective: "bangun laporan", DependsOn: []string{"CMD-001"}, Covers: []string{"REQ-002"}, FeatureIDs: []string{"FEAT-002"}},
			{ID: "CMD-003", Name: "Lain", Objective: "tujuan jauh", Covers: []string{"REQ-001"}},
		}},
	}
}

// Every detail round shares the static part byte for byte, so the provider
// cache serves it; the dynamic part carries what the round's command needs.
func TestDetailPromptRounds(t *testing.T) {
	p := detailProject()
	static1, body1 := SplitPrompt(WithPolicy(WorkflowPrompt(p, nil)))
	if !strings.Contains(body1, "- CMD-001: Fondasi") || !strings.Contains(body1, "REQ-001-AC1 (behavior): sesi dibuat [evidence: test login]") {
		t.Fatalf("round 1:\n%s", body1)
	}
	c := &p.Workflow.Commands[0]
	c.Instructions, c.Outputs, c.Files = "isi", []string{"skema users"}, []string{"apps/api/"}
	static2, body2 := SplitPrompt(WithPolicy(WorkflowPrompt(p, nil)))
	if static1 != static2 {
		t.Fatal("static part changed between rounds")
	}
	if !strings.Contains(static2, "Any other criterion has no id: never number new ones") {
		t.Error("rule against invented criterion ids missing")
	}
	for _, want := range []string{
		"- CMD-002: Laporan",                     // the round
		"REQ-002-AC1 (behavior): laporan tampil", // its criteria, whole
		"output: skema users",                    // what its dependency hands over
		"files: apps/api/",
		"- CMD-003 Lain | covers REQ-001", // every command named
		"FEAT-002 Laporan",
		`"id" is exactly CMD-002`,
	} {
		if !strings.Contains(body2, want) {
			t.Errorf("round 2 misses %q:\n%s", want, body2)
		}
	}
	if strings.Contains(body2, "tujuan jauh") || strings.Contains(body2, "REQ-001-AC1") {
		t.Errorf("round 2 carries what another command needs:\n%s", body2)
	}
	// A command that covers no requirement is told its criteria carry no id.
	p.Workflow.Commands[1].Covers = nil
	if _, body := SplitPrompt(WithPolicy(WorkflowPrompt(p, nil))); !strings.Contains(body, "this command covers none; its acceptance criteria carry no requirement or criterion id") {
		t.Errorf("no-cover note missing:\n%s", body)
	}
	p.Workflow.Commands[1].Covers = []string{"REQ-002"}
	// A refused answer is named in the retry.
	p.Workflow.Request = &core.ChangeRequest{Rejected: "detail untuk CMD-002 tidak ada"}
	if _, body := SplitPrompt(WithPolicy(WorkflowPrompt(p, nil))); !strings.Contains(body, "YOUR LAST ANSWER WAS REFUSED, fix this: detail untuk CMD-002 tidak ada") {
		t.Errorf("refusal missing:\n%s", body)
	}
}
