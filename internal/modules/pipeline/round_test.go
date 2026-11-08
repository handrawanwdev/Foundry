package pipeline

import (
	"errors"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
)

// The next round of a workflow job starts only when no other job took over
// in between and its request is still open.
func TestWorkflowRoundOpen(t *testing.T) {
	open := &core.Workflow{Request: &core.ChangeRequest{Text: "x", Handled: []string{"changed CMD-001"}}}
	for _, tc := range []struct {
		name string
		p    core.Project
		want error
	}{
		{"round done, request open", core.Project{Workflow: open, Job: &core.Job{Status: core.JobDone}}, nil},
		{"continued meanwhile", core.Project{Workflow: open, Job: &core.Job{Status: core.JobRunning}}, ErrBusy},
		{"host owes a round", core.Project{Workflow: open, Job: &core.Job{Status: core.JobWaiting}}, ErrBusy},
		{"cancelled meanwhile", core.Project{Workflow: &core.Workflow{}, Job: &core.Job{Status: core.JobDone}}, ErrNothingToDo},
		{"no workflow", core.Project{}, ErrNothingToDo},
	} {
		if err := workflowRoundOpen(&tc.p); !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", tc.name, err, tc.want)
		}
	}
}
