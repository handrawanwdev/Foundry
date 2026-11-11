package usage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/handrawanwdev/foundry/internal/core"
)

func n(v int64) *int64   { return &v }
func s(v string) *string { return &v }

func TestAppendRead(t *testing.T) {
	dir := t.TempDir()
	day := time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)
	recs := []Record{
		{RunID: "a", Project: "p1", Operation: "workflow", Model: s("opus"), Time: day, InputTokens: n(10), OutputTokens: n(5), CacheCreationTokens: n(100), CacheReadTokens: n(900), TotalReported: n(1015), Status: StatusOK, Telemetry: Reported},
		{RunID: "b", Project: "p1", Operation: "workflow", Time: day, Status: StatusOverloaded, Telemetry: Unavailable},
		{RunID: "c", Project: "p2", Operation: "blueprint", Model: s("opus"), Time: day.Add(24 * time.Hour), InputTokens: n(1), OutputTokens: n(1), Status: StatusOK, Retry: 1, Telemetry: Reported},
		{RunID: "a", Project: "p1", Operation: "workflow", Model: s("opus"), Time: day, InputTokens: n(20), OutputTokens: n(5), CacheCreationTokens: n(100), CacheReadTokens: n(900), TotalReported: n(1025), Status: StatusOK, Telemetry: Reported},
	}
	for _, r := range recs {
		if err := Append(dir, r); err != nil {
			t.Fatal(err)
		}
	}
	// A broken line (a crash mid-write, a hand edit) is skipped, not fatal.
	f, _ := os.OpenFile(filepath.Join(dir, core.StateDir, File), os.O_APPEND|os.O_WRONLY, 0)
	f.WriteString("{not json\n\n{\"time\":\"x\"}\n")
	f.Close()
	got, bad, err := Read(dir)
	if err != nil || bad != 2 || len(got) != 3 {
		t.Fatalf("got=%d bad=%d err=%v", len(got), bad, err)
	}
	if *got[0].InputTokens != 20 { // the duplicate run id keeps its last record
		t.Fatalf("duplicate: %+v", got[0])
	}
	if got[1].InputTokens != nil || got[1].Model != nil {
		t.Fatalf("null fields read back as values: %+v", got[1])
	}
	raw, _ := os.ReadFile(filepath.Join(dir, core.StateDir, File))
	if !strings.Contains(string(raw), `"input_tokens":null`) {
		t.Fatalf("missing token fields must be stored as null: %s", raw)
	}
	if fi, _ := os.Stat(filepath.Join(dir, core.StateDir, File)); fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v", fi.Mode().Perm())
	}

	ops, _ := Summarize(got, "operation")
	if len(ops) != 2 || ops[1].Key != "workflow" || ops[1].Runs != 2 || ops[1].Reported != 1 || ops[1].Failed != 1 ||
		ops[1].InputTokens != 20 || ops[1].TotalReported != 1025 || ops[1].CacheReadRatio != 900.0/1020.0 {
		t.Fatalf("by operation: %+v", ops)
	}
	models, _ := Summarize(got, "model")
	if len(models) != 2 || models[0].Key != "(tidak dilaporkan)" || models[1].Runs != 2 || models[1].Retries != 1 {
		t.Fatalf("by model: %+v", models)
	}
	days, _ := Summarize(got, "day")
	if len(days) != 2 || days[0].Key != "2026-11-11" || days[1].Key != "2026-11-12" {
		t.Fatalf("by day: %+v", days)
	}
	for _, by := range []string{"run", "project"} {
		if _, err := Summarize(got, by); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Summarize(got, "week"); err == nil {
		t.Fatal("unknown grouping accepted")
	}
}

func TestReadMissing(t *testing.T) {
	recs, bad, err := Read(t.TempDir())
	if err != nil || bad != 0 || len(recs) != 0 {
		t.Fatalf("recs=%v bad=%d err=%v", recs, bad, err)
	}
}

func TestAppendFailure(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, core.StateDir, File), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := Append(dir, Record{RunID: "a"}); err == nil {
		t.Fatal("write into a folder must fail")
	}
	if err := Append("", Record{RunID: "a"}); err == nil {
		t.Fatal("empty folder accepted")
	}
}

func TestRotate(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, core.StateDir, File)
	os.MkdirAll(filepath.Dir(p), 0o700)
	big := `{"run_id":"old","status":"ok"}` + "\n"
	os.WriteFile(p, []byte(strings.Repeat(" ", MaxSize-len(big))+big), 0o600)
	if err := Append(dir, Record{RunID: "new"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p + ".1"); err != nil {
		t.Fatal("not rotated")
	}
	recs, _, _ := Read(dir)
	if len(recs) != 2 || recs[0].RunID != "old" || recs[1].RunID != "new" {
		t.Fatalf("recs = %+v", recs)
	}
}

func TestProjectIDHidesPath(t *testing.T) {
	id := ProjectID("/home/user/secret-client")
	if len(id) != 12 || strings.Contains(id, "secret") || id != ProjectID("/home/user/secret-client") || id == ProjectID("/home/user/other") {
		t.Fatalf("id = %q", id)
	}
	if NewRunID() == NewRunID() {
		t.Fatal("run ids repeat")
	}
}
