// Package core holds the single project context shared by every stage.
// All modules read and write this model through Store; no stage keeps its
// own copy of project data.
package core

import (
	"encoding/json"
	"path"
	"strings"
	"time"
)

// Stage is the internal pipeline position. The UI groups these into
// Ceritakan → Rancang → Bangun → Siap.
type Stage string

const (
	StageTell      Stage = "TELL"
	StageDiscovery Stage = "DISCOVERY"
	StageDesign    Stage = "DESIGN"
	StagePlan      Stage = "PLAN"
	StageBuild     Stage = "BUILD"
	StageVerify    Stage = "VERIFY"
	StageReady     Stage = "READY"
)

type TaskStatus string

const (
	TaskTodo       TaskStatus = "TODO"
	TaskInProgress TaskStatus = "IN_PROGRESS"
	TaskReview     TaskStatus = "REVIEW"
	TaskDone       TaskStatus = "DONE"
	TaskBlocked    TaskStatus = "BLOCKED"
)

type Project struct {
	Version   int        `json:"version"`
	Name      string     `json:"name"`
	Dir       string     `json:"dir"`
	Stage     Stage      `json:"stage"`
	Story     string     `json:"story"`
	Discovery *Discovery `json:"discovery,omitempty"`
	// Blueprint is the production architecture decided during discovery.
	// Coding agents treat it as source of truth and do not invent another.
	Blueprint    *Blueprint    `json:"blueprint,omitempty"`
	Requirements []Requirement `json:"requirements"`
	Features     []Feature     `json:"features"`
	Flows        []Flow        `json:"flows"`
	Platform     *PlatformPlan `json:"platform,omitempty"`
	Structure    *Structure    `json:"structure,omitempty"`
	// Stack is the technology decision every later stage builds with.
	Stack  *TechStack `json:"stack,omitempty"`
	Design *Design    `json:"design,omitempty"`
	// Theme is the theme the user picked for the design; empty lets
	// Foundry select one from the requirement.
	Theme string `json:"theme,omitempty"`
	// DesignTemplate is the template the user picked as the design's
	// starting structure: empty lets the AI choose, TemplateBlank asks for
	// none. It applies when the design is generated.
	DesignTemplate string `json:"design_template,omitempty"`
	// Reference is the website or app the design takes inspiration from.
	Reference    *DesignReference `json:"reference,omitempty"`
	Tasks []Task `json:"tasks"`
	// PRD is the product source of truth: capability → business flow →
	// technical contract → task. Generated from the project context.
	PRD          *PRD          `json:"prd,omitempty"`
	Builds       []BuildRun    `json:"builds"`
	Verification *Verification `json:"verification,omitempty"`
	Job          *Job             `json:"job,omitempty"`
	Support      *Support         `json:"support,omitempty"`
	Events       []Event          `json:"events"`
	UpdatedAt    time.Time        `json:"updated_at"`
}

type Discovery struct {
	Summary      string   `json:"summary"`
	Business     string   `json:"business"`
	Users        []string `json:"users"`
	Roles        []Role   `json:"roles"`
	Data         []string `json:"data"`
	Integrations []string `json:"integrations"`
	Security     []string `json:"security"`
	Workflows    []string `json:"workflows"`
	// TechNeeds are the technical signals the analyst found (realtime,
	// offline, seo, ai, ...); the technology decision starts from them.
	TechNeeds []string `json:"tech_needs,omitempty"`
}

type Role struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Platform    string `json:"platform"`
}

type Requirement struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	Priority string `json:"priority"` // MUST | SHOULD | COULD
}

// PRD is the product requirements document derived from the project
// context. Markdown is the document humans and agents read; Findings
// are gaps between requirement, flow and task.
type PRD struct {
	Markdown  string       `json:"markdown"`
	Findings  []PRDFinding `json:"findings,omitempty"`
	Generated time.Time    `json:"generated"`
}

// PRDFinding kinds: GAP | UNASSIGNED | MISSING_TASK | DUPLICATE | OVERLAP | INVALID_DEPENDENCY.
type PRDFinding struct {
	Kind    string `json:"kind"`
	Ref     string `json:"ref"`
	Message string `json:"message"`
}

type Feature struct {
	ID             string       `json:"id"`
	Name           string       `json:"name"`
	Description    string       `json:"description"`
	RequirementIDs []string     `json:"requirement_ids"`
	Platforms      []string     `json:"platforms"`
	SubFeatures    []SubFeature `json:"sub_features"`
}

type SubFeature struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Flow struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	FeatureID string   `json:"feature_id"`
	Role      string   `json:"role"`
	Steps     []string `json:"steps"`
	PageIDs   []string `json:"page_ids"`
}

