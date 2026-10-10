# Standar Sederhana untuk Semua Software

> Satu file. Cukup untuk membangun web, mobile, desktop, CLI, API, dan tools dengan struktur rapi, kode bersih, latency kecil, ORM, Redis, event internal, dan webhook.
> Cara pakai: (1) jawab Survey di bagian 3, (2) tempel Master Prompt di bagian 8 ke AI, (3) AI meriset, mengisi Profile, lalu membuat prompt kerja.
> Taruh di root sebagai `FOUNDRY.md`, rujuk dari `AGENTS.md` / `CLAUDE.md`.

---

## 1. Delapan Aturan Wajib (tidak bisa ditawar)

| #   | Aturan                           | Artinya                                                                                                                                                                                                              |
| --- | -------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | **Modular + Clean Architecture** | Kode dikelompokkan per fitur/domain. Dependency hanya menunjuk ke dalam: entry point → use case → domain. Domain tidak tahu framework, DB, atau UI                                                                   |
| 2   | **Kode mudah dibaca**            | Nama menjelaskan maksud. Fungsi ≤ 40 baris, parameter ≤ 4, nesting ≤ 3, file ≤ 300 baris. Tidak ada magic value, dead code, atau tipe longgar (`any`)                                                                |
| 3   | **ORM wajib**                    | Semua akses database lewat ORM, hanya di layer data access (repository). Raw SQL hanya untuk kasus terdokumentasi (agregasi berat, bulk). Query builder murni (mis. Knex tanpa lapisan model) **tidak dihitung** ORM |
| 4   | **Redis wajib**                  | Cache, job queue, rate limit, dan event antar-service. Database tetap sumber kebenaran; Redis boleh mati tanpa menghilangkan data                                                                                    |
| 5   | **Event internal**               | Komunikasi antar modul/service lewat event dengan format seragam (bagian 5). Tidak ada pemanggilan langsung lintas modul untuk efek samping                                                                          |
| 6   | **Latency kecil**                | Anggaran latency ditetapkan di awal dan diukur (bagian 6). Tidak ada N+1, pekerjaan berat sinkron, atau query tanpa index                                                                                            |
| 7   | **Frontend bukan "AI slop"**     | Desain berbasis token dan riset referensi nyata, bukan tampilan generik (bagian 7)                                                                                                                                   |
| 8   | **Struktur adalah kontrak**      | Ikuti struktur existing. Folder/helper/pola baru butuh alasan. Perubahan arsitektur butuh ADR dan persetujuan                                                                                                        |

Prioritas jika konflik: struktur existing yang valid → Profile project → dokumen ini → preferensi pribadi.

---

## 2. Struktur Folder Standar

Monorepo, berlaku untuk semua jenis software. Ambil hanya yang dibutuhkan; **tidak ada folder kosong**.

```text
project/
├── apps/
│   ├── api/                 # backend (HTTP + webhook)
│   ├── worker/              # consumer event + job queue
│   ├── web/                 # web
│   ├── mobile/              # mobile
│   ├── desktop/             # desktop
│   └── cli/                 # CLI/tools
├── packages/
│   ├── domain/              # entity, rule, kalkulasi murni (tanpa I/O)
│   ├── contracts/           # skema input/output + skema event (satu sumber kebenaran)
│   ├── ui/                  # komponen + design tokens (jika ada frontend)
│   └── config/              # lint, tsconfig, dsb.
├── docs/                    # foundry-profile.md, requirements.md, adr/
├── scripts/
├── docker-compose.yml       # database + Redis untuk dev
├── .env.example
└── FOUNDRY.md
```

Di dalam satu aplikasi backend (per fitur, bukan per jenis file):

```text
src/
├── config/                  # env divalidasi, dibaca di satu tempat
├── infra/                   # adapter keluar: database (ORM), redis, event bus, http client
├── common/                  # teknis lintas modul (logger, error, guard); tanpa business logic
└── modules/{fitur}/
    ├── {fitur}.controller   # entry point tipis: parsing, validasi, format output
    ├── {fitur}.service      # use case: aturan bisnis dan orkestrasi
    ├── {fitur}.repository   # satu-satunya tempat memakai ORM
    ├── {fitur}.events       # event yang diterbitkan/dikonsumsi modul ini
    ├── dto/                 # kontrak input/output
    └── {fitur}.mapper       # model ORM ⇄ domain
```

