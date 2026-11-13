package structure

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/handrawanwdev/foundry/internal/core"
)

// The structure decision is reviewed like the technology: an edit becomes
// a proposal with its validation and impact, the user applies or cancels
// it, every applied change is a version, and approval needs no errors.

var (
	ErrNoProposal = errors.New("tidak ada perubahan struktur yang menunggu")
	ErrNoChange   = errors.New("tidak ada perubahan: struktur sama dengan yang sekarang")
	ErrPending    = errors.New("terapkan atau batalkan perubahan struktur yang menunggu dulu")
	ErrExisting   = errors.New("project existing: struktur mengikuti kode yang ada")
)

const maxHistory = 50

// ---------- edits ----------

func list(ir *core.StructureIR, kind string) (*[]core.StructurePart, error) {
	switch kind {
	case core.PartApp:
		return &ir.Apps, nil
	case core.PartService:
		return &ir.Services, nil
	case core.PartWorker:
		return &ir.Workers, nil
	case core.PartPackage:
		return &ir.Packages, nil
	}
	return nil, errUnknownKind
}

func find(ps []core.StructurePart, name string) int {
	return slices.IndexFunc(ps, func(p core.StructurePart) bool { return p.Name == name })
}

// owner is the index of the service holding module m, or -1.
func owner(ir *core.StructureIR, m string) int {
	return slices.IndexFunc(ir.Services, func(s core.StructurePart) bool { return slices.Contains(s.Modules, m) })
}

