package repoplan

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
)

// Issue areas. A "consistency" error is an unresolved conflict the AI
// reported itself: it blocks approval but is not a reason to refuse the
// AI's answer.
const (
	areaFolders     = "folders"
	areaModules     = "modules"
	areaStructure   = "structure"
	areaDependency  = "dependency"
	areaEvaluation  = "evaluation"
	areaConsistency = "consistency"
	areaSources     = "sources"
)

type issues []core.TechIssue

func (is *issues) add(level, area, format string, args ...any) {
	*is = append(*is, core.TechIssue{Level: level, Area: area, Message: fmt.Sprintf(format, args...)})
}

// list adds one issue that names up to three of many paths.
func (is *issues) list(level, area, what string, paths []string) {
	if len(paths) == 0 {
		return
	}
	shown := paths
	more := ""
	if len(shown) > 3 {
		shown, more = shown[:3], fmt.Sprintf(" dan %d lainnya", len(paths)-3)
	}
	is.add(level, area, "%d %s: %s%s", len(paths), what, strings.Join(shown, ", "), more)
}

// index resolves endpoints (module ids, shared/platform and node paths).
type index struct {
	plan   *core.RepoPlan
	mods   map[string]bool
	byPath map[string]*core.RepoNode
}

func newIndex(p *core.Project, plan *core.RepoPlan) *index {
	x := &index{plan: plan, mods: map[string]bool{}, byPath: map[string]*core.RepoNode{}}
	for _, m := range Modules(p) {
		x.mods[m.ID] = true
	}
	for i := range plan.Nodes {
		x.byPath[plan.Nodes[i].Path] = &plan.Nodes[i]
	}
	return x
}

func isUnit(s string) bool { return s == core.OwnerShared || s == core.OwnerPlatform }

func (x *index) validOwner(o string) bool { return x.mods[o] || isUnit(o) }

// node is the folder or file at p, with or without a trailing slash.
func (x *index) node(p string) *core.RepoNode {
	if n := x.byPath[p]; n != nil {
		return n
	}
	if !strings.HasSuffix(p, "/") {
		return x.byPath[p+"/"]
	}
	return x.byPath[strings.TrimSuffix(p, "/")]
}

// ep is a resolved dependency endpoint: a unit (module, shared, platform),
// and when it is a path the node it points at.
type ep struct {
	raw, unit, path string
}

func (x *index) resolve(e string) (ep, bool) {
	if !strings.ContainsAny(e, "/.") && x.validOwner(e) {
		return ep{raw: e, unit: e}, true
	}
	for _, key := range []string{e, e + "/"} {
		if n := x.byPath[key]; n != nil {
			return ep{raw: e, unit: n.Owner, path: n.Path}, true
		}
	}
	return ep{}, false
}

// covers reports whether endpoint a includes b: a unit covers everything
// that unit owns, a folder everything below it, a file only itself.
func covers(a, b ep) bool {
	switch {
	case a.path == "":
		return a.unit == b.unit
	case b.path == "":
		return false
	case strings.HasSuffix(a.path, "/"):
		return strings.HasPrefix(b.path, a.path)
	}
	return a.path == b.path
}

func nodeEP(n *core.RepoNode) ep { return ep{raw: n.Path, unit: n.Owner, path: n.Path} }

// nodeEdge is a dependency one node declares.
type nodeEdge struct {
	from *core.RepoNode
	to   ep
}

// Refresh recomputes everything Foundry derives from the plan and the
// project. It does not touch the nodes; call Normalize first when the plan
// came from outside.
func Refresh(p *core.Project, plan *core.RepoPlan) {
	if plan == nil {
		return
	}
	a := analyze(p, plan)
	plan.Analysis = a
	plan.Stale = len(a.StaleAreas) > 0
}

