package work

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/handrawanw/foundry/internal/core"
)

const voiceflowDNA = `{"reference": "Voiceflow", "category": "editor", "personality": ["Modern", "creative", "technical", "saas"],
 "layout": ["canvas", "workspace", "split-panel"], "navigation": "side", "density": "comfortable",
 "visual": {"color_mode": "light", "spacing": "comfortable", "surface": "soft", "radius": "medium"},
 "typography": {"scale": "balanced", "personality": "geometric", "weights": "semibold headings"},
 "colors": {"primary": "#3d5afe", "background": "#f7f7f8", "surface": "#ffffff", "text": "#1a1a1a", "text_muted": "#5f6368", "border": "#e4e4e7",
            "success": "#16a34a", "warning": "#b45309", "danger": "#dc2626"},
 "components": {"button": "soft", "nav": "light"},
 "interaction": ["side panel opens on select", "inline editing"],
 "rules": [{"if": "the user builds a flow", "then": "canvas first, properties in a side panel", "why": "the work stays visible"}],
 "avoid": ["dense enterprise table", "hero_section"]}`

const linearDNA = `{"reference": "Linear", "category": "workspace", "personality": ["fast", "precise", "dark"],
 "layout": ["list", "master-detail"], "navigation": "side", "density": "compact",
 "visual": {"color_mode": "dark", "spacing": "compact", "surface": "bordered", "radius": "small"},
 "typography": {"scale": "compact", "personality": "technical"},
 "colors": {"primary": "#5e6ad2", "background": "#0f1012", "surface": "#16171a", "surface_alt": "#1c1d21", "text": "#e6e6e6", "text_muted": "#9a9ba0",
            "border": "#26272b", "success": "#4cb782", "warning": "#f2c94c", "danger": "#eb5757", "info": "#4ea7fc"},
 "components": {"button": "solid", "nav": "dark"}}`

func refTheme(t *testing.T, url, dna string) core.Theme {
	t.Helper()
	r, err := ParseReference(url, "")
	if err != nil {
		t.Fatal(err)
	}
	var out referenceOut
	if err := ExtractJSON(dna, &out); err != nil {
		t.Fatal(err)
	}
	th, err := ThemeFromDNA(*r, out.ThemeDNA)
	if err != nil {
		t.Fatal(err)
	}
	return th
}

func TestParseReference(t *testing.T) {
	for in, want := range map[string][2]string{
		"voiceflow.com":                 {"https://voiceflow.com", "Voiceflow"},
		"https://www.linear.app/":       {"https://www.linear.app/", "Linear"},
		"https://app.example.co.id/x#a": {"https://app.example.co.id/x", "Example"},
		"http://localhost:3000":         {"http://localhost:3000", "localhost"},
	} {
		r, err := ParseReference(in, " suka sidebarnya ")
		if err != nil || r.URL != want[0] || r.Name != want[1] || r.Status != "pending" || r.Notes != "suka sidebarnya" {
			t.Errorf("%s: %+v %v", in, r, err)
		}
	}
	for _, in := range []string{"", "ftp://example.com", "javascript:alert(1)", "https://", "https://user:pw@example.com", "bukan url"} {
		if _, err := ParseReference(in, ""); err == nil {
			t.Errorf("%q accepted", in)
		}
	}
}

func TestThemeFromDNA(t *testing.T) {
	th := refTheme(t, "voiceflow.com", voiceflowDNA)
	if th.ID != "voiceflow-inspired" || th.Name != "Voiceflow Inspired" || th.Reference.Category != "editor" || th.Reference.URL != "https://voiceflow.com" {
		t.Fatalf("identity = %+v", th)
	}
	if !slices.Equal(th.Preferences.Layouts, []string{"workspace", "master-detail"}) || th.Preferences.Navigation != "side" {
		t.Fatalf("preferences = %+v", th.Preferences)
	}
	tok := func(name string) string {
		i := slices.IndexFunc(th.Colors, func(c core.Token) bool { return c.Name == name })
		return th.Colors[i].Value
	}
	// Roles become semantic tokens; missing roles are derived.
	if tok("action.primary") != "#3d5afe" || tok("surface.secondary") != "#ffffff" || tok("status.info") != "#3d5afe" || tok("status.error") != "#dc2626" {
		t.Fatalf("colors = %+v", th.Colors)
	}
	if th.Components["card"] != "flat" || th.Components["button"] != "soft" || th.Components["nav"] != "light" || th.Components["font"] != "sans" {
		t.Fatalf("components = %+v", th.Components)
	}
	if !slices.Equal(th.Avoid, []string{"hero_section"}) || !slices.ContainsFunc(th.Rules, func(r core.ThemeRule) bool { return r.Then == "avoid dense enterprise table" }) {
		t.Fatalf("avoid = %v rules = %+v", th.Avoid, th.Rules)
	}
	if len(th.Interaction) != 2 || core.ValidateTheme(th) != nil {
		t.Fatalf("theme = %+v", th)
	}

	r, _ := ParseReference("voiceflow.com", "")
	var inv *core.ThemeInvalid
	_, err := ThemeFromDNA(*r, core.ThemeDNA{Category: "spaceship", Personality: []string{"x"}, Layout: []string{"canvas-ish"},
		Visual: core.DNAVisual{ColorMode: "light", Radius: "huge"}, Colors: map[string]string{"primary": "#123456", "background": "url(x)"}})
	if !errors.As(err, &inv) {
		t.Fatalf("err = %v", err)
	}
	for _, want := range []string{`category "spaceship"`, `visual.radius "huge"`, "layout tidak berisi", "warna text tidak ada"} {
		if !slices.ContainsFunc(inv.Problems, func(p string) bool { return strings.Contains(p, want) }) {
			t.Errorf("missing problem %q in %v", want, inv.Problems)
		}
	}
	// Well-formed DNA with a bad color value fails the theme validation.
	bad := strings.Replace(voiceflowDNA, `"#f7f7f8"`, `"url(x)"`, 1)
	var out referenceOut
	ExtractJSON(bad, &out)
	if _, err := ThemeFromDNA(*r, out.ThemeDNA); err == nil || !strings.Contains(err.Error(), "surface.page") {
		t.Fatalf("bad color err = %v", err)
	}
}

