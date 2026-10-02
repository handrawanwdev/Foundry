package core

import (
	"slices"
	"time"
)

// Project structure decision of a new project: Foundry recommends one, the
// user edits it (by hand or in words), sees the impact and approves it. The
// approved StructureIR is the contract the scaffold and the build follow.

// Structure types, from the simplest.
const (
	StructSimple        = "simple"
	StructModular       = "modular"
	StructMonorepo      = "monorepo"
	StructMicroservices = "microservices"
)

var StructureTypes = []string{StructSimple, StructModular, StructMonorepo, StructMicroservices}

// Kinds of structure parts. A module lives inside a service.
const (
	PartApp     = "app"
	PartService = "service"
	PartModule  = "module"
	PartWorker  = "worker"
	PartPackage = "package"
)

// StructureIR is the project structure: what is built and where. Paths are
// derived from the type and names, so the IR alone decides the folders.
type StructureIR struct {
	Type     string          `json:"type"`
	Apps     []StructurePart `json:"apps,omitempty"`
	Services []StructurePart `json:"services,omitempty"`
	Workers  []StructurePart `json:"workers,omitempty"`
	Packages []StructurePart `json:"packages,omitempty"`
	Docs     string          `json:"docs"`
}

// StructurePart is one application, service, worker or package. Modules
// are the domain boundaries inside a service.
type StructurePart struct {
	Name    string   `json:"name"`
	Path    string   `json:"path"`
	Modules []string `json:"modules,omitempty"`
	Reason  string   `json:"reason,omitempty"`
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
			out[i] = p
		}
		return out
	}
	c.Apps, c.Services, c.Workers, c.Packages = cp(ir.Apps), cp(ir.Services), cp(ir.Workers), cp(ir.Packages)
	return &c
}

// StructureOp is one edit of the structure:
//   - type: Name is the new structure type
//   - add: Kind and Name; a module goes into service To
//   - remove: Kind and Name
//   - rename: Kind and Name to To
//   - move: a module Name into service To, or a service Name (with its
//     modules) into service To as modules
type StructureOp struct {
	Op     string `json:"op"`
	Kind   string `json:"kind,omitempty"`
	Name   string `json:"name"`
	To     string `json:"to,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// StructureImpact is what a change reaches into, per area (Repository,
// Deployment, CI/CD, API, Database, ...).
type StructureImpact struct {
	Area    string `json:"area"`
	Message string `json:"message"`
}

type StructureProposal struct {
	Source  string        `json:"source"` // user | ai | recommend
	Request string        `json:"request,omitempty"`
	Note    string        `json:"note,omitempty"`
	Ops     []StructureOp `json:"ops,omitempty"`
	IR      *StructureIR  `json:"ir"`
	Changes []string      `json:"changes"`
	// Tech are the architecture/repository changes the new structure needs;
	// they are applied with the proposal, shown before.
	Tech     []TechChange      `json:"tech,omitempty"`
	Issues   []TechIssue       `json:"issues,omitempty"`
	Impact   []StructureImpact `json:"impact,omitempty"`
	Previous TechStatus        `json:"previous"`
	At       time.Time         `json:"at"`
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
