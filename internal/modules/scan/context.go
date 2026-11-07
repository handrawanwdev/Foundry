package scan

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
)

// Limits keep the context scan bounded on large repositories.
const (
	maxContextDepth = 4
	maxContextFile  = 256 << 10
	maxLayout       = 40
	maxModules      = 40
	maxEntities     = 40
	maxAPIs         = 40
	maxFindings     = 30
	maxDocBytes     = 8 << 10
	maxRouteFiles   = 25
	maxFields       = 16
	maxDeps         = 80
	maxDepFiles     = 80
	maxChanges      = 40
)

var moduleRoots = []string{"internal", "pkg", "modules", "src", "app", "lib", "libs", "packages", "services"}

var sourceExt = map[string]bool{
	".go": true, ".ts": true, ".tsx": true, ".js": true, ".jsx": true, ".py": true,
	".rs": true, ".java": true, ".kt": true, ".rb": true, ".php": true, ".cs": true,
	".vue": true, ".svelte": true,
}

var authName = regexp.MustCompile(`(?i)(^|/)(auth|jwt|oauth|oidc|session|rbac|casbin|passport|permission|authorize)s?([./_]|$)`)

var (
	rePrismaModel = regexp.MustCompile(`(?m)^model\s+(\w+)\s*\{`)
	rePrismaField = regexp.MustCompile(`(?m)^\s+(\w+)\s+(\w+)`)
	reCreateTable = regexp.MustCompile(`(?i)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?["'\[]?([A-Za-z_][\w]*)`)
	reProtoSvc    = regexp.MustCompile(`(?m)^service\s+(\w+)`)
	reRoute       = regexp.MustCompile(`(?i)\.(GET|POST|PUT|PATCH|DELETE|Get|Post|Put|Patch|Delete|get|post|put|patch|delete)\(\s*["'\x60](/[^"'\x60]*)`)
	reFastAPI     = regexp.MustCompile(`(?m)@(?:app|router)\.(get|post|put|patch|delete)\(\s*["'](/[^"']*)`)
	reOpenAPIPath = regexp.MustCompile(`(?m)^\s{2,}(/[-A-Za-z0-9_{}/]+):\s*$`)
	reHeading     = regexp.MustCompile(`(?m)^#\s+(.+)$`)
	reFeatureHead = regexp.MustCompile(`(?im)^##\s+(features?|fitur)\s*$`)
	reListItem    = regexp.MustCompile(`(?m)^[-*]\s+(.+)$`)
)

// Context builds an evidence-based picture of dir from the scan components
// and a bounded read of schemas, docs, routes and module folders.
// Nothing is invented from a folder name: a path is recorded as observed.
func Context(dir string, comps []core.Component) *core.ProjectContext {
	return Refresh(dir, nil, comps)
}

