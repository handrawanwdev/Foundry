package core

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
)

// Theme is a visual identity a design follows: personality, tokens,
// component style, the layouts and navigation it prefers and what it
// avoids. It is design context, never a page template: the designer still
// composes every page's Design IR, the theme only resolves its semantic
// tokens and styles. Kept apart from the IR.
type Theme struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Personality []string `json:"personality"`
	// Tokens is the theme's character in three words; the values are in
	// Colors, Typography, Spacing, Radius and Shadow (tokens.json).
	Tokens     ThemeTokens       `json:"tokens"`
	Colors     []Token           `json:"colors"`
	Typography []Token           `json:"typography"`
	Spacing    []Token           `json:"spacing"`
	Radius     []Token           `json:"radius"`
	Shadow     []Token           `json:"shadow"`
	Components map[string]string `json:"components"` // component → style (components.json)
	// Preferences and Avoid guide the designer and are measured by the
	// quality check; UseWhen is what theme selection matches.
	Preferences ThemePreferences `json:"preferences"`
	Avoid       []string         `json:"avoid,omitempty"`
	UseWhen     ThemeUse         `json:"use_when"`
	Rules       []ThemeRule      `json:"rules,omitempty"`
	// Interaction is how the theme's products behave (e.g. "inline edit
	// on the canvas"); guidance for the designer.
	Interaction []string `json:"interaction,omitempty"`
	// Reference is the website or app the theme was extracted from; nil
	// for hand-made themes.
	Reference *ThemeSource `json:"reference,omitempty"`
}

// ThemeSource is where a reference theme comes from and the design
// direction (layout philosophy) it carries.
type ThemeSource struct {
	URL      string `json:"url"`
	Name     string `json:"name"`
	Category string `json:"category"` // marketing | workspace | dashboard | editor | enterprise | consumer
}

// DesignReference is a website or app the user named as inspiration.
// Foundry only takes screenshots as evidence; the AI extracts its design
// language as Theme DNA and Foundry turns that into a theme. Nothing of the
// reference (HTML, CSS, text, assets) is copied.
type DesignReference struct {
	URL    string `json:"url"`
	Name   string `json:"name"`
	Notes  string `json:"notes,omitempty"` // what the user likes about it
	Status string `json:"status"`          // pending | analyzing | done | failed
	// Shots are the screenshots the analysis looked at (evidence only).
	Shots   []QAShot  `json:"shots,omitempty"`
	DNA     *ThemeDNA `json:"dna,omitempty"`
	ThemeID string    `json:"theme_id,omitempty"`
	Error   string    `json:"error,omitempty"`
	At      time.Time `json:"at"`
}

// ThemeDNA is the design language of a reference: principles and roles,
// never its markup, styles, text or assets.
type ThemeDNA struct {
	Reference   string        `json:"reference"`
	Category    string        `json:"category"`
	Personality []string      `json:"personality"`
	Layout      []string      `json:"layout"`     // layout strategies it favours
	Navigation  string        `json:"navigation"` // side | rail | top
	Density     string        `json:"density"`    // compact | comfortable | spacious
	Visual      DNAVisual     `json:"visual"`
	Typography  DNATypography `json:"typography"`
	// Colors by semantic role: primary, background, surface, surface_alt,
	// text, text_muted, border, success, warning, danger, info.
	Colors      map[string]string `json:"colors"`
	Components  DNAComponents     `json:"components"`
	Interaction []string          `json:"interaction,omitempty"`
	Rules       []ThemeRule       `json:"rules,omitempty"`
	Avoid       []string          `json:"avoid,omitempty"`
}

type DNAVisual struct {
	ColorMode string `json:"color_mode"` // light | dark
	Spacing   string `json:"spacing"`    // compact | comfortable | spacious
	Surface   string `json:"surface"`    // soft | bordered | elevated | flat
	Radius    string `json:"radius"`     // small | medium | large
}

