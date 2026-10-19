import { create } from "zustand";

type Dialog = "open" | "new" | null;

interface WorkbenchStore {
  dialog: Dialog;
  setDialog: (d: Dialog) => void;
}

// Which project dialog is open. Per viewer only, not part of the project.
export const useWorkbench = create<WorkbenchStore>((set) => ({
  dialog: null,
  setDialog: (dialog) => set({ dialog }),
}));
