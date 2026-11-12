# Pekerjaan yang Belum Selesai

Per 2026-11-12 (diperbarui 22:05 WIB). Mencakup Phase 7 (keandalan Step 9 Teknologi) dan perbaikan coverage workflow (REQ-025/REQ-031 ParkOS).
Kode sudah di-commit user: `2d490fa` (Phase 7) dan `9c9fa86` (coverage workflow). Perbaikan kelemahan workflow (retry, ID, raw, batas pesan, contoh ID detail) belum di-commit.

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
- [ ] **Fase detail lambat dan mahal.** Sekitar 82 detik per command (25 command sekitar 34 menit), prompt 50–240 KB per putaran. Artefak yang dirujuk dimuat utuh (`technology-plan` 72 KB, `repository-structure` 64 KB). Biaya sekitar $0,05 per command dengan Haiku.
- [x] **Putaran detail menjawab ID yang salah.** Akar masalah: contoh balasan prompt detail memakai `"id": "C1"` (ID outline); AI menyalinnya. Bench terakhir: 4 dari 25 putaran ditolak karena ini, semua pulih dengan retry. Diperbaiki: contoh memakai ID putaran + `"id" is exactly CMD-xxx`. Belum dibuktikan live.
- [ ] **File sementara tertinggal** (`/tmp/foundry-system-*.txt`) saat proses agent dibunuh. `TestExecSplitsPrompt` (agent) memeriksa glob `/tmp` global, sehingga gagal bila ada run live bersamaan atau file sisa.

## B. Phase 7 — Step 9 Teknologi

### Butuh keputusan user
- [ ] **distra: mobile dinilai "Tidak perlu"** (DEC-001 PWA) oleh Haiku dan Sonnet. Status `PLATFORM_DECLINED` memblokir approval. Pilih: hapus target mobile, atau minta revisi teknologi.
- [ ] **Parking: backend Python + FastAPI** padahal story menyebut "Backend: Go". Terjadi sebelum dan sesudah perbaikan. Perlu revisi teknologi atau keputusan eksplisit.

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
- [ ] Race test `internal/modules/pipeline` butuh `-timeout 40m` (default 10m timeout; terakhir PASS 649,6 detik, 0 data race).
- [ ] gofmt lama (bukan dari Phase 7): `learning.go`, `remove.go`, `context_test.go`.

## Bukti
- Benchmark Step 9: `<scratchpad>/bench/SUMMARY.md`, `out/`, `out2/`, `logs2/`
- Workflow lolos penuh: `/tmp/claude-1000/-home-handrawan-Project-Foundry/6e73f342-d3ad-4d96-bbb4-925a76766846/scratchpad/wf/parking`
- Workflow lama: `<scratchpad>/wf/parking` (detail 14/25), `<scratchpad>/wf/parking-outline` (outline lolos), `<scratchpad>/wf/parking-run1` (gagal aturan subfitur)
- `<scratchpad>` = `/tmp/claude-1000/-home-handrawan-Project-Foundry/1cfd5da9-6391-4bd5-9ca1-600c9057757a/scratchpad`. Lokasi sementara, bisa hilang.
