# Foundry

Satu pipeline dari ide sampai aplikasi siap diperiksa:

```
Hubungkan AI → Ceritakan → Rancang → Bangun → Siap
   (connect)    (discovery)  (design+plan) (kanban+agent+build+test) (verify)
```

Foundry = orchestrator + source of truth. AI Agent = executor. MCP = bridge. User = decision maker.

## Struktur (monorepo modular)

```
cmd/foundry/                  entry: foundry serve | foundry mcp
internal/core/              model Project (satu context) + Store (.foundry/project.json, lock lintas proses)
internal/config/            koneksi AI + recent project (~/.foundry/foundry.json)
internal/modules/
  agent/                    runner CLI agent (Claude Code, Codex, Qwen Code, Grok, Gemini, custom)
  scan/                     deteksi komponen + command build/test nyata
  uiscan/                   UI project existing: route, layout shell, navigasi, komponen, token desain → theme
  stack/                    keputusan teknologi: kebutuhan → profil (profiles/*.json) → aturan; override user
  structure/                existing vs template baru (simple/modular/monorepo/microservices), scaffold
  work/                     prompt + apply: discovery, design, plan, task, review (dipakai agent & MCP)
  checks/                   jalankan command allowlist, catat hasil, deteksi file berubah
  verify/                   traceability REQ→FEAT→PAGE→TASK→TEST→BUILD + dokumentasi
  pipeline/                 orchestrator: job, kanban gate, build loop, verify, fix
internal/transport/
  httpapi/                  REST /api/v1 + SSE + SPA
  mcp/                      MCP stdio server (JSON-RPC 2.0)
apps/web/                   React + Vite + Zustand (embed ke binary)
```

## Jalankan

```bash
make build                         # build UI + binary bin/foundry
./bin/foundry serve                  # halaman Start: Project Baru / Buka Project / terakhir
./bin/foundry serve ~/kerja/klinik   # langsung buka project itu
```

UI bergaya Visual Studio: menu bar (File: Project Baru, Buka Project, terakhir, Tutup Project), activity bar, Explorer (file project read-only, pipeline, kebutuhan, fitur, halaman, task), editor bertab, status bar (AI, tahap, job, task, verifikasi). Menutup project hanya melepasnya dari UI; state tetap di `.foundry/`. Project tidak bisa ditutup/diganti saat agent sedang berjalan.

Dev: `make dev-api` + `make dev-web` (Vite :5173 proxy ke :8080).

## Teknologi

Discovery → platform → teknologi → struktur project. Foundry menganalisa kebutuhan teknis (`discovery.tech_needs` + kata kunci: realtime, offline, seo, ai, high_concurrency, flexible_schema, transactions, independent_scaling, enterprise), memilih profil (`internal/modules/stack/profiles/`: trading, ai-product, mobile-app, enterprise, saas), lalu menentukan frontend, mobile, desktop, backend, database, cache, arsitektur, dan repository beserta alasannya. Deterministik: kebutuhan sama, rekomendasi sama.

- **Struktur** diturunkan dari arsitektur + repository (Simple, Modular Monolith dengan `modules/<fitur>/`, Microservices, Monorepo).
- **Override**: `PUT /api/v1/project/stack` `{area, value, reason}` (`value ""` = kembali ke rekomendasi). Cache/arsitektur/repository harus dari katalog (`GET /api/v1/stack/catalog`). Override tetap dipakai saat platform diubah atau dianalisa ulang (`POST /api/v1/project/stack/recommend`).
- **Satu keputusan**: disimpan di `project.stack`, dipakai prompt design, plan, task, handoff Kode, dokumentasi, dan MCP `get_project`. Terkunci saat rancangan disetujui.
- Project existing: teknologi dibaca dari kode, tidak bisa diganti.

## Rancangan (Design IR)

Prompt → discovery → design intelligence → theme → template → adaptasi → Design IR → renderer → prototipe. Design IR satu-satunya sumber untuk pratinjau, prototipe, dan build.