// Platform kinds.
const (
	PlatformWeb     = "web"
	PlatformAndroid = "android"
	PlatformIOS     = "ios"
	PlatformDesktop = "desktop"
	PlatformBackend = "backend"
)

type PlatformPlan struct {
	Targets  []PlatformTarget `json:"targets"`
	Reason   string           `json:"reason"`
	Approved bool             `json:"approved"`
}

type PlatformTarget struct {
	Kind      string   `json:"kind"`
	Audience  []string `json:"audience"`
	Rationale string   `json:"rationale"`
}

type StructureMode string

const (
	StructureExisting StructureMode = "existing"
	StructureNew      StructureMode = "new"
)

type Structure struct {
	Mode       StructureMode `json:"mode"`
	Components []Component   `json:"components"` // detected (existing) or planned (new)
	// Template is the structure type of a new project (IR.Type); Layout
	// the folders derived from IR.
	Template string    `json:"template,omitempty"`
	Layout   []string  `json:"layout,omitempty"`
	Reason   string    `json:"reason"`
	Commands []Command `json:"commands"`
	// IR is the structure decision of a new project, the build contract.
	IR *StructureIR `json:"ir,omitempty"`
	// Recommended is the type Foundry recommends for the current needs.
	Recommended string             `json:"recommended,omitempty"`
	Status      TechStatus         `json:"status,omitempty"`
	Issues      []TechIssue        `json:"issues,omitempty"`
	Proposal    *StructureProposal `json:"proposal,omitempty"`
	// Request is a natural-language change waiting for the AI.
	Request string             `json:"request,omitempty"`
	History []StructureVersion `json:"history,omitempty"`
	// Approved mirrors Status == approved for older readers.
	Approved   bool `json:"approved"`
	Scaffolded bool `json:"scaffolded"`
	// UI is what the existing frontend already has; nil for new projects
	// or when no frontend was found.
	UI *ExistingUI `json:"ui,omitempty"`
}

// ExistingUI is the frontend an existing project already has, read from
// its code: new pages are designed to fit into it, not beside it.
type ExistingUI struct {
	// Paths are the frontend components that were read.
	Paths  []string `json:"paths"`
	Routes []string `json:"routes,omitempty"`
	// Shell are the layout and navigation files every page sits in.
	Shell []string `json:"shell,omitempty"`
	// Navigation is side or top, "" when unknown; NavItems its menu labels.
	Navigation string   `json:"navigation,omitempty"`
	NavItems   []string `json:"nav_items,omitempty"`
	// Components are the shared UI components ("Button (src/components/ui/button.tsx)").
	Components []string `json:"components,omitempty"`
	// ThemeID is the theme generated from the code's design tokens;
	// ThemeSources the files they came from.
	ThemeID      string   `json:"theme_id,omitempty"`
	ThemeSources []string `json:"theme_sources,omitempty"`
}

type Component struct {
	Kind     string `json:"kind"` // backend | web | mobile | desktop | shared | database | infra | testing
	Path     string `json:"path"`
	Tech     string `json:"tech"`
	Evidence string `json:"evidence"`
}

// Command is a verification step detected from real project tooling.
type Command struct {
	Name string   `json:"name"` // build | typecheck | lint | vet | test
	Dir  string   `json:"dir"`  // relative to project dir
	Argv []string `json:"argv"`
}

type DesignStatus string

const (
	DesignDraft    DesignStatus = "draft"
	DesignChanges  DesignStatus = "changes_requested"
	DesignApproved DesignStatus = "approved"
)

// MarshalJSON writes nil Pages as [] so clients never see "pages": null.
func (d Design) MarshalJSON() ([]byte, error) {
	if d.Pages == nil {
		d.Pages = []Page{}
	}
	type plain Design
	return json.Marshal(plain(d))
}

type Design struct {
	// System is the design system every page and the build follow.
	System   *DesignSystem `json:"system,omitempty"`
	Pages    []Page        `json:"pages"`
	Status   DesignStatus  `json:"status"`
	Feedback []string      `json:"feedback"`
	// Revision is the last requested change, waiting for the user to keep or
	// undo it. Nil when there is nothing to review.
	Revision *DesignRevision `json:"revision,omitempty"`
	// Screens is what the user saw on each page (page id → outline read from
	// the rendered wireframe) when asking for the last change, so the AI
	// works from the same screen.
	Screens map[string]string `json:"screens,omitempty"`
	// Scope is what the last change request may touch; Foundry locks it and
	// rejects a patch that goes outside it.
	Scope *ChangeScope `json:"scope,omitempty"`
	// Rejected is why the last patch was refused; the retry prompt shows it.
	Rejected string `json:"rejected,omitempty"`
	// TechVersion is the technology version the design was last checked
	// against; a later version that affects the design asks for a review.
	TechVersion int `json:"tech_version,omitempty"`
	// Domain is the product domain the design decisions follow (ticketing,
	// trading, ...); empty when none fits.
	Domain string `json:"domain,omitempty"`
	// QA is the visual critic of a newly generated design; a revision keeps
	// its own check in Revision.QA.
	QA *DesignQA `json:"qa,omitempty"`
	// Quality is the last measurement of the design: rule metrics on the
	// Design IR plus the critic's findings. An internal indicator, not a
	// ranking.
	Quality *DesignQuality `json:"quality,omitempty"`
	// Theme is the theme the design follows: System holds its tokens, the
	// renderer reads its style. Nil on designs made before themes.
	Theme *ThemeContext `json:"theme,omitempty"`
	// Template is the UX pattern the pages are structured on. Nil on
	// designs made before templates or when none fits.
	Template *TemplateChoice `json:"template,omitempty"`
}

