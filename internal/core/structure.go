package core

import (
	"maps"
	"slices"
	"time"
)

// Project structure decision of a new project: the AI designs alternatives
// for THIS system, the user adopts one, edits it (by hand or in words),
// sees the impact and approves it. The approved StructureIR is the contract
// the scaffold and the build follow. Foundry has no structure template:
// the type is the name of the style the design chose, the paths are the
// folders it chose.

// Kinds of structure parts. A module lives inside a service.
const (
	PartApp     = "app"
	PartService = "service"
	PartModule  = "module"
	PartWorker  = "worker"
	PartPackage = "package"
	// PartRule is a structure convention (internal module layout, layers,
	// dependency or naming rules); its Name is the rule text.
	PartRule = "rule"
)

// StructureIR is the project structure: what is built and where. Type
// names the architecture style in the design's own words; every part
// carries the folder the design chose for it.
type StructureIR struct {
	Type     string          `json:"type"`
	Apps     []StructurePart `json:"apps,omitempty"`
	Services []StructurePart `json:"services,omitempty"`
	Workers  []StructurePart `json:"workers,omitempty"`
	Packages []StructurePart `json:"packages,omitempty"`
	Docs     string          `json:"docs"`
	// Rules are the conventions every part follows: internal module layout,
	// layers, dependencies between modules, naming. The scaffold and the
	// build follow them like the folders.
	Rules []string `json:"rules,omitempty"`
}

// StructurePart is one application, service, worker or package. Modules
// are the domain boundaries inside a service.
type StructurePart struct {
	Name    string   `json:"name"`
	Path    string   `json:"path"`
	Modules []string `json:"modules,omitempty"`
	// ModulePaths is the folder of each module, chosen by the design; a
	// module without one lives directly under the part's folder.
	ModulePaths map[string]string `json:"module_paths,omitempty"`
	// ModuleReasons explains what each module (domain folder) is responsible
	// for; a module is a folder like any other, so it carries its reason.
	ModuleReasons map[string]string `json:"module_reasons,omitempty"`
	Reason        string            `json:"reason,omitempty"`
}

func (ir *StructureIR) Clone() *StructureIR {
	if ir == nil {
		return nil
	}
	c := *ir
	cp := func(ps []StructurePart) []StructurePart {
		out := make([]StructurePart, len(ps))
		for i, p := range ps {
			p.Modules = slices.Clone(p.Modules)
			p.ModuleReasons = maps.Clone(p.ModuleReasons)
			p.ModulePaths = maps.Clone(p.ModulePaths)
			out[i] = p
		}
		return out
	}
	c.Apps, c.Services, c.Workers, c.Packages = cp(ir.Apps), cp(ir.Services), cp(ir.Workers), cp(ir.Packages)
	c.Rules = slices.Clone(ir.Rules)
	return &c
}

