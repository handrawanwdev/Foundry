// Package stack decides the technology of the product per domain:
// application stack (framework with its language and runtime, UI library,
// API protocol), data and communication, architecture and repository,
// infrastructure and operations, engineering standards.
//
// A recommendation starts from the strongest evidence: the project's
// existing code, the stack the user stated, the technical needs, then the
// profile's policy for the kind of product (profiles/*.json) and last a
// default. Profiles are policy, not rules: they only fill what nothing more
// specific decides. Optional components (cache, messaging, workers,
// storage, search) are recommended only for a need. The decision is
// deterministic, so the same requirement gets the same recommendation; the
// user may override any area.
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

// Profile is the recommendation policy for a kind of product: the
// defaults it prefers when the code, the user and the needs leave an area
// open.
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

// Option is a technology Foundry knows for an area, with what it costs.
type Option struct {
	Value    string `json:"value"`
	Language string `json:"language,omitempty"`
	Runtime  string `json:"runtime,omitempty"`
	Fit      string `json:"fit"`
	// Tradeoff is what the option gives up; Ops what it asks from operations.
	Tradeoff string `json:"tradeoff,omitempty"`
	Ops      string `json:"ops,omitempty"`
	// Needs are the technical needs it serves well; they rank alternatives.
	Needs []string `json:"needs,omitempty"`
	// Framework is the frontend framework a UI library is built for
	// ("" = any); Platform the only mobile platform an option runs on.
	Framework string `json:"framework,omitempty"`
	Platform  string `json:"platform,omitempty"`
	// Keys are words in a stated preference that name this option.
	Keys []string `json:"-"`
	// Langs are the languages a testing or quality tool is for, when more
	// than Language.
	Langs []string `json:"-"`
}

