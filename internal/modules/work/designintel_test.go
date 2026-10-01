package work

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/handrawanw/foundry/internal/core"
)

func TestDecide(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   PlanInput
		want Decision
	}{
		{"support agent resolves tickets all day",
			PlanInput{Intent: core.PageIntent{PrimaryTask: "process", Frequency: "high", Complexity: "high", InformationDensity: "high"}, Platform: "web", Domain: "ticketing", HasNav: true},
			Decision{PageType: "workspace", Layout: "master-detail", Navigation: "side", Density: "compact", Responsive: "list_then_detail"}},
		{"occasional review of a queue",
			PlanInput{Intent: core.PageIntent{PrimaryTask: "process", Frequency: "low"}, Platform: "web", HasNav: true},
			Decision{PageType: "master-detail", Layout: "master-detail", Navigation: "side", Density: "spacious", Responsive: "list_then_detail"}},
		{"sign in",
			PlanInput{Intent: core.PageIntent{PrimaryTask: "authenticate", Frequency: "high"}, Platform: "web"},
			Decision{PageType: "auth", Layout: "focused", Navigation: "none", Density: "spacious", Responsive: "same"}},
		{"comparison-heavy admin list",
			PlanInput{Intent: core.PageIntent{PrimaryTask: "browse", Frequency: "high", Complexity: "medium", Comparison: true}, Platform: "web", Domain: "admin_operations", HasNav: true},
			Decision{PageType: "table", Layout: "table", Navigation: "side", Density: "compact", Responsive: "table_to_list"}},
		{"simple browse stays a list",
			PlanInput{Intent: core.PageIntent{PrimaryTask: "browse", Frequency: "medium", InformationDensity: "low"}, Platform: "web", HasNav: true},
			Decision{PageType: "list", Layout: "list", Navigation: "side", Density: "comfortable", Responsive: "stack"}},
		{"rare settings are guided",
			PlanInput{Intent: core.PageIntent{PrimaryTask: "configure", Frequency: "low"}, Platform: "web", HasNav: true},
			Decision{PageType: "settings", Layout: "sections", Navigation: "side", Density: "spacious", Responsive: "stack"}},
		{"frequent data entry keeps room for labels",
			PlanInput{Intent: core.PageIntent{PrimaryTask: "enter", Frequency: "high", Complexity: "high"}, Platform: "web", HasNav: true},
			Decision{PageType: "form", Layout: "sections", Navigation: "side", Density: "comfortable", Responsive: "stack"}},
		{"trading monitor",
			PlanInput{Intent: core.PageIntent{PrimaryTask: "monitor", Frequency: "high", InformationDensity: "high", Comparison: true}, Platform: "web", Domain: "trading", HasNav: true},
			Decision{PageType: "monitoring", Layout: "workspace", Navigation: "side", Density: "compact", Responsive: "context_to_overlay"}},
		{"mobile queue becomes list then detail",
			PlanInput{Intent: core.PageIntent{PrimaryTask: "process", Frequency: "high", Complexity: "medium"}, Platform: "android", Domain: "ticketing", HasNav: true},
			Decision{PageType: "workspace", Layout: "list", Navigation: "bottom", Density: "compact", Responsive: "same"}},
		{"finance overview",
			PlanInput{Intent: core.PageIntent{PrimaryTask: "overview", Frequency: "medium", InformationDensity: "medium"}, Platform: "web", Domain: "finance", HasNav: true},
			Decision{PageType: "dashboard", Layout: "dashboard", Navigation: "side", Density: "comfortable", Responsive: "stack"}},
	} {
		got := Decide(tc.in)
		got.Reasons = nil
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: got %+v, want %+v", tc.name, got, tc.want)
		}
	}
}

