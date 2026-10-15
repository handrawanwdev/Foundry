package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/handrawanw/foundry/internal/config"
	"github.com/handrawanw/foundry/internal/core"
)

func TestGuardAndFlow(t *testing.T) {
	t.Setenv("FOUNDRY_HOME", t.TempDir())
	s := New(nil, "/bin/foundry")
	if _, err := s.Open(t.TempDir(), "demo", false); err != nil {
		t.Fatal(err)
	}
	h := s.Handler()
	do := func(method, path, body, origin, host string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.RemoteAddr = "127.0.0.1:50000"
		r.Host = host
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := do("GET", "/api/v1/project", "", "", "127.0.0.1:8080"); w.Code != 200 || !strings.Contains(w.Body.String(), `"stage":"TELL"`) || !strings.Contains(w.Body.String(), `"schedule":{"independent":[]}`) {
		t.Fatalf("project: %d %s", w.Code, w.Body)
	}
	if w := do("POST", "/api/v1/project/story", `{"story":"aplikasi klinik untuk pasien"}`, "https://evil.example", "127.0.0.1:8080"); w.Code != 403 {
		t.Fatalf("cross-origin allowed: %d", w.Code)
	}
	if w := do("GET", "/api/v1/project", "", "", "evil.example"); w.Code != 403 {
		t.Fatalf("rebinding host allowed: %d", w.Code)
	}
	// A LAN peer that claims Host: localhost is still refused.
	r := httptest.NewRequest("GET", "/api/v1/project", nil)
	r.Host, r.RemoteAddr = "localhost:8080", "192.168.1.9:40000"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatalf("remote peer allowed: %d", w.Code)
	}
	// No AI connected → story rejected with 412.
	if w := do("POST", "/api/v1/project/story", `{"story":"aplikasi klinik untuk pasien"}`, "http://localhost:5173", "127.0.0.1:8080"); w.Code != http.StatusPreconditionFailed {
		t.Fatalf("story without AI: %d %s", w.Code, w.Body)
	}
	if w := do("GET", "/simple/tell", "", "", "localhost:8080"); w.Code != 200 {
		t.Fatalf("spa fallback: %d", w.Code)
	}
}

