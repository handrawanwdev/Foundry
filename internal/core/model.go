// Package core holds the single project context shared by every stage.
// All modules read and write this model through Store; no stage keeps its
// own copy of project data.
package core

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

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
	// Visual is the measured quality of the implemented screens, submitted
	// with browser snapshots and screenshot evidence. Nil until reviewed.
	Visual *VisualAudit `json:"visual,omitempty"`
	// Artifacts are the blueprint documents written for this project
	// besides the built-in ones; ArtifactRequest a change waiting for the AI.
	Artifacts       []Artifact     `json:"artifacts,omitempty"`
	ArtifactRequest *ChangeRequest `json:"artifact_request,omitempty"`
	// Decisions are the assumptions, open questions, conflicts and risks
	// of the blueprint; a critical one without resolution blocks finalize.
	Decisions []Decision `json:"decisions,omitempty"`
	// DecisionSync lists the decisions the user decided, changed or
	// reopened since the written artifacts were last brought in line with
	// them; the next blueprint revision rewrites what contradicts them.
	DecisionSync []string `json:"decision_sync,omitempty"`
	// Skips are the blueprint areas left out on purpose, with the reason.
	Skips []AreaSkip `json:"area_skips,omitempty"`
	// Done is the project's Definition of Done: checkable conditions every
	// finished piece of work meets.
	Done []string `json:"definition_of_done,omitempty"`
	// Workflow is the command workflow for external coding agents.
	Workflow *Workflow `json:"workflow,omitempty"`
	// Progress is how far an external coding agent has taken the workflow.
	// Nil until the first report or reconciliation.
	Progress  *Progress `json:"progress,omitempty"`
	Job       *Job      `json:"job,omitempty"`
	Events    []Event   `json:"events"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Survey answers. An empty answer means the user left it to Foundry: it is
// read from the story or recommended, never invented as a constraint.
const (
	SurveyUsersPublic     = "public"     // public, no account
	SurveyUsersRegistered = "registered" // registered users
	SurveyUsersInternal   = "internal"   // inside one company
	SurveyUsersMixed      = "mixed"      // users and admins
	SurveyUsersSystem     = "system"     // other systems or services

	SurveyStagePrototype = "prototype"
	SurveyStageMVP       = "mvp"
	SurveyStageProd      = "production"

	SurveyArchModular  = "modular"  // one modular application
	SurveyArchServices = "services" // several separate services
	SurveyArchExisting = "existing" // follows the existing project

	SurveyPrioritySecurity    = "security"
	SurveyPriorityPerformance = "performance"
	SurveyPriorityReliability = "reliability"
	SurveyPriorityScalability = "scalability"
	SurveyPriorityTesting     = "testing"
	SurveyPriorityOperations  = "operations"

	// Answers of the first survey, still read from older project files.
	SurveyLatency200  = "200"
	SurveyLatency100  = "100"
	SurveyWebhookNone = "none"
)

// SurveyKind values: what kind of software is built.
const (
	SurveyKindWeb      = "web"
	SurveyKindMobile   = "mobile"
	SurveyKindDesktop  = "desktop"
	SurveyKindCLI      = "cli"
	SurveyKindAPI      = "api"
	SurveyKindEmbedded = "embedded"
	SurveyKindOther    = "other"
	SurveyKindBot      = "bot" // first survey; read as api
)

// Survey choices in display order, with the words the blueprint uses.
var (
	SurveyKinds = []Choice{{SurveyKindWeb, "Web"}, {SurveyKindMobile, "Mobile"}, {SurveyKindDesktop, "Desktop"},
		{SurveyKindCLI, "CLI"}, {SurveyKindAPI, "API"}, {SurveyKindEmbedded, "Embedded"}, {SurveyKindOther, "Lainnya"}}
	SurveyUsers = []Choice{{SurveyUsersPublic, "Publik, tanpa akun"}, {SurveyUsersRegistered, "Pengguna terdaftar"},
		{SurveyUsersInternal, "Internal perusahaan"}, {SurveyUsersMixed, "Campuran pengguna dan admin"},
		{SurveyUsersSystem, "Sistem atau layanan lain"}}
	SurveyStages = []Choice{{SurveyStagePrototype, "Prototype"}, {SurveyStageMVP, "MVP"}, {SurveyStageProd, "Production-ready"}}
	SurveyArchs  = []Choice{{SurveyArchModular, "Satu aplikasi modular"}, {SurveyArchServices, "Beberapa service terpisah"},
		{SurveyArchExisting, "Mengikuti arsitektur project yang sudah ada"}}
	SurveyPriorities = []Choice{{SurveyPrioritySecurity, "Keamanan dan kontrol akses"},
		{SurveyPriorityPerformance, "Performa dan latency"}, {SurveyPriorityReliability, "Reliabilitas dan konsistensi data"},
		{SurveyPriorityScalability, "Skalabilitas"}, {SurveyPriorityTesting, "Testing dan verifikasi"},
		{SurveyPriorityOperations, "Deployment dan operasional"}}
)

// DeliveryCriteria is what each delivery target counts as done.
var DeliveryCriteria = map[string]string{
	SurveyStagePrototype: "Alur utama bisa dicoba dari awal sampai akhir untuk memvalidasi ide; data contoh boleh; tanpa hardening produksi.",
	SurveyStageMVP:       "Fitur inti siap dipakai pengguna nyata: validasi input, akses sesuai peran, dan test untuk alur utama.",
	SurveyStageProd:      "Siap produksi: keamanan, testing otomatis, deployment, observability (log, metrik, health check), dan operasional (konfigurasi, backup).",
}

// Choice is one answer of a survey question.
type Choice struct{ ID, Label string }

// ChoiceLabel is the label of id among cs, "" when unknown.
func ChoiceLabel(cs []Choice, id string) string {
	for _, c := range cs {
		if c.ID == id {
			return c.Label
		}
	}
	return ""
}

type Survey struct {
	What  string   `json:"what,omitempty"`  // usually the story
	Kinds []string `json:"kinds,omitempty"` // SurveyKinds; empty = from the story
	Users string   `json:"users,omitempty"` // SurveyUsers; empty = from the story
	Stage string   `json:"stage,omitempty"` // SurveyStages; empty = mvp
	// Architecture is SurveyArchs; empty = Foundry picks from the needs.
	Architecture string `json:"architecture,omitempty"`
	// Priorities are SurveyPriorities; the basic qualities hold anyway.
	Priorities []string `json:"priorities,omitempty"`
	// Constraints are the user's mandatory constraints in their words:
	// required stack, integrations, compatibility, limits, prohibitions.
	Constraints string `json:"constraints,omitempty"`

	// First survey: Latency and Webhook are still honored, Services and
	// Stack move to Architecture and Constraints when the project loads.
	Latency  string `json:"latency,omitempty"`
	Webhook  string `json:"webhook,omitempty"`
	Services string `json:"services,omitempty"`
	Stack    string `json:"stack,omitempty"`
}

// FillDefaults normalizes the answers and moves answers of the first
// survey. Unanswered questions stay empty: Foundry decides them.
func (s *Survey) FillDefaults() {
	if s == nil {
		return
	}
	trim := func(v string) string { return strings.ToLower(strings.TrimSpace(v)) }
	s.Users, s.Stage, s.Architecture = trim(s.Users), trim(s.Stage), trim(s.Architecture)
	s.Constraints = strings.TrimSpace(s.Constraints)
	switch s.Users { // first survey
	case "team":
		s.Users = SurveyUsersInternal
	case "self":
		s.Users = ""
	}
	if s.Services == "many" && s.Architecture == "" {
		s.Architecture = SurveyArchServices
	}
	if st := strings.TrimSpace(s.Stack); st != "" && !strings.Contains(s.Constraints, st) {
		s.Constraints = strings.TrimSpace(st + "\n" + s.Constraints)
	}
	s.Services, s.Stack = "", ""
	if s.Latency == "500" {
		s.Latency = "" // the old default, not an answer
	}
	if s.Webhook == SurveyWebhookNone {
		s.Webhook = ""
	}
	s.Kinds = normChoices(s.Kinds, func(k string) string {
		if k == SurveyKindBot {
			return SurveyKindAPI
		}
		return k
	})
	s.Priorities = normChoices(s.Priorities, nil)
}

// Delivery is the delivery target; MVP when the user gave none.
func (s *Survey) Delivery() string {
	if s == nil || s.Stage == "" {
		return SurveyStageMVP
	}
	return s.Stage
}

func normChoices(in []string, mapID func(string) string) []string {
	var out []string
	for _, v := range in {
		v = strings.ToLower(strings.TrimSpace(v))
		if mapID != nil {
			v = mapID(v)
		}
		if v != "" && !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	return out
}

// Check refuses answers that are not survey choices.
func (s *Survey) Check() error {
	if s == nil {
		return nil
	}
	bad := func(q string, v string) error { return fmt.Errorf("jawaban survey %s tidak dikenal: %q", q, v) }
	for _, k := range s.Kinds {
		if ChoiceLabel(SurveyKinds, k) == "" {
			return bad("platform", k)
		}
	}
	for _, x := range s.Priorities {
		if ChoiceLabel(SurveyPriorities, x) == "" {
			return bad("prioritas", x)
		}
	}
	for _, q := range []struct {
		name, v string
		cs      []Choice
	}{{"pengguna", s.Users, SurveyUsers}, {"target delivery", s.Stage, SurveyStages}, {"arsitektur", s.Architecture, SurveyArchs}} {
		if q.v != "" && ChoiceLabel(q.cs, q.v) == "" {
			return bad(q.name, q.v)
		}
	}
	if len(s.Constraints) > 4000 {
		return errors.New("batasan wajib terlalu panjang (maks 4000 karakter)")
	}
	return nil
}

func (s *Survey) HasPriority(id string) bool { return s != nil && slices.Contains(s.Priorities, id) }

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
	// Acceptance are the conditions that show the requirement is met;
	// Untestable is why a requirement has none.
	Acceptance []Criterion `json:"acceptance,omitempty"`
	Untestable string      `json:"untestable,omitempty"`
}

// Criterion is one acceptance criterion. Evidence is the proof the coding
// agent is expected to hand back (a test, a command and its output, a
// screenshot); it is a requirement, never a claim that it was verified.
type Criterion struct {
	ID       string `json:"id"` // REQ-001-AC1
	Kind     string `json:"kind"`
	Text     string `json:"text"`
	Evidence string `json:"evidence,omitempty"`
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

// Flow is a use case: an actor reaching a goal end to end. Role is the
// actor and Steps the main flow; its requirements come from its feature.
type Flow struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	FeatureID string   `json:"feature_id"`
	Role      string   `json:"role"`
	Steps     []string `json:"steps"`
	PageIDs   []string `json:"page_ids"`
	Goal      string   `json:"goal,omitempty"`
	Pre       []string `json:"preconditions,omitempty"`
	// Post are the conditions that hold when the use case succeeds.
	Post         []string `json:"postconditions,omitempty"`
	Alternatives []string `json:"alternatives,omitempty"`
	Failures     []string `json:"failures,omitempty"`
	Rules        []string `json:"business_rules,omitempty"`
	Permissions  []string `json:"permissions,omitempty"`
	// Data are the effects on data ("Appointment: dibuat, status pending");
	// Integrations the external systems the use case calls or notifies.
	Data         []string `json:"data_effects,omitempty"`
	Integrations []string `json:"integrations,omitempty"`
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

// PlatformTarget is one platform the product runs on. Who uses it is not
// part of the target: users, roles and their platform are in
// Discovery.Roles (requirements and security).
type PlatformTarget struct {
	Kind      string `json:"kind"`
	Rationale string `json:"rationale"`
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
	// backend | web | mobile | desktop | cli | shared | database | infra | testing, or a technology area
	// (ui, cache, messaging, deployment, observability, testing, quality, repository) for what the scan
	// read from dependencies and config files.
	Kind     string `json:"kind"`
	Path     string `json:"path"`
	Tech     string `json:"tech"`
	Language string `json:"language,omitempty"`
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
