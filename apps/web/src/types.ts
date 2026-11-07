// Mirrors internal/core/model.go — the single project context.

// DESIGN: blueprint and workflow in progress; READY: both finalized, export written.
export type Stage = "TELL" | "DISCOVERY" | "DESIGN" | "READY";
export type PlatformKind = "web" | "android" | "ios" | "desktop" | "backend" | "cli" | "embedded" | "other";
export type SurveyKind = "web" | "mobile" | "desktop" | "cli" | "api" | "embedded" | "other";
export type SurveyUsers = "public" | "registered" | "internal" | "mixed" | "system";
export type SurveyStage = "prototype" | "mvp" | "production";
export type SurveyArch = "modular" | "services" | "existing";
export type WorkIntent =
  | "new_project"
  | "existing_project"
  | "new_feature"
  | "enhancement"
  | "refactoring"
  | "bug_fix"
  | "architecture";
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
  design_reference?: string;
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
export interface SubFeature { id: string; name: string; feature_id?: string; source_id?: string }
export interface Feature {
  id: string; name: string; description: string; requirement_ids: string[];
  platforms: PlatformKind[]; sub_features: SubFeature[]; source_id?: string;
}
// Feature Discovery tree, composed before Blueprint. Independent of Feature.
export interface FeatureNode {
  id: string; parent_id?: string; name: string;
  feature_id?: string; sub_feature_id?: string;
  description?: string; purpose?: string; benefit?: string; outcome?: string;
  children?: FeatureNode[];
}
export interface FeatureModule { id: string; name: string; features: FeatureNode[] }
export interface FeatureMapRequest { action: string; node_id?: string; text?: string; rejected?: string }
export interface FeatureMap { summary?: string; modules: FeatureModule[]; request?: FeatureMapRequest }
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
export interface StructurePart {
  name: string; path: string; modules?: string[]; reason?: string;
  // Responsibility of each module (domain folder).
  module_reasons?: Record<string, string>;
}
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
  source: "user" | "ai" | "recommend"; request?: string; note?: string; alternative?: string;
  ops?: StructureOp[]; ir: StructureIR; changes: string[];
  tech?: TechChange[]; issues?: TechIssue[]; impact?: StructureImpact[]; previous: TechStatus; at: string;
}
export interface StructureEvaluation {
  maintainability: number; simplicity: number; extensibility: number; testability: number;
  separation_of_concerns: number; complexity: number; score?: number;
}
export interface StructureAlternative {
  id: string; name: string; summary: string; type: StructureType; ir: StructureIR;
  evaluation: StructureEvaluation; pros?: string[]; cons?: string[]; tradeoffs?: string[]; recommended?: boolean;
}
export interface StructureDomain { name: string; responsibility: string }
export interface StructurePlan {
  summary: string; drivers?: string[]; domains?: StructureDomain[]; alternatives: StructureAlternative[];
  recommendation: string; reason: string; dependency_rules?: string[];
  source_hash?: string; generated_at?: string; stale?: boolean;
  selected?: string; decision?: string; decided_at?: string;
}
export interface StructurePlanRequest { text?: string; source_hash?: string; rejected?: string }
export interface StructureVersion {
  version: number; at: string; source: string; request?: string; changes?: string[]; impact?: StructureImpact[];
  status: TechStatus; ir: StructureIR; analysis?: string; alternative?: string; reason?: string; tree?: string;
}
export interface ContextFinding {
  kind?: string; text: string; path?: string; evidence?: string; confidence?: "observed" | "documented" | "assumed" | "estimated";
}
export interface ContextModule { name: string; path: string; evidence?: string; depends_on?: string[] }
export interface ContextEntity { name: string; fields?: string[]; path?: string; evidence?: string }
export interface ContextAPI { kind?: string; path: string; detail?: string; evidence?: string }
export interface ContextDep { from: string; to: string; kind?: string; path?: string; evidence?: string; confidence?: string }
export interface ContextChange { area: string; path?: string; action: "add" | "modify" | "remove" }
export interface ProjectContext {
  layout?: ContextFinding[]; languages?: ContextFinding[]; frameworks?: ContextFinding[];
  modules?: ContextModule[]; data?: ContextEntity[]; apis?: ContextAPI[];
  auth?: ContextFinding[]; tests?: ContextFinding[]; docs?: ContextFinding[];
  conventions?: ContextFinding[]; features?: ContextFinding[];
  deps?: ContextDep[]; changes?: ContextChange[];
  scanned_at?: string; fingerprint?: string; area_hash?: Record<string, string>;
  stale?: boolean; stale_areas?: string[];
  corrections?: string;
}
export interface ChangeItem { action: "add" | "modify" | "reuse" | "refactor" | "remove"; target: string; module?: string; reason?: string; files?: string[] }
export interface ChangeImpact {
  area: string; message: string; risk?: "low" | "medium" | "high";
  kind?: string; source?: "observed" | "estimated"; evidence?: string; path?: string;
}
export interface ChangeConflict { kind?: string; where?: string; message: string; keep?: string; reason?: string }
export interface ChangeFile { path: string; responsibility?: string; reason?: string }
export interface ChangeFiles { add?: ChangeFile[]; modify?: ChangeFile[]; remove?: ChangeFile[] }
export interface ChangePlan {
  scope?: "full" | "change"; summary?: string; existing?: string[];
  items?: ChangeItem[]; impact?: ChangeImpact[]; files?: ChangeFiles;
  conflicts?: ChangeConflict[]; kept?: string[];
}
export interface Structure {
  mode: "existing" | "new"; components: Component[] | null; template?: string; layout?: string[];
  reason: string; approved: boolean;
  ui?: ExistingUI;
  context?: ProjectContext;
  ir?: StructureIR; recommended?: StructureType; status?: TechStatus; issues?: TechIssue[];
  proposal?: StructureProposal; plan?: StructurePlan; plan_request?: StructurePlanRequest;
  request?: string; history?: StructureVersion[];
  // The detailed repository plan the AI designs, and the design or revision waiting for it.
  repo?: RepoPlan; repo_request?: RepoRequest;
  adaptation?: ChangeItem[];
}

