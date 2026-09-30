import { useEffect, useState, type CSSProperties, type ReactNode } from "react";
import { cells, formGroups, formSubmit, navTarget, pageDensity, pageKind, statParts, type Device } from "../design";
import { leaves, nodeStyle, resolveLayout, tokenVars, validateIR } from "../ir";
import type { DesignSystem, FormField, LayoutChild, Page, PageComponent } from "../types";

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
  app?: string; // product name shown in the app shell
  system?: DesignSystem; // resolves the semantic tokens
}

type Item = { c: PageComponent; i: number };

// Deterministic renderer of one page's Design IR. It draws the frame, the
// component tree and the device changes exactly as page.layout says; it
// never picks a layout, groups components or adds UI of its own. An invalid
// IR is not drawn: the errors are shown instead.
export function Wireframe({ page, pages, device, mode, state = "normal", selected, onSelect, onNavigate, app = "Aplikasi", system }: Props) {
  const errors = validateIR(page, new Set(pages.map((p) => p.id)));
  const mobile = device === "mobile";
  const bar = (
    <div className="wf-bar">
      {mobile ? <><span className="wf-time">9:41</span><span className="wf-notch" /><span className="wf-sig" /></>
        : <><span className="wf-dots"><i /><i /><i /></span><span className="wf-url">{host(app)}{page.route ?? ""}</span></>}
    </div>
  );
  if (errors.length > 0 || !page.layout) {
    return (
      <div className={`wf-screen ${device}`}>
        {bar}
        <div className="wf-invalid" role="alert">
          <strong>DESIGN_IR_INVALID</strong>
          <p>Halaman ini tidak dirender karena Design IR-nya tidak valid:</p>
          <ul>{errors.map((e) => <li key={e}>{e}</li>)}</ul>
        </div>
      </div>
    );
  }

  const r = resolveLayout(page.layout, device);
  const run = mode === "run";
  const kind = pageKind(page);
  const comps = page.components ?? [];
  const byId = new Map<string, Item>(comps.map((c, i) => [c.id ?? "", { c, i }]));
  const navs: Item[] = comps.map((c, i) => ({ c, i })).filter((x) => x.c.type === "nav");
  const back = r.back ? pages.find((p) => p.id === r.back) : undefined;

  const wrap = (x: Item, node: ReactNode, cls = "") => (
    <div key={x.i} data-c={x.i} data-id={x.c.id} data-type={x.c.type} className={`wf-block ${cls} ${mode === "select" ? "pickable" : ""} ${selected === x.i ? "selected" : ""}`}
      onClick={mode === "select" ? (e) => { e.stopPropagation(); onSelect?.(x.i); } : undefined}>
      {node}
    </div>
  );
  const block = (x: Item) => {
    const c = r.swap[x.c.id ?? ""] ? { ...x.c, type: r.swap[x.c.id ?? ""] } : x.c;
    return wrap(x,
      <Block c={c} cancel={kind === "form" || kind === "settings" || kind === "profile"} submit={formSubmit(page)} run={run} onNavigate={onNavigate}
        empty={state === "empty" ? page.states?.empty : state === "no_results" ? page.states?.no_results : undefined} filtered={state === "no_results"} />,
      c.type === "button" ? "shrink" : "");
  };
  const nav = (x: Item, variant: NavVariant) => wrap(x, <Nav c={x.c} page={page} pages={pages} run={run} onNavigate={onNavigate} variant={variant} />);

  // One layout node or component leaf, exactly as the tree says.
  const draw = (n: LayoutChild, k: number | string): ReactNode => {
    if (typeof n === "string") {
      const x = byId.get(n);
      return x ? block(x) : null;
    }
    const cls = `wf-n wf-n-${n.type}${n.surface ? " wf-" + n.surface.replace(".", "-") : ""}${n.elevation ? " wf-" + n.elevation.replace(".", "-") : ""}${n.placement ? " " + n.placement : ""}`;
    const el = (
      <div key={k} className={cls} data-node={n.id ?? ""} data-node-type={n.type} style={nodeStyle(n) as CSSProperties}>
        {(n.children ?? []).map((c, i) => draw(c, typeof c === "string" ? c : c.id ?? i))}
      </div>
    );
    return n.type === "overlay" ? <div key={k} className="wf-backdrop">{el}</div> : el;
  };

  let content: ReactNode;
  if (state === "loading" || state === "error" || state === "success") {
    // A state replaces the content; the page header stays for orientation.
    const titles = leaves(r.root).map((id) => byId.get(id)).filter((x): x is Item => !!x && x.c.type === "header");
    content = (
      <div className="wf-n wf-n-stack">
        {titles.map(block)}
        {state === "loading" ? (
          <div className="wf-loading" aria-busy="true">
            <span /><span /><span />
            {page.states?.loading && <p>{page.states.loading}</p>}
          </div>
        ) : <StateBox kind={state} text={(state === "error" ? page.states?.error : page.states?.success) ?? ""} />}
      </div>
    );
  } else {
    content = draw(r.root, "root");
  }

  const layout = (
    <div className={`wf-layout w-${r.width}`}>
      {back && (
        <button type="button" className="wf-back" onClick={run ? () => onNavigate(back.id) : undefined} tabIndex={run ? 0 : -1}>← {back.name}</button>
      )}
      {content}
      {comps.every((c) => c.type === "nav") && <p className="wf-text">Halaman ini belum punya isi.</p>}
    </div>
  );
  const brand = <div className="wf-brand"><span>{app.trim().charAt(0).toUpperCase() || "A"}</span>{app}</div>;
  const body = <div className="wf-body" key={page.id + state}>{layout}</div>;

  let frame: ReactNode;
  if (r.frame === "center") {
    frame = (
      <div className="wf-body wf-center" key={page.id + state}>
        <div className={`wf-centerpanel w-${r.width}`}>{r.brand && brand}{layout}</div>
      </div>
    );
  } else if (r.frame === "full") {
    frame = <div className="wf-body wf-full" key={page.id + state}>{layout}</div>;
  } else {
    const side = r.nav === "side" || r.nav === "rail";
    const top = !mobile && (
      <header className="wf-header">
        {side ? <span className="wf-crumb">{page.name}</span> : brand}
        {r.nav === "top" && <div className="wf-topnav">{navs.map((x) => nav(x, "top"))}</div>}
        <span className="wf-avatar" title={page.role}>{page.role.trim().charAt(0).toUpperCase() || "U"}</span>
      </header>
    );
    frame = (
      <div className="wf-app">
        {side && <aside className={`wf-sidenav ${r.nav}`}>{r.nav === "side" && brand}{navs.map((x) => nav(x, r.nav as NavVariant))}</aside>}
        <div className="wf-main">
          {top}
          {mobile && r.nav === "top" && <div className="wf-topnav">{navs.map((x) => nav(x, "top"))}</div>}
          {body}
          {r.nav === "bottom" && <div className="wf-bottom">{navs.map((x) => nav(x, "tabs"))}</div>}
        </div>
      </div>
    );
  }

  return (
    <div className={`wf-screen ${device} d-${pageDensity(page)}`} style={tokenVars(system) as CSSProperties} onClick={() => mode === "select" && onSelect?.(null)}>
      {bar}
      {frame}
    </div>
  );
}

