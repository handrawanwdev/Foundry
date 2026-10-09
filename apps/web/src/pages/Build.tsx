import { useEffect, useRef, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { api } from "../api";
import { Kanban } from "../components/Kanban";
import { IssueList } from "../components/IssueList";
import { isWorking, queueItem, queueLabel, taskCounts } from "../flow";
import { AgentsField } from "../components/AgentsField";
import { Icon } from "../components/Icon";
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
  const [tab, setTab] = useState<"task" | "prd">(params.get("prd") === "1" ? "prd" : "task");
  useEffect(() => {
    const t = params.get("task");
    if (t) {
      setSelected(t);
      setTab("task");
    }
    if (params.get("prd") === "1") {
      setShowPRD(true);
      setTab("prd");
    }
  }, [params]);
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== "Escape") return;
      setSelected(undefined);
      setShowPRD(false);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);
  if (!project) return null;

  const tasks = project.tasks ?? [];
  if (tasks.length === 0) {
    return (
      <div className="build">
        <div className="build-empty">
          {isWorking(project) ? (
            <>
              <span className="spinner" />
              <h1>Membuat fitur, sub-fitur, dan task</h1>
              <p className="muted">Papan muncul setelah plan selesai.</p>
            </>
          ) : (
            <>
              <h1>Belum ada task</h1>
              <p className="muted">Setujui rancangan dulu. Foundry pecah jadi task di papan ini.</p>
              <Link className="button primary" to="/simple/rancang">Ke Rancang</Link>
            </>
          )}
        </div>
      </div>
    );
  }

  const c = taskCounts(project);
  const working = isWorking(project);
  const v = project.verification;
  const task = tasks.find((t) => t.id === selected);
  const independent = project.schedule?.independent ?? [];
  const prdOpen = showPRD && !!project.prd;
  const inspectOpen = !!task || prdOpen;
  const inspectTab = prdOpen && (!task || tab === "prd") ? "prd" : "task";

  const pickTask = (id: string) => {
    setSelected((cur) => (cur === id ? undefined : id));
    setTab("task");
  };
  const closeInspect = () => {
    setSelected(undefined);
    setShowPRD(false);
  };

  return (
    <div className="build">
      <div className="build-toolbar">
        <div className="progress">
          <div className="bar" aria-hidden>
            <div style={{ transform: `scaleX(${c.DONE / tasks.length})` }} />
          </div>
          <span className="small">
            <strong>{c.DONE}</strong>/{tasks.length} selesai
            {c.IN_PROGRESS ? ` · ${c.IN_PROGRESS} dikerjakan` : ""}
            {c.REVIEW ? ` · ${c.REVIEW} review` : ""}
            {c.BLOCKED ? ` · ${c.BLOCKED} terblokir` : ""}
            {independent.length > 0 ? ` · ${independent.length} siap paralel` : ""}
          </span>
        </div>
        <div className="build-actions">
          {parallel && (
            <AgentsField value={parallel.agents} max={parallel.max} fromEnv={parallel.from_env} onChange={(n) => void setAgents(n)} />
          )}
          <PRDControl
            hasPRD={!!project.prd}
            findings={project.prd?.findings?.length ?? 0}
            pending={pending === "prd"}
            open={prdOpen}
            canGenerate={!working}
            onToggle={() => {
              if (!project.prd) return;
              setShowPRD((v) => !v);
              setTab("prd");
            }}
            onGenerate={() => void run("prd", api.generatePRD).then((ok) => { if (ok) { setShowPRD(true); setTab("prd"); } })}
          />
          {!working && (
            <button type="button" disabled={pending === "verify"} onClick={() => run("verify", api.verify)}>Verifikasi</button>
          )}
          {!working && c.TODO > 0 && (
            <button type="button" className="primary" disabled={pending === "build"} onClick={() => run("build", api.startBuild)}>
              {c.DONE > 0 ? "Lanjutkan Bangun" : "Mulai Bangun"}
            </button>
          )}
        </div>
      </div>

      {v && !v.ready && !working && (
        <div className="build-alert warn">
          <Icon name="warn" size={16} />
          <div className="grow">
            <strong>Masih ada pekerjaan</strong>
            <IssueList v={v} />
          </div>
          <button type="button" className="primary" disabled={pending === "fix"} onClick={() => run("fix", api.fix)}>Perbaiki Otomatis</button>
        </div>
      )}
      {v?.ready && (
        <div className="build-alert ok">
          <Icon name="check" size={16} />
          <span>Semua terverifikasi.</span>
          <Link to="/simple/siap">Lihat hasil</Link>
        </div>
      )}

      <div className={`build-stage${inspectOpen ? " open" : ""}`}>
        <Kanban project={project} selected={selected} onSelect={pickTask} />
        {inspectOpen && (
          <aside className="build-inspect" aria-label={inspectTab === "prd" ? "PRD" : "Detail task"}>
            <div className="build-inspect-head">
              {task && prdOpen ? (
                <div className="seg subtle">
                  <button type="button" className={inspectTab === "task" ? "on" : ""} onClick={() => setTab("task")}>Task</button>
                  <button type="button" className={inspectTab === "prd" ? "on" : ""} onClick={() => setTab("prd")}>PRD</button>
                </div>
              ) : (
                <strong>{inspectTab === "prd" ? "PRD" : "Task"}</strong>
              )}
              <button type="button" className="icon-btn" onClick={closeInspect} aria-label="Tutup panel" title="Tutup">
                <Icon name="close" size={14} />
              </button>
            </div>
            <div className="build-inspect-body">
              {inspectTab === "prd" && project.prd ? <PRDPanel prd={project.prd} /> : task ? <TaskDetail project={project} task={task} /> : null}
            </div>
            {inspectTab === "task" && task && <TaskActions project={project} task={task} />}
          </aside>
        )}
      </div>
    </div>
  );
}