func TestApplyReference(t *testing.T) {
	dir := t.TempDir()
	r, _ := ParseReference("voiceflow.com", "")
	p := &core.Project{Dir: dir, Reference: r}
	th, err := ApplyReference(p, "```json\n"+voiceflowDNA+"\n```")
	if err != nil || th == nil || p.Theme != "voiceflow-inspired" || r.Status != "done" || r.ThemeID != th.ID || r.DNA == nil {
		t.Fatalf("err = %v ref = %+v", err, r)
	}
	themes, bad := Themes(dir)
	loaded := findTheme(themes, "voiceflow-inspired")
	if len(bad) > 0 || loaded == nil || loaded.Reference == nil || mustJSON(*loaded) != mustJSON(*th) {
		t.Fatalf("saved theme: bad = %+v loaded = %+v", bad, loaded)
	}

	// The analyst could not see a product: the reference fails, no theme.
	p2 := &core.Project{Dir: t.TempDir(), Reference: &core.DesignReference{URL: "https://x.test", Name: "X", Status: "analyzing"}}
	if th, err := ApplyReference(p2, `{"error": "halaman hanya berisi login"}`); err != nil || th != nil || p2.Reference.Status != "failed" || p2.Theme != "" {
		t.Fatalf("error reply: %v %+v", err, p2.Reference)
	}
	if _, err := ApplyReference(p2, "maaf"); err == nil {
		t.Fatal("no json accepted")
	}
	if _, err := ApplyReference(&core.Project{}, voiceflowDNA); err == nil {
		t.Fatal("no reference accepted")
	}
}

// Same requirement, another reference: another theme, another direction
// for the designer; the reference is never a template.
func TestDifferentReferencesDifferentDesign(t *testing.T) {
	dir := t.TempDir()
	prompts := map[string]string{}
	for url, dna := range map[string]string{"voiceflow.com": voiceflowDNA, "linear.app": linearDNA} {
		r, _ := ParseReference(url, "")
		p := &core.Project{Dir: dir, Story: "Platform manajemen AI agent untuk tim support", Platform: &core.PlatformPlan{}, Reference: r}
		if _, err := ApplyReference(p, dna); err != nil {
			t.Fatal(err)
		}
		prompts[r.Name] = DesignPrompt(p)
	}
	v, l := prompts["Voiceflow"], prompts["Linear"]
	if v == l || !strings.Contains(v, "Inspired by Voiceflow") || !strings.Contains(l, "Theme: Linear Inspired") || !strings.Contains(l, "color mode dark") {
		t.Fatal("prompts do not follow the reference")
	}
	if !strings.Contains(v, "Never copy it") || !strings.Contains(v, "brand name (Voiceflow)") || !strings.Contains(v, "canvas first") {
		t.Fatal("anti-copy rule or extracted rules missing")
	}
	vt, lt := refTheme(t, "voiceflow.com", voiceflowDNA), refTheme(t, "linear.app", linearDNA)
	if vt.Tokens == lt.Tokens || slices.Equal(vt.Colors, lt.Colors) || vt.Components["font"] == lt.Components["font"] {
		t.Fatal("themes do not differ")
	}
}

func TestReferenceConsistency(t *testing.T) {
	th := refTheme(t, "voiceflow.com", voiceflowDNA)
	pg := ticketWorkspace()
	if got := ReferenceLint(pg, &th); len(got) != 0 {
		t.Fatalf("master-detail page in a workspace reference: %v", categories(got))
	}
	pg.Plan.Layout = "table"
	pg.Components[1].Label = "Tiket ala Voiceflow"
	got := ReferenceLint(pg, &th)
	rules := map[string]string{}
	for _, is := range got {
		rules[is.Rule] = is.Severity + " " + is.Category + " " + is.Component
	}
	if rules["reference_copy"] != "major reference title" || rules["reference_direction"] != "minor reference " {
		t.Fatalf("findings = %v", rules)
	}
	d := scoreDims(got)
	if d["reference"] >= 100 || IsBlocker(got[0]) || IsBlocker(got[1]) {
		t.Fatalf("dims = %v", d)
	}
	// A hand-made theme has no reference direction.
	builtin := builtin(t, "modern-saas")
	if len(ReferenceLint(pg, &builtin)) != 0 || len(ReferenceLint(pg, nil)) != 0 {
		t.Fatal("reference findings without a reference")
	}
}
