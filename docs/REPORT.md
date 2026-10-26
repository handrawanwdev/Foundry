# Laporan Trace UI/UX Foundry

**Tanggal:** 2026-10-26
**Fokus kriteria:** (1) UI harus berbasis data, (2) UX modern seperti web manusia pada umumnya.

## 1. Stack UI

| Aspek | Fakta | Bukti |
|---|---|---|
| Framework | React 19.3 + TypeScript 5.9 (strict) | `apps/web/package.json:13,22` |
| Build | Vite 8 (rolldown), proxy `/api` → `127.0.0.1:8080` | `apps/web/vite.config.ts:6-15` |
| Routing | react-router-dom 7 (`BrowserRouter`) | `src/main.tsx:3,9`, `App.tsx:75-83` |
| State | zustand 5 — 3 store (`ai`, `project`, `workbench`) | `src/store/*.ts` |
| Styling | Satu file CSS manual `src/styles.css` (518 baris); tanpa Tailwind/CSS-in-JS/component library | `src/styles.css`, `main.tsx:5` |
| Backend | Go (embed UI ke binary) | `apps/web/embed.go` |
| Data layer | Fetch wrapper + envelope `{success,data\|error}` + SSE `EventSource` | `src/api.ts:12-28`, `src/store/project.ts:38-72` |
| Test | Vitest, hanya logika murni (7 file `.ts`); tidak ada render/component/a11y test | `package.json:10` |

## 2. Pola Desain

**Terpusat (bagus):**
- Semua token di satu blok `:root` (`styles.css:1-35`) + override light theme `prefers-color-scheme` (`styles.css:36-62`).
- Nol hex/rgba inline di seluruh `.tsx`.
- Pola **data → store → map()**: nav dari `flow.ts:5-10` (`STEPS`), artefak digroup dari `blueprint.ts:40-50`, label/status dari map data (`blueprint.ts:29-84`, `tech.ts:3-20`), form survey dari konstanta `tell.ts` (`Tell.tsx:80-113`), katalog teknologi dari API (`TechStack.tsx:37,76-84`).

**Ironi inti:** Foundry punya model design-system lengkap di data — `DesignTokens`, `DesignRules`, `LayoutSpec` dengan `states: loading|empty|error|...` (`types.ts:150-178`), divalidasi backend (`internal/core/uidesign.go:186-190,524-525`) dan di-scan dari kode (`internal/modules/uiscan/tokens.go`) — untuk produk yang di-generate, **tetapi UI Foundry sendiri tidak memakainya**.

**Pola markup manual:**
- Dua workspace menulis ulang struktur identik `.ws/.ws-nav/.ws-main/.ws-dock`: `Blueprint.tsx:54-127` vs `Workflow.tsx:73-147`.
- Editor data = `<textarea>` JSON mentah, bukan form dari schema: `Blueprint.tsx:253-266,303-342`.

## 3. Inventaris Kemampuan

| Item | Ada? | Bukti |
|---|---|---|
| Routing + guard + redirect stage | Ya | `App.tsx:68-83`, `flow.ts:15-30` |
| Layout shell + topbar + nav | Ya | `Workbench.tsx:16-64` |
| Loading state | Ya, teks+spinner; **tanpa skeleton** | `App.tsx:64`, `Blueprint.tsx:103` |
| Error state | Parsial — toast global, `role="alert"` hanya 1 tempat | `Workbench.tsx:21-25`, `JobBanner.tsx:10-22` |
| Empty state | Ya, duplikat 3× | `Blueprint.tsx:30-38`, `Workflow.tsx:37-45`, `Export.tsx:20-28` |
| Responsive | Ya — 4 media query + 1 container query | `styles.css:218,279,453,474,442-451` |
| Dark mode | Auto-only (`prefers-color-scheme`), tanpa toggle | `styles.css:33,36-62` |
| Reduced motion | Ya (4 tempat) | `styles.css:122,275,311` |
| Error boundary | **Tidak ada** | grep 0 |
| Skeleton / Suspense / lazy | **Tidak ada** | grep 0 |
| Infinite scroll / pagination / search | **Tidak ada** | list full render |
| Skip link | **Tidak ada** | — |

