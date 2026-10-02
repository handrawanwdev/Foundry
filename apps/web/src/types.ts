// Mirrors internal/core/model.go — the single project context.

export type Stage = "TELL" | "DISCOVERY" | "DESIGN" | "PLAN" | "BUILD" | "VERIFY" | "READY";
export type TaskStatus = "TODO" | "IN_PROGRESS" | "REVIEW" | "DONE" | "BLOCKED";
export type PlatformKind = "web" | "android" | "ios" | "desktop" | "backend";

export interface Role { name: string; description: string; platform: string }
export interface Discovery {
  summary: string; business: string; users: string[]; roles: Role[];
  data: string[]; integrations: string[]; security: string[]; workflows: string[];
  tech_needs?: string[];
}
export interface BlueprintModule {
  id: string; responsibility: string; data?: string[]; interface?: string[]; depends_on?: string[];
}
export interface BlueprintEntity { name: string; fields?: string[]; module?: string; relates?: string[] }
export interface BlueprintWorker { name: string; trigger: string; job: string }
export interface Blueprint {
  overview: { goal: string; users?: string[]; capabilities?: string[]; core_domain: string };
  platform: { reason: string; devices?: string[]; offline?: string; realtime?: string; deployment?: string };
  frontend?: { state?: string; server_state?: string; forms?: string; validation?: string; routing?: string; api?: string; components?: string };
  backend?: { pattern?: string; di?: string; validation?: string; auth?: string; background?: string };
  database: {
    engine?: string; orm?: string; migrations?: string; naming?: string; pk?: string; soft_delete?: string;
    timestamps?: string; audit?: string; entities?: BlueprintEntity[]; transactions?: string; indexing?: string[];
  };
  cache?: { use?: string; store?: string[]; never?: string[]; invalidation?: string };
  architecture: { pattern: string; reason: string; tradeoff?: string; evolve_when?: string[]; scale?: string };
  modules: BlueprintModule[];
  storage?: { strategy?: string; provider?: string; upload?: string; types?: string[]; security?: string[]; processing?: string[]; lifecycle?: string; cdn?: string };
  auth?: { method?: string; hashing?: string; tokens?: string; session?: string; refresh?: string };
  authz?: { model?: string; convention?: string; permissions?: string[] };
  realtime?: { needed?: boolean; broker?: string; events?: string[]; retry?: string };
  workers?: BlueprintWorker[];
  aggregation?: { needed?: boolean; approach?: string; reads?: string[] };
  performance: { pagination?: string; caching?: string; avoid?: string[]; indexing?: string };
  coding: { backend?: string[]; frontend?: string[]; api?: { style?: string; versioning?: string; errors?: string; pagination?: string } };
  docs?: string[]; workflow?: string[]; change_flow?: string[]; ai_guidelines?: string[];
}
export interface Requirement { id: string; text: string; priority: string }
export interface SubFeature { id: string; name: string }
export interface Feature {
  id: string; name: string; description: string; requirement_ids: string[];
  platforms: PlatformKind[]; sub_features: SubFeature[];
}
export interface Flow { id: string; name: string; feature_id: string; role: string; steps: string[]; page_ids: string[] | null }
export interface PlatformTarget { kind: PlatformKind; audience: string[]; rationale: string }
export interface PlatformPlan { targets: PlatformTarget[]; reason: string; approved: boolean }
export interface Component { kind: string; path: string; tech: string; evidence: string }
export interface Command { name: string; dir: string; argv: string[] }
export type StructureType = "simple" | "modular" | "monorepo" | "microservices";
export type PartKind = "app" | "service" | "module" | "worker" | "package";
export interface StructurePart { name: string; path: string; modules?: string[]; reason?: string }
// The structure decision of a new project: the build contract.
export interface StructureIR {
  type: StructureType; apps?: StructurePart[]; services?: StructurePart[]; workers?: StructurePart[];
  packages?: StructurePart[]; docs: string;
}
export interface StructureOp { op: "type" | "add" | "remove" | "rename" | "move"; kind?: PartKind; name: string; to?: string; reason?: string }
export interface StructureImpact { area: string; message: string }
export interface StructureProposal {
  source: "user" | "ai" | "recommend"; request?: string; note?: string; ops?: StructureOp[]; ir: StructureIR; changes: string[];
  tech?: TechChange[]; issues?: TechIssue[]; impact?: StructureImpact[]; previous: TechStatus; at: string;
}
export interface StructureVersion {
  version: number; at: string; source: string; request?: string; changes?: string[]; impact?: StructureImpact[];
  status: TechStatus; ir: StructureIR;
}
export interface Structure {
  mode: "existing" | "new"; components: Component[] | null; template?: string; layout?: string[];
  reason: string; commands: Command[] | null; approved: boolean; scaffolded: boolean;
  ui?: ExistingUI;
  ir?: StructureIR; recommended?: StructureType; status?: TechStatus; issues?: TechIssue[];
  proposal?: StructureProposal; request?: string; history?: StructureVersion[];
}
// The frontend an existing project already has, read from its code.
export interface ExistingUI {
  paths: string[]; routes?: string[]; shell?: string[]; navigation?: "side" | "top" | "";
  nav_items?: string[]; components?: string[]; theme_id?: string; theme_sources?: string[];
}
// Technology decision: one choice per area with its reason; the user's
// override wins over the recommendation.
export type TechArea = "frontend" | "mobile" | "desktop" | "backend" | "database" | "cache" | "messaging" | "workers" | "storage" | "search"
  | "architecture" | "repository" | "deployment" | "observability";
