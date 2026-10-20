import { api } from "../api";
import { PLATFORM_LABEL } from "../flow";
import { StructurePanel } from "../components/StructurePanel";
import { TechStackPanel } from "../components/TechStack";
import { useProject } from "../store/project";
import type { ExistingUI, PlatformKind, PlatformTarget, Project } from "../types";

// Platform, technology and project structure: the decisions the blueprint
// is built on.

const KINDS: PlatformKind[] = ["web", "android", "ios", "desktop", "backend", "cli", "embedded", "other"];

export function PlatformStructure({ project }: { project: Project }) {
  const { run } = useProject();
  const locked = project.stage === "READY" || !!project.platform?.approved;
  const targets = project.platform?.targets ?? [];
  const has = (k: PlatformKind) => targets.some((t) => t.kind === k);

  const toggle = (k: PlatformKind) => {
    const next: PlatformTarget[] = has(k)
      ? targets.filter((t) => t.kind !== k)
      : [...targets, { kind: k, rationale: "Ditambahkan oleh user" }];
    void run("platform", () => api.setPlatform(next));
  };
  const s = project.structure;

  return (
    <div className="info">
      <h1>Platform & Teknologi</h1>
      <section className="info-sec">
        <h3>Target Platform</h3>
        <p className="small muted">Di mana produk berjalan. Pengguna, peran, dan izin diatur di Requirements dan Security, bukan di sini.</p>
        {KINDS.map((k) => {
          const t = targets.find((x) => x.kind === k);
          return (
            <label key={k} className="check">
              <input type="checkbox" checked={!!t} disabled={locked} onChange={() => toggle(k)} />
              <strong>{PLATFORM_LABEL[k]}</strong>
              {t?.rationale && <span className="muted small"> — {t.rationale}</span>}
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
        </section>
      ) : s?.ir ? <StructurePanel project={project} />
        : <section className="info-sec"><h3>Struktur project</h3><p className="muted">Belum ditentukan</p></section>}
    </div>
  );
}

const NAV_LABEL = { side: "menu samping", top: "menu atas" } as const;

// What the existing frontend has; the design rules and new pages follow it.
function ExistingUIInfo({ ui }: { ui: ExistingUI }) {
  const list = (items: string[] | undefined, label: string) =>
    items?.length ? <details className="small"><summary>{label} ({items.length})</summary>{items.map((x) => <div key={x}><code>{x}</code></div>)}</details> : null;
  return (
    <div className="small">
      <p><strong>UI yang sudah ada</strong> — aturan desain dan halaman baru mengikuti ini.</p>
      {ui.navigation && <div>Navigasi: {NAV_LABEL[ui.navigation]}{ui.nav_items?.length ? ` (${ui.nav_items.join(", ")})` : ""}</div>}
      {ui.tokens
        ? <div>Token desain dari {ui.token_sources?.join(", ")} dipakai di aturan desain</div>
        : <div className="muted">Token desain tidak ditemukan di kode; aturan desain menyusun token baru.</div>}
      {list(ui.routes, "Route")}
      {list(ui.shell, "Layout & navigasi")}
      {list(ui.components, "Komponen bersama")}
    </div>
  );
}
