// Package repoplan validates, normalizes and revises the repository plan the
// AI designs (core.RepoPlan). The AI decides the organization; this package
// only cleans what it wrote and checks it against the rest of the blueprint:
// project modules, the Structure IR, dependency rules and source changes.
package repoplan

import (
	"slices"
	"sort"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/structure"
)

const (
	maxNodes   = 800
	maxText    = 700
	maxList    = 14
	maxRules   = 60
	maxDeps    = 200
	maxDepth   = 14
	maxPathLen = 200
)

// ModuleRef is a project module the repository must place.
type ModuleRef struct {
	ID             string
	Responsibility string
}

// Modules are the project modules: the blueprint modules, else the domains
// found in the features.
func Modules(p *core.Project) []ModuleRef {
	var out []ModuleRef
	if bp := p.Blueprint; bp != nil {
		for _, m := range bp.Modules {
			id := strings.ToLower(strings.TrimSpace(m.ID))
			if id != "" && !slices.ContainsFunc(out, func(x ModuleRef) bool { return x.ID == id }) {
				out = append(out, ModuleRef{ID: id, Responsibility: strings.TrimSpace(m.Responsibility)})
			}
		}
	}
	if len(out) > 0 {
		return out
	}
	for _, d := range structure.Domains(p) {
		out = append(out, ModuleRef{ID: d})
	}
	return out
}

func moduleIDs(p *core.Project) []string {
	var out []string
	for _, m := range Modules(p) {
		out = append(out, m.ID)
	}
	return out
}

// IRPart is a part of the Structure IR with the folder the structure design chose for
// it. The plan may place the part elsewhere; Ref is what a node's Part names.
type IRPart struct {
	Ref  string
	Path string
	Kind string
}

// IRParts lists every part of the structure decision: apps, services,
// modules, workers and packages.
func IRParts(ir *core.StructureIR) []IRPart {
	if ir == nil {
		return nil
	}
	var out []IRPart
	add := func(kind string, ps []core.StructurePart) {
		for _, p := range ps {
			out = append(out, IRPart{Ref: kind + ":" + p.Name, Path: p.Path, Kind: kind})
			if kind == core.PartService {
				for _, m := range p.Modules {
					out = append(out, IRPart{Ref: core.PartModule + ":" + m, Path: structure.ModulePath(p, m), Kind: core.PartModule})
				}
			}
		}
	}
	add(core.PartApp, ir.Apps)
	add(core.PartService, ir.Services)
	add(core.PartWorker, ir.Workers)
	add(core.PartPackage, ir.Packages)
	return out
}

func clean(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > maxText {
		s = strings.TrimSpace(string(r[:maxText]))
	}
	return s
}

func cleanList(in []string, max int) []string {
	var out []string
	for _, v := range in {
		v = clean(v)
		if v == "" || slices.ContainsFunc(out, func(x string) bool { return strings.EqualFold(x, v) }) {
			continue
		}
		if out = append(out, v); len(out) == max {
			break
		}
	}
	return out
}

// CleanPath turns what the AI wrote into a repository-relative path: slash
// separated, no "." or ".." segments, a folder ends with "/". It returns ""
// for a path that cannot be a repository path.
func CleanPath(raw string, dir bool) string {
	raw = strings.ReplaceAll(strings.TrimSpace(raw), "\\", "/")
	var parts []string
	for _, seg := range strings.Split(raw, "/") {
		seg = strings.TrimSpace(seg)
		switch {
		case seg == "" || seg == ".":
			continue
		case seg == ".." || strings.ContainsAny(seg, ":*?\"<>|"):
			return ""
		}
		parts = append(parts, seg)
	}
	if len(parts) == 0 || len(parts) > maxDepth {
		return ""
	}
	out := strings.Join(parts, "/")
	if len(out) > maxPathLen {
		return ""
	}
	if dir {
		out += "/"
	}
	return out
}

func parentPath(p string) string {
	p = strings.TrimSuffix(p, "/")
	i := strings.LastIndex(p, "/")
	if i < 0 {
		return ""
	}
	return p[:i+1]
}

func looksLikeFile(raw string) bool {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	if raw == "" || strings.HasSuffix(raw, "/") {
		return false
	}
	base := raw
	if i := strings.LastIndex(raw, "/"); i >= 0 {
		base = raw[i+1:]
	}
	dot := strings.LastIndex(base, ".")
	return dot >= 0 && dot < len(base)-1
}

