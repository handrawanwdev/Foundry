import { useEffect, useRef, useState } from "react";
import { api } from "../api";
import { useProject } from "../store/project";
import type { BenchKind, DesignBenchmark, DesignGate, DesignView, FlowStep, FlowWalk, GateStatus, Lifecycle, MockupApproval, MockupStatus, Project } from "../types";

// Every designed page rendered as an HTML mockup from the UI kit (token
// preset + page template). The user reviews each page per viewport and
// state, then approves it; the approved mockup is the reference the coding
// agent builds from. Mockups have no script; GET to the same URL simulates
// role and document status and stores nothing. The frame allows same-origin
// loads and form GET so links stay inside it. A use case can be walked
// step by step, pages narrowed to one role, and the quality gate plus
// per-dimension benchmark read before the mockups are approved.

const VIEWPORTS = [
  { id: "desktop", label: "Desktop", width: 1280, height: 800 },
  { id: "tablet", label: "Tablet", width: 820, height: 1000 },
  { id: "mobile", label: "Mobile", width: 390, height: 780 },
] as const;
type Viewport = (typeof VIEWPORTS)[number]["id"];

const APPROVAL: Record<MockupApproval, { label: string; cls: string }> = {
  approved: { label: "Disetujui", cls: "st-final" },
  changed: { label: "Berubah sejak disetujui", cls: "st-issue" },
  pending: { label: "Belum disetujui", cls: "st-review" },
};

const GATE: Record<GateStatus, { label: string; cls: string }> = {
  pass: { label: "lolos", cls: "st-final" },
  warn: { label: "tinjau", cls: "st-review" },
  fail: { label: "gagal", cls: "st-issue" },
  skip: { label: "tidak berlaku", cls: "" },
};

const VERDICT: Record<DesignGate["verdict"], { label: string; cls: string }> = {
  ready: { label: "siap", cls: "st-final" },
  review: { label: "perlu ditinjau", cls: "st-review" },
  blocked: { label: "belum siap", cls: "st-issue" },
};

const STATE_LABEL: Record<string, string> = {
  "": "Terisi", loading: "Loading", empty: "Kosong", no_results: "Tanpa hasil", error: "Error",
  success: "Sukses", disabled: "Nonaktif", permission_denied: "Akses ditolak", interactive: "Interaktif",
};