Dilarang: folder `helpers`, `utils`, `misc`, `manager` sebagai penampung; business logic di `common`; model ORM bocor ke domain atau response.

---

## 3. Survey (8 pertanyaan)

Jawab dengan huruf, atau `default` untuk memakai ★. Jawaban `tidak tahu` = AI memakai default dan mencatatnya sebagai asumsi.

| #   | Pertanyaan                                                                  | Pilihan                                                                                              |
| --- | --------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------- |
| 1   | **Apa yang dibangun?** (1–3 kalimat: masalah, pengguna, dan 3–5 fitur inti) | teks bebas                                                                                           |
| 2   | **Jenis software?** (boleh lebih dari satu)                                 | a) web b) mobile c) desktop d) CLI/tools e) API/backend saja f) bot/engine                           |
| 3   | **Siapa penggunanya?**                                                      | a) hanya saya ★ b) tim kecil c) publik                                                               |
| 4   | **Tahap?**                                                                  | a) prototipe b) MVP ★ c) produksi                                                                    |
| 5   | **Target latency?**                                                         | a) < 500 ms ★ b) < 200 ms c) < 100 ms (p95)                                                          |
| 6   | **Apakah perlu webhook?**                                                   | a) tidak b) menerima webhook dari pihak lain c) mengirim webhook ke pihak lain d) keduanya           |
| 7   | **Berapa service?**                                                         | a) satu aplikasi modular ★ (event internal tetap dipakai, siap dipecah) b) beberapa service terpisah |
| 8   | **Preferensi stack?**                                                       | a) AI riset dan pilih yang terbaik ★ b) saya tentukan: **\_\_**                                      |

Hanya itu. Semua keputusan teknis lain ditentukan AI lewat riset (bagian 8), dicatat di Profile, lalu disetujui kamu.

---

## 4. Default Awal Stack (titik mulai, wajib diverifikasi AI lewat riset)

AI tidak boleh memakai tabel ini mentah. Ia membandingkan minimal 2 opsi per kategori, memeriksa versi dan status pemeliharaan terkini, lalu memutuskan.

| Jenis               | Titik mulai                                                                                                                       | Catatan                                                            |
| ------------------- | --------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------ |
| Backend             | NestJS (TypeScript) atau Go (Gin/Chi)                                                                                             | Pilih Go jika target latency < 100 ms atau beban tinggi            |
| ORM                 | TypeScript: Prisma / Drizzle / TypeORM · Go: GORM / Ent · Python: SQLAlchemy · Java/Kotlin: Hibernate/JPA · Rust: SeaORM / Diesel | Wajib ORM penuh (aturan 3)                                         |
| Database            | PostgreSQL                                                                                                                        | SQLite untuk data lokal di mobile/desktop/CLI, tetap lewat ORM     |
| Web                 | React + Vite (atau Next.js bila butuh SSR/SEO)                                                                                    | Data server: TanStack Query. State UI lokal: Zustand               |
| Mobile              | React Native (Expo) atau Flutter                                                                                                  | Berbagi `packages/domain` dan `packages/contracts` bila TypeScript |
| Desktop             | Tauri + frontend web yang sama (Electron bila butuh ekosistem Node penuh)                                                         | Logic di `packages/domain`                                         |
| CLI/tools           | Go (Cobra) atau TypeScript                                                                                                        | Biner tunggal bila didistribusikan                                 |
| Cache, queue, event | Redis (+ BullMQ untuk job di TypeScript)                                                                                          | Lihat bagian 5                                                     |
| Validasi & kontrak  | Zod (TypeScript) atau skema bawaan bahasa                                                                                         | Satu sumber di `packages/contracts`                                |
| Test                | Vitest/Jest + Testcontainers + Playwright (web/desktop)                                                                           |                                                                    |

