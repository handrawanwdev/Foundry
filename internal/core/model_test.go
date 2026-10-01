package core

import "testing"

func TestTaskLocks(t *testing.T) {
	p := &Project{Tasks: []Task{
		{ID: "A", Area: "ui", PageIDs: []string{"PAGE-001"}, Status: TaskInProgress},
		{ID: "B", Area: "ui", PageIDs: []string{"PAGE-001"}, Status: TaskTodo}, // same page → waits
		{ID: "C", Area: "ui", PageIDs: []string{"PAGE-002"}, Status: TaskTodo}, // other page → runs
		{ID: "D", Area: "api", Files: []string{`services\api\`}, Status: TaskReview},
		{ID: "E", Area: "api", Files: []string{"./Services/API/handler.go"}, Status: TaskTodo}, // inside D's dir
		{ID: "F", Area: "api", Files: []string{"services/apix"}, Status: TaskTodo},             // prefix, not inside
		{ID: "G", Area: "scaffold", Status: TaskTodo},                                          // holds everything
		{ID: "H", Area: "test", FeatureID: "FEAT-001", Status: TaskDone},
	}}
	want := map[string]string{"B": "A", "C": "", "E": "D", "F": "", "G": "A"}
	for id, blocker := range want {
		o, _ := p.Blocker(p.Task(id))
		got := ""
		if o != nil {
			got = o.ID
		}
		if got != blocker {
			t.Errorf("%s blocked by %q, want %q", id, got, blocker)
		}
	}
	var ids []string
	for _, c := range p.ClaimableTasks() {
		ids = append(ids, c.ID)
	}
	if len(ids) != 2 || ids[0] != "C" || ids[1] != "F" {
		t.Fatalf("claimable = %v", ids)
	}
	// A running scaffold task blocks every other task.
	p.Tasks[0].Status, p.Tasks[3].Status, p.Tasks[6].Status = TaskDone, TaskDone, TaskInProgress
	if c := p.ClaimableTasks(); len(c) != 0 {
		t.Fatalf("claimable during scaffold = %d", len(c))
	}
	if l := (&Task{}).Locks(); len(l) != 1 || l[0] != LockAll {
		t.Fatalf("unscoped locks = %v", l)
	}
}
