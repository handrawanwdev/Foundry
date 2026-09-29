import type { Project, Stage, TaskStatus } from "./types";

// User-facing steps. Discovery, Plan and Verify live inside these.
export const STEPS = [
  { key: "tell", label: "Ceritakan", path: "/simple/tell" },
  { key: "rancang", label: "Rancang", path: "/simple/rancang" },
  { key: "bangun", label: "Bangun", path: "/simple/bangun" },
  { key: "siap", label: "Siap", path: "/simple/siap" },
] as const;

export type StepKey = (typeof STEPS)[number]["key"];

const stageStep: Record<Stage, number> = {
  TELL: 0, DISCOVERY: 0, DESIGN: 1, PLAN: 1, BUILD: 2, VERIFY: 2, READY: 3,
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

export const COLUMNS: { status: TaskStatus; label: string }[] = [
  { status: "TODO", label: "TODO" },
  { status: "IN_PROGRESS", label: "IN PROGRESS" },
  { status: "REVIEW", label: "REVIEW" },
  { status: "DONE", label: "DONE" },
  { status: "BLOCKED", label: "BLOCKED" },
];

export function taskCounts(p: Project): Record<TaskStatus, number> {
  const c: Record<TaskStatus, number> = { TODO: 0, IN_PROGRESS: 0, REVIEW: 0, DONE: 0, BLOCKED: 0 };
  for (const t of p.tasks ?? []) c[t.status]++;
  return c;
}

export function isWorking(p: Project | null): boolean {
  return !!p?.job && (p.job.status === "running" || p.job.status === "waiting_host");
}

export const PLATFORM_LABEL: Record<string, string> = {
  web: "Web", android: "Android", ios: "iPhone", desktop: "Desktop", backend: "Backend",
};

// Editor tabs / activity bar entries.
export const VIEWS: Record<string, { label: string; icon: IconName }> = {
  "/simple/tell": { label: "Ceritakan", icon: "chat" },
  "/simple/rancang": { label: "Rancang", icon: "design" },
  "/simple/bangun": { label: "Bangun", icon: "board" },
  "/simple/siap": { label: "Siap", icon: "check" },
  "/simple/connect": { label: "Hubungkan AI", icon: "plug" },
};

export type IconName = "files" | "chat" | "design" | "board" | "check" | "plug" | "close";

// Project files open as tabs at /files/<relative path>.
export const FILE_PREFIX = "/files/";

export function fileRoute(path: string): string {
  return FILE_PREFIX + path.split("/").map(encodeURIComponent).join("/");
}

export function viewOf(pathname: string): { label: string; icon: IconName; title?: string } | null {
  if (VIEWS[pathname]) return VIEWS[pathname];
  if (pathname.startsWith(FILE_PREFIX)) {
    const rel = decodeURIComponent(pathname.slice(FILE_PREFIX.length));
    return { label: rel.split("/").pop() || rel, icon: "files", title: rel };
  }
  return null;
}
