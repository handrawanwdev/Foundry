import { useEffect, useState } from "react";
import { COLUMN_EMPTY, COLUMNS, formatElapsed, formatWhen, PLATFORM_LABEL, queueItem, queueLabel, validTime } from "../flow";
import type { Project, Task } from "../types";

interface Props {
  project: Project;
  selected?: string;
  onSelect: (id: string) => void;
  onSetClosed: (id: string, closed: boolean) => void;
}

export function Kanban({ project, selected, onSelect, onSetClosed }: Props) {
  const tasks = project.tasks ?? [];
  const ticking = tasks.some((t) => (t.status === "IN_PROGRESS" || t.status === "REVIEW") && validTime(t.started_at));
  const now = useNow(ticking);
  const featureName = (id: string) => project.features?.find((f) => f.id === id)?.name ?? "Fondasi";
  return (
    <div className="kanban" role="list">
      {COLUMNS.map((col) => {
        const items = tasks.filter((t) => t.status === col.status);
        if (col.status === "BLOCKED" && items.length === 0) return null;
        const live = col.status === "IN_PROGRESS" && items.length > 0;
        return (
          <section key={col.status} className={`column col-${col.status}${live ? " col-live" : ""}`} aria-label={col.label}>
            <header className="column-head">
              <span className="col-swatch" aria-hidden />
              <span className="col-label">{col.label}</span>
              {live && <span className="spinner" />}
              <span className="col-count">{items.length}</span>
            </header>
            <div className="column-body">
              {items.length === 0 && <div className="column-empty">{COLUMN_EMPTY[col.status]}</div>}
              {items.map((t) => (
                <TaskCard
                  key={t.id}
                  t={t}
                  now={now}
                  feature={featureName(t.feature_id)}
                  queue={queueLabel(queueItem(project, t.id))}
                  active={t.id === selected}
                  onClick={() => onSelect(t.id)}
                  onSetClosed={(closed) => onSetClosed(t.id, closed)}
                />
              ))}
            </div>
          </section>
        );
      })}
    </div>
  );
}

function useNow(active: boolean) {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    if (!active) return;
    const id = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(id);
  }, [active]);
  return now;
}

function TaskCard({ t, now, feature, queue, active, onClick, onSetClosed }: {
  t: Task; now: number; feature: string; queue: string; active: boolean; onClick: () => void; onSetClosed: (closed: boolean) => void;
}) {
  const ready = queue === "siap paralel";
  const working = t.status === "IN_PROGRESS";
  const closed = !!t.closed;
  return (
    <article
      className={`task ${active ? "active" : ""} ${ready ? "task-ready" : ""} ${working ? "working" : ""} ${closed ? "closed" : ""}`}
      role="listitem"
    >
      <button type="button" className="task-hit" onClick={onClick} aria-pressed={active}>
        <div className="task-title">
          {working && <span className="spinner" />}
          {t.title}
        </div>
        <div className="task-meta">
          <span className="task-id">{t.id}</span>
          <span>{t.area}</span>
          {t.platform ? <span>{PLATFORM_LABEL[t.platform] ?? t.platform}</span> : null}
        </div>
        <div className="task-feature">
          {feature}
          {t.repair ? " · perbaikan terblokir" : ""}
          {t.attempts > 1 ? ` · percobaan ${t.attempts}` : ""}
          {t.owner && working ? ` · ${t.owner}` : ""}
        </div>
        {t.progress && (working || t.status === "REVIEW") && (
          <div className="task-progress">{t.progress}</div>
        )}
        <TaskTime t={t} now={now} />
        {closed ? (
          <div className="task-queue">ditutup</div>
        ) : queue ? (
          <div className={`task-queue ${ready ? "ok" : ""}`}>{queue}</div>
        ) : null}
      </button>
      <div className="task-gate" role="group" aria-label="Status pengerjaan">
        <button type="button" className={`open ${!closed ? "on" : ""}`} aria-pressed={!closed} onClick={() => onSetClosed(false)}>Open</button>
        <button type="button" className={`shut ${closed ? "on" : ""}`} aria-pressed={closed} onClick={() => onSetClosed(true)}>Closed</button>
      </div>
      {working && <span className="task-strip" aria-hidden />}
    </article>
  );
}

function TaskTime({ t, now }: { t: Task; now: number }) {
  const live = (t.status === "IN_PROGRESS" || t.status === "REVIEW") && validTime(t.started_at);
  if (live) {
    return <div className="task-time live">pengerjaan {formatElapsed(now - Date.parse(t.started_at!))}</div>;
  }
  if (t.status === "DONE" && validTime(t.finished_at)) {
    const dur = validTime(t.started_at) ? formatElapsed(Date.parse(t.finished_at!) - Date.parse(t.started_at!)) : "";
    return (
      <div className="task-time done">
        selesai {formatWhen(t.finished_at)}{dur ? ` · ${dur}` : ""}
      </div>
    );
  }
  return null;
}
