import { useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { api } from "../api";
import type { BenchKind, DesignBenchmark, DesignGate, GateStatus, Lifecycle, MockupApproval, MockupStatus, Project } from "../types";

// Every designed page rendered as an HTML mockup from the UI kit (token
// preset + page template). The pages are reviewed in the mockup workspace
// (pages/Mockups.tsx); these are the parts it shares with the layout spec.
// Mockups have no script; GET to the same URL simulates role and document
// status and stores nothing. The frame allows same-origin loads and form
// GET so links stay inside it.

export const VIEWPORTS = [
  { id: "desktop", label: "Desktop", width: 1280, height: 800 },
  { id: "tablet", label: "Tablet", width: 820, height: 1000 },
  { id: "mobile", label: "Mobile", width: 390, height: 780 },
] as const;
export type Viewport = (typeof VIEWPORTS)[number]["id"];

export const APPROVAL: Record<MockupApproval, { label: string; cls: string }> = {
  approved: { label: "Disetujui", cls: "st-final" },
  changed: { label: "Berubah sejak disetujui", cls: "st-issue" },
  pending: { label: "Belum disetujui", cls: "st-review" },
};

export const GATE: Record<GateStatus, { label: string; cls: string }> = {
  pass: { label: "lolos", cls: "st-final" },
  warn: { label: "tinjau", cls: "st-review" },
  fail: { label: "gagal", cls: "st-issue" },
  skip: { label: "tidak berlaku", cls: "" },
};

export const VERDICT: Record<DesignGate["verdict"], { label: string; cls: string }> = {
  ready: { label: "siap", cls: "st-final" },
  review: { label: "perlu ditinjau", cls: "st-review" },
  blocked: { label: "belum siap", cls: "st-issue" },
};

export const STATE_LABEL: Record<string, string> = {
  "": "Terisi", loading: "Loading", empty: "Kosong", no_results: "Tanpa hasil", error: "Error",
  success: "Sukses", disabled: "Nonaktif", permission_denied: "Akses ditolak", interactive: "Interaktif",
};

// MockupSummary is the mockups' place in the layout spec: how many pages
// are approved, the gate verdict, and the way into the mockup workspace.
export function MockupSummary({ project }: { project: Project }) {
  const [list, setList] = useState<MockupStatus[] | null>(null);
  const [gate, setGate] = useState<DesignGate | null>(null);
  useEffect(() => {
    api.mockups().then((r) => { setList(r.mockups); setGate(r.gate ?? null); }).catch(() => setList([]));
  }, [project.version, project.dir]);
  if (!list || list.length === 0) return null;
  const approved = list.filter((m) => m.status === "approved").length;
  return (
    <section className="mockup-card">
      <div>
        <h2>Mockup halaman</h2>
        <p className="muted small">
          {list.length} halaman dirender dari spesifikasi ini · {approved}/{list.length} disetujui
          {gate && <> · quality gate <span className={`state ${VERDICT[gate.verdict].cls}`}>{VERDICT[gate.verdict].label}</span></>}
        </p>
      </div>
      <Link className="button primary" to="/mockups">Buka workspace mockup</Link>
    </section>
  );
}

const BENCH_KIND: Record<BenchKind, string> = {
  structural: "struktural", semantic: "semantik", visual: "visual", interaction: "interaksi", business: "bisnis",
};

export function BenchmarkPanel({ bench }: { bench: DesignBenchmark }) {
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

export function LifecyclePanel({ lifecycles }: { lifecycles: Lifecycle[] }) {
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
export function GatePanel({ gate }: { gate: DesignGate }) {
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
// Frame shows a mockup at its viewport, scaled to fit the width available,
// or at zoom when one is set (wider than the space: it scrolls).
export function Frame({ src, width, height, title, zoom }: { src: string; width: number; height: number; title: string; zoom?: number }) {
  const box = useRef<HTMLDivElement>(null);
  const [avail, setAvail] = useState(width);
  useEffect(() => {
    const el = box.current;
    if (!el) return;
    const ro = new ResizeObserver(([e]) => setAvail(e.contentRect.width));
    ro.observe(el);
    return () => ro.disconnect();
  }, []);
  const scale = zoom ?? Math.min(1, avail / width);
  return (
    <div className="mockup-stage" ref={box} style={{ height: height * scale, minWidth: zoom ? width * scale : undefined }}>
      <iframe
        title={title}
        src={src}
        sandbox="allow-same-origin allow-forms"
        style={{ width, height, transform: `scale(${scale})`, left: Math.max(0, (avail - width * scale) / 2) }}
      />
    </div>
  );
}

export function FullView({ src, title, width, onClose }: { src: string; title: string; width: number; onClose: () => void }) {
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