// ApplyOp edits ir in place.
func ApplyOp(ir *core.StructureIR, op core.StructureOp, p *core.Project) error {
	name, to := Slug(op.Name), Slug(op.To)
	kind := strings.ToLower(strings.TrimSpace(op.Kind))
	dir := ""
	if strings.TrimSpace(op.Path) != "" {
		if dir = CleanDir(op.Path); dir == "" {
			return fmt.Errorf("folder %q tidak valid: harus folder relatif di dalam repository", op.Path)
		}
	}
	if kind == core.PartRule {
		if err := applyRule(ir, op); err != nil {
			return err
		}
		Normalize(ir)
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(op.Op)) {
	case "type":
		t := strings.Join(strings.Fields(op.Name), " ")
		if t == "" {
			return errors.New("nama gaya arsitektur kosong")
		}
		ir.Type = t
	case "add":
		if name == "" {
			return errors.New("nama bagian kosong")
		}
		if kind == core.PartModule {
			if owner(ir, name) >= 0 {
				return fmt.Errorf("modul %s sudah ada", name)
			}
			i := find(ir.Services, to)
			if to == "" && len(ir.Services) == 1 {
				i = 0
			}
			if i < 0 {
				return fmt.Errorf("service tujuan modul %s tidak ditemukan", name)
			}
			svc := &ir.Services[i]
			if dir == "" {
				dir = modulePlace(*svc, name)
			}
			svc.Modules = append(svc.Modules, name)
			setModule(svc, name, dir, strings.TrimSpace(op.Reason))
			break
		}
		ps, err := list(ir, kind)
		if err != nil {
			return err
		}
		if find(*ps, name) >= 0 {
			return fmt.Errorf("%s %s sudah ada", kind, name)
		}
		if dir == "" {
			dir = placeFor(name, *ps, ir.Apps, ir.Services, ir.Workers, ir.Packages)
		}
		*ps = append(*ps, core.StructurePart{Name: name, Path: dir, Reason: strings.TrimSpace(op.Reason)})
	case "path":
		if dir == "" {
			return errors.New("folder baru kosong")
		}
		if kind == core.PartModule {
			i := owner(ir, name)
			if i < 0 {
				return fmt.Errorf("modul %s tidak ditemukan", name)
			}
			setModule(&ir.Services[i], name, dir, "")
			break
		}
		ps, err := list(ir, kind)
		if err != nil {
			return err
		}
		i := find(*ps, name)
		if i < 0 {
			return fmt.Errorf("%s %s tidak ditemukan", kind, name)
		}
		part := &(*ps)[i]
		for m, d := range part.ModulePaths {
			part.ModulePaths[m] = rebase(d, part.Path, dir)
		}
		part.Path = dir
	case "remove":
		if kind == core.PartModule {
			i := owner(ir, name)
			if i < 0 {
				return fmt.Errorf("modul %s tidak ditemukan", name)
			}
			ir.Services[i].Modules = slices.DeleteFunc(ir.Services[i].Modules, func(m string) bool { return m == name })
			delete(ir.Services[i].ModuleReasons, name)
			delete(ir.Services[i].ModulePaths, name)
			break
		}
		ps, err := list(ir, kind)
		if err != nil {
			return err
		}
		i := find(*ps, name)
		if i < 0 {
			return fmt.Errorf("%s %s tidak ditemukan", kind, name)
		}
		*ps = slices.Delete(*ps, i, i+1)
	case "rename":
		if to == "" {
			return errors.New("nama baru kosong")
		}
		if kind == core.PartModule {
			i := owner(ir, name)
			if i < 0 {
				return fmt.Errorf("modul %s tidak ditemukan", name)
			}
			if owner(ir, to) >= 0 {
				return fmt.Errorf("modul %s sudah ada", to)
			}
			svc := &ir.Services[i]
			svc.Modules[slices.Index(svc.Modules, name)] = to
			setModule(svc, to, renamed(ModulePath(*svc, name), name, to), svc.ModuleReasons[name])
			delete(svc.ModuleReasons, name)
			delete(svc.ModulePaths, name)
			break
		}
		ps, err := list(ir, kind)
		if err != nil {
			return err
		}
		i := find(*ps, name)
		if i < 0 {
			return fmt.Errorf("%s %s tidak ditemukan", kind, name)
		}
		if find(*ps, to) >= 0 {
			return fmt.Errorf("%s %s sudah ada", kind, to)
		}
		part := &(*ps)[i]
		next := renamed(part.Path, name, to)
		for m, d := range part.ModulePaths {
			part.ModulePaths[m] = rebase(d, part.Path, next)
		}
		part.Name, part.Path = to, next
	case "move":
		dst := find(ir.Services, to)
		if dst < 0 {
			return fmt.Errorf("service tujuan %s tidak ditemukan", op.To)
		}
		switch kind {
		case core.PartModule:
			src := owner(ir, name)
			if src < 0 {
				return fmt.Errorf("modul %s tidak ditemukan", name)
			}
			reason := ir.Services[src].ModuleReasons[name]
			ir.Services[src].Modules = slices.DeleteFunc(ir.Services[src].Modules, func(m string) bool { return m == name })
			delete(ir.Services[src].ModuleReasons, name)
			delete(ir.Services[src].ModulePaths, name)
			d := &ir.Services[dst]
			if dir == "" {
				dir = modulePlace(*d, name)
			}
			d.Modules = append(d.Modules, name)
			setModule(d, name, dir, reason)
		case core.PartService:
			src := find(ir.Services, name)
			if src < 0 || src == dst {
				return fmt.Errorf("service %s tidak ditemukan atau sama dengan tujuan", name)
			}
			// A service merged into another becomes its module(s).
			svc := ir.Services[src]
			mods := svc.Modules
			reasons := moduleReasons(ir)
			if domainService(svc) {
				mods = []string{svc.Name}
				if svc.Reason != "" {
					reasons[svc.Name] = svc.Reason
				}
			}
			d := &ir.Services[dst]
			if domainService(*d) {
				// the target keeps its own domain as a module
				d.Modules = append(d.Modules, d.Name)
				setModule(d, d.Name, d.Path, d.Reason)
			}
			for _, m := range mods {
				d.Modules = append(d.Modules, m)
				setModule(d, m, modulePlace(*d, m), reasons[m])
			}
			ir.Services = slices.Delete(ir.Services, src, src+1)
		default:
			return errors.New("yang bisa dipindah hanya module atau service")
		}
	default:
		return fmt.Errorf("operasi %q tidak dikenal (type, add, path, remove, rename, move)", op.Op)
	}
	Normalize(ir)
	return nil
}

// setModule records the folder and reason of module m in svc; an empty
// reason keeps the current one.
func setModule(svc *core.StructurePart, m, dir, reason string) {
	if svc.ModulePaths == nil {
		svc.ModulePaths = map[string]string{}
	}
	svc.ModulePaths[m] = dir
	if reason != "" {
		if svc.ModuleReasons == nil {
			svc.ModuleReasons = map[string]string{}
		}
		svc.ModuleReasons[m] = reason
	}
}

