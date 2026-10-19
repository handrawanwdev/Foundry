import type { AgentOptions, AIState, Artifact, BlueprintView, Browse, CommandEdit, ExportView, WorkflowView, CanvasOp, CanvasResult, ChangeScope, DesignSpec, PlatformTarget, Project, StackCatalog, StructureOp, Survey, TechArea, Prototype, Template, TemplateGallery, Theme, ThemeProblem, VersionComparison, VersionSummary } from "./types";

export class ApiError extends Error {
  constructor(message: string, public status: number, public code = "") {
    super(message);
  }
}

// Standard envelope: {success:true,data,meta} | {success:false,error:{code,message,details}}.
type Envelope<T> = { success: true; data: T; meta: unknown } | { success: false; error: { code: string; message: string; details: unknown[] } };

async function call<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(`/api/v1${path}`, {
    method,
    headers: body !== undefined ? { "Content-Type": "application/json" } : undefined,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
  if (res.status === 204) return undefined as T;
  const env = (await res.json().catch(() => null)) as Envelope<T> | null;
  if (!env || !env.success) throw new ApiError(env && !env.success ? env.error.message : res.statusText, res.status, env && !env.success ? env.error.code : "");
  return env.data;
}

export const api = {
  ai: () => call<AIState>("GET", "/ai"),
  models: (agent: string) => call<{ models: string[] }>("GET", `/ai/models?agent=${encodeURIComponent(agent)}`),
  connectAgent: (agent_id: string, command?: string[], opts?: AgentOptions) => call("POST", "/ai/connect", { agent_id, command, ...opts }),
  connectMCP: () => call("POST", "/ai/connect-mcp", {}),
  disconnect: (all = false) => call("POST", "/ai/disconnect", { all }),

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
  story: (story: string, survey?: Survey) => call<Project>("POST", "/project/story", { story, survey }),
  retry: () => call("POST", "/project/retry", {}),
  dismissJob: () => call("POST", "/project/job/dismiss", {}),
  setPlatform: (targets: PlatformTarget[]) => call<Project>("PUT", "/project/platform", { targets }),
  // Structure edits become a proposal; applyStructure/discardStructure decide it.
  proposeStructure: (ops: StructureOp[]) => call<Project>("PUT", "/project/structure", { ops }),
  requestStructure: (text: string) => call("POST", "/project/structure/request", { text }),
  applyStructure: () => call<Project>("POST", "/project/structure/apply", {}),
  discardStructure: () => call<Project>("POST", "/project/structure/discard", {}),
  approveStructure: () => call<Project>("POST", "/project/structure/approve", {}),
  recommendStructure: () => call<Project>("POST", "/project/structure/recommend", {}),
  // value "" returns the area to Foundry's recommendation.
  // Technology changes become a proposal; applyStack/discardStack decide it.
  proposeStack: (changes: { area: TechArea; value: string; reason?: string }[]) => call<Project>("PUT", "/project/stack", { changes }),
  requestStack: (text: string) => call("POST", "/project/stack/request", { text }),
  applyStack: () => call<Project>("POST", "/project/stack/apply", {}),
  discardStack: () => call<Project>("POST", "/project/stack/discard", {}),
  approveStack: () => call<Project>("POST", "/project/stack/approve", {}),
  reviewedDesignTech: () => call<Project>("POST", "/project/stack/review-design", {}),
  recommendStack: () => call<Project>("POST", "/project/stack/recommend", {}),
  stackCatalog: () => call<StackCatalog>("GET", "/stack/catalog"),
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
  designSpec: () => call<DesignSpec>("GET", "/project/design/handoff"),
  // Canvas: direct Design IR edits of one page; rejected as a whole when invalid.
  editDesign: (page: string, ops: CanvasOp[]) => call<CanvasResult>("POST", "/project/design/edit", { page, ops }),
  prototype: () => call<Prototype>("GET", "/project/design/prototype"),
  versions: () => call<VersionSummary[]>("GET", "/project/design/versions"),
  // to 0 = the current design.
  compareVersions: (from: number, to = 0) => call<VersionComparison>("GET", `/project/design/versions/compare?from=${from}&to=${to}`),
  restoreVersion: (id: number) => call<Project>("POST", `/project/design/versions/${id}/restore`),

  // Blueprint: built-in artifacts plus the ones written for the project.
  blueprint: () => call<BlueprintView>("GET", "/project/blueprint"),
  reviseBlueprint: (text: string, focus: string[] = []) => call("POST", "/project/blueprint/revise", { text, focus }),
  editArtifact: (id: string, a: Pick<Artifact, "title" | "summary" | "body" | "category" | "refs">) =>
    call<Project>("PUT", `/project/blueprint/artifacts/${encodeURIComponent(id)}`, a),
  removeArtifact: (id: string) => call<Project>("DELETE", `/project/blueprint/artifacts/${encodeURIComponent(id)}`),

  // Command workflow: instructions for external coding agents, never run by Foundry.
  workflow: () => call<WorkflowView>("GET", "/project/workflow"),
  generateWorkflow: () => call("POST", "/project/workflow/generate", {}),
  reviseWorkflow: (text: string, focus: string[] = []) => call("POST", "/project/workflow/revise", { text, focus }),
  editCommand: (id: string, edit: CommandEdit) => call<Project>("PUT", `/project/workflow/commands/${encodeURIComponent(id)}`, edit),
  // needs true marks it for review; false records it as reviewed.
  reviewCommand: (id: string, needs: boolean, reason = "") =>
    call<Project>("POST", `/project/workflow/commands/${encodeURIComponent(id)}/review`, { needs_review: needs, reason }),

  // Finalize approves blueprint + workflow and writes the export package.
  finalize: () => call("POST", "/project/finalize", {}),
  reopen: () => call("POST", "/project/reopen", {}),
  exportInfo: () => call<ExportView>("GET", "/project/export"),
};

// Downloads for external coding agents.
export const EXPORT_ZIP_URL = "/api/v1/project/export.zip";
export const SPEC_MD_URL = "/api/v1/project/spec.md";
export const SPEC_JSON_URL = "/api/v1/project/spec.json";
