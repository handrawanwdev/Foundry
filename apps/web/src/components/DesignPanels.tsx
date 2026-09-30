import { useState, type KeyboardEvent } from "react";
import { Link } from "react-router-dom";
import { api } from "../api";
import { changeRequest, componentName, DENSITY_LABEL, designChecks, featuresWithoutPage, fixRequest, flowPages, incoming, KIND_LABEL, navTarget, pageDensity, pageKind, requirementsOf, type PageRequest } from "../design";
import { isWorking, PLATFORM_LABEL } from "../flow";
import { useProject } from "../store/project";
import type { Blocker, Page, PageHandoff, Project } from "../types";
import type { Device } from "../design";

// ---------- Inspect ----------

interface InspectProps {
  project: Project;
  page: Page;
  handoff?: PageHandoff;
  selected: number | null;
  onSelect: (i: number | null) => void;
  onOpen: (pageId: string) => void;
}

const List = ({ title, items }: { title: string; items?: string[] | null }) =>
  items?.length ? <><h5>{title}</h5><ul>{items.map((x) => <li key={x}>{x}</li>)}</ul></> : null;

export function Inspect({ project, page, handoff, selected, onSelect, onOpen }: InspectProps) {
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
  const tasks = (project.tasks ?? []).filter((t) => t.page_ids?.includes(page.id));
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
        {tasks.length > 0 && <><dt>Task</dt><dd>{tasks.map((t) => <div key={t.id}>{t.title} <span className="muted small">{t.status}</span></div>)}</dd></>}
      </dl>
      {(page.components ?? []).length > 0 && (
        <section>
          <h4>Isi halaman</h4>
          {(page.components ?? []).map((x, i) => (
            <button key={i} className="inspect-item" onClick={() => onSelect(i)}>{componentName(x)}</button>
          ))}
        </section>
      )}
      {handoff && (
        <details className="tech handoff"><summary>Handoff ke Build</summary>
          <List title="Interaksi" items={handoff.interactions} />
          <List title="Validasi" items={handoff.validation} />
          <List title="Tes" items={handoff.tests} />
          <List title="Kebutuhan" items={handoff.requirement_ids} />
        </details>
      )}
      <details className="tech"><summary>Detail teknis</summary>
        <div>halaman: {page.id}</div><div>fitur: {page.feature_id}</div><div>platform: {page.platform}</div><div>kind: {page.kind ?? "—"}</div>
      </details>
    </div>
  );
}

const STATE_LABEL: ["loading" | "empty" | "no_results" | "error" | "success", string][] = [
  ["loading", "Memuat"], ["empty", "Kosong"], ["no_results", "Tanpa hasil"], ["error", "Error"], ["success", "Berhasil"],
];

// ---------- Change preview ----------

const SYMBOL = { added: "+", changed: "~", removed: "−" } as const;
const CHANGE_LABEL = { added: "halaman baru", changed: "diubah", removed: "dihapus" } as const;

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
        </ul>
      )}
    </div>
  );
}

// ---------- Ask for change ----------

// What the user sees on each page; loaded on demand (it pulls in the React
// server renderer).
async function screensOf(pages: Page[], app = "Aplikasi", devices?: Record<string, Device>) {
  return (await import("../screen")).screens(pages, app, devices);
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
  const send = async () => {
    if (!feedback || working) return;
    if (await run("revise", async () => api.reviseDesign(feedback, await screensOf(pages, project?.name, page && device ? { [page.id]: device } : undefined)))) onClose();
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
            placeholder="mis. Tambahkan filter dokter" />
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
      <div className="small muted">Semua perubahan diproses sekali jalan, lalu bisa disimpan atau dibatalkan bersama. Perubahan hanya ke rancangan, belum ke kode.</div>
    </div>
  );
}

// ---------- Review ----------

interface ReviewProps {
  project: Project;
  blockers: Blocker[];
  onAsk: () => void;
  onTry: (flowId: string) => void;
  onInfo: (v: "platform") => void;
  onOpen: (pageId: string) => void;
}

export function Review({ project, blockers, onAsk, onTry, onInfo, onOpen }: ReviewProps) {
  const { run, pending } = useProject();
  const findings = designChecks(project);
  const d = project.design!;
  const pages = d.pages;
  const flows = (project.flows ?? []).map((f) => ({ f, ps: flowPages(f, pages) })).filter((x) => x.ps.length > 0);
  const components = pages.reduce((n, p) => n + (p.components?.length ?? 0), 0);
  const uncovered = featuresWithoutPage(project);
  const s = project.structure;
  const working = isWorking(project);
  const approved = d.status === "approved";

  return (
    <div className="review">
      <h1>Tinjau rancangan</h1>
      <p className="muted">Setelah disetujui, Foundry membuat task dari rancangan ini.</p>
      <ul className="review-list">
        <li className="ok">✓ {pages.length} halaman</li>
        <li className="ok">✓ {flows.length} alur pengguna</li>
        <li className="ok">✓ {components} komponen</li>
        {uncovered.length ? <li className="warn">! Fitur tanpa halaman: {uncovered.map((f) => f.name).join(", ")}</li>
          : <li className="ok">✓ Semua fitur punya halaman</li>}
        <li className="ok">✓ Platform: {(project.platform?.targets ?? []).map((t) => PLATFORM_LABEL[t.kind]).join(" + ") || "—"}
          {" · "}Struktur: {s?.mode === "existing" ? "project yang sudah ada" : s?.template ?? "—"}
          {!approved && <> <button className="link" onClick={() => onInfo("platform")}>Ubah</button></>}
        </li>
        {blockers.length ? <li className="warn">! Wajib dilengkapi: {blockers.length} hal (route, akses, aksi utama, kolom, menu)</li>
          : <li className="ok">✓ Tiap halaman punya route, akses dan aksi utama</li>}
        {findings.length ? <li className="warn">! Kualitas rancangan: {findings.length} temuan</li>
          : <li className="ok">✓ Kualitas rancangan: tiap halaman punya layout sesuai jenisnya</li>}
        {d.revision && !approved && <li className="warn">! Perubahan terakhir belum disimpan. Menyetujui akan menyimpannya.</li>}
      </ul>

      {!approved && blockers.length > 0 && (
        <section className="review-flows">
          <h3>Wajib dilengkapi sebelum disetujui</h3>
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
          onClick={() => run("revise", async () => api.reviseDesign(fixRequest(blockers, findings), await screensOf(d.pages, project.name)))}>✦ Perbaiki otomatis</button>
      )}

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
            <span className="ok-text">✓ Rancangan disetujui</span>
            {(project.tasks?.length ?? 0) > 0 ? <Link className="button primary" to="/simple/bangun">Lihat Kanban</Link>
              : working ? <span><span className="spinner" /> Membuat task…</span> : null}
          </>
        ) : (
          <>
            <button className="primary" disabled={working || pending === "approve" || blockers.length > 0}
              title={blockers.length ? "Lengkapi dulu hal wajib di atas" : undefined} onClick={() => run("approve", api.approveDesign)}>Setujui Rancangan</button>
            <button disabled={working} onClick={onAsk}>Minta perubahan</button>
          </>
        )}
      </div>
    </div>
  );
}
