package core

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

// Technology decision areas, in the order they are decided and shown.
// Each area is one domain: a framework (with its language and runtime as
// attributes), the API protocol, the UI library, a data or communication
// component, the application architecture, the repository layout, the
// deployment strategy, observability and the engineering tools.
const (
	TechFrontend      = "frontend"
	TechUI            = "ui" // UI library of the web/desktop frontend
	TechMobile        = "mobile"
	TechDesktop       = "desktop"
	TechCLI           = "cli"
	TechBackend       = "backend"
	TechAPI           = "api" // API protocol between clients and backend
	TechDatabase      = "database"
	TechCache         = "cache"
	TechMessaging     = "messaging"
	TechWorkers       = "workers"
	TechStorage       = "storage"
	TechSearch        = "search"
	TechArchitecture  = "architecture"
	TechRepository    = "repository"
	TechDeployment    = "deployment"
	TechObservability = "observability"
	TechTesting       = "testing"
	TechQuality       = "quality" // lint and format
)

var TechAreas = []string{TechFrontend, TechUI, TechMobile, TechDesktop, TechCLI, TechBackend, TechAPI, TechDatabase, TechCache,
	TechMessaging, TechWorkers, TechStorage, TechSearch, TechArchitecture, TechRepository, TechDeployment, TechObservability,
	TechTesting, TechQuality}

// TechGroup is one section of the technology decision. The target
// platform (Project.Platform) comes before them; user roles and
// permissions belong to the requirements and security, not here.
type TechGroup struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Areas       []string `json:"areas"`
}

var TechGroups = []TechGroup{
	{ID: "stack", Title: "Application Stack", Description: "Framework per bagian aplikasi beserta bahasa dan runtime-nya, UI library, dan protokol API.",
		Areas: []string{TechFrontend, TechUI, TechMobile, TechDesktop, TechCLI, TechBackend, TechAPI}},
	{ID: "data", Title: "Data & Communication", Description: "Database, cache, messaging, worker, file storage, dan search; komponen tambahan hanya bila ada kebutuhannya.",
		Areas: []string{TechDatabase, TechCache, TechMessaging, TechWorkers, TechStorage, TechSearch}},
	{ID: "architecture", Title: "Architecture & Repository", Description: "Arsitektur aplikasi dan struktur repository adalah dua keputusan terpisah; struktur folder diturunkan dari keduanya.",
		Areas: []string{TechArchitecture, TechRepository}},
	{ID: "operations", Title: "Infrastructure & Operations", Description: "Strategi deployment dan observability.",
		Areas: []string{TechDeployment, TechObservability}},
	{ID: "standards", Title: "Engineering Standards", Description: "Tool testing dan kualitas kode per bahasa. Aturan kode ada di artefak Arsitektur, tidak diulang di sini.",
		Areas: []string{TechTesting, TechQuality}},
}

// GroupOf is the group id of area, "" when unknown.
func GroupOf(area string) string {
	for _, g := range TechGroups {
		if slices.Contains(g.Areas, area) {
			return g.ID
		}
	}
	return ""
}

// Architecture and repository values; the project structure is derived
// from them.
const (
	ArchSimple        = "Simple"
	ArchModular       = "Modular Monolith"
	ArchMicroservices = "Microservices"
	RepoSimple        = "Simple"
	RepoMonorepo      = "Monorepo"
	TechNone          = "Tidak perlu"
	CacheNone         = TechNone
)

// TechStatus is where the technology decision is in its review. A change
// after approval makes it changed: it must be approved again before the
// design is approved.
type TechStatus string

const (
	TechRecommended   TechStatus = "recommended"    // the recommendation, untouched
	TechReview        TechStatus = "review"         // a proposal waits for the user
	TechImpactChecked TechStatus = "impact_checked" // changes applied after the impact was shown
	TechApproved      TechStatus = "approved"
	TechChanged       TechStatus = "changed" // changed after approval; impact check and approval required
)

