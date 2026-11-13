import { useState } from "react";
import { api } from "../api";
import { Elapsed } from "./Elapsed";
import { isWorking } from "../flow";
import {
  modulePath, otherAlternatives, recommendedAlternative, SCORE_LABEL, structureState, structureVersion, tree,
} from "../structure";
import { changeText, TECH_STATUS } from "../tech";
import { useProject } from "../store/project";
import type {
  PartKind, Project, StructureAlternative, StructureIR, StructureOp, StructurePlan, StructureProposal, Structure,
  StructureEvaluation, TechIssue,
} from "../types";

const KIND: Record<string, string> = {
  app: "App", service: "Service", module: "Modul", worker: "Worker", package: "Package", structure: "Struktur", technology: "Teknologi",
};
const SOURCE: Record<string, string> = { recommend: "rekomendasi", user: "diubah user", ai: "AI", technology: "mengikuti teknologi" };

// Structure decision: the AI designs the style and folders (Foundry has no
// template), the user edits (by hand or in words), sees the impact,
// applies and approves. The folder plan on this page spells out the
// approved shape.
export function StructurePanel({ project }: { project: Project }) {
  const { run, pending } = useProject();
  const [editing, setEditing] = useState(false);
  const [typeOpen, setTypeOpen] = useState(false);
  const s = project.structure!;
  const ir = s.ir!;
  const finalized = project.stage === "READY";
  const busy = isWorking(project) || pending === "structure";
  const editable = !finalized && !busy && !s.request && !s.plan_request;
  const state = structureState(s);
  const shown = s.proposal?.ir ?? ir;
  const errors = (s.issues ?? []).filter((i) => i.level === "error");
  const plan = s.plan;
  const rec = recommendedAlternative(plan);
  const recommendation = rec?.name ?? s.recommended ?? "—";
  // Folder tree, specs, rules and history live in the folder plan once it exists.
  const repo = s.repo && !s.proposal ? s.repo : undefined;
  const compact = !s.proposal && state === "approved";

  return (
    <section className="info-sec">
      <div className="tech-title">
        <h2>Keputusan struktur</h2>
        <span className={`tech-status ${state}`}>{TECH_STATUS[state]}{structureVersion(s) ? ` · v${structureVersion(s)}` : ""}</span>
      </div>
      <p className="small muted">
        Rekomendasi: <strong>{recommendation}</strong>
        {!compact && s.reason && <> — {s.reason}</>}
        {!compact && ". Kamu yang memutuskan; yang disetujui masuk spesifikasi final."}
      </p>
      {!compact && (
        <p className="small muted">
          Gaya arsitektur dan folder dirancang AI untuk sistem ini (tanpa template), konsisten dengan keputusan
          Teknologi dan seluruh blueprint. Rencana folder di bawah merinci folder dan tanggung jawabnya.
        </p>
      )}

      {!finalized && s.request && <p className="small muted"><span className="spinner" /> AI memahami permintaan: “{s.request}” <Elapsed started={project.job?.started} /></p>}
      {!finalized && s.plan_request && <p className="small muted"><span className="spinner" /> AI menganalisis sistem dan membandingkan alternatif struktur… <Elapsed started={project.job?.started} /></p>}

      {compact && plan?.stale && (
        <p className="small warn-text">
          Artefak sumber berubah setelah analisis ini. Analisis ulang dulu, atau buka analisis di bawah.
        </p>
      )}
      {s.proposal && <Proposal p={s.proposal} disabled={busy || finalized} />}
      {!s.proposal && <StructureAnalysis plan={plan} editable={editable} busy={busy} finalized={finalized} hideRules={!!repo || compact} compact={compact} />}

      <div className="tech-list">
        <TypeRow s={s} type={shown.type} editable={editable && !s.proposal} open={typeOpen}
          compact={compact} onToggle={() => setTypeOpen(!typeOpen)} onDone={() => setTypeOpen(false)} />
      </div>

      {!repo && !compact && <>
        <p className="small muted">{s.proposal ? "Preview usulan (belum diterapkan):" : "Preview:"}</p>
        <pre className="code small">{tree(shown)}</pre>
        <Parts ir={shown} />
        <Rules rules={shown.rules ?? []} editable={editable} />
      </>}

      {editable && (editing
        ? <StructureEdit ir={shown} onDone={() => setEditing(false)} />
        : <button className="ghost small" onClick={() => setEditing(true)}>Edit struktur</button>)}

      {!s.proposal && <Issues issues={s.issues} />}

      <div className="tech-foot">
        {state === "approved" ? <span className="ok-text small">✓ Struktur v{structureVersion(s)} disetujui{finalized ? " dan masuk spesifikasi final" : ""}</span>
          : !finalized && (
            <button className="primary" disabled={!editable || !!s.proposal || errors.length > 0}
              title={s.proposal ? "Terapkan atau batalkan perubahan dulu" : errors.length ? "Selesaikan masalah di atas dulu" : undefined}
              onClick={() => void run("structure", api.approveStructure)}>Setujui struktur</button>
          )}
      </div>
      {state === "changed" && <p className="small warn-text">Struktur berubah setelah disetujui. Setujui lagi sebelum merancang folder.</p>}

      {!repo && !compact && <History s={s} />}
    </section>
  );
}

