package work

// designTemplate is the framework-agnostic UI design guidance seeded from
// ui-template-ai-friendly/. It is one consistent design language for every
// product, replacing the per-product presets and contextual rules. The
// existing design system always wins over it. The structured-output rules
// stay in uxRules/designRules/layoutRules; this block is the design taste.
const designTemplate = `UI DESIGN TEMPLATE (guidance, not output format)
Framework-agnostic; adapt to the project. The existing design system wins over this template.

MISI
Bangun UI yang konsisten dan mudah dipelihara. Jangan merancang ulang seluruh produk dari nol.

PRIORITAS KEPUTUSAN
1. Konvensi dan implementasi proyek yang sudah berjalan.
2. Kebutuhan eksplisit pengguna dan acceptance criteria.
3. Dokumen blueprint dan design token proyek.
4. Pola yang sudah digunakan pada halaman sejenis.
5. Keputusan baru seminimal mungkin; jelaskan jika ada konflik.

ARSITEKTUR HALAMAN
App shell (nav + header + content) → page header → satu tujuan utama → feedback (loading/empty/error/success). Jangan menambah sidebar/topbar kedua.

ATURAN LAYOUT
Pakai spacing/ukuran dari token. Hierarki: judul > section > label > helper. Satu aksi utama per area. Hindari nested card, dekorasi, dan warna sebagai satu-satunya status.

ATURAN KONTEN
Istilah produk. Label aksi spesifik. Error + pemulihan. Jangan membuat data atau teks produk palsu.

DESIGN TOKENS (skema, bukan nilai wajib)
color: bg, surface, text, text-muted, border, primary, success, warning, danger, focus.
typography: font.family, size xs/sm/md/lg/xl, weight normal/medium/bold.
spacing: space.n; radius.sm/md/lg; shadow.sm/md; control.height; content.max-width; layout.sidebar-width.

KOMPONEN
Pakai yang ada sebelum membuat baru. Button=aksi, link=navigasi; input berlabel; table berempty/loading/error; icon-only beraccessible name; destructive minta konfirmasi.

PAGE PATTERNS
Dashboard: header+filter, metrik hanya jika mendukung keputusan. List/table: satu aksi utama, kolom sesuai tugas. Detail: identitas + aksi. Form: kelompok field, validasi dekat field. Settings: kelompok per area. Auth: satu tugas.

INTERAKSI
Loading pada operasi lama; jangan hapus input saat gagal; konfirmasi hanya untuk aksi berisiko; hormati reduced-motion; fokus keyboard terlihat.

AKSESIBILITAS
Semantik, keyboard, fokus terlihat, label, kontras, status tidak hanya warna, heading berurutan, target cukup besar. WCAG 2.2 AA.

RESPONSIF
Konten dulu; hindari overflow horizontal; form satu kolom saat sempit; tabel boleh scroll; dialog tidak melebihi viewport.

DEFINISI SELESAI
Acceptance terpenuhi; pola konsisten; state utama ditangani; responsive dan a11y dasar ada.
`