## 4. Kelemahan — Kriteria A: UI Data-Driven

| # | Kelemahan | Bukti |
|---|---|---|
| A1 | **Model design tidak dirender sebagai UI** — `DesignTokens`/`DesignRules` hanya di-edit sebagai JSON mentah (textarea 24 baris); tidak ada tabel token, preview warna/typografi, atau form dari schema | `types.ts:150-178` ↔ `Blueprint.tsx:253-266,303-342` |
| A2 | Duplikasi layout workspace — markup `.ws/.ws-nav/.ws-main/.ws-dock` ditulis manual 2×, tanpa komponen generik `<Workspace>` | `Blueprint.tsx:54-127` vs `Workflow.tsx:73-147` |
| A3 | Duplikasi empty state — struktur `.dw-empty > h1 + p + Link` di 3 halaman dengan copy berbeda | `Blueprint.tsx:30-38`, `Workflow.tsx:37-45`, `Export.tsx:20-28` |
| A4 | Daftar platform didefinisikan 3× (`PlatformKind`, `PLATFORM_LABEL`, `KINDS`) — bisa tidak sinkron | `types.ts:5`, `flow.ts:41-44`, `Platform.tsx:11` |
| A5 | Map `SOURCE` didefinisikan 2× dengan isi berbeda (`structure:` vs `technology:`) | `TechStack.tsx:19` vs `StructurePanel.tsx:12` |
| A6 | Palet status di-copy-paste per komponen CSS, bukan memakai token status tunggal | `styles.css:368-378,496-498,512-515,254,142` |
| A7 | Warna hardcoded di luar sistem token (brand agent, toast, primary text) | `styles.css:244-248,153-154,81,138` |
| A8 | Referensi class mati — style yang dimaksud tidak ada → visual tidak konsisten | `.fg` (`TechStack.tsx:65`, `StructurePanel.tsx:39`), `.brand-name` (`Workbench.tsx:39`), `.delete-impact` (`DeleteDialog.tsx:32`) |
| A9 | **Link ke route yang tidak ada** — `to="/blueprint/ui"` padahal route hanya `/blueprint`; jatuh ke `*` → redirect `/` | `Export.tsx:146` vs `App.tsx:79,82`, `blueprint.ts:4` |
| A10 | Copy hardcoded di JSX, tidak lewat modul teks (kontras `tell.ts` yang terpusat) | `Tell.tsx:11-16`, `Connect.tsx:11-18` |
| A11 | Waktu muat per halaman tidak seragam — Blueprint/Export fetch sekali saat `project.version` berubah; Workflow poll 15 dtk; store sudah punya SSE | `Blueprint.tsx:23-27`, `Export.tsx:14-17` vs `Workflow.tsx:25`, `store/project.ts:48-49` |

## 5. Kelemahan — Kriteria B: UX Modern

