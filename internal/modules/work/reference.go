package work

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/handrawanw/foundry/internal/core"
)

// Reference based design: a website or app the user names is analyzed for
// its design language (Theme DNA), and Foundry turns the DNA into a theme.
// The reference is never a template: no HTML, CSS, text or assets are
// taken, only principles, applied to this product.

const KindReference = "reference"

// ParseReference checks a reference URL (http/https, a host) and names it
// after its host. A missing scheme means https.
func ParseReference(raw, notes string) (*core.DesignReference, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("isi alamat website referensi")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || strings.ContainsAny(u.Hostname(), " _") {
		return nil, fmt.Errorf("referensi %q bukan alamat website yang valid (pakai http/https)", strings.TrimSpace(raw))
	}
	u.Fragment = ""
	return &core.DesignReference{URL: u.String(), Name: referenceName(u.Hostname()), Notes: strings.TrimSpace(notes), Status: "pending", At: time.Now().UTC()}, nil
}

// referenceName: "www.voiceflow.com" → "Voiceflow", "app.example.co.id" → "Example".
func referenceName(host string) string {
	labels := strings.Split(strings.ToLower(host), ".")
	if net := regexp.MustCompile(`^[\d.]+$`); net.MatchString(host) || len(labels) == 1 {
		return host
	}
	i := len(labels) - 2
	if i > 0 && slices.Contains([]string{"co", "com", "ac", "or", "go", "net", "org", "web"}, labels[i]) {
		i--
	}
	name := labels[i]
	return strings.ToUpper(name[:1]) + name[1:]
}

const dnaShape = `{
  "reference": "<name>",
  "category": "marketing|workspace|dashboard|editor|enterprise|consumer" (its layout philosophy: what kind of product screen it is),
  "personality": ["3-6 words, e.g. modern, creative, technical"],
  "layout": ["layout strategies it favours: focused|sections|list|table|master-detail|workspace|dashboard|steps"],
  "navigation": "side|rail|top", "density": "compact|comfortable|spacious",
  "visual": {"color_mode": "light|dark", "spacing": "compact|comfortable|spacious", "surface": "soft|bordered|elevated|flat", "radius": "small|medium|large"},
  "typography": {"scale": "compact|balanced|large", "personality": "geometric|humanist|technical", "weights": "how weight is used", "readability": "short note"},
  "colors": {"primary": "#hex", "background": "#hex", "surface": "#hex", "surface_alt": "#hex", "text": "#hex", "text_muted": "#hex", "border": "#hex",
             "success": "#hex", "warning": "#hex", "danger": "#hex", "info": "#hex"},
  "components": {"button": "solid|outline|soft", "nav": "dark|light|accent"},
  "interaction": ["interaction patterns, e.g. inline editing, command palette, side panel opens on select"],
  "rules": [{"if": "", "then": "", "why": ""}],
  "avoid": ["what does not fit this language, e.g. hero_section, large_cards, excessive_whitespace, decorative_gradient, illustration, glassmorphism or a short phrase"]
}`

