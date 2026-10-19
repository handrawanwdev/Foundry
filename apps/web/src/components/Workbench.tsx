import { useEffect, useRef, useState, type ReactNode } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { reachable, stepFor, STEPS, viewOf, VIEWS } from "../flow";
import { agentName, agentOptions, useAI } from "../store/ai";
import { useProject } from "../store/project";
import { useWorkbench } from "../store/workbench";
import { Explorer } from "./Explorer";
import { Icon } from "./Icon";
import { JobBanner } from "./JobBanner";
import { ProjectDialog } from "./ProjectDialog";

// VS-style layout: menu bar, activity bar, explorer, tabbed editor, status bar.
export function Workbench({ children }: { children: ReactNode }) {
  const { project, error, clearError } = useProject();
  const wb = useWorkbench();
  const loc = useLocation();
  const nav = useNavigate();
  const isView = !!viewOf(loc.pathname);

  // Every visited view becomes an editor tab.
  useEffect(() => {
    if (isView) wb.openTab(loc.pathname);
  }, [loc.pathname]);

  const closeTab = (path: string) => {
    const next = wb.closeTab(path);
    if (path === loc.pathname) nav(next ?? "/");
  };

  return (
    <div className="wb">
      <MenuBar />
      <div className="wb-main">
        <ActivityBar />
        {wb.sidebar && project && <Explorer />}
        <div className="editor">
          {wb.tabs.length > 0 && (
            <div className="tabbar">
              {wb.tabs.map((t) => {
                const v = viewOf(t);
                return (
                <div key={t} className={`tab-item ${t === loc.pathname ? "active" : ""}`} onClick={() => nav(t)}
                  title={v?.title} onAuxClick={(e) => e.button === 1 && closeTab(t)}>
                  <Icon name={v?.icon ?? "files"} size={14} />
                  <span>{v?.label ?? t}</span>
                  <button className="tab-close" title="Tutup" onClick={(e) => { e.stopPropagation(); closeTab(t); }}>
                    <Icon name="close" size={12} />
                  </button>
                </div>
                );
              })}
            </div>
          )}
          {project && VIEWS[loc.pathname] && <JobBanner />}
          <div className="editor-content">{children}</div>
        </div>
      </div>
      <StatusBar />
      <ProjectDialog />
      {error && (
        <div className="toast error" onClick={clearError}>
          {error} <span className="muted">(tutup)</span>
        </div>
      )}
    </div>
  );
}

// ---------- Menu bar ----------

type MenuItem = { label: string; hint?: string; action?: () => void; disabled?: boolean } | "sep";

function MenuBar() {
  const { project, recent, open, close } = useProject();
  const wb = useWorkbench();
  const nav = useNavigate();
  const [openMenu, setOpenMenu] = useState<string | null>(null);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const onDown = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpenMenu(null);
    };
    document.addEventListener("mousedown", onDown);
    return () => document.removeEventListener("mousedown", onDown);
  }, []);

  const openRecent = async (dir: string) => {
    await open(dir);
  };
  const closeProject = async () => {
    await close();
  };

  const menus: Record<string, MenuItem[]> = {
    File: [
      { label: "Project Baru…", action: () => wb.setDialog("new") },
      { label: "Buka Project…", action: () => wb.setDialog("open") },
      "sep",
      ...recent.slice(0, 6).map((d) => ({ label: d, hint: "terakhir", action: () => void openRecent(d), disabled: d === project?.dir })),
      ...(recent.length ? ["sep" as const] : []),
      { label: "Tutup Project", action: () => void closeProject(), disabled: !project },
    ],
    View: [
      { label: wb.sidebar ? "Sembunyikan Explorer" : "Tampilkan Explorer", action: wb.toggleSidebar, disabled: !project },
      "sep",
      ...STEPS.map((s, i) => ({ label: s.label, action: () => nav(s.path), disabled: !project || !reachable(project.stage, i) })),
    ],
    AI: [{ label: "Hubungkan AI…", action: () => nav("/simple/connect") }],
  };

  return (
    <div className="menubar" ref={ref}>
      <span className="menubar-logo">◆</span>
      {Object.entries(menus).map(([name, items]) => (
        <div key={name} className="menu">
          <button className={`menu-btn ${openMenu === name ? "open" : ""}`}
            onClick={() => setOpenMenu(openMenu === name ? null : name)}
            onMouseEnter={() => openMenu && setOpenMenu(name)}>
            {name}
          </button>
          {openMenu === name && (
            <div className="menu-drop">
              {items.map((it, i) => it === "sep" ? <div key={i} className="menu-sep" /> : (
                <button key={i} className="menu-item" disabled={it.disabled}
                  onClick={() => { setOpenMenu(null); it.action?.(); }}>
                  <span className="menu-label">{it.label}</span>
                  {it.hint && <span className="muted small">{it.hint}</span>}
                </button>
              ))}
            </div>
          )}
        </div>
      ))}
      <div className="menubar-title">{project ? `${project.name} — Foundry` : "Foundry"}</div>
    </div>
  );
}

// ---------- Activity bar ----------

function ActivityBar() {
  const { project } = useProject();
  const wb = useWorkbench();
  const loc = useLocation();
  const nav = useNavigate();
  return (
    <div className="activitybar">
      <button className={`act ${wb.sidebar && project ? "active" : ""}`} title="Explorer" disabled={!project} onClick={wb.toggleSidebar}>
        <Icon name="files" />
      </button>
      {STEPS.map((s, i) => {
        const ok = !!project && reachable(project.stage, i);
        return (
          <button key={s.key} className={`act ${loc.pathname === s.path ? "active" : ""}`} title={s.label} disabled={!ok} onClick={() => nav(s.path)}>
            <Icon name={VIEWS[s.path].icon} />
          </button>
        );
      })}
      <div className="grow" />
      <button className={`act ${loc.pathname === "/simple/connect" ? "active" : ""}`} title="Hubungkan AI" onClick={() => nav("/simple/connect")}>
        <Icon name="plug" />
      </button>
    </div>
  );
}

// ---------- Status bar ----------

function StatusBar() {
  const ai = useAI((s) => s.state);
  const project = useProject((s) => s.project);
  const nav = useNavigate();
  const job = project?.job;
  return (
    <div className={`statusbar ${project ? "" : "noproject"}`}>
      <button className="sb-item" onClick={() => nav("/simple/connect")}>
        {ai?.ready ? `⚡ ${agentName(ai)}${agentOptions(ai) ? ` · ${agentOptions(ai)}` : ""}` : "⚠ AI belum terhubung"}
      </button>
      {project && <span className="sb-item">{stepFor(project.stage).label}{project.stage === "READY" ? " · Final" : ""}</span>}
      {job && job.status !== "done" && (
        <span className="sb-item">
          {job.status === "failed" ? `✗ ${job.message} gagal` : <><span className="spinner light" /> {job.message}</>}
        </span>
      )}
      <div className="grow" />
      {project && <span className="sb-item path" title={project.dir}>{project.dir}</span>}
    </div>
  );
}
