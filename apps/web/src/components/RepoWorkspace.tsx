import { useEffect, useMemo, useState } from "react";
import { api } from "../api";
import { isWorking } from "../flow";
import {
  ancestors, baseName, buildTree, issueCounts, modulesAt, nodeKind, OWNER_LABEL, SECTION, SOURCE_AREA, STATUS_LABEL,
  usedBy, visibleRows,
} from "../repo";
import { structureState } from "../structure";
import { useProject } from "../store/project";
import type { Project, RepoNode, RepoPlan, Structure } from "../types";
import { Elapsed } from "./Elapsed";
import { StructurePanel } from "./StructurePanel";

// Structure workspace: decide the shape, then the folder plan. One page,
// two steps — approve the decision, then generate, revise, and approve folders.
export function RepoWorkspace({ project }: { project: Project }) {
  const s = project.structure as Structure;

  if (s.mode === "existing") {
    return <div className="repo"><h1>Struktur repository</h1><p className="muted">Project yang sudah ada mengikuti struktur kodenya; rencana folder hanya untuk project baru.</p></div>;
  }
  if (!s.ir) {
    return <div className="repo"><h1>Struktur repository</h1><p className="muted">Keputusan struktur belum ada. Muncul setelah platform dan teknologi dipilih.</p></div>;
  }

  return (
    <div className="repo">
      <header className="repo-head">
        <div>
          <h1>Struktur repository</h1>
          <p className="small muted">Bentuk struktur dulu, lalu rencana folder dan tanggung jawabnya.</p>
        </div>
      </header>
      <StructurePanel project={project} />
      <RepoPlan project={project} />
    </div>
  );
}

function RepoPlan({ project }: { project: Project }) {
  const { run, pending } = useProject();
  const [selected, setSelected] = useState("");
  const s = project.structure as Structure;
  const plan = s.repo;
  const finalized = project.stage === "READY";
  const busy = isWorking(project) || pending === "repo" || !!s.repo_request || !!s.request || !!s.plan_request;
  const editable = !finalized && !busy && !s.proposal;
  const structureApproved = structureState(s) === "approved";

  const working = !!s.repo_request && (
    <p className="small muted"><span className="spinner" /> {s.repo_request.action === "generate" ? "AI merancang folder…" : "AI merevisi rencana folder…"} <Elapsed started={project.job?.started} /></p>
  );

  if (!plan) {
    return (
      <section className="info-sec">
        <div className="tech-title"><h2>Rencana folder</h2></div>
        {working}
        {structureApproved
          ? !s.repo_request && <p className="small muted">Belum ada. Tekan <strong>Susun rancangan</strong> — folder dirancang dari keputusan struktur, dalam satu proses bersama rancangan lain.</p>
          : <p className="small muted">Setujui keputusan struktur di atas dulu. Rencana folder merinci struktur yang disetujui.</p>}
        {s.repo_request?.rejected && <p className="small warn-text">Jawaban AI sebelumnya ditolak: {s.repo_request.rejected}</p>}
      </section>
    );
  }

  const counts = issueCounts(plan);
  const issues = plan.analysis.issues ?? [];
  const ver = plan.history?.at(-1)?.version;
  const staleEval = plan.evaluation.outdated && !plan.stale;

  return (
    <section className="info-sec">
      <div className="tech-title">
        <h2>Rencana folder</h2>
        <span className={`tech-status ${plan.status}`}>{STATUS_LABEL[plan.status]}{ver ? ` · v${ver}` : ""}</span>
        <span className="tag">Skor {plan.evaluation.score}</span>
        {plan.stale && <span className="tag warn">Kedaluwarsa</span>}
        {counts.errors > 0 && <span className="tag bad">{counts.errors} error</span>}
      </div>
      <p className="small muted">{plan.approach} · {plan.analysis.dirs} folder, {plan.analysis.files} file</p>

      {working}
      {s.repo_request?.rejected && <p className="small warn-text">Jawaban AI ditolak, mencoba lagi: {s.repo_request.rejected}</p>}

      {plan.stale && (
        <div className="repo-banner warn">
          <strong>Sumber berubah setelah rencana ditulis</strong>
          <div className="small">
            {(plan.analysis.stale_areas ?? []).map((x) => SOURCE_AREA[x] ?? x).join(", ") || "sumber blueprint"}
            {!!plan.analysis.affected?.length && <> · terdampak: {plan.analysis.affected.map((x) => SECTION[x] ?? x).join(", ")}</>}
          </div>
          {editable && <button disabled={pending === "repo"} onClick={() => void run("repo", () => api.reviseRepo(""))}>Revisi bagian terdampak dengan AI</button>}
        </div>
      )}

      <p className="repo-lede">{plan.summary}</p>
      {plan.topology && <p className="small muted">{plan.topology}</p>}

      <Tree plan={plan} selected={selected} onSelect={setSelected} />

      {plan.modules.length > 0 && (
        <section className="repo-mods">
          <h3>Modul</h3>
          <ul>
            {plan.modules.map((m) => (
              <li key={m.module}>
                <strong>{m.module}</strong>
                {m.locations?.length
                  ? m.locations.map((l) => (
                    <button key={l.path} className="link" onClick={() => setSelected(l.path)}>{l.path}{l.role ? ` (${l.role})` : ""}</button>
                  ))
                  : <span className="muted"> — belum dipetakan</span>}
              </li>
            ))}
          </ul>
        </section>
      )}

      {issues.length > 0 && (
        <ul className="tech-issues small">{issues.map((i, k) => <li key={k} className={i.level}>{i.level === "error" ? "✕" : "!"} {i.message}</li>)}</ul>
      )}

      <section className="repo-foot">
        {plan.status === "approved"
          ? <p className="ok-text small">✓ Rencana folder{ver ? ` v${ver}` : ""} disetujui dan masuk ekspor untuk coding agent.</p>
          : (
            <div className="repo-actions">
              <button className="primary" disabled={!editable || pending === "repo" || counts.errors > 0 || plan.stale || !structureApproved}
                title={!structureApproved ? "Setujui keputusan struktur di atas dulu" : plan.stale ? "Revisi bagian yang terdampak dulu" : counts.errors > 0 ? "Selesaikan error dulu" : undefined}
                onClick={() => void run("repo", api.approveRepo)}>Setujui rencana folder</button>
              {!structureApproved && <span className="small muted">Setujui keputusan struktur di atas dulu.</span>}
            </div>
          )}
        {plan.status === "changed" && <p className="small warn-text">Berubah setelah disetujui. Setujui lagi sebelum finalisasi.</p>}
        {editable && staleEval && (
          <div className="repo-actions">
            <button disabled={pending === "repo"} onClick={() => void run("repo", () => api.reviseRepo(""))}>Evaluasi ulang</button>
          </div>
        )}
        {editable && <RegenDiscard />}
      </section>

      <More plan={plan} onOpen={setSelected} />
    </section>
  );
}

