import { useEffect, useState } from "react";
import { api } from "../api";
import { isWorking } from "../flow";
import {
  allChecks, allCompat, allFindings, areaLabel, groupItems, isNone, issueCounts, itemsOf, PLAN_CHECK, PLAN_CRITERIA,
  PLAN_DRIVER, PLAN_FINDING, PLAN_SECTION, PLAN_SOURCE, PLAN_TABS, planGate, planSummary, type PlanTab,
} from "../techplan";
import { DECISION_STATE, decisionState, needsApproval, TECH_AREA, TECH_STATUS, techState, techVersion } from "../tech";
import { useProject } from "../store/project";
import type { Project, StackCatalog, TechArea, TechCompat, TechItem, TechPlan } from "../types";
import { Elapsed } from "./Elapsed";
import { Proposal, TechRow } from "./TechStack";

// Technology plan workspace: 12 tabs of the AI's decisions, reasoning,
// alternatives, compatibility and evaluation. Changing a core area still
// goes through the existing stack proposal.

export function TechWorkspace({ project }: { project: Project }) {
  const { pending } = useProject();
  const [tab, setTab] = useState<PlanTab>("overview");
  const [selected, setSelected] = useState("");
  const [cat, setCat] = useState<StackCatalog | null>(null);
  useEffect(() => { api.stackCatalog().then(setCat, () => {}); }, []);
  const s = project.stack;
  const plan = s?.plan;
  const finalized = project.stage === "READY";
  const busy = isWorking(project) || pending === "stack" || !!s?.request || !!s?.plan_request;
  const editable = !finalized && !busy && !s?.proposal;

  if (!s || !plan) return null;

  const counts = issueCounts(plan);
  const summary = planSummary(plan);
  const affected = new Set(plan.analysis?.affected ?? []);
  const working = !!s.plan_request && (
    <p className="small muted"><span className="spinner" /> {s.plan_request.action === "generate" ? "AI merancang teknologi…" : "AI merevisi rencana teknologi…"} <Elapsed started={project.job?.started} /></p>
  );

  return (
    <div className="repo tp">
      <header className="repo-head">
        <div>
          <h2>Rencana teknologi</h2>
          <p className="small muted">{plan.approach} · {summary.items} keputusan · skor {summary.score}</p>
        </div>
        <div className="repo-chips">
          <span className={`tech-status ${techState(s)}`}>{TECH_STATUS[techState(s)]}{techVersion(s) ? ` · v${techVersion(s)}` : ""}</span>
          {summary.outside > 0 && <span className="tag">Di luar katalog: {summary.outside}</span>}
          {plan.stale && <span className="tag warn">Kedaluwarsa</span>}
          {counts.errors > 0 && <span className="tag bad">{counts.errors} error</span>}
        </div>
      </header>

      <nav className="repo-tabs" role="tablist" aria-label="Bagian rencana teknologi">
        {PLAN_TABS.map((t) => (
          <button key={t} role="tab" aria-selected={tab === t} className={tab === t ? "on" : ""} onClick={() => setTab(t)}
            title={affected.has(t) ? "Terdampak perubahan sumber; perlu ditinjau" : PLAN_SECTION[t]}>
            {PLAN_SECTION[t]}{affected.has(t) && <span className="repo-dot" aria-label="perlu ditinjau" />}
          </button>
        ))}
      </nav>

      {working}
      {s.plan_request?.rejected && <p className="small warn-text">Jawaban AI ditolak, mencoba lagi: {s.plan_request.rejected}</p>}
      {s.proposal && <Proposal p={s.proposal} disabled={busy || finalized} />}

      <div className="repo-body">
        {tab === "context" && <ContextTab plan={plan} />}
        {tab === "overview" && <OverviewTab plan={plan} project={project} editable={editable} />}
        {tab === "compatibility" && <CompatTab plan={plan} />}
        {tab === "alternatives" && <AlternativesTab plan={plan} onOpen={(a) => { setSelected(a); setTab(groupTab(plan, a)); }} />}
        {tab === "reasoning" && <ReasoningTab plan={plan} />}
        {tab === "evaluation" && <EvaluationTab plan={plan} />}
        {tab === "history" && <HistoryTab plan={plan} />}
        {(tab === "stack" || tab === "data" || tab === "infra" || tab === "architecture" || tab === "standards") && (
          <DecisionsTab plan={plan} tab={tab} project={project} catalog={cat} selected={selected} onSelect={setSelected} editable={editable} />
        )}
      </div>

      {editable && <AskAI plan={plan} />}
    </div>
  );
}

