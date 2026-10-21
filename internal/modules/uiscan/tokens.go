package uiscan

import (
	"fmt"
	"math"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
)

// def is a token value as the code declares it.
type def struct{ value, file string }

// tokens are the variables found in the stylesheets and Tailwind config.
type tokens struct {
	raw  map[string]def // declared name (without -- or $) → value, for var() lookups
	norm map[string]def // normalized name → value, for role matching
	font []def          // font-family candidates, in priority order
}

func (t *tokens) add(name, value, file string) {
	value = strings.TrimSpace(value)
	if name == "" || value == "" {
		return
	}
	if _, ok := t.raw[name]; !ok {
		t.raw[name] = def{value, file}
	}
	if n := normalize(name); n != "" {
		if _, ok := t.norm[n]; !ok {
			t.norm[n] = def{value, file}
		}
	}
}

var (
	prefixes = []string{"tw-", "bs-", "color-", "colors-", "clr-", "theme-", "c-"}
	suffixes = []string{"-color", "-colour"}
)

// normalize maps "--bs-primary", "--color-primary" or "$primary-color" to
// "primary".
func normalize(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	for changed := true; changed; {
		changed = false
		for _, p := range prefixes {
			if strings.HasPrefix(n, p) && len(n) > len(p) {
				n, changed = n[len(p):], true
			}
		}
	}
	for _, s := range suffixes {
		n = strings.TrimSuffix(n, s)
	}
	return n
}

// Tokens reads the design tokens the frontend's code declares: CSS custom
// properties, SCSS variables and the Tailwind config. Only what the code
// defines is returned; nothing is filled in. nil when the code defines too
// few colors to be a design system of its own. sources are the files the
// tokens came from.
func Tokens(dir string, ui *core.ExistingUI) (*core.DesignTokens, []string) {
	if ui == nil {
		return nil, nil
	}
	tk := collect(dir, ui.Paths)
	colors := pickColors(tk)
	if colors["action.primary"].value == "" || len(colors) < 4 {
		return nil, nil
	}
	used := map[string]bool{}
	from := func(d def) string {
		used[d.file] = true
		return "dari " + d.file
	}
	out := &core.DesignTokens{}
	for _, r := range colorRoles {
		if d, ok := colors[r.token]; ok {
			out.Colors = append(out.Colors, core.Token{Name: r.token, Value: d.value, Use: from(d)})
		}
	}
	for _, r := range radius(tk) {
		out.Radius = append(out.Radius, core.Token{Name: r.name, Value: r.value, Use: from(r.def)})
	}
	if f, d, ok := fontFamily(tk); ok {
		out.Typography = append(out.Typography, core.Token{Name: "font.family", Value: f, Use: from(d)})
	}
	sources := make([]string, 0, len(used))
	for f := range used {
		sources = append(sources, f)
	}
	sort.Strings(sources)
	return out, sources
}

// ---------- reading ----------

var styleExt = map[string]bool{".css": true, ".scss": true, ".pcss": true, ".postcss": true}

// collect reads the stylesheets and Tailwind configs of the frontend.
// Theme-like files (tokens, theme, variables, globals) are read first, so
// their definitions win.
func collect(dir string, roots []string) *tokens {
	tk := &tokens{raw: map[string]def{}, norm: map[string]def{}}
	var files []file
	for _, r := range roots {
		files = append(files, walk(dir, r, "", func(name string) bool {
			return (styleExt[filepath.Ext(name)] && !strings.HasSuffix(name, ".min.css")) || strings.HasPrefix(name, "tailwind.config.")
		})...)
	}
	rank := func(f file) int {
		n := strings.ToLower(stem(f.sub))
		for i, w := range []string{"token", "theme", "variable", "global", "index", "app", "main", "style"} {
			if strings.Contains(n, w) {
				return i
			}
		}
		return 99
	}
	sort.SliceStable(files, func(i, j int) bool { return rank(files[i]) < rank(files[j]) })
	var tailwind []file
	for _, f := range files {
		if strings.HasPrefix(filepath.Base(f.sub), "tailwind.config.") {
			tailwind = append(tailwind, f)
			continue
		}
		readStyles(read(dir, f.rel), f.rel, strings.HasSuffix(f.sub, ".scss"), tk)
	}
	// Tailwind names often point at the CSS variables read above.
	for _, f := range tailwind {
		readTailwind(read(dir, f.rel), f.rel, tk)
	}
	return tk
}

