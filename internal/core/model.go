// Package core holds the single project context shared by every stage.
// All modules read and write this model through Store; no stage keeps its
// own copy of project data.
package core

import "time"

// Stage is the internal pipeline position. The UI groups these into
// Ceritakan → Rancang → Bangun → Siap.
type Stage string

const (
	StageTell      Stage = "TELL"
	StageDiscovery Stage = "DISCOVERY"
	StageDesign    Stage = "DESIGN"
	StagePlan      Stage = "PLAN"
	StageBuild     Stage = "BUILD"
	StageVerify    Stage = "VERIFY"
	StageReady     Stage = "READY"
)

type TaskStatus string

const (
	TaskTodo       TaskStatus = "TODO"
	TaskInProgress TaskStatus = "IN_PROGRESS"
	TaskReview     TaskStatus = "REVIEW"
	TaskDone       TaskStatus = "DONE"
	TaskBlocked    TaskStatus = "BLOCKED"
)

type Project struct {
	Version      int           `json:"version"`
	Name         string        `json:"name"`
	Dir          string        `json:"dir"`
	Stage        Stage         `json:"stage"`
	Story        string        `json:"story"`
	Discovery    *Discovery    `json:"discovery,omitempty"`
	Requirements []Requirement `json:"requirements"`
	Features     []Feature     `json:"features"`
	Flows        []Flow        `json:"flows"`
	Platform     *PlatformPlan `json:"platform,omitempty"`
	Structure    *Structure    `json:"structure,omitempty"`
	Design       *Design       `json:"design,omitempty"`
	Tasks        []Task        `json:"tasks"`
	Builds       []BuildRun    `json:"builds"`
	Verification *Verification `json:"verification,omitempty"`
	Job          *Job          `json:"job,omitempty"`
	Events       []Event       `json:"events"`
	UpdatedAt    time.Time     `json:"updated_at"`
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
	Template   string        `json:"template,omitempty"`
	Layout     []string      `json:"layout,omitempty"`
	Reason     string        `json:"reason"`
	Commands   []Command     `json:"commands"`
	Approved   bool          `json:"approved"`
	Scaffolded bool          `json:"scaffolded"`
}

type Component struct {
	Kind     string `json:"kind"` // backend | web | mobile | desktop | shared | database | infra | testing
	Path     string `json:"path"`
	Tech     string `json:"tech"`
	Evidence string `json:"evidence"`
}

// Command is a verification step detected from real project tooling.
type Command struct {
	Name string   `json:"name"` // build | typecheck | lint | vet | test
	Dir  string   `json:"dir"`  // relative to project dir
	Argv []string `json:"argv"`
}

type DesignStatus string

const (
	DesignDraft    DesignStatus = "draft"
	DesignChanges  DesignStatus = "changes_requested"
	DesignApproved DesignStatus = "approved"
)

type Design struct {
	Pages    []Page       `json:"pages"`
	Status   DesignStatus `json:"status"`
	Feedback []string     `json:"feedback"`
	// Revision is the last requested change, waiting for the user to keep or
	// undo it. Nil when there is nothing to review.
	Revision *DesignRevision `json:"revision,omitempty"`
}

// DesignRevision records what one change request did to the design and the
// state needed to undo it.
type DesignRevision struct {
	Request       string              `json:"request"`
	Changes       []DesignChange      `json:"changes"`
	PreviousPages []Page              `json:"previous_pages"`
	PreviousFlows map[string][]string `json:"previous_flows"` // flow id -> page ids
}

// DesignChange is one page-level difference between two design versions.
type DesignChange struct {
	Kind    string   `json:"kind"`              // added | changed | removed
	PageID  string   `json:"page_id,omitempty"` // id in the new design; empty when removed
	Page    string   `json:"page"`
	Details []string `json:"details,omitempty"`
}

type Page struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	Platform      string          `json:"platform"`
	Role          string          `json:"role"`
	FeatureID     string          `json:"feature_id"`
	SubFeatureIDs []string        `json:"sub_feature_ids"`
	Purpose       string          `json:"purpose"`
	Components    []PageComponent `json:"components"`
}

// PageComponent is a wireframe element. Target links to another page for
// prototype navigation.
type PageComponent struct {
	Type   string   `json:"type"` // header | nav | text | list | table | form | button | card | stat | input
	Label  string   `json:"label"`
	Items  []string `json:"items,omitempty"`
	Target string   `json:"target,omitempty"`
}

