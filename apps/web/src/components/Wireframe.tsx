import { useEffect, useState } from "react";
import { navTarget, type Device } from "../design";
import type { Page, PageComponent } from "../types";

interface Props {
  page: Page;
  pages: Page[];
  device: Device;
  // select: a click picks an element for the inspect panel.
  // run: the page behaves like the app (links, nav, form submit).
  mode: "select" | "run";
  selected?: number | null;
  onSelect?: (index: number | null) => void;
  onNavigate: (pageId: string) => void;
}

// Interactive wireframe of one designed page.
export function Wireframe({ page, pages, device, mode, selected, onSelect, onNavigate }: Props) {
  const comps = page.components ?? [];
  return (
    <div className={`wf-screen ${device}`} onClick={() => mode === "select" && onSelect?.(null)}>
      <div className="wf-bar">{device === "mobile" ? <span className="wf-notch" /> : <span>{page.name}</span>}</div>
      <div className="wf-body" key={page.id}>
        {comps.map((c, i) => (
          <div key={i} className={`wf-block ${mode === "select" ? "pickable" : ""} ${selected === i ? "selected" : ""}`}
            onClick={mode === "select" ? (e) => { e.stopPropagation(); onSelect?.(i); } : undefined}>
            <Block c={c} page={page} pages={pages} run={mode === "run"} onNavigate={onNavigate} />
          </div>
        ))}
        {comps.length === 0 && <p className="wf-text">Halaman ini belum punya isi.</p>}
      </div>
    </div>
  );
}

interface BlockProps { c: PageComponent; page: Page; pages: Page[]; run: boolean; onNavigate: (id: string) => void }

function Block({ c, page, pages, run, onNavigate }: BlockProps) {
  const open = run && c.target ? () => onNavigate(c.target!) : undefined;
  switch (c.type) {
    case "header":
      return <h2 className="wf-title">{c.label}</h2>;
    case "nav":
      return (
        <nav className="wf-nav">
          {(c.items?.length ? c.items : [c.label]).map((it) => {
            const to = navTarget(it, page, pages);
            return (
              <button key={it} type="button" className={`wf-nav-item ${to?.id === page.id ? "current" : ""}`}
                onClick={run && to ? () => onNavigate(to.id) : undefined} tabIndex={run ? 0 : -1}>{it}</button>
            );
          })}
        </nav>
      );
    case "button":
      return <button type="button" className={`wf-btn ${c.target ? "primary" : ""}`} onClick={open} tabIndex={run ? 0 : -1}>{c.label}</button>;
    case "input":
      return <Field label={c.label} />;
    case "form":
      return <Form c={c} run={run} onSubmit={open} />;
    case "list":
    case "table":
      return (
        <div className={`wf-${c.type}`}>
          <div className="wf-label">{c.label}</div>
          {(c.items?.length ? c.items : ["…"]).map((it) => (
            <div key={it} className={`wf-row ${c.target ? "link" : ""}`} onClick={open}>
              <span>{it}</span>
              {c.type === "table" ? <span className="wf-cells"><i /><i /></span> : c.target && <span className="wf-chev">›</span>}
            </div>
          ))}
        </div>
      );
    case "stat":
      return <div className="wf-stat"><span>{c.label}</span><strong>123</strong></div>;
    case "card":
      return (
        <div className={`wf-card ${c.target ? "link" : ""}`} onClick={open}>
          <strong>{c.label}</strong>
          {(c.items ?? []).map((it) => <span key={it}>{it}</span>)}
        </div>
      );
    default:
      return <p className="wf-text">{c.label}</p>;
  }
}

function Field({ label }: { label: string }) {
  return <label className="wf-field"><span>{label}</span><input placeholder={label} /></label>;
}

function Form({ c, run, onSubmit }: { c: PageComponent; run: boolean; onSubmit?: () => void }) {
  const [sent, setSent] = useState(false);
  useEffect(() => { if (!run) setSent(false); }, [run]);
  return (
    <form className="wf-form" onSubmit={(e) => {
      e.preventDefault();
      if (!run) return;
      if (onSubmit) onSubmit();
      else setSent(true);
    }}>
      <div className="wf-label">{c.label}</div>
      {(c.items ?? []).map((it) => <Field key={it} label={it} />)}
      <div className="wf-form-foot">
        <button type="submit" className="wf-btn primary" tabIndex={run ? 0 : -1}>Kirim</button>
        {sent && <span className="wf-sent">✓ Terkirim</span>}
      </div>
    </form>
  );
}
