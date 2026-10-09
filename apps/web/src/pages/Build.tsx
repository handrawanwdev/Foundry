import { useEffect, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { api } from "../api";
import { Kanban } from "../components/Kanban";
import { IssueList } from "../components/IssueList";
import { isWorking, queueItem, queueLabel, taskCounts } from "../flow";
import { AgentsField } from "../components/AgentsField";
import { useAI } from "../store/ai";
import { useProject } from "../store/project";
import type { PRDFinding, Project, Task } from "../types";

export function BuildPage() {
  const { project, run, pending } = useProject();
  const { state: ai, setAgents } = useAI();
  const parallel = ai?.parallel;
  const [params] = useSearchParams();
  const [selected, setSelected] = useState<string | undefined>(params.get("task") ?? undefined);
  const [showPRD, setShowPRD] = useState(params.get("prd") === "1");
  useEffect(() => {
    const t = params.get("task");
    if (t) setSelected(t);
    if (params.get("prd") === "1") setShowPRD(true);
  }, [params]);
  if (!project) return null;

  const tasks = project.tasks ?? [];
  if (tasks.length === 0) {
    return (
      <section className="card narrow center-text">
        {isWorking(project) ? <h1><span className="spinner" /> Membuat fitur, sub-fitur, dan task</h1>
          : <><h1>Belum ada task</h1><p><Link to="/simple/rancang">Setujui rancangan dulu</Link></p></>}
      </section>
    );
  }

  const c = taskCounts(project);
  const working = isWorking(project);
  const v = project.verification;
  const task = tasks.find((t) => t.id === selected);
  const independent = project.schedule?.independent ?? [];

  return (
    <div className="build">
      <div className="build-head">
        <div className="progress">
          <div className="bar"><div style={{ transform: `scaleX(${c.DONE / tasks.length})` }} /></div>
          <span className="small">{c.DONE}/{tasks.length} task selesai{c.BLOCKED ? ` · ${c.BLOCKED} terblokir` : ""}{independent.length > 0 ? ` · ${independent.length} siap paralel` : ""}</span>
        </div>
        <div className="row">
          {parallel && (
            <AgentsField value={parallel.agents} max={parallel.max} fromEnv={parallel.from_env} onChange={(n) => void setAgents(n)} />
          )}
          {!working && c.TODO > 0 && (
            <button className="primary" disabled={pending === "build"} onClick={() => run("build", api.startBuild)}>
              {c.DONE > 0 ? "Lanjutkan Bangun" : "Mulai Bangun"}
            </button>
          )}
          {!working && (
            <button disabled={pending === "prd"} onClick={() => void run("prd", api.generatePRD).then((ok) => { if (ok) setShowPRD(true); })}>
              {project.prd ? "Buat ulang PRD" : "Generate PRD"}
            </button>
          )}
          {project.prd && (
            <>
              <button className={showPRD ? "primary" : ""} onClick={() => setShowPRD(!showPRD)}>
                {showPRD ? "Tutup PRD" : "Lihat PRD"}
              </button>
              <a className="button" href="/api/v1/project/prd.md" download>Unduh PRD</a>
            </>
          )}
          {!working && (
            <button disabled={pending === "verify"} onClick={() => run("verify", api.verify)}>Verifikasi</button>
          )}
        </div>
      </div>

      {v && !v.ready && !working && (
        <div className="card warn-card">
          <strong>⚠ Masih ada pekerjaan</strong>
          <IssueList v={v} />
          <button className="primary" disabled={pending === "fix"} onClick={() => run("fix", api.fix)}>Perbaiki Otomatis</button>
        </div>
      )}
      {v?.ready && <div className="card ok-card">🎉 Semua terverifikasi. <Link to="/simple/siap">Lihat hasil</Link></div>}

      {showPRD && project.prd && <PRDPanel prd={project.prd} />}

      <div className={task ? "split build-split" : ""}>
        <Kanban project={project} selected={selected} onSelect={setSelected} />
        {task && <TaskDetail project={project} task={task} onClose={() => setSelected(undefined)} />}
      </div>
    </div>
  );
}

const FINDING_LABEL: Record<PRDFinding["kind"], string> = {
  GAP: "tanpa requirement",
  MISSING_TASK: "belum ada task",
  DUPLICATE: "tumpang tindih",
  OVERLAP: "berbagi file/halaman",
  INVALID_DEPENDENCY: "dependensi salah",
};

function PRDPanel({ prd }: { prd: NonNullable<Project["prd"]> }) {
  const findings = prd.findings ?? [];
  return (
    <section className="card prd-panel">
      <div className="row between">
        <h3>PRD</h3>
        <div className="row">
          <span className="muted small">{findings.length} temuan</span>
          <a className="button" href="/api/v1/project/prd.md" download>Unduh .md</a>
        </div>
      </div>
      {findings.length > 0 && (
        <ul className="small prd-findings">
          {findings.map((f, i) => (
            <li key={i}><span className="tag">{f.kind}</span> {f.ref} {f.message || FINDING_LABEL[f.kind]}</li>
          ))}
        </ul>
      )}
      <pre className="doc">{prd.markdown}</pre>
    </section>
  );
}

function TaskDetail({ project, task, onClose }: { project: Project; task: Task; onClose: () => void }) {
  const { run } = useProject();
  const build = project.builds?.find((b) => b.id === task.last_build_id);
  const reqs = (task.requirement_ids ?? []).map((id) => project.requirements?.find((r) => r.id === id)).filter(Boolean);
  const pages = (task.page_ids ?? []).map((id) => project.design?.pages?.find((p) => p.id === id)?.name ?? id);
  const move = (s: Task["status"]) => run("move", () => api.moveTask(task.id, s));
  const waiting = (task.depends_on ?? []).filter((id) => project.tasks?.find((t) => t.id === id)?.status !== "DONE");
  const queue = task.status === "TODO" ? queueLabel(queueItem(project, task.id)) : "";

  return (
    <aside className="card detail">
      <div className="row between">
        <span className="tag">{task.id} · {task.status}</span>
        <button className="link" onClick={onClose}>tutup</button>
      </div>
      <h3>{task.title}</h3>
      <p className="small pre">{task.description}</p>
      {reqs.map((r) => <div key={r!.id} className="small"><span className="tag">{r!.id}</span> {r!.text}</div>)}
      {pages.length > 0 && <div className="small">Halaman: {pages.join(", ")}</div>}
      {waiting.length > 0 && <div className="small">Menunggu: {waiting.join(", ")}</div>}
      {queue && <div className="small">{queue}</div>}
      {task.status === "IN_PROGRESS" && task.owner && <div className="small">Dikerjakan {task.owner}</div>}
      {task.progress && (task.status === "IN_PROGRESS" || task.status === "REVIEW") && (
        <div className="task-progress">{task.status === "IN_PROGRESS" && <span className="spinner" />} {task.progress}</div>
      )}

      <h4>Acceptance criteria</h4>
      <ul className="small">
        {task.acceptance_criteria.map((a, i) => {
          const c = task.criteria?.[i];
          return <li key={i}>{c ? (c.met ? "✓ " : "✗ ") : ""}{a}{c?.evidence && <div className="muted">{c.evidence}</div>}</li>;
        })}
      </ul>
      {(task.test_requirements ?? []).length > 0 && (<><h4>Test</h4><ul className="small">{task.test_requirements!.map((t) => <li key={t}>{t}</li>)}</ul></>)}
      {(task.files ?? []).length > 0 && <div className="small muted">File: {task.files!.join(", ")}</div>}

      {(task.notes ?? []).length > 0 && (
        <><h4>Catatan</h4>{task.notes!.map((n, i) => <pre key={i} className="code small">{n}</pre>)}</>
      )}

      {build && (
        <>
          <h4>Verifikasi {build.passed ? "✓ lulus" : "✗ gagal"}</h4>
          {(build.steps ?? []).map((s, i) => (
            <details key={i} className="small">
              <summary>{s.exit_code === 0 ? "✓" : "✗"} {s.command} <span className="muted">({s.dir}, {s.millis} ms)</span></summary>
              <pre className="code">{s.output || "(tanpa output)"}</pre>
            </details>
          ))}
        </>
      )}

      <div className="row">
        {task.status === "REVIEW" && build?.passed && <button className="primary" onClick={() => move("DONE")}>Setujui (DONE)</button>}
        {(task.status === "BLOCKED" || task.status === "DONE" || task.status === "REVIEW") && <button onClick={() => move("TODO")}>Kerjakan ulang</button>}
        {task.status === "TODO" && <button className="ghost" onClick={() => move("BLOCKED")}>Tandai terblokir</button>}
      </div>
    </aside>
  );
}
