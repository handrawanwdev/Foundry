// Package core holds the single project context shared by every stage.
// All modules read and write this model through Store; no stage keeps its
// own copy of project data.
package core

import (
	"encoding/json"
	"time"
)

// Stage is the pipeline position: Prompt (TELL, DISCOVERY) → Blueprint and
// Workflow (DESIGN, with the UI design when the product has screens) →
// Finalized (READY: blueprint and workflow approved, export written).
type Stage string

const (
	StageTell      Stage = "TELL"
	StageDiscovery Stage = "DISCOVERY"
	StageDesign    Stage = "DESIGN"
	StageReady     Stage = "READY"
)

type Project struct {
	Version int    `json:"version"`
	Name    string `json:"name"`
	Dir     string `json:"dir"`
	Stage   Stage  `json:"stage"`
	Story   string `json:"story"`
	// Survey holds the optional answers that bind technical decisions.
	Survey    *Survey    `json:"survey,omitempty"`
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
	Reference *DesignReference `json:"reference,omitempty"`
	// Artifacts are the blueprint documents written for this project
	// besides the built-in ones; ArtifactRequest a change waiting for the AI.
	Artifacts       []Artifact     `json:"artifacts,omitempty"`
	ArtifactRequest *ChangeRequest `json:"artifact_request,omitempty"`
	// Workflow is the command workflow for external coding agents.
	Workflow  *Workflow `json:"workflow,omitempty"`
	Job       *Job      `json:"job,omitempty"`
	Events    []Event   `json:"events"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Survey answers. Empty fields use the defaults.
const (
	SurveyUsersSelf      = "self"
	SurveyUsersTeam      = "team"
	SurveyUsersPublic    = "public"
	SurveyStagePrototype = "prototype"
	SurveyStageMVP       = "mvp"
	SurveyStageProd      = "production"
	SurveyLatency500     = "500"
	SurveyLatency200     = "200"
	SurveyLatency100     = "100"
	SurveyWebhookNone    = "none"
	SurveyWebhookIn      = "in"
	SurveyWebhookOut     = "out"
	SurveyWebhookBoth    = "both"
	SurveyServicesOne    = "one"
	SurveyServicesMany   = "many"
)

// SurveyKind values: what kind of software is built.
const (
	SurveyKindWeb     = "web"
	SurveyKindMobile  = "mobile"
	SurveyKindDesktop = "desktop"
	SurveyKindCLI     = "cli"
	SurveyKindAPI     = "api"
	SurveyKindBot     = "bot"
)

type Survey struct {
	What     string   `json:"what,omitempty"`     // Q1; usually the story
	Kinds    []string `json:"kinds,omitempty"`    // Q2: web, mobile, desktop, cli, api, bot
	Users    string   `json:"users,omitempty"`    // self | team | public
	Stage    string   `json:"stage,omitempty"`    // prototype | mvp | production
	Latency  string   `json:"latency,omitempty"`  // 500 | 200 | 100 (p95 ms)
	Webhook  string   `json:"webhook,omitempty"`  // none | in | out | both
	Services string   `json:"services,omitempty"` // one | many
	Stack    string   `json:"stack,omitempty"`    // empty = Foundry picks
}

func (s *Survey) FillDefaults() {
	if s == nil {
		return
	}
	if s.Users == "" {
		s.Users = SurveyUsersSelf
	}
	if s.Stage == "" {
		s.Stage = SurveyStageMVP
	}
	if s.Latency == "" {
		s.Latency = SurveyLatency500
	}
	if s.Webhook == "" {
		s.Webhook = SurveyWebhookNone
	}
	if s.Services == "" {
		s.Services = SurveyServicesOne
	}
}

func (s *Survey) HasWebhook() bool {
	return s != nil && s.Webhook != "" && s.Webhook != SurveyWebhookNone
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
	PlatformCLI     = "cli"
	// PlatformEmbedded is firmware, IoT and robotics software;
	// PlatformOther any software without its own screens or server
	// (library, SDK, data pipeline, infrastructure, tooling).
	PlatformEmbedded = "embedded"
	PlatformOther    = "other"
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
	Template string   `json:"template,omitempty"`
	Layout   []string `json:"layout,omitempty"`
	Reason   string   `json:"reason"`
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
	Approved bool `json:"approved"`
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

type JobStatus string

const (
	JobRunning JobStatus = "running"
	JobDone    JobStatus = "done"
	JobFailed  JobStatus = "failed"
	JobWaiting JobStatus = "waiting_host" // MCP mode: waiting for the AI host
)

type Job struct {
	Kind    string    `json:"kind"` // discovery | design | design_qa | reference | stack | structure
	Status  JobStatus `json:"status"`
	Message string    `json:"message"`
	Error   string    `json:"error,omitempty"`
	Started time.Time `json:"started"`
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
