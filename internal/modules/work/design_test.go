package work

import (
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
)

const existingDesign = `{"rules": {"source": "new", "principles": ["Ikuti UI yang ada"], "ux_guidelines": ["Menu tetap di kiri"],
  "accessibility": ["AA"], "responsive": ["Sidebar jadi drawer di mobile"],
  "tokens": {"colors": [{"name": "action.primary", "value": "#000000"}, {"name": "status.info", "value": "#0ea5e9"}],
             "typography": [{"name": "type.body", "value": "Inter 14px"}], "spacing": [{"name": "space.md", "value": "16px"}]},
  "components": [{"name": "sidebar", "use": "navigasi"}]},
 "users": [{"role": "Operator", "platform": "web", "context": "harian", "tasks": ["ubah filter"]}],
 "goals": [{"id": "GOAL-1", "role": "Operator", "text": "Ubah filter pesanan"}],
 "navigation": [{"name": "Utama", "items": [{"label": "Pesanan", "page": "orders"}, {"label": "Anggaran", "page": "budget"}]}],
 "decisions": [{"subject": "orders", "choice": "detail", "why": ["filter cepat"], "basis": ["GOAL-1"], "source": "fact"},
               {"subject": "budget", "choice": "detail", "why": ["limit bulanan"], "basis": ["GOAL-1"], "source": "fact"}],
 "pages": [
  {"id": "orders", "name": "Pesanan", "platform": "web", "route": "/orders", "purpose": "Ubah filter pesanan", "goal": "GOAL-1", "pattern": "detail",
   "sections": [{"id": "filter", "name": "Filter", "content": ["Status"]}], "states": {"loading": "Memuat", "error": "Gagal"}},
  {"id": "budget", "name": "Anggaran", "platform": "web", "route": "/budget", "purpose": "Atur anggaran", "goal": "GOAL-1", "pattern": "detail",
   "sections": [{"id": "isi", "name": "Isi", "content": ["Limit bulanan"]}], "states": {"loading": "Memuat", "error": "Gagal"}}]}`

// An existing project's design follows its code: the code's tokens win,
// its components are listed for reuse and pages on its routes are marked
// existing.
func TestApplyDesignExisting(t *testing.T) {
	p := &core.Project{
		Platform: &core.PlatformPlan{Targets: []core.PlatformTarget{{Kind: core.PlatformWeb}}},
		Structure: &core.Structure{Mode: core.StructureExisting, UI: &core.ExistingUI{
			Routes:       []string{"/orders"},
			Components:   []string{"Sidebar (web/src/components/Sidebar.tsx)", "DataTable (web/src/components/DataTable.tsx)"},
			Tokens:       &core.DesignTokens{Colors: []core.Token{{Name: "action.primary", Value: "#7c3aed", Use: "dari web/src/index.css"}}},
			TokenSources: []string{"web/src/index.css"},
		}},
	}
	if err := ApplyDesign(p, existingDesign); err != nil {
		t.Fatal(err)
	}
	r := p.UI.Rules
	if r.Source != core.RulesExisting || len(r.Tokens.Colors) != 2 || r.Tokens.Colors[0].Value != "#7c3aed" || r.Tokens.Colors[1].Name != "status.info" {
		t.Fatalf("tokens = %+v", r.Tokens.Colors)
	}
	if len(r.Components) != 2 || r.Components[0].Source != "web/src/components/Sidebar.tsx" || r.Components[1].Name != "DataTable" {
		t.Fatalf("components = %+v", r.Components)
	}
	if pg := p.UI.Layout.Pages; !pg[0].Existing || pg[1].Existing {
		t.Fatalf("existing = %v %v", pg[0].Existing, pg[1].Existing)
	}
	if !strings.Contains(DesignPrompt(p), "CURRENT LAYOUT SPECIFICATION") {
		t.Fatal("a written design is revised, not rewritten")
	}
}

func TestApplyDesignRefusesIncomplete(t *testing.T) {
	p := &core.Project{Platform: &core.PlatformPlan{Targets: []core.PlatformTarget{{Kind: core.PlatformWeb}}}}
	if err := ApplyDesign(p, `{"pages": []}`); err == nil || p.UI != nil {
		t.Fatalf("incomplete design err = %v ui = %+v", err, p.UI)
	}
	bad := strings.Replace(existingDesign, `"pattern": "detail"`, `"pattern": "hero"`, 1)
	err := ApplyDesign(p, bad)
	if err == nil || !strings.Contains(err.Error(), `pattern "hero" tidak dikenal`) || p.UI != nil {
		t.Fatalf("unknown pattern err = %v", err)
	}
}

// A record's identity (symbol, name) maps to the supported card-title
// presentation; the new value/delta presentations pass through unchanged.
func TestNormPresentationAliases(t *testing.T) {
	for in, want := range map[string]string{
		"identity":     "card-title",
		"card-title":   "card-title",
		"card-value":   "card-value",
		"delta":        "delta",
		"row-action":   "row-action",
		"table-column": "table-column",
	} {
		if got := normPresentation(in); got != want {
			t.Errorf("normPresentation(%q) = %q, want %q", in, got, want)
		}
	}
}

