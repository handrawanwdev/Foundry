import { useEffect, useState } from "react";
import { api } from "../api";
import type { DesignTemplate, Project, TemplateGallery, Theme } from "../types";
import { Wireframe } from "./Wireframe";

const DOMAINS: [string, string][] = [["", "Domain proyek"], ["ticketing", "Ticketing"], ["finance", "Finance"], ["trading", "Trading"],
  ["developer_tools", "Developer tools"], ["admin_operations", "Admin & operasional"]];
const SOURCE = { ai: "dipilih AI", auto: "rekomendasi Foundry" };
const AVOID: Record<string, string> = {
  hero_section: "hero besar", marketing_section: "bagian pemasaran", large_cards: "kartu besar", excessive_stats: "statistik berlebih",
  decorative_chart: "chart dekoratif", competing_actions: "aksi utama bersaing",
};
// Previews are drawn at desktop width and scaled into the card.
const THUMB_SCALE = 0.25;

// Template Library: proven UX patterns the designer adapts. A preview is
// the template's Design IR with sample data, styled by a theme and drawn
// by the same renderer as the design; nothing here is copied into a page.
export function TemplateLibrary({ project }: { project: Project }) {
  const [themes, setThemes] = useState<Theme[]>([]);
  const [theme, setTheme] = useState("");
  const [domain, setDomain] = useState("");
  const [g, setG] = useState<TemplateGallery | null>(null);
  const [open, setOpen] = useState<string | null>(null);
  const [error, setError] = useState("");
  useEffect(() => { api.themes().then((r) => setThemes(r.themes ?? []), () => {}); }, []);
  useEffect(() => {
    api.designTemplates(theme, domain).then((r) => { setG(r); setError(""); }, (e) => setError(String(e)));
  }, [theme, domain, project.design?.theme?.id, project.design?.domain]);

  const chosen = project.design?.template;
  const rec = g?.recommended;
  const detail = g?.templates?.find((t) => t.id === open);
  return (
    <div className="info">
      <h1>Template Desain</h1>
      <p className="muted small">Pola struktur yang sudah terbukti. AI memilih dan menyesuaikannya dengan fitur produk; theme tetap menentukan tampilan.</p>
      {chosen ? (
        <p className="small">
          Dipakai: <strong>{chosen.name}</strong>{chosen.secondary ? ` + ${g?.templates?.find((t) => t.id === chosen.secondary)?.name ?? chosen.secondary}` : ""} ({SOURCE[chosen.source]})
          {chosen.reasons?.length ? ` — ${chosen.reasons.join("; ")}` : ""}
        </p>
      ) : project.design?.pages.length ? <p className="small muted">Rancangan ini tidak memakai template.</p> : null}
      {rec && (!chosen || chosen.id !== rec.id) && (
        <p className="small muted">Rekomendasi Foundry: <strong className="fg">{g?.templates?.find((t) => t.id === rec.id)?.name}</strong> — {rec.reasons?.join("; ")}</p>
      )}
      <div className="tpl-filters">
        <label className="small">Domain contoh{" "}
          <select value={domain} onChange={(e) => setDomain(e.target.value)}>
            {DOMAINS.map(([v, l]) => <option key={v} value={v}>{l}</option>)}
          </select>
        </label>
        <label className="small">Theme pratinjau{" "}
          <select value={theme} onChange={(e) => setTheme(e.target.value)}>
            <option value="">Theme proyek</option>
            {themes.map((t) => <option key={t.id} value={t.id}>{t.name}</option>)}
          </select>
        </label>
      </div>
      {error && <p className="warn-text small">{error}</p>}
      <div className="tpl-grid">
        {g?.templates?.map((t) => (
          // A div, not a button: the preview draws buttons of its own.
          <div key={t.id} role="button" tabIndex={0} aria-pressed={open === t.id} className={`tpl-card ${open === t.id ? "on" : ""}`}
            onClick={() => setOpen(open === t.id ? null : t.id)}
            onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); setOpen(open === t.id ? null : t.id); } }}>
            <span className="tpl-thumb" aria-hidden>
              <span className="tpl-thumb-inner" style={{ transform: `scale(${THUMB_SCALE})` }}>
                {g.previews[t.id] && (
                  <Wireframe page={g.previews[t.id]} pages={[g.previews[t.id]]} device="desktop" mode="run" onNavigate={() => {}}
                    app={g.previews[t.id].name} system={g.system} theme={g.theme} />
                )}
              </span>
            </span>
            <strong>{t.name}{chosen?.id === t.id ? " · dipakai" : rec?.id === t.id ? " · rekomendasi" : ""}</strong>
            <span className="small muted">{t.layout_pattern.structure}</span>
            <span className="theme-tags">{t.characteristics.map((c) => <i key={c}>{c}</i>)}</span>
          </div>
        ))}
      </div>
      {detail && <TemplateDetail t={detail} />}
      {g?.invalid?.length ? <ul className="small warn-text">{g.invalid.map((p) => <li key={p.id}>Template <code>{p.id}</code> tidak dipakai: {p.error}</li>)}</ul> : null}
    </div>
  );
}

function TemplateDetail({ t }: { t: DesignTemplate }) {
  const fit = t.suitable_for;
  return (
    <section className="tpl-detail">
      <h2>{t.name}</h2>
      <p>{t.intent}</p>
      <dl className="small">
        <dt>Cocok untuk</dt>
        <dd>{[fit.domains?.join(", "), fit.roles?.length ? `peran ${fit.roles.join(", ")}` : "", fit.tasks?.length ? `tugas ${fit.tasks.join(", ")}` : "",
          fit.frequency ? `frekuensi ${fit.frequency}` : "", fit.density ? `densitas ${fit.density}` : ""].filter(Boolean).join(" · ")}</dd>
        <dt>Pola layout</dt>
        <dd>{t.layout_pattern.strategies.join(" / ")} · navigasi {t.layout_pattern.navigation} · {t.layout_pattern.density}</dd>
        <dt>Hierarki informasi</dt>
        <dd>{t.information_hierarchy.join(" › ")}</dd>
        <dt>Komposisi</dt>
        <dd><ul>{t.component_composition.map((r) => <li key={r.region}><code>{r.region}</code> {r.role} · {r.type} · {r.area} — {r.purpose}</li>)}</ul></dd>
        <dt>Responsif</dt>
        <dd>{t.responsive_strategy.strategy}: desktop {t.responsive_strategy.desktop}; mobile {t.responsive_strategy.mobile}</dd>
        <dt>Diharapkan</dt>
        <dd>{t.quality_rules.expected_behavior.join("; ")}</dd>
        <dt>Dihindari</dt>
        <dd>{t.quality_rules.avoid.map((a) => AVOID[a] ?? a).join(", ")}</dd>
      </dl>
    </section>
  );
}
