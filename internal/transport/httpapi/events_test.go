package httpapi

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/handrawanwdev/foundry/internal/core"
)

func openRun(t *testing.T, s *Server, id string, base int64) {
	t.Helper()
	svc, err := s.service()
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	if _, err := svc.Store.Update(func(p *core.Project) error {
		p.BlueprintProgress = core.NewBlueprintProgress(id, []core.BlueprintSection{
			{ID: "discovery", Name: "Penjelasan Sistem", Status: core.BPPending},
		}, base, at)
		p.BlueprintProgress.BeginExecution("discovery", core.BPTriggerInitial, "Membaca kebutuhan sistem", "", at)
		p.BlueprintProgress.BeginValidation("discovery", "Memvalidasi output Penjelasan Sistem", at)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// readFrames opens the stream and collects progress frames until want have
// arrived or the deadline passes.
func readFrames(t *testing.T, url, lastEventID string, want int, then func()) []progressFrame {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		t.Fatal(err)
	}
	if lastEventID != "" {
		req.Header.Set("Last-Event-ID", lastEventID)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if !strings.Contains(res.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("content-type %s", res.Header.Get("Content-Type"))
	}
	var frames []progressFrame
	sc := bufio.NewScanner(res.Body)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	progress := false
	go func() {
		<-ctx.Done()
		res.Body.Close()
	}()
	first := true
	for len(frames) < want && sc.Scan() {
		line := sc.Text()
		if line == "event: change" && first {
			first = false
			if then != nil {
				then()
			}
		}
		if line == "event: progress" {
			progress = true
			continue
		}
		if progress && strings.HasPrefix(line, "data: ") {
			progress = false
			var f progressFrame
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &f); err != nil {
				t.Fatal(err)
			}
			frames = append(frames, f)
		}
	}
	return frames
}

func TestEventsReplayAfterLastEventID(t *testing.T) {
	t.Setenv("FOUNDRY_HOME", t.TempDir())
	s := New(nil, "/bin/foundry")
	if _, err := s.Open(t.TempDir(), "demo", false); err != nil {
		t.Fatal(err)
	}
	openRun(t, s, "run-1", 0)
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	frames := readFrames(t, ts.URL+"/api/v1/events", "1", 1, nil)
	if len(frames) != 1 {
		t.Fatalf("frames %d", len(frames))
	}
	f := frames[0]
	if f.Progress == nil || f.Progress.RunID != "run-1" || len(f.Progress.Events) != 0 {
		t.Fatalf("progress %+v", f.Progress)
	}
	if len(f.Events) == 0 {
		t.Fatal("nothing replayed")
	}
	for _, e := range f.Events {
		if e.Seq <= 1 {
			t.Fatalf("event at or before the cursor: %+v", e)
		}
	}
	if f.Progress.Summary.Total != 1 {
		t.Fatalf("summary %+v", f.Progress.Summary)
	}
}

func TestEventsDoNotRepeatWhenCaughtUp(t *testing.T) {
	t.Setenv("FOUNDRY_HOME", t.TempDir())
	s := New(nil, "/bin/foundry")
	if _, err := s.Open(t.TempDir(), "demo", false); err != nil {
		t.Fatal(err)
	}
	openRun(t, s, "run-1", 0)
	svc, _ := s.service()
	p, _ := svc.Store.Load()
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()
	seq := p.BlueprintProgress.Seq

	frames := readFrames(t, ts.URL+"/api/v1/events?after="+jsonNum(seq), "", 1, func() {
		svc.Store.Update(func(p *core.Project) error {
			p.BlueprintProgress.CompleteSection("discovery", "", time.Now().UTC())
			return nil
		})
	})
	if len(frames) != 1 {
		t.Fatalf("frames %d", len(frames))
	}
	for _, e := range frames[0].Events {
		if e.Seq <= seq {
			t.Fatalf("replayed %+v", e)
		}
	}
	if frames[0].Progress.Summary.Completed != 1 {
		t.Fatalf("summary %+v", frames[0].Progress.Summary)
	}
}

func jsonNum(n int64) string {
	b, _ := json.Marshal(n)
	return string(b)
}

// A cursor from another project must not hide this project's run.
func TestEventsResetCursorOnProjectSwitch(t *testing.T) {
	t.Setenv("FOUNDRY_HOME", t.TempDir())
	s := New(nil, "/bin/foundry")
	if _, err := s.Open(t.TempDir(), "demo", false); err != nil {
		t.Fatal(err)
	}
	openRun(t, s, "run-a", 500)
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	other := filepath.Join(t.TempDir(), "other")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	frames := readFrames(t, ts.URL+"/api/v1/events", "", 2, func() {
		if _, err := s.Open(other, "other", true); err != nil {
			t.Error(err)
			return
		}
		openRun(t, s, "run-b", 0)
	})
	if len(frames) != 2 || frames[1].Progress.RunID != "run-b" || len(frames[1].Events) == 0 {
		t.Fatalf("frames %+v", frames)
	}
}

func TestBlueprintProgressEndpointAfterCursor(t *testing.T) {
	t.Setenv("FOUNDRY_HOME", t.TempDir())
	s := New(nil, "/bin/foundry")
	if _, err := s.Open(t.TempDir(), "demo", false); err != nil {
		t.Fatal(err)
	}
	openRun(t, s, "run-2", 0)
	h := s.Handler()
	r := httptest.NewRequest("GET", "/api/v1/project/blueprint-progress?after=1", nil)
	r.RemoteAddr, r.Host = "127.0.0.1:50000", "127.0.0.1:8080"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("status %d %s", w.Code, w.Body)
	}
	var env struct {
		Success bool          `json:"success"`
		Data    progressFrame `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if !env.Success || env.Data.Progress == nil || len(env.Data.Events) == 0 {
		t.Fatalf("body %s", w.Body)
	}
	for _, e := range env.Data.Events {
		if e.Seq <= 1 || e.ActivityType == core.EvRunStarted {
			t.Fatalf("leaked into after=1: %+v", e)
		}
	}
}

func TestProjectResponseOmitsTheTimeline(t *testing.T) {
	t.Setenv("FOUNDRY_HOME", t.TempDir())
	s := New(nil, "/bin/foundry")
	if _, err := s.Open(t.TempDir(), "demo", false); err != nil {
		t.Fatal(err)
	}
	openRun(t, s, "run-3", 0)
	r := httptest.NewRequest("GET", "/api/v1/project", nil)
	r.RemoteAddr, r.Host = "127.0.0.1:50000", "127.0.0.1:8080"
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	var env struct {
		Data core.Project `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	bp := env.Data.BlueprintProgress
	if bp == nil || bp.RunID != "run-3" || len(bp.Events) != 0 {
		t.Fatalf("%+v", bp)
	}
}
