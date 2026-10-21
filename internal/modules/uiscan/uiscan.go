// Package uiscan reads the frontend of an existing project: its routes,
// layout shell, navigation, shared components and design tokens, so new
// pages are designed to fit into the product that is already there.
// Everything comes from the code; nothing is guessed.
package uiscan

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode"

	"github.com/handrawanwdev/foundry/internal/core"
)

var skipDirs = map[string]bool{
	"node_modules": true, ".git": true, core.StateDir: true, "dist": true, "build": true, "out": true,
	"vendor": true, ".next": true, ".nuxt": true, ".svelte-kit": true, "target": true, "coverage": true,
	"public": true, "storybook-static": true, "__tests__": true, "e2e": true,
}

// Limits keep the scan quick on big repositories and the prompt short.
const (
	maxDepth      = 7
	maxFileSize   = 512 << 10
	maxRoutes     = 60
	maxShell      = 15
	maxNavItems   = 20
	maxComponents = 40
)

var sourceExt = map[string]bool{".tsx": true, ".jsx": true, ".ts": true, ".js": true, ".vue": true, ".svelte": true}

// file is one frontend source file, rel to the project root.
type file struct {
	rel  string // slash separated, from the project root
	sub  string // slash separated, from its component root
	tech string // the component's tech
}

// Scan reads the frontend components (web, desktop, mobile) of an existing
// project. nil when there is no frontend.
func Scan(dir string, comps []core.Component) *core.ExistingUI {
	ui := &core.ExistingUI{}
	var files []file
	for _, c := range comps {
		if c.Kind != "web" && c.Kind != "desktop" && c.Kind != "mobile" {
			continue
		}
		if slices.Contains(ui.Paths, c.Path) {
			continue
		}
		ui.Paths = append(ui.Paths, c.Path)
		files = append(files, walk(dir, c.Path, c.Tech, func(name string) bool {
			return sourceExt[filepath.Ext(name)] && !isTest(name)
		})...)
	}
	if len(ui.Paths) == 0 {
		return nil
	}
	ui.Routes = routes(dir, files)
	ui.Shell, ui.Navigation = shell(files)
	ui.NavItems = navItems(dir, files)
	ui.Components = components(files)
	return ui
}

// walk lists the files under root that keep accepts.
func walk(dir, root, tech string, keep func(name string) bool) []file {
	var out []file
	base := filepath.Join(dir, root)
	filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(base, path)
		if d.IsDir() {
			if path != base && (skipDirs[d.Name()] || strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			if strings.Count(rel, string(filepath.Separator)) >= maxDepth {
				return filepath.SkipDir
			}
			return nil
		}
		if !keep(d.Name()) {
			return nil
		}
		if info, err := d.Info(); err != nil || info.Size() > maxFileSize {
			return nil
		}
		full, _ := filepath.Rel(dir, path)
		out = append(out, file{rel: filepath.ToSlash(full), sub: filepath.ToSlash(rel), tech: tech})
		return nil
	})
	return out
}

func isTest(name string) bool {
	return strings.Contains(name, ".test.") || strings.Contains(name, ".spec.") || strings.Contains(name, ".stories.") || strings.HasSuffix(name, ".d.ts")
}

func read(dir, rel string) string {
	b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		return ""
	}
	return string(b)
}

func stem(name string) string {
	name = filepath.Base(name)
	return strings.TrimSuffix(name, filepath.Ext(name))
}

var (
	// Router config: { path: "/orders" } (React Router, Vue Router, Angular).
	routeObj = regexp.MustCompile(`\bpath\s*:\s*["'\x60](/[^"'\x60]*)["'\x60]`)
	// <Route path="orders" />
	routeJSX   = regexp.MustCompile(`<Route\b[^>]*?\spath=["']([^"']+)["']`)
	routeGroup = regexp.MustCompile(`^\(.*\)$`)
)

// routes are the app's pages: file-based routing (Next.js, Nuxt,
// SvelteKit) or the paths in a router config.
func routes(dir string, files []file) []string {
	seen := map[string]bool{}
	var out []string
	add := func(r string) {
		r = "/" + strings.Trim(r, "/")
		if r == "/*" || strings.Contains(r, "*") || seen[r] {
			return
		}
		seen[r] = true
		out = append(out, r)
	}
	for _, f := range files {
		segs := strings.Split(f.sub, "/")
		name := segs[len(segs)-1]
		switch {
		case stem(name) == "page" && routerDir(segs, "app") >= 0: // Next.js app router
			add(routePath(segs[routerDir(segs, "app")+1 : len(segs)-1]))
			continue
		case name == "+page.svelte" && routerDir(segs, "routes") >= 0: // SvelteKit
			add(routePath(segs[routerDir(segs, "routes")+1 : len(segs)-1]))
			continue
		case (f.tech == "Next.js" || hasNuxt(dir, f)) && routerDir(segs, "pages") >= 0: // pages router
			rest := segs[routerDir(segs, "pages")+1:]
			if strings.HasPrefix(name, "_") || (len(rest) > 0 && rest[0] == "api") {
				continue
			}
			rest[len(rest)-1] = stem(name)
			if rest[len(rest)-1] == "index" {
				rest = rest[:len(rest)-1]
			}
			add(routePath(rest))
			continue
		}
		if !strings.Contains(strings.ToLower(f.sub), "rout") && !strings.HasPrefix(stem(name), "App") && stem(name) != "main" {
			continue // router configs live in router/routes files or the app root
		}
		src := read(dir, f.rel)
		for _, m := range routeObj.FindAllStringSubmatch(src, -1) {
			add(m[1])
		}
		for _, m := range routeJSX.FindAllStringSubmatch(src, -1) {
			add(m[1])
		}
	}
	sort.Strings(out)
	if len(out) > maxRoutes {
		out = out[:maxRoutes]
	}
	return out
}

