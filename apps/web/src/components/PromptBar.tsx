import { useState, type KeyboardEvent, type ReactNode } from "react";

// Floating prompt for a change request. Collapsed it is one line; it opens
// into a text box. Ctrl/⌘+Enter sends.
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
  const send = async () => {
    if (!text.trim() || busy) return;
    if (await onSend(text.trim())) {
      setText("");
      setOpen(false);
    }
  };
  const onKey = (e: KeyboardEvent) => {
    if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) void send();
    if (e.key === "Escape") setOpen(false);
  };
  return (
    <div className="ws-dock">
      {open ? (
        <div className="prompt-box">
          <textarea autoFocus value={text} placeholder={placeholder} onChange={(e) => setText(e.target.value)} onKeyDown={onKey} rows={3} />
          <div className="prompt-foot">
            {extra}
            <span className="muted small">{hint ?? "Ctrl+Enter untuk kirim"}</span>
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
