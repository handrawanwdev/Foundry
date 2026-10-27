# Design Tokens

Token di bawah adalah **skema yang harus diisi/diverifikasi**, bukan nilai wajib untuk semua produk. Jika proyek sudah memiliki token, gunakan token proyek dan sinkronkan dokumen ini.

## Color roles
- `color.bg`: latar utama halaman.
- `color.surface`: permukaan card/panel.
- `color.text`: teks utama.
- `color.text-muted`: teks sekunder.
- `color.border`: garis pembatas.
- `color.primary`: aksi utama dan fokus.
- `color.success`: status berhasil.
- `color.warning`: status peringatan.
- `color.danger`: status gagal/destruktif.
- `color.focus`: indikator fokus keyboard.

## Typography roles
- `font.family`: font UI utama.
- `font.size.xs`: metadata/helper.
- `font.size.sm`: teks sekunder/label.
- `font.size.md`: body default.
- `font.size.lg`: subheading.
- `font.size.xl`: heading halaman.
- `font.weight.normal`, `font.weight.medium`, `font.weight.bold`.
- Gunakan hierarki ukuran terbatas; jangan menambahkan ukuran baru untuk satu elemen tanpa kebutuhan.

## Spacing and geometry
- `space.1` ... `space.n`: skala spacing konsisten.
- `radius.sm`, `radius.md`, `radius.lg`: radius.
- `shadow.sm`, `shadow.md`: elevasi seperlunya.
- `control.height.sm`, `control.height.md`, `control.height.lg`.
- `content.max-width`: batas lebar konten jika sesuai.
- `layout.sidebar-width`: hanya bila app shell memakai sidebar.

## Token rules
1. Gunakan token semantik, bukan warna literal tersebar di komponen.
2. Jangan membuat token baru jika token yang ada sudah sesuai.
3. Kontras teks dan kontrol harus tetap terbaca.
4. Status sukses/peringatan/error harus punya label atau ikon selain warna.
5. Perubahan token global perlu dicek terhadap semua halaman utama.
6. Nilai token aktual harus diambil dari stylesheet/theme proyek, bukan ditebak oleh agent.