// The design prompt tells the designer to give every page a decision (its
// pattern and why, plus form concerns) and documents the row-action
// presentation, so the generator produces complete, valid field mappings.
func TestDesignPromptCoversEveryPageAndRowAction(t *testing.T) {
	p := &core.Project{Story: "Aplikasi register akun", Platform: &core.PlatformPlan{Targets: []core.PlatformTarget{{Kind: core.PlatformWeb}}}}
	prompt := DesignPrompt(p)
	for _, want := range []string{"row-action", "Every page must have at least one decision", "No page is left without a decision", "submit state and error feedback"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
}

// The design language is seeded from the UI template (ui-template-ai-friendly),
// not from per-product presets or contextual rules.
func TestDesignPromptSeedsFromTemplate(t *testing.T) {
	p := &core.Project{Story: "Aplikasi data pasar", Platform: &core.PlatformPlan{Targets: []core.PlatformTarget{{Kind: core.PlatformWeb}}}}
	prompt := DesignPrompt(p)
	for _, want := range []string{"UI DESIGN TEMPLATE", "PRIORITAS KEPUTUSAN", "PAGE PATTERNS", "AKSESIBILITAS", "RESPONSIF"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing template marker %q", want)
		}
	}
	// The old per-product presets and contextual rules are gone.
	for _, gone := range []string{"PRODUCT CONTEXT:", "Clean SaaS Dashboard", "Enterprise Admin"} {
		if strings.Contains(prompt, gone) {
			t.Errorf("prompt still carries removed guidance %q", gone)
		}
	}
}

// The generator fills the states every pattern requires, so a table page that
// omits `empty` still lands complete; and a design aspect stays in the
// decision's topic, never in the subject.
func TestApplyDesignFillsRequiredStatesAndKeepsTopics(t *testing.T) {
	p := &core.Project{Platform: &core.PlatformPlan{Targets: []core.PlatformTarget{{Kind: core.PlatformWeb}}}}
	raw := `{"rules": {"source": "new", "principles": ["Satu tugas per halaman"], "ux_guidelines": ["Filter di atas"],
	  "accessibility": ["AA"], "responsive": ["Tabel scroll di mobile"],
	  "tokens": {"colors": [{"name": "action.primary", "value": "#000000"}], "typography": [{"name": "type.body", "value": "Inter 14px"}],
	              "spacing": [{"name": "space.md", "value": "16px"}]},
	  "components": [{"name": "DataTable", "use": "daftar data"}]},
	 "users": [{"role": "Operator", "platform": "web", "tasks": ["saring"]}],
	 "goals": [{"id": "GOAL-1", "role": "Operator", "text": "Saring data pasar"}],
	 "navigation": [{"name": "Utama", "items": [{"label": "Screener", "page": "screener"}]}],
	 "decisions": [
	   {"subject": "screener", "topic": "kepadatan", "choice": "compact", "why": ["banyak baris"], "basis": ["GOAL-1"], "source": "fact"},
	   {"subject": "screener", "topic": "status data", "choice": "badge", "why": ["status jelas"], "basis": ["GOAL-1"], "source": "fact"},
	   {"subject": "screener", "topic": "perubahan harga", "choice": "warna naik/turun", "why": ["arah jelas"], "basis": ["GOAL-1"], "source": "fact"},
	   {"subject": "screener", "topic": "tick real-time", "choice": "highlight", "why": ["perubahan terlihat"], "basis": ["GOAL-1"], "source": "fact"},
	   {"subject": "*", "topic": "navigation", "choice": "sidebar", "why": ["grup tugas"], "basis": ["GOAL-1"], "source": "fact"}],
	 "pages": [{"id": "screener", "name": "Screener", "platform": "web", "route": "/screener", "purpose": "Saring", "goal": "GOAL-1", "pattern": "table",
	   "sections": [{"id": "tabel", "name": "Tabel", "content": ["Simbol, Harga, Perubahan"]}],
	   "states": {"loading": "Memuat"}}]}`
	if err := ApplyDesign(p, raw); err != nil {
		t.Fatal(err)
	}
	pg := p.UI.Layout.Pages[0]
	for _, st := range []string{"loading", "empty", "error"} {
		if pg.States[st] == "" {
			t.Errorf("required state %q not filled: %+v", st, pg.States)
		}
	}
	if len(p.UI.Layout.Decisions) != 5 {
		t.Fatalf("decisions = %+v", p.UI.Layout.Decisions)
	}
	topics := map[string]bool{}
	for _, d := range p.UI.Layout.Decisions {
		topics[d.Topic] = true
	}
	for _, tp := range []string{"kepadatan", "status data", "perubahan harga", "tick real-time", "navigation"} {
		if !topics[tp] {
			t.Errorf("topic %q lost: %+v", tp, p.UI.Layout.Decisions)
		}
	}
}

