# Foundry

Universal Software Blueprint & Command Workflow Designer. Foundry mengubah ide, kebutuhan, atau perubahan software menjadi **blueprint** yang terstruktur dan **command workflow**, yaitu instruksi untuk coding agent eksternal dengan dependency yang tervalidasi. Hasilnya dipaket dan diekspor.

```
Prompt → Blueprint → Command Workflow → Review & Revisi → Finalisasi → Export
```

Foundry **merancang dan menyusun instruksi**. Coding agent eksternal (Claude Code, Codex, OpenCode, Qwen Code, …) yang mengimplementasikan. Foundry tidak menulis kode aplikasi, tidak menjalankan build/test/deploy, dan tidak mengeksekusi workflow.

Cakupannya tidak terbatas pada web/backend. Platform yang dikenal: `web`, `android`, `ios`, `desktop`, `backend`, `cli`, `embedded` (firmware, IoT, robotics), dan `other` (library/SDK, pipeline data, sistem ML, infrastruktur, tooling). Artefak blueprint menyesuaikan kebutuhan proyek, bukan template.

## Blueprint

Blueprint adalah sumber kebenaran. Isinya kumpulan artefak (`GET /api/v1/project/blueprint`):

- **Bawaan** (dirender dari data terstruktur, hanya bila ada isinya): `requirements`, `architecture`, `technology`, `ui-design` (Design IR, hanya bila produk punya layar).
- **Khusus proyek** (`project.artifacts`): dokumen Markdown yang ditulis AI saat discovery atau lewat revisi, mis. model domain, kontrak API/event, spesifikasi protokol, desain pipeline, threat model, strategi test. Bisa diedit manual (`PUT /api/v1/project/blueprint/artifacts/{id}`), dihapus, atau direvisi lewat prompt (`POST /api/v1/project/blueprint/revise` `{text, focus}`). Revisi hanya mengubah artefak yang dikembalikan AI.

Setiap artefak punya hash isi. Hash ini dipakai untuk mendeteksi command yang terdampak perubahan.

## Command Workflow

`project.workflow.commands[]`: `id` (CMD-001…), `name`, `objective`, `context`, `inputs`, `instructions` (Markdown), `depends_on`, `artifacts` (id + hash saat ditulis/ditinjau), `expected_outputs`, `acceptance_criteria`, `constraints`, `status` (`draft | needs_review | finalized`, status rancangan, bukan eksekusi).

- **Susun**: `POST /api/v1/project/workflow/generate` (hanya bila belum ada command).
- **Revisi lewat prompt**: `POST /api/v1/project/workflow/revise` `{text, focus}`. AI mengembalikan command yang diubah atau ditambah saja; yang lain tidak disentuh.
- **Edit manual**: `PUT /api/v1/project/workflow/commands/{id}`.
- **Tandai perlu ditinjau / sudah ditinjau**: `POST /api/v1/project/workflow/commands/{id}/review` `{needs_review, reason}`. "Sudah ditinjau" mencatat ulang hash artefak.
- **Validasi** (`internal/modules/workflow`): ID unik, dependency ada, tanpa self-dependency, tanpa siklus (DAG), artefak yang dirujuk ada, field wajib terisi. Jawaban AI atau edit yang tidak valid ditolak utuh. Jawaban AI yang ditolak dicoba ulang sekali dengan alasan penolakan di prompt.
- **Urutan**: command dikelompokkan per **gelombang**. Satu gelombang hanya bergantung pada gelombang sebelumnya, jadi command di dalamnya boleh paralel.
- **Dampak perubahan**: command yang merujuk artefak dengan hash berbeda, atau artefak yang sudah dihapus, dilaporkan "perlu ditinjau" (`GET /api/v1/project/workflow` → `report.stale`/`report.review`). Deteksi hanya berdasarkan relasi command→artefak yang tersimpan.

## Finalisasi & Export

`POST /api/v1/project/finalize` (alias lama `/design/approve`) menyetujui blueprint dan workflow sebagai satu versi. Syaratnya:

- desain UI lengkap bila produk punya layar
- teknologi dan struktur disetujui untuk produk app/server
- workflow ada, valid, dan tidak ada command yang perlu ditinjau

Setelah itu paket ekspor ditulis ke `docs/foundry/` di folder project. `AGENTS.md` ditambahkan di root bila belum ada. Selama final, blueprint dan workflow terkunci. `POST /api/v1/project/reopen` membukanya lagi.