// applyRule adds, removes or rewrites a structure rule; Name is its text.
func applyRule(ir *core.StructureIR, op core.StructureOp) error {
	text := strings.Join(strings.Fields(op.Name), " ")
	if text == "" {
		return errors.New("teks aturan kosong")
	}
	i := ruleIndex(ir.Rules, text)
	switch strings.ToLower(strings.TrimSpace(op.Op)) {
	case "add":
		if i >= 0 {
			return fmt.Errorf("aturan “%s” sudah ada", text)
		}
		if len(ir.Rules) >= maxRules {
			return fmt.Errorf("aturan maksimal %d", maxRules)
		}
		ir.Rules = append(ir.Rules, text)
	case "remove":
		if i < 0 {
			return fmt.Errorf("aturan “%s” tidak ditemukan", text)
		}
		ir.Rules = slices.Delete(ir.Rules, i, i+1)
	case "rename":
		to := strings.Join(strings.Fields(op.To), " ")
		if i < 0 {
			return fmt.Errorf("aturan “%s” tidak ditemukan", text)
		}
		if to == "" {
			return errors.New("teks aturan baru kosong")
		}
		ir.Rules[i] = to
	default:
		return errors.New("aturan hanya bisa ditambah, diubah (rename) atau dihapus")
	}
	return nil
}

// ---------- diff, validation, impact ----------

// where tells where a domain lives: its own service, or a module folder.
func where(ir *core.StructureIR) map[string]string {
	out := map[string]string{}
	for _, s := range ir.Services {
		if domainService(s) {
			out[s.Name] = "service " + s.Path
		}
		for _, m := range s.Modules {
			out[m] = "modul " + ModulePath(s, m)
		}
	}
	return out
}

// Diff describes the change from a to b in words.
func Diff(a, b *core.StructureIR) []string {
	if a == nil {
		a = &core.StructureIR{}
	}
	var out []string
	if a.Type != b.Type {
		out = append(out, fmt.Sprintf("Gaya arsitektur: %s → %s", orDash(a.Type), orDash(b.Type)))
	}
	parts := func(label string, x, y []core.StructurePart, skip func(core.StructurePart) bool) {
		for _, p := range y {
			i := find(x, p.Name)
			switch {
			case skip(p):
			case i < 0:
				out = append(out, fmt.Sprintf("%s ditambah: %s", label, p.Path))
			case x[i].Path != p.Path:
				out = append(out, fmt.Sprintf("%s %s dipindah: %s → %s", label, p.Name, x[i].Path, p.Path))
			}
		}
		for _, p := range x {
			if find(y, p.Name) < 0 && !skip(p) {
				out = append(out, fmt.Sprintf("%s dihapus: %s", label, p.Path))
			}
		}
	}
	none := func(core.StructurePart) bool { return false }
	parts("App", a.Apps, b.Apps, none)
	parts("Service", a.Services, b.Services, domainService)
	wa, wb := where(a), where(b)
	for _, d := range domainsIn(b) {
		switch from, ok := wa[d]; {
		case !ok:
			out = append(out, fmt.Sprintf("Domain %s ditambah (%s)", d, wb[d]))
		case from != wb[d]:
			out = append(out, fmt.Sprintf("Domain %s: %s → %s", d, from, wb[d]))
		}
	}
	for _, d := range domainsIn(a) {
		if _, ok := wb[d]; !ok {
			out = append(out, fmt.Sprintf("Domain %s dihapus (%s)", d, wa[d]))
		}
	}
	parts("Worker", a.Workers, b.Workers, none)
	parts("Package", a.Packages, b.Packages, none)
	if a.Docs != b.Docs {
		out = append(out, fmt.Sprintf("Folder dokumentasi: %s → %s", orDash(a.Docs), orDash(b.Docs)))
	}
	for _, r := range b.Rules {
		if ruleIndex(a.Rules, r) < 0 {
			out = append(out, "Aturan ditambah: "+r)
		}
	}
	for _, r := range a.Rules {
		if ruleIndex(b.Rules, r) < 0 {
			out = append(out, "Aturan dihapus: "+r)
		}
	}
	return out
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}

