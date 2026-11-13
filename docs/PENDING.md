# Pekerjaan yang Belum Selesai

Per 2026-11-13 (finalisasi pasca Phase 7). Mencakup Phase 7 (keandalan Step 9 Teknologi), perbaikan coverage workflow (REQ-025/REQ-031 ParkOS), dan optimasi prompt sketch/revisi.
Kode sudah di-commit user: `2d490fa` (Phase 7), `9c9fa86` (coverage workflow), `87b9efe` (retry, ID, raw, batas pesan), `554a296` (optimasi prompt detail, lifecycle file sementara, telemetry). Optimasi prompt sketch/revisi, guard coverage revisi, dan gofmt belum di-commit.

## A. Workflow — perbaikan coverage MUST

### Harus dikerjakan
- [x] **Proyek asli Parking-System diperbaiki** (2026-11-13). Workflow 25 command dipindah dari run live tervalidasi (`parking-detail2`, kode `554a296`) setelah dicek input identik (requirements, features, blueprint, artefak, keputusan, stack, struktur, story, UI, flows). Ditulis lewat `Store.Update` (lock). Validator resmi: 0 masalah, 26/26 MUST tercakup, semua AC MUST dikutip. Backup: `/home/handrawan/Project/Parking-System.foundry-backup-20261113`. `usage.jsonl` asli tidak digabung (riwayat salinan berbeda).
- [x] **Validasi workflow lengkap terbukti live** (2026-11-12, Haiku, 44m53s, 31 run, $0,69). Outline lolos percobaan pertama. 25/25 command terisi. `Covers`: 26/26 MUST tercakup. `Validate` + `ValidateFeatures` + `CheckUI`: 0 masalah.
  - REQ-025 → CMD-009 (SQLite WAL `synchronous=FULL`, outbox). AC1: uji SIGKILL 200 iterasi.
  - REQ-031 → CMD-002 (AC1, AC2), CMD-021 (AC1, AC2, AC3 gitleaks), CMD-024 (AC1 gateway redirect, AC3 job gitleaks CI).
  - Hasil: `/tmp/claude-1000/-home-handrawan-Project-Foundry/6e73f342-d3ad-4d96-bbb4-925a76766846/scratchpad/wf/parking` (`bench-workflow.json`).

### Optimasi prompt sketch dan revisi (2026-11-13)
- [x] **Sketch**: semua konteks stabil jadi bagian statis (cache); dinamis hanya penolakan + perbaikan coverage. Repo plan ringkas (path + owner). Replay ParkOS: 74,5 → 65,4 KB (−12,2%); dinamis percobaan pertama 73,1 KB → 0,3 KB; retry dinamis 74,8 → 2,0 KB. Live Haiku (1 run): lolos percobaan pertama, 24 command, 0 MUST tak tercakup, cache write 37,7k → 33,4k token, biaya $0,0204 → $0,0169.
- [x] **Revisi**: aturan pindah ke bagian statis; scope fokus = fokus + ronde sebelumnya + dependent langsung, dependent lebih dalam hanya selama muat 60 KB; command di luar scope jadi outline satu baris (output + files untuk dependensi); hash artefak dan `workflow_id` tidak dikirim; artefak non-binding dipotong ke section relevan (BM25) bukan 4 KB awal; sync hanya membaca utuh artefak yang berubah. Replay revisi fokus CMD-009: 261,3 → 194,9 KB (−25,4%), dinamis 128,2 → 62,6 KB (−51,2%). Live Haiku (1 run masing-masing): keduanya selesai 1 ronde, hanya CMD-009 berubah, 0 masalah validasi, 26/26 MUST; cache write 125,6k → 94,0k token.
- [x] **Revisi kini menolak ronde yang menghilangkan coverage MUST** (`lostCoverage`); MUST yang memang sudah tidak tercakup sebelumnya tidak memblokir.
- [ ] **Revisi tanpa fokus masih ~480 KB** (semua command utuh, ~120k token). Tidak dipotong karena AI butuh teks penuh untuk memilih command; perlu desain lain (mis. AI minta teks command) bila workflow makin besar.
- [ ] **Revisi live: run kedua menulis 11 AC untuk CMD-009** (aturan: sekitar 8 lalu dipecah); run lama 8. Satu sampel, belum bisa disimpulkan.

