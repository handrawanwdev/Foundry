# Foundry

Alat perancangan aplikasi berbasis prompt. User menjelaskan apa yang ingin dibuat, Foundry merancang dan menampilkan preview, user merevisi lewat prompt, lalu memfinalisasi rancangan menjadi spesifikasi untuk diimplementasikan **di luar Foundry**.

```
Hubungkan AI → Prompt → Rancang (preview + revisi) → Finalisasi (spesifikasi final)
```

Foundry **tidak** menulis kode aplikasi, menjalankan build/test, atau mengatur task coding agent. Hasil akhirnya adalah spesifikasi (`docs/` di folder project, unduhan `spec.md`/`spec.json`, atau tool MCP `get_spec`) yang dikonsumsi tool implementasi eksternal.

## Struktur (monorepo modular)

```
cmd/foundry/                  entry: foundry serve | foundry mcp
internal/core/              model Project (satu context) + Store (.foundry/project.json, lock lintas proses)
internal/config/            koneksi AI + recent project (~/.foundry/foundry.json)
internal/modules/
  agent/                    runner CLI agent (Claude Code, Codex, Qwen Code, Grok, Gemini, OpenCode, custom) untuk job AI rancangan
  scan/                     deteksi komponen project existing
  uiscan/                   UI project existing: route, layout shell, navigasi, komponen, token desain → theme
  stack/                    keputusan teknologi (level spesifikasi): kebutuhan → profil (profiles/*.json) → aturan; override user
  structure/                struktur project (level rancangan): existing vs simple/modular/monorepo/microservices
  work/                     prompt + apply: discovery, design, revisi, QA visual, theme, template, spesifikasi halaman
  render/                   screenshot Chrome untuk QA visual rancangan
  spec/                     spesifikasi final: Markdown + JSON, ditulis ke docs/ saat finalisasi
  pipeline/                 orchestrator: job AI, revisi, finalisasi
internal/transport/
  httpapi/                  REST /api/v1 + SSE + SPA
    respond/                helper response standar (success/created/noContent/fail/paginated)
  mcp/                      MCP stdio server (JSON-RPC 2.0)
apps/web/                   React + Vite + Zustand (embed ke binary)
```

## Jalankan

```bash
make build                         # build UI + binary bin/foundry
./bin/foundry serve                  # halaman Start: Project Baru / Buka Project / terakhir
./bin/foundry serve ~/kerja/klinik   # langsung buka project itu
```

UI: menu bar (File: Project Baru, Buka Project, terakhir, Tutup Project), activity bar (Prompt, Rancang, Hubungkan AI), Explorer (pipeline, kebutuhan, modul, fitur, halaman), editor bertab, status bar (AI, tahap, job). Preview tampil langsung di area Rancang (mode Desain / Prototipe / Spesifikasi / Tinjau & Finalisasi). Menutup project hanya melepasnya dari UI; state tetap di `.foundry/`. Project tidak bisa ditutup/diganti saat agent sedang berjalan.

Dev: `make dev-api` + `make dev-web` (Vite :5173 proxy ke :8080).

## Teknologi

Semua keputusan teknologi dan struktur berada di level rancangan/spesifikasi — Foundry tidak membuat folder atau kode. Discovery → blueprint arsitektur → platform → teknologi → struktur project. Foundry menganalisa bisnis, domain, modul, data, auth, file, worker, dan kebutuhan teknis (`discovery.tech_needs` + kata kunci: realtime, offline, seo, ai, high_concurrency, flexible_schema, transactions, independent_scaling, enterprise, files). Blueprint (domain, data model, API, aturan kode) disimpan di `project.blueprint` dan ditulis ke `docs/` saat finalisasi; stack dipilih deterministik dari profil (`internal/modules/stack/profiles/`: trading, ai-product, mobile-app, enterprise, saas). Kebutuhan sama, rekomendasi stack sama.

- **Struktur** diturunkan dari arsitektur + repository (Simple, Modular Monolith dengan `modules/<fitur>/`, Microservices, Monorepo).
- **Override**: `PUT /api/v1/project/stack` `{area, value, reason}` (`value ""` = kembali ke rekomendasi). Cache/arsitektur/repository harus dari katalog (`GET /api/v1/stack/catalog`). Override tetap dipakai saat platform diubah atau dianalisa ulang (`POST /api/v1/project/stack/recommend`).
- **Satu keputusan**: disimpan di `project.stack`, dipakai prompt design, spesifikasi halaman, spesifikasi final, dan MCP `get_project`/`get_spec`. Terkunci saat rancangan difinalisasi.
- Project existing: teknologi dibaca dari kode, tidak bisa diganti.

## Rancangan (Design IR)

Prompt → discovery → design intelligence → theme → template → adaptasi → Design IR → renderer → prototipe. Design IR satu-satunya sumber untuk pratinjau, prototipe, dan spesifikasi final.

