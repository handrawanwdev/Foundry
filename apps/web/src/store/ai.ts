import { create } from "zustand";
import { api } from "../api";
import type { AgentOptions, AIState } from "../types";

interface AIStore {
  state: AIState | null;
  loading: boolean;
  connecting: string | null;
  error: string | null;
  load: () => Promise<void>;
  connectAgent: (id: string, command?: string[], opts?: AgentOptions) => Promise<boolean>;
  connectMCP: (agents?: number) => Promise<void>;
  setAgents: (n: number) => Promise<void>;
  disconnect: () => Promise<void>;
}

export const useAI = create<AIStore>((set, get) => ({
  state: null,
  loading: true,
  connecting: null,
  error: null,
  load: async () => {
    try {
      set({ state: await api.ai(), loading: false });
    } catch (e) {
      set({ error: (e as Error).message, loading: false });
    }
  },
  connectAgent: async (id, command, opts) => {
    set({ connecting: id, error: null });
    try {
      await api.connectAgent(id, command, opts);
      await get().load();
      return true;
    } catch (e) {
      set({ error: (e as Error).message });
      return false;
    } finally {
      set({ connecting: null });
    }
  },
  connectMCP: async (agents) => {
    set({ connecting: "mcp", error: null });
    try {
      await api.connectMCP(agents);
      await get().load();
    } catch (e) {
      set({ error: (e as Error).message });
    } finally {
      set({ connecting: null });
    }
  },
  setAgents: async (n) => {
    try {
      set({ state: await api.setAgents(n), error: null });
    } catch (e) {
      set({ error: (e as Error).message });
    }
  },
  disconnect: async () => {
    await api.disconnect();
    await get().load();
  },
}));

export function agentName(s: AIState | null): string {
  if (!s?.connection) return "";
  if (s.connection.mode === "mcp") return s.mcp?.client ? `${s.mcp.client} (MCP)` : "AI Host (MCP)";
  return s.agents.find((a) => a.id === s.connection?.agent_id)?.name ?? s.connection.agent_id ?? "Agent";
}

// Model, effort and parallel workers of the connected agent, e.g. "opus · effort high · 3 agent".
export function agentOptions(s: AIState | null): string {
  const c = s?.connection;
  const n = s?.parallel?.agents;
  const bits = c?.mode === "agent" ? [c.model, c.effort && `effort ${c.effort}`] : [];
  if (n) bits.push(`${n} agent`);
  return bits.filter(Boolean).join(" · ");
}