func layout(dir string) []core.ContextFinding {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []core.ContextFinding
	for _, e := range ents {
		name := e.Name()
		if skipDirs[name] || (strings.HasPrefix(name, ".") && name != ".github") {
			continue
		}
		path := name
		if e.IsDir() {
			path += "/"
		}
		out = append(out, core.ContextFinding{
			Kind: "layout", Text: path, Path: name,
			Evidence: "top-level", Confidence: core.ConfidenceObserved,
		})
		if len(out) >= maxLayout {
			break
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func fromComponents(comps []core.Component) (langs, frames, tests, conv []core.ContextFinding) {
	seenLang, seenFrame, seenTest, seenConv := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
	add := func(dst *[]core.ContextFinding, seen map[string]bool, kind, text, path, evidence string) {
		key := kind + "|" + text + "|" + path
		if text == "" || seen[key] {
			return
		}
		seen[key] = true
		*dst = append(*dst, core.ContextFinding{
			Kind: kind, Text: text, Path: path, Evidence: evidence, Confidence: core.ConfidenceObserved,
		})
	}
	for _, c := range comps {
		if c.Language != "" {
			add(&langs, seenLang, "language", c.Language, c.Path, c.Evidence)
		}
		switch c.Kind {
		case "backend", "web", "mobile", "desktop", "cli":
			if c.Tech != "" {
				add(&frames, seenFrame, "framework", c.Tech, c.Path, c.Evidence)
			}
		case KindTesting:
			text := c.Tech
			if text == "" {
				text = "tests"
			}
			add(&tests, seenTest, "test", text, c.Path, c.Evidence)
		case KindQuality:
			if c.Tech != "" {
				add(&conv, seenConv, "convention", c.Tech, c.Path, c.Evidence)
			}
		}
	}
	return langs, frames, tests, conv
}

func modules(dir string) []core.ContextModule {
	var out []core.ContextModule
	seen := map[string]bool{}
	add := func(name, path, evidence string) {
		path = filepath.ToSlash(path)
		if path == "" || path == "." || seen[path] || len(out) >= maxModules {
			return
		}
		seen[path] = true
		out = append(out, core.ContextModule{Name: name, Path: path, Evidence: evidence})
	}
	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		rel = filepath.ToSlash(rel)
		if rel != "." && (skipDirs[d.Name()] || strings.HasPrefix(d.Name(), ".")) {
			return filepath.SkipDir
		}
		if strings.Count(rel, "/") >= maxContextDepth {
			return filepath.SkipDir
		}
		if contains(moduleRoots, d.Name()) && rel != "." {
			for _, child := range childSourceDirs(path) {
				add(child.name, filepath.Join(rel, child.name), child.evidence)
			}
		}
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	if len(out) > maxModules {
		out = out[:maxModules]
	}
	return out
}

type srcChild struct{ name, evidence string }

func childSourceDirs(root string) []srcChild {
	ents, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var out []srcChild
	for _, e := range ents {
		if !e.IsDir() || skipDirs[e.Name()] || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if ev := dirSourceEvidence(filepath.Join(root, e.Name())); ev != "" {
			out = append(out, srcChild{e.Name(), ev})
		}
	}
	return out
}

func dirSourceEvidence(dir string) string {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	counts := map[string]int{}
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if sourceExt[ext] {
			counts[ext]++
		}
	}
	if len(counts) == 0 {
		return ""
	}
	var parts []string
	for ext, n := range counts {
		parts = append(parts, strings.TrimPrefix(ext, ".")+"x"+strconv.Itoa(n))
	}
	sort.Strings(parts)
	return "source files: " + strings.Join(parts, ", ")
}

func dataModel(dir string) []core.ContextEntity {
	var out []core.ContextEntity
	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != dir && (skipDirs[d.Name()] || strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			rel, _ := filepath.Rel(dir, path)
			if strings.Count(filepath.ToSlash(rel), "/") >= maxContextDepth {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		rel, _ := filepath.Rel(dir, path)
		rel = filepath.ToSlash(rel)
		switch {
		case name == "schema.prisma":
			out = append(out, prismaEntities(rel, readCap(path))...)
		case strings.HasSuffix(name, ".sql"):
			out = append(out, sqlEntities(rel, readCap(path))...)
		}
		if len(out) >= maxEntities {
			return filepath.SkipAll
		}
		return nil
	})
	if len(out) > maxEntities {
		out = out[:maxEntities]
	}
	return out
}

func prismaEntities(rel, body string) []core.ContextEntity {
	var out []core.ContextEntity
	idx := rePrismaModel.FindAllStringSubmatchIndex(body, maxEntities)
	for i, loc := range idx {
		name := body[loc[2]:loc[3]]
		end := len(body)
		if i+1 < len(idx) {
			end = idx[i+1][0]
		}
		block := body[loc[1]:end]
		var fields []string
		for _, m := range rePrismaField.FindAllStringSubmatch(block, maxFields+4) {
			if m[1] == "@@" || strings.HasPrefix(m[1], "//") {
				continue
			}
			fields = append(fields, m[1])
			if len(fields) >= maxFields {
				break
			}
		}
		out = append(out, core.ContextEntity{Name: name, Fields: fields, Path: rel, Evidence: "schema.prisma model"})
	}
	return out
}

func sqlEntities(rel, body string) []core.ContextEntity {
	var out []core.ContextEntity
	for _, m := range reCreateTable.FindAllStringSubmatch(body, maxEntities) {
		out = append(out, core.ContextEntity{Name: m[1], Path: rel, Evidence: "CREATE TABLE"})
	}
	return out
}

func apis(dir string) []core.ContextAPI {
	var out []core.ContextAPI
	var routeFiles int
	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != dir && (skipDirs[d.Name()] || strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			rel, _ := filepath.Rel(dir, path)
			if strings.Count(filepath.ToSlash(rel), "/") >= maxContextDepth {
				return filepath.SkipDir
			}
			return nil
		}
		name := strings.ToLower(d.Name())
		rel, _ := filepath.Rel(dir, path)
		rel = filepath.ToSlash(rel)
		switch {
		case name == "openapi.yaml" || name == "openapi.yml" || name == "openapi.json" ||
			name == "swagger.yaml" || name == "swagger.yml" || name == "swagger.json":
			body := readCap(path)
			out = append(out, core.ContextAPI{Kind: "openapi", Path: rel, Evidence: name})
			for _, m := range reOpenAPIPath.FindAllStringSubmatch(body, 20) {
				out = append(out, core.ContextAPI{Kind: "openapi", Path: rel, Detail: m[1], Evidence: name})
				if len(out) >= maxAPIs {
					return filepath.SkipAll
				}
			}
		case strings.HasSuffix(name, ".proto"):
			body := readCap(path)
			out = append(out, core.ContextAPI{Kind: "proto", Path: rel, Evidence: name})
			for _, m := range reProtoSvc.FindAllStringSubmatch(body, 10) {
				out = append(out, core.ContextAPI{Kind: "proto", Path: rel, Detail: "service " + m[1], Evidence: name})
			}
		case isRouteFile(name) && routeFiles < maxRouteFiles:
			routeFiles++
			body := readCap(path)
			found := 0
			for _, m := range reRoute.FindAllStringSubmatch(body, 12) {
				out = append(out, core.ContextAPI{Kind: "route", Path: rel, Detail: strings.ToUpper(m[1]) + " " + m[2], Evidence: name})
				found++
			}
			for _, m := range reFastAPI.FindAllStringSubmatch(body, 12-found) {
				out = append(out, core.ContextAPI{Kind: "route", Path: rel, Detail: strings.ToUpper(m[1]) + " " + m[2], Evidence: name})
			}
			if found == 0 && !strings.Contains(body, "Router") && !strings.Contains(body, "HandleFunc") {
				// File name looked like a router but no method+path was read.
			}
		}
		if len(out) >= maxAPIs {
			return filepath.SkipAll
		}
		return nil
	})
	if len(out) > maxAPIs {
		out = out[:maxAPIs]
	}
	return out
}