// TechStack is the technology decision for the product: one choice per
// area with the reason, plus the user's overrides. Design, plan, tasks and
// docs all read it, so every stage builds with the same technology.
type TechStack struct {
	// Mode is existing when the stack was read from the project's code;
	// such a stack is never replaced by a recommendation.
	Mode StructureMode `json:"mode"`
	// Profile is the technology profile the recommendation started from.
	Profile string `json:"profile,omitempty"`
	// Needs are the technical signals found in the requirement
	// (realtime, offline, seo, ai, ...).
	Needs     []string       `json:"needs,omitempty"`
	Platform  TechPlatform   `json:"platform"`
	Decisions []TechDecision `json:"decisions"`
	// Approved mirrors Status == approved for older readers.
	Approved bool       `json:"approved"`
	Status   TechStatus `json:"status,omitempty"`
	// Issues are the validation findings of the current decisions.
	Issues []TechIssue `json:"issues,omitempty"`
	// Proposal is a change the user (or the AI on the user's request) has
	// asked for; it is applied only when the user confirms it.
	Proposal *TechProposal `json:"proposal,omitempty"`
	// Request is a natural-language change waiting for the AI.
	Request string        `json:"request,omitempty"`
	History []TechVersion `json:"history,omitempty"`
}

// TechChange is one area moving from one technology to another.
type TechChange struct {
	Area     string `json:"area"`
	From     string `json:"from"`
	To       string `json:"to"`
	Language string `json:"language,omitempty"`
	Reason   string `json:"reason,omitempty"`
	// Follow marks a recommendation that changed because a decision it
	// depends on changed; it is not the user's choice.
	Follow bool `json:"follow,omitempty"`
}

// TechIssue is a concrete validation finding; an error blocks approval.
type TechIssue struct {
	Level   string `json:"level"` // error | warning
	Area    string `json:"area,omitempty"`
	Message string `json:"message"`
}

// TechImpact is what a change of one area affects downstream.
type TechImpact struct {
	Area    string   `json:"area"`
	Affects []string `json:"affects"`
	// Design is true when the design should be reviewed again.
	Design bool `json:"design,omitempty"`
}

type TechProposal struct {
	Source  string       `json:"source"` // user | ai
	Request string       `json:"request,omitempty"`
	Note    string       `json:"note,omitempty"`
	Changes []TechChange `json:"changes"`
	Issues  []TechIssue  `json:"issues,omitempty"`
	Impact  []TechImpact `json:"impact,omitempty"`
	// Review are the user's (or the code's) choices that depend on a
	// changed area; they are marked for review when the proposal applies.
	Review []TechIssue `json:"review,omitempty"`
	// Affected are the blueprint artifacts whose content changes and the
	// commands that refer to them; the commands become needs_review.
	Affected *TechAffected `json:"affected,omitempty"`
	// Decisions is the decision as it will be after the proposal; nil in
	// proposals saved before it existed.
	Decisions []TechDecision `json:"decisions,omitempty"`
	// Previous is the status to return to when the proposal is cancelled.
	Previous TechStatus `json:"previous"`
	At       time.Time  `json:"at"`
}

// TechAffected is what a technology change reaches in the blueprint and
// the workflow, computed from the artifacts' content, not guessed.
type TechAffected struct {
	Artifacts []string `json:"artifacts,omitempty"`
	Commands  []string `json:"commands,omitempty"`
}

// TechVersion is one saved state of the decision.
type TechVersion struct {
	Version   int            `json:"version"`
	At        time.Time      `json:"at"`
	Source    string         `json:"source"` // recommend | user | ai | structure
	Request   string         `json:"request,omitempty"`
	Changes   []TechChange   `json:"changes,omitempty"`
	Impact    []TechImpact   `json:"impact,omitempty"`
	Design    bool           `json:"design,omitempty"`
	Status    TechStatus     `json:"status"`
	Decisions []TechDecision `json:"decisions"`
}

// State is the status, derived for stacks saved before it existed.
func (s *TechStack) State() TechStatus {
	switch {
	case s == nil:
		return ""
	case s.Status != "":
		return s.Status
	case s.Approved:
		return TechApproved
	}
	for _, d := range s.Decisions {
		if d.Override != nil {
			return TechImpactChecked
		}
	}
	return TechRecommended
}

