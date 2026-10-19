import type { ReactNode } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { reachable, stepOfPath, STEPS } from "../flow";
import { agentName, agentOptions, useAI } from "../store/ai";
import { useProject } from "../store/project";
import { useWorkbench } from "../store/workbench";
import { JobBanner } from "./JobBanner";
import { Menu } from "./Menu";
import { ProjectDialog } from "./ProjectDialog";

// App shell: one slim top bar and the page below it.
export function Workbench({ children }: { children: ReactNode }) {
  const { project, error, clearError } = useProject();
  const loc = useLocation();
  return (
    <div className="app">
      <TopBar />
      {project && loc.pathname !== "/" && loc.pathname !== "/simple/connect" && <JobBanner />}
      <main className="stage">{children}</main>
      <ProjectDialog />
      {error && (
        <div className="toast error" onClick={clearError}>
          {error} <span className="muted">(tutup)</span>
        </div>
      )}
    </div>
  );
}

function TopBar() {
  const { project } = useProject();
  const ai = useAI((s) => s.state);
  const loc = useLocation();
  const nav = useNavigate();
  return (
    <header className="topbar">
      <button className="brand" onClick={() => nav("/")} aria-label="Foundry">
        <span className="brand-mark" aria-hidden>◆</span>
        <span className="brand-name">Foundry</span>
      </button>
      <ProjectMenu />
      {project && (
        <nav className="steps" aria-label="Langkah">
          {STEPS.map((s, i) => {
            const current = stepOfPath(loc.pathname) === i;
            return (
              <button key={s.key} className={`step ${current ? "current" : ""}`} disabled={!reachable(project.stage, i)}
                aria-current={current ? "step" : undefined} onClick={() => nav(s.path)}>
                <span className="step-n">{i + 1}</span>
                <span className="step-label">{s.label}</span>
              </button>
            );
          })}
        </nav>
      )}
      <div className="grow" />
      <button className={`ai-chip ${ai?.ready ? "ok" : "off"}`} onClick={() => nav("/simple/connect")}
        title={ai?.ready ? "Pengaturan AI" : "Hubungkan AI"}>
        <span className="ai-dot" aria-hidden />
        {ai?.ready ? `${agentName(ai)}${agentOptions(ai) ? ` · ${agentOptions(ai)}` : ""}` : "Hubungkan AI"}
      </button>
    </header>
  );
}

function ProjectMenu() {
  const { project, recent, open, close } = useProject();
  const setDialog = useWorkbench((s) => s.setDialog);
  return (
    <Menu className="project-btn" trigger={<span className="project-name" title={project?.dir}>{project?.name ?? "Pilih project"}</span>}>
      {(done) => (
        <>
          <button className="menu-item" onClick={() => { done(); setDialog("new"); }}>Project baru…</button>
          <button className="menu-item" onClick={() => { done(); setDialog("open"); }}>Buka project…</button>
          {recent.length > 0 && <div className="menu-sep" />}
          {recent.length > 0 && <div className="menu-label">Terakhir</div>}
          {recent.slice(0, 6).map((d) => (
            <button key={d} className="menu-item" disabled={d === project?.dir} title={d} onClick={() => { done(); void open(d); }}>
              <span className="menu-text">{d.split("/").filter(Boolean).pop()}</span>
            </button>
          ))}
          {project && <div className="menu-sep" />}
          {project && <button className="menu-item" onClick={() => { done(); void close(); }}>Tutup project</button>}
        </>
      )}
    </Menu>
  );
}
