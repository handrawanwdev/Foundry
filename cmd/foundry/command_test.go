package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
)

func seedProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	st, err := core.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Init("Klinik"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Update(func(p *core.Project) error {
		p.Workflow = &core.Workflow{Version: 1, Commands: []core.Command{
			{ID: "CMD-001", Name: "Skema", Objective: "o", Instructions: "buat skema", Acceptance: []string{"migrasi jalan"}},
			{ID: "CMD-002", Name: "API", Objective: "o", Instructions: "buat API", DependsOn: []string{"CMD-001"}},
		}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return dir
}

func run(t *testing.T, dir string, want int, args ...string) string {
	t.Helper()
	var out, errOut bytes.Buffer
	if code := runCommand(append(args, "--project", dir), &out, &errOut); code != want {
		t.Fatalf("%v exit %d, want %d: %s %s", args, code, want, out.String(), errOut.String())
	}
	return out.String() + errOut.String()
}

func TestCommandCLI(t *testing.T) {
	dir := seedProject(t)
	var next struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal([]byte(run(t, dir, 0, "next", "--claim", "--session", "a", "--json")), &next); err != nil || next.Command != "CMD-001" {
		t.Fatalf("next = %+v %v", next, err)
	}
	if out := run(t, dir, 1, "start", "CMD-002", "--session", "a"); !strings.Contains(out, "dependency") {
		t.Fatalf("start = %q", out)
	}
	if out := run(t, dir, 0, "progress", "CMD-001", "-m", "tabel", "--percent", "50", "--file", "db.sql", "--session", "a"); !strings.Contains(out, "started") {
		t.Fatalf("progress = %q", out)
	}
	if out := run(t, dir, 0, "done", "CMD-001", "-m", "selesai", "--test", "migrate=pass", "--commit", "abc", "--session", "a"); !strings.Contains(out, "verified") {
		t.Fatalf("done = %q", out)
	}
	if out := run(t, dir, 0, "status"); !strings.Contains(out, "CMD-001") || !strings.Contains(out, "completed") || !strings.Contains(out, "ready") {
		t.Fatalf("status = %q", out)
	}
	if out := run(t, dir, 0, "show", "CMD-001", "--history"); !strings.Contains(out, "tabel") {
		t.Fatalf("history = %q", out)
	}
	if out := run(t, dir, 0, "show", "CMD-002"); !strings.Contains(out, "buat API") {
		t.Fatalf("show = %q", out)
	}
	if out := run(t, dir, 1, "skip", "CMD-002"); !strings.Contains(out, "alasan") {
		t.Fatalf("skip = %q", out)
	}
	if out := run(t, dir, 1, "done", "CMD-009"); !strings.Contains(out, "CMD-009") {
		t.Fatalf("unknown = %q", out)
	}
	run(t, dir, 1, "done", "CMD-002", "--test", "tanpa-hasil")
	run(t, dir, 2, "bogus")
}

func TestCommandCLINeedsAProject(t *testing.T) {
	if out := run(t, t.TempDir(), 1, "status"); !strings.Contains(out, "bukan project") {
		t.Fatalf("out = %q", out)
	}
}
