package core

import (
	"strings"
	"time"
)

// WorkIntent is the planning job the user asked Foundry to do.
type WorkIntent string

const (
	IntentNewProject   WorkIntent = "new_project"
	IntentExisting     WorkIntent = "existing_project"
	IntentNewFeature   WorkIntent = "new_feature"
	IntentEnhancement  WorkIntent = "enhancement"
	IntentRefactoring  WorkIntent = "refactoring"
	IntentBugFix       WorkIntent = "bug_fix"
	IntentArchitecture WorkIntent = "architecture"
)

// WorkIntents is the user-facing list, in display order.
var WorkIntents = []Choice{
	{string(IntentNewProject), "Project baru"},
	{string(IntentExisting), "Pahami project existing"},
	{string(IntentNewFeature), "Fitur baru"},
	{string(IntentEnhancement), "Peningkatan"},
	{string(IntentRefactoring), "Refactoring"},
	{string(IntentBugFix), "Perbaikan bug"},
	{string(IntentArchitecture), "Perbaikan arsitektur"},
}

// ExistingWorkIntents are the jobs that apply once a codebase is present.
var ExistingWorkIntents = []Choice{
	{string(IntentNewFeature), "Fitur baru"},
	{string(IntentEnhancement), "Peningkatan"},
	{string(IntentRefactoring), "Refactoring"},
	{string(IntentBugFix), "Perbaikan bug"},
	{string(IntentArchitecture), "Perbaikan arsitektur"},
	{string(IntentExisting), "Pahami project existing"},
}

const (
	ConfidenceObserved   = "observed"   // read from code or a manifest
	ConfidenceDocumented = "documented" // read from docs, not verified in code
	ConfidenceEstimated  = "estimated"  // AI inference; the scanner never writes this
)

const (
	ContextAreaLayout  = "layout"
	ContextAreaLang    = "languages"
	ContextAreaModules = "modules"
	ContextAreaData    = "data"
	ContextAreaAPIs    = "apis"
	ContextAreaAuth    = "auth"
	ContextAreaTests   = "tests"
	ContextAreaDocs    = "docs"
	ContextAreaDeps    = "deps"
)

const (
	DepKindModule  = "module"
	DepKindPackage = "package"
	DepKindService = "service"
	DepKindAPI     = "api"
	DepKindEntity  = "entity"
	DepKindShared  = "shared"
	DepKindEvent   = "event"
	DepKindConfig  = "config"
)

const (
	ImpactDirect      = "direct"
	ImpactIndirect    = "indirect"
	ImpactBreaking    = "breaking"
	ImpactMigration   = "migration"
	ImpactAPI         = "api"
	ImpactSecurity    = "security"
	ImpactPerformance = "performance"
	ImpactRegression  = "regression"
	ImpactRollback    = "rollback"
)

const (
	BlueprintScopeFull   = "full"
	BlueprintScopeChange = "change"
)

const (
	ChangeAdd      = "add"
	ChangeModify   = "modify"
	ChangeReuse    = "reuse"
	ChangeRefactor = "refactor"
	ChangeRemove   = "remove"
)

// ParseWorkIntent accepts a user or API value; "" when unknown.
func ParseWorkIntent(s string) WorkIntent {
	v := WorkIntent(strings.ToLower(strings.TrimSpace(s)))
	if v.Valid() {
		return v
	}
	return ""
}

// Valid reports whether i is a known work intent.
func (i WorkIntent) Valid() bool {
	switch i {
	case IntentNewProject, IntentExisting, IntentNewFeature, IntentEnhancement,
		IntentRefactoring, IntentBugFix, IntentArchitecture:
		return true
	}
	return false
}

// ChangeScope reports whether this job is a delta on an existing system
// (feature, enhancement, refactor, bug, architecture) rather than a full
// project blueprint.
func (i WorkIntent) ChangeScope() bool {
	switch i {
	case IntentNewFeature, IntentEnhancement, IntentRefactoring, IntentBugFix, IntentArchitecture:
		return true
	}
	return false
}

// ClampIntent keeps new-project jobs on empty folders and existing jobs on
// codebases. An unknown value falls back to the folder's mode.
func ClampIntent(existing bool, intent WorkIntent) WorkIntent {
	if !intent.Valid() {
		if existing {
			return IntentExisting
		}
		return IntentNewProject
	}
	if existing && intent == IntentNewProject {
		return IntentExisting
	}
	if !existing && intent != IntentNewProject {
		return IntentNewProject
	}
	return intent
}

// IntentOrDefault is the project's job, inferred from the folder when unset.
func (p *Project) IntentOrDefault() WorkIntent {
	if p == nil {
		return IntentNewProject
	}
	existing := p.Structure != nil && p.Structure.Mode == StructureExisting
	return ClampIntent(existing, p.Intent)
}

// ContextFinding is one observed or documented fact about the repository.
// The scanner never writes confidence "assumed".
type ContextFinding struct {
	Kind       string `json:"kind,omitempty"`
	Text       string `json:"text"`
	Path       string `json:"path,omitempty"`
	Evidence   string `json:"evidence,omitempty"`
	Confidence string `json:"confidence,omitempty"`
}

