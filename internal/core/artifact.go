package core

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

// Artifact is one blueprint document. Built-in artifacts are rendered from
// the structured project data (requirements, architecture, design, ...);
// the AI or the user writes the others for what this project needs (a
// threat model, a firmware architecture, an evaluation plan, ...). Only
// non-built-in artifacts are stored in Project.Artifacts.
type Artifact struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Category string `json:"category"`
	Summary  string `json:"summary,omitempty"`
	Body     string `json:"body"` // Markdown
	// Refs are the ids of related artifacts; Seen their hashes when this
	// artifact was written or last reviewed, so a change of a referenced
	// artifact marks this one for review.
	Refs []string          `json:"refs,omitempty"`
	Seen map[string]string `json:"seen,omitempty"`
	// Area is the blueprint area the artifact covers (security, data, ...).
	Area      string    `json:"area,omitempty"`
	Version   int       `json:"version"`
	UpdatedAt time.Time `json:"updated_at"`
	// Builtin marks an artifact rendered from structured data; it is never
	// stored and changes only through its own data.
	Builtin bool `json:"builtin,omitempty"`
	// Status is the review status at the start of Body ("Status:
	// **approved**"). It is not content: Hash leaves it out, so approving a
	// plan does not make the commands that read it need review.
	Status string `json:"-"`
}

// Artifact categories (blueprint sections).
const (
	ArtifactRequirements = "requirements"
	ArtifactDomain       = "domain"
	ArtifactArchitecture = "architecture"
	ArtifactUI           = "ui"
	ArtifactData         = "data"
	ArtifactPlatform     = "platform"
	ArtifactQuality      = "quality"
	ArtifactOther        = "other"
)

var ArtifactCategories = []string{ArtifactRequirements, ArtifactDomain, ArtifactArchitecture, ArtifactUI,
	ArtifactData, ArtifactPlatform, ArtifactQuality, ArtifactOther}

// BuiltinArtifacts are the ids of the artifacts rendered from structured
// data; AI and user artifacts may not take them.
var BuiltinArtifacts = map[string]bool{
	"requirements": true, "architecture": true, "technology": true, ArtifactDesignRules: true, ArtifactLayoutSpec: true,
	ArtifactUseCases: true, ArtifactAcceptance: true, ArtifactDecisions: true, ArtifactSurvey: true,
	ArtifactSystemOverview: true, ArtifactRepoStructure: true, ArtifactTechPlan: true,
	ArtifactProjectContext: true, ArtifactChangePlan: true,
}

// Built-in artifacts besides requirements, architecture and technology.
const (
	ArtifactDesignRules = "design-rules"
	ArtifactLayoutSpec  = "layout-spec"
	ArtifactUseCases    = "use-cases"
	ArtifactAcceptance  = "acceptance"
	ArtifactDecisions   = "decisions"
	// ArtifactSurvey is the survey as technical decisions. It is its own
	// document so the requirements of older projects keep their hash.
	ArtifactSurvey = "survey-decisions"
	// ArtifactSystemOverview is the plain-language explanation of the
	// product, rendered from the current blueprint.
	ArtifactSystemOverview = "system-overview"
	// ArtifactRepoStructure is the repository plan: folders, files,
	// responsibilities, module mapping and dependency rules. It exists once
	// the plan does.
	ArtifactRepoStructure = "repository-structure"
	// ArtifactTechPlan is the technology plan: decisions with their
	// reasoning, alternatives, compatibility and evaluation. It exists once
	// the plan does.
	ArtifactTechPlan = "technology-plan"
	// ArtifactProjectContext is the evidence-based reading of an existing
	// repository. It exists once the scan found something.
	ArtifactProjectContext = "project-context"
	// ArtifactChangePlan is the Feature Change Blueprint: what is added,
	// modified, reused, refactored or removed, and the impact of that.
	ArtifactChangePlan = "change-plan"
)

// Hash identifies the artifact's content; a command records it to notice
// when the artifact changes after the command was written.
func (a Artifact) Hash() string {
	body := a.Body
	if a.Status != "" {
		body = strings.TrimPrefix(body, a.Status)
	}
	return hashOf(a.Title, a.Summary, body)
}

// LegacyHash is Hash before the status was left out (HashVersion 0); it
// equals Hash for an artifact without a status.
func (a Artifact) LegacyHash() string { return hashOf(a.Title, a.Summary, a.Body) }

func hashOf(title, summary, body string) string {
	sum := sha256.Sum256([]byte(title + "\x00" + summary + "\x00" + body))
	return hex.EncodeToString(sum[:6])
}

// HashVersion is the current artifact hash: 1 leaves Artifact.Status out.
const HashVersion = 1

// ChangeRequest is a natural-language change waiting for the AI. Focus
// names the artifacts or commands the user asked about (empty: any).
type ChangeRequest struct {
	Text  string   `json:"text"`
	Focus []string `json:"focus,omitempty"`
	// Area is the one blueprint area this request writes, when the first
	// blueprint is built area by area; the UI shows that area as running.
	Area string `json:"area,omitempty"`
	// Rejected is why the AI's last answer was refused; the retry prompt
	// shows it.
	Rejected string `json:"rejected,omitempty"`
	// Cascade marks a request made by deleting a document or by changed
	// decisions: the artifact request is followed by the workflow request.
	Cascade bool `json:"cascade,omitempty"`
	// Sync marks a request that brings the blueprint or the commands in
	// line with changed decisions; an answer without changes is valid,
	// nothing contradicted them.
	Sync bool `json:"sync,omitempty"`
	// Handled lists what the earlier rounds of a workflow revision did
	// ("changed CMD-002", "added CMD-007", "removed CMD-004"): a revision
	// makes one command change per round.
	Handled []string `json:"handled,omitempty"`
}

// HasUI reports whether the product has screens to design.
func (p *Project) HasUI() bool {
	if p.Platform != nil {
		for _, t := range p.Platform.Targets {
			switch t.Kind {
			case PlatformWeb, PlatformAndroid, PlatformIOS, PlatformDesktop:
				return true
			}
		}
	}
	return p.UI != nil && p.UI.Layout != nil && len(p.UI.Layout.Pages) > 0
}

// Finalized reports whether the blueprint and workflow are approved.
func (p *Project) Finalized() bool { return p.Stage == StageReady }
