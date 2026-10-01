package work

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/handrawanw/foundry/internal/core"
)

func builtin(t *testing.T, id string) core.Theme {
	t.Helper()
	themes, _ := Themes("")
	th := findTheme(themes, id)
	if th == nil {
		t.Fatalf("theme %s missing", id)
	}
	return *th
}

// writeTheme stores a project theme from a built-in one changed by edit.
func writeTheme(t *testing.T, dir, id string, edit func(m map[string]any)) {
	t.Helper()
	th := builtin(t, "modern-saas")
	th.ID, th.Name = id, "Custom "+id
	b, _ := json.Marshal(th)
	var m map[string]any
	json.Unmarshal(b, &m)
	if edit != nil {
		edit(m)
	}
	out := filepath.Join(dir, ProjectThemesDir, id)
	os.MkdirAll(out, 0o755)
	b, _ = json.Marshal(m)
	// One file holds everything; tokens.json must exist too.
	os.WriteFile(filepath.Join(out, "theme.json"), b, 0o644)
	os.WriteFile(filepath.Join(out, "tokens.json"), []byte("{}"), 0o644)
}

func TestBuiltinThemes(t *testing.T) {
	themes, bad := Themes("")
	if len(bad) > 0 || len(themes) != 14 {
		t.Fatalf("themes = %d, invalid = %+v", len(themes), bad)
	}
	for _, th := range themes {
		if err := core.ValidateTheme(th); err != nil {
			t.Error(err)
		}
	}
	if findTheme(themes, FallbackTheme) == nil {
		t.Fatal("fallback theme missing")
	}
	if !slices.Equal(core.ThemeDomains, Domains) || !slices.Equal(core.ThemeLayouts, LayoutStrategies) {
		t.Fatal("theme vocabulary differs from design intelligence")
	}
}

func TestThemeLoading(t *testing.T) {
	dir := t.TempDir()
	writeTheme(t, dir, "brand", nil)
	writeTheme(t, dir, "bad-mode", func(m map[string]any) { m["tokens"].(map[string]any)["color_mode"] = "neon" })
	writeTheme(t, dir, "no-primary", func(m map[string]any) {
		var keep []any
		for _, c := range m["colors"].([]any) {
			if c.(map[string]any)["name"] != "action.primary" {
				keep = append(keep, c)
			}
		}
		m["colors"] = keep
	})
	writeTheme(t, dir, "unknown-token", func(m map[string]any) {
		m["radius"] = append(m["radius"].([]any), map[string]any{"name": "radius.large", "value": "20px"})
	})
	writeTheme(t, dir, "wrong-folder", func(m map[string]any) { m["id"] = "other" })
	writeTheme(t, dir, "css-inject", func(m map[string]any) {
		m["colors"].([]any)[0].(map[string]any)["value"] = "red;background:url(x)"
	})
	themes, bad := Themes(dir)
	if findTheme(themes, "brand") == nil || len(themes) != 15 {
		t.Fatalf("valid project theme not loaded: %d themes", len(themes))
	}
	want := map[string]string{"bad-mode": `color_mode "neon"`, "no-primary": "token action.primary tidak ada",
		"unknown-token": "token radius.large tidak dikenal", "css-inject": "token text.primary: nilai", "wrong-folder": "nama folder"}
	if len(bad) != len(want) {
		t.Fatalf("invalid = %+v", bad)
	}
	for _, b := range bad {
		if !strings.HasPrefix(b.Error, "THEME_INVALID "+b.ID) || !strings.Contains(b.Error, want[b.ID]) {
			t.Errorf("%s: %s", b.ID, b.Error)
		}
	}
	var inv *core.ThemeInvalid
	if err := core.ValidateTheme(core.Theme{ID: "x"}); !errors.As(err, &inv) || len(inv.Problems) < 5 {
		t.Fatalf("empty theme = %v", err)
	}
}

