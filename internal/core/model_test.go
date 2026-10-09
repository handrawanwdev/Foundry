package core

import (
	"strings"
	"testing"
)

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

func TestIndependentTasks(t *testing.T) {
	p := &Project{Tasks: []Task{
		{ID: "T1", Area: "data", Files: []string{"packages/types"}, Status: TaskDone},
		{ID: "T2", Area: "ui", Files: []string{"packages/ui/src/Button.tsx"}, Status: TaskTodo, DependsOn: []string{"T1"}},
		{ID: "T3", Area: "ui", Files: []string{"packages/ui"}, Status: TaskTodo, DependsOn: []string{"T1"}}, // overlaps T2
		{ID: "T4", Area: "data", Files: []string{"services/api/modules/autentikasi"}, Status: TaskTodo, DependsOn: []string{"T1"}},
		{ID: "T5", Area: "data", Files: []string{"services/api/modules/pembuatan-tiket"}, Status: TaskTodo, DependsOn: []string{"T1"}},
		{ID: "T6", Area: "api", Files: []string{"services/api/modules/autentikasi/auth.go"}, Status: TaskTodo, DependsOn: []string{"T4"}},
		{ID: "T7", Area: "data", Files: []string{"packages/types/src/index.ts"}, Status: TaskTodo}, // no deps, overlaps nothing running
	}}
	got := idsOf(p.IndependentTasks())
	if len(got) != 4 || got[0] != "T2" || got[1] != "T4" || got[2] != "T5" || got[3] != "T7" {
		t.Fatalf("independent = %v, want T2 T4 T5 T7 (T3 overlaps T2, T6 waits on T4)", got)
	}
	s := p.Schedule()
	by := map[string]QueueItem{}
	for _, it := range s.Items {
		by[it.ID] = it
	}
	if by["T2"].State != QueueIndependent || by["T3"].State != QueueQueued || by["T3"].BlockedBy != "T2" {
		t.Fatalf("T2/T3 = %+v %+v", by["T2"], by["T3"])
	}
	if by["T6"].State != QueueWaitingDeps || strings.Join(by["T6"].Waiting, ",") != "T4" {
		t.Fatalf("T6 = %+v", by["T6"])
	}
	p.Task("T2").Status = TaskInProgress
	p.Task("T2").Owner = "agent-1"
	got = idsOf(p.IndependentTasks())
	if len(got) != 3 || got[0] != "T4" || got[1] != "T5" || got[2] != "T7" {
		t.Fatalf("independent while T2 runs = %v", got)
	}
	if o, lock := p.Blocker(p.Task("T3")); o == nil || o.ID != "T2" || !strings.HasPrefix(lock, "file:") {
		t.Fatalf("T3 blocker = %v %s", o, lock)
	}
}

func TestClosedTasksSkipped(t *testing.T) {
	p := &Project{Tasks: []Task{
		{ID: "T1", Area: "data", Files: []string{"a.go"}, Status: TaskTodo},
		{ID: "T2", Area: "data", Files: []string{"b.go"}, Status: TaskTodo, Closed: true},
		{ID: "T3", Area: "data", Files: []string{"c.go"}, Status: TaskTodo, DependsOn: []string{"T2"}},
	}}
	got := idsOf(p.ReadyTasks())
	if len(got) != 1 || got[0] != "T1" {
		t.Fatalf("ready = %v, want T1 (T2 closed, T3 waits on T2)", got)
	}
	if got = idsOf(p.IndependentTasks()); len(got) != 1 || got[0] != "T1" {
		t.Fatalf("independent = %v", got)
	}
	s := p.Schedule()
	if len(s.Independent) != 1 || s.Independent[0] != "T1" {
		t.Fatalf("schedule independent = %v", s.Independent)
	}
	for _, it := range s.Items {
		if it.ID == "T2" {
			t.Fatalf("closed T2 in schedule: %+v", it)
		}
	}
	p.Task("T2").Closed = false
	got = idsOf(p.ReadyTasks())
	if len(got) != 2 || got[0] != "T1" || got[1] != "T2" {
		t.Fatalf("ready after open = %v, want T1 T2", got)
	}
}

func idsOf(ts []*Task) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.ID
	}
	return out
}
