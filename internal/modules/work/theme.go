package work

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/handrawanw/foundry/internal/core"
)

// Theme layer: a theme is design context (style, preferences,
// restrictions), never a page template. Foundry selects one from the
// requirement or takes the user's pick, gives it to the designer as
// knowledge, resolves the design system tokens from it and measures the
// pages against it. The Design IR stays the record.

//go:embed themes
var builtinThemes embed.FS

// FallbackTheme is used when no theme fits or the picked one is missing.
const FallbackTheme = "modern-saas"

// ProjectThemesDir holds a project's own themes, same layout as the
// built-in ones: <id>/theme.json, tokens.json, components.json.
var ProjectThemesDir = filepath.Join(core.StateDir, "themes")

// ThemeProblem is a theme that failed to load or validate.
type ThemeProblem struct {
	ID    string `json:"id"`
	Error string `json:"error"`
}

// LoadTheme reads one theme directory of fsys and validates it.
func LoadTheme(fsys fs.FS, dir string) (core.Theme, error) {
	var t core.Theme
	for _, f := range []struct {
		name     string
		v        any
		required bool
	}{{"theme.json", &t, true}, {"tokens.json", &t, true}, {"components.json", &t.Components, false}} {
		b, err := fs.ReadFile(fsys, dir+"/"+f.name)
		if errors.Is(err, fs.ErrNotExist) && !f.required {
			continue
		}
		if err != nil {
			return t, &core.ThemeInvalid{ID: dir, Problems: []string{f.name + " tidak bisa dibaca"}}
		}
		if err := json.Unmarshal(b, f.v); err != nil {
			return t, &core.ThemeInvalid{ID: dir, Problems: []string{f.name + ": " + err.Error()}}
		}
	}
	if t.ID != dir {
		return t, &core.ThemeInvalid{ID: dir, Problems: []string{fmt.Sprintf("id %q harus sama dengan nama folder %q", t.ID, dir)}}
	}
	return t, core.ValidateTheme(t)
}

func loadDir(fsys fs.FS, root string) (map[string]core.Theme, []ThemeProblem) {
	out := map[string]core.Theme{}
	var bad []ThemeProblem
	sub, err := fs.Sub(fsys, root)
	if err != nil {
		return out, nil
	}
	entries, _ := fs.ReadDir(sub, ".")
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		t, err := LoadTheme(sub, e.Name())
		if err != nil {
			bad = append(bad, ThemeProblem{ID: e.Name(), Error: err.Error()})
			continue
		}
		out[t.ID] = t
	}
	return out, bad
}

