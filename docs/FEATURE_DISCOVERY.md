# Feature Discovery

Feature Discovery membantu pengguna mengetahui dan menyusun daftar fitur produk dari deskripsi biasa.

Alur: **Prompt → Fitur → Blueprint → Workflow → Export**.

Pengguna menulis ide di Prompt. Foundry menyusun modul, fitur, dan subfitur. Pengguna meninjau, menambah, mengubah, atau menghapus item, lalu meminta AI mengembangkan suatu fitur atau mencari yang belum tercakup. Setelah daftar dirasa cukup, **Lanjut ke Blueprint** menjalankan discovery yang sudah ada, dengan daftar fitur sebagai referensi.

Setiap fitur punya nama, deskripsi, tujuan, manfaat, dan hasil yang ingin dicapai.

Batasan yang disengaja:

- Bukan requirements engineering, scoring, atau versioning.
- Struktur fleksibel; berlaku untuk semua jenis produk.
- `Project.Features` tetap kontrak blueprint; Feature Map adalah pohon yang diedit di langkah Fitur.
- Feature Map terhubung ke blueprint lewat `source_id` pada feature/subfeature; command workflow menunjuk feature dengan `feature_ids`/`sub_feature_ids`. Progress fitur dihitung dari command tersebut (fallback: requirement yang dicakup command), memakai bobot command yang sama dengan progress workflow. Foundry tetap tidak menjalankan command atau menilai implementasi.
