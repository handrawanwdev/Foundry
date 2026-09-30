package work

import (
	"errors"
	"slices"
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

func TestRevisionPromptScoped(t *testing.T) {
	p := opsProject()
	p.Design.Screens = map[string]string{"PAGE-001": "Tampilan mobile:\n[1] button: Tombol \"Buka\" (utama)"}
	p.Design.Scope = &core.ChangeScope{Pages: []core.ScopePage{{ID: "PAGE-001", Request: "pindahkan ke atas", Components: []int{1}, Device: "mobile"}}}
	got := DesignPrompt(p)
	for _, want := range []string{
		"EARLIER REQUESTS (already applied, do not redo):\n- lama satu",
		"CURRENT PAGE:\nPAGE-001 \"Daftar\" (, kind list, user sees it on mobile)",
		"[2] {\"type\":\"text\",\"label\":\"Catatan\"}",
		"WHAT THE USER SEES", "[1] button: Tombol",
		"TARGET:\n[1] button \"Buka\"", "CHANGE:\npindahkan ke atas",
		"components [0] [2] [3]", "all page fields: kind, route",
		"Patch operations only. Do not return the full page.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
	for _, not := range []string{"CURRENT PAGE:\nPAGE-002", "DESIGN RULES", "auth: login/sign up"} {
		if strings.Contains(got, not) {
			t.Errorf("scoped prompt must not contain %q", not)
		}
	}
	p.Design.Scope = &core.ChangeScope{Open: true, General: "rapikan"}
	if got := DesignPrompt(p); !strings.Contains(got, "DESIGN RULES (only for new pages") || !strings.Contains(got, "CURRENT PAGE:\nPAGE-002") {
		t.Error("open request needs every page and the design rules")
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

func TestApplyRevisionPatch(t *testing.T) {
	p := opsProject()
	p.Design.Scope = &core.ChangeScope{Pages: []core.ScopePage{{ID: "PAGE-001", Fields: true}}}
	before := p.Design.Pages[1]
	raw := "```json\n{\"patches\": [{\"page\": \"PAGE-001\", \"operations\": [" +
		"{\"op\": \"remove\", \"target\": 2}," +
		"{\"op\": \"move\", \"target\": 3, \"after\": 0}," +
		"{\"op\": \"add\", \"after\": -1, \"component\": {\"type\": \"text\", \"label\": \"Info\"}}," +
		"{\"op\": \"update\", \"target\": 0, \"component\": {\"type\": \"header\", \"label\": \"Janji Temu\"}}," +
		"{\"op\": \"update\", \"page_fields\": {\"density\": \"high\"}}]}]}\n```"
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
	rev := p.Design.Revision
	if prev := rev.PreviousPages[0]; labels(prev) != "Janji,Buka,Catatan,Daftar" || prev.Density != "" {
		t.Fatalf("previous version changed: %+v", prev)
	}
	if len(rev.Patch) != 5 || rev.Patch[1] != "PAGE-001 [3] MOVE → setelah [0]" || rev.Scope == nil {
		t.Fatalf("revision = %+v", rev)
	}
}

func TestApplyRevisionPatchRejected(t *testing.T) {
	picked := &core.ChangeScope{Pages: []core.ScopePage{{ID: "PAGE-001", Components: []int{2}}}}
	for _, c := range []struct {
		scope *core.ChangeScope
		body  string
		want  string
	}{
		{picked, `"patches": [{"page": "PAGE-001", "operations": [{"op": "move", "target": 2, "after": 0}, {"op": "update", "target": 1, "component": {"type": "button", "label": "X"}}]}]`, "PAGE-001 [1] UPDATE, padahal yang dipilih [2]"},
		{picked, `"patches": [{"page": "PAGE-001", "operations": [{"op": "update", "page_fields": {"kind": "admin"}}]}]`, "PAGE-001 kind berubah"},
		{picked, `"patches": [{"page": "PAGE-002", "operations": [{"op": "add", "component": {"type": "text", "label": "X"}}]}]`, "PAGE-002 di-patch, padahal tidak diminta"},
		{picked, `"removed": ["PAGE-002"]`, "PAGE-002 dihapus"},
		{nil, `"pages": [{"id": "PAGE-001", "name": "Daftar", "feature_id": "FEAT-001"}]`, "PAGE-001 dikirim utuh"},
	} {
		p := opsProject()
		p.Design.Scope = c.scope
		err := ApplyDesign(p, "```json\n{"+c.body+"}\n```")
		var rej *PatchRejected
		if !errors.As(err, &rej) || !strings.Contains(err.Error(), c.want) || !strings.Contains(err.Error(), "PATCH DITOLAK") {
			t.Errorf("%s: err = %v, want %q", c.body, err, c.want)
		}
		if labels(p.Design.Pages[0]) != "Janji,Buka,Catatan,Daftar" || p.Design.Revision != nil {
			t.Errorf("rejected patch must not change the design")
		}
	}
}

func TestCheckPatchCatchesSideEffects(t *testing.T) {
	prev := opsProject().Design.Pages[0]
	next := prev
	next.Components = slices.Clone(prev.Components)
	next.Components[0].Label = "Lain"
	next.Kind = "admin"
	ops := []patchOp{{Op: "move", Target: ptr(3), After: ptr(1)}}
	got := strings.Join(checkPatch(prev, next, []int{0, 1, 2, 3}, ops, nil, nil), "; ")
	if got != "PAGE-001 kind berubah; PAGE-001 [0] label berubah" {
		t.Fatalf("got %q", got)
	}
}

func ptr(i int) *int { return &i }

func TestApplyRevisionPatchErrors(t *testing.T) {
	for _, ops := range []string{
		`{"op": "remove", "target": 7}`,
		`{"op": "remove", "target": 1}, {"op": "update", "target": 1, "component": {"type": "text"}}`,
		`{"op": "update", "page_fields": {"id": "PAGE-005"}}`,
		`{"op": "shuffle"}`,
	} {
		p := opsProject()
		if err := ApplyDesign(p, "```json\n{\"patches\": [{\"page\": \"PAGE-001\", \"operations\": ["+ops+"]}]}\n```"); err == nil {
			t.Errorf("ops %s must fail", ops)
		}
		if labels(p.Design.Pages[0]) != "Janji,Buka,Catatan,Daftar" {
			t.Errorf("failed ops must not change the design")
		}
	}
}

func TestApplyQAFix(t *testing.T) {
	p := opsProject()
	p.Design.Scope = &core.ChangeScope{Pages: []core.ScopePage{{ID: "PAGE-001"}}}
	if err := ApplyDesign(p, "```json\n{\"patches\": [{\"page\": \"PAGE-001\", \"operations\": [{\"op\": \"remove\", \"target\": 2}]}]}\n```"); err != nil {
		t.Fatal(err)
	}
	if err := ApplyQAFix(p, "```json\n{\"patches\": [{\"page\": \"PAGE-001\", \"operations\": [{\"op\": \"move\", \"target\": 2, \"after\": -1}]}]}\n```"); err != nil {
		t.Fatal(err)
	}
	if got := labels(p.Design.Pages[0]); got != "Daftar,Janji,Buka" {
		t.Fatalf("components = %s", got)
	}
	if labels(p.Design.Revision.PreviousPages[0]) != "Janji,Buka,Catatan,Daftar" || len(p.Design.Revision.Patch) != 2 {
		t.Fatalf("revision = %+v", p.Design.Revision)
	}
	if err := ApplyQAFix(p, "```json\n{\"patches\": [{\"page\": \"PAGE-002\", \"operations\": [{\"op\": \"add\", \"component\": {\"type\": \"text\"}}]}]}\n```"); err == nil {
		t.Fatal("QA fix outside the scope must be rejected")
	}
}