func isRouteFile(name string) bool {
	n := strings.ToLower(name)
	if !strings.HasSuffix(n, ".go") && !strings.HasSuffix(n, ".ts") && !strings.HasSuffix(n, ".js") &&
		!strings.HasSuffix(n, ".py") && !strings.HasSuffix(n, ".rb") {
		return false
	}
	stem := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(n, ".go"), ".ts"), ".js")
	stem = strings.TrimSuffix(strings.TrimSuffix(stem, ".py"), ".rb")
	return strings.Contains(stem, "route") || strings.Contains(stem, "router") ||
		strings.Contains(stem, "handler") || stem == "main" || stem == "app" ||
		strings.Contains(stem, "endpoint") || strings.Contains(stem, "controller")
}

func authFiles(dir string) []core.ContextFinding {
	var out []core.ContextFinding
	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if path != dir && (skipDirs[d.Name()] || strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			if strings.Count(rel, "/") >= maxContextDepth {
				return filepath.SkipDir
			}
			if authName.MatchString(rel+"/") && rel != "." {
				out = append(out, core.ContextFinding{
					Kind: "auth", Text: d.Name(), Path: rel,
					Evidence: "directory name", Confidence: core.ConfidenceObserved,
				})
			}
			return nil
		}
		if !authName.MatchString(rel) {
			return nil
		}
		out = append(out, core.ContextFinding{
			Kind: "auth", Text: d.Name(), Path: rel,
			Evidence: "file name", Confidence: core.ConfidenceObserved,
		})
		if len(out) >= maxFindings {
			return filepath.SkipAll
		}
		return nil
	})
	return out
}

func docsAndFeatures(dir string) (docs, features []core.ContextFinding) {
	addDoc := func(rel, body string) {
		title := rel
		if m := reHeading.FindStringSubmatch(body); m != nil {
			title = strings.TrimSpace(m[1])
		}
		docs = append(docs, core.ContextFinding{
			Kind: "doc", Text: title, Path: rel,
			Evidence: rel, Confidence: core.ConfidenceDocumented,
		})
		if loc := reFeatureHead.FindStringIndex(body); loc != nil {
			rest := body[loc[1]:]
			if next := regexp.MustCompile(`(?m)^##\s+`).FindStringIndex(rest); next != nil {
				rest = rest[:next[0]]
			}
			for _, m := range reListItem.FindAllStringSubmatch(rest, 12) {
				item := strings.TrimSpace(m[1])
				if item == "" {
					continue
				}
				features = append(features, core.ContextFinding{
					Kind: "feature", Text: item, Path: rel,
					Evidence: rel + " ## Features", Confidence: core.ConfidenceDocumented,
				})
			}
		}
	}
	for _, name := range []string{"README.md", "README", "CONTRIBUTING.md"} {
		body := readCapN(filepath.Join(dir, name), maxDocBytes)
		if body != "" {
			addDoc(name, body)
		}
	}
	docsDir := filepath.Join(dir, "docs")
	ents, err := os.ReadDir(docsDir)
	if err == nil {
		n := 0
		for _, e := range ents {
			if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
				continue
			}
			rel := "docs/" + e.Name()
			body := readCapN(filepath.Join(docsDir, e.Name()), maxDocBytes)
			if body != "" {
				addDoc(rel, body)
			}
			n++
			if n >= 8 {
				break
			}
		}
	}
	if len(docs) > maxFindings {
		docs = docs[:maxFindings]
	}
	if len(features) > maxFindings {
		features = features[:maxFindings]
	}
	return docs, features
}

func readCap(path string) string { return readCapN(path, maxContextFile) }

func readCapN(path string, n int) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	if len(b) > n {
		b = b[:n]
	}
	return string(b)
}

func contains(ss []string, v string) bool {
	for _, s := range ss {
		if s == v {
			return true
		}
	}
	return false
}