func TestDetectDomain(t *testing.T) {
	p := &core.Project{Story: "Helpdesk untuk tim support: agen menjawab tiket pelanggan sesuai SLA",
		Features: []core.Feature{{Name: "Antrean tiket", Description: "Agen melihat tiket dan eskalasi"}}}
	if got := DetectDomain(p); got != "ticketing" {
		t.Fatalf("domain = %q", got)
	}
	if got := DetectDomain(&core.Project{Story: "Aplikasi resep masakan keluarga"}); got != "" {
		t.Fatalf("no domain expected, got %q", got)
	}
}

// ticketWorkspace is a support agent's workspace that follows the rules.
func ticketWorkspace() core.Page {
	raw := `{"id": "PAGE-001", "name": "Tiket", "platform": "web", "role": "Agen support", "feature_id": "FEAT-001", "purpose": "Menyelesaikan tiket pelanggan",
 "kind": "workspace", "density": "compact", "primary_action": {"label": "Kirim balasan"},
 "intent": {"primary_task": "process", "frequency": "high", "complexity": "high", "information_density": "high"},
 "plan": {"layout_strategy": "master-detail", "responsive_strategy": "list_then_detail", "primary_focus": "conversation",
          "secondary_content": ["queue"], "supporting_content": ["customer"]},
 "layout": {"frame": "app", "nav": "side", "width": "full",
   "root": {"id": "main", "type": "stack", "gap": "space.sm", "children": ["title",
     {"id": "work", "type": "split", "ratio": "1:2", "children": ["queue",
       {"id": "detail", "type": "sidebar", "side": "end", "children": [{"id": "thread", "type": "stack", "children": ["conversation", "send"]}, "customer"]}]}]},
   "devices": {"mobile": {"nav": "bottom", "hide": ["queue"], "nodes": {"work": {"type": "stack"}, "detail": {"type": "stack"}}}}},
 "components": [{"id": "menu", "type": "nav", "label": "Menu", "items": ["Tiket"]},
   {"id": "title", "type": "header", "label": "Tiket"},
   {"id": "queue", "type": "table", "label": "Antrean", "columns": ["ID", "Subjek", "Prioritas", "Status", "SLA", "Agen"], "items": ["T-1042 | Gagal login | Tinggi | Baru | 2j | Rina"]},
   {"id": "conversation", "type": "list", "label": "Percakapan", "items": ["Pelanggan: tidak bisa login"]},
   {"id": "send", "type": "button", "label": "Kirim balasan", "emphasis": "action.primary"},
   {"id": "customer", "type": "card", "label": "Pelanggan", "items": ["Budi Santoso", "Paket Pro"]}]}`
	var pg core.Page
	if err := json.Unmarshal([]byte(raw), &pg); err != nil {
		panic(err)
	}
	return pg
}

func categories(issues []core.QAIssue) []string {
	var out []string
	for _, is := range issues {
		out = append(out, is.Severity+" "+is.Category+" "+is.Component+" "+is.Device)
	}
	return out
}

func TestLintGoodPage(t *testing.T) {
	pg := ticketWorkspace()
	if probs := core.ValidatePage(pg, map[string]bool{"PAGE-001": true}); len(probs) > 0 {
		t.Fatalf("fixture IR invalid: %v", probs)
	}
	if got := LintPage(pg, "ticketing"); len(got) > 0 {
		t.Fatalf("good page has findings: %v", categories(got))
	}
}

