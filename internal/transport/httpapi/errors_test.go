package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"

	"github.com/handrawanw/foundry/internal/config"
	"github.com/handrawanw/foundry/internal/core"
	"github.com/handrawanw/foundry/internal/modules/pipeline"
	"github.com/handrawanw/foundry/internal/safefile"
)

type envelope struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func failed(t *testing.T, err error) (int, envelope) {
	t.Helper()
	w := httptest.NewRecorder()
	fail(w, httptest.NewRequest("GET", "/api/v1/x", nil), err)
	var e envelope
	if jerr := json.Unmarshal(w.Body.Bytes(), &e); jerr != nil {
		t.Fatal(jerr)
	}
	return w.Code, e
}

// Internal errors are 500 with a fixed message; the details stay in the log.
func TestInternalErrorsAre500WithoutDetails(t *testing.T) {
	secret := "/home/someone/secret-project/.foundry/project.json"
	for name, err := range map[string]error{
		"io":       fmt.Errorf("gagal menulis: %w", &fs.PathError{Op: "open", Path: secret, Err: syscall.EACCES}),
		"errno":    fmt.Errorf("x %s: %w", secret, syscall.ENOSPC),
		"link":     &os.LinkError{Op: "rename", Old: secret, New: secret + ".tmp", Err: syscall.EXDEV},
		"git":      fmt.Errorf("git apply: %w: fatal: %s token=ghp_abc", &exec.ExitError{}, secret),
		"exec":     &exec.Error{Name: secret, Err: exec.ErrNotFound},
		"state":    fmt.Errorf("state rusak: %w", &json.SyntaxError{}),
		"locked":   core.ErrLocked,
		"config":   config.ErrLocked,
		"filelock": fmt.Errorf("build/test tidak dapat dikunci: %w", safefile.ErrTimeout),
		"cancel":   fmt.Errorf("%s: %w", secret, context.Canceled),
	} {
		code, e := failed(t, err)
		if code != http.StatusInternalServerError || e.Error.Code != "INTERNAL_ERROR" {
			t.Errorf("%s: %d %s, want 500 INTERNAL_ERROR", name, code, e.Error.Code)
		}
		if e.Error.Message != internalMessage || strings.Contains(e.Error.Message, "secret") || strings.Contains(e.Error.Message, "ghp_") {
			t.Errorf("%s: message leaks: %q", name, e.Error.Message)
		}
	}
}

// Mappings that were right stay as they were, message included.
func TestDomainErrorsKeepTheirStatus(t *testing.T) {
	for _, c := range []struct {
		err  error
		code int
	}{
		{badRequest("folder project wajib diisi"), 400},
		{fmt.Errorf("task %s tidak sedang IN_PROGRESS", "T1"), 400},
		{fmt.Errorf("%w: hentikan dulu", pipeline.ErrBusy), 409},
		{pipeline.ErrWrongStage, 409},
		{pipeline.ErrNoAI, 412},
		{core.ErrNotFound, 404},
		{httpError{http.StatusNotFound, "file tidak ditemukan"}, 404},
	} {
		code, e := failed(t, c.err)
		if code != c.code || e.Error.Message != c.err.Error() {
			t.Errorf("%v: %d %q, want %d with its message", c.err, code, e.Error.Message, c.code)
		}
	}
}

func openProject(t *testing.T) (http.Handler, string) {
	t.Helper()
	t.Setenv("FOUNDRY_HOME", t.TempDir())
	dir := t.TempDir()
	s := New(nil, "/bin/foundry")
	if _, err := s.Open(dir, "demo", true); err != nil {
		t.Fatal(err)
	}
	return s.Handler(), dir
}

func get(t *testing.T, h http.Handler, path string) (int, envelope) {
	t.Helper()
	r := httptest.NewRequest("GET", path, nil)
	r.RemoteAddr = "127.0.0.1:50000"
	r.Host = "127.0.0.1:8080"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var e envelope
	json.Unmarshal(w.Body.Bytes(), &e)
	return w.Code, e
}
