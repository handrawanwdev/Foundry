package work

import (
	"slices"
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
		"title":        "card-title",
		"card-title":   "card-title",
		"card-value":   "card-value",
		"kpi":          "card-value",
		"delta":        "delta",
		"row-action":   "row-action",
		"button":       "row-action",
		"table-column": "table-column",
		"column":       "table-column",
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

// A design that omitted rules.reference still lands: Foundry records the
// survey design reference so the rules stay traceable. Validation still
// requires the field; this fills the known value.
func TestApplyDesignFillsMissingReference(t *testing.T) {
	p := &core.Project{Platform: &core.PlatformPlan{Targets: []core.PlatformTarget{{Kind: core.PlatformWeb}}},
		Survey: &core.Survey{DesignReference: "Duolingo"}}
	if err := ApplyDesign(p, existingDesign); err != nil {
		t.Fatal(err)
	}
	if p.UI.Rules == nil || p.UI.Rules.Reference != "Duolingo" {
		t.Fatalf("reference = %q, want Duolingo", p.UI.Rules.Reference)
	}
	if errs := core.Errors(p.DesignIssues(nil)); len(errs) != 0 {
		t.Fatalf("filled reference still errors: %+v", errs)
	}
}

// Pages that name a shell the answer never emitted still land: Foundry
// synthesizes the missing shells. Designer-written shells stay as they are.
func TestApplyDesignFillsMissingShells(t *testing.T) {
	p := webProject()
	raw := `{"rules": ` + designRulesJSON() + `,
	 "pages": [
	  {"id": "masuk", "name": "Masuk", "platform": "web", "route": "/masuk", "purpose": "Masuk", "pattern": "auth", "shell": "auth",
	   "sections": [{"id": "form", "name": "Form", "content": ["Email, Kata sandi"]}], "states": {"error": "Gagal", "disabled": "Kirim"}},
	  {"id": "screener", "name": "Screener", "platform": "web", "route": "/screener", "purpose": "Saring", "pattern": "table", "shell": "app",
	   "feature_id": "FEAT-001",
	   "sections": [{"id": "tabel", "name": "Tabel", "content": ["Simbol, Harga"]}]}]}`
	if err := ApplyDesign(p, raw); err != nil {
		t.Fatal(err)
	}
	shells := map[string]core.LayoutShell{}
	for _, sh := range p.UI.Layout.Shells {
		shells[sh.ID] = sh
	}
	if sh, ok := shells["auth"]; !ok || sh.Pattern != "auth" || sh.Name == "" || len(sh.Regions) == 0 {
		t.Fatalf("auth shell = %+v", p.UI.Layout.Shells)
	}
	if sh, ok := shells["app"]; !ok || sh.Pattern != "sidebar" || sh.Name == "" || len(sh.Regions) == 0 {
		t.Fatalf("app shell = %+v", p.UI.Layout.Shells)
	}
	if errs := core.Errors(p.DesignIssues(nil)); len(errs) != 0 {
		t.Fatalf("filled shells still error: %+v", errs)
	}

	p2 := webProject()
	kept := `{"rules": ` + designRulesJSON() + `,
	 "shells": [{"id": "app", "name": "Kerangka", "platform": "web", "pattern": "navbar",
	   "regions": [{"id": "top", "name": "Atas", "content": ["Logo"]}]}],
	 "pages": [{"id": "screener", "name": "Screener", "platform": "web", "route": "/screener", "purpose": "Saring", "pattern": "table", "shell": "app",
	   "feature_id": "FEAT-001",
	   "sections": [{"id": "tabel", "name": "Tabel", "content": ["Simbol, Harga"]}]}]}`
	if err := ApplyDesign(p2, kept); err != nil {
		t.Fatal(err)
	}
	if n := len(p2.UI.Layout.Shells); n != 1 {
		t.Fatalf("designer shells overwritten: n=%d %+v", n, p2.UI.Layout.Shells)
	}
	sh := p2.UI.Layout.Shells[0]
	if sh.ID != "app" || sh.Name != "Kerangka" || sh.Pattern != "navbar" || sh.Regions[0].ID != "top" {
		t.Fatalf("designer shell mutated: %+v", sh)
	}
}

// MiniTask-shaped answer: long survey reference omitted, pages name shells
// that were never emitted. Both gaps are recoverable; the design lands.
func TestApplyDesignLandsLongReferenceAndNamedShells(t *testing.T) {
	ref := "Gunakan Todoist sebagai referensi desain dan pengalaman pengguna. Adaptasi karakteristik berikut: layout dashboard yang bersih, sidebar, daftar tugas sebagai fokus."
	p := webProject()
	p.Survey = &core.Survey{DesignReference: ref}
	raw := `{"rules": ` + designRulesJSON() + `,
	 "pages": [
	  {"id": "masuk", "name": "Masuk", "platform": "web", "route": "/masuk", "purpose": "Masuk", "pattern": "auth", "shell": "auth", "access": "public",
	   "sections": [{"id": "form", "name": "Form", "content": ["Email"]}], "states": {"error": "Gagal", "disabled": "Kirim"}},
	  {"id": "dasbor", "name": "Dasbor", "platform": "web", "route": "/", "purpose": "Lihat tugas", "pattern": "list", "shell": "app", "access": "auth",
	   "feature_id": "FEAT-001",
	   "sections": [{"id": "daftar", "name": "Daftar", "content": ["Tugas"]}], "states": {"loading": "x", "empty": "x", "error": "x"}}]}`
	if err := ApplyDesign(p, raw); err != nil {
		t.Fatal(err)
	}
	if p.UI.Rules.Reference != ref {
		t.Fatalf("reference = %q", p.UI.Rules.Reference)
	}
	got := map[string]bool{}
	for _, sh := range p.UI.Layout.Shells {
		got[sh.ID] = true
	}
	if !got["auth"] || !got["app"] {
		t.Fatalf("shells = %+v", p.UI.Layout.Shells)
	}
	if errs := core.Errors(p.DesignIssues(nil)); len(errs) != 0 {
		t.Fatalf("still invalid: %+v", errs)
	}
}

// Empty navigation groups (one named group per page, no items) are dropped
// and rebuilt from home destinations: auth/form/public pages stay off the menu.
func TestApplyDesignFillsEmptyNavGroups(t *testing.T) {
	p := webProject()
	raw := `{"rules": ` + designRulesJSON() + `,
	 "pages": [
	  {"id": "daftar", "name": "Daftar", "platform": "web", "route": "/daftar", "purpose": "Daftar", "pattern": "auth", "access": "public", "shell": "auth",
	   "sections": [{"id": "form", "name": "Form", "content": ["Email"]}], "states": {"error": "x", "disabled": "x"}},
	  {"id": "masuk", "name": "Masuk", "platform": "web", "route": "/masuk", "purpose": "Masuk", "pattern": "auth", "access": "public", "shell": "auth",
	   "sections": [{"id": "form", "name": "Form", "content": ["Email"]}], "states": {"error": "x", "disabled": "x"}},
	  {"id": "dasbor", "name": "Dasbor", "platform": "web", "route": "/", "purpose": "Lihat tugas", "pattern": "list", "access": "auth", "shell": "app",
	   "feature_id": "FEAT-001",
	   "sections": [{"id": "daftar", "name": "Daftar", "content": ["Tugas"]}], "states": {"loading": "x", "empty": "x", "error": "x"}},
	  {"id": "tambah-tugas", "name": "Tambah tugas", "platform": "web", "route": "/tugas/baru", "purpose": "Buat tugas", "pattern": "form", "access": "auth", "shell": "app",
	   "sections": [{"id": "form", "name": "Form", "content": ["Judul"]}], "states": {"error": "x", "success": "x", "disabled": "x"}},
	  {"id": "ubah-tugas", "name": "Ubah tugas", "platform": "web", "route": "/tugas/ubah", "purpose": "Ubah tugas", "pattern": "form", "access": "auth", "shell": "app",
	   "sections": [{"id": "form", "name": "Form", "content": ["Judul"]}], "states": {"error": "x", "success": "x", "disabled": "x"}}],
	 "navigation": [
	  {"name": "Daftar"}, {"name": "Masuk"}, {"name": "Tugas"}, {"name": "Tambah tugas"}, {"name": "Ubah tugas"}]}`
	if err := ApplyDesign(p, raw); err != nil {
		t.Fatal(err)
	}
	nav := p.UI.Layout.Navigation
	if len(nav) != 1 || nav[0].Name == "" || len(nav[0].Items) != 1 || nav[0].Items[0].Page != "dasbor" {
		t.Fatalf("nav = %+v", nav)
	}
	if errs := core.Errors(p.DesignIssues(nil)); len(errs) != 0 {
		t.Fatalf("still invalid: %+v", errs)
	}
}

// A designer-written navigation with valid items is kept; empty sibling groups
// and items that point nowhere are dropped.
func TestApplyDesignKeepsValidNavDropsEmptyGroups(t *testing.T) {
	p := webProject()
	raw := `{"rules": ` + designRulesJSON() + `,
	 "pages": [
	  {"id": "screener", "name": "Screener", "platform": "web", "route": "/screener", "purpose": "Saring", "pattern": "table",
	   "feature_id": "FEAT-001",
	   "sections": [{"id": "tabel", "name": "Tabel", "content": ["Simbol"]}], "states": {"loading": "x", "empty": "x", "error": "x"}},
	  {"id": "detail", "name": "Detail", "platform": "web", "route": "/detail", "purpose": "Lihat", "pattern": "detail",
	   "sections": [{"id": "isi", "name": "Isi", "content": ["Harga"]}], "states": {"loading": "x", "error": "x"}}],
	 "navigation": [
	  {"name": "Pasar", "items": [{"label": "Screener", "page": "screener", "primary": true}, {"label": "Hantu", "page": "ghost"}]},
	  {"name": "Kosong"}]}`
	if err := ApplyDesign(p, raw); err != nil {
		t.Fatal(err)
	}
	nav := p.UI.Layout.Navigation
	if len(nav) != 1 || nav[0].Name != "Pasar" || len(nav[0].Items) != 1 || nav[0].Items[0].Page != "screener" {
		t.Fatalf("nav = %+v", nav)
	}
}

// Platform rules for a platform the project did not select are dropped.
func TestApplyDesignDropsUnselectedPlatformRules(t *testing.T) {
	p := webProject()
	raw := `{"rules": {"source": "new", "principles": ["Satu tugas"], "ux_guidelines": ["Filter di atas"],
	  "accessibility": ["AA"], "responsive": ["Tabel scroll"],
	  "tokens": {"colors": [{"name": "action.primary", "value": "#000"}], "typography": [{"name": "type.body", "value": "Inter 14px"}],
	              "spacing": [{"name": "space.md", "value": "16px"}]},
	  "components": [{"name": "DataTable", "use": "daftar"}],
	  "platforms": [{"platform": "android", "rules": ["Material 3"]}, {"platform": "web", "rules": ["Fokus keyboard"]}]},
	 "pages": [{"id": "screener", "name": "Screener", "platform": "web", "route": "/screener", "purpose": "Saring", "pattern": "table",
	   "feature_id": "FEAT-001",
	   "sections": [{"id": "tabel", "name": "Tabel", "content": ["Simbol, Harga"]}]}]}`
	if err := ApplyDesign(p, raw); err != nil {
		t.Fatal(err)
	}
	got := p.UI.Rules.Platforms
	if len(got) != 1 || got[0].Platform != "web" {
		t.Fatalf("platforms = %+v", got)
	}
}

func clinicPromptProject() *core.Project {
	return &core.Project{
		Name:  "Klinik Sehat",
		Story: "Aplikasi klinik pratama: pasien membuat janji dari HP, admin mengelola jadwal di web.",
		Platform: &core.PlatformPlan{Targets: []core.PlatformTarget{
			{Kind: core.PlatformWeb}, {Kind: core.PlatformAndroid}, {Kind: core.PlatformBackend},
		}},
		Discovery: &core.Discovery{
			Roles: []core.Role{
				{Name: "Pasien", Platform: "android", Description: "membuat janji"},
				{Name: "Admin", Platform: "web", Description: "kelola jadwal"},
			},
			Users:     []string{"Pasien", "Admin"},
			Workflows: []string{"Pasien membuat janji"},
		},
		Features: []core.Feature{
			{ID: "FEAT-001", Name: "Janji Temu", Description: "Pasien membuat janji", Platforms: []string{"android", "backend"},
				SubFeatures: []core.SubFeature{{Name: "Cari dokter"}, {Name: "Buat janji"}}},
			{ID: "FEAT-002", Name: "Jadwal Dokter", Description: "Admin kelola jadwal", Platforms: []string{"web", "backend"}},
		},
		Flows: []core.Flow{
			{ID: "FLOW-001", Name: "Buat janji", FeatureID: "FEAT-001", Role: "Pasien", Goal: "Mendapat jadwal periksa",
				Steps: []string{"Pilih dokter", "Pilih jadwal", "Konfirmasi"}, Permissions: []string{"appointment:create"},
				Failures: []string{"Jadwal sudah terisi"}},
		},
		Blueprint: &core.Blueprint{Database: core.DatabaseArch{
			Entities: []core.Entity{
				{Name: "Appointment", Fields: []string{"id", "pasien", "dokter", "waktu", "status"}, Module: "clinic"},
				{Name: "Doctor", Fields: []string{"id", "name", "specialty"}, Module: "clinic"},
			},
		}},
	}
}

// The generate prompt stays compact: no duplicate inventory dump, an output
// budget, and a size cap so the model spends tokens on the answer.
func TestDesignPromptStaysCompact(t *testing.T) {
	prompt := DesignPrompt(clinicPromptProject())
	for _, gone := range []string{"ROLES:\n", "FLOWS:\n"} {
		if strings.Contains(prompt, gone) {
			t.Errorf("prompt still dumps duplicate %q", gone)
		}
	}
	for _, want := range []string{"OUTPUT DISCIPLINE", "FEATURES (pages.feature_id)", "one page per user task", "Emit compact JSON",
		"never one page per feature", "Never a group without items", "do not copy its product"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
	if n := len(prompt); n > 18000 {
		t.Errorf("clinic generate prompt is %d bytes; want ≤ 18000 so the LLM prefills less", n)
	}
}

// A nearly-complete answer that omitted users, goals, navigation, decisions
// and required states still lands: Foundry fills those gaps instead of a
// full LLM retry.
func TestApplyDesignFillsMissingExperience(t *testing.T) {
	p := &core.Project{
		Platform:  &core.PlatformPlan{Targets: []core.PlatformTarget{{Kind: core.PlatformWeb}}},
		Discovery: &core.Discovery{Roles: []core.Role{{Name: "Operator", Platform: "web", Description: "saring data"}}},
		Features:  []core.Feature{{ID: "FEAT-001", Name: "Screener", Platforms: []string{"web"}}},
		Flows:     []core.Flow{{ID: "FLOW-001", Name: "Saring", FeatureID: "FEAT-001", Role: "Operator", Goal: "Saring data pasar"}},
	}
	raw := `{"rules": {"source": "new", "principles": ["Satu tugas"], "ux_guidelines": ["Filter di atas"],
	  "accessibility": ["AA"], "responsive": ["Tabel scroll"],
	  "tokens": {"colors": [{"name": "action.primary", "value": "#000"}], "typography": [{"name": "type.body", "value": "Inter 14px"}],
	              "spacing": [{"name": "space.md", "value": "16px"}]},
	  "components": [{"name": "DataTable", "use": "daftar"}]},
	 "pages": [{"id": "screener", "name": "Screener", "platform": "web", "route": "/screener", "purpose": "Saring", "pattern": "table",
	   "feature_id": "FEAT-001",
	   "sections": [{"id": "tabel", "name": "Tabel", "content": ["Simbol, Harga"]}]}]}`
	if err := ApplyDesign(p, raw); err != nil {
		t.Fatal(err)
	}
	l := p.UI.Layout
	if len(l.Users) == 0 || l.Users[0].Role != "Operator" {
		t.Fatalf("users = %+v", l.Users)
	}
	if len(l.Goals) == 0 || l.Goals[0].ID != "GOAL-1" {
		t.Fatalf("goals = %+v", l.Goals)
	}
	if len(l.Navigation) == 0 || l.Navigation[0].Items[0].Page != "screener" {
		t.Fatalf("nav = %+v", l.Navigation)
	}
	if len(l.Decisions) == 0 || l.Decisions[0].Subject != "screener" {
		t.Fatalf("decisions = %+v", l.Decisions)
	}
	pg := l.Pages[0]
	if pg.States["empty"] == "" || pg.Goal != "GOAL-1" {
		t.Fatalf("page = %+v", pg)
	}
}

func designRulesJSON() string {
	return `{"source": "new", "principles": ["Satu tugas"], "ux_guidelines": ["Filter di atas"],
	  "accessibility": ["AA"], "responsive": ["Tabel scroll"],
	  "tokens": {"colors": [{"name": "action.primary", "value": "#000"}], "typography": [{"name": "type.body", "value": "Inter 14px"}],
	              "spacing": [{"name": "space.md", "value": "16px"}]},
	  "components": [{"name": "DataTable", "use": "daftar"}]}`
}

func webProject() *core.Project {
	return &core.Project{
		Platform:  &core.PlatformPlan{Targets: []core.PlatformTarget{{Kind: core.PlatformWeb}}},
		Discovery: &core.Discovery{Roles: []core.Role{{Name: "Operator", Platform: "web", Description: "saring data"}}},
		Features:  []core.Feature{{ID: "FEAT-001", Name: "Screener", Platforms: []string{"web"}}},
		Flows:     []core.Flow{{ID: "FLOW-001", Name: "Saring", FeatureID: "FEAT-001", Role: "Operator", Goal: "Saring data pasar"}},
		Decisions: []core.Decision{{ID: "DEC-004", Text: "Dark mode", Resolution: "ikuti sistem"}},
	}
}

// Recoverable mistakes (dangling link, unknown presentation, invented
// decision subject/basis) are repaired so the answer lands without an LLM retry.
func TestApplyDesignRepairsRecoverableIssues(t *testing.T) {
	p := webProject()
	raw := `{"rules": ` + designRulesJSON() + `,
	 "pages": [{"id": "screener", "name": "Screener", "platform": "web", "route": "/screener", "purpose": "Saring", "pattern": "table",
	   "feature_id": "FEAT-001",
	   "fields": [{"field": "Asset.price", "label": "Harga", "presentation": "kpi"}],
	   "sections": [{"id": "tabel", "name": "Tabel", "content": ["Simbol, Harga"]}],
	   "links": [{"to": "screener", "via": "diri"}, {"to": "ghost", "via": "X"}, {"to": "screener"}]}],
	 "decisions": [{"subject": "navigation", "choice": "sidebar", "why": ["grup tugas"], "basis": ["PRODUCT CONTEXT", "DEC-004"], "source": "fact"}]}`
	if err := ApplyDesign(p, raw); err != nil {
		t.Fatal(err)
	}
	if DesignNeedsDetails(p) {
		t.Fatal("complete page still flagged for details")
	}
	pg := p.UI.Layout.Pages[0]
	if len(pg.Links) != 0 {
		t.Fatalf("dangling/self links kept: %+v", pg.Links)
	}
	if pg.Fields[0].Presentation != "card-value" {
		t.Fatalf("presentation = %q", pg.Fields[0].Presentation)
	}
	foundDEC := false
	foundNav := false
	for _, d := range p.UI.Layout.Decisions {
		if d.Subject == "navigation" {
			foundNav = true
		}
		if d.Subject == "*" && slices.Contains(d.Basis, "DEC-004") {
			foundDEC = true
		}
	}
	if foundNav {
		t.Fatalf("invalid subject kept: %+v", p.UI.Layout.Decisions)
	}
	if !foundDEC {
		t.Fatalf("DEC-004 basis dropped: %+v", p.UI.Layout.Decisions)
	}
}

// A truncated JSON still lands the complete pages and rules; missing
// sections trigger a detail round instead of refusing the whole answer.
func TestApplyDesignSalvagesTruncationAndAsksDetails(t *testing.T) {
	p := webProject()
	raw := `{"rules": ` + designRulesJSON() + `,
 "pages": [
  {"id": "screener", "name": "Screener", "platform": "web", "route": "/screener", "purpose": "Saring", "pattern": "table", "feature_id": "FEAT-001"},
  {"id": "detail", "name": "Detail", "platform": "web", "route": "/asset", "purpose": "Lihat", "pattern": "detail",
   "sections": [{"id": "isi", "name": "Isi", "content": ["Harga"]}], "states": {"loading": "x", "error": "x"}},
  {"id": "cut", "name": "Cut", "platform": "web", "route": "/cut", "purpose": "X", "pattern": "table", "sections": [{"id": "t"`
	if err := ApplyDesign(p, raw); err != nil {
		t.Fatal(err)
	}
	if !DesignNeedsDetails(p) {
		t.Fatal("inventory page should need details")
	}
	if n := len(p.UI.Layout.Pages); n != 2 {
		t.Fatalf("salvaged pages = %d, want 2 (truncated third dropped)", n)
	}
	if p.UI.Rules == nil || p.UI.Rules.Tokens.Colors[0].Name != "action.primary" {
		t.Fatalf("rules = %+v", p.UI.Rules)
	}
	prompt := DesignPrompt(p)
	if !strings.Contains(prompt, "DETAIL PHASE") || !strings.Contains(prompt, `"id":"screener"`) {
		t.Fatalf("detail prompt missing inventory page:\n%s", prompt)
	}
	if strings.Contains(prompt, "UI DESIGN TEMPLATE") {
		t.Fatal("detail prompt should not resend the full template")
	}
	detail := `{"pages": [{"id": "screener", "name": "Screener", "platform": "web", "route": "/screener", "purpose": "Saring", "pattern": "table",
	  "feature_id": "FEAT-001", "sections": [{"id": "tabel", "name": "Tabel", "content": ["Simbol, Harga"]}],
	  "states": {"loading": "x", "empty": "x", "error": "x"}}]}`
	if err := ApplyDesign(p, detail); err != nil {
		t.Fatal(err)
	}
	if DesignNeedsDetails(p) {
		t.Fatalf("still needs details: %+v", p.UI.Layout.Pages)
	}
	if len(p.UI.Layout.Pages) != 2 {
		t.Fatalf("pages after detail = %d", len(p.UI.Layout.Pages))
	}
}

func TestApplyDesignLandsRulesThenAsksPages(t *testing.T) {
	p := webProject()
	raw := `{"rules": ` + designRulesJSON() + `}`
	if err := ApplyDesign(p, raw); err != nil {
		t.Fatal(err)
	}
	if !DesignNeedsDetails(p) || p.UI.Rules == nil {
		t.Fatalf("rules-only should need page details: %+v", p.UI)
	}
	prompt := DesignPrompt(p)
	if !strings.Contains(prompt, "no pages yet") {
		t.Fatalf("prompt = %s", prompt)
	}
}

// Pages without rules still land: Foundry fills a valid default design system
// instead of refusing the whole answer.
func TestApplyDesignFillsMissingRulesFromPages(t *testing.T) {
	p := webProject()
	raw := `{"pages": [{"id": "screener", "name": "Screener", "platform": "web", "route": "/screener",
	  "purpose": "Saring", "pattern": "table", "feature_id": "FEAT-001",
	  "sections": [{"id": "tabel", "name": "Tabel", "content": ["Simbol, Harga"]}]}]}`
	if err := ApplyDesign(p, raw); err != nil {
		t.Fatal(err)
	}
	if p.UI.Rules == nil || len(p.UI.Rules.Principles) == 0 || len(p.UI.Rules.Tokens.Colors) == 0 {
		t.Fatalf("default rules missing: %+v", p.UI.Rules)
	}
	if len(p.UI.Layout.Pages) != 1 || p.UI.Layout.Pages[0].ID != "screener" {
		t.Fatalf("pages = %+v", p.UI.Layout.Pages)
	}
}

// A cut-off ```json fence with rules and one complete page still lands.
func TestApplyDesignAcceptsTruncatedFence(t *testing.T) {
	p := webProject()
	raw := "here you go\n```json\n" + `{"rules": ` + designRulesJSON() + `,
 "pages": [
  {"id": "screener", "name": "Screener", "platform": "web", "route": "/screener", "purpose": "Saring", "pattern": "table", "feature_id": "FEAT-001"},
  {"id": "cut", "name": "Cut", "platform": "web", "route": "/x", "purpose": "X", "pattern": "table", "sections": [{"id": "t"`
	if err := ApplyDesign(p, raw); err != nil {
		t.Fatal(err)
	}
	if p.UI.Rules == nil || len(p.UI.Layout.Pages) < 1 {
		t.Fatalf("ui = %+v", p.UI)
	}
}

func TestApplyDesignStillRefusesEmptyProse(t *testing.T) {
	p := webProject()
	if err := ApplyDesign(p, "maaf, saya tidak bisa menyusun JSON saat ini"); err == nil || p.UI != nil {
		t.Fatalf("err = %v ui = %+v", err, p.UI)
	}
}

func TestDesignPromptPutsRulesBeforeUX(t *testing.T) {
	prompt := DesignPrompt(clinicPromptProject())
	rules := strings.Index(prompt, `"rules"`)
	users := strings.Index(prompt, `"users"`)
	if rules < 0 || users < 0 || rules > users {
		t.Fatalf("shape should emit rules before users (rules=%d users=%d)", rules, users)
	}
}
