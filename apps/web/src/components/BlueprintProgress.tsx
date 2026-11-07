import { isWorking } from "../flow";
import type { Project } from "../types";
import { Elapsed } from "./Elapsed";

type State = "done" | "running" | "pending";
interface Stage { id: string; title: string; hint: string; state: State }

// Stages of the blueprint package, derived from what already landed and
// what is being designed now.
export function blueprintStages(p: Project): Stage[] {
  const working = isWorking(p);
  const kind = working ? p.job?.kind : undefined;
  const queue = p.blueprint_queue ?? [];
  const st = p.structure;
  const stack = p.stack;
  const existing = st?.mode === "existing";
  const out: Stage[] = [{
    id: "discovery", title: "Penjelasan sistem",
    hint: "Ringkasan, pengguna dan peran, data, integrasi, keamanan.",
    state: kind === "discovery" ? "running" : p.discovery ? "done" : "pending",
  }];
  if (kind === "artifacts" || queue.includes("artifacts")) {
    out.push({
      id: "artifacts", title: "Dokumen pendukung",
      hint: "Use case, acceptance, dan area lain yang belum lengkap.",
      state: kind === "artifacts" ? "running" : queue[0] === "artifacts" ? "running" : "pending",
    });
  }
  if (kind === "design" || queue.includes("design")) {
    out.push({
      id: "design", title: "Aturan desain & layout",
      hint: "Gaya visual dan susunan halaman.",
      state: kind === "design" ? "running" : p.ui?.rules ? "done" : "pending",
    });
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
    out.push({
      id: "arch", title: "Arsitektur", hint: "Pola aplikasi dan pembagian modul.",
      state: stateOf("arch", !!st?.plan, archOpen),
    });
    out.push({
      id: "repo", title: "Struktur repository", hint: "Folder dan file yang akan dibuat.",
      state: stateOf("repo", !!st?.repo, repoOpen),
    });
  }
  return out;
}

// What is being written now and what is next; replaces a bare spinner.
export function BlueprintProgress({ project }: { project: Project }) {
  const stages = blueprintStages(project);
  const current = stages.find((s) => s.state === "running");
  const done = stages.filter((s) => s.state === "done").length;
  return (
    <div className="bp-progress">
      <h1><span className="spinner" /> Merancang Blueprint <Elapsed started={project.job?.started} /></h1>
      <p className="muted">
        {current ? <>Sedang menyusun <strong>{current.title}</strong>. {current.hint}</> : "Menyiapkan rancangan…"}
        {" "}({done}/{stages.length} bagian selesai)
      </p>
      <ol className="bp-stages">
        {stages.map((s) => (
          <li key={s.id} className={`bp-stage ${s.state}`} aria-current={s.state === "running" ? "step" : undefined}>
            <span className="bp-mark" aria-hidden>{s.state === "done" ? "✓" : s.state === "running" ? <span className="spinner" /> : "○"}</span>
            <span className="bp-text"><strong>{s.title}</strong><span className="muted small"> {s.hint}</span></span>
          </li>
        ))}
      </ol>
    </div>
  );
}
