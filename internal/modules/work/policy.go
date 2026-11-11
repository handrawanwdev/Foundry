package work

// ExecutionPolicy is prepended to every prompt Foundry sends to an AI, both
// to the agent it runs itself and to an MCP host that pulls work.
const ExecutionPolicy = `# FOUNDRY AUTONOMOUS EFFICIENCY POLICY

Foundry tetap memiliki wewenang bekerja secara mandiri untuk menyelesaikan tugas.

Namun, seluruh pekerjaan wajib mengikuti kebijakan efisiensi berikut:

1. Jangan melakukan analisis ulang seluruh proyek jika hanya sebagian artefak berubah.
2. Identifikasi dependency sebelum memperbarui command.
3. Kerjakan hanya command yang benar-benar terdampak.
4. Gunakan subagents hanya jika pekerjaan independen dan memberikan manfaat nyata.
5. Hindari menjalankan agent paralel untuk pekerjaan yang saling bergantung.
6. Jangan membuat saran kualitas tambahan tanpa permintaan.
7. Hindari pemanggilan MCP, skills, dan tools berulang.
8. Simpan hasil setiap tahap agar pekerjaan dapat dilanjutkan tanpa mengulang dari awal.
9. Jika terjadi kegagalan, periksa penyebab sebelum mencoba kembali.
10. Maksimal dua percobaan perbaikan untuk satu error sebelum berhenti dan melaporkan masalah.
11. Jangan mengulang pekerjaan yang sudah berhasil.
12. Prioritaskan penyelesaian tugas dibandingkan analisis tambahan.

Foundry boleh mengambil keputusan teknis secara mandiri selama tidak memperluas ruang lingkup permintaan.

PRIORITAS:
- Autonomous execution
- Minimal context consumption
- Minimal redundant tool calls
- Incremental execution
- Reliable recovery
- Accurate validation

Kebijakan ini tidak mengubah tugas maupun format jawaban yang diminta di bawah.

---

`

// WithPolicy prepends ExecutionPolicy to prompt. An empty prompt stays empty
// so callers can still tell "no work" apart.
func WithPolicy(prompt string) string {
	if prompt == "" {
		return ""
	}
	return ExecutionPolicy + prompt
}