### Kelemahan yang ditemukan, belum diperbaiki
- [x] **Workflow macet di putaran detail terakhir: "CMD-xxx merujuk layout-spec tetapi instruksi tidak menyebut satu pun id halaman"** (roda-sense 2026-11-13). Akar masalah: cek halaman UI hanya jalan saat semua command terisi, padahal putaran terakhir hanya menulis command terakhir dan tidak bisa memperbaiki command putaran sebelumnya; retry mengulang pesan yang sama lalu berhenti. Perbaikan (`work/uipages.go`): putaran detail kini menerima `pages` (id halaman layout-spec); Foundry menulis id itu ke instruksi, menolak id yang tidak ada, melepas design-rules + layout-spec bila `pages: []`, dan mewajibkan command terakhir yang merujuk layout-spec membangun halaman yang belum dibangun. Prompt detail menampilkan daftar halaman beserta command yang sudah membangunnya. Workflow lama yang sudah terlanjur gagal: putaran terakhir diterima, command bermasalah dibuka ulang dengan alasannya. Replay roda-sense: CMD-020 diterima, CMD-018/019 dibuka ulang.
- [x] **Satu percobaan per "Coba lagi".** Diperbaiki: `autoRetry` retry selama pesan penolakan berubah, maks 3 per request (`ChangeRequest.Retries`). "Coba lagi" mereset jatah. Berlaku untuk workflow, artifacts, design. Structure/repo/featuremap masih aturan lama.
- [x] **ID pada pesan penolakan outline.** Diperbaiki: `answerIDs` menulis ulang `CMD-xxx` ke ID jawaban (`C2`).
- [x] **Raw output workflow.** Diperbaiki: `ai-raw/workflow.txt` (jawaban terakhir) dan `ai-raw/workflow-rejected.txt` (jawaban terakhir yang ditolak).
- [x] **Pesan penolakan dipotong 5 masalah.** Diperbaiki: `invalidWorkflow`, batas 40 (`maxIssuesShown`), dipakai sketch dan validasi penuh.
- [x] **Fase detail lambat dan mahal.** Diperbaiki (`workflowdetail.go`, `sections.go`): artefak dipotong ke section relevan (BM25 + ID), anggaran 14 KB per putaran; aturan dipindah ke bagian statis (cache); outline ringkas; repo plan statis tanpa responsibility (node relevan di bagian dinamis). Replay 25 putaran: rata-rata 141,4 → 72,3 KB (−48,9%), dinamis 89,1 → 25,9 KB (−70,9%), maks 238,9 → 79,5 KB. Live Haiku: biaya $0,667 → $0,270, input token per run 68,9k → 36,8k, wall 43 → 26 menit. Target −50% total belum tercapai (−48,9%); sisa terbesar konteks statis (cerita 12 KB, keputusan binding 7 KB, repo plan 6 KB).
- [x] **Putaran detail menjawab ID yang salah.** Diperbaiki: contoh memakai ID putaran + `"id" is exactly CMD-xxx`. Terbukti live: 0 penolakan ID dalam 52 putaran (2 run), sebelumnya 5 dari 30.
- [x] **File sementara tertinggal.** Diperbaiki (`agent/tempfiles.go`): folder per run `$TMPDIR/foundry-agent/run-<pid>-*`, dihapus saat run selesai (sukses, error, timeout, cancel, panic); sweep saat start (`serve`, `mcp`, run pertama) menghapus folder milik PID mati dan semua yang > 6 jam; shutdown menghapus folder proses sendiri. Test memakai TMPDIR terisolasi.
- [x] **Criterion ID karangan** (muncul setelah optimasi, run 1: 11 ID seperti `REQ-031-AC9`). Diperbaiki dengan aturan prompt; run 2: 0.

