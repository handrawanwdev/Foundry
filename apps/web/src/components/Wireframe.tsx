import { useEffect, useState, type ReactNode } from "react";
import { cells, formGroups, incoming, navTarget, pageDensity, pageKind, statParts, type Device } from "../design";
import type { FormField, Page, PageComponent, PageKind } from "../types";

export type PageState = "normal" | "loading" | "empty" | "no_results" | "error" | "success";

interface Props {
  page: Page;
  pages: Page[];
  device: Device;
  // select: a click picks an element for the inspect panel.
  // run: the page behaves like the app (links, nav, form submit).
  mode: "select" | "run";
  state?: PageState;
  selected?: number | null;
  onSelect?: (index: number | null) => void;
  onNavigate: (pageId: string) => void;
}

type Item = { c: PageComponent; i: number };

const SUBMIT: Partial<Record<PageKind, string>> = { auth: "Masuk", payment: "Bayar", search: "Cari", wizard: "Lanjut" };

// Interactive wireframe of one designed page. The layout follows the page
// kind, so a login, a data list and a settings page do not look alike.
export function Wireframe({ page, pages, device, mode, state = "normal", selected, onSelect, onNavigate }: Props) {
  const kind = pageKind(page);
  const run = mode === "run";
  const items: Item[] = (page.components ?? []).map((c, i) => ({ c, i }));
  const navs = items.filter((x) => x.c.type === "nav");
  const body = items.filter((x) => x.c.type !== "nav");
  const mobile = device === "mobile";
  const back = kind === "detail" ? incoming(page, pages)[0] : undefined;

  const wrap = (x: Item, node: ReactNode, cls = "") => (
    <div key={x.i} className={`wf-block ${cls} ${mode === "select" ? "pickable" : ""} ${selected === x.i ? "selected" : ""}`}
      onClick={mode === "select" ? (e) => { e.stopPropagation(); onSelect?.(x.i); } : undefined}>
      {node}
    </div>
  );
  const block = (x: Item) => wrap(x,
    <Block c={x.c} kind={kind} run={run} onNavigate={onNavigate}
      empty={state === "empty" ? page.states?.empty : state === "no_results" ? page.states?.no_results : undefined} filtered={state === "no_results"} />,
    x.c.type === "button" ? "shrink" : "");
  const nav = (x: Item) => wrap(x, <Nav c={x.c} page={page} pages={pages} run={run} onNavigate={onNavigate} bottom={mobile} />);

  let content: ReactNode;
  if (state === "loading") {
    const title = body.find((x) => x.c.type === "header");
    content = (
      <>
        {title && block(title)}
        <div className="wf-loading" aria-busy="true">
          <span /><span /><span />
          {page.states?.loading && <p>{page.states.loading}</p>}
        </div>
      </>
    );
  } else if (state === "error" || state === "success") {
    const title = body.find((x) => x.c.type === "header");
    content = (
      <>
        {title && block(title)}
        <StateBox kind={state} text={(state === "error" ? page.states?.error : page.states?.success) ?? ""} />
      </>
    );
  } else if ((kind === "list" || kind === "admin") && !mobile) {
    // Toolbar: title and actions on one row, data below.
    const bar = body.filter((x) => x.c.type === "header" || x.c.type === "button");
    const rest = body.filter((x) => !bar.includes(x));
    content = (
      <>
        {bar.length > 0 && <div className="wf-toolbar">{bar.map(block)}</div>}
        {rows(rest, block)}
      </>
    );
  } else if (kind === "settings" && !mobile && body.some((x) => x.c.type === "section")) {
    const sections = body.filter((x) => x.c.type === "section");
    content = (
      <div className="wf-settings">
        <div className="wf-subnav">{sections.map((x) => <span key={x.i}>{x.c.label}</span>)}</div>
        <div className="wf-settings-body">{rows(body, block)}</div>
      </div>
    );
  } else {
    content = rows(body, block);
  }

  return (
    <div className={`wf-screen ${device} k-${kind} d-${pageDensity(page)}`} onClick={() => mode === "select" && onSelect?.(null)}>
      <div className="wf-bar">{mobile ? <span className="wf-notch" /> : <span>{page.name}</span>}</div>
      {!mobile && navs.length > 0 && <div className="wf-top">{navs.map(nav)}</div>}
      <div className="wf-body" key={page.id + state}>
        <div className="wf-layout">
          {back && (
            <button type="button" className="wf-back" onClick={run ? () => onNavigate(back.id) : undefined} tabIndex={run ? 0 : -1}>← {back.name}</button>
          )}
          {content}
          {body.length === 0 && <p className="wf-text">Halaman ini belum punya isi.</p>}
        </div>
      </div>
      {mobile && navs.length > 0 && <div className="wf-bottom">{navs.map(nav)}</div>}
    </div>
  );
}

