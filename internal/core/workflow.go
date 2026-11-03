package core

import (
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
}

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
	Status        string   `json:"status"`
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