// Validate lists concrete problems of ir for the project p; an error
// blocks approval. It checks what holds for any architecture: a named
// style, one folder per part, the platform's parts present, every domain
// placed. Which style fits is the design's decision, not a rule here.
func Validate(ir *core.StructureIR, p *core.Project, st *core.TechStack) []core.TechIssue {
	var out []core.TechIssue
	add := func(level, area, f string, a ...any) {
		out = append(out, core.TechIssue{Level: level, Area: area, Message: fmt.Sprintf(f, a...)})
	}
	if strings.TrimSpace(ir.Type) == "" {
		add("error", "structure", "Gaya arsitektur belum dinamai.")
	}
	owners := map[string]string{}
	claim := func(dir, who string) {
		if dir == "" {
			add("error", "structure", "%s tidak punya folder yang valid.", who)
			return
		}
		if other, ok := owners[dir]; ok {
			add("error", "structure", "Folder %s dipakai %s dan %s; beri folder sendiri.", dir, other, who)
			return
		}
		owners[dir] = who
	}
	for _, g := range []struct {
		kind string
		ps   []core.StructurePart
	}{{core.PartApp, ir.Apps}, {core.PartService, ir.Services}, {core.PartWorker, ir.Workers}, {core.PartPackage, ir.Packages}} {
		for _, part := range g.ps {
			claim(part.Path, g.kind+" "+part.Name)
			for _, m := range part.Modules {
				claim(ModulePath(part, m), "modul "+m)
			}
		}
	}
	pl := platformOf(p)
	if (pl.web || pl.mobile || pl.desktop) && len(ir.Apps) == 0 {
		add("error", core.PartApp, "Platform punya aplikasi klien, tapi struktur tidak punya app.")
	}
	if pl.backend && len(ir.Services) == 0 {
		add("error", core.PartService, "Platform backend dipilih, tapi struktur tidak punya service.")
	}
	if !pl.backend && len(ir.Services) > 0 {
		add("warning", core.PartService, "Ada service, tapi platform backend tidak dipilih.")
	}
	for _, d := range domainsIn(ir) {
		if crudName(d) {
			add("warning", core.PartModule, "%s mengikuti operasi (CRUD), bukan domain; gabungkan ke domain %s.", d, domainOf(d))
		}
	}
	if missing := missingDomains(ir, Domains(p)); len(missing) > 0 && len(ir.Services) > 0 {
		add("warning", core.PartModule, "Domain %s belum punya tempat di struktur.", strings.Join(missing, ", "))
	}
	return out
}

// Impact is what the change from a to b reaches into.
func Impact(a, b *core.StructureIR, tech []core.TechChange) []core.StructureImpact {
	var out []core.StructureImpact
	add := func(area, msg string) {
		i := core.StructureImpact{Area: area, Message: msg}
		if !slices.Contains(out, i) {
			out = append(out, i)
		}
	}
	for _, c := range tech {
		add("Teknologi", fmt.Sprintf("%s: %s → %s (diterapkan bersama perubahan ini)", c.Area, c.From, c.To))
	}
	if a != nil && a.Type != b.Type {
		add("Arsitektur", fmt.Sprintf("Gaya arsitektur berubah (%s → %s): deployment, CI/CD, batas domain dan komunikasi antarbagian ikut ditinjau.", orDash(a.Type), orDash(b.Type)))
	}
	if a == nil {
		a = &core.StructureIR{}
	}
	wa, wb := where(a), where(b)
	for d, w := range wb {
		if from, ok := wa[d]; !ok || from != w {
			add("Domain boundaries", "Batas domain berubah; ownership kode dan data domain ikut pindah.")
			add("API", "Route dan kontrak domain yang berubah perlu disesuaikan.")
			add("Database", "Kepemilikan tabel per domain mengikuti batas baru.")
			add("Authorization", "Permission per domain mengikuti batas baru.")
			break
		}
	}
	if len(wa) != len(wb) {
		add("Domain boundaries", "Jumlah domain berubah.")
	}
	if len(diffNames(a.Services, b.Services)) > 0 {
		add("Deployment", "Jumlah unit deploy backend berubah.")
		add("CI/CD", "Pipeline build/test per service berubah.")
	}
	if len(diffNames(a.Apps, b.Apps)) > 0 {
		add("Design", "Halaman untuk app yang berubah perlu ditinjau.")
		add("Deployment", "Unit deploy aplikasi klien berubah.")
	}
	if len(diffNames(a.Workers, b.Workers)) > 0 {
		add("Worker", "Job latar belakang berpindah proses.")
		add("Deployment", "Proses worker di-deploy terpisah.")
	}
	if len(diffNames(a.Packages, b.Packages)) > 0 {
		add("Shared packages", "Kode bersama berubah: "+strings.Join(diffNames(a.Packages, b.Packages), ", ")+".")
		add("Dependency management", "Dependency antar app/package berubah.")
	}
	if !slices.Equal(Layout(a), Layout(b)) {
		add("Repository", "Folder project berubah; rencana struktur repository perlu mengikuti.")
	}
	if !slices.Equal(a.Rules, b.Rules) {
		add("Struktur internal", "Isi tiap modul (layer, file, dependensi, penamaan) mengikuti aturan struktur.")
		add("Code review", "Task build dicek terhadap aturan struktur.")
	}
	add("Build", "Folder scaffold dan task build mengikuti struktur ini.")
	return out
}

