package core

import (
	"fmt"
	"strings"
	"time"
)

// Technology decision areas, in the order they are decided and shown.
const (
	TechFrontend      = "frontend"
	TechMobile        = "mobile"
	TechDesktop       = "desktop"
	TechBackend       = "backend"
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
)

var TechAreas = []string{TechFrontend, TechMobile, TechDesktop, TechBackend, TechDatabase, TechCache, TechMessaging,
	TechWorkers, TechStorage, TechSearch, TechArchitecture, TechRepository, TechDeployment, TechObservability}

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
	// Previous is the status to return to when the proposal is cancelled.
	Previous TechStatus `json:"previous"`
	At       time.Time  `json:"at"`
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
		out[i] = d
	}
	return out
}

type TechPlatform struct {
	Web     bool `json:"web"`
	Mobile  bool `json:"mobile"`
	Desktop bool `json:"desktop"`
	Backend bool `json:"backend"`
}

type TechDecision struct {
	Area        string `json:"area"`
	Recommended string `json:"recommended"`
	Language    string `json:"language,omitempty"`
	Reason      string `json:"reason"`
	// Alternatives are other known options worth considering.
	Alternatives []string `json:"alternatives,omitempty"`
	// Override is the user's choice; it wins over the recommendation.
	Override *TechOverride `json:"override,omitempty"`
}

type TechOverride struct {
	Value    string `json:"value"`
	Language string `json:"language,omitempty"`
	Reason   string `json:"reason,omitempty"`
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
		b.WriteString("TECHNOLOGY STACK (read from the existing code; keep it, do not add another framework or language):\n")
	} else {
		b.WriteString("TECHNOLOGY STACK (decided with the user; use exactly these, do not substitute another framework, language or database):\n")
	}
	for _, d := range s.Decisions {
		fmt.Fprintf(&b, "- %s: %s", d.Area, d.Choice())
		if l := d.Lang(); l != "" {
			fmt.Fprintf(&b, " (language %s)", l)
		}
		if d.Override != nil && d.Override.Reason != "" {
			fmt.Fprintf(&b, " — chosen by the user: %s", d.Override.Reason)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// TechReview lists the technology changes made after the design was last
// checked that affect it; the design is not approved until the user has
// reviewed them. Empty when there is no draft design.
func (p *Project) TechReview() []TechChange {
	d, s := p.Design, p.Stack
	if d == nil || len(d.Pages) == 0 || d.Status == DesignApproved || s == nil {
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
var designArea = map[string]bool{TechFrontend: true, TechMobile: true, TechDesktop: true, TechBackend: true, TechDatabase: true}

// AffectsDesign reports whether a change of area asks for a design review.
func AffectsDesign(area string) bool { return designArea[area] }
