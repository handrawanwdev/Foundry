import { useEffect, useState, type CSSProperties } from "react";
import { api } from "../api";
import { isWorking } from "../flow";
import { useProject } from "../store/project";
import type { Project, Theme, ThemeProblem } from "../types";

const SOURCE_LABEL = { user: "dipilih", auto: "otomatis", fallback: "pengganti" };
const color = (t: Theme, name: string) => t.colors.find((c) => c.name === name)?.value;

// Theme Gallery: the user picks the visual direction before the design is
// generated, or re-themes a draft (tokens and style change, pages do not).
// "Otomatis" lets Foundry select from the requirement.
export function ThemeGallery({ project, compact = false }: { project: Project; compact?: boolean }) {
  const { run, pending } = useProject();
  const [themes, setThemes] = useState<Theme[]>([]);
  const [invalid, setInvalid] = useState<ThemeProblem[]>([]);
  const [error, setError] = useState("");
  useEffect(() => {
    api.themes().then((r) => { setThemes(r.themes ?? []); setInvalid(r.invalid ?? []); }, (e) => setError(String(e)));
  }, [project.reference?.theme_id]);

  const d = project.design;
  const used = d?.theme;
  const locked = d?.status === "approved" || !!d?.revision || isWorking(project) || pending === "theme";
  const why = d?.status === "approved" ? "Rancangan sudah disetujui" : d?.revision ? "Simpan atau batalkan perubahan terakhir dulu" : undefined;
  const pick = (id: string) => run("theme", () => api.setTheme(id));
  const picked = project.theme ?? "";

  return (
    <section className={`theme-gallery ${compact ? "compact" : ""}`}>
      {used && (
        <p className="small muted">
          Dipakai: <strong className="fg">{used.name}</strong> ({SOURCE_LABEL[used.source]}){used.reasons?.length ? ` — ${used.reasons.join("; ")}` : ""}
        </p>
      )}
      {!used && d?.pages?.length ? <p className="small muted">Rancangan ini dibuat sebelum ada theme; memilih theme menerapkan token dan gayanya.</p> : null}
      {error && <p className="warn-text small">{error}</p>}
      <div className="theme-grid">
        <button className={`theme-card auto ${picked === "" ? "on" : ""}`} disabled={locked} title={why} onClick={() => pick("")}>
          <strong>Otomatis</strong>
          <span className="small muted">Foundry memilih dari domain dan alur kerja.</span>
        </button>
        {themes.map((t) => (
          <button key={t.id} className={`theme-card ${picked === t.id ? "on" : ""}`} disabled={locked} title={why} onClick={() => pick(t.id)}>
            <ThemePreview t={t} />
            <strong>{t.name}</strong>
            {t.reference && <span className="small muted">terinspirasi dari {t.reference.name} ({t.reference.category})</span>}
            {!compact && t.description && <span className="small muted">{t.description}</span>}
            <span className="theme-tags">{t.personality.map((p) => <i key={p}>{p}</i>)}</span>
          </button>
        ))}
      </div>
      <ReferenceForm project={project} locked={locked} />
      {invalid.length > 0 && (
        <ul className="small warn-text">
          {invalid.map((p) => <li key={p.id}>Theme <code>{p.id}</code> tidak dipakai: {p.error.split("\n").filter((l) => l.startsWith("- ")).join(" ")}</li>)}
        </ul>
      )}
    </section>
  );
}

const REF_STATUS = { pending: "menunggu cerita dianalisis", analyzing: "sedang dianalisis", done: "jadi theme", failed: "gagal dianalisis" };

// Reference: a website or app whose design language becomes a theme.
// Foundry only looks at it (screenshots); nothing is copied from it.
function ReferenceForm({ project, locked }: { project: Project; locked: boolean }) {
  const { run, pending } = useProject();
  const ref = project.reference;
  const [url, setUrl] = useState("");
  const [notes, setNotes] = useState("");
  const analyzing = isWorking(project) && project.job?.kind === "reference";
  return (
    <div className="theme-ref">
      <strong className="small">Dari referensi</strong>
      <span className="small muted">Website atau aplikasi yang gayanya kamu suka. Foundry mengambil prinsip desainnya, bukan menyalin halaman, teks atau mereknya.</span>
      {ref && (
        <p className="small">
          <a href={ref.url} target="_blank" rel="noreferrer">{ref.name}</a>: {analyzing ? <><span className="spinner" /> sedang dianalisis</> : REF_STATUS[ref.status]}
          {ref.error && <span className="warn-text"> — {ref.error}</span>}
          {project.job?.kind === "reference" && project.job.status === "failed" && <span className="warn-text"> — {project.job.error}</span>}
          {" "}<button className="link" disabled={locked} onClick={() => run("reference", () => api.setReference(""))}>hapus</button>
        </p>
      )}
      <div className="theme-ref-row">
        <input value={url} onChange={(e) => setUrl(e.target.value)} placeholder="mis. linear.app" />
        <input value={notes} onChange={(e) => setNotes(e.target.value)} placeholder="Yang disukai (opsional)" />
        <button disabled={locked || !url.trim() || pending === "reference"}
          onClick={async () => { if (await run("reference", () => api.setReference(url, notes))) { setUrl(""); setNotes(""); } }}>Analisis</button>
      </div>
    </div>
  );
}

// A small sample drawn with the theme's own tokens: page, panel, text,
// primary action and status colors.
function ThemePreview({ t }: { t: Theme }) {
  const btn = t.components?.button ?? "solid";
  const primary = color(t, "action.primary");
  const style = {
    background: color(t, "surface.page"), color: color(t, "text.primary"), borderColor: color(t, "border.default"),
    borderRadius: t.radius.find((r) => r.name === "radius.md")?.value,
  } as CSSProperties;
  return (
    <span className="theme-preview" style={style} aria-hidden>
      <span className="theme-panel" style={{ background: color(t, "surface.primary"), borderColor: color(t, "border.default") }}>
        <span className="theme-line" style={{ background: color(t, "text.primary") }} />
        <span className="theme-line short" style={{ background: color(t, "text.secondary") }} />
        <span className="theme-dots">
          {["status.success", "status.warning", "status.error"].map((n) => <i key={n} style={{ background: color(t, n) }} />)}
        </span>
      </span>
      <span className="theme-btn" style={btn === "solid" ? { background: primary, borderColor: primary }
        : { color: primary, borderColor: primary, background: btn === "soft" ? `color-mix(in srgb, ${primary} 15%, transparent)` : "transparent" }} />
    </span>
  );
}
