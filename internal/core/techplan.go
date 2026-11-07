package core

import (
	"encoding/json"
	"slices"
	"strings"
	"time"
)

// Technology plan: the AI analyzes what the system needs and designs the
// technology for it, area by area, with the reasoning behind every choice,
// the alternatives it weighed, how the choices fit together and an honest
// evaluation. Foundry stores, validates and exports the plan; it never
// builds the product.
//
// The plan is free-form. Foundry's catalog and profiles are reference, not a
// menu: the AI may choose any technology, combine technologies, propose
// something new or decide that an area needs none. The plan's decisions on
// the areas Foundry knows (TechAreas) become the TechStack decisions every
// later stage reads; the other areas (authentication, ORM, CI/CD, ...) stay
// in the plan and reach the later stages through the stack brief.

// Source of a decision the AI planned.
const SourceAI = "ai"

// Groups of the technology plan, in the order they are shown. They extend
// TechGroups with the application infrastructure the plan also decides.
const (
	TechPlanStack        = "stack"
	TechPlanData         = "data"
	TechPlanAppInfra     = "app_infra"
	TechPlanArchitecture = "architecture"
	TechPlanOperations   = "operations"
	TechPlanStandards    = "standards"
)

var TechPlanGroups = []string{TechPlanStack, TechPlanData, TechPlanAppInfra, TechPlanArchitecture, TechPlanOperations, TechPlanStandards}

// extraGroup is the group of the well-known areas outside TechAreas. Any
// other area names its group in the plan.
var extraGroup = map[string]string{
	"state_management": TechPlanStack, "runtime": TechPlanStack, "language": TechPlanStack, "api_gateway": TechPlanStack, "realtime": TechPlanStack,
	"orm": TechPlanData, "data_access": TechPlanData, "query_layer": TechPlanData, "migration": TechPlanData, "data_migration": TechPlanData,
	"object_storage": TechPlanData, "data_warehouse": TechPlanData,
	"authentication": TechPlanAppInfra, "auth": TechPlanAppInfra, "authorization": TechPlanAppInfra, "session": TechPlanAppInfra,
	"token": TechPlanAppInfra, "background_jobs": TechPlanAppInfra, "job_queue": TechPlanAppInfra, "message_broker": TechPlanAppInfra,
	"scheduler": TechPlanAppInfra, "integrations": TechPlanAppInfra, "external_integrations": TechPlanAppInfra, "email": TechPlanAppInfra,
	"containerization": TechPlanOperations, "hosting": TechPlanOperations, "networking": TechPlanOperations, "ci_cd": TechPlanOperations,
	"logging": TechPlanOperations, "metrics": TechPlanOperations, "tracing": TechPlanOperations, "monitoring": TechPlanOperations,
	"secrets": TechPlanOperations, "backup": TechPlanOperations,
	"build_tool": TechPlanStandards, "package_manager": TechPlanStandards, "static_analysis": TechPlanStandards, "formatting": TechPlanStandards,
	"linting": TechPlanStandards, "code_quality": TechPlanStandards,
}

// TechAreaGroup is the plan group of area, "" when the area is neither a
// known core area nor a well-known extra one.
func TechAreaGroup(area string) string {
	switch GroupOf(area) {
	case "stack":
		return TechPlanStack
	case "data":
		return TechPlanData
	case "architecture":
		return TechPlanArchitecture
	case "operations":
		return TechPlanOperations
	case "standards":
		return TechPlanStandards
	}
	return extraGroup[area]
}

// Why a decision was made. The explicit requirements of the project outrank
// everything else; a profile only informs.
const (
	DriverRequirement   = "requirement"
	DriverArchitecture  = "architecture"
	DriverCompatibility = "compatibility"
	DriverEngineering   = "engineering"
	DriverOperational   = "operational"
	DriverProfile       = "profile"
)

var TechDrivers = []string{DriverRequirement, DriverArchitecture, DriverCompatibility, DriverEngineering, DriverOperational, DriverProfile}

// Confidence in a decision.
var TechConfidence = []string{"high", "medium", "low"}

// TechFact is one thing the AI understood about the system before choosing
// technology. Category is functional, non_functional, data, transaction,
// integration, security, performance, deployment, operational,
// architecture, folder_structure or modules; Source is the project area it
// came from, or "assumed".
type TechFact struct {
	Category  string `json:"category"`
	Statement string `json:"statement"`
	Source    string `json:"source,omitempty"`
}