- **Revisi AI**: permintaan ("buat lebih compact") → patch IR dalam scope → QA visual; tidak regenerate project.
- **Kanvas**: mode Desain / Prototipe / Kode / Tinjau. Panel Layer (tree IR) + Properti. `POST /api/v1/project/design/edit` mengedit tree IR satu halaman (add/remove/move/update/set_node/wrap/unwrap/set_page) tanpa AI; hasil invalid ditolak utuh; penurunan kualitas dikembalikan sebagai `warnings`. Mode Kode = handoff implementasi per halaman dari IR.
- **Versi**: tiap perubahan tersimpan di `.foundry/design-versions.json` (maks 50); bandingkan dan kembalikan via `/api/v1/project/design/versions`.
- **Peta prototipe**: `GET /api/v1/project/design/prototype` — navigasi dari IR, halaman tak terjangkau, halaman buntu, celah alur.
- **Template desain**: 8 pola UX (`internal/modules/work/design-templates/<id>/{intent,layout,rules,examples}.json`): auth-clean, dashboard-executive, workspace-agent, table-management, master-detail, finance-overview, trading-terminal, developer-tool. Foundry merekomendasikan dari domain/peran/kata kunci, AI memilih + menjelaskan alasan (`design.template`), lalu mengadaptasi ke fitur. Template = keputusan struktur, bukan HTML; theme tetap mengatur visual. Lint kategori `template` masuk skor kualitas. Template awal opsional dipilih user sebelum generate (`PUT /api/v1/project/design-template`, `""` = AI memilih, `blank` = tanpa template); pilihan user mengikat AI. Galeri: `GET /api/v1/design-templates?theme=&domain=`. Buat ulang rancangan (fitur/alur sama, tanpa discovery): `POST /api/v1/project/design/regenerate`.
- **Referensi**: website/screenshot hanya menghasilkan Theme DNA; HTML/CSS/aset/konten tidak disalin.
- **Project existing**: setelah discovery Foundry membaca frontend yang ada (`structure.ui`): route (Next.js app/pages, SvelteKit, Nuxt, config router), layout shell + navigasi (samping/atas, label menu), komponen bersama, dan token desain (CSS custom properties, SCSS, Tailwind config; termasuk format shadcn/oklch). Token jadi theme `.foundry/themes/existing-ui/` (yang tidak ada di kode diisi theme bawaan, ditandai di `use`) dan dipakai rancangan bila user belum memilih theme. Prompt design/plan/task: halaman baru masuk shell + menu yang ada, pakai ulang komponen, jangan buat ulang halaman lama. `existing-ui` ditulis ulang tiap discovery; untuk edit manual salin ke folder theme lain.

## MCP

```bash
./bin/foundry mcp --print-config     # snippet config untuk AI Host
```

Loop host: `next_work` → kerjakan → `submit_work` / `submit_task`. Izin: `--allow READ|WRITE|MODIFY|EXECUTE|ALL` (default READ).

## Aturan penting

- **Bangun paralel**: saat list task ada, Foundry menganalisa TODO yang dependensinya selesai, file/halamannya tidak dipegang agent lain, dan tidak bentrok satu sama lain, lalu mengerjakannya paralel (default 3 agent, maks 8, `FOUNDRY_AGENTS`). Kanban menandai task siap paralel vs yang menunggu dependensi atau file.
- Task `DONE` hanya jika build/test project lulus (exit code) **dan** semua acceptance criteria terpenuhi dengan bukti.
- Command verifikasi hanya dari tooling project: Go → `go build/vet/test ./...`; Node → script `typecheck/lint/build/test` yang ada; Flutter → `flutter analyze/test` bila terpasang.
- Project existing tidak pernah diganti template.
- API hanya menerima request dari host/origin loopback.

## Test

```bash
make test
```

E2E Go memakai test binary sebagai fake agent: happy path sampai READY, tanpa AI, output AI rusak, revisi desain, test gagal → BLOCKED → Perbaiki Otomatis, reviewer menolak, agent crash, project existing, alur MCP host.
