package structure

import (
	"errors"
	"path"
	"slices"
	"strings"
	"unicode"

	"github.com/handrawanwdev/foundry/internal/core"
)

// The Structure IR is what the design (the AI, then the user) chose: the
// style it names, the parts, the folder of every part and module, the
// rules. Foundry keeps no template and derives no folder from a type; it
// only cleans names and paths and checks them. Boundaries follow the
// domains found in discovery (blueprint modules), never endpoints or CRUD
// operations; every part carries the reason it exists.

const (
	maxDomains = 16
	maxRules   = 40
	maxRuleLen = 400
	maxTypeLen = 120
)

type platform struct{ web, mobile, desktop, backend, cli bool }

func platformOf(p *core.Project) platform {
	if p.Stack != nil {
		pl := p.Stack.Platform
		out := platform{web: pl.Web, mobile: pl.Mobile, desktop: pl.Desktop, backend: pl.Backend, cli: pl.CLI}
		if p.Platform != nil {
			for _, t := range p.Platform.Targets {
				if t.Kind == core.PlatformCLI {
					out.cli = true
				}
			}
		}
		return out
	}
	var out platform
	if p.Platform != nil {
		for _, t := range p.Platform.Targets {
			switch t.Kind {
			case core.PlatformWeb:
				out.web = true
			case core.PlatformAndroid, core.PlatformIOS:
				out.mobile = true
			case core.PlatformDesktop:
				out.desktop = true
			case core.PlatformBackend:
				out.backend = true
			case core.PlatformCLI:
				out.cli = true
			}
		}
	}
	return out
}

// moduleReasons indexes the module reasons of every service.
func moduleReasons(ir *core.StructureIR) map[string]string {
	out := map[string]string{}
	for _, s := range ir.Services {
		for _, m := range s.Modules {
			if r := s.ModuleReasons[m]; r != "" {
				out[m] = r
			}
		}
	}
	return out
}

// Domains are the project's domain boundaries: blueprint modules, else
// features, with CRUD verbs removed so "create-booking" and "cancel-booking"
// are one domain "booking".
func Domains(p *core.Project) []string {
	var names []string
	if bp := p.Blueprint; bp != nil && len(bp.Modules) > 0 {
		for _, m := range bp.Modules {
			names = append(names, m.ID)
		}
	} else {
		for _, f := range p.Features {
			names = append(names, f.Name)
		}
	}
	var out []string
	for _, n := range names {
		d := domainOf(n)
		if d != "" && !slices.Contains(out, d) {
			out = append(out, d)
		}
		if len(out) == maxDomains {
			break
		}
	}
	return out
}

// Words that name an operation, not a domain.
var crudWords = map[string]bool{
	"create": true, "get": true, "update": true, "delete": true, "list": true, "cancel": true, "add": true, "edit": true,
	"remove": true, "view": true, "manage": true, "management": true, "crud": true, "buat": true, "tambah": true,
	"ubah": true, "hapus": true, "lihat": true, "kelola": true, "pengelolaan": true, "manajemen": true, "batalkan": true,
	"batal": true, "daftar": true, "cari": true, "detail": true,
}

func domainOf(name string) string {
	words := strings.Split(Slug(name), "-")
	var keep []string
	for _, w := range words {
		if w != "" && !crudWords[w] {
			keep = append(keep, w)
		}
	}
	return Slug(strings.Join(keep, "-"))
}

// crudName reports whether name looks like an operation ("create-booking").
func crudName(name string) bool {
	for _, w := range strings.Split(name, "-") {
		if crudWords[w] {
			return true
		}
	}
	return false
}

// Slug is a part name: lowercase ASCII words joined by "-".
func Slug(s string) string {
	s = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			return r
		case r >= 'A' && r <= 'Z':
			return unicode.ToLower(r)
		}
		return ' '
	}, s)
	s = strings.Join(strings.Fields(s), "-")
	if len(s) > 32 {
		s = strings.TrimRight(s[:32], "-")
	}
	return s
}

// CleanDir turns a folder the design wrote into a repository-relative
// folder: slash separated, no "." or ".." segment, ending with "/". It
// returns "" for anything that cannot be a folder inside the repository.
func CleanDir(raw string) string {
	raw = strings.ReplaceAll(strings.TrimSpace(raw), "\\", "/")
	if strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "~") {
		return ""
	}
	var segs []string
	for _, seg := range strings.Split(raw, "/") {
		seg = strings.TrimSpace(seg)
		switch {
		case seg == "" || seg == ".":
			continue
		case seg == ".." || strings.ContainsAny(seg, ":*?\"<>|"):
			return ""
		}
		segs = append(segs, seg)
	}
	if len(segs) == 0 {
		return ""
	}
	return strings.Join(segs, "/") + "/"
}

