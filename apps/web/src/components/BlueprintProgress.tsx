import { useEffect, useRef, useState } from "react";
import { api } from "../api";
import { formatElapsed } from "../elapsed";
import { isWorking } from "../flow";
import { useProject } from "../store/project";
import type {
  BlueprintEvent, BlueprintExecution, BlueprintProgress as Run, BlueprintSection,
  BlueprintSectionStatus, BlueprintSkipKind, BlueprintTask, Project,
} from "../types";
import { Elapsed } from "./Elapsed";

// Everything shown here comes from the backend run: statuses, counts, percent
// and estimate. This file only renders them; it derives no status of its own.

const ACTIVE = new Set<BlueprintSectionStatus>(["in_progress", "validating", "repairing", "retrying"]);

const MARK: Record<BlueprintSectionStatus, React.ReactNode> = {
  completed: "✓", skipped: "–", pending: "○", failed: "!", blocked: "⊘",
  retrying: "↻", repairing: "↻", validating: <span className="spinner" />, in_progress: <span className="spinner" />,
};

const STATUS_LABEL: Record<BlueprintSectionStatus, string> = {
  pending: "Menunggu",
  in_progress: "Sedang dikerjakan",
  validating: "Memvalidasi",
  repairing: "Memperbaiki",
  retrying: "Mencoba kembali",
  completed: "Selesai",
  failed: "Gagal",
  blocked: "Terhambat",
  skipped: "Dilewati",
};

const SKIP_LABEL: Record<BlueprintSkipKind, string> = {
  not_applicable: "Tidak relevan",
  not_required: "Tidak diperlukan",
  already_satisfied: "Sudah terpenuhi",
  deferred: "Ditunda",
};

const FAILURE_LABEL: Record<string, string> = {
  validation: "Gagal validasi",
  invalid_output: "Output tidak terbaca",
  overloaded: "Server AI sibuk",
  quota: "Kuota AI habis",
  agent: "AI gagal berjalan",
  interrupted: "Proses terputus",
};

const TASK_LABEL: Record<BlueprintTask["status"], string> = {
  pending: "Menunggu", running: "Berjalan", completed: "Selesai", failed: "Gagal", canceled: "Dibatalkan",
};

const TRIGGER_LABEL: Record<BlueprintExecution["trigger"], string> = {
  initial: "Percobaan pertama",
  continuation: "Lanjutan",
  repair: "Perbaikan",
  retry: "Pengulangan",
  busy_retry: "Pengulangan (server sibuk)",
};

const EXEC_LABEL: Record<BlueprintExecution["status"], string> = {
  running: "Berjalan", succeeded: "Lulus", rejected: "Ditolak validasi", failed: "Gagal", canceled: "Dibatalkan",
};

const VALIDATION_LABEL = { running: "Sedang divalidasi", passed: "Validasi lulus", failed: "Validasi gagal" };

export function buildingInitialBlueprint(p: Project): boolean {
  const bp = p.blueprint_progress;
  if (bp?.status === "running") return true;
  if (bp?.status === "failed" && p.job?.status === "failed") return true;
  if (!isWorking(p)) return false;
  if (!p.discovery) return p.job?.kind === "discovery";
  if (!p.blueprint_queue?.length) return false;
  const existing = p.structure?.mode === "existing";
  return !(p.stack?.plan && (existing || p.structure?.repo));
}

// The closing note of an ended run, for the blueprint page.
export function runBanner(run: Run | undefined): string {
  if (!run) return "";
  const { completed, applicable } = run.summary;
  const parts = `${completed}/${applicable} bagian tervalidasi`;
  if (run.status === "completed") return `Blueprint selesai · ${parts}`;
  if (run.status === "incomplete") return `Blueprint belum lengkap · ${parts}`;
  if (run.status === "stopped") return `Perancangan ditutup · ${parts}`;
  return "";
}

export function countLine(run: Run): string {
  const s = run.summary;
  const notes = [
    s.active && `${s.active} berjalan`,
    s.pending && `${s.pending} menunggu`,
    s.blocked && `${s.blocked} terhambat`,
    s.failed && `${s.failed} gagal`,
    s.skipped && `${s.skipped} dilewati`,
  ].filter(Boolean);
  return [`${s.completed}/${s.applicable} bagian selesai`, `${s.percent}%`, ...notes].join(" · ");
}

export function executionLine(run: Run): string {
  const s = run.summary;
  const parts = [`${s.tasks_completed}/${s.tasks_total} task`, `${s.executions} percobaan AI`];
  if (s.retries) parts.push(`${s.retries} perbaikan/pengulangan`);
  if (s.tasks_failed) parts.push(`${s.tasks_failed} task gagal`);
  return parts.join(" · ");
}

