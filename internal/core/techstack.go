package core

import (
	"fmt"
	"strings"
)

// Technology decision areas, in the order they are decided and shown.
const (
	TechFrontend     = "frontend"
	TechMobile       = "mobile"
	TechDesktop      = "desktop"
	TechBackend      = "backend"
	TechDatabase     = "database"
	TechCache        = "cache"
	TechArchitecture = "architecture"
	TechRepository   = "repository"
)

var TechAreas = []string{TechFrontend, TechMobile, TechDesktop, TechBackend, TechDatabase, TechCache, TechArchitecture, TechRepository}

// Architecture and repository values; the project structure is derived
// from them.
const (
	ArchSimple        = "Simple"
	ArchModular       = "Modular Monolith"
	ArchMicroservices = "Microservices"
	RepoSimple        = "Simple"
	RepoMonorepo      = "Monorepo"
	CacheNone         = "Tidak perlu"
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
	Approved  bool           `json:"approved"`
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
