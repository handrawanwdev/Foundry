package core

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strings"
	"time"
)

// Workflow is the set of instructions an external coding agent follows to
// implement the blueprint. Foundry writes and validates it; it never runs
// it. Commands depend on each other as a directed acyclic graph.
type Workflow struct {
	// ID is stable across revisions so progress and exports can name the
	// workflow even when commands are rewritten.
	ID       string    `json:"id"`
	Status   string    `json:"status"` // draft | finalized
	Commands []Command `json:"commands"`
	// Request is a change waiting for the AI; nil when the next AI job
	// writes the workflow from scratch.
	Request *ChangeRequest `json:"request,omitempty"`
	// Version counts the changes; the export names it.
	Version     int        `json:"version"`
	FinalizedAt *time.Time `json:"finalized_at,omitempty"`
	// Changes is the delta log of the workflow, newest last. Revising a
	// command never loses its earlier text: Before keeps it.
	Changes []WorkflowChange `json:"changes,omitempty"`
}

// WorkflowChange is one revision of the workflow as a delta: what was added,
// modified and removed; every other command was reused as it was.
type WorkflowChange struct {
	Version  int             `json:"version"`
	At       time.Time       `json:"at"`
	Kind     string          `json:"kind"` // generate | revise | edit | remove | visual
	Reason   string          `json:"reason,omitempty"`
	Source   string          `json:"source,omitempty"`
	Added    []string        `json:"added,omitempty"`
	Modified []CommandChange `json:"modified,omitempty"`
	Removed  []CommandChange `json:"removed,omitempty"`
	Reused   int             `json:"reused"`
}

// CommandChange is one command as it was before the change.
type CommandChange struct {
	ID string `json:"id"`
	// Revision is the revision after the change; 0 for a removed command.
	Revision int      `json:"revision,omitempty"`
	Fields   []string `json:"fields,omitempty"`
	Before   Command  `json:"before"`
}

// Command priority and complexity levels.
const (
	LevelHigh   = "high"
	LevelMedium = "medium"
	LevelLow    = "low"
)

// Workflow and command statuses: design states only, never execution.
const (
	CommandDraft       = "draft"
	CommandNeedsReview = "needs_review"
	CommandFinalized   = "finalized"
)

// Command is one instruction for a coding agent.
type Command struct {
	ID string `json:"id"` // CMD-001
	// WorkflowID is the parent workflow; it stays explicit for clients that
	// read commands flat.
	WorkflowID   string   `json:"workflow_id,omitempty"`
	Name         string   `json:"name"`
	Objective    string   `json:"objective"`
	Context      string   `json:"context,omitempty"`
	Inputs       []string `json:"inputs,omitempty"`
	Instructions string   `json:"instructions"` // Markdown
	DependsOn    []string `json:"depends_on,omitempty"`
	// Artifacts are the blueprint artifacts the command implements, with
	// the content hash seen when it was written or last reviewed.
	Artifacts []ArtifactRef `json:"artifacts,omitempty"`
	// Covers are the requirements the command implements; their acceptance
	// criteria are what it must prove.
	Covers []string `json:"covers,omitempty"`
	// FeatureIDs and SubFeatureIDs link the command to the Feature Discovery
	// tree. Progress rolls up through these links; a command without them
	// falls back to the features of the requirements it covers.
	FeatureIDs    []string `json:"feature_ids,omitempty"`
	SubFeatureIDs []string `json:"sub_feature_ids,omitempty"`
	Outputs       []string `json:"expected_outputs,omitempty"`
	Acceptance    []string `json:"acceptance_criteria,omitempty"`
	Constraints   []string `json:"constraints,omitempty"`
	// Files are the files, folders or modules the command touches, taken
	// from the project structure; empty when the repository is unknown.
	Files []string `json:"files,omitempty"`
	// Priority orders commands that are ready at the same time; Complexity
	// is the planner's estimate. Both are high | medium | low.
	Priority   string `json:"priority,omitempty"`
	Complexity string `json:"complexity,omitempty"`
	// Revision counts the changes of the definition; 0 and 1 both mean the
	// first version. Work done on an older revision has to be redone.
	Revision int    `json:"revision,omitempty"`
	Status   string `json:"status"`
	// Review is why the command was marked for review.
	Review string `json:"review,omitempty"`
}

type ArtifactRef struct {
	ID   string `json:"id"`
	Hash string `json:"hash,omitempty"`
}

// Command returns the command with this id, or nil.
func (w *Workflow) Command(id string) *Command {
	if w == nil {
		return nil
	}
	for i := range w.Commands {
		if w.Commands[i].ID == id {
			return &w.Commands[i]
		}
	}
	return nil
}

// EnsureID gives an older workflow its stable id. Called while loading a
// project; it never changes an id that is already set.
func (w *Workflow) EnsureID() {
	if w == nil {
		return
	}
	if w.ID == "" {
		w.ID = "WF-001"
	}
	for i := range w.Commands {
		if w.Commands[i].WorkflowID == "" {
			w.Commands[i].WorkflowID = w.ID
		}
	}
}

