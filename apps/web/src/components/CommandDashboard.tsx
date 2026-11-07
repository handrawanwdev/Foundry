import { useEffect, useState } from "react";
import { api } from "../api";
import { changeSummary, overviewTiles, PHASE_CLASS, PHASE_LABEL, REPORT_LABEL, RUN_LABEL } from "../blueprint";
import { useProject } from "../store/project";
import type { CommandHistory, CommandProgress, ProgressActivity, ProgressSnapshot, RunStatus, WorkflowChange } from "../types";

const time = (at: string) => new Date(at).toLocaleString([], { day: "numeric", month: "short", hour: "2-digit", minute: "2-digit" });

function runLabel(status: string) {
  return RUN_LABEL[status as RunStatus] ?? status;
}

// Numbers of the whole workflow plus everything that needs someone's eye.
// All of it comes from agent reports; a command with no report counts as not
// started, never as done.
export function ExecutionOverview({ progress, onOpen, onReconcile, busy }: {
  progress: ProgressSnapshot; onOpen: (id: string) => void; onReconcile: () => void; busy: boolean;
}) {
  const stream = useProject((s) => s.stream);
  const warnings = progress.warnings.filter((w) => w.kind !== "incomplete");
  const sessions = (progress.sessions ?? []).filter((s) => s.active);
  const o = progress.overview;
  return (
    <div className="prog-panel">
      <div className="prog-line">
        <strong>{progress.complete ? "Workflow terverifikasi" : "Progres pengerjaan"}</strong>
        <span className="muted small">{o.percent}% · bobot {progress.done_weight}/{progress.weight}
          {" "}<span className={`wf-live ${stream === "open" ? "on" : ""}`}><i />{stream === "open" ? "Langsung" : "Menyambung ulang"}</span>
        </span>
      </div>
      <div className="prog-bar big"><i style={{ width: `${o.percent}%` }} /></div>
      <div className="wf-tiles">
        {overviewTiles(o).map((t) => (
          <div key={t.label} className={`wf-tile ${t.value ? t.tone ?? "" : "zero"}`}><b>{t.value}</b><span className="muted small">{t.label}</span></div>
        ))}
      </div>
      <p className="muted small">
        {progress.last_report_at ? `Laporan terakhir ${time(progress.last_report_at)}${progress.last_source ? ` · ${progress.last_source}` : ""}` : "Belum ada laporan agen."}
        {sessions.length > 0 && ` · ${sessions.length} session aktif: ${sessions.map((s) => `${s.id} (${s.command})`).join(", ")}`}
      </p>
      {progress.next.length > 0 && !progress.complete && (
        <p className="small">Bisa dilanjutkan: {progress.next.map((id) => <button key={id} className="ref-chip" onClick={() => onOpen(id)}>{id}</button>)}</p>
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
          <strong>{warnings.length} perlu perhatian</strong>
          <ul>{warnings.slice(0, 6).map((w, i) => (
            <li key={i}>{w.command && <button className="link" onClick={() => onOpen(w.command!)}>{w.command}</button>} {w.message} {w.action && <span className="muted">{w.action}</span>}</li>
          ))}</ul>
        </div>
      )}
    </div>
  );
}

// While work runs the latest note says most; afterwards the final summary does.
function latest(c: CommandProgress) {
  return c.stored === "started" ? c.activity || c.summary : c.summary || c.activity;
}

function rowNote(c: CommandProgress): string {
  if (c.phase === "created") return `Menunggu ${c.waiting?.join(", ")}`;
  const when = [c.started_at && `Mulai ${time(c.started_at)}`, c.finished_at && `Selesai ${time(c.finished_at)}`].filter(Boolean).join(" · ");
  return [c.reason || c.revision_why || latest(c), when].filter(Boolean).join(" · ");
}

// The commands in execution order, then what agents reported lately.
export function ExecutionTimeline({ progress, onOpen }: { progress: ProgressSnapshot; onOpen: (id: string) => void }) {
  const byWave = new Map<number, CommandProgress[]>();
  for (const c of progress.commands) byWave.set(c.wave, [...(byWave.get(c.wave) ?? []), c]);
  const activity = progress.activity ?? [];
  return (
    <>
      <section className="wf-section">
        <h2>Timeline</h2>
        {[...byWave.entries()].sort(([a], [b]) => a - b).map(([wave, cmds]) => (
          <div key={wave}>
            <div className="wf-wave">{wave > 0 ? `Gelombang ${wave}${cmds.length > 1 ? " · paralel" : ""}` : "Urutan belum valid"}</div>
            <ul className="wf-rows">
              {cmds.map((c) => {
                const note = rowNote(c);
                const live = (c.phase === "in_progress" || c.phase === "updating") && c.percent !== undefined;
                return (
                  <li key={c.id} className="wf-row">
                    <button onClick={() => onOpen(c.id)}>
                      <span className="cmd-id">{c.id}</span>
                      <span>{c.name}</span>
                      <span className={`state ${PHASE_CLASS[c.phase]}`}>{PHASE_LABEL[c.phase]}{live ? ` ${c.percent}%` : ""}</span>
                      {note && <span className="wf-row-note">{note}</span>}
                      {live && <span className="prog-bar" title="Perkiraan agen"><i style={{ width: `${c.percent}%` }} /></span>}
                    </button>
                  </li>
                );
              })}
            </ul>
          </div>
        ))}
      </section>
      {activity.length > 0 && (
        <section className="wf-section">
          <h2>Aktivitas terbaru</h2>
          <Feed items={activity} onOpen={onOpen} />
        </section>
      )}
    </>
  );
}