// Consecutive stats sit on one row.
function rows(items: Item[], block: (x: Item) => ReactNode): ReactNode[] {
  const out: ReactNode[] = [];
  for (let k = 0; k < items.length; k++) {
    if (items[k].c.type !== "stat") { out.push(block(items[k])); continue; }
    const run: Item[] = [];
    while (k < items.length && items[k].c.type === "stat") run.push(items[k++]);
    k--;
    out.push(<div key={`s${run[0].i}`} className="wf-stats">{run.map(block)}</div>);
  }
  return out;
}

function StateBox({ kind, text }: { kind: "error" | "success"; text: string }) {
  return (
    <div className={`wf-state ${kind}`}>
      <strong>{kind === "error" ? "Gagal memuat" : "Berhasil"}</strong>
      {text && <p>{text}</p>}
      {kind === "error" && <button type="button" className="wf-btn">Coba lagi</button>}
    </div>
  );
}

interface NavProps { c: PageComponent; page: Page; pages: Page[]; run: boolean; bottom: boolean; onNavigate: (id: string) => void }

function Nav({ c, page, pages, run, bottom, onNavigate }: NavProps) {
  const items = c.items?.length ? c.items : [c.label];
  return (
    <nav className={bottom ? "wf-tabbar" : "wf-nav"}>
      {(bottom ? items.slice(0, 5) : items).map((it) => {
        const to = navTarget(it, page, pages);
        return (
          <button key={it} type="button" className={`wf-nav-item ${to?.id === page.id ? "current" : ""}`}
            onClick={run && to ? () => onNavigate(to.id) : undefined} tabIndex={run ? 0 : -1}>
            {bottom && <i />}{it}
          </button>
        );
      })}
    </nav>
  );
}

interface BlockProps { c: PageComponent; kind: PageKind; run: boolean; empty?: string; filtered?: boolean; onNavigate: (id: string) => void }

