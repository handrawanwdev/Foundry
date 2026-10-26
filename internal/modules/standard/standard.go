// Package standard holds the engineering principles Foundry designs with
// and the guide an external coding agent reads in the export package.
// Foundry itself never implements; these texts tell the agent how.
package standard

// Rules are the design principles injected into the discovery prompt.
// They adapt to the project instead of forcing one stack or architecture.
const Rules = `FOUNDRY DESIGN PRINCIPLES:
1. Fit the project. Derive architecture, data, interfaces and quality needs from THIS product and its constraints; do not force a stack, layer or tool it does not need.
2. Honor what is given. Technology, platform, conventions or an existing codebase named by the user are constraints; do not replace them without a stated reason.
3. Clear boundaries. Split by domain or responsibility, not by file type. Each part owns its data and exposes an explicit interface; dependencies have one direction and no cycles.
4. Proportional design. Start as simple as the requirements allow; record when to evolve (scale, team, isolation) instead of designing for it now.
5. Explicit contracts. Data models, APIs, events, messages and protocols are written down before the parts that use them.
6. Quality is designed. State the test strategy, failure handling, security controls and performance budgets the product needs; omit those it does not.
7. No invented facts. Mark assumptions. Never invent file paths, endpoints, tables or runtime facts of an existing system; mark them for the coding agent to verify in the repository.

Stop and ask (problem → ≥2 options → recommendation → impact) when: a requirement conflicts with a constraint; a platform cannot support a feature; the existing structure conflicts with the request; a budget or limit looks unreachable.
`

// PackageGuide is the "how to use" part of the export README.
const PackageGuide = `Paket ini dibuat oleh Foundry. Foundry merancang dan menyusun instruksi; **coding agent eksternal yang mengimplementasikan**.

## Cara memakai paket ini

1. Baca ` + "`manifest.json`" + ` untuk daftar artefak, command, dependency, dan urutan.
2. Blueprint (` + "`blueprint/`" + `) adalah sumber kebenaran. Setiap command menyebut artefak yang menjadi acuannya; baca itu sebelum mengerjakan.
3. Kerjakan command per gelombang. Command dalam satu gelombang tidak saling bergantung dan boleh dikerjakan paralel. Jangan mulai command sebelum semua dependency-nya selesai.
4. Sebelum mengubah kode, periksa repository: pakai struktur, konvensi, dan komponen yang sudah ada. Jangan mengarang path, API, atau tabel yang tidak ada.
5. Satu command selesai bila semua acceptance criteria terpenuhi dan output yang diharapkan diserahkan, dengan bukti (test, perintah, hasil). Foundry tidak menganggap selesai hanya karena instruksi sudah dikirim.
6. Laporkan progres ke Foundry lewat MCP: ` + "`report_progress`" + ` saat mulai, selesai, gagal, terhambat, dilewati (wajib alasan), atau diserahkan untuk ditinjau. Sertakan id laporan yang stabil, ringkasan, dan bukti (` + "`evidence.ref`" + `). Cek command berikutnya dengan ` + "`get_progress`" + `; jangan mulai sebelum dependency terverifikasi. Bila pengiriman gagal, kirim ulang laporan dengan id yang sama.
7. Bila blueprint bertentangan dengan kondisi repository atau dengan command lain, berhenti dan laporkan; jangan memutuskan sendiri.
8. Untuk layar: ` + "`blueprint/design-rules.md`" + ` (aturan desain dan token) dan ` + "`blueprint/layout-spec.md`" + ` (pengguna, tujuan, information architecture, navigasi, halaman, section, state, pemetaan field, dan keputusan desain) adalah acuannya; versi JSON ada di ` + "`design/`" + `. Semua nilai visual dari token; jangan nilai hardcode. Halaman bertanda *sudah ada* diubah seperlunya, tidak dibuat ulang. Halaman tanpa id di command belum tercakup workflow.
9. ` + "`blueprint/acceptance.md`" + ` berisi kriteria penerimaan per requirement dan Definition of Done; command menyebut requirement yang dicakupnya. Bukti yang diharapkan adalah syarat, bukan hasil: belum ada yang terverifikasi sampai agent menjalankannya dan Foundry menerima laporannya. ` + "`traceability.md`" + ` memetakan requirement ke use case, kriteria, dan command.
10. ` + "`blueprint/decisions.md`" + ` berisi asumsi, keputusan, risiko, dan area yang sengaja tidak dicakup. Keputusan di bagian *Diputuskan* mengikat dan mengalahkan asumsi, rekomendasi, atau aturan di artefak lain dan di command; bila ada yang bertentangan, ikuti keputusan dan laporkan pertentangannya. Jangan mengimplementasikan bagian yang bergantung pada keputusan yang masih terbuka; tanyakan ke user.
11. Setelah membangun layar, kirim snapshot browser terukur + screenshot setiap viewport dan state ke Foundry (` + "`record_visual_review`" + `). Build dan unit test bukan bukti kualitas UI. Baca temuan di ` + "`review/visual.md`" + ` (bila ada); temuan berat menghasilkan command koreksi lewat ` + "`create_visual_fixes`" + `, lalu kirim ulang snapshot untuk verifikasi.
`

// AgentsMD is written at the project root when it has no AGENTS.md.
const AgentsMD = `# Agents

Rancangan project ini ada di docs/foundry/ (dibuat oleh Foundry).

- Mulai dari docs/foundry/README.md dan docs/foundry/manifest.json.
- Blueprint di docs/foundry/blueprint/ adalah sumber kebenaran.
- Kerjakan command di docs/foundry/workflow/ sesuai urutan gelombang dan dependency.
- Untuk UI: ikuti docs/foundry/blueprint/design-rules.md dan layout-spec.md (pengguna, tujuan, navigasi, keputusan desain, pemetaan field, state).
- Setelah layar jadi: kirim snapshot browser terukur + screenshot ke Foundry (record_visual_review). Build/unit test bukan bukti kualitas UI; halaman tanpa screenshot belum diuji.
- Periksa repository sebelum mengubah kode; jangan mengarang path atau API.
- Penuhi acceptance criteria tiap command (docs/foundry/blueprint/acceptance.md) dan laporkan buktinya ke Foundry lewat report_progress (MCP). Jangan anggap selesai tanpa bukti.
- Keputusan di docs/foundry/blueprint/decisions.md bagian *Diputuskan* mengikat dan menimpa artefak, use case, aturan desain, layout, dan command lain. Bila ada yang bertentangan, ikuti keputusan dan laporkan pertentangannya.
- Jangan memutuskan sendiri hal yang tercatat terbuka di docs/foundry/blueprint/decisions.md.
`
