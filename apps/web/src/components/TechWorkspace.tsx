import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { api } from "../api";
import { isWorking } from "../flow";
import {
  allCompat, areaLabel, groupItems, isCoreGroup, isNone, issueCounts, PLAN_SECTION, PLAN_SOURCE, planGate, planNotes,
} from "../techplan";
import { needsApproval, TECH_AREA, TECH_STATUS, techState, techVersion } from "../tech";
import { useProject } from "../store/project";
import type { Project, StackCatalog, TechArea, TechItem, TechPlan } from "../types";
import { Elapsed } from "./Elapsed";
import { DesignReview, Proposal, TechRow } from "./TechStack";

// Technology plan page: the approach, what needs attention, the chosen
// technologies and the approval. Reasoning, evaluation, compatibility and
// history live in the full "Rencana Teknologi" document. Changing a core area
// still goes through the existing stack proposal.

type Stack = NonNullable<Project["stack"]>;
const DOC = "/blueprint?doc=technology-plan";
const areaName = (area: string) => TECH_AREA[area as TechArea] ?? areaLabel(area);

export function TechWorkspace({ project }: { project: Project }) {
  const { pending, run } = useProject();
  const [opened, setOpened] = useState("");
  const [cat, setCat] = useState<StackCatalog | null>(null);
  useEffect(() => { api.stackCatalog().then(setCat, () => {}); }, []);
  const s = project.stack;
  const plan = s?.plan;
  const finalized = project.stage === "READY";
  const busy = isWorking(project) || pending === "stack" || !!s?.request || !!s?.plan_request;
  const editable = !finalized && !busy && !s?.proposal;

  if (!s || !plan) return null;

  const errors = issueCounts(plan).errors;
  const notes = planNotes(plan);
  const reviews = s.decisions.filter((d) => d.review);
  const outdated = plan.evaluation.outdated && !plan.stale;
  const a = plan.analysis;
  const rows = { plan, stack: s, catalog: cat, editable, opened, onOpen: setOpened };

  return (
    <div className="repo tp">
      <header className="repo-head">
        <div>
          <h2>Rencana teknologi</h2>
          <p className="small muted">{plan.approach}</p>
        </div>
        <div className="repo-chips">
          <span className={`tech-status ${techState(s)}`}>{TECH_STATUS[techState(s)]}{techVersion(s) ? ` · v${techVersion(s)}` : ""}</span>
          <span className="tag">Skor {plan.evaluation.score}/100</span>
          {plan.stale && <span className="tag warn">Kedaluwarsa</span>}
          {errors > 0 && <span className="tag bad">{errors} error</span>}
        </div>
      </header>

      {!!s.plan_request && (
        <p className="small muted"><span className="spinner" /> {s.plan_request.action === "generate" ? "AI merancang teknologi…" : "AI merevisi rencana teknologi…"} <Elapsed started={project.job?.started} /></p>
      )}
      {s.plan_request?.rejected && <p className="small warn-text">Jawaban AI ditolak, mencoba lagi: {s.plan_request.rejected}</p>}
      {s.proposal && <Proposal p={s.proposal} disabled={busy || finalized} />}
      <DesignReview project={project} />

      {(plan.stale || outdated) && (
        <div className="repo-banner warn">
          <strong>{plan.stale ? "Sumber berubah setelah rencana ditulis" : "Evaluasi kedaluwarsa"}</strong>
          {plan.stale && <div className="small">Area: {(a.stale_areas ?? []).map((x) => PLAN_SOURCE[x] ?? x).join(", ")}. Bagian terdampak: {(a.affected ?? []).map((x) => PLAN_SECTION[x] ?? x).join(", ")}.</div>}
          {outdated && !!a.diverged?.length && <div className="small">Belum mencerminkan pilihan user: {a.diverged.map(areaName).join(", ")}.</div>}
          {editable && <button disabled={pending === "stack"} onClick={() => void run("stack", () => api.reviseTechPlan())}>{plan.stale ? "Revisi bagian terdampak dengan AI" : "Evaluasi ulang"}</button>}
        </div>
      )}

      {(notes.length > 0 || reviews.length > 0) && (
        <section className="tp-notes">
          <h3>Perlu perhatian</h3>
          <ul className="tech-issues small">
            {reviews.map((d) => (
              <li key={d.area} className="warning">! <button className="link" onClick={() => setOpened(d.area)}>{areaName(d.area)}</button>: {d.review}</li>
            ))}
            {notes.map((n, i) => (
              <li key={i} className={n.level}>{n.level === "error" ? "✕" : "!"} {n.area && <strong>{areaName(n.area)}: </strong>}{n.text}</li>
            ))}
          </ul>
        </section>
      )}

      {groupItems(plan).map((g) => {
        const used = g.items.filter((it) => !isNone(it));
        if (!used.length) return null;
        const list = <ItemRows items={used} {...rows} />;
        if (isCoreGroup(g.id)) return <section key={g.id} className="tp-group"><h3>{g.title}</h3>{list}</section>;
        const expand = used.some((it) => it.area === opened || flagged(it, s));
        return <details key={g.id} className="tp-group" open={expand}><summary>{g.title} ({used.length})</summary>{list}</details>;
      })}
      <Unused {...rows} />

      <Approval project={project} stack={s} editable={editable} />

      <p className="small tp-doc"><Link to={DOC}>Lihat dokumen lengkap</Link> <span className="muted">— alasan, alternatif, kompatibilitas, evaluasi, dan riwayat.</span></p>

      {editable && <MoreActions />}
    </div>
  );
}