function Tree({ plan, selected, onSelect }: { plan: RepoPlan; selected: string; onSelect: (p: string) => void }) {
  const roots = useMemo(() => buildTree(plan.nodes), [plan.nodes]);
  const [expanded, setExpanded] = useState<Set<string>>(() => new Set(roots.filter((r) => r.node.type === "dir").map((r) => r.node.path)));
  const [query, setQuery] = useState("");
  useEffect(() => {
    if (!selected) return;
    setExpanded((cur) => new Set([...cur, ...ancestors(selected)]));
  }, [selected]);
  const rows = visibleRows(roots, expanded, query);
  const firstDir = plan.nodes.find((n) => n.type === "dir");
  const shown = plan.nodes.find((n) => n.path === selected) ?? (selected ? undefined : firstDir ?? roots[0]?.node);
  const toggle = (path: string) => setExpanded((cur) => { const n = new Set(cur); if (n.has(path)) n.delete(path); else n.add(path); return n; });
  const select = (path: string) => { onSelect(path); setExpanded((cur) => new Set([...cur, ...ancestors(path)])); };
  return (
    <div className="repo-split">
      <div className="repo-tree">
        <input type="search" value={query} placeholder="Cari folder…" aria-label="Cari di folder tree" onChange={(e) => setQuery(e.target.value)} />
        <div className="repo-rows" role="tree">
          {rows.length === 0 && <p className="muted small">Tidak ada yang cocok.</p>}
          {rows.map((t) => {
            const isDir = t.node.type === "dir";
            const open = expanded.has(t.node.path);
            const on = t.node.path === (selected || shown?.path);
            return (
              <div key={t.node.path} role="treeitem" aria-expanded={isDir ? open || !!query : undefined} aria-selected={on}
                className={`repo-row ${on ? "on" : ""} ${t.node.implied ? "implied" : ""}`} style={{ paddingLeft: `${0.4 + t.depth * 0.9}rem` }}>
                <button className="repo-twist" disabled={!isDir || t.children.length === 0} aria-label={open ? "Tutup" : "Buka"} onClick={() => toggle(t.node.path)}>
                  {isDir ? (t.children.length === 0 ? "·" : open || query ? "▾" : "▸") : ""}
                </button>
                <button className="repo-name" onClick={() => select(t.node.path)}>
                  <span className={`repo-icon ${isDir ? "is-dir" : "is-file"}`} aria-hidden>{isDir ? "▣" : "▢"}</span>
                  {baseName(t.node.path)}{isDir ? "/" : ""}
                </button>
                <span className={`repo-owner o-${t.node.owner === "shared" || t.node.owner === "platform" ? t.node.owner : "module"}`}>{t.node.owner}</span>
              </div>
            );
          })}
        </div>
      </div>
      <div className="repo-detail">
        {shown ? <NodeCard plan={plan} node={shown} onSelect={select} /> : <p className="muted">Pilih folder untuk melihat tanggung jawabnya.</p>}
      </div>
    </div>
  );
}