export function etaText(run: Run): string {
  if (run.status !== "running") return "";
  const eta = run.eta;
  if (!eta) return "Estimasi belum tersedia: belum ada data durasi";
  const left = formatElapsed(eta.remaining_ms);
  if (eta.unknown_sections) return `Sisa minimal ±${left}, ${eta.unknown_sections} bagian belum punya data durasi`;
  return `Sisa ±${left}, dari ${eta.samples} pekerjaan sebelumnya`;
}

const clock = (at: string) =>
  new Date(at).toLocaleTimeString("id-ID", { hour: "2-digit", minute: "2-digit", second: "2-digit" });

const isRetryEvent = (e: BlueprintEvent) =>
  (e.attempt_number ?? 0) > 1 || ["validation.failed", "section.failed", "section.waiting"].includes(e.activity_type);

function SectionDetail({ s }: { s: BlueprintSection }) {
  return (
    <div className="bp-detail small">
      {!!s.validation_issues?.length && (
        <>
          <h4>Hasil validasi</h4>
          <ul className="bp-issues">{s.validation_issues.map((m, i) => <li key={i}>{m}</li>)}</ul>
        </>
      )}
      {!!s.tasks?.length && (
        <>
          <h4>Task</h4>
          <ul className="bp-tasks">
            {s.tasks.map((t) => (
              <li key={t.id} className={t.status}>
                <span className="bp-task-state">{TASK_LABEL[t.status]}</span> {t.title}
              </li>
            ))}
          </ul>
        </>
      )}
      {!!s.executions?.length && (
        <>
          <h4>Riwayat percobaan</h4>
          <ul className="bp-execs">
            {s.executions.map((e) => (
              <li key={e.id}>
                <span>#{e.attempt} · {TRIGGER_LABEL[e.trigger]} · {EXEC_LABEL[e.status]}
                  {e.duration_ms ? ` · ${formatElapsed(e.duration_ms)}` : ""}
                  {e.failure_kind ? ` · ${FAILURE_LABEL[e.failure_kind] ?? e.failure_kind}` : ""}</span>
                {e.error && <span className="bp-err">{e.error}</span>}
                <code>{e.id}</code>
              </li>
            ))}
          </ul>
        </>
      )}
    </div>
  );
}

function SectionRow({ s, n, rowRef }: { s: BlueprintSection; n: number; rowRef?: React.Ref<HTMLLIElement> }) {
  const active = ACTIVE.has(s.status);
  const [open, setOpen] = useState(s.status === "failed");
  const attempt = s.attempt ?? 0;
  const head = (
    <>
      <strong>{n}. {s.name}</strong>
      <span className="bp-state muted small"> {STATUS_LABEL[s.status]}</span>
      {attempt > 1 && <span className="bp-tag">percobaan {attempt}</span>}
      {s.validation && <span className={`bp-tag ${s.validation}`}>{VALIDATION_LABEL[s.validation]}</span>}
    </>
  );
  const detail = !!s.tasks?.length || !!s.validation_issues?.length;
  return (
    <li ref={rowRef} className={`bp-stage ${s.status}`} aria-current={active ? "step" : undefined}>
      <span className="bp-mark" aria-hidden>{MARK[s.status]}</span>
      <div className="bp-text">
        {detail ? (
          <details open={open} onToggle={(e) => setOpen((e.target as HTMLDetailsElement).open)}>
            <summary>{head}</summary>
            <SectionDetail s={s} />
          </details>
        ) : <div>{head}</div>}
        {active && s.activity && <span className="bp-sub muted small">└ {s.activity}</span>}
        {(s.status === "blocked" || s.status === "pending") && s.reason && <span className="bp-sub muted small">{s.reason}</span>}
        {s.status === "failed" && s.error && (
          <span className="bp-sub bp-err small">{s.failure_kind ? `${FAILURE_LABEL[s.failure_kind] ?? s.failure_kind}: ` : ""}{s.error.split("\n")[0]}</span>
        )}
      </div>
    </li>
  );
}

export function Timeline({ events }: { events: BlueprintEvent[] }) {
  const [open, setOpen] = useState(true);
  const [retryOnly, setRetryOnly] = useState(false);
  const shown = (retryOnly ? events.filter(isRetryEvent) : events).slice().reverse();
  return (
    <details className="bp-log" open={open} onToggle={(e) => setOpen((e.target as HTMLDetailsElement).open)}>
      <summary>Aktivitas AI ({events.length})</summary>
      <label className="bp-filter small">
        <input type="checkbox" checked={retryOnly} onChange={(e) => setRetryOnly(e.target.checked)} /> Hanya riwayat retry
      </label>
      {shown.length === 0 ? <p className="muted small">Belum ada retry.</p> : (
        <ol>
          {shown.map((e) => (
            <li key={e.event_id} className={e.activity_type.replace(".", "-")}>
              <time dateTime={e.timestamp}>{clock(e.timestamp)}</time>
              <span>{e.message}{(e.attempt_number ?? 0) > 1 && <span className="bp-tag">percobaan {e.attempt_number}</span>}</span>
            </li>
          ))}
        </ol>
      )}
    </details>
  );
}

