// Mirrors internal/core/model.go — the single project context.

// DESIGN: blueprint and workflow in progress; READY: both finalized, export written.
export type Stage = "TELL" | "DISCOVERY" | "DESIGN" | "READY";
export type PlatformKind = "web" | "android" | "ios" | "desktop" | "backend" | "cli" | "embedded" | "other";
export type SurveyKind = "web" | "mobile" | "desktop" | "cli" | "api" | "embedded" | "other";
export type SurveyUsers = "public" | "registered" | "internal" | "mixed" | "system";
export type SurveyStage = "prototype" | "mvp" | "production";
export type SurveyArch = "modular" | "services" | "existing";
export type SurveyPriority = "security" | "performance" | "reliability" | "scalability" | "testing" | "operations";
// An empty answer is left to Foundry: read from the story or recommended.
export interface Survey {
  what?: string;
  kinds?: SurveyKind[];
  users?: SurveyUsers | "";
  stage?: SurveyStage | "";
  architecture?: SurveyArch | "";
  priorities?: SurveyPriority[];
  constraints?: string;
  // First survey; still honored when an older project has them.
  latency?: string;
  webhook?: string;
}

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
  events?: { durable?: string; ephemeral?: string; outbox?: string; envelope?: string; events?: { type: string; producer?: string; consumers?: string[]; durable?: boolean }[] };
  workers?: BlueprintWorker[];
  aggregation?: { needed?: boolean; approach?: string; reads?: string[] };
  performance: { budget?: string; health?: string; pagination?: string; caching?: string; avoid?: string[]; indexing?: string };
  coding: { backend?: string[]; frontend?: string[]; api?: { style?: string; versioning?: string; errors?: string; pagination?: string } };
  docs?: string[]; workflow?: string[]; change_flow?: string[]; ai_guidelines?: string[];
}
// Acceptance criterion; evidence is the proof the coding agent must hand back, never a result.
export interface Criterion { id: string; kind: string; text: string; evidence?: string }
export interface Requirement { id: string; text: string; priority: string; acceptance?: Criterion[]; untestable?: string }
export interface SubFeature { id: string; name: string }
export interface Feature {
  id: string; name: string; description: string; requirement_ids: string[];
  platforms: PlatformKind[]; sub_features: SubFeature[];
}
// A use case: role is the actor, steps the main flow.
export interface Flow {
  id: string; name: string; feature_id: string; role: string; steps: string[]; page_ids: string[] | null;
  goal?: string; preconditions?: string[]; postconditions?: string[]; alternatives?: string[]; failures?: string[];
  business_rules?: string[]; permissions?: string[]; data_effects?: string[]; integrations?: string[];
}
// A target platform names where the product runs, not who uses it: roles
// live in the requirements (discovery.roles) and security.
export interface PlatformTarget { kind: PlatformKind; rationale: string }
export interface PlatformPlan { targets: PlatformTarget[]; reason: string; approved: boolean }
export interface Component { kind: string; path: string; tech: string; evidence: string }
export type StructureType = "simple" | "modular" | "monorepo" | "microservices";
export type PartKind = "app" | "service" | "module" | "worker" | "package";
export interface StructurePart { name: string; path: string; modules?: string[]; reason?: string }
// The structure decision of a new project (design level).
export interface StructureIR {
  type: StructureType; apps?: StructurePart[]; services?: StructurePart[]; workers?: StructurePart[];
  packages?: StructurePart[]; docs: string;
  // Conventions every part follows: internal module layout, layers, dependencies, naming.
  rules?: string[];
}
export interface StructureOp { op: "type" | "add" | "remove" | "rename" | "move"; kind?: PartKind | "rule"; name: string; to?: string; reason?: string }
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
  reason: string; approved: boolean;
  ui?: ExistingUI;
  ir?: StructureIR; recommended?: StructureType; status?: TechStatus; issues?: TechIssue[];
  proposal?: StructureProposal; request?: string; history?: StructureVersion[];
}
// The frontend an existing project already has, read from its code.
export interface ExistingUI {
  paths: string[]; routes?: string[]; shell?: string[]; navigation?: "side" | "top" | "";
  nav_items?: string[]; components?: string[]; tokens?: DesignTokens; token_sources?: string[];
}
// Technology decision: one choice per area with its reason; the user's
// override wins over the recommendation.
export type TechArea = "frontend" | "ui" | "mobile" | "desktop" | "cli" | "backend" | "api" | "database" | "cache" | "messaging"
  | "workers" | "storage" | "search" | "architecture" | "repository" | "deployment" | "observability" | "testing" | "quality";
