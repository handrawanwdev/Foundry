import { useEffect, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { api } from "../api";
import { commandState, commandText, dependents, lines, STATE_LABEL, waveOf } from "../blueprint";
import { Icon } from "../components/Icon";
import { Markdown } from "../components/Markdown";
import { PromptBar } from "../components/PromptBar";
import { isWorking } from "../flow";
import { useProject } from "../store/project";
import type { Command, CommandEdit, Project, WorkflowReport, WorkflowView } from "../types";

// Workflow workspace: commands by wave on the left, one command in full.
// Foundry writes and checks the instructions; an external coding agent
// carries them out.
export function WorkflowPage() {
  const { project, run, pending } = useProject();
  const [view, setView] = useState<WorkflowView | null>(null);
  const [params, setParams] = useSearchParams();

  useEffect(() => {
    if (!project?.discovery) return;
    api.workflow().then(setView).catch(() => setView(null));
  }, [project?.version, project?.dir]);

  if (!project) return null;
  const working = isWorking(project);
  const locked = project.stage === "READY";
  const w = view?.workflow;
  const r = view?.report ?? null;
  const cmds = w?.commands ?? [];

  if (!project.discovery) {
    return (
      <div className="dw-empty">
        <h1>Belum ada blueprint</h1>
        <p className="muted">Workflow disusun dari blueprint. Mulai dari prompt.</p>
        <Link className="button primary" to="/simple/tell">Tulis prompt</Link>
      </div>
    );
  }
  if (cmds.length === 0) {
    const writing = working && project.job?.kind === "workflow";
    return (
      <div className="dw-empty">
        <h1>Command workflow</h1>
        <p className="muted">
          Foundry menyusun instruksi untuk coding agent eksternal dari blueprint: tujuan, konteks, dependency, output, dan
          acceptance criteria tiap command. Foundry tidak menjalankannya.
        </p>
        {writing ? <p><span className="spinner" /> Menyusun workflow…</p> : (
          <button className="primary big" disabled={working || locked || pending === "generateWorkflow"}
            onClick={() => void run("generateWorkflow", api.generateWorkflow)}>Susun workflow dari blueprint</button>
        )}
        {working && !writing && <p className="muted small">Tunggu proses AI yang sedang berjalan selesai.</p>}
      </div>
    );
  }

  const id = params.get("cmd") ?? cmds[0].id;
  const cmd = cmds.find((c) => c.id === id) ?? cmds[0];
  const select = (next: string) => setParams({ cmd: next }, { replace: true });
  const reviewIds = Object.keys(r?.review ?? {});
  const staleIds = Object.keys(r?.stale ?? {}).filter((k) => (r?.stale[k]?.length ?? 0) > 0);
  const byId = new Map(cmds.map((c) => [c.id, c]));
  const waves = r && r.waves.length ? r.waves : [cmds.map((c) => c.id)];

  return (
    <div className="ws">
      <nav className="ws-nav" aria-label="Command">
        <div className="ws-nav-head">
          <strong>Workflow</strong>
          <span className="muted small">v{w!.version} · {cmds.length} command</span>
        </div>
        {waves.map((wave, i) => (
          <div key={i} className="ws-group">
            <div className="ws-group-label">{r?.waves.length ? `Gelombang ${i + 1}${wave.length > 1 ? " · paralel" : ""}` : "Urutan belum valid"}</div>
            {wave.map((cid) => {
              const c = byId.get(cid);
              if (!c) return null;
              const st = commandState(c, r);
              return (
                <button key={cid} className={`ws-item cmd ${cid === cmd.id ? "on" : ""}`} aria-current={cid === cmd.id ? "page" : undefined} onClick={() => select(cid)}>
                  <span className="cmd-id">{cid}</span>
                  <span className="ws-item-text">{c.name}</span>
                  {st !== "draft" && <span className={`cmd-dot st-${st}`} title={STATE_LABEL[st]} />}
                </button>
              );
            })}
          </div>
        ))}
      </nav>

      <main className="ws-main">
        {r && r.issues.length > 0 && (
          <div className="notice bad">
            <strong>{r.issues.length} masalah workflow</strong>
            <ul>{r.issues.slice(0, 6).map((is, i) => <li key={i}>{is.command && <button className="link" onClick={() => select(is.command!)}>{is.command}</button>} {is.message}</li>)}</ul>
          </div>
        )}
        {reviewIds.length > 0 && !locked && (
          <div className="notice warn">
            <span><strong>{reviewIds.length} command perlu ditinjau</strong>{staleIds.length > 0 && " — artefak blueprint yang dirujuk sudah berubah"}.</span>
            {staleIds.length > 0 && (
              <button disabled={working || pending === "reviseWorkflow"} onClick={() => void run("reviseWorkflow",
                () => api.reviseWorkflow("Perbarui command yang terdampak perubahan artefak blueprint. Ubah hanya yang memang perlu.", staleIds))}>
                ✦ Perbarui yang terdampak
              </button>
            )}
          </div>
        )}
        <CommandDoc key={cmd.id + w!.version} project={project} cmd={cmd} report={r} locked={locked} onOpen={select} />
      </main>

      {!locked && (
        <PromptBar
          label={`Minta perubahan workflow (sedang melihat: ${cmd.id})…`}
          placeholder="Contoh: pecah CMD-003 jadi backend dan frontend; tambahkan command migrasi data sebelum CMD-004."
          hint="AI mengubah command yang terdampak saja; dependency divalidasi."
          disabled={working}
          busy={pending === "reviseWorkflow"}
          onSend={(text) => run("reviseWorkflow", () => api.reviseWorkflow(text, [cmd.id]))}
        />
      )}
    </div>
  );
}

function CommandDoc({ project, cmd, report, locked, onOpen }: {
  project: Project; cmd: Command; report: WorkflowReport | null; locked: boolean; onOpen: (id: string) => void;
}) {
  const { run, pending } = useProject();
  const [editing, setEditing] = useState(false);
  const [copied, setCopied] = useState(false);
  const st = commandState(cmd, report);
  const wave = waveOf(report, cmd.id);
  const stale = report?.stale[cmd.id] ?? [];
  const deps = dependents(project.workflow, cmd.id);
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(commandText(cmd, report));
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch { /* clipboard blocked: nothing to do */ }
  };
  const list = (title: string, items?: string[], check = false) => items && items.length > 0 && (
    <section className="doc-sec">
      <h2>{title}</h2>
      <ul className={check ? "checks" : undefined}>{items.map((x, i) => <li key={i}>{check && <span aria-hidden>☐ </span>}{x}</li>)}</ul>
    </section>
  );

  return (
    <article className="doc">
      <header className="doc-head">
        <div className="doc-kicker">
          <span className="cmd-id big">{cmd.id}</span>
          <span className={`state st-${st}`}>{STATE_LABEL[st]}</span>
          {wave > 0 && <span className="muted small">Gelombang {wave}</span>}
        </div>
        <h1>{cmd.name}</h1>
        <p className="doc-summary">{cmd.objective}</p>
        <div className="doc-meta small">
          <span className="muted">Bergantung pada:</span>
          {cmd.depends_on?.length ? cmd.depends_on.map((d) => <button key={d} className="ref-chip" onClick={() => onOpen(d)}>{d}</button>) : <span>—</span>}
          {deps.length > 0 && <><span className="muted">· Dibutuhkan oleh:</span>{deps.map((d) => <button key={d} className="ref-chip" onClick={() => onOpen(d)}>{d}</button>)}</>}
        </div>
        <div className="doc-actions">
          <button onClick={() => void copy()}><Icon name="copy" size={14} /> {copied ? "Tersalin" : "Salin instruksi"}</button>
          {!locked && !editing && <button onClick={() => setEditing(true)}>Edit</button>}
          {!locked && st !== "review" && st !== "issue" && (
            <button className="ghost" disabled={pending === "review"} onClick={() => void run("review", () => api.reviewCommand(cmd.id, true))}>Tandai perlu ditinjau</button>
          )}
        </div>
      </header>

      {st === "review" && !locked && (
        <div className="notice warn">
          <span>{report?.review[cmd.id] || "Perlu ditinjau"}</span>
          <button disabled={pending === "review"} onClick={() => void run("review", () => api.reviewCommand(cmd.id, false))}>Tandai sudah ditinjau</button>
        </div>
      )}

      {editing ? <CommandEditor project={project} cmd={cmd} report={report} onDone={() => setEditing(false)} /> : (
        <>
          {cmd.context && <section className="doc-sec"><h2>Konteks</h2><Markdown text={cmd.context} /></section>}
          {(cmd.artifacts?.length ?? 0) > 0 && (
            <section className="doc-sec">
              <h2>Acuan blueprint</h2>
              <div className="ref-list">
                {cmd.artifacts!.map((a) => (
                  <Link key={a.id} className={`ref-chip ${stale.includes(a.id) ? "stale" : ""}`} to={`/blueprint?doc=${a.id}`}
                    title={stale.includes(a.id) ? "Berubah sejak command ditulis" : undefined}>
                    {report?.docs[a.id]?.title ?? a.id}{stale.includes(a.id) && " · berubah"}
                  </Link>
                ))}
              </div>
            </section>
          )}
          {list("Input", cmd.inputs)}
          <section className="doc-sec"><h2>Instruksi</h2><Markdown text={cmd.instructions} /></section>
          {list("Output yang diharapkan", cmd.expected_outputs)}
          {list("Acceptance criteria", cmd.acceptance_criteria, true)}
          {list("Batasan", cmd.constraints)}
        </>
      )}
    </article>
  );
}

