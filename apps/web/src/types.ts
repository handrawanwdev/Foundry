// Mirrors internal/core/model.go — the single project context.

export type Stage = "TELL" | "DISCOVERY" | "DESIGN" | "PLAN" | "BUILD" | "VERIFY" | "READY";
export type TaskStatus = "TODO" | "IN_PROGRESS" | "REVIEW" | "DONE" | "BLOCKED";
export type PlatformKind = "web" | "android" | "ios" | "desktop" | "backend";

export interface Role { name: string; description: string; platform: string }
export interface Discovery {
  summary: string; business: string; users: string[]; roles: Role[];
  data: string[]; integrations: string[]; security: string[]; workflows: string[];
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
export interface Structure {
  mode: "existing" | "new"; components: Component[] | null; template?: string; layout?: string[];
  reason: string; commands: Command[] | null; approved: boolean; scaffolded: boolean;
}
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
export interface DesignHandoff { pages: PageHandoff[] | null; blockers: Blocker[] | null }
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
  domain?: string;
  qa?: DesignQA; // visual critic of a newly generated design
  quality?: DesignQuality;
  theme?: ThemeContext; // nil on designs made before themes
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
  status: TaskStatus; attempts: number; notes: string[] | null; last_build_id?: string; criteria?: Criterion[];
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
  message: string; error?: string; task_id?: string; started: string;
}
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
  discovery?: Discovery; requirements: Requirement[] | null; features: Feature[] | null; flows: Flow[] | null;
  platform?: PlatformPlan; structure?: Structure; design?: Design; theme?: string; reference?: DesignReference; tasks: Task[] | null;
  builds: BuildRun[] | null; verification?: Verification; job?: Job; support?: Support; events: Event[] | null; updated_at: string;
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
export interface FileEntry { name: string; path: string; is_dir: boolean; size: number }
export interface FileContent { path: string; size: number; binary: boolean; truncated: boolean; content?: string }