export function MockupPanel({ project, design }: { project: Project; design: DesignView | null }) {
  const { run, pending } = useProject();
  const [list, setList] = useState<MockupStatus[] | null>(null);
  const [page, setPage] = useState("");
  const [viewport, setViewport] = useState<Viewport>("desktop");
  const [state, setState] = useState("");
  const [full, setFull] = useState(false);
  const [flows, setFlows] = useState<FlowWalk[]>([]);
  const [flowID, setFlowID] = useState("");
  const [step, setStep] = useState(0);
  const [role, setRole] = useState("");
  const [gate, setGate] = useState<DesignGate | null>(null);
  const [bench, setBench] = useState<DesignBenchmark | null>(null);
  const [docStatus, setDocStatus] = useState("");
  const [docEntity, setDocEntity] = useState("");

  useEffect(() => {
    api.mockups().then((r) => {
      setList(r.mockups); setFlows(r.flows ?? []); setGate(r.gate ?? null); setBench(r.benchmark ?? null);
    }).catch(() => setList([]));
  }, [project.version, project.dir]);

  // Role preview: the use cases of one role and the pages they pass
  // through or that name the role.
  const pages = design?.design?.layout?.pages ?? [];
  const roles = [...new Set([...flows.map((f) => f.role ?? ""), ...pages.map((p) => p.role ?? "")].map((r) => r.trim()).filter(Boolean))];
  const roleFlows = role ? flows.filter((f) => f.role === role) : flows;
  const rolePages = new Set([...roleFlows.flatMap((f) => f.pages ?? []), ...pages.filter((p) => p.role === role).map((p) => p.id)]);
  const shown = role ? (list ?? []).filter((m) => rolePages.has(m.page)) : (list ?? []);
  const current = list?.find((m) => m.page === page) ?? shown[0] ?? list?.[0];
  const spec = design?.design?.layout?.pages?.find((p) => p.id === current?.page);
  // A mobile page opens on the mobile viewport.
  useEffect(() => {
    if (spec && (spec.platform === "android" || spec.platform === "ios")) setViewport("mobile");
  }, [spec?.id]);

  if (!list || list.length === 0 || !current) return null;
  const vp = VIEWPORTS.find((v) => v.id === viewport)!;
  const states = ["", ...Object.keys(spec?.states ?? {}).filter((k) => k in STATE_LABEL)];
  const lifecycles = project.lifecycles ?? [];
  const lc = lifecycles.find((l) => l.entity === docEntity) ?? lifecycles[0];
  const mockupSrc = api.mockupURL(current.page, state, { role, status: docStatus, entity: docEntity || lc?.entity });
  const src = `${mockupSrc}${mockupSrc.includes("?") ? "&" : "?"}v=${project.version}`;
  const approval = APPROVAL[current.status];
  const pendingAll = list.filter((m) => m.status !== "approved").length;
  const rules = design?.design?.rules;
  const busy = pending === "approveMockups";
  const flow = flows.find((f) => f.id === flowID);
  const walkable = (flow?.steps ?? []).filter((st) => !st.system && st.page);
  const at = walkable.findIndex((st) => st.n === step);
  const go = (st?: FlowStep) => {
    if (!st?.page) return;
    setPage(st.page);
    setState("");
    setStep(st.n);
  };
  const pickFlow = (id: string) => {
    setFlowID(id);
    const f = flows.find((x) => x.id === id);
    go(f?.steps.find((st) => !st.system && st.page));
  };
  const pageName = (id?: string) => list.find((m) => m.page === id)?.name ?? id ?? "";
  const open = (id?: string) => { if (id) { setPage(id); setState(""); } };
  const pickRole = (r: string) => {
    setRole(r);
    if (flow && r && flow.role !== r) setFlowID("");
  };
  const approve = (pages: string[]) =>
    void run("approveMockups", async () => {
      const r = await api.approveMockups(pages);
      setList(r.mockups);
    });

  return (
    <section className="mockup">
      <header className="mockup-head">
        <div>
          <h2>Mockup halaman</h2>
          <p className="muted small">
            Dirender dari {rules?.preset ? <>preset <code>{rules.preset}</code></> : "token proyek"} dan template pattern UI kit, dengan data contoh.
            Ini acuan struktur, kepadatan, dan token untuk coding agent — bukan desain final piksel.
          </p>
        </div>
        <span className="muted small">{list.length - pendingAll}/{list.length} disetujui</span>
      </header>
      {gate && gate.checks.length > 0 && <GatePanel gate={gate} />}
      {bench && bench.dimensions.length > 0 && <BenchmarkPanel bench={bench} />}
      <div className="mockup-bar">
        {roles.length > 1 && (
          <select value={role} onChange={(e) => pickRole(e.target.value)} aria-label="Peran">
            <option value="">Semua peran</option>
            {roles.map((r) => <option key={r} value={r}>{r}</option>)}
          </select>
        )}
        {roleFlows.length > 0 && (
          <select value={flowID} onChange={(e) => pickFlow(e.target.value)} aria-label="Use case">
            <option value="">Semua halaman</option>
            {roleFlows.map((f) => <option key={f.id} value={f.id}>{f.id} · {f.name}</option>)}
          </select>
        )}
        <select value={current.page} onChange={(e) => { setPage(e.target.value); setState(""); }} aria-label="Halaman">
          {(shown.some((m) => m.page === current.page) ? shown : [current, ...shown]).map((m) => (
            <option key={m.page} value={m.page}>{m.name} · {m.pattern}{m.status === "approved" ? " ✓" : ""}</option>
          ))}
        </select>
        <div className="seg" role="group" aria-label="Viewport">
          {VIEWPORTS.map((v) => (
            <button key={v.id} className={viewport === v.id ? "on" : ""} aria-pressed={viewport === v.id} onClick={() => setViewport(v.id)}>{v.label}</button>
          ))}
        </div>
        {states.length > 1 && (
          <select value={state} onChange={(e) => setState(e.target.value)} aria-label="State">
            {states.map((s) => <option key={s} value={s}>{STATE_LABEL[s]}</option>)}
          </select>
        )}
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
        <button className="ghost" onClick={() => setFull(true)}>Perbesar</button>
        <a className="button ghost" href={src} target="_blank" rel="noreferrer">Tab baru</a>
      </div>
      {flow && (
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
              <li key={st.n} className={[st.system ? "sys" : "", st.n === step ? "on" : ""].join(" ").trim() || undefined}>
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
                  <button className="link" disabled={!br.page} onClick={() => open(br.page)}>{br.text}</button>
                  <span className="muted small">
                    {br.system ? "otomatis" : br.placed ? pageName(br.page) : "belum terlihat di layar"}
                    {(br.flows?.length ?? 0) > 0 && <> → {br.flows!.map((id) => (
                      <button key={id} className="link" disabled={!flows.some((f) => f.id === id)} onClick={() => pickFlow(id)}>{id}</button>
                    ))}</>}
                  </span>
                </li>
              ))}
            </ul>
          )}
          {(flow.postconditions?.length ?? 0) > 0 && <p className="muted small"><b>Hasil</b> {flow.postconditions!.join("; ")}</p>}
        </div>
      )}
      <Frame key={`${current.page}-${state}-${viewport}`} src={src} width={vp.width} height={vp.height} title={`Mockup ${current.name}`} />
      <footer className="mockup-foot">
        <span className={`state ${approval.cls}`}>{approval.label}</span>
        {!current.template && <span className="muted small">Pattern {current.pattern} belum punya template; mockup generik.</span>}
        <span className="mockup-actions">
          <button disabled={busy || current.status === "approved"} onClick={() => approve([current.page])}>Setujui halaman ini</button>
          {pendingAll > 1 && <button className="primary" disabled={busy} onClick={() => approve([])}>Setujui semua ({pendingAll})</button>}
        </span>
      </footer>
      {full && <FullView src={src} title={`Mockup ${current.name}`} width={vp.width} onClose={() => setFull(false)} />}
      {lifecycles.length > 0 && <LifecyclePanel lifecycles={lifecycles} />}
    </section>
  );
}

