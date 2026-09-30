package core

import (
	"encoding/json"
	"strings"
	"testing"
)

func irPage() Page {
	return Page{ID: "PAGE-001", Name: "Tiket", Platform: "web", Density: "compact",
		Components: []PageComponent{
			{ID: "menu", Type: "nav", Items: []string{"Tiket"}},
			{ID: "title", Type: "header", Label: "Tiket"},
			{ID: "create", Type: "button", Label: "Buat tiket", Emphasis: "action.primary"},
			{ID: "filters", Type: "card", Label: "Filter"},
			{ID: "queue", Type: "table", Label: "Antrian", Columns: []string{"ID", "SLA"}},
		},
		Layout: &PageLayout{Frame: "app", Nav: "side", Width: "full",
			Root: LayoutNode{ID: "main", Type: "stack", Gap: "space.md", Children: []LayoutNode{
				{ID: "toolbar", Type: "row", Align: "between", Children: []LayoutNode{{Component: "title"}, {Component: "create"}}},
				{ID: "body", Type: "sidebar", Children: []LayoutNode{{Component: "filters"}, {Component: "queue"}}},
			}},
			Devices: map[string]DeviceLayout{"mobile": {Nav: "bottom", Hide: []string{"filters"},
				Nodes: map[string]NodeChange{"body": {Type: "stack"}, "main": {Order: []string{"body", "toolbar"}}},
				Swap:  map[string]string{"queue": "list"}}}},
	}
}

func TestLayoutJSONRoundTrip(t *testing.T) {
	raw := `{"frame":"app","root":{"id":"main","type":"stack","children":["title",{"type":"row","children":["a","b"]}]}}`
	var l PageLayout
	if err := json.Unmarshal([]byte(raw), &l); err != nil {
		t.Fatal(err)
	}
	if l.Root.Children[0].Component != "title" || l.Root.Children[1].Type != "row" || l.Root.Children[1].Children[1].Component != "b" {
		t.Fatalf("parsed = %+v", l.Root)
	}
	out, _ := json.Marshal(l)
	if string(out) != raw {
		t.Fatalf("round trip:\n%s\n%s", out, raw)
	}
}

func TestValidatePageOK(t *testing.T) {
	if errs := ValidatePage(irPage(), map[string]bool{"PAGE-001": true}); len(errs) > 0 {
		t.Fatalf("valid page rejected: %v", errs)
	}
}