// A user-provided design reference is injected into the design prompt with
// instructions to derive concrete rules and record the reference — never to
// answer with the name alone.
func TestDesignPromptCarriesDesignReference(t *testing.T) {
	p := &core.Project{Story: "Aplikasi belajar bahasa", Platform: &core.PlatformPlan{Targets: []core.PlatformTarget{{Kind: core.PlatformWeb}}},
		Survey: &core.Survey{DesignReference: "Duolingo"}}
	prompt := DesignPrompt(p)
	for _, want := range []string{"DESIGN REFERENCE (the user asked", "Duolingo", "rules.reference", "Adapt, never clone", `"reference": ""`} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
	// No reference, no section.
	p2 := &core.Project{Story: "Aplikasi belajar bahasa", Platform: &core.PlatformPlan{Targets: []core.PlatformTarget{{Kind: core.PlatformWeb}}}}
	if strings.Contains(DesignPrompt(p2), "DESIGN REFERENCE (the user asked") {
		t.Fatal("no reference given, but section present")
	}
}

// A reference-derived design records the reference and lands concrete tokens
// and components, not just the reference name.
func TestApplyDesignRecordsReferenceWithConcreteRules(t *testing.T) {
	p := &core.Project{Platform: &core.PlatformPlan{Targets: []core.PlatformTarget{{Kind: core.PlatformWeb}}},
		Survey: &core.Survey{DesignReference: "Duolingo"}}
	raw := `{"rules": {"source": "new", "reference": "Duolingo", "direction": "Ceria, bulat, progres bertahap",
	  "principles": ["Belajar bertahap: satu pelajaran pendek per sesi", "Progres selalu terlihat"],
	  "ux_guidelines": ["Umpan balik langsung setelah jawaban"], "accessibility": ["Kontras AA"], "responsive": ["Satu kolom di mobile"],
	  "tokens": {"colors": [{"name": "action.primary", "value": "#58cc02"}], "typography": [{"name": "type.body", "value": "Nunito 16px/24px 600"}],
	              "spacing": [{"name": "space.md", "value": "16px"}], "radius": [{"name": "radius.md", "value": "12px"}]},
	  "components": [{"name": "ProgressBar", "use": "progres pelajaran", "variants": ["default", "crown"]},
	                 {"name": "LessonCard", "use": "satu pelajaran", "variants": ["locked", "active"]}]},
	 "users": [{"role": "Pelajar", "platform": "web", "tasks": ["ambil pelajaran"]}],
	 "goals": [{"id": "GOAL-1", "role": "Pelajar", "text": "Selesaikan pelajaran harian"}],
	 "navigation": [{"name": "Belajar", "items": [{"label": "Pelajaran", "page": "pelajaran"}]}],
	 "decisions": [{"subject": "pelajaran", "choice": "cards", "why": ["browse set kecil dengan identitas visual"], "basis": ["GOAL-1"], "source": "fact"}],
	 "pages": [{"id": "pelajaran", "name": "Pelajaran", "platform": "web", "route": "/pelajaran", "purpose": "Ambil pelajaran", "goal": "GOAL-1", "pattern": "cards",
	   "sections": [{"id": "daftar", "name": "Daftar pelajaran", "content": ["Kartu pelajaran"], "components": ["LessonCard"]}],
	   "states": {"loading": "Memuat"}}]}`
	if err := ApplyDesign(p, raw); err != nil {
		t.Fatal(err)
	}
	r := p.UI.Rules
	if r.Reference != "Duolingo" {
		t.Fatalf("reference = %q, want Duolingo", r.Reference)
	}
	if len(r.Tokens.Colors) != 1 || len(r.Tokens.Radius) != 1 || len(r.Components) != 2 {
		t.Fatalf("concrete rules not landed: colors=%d radius=%d components=%d", len(r.Tokens.Colors), len(r.Tokens.Radius), len(r.Components))
	}
	// A complete reference-derived design has no errors.
	if errs := core.Errors(p.DesignIssues(nil)); len(errs) != 0 {
		t.Fatalf("reference design has errors: %+v", errs)
	}
}

// A design that ignores the reference is refused as a whole: the rules must
// record the reference before the design lands.
func TestApplyDesignRefusesMissingReference(t *testing.T) {
	p := &core.Project{Platform: &core.PlatformPlan{Targets: []core.PlatformTarget{{Kind: core.PlatformWeb}}},
		Survey: &core.Survey{DesignReference: "Duolingo"}}
	// existingDesign has no reference field; the reference check must refuse it.
	err := ApplyDesign(p, existingDesign)
	if err == nil || !strings.Contains(err.Error(), "referensi desain") {
		t.Fatalf("err = %v, want missing-reference refusal", err)
	}
}