// ContextModule is a directory that holds source of one area of the code.
// Name is the folder name; responsibility is left empty unless evidence says it.
type ContextModule struct {
	Name     string   `json:"name"`
	Path     string   `json:"path"`
	Evidence string   `json:"evidence,omitempty"`
	Depends  []string `json:"depends_on,omitempty"`
}

// ContextEntity is a data model observed in a schema file.
type ContextEntity struct {
	Name     string   `json:"name"`
	Fields   []string `json:"fields,omitempty"`
	Path     string   `json:"path,omitempty"`
	Evidence string   `json:"evidence,omitempty"`
}

// ContextAPI is an API contract or a route observed in the repository.
type ContextAPI struct {
	Kind     string `json:"kind,omitempty"` // openapi, proto, route
	Path     string `json:"path"`
	Detail   string `json:"detail,omitempty"`
	Evidence string `json:"evidence,omitempty"`
}

// ContextDep is one observed relationship in the repository.
// The scanner only writes confidence "observed".
type ContextDep struct {
	From       string `json:"from"`
	To         string `json:"to"`
	Kind       string `json:"kind,omitempty"`
	Path       string `json:"path,omitempty"`
	Evidence   string `json:"evidence,omitempty"`
	Confidence string `json:"confidence,omitempty"`
}

// ContextChange is one repository difference since the previous scan.
type ContextChange struct {
	Area   string `json:"area"`
	Path   string `json:"path,omitempty"`
	Action string `json:"action"` // add | modify | remove
}

// ProjectContext is the evidence-based picture of an existing repository.
// Empty slices mean nothing was found, not that the area does not exist.
type ProjectContext struct {
	Layout      []ContextFinding `json:"layout,omitempty"`
	Languages   []ContextFinding `json:"languages,omitempty"`
	Frameworks  []ContextFinding `json:"frameworks,omitempty"`
	Modules     []ContextModule  `json:"modules,omitempty"`
	Data        []ContextEntity  `json:"data,omitempty"`
	APIs        []ContextAPI     `json:"apis,omitempty"`
	Auth        []ContextFinding `json:"auth,omitempty"`
	Tests       []ContextFinding `json:"tests,omitempty"`
	Docs        []ContextFinding `json:"docs,omitempty"`
	Conventions []ContextFinding `json:"conventions,omitempty"`
	Features    []ContextFinding `json:"features,omitempty"`
	Deps        []ContextDep     `json:"deps,omitempty"`
	Changes     []ContextChange  `json:"changes,omitempty"`
	// ScannedAt and Fingerprint identify this reading of the repository.
	ScannedAt   time.Time         `json:"scanned_at,omitempty"`
	Fingerprint string            `json:"fingerprint,omitempty"`
	AreaHash    map[string]string `json:"area_hash,omitempty"`
	// Stale is true when the repository changed after this context was read.
	Stale      bool     `json:"stale,omitempty"`
	StaleAreas []string `json:"stale_areas,omitempty"`
	// Corrections are the user's notes on the scan; they bind later stages.
	Corrections string `json:"corrections,omitempty"`
}

// Empty reports whether the scan found nothing besides user notes.
func (c *ProjectContext) Empty() bool {
	if c == nil {
		return true
	}
	return len(c.Layout) == 0 && len(c.Languages) == 0 && len(c.Frameworks) == 0 &&
		len(c.Modules) == 0 && len(c.Data) == 0 && len(c.APIs) == 0 &&
		len(c.Auth) == 0 && len(c.Tests) == 0 && len(c.Docs) == 0 &&
		len(c.Conventions) == 0 && len(c.Features) == 0
}

// ChangeItem is one add/modify/reuse/refactor/remove action of a feature change.
type ChangeItem struct {
	Action string   `json:"action"`
	Target string   `json:"target"`
	Module string   `json:"module,omitempty"`
	Reason string   `json:"reason,omitempty"`
	Files  []string `json:"files,omitempty"`
}

// ChangeImpact is one effect of the change on the existing system.
type ChangeImpact struct {
	Area     string `json:"area"`
	Message  string `json:"message"`
	Risk     string `json:"risk,omitempty"`
	Kind     string `json:"kind,omitempty"`   // direct | indirect | breaking | ...
	Source   string `json:"source,omitempty"` // observed | estimated
	Evidence string `json:"evidence,omitempty"`
	Path     string `json:"path,omitempty"`
}

