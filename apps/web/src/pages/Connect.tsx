import { useEffect, useRef, useState, type ReactNode } from "react";
import { api } from "../api";
import { useNavigate } from "react-router-dom";
import { Icon } from "../components/Icon";
import { ModelPicker } from "../components/ModelPicker";
import { stepFor } from "../flow";
import { agentName, agentOptions, useAI } from "../store/ai";
import { useProject } from "../store/project";
import type { AgentInfo } from "../types";

const MARK: Record<string, string> = {
  claude: "CC",
  codex: "CX",
  qwen: "QW",
  grok: "GK",
  gemini: "GM",
  opencode: "OC",
};

function AgentMark({ id, name }: { id: string; name: string }) {
  return (
    <span className={`agent-mark agent-mark-${id}`} aria-hidden>
      {MARK[id] ?? name.slice(0, 2).toUpperCase()}
    </span>
  );
}

export function ConnectPage() {
  const { state, connecting, error, connectAgent, connectMCP, disconnect, load } = useAI();
  const project = useProject((s) => s.project);
  const nav = useNavigate();
  const [custom, setCustom] = useState("");
  const waitingMCP = state?.connection?.mode === "mcp" && !state.ready;
  const noneDetected = !!state && !state.agents.some((a) => a.detected);
  const [showMCP, setShowMCP] = useState(waitingMCP || noneDetected);
  const [showCustom, setShowCustom] = useState(false);
  const [picked, setPicked] = useState<string | null>(null);
  const [model, setModel] = useState("");
  const [effort, setEffort] = useState("");
  const [copied, setCopied] = useState(false);
  const mcpPre = useRef<HTMLPreElement>(null);
  const chosen = state?.agents.find((a) => a.id === picked);
  const [models, setModels] = useState<{ agent: string; list: string[] | null } | null>(null);

  useEffect(() => {
    if (!picked) return;
    let live = true;
    api.models(picked).then((r) => { if (live) setModels({ agent: picked, list: r.models ?? [] }); }).catch(() => { if (live) setModels({ agent: picked, list: null }); });
    return () => { live = false; };
  }, [picked]);

  useEffect(() => {
    if (picked || showMCP || waitingMCP || !state) return;
    const found = state.agents.filter((a) => a.detected);
    // OpenCode is the default choice; otherwise a single detected agent picks itself.
    const preferred = found.find((a) => a.id === "opencode") ?? (found.length === 1 ? found[0] : undefined);
    if (preferred) setPicked(preferred.id);
  }, [state, picked, showMCP, waitingMCP]);

  const loadingModels = !!picked && models?.agent !== picked;
  const pick = (id: string) => {
    if (picked === id) { setPicked(null); return; }
    setPicked(id);
    setModel("");
    setEffort("");
    setShowMCP(false);
  };

  const copyMCP = async () => {
    const text = JSON.stringify(state?.mcp_config, null, 2);
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1600);
    } catch {
      const pre = mcpPre.current;
      if (!pre) return;
      const range = document.createRange();
      range.selectNodeContents(pre);
      const sel = window.getSelection();
      sel?.removeAllRanges();
      sel?.addRange(range);
    }
  };

  if (!state) {
    return (
      <section className="connect">
        <header className="connect-head">
          <h1>Hubungkan AI</h1>
          <p className="muted">{error ?? "Gagal memuat daftar agent."}</p>
        </header>
        <button className="primary" onClick={() => void load()}>Coba lagi</button>
      </section>
    );
  }

  if (state.ready) {
    const connected = state.agents.find((a) => a.id === state.connection?.agent_id);
    return (
      <section className="connect">
        <header className="connect-head">
          <h1>AI siap digunakan</h1>
          <p className="muted">Foundry memakai agent ini untuk memahami prompt dan menyusun rancangan aplikasi.</p>
        </header>
        <div className="connect-list">
          <div className="connect-item on">
            <div className="connect-row">
              <AgentMark id={connected?.id ?? state.connection?.mode ?? "host"} name={agentName(state)} />
              <span className="agent-meta">
                <strong>{agentName(state)}</strong>
                {agentOptions(state) && <span className="agent-sub">{agentOptions(state)}</span>}
              </span>
              <span className="connect-row-end">
                <span className="agent-status ok">Terhubung</span>
                <button type="button" className="ghost small" disabled={!!connecting} onClick={() => void disconnect()}>Putuskan</button>
              </span>
            </div>
          </div>
        </div>
        <div className="connect-actions">
          <button className="primary big" onClick={() => nav(project ? stepFor(project.stage).path : "/")}>Mulai</button>
          <button className="ghost" disabled={!!connecting} onClick={() => void disconnect(true)}>Putuskan semua</button>
        </div>
      </section>
    );
  }

  const mcpJSON = JSON.stringify(state.mcp_config, null, 2);

  return (
    <section className="connect">
      <header className="connect-head">
        <h1>Hubungkan AI</h1>
        <p className="muted">Foundry memakai AI untuk memahami prompt dan menyusun rancangan aplikasi.</p>
      </header>

      {noneDetected && (
        <p className="connect-empty">Tidak ada CLI AI di PATH. Pasang salah satu, atau hubungkan lewat MCP.</p>
      )}

      <ul className="connect-list">
        {state.agents.map((a) => (
          <AgentRow
            key={a.id}
            agent={a}
            picked={picked === a.id}
            connecting={connecting === a.id}
            disabled={!!connecting}
            onPick={() => pick(a.id)}
          >
            {chosen?.id === a.id && (
              <form className="connect-config" id="agent-config" onSubmit={(e) => {
                e.preventDefault();
                void connectAgent(chosen.id, undefined, { model: model.trim() || undefined, effort: effort || undefined });
              }}>
                <div className="small mp-field">Model
                  <ModelPicker models={(models?.agent === chosen.id && models.list?.length ? models.list : chosen.models) ?? []} loading={loadingModels} value={model} onChange={setModel} />
                </div>
                {!!chosen.efforts?.length && (
                  <fieldset className="effort-field">
                    <legend className="small">Effort</legend>
                    <div className="seg">
                      <button type="button" className={!effort ? "on" : ""} onClick={() => setEffort("")}>default</button>
                      {chosen.efforts.map((e) => (
                        <button type="button" key={e} className={effort === e ? "on" : ""} onClick={() => setEffort(e)}>{e}</button>
                      ))}
                    </div>
                  </fieldset>
                )}
                <div className="connect-config-foot">
                  <button type="submit" className="primary" disabled={!!connecting}>
                    {connecting === chosen.id ? <><span className="spinner" /> Menguji koneksi…</> : `Hubungkan ${chosen.name}`}
                  </button>
                </div>
              </form>
            )}
          </AgentRow>
        ))}
      </ul>

      <h2 className="connect-alt-title">Cara lain</h2>
      <div className="connect-list">
        <div className={`connect-item ${showCustom ? "on" : ""}`}>
          <button type="button" className="connect-row" aria-expanded={showCustom} onClick={() => setShowCustom((v) => !v)}>
            <span className="agent-mark agent-mark-custom" aria-hidden><Icon name="terminal" size={16} /></span>
            <span className="agent-meta">
              <strong>Agent lain</strong>
              <span className="agent-sub">Perintah sendiri, prompt lewat stdin</span>
            </span>
            <span className="connect-chevron"><Icon name="chevron" size={16} /></span>
          </button>
          {showCustom && (
            <form className="connect-config" onSubmit={(e) => {
              e.preventDefault();
              const argv = custom.trim().split(/\s+/).filter(Boolean);
              if (argv.length) void connectAgent("custom", argv);
            }}>
              <label className="small mp-field">Perintah
                <input value={custom} onChange={(e) => setCustom(e.target.value)} placeholder="mis. my-agent --headless" spellCheck={false} />
              </label>
              <div className="connect-config-foot">
                <button type="submit" className="primary" disabled={!!connecting || !custom.trim()}>Hubungkan</button>
              </div>
            </form>
          )}
        </div>

        <div className={`connect-item ${showMCP || waitingMCP ? "on" : ""}`}>
          <button type="button" className="connect-row" aria-expanded={showMCP || waitingMCP}
            onClick={() => { setShowMCP((v) => !v); setPicked(null); }}>
            <span className="agent-mark agent-mark-mcp" aria-hidden><Icon name="host" size={16} /></span>
            <span className="agent-meta">
              <strong>AI Host lewat MCP</strong>
              <span className="agent-sub">Cursor, Claude, Codex, OpenCode, Windsurf, VS Code, dan host lain</span>
            </span>
            {waitingMCP
              ? <span className="agent-status ok">Menunggu</span>
              : <span className="connect-chevron"><Icon name="chevron" size={16} /></span>}
          </button>
          {(showMCP || waitingMCP) && (
            <div className="connect-config">
              {waitingMCP
                ? <p className="connect-wait"><span className="spinner" /> Buka AI Host dan panggil tool <code>get_project</code>.</p>
                : <p className="connect-mcp-copy">Tambahkan Foundry ke konfigurasi MCP di AI Host, lalu konfirmasi.</p>}
              <div className="connect-config-foot">
                {!waitingMCP && (
                  <button className="primary" disabled={connecting === "mcp"} onClick={() => connectMCP()}>
                    {connecting === "mcp" ? <><span className="spinner" /> Menghubungkan…</> : "Sudah ditambahkan"}
                  </button>
                )}
                {waitingMCP && (
                  <button type="button" className="ghost" disabled={!!connecting} onClick={() => void disconnect()}>Putuskan</button>
                )}
              </div>
              <div className="code-head">
                <span className="muted small">mcp.json</span>
                <button type="button" className="ghost small" onClick={() => void copyMCP()}>
                  <Icon name="copy" size={14} /> {copied ? "Disalin" : "Salin"}
                </button>
              </div>
              <pre className="code" ref={mcpPre}>{mcpJSON}</pre>
            </div>
          )}
        </div>
      </div>

      {(state.connection || state.mcp) && (
        <p className="connect-refresh">
          Sisa koneksi manual (MCP / perintah sendiri)? <button type="button" className="link" disabled={!!connecting} onClick={() => void disconnect(true)}>Putuskan semua</button>
        </p>
      )}
      <p className="connect-refresh">
        Baru memasang CLI? <button type="button" className="link" onClick={() => void load()}>Periksa lagi</button>
      </p>

      {error && <p className="connect-error" role="alert">{error}</p>}
    </section>
  );
}

