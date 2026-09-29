import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { api } from "../api";
import { IssueList } from "../components/IssueList";
import { isWorking } from "../flow";
import { useProject } from "../store/project";
import type { Command } from "../types";

type Panel = "app" | "project" | "docs" | null;

export function ReadyPage() {
  const { project, run, pending } = useProject();
  const [panel, setPanel] = useState<Panel>(null);
  const [docs, setDocs] = useState<{ markdown: string; run: Command[] | null }>();
  useEffect(() => { if (panel) void api.docs().then(setDocs); }, [panel, project?.version]);
  if (!project) return null;
  const v = project.verification;

  if (!v) {
    return (
      <section className="card narrow center-text">
        <h1>Belum diverifikasi</h1>
        <p><Link to="/simple/bangun">Kembali ke Bangun</Link></p>
      </section>
    );
  }

  return (
    <div className="ready">
      <section className="card narrow">
        {v.ready ? <h1>🎉 Project siap diperiksa</h1> : <h1>⚠ Masih ada pekerjaan</h1>}
        <h2 className="muted">{project.name}</h2>
        <ul className="checks">
          {v.checks.map((c) => (
            <li key={c.name} className={c.ok ? "ok" : "bad"}>{c.ok ? "✓" : "✗"} {c.name}{c.info && <span className="muted small"> — {c.info}</span>}</li>
          ))}
        </ul>
        {!v.ready && (
          <>
            <IssueList v={v} />
            <button className="primary" disabled={isWorking(project) || pending === "fix"} onClick={() => run("fix", api.fix)}>Perbaiki Otomatis</button>
          </>
        )}
        {v.ready && (
          <div className="row">
            <button className="primary" onClick={() => setPanel("app")}>Buka Aplikasi</button>
            <button onClick={() => setPanel("project")}>Lihat Project</button>
            <button onClick={() => setPanel("docs")}>Lihat Dokumentasi</button>
          </div>
        )}
      </section>

      {panel === "app" && (
        <section className="card narrow">
          <h3>Menjalankan aplikasi</h3>
          {(docs?.run ?? []).length > 0 ? (
            (docs!.run ?? []).map((c, i) => (
              <div key={i}>Di folder <code>{project.dir}/{c.dir}</code> jalankan <pre className="code">{c.argv.join(" ")}</pre></div>
            ))
          ) : (
            <p className="muted">Project tidak mendeklarasikan script <code>dev</code>/<code>start</code>. Lihat README project untuk cara menjalankan.</p>
          )}
        </section>
      )}
      {panel === "project" && (
        <section className="card narrow">
          <h3>Lokasi project</h3>
          <pre className="code">{project.dir}</pre>
          <h3>Traceability</h3>
          <table className="trace">
            <thead><tr><th>Kebutuhan</th><th>Fitur</th><th>Halaman</th><th>Task</th><th>Test</th><th /></tr></thead>
            <tbody>
              {(v.trace ?? []).map((t) => (
                <tr key={t.requirement_id}>
                  <td>{t.requirement_id}</td><td>{(t.feature_ids ?? []).join(", ")}</td><td>{(t.page_ids ?? []).join(", ")}</td>
                  <td>{(t.task_ids ?? []).join(", ")}</td><td>{(t.test_task_ids ?? []).join(", ")}</td><td>{t.done ? "✓" : "…"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </section>
      )}
      {panel === "docs" && (
        <section className="card narrow"><pre className="doc">{docs?.markdown ?? "Memuat…"}</pre></section>
      )}
    </div>
  );
}
