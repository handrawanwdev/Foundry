import { useState, type KeyboardEvent } from "react";
import { useNavigate } from "react-router-dom";
import { api } from "../api";
import { changeRequest, changeScope, componentName, DENSITY_LABEL, designChecks, featuresWithoutPage, fixRequest, flowPages, incoming, KIND_LABEL, navTarget, pageDensity, pageKind, requirementsOf, type PageRequest } from "../design";
import { isWorking, PLATFORM_LABEL } from "../flow";
import { useProject } from "../store/project";
import { STRUCTURE_TYPE, structureGate, structureVersion } from "../structure";
import { changeText, TECH_STATUS, techGate, techReview, techState, techVersion } from "../tech";
import type { Blocker, DesignQA, DesignQuality, DesignSystem, Page, PageSpec, Project, QAIssue, ThemeContext, Token } from "../types";
import type { Device } from "../design";
import { screens } from "../screen";

// ---------- Inspect ----------

interface InspectProps {
  project: Project;
  page: Page;
  spec?: PageSpec;
  selected: number | null;
  onSelect: (i: number | null) => void;
  onOpen: (pageId: string) => void;
}

const List = ({ title, items }: { title: string; items?: string[] | null }) =>
  items?.length ? <><h5>{title}</h5><ul>{items.map((x) => <li key={x}>{x}</li>)}</ul></> : null;