const BENCH_KIND: Record<BenchKind, string> = {
  structural: "struktural", semantic: "semantik", visual: "visual", interaction: "interaksi", business: "bisnis",
};

function BenchmarkPanel({ bench }: { bench: DesignBenchmark }) {
  return (
    <details className="mockup-gate" open={!bench.ready}>
      <summary>
        <strong>Benchmark per dimensi</strong>
        <span className={`state ${bench.ready ? "st-final" : "st-issue"}`}>{bench.ready ? "tidak memblokir" : "memblokir"}</span>
        <span className="muted small">{bench.summary}</span>
      </summary>
      <ul>
        {bench.dimensions.map((d) => (
          <li key={d.id}>
            <span className={`state ${GATE[d.status].cls}`}>{GATE[d.status].label}</span>
            <span>{d.name}</span>
            <span className="muted small">{BENCH_KIND[d.kind]}{d.blocking ? " · memblokir bila gagal" : ""}{d.total > 0 ? ` · ${d.covered}/${d.total}` : ""} · {d.detail}</span>
          </li>
        ))}
      </ul>
      {(bench.blocking?.length ?? 0) > 0 && (
        <div className="mockup-gate-items">
          <b>Memblokir</b>
          <ul>{bench.blocking!.map((it) => <li key={it} className="small">{it}</li>)}</ul>
        </div>
      )}
    </details>
  );
}