type DNATypography struct {
	Scale       string `json:"scale"`       // compact | balanced | large
	Personality string `json:"personality"` // geometric | humanist | technical
	Weights     string `json:"weights,omitempty"`
	Readability string `json:"readability,omitempty"`
}

type DNAComponents struct {
	Button string `json:"button"` // solid | outline | soft
	Nav    string `json:"nav"`    // dark | light | accent
}

type ThemeTokens struct {
	ColorMode string `json:"color_mode"` // light | dark
	Radius    string `json:"radius"`     // small | medium | large
	Spacing   string `json:"spacing"`    // compact | comfortable | spacious
}

type ThemePreferences struct {
	Navigation string   `json:"navigation,omitempty"` // side | rail | top
	Layouts    []string `json:"layout,omitempty"`     // layout strategies it suits
	Density    string   `json:"density,omitempty"`    // compact | comfortable | spacious
}

// ThemeUse: the product domains the theme is made for, and when not.
type ThemeUse struct {
	Domains []string `json:"domains,omitempty"`
	Not     []string `json:"not,omitempty"`
}

// ThemeRule is theme knowledge: IF/THEN with its reason.
type ThemeRule struct {
	If   string `json:"if"`
	Then string `json:"then"`
	Why  string `json:"why"`
}

// ThemeContext is the theme a design follows, as the renderer and the
// quality check read it. Source: user (picked), auto (selected from the
// requirement), fallback (the picked theme was not available).
type ThemeContext struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Source    string            `json:"source"`
	Reasons   []string          `json:"reasons,omitempty"`
	ColorMode string            `json:"color_mode"`
	Style     map[string]string `json:"style,omitempty"`
}

// Theme vocabulary.
var (
	ThemeColorTokens = []string{"text.primary", "text.secondary", "surface.page", "surface.primary", "surface.secondary", "border.default",
		"action.primary", "action.danger", "status.success", "status.warning", "status.error", "status.info"}
	ThemeTypeTokens    = []string{"type.display", "type.heading", "type.body", "type.caption"}
	ThemeSpaceTokens   = []string{"space.xs", "space.sm", "space.md", "space.lg", "space.xl"}
	ThemeRadiusTokens  = []string{"radius.sm", "radius.md", "radius.lg"}
	ThemeShadowTokens  = []string{"elevation.raised", "elevation.overlay"}
	ThemeColorModes    = []string{"light", "dark"}
	ThemeRadii         = []string{"small", "medium", "large"}
	ThemeSpacings      = []string{"compact", "comfortable", "spacious"}
	ThemeNavigations   = []string{"side", "rail", "top"}
	ThemeLayouts       = []string{"focused", "sections", "list", "table", "master-detail", "workspace", "dashboard", "steps"}
	ThemeDomains       = []string{"trading", "finance", "ticketing", "developer_tools", "admin_operations"}
	ThemeComponentKeys = map[string][]string{
		"button": {"solid", "outline", "soft"},
		"card":   {"bordered", "flat", "elevated"},
		"nav":    {"dark", "light", "accent"},
		"table":  {"lined", "striped", "plain"},
		"font":   {"sans", "mono"},
	}
	// ThemeAvoid: anti-patterns a theme may forbid. hero_section,
	// large_cards and excessive_whitespace are measured on the IR; the
	// others cannot be expressed in the IR and only guide the designer.
	// ThemeCategories: the layout philosophy of a reference.
	ThemeCategories = []string{"marketing", "workspace", "dashboard", "editor", "enterprise", "consumer"}
	ThemeAvoid      = []string{"hero_section", "large_cards", "excessive_whitespace", "decorative_gradient", "illustration", "glassmorphism"}
)

var (
	themeID     = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	colorValue  = regexp.MustCompile(`(?i)^(#[0-9a-f]{3,8}|(rgb|rgba|hsl|hsla)\([\d\s.,%/]+\))$`)
	lengthValue = regexp.MustCompile(`^\d+(\.\d+)?(px|rem)$`)
	typeValue   = regexp.MustCompile(`\d+(\.\d+)?px`)
	shadowValue = regexp.MustCompile(`(?i)^(none|[\d\s.,#()a-z%-]+)$`)
)