export function Inspect({ project, page, spec, selected, onSelect, onOpen }: InspectProps) {
  const pages = project.design?.pages ?? [];
  const c = selected !== null ? page.components?.[selected] : undefined;
  const PageLink = ({ p }: { p: Page }) => <button className="link" onClick={() => onOpen(p.id)}>→ {p.name}</button>;

  if (c) {
    const target = pages.find((p) => p.id === c.target);
    const navLinks = c.type === "nav" ? (c.items ?? []).map((it) => navTarget(it, page, pages)).filter((p): p is Page => !!p) : [];
    return (
      <div className="inspect">
        <button className="link small" onClick={() => onSelect(null)}>← {page.name}</button>
        <h2>{componentName(c)}</h2>
        {c.id && page.plan?.reasons?.[c.id] && <p className="muted small">Alasan: {page.plan.reasons[c.id]}</p>}
        {!!c.items?.length && (
          <section><h4>Isi</h4><ul className="plain">{c.items.map((it) => <li key={it}>{it}</li>)}</ul></section>
        )}
        {(target || navLinks.length > 0) && (
          <section><h4>Aksi</h4>{target ? <PageLink p={target} /> : navLinks.map((p) => <div key={p.id}><PageLink p={p} /></div>)}</section>
        )}
        {!target && navLinks.length === 0 && <p className="muted small">Elemen ini tidak membuka halaman lain.</p>}
        <details className="tech"><summary>Detail teknis</summary>
          <div>tipe: {c.type}</div>{c.target && <div>target: {c.target}</div>}
        {!!c.columns?.length && <div>kolom: {c.columns.join(", ")}</div>}
          {(c.fields ?? []).map((f) => (
            <div key={f.label}>{f.label}: {f.type}{f.required ? ", wajib" : ""}{f.validation ? `, ${f.validation}` : ""}{f.error ? ` — “${f.error}”` : ""}</div>
          ))}
        </details>
      </div>
    );
  }

  const feature = project.features?.find((f) => f.id === page.feature_id);
  const reqs = (feature?.requirement_ids ?? []).map((id) => project.requirements?.find((r) => r.id === id)).filter((r) => !!r);
  const flows = (project.flows ?? []).filter((f) => f.page_ids?.includes(page.id));
  const from = incoming(page, pages);
  const action = page.primary_action;
  const actionTarget = pages.find((p) => p.id === action?.target);
  const data = page.data;
  return (
    <div className="inspect">
      <h2>{page.name}</h2>
      {page.purpose && <p className="muted">{page.purpose}</p>}
      <dl>
        <dt>Route</dt><dd>{page.route ? <code>{page.route}</code> : <span className="warn-text">belum ditentukan</span>}</dd>
        <dt>Akses</dt><dd>{page.access === "auth" ? "Harus login" : page.access === "public" ? "Publik" : <span className="warn-text">belum ditentukan</span>}</dd>
        <dt>Aksi utama</dt><dd>{action?.label ? <>{action.label}{actionTarget && <> <PageLink p={actionTarget} /></>}</>
          : action?.reason ? <span className="muted">Tidak ada — {action.reason}</span> : <span className="warn-text">belum ditentukan</span>}</dd>
        {!!page.secondary_actions?.length && <><dt>Aksi lain</dt><dd>{page.secondary_actions.join(", ")}</dd></>}
        <dt>Urutan informasi</dt><dd>{page.hierarchy?.length
          ? <ol className="plain-ol">{page.hierarchy.map((h) => <li key={h}>{h}</li>)}</ol> : <span className="warn-text">belum ditentukan</span>}</dd>
        {page.responsive && <><dt>Responsif</dt><dd>{page.responsive}</dd></>}
        {(page.intent || page.plan) && <><dt>Keputusan desain</dt><dd><PlanView page={page} /></dd></>}
        <dt>Jenis</dt><dd>{KIND_LABEL[pageKind(page)]}{!page.kind && <span className="muted small"> (ditebak)</span>} · kepadatan {DENSITY_LABEL[pageDensity(page)]}</dd>
        {page.states && <><dt>Kondisi</dt><dd>
          {STATE_LABEL.filter(([k]) => page.states?.[k]).map(([k, l]) => <div key={k}><span className="muted small">{l}:</span> {page.states![k]}</div>)}
        </dd></>}
        {data && <><dt>Data</dt><dd>
          {!!data.required?.length && <div><span className="muted small">Wajib:</span> {data.required.join(", ")}</div>}
          {!!data.optional?.length && <div><span className="muted small">Opsional:</span> {data.optional.join(", ")}</div>}
          {!!data.dynamic?.length && <div><span className="muted small">Dinamis:</span> {data.dynamic.join(", ")}</div>}
          {!!data.api?.length && <div><span className="muted small">API:</span> {data.api.map((a) => <code key={a} className="small"> {a}</code>)}</div>}
        </dd></>}
        {page.role && <><dt>Peran</dt><dd>{page.role}</dd></>}
        {feature && <><dt>Fitur</dt><dd>{feature.name}</dd></>}
        {reqs.length > 0 && <><dt>Kebutuhan</dt><dd>{reqs.map((r) => <div key={r!.id}>{r!.text}</div>)}</dd></>}
        {flows.length > 0 && <><dt>Alur</dt><dd>{flows.map((f) => <div key={f.id}>{f.name}</div>)}</dd></>}
        {from.length > 0 && <><dt>Dibuka dari</dt><dd>{from.map((p) => <div key={p.id}><PageLink p={p} /></div>)}</dd></>}
      </dl>
      {(page.components ?? []).length > 0 && (
        <section>
          <h4>Isi halaman</h4>
          {(page.components ?? []).map((x, i) => (
            <button key={i} className="inspect-item" onClick={() => onSelect(i)}>{componentName(x)}</button>
          ))}
        </section>
      )}
      {spec && (
        <details className="tech handoff"><summary>Spesifikasi implementasi</summary>
          <List title="Interaksi" items={spec.interactions} />
          <List title="Validasi" items={spec.validation} />
          <List title="Tes" items={spec.tests} />
          <List title="Kebutuhan" items={spec.requirement_ids} />
        </details>
      )}
      <details className="tech"><summary>Detail teknis</summary>
        <div>halaman: {page.id}</div><div>fitur: {page.feature_id}</div><div>platform: {page.platform}</div><div>kind: {page.kind ?? "—"}</div>
      </details>
    </div>
  );
}

