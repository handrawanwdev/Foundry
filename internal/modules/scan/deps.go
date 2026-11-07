package scan

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
)

var (
	reGoModule    = regexp.MustCompile(`(?m)^module\s+(\S+)`)
	reGoImport    = regexp.MustCompile(`(?m)^\s*(?:[\w.]+\s+)?"([^"]+)"\s*$`)
	reGoImportOne = regexp.MustCompile(`(?m)^import\s+(?:[\w.]+\s+)?"([^"]+)"`)
	reJSFrom      = regexp.MustCompile(`(?m)(?:from|import|require)\s*\(?\s*['"]([^'"]+)['"]`)
	rePyImport    = regexp.MustCompile(`(?m)^(?:from\s+(\.*[\w.]*)\s+import|import\s+([\w.]+))`)
	rePkgJSONName = regexp.MustCompile(`"name"\s*:\s*"([^"]+)"`)
	reEventFile   = regexp.MustCompile(`(?i)(event|handler|subscriber|listener)`)
	reConfigFile  = regexp.MustCompile(`(?i)(config|settings|env)`)
)

// dependencies maps observed imports between modules, packages, APIs,
// entities, shared code, events and config. Confidence is always observed.
func dependencies(dir string, mods []core.ContextModule) []core.ContextDep {
	if dir == "" || len(mods) == 0 {
		return nil
	}
	index := moduleIndex(mods)
	pkgNames := packageNames(dir, mods)
	goPrefix := goModulePrefix(dir)
	var out []core.ContextDep
	seen := map[string]bool{}
	add := func(from, to, kind, path, evidence string) {
		from, to = strings.TrimSpace(from), strings.TrimSpace(to)
		if from == "" || to == "" || from == to || len(out) >= maxDeps {
			return
		}
		key := kind + "|" + from + "|" + to + "|" + path
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, core.ContextDep{
			From: from, To: to, Kind: kind, Path: path,
			Evidence: evidence, Confidence: core.ConfidenceObserved,
		})
	}
	files := 0
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
			return nil
		}
		ext := strings.ToLower(filepath.Ext(d.Name()))
		if !sourceExt[ext] {
			return nil
		}
		from := ownerOf(rel, mods)
		if from == "" {
			return nil
		}
		if files >= maxDepFiles {
			return filepath.SkipAll
		}
		files++
		body := readCap(path)
		var specs []string
		switch ext {
		case ".go":
			specs = goImports(body)
		case ".ts", ".tsx", ".js", ".jsx":
			specs = jsImports(body)
		case ".py":
			specs = pyImports(body)
		}
		for _, spec := range specs {
			to, kind := resolveSpec(rel, spec, index, pkgNames, goPrefix)
			if to == "" {
				continue
			}
			add(from, to, kind, rel, spec)
			if reEventFile.MatchString(spec) || reEventFile.MatchString(to) {
				add(from, to, core.DepKindEvent, rel, spec)
			}
			if reConfigFile.MatchString(spec) || reConfigFile.MatchString(to) {
				add(from, to, core.DepKindConfig, rel, spec)
			}
		}
		return nil
	})
	for _, e := range dataModel(dir) {
		if owner := ownerOf(e.Path, mods); owner != "" {
			add(owner, e.Name, core.DepKindEntity, e.Path, e.Evidence)
		}
	}
	for _, a := range apis(dir) {
		if owner := ownerOf(a.Path, mods); owner != "" {
			detail := a.Detail
			if detail == "" {
				detail = a.Kind
			}
			add(detail, owner, core.DepKindAPI, a.Path, a.Evidence)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].From != out[j].From {
			return out[i].From < out[j].From
		}
		if out[i].To != out[j].To {
			return out[i].To < out[j].To
		}
		return out[i].Kind < out[j].Kind
	})
	if len(out) > maxDeps {
		out = out[:maxDeps]
	}
	return out
}

type modRef struct {
	mod  core.ContextModule
	path string
}

func moduleIndex(mods []core.ContextModule) []modRef {
	var out []modRef
	for _, m := range mods {
		out = append(out, modRef{m, strings.TrimSuffix(m.Path, "/")})
	}
	sort.Slice(out, func(i, j int) bool { return len(out[i].path) > len(out[j].path) })
	return out
}

func ownerOf(rel string, mods []core.ContextModule) string {
	rel = filepath.ToSlash(rel)
	best := ""
	bestLen := -1
	for _, m := range mods {
		p := strings.TrimSuffix(m.Path, "/")
		if p != "" && (rel == p || strings.HasPrefix(rel, p+"/")) && len(p) > bestLen {
			best, bestLen = m.Path, len(p)
		}
	}
	return best
}

