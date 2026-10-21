import { useEffect, useState } from "react";
import { api } from "../api";
import { isWorking } from "../flow";
import { useProject } from "../store/project";
import {
  changeText, DECISION_STATE, decisionState, fromExisting, needsApproval, NONE, TECH_AREA, TECH_SOURCE, TECH_STATUS,
  techChoice, techLang, techReview, techRuntime, techState, techVersion,
} from "../tech";
import type { Project, StackCatalog, TechArea, TechDecision, TechGroup, TechIssue, TechOption, TechProposal, TechStack } from "../types";

const NEED: Record<string, string> = {
  realtime: "realtime", offline: "offline", seo: "SEO", ai: "AI", high_concurrency: "konkurensi tinggi", low_latency: "latency rendah",
  flexible_schema: "skema fleksibel", transactions: "transaksi", independent_scaling: "skala independen", enterprise: "enterprise",
  files: "upload file", search: "pencarian teks", webhook: "webhook",
};
// These shape the project structure and must be a known option; the rest
// also accept the user's own choice.
const CLOSED = new Set<TechArea>(["cache", "architecture", "repository"]);
const SOURCE: Record<string, string> = { recommend: "rekomendasi", user: "diubah user", ai: "permintaan ke AI", structure: "struktur dipilih" };
// Used until the catalog (with the groups) has loaded.
const ALL: TechGroup[] = [{ id: "all", title: "Teknologi", description: "", areas: Object.keys(TECH_AREA) as TechArea[] }];

// Technology decision workspace, grouped by domain: the AI recommends, the
// user changes (by hand or in words), sees what follows and what it
// affects, applies and approves. A recommendation is never an approval.
export function TechStackPanel({ project }: { project: Project }) {
  const { run, pending } = useProject();
  const [cat, setCat] = useState<StackCatalog | null>(null);
  const [edit, setEdit] = useState<TechArea | null>(null);
  useEffect(() => { api.stackCatalog().then(setCat, () => {}); }, []);
  const s = project.stack;
  const busy = isWorking(project) || pending === "stack";
  const finalized = project.stage === "READY";
  if (!s) {
    return (
      <section className="info-sec">
        <h3>Teknologi</h3>
        <p className="small muted">Belum ada rekomendasi teknologi.</p>
        {project.platform && <button disabled={busy || finalized} onClick={() => void run("stack", api.recommendStack)}>Buat rekomendasi</button>}
      </section>
    );
  }
  const state = techState(s);
  const profile = cat?.profiles.find((p) => p.id === s.profile);
  const editable = !finalized && !busy && !s.request;
  const approval = needsApproval(s);
  const unused = s.decisions.filter((d) => techChoice(d) === NONE);
  const errors = (s.issues ?? []).filter((i) => i.level === "error");
  const reviews = s.decisions.filter((d) => d.review);
  const review = techReview(project);
  const row = (d: TechDecision) => (
    <TechRow key={d.area} s={s} d={d} editable={editable && !s.proposal} open={edit === d.area} options={cat?.catalog[d.area] ?? []}
      onToggle={() => setEdit(edit === d.area ? null : d.area)} onDone={() => setEdit(null)} />
  );
  return (
    <section className="info-sec">
      <div className="tech-title">
        <h3>Teknologi</h3>
        {approval && <span className={`tech-status ${state}`}>{TECH_STATUS[state]}{techVersion(s) ? ` · v${techVersion(s)}` : ""}</span>}
      </div>
      <p className="small muted">
        {s.mode === "existing"
          ? "Dibaca dari kode project yang ada. Teknologi yang sudah dipakai tetap dipakai; menggantinya adalah migrasi dan butuh alasan."
          : "Rekomendasi berdasarkan kebutuhan project — kamu yang memutuskan."}
        {profile && <> Profil <strong className="fg">{profile.name}</strong> hanya kebijakan rekomendasi untuk area yang tidak ditentukan kebutuhan: {profile.description}</>}
      </p>
      {s.needs?.length ? <p className="small">Kebutuhan teknis: <span className="theme-tags">{s.needs.map((n) => <i key={n}>{NEED[n] ?? n}</i>)}</span></p> : null}

      {!finalized && s.request && <p className="small muted"><span className="spinner" /> AI memahami permintaan: “{s.request}”</p>}

      {s.proposal && <Proposal p={s.proposal} disabled={busy || finalized} />}

      {review.length > 0 && <DesignReview project={project} />}

      {(cat?.groups ?? ALL).map((g) => {
        const ds = g.areas.map((a) => s.decisions.find((d) => d.area === a)).filter((d): d is TechDecision => !!d && techChoice(d) !== NONE);
        if (!ds.length) return null;
        return (
          <div key={g.id} className="tech-group">
            {g.id !== "all" && <h4>{g.title}</h4>}
            {g.description && <p className="small muted">{g.description}</p>}
            <div className="tech-list">{ds.map(row)}</div>
          </div>
        );
      })}
      {unused.length > 0 && (
        <details className="tech-unused">
          <summary className="small">Tidak dipakai — belum ada kebutuhannya ({unused.map((d) => TECH_AREA[d.area] ?? d.area).join(", ")})</summary>
          <div className="tech-list">{unused.map(row)}</div>
        </details>
      )}

      {!s.proposal && <Issues issues={s.issues} />}

      <div className="tech-foot">
        {editable && !s.proposal && <button className="ghost small" onClick={() => void run("stack", api.recommendStack)}>↻ Analisa ulang</button>}
        {!approval ? <span className="ok-text small">✓ Semua teknologi dibaca dari kode yang ada</span>
          : state === "approved" ? <span className="ok-text small">✓ Teknologi v{techVersion(s)} disetujui{finalized ? " dan masuk spesifikasi final" : ""}</span>
            : !finalized && (
              <button className="primary" disabled={!editable || !!s.proposal || errors.length > 0 || reviews.length > 0}
                title={s.proposal ? "Terapkan atau batalkan perubahan dulu" : reviews.length ? "Tinjau pilihan yang ditandai dulu"
                  : errors.length ? "Selesaikan konflik di atas dulu" : undefined}
                onClick={() => void run("stack", api.approveStack)}>Review & Approve</button>
            )}
      </div>
      {state === "changed" && <p className="small warn-text">Teknologi berubah setelah disetujui. Setujui lagi sebelum rancangan disetujui.</p>}

      <History s={s} />
    </section>
  );
}