function LifecyclePanel({ lifecycles }: { lifecycles: Lifecycle[] }) {
  return (
    <details className="mockup-gate">
      <summary>
        <strong>Siklus status dokumen</strong>
        <span className="muted small">{lifecycles.length} dokumen</span>
      </summary>
      {lifecycles.map((lc) => (
        <div key={lc.entity} className="mockup-gate-items">
          <b>{lc.entity}</b>
          <ul>
            {lc.states.map((s) => (
              <li key={s.id} className="small">{s.label || s.id}{s.kind ? ` · ${s.kind === "initial" ? "awal" : "akhir"}` : ""}{s.locked ? " · terkunci" : ""}</li>
            ))}
          </ul>
          <ul>
            {lc.transitions.map((t, i) => (
              <li key={i} className="small">{t.from.join("/")} → {t.to}: {t.action} ({t.actor}{t.flow ? ` · ${t.flow}${t.step ? ` langkah ${t.step}` : ""}` : ""})</li>
            ))}
          </ul>
        </div>
      ))}
    </details>
  );
}

// GatePanel is the design quality gate: each check with its status and,
// opened, what to fix. A design is not complete because every page exists.
function GatePanel({ gate }: { gate: DesignGate }) {
  const open = gate.checks.filter((c) => c.status === "fail" || c.status === "warn");
  return (
    <details className="mockup-gate" open={gate.verdict === "blocked"}>
      <summary>
        <strong>Quality gate</strong>
        <span className={`state ${VERDICT[gate.verdict].cls}`}>{VERDICT[gate.verdict].label}</span>
        <span className="muted small">{gate.summary}</span>
      </summary>
      <ul>
        {gate.checks.map((c) => (
          <li key={c.id}>
            <span className={`state ${GATE[c.status].cls}`}>{GATE[c.status].label}</span>
            <span>{c.name}</span>
            <span className="muted small">{c.detail}</span>
          </li>
        ))}
      </ul>
      {open.map((c) => (c.items?.length ?? 0) > 0 && (
        <div key={c.id} className="mockup-gate-items">
          <b>{c.name}</b>
          <ul>{c.items!.map((it, i) => <li key={i} className="small">{it}</li>)}</ul>
        </div>
      ))}
      {(gate.assumptions?.length ?? 0) + (gate.unknowns?.length ?? 0) > 0 && (
        <div className="mockup-gate-items">
          <b>Perlu dikonfirmasi</b>
          <ul>{[...(gate.assumptions ?? []), ...(gate.unknowns ?? [])].map((it, i) => <li key={i} className="small">{it}</li>)}</ul>
        </div>
      )}
    </details>
  );
}

// Frame shows the mockup at its real viewport width, scaled to fit.
function Frame({ src, width, height, title }: { src: string; width: number; height: number; title: string }) {
  const box = useRef<HTMLDivElement>(null);
  const [avail, setAvail] = useState(width);
  useEffect(() => {
    const el = box.current;
    if (!el) return;
    const ro = new ResizeObserver(([e]) => setAvail(e.contentRect.width));
    ro.observe(el);
    return () => ro.disconnect();
  }, []);
  const scale = Math.min(1, avail / width);
  return (
    <div className="mockup-stage" ref={box} style={{ height: height * scale }}>
      <iframe
        title={title}
        src={src}
        sandbox="allow-same-origin allow-forms"
        style={{ width, height, transform: `scale(${scale})`, left: Math.max(0, (avail - width * scale) / 2) }}
      />
    </div>
  );
}

function FullView({ src, title, width, onClose }: { src: string; title: string; width: number; onClose: () => void }) {
  useEffect(() => {
    const key = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    window.addEventListener("keydown", key);
    return () => window.removeEventListener("keydown", key);
  }, [onClose]);
  return (
    <div className="mockup-full" role="dialog" aria-modal="true" aria-label={title}>
      <div className="mockup-full-bar">
        <span>{title}</span>
        <button onClick={onClose} autoFocus>Tutup (Esc)</button>
      </div>
      <iframe title={title} src={src} sandbox="allow-same-origin allow-forms" style={{ width: Math.min(width, window.innerWidth - 32) }} />
    </div>
  );
}
