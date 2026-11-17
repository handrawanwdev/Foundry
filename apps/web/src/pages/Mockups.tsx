import { Fragment, useEffect, useRef, useState, type ReactNode, type RefObject } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { api } from "../api";
import { DESIGN_DOCS, SCOPE_DOCS } from "../blueprint";
import { APPROVAL, BenchmarkPanel, Frame, FullView, GatePanel, LifecyclePanel, STATE_LABEL, VIEWPORTS, type Viewport } from "../components/Mockup";
import { frameGroups, mobilePlatform, mockupGroups, pageFindings, withQuery } from "../mockups";
import { useProject } from "../store/project";
import type { DesignBenchmark, DesignGate, DesignView, FlowStep, FlowWalk, LayoutPage, MockupFrame, MockupStatus, Project } from "../types";

// Mockup workspace: the designed pages on their own, like a design tool.
// The pages panel lists them by the navigation of the layout spec, and
// under the open page its frames (each dialog, failure, success and state
// the page has); the canvas shows all pages at once, all frames of one
// page, or one frame to try out at a viewport;
// the guide shows what Foundry's own system says the page must be (layout
// spec, use cases, quality gate). The mockups are drawn from those, so a
// change is made there, never in the mockup.

// Canvas zoom, and the zoom of one page (fit to the width when unset).
const ZOOM = { min: 0.1, max: 1, step: 0.05, start: 0.2 } as const;
const PAGE_ZOOM = { min: 0.25, max: 2, step: 0.25 } as const;

const clampZoom = (z: number, lim: { min: number; max: number }) => Math.round(Math.min(lim.max, Math.max(lim.min, z)) * 100) / 100;

// useCtrlWheel zooms on Ctrl+scroll (and a trackpad pinch) over the
// element: native and not passive, so the browser does not zoom the page.
function useCtrlWheel(ref: RefObject<HTMLElement | null>, zoomBy: (dir: number) => void) {
  const fn = useRef(zoomBy);
  fn.current = zoomBy;
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    const wheel = (e: WheelEvent) => {
      if (!e.ctrlKey && !e.metaKey) return;
      e.preventDefault();
      fn.current(-Math.sign(e.deltaY));
    };
    el.addEventListener("wheel", wheel, { passive: false });
    return () => el.removeEventListener("wheel", wheel);
  }, [ref]);
}

interface Mockups { list: MockupStatus[]; flows: FlowWalk[]; gate: DesignGate | null; bench: DesignBenchmark | null }

const blueprintDoc = (id: string) => `/blueprint?doc=${id}`;

const frameLabel = (f: MockupFrame) => (f.kind === "state" ? STATE_LABEL[f.name] ?? f.name : f.name);