var TechFactCategories = []string{
	"functional", "non_functional", "data", "transaction", "integration", "security", "performance",
	"deployment", "operational", "architecture", "folder_structure", "modules",
}

// TechContext is the technical understanding the choices rest on.
type TechContext struct {
	Summary string     `json:"summary,omitempty"`
	Facts   []TechFact `json:"facts,omitempty"`
	// Unknowns are what the requirements do not say yet. The AI lists them
	// instead of inventing an answer.
	Unknowns    []string `json:"unknowns,omitempty"`
	Assumptions []string `json:"assumptions,omitempty"`
}

// TechAlternative is a technology that would also work, compared with the
// choice.
type TechAlternative struct {
	Technology         string   `json:"technology"`
	Why                string   `json:"why"`        // why it is viable here
	Difference         string   `json:"difference"` // how it differs from the choice
	Pros               []string `json:"pros,omitempty"`
	Cons               []string `json:"cons,omitempty"`
	ArchitectureImpact string   `json:"architecture_impact,omitempty"`
	MaintenanceImpact  string   `json:"maintenance_impact,omitempty"`
	CostImpact         string   `json:"cost_impact,omitempty"`
}

// TechItem is one technology decision of the plan. Area is a core area
// (TechAreas) or any other area the system needs (authentication, orm,
// ci_cd, ...). A decision that an area needs no technology has Technology
// TechNone and the reason in Reasoning.
type TechItem struct {
	Area       string `json:"area"`
	Group      string `json:"group,omitempty"`
	Technology string `json:"technology"`
	// Version is the version or compatibility range, when it can be
	// determined; VersionVerified says whether it was checked against a
	// source. An unverified version is shown as such.
	Version         string `json:"version,omitempty"`
	VersionVerified bool   `json:"version_verified,omitempty"`
	Language        string `json:"language,omitempty"`
	Runtime         string `json:"runtime,omitempty"`
	Purpose         string `json:"purpose"`
	// Fulfills are the requirement ids, feature ids, module ids or technical
	// needs the technology exists for.
	Fulfills  []string `json:"fulfills,omitempty"`
	Driver    string   `json:"driver"`
	Reasoning string   `json:"reasoning"`
	Pros      []string `json:"pros,omitempty"`
	Cons      []string `json:"cons,omitempty"`
	Tradeoffs []string `json:"tradeoffs,omitempty"`
	// DependsOn are the areas this technology integrates with or needs;
	// Integration says how.
	DependsOn    []string          `json:"depends_on,omitempty"`
	Integration  string            `json:"integration,omitempty"`
	Operational  string            `json:"operational,omitempty"`
	Alternatives []TechAlternative `json:"alternatives,omitempty"`
	Confidence   string            `json:"confidence"`
	Assumptions  []string          `json:"assumptions,omitempty"`
	// Action classifies the decision against an existing codebase:
	// reuse | add | modify | refactor | remove. Empty on greenfield.
	Action string `json:"action,omitempty"`

	// Computed by Foundry, never trusted from the AI.
	InCatalog bool `json:"in_catalog,omitempty"`
	// UserChosen is set when the user's own choice replaced this one.
	UserChosen string `json:"user_chosen,omitempty"`
}

// None reports whether the decision is that the area needs no technology.
func (i TechItem) None() bool { return i.Technology == "" || i.Technology == TechNone }

// TechConstraint is a compatibility rule between decisions, for example
// "Prisma 6 needs Node 18.18+". Areas are the areas it binds.
type TechConstraint struct {
	Areas  []string `json:"areas"`
	Rule   string   `json:"rule"`
	Reason string   `json:"reason,omitempty"`
}

// Compatibility areas the AI must verify.
var TechCompatAreas = []string{
	"language", "runtime", "framework", "package", "database", "api", "authentication", "storage", "build", "deployment", "testing", "operational",
}

// Compatibility statuses.
const (
	CompatCompatible    = "compatible" // verified; Basis says on what
	CompatUnverified    = "unverified" // plausible but not verified
	CompatConflict      = "conflict"   // an unresolved incompatibility; blocks approval
	CompatResolved      = "resolved"   // a conflict found and fixed in this plan
	CompatNotApplicable = "not_applicable"
)