var comments = regexp.MustCompile(`(?s)/\*.*?\*/`)

// readStyles reads custom properties (and SCSS variables) outside dark
// mode blocks, and the font-family of the document.
func readStyles(src, file string, scss bool, tk *tokens) {
	src = comments.ReplaceAllString(src, "")
	var stack []string
	var seg strings.Builder
	decl := func() {
		d := strings.TrimSpace(seg.String())
		seg.Reset()
		for _, sel := range stack {
			if strings.Contains(strings.ToLower(sel), "dark") {
				return
			}
		}
		name, value, ok := strings.Cut(d, ":")
		if !ok {
			return
		}
		name = strings.TrimSpace(name)
		switch {
		case strings.HasPrefix(name, "--"):
			tk.add(name[2:], value, file)
		case scss && strings.HasPrefix(name, "$"):
			tk.add(name[1:], strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(value), "!default")), file)
		case name == "font-family" && len(stack) > 0:
			if sel := strings.ToLower(stack[len(stack)-1]); strings.Contains(sel, "body") || strings.Contains(sel, "html") || strings.Contains(sel, ":root") {
				tk.font = append(tk.font, def{value, file})
			}
		}
	}
	for i := 0; i < len(src); i++ {
		c := src[i]
		switch {
		case c == '#' && i+1 < len(src) && src[i+1] == '{': // SCSS interpolation
			end := strings.IndexByte(src[i:], '}')
			if end < 0 {
				return
			}
			seg.WriteString(src[i : i+end+1])
			i += end
		case c == '{':
			stack = append(stack, strings.TrimSpace(seg.String()))
			seg.Reset()
		case c == '}':
			decl()
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case c == ';':
			decl()
		default:
			seg.WriteByte(c)
		}
	}
}

var (
	tailwindColors = regexp.MustCompile(`\bcolors\s*:\s*\{`)
	tailwindSans   = regexp.MustCompile(`\bsans\s*:\s*\[\s*["'\x60]([^"'\x60]+)["'\x60]`)
)

// readTailwind reads the colors objects (theme.colors, theme.extend.colors)
// and the sans font of a Tailwind config.
func readTailwind(src, file string, tk *tokens) {
	src = comments.ReplaceAllString(src, "")
	for _, m := range tailwindColors.FindAllStringIndex(src, -1) {
		parseObject(src, m[1]-1, "", func(name, value string) { tk.add(name, value, file) })
	}
	if m := tailwindSans.FindStringSubmatch(src); m != nil {
		tk.font = append(tk.font, def{m[1], file})
	}
}

// parseObject reads a JS object literal at src[i] == '{' and reports its
// string values by path ("primary", "primary-500", "primary-foreground");
// DEFAULT keys take the parent's name. It returns the index after '}'.
func parseObject(src string, i int, prefix string, emit func(name, value string)) int {
	i++
	for i < len(src) {
		for i < len(src) && strings.ContainsRune(" \t\r\n,", rune(src[i])) {
			i++
		}
		if i >= len(src) {
			return i
		}
		if src[i] == '}' {
			return i + 1
		}
		var key string
		if q := src[i]; q == '"' || q == '\'' {
			end := strings.IndexByte(src[i+1:], q)
			if end < 0 {
				return len(src)
			}
			key, i = src[i+1:i+1+end], i+end+2
		} else {
			start := i
			for i < len(src) && (isIdent(src[i])) {
				i++
			}
			key = src[start:i]
		}
		for i < len(src) && strings.ContainsRune(" \t\r\n", rune(src[i])) {
			i++
		}
		if key == "" || i >= len(src) || src[i] != ':' {
			i = skipValue(src, i)
			continue
		}
		i++
		for i < len(src) && strings.ContainsRune(" \t\r\n", rune(src[i])) {
			i++
		}
		name := strings.ToLower(key)
		switch {
		case key == "DEFAULT":
			name = prefix
		case prefix != "":
			name = prefix + "-" + name
		}
		if i >= len(src) {
			return i
		}
		switch q := src[i]; {
		case q == '{':
			i = parseObject(src, i, name, emit)
		case q == '"' || q == '\'' || q == '`':
			end := strings.IndexByte(src[i+1:], q)
			if end < 0 {
				return len(src)
			}
			emit(name, src[i+1:i+1+end])
			i += end + 2
		default:
			i = skipValue(src, i)
		}
	}
	return i
}