func (s *TechStack) SetStatus(st TechStatus) {
	s.Status, s.Approved = st, st == TechApproved
	if st == TechApproved && len(s.History) > 0 {
		s.History[len(s.History)-1].Status = st
	}
}

// Version is the current version number, 0 before the first is saved.
func (s *TechStack) Version() int {
	if s == nil || len(s.History) == 0 {
		return 0
	}
	return s.History[len(s.History)-1].Version
}

// Errors are the issues that block approval.
func (s *TechStack) Errors() []TechIssue {
	var out []TechIssue
	if s != nil {
		for _, i := range s.Issues {
			if i.Level == "error" {
				out = append(out, i)
			}
		}
	}
	return out
}

// CloneDecisions copies the decisions, overrides included.
func CloneDecisions(ds []TechDecision) []TechDecision {
	out := make([]TechDecision, len(ds))
	for i, d := range ds {
		if d.Override != nil {
			o := *d.Override
			d.Override = &o
		}
		d.Alternatives = append([]string(nil), d.Alternatives...)
		d.Evidence = append([]string(nil), d.Evidence...)
		out[i] = d
	}
	return out
}

// TechPlatform is which parts the product has, from the target platform.
// Android and IOS are false in stacks saved before they existed (Mobile
// alone then).
type TechPlatform struct {
	Web     bool `json:"web"`
	Mobile  bool `json:"mobile"`
	Android bool `json:"android,omitempty"`
	IOS     bool `json:"ios,omitempty"`
	Desktop bool `json:"desktop"`
	Backend bool `json:"backend"`
	CLI     bool `json:"cli,omitempty"`
}

// Sources of a recommendation, from the strongest.
const (
	SourceExisting   = "existing"   // read from the project's code
	SourcePreference = "preference" // the stack the user stated in the survey
	SourceNeed       = "need"       // a technical need of the project
	SourceProfile    = "profile"    // the profile's policy for this kind of product
	SourceDefault    = "default"
	SourceDependency = "dependency" // follows another decision (UI library of the frontend, ...)
)

type TechDecision struct {
	Area        string `json:"area"`
	Recommended string `json:"recommended"`
	Language    string `json:"language,omitempty"`
	Runtime     string `json:"runtime,omitempty"`
	Reason      string `json:"reason"`
	// Source is where the recommendation came from; "" in stacks saved
	// before it existed.
	Source string `json:"source,omitempty"`
	// Evidence are the files an existing technology was read from.
	Evidence []string `json:"evidence,omitempty"`
	// Alternatives are other known options that fit the other choices.
	Alternatives []string `json:"alternatives,omitempty"`
	// Override is the user's choice; it wins over the recommendation.
	Override *TechOverride `json:"override,omitempty"`
	// Review is why the choice must be checked again: a decision it
	// depends on changed after it was made. The user keeps or changes it.
	Review string `json:"review,omitempty"`
}

