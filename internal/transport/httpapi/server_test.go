package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	if w := do("GET", "/api/v1/project", "", "", "127.0.0.1:8080"); w.Code != 200 || !strings.Contains(w.Body.String(), `"stage":"TELL"`) || strings.Contains(w.Body.String(), `"tasks"`) {
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

func TestDownloadSpec(t *testing.T) {
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
	if w := do("/api/v1/project/spec.md"); w.Code != http.StatusNotFound {
		t.Fatalf("spec without design: %d %s", w.Code, w.Body)
	}
	st, err := core.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Update(func(p *core.Project) error {
		p.Design = &core.Design{Pages: []core.Page{{ID: "PAGE-001", Name: "Antrean", Route: "/antrean", Platform: "web"}}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	w := do("/api/v1/project/spec.md")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "/antrean") || !strings.Contains(w.Header().Get("Content-Type"), "text/markdown") ||
		!strings.Contains(w.Header().Get("Content-Disposition"), "spec-Klinik-Sehat.md") {
		t.Fatalf("spec.md: %d %v %s", w.Code, w.Header(), w.Body)
	}
	w = do("/api/v1/project/spec.json")
	var out struct {
		Pages []struct {
			PageID string `json:"page_id"`
		} `json:"pages"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &out) != nil || len(out.Pages) != 1 || out.Pages[0].PageID != "PAGE-001" {
		t.Fatalf("spec.json: %d %s", w.Code, w.Body)
	}
}

func TestSpecDownloadName(t *testing.T) {
	if got := specDownloadName("", ".md"); got != "spec.md" {
		t.Fatalf("empty = %s", got)
	}
	if got := specDownloadName("Klinik Sehat!", ".json"); got != "spec-Klinik-Sehat.json" {
		t.Fatalf("name = %s", got)
	}
}