export function MockupsPage() {
  const { project, run, pending } = useProject();
  const [design, setDesign] = useState<DesignView | null>(null);
  const [data, setData] = useState<Mockups | null>(null);
  const [params, setParams] = useSearchParams();
  const [role, setRole] = useState("");
  const [flowID, setFlowID] = useState("");
  const [query, setQuery] = useState("");
  const [zoom, setZoom] = useState<number>(ZOOM.start);

  useEffect(() => {
    if (!project?.discovery) return;
    api.design().then(setDesign).catch(() => setDesign(null));
    api.mockups()
      .then((r) => setData({ list: r.mockups, flows: r.flows ?? [], gate: r.gate ?? null, bench: r.benchmark ?? null }))
      .catch(() => setData({ list: [], flows: [], gate: null, bench: null }));
  }, [project?.version, project?.dir]);

  if (!project) return null;
  if (!data) return <div className="dw-empty"><span className="inline"><span className="spinner" /> Memuat mockup…</span></div>;
  if (data.list.length === 0) {
    return (
      <div className="dw-empty">
        <h1>Belum ada mockup</h1>
        <p className="muted">Mockup dirender dari spesifikasi layout. Tulis atau lengkapi dulu halamannya di blueprint.</p>
        <Link className="button primary" to={blueprintDoc(DESIGN_DOCS.layout)}>Buka spesifikasi layout</Link>
      </div>
    );
  }

  const { list, flows, gate, bench } = data;
  const layout = design?.design?.layout;
  const specs = layout?.pages ?? [];
  const pageID = params.get("page") ?? "";
  const frameID = params.get("frame") ?? "";
  const current = list.find((m) => m.page === pageID);
  const frame = current?.frames?.find((f) => f.id === frameID);
  const roles = [...new Set([...flows.map((f) => f.role ?? ""), ...specs.map((p) => p.role ?? "")].map((r) => r.trim()).filter(Boolean))];
  const roleFlows = role ? flows.filter((f) => f.role === role) : flows;
  const rolePages = new Set([...roleFlows.flatMap((f) => f.pages ?? []), ...specs.filter((p) => p.role === role).map((p) => p.id)]);
  const flow = flows.find((f) => f.id === flowID);
  const q = query.trim().toLowerCase();
  const shown = list.filter((m) => (!role || rolePages.has(m.page))
    && (!q || m.name.toLowerCase().includes(q) || m.page.includes(q) || m.pattern.includes(q)));
  const groups = mockupGroups(shown, layout, flow);
  const approved = list.filter((m) => m.status === "approved").length;
  const busy = pending === "approveMockups";
  const approve = (pages: string[]) =>
    void run("approveMockups", async () => {
      const r = await api.approveMockups(pages);
      setData((d) => d && { ...d, list: r.mockups });
    });
  const openPage = (id?: string) => { if (id) setParams({ page: id }); };
  const openFrame = (page: string, id: string) => setParams({ page, frame: id });
  const overview = () => setParams({});
  const pickRole = (r: string) => {
    setRole(r);
    if (flow && r && flow.role !== r) setFlowID("");
  };

  return (
    <div className="ws mw">
      <nav className="ws-nav mw-pages" aria-label="Halaman mockup">
        <div className="ws-nav-head">
          <strong>Halaman</strong>
          <span className="muted small">{approved}/{list.length} disetujui</span>
        </div>
        <button className={`ws-item ${current ? "" : "on"}`} onClick={overview} aria-current={current ? undefined : "page"}>
          <span className="ws-item-text">Overview kanvas</span>
          <span className="ws-count">{shown.length}</span>
        </button>
        <div className="mw-filters">
          <input type="search" placeholder="Cari halaman" value={query} onChange={(e) => setQuery(e.target.value)} aria-label="Cari halaman" />
          {roles.length > 1 && (
            <select value={role} onChange={(e) => pickRole(e.target.value)} aria-label="Peran">
              <option value="">Semua peran</option>
              {roles.map((r) => <option key={r} value={r}>{r}</option>)}
            </select>
          )}
          {roleFlows.length > 0 && (
            <select value={flowID} onChange={(e) => setFlowID(e.target.value)} aria-label="Use case">
              <option value="">Semua use case</option>
              {roleFlows.map((f) => <option key={f.id} value={f.id}>{f.id} · {f.name}</option>)}
            </select>
          )}
        </div>
        {groups.map((g) => (
          <div key={g.name} className="ws-group">
            <div className="ws-group-label">{g.name}</div>
            {g.pages.map((m, i) => (
              <Fragment key={m.page}>
                <button className={`ws-item ${m.page === pageID && !frame ? "on" : ""}`} onClick={() => openPage(m.page)}
                  aria-current={m.page === pageID ? "page" : undefined} title={`${m.name} · ${m.pattern} · ${APPROVAL[m.status].label}`}>
                  <span className={`cmd-dot ${APPROVAL[m.status].cls}`} aria-label={APPROVAL[m.status].label} />
                  <span className="ws-item-text">{flow ? `${i + 1}. ` : ""}{m.name}</span>
                  <span className="mw-pattern">{m.pattern}</span>
                </button>
                {m.page === pageID && (m.frames?.length ?? 0) > 0 && (
                  <div className="mw-frames" role="group" aria-label={`Frame ${m.name}`}>
                    {frameGroups(m.frames).map((fg) => (
                      <div key={fg.name}>
                        <div className="mw-frame-group">{fg.name === "State" ? "State halaman" : fg.name}</div>
                        {fg.frames.map((f) => (
                          <button key={f.id} className={`mw-frame mw-frame-${f.kind} ${f.id === frameID ? "on" : ""}`}
                            aria-current={f.id === frameID ? "page" : undefined} onClick={() => openFrame(m.page, f.id)}>
                            {frameLabel(f)}
                          </button>
                        ))}
                      </div>
                    ))}
                  </div>
                )}
              </Fragment>
            ))}
          </div>
        ))}
        {groups.length === 0 && <p className="muted small mw-none">Tidak ada halaman yang cocok.</p>}
      </nav>
      <main className="mw-main">
        {current && frame
          ? <PageView key={current.page} project={project} mockup={current} frame={frame} spec={specs.find((p) => p.id === current.page)}
              flow={flow} flows={flows} list={list} role={role} busy={busy} onApprove={approve} onOpen={openPage}
              onBack={() => openPage(current.page)} onFlow={setFlowID} />
          : current
            ? <Board zoom={zoom} setZoom={setZoom}
                head={<>
                  <button className="ghost" onClick={overview}>← Overview</button>
                  <strong>{current.name}</strong>
                  <span className={`state ${APPROVAL[current.status].cls}`}>{APPROVAL[current.status].label}</span>
                  <span className="muted small">Semua frame halaman ini. Klik frame untuk mencobanya sebagai prototype.</span>
                </>}
                end={<button disabled={busy || current.status === "approved"} onClick={() => approve([current.page])}>Setujui halaman ini</button>}
                groups={frameGroups(current.frames).map((fg) => ({
                  name: fg.name === "State" ? "State halaman" : fg.name,
                  cards: fg.frames.map((f) => ({
                    key: f.id, name: frameLabel(f), mobile: mobilePlatform(current.platform),
                    src: withQuery(api.mockupURL(current.page, "", { role }), f.query, `v=${project.version}`),
                    onOpen: () => openFrame(current.page, f.id),
                  })),
                }))} />
            : <Board zoom={zoom} setZoom={setZoom}
                head={<>
                  <strong>Overview kanvas</strong>
                  <span className="muted small">Klik halaman untuk membuka frame-nya. Geser latar untuk menggeser, Ctrl+scroll atau pinch untuk zoom.</span>
                </>}
                end={list.length - approved > 0 && (
                  <button className="primary" disabled={busy} onClick={() => approve([])}>Setujui semua ({list.length - approved})</button>
                )}
                groups={groups.map((g) => ({
                  name: g.name,
                  cards: g.pages.map((m, i) => ({
                    key: m.page, name: `${flow ? `${i + 1}. ` : ""}${m.name}`, mobile: mobilePlatform(m.platform),
                    src: withQuery(api.mockupURL(m.page), `v=${project.version}`), dot: APPROVAL[m.status],
                    count: m.frames?.length, onOpen: () => openPage(m.page),
                  })),
                }))} />}
      </main>
      <aside className="mw-guide" aria-label="Panduan">
        {current
          ? <PageGuide mockup={current} spec={specs.find((p) => p.id === current.page)} flows={flows} gate={gate} list={list}
              onOpen={openPage} onFlow={setFlowID} />
          : <SystemGuide project={project} gate={gate} bench={bench} />}
      </aside>
    </div>
  );
}