export interface TechOverride { value: string; language?: string; reason?: string }
export interface TechDecision {
  area: TechArea; recommended: string; language?: string; reason: string; alternatives?: string[]; override?: TechOverride;
}
// Lifecycle of the technology decision (internal/core/techstack.go).
export type TechStatus = "recommended" | "review" | "impact_checked" | "approved" | "changed";
export interface TechChange { area: TechArea; from: string; to: string; language?: string; reason?: string }
export interface TechIssue { level: "error" | "warning"; area?: TechArea; message: string }
export interface TechImpact { area: TechArea; affects: string[]; design?: boolean }
export interface TechProposal {
  source: "user" | "ai"; request?: string; note?: string; changes: TechChange[]; issues?: TechIssue[]; impact?: TechImpact[];
  previous: TechStatus; at: string;
}
export interface TechVersion {
  version: number; at: string; source: "recommend" | "user" | "ai" | "structure"; request?: string; changes?: TechChange[];
  impact?: TechImpact[]; design?: boolean; status: TechStatus; decisions: TechDecision[];
}
export interface TechStack {
  mode: "existing" | "new"; profile?: string; needs?: string[];
  platform: { web: boolean; mobile: boolean; desktop: boolean; backend: boolean };
  decisions: TechDecision[]; approved: boolean; status?: TechStatus; issues?: TechIssue[]; proposal?: TechProposal;
  request?: string; history?: TechVersion[];
}
export interface TechOption { value: string; language?: string; fit: string }
export interface TechProfile { id: string; name: string; description: string }
export interface StackCatalog { catalog: Record<string, TechOption[]>; profiles: TechProfile[]; areas: TechArea[] }
export interface FormField { label: string; type: string; required?: boolean; validation?: string; error?: string; group?: string }
// id: what the layout places. emphasis/tone: semantic tokens (action.*, text.* / status.*).
export interface PageComponent {
  id?: string; type: string; label: string; items?: string[]; columns?: string[]; fields?: FormField[]; target?: string;
  emphasis?: string; tone?: string;
}
// Design IR layout (internal/core/designir.go). A child is a component id or a node.
export interface LayoutNode {
  id?: string; type: string; children?: LayoutChild[]; columns?: number; ratio?: string; side?: string; placement?: string;
  gap?: string; align?: string; surface?: string; elevation?: string;
}
export type LayoutChild = string | LayoutNode;
export interface NodeChange { type?: string; columns?: number; order?: string[] }
export interface DeviceLayout {
  frame?: string; nav?: string; width?: string; hide?: string[]; nodes?: Record<string, NodeChange>; swap?: Record<string, string>;
}
export interface PageLayout {
  frame: string; nav?: string; width?: string; brand?: boolean; back?: string; root: LayoutNode; devices?: Record<string, DeviceLayout>;
}
export type PageKind =
  | "auth" | "dashboard" | "list" | "detail" | "form" | "wizard" | "search"
  | "settings" | "payment" | "profile" | "analytics" | "admin" | "status"
  | "table" | "workspace" | "master-detail" | "monitoring" | "report";