// TechCompat is one compatibility check across the stack. Between are the
// decisions it concerns. A claim of compatibility needs a Basis; without one
// Foundry records it as unverified.
type TechCompat struct {
	Area    string   `json:"area"`
	Status  string   `json:"status"`
	Between []string `json:"between,omitempty"`
	Note    string   `json:"note,omitempty"`
	Basis   string   `json:"basis,omitempty"`
	Source  string   `json:"source,omitempty"` // ai | computed
}

// Evaluation criteria of the whole stack. Operational complexity and total
// cost are scored 1 (low) to 5 (high), the others 1 (poor) to 5 (excellent).
var TechCriteria = []string{
	"requirement_coverage", "compatibility", "maintainability", "performance", "scalability", "security",
	"reliability", "developer_experience", "operational_complexity", "total_cost", "sustainability",
}

// LowerIsBetter are the criteria where a high score is bad.
var TechLowerIsBetter = []string{"operational_complexity", "total_cost"}

// TechCriterion is one evaluated quality. Evidence refers to the decisions
// (areas), requirements or modules the score rests on.
type TechCriterion struct {
	Name     string   `json:"name"`
	Score    int      `json:"score"`
	Reason   string   `json:"reason"`
	Evidence []string `json:"evidence,omitempty"`
}

// Technology finding categories.
var TechFindingCategories = []string{
	"redundant-dependency", "unnecessary-technology", "incompatibility", "over-engineering", "operational-risk",
	"single-point-of-failure", "vendor-lock-in", "unverified", "missing-decision", "other",
}

// TechFinding is a weakness of the plan. Source is "ai" for the architect's
// review and "computed" for what Foundry measured.
type TechFinding struct {
	Category   string `json:"category"`
	Severity   string `json:"severity"` // info | warning | error
	Area       string `json:"area,omitempty"`
	Message    string `json:"message"`
	Resolution string `json:"resolution,omitempty"`
	Source     string `json:"source,omitempty"`
}

// TechEvaluation is the quality review of the stack.
type TechEvaluation struct {
	Summary  string          `json:"summary,omitempty"`
	Criteria []TechCriterion `json:"criteria"`
	Findings []TechFinding   `json:"findings"`
	// Score is computed from the criteria (0-100); the AI does not set it.
	Score int `json:"score"`
	// Outdated is true after a decision changed outside the AI's review: the
	// scores no longer describe the stack until the AI evaluates it again.
	Outdated bool `json:"outdated,omitempty"`
}

// Consistency areas the architect must verify against the other artifacts.
var TechConsistencyAreas = []string{
	"requirements", "project_modules", "architecture", "folder_structure", "data_architecture", "security", "deployment",
}

// TechCheck is one consistency check between the technology and another
// artifact. Status is consistent, resolved or conflict (CheckConsistent,
// CheckResolved, CheckConflict); Source is ai or computed.
type TechCheck struct {
	Area   string `json:"area"`
	Status string `json:"status"`
	Note   string `json:"note,omitempty"`
	Source string `json:"source,omitempty"`
}

// TechCoverage says which decisions serve one need.
type TechCoverage struct {
	Ref   string   `json:"ref"`
	Areas []string `json:"areas"`
}

// TechAnalysis is everything Foundry computes from the plan and the
// project: recomputed on every read, never trusted from the AI.
type TechAnalysis struct {
	Issues   []TechIssue   `json:"issues,omitempty"`
	Findings []TechFinding `json:"findings,omitempty"`
	Checks   []TechCheck   `json:"checks,omitempty"`
	Compat   []TechCompat  `json:"compat,omitempty"`
	Items    int           `json:"items"`
	// Outside are the decisions whose technology Foundry's catalog does not
	// list; Unverified the ones whose version was not verified.
	Outside    int            `json:"outside_catalog"`
	Unverified int            `json:"unverified"`
	Coverage   []TechCoverage `json:"coverage,omitempty"`
	// Diverged are the areas where the stack's current choice (the user's
	// own) differs from what the plan reasoned about.
	Diverged []string `json:"diverged,omitempty"`
	// StaleAreas are the source areas that changed since the plan was
	// written; Affected are the plan sections to check again.
	StaleAreas []string `json:"stale_areas,omitempty"`
	Affected   []string `json:"affected,omitempty"`
}

