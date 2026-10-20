# Foundry

Universal Software Blueprint & Command Workflow Designer. Foundry mengubah ide, kebutuhan, atau perubahan software menjadi **blueprint** yang terstruktur dan **command workflow**, yaitu instruksi untuk coding agent eksternal dengan dependency yang tervalidasi. Hasilnya dipaket dan diekspor.

```
Prompt → Blueprint → Command Workflow → Review & Revisi → Finalisasi → Export
```

Foundry **merancang dan menyusun instruksi**. Coding agent eksternal (Claude Code, Codex, OpenCode, Qwen Code, …) yang mengimplementasikan. Foundry tidak menulis kode aplikasi, tidak menjalankan build/test/deploy, dan tidak mengeksekusi workflow.

Cakupannya tidak terbatas pada web/backend. Platform yang dikenal: `web`, `android`, `ios`, `desktop`, `backend`, `cli`, `embedded` (firmware, IoT, robotics), dan `other` (library/SDK, pipeline data, sistem ML, infrastruktur, tooling). Artefak blueprint menyesuaikan kebutuhan proyek, bukan template.

## Blueprint

Blueprint adalah sumber kebenaran. Isinya kumpulan artefak (`GET /api/v1/project/blueprint`):

- **Bawaan** (dirender dari data terstruktur, hanya bila ada isinya): `requirements`, `architecture`, `technology`, `design-rules` dan `layout-spec` (hanya bila produk punya layar; lihat Desain UI), `use-cases`, `acceptance`, `decisions` (lihat Cakupan Blueprint).
- **Khusus proyek** (`project.artifacts`): dokumen Markdown yang ditulis AI saat discovery atau lewat revisi, mis. model domain, kontrak API/event, spesifikasi protokol, desain pipeline, threat model, strategi test. Bisa diedit manual (`PUT /api/v1/project/blueprint/artifacts/{id}`), dihapus, atau direvisi lewat prompt (`POST /api/v1/project/blueprint/revise` `{text, focus}`). Revisi hanya mengubah artefak yang dikembalikan AI.

Setiap artefak punya hash isi. Hash ini dipakai untuk mendeteksi command yang terdampak perubahan. Artefak khusus proyek juga mencatat hash artefak yang dirujuknya (`refs` → `seen`); bila artefak acuan berubah, artefak itu berstatus `needs_review` sampai ditinjau (`POST /api/v1/project/blueprint/artifacts/{id}/review`) atau diedit. Status per artefak (`draft | valid | needs_review | finalized`) ada di `states` pada `GET /api/v1/project/blueprint`.

## Cakupan Blueprint

Blueprint dinilai per area, adaptif terhadap proyek (`core.Areas`, `core.Coverage`):

| Area | Artefak | Berlaku bila |
|---|---|---|
| Aturan desain & layout | `design-rules`, `layout-spec` (terstruktur) | produk punya layar |
| Alur bisnis & use case | `use-cases` (dari `project.flows`) | ada fitur |
| Data & siklus hidup | `data-lifecycle` (Markdown, `area: data`) | ada entitas/data |
| Keamanan & threat model | `threat-model` (`area: security`) | selalu |
| Integrasi eksternal | `integrations` (`area: integration`) | ada integrasi atau webhook |
| Observability & reliability | `operations` (`area: operations`) | selalu |
| Migrasi & kompatibilitas | `migration` (`area: migration`) | project existing |
| Acceptance criteria & DoD | `acceptance` (terstruktur) | ada requirement |

Status area: `covered`, `invalid`, `missing`, `skipped` (dilewati dengan alasan), `not_applicable`. Area yang berlaku harus dicakup atau dilewati dengan alasan; UI dan acceptance tidak bisa dilewati. Artefak Markdown dengan id lain bisa mencakup area lewat field `area`.