interface Card { key: string; name: string; src: string; mobile: boolean; dot?: { cls: string; label: string }; count?: number; onOpen: () => void }

// Board lays frames out at their own viewport, a row per group: every
// page of the design, or every frame of one page. Drag the background to
// pan; Ctrl+scroll zooms.
function Board({ groups, zoom, setZoom, head, end }: {
  groups: { name: string; cards: Card[] }[]; zoom: number; setZoom: (f: (z: number) => number) => void; head: ReactNode; end?: ReactNode;
}) {
  const box = useRef<HTMLDivElement>(null);
  const drag = useRef<{ x: number; y: number; left: number; top: number } | null>(null);
  const clamp = (z: number) => clampZoom(z, ZOOM);
  useCtrlWheel(box, (dir) => setZoom((z) => clamp(z + dir * ZOOM.step)));
  return (
    <>
      <div className="mw-bar">
        {head}
        <span className="mw-bar-end">
          <span className="seg" role="group" aria-label="Zoom">
            <button onClick={() => setZoom((z) => clamp(z - ZOOM.step))} disabled={zoom <= ZOOM.min} aria-label="Perkecil">−</button>
            <button onClick={() => setZoom(() => ZOOM.start)} title="Zoom awal">{Math.round(zoom * 100)}%</button>
            <button onClick={() => setZoom((z) => clamp(z + ZOOM.step))} disabled={zoom >= ZOOM.max} aria-label="Perbesar">+</button>
          </span>
          {end}
        </span>
      </div>
      <div className="mw-canvas" ref={box}
        onPointerDown={(e) => {
          if (e.button !== 0 || (e.target as HTMLElement).closest(".mw-card")) return;
          drag.current = { x: e.clientX, y: e.clientY, left: box.current!.scrollLeft, top: box.current!.scrollTop };
          e.currentTarget.setPointerCapture(e.pointerId);
        }}
        onPointerMove={(e) => {
          const d = drag.current;
          if (!d) return;
          box.current!.scrollLeft = d.left - (e.clientX - d.x);
          box.current!.scrollTop = d.top - (e.clientY - d.y);
        }}
        onPointerUp={() => { drag.current = null; }}
        onPointerCancel={() => { drag.current = null; }}>
        {groups.map((g) => (
          <section key={g.name} className="mw-group">
            <h3>{g.name} <span className="muted small">{g.cards.length}</span></h3>
            <div className="mw-row">
              {g.cards.map((c) => {
                const vp = VIEWPORTS.find((v) => v.id === (c.mobile ? "mobile" : "desktop"))!;
                return (
                  <article key={c.key} className="mw-card" style={{ width: vp.width * zoom }}>
                    <div className="mw-thumb" style={{ width: vp.width * zoom, height: vp.height * zoom }}>
                      <iframe title={`Mockup ${c.name}`} src={c.src} loading="lazy" tabIndex={-1} aria-hidden sandbox="allow-same-origin"
                        style={{ width: vp.width, height: vp.height, transform: `scale(${zoom})` }} />
                    </div>
                    <div className="mw-card-label">
                      {c.dot && <span className={`cmd-dot ${c.dot.cls}`} aria-label={c.dot.label} />}
                      <span className="ws-item-text">{c.name}</span>
                      {(c.count ?? 0) > 1 && <span className="muted small">{c.count} frame</span>}
                    </div>
                    <button className="mw-card-hit" onClick={c.onOpen} aria-label={`Buka ${c.name}`} />
                  </article>
                );
              })}
            </div>
          </section>
        ))}
      </div>
    </>
  );
}

