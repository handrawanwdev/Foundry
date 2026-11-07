import { useEffect, useRef, useState } from "react";
import { api } from "../api";
import { isWorking } from "../flow";
import { useProject } from "../store/project";
import type { BlueprintSectionStatus, BlueprintView, Project } from "../types";
import { Elapsed } from "./Elapsed";

type State = "done" | "running" | "pending" | "skipped" | "failed" | "retrying";
interface Stage { id: string; title: string; hint: string; state: State; activity?: string; error?: string }

const WRITTEN_BY_ARTIFACTS = new Set(["data", "security", "integration", "operations", "migration"]);
const DECIDED_BY_DISCOVERY = new Set(["data", "integration"]);
const ARTIFACT_JOB = new Set(["blueprint", "artifacts"]);

const cut = (topics?: string[]) => (topics ?? []).slice(0, 4).join(", ");

const fromSection = (s: BlueprintSectionStatus): State => {
  if (s === "in_progress") return "running";
  if (s === "completed") return "done";
  return s;
};

export function blueprintStages(p: Project, view: BlueprintView | null): Stage[] {
  if (p.blueprint_progress?.sections?.length) {
    return p.blueprint_progress.sections.map((s) => ({
      id: s.id, title: s.name, hint: s.activity || "", state: fromSection(s.status),
      activity: s.activity, error: s.error,
    }));
  }
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
    else if (WRITTEN_BY_ARTIFACTS.has(a.id) && ARTIFACT_JOB.has(kind ?? "") && (!area || area === a.id)) state = "running";
    out.push({ id: a.id, title: a.title, hint: cut(a.topics), state });
  }

  const planning = kind === "plan";
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

export function buildingInitialBlueprint(p: Project): boolean {
  const bp = p.blueprint_progress;
  if (bp?.status === "running") return true;
  if (bp?.status === "failed" && p.job?.status === "failed") return true;
  if (!isWorking(p)) return false;
  if (!p.discovery) return p.job?.kind === "discovery";
  if (!p.blueprint_queue?.length) return false;
  const existing = p.structure?.mode === "existing";
  return !(p.stack?.plan && (existing || p.structure?.repo));
}

const MARK: Record<State, React.ReactNode> = {
  done: "✓", skipped: "–", pending: "○", failed: "!", retrying: "↻",
  running: <span className="spinner" />,
};

const STATE_LABEL: Record<State, string> = {
  done: "Selesai",
  skipped: "Dilewati",
  pending: "Menunggu",
  failed: "Gagal",
  retrying: "Mencoba kembali",
  running: "Sedang dikerjakan",
};

function pct(done: number, total: number): string {
  if (!total) return "0%";
  return `${(done / total * 100).toLocaleString("id-ID", { maximumFractionDigits: 1 })}%`;
}

export function BlueprintProgress({ project }: { project: Project }) {
  const { run, pending, stream } = useProject();
  const [view, setView] = useState<BlueprintView | null>(null);
  const [logOpen, setLogOpen] = useState(true);
  const activeRef = useRef<HTMLLIElement | null>(null);
  const bp = project.blueprint_progress;

  useEffect(() => {
    if (bp?.sections?.length) return;
    let stale = false;
    api.blueprint().then((v) => { if (!stale) setView(v); }).catch(() => { if (!stale) setView(null); });
    return () => { stale = true; };
  }, [project.version, project.dir, bp?.sections?.length]);

  const stages = blueprintStages(project, view);
  const running = stages.filter((s) => s.state === "running" || s.state === "retrying");
  const counted = stages.filter((s) => s.state !== "skipped");
  const done = bp ? bp.sections.filter((s) => s.status === "completed").length : counted.filter((s) => s.state === "done").length;
  const total = bp ? bp.sections.filter((s) => s.status !== "skipped").length : counted.length;
  const failed = bp?.status === "failed" || project.job?.status === "failed";
  const rawActivity = bp?.activity || "";
  const activity = rawActivity
    ? /^(AI |Menunggu|Server)/.test(rawActivity) ? rawActivity : `AI sedang ${rawActivity.charAt(0).toLowerCase()}${rawActivity.slice(1)}.`
    : running.length ? `AI sedang menyusun ${running.map((s) => s.title).join(", ")}.` : "Menyiapkan rancangan…";
  const started = bp?.started_at ?? project.blueprint_started ?? project.job?.started;
  const activities = bp?.activities ?? [];

  useEffect(() => {
    activeRef.current?.scrollIntoView({ block: "nearest", behavior: "smooth" });
  }, [running[0]?.id, activity]);

  return (
    <div className="bp-progress">
      {stream === "reconnecting" && <p className="bp-reconnect" role="status">Menyambungkan ulang…</p>}
      <header className="bp-head">
        <h1>
          {failed ? null : <span className="spinner" />}
          {failed ? "Perancangan Blueprint gagal" : "Merancang Blueprint"}
        </h1>
        <p className="bp-activity-now">{activity}</p>
        <p className="muted">
          {done}/{total} bagian selesai{total ? ` · ${pct(done, total)}` : ""} · Berjalan <Elapsed started={started} />
        </p>
        <div className="bp-bar" aria-hidden>
          <span style={{ width: total ? `${(done / total) * 100}%` : "0%" }} />
        </div>
      </header>
      <ol className="bp-stages">
        {stages.map((s, i) => (
          <li key={s.id} ref={s.state === "running" || s.state === "retrying" ? activeRef : undefined}
            className={`bp-stage ${s.state}`} aria-current={s.state === "running" || s.state === "retrying" ? "step" : undefined}>
            <span className="bp-mark" aria-hidden>{MARK[s.state]}</span>
            <span className="bp-text">
              <strong>{i + 1}. {s.title}</strong>
              <span className="bp-state muted small"> {STATE_LABEL[s.state]}</span>
              {(s.state === "running" || s.state === "retrying") && s.activity && (
                <span className="bp-sub muted small">└ {s.activity}</span>
              )}
              {s.state === "failed" && s.error && <span className="bp-sub bp-err small">{s.error}</span>}
              {s.state !== "running" && s.state !== "retrying" && s.state !== "failed" && s.hint && (
                <span className="muted small"> {s.hint}</span>
              )}
            </span>
          </li>
        ))}
      </ol>
      {activities.length > 0 && (
        <details className="bp-log" open={logOpen} onToggle={(e) => setLogOpen((e.target as HTMLDetailsElement).open)}>
          <summary>Aktivitas AI</summary>
          <ol>
            {activities.slice().reverse().map((a) => (
              <li key={a.seq}>{a.text}</li>
            ))}
          </ol>
        </details>
      )}
      {failed && (
        <div className="bp-fail">
          <p className="bp-err">{bp?.error || project.job?.error || "Proses gagal."}</p>
          <button className="primary" disabled={pending === "retry"} onClick={() => void run("retry", api.retry)}>Coba lagi</button>
        </div>
      )}
    </div>
  );
}