// ChangeConflict is a clash between a feature change and the main blueprint
// or a decided decision. Foundry records it instead of overwriting silently.
type ChangeConflict struct {
	Kind    string `json:"kind,omitempty"` // duplicate | contradiction | decision | dependents
	Where   string `json:"where,omitempty"`
	Message string `json:"message"`
	Keep    string `json:"keep,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

// ChangeFile is one path the change adds, modifies, or removes.
type ChangeFile struct {
	Path           string `json:"path"`
	Responsibility string `json:"responsibility,omitempty"`
	Reason         string `json:"reason,omitempty"`
}

// ChangeFiles groups repository paths by what happens to them.
type ChangeFiles struct {
	Add    []ChangeFile `json:"add,omitempty"`
	Modify []ChangeFile `json:"modify,omitempty"`
	Remove []ChangeFile `json:"remove,omitempty"`
}

// ChangePlan is the Feature Change Blueprint: the delta against the current
// system, kept next to the main blueprint so the two cannot drift silently.
type ChangePlan struct {
	Scope     string           `json:"scope,omitempty"` // full | change
	Summary   string           `json:"summary,omitempty"`
	Existing  []string         `json:"existing,omitempty"`
	Items     []ChangeItem     `json:"items,omitempty"`
	Impact    []ChangeImpact   `json:"impact,omitempty"`
	Files     ChangeFiles      `json:"files,omitempty"`
	Conflicts []ChangeConflict `json:"conflicts,omitempty"`
	Kept      []string         `json:"kept,omitempty"`
}

// Empty reports whether the plan has nothing a later stage can follow.
func (c *ChangePlan) Empty() bool {
	if c == nil {
		return true
	}
	return strings.TrimSpace(c.Summary) == "" && len(c.Items) == 0 &&
		len(c.Impact) == 0 && len(c.Existing) == 0 &&
		len(c.Files.Add) == 0 && len(c.Files.Modify) == 0 && len(c.Files.Remove) == 0 &&
		len(c.Conflicts) == 0 && len(c.Kept) == 0
}

var changeActions = map[string]bool{
	ChangeAdd: true, ChangeModify: true, ChangeReuse: true, ChangeRefactor: true, ChangeRemove: true,
}

var changeRisks = map[string]bool{"low": true, "medium": true, "high": true}

// NormalizeChangePlan drops empty rows and unknown actions. nil when nothing remains.
func NormalizeChangePlan(c *ChangePlan) *ChangePlan {
	if c == nil {
		return nil
	}
	out := &ChangePlan{
		Scope:   strings.ToLower(strings.TrimSpace(c.Scope)),
		Summary: strings.TrimSpace(c.Summary),
	}
	if out.Scope != BlueprintScopeFull && out.Scope != BlueprintScopeChange {
		out.Scope = ""
	}
	for _, s := range c.Existing {
		if s = strings.TrimSpace(s); s != "" {
			out.Existing = append(out.Existing, s)
		}
	}
	for _, it := range c.Items {
		it.Action = strings.ToLower(strings.TrimSpace(it.Action))
		it.Target = strings.TrimSpace(it.Target)
		it.Module = strings.TrimSpace(it.Module)
		it.Reason = strings.TrimSpace(it.Reason)
		if !changeActions[it.Action] || it.Target == "" {
			continue
		}
		var files []string
		for _, f := range it.Files {
			if f = strings.TrimSpace(f); f != "" {
				files = append(files, f)
			}
		}
		it.Files = files
		out.Items = append(out.Items, it)
	}
	for _, im := range c.Impact {
		im.Area = strings.ToLower(strings.TrimSpace(im.Area))
		im.Message = strings.TrimSpace(im.Message)
		im.Risk = strings.ToLower(strings.TrimSpace(im.Risk))
		if im.Area == "" || im.Message == "" {
			continue
		}
		if !changeRisks[im.Risk] {
			im.Risk = ""
		}
		im.Kind = strings.ToLower(strings.TrimSpace(im.Kind))
		im.Source = strings.ToLower(strings.TrimSpace(im.Source))
		im.Evidence = strings.TrimSpace(im.Evidence)
		im.Path = strings.TrimSpace(im.Path)
		if im.Source != ConfidenceObserved && im.Source != ConfidenceEstimated {
			im.Source = ""
		}
		out.Impact = append(out.Impact, im)
	}
	for _, cf := range c.Conflicts {
		cf.Kind = strings.ToLower(strings.TrimSpace(cf.Kind))
		cf.Where = strings.TrimSpace(cf.Where)
		cf.Message = strings.TrimSpace(cf.Message)
		cf.Keep = strings.TrimSpace(cf.Keep)
		cf.Reason = strings.TrimSpace(cf.Reason)
		if cf.Message == "" {
			continue
		}
		out.Conflicts = append(out.Conflicts, cf)
	}
	for _, s := range c.Kept {
		if s = strings.TrimSpace(s); s != "" {
			out.Kept = append(out.Kept, s)
		}
	}
	out.Files.Add = cleanChangeFiles(c.Files.Add)
	out.Files.Modify = cleanChangeFiles(c.Files.Modify)
	out.Files.Remove = cleanChangeFiles(c.Files.Remove)
	if out.Empty() && out.Scope == "" {
		return nil
	}
	return out
}

func cleanChangeFiles(in []ChangeFile) []ChangeFile {
	var out []ChangeFile
	for _, f := range in {
		f.Path = strings.TrimSpace(f.Path)
		f.Responsibility = strings.TrimSpace(f.Responsibility)
		f.Reason = strings.TrimSpace(f.Reason)
		if f.Path == "" {
			continue
		}
		out = append(out, f)
	}
	return out
}