function groupTab(plan: TechPlan, area: string): PlanTab {
  const g = plan.items.find((i) => i.area === area)?.group;
  if (g === "data") return "data";
  if (g === "architecture") return "architecture";
  if (g === "standards") return "standards";
  if (g === "app_infra" || g === "operations") return "infra";
  return "stack";
}

function ContextTab({ plan }: { plan: TechPlan }) {
  const c = plan.context ?? {};
  const facts = c.facts ?? [];
  return (
    <div className="repo-grid">
      <section className="repo-card wide">
        <h3>Konteks teknis</h3>
        <p>{c.summary || "—"}</p>
        {facts.length > 0 && (
          <ul className="small">{facts.map((f, i) => <li key={i}><strong>{f.category}</strong>: {f.statement}{f.source ? <span className="muted"> ({f.source})</span> : null}</li>)}</ul>
        )}
        {!!c.unknowns?.length && <><h3>Belum jelas</h3><ul className="small">{c.unknowns.map((u) => <li key={u}>{u}</li>)}</ul></>}
        {!!c.assumptions?.length && <><h3>Asumsi</h3><ul className="small">{c.assumptions.map((u) => <li key={u}>{u}</li>)}</ul></>}
      </section>
    </div>
  );
}

function OverviewTab({ plan, project, editable }: { plan: TechPlan; project: Project; editable: boolean }) {
  const { run, pending } = useProject();
  const [confirm, setConfirm] = useState(false);
  const [again, setAgain] = useState("");
  const a = plan.analysis;
  const summary = planSummary(plan);
  const counts = issueCounts(plan);
  const s = project.stack!;
  const state = techState(s);
  const approval = needsApproval(s);
  const gate = planGate(project);
  const errors = (s.issues ?? []).filter((i) => i.level === "error");
  const reviews = s.decisions.filter((d) => d.review);
  return (
    <div className="repo-grid">
      {plan.stale && (
        <div className="repo-banner warn">
          <strong>Sumber berubah setelah rencana ditulis</strong>
          <div className="small">Area: {(a.stale_areas ?? []).map((x) => PLAN_SOURCE[x] ?? x).join(", ")}. Bagian terdampak: {(a.affected ?? []).map((x) => PLAN_SECTION[x] ?? x).join(", ")}.</div>
          {editable && <button disabled={pending === "stack"} onClick={() => void run("stack", () => api.reviseTechPlan())}>Revisi bagian terdampak dengan AI</button>}
        </div>
      )}
      <section className="repo-card wide">
        <h3>Pendekatan</h3><p><strong>{plan.approach}</strong></p>
        <h3>Ringkasan</h3><p>{plan.summary}</p>
        {plan.rationale && <><h3>Alasan</h3><p>{plan.rationale}</p></>}
      </section>
      <section className="repo-card wide">
        <h3>Ukuran</h3>
        <dl className="repo-stats">
          <div><dt>Keputusan</dt><dd>{summary.items}</dd></div>
          <div><dt>Di luar katalog</dt><dd>{summary.outside}</dd></div>
          <div><dt>Versi belum dicek</dt><dd>{summary.unverified}</dd></div>
          <div><dt>Skor</dt><dd>{summary.score}</dd></div>
        </dl>
        <h3>Validasi konsistensi</h3>
        <ul className="repo-checks small">
          {allChecks(plan).map((c) => (
            <li key={`${c.area}-${c.source}`} className={c.status}>
              <strong>{PLAN_CHECK[c.area] ?? c.area}</strong> <span className="tag">{c.status}</span> <span className="muted">{c.source === "computed" ? "diverifikasi Foundry" : "laporan AI"}</span>
              {c.note && <div className="muted">{c.note}</div>}
            </li>
          ))}
        </ul>
      </section>
      <section className="repo-card wide">
        <h3>Temuan validasi {counts.errors + counts.warnings > 0 && <span className="muted small">({counts.errors} error, {counts.warnings} peringatan)</span>}</h3>
        {(a.issues ?? []).length === 0 ? <p className="ok-text small">✓ Rencana konsisten dengan kebutuhan dan artefak lain.</p> : (
          <ul className="tech-issues small">{(a.issues ?? []).map((i, k) => <li key={k} className={i.level}>{i.level === "error" ? "✕" : "!"} {i.area && <strong>{TECH_AREA[i.area as TechArea] ?? i.area}: </strong>}{i.message}</li>)}</ul>
        )}
      </section>
      <section className="repo-card wide">
        {!approval ? <span className="ok-text small">✓ Semua teknologi dibaca dari kode yang ada</span>
          : state === "approved" ? <p className="ok-text small">✓ Teknologi v{techVersion(s)} disetujui{project.stage === "READY" ? " dan masuk spesifikasi final" : ""}</p>
            : editable && (
              <div className="repo-actions">
                <button className="primary" disabled={!editable || !!s.proposal || errors.length > 0 || reviews.length > 0 || !!gate}
                  title={gate ?? (s.proposal ? "Terapkan atau batalkan perubahan dulu" : reviews.length ? "Tinjau pilihan yang ditandai dulu" : errors.length ? "Selesaikan konflik dulu" : undefined)}
                  onClick={() => void run("stack", api.approveStack)}>Review & Approve</button>
                {gate && <span className="small warn-text">{gate}</span>}
              </div>
            )}
        {state === "changed" && <p className="small warn-text">Teknologi berubah setelah disetujui. Setujui lagi sebelum rancangan disetujui.</p>}
        {editable && (
          <div className="repo-actions">
            <input value={again} maxLength={6000} placeholder="Rancang ulang dari awal dengan arahan, mis. “pakai Go di backend, tetap PostgreSQL”" onChange={(e) => setAgain(e.target.value)} />
            <button disabled={pending === "stack" || !again.trim()} onClick={async () => { if (await run("stack", () => api.generateTechPlan(again.trim()))) setAgain(""); }}>Rancang ulang</button>
            {confirm
              ? <><button className="bad" disabled={pending === "stack"} onClick={() => void run("stack", api.discardTechPlan)}>Ya, hapus rencana</button><button className="ghost" onClick={() => setConfirm(false)}>Batal</button></>
              : <button className="ghost" onClick={() => setConfirm(true)}>Hapus rencana</button>}
          </div>
        )}
      </section>
      <section className="repo-card wide">
        <h3>Keputusan per grup</h3>
        {groupItems(plan).map((g) => (
          <p key={g.id} className="small"><strong>{g.title}:</strong> {g.items.map((it) => `${areaLabel(it.area)} ${it.technology}`).join(" · ")}</p>
        ))}
      </section>
    </div>
  );
}

