import { useEffect, useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { api } from "../api";
import { CATEGORY_LABEL, groupArtifacts, usedBy } from "../blueprint";
import { Markdown } from "../components/Markdown";
import { PromptBar } from "../components/PromptBar";
import { DESIGN_PATH, isWorking } from "../flow";
import { useProject } from "../store/project";
import type { Artifact, ArtifactCategory, BlueprintView, Project } from "../types";
import { PlatformStructure } from "./DesignInfo";

// Blueprint workspace: the artifact list on the left, one document at a time.
export function BlueprintPage() {
  const { project, run, pending } = useProject();
  const [view, setView] = useState<BlueprintView | null>(null);
  const [params, setParams] = useSearchParams();

  useEffect(() => {
    if (!project?.discovery) return;
    api.blueprint().then(setView).catch(() => setView(null));
  }, [project?.version, project?.dir]);

  if (!project) return null;
  if (!project.discovery) {
    return (
      <div className="dw-empty">
        <h1>Belum ada blueprint</h1>
        <p className="muted">Blueprint dibuat dari prompt: apa yang ingin dibuat atau diubah.</p>
        <Link className="button primary" to="/simple/tell">Tulis prompt</Link>
      </div>
    );
  }
  const docs = view?.artifacts ?? [];
  const id = params.get("doc") ?? docs[0]?.id ?? "";
  const doc = docs.find((d) => d.id === id);
  const locked = project.stage === "READY";
  const working = isWorking(project);
  const select = (next: string) => setParams({ doc: next }, { replace: true });
  const designing = !!view?.has_ui && !docs.some((d) => d.id === "ui-design");

  return (
    <div className="ws">
      <nav className="ws-nav" aria-label="Artefak blueprint">
        <div className="ws-nav-head">
          <strong>Blueprint</strong>
          <span className="muted small">{docs.length} artefak</span>
        </div>
        {groupArtifacts(docs).map(([cat, ds]) => (
          <div key={cat} className="ws-group">
            <div className="ws-group-label">{CATEGORY_LABEL[cat]}</div>
            {ds.map((d) => (
              <button key={d.id} className={`ws-item ${d.id === id ? "on" : ""}`} aria-current={d.id === id ? "page" : undefined} onClick={() => select(d.id)}>
                <span className="ws-item-text">{d.title}</span>
                {usedBy(project.workflow, d.id).length > 0 && <span className="ws-count" title="Dipakai command">{usedBy(project.workflow, d.id).length}</span>}
              </button>
            ))}
          </div>
        ))}
        {designing && (
          <div className="ws-group">
            <div className="ws-group-label">{CATEGORY_LABEL.ui}</div>
            <Link className="ws-item" to={DESIGN_PATH}><span className="ws-item-text">Desain UI/UX</span><span className="muted small">dibuat…</span></Link>
          </div>
        )}
        <div className="ws-group">
          <div className="ws-group-label">Keputusan</div>
          <button className={`ws-item ${id === "platform" ? "on" : ""}`} onClick={() => select("platform")}>
            <span className="ws-item-text">Platform & Teknologi</span>
          </button>
        </div>
      </nav>

      <main className="ws-main">
        {id === "platform" ? <PlatformStructure project={project} />
          : doc ? <ArtifactDoc key={doc.id} project={project} doc={doc} locked={locked} onOpen={select} />
          : <p className="muted">{view ? "Pilih artefak di kiri." : "Memuat blueprint…"}</p>}
      </main>

      {!locked && (
        <PromptBar
          label={doc ? `Minta perubahan blueprint (sedang melihat: ${doc.title})…` : "Minta perubahan blueprint…"}
          placeholder="Contoh: tambahkan threat model untuk login dan pembayaran; atau jelaskan kontrak event antar modul."
          hint="AI menambah atau mengubah artefak yang terdampak saja."
          disabled={working}
          busy={pending === "reviseBlueprint"}
          onSend={(text) => run("reviseBlueprint", () => api.reviseBlueprint(text, doc ? [doc.id] : []))}
        />
      )}
    </div>
  );
}

function ArtifactDoc({ project, doc, locked, onOpen }: { project: Project; doc: Artifact; locked: boolean; onOpen: (id: string) => void }) {
  const nav = useNavigate();
  const [editing, setEditing] = useState(false);
  const used = usedBy(project.workflow, doc.id);
  return (
    <article className="doc">
      <header className="doc-head">
        <div className="doc-kicker">
          <span className="tag">{CATEGORY_LABEL[doc.category] ?? doc.category}</span>
          {doc.builtin ? <span className="muted small">Dari data terstruktur</span> : <span className="muted small">v{doc.version}</span>}
        </div>
        <h1>{doc.title}</h1>
        {doc.summary && <p className="doc-summary">{doc.summary}</p>}
        <div className="doc-actions">
          {doc.id === "ui-design" && <button className="primary" onClick={() => nav(DESIGN_PATH)}>Buka workspace desain</button>}
          {!doc.builtin && !locked && !editing && <button onClick={() => setEditing(true)}>Edit</button>}
          {!doc.builtin && !locked && !editing && <RemoveButton id={doc.id} />}
          {used.length > 0 && (
            <span className="small muted">Dipakai oleh {used.map((c) => <Link key={c} className="ref-chip" to={`/workflow?cmd=${c}`}>{c}</Link>)}</span>
          )}
        </div>
      </header>
      {editing ? <ArtifactEditor doc={doc} onDone={() => setEditing(false)} /> : <Markdown text={doc.body} />}
      {(doc.refs?.length ?? 0) > 0 && (
        <footer className="doc-foot">
          <span className="muted small">Terkait:</span>
          {doc.refs!.map((r) => <button key={r} className="ref-chip" onClick={() => onOpen(r)}>{r}</button>)}
        </footer>
      )}
    </article>
  );
}

function ArtifactEditor({ doc, onDone }: { doc: Artifact; onDone: () => void }) {
  const { run, pending } = useProject();
  const [title, setTitle] = useState(doc.title);
  const [summary, setSummary] = useState(doc.summary ?? "");
  const [category, setCategory] = useState<ArtifactCategory>(doc.category);
  const [body, setBody] = useState(doc.body);
  const save = async () => {
    if (await run("editArtifact", () => api.editArtifact(doc.id, { title, summary, category, body, refs: doc.refs }))) onDone();
  };
  return (
    <div className="doc-edit">
      <label className="field">Judul<input value={title} onChange={(e) => setTitle(e.target.value)} /></label>
      <label className="field">Ringkasan<input value={summary} onChange={(e) => setSummary(e.target.value)} /></label>
      <label className="field">Kategori
        <select value={category} onChange={(e) => setCategory(e.target.value as ArtifactCategory)}>
          {(Object.keys(CATEGORY_LABEL) as ArtifactCategory[]).map((c) => <option key={c} value={c}>{CATEGORY_LABEL[c]}</option>)}
        </select>
      </label>
      <label className="field">Isi (Markdown)<textarea className="mono" rows={18} value={body} onChange={(e) => setBody(e.target.value)} /></label>
      <div className="row">
        <button className="primary" disabled={!title.trim() || !body.trim() || pending === "editArtifact"} onClick={() => void save()}>Simpan</button>
        <button className="ghost" onClick={onDone}>Batal</button>
      </div>
    </div>
  );
}

// Two clicks: the second one confirms.
function RemoveButton({ id }: { id: string }) {
  const { run, pending } = useProject();
  const [sure, setSure] = useState(false);
  if (!sure) return <button className="ghost" onClick={() => setSure(true)}>Hapus</button>;
  return (
    <>
      <button className="danger" disabled={pending === "removeArtifact"} onClick={() => void run("removeArtifact", () => api.removeArtifact(id))}>Ya, hapus artefak</button>
      <button className="ghost" onClick={() => setSure(false)}>Batal</button>
    </>
  );
}