function TechRow({ s, d, editable, open, options, onToggle, onDone }: {
  s: TechStack; d: TechDecision; editable: boolean; open: boolean; options: TechOption[]; onToggle: () => void; onDone: () => void;
}) {
  const { run, pending } = useProject();
  const st = decisionState(s, d);
  const attrs = [techLang(d), techRuntime(d) && `runtime ${techRuntime(d)}`].filter(Boolean).join(" · ");
  const alts = (d.alternatives ?? []).map((v) => options.find((o) => o.value === v) ?? { value: v, fit: "" });
  return (
    <div className="tech-row">
      <div className="tech-head">
        <span className="tech-area">{TECH_AREA[d.area] ?? d.area}</span>
        <strong>{techChoice(d)}</strong>
        {attrs && <span className="small muted">{attrs}</span>}
        <span className={`tech-state ${st}`}>{DECISION_STATE[st]}</span>
        {editable && <button className="ghost small" onClick={onToggle}>Ubah</button>}
      </div>
      {d.review && (
        <div className="small warn-text">
          Perlu ditinjau: {d.review}{" "}
          {editable && <button className="ghost small" disabled={pending === "stack"}
            onClick={() => void run("stack", () => api.confirmStackReview(d.area))}>Tetap pakai</button>}
        </div>
      )}
      <div className="small muted">
        {d.override ? <>Alasan kamu: {d.override.reason || "—"}<br />Sebelumnya: {d.recommended} — {d.reason}</> : <>Alasan: {d.reason}</>}
        {d.source && !d.override && <> <span className="tech-src">({TECH_SOURCE[d.source] ?? d.source})</span></>}
      </div>
      {!!d.evidence?.length && <div className="small muted">Bukti di kode: {d.evidence.map((e) => <code key={e}>{e}</code>)}</div>}
      {alts.length > 0 && (
        <details className="small tech-alts">
          <summary>Alternatif yang cocok ({alts.length})</summary>
          <ul>{alts.map((o) => <li key={o.value}><OptionInfo o={o} /></li>)}</ul>
        </details>
      )}
      {open && <TechEdit s={s} d={d} options={options} onDone={onDone} />}
    </div>
  );
}