function DecisionsTab({ plan, tab, project, catalog, selected, onSelect, editable }: {
  plan: TechPlan; tab: PlanTab; project: Project; catalog: StackCatalog | null; selected: string; onSelect: (a: string) => void; editable: boolean;
}) {
  const items = itemsOf(plan, tab);
  const it = items.find((i) => i.area === selected) ?? items[0];
  const s = project.stack!;
  return (
    <div className="repo-split">
      <div className="tp-list">
        {items.length === 0 ? <p className="muted small">Tidak ada keputusan di bagian ini.</p> : items.map((i) => (
          <button key={i.area} className={`tp-item${it?.area === i.area ? " on" : ""}`} onClick={() => onSelect(i.area)}>
            <span className="tech-area">{TECH_AREA[i.area as TechArea] ?? areaLabel(i.area)}</span>
            <strong>{i.technology}</strong>
            <ItemBadges it={i} />
          </button>
        ))}
      </div>
      {it && <ItemDetail key={it.area} it={it} plan={plan} stack={s} catalog={catalog} editable={editable} />}
    </div>
  );
}

function ItemBadges({ it }: { it: TechItem }) {
  return (
    <span className="tp-badges">
      {!isNone(it) && !it.in_catalog && <span className="tag">di luar katalog</span>}
      {it.user_chosen && <span className="tag">dipilih user</span>}
      {it.version && !it.version_verified && <span className="tag warn">versi belum terverifikasi</span>}
    </span>
  );
}

