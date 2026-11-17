package uikit

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
)

// Materialize fills the tokens of rules from their preset: the preset's
// tokens, adapted to the brand, then the rules' own tokens. Colors the
// rules name override the preset (a product may need its own status
// color); typography, spacing, radius, shadow and breakpoints of the
// preset are locked so every screen keeps the curated scale — the rules
// may only add names the preset lacks. Rules without a known preset are
// returned unchanged.
func (k *Kit) Materialize(r *core.DesignRules) {
	if r == nil || r.Preset == "" {
		return
	}
	p, ok := k.Preset(r.Preset)
	if !ok {
		return
	}
	base := cloneTokens(p.Tokens)
	// A primary color the rules set without a brand is the brand: hover,
	// text-on-primary, tint and focus follow it.
	if r.Brand == nil || r.Brand.Primary == "" {
		if i := indexToken(r.Tokens.Colors, "action.primary"); i >= 0 && validHex(r.Tokens.Colors[i].Value) {
			b := core.DesignBrand{Primary: strings.ToLower(r.Tokens.Colors[i].Value)}
			if r.Brand != nil {
				b.Font = r.Brand.Font
			}
			r.Brand = &b
		}
	}
	applyBrand(&base, r.Brand)
	own := r.Tokens
	baseGroups := base.Groups()
	for i, g := range own.Groups() {
		dst := baseGroups[i].Tokens
		for _, t := range *g.Tokens {
			j := indexToken(*dst, t.Name)
			switch {
			case j < 0:
				*dst = append(*dst, t)
			case g.Name == "colors" && !brandLocked(t.Name, r.Brand):
				(*dst)[j].Value = t.Value
				if t.Use != "" {
					(*dst)[j].Use = t.Use
				}
			}
		}
	}
	r.Tokens = base
}

// brandLocked: a color the brand sets wins over the same color in the
// rules.
func brandLocked(name string, b *core.DesignBrand) bool {
	if b == nil || !validHex(b.Primary) {
		return false
	}
	switch name {
	case "action.primary", "action.primary.hover", "action.primary.text", "action.secondary", "focus.ring":
		return true
	}
	return false
}

func applyBrand(t *core.DesignTokens, b *core.DesignBrand) {
	if b == nil {
		return
	}
	if c, ok := parseHex(b.Primary); ok {
		set := func(name, value string) {
			if i := indexToken(t.Colors, name); i >= 0 {
				t.Colors[i].Value = value
			} else {
				t.Colors = append(t.Colors, core.Token{Name: name, Value: value})
			}
		}
		set("action.primary", c.hex())
		set("action.primary.hover", c.mix(rgb{0, 0, 0}, 0.15).hex())
		set("action.primary.text", readableOn(c).hex())
		set("action.secondary", c.mix(rgb{255, 255, 255}, 0.9).hex())
		set("focus.ring", c.hex())
	}
	if f := strings.TrimSpace(b.Font); f != "" {
		for i := range t.Typography {
			if ft, ok := parseFont(t.Typography[i].Value); ok {
				ft.family = f
				t.Typography[i].Value = ft.token()
			}
		}
	}
}

func cloneTokens(t core.DesignTokens) core.DesignTokens {
	var out core.DesignTokens
	src, dst := t.Groups(), out.Groups()
	for i := range src {
		*dst[i].Tokens = append([]core.Token(nil), *src[i].Tokens...)
	}
	return out
}

func indexToken(ts []core.Token, name string) int {
	for i, t := range ts {
		if t.Name == name {
			return i
		}
	}
	return -1
}

// Resolve is every token a mockup needs: the preset of rules (or the
// default preset) adapted to the brand, overridden by every token the rules
// carry — an existing project's own tokens win by name.
func (k *Kit) Resolve(r *core.DesignRules) core.DesignTokens {
	id := DefaultPreset
	if r != nil && r.Preset != "" {
		if _, ok := k.Preset(r.Preset); ok {
			id = r.Preset
		}
	}
	p, ok := k.Preset(id)
	if !ok {
		return core.DesignTokens{}
	}
	out := cloneTokens(p.Tokens)
	if r == nil {
		return out
	}
	applyBrand(&out, r.Brand)
	groups := out.Groups()
	for i, g := range r.Tokens.Groups() {
		dst := groups[i].Tokens
		for _, t := range *g.Tokens {
			if j := indexToken(*dst, t.Name); j >= 0 {
				(*dst)[j].Value = t.Value
			} else {
				*dst = append(*dst, t)
			}
		}
	}
	return out
}

// CSSVar is the CSS custom property of a token ("action.primary" →
// "--action-primary").
func CSSVar(name string) string {
	var b strings.Builder
	b.WriteString("--")
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return b.String()
}