// PageView is one frame of a page at a viewport, live: the prototype runs
// from there, for the document and role it is simulated for, with the
// actions succeeding or failing, along the use case being walked.
function PageView({ project, mockup, frame, spec, flow, flows, list, role, busy, onApprove, onOpen, onBack, onFlow }: {
  project: Project; mockup: MockupStatus; frame: MockupFrame; spec?: LayoutPage; flow?: FlowWalk; flows: FlowWalk[]; list: MockupStatus[]; role: string;
  busy: boolean; onApprove: (pages: string[]) => void; onOpen: (id?: string) => void; onBack: () => void; onFlow: (id: string) => void;
}) {
  const [viewport, setViewport] = useState<Viewport>(mobilePlatform(spec?.platform) ? "mobile" : "desktop");
  const [fail, setFail] = useState(false);
  const [full, setFull] = useState(false);
  // undefined: fit the page to the width of the canvas.
  const [zoom, setZoom] = useState<number | undefined>();
  const stage = useRef<HTMLDivElement>(null);
  const zoomBy = (dir: number) => setZoom((z) => {
    const from = z ?? Math.min(1, ((stage.current?.clientWidth ?? 0) - 32) / VIEWPORTS.find((v) => v.id === viewport)!.width);
    return clampZoom(Math.round(from / PAGE_ZOOM.step) * PAGE_ZOOM.step + dir * PAGE_ZOOM.step, PAGE_ZOOM);
  });
  useCtrlWheel(stage, zoomBy);
  const [docStatus, setDocStatus] = useState("");
  const [docEntity, setDocEntity] = useState("");
  const vp = VIEWPORTS.find((v) => v.id === viewport)!;
  const lifecycles = project.lifecycles ?? [];
  const lc = lifecycles.find((l) => l.entity === docEntity) ?? lifecycles[0];
  const src = withQuery(api.mockupURL(mockup.page, "", { role, status: docStatus, entity: docEntity || lc?.entity }),
    fail && "outcome=error", frame.query, `v=${project.version}`);
  const approval = APPROVAL[mockup.status];
  return (
    <>
      <div className="mw-bar">
        <button className="ghost" onClick={onBack}>← {mockup.name}</button>
        <strong>{frame.group === "Halaman" ? frameLabel(frame) : `${frame.group} · ${frameLabel(frame)}`}</strong>
        <span className={`state ${approval.cls}`}>{approval.label}</span>
        <span className="mw-bar-end">
          <button disabled={busy || mockup.status === "approved"} onClick={() => onApprove([mockup.page])}>Setujui halaman ini</button>
        </span>
      </div>
      <div className="mw-bar mw-bar-sub">
        <div className="seg" role="group" aria-label="Viewport">
          {VIEWPORTS.map((v) => (
            <button key={v.id} className={viewport === v.id ? "on" : ""} aria-pressed={viewport === v.id} onClick={() => setViewport(v.id)}>{v.label}</button>
          ))}
        </div>
        <span className="seg" role="group" aria-label="Hasil aksi (simulasi)" title="Hasil saat Simpan, Hapus, atau aksi lain dijalankan di prototype">
          <button className={fail ? "" : "on"} aria-pressed={!fail} onClick={() => setFail(false)}>Aksi berhasil</button>
          <button className={fail ? "on" : ""} aria-pressed={fail} onClick={() => setFail(true)}>Aksi gagal</button>
        </span>
        {lifecycles.length > 1 && (
          <select value={docEntity} onChange={(e) => { setDocEntity(e.target.value); setDocStatus(""); }} aria-label="Dokumen">
            <option value="">Dokumen</option>
            {lifecycles.map((l) => <option key={l.entity} value={l.entity}>{l.entity}</option>)}
          </select>
        )}
        {lc && (
          <select value={docStatus} onChange={(e) => setDocStatus(e.target.value)} aria-label="Status dokumen (simulasi)">
            <option value="">Status awal</option>
            {lc.states.map((s) => <option key={s.id} value={s.id}>{s.label || s.id}{s.locked ? " · terkunci" : ""}</option>)}
          </select>
        )}
        <span className="seg" role="group" aria-label="Zoom halaman">
          <button onClick={() => zoomBy(-1)} disabled={zoom !== undefined && zoom <= PAGE_ZOOM.min} aria-label="Perkecil">−</button>
          <button onClick={() => setZoom(undefined)} className={zoom === undefined ? "on" : ""} title="Pas dengan lebar">
            {zoom === undefined ? "Pas" : `${Math.round(zoom * 100)}%`}
          </button>
          <button onClick={() => zoomBy(1)} disabled={zoom !== undefined && zoom >= PAGE_ZOOM.max} aria-label="Perbesar">+</button>
        </span>
        <button className="ghost" onClick={() => setFull(true)}>Layar penuh</button>
        <a className="button ghost" href={src} target="_blank" rel="noreferrer">Tab baru</a>
      </div>
      <div className="mw-page" ref={stage}>
        {flow && <FlowWalkPanel flow={flow} flows={flows} list={list} page={mockup.page} onOpen={onOpen} onFlow={onFlow} />}
        {!mockup.template && <p className="muted small">Pattern {mockup.pattern} belum punya template; mockup generik.</p>}
        <Frame key={`${frame.id}-${viewport}-${fail}`} src={src} width={vp.width} height={vp.height} title={`Mockup ${mockup.name}`} zoom={zoom} />
      </div>
      {full && <FullView src={src} title={`Mockup ${mockup.name}`} width={vp.width} onClose={() => setFull(false)} />}
    </>
  );
}

