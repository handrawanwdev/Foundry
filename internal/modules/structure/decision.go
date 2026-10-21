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
	if kind == core.PartRule {
		if err := applyRule(ir, op); err != nil {
			return err
		}
		Normalize(ir)
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(op.Op)) {
	case "type":
		t := strings.ToLower(strings.TrimSpace(op.Name))
		if !slices.Contains(core.StructureTypes, t) {
			return fmt.Errorf("tipe struktur %q tidak dikenal (simple, modular, monorepo, microservices)", op.Name)
		}
		*ir = *Convert(ir, t, p)
		return nil
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
			ir.Services[i].Modules = append(ir.Services[i].Modules, name)
			break
		}
		ps, err := list(ir, kind)
		if err != nil {
			return err
		}
		if find(*ps, name) >= 0 {
			return fmt.Errorf("%s %s sudah ada", kind, name)
		}
		*ps = append(*ps, core.StructurePart{Name: name, Reason: strings.TrimSpace(op.Reason)})
	case "remove":
		if kind == core.PartModule {
			i := owner(ir, name)
			if i < 0 {
				return fmt.Errorf("modul %s tidak ditemukan", name)
			}
			ir.Services[i].Modules = slices.DeleteFunc(ir.Services[i].Modules, func(m string) bool { return m == name })
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
			ms := ir.Services[i].Modules
			ms[slices.Index(ms, name)] = to
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
		(*ps)[i].Name = to
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
			ir.Services[src].Modules = slices.DeleteFunc(ir.Services[src].Modules, func(m string) bool { return m == name })
			ir.Services[dst].Modules = append(ir.Services[dst].Modules, name)
		case core.PartService:
			src := find(ir.Services, name)
			if src < 0 || src == dst {
				return fmt.Errorf("service %s tidak ditemukan atau sama dengan tujuan", name)
			}
			// A service merged into another becomes its module(s).
			svc := ir.Services[src]
			mods := svc.Modules
			if !generic(svc.Name) {
				mods = append([]string{svc.Name}, mods...)
			}
			d := &ir.Services[dst]
			if len(d.Modules) == 0 && !generic(d.Name) {
				d.Modules = append(d.Modules, d.Name) // the target keeps its own domain as a module
			}
			d.Modules = append(d.Modules, mods...)
			ir.Services = slices.Delete(ir.Services, src, src+1)
		default:
			return errors.New("yang bisa dipindah hanya module atau service")
		}
	default:
		return fmt.Errorf("operasi %q tidak dikenal (type, add, remove, rename, move)", op.Op)
	}
	Normalize(ir)
	return nil
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

var typeName = map[string]string{core.StructSimple: "Simple", core.StructModular: "Modular",
	core.StructMonorepo: "Monorepo", core.StructMicroservices: "Microservices"}