// cssValue makes a token value safe inside a style block: no characters
// that end the declaration, the rule or the element.
var unsafeCSS = regexp.MustCompile(`[;{}<>\\]|/\*|\*/`)

func cssValue(group, v string) string {
	v = strings.TrimSpace(unsafeCSS.ReplaceAllString(v, " "))
	switch group {
	case "typography":
		if f, ok := parseFont(v); ok {
			return f.shorthand()
		}
	case "spacing", "radius", "breakpoints":
		// A size with a note ("36px; 44px untuk sentuh") is its first
		// length; the rest would void the property.
		if !lengths.MatchString(v) {
			if m := firstLength.FindString(v); m != "" {
				return m
			}
		}
	}
	return v
}

var (
	lengths     = regexp.MustCompile(`^-?\d+(?:\.\d+)?(?:px|rem|em|%)?(?:\s+-?\d+(?:\.\d+)?(?:px|rem|em|%)?)*$`)
	firstLength = regexp.MustCompile(`-?\d+(?:\.\d+)?(?:px|rem|em|%)`)
)

// RootCSS is the :root block of every token as a CSS variable.
func RootCSS(t core.DesignTokens) string {
	var b strings.Builder
	b.WriteString(":root{")
	for _, g := range t.Groups() {
		for _, tok := range *g.Tokens {
			if tok.Name == "" || tok.Value == "" {
				continue
			}
			fmt.Fprintf(&b, "%s:%s;", CSSVar(tok.Name), cssValue(g.Name, tok.Value))
		}
	}
	b.WriteString("}")
	return b.String()
}

// font is a parsed typography token: "Inter 14px/20px 400".
type font struct {
	family, size, line, weight string
}

var (
	fontParts = regexp.MustCompile(`^(.*?)\s*(\d+(?:\.\d+)?(?:px|rem))(?:\s*/\s*(\d+(?:\.\d+)?(?:px|rem)?))?\s*(\d{3})?(?:\s+[a-z][a-z-]*)*\s*$`)
	// fontNote is a note a token carries: "(mobile 16px/24px)".
	fontNote = regexp.MustCompile(`\s*\([^)]*\)`)
)

// parseFont reads a typography token; a note in parentheses and words
// after the weight ("tabular-nums") are left out.
func parseFont(v string) (font, bool) {
	m := fontParts.FindStringSubmatch(strings.TrimSpace(fontNote.ReplaceAllString(v, "")))
	if m == nil {
		return font{}, false
	}
	f := font{family: strings.Trim(strings.TrimSpace(m[1]), `"',`), size: m[2], line: m[3], weight: m[4]}
	if f.weight == "" {
		f.weight = "400"
	}
	return f, true
}

func (f font) token() string {
	s := strings.TrimSpace(f.family + " " + f.size)
	if f.line != "" {
		s += "/" + f.line
	}
	return s + " " + f.weight
}

func (f font) shorthand() string {
	s := f.weight + " " + f.size
	if f.line != "" {
		s += "/" + f.line
	}
	fam := "system-ui, -apple-system, \"Segoe UI\", sans-serif"
	if f.family != "" {
		fam = `"` + strings.ReplaceAll(f.family, `"`, "") + `", ` + fam
	}
	return s + " " + fam
}

// Colors.

type rgb struct{ r, g, b float64 }

var hexRe = regexp.MustCompile(`^#([0-9a-fA-F]{6})$`)

func validHex(s string) bool { return hexRe.MatchString(strings.TrimSpace(s)) }

func parseHex(s string) (rgb, bool) {
	m := hexRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return rgb{}, false
	}
	n, _ := strconv.ParseUint(m[1], 16, 32)
	return rgb{float64(n >> 16 & 0xff), float64(n >> 8 & 0xff), float64(n & 0xff)}, true
}

func (c rgb) hex() string {
	cl := func(v float64) int { return int(math.Round(math.Max(0, math.Min(255, v)))) }
	return fmt.Sprintf("#%02x%02x%02x", cl(c.r), cl(c.g), cl(c.b))
}

// mix moves c toward o by w (0..1).
func (c rgb) mix(o rgb, w float64) rgb {
	return rgb{c.r + (o.r-c.r)*w, c.g + (o.g-c.g)*w, c.b + (o.b-c.b)*w}
}

func (c rgb) luminance() float64 {
	ch := func(v float64) float64 {
		v /= 255
		if v <= 0.03928 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	return 0.2126*ch(c.r) + 0.7152*ch(c.g) + 0.0722*ch(c.b)
}

func contrast(a, b rgb) float64 {
	la, lb := a.luminance(), b.luminance()
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// readableOn is white or near-black, whichever contrasts more with c.
func readableOn(c rgb) rgb {
	white, dark := rgb{255, 255, 255}, rgb{15, 23, 42}
	if contrast(c, white) >= contrast(c, dark) {
		return white
	}
	return dark
}