function AgentRow({
  agent, picked, connecting, disabled, onPick, children,
}: {
  agent: AgentInfo; picked: boolean; connecting: boolean; disabled: boolean;
  onPick: () => void; children?: ReactNode;
}) {
  if (!agent.detected) {
    return (
      <li className="connect-item off">
        <div className="connect-row">
          <AgentMark id={agent.id} name={agent.name} />
          <span className="agent-meta">
            <strong>{agent.name}</strong>
            <span className="agent-sub">{agent.binary} tidak ada di PATH</span>
          </span>
          <span className="agent-status">Belum terpasang</span>
        </div>
      </li>
    );
  }
  return (
    <li className={`connect-item ${picked ? "on" : ""}`}>
      <button
        type="button"
        className="connect-row"
        disabled={disabled}
        aria-expanded={picked}
        aria-controls={picked ? "agent-config" : undefined}
        title={agent.path}
        onClick={onPick}
      >
        <AgentMark id={agent.id} name={agent.name} />
        <span className="agent-meta">
          <strong>{agent.name}</strong>
          <span className="agent-sub">{connecting ? "Menguji koneksi…" : agent.binary}</span>
        </span>
        {connecting
          ? <span className="agent-status ok"><span className="spinner" /> Menguji</span>
          : <span className="agent-status ok">Terdeteksi</span>}
      </button>
      {children}
    </li>
  );
}