- **Use case** (`project.flows[]`): aktor (`role`), `goal`, `preconditions`, `steps`, `postconditions`, `alternatives`/`failures` (minimal satu), `business_rules`, `permissions`, `data_effects`, `integrations`. Requirement berasal dari fitur. Edit: `PUT /api/v1/project/blueprint/use-cases`.
- **Acceptance** (`requirements[].acceptance[]`: `id` REQ-001-AC1, `kind` behavior|negative|edge|security|performance|compatibility|accessibility, `text`, `evidence`; atau `untestable` dengan alasan) dan `definition_of_done`. `evidence` adalah bukti yang wajib diserahkan agent, bukan klaim sudah diuji. Edit: `PUT /api/v1/project/blueprint/acceptance`.
- **Keputusan** (`project.decisions[]`: DEC-001, `kind` assumption|question|conflict|risk, `area`, `critical`, `resolution`). Keputusan kritis tanpa `resolution` memblokir finalisasi. Putuskan/buka lagi: `PUT /api/v1/project/blueprint/decisions/{id}` `{resolution}`.
- **Lewati area**: `PUT|DELETE /api/v1/project/blueprint/areas/{id}/skip` `{reason}`.
- **AI**: discovery menulis semuanya; `POST /api/v1/project/blueprint/complete` meminta AI menulis/memperbaiki area yang `missing`/`invalid` (job `blueprint`). Revisi blueprint juga bisa mengubah use case, acceptance, keputusan, dan skip; item yang diubah AI harus valid (ditolak utuh lalu dicoba ulang sekali), masalah lama yang tidak disentuh tetap jadi blocker.

## Command Workflow

`project.workflow.commands[]`: `id` (CMD-001…), `name`, `objective`, `context`, `inputs`, `instructions` (Markdown), `depends_on`, `artifacts` (id + hash saat ditulis/ditinjau), `covers` (requirement yang diimplementasikan), `expected_outputs`, `acceptance_criteria`, `constraints`, `status` (`draft | needs_review | finalized`, status rancangan, bukan eksekusi).

- **Susun**: `POST /api/v1/project/workflow/generate` (hanya bila belum ada command).
- **Revisi lewat prompt**: `POST /api/v1/project/workflow/revise` `{text, focus}`. AI mengembalikan command yang diubah atau ditambah saja; yang lain tidak disentuh.
- **Edit manual**: `PUT /api/v1/project/workflow/commands/{id}`.
- **Tandai perlu ditinjau / sudah ditinjau**: `POST /api/v1/project/workflow/commands/{id}/review` `{needs_review, reason}`. "Sudah ditinjau" mencatat ulang hash artefak.
- **Validasi** (`internal/modules/workflow`): ID unik, dependency ada, tanpa self-dependency, tanpa siklus (DAG), setiap command merujuk minimal satu artefak dan artefak itu ada, field wajib terisi; bila blueprint punya `design-rules`/`layout-spec`, minimal satu command merujuk masing-masing; `covers` hanya berisi requirement yang ada. Workflow baru dari AI wajib mencakup semua requirement MUST; untuk workflow lama, requirement MUST yang belum dicakup hanya dilaporkan (`report.uncovered`). Jawaban AI atau edit yang tidak valid ditolak utuh. Jawaban AI yang ditolak dicoba ulang sekali dengan alasan penolakan di prompt.
- **Urutan**: command dikelompokkan per **gelombang**. Satu gelombang hanya bergantung pada gelombang sebelumnya, jadi command di dalamnya boleh paralel.
- **Dampak perubahan**: command yang merujuk artefak dengan hash berbeda, atau artefak yang sudah dihapus, dilaporkan "perlu ditinjau" (`GET /api/v1/project/workflow` → `report.stale`/`report.review`). Deteksi hanya berdasarkan relasi command→artefak yang tersimpan.

## Finalisasi & Export

`POST /api/v1/project/finalize` menyetujui blueprint dan workflow sebagai satu versi. Syaratnya:

- aturan desain dan spesifikasi layout ada dan tanpa error bila produk punya layar
- setiap area yang berlaku dicakup tanpa error atau dilewati dengan alasan, tidak ada keputusan kritis yang terbuka, dan tidak ada artefak `needs_review`
- teknologi dan struktur disetujui untuk produk app/server
- workflow ada, valid, dan tidak ada command yang perlu ditinjau

Setelah itu paket ekspor ditulis ke `docs/foundry/` di folder project. `AGENTS.md` ditambahkan di root bila belum ada. Selama final, blueprint dan workflow terkunci. `POST /api/v1/project/reopen` membukanya lagi.

Paket (`GET /api/v1/project/export` untuk daftar, `GET /api/v1/project/export.zip` untuk unduhan bila sudah final; ZIP berisi folder `docs/foundry/`, ekstrak di root repository):

```
README.md            cara memakai paket + daftar artefak + cakupan area + urutan gelombang
manifest.json        format, status, blueprint_hash, workflow_version, artefak, areas, command (dependency, covers, gelombang)
traceability.md      requirement → fitur → use case → kriteria → command
blueprint/<id>.md    setiap artefak
workflow/<CMD>.md    instruksi lengkap per command
workflow/workflow.json
spec.json            spesifikasi terstruktur (termasuk aturan desain, layout, use case, acceptance, keputusan, skip, DoD, artefak, workflow)
adr/0001-stack.md    bila ada keputusan teknologi
design/design-rules.json, design/layout-spec.json   bila produk punya layar (versi JSON artefak desain)
```

