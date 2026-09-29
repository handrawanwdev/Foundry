import { useState } from "react";
import { Link } from "react-router-dom";
import { api } from "../api";
import { componentName, featuresWithoutPage, flowPages, incoming, navTarget, requirementsOf } from "../design";
import { isWorking, PLATFORM_LABEL } from "../flow";
import { useProject } from "../store/project";
import type { Page, Project } from "../types";

// ---------- Inspect ----------

interface InspectProps {
  project: Project;
  page: Page;
  selected: number | null;
  onSelect: (i: number | null) => void;
  onOpen: (pageId: string) => void;
}

export function Inspect({ project, page, selected, onSelect, onOpen }: InspectProps) {
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
        </details>
      </div>
    );
  }

  const feature = project.features?.find((f) => f.id === page.feature_id);
  const reqs = (feature?.requirement_ids ?? []).map((id) => project.requirements?.find((r) => r.id === id)).filter((r) => !!r);
  const flows = (project.flows ?? []).filter((f) => f.page_ids?.includes(page.id));
  const from = incoming(page, pages);
  const tasks = (project.tasks ?? []).filter((t) => t.page_ids?.includes(page.id));
  return (
    <div className="inspect">
      <h2>{page.name}</h2>
      {page.purpose && <p className="muted">{page.purpose}</p>}
      <dl>
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
      <details className="tech"><summary>Detail teknis</summary>
        <div>halaman: {page.id}</div><div>fitur: {page.feature_id}</div><div>platform: {page.platform}</div>
      </details>
    </div>
  );
}

// ---------- Change preview ----------

const SYMBOL = { added: "+", changed: "~", removed: "−" } as const;
const KIND_LABEL = { added: "halaman baru", changed: "diubah", removed: "dihapus" } as const;

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
          <div className="small muted">“{rev.request}”</div>
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
              <span className="muted small"> {KIND_LABEL[c.kind]}</span>
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

export function AskChange({ page, onClose }: { page?: Page; onClose: () => void }) {
  const { run, project } = useProject();
  const [text, setText] = useState("");
  const [scoped, setScoped] = useState(!!page);
  const working = isWorking(project);
  const send = async () => {
    const t = text.trim();
    if (!t || working) return;
    const feedback = scoped && page ? `Di halaman "${page.name}": ${t}` : t;
    if (await run("revise", () => api.reviseDesign(feedback))) onClose();
  };
  return (
    <div className="dw-ask" onKeyDown={(e) => e.key === "Escape" && onClose()}>
      <textarea rows={3} autoFocus value={text} onChange={(e) => setText(e.target.value)}
        onKeyDown={(e) => { if (e.key === "Enter" && (e.ctrlKey || e.metaKey)) void send(); }}
        placeholder="mis. Tambahkan filter dokter di halaman Janji Temu" />
      <div className="dw-ask-foot">
        {page ? (
          <label className="small muted"><input type="checkbox" checked={scoped} onChange={(e) => setScoped(e.target.checked)} /> Tentang halaman {page.name}</label>
        ) : <span />}
        <div className="inline">
          <button className="ghost" onClick={onClose}>Batal</button>
          <button className="primary" disabled={!text.trim() || working} onClick={() => void send()}>Kirim</button>
        </div>
      </div>
      <div className="small muted">Perubahan hanya ke rancangan, belum ke kode. Kamu bisa menyimpan atau membatalkannya.</div>
    </div>
  );
}

// ---------- Review ----------

interface ReviewProps {
  project: Project;
  onAsk: () => void;
  onTry: (flowId: string) => void;
  onInfo: (v: "platform") => void;
}

export function Review({ project, onAsk, onTry, onInfo }: ReviewProps) {
  const { run, pending } = useProject();
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
        {d.revision && !approved && <li className="warn">! Perubahan terakhir belum disimpan. Menyetujui akan menyimpannya.</li>}
      </ul>

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
            <button className="primary" disabled={working || pending === "approve"} onClick={() => run("approve", api.approveDesign)}>Setujui Rancangan</button>
            <button disabled={working} onClick={onAsk}>Minta perubahan</button>
          </>
        )}
      </div>
    </div>
  );
}
