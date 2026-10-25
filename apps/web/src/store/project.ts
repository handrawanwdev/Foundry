import { create } from "zustand";
import { api } from "../api";
import type { Project, RecentProject } from "../types";

interface ProjectStore {
  loaded: boolean;
  project: Project | null; // null = no project open (Start page)
  recent: RecentProject[];
  error: string | null;
  pending: string | null; // action in flight
  load: () => Promise<void>;
  subscribe: () => () => void;
  run: (name: string, fn: () => Promise<unknown>) => Promise<boolean>;
  open: (dir: string, create?: boolean, name?: string) => Promise<boolean>;
  close: () => Promise<boolean>;
  removeRecent: (dir?: string) => Promise<boolean>;
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
    // The stream can stall without an error (sleeping laptop, throttled
    // background tab, proxy), and events missed meanwhile are never replayed.
    // So: refetch whenever the stream (re)opens or the tab returns, and
    // reopen it when the server's 15s ping stops arriving.
    let es: EventSource | null = null;
    let lastSeen = Date.now();
    const open = () => {
      es?.close();
      lastSeen = Date.now();
      es = new EventSource("/api/v1/events");
      es.addEventListener("change", () => { lastSeen = Date.now(); void get().load(); });
      es.addEventListener("ping", () => { lastSeen = Date.now(); });
      es.addEventListener("open", () => void get().load());
    };
    const refresh = () => {
      if (document.visibilityState === "hidden") return;
      if (!es || es.readyState === EventSource.CLOSED || Date.now() - lastSeen > 40_000) open();
      void get().load();
    };
    open();
    const watchdog = setInterval(() => {
      if (document.visibilityState === "visible" && Date.now() - lastSeen > 40_000) open();
    }, 10_000);
    document.addEventListener("visibilitychange", refresh);
    window.addEventListener("focus", refresh);
    window.addEventListener("online", refresh);
    return () => {
      clearInterval(watchdog);
      document.removeEventListener("visibilitychange", refresh);
      window.removeEventListener("focus", refresh);
      window.removeEventListener("online", refresh);
      es?.close();
    };
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
  removeRecent: (dir = "") => get().run("removeRecent", () => api.removeRecent(dir)),
  clearError: () => set({ error: null }),
}));
