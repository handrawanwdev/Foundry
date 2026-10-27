package core

import (
	"slices"
	"strings"
	"testing"
)

func uxProject() *Project {
	return &Project{
		Platform:  &PlatformPlan{Targets: []PlatformTarget{{Kind: PlatformWeb}, {Kind: PlatformAndroid}}},
		Features:  []Feature{{ID: "FEAT-001", Name: "Antrean", Platforms: []string{PlatformWeb}, RequirementIDs: []string{"REQ-001"}}},
		Flows:     []Flow{{ID: "FLOW-001", Name: "Cek antrean", FeatureID: "FEAT-001", Role: "Operator", Steps: []string{"Buka", "Filter", "Lihat"}, Permissions: []string{"queue:read"}}},
		Blueprint: &Blueprint{Database: DatabaseArch{Entities: []Entity{{Name: "Queue", Fields: []string{"id", "status", "patient"}}}}},
	}
}

func uxDesign() *UIDesign {
	return &UIDesign{Format: UIDesignFormat, Rules: &DesignRules{Source: RulesNew,
		Principles: []string{"Operator membandingkan banyak baris"}, UX: []string{"Filter di atas tabel"},
		Accessibility: []string{"Kontras AA"}, Responsive: []string{"Tabel jadi list di mobile"},
		Tokens: DesignTokens{Colors: []Token{{Name: "action.primary", Value: "#1f6feb"}},
			Typography: []Token{{Name: "type.body", Value: "Inter 14px"}}, Spacing: []Token{{Name: "space.md", Value: "16px"}}},
		Components: []ComponentRule{{Name: "DataTable", Use: "daftar data"}}},
		Layout: &LayoutSpec{
			Users:      []UIUser{{Role: "Operator", Platform: PlatformWeb, Context: "harian", Tasks: []string{"cek antrean"}}},
			Goals:      []UIGoal{{ID: "GOAL-1", Role: "Operator", Text: "Cek antrean", Priority: "must"}},
			Navigation: []NavGroup{{Name: "Operasional", Items: []NavItem{{Label: "Antrean", Page: "antrean", Primary: true}}}},
			Decisions: []DesignDecision{{Subject: "antrean", Choice: "table", Why: []string{"operator membandingkan banyak baris"},
				Basis: []string{"GOAL-1", "FLOW-001", "Queue", "queue:read", "web"}, Source: "fact"}},
			Pages: []LayoutPage{{ID: "antrean", Name: "Antrean", Platform: PlatformWeb, Route: "/antrean", FeatureID: "FEAT-001",
				Purpose: "Cek antrean", Goal: "GOAL-1", Pattern: "table", PrimaryAction: "Panggil",
				Fields:   []FieldRule{{Field: "Queue.status", Label: "Status", Presentation: "table-column"}},
				Sections: []Section{{ID: "tabel", Name: "Tabel", Content: []string{"No, Pasien, Status"}, Components: []string{"DataTable"}}},
				States:   map[string]string{"loading": "skeleton", "empty": "kosong", "error": "gagal"},
				Data:     []string{"Queue: antrean hari ini"}}},
		}}
}

func TestDesignIssuesRequiresAutonomousUXLayer(t *testing.T) {
	p := uxProject()
	d := uxDesign()
	issues := p.DesignIssues(d)
	if len(Errors(issues)) != 0 {
		t.Fatalf("complete design has errors: %+v", issues)
	}

	bare := uxDesign()
	bare.Layout.Users, bare.Layout.Goals, bare.Layout.Navigation, bare.Layout.Decisions = nil, nil, nil, nil
	bare.Layout.Pages[0].Goal = ""
	msgs := issueMessages(p.DesignIssues(bare))
	for _, want := range []string{"users", "goals", "navigation", "goal", "keputusan desain"} {
		if !strings.Contains(msgs, want) {
			t.Errorf("missing %q in\n%s", want, msgs)
		}
	}
}

func TestDesignDecisionsMustBeTraceable(t *testing.T) {
	p := uxProject()
	d := uxDesign()
	d.Layout.Decisions[0].Basis = []string{"GHOST"}
	msgs := issueMessages(p.DesignIssues(d))
	if !strings.Contains(msgs, `basis "GHOST" tidak ada`) {
		t.Fatalf("issues = %s", msgs)
	}
	d = uxDesign()
	d.Layout.Decisions[0].Why = nil
	if msgs := issueMessages(p.DesignIssues(d)); !strings.Contains(msgs, "alasan (why) belum diisi") {
		t.Fatalf("issues = %s", msgs)
	}
}

// A design aspect (navigation, density, data status, price change, real-time
// updates) is a topic, not the subject: the subject stays the affected
// element, and a subject that names no element is refused.
func TestDesignDecisionSubjectAndTopic(t *testing.T) {
	p := uxProject()
	d := uxDesign()
	d.Layout.Decisions = []DesignDecision{
		{Subject: "antrean", Topic: "kepadatan", Choice: "compact", Why: []string{"operator butuh padat"}, Basis: []string{"GOAL-1"}, Source: "fact"},
		{Subject: "ghost", Topic: "status data", Choice: "badge", Why: []string{"status jelas"}, Basis: []string{"GOAL-1"}, Source: "fact"},
	}
	msgs := issueMessages(p.DesignIssues(d))
	if strings.Contains(msgs, "kepadatan") || strings.Contains(msgs, "status data") {
		t.Fatalf("topic wrongly rejected:\n%s", msgs)
	}
	if !strings.Contains(msgs, `subjek "ghost" bukan halaman, section, atau komponen`) {
		t.Fatalf("missing subject reference error:\n%s", msgs)
	}
}