// DesignQuality is the measured quality of the current design.
type DesignQuality struct {
	Overall    int            `json:"overall"`
	Dimensions map[string]int `json:"dimensions"`
	Pages      []PageQuality  `json:"pages,omitempty"`
	Findings   []QAIssue      `json:"findings,omitempty"`
	// Confidence: low (rules on an IR without intent/plan), medium (rules
	// only), high (rules plus the critic on rendered pages).
	Confidence string `json:"confidence"`
	// Decision is the quality gate: pass | revision_required. Reasons are
	// the blocker findings that require a revision.
	Decision       string             `json:"decision"`
	Reasons        []QAIssue          `json:"reasons,omitempty"`
	Compared       *QualityComparison `json:"compared_to_previous,omitempty"`
	Recommendation string             `json:"recommendation,omitempty"`
	MeasuredAt     time.Time          `json:"measured_at"`
}

type PageQuality struct {
	PageID     string         `json:"page_id"`
	Overall    int            `json:"overall"`
	Dimensions map[string]int `json:"dimensions"`
	// Metrics are the deterministic counts measured on the IR.
	Metrics map[string]int `json:"metrics,omitempty"`
}

// QualityScore is the rule score of one design version.
type QualityScore struct {
	Overall    int            `json:"overall"`
	Dimensions map[string]int `json:"dimensions"`
	Blockers   int            `json:"blockers"`
}

// QualityComparison is the rule score before and after a critic patch.
// Verdict: improved | neutral | degraded; a degraded patch is rejected.
type QualityComparison struct {
	Round    int          `json:"round"`
	Before   QualityScore `json:"before"`
	After    QualityScore `json:"after"`
	Verdict  string       `json:"verdict"`
	Rejected bool         `json:"rejected,omitempty"`
}

// DesignSystem holds the design tokens: the build maps them onto the
// project's styling instead of arbitrary values.
type DesignSystem struct {
	// Direction is the one visual direction the tokens follow
	// (e.g. "Operations: dense, restrained, semantic color only").
	Direction   string  `json:"direction,omitempty"`
	Colors      []Token `json:"colors"`
	Typography  []Token `json:"typography"`
	Spacing     []Token `json:"spacing"`
	Radius      []Token `json:"radius,omitempty"`
	Shadow      []Token `json:"shadow,omitempty"`
	Breakpoints []Token `json:"breakpoints,omitempty"`
	// Components: component name → how it is used (variants, rules).
	Components []Token `json:"components,omitempty"`
}

type Token struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	Use   string `json:"use,omitempty"`
}

// ChangeScope limits a change request. Pages lists the pages the user asked
// about; Open also allows every other page (a general request).
type ChangeScope struct {
	Open    bool        `json:"open,omitempty"`
	Fields  bool        `json:"fields,omitempty"` // page fields may change on every page
	General string      `json:"general,omitempty"`
	Pages   []ScopePage `json:"pages,omitempty"`
}

type ScopePage struct {
	ID      string `json:"id"`
	Request string `json:"request,omitempty"`
	// Components picked in the inspector: only these may be removed, moved
	// or updated on this page. Empty means any component of the page.
	Components []int `json:"components,omitempty"`
	// Fields allows page fields (kind, route, access, data, states, ...).
	Fields bool   `json:"fields,omitempty"`
	Device string `json:"device,omitempty"` // desktop | tablet | mobile the user sees
}

// Page returns the scope entry of a page, or nil.
func (s *ChangeScope) Page(id string) *ScopePage {
	if s == nil {
		return nil
	}
	for i := range s.Pages {
		if s.Pages[i].ID == id {
			return &s.Pages[i]
		}
	}
	return nil
}

