import { useEffect, useRef, useState } from "react";
import { api } from "../api";
import { useProject } from "../store/project";
import { useWorkbench } from "../store/workbench";
import type { Browse } from "../types";

// Open Project / New Project dialog with a folder browser.
export function ProjectDialog() {
  const { dialog, setDialog } = useWorkbench();
  const { open, pending } = useProject();
  const [path, setPath] = useState("");
  const [data, setData] = useState<Browse | null>(null);
  const [err, setErr] = useState<string | null>(null);
  const [name, setName] = useState("");

  const seq = useRef(0);
  const typed = useRef(false); // user edited the path field since the last listing
  const browse = async (p?: string, initial = false) => {
    const id = ++seq.current;
    if (!initial) typed.current = false;
    try {
      const b = await api.browse(p);
      if (id !== seq.current) return; // a newer navigation won
      if (initial && typed.current) return; // don't clobber what the user typed
      setData(b);
      setPath(b.path);
      setErr(null);
    } catch (e) {
      if (id === seq.current) setErr((e as Error).message);
    }
  };

  useEffect(() => {
    if (!dialog) return;
    setName("");
    setData(null); // no stale listing while the fresh one loads
    typed.current = false;
    void browse(data?.path, true);
  }, [dialog]);

  if (!dialog) return null;
  const isNew = dialog === "new";
  const target = isNew ? `${path.replace(/\/$/, "")}/${name.trim()}` : path;

  const submit = async () => {
    if (await open(target, isNew, isNew ? name.trim() : "")) {
      setDialog(null);
    }
  };

  return (
    <div className="modal-backdrop" onMouseDown={() => setDialog(null)}>
      <div className="modal" onMouseDown={(e) => e.stopPropagation()}>
        <div className="modal-title">{isNew ? "Project Baru" : "Buka Project"}</div>
        <form className="path-row" onSubmit={(e) => { e.preventDefault(); void browse(path); }}>
          <button type="button" disabled={!data?.parent} onClick={() => browse(data?.parent)} title="Folder induk">↑</button>
          <input value={path} onChange={(e) => { typed.current = true; setPath(e.target.value); }} className="grow" />
          <button type="submit">Buka folder</button>
        </form>
        <div className="dir-list">
          {(data?.dirs ?? []).map((d) => (
            <button key={d.path} className="dir" onClick={() => browse(d.path)} onDoubleClick={() => { if (!isNew) void open(d.path).then((ok) => { if (ok) setDialog(null); }); }}>
              <span>📁 {d.name}</span>
              {d.is_project && <span className="tag">Foundry</span>}
            </button>
          ))}
          {data && data.dirs.length === 0 && <div className="muted small pad">Tidak ada sub-folder</div>}
        </div>
        {err && <div className="error-text small">{err}</div>}
        {isNew ? (
          <label className="field">Nama project (folder baru di lokasi di atas)
            <input value={name} onChange={(e) => setName(e.target.value)} placeholder="klinik-sehat" autoFocus />
          </label>
        ) : (
          <div className="small muted">
            {data?.is_project ? "Folder ini sudah berisi project Foundry." : "Folder biasa: Foundry akan membaca struktur project yang sudah ada."}
          </div>
        )}
        <div className="modal-actions">
          <span className="muted small path-preview">{target}</span>
          <button className="ghost" onClick={() => setDialog(null)}>Batal</button>
          <button className="primary" disabled={pending === "open" || (isNew && !/^[\w.-]+$/.test(name.trim()))} onClick={submit}>
            {isNew ? "Buat" : "Buka"}
          </button>
        </div>
      </div>
    </div>
  );
}