Jika platform lebih dari satu: **shared dulu** (domain, kontrak, validasi, kalkulasi), kode spesifik platform hanya untuk UI, API native, filesystem, kamera, notifikasi, dan sejenisnya. Dilarang mengurangi platform target secara diam-diam.

---

## 5. Event Internal & Webhook

### 5.1 Event antar-modul / antar-service

| Kebutuhan                                                        | Pilihan standar                                                                                                 |
| ---------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------- |
| Event yang **tidak boleh hilang** (transaksi, order, eksekusi)   | **Redis Streams** + consumer group + ack + retry + dead-letter stream ★                                         |
| Notifikasi yang **boleh hilang** (update UI, cache invalidation) | Redis Pub/Sub                                                                                                   |
| Event harus konsisten dengan perubahan database                  | **Outbox pattern**: event ditulis ke tabel `outbox` dalam transaksi yang sama, lalu dikirim ke Redis oleh relay |
| Skala besar atau butuh replay panjang                            | Evaluasi Kafka/NATS lewat ADR (bukan default)                                                                   |

Format event (envelope) wajib seragam, skemanya di `packages/contracts`:

```json
{
  "eventId": "uuid",
  "type": "transaction.created",
  "version": 1,
  "occurredAt": "2026-10-10T12:00:00Z",
  "correlationId": "uuid",
  "payload": {}
}
```

Aturan:

- Nama event `domain.kejadian` (lampau): `transaction.created`, `budget.exceeded`.
- Consumer **idempotent**: simpan `eventId` yang sudah diproses.
- Perubahan payload = naikkan `version`, jangan mengubah yang lama.
- Key/stream berprefix project: `{project}:stream:{domain}`, wajib `MAXLEN` atau TTL.
- Publisher tidak tahu siapa consumer-nya.

### 5.2 Webhook (jika dibutuhkan)

**Menerima webhook (inbound)**

1. Verifikasi tanda tangan (HMAC) dan timestamp sebelum memproses apa pun.
2. Balas `2xx` secepatnya, lalu proses lewat queue (jangan sinkron).
3. Idempotent berdasarkan ID event pengirim.
4. Simpan payload mentah untuk audit dan replay.

**Mengirim webhook (outbound)**

1. Endpoint penerima didaftarkan, dengan secret per endpoint.
2. Setiap kiriman ditandatangani HMAC dan membawa `eventId` serta timestamp.
3. Retry dengan exponential backoff + jitter, lalu dead-letter setelah batas percobaan.
4. Log pengiriman (status, durasi, percobaan) dan tombol kirim ulang.
5. Pengiriman lewat worker, bukan di request pengguna.

---

## 6. Latency & Performa

| Area        | Aturan                                                                                                                        |
| ----------- | ----------------------------------------------------------------------------------------------------------------------------- |
| Anggaran    | Target dari Survey #5 dicatat di Profile dan diuji di CI/load test sebelum rilis                                              |
| Database    | Index untuk kolom filter/join/sort. Dilarang N+1 (gunakan eager load/batch). Pagination cursor. Connection pool dikonfigurasi |
| Cache       | Cache-aside di Redis dengan TTL dan invalidasi lewat event. Cache tidak pernah jadi sumber kebenaran                          |
| Kerja berat | Agregasi besar, export, import, notifikasi, dan panggilan eksternal lewat queue                                               |
| API         | Respons ringkas (hanya field dibutuhkan), kompresi, timeout dan retry dengan backoff ke dependency eksternal                  |
| Frontend    | Code splitting per rute, gambar dioptimalkan, anggaran LCP < 2,5 s, tidak ada render berulang yang tidak perlu                |
| Pengukuran  | Log durasi per endpoint dan query, metrik p50/p95, health check `/health` dan `/ready`                                        |

---

## 7. Frontend: Bukan "AI Slop"

"AI slop" = tampilan generik: gradient ungu default, hero berisi kalimat klise, kartu seragam tanpa hierarki, ikon emoji sebagai UI, teks lorem ipsum, spasi dan warna acak. Cegah dengan aturan berikut.

**Sebelum satu layar pun dibuat, AI wajib menyiapkan `docs/design-brief.md`:**

