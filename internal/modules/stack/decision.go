package stack

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/handrawanw/foundry/internal/core"
)

// The technology decision is reviewed, not just shown: a change becomes a
// proposal with its validation and impact, the user applies or cancels it,
// every applied change is a new version, and approval needs no errors.

// Change is one requested change: an empty Value returns the area to the
// recommendation.
type Change struct {
	Area   string `json:"area"`
	Value  string `json:"value"`
	Reason string `json:"reason,omitempty"`
}

var (
	ErrNoProposal = errors.New("tidak ada perubahan teknologi yang menunggu")
	ErrNoChange   = errors.New("tidak ada perubahan: pilihan sama dengan yang sekarang")
	ErrPending    = errors.New("terapkan atau batalkan perubahan teknologi yang menunggu dulu")
)

const maxHistory = 50

// Propose validates changes and records them as a proposal together with
// the issues and impact the stack would have. It merges with a waiting
// proposal; nothing in the decisions changes until Apply.
func Propose(s *core.TechStack, changes []Change, source, request, note string) error {
	if s == nil {
		return errors.New("teknologi belum direkomendasikan")
	}
	if s.Mode == core.StructureExisting {
		return ErrExisting
	}
	if len(changes) == 0 {
		return ErrNoChange
	}
	want := map[string]Change{}
	var order []string
	if s.Proposal != nil {
		for _, c := range s.Proposal.Changes {
			want[c.Area] = Change{Area: c.Area, Value: c.To, Reason: c.Reason}
			order = append(order, c.Area)
		}
	}
	for _, c := range changes {
		c.Area = strings.TrimSpace(c.Area)
		if _, ok := want[c.Area]; !ok {
			order = append(order, c.Area)
		}
		want[c.Area] = c
	}
	next := &core.TechStack{Mode: s.Mode, Needs: s.Needs, Platform: s.Platform, Decisions: core.CloneDecisions(s.Decisions)}
	for _, a := range order {
		c := want[a]
		if err := Override(next, c.Area, c.Value, c.Reason); err != nil {
			return err
		}
	}
	diff := Diff(s.Decisions, next.Decisions)
	if len(diff) == 0 {
		if s.Proposal != nil {
			Discard(s)
		}
		return ErrNoChange
	}
	prev := s.State()
	if s.Proposal != nil {
		prev = s.Proposal.Previous
	}
	s.Proposal = &core.TechProposal{Source: source, Request: request, Note: note, Changes: diff,
		Issues: Validate(next), Impact: Impact(next, diff), Previous: prev, At: time.Now().UTC()}
	s.Status, s.Approved = core.TechReview, false
	return nil
}

// Apply makes the waiting proposal the decision and saves it as a new
// version. Errors do not stop it (the user decides); they block approval.
func Apply(s *core.TechStack) (*core.TechVersion, error) {
	if s == nil || s.Proposal == nil {
		return nil, ErrNoProposal
	}
	pr := s.Proposal
	before := core.CloneDecisions(s.Decisions)
	for _, c := range pr.Changes {
		value := c.To
		if d := s.Decision(c.Area); d != nil && value == d.Recommended {
			value = ""
		}
		if err := Override(s, c.Area, value, c.Reason); err != nil {
			return nil, err
		}
	}
	s.Proposal = nil
	st := core.TechImpactChecked
	if pr.Previous == core.TechApproved || pr.Previous == core.TechChanged {
		st = core.TechChanged
	}
	s.SetStatus(st)
	Refresh(s)
	return Record(s, before, pr.Source, pr.Request), nil
}

// Discard drops the waiting proposal; the decisions never changed.
func Discard(s *core.TechStack) error {
	if s == nil || s.Proposal == nil {
		return ErrNoProposal
	}
	s.SetStatus(s.Proposal.Previous)
	s.Proposal = nil
	return nil
}

