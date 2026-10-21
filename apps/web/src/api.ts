import type { AcceptanceEdit, AgentOptions, AIState, Artifact, BlueprintView, Browse, CommandEdit, DeleteImpact, DesignRules, DesignView, ExportView, LayoutSpec, PlatformTarget, Project, StackCatalog, StructureOp, Survey, TechArea, Flow, Template, WorkflowView } from "./types";

// details lists what an invalid request broke (field = where).
export interface ErrorDetail { field?: string; message: string }

export class ApiError extends Error {
  constructor(message: string, public status: number, public code = "", public details: ErrorDetail[] = []) {
    super(message);
  }
}

// Standard envelope: {success:true,data,meta} | {success:false,error:{code,message,details}}.
type Envelope<T> = { success: true; data: T; meta: unknown } | { success: false; error: { code: string; message: string; details: ErrorDetail[] } };

async function call<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(`/api/v1${path}`, {
    method,
    headers: body !== undefined ? { "Content-Type": "application/json" } : undefined,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
  if (res.status === 204) return undefined as T;
  const env = (await res.json().catch(() => null)) as Envelope<T> | null;
  if (!env || !env.success) {
    const err = env && !env.success ? env.error : null;
    throw new ApiError(err?.message ?? res.statusText, res.status, err?.code ?? "", err?.details ?? []);
  }
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
  // Keep a choice marked for review after a decision it depends on changed.
  confirmStackReview: (area: TechArea) => call<Project>("POST", "/project/stack/review", { area }),
  reviewedDesignTech: () => call<Project>("POST", "/project/stack/review-design", {}),
  recommendStack: () => call<Project>("POST", "/project/stack/recommend", {}),
  stackCatalog: () => call<StackCatalog>("GET", "/stack/catalog"),
  // UI design: design rules + layout spec, blueprint artifacts edited as data.
  design: () => call<DesignView>("GET", "/project/design"),
  generateDesign: () => call("POST", "/project/design/generate", {}),
  // focus: design-rules, layout-spec, or shell/page ids.
  reviseDesign: (text: string, focus: string[] = []) => call("POST", "/project/design/revise", { text, focus }),
  // Saved whole; refused with ApiError.details when invalid.
  editDesignRules: (rules: DesignRules) => call<Project>("PUT", "/project/design/rules", rules),
  editLayout: (layout: LayoutSpec) => call<Project>("PUT", "/project/design/layout", layout),

  // Blueprint: built-in artifacts plus the ones written for the project.
  blueprint: () => call<BlueprintView>("GET", "/project/blueprint"),
  reviseBlueprint: (text: string, focus: string[] = []) => call("POST", "/project/blueprint/revise", { text, focus }),
  editArtifact: (id: string, a: Pick<Artifact, "title" | "summary" | "body" | "category" | "refs">) =>
    call<Project>("PUT", `/project/blueprint/artifacts/${encodeURIComponent(id)}`, a),
  deleteImpact: (id: string) => call<DeleteImpact>("GET", `/project/blueprint/documents/${encodeURIComponent(id)}/impact`),
  deleteDocument: (id: string) => call("DELETE", `/project/blueprint/documents/${encodeURIComponent(id)}`),
  // Accepts the current state of the artifacts it refers to.
  reviewArtifact: (id: string) => call<Project>("POST", `/project/blueprint/artifacts/${encodeURIComponent(id)}/review`, {}),
  // AI writes or fixes the areas that are missing or invalid.
  completeScope: () => call("POST", "/project/blueprint/complete", {}),
  // Saved whole; changed items are refused with ApiError.details when invalid.
  editUseCases: (use_cases: Flow[]) => call<Project>("PUT", "/project/blueprint/use-cases", { use_cases }),
  editAcceptance: (acceptance: AcceptanceEdit[], definition_of_done: string[]) =>
    call<Project>("PUT", "/project/blueprint/acceptance", { acceptance, definition_of_done }),
  // resolution "" opens the decision again.
  resolveDecision: (id: string, resolution: string) =>
    call<Project>("PUT", `/project/blueprint/decisions/${encodeURIComponent(id)}`, { resolution }),
  skipArea: (id: string, reason: string) => call<Project>("PUT", `/project/blueprint/areas/${encodeURIComponent(id)}/skip`, { reason }),
  unskipArea: (id: string) => call<Project>("DELETE", `/project/blueprint/areas/${encodeURIComponent(id)}/skip`),

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