// diffNames are the names in only one of x and y.
func diffNames(x, y []core.StructurePart) []string {
	var out []string
	for _, p := range x {
		if find(y, p.Name) < 0 {
			out = append(out, p.Name)
		}
	}
	for _, p := range y {
		if find(x, p.Name) < 0 {
			out = append(out, p.Name)
		}
	}
	return out
}

// ---------- lifecycle ----------

func editable(s *core.Structure) error {
	switch {
	case s == nil:
		return errors.New("struktur belum ditentukan")
	case s.Mode == core.StructureExisting:
		return ErrExisting
	}
	return nil
}

// ErrNoStructure: hand edits need a structure; the AI analysis designs the
// first one.
var ErrNoStructure = errors.New("struktur belum dirancang: jalankan analisis struktur AI lalu pilih alternatifnya")

// Propose applies ops to the structure (or the waiting proposal) and
// records the result as a proposal; nothing changes until Apply.
func Propose(s *core.Structure, p *core.Project, ops []core.StructureOp, source, request, note string) error {
	if err := editable(s); err != nil {
		return err
	}
	if len(ops) == 0 {
		return ErrNoChange
	}
	base, prev := s.IR, []core.StructureOp(nil)
	if s.Proposal != nil {
		base, prev = s.Proposal.IR, s.Proposal.Ops
		if request == "" {
			request = s.Proposal.Request
		}
	}
	if base == nil {
		return ErrNoStructure
	}
	next := base.Clone()
	for _, op := range ops {
		if err := ApplyOp(next, op, p); err != nil {
			return err
		}
	}
	return ProposeIR(s, p, next, append(slices.Clone(prev), ops...), source, request, note)
}

// Usable splits ops into those that apply in order to the structure (or
// the waiting proposal) and why the others were skipped, so one wrong op
// from the AI does not drop the whole request.
func Usable(s *core.Structure, p *core.Project, ops []core.StructureOp) ([]core.StructureOp, []string) {
	if s == nil || s.IR == nil {
		return nil, nil
	}
	base := s.IR
	if s.Proposal != nil {
		base = s.Proposal.IR
	}
	next := base.Clone()
	var ok []core.StructureOp
	var skipped []string
	for _, op := range ops {
		try := next.Clone()
		if err := ApplyOp(try, op, p); err != nil {
			skipped = append(skipped, err.Error())
			continue
		}
		next, ok = try, append(ok, op)
	}
	return ok, skipped
}

// ProposeIR records ir as the proposal.
func ProposeIR(s *core.Structure, p *core.Project, ir *core.StructureIR, ops []core.StructureOp, source, request, note string) error {
	if err := editable(s); err != nil {
		return err
	}
	Normalize(ir)
	changes := Diff(s.IR, ir)
	if len(changes) == 0 {
		if s.Proposal != nil {
			_ = Discard(s)
		}
		return ErrNoChange
	}
	prev := s.State()
	if s.Proposal != nil {
		prev = s.Proposal.Previous
	}
	s.Proposal = &core.StructureProposal{Source: source, Request: request, Note: note, Ops: ops, IR: ir, Changes: changes,
		Issues: Validate(ir, p, p.Stack), Impact: Impact(s.IR, ir, nil), Previous: prev, At: time.Now().UTC()}
	s.Status, s.Approved = core.TechReview, false
	return nil
}