// The AI analysis: system summary, the recommendation, 1-2 other
// alternatives with their trade-offs, and the actions the user asked for.
function StructureAnalysis({ plan, editable, busy, finalized, hideRules, compact }: {
  plan?: StructurePlan; editable: boolean; busy: boolean; finalized: boolean; hideRules?: boolean; compact?: boolean;
}) {
  const { run, pending } = useProject();
  const [revision, setRevision] = useState("");
  if (!plan) {
    return (
      <div className="plan-card">
        <div className="tech-title"><strong>Analisis AI</strong><span className="tech-tag">belum ada</span></div>
        <p className="small muted">
          Rekomendasi di atas baru diturunkan dari keputusan teknologi. Tekan <strong>Susun rancangan</strong> —
          AI membaca seluruh blueprint lalu membandingkan 2–3 alternatif struktur, dalam satu proses bersama rancangan lain.
        </p>
      </div>
    );
  }
  const rec = recommendedAlternative(plan);
  const others = otherAlternatives(plan);
  const stale = !!plan.stale;
  const canUse = editable && !busy && !stale;
  const title = (
    <div className="tech-title">
      <strong>Analisis AI</strong>
      {plan.selected && <span className="tech-tag">dipilih {plan.alternatives.find((a) => a.id === plan.selected)?.name}</span>}
      {stale && <span className="tech-tag">kedaluwarsa</span>}
    </div>
  );
  const body = (
    <>
      {plan.summary && <p className="small">{plan.summary}</p>}
      {!!plan.drivers?.length && (
        <p className="small muted">Yang menentukan struktur: {plan.drivers.join(" · ")}</p>
      )}
      {!!plan.domains?.length && (
        <details className="small">
          <summary>Domain dan tanggung jawab ({plan.domains.length})</summary>
          <ul>{plan.domains.map((d) => <li key={d.name}><code>{d.name}</code> — {d.responsibility}</li>)}</ul>
        </details>
      )}
      {stale && (
        <p className="small warn-text">
          Artefak sumber berubah setelah analisis ini. Analisis ulang dulu sebelum memakai rekomendasinya.
        </p>
      )}
      {rec && (
        <AlternativeCard a={rec} plan={plan} primary canUse={canUse} busy={busy} finalized={finalized} open={!compact} />
      )}
      {others.map((a) => (
        <AlternativeCard key={a.id} a={a} plan={plan} canUse={canUse} busy={busy} finalized={finalized} />
      ))}
      {!hideRules && !!plan.dependency_rules?.length && (
        <details className="small">
          <summary>Aturan dependency antarbagian</summary>
          <ul>{plan.dependency_rules.map((r) => <li key={r}>{r}</li>)}</ul>
        </details>
      )}
      {!finalized && editable && (
        <div className="tech-actions plan-revise">
          <input value={revision} placeholder="Minta revisi analisis, mis. “jangan pisahkan reporting; buat worker terpisah”"
            onChange={(e) => setRevision(e.target.value)} />
          <button disabled={busy || pending === "structure" || !revision.trim()}
            onClick={async () => { if (await run("structure", () => api.analyzeStructure(revision.trim()))) setRevision(""); }}>
            Minta revisi
          </button>
          <button className="ghost" disabled={busy || pending === "structure" || !stale}
            title={!stale ? "Artefak sumber belum berubah; analisis tidak diulang" : undefined}
            onClick={() => void run("structure", () => api.analyzeStructure(""))}>
            Analisis ulang
          </button>
        </div>
      )}
    </>
  );
  if (compact) {
    return (
      <details className="plan-card">
        <summary>{title}</summary>
        {body}
      </details>
    );
  }
  return (
    <div className="plan-card">
      {title}
      {body}
    </div>
  );
}