func TestThemeSelection(t *testing.T) {
	themes, _ := Themes("")
	for _, tc := range []struct{ story, theme, personality string }{
		{"Buat aplikasi market maker management untuk trading kripto: order book, watchlist", "trading-terminal", "high density"},
		{"Aplikasi keuangan: saldo rekening, transaksi dan anggaran bulanan", "finance-minimal", "trustworthy"},
		{"Developer tool: deploy, logs dan pipeline CI/CD per repository", "enterprise-dark", "technical"},
		{"Aplikasi resep masakan keluarga", "modern-saas", "clean"},
	} {
		p := &core.Project{Story: tc.story}
		th, ctx := SelectTheme(themes, "", DetectDomain(p))
		if th == nil || th.ID != tc.theme || ctx.Source != "auto" || len(ctx.Reasons) == 0 || !slices.Contains(th.Personality, tc.personality) {
			t.Errorf("%q: theme = %+v", tc.story, ctx)
		}
	}
	if _, ctx := SelectTheme(themes, "finance-minimal", "trading"); ctx.ID != "finance-minimal" || ctx.Source != "user" {
		t.Fatalf("user pick = %+v", ctx)
	}
	if _, ctx := SelectTheme(themes, "gone", "trading"); ctx.ID != "trading-terminal" || ctx.Source != "fallback" || !strings.Contains(ctx.Reasons[0], "tidak tersedia") {
		t.Fatalf("missing pick = %+v", ctx)
	}
	if th, _ := SelectTheme(nil, "", ""); th != nil {
		t.Fatal("no themes: no theme")
	}
}

// A theme changes the tokens and the style, never the Design IR.
func TestApplyThemeKeepsIR(t *testing.T) {
	d := &core.Design{Pages: []core.Page{ticketWorkspace()},
		System: &core.DesignSystem{Colors: []core.Token{{Name: "action.primary", Value: "#ff00ff"}},
			Components: []core.Token{{Name: "Button", Value: "primary | ghost"}}, Breakpoints: []core.Token{{Name: "mobile", Value: "<640px"}}}}
	ir := mustJSON(d.Pages)
	themes, _ := Themes("")
	for _, id := range []string{"trading-terminal", "finance-minimal"} {
		th, ctx := SelectTheme(themes, id, "")
		ApplyTheme(d, th, ctx)
		if mustJSON(d.Pages) != ir {
			t.Fatal("theme changed the Design IR")
		}
		if d.Theme.ID != id || d.Theme.ColorMode != th.Tokens.ColorMode || d.Theme.Style["button"] != th.Components["button"] {
			t.Fatalf("context = %+v", d.Theme)
		}
		if !slices.Equal(d.System.Colors, th.Colors) || !slices.ContainsFunc(d.System.Components, func(c core.Token) bool { return c.Name == "Button" }) ||
			len(d.System.Breakpoints) != 1 || slices.IndexFunc(d.System.Components, func(c core.Token) bool { return c.Name == "style.button" }) < 0 {
			t.Fatalf("system = %+v", d.System)
		}
	}
	// Re-applying does not duplicate the style tokens.
	n := len(d.System.Components)
	th, ctx := SelectTheme(themes, "finance-minimal", "")
	ApplyTheme(d, th, ctx)
	if len(d.System.Components) != n {
		t.Fatalf("components grew: %+v", d.System.Components)
	}
}

func TestThemePrompt(t *testing.T) {
	p := &core.Project{Story: "Platform trading saham dengan watchlist dan order book", Platform: &core.PlatformPlan{}}
	s := DesignPrompt(p)
	for _, want := range []string{"THEME CONTEXT", "Trading Terminal", "Avoid (conflicts with this theme): hero_section", "IF the workflow monitors many states", "come from the theme"} {
		if !strings.Contains(s, want) {
			t.Errorf("prompt misses %q", want)
		}
	}
}

func themedProject(id string, pages ...core.Page) *core.Project {
	p := qualityProject(pages...)
	themes, _ := Themes("")
	th, ctx := SelectTheme(themes, id, "")
	ApplyTheme(p.Design, th, ctx)
	return p
}