// FlowWalkPanel walks a use case step by step, page by page.
function FlowWalkPanel({ flow, flows, list, page, onOpen, onFlow }: {
  flow: FlowWalk; flows: FlowWalk[]; list: MockupStatus[]; page: string; onOpen: (id?: string) => void; onFlow: (id: string) => void;
}) {
  const walkable = flow.steps.filter((st) => !st.system && st.page);
  const at = walkable.findIndex((st) => st.page === page);
  const pageName = (id?: string) => list.find((m) => m.page === id)?.name ?? id ?? "";
  const go = (st?: FlowStep) => onOpen(st?.page);
  return (
    <div className="mockup-walk">
      <div className="mockup-walk-head">
        <strong>{flow.name}</strong>
        {flow.role && <span className="muted small">{flow.role}</span>}
        {flow.inferred && <span className="state st-review" title="Rancangan belum memetakan use case ini ke halaman; halaman ditebak dari langkahnya.">Halaman ditebak</span>}
        <span className="mockup-walk-nav">
          <button className="ghost" disabled={at <= 0} onClick={() => go(walkable[at - 1])}>← Sebelumnya</button>
          <button disabled={at < 0 || at >= walkable.length - 1} onClick={() => go(walkable[at + 1])}>Berikutnya →</button>
        </span>
      </div>
      {(flow.trigger || (flow.preconditions?.length ?? 0) > 0) && (
        <p className="muted small">
          {flow.trigger && <><b>Pemicu</b> {flow.trigger}. </>}
          {(flow.preconditions?.length ?? 0) > 0 && <><b>Syarat</b> {flow.preconditions!.join("; ")}</>}
        </p>
      )}
      <ol>
        {flow.steps.map((st) => (
          <li key={st.n} className={[st.system ? "sys" : "", st.page === page && !st.system ? "on" : ""].join(" ").trim() || undefined}>
            <button className="link" disabled={st.system || !st.page} onClick={() => go(st)}>{st.text}</button>
            <span className="muted small">{st.system ? "otomatis" : st.page ? pageName(st.page) : "tanpa halaman"}</span>
            {st.decision && <span className="state" title="Titik keputusan: halamannya menyediakan setujui dan tolak/kembalikan.">keputusan</span>}
            {st.risky && <span className="state st-review" title="Tidak dapat dibatalkan: perlu konfirmasi.">perlu konfirmasi</span>}
            {(st.group ?? 0) > 0 && <span className="state" title="Berjalan bersamaan dengan langkah lain di kelompok ini.">paralel {st.group}</span>}
            {(st.status?.length ?? 0) > 0 && <span className="muted small">{st.status!.join("; ")}</span>}
            {!st.system && !st.placed && <span className="state st-issue" title="Belum ada section, field, atau aksi untuk langkah ini di halamannya.">belum tergambar</span>}
          </li>
        ))}
      </ol>
      {(flow.branches?.length ?? 0) > 0 && (
        <ul className="mockup-walk-branches">
          {flow.branches!.map((br, i) => (
            <li key={i}>
              <span className={`state ${br.kind === "failure" ? "st-issue" : ""}`}>{br.kind === "failure" ? "gagal" : "alternatif"}</span>
              {br.from ? <span className="muted small">langkah {br.from}</span> : null}
              <button className="link" disabled={!br.page} onClick={() => onOpen(br.page)}>{br.text}</button>
              <span className="muted small">
                {br.system ? "otomatis" : br.placed ? pageName(br.page) : "belum terlihat di layar"}
                {(br.flows?.length ?? 0) > 0 && <> → {br.flows!.map((id) => (
                  <button key={id} className="link" disabled={!flows.some((f) => f.id === id)} onClick={() => onFlow(id)}>{id}</button>
                ))}</>}
              </span>
            </li>
          ))}
        </ul>
      )}
      {(flow.postconditions?.length ?? 0) > 0 && <p className="muted small"><b>Hasil</b> {flow.postconditions!.join("; ")}</p>}
    </div>
  );
}