// ThemeInvalid is a theme that cannot be used; selection falls back to a
// valid one.
type ThemeInvalid struct {
	ID       string
	Problems []string
}

func (e *ThemeInvalid) Error() string {
	return "THEME_INVALID " + e.ID + "\n\n- " + strings.Join(e.Problems, "\n- ")
}

// ValidateTheme checks a theme: identity, every required token with a
// well-formed value, known component styles and preferences.
func ValidateTheme(t Theme) error {
	var out []string
	bad := func(format string, a ...any) { out = append(out, fmt.Sprintf(format, a...)) }
	oneOf := func(what, v string, set []string) {
		if v != "" && !slices.Contains(set, v) {
			bad("%s %q tidak dikenal (pakai: %s)", what, v, strings.Join(set, ", "))
		}
	}
	if !themeID.MatchString(t.ID) {
		bad("id %q harus huruf kecil, angka dan tanda hubung", t.ID)
	}
	if strings.TrimSpace(t.Name) == "" {
		bad("name kosong")
	}
	if len(t.Personality) == 0 {
		bad("personality kosong")
	}
	if t.Tokens.ColorMode == "" {
		bad("tokens.color_mode kosong")
	}
	oneOf("tokens.color_mode", t.Tokens.ColorMode, ThemeColorModes)
	oneOf("tokens.radius", t.Tokens.Radius, ThemeRadii)
	oneOf("tokens.spacing", t.Tokens.Spacing, ThemeSpacings)
	for _, g := range []struct {
		group string
		toks  []Token
		names []string
		value *regexp.Regexp
	}{
		{"colors", t.Colors, ThemeColorTokens, colorValue},
		{"typography", t.Typography, ThemeTypeTokens, typeValue},
		{"spacing", t.Spacing, ThemeSpaceTokens, lengthValue},
		{"radius", t.Radius, ThemeRadiusTokens, lengthValue},
		{"shadow", t.Shadow, ThemeShadowTokens, shadowValue},
	} {
		seen := map[string]bool{}
		for _, tok := range g.toks {
			switch {
			case !slices.Contains(g.names, tok.Name):
				bad("token %s tidak dikenal di %s (pakai: %s)", tok.Name, g.group, strings.Join(g.names, ", "))
			case seen[tok.Name]:
				bad("token %s didefinisikan lebih dari sekali", tok.Name)
			case !g.value.MatchString(strings.TrimSpace(tok.Value)) || strings.ContainsAny(tok.Value, ";{}") || strings.Contains(strings.ToLower(tok.Value), "url"):
				bad("token %s: nilai %q tidak valid", tok.Name, tok.Value)
			}
			seen[tok.Name] = true
		}
		for _, n := range g.names {
			if !seen[n] {
				bad("token %s tidak ada", n)
			}
		}
	}
	for k, v := range t.Components {
		set, ok := ThemeComponentKeys[k]
		if !ok {
			bad("component style %q tidak dikenal", k)
			continue
		}
		oneOf("component style "+k, v, set)
	}
	oneOf("preferences.navigation", t.Preferences.Navigation, ThemeNavigations)
	oneOf("preferences.density", t.Preferences.Density, ThemeSpacings)
	for _, l := range t.Preferences.Layouts {
		oneOf("preferences.layout", l, ThemeLayouts)
	}
	for _, a := range t.Avoid {
		oneOf("avoid", a, ThemeAvoid)
	}
	for _, d := range append(slices.Clone(t.UseWhen.Domains), t.UseWhen.Not...) {
		oneOf("use_when domain", d, ThemeDomains)
	}
	if r := t.Reference; r != nil {
		if r.URL == "" || r.Name == "" {
			bad("reference butuh url dan name")
		}
		oneOf("reference.category", r.Category, ThemeCategories)
	}
	if len(out) > 0 {
		return &ThemeInvalid{ID: t.ID, Problems: out}
	}
	return nil
}