export type Density = "compact" | "comfortable" | "spacious";
export interface PageStates { loading?: string; empty?: string; no_results?: string; error?: string; success?: string }
export interface PageAction { label?: string; target?: string; reason?: string }
// How the page is used (internal/core/model.go PageIntent); the user is the
// role and the goal the purpose.
export interface PageIntent {
  primary_task?: string; frequency?: string; complexity?: string; information_density?: string; comparison?: boolean;
}
// Design decisions taken before the layout; ids are component ids.
export interface PagePlan {
  layout_strategy?: string; responsive_strategy?: string; primary_focus?: string;
  secondary_content?: string[]; supporting_content?: string[]; reasons?: Record<string, string>;
}
export interface PageData { required?: string[]; optional?: string[]; dynamic?: string[]; api?: string[] }
export interface Page {
  id: string; name: string; platform: string; role: string; feature_id: string;
  sub_feature_ids: string[] | null; purpose: string; components: PageComponent[] | null;
  kind?: PageKind; density?: Density; states?: PageStates;
  hierarchy?: string[]; secondary_actions?: string[]; responsive?: string;
  route?: string; access?: "public" | "auth"; primary_action?: PageAction; data?: PageData;
  intent?: PageIntent; plan?: PagePlan; layout?: PageLayout;
}
// Build spec of one page, assembled by the server from the design.
export interface PageHandoff {
  page_id: string; name: string; route: string; platform: string; access: string; role: string; kind?: string;
  purpose: string; hierarchy?: string[]; primary_action: string; secondary_actions?: string[]; responsive?: string;
  density?: string; layout?: PageLayout; plan?: PagePlan; components: string[] | null; data?: PageData;
  interactions: string[] | null; entry_points: string[] | null; states: string[] | null; validation: string[] | null;
  tests: string[] | null; feature_id: string; sub_feature_ids?: string[]; requirement_ids: string[] | null; flow_ids: string[] | null;
}
export interface Blocker { page_id?: string; page?: string; text: string }
export interface DesignHandoff { pages: PageHandoff[] | null; blockers: Blocker[] | null; stack?: TechStack | null; structure?: Structure | null }
export interface DesignChange { kind: "added" | "changed" | "removed"; page_id?: string; page: string; details?: string[] }
export interface ScopePage { id: string; request?: string; components?: number[]; fields?: boolean; device?: string }
// What a change request may touch; the server rejects a patch outside it.
export interface ChangeScope { open?: boolean; fields?: boolean; general?: string; pages?: ScopePage[] }
export interface QAShot { page_id: string; device: string; image: string; outline: string }
export interface QAIssue {
  page_id?: string; severity: "critical" | "major" | "minor"; problem: string; why?: string; impact?: string; fix?: string;
  category?: string; component?: string; device?: string; source?: "lint"; rule?: string;
}
export interface DesignQA {
  status: "running" | "pass" | "fixed" | "failed" | "skipped"; round: number; notes?: string; issues?: QAIssue[]; shots?: QAShot[]; next?: QAShot[];
  compared?: QualityComparison[];
}
// Design quality: rule metrics on the Design IR plus the critic's findings.
// An internal indicator, never a ranking.
export interface QualityScore { overall: number; dimensions: Record<string, number>; blockers: number }
export interface QualityComparison { round: number; before: QualityScore; after: QualityScore; verdict: "improved" | "neutral" | "degraded"; rejected?: boolean }
export interface PageQuality { page_id: string; overall: number; dimensions: Record<string, number>; metrics?: Record<string, number> }
export interface DesignQuality {
  overall: number; dimensions: Record<string, number>; pages?: PageQuality[]; findings?: QAIssue[];
  confidence: "low" | "medium" | "high"; decision: "pass" | "revision_required"; reasons?: QAIssue[];
  compared_to_previous?: QualityComparison; recommendation?: string; measured_at: string;
}
// Canvas: one direct edit of a page's Design IR tree (see work.CanvasOp).
export interface CanvasOp {
  op: "add" | "remove" | "move" | "update" | "set_node" | "wrap" | "unwrap" | "set_page";
  id?: string; parent?: string; index?: number; component?: PageComponent; node?: LayoutNode; ids?: string[];
  fields?: Partial<Record<"name" | "purpose" | "kind" | "density" | "frame" | "nav" | "width", string>>;
}
export interface VersionSummary { id: number; source: string; note?: string; quality?: number; pages: number; theme?: string; at: string; current?: boolean }
export interface VersionComparison { from: number; to: number; changes: DesignChange[] | null; system_changed?: boolean; theme?: [string, string]; template?: [string, string]; flows_changed?: string[] }
export interface ProtoLink { to: string; via: string; kind: "target" | "nav" | "back" | "primary" }
export interface ProtoPage { id: string; name: string; role?: string; platform?: string; route?: string; entry?: boolean; links: ProtoLink[] }
export interface ProtoFlow { id: string; name: string; steps: string[]; gaps?: string[] }
export interface Prototype { pages: ProtoPage[] | null; flows: ProtoFlow[] | null; unreachable?: string[]; dead_ends?: string[] }
export interface DesignRevision { request: string; changes: DesignChange[] | null; scope?: ChangeScope; patch?: string[]; qa?: DesignQA }
export interface Token { name: string; value: string; use?: string }
export interface DesignSystem {
  direction?: string; colors: Token[]; typography: Token[]; spacing: Token[]; radius?: Token[]; shadow?: Token[];
  breakpoints?: Token[]; components?: Token[];
}
export interface Design {
  system?: DesignSystem;
  pages: Page[]; status: "draft" | "changes_requested" | "approved"; feedback: string[] | null;
  revision?: DesignRevision;
  rejected?: string; // why the last patch was refused
  tech_version?: number; // technology version the design was last checked against
  domain?: string;
  qa?: DesignQA; // visual critic of a newly generated design
  quality?: DesignQuality;
  theme?: ThemeContext; // nil on designs made before themes
  template?: TemplateChoice; // nil on designs made before templates or when none fits
}
// A design template: a proven UX pattern (structure), never a page to copy.
export interface TemplateRegion { region: string; role: "primary" | "secondary" | "supporting" | "action" | "navigation"; type: string; area: string; purpose: string }
export interface DesignTemplate {
  id: string; name: string; category: string; intent: string; characteristics: string[];
  suitable_for: { domains?: string[]; roles?: string[]; tasks?: string[]; frequency?: string; density?: string; page_kinds: string[]; keywords?: string[] };
  layout_pattern: { strategies: string[]; navigation: string; density: string; structure: string };
  information_hierarchy: string[];
  component_composition: TemplateRegion[];
  responsive_strategy: { strategy: string; desktop: string; mobile: string };
  quality_rules: { expected_behavior: string[]; avoid: string[]; quality_checks?: string[] };
  wireframes: TemplateWireframe[];
}
// One page of a template sketched in ASCII: how the pattern carries across a product.
export interface TemplateWireframe { page: string; kind: string; layout: string; purpose: string; regions: string[]; sketch: string[]; notes?: string[] }
export interface TemplateChoice { id: string; name: string; secondary?: string; source: "user" | "ai" | "auto"; reasons?: string[] }
// A landed canvas edit and what it made worse.
export interface CanvasResult { project: Project; warnings?: string[] }
export interface TemplateGallery {
  templates: DesignTemplate[] | null; previews: Record<string, Page>; domain?: string;
  system?: DesignSystem; theme?: ThemeContext; recommended?: TemplateChoice; invalid?: ThemeProblem[] | null;
}
// The theme a design follows, as the renderer reads it.
export interface ThemeContext {
  id: string; name: string; source: "user" | "auto" | "fallback"; reasons?: string[];
  color_mode: "light" | "dark"; style?: Record<string, string>;
}
export interface ThemeRule { if: string; then: string; why: string }
export interface Theme {
  id: string; name: string; description?: string; personality: string[];
  tokens: { color_mode: string; radius: string; spacing: string };
  colors: Token[]; typography: Token[]; spacing: Token[]; radius: Token[]; shadow: Token[];
  components: Record<string, string> | null;
  preferences: { navigation?: string; layout?: string[]; density?: string };
  avoid?: string[]; use_when: { domains?: string[]; not?: string[] }; rules?: ThemeRule[];
  interaction?: string[];
  reference?: { url: string; name: string; category: string }; // theme extracted from a reference
}
// A website or app named as inspiration; analyzed into a theme, never copied.
export interface DesignReference {
  url: string; name: string; notes?: string; status: "pending" | "analyzing" | "done" | "failed";
  theme_id?: string; error?: string; at: string;
}
export interface ThemeProblem { id: string; error: string }
export interface Criterion { text: string; met: boolean; evidence: string }
export interface Task {
  id: string; title: string; description: string; area: string; requirement_ids: string[] | null;
  feature_id: string; page_ids: string[] | null; platform: string; depends_on: string[] | null;
  acceptance_criteria: string[]; files: string[] | null; test_requirements: string[] | null;
  status: TaskStatus; attempts: number; owner?: string; progress?: string; notes: string[] | null; last_build_id?: string; criteria?: Criterion[];
}
export interface StepResult { name: string; command: string; dir: string; exit_code: number; output: string; millis: number }
export interface BuildRun { id: string; task_id?: string; steps: StepResult[] | null; passed: boolean; reason?: string; at: string }
export interface Check { name: string; ok: boolean; info: string }
export interface Issue { kind: string; ref: string; message: string }
export interface Trace {
  requirement_id: string; feature_ids: string[] | null; page_ids: string[] | null;
  task_ids: string[] | null; test_task_ids: string[] | null; done: boolean;
}
export interface Verification { ready: boolean; checks: Check[]; issues: Issue[] | null; trace: Trace[] | null; at: string }
export interface Job {
  kind: string; status: "running" | "done" | "failed" | "waiting_host";
  message: string; error?: string; task_id?: string; task_ids?: string[] | null; started: string;
}
export interface QueueItem {
  id: string; state: "independent" | "queued" | "waiting_deps";
  blocked_by?: string; lock?: string; waiting?: string[] | null;
}
export interface Schedule { independent: string[]; items?: QueueItem[] | null }
export interface FlowStep { name: string; detail?: string; role?: string; module?: string; page_id?: string; api?: string; current?: boolean }
export interface SupportFlow { flow_id?: string; name: string; steps: FlowStep[] }
// A cause is confirmed only with evidence; otherwise it is a possibility.
export interface Cause { text: string; evidence?: string; confirmed?: boolean }
export interface Troubleshoot { problem: string; status: string; module?: string; step?: string; causes?: Cause[]; actions?: string[] }
export interface RoleAccess { role: string; access: string; source?: string }
export interface SupportAnswer {
  kind: "explain" | "flow" | "troubleshoot" | "roles" | "unknown"; summary: string;
  flow?: SupportFlow; trouble?: Troubleshoot; roles?: RoleAccess[];
  missing?: string[]; sources?: string[]; follow_ups?: string[]; escalate?: boolean;
}
export interface SupportMessage {
  id: string; role: "user" | "assistant"; text: string;
  status?: "running" | "waiting" | "done" | "failed"; error?: string; answer?: SupportAnswer; at: string;
}
export interface Escalation {
  id: string; issue: string; module?: string; flow?: string; status?: string;
  evidence?: string[]; causes?: string[]; investigation?: string[]; at: string;
}
export interface Support { messages: SupportMessage[] | null; escalations?: Escalation[] }
export interface Event { at: string; source: string; message: string }