func analyze(p *core.Project, plan *core.RepoPlan) core.RepoAnalysis {
	x := newIndex(p, plan)
	var is issues
	var a core.RepoAnalysis

	for _, n := range plan.Nodes {
		if n.IsDir() {
			a.Dirs++
		} else {
			a.Files++
		}
		a.MaxDepth = max(a.MaxDepth, n.Depth())
	}
	if len(plan.Nodes) == 0 {
		is.add("error", areaFolders, "rencana belum punya folder atau file")
	}
	checkNodes(x, &is)
	checkModules(p, x, &is)
	checkParts(p, x, &is)
	edges, cyc := checkDependency(x, &is, &a)
	checkAISections(plan, &is)

	a.StaleAreas = StaleAreas(p, plan)
	a.Affected = Affected(a.StaleAreas)
	if len(a.StaleAreas) > 0 {
		is.add("warning", areaSources, "sumber berubah setelah rencana ditulis (%s); bagian terdampak: %s. Minta AI merevisi bagian itu.",
			strings.Join(a.StaleAreas, ", "), strings.Join(a.Affected, ", "))
	}
	if plan.Evaluation.Outdated {
		is.add("warning", areaEvaluation, "struktur diubah manual; evaluasi AI belum diperbarui")
	}
	a.Findings = findings(x, edges, cyc, &a)
	a.Issues = is
	a.Checks = checksOf(plan, is)
	return a
}

func checkNodes(x *index, is *issues) {
	var noResp, noReason, badOwner, noAllowed, implied, badShared []string
	for _, n := range x.plan.Nodes {
		if n.Implied {
			implied = append(implied, n.Path)
			continue
		}
		if n.Responsibility == "" {
			noResp = append(noResp, n.Path)
		}
		if n.Reason == "" {
			noReason = append(noReason, n.Path)
		}
		if !x.validOwner(n.Owner) {
			badOwner = append(badOwner, fmt.Sprintf("%s (owner %q)", n.Path, n.Owner))
		}
		if n.IsDir() && (len(n.Allowed) == 0 || len(n.Forbidden) == 0) {
			noAllowed = append(noAllowed, n.Path)
		}
		for _, m := range n.SharedBy {
			if !x.mods[m] {
				badShared = append(badShared, fmt.Sprintf("%s (%q)", n.Path, m))
			}
		}
	}
	is.list("error", areaFolders, "folder/file tanpa responsibility", noResp)
	is.list("error", areaFolders, "folder/file tanpa alasan keberadaan", noReason)
	is.list("error", areaFolders, "folder/file dengan owner yang bukan modul project, shared, atau platform", badOwner)
	is.list("error", areaFolders, "folder/file dengan shared_by yang bukan modul project", badShared)
	is.list("warning", areaFolders, "folder tanpa daftar kode yang boleh dan tidak boleh ditempatkan", noAllowed)
	is.list("warning", areaFolders, "folder induk ditambahkan Foundry karena tidak dijelaskan AI", implied)

	// Ownership: a module's code must not sit inside another module's
	// folder; a business module and a technical folder are told apart by
	// the owner.
	var leaks []string
	for _, n := range x.plan.Nodes {
		if !x.mods[n.Owner] {
			continue
		}
		for parent := n.Parent(); parent != ""; {
			pn := x.byPath[parent]
			if pn == nil {
				break
			}
			if x.mods[pn.Owner] {
				if pn.Owner != n.Owner && !slices.Contains(n.SharedBy, pn.Owner) {
					leaks = append(leaks, fmt.Sprintf("%s (milik %s di dalam folder milik %s)", n.Path, n.Owner, pn.Owner))
				}
				break
			}
			parent = pn.Parent()
		}
	}
	is.list("warning", areaFolders, "folder/file yang pemiliknya bercampur dengan folder induk", leaks)
}

