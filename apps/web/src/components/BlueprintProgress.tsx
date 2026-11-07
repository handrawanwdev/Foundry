import { useEffect, useState } from "react";
import { api } from "../api";
import { isWorking } from "../flow";
import type { BlueprintView, Project } from "../types";
import { Elapsed } from "./Elapsed";

type State = "done" | "running" | "pending" | "skipped";
interface Stage { id: string; title: string; hint: string; state: State }

// Areas whose Markdown the AI writes in the "artifacts" job, one area per
// job during the first build (artifact_request.area names it). The rest are
// written by their own step (ui = design job, use cases and acceptance
// come from the feature list).
const WRITTEN_BY_ARTIFACTS = new Set(["data", "security", "integration", "operations", "migration"]);
// Data and integration need depend on the discovery, so before it ran
// "not applicable" only means "not known yet".
const DECIDED_BY_DISCOVERY = new Set(["data", "integration"]);

const cut = (topics?: string[]) => (topics ?? []).slice(0, 4).join(", ");

// Stages of the blueprint package: the areas the blueprint covers for this
// project, then technology, architecture and folders.
export function blueprintStages(p: Project, view: BlueprintView | null): Stage[] {
  const working = isWorking(p);
  const kind = working ? p.job?.kind : undefined;
  const st = p.structure;
  const stack = p.stack;
  const existing = st?.mode === "existing";
  const area = p.artifact_request?.area;
  const out: Stage[] = [{
    id: "discovery", title: "Penjelasan sistem",
    hint: "Ringkasan, pengguna dan peran, data, integrasi, keamanan.",
    state: kind === "discovery" ? "running" : p.discovery ? "done" : "pending",
  }];

  const areas = [...(view?.areas ?? [])].sort((a, b) => Number(a.id === "ui") - Number(b.id === "ui"));
  for (const a of areas) {
    const unknown = !p.discovery && DECIDED_BY_DISCOVERY.has(a.id);
    if (a.status === "not_applicable" && !unknown) continue;
    let state: State = "pending";
    if (a.status === "skipped") state = "skipped";
    else if (a.status === "covered") state = "done";
    else if (a.id === "ui" && kind === "design") state = "running";
    else if (WRITTEN_BY_ARTIFACTS.has(a.id) && kind === "artifacts" && (!area || area === a.id)) state = "running";
    out.push({ id: a.id, title: a.title, hint: cut(a.topics), state });
  }

  const planning = kind === "plan";
  // The open section is the first one still requested.
  const techOpen = !!stack?.plan_request;
  const archOpen = !!st?.plan_request;
  const repoOpen = !!st?.repo_request;
  const first = techOpen ? "tech" : archOpen ? "arch" : repoOpen ? "repo" : "";
  const stateOf = (id: string, landed: boolean, open: boolean): State =>
    planning && first === id ? "running" : landed && !open ? "done" : "pending";
  out.push({
    id: "tech", title: "Teknologi", hint: "Bahasa, framework, database, dan alat yang dipakai.",
    state: stateOf("tech", !!stack?.plan, techOpen),
  });
  if (!existing) {
    out.push({ id: "arch", title: "Arsitektur", hint: "Pola aplikasi dan pembagian modul.", state: stateOf("arch", !!st?.plan, archOpen) });
    out.push({ id: "repo", title: "Struktur repository", hint: "Folder dan file yang akan dibuat.", state: stateOf("repo", !!st?.repo, repoOpen) });
  }
  return out;
}

// True while the first blueprint is being built: the discovery job, then the
// queued design and plan jobs, until technology, architecture and folders
// have all landed. Later regenerations keep the document view.
export function buildingInitialBlueprint(p: Project): boolean {
  if (!isWorking(p)) return false;
  if (!p.discovery) return p.job?.kind === "discovery";
  if (!p.blueprint_queue?.length) return false;
  const existing = p.structure?.mode === "existing";
  return !(p.stack?.plan && (existing || p.structure?.repo));
}

const MARK: Record<State, React.ReactNode> = {
  done: "✓", skipped: "–", pending: "○", running: <span className="spinner" />,
};

// What is being written now and what is next; replaces a bare spinner.
export function BlueprintProgress({ project }: { project: Project }) {
  const [view, setView] = useState<BlueprintView | null>(null);
  useEffect(() => {
    let stale = false;
    api.blueprint().then((v) => { if (!stale) setView(v); }).catch(() => { if (!stale) setView(null); });
    return () => { stale = true; };
  }, [project.version, project.dir]);
  const stages = blueprintStages(project, view);
  const running = stages.filter((s) => s.state === "running");
  const counted = stages.filter((s) => s.state !== "skipped");
  const done = counted.filter((s) => s.state === "done").length;
  return (
    <div className="bp-progress">
      <h1><span className="spinner" /> Merancang Blueprint <Elapsed started={project.blueprint_started ?? project.job?.started} /></h1>
      <p className="muted">
        {running.length
          ? <>Sedang menyusun <strong>{running.map((s) => s.title).join(", ")}</strong>.</>
          : "Menyiapkan rancangan…"}
        {" "}({done}/{counted.length} bagian selesai)
      </p>
      <ol className="bp-stages">
        {stages.map((s) => (
          <li key={s.id} className={`bp-stage ${s.state}`} aria-current={s.state === "running" ? "step" : undefined}>
            <span className="bp-mark" aria-hidden>{MARK[s.state]}</span>
            <span className="bp-text"><strong>{s.title}</strong>{s.hint && <span className="muted small"> {s.hint}</span>}</span>
          </li>
        ))}
      </ol>
    </div>
  );
}
