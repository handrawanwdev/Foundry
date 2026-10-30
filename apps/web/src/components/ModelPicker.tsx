import { useEffect, useId, useMemo, useRef, useState } from "react";
import { Elapsed } from "./Elapsed";

// Model input with a searchable dropdown: type to filter or enter any name
// (the CLI accepts models not in the list); empty means the CLI default.
export function ModelPicker({ models, value, onChange, loading }: { models: string[]; value: string; onChange: (v: string) => void; loading?: boolean }) {
  const [open, setOpen] = useState(false);
  const [typed, setTyped] = useState(false); // filter only after the user types
  const [active, setActive] = useState(-1);
  const box = useRef<HTMLDivElement>(null);
  const input = useRef<HTMLInputElement>(null);
  const listId = useId();

  const options = useMemo(() => {
    const q = value.trim().toLowerCase();
    return typed && q ? models.filter((m) => m.toLowerCase().includes(q)) : models;
  }, [models, value, typed]);

  useEffect(() => {
    if (!open) return;
    const close = (e: MouseEvent) => { if (!box.current?.contains(e.target as Node)) setOpen(false); };
    document.addEventListener("mousedown", close);
    return () => document.removeEventListener("mousedown", close);
  }, [open]);

  const show = () => { setOpen(true); setTyped(false); setActive(-1); };
  const choose = (m: string) => { onChange(m); setOpen(false); };

  return (
    <div className="mp" ref={box}>
      <input ref={input} role="combobox" aria-autocomplete="list" aria-expanded={open} aria-controls={listId}
        aria-activedescendant={open && active >= 0 ? `${listId}-${active}` : undefined}
        value={value} placeholder="default" spellCheck={false} autoComplete="off"
        onFocus={show} onClick={() => !open && show()}
        onChange={(e) => { onChange(e.target.value); setTyped(true); setOpen(true); setActive(-1); }}
        onKeyDown={(e) => {
          if (e.key === "ArrowDown") { e.preventDefault(); if (!open) show(); else setActive((a) => Math.min(a + 1, options.length - 1)); }
          else if (e.key === "ArrowUp") { e.preventDefault(); setActive((a) => Math.max(a - 1, 0)); }
          else if (e.key === "Enter" && open && active >= 0) { e.preventDefault(); choose(options[active]); }
          else if (e.key === "Escape" && open) { e.preventDefault(); setOpen(false); }
          else if (e.key === "Tab") setOpen(false);
        }} />
      <button type="button" className="mp-toggle" tabIndex={-1} aria-label="Tampilkan daftar model"
        onMouseDown={(e) => { e.preventDefault(); if (open) setOpen(false); else { show(); input.current?.focus(); } }}>▾</button>
      {open && (
        <ul role="listbox" id={listId} className="mp-pop">
          {options.map((m, i) => (
            <li key={m} id={`${listId}-${i}`} role="option" aria-selected={m === value}
              className={`${i === active ? "active" : ""} ${m === value ? "on" : ""}`}
              onMouseEnter={() => setActive(i)} onMouseDown={(e) => { e.preventDefault(); choose(m); }}>
              {m}
            </li>
          ))}
          {loading && <li className="muted" aria-live="polite"><span className="spinner" /> Memuat model terbaru… <Elapsed /></li>}
          {!loading && !options.length && <li className="muted">Tidak ada di daftar — nama ini tetap dipakai</li>}
        </ul>
      )}
    </div>
  );
}