func checkModules(p *core.Project, x *index, is *issues) {
	mapped := map[string]*core.RepoModuleMap{}
	for i := range x.plan.Modules {
		m := &x.plan.Modules[i]
		mapped[m.Module] = m
		if !x.mods[m.Module] {
			is.add("error", areaModules, "pemetaan untuk %q, yang bukan Project Module", m.Module)
		}
	}
	var missing []string
	for _, m := range Modules(p) {
		mm := mapped[m.ID]
		if mm == nil {
			missing = append(missing, m.ID)
			continue
		}
		if len(mm.Locations) == 0 {
			if mm.Frontend || mm.Backend {
				is.add("error", areaModules, "modul %s punya implementasi frontend/backend tetapi tanpa lokasi di repository", m.ID)
			} else if mm.Note == "" {
				is.add("error", areaModules, "modul %s tanpa lokasi dan tanpa alasan tidak membutuhkan implementasi frontend maupun backend", m.ID)
			}
			continue
		}
		for _, l := range mm.Locations {
			n := x.node(l.Path)
			switch {
			case n == nil:
				is.add("error", areaModules, "lokasi %s milik modul %s tidak ada di struktur folder", l.Path, m.ID)
			case n.Owner != m.ID && !slices.Contains(n.SharedBy, m.ID) && n.Owner != core.OwnerShared:
				is.add("warning", areaModules, "lokasi %s dipakai modul %s tetapi pemiliknya %s", l.Path, m.ID, n.Owner)
			}
		}
	}
	if len(missing) > 0 {
		is.add("error", areaModules, "%d Project Module belum punya lokasi di repository: %s", len(missing), strings.Join(missing, ", "))
	}
	// A folder used by several modules needs the reason.
	users := map[string][]string{}
	for _, m := range x.plan.Modules {
		for _, l := range m.Locations {
			users[l.Path] = append(users[l.Path], m.Module)
		}
	}
	paths := make([]string, 0, len(users))
	for k := range users {
		paths = append(paths, k)
	}
	sort.Strings(paths)
	for _, path := range paths {
		if len(users[path]) < 2 {
			continue
		}
		if n := x.node(path); n != nil && (n.Reason == "" || n.Implied || n.Owner != core.OwnerShared && len(n.SharedBy) < 2) {
			is.add("warning", areaModules, "folder %s dipakai modul %s; tandai owner shared atau shared_by dan jelaskan alasannya", path, strings.Join(users[path], ", "))
		}
	}
}

func checkParts(p *core.Project, x *index, is *issues) {
	ir := p.Structure.IR
	parts := IRParts(ir)
	if len(parts) == 0 {
		return
	}
	known := map[string]IRPart{}
	for _, part := range parts {
		known[part.Ref] = part
	}
	where := map[string][]string{}
	for _, n := range x.plan.Nodes {
		if n.Part == "" {
			continue
		}
		part, ok := known[n.Part]
		switch {
		case !ok:
			is.add("error", areaStructure, "%s mengacu ke bagian struktur %q yang tidak ada di keputusan struktur", n.Path, n.Part)
			continue
		case !n.IsDir():
			is.add("error", areaStructure, "bagian struktur %s harus dipetakan ke folder, bukan file %s", part.Ref, n.Path)
			continue
		}
		where[n.Part] = append(where[n.Part], n.Path)
	}
	var unmapped, moved []string
	for _, part := range parts {
		switch w := where[part.Ref]; {
		case len(w) == 0:
			unmapped = append(unmapped, part.Ref)
		case len(w) > 1:
			is.add("error", areaStructure, "bagian struktur %s dipetakan ke lebih dari satu folder: %s", part.Ref, strings.Join(w, ", "))
		case w[0] != part.Path:
			moved = append(moved, fmt.Sprintf("%s di %s (default %s)", part.Ref, w[0], part.Path))
		}
	}
	is.list("error", areaStructure, "bagian keputusan struktur belum dipetakan ke folder", unmapped)
	is.list("warning", areaStructure, "bagian struktur ditempatkan di luar lokasi default; rencana repository menjadi sumber kebenaran", moved)
}

// cycle is a circular dependency: the members of one strongly connected
// component, between units or between paths.
type cycle struct {
	between string
	members []string
}

