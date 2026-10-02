import { useEffect, useState } from "react";
import { api } from "../api";
import { isWorking } from "../flow";
import { STRUCTURE_TYPE, structureState, structureVersion, tree } from "../structure";
import { changeText, TECH_STATUS } from "../tech";
import { useProject } from "../store/project";
import type { PartKind, Project, StructureIR, StructureOp, StructureProposal, Structure, Template, TechIssue } from "../types";

const KIND: Record<string, string> = {
  app: "App", service: "Service", module: "Modul", worker: "Worker", package: "Package", structure: "Struktur", technology: "Teknologi",
};
const SOURCE: Record<string, string> = { recommend: "rekomendasi", user: "diubah user", ai: "permintaan ke AI", technology: "mengikuti teknologi" };

// Project structure workspace: Foundry recommends, the user edits (by hand
// or in words), sees the impact, applies and approves. The build follows
// the approved structure.
export function StructurePanel({ project }: { project: Project }) {
  const { run, pending } = useProject();
  const [types, setTypes] = useState<Template[]>([]);
  const [ask, setAsk] = useState("");
  const [editing, setEditing] = useState(false);
  useEffect(() => { void api.templates().then(setTypes, () => {}); }, []);
  const s = project.structure!;
  const ir = s.ir!;
  const designApproved = project.design?.status === "approved";
  const busy = isWorking(project) || pending === "structure";
  const editable = !designApproved && !busy && !s.request;
  const state = structureState(s);
  const shown = s.proposal?.ir ?? ir;
  const errors = (s.issues ?? []).filter((i) => i.level === "error");
  const propose = (ops: StructureOp[]) => run("structure", () => api.proposeStructure(ops));
  const send = async () => { if (await run("structure", () => api.requestStructure(ask))) setAsk(""); };

  return (
    <section className="info-sec">
      <div className="tech-title">
        <h3>Struktur project</h3>
        <span className={`tech-status ${state}`}>{TECH_STATUS[state]}{structureVersion(s) ? ` · v${structureVersion(s)}` : ""}</span>
      </div>
      <p className="small muted">
        Rekomendasi Foundry: <strong className="fg">{STRUCTURE_TYPE[s.recommended ?? ""] ?? "—"}</strong>
        {s.reason && <> — {s.reason}</>}. Kamu yang memutuskan; yang disetujui dipakai saat Build.
      </p>

      <div className="tech-modes">
        {types.map((t) => (
          <label key={t.id} className="check small">
            <input type="radio" name="structure-type" checked={shown.type === t.id} disabled={!editable}
              onChange={() => void propose([{ op: "type", name: t.id }])} />
            <strong>{t.name}</strong>{s.recommended === t.id && <span className="tech-tag">rekomendasi</span>} <span className="muted">{t.description}</span>
          </label>
        ))}
      </div>

      <p className="small muted">{s.proposal ? "Preview usulan (belum diterapkan):" : "Preview:"}</p>
      <pre className="code small">{tree(shown)}</pre>
      <Parts ir={shown} />

      {!designApproved && (
        <div className="tech-ask">
          <textarea rows={2} maxLength={1000} value={ask} disabled={!editable}
            placeholder="mis. Reporting jangan jadi service sendiri, jadikan modul di operations."
            onChange={(e) => setAsk(e.target.value)} />
          <button disabled={!editable || !ask.trim()} onClick={() => void send()}>✦ Minta Perubahan dengan AI</button>
          {s.request && <p className="small muted"><span className="spinner" /> AI memahami permintaan: “{s.request}”</p>}
        </div>
      )}

      {editable && (editing
        ? <StructureEdit ir={shown} onDone={() => setEditing(false)} />
        : <button className="ghost small" onClick={() => setEditing(true)}>✎ Edit Structure</button>)}

      {s.proposal && <Proposal p={s.proposal} disabled={busy || designApproved} />}
      {!s.proposal && <Issues issues={s.issues} />}

      <div className="tech-foot">
        {editable && !s.proposal && s.recommended && (
          <button className="ghost small" onClick={() => void run("structure", api.recommendStructure)}>↻ Gunakan rekomendasi</button>
        )}
        {state === "approved" ? <span className="ok-text small">✓ Struktur v{structureVersion(s)} disetujui{designApproved ? " dan dipakai build" : ""}</span>
          : !designApproved && (
            <button className="primary" disabled={!editable || !!s.proposal || errors.length > 0}
              title={s.proposal ? "Terapkan atau batalkan perubahan dulu" : errors.length ? "Selesaikan masalah di atas dulu" : undefined}
              onClick={() => void run("structure", api.approveStructure)}>Approve Structure</button>
          )}
      </div>
      {state === "changed" && <p className="small warn-text">Struktur berubah setelah disetujui. Setujui lagi sebelum rancangan disetujui.</p>}

      <History s={s} />
    </section>
  );
}

