package work

import (
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
)

func pagesProject() *core.Project {
	p := uiProject()
	p.UI.Layout.Pages = []core.LayoutPage{{ID: "antrean", Name: "Antrean"}, {ID: "privasi-data", Name: "Privasi dan Hapus Data"}}
	return p
}

func design() []core.ArtifactRef {
	return []core.ArtifactRef{{ID: core.ArtifactDesignRules}, {ID: core.ArtifactLayoutSpec}}
}

func pages(ids ...string) *[]string { return &ids }

func TestSettlePagesWritesTheAnsweredPages(t *testing.T) {
	p := pagesProject()
	cmds := []core.Command{
		{ID: "CMD-001", Instructions: "Bangun hapus lokasi dan hapus semua data.", Artifacts: design()},
		{ID: "CMD-002", Artifacts: design()},
	}
	if err := settlePages(p, cmds, &cmds[0], pages("privasi-data")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cmds[0].Instructions, "`privasi-data`") || len(CheckUI(p, cmds[:1])) != 1 {
		// Only antrean is left unbuilt; CMD-001 itself is fine.
		t.Fatalf("instructions = %q issues = %+v", cmds[0].Instructions, CheckUI(p, cmds[:1]))
	}
	// A page id the layout does not have is refused in the same round.
	c := core.Command{ID: "CMD-001", Instructions: "x", Artifacts: design()}
	if err := settlePages(p, cmds, &c, pages("diagnostik")); err == nil || !strings.Contains(err.Error(), "diagnostik") {
		t.Fatalf("err = %v", err)
	}
	// No page named and no pages: refused with the ids to choose from.
	c = core.Command{ID: "CMD-001", Instructions: "Bangun layar diagnostik.", Artifacts: design()}
	if err := settlePages(p, cmds, &c, nil); err == nil || !strings.Contains(err.Error(), "antrean, privasi-data") {
		t.Fatalf("err = %v", err)
	}
}

func TestSettlePagesDropsDesignForACommandWithoutPages(t *testing.T) {
	p := pagesProject()
	cmds := []core.Command{
		{ID: "CMD-001", Instructions: "Bangun layar antrean dan privasi-data.", Artifacts: design()},
		{ID: "CMD-002", Instructions: "Diagnostik lokal dan log.", Artifacts: append(design(), core.ArtifactRef{ID: "operations"})},
	}
	if err := settlePages(p, cmds, &cmds[1], pages()); err != nil {
		t.Fatal(err)
	}
	if len(cmds[1].Artifacts) != 1 || cmds[1].Artifacts[0].ID != "operations" {
		t.Fatalf("artifacts = %+v", cmds[1].Artifacts)
	}
	if issues := CheckUI(p, cmds); len(issues) != 0 {
		t.Fatalf("issues = %+v", issues)
	}
}

func TestLastLayoutCommandBuildsTheRest(t *testing.T) {
	p := pagesProject()
	cmds := []core.Command{
		{ID: "CMD-001", Instructions: "Bangun layar antrean.", Artifacts: design()},
		{ID: "CMD-002", Instructions: "Diagnostik.", Artifacts: design()},
	}
	err := settlePages(p, cmds, &cmds[1], pages())
	if err == nil || !strings.Contains(err.Error(), "privasi-data") {
		t.Fatalf("dropping design with a page unbuilt: %v", err)
	}
	if err := settlePages(p, cmds, &cmds[1], pages("antrean")); err == nil || !strings.Contains(err.Error(), "privasi-data") {
		t.Fatalf("last command left a page unbuilt: %v", err)
	}
	var b strings.Builder
	writeLayoutPages(&b, p, cmds, cmds[1])
	if out := b.String(); !strings.Contains(out, "antrean: Antrean (already built by CMD-001)") || !strings.Contains(out, "MUST build") || !strings.Contains(out, "privasi-data") {
		t.Fatalf("prompt = %s", out)
	}
}

// The workflow is complete but commands written in earlier rounds fail the
// UI check: they are reopened for their own rounds instead of refusing the
// last round forever.
func TestReopenForUIReopensEarlierCommands(t *testing.T) {
	p := pagesProject()
	w := &core.Workflow{Commands: []core.Command{
		{ID: "CMD-001", Instructions: "Bangun layar antrean.", Artifacts: design(), Acceptance: []string{"a"}},
		{ID: "CMD-002", Instructions: "Hapus semua data.", Artifacts: design(), Files: []string{"x"}},
		{ID: "CMD-003", Instructions: "Verifikasi akhir.", Artifacts: []core.ArtifactRef{{ID: "operations"}}},
	}}
	issues := CheckUI(p, w.Commands)
	reason := reopenForUI(p, w, issues, "CMD-003")
	if reason == "" || w.Commands[1].Instructions != "" || w.Commands[1].Files != nil || w.Commands[0].Instructions == "" {
		t.Fatalf("reason=%q cmds=%+v", reason, w.Commands)
	}
	if next := nextFillCommand(w.Commands); next == nil || next.ID != "CMD-002" {
		t.Fatalf("next = %+v", next)
	}
	// Nothing outside the current command to reopen: the refusal stays.
	w = &core.Workflow{Commands: []core.Command{{ID: "CMD-001", Instructions: "x", Artifacts: design()}}}
	if reopenForUI(p, w, CheckUI(p, w.Commands), "CMD-001") != "" {
		t.Fatal("reopened the current command")
	}
	// A design artifact without its pair is the outline's to fix.
	w = &core.Workflow{Commands: []core.Command{
		{ID: "CMD-001", Instructions: "Hapus data.", Artifacts: design()},
		{ID: "CMD-002", Instructions: "Layar antrean privasi-data.", Artifacts: []core.ArtifactRef{{ID: core.ArtifactLayoutSpec}}},
	}}
	if reopenForUI(p, w, CheckUI(p, w.Commands), "CMD-002") != "" {
		t.Fatal("reopened a command a detail round cannot fix")
	}
}