function ItemDetail({ it, plan, stack, catalog, editable }: {
  it: TechItem; plan: TechPlan; stack: NonNullable<Project["stack"]>; catalog: StackCatalog | null; editable: boolean;
}) {
  const [open, setOpen] = useState(false);
  const d = stack.decisions.find((x) => x.area === it.area);
  const related = allCompat(plan).filter((c) => (c.between ?? []).includes(it.area) || c.area === it.area);
  return (
    <div className="repo-detail">
      <div className="repo-node-head">
        <h2>{TECH_AREA[it.area as TechArea] ?? areaLabel(it.area)} — {it.technology}</h2>
        {d && <span className={`tech-state ${decisionState(stack, d)}`}>{DECISION_STATE[decisionState(stack, d)]}</span>}
        <ItemBadges it={it} />
      </div>
      {isNone(it) ? <p>{it.reasoning}</p> : (
        <dl className="repo-fields">
          <dt>Tujuan</dt><dd>{it.purpose || "—"}</dd>
          {!!it.fulfills?.length && <><dt>Kebutuhan dipenuhi</dt><dd>{it.fulfills.join(", ")}</dd></>}
          <dt>Driver</dt><dd>{PLAN_DRIVER[it.driver] ?? it.driver}</dd>
          <dt>Alasan</dt><dd>{it.reasoning}</dd>
          {!!it.pros?.length && <><dt>Kelebihan</dt><dd><ul>{it.pros.map((x) => <li key={x}>{x}</li>)}</ul></dd></>}
          {!!it.cons?.length && <><dt>Kekurangan</dt><dd><ul>{it.cons.map((x) => <li key={x}>{x}</li>)}</ul></dd></>}
          {!!it.tradeoffs?.length && <><dt>Trade-off</dt><dd><ul>{it.tradeoffs.map((x) => <li key={x}>{x}</li>)}</ul></dd></>}
          {!!it.depends_on?.length && <><dt>Terintegrasi dengan</dt><dd>{it.depends_on.map((a) => TECH_AREA[a as TechArea] ?? areaLabel(a)).join(", ")}</dd></>}
          {it.integration && <><dt>Integrasi</dt><dd>{it.integration}</dd></>}
          {it.operational && <><dt>Operasional</dt><dd>{it.operational}</dd></>}
          {it.version && <><dt>Versi</dt><dd>{it.version}{it.version_verified ? "" : " (belum terverifikasi)"}</dd></>}
          {it.language && <><dt>Bahasa</dt><dd>{it.language}</dd></>}
          {it.runtime && <><dt>Runtime</dt><dd>{it.runtime}</dd></>}
          <dt>Kepercayaan</dt><dd>{it.confidence}</dd>
          {it.user_chosen && <><dt>Dipilih user</dt><dd>{it.user_chosen}</dd></>}
        </dl>
      )}
      {!!it.alternatives?.length && (
        <><h3>Alternatif</h3><ul className="small">{it.alternatives.map((a) => (
          <li key={a.technology}><strong>{a.technology}</strong> — {a.why} Beda: {a.difference}
            {a.pros?.length ? ` Kelebihan: ${a.pros.join("; ")}.` : ""}{a.cons?.length ? ` Kekurangan: ${a.cons.join("; ")}.` : ""}</li>
        ))}</ul></>
      )}
      {related.length > 0 && (
        <><h3>Kompatibilitas</h3><CompatTable rows={related} /></>
      )}
      {d && editable && !isNone(it) && (
        <div className="tp-edit">
          <TechRow s={stack} d={d} editable={editable} open={open} options={catalog?.catalog[d.area] ?? []}
            onToggle={() => setOpen(!open)} onDone={() => setOpen(false)} />
        </div>
      )}
    </div>
  );
}

