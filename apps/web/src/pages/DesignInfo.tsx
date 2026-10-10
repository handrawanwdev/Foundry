import { api } from "../api";
import { PLATFORM_LABEL } from "../flow";
import { StructurePanel } from "../components/StructurePanel";
import { TechStackPanel } from "../components/TechStack";
import { useProject } from "../store/project";
import type { Blueprint, ExistingUI, PlatformKind, PlatformTarget, Project } from "../types";

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
  const locked = project.design?.status === "approved" || !!project.platform?.approved;
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
      <h1>Platform & Teknologi</h1>
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
      <TechStackPanel project={project} />
      {s?.mode === "existing" ? (
        <section className="info-sec">
          <h3>Struktur project</h3>
            {(s.components ?? []).map((c, i) => (
              <div key={i} className="small">✓ {c.kind} ditemukan <code>{c.path}</code> {c.tech && <span className="muted">({c.tech})</span>}</div>
            ))}
            <p className="small">{s.reason}</p>
            {s.ui && <ExistingUIInfo ui={s.ui} />}
            {(s.commands ?? []).length > 0 && (
              <details className="small"><summary>Command verifikasi</summary>
                {(s.commands ?? []).map((c, i) => <div key={i}><code>{c.argv.join(" ")}</code> <span className="muted">di {c.dir}</span></div>)}
              </details>
            )}
        </section>
      ) : s?.ir ? <StructurePanel project={project} />
        : <section className="info-sec"><h3>Struktur project</h3><p className="muted">Belum ditentukan</p></section>}
    </div>
  );
}

