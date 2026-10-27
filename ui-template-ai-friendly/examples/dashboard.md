# Dashboard Pattern Example

## Tujuan
Membantu pengguna memahami kondisi utama dan menentukan tindakan berikutnya.

## Struktur
1. Page header: judul, deskripsi singkat, filter waktu bila dibutuhkan.
2. Summary: maksimal metrik yang benar-benar relevan untuk keputusan.
3. Main analysis: chart atau tabel yang menjawab pertanyaan utama.
4. Recent activity: hanya bila aktivitas terbaru berguna untuk alur kerja.

## Rules
- Jangan menampilkan angka tanpa sumber data.
- Hindari menaruh setiap widget di dalam card bertingkat.
- Berikan label, satuan, dan rentang waktu untuk metrik.
- Sertakan loading, empty, dan error state.
- Prioritaskan satu tindakan utama.
