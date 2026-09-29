import { useEffect, useState } from "react";
import { useLocation } from "react-router-dom";
import { api } from "../api";
import { FILE_PREFIX } from "../flow";
import type { FileContent } from "../types";

// Read-only viewer for one project file (opened from the file explorer).
export function FileViewer() {
  const loc = useLocation();
  const rel = decodeURIComponent(loc.pathname.slice(FILE_PREFIX.length));
  const [file, setFile] = useState<FileContent | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [nonce, setNonce] = useState(0);

  useEffect(() => {
    let live = true;
    setError(null);
    api.file(rel)
      .then((f) => live && setFile(f))
      .catch((e: Error) => live && (setError(e.message), setFile(null)));
    return () => { live = false; };
  }, [rel, nonce]);

  const lines = file?.content?.split("\n") ?? [];
  if (lines.length > 1 && lines[lines.length - 1] === "") lines.pop();

  return (
    <div className="viewer">
      <div className="viewer-head">
        <span className="breadcrumb">{rel.split("/").join(" › ")}</span>
        {file && <span className="muted small">{formatSize(file.size)}{file.content !== undefined ? ` · ${lines.length} baris` : ""} · hanya baca</span>}
        <button className="icon-btn" title="Muat ulang" onClick={() => setNonce(nonce + 1)}>↻</button>
      </div>
      {error && <div className="viewer-msg error-text">{error}</div>}
      {file?.binary && <div className="viewer-msg muted">File biner — tidak ditampilkan.</div>}
      {file?.truncated && <div className="viewer-msg warn">File lebih dari 1 MB, hanya bagian awal yang ditampilkan.</div>}
      {file && !file.binary && (
        <pre className="code-view">
          {lines.map((l, i) => (
            <div key={i} className="code-line"><span className="ln">{i + 1}</span><span className="lc">{l || " "}</span></div>
          ))}
        </pre>
      )}
    </div>
  );
}

function formatSize(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / 1024 / 1024).toFixed(1)} MB`;
}