// Approve accepts the current decision. A waiting proposal or an error
// finding blocks it.
func Approve(s *core.TechStack) error {
	if s == nil {
		return errors.New("teknologi belum direkomendasikan")
	}
	if s.Proposal != nil {
		return ErrPending
	}
	Refresh(s)
	if errs := s.Errors(); len(errs) > 0 {
		return fmt.Errorf("%d masalah teknologi harus diselesaikan dulu: %s", len(errs), errs[0].Message)
	}
	if len(s.History) == 0 {
		Record(s, nil, "recommend", "")
	}
	s.SetStatus(core.TechApproved)
	return nil
}

// Record saves the current decisions as a new version. before is the state
// they changed from (nil for the first version); a stack saved before
// versions existed gets its old state as version 1 first.
func Record(s *core.TechStack, before []core.TechDecision, source, request string) *core.TechVersion {
	now := time.Now().UTC()
	if len(s.History) == 0 && before != nil {
		s.History = append(s.History, core.TechVersion{Version: 1, At: now, Source: "recommend",
			Status: core.TechRecommended, Decisions: before})
	}
	v := core.TechVersion{Version: s.Version() + 1, At: now, Source: source, Request: request,
		Status: s.State(), Decisions: core.CloneDecisions(s.Decisions)}
	if before != nil {
		v.Changes = Diff(before, s.Decisions)
		v.Impact = Impact(s, v.Changes)
		v.Design = slices.ContainsFunc(v.Impact, func(i core.TechImpact) bool { return i.Design })
	}
	s.History = append(s.History, v)
	if n := len(s.History); n > maxHistory {
		s.History = slices.Delete(s.History, 1, n-maxHistory+1) // keep v1 as the baseline
	}
	return &s.History[len(s.History)-1]
}

// Diff lists the areas whose chosen technology differs from before to
// after; an area that appears or disappears counts as a change.
func Diff(before, after []core.TechDecision) []core.TechChange {
	old := map[string]core.TechDecision{}
	for _, d := range before {
		old[d.Area] = d
	}
	var out []core.TechChange
	seen := map[string]bool{}
	for _, d := range after {
		seen[d.Area] = true
		o, ok := old[d.Area]
		if ok && o.Choice() == d.Choice() {
			continue
		}
		c := core.TechChange{Area: d.Area, To: d.Choice(), Language: d.Lang()}
		if ok {
			c.From = o.Choice()
		}
		if d.Override != nil {
			c.Reason = d.Override.Reason
		}
		out = append(out, c)
	}
	for _, d := range before {
		if !seen[d.Area] {
			out = append(out, core.TechChange{Area: d.Area, From: d.Choice()})
		}
	}
	return out
}

// affects is what each area's technology reaches into.
var affects = map[string][]string{
	core.TechFrontend:      {"kode frontend", "komponen UI dan handoff rancangan", "testing frontend", "deployment frontend"},
	core.TechMobile:        {"aplikasi mobile", "handoff rancangan mobile", "testing mobile", "rilis ke store"},
	core.TechDesktop:       {"aplikasi desktop", "handoff rancangan desktop", "testing desktop", "distribusi installer"},
	core.TechBackend:       {"implementasi API", "struktur backend", "authentication dan authorization", "implementasi worker", "testing backend", "deployment", "observability"},
	core.TechDatabase:      {"model data dan migrasi", "query dan repository layer", "transaksi", "backup dan deployment", "testing data"},
	core.TechCache:         {"performance", "session dan rate limit", "state realtime", "deployment"},
	core.TechMessaging:     {"komunikasi antar proses", "worker", "realtime", "deployment", "observability"},
	core.TechWorkers:       {"job latar belakang", "antrean", "deployment", "observability"},
	core.TechStorage:       {"upload dan download file", "security file (signed URL)", "deployment", "backup"},
	core.TechSearch:        {"index pencarian", "sinkronisasi data", "API pencarian", "deployment"},
	core.TechArchitecture:  {"service boundary", "struktur repository", "deployment strategy", "komunikasi antar modul", "testing integrasi"},
	core.TechRepository:    {"struktur repository", "paket bersama", "CI/CD", "deployment"},
	core.TechDeployment:    {"infrastruktur", "CI/CD", "scalability", "security jaringan", "biaya operasional"},
	core.TechObservability: {"logging", "metrics dan tracing", "alerting", "deployment"},
}