File lama di `docs/foundry/blueprint|workflow|adr|design` (dan `design-brief.md` versi lama, `traceability.md` bila tidak ada requirement) yang tidak lagi ada di paket dihapus saat finalisasi berikutnya. File lain di project tidak disentuh.

## Struktur (monorepo modular)

```
cmd/foundry/                  entry: foundry serve | foundry mcp
internal/core/              model Project (satu context) + Store (.foundry/project.json, lock lintas proses)
internal/config/            koneksi AI + recent project (~/.foundry/foundry.json)
internal/modules/
  agent/                    runner CLI agent (Claude Code, Codex, Qwen Code, Grok, Gemini, OpenCode, custom) untuk job AI rancangan
  scan/                     deteksi komponen project existing
  uiscan/                   UI project existing: route, layout shell, navigasi, komponen, token desain dari kode
  stack/                    keputusan teknologi (level spesifikasi): kebutuhan → profil (profiles/*.json) → aturan; override user
  structure/                struktur project (level rancangan): existing vs simple/modular/monorepo/microservices
  work/                     prompt + apply job AI: discovery, artefak blueprint, workflow, aturan desain + layout, revisi
  workflow/                 validasi command workflow: DAG, referensi, gelombang, command terdampak
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
- **Blueprint**: daftar artefak di kiri dan satu dokumen di kanan. "Aturan Desain" dan "Spesifikasi Layout" adalah artefak biasa: dibaca sebagai dokumen, diedit sebagai data (JSON, divalidasi server), direvisi lewat prompt. "Platform & Teknologi" untuk persetujuan stack dan struktur.
- **Workflow**: command per gelombang di kiri dan detail command di kanan, dengan aksi Salin instruksi / Edit / Tandai perlu ditinjau.
- **Export**: finalisasi dan unduhan.

Revisi lewat prompt tersedia di bar bawah setiap workspace. Menutup project hanya melepasnya dari UI; state tetap di `.foundry/`. Project tidak bisa ditutup atau diganti saat agent sedang berjalan.

Dev: `make dev-api` + `make dev-web` (Vite :5173 proxy ke :8080).

## Teknologi

Semua keputusan teknologi dan struktur berada di level rancangan/spesifikasi — Foundry tidak membuat folder atau kode. Discovery → blueprint arsitektur → target platform → teknologi → struktur folder.

- **Target platform** hanya menyatakan di mana produk berjalan (`web`, `android`, `ios`, `desktop`, `backend`, `cli`, …). Pengguna, peran, dan izin ada di requirements (`discovery.roles`) dan security. Project lama dengan `targets[].audience` dimigrasi sekali: peran yang disebut di sana mendapat platform-nya.
- **Grup keputusan** (`core.TechGroups`): Application Stack (frontend, UI library, mobile, desktop, CLI, backend, API protocol; bahasa dan runtime adalah atribut keputusan), Data & Communication (database, cache, messaging, worker, file storage, search), Architecture & Repository (dua keputusan terpisah), Infrastructure & Operations (deployment, observability), Engineering Standards (testing, lint & format per bahasa).
- **Dasar rekomendasi**, dari yang terkuat: kode yang ada → preferensi stack di survey → kebutuhan teknis (`discovery.tech_needs` + kata kunci: realtime, offline, seo, ai, high_concurrency, low_latency, flexible_schema, transactions, independent_scaling, enterprise, files, search, webhook) → kebijakan profil (`internal/modules/stack/profiles/`: trading, ai-product, mobile-app, enterprise, saas; hanya mengisi yang belum ditentukan) → default. Setiap keputusan mencatat `source`. Deterministik: kebutuhan sama, rekomendasi sama.
- **Kondisional**: cache, messaging, worker terpisah, file storage, search, dan observability terpusat hanya direkomendasikan bila ada kebutuhannya; selain itu “Tidak perlu”. Blueprint (event, realtime, worker, caching, health check, gaya API) mengikuti pilihan ini, tidak memaksa Redis.
- **Alternatif**: maksimal tiga opsi katalog yang tetap kompatibel dengan pilihan lain, dengan kecocokan, trade-off, dan konsekuensi operasional (`GET /api/v1/stack/catalog`: `catalog`, `profiles`, `areas`, `groups`).
- **Status per keputusan**: `recommended` (rekomendasi AI, bukan persetujuan), `existing`, `selected` (dipilih user), `approved`, `needs_review`, `conflict` (error validasi).
- **Validasi**: error hanya untuk kombinasi yang tidak bisa jalan (PostgreSQL FTS tanpa PostgreSQL, static hosting dengan backend, UI library untuk framework lain, tRPC dengan backend/klien non-TypeScript, aplikasi native satu platform untuk Android + iOS). Sisanya peringatan.
- **Dependensi**: perubahan satu area menghitung ulang area yang bergantung padanya (UI library ← frontend, API ← backend/klien, cache ← worker, messaging ← arsitektur/cache, testing/lint ← bahasa, …). Rekomendasi ikut berubah (ditandai “ikut berubah” di usulan); pilihan user atau kode tidak diganti, tapi ditandai `needs_review` sampai user tetap memakainya (`POST /api/v1/project/stack/review` `{area}`) atau menggantinya. Usulan menampilkan artefak blueprint yang isinya berubah dan command yang merujuknya; setelah diterapkan command itu berstatus perlu ditinjau lewat hash artefak.
- **Alur**: `PUT /api/v1/project/stack` `{changes:[{area, value, reason}]}` (`value ""` = kembali ke rekomendasi) membuat usulan → `apply` / `discard` → `approve` (diblokir oleh konflik dan `needs_review`). Cache/arsitektur/repository harus dari katalog. Pilihan user tetap dipakai saat platform diubah atau dianalisa ulang (`POST /api/v1/project/stack/recommend`).
- **Struktur folder** diturunkan dari arsitektur + repository (Simple, Modular Monolith dengan `modules/<fitur>/`, Microservices, Monorepo); deployment keputusan terpisah.
- **Project existing**: scan membaca bahasa, framework, database, cache, messaging, UI library, tool test/lint, workspace (repository), build, CI, dan deployment (compose, Dockerfile, k8s, Vercel, …) beserta file buktinya. Teknologi dari kode tetap dipakai; area yang tidak terlihat di kode hanya direkomendasikan untuk bagian baru produk atau kebutuhan teknis. Mengganti teknologi dari kode adalah migrasi dan wajib beralasan. Stack yang seluruhnya dari kode disetujui bersama kode; yang direkomendasikan atau diubah perlu disetujui eksplisit.
- **Satu keputusan**: disimpan di `project.stack`, dipakai prompt, artefak `technology`, spesifikasi final, ADR `docs/foundry/adr/0001-stack.md` (status `accepted` hanya bila disetujui; per keputusan tercatat statusnya), dan MCP `get_project`/`get_spec`. Terkunci saat rancangan difinalisasi.

## Desain UI (aturan desain + spesifikasi layout, bila produk punya layar)

Hanya teks dan data terstruktur: tidak ada gambar, kanvas, editor visual, prototipe visual, atau screenshot. Model: `project.ui` (`internal/core/uidesign.go`).

- **Aturan desain** (`ui.rules` → artefak `design-rules`): `source` (`new` | `existing`), arah visual, prinsip, panduan UX, token (`colors`, `typography`, `spacing` wajib; `radius`, `shadow`, `breakpoints`), komponen (pemakaian, varian, aturan state interaktif, `source` = file komponen yang sudah ada), hierarki visual, aksesibilitas, responsif, interaksi, aturan per platform (web/android/ios/desktop).
- **Spesifikasi layout** (`ui.layout` → artefak `layout-spec`): `shells` (layout bersama) dan `pages`: route, akses, fitur, shell, `pattern` (sidebar, navbar, dashboard, form, table, list, detail, …), `sections` berurutan (isi, komponen, grid, perataan, spasi, ukuran, responsif, state), `hierarchy`, `links` antarhalaman, `states` (`loading`, `empty`, `no_results`, `error`, `success`, `disabled`, `interactive`), data. Alur (`flows[].page_ids`) memetakan halaman per alur.
- **Validasi** (`Project.DesignIssues`): kosakata dikenal, id unik, referensi (shell, link, fitur, alur, platform project) ada, state wajib per pattern (mis. table: loading/empty/error; form: error/success/disabled). Error memblokir simpan dan finalisasi; peringatan (komponen tak terdaftar, fitur tanpa halaman) hanya saran.
- **AI**: setelah discovery, produk dengan layar langsung disusun (job `design`). `GET /api/v1/project/design` (data, issues, kosakata), `POST /api/v1/project/design/generate` (hanya bila belum ada), `POST /api/v1/project/design/revise` `{text, focus}` (focus: `design-rules`, `layout-spec`, id shell/halaman). AI mengembalikan bagian yang berubah saja; jawaban invalid ditolak utuh dan dicoba ulang sekali dengan alasannya.
- **Edit manual**: `PUT /api/v1/project/design/rules`, `PUT /api/v1/project/design/layout` (dokumen utuh; invalid → 400 dengan `details` per masalah). Link dan langkah alur ke halaman yang dihapus ikut dibuang.
- **Dampak**: perubahan mengubah hash artefak → command yang merujuknya dilaporkan perlu ditinjau.
- **Project existing**: Foundry membaca route, layout shell + navigasi, komponen bersama, dan token desain dari kode (CSS custom properties, SCSS, Tailwind config; termasuk shadcn/oklch) ke `structure.ui`. Saat AI menyusun: `source` = `existing`, token dari kode menang atas usulan AI (AI hanya menambah yang belum ada), komponen yang ada didaftarkan dengan file-nya, halaman pada route yang sudah ada ditandai `existing` (diubah seperlunya, tidak dibuat ulang).
- **Migrasi**: project lama dengan Design IR dimigrasi sekali saat dibuka: token dan komponen → aturan desain, halaman → layout (pattern dari jenis halaman, komponen → section, target → link); yang tidak tercatat dulu (prinsip, aksesibilitas, …) dibiarkan kosong dan muncul sebagai error untuk dilengkapi. Command yang merujuk `ui-design` dialihkan ke dua artefak baru dan perlu ditinjau.

## MCP

```bash
./bin/foundry mcp --print-config     # snippet config untuk AI Host
```

Loop host: `next_work` → kerjakan prompt → `submit_work` (jenis: discovery, design, stack, structure, blueprint, workflow). Tool baca: `get_project`, `get_requirements` (termasuk acceptance, use case, DoD), `get_blueprint` (semua artefak, cakupan area, status artefak, keputusan), `get_workflow` (command, gelombang, masalah, yang perlu ditinjau), `get_design` (aturan desain, layout, issues), `get_spec`. Izin: `--allow READ|WRITE|MODIFY|ALL` (default READ; `EXECUTE` masih diterima untuk config lama, tidak dipakai tool mana pun).

## Aturan penting

- Foundry tidak menulis kode aplikasi, tidak menjalankan build/test/deploy, tidak menjalankan coding agent untuk implementasi, dan tidak membuat folder scaffold. Selain state `.foundry/`, file yang ditulis ke project hanya paket ekspor saat finalisasi (`docs/foundry/`, plus `AGENTS.md` bila belum ada).
- Aturan desain (`internal/modules/standard`) menyesuaikan proyek; tidak ada stack atau tool yang diwajibkan. Arsitektur default web/SaaS (REST, ORM, worker) hanya dilengkapi untuk produk app/server.
- Project existing tidak pernah diganti template; aturan desain dan layout mengikuti design system dan UI yang sudah ada.
- Project lama tetap dimuat: requirement/flow tanpa acceptance atau detail use case muncul sebagai area yang perlu dilengkapi (tidak diisi karangan). Artefak tersimpan yang memakai id yang kini bawaan (`use-cases`, `acceptance`, `decisions`) diganti nama menjadi `<id>-doc`; rujukan command ikut, hash tidak berubah.
- Project lama (tahap `PLAN`/`BUILD`/`VERIFY`) dibuka sebagai `READY`; data task/build lama diabaikan dan hilang saat state disimpan berikutnya. Project yang difinalisasi sebelum ada workflow: buka kembali (reopen), susun workflow, lalu finalisasi ulang.
- API hanya menerima request dari host/origin loopback.

## Test

```bash
make test
```

E2E Go memakai test binary sebagai fake agent: prompt → blueprint → desain → workflow → revisi → dampak perubahan → finalisasi (paket di `docs/foundry/`) → buka ulang; workflow melingkar ditolak lalu dicoba ulang; proyek embedded tanpa UI; alur MCP host; tanpa AI; output AI rusak; rancangan UI invalid ditolak lalu dicoba ulang; revisi/edit rancangan menandai command terdampak; migrasi desain lama; project existing; cakupan area (skip, lengkapi dengan AI), keputusan kritis memblokir finalisasi lalu dibuka ulang, artefak `needs_review` saat acuannya berubah, ekspor use case/acceptance/keputusan/traceability, rename artefak lama yang bentrok id bawaan.

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
