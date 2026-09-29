import type { Page, PageComponent } from "../types";

interface Props {
  page: Page;
  onNavigate: (pageId: string) => void;
}

// Simple, clickable wireframe of a designed page (prototype mode).
export function Wireframe({ page, onNavigate }: Props) {
  const mobile = page.platform === "android" || page.platform === "ios";
  return (
    <div className={mobile ? "frame phone" : "frame browser"}>
      <div className="frame-bar">{mobile ? "●  ▬  ●" : `● ● ●   ${page.name}`}</div>
      <div className="frame-body">
        {(page.components ?? []).map((c, i) => (
          <Block key={i} c={c} onNavigate={onNavigate} />
        ))}
        {(page.components ?? []).length === 0 && <p className="muted small">Belum ada komponen</p>}
      </div>
    </div>
  );
}

function Block({ c, onNavigate }: { c: PageComponent; onNavigate: (id: string) => void }) {
  const link = c.target ? () => onNavigate(c.target!) : undefined;
  switch (c.type) {
    case "header":
      return <h3 className="wf-header">{c.label}</h3>;
    case "nav":
      return <div className="wf-nav">{(c.items ?? [c.label]).map((it) => <span key={it}>{it}</span>)}</div>;
    case "button":
      return <button className={`wf-button ${link ? "linked" : ""}`} onClick={link} title={link ? "Klik untuk navigasi" : undefined}>{c.label}</button>;
    case "input":
      return <label className="wf-input">{c.label}<span /></label>;
    case "form":
      return (
        <div className="wf-form">
          <strong>{c.label}</strong>
          {(c.items ?? []).map((it) => <label key={it} className="wf-input">{it}<span /></label>)}
        </div>
      );
    case "list":
      return (
        <div className="wf-list">
          <strong>{c.label}</strong>
          {(c.items ?? ["…"]).map((it) => <div key={it} className="wf-row">{it}</div>)}
        </div>
      );
    case "table":
      return (
        <div className="wf-table">
          <strong>{c.label}</strong>
          {(c.items ?? ["…"]).map((it) => <div key={it} className="wf-row">{it}<span>▭ ▭</span></div>)}
        </div>
      );
    case "stat":
      return <div className="wf-stat"><span className="small muted">{c.label}</span><strong>123</strong></div>;
    case "card":
      return (
        <div className={`wf-card ${link ? "linked" : ""}`} onClick={link}>
          <strong>{c.label}</strong>
          {(c.items ?? []).map((it) => <div key={it} className="small muted">{it}</div>)}
        </div>
      );
    default:
      return <p className="wf-text">{c.label}</p>;
  }
}