type TechOverride struct {
	Value    string `json:"value"`
	Language string `json:"language,omitempty"`
	Runtime  string `json:"runtime,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// Decision states: what a decision is, as the user reads it. A
// recommendation is never an approval.
const (
	StateRecommended = "recommended" // Foundry's recommendation, not chosen or approved yet
	StateExisting    = "existing"    // already used in the code
	StateSelected    = "selected"    // chosen by the user, not approved yet
	StateApproved    = "approved"    // part of the last approved version
	StateConflict    = "conflict"    // an unresolved incompatibility (error issue)
)

// DecisionState is the state of d in s.
func (s *TechStack) DecisionState(d TechDecision) string {
	for _, i := range s.Issues {
		if i.Level == "error" && i.Area == d.Area {
			return StateConflict
		}
	}
	if d.Review != "" {
		return StateNeedsReview
	}
	if v, ok := s.approvedChoices()[d.Area]; ok && v == d.Choice() {
		return StateApproved
	}
	if d.Override != nil {
		return StateSelected
	}
	if d.Source == SourceExisting || d.Source == "" && s.Mode == StructureExisting {
		return StateExisting
	}
	return StateRecommended
}

// approvedChoices are the choices of the last approved version.
func (s *TechStack) approvedChoices() map[string]string {
	out := map[string]string{}
	if s.State() == TechApproved && len(s.History) == 0 {
		for _, d := range s.Decisions {
			out[d.Area] = d.Choice()
		}
		return out
	}
	for i := len(s.History) - 1; i >= 0; i-- {
		if s.History[i].Status == TechApproved {
			for _, d := range s.History[i].Decisions {
				out[d.Area] = d.Choice()
			}
			break
		}
	}
	return out
}

// Reviews are the decisions waiting to be kept or changed.
func (s *TechStack) Reviews() []TechDecision {
	var out []TechDecision
	if s != nil {
		for _, d := range s.Decisions {
			if d.Review != "" {
				out = append(out, d)
			}
		}
	}
	return out
}

// Choice is the technology the project uses for this area.
func (d TechDecision) Choice() string {
	if d.Override != nil {
		return d.Override.Value
	}
	return d.Recommended
}

func (d TechDecision) Lang() string {
	if d.Override != nil {
		return d.Override.Language
	}
	return d.Language
}

// Run is the runtime of the chosen technology, "" when unknown.
func (d TechDecision) Run() string {
	if d.Override != nil {
		return d.Override.Runtime
	}
	return d.Runtime
}

// Decision returns the decision for area, nil when the area is not part of
// the product (for example mobile on a web-only product).
func (s *TechStack) Decision(area string) *TechDecision {
	if s == nil {
		return nil
	}
	for i := range s.Decisions {
		if s.Decisions[i].Area == area {
			return &s.Decisions[i]
		}
	}
	return nil
}

// Choice is the chosen technology for area, "" when absent.
func (s *TechStack) Choice(area string) string {
	if d := s.Decision(area); d != nil {
		return d.Choice()
	}
	return ""
}

// Brief is the stack as prompt lines for the AI.
func (s *TechStack) Brief() string {
	if s == nil || len(s.Decisions) == 0 {
		return ""
	}
	var b strings.Builder
	if s.Mode == StructureExisting {
		b.WriteString("TECHNOLOGY STACK (existing code; keep the [existing] technology and its conventions, no extra framework/language):\n")
	} else {
		b.WriteString("TECHNOLOGY STACK (use exactly these; no substitute framework/language/database):\n")
	}
	for _, d := range s.Decisions {
		fmt.Fprintf(&b, "- %s: %s", d.Area, d.Choice())
		if l := d.Lang(); l != "" {
			fmt.Fprintf(&b, " (language %s)", l)
		}
		if r := d.Run(); r != "" {
			fmt.Fprintf(&b, " (runtime %s)", r)
		}
		if d.Override != nil && d.Override.Reason != "" {
			fmt.Fprintf(&b, " — chosen by the user: %s", d.Override.Reason)
		}
		fmt.Fprintf(&b, " [%s]", s.DecisionState(d))
		b.WriteString("\n")
	}
	return b.String()
}

// TechReview lists the technology changes made after the UI design was
// last checked that affect it; the project is not finalized until the user
// has reviewed them. Empty when there is no draft design.
func (p *Project) TechReview() []TechChange {
	d, s := p.UI, p.Stack
	if !d.Written() || p.Finalized() || s == nil {
		return nil
	}
	var out []TechChange
	for _, v := range s.History {
		if v.Version <= d.TechVersion || !v.Design {
			continue
		}
		for _, c := range v.Changes {
			if designArea[c.Area] {
				out = append(out, c)
			}
		}
	}
	return out
}

// designArea: a change of these areas is reviewed against the design (UI
// implementation, API and data the pages show).
var designArea = map[string]bool{TechFrontend: true, TechUI: true, TechMobile: true, TechDesktop: true, TechBackend: true, TechDatabase: true}

// AffectsDesign reports whether a change of area asks for a design review.
func AffectsDesign(area string) bool { return designArea[area] }
