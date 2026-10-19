import { useEffect, useState } from "react";
import { api } from "../api";
import { isWorking } from "../flow";
import { useProject } from "../store/project";
import { changeText, NONE, TECH_AREA, TECH_STATUS, techChoice, techLang, techReview, techState, techVersion } from "../tech";
import type { Project, StackCatalog, TechArea, TechDecision, TechIssue, TechOption, TechProposal, TechStack } from "../types";

const NEED: Record<string, string> = {
  realtime: "realtime", offline: "offline", seo: "SEO", ai: "AI", high_concurrency: "konkurensi tinggi",
  flexible_schema: "skema fleksibel", transactions: "transaksi", independent_scaling: "skala independen", enterprise: "enterprise",
  files: "upload file", search: "pencarian teks",
};
// These shape the project structure and must be a known option; the rest
// also accept the user's own choice.
const CLOSED = new Set<TechArea>(["cache", "architecture", "repository"]);
const SOURCE: Record<string, string> = { recommend: "rekomendasi", user: "diubah user", ai: "permintaan ke AI", structure: "struktur dipilih" };

// Technology decision workspace: the AI recommends, the user changes (by
// hand or in words), sees the impact, applies and approves. Design and
// the final spec use the approved decision.
export function TechStackPanel({ project }: { project: Project }) {
  const { run, pending } = useProject();
  const [cat, setCat] = useState<StackCatalog | null>(null);
  const [edit, setEdit] = useState<TechArea | null>(null);
  const [ask, setAsk] = useState("");
  useEffect(() => { api.stackCatalog().then(setCat, () => {}); }, []);
  const s = project.stack;
  const busy = isWorking(project) || pending === "stack";
  const designApproved = project.design?.status === "approved";
  if (!s) {
    return (
      <section className="info-sec">
        <h3>Teknologi</h3>
        <p className="small muted">Belum ada rekomendasi teknologi.</p>
        {project.platform && <button disabled={busy || designApproved} onClick={() => void run("stack", api.recommendStack)}>Buat rekomendasi</button>}
      </section>
    );
  }
  const state = techState(s);
  const profile = cat?.profiles.find((p) => p.id === s.profile);
  const editable = s.mode === "new" && !designApproved && !busy && !s.request;
  const used = s.decisions.filter((d) => techChoice(d) !== NONE);
  const unused = s.decisions.filter((d) => techChoice(d) === NONE);
  const errors = (s.issues ?? []).filter((i) => i.level === "error");
  const review = techReview(project);
  const send = async () => { if (await run("stack", () => api.requestStack(ask))) setAsk(""); };
  const row = (d: TechDecision) => (
    <TechRow key={d.area} d={d} editable={editable && !s.proposal} open={edit === d.area} options={cat?.catalog[d.area] ?? []}
      onToggle={() => setEdit(edit === d.area ? null : d.area)} onDone={() => setEdit(null)} />
  );
  return (
    <section className="info-sec">
      <div className="tech-title">
        <h3>Teknologi</h3>
        {s.mode === "new" && <span className={`tech-status ${state}`}>{TECH_STATUS[state]}{techVersion(s) ? ` · v${techVersion(s)}` : ""}</span>}
      </div>
      <p className="small muted">
        {s.mode === "existing" ? "Dibaca dari kode project yang ada; tidak diganti."
          : <>Rekomendasi berdasarkan kebutuhan project — kamu yang memutuskan. Profil <strong className="fg">{profile?.name ?? s.profile}</strong>{profile ? `: ${profile.description}` : ""}</>}
      </p>
      {s.needs?.length ? <p className="small">Kebutuhan teknis: <span className="theme-tags">{s.needs.map((n) => <i key={n}>{NEED[n] ?? n}</i>)}</span></p> : null}

      {s.mode === "new" && !designApproved && (
        <div className="tech-ask">
          <textarea rows={2} maxLength={1000} value={ask} disabled={!editable || !!s.proposal}
            placeholder="mis. Ganti backend ke Rust. Tetap gunakan PostgreSQL dan Redis. Arsitektur modular monolith."
            onChange={(e) => setAsk(e.target.value)} />
          <button disabled={!editable || !!s.proposal || !ask.trim()} onClick={() => void send()}>✦ Minta Perubahan dengan AI</button>
          {s.request && <p className="small muted"><span className="spinner" /> AI memahami permintaan: “{s.request}”</p>}
        </div>
      )}

      {s.proposal && <Proposal p={s.proposal} disabled={busy || designApproved} />}

      {review.length > 0 && <DesignReview project={project} />}

      <div className="tech-list">{used.map(row)}</div>
      {unused.length > 0 && (
        <details className="tech-unused">
          <summary className="small">Tidak dipakai ({unused.map((d) => TECH_AREA[d.area] ?? d.area).join(", ")})</summary>
          <div className="tech-list">{unused.map(row)}</div>
        </details>
      )}

      {!s.proposal && <Issues issues={s.issues} />}

      {s.mode === "new" && (
        <div className="tech-foot">
          {editable && !s.proposal && <button className="ghost small" onClick={() => void run("stack", api.recommendStack)}>↻ Analisa ulang</button>}
          {state === "approved" ? <span className="ok-text small">✓ Teknologi v{techVersion(s)} disetujui{designApproved ? " dan masuk spesifikasi final" : ""}</span>
            : !designApproved && (
              <button className="primary" disabled={!editable || !!s.proposal || errors.length > 0}
                title={s.proposal ? "Terapkan atau batalkan perubahan dulu" : errors.length ? "Selesaikan masalah di atas dulu" : undefined}
                onClick={() => void run("stack", api.approveStack)}>Review & Approve</button>
            )}
        </div>
      )}
      {state === "changed" && <p className="small warn-text">Teknologi berubah setelah disetujui. Setujui lagi sebelum rancangan disetujui.</p>}

      <History s={s} />
    </section>
  );
}

