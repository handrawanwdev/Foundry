# Pekerjaan yang Belum Selesai

Per 2026-11-12 (diperbarui 23:30 WIB). Mencakup Phase 7 (keandalan Step 9 Teknologi) dan perbaikan coverage workflow (REQ-025/REQ-031 ParkOS).
Kode sudah di-commit user: `2d490fa` (Phase 7), `9c9fa86` (coverage workflow), `87b9efe` (retry, ID, raw, batas pesan). Optimasi prompt detail, lifecycle file sementara, dan telemetry belum di-commit.

## A. Workflow — perbaikan coverage MUST

### Harus dikerjakan
- [ ] **Proyek asli Parking-System belum diperbarui** (`job.status=failed`, `signal: killed`, workflow kosong). Salinan sudah lolos penuh (lihat di bawah). Pilih: salin `project.json` + `usage.jsonl` + `ai-raw/` dari salinan ke proyek asli (backup dulu), atau build ulang Foundry lalu tekan "Coba lagi".
- [x] **Validasi workflow lengkap terbukti live** (2026-11-12, Haiku, 44m53s, 31 run, $0,69). Outline lolos percobaan pertama. 25/25 command terisi. `Covers`: 26/26 MUST tercakup. `Validate` + `ValidateFeatures` + `CheckUI`: 0 masalah.
  - REQ-025 → CMD-009 (SQLite WAL `synchronous=FULL`, outbox). AC1: uji SIGKILL 200 iterasi.
  - REQ-031 → CMD-002 (AC1, AC2), CMD-021 (AC1, AC2, AC3 gitleaks), CMD-024 (AC1 gateway redirect, AC3 job gitleaks CI).
  - Hasil: `/tmp/claude-1000/-home-handrawan-Project-Foundry/6e73f342-d3ad-4d96-bbb4-925a76766846/scratchpad/wf/parking` (`bench-workflow.json`).

### Kelemahan yang ditemukan, belum diperbaiki
- [x] **Satu percobaan per "Coba lagi".** Diperbaiki: `autoRetry` retry selama pesan penolakan berubah, maks 3 per request (`ChangeRequest.Retries`). "Coba lagi" mereset jatah. Berlaku untuk workflow, artifacts, design. Structure/repo/featuremap masih aturan lama.
- [x] **ID pada pesan penolakan outline.** Diperbaiki: `answerIDs` menulis ulang `CMD-xxx` ke ID jawaban (`C2`).
- [x] **Raw output workflow.** Diperbaiki: `ai-raw/workflow.txt` (jawaban terakhir) dan `ai-raw/workflow-rejected.txt` (jawaban terakhir yang ditolak).
- [x] **Pesan penolakan dipotong 5 masalah.** Diperbaiki: `invalidWorkflow`, batas 40 (`maxIssuesShown`), dipakai sketch dan validasi penuh.
- [x] **Fase detail lambat dan mahal.** Diperbaiki (`workflowdetail.go`, `sections.go`): artefak dipotong ke section relevan (BM25 + ID), anggaran 14 KB per putaran; aturan dipindah ke bagian statis (cache); outline ringkas; repo plan statis tanpa responsibility (node relevan di bagian dinamis). Replay 25 putaran: rata-rata 141,4 → 72,3 KB (−48,9%), dinamis 89,1 → 25,9 KB (−70,9%), maks 238,9 → 79,5 KB. Live Haiku: biaya $0,667 → $0,270, input token per run 68,9k → 36,8k, wall 43 → 26 menit. Target −50% total belum tercapai (−48,9%); sisa terbesar konteks statis (cerita 12 KB, keputusan binding 7 KB, repo plan 6 KB).
- [x] **Putaran detail menjawab ID yang salah.** Diperbaiki: contoh memakai ID putaran + `"id" is exactly CMD-xxx`. Terbukti live: 0 penolakan ID dalam 52 putaran (2 run), sebelumnya 5 dari 30.
- [x] **File sementara tertinggal.** Diperbaiki (`agent/tempfiles.go`): folder per run `$TMPDIR/foundry-agent/run-<pid>-*`, dihapus saat run selesai (sukses, error, timeout, cancel, panic); sweep saat start (`serve`, `mcp`, run pertama) menghapus folder milik PID mati dan semua yang > 6 jam; shutdown menghapus folder proses sendiri. Test memakai TMPDIR terisolasi.
- [x] **Criterion ID karangan** (muncul setelah optimasi, run 1: 11 ID seperti `REQ-031-AC9`). Diperbaiki dengan aturan prompt; run 2: 0.