// TechPlanVersion is one entry of the plan's revision history.
type TechPlanVersion struct {
	Version int        `json:"version"`
	At      time.Time  `json:"at"`
	Source  string     `json:"source"` // generate | ai | user
	Request string     `json:"request,omitempty"`
	Reason  string     `json:"reason,omitempty"`
	Changes []string   `json:"changes,omitempty"`
	Status  TechStatus `json:"status"`
	Items   int        `json:"items"`
	Score   int        `json:"score"`
}

// TechPlanRequest is an AI job waiting for an answer: a fresh design
// ("generate") or a revision of the current plan ("revise").
type TechPlanRequest struct {
	Action     string   `json:"action"`
	Text       string   `json:"text,omitempty"`
	Reason     string   `json:"reason,omitempty"`
	Areas      []string `json:"areas,omitempty"`
	SourceHash string   `json:"source_hash,omitempty"`
	// Rejected is why the AI's last answer was refused; the retry prompt
	// shows it.
	Rejected string `json:"rejected,omitempty"`
}

const (
	TechPlanGenerate = "generate"
	TechPlanRevise   = "revise"
)

// TechPlan is the technology plan of a product.
type TechPlan struct {
	// Approach names the AI's overall technology strategy (free text, never
	// picked from a list).
	Approach  string `json:"approach"`
	Summary   string `json:"summary"`
	Rationale string `json:"rationale,omitempty"`

	Context       TechContext      `json:"context"`
	Items         []TechItem       `json:"items"`
	Constraints   []TechConstraint `json:"constraints,omitempty"`
	Compatibility []TechCompat     `json:"compatibility,omitempty"`
	Evaluation    TechEvaluation   `json:"evaluation"`
	Consistency   []TechCheck      `json:"consistency,omitempty"`

	History []TechPlanVersion `json:"history,omitempty"`
	// Sources are hashes of the project areas the plan was written from;
	// GeneratedAt is when the AI last wrote or reviewed the whole plan.
	Sources     map[string]string `json:"sources,omitempty"`
	GeneratedAt time.Time         `json:"generated_at,omitempty"`
	// Stale is true when a source area changed after the plan was written.
	Stale bool `json:"stale,omitempty"`

	Analysis TechAnalysis `json:"analysis"`
}

// Version is the number of the last revision.
func (t *TechPlan) Version() int {
	if t == nil || len(t.History) == 0 {
		return 0
	}
	return t.History[len(t.History)-1].Version
}

// Item returns the decision for area, nil when absent.
func (t *TechPlan) Item(area string) *TechItem {
	if t == nil {
		return nil
	}
	for i := range t.Items {
		if t.Items[i].Area == area {
			return &t.Items[i]
		}
	}
	return nil
}

// Tech is the technology of area, "" when the plan has no decision or none
// is needed.
func (t *TechPlan) Tech(area string) string {
	if i := t.Item(area); i != nil && !i.None() {
		return i.Technology
	}
	return ""
}

// Errors are the issues that block approval.
func (t *TechPlan) Errors() []TechIssue {
	var out []TechIssue
	if t != nil {
		for _, i := range t.Analysis.Issues {
			if i.Level == "error" {
				out = append(out, i)
			}
		}
	}
	return out
}

// Extras are the decisions outside TechAreas, which have no TechDecision.
func (t *TechPlan) Extras() []TechItem {
	var out []TechItem
	if t != nil {
		for _, i := range t.Items {
			if !slices.Contains(TechAreas, i.Area) {
				out = append(out, i)
			}
		}
	}
	return out
}

// Clone is a deep copy of the plan.
func (t *TechPlan) Clone() *TechPlan {
	if t == nil {
		return nil
	}
	b, err := json.Marshal(t)
	if err != nil {
		c := *t
		return &c
	}
	var c TechPlan
	if err := json.Unmarshal(b, &c); err != nil {
		c = *t
	}
	return &c
}

// Brief is the plan's decisions outside TechAreas as prompt lines, for the
// stages that build on the technology.
func (t *TechPlan) Brief() string {
	var b strings.Builder
	for _, i := range t.Extras() {
		b.WriteString("- " + i.Area + ": " + i.Technology)
		if i.Version != "" {
			b.WriteString(" " + i.Version)
		}
		if i.Language != "" {
			b.WriteString(" (language " + i.Language + ")")
		}
		if i.Purpose != "" {
			b.WriteString(" — " + i.Purpose)
		}
		b.WriteString("\n")
	}
	return b.String()
}
