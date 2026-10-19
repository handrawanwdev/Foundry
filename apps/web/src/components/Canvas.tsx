import { useEffect, useState } from "react";
import { create } from "zustand";
import { api } from "../api";
import { componentName, DENSITY_LABEL } from "../design";
import { isWorking } from "../flow";
import { DENSITIES, EMPHASES, locate } from "../ir";
import { useProject } from "../store/project";
import type { CanvasOp, Density, LayoutChild, Page, Project, Prototype, VersionComparison, VersionSummary } from "../types";

// Canvas, versions and prototype map: the visual editor works on the Design
// IR tree (by component and node ids), never on the rendered DOM. Every edit
// is validated by the server and saved as a version.

// Why the design cannot be edited directly now, or "".
export function canvasLock(project: Project): string {
  const d = project.design;
  if (!d) return "Belum ada rancangan";
  if (d.status === "approved") return "Rancangan sudah disetujui";
  if (d.revision) return "Simpan atau batalkan perubahan terakhir dulu";
  if (isWorking(project)) return "Tunggu proses yang berjalan";
  return "";
}

// What the last canvas edit made worse; it outlives the panel that made it
// (a removed component unmounts its panel).
export const useCanvasWarnings = create<{ page: string; warnings: string[]; clear: () => void }>((set) => ({
  page: "", warnings: [], clear: () => set({ page: "", warnings: [] }),
}));

function useEdit(page: Page) {
  const { run, pending } = useProject();
  const edit = (ops: CanvasOp[]) => run("canvas", async () => {
    const r = await api.editDesign(page.id, ops);
    useCanvasWarnings.setState({ page: page.id, warnings: r.warnings ?? [] });
  });
  return { busy: pending === "canvas", edit };
}

// Quality check of the last direct edit: a drop is shown, not refused.
export function CanvasWarnings({ page }: { page?: Page }) {
  const { page: at, warnings, clear } = useCanvasWarnings();
  if (!warnings.length || at !== page?.id) return null;
  return (
    <div className="canvas-warn small" role="status">
      <span><strong>Peringatan kualitas:</strong> {warnings.join("; ")}. Kembalikan lewat tab Versi bila tidak diinginkan.</span>
      <button className="link" onClick={clear}>Tutup</button>
    </div>
  );
}

const NODE_LABEL: Record<string, string> = {
  stack: "Tumpukan", row: "Baris", split: "Split", grid: "Grid", sidebar: "Sidebar", section: "Bagian", card: "Kartu", tabs: "Tab",
};

// Layers: the page's Design IR tree. Containers are layout nodes; leaves are
// components, selected here as on the canvas.
export function Layers({ page, selected, onSelect }: { page: Page; selected: number | null; onSelect: (i: number | null) => void }) {
  const comps = page.components ?? [];
  const index = (id: string) => comps.findIndex((c) => c.id === id);
  const Node = ({ n }: { n: LayoutChild }) => {
    if (typeof n === "string") {
      const i = index(n);
      if (i < 0) return null;
      return (
        <li><button className={`layer leaf ${selected === i ? "on" : ""}`} onClick={() => onSelect(selected === i ? null : i)}>{componentName(comps[i])}</button></li>
      );
    }
    const label = (NODE_LABEL[n.type] ?? n.type) + (n.columns ? ` ${n.columns} kolom` : "") + (n.ratio ? ` ${n.ratio}` : "");
    return (
      <li>
        <span className="layer node">{label}</span>
        {!!n.children?.length && <ul>{n.children.map((c, i) => <Node key={typeof c === "string" ? c : c.id ?? i} n={c} />)}</ul>}
      </li>
    );
  };
  return (
    <section className="canvas layers">
      <h4>Layer</h4>
      <ul className="layer-tree">
        <li>
          <button className={`layer page ${selected === null ? "on" : ""}`} onClick={() => onSelect(null)}>
            {page.name}{page.layout ? ` · ${page.layout.frame}${page.layout.nav ? `, nav ${page.layout.nav}` : ""}` : ""}
          </button>
          <ul>
            {page.layout ? <Node n={page.layout.root} />
              : comps.map((c, i) => <li key={i}><button className={`layer leaf ${selected === i ? "on" : ""}`} onClick={() => onSelect(i)}>{componentName(c)}</button></li>)}
          </ul>
        </li>
      </ul>
    </section>
  );
}