// Why each part exists.
function Parts({ ir }: { ir: StructureIR }) {
  const parts = [...(ir.apps ?? []), ...(ir.services ?? []), ...(ir.workers ?? []), ...(ir.packages ?? [])];
  return (
    <details className="small">
      <summary>Alasan tiap bagian</summary>
      <ul>{parts.map((p) => <li key={p.path}><code>{p.path}</code> {p.reason || <span className="muted">ditambahkan user</span>}
        {!!p.modules?.length && <span className="muted"> · modul: {p.modules.join(", ")}</span>}</li>)}</ul>
    </details>
  );
}

type Op = StructureOp["op"];
const OPS: [Op, string][] = [["add", "Tambah"], ["rename", "Ganti nama"], ["move", "Pindahkan"], ["remove", "Hapus"]];

// One edit at a time; each becomes (part of) the waiting proposal.
function StructureEdit({ ir, onDone }: { ir: StructureIR; onDone: () => void }) {
  const { run, pending } = useProject();
  const [op, setOp] = useState<Op>("add");
  const [kind, setKind] = useState<PartKind>("module");
  const [name, setName] = useState("");
  const [to, setTo] = useState("");
  const [reason, setReason] = useState("");
  const services = (ir.services ?? []).map((p) => p.name);
  const names: Record<PartKind, string[]> = {
    app: (ir.apps ?? []).map((p) => p.name), service: services,
    module: (ir.services ?? []).flatMap((p) => p.modules ?? []), worker: (ir.workers ?? []).map((p) => p.name),
    package: (ir.packages ?? []).map((p) => p.name),
  };
  const kinds: PartKind[] = op === "move" ? ["module", "service"] : ["app", "service", "module", "worker", "package"];
  const needsTo = op === "rename" || op === "move" || (op === "add" && kind === "module");
  const save = async () => {
    if (await run("structure", () => api.proposeStructure([{ op, kind, name: name.trim(), to: to.trim() || undefined, reason: reason.trim() || undefined }]))) {
      setName(""); setTo(""); setReason("");
    }
  };
  return (
    <div className="tech-edit">
      <div className="tech-actions">
        <select value={op} onChange={(e) => { setOp(e.target.value as Op); setName(""); setTo(""); if (e.target.value === "move") setKind("module"); }}>
          {OPS.map(([v, l]) => <option key={v} value={v}>{l}</option>)}
        </select>
        <select value={kind} onChange={(e) => { setKind(e.target.value as PartKind); setName(""); setTo(""); }}>
          {kinds.map((k) => <option key={k} value={k}>{KIND[k]}</option>)}
        </select>
        {op === "add"
          ? <input value={name} maxLength={32} placeholder="nama, mis. reporting" onChange={(e) => setName(e.target.value)} />
          : (
            <select value={name} onChange={(e) => setName(e.target.value)}>
              <option value="">— pilih —</option>
              {names[kind].map((n) => <option key={n} value={n}>{n}</option>)}
            </select>
          )}
        {needsTo && (op === "rename"
          ? <input value={to} maxLength={32} placeholder="nama baru" onChange={(e) => setTo(e.target.value)} />
          : (
            <select value={to} onChange={(e) => setTo(e.target.value)}>
              <option value="">{op === "move" ? "ke service…" : "di service…"}</option>
              {services.filter((n) => n !== name).map((n) => <option key={n} value={n}>{n}</option>)}
            </select>
          ))}
      </div>
      {op === "add" && <input value={reason} maxLength={200} placeholder="Alasan, mis. domain laporan terpisah" onChange={(e) => setReason(e.target.value)} />}
      <div className="tech-actions">
        <button disabled={pending === "structure" || !name.trim() || (needsTo && !to.trim() && !(op === "add" && services.length === 1))}
          onClick={() => void save()}>Lihat dampak</button>
        <button className="ghost" onClick={onDone}>Tutup</button>
      </div>
    </div>
  );
}