// CommandRevision is the revision of the definition, at least 1.
func CommandRevision(c Command) int { return max(c.Revision, 1) }

// CommandComplexity is the planner's estimate, or for commands written
// without one a level derived from the command's deterministic weight.
func CommandComplexity(c Command) string {
	if c.Complexity != "" {
		return c.Complexity
	}
	switch w := CommandWeight(c); {
	case w >= 9:
		return LevelHigh
	case w >= 5:
		return LevelMedium
	}
	return LevelLow
}

// CommandFields names what an agent is told by the command, one fingerprint
// per field. Design state (status, review, revision) and artifact hashes are
// left out: accepting a blueprint change leaves the instruction as it was.
func CommandFields(c Command) map[string]string {
	refs := make([]string, len(c.Artifacts))
	for i, r := range c.Artifacts {
		refs[i] = r.ID
	}
	join := func(items []string) string { return strings.Join(items, "\x00") }
	out := map[string]string{}
	for name, text := range map[string]string{
		"name": c.Name, "objective": c.Objective, "context": c.Context, "inputs": join(c.Inputs),
		"instructions": c.Instructions, "depends_on": join(c.DependsOn), "artifacts": join(refs), "covers": join(c.Covers),
		"feature_ids": join(c.FeatureIDs), "sub_feature_ids": join(c.SubFeatureIDs), "expected_outputs": join(c.Outputs),
		"acceptance_criteria": join(c.Acceptance), "constraints": join(c.Constraints), "files": join(c.Files),
		"priority": c.Priority, "complexity": c.Complexity,
	} {
		h := sha256.Sum256([]byte(text))
		out[name] = hex.EncodeToString(h[:6])
	}
	return out
}

// CommandDiff lists the fields that differ between two versions of a command,
// sorted so the result is stable.
func CommandDiff(a, b Command) []string {
	x, y := CommandFields(a), CommandFields(b)
	var out []string
	for name, h := range x {
		if y[name] != h {
			out = append(out, name)
		}
	}
	slices.Sort(out)
	return out
}

// CommandWeight is the execution weight of one command. It is deterministic
// and visible in progress, so a command with more acceptance criteria,
// outputs, dependencies and instructions contributes more than a small one.
// Foundry does not measure complexity by running the work.
func CommandWeight(c Command) int {
	w := 1 + len(c.Acceptance) + len(c.Outputs) + len(c.DependsOn)
	switch n := len(c.Instructions); {
	case n > 1200:
		w += 2
	case n > 400:
		w++
	}
	if len(c.Artifacts) > 2 {
		w++
	}
	if w < 1 {
		w = 1
	}
	return w
}

// ResolveCommandFeatureIDs returns the features a command implements:
// explicit FeatureIDs, the parent of its SubFeatureIDs, or, when neither is
// set, the features of the requirements it covers. Results keep feature
// order and are unique.
func ResolveCommandFeatureIDs(c Command, feats []Feature) []string {
	want := map[string]bool{}
	explicit := len(c.FeatureIDs) > 0 || len(c.SubFeatureIDs) > 0
	for _, id := range c.FeatureIDs {
		want[strings.ToLower(strings.TrimSpace(id))] = true
	}
	for _, sf := range feats {
		for _, s := range sf.SubFeatures {
			if containsID(c.SubFeatureIDs, s.ID) {
				want[strings.ToLower(strings.TrimSpace(sf.ID))] = true
			}
		}
	}
	if !explicit {
		covered := map[string]bool{}
		for _, id := range c.Covers {
			covered[strings.ToUpper(strings.TrimSpace(id))] = true
		}
		for _, f := range feats {
			for _, r := range f.RequirementIDs {
				if covered[strings.ToUpper(strings.TrimSpace(r))] {
					want[strings.ToLower(strings.TrimSpace(f.ID))] = true
					break
				}
			}
		}
	}
	var out []string
	for _, f := range feats {
		if want[strings.ToLower(strings.TrimSpace(f.ID))] && !slices.Contains(out, f.ID) {
			out = append(out, f.ID)
		}
	}
	return out
}

// ResolveCommandSubFeatureIDs returns the explicit subfeatures that exist,
// in feature-tree order.
func ResolveCommandSubFeatureIDs(c Command, feats []Feature) []string {
	var out []string
	for _, f := range feats {
		for _, s := range f.SubFeatures {
			if containsID(c.SubFeatureIDs, s.ID) && !slices.Contains(out, s.ID) {
				out = append(out, s.ID)
			}
		}
	}
	return out
}

// ParentFeature returns the feature that owns subfeature id.
func ParentFeature(feats []Feature, subID string) string {
	for _, f := range feats {
		for _, s := range f.SubFeatures {
			if strings.EqualFold(strings.TrimSpace(s.ID), strings.TrimSpace(subID)) {
				return f.ID
			}
		}
	}
	return ""
}

func containsID(ids []string, id string) bool {
	for _, x := range ids {
		if strings.EqualFold(strings.TrimSpace(x), strings.TrimSpace(id)) {
			return true
		}
	}
	return false
}
