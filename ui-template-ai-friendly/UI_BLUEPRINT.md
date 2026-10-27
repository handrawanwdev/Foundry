# UI Blueprint

## Misi
Membangun UI yang konsisten dan mudah dipelihara dengan mengikuti arsitektur desain yang telah ditentukan. Agent bertugas menerapkan blueprint, bukan merancang ulang seluruh produk dari nol.

## Prioritas keputusan
1. Konvensi dan implementasi proyek yang sudah berjalan.
2. Kebutuhan eksplisit pengguna dan acceptance criteria.
3. Dokumen blueprint ini dan design tokens proyek.
4. Pola yang sudah digunakan pada halaman sejenis.
5. Keputusan baru seminimal mungkin; jelaskan jika ada konflik.

Jangan mengganti teknologi, framework, routing, atau library UI tanpa kebutuhan nyata dan persetujuan yang sesuai.

## Alur wajib
1. **Inspect** — periksa framework, route, struktur folder, design tokens, komponen, halaman sejenis, dan test yang tersedia.
2. **Map** — petakan kebutuhan ke page pattern yang paling dekat.
3. **Reuse** — gunakan komponen dan pola yang ada sebelum membuat yang baru.
4. **Implement** — ubah sekecil mungkin; jaga perilaku dan API yang tidak diminta agar tetap kompatibel.
5. **Verify** — jalankan lint, typecheck, test, build, dan/atau browser check yang memang tersedia.
6. **Report** — ringkas perubahan, hasil verifikasi, risiko, dan hal yang belum dapat diverifikasi.

## Arsitektur halaman standar
- App shell: navigasi utama, header/topbar, area konten.
- Page header: breadcrumb bila diperlukan, judul, deskripsi singkat, aksi utama.
- Main content: satu tujuan utama per halaman; gunakan pola tabel, form, detail, atau dashboard yang sesuai.
- Feedback layer: loading, empty, error, success, dan confirmation state.
- Responsive behavior: navigasi dan konten tetap dapat digunakan pada layar sempit.

Jangan menambahkan sidebar atau topbar kedua bila aplikasi sudah memiliki app shell.

## Aturan layout
- Gunakan spacing dan ukuran dari `DESIGN_TOKENS.md`; jangan menambahkan nilai acak jika token setara tersedia.
- Tentukan hierarki yang jelas: judul halaman > section heading > label > helper text.
- Satu aksi utama per area; aksi sekunder tidak boleh bersaing secara visual.
- Gunakan lebar konten yang sesuai tugas; tabel padat boleh lebih lebar daripada form.
- Hindari nested card tanpa alasan, dekorasi berlebihan, gradient dekoratif, dan ruang kosong yang tidak membantu.
- Jangan menggunakan warna sebagai satu-satunya indikator status.

## Aturan konten
- Gunakan bahasa dan istilah yang konsisten dengan produk.
- Label aksi harus spesifik: “Simpan perubahan”, bukan “OK” jika konteksnya dapat diperjelas.
- Pesan error menjelaskan masalah dan tindakan pemulihan jika diketahui.
- Jangan membuat data, statistik, status, atau teks produk palsu untuk UI produksi. Data dummy hanya untuk fixture/demo yang jelas.

## Definisi selesai
- Kebutuhan dan acceptance criteria terpenuhi.
- Pola visual konsisten dengan halaman lain.
- State utama (loading, empty, error, success) ditangani sesuai konteks.
- Responsive dan aksesibilitas dasar diperiksa.
- Pemeriksaan yang tersedia dijalankan; kegagalan dan pemeriksaan yang dilewati dilaporkan jujur.