// row-action is a real table/list concept: an action on each row, not a data
// value. It is a valid presentation, never a reason to refuse the design.
func TestRowActionPresentationValid(t *testing.T) {
	p := uxProject()
	d := uxDesign()
	d.Layout.Pages[0].Fields = []FieldRule{
		{Field: "WatchlistItem", Label: "Item", Presentation: "row-action", Note: "hapus dari watchlist"},
		{Field: "Alert", Label: "Peringatan", Presentation: "row-action", Note: "akui atau abaikan"},
	}
	if errs := Errors(p.DesignIssues(d)); len(errs) != 0 {
		t.Fatalf("row-action field rejected: %+v", errs)
	}
}

// card-value (a prominent card value) and delta (a change indicator) are
// real concepts, valid presentations, never a reason to refuse the design.
func TestValueAndDeltaPresentationsValid(t *testing.T) {
	p := uxProject()
	d := uxDesign()
	d.Layout.Pages[0].Fields = []FieldRule{
		{Field: "Asset.current_price", Presentation: "card-value"},
		{Field: "Asset.price_change_pct_24h", Presentation: "delta"},
	}
	if errs := Errors(p.DesignIssues(d)); len(errs) != 0 {
		t.Fatalf("card-value/delta field rejected: %+v", errs)
	}
}

func TestDesignLegacyFormatOnlyWarns(t *testing.T) {
	p := uxProject()
	d := uxDesign()
	d.Format = 0
	d.Layout.Users, d.Layout.Goals, d.Layout.Navigation, d.Layout.Decisions = nil, nil, nil, nil
	d.Format = 0
	if errs := Errors(p.DesignIssues(d)); len(errs) != 0 {
		t.Fatalf("legacy design blocked: %+v", errs)
	}
}

func TestDesignAdviceDependsOnPlatformAndShape(t *testing.T) {
	p := uxProject()
	d := uxDesign()
	d.Layout.Pages[0].Platform = PlatformAndroid
	d.Layout.Pages[0].Sections[0].Content = []string{"No, Pasien, Dokter, Tanggal, Status, Aksi"}
	msgs := issueMessages(p.DesignIssues(d))
	if !strings.Contains(msgs, "tabel 6 kolom") {
		t.Fatalf("mobile table advice missing:\n%s", msgs)
	}
	d.Layout.Pages[0].Platform = PlatformWeb
	if msgs := issueMessages(p.DesignIssues(d)); strings.Contains(msgs, "tabel 6 kolom") {
		t.Fatalf("web table wrongly advised:\n%s", msgs)
	}

	// A long dependent task on form pages is advised to use a wizard; the
	// same pages with the wizard pattern are not.
	p.Flows[0].Steps = []string{"Pilih dokter", "Pilih tanggal", "Pilih jam", "Isi catatan", "Konfirmasi"}
	p.Flows[0].PageIDs = []string{"antrean"}
	d.Layout.Pages[0].Platform = PlatformWeb
	d.Layout.Pages[0].Pattern = "form"
	d.Layout.Pages[0].States = map[string]string{"error": "x", "success": "x", "disabled": "x"}
	msgs = issueMessages(p.DesignIssues(d))
	if !strings.Contains(msgs, "pertimbangkan wizard") {
		t.Fatalf("wizard advice missing:\n%s", msgs)
	}
	d.Layout.Pages[0].Pattern = "wizard"
	if msgs := issueMessages(p.DesignIssues(d)); strings.Contains(msgs, "pertimbangkan wizard") {
		t.Fatalf("wizard wrongly advised:\n%s", msgs)
	}
}

func issueMessages(issues []Issue) string {
	var out []string
	for _, is := range issues {
		out = append(out, is.Where+": "+is.Message)
	}
	return strings.Join(out, "\n")
}

// The design reference a user asked for must be recorded on the rules, so
// the derived tokens, components and patterns stay traceable to the template.
func TestDesignReferenceMustBeRecorded(t *testing.T) {
	p := uxProject()
	p.Survey = &Survey{DesignReference: "Duolingo"}
	d := uxDesign()
	if msgs := issueMessages(p.DesignIssues(d)); !strings.Contains(msgs, "referensi desain") {
		t.Fatalf("missing reference error:\n%s", msgs)
	}
	d.Rules.Reference = "Duolingo"
	if errs := Errors(p.DesignIssues(d)); len(errs) != 0 {
		t.Fatalf("recorded reference still errors: %+v", errs)
	}
	// No reference asked, no requirement.
	p.Survey = &Survey{}
	if msgs := issueMessages(p.DesignIssues(uxDesign())); strings.Contains(msgs, "referensi desain") {
		t.Fatalf("no reference asked, but flagged:\n%s", msgs)
	}
}

func TestVisualSummaryNeverInventsPass(t *testing.T) {
	pages := []LayoutPage{{ID: "a"}, {ID: "b"}}
	var v *VisualAudit
	s := v.Summary(pages)
	if s.Untested != 2 || s.Pass != 0 || !slices.Equal(s.NotChecked, []string{"a", "b"}) {
		t.Fatalf("summary = %+v", s)
	}
	v = &VisualAudit{Pages: map[string]PageAudit{"a": {Page: "a", Status: VisualFail,
		Findings: []VisualFinding{{Severity: SeverityHigh, Page: "a"}}}}}
	s = v.Summary(pages)
	if s.Fail != 1 || s.Untested != 1 || s.OpenHigh != 1 {
		t.Fatalf("summary = %+v", s)
	}
}
