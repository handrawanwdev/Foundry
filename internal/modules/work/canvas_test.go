package work

import (
	"errors"
	"strings"
	"testing"

	"github.com/handrawanw/foundry/internal/core"
)

func canvasProject() *core.Project {
	p := opsProject()
	p.Design.Status = core.DesignDraft
	core.EnsureIR(p.Design.Pages, p.Name)
	return p
}

func intp(n int) *int { return &n }

func TestEditPageTree(t *testing.T) {
	p := canvasProject()
	other := mustJSON(p.Design.Pages[1])
	done, err := EditPage(p, "PAGE-001", []CanvasOp{
		{Op: "move", ID: "table-4", Parent: "main", Index: intp(0)},
		{Op: "wrap", IDs: []string{"text-3"}, Node: &core.LayoutNode{ID: "notes", Type: "stack", Surface: "surface.secondary"}},
		{Op: "update", ID: "header-1", Component: &core.PageComponent{Type: "header", Label: "Janji Temu"}},
		{Op: "add", Parent: "toolbar", Component: &core.PageComponent{Type: "button", Label: "Ekspor"}},
		{Op: "set_page", Fields: map[string]string{"density": "compact"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	pg := p.Design.Pages[0]
	if got := mustJSON(pg.Layout.Root); got != `{"id":"main","type":"stack","children":["table-4",{"id":"toolbar","type":"row","children":["header-1","button-2","button-5"],"align":"between"},{"id":"notes","type":"stack","children":["text-3"],"surface":"surface.secondary"}]}` {
		t.Fatalf("layout = %s", got)
	}
	if pg.Components[0].Label != "Janji Temu" || pg.Components[0].ID != "header-1" || pg.Density != "compact" || len(pg.Components) != 5 {
		t.Fatalf("page = %+v", pg)
	}
	if len(done) != 5 || !strings.Contains(done[0], "MOVE #table-4") {
		t.Fatalf("done = %v", done)
	}
	if mustJSON(p.Design.Pages[1]) != other {
		t.Fatal("other page changed")
	}

	if _, err := EditPage(p, "PAGE-001", []CanvasOp{{Op: "unwrap", ID: "toolbar"}, {Op: "remove", ID: "button-5"}}); err != nil {
		t.Fatal(err)
	}
	pg = p.Design.Pages[0]
	if got := mustJSON(pg.Layout.Root); got != `{"id":"main","type":"stack","children":["table-4","header-1","button-2",{"id":"notes","type":"stack","children":["text-3"],"surface":"surface.secondary"}]}` {
		t.Fatalf("layout = %s", got)
	}
	if _, ok := pg.Layout.Devices["mobile"].Nodes["toolbar"]; ok {
		t.Fatal("device change of an unwrapped node must go")
	}
}

func TestEditPageRejectsInvalidIR(t *testing.T) {
	p := canvasProject()
	before := mustJSON(p.Design.Pages)
	for _, ops := range [][]CanvasOp{
		{{Op: "wrap", IDs: []string{"text-3", "table-4"}, Node: &core.LayoutNode{ID: "g", Type: "grid", Columns: 2}},
			{Op: "set_node", ID: "g", Node: &core.LayoutNode{Type: "grid", Columns: 6}}},
		{{Op: "move", ID: "main"}},
		{{Op: "move", ID: "toolbar", Parent: "toolbar"}},
		{{Op: "remove", ID: "nope"}},
		{{Op: "set_page", Fields: map[string]string{"route": "/x"}}},
		{{Op: "explode"}},
	} {
		if _, err := EditPage(p, "PAGE-001", ops); err == nil {
			t.Errorf("ops %+v must be rejected", ops)
		}
	}
	_, err := EditPage(p, "PAGE-001", []CanvasOp{{Op: "set_page", Fields: map[string]string{"nav": "floating"}}})
	var invalid *core.IRInvalid
	if !errors.As(err, &invalid) {
		t.Fatalf("err = %v, want DESIGN_IR_INVALID", err)
	}
	if mustJSON(p.Design.Pages) != before {
		t.Fatal("a rejected edit must change nothing")
	}
	p.Design.Revision = &core.DesignRevision{}
	if _, err := EditPage(p, "PAGE-001", []CanvasOp{{Op: "set_page", Fields: map[string]string{"name": "X"}}}); err == nil {
		t.Fatal("pending revision must block canvas edits")
	}
}
