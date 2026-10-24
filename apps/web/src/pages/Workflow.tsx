import { useEffect, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { api } from "../api";
import { commandState, commandText, dependents, lines, RUN_CLASS, RUN_LABEL, runOf, STATE_LABEL, waveOf } from "../blueprint";
import { Icon } from "../components/Icon";
import { Markdown } from "../components/Markdown";
import { PromptBar } from "../components/PromptBar";
import { isWorking } from "../flow";
import { useProject } from "../store/project";
import type { Command, CommandEdit, CommandProgress, ProgressSnapshot, Project, RunStatus, WorkflowReport, WorkflowView } from "../types";

// Workflow workspace: commands by wave on the left, one command in full.
// Foundry writes and checks the instructions; an external coding agent
// carries them out.
export function WorkflowPage() {
  const { project, run, pending } = useProject();
  const [view, setView] = useState<WorkflowView | null>(null);
  const [params, setParams] = useSearchParams();

  useEffect(() => {
    if (!project?.discovery) return;
    let ignore = false;
    const load = () => api.workflow().then((v) => { if (!ignore) setView(v); }).catch(() => { if (!ignore) setView(null); });
    load();
    const t = setInterval(load, 15_000);
    return () => { ignore = true; clearInterval(t); };
  }, [project?.version, project?.dir]);

  if (!project) return null;
  const working = isWorking(project);
  const locked = project.stage === "READY";
  const w = view?.workflow;
  const r = view?.report ?? null;
  const prog = view?.progress ?? null;
  const cmds = w?.commands ?? [];

  if (!project.discovery) {
    return (
      <div className="dw-empty">
        <h1>Belum ada blueprint</h1>
        <p className="muted">Workflow disusun dari blueprint. Mulai dari prompt.</p>
        <Link className="button primary" to="/simple/tell">Tulis prompt</Link>
      </div>
    );
  }
  if (cmds.length === 0) {
    const writing = working && project.job?.kind === "workflow";
    return (
      <div className="dw-empty">
        <h1>Command workflow</h1>
        <p className="muted">
          Foundry menyusun instruksi untuk coding agent eksternal dari blueprint: tujuan, konteks, dependency, output, dan
          acceptance criteria tiap command. Foundry tidak menjalankannya.
        </p>
        {writing ? <p><span className="spinner" /> Menyusun workflow…</p> : (
          <button className="primary big" disabled={working || locked || pending === "generateWorkflow"}
            onClick={() => void run("generateWorkflow", api.generateWorkflow)}>Susun workflow dari blueprint</button>
        )}
        {working && !writing && <p className="muted small">Tunggu proses AI yang sedang berjalan selesai.</p>}
      </div>
    );
  }

  const id = params.get("cmd") ?? cmds[0].id;
  const cmd = cmds.find((c) => c.id === id) ?? cmds[0];
  const select = (next: string) => setParams({ cmd: next }, { replace: true });
  const reviewIds = Object.keys(r?.review ?? {});
  const staleIds = Object.keys(r?.stale ?? {}).filter((k) => (r?.stale[k]?.length ?? 0) > 0);
  const byId = new Map(cmds.map((c) => [c.id, c]));
  const waves = r && r.waves.length ? r.waves : [cmds.map((c) => c.id)];

  return (
    <div className="ws">
      <nav className="ws-nav" aria-label="Command">
        <div className="ws-nav-head">
          <strong>Workflow</strong>
          <span className="muted small">v{w!.version} · {cmds.length} command</span>
        </div>
        {prog && (locked || isTracked(prog)) && prog.counts.total > 0 && (
          <div className="prog-mini" title={`${prog.counts.verified}/${prog.counts.total} terverifikasi`}>
            <div className="prog-bar"><i style={{ width: `${Math.round((prog.counts.verified / Math.max(prog.counts.total, 1)) * 100)}%` }} /></div>
            <span className="muted small">{prog.counts.verified}/{prog.counts.total} terverifikasi</span>
          </div>
        )}
        {waves.map((wave, i) => (
          <div key={i} className="ws-group">
            <div className="ws-group-label">
              {r?.waves.length ? `Gelombang ${i + 1}${wave.length > 1 ? " · paralel" : ""}` : "Urutan belum valid"}
              {prog?.waves[i] && (locked || isTracked(prog)) && <span> · {prog.waves[i].verified}/{prog.waves[i].total}</span>}
            </div>
            {wave.map((cid) => {
              const c = byId.get(cid);
              if (!c) return null;
              const st = commandState(c, r);
              const run = runOf(prog, cid);
              const runClass = run ? RUN_CLASS[run.status] : "";
              return (
                <button key={cid} className={`ws-item cmd ${cid === cmd.id ? "on" : ""}`} aria-current={cid === cmd.id ? "page" : undefined} onClick={() => select(cid)}>
                  <span className="cmd-id">{cid}</span>
                  <span className="ws-item-text">{c.name}</span>
                  {run && run.status !== "unconfirmed" && <span className={`cmd-dot ${runClass || "st-muted"}`} title={RUN_LABEL[run.status]} />}
                  {st !== "draft" && !(run && run.status !== "unconfirmed") && <span className={`cmd-dot st-${st}`} title={STATE_LABEL[st]} />}
                </button>
              );
            })}
          </div>
        ))}
      </nav>

      <main className="ws-main">
        {prog && (locked || isTracked(prog)) && <ProgressPanel progress={prog} onOpen={select} onReconcile={() => void run("reconcileProgress", () => api.reconcileProgress())} busy={pending === "reconcileProgress"} />}
        {r && r.issues.length > 0 && (
          <div className="notice bad">
            <strong>{r.issues.length} masalah workflow</strong>
            <ul>{r.issues.slice(0, 6).map((is, i) => <li key={i}>{is.command && <button className="link" onClick={() => select(is.command!)}>{is.command}</button>} {is.message}</li>)}</ul>
          </div>
        )}
        {r && r.uncovered.length > 0 && !locked && (
          <div className="notice warn">
            <span><strong>Requirement MUST belum dicakup command:</strong> {r.uncovered.join(", ")}. Isi “Mencakup requirement” di command terkait.</span>
          </div>
        )}
        {reviewIds.length > 0 && !locked && (
          <div className="notice warn">
            <span><strong>{reviewIds.length} command perlu ditinjau</strong>{staleIds.length > 0 && " — artefak blueprint yang dirujuk sudah berubah"}.</span>
            {staleIds.length > 0 && (
              <button disabled={working || pending === "reviseWorkflow"} onClick={() => void run("reviseWorkflow",
                () => api.reviseWorkflow("Perbarui command yang terdampak perubahan artefak blueprint. Ubah hanya yang memang perlu.", staleIds))}>
                ✦ Perbarui yang terdampak
              </button>
            )}
          </div>
        )}
        <CommandDoc key={cmd.id + w!.version} project={project} cmd={cmd} report={r} run={runOf(prog, cmd.id)} locked={locked} onOpen={select} />
      </main>

      {!locked && (
        <PromptBar
          label={`Minta perubahan workflow (sedang melihat: ${cmd.id})…`}
          placeholder="Contoh: pecah CMD-003 jadi backend dan frontend; tambahkan command migrasi data sebelum CMD-004."
          hint="AI mengubah command yang terdampak saja; dependency divalidasi."
          disabled={working}
          busy={pending === "reviseWorkflow"}
          onSend={(text) => run("reviseWorkflow", () => api.reviseWorkflow(text, [cmd.id]))}
        />
      )}
    </div>
  );
}

function isTracked(p: ProgressSnapshot) {
  const c = p.counts;
  return c.started + c.blocked + c.skipped + c.failed + c.review + c.done + c.verified + c.stale + c.unknown > 0;
}

function ProgressPanel({ progress, onOpen, onReconcile, busy }: {
  progress: ProgressSnapshot; onOpen: (id: string) => void; onReconcile: () => void; busy: boolean;
}) {
  const c = progress.counts;
  const bits = [
    c.started && `${c.started} dikerjakan`,
    c.review && `${c.review} ditinjau`,
    c.failed && `${c.failed} gagal`,
    c.blocked && `${c.blocked} terhambat`,
    c.stale && `${c.stale} kedaluwarsa`,
    c.unknown && `${c.unknown} tidak diketahui`,
    c.waiting && `${c.waiting} menunggu`,
    c.unconfirmed && `${c.unconfirmed} belum terkonfirmasi`,
  ].filter(Boolean);
  const warnings = progress.warnings.filter((w) => w.kind !== "incomplete");
  const incomplete = progress.warnings.find((w) => w.kind === "incomplete");
  return (
    <div className="prog-panel">
      <div className="prog-line">
        <strong>{progress.complete ? "Workflow terverifikasi" : incomplete?.message ?? "Progres pengerjaan"}</strong>
        <span className="muted small">{bits.join(" · ")}</span>
      </div>
      {progress.last_report_at && (
        <p className="muted small">Laporan terakhir {new Date(progress.last_report_at).toLocaleString()}{progress.last_source ? ` · ${progress.last_source}` : ""}</p>
      )}
      {progress.next.length > 0 && !progress.complete && (
        <p className="small">Siap dikerjakan: {progress.next.map((id) => <button key={id} className="ref-chip" onClick={() => onOpen(id)}>{id}</button>)}</p>
      )}
      {progress.issues.length > 0 && (
        <div className="notice warn">
          <strong>Status tidak sinkron</strong>
          <ul>{progress.issues.map((m, i) => <li key={i}>{m}</li>)}</ul>
          <button disabled={busy} onClick={onReconcile}>Samakan dengan workflow</button>
        </div>
      )}
      {warnings.length > 0 && (
        <div className="notice warn">
          <strong>{warnings.length} peringatan pelacakan</strong>
          <ul>{warnings.slice(0, 6).map((w, i) => (
            <li key={i}>{w.command && <button className="link" onClick={() => onOpen(w.command!)}>{w.command}</button>} {w.message} {w.action && <span className="muted">{w.action}</span>}</li>
          ))}</ul>
        </div>
      )}
    </div>
  );
}

function CommandDoc({ project, cmd, report, run, locked, onOpen }: {
  project: Project; cmd: Command; report: WorkflowReport | null; run?: CommandProgress; locked: boolean; onOpen: (id: string) => void;
}) {
  const { run: act, pending } = useProject();
  const [editing, setEditing] = useState(false);
  const [copied, setCopied] = useState(false);
  const st = commandState(cmd, report);
  const wave = waveOf(report, cmd.id);
  const stale = report?.stale[cmd.id] ?? [];
  const deps = dependents(project.workflow, cmd.id);
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(commandText(cmd, report));
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch { /* clipboard blocked: nothing to do */ }
  };
  const list = (title: string, items?: string[], check = false) => items && items.length > 0 && (
    <section className="doc-sec">
      <h2>{title}</h2>
      <ul className={check ? "checks" : undefined}>{items.map((x, i) => <li key={i}>{check && <span aria-hidden>☐ </span>}{x}</li>)}</ul>
    </section>
  );

  return (
    <article className="doc">
      <header className="doc-head">
        <div className="doc-kicker">
          <span className="cmd-id big">{cmd.id}</span>
          <span className={`state st-${st}`}>{STATE_LABEL[st]}</span>
          {run && <span className={`state ${RUN_CLASS[run.status as RunStatus]}`}>{RUN_LABEL[run.status]}</span>}
          {wave > 0 && <span className="muted small">Gelombang {wave}</span>}
        </div>
        <h1>{cmd.name}</h1>
        <p className="doc-summary">{cmd.objective}</p>
        <div className="doc-meta small">
          <span className="muted">Bergantung pada:</span>
          {cmd.depends_on?.length ? cmd.depends_on.map((d) => <button key={d} className="ref-chip" onClick={() => onOpen(d)}>{d}</button>) : <span>—</span>}
          {deps.length > 0 && <><span className="muted">· Dibutuhkan oleh:</span>{deps.map((d) => <button key={d} className="ref-chip" onClick={() => onOpen(d)}>{d}</button>)}</>}
        </div>
        <div className="doc-actions">
          <button onClick={() => void copy()}><Icon name="copy" size={14} /> {copied ? "Tersalin" : "Salin instruksi"}</button>
          {!locked && !editing && <button onClick={() => setEditing(true)}>Edit</button>}
          {!locked && st !== "review" && st !== "issue" && (
            <button className="ghost" disabled={pending === "review"} onClick={() => void act("review", () => api.reviewCommand(cmd.id, true))}>Tandai perlu ditinjau</button>
          )}
        </div>
      </header>

      {st === "review" && !locked && (
        <div className="notice warn">
          <span>{report?.review[cmd.id] || "Perlu ditinjau"}</span>
          <button disabled={pending === "review"} onClick={() => void act("review", () => api.reviewCommand(cmd.id, false))}>Tandai sudah ditinjau</button>
        </div>
      )}

      {run && run.status !== "unconfirmed" && (
        <section className="doc-sec">
          <h2>Progres pengerjaan</h2>
          <p className="small">{run.summary || RUN_LABEL[run.status]}{run.reason ? ` — ${run.reason}` : ""}</p>
          <p className="muted small">
            {run.source && `Sumber: ${run.source}`}
            {run.updated_at && ` · Diperbarui ${new Date(run.updated_at).toLocaleString()}`}
            {run.started_at && ` · Mulai ${new Date(run.started_at).toLocaleString()}`}
          </p>
          {run.waiting && run.waiting.length > 0 && <p className="small">Menunggu: {run.waiting.join(", ")}</p>}
          {run.evidence && run.evidence.length > 0 && (
            <ul>{run.evidence.map((e, i) => <li key={i}><code>{e.ref}</code>{e.kind ? ` · ${e.kind}` : ""}</li>)}</ul>
          )}
          {run.tests && run.tests.length > 0 && (
            <ul>{run.tests.map((t, i) => <li key={i}>{t.name}: {t.result}{t.detail ? ` — ${t.detail}` : ""}</li>)}</ul>
          )}
        </section>
      )}
      {editing ? <CommandEditor project={project} cmd={cmd} report={report} onDone={() => setEditing(false)} /> : (
        <>
          {cmd.context && <section className="doc-sec"><h2>Konteks</h2><Markdown text={cmd.context} /></section>}
          {(cmd.artifacts?.length ?? 0) > 0 && (
            <section className="doc-sec">
              <h2>Acuan blueprint</h2>
              <div className="ref-list">
                {cmd.artifacts!.map((a) => (
                  <Link key={a.id} className={`ref-chip ${stale.includes(a.id) ? "stale" : ""}`} to={`/blueprint?doc=${a.id}`}
                    title={stale.includes(a.id) ? "Berubah sejak command ditulis" : undefined}>
                    {report?.docs[a.id]?.title ?? a.id}{stale.includes(a.id) && " · berubah"}
                  </Link>
                ))}
              </div>
            </section>
          )}
          {(cmd.covers?.length ?? 0) > 0 && (
            <section className="doc-sec">
              <h2>Mencakup requirement</h2>
              <div className="ref-list">
                {cmd.covers!.map((id) => <Link key={id} className="ref-chip" to="/blueprint?doc=acceptance" title="Kriteria penerimaan">{id}</Link>)}
              </div>
            </section>
          )}
          {list("Input", cmd.inputs)}
          <section className="doc-sec"><h2>Instruksi</h2><Markdown text={cmd.instructions} /></section>
          {list("Output yang diharapkan", cmd.expected_outputs)}
          {list("Acceptance criteria", cmd.acceptance_criteria, true)}
          {list("Batasan", cmd.constraints)}
        </>
      )}
    </article>
  );
}