// parentOf is the folder that holds dir ("" for a top-level folder).
func parentOf(dir string) string {
	p := path.Dir(strings.TrimSuffix(dir, "/"))
	if p == "." || p == "/" {
		return ""
	}
	return p + "/"
}

// commonParent is the folder all dirs share as their parent, "" when they
// differ or there are none.
func commonParent(dirs []string) (string, bool) {
	if len(dirs) == 0 {
		return "", false
	}
	first := parentOf(dirs[0])
	for _, d := range dirs[1:] {
		if parentOf(d) != first {
			return "", false
		}
	}
	return first, true
}

// placeFor is the folder for a new part named name: next to its siblings
// when they share a parent folder (the layout the design already chose),
// else at the repository root.
func placeFor(name string, siblings ...[]core.StructurePart) string {
	for _, ps := range siblings {
		var dirs []string
		for _, p := range ps {
			if p.Path != "" && p.Name != name {
				dirs = append(dirs, p.Path)
			}
		}
		if parent, ok := commonParent(dirs); ok {
			return parent + name + "/"
		}
	}
	return name + "/"
}

// modulePlace is the folder for module m of svc: next to its other
// modules when they share a parent folder, else inside the service folder.
func modulePlace(svc core.StructurePart, m string) string {
	var dirs []string
	for _, x := range svc.Modules {
		if x != m && svc.ModulePaths[x] != "" {
			dirs = append(dirs, svc.ModulePaths[x])
		}
	}
	if parent, ok := commonParent(dirs); ok && parent != "" {
		return parent + m + "/"
	}
	return svc.Path + m + "/"
}

// rebase moves dir from under old to under next; dir outside old stays.
func rebase(dir, old, next string) string {
	if old != "" && strings.HasPrefix(dir, old) {
		return next + strings.TrimPrefix(dir, old)
	}
	return dir
}

// renamed is dir with its last folder renamed from old to next, when the
// folder carries the part's name; otherwise dir stays.
func renamed(dir, old, next string) string {
	trim := strings.TrimSuffix(dir, "/")
	if path.Base(trim) != old {
		return dir
	}
	return parentOf(dir) + next + "/"
}

// Normalize cleans what the design wrote without changing its choices:
// names become slugs, duplicates go, paths are cleaned, a part or module
// without a folder gets one next to its siblings.
func Normalize(ir *core.StructureIR) {
	ir.Type = strings.Join(strings.Fields(ir.Type), " ")
	if r := []rune(ir.Type); len(r) > maxTypeLen {
		ir.Type = strings.TrimSpace(string(r[:maxTypeLen]))
	}
	ir.Docs = CleanDir(ir.Docs)
	fix := func(ps []core.StructurePart) []core.StructurePart {
		var out []core.StructurePart
		for _, p := range ps {
			p.Name = Slug(p.Name)
			if p.Name == "" || slices.ContainsFunc(out, func(x core.StructurePart) bool { return x.Name == p.Name }) {
				continue
			}
			p.Path = CleanDir(p.Path)
			p.Reason = strings.TrimSpace(p.Reason)
			out = append(out, p)
		}
		return out
	}
	ir.Apps, ir.Services, ir.Workers, ir.Packages = fix(ir.Apps), fix(ir.Services), fix(ir.Workers), fix(ir.Packages)
	all := [][]core.StructurePart{ir.Apps, ir.Services, ir.Workers, ir.Packages}
	for k, ps := range all {
		for i := range ps {
			if ps[i].Path == "" {
				order := append([][]core.StructurePart{ps}, all[:k]...)
				ps[i].Path = placeFor(ps[i].Name, append(order, all[k+1:]...)...)
			}
		}
	}
	for i := range ir.Apps {
		ir.Apps[i].Modules, ir.Apps[i].ModuleReasons, ir.Apps[i].ModulePaths = nil, nil, nil
	}
	seen := map[string]bool{}
	for i := range ir.Services {
		svc := &ir.Services[i]
		var mods []string
		reasons, paths := map[string]string{}, map[string]string{}
		for _, m := range svc.Modules {
			orig := m
			if m = Slug(m); m == "" || seen[m] {
				continue
			}
			seen[m] = true
			mods = append(mods, m)
			for _, k := range []string{orig, m} {
				if r := strings.TrimSpace(svc.ModuleReasons[k]); r != "" && reasons[m] == "" {
					reasons[m] = r
				}
				if d := CleanDir(svc.ModulePaths[k]); d != "" && paths[m] == "" {
					paths[m] = d
				}
			}
		}
		svc.Modules, svc.ModuleReasons, svc.ModulePaths = mods, reasons, paths
		for _, m := range mods {
			if svc.ModulePaths[m] == "" {
				svc.ModulePaths[m] = modulePlace(*svc, m)
			}
		}
		if len(reasons) == 0 {
			svc.ModuleReasons = nil
		}
		if len(svc.ModulePaths) == 0 {
			svc.ModulePaths = nil
		}
	}
	ir.Rules = cleanRules(ir.Rules)
}