// What an option is good for, gives up and asks from operations.
function OptionInfo({ o }: { o: TechOption }) {
  return (
    <>
      <strong>{o.value}</strong>{o.fit && ` — ${o.fit}`}
      {o.tradeoff && <div className="muted">Trade-off: {o.tradeoff}</div>}
      {o.ops && <div className="muted">Operasional: {o.ops}</div>}
    </>
  );
}

type Mode = "recommended" | "alternative" | "custom";

// Gunakan Rekomendasi / Pilih Alternatif / Tentukan Sendiri. Saving makes
// a proposal; nothing changes until it is applied. Alternatives are the
// options that fit the other choices; replacing code technology needs a
// reason.
function TechEdit({ s, d, options, onDone }: { s: TechStack; d: TechDecision; options: TechOption[]; onDone: () => void }) {
  const { run, pending } = useProject();
  const current = techChoice(d);
  const fit = d.alternatives?.length ? options.filter((o) => d.alternatives!.includes(o.value)) : [];
  const others = (fit.length ? fit : options.filter((o) => o.value !== NONE || !fromExisting(s, d)))
    .filter((o) => o.value !== d.recommended);
  const known = others.some((o) => o.value === current);
  const [mode, setMode] = useState<Mode>(!d.override ? (others.length ? "alternative" : "custom") : known ? "alternative" : "custom");
  const [pick, setPick] = useState(d.override && known ? current : others[0]?.value ?? "");
  const [custom, setCustom] = useState(d.override && !known ? current : "");
  const [reason, setReason] = useState(d.override?.reason ?? "");
  const migration = fromExisting(s, d) && mode !== "recommended";
  const value = mode === "recommended" ? "" : mode === "alternative" ? pick : custom.trim();
  const save = async () => { if (await run("stack", () => api.proposeStack([{ area: d.area, value, reason }]))) onDone(); };
  const radio = (m: Mode, label: string) => (
    <label className="check small"><input type="radio" checked={mode === m} onChange={() => setMode(m)} /> {label}</label>
  );
  const picked = others.find((o) => o.value === pick);
  return (
    <div className="tech-edit">
      <div className="tech-modes">
        {radio("recommended", fromExisting(s, d) ? `Tetap pakai yang ada (${d.recommended})` : `Gunakan rekomendasi (${d.recommended})`)}
        {others.length > 0 && radio("alternative", fit.length ? "Pilih alternatif yang cocok" : "Pilih dari katalog")}
        {!CLOSED.has(d.area) && radio("custom", "Tentukan sendiri")}
      </div>
      {mode === "alternative" && (
        <>
          <select value={pick} onChange={(e) => setPick(e.target.value)}>
            {others.map((o) => <option key={o.value} value={o.value}>{o.value} — {o.fit}</option>)}
          </select>
          {picked && (picked.tradeoff || picked.ops) && <div className="small"><OptionInfo o={picked} /></div>}
        </>
      )}
      {mode === "custom" && <input value={custom} maxLength={80} placeholder="mis. Rust + Axum" onChange={(e) => setCustom(e.target.value)} />}
      {mode !== "recommended" && (
        <input value={reason} maxLength={200} placeholder={migration ? "Alasan migrasi (wajib)" : "Alasan, mis. keahlian tim"}
          onChange={(e) => setReason(e.target.value)} />
      )}
      {migration && <p className="small warn-text">Teknologi ini sudah dipakai di kode; menggantinya berarti migrasi kode dan data.</p>}
      <div className="tech-actions">
        <button disabled={pending === "stack" || (mode !== "recommended" && !value) || (mode === "recommended" && !d.override)
          || (migration && !reason.trim())}
          onClick={() => void save()}>Lihat dampak</button>
        <button className="ghost" onClick={onDone}>Batal</button>
      </div>
    </div>
  );
}