1. Pengguna dan tugas utama tiap layar (apa yang ingin diselesaikan orang di sana).
2. **Riset 3–5 produk nyata** di domain yang sama: apa pola yang dipakai, mengapa efektif, apa yang dihindari. Sertakan sumber.
3. Prinsip desain turunan (3–5 kalimat) dan arah visual (nada, kepadatan informasi).
4. Design tokens: palet terbatas (warna netral + 1 aksen + warna status), skala tipografi (maksimal 2 keluarga font), skala spasi, radius, bayangan, durasi animasi.

**Aturan implementasi:**

- Semua nilai visual berasal dari token di `packages/ui`. Dilarang warna, ukuran, atau spasi hardcode di komponen.
- Kontras memenuhi WCAG AA. Navigasi keyboard dan label aksesibilitas wajib.
- Setiap layar punya state **loading, kosong, error, dan sukses**.
- Konten dan data realistis, bukan lorem ipsum atau angka serba bulat.
- Hierarki jelas: satu aksi utama per layar, informasi terpenting paling menonjol.
- Komponen dipakai ulang dari satu pustaka; dilarang membuat variasi tombol/kartu baru tanpa alasan.
- Ikon dari satu keluarga ikon, bukan emoji.
- Responsif mulai dari layar kecil; di desktop/mobile ikuti konvensi platform (navigasi, gestur, pintasan).
- Tema terang dan gelap berasal dari token yang sama.

**Pemeriksaan sebelum selesai (AI wajib menjawab):** apakah layar ini bisa tertukar dengan template generik? Jika ya, kembali ke design brief.

---

## 8. Master Prompt (tempel ini ke AI)

### 8.1 Prompt utama: riset → Profile → prompt kerja

```text
Baca FOUNDRY.md. Saya ingin membangun software baru. Ini jawaban Survey saya:

1. Apa yang dibangun: <isi>
2. Jenis software: <isi>
3. Pengguna: <isi>
4. Tahap: <isi>
5. Target latency: <isi>
6. Webhook: <isi>
7. Jumlah service: <isi>
8. Preferensi stack: <AI riset | isi>

Tugasmu, urut, dan JANGAN menulis kode atau scaffold sebelum saya menyetujui hasil tahap 3:

TAHAP 1 — RISET (gunakan pencarian web, bukan ingatan)
- Untuk tiap kategori di FOUNDRY.md bagian 4 yang relevan, bandingkan minimal 2 opsi:
  bahasa/framework, ORM, frontend, mobile/desktop/CLI (sesuai jenis software), event dan queue.
- Periksa versi stabil terbaru, status pemeliharaan, dukungan ORM-nya, performa untuk target latency saya,
  dan kecocokan dengan aturan wajib FOUNDRY.md bagian 1.
- Riset 3–5 produk nyata sejenis untuk design-brief (jika ada frontend) dan 3–5 praktik/arsitektur
  pembanding untuk domain ini.
- Setiap klaim penting disertai sumber (URL). Tandai yang tidak bisa diverifikasi sebagai "belum terverifikasi".
  Jangan mengarang versi, nama paket, atau fitur.

TAHAP 2 — KEPUTUSAN
- Tulis tabel keputusan: kategori | pilihan | alasan | alternatif yang ditolak | risiko.
- Pilih pola arsitektur (layered atau hexagonal) dan jelaskan alasannya dari kompleksitas domain.
- Rancang: daftar modul, model data (entitas + relasi), daftar event (nama, producer, consumer),
  daftar endpoint/webhook, dan anggaran latency per operasi utama.
- Sebutkan hal yang bertentangan dengan aturan wajib atau perlu keputusan saya (format: masalah → opsi → rekomendasi → dampak).

TAHAP 3 — DOKUMEN (tulis ke docs/, belum ada kode)
- docs/foundry-profile.md : keputusan teknis final, struktur folder aktual rencana, pemetaan peran → lokasi, tooling, batas kode, asumsi.
- docs/requirements.md   : masalah, fitur MVP (maks 5), non-goals, skenario pengguna, ukuran sukses, risiko.
- docs/design-brief.md   : hasil riset referensi, prinsip desain, design tokens (jika ada frontend).
- docs/roadmap-prompts.md: urutan tugas pembangunan; untuk SETIAP tugas, tulis prompt siap pakai yang
  berisi: tujuan, file/modul yang disentuh, aturan FOUNDRY yang relevan, kriteria selesai, dan cara verifikasi.
  Urutan: fondasi (repo, docker-compose, CI, config) → domain + model data + migration →
  modul inti per fitur → event + worker → webhook (jika ada) → frontend per layar → pengujian latency → rilis.
Setelah selesai, tampilkan ringkasan 15 baris dan daftar hal yang butuh persetujuan saya. Berhenti di sini.
```

