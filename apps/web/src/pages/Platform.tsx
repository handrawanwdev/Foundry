import { useState } from "react";
import { api } from "../api";
import { isWorking, PLATFORM_LABEL } from "../flow";
import { StructurePanel } from "../components/StructurePanel";
import { TechStackPanel } from "../components/TechStack";
import { TechWorkspace } from "../components/TechWorkspace";
import { useProject } from "../store/project";
import type { ExistingUI, PlatformKind, PlatformTarget, Project } from "../types";

// Platform, technology and project structure: the decisions the blueprint
// is built on.

const KINDS: PlatformKind[] = ["web", "android", "ios", "desktop", "backend", "cli", "embedded", "other"];

export function PlatformStructure({ project }: { project: Project }) {
  const { run } = useProject();
  const locked = project.stage === "READY";
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
    <div className="repo plat">
      <header className="repo-head">
        <div>
          <h1>Platform & Teknologi</h1>
          <p className="small muted">Di mana produk berjalan, stack yang dipilih, dan bentuk struktur project.</p>
        </div>
      </header>
      <section className="plat-targets">
        <h3>Target platform</h3>
        <div className="plat-kinds">
          {KINDS.map((k) => {
            const t = targets.find((x) => x.kind === k);
            return (
              <label key={k} className={`plat-kind${t ? " on" : ""}`} title={t?.rationale || undefined}>
                <input type="checkbox" checked={!!t} disabled={locked} onChange={() => toggle(k)} />
                {PLATFORM_LABEL[k]}
              </label>
            );
          })}
        </div>
        {project.platform?.reason && <p className="small muted">{project.platform.reason}</p>}
      </section>
      {project.stack?.plan ? <TechWorkspace project={project} /> : (
        <>
          <TechStackPanel project={project} />
          <TechPlanInvite project={project} />
        </>
      )}
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

function TechPlanInvite({ project }: { project: Project }) {
  const { run, pending } = useProject();
  const [text, setText] = useState("");
  const locked = project.stage === "READY" || isWorking(project) || pending === "stack" || !!project.stack?.proposal || !!project.stack?.request;
  if (!project.stack) return null;
  return (
    <section className="info-sec tp-invite">
      <h3>Rancang teknologi dengan AI</h3>
      <p className="small muted">AI menganalisis kebutuhan dan memilih seluruh stack — termasuk di luar katalog Foundry — beserta alasan, alternatif, dan tinjauan kompatibilitas.</p>
      <textarea rows={2} value={text} maxLength={6000} disabled={locked}
        placeholder="Arahan opsional, mis. “tim sudah Elixir, butuh realtime dan transaksi”"
        onChange={(e) => setText(e.target.value)} />
      <div className="repo-actions">
        <button className="primary" disabled={locked} onClick={() => void run("stack", () => api.generateTechPlan(text.trim()))}>Rancang dengan AI</button>
      </div>
      {project.stack.plan_request?.rejected && <p className="small warn-text">Jawaban AI sebelumnya ditolak: {project.stack.plan_request.rejected}</p>}
    </section>
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