## C. Arsitektur tanpa katalog/hardcode (2026-11-13)

- [x] **Teknologi**: katalog (`stack.Catalog`), profil (`profiles/*.json`), deteksi kebutuhan dari kata kunci, rekomender deterministik, nilai tertutup arsitektur/repository/cache, aturan kompatibilitas berbasis nama teknologi dihapus. Stack baru kosong sampai rencana AI; existing tetap dibaca dari kode. Override menerima teknologi dan area apa pun. Prompt tidak lagi memuat CATALOG REFERENCE / FOUNDRY BASELINE / PROFILE HINT / KNOWN OPTIONS.
- [x] **Struktur**: tipe tertutup (simple/modular/monorepo/microservices), `Templates`, `/api/v1/templates`, `Recommend`/`Convert`, penurunan folder (`apps/`, `services/`, `packages/`, infix `modules/`) dan validasi/dampak per tipe dihapus. IR menyimpan gaya arsitektur bebas + folder pilihan AI (`path`, `module_paths`); op baru `path`. Struktur pertama berasal dari analisis AI.
- [x] **Blueprint**: `CompleteBlueprint` tidak lagi mengarang modul, entity, auth, ORM, library frontend, aturan coding, worker; hanya menyalin fakta discovery/platform dan pilihan teknologi.
- [x] **Data lama**: `migrateModulePaths` menulis folder modul yang dulu diturunkan. Replay 12 project nyata (read-only): layout identik, 0 error validasi baru.
- [ ] **Belum terbukti live**: alur penuh dengan AI sungguhan (rencana AI kini satu-satunya sumber teknologi dan struktur project baru).
- [ ] **Masih kosakata tetap (bukan resep arsitektur)**: area teknologi bawaan (`core.TechAreas`, AI boleh menambah area lain), jenis bagian IR (app/service/module/worker/package), opsi survey arsitektur (preferensi user, diteruskan ke AI), deteksi teknologi dari manifest di `scan` (fakta kode existing).

## B. Phase 7 — Step 9 Teknologi

### Butuh keputusan user
- [x] **distra: web responsif + PWA diterapkan** (disetujui user 2026-11-13). Lewat API resmi: `SetPlatform` (target web + backend; android/ios dihapus), keputusan stack mobile React Native hilang otomatis, `apps/mobile/` hilang dari struktur, repository dipertahankan Monorepo (override; recommender sempat mengubah ke Simple), DEC-001 diputuskan. REQ-017, REQ-031, features, flows tidak berubah (hash sama). Stack belum di-approve (status `impact_checked`). Backup: `/home/handrawan/Project/distra-project.foundry-backup-20261113`.
- [x] **distra: kebutuhan offline dihapus** (keputusan user 2026-11-13; tidak ada requirement offline). `blueprint.platform.offline` partial → none; aturan UI "Mode offline menampilkan antrean aksi tertunda" dihapus lewat `EditDesignRules`. Requirements, features, flows, stack, platform, keputusan, artefak tidak berubah. Backup: `/home/handrawan/Project/distra-project.foundry-backup-20261113-offline`.
- [x] **Parking: backend Python + FastAPI.** Sudah selesai di proyek asli: stack v2 (2026-11-12 01:18) Go + Gin, approved. Python tidak dibutuhkan sekarang (ANPR lewat kamera vendor; AI = REQ-033 COULD, fase akhir).