// DesignRevision records what one change request did to the design and the
// state needed to undo it.
type DesignRevision struct {
	Request       string              `json:"request"`
	Changes       []DesignChange      `json:"changes"`
	PreviousPages []Page              `json:"previous_pages"`
	PreviousFlows map[string][]string `json:"previous_flows"` // flow id -> page ids
	// PreviousSystem is the design system before the request; undo restores it.
	PreviousSystem *DesignSystem `json:"previous_system,omitempty"`
	Scope          *ChangeScope  `json:"scope,omitempty"`
	Patch          []string      `json:"patch,omitempty"` // operations applied, readable
	QA             *DesignQA     `json:"qa,omitempty"`
}

// DesignQA is the visual check of a revision: the changed pages are
// rendered in a browser at the user's viewport and reviewed by the AI.
type DesignQA struct {
	Status string    `json:"status"` // running | pass | fixed | failed | skipped
	Round  int       `json:"round"`
	Notes  string    `json:"notes,omitempty"`
	Issues []QAIssue `json:"issues,omitempty"`
	Shots  []QAShot  `json:"shots,omitempty"`
	// Next are the page/device pairs the next round renders again: only
	// what the last fix touched, on the device its findings were about.
	Next []QAShot `json:"next,omitempty"`
	// Compared is the rule score before and after each fix patch.
	Compared []QualityComparison `json:"compared,omitempty"`
}

// QAIssue is one problem the visual critic saw on a screenshot.
type QAIssue struct {
	PageID   string `json:"page_id,omitempty"`
	Severity string `json:"severity"` // critical | major | minor
	Problem  string `json:"problem"`
	Why      string `json:"why,omitempty"`
	Impact   string `json:"impact,omitempty"`
	Fix      string `json:"fix,omitempty"`
	// Category is the kind of problem (hierarchy, spacing, ...); Component
	// the id of the component it is about.
	Category  string `json:"category,omitempty"`
	Component string `json:"component,omitempty"`
	// Device is the screenshot the problem was seen on; Source is "lint"
	// for findings of the deterministic design rules, empty for the critic.
	Device string `json:"device,omitempty"`
	Source string `json:"source,omitempty"`
	// Rule is the id of the design rule a lint finding comes from.
	Rule string `json:"rule,omitempty"`
}

type QAShot struct {
	PageID  string `json:"page_id"`
	Device  string `json:"device"`
	Image   string `json:"image"`   // screenshot path inside the project
	Outline string `json:"outline"` // what the wireframe drew, as text
}

// DesignChange is one page-level difference between two design versions.
type DesignChange struct {
	Kind    string   `json:"kind"`              // added | changed | removed
	PageID  string   `json:"page_id,omitempty"` // id in the new design; empty when removed
	Page    string   `json:"page"`
	Details []string `json:"details,omitempty"`
}

type Page struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Platform      string   `json:"platform"`
	Role          string   `json:"role"`
	FeatureID     string   `json:"feature_id"`
	SubFeatureIDs []string `json:"sub_feature_ids"`
	Purpose       string   `json:"purpose"`
	// Screen intent: what the user must see first (most important first),
	// the lower-weight actions besides the primary one, and how the layout
	// adapts on tablet and mobile.
	Hierarchy        []string `json:"hierarchy,omitempty"`
	SecondaryActions []string `json:"secondary_actions,omitempty"`
	Responsive       string   `json:"responsive,omitempty"`
	// Kind is the page's job (auth, list, form, ...); Density is
	// compact | comfortable | spacious. Kind may be empty on older designs.
	Kind    string      `json:"kind,omitempty"`
	Density string      `json:"density,omitempty"`
	States  *PageStates `json:"states,omitempty"`
	// Build contract: where the page lives, who may open it, what it is
	// for and what data it needs. Required before the design is approved.
	Route         string      `json:"route,omitempty"`
	Access        string      `json:"access,omitempty"` // public | auth
	PrimaryAction *PageAction `json:"primary_action,omitempty"`
	Data          *PageData   `json:"data,omitempty"`
	// Intent is how the page is used and Plan the design decisions taken
	// from it before the layout was composed. Both may be nil on older
	// designs.
	Intent *PageIntent `json:"intent,omitempty"`
	Plan   *PagePlan   `json:"plan,omitempty"`
	// Layout is the explicit composition (Design IR): frame, component
	// tree and per-device changes. The renderer draws only this.
	Layout     *PageLayout     `json:"layout,omitempty"`
	Components []PageComponent `json:"components"`
}

// PageIntent is what the designer must understand before choosing a
// layout. The user is Page.Role and the goal Page.Purpose.
type PageIntent struct {
	PrimaryTask        string `json:"primary_task,omitempty"`        // authenticate | enter | configure | search | browse | compare | process | monitor | inspect | analyze | overview
	Frequency          string `json:"frequency,omitempty"`           // low | medium | high
	Complexity         string `json:"complexity,omitempty"`          // low | medium | high
	InformationDensity string `json:"information_density,omitempty"` // low | medium | high
	Comparison         bool   `json:"comparison,omitempty"`          // the user compares many records
}

