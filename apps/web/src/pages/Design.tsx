import { useEffect, useLayoutEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { AskChange, ChangeBar, Inspect, Review } from "../components/DesignPanels";
import { Wireframe, type PageState } from "../components/Wireframe";
import { DEVICES, defaultDevice, flowPages, groupPages, pageOrder, roleOf, SCREEN_WIDTH, type Device } from "../design";
import { isWorking } from "../flow";
import { useProject } from "../store/project";
import { useWorkbench } from "../store/workbench";
import { api } from "../api";
import type { Blocker, DesignHandoff, PageHandoff, PageStates, Project, TechStack } from "../types";
import { NONE, TECH_AREA, techChoice } from "../tech";
import { ThemeGallery } from "../components/ThemeGallery";
import { RegenerateButton, TemplateLibrary } from "../components/TemplateGallery";
import { Architecture, Features, PlatformStructure, Summary } from "./DesignInfo";
import { CanvasComponent, CanvasPage, CanvasWarnings, Layers, PrototypeMap, Versions } from "../components/Canvas";

type Mode = "pratinjau" | "prototipe" | "kode" | "tinjau";
type Info = "fitur" | "platform" | "kebutuhan" | "arsitektur" | "tema" | "template" | "peta" | "versi";

const MODES: [Mode, string][] = [["pratinjau", "Desain"], ["prototipe", "Prototipe"], ["kode", "Kode"], ["tinjau", "Tinjau"]];
const STATES: [PageState, string, keyof PageStates | null][] = [
  ["normal", "Normal", null], ["loading", "Memuat", "loading"], ["empty", "Kosong", "empty"],
  ["no_results", "Tanpa hasil", "no_results"], ["error", "Error", "error"], ["success", "Berhasil", "success"],
];
const INFOS: [Info, string][] = [["fitur", "Fitur & Alur"], ["platform", "Platform & Teknologi"], ["kebutuhan", "Kebutuhan"], ["arsitektur", "Arsitektur"], ["tema", "Theme"], ["template", "Template"], ["peta", "Peta Prototipe"], ["versi", "Versi"]];
// Explorer deep links use ?tab=fitur|platform|ringkasan|arsitektur.
const TAB_INFO: Record<string, Info> = { fitur: "fitur", platform: "platform", ringkasan: "kebutuhan", arsitektur: "arsitektur" };

// Logical screen width per device; the frame is scaled down to fit.
const MIN_SCALE = 0.65;

export function DesignPage() {
  const { project } = useProject();
  if (!project) return null;
  if (!project.design?.pages?.length) return <Designing project={project} />;
  return <Workspace project={project} />;
}

function Workspace({ project }: { project: Project }) {
  const [params] = useSearchParams();
  const design = project.design!;
  const pages = design.pages ?? [];
  const flows = project.flows ?? [];
  const groups = useMemo(() => groupPages(pages, flows), [pages, flows]);
  const order = useMemo(() => pageOrder(groups), [groups]);

  const [mode, setMode] = useState<Mode>("pratinjau");
  const [info, setInfo] = useState<Info | null>(null);
  const [pageId, setPageId] = useState(params.get("page") ?? order[0]?.id);
  const [device, setDevice] = useState<Device | null>(null); // null = follows the page platform
  const [selected, setSelected] = useState<number | null>(null);
  const [inspect, setInspect] = useState(false);
  const [sideOpen, setSideOpen] = useState(false);
  const [asking, setAsking] = useState(false);
  const [flowId, setFlowId] = useState<string | null>(null);
  const [history, setHistory] = useState<string[]>([]);
  const [pstate, setPstate] = useState<PageState>("normal");
  const [handoff, setHandoff] = useState<DesignHandoff | null>(null);

  // The explorer takes canvas space; hide it here and restore it on leave.
  useEffect(() => {
    if (!useWorkbench.getState().sidebar) return;
    useWorkbench.getState().toggleSidebar();
    return () => { if (!useWorkbench.getState().sidebar) useWorkbench.getState().toggleSidebar(); };
  }, []);

  // Handoff and blockers are derived by the server; refresh on every change.
  useEffect(() => {
    let live = true;
    api.designHandoff().then((h) => live && setHandoff(h), () => live && setHandoff(null));
    return () => { live = false; };
  }, [project.updated_at]);

  useEffect(() => {
    const t = params.get("tab");
    const p = params.get("page");
    if (t && TAB_INFO[t]) setInfo(TAB_INFO[t]);
    else if (p) { setPageId(p); setInfo(null); setMode("pratinjau"); }
  }, [params]);
  useEffect(() => {
    if (!pages.some((p) => p.id === pageId)) setPageId(order[0]?.id);
  }, [pages, pageId, order]);

  const page = pages.find((p) => p.id === pageId) ?? order[0];
  const dev = device ?? defaultDevice(page);
  const approved = design.status === "approved";
  const working = isWorking(project);
  const changed = new Set((design.revision?.changes ?? []).map((c) => c.page_id).filter(Boolean));

  const show = (id: string) => { setPageId(id); setPstate("normal"); setSelected(null); setInfo(null); setSideOpen(false); };
  const switchMode = (m: Mode) => { setMode(m); setInfo(null); setSelected(null); if (m === "prototipe") startFlow(currentFlow()?.id ?? null); };

  // ---- prototype ----
  const runnable = flows.filter((f) => flowPages(f, pages).length > 0);
  const currentFlow = () => runnable.find((f) => f.page_ids?.includes(page?.id ?? "")) ?? runnable[0];
  const flow = runnable.find((f) => f.id === flowId);
  const steps = flow ? flowPages(flow, pages) : [];
  const step = steps.findIndex((p) => p.id === page?.id);
  // Starts a flow at its first page; keeps the current page when it is
  // already part of the flow, unless restart is set.
  const startFlow = (id: string | null, restart = false) => {
    setFlowId(id);
    setHistory([]);
    const f = runnable.find((x) => x.id === id);
    const first = f && flowPages(f, pages)[0];
    if (first && (restart || !f.page_ids?.includes(page?.id ?? ""))) show(first.id);
  };
  const go = (id: string) => {
    if (id === page?.id) return;
    if (mode === "prototipe" && page) setHistory([...history, page.id]);
    show(id);
  };
  const back = () => {
    const prev = history[history.length - 1];
    if (!prev) return;
    setHistory(history.slice(0, -1));
    show(prev);
  };

  const states = STATES.filter(([, , key]) => !key || !!page?.states?.[key]);
  const idx = order.findIndex((p) => p.id === page?.id);
  const showInspect = inspect && mode === "pratinjau" && !info && !!page;

  return (
    <div className="dw">
      <header className="dw-head">
        <button className="ghost dw-side-toggle" onClick={() => setSideOpen(!sideOpen)}>Halaman</button>
        <div className="dw-title">
          <strong>Rancangan</strong>
          <Status project={project} />
        </div>
        <div className="seg">
          {MODES.map(([k, l]) => (
            <button key={k} className={mode === k && !info ? "on" : ""} onClick={() => switchMode(k)}>{l}</button>
          ))}
        </div>
        <div className="dw-actions">
          {(mode === "pratinjau" || mode === "prototipe") && !info && (
            <div className="seg subtle">
              {DEVICES.map((d) => <button key={d.key} className={dev === d.key ? "on" : ""} onClick={() => setDevice(d.key)}>{d.label}</button>)}
            </div>
          )}
          {!approved && <RegenerateButton project={project} />}
          {!approved && <button className="ask-btn" disabled={working} onClick={() => setAsking(!asking)}>✦ Minta perubahan</button>}
          {mode === "pratinjau" && !info && (
            <button className={`ghost ${inspect ? "on" : ""}`} title="Panel detail" onClick={() => setInspect(!inspect)}>Detail</button>
          )}
        </div>
      </header>

      {asking && <AskChange page={mode !== "tinjau" && !info ? page : undefined} pages={order} component={selected} device={dev} onClose={() => setAsking(false)} />}
      <ChangeBar project={project} onOpen={(id) => { setMode("pratinjau"); show(id); }} />
      {!info && mode === "pratinjau" && <CanvasWarnings page={page} />}

      <div className={`dw-main ${sideOpen ? "side-open" : ""}`}>
        <aside className="dw-side">
          {groups.map((g) => (
            <div key={g.role} className="dw-group">
              <div className="dw-role">{g.role}</div>
              {g.loose.map((p) => <PageItem key={p.id} name={p.name} active={!info && p.id === page?.id} changed={changed.has(p.id)} onClick={() => go(p.id)} />)}
              {g.flows.map(({ flow: f, pages: ps }) => (
                <div key={f.id} className="dw-flow">
                  <div className="dw-flow-name">{f.name}</div>
                  {ps.map((p) => <PageItem key={p.id} name={p.name} active={!info && p.id === page?.id} changed={changed.has(p.id)} onClick={() => go(p.id)} />)}
                </div>
              ))}
            </div>
          ))}
          <div className="dw-side-foot">
            {INFOS.map(([k, l]) => (
              <button key={k} className={`dw-page ${info === k ? "active" : ""}`} onClick={() => { setInfo(k); setSideOpen(false); }}>{l}</button>
            ))}
          </div>
        </aside>

        <main className="dw-center">
          {info === "fitur" ? <Features project={project} />
            : info === "platform" ? <PlatformStructure project={project} />
            : info === "kebutuhan" ? <Summary project={project} />
            : info === "arsitektur" ? <Architecture project={project} />
            : info === "tema" ? <div className="info"><h1>Theme</h1><ThemeGallery project={project} /></div>
            : info === "template" ? <TemplateLibrary project={project} />
            : info === "peta" ? <PrototypeMap project={project} onOpen={(id) => { setMode("pratinjau"); show(id); }} onTry={(id) => { setMode("prototipe"); setInfo(null); startFlow(id, true); }} />
            : info === "versi" ? <Versions project={project} onOpen={(id) => { setMode("pratinjau"); show(id); }} />
            : mode === "tinjau" ? (
              <Review project={project} blockers={handoff?.blockers ?? []} onAsk={() => setAsking(true)} onInfo={setInfo}
                onOpen={(id) => { setMode("pratinjau"); show(id); }}
                onTry={(id) => { setMode("prototipe"); startFlow(id, true); }} />
            ) : mode === "kode" ? (
              page && <CodeHandoff key={page.id} stack={handoff?.stack} handoff={handoff?.pages?.find((h) => h.page_id === page.id)} blockers={(handoff?.blockers ?? []).filter((b) => b.page_id === page.id)} />
            ) : page && (
              <div className={`dw-stage ${dev}`}>
                <Fit width={SCREEN_WIDTH[dev]}>
                <Wireframe app={project.name} page={page} pages={pages} device={dev} mode={mode === "prototipe" ? "run" : "select"} state={mode === "pratinjau" ? pstate : "normal"}
                  selected={selected} onSelect={(i) => { setSelected(i); if (i !== null) setInspect(true); }} onNavigate={go}
                  system={project.design?.system} theme={project.design?.theme} />
                </Fit>
              </div>
            )}
        </main>

        {showInspect && (
          <aside className="dw-inspect">
            <Layers page={page!} selected={selected} onSelect={setSelected} />
            {selected !== null && page!.components?.[selected]
              ? <CanvasComponent project={project} page={page!} index={selected} onRemoved={() => setSelected(null)} />
              : <CanvasPage project={project} page={page!} />}
            <Inspect project={project} page={page!} handoff={handoff?.pages?.find((h) => h.page_id === page!.id)} selected={selected} onSelect={setSelected} onOpen={go} />
          </aside>
        )}
      </div>

      {!info && mode === "pratinjau" && page && (
        <footer className="dw-foot">
          <button className="ghost" disabled={idx <= 0} onClick={() => go(order[idx - 1].id)}>← Sebelumnya</button>
          <span className="inline">
            <span className="muted">{roleOf(page)} / <strong className="fg">{page.name}</strong></span>
            {states.length > 1 && (
              <span className="seg subtle" title="Kondisi halaman">
                {states.map(([k, l]) => <button key={k} className={pstate === k ? "on" : ""} onClick={() => setPstate(k)}>{l}</button>)}
              </span>
            )}
          </span>
          <button className="ghost" disabled={idx >= order.length - 1} onClick={() => go(order[idx + 1].id)}>Berikutnya →</button>
        </footer>
      )}
      {!info && mode === "prototipe" && page && (
        <footer className="dw-foot">
          <button className="ghost" disabled={history.length === 0} onClick={back}>← Kembali</button>
          <span className="inline">
            {runnable.length > 0 ? (
              <select value={flowId ?? ""} onChange={(e) => startFlow(e.target.value, true)}>
                {runnable.map((f) => <option key={f.id} value={f.id}>{f.name}</option>)}
              </select>
            ) : <span className="muted">Belum ada alur</span>}
            {flow && <span className="muted">{step >= 0 ? `Langkah ${step + 1} dari ${steps.length}` : "Di luar alur"}</span>}
          </span>
          {flow && step >= 0 && step < steps.length - 1 ? (
            <button className="ghost" onClick={() => go(steps[step + 1].id)}>Lanjut →</button>
          ) : flow ? (
            <button className="ghost" onClick={() => { setHistory([]); show(steps[0].id); }}>Ulangi</button>
          ) : <span />}
        </footer>
      )}
    </div>
  );
}

// Code: the implementation handoff of one page, derived from the Design IR;
// the build receives exactly this.
function CodeHandoff({ handoff, blockers, stack }: { handoff?: PageHandoff; blockers: Blocker[]; stack?: TechStack | null }) {
  const [copied, setCopied] = useState(false);
  if (!handoff) return <div className="code-view"><p className="muted">Handoff belum tersedia.</p></div>;
  const tech = (stack?.decisions ?? []).filter((d) => techChoice(d) !== NONE);
  // The copied spec carries the technology so the code follows the same decision.
  const technology = Object.fromEntries(tech.map((d) => [d.area, techChoice(d)]));
  const json = JSON.stringify(tech.length ? { technology, page: handoff } : handoff, null, 2);
  return (
    <div className="code-view">
      <h1>{handoff.name}</h1>
      <p className="small muted">
        <code>{handoff.route || "route belum ditentukan"}</code> · {handoff.platform} · {handoff.access === "auth" ? "harus login" : handoff.access || "akses belum ditentukan"}
      </p>
      {tech.length > 0 && <p className="small">Dibangun dengan: {tech.map((d) => `${TECH_AREA[d.area] ?? d.area} ${techChoice(d)}`).join(" · ")}</p>}
      {blockers.map((b, i) => <p key={i} className="warn-text small">! {b.text}</p>)}
      <p className="small muted">Handoff implementasi dari Design IR — build memakai data yang sama.</p>
      <button className="ghost small" onClick={() => navigator.clipboard?.writeText(json).then(() => setCopied(true), () => {})}>{copied ? "Tersalin" : "Salin JSON"}</button>
      <pre>{json}</pre>
    </div>
  );
}

function PageItem({ name, active, changed, onClick }: { name: string; active: boolean; changed: boolean; onClick: () => void }) {
  return (
    <button className={`dw-page ${active ? "active" : ""}`} onClick={onClick}>
      <span>{name}</span>
      {changed && <span className="dw-dot" title="Berubah di revisi terakhir" />}
    </button>
  );
}

function Status({ project }: { project: Project }) {
  const d = project.design!;
  const job = project.job;
  if (d.status === "approved") return <span className="dw-status ok">✓ Disetujui</span>;
  if (isWorking(project) && job?.kind === "design") return <span className="dw-status"><span className="spinner" /> Memperbarui rancangan</span>;
  if (isWorking(project) && job?.kind === "plan") return <span className="dw-status"><span className="spinner" /> Membuat task</span>;
  if (d.revision) return <span className="dw-status warn">● Perubahan belum disimpan</span>;
  return <span className="dw-status ok">✓ Siap ditinjau</span>;
}

// First design run: progress from the real pipeline state, no fake steps.
function Designing({ project }: { project: Project }) {
  const job = project.job;
  const working = isWorking(project);
  const failed = job?.status === "failed";
  if (!working && !failed) {
    return (
      <div className="dw-empty">
        <h1>Belum ada rancangan</h1>
        <p className="muted">Rancangan dibuat dari cerita dan kebutuhan aplikasimu.</p>
        <Link className="button primary" to="/simple/tell">Ceritakan idemu</Link>
      </div>
    );
  }
  const state = (kind: string, done: boolean) =>
    done ? "done" : job?.kind === kind ? (failed ? "failed" : "active") : "todo";
  const steps: [string, string][] = [
    ["Cerita diterima", project.story ? "done" : "todo"],
    ["Memahami kebutuhan & blueprint", state("discovery", !!project.discovery)],
    ...(project.reference ? [["Menganalisis referensi " + project.reference.name, state("reference", ["done", "failed"].includes(project.reference.status))] as [string, string]] : []),
    ["Membuat halaman dan alur", state("design", false)],
    ["Siap ditinjau", "todo"],
  ];
  const mark: Record<string, string> = { done: "✓", active: "●", failed: "✕", todo: "○" };
  return (
    <div className="dw-empty">
      <h1>Merancang aplikasimu</h1>
      <p className="muted">Foundry mengubah kebutuhan menjadi halaman dan alur pengguna. Biasanya butuh beberapa saat.</p>
      <ul className="dw-steps">
        {steps.map(([l, s]) => <li key={l} className={s}><span>{s === "active" ? <span className="spinner" /> : mark[s]}</span>{l}</li>)}
      </ul>
    </div>
  );
}

// Renders children at a device's logical width, scaled to the stage. Below
// MIN_SCALE the logical width shrinks instead so text stays readable.
function Fit({ width, children }: { width: number; children: ReactNode }) {
  const ref = useRef<HTMLDivElement>(null);
  const [box, setBox] = useState({ w: 0, h: 0 });
  useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return;
    const ro = new ResizeObserver(([e]) => setBox({ w: e.contentRect.width, h: e.contentRect.height }));
    ro.observe(el);
    return () => ro.disconnect();
  }, []);
  const scale = box.w ? Math.min(1, Math.max(MIN_SCALE, box.w / width)) : 1;
  const w = Math.min(width, box.w / scale);
  return (
    <div ref={ref} className="dw-fit">
      {box.w > 0 && (
        <div className="dw-fit-inner" style={{ width: w, height: box.h / scale, transform: `translateX(-50%) scale(${scale})` }}>
          {children}
        </div>
      )}
    </div>
  );
}
