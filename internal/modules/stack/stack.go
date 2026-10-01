// Package stack decides the technology stack from the product's needs:
// platform, language, framework, database, architecture and repository
// layout. The decision is deterministic (needs → profile → rules) so the
// same requirement always gets the same recommendation; the user may
// override any area. Existing projects keep the stack read from their code.
package stack

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/handrawanw/foundry/internal/core"
)

//go:embed profiles/*.json
var profileFS embed.FS

// Pick is one area's default in a profile.
type Pick struct {
	Value  string `json:"value"`
	Reason string `json:"reason"`
}

// Profile is a technology profile: a proven stack for a kind of product.
type Profile struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Match       []string        `json:"match,omitempty"` // needs that select this profile
	Decisions   map[string]Pick `json:"decisions"`
}

// profileOrder is the selection priority: the first profile whose needs
// match wins; saas is the default.
var profileOrder = []string{"trading", "ai-product", "mobile-app", "enterprise", "saas"}

var Profiles = mustProfiles()

func mustProfiles() []Profile {
	var out []Profile
	for _, id := range profileOrder {
		b, err := profileFS.ReadFile("profiles/" + id + ".json")
		if err != nil {
			panic(err)
		}
		var p Profile
		if err := json.Unmarshal(b, &p); err != nil {
			panic(fmt.Sprintf("profile %s: %v", id, err))
		}
		out = append(out, p)
	}
	return out
}

func ProfileByID(id string) *Profile {
	for i := range Profiles {
		if Profiles[i].ID == id {
			return &Profiles[i]
		}
	}
	return nil
}

// Option is a technology Foundry knows for an area.
type Option struct {
	Value    string `json:"value"`
	Language string `json:"language,omitempty"`
	Fit      string `json:"fit"`
}

// Catalog lists the known options per area. Frontend, mobile, desktop,
// backend and database also accept a free value from the user; cache,
// architecture and repository must be one of these because the project
// structure is derived from them.
var Catalog = map[string][]Option{
	core.TechFrontend: {
		{"React + TypeScript + Vite", "TypeScript", "SaaS, dashboard, aplikasi bisnis, UI kompleks"},
		{"Next.js + TypeScript", "TypeScript", "website publik, SEO, aplikasi hybrid"},
		{"Vue + TypeScript + Vite", "TypeScript", "aplikasi bisnis sederhana, pengembangan cepat"},
	},
	core.TechMobile: {
		{"React Native + TypeScript + Expo", "TypeScript", "tim web yang sudah ada, aplikasi bisnis mobile"},
		{"Flutter", "Dart", "UI mobile berkualitas tinggi, konsisten lintas platform"},
	},
	core.TechDesktop: {
		{"Tauri + React + TypeScript", "TypeScript", "aplikasi desktop ringan dengan UI web"},
		{"Electron + React + TypeScript", "TypeScript", "ekosistem Node penuh di desktop"},
	},
	core.TechBackend: {
		{"NestJS + TypeScript", "TypeScript", "SaaS, API, realtime, iterasi cepat"},
		{"Go + Gin", "Go", "konkurensi tinggi, realtime, trading, infrastruktur"},
		{"Go + Fiber", "Go", "konkurensi tinggi dengan API bergaya Express"},
		{"Python + FastAPI", "Python", "AI, data processing, otomasi"},
	},
	core.TechDatabase: {
		{"PostgreSQL", "SQL", "aplikasi bisnis relasional, transaksi, enterprise"},
		{"MySQL", "SQL", "CRUD standar, ekosistem yang sudah ada"},
		{"MongoDB", "", "data berbentuk dokumen, skema fleksibel"},
	},
	core.TechCache: {
		{"Redis", "", "cache, state realtime, antrean"},
		{core.CacheNone, "", "tanpa cache terpisah"},
	},
	core.TechArchitecture: {
		{core.ArchSimple, "", "prototipe, aplikasi kecil"},
		{core.ArchModular, "", "aplikasi bisnis dengan beberapa domain"},
		{core.ArchMicroservices, "", "skala dan deployment independen, organisasi besar"},
	},
	core.TechRepository: {
		{core.RepoSimple, "", "satu frontend dan satu backend"},
		{core.RepoMonorepo, "", "web + mobile, paket bersama, banyak aplikasi"},
	},
}

var closedAreas = map[string]bool{core.TechCache: true, core.TechArchitecture: true, core.TechRepository: true}