function PRDControl({
  hasPRD, findings, pending, open, canGenerate, onToggle, onGenerate,
}: {
  hasPRD: boolean;
  findings: number;
  pending: boolean;
  open: boolean;
  canGenerate: boolean;
  onToggle: () => void;
  onGenerate: () => void;
}) {
  const menu = useRef<HTMLDetailsElement>(null);
  const closeMenu = () => { if (menu.current) menu.current.open = false; };
  useEffect(() => {
    const el = menu.current;
    if (!el) return;
    const onDoc = (e: MouseEvent) => {
      if (!el.open) return;
      if (!el.contains(e.target as Node)) el.open = false;
    };
    document.addEventListener("mousedown", onDoc);
    return () => document.removeEventListener("mousedown", onDoc);
  });
  if (pending) {
    return (
      <button type="button" className="prd-control solo" disabled>
        <span className="spinner" /> Menyusun PRD…
      </button>
    );
  }
  if (!hasPRD) {
    return (
      <button
        type="button"
        className="prd-control solo"
        disabled={!canGenerate}
        title="Susun dokumen kebutuhan dari task yang ada"
        onClick={onGenerate}
      >
        <Icon name="files" size={14} /> Buat PRD
      </button>
    );
  }
  return (
    <div className={`prd-control ${open ? "open" : ""}`}>
      <button
        type="button"
        className="prd-main"
        aria-pressed={open}
        title={open ? "Tutup PRD" : "Lihat PRD"}
        onClick={onToggle}
      >
        <Icon name="files" size={14} />
        PRD
        {findings > 0 && <span className="prd-badge" title={`${findings} temuan`}>{findings}</span>}
      </button>
      <details ref={menu} className="prd-more">
        <summary aria-label="Aksi PRD"><Icon name="chevron" size={14} /></summary>
        <div className="prd-menu" role="menu">
          <button type="button" role="menuitem" onClick={() => { onToggle(); closeMenu(); }}>
            {open ? "Tutup panel" : "Lihat PRD"}
          </button>
          <a className="prd-menu-item" role="menuitem" href="/api/v1/project/prd.md" download onClick={closeMenu}>
            <Icon name="download" size={14} /> Unduh .md
          </a>
          <button
            type="button"
            role="menuitem"
            disabled={!canGenerate}
            onClick={() => {
              closeMenu();
              if (window.confirm("Buat ulang PRD? Versi sekarang diganti.")) onGenerate();
            }}
          >
            Buat ulang
          </button>
        </div>
      </details>
    </div>
  );
}

