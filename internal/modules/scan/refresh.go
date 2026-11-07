package scan

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/handrawanwdev/foundry/internal/core"
)

var contextAreas = []string{
	core.ContextAreaLayout, core.ContextAreaLang, core.ContextAreaModules,
	core.ContextAreaData, core.ContextAreaAPIs, core.ContextAreaAuth,
	core.ContextAreaTests, core.ContextAreaDocs, core.ContextAreaDeps,
}

var manifestNames = map[string]bool{
	"go.mod": true, "go.work": true, "package.json": true, "pnpm-workspace.yaml": true,
	"turbo.json": true, "nx.json": true, "lerna.json": true, "pyproject.toml": true,
	"requirements.txt": true, "Pipfile": true, "Cargo.toml": true, "pom.xml": true,
	"build.gradle": true, "build.gradle.kts": true, "composer.json": true, "Gemfile": true,
	"pubspec.yaml": true, "tsconfig.json": true, "Makefile": true, "Dockerfile": true,
	"docker-compose.yml": true, "docker-compose.yaml": true, "compose.yaml": true,
	"compose.yml": true, ".golangci.yml": true, ".golangci.yaml": true,
}

// Refresh rebuilds context from dir. Unchanged areas are copied from prev.
// Corrections are kept. Stale is cleared because this is a fresh reading.
func Refresh(dir string, prev *core.ProjectContext, comps []core.Component) *core.ProjectContext {
	if dir == "" {
		return nil
	}
	hashes := areaHashes(dir)
	fp := fingerprintOf(hashes)
	now := time.Now().UTC()
	if prev != nil && prev.Fingerprint != "" && prev.Fingerprint == fp && !prev.Empty() {
		out := cloneContext(prev)
		out.Stale = false
		out.StaleAreas = nil
		out.Changes = nil
		out.ScannedAt = now
		out.Fingerprint = fp
		out.AreaHash = hashes
		return out
	}
	c := &core.ProjectContext{AreaHash: hashes, Fingerprint: fp, ScannedAt: now}
	if prev != nil {
		c.Corrections = prev.Corrections
	}
	changed := map[string]bool{}
	if prev == nil || prev.AreaHash == nil {
		fillAll(c, dir, comps)
	} else {
		for _, area := range contextAreas {
			if prev.AreaHash[area] != hashes[area] {
				changed[area] = true
			}
		}
		copyUnchanged(c, prev, changed)
		fillChanged(c, dir, comps, changed)
	}
	attachDeps(c, dir, prev, changed)
	if prev != nil {
		c.Changes = diffContext(prev, c)
	}
	if c.Empty() && strings.TrimSpace(c.Corrections) == "" {
		return nil
	}
	return c
}

// MarkStale sets Stale when the repository no longer matches the stored
// fingerprint. Findings are left as they were; they are not current fact.
func MarkStale(dir string, c *core.ProjectContext) {
	if c == nil || c.Fingerprint == "" {
		return
	}
	hashes := areaHashes(dir)
	if fingerprintOf(hashes) == c.Fingerprint {
		c.Stale = false
		c.StaleAreas = nil
		return
	}
	c.Stale = true
	var stale []string
	for _, area := range contextAreas {
		if c.AreaHash[area] != hashes[area] {
			stale = append(stale, area)
		}
	}
	c.StaleAreas = stale
}

func fillAll(c *core.ProjectContext, dir string, comps []core.Component) {
	c.Layout = layout(dir)
	c.Languages, c.Frameworks, c.Tests, c.Conventions = fromComponents(comps)
	c.Modules = modules(dir)
	c.Data = dataModel(dir)
	c.APIs = apis(dir)
	c.Auth = authFiles(dir)
	c.Docs, c.Features = docsAndFeatures(dir)
}

func fillChanged(c *core.ProjectContext, dir string, comps []core.Component, changed map[string]bool) {
	if changed[core.ContextAreaLayout] {
		c.Layout = layout(dir)
	}
	if changed[core.ContextAreaLang] || changed[core.ContextAreaTests] {
		lang, frames, tests, conv := fromComponents(comps)
		if changed[core.ContextAreaLang] {
			c.Languages, c.Frameworks, c.Conventions = lang, frames, conv
		}
		if changed[core.ContextAreaTests] {
			c.Tests = tests
		}
	}
	if changed[core.ContextAreaModules] {
		c.Modules = modules(dir)
	}
	if changed[core.ContextAreaData] {
		c.Data = dataModel(dir)
	}
	if changed[core.ContextAreaAPIs] {
		c.APIs = apis(dir)
	}
	if changed[core.ContextAreaAuth] {
		c.Auth = authFiles(dir)
	}
	if changed[core.ContextAreaDocs] {
		c.Docs, c.Features = docsAndFeatures(dir)
	}
}

