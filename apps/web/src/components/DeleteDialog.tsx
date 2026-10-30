import { useEffect, useState } from "react";
import { api } from "../api";
import { Elapsed } from "./Elapsed";
import { useProject } from "../store/project";
import type { DeleteImpact } from "../types";

// Confirmation before a blueprint document is deleted: what goes with it and
// what it reaches, in plain words. The numbers come from the server, which
// computes them with the same code that does the deletion.
export function DeleteDialog({ id, title, onClose, onDeleted }: {
  id: string; title: string; onClose: () => void; onDeleted: () => void;
}) {
  const { run, pending } = useProject();
  const [impact, setImpact] = useState<DeleteImpact | null>(null);
  const [error, setError] = useState("");
  useEffect(() => {
    api.deleteImpact(id).then(setImpact, (e: Error) => setError(e.message));
  }, [id]);

  const removed = impact?.commands.filter((c) => c.fate === "removed") ?? [];
  const review = impact?.commands.filter((c) => c.fate === "review") ?? [];
  const busy = pending === "deleteDocument";
  const confirm = async () => {
    if (await run("deleteDocument", () => api.deleteDocument(id))) onDeleted();
  };
  const cmd = (c: { id: string; name: string; why: string }) => <li key={c.id}><strong>{c.id}</strong> {c.name} <span className="muted">— {c.why}</span></li>;

  return (
    <div className="modal-backdrop" onMouseDown={onClose}>
      <div className="modal" role="dialog" aria-label={`Hapus ${title}`} onMouseDown={(e) => e.stopPropagation()}>
        <div className="modal-title">Hapus “{title}”?</div>
        {error ? <p className="warn-text small">{error}</p> : !impact ? <p className="muted small"><span className="spinner" /> Menghitung dampak… <Elapsed /></p> : (
          <div className="delete-impact small">
            <p>{impact.erases}</p>
            {removed.length > 0 && (
              <>
                <p><strong>{removed.length} command ikut dihapus</strong> karena hanya dibuat untuk dokumen ini:</p>
                <ul>{removed.map(cmd)}</ul>
              </>
            )}
            {review.length > 0 && (
              <>
                <p><strong>{review.length} command dipertahankan</strong> tetapi ditandai perlu ditinjau, karena sebagian isinya mengacu ke dokumen ini:</p>
                <ul>{review.map(cmd)}</ul>
              </>
            )}
            {impact.artifacts.length > 0 && (
              <>
                <p><strong>{impact.artifacts.length} dokumen lain</strong> menyebut dokumen ini dan akan ditulis ulang:</p>
                <ul>{impact.artifacts.map((a) => <li key={a.id}>{a.title}</li>)}</ul>
              </>
            )}
            {impact.areas.length > 0 && (
              <p className="warn-text">
                Bagian blueprint ini jadi belum lengkap: {impact.areas.join(", ")}. Tulis ulang dengan AI atau lewati dengan alasan sebelum finalisasi.
              </p>
            )}
            {impact.uncovered.length > 0 && (
              <p className="warn-text">
                Kebutuhan {impact.uncovered.join(", ")} tidak lagi dikerjakan command mana pun. Buat command pengganti sebelum finalisasi.
              </p>
            )}
            {impact.ai
              ? <p className="muted">Setelah dihapus, AI merapikan dokumen dan command yang tersisa agar tidak lagi menyebut dokumen ini. Kalau AI gagal, penghapusan tetap berlaku dan Anda bisa mencoba lagi.</p>
              : <p className="muted">Tidak ada dokumen atau command lain yang terpengaruh.</p>}
            <p className="warn-text">Penghapusan tidak bisa dibatalkan.</p>
          </div>
        )}
        <div className="modal-actions">
          <button className="ghost" disabled={busy} onClick={onClose}>Batal</button>
          <button className="danger" disabled={!impact || busy} onClick={() => void confirm()}>
            {busy ? "Menghapus…" : impact?.ai ? "Ya, hapus dan rapikan" : "Ya, hapus"}
          </button>
        </div>
      </div>
    </div>
  );
}