| # | Kelemahan | Bukti |
|---|---|---|
| B1 | **Tidak ada skeleton/placeholder** — semua loading = teks + spinner; nol `Suspense`/lazy | `App.tsx:64`, `Blueprint.tsx:103`, `Workflow.tsx:55`; grep 0 |
| B2 | **Error fetch ditelan → loading abadi / state salah** — `.catch(() => setView(null))` → "Memuat blueprint…" tanpa tombol coba lagi; Workflow menampilkan *"Susun workflow dari blueprint"* seolah data tidak ada (risiko generate ulang); tombol Finalisasi `disabled` tanpa penjelasan; TechStack/StructurePanel gagal senyap | `Blueprint.tsx:25-26,103`, `Workflow.tsx:23,46`, `Export.tsx:16,72`, `TechStack.tsx:37`, `StructurePanel.tsx:28` |
| B3 | **Blank screen** — `return null` tanpa fallback | `Blueprint.tsx:29`, `Workflow.tsx:29`, `Export.tsx:19`, `Tell.tsx:23` |
| B4 | **Tidak ada ErrorBoundary** → crash React = halaman kosong total | grep 0 |
| B5 | Toast error bukan elemen aksesibel: `div` onClick, tidak fokusable, tanpa `role="alert"`, tanpa auto-dismiss | `Workbench.tsx:21-25` |
| B6 | Dialog tanpa Escape & focus trap (hanya backdrop `mousedown`); banding `Menu.tsx:20-22` yang benar | `DeleteDialog.tsx:28-29`, `ProjectDialog.tsx:52-53` |
| B7 | Enter tidak submit di banyak input (keputusan, alasan skip, path) — hanya 3 `<form>` di seluruh app | `Blueprint.tsx:452,512` vs `ProjectDialog.tsx:55`, `Connect.tsx:151,192` |
| B8 | Tanpa search/filter/pagination/infinite scroll/virtualisasi — list artefak & command render penuh; recent project dipotong `slice(0,6)` tanpa "lihat semua" | `Blueprint.tsx:66-77`, `Workflow.tsx:85-107`, `Workbench.tsx:77` |
| B9 | Tidak ada dark mode toggle — hanya ikut sistem | `styles.css:36-62` |
| B10 | Tanpa optimistic update/transisi — semua aksi tunggu round-trip lalu refetch penuh | `store/project.ts:73-85` |
| B11 | Clipboard failure ditelan diam tanpa fallback (Export sudah benar) | `Workflow.tsx:218` vs `Export.tsx:117-122` |
| B12 | ProjectDialog: list folder kosong saat loading tanpa spinner | `ProjectDialog.tsx:33-39,60-68` |
| B13 | Tidak ada undo untuk aksi non-dialog (mis. "Hapus semua" recent) | `Start.tsx:28-30` |

## 6. Kelemahan — Kriteria C: Aksesibilitas

| # | Kelemahan | Bukti |
|---|---|---|
| C1 | Dialog utama tanpa `role="dialog"`; tidak ada `aria-modal`/focus trap/autofocus | `ProjectDialog.tsx:53`, `DeleteDialog.tsx:29` |
| C2 | Status hanya warna + tooltip `title` — `title` tidak terbaca keyboard/SR | `Blueprint.tsx:382-385`, `Workflow.tsx:101-102` |
| C3 | Progres job tidak diumumkan — `JobBanner` tanpa `aria-live`/`role="status"` (satu-satunya `aria-live` di `ModelPicker.tsx:53`) | `JobBanner.tsx:23-37` |
| C4 | `role="menu"` tapi item bukan `role="menuitem"`, tanpa navigasi panah | `Menu.tsx:37` |
| C5 | Input tanpa label: path browser, textarea story, textarea prompt (placeholder saja) | `ProjectDialog.tsx:57`, `Tell.tsx:58-63`, `PromptBar.tsx:31` |
| C6 | Error list ProjectDialog tanpa `role="alert"` | `ProjectDialog.tsx:69` |
| C7 | Semua `input` tanpa `type` dan tanpa `autocomplete` kecuali ModelPicker | `ModelPicker.tsx:32` |
| C8 | Tidak ada skip-link; landmark hanya `header/nav/main` | `Workbench.tsx:36,43,19` |

## 7. Yang Sudah Benar (Pertahankan)

- Token terpusat + light/dark otomatis (`styles.css:1-62`).
- `:focus-visible` global (`styles.css:71`).
- Combobox ARIA lengkap (`ModelPicker.tsx:30-47`).
- `aria-current="step"` (`Workbench.tsx:48`), `aria-pressed` chips (`Tell.tsx:81,109`).
- `prefers-reduced-motion` (4 tempat).
- `rel="noreferrer noopener"` (`Markdown.tsx:120`), `lang="id"` (`index.html:2`).
- Markdown renderer tanpa raw HTML — XSS-safe (`Markdown.tsx:5`).
- Konfirmasi hapus dengan dampak dari server (`DeleteDialog.tsx:15-16`).

## 8. Prioritas Perbaikan (rekomendasi)

1. **B2 + B4** — error loading abadi & tanpa error boundary: bug fungsional, bukan selera.
2. **A1** — render `DesignTokens`/`DesignRules` sebagai UI (tabel token + preview): inti kriteria "UI berbasis data".
3. **A2/A3/A4/A5/A6** — ekstrak komponen & konstanta generik, hilangkan duplikasi.
4. **C1/C2/C3** — aksesibilitas dialog, status non-warna, live region.
