package work

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/handrawanw/foundry/internal/core"
)

// NormalizeBlueprint cleans AI output: slug module ids, drop empty rows,
// drop unknown or cyclic depends_on. Nil stays nil.
func NormalizeBlueprint(b *core.Blueprint) *core.Blueprint {
	if b == nil {
		return nil
	}
	b.Modules = normModules(b.Modules)
	ids := map[string]bool{}
	for _, m := range b.Modules {
		ids[m.ID] = true
	}
	var ents []core.Entity
	for _, e := range b.Database.Entities {
		e.Name = strings.TrimSpace(e.Name)
		if e.Name == "" {
			continue
		}
		e.Fields = nonEmpty(e.Fields)
		e.Relates = nonEmpty(e.Relates)
		if e.Module != "" && !ids[slugID(e.Module)] {
			e.Module = slugID(e.Module)
		}
		ents = append(ents, e)
	}
	b.Database.Entities = ents
	var workers []core.Worker
	for _, w := range b.Workers {
		w.Name = strings.TrimSpace(w.Name)
		if w.Name == "" || strings.TrimSpace(w.Job) == "" {
			continue
		}
		switch strings.ToLower(w.Trigger) {
		case "event", "cron", "queue":
			w.Trigger = strings.ToLower(w.Trigger)
		default:
			w.Trigger = "queue"
		}
		workers = append(workers, w)
	}
	b.Workers = workers
	b.Overview.Users = nonEmpty(b.Overview.Users)
	b.Overview.Capabilities = nonEmpty(b.Overview.Capabilities)
	b.Authz.Permissions = nonEmpty(b.Authz.Permissions)
	b.Cache.Store = nonEmpty(b.Cache.Store)
	b.Cache.Never = nonEmpty(b.Cache.Never)
	return b
}

func normModules(in []core.Module) []core.Module {
	var out []core.Module
	seen := map[string]int{}
	for _, m := range in {
		id := slugID(m.ID)
		if id == "" {
			id = slugID(m.Responsibility)
		}
		if id == "" || seen[id] > 0 {
			continue
		}
		seen[id] = 1
		m.ID = id
		m.Responsibility = strings.TrimSpace(m.Responsibility)
		m.Data = nonEmpty(m.Data)
		m.Interface = nonEmpty(m.Interface)
		m.DependsOn = nil
		out = append(out, m)
		if len(out) == 16 {
			break
		}
	}
	valid := map[string]bool{}
	for _, m := range out {
		valid[m.ID] = true
	}
	// Second pass: depends_on that exist and do not form a cycle.
	for _, m := range in {
		id := slugID(m.ID)
		if id == "" {
			id = slugID(m.Responsibility)
		}
		idx := -1
		for j := range out {
			if out[j].ID == id {
				idx = j
				break
			}
		}
		if idx < 0 {
			continue
		}
		var deps []string
		for _, d := range m.DependsOn {
			d = slugID(d)
			if d == "" || d == id || !valid[d] || containsStr(deps, d) {
				continue
			}
			if wouldCycle(out, id, d) {
				continue
			}
			out[idx].DependsOn = append(out[idx].DependsOn, d)
			deps = append(deps, d)
		}
	}
	return out
}

func wouldCycle(mods []core.Module, from, to string) bool {
	graph := map[string][]string{}
	for _, m := range mods {
		graph[m.ID] = append([]string{}, m.DependsOn...)
	}
	graph[from] = append(graph[from], to)
	seen := map[string]bool{}
	var walk func(string) bool
	walk = func(n string) bool {
		if n == from && seen[n] {
			return true
		}
		if seen[n] {
			return false
		}
		seen[n] = true
		for _, d := range graph[n] {
			if d == from || walk(d) {
				return true
			}
		}
		return false
	}
	return walk(to)
}

func slugID(s string) string {
	s = strings.Map(func(r rune) rune {
		if r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
			return unicode.ToLower(r)
		}
		return '-'
	}, strings.TrimSpace(s))
	s = strings.Trim(s, "-")
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	if len(s) > 32 {
		s = strings.TrimRight(s[:32], "-")
	}
	return s
}