// checkDependency validates the dependency architecture and returns the
// node-level edges and cycles for the findings.
func checkDependency(x *index, is *issues, a *core.RepoAnalysis) ([]nodeEdge, []cycle) {
	d := x.plan.Dependency
	type rule struct {
		kind string
		core.RepoDepRule
		from, to ep
	}
	var rules []rule
	for _, set := range []struct {
		kind  string
		rules []core.RepoDepRule
	}{{"allowed", d.Allowed}, {"forbidden", d.Forbidden}, {"communication", d.Communication}} {
		for _, r := range set.rules {
			f, okF := x.resolve(r.From)
			t, okT := x.resolve(r.To)
			if !okF || !okT {
				bad := r.From
				if okF {
					bad = r.To
				}
				is.add("error", areaDependency, "aturan %s %s → %s mengacu ke %q yang bukan modul atau path di struktur", set.kind, r.From, r.To, bad)
				continue
			}
			rules = append(rules, rule{kind: set.kind, RepoDepRule: r, from: f, to: t})
		}
	}
	// Edges the nodes declare.
	var edges []nodeEdge
	for i := range x.plan.Nodes {
		n := &x.plan.Nodes[i]
		for _, dep := range n.DependsOn {
			t, ok := x.resolve(dep)
			if !ok {
				is.add("error", areaDependency, "%s bergantung pada %q, yang bukan modul atau path di struktur", n.Path, dep)
				continue
			}
			edges = append(edges, nodeEdge{from: n, to: t})
		}
	}
	// Forbidden rules against declared edges and against allowed rules.
	for _, f := range rules {
		if f.kind != "forbidden" {
			continue
		}
		for _, e := range edges {
			if covers(f.from, nodeEP(e.from)) && covers(f.to, e.to) {
				is.add("error", areaDependency, "%s bergantung pada %s, padahal dilarang oleh aturan %s → %s", e.from.Path, e.to.raw, f.From, f.To)
			}
		}
		for _, r := range rules {
			if r.kind != "forbidden" && covers(f.from, r.from) && covers(f.to, r.to) {
				is.add("error", areaDependency, "aturan %s %s → %s bertentangan dengan aturan forbidden %s → %s", r.kind, r.From, r.To, f.From, f.To)
			}
		}
	}

	// Interfaces.
	ifaces := map[string][]string{}
	for _, i := range d.Interfaces {
		if !x.mods[i.Module] {
			is.add("error", areaDependency, "public interface mengacu ke modul %q yang bukan Project Module", i.Module)
			continue
		}
		if n := x.node(i.Path); n == nil {
			is.add("error", areaDependency, "public interface %s milik modul %s tidak ada di struktur folder", i.Path, i.Module)
		} else {
			ifaces[i.Module] = append(ifaces[i.Module], n.Path)
		}
	}
	var internal []string
	for _, e := range edges {
		u := e.to.unit
		if !x.mods[u] || u == e.from.Owner || e.to.path == "" || len(ifaces[u]) == 0 {
			continue
		}
		if !slices.ContainsFunc(ifaces[u], func(pi string) bool { return strings.HasPrefix(e.to.path, pi) }) {
			internal = append(internal, fmt.Sprintf("%s → %s", e.from.Path, e.to.path))
		}
	}
	is.list("warning", areaDependency, "dependensi lintas modul yang melewati public interface", internal)

	// Allowed rules should cover the cross-unit edges the nodes declare.
	if len(d.Allowed) > 0 {
		var uncovered []string
		for _, e := range edges {
			if e.to.unit == e.from.Owner {
				continue
			}
			ok := false
			for _, r := range rules {
				if r.kind != "forbidden" && covers(r.from, nodeEP(e.from)) && covers(r.to, e.to) {
					ok = true
					break
				}
			}
			if !ok {
				uncovered = append(uncovered, fmt.Sprintf("%s → %s", e.from.Path, e.to.raw))
			}
		}
		is.list("warning", areaDependency, "dependensi lintas unit yang tidak tercantum di aturan allowed/communication", uncovered)
	}

	// Cycles: between units over synchronous edges, and between paths.
	units := map[string]map[string]bool{}
	link := func(from, to string) {
		if from == to {
			return
		}
		if units[from] == nil {
			units[from] = map[string]bool{}
		}
		units[from][to] = true
	}
	for _, e := range edges {
		link(e.from.Owner, e.to.unit)
	}
	for _, r := range rules {
		if r.kind != "forbidden" && r.Mode != core.DepEvent {
			link(r.from.unit, r.to.unit)
		}
	}
	var cyc []cycle
	for _, c := range cycles(units) {
		cyc = append(cyc, cycle{"unit", c})
		is.add("error", areaDependency, "dependensi melingkar antar-unit: %s", strings.Join(c, " ↔ "))
	}
	paths := map[string]map[string]bool{}
	for _, e := range edges {
		if e.to.path == "" || e.to.path == e.from.Path {
			continue
		}
		if paths[e.from.Path] == nil {
			paths[e.from.Path] = map[string]bool{}
		}
		paths[e.from.Path][e.to.path] = true
	}
	for _, c := range cycles(paths) {
		cyc = append(cyc, cycle{"path", c})
		is.add("error", areaDependency, "dependensi melingkar antar-path: %s", strings.Join(c, " ↔ "))
	}
	if d.Direction == "" {
		is.add("error", areaDependency, "arah dependency belum dijelaskan")
	}

	diagram(x, edges, a)
	return edges, cyc
}