export function Summary({ project }: { project: Project }) {
  const d = project.discovery;
  const o = project.blueprint?.overview;
  return (
    <div className="info">
      <h1>Kebutuhan</h1>
      <section className="info-sec">
        <h3>Pemahaman Foundry</h3>
        <p>{o?.goal || d?.summary}</p>
        {project.survey && (
          <p className="small"><strong>Survey:</strong> {(project.survey.kinds ?? []).join(", ") || "otomatis"} · {project.survey.stage} · p95 &lt;{project.survey.latency}ms · webhook {project.survey.webhook}</p>
        )}
        <p className="small"><strong>Bisnis:</strong> {d?.business}</p>
        {o?.core_domain && <p className="small"><strong>Domain:</strong> {o.core_domain}</p>}
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

export function Architecture({ project }: { project: Project }) {
  const b = project.blueprint;
  if (!b) {
    return (
      <div className="info">
        <h1>Arsitektur</h1>
        <p className="muted">Blueprint disusun saat Foundry memahami cerita. Jalankan Ceritakan dulu.</p>
      </div>
    );
  }
  const line = (xs?: string[]) => (xs && xs.length ? xs.join(", ") : "");
  return (
    <div className="info">
      <h1>Arsitektur</h1>
      <section className="info-sec">
        <h3>{b.architecture.pattern || "Blueprint"}</h3>
        <p>{b.architecture.reason}</p>
        {b.architecture.tradeoff && <p className="small muted">{b.architecture.tradeoff}</p>}
        {b.architecture.scale && <p className="small"><strong>Scale:</strong> {b.architecture.scale}</p>}
        <p className="small">Offline: {b.platform.offline || "—"} · Realtime: {b.platform.realtime || "—"}</p>
      </section>
      <section className="info-sec">
        <h3>Modul</h3>
        {(b.modules ?? []).map((m) => (
          <div key={m.id} className="small info-flow">
            <strong><code>{m.id}</code></strong> {m.responsibility}
            {!!m.data?.length && <div className="muted">Data: {m.data.join(", ")}</div>}
            {!!m.interface?.length && <div className="muted">API: {m.interface.join(", ")}</div>}
            {!!m.depends_on?.length && <div className="muted">Depends: {m.depends_on.join(", ")}</div>}
          </div>
        ))}
      </section>
      <EntityTable b={b} />
      <section className="info-sec">
        <h3>Auth</h3>
        <p className="small">{b.auth?.method} · {b.auth?.hashing} · {b.auth?.tokens}</p>
        <p className="small"><strong>Izin:</strong> {b.authz?.model} <code>{b.authz?.convention}</code></p>
        {!!b.authz?.permissions?.length && <p className="small muted">{b.authz.permissions.join(", ")}</p>}
      </section>
      {b.storage?.strategy && b.storage.strategy !== "none" && (
        <section className="info-sec">
          <h3>File</h3>
          <p className="small">{b.storage.strategy} {b.storage.provider}</p>
          <p className="small">{b.storage.upload}</p>
        </section>
      )}
      {!!b.workers?.length && (
        <section className="info-sec">
          <h3>Worker</h3>
          {b.workers.map((w) => (
            <div key={w.name} className="small"><strong>{w.name}</strong> [{w.trigger}] {w.job}</div>
          ))}
        </section>
      )}
      <section className="info-sec">
        <h3>Performa</h3>
        <p className="small">{b.performance.pagination}</p>
        {!!b.performance.avoid?.length && <p className="small muted">Hindari: {line(b.performance.avoid)}</p>}
      </section>
      <section className="info-sec">
        <h3>Aturan kode</h3>
        <p className="small"><strong>Backend:</strong> {(b.coding.backend ?? []).join(" · ")}</p>
        <p className="small"><strong>Frontend:</strong> {(b.coding.frontend ?? []).join(" · ")}</p>
        {b.coding.api?.style && (
          <p className="small"><strong>API:</strong> {b.coding.api.style} {b.coding.api.versioning}</p>
        )}
        {b.frontend?.state && (
          <p className="small muted">{b.frontend.state} · {b.frontend.server_state} · {b.frontend.forms} · {b.frontend.validation}</p>
        )}
      </section>
      {!!b.ai_guidelines?.length && (
        <section className="info-sec">
          <h3>Aturan AI agent</h3>
          <ul className="small">{b.ai_guidelines.map((r) => <li key={r}>{r}</li>)}</ul>
        </section>
      )}
    </div>
  );
}

function EntityTable({ b }: { b: Blueprint }) {
  const ents = b.database.entities ?? [];
  if (!ents.length) return null;
  return (
    <section className="info-sec">
      <h3>Data · {b.database.engine} {b.database.orm && `(${b.database.orm})`}</h3>
      <p className="small muted">{b.database.naming} {b.database.pk}</p>
      {ents.map((e) => (
        <div key={e.name} className="small"><code>{e.name}</code> {e.module && <span className="muted">[{e.module}]</span>} {(e.fields ?? []).join(", ")}</div>
      ))}
    </section>
  );
}

const NAV_LABEL = { side: "menu samping", top: "menu atas" } as const;

// What the existing frontend has; new pages are designed to fit into it.
function ExistingUIInfo({ ui }: { ui: ExistingUI }) {
  const list = (items: string[] | undefined, label: string) =>
    items?.length ? <details className="small"><summary>{label} ({items.length})</summary>{items.map((x) => <div key={x}><code>{x}</code></div>)}</details> : null;
  return (
    <div className="small">
      <p><strong>UI yang sudah ada</strong> — halaman baru dirancang mengikuti ini.</p>
      {ui.navigation && <div>Navigasi: {NAV_LABEL[ui.navigation]}{ui.nav_items?.length ? ` (${ui.nav_items.join(", ")})` : ""}</div>}
      {ui.theme_id
        ? <div>Theme <code>{ui.theme_id}</code> dari token di {ui.theme_sources?.join(", ")}</div>
        : <div className="muted">Token desain tidak ditemukan di kode; theme dipilih seperti biasa.</div>}
      {list(ui.routes, "Route")}
      {list(ui.shell, "Layout & navigasi")}
      {list(ui.components, "Komponen bersama")}
    </div>
  );
}