export interface TechOverride { value: string; language?: string; runtime?: string; reason?: string }
// Where a recommendation came from (core.Source*); "" in older stacks.
export type TechSource = "existing" | "preference" | "need" | "profile" | "default" | "dependency";
export interface TechDecision {
  area: TechArea; recommended: string; language?: string; runtime?: string; reason: string; source?: TechSource;
  evidence?: string[]; alternatives?: string[]; override?: TechOverride; review?: string;
}
// State of one decision as the user reads it (core.TechStack.DecisionState).
export type DecisionState = "recommended" | "existing" | "selected" | "approved" | "needs_review" | "conflict";
// Lifecycle of the technology decision (internal/core/techstack.go).
export type TechStatus = "recommended" | "review" | "impact_checked" | "approved" | "changed";
export interface TechChange { area: TechArea; from: string; to: string; language?: string; reason?: string; follow?: boolean }
export interface TechIssue { level: "error" | "warning"; area?: TechArea; message: string }
export interface TechImpact { area: TechArea; affects: string[]; design?: boolean }
export interface TechProposal {
  source: "user" | "ai"; request?: string; note?: string; changes: TechChange[]; issues?: TechIssue[]; impact?: TechImpact[];
  review?: TechIssue[]; affected?: { artifacts?: string[]; commands?: string[] }; decisions?: TechDecision[];
  previous: TechStatus; at: string;
}
export interface TechVersion {
  version: number; at: string; source: "recommend" | "user" | "ai" | "structure"; request?: string; changes?: TechChange[];
  impact?: TechImpact[]; design?: boolean; status: TechStatus; decisions: TechDecision[];
}
export interface TechStack {
  mode: "existing" | "new"; profile?: string; needs?: string[];
  platform: { web: boolean; mobile: boolean; android?: boolean; ios?: boolean; desktop: boolean; backend: boolean; cli?: boolean };
  decisions: TechDecision[]; approved: boolean; status?: TechStatus; issues?: TechIssue[]; proposal?: TechProposal;
  request?: string; history?: TechVersion[];
}
export interface TechOption {
  value: string; language?: string; runtime?: string; fit: string; tradeoff?: string; ops?: string; needs?: string[];
  framework?: string; platform?: string;
}
export interface TechProfile { id: string; name: string; description: string }
export interface TechGroup { id: string; title: string; description: string; areas: TechArea[] }
export interface StackCatalog { catalog: Record<string, TechOption[]>; profiles: TechProfile[]; areas: TechArea[]; groups: TechGroup[] }
export interface Token { name: string; value: string; use?: string }
export interface DesignTokens {
  colors: Token[] | null; typography: Token[] | null; spacing: Token[] | null;
  radius?: Token[] | null; shadow?: Token[] | null; breakpoints?: Token[] | null;
}
export interface ComponentRule { name: string; use: string; variants?: string[]; rules?: string[]; source?: string }
// Design rules (internal/core/uidesign.go): what every screen follows.
export interface DesignRules {
  source: "new" | "existing"; direction?: string; principles: string[] | null; ux_guidelines: string[] | null;
  tokens: DesignTokens; components: ComponentRule[] | null; visual_hierarchy?: string[]; accessibility: string[] | null;
  responsive: string[] | null; interaction?: string[]; platforms?: { platform: string; rules: string[] }[]; sources?: string[];
}
export type UIState = "loading" | "empty" | "no_results" | "error" | "success" | "disabled" | "interactive";
export interface Section {
  id: string; name: string; pattern?: string; purpose?: string; content: string[] | null; components?: string[];
  grid?: string; alignment?: string; spacing?: string; sizing?: string; responsive?: string; states?: Partial<Record<UIState, string>>;
}
export interface LayoutShell {
  id: string; name: string; platform?: string; pattern: string; regions: Section[] | null; navigation?: string; grid?: string;
  responsive?: string[]; existing?: boolean; source?: string;
}
export interface LayoutPage {
  id: string; name: string; platform: string; route?: string; role?: string; access?: "public" | "auth"; feature_id?: string;
  purpose: string; shell?: string; pattern: string; primary_action?: string; sections: Section[] | null; hierarchy?: string[];
  grid?: string; responsive?: string[]; links?: { to: string; via: string }[]; states?: Partial<Record<UIState, string>>;
  data?: string[]; existing?: boolean;
}
// Layout specification: shells pages share, then pages of sections.
export interface LayoutSpec { shells: LayoutShell[] | null; pages: LayoutPage[] | null }
// The UI design as text and structured data; never drawn or rendered.
export interface UIDesign {
  rules?: DesignRules; layout?: LayoutSpec; request?: ChangeRequest; version: number; tech_version?: number; updated_at: string;
}
export interface DesignIssue { level: "error" | "warning"; where?: string; message: string }
export interface DesignView {
  design: UIDesign | null; issues: DesignIssue[]; has_ui: boolean; platforms: string[] | null;
  vocabulary: { patterns: string[]; states: UIState[]; required_states: Record<string, UIState[]>; platforms: string[] };
}
export interface Job {
  kind: string; status: "running" | "done" | "failed" | "waiting_host";
  message: string; error?: string; started: string;
}
export interface Event { at: string; source: string; message: string }

