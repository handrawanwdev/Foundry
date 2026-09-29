import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { stepFor } from "../flow";
import { agentName, useAI } from "../store/ai";
import { useProject } from "../store/project";

export function ConnectPage() {
  const { state, connecting, error, connectAgent, connectMCP, disconnect } = useAI();
  const project = useProject((s) => s.project);
  const nav = useNavigate();
  const [custom, setCustom] = useState("");
  const [showMCP, setShowMCP] = useState(state?.connection?.mode === "mcp");

  if (state?.ready) {
    return (
      <section className="card narrow center-text">
        <h1>✓ AI siap digunakan</h1>
        <p>Agent: <strong>{agentName(state)}</strong></p>
        <div className="row center">
          <button className="primary big" onClick={() => nav(project ? stepFor(project.stage).path : "/")}>Mulai</button>
          <button className="ghost" onClick={disconnect}>Ganti AI</button>
        </div>
      </section>
    );
  }

  const waitingMCP = state?.connection?.mode === "mcp";

  return (
    <section className="card narrow">
      <h1>Hubungkan AI</h1>
      <p className="muted">
        Foundry membutuhkan AI untuk membantu memahami ide, merancang aplikasi, dan membangun project.
      </p>

      <div className="agent-grid">
        {state?.agents.map((a) => (
          <button
            key={a.id}
            className="agent"
            disabled={!a.detected || !!connecting}
            title={a.detected ? a.path : `${a.binary} tidak ditemukan di PATH`}
            onClick={() => connectAgent(a.id)}
          >
            <strong>{a.name}</strong>
            <span className="small muted">
              {connecting === a.id ? "Menguji koneksi…" : a.detected ? "Terdeteksi" : "Belum terpasang"}
            </span>
          </button>
        ))}
      </div>

      <details className="custom">
        <summary className="muted small">Agent lain (perintah sendiri)</summary>
        <form className="inline" onSubmit={(e) => {
          e.preventDefault();
          const argv = custom.trim().split(/\s+/).filter(Boolean);
          if (argv.length) void connectAgent("custom", argv);
        }}>
          <input value={custom} onChange={(e) => setCustom(e.target.value)} placeholder="mis. my-agent --headless (prompt via stdin)" size={40} />
          <button type="submit" disabled={!!connecting}>Hubungkan</button>
        </form>
      </details>

      <div className="divider">atau</div>

      {!showMCP && !waitingMCP ? (
        <button className="wide" onClick={() => setShowMCP(true)}>Hubungkan AI Host lewat MCP</button>
      ) : (
        <div className="mcp-box">
          <p>Tambahkan Foundry ke konfigurasi MCP di AI Host kamu (Claude, Codex, Qwen, Gemini, dll):</p>
          <pre className="code">{JSON.stringify(state?.mcp_config, null, 2)}</pre>
          {!waitingMCP ? (
            <button className="primary" disabled={connecting === "mcp"} onClick={connectMCP}>Sudah ditambahkan</button>
          ) : (
            <p className="waiting"><span className="spinner" /> Menunggu AI Host terhubung… buka AI Host dan panggil tool <code>get_project</code>.</p>
          )}
        </div>
      )}

      {error && <p className="error-text">{error}</p>}
    </section>
  );
}