// diagram fills the units and edges the UI draws.
func diagram(x *index, edges []nodeEdge, a *core.RepoAnalysis) {
	d := x.plan.Dependency
	seen := map[string]bool{}
	add := func(from, to, kind, mode string) {
		if from == "" || to == "" {
			return
		}
		key := from + "|" + to + "|" + kind
		if seen[key] {
			return
		}
		seen[key] = true
		a.Edges = append(a.Edges, core.RepoEdge{From: from, To: to, Kind: kind, Mode: mode})
	}
	unitOf := func(e string) string {
		if r, ok := x.resolve(e); ok {
			return r.unit
		}
		return ""
	}
	for _, e := range edges {
		if e.to.unit != e.from.Owner {
			add(e.from.Owner, e.to.unit, "derived", core.DepSync)
		}
	}
	for _, set := range []struct {
		kind  string
		rules []core.RepoDepRule
	}{{"allowed", d.Allowed}, {"forbidden", d.Forbidden}, {"communication", d.Communication}} {
		for _, r := range set.rules {
			if f, t := unitOf(r.From), unitOf(r.To); f != t {
				add(f, t, set.kind, r.Mode)
			}
		}
	}
	sort.SliceStable(a.Edges, func(i, j int) bool {
		if a.Edges[i].From != a.Edges[j].From {
			return a.Edges[i].From < a.Edges[j].From
		}
		return a.Edges[i].To < a.Edges[j].To
	})
	out, in := map[string]map[string]bool{}, map[string]map[string]bool{}
	for _, e := range a.Edges {
		if e.Kind == "forbidden" || e.Kind == "allowed" || e.Mode == core.DepEvent {
			continue
		}
		if out[e.From] == nil {
			out[e.From] = map[string]bool{}
		}
		if in[e.To] == nil {
			in[e.To] = map[string]bool{}
		}
		out[e.From][e.To], in[e.To][e.From] = true, true
	}
	count := map[string]int{}
	for _, n := range x.plan.Nodes {
		count[n.Owner]++
	}
	names := map[string]bool{}
	for m := range x.mods {
		names[m] = true
	}
	for u := range count {
		names[u] = true
	}
	for _, e := range a.Edges {
		names[e.From], names[e.To] = true, true
	}
	for u := range names {
		a.Units = append(a.Units, core.RepoUnit{Name: u, Nodes: count[u], FanOut: len(out[u]), FanIn: len(in[u])})
	}
	sort.Slice(a.Units, func(i, j int) bool { return a.Units[i].Name < a.Units[j].Name })
}

// cycles returns the strongly connected components with more than one
// member (Tarjan), each sorted; they are the circular dependencies.
func cycles(g map[string]map[string]bool) [][]string {
	index, low := map[string]int{}, map[string]int{}
	on := map[string]bool{}
	var stack []string
	var out [][]string
	next := 0
	nodes := make([]string, 0, len(g))
	for k := range g {
		nodes = append(nodes, k)
	}
	sort.Strings(nodes)
	var visit func(v string)
	visit = func(v string) {
		index[v], low[v] = next, next
		next++
		stack, on[v] = append(stack, v), true
		succ := make([]string, 0, len(g[v]))
		for w := range g[v] {
			succ = append(succ, w)
		}
		sort.Strings(succ)
		for _, w := range succ {
			if _, seen := index[w]; !seen {
				visit(w)
				low[v] = min(low[v], low[w])
			} else if on[w] {
				low[v] = min(low[v], index[w])
			}
		}
		if low[v] != index[v] {
			return
		}
		var comp []string
		for {
			w := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			on[w] = false
			comp = append(comp, w)
			if w == v {
				break
			}
		}
		if len(comp) > 1 {
			sort.Strings(comp)
			out = append(out, comp)
		}
	}
	for _, v := range nodes {
		if _, seen := index[v]; !seen {
			visit(v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i][0] < out[j][0] })
	return out
}