// Themes lists the themes available to a project: the built-in ones plus
// valid themes in .foundry/themes (same id replaces the built-in one),
// sorted by name, and the project themes that are invalid.
func Themes(projectDir string) ([]core.Theme, []ThemeProblem) {
	all, bad := loadDir(builtinThemes, "themes")
	if projectDir != "" {
		own, ownBad := loadDir(os.DirFS(projectDir), filepath.ToSlash(ProjectThemesDir))
		for id, t := range own {
			all[id] = t
		}
		bad = append(bad, ownBad...)
	}
	out := make([]core.Theme, 0, len(all))
	for _, t := range all {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, bad
}

func findTheme(themes []core.Theme, id string) *core.Theme {
	for i := range themes {
		if themes[i].ID == id {
			return &themes[i]
		}
	}
	return nil
}

// SelectTheme picks the theme of a design: the user's pick when it is
// available, else the theme made for the domain, else the fallback.
func SelectTheme(themes []core.Theme, picked, domain string) (*core.Theme, core.ThemeContext) {
	var why []string
	if picked != "" {
		if t := findTheme(themes, picked); t != nil {
			return t, themeContext(*t, "user", []string{"dipilih oleh user"})
		}
		why = append(why, fmt.Sprintf("theme %q tidak tersedia", picked))
	}
	if domain != "" {
		for i := range themes {
			t := themes[i]
			if slices.Contains(t.UseWhen.Domains, domain) {
				why = append(why, fmt.Sprintf("domain %s: %s", domain, strings.Join(t.Personality, ", ")))
				return &themes[i], themeContext(t, source(picked), why)
			}
		}
	}
	t := findTheme(themes, FallbackTheme)
	if t == nil {
		return nil, core.ThemeContext{}
	}
	if domain == "" {
		why = append(why, "domain tidak terdeteksi: theme default")
	} else {
		why = append(why, fmt.Sprintf("belum ada theme khusus domain %s: theme default", domain))
	}
	return t, themeContext(*t, source(picked), why)
}

func source(picked string) string {
	if picked != "" {
		return "fallback"
	}
	return "auto"
}

func themeContext(t core.Theme, src string, why []string) core.ThemeContext {
	style := map[string]string{}
	for k, v := range t.Components {
		style[k] = v
	}
	return core.ThemeContext{ID: t.ID, Name: t.Name, Source: src, Reasons: why, ColorMode: t.Tokens.ColorMode, Style: style}
}

// ResolveTheme selects the theme for the project's design.
func ResolveTheme(p *core.Project) (*core.Theme, core.ThemeContext) {
	themes, _ := Themes(p.Dir)
	return SelectTheme(themes, p.Theme, DetectDomain(p))
}

// currentTheme loads the theme the design follows, nil without one.
func currentTheme(p *core.Project) *core.Theme {
	if p.Design == nil || p.Design.Theme == nil {
		return nil
	}
	themes, _ := Themes(p.Dir)
	return findTheme(themes, p.Design.Theme.ID)
}

// ThemeSystem is the design system of a themed design: every token from
// the theme, the designer's direction notes, breakpoints and component
// rules kept, the component style added.
func ThemeSystem(t core.Theme, ai *core.DesignSystem) *core.DesignSystem {
	s := &core.DesignSystem{Direction: t.Name + ": " + strings.Join(t.Personality, ", "),
		Colors: slices.Clone(t.Colors), Typography: slices.Clone(t.Typography), Spacing: slices.Clone(t.Spacing),
		Radius: slices.Clone(t.Radius), Shadow: slices.Clone(t.Shadow)}
	keys := make([]string, 0, len(t.Components))
	for k := range t.Components {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		s.Components = append(s.Components, core.Token{Name: "style." + k, Value: t.Components[k], Use: "theme " + t.ID})
	}
	if ai != nil {
		s.Breakpoints = normTokens(ai.Breakpoints)
		for _, c := range normTokens(ai.Components) {
			if !strings.HasPrefix(c.Name, "style.") {
				s.Components = append(s.Components, c)
			}
		}
	}
	return s
}

// ApplyTheme makes the design follow a theme: its tokens and style. The
// pages (Design IR) are not touched.
func ApplyTheme(d *core.Design, t *core.Theme, ctx core.ThemeContext) {
	if d == nil || t == nil {
		return
	}
	d.System = ThemeSystem(*t, d.System)
	d.Theme = &ctx
}

// themeKnowledge is the theme section of the design prompt.
func themeKnowledge(t *core.Theme) string {
	if t == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "THEME CONTEXT (visual direction, not a template; compose every page for its job):\nTheme: %s. Personality: %s.\n", t.Name, strings.Join(t.Personality, ", "))
	fmt.Fprintf(&b, "Tokens: color mode %s, radius %s, spacing %s. The token values come from the theme: use the semantic token names only.\n",
		t.Tokens.ColorMode, t.Tokens.Radius, t.Tokens.Spacing)
	if pr := t.Preferences; pr.Navigation != "" || len(pr.Layouts) > 0 {
		fmt.Fprintf(&b, "Prefer (when the page's job allows): navigation %s, layout strategies %s, density %s.\n", pr.Navigation, strings.Join(pr.Layouts, ", "), pr.Density)
	}
	if len(t.Avoid) > 0 {
		b.WriteString("Avoid (conflicts with this theme): " + strings.Join(t.Avoid, ", ") + ".\n")
	}
	for _, r := range t.Rules {
		fmt.Fprintf(&b, "- IF %s THEN %s. WHY: %s.\n", r.If, r.Then, r.Why)
	}
	if len(t.Interaction) > 0 {
		b.WriteString("Interaction style: " + strings.Join(t.Interaction, "; ") + ".\n")
	}
	if r := t.Reference; r != nil {
		fmt.Fprintf(&b, "Inspired by %s (%s, a %s product): apply its principles to THIS product's own features and flows. "+
			"Never copy it: no pages cloned from it, no text, slogans or brand name (%s) in labels, no logo or illustrations.\n", r.Name, r.URL, r.Category, r.Name)
	}
	return b.String()
}

// ---------- Theme consistency ----------

// ThemeLint lists where one page conflicts with the theme: an anti-pattern
// the theme avoids or a navigation it does not use. Category "theme".
func ThemeLint(pg core.Page, t *core.Theme) []core.QAIssue {
	l := pg.Layout
	if t == nil || l == nil {
		return nil
	}
	var out []core.QAIssue
	add := func(rule, sev, comp, problem, why, fix string) {
		out = append(out, core.QAIssue{PageID: pg.ID, Severity: sev, Category: "theme", Component: comp,
			Problem: problem, Why: why, Fix: fix, Source: "lint", Rule: rule})
	}
	conflict := "bertentangan dengan theme " + t.Name + " (" + strings.Join(t.Personality, ", ") + ")"
	kind := core.PageKind(pg)
	avoid := func(a string) bool { return slices.Contains(t.Avoid, a) }

	if avoid("hero_section") && kind != "auth" && kind != "status" {
		if n, ok := heroNode(l.Root, pg); ok {
			add("theme_hero", "major", n, "Hero section besar di awal halaman", conflict,
				"Ganti hero dengan judul halaman dan isi kerja utama langsung.")
		}
	}
	if avoid("large_cards") {
		cards := 0
		for _, c := range pg.Components {
			if c.Type == "card" {
				cards++
			}
		}
		if cards >= 3 {
			add("theme_large_cards", "major", "", fmt.Sprintf("%d kartu besar terpisah", cards), conflict,
				"Tampilkan data sebagai tabel atau daftar yang padat.")
		}
	}
	if avoid("excessive_whitespace") && pg.Density == "spacious" && kind != "auth" && kind != "status" && kind != "payment" {
		add("theme_whitespace", "minor", "", "Density spacious pada halaman kerja", conflict,
			"Pakai density "+orDefault(t.Preferences.Density, "compact")+".")
	}
	if want := t.Preferences.Navigation; want != "" && l.Frame == "app" && l.Nav != "" && l.Nav != "none" && l.Nav != want &&
		(pg.Platform == core.PlatformWeb || pg.Platform == core.PlatformDesktop) {
		add("theme_nav", "minor", "", fmt.Sprintf("Navigasi %s, theme memakai %s", l.Nav, want), conflict,
			"Ubah layout.nav ke "+want+".")
	}
	return out
}

