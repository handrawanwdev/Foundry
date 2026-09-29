import { create } from "zustand";
import { api } from "../api";
import type { AIState } from "../types";

interface AIStore {
  state: AIState | null;
  loading: boolean;
  connecting: string | null;
  error: string | null;
  load: () => Promise<void>;
  connectAgent: (id: string, command?: string[]) => Promise<boolean>;
  connectMCP: () => Promise<void>;
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
  connectAgent: async (id, command) => {
    set({ connecting: id, error: null });
    try {
      await api.connectAgent(id, command);
      await get().load();
      return true;
    } catch (e) {
      set({ error: (e as Error).message });
      return false;
    } finally {
      set({ connecting: null });
    }
  },
  connectMCP: async () => {
    set({ connecting: "mcp", error: null });
    try {
      await api.connectMCP();
      await get().load();
    } catch (e) {
      set({ error: (e as Error).message });
    } finally {
      set({ connecting: null });
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
