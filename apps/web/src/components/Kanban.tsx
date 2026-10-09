import { COLUMN_EMPTY, COLUMNS, PLATFORM_LABEL, queueItem, queueLabel } from "../flow";
import type { Project, Task } from "../types";

interface Props {
  project: Project;
  selected?: string;
  onSelect: (id: string) => void;
}

export function Kanban({ project, selected, onSelect }: Props) {
  const tasks = project.tasks ?? [];
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
                  feature={featureName(t.feature_id)}
                  queue={queueLabel(queueItem(project, t.id))}
                  active={t.id === selected}
                  onClick={() => onSelect(t.id)}
                />
              ))}
            </div>
          </section>
        );
      })}
    </div>
  );
}

function TaskCard({ t, feature, queue, active, onClick }: { t: Task; feature: string; queue: string; active: boolean; onClick: () => void }) {
  const ready = queue === "siap paralel";
  const working = t.status === "IN_PROGRESS";
  return (
    <button
      type="button"
      className={`task ${active ? "active" : ""} ${ready ? "task-ready" : ""} ${working ? "working" : ""}`}
      onClick={onClick}
      aria-pressed={active}
    >
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
        {t.attempts > 1 ? ` · percobaan ${t.attempts}` : ""}
        {t.owner && working ? ` · ${t.owner}` : ""}
      </div>
      {t.progress && (working || t.status === "REVIEW") && (
        <div className="task-progress">{t.progress}</div>
      )}
      {queue && <div className={`task-queue ${ready ? "ok" : ""}`}>{queue}</div>}
      {working && <span className="task-strip" aria-hidden />}
    </button>
  );
}