// where tells where a domain lives: its own service, or a module of one.
func where(ir *core.StructureIR) map[string]string {
	out := map[string]string{}
	for _, s := range ir.Services {
		if !generic(s.Name) {
			out[s.Name] = "service " + s.Path
		}
		for _, m := range s.Modules {
			out[m] = "modul di " + s.Path
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
		out = append(out, fmt.Sprintf("Tipe: %s → %s", typeName[a.Type], typeName[b.Type]))
	}
	parts := func(label string, x, y []core.StructurePart, skip func(string) bool) {
		for _, p := range y {
			if find(x, p.Name) < 0 && !skip(p.Name) {
				out = append(out, fmt.Sprintf("%s ditambah: %s", label, p.Path))
			}
		}
		for _, p := range x {
			if find(y, p.Name) < 0 && !skip(p.Name) {
				out = append(out, fmt.Sprintf("%s dihapus: %s", label, p.Path))
			}
		}
	}
	none := func(string) bool { return false }
	parts("App", a.Apps, b.Apps, none)
	parts("Service", a.Services, b.Services, func(n string) bool { return !generic(n) })
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

// TechFor are the architecture and repository changes structure type t
// needs to agree with the technology decision.
func TechFor(t string, s *core.TechStack) []core.TechChange {
	if s == nil || s.Mode == core.StructureExisting {
		return nil
	}
	arch, repo := s.Choice(core.TechArchitecture), s.Choice(core.TechRepository)
	wantArch, wantRepo := arch, repo
	switch t {
	case core.StructSimple, core.StructModular:
		wantArch = map[string]string{core.StructSimple: core.ArchSimple, core.StructModular: core.ArchModular}[t]
		if repo == core.RepoMonorepo {
			wantRepo = core.RepoSimple
		}
	case core.StructMonorepo:
		wantRepo = core.RepoMonorepo
		if arch == core.ArchMicroservices {
			wantArch = core.ArchModular
		}
	case core.StructMicroservices:
		wantArch = core.ArchMicroservices
	}
	var out []core.TechChange
	why := "Mengikuti struktur " + typeName[t] + "."
	if arch != wantArch {
		out = append(out, core.TechChange{Area: core.TechArchitecture, From: arch, To: wantArch, Reason: why})
	}
	if repo != wantRepo {
		out = append(out, core.TechChange{Area: core.TechRepository, From: repo, To: wantRepo, Reason: why})
	}
	return out
}

// withTech is s as it would be after the changes.
func withTech(s *core.TechStack, changes []core.TechChange) *core.TechStack {
	if s == nil || len(changes) == 0 {
		return s
	}
	c := *s
	c.Decisions = core.CloneDecisions(s.Decisions)
	for _, ch := range changes {
		if d := c.Decision(ch.Area); d != nil {
			d.Override = &core.TechOverride{Value: ch.To}
		}
	}
	return &c
}

// Validate lists concrete problems of ir for the project p with the
// technology st; an error blocks approval.
func Validate(ir *core.StructureIR, p *core.Project, st *core.TechStack) []core.TechIssue {
	var out []core.TechIssue
	add := func(level, area, f string, a ...any) {
		out = append(out, core.TechIssue{Level: level, Area: area, Message: fmt.Sprintf(f, a...)})
	}
	if !slices.Contains(core.StructureTypes, ir.Type) {
		add("error", "structure", "Tipe struktur %q tidak dikenal.", ir.Type)
	}
	paths := map[string]bool{}
	for _, l := range Layout(ir) {
		if paths[l] {
			add("error", "structure", "Folder %s dipakai dua bagian; ganti salah satu nama.", l)
		}
		paths[l] = true
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
	for _, c := range TechFor(ir.Type, st) {
		add("error", "technology", "Struktur %s tidak sesuai teknologi (%s: %s). Ubah struktur atau teknologinya; Foundry tidak mengubahnya diam-diam.",
			typeName[ir.Type], c.Area, c.From)
	}
	for _, d := range domainsIn(ir) {
		if crudName(d) {
			add("warning", core.PartModule, "%s mengikuti operasi (CRUD), bukan domain; gabungkan ke domain %s.", d, domainOf(d))
		}
	}
	for _, s := range ir.Services {
		if (ir.Type == core.StructModular || ir.Type == core.StructMonorepo) && generic(s.Name) && len(s.Modules) == 0 && len(Domains(p)) > 1 {
			add("warning", core.PartService, "%s tanpa modul; batas domain tidak terlihat di folder.", s.Path)
		}
	}
	if ir.Type == core.StructMicroservices {
		switch n := len(ir.Services); {
		case n == 1:
			add("warning", core.PartService, "Microservices dengan satu service sama dengan monolith; Modular lebih sederhana.")
		case n > 8:
			add("warning", core.PartService, "%d service: biaya deploy, observability dan koordinasi tinggi. Gabungkan domain yang selalu berubah bersama.", n)
		}
		if st != nil && !slices.Contains(st.Needs, "independent_scaling") {
			add("warning", "structure", "Tidak ada kebutuhan deploy/skala independen; Microservices menambah biaya operasional, Modular biasanya cukup.")
		}
	}
	if ir.Type == core.StructSimple && len(ir.Packages) > 0 {
		add("warning", core.PartPackage, "Simple tidak memakai packages/; pindahkan ke app atau naikkan ke Modular.")
	}
	for _, pk := range ir.Packages {
		if pk.Name == "ui" && len(ir.Apps) == 0 {
			add("warning", core.PartPackage, "packages/ui tanpa app frontend.")
		}
	}
	separate := st != nil && st.Choice(core.TechWorkers) == workersSeparate
	switch {
	case len(ir.Workers) > 0 && st != nil && !separate:
		add("warning", core.PartWorker, "Ada folder worker, tapi teknologi worker “%s”; ubah teknologi worker ke %s atau hapus worker.", st.Choice(core.TechWorkers), workersSeparate)
	case len(ir.Workers) == 0 && separate:
		add("warning", core.PartWorker, "Teknologi memilih %s, tapi struktur tidak punya worker.", workersSeparate)
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
		switch {
		case b.Type == core.StructMicroservices:
			add("Deployment", "Tiap service punya build, image dan pipeline deploy sendiri.")
			add("CI/CD", "Pipeline per service; rilis bisa independen.")
			add("Database", "Data dimiliki per service; join antar domain lewat API atau event.")
			add("API", "Panggilan antar domain jadi network call (timeout, retry, versi kontrak).")
			add("Authentication", "Token diverifikasi di tiap service.")
			add("Observability", "Butuh tracing lintas service.")
			add("Testing", "Contract test antar service.")
			add("Local development", "Semua service dijalankan bersama (mis. Docker Compose).")
			add("Team ownership", "Ownership per service.")
		case a.Type == core.StructMicroservices:
			add("Deployment", "Satu deploy backend; skala per domain tidak lagi independen.")
			add("Database", "Satu database; batas data per modul dijaga di kode.")
			add("API", "Panggilan antar domain jadi function call di dalam backend.")
			add("Observability", "Tracing lintas service tidak lagi diperlukan.")
		}
		switch {
		case b.Type == core.StructMonorepo:
			add("Repository", "Satu repo dengan workspace (apps/, services/, packages/).")
			add("CI/CD", "Pipeline per app dengan filter path.")
		case a.Type == core.StructMonorepo:
			add("Repository", "Workspace monorepo tidak dipakai lagi.")
			add("Shared packages", "packages/* tidak lagi diasumsikan shared; distribusi type/UI perlu registry atau codegen.")
		}
		if b.Type == core.StructSimple {
			add("Domain boundaries", "Batas modul hilang; kode backend dalam satu folder.")
		}
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
		add("Queue", "Worker terpisah butuh antrean (cache/messaging).")
		add("Deployment", "Proses worker di-deploy terpisah.")
	}
	if len(diffNames(a.Packages, b.Packages)) > 0 {
		add("Shared packages", "Kode bersama berubah: "+strings.Join(diffNames(a.Packages, b.Packages), ", ")+".")
		add("Dependency management", "Dependency workspace antar app/package berubah.")
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
	case s.IR == nil:
		return errors.New("struktur belum direkomendasikan")
	}
	return nil
}

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
	tech := TechFor(ir.Type, p.Stack)
	s.Proposal = &core.StructureProposal{Source: source, Request: request, Note: note, Ops: ops, IR: ir, Changes: changes,
		Tech: tech, Issues: Validate(ir, p, withTech(p.Stack, tech)), Impact: Impact(s.IR, ir, tech), Previous: prev, At: time.Now().UTC()}
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

// Refresh re-validates the structure against the project and technology.
func Refresh(s *core.Structure, p *core.Project) {
	if s != nil && s.IR != nil {
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

// Sync keeps a new project's structure in step with the requirement and
// technology. A recommendation the user has not touched follows them; a
// structure the user changed or approved is never rewritten, it only shows
// where it no longer fits.
func Sync(p *core.Project, why string) {
	s := p.Structure
	if s == nil || s.Mode != core.StructureNew {
		return
	}
	ir, reason := Recommend(p)
	s.Recommended = ir.Type
	follow := s.IR == nil && !s.Approved || s.IR != nil && s.State() == core.TechRecommended && s.Proposal == nil
	if follow {
		before := s.IR
		if before == nil || len(Diff(before, ir)) > 0 {
			set(s, ir)
			s.Reason = reason
			s.SetStatus(core.TechRecommended)
			if before == nil {
				s.History = nil
				Record(s, nil, "recommend", why)
			} else {
				Record(s, before, "technology", why)
			}
		}
	}
	Refresh(s, p)
}