// routerDir is the index of the routing folder (app, pages, routes) at the
// top of the component or under src/.
func routerDir(segs []string, name string) int {
	switch {
	case len(segs) > 1 && segs[0] == name:
		return 0
	case len(segs) > 2 && segs[0] == "src" && segs[1] == name:
		return 1
	}
	return -1
}

func routePath(segs []string) string {
	var keep []string
	for _, s := range segs {
		if !routeGroup.MatchString(s) && !strings.HasPrefix(s, "@") {
			keep = append(keep, s)
		}
	}
	return "/" + strings.Join(keep, "/")
}

func hasNuxt(dir string, f file) bool {
	root := strings.TrimSuffix(strings.TrimSuffix(f.rel, f.sub), "/")
	for _, n := range []string{"nuxt.config.ts", "nuxt.config.js"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(root), n)); err == nil {
			return true
		}
	}
	return false
}

var (
	sideWords = []string{"sidebar", "sidenav", "sidemenu", "drawer"}
	topWords  = []string{"navbar", "topbar", "topnav", "appbar", "header"}
	// notShell: headers of a part of a page, not of the app.
	notShell = []string{"page", "card", "table", "modal", "dialog", "section", "sheet", "column", "list"}
)

// shell are the layout and navigation files every page sits in, and the
// navigation they suggest. UI primitives (components/ui) are not shell.
func shell(files []file) ([]string, string) {
	var out []string
	side, top := false, false
	for _, f := range files {
		if isPrimitive(f.sub) {
			continue
		}
		n := strings.ToLower(stem(f.sub))
		n = strings.ReplaceAll(strings.ReplaceAll(n, "-", ""), "_", "")
		if containsAny(n, notShell) {
			continue
		}
		isSide := containsAny(n, sideWords)
		isTop := containsAny(n, topWords)
		if !isSide && !isTop && !strings.Contains(n, "layout") && n != "appshell" && n != "shell" && n != "navigation" && n != "nav" {
			continue
		}
		side, top = side || isSide, top || isTop
		if len(out) < maxShell {
			out = append(out, f.rel)
		}
	}
	nav := ""
	switch {
	case side:
		nav = "side"
	case top:
		nav = "top"
	}
	return out, nav
}

func containsAny(s string, words []string) bool {
	for _, w := range words {
		if strings.Contains(s, w) {
			return true
		}
	}
	return false
}

func isPrimitive(sub string) bool {
	return strings.Contains("/"+sub, "/components/ui/")
}

var (
	navLabel = regexp.MustCompile(`\b(?:label|title)\s*:\s*["'\x60]([^"'\x60{}<>]{2,40})["'\x60]`)
	navLink  = regexp.MustCompile(`<(?:Link|NavLink|RouterLink|router-link|a)\b[^>]*>\s*([^<>{}]{2,40}?)\s*</`)
)

// navItems are the menu labels in the navigation files.
func navItems(dir string, files []file) []string {
	var out []string
	seen := map[string]bool{}
	for _, f := range files {
		n := strings.ToLower(stem(f.sub))
		if isPrimitive(f.sub) || containsAny(n, notShell) || !(containsAny(n, sideWords) || containsAny(n, topWords) || strings.Contains(n, "nav") || strings.Contains(n, "menu")) {
			continue
		}
		src := read(dir, f.rel)
		for _, re := range []*regexp.Regexp{navLabel, navLink} {
			for _, m := range re.FindAllStringSubmatch(src, -1) {
				l := strings.TrimSpace(m[1])
				if l == "" || seen[l] || strings.ContainsAny(l, "/=;()") {
					continue
				}
				seen[l] = true
				if out = append(out, l); len(out) == maxNavItems {
					return out
				}
			}
		}
	}
	return out
}

// components are the reusable UI components: files in a components
// folder. Primitives and common/shared ones come first.
func components(files []file) []string {
	type comp struct {
		name, rel string
		rank      int
	}
	var all []comp
	seen := map[string]bool{}
	for _, f := range files {
		ext := filepath.Ext(f.sub)
		if ext == ".ts" || ext == ".js" || !strings.Contains("/"+f.sub, "/components/") {
			continue
		}
		name := pascal(stem(f.sub))
		if name == "" || name == "Index" || seen[name] {
			continue
		}
		seen[name] = true
		rank := 2
		switch {
		case isPrimitive(f.sub):
			rank = 0
		case containsAny(f.sub, []string{"/common/", "/shared/", "/base/"}):
			rank = 1
		}
		all = append(all, comp{name, f.rel, rank})
	}
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].rank != all[j].rank {
			return all[i].rank < all[j].rank
		}
		return all[i].name < all[j].name
	})
	var out []string
	for i, c := range all {
		if i == maxComponents {
			break
		}
		out = append(out, c.name+" ("+c.rel+")")
	}
	return out
}

// pascal turns "date-picker" or "data_table" into "DatePicker" / "DataTable".
func pascal(s string) string {
	var b strings.Builder
	up := true
	for _, r := range s {
		if r == '-' || r == '_' || r == '.' || r == ' ' {
			up = true
			continue
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return ""
		}
		if up {
			r = unicode.ToUpper(r)
			up = false
		}
		b.WriteRune(r)
	}
	return b.String()
}