// Intent and plan: why the layout looks the way it does.
function PlanView({ page }: { page: Page }) {
  const { intent: it, plan: pl } = page;
  const ids = (xs?: string[]) => (xs ?? []).map((id) => "#" + id).join(", ");
  return (
    <>
      {it && <div className="small">
        {[it.primary_task && `tugas ${it.primary_task}`, it.frequency && `frekuensi ${it.frequency}`, it.complexity && `kompleksitas ${it.complexity}`,
          it.information_density && `kepadatan info ${it.information_density}`, it.comparison && "membandingkan banyak data"].filter(Boolean).join(" · ")}
      </div>}
      {pl?.layout_strategy && <div className="small"><span className="muted">Layout:</span> {pl.layout_strategy}{pl.responsive_strategy && ` · mobile: ${pl.responsive_strategy}`}</div>}
      {pl?.primary_focus && <div className="small"><span className="muted">Fokus utama:</span> #{pl.primary_focus}</div>}
      {!!pl?.secondary_content?.length && <div className="small"><span className="muted">Sekunder:</span> {ids(pl.secondary_content)}</div>}
      {!!pl?.supporting_content?.length && <div className="small"><span className="muted">Pendukung:</span> {ids(pl.supporting_content)}</div>}
    </>
  );
}

const STATE_LABEL: ["loading" | "empty" | "no_results" | "error" | "success", string][] = [
  ["loading", "Memuat"], ["empty", "Kosong"], ["no_results", "Tanpa hasil"], ["error", "Error"], ["success", "Berhasil"],
];

// ---------- Change preview ----------

const SYMBOL = { added: "+", changed: "~", removed: "−" } as const;
const CHANGE_LABEL = { added: "halaman baru", changed: "diubah", removed: "dihapus" } as const;

function qaText(qa: DesignQA): string {
  const where = qa.shots?.length ? ` (${[...new Set(qa.shots.map((s) => s.device))].join(", ")})` : "";
  switch (qa.status) {
    case "pass": return `✓ Lolos pemeriksaan visual di browser${where}`;
    case "fixed": return `✓ Lolos pemeriksaan visual${where} setelah ${qa.round - 1} patch perbaikan${qa.notes ? `: ${qa.notes}` : ""}`;
    case "failed": return `! Pemeriksaan visual belum lolos${where}: ${qa.notes ?? ""}`;
    case "skipped": return `Pemeriksaan visual dilewati: ${qa.notes ?? ""}`;
  }
  return "Memeriksa hasil visual…";
}

const SEVERITY_LABEL = { critical: "kritis", major: "besar", minor: "kecil" } as const;

// Problems the visual critic saw on the screenshots; minor ones do not
// block, they are kept for the next change.
function QAIssues({ issues, pages, onOpen }: { issues: QAIssue[]; pages: Page[]; onOpen: (pageId: string) => void }) {
  return (
    <details className="small qa-issues"><summary>{issues.length} temuan desain</summary>
      <ul>
        {issues.map((it, i) => {
          const pg = pages.find((p) => p.id === it.page_id);
          return (
            <li key={i}>
              <span className={`sev ${it.severity}`}>{SEVERITY_LABEL[it.severity]}</span>{" "}
              {(it.category || it.component || it.device || it.source) && <span className="muted">[{[it.source === "lint" && "aturan", it.category, it.component && "#" + it.component, it.device].filter(Boolean).join(" · ")}] </span>}
              {pg && <><button className="link" onClick={() => onOpen(pg.id)}>{pg.name}</button>: </>}
              {it.problem}
              {(it.why || it.impact || it.fix) && (
                <div className="muted">
                  {it.why && <>Mengapa: {it.why}. </>}{it.impact && <>Dampak: {it.impact}. </>}{it.fix && <>Perbaikan: {it.fix}</>}
                </div>
              )}
            </li>
          );
        })}
      </ul>
    </details>
  );
}

const DIMENSION_LABEL: Record<string, string> = {
  hierarchy: "Hierarki", composition: "Komposisi", spacing: "Spasi", density: "Kepadatan",
  responsive: "Responsif", navigation: "Navigasi", ai_slop: "Anti AI-slop", theme: "Konsistensi theme", reference: "Konsistensi referensi",
};
const CONFIDENCE_LABEL = { low: "rendah", medium: "sedang", high: "tinggi" };
const VERDICT_LABEL = { improved: "membaik", neutral: "tetap", degraded: "memburuk" };

// Quality report: why the design passes or needs a revision. The score is an
// indicator of the decisions, not a ranking.
function QualityPanel({ q, pages, theme, onOpen }: { q: DesignQuality; pages: Page[]; theme?: ThemeContext; onOpen: (pageId: string) => void }) {
  const c = q.compared_to_previous;
  const blocked = q.decision === "revision_required";
  return (
    <section className="review-flows quality">
      <h3>Kualitas desain</h3>
      <div className="quality-head">
        <strong>{q.overall}/100</strong>
        <span className={blocked ? "warn-text" : "ok-text"}>{blocked ? "Perlu revisi" : "Lolos gate"}</span>
        <span className="muted small">keyakinan {CONFIDENCE_LABEL[q.confidence]}</span>
      </div>
      <ul className="quality-bars">
        {Object.entries(DIMENSION_LABEL).map(([k, label]) => (
          <li key={k}><span>{label}</span><span className="quality-bar"><span style={{ width: `${q.dimensions[k] ?? 0}%` }} /></span><span className="muted">{q.dimensions[k] ?? 0}</span></li>
        ))}
      </ul>
      {c && <p className="small muted">Patch kritik ronde {c.round}: {c.before.overall} → {c.after.overall}, blocker {c.before.blockers} → {c.after.blockers} ({VERDICT_LABEL[c.verdict]}{c.rejected ? ", patch ditolak" : ""})</p>}
      <ul className="review-findings small">
        {(q.dimensions.ai_slop ?? 0) === 100 && <li className="ok-text">✓ Tidak ada AI slop</li>}
        {(q.dimensions.responsive ?? 0) === 100 && <li className="ok-text">✓ Strategi mobile valid</li>}
        {theme && (q.dimensions.theme ?? 0) === 100 && <li className="ok-text">✓ Konsisten dengan theme {theme.name}</li>}
        {q.reasons?.map((r, i) => <li key={i} className="warn-text">! {r.problem}{r.fix && <span className="muted"> Perbaikan: {r.fix}</span>}</li>)}
      </ul>
      {q.recommendation && <p className="small muted">{q.recommendation}</p>}
      {!!q.findings?.length && <QAIssues issues={q.findings} pages={pages} onOpen={onOpen} />}
    </section>
  );
}

export function ChangeBar({ project, onOpen }: { project: Project; onOpen: (pageId: string) => void }) {
  const { run, pending } = useProject();
  const [open, setOpen] = useState(true);
  const rev = project.design?.revision;
  if (!rev || isWorking(project)) return null;
  const changes = rev.changes ?? [];
  const touched = changes.filter((c) => c.page_id).map((c) => c.page_id!);
  const reqs = requirementsOf(touched, project);
  return (
    <div className="dw-change">
      <div className="dw-change-head">
        <div>
          <strong>Rancangan diperbarui</strong>
          <span className="muted"> · {changes.length ? `${changes.length} perubahan` : "tidak ada perubahan halaman"}</span>
          <div className="small muted" style={{ whiteSpace: "pre-line" }}>“{rev.request}”</div>
          {rev.qa && <div className={`small ${rev.qa.status === "failed" ? "warn-text" : "muted"}`} style={{ whiteSpace: "pre-line" }}>{qaText(rev.qa)}</div>}
          {!!rev.qa?.issues?.length && <QAIssues issues={rev.qa.issues} pages={project.design?.pages ?? []} onOpen={onOpen} />}
        </div>
        <div className="inline">
          {changes.length > 0 && <button className="ghost" onClick={() => setOpen(!open)}>{open ? "Sembunyikan" : "Lihat"}</button>}
          <button disabled={!!pending} onClick={() => run("undo", api.undoDesign)}>Batalkan</button>
          <button className="primary" disabled={!!pending} onClick={() => run("keep", api.keepDesign)}>Simpan</button>
        </div>
      </div>
      {open && changes.length > 0 && (
        <ul className="dw-change-list">
          {changes.map((c, i) => (
            <li key={i}>
              <span className={`sym ${c.kind}`}>{SYMBOL[c.kind]}</span>
              {c.page_id ? <button className="link" onClick={() => onOpen(c.page_id!)}>{c.page}</button> : <span>{c.page}</span>}
              <span className="muted small"> {CHANGE_LABEL[c.kind]}</span>
              {!!c.details?.length && <div className="small muted">{c.details.join(" · ")}</div>}
            </li>
          ))}
          {reqs.length > 0 && <li className="small muted">Kebutuhan terkait: {reqs.join(", ")}</li>}
          {!!rev.patch?.length && <li className="small muted">Operasi: {rev.patch.join(" · ")}</li>}
        </ul>
      )}
    </div>
  );
}

// ---------- Ask for change ----------

function screensOf(pages: Page[], app = "Aplikasi", devices?: Record<string, Device>) {
  return screens(pages, app, devices);
}

interface AskProps {
  page?: Page;
  pages: Page[];
  component?: number | null; // picked in the inspector
  device?: Device; // the page is shown on this device
  onClose: () => void;
}

export function AskChange({ page, pages, component, device, onClose }: AskProps) {
  const { run, project } = useProject();
  const [general, setGeneral] = useState("");
  const [items, setItems] = useState<PageRequest[]>(page
    ? [{ page, text: "", component: component != null && page.components?.[component] ? component : undefined }] : []);
  const working = isWorking(project);
  const feedback = changeRequest(general, items);
  const free = pages.filter((p) => !items.some((r) => r.page.id === p.id));
  const multi = new Set(pages.map((p) => p.platform)).size > 1;
  const label = (p: Page) => multi ? `${p.name} · ${PLATFORM_LABEL[p.platform] ?? p.platform}` : p.name;
  const setText = (id: string, text: string) => setItems(items.map((r) => r.page.id === id ? { ...r, text } : r));
  const add = (id: string) => { const p = pages.find((x) => x.id === id); if (p) setItems([...items, { page: p, text: "" }]); };
  const devices = page && device ? { [page.id]: device } : undefined;
  const send = async () => {
    if (!feedback || working) return;
    const scope = changeScope(general, items, devices);
    if (await run("revise", async () => api.reviseDesign(feedback, await screensOf(pages, project?.name, devices), scope))) onClose();
  };
  const keys = (e: KeyboardEvent) => { if (e.key === "Enter" && (e.ctrlKey || e.metaKey)) void send(); };
  return (
    <div className="dw-ask" onKeyDown={(e) => e.key === "Escape" && onClose()}>
      {items.map((r, i) => (
        <div key={r.page.id} className="dw-ask-page">
          <div className="dw-ask-page-head">
            <span className="small">{label(r.page)}</span>
            {r.component !== undefined && r.page.components?.[r.component] && (
              <span className="dw-ask-comp small" title="Komponen yang dipilih di panel inspect">
                [{r.component}] {componentName(r.page.components[r.component])}
                <button className="ghost small" title="Jangan tunjuk komponen" onClick={() => setItems(items.map((x) => x.page.id === r.page.id ? { ...x, component: undefined } : x))}>×</button>
              </span>
            )}
            <button className="ghost small" title="Hapus halaman ini" onClick={() => setItems(items.filter((x) => x.page.id !== r.page.id))}>×</button>
          </div>
          <textarea rows={2} autoFocus={i === 0} value={r.text} onChange={(e) => setText(r.page.id, e.target.value)} onKeyDown={keys}
            placeholder={r.component !== undefined ? "mis. Pindahkan ke bawah form" : "mis. Tambahkan filter dokter"} />
          <label className="small muted dw-ask-fields">
            <input type="checkbox" checked={!!r.fields} onChange={(e) => setItems(items.map((x) => x.page.id === r.page.id ? { ...x, fields: e.target.checked } : x))} />
            Boleh ubah properti halaman (jenis, route, data, kondisi)
          </label>
        </div>
      ))}
      {free.length > 0 && (
        <select value="" onChange={(e) => add(e.target.value)}>
          <option value="">+ Tambah halaman…</option>
          {free.map((p) => <option key={p.id} value={p.id}>{label(p)}</option>)}
        </select>
      )}
      <textarea rows={items.length ? 2 : 3} autoFocus={!items.length} value={general} onChange={(e) => setGeneral(e.target.value)} onKeyDown={keys}
        placeholder={items.length ? "Perubahan umum (opsional)" : "mis. Tambahkan filter dokter di halaman Janji Temu"} />
      <div className="dw-ask-foot">
        <span className="small muted">{items.filter((r) => r.text.trim()).length > 1 ? `${items.filter((r) => r.text.trim()).length} halaman sekaligus` : ""}</span>
        <div className="inline">
          <button className="ghost" onClick={onClose}>Batal</button>
          <button className="primary" disabled={!feedback || working} onClick={() => void send()}>Kirim</button>
        </div>
      </div>
      <div className="small muted">AI hanya boleh mengubah halaman{items.some((r) => r.component !== undefined) ? " dan komponen" : ""} yang ditunjuk{general.trim() ? " (perubahan umum membuka semua halaman)" : ""}; patch di luar itu ditolak. Hasilnya diperiksa di browser, lalu bisa disimpan atau dibatalkan bersama.</div>
    </div>
  );
}

// ---------- Review ----------

interface ReviewProps {
  project: Project;
  blockers: Blocker[];
  onTry: (flowId: string) => void;
  onInfo: (v: "platform") => void;
  onOpen: (pageId: string) => void;
}

export function Review({ project, blockers, onTry, onInfo, onOpen }: ReviewProps) {
  const { run, pending } = useProject();
  const nav = useNavigate();
  const findings = designChecks(project);
  const d = project.design!;
  const pages = d.pages ?? [];
  const flows = (project.flows ?? []).map((f) => ({ f, ps: flowPages(f, pages) })).filter((x) => x.ps.length > 0);
  const components = pages.reduce((n, p) => n + (p.components?.length ?? 0), 0);
  const uncovered = featuresWithoutPage(project);
  const s = project.structure;
  const working = isWorking(project);
  const approved = d.status === "approved";
  const tech = techReview(project);
  const gate = approved ? null : techGate(project) ?? structureGate(project);

  return (
    <div className="review">
      <h1>Tinjau rancangan</h1>
      <p className="muted">Setelah difinalisasi, Foundry menulis spesifikasi final ke folder <code>docs/</code> project untuk diimplementasikan dengan tool lain.</p>
      <ul className="review-list">
        <li className="ok">✓ {pages.length} halaman</li>
        <li className="ok">✓ {flows.length} alur pengguna</li>
        <li className="ok">✓ {components} komponen</li>
        {uncovered.length ? <li className="warn">! Fitur tanpa halaman: {uncovered.map((f) => f.name).join(", ")}</li>
          : <li className="ok">✓ Semua fitur punya halaman</li>}
        <li className="ok">✓ Platform: {(project.platform?.targets ?? []).map((t) => PLATFORM_LABEL[t.kind]).join(" + ") || "—"}
          {!approved && <> <button className="link" onClick={() => onInfo("platform")}>Ubah</button></>}
        </li>
        {s?.mode === "existing" ? <li className="ok">✓ Struktur: project yang sudah ada</li>
          : s?.ir && (approved || structureGate(project) === null
            ? <li className="ok">✓ Struktur {STRUCTURE_TYPE[s.ir.type]} v{structureVersion(s)} disetujui</li>
            : <li className="warn">! Struktur {STRUCTURE_TYPE[s.ir.type]}: {structureGate(project)}
              {" "}<button className="link" onClick={() => onInfo("platform")}>Tinjau</button></li>)}
        {project.stack && project.stack.mode === "new" && (techState(project.stack) === "approved"
          ? <li className="ok">✓ Teknologi v{techVersion(project.stack)} disetujui</li>
          : <li className="warn">! Teknologi: {TECH_STATUS[techState(project.stack)]} — setujui dulu
            {!approved && <> <button className="link" onClick={() => onInfo("platform")}>Tinjau</button></>}</li>)}
        {tech.length > 0 && <li className="warn">! Teknologi berubah setelah rancangan dibuat: {tech.map(changeText).join("; ")}
          {" "}<button className="link" onClick={() => onInfo("platform")}>Periksa dampak</button></li>}
        {blockers.length ? <li className="warn">! Wajib dilengkapi: {blockers.length} hal (design system, route, akses, aksi, urutan informasi, kondisi, menu, kualitas)</li>
          : <li className="ok">✓ Design system ada; tiap halaman punya route, akses, satu aksi utama, urutan informasi dan kondisi data</li>}
        {findings.length ? <li className="warn">! Kualitas rancangan: {findings.length} temuan</li>
          : <li className="ok">✓ Kualitas rancangan: tiap halaman punya layout sesuai jenisnya</li>}
        {d.revision && !approved && <li className="warn">! Perubahan terakhir belum disimpan atau dibatalkan.</li>}
        {d.qa && !d.revision && <li className={d.qa.status === "failed" ? "warn" : "ok"} style={{ whiteSpace: "pre-line" }}>{qaText(d.qa)}
          {!!d.qa.issues?.length && <QAIssues issues={d.qa.issues} pages={pages} onOpen={onOpen} />}</li>}
      </ul>

      {d.quality && <QualityPanel q={d.quality} pages={pages} theme={d.theme} onOpen={onOpen} />}

      {!approved && blockers.length > 0 && (
        <section className="review-flows">
          <h3>Wajib dilengkapi sebelum finalisasi</h3>
          <ul className="review-findings review-blockers">
            {blockers.map((b, i) => (
              <li key={i}>
                {b.page_id && <><button className="link" onClick={() => onOpen(b.page_id!)}>{b.page}</button>: </>}
                <span>{b.text}</span>
              </li>
            ))}
          </ul>
        </section>
      )}

      {findings.length > 0 && (
        <section className="review-flows">
          <h3>Temuan kualitas</h3>
          <ul className="review-findings">
            {findings.map((f, i) => (
              <li key={i}>
                {f.page && <><button className="link" onClick={() => onOpen(f.page!.id)}>{f.page.name}</button>: </>}
                <span className="muted">{f.text}</span>
              </li>
            ))}
          </ul>
        </section>
      )}

      {!approved && (blockers.length > 0 || findings.length > 0) && (
        <button className="review-fix" disabled={working || !!d.revision || pending === "revise"} title={d.revision ? "Simpan atau batalkan perubahan terakhir dulu" : undefined}
          onClick={() => run("revise", async () => api.reviseDesign(fixRequest(blockers, findings), await screensOf(pages, project.name),
            { open: true, fields: true, general: fixRequest(blockers, findings) }))}>✦ Perbaiki otomatis</button>
      )}

      {d.system && <SystemView system={d.system} />}

      {flows.length > 0 && (
        <section className="review-flows">
          <h3>Alur</h3>
          {flows.map(({ f, ps }) => (
            <div key={f.id} className="review-flow">
              <div><strong>{f.name}</strong> <span className="muted small">{f.role}</span></div>
              <div className="small muted">{ps.map((p) => p.name).join(" → ")}</div>
              <button className="link small" onClick={() => onTry(f.id)}>Coba alur →</button>
            </div>
          ))}
        </section>
      )}

      <div className="review-foot">
        {approved ? (
          <>
            <span className="ok-text">✓ Desain final — bagian dari paket ekspor</span>
            <button className="primary" onClick={() => nav("/export")}>Buka Export</button>
          </>
        ) : (
          <>
            <span className="muted small">{blockers.length > 0 || gate ? "Lengkapi hal wajib di atas sebelum finalisasi." : "Desain siap. Finalisasi dilakukan bersama command workflow."}</span>
            <button className="primary" onClick={() => nav("/workflow")}>Lanjut ke Workflow →</button>
          </>
        )}
      </div>
    </div>
  );
}

// ---------- Design system ----------

const COLOR = /^(#[0-9a-f]{3,8}|rgba?\(|hsla?\()/i;

function TokenRows({ title, tokens }: { title: string; tokens?: Token[] }) {
  if (!tokens?.length) return null;
  return (
    <>
      <h4>{title}</h4>
      <ul className="ds-tokens">
        {tokens.map((t) => (
          <li key={t.name}>
            {COLOR.test(t.value) && <span className="ds-swatch" style={{ background: t.value }} aria-hidden="true" />}
            <code>{t.name}</code> <span>{t.value}</span>{t.use && <span className="muted"> — {t.use}</span>}
          </li>
        ))}
      </ul>
    </>
  );
}

// The tokens every page follows.
function SystemView({ system }: { system: DesignSystem }) {
  return (
    <section className="review-flows ds">
      <h3>Design system</h3>
      {system.direction && <p className="muted">Arah visual: {system.direction}</p>}
      <TokenRows title="Warna" tokens={system.colors} />
      <TokenRows title="Tipografi" tokens={system.typography} />
      <TokenRows title="Spacing" tokens={system.spacing} />
      <TokenRows title="Radius" tokens={system.radius} />
      <TokenRows title="Shadow" tokens={system.shadow} />
      <TokenRows title="Breakpoint" tokens={system.breakpoints} />
      <TokenRows title="Komponen" tokens={system.components} />
    </section>
  );
}
