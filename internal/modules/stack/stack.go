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
	core.TechMessaging: {
		{"Redis Streams", "", "event antar modul yang tidak boleh hilang (consumer group, ack, retry)"},
		{"Redis Pub/Sub", "", "notifikasi UI dan invalidasi cache yang boleh hilang"},
		{"NATS", "", "event antar layanan, latency rendah"},
		{"RabbitMQ", "", "antrean pesan dengan routing dan acknowledgement"},
		{"Kafka", "", "event stream volume sangat tinggi, replay; butuh ADR"},
		{core.TechNone, "", "hanya prototipe tanpa efek samping lintas modul"},
	},
	core.TechWorkers: {
		{core.TechNone, "", "semua proses selesai di dalam request"},
		{"Job in-process", "", "job ringan di proses backend yang sama"},
		{"Worker terpisah + antrean", "", "job lama/berat (AI, file, laporan) di luar request"},
	},
	core.TechStorage: {
		{core.TechNone, "", "tidak ada file dari user"},
		{"S3-compatible (S3 / MinIO)", "", "upload file, signed URL, skala besar"},
		{"Disk server lokal", "", "prototipe satu server, file sedikit"},
	},
	core.TechSearch: {
		{core.TechNone, "", "filter dan query database biasa cukup"},
		{"PostgreSQL full-text search", "", "pencarian teks tanpa layanan tambahan"},
		{"Meilisearch", "", "pencarian cepat, typo-tolerant"},
		{"Elasticsearch / OpenSearch", "", "pencarian dan analitik data besar"},
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
	core.TechDeployment: {
		{"Docker Compose (satu server)", "", "satu server/VPS, operasional sederhana"},
		{"Managed PaaS (container)", "", "tanpa mengelola server, skala otomatis"},
		{"Kubernetes", "", "banyak layanan, skala dan deploy independen"},
		{"Static hosting + CDN", "", "frontend statis tanpa backend"},
		{"Node server / Vercel", "", "frontend dengan render di server (SSR)"},
	},
	core.TechObservability: {
		{"Log terstruktur + health check", "", "aplikasi kecil-menengah"},
		{"OpenTelemetry + Prometheus + Grafana", "", "tracing dan metrics terpusat, beban tinggi, audit"},
		{"Sentry", "", "pelacakan error aplikasi"},
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
var KnownNeeds = []string{"realtime", "offline", "seo", "ai", "high_concurrency", "flexible_schema", "transactions", "independent_scaling", "enterprise", "files", "search", "webhook"}

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
	"files":               {"upload", "lampiran", "attachment", "berkas", "object storage", "signed url"},
	"search":              {"pencarian teks", "full text", "full text search", "search engine", "fuzzy search", "pencarian cepat"},
	"webhook":             {"webhook", "callback http", "callback HTTP"},
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
	if p.Survey != nil {
		if p.Survey.HasWebhook() {
			found["webhook"] = true
		}
		if p.Survey.Services == core.SurveyServicesMany {
			found["independent_scaling"] = true
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
		d := core.TechDecision{Area: area, Recommended: value, Reason: reason, Alternatives: alternatives(area, value)}
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
		if p.Survey != nil && p.Survey.Latency == core.SurveyLatency100 {
			v, r = "Go + Gin", "Target latency p95 < 100 ms."
		}
		add(core.TechBackend, v, r)
		v, r = pick(core.TechDatabase)
		if has("flexible_schema") && !has("transactions") {
			v, r = "MongoDB", "Data berbentuk dokumen dengan skema yang sering berubah."
		}
		add(core.TechDatabase, v, r)
		v, r = pick(core.TechCache)
		if v == "" || v == core.CacheNone {
			v, r = "Redis", "Cache, antrean, rate limit, dan event; database tetap sumber kebenaran."
		}
		if has("realtime") || has("high_concurrency") {
			v, r = "Redis", "State realtime, pub/sub, dan antrean."
		}
		add(core.TechCache, v, r)
	}
	v, r := pick(core.TechArchitecture)
	switch {
	case has("independent_scaling"):
		v, r = core.ArchMicroservices, "Butuh skala dan deployment independen per layanan."
	case p.Survey != nil && p.Survey.Stage == core.SurveyStagePrototype:
		v, r = core.ArchSimple, "Tahap prototipe; struktur sederhana dulu."
	case len(p.Features) <= 3:
		v, r = core.ArchSimple, "Fiturnya sedikit; struktur sederhana cukup dan mudah dipelihara."
	}
	arch := v
	if plat.Backend {
		infra(s, add, has, arch)
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
	switch {
	case plat.Backend && arch == core.ArchMicroservices && has("independent_scaling"):
		add(core.TechDeployment, "Kubernetes", "Layanan di-deploy dan diskalakan sendiri-sendiri.")
	case plat.Backend:
		add(core.TechDeployment, "Docker Compose (satu server)", "Satu backend dan database; satu server cukup dan murah dioperasikan.")
	case plat.Web && s.Choice(core.TechFrontend) == "Next.js + TypeScript":
		add(core.TechDeployment, "Node server / Vercel", "Next.js merender halaman di server.")
	case plat.Web:
		add(core.TechDeployment, "Static hosting + CDN", "Tanpa backend; frontend cukup file statis.")
	}
	if plat.Backend {
		if has("enterprise") || has("high_concurrency") || arch == core.ArchMicroservices {
			add(core.TechObservability, "OpenTelemetry + Prometheus + Grafana", "Audit, beban tinggi atau banyak layanan butuh tracing dan metrics terpusat.")
		} else {
			add(core.TechObservability, "Log terstruktur + health check", "Skala awal; log terstruktur dan health check cukup.")
		}
	}
	return s
}

// infra decides the backend infrastructure areas. Each is only used when a
// need asks for it; otherwise it is "Tidak perlu" with the reason, so the
// user sees it was considered and can still choose one.
func infra(s *core.TechStack, add func(area, value, reason string), has func(string) bool, arch string) {
	switch {
	case arch == core.ArchMicroservices:
		add(core.TechMessaging, "NATS", "Layanan terpisah berkomunikasi lewat event.")
	case s.Choice(core.TechCache) == "Redis":
		add(core.TechMessaging, "Redis Streams", "Event antar modul yang tidak boleh hilang; Redis Pub/Sub hanya untuk notifikasi UI.")
	default:
		add(core.TechMessaging, "Redis Streams", "Event antar modul berformat seragam; Redis dipakai untuk stream.")
	}
	var why []string
	for _, w := range []struct{ need, text string }{
		{"ai", "panggilan AI yang lama"}, {"files", "pemrosesan file"},
		{"transactions", "laporan dan rekonsiliasi terjadwal"}, {"high_concurrency", "agregasi data di luar jalur request"},
		{"webhook", "penerimaan dan pengiriman webhook di luar request"},
	} {
		if has(w.need) {
			why = append(why, w.text)
		}
	}
	if len(why) > 0 {
		add(core.TechWorkers, "Worker terpisah + antrean", "Dikerjakan di luar request: "+strings.Join(why, ", ")+".")
	} else {
		add(core.TechWorkers, "Job in-process", "Belum ada job berat; job ringan (email) cukup di proses backend.")
	}
	if has("files") {
		add(core.TechStorage, "S3-compatible (S3 / MinIO)", "Ada upload file; disimpan di object storage dengan signed URL.")
	} else {
		add(core.TechStorage, core.TechNone, "Tidak ada kebutuhan upload file.")
	}
	switch {
	case !has("search"):
		add(core.TechSearch, core.TechNone, "Filter dan query database biasa cukup.")
	case s.Choice(core.TechDatabase) == "PostgreSQL":
		add(core.TechSearch, "PostgreSQL full-text search", "Pencarian teks memakai database yang sama; tanpa layanan tambahan.")
	default:
		add(core.TechSearch, "Meilisearch", "Pencarian teks cepat terpisah dari database dokumen.")
	}
}

// alternatives are up to two other known options for area.
func alternatives(area, value string) []string {
	var out []string
	for _, o := range Catalog[area] {
		if !strings.EqualFold(o.Value, value) && o.Value != core.TechNone && len(out) < 2 {
			out = append(out, o.Value)
		}
	}
	return out
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
	next.History, next.Status, next.Approved = old.History, old.State(), old.Approved
	if old.Proposal != nil {
		next.SetStatus(old.Proposal.Previous) // the proposal was made on the old decisions
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
	d.Override = &core.TechOverride{Value: value, Reason: reason, Language: languageOf(value)}
	if o != nil {
		d.Override.Language = o.Language
	}
	return nil
}

// languageOf guesses the language of a technology outside the catalog.
func languageOf(value string) string {
	v := normalize(value)
	for _, l := range []struct{ word, lang string }{
		{" rust ", "Rust"}, {" axum ", "Rust"}, {" go ", "Go"}, {" golang ", "Go"}, {" python ", "Python"}, {" django ", "Python"},
		{" java ", "Java"}, {" spring ", "Java"}, {" kotlin ", "Kotlin"}, {" php ", "PHP"}, {" laravel ", "PHP"},
		{" ruby ", "Ruby"}, {" rails ", "Ruby"}, {" net ", "C#"}, {" c ", "C#"}, {" elixir ", "Elixir"}, {" phoenix ", "Elixir"},
		{" typescript ", "TypeScript"}, {" node ", "TypeScript"}, {" express ", "TypeScript"}, {" swift ", "Swift"}, {" dart ", "Dart"},
	} {
		if strings.Contains(v, l.word) {
			return l.lang
		}
	}
	return ""
}

func values(area string) []string {
	var v []string
	for _, o := range Catalog[area] {
		v = append(v, o.Value)
	}
	return v
}
