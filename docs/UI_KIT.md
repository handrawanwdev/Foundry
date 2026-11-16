# UI Kit & Mockup

Rancangan UI Foundry dibangun dari template yang dikurasi, bukan dari nol. AI memilih; Foundry mewujudkan.

## Alur

1. **Preset token** (`rules.preset`): AI memilih satu preset; Foundry mengisi semua token (warna, tipografi, spasi, `size.control`, radius, shadow, breakpoint). `rules.brand` (`primary` hex, `font`) mengadaptasi aksen dan font; hover, teks di atas warna utama, tint, dan focus ring diturunkan otomatis. Skala tipografi/spasi/radius/breakpoint preset terkunci; `rules.tokens` hanya untuk warna semantik tambahan.
2. **Template pattern** (`sections[].slot`): setiap pattern punya slot (mis. `table`: `toolbar`, `summary`, `table*`, `pagination`). Section tanpa slot yang sah ditempatkan Foundry; slot wajib (`*`) selalu diisi. Header halaman dirender dari `name`, `purpose`, `primary_action`.
3. **Mockup**: setiap halaman dirender jadi HTML statis (token → CSS variable, slot → grid template, data contoh). Tinjau di Blueprint → Spesifikasi Layout per viewport (desktop/tablet/mobile) dan state (loading, empty, error, …), lalu setujui. Perubahan desain yang mengubah mockup membuat persetujuan berstatus *berubah*.
4. **Ekspor**: `design/mockups/<page>.html`, `design/mockups/index.html`, `design/patterns/<pattern>.html` (contoh template). Command yang membangun halaman menautkan mockup-nya. MCP: `get_mockup`.
5. **Evaluasi visual**: implementasi menandai akar tiap section dengan `data-foundry-section="<id>"`; `record_visual_review` memeriksa section yang hilang dan urutannya di viewport sempit (kriteria `structure`).

## Aturan

- **Tabel dan daftar wajib punya pencarian + filter dan paginasi.** Sumbernya: section toolbar/pagination di halaman (melayani tabel/daftar pertama), atau item content section itu sendiri (`"Filter: cari plat, status"`, `"Paginasi: 20 per halaman"` / `"muat lebih banyak"`). Format 3: error; format lama: peringatan. Saat AI melewatkannya, Foundry menambahkan item itu otomatis (ApplyDesign/EditLayout). Mockup selalu menggambarnya: toolbar, tabel, dan paginasi pattern `table`/`list`/`cards`/`master-detail` disatukan dalam satu panel (`panel` di JSON pattern).
- **Layar mengikuti use case.** Tiap use case berlayar dipetakan ke halaman yang dilaluinya (`flows`), berurutan. Foundry menempatkan tiap langkah di halaman yang menyebutnya (`core.MapFlowSteps`); langkah sistem (Sistem/Worker/Perangkat/…, kalimat pasif, nama teknis) jadi umpan balik, bukan halaman. Cek: halaman berurutan wajib saling link (format 3: error), use case tanpa halaman, langkah pengguna yang belum tergambar, dan ≥5 langkah di satu halaman (saran pecah/wizard) — peringatan. Saat AI melewatkannya, `completeFlows` menebak halaman dari langkah (`core.InferFlowPages`), menambah link antarhalaman, dan mengisi `primary_action` dari langkah "Buat/Ajukan/…" pertama. Mockup: tombol ke halaman berikutnya di header, stepper langkah use case yang dijalankan di halaman itu, aksi detail ("Aksi ajukan atau kirim") jadi tombol, dan `index.html` memuat storyboard "Alur use case". Panel mockup punya mode jalan-per-use-case.
- **Cabang, keputusan, dan quality gate.** Alternatif/kegagalan use case dibaca jadi cabang (langkah asal, tujuan: langkah lain, selesai, batal, atau use case lain). Desainer menempatkan tiap langkah dan cabang di halaman (`step_pages`, `branch_pages`); tanpa itu Foundry mencocokkan kata. Titik keputusan butuh halaman dengan aksi setujui dan tolak/kembalikan; langkah hapus/batal/posting butuh konfirmasi (di halaman atau di `ux_guidelines`). `core.DesignGate` menilai desain dengan 10 pemeriksaan dan traceability requirement → use case → halaman → aksi utama; laporannya `review/design-gate.md`. Desain tidak dianggap lengkap hanya karena semua halaman ada: gate yang gagal memblokir persetujuan mockup.
- Proyek existing: design system dari kode tetap menang. Preset hanya mengisi mockup; tokennya tidak ditambahkan ke rules.
- Desain lama (format 2) tetap valid: aturan slot/preset hanya peringatan, tidak dicek terhadap mockup.
- Mockup disajikan inert: tanpa script, tanpa request jaringan (CSP `default-src 'none'`).

## Menambah atau mengganti template

Taruh file dengan format yang sama seperti `internal/modules/uikit/data/` di `$FOUNDRY_HOME/uikit/` (default `~/.foundry/uikit/`):

- `presets/<id>.json` — `id`, `name`, `fit`, `direction`, `density`, `tokens` (nama token sama dengan preset bawaan agar mockup lengkap).
- `patterns/<id>.json` — `id` harus salah satu pattern layout Foundry; `columns`, `areas` (baris grid berisi id slot), `slots` (`id`, `kind`, `guide`, `required`, `match`), `panel` (opsional: slot toolbar, koleksi, paginasi yang dirender satu kartu), `rules`, `avoid`, `example`.

File dengan `id` sama menimpa bawaan. Restart Foundry setelah mengubahnya.
