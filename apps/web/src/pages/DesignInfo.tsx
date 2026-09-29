import { useEffect, useState } from "react";
import { api } from "../api";
import { PLATFORM_LABEL } from "../flow";
import { useProject } from "../store/project";
import type { PlatformKind, PlatformTarget, Project, Template } from "../types";

// Secondary views of the design workspace: what the pages are built from.

export function Features({ project }: { project: Project }) {
  const pages = project.design?.pages ?? [];
  return (
    <div className="info">
      <h1>Fitur & Alur</h1>
      {(project.features ?? []).map((f) => (
        <section key={f.id} className="info-sec">
          <h3>{f.name}</h3>
          <p className="muted">{f.description}</p>
          <div className="small">Sub-fitur: {f.sub_features.map((s) => s.name).join(", ") || "—"}</div>
          <div className="small">Halaman: {pages.filter((p) => p.feature_id === f.id).map((p) => p.name).join(", ") || "— (backend)"}</div>
          <div className="small muted">{f.platforms.map((p) => PLATFORM_LABEL[p]).join(", ")}</div>
          {(project.flows ?? []).filter((fl) => fl.feature_id === f.id).map((fl) => (
            <div key={fl.id} className="small info-flow">
              <strong>{fl.name}</strong> <span className="muted">({fl.role})</span>
              <div>{fl.steps.join(" → ")}</div>
            </div>
          ))}
        </section>
      ))}
    </div>
  );
}

const KINDS: PlatformKind[] = ["web", "android", "ios", "desktop", "backend"];

export function PlatformStructure({ project }: { project: Project }) {
  const { run } = useProject();
  const [templates, setTemplates] = useState<Template[]>([]);
  const locked = project.design?.status === "approved";
  useEffect(() => { void api.templates().then(setTemplates); }, []);
  const targets = project.platform?.targets ?? [];
  const has = (k: PlatformKind) => targets.some((t) => t.kind === k);

  const toggle = (k: PlatformKind) => {
    const next: PlatformTarget[] = has(k)
      ? targets.filter((t) => t.kind !== k)
      : [...targets, { kind: k, audience: [], rationale: "Ditambahkan oleh user" }];
    void run("platform", () => api.setPlatform(next));
  };
  const s = project.structure;

  return (
    <div className="info">
      <h1>Platform & Struktur</h1>
      <section className="info-sec">
        <h3>Platform</h3>
        {KINDS.map((k) => {
          const t = targets.find((x) => x.kind === k);
          return (
            <label key={k} className="check">
              <input type="checkbox" checked={!!t} disabled={locked} onChange={() => toggle(k)} />
              <strong>{PLATFORM_LABEL[k]}</strong>
              {t && t.audience.length > 0 && <span className="muted small"> — {t.audience.join(", ")}</span>}
            </label>
          );
        })}
        {project.platform?.reason && <p className="small"><strong>Alasan:</strong> {project.platform.reason}</p>}
      </section>
      <section className="info-sec">
        <h3>Struktur project</h3>
        {s?.mode === "existing" ? (
          <>
            {(s.components ?? []).map((c, i) => (
              <div key={i} className="small">✓ {c.kind} ditemukan <code>{c.path}</code> {c.tech && <span className="muted">({c.tech})</span>}</div>
            ))}
            <p className="small">{s.reason}</p>
            {(s.commands ?? []).length > 0 && (
              <details className="small"><summary>Command verifikasi</summary>
                {(s.commands ?? []).map((c, i) => <div key={i}><code>{c.argv.join(" ")}</code> <span className="muted">di {c.dir}</span></div>)}
              </details>
            )}
          </>
        ) : s ? (
          <>
            {templates.map((t) => (
              <label key={t.id} className="check">
                <input type="radio" name="tpl" checked={s.template === t.id} disabled={locked}
                  onChange={() => void run("template", () => api.setTemplate(t.id))} />
                <strong>{t.name}</strong> <span className="muted small">{t.description}</span>
              </label>
            ))}
            <pre className="code small">{(s.layout ?? []).join("\n")}</pre>
            <p className="small">{s.reason}</p>
          </>
        ) : <p className="muted">Belum ditentukan</p>}
      </section>
    </div>
  );
}

export function Summary({ project }: { project: Project }) {
  const d = project.discovery;
  return (
    <div className="info">
      <h1>Kebutuhan</h1>
      <section className="info-sec">
        <h3>Pemahaman Foundry</h3>
        <p>{d?.summary}</p>
        <p className="small"><strong>Bisnis:</strong> {d?.business}</p>
        <p className="small"><strong>Peran:</strong> {d?.roles.map((r) => `${r.name} (${PLATFORM_LABEL[r.platform] ?? r.platform})`).join(", ")}</p>
        {!!d?.data.length && <p className="small"><strong>Data:</strong> {d.data.join("; ")}</p>}
        {!!d?.integrations.length && <p className="small"><strong>Integrasi:</strong> {d.integrations.join(", ")}</p>}
        {!!d?.security.length && <p className="small"><strong>Keamanan:</strong> {d.security.join(", ")}</p>}
      </section>
      <section className="info-sec">
        <h3>Daftar kebutuhan</h3>
        {(project.requirements ?? []).map((r) => (
          <div key={r.id} className="small req"><span className="muted">{r.id}</span> <span className={`prio ${r.priority}`}>{r.priority}</span> {r.text}</div>
        ))}
      </section>
    </div>
  );
}