### 8.2 Prompt mengerjakan satu tugas

```text
Ikuti FOUNDRY.md dan docs/foundry-profile.md. Kerjakan tugas berikut dari docs/roadmap-prompts.md: <nomor/nama tugas>.

Alur: Inspect → Plan → Implement → Verify → Report.
- Inspect: cari kode existing yang bisa dipakai ulang. Jangan membuat folder, helper, atau duplikasi baru.
- Implement: patuhi 8 aturan wajib. ORM hanya di repository. Efek samping lintas modul lewat event.
  Frontend hanya memakai token dari packages/ui dan mengikuti docs/design-brief.md.
- Verify: tulis test untuk logic yang berubah, jalankan lint + typecheck + test, dan ukur latency operasi yang disentuh terhadap anggarannya.
- Jika butuh perubahan struktur, framework, atau migration lama: tulis ADR lalu BERHENTI menunggu persetujuan saya.
- Laporan akhir: ringkasan, file yang berubah (dan alasannya), implementasi existing yang dipakai, hasil test dan latency, risiko.
```

### 8.3 Prompt audit kepatuhan (tanpa mengubah kode)

```text
Audit repo ini terhadap FOUNDRY.md dan docs/foundry-profile.md. Jangan ubah kode.
Periksa 8 aturan wajib: pelanggaran layer/dependency, struktur dan duplikasi, fungsi/file melebihi batas,
akses database di luar repository atau tanpa ORM, penggunaan Redis dan event (idempotensi, TTL/MAXLEN),
N+1 dan query tanpa index, pekerjaan berat sinkron, webhook tanpa verifikasi/retry,
frontend yang melanggar token atau tidak punya state loading/kosong/error, secret hardcode.
Output: tabel Temuan | Lokasi | Aturan | Tingkat (tinggi/sedang/rendah) | Saran. Urutkan dari tingkat tertinggi.
```

---

## 9. Definition of Done (ringkas)

- [ ] Lokasi file sesuai struktur dan Profile; tidak ada duplikasi atau folder kosong
- [ ] Dependency direction benar; domain bebas dari framework/DB/UI
- [ ] Akses database hanya lewat ORM di repository
- [ ] Efek samping lintas modul lewat event berformat seragam; consumer idempotent
- [ ] Webhook (jika ada): tanda tangan diverifikasi, diproses lewat queue, retry dan dead-letter ada
- [ ] Latency operasi yang disentuh diukur dan dalam anggaran
- [ ] Frontend: token dipakai, state loading/kosong/error ada, lolos pemeriksaan "bisa tertukar dengan template generik?"
- [ ] Test untuk logic yang berubah; lint, typecheck, dan test lulus
- [ ] Tidak ada secret hardcode, dead code, atau tipe longgar tanpa alasan

---

## 10. Kapan AI Wajib Berhenti dan Bertanya

1. Butuh folder/modul/layer di luar struktur atau perubahan framework, arsitektur, atau migration lama (tulis ADR).
2. Fitur tidak bisa berjalan di salah satu platform target.
3. Dua struktur existing saling bertentangan, atau Profile tidak cukup untuk menentukan lokasi.
4. Requirement bertentangan dengan 8 aturan wajib.
5. Target latency tampak tidak tercapai dengan desain yang dipilih.

Format pertanyaan: masalah → opsi (minimal 2) → rekomendasi → dampak.
