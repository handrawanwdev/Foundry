import type { AgentOptions, AIState, Browse, CanvasOp, CanvasResult, ChangeScope, Command, DesignHandoff, Escalation, FileContent, FileEntry, PlatformTarget, Project, Prototype, TaskStatus, Template, TemplateGallery, Theme, ThemeProblem, VersionComparison, VersionSummary } from "./types";

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
  models: (agent: string) => call<{ models: string[] }>("GET", `/ai/models?agent=${encodeURIComponent(agent)}`),
  connectAgent: (agent_id: string, command?: string[], opts?: AgentOptions) => call("POST", "/ai/connect", { agent_id, command, ...opts }),
  connectMCP: () => call("POST", "/ai/connect-mcp", {}),
  disconnect: () => call("POST", "/ai/disconnect", {}),

  projects: () => call<{ current: string; recent: string[] | null }>("GET", "/projects"),
  openProject: (dir: string, create = false, name = "") => call<Project>("POST", "/projects/open", { dir, create, name }),
  removeRecent: (dir = "") => call("POST", "/projects/recent/remove", { dir }),
  closeProject: () => call("POST", "/projects/close", {}),
  browse: (path?: string) => call<Browse>("GET", `/fs${path ? `?path=${encodeURIComponent(path)}` : ""}`),
  templates: () => call<Template[]>("GET", "/templates"),
  // Design templates with previews; theme/domain "" = the open project's.
  designTemplates: (theme = "", domain = "") =>
    call<TemplateGallery>("GET", `/design-templates?theme=${encodeURIComponent(theme)}&domain=${encodeURIComponent(domain)}`),
  themes: () => call<{ themes: Theme[] | null; invalid: ThemeProblem[] | null }>("GET", "/themes"),

  project: () => call<Project>("GET", "/project"),
  story: (story: string) => call<Project>("POST", "/project/story", { story }),
  retry: () => call("POST", "/project/retry", {}),
  setPlatform: (targets: PlatformTarget[]) => call<Project>("PUT", "/project/platform", { targets }),
  setTemplate: (template: string) => call<Project>("PUT", "/project/structure", { template }),
  // theme "" lets Foundry select one; a draft design is re-themed at once.
  setTheme: (theme: string) => call<Project>("PUT", "/project/theme", { theme }),
  // template "" lets the AI choose, "blank" asks for none; applies on generate.
  setDesignTemplate: (template: string) => call<Project>("PUT", "/project/design-template", { template }),
  // url "" removes the reference; otherwise it is analyzed into a theme.
  setReference: (url: string, notes = "") => call<Project>("PUT", "/project/reference", { url, notes }),
  // screens: what the user sees on each page, so the designer AI sees it too.
  // scope: what the change may touch; the server rejects a patch outside it.
  reviseDesign: (feedback: string, screens: Record<string, string>, scope: ChangeScope) =>
    call("POST", "/project/design/revise", { feedback, screens, scope }),
  keepDesign: () => call("POST", "/project/design/keep", {}),
  // A new design from the current features and flows (no discovery).
  regenerateDesign: () => call("POST", "/project/design/regenerate", {}),
  undoDesign: () => call("POST", "/project/design/undo", {}),
  designHandoff: () => call<DesignHandoff>("GET", "/project/design/handoff"),
  // Canvas: direct Design IR edits of one page; rejected as a whole when invalid.
  editDesign: (page: string, ops: CanvasOp[]) => call<CanvasResult>("POST", "/project/design/edit", { page, ops }),
  prototype: () => call<Prototype>("GET", "/project/design/prototype"),
  versions: () => call<VersionSummary[]>("GET", "/project/design/versions"),
  // to 0 = the current design.
  compareVersions: (from: number, to = 0) => call<VersionComparison>("GET", `/project/design/versions/compare?from=${from}&to=${to}`),
  restoreVersion: (id: number) => call<Project>("POST", `/project/design/versions/${id}/restore`),
  approveDesign: () => call("POST", "/project/design/approve", {}),
  startBuild: () => call("POST", "/project/build/start", {}),
  stopBuild: () => call("POST", "/project/build/stop", {}),
  moveTask: (id: string, status: TaskStatus) => call<Project>("PATCH", `/project/tasks/${id}`, { status }),
  verify: () => call("POST", "/project/verify", {}),
  fix: () => call("POST", "/project/fix", {}),
  files: (path = "") => call<FileEntry[]>("GET", `/project/files?path=${encodeURIComponent(path)}`),
  file: (path: string) => call<FileContent>("GET", `/project/file?path=${encodeURIComponent(path)}`),
  askSupport: (question: string) => call("POST", "/project/support/ask", { question }),
  retrySupport: () => call("POST", "/project/support/retry", {}),
  clearSupport: () => call("POST", "/project/support/clear", {}),
  escalate: (e: Omit<Escalation, "id" | "at">) => call<Escalation>("POST", "/project/support/escalate", e),
  docs: () => call<{ markdown: string; run: Command[] | null }>("GET", "/project/docs"),
};