// Catalog lists the known options per area. Every area accepts a free
// value from the user except closedAreas, from which the project
// structure is derived.
var Catalog = map[string][]Option{
	core.TechFrontend: {
		{Value: "React + TypeScript + Vite", Language: "TypeScript", Fit: "SaaS, dashboard, aplikasi bisnis, UI kompleks",
			Tradeoff: "Dirender di browser (SPA); halaman publik yang butuh SEO perlu prerender atau SSR.", Ops: "Hasil build file statis; cukup static hosting/CDN.", Keys: []string{"react", "vite"}},
		{Value: "Next.js + TypeScript", Language: "TypeScript", Runtime: "Node.js (render server)", Fit: "website publik, SEO, aplikasi hybrid",
			Tradeoff: "Render di server menambah kompleksitas data fetching dan caching.", Ops: "Butuh Node server atau platform SSR; static export menghilangkan fitur server.",
			Needs: []string{"seo"}, Keys: []string{"next", "nextjs", "next js"}},
		{Value: "Vue + TypeScript + Vite", Language: "TypeScript", Fit: "aplikasi bisnis, pengembangan cepat",
			Tradeoff: "Ekosistem komponen lebih kecil dibanding React.", Ops: "Hasil build file statis; cukup static hosting/CDN.", Keys: []string{"vue"}},
	},
	core.TechUI: {
		{Value: "Tailwind CSS + shadcn/ui", Framework: "React", Fit: "UI kustom; komponen disalin ke repo dan bisa diubah",
			Tradeoff: "Komponen menjadi kode project sendiri; pembaruan tidak otomatis.", Ops: "Tanpa runtime tambahan; ikut build frontend.", Keys: []string{"shadcn", "tailwind"}},
		{Value: "MUI (Material UI)", Framework: "React", Fit: "aplikasi internal dengan komponen siap pakai yang lengkap",
			Tradeoff: "Gaya Material Design; kustomisasi visual mendalam lebih berat.", Keys: []string{"mui", "material ui"}},
		{Value: "Ant Design", Framework: "React", Fit: "aplikasi enterprise dengan banyak tabel dan form", Needs: []string{"enterprise"},
			Tradeoff: "Gaya visual khas Ant Design; ukuran bundle relatif besar.", Keys: []string{"antd", "ant design"}},
		{Value: "PrimeVue", Framework: "Vue", Fit: "aplikasi bisnis Vue dengan tabel dan form yang kaya", Keys: []string{"primevue"}},
		{Value: "Vuetify", Framework: "Vue", Fit: "aplikasi Vue dengan komponen Material Design", Keys: []string{"vuetify"}},
		{Value: "Tailwind CSS", Fit: "styling utility-first tanpa pustaka komponen; framework apa pun",
			Tradeoff: "Komponen (dialog, tabel, aksesibilitas) dibangun sendiri.", Keys: []string{"tailwind", "tailwindcss"}},
	},
	core.TechMobile: {
		{Value: "React Native + TypeScript + Expo", Language: "TypeScript", Runtime: "Hermes (JavaScript engine)", Fit: "tim TypeScript, aplikasi bisnis Android dan iOS",
			Tradeoff: "Modul native khusus butuh development build atau config plugin.", Ops: "Build dan rilis store lewat EAS atau build lokal.", Keys: []string{"react native", "expo"}},
		{Value: "Flutter", Language: "Dart", Runtime: "Dart (kompilasi AOT ke native)", Fit: "UI konsisten di Android dan iOS",
			Tradeoff: "Bahasa Dart terpisah dari tim web TypeScript; UI tidak bisa dibagi dengan web React/Vue.", Ops: "Build per platform dengan toolchain Flutter.", Keys: []string{"flutter", "dart"}},
		{Value: "Kotlin + Jetpack Compose", Language: "Kotlin", Runtime: "Android (ART)", Platform: core.PlatformAndroid, Fit: "aplikasi khusus Android dengan akses API native penuh",
			Tradeoff: "Hanya Android; iOS butuh aplikasi terpisah.", Keys: []string{"kotlin", "jetpack compose"}},
		{Value: "Swift + SwiftUI", Language: "Swift", Runtime: "iOS (native)", Platform: core.PlatformIOS, Fit: "aplikasi khusus iOS dengan akses API native penuh",
			Tradeoff: "Hanya platform Apple; Android butuh aplikasi terpisah.", Keys: []string{"swift", "swiftui"}},
	},
	core.TechDesktop: {
		{Value: "Tauri + React + TypeScript", Language: "TypeScript", Runtime: "WebView sistem + core Rust", Fit: "aplikasi desktop ringan dengan UI web",
			Tradeoff: "Tampilan bergantung WebView tiap OS; logika native ditulis di Rust.", Ops: "Installer kecil; signing per OS.", Keys: []string{"tauri"}},
		{Value: "Electron + React + TypeScript", Language: "TypeScript", Runtime: "Chromium + Node.js (dibundel)", Fit: "ekosistem Node penuh di desktop",
			Tradeoff: "Ukuran installer dan pemakaian memori lebih besar.", Ops: "Installer besar; auto-update dan signing per OS.", Keys: []string{"electron"}},
	},
	core.TechCLI: {
		{Value: "Go + Cobra", Language: "Go", Runtime: "binary native", Fit: "CLI lintas OS dalam satu binary",
			Ops: "Distribusi satu file binary per OS/arsitektur.", Keys: []string{"cobra"}},
		{Value: "Rust + clap", Language: "Rust", Runtime: "binary native", Fit: "CLI cepat dengan kontrol memori",
			Tradeoff: "Kurva belajar dan waktu kompilasi lebih tinggi.", Ops: "Distribusi satu file binary per OS/arsitektur.", Keys: []string{"rust", "clap"}},
		{Value: "Python + Typer", Language: "Python", Runtime: "CPython", Fit: "otomasi dan skrip data untuk tim Python",
			Ops: "Pengguna butuh Python (pipx) atau paket hasil bundling.", Keys: []string{"typer"}},
		{Value: "Node.js + TypeScript + Commander", Language: "TypeScript", Runtime: "Node.js", Fit: "tooling di ekosistem JavaScript",
			Ops: "Pengguna butuh Node.js (npx) atau paket hasil bundling.", Keys: []string{"commander"}},
	},
	core.TechBackend: {
		{Value: "NestJS + TypeScript", Language: "TypeScript", Runtime: "Node.js", Fit: "SaaS, API, realtime, iterasi cepat", Needs: []string{"enterprise", "realtime"},
			Tradeoff: "Struktur modul dan dependency injection terasa berat untuk API kecil.", Ops: "Proses Node.js; skala horizontal per instance.", Keys: []string{"nest", "nestjs"}},
		{Value: "Go + Gin", Language: "Go", Runtime: "binary native", Fit: "konkurensi tinggi, latency rendah, infrastruktur", Needs: []string{"high_concurrency", "low_latency", "realtime"},
			Tradeoff: "Lebih banyak kode manual; tanpa DI atau ORM bawaan.", Ops: "Satu binary; image container kecil.", Keys: []string{"gin", "go", "golang"}},
		{Value: "Go + Fiber", Language: "Go", Runtime: "binary native", Fit: "konkurensi tinggi dengan API bergaya Express", Needs: []string{"high_concurrency", "low_latency"},
			Tradeoff: "Berbasis fasthttp, bukan net/http; sebagian middleware net/http tidak langsung kompatibel.", Ops: "Satu binary; image container kecil.", Keys: []string{"fiber", "go", "golang"}},
		{Value: "Python + FastAPI", Language: "Python", Runtime: "CPython (server ASGI, mis. Uvicorn)", Fit: "AI, data processing, otomasi", Needs: []string{"ai"},
			Tradeoff: "Kerja CPU-bound dibatasi GIL; skala lewat banyak proses.", Ops: "Jalankan beberapa proses worker ASGI per instance.", Keys: []string{"fastapi", "python"}},
		{Value: "Spring Boot + Java", Language: "Java", Runtime: "JVM", Fit: "enterprise, integrasi luas, tim Java", Needs: []string{"enterprise", "transactions"},
			Tradeoff: "Waktu start dan memori lebih besar dibanding Go.", Ops: "Atur ukuran heap JVM per instance.", Keys: []string{"spring", "spring boot", "java"}},
		{Value: "ASP.NET Core + C#", Language: "C#", Runtime: ".NET", Fit: "enterprise, ekosistem Microsoft", Needs: []string{"enterprise"},
			Ops: "Runtime .NET di server atau container.", Keys: []string{"asp net", "dotnet", "net core", "c#"}},
		{Value: "Laravel + PHP", Language: "PHP", Runtime: "PHP-FPM", Fit: "aplikasi CRUD bisnis, tim PHP",
			Tradeoff: "Model proses per request; realtime butuh komponen tambahan.", Ops: "PHP-FPM di belakang web server.", Keys: []string{"laravel", "php"}},
	},
	core.TechAPI: {
		{Value: "REST + OpenAPI", Fit: "API umum untuk web, mobile, dan integrasi pihak ketiga",
			Tradeoff: "Bentuk data tetap per endpoint; klien bisa mengambil data berlebih atau kurang.", Ops: "Kontrak OpenAPI dipakai untuk generate client dan dokumentasi.", Keys: []string{"rest", "openapi"}},
		{Value: "REST + OpenAPI + WebSocket", Fit: "API umum plus update realtime ke klien", Needs: []string{"realtime"},
			Tradeoff: "Koneksi WebSocket stateful; banyak instance butuh pub/sub bersama.", Ops: "Load balancer harus meneruskan WebSocket.", Keys: []string{"websocket"}},
		{Value: "GraphQL", Fit: "klien beragam dengan kebutuhan data berbeda",
			Tradeoff: "Caching HTTP dan pembatasan biaya query ditangani sendiri.", Keys: []string{"graphql"}},
		{Value: "gRPC", Fit: "komunikasi antar layanan internal dengan kontrak ketat", Needs: []string{"independent_scaling", "low_latency"},
			Tradeoff: "Browser tidak memanggil gRPC langsung; web butuh gRPC-Web atau proxy.", Keys: []string{"grpc"}},
		{Value: "tRPC", Language: "TypeScript", Fit: "frontend dan backend TypeScript dalam satu repo",
			Tradeoff: "Hanya klien TypeScript; kurang cocok untuk API publik.", Keys: []string{"trpc"}},
	},
	core.TechDatabase: {
		{Value: "PostgreSQL", Language: "SQL", Fit: "aplikasi bisnis relasional, transaksi, enterprise", Needs: []string{"transactions", "enterprise", "search"},
			Ops: "Database server: backup, upgrade, dan pemantauan.", Keys: []string{"postgres", "postgresql"}},
		{Value: "MySQL", Language: "SQL", Fit: "CRUD standar, ekosistem yang sudah ada", Needs: []string{"transactions"},
			Ops: "Database server: backup, upgrade, dan pemantauan.", Keys: []string{"mysql"}},
		{Value: "MongoDB", Fit: "data berbentuk dokumen, skema fleksibel", Needs: []string{"flexible_schema"},
			Tradeoff: "Relasi dan join antar koleksi lebih terbatas; skema dijaga di aplikasi.", Ops: "Replica set dibutuhkan untuk transaksi multi-dokumen.", Keys: []string{"mongo", "mongodb"}},
		{Value: "SQLite", Language: "SQL", Fit: "aplikasi kecil satu server, atau database lokal desktop/CLI",
			Tradeoff: "Satu penulis dalam satu waktu; tidak dibagi banyak instance backend.", Ops: "Satu file; backup cukup salin file secara aman.", Keys: []string{"sqlite"}},
	},
	core.TechCache: {
		{Value: "Redis", Fit: "cache, state realtime bersama, antrean", Needs: []string{"realtime", "high_concurrency", "low_latency"},
			Tradeoff: "Data di memori; bukan sumber kebenaran.", Ops: "Satu layanan tambahan untuk dioperasikan dan dipantau.", Keys: []string{"redis"}},
		{Value: core.CacheNone, Fit: "tanpa cache terpisah; database cukup"},
	},
	core.TechMessaging: {
		{Value: "Redis Streams", Fit: "event antar modul yang tidak boleh hilang (consumer group, ack, retry)", Ops: "Butuh Redis; atur retensi stream.", Keys: []string{"redis streams"}},
		{Value: "Redis Pub/Sub", Fit: "notifikasi UI dan invalidasi cache yang boleh hilang", Needs: []string{"realtime"},
			Tradeoff: "Pesan hilang bila tidak ada subscriber yang terhubung.", Ops: "Butuh Redis.", Keys: []string{"pub sub", "pubsub"}},
		{Value: "NATS", Fit: "event antar layanan, latency rendah", Needs: []string{"independent_scaling"},
			Ops: "Server NATS tambahan; JetStream untuk pesan persisten.", Keys: []string{"nats"}},
		{Value: "RabbitMQ", Fit: "antrean pesan dengan routing dan acknowledgement", Ops: "Broker tambahan untuk dioperasikan.", Keys: []string{"rabbitmq"}},
		{Value: "Kafka", Fit: "event stream volume sangat tinggi dengan replay", Tradeoff: "Berlebihan untuk volume kecil.",
			Ops: "Operasional cluster berat; butuh ADR.", Keys: []string{"kafka"}},
		{Value: core.TechNone, Fit: "modul dalam satu proses; tanpa event lintas layanan"},
	},
	core.TechWorkers: {
		{Value: core.TechNone, Fit: "semua proses selesai di dalam request"},
		{Value: "Job in-process", Fit: "job ringan di proses backend yang sama", Needs: []string{"files"},
			Tradeoff: "Job hilang bila proses mati, kecuali dicatat di database."},
		{Value: "Worker terpisah + antrean", Fit: "job lama/berat (AI, webhook, file, laporan) di luar request", Needs: []string{"ai", "webhook"},
			Ops: "Proses dan deployment tambahan; butuh antrean (Redis, broker, atau tabel database)."},
	},
	core.TechStorage: {
		{Value: core.TechNone, Fit: "tidak ada file dari user"},
		{Value: "S3-compatible (S3 / MinIO)", Fit: "upload file, signed URL, skala besar", Needs: []string{"files"},
			Ops: "Bucket, kebijakan akses, dan lifecycle dikelola.", Keys: []string{"s3", "minio"}},
		{Value: "Disk server lokal", Fit: "prototipe satu server, file sedikit",
			Tradeoff: "Tidak bisa dibagi banyak instance; backup ikut server."},
	},
	core.TechSearch: {
		{Value: core.TechNone, Fit: "filter dan query database biasa cukup"},
		{Value: "PostgreSQL full-text search", Fit: "pencarian teks tanpa layanan tambahan", Needs: []string{"search"}},
		{Value: "Meilisearch", Fit: "pencarian cepat, typo-tolerant", Needs: []string{"search"},
			Ops: "Layanan tambahan; index disinkronkan dari database.", Keys: []string{"meilisearch"}},
		{Value: "Elasticsearch / OpenSearch", Fit: "pencarian dan analitik data besar",
			Ops: "Cluster berat untuk dioperasikan; index disinkronkan dari database.", Keys: []string{"elasticsearch", "opensearch"}},
	},
	core.TechArchitecture: {
		{Value: core.ArchSimple, Fit: "prototipe, aplikasi kecil", Tradeoff: "Batas domain kabur bila fitur bertambah."},
		{Value: core.ArchModular, Fit: "aplikasi bisnis dengan beberapa domain",
			Tradeoff: "Disiplin batas modul dijaga di kode, bukan oleh jaringan.", Ops: "Satu deployment.", Keys: []string{"modular monolith", "modular"}},
		{Value: core.ArchMicroservices, Fit: "skala dan deployment independen, organisasi besar", Needs: []string{"independent_scaling"},
			Tradeoff: "Konsistensi data lintas layanan dan debugging lebih sulit.", Ops: "Banyak deployment, service discovery, tracing terpusat.", Keys: []string{"microservice", "microservices"}},
	},
	core.TechRepository: {
		{Value: core.RepoSimple, Fit: "satu repo tanpa workspace bersama; satu aplikasi klien dan backend"},
		{Value: core.RepoMonorepo, Fit: "beberapa aplikasi berbagi tipe, kontrak, atau UI",
			Tradeoff: "Butuh tooling workspace (pnpm/npm workspaces, go.work, …).", Ops: "CI perlu membangun hanya bagian yang berubah.", Keys: []string{"monorepo"}},
	},
	core.TechDeployment: {
		{Value: "Docker Compose (satu server)", Fit: "satu server/VPS, operasional sederhana",
			Tradeoff: "Satu server menjadi titik gagal tunggal.", Ops: "Update dan backup server dikelola sendiri.", Keys: []string{"docker compose", "compose"}},
		{Value: "Managed PaaS (container)", Fit: "tanpa mengelola server, skala otomatis", Tradeoff: "Biaya per resource lebih tinggi; terikat fitur platform."},
		{Value: "Kubernetes", Fit: "banyak layanan, skala dan deploy independen", Needs: []string{"independent_scaling"},
			Ops: "Cluster dan manifest dikelola; butuh keahlian operasional.", Keys: []string{"kubernetes", "k8s"}},
		{Value: "Static hosting + CDN", Fit: "frontend statis tanpa backend"},
		{Value: "Node server / Vercel", Fit: "frontend dengan render di server (SSR)", Needs: []string{"seo"}, Keys: []string{"vercel"}},
	},
	core.TechObservability: {
		{Value: "Log terstruktur + health check", Fit: "aplikasi kecil-menengah"},
		{Value: "OpenTelemetry + Prometheus + Grafana", Fit: "tracing dan metrics terpusat, beban tinggi, banyak layanan", Needs: []string{"high_concurrency", "independent_scaling"},
			Ops: "Collector, Prometheus, dan Grafana dioperasikan.", Keys: []string{"opentelemetry", "prometheus", "grafana"}},
		{Value: "Sentry", Fit: "pelacakan error aplikasi", Keys: []string{"sentry"}},
	},
	core.TechTesting: {
		{Value: "Vitest", Language: "TypeScript", Langs: []string{"TypeScript", "JavaScript"}, Fit: "unit dan integrasi TypeScript", Keys: []string{"vitest"}},
		{Value: "Jest", Language: "TypeScript", Langs: []string{"TypeScript", "JavaScript"}, Fit: "unit dan integrasi TypeScript/JavaScript", Keys: []string{"jest"}},
		{Value: "go test", Language: "Go", Fit: "unit dan integrasi Go (bawaan toolchain)"},
		{Value: "pytest", Language: "Python", Fit: "unit dan integrasi Python", Keys: []string{"pytest"}},
		{Value: "flutter_test", Language: "Dart", Fit: "unit dan widget test Flutter"},
		{Value: "JUnit 5", Language: "Java", Langs: []string{"Java", "Kotlin"}, Fit: "unit dan integrasi JVM", Keys: []string{"junit"}},
		{Value: "xUnit", Language: "C#", Fit: "unit dan integrasi .NET", Keys: []string{"xunit"}},
		{Value: "PHPUnit", Language: "PHP", Fit: "unit dan integrasi PHP", Keys: []string{"phpunit"}},
		{Value: "cargo test", Language: "Rust", Fit: "unit dan integrasi Rust (bawaan toolchain)"},
		{Value: "XCTest", Language: "Swift", Fit: "unit dan UI test iOS"},
		{Value: "Playwright (E2E)", Language: "TypeScript", Langs: []string{"TypeScript", "JavaScript"}, Fit: "end-to-end web di browser nyata",
			Ops: "Browser test dijalankan di CI.", Keys: []string{"playwright"}},
	},
	core.TechQuality: {
		{Value: "ESLint + Prettier", Language: "TypeScript", Langs: []string{"TypeScript", "JavaScript"}, Fit: "lint dan format TypeScript/JavaScript", Keys: []string{"eslint", "prettier"}},
		{Value: "Biome", Language: "TypeScript", Langs: []string{"TypeScript", "JavaScript"}, Fit: "lint dan format TypeScript dalam satu tool", Keys: []string{"biome"}},
		{Value: "gofmt + go vet + golangci-lint", Language: "Go", Fit: "format dan lint Go", Keys: []string{"golangci"}},
		{Value: "Ruff", Language: "Python", Fit: "lint dan format Python", Keys: []string{"ruff"}},
		{Value: "dart analyze + dart format", Language: "Dart", Fit: "lint dan format Dart"},
		{Value: "rustfmt + clippy", Language: "Rust", Fit: "format dan lint Rust", Keys: []string{"clippy"}},
		{Value: "Checkstyle", Language: "Java", Fit: "aturan gaya kode Java", Keys: []string{"checkstyle"}},
		{Value: "ktlint", Language: "Kotlin", Fit: "lint dan format Kotlin", Keys: []string{"ktlint"}},
		{Value: "dotnet format", Language: "C#", Fit: "format kode .NET"},
		{Value: "PHP-CS-Fixer + PHPStan", Language: "PHP", Fit: "format dan analisis statis PHP", Keys: []string{"phpstan"}},
		{Value: "SwiftLint", Language: "Swift", Fit: "lint Swift", Keys: []string{"swiftlint"}},
	},
}

