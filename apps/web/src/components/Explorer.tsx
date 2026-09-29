import { useState, type ReactNode } from "react";
import { FileTree } from "./FileTree";
import { useLocation, useNavigate } from "react-router-dom";
import { COLUMNS, reachable, stepIndex, STEPS } from "../flow";
import { useProject } from "../store/project";
import { Icon } from "./Icon";

// Solution-Explorer style tree of the single project context.
export function Explorer() {
  const { project, close } = useProject();
  const [refresh, setRefresh] = useState(0);
  const nav = useNavigate();
  const loc = useLocation();
  if (!project) return null;
  const current = stepIndex(project.stage);
  const reload = project.version + refresh;
  const pages = project.design?.pages ?? [];
  const tasks = project.tasks ?? [];

  const closeProject = async () => {
    await close();
  };

  return (
    <aside className="explorer">
      <div className="explorer-title">
        <span>EXPLORER</span>
        <button className="icon-btn" title="Tutup Project" onClick={closeProject}><Icon name="close" size={14} /></button>
      </div>
      <div className="explorer-project" title={project.dir}>{project.name.toUpperCase()}</div>

      <Section title="Files" defaultOpen
        actions={<button className="icon-btn" title="Muat ulang file" onClick={() => setRefresh(refresh + 1)}>↻</button>}>
        <FileTree reload={reload} />
      </Section>

      <Section title="Pipeline" defaultOpen>
        {STEPS.map((s, i) => {
          const ok = reachable(project.stage, i);
          return (
            <Leaf key={s.key} active={loc.pathname === s.path} disabled={!ok} onClick={() => nav(s.path)}>
              <span className="leaf-mark">{i < current ? "✓" : i === current ? "●" : "○"}</span> {s.label}
            </Leaf>
          );
        })}
      </Section>

      <Section title="Kebutuhan" count={project.requirements?.length}>
        {(project.requirements ?? []).map((r) => (
          <Leaf key={r.id} title={r.text} onClick={() => nav("/simple/rancang?tab=ringkasan")}>
            <span className="leaf-id">{r.id}</span> {r.text}
          </Leaf>
        ))}
      </Section>

      <Section title="Fitur" count={project.features?.length}>
        {(project.features ?? []).map((f) => (
          <Section key={f.id} title={f.name} nested>
            {f.sub_features.map((s) => (
              <Leaf key={s.id} onClick={() => nav("/simple/rancang?tab=fitur")}>{s.name}</Leaf>
            ))}
          </Section>
        ))}
      </Section>

      <Section title="Halaman" count={pages.length}>
        {pages.map((p) => (
          <Leaf key={p.id} active={loc.search === `?page=${p.id}`} onClick={() => nav(`/simple/rancang?page=${p.id}`)}>
            <span className="leaf-id">{p.platform}</span> {p.name}
          </Leaf>
        ))}
      </Section>

      <Section title="Task" count={tasks.length}>
        {COLUMNS.map((col) => {
          const items = tasks.filter((t) => t.status === col.status);
          if (!items.length) return null;
          return (
            <Section key={col.status} title={`${col.label} (${items.length})`} nested defaultOpen={col.status !== "DONE"}>
              {items.map((t) => (
                <Leaf key={t.id} title={t.title} active={loc.search === `?task=${t.id}`} onClick={() => nav(`/simple/bangun?task=${t.id}`)}>
                  <span className="leaf-id">{t.id}</span> {t.title}
                </Leaf>
              ))}
            </Section>
          );
        })}
      </Section>
    </aside>
  );
}

function Section({ title, count, nested, defaultOpen, actions, children }: {
  title: string; count?: number; nested?: boolean; defaultOpen?: boolean; actions?: ReactNode; children: ReactNode;
}) {
  const [open, setOpen] = useState(!!defaultOpen);
  return (
    <div className={nested ? "tree-node nested" : "tree-section"}>
      <div className="tree-head-row">
        <button className="tree-head" onClick={() => setOpen(!open)}>
          <span className="caret">{open ? "▾" : "▸"}</span> {nested ? title : title.toUpperCase()}
          {count !== undefined && <span className="tree-count">{count}</span>}
        </button>
        {actions && <span className="tree-actions">{actions}</span>}
      </div>
      {open && <div className="tree-children">{children}</div>}
    </div>
  );
}

function Leaf({ children, onClick, active, disabled, title }: {
  children: ReactNode; onClick: () => void; active?: boolean; disabled?: boolean; title?: string;
}) {
  return (
    <button className={`leaf ${active ? "active" : ""}`} disabled={disabled} title={title} onClick={onClick}>
      {children}
    </button>
  );
}
