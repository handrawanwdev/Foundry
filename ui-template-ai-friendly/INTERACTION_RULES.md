# Interaction Rules

- Setiap kontrol harus memiliki perilaku yang jelas dan dapat digunakan.
- Tampilkan state loading pada operasi yang memerlukan waktu; cegah submit ganda jika relevan.
- Jangan menghapus input pengguna ketika request gagal.
- Berikan feedback yang dekat dengan konteks aksi.
- Gunakan confirmation dialog hanya untuk tindakan berisiko atau sulit dibatalkan.
- Hindari toast untuk informasi penting yang perlu tetap terlihat.
- Jangan menambahkan animasi jika tidak memperjelas perubahan state.
- Hormati reduced-motion preference bila animasi digunakan.
- Fokus keyboard harus terlihat dan urutan tab masuk akal.
- Modal harus mengelola fokus dan mengembalikannya saat ditutup.
- Jangan mengasumsikan aksi berhasil sebelum hasil operasi mengonfirmasi keberhasilan.
- Jangan membuat interaksi visual palsu: tombol harus terhubung ke aksi nyata atau ditandai jelas sebagai belum tersedia.
