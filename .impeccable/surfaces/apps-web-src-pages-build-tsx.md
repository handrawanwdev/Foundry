---
version: 1
slug: "apps-web-src-pages-build-tsx"
primary_target: "apps/web/src/pages/Build.tsx"
related_targets: ["apps/web/src/components/Kanban.tsx"]
---

# Bangun / Kanban

<!-- impeccable:surface-brief 1 -->

## Scope
Halaman Bangun (`/simple/bangun`): papan task, toolbar aksi, panel PRD, inspector task. Bukan shell Workbench, bukan halaman lain.

## Visitor mode
Operate.

## Audience
User Foundry yang memutuskan kerja agent: mulai/lanjutkan build, pantau task, review, buka PRD.

## Job
Dalam 2 detik: siapa yang dikerjakan, apa yang siap paralel, aksi utama Mulai/Lanjutkan Bangun. PRD dokumen sampingan.

## Constraints
Tetap di shell VS Code gelap. Token `--bg/--card/--accent/--ok/--bad` tidak diganti. Jangan halaman terpisah. Jangan saingi CTA Build. Jangan border-kiri berwarna di kartu.

## Direction contract

THESIS: Papan Bangun adalah permukaan keputusan: pantau agent dan pilih langkah berikutnya. Menolak toolbar tombol setara dan lima kolom generik yang sama berat. IN PROGRESS dan REVIEW memimpin. Mulai/Lanjutkan Bangun satu-satunya aksi primer. PRD dokumen sampingan.

OWN-WORLD: Warisan workbench VS Code Dark+. Palet `--bg #1e1e1e`, `--card #252526`, `--accent #3794ff`, `--ok #4ec9b0`, `--bad #f48771`, `--status #007acc`. Tipografi system-ui 14px. Densitas editor. Kepala kolom membawa swatch status 8px, bukan isi kolom penuh. Kartu `--card` + garis `--line` 1px. Signature: live lane — kartu Dikerjakan berstrip progres; pill "siap paralel" memakai `--ok`.

STORY: User membuka Bangun, membaca progres, melihat siapa yang hidup, memulai atau menyetujui. PRD dibuka saat butuh dokumen, tidak sebagai CTA saingan.

FIRST VIEWPORT: Board mengisi pane editor (padding 0). Toolbar lengket: progres kiri; agent, kontrol PRD (generate / buka / menu unduh-buat ulang), Verifikasi, Mulai Bangun (primer, kanan). Kolom full-height, scroll di dalam. Task atau PRD membuka inspector kanan 380px; papan tetap. Kolom kosong: "Kosong". Board kosong: ajak ke Rancang.

FORM: Perluasan permukaan Bangun yang ada. Tanpa concept-seed. Interaksi tanda: kontrol PRD split + progres hidup di kartu Dikerjakan. Seed: none (local extension).

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance

## Post-build
Lane kosong memakai `COLUMN_EMPTY` per status, bukan kata "Kosong". Terblokir dihilangkan jika kosong. Dikerjakan/Review lebih lebar (`flex 1.3`). Inspector `minmax(300px, 380px)` di `--card`. `#d7ba7d` REVIEW/warn sudah ada di chrome workbench, bukan token `:root` baru.
