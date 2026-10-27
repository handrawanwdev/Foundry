# Component Rules

## Sebelum membuat komponen
1. Cari komponen yang sudah ada dan contoh pemakaiannya.
2. Periksa apakah kebutuhan dapat dipenuhi melalui props/variant yang tersedia.
3. Buat komponen baru hanya jika pola benar-benar reusable atau kompleksitas halaman membenarkannya.

## Semantik komponen
- Button: gunakan untuk aksi; tautan navigasi harus tetap berupa link.
- Input: selalu punya label yang terlihat atau accessible name.
- Dialog/modal: punya judul, fokus yang terkelola, aksi jelas, dan dapat ditutup sesuai konteks.
- Table: gunakan header kolom yang bermakna; dukung empty/loading/error state.
- Badge/status: teks status harus jelas; jangan hanya mengandalkan warna.
- Tooltip: bukan satu-satunya tempat untuk informasi penting.
- Icon-only button: wajib memiliki accessible name.
- Destructive action: jelaskan dampak dan minta konfirmasi bila dampaknya signifikan.

## API dan konsistensi
- Ikuti nama, prop, variant, dan pola styling yang sudah ada.
- Jangan membuat wrapper komponen baru hanya untuk mengganti nama.
- Hindari duplikasi style dan conditional logic yang tersebar.
- Jangan menambahkan dependency UI baru tanpa mengecek library yang sudah dipakai.
- Jangan mengubah kontrak komponen bersama tanpa memeriksa semua pemanggilnya.

## State wajib sesuai konteks
- Loading: tunjukkan proses yang sedang berjalan tanpa membuat layout meloncat berlebihan.
- Empty: jelaskan belum ada data dan langkah berikutnya jika relevan.
- Error: tampilkan pesan yang dapat dipahami serta opsi retry jika aman.
- Success: berikan feedback yang proporsional.
- Disabled: hanya ketika aksi memang belum tersedia; jelaskan alasan bila tidak jelas.
