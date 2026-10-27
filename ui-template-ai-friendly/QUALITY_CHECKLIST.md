# UI Quality Checklist

## Sebelum implementasi
- [ ] Framework, struktur, route, dan design system sudah diinspeksi.
- [ ] Komponen dan halaman sejenis sudah dicari.
- [ ] Kebutuhan dipetakan ke page pattern.
- [ ] Acceptance criteria dipahami; ketidakjelasan penting dicatat.

## Saat implementasi
- [ ] Reuse komponen yang tersedia.
- [ ] Menggunakan token semantik yang ada.
- [ ] Tidak menambahkan dependency tanpa alasan.
- [ ] State loading/empty/error/success ditangani sesuai konteks.
- [ ] Label, validasi, dan aksi dapat dipahami.
- [ ] Responsive dan aksesibilitas dasar dipertimbangkan.
- [ ] Tidak ada data palsu atau tombol tanpa fungsi yang disamarkan sebagai fitur selesai.

## Verifikasi
- [ ] Lint/typecheck/test/build dijalankan jika tersedia.
- [ ] Halaman diperiksa pada viewport yang relevan jika browser tersedia.
- [ ] Console error dan masalah layout diperiksa bila alat mendukung.
- [ ] Pemeriksaan yang gagal atau tidak dijalankan disebutkan secara eksplisit.

## Laporan agent
Laporkan:
1. Ringkasan perubahan.
2. File utama yang diubah.
3. Pemeriksaan yang dijalankan dan hasilnya.
4. Risiko, asumsi, dan hal yang belum diverifikasi.