function host(app: string): string {
  return (app.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "") || "app") + ".app";
}

function StateBox({ kind, text }: { kind: "error" | "success"; text: string }) {
  return (
    <div className={`wf-state ${kind}`}>
      <span className="wf-state-icon">{kind === "error" ? "!" : "✓"}</span>
      <strong>{kind === "error" ? "Gagal memuat" : "Berhasil"}</strong>
      {text && <p>{text}</p>}
      {kind === "error" && <button type="button" className="wf-btn">Coba lagi</button>}
    </div>
  );
}

type NavVariant = "side" | "rail" | "top" | "tabs";
interface NavProps { c: PageComponent; page: Page; pages: Page[]; run: boolean; variant: NavVariant; onNavigate: (id: string) => void }

function Nav({ c, page, pages, run, variant, onNavigate }: NavProps) {
  const items = c.items?.length ? c.items : [c.label];
  return (
    <nav className={`wf-nav ${variant}`}>
      {items.map((it) => {
        const to = navTarget(it, page, pages);
        return (
          <button key={it} type="button" className={`wf-nav-item ${to?.id === page.id ? "current" : ""}`} title={it}
            onClick={run && to ? () => onNavigate(to.id) : undefined} tabIndex={run ? 0 : -1}>
            <Icon name={it} /><span className="wf-nav-label">{it}</span>
          </button>
        );
      })}
    </nav>
  );
}

