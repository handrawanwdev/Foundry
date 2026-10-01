package work

import (
	"slices"
	"testing"

	"github.com/handrawanw/foundry/internal/core"
)

func TestBuildPrototype(t *testing.T) {
	p := &core.Project{
		Flows: []core.Flow{{ID: "FLOW-001", Name: "Tiket", PageIDs: []string{"PAGE-001", "PAGE-002", "PAGE-003"}}},
		Design: &core.Design{Pages: []core.Page{
			{ID: "PAGE-001", Name: "Masuk", Kind: "auth", PrimaryAction: &core.PageAction{Label: "Masuk", Target: "PAGE-002"}},
			{ID: "PAGE-002", Name: "Tiket", Platform: "web", Components: []core.PageComponent{
				{Type: "nav", Items: []string{"Tiket", "Laporan"}},
				{Type: "button", Label: "Buat", Target: "PAGE-003"}, {Type: "button", Label: "Lagi", Target: "PAGE-003"}}},
			{ID: "PAGE-003", Name: "Detail", Layout: &core.PageLayout{Back: "PAGE-002"}},
			{ID: "PAGE-004", Name: "Laporan", Platform: "web"},
			{ID: "PAGE-005", Name: "Arsip", Platform: "web", Components: []core.PageComponent{{Type: "button", Target: "PAGE-002"}}},
		}},
	}
	got := BuildPrototype(p)
	if len(got.Pages) != 5 || !got.Pages[0].Entry || got.Pages[1].Entry {
		t.Fatalf("pages = %+v", got.Pages)
	}
	if l := got.Pages[1].Links; len(l) != 2 || l[0].To != "PAGE-004" || l[0].Kind != "nav" || l[1].To != "PAGE-003" {
		t.Fatalf("links = %+v", l)
	}
	if got.Pages[2].Links[0].Kind != "back" {
		t.Fatalf("back link = %+v", got.Pages[2].Links)
	}
	if !slices.Equal(got.Unreachable, []string{"PAGE-005"}) || !slices.Equal(got.DeadEnds, []string{"PAGE-004"}) {
		t.Fatalf("unreachable %v, dead ends %v", got.Unreachable, got.DeadEnds)
	}
	if len(got.Flows) != 1 || len(got.Flows[0].Gaps) != 0 {
		t.Fatalf("flows = %+v", got.Flows)
	}
	p.Flows[0].PageIDs = []string{"PAGE-001", "PAGE-004"}
	if g := BuildPrototype(p).Flows[0].Gaps; !slices.Equal(g, []string{"PAGE-001 → PAGE-004"}) {
		t.Fatalf("gaps = %v", g)
	}
}
