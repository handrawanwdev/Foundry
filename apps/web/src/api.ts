import type { AIState, Browse, Command, DesignHandoff, FileContent, FileEntry, PlatformTarget, Project, TaskStatus, Template } from "./types";

export class ApiError extends Error {
  constructor(message: string, public status: number) {
    super(message);
  }
}

async function call<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(`/api/v1${path}`, {
    method,
    headers: body !== undefined ? { "Content-Type": "application/json" } : undefined,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new ApiError(data.error ?? res.statusText, res.status);
  return data as T;
}

export const api = {
  ai: () => call<AIState>("GET", "/ai"),
  connectAgent: (agent_id: string, command?: string[]) => call("POST", "/ai/connect", { agent_id, command }),
  connectMCP: () => call("POST", "/ai/connect-mcp", {}),
  disconnect: () => call("POST", "/ai/disconnect", {}),

  projects: () => call<{ current: string; recent: string[] | null }>("GET", "/projects"),
  openProject: (dir: string, create = false, name = "") => call<Project>("POST", "/projects/open", { dir, create, name }),
  closeProject: () => call("POST", "/projects/close", {}),
  browse: (path?: string) => call<Browse>("GET", `/fs${path ? `?path=${encodeURIComponent(path)}` : ""}`),
  templates: () => call<Template[]>("GET", "/templates"),

  project: () => call<Project>("GET", "/project"),
  story: (story: string) => call<Project>("POST", "/project/story", { story }),
  retry: () => call("POST", "/project/retry", {}),
  setPlatform: (targets: PlatformTarget[]) => call<Project>("PUT", "/project/platform", { targets }),
  setTemplate: (template: string) => call<Project>("PUT", "/project/structure", { template }),
  reviseDesign: (feedback: string) => call("POST", "/project/design/revise", { feedback }),
  keepDesign: () => call("POST", "/project/design/keep", {}),
  undoDesign: () => call("POST", "/project/design/undo", {}),
  designHandoff: () => call<DesignHandoff>("GET", "/project/design/handoff"),
  approveDesign: () => call("POST", "/project/design/approve", {}),
  startBuild: () => call("POST", "/project/build/start", {}),
  stopBuild: () => call("POST", "/project/build/stop", {}),
  moveTask: (id: string, status: TaskStatus) => call<Project>("PATCH", `/project/tasks/${id}`, { status }),
  verify: () => call("POST", "/project/verify", {}),
  fix: () => call("POST", "/project/fix", {}),
  files: (path = "") => call<FileEntry[]>("GET", `/project/files?path=${encodeURIComponent(path)}`),
  file: (path: string) => call<FileContent>("GET", `/project/file?path=${encodeURIComponent(path)}`),
  docs: () => call<{ markdown: string; run: Command[] | null }>("GET", "/project/docs"),
};