// matchNode is the node path raw names, whether the AI wrote a folder slash.
func matchNode(raw string, have map[string]bool) string {
	for _, dir := range []bool{true, false} {
		if p := CleanPath(raw, dir); p != "" && have[p] {
			return p
		}
	}
	return ""
}

// cleanLocation turns a module location into a folder that exists in the
// tree. The AI often lists files (schema.ts); the module lives in the
// folder that contains them.
func cleanLocation(raw string, have map[string]bool) string {
	if p := matchNode(raw, have); p != "" {
		if strings.HasSuffix(p, "/") {
			return p
		}
		if parent := parentPath(p); parent != "" {
			return parent
		}
		return p
	}
	asFile := CleanPath(raw, false)
	if looksLikeFile(raw) && asFile != "" {
		if parent := parentPath(asFile); parent != "" {
			return parent
		}
		return asFile
	}
	return CleanPath(raw, true)
}

// assignParts puts each structure-part stamp on exactly one folder: a part
// on a file is lifted to the parent folder, and when several folders share
// a part the one whose owner matches the part name is kept (module:identity
// → owner identity), else the first in tree order.
func assignParts(nodes []core.RepoNode) {
	byPath := map[string]int{}
	for i, n := range nodes {
		byPath[n.Path] = i
	}
	for i, n := range nodes {
		if n.IsDir() || n.Part == "" {
			continue
		}
		if j, ok := byPath[n.Parent()]; ok && nodes[j].Part == "" {
			nodes[j].Part = n.Part
		}
		nodes[i].Part = ""
	}
	keep := map[string]int{}
	for i, n := range nodes {
		if n.Part == "" || !n.IsDir() {
			continue
		}
		if prev, dup := keep[n.Part]; dup {
			winner, loser := prev, i
			_, name, _ := strings.Cut(n.Part, ":")
			if nodes[loser].Owner == name && nodes[winner].Owner != name {
				winner, loser = loser, prev
				keep[n.Part] = winner
			}
			nodes[loser].Part = ""
			continue
		}
		keep[n.Part] = i
	}
}

// guessType decides a node's type when the AI left it out or contradicted
// its own path.
func guessType(n core.RepoNode) string {
	switch strings.ToLower(strings.TrimSpace(n.Type)) {
	case core.NodeDir, "folder", "directory":
		return core.NodeDir
	case core.NodeFile:
		return core.NodeFile
	}
	if strings.HasSuffix(strings.TrimSpace(n.Path), "/") {
		return core.NodeDir
	}
	return core.NodeFile
}

// normalizeNode cleans one node; ok is false when its path is unusable.
func normalizeNode(n core.RepoNode) (core.RepoNode, bool) {
	n.Type = guessType(n)
	n.Path = CleanPath(n.Path, n.IsDir())
	if n.Path == "" {
		return n, false
	}
	n.Name = n.BaseName()
	n.Role = clean(n.Role)
	n.Responsibility = clean(n.Responsibility)
	n.Reason = clean(n.Reason)
	n.Owner = strings.ToLower(strings.TrimSpace(n.Owner))
	n.Part = strings.ToLower(strings.TrimSpace(n.Part))
	n.Relations = clean(n.Relations)
	n.Boundary = clean(n.Boundary)
	n.Allowed = cleanList(n.Allowed, maxList)
	n.Forbidden = cleanList(n.Forbidden, maxList)
	n.DependsOn = cleanEndpoints(n.DependsOn, maxList)
	n.SharedBy = cleanIDs(n.SharedBy)
	if !n.IsDir() {
		n.Allowed, n.Forbidden = nil, nil
	}
	return n, true
}

