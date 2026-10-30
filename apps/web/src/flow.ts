import type { Project, Stage } from "./types";

// User-facing steps: Prompt → Fitur → Blueprint (with design rules and layout) →
// Workflow → Export (finalize + download).
export const STEPS = [
  { key: "tell", label: "Prompt", path: "/simple/tell" },
  { key: "features", label: "Fitur", path: "/features" },
  { key: "blueprint", label: "Blueprint", path: "/blueprint" },
  { key: "workflow", label: "Workflow", path: "/workflow" },
  { key: "export", label: "Export", path: "/export" },
] as const;

export type StepKey = (typeof STEPS)[number]["key"];

// The step a project lands on for its stage.
const stageStep: Record<Stage, number> = {
  TELL: 0, DISCOVERY: 1, DESIGN: 2, READY: 4,
};

export function stepIndex(stage: Stage): number {
  return stageStep[stage] ?? 0;
}

export function stepFor(stage: Stage) {
  return STEPS[stepIndex(stage)];
}

// Prompt is always reachable; Fitur once the story is in; the rest once a blueprint exists.
export function reachable(stage: Stage, step: number): boolean {
  if (step === 0) return true;
  if (step === 1) return stage === "DISCOVERY" || stage === "DESIGN" || stage === "READY";
  return stage === "DESIGN" || stage === "READY";
}

export function stepOfPath(path: string): number {
  if (path.startsWith("/features")) return 1;
  if (path.startsWith("/blueprint")) return 2;
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