// ReferencePrompt asks the AI for the Theme DNA of the reference.
func ReferencePrompt(p *core.Project) string {
	r := p.Reference
	var b strings.Builder
	b.WriteString(`You are the design analyst of Foundry. Extract the design language (Theme DNA) of the reference below so Foundry can design a NEW product in the same spirit.
Do not write code. Do not modify files.

Analyze principles only: typography (heading scale, font personality, weight usage, readability), color system (the role of primary and accent,
contrast, semantic colors), layout philosophy (marketing, workspace, dashboard, editor, enterprise or consumer), component personality (buttons, cards/surfaces,
navigation) and interaction patterns.
Never copy: no HTML, no CSS, no text or slogans, no logos, illustrations or other brand assets, no page structure to clone.
Colors are semantic roles (primary, background, surface, text, success, ...): keep the reference's color character (hue family, contrast,
saturation), not its exact brand identity; text colors must reach WCAG AA on their background.
The rules you extract must hold for the new product, which is different from the reference.

`)
	fmt.Fprintf(&b, "REFERENCE: %s (%s)\n", r.Name, r.URL)
	if r.Notes != "" {
		b.WriteString("WHAT THE USER LIKES ABOUT IT: " + r.Notes + "\n")
	}
	if len(r.Shots) > 0 {
		b.WriteString("SCREENSHOTS (open these image files; evidence only):\n")
		for _, s := range r.Shots {
			fmt.Fprintf(&b, "- %s: %s\n", s.Device, s.Image)
		}
	} else {
		b.WriteString("No screenshot was taken (no browser available). Analyze the URL if you can open it.\n")
	}
	b.WriteString("When you cannot see a real product UI (error page, blank page, only a login wall, unknown site), reply only {\"error\": \"<why>\"}.\n\n")
	b.WriteString("THE NEW PRODUCT (apply the principles to this, not to the reference):\n" + p.Story + "\n")
	if d := p.Discovery; d != nil {
		b.WriteString(d.Summary + "\n")
	}
	b.WriteString("\nReply with ONLY one JSON object in a ```json block, shape:\n" + dnaShape)
	return b.String()
}

// dnaLayouts maps the reference vocabulary onto the layout strategies.
var dnaLayouts = map[string]string{"canvas": "workspace", "editor": "workspace", "builder": "workspace", "split-panel": "master-detail",
	"split-view": "master-detail", "split": "master-detail", "grid": "dashboard", "wizard": "steps", "form": "focused"}

var (
	dnaSpacing = map[string][]string{"compact": {"4px", "6px", "12px", "16px", "24px"}, "comfortable": {"4px", "8px", "16px", "24px", "32px"},
		"spacious": {"4px", "8px", "20px", "32px", "48px"}}
	dnaRadius = map[string][]string{"small": {"2px", "4px", "6px"}, "medium": {"6px", "10px", "14px"}, "large": {"10px", "16px", "24px"}}
	dnaType   = map[string][]string{"compact": {"24px/30px 700", "18px/24px 600", "13px/18px 400", "11px/14px 400"},
		"balanced": {"28px/34px 700", "22px/28px 600", "15px/22px 400", "12px/16px 400"},
		"large":    {"36px/44px 700", "26px/32px 600", "16px/24px 400", "13px/18px 400"}}
	dnaCard = map[string]string{"soft": "flat", "flat": "flat", "bordered": "bordered", "elevated": "elevated"}
	// colorRoles: DNA role → theme token. surface_alt and info may be
	// omitted.
	colorRoles = [][2]string{{"text", "text.primary"}, {"text_muted", "text.secondary"}, {"background", "surface.page"}, {"surface", "surface.primary"},
		{"surface_alt", "surface.secondary"}, {"border", "border.default"}, {"primary", "action.primary"}, {"danger", "action.danger"},
		{"success", "status.success"}, {"warning", "status.warning"}, {"danger", "status.error"}, {"info", "status.info"}}
)