## B. Phase 7 — Step 9 Teknologi

### Butuh keputusan user
- [ ] **distra: mobile dinilai "Tidak perlu"** (DEC-001 PWA). Rekomendasi: web responsif + PWA, hapus target android/ios (REQ-031 menyebut browser Chrome/Safari; REQ-017 foto/tanda tangan/lokasi bisa lewat browser; tidak ada requirement offline, push, atau barcode). Menunggu persetujuan user.
- [x] **Parking: backend Python + FastAPI.** Sudah selesai di proyek asli: stack v2 (2026-11-12 01:18) Go + Gin, approved. Python tidak dibutuhkan sekarang (ANPR lewat kamera vendor; AI = REQ-033 COULD, fase akhir).

### Belum terbukti / belum dikerjakan
- [ ] **Eskalasi alami Haiku → Sonnet** (dipicu stall/regress) belum pernah terjadi live. Hanya terbukti lewat test. Runner Sonnet terbukti live lewat model paksa.
- [ ] **Eskalasi hanya untuk preset `claude`** dengan model yang mengandung `haiku`. Agent lain (Codex, OpenCode, Qwen) tidak punya eskalasi.
- [ ] **Step 11 Struktur Repository**: retry masih sekali, tanpa repair terarah. Kawal gagal 19 → 2 masalah (cycle).
- [ ] **Job KindTech mandiri** (di luar Blueprint terpadu) masih membuang request saat gagal akhir. Pola draft/resume belum diterapkan di sana.
- [ ] **UI frontend belum menampilkan** riwayat percobaan, progres, dan eskalasi (`TechAttempt`). Data sudah ada di `project.json` dan diagnosis teks.
- [ ] **Jumlah alternatif teknologi turun** (Parking 14/42, kawal 5/35). Lebih aman bagi validator, tapi informasi pembanding berkurang.
- [ ] **Draft rencana teknologi (sekitar 60 KB) disimpan di `project.json`**, sehingga file state membesar.
- [ ] **Sampel benchmark kecil**: 3 proyek sebelum/sesudah, 9 run repair (1–2 run per skenario).

### Teknis
- [ ] Race test `internal/modules/pipeline` butuh `-timeout 40m` (default 10m timeout; terakhir PASS 649,3 detik, 0 data race, 2026-11-12).
- [ ] gofmt lama (bukan dari Phase 7): `learning.go`, `remove.go`, `context_test.go`.

## Bukti
- Ukuran prompt detail: `go test -tags foundrybench -run TestBenchDetailPromptSize` (BENCH_PROJECT=…/project.json). Live: `/tmp/claude-1000/-home-handrawan-Project-Foundry/6e73f342-d3ad-4d96-bbb4-925a76766846/scratchpad/wf/parking-detail2`
- Benchmark Step 9: `<scratchpad>/bench/SUMMARY.md`, `out/`, `out2/`, `logs2/`
- Workflow lolos penuh: `/tmp/claude-1000/-home-handrawan-Project-Foundry/6e73f342-d3ad-4d96-bbb4-925a76766846/scratchpad/wf/parking`
- Workflow lama: `<scratchpad>/wf/parking` (detail 14/25), `<scratchpad>/wf/parking-outline` (outline lolos), `<scratchpad>/wf/parking-run1` (gagal aturan subfitur)
- `<scratchpad>` = `/tmp/claude-1000/-home-handrawan-Project-Foundry/1cfd5da9-6391-4bd5-9ca1-600c9057757a/scratchpad`. Lokasi sementara, bisa hilang.
