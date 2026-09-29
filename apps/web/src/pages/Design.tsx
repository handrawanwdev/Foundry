import { useEffect, useMemo, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { api } from "../api";
import { Wireframe } from "../components/Wireframe";
import { isWorking, PLATFORM_LABEL } from "../flow";
import { useProject } from "../store/project";
import type { PlatformKind, PlatformTarget, Project, Template } from "../types";

type Tab = "rancangan" | "fitur" | "platform" | "ringkasan";

export function DesignPage() {
  const { project } = useProject();
  const [params] = useSearchParams();
  const [tab, setTab] = useState<Tab>("rancangan");
  // Explorer deep links: ?tab=fitur, ?page=PAGE-001
  useEffect(() => {
    const t = params.get("tab") as Tab | null;
    if (t) setTab(t);
    else if (params.get("page")) setTab("rancangan");
  }, [params]);
  if (!project) return null;

  if (!project.design) {
    return (
      <section className="card narrow center-text">
        {isWorking(project) ? (
          <><h1><span className="spinner" /> Membuat rancangan aplikasi</h1><p className="muted">Halaman dibuat dari fitur dan alur pengguna.</p></>
        ) : (
          <><h1>Belum ada rancangan</h1><p><Link to="/simple/tell">Ceritakan idemu dulu</Link></p></>
        )}
      </section>
    );
  }

  return (
    <div className="design">
      <div className="tabs">
        {([
          ["rancangan", "Rancangan"],
          ["fitur", "Fitur & Alur"],
          ["platform", "Platform & Struktur"],
          ["ringkasan", "Kebutuhan"],
        ] as [Tab, string][]).map(([k, l]) => (
          <button key={k} className={`tab ${tab === k ? "active" : ""}`} onClick={() => setTab(k)}>{l}</button>
        ))}
      </div>
      {tab === "rancangan" && <Prototype project={project} />}
      {tab === "fitur" && <Features project={project} />}
      {tab === "platform" && <PlatformStructure project={project} />}
      {tab === "ringkasan" && <Summary project={project} />}
      <ApproveBar project={project} />
    </div>
  );
}

function Prototype({ project }: { project: Project }) {
  const pages = project.design?.pages ?? [];
  const [params] = useSearchParams();
  const [current, setCurrent] = useState(params.get("page") ?? pages[0]?.id);
  useEffect(() => {
    const p = params.get("page");
    if (p) setCurrent(p);
  }, [params]);
  useEffect(() => {
    if (!pages.find((p) => p.id === current)) setCurrent(pages[0]?.id);
  }, [pages, current]);
  const page = pages.find((p) => p.id === current);
  const feature = project.features?.find((f) => f.id === page?.feature_id);
  const byPlatform = useMemo(() => {
    const m = new Map<string, typeof pages>();
    for (const p of pages) m.set(p.platform, [...(m.get(p.platform) ?? []), p]);
    return [...m.entries()];
  }, [pages]);

  return (
    <div className="split">
      <aside className="pages">
        <h3>Pages</h3>
        {byPlatform.map(([pl, ps]) => (
          <div key={pl}>
            <div className="small muted upper">{PLATFORM_LABEL[pl] ?? pl}</div>
            {ps.map((p) => (
              <button key={p.id} className={`page-item ${p.id === current ? "active" : ""}`} onClick={() => setCurrent(p.id)}>
                {p.name}
              </button>
            ))}
          </div>
        ))}
      </aside>
      <div className="preview">
        {page && (
          <>
            <div className="preview-meta">
              <strong>{page.name}</strong> <span className="tag">{page.id}</span>
              {feature && <span className="tag">{feature.id} {feature.name}</span>}
              {page.role && <span className="tag">{page.role}</span>}
              <div className="small muted">{page.purpose}</div>
            </div>
            <Wireframe page={page} onNavigate={setCurrent} />
            <p className="small muted center-text">Klik tombol bertanda ↗ untuk mencoba alur prototype.</p>
          </>
        )}
      </div>
    </div>
  );
}

function Features({ project }: { project: Project }) {
  const pages = project.design?.pages ?? [];
  return (
    <div className="grid">
      {(project.features ?? []).map((f) => (
        <div key={f.id} className="card">
          <div className="row between">
            <strong>{f.name}</strong>
            <span className="tag">{f.id}</span>
          </div>
          <p className="small muted">{f.description}</p>
          <div className="small">Sub-fitur:</div>
          <ul className="small">{f.sub_features.map((s) => <li key={s.id}>{s.name}</li>)}</ul>
          <div className="small">Halaman: {pages.filter((p) => p.feature_id === f.id).map((p) => p.name).join(", ") || "— (backend)"}</div>
          <div className="small muted">Kebutuhan: {f.requirement_ids.join(", ")} · {f.platforms.map((p) => PLATFORM_LABEL[p]).join(", ")}</div>
          {(project.flows ?? []).filter((fl) => fl.feature_id === f.id).map((fl) => (
            <div key={fl.id} className="flow small">
              <strong>Alur: {fl.name}</strong> ({fl.role})
              <div>{fl.steps.join(" → ")}</div>
            </div>
          ))}
        </div>
      ))}
    </div>
  );
}

const KINDS: PlatformKind[] = ["web", "android", "ios", "desktop", "backend"];

function PlatformStructure({ project }: { project: Project }) {
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
    <div className="grid two">
      <div className="card">
        <h3>Platform yang disarankan</h3>
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
      </div>
      <div className="card">
        <h3>Struktur Project</h3>
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
      </div>
    </div>
  );
}