// SystemGuide is what the whole set of pages is built from and checked
// against: the sources in the blueprint and the design quality gate.
function SystemGuide({ project, gate, bench }: { project: Project; gate: DesignGate | null; bench: DesignBenchmark | null }) {
  const lifecycles = project.lifecycles ?? [];
  return (
    <>
      <h2>Panduan</h2>
      <p className="muted small">
        Mockup dirender dari sistem utama Foundry dengan data contoh: acuan struktur, kepadatan, dan token untuk coding agent,
        bukan desain final piksel. Ubah sumbernya, bukan mockupnya.
      </p>
      <ul className="mw-sources">
        <li><Link to={blueprintDoc(DESIGN_DOCS.layout)}>Spesifikasi layout</Link> <span className="muted small">halaman, bagian, navigasi</span></li>
        <li><Link to={blueprintDoc(DESIGN_DOCS.rules)}>Aturan desain</Link> <span className="muted small">token, komponen, aksesibilitas</span></li>
        <li><Link to={blueprintDoc(SCOPE_DOCS.useCases)}>Use case</Link> <span className="muted small">alur dan langkah tiap peran</span></li>
      </ul>
      {gate && gate.checks.length > 0 && <GatePanel gate={gate} />}
      {bench && bench.dimensions.length > 0 && <BenchmarkPanel bench={bench} />}
      {lifecycles.length > 0 && <LifecyclePanel lifecycles={lifecycles} />}
    </>
  );
}