function CommandEditor({ project, cmd, report, onDone }: { project: Project; cmd: Command; report: WorkflowReport | null; onDone: () => void }) {
  const { run, pending } = useProject();
  const others = (project.workflow?.commands ?? []).filter((c) => c.id !== cmd.id);
  const docs = Object.entries(report?.docs ?? {});
  const [e, setE] = useState({
    name: cmd.name, objective: cmd.objective, context: cmd.context ?? "", instructions: cmd.instructions,
    inputs: (cmd.inputs ?? []).join("\n"), outputs: (cmd.expected_outputs ?? []).join("\n"),
    acceptance: (cmd.acceptance_criteria ?? []).join("\n"), constraints: (cmd.constraints ?? []).join("\n"),
    deps: cmd.depends_on ?? [], artifacts: (cmd.artifacts ?? []).map((a) => a.id),
  });
  const set = (k: keyof typeof e) => (ev: { target: { value: string } }) => setE({ ...e, [k]: ev.target.value });
  const toggle = (k: "deps" | "artifacts", v: string) => setE({ ...e, [k]: e[k].includes(v) ? e[k].filter((x) => x !== v) : [...e[k], v] });
  const save = async () => {
    const edit: CommandEdit = {
      name: e.name, objective: e.objective, context: e.context, instructions: e.instructions, inputs: lines(e.inputs),
      depends_on: e.deps, artifacts: e.artifacts, expected_outputs: lines(e.outputs), acceptance_criteria: lines(e.acceptance), constraints: lines(e.constraints),
    };
    if (await run("editCommand", () => api.editCommand(cmd.id, edit))) onDone();
  };
  return (
    <div className="doc-edit">
      <label className="field">Nama<input value={e.name} onChange={set("name")} /></label>
      <label className="field">Tujuan<input value={e.objective} onChange={set("objective")} /></label>
      <label className="field">Konteks<textarea rows={3} value={e.context} onChange={set("context")} /></label>
      <label className="field">Instruksi (Markdown)<textarea className="mono" rows={12} value={e.instructions} onChange={set("instructions")} /></label>
      <fieldset className="field">
        <legend>Bergantung pada</legend>
        <div className="check-grid">
          {others.length === 0 && <span className="muted small">Tidak ada command lain.</span>}
          {others.map((c) => (
            <label key={c.id} className="check"><input type="checkbox" checked={e.deps.includes(c.id)} onChange={() => toggle("deps", c.id)} /> {c.id} <span className="muted">{c.name}</span></label>
          ))}
        </div>
      </fieldset>
      <fieldset className="field">
        <legend>Acuan blueprint</legend>
        <div className="check-grid">
          {docs.map(([id, d]) => (
            <label key={id} className="check"><input type="checkbox" checked={e.artifacts.includes(id)} onChange={() => toggle("artifacts", id)} /> {d.title}</label>
          ))}
        </div>
      </fieldset>
      <label className="field">Input <span className="muted small">(satu per baris)</span><textarea rows={3} value={e.inputs} onChange={set("inputs")} /></label>
      <label className="field">Output yang diharapkan <span className="muted small">(satu per baris)</span><textarea rows={3} value={e.outputs} onChange={set("outputs")} /></label>
      <label className="field">Acceptance criteria <span className="muted small">(satu per baris)</span><textarea rows={4} value={e.acceptance} onChange={set("acceptance")} /></label>
      <label className="field">Batasan <span className="muted small">(satu per baris)</span><textarea rows={3} value={e.constraints} onChange={set("constraints")} /></label>
      <div className="row">
        <button className="primary" disabled={!e.name.trim() || !e.objective.trim() || !e.instructions.trim() || pending === "editCommand"} onClick={() => void save()}>Simpan</button>
        <button className="ghost" onClick={onDone}>Batal</button>
        <span className="muted small">Dependency melingkar atau referensi yang tidak ada ditolak.</span>
      </div>
    </div>
  );
}
