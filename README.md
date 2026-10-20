# Foundry

**Universal Software Blueprint & Command Workflow Designer**

Foundry mengubah ide, kebutuhan bisnis, atau perubahan software menjadi **blueprint terstruktur dan command workflow** yang siap digunakan oleh coding agent eksternal.

Foundry membantu developer merencanakan software sebelum implementasi dimulai. Kebutuhan, arsitektur, teknologi, desain UI, use case, acceptance criteria, dan instruksi implementasi disusun menjadi satu spesifikasi yang saling terhubung, dapat ditinjau, dan dapat diekspor.

Foundry bukan coding agent dan bukan IDE. Foundry adalah **lapisan perencanaan dan spesifikasi yang menghubungkan ide dengan proses implementasi software**.

## Mengapa Foundry?

Menggunakan AI untuk membangun software tidak cukup hanya dengan memberikan prompt lalu meminta AI menulis kode.

Tanpa perencanaan yang terstruktur, implementasi berisiko mengalami:

- Kebutuhan yang ambigu atau saling bertentangan.
- Arsitektur dan teknologi yang tidak sesuai kebutuhan.
- Instruksi implementasi yang kehilangan konteks.
- Perubahan spesifikasi yang tidak terdeteksi dampaknya.
- Fitur yang selesai dibuat tetapi belum memenuhi kriteria penerimaan.
- Hasil kerja yang sulit ditinjau dan dilanjutkan oleh agent lain.

Foundry mengatasi masalah tersebut dengan memisahkan **perancangan software** dari **implementasi software**.

## Cara kerja

```text
┌──────────────────────────────┐
│  1. PROMPT                   │
│  Ide, kebutuhan, atau        │
│  perubahan software         │
└──────────────┬───────────────┘
               ▼
┌──────────────────────────────┐
│  2. BLUEPRINT                │
│  Requirements, arsitektur,   │
│  teknologi, desain, use case │
│  dan acceptance criteria     │
└──────────────┬───────────────┘
               ▼
┌──────────────────────────────┐
│  3. COMMAND WORKFLOW         │
│  Instruksi terstruktur,      │
│  dependency dan urutan kerja │
└──────────────┬───────────────┘
               ▼
┌──────────────────────────────┐
│  4. REVIEW & REVISI          │
│  Periksa spesifikasi,        │
│  validasi dan dampak perubahan│
└──────────────┬───────────────┘
               ▼
┌──────────────────────────────┐
│  5. FINALISASI & EXPORT      │
│  Paket dokumentasi siap      │
│  digunakan coding agent     │
└──────────────┬───────────────┘
               ▼
       Coding Agent Eksternal
       mengimplementasikan
       software sesuai spesifikasi
```

Foundry menyiapkan instruksi dan spesifikasi. Developer tetap memiliki kendali atas keputusan, review, dan persetujuan sebelum spesifikasi difinalisasi.

## Apa yang dihasilkan Foundry?

### 1. Blueprint software

Blueprint merupakan sumber kebenaran untuk rancangan proyek.

Artefaknya menyesuaikan kebutuhan proyek, bukan memaksakan satu template untuk semua jenis software.

| Artefak                    | Tujuan                                                                                                         |
| -------------------------- | -------------------------------------------------------------------------------------------------------------- |
| Requirements               | Mendefinisikan kebutuhan dan batasan sistem                                                                    |
| Architecture               | Menjelaskan arsitektur dan pembagian tanggung jawab                                                            |
| Technology                 | Mencatat pilihan teknologi dan alasan pemilihannya                                                             |
| Design Rules               | Mendefinisikan prinsip UI, token desain, dan aturan UX bila diperlukan                                         |
| Layout Specification       | Mendefinisikan halaman, komponen, navigasi, dan state UI bila diperlukan                                       |
| Use Cases                  | Menjelaskan aktor, alur, aturan bisnis, dan kondisi kegagalan                                                  |
| Acceptance Criteria        | Menentukan kondisi yang harus dipenuhi fitur                                                                   |
| Decisions                  | Mencatat asumsi, pertanyaan, konflik, risiko, dan penyelesaiannya                                              |
| Project-specific Artifacts | Menampung spesifikasi khusus seperti kontrak API, model domain, protokol, threat model, dan strategi pengujian |

Tidak semua proyek memerlukan semua artefak. Proyek firmware, misalnya, tidak dipaksa memiliki rancangan UI. Sebaliknya, produk dengan antarmuka harus memiliki aturan desain dan spesifikasi layout yang valid.

### 2. Command workflow

Blueprint menjelaskan **apa yang harus dibangun dan mengapa**. Command workflow menjelaskan **bagaimana coding agent harus mengerjakannya**.

Setiap command dapat berisi:

