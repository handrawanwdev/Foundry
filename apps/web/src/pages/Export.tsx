import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { api, EXPORT_ZIP_URL, SPEC_JSON_URL, SPEC_MD_URL } from "../api";
import { isWorking } from "../flow";
import { useProject } from "../store/project";
import type { ExportView } from "../types";

// Finalize and export: the package an external coding agent reads.
export function ExportPage() {
  const { project, run, pending } = useProject();
  const [view, setView] = useState<ExportView | null>(null);

  useEffect(() => {
    if (!project?.discovery) return;
    api.exportInfo().then(setView).catch(() => setView(null));
  }, [project?.version, project?.dir]);

  if (!project) return null;
  if (!project.discovery) {
    return (
      <div className="dw-empty">
        <h1>Belum ada yang diekspor</h1>
        <p className="muted">Paket ekspor berisi blueprint dan command workflow. Mulai dari prompt.</p>
        <Link className="button primary" to="/simple/tell">Tulis prompt</Link>
      </div>
    );
  }
  const finalized = project.stage === "READY";
  const blockers = view?.blockers ?? [];
  const m = view?.manifest;
  const working = isWorking(project);

  return (
    <div className="export">
      <header className="export-head">
        <h1>Export</h1>
        <p className="muted">
          Paket berisi blueprint, command workflow beserta dependency dan urutannya, dan petunjuk untuk coding agent
          (Claude Code, Codex, OpenCode, Qwen Code, atau lainnya). Foundry tidak menjalankan instruksinya.
        </p>
      </header>

      <section className={`export-status ${finalized ? "ok" : ""}`}>
        {finalized ? (
          <>
            <div>
              <strong>✓ Final</strong>
              <span className="muted small"> · workflow v{m?.workflow_version} · blueprint <code>{m?.blueprint_hash}</code>
                {m?.finalized_at && ` · ${new Date(m.finalized_at).toLocaleString()}`}</span>
              <p className="small">Ditulis ke <code>{view?.dir}/</code> di folder project.</p>
            </div>
            <div className="row">
              <a className="button primary" href={EXPORT_ZIP_URL} download>Unduh paket (.zip)</a>
              <a className="button" href={SPEC_MD_URL} download>spec.md</a>
              <a className="button" href={SPEC_JSON_URL} download>spec.json</a>
              <button className="ghost" disabled={working || pending === "reopen"} onClick={() => void run("reopen", api.reopen)}>Buka kembali untuk revisi</button>
            </div>
          </>
        ) : (
          <>
            <div>
              <strong>Draft</strong>
              <p className="small muted">Finalisasi menyetujui blueprint dan workflow sebagai satu versi yang konsisten, lalu menulis paketnya. Ini menyetujui spesifikasi, bukan implementasi.</p>
            </div>
            {blockers.length > 0 ? (
              <ul className="export-blockers">
                {blockers.map((b) => <li key={b}>○ {b} <BlockerLink text={b} /></li>)}
              </ul>
            ) : view && <p className="ok-text small">✓ Semua siap difinalisasi.</p>}
            <div className="row">
              <button className="primary" disabled={!view || blockers.length > 0 || working || pending === "finalize"}
                onClick={() => void run("finalize", api.finalize)}>Finalisasi blueprint & workflow</button>
            </div>
          </>
        )}
      </section>

      {m && m.commands.length > 0 && (
        <section className="export-sec">
          <h2>Urutan pengerjaan</h2>
          <ol className="waves">
            {m.waves.map((w, i) => (
              <li key={i}>
                <span className="muted small">Gelombang {i + 1}{w.length > 1 ? " · paralel" : ""}</span>
                <div className="ref-list">{w.map((id) => <Link key={id} className="ref-chip" to={`/workflow?cmd=${id}`}>{id} {m.commands.find((c) => c.id === id)?.name}</Link>)}</div>
              </li>
            ))}
          </ol>
        </section>
      )}

      {view && (
        <section className="export-sec">
          <h2>Isi paket</h2>
          <ul className="files">
            {view.files.map((f) => <li key={f.path}><code>{f.path}</code><span className="muted small">{size(f.size)}</span></li>)}
          </ul>
          <p className="small muted">
            Agent mulai dari <code>README.md</code> dan <code>manifest.json</code>. Lewat MCP, agent bisa membaca <code>get_blueprint</code> dan <code>get_workflow</code>.
          </p>
        </section>
      )}
    </div>
  );
}

// Where to fix a blocker.
function BlockerLink({ text }: { text: string }) {
  const to = text.includes("workflow") ? "/workflow"
    : text.includes("desain") ? "/blueprint/ui"
    : text.includes("teknologi") || text.includes("struktur") ? "/blueprint?doc=platform"
    : text.includes("prompt") ? "/simple/tell" : "";
  return to ? <Link className="small" to={to}>Buka</Link> : null;
}

function size(n: number): string {
  return n < 1024 ? `${n} B` : `${(n / 1024).toFixed(1)} KB`;
}