// ThemeFromDNA turns the analysis into a theme: roles become semantic
// tokens, the visual words pick Foundry's own scales. The theme is
// validated like any other.
func ThemeFromDNA(r core.DesignReference, dna core.ThemeDNA) (core.Theme, error) {
	var bad []string
	low := func(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
	pick := func(what, v string, set []string, def string) string {
		v = low(v)
		if v == "" {
			return def
		}
		if !slices.Contains(set, v) {
			bad = append(bad, fmt.Sprintf("%s %q tidak dikenal (pakai: %s)", what, v, strings.Join(set, ", ")))
		}
		return v
	}
	vis := dna.Visual
	mode := pick("visual.color_mode", vis.ColorMode, core.ThemeColorModes, "")
	if mode == "" {
		bad = append(bad, "visual.color_mode kosong")
	}
	density := pick("density", cmpOr(dna.Density, vis.Spacing), core.ThemeSpacings, "comfortable")
	spacing := pick("visual.spacing", cmpOr(vis.Spacing, density), core.ThemeSpacings, "comfortable")
	radius := pick("visual.radius", vis.Radius, core.ThemeRadii, "medium")
	surface := pick("visual.surface", vis.Surface, []string{"soft", "bordered", "elevated", "flat"}, "bordered")
	scale := pick("typography.scale", dna.Typography.Scale, []string{"compact", "balanced", "large"}, "balanced")
	font := pick("typography.personality", dna.Typography.Personality, []string{"geometric", "humanist", "technical"}, "geometric")
	button := pick("components.button", dna.Components.Button, core.ThemeComponentKeys["button"], "solid")
	nav := pick("components.nav", dna.Components.Nav, core.ThemeComponentKeys["nav"], "dark")
	navigation := pick("navigation", dna.Navigation, core.ThemeNavigations, "side")
	category := pick("category", dna.Category, core.ThemeCategories, "")
	if category == "" {
		bad = append(bad, "category kosong")
	}

	var personality []string
	for _, w := range dna.Personality {
		if w = low(w); w != "" && !slices.Contains(personality, w) && len(personality) < 6 {
			personality = append(personality, w)
		}
	}
	if len(personality) == 0 {
		bad = append(bad, "personality kosong")
	}
	var layouts []string
	for _, l := range dna.Layout {
		l = low(l)
		if m, ok := dnaLayouts[l]; ok {
			l = m
		}
		if slices.Contains(LayoutStrategies, l) && !slices.Contains(layouts, l) {
			layouts = append(layouts, l)
		}
	}
	if len(layouts) == 0 {
		bad = append(bad, "layout tidak berisi strategi yang dikenal (pakai: "+strings.Join(LayoutStrategies, ", ")+")")
	}

	colors := map[string]string{}
	for k, v := range dna.Colors {
		colors[low(k)] = strings.TrimSpace(v)
	}
	colors["surface_alt"] = cmpOr(colors["surface_alt"], colors["surface"])
	colors["info"] = cmpOr(colors["info"], colors["primary"])
	var toks []core.Token
	for _, cr := range colorRoles {
		if colors[cr[0]] == "" {
			if !slices.ContainsFunc(bad, func(s string) bool { return strings.HasSuffix(s, "warna "+cr[0]+" tidak ada") }) {
				bad = append(bad, "warna "+cr[0]+" tidak ada")
			}
			continue
		}
		toks = append(toks, core.Token{Name: cr[1], Value: colors[cr[0]], Use: "role " + cr[0]})
	}
	if len(bad) > 0 {
		return core.Theme{}, &core.ThemeInvalid{ID: "reference " + r.Name, Problems: bad}
	}

	id := strings.Trim(regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(strings.ToLower(r.Name), "-"), "-") + "-inspired"
	t := core.Theme{ID: id, Name: r.Name + " Inspired", Personality: personality,
		Description: fmt.Sprintf("Bahasa desain %s (%s), diterapkan ke produk ini; bukan salinan.", r.Name, category),
		Tokens:      core.ThemeTokens{ColorMode: mode, Radius: radius, Spacing: spacing},
		Colors:      toks,
		Preferences: core.ThemePreferences{Navigation: navigation, Layouts: layouts, Density: density},
		Reference:   &core.ThemeSource{URL: r.URL, Name: r.Name, Category: category},
		Components: map[string]string{"button": button, "card": dnaCard[surface], "nav": nav,
			"table": map[bool]string{true: "plain", false: "lined"}[surface == "soft" || surface == "flat"],
			"font":  map[bool]string{true: "mono", false: "sans"}[font == "technical"]},
	}
	for i, n := range core.ThemeTypeTokens {
		t.Typography = append(t.Typography, core.Token{Name: n, Value: "Inter " + dnaType[scale][i]})
	}
	for i, n := range core.ThemeSpaceTokens {
		t.Spacing = append(t.Spacing, core.Token{Name: n, Value: dnaSpacing[spacing][i]})
	}
	for i, n := range core.ThemeRadiusTokens {
		t.Radius = append(t.Radius, core.Token{Name: n, Value: dnaRadius[radius][i]})
	}
	shade := map[bool]string{true: ".5", false: ".1"}[mode == "dark"]
	raised := "0 1px 2px rgba(0,0,0," + shade + ")"
	if surface == "elevated" {
		raised += ", 0 8px 24px rgba(0,0,0," + shade + ")"
	}
	t.Shadow = []core.Token{{Name: "elevation.raised", Value: raised}, {Name: "elevation.overlay", Value: "0 12px 40px rgba(0,0,0," + shade + ")"}}
	for _, a := range dna.Avoid {
		if a = strings.TrimSpace(a); a == "" {
			continue
		}
		if k := low(strings.ReplaceAll(a, " ", "_")); slices.Contains(core.ThemeAvoid, k) {
			if !slices.Contains(t.Avoid, k) {
				t.Avoid = append(t.Avoid, k)
			}
			continue
		}
		t.Rules = append(t.Rules, core.ThemeRule{If: "a page is composed in this direction", Then: "avoid " + a, Why: "it conflicts with the " + r.Name + " design language"})
	}
	for _, rule := range dna.Rules {
		if rule.If != "" && rule.Then != "" && len(t.Rules) < 10 {
			t.Rules = append(t.Rules, rule)
		}
	}
	for _, it := range dna.Interaction {
		if it = strings.TrimSpace(it); it != "" && len(t.Interaction) < 6 {
			t.Interaction = append(t.Interaction, it)
		}
	}
	return t, core.ValidateTheme(t)
}

func cmpOr(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

// referenceOut is the AI's reply: the DNA, or why the reference could not
// be analyzed.
type referenceOut struct {
	core.ThemeDNA
	Error string `json:"error"`
}

// ApplyReference lands the analysis: an invalid reference is recorded as
// failed; a valid DNA becomes a theme in .foundry/themes/<id>/ and the
// project's theme. Returns the theme, nil when the reference failed.
func ApplyReference(p *core.Project, raw string) (*core.Theme, error) {
	r := p.Reference
	if r == nil {
		return nil, errors.New("tidak ada referensi yang dianalisis")
	}
	var out referenceOut
	if err := ExtractJSON(raw, &out); err != nil {
		return nil, err
	}
	if out.Error != "" {
		r.Status, r.Error = "failed", strings.TrimSpace(out.Error)
		return nil, nil
	}
	t, err := ThemeFromDNA(*r, out.ThemeDNA)
	if err != nil {
		return nil, err
	}
	if err := saveTheme(p.Dir, t, out.ThemeDNA); err != nil {
		return nil, err
	}
	dna := out.ThemeDNA
	r.Status, r.Error, r.DNA, r.ThemeID = "done", "", &dna, t.ID
	p.Theme = t.ID
	return &t, nil
}

// saveTheme writes a theme in the layout LoadTheme reads, plus the DNA it
// came from (reference.json) for traceability.
func saveTheme(dir string, t core.Theme, dna core.ThemeDNA) error {
	out := filepath.Join(dir, ProjectThemesDir, t.ID)
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	tokens := struct {
		Colors     []core.Token `json:"colors"`
		Typography []core.Token `json:"typography"`
		Spacing    []core.Token `json:"spacing"`
		Radius     []core.Token `json:"radius"`
		Shadow     []core.Token `json:"shadow"`
	}{t.Colors, t.Typography, t.Spacing, t.Radius, t.Shadow}
	head := t
	head.Colors, head.Typography, head.Spacing, head.Radius, head.Shadow, head.Components = nil, nil, nil, nil, nil, nil
	for name, v := range map[string]any{"theme.json": head, "tokens.json": tokens, "components.json": t.Components, "reference.json": dna} {
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(out, name), append(b, '\n'), 0o644); err != nil {
			return err
		}
	}
	return nil
}