function Block({ c, kind, run, empty, filtered, onNavigate }: BlockProps) {
  const open = run && c.target ? () => onNavigate(c.target!) : undefined;
  switch (c.type) {
    case "header":
      return <h2 className="wf-title">{c.label}</h2>;
    case "section":
      return <h3 className="wf-section">{c.label}</h3>;
    case "steps":
      return (
        <ol className="wf-steps">
          {(c.items ?? [c.label]).map((it, i) => <li key={it} className={i === 0 ? "current" : ""}><span>{i + 1}</span>{it}</li>)}
        </ol>
      );
    case "button":
      return <button type="button" className={`wf-btn ${c.target ? "primary" : ""}`} onClick={open} tabIndex={run ? 0 : -1}>{c.label}</button>;
    case "input":
      return <Field label={c.label} />;
    case "form":
      return <Form c={c} submit={SUBMIT[kind] ?? "Simpan"} cancel={kind === "form" || kind === "settings" || kind === "profile"} run={run} onSubmit={open} />;
    case "table":
      return (
        <div className="wf-table">
          {c.label && <div className="wf-label">{c.label}</div>}
          {!!c.columns?.length && <div className="wf-tr head">{c.columns.map((col) => <span key={col}>{col}</span>)}</div>}
          {empty !== undefined ? <Empty text={empty} filtered={filtered} /> : (c.items ?? []).map((row, i) => (
            <div key={i} className={`wf-tr ${c.target ? "link" : ""}`} onClick={open}>
              {cells(row).map((cell, j) => <span key={j}>{cell}</span>)}
            </div>
          ))}
        </div>
      );
    case "list":
      return (
        <div className="wf-list">
          {c.label && <div className="wf-label">{c.label}</div>}
          {empty !== undefined ? <Empty text={empty} filtered={filtered} /> : (c.items ?? []).map((it, i) => {
            const [main, ...sub] = cells(it);
            return (
              <div key={i} className={`wf-row ${c.target ? "link" : ""}`} onClick={open}>
                <span>{main}{sub.length > 0 && <small>{sub.join(" · ")}</small>}</span>
                {c.target && <span className="wf-chev">›</span>}
              </div>
            );
          })}
        </div>
      );
    case "stat": {
      const [name, value] = statParts(c.label);
      return <div className="wf-stat"><span>{name}</span><strong>{value ?? "—"}</strong></div>;
    }
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

function Empty({ text, filtered }: { text: string; filtered?: boolean }) {
  return (
    <div className="wf-empty">
      {text || "Belum ada data."}
      {filtered && <div><button type="button" className="wf-btn ghost">Hapus filter</button></div>}
    </div>
  );
}

function Field({ label, spec, error }: { label: string; spec?: FormField; error?: string }) {
  const name = spec?.label ?? label;
  const type = spec?.type ?? "text";
  const input = type === "textarea" ? <textarea name={name} placeholder={name} rows={2} />
    : type === "select" ? <select name={name} defaultValue=""><option value="">Pilih {name}</option></select>
    : type === "checkbox" || type === "radio" ? <input name={name} type={type} />
    : <input name={name} placeholder={name} inputMode={type === "currency" || type === "number" ? "decimal" : undefined}
        type={["email", "password", "number", "tel", "date", "time"].includes(type) ? type : type === "datetime" ? "datetime-local" : "text"} />;
  return (
    <label className={`wf-field ${error ? "invalid" : ""}`}>
      <span>{name}{spec?.required && <em aria-hidden> *</em>}</span>
      {input}
      {error && <small role="alert">{error}</small>}
    </label>
  );
}

// Groups for the form: from the field spec when present, else from items.
function fieldGroups(c: PageComponent): { title?: string; fields: { label: string; spec?: FormField }[] }[] {
  if (!c.fields?.length) return formGroups(c.items ?? []).map((g) => ({ title: g.title, fields: g.fields.map((label) => ({ label })) }));
  const out: { title?: string; fields: { label: string; spec?: FormField }[] }[] = [];
  for (const f of c.fields) {
    const last = out[out.length - 1];
    if (!last || last.title !== (f.group || undefined)) out.push({ title: f.group || undefined, fields: [] });
    out[out.length - 1].fields.push({ label: f.label, spec: f });
  }
  return out;
}

interface FormProps { c: PageComponent; submit: string; cancel: boolean; run: boolean; onSubmit?: () => void }

function Form({ c, submit, cancel, run, onSubmit }: FormProps) {
  const [sent, setSent] = useState(false);
  const [errors, setErrors] = useState<Record<string, string>>({});
  useEffect(() => { if (!run) { setSent(false); setErrors({}); } }, [run]);
  return (
    <form className="wf-form" noValidate onSubmit={(e) => {
      e.preventDefault();
      if (!run) return;
      // Prototype validation: required fields from the spec must be filled.
      const data = new FormData(e.currentTarget);
      const errs: Record<string, string> = {};
      for (const f of c.fields ?? []) {
        if (f.required && !String(data.get(f.label) ?? "").trim()) errs[f.label] = f.error || `${f.label} wajib diisi`;
      }
      setErrors(errs);
      if (Object.keys(errs).length) return;
      if (onSubmit) onSubmit();
      else setSent(true);
    }}>
      {c.label && <div className="wf-label">{c.label}</div>}
      {fieldGroups(c).map((g, i) => (
        <fieldset key={i} className="wf-group">
          {g.title && <legend>{g.title}</legend>}
          {g.fields.map((f) => <Field key={f.label} label={f.label} spec={f.spec} error={errors[f.label]} />)}
        </fieldset>
      ))}
      <div className="wf-form-foot">
        {sent && <span className="wf-sent">✓ Tersimpan</span>}
        {cancel && <button type="button" className="wf-btn ghost" tabIndex={run ? 0 : -1}>Batal</button>}
        <button type="submit" className="wf-btn primary" tabIndex={run ? 0 : -1}>{submit}</button>
      </div>
    </form>
  );
}