function Duration({ run }: { run: Run }) {
  if (!run.ended_at) return <>Berjalan <Elapsed started={run.started_at} /></>;
  return <>Durasi {formatElapsed(Date.parse(run.ended_at) - Date.parse(run.started_at))}</>;
}

export function BlueprintProgress({ project }: { project: Project }) {
  const { run: act, pending, stream, timeline } = useProject();
  const bp = project.blueprint_progress;
  const activeRef = useRef<HTMLLIElement | null>(null);
  const firstActive = bp?.sections.find((s) => ACTIVE.has(s.status))?.id;

  useEffect(() => {
    activeRef.current?.scrollIntoView({ block: "nearest", behavior: "smooth" });
  }, [firstActive]);

  if (!bp) {
    return (
      <div className="bp-progress">
        <header className="bp-head">
          <h1><span className="spinner" />Merancang Blueprint</h1>
          <p className="bp-activity-now">{project.job?.message ?? "Menyiapkan rancangan…"}</p>
        </header>
      </div>
    );
  }

  const failed = bp.status === "failed" || project.job?.status === "failed";
  const applicable = bp.sections.filter((s) => s.status !== "skipped");
  const skipped = bp.sections.filter((s) => s.status === "skipped");
  const active = applicable.filter((s) => ACTIVE.has(s.status));
  const eta = etaText(bp);
  const changes = bp.plan_changes ?? [];

  return (
    <div className="bp-progress">
      {stream === "reconnecting" && <p className="bp-reconnect" role="status">Menyambungkan ulang… progress dipulihkan dari server.</p>}
      <header className="bp-head">
        <h1>
          {failed ? null : <span className="spinner" />}
          {failed ? "Perancangan Blueprint gagal" : "Merancang Blueprint"}
        </h1>
        {active.length > 1 && <p className="bp-activity-now">AI mengerjakan {active.length} bagian paralel: {active.map((s) => s.name).join(", ")}</p>}
        {active.length <= 1 && !failed && <p className="bp-activity-now">{active[0]?.activity ?? bp.activity}</p>}
        <p className="muted">{countLine(bp)}</p>
        <div className="bp-bar" aria-hidden><span style={{ width: `${bp.summary.percent}%` }} /></div>
        <p className="muted small bp-meta"><Duration run={bp} />{eta && ` · ${eta}`}</p>
        <p className="muted small bp-meta">Eksekusi: {executionLine(bp)}</p>
      </header>
      <ol className="bp-stages">
        {applicable.map((s, i) => (
          <SectionRow key={s.id} s={s} n={i + 1} rowRef={s.id === firstActive ? activeRef : undefined} />
        ))}
      </ol>
      {skipped.length > 0 && (
        <details className="bp-log">
          <summary>Dilewati ({skipped.length})</summary>
          <ul className="bp-skips small">
            {skipped.map((s) => (
              <li key={s.id}>
                <strong>{s.name}</strong> <span className="bp-tag">{s.skip_kind ? SKIP_LABEL[s.skip_kind] : "Dilewati"}</span>
                <span className="muted"> {s.reason}</span>
              </li>
            ))}
          </ul>
        </details>
      )}
      {changes.length > 0 && (
        <details className="bp-log">
          <summary>Perubahan rencana ({changes.length})</summary>
          <ul className="bp-skips small">
            {changes.map((c) => (
              <li key={c.version}><strong>{c.from} menjadi {c.to} bagian</strong><span className="muted"> {c.reason}</span></li>
            ))}
          </ul>
        </details>
      )}
      {timeline.length > 0 && <Timeline events={timeline} />}
      {failed && (
        <div className="bp-fail">
          <p className="bp-err">{bp.error || project.job?.error || "Proses gagal."}</p>
          <div className="bp-actions">
            <button className="primary" disabled={pending === "retry"} onClick={() => void act("retry", api.retry)}>Coba lagi</button>
            <button className="ghost" disabled={pending === "dismiss"} onClick={() => void act("dismiss", api.dismissJob)}>Tutup</button>
          </div>
        </div>
      )}
    </div>
  );
}