func goModulePrefix(dir string) string {
	var found string
	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || found != "" {
			return nil
		}
		if d.IsDir() {
			if path != dir && (skipDirs[d.Name()] || strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			rel, _ := filepath.Rel(dir, path)
			if strings.Count(filepath.ToSlash(rel), "/") >= 2 {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() != "go.mod" {
			return nil
		}
		if m := reGoModule.FindStringSubmatch(readCapN(path, 4<<10)); len(m) > 1 {
			found = m[1]
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

func packageNames(dir string, mods []core.ContextModule) map[string]string {
	out := map[string]string{}
	for _, m := range mods {
		body := readCapN(filepath.Join(dir, m.Path, "package.json"), 8<<10)
		if body == "" {
			continue
		}
		if n := rePkgJSONName.FindStringSubmatch(body); len(n) > 1 {
			out[n[1]] = m.Path
		}
	}
	return out
}

func goImports(body string) []string {
	var out []string
	if i := strings.Index(body, "import ("); i >= 0 {
		rest := body[i:]
		if j := strings.Index(rest, ")"); j > 0 {
			for _, m := range reGoImport.FindAllStringSubmatch(rest[:j], 40) {
				out = append(out, m[1])
			}
		}
	}
	for _, m := range reGoImportOne.FindAllStringSubmatch(body, 20) {
		out = append(out, m[1])
	}
	return out
}

func jsImports(body string) []string {
	var out []string
	for _, m := range reJSFrom.FindAllStringSubmatch(body, 40) {
		out = append(out, m[1])
	}
	return out
}

func pyImports(body string) []string {
	var out []string
	for _, m := range rePyImport.FindAllStringSubmatch(body, 40) {
		spec := m[1]
		if spec == "" {
			spec = m[2]
		}
		if spec != "" {
			out = append(out, spec)
		}
	}
	return out
}

func resolveSpec(fromFile, spec string, index []modRef, pkgNames map[string]string, goPrefix string) (to, kind string) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return "", ""
	}
	if goPrefix != "" && (spec == goPrefix || strings.HasPrefix(spec, goPrefix+"/")) {
		rel := strings.TrimPrefix(spec, goPrefix)
		rel = strings.TrimPrefix(rel, "/")
		if hit := matchPath(rel, index); hit != "" {
			return hit, core.DepKindModule
		}
	}
	if pkg, ok := pkgNames[spec]; ok {
		kind = core.DepKindPackage
		if isShared(pkg) {
			kind = core.DepKindShared
		}
		return pkg, kind
	}
	if strings.HasPrefix(spec, ".") || strings.HasPrefix(spec, "/") {
		base := filepath.ToSlash(filepath.Dir(fromFile))
		rel := spec
		if strings.HasPrefix(spec, ".") && !strings.Contains(spec, "/") {
			dots := 0
			for _, c := range spec {
				if c == '.' {
					dots++
					continue
				}
				break
			}
			rel = strings.ReplaceAll(spec[dots:], ".", "/")
			for i := 1; i < dots; i++ {
				base = filepath.ToSlash(filepath.Dir(base))
			}
		}
		resolved := filepath.ToSlash(filepath.Clean(filepath.Join(base, rel)))
		if hit := matchPath(resolved, index); hit != "" {
			kind = core.DepKindModule
			if isShared(hit) {
				kind = core.DepKindShared
			}
			return hit, kind
		}
	}
	if strings.HasPrefix(spec, "@/") || strings.HasPrefix(spec, "~/") {
		rest := spec[2:]
		for _, base := range []string{"src", "app", "apps/web", "."} {
			resolved := filepath.ToSlash(filepath.Clean(filepath.Join(base, rest)))
			if hit := matchPath(resolved, index); hit != "" {
				return hit, core.DepKindModule
			}
		}
	}
	if strings.Contains(spec, ".") && !strings.Contains(spec, "/") {
		py := strings.ReplaceAll(spec, ".", "/")
		if hit := matchPath(py, index); hit != "" {
			return hit, core.DepKindModule
		}
	}
	if hit := matchPath(spec, index); hit != "" {
		kind = core.DepKindModule
		if isShared(hit) {
			kind = core.DepKindShared
		}
		return hit, kind
	}
	return "", ""
}

func matchPath(rel string, index []modRef) string {
	rel = strings.TrimPrefix(filepath.ToSlash(rel), "/")
	rel = strings.TrimSuffix(rel, "/")
	for _, it := range index {
		if rel == it.path || strings.HasPrefix(rel, it.path+"/") {
			return it.mod.Path
		}
	}
	return ""
}

func isShared(path string) bool {
	p := "/" + filepath.ToSlash(path) + "/"
	return strings.Contains(p, "/packages/") || strings.Contains(p, "/libs/") ||
		strings.Contains(p, "/shared/") || strings.Contains(p, "/lib/")
}