export interface Project {
  version: number; name: string; dir: string; stage: Stage; story: string; survey?: Survey;
  discovery?: Discovery; blueprint?: Blueprint; requirements: Requirement[] | null; features: Feature[] | null; flows: Flow[] | null;
  platform?: PlatformPlan; structure?: Structure; stack?: TechStack; ui?: UIDesign;
  artifacts?: Artifact[]; artifact_request?: ChangeRequest; workflow?: Workflow;
  decisions?: Decision[]; area_skips?: AreaSkip[]; definition_of_done?: string[];
  // Decisions changed since the artifacts and commands were brought in line.
  decision_sync?: string[];
  job?: Job; events: Event[] | null; updated_at: string;
}

export interface AgentInfo { id: string; name: string; binary: string; detected: boolean; path?: string; models: string[] | null; efforts: string[] | null }
export interface AgentOptions { model?: string; effort?: string }
export interface Connection { mode: "agent" | "mcp"; agent_id?: string; command?: string[]; model?: string; effort?: string; since: string }
export interface AIState {
  connection: Connection | null; ready: boolean; mcp_alive: boolean;
  mcp: { client: string; dir: string; at: string } | null;
  agents: AgentInfo[]; mcp_config: unknown;
}
export interface Template { id: string; name: string; description: string; layout: string[] }
export interface Browse {
  path: string; parent: string; is_project: boolean;
  dirs: { name: string; path: string; is_project: boolean }[];
}

// Blueprint artifacts: built-in ones are rendered from structured data.
export type ArtifactCategory = "requirements" | "domain" | "architecture" | "ui" | "data" | "platform" | "quality" | "other";
export interface Artifact {
  id: string; title: string; category: ArtifactCategory; summary?: string; body: string;
  refs?: string[]; version: number; updated_at: string; builtin?: boolean; area?: string;
}
export type DecisionKind = "assumption" | "question" | "conflict" | "risk";
// A critical decision without resolution blocks finalize.
export interface Decision { id: string; kind: DecisionKind; text: string; area?: string; critical?: boolean; resolution?: string }
export interface AreaSkip { area: string; reason: string }
export type AreaStatusKind = "covered" | "invalid" | "missing" | "skipped" | "not_applicable";
export interface AreaStatus {
  id: string; title: string; status: AreaStatusKind; artifacts: string[]; reason?: string; errors?: number; skippable?: boolean;
}
export type ArtifactStateKind = "draft" | "valid" | "needs_review" | "finalized";
export interface ArtifactState { status: ArtifactStateKind; reason?: string }
export interface ChangeRequest { text: string; focus?: string[]; rejected?: string; cascade?: boolean; sync?: boolean }
// What deleting one blueprint document reaches.
export interface DeleteImpact {
  doc: string;
  title: string;
  erases: string;
  docs: string[];
  commands: { id: string; name: string; fate: "removed" | "review"; why: string }[];
  artifacts: { id: string; title: string }[];
  areas: string[];
  uncovered: string[];
  ai: boolean;
}

