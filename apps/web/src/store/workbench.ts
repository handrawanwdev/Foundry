import { create } from "zustand";

type Dialog = "open" | "new" | null;

interface WorkbenchStore {
  tabs: string[]; // open editor tabs (route paths)
  sidebar: boolean;
  dialog: Dialog;
  openTab: (path: string) => void;
  closeTab: (path: string) => string | null; // returns the tab to show next
  closeAll: () => void;
  toggleSidebar: () => void;
  setDialog: (d: Dialog) => void;
}

// Editor layout state (VS-style). Per viewer only, not part of the project.
export const useWorkbench = create<WorkbenchStore>((set, get) => ({
  tabs: [],
  sidebar: true,
  dialog: null,
  openTab: (path) => {
    if (!get().tabs.includes(path)) set({ tabs: [...get().tabs, path] });
  },
  closeTab: (path) => {
    const tabs = get().tabs;
    const i = tabs.indexOf(path);
    const next = tabs.filter((t) => t !== path);
    set({ tabs: next });
    return next[Math.min(i, next.length - 1)] ?? null;
  },
  closeAll: () => set({ tabs: [] }),
  toggleSidebar: () => set({ sidebar: !get().sidebar }),
  setDialog: (dialog) => set({ dialog }),
}));