// ReferenceLint measures a page against the reference its theme comes
// from: the reference's brand or text copied into the page, and a layout
// strategy outside the reference direction. Category "reference".
func ReferenceLint(pg core.Page, t *core.Theme) []core.QAIssue {
	if t == nil || t.Reference == nil {
		return nil
	}
	ref := t.Reference
	var out []core.QAIssue
	add := func(rule, sev, comp, problem, why, fix string) {
		out = append(out, core.QAIssue{PageID: pg.ID, Severity: sev, Category: "reference", Component: comp,
			Problem: problem, Why: why, Fix: fix, Source: "lint", Rule: rule})
	}
	if len(ref.Name) >= 3 {
		brand := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(ref.Name) + `\b`)
		hit := ""
		if brand.MatchString(pg.Name) {
			hit = "page"
		}
		for _, c := range pg.Components {
			if hit == "" && (brand.MatchString(c.Label) || slices.ContainsFunc(c.Items, brand.MatchString)) {
				hit = c.ID
			}
		}
		if hit != "" {
			comp := hit
			if hit == "page" {
				comp = ""
			}
			add("reference_copy", "major", comp, "Nama atau teks "+ref.Name+" muncul di halaman",
				"referensi hanya inspirasi; teks dan merek tidak boleh disalin", "Pakai istilah produk ini sendiri.")
		}
	}
	work := map[string]bool{"list": true, "table": true, "master-detail": true, "workspace": true, "dashboard": true}
	if pl := pg.Plan; pl != nil && work[pl.Layout] && len(t.Preferences.Layouts) > 0 && !slices.Contains(t.Preferences.Layouts, pl.Layout) {
		add("reference_direction", "minor", "", fmt.Sprintf("Layout %s di luar arah referensi %s (%s)", pl.Layout, ref.Name, strings.Join(t.Preferences.Layouts, ", ")),
			fmt.Sprintf("referensi %s adalah produk %s", ref.Name, ref.Category),
			"Pertimbangkan "+t.Preferences.Layouts[0]+" bila tugas halaman mengizinkan.")
	}
	return out
}

func orDefault(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

// heroNode: the first part of the page is a node that holds a header, a
// line of text and the primary button and nothing else: a marketing hero.
func heroNode(root core.LayoutNode, pg core.Page) (string, bool) {
	if len(root.Children) == 0 {
		return "", false
	}
	first := root.Children[0]
	if first.Component != "" || len(first.Children) == 0 {
		return "", false
	}
	types := map[string]int{}
	primary := false
	ok := true
	first.Walk(func(n core.LayoutNode, _ *core.LayoutNode) {
		if n.Component == "" {
			return
		}
		c := findComponent(pg, n.Component)
		if c == nil {
			return
		}
		types[c.Type]++
		switch c.Type {
		case "header", "text":
		case "button":
			primary = primary || c.Emphasis == "action.primary"
		default:
			ok = false
		}
	})
	if !ok || !primary || types["header"] == 0 || types["text"] == 0 {
		return "", false
	}
	return first.ID, true
}

func findComponent(pg core.Page, id string) *core.PageComponent {
	for i := range pg.Components {
		if pg.Components[i].ID == id {
			return &pg.Components[i]
		}
	}
	return nil
}

// themeDrift reports design system tokens that no longer match the theme
// (the theme files changed after the design was made).
func themeDrift(d *core.Design, t *core.Theme) *core.QAIssue {
	if d == nil || d.System == nil || t == nil {
		return nil
	}
	want := map[string]string{}
	for _, tok := range slices.Concat(t.Colors, t.Typography, t.Spacing, t.Radius, t.Shadow) {
		want[tok.Name] = tok.Value
	}
	var diff []string
	for _, tok := range slices.Concat(d.System.Colors, d.System.Typography, d.System.Spacing, d.System.Radius, d.System.Shadow) {
		if v, ok := want[tok.Name]; ok && v != tok.Value {
			diff = append(diff, tok.Name)
		}
	}
	if len(diff) == 0 {
		return nil
	}
	return &core.QAIssue{Severity: "minor", Category: "theme", Source: "lint", Rule: "theme_token_drift",
		Problem: "Token desain tidak sama dengan theme " + t.Name + ": " + strings.Join(diff, ", "),
		Why:     "theme berubah setelah rancangan dibuat", Fix: "Pilih ulang theme agar token diterapkan lagi."}
}