// A flagged decision must stay reachable: its group opens by itself.
function flagged(it: TechItem, s: Stack): boolean {
  return (!!it.version && !it.version_verified) || !!s.decisions.find((d) => d.area === it.area)?.review;
}

type RowProps = {
  plan: TechPlan; stack: Stack; catalog: StackCatalog | null; editable: boolean; opened: string; onOpen: (a: string) => void;
};

function ItemRows({ items, opened, onOpen, ...rest }: RowProps & { items: TechItem[] }) {
  return (
    <div className="tp-rows">
      {items.map((it) => {
        const open = opened === it.area;
        return (
          <div key={it.area} className={`tp-row${open ? " on" : ""}`}>
            <button className="tp-row-head" aria-expanded={open} onClick={() => onOpen(open ? "" : it.area)}>
              <span className="tech-area">{areaName(it.area)}</span>
              <strong>{it.technology}</strong>
              <span className="tp-badges">
                {it.user_chosen && <span className="tag">dipilih user</span>}
                {!isNone(it) && it.version && !it.version_verified && <span className="tag warn">versi belum terverifikasi</span>}
              </span>
            </button>
            {open && <ItemDetail it={it} {...rest} />}
          </div>
        );
      })}
    </div>
  );
}

function Unused(props: RowProps) {
  const items = props.plan.items.filter(isNone);
  if (!items.length) return null;
  return (
    <details className="tp-group" open={items.some((it) => it.area === props.opened)}>
      <summary>Tidak dipakai ({items.map((it) => areaName(it.area)).join(", ")})</summary>
      <ItemRows items={items} {...props} />
    </details>
  );
}

