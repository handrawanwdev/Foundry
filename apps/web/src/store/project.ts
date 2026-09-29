import { create } from "zustand";
import { api } from "../api";
import type { Project } from "../types";

interface ProjectStore {
  loaded: boolean;
  project: Project | null; // null = no project open (Start page)
  recent: string[];
  error: string | null;
  pending: string | null; // action in flight
  load: () => Promise<void>;
  subscribe: () => () => void;
  run: (name: string, fn: () => Promise<unknown>) => Promise<boolean>;
  open: (dir: string, create?: boolean, name?: string) => Promise<boolean>;
  close: () => Promise<boolean>;
  clearError: () => void;
}

// One store for the one project context. Every view reads from here; the
// server pushes a change event whenever the state file changes (UI, agent or
// MCP host) or the active project is opened/closed, and the store refetches.
export const useProject = create<ProjectStore>((set, get) => ({
  loaded: false,
  project: null,
  recent: [],
  error: null,
  pending: null,
  load: async () => {
    try {
      const { current, recent } = await api.projects();
      const project = current ? await api.project() : null;
      set({ project, recent: recent ?? [], loaded: true });
    } catch (e) {
      set({ error: (e as Error).message, loaded: true });
    }
  },
  subscribe: () => {
    const es = new EventSource("/api/v1/events");
    es.addEventListener("change", () => void get().load());
    return () => es.close();
  },
  run: async (name, fn) => {
    set({ pending: name, error: null });
    try {
      await fn();
      await get().load();
      return true;
    } catch (e) {
      set({ error: (e as Error).message });
      return false;
    } finally {
      set({ pending: null });
    }
  },
  open: (dir, create = false, name = "") => get().run("open", () => api.openProject(dir, create, name)),
  close: () => get().run("close", api.closeProject),
  clearError: () => set({ error: null }),
}));
