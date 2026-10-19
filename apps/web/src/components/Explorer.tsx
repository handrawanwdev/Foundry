import { useState, type ReactNode } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { reachable, stepIndex, STEPS } from "../flow";
import { useProject } from "../store/project";
import { Icon } from "./Icon";

// Solution-Explorer style tree of the single project context.
export function Explorer() {
  const { project, close } = useProject();
  const nav = useNavigate();
  const loc = useLocation();
  if (!project) return null;
  const current = stepIndex(project.stage);
  const pages = project.design?.pages ?? [];

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

      <Section title="Modul" count={project.blueprint?.modules?.length}>
        {(project.blueprint?.modules ?? []).map((m) => (
          <Leaf key={m.id} title={m.responsibility} onClick={() => nav("/simple/rancang?tab=arsitektur")}>
            <span className="leaf-id">{m.id}</span> {m.responsibility}
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

    </aside>
  );
}

function Section({ title, count, nested, defaultOpen, children }: {
  title: string; count?: number; nested?: boolean; defaultOpen?: boolean; children: ReactNode;
}) {
  const [open, setOpen] = useState(!!defaultOpen);
  return (
    <div className={nested ? "tree-node nested" : "tree-section"}>
      <div className="tree-head-row">
        <button className="tree-head" onClick={() => setOpen(!open)}>
          <span className="caret">{open ? "▾" : "▸"}</span> {nested ? title : title.toUpperCase()}
          {count !== undefined && <span className="tree-count">{count}</span>}
        </button>
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
