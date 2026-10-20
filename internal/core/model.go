// Package core holds the single project context shared by every stage.
// All modules read and write this model through Store; no stage keeps its
// own copy of project data.
package core

import "time"

// Stage is the pipeline position: Prompt (TELL, DISCOVERY) → Blueprint and
// Workflow (DESIGN, with the design rules and layout specification when the
// product has screens) →
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
	Stack *TechStack `json:"stack,omitempty"`
	// UI is the design rules and layout specification of a product with
	// screens; nil until they are written.
	UI *UIDesign `json:"ui,omitempty"`
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
	// Tokens are the design tokens the code declares; TokenSources the
	// files they came from. Nil when the code has too few to be a design
	// system of its own.
	Tokens       *DesignTokens `json:"tokens,omitempty"`
	TokenSources []string      `json:"token_sources,omitempty"`
}

type Component struct {
	Kind     string `json:"kind"` // backend | web | mobile | desktop | shared | database | infra | testing
	Path     string `json:"path"`
	Tech     string `json:"tech"`
	Evidence string `json:"evidence"`
}

type JobStatus string

const (
	JobRunning JobStatus = "running"
	JobDone    JobStatus = "done"
	JobFailed  JobStatus = "failed"
	JobWaiting JobStatus = "waiting_host" // MCP mode: waiting for the AI host
)

type Job struct {
	Kind    string    `json:"kind"` // discovery | design | stack | structure | blueprint | workflow
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

// Busy reports whether a job currently owns the project.
func (p *Project) Busy() bool {
	return p.Job != nil && (p.Job.Status == JobRunning)
}
