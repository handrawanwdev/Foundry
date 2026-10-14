// Package standard encodes FOUNDRY.md as the contract generated projects
// follow. Foundry itself stays an orchestrator; this package writes the
// standard into the project the coding agents build.
package standard

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/handrawanw/foundry/internal/core"
)

// The compact FOUNDRY.md contract injected into AI prompts, in pieces so a
// task gets the rules that apply to it. Pieces keep the original numbering.
const (
	rulesHead = `FOUNDRY RULES (mandatory, FOUNDRY.md):
1. Modular + Clean Architecture. Dependency points inward: entry → use case → domain. Domain has no framework, DB, or UI.
2. Readable code: function ≤ 40 lines, params ≤ 4, nesting ≤ 3, file ≤ 300 lines. No magic values, dead code, or loose types (any).
3. ORM required. Database access only in the repository layer. Raw SQL only for documented heavy aggregation or bulk. A query builder without a model layer is not an ORM.
`
	ruleRedis = `4. Redis required: cache, job queue, rate limit, and events. Database stays source of truth; Redis may die without data loss.
`
	ruleEvents = `5. Cross-module side effects go through events with a uniform envelope. No direct cross-module calls for side effects.
`
	ruleLatency = `6. Low latency: budget from the survey, measured. No N+1, no heavy sync work, no unindexed filter/join/sort.
`
	ruleFrontend = `7. Frontend is not AI slop: tokens only, realistic copy, loading/empty/error/success, WCAG AA, one primary action per screen.
`
	ruleStructure = `8. Structure is the contract. Follow the existing tree. New folder/helper needs a reason. Architecture change needs an ADR and user approval.
`
	eventsBlock = `
Event envelope (packages/contracts): {"eventId":"uuid","type":"domain.event","version":1,"occurredAt":"RFC3339","correlationId":"uuid","payload":{}}.
Name events domain.happened (past tense). Consumers are idempotent by eventId. Payload change bumps version.
Durable events: Redis Streams + consumer group + ack + retry + dead-letter. Ephemeral (UI, cache invalidation): Redis Pub/Sub.
Outbox: write the event to table outbox in the same DB transaction, then a relay publishes to Redis.
`
	webhookBlock = `Inbound webhook: verify HMAC+timestamp, reply 2xx, process via queue, idempotent, store raw payload.
Outbound webhook: per-endpoint secret, HMAC+eventId+timestamp, backoff+jitter, dead-letter, send from worker.
`
	stopAsk = `
Stop and ask (problem → ≥2 options → recommendation → impact) when: structure/framework/old-migration change; a feature cannot run on a target platform; two existing structures conflict; a requirement fights these rules; the latency budget looks unreachable.
`
)

// Rules is the full contract (every piece), used by discovery and planning.
const Rules = rulesHead + ruleRedis + ruleEvents + ruleLatency + ruleFrontend + ruleStructure + eventsBlock + webhookBlock + stopAsk

// Scope says which conditional rules a task needs. Rules 1, 2, 3, 5, 8 and
// the stop-and-ask clause are global and always sent: architecture, code
// quality, ORM boundary, the event contract, structure.
type Scope struct {
	Frontend bool // UI work: rule 7
	Backend  bool // server work: Redis, latency, event/outbox details
	Webhooks bool // webhook details
}

// AllScope keeps every rule (unknown or cross-cutting work).
var AllScope = Scope{Frontend: true, Backend: true, Webhooks: true}

// RulesFor is the contract limited to scope. Global pieces never drop.
func RulesFor(s Scope) string {
	var b strings.Builder
	b.WriteString(rulesHead)
	if s.Backend {
		b.WriteString(ruleRedis)
	}
	b.WriteString(ruleEvents)
	if s.Backend {
		b.WriteString(ruleLatency)
	}
	if s.Frontend {
		b.WriteString(ruleFrontend)
	}
	b.WriteString(ruleStructure)
	if s.Backend {
		b.WriteString(eventsBlock)
	}
	if s.Webhooks {
		b.WriteString(webhookBlock)
	}
	b.WriteString(stopAsk)
	return b.String()
}

// Version identifies the rule text (and so a change to it): a short hash of
// the full contract. Memory and caches keyed on rules include it.
func Version() string {
	sum := sha256.Sum256([]byte(Rules))
	return hex.EncodeToString(sum[:6])
}