export interface BlueprintView {
  artifacts: Artifact[]; has_ui: boolean; request: ChangeRequest | null; blueprint_hash: string;
  areas: AreaStatus[]; states: Record<string, ArtifactState>; issues: DesignIssue[]; decisions: Decision[]; criterion_kinds: string[];
}
export interface AcceptanceEdit { requirement: string; criteria: Criterion[]; untestable: string }

// Command workflow for external coding agents. Statuses are design states only.
export type CommandStatus = "draft" | "needs_review" | "finalized";
export interface ArtifactRef { id: string; hash?: string }
export interface Command {
  id: string; name: string; objective: string; context?: string; inputs?: string[]; instructions: string;
  depends_on?: string[]; artifacts?: ArtifactRef[]; covers?: string[]; expected_outputs?: string[]; acceptance_criteria?: string[];
  constraints?: string[]; status: CommandStatus; review?: string;
}
export interface Workflow { status: "draft" | "finalized"; commands: Command[]; request?: ChangeRequest; version: number; finalized_at?: string }
export interface WorkflowIssue { command?: string; message: string }
export interface WorkflowReport {
  issues: WorkflowIssue[]; waves: string[][]; stale: Record<string, string[]>; ready: boolean;
  review: Record<string, string>; docs: Record<string, { title: string; category: ArtifactCategory }>;
  // MUST requirements no command covers; shown, not blocking.
  uncovered: string[];
}
export interface WorkflowView { workflow: Workflow | null; report: WorkflowReport; progress?: ProgressSnapshot }

export type RunStatus = "unconfirmed" | "waiting" | "started" | "blocked" | "skipped" | "failed" | "review" | "done" | "verified" | "stale" | "unknown";
export interface ProgressEvidence { kind?: string; ref: string }
export interface ProgressTest { name: string; result: string; detail?: string }
export interface ProgressWarning { command?: string; kind: string; message: string; action?: string }
export interface ProgressCounts {
  total: number; unconfirmed: number; waiting: number; started: number; blocked: number; skipped: number;
  failed: number; review: number; done: number; verified: number; stale: number; unknown: number;
}
export interface CommandProgress {
  id: string; name: string; status: RunStatus; stored: RunStatus; wave: number; waiting?: string[];
  started_at?: string; updated_at?: string; source?: string; summary?: string; reason?: string;
  evidence?: ProgressEvidence[]; tests?: ProgressTest[]; early_start?: boolean; report_id?: string; seq?: number;
}
export interface WaveProgress { index: number; commands: string[]; verified: number; total: number }
export interface StatusChange { at: string; command: string; from: string; to: string; reason?: string; report_id?: string; source?: string }
export interface ProgressSnapshot {
  workflow_version: number; stale_after_sec: number; updated_at?: string; last_report_at?: string; last_source?: string;
  complete: boolean; counts: ProgressCounts; waves: WaveProgress[]; commands: CommandProgress[];
  next: string[]; warnings: ProgressWarning[]; issues: string[]; history: StatusChange[];
}
export interface CommandEdit {
  name: string; objective: string; context: string; inputs: string[]; instructions: string; depends_on: string[];
  artifacts: string[]; covers: string[]; expected_outputs: string[]; acceptance_criteria: string[]; constraints: string[];
}
export interface ExportManifest {
  format: string; project: string; status: "draft" | "finalized"; finalized_at?: string; blueprint_hash: string;
  workflow_version: number; entry: string; waves: string[][];
  artifacts: { id: string; title: string; category: string; file: string; hash: string }[];
  commands: { id: string; name: string; file: string; depends_on: string[]; artifacts: string[]; covers: string[]; wave: number }[];
  areas: AreaStatus[];
}
export interface ExportView { dir: string; finalized: boolean; manifest: ExportManifest; files: { path: string; size: number }[]; blockers: string[] | null }