Paket (`GET /api/v1/project/export` untuk daftar, `GET /api/v1/project/export.zip` untuk unduhan bila sudah final; ZIP berisi folder `docs/foundry/`, ekstrak di root repository):

```
README.md            cara memakai paket + daftar artefak + urutan gelombang
manifest.json        format, status, blueprint_hash, workflow_version, artefak, command (dependency, gelombang)
blueprint/<id>.md    setiap artefak
workflow/<CMD>.md    instruksi lengkap per command
workflow/workflow.json
spec.json            spesifikasi terstruktur (termasuk Design IR, artefak, workflow)
adr/0001-stack.md    bila ada keputusan teknologi
design-brief.md      bila produk punya layar
```

File lama di `docs/foundry/blueprint|workflow|adr` yang tidak lagi ada di paket dihapus saat finalisasi berikutnya. File lain di project tidak disentuh.

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
  work/                     prompt + apply job AI: discovery, artefak blueprint, workflow, design, revisi, QA visual, theme, template
  workflow/                 validasi command workflow: DAG, referensi, gelombang, command terdampak
  render/                   screenshot Chrome untuk QA visual rancangan
  spec/                     artefak blueprint bawaan + paket ekspor (Markdown + JSON) ke docs/foundry/
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

UI: satu top bar (menu project, langkah **Prompt · Blueprint · Workflow · Export**, status AI).
- **Blueprint**: daftar artefak di kiri dan satu dokumen di kanan. "Desain UI/UX" membuka workspace desain (`/blueprint/ui`: Desain / Prototipe / Spesifikasi / Tinjau). "Platform & Teknologi" untuk persetujuan stack dan struktur.
- **Workflow**: command per gelombang di kiri dan detail command di kanan, dengan aksi Salin instruksi / Edit / Tandai perlu ditinjau.
- **Export**: finalisasi dan unduhan.

Revisi lewat prompt tersedia di bar bawah setiap workspace. Menutup project hanya melepasnya dari UI; state tetap di `.foundry/`. Project tidak bisa ditutup atau diganti saat agent sedang berjalan.

Dev: `make dev-api` + `make dev-web` (Vite :5173 proxy ke :8080).

## Teknologi

Semua keputusan teknologi dan struktur berada di level rancangan/spesifikasi — Foundry tidak membuat folder atau kode. Discovery → blueprint arsitektur → platform → teknologi → struktur project. Foundry menganalisa bisnis, domain, modul, data, auth, file, worker, dan kebutuhan teknis (`discovery.tech_needs` + kata kunci: realtime, offline, seo, ai, high_concurrency, flexible_schema, transactions, independent_scaling, enterprise, files). Blueprint (domain, data model, API, aturan kode) disimpan di `project.blueprint` dan ditulis ke `docs/` saat finalisasi; stack dipilih deterministik dari profil (`internal/modules/stack/profiles/`: trading, ai-product, mobile-app, enterprise, saas). Kebutuhan sama, rekomendasi stack sama.

- **Struktur** diturunkan dari arsitektur + repository (Simple, Modular Monolith dengan `modules/<fitur>/`, Microservices, Monorepo).
- **Override**: `PUT /api/v1/project/stack` `{area, value, reason}` (`value ""` = kembali ke rekomendasi). Cache/arsitektur/repository harus dari katalog (`GET /api/v1/stack/catalog`). Override tetap dipakai saat platform diubah atau dianalisa ulang (`POST /api/v1/project/stack/recommend`).
- **Satu keputusan**: disimpan di `project.stack`, dipakai prompt design, spesifikasi halaman, spesifikasi final, dan MCP `get_project`/`get_spec`. Terkunci saat rancangan difinalisasi.
- Project existing: teknologi dibaca dari kode, tidak bisa diganti.

## Desain UI (Design IR, bagian dari blueprint bila produk punya layar)

Prompt → discovery → design intelligence → theme → template → adaptasi → Design IR → renderer → prototipe. Design IR satu-satunya sumber untuk pratinjau, prototipe, dan spesifikasi final.