// The waiting change: what the user asked, what follows from it, the
// choices to review, validation, and what it reaches in the blueprint.
function Proposal({ p, disabled }: { p: TechProposal; disabled: boolean }) {
  const { run, pending } = useProject();
  const design = (p.impact ?? []).some((i) => i.design);
  const own = p.changes.filter((c) => !c.follow), follow = p.changes.filter((c) => c.follow);
  const aff = p.affected;
  return (
    <div className="tech-proposal">
      <strong>⚠️ Potensi dampak{p.source === "ai" ? " — usulan AI" : ""}</strong>
      {p.request && <p className="small muted">Permintaan: “{p.request}”</p>}
      {p.note && <p className="small">{p.note}</p>}
      <ul className="small">{own.map((c) => <li key={c.area}><strong>{changeText(c)}</strong>{c.reason ? ` — ${c.reason}` : ""}</li>)}</ul>
      {follow.length > 0 && (
        <>
          <span className="small">Ikut berubah karena bergantung pada perubahan di atas:</span>
          <ul className="small">{follow.map((c) => <li key={c.area}>{changeText(c)}{c.reason ? ` — ${c.reason}` : ""}</li>)}</ul>
        </>
      )}
      {!!p.review?.length && (
        <>
          <span className="small warn-text">Pilihanmu yang perlu ditinjau setelah diterapkan (tidak diubah otomatis):</span>
          <ul className="small">{p.review.map((i, k) => <li key={k}>{i.area && <strong>{TECH_AREA[i.area] ?? i.area}: </strong>}{i.message}</li>)}</ul>
        </>
      )}
      <Issues issues={p.issues} ok="Kombinasi teknologi ini kompatibel dengan pilihan lain dan kebutuhan project." />
      {!!p.impact?.length && (
        <details className="small" open>
          <summary>Bagian yang terdampak</summary>
          <ul>{p.impact.map((i) => <li key={i.area}>{TECH_AREA[i.area] ?? i.area}: {i.affects.join(", ")}</li>)}</ul>
        </details>
      )}
      {aff && (aff.artifacts?.length || aff.commands?.length) ? (
        <p className="small">
          {aff.artifacts?.length ? <>Artefak blueprint yang berubah: {aff.artifacts.map((a) => <code key={a}>{a}</code>)}. </> : null}
          {aff.commands?.length ? <>Command yang akan berstatus <em>needs review</em>: {aff.commands.join(", ")}.</> : null}
        </p>
      ) : null}
      {design && <p className="small warn-text">Rancangan UI perlu diperiksa ulang setelah perubahan ini; rancangan tidak diubah otomatis.</p>}
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

// Technology changed after the UI design was written: the user checks the
// design rules and layout, Foundry does not change them.
export function DesignReview({ project }: { project: Project }) {
  const { run, pending } = useProject();
  const changes = techReview(project);
  if (!changes.length) return null;
  const affects = [...new Set((project.stack?.history ?? []).filter((v) => v.version > (project.ui?.tech_version ?? 0))
    .flatMap((v) => v.impact ?? []).filter((i) => i.design).flatMap((i) => i.affects))];
  return (
    <div className="tech-proposal">
      <strong>⚠️ Rancangan UI perlu diperiksa ulang</strong>
      <ul className="small">{changes.map((c, i) => <li key={i}>{changeText(c)}</li>)}</ul>
      {affects.length > 0 && <p className="small muted">Dampak: {affects.join(", ")}</p>}
      <p className="small muted">Aturan desain dan layout tidak diubah otomatis. Minta perubahan bila perlu, lalu tandai sudah diperiksa.</p>
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