func copyUnchanged(c, prev *core.ProjectContext, changed map[string]bool) {
	if !changed[core.ContextAreaLayout] {
		c.Layout = prev.Layout
	}
	if !changed[core.ContextAreaLang] {
		c.Languages, c.Frameworks, c.Conventions = prev.Languages, prev.Frameworks, prev.Conventions
	}
	if !changed[core.ContextAreaTests] {
		c.Tests = prev.Tests
	}
	if !changed[core.ContextAreaModules] {
		c.Modules = prev.Modules
	}
	if !changed[core.ContextAreaData] {
		c.Data = prev.Data
	}
	if !changed[core.ContextAreaAPIs] {
		c.APIs = prev.APIs
	}
	if !changed[core.ContextAreaAuth] {
		c.Auth = prev.Auth
	}
	if !changed[core.ContextAreaDocs] {
		c.Docs, c.Features = prev.Docs, prev.Features
	}
}

func attachDeps(c *core.ProjectContext, dir string, prev *core.ProjectContext, changed map[string]bool) {
	if prev != nil && !changed[core.ContextAreaModules] && !changed[core.ContextAreaDeps] && !changed[core.ContextAreaLang] && len(prev.Deps) > 0 {
		c.Deps = prev.Deps
		return
	}
	c.Deps = dependencies(dir, c.Modules)
	byMod := map[string][]string{}
	for _, d := range c.Deps {
		if d.Kind != core.DepKindModule && d.Kind != core.DepKindPackage && d.Kind != core.DepKindShared {
			continue
		}
		if !contains(byMod[d.From], d.To) {
			byMod[d.From] = append(byMod[d.From], d.To)
		}
	}
	for i, m := range c.Modules {
		c.Modules[i].Depends = byMod[m.Path]
		if len(c.Modules[i].Depends) == 0 {
			c.Modules[i].Depends = byMod[m.Name]
		}
	}
}

func cloneContext(c *core.ProjectContext) *core.ProjectContext {
	if c == nil {
		return nil
	}
	out := *c
	out.Layout = slices.Clone(c.Layout)
	out.Languages = slices.Clone(c.Languages)
	out.Frameworks = slices.Clone(c.Frameworks)
	out.Modules = slices.Clone(c.Modules)
	out.Data = slices.Clone(c.Data)
	out.APIs = slices.Clone(c.APIs)
	out.Auth = slices.Clone(c.Auth)
	out.Tests = slices.Clone(c.Tests)
	out.Docs = slices.Clone(c.Docs)
	out.Conventions = slices.Clone(c.Conventions)
	out.Features = slices.Clone(c.Features)
	out.Deps = slices.Clone(c.Deps)
	out.Changes = slices.Clone(c.Changes)
	out.StaleAreas = slices.Clone(c.StaleAreas)
	if c.AreaHash != nil {
		out.AreaHash = make(map[string]string, len(c.AreaHash))
		for k, v := range c.AreaHash {
			out.AreaHash[k] = v
		}
	}
	return &out
}

