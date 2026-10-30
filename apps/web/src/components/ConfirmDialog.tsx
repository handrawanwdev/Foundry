import { useEffect, useId, useRef } from "react";
import { createPortal } from "react-dom";

// Short confirm overlay. Escape and the backdrop cancel; the first action
// that needs a second thought (hapus, generate ulang) should use this.
export function ConfirmDialog({
  title,
  body,
  confirmLabel,
  cancelLabel = "Batal",
  danger,
  busy,
  busyLabel,
  onCancel,
  onConfirm,
}: {
  title: string;
  body: string;
  confirmLabel: string;
  cancelLabel?: string;
  danger?: boolean;
  busy?: boolean;
  busyLabel?: string;
  onCancel: () => void;
  onConfirm: () => void;
}) {
  const titleId = useId();
  const bodyId = useId();
  const cancelRef = useRef<HTMLButtonElement>(null);
  const onCancelRef = useRef(onCancel);
  onCancelRef.current = onCancel;

  useEffect(() => {
    cancelRef.current?.focus();
  }, []);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape" && !busy) onCancelRef.current();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [busy]);

  return createPortal(
    <div className="modal-backdrop" onMouseDown={() => { if (!busy) onCancel(); }}>
      <div
        className="modal confirm"
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        aria-describedby={bodyId}
        onMouseDown={(e) => e.stopPropagation()}
      >
        <div className="modal-title" id={titleId}>{title}</div>
        <p id={bodyId}>{body}</p>
        <div className="modal-actions">
          <button ref={cancelRef} type="button" className="ghost" disabled={busy} onClick={onCancel}>
            {cancelLabel}
          </button>
          <button
            type="button"
            className={danger ? "danger" : "primary"}
            disabled={busy}
            onClick={onConfirm}
          >
            {busy ? (busyLabel ?? "Menunggu…") : confirmLabel}
          </button>
        </div>
      </div>
    </div>,
    document.body,
  );
}
