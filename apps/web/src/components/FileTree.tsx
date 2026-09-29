import { useCallback, useEffect, useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { api } from "../api";
import { fileRoute } from "../flow";
import type { FileEntry } from "../types";

// Lazy project file tree: folders load their children when expanded.
// `reload` changes force every open folder to refetch.
export function FileTree({ reload }: { reload: number }) {
  return (
    <div className="file-tree">
      <Folder path="" depth={0} reload={reload} open />
    </div>
  );
}

function Folder({ path, depth, reload, open }: { path: string; depth: number; reload: number; open: boolean }) {
  const [entries, setEntries] = useState<FileEntry[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      setEntries(await api.files(path));
      setError(null);
    } catch (e) {
      setError((e as Error).message);
    }
  }, [path]);

  useEffect(() => {
    if (open) void load();
  }, [open, reload, load]);

  if (!open) return null;
  if (error) return <div className="file-row muted small" style={{ paddingLeft: pad(depth) }}>{error}</div>;
  if (!entries) return <div className="file-row muted small" style={{ paddingLeft: pad(depth) }}>…</div>;
  if (entries.length === 0) return <div className="file-row muted small" style={{ paddingLeft: pad(depth) }}>(kosong)</div>;
  return (
    <>
      {entries.map((e) => e.is_dir
        ? <DirNode key={e.path} entry={e} depth={depth} reload={reload} />
        : <FileNode key={e.path} entry={e} depth={depth} />)}
    </>
  );
}

function DirNode({ entry, depth, reload }: { entry: FileEntry; depth: number; reload: number }) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <button className="file-row" style={{ paddingLeft: pad(depth) }} onClick={() => setOpen(!open)} title={entry.path}>
        <span className="caret">{open ? "▾" : "▸"}</span>
        <span className="file-name">{entry.name}</span>
      </button>
      <Folder path={entry.path} depth={depth + 1} reload={reload} open={open} />
    </>
  );
}

function FileNode({ entry, depth }: { entry: FileEntry; depth: number }) {
  const nav = useNavigate();
  const loc = useLocation();
  const route = fileRoute(entry.path);
  return (
    <button className={`file-row ${loc.pathname === route ? "active" : ""}`} style={{ paddingLeft: pad(depth) }}
      onClick={() => nav(route)} title={entry.path}>
      <span className="caret" />
      <span className={`file-dot ext-${ext(entry.name)}`} />
      <span className="file-name">{entry.name}</span>
    </button>
  );
}

const pad = (depth: number) => `${0.5 + depth * 0.8}rem`;

export function ext(name: string): string {
  const i = name.lastIndexOf(".");
  return i > 0 ? name.slice(i + 1).toLowerCase() : "";
}
