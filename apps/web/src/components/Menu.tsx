import { useEffect, useRef, useState, type ReactNode } from "react";
import { Icon } from "./Icon";

// Small dropdown. Closes on outside click and Escape; the content gets a
// close callback so an item can dismiss the menu.
export function Menu({ trigger, align = "left", className = "", children }: {
  trigger: ReactNode;
  align?: "left" | "right";
  className?: string;
  children: (close: () => void) => ReactNode;
}) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      if (!ref.current?.contains(e.target as Node)) setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    document.addEventListener("mousedown", onDown);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onDown);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  return (
    <div className="menu" ref={ref}>
      <button className={`menu-trigger ${className} ${open ? "open" : ""}`} aria-haspopup="menu" aria-expanded={open} onClick={() => setOpen(!open)}>
        {trigger}
        <Icon name="chevron" size={14} />
      </button>
      {open && <div className={`menu-pop ${align}`} role="menu">{children(() => setOpen(false))}</div>}
    </div>
  );
}