// cleanRules trims the rules and drops empty and duplicate ones.
func cleanRules(rules []string) []string {
	var out []string
	for _, r := range rules {
		r = strings.Join(strings.Fields(r), " ")
		if len(r) > maxRuleLen {
			r = strings.TrimSpace(r[:maxRuleLen])
		}
		if r == "" || slices.ContainsFunc(out, func(x string) bool { return strings.EqualFold(x, r) }) {
			continue
		}
		if out = append(out, r); len(out) == maxRules {
			break
		}
	}
	return out
}

// ruleIndex finds a rule by its text, ignoring case and spacing.
func ruleIndex(rules []string, text string) int {
	text = strings.Join(strings.Fields(text), " ")
	return slices.IndexFunc(rules, func(r string) bool { return strings.EqualFold(r, text) })
}

// Layout is every folder of the IR, in scaffold order.
func Layout(ir *core.StructureIR) []string {
	var out []string
	for _, p := range ir.Apps {
		out = append(out, p.Path)
	}
	for _, p := range ir.Services {
		out = append(out, p.Path)
		for _, m := range p.Modules {
			out = append(out, ModulePath(p, m))
		}
	}
	for _, ps := range [][]core.StructurePart{ir.Workers, ir.Packages} {
		for _, p := range ps {
			out = append(out, p.Path)
		}
	}
	if ir.Docs != "" {
		out = append(out, ir.Docs)
	}
	return out
}

// Tree draws the layout as a folder tree.
func Tree(ir *core.StructureIR) string {
	type node struct {
		name string
		kids []*node
	}
	root := &node{}
	for _, l := range Layout(ir) {
		at := root
		for _, part := range strings.Split(strings.TrimSuffix(l, "/"), "/") {
			if part == "" {
				continue
			}
			i := slices.IndexFunc(at.kids, func(n *node) bool { return n.name == part })
			if i < 0 {
				at.kids = append(at.kids, &node{name: part})
				i = len(at.kids) - 1
			}
			at = at.kids[i]
		}
	}
	var b strings.Builder
	var walk func(n *node, prefix string)
	walk = func(n *node, prefix string) {
		for i, k := range n.kids {
			branch, next := "├── ", "│   "
			if i == len(n.kids)-1 {
				branch, next = "└── ", "    "
			}
			b.WriteString(prefix + branch + k.name + "/\n")
			walk(k, prefix+next)
		}
	}
	for _, k := range root.kids {
		b.WriteString(k.name + "/\n")
		walk(k, "")
	}
	return b.String()
}

// ModulePath is the folder of module m inside service svc.
func ModulePath(svc core.StructurePart, m string) string {
	if d := svc.ModulePaths[m]; d != "" {
		return d
	}
	return svc.Path + m + "/"
}

// domainService reports whether a service is a domain itself: one without
// modules. A service with modules is a container of domains.
func domainService(s core.StructurePart) bool { return len(s.Modules) == 0 }

// domainsIn are the domains of an IR in order: domain services and modules.
func domainsIn(ir *core.StructureIR) []string {
	var out []string
	add := func(d string) {
		if d != "" && !slices.Contains(out, d) {
			out = append(out, d)
		}
	}
	for _, s := range ir.Services {
		if domainService(s) {
			add(s.Name)
		}
		for _, m := range s.Modules {
			add(m)
		}
	}
	return out
}

var errUnknownKind = errors.New("jenis bagian tidak dikenal (app, service, module, worker, package, rule)")
