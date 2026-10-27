# AI-Friendly UI Template

Template desain UI berbasis blueprint untuk coding agent apa pun. Tujuannya bukan membuat AI bebas menciptakan desain dari nol, tetapi mengarahkan AI untuk merakit halaman dari pola dan aturan yang konsisten.

## Cara pakai
1. Salin folder ini ke root proyek, misalnya `docs/ui-template/` atau `.foundry/ui-template/`.
2. Sesuaikan `DESIGN_TOKENS.md` dengan brand/proyek.
3. Isi referensi implementasi komponen yang benar-benar tersedia di proyek pada `COMPONENT_RULES.md`.
4. Minta agent membaca `UI_BLUEPRINT.md` dan checklist yang relevan sebelum mengubah UI.
5. Evaluasi hasil lewat build, test, dan pemeriksaan visual bila browser tersedia.

## Prompt singkat untuk agent
> Baca `UI_BLUEPRINT.md` beserta dokumen relevan di folder template. Inspeksi struktur proyek dan komponen yang sudah ada sebelum mengubah kode. Ikuti blueprint dan design token; reuse komponen yang tersedia; jangan membuat sistem desain baru atau menambah dependency tanpa alasan. Implementasikan hanya kebutuhan yang diminta, lalu jalankan pemeriksaan yang tersedia dan laporkan hasil serta file yang berubah.

## Prinsip
- Existing project conventions > blueprint template > preferensi pribadi agent.
- Reuse-first; hindari duplikasi komponen.
- Blueprint adalah sumber aturan; kode aplikasi adalah sumber kebenaran implementasi.
- Jangan mengarang komponen, token, route, API, atau perilaku yang belum diverifikasi.
