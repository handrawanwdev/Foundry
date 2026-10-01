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
  }, []);

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
      {!used && d?.pages.length ? <p className="small muted">Rancangan ini dibuat sebelum ada theme; memilih theme menerapkan token dan gayanya.</p> : null}
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
            {!compact && t.description && <span className="small muted">{t.description}</span>}
            <span className="theme-tags">{t.personality.map((p) => <i key={p}>{p}</i>)}</span>
          </button>
        ))}
      </div>
      {invalid.length > 0 && (
        <ul className="small warn-text">
          {invalid.map((p) => <li key={p.id}>Theme <code>{p.id}</code> tidak dipakai: {p.error.split("\n").filter((l) => l.startsWith("- ")).join(" ")}</li>)}
        </ul>
      )}
    </section>
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