- Tujuan dan konteks pekerjaan.
- Input dan artefak referensi.
- Instruksi implementasi.
- Dependency terhadap command lain.
- Requirement yang harus dipenuhi.
- Batasan implementasi.
- Output yang diharapkan.
- Acceptance criteria.

Contoh konseptual:

```text
CMD-001 — Define Domain Model
    ↓
CMD-002 — Implement Persistence
    ↓
CMD-003 — Implement Business Use Cases
    ↓
CMD-004 — Implement API
    ↓
CMD-005 — Verify Acceptance Criteria
```

Urutan aktual ditentukan oleh dependency. Command yang tidak saling bergantung dapat ditempatkan dalam gelombang yang sama dan dikerjakan secara paralel oleh coding agent.

Foundry memvalidasi dependency, referensi artefak, cakupan requirement, dan siklus dependency sebelum workflow dapat difinalisasi.

### 3. Traceability

Foundry menghubungkan requirement dengan implementasi yang direncanakan.

```text
Requirement
    → Feature
        → Use Case
            → Acceptance Criteria
                → Command
```

Hubungan ini membantu developer menelusuri alasan sebuah pekerjaan diperlukan, bagian spesifikasi yang menjadi acuannya, serta kriteria yang harus dipenuhi.

### 4. Review dan pengelolaan perubahan

Blueprint dan command workflow dapat direvisi melalui AI atau diedit secara manual.

Foundry menggunakan hash artefak untuk mendeteksi perubahan terhadap spesifikasi yang dirujuk command. Perubahan yang berdampak akan menandai command terkait untuk ditinjau kembali.

Artefak yang merujuk artefak lain juga dapat ditandai `needs_review` ketika referensinya berubah.

Dengan demikian, perubahan spesifikasi tidak otomatis dianggap aman hanya karena instruksi sebelumnya masih tersedia.

### 5. Paket ekspor

Setelah blueprint dan workflow memenuhi validasi dan difinalisasi, Foundry menghasilkan paket dokumentasi di `docs/foundry/`.

Contoh hasil:

```text
docs/foundry/
├── README.md
├── manifest.json
├── spec.json
├── traceability.md
├── blueprint/
│   ├── requirements.md
│   ├── architecture.md
│   └── threat-model.md
├── workflow/
│   ├── CMD-001.md
│   ├── CMD-002.md
│   └── workflow.json
├── design/
│   ├── design-rules.json
│   └── layout-spec.json
└── adr/
    └── 0001-stack.md
```

File yang dihasilkan menyesuaikan artefak yang benar-benar berlaku bagi proyek. Struktur di atas merupakan ilustrasi, bukan daftar file wajib.

Paket dapat diekspor sebagai ZIP dan digunakan sebagai referensi oleh coding agent eksternal. Dokumentasi ini juga dapat disimpan di repository agar menjadi acuan bersama bagi developer dan agent.

## Software seperti apa yang didukung?

Foundry dirancang untuk berbagai jenis software, bukan hanya aplikasi web atau backend.

| Platform | Contoh                                                             |
| -------- | ------------------------------------------------------------------ |
| Web      | Dashboard, SaaS, aplikasi bisnis, konsol administrasi              |
| Android  | Aplikasi mobile native atau lintas platform                        |
| iOS      | Aplikasi iPhone dan iPad                                           |
| Desktop  | Aplikasi Linux, Windows, dan macOS                                 |
| Backend  | API, layanan bisnis, sistem transaksi                              |
| CLI      | Developer tools, automasi, command-line utilities                  |
| Embedded | Firmware, IoT, robotics                                            |
| Other    | Library, SDK, pipeline data, sistem ML, infrastruktur, dan tooling |

Satu proyek dapat mencakup beberapa platform sekaligus. Blueprint akan menyesuaikan kebutuhan, batasan, dan hubungan antarbagian sistem.

## Teknologi dan arsitektur

Foundry tidak memaksakan stack tertentu.

Rekomendasi teknologi mempertimbangkan kebutuhan proyek, kode yang sudah ada, preferensi developer, kompatibilitas, serta profil proyek bila relevan.

Keputusan dapat mencakup:

- Bahasa pemrograman dan runtime.
- Frontend dan UI library.
- Backend dan protokol API.
- Database, cache, messaging, dan penyimpanan.
- Arsitektur aplikasi dan struktur repository.
- Testing, linting, dan formatting.
- Deployment dan observability.

Rekomendasi bukan keputusan final. Developer dapat meninjau dan menyetujui pilihan yang sesuai.

Untuk project existing, Foundry mempertahankan teknologi, struktur, dan design system yang sudah ada sebagai dasar rancangan. Perubahan teknologi yang terdeteksi dari kode diperlakukan sebagai perubahan atau migrasi yang memerlukan alasan, bukan penggantian otomatis.