func TestSPAAssets(t *testing.T) {
	ui := fstest.MapFS{
		"index.html":    {Data: []byte("<!doctype html><title>Foundry</title>")},
		"assets/app.js": {Data: []byte("console.log(1)")},
	}
	h := New(ui, "/bin/foundry").Handler()
	get := func(handler http.Handler, path string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", path, nil)
		r.RemoteAddr, r.Host = "127.0.0.1:50000", "127.0.0.1:8080"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	if w := get(h, "/assets/app.js"); w.Code != 200 || !strings.Contains(w.Header().Get("Content-Type"), "javascript") || w.Body.String() != "console.log(1)" {
		t.Fatalf("asset: %d %s %s", w.Code, w.Header().Get("Content-Type"), w.Body)
	}
	if w := get(h, "/assets/screen-missing.js"); w.Code != 404 || strings.Contains(w.Body.String(), "<!doctype html>") {
		t.Fatalf("missing asset served as SPA: %d %s", w.Code, w.Body)
	}
	if w := get(h, "/simple/rancang"); w.Code != 200 || !strings.Contains(w.Body.String(), "<!doctype html>") || w.Header().Get("Cache-Control") != "no-cache" {
		t.Fatalf("spa route: %d %q %s", w.Code, w.Header().Get("Cache-Control"), w.Body)
	}
	if w := get(New(nil, "/bin/foundry").Handler(), "/assets/screen-missing.js"); w.Code != 404 || strings.Contains(w.Body.String(), "UI belum") {
		t.Fatalf("missing asset without UI: %d %s", w.Code, w.Body)
	}
}

func TestOpenCloseBrowse(t *testing.T) {
	t.Setenv("FOUNDRY_HOME", t.TempDir())
	s := New(nil, "/bin/foundry")
	h := s.Handler()
	do := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.RemoteAddr = "127.0.0.1:50000"
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

func TestProjectSchedule(t *testing.T) {
	t.Setenv("FOUNDRY_HOME", t.TempDir())
	dir := t.TempDir()
	s := New(nil, "/bin/foundry")
	if _, err := s.Open(dir, "demo", true); err != nil {
		t.Fatal(err)
	}
	st, err := core.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Update(func(p *core.Project) error {
		p.Stage = core.StageBuild
		p.Tasks = []core.Task{
			{ID: "T1", Area: "scaffold", Status: core.TaskDone},
			{ID: "T2", Area: "ui", Files: []string{"packages/ui"}, Status: core.TaskTodo, DependsOn: []string{"T1"}},
			{ID: "T3", Area: "ui", Files: []string{"packages/ui/src/Button.tsx"}, Status: core.TaskTodo, DependsOn: []string{"T1"}},
			{ID: "T4", Area: "data", Files: []string{"services/api/auth"}, Status: core.TaskTodo, DependsOn: []string{"T1"}},
			{ID: "T5", Area: "api", Files: []string{"services/api/auth/login.go"}, Status: core.TaskTodo, DependsOn: []string{"T4"}},
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", "/api/v1/project", nil)
	r.RemoteAddr = "127.0.0.1:50000"
	r.Host = "127.0.0.1:8080"
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("code %d %s", w.Code, w.Body)
	}
	if !strings.Contains(w.Body.String(), `"independent":["T2","T4"]`) {
		t.Fatalf("schedule = %s", w.Body)
	}
	if !strings.Contains(w.Body.String(), `"state":"queued"`) || !strings.Contains(w.Body.String(), `"blocked_by":"T2"`) {
		t.Fatalf("T3 queued = %s", w.Body)
	}
	if !strings.Contains(w.Body.String(), `"state":"waiting_deps"`) {
		t.Fatalf("T5 waiting = %s", w.Body)
	}
}

func TestPatchTaskClosed(t *testing.T) {
	t.Setenv("FOUNDRY_HOME", t.TempDir())
	dir := t.TempDir()
	s := New(nil, "/bin/foundry")
	if _, err := s.Open(dir, "demo", true); err != nil {
		t.Fatal(err)
	}
	st, err := core.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Update(func(p *core.Project) error {
		p.Stage = core.StageBuild
		p.Tasks = []core.Task{
			{ID: "T1", Area: "data", Files: []string{"a.go"}, Status: core.TaskTodo},
			{ID: "T2", Area: "data", Files: []string{"b.go"}, Status: core.TaskTodo},
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	h := s.Handler()
	do := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.RemoteAddr = "127.0.0.1:50000"
		r.Host = "127.0.0.1:8080"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	w := do("PATCH", "/api/v1/project/tasks/T2", `{"closed":true}`)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"closed":true`) {
		t.Fatalf("close: %d %s", w.Code, w.Body)
	}
	w = do("GET", "/api/v1/project", "")
	if w.Code != 200 {
		t.Fatalf("get: %d %s", w.Code, w.Body)
	}
	if !strings.Contains(w.Body.String(), `"id":"T2"`) || !strings.Contains(w.Body.String(), `"closed":true`) {
		t.Fatalf("persisted = %s", w.Body)
	}
	if strings.Contains(w.Body.String(), `"independent":["T1","T2"]`) {
		t.Fatalf("closed T2 still independent: %s", w.Body)
	}
	w = do("PATCH", "/api/v1/project/tasks/T2", `{"closed":false}`)
	if w.Code != 200 || strings.Contains(w.Body.String(), `"closed":true`) {
		t.Fatalf("open: %d %s", w.Code, w.Body)
	}
	w = do("PATCH", "/api/v1/project/tasks/T2", `{}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("empty patch: %d %s", w.Code, w.Body)
	}
}

func TestDownloadPRD(t *testing.T) {
	t.Setenv("FOUNDRY_HOME", t.TempDir())
	dir := t.TempDir()
	s := New(nil, "/bin/foundry")
	if _, err := s.Open(dir, "Klinik Sehat", true); err != nil {
		t.Fatal(err)
	}
	h := s.Handler()
	do := func(path string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", path, nil)
		r.RemoteAddr = "127.0.0.1:50000"
		r.Host = "127.0.0.1:8080"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := do("/api/v1/project/prd.md"); w.Code != http.StatusNotFound {
		t.Fatalf("missing prd: %d %s", w.Code, w.Body)
	}
	st, err := core.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Update(func(p *core.Project) error {
		p.PRD = &core.PRD{Markdown: "# 1. Product Overview\nKlinik\n"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	w := do("/api/v1/project/prd.md")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Product Overview") {
		t.Fatalf("download: %d %s", w.Code, w.Body)
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "text/markdown") {
		t.Fatalf("type = %s", ct)
	}
	if cd := w.Header().Get("Content-Disposition"); !strings.Contains(cd, "PRD-Klinik-Sehat.md") {
		t.Fatalf("disposition = %s", cd)
	}
}

func TestPrdDownloadName(t *testing.T) {
	if got := prdDownloadName(""); got != "PRD.md" {
		t.Fatalf("empty = %s", got)
	}
	if got := prdDownloadName("Klinik Sehat"); got != "PRD-Klinik-Sehat.md" {
		t.Fatalf("name = %s", got)
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

	s := New(nil, "/bin/foundry")
	if _, err := s.Open(root, "", false); err != nil {
		t.Fatal(err)
	}
	h := s.Handler()
	get := func(path string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", path, nil)
		r.RemoteAddr = "127.0.0.1:50000"
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

func TestDesignVersionRoutes(t *testing.T) {
	t.Setenv("FOUNDRY_HOME", t.TempDir())
	s := New(nil, "/bin/foundry")
	if _, err := s.Open(t.TempDir(), "demo", false); err != nil {
		t.Fatal(err)
	}
	h := s.Handler()
	do := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.RemoteAddr = "127.0.0.1:50000"
		r.Host = "127.0.0.1:8080"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := do("GET", "/api/v1/project/design/versions", ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"data":[]`) {
		t.Fatalf("versions: %d %s", w.Code, w.Body)
	}
	if w := do("GET", "/api/v1/project/design/prototype", ""); w.Code != 200 {
		t.Fatalf("prototype: %d %s", w.Code, w.Body)
	}
	if w := do("GET", "/api/v1/project/design/versions/compare?from=x", ""); w.Code != http.StatusBadRequest {
		t.Fatalf("compare bad id: %d", w.Code)
	}
	if w := do("POST", "/api/v1/project/design/versions/0/restore", ""); w.Code != http.StatusBadRequest {
		t.Fatalf("restore bad id: %d", w.Code)
	}
	// No design yet: canvas edits and restores are refused.
	if w := do("POST", "/api/v1/project/design/edit", `{"page":"PAGE-001","ops":[{"op":"remove","id":"x"}]}`); w.Code != http.StatusConflict {
		t.Fatalf("edit without design: %d %s", w.Code, w.Body)
	}
	if w := do("POST", "/api/v1/project/design/versions/1/restore", ""); w.Code != http.StatusConflict {
		t.Fatalf("restore without design: %d %s", w.Code, w.Body)
	}
}

func TestDesignTemplateRoute(t *testing.T) {
	t.Setenv("FOUNDRY_HOME", t.TempDir())
	h := New(nil, "/bin/foundry").Handler()
	r := httptest.NewRequest("GET", "/api/v1/design-templates?theme=sakura-light&domain=ticketing", nil)
	r.RemoteAddr = "127.0.0.1:50000"
	r.Host = "127.0.0.1:8080"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var env struct {
		Data struct {
			Templates []struct{ ID string }
			Previews  map[string]struct{ Name string }
			Theme     struct{ ID string }
			Domain    string
		}
	}
	err := json.Unmarshal(w.Body.Bytes(), &env)
	g := env.Data
	if w.Code != 200 || err != nil {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if len(g.Templates) != 8 || g.Theme.ID != "sakura-light" || g.Domain != "ticketing" || g.Previews["workspace-agent"].Name != "Helpdesk tiket" {
		t.Fatalf("gallery = %+v", g)
	}
}

func TestAIParallelSettings(t *testing.T) {
	t.Setenv("FOUNDRY_HOME", t.TempDir())
	h := New(nil, "/bin/foundry").Handler()
	do := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.RemoteAddr = "127.0.0.1:50000"
		r.Host = "127.0.0.1:8080"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	w := do("GET", "/api/v1/ai", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"max":2`) || !strings.Contains(w.Body.String(), `"default":2`) {
		t.Fatalf("get ai: %d %s", w.Code, w.Body)
	}
	if w := do("PATCH", "/api/v1/ai", `{"agents":5}`); w.Code != http.StatusPreconditionFailed {
		t.Fatalf("patch without connection: %d %s", w.Code, w.Body)
	}
	if w := do("POST", "/api/v1/ai/connect-mcp", `{"agents":1}`); w.Code != 200 {
		t.Fatalf("mcp: %d %s", w.Code, w.Body)
	}
	w = do("GET", "/api/v1/ai", "")
	if !strings.Contains(w.Body.String(), `"agents":1`) {
		t.Fatalf("stored agents: %s", w.Body)
	}
	if w := do("PATCH", "/api/v1/ai", `{"agents":2}`); w.Code != 200 || !strings.Contains(w.Body.String(), `"agents":2`) {
		t.Fatalf("patch: %s", w.Body)
	}
	if w := do("PATCH", "/api/v1/ai", `{"agents":99}`); w.Code != 200 || !strings.Contains(w.Body.String(), `"agents":2`) {
		t.Fatalf("cap: %s", w.Body)
	}
	if w := do("POST", "/api/v1/ai/connect-mcp", `{}`); w.Code != 200 || !strings.Contains(w.Body.String(), `"agents":2`) {
		t.Fatalf("reconnect keeps agents: %s", w.Body)
	}
}

func TestAIDisconnect(t *testing.T) {
	t.Setenv("FOUNDRY_HOME", t.TempDir())
	h := New(nil, "/bin/foundry").Handler()
	do := func(path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", path, strings.NewReader(body))
		r.RemoteAddr = "127.0.0.1:50000"
		r.Host = "127.0.0.1:8080"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	beat := func() {
		if _, err := config.Update(func(c *config.Config) {
			c.MCP = &config.MCPHeartbeat{Client: "opencode", At: time.Now().UTC()}
		}); err != nil {
			t.Fatal(err)
		}
	}
	state := func() *config.Config {
		c, err := config.Load()
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	// Disconnecting the MCP host forgets its heartbeat too.
	do("/api/v1/ai/connect-mcp", `{}`)
	beat()
	if w := do("/api/v1/ai/disconnect", `{}`); w.Code != 200 {
		t.Fatalf("disconnect: %d %s", w.Code, w.Body)
	}
	if c := state(); c.Connection != nil || c.MCP != nil {
		t.Fatalf("after disconnect: %+v %+v", c.Connection, c.MCP)
	}
	// An agent connection keeps a stray MCP heartbeat unless "all" is set.
	if _, err := config.Update(func(c *config.Config) {
		c.Connection = &config.Connection{Mode: config.ModeAgent, AgentID: "custom"}
	}); err != nil {
		t.Fatal(err)
	}
	beat()
	do("/api/v1/ai/disconnect", `{}`)
	if c := state(); c.Connection != nil || c.MCP == nil {
		t.Fatalf("single: %+v %+v", c.Connection, c.MCP)
	}
	do("/api/v1/ai/disconnect", `{"all": true}`)
	if c := state(); c.Connection != nil || c.MCP != nil {
		t.Fatalf("all: %+v %+v", c.Connection, c.MCP)
	}
}

func TestResponseEnvelope(t *testing.T) {
	t.Setenv("FOUNDRY_HOME", t.TempDir())
	s := New(nil, "/bin/foundry")
	h := s.Handler()
	get := func(path, host string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", path, nil)
		r.RemoteAddr, r.Host = "127.0.0.1:50000", host
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	type env struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
		Error   *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
			Details []any  `json:"details"`
		} `json:"error"`
	}
	w := get("/api/v1/ai", "127.0.0.1:8080")
	var ok env
	if err := json.Unmarshal(w.Body.Bytes(), &ok); err != nil || w.Code != 200 || !ok.Success || ok.Error != nil || len(ok.Data) == 0 || !strings.Contains(w.Body.String(), `"meta":null`) {
		t.Fatalf("success envelope: %d %s", w.Code, w.Body)
	}
	for _, c := range []struct {
		path, host, code string
		status           int
	}{
		{"/api/v1/project", "127.0.0.1:8080", "NOT_FOUND", 404},
		{"/api/v1/nope", "127.0.0.1:8080", "NOT_FOUND", 404},
		{"/api/v1/ai", "evil.example", "FORBIDDEN", 403},
	} {
		w := get(c.path, c.host)
		var e env
		if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil || w.Code != c.status || e.Success || e.Error == nil || e.Error.Code != c.code || e.Error.Message == "" || e.Error.Details == nil {
			t.Fatalf("%s: want %d %s, got %d %s", c.path, c.status, c.code, w.Code, w.Body)
		}
	}
}
