import type { Project, Stage } from "./types";

// User-facing steps: Prompt → Blueprint (with the UI design) → Workflow →
// Export (finalize + download). Discovery lives inside Prompt.
export const STEPS = [
  { key: "tell", label: "Prompt", path: "/simple/tell" },
  { key: "blueprint", label: "Blueprint", path: "/blueprint" },
  { key: "workflow", label: "Workflow", path: "/workflow" },
  { key: "export", label: "Export", path: "/export" },
] as const;

export type StepKey = (typeof STEPS)[number]["key"];

// The step a project lands on for its stage.
const stageStep: Record<Stage, number> = {
  TELL: 0, DISCOVERY: 0, DESIGN: 1, READY: 3,
};

export function stepIndex(stage: Stage): number {
  return stageStep[stage] ?? 0;
}

export function stepFor(stage: Stage) {
  return STEPS[stepIndex(stage)];
}

// Prompt is always reachable; the rest once a blueprint exists.
export function reachable(stage: Stage, step: number): boolean {
  return step === 0 || stage === "DESIGN" || stage === "READY";
}

// The design workspace is part of the Blueprint step.
export const DESIGN_PATH = "/blueprint/ui";

export function stepOfPath(path: string): number {
  if (path.startsWith("/blueprint")) return 1;
  return STEPS.findIndex((s) => s.path === path);
}

export function isWorking(p: Project | null): boolean {
  return !!p?.job && (p.job.status === "running" || p.job.status === "waiting_host");
}

export const PLATFORM_LABEL: Record<string, string> = {
  web: "Web", android: "Android", ios: "iPhone", desktop: "Desktop", backend: "Backend",
  cli: "CLI", embedded: "Embedded", other: "Lainnya",
};

export type IconName = "copy" | "chevron" | "terminal" | "host";
