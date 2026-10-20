import { useEffect, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { api, ApiError, type ErrorDetail } from "../api";
import {
  AREA_CLASS, AREA_LABEL, ARTIFACT_STATE_CLASS, ARTIFACT_STATE_LABEL, CATEGORY_LABEL, DESIGN_DOCS, designIssuesOf, groupArtifacts,
  isDesignDoc, SCOPE_DOCS, scopeIssuesOf, usedBy,
} from "../blueprint";
import { Markdown } from "../components/Markdown";
import { PromptBar } from "../components/PromptBar";
import { isWorking } from "../flow";
import { useProject } from "../store/project";
import type { AcceptanceEdit, Artifact, ArtifactCategory, ArtifactState, BlueprintView, Decision, DesignIssue, DesignView, Flow, Project } from "../types";
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
  const id = params.get("doc") ?? "scope";
  const doc = docs.find((d) => d.id === id);
  const locked = project.stage === "READY";
  const working = isWorking(project);
  const select = (next: string) => setParams({ doc: next }, { replace: true });
  // A product with screens but no design rules or layout yet.
  const designing = !!view?.has_ui && !docs.some((d) => isDesignDoc(d.id));
  const designDoc = !!doc && isDesignDoc(doc.id);
  // Areas still to write or fix, and critical decisions still open.
  const todo = (view?.areas ?? []).filter((a) => a.status === "missing" || a.status === "invalid").length
    + (view?.decisions ?? []).filter((d) => d.critical && !d.resolution).length;

  return (
    <div className="ws">
      <nav className="ws-nav" aria-label="Artefak blueprint">
        <div className="ws-nav-head">
          <strong>Blueprint</strong>
          <span className="muted small">{docs.length} artefak</span>
        </div>
        <div className="ws-group">
          <button className={`ws-item ${id === "scope" ? "on" : ""}`} aria-current={id === "scope" ? "page" : undefined} onClick={() => select("scope")}>
            <span className="ws-item-text">Cakupan blueprint</span>
            {todo > 0 && <span className="ws-count" title="Area belum lengkap atau keputusan kritis terbuka">{todo}</span>}
          </button>
        </div>
        {groupArtifacts(docs).map(([cat, ds]) => (
          <div key={cat} className="ws-group">
            <div className="ws-group-label">{CATEGORY_LABEL[cat]}</div>
            {ds.map((d) => (
              <button key={d.id} className={`ws-item ${d.id === id ? "on" : ""}`} aria-current={d.id === id ? "page" : undefined} onClick={() => select(d.id)}>
                <span className="ws-item-text">{d.title}</span>
                <StateDot state={view?.states[d.id]} />
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
          : id === "scope" ? view && <ScopePanel project={project} view={view} locked={locked} onOpen={select} />
          : id === "design" && designing ? <DesignEmpty project={project} locked={locked} />
          : doc ? <ArtifactDoc key={doc.id} project={project} doc={doc} locked={locked} design={design} view={view} onOpen={select} />
          : (id === SCOPE_DOCS.useCases || id === SCOPE_DOCS.acceptance) && !locked
            ? <div className="doc"><h1>{id === SCOPE_DOCS.useCases ? "Alur Bisnis & Use Case" : "Acceptance Criteria & Definition of Done"}</h1>
                <ScopeEditor doc={id} project={project} onDone={() => select("scope")} /></div>
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

function ArtifactDoc({ project, doc, locked, design, view, onOpen }: {
  project: Project; doc: Artifact; locked: boolean; design: DesignView | null; view: BlueprintView | null; onOpen: (id: string) => void;
}) {
  const { run, pending } = useProject();
  const [editing, setEditing] = useState(false);
  const used = usedBy(project.workflow, doc.id);
  const designDoc = isDesignDoc(doc.id);
  const scopeDoc = doc.id === SCOPE_DOCS.useCases || doc.id === SCOPE_DOCS.acceptance;
  const decisions = doc.id === SCOPE_DOCS.decisions;
  const state = view?.states[doc.id];
  return (
    <article className="doc">
      <header className="doc-head">
        <div className="doc-kicker">
          <span className="tag">{CATEGORY_LABEL[doc.category] ?? doc.category}</span>
          {doc.builtin ? <span className="muted small">Dari data terstruktur</span> : <span className="muted small">v{doc.version}</span>}
          {state && <span className={`state ${ARTIFACT_STATE_CLASS[state.status]}`} title={state.reason}>{ARTIFACT_STATE_LABEL[state.status]}</span>}
        </div>
        <h1>{doc.title}</h1>
        {doc.summary && <p className="doc-summary">{doc.summary}</p>}
        <div className="doc-actions">
          {(designDoc || scopeDoc || !doc.builtin) && !locked && !editing && <button onClick={() => setEditing(true)}>{designDoc || scopeDoc ? "Edit data" : "Edit"}</button>}
          {!doc.builtin && !locked && !editing && <RemoveButton id={doc.id} />}
          {used.length > 0 && (
            <span className="small muted">Dipakai oleh {used.map((c) => <Link key={c} className="ref-chip" to={`/workflow?cmd=${c}`}>{c}</Link>)}</span>
          )}
        </div>
      </header>
      {state?.status === "needs_review" && !locked && (
        <div className="notice warn">
          <span>{state.reason}. Periksa apakah isi artefak ini masih sesuai.</span>
          <button disabled={pending === "reviewArtifact"} onClick={() => void run("reviewArtifact", () => api.reviewArtifact(doc.id))}>Tandai sudah ditinjau</button>
        </div>
      )}
      {designDoc && design && <IssueList issues={designIssuesOf(design.issues, doc.id)} />}
      {(scopeDoc || decisions) && view && <IssueList issues={scopeIssuesOf(view.issues, doc.id)} />}
      {editing
        ? designDoc && design ? <DesignEditor doc={doc.id} design={design} onDone={() => setEditing(false)} />
          : scopeDoc ? <ScopeEditor doc={doc.id} project={project} onDone={() => setEditing(false)} />
          : <ArtifactEditor doc={doc} onDone={() => setEditing(false)} />
        : decisions && !locked ? <DecisionList decisions={view?.decisions ?? []} />
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
function IssueList({ issues }: { issues: DesignIssue[] }) {
  if (!issues.length) return null;
  const errors = issues.filter((i) => i.level === "error").length;
  return (
    <details className={`design-issues ${errors ? "has-errors" : ""}`} open={errors > 0}>
      <summary className="small">
        {errors ? `${errors} hal wajib diperbaiki sebelum finalisasi` : "Valid"}
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

// The design rules or layout spec edited as JSON.
function DesignEditor({ doc, design, onDone }: { doc: string; design: DesignView; onDone: () => void }) {
  const rules = doc === DESIGN_DOCS.rules;
  const v = design.vocabulary;
  return (
    <JsonEditor
      initial={(rules ? design.design?.rules : design.design?.layout) ?? {}}
      hint={rules
        ? "Token wajib: colors, typography, spacing. Wajib diisi: principles, ux_guidelines, accessibility, responsive."
        : `Pattern: ${v.patterns.join(", ")}. State: ${v.states.join(", ")}. Id: huruf kecil, angka, tanda minus.`}
      save={(data) => (rules ? api.editDesignRules(data as never) : api.editLayout(data as never))}
      onDone={onDone}
    />
  );
}

// Use cases or acceptance criteria edited as JSON; the server refuses
// changed items that are invalid.
function ScopeEditor({ doc, project, onDone }: { doc: string; project: Project; onDone: () => void }) {
  if (doc === SCOPE_DOCS.useCases) {
    return (
      <JsonEditor
        initial={project.flows ?? []}
        hint={"Per use case wajib: role (aktor), goal, steps (alur utama), postconditions, dan alternatives atau failures. " +
          "Opsional: preconditions, business_rules, permissions, data_effects, integrations. feature_id harus ada; id kosong = use case baru."}
        save={(data) => api.editUseCases(data as Flow[])}
        onDone={onDone}
      />
    );
  }
  const initial = {
    acceptance: (project.requirements ?? []).map((r) => ({
      requirement: r.id, priority: r.priority, text: r.text, criteria: r.acceptance ?? [], untestable: r.untestable ?? "",
    })),
    definition_of_done: project.definition_of_done ?? [],
  };
  return (
    <JsonEditor
      initial={initial}
      hint={"Tiap requirement: criteria (kind: behavior, negative, edge, security, performance, compatibility, accessibility; text; evidence) " +
        "atau untestable berisi alasan. Requirement MUST sebaiknya punya negative/edge case. text dan priority hanya untuk dibaca."}
      save={(data) => {
        const d = data as { acceptance?: AcceptanceEdit[]; definition_of_done?: string[] };
        return api.editAcceptance(d.acceptance ?? [], d.definition_of_done ?? []);
      }}
      onDone={onDone}
    />
  );
}

// Structured data edited as JSON; a refusal lists every problem.
function JsonEditor({ initial, hint, save, onDone }: {
  initial: unknown; hint: string; save: (data: unknown) => Promise<unknown>; onDone: () => void;
}) {
  const { load } = useProject();
  const [text, setText] = useState(() => JSON.stringify(initial, null, 2));
  const [errors, setErrors] = useState<ErrorDetail[]>([]);
  const [busy, setBusy] = useState(false);
  const submit = async () => {
    let data: unknown;
    try {
      data = JSON.parse(text);
    } catch (e) {
      setErrors([{ message: `JSON tidak valid: ${(e as Error).message}` }]);
      return;
    }
    setBusy(true);
    try {
      await save(data);
      await load();
      onDone();
    } catch (e) {
      setErrors(e instanceof ApiError && e.details.length ? e.details : [{ message: (e as Error).message }]);
    } finally {
      setBusy(false);
    }
  };
  return (
    <div className="doc-edit">
      <p className="small muted">{hint}</p>
      <textarea className="mono" rows={24} value={text} spellCheck={false} onChange={(e) => setText(e.target.value)} />
      {errors.length > 0 && (
        <ul className="small warn-text">{errors.map((e, i) => <li key={i}>{e.field && <code>{e.field}</code>} {e.message}</li>)}</ul>
      )}
      <div className="row">
        <button className="primary" disabled={busy} onClick={() => void submit()}>Simpan</button>
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

function StateDot({ state }: { state?: ArtifactState }) {
  if (!state || state.status === "valid" || state.status === "finalized") return null;
  return <span className={`cmd-dot ${ARTIFACT_STATE_CLASS[state.status]}`} title={`${ARTIFACT_STATE_LABEL[state.status]}${state.reason ? `: ${state.reason}` : ""}`} />;
}

// What the blueprint covers: every area with its status, the reason an
// area is left out, and the critical decisions the user still has to make.
function ScopePanel({ project, view, locked, onOpen }: { project: Project; view: BlueprintView; locked: boolean; onOpen: (id: string) => void }) {
  const { run, pending } = useProject();
  const [skipping, setSkipping] = useState("");
  const [reason, setReason] = useState("");
  const busy = project.job?.kind === "blueprint" && isWorking(project);
  const open = view.areas.filter((a) => a.id !== "ui" && (a.status === "missing" || a.status === "invalid")).length;
  const critical = view.decisions.filter((d) => d.critical && !d.resolution);
  const skip = async (area: string) => {
    if (await run("skipArea", () => api.skipArea(area, reason))) {
      setSkipping("");
      setReason("");
    }
  };
  return (
    <article className="doc">
      <header className="doc-head">
        <h1>Cakupan blueprint</h1>
        <p className="doc-summary">
          Area yang relevan harus ditulis, atau dilewati dengan alasan jelas. Area yang tidak relevan untuk project ini tidak perlu apa-apa.
        </p>
        {!locked && open > 0 && (
          <div className="doc-actions">
            {busy ? <span><span className="spinner" /> {project.job?.message}…</span> : (
              <button className="primary" disabled={isWorking(project) || pending === "completeScope"} onClick={() => void run("completeScope", api.completeScope)}>
                Lengkapi {open} area dengan AI
              </button>
            )}
          </div>
        )}
      </header>
      {critical.length > 0 && (
        <div className="notice bad">
          <strong>{critical.length} keputusan kritis menunggu keputusan Anda</strong>
          <ul>{critical.map((d) => <li key={d.id}><button className="link" onClick={() => onOpen(SCOPE_DOCS.decisions)}>{d.id}</button> {d.text}</li>)}</ul>
        </div>
      )}
      <ul className="scope-list">
        {view.areas.map((a) => (
          <li key={a.id} className="scope-item">
            <div className="scope-row">
              <strong>{a.title}</strong>
              <span className={`state ${AREA_CLASS[a.status]}`}>{AREA_LABEL[a.status]}</span>
            </div>
            <div className="small muted">
              {a.reason ?? (a.errors ? `${a.errors} hal wajib diperbaiki` : null)}
              {a.artifacts.map((d) => <button key={d} className="ref-chip" onClick={() => onOpen(d)}>{d}</button>)}
            </div>
            {!locked && (
              <div className="row small">
                {a.id === "ui" && a.status === "missing" && <button className="ghost" onClick={() => onOpen("design")}>Susun rancangan UI</button>}
                {(a.id === SCOPE_DOCS.useCases || a.id === SCOPE_DOCS.acceptance) && a.status !== "skipped" && a.status !== "not_applicable" && (
                  <button className="ghost" onClick={() => onOpen(a.id)}>Edit data</button>
                )}
                {a.skippable && (a.status === "missing" || a.status === "invalid") && skipping !== a.id && (
                  <button className="ghost" onClick={() => { setSkipping(a.id); setReason(""); }}>Lewati…</button>
                )}
                {a.status === "skipped" && (
                  <button className="ghost" disabled={pending === "unskipArea"} onClick={() => void run("unskipArea", () => api.unskipArea(a.id))}>Cakup lagi</button>
                )}
              </div>
            )}
            {skipping === a.id && (
              <div className="row">
                <input autoFocus placeholder="Alasan area ini tidak diperlukan" value={reason} onChange={(e) => setReason(e.target.value)} />
                <button disabled={!reason.trim() || pending === "skipArea"} onClick={() => void skip(a.id)}>Lewati</button>
                <button className="ghost" onClick={() => setSkipping("")}>Batal</button>
              </div>
            )}
          </li>
        ))}
      </ul>
    </article>
  );
}

const DECISION_LABEL: Record<Decision["kind"], string> = { assumption: "Asumsi", question: "Pertanyaan", conflict: "Konflik", risk: "Risiko" };

// The decision register: the user decides open items; a decided one can
// be opened again.
function DecisionList({ decisions }: { decisions: Decision[] }) {
  if (!decisions.length) return <p className="muted">Belum ada asumsi, pertanyaan, konflik, atau risiko yang tercatat.</p>;
  return (
    <ul className="scope-list">
      {decisions.map((d) => <DecisionItem key={d.id} d={d} />)}
    </ul>
  );
}

function DecisionItem({ d }: { d: Decision }) {
  const { run, pending } = useProject();
  const [text, setText] = useState("");
  const key = `decide-${d.id}`;
  return (
    <li className="scope-item">
      <div className="scope-row">
        <span><code>{d.id}</code> <span className="tag">{DECISION_LABEL[d.kind] ?? d.kind}</span>{d.critical && <span className="state st-issue">kritis</span>}</span>
        {d.area && <span className="muted small">area {d.area}</span>}
      </div>
      <p>{d.text}</p>
      {d.resolution ? (
        <div className="row small">
          <span><strong>Keputusan:</strong> {d.resolution}</span>
          <button className="ghost" disabled={pending === key} onClick={() => void run(key, () => api.resolveDecision(d.id, ""))}>Buka lagi</button>
        </div>
      ) : (
        <div className="row">
          <input placeholder={d.kind === "risk" ? "Mitigasi atau penerimaan risiko" : "Keputusan Anda"} value={text} onChange={(e) => setText(e.target.value)} />
          <button disabled={!text.trim() || pending === key} onClick={() => void run(key, () => api.resolveDecision(d.id, text))}>Putuskan</button>
        </div>
      )}
    </li>
  );
}