func TestValidatePageErrors(t *testing.T) {
	for _, c := range []struct {
		name string
		edit func(pg *Page)
		want string
	}{
		{"no layout", func(pg *Page) { pg.Layout = nil }, "layout belum ditentukan"},
		{"unknown ref", func(pg *Page) { pg.Layout.Root.Children[0].Children[0].Component = "ghost" }, `komponen "ghost" yang tidak ada`},
		{"orphan", func(pg *Page) { pg.Layout.Root.Children[0].Children = pg.Layout.Root.Children[0].Children[:1] }, `"create" tidak ditempatkan`},
		{"placed twice", func(pg *Page) {
			pg.Layout.Root.Children = append(pg.Layout.Root.Children, LayoutNode{Component: "queue"})
		}, `"queue" ditempatkan 2 kali`},
		{"duplicate id", func(pg *Page) { pg.Components[3].ID = "queue" }, `"queue" dipakai lebih dari sekali`},
		{"nav in tree", func(pg *Page) {
			pg.Layout.Root.Children = append(pg.Layout.Root.Children, LayoutNode{Component: "menu"})
		}, `nav "menu" tidak boleh di layout`},
		{"primitive", func(pg *Page) { pg.Layout.Root.Children[0].Type = "flexbox" }, `primitive "flexbox" tidak didukung`},
		{"arbitrary css", func(pg *Page) { pg.Layout.Root.Gap = "37px" }, `gap "37px" tidak dikenal`},
		{"raw color", func(pg *Page) { pg.Components[2].Emphasis = "#123456" }, `emphasis "#123456" tidak dikenal`},
		{"two primaries", func(pg *Page) { pg.Components[3].Emphasis = "action.primary" }, "maksimal satu aksi utama"},
		{"surface in surface", func(pg *Page) {
			pg.Layout.Root.Surface = "surface.primary"
			pg.Layout.Root.Children[1].Surface = "surface.secondary"
		}, "kartu di dalam kartu"},
		{"split children", func(pg *Page) {
			pg.Layout.Root.Children[1].Children = append(pg.Layout.Root.Children[1].Children, LayoutNode{Component: "title"})
			pg.Layout.Root.Children[0].Children = pg.Layout.Root.Children[0].Children[1:]
		}, "butuh tepat 2 anak"},
		{"mobile sidebar", func(pg *Page) { delete(pg.Layout.Devices["mobile"].Nodes, "body") }, "mobile: node \"body\" masih sidebar"},
		{"mobile nav", func(pg *Page) {
			d := pg.Layout.Devices["mobile"]
			d.Nav = ""
			pg.Layout.Devices["mobile"] = d
		}, `mobile: nav "side" tidak muat`},
		{"bad order", func(pg *Page) { pg.Layout.Devices["mobile"].Nodes["main"] = NodeChange{Order: []string{"body"}} }, "order harus berisi tepat anak node"},
		{"bad device", func(pg *Page) { pg.Layout.Devices["watch"] = DeviceLayout{} }, `perangkat "watch" tidak dikenal`},
		{"bad swap", func(pg *Page) { pg.Layout.Devices["mobile"].Swap["title"] = "list" }, "tidak didukung (hanya table ↔ list)"},
		{"hide unknown", func(pg *Page) {
			d := pg.Layout.Devices["mobile"]
			d.Hide = []string{"nope"}
			pg.Layout.Devices["mobile"] = d
		}, `hide "nope" tidak ada`},
		{"back unknown", func(pg *Page) { pg.Layout.Back = "PAGE-009" }, `back menuju halaman "PAGE-009"`},
		{"grid columns", func(pg *Page) { pg.Layout.Root.Children[1].Type = "grid" }, "grid butuh columns 1-4"},
	} {
		pg := irPage()
		c.edit(&pg)
		err := ValidateDesign([]Page{pg}, []Page{pg})
		if err == nil || !strings.Contains(err.Error(), c.want) || !strings.HasPrefix(err.Error(), "DESIGN_IR_INVALID") {
			t.Errorf("%s: want %q, got %v", c.name, c.want, err)
		}
	}
}

func keys(n LayoutNode) string {
	var out []string
	for _, k := range n.Children {
		if k.Component != "" {
			out = append(out, k.Component)
		} else {
			out = append(out, k.ID+"("+k.Type+":"+keys(k)+")")
		}
	}
	return strings.Join(out, ",")
}

func TestResolvePerDevice(t *testing.T) {
	l := irPage().Layout
	desk, swap := l.Resolve("desktop")
	if got := keys(desk.Root); got != "toolbar(row:title,create),body(sidebar:filters,queue)" || desk.Nav != "side" || swap != nil {
		t.Fatalf("desktop = %s nav %s swap %v", got, desk.Nav, swap)
	}
	mob, swap := l.Resolve("mobile")
	if got := keys(mob.Root); got != "body(stack:queue),toolbar(row:title,create)" || mob.Nav != "bottom" || swap["queue"] != "list" {
		t.Fatalf("mobile = %s nav %s swap %v", got, mob.Nav, swap)
	}
	again, _ := l.Resolve("mobile")
	a, _ := json.Marshal(mob)
	b, _ := json.Marshal(again)
	if string(a) != string(b) {
		t.Fatal("resolve is not deterministic")
	}
	if keys(l.Root) != "toolbar(row:title,create),body(sidebar:filters,queue)" {
		t.Fatal("resolve changed the source layout")
	}
}