func TestLintFindings(t *testing.T) {
	has := func(t *testing.T, issues []core.QAIssue, want string) {
		t.Helper()
		if !slices.ContainsFunc(issues, func(is core.QAIssue) bool {
			return strings.HasPrefix(is.Severity+" "+is.Category+" "+is.Component+" "+is.Device, want) && is.Source == "lint" && is.Fix != ""
		}) {
			t.Errorf("missing %q in %v", want, categories(issues))
		}
	}
	for _, tc := range []struct {
		name string
		edit func(pg *core.Page)
		want string
	}{
		{"bad hierarchy: primary focus hidden", func(pg *core.Page) {
			pg.Layout.Devices["desktop"] = core.DeviceLayout{Hide: []string{"conversation"}}
		}, "major hierarchy conversation desktop"},
		{"bad hierarchy: supporting content first", func(pg *core.Page) {
			pg.Layout.Root.Children[1].Children[1].Children[0].Children = []core.LayoutNode{{Component: "send"}, {Component: "conversation"}}
			pg.Plan.Supporting = []string{"send"}
		}, "minor hierarchy send desktop"},
		{"bad hierarchy: primary action without focal point", func(pg *core.Page) {
			pg.Components[4].Emphasis = "action.secondary"
		}, "major hierarchy  desktop"},
		{"bad spacing", func(pg *core.Page) { pg.Layout.Root.Gap = "space.xl" }, "minor spacing main"},
		{"density against intent", func(pg *core.Page) { pg.Density = "spacious" }, "major density"},
		{"master-detail without split", func(pg *core.Page) {
			pg.Layout.Root.Children[1].Type = "stack"
			pg.Layout.Root.Children[1].Ratio = ""
			pg.Layout.Root.Children[1].Children[1].Type = "stack"
			pg.Layout.Root.Children[1].Children[1].Side = ""
		}, "major composition  desktop"},
		{"desktop/mobile mismatch: wide table shrunk", func(pg *core.Page) {
			pg.Layout.Devices["mobile"] = core.DeviceLayout{Nav: "bottom", Nodes: map[string]core.NodeChange{"work": {Type: "stack"}, "detail": {Type: "stack"}}}
		}, "major responsive queue mobile"},
		{"desktop/mobile mismatch: list and detail stacked", func(pg *core.Page) {
			pg.Layout.Devices["mobile"] = core.DeviceLayout{Nav: "bottom", Swap: map[string]string{"queue": "list"}, Nodes: map[string]core.NodeChange{"work": {Type: "stack"}, "detail": {Type: "stack"}}}
		}, "major responsive conversation mobile"},
		{"navigation without place", func(pg *core.Page) { pg.Layout.Nav = "none" }, "major navigation"},
		{"ai slop: meaningless stats", func(pg *core.Page) {
			for i := range 5 {
				id := "stat-" + string(rune('a'+i))
				pg.Components = append(pg.Components, core.PageComponent{ID: id, Type: "stat", Label: "Angka: 1"})
				pg.Layout.Root.Children = append(pg.Layout.Root.Children, core.LayoutNode{Component: id})
			}
		}, "major ai_slop"},
		{"ai slop: comparison in cards", func(pg *core.Page) {
			grid := core.LayoutNode{ID: "cards", Type: "grid", Columns: 3}
			for _, id := range []string{"c1", "c2", "c3"} {
				pg.Components = append(pg.Components, core.PageComponent{ID: id, Type: "card", Label: id})
				grid.Children = append(grid.Children, core.LayoutNode{Component: id})
			}
			pg.Components = slices.DeleteFunc(pg.Components, func(c core.PageComponent) bool { return c.ID == "queue" || c.ID == "conversation" })
			pg.Layout.Root.Children = []core.LayoutNode{{Component: "title"}, grid, {Component: "send"}, {Component: "customer"}}
			pg.Plan = &core.PagePlan{}
		}, "major composition cards"},
		{"ai slop: decorative cards", func(pg *core.Page) {
			for _, id := range []string{"c1", "c2", "c3"} {
				pg.Components = append(pg.Components, core.PageComponent{ID: id, Type: "card", Label: id})
				pg.Layout.Root.Children = append(pg.Layout.Root.Children, core.LayoutNode{Component: id})
			}
		}, "major ai_slop"},
		{"ai slop: component without reason", func(pg *core.Page) {
			pg.Plan.Reasons = map[string]string{"title": "judul"}
		}, "minor ai_slop queue"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pg := ticketWorkspace()
			if pg.Layout.Devices == nil {
				pg.Layout.Devices = map[string]core.DeviceLayout{}
			}
			tc.edit(&pg)
			has(t, LintPage(pg, "ticketing"), tc.want)
		})
	}
}