function CommandEditor({ project, cmd, report, onDone }: { project: Project; cmd: Command; report: WorkflowReport | null; onDone: () => void }) {
  const { run, pending } = useProject();
  const others = (project.workflow?.commands ?? []).filter((c) => c.id !== cmd.id);
  const docs = Object.entries(report?.docs ?? {});
  const [e, setE] = useState({
    name: cmd.name, objective: cmd.objective, context: cmd.context ?? "", instructions: cmd.instructions,
    inputs: (cmd.inputs ?? []).join("\n"), outputs: (cmd.expected_outputs ?? []).join("\n"),
    acceptance: (cmd.acceptance_criteria ?? []).join("\n"), constraints: (cmd.constraints ?? []).join("\n"),
    deps: cmd.depends_on ?? [], artifacts: (cmd.artifacts ?? []).map((a) => a.id), covers: cmd.covers ?? [],
  });
  const set = (k: keyof typeof e) => (ev: { target: { value: string } }) => setE({ ...e, [k]: ev.target.value });
  const toggle = (k: "deps" | "artifacts" | "covers", v: string) => setE({ ...e, [k]: e[k].includes(v) ? e[k].filter((x) => x !== v) : [...e[k], v] });
  const save = async () => {
    const edit: CommandEdit = {
      name: e.name, objective: e.objective, context: e.context, instructions: e.instructions, inputs: lines(e.inputs),
      depends_on: e.deps, artifacts: e.artifacts, covers: e.covers, expected_outputs: lines(e.outputs), acceptance_criteria: lines(e.acceptance), constraints: lines(e.constraints),
    };
    if (await run("editCommand", () => api.editCommand(cmd.id, edit))) onDone();
  };
  return (
    <div className="doc-edit">
      <label className="field">Nama<input value={e.name} onChange={set("name")} /></label>
      <label className="field">Tujuan<input value={e.objective} onChange={set("objective")} /></label>
      <label className="field">Konteks<textarea rows={3} value={e.context} onChange={set("context")} /></label>
      <label className="field">Instruksi (Markdown)<textarea className="mono" rows={12} value={e.instructions} onChange={set("instructions")} /></label>
      <fieldset className="field">
        <legend>Bergantung pada</legend>
        <div className="check-grid">
          {others.length === 0 && <span className="muted small">Tidak ada command lain.</span>}
          {others.map((c) => (
            <label key={c.id} className="check"><input type="checkbox" checked={e.deps.includes(c.id)} onChange={() => toggle("deps", c.id)} /> {c.id} <span className="muted">{c.name}</span></label>
          ))}
        </div>
      </fieldset>
      <fieldset className="field">
        <legend>Acuan blueprint</legend>
        <div className="check-grid">
          {docs.map(([id, d]) => (
            <label key={id} className="check"><input type="checkbox" checked={e.artifacts.includes(id)} onChange={() => toggle("artifacts", id)} /> {d.title}</label>
          ))}
        </div>
      </fieldset>
      <fieldset className="field">
        <legend>Mencakup requirement</legend>
        <div className="check-grid">
          {(project.requirements ?? []).map((q) => (
            <label key={q.id} className="check" title={q.text}><input type="checkbox" checked={e.covers.includes(q.id)} onChange={() => toggle("covers", q.id)} /> {q.id} <span className="muted">{q.priority}</span></label>
          ))}
        </div>
      </fieldset>
      <label className="field">Input <span className="muted small">(satu per baris)</span><textarea rows={3} value={e.inputs} onChange={set("inputs")} /></label>
      <label className="field">Output yang diharapkan <span className="muted small">(satu per baris)</span><textarea rows={3} value={e.outputs} onChange={set("outputs")} /></label>
      <label className="field">Acceptance criteria <span className="muted small">(satu per baris)</span><textarea rows={4} value={e.acceptance} onChange={set("acceptance")} /></label>
      <label className="field">Batasan <span className="muted small">(satu per baris)</span><textarea rows={3} value={e.constraints} onChange={set("constraints")} /></label>
      <div className="row">
        <button className="primary" disabled={!e.name.trim() || !e.objective.trim() || !e.instructions.trim() || pending === "editCommand"} onClick={() => void save()}>Simpan</button>
        <button className="ghost" onClick={onDone}>Batal</button>
        <span className="muted small">Dependency melingkar atau referensi yang tidak ada ditolak.</span>
      </div>
    </div>
  );
}
