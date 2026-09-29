package httpapi

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGuardAndFlow(t *testing.T) {
	t.Setenv("FOUNDRY_HOME", t.TempDir())
	s := New(nil, "/bin/forge")
	if _, err := s.Open(t.TempDir(), "demo", false); err != nil {
		t.Fatal(err)
	}
	h := s.Handler()
	do := func(method, path, body, origin, host string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Host = host
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := do("GET", "/api/v1/project", "", "", "127.0.0.1:8080"); w.Code != 200 || !strings.Contains(w.Body.String(), `"stage":"TELL"`) {
		t.Fatalf("project: %d %s", w.Code, w.Body)
	}
	if w := do("POST", "/api/v1/project/story", `{"story":"aplikasi klinik untuk pasien"}`, "https://evil.example", "127.0.0.1:8080"); w.Code != 403 {
		t.Fatalf("cross-origin allowed: %d", w.Code)
	}
	if w := do("GET", "/api/v1/project", "", "", "evil.example"); w.Code != 403 {
		t.Fatalf("rebinding host allowed: %d", w.Code)
	}
	// No AI connected → story rejected with 412.
	if w := do("POST", "/api/v1/project/story", `{"story":"aplikasi klinik untuk pasien"}`, "http://localhost:5173", "127.0.0.1:8080"); w.Code != http.StatusPreconditionFailed {
		t.Fatalf("story without AI: %d %s", w.Code, w.Body)
	}
	if w := do("GET", "/simple/tell", "", "", "localhost:8080"); w.Code != 200 {
		t.Fatalf("spa fallback: %d", w.Code)
	}
}

func TestOpenCloseBrowse(t *testing.T) {
	t.Setenv("FOUNDRY_HOME", t.TempDir())
	s := New(nil, "/bin/forge")
	h := s.Handler()
	do := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Host = "127.0.0.1:8080"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := do("GET", "/api/v1/project", ""); w.Code != http.StatusNotFound {
		t.Fatalf("no project: %d", w.Code)
	}
	root := t.TempDir()
	missing := root + "/baru"
	if w := do("POST", "/api/v1/projects/open", `{"dir":"`+missing+`"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("open missing folder: %d", w.Code)
	}
	if w := do("POST", "/api/v1/projects/open", `{"dir":"`+missing+`","create":true,"name":"Baru"}`); w.Code != 200 {
		t.Fatalf("new project: %d %s", w.Code, w.Body)
	}
	if w := do("GET", "/api/v1/fs?path="+root, ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"is_project":true`) {
		t.Fatalf("browse: %d %s", w.Code, w.Body)
	}
	if w := do("POST", "/api/v1/projects/close", `{}`); w.Code != 200 {
		t.Fatalf("close: %d", w.Code)
	}
	if w := do("GET", "/api/v1/projects", ""); !strings.Contains(w.Body.String(), `"current":""`) || !strings.Contains(w.Body.String(), missing) {
		t.Fatalf("after close: %s", w.Body)
	}
}

func TestFileExplorer(t *testing.T) {
	t.Setenv("FOUNDRY_HOME", t.TempDir())
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "src"), 0o755)
	os.MkdirAll(filepath.Join(root, ".git"), 0o755)
	os.WriteFile(filepath.Join(root, "src", "main.go"), []byte("package main\n"), 0o644)
	os.WriteFile(filepath.Join(root, "logo.bin"), []byte{0, 1, 2}, 0o644)
	outside := t.TempDir()
	os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("x"), 0o644)
	os.Symlink(outside, filepath.Join(root, "escape"))

	s := New(nil, "/bin/forge")
	if _, err := s.Open(root, "", false); err != nil {
		t.Fatal(err)
	}
	h := s.Handler()
	get := func(path string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", path, nil)
		r.Host = "127.0.0.1:8080"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	w := get("/api/v1/project/files")
	body := w.Body.String()
	if w.Code != 200 || strings.Contains(body, `".git"`) || !strings.Contains(body, `"src"`) || strings.Index(body, `"src"`) > strings.Index(body, `"logo.bin"`) {
		t.Fatalf("list root: %d %s", w.Code, body)
	}
	if w := get("/api/v1/project/file?path=src/main.go"); w.Code != 200 || !strings.Contains(w.Body.String(), `package main`) {
		t.Fatalf("read: %d %s", w.Code, w.Body)
	}
	if w := get("/api/v1/project/file?path=logo.bin"); !strings.Contains(w.Body.String(), `"binary":true`) {
		t.Fatalf("binary: %s", w.Body)
	}
	for _, p := range []string{"../../etc/passwd", "escape/secret.txt", "/../x"} {
		if w := get("/api/v1/project/file?path=" + p); w.Code == 200 {
			t.Fatalf("%s must be refused: %s", p, w.Body)
		}
	}
}
