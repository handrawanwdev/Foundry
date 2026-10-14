# Efisiensi token

Prinsip: konteks minimum yang tetap lengkap. Aturan wajib, security, kriteria, dan test tidak dibuang demi token.

## Yang aktif

| Fitur | Perilaku | Fallback aman |
|---|---|---|
| Konteks task (`work/context.go`) | Blueprint dipersempit ke modul yang disebut task atau task dependency-nya, ditambah dependency modul secara transitif dan entity berelasi. | Task `scaffold`/`integration` atau tanpa modul cocok → blueprint penuh. |
| Aturan kondisional | Global (selalu): aturan 1, 2, 3 (ORM), 5 (kontrak event), 8 (struktur), stop-and-ask, WORK RULES, auth/authz/API/coding rules di blueprint, acceptance criteria. Kondisional: aturan 7 frontend hanya task UI/halaman; Redis, latency, envelope/outbox hanya task non-UI; webhook hanya bila task menyebutnya atau area tidak spesifik. | Area `scaffold`, `integration`, `test`, atau tak dikenal → semua aturan. |
| Versi aturan | `standard.Version()` = hash aturan penuh; `work.rulesVersion()` menambah WORK/UI RULES. Masuk ke hash kontrak Task Memory, jadi mengubah aturan membuang memory lama. Test `TestRulesPinnedAndComposable` mengunci hash: mengubah aturan harus disengaja. | — |
| Task Memory (`work/memory.go`) | Task DONE menyimpan ringkasan bukti kriteria, hash isi file, ukuran source, dan hash kontrak (kriteria + brief arsitektur + versi aturan). Dependent menerimanya hanya bila **layak**: source ≥ 3000 byte dan ringkasan ≤ ¼ source; blok dibatasi 1600 karakter. | File berubah / file baru / kontrak berubah → ringkasan dibuang, agent disuruh membaca source. Source kecil → tidak ada memory (membaca source lebih murah). |
| Review jalur worktree | Reviewer membaca git diff (per file ≤ 6000 B, total ≤ 24000 B; biner hanya disebut; file di luar budget tetap disebut namanya). | — |
| Review jalur shared (tanpa worktree) | `checks.Capture` menyimpan hash (dan salinan "sebelum" file teks ≤ 64 KB) tanpa menyentuh tree. `Compare` melaporkan added / modified (diff baris) / deleted / renamed (hash sama) / biner. File milik task lain yang sedang berjalan disaring dan disebut di catatan reviewer. | File besar / tanpa salinan "sebelum" ditandai `diff not available`, bukan dihilangkan. Agent yang menulis ulang isi identik tetap dinilai lewat file yang disentuh. |
| Retry guard | Kegagalan build/test atau kriteria identik dua kali → BLOCKED lebih awal. | Agent crash, timeout, limit, "tidak ada file berubah", dan output bernuansa lingkungan (timeout, port terpakai, 5xx, OOM) tetap dapat 3 percobaan. |

## Metrik: aktual vs estimasi

Satu **run** = satu proses agent (eksekusi, perbaikan di tempat, atau review; retry dihitung sendiri). Setiap run dicatat ke task dan ke `Project.BuildMetrics` (total build, direset saat build dimulai). Preflight/probe model tidak dicatat.

- **Terukur persis:** karakter prompt/output, durasi, jumlah run, run gagal.
- **Aktual (dilaporkan CLI):** `input_tokens` (input segar), `cache_write_tokens`, `cached_tokens` (cache-read), `output_tokens`, `cost_usd`. Empat jenis token **terpisah dan tidak tumpang tindih**; input diproses = jumlah tiga jenis input. Dijumlahkan hanya dari run yang punya data (`usage_runs`). Adapter: **Claude Code** saja (`claude -p --output-format json`, format diverifikasi pada 2.1.x; hanya `usage`, bukan `modelUsage`, agar tidak dihitung dua kali). Biaya dari `total_cost_usd` CLI, tidak dihitung Foundry.
- **Estimasi:** `est_tokens` = karakter/4, ditambahkan **hanya untuk run tanpa data provider**. Run dengan data aktual tidak menambah estimasi.
- Codex, Qwen, Grok, Gemini, OpenCode, command custom: tanpa adapter (format usage belum diverifikasi) → hanya terukur persis + estimasi. Output tak dikenal dilewatkan apa adanya.
- Panggilan AI di luar build (discovery, desain, rencana) belum dicatat.

## Mengukur penghematan

```bash
foundry metrics [--json] ~/kerja/proyek    # hanya membaca .foundry/project.json
```

Isi: (1) ukuran prompt task, karakter persis, baseline (blueprint + aturan penuh) vs sekarang, plus kontribusi task memory; (2) metrik build dan per task, dengan AKTUAL dan ESTIMASI terpisah. Selisih karakter prompt **bukan** bukti penghematan token: sebagian besar token berasal dari agent membaca file dan memakai tool.

### Prosedur benchmark (versi lama vs baru)

Syarat sebanding. Jalankan hanya dengan persetujuan eksplisit: build memakai kuota/biaya AI.

1. Salin project ke dua folder dari commit yang sama (`git worktree add ../bench-old <commit>`), jangan memakai project asli. State `.foundry/` sama (rencana, task, design) di kedua salinan.
2. Bangun dua binary: `bin/foundry-old` (commit sebelum optimasi) dan `bin/foundry-new`.
3. Tetapkan sama: agent CLI + versi, model, effort, daftar task (jangan ubah rencana), `MaxAttempts`.
4. Kondisi cache: provider menyimpan prompt cache (Claude: 5 menit / 1 jam). Tunggu melewati TTL sebelum tiap run atau catat `cache_read` terpisah dan bandingkan `input + cache_write + output + cost`, bukan input saja. Selang-seling urutan old/new.
5. Ulangi ≥ 3 kali per versi (hasil agent bervariasi). Hanya hitung run dengan semua task DONE; laporkan juga yang gagal.
6. Setelah tiap run: `foundry metrics --json <folder> > run-N.json`. Bandingkan `build.input_tokens`, `cache_write_tokens`, `cached_tokens`, `output_tokens`, `cost_usd`, `runs`, `failures`, dan hasil akhir (build/test lulus, kriteria terpenuhi). Contoh: `jq '.build | {runs, input_tokens, cache_write_tokens, cached_tokens, output_tokens, cost_usd}' run-*.json`.
7. Klaim penghematan hanya bila median biaya/token versi baru lebih rendah **dan** kualitas (task DONE, test lulus) tidak turun. Jika `usage_runs < runs`, laporkan sebagai tidak lengkap.

## Keterbatasan

- Pencocokan modul memakai substring ID modul pada teks task/dependency. Salah cocok menambah konteks; tanpa modul cocok sama sekali → blueprint penuh. Task yang butuh modul yang tak tersebut di teks/dependency-nya bisa kehilangan kontrak modul itu.
- Memory konservatif: folder bersama yang diubah task lain membuat ringkasan kadaluarsa. Manfaat memory belum terbukti pada build nyata; metrik kontribusinya tersedia untuk dinilai.
- Review shared membaca tree, bukan git: perubahan manual pengguna pada waktu yang sama ikut terlihat (tidak bisa dibedakan dari agent). Tree > 20000 file dipotong dan reviewer diberi tahu.
- Aturan "kondisional" bergantung pada `Task.Area` dari rencana.