- **Revisi AI**: permintaan ("buat lebih compact") → patch IR dalam scope → QA visual; tidak regenerate project.
- **Kanvas**: mode Desain / Prototipe / Spesifikasi / Tinjau & Finalisasi. Panel Layer (tree IR) + Properti. `POST /api/v1/project/design/edit` mengedit tree IR satu halaman (add/remove/move/update/set_node/wrap/unwrap/set_page) tanpa AI; hasil invalid ditolak utuh; penurunan kualitas dikembalikan sebagai `warnings`. Mode Spesifikasi = spesifikasi implementasi per halaman dari IR (`GET /api/v1/project/design/handoff`).
- **Finalisasi**: `POST /api/v1/project/design/approve` mengunci rancangan (butuh teknologi + struktur disetujui, tanpa blocker), lalu menulis spesifikasi final ke `docs/` project (`README.md`, `pages.md`, `spec.json`, `architecture.md`, `database.md`, `api.md`, dll.) plus `FOUNDRY.md`/`AGENTS.md` untuk tool implementasi. `POST /api/v1/project/design/reopen` membuka lagi untuk revisi. Unduhan: `GET /api/v1/project/spec.md`, `GET /api/v1/project/spec.json`; ringkasan: `GET /api/v1/project/spec`.
- **Versi**: tiap perubahan tersimpan di `.foundry/design-versions.json` (maks 50); bandingkan dan kembalikan via `/api/v1/project/design/versions`.
- **Peta prototipe**: `GET /api/v1/project/design/prototype` — navigasi dari IR, halaman tak terjangkau, halaman buntu, celah alur.
- **Template desain**: 8 pola UX (`internal/modules/work/design-templates/<id>/{intent,layout,rules,examples}.json`): auth-clean, dashboard-executive, workspace-agent, table-management, master-detail, finance-overview, trading-terminal, developer-tool. Foundry merekomendasikan dari domain/peran/kata kunci, AI memilih + menjelaskan alasan (`design.template`), lalu mengadaptasi ke fitur. Template = keputusan struktur, bukan HTML; theme tetap mengatur visual. Lint kategori `template` masuk skor kualitas. Template awal opsional dipilih user sebelum generate (`PUT /api/v1/project/design-template`, `""` = AI memilih, `blank` = tanpa template); pilihan user mengikat AI. Galeri: `GET /api/v1/design-templates?theme=&domain=`. Buat ulang rancangan (fitur/alur sama, tanpa discovery): `POST /api/v1/project/design/regenerate`.
- **Referensi**: website/screenshot hanya menghasilkan Theme DNA; HTML/CSS/aset/konten tidak disalin.
- **Project existing**: setelah discovery Foundry membaca frontend yang ada (`structure.ui`): route (Next.js app/pages, SvelteKit, Nuxt, config router), layout shell + navigasi (samping/atas, label menu), komponen bersama, dan token desain (CSS custom properties, SCSS, Tailwind config; termasuk format shadcn/oklch). Token jadi theme `.foundry/themes/existing-ui/` (yang tidak ada di kode diisi theme bawaan, ditandai di `use`) dan dipakai rancangan bila user belum memilih theme. Prompt design: halaman baru masuk shell + menu yang ada, pakai ulang komponen, jangan buat ulang halaman lama. `existing-ui` ditulis ulang tiap discovery; untuk edit manual salin ke folder theme lain.

## MCP

```bash
./bin/foundry mcp --print-config     # snippet config untuk AI Host
```

Loop host: `next_work` → kerjakan prompt → `submit_work`. Tool baca: `get_project`, `get_requirements`, `get_blueprint`, `get_design`, `get_spec` (spesifikasi final untuk implementasi di luar Foundry). Izin: `--allow READ|WRITE|MODIFY|ALL` (default READ; `EXECUTE` masih diterima untuk config lama, tidak dipakai tool mana pun).

## Aturan penting

- Foundry tidak menulis kode aplikasi, tidak menjalankan build/test, dan tidak membuat folder scaffold. Selain state `.foundry/`, satu-satunya file yang ditulis ke project adalah spesifikasi saat finalisasi (`docs/`, dan `FOUNDRY.md`/`AGENTS.md`/`CLAUDE.md` untuk project baru).
- Project existing tidak pernah diganti template; rancangan mengikuti UI yang sudah ada.
- Project lama (tahap `PLAN`/`BUILD`/`VERIFY`) dibuka sebagai `READY` (rancangan final); data task/build lama diabaikan dan hilang saat state disimpan berikutnya.
- API hanya menerima request dari host/origin loopback.

## Test

```bash
make test
```

E2E Go memakai test binary sebagai fake agent: prompt → rancangan → finalisasi (spesifikasi di `docs/`), finalisasi → buka ulang, tanpa AI, output AI rusak, revisi desain, QA visual, project existing, alur MCP host.

## Kontrak Response API

Semua endpoint JSON `/api/v1` memakai satu envelope (`internal/transport/httpapi/respond`). Pengecualian: `GET /api/v1/project/spec.md` / `spec.json` (file unduhan) dan `GET /api/v1/events` (SSE); kegagalannya tetap envelope error.

Sukses:

```json
{ "success": true, "data": {}, "meta": null }
```

Error:

```json
{ "success": false, "error": { "code": "VALIDATION_ERROR", "message": "Request tidak valid", "details": [] } }
```

| HTTP | `error.code`          | Kapan                                                      |
| ---- | --------------------- | ---------------------------------------------------------- |
| 400  | `VALIDATION_ERROR`    | body/parameter tidak valid                                 |
| 403  | `FORBIDDEN`           | host/origin ditolak                                        |
| 404  | `NOT_FOUND`           | resource/proyek tidak ada                                  |
| 409  | `CONFLICT`            | job sibuk, tahap salah, belum ada yang dikerjakan          |
| 412  | `PRECONDITION_FAILED` | AI belum terhubung                                         |
| 500  | `INTERNAL_ERROR`      | kegagalan server                                           |

Helper: `Success(w, data, meta?)` 200 · `Created(w, data)` 201 · `NoContent(w)` 204 · `Fail(w, code, message, status, details...)` · `Paginated(w, data, Pagination{Page, Limit, Total})` → `meta` berisi `page`, `limit`, `total`, `totalPages` (dihitung). Klien bercabang pada `error.code`, bukan teks `message`. Frontend meng-unwrap `data` di `apps/web/src/api.ts`.