// Apply makes the proposal the structure and returns what it replaced and
// the proposal; the caller applies its technology changes, then Records.
func Apply(s *core.Structure) (*core.StructureIR, *core.StructureProposal, error) {
	if s == nil || s.Proposal == nil {
		return nil, nil, ErrNoProposal
	}
	pr, before := s.Proposal, s.IR
	set(s, pr.IR)
	s.Proposal = nil
	st := core.TechImpactChecked
	if pr.Previous == core.TechApproved || pr.Previous == core.TechChanged {
		st = core.TechChanged
	}
	s.SetStatus(st)
	return before, pr, nil
}

func set(s *core.Structure, ir *core.StructureIR) {
	s.IR = ir
	s.Template, s.Layout = ir.Type, Layout(ir)
}

func Discard(s *core.Structure) error {
	if s == nil || s.Proposal == nil {
		return ErrNoProposal
	}
	s.SetStatus(s.Proposal.Previous)
	s.Proposal = nil
	return nil
}

// Approve accepts the current structure; a proposal or an error blocks it.
func Approve(s *core.Structure, p *core.Project) error {
	if err := editable(s); err != nil {
		return err
	}
	if s.Proposal != nil {
		return ErrPending
	}
	if s.IR == nil {
		return ErrNoStructure
	}
	Refresh(s, p)
	if errs := s.Errors(); len(errs) > 0 {
		return fmt.Errorf("%d masalah struktur harus diselesaikan dulu: %s", len(errs), errs[0].Message)
	}
	if len(s.History) == 0 {
		Record(s, nil, "recommend", "")
	}
	s.SetStatus(core.TechApproved)
	return nil
}

// Refresh re-validates the structure against the project and technology,
// and marks the AI analysis stale when the system it read changed.
func Refresh(s *core.Structure, p *core.Project) {
	if s == nil {
		return
	}
	RefreshPlan(s, p)
	if s.IR != nil {
		s.Issues = Validate(s.IR, p, p.Stack)
	}
}

// Record saves the current IR as a version; before is what it changed from.
func Record(s *core.Structure, before *core.StructureIR, source, request string) *core.StructureVersion {
	now := time.Now().UTC()
	if len(s.History) == 0 && before != nil {
		s.History = append(s.History, core.StructureVersion{Version: 1, At: now, Source: "recommend", Status: core.TechRecommended, IR: *before.Clone()})
	}
	v := core.StructureVersion{Version: s.Version() + 1, At: now, Source: source, Request: request, Status: s.State(), IR: *s.IR.Clone()}
	if s.Plan != nil {
		v.Analysis, v.Tree = s.Plan.Summary, Tree(&v.IR)
		if a := s.Plan.SelectedAlternative(); a != nil {
			v.Alternative, v.Reason = a.ID, s.Plan.Decision
			if v.Reason == "" {
				v.Reason = a.Summary
			}
		}
	}
	if before != nil {
		v.Changes = Diff(before, s.IR)
		v.Impact = Impact(before, s.IR, nil)
	}
	s.History = append(s.History, v)
	if n := len(s.History); n > maxHistory {
		s.History = slices.Delete(s.History, 1, n-maxHistory+1)
	}
	return &s.History[len(s.History)-1]
}

// Sync keeps a new project's structure in step with the AI analysis: the
// recommended style is the recommended alternative's, and the decision is
// revalidated. Foundry never designs or rewrites a structure itself; the
// user adopts an alternative or edits the structure.
func Sync(p *core.Project, why string) {
	s := p.Structure
	if s == nil || s.Mode != core.StructureNew {
		return
	}
	if PlanCurrent(s, p) {
		if rec := s.Plan.RecommendedAlternative(); rec != nil && rec.IR != nil {
			s.Recommended, s.Reason = rec.IR.Type, s.Plan.Reason
		}
	}
	Refresh(s, p)
}