// StructureOp is one edit of the structure:
//   - type: Name is the new name of the architecture style
//   - add: Kind and Name, optionally its Path; a module goes into service
//     To; a rule's Name is its text
//   - path: the part or module Name moves to folder Path
//   - remove: Kind and Name
//   - rename: Kind and Name to To
//   - move: a module Name into service To, or a service Name (with its
//     modules) into service To as modules
type StructureOp struct {
	Op     string `json:"op"`
	Kind   string `json:"kind,omitempty"`
	Name   string `json:"name"`
	To     string `json:"to,omitempty"`
	Path   string `json:"path,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// StructureImpact is what a change reaches into, per area (Repository,
// Deployment, CI/CD, API, Database, ...).
type StructureImpact struct {
	Area    string `json:"area"`
	Message string `json:"message"`
}

type StructureProposal struct {
	Source  string `json:"source"` // user | ai | recommend
	Request string `json:"request,omitempty"`
	Note    string `json:"note,omitempty"`
	// Alternative is the AI alternative this proposal came from, "" for a
	// hand edit or a plain recommendation.
	Alternative string        `json:"alternative,omitempty"`
	Ops         []StructureOp `json:"ops,omitempty"`
	IR          *StructureIR  `json:"ir"`
	Changes     []string      `json:"changes"`
	// Tech are the architecture/repository changes the new structure needs;
	// they are applied with the proposal, shown before.
	Tech     []TechChange      `json:"tech,omitempty"`
	Issues   []TechIssue       `json:"issues,omitempty"`
	Impact   []StructureImpact `json:"impact,omitempty"`
	Previous TechStatus        `json:"previous"`
	At       time.Time         `json:"at"`
}

// StructureEvaluation is how one alternative scores on the qualities that
// decide maintainability. Each score is 1 (poor) to 5 (excellent);
// Complexity is 1 (simple) to 5 (complex), so lower is better.
type StructureEvaluation struct {
	Maintainability int `json:"maintainability"`
	Simplicity      int `json:"simplicity"`
	Extensibility   int `json:"extensibility"`
	Testability     int `json:"testability"`
	Separation      int `json:"separation_of_concerns"`
	Complexity      int `json:"complexity"`
	// Score is computed from the six scores (0-100); the AI does not set it.
	Score int `json:"score,omitempty"`
}

// StructureAlternative is one complete project layout the AI considered.
type StructureAlternative struct {
	ID         string              `json:"id"`
	Name       string              `json:"name"`
	Summary    string              `json:"summary"`
	Type       string              `json:"type"`
	IR         *StructureIR        `json:"ir"`
	Evaluation StructureEvaluation `json:"evaluation"`
	Pros       []string            `json:"pros,omitempty"`
	Cons       []string            `json:"cons,omitempty"`
	Tradeoffs  []string            `json:"tradeoffs,omitempty"`
	// Recommended mirrors Plan.Recommendation for readers that look at one
	// alternative at a time.
	Recommended bool `json:"recommended,omitempty"`
}

// StructureDomain is a domain boundary the analysis found, with its
// responsibility; the folders of every alternative follow it.
type StructureDomain struct {
	Name           string `json:"name"`
	Responsibility string `json:"responsibility"`
}

// StructurePlan is the AI's analysis of the whole system and the
// alternatives it compared. It is a recommendation; the Structure IR stays
// the decision until the user adopts one alternative.
type StructurePlan struct {
	Summary      string                 `json:"summary"`
	Drivers      []string               `json:"drivers,omitempty"`
	Domains      []StructureDomain      `json:"domains,omitempty"`
	Alternatives []StructureAlternative `json:"alternatives"`
	// Recommendation is the id of the alternative the AI recommends.
	Recommendation string `json:"recommendation"`
	Reason         string `json:"reason"`
	// DependencyRules apply to every part of the chosen alternative: what may
	// import what, and what must go through a public interface or event.
	DependencyRules []string `json:"dependency_rules,omitempty"`
	// SourceHash identifies the system snapshot the analysis read. A plan
	// with the same hash is not generated again.
	SourceHash  string    `json:"source_hash,omitempty"`
	GeneratedAt time.Time `json:"generated_at,omitempty"`
	// Stale is true when the source artifacts changed after the analysis.
	Stale bool `json:"stale,omitempty"`
	// Selected is the alternative the user adopted; Decision is why.
	Selected  string    `json:"selected,omitempty"`
	Decision  string    `json:"decision,omitempty"`
	DecidedAt time.Time `json:"decided_at,omitempty"`
}

// Alternative returns the alternative with id, nil when absent.
func (p *StructurePlan) Alternative(id string) *StructureAlternative {
	if p == nil {
		return nil
	}
	for i := range p.Alternatives {
		if p.Alternatives[i].ID == id {
			return &p.Alternatives[i]
		}
	}
	return nil
}

// RecommendedAlternative returns the alternative the plan recommends.
func (p *StructurePlan) RecommendedAlternative() *StructureAlternative {
	if p == nil {
		return nil
	}
	if a := p.Alternative(p.Recommendation); a != nil {
		return a
	}
	if len(p.Alternatives) > 0 {
		return &p.Alternatives[0]
	}
	return nil
}

// SelectedAlternative returns the alternative the user adopted.
func (p *StructurePlan) SelectedAlternative() *StructureAlternative {
	if p == nil || p.Selected == "" {
		return nil
	}
	return p.Alternative(p.Selected)
}

// PlanRequest is an AI structure analysis waiting to run. Text is a
// revision request; empty means a fresh analysis of the current system.
type StructurePlanRequest struct {
	Text       string `json:"text,omitempty"`
	SourceHash string `json:"source_hash,omitempty"`
	Rejected   string `json:"rejected,omitempty"`
	Retries    int    `json:"retries,omitempty"`
}

// StructureVersion is one saved state of the structure.
type StructureVersion struct {
	Version int               `json:"version"`
	At      time.Time         `json:"at"`
	Source  string            `json:"source"` // recommend | user | ai | technology
	Request string            `json:"request,omitempty"`
	Changes []string          `json:"changes,omitempty"`
	Impact  []StructureImpact `json:"impact,omitempty"`
	Status  TechStatus        `json:"status"`
	IR      StructureIR       `json:"ir"`
	// Analysis, Alternative, Reason and Tree keep the AI recommendation
	// that was current when this version was saved.
	Analysis    string `json:"analysis,omitempty"`
	Alternative string `json:"alternative,omitempty"`
	Reason      string `json:"reason,omitempty"`
	Tree        string `json:"tree,omitempty"`
}

// State is the review status; structures saved before the lifecycle
// derive it.
func (s *Structure) State() TechStatus {
	switch {
	case s == nil:
		return ""
	case s.Status != "":
		return s.Status
	case s.Approved:
		return TechApproved
	}
	return TechRecommended
}

func (s *Structure) SetStatus(st TechStatus) {
	s.Status, s.Approved = st, st == TechApproved
	if st == TechApproved && len(s.History) > 0 {
		s.History[len(s.History)-1].Status = st
	}
}

func (s *Structure) Version() int {
	if s == nil || len(s.History) == 0 {
		return 0
	}
	return s.History[len(s.History)-1].Version
}

func (s *Structure) Errors() []TechIssue {
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