### Belum terbukti / belum dikerjakan
- [ ] **Eskalasi alami Haiku → Sonnet** (dipicu stall/regress) belum pernah terjadi live. Hanya terbukti lewat test. Runner Sonnet terbukti live lewat model paksa.
- [ ] **Eskalasi hanya untuk preset `claude`** dengan model yang mengandung `haiku`. Agent lain (Codex, OpenCode, Qwen) tidak punya eskalasi.
- [x] **Step 11 Struktur Repository: retry** (2026-11-13). Repo, analisis struktur, dan feature map kini memakai aturan retry yang sama dengan workflow (`retryBudget`: retry selama penolakan berubah, maks 3; "Coba lagi" mereset). Repair terarah untuk cycle (Kawal 2 masalah) belum ada.
- [x] **Step 11: revisi gagal `remove_node ... tidak ada di struktur`** (roda-sense 2026-11-13, op 10 dari 90). Akar masalah: op 9 memindahkan satu-satunya isi folder implied `lib/`, `Normalize` menghapus folder itu diam-diam, op 10 lalu menghapusnya lagi. `repoplan.Apply` kini mencatat journal per op (`journal.go`): path yang dipindah diikuti ke lokasi baru (termasuk referensi modul, interface, dependency, depends_on), hapus path yang sudah hilang jadi no-op tercatat, pindah path yang sudah hilang ditolak, path yang tidak pernah ada ditolak dengan isi folder terdekat. Replay jawaban asli: 90 op lolos; sisa penolakan 3 `depends_on` teks bebas (validasi sah). Listing plan di prompt tidak lagi menyembunyikan path saat melewati anggaran (deskripsi saja yang dibuang).
- [x] **Step 11: lokasi modul yang bukan node** (roda-sense 2026-11-13, percobaan 2: 10 lokasi `apps/mobile/lib/features/<modul>/` tidak ada di `nodes`). Prompt plan dan revise kini mewajibkan setiap lokasi modul juga jadi node. Validator tidak diubah. Proyek itu sendiri lolos di percobaan berikutnya (01:21).
- [ ] **Job KindTech mandiri** (di luar Blueprint terpadu) masih membuang request saat gagal akhir. Pola draft/resume belum diterapkan di sana.
- [ ] **UI frontend belum menampilkan** riwayat percobaan, progres, dan eskalasi (`TechAttempt`). Data sudah ada di `project.json` dan diagnosis teks.
- [ ] **Jumlah alternatif teknologi turun** (Parking 14/42, kawal 5/35). Lebih aman bagi validator, tapi informasi pembanding berkurang.
- [ ] **Draft rencana teknologi (sekitar 60 KB) disimpan di `project.json`**, sehingga file state membesar.
- [ ] **Sampel benchmark kecil**: 3 proyek sebelum/sesudah, 9 run repair (1–2 run per skenario).

### Teknis
- [ ] Race test `internal/modules/pipeline` butuh `-timeout 40m` (default 10m timeout; terakhir PASS 649,3 detik, 0 data race, 2026-11-12).
- [x] gofmt lama: `learning.go`, `remove.go`, `context_test.go` (hanya spasi; `git diff -w` kosong).

## Bukti
- Ukuran prompt sketch/revisi: `go test -tags foundrybench -run 'TestBench(Sketch|Revision)PromptSize'` (BENCH_PROJECT=…/project.json). Revisi live: `TestBenchWorkflowRevise` (BENCH_DIR, BENCH_REVISE_TEXT, BENCH_REVISE_FOCUS).
- Ukuran prompt detail: `go test -tags foundrybench -run TestBenchDetailPromptSize` (BENCH_PROJECT=…/project.json). Live: `/tmp/claude-1000/-home-handrawan-Project-Foundry/6e73f342-d3ad-4d96-bbb4-925a76766846/scratchpad/wf/parking-detail2`
- Benchmark Step 9: `<scratchpad>/bench/SUMMARY.md`, `out/`, `out2/`, `logs2/`
- Workflow lolos penuh: `/tmp/claude-1000/-home-handrawan-Project-Foundry/6e73f342-d3ad-4d96-bbb4-925a76766846/scratchpad/wf/parking`
- Workflow lama: `<scratchpad>/wf/parking` (detail 14/25), `<scratchpad>/wf/parking-outline` (outline lolos), `<scratchpad>/wf/parking-run1` (gagal aturan subfitur)
- `<scratchpad>` = `/tmp/claude-1000/-home-handrawan-Project-Foundry/1cfd5da9-6391-4bd5-9ca1-600c9057757a/scratchpad`. Lokasi sementara, bisa hilang.