func option(area, value string) *Option {
	for i, o := range Catalog[area] {
		if strings.EqualFold(o.Value, value) {
			return &Catalog[area][i]
		}
	}
	return nil
}

// ---------- analysis ----------

// Needs Foundry recognizes. The analyst may report them in
// discovery.tech_needs; keywords in the requirement add the rest.
var KnownNeeds = []string{"realtime", "offline", "seo", "ai", "high_concurrency", "flexible_schema", "transactions", "independent_scaling", "enterprise"}

var needWords = map[string][]string{
	"realtime":            {"realtime", "real time", "live", "websocket", "streaming", "chat", "notifikasi instan", "push notification"},
	"offline":             {"offline", "tanpa internet", "tanpa koneksi"},
	"seo":                 {"seo", "landing page", "website publik", "public website", "blog", "halaman publik", "marketing site"},
	"ai":                  {"ai", "llm", "gpt", "machine learning", "kecerdasan buatan", "chatbot", "embedding", "model bahasa"},
	"high_concurrency":    {"trading", "order book", "orderbook", "exchange", "bursa", "kripto", "crypto", "high frequency", "high concurrency", "konkurensi tinggi"},
	"flexible_schema":     {"skema fleksibel", "flexible schema", "form dinamis", "dynamic form", "field kustom", "custom field", "schemaless"},
	"transactions":        {"pembayaran", "payment", "transaksi", "invoice", "tagihan", "saldo", "akuntansi", "accounting", "ledger", "checkout"},
	"independent_scaling": {"microservice", "microservices", "skala independen", "independent scaling", "deploy terpisah", "separate deployment"},
	"enterprise":          {"sso", "audit", "rbac", "compliance", "kepatuhan", "ldap", "enterprise"},
}

// normalize lowercases text and turns punctuation into spaces, padded so
// " word " matches whole words only.
func normalize(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return ' '
	}, s)
	return " " + strings.Join(strings.Fields(s), " ") + " "
}

func requirementText(p *core.Project) string {
	parts := []string{p.Story}
	if d := p.Discovery; d != nil {
		parts = append(parts, d.Summary, d.Business)
		parts = append(parts, d.Workflows...)
		parts = append(parts, d.Integrations...)
		parts = append(parts, d.Security...)
		parts = append(parts, d.Data...)
	}
	for _, r := range p.Requirements {
		parts = append(parts, r.Text)
	}
	for _, f := range p.Features {
		parts = append(parts, f.Name, f.Description)
	}
	return normalize(strings.Join(parts, " "))
}

// Analyze returns the technical needs of the product, in KnownNeeds order.
func Analyze(p *core.Project) []string {
	found := map[string]bool{}
	if d := p.Discovery; d != nil {
		for _, n := range d.TechNeeds {
			found[strings.ToLower(strings.TrimSpace(n))] = true
		}
		if len(d.Roles) >= 4 {
			found["enterprise"] = true
		}
	}
	text := requirementText(p)
	for need, words := range needWords {
		for _, w := range words {
			if strings.Contains(text, " "+w+" ") {
				found[need] = true
				break
			}
		}
	}
	var out []string
	for _, n := range KnownNeeds {
		if found[n] {
			out = append(out, n)
		}
	}
	return out
}

func platformOf(p *core.Project) core.TechPlatform {
	var t core.TechPlatform
	if p.Platform == nil {
		return t
	}
	for _, x := range p.Platform.Targets {
		switch x.Kind {
		case core.PlatformWeb:
			t.Web = true
		case core.PlatformAndroid, core.PlatformIOS:
			t.Mobile = true
		case core.PlatformDesktop:
			t.Desktop = true
		case core.PlatformBackend:
			t.Backend = true
		}
	}
	return t
}

// ---------- recommendation ----------