const AgentsMD = `# Agents

Ikuti FOUNDRY.md di root dan docs/foundry-profile.md.

Alur tugas: Inspect → Plan → Implement → Verify → Report.
- Inspect: pakai kode yang sudah ada. Jangan buat folder, helper, atau duplikasi baru.
- Implement: 8 aturan wajib FOUNDRY.md. ORM hanya di repository. Efek samping lintas modul lewat event. Frontend hanya token dari packages/ui dan docs/design-brief.md.
- Verify: test untuk logic yang berubah; lint + typecheck + test; ukur latency terhadap anggaran.
- Perubahan struktur, framework, atau migration lama: tulis ADR di docs/adr/ lalu berhenti menunggu persetujuan.
`

// Document is the FOUNDRY.md written to a generated project's root.
func Document(p *core.Project) string {
	var b strings.Builder
	b.WriteString("# FOUNDRY.md\n\n")
	b.WriteString("> Standar wajib project ini. Foundry menulis file ini dari keputusan yang sudah disetujui.\n")
	b.WriteString("> Coding agent membaca FOUNDRY.md, docs/foundry-profile.md, dan docs/requirements.md sebelum menulis kode.\n\n")
	b.WriteString("## Delapan aturan wajib\n\n")
	b.WriteString(Rules)
	b.WriteString("\n## Struktur folder\n\n")
	b.WriteString("Ambil hanya yang dibutuhkan; tidak ada folder kosong.\n\n")
	b.WriteString("```text\n")
	b.WriteString("apps/api  apps/worker  apps/web  apps/mobile  apps/desktop  apps/cli\n")
	b.WriteString("packages/domain  packages/contracts  packages/ui  packages/config\n")
	b.WriteString("docs/  scripts/  docker-compose.yml  .env.example  FOUNDRY.md\n")
	b.WriteString("```\n\n")
	b.WriteString("Backend per fitur, bukan per jenis file:\n\n")
	b.WriteString("```text\n")
	b.WriteString("src/config  src/infra  src/common  src/modules/{fitur}/\n")
	b.WriteString("  {fitur}.controller  {fitur}.service  {fitur}.repository\n")
	b.WriteString("  {fitur}.events  dto/  {fitur}.mapper\n")
	b.WriteString("```\n\n")
	b.WriteString("Dilarang: folder helpers/utils/misc/manager sebagai penampung; business logic di common; model ORM bocor ke domain atau response.\n\n")
	if p != nil && p.Structure != nil && p.Structure.IR != nil {
		b.WriteString("Struktur yang disetujui untuk project ini ada di docs/foundry-profile.md.\n\n")
	}
	b.WriteString("## Definition of Done\n\n")
	b.WriteString("- Lokasi file sesuai struktur dan Profile; tidak ada duplikasi atau folder kosong\n")
	b.WriteString("- Dependency direction benar; domain bebas dari framework/DB/UI\n")
	b.WriteString("- Akses database hanya lewat ORM di repository\n")
	b.WriteString("- Efek samping lintas modul lewat event berformat seragam; consumer idempotent\n")
	b.WriteString("- Webhook (jika ada): tanda tangan diverifikasi, diproses lewat queue, retry dan dead-letter ada\n")
	b.WriteString("- Latency operasi yang disentuh diukur dan dalam anggaran\n")
	b.WriteString("- Frontend: token dipakai, state loading/kosong/error ada, tidak tertukar dengan template generik\n")
	b.WriteString("- Test untuk logic yang berubah; lint, typecheck, dan test lulus\n")
	b.WriteString("- Tidak ada secret hardcode, dead code, atau tipe longgar tanpa alasan\n")
	return b.String()
}

func latencyLabel(p *core.Project) string {
	if p != nil && p.Survey != nil {
		switch p.Survey.Latency {
		case core.SurveyLatency200:
			return "< 200 ms p95"
		case core.SurveyLatency100:
			return "< 100 ms p95"
		}
	}
	if p != nil && p.Blueprint != nil && p.Blueprint.Performance.Budget != "" {
		return p.Blueprint.Performance.Budget
	}
	return "< 500 ms p95"
}