function TechRow({ d, editable, open, options, onToggle, onDone }: {
  d: TechDecision; editable: boolean; open: boolean; options: TechOption[]; onToggle: () => void; onDone: () => void;
}) {
  return (
    <div className="tech-row">
      <div className="tech-head">
        <span className="tech-area">{TECH_AREA[d.area] ?? d.area}</span>
        <strong>{techChoice(d)}</strong>
        {techLang(d) && <span className="small muted">{techLang(d)}</span>}
        {d.override && <span className="tech-tag">pilihan kamu</span>}
        {editable && <button className="ghost small" onClick={onToggle}>Ubah</button>}
      </div>
      <div className="small muted">
        {d.override ? <>Alasan kamu: {d.override.reason || "—"}<br />Rekomendasi: {d.recommended} — {d.reason}</> : <>Alasan: {d.reason}</>}
      </div>
      {!d.override && !!d.alternatives?.length && <div className="small muted">Alternatif: {d.alternatives.join(" · ")}</div>}
      {open && <TechEdit d={d} options={options} onDone={onDone} />}
    </div>
  );
}

type Mode = "recommended" | "alternative" | "custom";

// Gunakan Rekomendasi / Pilih Alternatif / Tentukan Sendiri. Saving makes
// a proposal; nothing changes until it is applied.
function TechEdit({ d, options, onDone }: { d: TechDecision; options: TechOption[]; onDone: () => void }) {
  const { run, pending } = useProject();
  const current = techChoice(d);
  const others = options.filter((o) => o.value !== d.recommended);
  const known = options.some((o) => o.value === current);
  const [mode, setMode] = useState<Mode>(!d.override ? "alternative" : known ? "alternative" : "custom");
  const [pick, setPick] = useState(d.override && known ? current : others[0]?.value ?? "");
  const [custom, setCustom] = useState(d.override && !known ? current : "");
  const [reason, setReason] = useState(d.override?.reason ?? "");
  const value = mode === "recommended" ? "" : mode === "alternative" ? pick : custom.trim();
  const save = async () => { if (await run("stack", () => api.proposeStack([{ area: d.area, value, reason }]))) onDone(); };
  const radio = (m: Mode, label: string) => (
    <label className="check small"><input type="radio" checked={mode === m} onChange={() => setMode(m)} /> {label}</label>
  );
  return (
    <div className="tech-edit">
      <div className="tech-modes">
        {radio("recommended", `Gunakan rekomendasi (${d.recommended})`)}
        {others.length > 0 && radio("alternative", "Pilih alternatif")}
        {!CLOSED.has(d.area) && radio("custom", "Tentukan sendiri")}
      </div>
      {mode === "alternative" && (
        <select value={pick} onChange={(e) => setPick(e.target.value)}>
          {others.map((o) => <option key={o.value} value={o.value}>{o.value} — {o.fit}</option>)}
        </select>
      )}
      {mode === "custom" && <input value={custom} maxLength={80} placeholder="mis. Rust + Axum" onChange={(e) => setCustom(e.target.value)} />}
      {mode !== "recommended" && (
        <input value={reason} maxLength={200} placeholder="Alasan, mis. keahlian tim" onChange={(e) => setReason(e.target.value)} />
      )}
      <div className="tech-actions">
        <button disabled={pending === "stack" || (mode !== "recommended" && !value) || (mode === "recommended" && !d.override)}
          onClick={() => void save()}>Lihat dampak</button>
        <button className="ghost" onClick={onDone}>Batal</button>
      </div>
    </div>
  );
}

