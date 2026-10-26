package uicheck

import (
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
)

// Colors are compared as WCAG relative luminance. Only colors a browser
// computes (hex, rgb(), hsl(), bare HSL channels) are read; an unknown value
// makes the contrast check untested instead of guessing.

var (
	rgbFn   = regexp.MustCompile(`(?i)^rgba?\(\s*([\d.]+%?)[\s,]+([\d.]+%?)[\s,]+([\d.]+%?)(?:[\s,/]+([\d.]+%?))?\s*\)$`)
	hslFn   = regexp.MustCompile(`(?i)^hsla?\(\s*([\d.]+)(?:deg)?[\s,]+([\d.]+)%?[\s,]+([\d.]+)%?(?:[\s,/]+([\d.]+%?))?\s*\)$`)
	hslBare = regexp.MustCompile(`^([\d.]+)\s+([\d.]+)%\s+([\d.]+)%(\s*/\s*([\d.]+%?))?$`)
	hexCol  = regexp.MustCompile(`(?i)^#([0-9a-f]{3,8})$`)
)

// color is an sRGB color with 0..1 channels and alpha.
type color struct {
	r, g, b, a float64
	ok         bool
}

func parseColor(v string) color {
	v = strings.TrimSpace(strings.ToLower(v))
	if v == "" || v == "transparent" {
		return color{}
	}
	switch v {
	case "white":
		return color{1, 1, 1, 1, true}
	case "black":
		return color{0, 0, 0, 1, true}
	}
	if m := hexCol.FindStringSubmatch(v); m != nil {
		h := m[1]
		switch len(h) {
		case 3, 4:
			full := make([]byte, 0, 8)
			for i := 0; i < len(h); i++ {
				full = append(full, h[i], h[i])
			}
			h = string(full)
		case 6, 8:
		default:
			return color{}
		}
		n, err := strconv.ParseUint(h[:6], 16, 32)
		if err != nil {
			return color{}
		}
		c := color{r: float64(n>>16&0xff) / 255, g: float64(n>>8&0xff) / 255, b: float64(n&0xff) / 255, a: 1, ok: true}
		if len(h) == 8 {
			al, err := strconv.ParseUint(h[6:8], 16, 8)
			if err != nil {
				return color{}
			}
			c.a = float64(al) / 255
		}
		return c
	}
	if m := rgbFn.FindStringSubmatch(v); m != nil {
		ch := func(s string) float64 {
			if strings.HasSuffix(s, "%") {
				f, _ := strconv.ParseFloat(strings.TrimSuffix(s, "%"), 64)
				return f / 100
			}
			f, _ := strconv.ParseFloat(s, 64)
			return f / 255
		}
		c := color{r: ch(m[1]), g: ch(m[2]), b: ch(m[3]), a: 1, ok: true}
		if m[4] != "" {
			c.a = alpha(m[4])
		}
		return c
	}
	if m := hslFn.FindStringSubmatch(v); m != nil {
		c := fromHSL(num(m[1]), num(m[2])/100, num(m[3])/100)
		if m[4] != "" {
			c.a = alpha(m[4])
		}
		return c
	}
	if m := hslBare.FindStringSubmatch(v); m != nil {
		c := fromHSL(num(m[1]), num(m[2])/100, num(m[3])/100)
		if m[5] != "" {
			c.a = alpha(m[5])
		}
		return c
	}
	return color{}
}

func num(s string) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSuffix(s, "%"), 64)
	return f
}

func alpha(s string) float64 {
	if strings.HasSuffix(s, "%") {
		return num(s) / 100
	}
	return num(s)
}

func fromHSL(h, s, l float64) color {
	f := func(n float64) float64 {
		k := math.Mod(n+h/30, 12)
		a := s * math.Min(l, 1-l)
		return l - a*math.Max(-1, math.Min(k-3, math.Min(9-k, 1)))
	}
	return color{r: f(0), g: f(8), b: f(4), a: 1, ok: true}
}

// blend puts a possibly translucent color over an opaque background.
func blend(fg, bg color) color {
	if !fg.ok {
		return bg
	}
	if fg.a >= 0.999 {
		return fg
	}
	a := fg.a
	return color{
		r: fg.r*a + bg.r*(1-a),
		g: fg.g*a + bg.g*(1-a),
		b: fg.b*a + bg.b*(1-a),
		a: 1, ok: true,
	}
}

func relativeLuminance(c color) float64 {
	lin := func(x float64) float64 {
		if x <= 0.03928 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c.r) + 0.7152*lin(c.g) + 0.0722*lin(c.b)
}

// contrastRatio is the WCAG contrast ratio, 1..21.
func contrastRatio(a, b color) float64 {
	la, lb := relativeLuminance(a), relativeLuminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// opaque reports whether the color is usable as a background sample.
func (c color) opaque() bool { return c.ok && c.a > 0.5 }

// lengthTokens reads px/rem values from design tokens.
func lengthTokens(tokens []core.Token) []float64 {
	var out []float64
	for _, t := range tokens {
		for _, v := range lengthValues(t.Value) {
			out = append(out, v)
		}
	}
	return out
}

var lengthRe = regexp.MustCompile(`(?i)(-?\d+(?:\.\d+)?)(px|rem|em)`)

func lengthValues(s string) []float64 {
	var out []float64
	for _, m := range lengthRe.FindAllStringSubmatch(s, -1) {
		f, _ := strconv.ParseFloat(m[1], 64)
		if strings.EqualFold(m[2], "rem") || strings.EqualFold(m[2], "em") {
			f *= 16
		}
		out = append(out, f)
	}
	return out
}

// core.Token values carry the length strings; lengthValues extracts px/rem.

func inSet(v float64, set []float64) bool {
	for _, x := range set {
		if math.Abs(v-x) <= 0.6 {
			return true
		}
	}
	return false
}
