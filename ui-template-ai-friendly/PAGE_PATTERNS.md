# Page Patterns

Gunakan pola paling dekat; jangan memaksakan pola jika tidak sesuai kebutuhan.

## Dashboard
- Page header dengan judul dan rentang waktu/filter bila diperlukan.
- Ringkasan metrik hanya jika mendukung keputusan pengguna.
- Visualisasi harus menjawab pertanyaan yang jelas dan memiliki label.
- Sertakan loading, empty, dan error state.
- Hindari metrik dekoratif atau angka yang tidak punya sumber data.

## Data list / table
- Page header dan satu aksi utama (misalnya tambah item).
- Search/filter hanya bila data atau use case memerlukannya.
- Kolom dipilih berdasarkan tugas pengguna; jangan menampilkan semua field database.
- Tampilkan pagination/sort hanya jika didukung data layer.
- Sediakan empty state dan feedback saat aksi berhasil/gagal.
- Aksi per baris tidak boleh membuat tabel sulit dipindai.

## Detail
- Page header dengan identitas record dan aksi yang relevan.
- Kelompokkan informasi berdasarkan makna, bukan urutan field API.
- Tampilkan status dan metadata secara konsisten.
- Edit/delete harus mengikuti permission dan pola konfirmasi yang berlaku.

## Form / create / edit
- Kelompokkan field terkait ke dalam section.
- Label selalu terlihat; placeholder bukan pengganti label.
- Validasi dekat dengan field dan rangkum error jika form panjang.
- Pertahankan input valid ketika terjadi error.
- Bedakan cancel, save draft, dan submit final bila semuanya tersedia.
- Tampilkan status penyimpanan dan cegah submit ganda bila relevan.

## Settings
- Kelompokkan berdasarkan area pengaturan.
- Jelaskan dampak setting yang tidak jelas.
- Pisahkan perubahan lokal dari perubahan yang langsung diterapkan.
- Gunakan konfirmasi untuk perubahan berisiko tinggi.

## Authentication
- Fokus pada satu tugas.
- Minimalkan distraksi.
- Tampilkan error autentikasi tanpa membocorkan detail sensitif.
- Jangan mengubah kebijakan autentikasi hanya melalui pekerjaan UI.