// Small line icons picked from the menu label.
const ICONS: [RegExp, string][] = [
  [/^kosong$/, "M3 13l3-8h12l3 8v6H3zM3 13h5l1 2h6l1-2h5"],
  [/beranda|home|dashboard|ringkas|utama/i, "M3 11l9-7 9 7v9a1 1 0 0 1-1 1h-5v-6H9v6H4a1 1 0 0 1-1-1z"],
  [/janji|jadwal|kalender|booking|reserv|agenda/i, "M4 6h16v14H4zM4 10h16M8 3v4M16 3v4"],
  [/bayar|tagih|invoice|keuangan|transaksi|dompet|saldo/i, "M3 6h18v12H3zM3 10h18M7 15h4"],
  [/laporan|analit|statistik|report|grafik/i, "M4 20V10M10 20V4M16 20v-8M22 20H2"],
  [/pengaturan|setting|konfigurasi/i, "M12 9a3 3 0 1 0 0 6 3 3 0 0 0 0-6zM12 2v3M12 19v3M2 12h3M19 12h3M5 5l2 2M17 17l2 2M5 19l2-2M17 7l2-2"],
  [/cari|search|jelajah/i, "M11 4a7 7 0 1 0 0 14 7 7 0 0 0 0-14zM21 21l-5-5"],
  [/pesan|chat|notif|inbox/i, "M4 5h16v11H8l-4 4z"],
  [/profil|akun|pasien|pengguna|user|dokter|staf|anggota|pelanggan/i, "M12 12a4 4 0 1 0 0-8 4 4 0 0 0 0 8zM4 21a8 8 0 0 1 16 0"],
];

function Icon({ name }: { name: string }) {
  const d = ICONS.find(([re]) => re.test(name))?.[1] ?? "M4 4h7v7H4zM13 4h7v7h-7zM4 13h7v7H4zM13 13h7v7h-7z";
  return (
    <svg className="wf-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden>
      <path d={d} />
    </svg>
  );
}

// Status-like cells become colored badges.
const TONES: [RegExp, string][] = [
  [/batal|tolak|gagal|ditolak|expired|kedaluwarsa|tidak hadir|nonaktif|telat|overdue/i, "bad"],
  [/menunggu|pending|proses|diproses|draf|draft|belum|antri|dijadwalkan/i, "warn"],
  [/selesai|dikonfirmasi|konfirmasi|lunas|aktif|berhasil|hadir|disetujui|terkirim|sukses|dibayar/i, "ok"],
];

function Cell({ text, column }: { text: string; column?: string }) {
  const tone = TONES.find(([re]) => re.test(text))?.[1];
  if (tone && (!column || /status|keadaan|kondisi|state/i.test(column) || text.split(" ").length <= 2)) {
    return <span><b className={`wf-badge ${tone}`}>{text}</b></span>;
  }
  return <span>{text}</span>;
}

interface BlockProps { c: PageComponent; submit: string; cancel: boolean; run: boolean; empty?: string; filtered?: boolean; onNavigate: (id: string) => void }

// Emphasis and tone tokens as classes; nothing is styled from the label.
const emphasisClass = (e?: string) => ({ "action.primary": "primary", "action.ghost": "ghost", "action.danger": "danger" } as Record<string, string>)[e ?? ""] ?? "";
const toneClass = (t?: string) => (t ? " tone-" + t.replace(".", "-") : "");

