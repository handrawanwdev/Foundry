# Feature Discovery

Feature Discovery membantu pengguna mengetahui dan menyusun daftar fitur produk dari deskripsi biasa.

Alur: **Prompt → Fitur → Blueprint → Workflow → Export**.

Pengguna menulis ide di Prompt. Foundry menyusun modul, fitur, dan subfitur. Pengguna meninjau, menambah, mengubah, atau menghapus item, lalu meminta AI mengembangkan suatu fitur atau mencari yang belum tercakup. Setelah daftar dirasa cukup, **Lanjut ke Blueprint** menjalankan discovery yang sudah ada, dengan daftar fitur sebagai referensi.

Setiap fitur punya nama, deskripsi, tujuan, manfaat, dan hasil yang ingin dicapai.

Batasan yang disengaja:

- Bukan requirements engineering, scoring, atau versioning.
- Struktur fleksibel; berlaku untuk semua jenis produk.
- `Project.Features` tetap kontrak blueprint; Feature Map adalah pohon yang diedit di langkah Fitur.
- Feature Map terhubung ke blueprint lewat `source_id` pada feature/subfeature; node tanpa `source_id` dipetakan sekali secara deterministik: nama persis, lalu teks node + konteks modul, lalu fitur yang sudah pasti di modul yang sama. Node yang tidak yakin tetap kosong. Command workflow menunjuk feature dengan `feature_ids`/`sub_feature_ids`; project lama mengisi `feature_ids` dari requirement yang dicakup command. Progress fitur dihitung dari command tersebut, memakai bobot command yang sama dengan progress workflow. Foundry tetap tidak menjalankan command atau menilai implementasi.