// Recommend decides the stack for p. Overrides are not carried; use Carry.
func Recommend(p *core.Project) *core.TechStack {
	if p.Structure != nil && p.Structure.Mode == core.StructureExisting {
		return fromCode(p)
	}
	needs := Analyze(p)
	plat := platformOf(p)
	has := func(n string) bool { return slices.Contains(needs, n) }
	match := map[string]bool{}
	for _, n := range needs {
		match[n] = true
	}
	if plat.Mobile && !plat.Web && !plat.Desktop {
		match["mobile_only"] = true
	}
	prof := ProfileByID("saas")
	for i := range Profiles {
		if slices.ContainsFunc(Profiles[i].Match, func(m string) bool { return match[m] }) {
			prof = &Profiles[i]
			break
		}
	}
	s := &core.TechStack{Mode: core.StructureNew, Profile: prof.ID, Needs: needs, Platform: plat}
	add := func(area, value, reason string) {
		d := core.TechDecision{Area: area, Recommended: value, Reason: reason}
		if o := option(area, value); o != nil {
			d.Language = o.Language
		}
		s.Decisions = append(s.Decisions, d)
	}
	pick := func(area string) (string, string) { x := prof.Decisions[area]; return x.Value, x.Reason }

	if plat.Web {
		v, r := pick(core.TechFrontend)
		if has("seo") {
			v, r = "Next.js + TypeScript", "Halaman publik butuh SEO dan render di server."
		}
		add(core.TechFrontend, v, r)
	}
	if plat.Mobile {
		v, r := pick(core.TechMobile)
		add(core.TechMobile, v, r)
	}
	if plat.Desktop {
		v, r := pick(core.TechDesktop)
		add(core.TechDesktop, v, r)
	}
	if plat.Backend {
		v, r := pick(core.TechBackend)
		add(core.TechBackend, v, r)
		v, r = pick(core.TechDatabase)
		if has("flexible_schema") && !has("transactions") {
			v, r = "MongoDB", "Data berbentuk dokumen dengan skema yang sering berubah."
		}
		add(core.TechDatabase, v, r)
		v, r = pick(core.TechCache)
		if has("realtime") || has("high_concurrency") {
			v, r = "Redis", "State realtime, pub/sub, dan antrean."
		}
		add(core.TechCache, v, r)
	}
	v, r := pick(core.TechArchitecture)
	switch {
	case has("independent_scaling"):
		v, r = core.ArchMicroservices, "Butuh skala dan deployment independen per layanan."
	case len(p.Features) <= 3:
		v, r = core.ArchSimple, "Fiturnya sedikit; struktur sederhana cukup dan mudah dipelihara."
	}
	add(core.TechArchitecture, v, r)
	apps := 0
	for _, b := range []bool{plat.Web, plat.Mobile, plat.Desktop} {
		if b {
			apps++
		}
	}
	if apps >= 2 {
		add(core.TechRepository, core.RepoMonorepo, "Beberapa aplikasi berbagi tipe dan komponen UI dalam satu repo.")
	} else {
		add(core.TechRepository, core.RepoSimple, "Satu aplikasi klien dan backend cukup dipisah per folder.")
	}
	return s
}

// fromCode reads the stack of an existing project from its components.
func fromCode(p *core.Project) *core.TechStack {
	s := &core.TechStack{Mode: core.StructureExisting, Needs: Analyze(p), Platform: platformOf(p)}
	area := map[string]string{"web": core.TechFrontend, "mobile": core.TechMobile, "desktop": core.TechDesktop,
		"backend": core.TechBackend, "database": core.TechDatabase}
	byArea := map[string]int{}
	for _, c := range p.Structure.Components {
		a, ok := area[c.Kind]
		if !ok || c.Tech == "" {
			continue
		}
		switch a {
		case core.TechFrontend:
			s.Platform.Web = true
		case core.TechMobile:
			s.Platform.Mobile = true
		case core.TechDesktop:
			s.Platform.Desktop = true
		default:
			s.Platform.Backend = true
		}
		if i, ok := byArea[a]; ok {
			d := &s.Decisions[i]
			if !strings.Contains(d.Recommended, c.Tech) {
				d.Recommended += " + " + c.Tech
				d.Reason += fmt.Sprintf(", %s", c.Path)
			}
			continue
		}
		s.Decisions = append(s.Decisions, core.TechDecision{Area: a, Recommended: c.Tech, Reason: "Sudah dipakai di " + c.Path})
		byArea[a] = len(s.Decisions) - 1
	}
	s.Decisions = append(s.Decisions, core.TechDecision{Area: core.TechArchitecture, Recommended: "Struktur yang ada",
		Reason: "Project existing tidak diganti template."})
	slices.SortStableFunc(s.Decisions, func(a, b core.TechDecision) int {
		return slices.Index(core.TechAreas, a.Area) - slices.Index(core.TechAreas, b.Area)
	})
	return s
}

// Carry keeps the user's overrides from old in next for areas that still
// exist and are still valid.
func Carry(old, next *core.TechStack) {
	if old == nil || next == nil || next.Mode == core.StructureExisting {
		return
	}
	for _, d := range old.Decisions {
		if d.Override != nil {
			_ = Override(next, d.Area, d.Override.Value, d.Override.Reason)
		}
	}
}

