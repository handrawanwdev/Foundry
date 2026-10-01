import { useEffect, useState } from "react";
import { api } from "../api";
import { isWorking } from "../flow";
import { useProject } from "../store/project";
import type { Project, StackCatalog, TechArea, TechDecision, TechOption } from "../types";

export const TECH_AREA: Record<TechArea, string> = {
  frontend: "Frontend", mobile: "Mobile", desktop: "Desktop", backend: "Backend",
  database: "Database", cache: "Cache", architecture: "Arsitektur", repository: "Repository",
};
const NEED: Record<string, string> = {
  realtime: "realtime", offline: "offline", seo: "SEO", ai: "AI", high_concurrency: "konkurensi tinggi",
  flexible_schema: "skema fleksibel", transactions: "transaksi", independent_scaling: "skala independen", enterprise: "enterprise",
};
// Areas whose value may be outside the catalog; the others shape the
// project structure and must be a known option.
const OPEN = new Set<TechArea>(["frontend", "mobile", "desktop", "backend", "database"]);
const OTHER = "__other";

export const techChoice = (d: TechDecision) => d.override?.value ?? d.recommended;
const techLang = (d: TechDecision) => (d.override ? d.override.language : d.language);

// Technology Recommendation: what the product is built with and why. The
// user may change any area; the plan, tasks and code handoff follow it.
export function TechStackPanel({ project }: { project: Project }) {
  const { run, pending } = useProject();
  const [cat, setCat] = useState<StackCatalog | null>(null);
  const [edit, setEdit] = useState<TechArea | null>(null);
  useEffect(() => { api.stackCatalog().then(setCat, () => {}); }, []);
  const s = project.stack;
  const locked = !!s?.approved || isWorking(project) || project.design?.status === "approved" || pending === "stack";
  const analyze = () => void run("stack", api.recommendStack);
  if (!s) {
    return (
      <section className="info-sec">
        <h3>Teknologi</h3>
        <p className="small muted">Belum ada rekomendasi teknologi.</p>
        {project.platform && <button disabled={locked} onClick={analyze}>Buat rekomendasi</button>}
      </section>
    );
  }
  const profile = cat?.profiles.find((p) => p.id === s.profile);
  const editable = s.mode === "new" && !locked;
  return (
    <section className="info-sec">
      <h3>Teknologi</h3>
      <p className="small muted">
        {s.mode === "existing" ? "Dibaca dari kode project yang ada; tidak diganti."
          : <>Profil <strong className="fg">{profile?.name ?? s.profile}</strong>{profile ? ` — ${profile.description}` : ""}</>}
      </p>
      {s.needs?.length ? <p className="small">Kebutuhan teknis: <span className="theme-tags">{s.needs.map((n) => <i key={n}>{NEED[n] ?? n}</i>)}</span></p> : null}
      <div className="tech-list">
        {s.decisions.map((d) => (
          <div key={d.area} className="tech-row">
            <div className="tech-head">
              <span className="tech-area">{TECH_AREA[d.area] ?? d.area}</span>
              <strong>{techChoice(d)}</strong>
              {techLang(d) && <span className="small muted">{techLang(d)}</span>}
              {d.override && <span className="tech-tag">diganti</span>}
              {editable && <button className="ghost small" onClick={() => setEdit(edit === d.area ? null : d.area)}>Ubah</button>}
            </div>
            <div className="small muted">
              {d.override ? <>Rekomendasi: {d.recommended} — {d.reason}<br />Alasan diganti: {d.override.reason || "—"}</> : <>Alasan: {d.reason}</>}
            </div>
            {edit === d.area && <TechEdit decision={d} options={cat?.catalog[d.area] ?? []} onDone={() => setEdit(null)} />}
          </div>
        ))}
      </div>
      {editable && <button className="ghost small" onClick={analyze}>↻ Analisa ulang</button>}
    </section>
  );
}

function TechEdit({ decision: d, options, onDone }: { decision: TechDecision; options: TechOption[]; onDone: () => void }) {
  const { run, pending } = useProject();
  const current = techChoice(d);
  const [value, setValue] = useState(current);
  const [custom, setCustom] = useState(!options.some((o) => o.value === current));
  const [reason, setReason] = useState(d.override?.reason ?? "");
  const save = async (v: string, r: string) => { if (await run("stack", () => api.setStack(d.area, v, r))) onDone(); };
  return (
    <div className="tech-edit">
      <select value={custom ? OTHER : value} onChange={(e) => {
        const v = e.target.value;
        setCustom(v === OTHER);
        setValue(v === OTHER ? "" : v);
      }}>
        {options.map((o) => <option key={o.value} value={o.value}>{o.value}{o.value === d.recommended ? " (rekomendasi)" : ""} — {o.fit}</option>)}
        {OPEN.has(d.area) && <option value={OTHER}>Lainnya…</option>}
      </select>
      {custom && <input value={value} maxLength={80} placeholder="mis. Laravel + PHP" onChange={(e) => setValue(e.target.value)} />}
      {value.trim() !== d.recommended && (
        <input value={reason} maxLength={200} placeholder="Alasan, mis. keahlian tim" onChange={(e) => setReason(e.target.value)} />
      )}
      <div className="tech-actions">
        <button disabled={pending === "stack" || !value.trim()} onClick={() => void save(value, reason)}>Simpan</button>
        {d.override && <button className="ghost" disabled={pending === "stack"} onClick={() => void save("", "")}>Kembali ke rekomendasi</button>}
        <button className="ghost" onClick={onDone}>Batal</button>
      </div>
    </div>
  );
}
