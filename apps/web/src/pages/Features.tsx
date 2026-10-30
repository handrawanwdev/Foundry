import { useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { api } from "../api";
import { ConfirmDialog } from "../components/ConfirmDialog";
import { Elapsed } from "../components/Elapsed";
import { PromptBar } from "../components/PromptBar";
import {
  addFeature, addModule, featureCount, featureMapOf, findModule, locate, pathOf, removeId, toSave, updateModule, updateNode,
} from "../features";
import { isWorking } from "../flow";
import { useProject } from "../store/project";
import type { FeatureMap, FeatureNode } from "../types";

type NodeForm = { name: string; description: string; purpose: string; benefit: string; outcome: string };

const emptyForm = (): NodeForm => ({ name: "", description: "", purpose: "", benefit: "", outcome: "" });

export function FeaturesPage() {
  const { project, run, pending } = useProject();
  const nav = useNavigate();
  const [params, setParams] = useSearchParams();
  const [editing, setEditing] = useState(false);
  const [form, setForm] = useState<NodeForm>(emptyForm());
  const [confirm, setConfirm] = useState<"generate" | "delete" | null>(null);

  if (!project) return null;
  if (project.stage === "TELL") {
    return (
      <div className="dw-empty">
        <h1>Belum ada daftar fitur</h1>
        <p className="muted">Ceritakan dulu produknya. Nanti Foundry bikin daftar modul dan fiturnya.</p>
        <Link className="button primary" to="/simple/tell">Tulis prompt</Link>
      </div>
    );
  }

  const map = featureMapOf(project);
  const working = isWorking(project);
  const mapping = working && project.job?.kind === "feature_map";
  const locked = project.stage === "READY";
  const count = featureCount(map);
  const empty = count === 0;
  const mid = params.get("m") ?? map.modules[0]?.id ?? "";
  const fid = params.get("f") ?? "";
  const located = fid ? locate(map, fid) : {};
  const mod = located.module ?? findModule(map, mid) ?? map.modules[0];
  const node = located.node;
  const trail = node ? pathOf(map, node.id) : { module: mod, nodes: [] as FeatureNode[] };
  const ancestors = trail.nodes.slice(0, -1);
  const selectedId = node?.id || mod?.id || "";
  const canEdit = !locked && !working;
  const canContinue = count > 0 && !working && !locked && project.stage === "DISCOVERY";
  const saving = pending === "saveFeatureMap";
  const generating = pending === "generateFeatureMap";

  const select = (m = "", f = "") => {
    setEditing(false);
    setConfirm(null);
    const next: Record<string, string> = {};
    if (m) next.m = m;
    if (f) next.f = f;
    setParams(next, { replace: true });
  };

  const save = (next: FeatureMap) => run("saveFeatureMap", () => api.saveFeatureMap(toSave(next)));

  if (empty && mapping) {
    return (
      <div className="dw-empty">
        <h1><span className="spinner" /> Menyusun daftar fitur <Elapsed started={project.job?.started} /></h1>
        <p className="muted">Foundry baca ceritamu, lalu bikin modul, fitur, dan subfitur.</p>
        {project.story && <blockquote>{project.story}</blockquote>}
      </div>
    );
  }

  const startEdit = () => {
    if (node) {
      setForm({
        name: node.name, description: node.description ?? "", purpose: node.purpose ?? "",
        benefit: node.benefit ?? "", outcome: node.outcome ?? "",
      });
    } else if (mod) {
      setForm({ ...emptyForm(), name: mod.name });
    }
    setEditing(true);
    setConfirm(null);
  };

  const submitEdit = async () => {
    if (!form.name.trim() || !mod) return;
    const next = node
      ? updateNode(map, node.id, {
        name: form.name.trim(), description: form.description.trim(),
        purpose: form.purpose.trim(), benefit: form.benefit.trim(), outcome: form.outcome.trim(),
      })
      : updateModule(map, mod.id, form.name.trim());
    if (await save(next)) setEditing(false);
  };

  const onAddModule = async () => {
    const next = addModule(map);
    const created = next.modules[next.modules.length - 1];
    if (await save(next)) select(created.id);
  };

  const onAddFeature = async (parentId: string | null) => {
    if (!mod) return;
    const next = addFeature(map, mod.id, parentId);
    const created = parentId
      ? (locate(next, parentId).node?.children ?? []).at(-1)
      : next.modules.find((x) => x.id === mod.id)?.features.at(-1);
    if (await save(next) && created) select(mod.id, created.id);
  };

  const confirmDelete = async () => {
    if (!selectedId) return;
    const next = removeId(map, selectedId);
    if (await save(next)) {
      setConfirm(null);
      select(node ? mod?.id : next.modules[0]?.id);
    }
  };

  const runGenerate = async () => {
    setConfirm(null);
    await run("generateFeatureMap", () => api.generateFeatureMap(""));
  };

  const onGenerate = () => {
    if (count > 0) {
      setConfirm("generate");
      return;
    }
    void runGenerate();
  };

  const continueToBlueprint = async () => {
    if (await run("startBlueprint", () => api.startBlueprint())) nav("/blueprint");
  };

  return (
    <div className="ws">
      <nav className="ws-nav" aria-label="Modul fitur">
        <div className="ws-nav-head">
          <strong>Fitur</strong>
          <span className="muted small">{count} fitur</span>
        </div>
        {map.summary && <p className="feat-summary">{map.summary}</p>}
        <div className="ws-group">
          <div className="ws-group-label feat-mod-head">
            <span>Modul</span>
            {canEdit && (
              <button type="button" className="ghost" onClick={() => void onAddModule()} disabled={saving}>
                Tambah
              </button>
            )}
          </div>
          {map.modules.map((m) => (
            <button key={m.id} className={`ws-item ${m.id === mod?.id ? "on" : ""}`}
              aria-current={m.id === mod?.id ? "page" : undefined} onClick={() => select(m.id)}>
              <span className="ws-item-text">{m.name}</span>
              <span className="ws-count">{featureCount({ modules: [m] })}</span>
            </button>
          ))}
          {map.modules.length === 0 && <p className="muted small feat-empty-nav">Belum ada modul.</p>}
        </div>
        {canEdit && (
          <div className="feat-nav-actions">
            <button type="button" className="ghost" onClick={onGenerate} disabled={generating}>
              {count > 0 ? "Generate ulang" : "Generate fitur"}
            </button>
            {count > 0 && (
              <button type="button" className="ghost" onClick={() => void run("findFeatureGaps", () => api.findFeatureGaps(""))}
                disabled={pending === "findFeatureGaps"}>Cari yang belum ada</button>
            )}
          </div>
        )}
        <div className="feat-nav-foot">
          {canContinue && (
            <button className="primary" disabled={pending === "startBlueprint"} onClick={() => void continueToBlueprint()}>
              Lanjut ke Blueprint
            </button>
          )}
          {project.stage === "DESIGN" && <p className="muted small">Blueprint sudah ada. Daftar fitur masih bisa diubah.</p>}
          {locked && <p className="muted small">Blueprint sudah dikunci. Buka lagi kalau mau ubah daftar fitur.</p>}
        </div>
      </nav>

      <main className="ws-main">
        {empty ? (
          <div className="doc">
            <header className="doc-head">
              <h1>Daftar fitur</h1>
              <p className="doc-summary">Belum ada fitur. Generate dari cerita, atau tambah modul sendiri.</p>
              {canEdit && (
                <div className="doc-actions">
                  <button className="primary" disabled={generating} onClick={onGenerate}>Generate fitur</button>
                  <button onClick={() => void onAddModule()} disabled={saving}>Tambah modul</button>
                </div>
              )}
            </header>
          </div>
        ) : !mod ? (
          <p className="muted">Pilih modul di kiri.</p>
        ) : (
          <div className="doc">
            <header className="doc-head">
              {node && (
                <nav className="feat-path" aria-label="Lokasi fitur">
                  <button type="button" className="ghost" onClick={() => select(mod.id)}>{mod.name}</button>
                  {ancestors.map((a) => (
                    <span key={a.id} className="feat-path-seg">
                      <span className="feat-path-sep" aria-hidden="true">/</span>
                      <button type="button" className="ghost" onClick={() => select(mod.id, a.id)}>{a.name}</button>
                    </span>
                  ))}
                  <span className="feat-path-seg">
                    <span className="feat-path-sep" aria-hidden="true">/</span>
                    <span className="feat-path-now" aria-current="page">{node.name}</span>
                  </span>
                </nav>
              )}
              <h1>{node ? node.name : mod.name}</h1>
              {!node && (
                <p className="doc-summary">
                  {mod.features?.length
                    ? `${featureCount({ modules: [mod] })} fitur di modul ini.`
                    : "Modul ini belum punya fitur."}
                </p>
              )}
              {canEdit && !editing && (
                <div className="doc-actions">
                  {!node && <button onClick={() => void onAddFeature(null)} disabled={saving}>Tambah fitur</button>}
                  {node && <button onClick={() => void onAddFeature(node.id)} disabled={saving}>Tambah subfitur</button>}
                  <button onClick={startEdit}>Ubah</button>
                  <button className="ghost danger" onClick={() => setConfirm("delete")} disabled={saving}>Hapus</button>
                </div>
              )}
            </header>

            {editing ? (
              <form className="doc-edit" onSubmit={(e) => { e.preventDefault(); void submitEdit(); }}>
                <label className="field">Nama
                  <input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required />
                </label>
                {node && (
                  <>
                    <label className="field">Deskripsi
                      <textarea rows={3} value={form.description} onChange={(e) => setForm({ ...form, description: e.target.value })} placeholder="Apa yang dilakukan fitur ini" />
                    </label>
                    <label className="field">Tujuan
                      <textarea rows={2} value={form.purpose} onChange={(e) => setForm({ ...form, purpose: e.target.value })} placeholder="Buat apa" />
                    </label>
                    <label className="field">Manfaat
                      <textarea rows={2} value={form.benefit} onChange={(e) => setForm({ ...form, benefit: e.target.value })} placeholder="Enaknya apa buat yang pakai" />
                    </label>
                    <label className="field">Hasil
                      <textarea rows={2} value={form.outcome} onChange={(e) => setForm({ ...form, outcome: e.target.value })} placeholder="Kalau berhasil, jadi apa" />
                    </label>
                  </>
                )}
                <div className="row">
                  <button type="submit" className="primary" disabled={!form.name.trim() || saving}>Simpan</button>
                  <button type="button" className="ghost" onClick={() => setEditing(false)}>Batal</button>
                </div>
              </form>
            ) : (
              <FeatureList
                nodes={mod.features ?? []}
                selectedId={node?.id}
                onOpen={(id) => id ? select(mod.id, id) : select(mod.id)}
              />
            )}
          </div>
        )}
      </main>

      {canEdit && !empty && (
        <PromptBar
          label={selectedId
            ? `Kembangkan ${node?.name ?? mod?.name ?? "item ini"}…`
            : "Cari fitur yang belum ada…"}
          placeholder={node || mod
            ? "Contoh: pecah jadi subfitur, atau isi tujuan dan manfaatnya."
            : "Contoh: yang belum ada untuk pembayaran, notifikasi, atau laporan."}
          hint={node || mod
            ? "AI bantu lengkapi yang sedang dipilih. Ctrl+Enter untuk kirim."
            : "AI cari modul atau fitur yang belum ada. Ctrl+Enter untuk kirim."}
          disabled={working}
          busy={pending === "expandFeature" || pending === "findFeatureGaps"}
          onSend={(text) => selectedId
            ? run("expandFeature", () => api.expandFeature(selectedId, text))
            : run("findFeatureGaps", () => api.findFeatureGaps(text))}
        />
      )}

      {confirm === "generate" && (
        <ConfirmDialog
          title="Generate ulang daftar fitur?"
          body="Daftar yang sekarang diganti semua. Yang sudah kamu ubah juga hilang."
          confirmLabel="Ya, generate ulang"
          danger
          busy={generating}
          busyLabel="Mengganti…"
          onCancel={() => setConfirm(null)}
          onConfirm={() => void runGenerate()}
        />
      )}
      {confirm === "delete" && mod && (
        <ConfirmDialog
          title={node ? `Hapus fitur “${node.name}”?` : `Hapus modul “${mod.name}”?`}
          body={node
            ? "Fitur ini dan subfiturnya ikut terhapus."
            : "Modul ini dan semua fiturnya ikut terhapus."}
          confirmLabel="Ya, hapus"
          danger
          busy={saving}
          busyLabel="Menghapus…"
          onCancel={() => setConfirm(null)}
          onConfirm={() => void confirmDelete()}
        />
      )}
    </div>
  );
}

function FeatureList({ nodes, onOpen, selectedId, nested = false }: {
  nodes: FeatureNode[];
  onOpen: (id: string) => void;
  selectedId?: string;
  nested?: boolean;
}) {
  if (!nodes.length) return null;
  return (
    <ul className={`feat-list${nested ? " nested" : ""}`}>
      {nodes.map((n) => {
        const kids = n.children?.length ?? 0;
        const on = n.id === selectedId;
        return (
          <li key={n.id}>
            <button
              type="button"
              className={`feat-row${on ? " on" : ""}`}
              aria-current={on ? "page" : undefined}
              onClick={() => onOpen(on ? "" : n.id)}
            >
              <span className="feat-row-name">{n.name}</span>
              {n.description && <span className="feat-row-desc">{n.description}</span>}
              {kids > 0 && <span className="feat-row-n">{kids}</span>}
            </button>
            {on && <FeatureDetail node={n} />}
            {kids > 0 && <FeatureList nodes={n.children!} onOpen={onOpen} selectedId={selectedId} nested />}
          </li>
        );
      })}
    </ul>
  );
}

function FeatureDetail({ node }: { node: FeatureNode }) {
  const facts: [string, string][] = [
    ["Tujuan", node.purpose ?? ""],
    ["Manfaat", node.benefit ?? ""],
    ["Hasil", node.outcome ?? ""],
  ];
  return (
    <div className="feat-detail">
      <p className="feat-lead">
        {node.description || <span className="muted">Belum ada deskripsi.</span>}
      </p>
      <dl className="feat-facts">
        {facts.map(([k, v]) => (
          <div key={k}>
            <dt>{k}</dt>
            <dd>{v || <span className="muted">Belum diisi.</span>}</dd>
          </div>
        ))}
      </dl>
    </div>
  );
}