// PagePlan is the design decision recorded before the layout: the layout
// and responsive strategy and the information hierarchy by component id.
// Page type, density and navigation live in Kind, Density and Layout.Nav.
type PagePlan struct {
	Layout     string   `json:"layout_strategy,omitempty"`
	Responsive string   `json:"responsive_strategy,omitempty"`
	Primary    string   `json:"primary_focus,omitempty"`
	Secondary  []string `json:"secondary_content,omitempty"`
	Supporting []string `json:"supporting_content,omitempty"`
	// Reasons: component or node id → why it is on the page.
	Reasons map[string]string `json:"reasons,omitempty"`
}

// PageAction is the main thing the user does on a page. A read-only page has
// no label and gives the reason instead.
type PageAction struct {
	Label  string `json:"label,omitempty"`
	Target string `json:"target,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// PageData is the content a page needs; API lists "METHOD /path" calls.
type PageData struct {
	Required []string `json:"required,omitempty"`
	Optional []string `json:"optional,omitempty"`
	Dynamic  []string `json:"dynamic,omitempty"`
	API      []string `json:"api,omitempty"`
}

// PageStates holds the copy shown per page state. NoResults is a search or
// filter that matched nothing, as opposed to Empty (no data at all).
type PageStates struct {
	Loading   string `json:"loading,omitempty"`
	Empty     string `json:"empty,omitempty"`
	NoResults string `json:"no_results,omitempty"`
	Error     string `json:"error,omitempty"`
	Success   string `json:"success,omitempty"`
}

// PageComponent is a wireframe element. ID is what the layout places.
// Target links to another page for prototype navigation. Emphasis and Tone
// are semantic tokens (action.*, text.* / status.*), never raw values.
type PageComponent struct {
	ID       string      `json:"id,omitempty"`
	Type     string      `json:"type"` // header | nav | text | list | table | form | button | card | stat | input | section | steps
	Emphasis string      `json:"emphasis,omitempty"`
	Tone     string      `json:"tone,omitempty"`
	Label    string      `json:"label"`
	Items    []string    `json:"items,omitempty"`
	Columns  []string    `json:"columns,omitempty"` // table only
	Fields   []FormField `json:"fields,omitempty"`  // form only
	Target   string      `json:"target,omitempty"`
}

// FormField specifies one form input for the build phase.
type FormField struct {
	Label      string `json:"label"`
	Type       string `json:"type"`
	Required   bool   `json:"required,omitempty"`
	Validation string `json:"validation,omitempty"`
	Error      string `json:"error,omitempty"`
	Group      string `json:"group,omitempty"`
}

type Task struct {
	ID                 string     `json:"id"`
	Title              string     `json:"title"`
	Description        string     `json:"description"`
	Area               string     `json:"area"` // scaffold | data | api | ui | test | integration
	RequirementIDs     []string   `json:"requirement_ids"`
	FeatureID          string     `json:"feature_id"`
	PageIDs            []string   `json:"page_ids"`
	Platform           string     `json:"platform"`
	DependsOn          []string   `json:"depends_on"`
	AcceptanceCriteria []string   `json:"acceptance_criteria"`
	Files              []string   `json:"files"`
	TestRequirements   []string   `json:"test_requirements"`
	Status             TaskStatus `json:"status"`
	Attempts           int        `json:"attempts"`
	// Conflicts counts merge clashes that were not charged as attempts.
	Conflicts int `json:"conflicts,omitempty"`
	// Owner is the agent or MCP worker that claimed the task last.
	Owner string `json:"owner,omitempty"`
	// Seen is the owner's last sign of life (claim, next_work, update_task);
	// an MCP host silent past the lease loses the task.
	Seen time.Time `json:"seen,omitempty"`
	// Progress is a short live line on the Kanban while the task is active.
	Progress string `json:"progress,omitempty"`
	// StartedAt is when the current (or last) work attempt was claimed.
	StartedAt time.Time `json:"started_at,omitempty"`
	// FinishedAt is when the task last became DONE.
	FinishedAt time.Time `json:"finished_at,omitempty"`
	// Closed is a user gate: a closed task is skipped by the build queue.
	Closed      bool        `json:"closed,omitempty"`
	Notes       []string    `json:"notes"`
	LastBuildID string      `json:"last_build_id,omitempty"`
	Criteria    []Criterion `json:"criteria,omitempty"`
}

type Criterion struct {
	Text     string `json:"text"`
	Met      bool   `json:"met"`
	Evidence string `json:"evidence"`
}

type BuildRun struct {
	ID     string       `json:"id"`
	TaskID string       `json:"task_id,omitempty"`
	Steps  []StepResult `json:"steps"`
	Passed bool         `json:"passed"`
	Reason string       `json:"reason,omitempty"`
	At     time.Time    `json:"at"`
}

type StepResult struct {
	Name     string `json:"name"`
	Command  string `json:"command"`
	Dir      string `json:"dir"`
	ExitCode int    `json:"exit_code"`
	Output   string `json:"output"`
	Millis   int64  `json:"millis"`
}

type Verification struct {
	Ready  bool      `json:"ready"`
	Checks []Check   `json:"checks"`
	Issues []Issue   `json:"issues"`
	Trace  []Trace   `json:"trace"`
	At     time.Time `json:"at"`
}

type Check struct {
	Name string `json:"name"`
	OK   bool   `json:"ok"`
	Info string `json:"info"`
}

type Issue struct {
	Kind    string `json:"kind"` // missing_feature | missing_page | missing_task | missing_test | task_open | task_blocked | build_failed | test_failed | no_commands | broken_flow
	Ref     string `json:"ref"`
	Message string `json:"message"`
}

// Trace is one requirement's chain down to verification.
type Trace struct {
	RequirementID string   `json:"requirement_id"`
	FeatureIDs    []string `json:"feature_ids"`
	PageIDs       []string `json:"page_ids"`
	TaskIDs       []string `json:"task_ids"`
	TestTaskIDs   []string `json:"test_task_ids"`
	Done          bool     `json:"done"`
}

type JobStatus string

const (
	JobRunning JobStatus = "running"
	JobDone    JobStatus = "done"
	JobFailed  JobStatus = "failed"
	JobWaiting JobStatus = "waiting_host" // MCP mode: waiting for the AI host
)

type Job struct {
	Kind    string    `json:"kind"` // discovery | design | plan | build | verify | fix
	Status  JobStatus `json:"status"`
	Message string    `json:"message"`
	Error   string    `json:"error,omitempty"`
	TaskID  string    `json:"task_id,omitempty"`
	TaskIDs []string  `json:"task_ids,omitempty"` // IN_PROGRESS / REVIEW during a parallel build
	Started time.Time `json:"started"`
}

// Queue states for a TODO task in the live build schedule.
const (
	QueueIndependent = "independent"  // deps done, files/pages free, no overlap with others in this set
	QueueQueued      = "queued"       // deps done, but a file/page is held (or would be) by another task
	QueueWaitingDeps = "waiting_deps" // a depends_on task is not DONE
)

// QueueItem is why one TODO task is or is not in the parallel set.
type QueueItem struct {
	ID        string   `json:"id"`
	State     string   `json:"state"` // independent | queued | waiting_deps
	BlockedBy string   `json:"blocked_by,omitempty"`
	Lock      string   `json:"lock,omitempty"`
	Waiting   []string `json:"waiting,omitempty"`
}

// Schedule is the live build queue. It is computed, never persisted.
type Schedule struct {
	Independent []string    `json:"independent"`
	Items       []QueueItem `json:"items,omitempty"`
}

// Support is the Technical Support Assistant: a conversation about the system
// being built, answered only from the project context and its files.
type Support struct {
	Messages    []SupportMessage `json:"messages"`
	Escalations []Escalation     `json:"escalations,omitempty"`
}

// Support message statuses (assistant messages only).
const (
	SupportRunning = "running" // the agent is answering
	SupportWaiting = "waiting" // waiting for the MCP host
	SupportDone    = "done"
	SupportFailed  = "failed"
)

type SupportMessage struct {
	ID     string         `json:"id"`
	Role   string         `json:"role"` // user | assistant
	Text   string         `json:"text"` // the question (user)
	Status string         `json:"status,omitempty"`
	Error  string         `json:"error,omitempty"`
	Answer *SupportAnswer `json:"answer,omitempty"`
	At     time.Time      `json:"at"`
}

// SupportAnswer is one grounded answer. Sources are project ids (FLOW-001,
// PAGE-002, ...) or project files; unknown references are dropped.
type SupportAnswer struct {
	Kind      string        `json:"kind"` // explain | flow | troubleshoot | roles | unknown
	Summary   string        `json:"summary"`
	Flow      *SupportFlow  `json:"flow,omitempty"`
	Trouble   *Troubleshoot `json:"trouble,omitempty"`
	Roles     []RoleAccess  `json:"roles,omitempty"`
	Missing   []string      `json:"missing,omitempty"` // what the system does not define
	Sources   []string      `json:"sources,omitempty"`
	FollowUps []string      `json:"follow_ups,omitempty"`
	Escalate  bool          `json:"escalate,omitempty"` // the assistant cannot resolve it
}

type SupportFlow struct {
	FlowID string     `json:"flow_id,omitempty"`
	Name   string     `json:"name"`
	Steps  []FlowStep `json:"steps"`
}

type FlowStep struct {
	Name    string `json:"name"`
	Detail  string `json:"detail,omitempty"`
	Role    string `json:"role,omitempty"`
	Module  string `json:"module,omitempty"`
	PageID  string `json:"page_id,omitempty"`
	API     string `json:"api,omitempty"`
	Current bool   `json:"current,omitempty"` // where the reported problem is
}

type Troubleshoot struct {
	Problem string   `json:"problem"`
	Status  string   `json:"status"`
	Module  string   `json:"module,omitempty"`
	Step    string   `json:"step,omitempty"`
	Causes  []Cause  `json:"causes,omitempty"`
	Actions []string `json:"actions,omitempty"`
}

// Cause is confirmed only with evidence; otherwise it is a possibility.
type Cause struct {
	Text      string `json:"text"`
	Evidence  string `json:"evidence,omitempty"`
	Confirmed bool   `json:"confirmed,omitempty"`
}

type RoleAccess struct {
	Role   string `json:"role"`
	Access string `json:"access"`
	Source string `json:"source,omitempty"`
}

// Escalation is a problem handed to the technical team, as confirmed by the
// user from the conversation.
type Escalation struct {
	ID            string    `json:"id"`
	Issue         string    `json:"issue"`
	Module        string    `json:"module,omitempty"`
	Flow          string    `json:"flow,omitempty"`
	Status        string    `json:"status,omitempty"`
	Evidence      []string  `json:"evidence,omitempty"`
	Causes        []string  `json:"causes,omitempty"`
	Investigation []string  `json:"investigation,omitempty"`
	At            time.Time `json:"at"`
}

type Event struct {
	At      time.Time `json:"at"`
	Source  string    `json:"source"` // ui | agent | mcp | system
	Message string    `json:"message"`
}

// Log appends an audit event, keeping the log bounded.
func (p *Project) Log(source, msg string) {
	p.Events = append(p.Events, Event{At: time.Now().UTC(), Source: source, Message: msg})
	if n := len(p.Events); n > 300 {
		p.Events = p.Events[n-300:]
	}
}

func (p *Project) Task(id string) *Task {
	for i := range p.Tasks {
		if p.Tasks[i].ID == id {
			return &p.Tasks[i]
		}
	}
	return nil
}

func (p *Project) Build(id string) *BuildRun {
	for i := range p.Builds {
		if p.Builds[i].ID == id {
			return &p.Builds[i]
		}
	}
	return nil
}

func (p *Project) Feature(id string) *Feature {
	for i := range p.Features {
		if p.Features[i].ID == id {
			return &p.Features[i]
		}
	}
	return nil
}

func (p *Project) Page(id string) *Page {
	if p.Design == nil {
		return nil
	}
	for i := range p.Design.Pages {
		if p.Design.Pages[i].ID == id {
			return &p.Design.Pages[i]
		}
	}
	return nil
}

// ReadyTasks returns TODO tasks whose dependencies are all DONE.
func (p *Project) ReadyTasks() []*Task {
	var out []*Task
	for i := range p.Tasks {
		t := &p.Tasks[i]
		if t.Status != TaskTodo || t.Closed {
			continue
		}
		ok := true
		for _, d := range t.DependsOn {
			if dt := p.Task(d); dt == nil || dt.Status != TaskDone {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, t)
		}
	}
	return out
}

// Busy reports whether a job currently owns the project.
func (p *Project) Busy() bool {
	return p.Job != nil && (p.Job.Status == JobRunning)
}

// DesignVersion is one saved state of the design: the pages (Design IR),
// the design system, the theme and the pages of each flow. Versions live in
// .foundry/design-versions.json, apart from project.json; restoring one is
// a design change like any other.
type DesignVersion struct {
	ID       int                 `json:"id"`
	Source   string              `json:"source"` // generate | revision | qa | reference | theme | canvas | keep | undo | restore
	Note     string              `json:"note,omitempty"`
	Hash     string              `json:"hash"`
	Quality  int                 `json:"quality,omitempty"`
	At       time.Time           `json:"at"`
	Pages    []Page              `json:"pages"`
	System   *DesignSystem       `json:"system,omitempty"`
	Theme    *ThemeContext       `json:"theme,omitempty"`
	Flows    map[string][]string `json:"flows,omitempty"`
	Template *TemplateChoice     `json:"template,omitempty"`
}

// MarshalJSON writes nil Pages as [] so clients never see "pages": null.
func (v DesignVersion) MarshalJSON() ([]byte, error) {
	if v.Pages == nil {
		v.Pages = []Page{}
	}
	type plain DesignVersion
	return json.Marshal(plain(v))
}

// LockAll is held by a task that may touch anything in the project.
const LockAll = "*"

// Locks names what a task holds while it runs: its pages and files, else its
// feature. Scaffold and integration work, or a task that names nothing,
// holds the whole project.
func (t *Task) Locks() []string {
	if t.Area == "scaffold" || t.Area == "integration" {
		return []string{LockAll}
	}
	var out []string
	for _, pg := range t.PageIDs {
		out = append(out, "page:"+pg)
	}
	for _, f := range t.Files {
		f = strings.TrimPrefix(path.Clean(strings.ReplaceAll(strings.TrimSpace(f), "\\", "/")), "./")
		if f != "" && f != "." && f != "/" {
			out = append(out, "file:"+strings.ToLower(strings.TrimSuffix(f, "/")))
		}
	}
	if len(out) == 0 && t.FeatureID != "" {
		out = append(out, "feature:"+t.FeatureID)
	}
	if len(out) == 0 {
		return []string{LockAll}
	}
	return out
}

// lockOverlap: same key, the project lock, or one file path inside the other.
func lockOverlap(a, b string) bool {
	if a == LockAll || b == LockAll || a == b {
		return true
	}
	if strings.HasPrefix(a, "file:") && strings.HasPrefix(b, "file:") {
		return strings.HasPrefix(a, b+"/") || strings.HasPrefix(b, a+"/")
	}
	return false
}

// Active reports whether a task is being worked on and holds its locks.
func (t *Task) Active() bool {
	return t.Status == TaskInProgress || t.Status == TaskReview
}

// Blocker returns the active task holding a lock t needs, and that lock.
func (p *Project) Blocker(t *Task) (*Task, string) {
	mine := t.Locks()
	for i := range p.Tasks {
		o := &p.Tasks[i]
		if o.ID == t.ID || !o.Active() {
			continue
		}
		for _, b := range o.Locks() {
			for _, a := range mine {
				if lockOverlap(a, b) {
					return o, b
				}
			}
		}
	}
	return nil, ""
}

// ClaimableTasks are ready tasks no active task blocks, in plan order.
func (p *Project) ClaimableTasks() []*Task {
	var out []*Task
	for _, t := range p.ReadyTasks() {
		if o, _ := p.Blocker(t); o == nil {
			out = append(out, t)
		}
	}
	return out
}

// overlapLock is the lock key two tasks share, if any.
func overlapLock(a, b *Task) (string, bool) {
	if a == nil || b == nil {
		return "", false
	}
	for _, x := range a.Locks() {
		for _, y := range b.Locks() {
			if lockOverlap(x, y) {
				return y, true
			}
		}
	}
	return "", false
}

// IndependentTasks are claimable tasks that also do not overlap each other,
// in plan order. These can run in parallel without file or page conflicts.
func (p *Project) IndependentTasks() []*Task {
	var out []*Task
	for _, t := range p.ClaimableTasks() {
		ok := true
		for _, o := range out {
			if _, hit := overlapLock(t, o); hit {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, t)
		}
	}
	return out
}

// Schedule is the live TODO queue: which tasks can start in parallel now,
// which wait on a file/page held by another agent, and which wait on deps.
func (p *Project) Schedule() Schedule {
	indep := p.IndependentTasks()
	in := make(map[string]*Task, len(indep))
	ids := make([]string, len(indep))
	for i, t := range indep {
		ids[i] = t.ID
		in[t.ID] = t
	}
	var items []QueueItem
	for i := range p.Tasks {
		t := &p.Tasks[i]
		if t.Status != TaskTodo || t.Closed {
			continue
		}
		if in[t.ID] != nil {
			items = append(items, QueueItem{ID: t.ID, State: QueueIndependent})
			continue
		}
		var wait []string
		for _, d := range t.DependsOn {
			if dt := p.Task(d); dt == nil || dt.Status != TaskDone {
				wait = append(wait, d)
			}
		}
		if len(wait) > 0 {
			items = append(items, QueueItem{ID: t.ID, State: QueueWaitingDeps, Waiting: wait})
			continue
		}
		o, lock := p.Blocker(t)
		if o == nil {
			for _, x := range indep {
				if l, hit := overlapLock(t, x); hit {
					o, lock = x, l
					break
				}
			}
		}
		item := QueueItem{ID: t.ID, State: QueueQueued}
		if o != nil {
			item.BlockedBy = o.ID
			item.Lock = lock
		}
		items = append(items, item)
	}
	return Schedule{Independent: ids, Items: items}
}

// ActiveTasks are the tasks currently holding locks.
func (p *Project) ActiveTasks() []*Task {
	var out []*Task
	for i := range p.Tasks {
		if p.Tasks[i].Active() {
			out = append(out, &p.Tasks[i])
		}
	}
	return out
}
