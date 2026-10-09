package pipeline

import (
	"strings"
	"testing"
	"time"

	"github.com/handrawanw/foundry/internal/core"
	"github.com/handrawanw/foundry/internal/modules/agent"
)

func TestClipAndFileProgress(t *testing.T) {
	if got := clipProgress("  hello\nworld  "); got != "hello" {
		t.Fatalf("clip first line = %q", got)
	}
	long := strings.Repeat("あ", 80)
	got := clipProgress(long)
	if n := len([]rune(got)); n != maxProgress || !strings.HasSuffix(got, "…") {
		t.Fatalf("clip length = %d %q", n, got)
	}
	if progressFromFiles(nil) != "Menulis kode" {
		t.Fatal(progressFromFiles(nil))
	}
	if progressFromFiles([]string{"services/api/user.ts"}) != "Mengubah user.ts" {
		t.Fatal(progressFromFiles([]string{"services/api/user.ts"}))
	}
	if progressFromFiles([]string{"a.ts", "b.ts"}) != "Mengubah 2 file" {
		t.Fatal(progressFromFiles([]string{"a.ts", "b.ts"}))
	}
}

type mcpAI struct{}

func (mcpAI) Runner() (*agent.Runner, bool, error) { return nil, true, nil }

func TestUpdateTaskProgress(t *testing.T) {
	t.Setenv("FOUNDRY_HOME", t.TempDir())
	st, err := core.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Init("t"); err != nil {
		t.Fatal(err)
	}
	s := New(st, mcpAI{})
	if _, err := st.Update(func(p *core.Project) error {
		p.Tasks = []core.Task{{ID: "T1", Title: "x", Status: core.TaskInProgress, Owner: "a"}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	tk, err := s.UpdateTaskNote("T1", "", "Menulis skema user", false)
	if err != nil || tk.Progress != "Menulis skema user" || len(tk.Notes) != 0 {
		t.Fatalf("progress = %+v err %v", tk, err)
	}
	tk, err = s.UpdateTaskNote("T1", "Menambah tes login", "", false)
	if err != nil || tk.Progress != "Menambah tes login" || len(tk.Notes) != 0 {
		t.Fatalf("note-as-progress = %+v", tk)
	}
}

func TestDraftRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if hasDraft(dir, "T1") {
		t.Fatal("empty dir")
	}
	markResume(dir, "T1")
	if !hasDraft(dir, "T1") {
		t.Fatal("mark")
	}
	clearDraft(dir, "T1")
	if hasDraft(dir, "T1") {
		t.Fatal("clear")
	}
}

func TestClaimKeepsStartedAtOnResume(t *testing.T) {
	dir := t.TempDir()
	start := time.Now().UTC().Add(-5 * time.Minute).Truncate(time.Millisecond)
	markResume(dir, "T1")
	p := &core.Project{Dir: dir, Tasks: []core.Task{{ID: "T1", Title: "x", Status: core.TaskTodo, StartedAt: start}}}
	claim(p, &p.Tasks[0], "agent-1", "agent")
	if !p.Tasks[0].StartedAt.Equal(start) {
		t.Fatalf("started_at reset: %v", p.Tasks[0].StartedAt)
	}
	if p.Tasks[0].Progress != "Melanjutkan" {
		t.Fatalf("progress = %q", p.Tasks[0].Progress)
	}
}

func TestClaimSetsStartedAt(t *testing.T) {
	p := &core.Project{Tasks: []core.Task{{ID: "T1", Title: "x", Status: core.TaskTodo}}}
	before := time.Now().UTC().Add(-time.Second)
	claim(p, &p.Tasks[0], "agent-1", "agent")
	got := p.Tasks[0].StartedAt
	if got.IsZero() || got.Before(before) {
		t.Fatalf("started_at = %v", got)
	}
	if p.Tasks[0].Status != core.TaskInProgress {
		t.Fatalf("status = %s", p.Tasks[0].Status)
	}
}

func TestConcludeTaskStampsFinishedAt(t *testing.T) {
	t.Setenv("FOUNDRY_HOME", t.TempDir())
	st, err := core.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Init("t"); err != nil {
		t.Fatal(err)
	}
	start := time.Now().UTC().Add(-2 * time.Minute)
	if _, err := st.Update(func(p *core.Project) error {
		p.Tasks = []core.Task{{
			ID: "T1", Title: "x", Status: core.TaskReview, StartedAt: start,
			AcceptanceCriteria: []string{"ok"}, LastBuildID: "B1",
		}}
		p.Builds = []core.BuildRun{{ID: "B1", Passed: true}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	s := New(st, mcpAI{})
	if err := s.concludeTask("T1", []core.Criterion{{Text: "ok", Met: true, Evidence: "file.go"}}, "B1", "agent"); err != nil {
		t.Fatal(err)
	}
	tk := mustLoad(t, st).Task("T1")
	if tk.Status != core.TaskDone || tk.FinishedAt.IsZero() || tk.FinishedAt.Before(start) {
		t.Fatalf("done stamp = %s start=%v finish=%v", tk.Status, tk.StartedAt, tk.FinishedAt)
	}
}

func TestMoveTaskClearsDraft(t *testing.T) {
	t.Setenv("FOUNDRY_HOME", t.TempDir())
	st, err := core.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Init("t"); err != nil {
		t.Fatal(err)
	}
	start := time.Now().UTC().Add(-time.Minute)
	if _, err := st.Update(func(p *core.Project) error {
		p.Tasks = []core.Task{{ID: "T1", Title: "x", Status: core.TaskBlocked, StartedAt: start}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	markResume(st.Dir, "T1")
	s := New(st, mcpAI{})
	p, err := s.MoveTask("T1", core.TaskTodo)
	if err != nil {
		t.Fatal(err)
	}
	tk := p.Task("T1")
	if tk.Status != core.TaskTodo || !tk.StartedAt.IsZero() {
		t.Fatalf("todo = %s start=%v", tk.Status, tk.StartedAt)
	}
	if hasDraft(st.Dir, "T1") {
		t.Fatal("draft tetap")
	}
}

func TestSetTaskClosed(t *testing.T) {
	t.Setenv("FOUNDRY_HOME", t.TempDir())
	st, err := core.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Init("t"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Update(func(p *core.Project) error {
		p.Tasks = []core.Task{
			{ID: "T1", Title: "a", Area: "data", Files: []string{"a.go"}, Status: core.TaskTodo},
			{ID: "T2", Title: "b", Area: "data", Files: []string{"b.go"}, Status: core.TaskTodo},
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	s := New(st, mcpAI{})
	p, err := s.SetTaskClosed("T2", true)
	if err != nil {
		t.Fatal(err)
	}
	if !p.Task("T2").Closed {
		t.Fatal("T2 not closed")
	}
	if n := len(p.ReadyTasks()); n != 1 || p.ReadyTasks()[0].ID != "T1" {
		t.Fatalf("ready = %v", p.ReadyTasks())
	}
	p, err = s.SetTaskClosed("T2", false)
	if err != nil || p.Task("T2").Closed {
		t.Fatalf("reopen = closed=%v err=%v", p.Task("T2").Closed, err)
	}
	if _, err := s.SetTaskClosed("NOPE", true); err == nil {
		t.Fatal("missing task")
	}
}

func mustLoad(t *testing.T, st *core.Store) *core.Project {
	t.Helper()
	p, err := st.Load()
	if err != nil {
		t.Fatal(err)
	}
	return p
}
