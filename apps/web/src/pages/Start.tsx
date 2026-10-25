import { useNavigate } from "react-router-dom";
import { agentName, useAI } from "../store/ai";
import { useProject } from "../store/project";
import { useWorkbench } from "../store/workbench";

// Start page when no project is open.
export function StartPage() {
  const { recent, open, removeRecent } = useProject();
  const { setDialog } = useWorkbench();
  const ai = useAI((s) => s.state);
  const nav = useNavigate();

  return (
    <div className="start">
      <h1 className="start-title">Dari prompt jadi blueprint dan instruksi.</h1>
      <p className="start-lead">Jelaskan software yang ingin dibuat atau diubah. Foundry menyusun blueprint dan command workflow; kamu tinjau, revisi, finalisasi, lalu ekspor untuk coding agent eksternal.</p>
      <div className="start-actions">
        <button className="primary big" onClick={() => setDialog("new")}>Project baru</button>
        <button className="big" onClick={() => setDialog("open")}>Buka project</button>
        <button className="ghost" onClick={() => nav("/simple/connect")}>
          {ai?.ready ? `AI: ${agentName(ai)}` : "Hubungkan AI dulu"}
        </button>
      </div>
      {recent.length > 0 && (
        <section className="recent-section">
          <div className="recent-head">
            <h2>Terakhir dibuka</h2>
            <button className="link small" onClick={() => removeRecent()} title="Hapus semua dari daftar (folder project tidak dihapus)">
              Hapus semua
            </button>
          </div>
          <div className="recent-grid">
            {recent.map((r) => {
              const folder = recentFolder(r.dir);
              return (
                <div key={r.dir} className="recent-row">
                  <button className="recent" onClick={() => void open(r.dir)} title={r.dir}>
                    <strong>{r.name}</strong>
                    {folder && <span className="muted small">{folder}</span>}
                  </button>
                  <button className="recent-remove" onClick={() => removeRecent(r.dir)} title="Hapus dari daftar (folder tidak dihapus)" aria-label={`Hapus ${r.name} dari daftar`}>
                    ×
                  </button>
                </div>
              );
            })}
          </div>
        </section>
      )}
    </div>
  );
}

// Parent folder of a project dir (full path stays in the card title).
export function recentFolder(dir: string) {
  const parts = dir.split(/[\\/]/).filter(Boolean);
  return parts.at(-2) ?? "";
}