// Direct edits of one component: label, emphasis, position in its node,
// remove.
export function CanvasComponent({ project, page, index, onRemoved }: { project: Project; page: Page; index: number; onRemoved: () => void }) {
  const c = page.components?.[index];
  const { busy, edit } = useEdit(page);
  const [label, setLabel] = useState(c?.label ?? "");
  useEffect(() => setLabel(c?.label ?? ""), [c?.label]);
  const lock = canvasLock(project);
  if (!c?.id || lock) return lock ? <p className="small muted canvas-lock">Edit langsung: {lock}</p> : null;
  const at = page.layout ? locate(page.layout.root, c.id) : null;
  const update = (patch: Partial<typeof c>) => edit([{ op: "update", id: c.id, component: { ...c, ...patch } }]);
  const move = (d: number) => at && edit([{ op: "move", id: c.id, parent: at.parent, index: at.index + d }]);
  return (
    <section className="canvas">
      <h4>Edit langsung</h4>
      <label className="canvas-row">Label
        <input value={label} disabled={busy} onChange={(e) => setLabel(e.target.value)}
          onBlur={() => label !== c.label && update({ label })} onKeyDown={(e) => e.key === "Enter" && (e.target as HTMLInputElement).blur()} />
      </label>
      {c.type === "button" && (
        <label className="canvas-row">Penekanan
          <select value={c.emphasis ?? ""} disabled={busy} onChange={(e) => update({ emphasis: e.target.value || undefined })}>
            <option value="">bawaan</option>
            {EMPHASES.map((x) => <option key={x} value={x}>{x}</option>)}
          </select>
        </label>
      )}
      <div className="inline">
        {at && <button className="ghost small" disabled={busy || at.index === 0} onClick={() => move(-1)}>↑ Naik</button>}
        {at && <button className="ghost small" disabled={busy || at.index >= at.count - 1} onClick={() => move(1)}>↓ Turun</button>}
        <button className="ghost small danger" disabled={busy} onClick={async () => { if (await edit([{ op: "remove", id: c.id }])) onRemoved(); }}>Hapus</button>
      </div>
    </section>
  );
}

// Direct edits of the page: name and density.
export function CanvasPage({ project, page }: { project: Project; page: Page }) {
  const { busy, edit } = useEdit(page);
  const [name, setName] = useState(page.name);
  useEffect(() => setName(page.name), [page.name]);
  const lock = canvasLock(project);
  if (lock) return <p className="small muted canvas-lock">Edit langsung: {lock}</p>;
  return (
    <section className="canvas">
      <h4>Edit langsung</h4>
      <label className="canvas-row">Nama
        <input value={name} disabled={busy} onChange={(e) => setName(e.target.value)}
          onBlur={() => name.trim() && name !== page.name && edit([{ op: "set_page", fields: { name } }])}
          onKeyDown={(e) => e.key === "Enter" && (e.target as HTMLInputElement).blur()} />
      </label>
      <label className="canvas-row">Kepadatan
        <select value={page.density ?? ""} disabled={busy} onChange={(e) => edit([{ op: "set_page", fields: { density: e.target.value } }])}>
          {!page.density && <option value="">—</option>}
          {DENSITIES.map((d) => <option key={d} value={d}>{DENSITY_LABEL[d as Density]}</option>)}
        </select>
      </label>
    </section>
  );
}

const SOURCE: Record<string, string> = {
  generate: "Dibuat AI", revision: "Revisi AI", qa: "Perbaikan visual", reference: "Referensi", theme: "Ganti theme",
  canvas: "Edit langsung", keep: "Disimpan", undo: "Dibatalkan", restore: "Dikembalikan",
};

