//go:build unix

package httpapi

import (
	"net/http"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// A FIFO in the project must be refused, not block the handler waiting for
// a writer that never comes.
func TestFIFOIsRefused(t *testing.T) {
	h, dir := openProject(t)
	if err := syscall.Mkfifo(filepath.Join(dir, "pipe"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"/api/v1/project/file?path=pipe",
		"/api/v1/project/files?path=pipe",
		"/api/v1/fs?path=" + filepath.Join(dir, "pipe"),
	} {
		done := make(chan int, 1)
		go func() {
			code, _ := get(t, h, path)
			done <- code
		}()
		select {
		case code := <-done:
			if code != http.StatusBadRequest {
				t.Errorf("%s: %d, want 400", path, code)
			}
		case <-time.After(10 * time.Second): // bounds a hang; a pass returns at once
			t.Fatalf("%s: handler blocked on the FIFO", path)
		}
	}
}