export interface Project {
  version: number; name: string; dir: string; stage: Stage; story: string;
  discovery?: Discovery; blueprint?: Blueprint; requirements: Requirement[] | null; features: Feature[] | null; flows: Flow[] | null;
  platform?: PlatformPlan; structure?: Structure; stack?: TechStack; design?: Design; theme?: string; design_template?: string; reference?: DesignReference; tasks: Task[] | null;
  builds: BuildRun[] | null; verification?: Verification; job?: Job; support?: Support; events: Event[] | null; updated_at: string;
  schedule?: Schedule;
}

export interface AgentInfo { id: string; name: string; binary: string; detected: boolean; path?: string; models: string[] | null; efforts: string[] | null }
export interface AgentOptions { model?: string; effort?: string; agents?: number }
export interface Connection { mode: "agent" | "mcp"; agent_id?: string; command?: string[]; model?: string; effort?: string; agents?: number; since: string }
export interface Parallel {
  agents: number; max: number; default: number; from_env: boolean;
}
export interface AIState {
  connection: Connection | null; ready: boolean; mcp_alive: boolean;
  mcp: { client: string; dir: string; at: string } | null;
  agents: AgentInfo[]; mcp_config: unknown; parallel?: Parallel;
}
export interface Template { id: string; name: string; description: string; layout: string[] }
export interface Browse {
  path: string; parent: string; is_project: boolean;
  dirs: { name: string; path: string; is_project: boolean }[];
}
export interface FileEntry { name: string; path: string; is_dir: boolean; size: number }
export interface FileContent { path: string; size: number; binary: boolean; truncated: boolean; content?: string }
