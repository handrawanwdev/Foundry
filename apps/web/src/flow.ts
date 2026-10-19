import type { Project, Stage } from "./types";

// User-facing steps: Prompt → Rancang (preview, revise, finalize). Discovery
// lives inside Prompt; the finalized design (READY) stays in Rancang.
export const STEPS = [
  { key: "tell", label: "Prompt", path: "/simple/tell" },
  { key: "rancang", label: "Rancang", path: "/simple/rancang" },
] as const;

export type StepKey = (typeof STEPS)[number]["key"];

const stageStep: Record<Stage, number> = {
  TELL: 0, DISCOVERY: 0, DESIGN: 1, READY: 1,
};

export function stepIndex(stage: Stage): number {
  return stageStep[stage] ?? 0;
}

export function stepFor(stage: Stage) {
  return STEPS[stepIndex(stage)];
}

// A step is reachable when the project has reached it.
export function reachable(stage: Stage, step: number): boolean {
  return step <= stepIndex(stage);
}

export function isWorking(p: Project | null): boolean {
  return !!p?.job && (p.job.status === "running" || p.job.status === "waiting_host");
}

export const PLATFORM_LABEL: Record<string, string> = {
  web: "Web", android: "Android", ios: "iPhone", desktop: "Desktop", backend: "Backend",
};

// Editor tabs / activity bar entries.
export const VIEWS: Record<string, { label: string; icon: IconName }> = {
  "/simple/tell": { label: "Prompt", icon: "chat" },
  "/simple/rancang": { label: "Rancang", icon: "design" },
  "/simple/connect": { label: "Hubungkan AI", icon: "plug" },
};

export type IconName =
  | "files" | "chat" | "design" | "plug" | "close"
  | "copy" | "plus" | "minus" | "chevron" | "terminal" | "host"
  | "download" | "warn";

export function viewOf(pathname: string): { label: string; icon: IconName; title?: string } | null {
  return VIEWS[pathname] ?? null;
}