// closedAreas must be one of the catalog: the project structure is
// derived from them.
var closedAreas = map[string]bool{core.TechCache: true, core.TechArchitecture: true, core.TechRepository: true}

// multiAreas hold one tool per language, joined with ", ".
var multiAreas = map[string]bool{core.TechTesting: true, core.TechQuality: true}

func option(area, value string) *Option {
	for i, o := range Catalog[area] {
		if strings.EqualFold(o.Value, value) {
			return &Catalog[area][i]
		}
	}
	return nil
}

// parts splits a value into its tools (multiAreas) or returns it whole.
func parts(area, value string) []string {
	if !multiAreas[area] {
		return []string{value}
	}
	var out []string
	for _, p := range strings.Split(value, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// ---------- analysis ----------

// Needs Foundry recognizes. The analyst may report them in
// discovery.tech_needs; keywords in the requirement and the survey add the
// rest.
var KnownNeeds = []string{"realtime", "offline", "seo", "ai", "high_concurrency", "low_latency", "flexible_schema", "transactions",
	"independent_scaling", "enterprise", "files", "search", "webhook"}

var needWords = map[string][]string{
	"realtime":            {"realtime", "real time", "live", "websocket", "streaming", "chat", "notifikasi instan", "push notification"},
	"offline":             {"offline", "tanpa internet", "tanpa koneksi"},
	"seo":                 {"seo", "landing page", "website publik", "public website", "blog", "halaman publik", "marketing site"},
	"ai":                  {"ai", "llm", "gpt", "machine learning", "kecerdasan buatan", "chatbot", "embedding", "model bahasa"},
	"high_concurrency":    {"trading", "order book", "orderbook", "exchange", "bursa", "kripto", "crypto", "high frequency", "high concurrency", "konkurensi tinggi"},
	"flexible_schema":     {"skema fleksibel", "flexible schema", "form dinamis", "dynamic form", "field kustom", "custom field", "schemaless"},
	"transactions":        {"pembayaran", "payment", "transaksi", "invoice", "tagihan", "saldo", "akuntansi", "accounting", "ledger", "checkout"},
	"independent_scaling": {"microservice", "microservices", "skala independen", "independent scaling", "deploy terpisah", "separate deployment"},
	"enterprise":          {"sso", "audit trail", "audit log", "log audit", "jejak audit", "rbac", "compliance", "kepatuhan", "ldap", "enterprise"},
	"files":               {"upload", "lampiran", "attachment", "berkas", "object storage", "signed url"},
	"search":              {"pencarian teks", "full text", "full text search", "search engine", "fuzzy search", "pencarian cepat"},
	"webhook":             {"webhook", "callback http", "callback HTTP"},
}

// normalize lowercases text and turns punctuation into spaces, padded so
// " word " matches whole words only. "#" stays for "c#".
func normalize(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '#' {
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
		if p.Survey.Latency == core.SurveyLatency100 {
			found["low_latency"] = true
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
		case core.PlatformAndroid:
			t.Mobile, t.Android = true, true
		case core.PlatformIOS:
			t.Mobile, t.IOS = true, true
		case core.PlatformDesktop:
			t.Desktop = true
		case core.PlatformBackend:
			t.Backend = true
		case core.PlatformCLI:
			t.CLI = true
		}
	}
	return t
}

// preferences reads the stack the user stated in the survey: per area the
// known option it names best. A longer phrase of another area wins over a
// word inside it ("react native" is mobile, not the React frontend).
func preferences(p *core.Project) map[string]string {
	if p.Survey == nil || strings.TrimSpace(p.Survey.Stack) == "" {
		return nil
	}
	text := normalize(p.Survey.Stack)
	out := map[string]string{}
	for area, opts := range Catalog {
		own := text
		for other, oo := range Catalog {
			if other == area {
				continue
			}
			for _, o := range oo {
				for _, k := range o.Keys {
					for _, mine := range opts {
						for _, mk := range mine.Keys {
							if len(k) > len(mk) && strings.Contains(" "+k+" ", " "+mk+" ") {
								own = strings.ReplaceAll(own, " "+k+" ", "  ")
							}
						}
					}
				}
			}
		}
		best, score := "", 0
		for _, o := range opts {
			n := 0
			for _, k := range o.Keys {
				if strings.Contains(own, " "+normalize(k)[1:]) {
					n += len(k)
				}
			}
			if n > score || n == score && n > 0 && len(o.Value) < len(best) {
				best, score = o.Value, n
			}
		}
		if best != "" {
			out[area] = best
		}
	}
	return out
}

// ---------- recommendation ----------

// baseAreas are decided from the project (platform, needs, preference,
// profile); derivedAreas from the other choices, in dependency order.
var (
	baseAreas    = []string{core.TechFrontend, core.TechMobile, core.TechDesktop, core.TechBackend, core.TechCLI, core.TechDatabase, core.TechArchitecture, core.TechRepository}
	derivedAreas = []string{core.TechUI, core.TechAPI, core.TechWorkers, core.TechCache, core.TechMessaging, core.TechStorage, core.TechSearch,
		core.TechDeployment, core.TechObservability, core.TechTesting, core.TechQuality}
)

// dependsOn: a change of any of these areas re-evaluates the area.
var dependsOn = map[string][]string{
	core.TechUI:            {core.TechFrontend, core.TechDesktop},
	core.TechAPI:           {core.TechBackend, core.TechFrontend, core.TechMobile},
	core.TechCache:         {core.TechWorkers},
	core.TechMessaging:     {core.TechArchitecture, core.TechCache},
	core.TechSearch:        {core.TechDatabase},
	core.TechDeployment:    {core.TechFrontend, core.TechBackend, core.TechArchitecture},
	core.TechObservability: {core.TechArchitecture},
	core.TechTesting:       {core.TechFrontend, core.TechMobile, core.TechDesktop, core.TechCLI, core.TechBackend},
	core.TechQuality:       {core.TechFrontend, core.TechMobile, core.TechDesktop, core.TechCLI, core.TechBackend},
}

func pickProfile(needs []string, plat core.TechPlatform) *Profile {
	match := map[string]bool{}
	for _, n := range needs {
		match[n] = true
	}
	if plat.Mobile && !plat.Web && !plat.Desktop {
		match["mobile_only"] = true
	}
	for i := range Profiles {
		if slices.ContainsFunc(Profiles[i].Match, func(m string) bool { return match[m] }) {
			return &Profiles[i]
		}
	}
	return ProfileByID("saas")
}

// Recommend decides the stack for p. An existing project keeps what its
// code uses. Of the areas its code does not show, only a new part of the
// product (a platform the code has no app for) and a technical need are
// recommended: no database behind an existing backend (the scan may miss
// it), no defaults, and no "Tidak perlu". Overrides are not carried; use
// Carry.
func Recommend(p *core.Project) *core.TechStack {
	needs, plat := Analyze(p), platformOf(p)
	prof := pickProfile(needs, plat)
	existing := p.Structure != nil && p.Structure.Mode == core.StructureExisting
	s := &core.TechStack{Mode: core.StructureNew, Needs: needs, Platform: plat}
	if existing {
		s = fromCode(p, needs, plat)
	}
	s.Profile = prof.ID
	r := recommender{p: p, prof: prof, pref: preferences(p)}
	for _, area := range baseAreas {
		if s.Decision(area) != nil {
			continue
		}
		if existing && area == core.TechDatabase && s.Decision(core.TechBackend) != nil {
			continue
		}
		if v, why, src, ok := r.base(s, area); ok && !(existing && none(v)) {
			add(s, area, v, why, src)
		}
	}
	for _, area := range derivedAreas {
		if s.Decision(area) != nil {
			continue
		}
		if v, why, src, ok := derive(s, area, r.pref); ok && !(existing && (none(v) || src != core.SourceNeed && !newPart(s, area))) {
			add(s, area, v, why, src)
		}
	}
	sortDecisions(s)
	fillAlternatives(s)
	return s
}

// newPart reports whether an area the derived one depends on was not read
// from the code: it belongs to a part of the product still to be built.
func newPart(s *core.TechStack, area string) bool {
	return slices.ContainsFunc(dependsOn[area], func(a string) bool {
		d := s.Decision(a)
		return d != nil && !fromExisting(s, *d)
	})
}

func add(s *core.TechStack, area, value, reason, source string) {
	d := core.TechDecision{Area: area, Recommended: value, Reason: reason, Source: source}
	d.Language, d.Runtime = describe(area, value)
	s.Decisions = append(s.Decisions, d)
}

// describe is the language and runtime of value: from the catalog, else
// guessed from its name (runtime only from the catalog).
func describe(area, value string) (string, string) {
	if o := option(area, value); o != nil {
		return o.Language, o.Runtime
	}
	if multiAreas[area] {
		return "", ""
	}
	return languageOf(value), ""
}

func sortDecisions(s *core.TechStack) {
	slices.SortStableFunc(s.Decisions, func(a, b core.TechDecision) int {
		return slices.Index(core.TechAreas, a.Area) - slices.Index(core.TechAreas, b.Area)
	})
}

type recommender struct {
	p    *core.Project
	prof *Profile
	pref map[string]string
}

// base recommends an area decided from the project itself.
func (r recommender) base(s *core.TechStack, area string) (value, reason, source string, ok bool) {
	plat, has := s.Platform, func(n string) bool { return slices.Contains(s.Needs, n) }
	on := map[string]bool{core.TechFrontend: plat.Web, core.TechMobile: plat.Mobile, core.TechDesktop: plat.Desktop,
		core.TechBackend: plat.Backend, core.TechCLI: plat.CLI, core.TechDatabase: plat.Backend,
		core.TechArchitecture: true, core.TechRepository: true}
	if !on[area] {
		return "", "", "", false
	}
	if v := r.pref[area]; v != "" && fits(s, area, v) {
		return v, "Sesuai preferensi stack kamu: “" + strings.TrimSpace(r.p.Survey.Stack) + "”.", core.SourcePreference, true
	}
	profile := func() (string, string, string, bool) {
		x := r.prof.Decisions[area]
		return x.Value, "Kebijakan profil " + r.prof.Name + ": " + x.Reason, core.SourceProfile, x.Value != ""
	}
	switch area {
	case core.TechFrontend:
		if has("seo") {
			return "Next.js + TypeScript", "Kebutuhan SEO: halaman publik dirender di server.", core.SourceNeed, true
		}
	case core.TechMobile:
		if plat.Android != plat.IOS && plat.Android {
			// one platform only: the profile's cross-platform pick still fits;
			// native stays an alternative.
			break
		}
	case core.TechBackend:
		if has("low_latency") {
			return "Go + Gin", "Kebutuhan latency p95 < 100 ms: Go dengan konkurensi tinggi.", core.SourceNeed, true
		}
	case core.TechCLI:
		switch langOf(s, core.TechBackend) {
		case "TypeScript":
			return "Node.js + TypeScript + Commander", "Satu bahasa dengan backend TypeScript.", core.SourceDependency, true
		case "Python":
			return "Python + Typer", "Satu bahasa dengan backend Python.", core.SourceDependency, true
		}
		return "Go + Cobra", "Satu binary tanpa runtime yang harus dipasang pengguna.", core.SourceDefault, true
	case core.TechDatabase:
		if has("flexible_schema") && !has("transactions") {
			return "MongoDB", "Kebutuhan skema fleksibel: data berbentuk dokumen yang sering berubah.", core.SourceNeed, true
		}
	case core.TechArchitecture:
		switch {
		case has("independent_scaling"):
			return core.ArchMicroservices, "Kebutuhan skala dan deployment independen per layanan.", core.SourceNeed, true
		case r.p.Survey != nil && r.p.Survey.Stage == core.SurveyStagePrototype:
			return core.ArchSimple, "Tahap prototipe; struktur sederhana dulu.", core.SourceNeed, true
		case len(r.p.Features) <= 3:
			return core.ArchSimple, "Fiturnya sedikit; struktur sederhana cukup dan mudah dipelihara.", core.SourceNeed, true
		}
	case core.TechRepository:
		apps := 0
		for _, b := range []bool{plat.Web, plat.Mobile, plat.Desktop} {
			if b {
				apps++
			}
		}
		if apps >= 2 {
			return core.RepoMonorepo, "Beberapa aplikasi klien berbagi tipe dan komponen dalam satu repo.", core.SourceNeed, true
		}
		return core.RepoSimple, "Satu aplikasi klien (bila ada) dan backend cukup dipisah per folder.", core.SourceDefault, true
	}
	return profile()
}

// langOf is the language chosen for area, "" when the area is not decided.
func langOf(s *core.TechStack, area string) string {
	if d := s.Decision(area); d != nil {
		return d.Lang()
	}
	return ""
}

// derive recommends an area from the other choices and the needs. ok is
// false when the area is not part of the product.
func derive(s *core.TechStack, area string, pref map[string]string) (value, reason, source string, ok bool) {
	plat, has, ch := s.Platform, func(n string) bool { return slices.Contains(s.Needs, n) }, s.Choice
	on := map[string]bool{core.TechUI: plat.Web || plat.Desktop, core.TechDeployment: plat.Backend || plat.Web,
		core.TechTesting: len(languages(s)) > 0, core.TechQuality: len(languages(s)) > 0}
	for _, a := range []string{core.TechAPI, core.TechWorkers, core.TechCache, core.TechMessaging, core.TechStorage, core.TechSearch, core.TechObservability} {
		on[a] = plat.Backend
	}
	if !on[area] {
		return "", "", "", false
	}
	if v := pref[area]; v != "" && fits(s, area, v) {
		return v, "Sesuai preferensi stack kamu.", core.SourcePreference, true
	}
	switch area {
	case core.TechUI:
		fam := family(s)
		if pr := ProfileByID(s.Profile); pr != nil {
			if x, ok := pr.Decisions[core.TechUI]; ok && x.Value != "" && uiFits(x.Value, fam) {
				return x.Value, "Kebijakan profil " + pr.Name + ": " + x.Reason, core.SourceProfile, true
			}
		}
		switch fam {
		case "React":
			return "Tailwind CSS + shadcn/ui", "Mengikuti frontend React: komponen dapat diubah dan tetap memakai token desain project.", core.SourceDependency, true
		case "Vue":
			return "PrimeVue", "Mengikuti frontend Vue: komponen tabel dan form lengkap.", core.SourceDependency, true
		}
		return "Tailwind CSS", "Framework frontend tidak dikenal Foundry; styling utility-first cocok untuk framework apa pun.", core.SourceDependency, true
	case core.TechAPI:
		if has("realtime") {
			return "REST + OpenAPI + WebSocket", "Kebutuhan realtime: update dikirim ke klien lewat WebSocket.", core.SourceNeed, true
		}
		return "REST + OpenAPI", "API umum untuk semua klien; kontrak OpenAPI untuk generate client.", core.SourceDefault, true
	case core.TechWorkers:
		var why []string
		for _, w := range []struct{ need, text string }{{"ai", "panggilan AI yang lama"}, {"webhook", "penerimaan dan pengiriman webhook"}} {
			if has(w.need) {
				why = append(why, w.text)
			}
		}
		switch {
		case len(why) > 0:
			return "Worker terpisah + antrean", "Dikerjakan di luar request: " + strings.Join(why, ", ") + ".", core.SourceNeed, true
		case has("files"):
			return "Job in-process", "Pemrosesan file ringan di luar response, di proses backend yang sama.", core.SourceNeed, true
		}
		return core.TechNone, "Belum ada job latar belakang di kebutuhan; semua proses selesai di request.", core.SourceDefault, true
	case core.TechCache:
		switch {
		case has("realtime"):
			return "Redis", "Kebutuhan realtime: state dan pub/sub bersama antar instance.", core.SourceNeed, true
		case has("high_concurrency") || has("low_latency"):
			return "Redis", "Kebutuhan beban/latency tinggi: cache baca di depan database.", core.SourceNeed, true
		case ch(core.TechWorkers) == workersSeparate:
			return "Redis", "Antrean job untuk worker terpisah.", core.SourceDependency, true
		}
		return core.CacheNone, "Belum ada kebutuhan cache atau state bersama; database cukup.", core.SourceDefault, true
	case core.TechMessaging:
		switch {
		case ch(core.TechArchitecture) == core.ArchMicroservices:
			return "NATS", "Layanan terpisah berkomunikasi lewat event.", core.SourceDependency, true
		case has("realtime") && ch(core.TechCache) == "Redis":
			return "Redis Pub/Sub", "Fan-out update realtime ke semua instance backend; memakai Redis yang sudah ada.", core.SourceNeed, true
		}
		return core.TechNone, "Modul berjalan dalam satu proses; broker belum dibutuhkan.", core.SourceDefault, true
	case core.TechStorage:
		if has("files") {
			return "S3-compatible (S3 / MinIO)", "Ada upload file; disimpan di object storage dengan signed URL.", core.SourceNeed, true
		}
		return core.TechNone, "Tidak ada kebutuhan upload file.", core.SourceDefault, true
	case core.TechSearch:
		switch {
		case !has("search"):
			return core.TechNone, "Filter dan query database biasa cukup.", core.SourceDefault, true
		case ch(core.TechDatabase) == "PostgreSQL":
			return "PostgreSQL full-text search", "Pencarian teks memakai database yang sama; tanpa layanan tambahan.", core.SourceDependency, true
		}
		return "Meilisearch", "Pencarian teks cepat terpisah dari database.", core.SourceNeed, true
	case core.TechDeployment:
		switch {
		case plat.Backend && ch(core.TechArchitecture) == core.ArchMicroservices && has("independent_scaling"):
			return "Kubernetes", "Layanan di-deploy dan diskalakan sendiri-sendiri.", core.SourceDependency, true
		case plat.Backend:
			return "Docker Compose (satu server)", "Satu backend dan database; satu server cukup dan murah dioperasikan.", core.SourceDefault, true
		case strings.Contains(ch(core.TechFrontend), "Next.js"):
			return "Node server / Vercel", "Next.js merender halaman di server.", core.SourceDependency, true
		}
		return "Static hosting + CDN", "Tanpa backend; frontend cukup file statis.", core.SourceDependency, true
	case core.TechObservability:
		if has("high_concurrency") || ch(core.TechArchitecture) == core.ArchMicroservices {
			return "OpenTelemetry + Prometheus + Grafana", "Beban tinggi atau banyak layanan butuh tracing dan metrics terpusat.", core.SourceNeed, true
		}
		return "Log terstruktur + health check", "Skala awal; log terstruktur dan health check cukup.", core.SourceDefault, true
	case core.TechTesting, core.TechQuality:
		tools := toolFor[area]
		var picks, why []string
		for _, l := range languages(s) {
			if t := tools[l]; t != "" && !slices.Contains(picks, t) {
				picks = append(picks, t)
				why = append(why, l+" → "+t)
			}
		}
		if len(picks) == 0 {
			return "", "", "", false
		}
		return strings.Join(picks, ", "), "Satu tool per bahasa yang dipakai: " + strings.Join(why, ", ") + ".", core.SourceDependency, true
	}
	return "", "", "", false
}

const workersSeparate = "Worker terpisah + antrean"

// toolFor is the default testing and quality tool per language.
var toolFor = map[string]map[string]string{
	core.TechTesting: {"TypeScript": "Vitest", "JavaScript": "Vitest", "Go": "go test", "Python": "pytest", "Dart": "flutter_test",
		"Java": "JUnit 5", "Kotlin": "JUnit 5", "C#": "xUnit", "PHP": "PHPUnit", "Rust": "cargo test", "Swift": "XCTest"},
	core.TechQuality: {"TypeScript": "ESLint + Prettier", "JavaScript": "ESLint + Prettier", "Go": "gofmt + go vet + golangci-lint", "Python": "Ruff",
		"Dart": "dart analyze + dart format", "Java": "Checkstyle", "Kotlin": "ktlint", "C#": "dotnet format", "PHP": "PHP-CS-Fixer + PHPStan",
		"Rust": "rustfmt + clippy", "Swift": "SwiftLint"},
}

// languages are the programming languages of the chosen frameworks.
func languages(s *core.TechStack) []string {
	var out []string
	for _, a := range []string{core.TechFrontend, core.TechMobile, core.TechDesktop, core.TechCLI, core.TechBackend} {
		if d := s.Decision(a); d != nil {
			if l := d.Lang(); l != "" && !slices.Contains(out, l) {
				out = append(out, l)
			}
		}
	}
	return out
}

// family is the UI framework the web/desktop UI is written with: React,
// Vue, or another name; "" when there is no web or desktop UI.
func family(s *core.TechStack) string {
	for _, a := range []string{core.TechFrontend, core.TechDesktop} {
		v := normalize(s.Choice(a))
		switch {
		case v == "  ":
			continue
		case strings.Contains(v, " react ") || strings.Contains(v, " next "):
			return "React"
		case strings.Contains(v, " vue ") || strings.Contains(v, " nuxt "):
			return "Vue"
		case strings.Contains(v, " svelte "):
			return "Svelte"
		case strings.Contains(v, " angular "):
			return "Angular"
		}
		return "?"
	}
	return ""
}

func uiFits(value, fam string) bool {
	o := option(core.TechUI, value)
	return o == nil || o.Framework == "" || o.Framework == fam
}

// fits reports whether value in area adds no error to s.
func fits(s *core.TechStack, area, value string) bool {
	c := *s
	c.Decisions = core.CloneDecisions(s.Decisions)
	d := c.Decision(area)
	if d == nil {
		c.Decisions = append(c.Decisions, core.TechDecision{Area: area, Recommended: value})
		d = &c.Decisions[len(c.Decisions)-1]
	}
	before := errorCount(Validate(&c))
	d.Override = &core.TechOverride{Value: value}
	d.Override.Language, d.Override.Runtime = describe(area, value)
	settle(&c, map[string]bool{area: true}, false)
	return errorCount(Validate(&c)) <= before
}

func errorCount(issues []core.TechIssue) int {
	n := 0
	for _, i := range issues {
		if i.Level == "error" {
			n++
		}
	}
	return n
}

// fillAlternatives lists, per decision, up to three other known options
// that fit the other choices, the ones serving the project's needs first.
// Technology already used by the code gets none.
func fillAlternatives(s *core.TechStack) {
	for i := range s.Decisions {
		d := &s.Decisions[i]
		d.Alternatives = nil
		if fromExisting(s, *d) && d.Override == nil {
			continue
		}
		var opts []Option
		for _, o := range Catalog[d.Area] {
			if o.Value != core.TechNone && !strings.EqualFold(o.Value, d.Choice()) && !strings.EqualFold(o.Value, d.Recommended) && fits(s, d.Area, o.Value) {
				opts = append(opts, o)
			}
		}
		score := func(o Option) int {
			n := 0
			for _, need := range o.Needs {
				if slices.Contains(s.Needs, need) {
					n++
				}
			}
			return n
		}
		slices.SortStableFunc(opts, func(a, b Option) int { return score(b) - score(a) })
		for _, o := range opts {
			if len(d.Alternatives) < 3 {
				d.Alternatives = append(d.Alternatives, o.Value)
			}
		}
	}
}

// fromExisting reports whether d was read from the project's code.
func fromExisting(s *core.TechStack, d core.TechDecision) bool {
	return d.Source == core.SourceExisting || d.Source == "" && s.Mode == core.StructureExisting
}

// settle re-evaluates the areas that depend on a changed one, in
// dependency order: a recommendation follows the new choices; a choice of
// the user or of the code is marked for review when flag is set. changed
// grows with the areas whose recommendation followed.
func settle(s *core.TechStack, changed map[string]bool, flag bool) {
	for _, area := range derivedAreas {
		d := s.Decision(area)
		if d == nil {
			continue
		}
		dep := ""
		for _, x := range dependsOn[area] {
			if changed[x] {
				dep = x
				break
			}
		}
		if dep == "" {
			continue
		}
		if (d.Override != nil || fromExisting(s, *d)) && flag {
			d.Review = fmt.Sprintf("%s berubah menjadi %s; pastikan %s masih sesuai.", AreaName(dep), orNone(s.Choice(dep)), d.Choice())
		}
		if fromExisting(s, *d) {
			continue // the code's technology stays the recommendation
		}
		v, why, src, ok := derive(s, area, nil)
		if !ok || strings.EqualFold(v, d.Recommended) {
			continue
		}
		if d.Override != nil {
			// the user's choice stays; the recommendation behind it follows
			d.Recommended, d.Reason, d.Source = v, why, src
			d.Language, d.Runtime = describe(area, v)
			continue
		}
		d.Recommended, d.Reason, d.Source, d.Review = v, why, src, ""
		d.Language, d.Runtime = describe(area, v)
		changed[area] = true
	}
}

// Follow re-evaluates what depends on areas after they changed outside a
// proposal (the structure decides architecture and repository): the
// recommendations follow, the user's and the code's choices are marked for
// review.
func Follow(s *core.TechStack, areas ...string) {
	changed := map[string]bool{}
	for _, a := range areas {
		changed[a] = true
	}
	settle(s, changed, true)
	fillAlternatives(s)
}

func orNone(v string) string {
	if v == "" {
		return "—"
	}
	return v
}

// AreaName is the area's label for messages and documents.
func AreaName(area string) string {
	if n, ok := areaNames[area]; ok {
		return n
	}
	return area
}

var areaNames = map[string]string{
	core.TechFrontend: "Frontend", core.TechUI: "UI library", core.TechMobile: "Mobile", core.TechDesktop: "Desktop", core.TechCLI: "CLI",
	core.TechBackend: "Backend", core.TechAPI: "API protocol", core.TechDatabase: "Database", core.TechCache: "Cache", core.TechMessaging: "Messaging",
	core.TechWorkers: "Worker", core.TechStorage: "File storage", core.TechSearch: "Search", core.TechArchitecture: "Arsitektur",
	core.TechRepository: "Repository", core.TechDeployment: "Deployment", core.TechObservability: "Observability",
	core.TechTesting: "Testing", core.TechQuality: "Lint & format",
}

// fromCode reads the stack of an existing project from its components:
// each area the code shows is a decision with its evidence.
func fromCode(p *core.Project, needs []string, plat core.TechPlatform) *core.TechStack {
	s := &core.TechStack{Mode: core.StructureExisting, Needs: needs, Platform: plat}
	// component kinds are area names, except "web" for the frontend
	area := map[string]string{"web": core.TechFrontend}
	for _, a := range core.TechAreas {
		if a != core.TechFrontend {
			area[a] = a
		}
	}
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
		case core.TechCLI:
			s.Platform.CLI = true
		case core.TechBackend, core.TechDatabase:
			s.Platform.Backend = true
		}
		evidence := c.Path + "/" + c.Evidence
		if c.Path == "." || c.Path == "" {
			evidence = c.Evidence
		}
		if i, ok := byArea[a]; ok {
			d := &s.Decisions[i]
			sep := " + "
			if multiAreas[a] {
				sep = ", "
			}
			if !slices.Contains(parts(a, d.Recommended), c.Tech) && !strings.Contains(d.Recommended, c.Tech) {
				d.Recommended += sep + c.Tech
			}
			if !slices.Contains(d.Evidence, evidence) {
				d.Evidence = append(d.Evidence, evidence)
				d.Reason = "Sudah dipakai di kode: " + strings.Join(d.Evidence, ", ") + "."
			}
			continue
		}
		lang, run := describe(a, c.Tech)
		if c.Language != "" {
			lang = c.Language
		}
		s.Decisions = append(s.Decisions, core.TechDecision{Area: a, Recommended: c.Tech, Language: lang, Runtime: run,
			Reason: "Sudah dipakai di kode: " + evidence + ".", Source: core.SourceExisting, Evidence: []string{evidence}})
		byArea[a] = len(s.Decisions) - 1
	}
	s.Decisions = append(s.Decisions, core.TechDecision{Area: core.TechArchitecture, Recommended: "Struktur yang ada",
		Reason: "Project existing tidak diganti template.", Source: core.SourceExisting})
	if s.Decision(core.TechRepository) == nil {
		s.Decisions = append(s.Decisions, core.TechDecision{Area: core.TechRepository, Recommended: core.RepoSimple, Source: core.SourceExisting,
			Reason: "Tidak ada konfigurasi workspace (npm/pnpm workspaces, go.work, turbo, nx) di kode."})
	}
	return s
}