function Summary({ project }: { project: Project }) {
  const d = project.discovery;
  return (
    <div className="grid two">
      <div className="card">
        <h3>Pemahaman Foundry</h3>
        <p>{d?.summary}</p>
        <p className="small"><strong>Bisnis:</strong> {d?.business}</p>
        <p className="small"><strong>Peran:</strong> {d?.roles.map((r) => `${r.name} (${PLATFORM_LABEL[r.platform] ?? r.platform})`).join(", ")}</p>
        {!!d?.data.length && <p className="small"><strong>Data:</strong> {d.data.join("; ")}</p>}
        {!!d?.integrations.length && <p className="small"><strong>Integrasi:</strong> {d.integrations.join(", ")}</p>}
        {!!d?.security.length && <p className="small"><strong>Keamanan:</strong> {d.security.join(", ")}</p>}
      </div>
      <div className="card">
        <h3>Kebutuhan</h3>
        {(project.requirements ?? []).map((r) => (
          <div key={r.id} className="small req"><span className="tag">{r.id}</span> <span className={`prio ${r.priority}`}>{r.priority}</span> {r.text}</div>
        ))}
      </div>
    </div>
  );
}

function ApproveBar({ project }: { project: Project }) {
  const { run, pending } = useProject();
  const [feedback, setFeedback] = useState("");
  const [editing, setEditing] = useState(false);
  const d = project.design!;
  const working = isWorking(project);

  if (d.status === "approved") {
    return (
      <div className="approve-bar">
        <span>✓ Rancangan disetujui.</span>
        {(project.tasks?.length ?? 0) > 0 ? <Link className="button primary" to="/simple/bangun">Lihat Kanban</Link>
          : working ? <span><span className="spinner" /> Membuat task…</span> : null}
      </div>
    );
  }
  const stat = (n: number, l: string) => <div className="stat"><strong>{n}</strong><span>{l}</span></div>;
  return (
    <div className="approve-bar">
      <div className="stats">
        <span className="muted">Rancangan siap</span>
        {stat(d.pages.length, "Pages")}
        {stat(project.features?.length ?? 0, "Features")}
        {stat(project.flows?.length ?? 0, "Flows")}
        <div className="stat"><strong>{(project.platform?.targets ?? []).map((t) => PLATFORM_LABEL[t.kind]).join(" + ")}</strong><span>Platform</span></div>
      </div>
      {editing ? (
        <form className="inline grow" onSubmit={async (e) => {
          e.preventDefault();
          if (await run("revise", () => api.reviseDesign(feedback))) { setFeedback(""); setEditing(false); }
        }}>
          <input className="grow" value={feedback} onChange={(e) => setFeedback(e.target.value)} placeholder="mis. Tambahkan halaman riwayat pembayaran" autoFocus />
          <button type="submit" disabled={!feedback.trim() || working}>Kirim</button>
          <button type="button" className="ghost" onClick={() => setEditing(false)}>Batal</button>
        </form>
      ) : (
        <div className="row">
          <button disabled={working} onClick={() => setEditing(true)}>Ubah Rancangan</button>
          <button className="primary" disabled={working || pending === "approve"} onClick={() => run("approve", api.approveDesign)}>
            Setujui & Buat Task
          </button>
        </div>
      )}
    </div>
  );
}
