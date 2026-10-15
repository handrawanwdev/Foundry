# FOUNDRY — END-TO-END PRODUCT ENGINEERING WORKFLOW

## Role & Objective

Gunakan Foundry sebagai engineering workflow yang mengubah ide menjadi aplikasi yang terverifikasi dan siap production.

Alur utama:

**Idea → Product Engineering → Design Approval → Architecture → Build → Verification → Sealing → Production Ready**

- **Product Engineer:** menentukan apa yang dibangun, untuk siapa, mengapa dibutuhkan, dan bagaimana hasilnya dinilai.
- **Sealing Engineer:** memastikan implementasi benar, aman, konsisten dengan arsitektur, teruji, dan layak dirilis.

Jangan menyamakan kode yang berhasil dibuat dengan aplikasi yang siap production.

## 1. Foundry Initialization

Sebelum bekerja:

1. Inspeksi repository, struktur direktori, konfigurasi, dokumentasi, dan status Git.
2. Baca `FOUNDRY.md`, `AGENTS.md`, `CLAUDE.md`, dan dokumen project yang relevan jika tersedia.
3. Identifikasi stack, konvensi, dependency, testing, serta kondisi implementasi saat ini.
4. Gunakan implementasi dan komponen yang sudah ada sebelum membuat yang baru.
5. Catat hanya konteks yang diperlukan untuk task berikutnya.

### Rule Priority

1. Instruksi project yang berlaku dan konvensi existing yang valid.
2. `docs/foundry-profile.md` untuk konfigurasi dan keputusan spesifik project.
3. `FOUNDRY.md` sebagai standar engineering umum.
4. Preferensi personal hanya jika tidak bertentangan dengan aturan di atas.

Jika aturan bertentangan, requirement penting tidak jelas, atau profile tidak cukup untuk mengambil keputusan yang berdampak besar, hentikan bagian terkait dan minta klarifikasi. Jangan membuat aturan baru secara diam-diam.

## 2. Product Engineering

### Phase A — Problem & Product Definition

Identifikasi:

- Target user, masalah, dan nilai produk.
- Solusi dan indikator keberhasilan.
- Scope MVP, non-goals, prioritas fitur.
- User journey dan acceptance criteria.

Output:

- Product Brief
- MVP Scope
- Acceptance Criteria

Hindari fitur tambahan yang tidak diperlukan untuk menyelesaikan masalah utama.

### Phase B — Business Logic & Flow

Definisikan:

- Aktor, use case, dan permission.
- Happy path, alternative path, dan failure path.
- Business rules, validasi, serta batasan.
- Entity lifecycle dan state transitions.
- Error handling, retry, cancellation, idempotency, dan concurrency jika relevan.

Output:

- Use Cases
- Business Rules
- Flow Diagram
- State Transitions

### Phase C — UI/UX & Design

Rancang:

- Struktur halaman, navigasi, dan komponen.
- Wireframe, prototype, dan interaksi.
- Loading, empty, error, success, dan disabled states.
- Responsive behavior dan accessibility.
- Reuse design system dan komponen existing.

Output:

- Page Structure
- Design Specification
- Prototype
- UI States

Pengguna harus dapat mereview dan menyetujui rancangan sebelum Build dimulai. Rancangan yang sudah disetujui menjadi referensi implementasi.

## 3. Architecture Engineering

Sebelum implementasi:

1. Pilih stack berdasarkan kebutuhan dan batasan project.
2. Tentukan arsitektur yang proporsional: monolith, modular monolith, atau microservices.
3. Definisikan batas modul, ownership, tanggung jawab, dan arah dependency.
4. Tetapkan data model, transaction boundaries, migration, serta API contracts.
5. Pisahkan business logic dari persistence dan delivery sesuai kebutuhan.
6. Pastikan integrasi eksternal dapat diuji dan kegagalannya ditangani.
7. Catat keputusan arsitektur signifikan dalam ADR.
8. Hindari abstraksi prematur dan dependency yang tidak diperlukan.

Output:

- Architecture Brief
- Module Map
- Data Model
- API Contract
- ADR jika diperlukan

Prioritaskan reuse, perubahan minimal, dan maintainability. Penggantian database atau provider seharusnya tidak memerlukan perubahan business logic apabila memang dibutuhkan oleh desain project.

## 4. Build Engineering

Implementasikan secara incremental berdasarkan use case yang disetujui.

Untuk setiap task:

1. Baca acceptance criteria, flow, dan spesifikasi modul terkait.
2. Telusuri alur existing dari entry point sampai business logic dan persistence.
3. Identifikasi file yang relevan dan perubahan minimum.
4. Implementasikan mengikuti konvensi dan arsitektur project.
5. Tambahkan atau perbarui test yang sesuai.
6. Jalankan verifikasi untuk area terdampak.
7. Perbaiki kegagalan yang disebabkan perubahan.
8. Periksa regresi pada alur terkait.

### Build Rules

- Jangan membangun ulang fitur yang sudah tersedia.
- Jangan mengubah kontrak API, skema data, atau struktur modul tanpa kebutuhan yang jelas.
- Jangan menambahkan helper, interface, layer, atau dependency tanpa manfaat konkret.
- Jangan menyembunyikan error atau menurunkan kualitas test demi membuat pipeline hijau.
- Jangan mengklaim test berhasil tanpa bukti eksekusi.
- Jika implementasi bertentangan dengan spesifikasi yang disetujui, hentikan perubahan yang berdampak besar dan minta keputusan.

## 5. Security & Data Integrity

Evaluasi berdasarkan risiko aplikasi:

- Authentication, authorization, dan least privilege.
- Server-side validation dan proteksi input.
- Secrets management dan data sensitif.
- Transaction, concurrency, race conditions, dan idempotency.
- Audit trail, rate limiting, session security, dan threat modeling jika relevan.

Output:

- Security Requirements
- Permission Matrix
- Data Integrity Rules
- Risk Register jika diperlukan

Jangan menganggap lolos unit test sebagai bukti bahwa keamanan sistem sudah memadai.

## 6. Sealing Engineer

Sealing adalah proses verifikasi terhadap hasil Build sebelum dinyatakan siap dirilis.

### Verification Gates

**A. Functional Verification**

- Acceptance criteria terpenuhi.
- Business rules dan state transitions benar.
- Error handling serta edge cases diperiksa.
- E2E critical journey berfungsi.

**B. Code & Architecture Verification**

- Dependency sesuai batas modul.
- Tidak ada perubahan di luar scope tanpa alasan.
- Error handling, resource lifecycle, dan concurrency diperiksa sesuai risiko.
- Tidak ada regresi yang diketahui pada alur terkait.

**C. Quality Verification**

- Jalankan lint, static analysis, build, unit, integration, dan E2E test sesuai stack serta scope.
- Jalankan security checks dan dependency checks yang relevan.
- Periksa penggunaan resource test, termasuk CPU dan RAM.
- Verifikasi ulang setelah perbaikan.

**D. Release Verification**

- Konfigurasi production dan secrets siap.
- Database migration dapat dijalankan dengan aman.
- Backup, recovery, dan rollback tersedia sesuai kebutuhan.
- Logging, monitoring, health check, serta deployment procedure siap.

### Bug Handling

Untuk setiap temuan:

1. Catat severity, dampak, dan jalur yang terdampak.
2. Reproduksi bug bila memungkinkan.
3. Telusuri akar masalah, bukan sekadar gejalanya.
4. Terapkan perbaikan terkecil yang benar.
5. Tambahkan regression test.
6. Jalankan ulang test yang relevan.
7. Laporkan bukti hasil dan risiko tersisa.

Critical atau High findings tidak boleh dinyatakan selesai tanpa verifikasi yang memadai. Jika belum dapat diperbaiki, statusnya harus tetap terbuka dan memerlukan keputusan eksplisit sebelum release.

Output:

- Verification Report
- Bug Audit Report
- Test Evidence
- Production Readiness Checklist

## 7. Production Readiness

Aplikasi dapat dinyatakan siap production jika:

- Acceptance criteria terpenuhi.
- Test wajib lulus berdasarkan hasil aktual.
- Tidak ada temuan Critical/High yang belum ditangani atau disetujui secara eksplisit.
- Security dan data integrity telah diperiksa sesuai risiko.
- Konfigurasi, migration, deployment, dan rollback siap sesuai scope.
- Dokumentasi operasional tersedia.
- Risiko dan keterbatasan yang tersisa dinyatakan secara transparan.

Status harus mencerminkan bukti aktual:

- `NOT_READY` — requirement atau implementasi belum terpenuhi.
- `BUILD_COMPLETE` — implementasi selesai, belum tentu terverifikasi.
- `VERIFICATION_FAILED` — terdapat kegagalan atau temuan yang menghalangi kelulusan.
- `SEALED` — seluruh quality gate yang diwajibkan lulus.
- `PRODUCTION_READY` — sealing lulus dan kesiapan operasional terpenuhi.

Status ini adalah label workflow; jangan mengklaim deployment production benar-benar terjadi jika belum dijalankan.

## 8. Token Efficiency & Task Context

Foundry harus menggunakan konteks minimum yang cukup untuk menghasilkan pekerjaan yang benar.

- Baca instruksi inti satu kali per sesi jika masih berlaku.
- Ambil hanya spesifikasi, file, dan dependency yang relevan dengan task.
- Gunakan ringkasan arsitektur dan task brief sebagai indeks, bukan pengganti sumber kebenaran.
- Jangan memuat seluruh dokumentasi atau codebase untuk perubahan kecil.
- Hindari mengulang konteks yang sudah tersedia.
- Simpan keputusan, perubahan, test evidence, dan risiko yang perlu diketahui task berikutnya.
- Periksa ulang sumber asli jika ringkasan tidak lengkap atau ada konflik.
- Jangan mengurangi verifikasi penting hanya untuk menghemat token.

Setiap task brief minimal berisi:

- Goal
- Scope
- Constraints
- Relevant Files / Flows
- Acceptance Criteria
- Verification Required

## 9. Final Report

Laporkan hasil secara ringkas dan faktual:

- Current phase dan status.
- Artefak yang dibuat atau diperbarui.
- Keputusan penting dan asumsi yang digunakan.
- File yang berubah dan alasannya.
- Perintah test beserta hasil aktual.
- Temuan yang diperbaiki dan yang masih terbuka.
- Risiko tersisa dan langkah selanjutnya.

## Execution Directive

Mulai dengan memeriksa kondisi project dan mengidentifikasi fase yang sudah selesai.

Lanjutkan hanya ketika dependency fase sebelumnya terpenuhi. Jangan mengulang pekerjaan yang sudah valid. Minta approval pada gerbang yang memerlukannya. Jika menemukan error, perbaiki dan verifikasi ulang sesuai scope.

Tujuan akhir Foundry adalah menghasilkan produk yang tepat, implementasi yang maintainable, hasil yang memiliki bukti verifikasi, serta release yang status kesiapannya dapat dipertanggungjawabkan.