// Carry keeps the user's overrides (and their pending reviews) from old in
// next for areas that still exist and are still valid; the
// recommendations that depend on them follow.
func Carry(old, next *core.TechStack) {
	if old == nil || next == nil {
		return
	}
	changed := map[string]bool{}
	for _, d := range old.Decisions {
		if d.Override != nil && Override(next, d.Area, d.Override.Value, d.Override.Reason) == nil {
			changed[d.Area] = true
		}
	}
	settle(next, changed, false)
	for _, d := range old.Decisions {
		if n := next.Decision(d.Area); n != nil && d.Review != "" && (n.Override != nil || fromExisting(next, *n)) {
			n.Review = d.Review
		}
	}
	fillAlternatives(next)
	next.History, next.Status, next.Approved = old.History, old.State(), old.Approved
	if old.Proposal != nil {
		next.SetStatus(old.Proposal.Previous) // the proposal was made on the old decisions
	}
}

var ErrExistingReason = errors.New("mengganti teknologi yang sudah dipakai di kode adalah migrasi: tuliskan alasannya")

// Override sets the user's choice for area; an empty value or the
// recommended value removes the override. Replacing a technology the code
// already uses needs a reason.
func Override(s *core.TechStack, area, value, reason string) error {
	if s == nil {
		return errors.New("teknologi belum direkomendasikan")
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
	if value == "" || strings.EqualFold(value, d.Recommended) {
		d.Override, d.Review = nil, ""
		return nil
	}
	if fromExisting(s, *d) && reason == "" {
		return ErrExistingReason
	}
	lang, run := describe(area, value)
	d.Override, d.Review = &core.TechOverride{Value: value, Reason: reason, Language: lang, Runtime: run}, ""
	return nil
}

// languageOf guesses the language of a technology outside the catalog.
func languageOf(value string) string {
	v := normalize(value)
	for _, l := range []struct{ word, lang string }{
		{" rust ", "Rust"}, {" axum ", "Rust"}, {" go ", "Go"}, {" golang ", "Go"}, {" python ", "Python"}, {" django ", "Python"}, {" flask ", "Python"},
		{" java ", "Java"}, {" spring ", "Java"}, {" kotlin ", "Kotlin"}, {" php ", "PHP"}, {" laravel ", "PHP"},
		{" ruby ", "Ruby"}, {" rails ", "Ruby"}, {" net ", "C#"}, {" c# ", "C#"}, {" elixir ", "Elixir"}, {" phoenix ", "Elixir"},
		{" typescript ", "TypeScript"}, {" node ", "TypeScript"}, {" express ", "TypeScript"}, {" swift ", "Swift"}, {" dart ", "Dart"},
		{" flutter ", "Dart"},
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