function NodeCard({ plan, node, onSelect }: { plan: RepoPlan; node: RepoNode; onSelect: (p: string) => void }) {
  const mods = modulesAt(plan, node.path);
  const used = usedBy(plan, node.path);
  const link = (p: string) => plan.nodes.some((n) => n.path === p)
    ? <button className="link" onClick={() => onSelect(p)}>{p}</button>
    : <span>{p}</span>;
  return (
    <article>
      <header className="repo-node-head">
        <h2><code>{node.path}</code></h2>
        <span className="tag">{nodeKind(node)}</span>
        {node.implied && <span className="tag warn">belum dijelaskan</span>}
      </header>
      <p>{node.responsibility}</p>
      <p className="small muted">Owner: {OWNER_LABEL[node.owner] ?? node.owner}{node.reason ? ` · ${node.reason}` : ""}</p>
      {!!mods.length && <p className="small">Modul: {mods.join(", ")}</p>}
      {!!node.depends_on?.length && <p className="small">Bergantung pada: {node.depends_on.map((d, i) => <span key={d}>{i > 0 && ", "}{link(d)}</span>)}</p>}
      {!!used.length && <p className="small muted">Dipakai: {used.map((d, i) => <span key={d}>{i > 0 && ", "}{link(d)}</span>)}</p>}
    </article>
  );
}

function RegenDiscard() {
  const { run, pending } = useProject();
  const [again, setAgain] = useState("");
  const [confirm, setConfirm] = useState(false);
  return (
    <div className="repo-actions">
      <input value={again} maxLength={6000} placeholder="Rancang ulang, mis. “satu app, tanpa packages/”" onChange={(e) => setAgain(e.target.value)} />
      <button disabled={pending === "repo" || !again.trim()} onClick={async () => { if (await run("repo", () => api.generateRepo(again.trim()))) setAgain(""); }}>Rancang ulang</button>
      {confirm
        ? <><button className="bad" disabled={pending === "repo"} onClick={() => void run("repo", api.discardRepo)}>Ya, hapus</button><button className="ghost" onClick={() => setConfirm(false)}>Batal</button></>
        : <button className="ghost" onClick={() => setConfirm(true)}>Hapus rencana</button>}
    </div>
  );
}

function More({ plan, onOpen }: { plan: RepoPlan; onOpen: (p: string) => void }) {
  const rules = plan.rules ?? [];
  const versions = [...(plan.history ?? [])].reverse();
  const deps = plan.dependency;
  if (!rules.length && !versions.length && !deps.direction) return null;
  return (
    <details className="repo-more small">
      <summary>Selengkapnya</summary>
      {deps.direction && <p><strong>Arah dependency:</strong> {deps.direction}</p>}
      {!!deps.forbidden?.length && (
        <p>Dilarang: {deps.forbidden.map((r) => `${r.from} → ${r.to}`).join("; ")}</p>
      )}
      {rules.length > 0 && <ol>{rules.map((r) => <li key={r}>{r}</li>)}</ol>}
      {versions.length > 0 && (
        <ul className="repo-history">{versions.map((v, i) => (
          <li key={v.version}>
            <strong>v{v.version}</strong> {i === 0 && <span className="tag">saat ini</span>} · {STATUS_LABEL[v.status] ?? v.status} · skor {v.score}
            {v.request && <div className="muted">“{v.request}”</div>}
          </li>
        ))}</ul>
      )}
      {!!plan.decisions?.length && plan.decisions.map((d) => (
        <p key={d.title}><strong>{d.title}:</strong> {d.choice} — {d.reason}</p>
      ))}
      {!!plan.evaluation.findings?.length && (
        <ul>{plan.evaluation.findings.map((f, i) => (
          <li key={i}>{f.message}{f.path && <> · <button className="link" onClick={() => onOpen(f.path!)}>{f.path}</button></>}</li>
        ))}</ul>
      )}
    </details>
  );
}
