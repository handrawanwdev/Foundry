import { useEffect, useRef, type ReactElement } from "react";
import { Navigate, Route, Routes, useLocation, useNavigate } from "react-router-dom";
import { Workbench } from "./components/Workbench";
import { stepFor, stepIndex, STEPS } from "./flow";
import { ConnectPage } from "./pages/Connect";
import { DesignPage } from "./pages/Design";
import { EmptyEditor, StartPage } from "./pages/Start";
import { TellPage } from "./pages/Tell";
import { useAI } from "./store/ai";
import { useProject } from "./store/project";
import { useWorkbench } from "./store/workbench";

export default function App() {
  const ai = useAI();
  const { project, loaded, load, subscribe } = useProject();
  const closeTabs = useWorkbench((s) => s.closeAll);
  const nav = useNavigate();
  const loc = useLocation();

  useEffect(() => {
    void ai.load();
    void load();
    return subscribe();
  }, []);

  // While waiting for an MCP host, poll the connection so the UI flips to ready.
  const waitingHost = ai.state?.connection?.mode === "mcp" && !ai.state.ready;
  useEffect(() => {
    if (!waitingHost) return;
    const t = setInterval(() => void ai.load(), 2000);
    return () => clearInterval(t);
  }, [waitingHost]);

  // Opening a project lands on its current step (or AI connect); closing it
  // returns to the Start page. On first load a deep link is kept.
  const lastDir = useRef<string | null>(null);
  useEffect(() => {
    if (!loaded || ai.loading) return;
    const dir = project?.dir ?? "";
    const first = lastDir.current === null;
    if (dir !== lastDir.current) {
      if (!first) closeTabs();
      if (!dir) {
        if (!first) nav("/");
      } else if (!first || loc.pathname === "/") {
        nav(ai.state?.ready ? stepFor(project!.stage).path : "/simple/connect", { replace: first });
      }
    }
    lastDir.current = dir;
  }, [project?.dir, loaded, ai.loading]);

  // Move the user forward automatically when the pipeline advances.
  const lastStep = useRef<number | null>(null);
  useEffect(() => {
    if (!project) {
      lastStep.current = null;
      return;
    }
    const step = stepIndex(project.stage);
    if (lastStep.current !== null && step > lastStep.current) nav(STEPS[step].path);
    lastStep.current = step;
  }, [project?.stage, project?.dir]);

  if (ai.loading || !loaded) {
    return <div className="boot">Memuat Foundry…</div>;
  }

  const ready = !!ai.state?.ready;
  const guard = (el: ReactElement) => {
    if (!project) return <Navigate to="/" replace />;
    return ready ? el : <Navigate to="/simple/connect" replace state={{ from: loc.pathname }} />;
  };

  return (
    <Workbench>
      <Routes>
        <Route path="/" element={project ? <EmptyEditor /> : <StartPage />} />
        <Route path="/simple/connect" element={<ConnectPage />} />
        <Route path="/simple/tell" element={guard(<TellPage />)} />
        <Route path="/simple/rancang" element={guard(<DesignPage />)} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </Workbench>
  );
}