function Feed({ items, onOpen }: { items: ProgressActivity[]; onOpen?: (id: string) => void }) {
  return (
    <ul className="wf-feed">
      {items.map((a, i) => (
        <li key={i}>
          <time dateTime={a.at}>{time(a.at)}</time>
          <span className="wf-body">
            {onOpen && <><button className="link" onClick={() => onOpen(a.command)}>{a.command}</button>{" "}</>}
            <strong>{REPORT_LABEL[a.status] ?? a.status}{a.percent !== undefined ? ` ${a.percent}%` : ""}</strong>
            {a.applied && a.applied !== a.status && a.status !== "progress" && ` · tercatat ${runLabel(a.applied).toLowerCase()}`}
            {a.outcome === "ignored" && " · diabaikan (lama)"}
            {(a.summary || a.reason) && ` · ${[a.summary, a.reason].filter(Boolean).join(" — ")}`}
          </span>
          {(a.source || a.session || a.files?.length) && (
            <span className="wf-meta">{[a.source, a.session, a.files?.join(", ")].filter(Boolean).join(" · ")}</span>
          )}
        </li>
      ))}
    </ul>
  );
}

// Execution of one command: what the agent says now, its evidence, and every
// report, status change and revision kept for it.
export function CommandReport({ id, run, version, onOpen }: {
  id: string; run: CommandProgress; version: number; onOpen: (id: string) => void;
}) {
  const [history, setHistory] = useState<CommandHistory | null>(null);
  useEffect(() => {
    let ignore = false;
    api.commandHistory(id).then((h) => { if (!ignore) setHistory(h); }).catch(() => { if (!ignore) setHistory(null); });
    return () => { ignore = true; };
  }, [id, version]);
  const trouble = run.phase === "blocked" || run.phase === "failed" || run.phase === "revision_required";
  const times = [run.source && `Sumber: ${run.source}`, run.session && `Session ${run.session}`,
    run.started_at && `Mulai ${time(run.started_at)}`, run.finished_at && `Selesai ${time(run.finished_at)}`,
    run.updated_at && `Diperbarui ${time(run.updated_at)}`].filter(Boolean).join(" · ");
  const noReport = run.status === "unconfirmed" || run.status === "waiting";
  const events = history ? history.history.length + history.changes.length + history.reports.length : 0;
  return (
    <section className="doc-sec">
      <h2>Progres pengerjaan</h2>
      {noReport ? (
        <p className="muted small">{run.waiting?.length ? `Menunggu ${run.waiting.join(", ")}. ` : ""}Belum ada laporan agen.</p>
      ) : (
        <>
          <p className="small">{latest(run) || PHASE_LABEL[run.phase]}{run.percent !== undefined && run.stored === "started" ? ` (perkiraan agen ${run.percent}%)` : ""}</p>
          {trouble && <div className="notice warn"><span><strong>{PHASE_LABEL[run.phase]}:</strong> {run.reason || run.revision_why || "tanpa keterangan"}</span></div>}
          {run.impact && run.impact.length > 0 && (
            <p className="small">Menahan: {run.impact.map((d) => <button key={d} className="ref-chip" onClick={() => onOpen(d)}>{d}</button>)}</p>
          )}
          {times && <p className="muted small">{times}</p>}
        </>
      )}
      {run.waiting && run.waiting.length > 0 && !noReport && <p className="small">Menunggu: {run.waiting.join(", ")}</p>}
      {(history?.evidence.length ?? 0) > 0 && (
        <ul>{history!.evidence.map((e, i) => <li key={i}><code>{e.ref}</code>{e.kind ? ` · ${e.kind}` : ""}</li>)}</ul>
      )}
      {(history?.tests.length ?? 0) > 0 && (
        <ul>{history!.tests.map((t, i) => <li key={i}>{t.name}: {t.result}{t.detail ? ` — ${t.detail}` : ""}</li>)}</ul>
      )}
      {history && events > 0 && (
        <details className="tp-more">
          <summary>Riwayat ({events})</summary>
          <Feed items={history.reports} />
          {history.history.length > 0 && <ul>{history.history.map((h, i) => (
            <li key={i} className="small">{time(h.at)}: {runLabel(h.from)} ke {runLabel(h.to).toLowerCase()}{h.reason ? ` — ${h.reason}` : ""}</li>
          ))}</ul>}
          {history.changes.map((c, i) => <Revision key={i} change={c} />)}
        </details>
      )}
    </section>
  );
}

function Revision({ change: c }: { change: CommandHistory["changes"][number] }) {
  return (
    <details className="small">
      <summary>Revisi {c.revision} · workflow v{c.version} · {time(c.at)}{c.reason ? ` · ${c.reason}` : ""}</summary>
      {c.fields && c.fields.length > 0 && <p className="muted">Berubah: {c.fields.join(", ")}</p>}
      {c.before && <pre className="mono">{c.before.instructions}</pre>}
    </details>
  );
}

// The delta log of the workflow, newest first.
export function WorkflowChanges({ changes, onOpen }: { changes: WorkflowChange[]; onOpen: (id: string) => void }) {
  if (changes.length === 0) return null;
  return (
    <section className="wf-section">
      <details>
        <summary><strong className="small">Riwayat revisi workflow ({changes.length})</strong></summary>
        <ul className="wf-feed">
          {[...changes].reverse().map((c, i) => (
            <li key={i}>
              <time dateTime={c.at}>{time(c.at)}</time>
              <span className="wf-body">
                <strong>v{c.version}</strong> · {changeSummary(c)}{c.reason ? ` · ${c.reason}` : ""}
                {" "}{[...(c.added ?? []), ...(c.modified ?? []).map((m) => m.id)].slice(0, 8).map((id) => <button key={id} className="ref-chip" onClick={() => onOpen(id)}>{id}</button>)}
              </span>
            </li>
          ))}
        </ul>
      </details>
    </section>
  );
}