Komponen opsional seperti cache, messaging, worker terpisah, dan search hanya direkomendasikan bila memang dibutuhkan.

## Desain UI

Untuk produk yang memiliki layar, Foundry menyusun dua artefak utama:

- **Design Rules** — arah visual, token warna dan tipografi, komponen, aksesibilitas, responsivitas, dan aturan interaksi.
- **Layout Specification** — shell, halaman, route, section, hierarki visual, navigasi, data, dan state UI.

Untuk project existing, Foundry dapat menggunakan hasil pemindaian kode sebagai dasar rancangan, termasuk route, layout shell, komponen bersama, dan token desain yang terdeteksi.

Rancangan disimpan sebagai teks dan data terstruktur yang dapat ditinjau, diedit, divalidasi, dan diekspor. Foundry tidak bertujuan menjadi editor desain visual atau pengganti Figma.

## Integrasi coding agent

Foundry dirancang untuk bekerja dengan coding agent eksternal, bukan bergantung pada satu penyedia AI.

Agent yang kompatibel dapat menggunakan instruksi hasil ekspor atau berinteraksi melalui Model Context Protocol (MCP), sesuai kemampuan host dan konfigurasi yang tersedia.

Contoh agent yang dapat menjadi bagian dari workflow:

- Claude Code
- OpenAI Codex
- OpenCode
- Qwen Code
- Gemini
- Agent CLI khusus atau implementasi lain yang kompatibel

Nama-nama tersebut merupakan contoh target integrasi, bukan jaminan bahwa setiap agent sudah diuji atau didukung pada semua jalur integrasi.

Melalui MCP, host yang kompatibel dapat membaca konteks proyek, mengambil blueprint dan workflow, lalu mengirimkan hasil pekerjaan rancangan kembali ke Foundry.

## Apa yang tidak dilakukan Foundry?

Batas tanggung jawab ini merupakan bagian penting dari desain Foundry.

Foundry:

- Tidak menulis kode aplikasi sebagai hasil implementasi.
- Tidak menjalankan build, test, atau deployment aplikasi.
- Tidak mengeksekusi command workflow.
- Tidak menjalankan coding agent untuk mengimplementasikan software.
- Tidak membuat scaffold atau struktur folder aplikasi.
- Tidak menggantikan IDE, repository, atau tool implementasi.

Foundry menulis state perencanaannya di `.foundry/` dan menghasilkan paket dokumentasi proyek saat finalisasi. Implementasi software tetap dilakukan oleh developer dan coding agent eksternal.

## Cara menjalankan

Dari repository Foundry:

```bash
# Build UI dan binary
make build

# Jalankan Foundry
./bin/foundry serve

# Buka project tertentu
./bin/foundry serve ~/kerja/klinik

# Tampilkan konfigurasi MCP
./bin/foundry mcp --print-config

# Jalankan test suite
make test
```

Untuk pengembangan, UI dan API juga dapat dijalankan secara terpisah:

```bash
make dev-api
make dev-web
```

Periksa dokumentasi repository untuk prasyarat, konfigurasi AI, dan petunjuk instalasi sesuai versi yang digunakan.

## Prinsip desain

Foundry dibangun dengan prinsip berikut:

1. **Requirements before implementation** — kebutuhan harus cukup jelas sebelum menjadi instruksi implementasi.
2. **Blueprint as source of truth** — artefak yang saling terkait menjadi acuan rancangan proyek.
3. **Project-adaptive** — rancangan menyesuaikan jenis dan kebutuhan software.
4. **Existing-project aware** — project existing tidak dipaksa mengikuti template baru.
5. **Explicit decisions** — keputusan penting, asumsi, konflik, dan risiko harus terlihat.
6. **Validated dependencies** — workflow harus memiliki dependency yang valid dan bebas siklus.
7. **Traceable requirements** — requirement harus dapat ditelusuri ke kriteria penerimaan dan command terkait.
8. **Human approval** — keputusan penting dan finalisasi berada di bawah kendali developer.
9. **Portable output** — hasil rancangan dapat diekspor dan digunakan di luar Foundry.
10. **Implementation agnostic** — Foundry merancang workflow tanpa mengambil alih peran coding agent.

## Status proyek

Foundry ditujukan untuk menjadi lapisan perencanaan software yang universal bagi developer yang bekerja dengan AI coding agent.

Status fitur, kompatibilitas agent, batasan versi, dan kesiapan penggunaan harus mengacu pada implementasi serta pengujian aktual repository.

---

**Foundry — From software ideas to structured blueprints and actionable command workflows.**

Turn ideas into specifications. Turn specifications into workflows. Let your coding agent implement.