const FINDING_LABEL: Record<PRDFinding["kind"], string> = {
  GAP: "tanpa requirement",
  UNASSIGNED: "belum jelas pemilik task",
  MISSING_TASK: "belum ada task",
  DUPLICATE: "tumpang tindih",
  OVERLAP: "berbagi file/halaman",
  INVALID_DEPENDENCY: "dependensi salah",
};

function formatWhen(iso: string) {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString("id-ID", { dateStyle: "medium", timeStyle: "short" });
}

function PRDPanel({ prd }: { prd: NonNullable<Project["prd"]> }) {
  const findings = prd.findings ?? [];
  return (
    <div className="prd-panel">
      <div className="prd-panel-head">
        <span className="muted small">{formatWhen(prd.generated)}</span>
        <a className="button" href="/api/v1/project/prd.md" download>
          <Icon name="download" size={14} /> Unduh
        </a>
      </div>
      {findings.length > 0 ? (
        <ul className="small prd-findings">
          {findings.map((f, i) => (
            <li key={i}><span className="tag">{f.kind}</span> {f.ref} {f.message || FINDING_LABEL[f.kind]}</li>
          ))}
        </ul>
      ) : (
        <p className="muted small">Tidak ada temuan gap.</p>
      )}
      <pre className="doc">{prd.markdown}</pre>
    </div>
  );
}

function TaskDetail({ project, task }: { project: Project; task: Task }) {
  const reqs = (task.requirement_ids ?? []).map((id) => project.requirements?.find((r) => r.id === id)).filter(Boolean);
  const pages = (task.page_ids ?? []).map((id) => project.design?.pages?.find((p) => p.id === id)?.name ?? id);
  const waiting = (task.depends_on ?? []).filter((id) => project.tasks?.find((t) => t.id === id)?.status !== "DONE");
  const queue = task.status === "TODO" ? queueLabel(queueItem(project, task.id)) : "";
  const build = project.builds?.find((b) => b.id === task.last_build_id);

  return (
    <>
      <div className="inspect-id">
        <span className={`status-pill st-${task.status}`}>{task.status.replace("_", " ")}</span>
        <span className="task-id">{task.id}</span>
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
      <ul className="small inspect-list">
        {task.acceptance_criteria.map((a, i) => {
          const c = task.criteria?.[i];
          return (
            <li key={i} className={c ? (c.met ? "met" : "unmet") : ""}>
              {a}
              {c?.evidence && <div className="muted">{c.evidence}</div>}
            </li>
          );
        })}
      </ul>
      {(task.test_requirements ?? []).length > 0 && (
        <><h4>Test</h4><ul className="small inspect-list">{task.test_requirements!.map((t) => <li key={t}>{t}</li>)}</ul></>
      )}
      {(task.files ?? []).length > 0 && <div className="small muted">File: {task.files!.join(", ")}</div>}

      {(task.notes ?? []).length > 0 && (
        <><h4>Catatan</h4>{task.notes!.map((n, i) => <pre key={i} className="code small">{n}</pre>)}</>
      )}

      {build && (
        <>
          <h4>Verifikasi {build.passed ? "lulus" : "gagal"}</h4>
          {(build.steps ?? []).map((s, i) => (
            <details key={i} className="small">
              <summary className={s.exit_code === 0 ? "met" : "unmet"}>{s.command} <span className="muted">({s.dir}, {s.millis} ms)</span></summary>
              <pre className="code">{s.output || "(tanpa output)"}</pre>
            </details>
          ))}
        </>
      )}
    </>
  );
}

function TaskActions({ project, task }: { project: Project; task: Task }) {
  const { run } = useProject();
  const build = project.builds?.find((b) => b.id === task.last_build_id);
  const move = (s: Task["status"]) => run("move", () => api.moveTask(task.id, s));
  const approve = task.status === "REVIEW" && !!build?.passed;
  const retry = task.status === "BLOCKED" || task.status === "DONE" || task.status === "REVIEW";
  const block = task.status === "TODO";
  if (!approve && !retry && !block) return null;
  return (
    <div className="build-inspect-actions">
      {approve && <button type="button" className="primary" onClick={() => move("DONE")}>Setujui (DONE)</button>}
      {retry && <button type="button" onClick={() => move("TODO")}>Kerjakan ulang</button>}
      {block && <button type="button" className="ghost" onClick={() => move("BLOCKED")}>Tandai terblokir</button>}
    </div>
  );
}
