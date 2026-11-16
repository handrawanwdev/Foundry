package uikit

import (
	"fmt"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
)

// PresetBrief is the prompt section that lets the designer pick a preset
// instead of inventing token values.
func (k *Kit) PresetBrief() string {
	var b strings.Builder
	b.WriteString("UI KIT PRESETS (rules.preset; Foundry fills every token from it)\n")
	for _, p := range k.Presets {
		fmt.Fprintf(&b, "- %s: %s (%s)\n", p.ID, clause(p.Fit), presetScale(p.Tokens))
	}
	b.WriteString(`Adapt with rules.brand {"primary": "#RRGGBB", "font": "Family"} only when the product needs its own accent or typeface (primary readable with white or near-black text).
`)
	return b.String()
}

// clause is the text up to its first colon, semicolon or period.
func clause(s string) string {
	if i := strings.IndexAny(s, ":.;"); i > 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}

func presetScale(t core.DesignTokens) string {
	get := func(ts []core.Token, n string) string {
		if i := indexToken(ts, n); i >= 0 {
			if f, ok := parseFont(ts[i].Value); ok {
				return f.size
			}
			return ts[i].Value
		}
		return "-"
	}
	var space []string
	for _, tok := range t.Spacing {
		if strings.HasPrefix(tok.Name, "space.") {
			space = append(space, strings.TrimSuffix(tok.Value, "px"))
		}
	}
	return fmt.Sprintf("body %s, space %s, control %s, radius %s",
		get(t.Typography, "type.body"), strings.Join(space, "/"), get(t.Spacing, "size.control"), get(t.Radius, "radius.md"))
}

// PatternBrief is the prompt section that makes every page a template with
// slots to fill.
func (k *Kit) PatternBrief() string {
	var b strings.Builder
	b.WriteString("UI KIT PAGE TEMPLATES (pages[].pattern → its slots for sections[].slot; * required)\n")
	var shells []string
	for _, p := range k.Patterns {
		var slots []string
		for _, s := range p.Slots {
			if s.Required {
				slots = append(slots, s.ID+"*")
			} else {
				slots = append(slots, s.ID)
			}
		}
		if p.Shell {
			shells = append(shells, p.ID)
			continue
		}
		fmt.Fprintf(&b, "- %s: %s | %s\n", p.ID, clause(p.Use), strings.Join(slots, ", "))
	}
	if len(shells) > 0 {
		fmt.Fprintf(&b, "- shells %s: nav, topbar, content\n", strings.Join(shells, ", "))
	}
	b.WriteString(`Several sections may share a slot. Other patterns are allowed; their sections take no slot. Foundry renders each page as an HTML mockup.
`)
	return b.String()
}