- **Revisi AI**: permintaan ("buat lebih compact") → patch IR dalam scope → QA visual; tidak regenerate project.
- **Kanvas**: mode Desain / Prototipe / Spesifikasi / Tinjau & Finalisasi. Panel Layer (tree IR) + Properti. `POST /api/v1/project/design/edit` mengedit tree IR satu halaman (add/remove/move/update/set_node/wrap/unwrap/set_page) tanpa AI; hasil invalid ditolak utuh; penurunan kualitas dikembalikan sebagai `warnings`. Mode Spesifikasi = spesifikasi implementasi per halaman dari IR (`GET /api/v1/project/design/handoff`).
- **Finalisasi**: lihat bagian Finalisasi & Export. Unduhan satu file: `GET /api/v1/project/spec.md`, `GET /api/v1/project/spec.json`; ringkasan: `GET /api/v1/project/spec`.
- **Versi**: tiap perubahan tersimpan di `.foundry/design-versions.json` (maks 50); bandingkan dan kembalikan via `/api/v1/project/design/versions`.
- **Peta prototipe**: `GET /api/v1/project/design/prototype` — navigasi dari IR, halaman tak terjangkau, halaman buntu, celah alur.
- **Template desain**: 8 pola UX (`internal/modules/work/design-templates/<id>/{intent,layout,rules,examples}.json`): auth-clean, dashboard-executive, workspace-agent, table-management, master-detail, finance-overview, trading-terminal, developer-tool. Foundry merekomendasikan dari domain/peran/kata kunci, AI memilih + menjelaskan alasan (`design.template`), lalu mengadaptasi ke fitur. Template = keputusan struktur, bukan HTML; theme tetap mengatur visual. Lint kategori `template` masuk skor kualitas. Template awal opsional dipilih user sebelum generate (`PUT /api/v1/project/design-template`, `""` = AI memilih, `blank` = tanpa template); pilihan user mengikat AI. Galeri: `GET /api/v1/design-templates?theme=&domain=`. Buat ulang rancangan (fitur/alur sama, tanpa discovery): `POST /api/v1/project/design/regenerate`.
- **Referensi**: website/screenshot hanya menghasilkan Theme DNA; HTML/CSS/aset/konten tidak disalin.
- **Project existing**: setelah discovery Foundry membaca frontend yang ada (`structure.ui`): route (Next.js app/pages, SvelteKit, Nuxt, config router), layout shell + navigasi (samping/atas, label menu), komponen bersama, dan token desain (CSS custom properties, SCSS, Tailwind config; termasuk format shadcn/oklch). Token jadi theme `.foundry/themes/existing-ui/` (yang tidak ada di kode diisi theme bawaan, ditandai di `use`) dan dipakai rancangan bila user belum memilih theme. Prompt design: halaman baru masuk shell + menu yang ada, pakai ulang komponen, jangan buat ulang halaman lama. `existing-ui` ditulis ulang tiap discovery; untuk edit manual salin ke folder theme lain.

## MCP

```bash
./bin/foundry mcp --print-config     # snippet config untuk AI Host
```

Loop host: `next_work` → kerjakan prompt → `submit_work` (jenis: discovery, design, design_qa, reference, stack, structure, blueprint, workflow). Tool baca: `get_project`, `get_requirements`, `get_blueprint` (semua artefak), `get_workflow` (command, gelombang, masalah, yang perlu ditinjau), `get_design`, `get_spec`. Izin: `--allow READ|WRITE|MODIFY|ALL` (default READ; `EXECUTE` masih diterima untuk config lama, tidak dipakai tool mana pun).

## Aturan penting

- Foundry tidak menulis kode aplikasi, tidak menjalankan build/test/deploy, tidak menjalankan coding agent untuk implementasi, dan tidak membuat folder scaffold. Selain state `.foundry/`, file yang ditulis ke project hanya paket ekspor saat finalisasi (`docs/foundry/`, plus `AGENTS.md` bila belum ada).
- Aturan desain (`internal/modules/standard`) menyesuaikan proyek; tidak ada stack atau tool yang diwajibkan. Arsitektur default web/SaaS (REST, ORM, worker) hanya dilengkapi untuk produk app/server.
- Project existing tidak pernah diganti template; rancangan mengikuti UI yang sudah ada.
- Project lama (tahap `PLAN`/`BUILD`/`VERIFY`) dibuka sebagai `READY`; data task/build lama diabaikan dan hilang saat state disimpan berikutnya. Project yang difinalisasi sebelum ada workflow: buka kembali (reopen), susun workflow, lalu finalisasi ulang.
- API hanya menerima request dari host/origin loopback.

## Test

```bash
make test
```

E2E Go memakai test binary sebagai fake agent: prompt → blueprint → desain → workflow → revisi → dampak perubahan → finalisasi (paket di `docs/foundry/`) → buka ulang; workflow melingkar ditolak lalu dicoba ulang; proyek embedded tanpa UI; alur MCP host; tanpa AI; output AI rusak; QA visual; project existing.

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