func containsStr(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

func fill(dst *string, v string) {
	if strings.TrimSpace(*dst) == "" {
		*dst = v
	}
}

func fillSlice(dst *[]string, v []string) {
	if len(*dst) == 0 {
		*dst = v
	}
}

// CompleteBlueprint fills empty architecture fields from the decided stack
// and the discovery. AI-provided values win. Called after stack recommend
// so ORM, cache and architecture pattern match Foundry's technology choice.
func CompleteBlueprint(p *core.Project) {
	if p == nil {
		return
	}
	b := p.Blueprint
	if b == nil {
		b = &core.Blueprint{}
	}
	d := p.Discovery
	if b.Overview.Goal == "" && d != nil {
		b.Overview.Goal = firstNonEmpty(d.Business, d.Summary)
	}
	if b.Overview.CoreDomain == "" && d != nil {
		b.Overview.CoreDomain = firstNonEmpty(d.Summary, p.Name)
	}
	if len(b.Overview.Users) == 0 && d != nil {
		b.Overview.Users = append([]string{}, d.Users...)
	}
	if len(b.Overview.Capabilities) == 0 {
		for _, f := range p.Features {
			b.Overview.Capabilities = append(b.Overview.Capabilities, f.Name)
		}
	}
	if b.Platform.Reason == "" && p.Platform != nil {
		b.Platform.Reason = p.Platform.Reason
	}
	if len(b.Platform.Devices) == 0 && p.Platform != nil {
		for _, t := range p.Platform.Targets {
			b.Platform.Devices = append(b.Platform.Devices, t.Kind)
		}
	}
	needs := map[string]bool{}
	if d != nil {
		for _, n := range d.TechNeeds {
			needs[strings.ToLower(n)] = true
		}
	}
	if p.Stack != nil {
		for _, n := range p.Stack.Needs {
			needs[strings.ToLower(n)] = true
		}
	}
	fill(&b.Platform.Offline, offlineOf(needs))
	fill(&b.Platform.Realtime, realtimeOf(needs))
	fill(&b.Platform.Deployment, "Container (satu layanan API + klien). Skala horizontal pada API.")

	completeStack(b, p, needs)
	if len(b.Modules) == 0 {
		b.Modules = defaultModules(p, needs)
	} else {
		b.Modules = ensureCrossCutting(b.Modules, p, needs)
	}
	if len(b.Database.Entities) == 0 {
		b.Database.Entities = defaultEntities(p, b.Modules)
	}
	completeAuth(b, needs)
	completeStorage(b, needs)
	completeRealtime(b, needs, p)
	if len(b.Workers) == 0 {
		b.Workers = defaultWorkers(needs, b.Storage.Strategy)
	}
	completeAggregation(b, needs)
	completePerformance(b, needs)
	completeCoding(b, p)
	fillSlice(&b.Docs, defaultDocs)
	fillSlice(&b.Workflow, defaultWorkflow)
	fillSlice(&b.ChangeFlow, defaultChangeFlow)
	fillSlice(&b.AIGuidelines, defaultAIGuidelines)
	p.Blueprint = NormalizeBlueprint(b)
}

func completeStack(b *core.Blueprint, p *core.Project, needs map[string]bool) {
	st := p.Stack
	backend, db, cache, arch := "", "", "", ""
	if st != nil {
		backend, db, cache, arch = st.Choice(core.TechBackend), st.Choice(core.TechDatabase), st.Choice(core.TechCache), st.Choice(core.TechArchitecture)
	}
	if arch != "" {
		b.Architecture.Pattern = arch
	}
	if b.Architecture.Reason == "" && st != nil {
		if d := st.Decision(core.TechArchitecture); d != nil {
			b.Architecture.Reason = d.Reason
		}
	}
	fill(&b.Architecture.Tradeoff, archTradeoff(arch))
	fillSlice(&b.Architecture.EvolveWhen, []string{
		"Domain baru membutuhkan tim dan deploy independen",
		"Beban satu modul mengganggu modul lain secara terukur",
	})
	fill(&b.Architecture.Scale, "Scale-out API stateless; database primary + replica baca bila query laporan berat; worker terpisah untuk job panjang.")
	if db != "" {
		b.Database.Engine = db
	}
	if o := ormOf(backend, db); o != "" {
		b.Database.ORM = o
	}
	fill(&b.Database.ORM, ormOf(backend, db))
	fill(&b.Database.Migrations, "Schema di repo → migration versioned → apply → generate type dari schema.")
	fill(&b.Database.Naming, "Tabel dan kolom snake_case, plural untuk tabel (users, order_items).")
	fill(&b.Database.PK, "UUID v7 (id) sebagai primary key.")
	fill(&b.Database.SoftDelete, "deleted_at timestamptz nullable; query default WHERE deleted_at IS NULL.")
	fill(&b.Database.Timestamps, "created_at dan updated_at timestamptz, diisi aplikasi/DB, UTC.")
	fill(&b.Database.Audit, "created_by, updated_by UUID nullable pada tabel bisnis.")
	fill(&b.Database.Transactions, "Satu use-case = satu transaksi di service; repository tidak commit sendiri.")
	if len(b.Database.Indexing) == 0 {
		b.Database.Indexing = []string{
			"Index setiap foreign key",
			"Index kolom filter/sort yang muncul di list API (status, created_at, owner_id)",
			"Unique pada natural key (email, kode, nomor dokumen)",
		}
	}
	fill(&b.Cache.Use, cacheOf(cache, needs))
	if b.Cache.Use != "" && b.Cache.Use != core.CacheNone && b.Cache.Use != "none" {
		fillSlice(&b.Cache.Store, []string{"session", "permission snapshot", "read-model / summary", "rate limit", "idempotency key"})
		fillSlice(&b.Cache.Never, []string{"password hash", "refresh token mentah", "isi file", "data keuangan yang belum ter-commit", "PII mentah jangka panjang"})
		fill(&b.Cache.Invalidation, "Write-through pada mutasi entity: hapus key by entity id dan daftar terkait. TTL pendek (30–300s) pada read-model.")
	} else {
		fill(&b.Cache.Use, "none")
	}
	fe := ""
	if st != nil {
		fe = st.Choice(core.TechFrontend)
		if v := st.Choice(core.TechDeployment); v != "" {
			b.Platform.Deployment = v
		}
		if v := st.Choice(core.TechMessaging); v != "" && v != core.TechNone {
			b.Realtime.Broker = v
		}
		switch v := st.Choice(core.TechStorage); {
		case v == core.TechNone:
			b.Storage.Strategy, b.Storage.Provider = "none", ""
		case v == "Disk server lokal":
			b.Storage.Strategy, b.Storage.Provider = "local", v
		case v != "":
			b.Storage.Strategy, b.Storage.Provider = "object", v
		}
	}
	completeFrontend(b, fe)
	completeBackend(b, backend, needs)
}

// ResetBlueprintTech clears the blueprint details derived from the changed
// technology areas so CompleteBlueprint derives them again from the new
// decision; nothing else in the blueprint changes.
func ResetBlueprintTech(p *core.Project, changes []core.TechChange) {
	b := p.Blueprint
	if b == nil {
		return
	}
	for _, c := range changes {
		switch c.Area {
		case core.TechFrontend:
			b.Frontend = core.FrontendArch{}
		case core.TechBackend:
			b.Backend = core.BackendArch{}
			b.Database.ORM = ""
		case core.TechDatabase:
			b.Database.ORM = ""
		case core.TechCache:
			b.Cache = core.CacheArch{}
			b.Auth.Session = ""
		case core.TechArchitecture:
			b.Architecture.Reason, b.Architecture.Tradeoff = "", ""
		case core.TechMessaging:
			b.Realtime.Broker = ""
		case core.TechStorage:
			b.Storage = core.StorageArch{}
			b.Workers = nil
		case core.TechWorkers:
			b.Workers = nil
		}
	}
}

func completeFrontend(b *core.Blueprint, fe string) {
	switch {
	case strings.Contains(fe, "Vue"):
		fill(&b.Frontend.State, "Pinia")
		fill(&b.Frontend.ServerState, "TanStack Query")
		fill(&b.Frontend.Forms, "VeeValidate")
		fill(&b.Frontend.Routing, "Vue Router")
	case strings.Contains(fe, "Next"):
		fill(&b.Frontend.State, "Zustand")
		fill(&b.Frontend.ServerState, "TanStack Query")
		fill(&b.Frontend.Forms, "React Hook Form")
		fill(&b.Frontend.Routing, "Next.js App Router")
	default:
		fill(&b.Frontend.State, "Zustand")
		fill(&b.Frontend.ServerState, "TanStack Query")
		fill(&b.Frontend.Forms, "React Hook Form")
		fill(&b.Frontend.Routing, "React Router")
	}
	fill(&b.Frontend.Validation, "Zod, schema dibagi dengan API bila stack TypeScript")
	fill(&b.Frontend.API, "Client typed per resource; tidak fetch langsung dari komponen layar")
	fill(&b.Frontend.Components, "Feature folder + komponen UI generik di packages/ui atau src/components")
}

func completeBackend(b *core.Blueprint, backend string, needs map[string]bool) {
	fill(&b.Backend.Pattern, "controller → service → repository → database. Controller tanpa business rule.")
	switch {
	case strings.Contains(backend, "Nest"):
		fill(&b.Backend.DI, "NestJS module providers")
		fill(&b.Backend.Validation, "class-validator / Zod pada DTO")
		fill(&b.Backend.Auth, "Passport JWT")
		fill(&b.Backend.Background, "BullMQ")
	case strings.Contains(backend, "FastAPI"):
		fill(&b.Backend.DI, "FastAPI Depends")
		fill(&b.Backend.Validation, "Pydantic")
		fill(&b.Backend.Auth, "OAuth2 Password + JWT")
		fill(&b.Backend.Background, "ARQ")
	case strings.Contains(backend, "Go"):
		fill(&b.Backend.DI, "konstruktor explicit, tanpa container")
		fill(&b.Backend.Validation, "validator pada DTO")
		fill(&b.Backend.Auth, "JWT middleware")
		fill(&b.Backend.Background, "asynq")
	default:
		fill(&b.Backend.DI, "constructor injection")
		fill(&b.Backend.Validation, "DTO + schema")
		fill(&b.Backend.Auth, "JWT middleware")
		fill(&b.Backend.Background, "queue worker")
	}
	if !needs["realtime"] && !needs["files"] && b.Backend.Background == "" {
		b.Backend.Background = "inline hanya untuk job < 200ms; selain itu queue"
	}
}

func completeAuth(b *core.Blueprint, needs map[string]bool) {
	if needs["enterprise"] {
		fill(&b.Auth.Method, "password + SSO (OIDC)")
	} else {
		fill(&b.Auth.Method, "email + password")
	}
	fill(&b.Auth.Hashing, "Argon2id")
	fill(&b.Auth.Tokens, "Access JWT pendek (15 menit), di Authorization Bearer")
	fill(&b.Auth.Refresh, "Refresh token opaque, rotate setiap pakai, simpan hash di DB, revoke family on reuse")
	if b.Cache.Use != "" && b.Cache.Use != "none" && b.Cache.Use != core.CacheNone {
		fill(&b.Auth.Session, "Session id di Redis; cookie httpOnly Secure SameSite=Lax")
	} else {
		fill(&b.Auth.Session, "Session di DB; cookie httpOnly Secure SameSite=Lax")
	}
	fill(&b.Authz.Model, "RBAC")
	fill(&b.Authz.Convention, "resource:action (contoh appointment:create)")
	if len(b.Authz.Permissions) == 0 {
		b.Authz.Permissions = []string{"*.read", "*.create", "*.update", "*.delete"}
	}
}

func completeStorage(b *core.Blueprint, needs map[string]bool) {
	if !needs["files"] && b.Storage.Strategy == "" {
		fill(&b.Storage.Strategy, "none")
		return
	}
	if b.Storage.Strategy == "none" {
		return
	}
	fill(&b.Storage.Strategy, "object")
	fill(&b.Storage.Provider, "S3-compatible (AWS S3 / MinIO)")
	fill(&b.Storage.Upload, "Frontend minta izin → backend terbitkan signed PUT URL → upload langsung ke object storage → simpan metadata File di DB")
	fillSlice(&b.Storage.Types, []string{"image", "document", "attachment", "export"})
	fillSlice(&b.Storage.Security, []string{
		"hanya user terautentikasi yang minta upload URL",
		"izin per resource (pemilik atau permission file:read)",
		"file privat: GET lewat signed URL berumur pendek",
		"validasi mime + ukuran di backend sebelum URL terbit",
		"scan virus di worker setelah upload",
	})
	fillSlice(&b.Storage.Processing, []string{"thumbnail gambar", "validasi mime", "virus scan"})
	fill(&b.Storage.Lifecycle, "Hapus object saat metadata soft-deleted melewati retention; backup bucket versioning.")
	fill(&b.Storage.CDN, "CDN hanya untuk file publik; privat tetap signed URL.")
}

func completeRealtime(b *core.Blueprint, needs map[string]bool, p *core.Project) {
	if !needs["realtime"] {
		return
	}
	b.Realtime.Needed = true
	broker := "Redis pub/sub"
	if p.Stack != nil && p.Stack.Choice(core.TechCache) == "Redis" {
		broker = "Redis pub/sub"
	}
	fill(&b.Realtime.Broker, broker)
	fillSlice(&b.Realtime.Events, []string{"<domain>.<entity>.<action> (contoh appointment.created)"})
	fill(&b.Realtime.Retry, "At-least-once, consumer idempotent by event id, dead-letter setelah 5 percobaan")
}

func completeAggregation(b *core.Blueprint, needs map[string]bool) {
	if needs["high_concurrency"] || len(b.Workers) > 0 {
		b.Aggregation.Needed = true
	}
	if !b.Aggregation.Needed {
		return
	}
	fill(&b.Aggregation.Approach, "Write ke DB transaksi → event → worker agregasi → tabel summary / read model → API baca cepat")
	fillSlice(&b.Aggregation.Reads, []string{"dashboard summary", "laporan periode"})
}

func completePerformance(b *core.Blueprint, _ map[string]bool) {
	fill(&b.Performance.Pagination, "Cursor pada list besar (id+created_at); limit default 20, max 100")
	fill(&b.Performance.Caching, "Cache read-model dan opsi referensi; jangan cache hasil otorisasi per-user jangka panjang")
	fill(&b.Performance.Indexing, "Index mengikuti filter list API; EXPLAIN pada query > 50ms")
	fillSlice(&b.Performance.Avoid, []string{
		"N+1 query",
		"agregasi berat di request user",
		"SELECT * pada tabel lebar",
		"file di dalam database",
		"transaksi terbuka lintas I/O jaringan",
	})
}

func completeCoding(b *core.Blueprint, p *core.Project) {
	fillSlice(&b.Coding.Backend, []string{
		"controller tidak berisi business logic",
		"service menangani business rule dan transaksi",
		"repository menangani akses data",
		"DTO untuk input/output dan validasi",
		"satu module = satu domain; dilarang import intern module lain kecuali lewat public interface",
		"proses berat masuk queue, bukan request HTTP",
	})
	fillSlice(&b.Coding.Frontend, []string{
		"struktur per fitur, bukan per jenis file",
		"komponen UI generik terpisah dari layar fitur",
		"semua panggilan API lewat layer client typed",
		"server state di TanStack Query / setara; UI state di store yang dipilih",
		"ikuti design system; tidak menambah token baru tanpa keputusan",
	})
	if b.Coding.API.Style == "" {
		b.Coding.API = core.APIStd{
			Style:      "REST",
			Versioning: "/api/v1",
			Errors:     `{"error":{"code":"appointment_conflict","message":"..."}}`,
			Pagination: `{"data":[],"meta":{"limit":20,"next_cursor":""}}`,
		}
	}
}

func defaultModules(p *core.Project, needs map[string]bool) []core.Module {
	var out []core.Module
	out = append(out, core.Module{
		ID: "identity", Responsibility: "Registrasi, login, sesi, user, role, permission",
		Data:      []string{"User", "Role", "Permission", "Session"},
		Interface: []string{"POST /auth/login", "POST /auth/refresh", "GET /me"},
	})
	for _, f := range p.Features {
		id := slugID(f.Name)
		if id == "" || id == "identity" {
			continue
		}
		var data []string
		if d := p.Discovery; d != nil {
			for _, row := range d.Data {
				if strings.Contains(strings.ToLower(row), strings.ToLower(f.Name)) || strings.Contains(strings.ToLower(row), id) {
					data = append(data, row)
				}
			}
		}
		out = append(out, core.Module{
			ID: id, Responsibility: firstNonEmpty(f.Description, f.Name),
			Data: data, DependsOn: []string{"identity"},
		})
		if len(out) == 12 {
			break
		}
	}
	return ensureCrossCutting(out, p, needs)
}

func ensureCrossCutting(mods []core.Module, _ *core.Project, needs map[string]bool) []core.Module {
	has := map[string]bool{}
	for _, m := range mods {
		has[m.ID] = true
	}
	add := func(m core.Module) {
		if has[m.ID] {
			return
		}
		has[m.ID] = true
		mods = append(mods, m)
	}
	if !has["identity"] {
		add(core.Module{ID: "identity", Responsibility: "Auth, user, role, permission",
			Data: []string{"User", "Role", "Session"}, Interface: []string{"POST /auth/login", "GET /me"}})
	}
	if needs["files"] {
		add(core.Module{ID: "files", Responsibility: "Metadata file, signed URL, lifecycle",
			Data: []string{"File"}, Interface: []string{"POST /files/uploads", "GET /files/:id"}, DependsOn: []string{"identity"}})
	}
	if needs["enterprise"] {
		add(core.Module{ID: "audit", Responsibility: "Jejak aksi user pada resource",
			Data: []string{"AuditLog"}, DependsOn: []string{"identity"}})
	}
	return mods
}

func defaultEntities(p *core.Project, mods []core.Module) []core.Entity {
	var out []core.Entity
	if d := p.Discovery; d != nil {
		for _, row := range d.Data {
			name, fields := splitEntity(row)
			if name == "" {
				continue
			}
			mod := ""
			low := strings.ToLower(name)
			for _, m := range mods {
				if strings.Contains(low, m.ID) || strings.Contains(strings.ToLower(m.Responsibility), low) {
					mod = m.ID
					break
				}
			}
			out = append(out, core.Entity{Name: name, Fields: fields, Module: mod})
		}
	}
	if len(out) == 0 {
		out = append(out, core.Entity{Name: "User", Fields: []string{"id", "email", "password_hash", "role_id", "created_at"}, Module: "identity"})
	}
	return out
}

func splitEntity(row string) (string, []string) {
	row = strings.TrimSpace(row)
	name, rest, ok := strings.Cut(row, ":")
	name = strings.TrimSpace(name)
	if name == "" {
		return "", nil
	}
	if !ok {
		return name, nil
	}
	var fields []string
	for _, f := range strings.Split(rest, ",") {
		f = strings.TrimSpace(f)
		if f != "" {
			fields = append(fields, f)
		}
	}
	return name, fields
}

func defaultWorkers(needs map[string]bool, storage string) []core.Worker {
	var out []core.Worker
	out = append(out, core.Worker{Name: "email", Trigger: "queue", Job: "Kirim email transaksional (verifikasi, reset, notifikasi)"})
	if needs["files"] || (storage != "" && storage != "none") {
		out = append(out, core.Worker{Name: "files", Trigger: "queue", Job: "Thumbnail, validasi mime, virus scan, hapus object expired"})
	}
	if needs["enterprise"] || needs["transactions"] {
		out = append(out, core.Worker{Name: "reports", Trigger: "cron", Job: "Agregasi laporan harian ke tabel summary"})
	}
	return out
}

func ormOf(backend, db string) string {
	mongo := strings.Contains(strings.ToLower(db), "mongo")
	switch {
	case strings.Contains(backend, "Nest") && mongo:
		return "Prisma"
	case strings.Contains(backend, "Nest"):
		return "Prisma"
	case strings.Contains(backend, "FastAPI") && mongo:
		return "Beanie"
	case strings.Contains(backend, "FastAPI"):
		return "SQLAlchemy + Alembic"
	case strings.Contains(backend, "Go") && mongo:
		return "mongo-driver"
	case strings.Contains(backend, "Go"):
		return "sqlc"
	default:
		if mongo {
			return "driver native"
		}
		return "migration SQL + query typed"
	}
}

func cacheOf(choice string, needs map[string]bool) string {
	if choice != "" && choice != core.CacheNone {
		return choice
	}
	if needs["realtime"] || needs["high_concurrency"] {
		return "Redis"
	}
	return "none"
}

func offlineOf(needs map[string]bool) string {
	if needs["offline"] {
		return "partial"
	}
	return "none"
}

func realtimeOf(needs map[string]bool) string {
	switch {
	case needs["high_concurrency"]:
		return "critical"
	case needs["realtime"]:
		return "needed"
	default:
		return "none"
	}
}

func archTradeoff(arch string) string {
	switch arch {
	case core.ArchMicroservices:
		return "Deploy independen, operasional dan kompleksitas jaringan lebih tinggi."
	case core.ArchSimple:
		return "Cepat dibangun; batas domain kabur jika fitur bertambah."
	default:
		return "Satu deploy, batas domain jelas; pecah ke layanan terpisah hanya saat ada bukti kebutuhan."
	}
}

func firstNonEmpty(xs ...string) string {
	for _, x := range xs {
		if strings.TrimSpace(x) != "" {
			return x
		}
	}
	return ""
}

var defaultDocs = []string{
	"docs/README.md", "docs/architecture.md", "docs/database.md", "docs/api.md",
	"docs/security.md", "docs/storage.md", "docs/performance.md", "docs/workflow.md",
	"docs/deployment.md", "docs/adr/",
}

var defaultWorkflow = []string{
	"IDEA", "DISCOVERY", "REQUIREMENT", "DOMAIN ANALYSIS", "TECHNICAL DESIGN",
	"DATABASE DESIGN", "STORAGE DESIGN", "API DESIGN", "UI DESIGN",
	"IMPLEMENTATION", "TESTING", "DOCUMENTATION UPDATE",
}

var defaultChangeFlow = []string{
	"Feature Request", "Impact Analysis", "Database Impact", "API Impact",
	"UI Impact", "Performance Impact", "Storage Impact", "Implementation",
	"Testing", "Documentation Update",
}

var defaultAIGuidelines = []string{
	"Blueprint dan stack adalah source of truth; jangan ganti arsitektur, framework, atau database tanpa keputusan Foundry",
	"Ikuti batas module: data dan logic milik module tidak diakses langsung dari module lain",
	"Jangan membuat folder berdasarkan jenis file (controllers/, services/ global)",
	"Jangan menduplikasi logic; perluas yang sudah ada",
	"Proses berat (laporan, export, resize, email, sync) lewat worker",
	"Jangan simpan file besar di database",
	"Update docs/ setelah mengubah kontrak API, skema, atau alur",
	"Naming konsisten dengan konvensi blueprint (snake_case DB, resource:action permission, event domain.entity.action)",
}

// ArchitectureBrief is the compact contract injected into plan and task
// prompts so coding agents follow the same architecture.
func ArchitectureBrief(p *core.Project) string {
	b := p.Blueprint
	if b == nil {
		return ""
	}
	var s strings.Builder
	s.WriteString("ARCHITECTURE BLUEPRINT (source of truth; do not invent another architecture, stack, or folder layout):\n")
	if b.Overview.Goal != "" {
		fmt.Fprintf(&s, "Goal: %s\n", b.Overview.Goal)
	}
	if b.Overview.CoreDomain != "" {
		fmt.Fprintf(&s, "Core domain: %s\n", b.Overview.CoreDomain)
	}
	if b.Architecture.Pattern != "" {
		fmt.Fprintf(&s, "Architecture: %s — %s\n", b.Architecture.Pattern, b.Architecture.Reason)
	}
	if len(b.Modules) > 0 {
		s.WriteString("Modules:\n")
		for _, m := range b.Modules {
			fmt.Fprintf(&s, "- %s: %s", m.ID, m.Responsibility)
			if len(m.DependsOn) > 0 {
				fmt.Fprintf(&s, " (depends: %s)", strings.Join(m.DependsOn, ", "))
			}
			s.WriteByte('\n')
		}
	}
	if b.Database.Engine != "" {
		fmt.Fprintf(&s, "Database: %s · ORM %s · PK %s\n", b.Database.Engine, b.Database.ORM, b.Database.PK)
	}
	if b.Database.Naming != "" {
		fmt.Fprintf(&s, "DB convention: %s %s %s\n", b.Database.Naming, b.Database.Timestamps, b.Database.SoftDelete)
	}
	if len(b.Database.Entities) > 0 {
		s.WriteString("Entities:\n")
		for _, e := range b.Database.Entities {
			fmt.Fprintf(&s, "- %s [%s] %s\n", e.Name, e.Module, strings.Join(e.Fields, ", "))
		}
	}
	if b.Auth.Method != "" {
		fmt.Fprintf(&s, "Auth: %s · %s · %s\n", b.Auth.Method, b.Auth.Hashing, b.Auth.Tokens)
	}
	if b.Authz.Model != "" {
		fmt.Fprintf(&s, "Authz: %s %s\n", b.Authz.Model, b.Authz.Convention)
	}
	if b.Storage.Strategy != "" && b.Storage.Strategy != "none" {
		fmt.Fprintf(&s, "Storage: %s (%s) upload %s\n", b.Storage.Strategy, b.Storage.Provider, b.Storage.Upload)
	}
	if b.Cache.Use != "" && b.Cache.Use != "none" && b.Cache.Use != core.CacheNone {
		fmt.Fprintf(&s, "Cache: %s. Store: %s. Never: %s\n", b.Cache.Use, strings.Join(b.Cache.Store, ", "), strings.Join(b.Cache.Never, ", "))
	}
	if b.Realtime.Needed {
		fmt.Fprintf(&s, "Realtime: %s events %s retry %s\n", b.Realtime.Broker, strings.Join(b.Realtime.Events, ", "), b.Realtime.Retry)
	}
	if len(b.Workers) > 0 {
		s.WriteString("Workers:\n")
		for _, w := range b.Workers {
			fmt.Fprintf(&s, "- %s [%s] %s\n", w.Name, w.Trigger, w.Job)
		}
	}
	if b.Aggregation.Needed {
		fmt.Fprintf(&s, "Aggregation: %s\n", b.Aggregation.Approach)
	}
	if b.Performance.Pagination != "" {
		fmt.Fprintf(&s, "Performance: %s. Avoid: %s\n", b.Performance.Pagination, strings.Join(b.Performance.Avoid, "; "))
	}
	if b.Coding.API.Style != "" {
		fmt.Fprintf(&s, "API: %s %s errors %s pagination %s\n", b.Coding.API.Style, b.Coding.API.Versioning, b.Coding.API.Errors, b.Coding.API.Pagination)
	}
	if len(b.Coding.Backend) > 0 {
		s.WriteString("Backend rules: " + strings.Join(b.Coding.Backend, "; ") + "\n")
	}
	if len(b.Coding.Frontend) > 0 {
		s.WriteString("Frontend rules: " + strings.Join(b.Coding.Frontend, "; ") + "\n")
	}
	if len(b.AIGuidelines) > 0 {
		s.WriteString("AI rules:\n- " + strings.Join(b.AIGuidelines, "\n- ") + "\n")
	}
	if b.Frontend.State != "" {
		fmt.Fprintf(&s, "Frontend libs: state %s, server state %s, forms %s, validation %s, routing %s\n",
			b.Frontend.State, b.Frontend.ServerState, b.Frontend.Forms, b.Frontend.Validation, b.Frontend.Routing)
	}
	if b.Backend.Pattern != "" {
		fmt.Fprintf(&s, "Backend shape: %s · DI %s · background %s\n", b.Backend.Pattern, b.Backend.DI, b.Backend.Background)
	}
	return s.String()
}
