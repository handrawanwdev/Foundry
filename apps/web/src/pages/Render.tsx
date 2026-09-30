import { useEffect, useState } from "react";
import { api } from "../api";
import { Wireframe } from "../components/Wireframe";
import { DEVICES, SCREEN_WIDTH, type Device } from "../design";
import { screenOutline } from "../screen";
import type { Project } from "../types";

// Bare page for Foundry's visual check: headless Chrome opens
// /render?page=PAGE-001&device=mobile, takes a screenshot and reads the
// outline below. Same Wireframe as the design workspace, nothing around it.
export default function RenderPage() {
  const params = new URLSearchParams(location.search);
  const id = params.get("page") ?? "";
  const q = params.get("device") as Device;
  const [project, setProject] = useState<Project | null>(null);
  const [error, setError] = useState("");
  useEffect(() => { api.project().then(setProject, (e) => setError(String(e))); }, []);

  const pages = project?.design?.pages ?? [];
  const page = pages.find((p) => p.id === id);
  if (error || (project && !page)) return <p>{error || `Halaman ${id} tidak ada`}</p>;
  if (!project || !page) return null;
  const device: Device = DEVICES.some((d) => d.key === q) ? q : "desktop";
  return (
    <div className="render-page" style={{ width: SCREEN_WIDTH[device] }}>
      <Wireframe app={project.name} page={page} pages={pages} device={device} mode="run" onNavigate={() => {}} system={project.design?.system} />
      <pre id="outline" data-dir={project.dir} hidden>{screenOutline(page, pages, project.name, device, project.design?.system)}</pre>
    </div>
  );
}
