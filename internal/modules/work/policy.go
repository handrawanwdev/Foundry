package work

import "strings"

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

// CacheBreak marks where a prompt stops being the same from run to run:
// before it are the rules, briefs and blueprint a job reads every round, after
// it the commands, request and state that change. A runner that can keep the
// part before it in the provider's prompt cache sends it as system prompt; the
// marker never reaches a model (see SplitPrompt and JoinPrompt).
const CacheBreak = "\x1eFOUNDRY_CACHE_BREAK\x1e"

// WithPolicy prepends ExecutionPolicy to prompt. An empty prompt stays empty
// so callers can still tell "no work" apart. A prompt without its own
// CacheBreak gets one after the policy, the part every run shares.
func WithPolicy(prompt string) string {
	if prompt == "" {
		return ""
	}
	if !strings.Contains(prompt, CacheBreak) {
		return ExecutionPolicy + CacheBreak + prompt
	}
	return ExecutionPolicy + prompt
}

// SplitPrompt cuts prompt at its first CacheBreak into the static part and
// the rest; other markers are dropped. Without a marker static is "".
func SplitPrompt(prompt string) (static, body string) {
	static, body, ok := strings.Cut(prompt, CacheBreak)
	if !ok {
		return "", prompt
	}
	return static, strings.ReplaceAll(body, CacheBreak, "")
}

// JoinPrompt is prompt as one text, for a reader that takes no split.
func JoinPrompt(prompt string) string { return strings.ReplaceAll(prompt, CacheBreak, "") }