// One alternative: scores, reason, tree, responsibility of each folder,
// trade-offs and the action that adopts it.
function AlternativeCard({ a, plan, primary, canUse, busy, finalized, open }: {
  a: StructureAlternative; plan: StructurePlan; primary?: boolean; canUse: boolean; busy: boolean; finalized: boolean; open?: boolean;
}) {
  const { run, pending } = useProject();
  const chosen = plan.selected === a.id;
  return (
    <details className="plan-alt" open={open}>
      <summary>
        <strong>{a.name}</strong>
        <span className="muted small"> · {a.type} · skor {a.evaluation.score ?? "—"}</span>
        {a.recommended && <span className="tech-tag">rekomendasi AI</span>}
        {chosen && <span className="tech-tag">dipilih</span>}
      </summary>
      <p className="small">{a.summary}</p>
      <Scores e={a.evaluation} />
      {!!a.pros?.length && <p className="small"><strong>Kelebihan:</strong> {a.pros.join(" · ")}</p>}
      {!!a.cons?.length && <p className="small"><strong>Kekurangan:</strong> {a.cons.join(" · ")}</p>}
      {!!a.tradeoffs?.length && <p className="small muted"><strong>Trade-off:</strong> {a.tradeoffs.join(" · ")}</p>}
      <pre className="code small">{tree(a.ir)}</pre>
      <Parts ir={a.ir} />
      {!finalized && canUse && !chosen && (
        <div className="tech-actions">
          <button className={primary ? "primary" : ""} disabled={busy || pending === "structure"}
            onClick={() => void run("structure", () => api.useStructureAlternative(a.id))}>
            {primary ? "Gunakan rekomendasi AI" : "Pilih alternatif ini"}
          </button>
        </div>
      )}
    </details>
  );
}

function Scores({ e }: { e: StructureEvaluation }) {
  return (
    <div className="plan-scores">
      {Object.entries(SCORE_LABEL).map(([key, label]) => (
        <span key={key} className="plan-score">{label} <strong>{(e as unknown as Record<string, number>)[key] ?? 0}</strong></span>
      ))}
      <span className="plan-score total">Skor <strong>{e.score ?? 0}</strong></span>
    </div>
  );
}

// The architecture style the design named; the user may rename it in
// their own words (there is no list of types). Folders change with the
// structure edits, never from the name.
function TypeRow({ s, type, editable, open, compact, onToggle, onDone }: {
  s: Structure; type: string; editable: boolean; open: boolean; compact?: boolean; onToggle: () => void; onDone: () => void;
}) {
  const recommended = s.recommended ?? "";
  return (
    <div className="tech-row">
      <div className="tech-head">
        <span className="tech-area">Gaya arsitektur</span>
        <strong>{type || "—"}</strong>
        {recommended && type !== recommended && <span className="tech-tag">pilihan kamu</span>}
        {editable && <button className="ghost small" onClick={onToggle}>Ubah</button>}
      </div>
      {!compact && (
        <div className="small muted">
          {recommended && type !== recommended ? <>Rekomendasi AI: {recommended}{s.reason && <> — {s.reason}</>}</> : <>Alasan: {s.reason || "—"}</>}
        </div>
      )}
      {open && <TypeEdit type={type} onDone={onDone} />}
    </div>
  );
}

function TypeEdit({ type, onDone }: { type: string; onDone: () => void }) {
  const { run, pending } = useProject();
  const [value, setValue] = useState(type);
  const save = async () => { if (await run("structure", () => api.proposeStructure([{ op: "type", name: value.trim() }]))) onDone(); };
  return (
    <div className="tech-edit">
      <input value={value} maxLength={120} placeholder="nama gaya arsitektur, mis. modular monolith heksagonal" onChange={(e) => setValue(e.target.value)} />
      <p className="small muted">Hanya nama gayanya. Untuk mengubah bentuknya, edit bagian struktur atau minta AI.</p>
      <div className="tech-actions">
        <button disabled={pending === "structure" || !value.trim() || value.trim() === type} onClick={() => void save()}>Lihat dampak</button>
        <button className="ghost" onClick={onDone}>Batal</button>
      </div>
    </div>
  );
}

// Why each part exists.
function Parts({ ir }: { ir: StructureIR }) {
  const parts = [...(ir.apps ?? []), ...(ir.services ?? []), ...(ir.workers ?? []), ...(ir.packages ?? [])];
  return (
    <details className="small">
      <summary>Tanggung jawab tiap folder</summary>
      <ul>{parts.map((p) => (
        <li key={p.path}>
          <code>{p.path}</code> {p.reason || <span className="muted">ditambahkan user</span>}
          {!!p.modules?.length && (
            <ul>{p.modules.map((m) => (
              <li key={m}><code>{modulePath(p, m)}</code> {p.module_reasons?.[m] || <span className="muted">tanggung jawab belum diisi</span>}</li>
            ))}</ul>
          )}
        </li>
      ))}</ul>
    </details>
  );
}

