package core

import (
	"encoding/json"
	"path"
	"strings"
	"time"
)

// Repository plan: the AI designs how the source code of the product is
// organized — repository topology, every folder and important file with its
// responsibility, where each project module lives, which dependencies are
// allowed and forbidden — and the user reviews, revises and approves it.
// Foundry stores and exports the plan; it never writes the project's code.
//
// The plan is free-form: Foundry does not pick the organization approach
// from a catalog. The coarse Structure IR (apps, services, modules, ...)
// stays the architecture decision; every IR part is mapped to the folder
// that implements it (RepoNode.Part), wherever the AI put it.

// Owners that are not a project module. Every other owner must be a
// project module id.
const (
	OwnerShared   = "shared"   // code several modules use
	OwnerPlatform = "platform" // build, config, tooling, docs, infrastructure, app shells
)

// Node types.
const (
	NodeDir  = "dir"
	NodeFile = "file"
)

// RepoNode is one folder or file of the repository. Path is relative to the
// repository root; a folder ends with "/".
type RepoNode struct {
	Path string `json:"path"`
	// Name and Parent are derived from Path.
	Name           string `json:"name,omitempty"`
	Type           string `json:"type"`
	Role           string `json:"role,omitempty"` // free text: source, test, config, docs, infra, ...
	Responsibility string `json:"responsibility"`
	// Owner is the project module that owns the node, or shared/platform.
	Owner string `json:"owner"`
	// DependsOn are the node paths or module ids this node may use.
	DependsOn []string `json:"depends_on,omitempty"`
	Reason    string   `json:"reason"`
	// Part links the node to a Structure IR part: app:web, service:api,
	// module:booking, worker:x or package:contracts.
	Part string `json:"part,omitempty"`
	// SharedBy lists the modules that use a shared node.
	SharedBy []string `json:"shared_by,omitempty"`
	// Allowed and Forbidden say what code may and may not live in a folder.
	Allowed   []string `json:"allowed,omitempty"`
	Forbidden []string `json:"forbidden,omitempty"`
	// Relations is how the node relates to other modules or folders;
	// Boundary is where its ownership ends.
	Relations string `json:"relations,omitempty"`
	Boundary  string `json:"boundary,omitempty"`
	// Implied marks a parent folder the AI did not describe; Foundry added it.
	Implied bool `json:"implied,omitempty"`
}

// IsDir reports whether the node is a folder.
func (n RepoNode) IsDir() bool { return n.Type == NodeDir }

// Parent is the folder that contains the node, "" at the repository root.
func (n RepoNode) Parent() string {
	p := strings.TrimSuffix(n.Path, "/")
	i := strings.LastIndex(p, "/")
	if i < 0 {
		return ""
	}
	return p[:i+1]
}

// Depth counts the path segments: "a/" is 1, "a/b.go" is 2.
func (n RepoNode) Depth() int { return strings.Count(strings.TrimSuffix(n.Path, "/"), "/") + 1 }

// BaseName is the last path segment.
func (n RepoNode) BaseName() string { return path.Base(strings.TrimSuffix(n.Path, "/")) }

// RepoLocation is one place a module's code lives.
type RepoLocation struct {
	Path string `json:"path"`
	Role string `json:"role,omitempty"` // ui, api, domain, data, test, ...
}

// RepoModuleMap places one project module in the repository.
type RepoModuleMap struct {
	Module    string         `json:"module"`
	Locations []RepoLocation `json:"locations,omitempty"`
	// Frontend and Backend say whether the module has an implementation
	// there. A module with neither explains why in Note (documentation,
	// infrastructure, a vendor service, ...).
	Frontend bool   `json:"frontend"`
	Backend  bool   `json:"backend"`
	Note     string `json:"note,omitempty"`
}

// Dependency modes.
const (
	DepSync  = "sync"  // direct call or import
	DepEvent = "event" // through an event or queue; does not couple modules
)

