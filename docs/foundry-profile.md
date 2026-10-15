# Foundry Profile

Profil proyek Foundry sendiri. Prioritas: konvensi existing → dokumen ini → `docs/FOUNDRY.md` → preferensi pribadi.

## Identitas

- Jenis: CLI + HTTP API lokal + web UI (SPA di-embed ke binary) + MCP stdio server.
- Bahasa: Go 1.22 (backend, tanpa dependency pihak ketiga di `go.mod`), TypeScript/React/Vite/Zustand (`apps/web`).
- Persistensi: file JSON di `.foundry/` per proyek. Tanpa database, ORM, atau Redis.

## Struktur

```text
cmd/foundry/              entry point CLI
internal/core/            model domain + store (tanpa HTTP/UI)
internal/modules/         use case per fitur: agent, checks, gitwt, pipeline, render,
                          scan, stack, standard, structure, uiscan, verify, work
internal/transport/       httpapi (REST + SSE + SPA), mcp (JSON-RPC stdio)
internal/transport/httpapi/respond/   helper response standar
apps/web/                 frontend
docs/                     dokumentasi
```

Dependency: `transport → modules → core`. `respond` tidak mengimpor modul bisnis.

## Pengecualian dari FOUNDRY.md

| Aturan | Status | Alasan |
| ------ | ------ | ------ |
| 3 ORM | Tidak berlaku | Tidak ada database; state berupa file JSON |
| 4 Redis | Tidak berlaku | Alat lokal satu proses; tidak ada cache/queue/rate limit terdistribusi |
| 5 Event | Tidak berlaku | Satu proses; SSE `/api/v1/events` hanya memberi sinyal perubahan state ke UI |
| 2 file ≤ 300 baris | Dilanggar sementara | `core/model.go`, `httpapi/server.go`, `pipeline/build.go`, `work/blueprint.go`, `work/design.go`, `core/designir.go`, `work/designtemplate.go` melebihi batas. Pemecahan per domain (tanpa ubah perilaku) menunggu persetujuan |

## Kontrak Response API

Lihat bagian "Kontrak Response API" di `README.md`.

## Verifikasi

`make check` = `go vet` + `go test` + typecheck + test frontend + build. Format Go: `gofmt -l .` harus kosong.