function CompatTable({ rows }: { rows: TechCompat[] }) {
  return (
    <table className="repo-table">
      <thead><tr><th>Area</th><th>Status</th><th>Antar</th><th>Catatan</th><th>Dasar</th></tr></thead>
      <tbody>
        {rows.map((x, i) => (
          <tr key={`${x.area}-${x.source}-${i}`}>
            <td>{x.area}</td>
            <td>{x.status}{x.source === "computed" ? " (Foundry)" : ""}</td>
            <td>{(x.between ?? []).join(", ")}</td>
            <td>{x.note || "—"}</td>
            <td>{x.basis || "—"}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function CompatTab({ plan }: { plan: TechPlan }) {
  const ai = plan.compatibility ?? [];
  const computed = plan.analysis?.compat ?? [];
  return (
    <div className="repo-grid">
      <section className="repo-card wide">
        <h3>Tinjauan AI</h3>
        {ai.length ? <CompatTable rows={ai} /> : <p className="muted small">Belum ada tinjauan kompatibilitas AI.</p>}
        {!!plan.constraints?.length && (
          <><h3>Constraint</h3><ul className="small">{plan.constraints.map((k, i) => <li key={i}>[{k.areas.join(", ")}] {k.rule}{k.reason ? ` — ${k.reason}` : ""}</li>)}</ul></>
        )}
      </section>
      <section className="repo-card wide">
        <h3>Hasil hitungan Foundry</h3>
        {computed.length ? <CompatTable rows={computed} /> : <p className="ok-text small">✓ Foundry tidak menemukan konflik kompatibilitas tambahan.</p>}
      </section>
    </div>
  );
}

function AlternativesTab({ plan, onOpen }: { plan: TechPlan; onOpen: (area: string) => void }) {
  const withAlts = plan.items.filter((it) => (it.alternatives ?? []).length > 0);
  return (
    <div className="repo-grid">
      {withAlts.length === 0 ? <section className="repo-card wide"><p className="muted">Tidak ada alternatif yang ditimbang.</p></section> : withAlts.map((it) => (
        <section key={it.area} className="repo-card">
          <h3><button className="link" onClick={() => onOpen(it.area)}>{TECH_AREA[it.area as TechArea] ?? areaLabel(it.area)}</button> — {it.technology}</h3>
          <ul className="small">{(it.alternatives ?? []).map((a) => (
            <li key={a.technology}><strong>{a.technology}</strong> — {a.why}<div className="muted">Beda: {a.difference}</div></li>
          ))}</ul>
        </section>
      ))}
    </div>
  );
}

function ReasoningTab({ plan }: { plan: TechPlan }) {
  return (
    <div className="repo-grid">
      {plan.items.map((it) => (
        <section key={it.area} className="repo-card">
          <h3>{TECH_AREA[it.area as TechArea] ?? areaLabel(it.area)} — {it.technology}</h3>
          <p className="small muted">Driver: {PLAN_DRIVER[it.driver] ?? it.driver} · {it.confidence}</p>
          <p>{it.reasoning}</p>
          {!!it.fulfills?.length && <p className="small">Memenuhi: {it.fulfills.join(", ")}</p>}
        </section>
      ))}
    </div>
  );
}

function EvaluationTab({ plan }: { plan: TechPlan }) {
  const findings = allFindings(plan);
  const diverged = plan.analysis?.diverged ?? [];
  return (
    <div className="repo-grid">
      {diverged.length > 0 && (
        <div className="repo-banner warn">
          <strong>Evaluasi belum mencerminkan pilihan user</strong>
          <div className="small">Area yang berubah setelah rencana ditulis: {diverged.map((a) => TECH_AREA[a as TechArea] ?? areaLabel(a)).join(", ")}.</div>
        </div>
      )}
      {plan.evaluation.outdated && <div className="repo-banner warn"><strong>Evaluasi kedaluwarsa.</strong> Minta AI menilai ulang.</div>}
      <section className="repo-card wide">
        <h3>Skor {plan.evaluation.score}/100</h3>
        {plan.evaluation.summary && <p>{plan.evaluation.summary}</p>}
        <table className="repo-table">
          <thead><tr><th>Kriteria</th><th>Skor</th><th>Alasan</th><th>Evidence</th></tr></thead>
          <tbody>
            {(plan.evaluation.criteria ?? []).map((c) => (
              <tr key={c.name}><td>{PLAN_CRITERIA[c.name] ?? c.name}</td><td>{c.score}/5</td><td>{c.reason}</td><td>{(c.evidence ?? []).join(", ") || "—"}</td></tr>
            ))}
          </tbody>
        </table>
      </section>
      <section className="repo-card wide">
        <h3>Temuan</h3>
        {findings.length === 0 ? <p className="muted small">Tidak ada temuan.</p> : (
          <ul className="repo-findings">
            {findings.map((f, i) => (
              <li key={i} className={f.severity}>
                <strong>{PLAN_FINDING[f.category] ?? f.category}</strong> <span className="tag">{f.severity}</span> <span className="muted">{f.source === "computed" ? "Foundry" : "AI"}</span>
                {f.area && <code>{f.area}</code>}
                <div>{f.message}</div>
                {f.resolution && <div className="muted">→ {f.resolution}</div>}
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}

function HistoryTab({ plan }: { plan: TechPlan }) {
  const versions = [...(plan.history ?? [])].reverse();
  if (!versions.length) return <p className="muted">Belum ada riwayat revisi.</p>;
  return (
    <ol className="repo-history">
      {versions.map((v) => (
        <li key={v.version}>
          <strong>v{v.version}</strong> · {new Date(v.at).toLocaleString("id-ID")} · {v.source} · {TECH_STATUS[v.status] ?? v.status} · {v.items} keputusan · skor {v.score}
          {v.request && <div className="muted">“{v.request}”</div>}
          {v.reason && <div className="muted">{v.reason}</div>}
          {!!v.changes?.length && <ul className="small">{v.changes.map((c) => <li key={c}>{c}</li>)}</ul>}
        </li>
      ))}
    </ol>
  );
}

function AskAI({ plan }: { plan: TechPlan }) {
  const { run, pending } = useProject();
  const [text, setText] = useState("");
  const [why, setWhy] = useState("");
  const stale = plan.stale || plan.evaluation.outdated;
  return (
    <section className="repo-ask">
      <strong>Minta AI merevisi</strong>
      <textarea rows={2} value={text} maxLength={6000} placeholder="mis. ganti cache ke Valkey; atau jelaskan ulang alasan memilih Elixir"
        onChange={(e) => setText(e.target.value)} />
      <div className="repo-actions">
        <input value={why} maxLength={600} placeholder="Alasan (opsional, dicatat di riwayat)" onChange={(e) => setWhy(e.target.value)} />
        <button className="primary" disabled={pending === "stack" || !text.trim()}
          onClick={async () => { if (await run("stack", () => api.reviseTechPlan(text.trim(), why.trim()))) { setText(""); setWhy(""); } }}>Revisi</button>
        {stale && (
          <button disabled={pending === "stack"} title="Sesuaikan dengan sumber yang berubah dan evaluasi ulang"
            onClick={() => void run("stack", () => api.reviseTechPlan())}>{plan.stale ? "Revisi bagian terdampak" : "Evaluasi ulang"}</button>
        )}
      </div>
    </section>
  );
}