var ErrExisting = errors.New("project existing: teknologi mengikuti kode yang ada")

// Override sets the user's choice for area; an empty value or the
// recommended value removes the override.
func Override(s *core.TechStack, area, value, reason string) error {
	if s == nil {
		return errors.New("teknologi belum direkomendasikan")
	}
	if s.Mode == core.StructureExisting {
		return ErrExisting
	}
	d := s.Decision(area)
	if d == nil {
		if slices.Contains(core.TechAreas, area) {
			return fmt.Errorf("%s tidak dipakai produk ini; ubah platform dulu", area)
		}
		return fmt.Errorf("area %q tidak dikenal", area)
	}
	value, reason = strings.TrimSpace(value), strings.TrimSpace(reason)
	if strings.ContainsAny(value, "\n\r") || len(value) > 80 || len(reason) > 200 {
		return errors.New("pilihan teknologi terlalu panjang")
	}
	o := option(area, value)
	if value != "" && o == nil && closedAreas[area] {
		return fmt.Errorf("%s harus salah satu dari: %s", area, strings.Join(values(area), ", "))
	}
	if o != nil {
		value = o.Value
	}
	if value == "" || value == d.Recommended {
		d.Override = nil
		return nil
	}
	d.Override = &core.TechOverride{Value: value, Reason: reason}
	if o != nil {
		d.Override.Language = o.Language
	}
	return nil
}

func values(area string) []string {
	var v []string
	for _, o := range Catalog[area] {
		v = append(v, o.Value)
	}
	return v
}

// ---------- project structure ----------

// Structure derives the template, folder layout and reason of a new project
// from the stack: architecture and repository decide the shape, features
// become backend modules.
func Structure(s *core.TechStack, features []core.Feature) (template string, layout []string, reason string) {
	arch, repo := s.Choice(core.TechArchitecture), s.Choice(core.TechRepository)
	pl := s.Platform
	mods := modules(features)
	var l []string
	clients := func(base string) {
		if pl.Web {
			l = append(l, base+"web/")
		}
		if pl.Mobile {
			l = append(l, base+"mobile/")
		}
		if pl.Desktop {
			l = append(l, base+"desktop/")
		}
	}
	api := func(dir string) {
		if !pl.Backend {
			return
		}
		switch arch {
		case core.ArchMicroservices:
			for _, m := range mods {
				l = append(l, "services/"+m+"/")
			}
			if len(mods) == 0 {
				l = append(l, "services/api/")
			}
		case core.ArchModular:
			l = append(l, dir)
			for _, m := range mods {
				l = append(l, dir+"modules/"+m+"/")
			}
		default:
			l = append(l, dir)
		}
	}
	switch {
	case repo == core.RepoMonorepo:
		template = "monorepo"
		clients("apps/")
		api("services/api/")
		if pl.Web || pl.Mobile || pl.Desktop {
			l = append(l, "packages/ui/")
		}
		l = append(l, "packages/types/")
	case arch == core.ArchMicroservices:
		template = "microservices"
		clients("apps/")
		api("")
	case arch == core.ArchModular:
		template = "modular"
		clients("apps/")
		api("apps/api/")
	default:
		template = "simple"
		n := 0
		for _, b := range []bool{pl.Web, pl.Mobile, pl.Desktop, pl.Backend} {
			if b {
				n++
			}
		}
		if n <= 1 {
			l = append(l, "src/", "tests/")
		} else {
			if pl.Web {
				l = append(l, "frontend/")
			}
			if pl.Mobile {
				l = append(l, "mobile/")
			}
			if pl.Desktop {
				l = append(l, "desktop/")
			}
			if pl.Backend {
				l = append(l, "backend/")
			}
		}
	}
	l = append(l, "docs/")
	reason = fmt.Sprintf("Mengikuti keputusan teknologi: arsitektur %s, repository %s.", arch, repo)
	return template, l, reason
}

// modules turns feature names into backend module folder names.
func modules(features []core.Feature) []string {
	var out []string
	for _, f := range features {
		ascii := strings.Map(func(r rune) rune {
			if r < unicode.MaxASCII {
				return r
			}
			return ' '
		}, normalize(f.Name))
		m := strings.Join(strings.Fields(ascii), "-")
		if len(m) > 24 {
			m = strings.TrimRight(m[:24], "-")
		}
		if m == "" || slices.Contains(out, m) {
			continue
		}
		out = append(out, m)
		if len(out) == 8 {
			break
		}
	}
	return out
}