function ItemDetail({ it, plan, stack, catalog, editable }: {
  it: TechItem; plan: TechPlan; stack: Stack; catalog: StackCatalog | null; editable: boolean;
}) {
  const [edit, setEdit] = useState(false);
  const d = stack.decisions.find((x) => x.area === it.area);
  const alternatives = (it.alternatives ?? []).map((a) => a.technology);
  const conflicts = allCompat(plan).filter((c) => c.status === "conflict" && ((c.between ?? []).includes(it.area) || c.area === it.area));
  return (
    <div className="tp-detail">
      {!isNone(it) && it.purpose && <p>{it.purpose}</p>}
      <p className="small">{it.reasoning}</p>
      {alternatives.length > 0 && <p className="small muted">Alternatif: {alternatives.join(", ")}</p>}
      {conflicts.map((c, i) => <p key={i} className="small warn-text">Konflik {(c.between ?? []).join(", ") || c.area}: {c.note || "—"}</p>)}
      {d && editable && !isNone(it) && (edit || d.review
        ? <TechRow s={stack} d={d} editable={editable} open={edit} options={catalog?.catalog[d.area] ?? []}
            onToggle={() => setEdit(!edit)} onDone={() => setEdit(false)} />
        : <button className="ghost small" onClick={() => setEdit(true)}>Ubah</button>)}
    </div>
  );
}

function Approval({ project, stack: s, editable }: { project: Project; stack: Stack; editable: boolean }) {
  const { run } = useProject();
  const state = techState(s);
  const gate = planGate(project);
  const errors = (s.issues ?? []).filter((i) => i.level === "error");
  const reviews = s.decisions.filter((d) => d.review);
  return (
    <section className="tp-foot">
      {!needsApproval(s) ? <span className="ok-text small">✓ Semua teknologi dibaca dari kode yang ada</span>
        : state === "approved" ? <span className="ok-text small">✓ Teknologi v{techVersion(s)} disetujui{project.stage === "READY" ? " dan masuk spesifikasi final" : ""}</span>
          : editable && (
            <div className="repo-actions">
              <button className="primary" disabled={!editable || !!s.proposal || errors.length > 0 || reviews.length > 0 || !!gate}
                title={gate ?? (s.proposal ? "Terapkan atau batalkan perubahan dulu" : reviews.length ? "Tinjau pilihan yang ditandai dulu" : errors.length ? "Selesaikan konflik dulu" : undefined)}
                onClick={() => void run("stack", api.approveStack)}>Review & Approve</button>
              {gate && <span className="small warn-text">{gate}</span>}
            </div>
          )}
      {state === "changed" && <p className="small warn-text">Teknologi berubah setelah disetujui. Setujui lagi sebelum rancangan disetujui.</p>}
    </section>
  );
}

function MoreActions() {
  const { run, pending } = useProject();
  const [text, setText] = useState("");
  const [why, setWhy] = useState("");
  const [again, setAgain] = useState("");
  const [confirm, setConfirm] = useState(false);
  return (
    <details className="tp-more">
      <summary className="small">Revisi atau rancang ulang</summary>
      <section className="repo-ask">
        <strong>Minta AI merevisi</strong>
        <textarea rows={2} value={text} placeholder="mis. ganti cache ke Valkey; atau jelaskan ulang alasan memilih Elixir"
          onChange={(e) => setText(e.target.value)} />
        <div className="repo-actions">
          <input value={why} maxLength={600} placeholder="Alasan (opsional, dicatat di riwayat)" onChange={(e) => setWhy(e.target.value)} />
          <button className="primary" disabled={pending === "stack" || !text.trim()}
            onClick={async () => { if (await run("stack", () => api.reviseTechPlan(text.trim(), why.trim()))) { setText(""); setWhy(""); } }}>Revisi</button>
        </div>
      </section>
      <div className="repo-actions">
        <input value={again} placeholder="Rancang ulang dari awal dengan arahan, mis. “pakai Go di backend, tetap PostgreSQL”" onChange={(e) => setAgain(e.target.value)} />
        <button disabled={pending === "stack" || !again.trim()} onClick={async () => { if (await run("stack", () => api.generateTechPlan(again.trim()))) setAgain(""); }}>Rancang ulang</button>
        {confirm
          ? <><button className="bad" disabled={pending === "stack"} onClick={() => void run("stack", api.discardTechPlan)}>Ya, hapus rencana</button><button className="ghost" onClick={() => setConfirm(false)}>Batal</button></>
          : <button className="ghost" onClick={() => setConfirm(true)}>Hapus rencana</button>}
      </div>
    </details>
  );
}
