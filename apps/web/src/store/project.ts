import { create } from "zustand";
import { api } from "../api";
import { mergeTimeline, timelineOf } from "../bp-event";
import type { BlueprintEvent, ProgressFrame, Project, RecentProject } from "../types";

interface ProjectStore {
  loaded: boolean;
  project: Project | null; // null = no project open (Start page)
  recent: RecentProject[];
  error: string | null;
  pending: string | null; // action in flight
  stream: "open" | "reconnecting" | "offline";
  timeline: BlueprintEvent[]; // the run's activity, in sequence order
  load: () => Promise<void>;
  applyProgress: (frame: ProgressFrame) => void;
  subscribe: () => () => void;
  run: (name: string, fn: () => Promise<unknown>) => Promise<boolean>;
  open: (dir: string, create?: boolean, name?: string) => Promise<boolean>;
  close: () => Promise<boolean>;
  removeRecent: (dir?: string) => Promise<boolean>;
  clearError: () => void;
}

// Calls that arrive during a load share one follow-up load that starts after
// them, so each caller still sees state at least as new as its own call.
let loading: Promise<void> | null = null;
let stale = false;

// One store for the one project context. Every view reads from here; the
// server pushes a change event whenever the state file changes (UI, agent or
// MCP host) or the active project is opened/closed, and the store refetches.
export const useProject = create<ProjectStore>((set, get) => ({
  loaded: false,
  project: null,
  recent: [],
  error: null,
  pending: null,
  stream: "reconnecting",
  timeline: [],
  load: () => {
    if (loading) {
      stale = true;
      return loading;
    }
    loading = (async () => {
      do {
        stale = false;
        try {
          const { current, recent } = await api.projects();
          const project = current ? await api.project() : null;
          const prev = get().project;
          if (project && prev?.dir === project.dir && prev.blueprint_progress && project.blueprint_progress
            && project.blueprint_progress.seq < prev.blueprint_progress.seq) {
            project.blueprint_progress = prev.blueprint_progress; // a slower response than the stream
          }
          // Only another run's events go: the stream can land before this load.
          const timeline = timelineOf(get().timeline, project?.blueprint_progress?.run_id);
          set({ project, recent: recent ?? [], loaded: true, timeline });
        } catch (e) {
          set({ error: (e as Error).message, loaded: true });
        }
      } while (stale);
      loading = null;
    })();
    return loading;
  },
  // The frame is the backend's state: replace, never derive. seq only grows,
  // so a late or replayed frame cannot overwrite a newer status.
  applyProgress: ({ progress, events }) => {
    if (!progress) return;
    const timeline = mergeTimeline(get().timeline, progress.run_id, events ?? []);
    const project = get().project;
    if (!project || progress.seq < (project.blueprint_progress?.seq ?? 0)) {
      set({ timeline });
      return;
    }
    set({ timeline, project: { ...project, blueprint_progress: progress } });
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
      set({ stream: "reconnecting" });
      const after = get().timeline.at(-1)?.sequence_number ?? 0;
      es = new EventSource(`/api/v1/events?after=${after}`);
      es.addEventListener("change", () => { lastSeen = Date.now(); void get().load(); });
      es.addEventListener("progress", (e: MessageEvent) => {
        lastSeen = Date.now();
        try {
          get().applyProgress(JSON.parse(e.data) as ProgressFrame);
        } catch { /* ignore a truncated frame */ }
      });
      es.addEventListener("ping", () => { lastSeen = Date.now(); });
      es.addEventListener("open", () => { set({ stream: "open" }); void get().load(); });
      es.onerror = () => set({ stream: "reconnecting" });
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