// The waiting change with its validation and impact; the user decides.
function Proposal({ p, disabled }: { p: TechProposal; disabled: boolean }) {
  const { run, pending } = useProject();
  const design = (p.impact ?? []).some((i) => i.design);
  return (
    <div className="tech-proposal">
      <strong>⚠️ Potensi dampak{p.source === "ai" ? " — usulan AI" : ""}</strong>
      {p.request && <p className="small muted">Permintaan: “{p.request}”</p>}
      {p.note && <p className="small">{p.note}</p>}
      <ul className="small">{p.changes.map((c) => <li key={c.area}><strong>{changeText(c)}</strong>{c.reason ? ` — ${c.reason}` : ""}</li>)}</ul>
      <Issues issues={p.issues} ok="Kombinasi teknologi ini kompatibel dengan pilihan lain dan kebutuhan project." />
      {!!p.impact?.length && (
        <details className="small" open>
          <summary>Bagian yang terdampak</summary>
          <ul>{p.impact.map((i) => <li key={i.area}>{TECH_AREA[i.area] ?? i.area}: {i.affects.join(", ")}</li>)}</ul>
        </details>
      )}
      {design && <p className="small warn-text">Rancangan perlu diperiksa ulang setelah perubahan ini; rancangan tidak diubah otomatis.</p>}
      <div className="tech-actions">
        <button className="primary" disabled={disabled || pending === "stack"} onClick={() => void run("stack", api.applyStack)}>Terapkan Perubahan</button>
        <button className="ghost" disabled={disabled || pending === "stack"} onClick={() => void run("stack", api.discardStack)}>Batalkan</button>
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
          {i.level === "error" ? "✕" : "!"} {i.area && <strong>{TECH_AREA[i.area] ?? i.area}: </strong>}{i.message}
        </li>
      ))}
    </ul>
  );
}

// Technology changed after the design was made: the user checks the design,
// Foundry does not change it.
export function DesignReview({ project }: { project: Project }) {
  const { run, pending } = useProject();
  const changes = techReview(project);
  if (!changes.length) return null;
  const affects = [...new Set((project.stack?.history ?? []).filter((v) => v.version > (project.design?.tech_version ?? 0))
    .flatMap((v) => v.impact ?? []).filter((i) => i.design).flatMap((i) => i.affects))];
  return (
    <div className="tech-proposal">
      <strong>⚠️ Rancangan perlu diperiksa ulang</strong>
      <ul className="small">{changes.map((c, i) => <li key={i}>{changeText(c)}</li>)}</ul>
      {affects.length > 0 && <p className="small muted">Dampak: {affects.join(", ")}</p>}
      <p className="small muted">Rancangan tidak diubah otomatis. Minta perubahan rancangan bila perlu, lalu tandai sudah diperiksa.</p>
      <div className="tech-actions">
        <button disabled={isWorking(project) || pending === "stack"} onClick={() => void run("stack", api.reviewedDesignTech)}>Sudah saya periksa</button>
      </div>
    </div>
  );
}

function History({ s }: { s: TechStack }) {
  const versions = [...(s.history ?? [])].reverse();
  if (versions.length < 2) return null;
  return (
    <details className="tech-history small">
      <summary>Riwayat teknologi ({versions.length} versi)</summary>
      <ul>
        {versions.map((v, i) => (
          <li key={v.version}>
            <strong>v{v.version}</strong> — {i === 0 ? "saat ini" : "sebelumnya"} · {new Date(v.at).toLocaleString("id-ID")} · {SOURCE[v.source] ?? v.source}
            {" · "}{TECH_STATUS[v.status] ?? v.status}
            {v.request && <div className="muted">“{v.request}”</div>}
            {!!v.changes?.length && <div className="muted">{v.changes.map(changeText).join("; ")}</div>}
          </li>
        ))}
      </ul>
    </details>
  );
}