// Conventions inside the structure (layers, dependencies, naming); the
// scaffold and the build follow them like the folders.
function Rules({ rules, editable }: { rules: string[]; editable: boolean }) {
  const { run, pending } = useProject();
  const [text, setText] = useState("");
  const propose = (op: StructureOp) => run("structure", () => api.proposeStructure([op]));
  const add = async () => { if (await propose({ op: "add", kind: "rule", name: text.trim() })) setText(""); };
  if (!rules.length && !editable) return null;
  return (
    <details className="small" open={rules.length > 0}>
      <summary>Aturan struktur ({rules.length})</summary>
      {rules.length > 0
        ? <ol>{rules.map((r) => (
          <li key={r}>{r}{editable && (
            <> <button className="ghost small" disabled={pending === "structure"} onClick={() => void propose({ op: "remove", kind: "rule", name: r })}>Hapus</button></>
          )}</li>
        ))}</ol>
        : <p className="muted">Belum ada. Tulis standar di kotak prompt di bawah, atau tambah satu per satu.</p>}
      {editable && (
        <div className="tech-actions">
          <input value={text} maxLength={400} placeholder="mis. Dependensi antar-modul lewat public API atau event" onChange={(e) => setText(e.target.value)} />
          <button disabled={pending === "structure" || !text.trim()} onClick={() => void add()}>Tambah aturan</button>
        </div>
      )}
    </details>
  );
}

type Op = StructureOp["op"];
const OPS: [Op, string][] = [["add", "Tambah"], ["path", "Ganti folder"], ["rename", "Ganti nama"], ["move", "Pindahkan"], ["remove", "Hapus"]];

// One edit at a time; each becomes (part of) the waiting proposal.
function StructureEdit({ ir, onDone }: { ir: StructureIR; onDone: () => void }) {
  const { run, pending } = useProject();
  const [op, setOp] = useState<Op>("add");
  const [kind, setKind] = useState<PartKind>("module");
  const [name, setName] = useState("");
  const [to, setTo] = useState("");
  const [reason, setReason] = useState("");
  const [path, setPath] = useState("");
  const services = (ir.services ?? []).map((p) => p.name);
  const names: Record<PartKind, string[]> = {
    app: (ir.apps ?? []).map((p) => p.name), service: services,
    module: (ir.services ?? []).flatMap((p) => p.modules ?? []), worker: (ir.workers ?? []).map((p) => p.name),
    package: (ir.packages ?? []).map((p) => p.name),
  };
  const kinds: PartKind[] = op === "move" ? ["module", "service"] : ["app", "service", "module", "worker", "package"];
  const needsTo = op === "rename" || op === "move" || (op === "add" && kind === "module");
  const save = async () => {
    if (await run("structure", () => api.proposeStructure([{ op, kind, name: name.trim(), to: to.trim() || undefined, path: path.trim() || undefined, reason: reason.trim() || undefined }]))) {
      setName(""); setTo(""); setReason(""); setPath("");
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
      {(op === "add" || op === "path") && (
        <input value={path} maxLength={200} placeholder={op === "path" ? "folder baru, mis. server/" : "folder (opsional; kosong = di samping bagian sejenis)"}
          onChange={(e) => setPath(e.target.value)} />
      )}
      {op === "add" && <input value={reason} maxLength={200} placeholder="Alasan, mis. domain laporan terpisah" onChange={(e) => setReason(e.target.value)} />}
      <div className="tech-actions">
        <button disabled={pending === "structure" || !name.trim() || (op === "path" && !path.trim()) || (needsTo && !to.trim() && !(op === "add" && services.length === 1))}
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
            <strong>v{v.version}</strong> {v.ir.type} — {i === 0 ? "saat ini" : "sebelumnya"} · {new Date(v.at).toLocaleString("id-ID")}
            {" · "}{SOURCE[v.source] ?? v.source} · {TECH_STATUS[v.status] ?? v.status}
            {v.request && <div className="muted">“{v.request}”</div>}
            {!!v.changes?.length && <div className="muted">{v.changes.join("; ")}</div>}
          </li>
        ))}
      </ul>
    </details>
  );
}
