package work

import (
	"strings"
	"testing"

	"github.com/handrawanw/foundry/internal/core"
)

func revisionProject() *core.Project {
	return &core.Project{
		Platform: &core.PlatformPlan{},
		Features: []core.Feature{{ID: "FEAT-001", Name: "Janji"}},
		Flows:    []core.Flow{{ID: "FLOW-001", PageIDs: []string{"PAGE-001", "PAGE-002"}}},
		Design: &core.Design{Status: core.DesignChanges, Feedback: []string{"lama satu", "baru"}, Pages: []core.Page{
			{ID: "PAGE-001", Name: "Daftar", FeatureID: "FEAT-001", Components: []core.PageComponent{{Type: "button", Label: "Buka", Target: "PAGE-002"}}},
			{ID: "PAGE-002", Name: "Detail", FeatureID: "FEAT-001"},
		}},
	}
}

func TestDesignPromptRevision(t *testing.T) {
	got := DesignPrompt(revisionProject())
	for _, want := range []string{"CURRENT DESIGN:", "EARLIER REQUESTS (already applied in the current design, do not redo):\n- lama satu", "CHANGE REQUEST (apply now):\nbaru", `"removed"`} {
		if !strings.Contains(got, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
	if strings.Contains(got, "apply all") {
		t.Error("earlier requests must not be applied again")
	}
}

func TestDesignPromptScreens(t *testing.T) {
	p := revisionProject()
	p.Design.Screens = map[string]string{"PAGE-001": "Tampilan desktop:\n[0] Tombol \"Buka\" (utama)"}
	got := DesignPrompt(p)
	if !strings.Contains(got, "WHAT THE USER SEES NOW") || !strings.Contains(got, "PAGE-001 \"Daftar\"\nTampilan desktop:\n[0] Tombol") {
		t.Fatalf("prompt missing screens:\n%s", got)
	}
	if err := ApplyDesign(p, "```json\n{\"removed\": [\"PAGE-002\"]}\n```"); err != nil || p.Design.Screens != nil {
		t.Fatalf("screens must be dropped after a revision: %v %v", err, p.Design.Screens)
	}
}

func TestApplyRevisionRemove(t *testing.T) {
	p := revisionProject()
	if err := ApplyDesign(p, "```json\n{\"removed\": [\"PAGE-002\"]}\n```"); err != nil {
		t.Fatal(err)
	}
	if len(p.Design.Pages) != 1 || p.Design.Pages[0].Components[0].Target != "" {
		t.Fatalf("pages = %+v", p.Design.Pages)
	}
	if got := p.Flows[0].PageIDs; len(got) != 1 || got[0] != "PAGE-001" {
		t.Fatalf("flow = %v", got)
	}
	if r := p.Design.Revision; r == nil || len(r.Changes) != 1 || r.Changes[0].Kind != "removed" {
		t.Fatalf("revision = %+v", r)
	}
	if err := ApplyDesign(revisionProject(), "```json\n{}\n```"); err == nil {
		t.Fatal("empty change set must fail")
	}
}

func TestFormButtonsNotDuplicated(t *testing.T) {
	p := revisionProject()
	raw := "```json\n{\"pages\": [{\"id\": \"PAGE-002\", \"name\": \"Masuk\", \"feature_id\": \"FEAT-001\", \"kind\": \"auth\"," +
		"\"primary_action\": {\"label\": \"Masuk\"}, \"components\": [{\"type\": \"form\", \"fields\": [{\"label\": \"Email\"}]}," +
		"{\"type\": \"button\", \"label\": \"masuk\"}, {\"type\": \"button\", \"label\": \"Daftar akun\"}]}]}\n```"
	if err := ApplyDesign(p, raw); err != nil {
		t.Fatal(err)
	}
	var labels []string
	for _, c := range p.Design.Pages[1].Components {
		labels = append(labels, c.Type+":"+c.Label)
	}
	if got := strings.Join(labels, ","); got != "form:,button:Daftar akun" {
		t.Fatalf("components = %s", got)
	}
	if !strings.Contains(DesignPrompt(revisionProject()), "Never add a \"button\" that submits") {
		t.Error("prompt must explain the form's own buttons")
	}
}

func opsProject() *core.Project {
	p := revisionProject()
	p.Design.Pages[0].Kind = "list"
	p.Design.Pages[0].States = &core.PageStates{Empty: "Belum ada janji."}
	p.Design.Pages[0].Components = []core.PageComponent{
		{Type: "header", Label: "Janji"}, {Type: "button", Label: "Buka", Target: "PAGE-002"},
		{Type: "text", Label: "Catatan"}, {Type: "table", Label: "Daftar", Columns: []string{"Pasien"}, Items: []string{"Budi"}},
	}
	return p
}

func labels(pg core.Page) string {
	var out []string
	for _, c := range pg.Components {
		out = append(out, c.Label)
	}
	return strings.Join(out, ",")
}

func TestApplyRevisionOps(t *testing.T) {
	p := opsProject()
	before := p.Design.Pages[1]
	raw := "```json\n{\"ops\": [" +
		"{\"page\": \"PAGE-001\", \"op\": \"remove\", \"index\": 2}," +
		"{\"page\": \"PAGE-001\", \"op\": \"move\", \"index\": 3, \"after\": 0}," +
		"{\"page\": \"PAGE-001\", \"op\": \"insert\", \"after\": -1, \"component\": {\"type\": \"text\", \"label\": \"Info\"}}," +
		"{\"page\": \"PAGE-001\", \"op\": \"update\", \"index\": 0, \"component\": {\"type\": \"header\", \"label\": \"Janji Temu\"}}," +
		"{\"page\": \"PAGE-001\", \"op\": \"page\", \"set\": {\"density\": \"high\"}}]}\n```"
	if err := ApplyDesign(p, raw); err != nil {
		t.Fatal(err)
	}
	pg := p.Design.Pages[0]
	if got := labels(pg); got != "Info,Janji Temu,Daftar,Buka" {
		t.Fatalf("components = %s", got)
	}
	if pg.Kind != "list" || pg.Density != "high" || pg.States == nil || pg.States.Empty != "Belum ada janji." || pg.Components[3].Target != "PAGE-002" {
		t.Fatalf("untouched fields changed: %+v", pg)
	}
	if got := p.Design.Pages[1]; got.Name != before.Name || len(got.Components) != len(before.Components) {
		t.Fatalf("other page changed: %+v", got)
	}
	if prev := p.Design.Revision.PreviousPages[0]; labels(prev) != "Janji,Buka,Catatan,Daftar" || prev.Density != "" {
		t.Fatalf("previous version changed: %+v", prev)
	}
}

func TestApplyRevisionOpsErrors(t *testing.T) {
	for _, ops := range []string{
		`{"page": "PAGE-009", "op": "remove", "index": 0}`,
		`{"page": "PAGE-001", "op": "remove", "index": 7}`,
		`{"page": "PAGE-001", "op": "remove", "index": 1}, {"page": "PAGE-001", "op": "update", "index": 1, "component": {"type": "text"}}`,
		`{"page": "PAGE-001", "op": "page", "set": {"id": "PAGE-005"}}`,
		`{"page": "PAGE-001", "op": "shuffle"}`,
	} {
		p := opsProject()
		if err := ApplyDesign(p, "```json\n{\"ops\": ["+ops+"]}\n```"); err == nil {
			t.Errorf("ops %s must fail", ops)
		}
		if labels(p.Design.Pages[0]) != "Janji,Buka,Catatan,Daftar" {
			t.Errorf("failed ops must not change the design")
		}
	}
}