func TestNormPlan(t *testing.T) {
	pg := ticketWorkspace()
	pg.Intent = normIntent(&core.PageIntent{PrimaryTask: " Process ", Frequency: "HIGH", Complexity: "x"})
	if pg.Intent.PrimaryTask != "process" || pg.Intent.Frequency != "high" || pg.Intent.Complexity != "" {
		t.Fatalf("intent = %+v", pg.Intent)
	}
	got := normPlan(&core.PagePlan{Layout: "bento", Primary: "ghost", Secondary: []string{"queue", "ghost", "queue"}, Supporting: []string{"queue", "customer"}}, pg)
	if got.Layout != "master-detail" || got.Responsive != "list_then_detail" || got.Primary != "" ||
		!slices.Equal(got.Secondary, []string{"queue"}) || !slices.Equal(got.Supporting, []string{"customer"}) {
		t.Fatalf("plan = %+v", got)
	}
	if normIntent(&core.PageIntent{PrimaryTask: "dance"}) != nil {
		t.Fatal("empty intent must be nil")
	}
	pg.Intent = nil
	if normPlan(nil, pg) != nil {
		t.Fatal("no plan and no intent: nil (older designs stay as they are)")
	}
}

func TestDesignPromptHasIntelligence(t *testing.T) {
	p := &core.Project{Story: "Helpdesk tiket dengan SLA", Platform: &core.PlatformPlan{}}
	got := DesignPrompt(p)
	for _, want := range []string{"DESIGN KNOWLEDGE", "Domain detected: ticketing", "DESIGN PROCESS", `"intent": {"primary_task"`,
		`"plan": {"layout_strategy"`, "master-detail: master list", "Anti-slop", "IF the user performs the task frequently"} {
		if !strings.Contains(got, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
	if strings.Index(got, `"intent"`) > strings.Index(got, `"layout": {"frame"`) {
		t.Error("intent and plan must come before the layout")
	}
}

func initialQAProject() *core.Project {
	pg := ticketWorkspace()
	// The wide table is shrunk on mobile and both panes stack.
	pg.Layout.Devices["mobile"] = core.DeviceLayout{Nav: "bottom", Nodes: map[string]core.NodeChange{"work": {Type: "stack"}, "detail": {Type: "stack"}}}
	return &core.Project{
		Features: []core.Feature{{ID: "FEAT-001", Name: "Tiket"}},
		Design: &core.Design{Status: core.DesignDraft, Domain: "ticketing", Pages: []core.Page{pg},
			QA: &core.DesignQA{Status: "running", Round: 1, Shots: []core.QAShot{{PageID: "PAGE-001", Device: "desktop"}, {PageID: "PAGE-001", Device: "mobile"}}}},
	}
}

// A critic finding becomes an IR patch; the validator passes and the
// rendered composition changes on the device of the finding.
func TestCriticFixPatchesIR(t *testing.T) {
	p := initialQAProject()
	if !slices.ContainsFunc(LintPage(p.Design.Pages[0], "ticketing"), func(is core.QAIssue) bool { return is.Category == "responsive" }) {
		t.Fatal("fixture must break the responsive rules")
	}
	fixed := p.Design.Pages[0].Layout.Clone()
	fixed.Devices["mobile"] = core.DeviceLayout{Nav: "bottom", Hide: []string{"queue"}, Swap: map[string]string{"queue": "list"},
		Nodes: map[string]core.NodeChange{"work": {Type: "stack"}, "detail": {Type: "stack"}}}
	raw, _ := json.Marshal(map[string]any{"status": "fix", "notes": "mobile menumpuk", "issues": []map[string]string{
		{"page": "PAGE-001", "device": "mobile", "severity": "high", "category": "responsive", "component": "queue", "problem": "Antrean dan percakapan bertumpuk di HP", "fix": "Sembunyikan antrean di mobile"},
		{"page": "PAGE-001", "severity": "minor", "category": "spacing", "problem": "Make it cleaner", "fix": "Improve the UI"},
	}, "patches": []map[string]any{{"page": "PAGE-001", "operations": []map[string]any{{"op": "update", "page_fields": map[string]any{"layout": fixed}}}}}})
	again, err := ApplyQA(p, string(raw))
	if err != nil || !again {
		t.Fatalf("again=%v err=%v", again, err)
	}
	qa := p.Design.QA
	if qa.Round != 2 || qa.Status != "running" || len(qa.Next) != 1 || qa.Next[0] != (core.QAShot{PageID: "PAGE-001", Device: "mobile"}) {
		t.Fatalf("qa = %+v", qa)
	}
	critic := slices.DeleteFunc(slices.Clone(qa.Issues), func(is core.QAIssue) bool { return is.Source == "lint" })
	if len(critic) != 1 || critic[0].Severity != "major" || critic[0].Component != "queue" || critic[0].Device != "mobile" {
		t.Fatalf("vague finding dropped, severity mapped: %+v", critic)
	}
	pg := p.Design.Pages[0]
	if err := core.ValidateDesign(p.Design.Pages, p.Design.Pages); err != nil {
		t.Fatal(err)
	}
	if slices.ContainsFunc(LintPage(pg, "ticketing"), func(is core.QAIssue) bool { return is.Category == "responsive" }) {
		t.Fatalf("responsive finding left: %v", categories(LintPage(pg, "ticketing")))
	}
	if r, _ := pg.Layout.Resolve("mobile"); contains(r.Root, "queue") {
		t.Fatal("mobile still draws the queue")
	}
	if r, _ := pg.Layout.Resolve("desktop"); !contains(r.Root, "queue") {
		t.Fatal("desktop must keep the queue")
	}
	if ids, devs := QAPages(p); !slices.Equal(ids, []string{"PAGE-001"}) || !slices.Equal(devs, []string{"mobile"}) {
		t.Fatalf("next round renders %v %v", ids, devs)
	}

	// The patch is limited: no rounds beyond MaxDesignRevisions.
	qa.Round = MaxQARounds
	if again, _ := ApplyQA(p, string(raw)); again || qa.Status != "failed" {
		t.Fatalf("round limit: again=%v qa=%+v", again, qa)
	}
}

func TestCriticFixStaysOnRenderedPages(t *testing.T) {
	p := initialQAProject()
	other := ticketWorkspace()
	other.ID = "PAGE-002"
	p.Design.Pages = append(p.Design.Pages, other)
	again, _ := ApplyQA(p, `{"status": "fix", "patches": [{"page": "PAGE-002", "operations": [{"op": "remove", "target": 5}]}]}`)
	if again || p.Design.QA.Status != "failed" || !strings.Contains(p.Design.QA.Notes, "PAGE-002 di-patch") {
		t.Fatalf("again=%v qa=%+v", again, p.Design.QA)
	}
}

func TestInitialQAPagesBudget(t *testing.T) {
	p := &core.Project{Design: &core.Design{QA: &core.DesignQA{Round: 1}}}
	for i := range MaxInitialShots + 3 {
		p.Design.Pages = append(p.Design.Pages, core.Page{ID: "PAGE-" + string(rune('A'+i)), Platform: "web"})
	}
	p.Design.Pages[0].Platform = "android"
	ids, devs := QAPages(p)
	if len(ids) != MaxInitialShots || devs[0] != "mobile" || devs[1] != "desktop" {
		t.Fatalf("ids=%v devs=%v", ids, devs)
	}
	p.Design.Pages = p.Design.Pages[:3]
	ids, devs = QAPages(p)
	if strings.Join(devs, ",") != "mobile,desktop,desktop,mobile,mobile" || ids[3] != "PAGE-B" {
		t.Fatalf("base devices, then mobile of web pages: %v %v", ids, devs)
	}
}
