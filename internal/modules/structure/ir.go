package structure

import (
	"errors"
	"slices"
	"strings"
	"unicode"

	"github.com/handrawanwdev/foundry/internal/core"
)

// Building and editing the Structure IR. Boundaries follow the domains
// found in discovery (blueprint modules), never endpoints or CRUD
// operations; every part carries the reason it exists.

const (
	workersSeparate = "Worker terpisah + antrean"
	maxDomains      = 16
	maxRules        = 40
	maxRuleLen      = 400
)

type platform struct{ web, mobile, desktop, backend, cli bool }

func platformOf(p *core.Project) platform {
	if p.Stack != nil {
		pl := p.Stack.Platform
		out := platform{web: pl.Web, mobile: pl.Mobile, desktop: pl.Desktop, backend: pl.Backend}
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

// Recommend is the simplest structure that meets the project's needs. The
// type follows the technology decision (architecture and repository),
// which is itself derived from the needs.
func Recommend(p *core.Project) (*core.StructureIR, string) {
	t, why := recommendType(p)
	pl := platformOf(p)
	ir := &core.StructureIR{Type: t, Docs: "docs/"}
	for _, a := range []struct {
		on         bool
		name, what string
	}{{pl.web, "web", "Aplikasi web"}, {pl.mobile, "mobile", "Aplikasi mobile (Android/iOS)"}, {pl.desktop, "desktop", "Aplikasi desktop"}, {pl.cli, "cli", "CLI/tools"}} {
		if a.on {
			ir.Apps = append(ir.Apps, core.StructurePart{Name: a.name, Reason: a.what + " ada di platform yang dipilih."})
		}
	}
	if pl.backend {
		ir.Services = servicesFor(t, Domains(p))
	}
	ir.Workers = workersOf(p)
	ir.Packages = packagesFor(ir, p)
	Normalize(ir)
	return ir, why
}

func recommendType(p *core.Project) (string, string) {
	s := p.Stack
	if s == nil {
		pl := platformOf(p)
		switch {
		case (pl.mobile || pl.desktop) && pl.web:
			return core.StructMonorepo, "Web dan aplikasi lain berbagi kode dalam satu repo."
		case pl.backend && pl.web:
			return core.StructModular, "Backend dibagi per domain dalam satu aplikasi."
		}
		return core.StructSimple, "Aplikasi kecil; satu folder per bagian sudah cukup."
	}
	reason := func(area string) string {
		if d := s.Decision(area); d != nil {
			return d.Reason
		}
		return ""
	}
	switch {
	case s.Choice(core.TechArchitecture) == core.ArchMicroservices:
		return core.StructMicroservices, "Microservices: " + reason(core.TechArchitecture)
	case s.Choice(core.TechRepository) == core.RepoMonorepo:
		return core.StructMonorepo, "Monorepo: " + reason(core.TechRepository)
	case s.Choice(core.TechArchitecture) == core.ArchModular:
		return core.StructModular, "Modular: " + reason(core.TechArchitecture)
	}
	return core.StructSimple, "Simple: " + reason(core.TechArchitecture)
}

func servicesFor(t string, domains []string) []core.StructurePart {
	switch t {
	case core.StructSimple:
		return []core.StructurePart{{Name: "api", Reason: "Satu API tanpa batas modul; cukup untuk fitur yang sedikit."}}
	case core.StructModular:
		return []core.StructurePart{{Name: "api", Modules: domains, Reason: "Satu API (modular monolith); tiap domain jadi modul di src/modules, satu deploy."}}
	case core.StructMonorepo:
		return []core.StructurePart{{Name: "api", Modules: domains, Reason: "Satu API di apps/api; tiap domain jadi modul."}}
	}
	if len(domains) == 0 {
		return []core.StructurePart{{Name: "api", Reason: "Belum ada domain; satu service dulu."}}
	}
	var out []core.StructurePart
	for _, d := range domains {
		out = append(out, core.StructurePart{Name: d, Reason: "Domain " + d + " di-deploy dan diskalakan sendiri."})
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

// Slug is a folder name: lowercase ASCII words joined by "-".
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

func workersOf(p *core.Project) []core.StructurePart {
	if p.Stack == nil || p.Stack.Choice(core.TechWorkers) != workersSeparate {
		return nil
	}
	reason := "Consumer event dan job queue di luar request."
	if bp := p.Blueprint; bp != nil && len(bp.Workers) > 0 {
		var jobs []string
		for _, w := range bp.Workers {
			if w.Job != "" {
				jobs = append(jobs, w.Name+": "+w.Job)
			} else if w.Name != "" {
				jobs = append(jobs, w.Name)
			}
		}
		if len(jobs) > 0 {
			reason = strings.Join(jobs, "; ")
		}
	}
	return []core.StructurePart{{Name: "worker", Reason: reason}}
}

// packagesFor adds shared packages only where code is really shared: a
// monorepo with more than one consumer.
func packagesFor(ir *core.StructureIR, p *core.Project) []core.StructurePart {
	if ir.Type == core.StructSimple {
		return nil
	}
	var out []core.StructurePart
	if len(ir.Services) > 0 {
		out = append(out,
			core.StructurePart{Name: "domain", Reason: "Entity, rule, dan kalkulasi murni tanpa I/O."},
			core.StructurePart{Name: "contracts", Reason: "Skema input/output dan event; satu sumber kebenaran."},
		)
	}
	if js := jsApps(ir, p.Stack); len(js) >= 2 {
		out = append(out, core.StructurePart{Name: "ui", Reason: "Komponen dan design tokens dipakai bersama oleh " + strings.Join(js, ", ") + "."})
	} else if ir.Type != core.StructSimple && len(ir.Apps) > 0 {
		out = append(out, core.StructurePart{Name: "ui", Reason: "Komponen dan design tokens frontend."})
	}
	if ir.Type == core.StructMonorepo || ir.Type == core.StructMicroservices {
		out = append(out, core.StructurePart{Name: "config", Reason: "Lint, tsconfig, dan tooling bersama."})
	}
	return out
}

// jsApps are the apps whose UI is React-based and can share components.
func jsApps(ir *core.StructureIR, s *core.TechStack) []string {
	var out []string
	for _, a := range ir.Apps {
		if s == nil {
			if a.Name == "web" || a.Name == "desktop" {
				out = append(out, a.Name)
			}
			continue
		}
		tech := map[string]string{"web": s.Choice(core.TechFrontend), "mobile": s.Choice(core.TechMobile), "desktop": s.Choice(core.TechDesktop)}
		t := strings.ToLower(tech[a.Name])
		if strings.Contains(t, "react") || strings.Contains(t, "electron") || strings.Contains(t, "tauri") || strings.Contains(t, "vue") || strings.Contains(t, "next") {
			out = append(out, a.Name)
		}
	}
	return out
}

// Normalize slugs the names, drops duplicates and derives every path from
// the type, so folders never disagree with the IR.
func Normalize(ir *core.StructureIR) {
	if ir.Docs == "" {
		ir.Docs = "docs/"
	}
	appRoot, svcRoot := "apps/", "apps/"
	if ir.Type == core.StructMicroservices {
		svcRoot = "services/"
	}
	fix := func(ps []core.StructurePart, root string) []core.StructurePart {
		var out []core.StructurePart
		for _, p := range ps {
			p.Name = Slug(p.Name)
			if p.Name == "" || slices.ContainsFunc(out, func(x core.StructurePart) bool { return x.Name == p.Name }) {
				continue
			}
			p.Path = root + p.Name + "/"
			out = append(out, p)
		}
		return out
	}
	ir.Apps = fix(ir.Apps, appRoot)
	ir.Services = fix(ir.Services, svcRoot)
	ir.Workers = fix(ir.Workers, "apps/")
	ir.Packages = fix(ir.Packages, "packages/")
	seen := map[string]bool{}
	for i := range ir.Apps {
		ir.Apps[i].Modules = nil
	}
	for i := range ir.Services {
		var mods []string
		for _, m := range ir.Services[i].Modules {
			if m = Slug(m); m != "" && !seen[m] {
				seen[m] = true
				mods = append(mods, m)
			}
		}
		ir.Services[i].Modules = mods
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
		infix := ""
		if generic(p.Name) {
			infix = "modules/"
		}
		for _, m := range p.Modules {
			out = append(out, p.Path+infix+m+"/")
		}
	}
	for _, ps := range [][]core.StructurePart{ir.Workers, ir.Packages} {
		for _, p := range ps {
			out = append(out, p.Path)
		}
	}
	return append(out, ir.Docs)
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

// generic services hold modules but are not a domain themselves.
func generic(name string) bool { return name == "api" || name == "backend" }

// domainsIn are the domains of an IR in order: domain services and modules.
func domainsIn(ir *core.StructureIR) []string {
	var out []string
	add := func(d string) {
		if d != "" && !slices.Contains(out, d) {
			out = append(out, d)
		}
	}
	for _, s := range ir.Services {
		if !generic(s.Name) {
			add(s.Name)
		}
		for _, m := range s.Modules {
			add(m)
		}
	}
	return out
}

// Convert changes the type and regroups the same domains for it: modules
// of one backend, or one service per domain. Apps, workers and packages
// stay; a monorepo gets the shared packages it needs.
func Convert(ir *core.StructureIR, t string, p *core.Project) *core.StructureIR {
	out := ir.Clone()
	out.Type = t
	if len(ir.Services) > 0 && !(t == core.StructMicroservices && ir.Type == core.StructMicroservices) {
		out.Services = servicesFor(t, domainsIn(ir))
	}
	if t == core.StructSimple {
		out.Packages = nil
	} else if len(out.Packages) == 0 {
		out.Packages = packagesFor(out, p)
	}
	Normalize(out)
	return out
}

var errUnknownKind = errors.New("jenis bagian tidak dikenal (app, service, module, worker, package, rule)")
