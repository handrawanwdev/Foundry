import { COLUMNS, PLATFORM_LABEL } from "../flow";
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
    <div className="kanban">
      {COLUMNS.map((col) => {
        const items = tasks.filter((t) => t.status === col.status);
        if (col.status === "BLOCKED" && items.length === 0) return null;
        return (
          <div key={col.status} className={`column col-${col.status}`}>
            <div className="column-head">{col.label} <span className="muted">{items.length}</span></div>
            {items.map((t) => (
              <TaskCard key={t.id} t={t} feature={featureName(t.feature_id)} active={t.id === selected} onClick={() => onSelect(t.id)} />
            ))}
          </div>
        );
      })}
    </div>
  );
}

function TaskCard({ t, feature, active, onClick }: { t: Task; feature: string; active: boolean; onClick: () => void }) {
  return (
    <button className={`task ${active ? "active" : ""}`} onClick={onClick}>
      <div className="row between small muted">
        <span>{t.id}</span>
        <span>{t.area}{t.platform ? ` · ${PLATFORM_LABEL[t.platform] ?? t.platform}` : ""}</span>
      </div>
      <div className="task-title">{t.status === "IN_PROGRESS" && <span className="spinner" />} {t.title}</div>
      <div className="small muted">{feature}{t.attempts > 1 ? ` · percobaan ${t.attempts}` : ""}</div>
    </button>
  );
}
