import type { Project, QueueItem, Stage, TaskStatus } from "./types";

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
  { status: "TODO", label: "Antrian" },
  { status: "IN_PROGRESS", label: "Dikerjakan" },
  { status: "REVIEW", label: "Review" },
  { status: "DONE", label: "Selesai" },
  { status: "BLOCKED", label: "Terblokir" },
];

export const COLUMN_EMPTY: Record<TaskStatus, string> = {
  TODO: "Tidak ada antrian",
  IN_PROGRESS: "Tidak ada yang dikerjakan",
  REVIEW: "Tidak ada yang menunggu review",
  DONE: "Belum ada yang selesai",
  BLOCKED: "Tidak ada yang terblokir",
};

export function taskCounts(p: Project): Record<TaskStatus, number> {
  const c: Record<TaskStatus, number> = { TODO: 0, IN_PROGRESS: 0, REVIEW: 0, DONE: 0, BLOCKED: 0 };
  for (const t of p.tasks ?? []) c[t.status]++;
  return c;
}

export function openTodos(p: Project): number {
  return (p.tasks ?? []).filter((t) => t.status === "TODO" && !t.closed).length;
}

export function closedTodos(p: Project): number {
  return (p.tasks ?? []).filter((t) => t.status === "TODO" && t.closed).length;
}

export function formatElapsed(ms: number): string {
  const sec = Math.max(0, Math.floor(ms / 1000));
  const h = Math.floor(sec / 3600);
  const m = Math.floor((sec % 3600) / 60);
  const s = sec % 60;
  if (h > 0) return `${h}j ${m}m ${s}s`;
  if (m > 0) return `${m}m ${s}s`;
  return `${s}s`;
}

export function validTime(iso?: string): boolean {
  if (!iso) return false;
  const t = Date.parse(iso);
  return Number.isFinite(t) && t > Date.parse("2000-01-01T00:00:00Z");
}

export function formatWhen(iso?: string): string {
  if (!validTime(iso)) return "";
  return new Date(iso!).toLocaleString("id-ID", { dateStyle: "medium", timeStyle: "short" });
}

export function isWorking(p: Project | null): boolean {
  return !!p?.job && (p.job.status === "running" || p.job.status === "waiting_host");
}

export function queueItem(p: Project, id: string): QueueItem | undefined {
  return p.schedule?.items?.find((it) => it.id === id);
}

export function queueLabel(it?: QueueItem): string {
  if (!it) return "";
  if (it.state === "independent") return "siap paralel";
  if (it.state === "waiting_deps") return `menunggu ${(it.waiting ?? []).join(", ")}`;
  if (it.blocked_by) {
    const lock = it.lock === "*" ? "seluruh project" : (it.lock ?? "").replace(":", " ");
    return lock ? `dipakai ${it.blocked_by} (${lock})` : `dipakai ${it.blocked_by}`;
  }
  return "antri";
}

export const PLATFORM_LABEL: Record<string, string> = {
  web: "Web", android: "Android", ios: "iPhone", desktop: "Desktop", backend: "Backend",
};

export const SUPPORT_PATH = "/simple/support";

// Editor tabs / activity bar entries.
export const VIEWS: Record<string, { label: string; icon: IconName }> = {
  "/simple/tell": { label: "Ceritakan", icon: "chat" },
  "/simple/rancang": { label: "Rancang", icon: "design" },
  "/simple/bangun": { label: "Bangun", icon: "board" },
  "/simple/siap": { label: "Siap", icon: "check" },
  "/simple/connect": { label: "Hubungkan AI", icon: "plug" },
  "/simple/support": { label: "Support", icon: "help" },
};

export type IconName =
  | "files" | "chat" | "design" | "board" | "check" | "plug" | "help" | "close"
  | "copy" | "plus" | "minus" | "chevron" | "terminal" | "host"
  | "download" | "warn";

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