func isIdent(c byte) bool {
	return c == '_' || c == '$' || c == '-' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// skipValue moves past a value the parser does not read (a reference,
// a function call, a spread) to the next ',' or the closing '}'.
func skipValue(src string, i int) int {
	depth := 0
	for ; i < len(src); i++ {
		switch src[i] {
		case '(', '[', '{':
			depth++
		case ')', ']':
			depth--
		case '}':
			if depth == 0 {
				return i
			}
			depth--
		case ',':
			if depth == 0 {
				return i + 1
			}
		}
	}
	return i
}

// ---------- resolving ----------

var (
	varRef   = regexp.MustCompile(`var\(\s*--([\w-]+)\s*(?:,\s*([^()]*))?\)`)
	scssRef  = regexp.MustCompile(`#\{\s*\$([\w-]+)\s*\}|\$([\w-]+)`)
	alphaVal = regexp.MustCompile(`\s*/\s*<alpha-value>`)
)

// resolve replaces var(--x) and $x by their values, a few levels deep.
func (t *tokens) resolve(v string) string {
	for depth := 0; depth < 5 && (strings.Contains(v, "var(") || strings.Contains(v, "$")); depth++ {
		v = varRef.ReplaceAllStringFunc(v, func(m string) string {
			s := varRef.FindStringSubmatch(m)
			if d, ok := t.raw[s[1]]; ok {
				return d.value
			}
			return strings.TrimSpace(s[2])
		})
		v = scssRef.ReplaceAllStringFunc(v, func(m string) string {
			s := scssRef.FindStringSubmatch(m)
			if d, ok := t.raw[s[1]+s[2]]; ok {
				return d.value
			}
			return m
		})
	}
	return strings.TrimSpace(alphaVal.ReplaceAllString(v, ""))
}

// ---------- colors ----------

// colorRoles: the code names that mean each color token, most specific
// first.
var colorRoles = []struct {
	token string
	names []string
}{
	{"surface.page", []string{"background", "bg", "page", "page-bg", "bg-page", "body-bg", "surface-page", "base-100", "app-bg", "canvas"}},
	{"text.primary", []string{"foreground", "text-primary", "text", "fg", "body-color", "body", "text-base", "base-content", "on-background", "on-surface"}},
	{"text.secondary", []string{"muted-foreground", "text-secondary", "text-muted", "secondary-text", "text-subtle", "fg-muted", "subtle"}},
	{"surface.primary", []string{"card", "surface", "panel", "surface-primary", "bg-surface", "popover", "paper"}},
	{"surface.secondary", []string{"muted", "secondary", "surface-secondary", "surface-alt", "bg-muted", "bg-subtle", "bg-secondary", "base-200"}},
	{"border.default", []string{"border", "border-default", "input", "divider", "line", "stroke", "base-300"}},
	{"action.primary", []string{"primary", "brand", "accent", "main"}},
	{"action.danger", []string{"destructive", "danger", "error", "negative"}},
	{"status.success", []string{"success", "positive", "green"}},
	{"status.warning", []string{"warning", "warn", "caution", "amber", "yellow"}},
	{"status.error", []string{"error", "danger", "destructive", "negative", "red"}},
	{"status.info", []string{"info", "notice", "sky", "blue"}},
}

// pickColors maps code colors onto the color tokens. Surfaces must sit
// close to the page background and text must contrast with it, so a
// "secondary" button color is not taken for a panel.
func pickColors(tk *tokens) map[string]def {
	out := map[string]def{}
	var page *[3]float64
	for _, r := range colorRoles {
		for _, n := range r.names {
			d, v, ok := tk.color(n)
			if !ok {
				continue
			}
			c, _ := rgbOf(v)
			if page != nil {
				diff := math.Abs(luminance(c) - luminance(*page))
				switch {
				case strings.HasPrefix(r.token, "surface.") && diff > 0.15,
					r.token == "text.primary" && diff < 0.4,
					r.token == "text.secondary" && diff < 0.2,
					r.token == "border.default" && diff > 0.5:
					continue
				}
			}
			if r.token == "surface.page" {
				page = &c
			}
			out[r.token] = def{v, d.file}
			break
		}
	}
	return out
}

// color finds a usable color under name or its common scale steps.
func (t *tokens) color(name string) (def, string, bool) {
	for _, suffix := range []string{"", "-default", "-500", "-600"} {
		d, ok := t.norm[name+suffix]
		if !ok {
			continue
		}
		if v, ok := toColor(t.resolve(d.value)); ok {
			return d, v, true
		}
	}
	return def{}, "", false
}

var (
	hexColor  = regexp.MustCompile(`(?i)^#([0-9a-f]{3}|[0-9a-f]{4}|[0-9a-f]{6}|[0-9a-f]{8})$`)
	funcColor = regexp.MustCompile(`(?i)^(rgb|rgba|hsl|hsla)\(([\d\s.,%/]+)\)$`)
	hslBare   = regexp.MustCompile(`^[\d.]+\s+[\d.]+%\s+[\d.]+%(\s*/\s*[\d.]+%?)?$`)
	rgbBare   = regexp.MustCompile(`^\d{1,3}\s+\d{1,3}\s+\d{1,3}$`)
	oklchFn   = regexp.MustCompile(`(?i)^oklch\(\s*([\d.]+)(%?)\s+([\d.]+)\s+([\d.]+)(?:deg)?\s*(?:/\s*[\d.]+%?)?\s*\)$`)
	named     = map[string]string{"white": "#ffffff", "black": "#000000"}
)

// toColor turns a CSS color into a token value (hex, rgb() or
// hsl()); shadcn-style bare "222 47% 11%" and oklch() are converted.
func toColor(v string) (string, bool) {
	v = strings.TrimSpace(strings.ReplaceAll(v, "deg", ""))
	switch {
	case hexColor.MatchString(v), funcColor.MatchString(v):
		return strings.ToLower(v), true
	case hslBare.MatchString(v):
		return "hsl(" + v + ")", true
	case rgbBare.MatchString(v):
		return "rgb(" + v + ")", true
	case named[strings.ToLower(v)] != "":
		return named[strings.ToLower(v)], true
	}
	if m := oklchFn.FindStringSubmatch(v); m != nil {
		l, _ := strconv.ParseFloat(m[1], 64)
		if m[2] == "%" {
			l /= 100
		}
		c, _ := strconv.ParseFloat(m[3], 64)
		h, _ := strconv.ParseFloat(m[4], 64)
		return oklchHex(l, c, h), true
	}
	return "", false
}

func oklchHex(l, c, h float64) string {
	a, b := c*math.Cos(h*math.Pi/180), c*math.Sin(h*math.Pi/180)
	l_ := math.Pow(l+0.3963377774*a+0.2158037573*b, 3)
	m_ := math.Pow(l-0.1055613458*a-0.0638541728*b, 3)
	s_ := math.Pow(l-0.0894841775*a-1.2914855480*b, 3)
	lin := [3]float64{
		4.0767416621*l_ - 3.3077115913*m_ + 0.2309699292*s_,
		-1.2684380046*l_ + 2.6097574011*m_ - 0.3413193965*s_,
		-0.0041960863*l_ - 0.7034186147*m_ + 1.7076147010*s_,
	}
	var out [3]int
	for i, x := range lin {
		if x <= 0.0031308 {
			x *= 12.92
		} else {
			x = 1.055*math.Pow(x, 1/2.4) - 0.055
		}
		out[i] = int(math.Round(math.Max(0, math.Min(1, x)) * 255))
	}
	return fmt.Sprintf("#%02x%02x%02x", out[0], out[1], out[2])
}

// rgbOf parses a value from toColor into 0..1 channels.
func rgbOf(v string) ([3]float64, bool) {
	if m := hexColor.FindStringSubmatch(v); m != nil {
		h := m[1]
		if len(h) <= 4 {
			h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
		}
		var c [3]float64
		for i := range c {
			n, _ := strconv.ParseUint(h[i*2:i*2+2], 16, 8)
			c[i] = float64(n) / 255
		}
		return c, true
	}
	m := funcColor.FindStringSubmatch(v)
	if m == nil {
		return [3]float64{}, false
	}
	parts := strings.FieldsFunc(m[2], func(r rune) bool { return r == ' ' || r == ',' || r == '/' })
	if len(parts) < 3 {
		return [3]float64{}, false
	}
	num := func(s string) float64 { f, _ := strconv.ParseFloat(strings.TrimSuffix(s, "%"), 64); return f }
	if strings.HasPrefix(strings.ToLower(m[1]), "rgb") {
		return [3]float64{num(parts[0]) / 255, num(parts[1]) / 255, num(parts[2]) / 255}, true
	}
	return hslRGB(num(parts[0]), num(parts[1])/100, num(parts[2])/100), true
}

func hslRGB(h, s, l float64) [3]float64 {
	f := func(n float64) float64 {
		k := math.Mod(n+h/30, 12)
		a := s * math.Min(l, 1-l)
		return l - a*math.Max(-1, math.Min(k-3, math.Min(9-k, 1)))
	}
	return [3]float64{f(0), f(8), f(4)}
}

// luminance is the relative luminance (WCAG) of a color.
func luminance(c [3]float64) float64 {
	lin := func(x float64) float64 {
		if x <= 0.03928 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c[0]) + 0.7152*lin(c[1]) + 0.0722*lin(c[2])
}

// ---------- radius and font ----------

var length = regexp.MustCompile(`^(\d+(?:\.\d+)?)(px|rem)$`)

type found struct {
	name, value string
	def
}

// radius takes the radius the code declares, at most small, medium and
// large.
func radius(tk *tokens) []found {
	var out []found
	for _, r := range []struct {
		token string
		names []string
	}{
		{"radius.sm", []string{"radius-sm", "border-radius-sm"}},
		{"radius.md", []string{"radius", "radius-md", "border-radius", "radius-base", "rounded", "border-radius-md"}},
		{"radius.lg", []string{"radius-lg", "border-radius-lg"}},
	} {
		for _, n := range r.names {
			d, ok := tk.norm[n]
			if !ok {
				continue
			}
			if v := tk.resolve(d.value); length.MatchString(v) {
				out = append(out, found{r.token, v, d})
				break
			}
		}
	}
	return out
}

var (
	fontName    = regexp.MustCompile(`^[\w][\w\s-]*$`)
	genericFont = map[string]bool{"sans-serif": true, "serif": true, "monospace": true, "inherit": true, "initial": true,
		"ui-sans-serif": true, "ui-serif": true, "ui-monospace": true, "system-ui": true, "-apple-system": true, "blinkmacsystemfont": true}
)

// fontFamily is the first named font family the code declares.
func fontFamily(tk *tokens) (string, def, bool) {
	cands := slices.Clone(tk.font)
	for _, n := range []string{"font-sans", "font-family", "font-body", "font-base", "font-primary", "font-sans-serif", "body-font-family", "font-family-sans-serif"} {
		if d, ok := tk.norm[n]; ok {
			cands = append([]def{d}, cands...)
		}
	}
	for _, d := range cands {
		if family := firstFamily(tk.resolve(d.value)); family != "" {
			return family, d, true
		}
	}
	return "", def{}, false
}

// firstFamily is the first named font of a font stack; a system font stack
// becomes system-ui.
func firstFamily(stack string) string {
	for i, f := range strings.Split(stack, ",") {
		f = strings.Trim(strings.TrimSpace(f), `"'`)
		if genericFont[strings.ToLower(f)] {
			if i == 0 && (strings.EqualFold(f, "system-ui") || strings.EqualFold(f, "-apple-system")) {
				return "system-ui"
			}
			continue
		}
		if fontName.MatchString(f) && !strings.Contains(strings.ToLower(f), "var") {
			return f
		}
		return ""
	}
	return ""
}
