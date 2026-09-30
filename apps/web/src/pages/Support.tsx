import { useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { api } from "../api";
import { useAI } from "../store/ai";
import { useProject } from "../store/project";
import { coverage, escalationDraft, escalationText, suggestions, type EscalationDraft } from "../support";
import type { Page, Project, SupportAnswer, SupportFlow, SupportMessage, Troubleshoot } from "../types";

const KIND_LABEL: Record<SupportAnswer["kind"], string> = {
  explain: "Penjelasan", flow: "Alur proses", troubleshoot: "Troubleshooting", roles: "Role & akses", unknown: "Info belum tersedia",
};

// Technical Support Assistant: Ask → Understand → Show Flow → Explain →
// Troubleshoot → Resolve / Escalate. Answers come from the project only.
export function SupportPage() {
  const { project, run, pending } = useProject();
  const mcp = useAI((s) => s.state?.connection?.mode === "mcp");
  const [text, setText] = useState("");
  const [escalating, setEscalating] = useState(false);
  const end = useRef<HTMLDivElement>(null);
  const ms = project?.support?.messages ?? [];
  const last = ms[ms.length - 1];
  useEffect(() => end.current?.scrollIntoView({ block: "end" }), [ms.length, last?.status]);
  if (!project) return null;

  const answering = last?.status === "running" || last?.status === "waiting";
  const ask = async (q: string) => {
    if (!q.trim() || answering) return;
    if (await run("ask", () => api.askSupport(q))) setText("");
  };

  return (
    <div className="sp">
      <header className="sp-head">
        <div className="sp-title">
          <strong>Technical Support Assistant</strong>
          <Understanding project={project} />
        </div>
        <button disabled={ms.length === 0 || answering || pending === "clear"} onClick={() => run("clear", api.clearSupport)}>Percakapan baru</button>
        <button disabled={!project.discovery} onClick={() => setEscalating(true)}>
          Eskalasi{project.support?.escalations?.length ? ` (${project.support.escalations.length})` : ""}
        </button>
      </header>

      <div className="sp-body">
        <div className="sp-col">
          {!project.discovery ? (
            <section className="sp-welcome">
              <h2>Sistem belum dipahami</h2>
              <p className="muted">Asisten menjawab dari fitur, alur, dan aturan project ini. Ceritakan idenya dulu di <Link to="/simple/tell">Ceritakan</Link>.</p>
            </section>
          ) : ms.length === 0 ? (
            <section className="sp-welcome">
              <h2>Tanya apa saja tentang {project.name}</h2>
              <p className="muted">Jawaban disusun dari fitur, alur, role, aturan, module, dan API yang ada di sistem ini. Yang belum ada akan disebut belum tersedia.</p>
              <div className="sp-suggest">
                {suggestions(project).map((q) => <button key={q} onClick={() => void ask(q)} disabled={pending === "ask"}>{q}</button>)}
              </div>
            </section>
          ) : (
            ms.map((m, i) => <Message key={m.id} m={m} last={i === ms.length - 1} project={project} mcp={mcp}
              onAsk={ask} onRetry={() => run("retry", api.retrySupport)} onEscalate={() => setEscalating(true)} />)
          )}
          <div ref={end} />
        </div>
      </div>

      <form className="sp-compose" onSubmit={(e) => { e.preventDefault(); void ask(text); }}>
        <div className="sp-col sp-compose-row">
          <label className="sr-only" htmlFor="sp-q">Pertanyaan</label>
          <textarea id="sp-q" rows={2} value={text} maxLength={2000} disabled={!project.discovery}
            placeholder="Tanyakan proses, role, atau masalah — mis. “Kenapa order saya pending?”"
            onChange={(e) => setText(e.target.value)}
            onKeyDown={(e) => { if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); void ask(text); } }} />
          <button className="primary" type="submit" disabled={!text.trim() || answering || pending === "ask"}>Kirim</button>
        </div>
      </form>

      {escalating && <EscalateDialog project={project} messages={ms} onClose={() => setEscalating(false)} />}
    </div>
  );
}

function Understanding({ project }: { project: Project }) {
  const [open, setOpen] = useState(false);
  const areas = coverage(project);
  const known = areas.filter((a) => a.count > 0).length;
  return (
    <span className="sp-understand">
      <button className={`sp-badge ${known === areas.length ? "ok" : known === 0 ? "bad" : "warn"}`} aria-expanded={open} onClick={() => setOpen(!open)}>
        Pemahaman sistem {known}/{areas.length}
      </button>
      {open && (
        <ul className="sp-pop" onMouseLeave={() => setOpen(false)}>
          {areas.map((a) => (
            <li key={a.area}>
              <span>{a.area}</span>
              {a.count > 0 ? <span className="ok-text">✓ {a.count}</span> : <span className="muted">belum tersedia</span>}
            </li>
          ))}
        </ul>
      )}
    </span>
  );
}