function Block({ c, submit, cancel, run, empty, filtered, onNavigate }: BlockProps) {
  const open = run && c.target ? () => onNavigate(c.target!) : undefined;
  switch (c.type) {
    case "header":
      return <h2 className="wf-title">{c.label}</h2>;
    case "section":
      return <h3 className={"wf-section" + toneClass(c.tone)}>{c.label}</h3>;
    case "steps":
      return (
        <ol className="wf-steps">
          {(c.items ?? [c.label]).map((it, i) => <li key={it} className={i === 0 ? "current" : ""}><span>{i + 1}</span>{it}</li>)}
        </ol>
      );
    case "button":
      return <button type="button" className={`wf-btn ${emphasisClass(c.emphasis)}`} onClick={open} tabIndex={run ? 0 : -1}>{c.label}</button>;
    case "input":
      return /^(cari|search)/i.test(c.label)
        ? <label className="wf-search"><Icon name="cari" /><input placeholder={c.label + "…"} tabIndex={run ? 0 : -1} /></label>
        : <Field label={c.label} />;
    case "form":
      return <Form c={c} submit={submit} cancel={cancel} run={run} onSubmit={open} />;
    case "table":
      return (
        <div className="wf-table">
          {c.label && <div className="wf-label">{c.label}</div>}
          <div className="wf-panel">
            {!!c.columns?.length && <div className="wf-tr head">{c.columns.map((col) => <span key={col}>{col}</span>)}</div>}
            {empty !== undefined ? <Empty text={empty} filtered={filtered} /> : (c.items ?? []).map((row, i) => (
              <div key={i} className={`wf-tr ${c.target ? "link" : ""}`} onClick={open}>
                {cells(row).map((cell, j) => <Cell key={j} text={cell} column={c.columns?.[j]} />)}
              </div>
            ))}
          </div>
        </div>
      );
    case "list":
      return (
        <div className="wf-list">
          {c.label && <div className="wf-label">{c.label}</div>}
          <div className="wf-panel">
            {empty !== undefined ? <Empty text={empty} filtered={filtered} /> : (c.items ?? []).map((it, i) => {
              const [main, ...sub] = cells(it);
              const tone = sub.length > 0 ? TONES.find(([re]) => re.test(sub[sub.length - 1]))?.[1] : undefined;
              const info = tone ? sub.slice(0, -1) : sub;
              return (
                <div key={i} className={`wf-row ${c.target ? "link" : ""}`} onClick={open}>
                  {/^\p{L}/u.test(main.trim()) && <span className="wf-initial">{main.trim().charAt(0).toUpperCase()}</span>}
                  <span className="wf-row-main">{main}{info.length > 0 && <small>{info.join(" · ")}</small>}</span>
                  {tone && <b className={`wf-badge ${tone}`}>{sub[sub.length - 1]}</b>}
                  {c.target && <span className="wf-chev">›</span>}
                </div>
              );
            })}
          </div>
        </div>
      );
    case "stat": {
      const [name, value] = statParts(c.label);
      return <div className={"wf-stat" + toneClass(c.tone)}><span>{name}</span><strong>{value ?? "—"}</strong></div>;
    }
    case "card":
      return (
        <div className={`wf-card ${c.target ? "link" : ""} ${emphasisClass(c.emphasis)}${toneClass(c.tone)}`} onClick={open}>
          <strong>{c.label}</strong>
          {(c.items ?? []).map((it) => <span key={it}>{it}</span>)}
        </div>
      );
    default:
      return <p className={"wf-text" + toneClass(c.tone)}>{c.label}</p>;
  }
}

function Empty({ text, filtered }: { text: string; filtered?: boolean }) {
  return (
    <div className="wf-empty">
      <span className="wf-empty-icon"><Icon name={filtered ? "cari" : "kosong"} /></span>
      {text || "Belum ada data."}
      {filtered && <div><button type="button" className="wf-btn ghost">Hapus filter</button></div>}
    </div>
  );
}

function Field({ label, spec, error }: { label: string; spec?: FormField; error?: string }) {
  const name = spec?.label ?? label;
  const type = spec?.type ?? "text";
  const hint = example(name, type);
  const input = type === "textarea" ? <textarea name={name} placeholder={hint} rows={3} />
    : type === "select" ? <select name={name} defaultValue=""><option value="">Pilih {name}</option></select>
    : type === "checkbox" || type === "radio" ? <input name={name} type={type} />
    : <input name={name} placeholder={hint} inputMode={type === "currency" || type === "number" ? "decimal" : undefined}
        type={["email", "password", "number", "tel", "date", "time"].includes(type) ? type : type === "datetime" ? "datetime-local" : "text"} />;
  return (
    <label className={`wf-field ${error ? "invalid" : ""}`}>
      <span>{name}{spec?.required && <em aria-hidden> *</em>}</span>
      {input}
      {error && <small role="alert">{error}</small>}
    </label>
  );
}

// Placeholder with an example value; repeating the label adds nothing.
function example(label: string, type: string): string | undefined {
  const l = label.toLowerCase();
  if (type === "email" || /email/.test(l)) return "nama@contoh.com";
  if (type === "tel" || /telepon|hp|whatsapp|ponsel/.test(l)) return "0812 3456 7890";
  if (type === "password" || /sandi|password/.test(l)) return "••••••••";
  if (type === "currency" || /harga|biaya|jumlah|nominal|tarif/.test(l)) return "Rp 0";
  if (/nama/.test(l)) return "mis. Budi Santoso";
  if (/alamat/.test(l)) return "Jl. Contoh No. 1";
  if (/catatan|keterangan|deskripsi|keluhan/.test(l)) return "Tulis di sini (opsional)";
  if (/cari/.test(l)) return label + "…";
  return undefined;
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
