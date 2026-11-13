import { useLayoutEffect, useRef, useState, type KeyboardEvent, type ReactNode } from "react";

// Floating prompt for a change request, written like a chat message: no
// length limit, the box grows with the text, Enter sends and Shift+Enter
// starts a new line. Collapsed it is one line.
export function PromptBar({ label, placeholder, hint, extra, disabled, busy, onSend }: {
  label: string;
  placeholder: string;
  hint?: string;
  extra?: ReactNode;
  disabled?: boolean;
  busy?: boolean;
  onSend: (text: string) => Promise<boolean>;
}) {
  const [open, setOpen] = useState(false);
  const [text, setText] = useState("");
  const box = useRef<HTMLTextAreaElement>(null);
  useLayoutEffect(() => {
    const el = box.current;
    if (!el) return;
    el.style.height = "auto";
    el.style.height = `${el.scrollHeight}px`;
  }, [text, open]);
  const send = async () => {
    if (!text.trim() || busy || disabled) return;
    if (await onSend(text.trim())) {
      setText("");
      setOpen(false);
    }
  };
  const onKey = (e: KeyboardEvent) => {
    if (e.key === "Enter" && !e.shiftKey && !e.nativeEvent.isComposing) {
      e.preventDefault();
      void send();
    }
    if (e.key === "Escape") setOpen(false);
  };
  return (
    <div className="ws-dock">
      {open ? (
        <div className="prompt-box">
          <textarea ref={box} autoFocus value={text} placeholder={placeholder} onChange={(e) => setText(e.target.value)} onKeyDown={onKey} rows={3} />
          <div className="prompt-foot">
            {extra}
            <span className="muted small">{hint ? `${hint} · ` : ""}Enter kirim · Shift+Enter baris baru</span>
            <button className="ghost" onClick={() => setOpen(false)}>Batal</button>
            <button className="primary" disabled={!text.trim() || busy || disabled} onClick={() => void send()}>{busy ? "Mengirim…" : "Kirim"}</button>
          </div>
        </div>
      ) : (
        <button className="composer" disabled={disabled} onClick={() => setOpen(true)}>
          <span className="composer-spark" aria-hidden>✦</span>
          <span className="composer-text">{label}</span>
        </button>
      )}
    </div>
  );
}