func TestThemeConsistency(t *testing.T) {
	q := MeasureDesign(themedProject("enterprise-dark", ticketWorkspace()))
	if q.Dimensions["theme"] != 100 || slices.ContainsFunc(q.Findings, func(is core.QAIssue) bool { return is.Category == "theme" }) {
		t.Fatalf("good page = %+v", q.Findings)
	}
	hero := func(pg *core.Page) {
		pg.Components = append(pg.Components, core.PageComponent{ID: "hero-title", Type: "header", Label: "Kelola tiket lebih cepat"},
			core.PageComponent{ID: "hero-text", Type: "text", Label: "Semua tiket di satu tempat"})
		pg.Layout.Root.Children = append([]core.LayoutNode{{ID: "hero", Type: "center", Children: []core.LayoutNode{{Component: "hero-title"}, {Component: "hero-text"}, {Component: "send"}}}},
			pg.Layout.Root.Children...)
		pg.Layout.Root.Children[2].Children[1].Children[0].Children = pg.Layout.Root.Children[2].Children[1].Children[0].Children[:1]
	}
	for _, tc := range []struct {
		name, theme string
		edit        func(pg *core.Page)
		rule, sev   string
	}{
		{"marketing hero", "enterprise-dark", hero, "theme_hero", "major"},
		{"large cards", "enterprise-dark", func(pg *core.Page) {
			for _, id := range []string{"c1", "c2"} {
				pg.Components = append(pg.Components, core.PageComponent{ID: id, Type: "card", Label: id})
				pg.Layout.Root.Children = append(pg.Layout.Root.Children, core.LayoutNode{Component: id})
			}
		}, "theme_large_cards", "major"},
		{"excessive whitespace", "trading-terminal", func(pg *core.Page) { pg.Density = "spacious" }, "theme_whitespace", "minor"},
		{"navigation", "trading-terminal", func(pg *core.Page) {}, "theme_nav", "minor"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pg := ticketWorkspace()
			tc.edit(&pg)
			if probs := core.ValidatePage(pg, map[string]bool{"PAGE-001": true}); len(probs) > 0 {
				t.Fatalf("fixture IR invalid: %v", probs)
			}
			q := MeasureDesign(themedProject(tc.theme, pg))
			i := slices.IndexFunc(q.Findings, func(is core.QAIssue) bool { return is.Rule == tc.rule })
			if i < 0 || q.Findings[i].Severity != tc.sev || q.Findings[i].Category != "theme" || !strings.Contains(q.Findings[i].Why, "theme") {
				t.Fatalf("findings = %v", categories(q.Findings))
			}
			if q.Dimensions["theme"] >= 100 || IsBlocker(q.Findings[i]) {
				t.Fatalf("theme dimension = %d, blocker = %v", q.Dimensions["theme"], IsBlocker(q.Findings[i]))
			}
		})
	}
	// Without a theme the same page has no theme findings.
	pg := ticketWorkspace()
	hero(&pg)
	if q := MeasureDesign(qualityProject(pg)); q.Dimensions["theme"] != 100 {
		t.Fatalf("unthemed = %+v", q.Dimensions)
	}
}

func TestThemeTokenDrift(t *testing.T) {
	p := themedProject("modern-saas", ticketWorkspace())
	p.Design.System.Colors[6].Value = "#ff00ff"
	q := MeasureDesign(p)
	if !slices.ContainsFunc(q.Findings, func(is core.QAIssue) bool {
		return is.Rule == "theme_token_drift" && strings.Contains(is.Problem, "action.primary")
	}) {
		t.Fatalf("findings = %+v", q.Findings)
	}
}

// Designs made before themes keep working: no theme, their own tokens.
func TestUnthemedDesign(t *testing.T) {
	var d core.Design
	if err := json.Unmarshal([]byte(`{"pages": [], "status": "draft", "feedback": [], "system": {"colors": [{"name": "action.primary", "value": "#123456"}]}}`), &d); err != nil {
		t.Fatal(err)
	}
	d.Pages = []core.Page{ticketWorkspace()}
	p := &core.Project{Design: &d}
	if d.Theme != nil || currentTheme(p) != nil || len(LintDesign(p, nil)) != 0 {
		t.Fatal("old design got a theme")
	}
	if q := MeasureDesign(p); q.Overall != 100 || len(q.Dimensions) != len(QualityDimensions) {
		t.Fatalf("quality = %+v", q)
	}
	if d.System.Colors[0].Value != "#123456" {
		t.Fatal("tokens changed")
	}
}