// Versions: every design state that landed; compare one with the current
// design or restore it.
export function Versions({ project, onOpen }: { project: Project; onOpen: (pageId: string) => void }) {
  const { run, pending } = useProject();
  const [list, setList] = useState<VersionSummary[]>([]);
  const [error, setError] = useState("");
  const [cmp, setCmp] = useState<VersionComparison | null>(null);
  useEffect(() => {
    api.versions().then(setList, (e) => setError(String(e)));
  }, [project.updated_at]);
  const lock = canvasLock(project);
  const compare = (id: number) => api.compareVersions(id).then(setCmp, (e) => setError(String(e)));

  return (
    <div className="info versions">
      <h1>Versi rancangan</h1>
      <p className="muted small">Setiap perubahan yang tersimpan menjadi versi. Bandingkan dengan rancangan sekarang atau kembalikan.</p>
      {error && <p className="warn-text small">{error}</p>}
      {list.length === 0 && !error && <p className="muted">Belum ada versi.</p>}
      <ul className="version-list">
        {list.map((v) => (
          <li key={v.id} className={v.current ? "current" : ""}>
            <div>
              <strong>v{v.id}</strong> {SOURCE[v.source] ?? v.source}
              {v.current && <span className="dw-status ok small"> sekarang</span>}
              <div className="small muted">
                {new Date(v.at).toLocaleString()} · {v.pages} halaman{v.theme ? ` · ${v.theme}` : ""}{v.quality ? ` · kualitas ${v.quality}` : ""}
              </div>
              {v.note && <div className="small muted version-note">{v.note}</div>}
            </div>
            {!v.current && (
              <span className="inline">
                <button className="ghost small" onClick={() => compare(v.id)}>Bandingkan</button>
                <button className="ghost small" disabled={!!lock || pending === "restore"} title={lock || undefined}
                  onClick={() => run("restore", () => api.restoreVersion(v.id)).then((ok) => ok && setCmp(null))}>Kembalikan</button>
              </span>
            )}
          </li>
        ))}
      </ul>
      {cmp && (
        <section className="version-compare">
          <h4>v{cmp.from} → sekarang</h4>
          {!cmp.changes?.length && !cmp.system_changed && !cmp.theme && !cmp.template && !cmp.flows_changed?.length && <p className="muted small">Tidak ada perbedaan halaman.</p>}
          {cmp.theme && <p className="small">Theme: {cmp.theme[0] || "—"} → {cmp.theme[1] || "—"}</p>}
          {cmp.template && <p className="small">Template: {cmp.template[0] || "—"} → {cmp.template[1] || "—"}</p>}
          {cmp.system_changed && <p className="small">Design system berubah</p>}
          {!!cmp.flows_changed?.length && <p className="small">Alur berubah: {cmp.flows_changed.join(", ")}</p>}
          <ul className="plain">
            {(cmp.changes ?? []).map((c, i) => (
              <li key={i}>
                <span className={`sym ${c.kind}`}>{c.kind === "added" ? "+" : c.kind === "removed" ? "−" : "~"}</span>
                {c.page_id ? <button className="link" onClick={() => onOpen(c.page_id!)}>{c.page}</button> : c.page}
                {!!c.details?.length && <div className="small muted">{c.details.join(" · ")}</div>}
              </li>
            ))}
          </ul>
        </section>
      )}
    </div>
  );
}

const LINK_KIND: Record<string, string> = { target: "tombol/elemen", nav: "menu", back: "kembali", primary: "aksi utama" };

// Prototype map: the navigation the prototype runs on, derived from the IR;
// the same links the final spec carries.
export function PrototypeMap({ project, onOpen, onTry }: { project: Project; onOpen: (pageId: string) => void; onTry: (flowId: string) => void }) {
  const [proto, setProto] = useState<Prototype | null>(null);
  const [error, setError] = useState("");
  useEffect(() => {
    api.prototype().then(setProto, (e) => setError(String(e)));
  }, [project.updated_at]);
  const name = (id: string) => proto?.pages?.find((p) => p.id === id)?.name ?? id;
  const Page = ({ id }: { id: string }) => <button className="link" onClick={() => onOpen(id)}>{name(id)}</button>;
  if (error) return <div className="info"><p className="warn-text">{error}</p></div>;
  if (!proto) return null;
  return (
    <div className="info proto-map">
      <h1>Peta prototipe</h1>
      <p className="muted small">Navigasi diturunkan dari Design IR; spesifikasi final memakai tautan yang sama.</p>
      {(!!proto.unreachable?.length || !!proto.dead_ends?.length) && (
        <section>
          {!!proto.unreachable?.length && <p className="warn-text small">Tidak bisa dicapai dari halaman awal: {proto.unreachable.map((id) => <Page key={id} id={id} />)}</p>}
          {!!proto.dead_ends?.length && <p className="warn-text small">Halaman buntu (tanpa jalan keluar): {proto.dead_ends.map((id) => <Page key={id} id={id} />)}</p>}
        </section>
      )}
      {!!proto.flows?.length && (
        <section>
          <h4>Alur</h4>
          {proto.flows.map((f) => (
            <div key={f.id} className="proto-flow">
              <strong>{f.name}</strong> <button className="ghost small" onClick={() => onTry(f.id)}>▶ Coba</button>
              <div className="small">{f.steps.map((id, i) => <span key={id}>{i > 0 && " → "}<Page id={id} /></span>)}</div>
              {!!f.gaps?.length && <div className="warn-text small">Tanpa tautan: {f.gaps.join(", ")}</div>}
            </div>
          ))}
        </section>
      )}
      <section>
        <h4>Halaman</h4>
        <ul className="plain">
          {(proto.pages ?? []).map((p) => (
            <li key={p.id}>
              <Page id={p.id} />{p.entry && <span className="small muted"> (awal)</span>}{p.route && <code className="small"> {p.route}</code>}
              {p.links.length > 0 && (
                <div className="small muted">{p.links.map((l) => `${l.via || LINK_KIND[l.kind]} → ${name(l.to)}`).join(" · ")}</div>
              )}
            </li>
          ))}
        </ul>
      </section>
    </div>
  );
}