// The waiting change with its validation and impact; the user decides.
function Proposal({ p, disabled }: { p: StructureProposal; disabled: boolean }) {
  const { run, pending } = useProject();
  return (
    <div className="tech-proposal">
      <strong>⚠️ Review Impact{p.source === "ai" ? " — usulan AI" : p.source === "recommend" ? " — rekomendasi Foundry" : ""}</strong>
      {p.request && <p className="small muted">Permintaan: “{p.request}”</p>}
      {p.note && <p className="small">{p.note}</p>}
      <ul className="small">{p.changes.map((c) => <li key={c}><strong>{c}</strong></li>)}</ul>
      {!!p.tech?.length && (
        <p className="small warn-text">Teknologi ikut berubah saat diterapkan: {p.tech.map(changeText).join("; ")}</p>
      )}
      <Issues issues={p.issues} ok="Struktur ini sesuai dengan teknologi dan kebutuhan project." />
      {!!p.impact?.length && (
        <details className="small" open>
          <summary>Bagian yang terdampak</summary>
          <ul>{p.impact.map((i, k) => <li key={k}><strong>{i.area}:</strong> {i.message}</li>)}</ul>
        </details>
      )}
      <div className="tech-actions">
        <button className="primary" disabled={disabled || pending === "structure"} onClick={() => void run("structure", api.applyStructure)}>Terapkan Perubahan</button>
        <button className="ghost" disabled={disabled || pending === "structure"} onClick={() => void run("structure", api.discardStructure)}>Batalkan</button>
      </div>
    </div>
  );
}

function Issues({ issues, ok }: { issues?: TechIssue[]; ok?: string }) {
  if (!issues?.length) return ok ? <p className="small ok-text">✓ {ok}</p> : null;
  return (
    <ul className="tech-issues small">
      {issues.map((i, k) => (
        <li key={k} className={i.level}>
          {i.level === "error" ? "✕" : "!"} {i.area && <strong>{KIND[i.area] ?? i.area}: </strong>}{i.message}
        </li>
      ))}
    </ul>
  );
}

function History({ s }: { s: Structure }) {
  const versions = [...(s.history ?? [])].reverse();
  if (versions.length < 2) return null;
  return (
    <details className="tech-history small">
      <summary>Riwayat struktur ({versions.length} versi)</summary>
      <ul>
        {versions.map((v, i) => (
          <li key={v.version}>
            <strong>v{v.version}</strong> {STRUCTURE_TYPE[v.ir.type] ?? v.ir.type} — {i === 0 ? "saat ini" : "sebelumnya"} · {new Date(v.at).toLocaleString("id-ID")}
            {" · "}{SOURCE[v.source] ?? v.source} · {TECH_STATUS[v.status] ?? v.status}
            {v.request && <div className="muted">“{v.request}”</div>}
            {!!v.changes?.length && <div className="muted">{v.changes.join("; ")}</div>}
          </li>
        ))}
      </ul>
    </details>
  );
}