// RepoDepRule is one dependency rule. From and To are a module id,
// shared/platform, or a node path (a folder covers everything below it).
type RepoDepRule struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Via    string `json:"via,omitempty"` // the public interface, event or contract used
	Mode   string `json:"mode,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// RepoInterface is the public entry point of a module.
type RepoInterface struct {
	Module string `json:"module"`
	Path   string `json:"path"`
	Reason string `json:"reason,omitempty"`
}

// RepoDependency is the dependency architecture of the repository.
type RepoDependency struct {
	// Direction says which way dependencies point, in one or two sentences.
	Direction     string          `json:"direction"`
	Allowed       []RepoDepRule   `json:"allowed,omitempty"`
	Forbidden     []RepoDepRule   `json:"forbidden,omitempty"`
	Communication []RepoDepRule   `json:"communication,omitempty"`
	Interfaces    []RepoInterface `json:"interfaces,omitempty"`
	SharedRules   []string        `json:"shared_rules,omitempty"`
	// CycleGuard is how circular dependencies are prevented.
	CycleGuard string `json:"cycle_guard,omitempty"`
}

// Quality criteria of the evaluation.
var RepoCriteria = []string{
	"cohesion", "coupling", "ownership", "maintainability", "discoverability",
	"testability", "extensibility", "simplicity", "complexity",
}

// RepoCriterion is one evaluated quality with the concrete reason for its
// score: 1 (poor) to 5 (excellent); complexity is 1 (simple) to 5 (complex).
type RepoCriterion struct {
	Name   string `json:"name"`
	Score  int    `json:"score"`
	Reason string `json:"reason"`
}

// Finding categories.
var RepoFindingCategories = []string{
	"unnecessary-directory", "unnecessary-abstraction", "responsibility-overlap", "circular-dependency",
	"excessive-coupling", "poor-naming", "duplication", "over-engineering", "other",
}

// RepoFinding is a weakness of the structure. Source is "ai" for the
// architect's own review and "computed" for what Foundry measured.
type RepoFinding struct {
	Category   string `json:"category"`
	Severity   string `json:"severity"` // info | warning | error
	Path       string `json:"path,omitempty"`
	Message    string `json:"message"`
	Resolution string `json:"resolution,omitempty"`
	Source     string `json:"source,omitempty"`
}

// RepoEvaluation is the quality review of the structure.
type RepoEvaluation struct {
	Summary  string          `json:"summary,omitempty"`
	Criteria []RepoCriterion `json:"criteria"`
	Findings []RepoFinding   `json:"findings"`
	// Score is computed from the criteria (0-100); the AI does not set it.
	Score int `json:"score"`
	// Outdated is true after a manual edit: the review no longer describes
	// the structure until the AI evaluates it again.
	Outdated bool `json:"outdated,omitempty"`
}

// Consistency areas the architect must verify.
var RepoConsistencyAreas = []string{
	"architecture_identity", "project_modules", "folder_responsibility",
	"technology", "dependency_rules", "architecture_rules", "structure_decision",
}

// Consistency statuses.
const (
	CheckConsistent = "consistent"
	CheckResolved   = "resolved" // a conflict was found and fixed in this plan
	CheckConflict   = "conflict" // an unresolved conflict; blocks approval
)

// RepoCheck is one consistency check between the repository structure and
// another artifact. Source is "ai" or "computed".
type RepoCheck struct {
	Area   string `json:"area"`
	Status string `json:"status"`
	Note   string `json:"note,omitempty"`
	Source string `json:"source,omitempty"`
}

// RepoDecision records a design decision of the structure and why.
type RepoDecision struct {
	Title        string   `json:"title"`
	Choice       string   `json:"choice"`
	Reason       string   `json:"reason"`
	Alternatives []string `json:"alternatives,omitempty"`
	Tradeoff     string   `json:"tradeoff,omitempty"`
}

// RepoEdge is one dependency between two units (modules, shared, platform)
// for the diagram. Kind: derived | allowed | forbidden | communication.
type RepoEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"`
	Mode string `json:"mode,omitempty"`
}

// RepoUnit is one unit of the dependency graph with its measures.
type RepoUnit struct {
	Name   string `json:"name"`
	Nodes  int    `json:"nodes"`
	FanOut int    `json:"fan_out"`
	FanIn  int    `json:"fan_in"`
}

// RepoAnalysis is everything Foundry computes from the plan and the project:
// it is recomputed on every read and never trusted from the AI.
type RepoAnalysis struct {
	Issues   []TechIssue   `json:"issues,omitempty"`
	Findings []RepoFinding `json:"findings,omitempty"`
	Checks   []RepoCheck   `json:"checks,omitempty"`
	Units    []RepoUnit    `json:"units,omitempty"`
	Edges    []RepoEdge    `json:"edges,omitempty"`
	Dirs     int           `json:"dirs"`
	Files    int           `json:"files"`
	MaxDepth int           `json:"max_depth"`
	// StaleAreas are the source areas that changed since the plan was
	// written; Affected are the plan sections that must be re-checked.
	StaleAreas []string `json:"stale_areas,omitempty"`
	Affected   []string `json:"affected,omitempty"`
}

