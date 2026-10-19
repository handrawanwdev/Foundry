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
5. Satu command selesai bila semua acceptance criteria terpenuhi dan output yang diharapkan diserahkan, dengan bukti (test, perintah, hasil).
6. Bila blueprint bertentangan dengan kondisi repository atau dengan command lain, berhenti dan laporkan; jangan memutuskan sendiri.
`

// AgentsMD is written at the project root when it has no AGENTS.md.
const AgentsMD = `# Agents

Rancangan project ini ada di docs/foundry/ (dibuat oleh Foundry).

- Mulai dari docs/foundry/README.md dan docs/foundry/manifest.json.
- Blueprint di docs/foundry/blueprint/ adalah sumber kebenaran.
- Kerjakan command di docs/foundry/workflow/ sesuai urutan gelombang dan dependency.
- Periksa repository sebelum mengubah kode; jangan mengarang path atau API.
- Penuhi acceptance criteria tiap command dan laporkan buktinya.
`
