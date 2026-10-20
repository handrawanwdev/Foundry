import { useEffect, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { api, ApiError, type ErrorDetail } from "../api";
import { CATEGORY_LABEL, DESIGN_DOCS, designIssuesOf, groupArtifacts, isDesignDoc, usedBy } from "../blueprint";
import { Markdown } from "../components/Markdown";
import { PromptBar } from "../components/PromptBar";
import { isWorking } from "../flow";
import { useProject } from "../store/project";
import type { Artifact, ArtifactCategory, BlueprintView, DesignView, Project } from "../types";
import { PlatformStructure } from "./Platform";

// Blueprint workspace: the artifact list on the left, one document at a time.
export function BlueprintPage() {
  const { project, run, pending } = useProject();
  const [view, setView] = useState<BlueprintView | null>(null);
  const [design, setDesign] = useState<DesignView | null>(null);
  const [params, setParams] = useSearchParams();

  useEffect(() => {
    if (!project?.discovery) return;
    api.blueprint().then(setView).catch(() => setView(null));
    api.design().then(setDesign).catch(() => setDesign(null));
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
  // A product with screens but no design rules or layout yet.
  const designing = !!view?.has_ui && !docs.some((d) => isDesignDoc(d.id));
  const designDoc = !!doc && isDesignDoc(doc.id);

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
            <button className={`ws-item ${id === "design" ? "on" : ""}`} onClick={() => select("design")}>
              <span className="ws-item-text">Aturan desain & layout</span><span className="muted small">belum ada</span>
            </button>
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
          : id === "design" && designing ? <DesignEmpty project={project} locked={locked} />
          : doc ? <ArtifactDoc key={doc.id} project={project} doc={doc} locked={locked} design={design} onOpen={select} />
          : <p className="muted">{view ? "Pilih artefak di kiri." : "Memuat blueprint…"}</p>}
      </main>

      {!locked && (designDoc ? (
        <PromptBar
          label={`Minta perubahan rancangan UI (sedang melihat: ${doc!.title})…`}
          placeholder="Contoh: halaman Antrean pakai tabel padat dengan filter di atas; tambahkan state kosong untuk pencarian."
          hint="AI mengubah aturan desain atau halaman yang terdampak saja; hasil divalidasi sebelum disimpan."
          disabled={working}
          busy={pending === "reviseDesign"}
          onSend={(text) => run("reviseDesign", () => api.reviseDesign(text, [doc!.id]))}
        />
      ) : (
        <PromptBar
          label={doc ? `Minta perubahan blueprint (sedang melihat: ${doc.title})…` : "Minta perubahan blueprint…"}
          placeholder="Contoh: tambahkan threat model untuk login dan pembayaran; atau jelaskan kontrak event antar modul."
          hint="AI menambah atau mengubah artefak yang terdampak saja."
          disabled={working}
          busy={pending === "reviseBlueprint"}
          onSend={(text) => run("reviseBlueprint", () => api.reviseBlueprint(text, doc ? [doc.id] : []))}
        />
      ))}
    </div>
  );
}

function ArtifactDoc({ project, doc, locked, design, onOpen }: {
  project: Project; doc: Artifact; locked: boolean; design: DesignView | null; onOpen: (id: string) => void;
}) {
  const [editing, setEditing] = useState(false);
  const used = usedBy(project.workflow, doc.id);
  const designDoc = isDesignDoc(doc.id);
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
          {(designDoc || !doc.builtin) && !locked && !editing && <button onClick={() => setEditing(true)}>{designDoc ? "Edit data" : "Edit"}</button>}
          {!doc.builtin && !locked && !editing && <RemoveButton id={doc.id} />}
          {used.length > 0 && (
            <span className="small muted">Dipakai oleh {used.map((c) => <Link key={c} className="ref-chip" to={`/workflow?cmd=${c}`}>{c}</Link>)}</span>
          )}
        </div>
      </header>
      {designDoc && design && <DesignIssues issues={designIssuesOf(design.issues, doc.id)} />}
      {editing
        ? designDoc && design ? <DesignEditor doc={doc.id} design={design} onDone={() => setEditing(false)} /> : <ArtifactEditor doc={doc} onDone={() => setEditing(false)} />
        : <Markdown text={doc.body} />}
      {(doc.refs?.length ?? 0) > 0 && (
        <footer className="doc-foot">
          <span className="muted small">Terkait:</span>
          {doc.refs!.map((r) => <button key={r} className="ref-chip" onClick={() => onOpen(r)}>{r}</button>)}
        </footer>
      )}
    </article>
  );
}