// checkAISections requires what only the architect can write.
func checkAISections(plan *core.RepoPlan, is *issues) {
	for _, f := range []struct{ name, v string }{{"pendekatan organisasi (approach)", plan.Approach}, {"topologi repository", plan.Topology}, {"ringkasan", plan.Summary}} {
		if f.v == "" {
			is.add("error", areaEvaluation, "%s belum ditulis", f.name)
		}
	}
	if len(plan.Decisions) == 0 {
		is.add("error", areaEvaluation, "keputusan desain belum dicatat (minimal satu, dengan alasan)")
	}
	for _, d := range plan.Decisions {
		if d.Choice == "" || d.Reason == "" {
			is.add("error", areaEvaluation, "keputusan desain %q harus punya pilihan dan alasan", d.Title)
		}
	}
	var missing, noReason []string
	for _, name := range core.RepoCriteria {
		i := slices.IndexFunc(plan.Evaluation.Criteria, func(c core.RepoCriterion) bool { return c.Name == name })
		switch {
		case i < 0 || plan.Evaluation.Criteria[i].Score < 1:
			missing = append(missing, name)
		case plan.Evaluation.Criteria[i].Reason == "":
			noReason = append(noReason, name)
		}
	}
	if len(missing) > 0 {
		is.add("error", areaEvaluation, "evaluasi kualitas belum menilai (skor 1-5): %s", strings.Join(missing, ", "))
	}
	if len(noReason) > 0 {
		is.add("error", areaEvaluation, "skor tanpa alasan konkret: %s", strings.Join(noReason, ", "))
	}
	var uncovered []string
	for _, area := range core.RepoConsistencyAreas {
		i := slices.IndexFunc(plan.Consistency, func(c core.RepoCheck) bool { return c.Area == area })
		if i < 0 {
			uncovered = append(uncovered, area)
			continue
		}
		c := plan.Consistency[i]
		switch c.Status {
		case core.CheckConsistent, core.CheckResolved:
		case core.CheckConflict:
			is.add("error", areaConsistency, "konflik %s belum diselesaikan: %s", area, c.Note)
		default:
			is.add("error", areaEvaluation, "status konsistensi %s harus consistent, resolved, atau conflict", area)
		}
	}
	if len(uncovered) > 0 {
		is.add("error", areaEvaluation, "validasi konsistensi belum mencakup: %s", strings.Join(uncovered, ", "))
	}
}

// checksOf are the consistency checks Foundry verifies itself, next to the
// ones the AI reported.
func checksOf(plan *core.RepoPlan, is issues) []core.RepoCheck {
	out := slices.Clone(plan.Consistency)
	for i := range out {
		out[i].Source = "ai"
	}
	verify := func(area, issueArea, ok, bad string) {
		n := 0
		first := ""
		for _, i := range is {
			if i.Level == "error" && i.Area == issueArea {
				if n++; first == "" {
					first = i.Message
				}
			}
		}
		c := core.RepoCheck{Area: area, Status: core.CheckConsistent, Note: ok, Source: "computed"}
		if n > 0 {
			c.Status, c.Note = core.CheckConflict, fmt.Sprintf("%s (%d masalah): %s", bad, n, first)
		}
		out = append(out, c)
	}
	verify("project_modules", areaModules, "Setiap Project Module punya lokasi.", "Pemetaan modul bermasalah")
	verify("folder_responsibility", areaFolders, "Setiap folder dan file punya responsibility, alasan, dan owner.", "Dokumentasi folder bermasalah")
	verify("dependency_rules", areaDependency, "Tidak ada dependensi melingkar atau yang melanggar aturan forbidden.", "Aturan dependency dilanggar")
	verify("structure_decision", areaStructure, "Setiap bagian keputusan struktur dipetakan ke satu folder.", "Pemetaan keputusan struktur bermasalah")
	return out
}

// Blocking reports why the plan cannot be accepted from the AI: every error
// except a conflict the AI reported honestly.
func Blocking(plan *core.RepoPlan) []core.TechIssue {
	var out []core.TechIssue
	for _, i := range plan.Analysis.Issues {
		if i.Level == "error" && i.Area != areaConsistency {
			out = append(out, i)
		}
	}
	return out
}