func TestPlaceAndDetach(t *testing.T) {
	l := irPage().Layout.Clone()
	l.Detach("filters")
	l.Place("note", "title", false)
	l.Place("top", "", true)
	l.Place("end", "", false)
	if got := keys(l.Root); got != "top,toolbar(row:title,note,create),body(sidebar:queue),end" {
		t.Fatalf("tree = %s", got)
	}
	d := l.Devices["mobile"]
	if len(d.Hide) != 0 || strings.Join(d.Nodes["main"].Order, ",") != "top,body,toolbar,end" {
		t.Fatalf("device refs = %+v", d)
	}
	if keys(irPage().Layout.Root) != "toolbar(row:title,create),body(sidebar:filters,queue)" {
		t.Fatal("clone shares the tree")
	}
}

// Designs saved before the IR get a layout that passes validation and draws
// them as before: auth centered, toolbar and stats on list pages, back link.
func TestEnsureIRLegacy(t *testing.T) {
	pages := []Page{
		{ID: "PAGE-001", Name: "Masuk", Platform: "web", Density: "low", Components: []PageComponent{
			{Type: "header", Label: "Klinik"}, {Type: "form", Label: "Login"}}},
		{ID: "PAGE-002", Name: "Janji", Platform: "web", Kind: "list", Density: "high",
			PrimaryAction: &PageAction{Label: "Buat"}, Components: []PageComponent{
				{Type: "nav", Items: []string{"Janji"}}, {Type: "header", Label: "Janji"}, {Type: "button", Label: "Filter"},
				{Type: "button", Label: "Buat"}, {Type: "stat", Label: "A: 1"}, {Type: "stat", Label: "B: 2"}, {Type: "stat", Label: "C: 3"},
				{Type: "table", Label: "Daftar", Target: "PAGE-003"}}},
		{ID: "PAGE-003", Name: "Detail janji", Platform: "android", Kind: "detail", Components: []PageComponent{
			{Type: "header", Label: "Janji"}, {Type: "stat", Label: "A: 1"}, {Type: "stat", Label: "B: 2"}, {Type: "stat", Label: "C: 3"}}},
		{ID: "PAGE-004", Name: "Kosong", Platform: "web"},
	}
	EnsureIR(pages, "Klinik")
	if err := ValidateDesign(pages, pages); err != nil {
		t.Fatal(err)
	}
	auth := pages[0]
	if l := auth.Layout; l.Frame != "center" || l.Brand || l.Width != "narrow" || auth.Density != "spacious" || auth.Components[1].ID != "form-2" {
		t.Fatalf("auth = %+v %+v", auth, l)
	}
	list := pages[1]
	if got := keys(list.Layout.Root); got != "toolbar(row:header-2,button-3,button-4),stats-stat-5(grid:stat-5,stat-6,stat-7),table-8" {
		t.Fatalf("list tree = %s", got)
	}
	if list.Density != "compact" || list.Layout.Nav != "side" || list.Layout.Devices["mobile"].Nav != "bottom" ||
		list.Components[3].Emphasis != "action.primary" || list.Components[2].Emphasis != "" {
		t.Fatalf("list = %+v", list)
	}
	if l := pages[2].Layout; l.Back != "PAGE-002" || l.Root.Children[1].Columns != 2 || l.Devices != nil {
		t.Fatalf("mobile detail = %+v", l)
	}
	pages[2].Layout, pages[2].Platform = nil, "web"
	EnsureIR(pages, "Klinik")
	if l := pages[2].Layout; l.Back != "PAGE-002" || l.Width != "wide" || l.Root.Children[1].Columns != 3 || l.Devices["mobile"].Nodes["stats-stat-2"].Columns != 2 {
		t.Fatalf("detail = %+v", l)
	}
	prev := *pages[1].Layout
	EnsureIR(pages, "Klinik")
	if keys(pages[1].Layout.Root) != keys(prev.Root) {
		t.Fatal("EnsureIR changed an existing layout")
	}
}
