package httpapi

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/handrawanwdev/foundry/internal/core"
)

func TestEventsReplayProgressByLastEventID(t *testing.T) {
	t.Setenv("FOUNDRY_HOME", t.TempDir())
	s := New(nil, "/bin/foundry")
	if _, err := s.Open(t.TempDir(), "demo", false); err != nil {
		t.Fatal(err)
	}
	svc, err := s.service()
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	if _, err := svc.Store.Update(func(p *core.Project) error {
		p.BlueprintProgress = core.NewBlueprintProgress("blueprint-1", []core.BlueprintSection{
			{ID: "discovery", Name: "Penjelasan Sistem", Status: core.BPPending},
		}, at)
		p.BlueprintProgress.StartSection("discovery", "Membaca kebutuhan sistem", at)
		p.BlueprintProgress.SetActivity("discovery", "Memvalidasi output Penjelasan Sistem", at)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", ts.URL+"/api/v1/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Last-Event-ID", "1")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status %d", res.StatusCode)
	}
	if !strings.Contains(res.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("content-type %s", res.Header.Get("Content-Type"))
	}

	var progress []core.BlueprintEvent
	sc := bufio.NewScanner(res.Body)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && len(progress) < 2 {
		if !sc.Scan() {
			if sc.Err() != nil && sc.Err() != io.EOF && ctx.Err() == nil {
				t.Fatal(sc.Err())
			}
			break
		}
		line := sc.Text()
		if !strings.HasPrefix(line, "data: {") {
			continue
		}
		var ev core.BlueprintEvent
		if json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &ev) != nil || ev.Event == "" {
			continue
		}
		progress = append(progress, ev)
	}
	cancel()
	if len(progress) == 0 {
		t.Fatal("no progress events replayed")
	}
	for _, ev := range progress {
		if ev.Seq <= 1 {
			t.Fatalf("duplicate/old seq %d event %s", ev.Seq, ev.Event)
		}
		if ev.TaskID != "blueprint-1" {
			t.Fatalf("taskId %s", ev.TaskID)
		}
	}
}

func TestBlueprintProgressSnapshotAfter(t *testing.T) {
	t.Setenv("FOUNDRY_HOME", t.TempDir())
	s := New(nil, "/bin/foundry")
	if _, err := s.Open(t.TempDir(), "demo", false); err != nil {
		t.Fatal(err)
	}
	svc, err := s.service()
	if err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC()
	if _, err := svc.Store.Update(func(p *core.Project) error {
		p.BlueprintProgress = core.NewBlueprintProgress("blueprint-2", []core.BlueprintSection{
			{ID: "discovery", Name: "Penjelasan Sistem", Status: core.BPPending},
		}, at)
		p.BlueprintProgress.StartSection("discovery", "Membaca kebutuhan sistem", at)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	h := s.Handler()
	r := httptest.NewRequest("GET", "/api/v1/project/blueprint-progress?after=1", nil)
	r.RemoteAddr, r.Host = "127.0.0.1:50000", "127.0.0.1:8080"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("status %d %s", w.Code, w.Body)
	}
	var env struct {
		Success bool `json:"success"`
		Data    struct {
			Events []core.BlueprintEvent `json:"events"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if !env.Success || len(env.Data.Events) == 0 {
		t.Fatalf("body %s", w.Body)
	}
	for _, e := range env.Data.Events {
		if e.Seq <= 1 {
			t.Fatalf("seq %d leaked into after=1: %+v", e.Seq, e)
		}
		if e.Event == core.EvTaskStarted {
			t.Fatal("task.started leaked into after=1")
		}
	}
}
