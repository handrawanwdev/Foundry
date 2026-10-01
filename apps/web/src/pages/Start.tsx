import { useNavigate } from "react-router-dom";
import { stepFor } from "../flow";
import { agentName, useAI } from "../store/ai";
import { useProject } from "../store/project";
import { useWorkbench } from "../store/workbench";

// Start page when no project is open (like Visual Studio's start window).
export function StartPage() {
  const { recent, open, removeRecent } = useProject();
  const { setDialog } = useWorkbench();
  const ai = useAI((s) => s.state);
  const nav = useNavigate();

  const openRecent = async (dir: string) => {
    await open(dir);
  };

  return (
    <div className="start">
      <h1 className="start-title">Foundry</h1>
      <p className="muted">Ceritakan aplikasi yang kamu inginkan. Foundry mengurus sisanya sampai siap diperiksa.</p>
      <div className="start-grid">
        <div>
          <div className="recent-head">
            <h3>Buka terakhir</h3>
            {recent.length > 0 && (
              <button className="link small" onClick={() => removeRecent()} title="Hapus semua dari daftar (folder project tidak dihapus)">
                Hapus semua
              </button>
            )}
          </div>
          {recent.length === 0 && <p className="muted small">Belum ada project.</p>}
          {recent.map((d) => (
            <div key={d} className="recent-row">
              <button className="recent" onClick={() => openRecent(d)} title={d}>
                <strong>{d.split("/").filter(Boolean).pop()}</strong>
                <span className="muted small">{d}</span>
              </button>
              <button className="recent-remove" onClick={() => removeRecent(d)} title="Hapus dari daftar (folder tidak dihapus)" aria-label={`Hapus ${d} dari daftar`}>
                ×
              </button>
            </div>
          ))}
        </div>
        <div>
          <h3>Mulai</h3>
          <button className="start-action" onClick={() => setDialog("new")}>
            <strong>Project baru</strong><span className="muted small">Buat folder baru lalu ceritakan idenya</span>
          </button>
          <button className="start-action" onClick={() => setDialog("open")}>
            <strong>Buka project</strong><span className="muted small">Folder Foundry atau project yang sudah ada</span>
          </button>
          <button className="start-action" onClick={() => nav("/simple/connect")}>
            <strong>{ai?.ready ? `AI: ${agentName(ai)}` : "Hubungkan AI"}</strong>
            <span className="muted small">{ai?.ready ? "Siap digunakan" : "Wajib sebelum mulai merancang"}</span>
          </button>
        </div>
      </div>
    </div>
  );
}

// Empty editor (all tabs closed) while a project is open.
export function EmptyEditor() {
  const project = useProject((s) => s.project);
  const nav = useNavigate();
  if (!project) return null;
  const step = stepFor(project.stage);
  return (
    <div className="watermark">
      <div className="start-title">◆</div>
      <button className="link" onClick={() => nav(step.path)}>Lanjutkan: {step.label}</button>
      <div className="muted small">File → Tutup Project untuk kembali ke Start</div>
    </div>
  );
}