function Message({ m, last, project, mcp, onAsk, onRetry, onEscalate }: {
  m: SupportMessage; last: boolean; project: Project; mcp: boolean;
  onAsk: (q: string) => void; onRetry: () => void; onEscalate: () => void;
}) {
  if (m.role === "user") return <div className="sp-msg user"><div className="sp-bubble">{m.text}</div></div>;
  if (m.status === "running" || m.status === "waiting") {
    return (
      <div className="sp-msg ai" aria-live="polite">
        <span className="spinner" /> Menelusuri alur, role, dan aturan sistem…
        {m.status === "waiting" && mcp && <div className="muted small">Di AI Host kamu, minta: <code>Lanjutkan pekerjaan Foundry (panggil next_work)</code></div>}
      </div>
    );
  }
  if (m.status === "failed" || !m.answer) {
    return (
      <div className="sp-msg ai">
        <p className="error-text">Jawaban gagal dibuat. <span className="muted small">{m.error}</span></p>
        {last && <button onClick={onRetry}>Coba lagi</button>}
      </div>
    );
  }
  const a = m.answer;
  return (
    <div className="sp-msg ai">
      <span className="sp-kind">{KIND_LABEL[a.kind] ?? a.kind}</span>
      <p className="pre sp-summary">{a.summary}</p>
      {a.trouble && <TroubleCard t={a.trouble} />}
      {a.flow && <FlowView flow={a.flow} pages={project.design?.pages ?? []} />}
      {(a.roles ?? []).length > 0 && (
        <section className="sp-card">
          <h4>Role & akses</h4>
          <ul className="sp-roles">
            {a.roles!.map((r, i) => (
              <li key={i}><span className="tag">{r.role}</span> {r.access}{r.source && <span className="muted small"> · {r.source}</span>}</li>
            ))}
          </ul>
        </section>
      )}
      {(a.missing ?? []).length > 0 && (
        <section className="sp-card sp-missing">
          <h4>Belum tersedia di sistem</h4>
          <ul>{a.missing!.map((x, i) => <li key={i}>{x}</li>)}</ul>
        </section>
      )}
      {(a.sources ?? []).length > 0 && (
        <details className="sp-sources">
          <summary>Sumber ({a.sources!.length})</summary>
          {a.sources!.map((s) => <code key={s}>{s}</code>)}
        </details>
      )}
      {(last && ((a.follow_ups ?? []).length > 0 || a.escalate)) && (
        <div className="sp-next">
          {(a.follow_ups ?? []).map((q) => <button key={q} className="chip" onClick={() => onAsk(q)}>{q}</button>)}
          {a.escalate && <button onClick={onEscalate}>Eskalasi ke Tim Teknis</button>}
        </div>
      )}
    </div>
  );
}

function TroubleCard({ t }: { t: Troubleshoot }) {
  const unknown = <span className="muted">belum diketahui</span>;
  return (
    <section className="sp-card">
      <dl className="sp-facts">
        <dt>Masalah</dt><dd>{t.problem}</dd>
        <dt>Status saat ini</dt><dd>{t.status || unknown}</dd>
        <dt>Module terdampak</dt><dd>{t.module || unknown}</dd>
        <dt>Langkah alur saat ini</dt><dd>{t.step || unknown}</dd>
      </dl>
      {(t.causes ?? []).length > 0 && (
        <>
          <h4>Penyebab</h4>
          <ul className="sp-causes">
            {t.causes!.map((c, i) => (
              <li key={i}>
                <span className={`sp-badge ${c.confirmed ? "bad" : "warn"}`}>{c.confirmed ? "Terkonfirmasi" : "Kemungkinan"}</span> {c.text}
                {c.evidence && <div className="muted small">Bukti: {c.evidence}</div>}
              </li>
            ))}
          </ul>
        </>
      )}
      {(t.actions ?? []).length > 0 && (
        <>
          <h4>Tindakan yang disarankan</h4>
          <ol className="sp-actions">{t.actions!.map((x, i) => <li key={i}>{x}</li>)}</ol>
        </>
      )}
    </section>
  );
}