// ---- Repository plan: folders and files with their responsibility, module mapping, dependency rules ----
export interface RepoNode {
  path: string; name?: string; type: "dir" | "file"; role?: string; responsibility: string;
  // A project module id, "shared" or "platform".
  owner: string; depends_on?: string[]; reason: string;
  // A part of the structure decision: app:web, service:api, module:booking, ...
  part?: string; shared_by?: string[]; allowed?: string[]; forbidden?: string[];
  relations?: string; boundary?: string; implied?: boolean;
}
export interface RepoLocation { path: string; role?: string }
export interface RepoModuleMap { module: string; locations?: RepoLocation[]; frontend: boolean; backend: boolean; note?: string }
export interface RepoDepRule { from: string; to: string; via?: string; mode?: "sync" | "event"; reason?: string }
export interface RepoInterface { module: string; path: string; reason?: string }
export interface RepoDependency {
  direction: string; allowed?: RepoDepRule[]; forbidden?: RepoDepRule[]; communication?: RepoDepRule[];
  interfaces?: RepoInterface[]; shared_rules?: string[]; cycle_guard?: string;
}
export interface RepoCriterion { name: string; score: number; reason: string }
export interface RepoFinding { category: string; severity: "info" | "warning" | "error"; path?: string; message: string; resolution?: string; source?: "ai" | "computed" }
export interface RepoEvaluation { summary?: string; criteria: RepoCriterion[]; findings: RepoFinding[]; score: number; outdated?: boolean }
export interface RepoCheck { area: string; status: "consistent" | "resolved" | "conflict"; note?: string; source?: "ai" | "computed" }
export interface RepoDecision { title: string; choice: string; reason: string; alternatives?: string[]; tradeoff?: string }
export interface RepoEdge { from: string; to: string; kind: "derived" | "allowed" | "forbidden" | "communication"; mode?: string }
export interface RepoUnit { name: string; nodes: number; fan_out: number; fan_in: number }
// area: folders | modules | structure | dependency | evaluation | consistency | sources
export interface RepoIssue { level: "error" | "warning"; area?: string; message: string }
export interface RepoAnalysis {
  issues?: RepoIssue[]; findings?: RepoFinding[]; checks?: RepoCheck[]; units?: RepoUnit[]; edges?: RepoEdge[];
  dirs: number; files: number; max_depth: number; stale_areas?: string[]; affected?: string[];
}
export interface RepoVersion {
  version: number; at: string; source: "generate" | "ai" | "user"; request?: string; reason?: string; changes?: string[];
  status: TechStatus; nodes: number; score: number;
}
export interface RepoRequest { action: "generate" | "revise"; text?: string; reason?: string; areas?: string[]; rejected?: string }
export interface RepoPlan {
  approach: string; topology: string; summary: string; rationale?: string;
  nodes: RepoNode[]; modules: RepoModuleMap[]; dependency: RepoDependency; rules?: string[];
  evaluation: RepoEvaluation; consistency?: RepoCheck[]; decisions?: RepoDecision[];
  status: TechStatus; history?: RepoVersion[]; generated_at?: string; stale?: boolean; analysis: RepoAnalysis;
}
// One revision of the plan; the server documents every op.
export interface RepoOp {
  op: "set_node" | "remove_node" | "move_node" | "set_module" | "remove_module" | "add_rule" | "remove_rule"
    | "add_dependency" | "remove_dependency" | "set_interface" | "remove_interface" | "set_meta";
  path?: string; to?: string; node?: Partial<RepoNode>; module?: Partial<RepoModuleMap>; text?: string;
  kind?: "allowed" | "forbidden" | "communication"; dep?: Partial<RepoDepRule>; interface?: Partial<RepoInterface>;
  approach?: string; topology?: string; summary?: string;
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
export type TechSource = "existing" | "preference" | "need" | "profile" | "default" | "dependency" | "ai";
export interface TechDecision {
  area: TechArea; recommended: string; language?: string; runtime?: string; reason: string; source?: TechSource;
  evidence?: string[]; alternatives?: string[]; override?: TechOverride; review?: string;
}
// State of one decision as the user reads it (core.TechStack.DecisionState).
export type DecisionState = "recommended" | "existing" | "selected" | "approved" | "needs_review" | "conflict";
// Lifecycle of the technology decision (internal/core/techstack.go).
export type TechStatus = "recommended" | "review" | "impact_checked" | "approved" | "changed";
export interface TechChange { area: TechArea; from: string; to: string; language?: string; reason?: string; follow?: boolean }
export interface TechIssue { level: "error" | "warning"; area?: string; message: string }
export interface TechImpact { area: TechArea; affects: string[]; design?: boolean }
export interface TechProposal {
  source: "user" | "ai"; request?: string; note?: string; changes: TechChange[]; issues?: TechIssue[]; impact?: TechImpact[];
  review?: TechIssue[]; affected?: { artifacts?: string[]; commands?: string[] }; decisions?: TechDecision[];
  previous: TechStatus; at: string; plan?: TechPlan;
}
export interface TechVersion {
  version: number; at: string; source: "recommend" | "user" | "ai" | "structure"; request?: string; changes?: TechChange[];
  impact?: TechImpact[]; design?: boolean; status: TechStatus; decisions: TechDecision[];
}
export interface TechFact { category: string; statement: string; source?: string }
export interface TechContext { summary?: string; facts?: TechFact[]; unknowns?: string[]; assumptions?: string[] }
export interface TechAlternative {
  technology: string; why: string; difference: string; pros?: string[]; cons?: string[];
  architecture_impact?: string; maintenance_impact?: string; cost_impact?: string;
}
export interface TechItem {
  area: string; group?: string; technology: string; version?: string; version_verified?: boolean;
  language?: string; runtime?: string; purpose: string; fulfills?: string[]; driver: string; reasoning: string;
  pros?: string[]; cons?: string[]; tradeoffs?: string[]; depends_on?: string[]; integration?: string; operational?: string;
  alternatives?: TechAlternative[]; confidence: string; assumptions?: string[]; action?: "reuse" | "add" | "modify" | "refactor" | "remove";
  in_catalog?: boolean; user_chosen?: string;
}
export interface TechConstraint { areas: string[]; rule: string; reason?: string }
export interface TechCompat { area: string; status: string; between?: string[]; note?: string; basis?: string; source?: string }
export interface TechCriterion { name: string; score: number; reason: string; evidence?: string[] }
export interface TechFinding { category: string; severity: "info" | "warning" | "error"; area?: string; message: string; resolution?: string; source?: string }
export interface TechEvaluation { summary?: string; criteria: TechCriterion[]; findings: TechFinding[]; score: number; outdated?: boolean }
export interface TechCheck { area: string; status: string; note?: string; source?: string }
export interface TechCoverage { ref: string; areas: string[] }
export interface TechAnalysis {
  issues?: TechIssue[]; findings?: TechFinding[]; checks?: TechCheck[]; compat?: TechCompat[];
  items: number; outside_catalog: number; unverified: number; coverage?: TechCoverage[];
  diverged?: string[]; stale_areas?: string[]; affected?: string[];
}
export interface TechPlanVersion {
  version: number; at: string; source: string; request?: string; reason?: string; changes?: string[];
  status: TechStatus; items: number; score: number;
}
export interface TechPlanRequest { action: string; text?: string; reason?: string; areas?: string[]; source_hash?: string; rejected?: string }
export interface TechPlan {
  approach: string; summary: string; rationale?: string; context: TechContext; items: TechItem[];
  constraints?: TechConstraint[]; compatibility?: TechCompat[]; evaluation: TechEvaluation; consistency?: TechCheck[];
  history?: TechPlanVersion[]; sources?: Record<string, string>; generated_at?: string; stale?: boolean; analysis: TechAnalysis;
}
export interface TechStack {
  mode: "existing" | "new"; profile?: string; needs?: string[];
  platform: { web: boolean; mobile: boolean; android?: boolean; ios?: boolean; desktop: boolean; backend: boolean; cli?: boolean };
  decisions: TechDecision[]; approved: boolean; status?: TechStatus; issues?: TechIssue[]; proposal?: TechProposal;
  request?: string; history?: TechVersion[]; plan?: TechPlan; plan_request?: TechPlanRequest;
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
  source: "new" | "existing"; reference?: string; direction?: string; principles: string[] | null; ux_guidelines: string[] | null;
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
export interface UIUser { role: string; platform?: string; context?: string; goals?: string[]; tasks?: string[] }
export interface UIGoal { id: string; role?: string; text: string; success?: string; priority?: "must" | "should" | "could" }
export interface NavItem { label: string; page: string; primary?: boolean }
export interface NavGroup { name: string; items: NavItem[] }
export interface DesignDecision {
  subject: string; topic?: string; choice: string; why: string[]; basis?: string[];
  source: "fact" | "assumption" | "constraint"; alternative?: string;
}
export type FieldPresentation = "table-column" | "list-item" | "card-title" | "card-meta" | "card-value" |
  "delta" | "detail" | "form-input" | "filter" | "summary" | "chart" | "badge" | "row-action" | "hidden";
export interface FieldRule { field: string; label?: string; presentation: FieldPresentation; format?: string; note?: string }
export interface LayoutPage {
  id: string; name: string; platform: string; route?: string; role?: string; access?: "public" | "auth"; feature_id?: string;
  purpose: string; goal?: string; shell?: string; pattern: string; primary_action?: string; fields?: FieldRule[];
  sections: Section[] | null; hierarchy?: string[];
  grid?: string; responsive?: string[]; links?: { to: string; via: string }[]; states?: Partial<Record<UIState, string>>;
  data?: string[]; existing?: boolean;
}
// Layout specification: shells pages share, then pages of sections.
export interface LayoutSpec {
  shells: LayoutShell[] | null; pages: LayoutPage[] | null;
  users?: UIUser[]; goals?: UIGoal[]; navigation?: NavGroup[]; decisions?: DesignDecision[];
  assumptions?: string[]; unknowns?: string[];
}
// The UI design as text and structured data; never drawn or rendered.
export interface UIDesign {
  format?: number; rules?: DesignRules; layout?: LayoutSpec; request?: ChangeRequest; version: number; tech_version?: number; updated_at: string;
}
export interface DesignIssue { level: "error" | "warning" | "info"; where?: string; message: string; impact?: string; suggestion?: string }
export interface DesignView {
  design: UIDesign | null; issues: DesignIssue[]; has_ui: boolean; platforms: string[] | null;
  vocabulary: { patterns: string[]; states: UIState[]; required_states: Record<string, UIState[]>; platforms: string[];
    field_presentations?: string[]; decision_sources?: string[] };
}
export type VisualStatus = "pass" | "partial" | "fail" | "untested";
export interface VisualFinding {
  page: string; criterion: string; severity: "critical" | "high" | "medium" | "low";
  message: string; evidence?: string; fix?: string;
}
export interface VisualCriterion { id: string; title: string; status: VisualStatus; severity?: string; detail?: string }
export interface VisualPageAudit {
  page: string; url?: string; status: VisualStatus; previous_status?: VisualStatus; resolved?: string[];
  source?: string; checked_at: string; viewports?: string[]; screenshots?: string[];
  findings: VisualFinding[]; criteria: VisualCriterion[];
}
export interface VisualAudit {
  version: number; updated_at: string; source?: string;
  pages?: Record<string, VisualPageAudit>; reviews?: { at: string; page: string; source?: string; status: VisualStatus; findings: number; viewports?: string[] }[];
}
export interface VisualSummary {
  pages: number; pass: number; partial: number; fail: number; untested: number;
  open_findings: number; open_high: number; checked_at?: string; not_checked?: string[];
}
export interface JobStep {
  id: string; label: string; status: "pending" | "running" | "done" | "skipped" | "failed";
  started_at?: string; ended_at?: string;
}
export interface JobTiming { kind: string; ms: number; at: string }
export interface Job {
  kind: string; status: "running" | "done" | "failed" | "waiting_host";
  message: string; error?: string; started: string;
  steps?: JobStep[]; eta_ms?: number;
}
export interface Event { at: string; source: string; message: string }

export type BlueprintSectionStatus =
  | "pending" | "in_progress" | "validating" | "repairing" | "retrying"
  | "completed" | "failed" | "blocked" | "skipped";
export type BlueprintRunStatus = "running" | "completed" | "incomplete" | "failed" | "stopped";
export type BlueprintSkipKind = "not_applicable" | "not_required" | "already_satisfied" | "deferred";
export interface BlueprintTask {
  id: string; kind: "generate" | "retry" | "repair" | "validate"; title: string;
  status: "pending" | "running" | "completed" | "failed" | "canceled";
  attempt: number; execution_id?: string; depends_on?: string[]; error?: string;
  started_at?: string; ended_at?: string;
}
export interface BlueprintExecution {
  id: string; task_id: string; attempt: number;
  trigger: "initial" | "continuation" | "repair" | "retry" | "busy_retry";
  status: "running" | "succeeded" | "rejected" | "failed" | "canceled";
  cause?: string; error?: string; failure_kind?: string;
  started_at: string; ended_at?: string; duration_ms?: number;
}
export interface BlueprintSection {
  id: string; name: string; kind?: string;
  status: BlueprintSectionStatus;
  depends_on?: string[]; activity?: string;
  skip_kind?: BlueprintSkipKind; reason?: string;
  error?: string; failure_kind?: string; attempt?: number;
  validation?: "running" | "passed" | "failed"; validation_issues?: string[];
  started_at?: string; ended_at?: string;
  tasks?: BlueprintTask[]; executions?: BlueprintExecution[];
}
export interface BlueprintSummary {
  total: number; applicable: number; completed: number; active: number;
  pending: number; blocked: number; failed: number; skipped: number; percent: number;
  tasks_total: number; tasks_completed: number; tasks_active: number; tasks_failed: number;
  executions: number; retries: number;
}
export interface BlueprintETA { remaining_ms: number; samples: number; unknown_sections?: number }
export interface BlueprintPlanChange { version: number; from: number; to: number; reason: string; at: string }
export interface BlueprintEvent {
  event_id: string; run_id: string;
  section_id?: string; section_name?: string; task_id?: string; execution_id?: string;
  sequence_number: number; timestamp: string; activity_type: string;
  status?: string; attempt_number?: number; message?: string; detail?: string;
}
export interface BlueprintProgress {
  run_id: string; status: BlueprintRunStatus; activity?: string;
  started_at: string; ended_at?: string; seq: number; error?: string;
  parallel?: number; plan_version: number; plan_changes?: BlueprintPlanChange[];
  summary: BlueprintSummary; eta?: BlueprintETA; sections: BlueprintSection[];
}
// One push of the run: authoritative state plus the timeline after the cursor.
export interface ProgressFrame { progress: BlueprintProgress | null; events: BlueprintEvent[] | null }

export interface RecentProject { dir: string; name: string }

export interface Project {
  version: number; name: string; dir: string; stage: Stage; story: string;
  intent?: WorkIntent; change?: ChangePlan; survey?: Survey;
  discovery?: Discovery; feature_map?: FeatureMap; blueprint?: Blueprint; requirements: Requirement[] | null; features: Feature[] | null; flows: Flow[] | null;
  platform?: PlatformPlan; structure?: Structure; stack?: TechStack; ui?: UIDesign; visual?: VisualAudit;
  artifacts?: Artifact[]; artifact_request?: ChangeRequest; workflow?: Workflow;
  decisions?: Decision[]; area_skips?: AreaSkip[]; definition_of_done?: string[];
  // Decisions changed since the artifacts and commands were brought in line.
  decision_sync?: string[];
  blueprint_queue?: string[];
  blueprint_batch?: number;
  blueprint_started?: string;
  blueprint_progress?: BlueprintProgress;
  job_history?: JobTiming[];
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
  id: string; title: string; status: AreaStatusKind; artifacts: string[]; reason?: string; errors?: number; skippable?: boolean; topics?: string[];
}
export type ArtifactStateKind = "draft" | "valid" | "needs_review" | "finalized";
export interface ArtifactState { status: ArtifactStateKind; reason?: string }
export interface ChangeRequest { text: string; focus?: string[]; area?: string; rejected?: string; cascade?: boolean; sync?: boolean }
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
  id: string; workflow_id?: string; name: string; objective: string; context?: string; inputs?: string[]; instructions: string;
  depends_on?: string[]; artifacts?: ArtifactRef[]; covers?: string[]; feature_ids?: string[]; sub_feature_ids?: string[];
  expected_outputs?: string[]; acceptance_criteria?: string[];
  constraints?: string[]; files?: string[]; priority?: CommandLevel; complexity?: CommandLevel; revision?: number;
  status: CommandStatus; review?: string;
}
export type CommandLevel = "high" | "medium" | "low";
export interface CommandChange { id: string; revision?: number; fields?: string[]; before: Command }
// One revision of the workflow as a delta; commands not named were reused.
export interface WorkflowChange {
  version: number; at: string; kind: string; reason?: string; source?: string;
  added?: string[]; modified?: CommandChange[]; removed?: CommandChange[]; reused: number;
}
export interface Workflow {
  id?: string; status: "draft" | "finalized"; commands: Command[]; request?: ChangeRequest; version: number; finalized_at?: string;
  changes?: WorkflowChange[];
}
export interface WorkflowIssue { command?: string; message: string }
export interface WorkflowReport {
  issues: WorkflowIssue[]; waves: string[][]; stale: Record<string, string[]>; ready: boolean;
  review: Record<string, string>; docs: Record<string, { title: string; category: ArtifactCategory }>;
  // MUST requirements no command covers; shown, not blocking.
  uncovered: string[];
  // Feature id → command ids that implement it.
  feature_commands: Record<string, string[]>;
  // Features no command implements, and advice on the commands; neither blocks.
  uncovered_features: string[];
  quality: WorkflowIssue[];
}
export interface WorkflowView { workflow: Workflow | null; report: WorkflowReport; progress?: ProgressSnapshot }

export type RunStatus = "unconfirmed" | "waiting" | "started" | "blocked" | "skipped" | "failed" | "review" | "done" | "verified" | "stale" | "unknown" | "revision";
// Lifecycle of a command, derived by the backend from the reports.
export type CommandPhase = "created" | "ready" | "in_progress" | "updating" | "review" | "completed" | "blocked" | "failed" | "revision_required" | "skipped" | "stale";
export interface ProgressEvidence { kind?: string; ref: string }
export interface ProgressTest { name: string; result: string; detail?: string }
export interface ProgressWarning { command?: string; kind: string; message: string; action?: string }
export interface ProgressCounts {
  total: number; unconfirmed: number; waiting: number; started: number; blocked: number; skipped: number;
  failed: number; review: number; done: number; verified: number; stale: number; unknown: number; revision: number;
}
export interface ProgressOverview {
  total: number; completed: number; in_progress: number; pending: number; blocked: number; failed: number;
  review: number; revision: number; skipped: number; stale: number; percent: number;
}
export interface ProgressActivity {
  at: string; command: string; name?: string; status: string; applied?: string; outcome: string; summary?: string; reason?: string;
  files?: string[]; percent?: number; source?: string; session?: string;
}
export interface AgentSession { id: string; source?: string; command: string; last_at: string; active: boolean }
export interface CommandRevisionEntry {
  version: number; at: string; kind: string; reason?: string; source?: string; action: "add" | "modify"; revision: number;
  fields?: string[]; before?: Command;
}
export interface CommandHistory {
  command: string; reports: ProgressActivity[]; history: StatusChange[]; changes: CommandRevisionEntry[];
  evidence: ProgressEvidence[]; tests: ProgressTest[];
}
export interface CommandProgress {
  id: string; name: string; status: RunStatus; phase: CommandPhase; stored: RunStatus; wave: number; weight: number;
  revision: number; run_revision?: number; priority?: CommandLevel; complexity: CommandLevel; files?: string[];
  impact?: string[]; revision_why?: string; stale_docs?: string[]; activity?: string; percent?: number; session?: string;
  depends_on: string[]; dependents: string[]; feature_ids?: string[]; sub_feature_ids?: string[];
  waiting?: string[]; started_at?: string; finished_at?: string; updated_at?: string; source?: string; summary?: string; reason?: string;
  evidence?: ProgressEvidence[]; tests?: ProgressTest[]; early_start?: boolean; report_id?: string; seq?: number;
}
export type RollupStatus = "no_commands" | "not_started" | "in_progress" | "blocked" | "unknown" | "done";
export interface Rollup {
  percent: number; weight: number; done_weight: number; total: number;
  completed: number; claimed: number; in_progress: number; not_started: number;
  failed: number; blocked: number; skipped: number; unknown: number; revision: number;
  status: RollupStatus; last_status?: string; last_command?: string; last_at?: string;
}
export interface DependencyProgress { from: string; to: string; satisfied: boolean; status: RunStatus }
export interface WorkflowProgress extends Rollup { id: string; name: string; design_status: string; dependencies: DependencyProgress[] }
export interface ModuleProgress extends Rollup { id: string; name: string; feature_ids: string[] }
export interface FeatureProgress extends Rollup {
  id: string; name: string; module_id: string; module_name: string; feature_map_id?: string;
  commands: string[]; sub_features: SubFeatureProgress[];
}
export interface SubFeatureProgress extends Rollup {
  id: string; name: string; feature_id: string; feature_map_id?: string; commands: string[];
}
export interface WaveProgress { index: number; commands: string[]; verified: number; total: number }
export interface StatusChange { at: string; command: string; from: string; to: string; reason?: string; report_id?: string; source?: string }
export interface ProgressSnapshot {
  workflow_version: number; percent: number; weight: number; done_weight: number;
  overall: Rollup; workflow: WorkflowProgress; modules: ModuleProgress[]; features: FeatureProgress[];
  stale_after_sec: number; updated_at?: string; last_report_at?: string; last_source?: string;
  complete: boolean; counts: ProgressCounts; overview: ProgressOverview; activity: ProgressActivity[]; sessions: AgentSession[];
  waves: WaveProgress[]; commands: CommandProgress[];
  next: string[]; warnings: ProgressWarning[]; issues: string[]; history: StatusChange[];
  visual?: VisualSummary; visual_ready?: boolean;
}
export interface CommandEdit {
  name: string; objective: string; context: string; inputs: string[]; instructions: string; depends_on: string[];
  artifacts: string[]; covers: string[]; feature_ids: string[]; sub_feature_ids: string[];
  expected_outputs: string[]; acceptance_criteria: string[]; constraints: string[];
  files: string[]; priority: string; complexity: string;
}
export interface ExportManifest {
  format: string; project: string; status: "draft" | "finalized"; finalized_at?: string; blueprint_hash: string;
  workflow_version: number; entry: string; waves: string[][];
  artifacts: { id: string; title: string; category: string; file: string; hash: string }[];
  commands: { id: string; name: string; file: string; depends_on: string[]; artifacts: string[]; covers: string[]; feature_ids?: string[]; sub_feature_ids?: string[]; wave: number; files?: string[]; priority?: string; complexity?: string; revision?: number }[];
  areas: AreaStatus[];
}
export interface ExportView { dir: string; finalized: boolean; manifest: ExportManifest; files: { path: string; size: number }[]; blockers: string[] | null }