func areaHashes(dir string) map[string]string {
	acc := map[string]*strings.Builder{}
	for _, a := range contextAreas {
		acc[a] = &strings.Builder{}
	}
	ents, err := os.ReadDir(dir)
	if err == nil {
		var names []string
		for _, e := range ents {
			name := e.Name()
			if skipDirs[name] || (strings.HasPrefix(name, ".") && name != ".github") {
				continue
			}
			names = append(names, name)
		}
		sort.Strings(names)
		for _, n := range names {
			acc[core.ContextAreaLayout].WriteString(n)
			acc[core.ContextAreaLayout].WriteByte(0)
		}
	}
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
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
		name := d.Name()
		info, err := d.Info()
		if err != nil {
			return nil
		}
		meta := rel + "|" + strconv.FormatInt(info.Size(), 10) + "|" + strconv.FormatInt(info.ModTime().UnixNano(), 10) + "\n"
		lname := strings.ToLower(name)
		ext := strings.ToLower(filepath.Ext(name))
		if manifestNames[name] || strings.HasSuffix(lname, ".csproj") {
			acc[core.ContextAreaLang].WriteString(meta)
		}
		if name == "schema.prisma" || ext == ".sql" {
			acc[core.ContextAreaData].WriteString(meta)
		}
		if lname == "openapi.yaml" || lname == "openapi.yml" || lname == "openapi.json" ||
			lname == "swagger.yaml" || lname == "swagger.yml" || lname == "swagger.json" ||
			ext == ".proto" || isRouteFile(name) {
			acc[core.ContextAreaAPIs].WriteString(meta)
		}
		if authName.MatchString(rel) {
			acc[core.ContextAreaAuth].WriteString(meta)
		}
		if strings.HasSuffix(name, "_test.go") || strings.Contains(lname, ".test.") || strings.Contains(lname, ".spec.") {
			acc[core.ContextAreaTests].WriteString(meta)
		}
		if lname == "readme.md" || lname == "readme" || lname == "contributing.md" ||
			strings.HasPrefix(rel, "docs/") && ext == ".md" {
			acc[core.ContextAreaDocs].WriteString(meta)
		}
		if sourceExt[ext] {
			acc[core.ContextAreaModules].WriteString(meta)
			acc[core.ContextAreaDeps].WriteString(meta)
		}
		return nil
	})
	out := map[string]string{}
	for _, a := range contextAreas {
		sum := sha256.Sum256([]byte(acc[a].String()))
		out[a] = hex.EncodeToString(sum[:8])
	}
	return out
}

func fingerprintOf(hashes map[string]string) string {
	h := sha256.New()
	for _, a := range contextAreas {
		h.Write([]byte(a))
		h.Write([]byte{0})
		h.Write([]byte(hashes[a]))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)[:8])
}

func diffContext(prev, cur *core.ProjectContext) []core.ContextChange {
	if prev == nil || cur == nil {
		return nil
	}
	var out []core.ContextChange
	add := func(area, path, action string) {
		if len(out) >= maxChanges || path == "" {
			return
		}
		out = append(out, core.ContextChange{Area: area, Path: path, Action: action})
	}
	prevMod, curMod := map[string]string{}, map[string]string{}
	for _, m := range prev.Modules {
		prevMod[m.Path] = m.Evidence
	}
	for _, m := range cur.Modules {
		curMod[m.Path] = m.Evidence
	}
	for p, ev := range curMod {
		if old, ok := prevMod[p]; !ok {
			add(core.ContextAreaModules, p, core.ChangeAdd)
		} else if old != ev {
			add(core.ContextAreaModules, p, core.ChangeModify)
		}
	}
	for p := range prevMod {
		if _, ok := curMod[p]; !ok {
			add(core.ContextAreaModules, p, core.ChangeRemove)
		}
	}
	prevEnt, curEnt := map[string]bool{}, map[string]bool{}
	for _, e := range prev.Data {
		prevEnt[e.Name+"|"+e.Path] = true
	}
	for _, e := range cur.Data {
		curEnt[e.Name+"|"+e.Path] = true
		if !prevEnt[e.Name+"|"+e.Path] {
			add(core.ContextAreaData, e.Name, core.ChangeAdd)
		}
	}
	for k := range prevEnt {
		if !curEnt[k] {
			name, _, _ := strings.Cut(k, "|")
			add(core.ContextAreaData, name, core.ChangeRemove)
		}
	}
	prevAPI, curAPI := map[string]bool{}, map[string]bool{}
	for _, a := range prev.APIs {
		prevAPI[a.Path+"|"+a.Detail] = true
	}
	for _, a := range cur.APIs {
		curAPI[a.Path+"|"+a.Detail] = true
		if !prevAPI[a.Path+"|"+a.Detail] {
			add(core.ContextAreaAPIs, or(a.Detail, a.Path), core.ChangeAdd)
		}
	}
	for k := range prevAPI {
		if !curAPI[k] {
			_, detail, _ := strings.Cut(k, "|")
			add(core.ContextAreaAPIs, or(detail, k), core.ChangeRemove)
		}
	}
	return out
}

func or(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}
