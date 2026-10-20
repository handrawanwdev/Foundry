package core

import "time"

// Workflow is the set of instructions an external coding agent follows to
// implement the blueprint. Foundry writes and validates it; it never runs
// it. Commands depend on each other as a directed acyclic graph.
type Workflow struct {
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
	ID           string   `json:"id"` // CMD-001
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
	Covers      []string `json:"covers,omitempty"`
	Outputs     []string `json:"expected_outputs,omitempty"`
	Acceptance  []string `json:"acceptance_criteria,omitempty"`
	Constraints []string `json:"constraints,omitempty"`
	Status      string   `json:"status"`
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
