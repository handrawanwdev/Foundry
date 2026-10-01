package pipeline

import (
	"strings"
	"testing"

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