type Task struct {
	ID                 string      `json:"id"`
	Title              string      `json:"title"`
	Description        string      `json:"description"`
	Area               string      `json:"area"` // scaffold | data | api | ui | test | integration
	RequirementIDs     []string    `json:"requirement_ids"`
	FeatureID          string      `json:"feature_id"`
	PageIDs            []string    `json:"page_ids"`
	Platform           string      `json:"platform"`
	DependsOn          []string    `json:"depends_on"`
	AcceptanceCriteria []string    `json:"acceptance_criteria"`
	Files              []string    `json:"files"`
	TestRequirements   []string    `json:"test_requirements"`
	Status             TaskStatus  `json:"status"`
	Attempts           int         `json:"attempts"`
	Notes              []string    `json:"notes"`
	LastBuildID        string      `json:"last_build_id,omitempty"`
	Criteria           []Criterion `json:"criteria,omitempty"`
}

type Criterion struct {
	Text     string `json:"text"`
	Met      bool   `json:"met"`
	Evidence string `json:"evidence"`
}

type BuildRun struct {
	ID     string       `json:"id"`
	TaskID string       `json:"task_id,omitempty"`
	Steps  []StepResult `json:"steps"`
	Passed bool         `json:"passed"`
	Reason string       `json:"reason,omitempty"`
	At     time.Time    `json:"at"`
}

type StepResult struct {
	Name     string `json:"name"`
	Command  string `json:"command"`
	Dir      string `json:"dir"`
	ExitCode int    `json:"exit_code"`
	Output   string `json:"output"`
	Millis   int64  `json:"millis"`
}

type Verification struct {
	Ready  bool      `json:"ready"`
	Checks []Check   `json:"checks"`
	Issues []Issue   `json:"issues"`
	Trace  []Trace   `json:"trace"`
	At     time.Time `json:"at"`
}

type Check struct {
	Name string `json:"name"`
	OK   bool   `json:"ok"`
	Info string `json:"info"`
}

type Issue struct {
	Kind    string `json:"kind"` // missing_feature | missing_page | missing_task | missing_test | task_open | task_blocked | build_failed | test_failed | no_commands | broken_flow
	Ref     string `json:"ref"`
	Message string `json:"message"`
}

// Trace is one requirement's chain down to verification.
type Trace struct {
	RequirementID string   `json:"requirement_id"`
	FeatureIDs    []string `json:"feature_ids"`
	PageIDs       []string `json:"page_ids"`
	TaskIDs       []string `json:"task_ids"`
	TestTaskIDs   []string `json:"test_task_ids"`
	Done          bool     `json:"done"`
}

type JobStatus string

const (
	JobRunning JobStatus = "running"
	JobDone    JobStatus = "done"
	JobFailed  JobStatus = "failed"
	JobWaiting JobStatus = "waiting_host" // MCP mode: waiting for the AI host
)

type Job struct {
	Kind    string    `json:"kind"` // discovery | design | plan | build | verify | fix
	Status  JobStatus `json:"status"`
	Message string    `json:"message"`
	Error   string    `json:"error,omitempty"`
	TaskID  string    `json:"task_id,omitempty"`
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

func (p *Project) Task(id string) *Task {
	for i := range p.Tasks {
		if p.Tasks[i].ID == id {
			return &p.Tasks[i]
		}
	}
	return nil
}

func (p *Project) Build(id string) *BuildRun {
	for i := range p.Builds {
		if p.Builds[i].ID == id {
			return &p.Builds[i]
		}
	}
	return nil
}

func (p *Project) Feature(id string) *Feature {
	for i := range p.Features {
		if p.Features[i].ID == id {
			return &p.Features[i]
		}
	}
	return nil
}

func (p *Project) Page(id string) *Page {
	if p.Design == nil {
		return nil
	}
	for i := range p.Design.Pages {
		if p.Design.Pages[i].ID == id {
			return &p.Design.Pages[i]
		}
	}
	return nil
}

// ReadyTasks returns TODO tasks whose dependencies are all DONE.
func (p *Project) ReadyTasks() []*Task {
	var out []*Task
	for i := range p.Tasks {
		t := &p.Tasks[i]
		if t.Status != TaskTodo {
			continue
		}
		ok := true
		for _, d := range t.DependsOn {
			if dt := p.Task(d); dt == nil || dt.Status != TaskDone {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, t)
		}
	}
	return out
}

// Busy reports whether a job currently owns the project.
func (p *Project) Busy() bool {
	return p.Job != nil && (p.Job.Status == JobRunning)
}