function FlowView({ flow, pages }: { flow: SupportFlow; pages: Page[] }) {
  const [open, setOpen] = useState<number | null>(null);
  return (
    <section className="sp-card">
      <h4>{flow.name || "Alur"}{flow.flow_id && <span className="muted small"> · {flow.flow_id}</span>}</h4>
      <ol className="sp-flow">
        {flow.steps.map((s, i) => {
          const page = pages.find((p) => p.id === s.page_id);
          const more = !!(s.detail || s.role || s.module || page || s.api);
          return (
            <li key={i} className={s.current ? "current" : ""}>
              <button className="sp-step" disabled={!more} aria-expanded={more ? open === i : undefined} onClick={() => setOpen(open === i ? null : i)}>
                <span className="sp-num">{i + 1}</span>
                <span className="grow">{s.name}</span>
                {s.current && <span className="sp-badge warn">Di sini</span>}
                {more && <span className="muted">{open === i ? "▾" : "▸"}</span>}
              </button>
              {open === i && (
                <dl className="sp-facts sp-step-detail">
                  {s.detail && <><dt>Yang terjadi</dt><dd>{s.detail}</dd></>}
                  {s.role && <><dt>Dilakukan oleh</dt><dd>{s.role}</dd></>}
                  {s.module && <><dt>Module</dt><dd>{s.module}</dd></>}
                  {page && <><dt>Halaman</dt><dd>{page.name}{page.route && <span className="muted"> ({page.route})</span>}</dd></>}
                  {s.api && <><dt>API</dt><dd><code>{s.api}</code></dd></>}
                </dl>
              )}
            </li>
          );
        })}
      </ol>
    </section>
  );
}

function EscalateDialog({ project, messages, onClose }: { project: Project; messages: SupportMessage[]; onClose: () => void }) {
  const { run, pending } = useProject();
  const [d, setD] = useState<EscalationDraft>(() => escalationDraft(messages));
  const [copied, setCopied] = useState(false);
  const [sent, setSent] = useState<string | null>(null);
  const past = project.support?.escalations ?? [];
  const lines = (k: "evidence" | "causes" | "investigation") => (d[k] ?? []).join("\n");
  const setLines = (k: "evidence" | "causes" | "investigation", v: string) => setD({ ...d, [k]: v.split("\n") });
  const send = async () => {
    let id = "";
    if (await run("escalate", async () => { id = (await api.escalate(d)).id; })) setSent(id);
  };
  const copy = async () => {
    await navigator.clipboard?.writeText(escalationText(d)).catch(() => undefined);
    setCopied(true);
  };

  return (
    <div className="modal-backdrop" onMouseDown={(e) => e.target === e.currentTarget && onClose()}>
      <div className="modal sp-esc" role="dialog" aria-modal="true" aria-labelledby="sp-esc-title">
        <div className="modal-title" id="sp-esc-title">Eskalasi ke Tim Teknis</div>
        {sent ? (
          <>
            <p><span className="sp-badge ok">{sent}</span> Eskalasi tercatat di project. Salin ringkasannya untuk dikirim ke tim teknis.</p>
            <div className="modal-actions">
              <button onClick={() => void copy()}>{copied ? "Tersalin" : "Salin ringkasan"}</button>
              <button className="primary" onClick={onClose}>Selesai</button>
            </div>
          </>
        ) : (
          <>
            <p className="muted small">Periksa ringkasan sebelum dikirim. Satu baris untuk satu poin.</p>
            <label className="field">Masalah<input value={d.issue} onChange={(e) => setD({ ...d, issue: e.target.value })} /></label>
            <div className="sp-esc-grid">
              <label className="field">Module terdampak<input value={d.module} onChange={(e) => setD({ ...d, module: e.target.value })} /></label>
              <label className="field">Alur saat ini<input value={d.flow} onChange={(e) => setD({ ...d, flow: e.target.value })} /></label>
            </div>
            <label className="field">Status saat ini<input value={d.status} onChange={(e) => setD({ ...d, status: e.target.value })} /></label>
            <label className="field">Bukti<textarea rows={3} value={lines("evidence")} onChange={(e) => setLines("evidence", e.target.value)} /></label>
            <label className="field">Kemungkinan penyebab<textarea rows={3} value={lines("causes")} onChange={(e) => setLines("causes", e.target.value)} /></label>
            <label className="field">Investigasi yang disarankan<textarea rows={3} value={lines("investigation")} onChange={(e) => setLines("investigation", e.target.value)} /></label>
            <div className="modal-actions">
              <button className="ghost" onClick={onClose}>Batal</button>
              <button onClick={() => void copy()}>{copied ? "Tersalin" : "Salin ringkasan"}</button>
              <button className="primary" disabled={!d.issue.trim() || pending === "escalate"} onClick={() => void send()}>Kirim eskalasi</button>
            </div>
          </>
        )}
        {past.length > 0 && (
          <details className="sp-sources">
            <summary>Eskalasi sebelumnya ({past.length})</summary>
            <ul className="sp-roles">
              {past.slice().reverse().map((e) => <li key={e.id}><span className="tag">{e.id}</span> {e.issue} <span className="muted small">· {new Date(e.at).toLocaleString()}</span></li>)}
            </ul>
          </details>
        )}
      </div>
    </div>
  );
}