// No design rules or layout yet: the AI writes both from the features and
// flows. Text and structured data only.
function DesignEmpty({ project, locked }: { project: Project; locked: boolean }) {
  const { run, pending } = useProject();
  const busy = project.job?.kind === "design" && isWorking(project);
  return (
    <div className="doc">
      <h1>Aturan desain & spesifikasi layout</h1>
      <p className="muted">
        Produk ini punya layar. Foundry menyusun aturan desain (prinsip, panduan UX, token, komponen, aksesibilitas, responsif,
        interaksi, aturan platform) dan spesifikasi layout per halaman (section, hierarki, grid, navigasi, state UI) sebagai teks
        dan data terstruktur untuk coding agent. Tidak ada gambar atau kanvas.
      </p>
      {busy ? <p><span className="spinner" /> {project.job?.message}…</p> : !locked && (
        <button className="primary" disabled={isWorking(project) || pending === "generateDesign"} onClick={() => void run("generateDesign", api.generateDesign)}>
          Susun dengan AI
        </button>
      )}
    </div>
  );
}

// Errors block finalizing; warnings are advice.
function DesignIssues({ issues }: { issues: { level: string; where?: string; message: string }[] }) {
  if (!issues.length) return null;
  const errors = issues.filter((i) => i.level === "error").length;
  return (
    <details className={`design-issues ${errors ? "has-errors" : ""}`} open={errors > 0}>
      <summary className="small">
        {errors ? `${errors} hal wajib diperbaiki sebelum finalisasi` : "Rancangan valid"}
        {issues.length - errors > 0 && ` · ${issues.length - errors} saran`}
      </summary>
      <ul className="small">
        {issues.map((i, n) => (
          <li key={n} className={i.level === "error" ? "warn-text" : "muted"}>{i.where && <code>{i.where}</code>} {i.message}</li>
        ))}
      </ul>
    </details>
  );
}

// The design rules or layout spec edited as JSON; the server validates the
// whole document and refuses it with every problem listed.
function DesignEditor({ doc, design, onDone }: { doc: string; design: DesignView; onDone: () => void }) {
  const { load } = useProject();
  const rules = doc === DESIGN_DOCS.rules;
  const [text, setText] = useState(() => JSON.stringify((rules ? design.design?.rules : design.design?.layout) ?? {}, null, 2));
  const [errors, setErrors] = useState<ErrorDetail[]>([]);
  const [busy, setBusy] = useState(false);
  const save = async () => {
    let data: unknown;
    try {
      data = JSON.parse(text);
    } catch (e) {
      setErrors([{ message: `JSON tidak valid: ${(e as Error).message}` }]);
      return;
    }
    setBusy(true);
    try {
      await (rules ? api.editDesignRules(data as never) : api.editLayout(data as never));
      await load();
      onDone();
    } catch (e) {
      setErrors(e instanceof ApiError && e.details.length ? e.details : [{ message: (e as Error).message }]);
    } finally {
      setBusy(false);
    }
  };
  const v = design.vocabulary;
  return (
    <div className="doc-edit">
      <p className="small muted">
        {rules
          ? "Token wajib: colors, typography, spacing. Wajib diisi: principles, ux_guidelines, accessibility, responsive."
          : `Pattern: ${v.patterns.join(", ")}. State: ${v.states.join(", ")}. Id: huruf kecil, angka, tanda minus.`}
      </p>
      <textarea className="mono" rows={24} value={text} spellCheck={false} onChange={(e) => setText(e.target.value)} />
      {errors.length > 0 && (
        <ul className="small warn-text">{errors.map((e, i) => <li key={i}>{e.field && <code>{e.field}</code>} {e.message}</li>)}</ul>
      )}
      <div className="row">
        <button className="primary" disabled={busy} onClick={() => void save()}>Simpan</button>
        <button className="ghost" onClick={onDone}>Batal</button>
      </div>
    </div>
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
