package work

// designTemplate is the framework-agnostic UI design guidance seeded from
// ui-template-ai-friendly/. It is one consistent design language for every
// product, replacing the per-product presets and contextual rules. The
// existing design system always wins over it. The structured-output rules
// stay in uxRules/designRules/layoutRules; this block is the design taste.
const designTemplate = `UI DESIGN TEMPLATE (guidance, not output format)
Framework-agnostic; adapt to the project. The existing design system wins over this template.

MISI
Bangun UI yang konsisten dan mudah dipelihara mengikuti arsitektur desain yang telah ditentukan. Jangan merancang ulang seluruh produk dari nol.

PRIORITAS KEPUTUSAN
1. Konvensi dan implementasi proyek yang sudah berjalan.
2. Kebutuhan eksplisit pengguna dan acceptance criteria.
3. Dokumen blueprint dan design token proyek.
4. Pola yang sudah digunakan pada halaman sejenis.
5. Keputusan baru seminimal mungkin; jelaskan jika ada konflik.

ARSITEKTUR HALAMAN
- App shell: navigasi utama, header/topbar, area konten.
- Page header: breadcrumb bila perlu, judul, deskripsi singkat, aksi utama.
- Main content: satu tujuan utama per halaman; gunakan pola tabel, form, detail, atau dashboard.
- Feedback layer: loading, empty, error, success, confirmation.
- Responsive: navigasi dan konten tetap dapat digunakan pada layar sempit.
- Jangan menambah sidebar/topbar kedua bila app shell sudah ada.

ATURAN LAYOUT
- Gunakan spacing dan ukuran dari design token; jangan menambah nilai acak bila token setara tersedia.
- Hierarki jelas: judul halaman > section heading > label > helper text.
- Satu aksi utama per area; aksi sekunder tidak bersaing secara visual.
- Lebar konten sesuai tugas; tabel padat boleh lebih lebar dari form.
- Hindari nested card tanpa alasan, dekorasi berlebihan, gradient dekoratif, dan ruang kosong yang tidak membantu.
- Jangan gunakan warna sebagai satu-satunya indikator status.

ATURAN KONTEN
- Bahasa dan istilah konsisten dengan produk.
- Label aksi spesifik ("Simpan perubahan", bukan "OK").
- Pesan error menjelaskan masalah dan tindakan pemulihan bila diketahui.
- Jangan membuat data, statistik, status, atau teks produk palsu.

DESIGN TOKENS (skema, bukan nilai wajib)
- color: bg, surface, text, text-muted, border, primary, success, warning, danger, focus.
- typography: font.family, font.size.xs/sm/md/lg/xl, font.weight.normal/medium/bold.
- spacing: space.1..n; radius.sm/md/lg; shadow.sm/md; control.height.sm/md/lg; content.max-width; layout.sidebar-width.
- Pakai token semantik; jangan buat token baru bila sudah ada; status selalu berlabel/ikon selain warna.

KOMPONEN
- Cari komponen yang ada dan contoh pemakaiannya sebelum membuat baru.
- Button untuk aksi, link untuk navigasi; input selalu berlabel; dialog/modal berjudul + fokus terkelola; table berempty/loading/error; badge/status tidak hanya warna; tooltip bukan satu-satunya info penting; icon-only button beraccessible name; destructive action minta konfirmasi.
- Ikuti nama/prop/variant/pola styling yang ada; hindari duplikasi; jangan ganti kontrak komponen bersama tanpa cek pemanggil.

PAGE PATTERNS
- Dashboard: header + rentang/filter; ringkasan metrik hanya jika mendukung keputusan; visualisasi berlabel; loading/empty/error; hindari metrik dekoratif.
- List/table: header + satu aksi utama; search/filter bila perlu; kolom sesuai tugas pengguna (bukan semua field DB); pagination/sort bila didukung data layer; empty state; aksi per baris tidak membuat tabel sulit dipindai.
- Detail: header identitas record + aksi relevan; kelompokkan berdasarkan makna; status dan metadata konsisten; edit/delete ikut permission + konfirmasi.
- Form: kelompokkan field terkait; label selalu terlihat (placeholder bukan pengganti label); validasi dekat field; pertahankan input valid saat error; bedakan cancel/save draft/submit; cegah submit ganda.
- Settings: kelompokkan per area; jelaskan dampak setting; pisahkan perubahan lokal vs langsung; konfirmasi perubahan berisiko.
- Authentication: satu tugas; minimalkan distraksi; error tanpa membocorkan detail sensitif.

INTERAKSI
- Loading pada operasi yang memakan waktu; cegah submit ganda.
- Jangan hapus input pengguna saat request gagal.
- Feedback dekat konteks aksi.
- Confirmation dialog hanya untuk tindakan berisiko/sulit dibatalkan.
- Hindari toast untuk info penting yang perlu tetap terlihat.
- Hormati reduced-motion; fokus keyboard terlihat; modal mengelola fokus dan mengembalikannya saat ditutup.
- Jangan asumsikan sukses sebelum hasil operasi mengonfirmasi.
- Jangan buat interaksi visual palsu (tombol harus terhubung aksi nyata).

AKSESIBILITAS
- Elemen semantik sesuai fungsi; semua fungsi utama bisa keyboard; fokus terlihat; input berlabel; kontras memadai; status tidak hanya warna; gambar informatif ber alt; heading berurutan; target interaksi cukup besar.

RESPONSIF
- Rancang dari kebutuhan konten; hindari horizontal overflow; form jadi satu kolom saat sempit; tabel kompleks boleh scroll horizontal atau pola alternatif; sidebar ikut pola responsif yang ada; dialog tidak melebihi viewport.

DEFINISI SELESAI (untuk desain)
- Acceptance criteria terpenuhi.
- Pola visual konsisten dengan halaman lain.
- State utama (loading, empty, error, success) ditangani sesuai konteks.
- Responsive dan aksesibilitas dasar dipertimbangkan.
`