// Impact is what each change affects in s (the stack after the change).
func Impact(s *core.TechStack, changes []core.TechChange) []core.TechImpact {
	var out []core.TechImpact
	for _, c := range changes {
		a := append([]string(nil), affects[c.Area]...)
		if c.Area == core.TechFrontend || c.Area == core.TechMobile || c.Area == core.TechBackend {
			if s.Choice(core.TechRepository) == core.RepoMonorepo {
				a = append(a, "paket bersama (packages/ui, packages/types)")
			}
		}
		if c.Area == core.TechArchitecture || c.Area == core.TechRepository {
			a = append(a, "struktur folder project dibuat ulang dari keputusan ini")
		}
		out = append(out, core.TechImpact{Area: c.Area, Affects: a, Design: core.AffectsDesign(c.Area)})
	}
	return out
}

// Refresh re-validates the current decisions.
func Refresh(s *core.TechStack) {
	if s != nil {
		s.Issues = Validate(s)
	}
}

func none(v string) bool { return v == "" || v == core.TechNone }

// Validate checks the stack against its needs and itself. Only real
// incompatibilities are errors; risks and gaps are warnings.
func Validate(s *core.TechStack) []core.TechIssue {
	if s == nil || s.Mode == core.StructureExisting {
		return nil
	}
	var out []core.TechIssue
	add := func(level, area, msg string, args ...any) {
		out = append(out, core.TechIssue{Level: level, Area: area, Message: fmt.Sprintf(msg, args...)})
	}
	has := func(n string) bool { return slices.Contains(s.Needs, n) }
	ch := s.Choice
	lang := func(area string) string {
		if d := s.Decision(area); d != nil {
			return d.Lang()
		}
		return ""
	}
	present := func(area string) bool { return s.Decision(area) != nil }

	for _, d := range s.Decisions {
		if v := d.Choice(); !none(v) && option(d.Area, v) == nil {
			add("warning", d.Area, "%s belum dikenal Foundry; kecocokannya dengan pilihan lain tidak diperiksa otomatis.", v)
		}
	}
	// Errors: combinations that cannot work as chosen.
	if v := ch(core.TechSearch); v == "PostgreSQL full-text search" && ch(core.TechDatabase) != "PostgreSQL" {
		add("error", core.TechSearch, "PostgreSQL full-text search butuh database PostgreSQL, sedangkan database %s.", ch(core.TechDatabase))
	}
	if ch(core.TechDeployment) == "Static hosting + CDN" {
		if s.Platform.Backend {
			add("error", core.TechDeployment, "Static hosting tidak bisa menjalankan backend %s; pilih server/container.", ch(core.TechBackend))
		} else if strings.Contains(ch(core.TechFrontend), "Next.js") {
			add("warning", core.TechDeployment, "Next.js dengan render server tidak jalan di static hosting; pakai static export atau Node server.")
		}
	}
	if ch(core.TechWorkers) == "Worker terpisah + antrean" && none(ch(core.TechCache)) && none(ch(core.TechMessaging)) {
		add("error", core.TechWorkers, "Worker terpisah butuh antrean, tapi cache dan messaging “Tidak perlu”; pilih Redis atau messaging, atau ubah worker ke Job in-process.")
	}
	if present(core.TechBackend) && none(ch(core.TechDatabase)) {
		add("warning", core.TechDatabase, "Backend tanpa database: data tidak tersimpan permanen.")
	}
	// Needs that the stack does not cover.
	if has("realtime") && present(core.TechCache) && none(ch(core.TechCache)) && none(ch(core.TechMessaging)) {
		add("warning", core.TechCache, "Kebutuhan realtime butuh pub/sub atau state bersama (mis. Redis); cache dan messaging “Tidak perlu”.")
	}
	if has("high_concurrency") && slices.Contains([]string{"TypeScript", "Python", "PHP", "Ruby"}, lang(core.TechBackend)) {
		add("warning", core.TechBackend, "Kebutuhan konkurensi tinggi; backend %s (%s) perlu scaling horizontal dan benchmark. Go, Rust atau Java lebih aman.", ch(core.TechBackend), lang(core.TechBackend))
	}
	if has("transactions") && strings.Contains(strings.ToLower(ch(core.TechDatabase)), "mongo") {
		add("warning", core.TechDatabase, "Ada transaksi keuangan; MongoDB mendukung transaksi ACID multi-dokumen dengan batasan. PostgreSQL lebih aman untuk ledger.")
	}
	if has("seo") && strings.Contains(ch(core.TechFrontend), "Vite") {
		add("warning", core.TechFrontend, "Halaman publik butuh SEO, tapi %s dirender di browser; pilih Next.js atau tambahkan prerender.", ch(core.TechFrontend))
	}
	if has("files") && present(core.TechStorage) && none(ch(core.TechStorage)) {
		add("warning", core.TechStorage, "Ada kebutuhan upload file, tapi file storage “Tidak perlu”.")
	}
	if has("search") && present(core.TechSearch) && none(ch(core.TechSearch)) {
		add("warning", core.TechSearch, "Ada kebutuhan pencarian teks, tapi search “Tidak perlu”; query LIKE lambat pada data besar.")
	}
	if has("ai") && present(core.TechWorkers) && ch(core.TechWorkers) != "Worker terpisah + antrean" {
		add("warning", core.TechWorkers, "Panggilan AI bisa puluhan detik; tanpa worker terpisah request HTTP tertahan.")
	}
	if (has("enterprise") || has("high_concurrency")) && present(core.TechObservability) && ch(core.TechObservability) == "Log terstruktur + health check" {
		add("warning", core.TechObservability, "Kebutuhan audit/beban tinggi butuh metrics dan tracing terpusat, bukan hanya log.")
	}
	if has("offline") && s.Platform.Web && !s.Platform.Mobile {
		add("warning", core.TechFrontend, "Kebutuhan offline di web butuh PWA (service worker + penyimpanan lokal); rencanakan sejak awal.")
	}
	// Architecture, repository and deployment consistency.
	arch, repo := ch(core.TechArchitecture), ch(core.TechRepository)
	if arch == core.ArchMicroservices {
		if present(core.TechMessaging) && none(ch(core.TechMessaging)) {
			add("warning", core.TechMessaging, "Microservices tanpa messaging: layanan saling memanggil langsung dan ikut gagal bersama.")
		}
		if !has("independent_scaling") {
			add("warning", core.TechArchitecture, "Microservices tanpa kebutuhan skala/deploy independen menambah biaya operasional; Modular Monolith biasanya cukup.")
		}
	}
	if ch(core.TechDeployment) == "Kubernetes" && arch != core.ArchMicroservices {
		add("warning", core.TechDeployment, "Kubernetes untuk %s berlebihan; Docker Compose atau PaaS lebih sederhana.", arch)
	}
	if repo == core.RepoMonorepo {
		if l := lang(core.TechBackend); l != "" && l != "TypeScript" && (s.Platform.Web || s.Platform.Mobile) {
			add("warning", core.TechRepository, "packages/types (TypeScript) tidak bisa dipakai backend %s; pakai kontrak API (OpenAPI) dan generate type.", l)
		}
		if lang(core.TechMobile) == "Dart" && s.Platform.Web {
			add("warning", core.TechRepository, "packages/ui berbasis web tidak bisa dipakai Flutter; UI mobile dibuat terpisah.")
		}
	}
	return out
}