func cleanIDs(in []string) []string {
	var out []string
	for _, v := range in {
		v = strings.ToLower(strings.TrimSpace(v))
		if v != "" && !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	return out
}

// cleanEndpoints keeps module ids and normalizes paths; a value with a
// slash or a dot is a path.
func cleanEndpoints(in []string, max int) []string {
	var out []string
	for _, v := range in {
		if v = Endpoint(v); v != "" && !slices.Contains(out, v) {
			if out = append(out, v); len(out) == max {
				break
			}
		}
	}
	return out
}

// Endpoint normalizes a dependency endpoint: a module id/shared/platform
// stays a lowercase word, a path is cleaned (folders keep their slash).
func Endpoint(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if !strings.ContainsAny(v, "/\\.") {
		return strings.ToLower(v)
	}
	dir := strings.HasSuffix(v, "/") || strings.HasSuffix(v, "\\")
	return CleanPath(v, dir)
}

func normalizeRules(in []core.RepoDepRule, max int) []core.RepoDepRule {
	var out []core.RepoDepRule
	for _, r := range in {
		r.From, r.To = Endpoint(r.From), Endpoint(r.To)
		if r.From == "" || r.To == "" {
			continue
		}
		r.Via, r.Reason = clean(r.Via), clean(r.Reason)
		r.Mode = strings.ToLower(strings.TrimSpace(r.Mode))
		if r.Mode != core.DepEvent {
			r.Mode = core.DepSync
		}
		if slices.ContainsFunc(out, func(x core.RepoDepRule) bool { return x.From == r.From && x.To == r.To }) {
			continue
		}
		if out = append(out, r); len(out) == max {
			break
		}
	}
	return out
}

// Normalize cleans every text, path and id of the plan, derives names,
// adds the parent folders the AI left out (marked Implied) and sorts the
// nodes into tree order. It never judges the plan; Refresh does.
func Normalize(plan *core.RepoPlan) {
	plan.Approach, plan.Topology = clean(plan.Approach), clean(plan.Topology)
	plan.Summary, plan.Rationale = clean(plan.Summary), clean(plan.Rationale)
	plan.Rules = cleanList(plan.Rules, maxRules)

	byPath := map[string]int{}
	var nodes []core.RepoNode
	for _, n := range plan.Nodes {
		n, ok := normalizeNode(n)
		if !ok {
			continue
		}
		if i, dup := byPath[n.Path]; dup {
			nodes[i] = n // the last description of a path wins
			continue
		}
		if len(nodes) >= maxNodes {
			break
		}
		byPath[n.Path] = len(nodes)
		nodes = append(nodes, n)
	}
	// A path cannot be a folder and a file at once: the folder wins.
	for _, n := range slices.Clone(nodes) {
		if n.IsDir() {
			continue
		}
		if _, clash := byPath[n.Path+"/"]; clash {
			nodes = slices.DeleteFunc(nodes, func(x core.RepoNode) bool { return x.Path == n.Path })
		}
	}
	have := map[string]bool{}
	for _, n := range nodes {
		have[n.Path] = true
	}
	for _, n := range slices.Clone(nodes) {
		for parent := n.Parent(); parent != "" && !have[parent]; {
			have[parent] = true
			imp := core.RepoNode{Path: parent, Type: core.NodeDir, Implied: true, Owner: core.OwnerPlatform,
				Responsibility: "Folder induk " + strings.TrimSuffix(n.Path, "/") + "; belum dijelaskan.", Reason: "Dibutuhkan agar " + n.Path + " punya lokasi."}
			imp.Name = imp.BaseName()
			nodes = append(nodes, imp)
			parent = imp.Parent()
		}
	}
	sort.SliceStable(nodes, func(i, j int) bool { return nodes[i].Path < nodes[j].Path })
	assignParts(nodes)
	plan.Nodes = nodes

	have = map[string]bool{}
	for _, n := range plan.Nodes {
		have[n.Path] = true
	}

	var mods []core.RepoModuleMap
	for _, m := range plan.Modules {
		m.Module = strings.ToLower(strings.TrimSpace(m.Module))
		if m.Module == "" || slices.ContainsFunc(mods, func(x core.RepoModuleMap) bool { return x.Module == m.Module }) {
			continue
		}
		var locs []core.RepoLocation
		for _, l := range m.Locations {
			l.Path = cleanLocation(l.Path, have)
			l.Role = clean(l.Role)
			if l.Path != "" && !slices.ContainsFunc(locs, func(x core.RepoLocation) bool { return x.Path == l.Path }) {
				locs = append(locs, l)
			}
		}
		m.Locations, m.Note = locs, clean(m.Note)
		mods = append(mods, m)
	}
	plan.Modules = mods

	d := &plan.Dependency
	d.Direction, d.CycleGuard = clean(d.Direction), clean(d.CycleGuard)
	d.Allowed = normalizeRules(d.Allowed, maxDeps)
	d.Forbidden = normalizeRules(d.Forbidden, maxDeps)
	d.Communication = normalizeRules(d.Communication, maxDeps)
	owners := map[string]bool{core.OwnerShared: true, core.OwnerPlatform: true}
	for _, m := range plan.Modules {
		owners[m.Module] = true
	}
	knownEP := func(e string) bool {
		if e == "" {
			return true
		}
		if !strings.ContainsAny(e, "/.") && owners[e] {
			return true
		}
		return have[e] || have[e+"/"]
	}
	keepRules := func(rules []core.RepoDepRule) []core.RepoDepRule {
		var out []core.RepoDepRule
		for _, r := range rules {
			if knownEP(r.From) && knownEP(r.To) {
				out = append(out, r)
			}
		}
		return out
	}
	d.Allowed, d.Forbidden, d.Communication = keepRules(d.Allowed), keepRules(d.Forbidden), keepRules(d.Communication)
	d.SharedRules = cleanList(d.SharedRules, maxRules)
	var ifs []core.RepoInterface
	for _, i := range d.Interfaces {
		i.Module, i.Path = strings.ToLower(strings.TrimSpace(i.Module)), Endpoint(i.Path)
		if p := matchNode(i.Path, have); p != "" {
			i.Path = p
		}
		i.Reason = clean(i.Reason)
		if i.Module != "" && i.Path != "" && !slices.ContainsFunc(ifs, func(x core.RepoInterface) bool { return x.Module == i.Module && x.Path == i.Path }) {
			ifs = append(ifs, i)
		}
	}
	d.Interfaces = ifs

	normalizeEvaluation(&plan.Evaluation)
	var checks []core.RepoCheck
	for _, c := range plan.Consistency {
		c.Area = strings.ToLower(strings.TrimSpace(c.Area))
		c.Status = strings.ToLower(strings.TrimSpace(c.Status))
		c.Note, c.Source = clean(c.Note), "ai"
		if c.Area != "" && !slices.ContainsFunc(checks, func(x core.RepoCheck) bool { return x.Area == c.Area }) {
			checks = append(checks, c)
		}
	}
	plan.Consistency = checks
	var decisions []core.RepoDecision
	for _, d := range plan.Decisions {
		d.Title, d.Choice, d.Reason, d.Tradeoff = clean(d.Title), clean(d.Choice), clean(d.Reason), clean(d.Tradeoff)
		d.Alternatives = cleanList(d.Alternatives, 6)
		if d.Title != "" && !slices.ContainsFunc(decisions, func(x core.RepoDecision) bool { return strings.EqualFold(x.Title, d.Title) }) {
			decisions = append(decisions, d)
		}
	}
	plan.Decisions = decisions
}

func normalizeEvaluation(e *core.RepoEvaluation) {
	e.Summary = clean(e.Summary)
	var crit []core.RepoCriterion
	for _, c := range e.Criteria {
		c.Name = strings.ToLower(strings.TrimSpace(c.Name))
		c.Reason = clean(c.Reason)
		if c.Score > 5 {
			c.Score = 5
		}
		if !slices.Contains(core.RepoCriteria, c.Name) || slices.ContainsFunc(crit, func(x core.RepoCriterion) bool { return x.Name == c.Name }) {
			continue
		}
		crit = append(crit, c)
	}
	sort.SliceStable(crit, func(i, j int) bool {
		return slices.Index(core.RepoCriteria, crit[i].Name) < slices.Index(core.RepoCriteria, crit[j].Name)
	})
	e.Criteria = crit
	var finds []core.RepoFinding
	for _, f := range e.Findings {
		f.Category = strings.ToLower(strings.TrimSpace(f.Category))
		if !slices.Contains(core.RepoFindingCategories, f.Category) {
			f.Category = "other"
		}
		f.Severity = strings.ToLower(strings.TrimSpace(f.Severity))
		if f.Severity != "error" && f.Severity != "warning" {
			f.Severity = "info"
		}
		f.Path, f.Message, f.Resolution = Endpoint(f.Path), clean(f.Message), clean(f.Resolution)
		f.Source = "ai"
		if f.Message != "" {
			finds = append(finds, f)
		}
	}
	e.Findings = finds
	e.Score = evaluationScore(e.Criteria)
}

// evaluationScore is 0-100 from the criteria: eight qualities where more
// is better and complexity where less is better. Missing criteria score 0.
func evaluationScore(c []core.RepoCriterion) int {
	if len(c) < len(core.RepoCriteria) {
		return 0
	}
	sum := 0
	for _, x := range c {
		v := x.Score
		if x.Name == "complexity" {
			v = 6 - v
		}
		sum += max(1, min(5, v))
	}
	n := len(core.RepoCriteria)
	return (sum - n) * 100 / (4 * n)
}

// Describe is the one-line label of a node for messages.
func Describe(n core.RepoNode) string {
	if n.IsDir() {
		return "folder " + n.Path
	}
	return "file " + n.Path
}
