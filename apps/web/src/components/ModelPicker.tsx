import { useEffect, useId, useMemo, useRef, useState } from "react";

// Searchable dropdown for the agent model. "" means the CLI default; a typed
// name that is not in the list can still be used, since the CLI accepts any.
export function ModelPicker({ models, value, onChange, loading }: { models: string[]; value: string; onChange: (v: string) => void; loading?: boolean }) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [active, setActive] = useState(0);
  const box = useRef<HTMLDivElement>(null);
  const listId = useId();

  const q = query.trim();
  const options = useMemo(() => {
    const hits = models.filter((m) => m.toLowerCase().includes(q.toLowerCase()));
    const out = q ? hits : ["", ...hits];
    if (q && !models.includes(q)) out.push(q);
    return out;
  }, [models, q]);

  useEffect(() => {
    if (!open) return;
    const close = (e: MouseEvent) => { if (!box.current?.contains(e.target as Node)) setOpen(false); };
    document.addEventListener("mousedown", close);
    return () => document.removeEventListener("mousedown", close);
  }, [open]);

  const choose = (v: string) => { onChange(v); setOpen(false); setQuery(""); };
  const label = (m: string) => (m === "" ? "default" : m === q && !models.includes(q) ? `Pakai “${q}”` : m);

  return (
    <div className="mp" ref={box}>
      <button type="button" className="mp-trigger" aria-haspopup="listbox" aria-expanded={open}
        onClick={() => { setOpen(!open); setActive(0); }}>
        <span className={value ? "" : "muted"}>{value || "default"}</span><span aria-hidden>▾</span>
      </button>
      {open && (
        <div className="mp-pop">
          <input autoFocus role="combobox" aria-expanded aria-controls={listId} aria-activedescendant={`${listId}-${active}`}
            value={query} placeholder="Cari model…"
            onChange={(e) => { setQuery(e.target.value); setActive(0); }}
            onKeyDown={(e) => {
              if (e.key === "ArrowDown") { e.preventDefault(); setActive((a) => Math.min(a + 1, options.length - 1)); }
              else if (e.key === "ArrowUp") { e.preventDefault(); setActive((a) => Math.max(a - 1, 0)); }
              else if (e.key === "Enter") { e.preventDefault(); if (options[active] !== undefined) choose(options[active]); }
              else if (e.key === "Escape") { e.preventDefault(); setOpen(false); }
            }} />
          <ul role="listbox" id={listId}>
            {options.map((m, i) => (
              <li key={m || "(default)"} id={`${listId}-${i}`} role="option" aria-selected={m === value}
                className={`${i === active ? "active" : ""} ${m === value ? "on" : ""}`}
                onMouseEnter={() => setActive(i)} onMouseDown={(e) => { e.preventDefault(); choose(m); }}>
                {label(m)}
              </li>
            ))}
            {!options.length && <li className="muted">Tidak ada model</li>}
            {loading && <li className="muted" aria-live="polite"><span className="spinner" /> Memuat model terbaru…</li>}
          </ul>
        </div>
      )}
    </div>
  );
}
