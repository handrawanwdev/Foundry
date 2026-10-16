package pipeline

import (
	"strings"
	"testing"

	"github.com/handrawanw/foundry/internal/core"
	"github.com/handrawanw/foundry/internal/modules/checks"
)

func TestRepeatedFailure(t *testing.T) {
	prev := []string{"Verifikasi gagal: test (go test) 12 passed, 3 failed in 1.2s"}
	if !RepeatedFailure(prev, "Verifikasi gagal: test (go test)  14 passed, 3 failed in 0.9s") {
		t.Fatal("same failure up to numbers must count as repeat")
	}
	if RepeatedFailure(prev, "Verifikasi gagal: build (go build) undefined: Foo") {
		t.Fatal("different failure is a new strategy chance")
	}
	if RepeatedFailure(nil, "x") || RepeatedFailure(prev, "  ") {
		t.Fatal("no history or empty note is never a repeat")
	}
}

func TestTaskMetricsRecord(t *testing.T) {
	var m core.TaskMetrics
	m.Record(4000, 400, 1500, false, false)
	m.Record(800, 0, 200, true, false)
	if m.Runs != 2 || m.Failures != 1 || m.PromptChars != 4800 || m.EstTokens != 1300 || m.Millis != 1700 {
		t.Fatalf("metrics = %+v", m)
	}
	if m.InputTokens != 0 || m.UsageRuns != 0 {
		t.Fatal("provider tokens must stay 0 (unavailable), never estimated")
	}
	// A run with provider usage adds actual numbers and NO estimate: no double counting.
	m.Record(4000, 400, 100, false, true)
	m.RecordUsage(2, 18718, 8608, 4, 0.5)
	m.RecordUsage(1, 0, 100, 9, 0.25)
	if m.EstTokens != 1300 || m.UsageRuns != 2 || m.InputTokens != 3 || m.CacheWriteTokens != 18718 || m.CachedTokens != 8708 || m.OutputTokens != 13 || m.CostUSD != 0.75 {
		t.Fatalf("usage = %+v", m)
	}
	var total core.TaskMetrics
	total.Add(&m)
	total.Add(&m)
	total.Add(nil)
	if total.Runs != 6 || total.CostUSD != 1.5 || total.EstTokens != 2600 {
		t.Fatalf("add = %+v", total)
	}
}

func TestRepeatedFailureSparesTransientAndAgentErrors(t *testing.T) {
	cases := map[string]string{
		"timeout":   "Verifikasi gagal: test (npm test)\nError: Test timed out after 5000ms",
		"port":      "Verifikasi gagal: test (go test)\nlisten tcp :8080: bind: address already in use",
		"5xx":       "Verifikasi gagal: test (pytest)\nHTTPError 503 Service Unavailable",
		"oom":       "Verifikasi gagal: build (npm run build)\nFATAL ERROR: JavaScript heap out of memory",
		"agent":     "Agent gagal menjalankan task: agent gagal: tidak ada aktivitas selama 10m0s",
		"limit":     "Agent gagal menjalankan task: agent gagal: limit/kuota AI habis",
		"no change": "Tidak ada file yang berubah; implementasi belum dibuat",
		"go proxy":  "Verifikasi gagal: install-go gagal (go mod tidy)\ngo: github.com/gin-gonic/gin@v1.10.0: Get \"https://proxy.golang.org/...\": dial tcp: lookup proxy.golang.org: no such host",
		"npm":       "Verifikasi gagal: install gagal (npm ci --no-audit --no-fund)\nnpm error code EAI_AGAIN",
	}
	for name, note := range cases {
		if RepeatedFailure([]string{note}, note) {
			t.Fatalf("%s: identical but transient/agent-level failure must keep its retries", name)
		}
	}
	// A module version the declared go version cannot use is code (go.mod),
	// not environment: retrying the same go.mod cannot pass.
	pin := "Verifikasi gagal: install-go gagal (go mod tidy -go=1.22)\ngo: github.com/Microsoft/go-winio@v0.6.3 requires go@1.26.0, but 1.22 is requested"
	if !RepeatedFailure([]string{pin}, pin) {
		t.Fatal("identical go version conflict must stop")
	}
	// The original TASK-001 failure is code, not environment: it still stops.
	vet := "Verifikasi gagal: vet gagal (go vet ./...)\napps/api/cmd/main.go:18:2: no required module provides package github.com/gin-gonic/gin; to add it:"
	if !RepeatedFailure([]string{vet}, vet) {
		t.Fatal("identical missing-module vet failure must stop")
	}
	det := "Verifikasi gagal: test (go test)\n--- FAIL: TestX\n    x_test.go:10: got 1 want 2"
	if !RepeatedFailure([]string{det}, det) {
		t.Fatal("identical deterministic test failure must stop")
	}
	crit := "Acceptance criteria belum terpenuhi: login works"
	if !RepeatedFailure([]string{crit}, crit) {
		t.Fatal("identical unmet criteria must stop")
	}
}

func TestOwnChangesSeparatesConcurrentTasks(t *testing.T) {
	store, err := core.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	store.Init("demo")
	store.Update(func(p *core.Project) error {
		p.Tasks = []core.Task{
			{ID: "TASK-001", Status: core.TaskInProgress, Files: []string{"app/users"}},
			{ID: "TASK-002", Status: core.TaskInProgress, Files: []string{"app/billing"}},
			{ID: "TASK-003", Status: core.TaskTodo, Files: []string{"app/reports"}},
		}
		return nil
	})
	s := New(store, nil)
	all := []checks.Change{
		{Path: "app/users/repo.go", Kind: checks.Modified},
		{Path: "app/billing/api.go", Kind: checks.Modified},
		{Path: "app/billing/new.go", From: "app/billing/old.go", Kind: checks.Renamed},
		{Path: "app/reports/x.go", Kind: checks.Added}, // pending, not running: nobody else is editing it
		{Path: "shared/util.go", Kind: checks.Added},
	}
	got, note := s.ownChanges("TASK-001", all)
	paths := changePaths(got)
	if strings.Join(paths, ",") != "app/users/repo.go,app/reports/x.go,shared/util.go" {
		t.Fatalf("kept = %v", paths)
	}
	if !strings.Contains(note, "2 file milik task lain") {
		t.Fatalf("reviewer must be told files were left out: %q", note)
	}
	// A file the task owns itself is never dropped, even if another task lists it too.
	store.Update(func(p *core.Project) error {
		p.Tasks[1].Files = append(p.Tasks[1].Files, "app/users/repo.go")
		return nil
	})
	if got, _ := s.ownChanges("TASK-001", all[:1]); len(got) != 1 {
		t.Fatal("own file dropped")
	}
}