// PageGuide is what the layout spec, the use cases and the quality gate
// say about one page: the brief to build it from.
function PageGuide({ mockup, spec, flows, gate, list, onOpen, onFlow }: {
  mockup: MockupStatus; spec?: LayoutPage; flows: FlowWalk[]; gate: DesignGate | null; list: MockupStatus[];
  onOpen: (id?: string) => void; onFlow: (id: string) => void;
}) {
  const through = flows.filter((f) => (f.pages ?? []).includes(mockup.page));
  const findings = pageFindings(gate, mockup.page);
  const name = (id: string) => list.find((m) => m.page === id)?.name ?? id;
  const states = Object.entries(spec?.states ?? {}).filter(([, v]) => v);
  return (
    <>
      <h2>Panduan halaman</h2>
      <p className="mw-meta">
        <code>{mockup.page}</code>
        {spec?.route && <code>{spec.route}</code>}
        <span className="state">{mockup.pattern}</span>
        {spec?.role && <span className="state">{spec.role}</span>}
        {spec?.access && <span className="state">{spec.access === "public" ? "publik" : "login"}</span>}
      </p>
      {spec?.purpose && <Item label="Tujuan">{spec.purpose}</Item>}
      {spec?.goal && <Item label="Goal pengguna">{spec.goal}</Item>}
      {spec?.primary_action && <Item label="Aksi utama">{spec.primary_action}</Item>}
      {(spec?.sections?.length ?? 0) > 0 && (
        <Item label={`Bagian (${spec!.sections!.length})`}>
          <ol className="mw-list">
            {spec!.sections!.map((s) => (
              <li key={s.id}><b>{s.name}</b>{s.purpose && <span className="muted small"> {s.purpose}</span>}</li>
            ))}
          </ol>
        </Item>
      )}
      {states.length > 0 && (
        <Item label="State">
          <ul className="mw-list">{states.map(([k, v]) => <li key={k}><b>{STATE_LABEL[k] ?? k}</b> <span className="muted small">{v}</span></li>)}</ul>
        </Item>
      )}
      {(spec?.links?.length ?? 0) > 0 && (
        <Item label="Menuju">
          <ul className="mw-list">
            {spec!.links!.map((l, i) => (
              <li key={i}><button className="link" onClick={() => onOpen(l.to)}>{name(l.to)}</button> <span className="muted small">{l.via}</span></li>
            ))}
          </ul>
        </Item>
      )}
      {through.length > 0 && (
        <Item label="Use case">
          <ul className="mw-list">
            {through.map((f) => (
              <li key={f.id}>
                <button className="link" onClick={() => onFlow(f.id)} title="Telusuri use case ini">{f.id} · {f.name}</button>
                {f.role && <span className="muted small"> {f.role}</span>}
              </li>
            ))}
          </ul>
        </Item>
      )}
      {findings.length > 0 && (
        <Item label="Temuan quality gate">
          <ul className="mw-list">{findings.map((it, i) => <li key={i} className="small">{it}</li>)}</ul>
        </Item>
      )}
      {!spec && <p className="muted small">Halaman ini tidak ada di spesifikasi layout yang termuat.</p>}
      <p className="small"><Link to={blueprintDoc(DESIGN_DOCS.layout)}>Sunting di spesifikasi layout →</Link></p>
    </>
  );
}

function Item({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="mw-item">
      <div className="ws-group-label">{label}</div>
      <div>{children}</div>
    </div>
  );
}