// RepoOp is one revision of the plan. Every set of ops carries a reason.
//
//	set_node: Node is added or replaced (its parents are implied)
//	remove_node: Path and everything below it
//	move_node: Path (and everything below) moves to To
//	set_module / remove_module: Module
//	add_rule / remove_rule: Text (an architecture rule)
//	add_dependency / remove_dependency: Kind allowed|forbidden|communication; Dep (remove matches From and To)
//	set_interface / remove_interface: Interface (remove matches Module and Path)
//	set_meta: Approach, Topology and Summary that are not empty
type RepoOp struct {
	Op        string         `json:"op"`
	Path      string         `json:"path,omitempty"`
	To        string         `json:"to,omitempty"`
	Node      *RepoNode      `json:"node,omitempty"`
	Module    *RepoModuleMap `json:"module,omitempty"`
	Text      string         `json:"text,omitempty"`
	Kind      string         `json:"kind,omitempty"`
	Dep       *RepoDepRule   `json:"dep,omitempty"`
	Interface *RepoInterface `json:"interface,omitempty"`
	Approach  string         `json:"approach,omitempty"`
	Topology  string         `json:"topology,omitempty"`
	Summary   string         `json:"summary,omitempty"`
}

// RepoVersion is one entry of the revision history.
type RepoVersion struct {
	Version int        `json:"version"`
	At      time.Time  `json:"at"`
	Source  string     `json:"source"` // ai | user | generate
	Request string     `json:"request,omitempty"`
	Reason  string     `json:"reason,omitempty"`
	Changes []string   `json:"changes,omitempty"`
	Status  TechStatus `json:"status"`
	Nodes   int        `json:"nodes"`
	Score   int        `json:"score"`
}

// RepoRequest is an AI job waiting for an answer: a fresh design
// ("generate") or a revision of the current plan ("revise").
type RepoRequest struct {
	Action     string   `json:"action"`
	Text       string   `json:"text,omitempty"`
	Reason     string   `json:"reason,omitempty"`
	Areas      []string `json:"areas,omitempty"`
	SourceHash string   `json:"source_hash,omitempty"`
	// Rejected is why the AI's last answer was refused; the retry prompt
	// shows it. Retries counts the automatic retries since "Coba lagi".
	Rejected string `json:"rejected,omitempty"`
	Retries  int    `json:"retries,omitempty"`
}

const (
	RepoGenerate = "generate"
	RepoRevise   = "revise"
)

// RepoPlan is the repository structure plan of a new project.
type RepoPlan struct {
	// Approach is the name the architect gives its organization of the
	// code (free text, never chosen from a list); Topology says how the
	// repositories and root folders are laid out.
	Approach  string `json:"approach"`
	Topology  string `json:"topology"`
	Summary   string `json:"summary"`
	Rationale string `json:"rationale,omitempty"`

	Nodes      []RepoNode      `json:"nodes"`
	Modules    []RepoModuleMap `json:"modules"`
	Dependency RepoDependency  `json:"dependency"`
	// Rules are the architecture rules of the repository: where code goes,
	// naming, public interfaces, tests, configuration.
	Rules       []string       `json:"rules,omitempty"`
	Evaluation  RepoEvaluation `json:"evaluation"`
	Consistency []RepoCheck    `json:"consistency,omitempty"`
	Decisions   []RepoDecision `json:"decisions,omitempty"`

	Status  TechStatus    `json:"status"`
	History []RepoVersion `json:"history,omitempty"`
	// Sources are hashes of the source areas the plan was written from;
	// GeneratedAt is when the AI last wrote or reviewed the whole plan.
	Sources     map[string]string `json:"sources,omitempty"`
	GeneratedAt time.Time         `json:"generated_at,omitempty"`
	// Stale is true when a source area changed after the plan was written.
	Stale bool `json:"stale,omitempty"`

	Analysis RepoAnalysis `json:"analysis"`
}

// Version is the number of the last revision.
func (r *RepoPlan) Version() int {
	if r == nil || len(r.History) == 0 {
		return 0
	}
	return r.History[len(r.History)-1].Version
}

// State is the review status.
func (r *RepoPlan) State() TechStatus {
	if r == nil {
		return ""
	}
	if r.Status == "" {
		return TechRecommended
	}
	return r.Status
}

// Node returns the node at path (a folder with or without its trailing
// slash), nil when absent.
func (r *RepoPlan) Node(p string) *RepoNode {
	if r == nil {
		return nil
	}
	for i := range r.Nodes {
		if r.Nodes[i].Path == p || r.Nodes[i].Path == p+"/" {
			return &r.Nodes[i]
		}
	}
	return nil
}

// Errors are the issues that block approval.
func (r *RepoPlan) Errors() []TechIssue {
	var out []TechIssue
	if r != nil {
		for _, i := range r.Analysis.Issues {
			if i.Level == "error" {
				out = append(out, i)
			}
		}
	}
	return out
}

// Clone is a deep copy of the plan, so a revision can fail without
// touching the real one.
func (r *RepoPlan) Clone() *RepoPlan {
	if r == nil {
		return nil
	}
	b, err := json.Marshal(r)
	if err != nil {
		c := *r
		return &c
	}
	var c RepoPlan
	if err := json.Unmarshal(b, &c); err != nil {
		c = *r
	}
	return &c
}
