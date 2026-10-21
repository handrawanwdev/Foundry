package httpapi

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
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

// The delete routes exist and map the pipeline's refusal to 409; the
// document id comes from the path.
func TestDeleteDocumentRoutes(t *testing.T) {
	t.Setenv("FOUNDRY_HOME", t.TempDir())
	s := New(nil, "/bin/foundry")
	if _, err := s.Open(t.TempDir(), "demo", false); err != nil {
		t.Fatal(err)
	}
	h := s.Handler()
	do := func(method, path string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, nil)
		r.RemoteAddr, r.Host = "127.0.0.1:50000", "127.0.0.1:8080"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	// No blueprint yet: both are refused as the wrong stage, not as an unknown route.
	for _, c := range []struct{ method, path string }{
		{"GET", "/api/v1/project/blueprint/documents/threat-model/impact"},
		{"DELETE", "/api/v1/project/blueprint/documents/threat-model"},
	} {
		if w := do(c.method, c.path); w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "blueprint belum dibuat") {
			t.Fatalf("%s %s: %d %s", c.method, c.path, w.Code, w.Body)
		}
	}
	// Only DELETE deletes; any other method falls through to the API's 404.
	if w := do("POST", "/api/v1/project/blueprint/documents/threat-model"); w.Code != http.StatusNotFound || !strings.Contains(w.Body.String(), "not found") {
		t.Fatalf("POST delete route: %d %s", w.Code, w.Body)
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

// UI design routes: read, AI generate/revise and hand edits that are
// validated whole. The visual design workspace routes are gone.
func TestDesignRoutes(t *testing.T) {
	t.Setenv("FOUNDRY_HOME", t.TempDir())
	dir := t.TempDir()
	s := New(nil, "/bin/foundry")
	if _, err := s.Open(dir, "demo", false); err != nil {
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
	if w := do("GET", "/api/v1/project/design", ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"design":null`) ||
		!strings.Contains(w.Body.String(), `"issues":[]`) || !strings.Contains(w.Body.String(), `"patterns":["sidebar"`) {
		t.Fatalf("design: %d %s", w.Code, w.Body)
	}
	for _, route := range []string{"GET /api/v1/themes", "GET /api/v1/design-templates", "GET /api/v1/project/design/versions",
		"GET /api/v1/project/design/prototype", "GET /api/v1/project/design/handoff", "POST /api/v1/project/design/edit",
		"POST /api/v1/project/design/keep", "POST /api/v1/project/design/undo", "POST /api/v1/project/design/approve",
		"PUT /api/v1/project/theme", "PUT /api/v1/project/reference", "PUT /api/v1/project/design-template"} {
		method, path, _ := strings.Cut(route, " ")
		if w := do(method, path, "{}"); w.Code != http.StatusNotFound {
			t.Errorf("%s: %d %s", route, w.Code, w.Body)
		}
	}
	// No blueprint yet: edits are refused.
	if w := do("PUT", "/api/v1/project/design/rules", `{}`); w.Code != http.StatusConflict {
		t.Fatalf("rules before discovery: %d %s", w.Code, w.Body)
	}
	st, _ := core.NewStore(dir)
	if _, err := st.Update(func(p *core.Project) error {
		p.Discovery, p.Stage = &core.Discovery{Summary: "Klinik"}, core.StageDesign
		p.Platform = &core.PlatformPlan{Targets: []core.PlatformTarget{{Kind: core.PlatformWeb}}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	w := do("PUT", "/api/v1/project/design/rules", `{"principles": ["Padat"]}`)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), `"field":"rules"`) || !strings.Contains(w.Body.String(), "accessibility belum diisi") {
		t.Fatalf("invalid rules: %d %s", w.Code, w.Body)
	}
	rules := `{"source": "new", "principles": ["Padat"], "ux_guidelines": ["Konfirmasi hapus"], "accessibility": ["AA"], "responsive": ["list di mobile"],
	  "tokens": {"colors": [{"name": "action.primary", "value": "#123456"}], "typography": [{"name": "type.body", "value": "Inter 14px"}],
	             "spacing": [{"name": "space.md", "value": "16px"}]}}`
	layout := `{"pages": [{"id": "antrean", "name": "Antrean", "platform": "web", "route": "/antrean", "purpose": "Lihat antrean", "pattern": "detail",
	  "sections": [{"id": "isi", "name": "Isi", "content": ["Nomor"]}], "states": {"loading": "Memuat", "error": "Gagal"}}]}`
	if w := do("PUT", "/api/v1/project/design/layout", layout); w.Code != 400 || !strings.Contains(w.Body.String(), "aturan desain belum ada") {
		t.Fatalf("layout without rules: %d %s", w.Code, w.Body)
	}
	if w := do("PUT", "/api/v1/project/design/rules", rules); w.Code != 400 || !strings.Contains(w.Body.String(), "belum berisi halaman") {
		t.Fatalf("rules without layout: %d %s", w.Code, w.Body)
	}
	// Rules and layout land together the first time through the store; the
	// routes then edit each part.
	if _, err := st.Update(func(p *core.Project) error {
		return json.Unmarshal([]byte(`{"rules": `+rules+`, "layout": `+layout+`}`), &p.UI)
	}); err != nil {
		t.Fatal(err)
	}
	if w := do("PUT", "/api/v1/project/design/layout", strings.Replace(layout, "Antrean", "Antrian", 1)); w.Code != 200 || !strings.Contains(w.Body.String(), "Antrian") {
		t.Fatalf("layout: %d %s", w.Code, w.Body)
	}
	if w := do("GET", "/api/v1/project/blueprint", ""); !strings.Contains(w.Body.String(), `"id":"layout-spec"`) || !strings.Contains(w.Body.String(), `"id":"design-rules"`) {
		t.Fatalf("blueprint: %s", w.Body)
	}
	if w := do("POST", "/api/v1/project/design/revise", `{"text": "lebih padat"}`); w.Code != http.StatusPreconditionFailed {
		t.Fatalf("revise without AI: %d %s", w.Code, w.Body)
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
		t.Fatalf("spec without blueprint: %d %s", w.Code, w.Body)
	}
	st, err := core.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Update(func(p *core.Project) error {
		p.Discovery = &core.Discovery{Summary: "Klinik"}
		p.UI = &core.UIDesign{Layout: &core.LayoutSpec{Pages: []core.LayoutPage{{ID: "antrean", Name: "Antrean", Route: "/antrean", Platform: "web"}}}}
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
		Layout struct {
			Pages []struct {
				ID string `json:"id"`
			} `json:"pages"`
		} `json:"layout"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &out) != nil || len(out.Layout.Pages) != 1 || out.Layout.Pages[0].ID != "antrean" {
		t.Fatalf("spec.json: %d %s", w.Code, w.Body)
	}
}

// The export zip exists only for a finalized project and holds the same
// package finalize writes.
func TestDownloadExport(t *testing.T) {
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
	if w := do("/api/v1/project/export.zip"); w.Code != http.StatusConflict {
		t.Fatalf("draft export: %d %s", w.Code, w.Body)
	}
	st, _ := core.NewStore(dir)
	if _, err := st.Update(func(p *core.Project) error {
		p.Stage, p.Discovery = core.StageReady, &core.Discovery{Summary: "Klinik"}
		p.Requirements = []core.Requirement{{ID: "REQ-001", Text: "Buat janji", Priority: "MUST"}}
		p.Workflow = &core.Workflow{Status: core.CommandFinalized, Version: 1, Commands: []core.Command{
			{ID: "CMD-001", Name: "Kontrak", Objective: "o", Instructions: "i", Status: core.CommandFinalized}}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	w := do("/api/v1/project/export.zip")
	if w.Code != 200 || w.Header().Get("Content-Type") != "application/zip" {
		t.Fatalf("export.zip: %d %s", w.Code, w.Body)
	}
	zr, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	for _, want := range []string{"docs/foundry/manifest.json", "docs/foundry/workflow/CMD-001.md", "docs/foundry/blueprint/requirements.md"} {
		if !slices.Contains(names, want) {
			t.Errorf("zip misses %s: %v", want, names)
		}
	}
	if w := do("/api/v1/project/workflow"); w.Code != 200 || !strings.Contains(w.Body.String(), `"waves":[["CMD-001"]]`) {
		t.Fatalf("workflow: %d %s", w.Code, w.Body)
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

func TestScopeRoutes(t *testing.T) {
	t.Setenv("FOUNDRY_HOME", t.TempDir())
	dir := t.TempDir()
	s := New(nil, "/bin/foundry")
	if _, err := s.Open(dir, "demo", false); err != nil {
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
	st, _ := core.NewStore(dir)
	if _, err := st.Update(func(p *core.Project) error {
		p.Discovery, p.Stage = &core.Discovery{Summary: "Klinik"}, core.StageDesign
		p.Platform = &core.PlatformPlan{Targets: []core.PlatformTarget{{Kind: core.PlatformBackend}}}
		p.Requirements = []core.Requirement{{ID: "REQ-001", Text: "Buat janji", Priority: "MUST"}}
		p.Features = []core.Feature{{ID: "FEAT-001", Name: "Janji", RequirementIDs: []string{"REQ-001"}}}
		p.Decisions = []core.Decision{{ID: "DEC-001", Kind: core.DecisionQuestion, Text: "Gateway?", Critical: true}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	w := do("GET", "/api/v1/project/blueprint", "")
	for _, want := range []string{`"areas":[`, `Threat Model","status":"missing"`, `"states":{`, `"decisions":[{"id":"DEC-001"`} {
		if w.Code != 200 || !strings.Contains(w.Body.String(), want) {
			t.Fatalf("blueprint lacks %s: %d %s", want, w.Code, w.Body)
		}
	}
	for _, c := range []struct {
		method, path, body string
		code               int
		want               string
	}{
		{"PUT", "/api/v1/project/blueprint/areas/security/skip", `{"reason": ""}`, 400, "alasan"},
		{"PUT", "/api/v1/project/blueprint/areas/acceptance/skip", `{"reason": "x"}`, 400, "tidak bisa dilewati"},
		{"PUT", "/api/v1/project/blueprint/areas/security/skip", `{"reason": "Demo tanpa data nyata"}`, 200, "Demo tanpa data nyata"},
		{"DELETE", "/api/v1/project/blueprint/areas/security/skip", ``, 200, "Area security dicakup lagi"},
		{"PUT", "/api/v1/project/blueprint/use-cases", `{"use_cases": [{"name": "Buat", "feature_id": "FEAT-001", "role": "Pasien"}]}`, 400, `"field":"FLOW-001"`},
		{"PUT", "/api/v1/project/blueprint/use-cases", `{"use_cases": [{"name": "Buat", "feature_id": "FEAT-001", "role": "Pasien", "goal": "Jadwal",
			"steps": ["a"], "postconditions": ["b"], "failures": ["c"]}]}`, 200, `"goal":"Jadwal"`},
		{"PUT", "/api/v1/project/blueprint/acceptance", `{"acceptance": [{"requirement": "REQ-001", "criteria": [{"kind": "edge", "text": "Bentrok ditolak"}]}], "definition_of_done": []}`, 400, "Definition of Done"},
		{"PUT", "/api/v1/project/blueprint/acceptance", `{"acceptance": [{"requirement": "REQ-001", "criteria": [{"kind": "edge", "text": "Bentrok ditolak"}]}], "definition_of_done": ["Test lulus"]}`, 200, `"REQ-001-AC1"`},
		{"PUT", "/api/v1/project/blueprint/decisions/DEC-404", `{"resolution": "x"}`, 400, "tidak ada"},
		{"PUT", "/api/v1/project/blueprint/decisions/DEC-001", `{"resolution": "Midtrans"}`, 200, `"resolution":"Midtrans"`},
		{"POST", "/api/v1/project/blueprint/artifacts/ghost/review", `{}`, 400, "tidak ada"},
		{"POST", "/api/v1/project/blueprint/complete", `{}`, 412, ""},
	} {
		if w := do(c.method, c.path, c.body); w.Code != c.code || !strings.Contains(w.Body.String(), c.want) {
			t.Errorf("%s %s: %d %s", c.method, c.path, w.Code, w.Body)
		}
	}
}
