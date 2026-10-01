// Package httpapi serves the Foundry REST API (/api/v1) and the embedded UI.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/handrawanw/foundry/internal/config"
	"github.com/handrawanw/foundry/internal/core"
	"github.com/handrawanw/foundry/internal/modules/agent"
	"github.com/handrawanw/foundry/internal/modules/pipeline"
	"github.com/handrawanw/foundry/internal/modules/scan"
	"github.com/handrawanw/foundry/internal/modules/structure"
	"github.com/handrawanw/foundry/internal/modules/verify"
	"github.com/handrawanw/foundry/internal/modules/work"
	"github.com/handrawanw/foundry/internal/transport/mcp"
)

type Server struct {
	UI     fs.FS // built SPA; may be nil
	Binary string
	// Render screenshots designed pages for the visual check; may be nil.
	Render pipeline.Renderer

	mu  sync.RWMutex
	svc *pipeline.Service
}

func New(ui fs.FS, binary string) *Server {
	return &Server{UI: ui, Binary: binary}
}

var errNoProject = errors.New("belum ada project yang dibuka")

// Open switches the active project. create=false requires an existing
// folder; create=true makes it (New Project).
func (s *Server) Open(dir, name string, create bool) (*core.Project, error) {
	if err := s.idle(); err != nil {
		return nil, err
	}
	dir = expandHome(dir)
	st, err := core.NewStore(dir)
	if err != nil {
		return nil, err
	}
	if create {
		if err := os.MkdirAll(st.Dir, 0o755); err != nil {
			return nil, err
		}
	} else if info, err := os.Stat(st.Dir); err != nil || !info.IsDir() {
		return nil, badRequest("folder tidak ditemukan: " + st.Dir)
	}
	p, err := st.Init(name)
	if err != nil {
		return nil, err
	}
	svc := pipeline.New(st, pipeline.ConfigAI{})
	if s.Render != nil {
		svc.Render = s.Render
	}
	svc.RecoverStale()
	s.mu.Lock()
	s.svc = svc
	s.mu.Unlock()
	config.Update(func(c *config.Config) { c.AddRecent(st.Dir) })
	return p, nil
}

// Close detaches the active project. Its state stays on disk.
func (s *Server) Close() error {
	if err := s.idle(); err != nil {
		return err
	}
	s.mu.Lock()
	s.svc = nil
	s.mu.Unlock()
	return nil
}

// idle refuses to switch projects while an agent job of the current project
// is still running in this process.
func (s *Server) idle() error {
	svc, err := s.service()
	if err != nil {
		return nil
	}
	p, err := svc.Store.Load()
	if err == nil && p.Job != nil && p.Job.Status == core.JobRunning {
		return fmt.Errorf("%w: hentikan atau tunggu proses \"%s\" selesai sebelum menutup project", pipeline.ErrBusy, p.Job.Message)
	}
	return nil
}

func (s *Server) service() (*pipeline.Service, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.svc == nil {
		return nil, errNoProject
	}
	return s.svc, nil
}

func expandHome(dir string) string {
	if dir == "~" || strings.HasPrefix(dir, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, strings.TrimPrefix(dir, "~"))
	}
	return dir
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	api := func(pattern string, h func(r *http.Request) (any, error)) {
		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			out, err := h(r)
			if err != nil {
				writeJSON(w, statusFor(err), map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
	}

	// AI connection
	api("GET /api/v1/ai", s.getAI)
	api("GET /api/v1/ai/models", s.listModels)
	api("POST /api/v1/ai/connect", s.connectAgent)
	api("POST /api/v1/ai/connect-mcp", s.connectMCP)
	api("POST /api/v1/ai/disconnect", func(r *http.Request) (any, error) {
		_, err := config.Update(func(c *config.Config) { c.Connection = nil })
		return map[string]bool{"ok": err == nil}, err
	})

	// Projects
	api("GET /api/v1/projects", func(r *http.Request) (any, error) {
		c, err := config.Load()
		if err != nil {
			return nil, err
		}
		cur := ""
		if svc, err := s.service(); err == nil {
			cur = svc.Store.Dir
		}
		return map[string]any{"current": cur, "recent": c.Recent}, nil
	})
	api("POST /api/v1/projects/open", func(r *http.Request) (any, error) {
		var in struct {
			Dir, Name string
			Create    bool
		}
		if err := decode(r, &in); err != nil {
			return nil, err
		}
		if strings.TrimSpace(in.Dir) == "" {
			return nil, badRequest("folder project wajib diisi")
		}
		return s.Open(in.Dir, in.Name, in.Create)
	})
	// Removes one entry from the recent list (dir empty = clear all). Project files stay.
	api("POST /api/v1/projects/recent/remove", func(r *http.Request) (any, error) {
		var in struct{ Dir string }
		if err := decode(r, &in); err != nil {
			return nil, err
		}
		_, err := config.Update(func(c *config.Config) { c.RemoveRecent(in.Dir) })
		return ok(err)
	})
	api("POST /api/v1/projects/close", func(r *http.Request) (any, error) { return ok(s.Close()) })
	api("GET /api/v1/fs", s.browse)
	api("GET /api/v1/templates", func(r *http.Request) (any, error) { return structure.Templates, nil })

	// Project pipeline
	api("GET /api/v1/project", s.withSvc(func(svc *pipeline.Service, r *http.Request) (any, error) { return svc.Store.Load() }))
	api("POST /api/v1/project/story", s.withSvc(func(svc *pipeline.Service, r *http.Request) (any, error) {
		var in struct{ Story string }
		if err := decode(r, &in); err != nil {
			return nil, err
		}
		return svc.SubmitStory(in.Story)
	}))
	api("POST /api/v1/project/retry", s.action(func(svc *pipeline.Service) error { return svc.RetryJob() }))
	api("PUT /api/v1/project/platform", s.withSvc(func(svc *pipeline.Service, r *http.Request) (any, error) {
		var in struct{ Targets []core.PlatformTarget }
		if err := decode(r, &in); err != nil {
			return nil, err
		}
		return svc.SetPlatform(in.Targets)
	}))
	api("PUT /api/v1/project/structure", s.withSvc(func(svc *pipeline.Service, r *http.Request) (any, error) {
		var in struct{ Template string }
		if err := decode(r, &in); err != nil {
			return nil, err
		}
		return svc.SetTemplate(in.Template)
	}))
	api("POST /api/v1/project/design/revise", s.withSvc(func(svc *pipeline.Service, r *http.Request) (any, error) {
		var in struct {
			Feedback string
			Screens  map[string]string
			Scope    *core.ChangeScope
		}
		if err := decode(r, &in); err != nil {
			return nil, err
		}
		return ok(svc.ReviseDesign(in.Feedback, in.Screens, in.Scope))
	}))
	api("POST /api/v1/project/design/keep", s.action(func(svc *pipeline.Service) error { return svc.KeepDesignChange() }))
	api("POST /api/v1/project/design/undo", s.action(func(svc *pipeline.Service) error { return svc.UndoDesignChange() }))
	api("GET /api/v1/project/design/handoff", s.withSvc(func(svc *pipeline.Service, r *http.Request) (any, error) {
		p, err := svc.Store.Load()
		if err != nil {
			return nil, err
		}
		return map[string]any{"pages": work.Handoffs(p), "blockers": work.Blockers(p)}, nil
	}))
	api("POST /api/v1/project/design/approve", s.action(func(svc *pipeline.Service) error { return svc.ApproveDesign() }))
	api("POST /api/v1/project/build/start", s.action(func(svc *pipeline.Service) error { return svc.StartBuild() }))
	api("POST /api/v1/project/build/stop", s.action(func(svc *pipeline.Service) error { return svc.StopBuild() }))
	api("PATCH /api/v1/project/tasks/{id}", s.withSvc(func(svc *pipeline.Service, r *http.Request) (any, error) {
		var in struct{ Status core.TaskStatus }
		if err := decode(r, &in); err != nil {
			return nil, err
		}
		return svc.MoveTask(r.PathValue("id"), in.Status)
	}))
	api("POST /api/v1/project/verify", s.action(func(svc *pipeline.Service) error { return svc.Verify() }))
	api("POST /api/v1/project/fix", s.action(func(svc *pipeline.Service) error { return svc.Fix() }))
	api("GET /api/v1/project/files", s.listFiles)
	api("GET /api/v1/project/file", s.readFile)
	api("GET /api/v1/project/docs", s.withSvc(func(svc *pipeline.Service, r *http.Request) (any, error) {
		p, err := svc.Store.Load()
		if err != nil {
			return nil, err
		}
		return map[string]any{"markdown": verify.Markdown(p), "run": scan.Scan(p.Dir).Run}, nil
	}))

	// Technical Support Assistant
	api("POST /api/v1/project/support/ask", s.withSvc(func(svc *pipeline.Service, r *http.Request) (any, error) {
		var in struct{ Question string }
		if err := decode(r, &in); err != nil {
			return nil, err
		}
		return ok(svc.AskSupport(in.Question))
	}))
	api("POST /api/v1/project/support/retry", s.action(func(svc *pipeline.Service) error { return svc.RetrySupport() }))
	api("POST /api/v1/project/support/clear", s.action(func(svc *pipeline.Service) error { return svc.ClearSupport() }))
	api("POST /api/v1/project/support/escalate", s.withSvc(func(svc *pipeline.Service, r *http.Request) (any, error) {
		var in core.Escalation
		if err := decode(r, &in); err != nil {
			return nil, err
		}
		return svc.Escalate(in)
	}))

	mux.HandleFunc("GET /api/v1/events", s.events)
	mux.HandleFunc("/", s.spa)
	return guard(mux)
}

func (s *Server) withSvc(fn func(*pipeline.Service, *http.Request) (any, error)) func(*http.Request) (any, error) {
	return func(r *http.Request) (any, error) {
		svc, err := s.service()
		if err != nil {
			return nil, err
		}
		return fn(svc, r)
	}
}

func (s *Server) action(fn func(*pipeline.Service) error) func(*http.Request) (any, error) {
	return s.withSvc(func(svc *pipeline.Service, r *http.Request) (any, error) { return ok(fn(svc)) })
}

func ok(err error) (any, error) {
	if err != nil {
		return nil, err
	}
	return map[string]bool{"ok": true}, nil
}

// ---------- AI ----------

type agentInfo struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Binary   string   `json:"binary"`
	Detected bool     `json:"detected"`
	Path     string   `json:"path,omitempty"`
	Models   []string `json:"models"`
	Efforts  []string `json:"efforts"`
}

func (s *Server) getAI(r *http.Request) (any, error) {
	c, err := config.Load()
	if err != nil {
		return nil, err
	}
	var agents []agentInfo
	for _, p := range agent.Presets {
		path, found := p.Detected()
		agents = append(agents, agentInfo{ID: p.ID, Name: p.Name, Binary: p.Binary, Detected: found, Path: path, Models: p.Models, Efforts: p.Efforts})
	}
	dir, _ := os.Getwd()
	if svc, err := s.service(); err == nil {
		dir = svc.Store.Dir
	}
	ready := false
	if c.Connection != nil {
		ready = c.Connection.Mode == config.ModeAgent || c.MCPAlive()
	}
	return map[string]any{
		"connection": c.Connection,
		"ready":      ready,
		"mcp":        c.MCP,
		"mcp_alive":  c.MCPAlive(),
		"agents":     agents,
		"mcp_config": mcp.ClientConfig(s.Binary, dir),
	}, nil
}

// listModels asks the agent CLI for its current models (?agent=id).
func (s *Server) listModels(r *http.Request) (any, error) {
	p, ok := agent.PresetByID(r.URL.Query().Get("agent"))
	if !ok {
		return nil, badRequest("agent tidak dikenal")
	}
	if _, found := p.Detected(); !found {
		return map[string]any{"models": p.Models}, nil
	}
	return map[string]any{"models": p.ListModels(r.Context())}, nil
}

func (s *Server) connectAgent(r *http.Request) (any, error) {
	var in struct {
		AgentID string   `json:"agent_id"`
		Command []string `json:"command"`
		agent.Options
	}
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	if in.AgentID == "" {
		in.AgentID = "custom"
	}
	if in.AgentID == "custom" && len(in.Command) == 0 {
		return nil, badRequest("perintah agent custom wajib diisi")
	}
	in.Model, in.Effort = strings.TrimSpace(in.Model), strings.TrimSpace(in.Effort)
	if len(in.Command) > 0 {
		in.Options = agent.Options{} // a custom command carries its own flags
	}
	runner, err := agent.NewRunner(in.AgentID, in.Command, in.Options)
	if err != nil {
		return nil, badRequest(err.Error())
	}
	dir := os.TempDir()
	if svc, err := s.service(); err == nil {
		dir = svc.Store.Dir
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Minute)
	defer cancel()
	if err := runner.Ping(ctx, dir); err != nil {
		return nil, badRequest("Agent tidak merespons: " + err.Error())
	}
	c, err := config.Update(func(c *config.Config) {
		c.Connection = &config.Connection{Mode: config.ModeAgent, AgentID: in.AgentID, Command: in.Command,
			Model: in.Model, Effort: in.Effort, Since: time.Now().UTC()}
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"connection": c.Connection, "agent": runner.Name}, nil
}

func (s *Server) connectMCP(r *http.Request) (any, error) {
	c, err := config.Update(func(c *config.Config) {
		c.Connection = &config.Connection{Mode: config.ModeMCP, Since: time.Now().UTC()}
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"connection": c.Connection, "mcp_alive": c.MCPAlive()}, nil
}

// ---------- Folder browser (Open / New Project dialog) ----------

type dirEntry struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	IsProject bool   `json:"is_project"` // has Foundry state
}

// browse lists sub-folders only (names, no file contents).
func (s *Server) browse(r *http.Request) (any, error) {
	dir := r.URL.Query().Get("path")
	if dir == "" {
		dir = "~"
	}
	dir, err := filepath.Abs(expandHome(dir))
	if err != nil {
		return nil, badRequest(err.Error())
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, badRequest("tidak bisa membaca folder: " + dir)
	}
	dirs := []dirEntry{}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") || e.Name() == "node_modules" {
			continue
		}
		p := filepath.Join(dir, e.Name())
		_, serr := os.Stat(filepath.Join(p, core.StateDir, "project.json"))
		dirs = append(dirs, dirEntry{Name: e.Name(), Path: p, IsProject: serr == nil})
	}
	parent := filepath.Dir(dir)
	if parent == dir {
		parent = ""
	}
	_, serr := os.Stat(filepath.Join(dir, core.StateDir, "project.json"))
	return map[string]any{"path": dir, "parent": parent, "dirs": dirs, "is_project": serr == nil}, nil
}

// ---------- SSE: push a version whenever the state file changes ----------

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	fl, okf := w.(http.Flusher)
	if !okf {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	var last time.Time
	lastDir := "\x00" // forces an initial event
	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()
	ping := time.NewTicker(15 * time.Second)
	defer ping.Stop()
	for {
		var mt time.Time
		dir := ""
		if svc, err := s.service(); err == nil {
			mt, dir = svc.Store.ModTime(), svc.Store.Dir
		}
		if !mt.Equal(last) || dir != lastDir {
			last, lastDir = mt, dir
			fmt.Fprintf(w, "event: change\ndata: %d\n\n", mt.UnixNano())
			fl.Flush()
		}
		select {
		case <-r.Context().Done():
			return
		case <-ping.C:
			fmt.Fprint(w, ": ping\n\n")
			fl.Flush()
		case <-tick.C:
		}
	}
}

// ---------- SPA ----------

func (s *Server) spa(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	if s.UI == nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><title>Foundry</title><p>UI belum di-build. Jalankan <code>make web</code> lalu build ulang.</p>`)
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/")
	if name != "" {
		if f, err := s.UI.Open(name); err == nil {
			st, _ := f.Stat()
			f.Close()
			if st != nil && !st.IsDir() {
				http.FileServer(http.FS(s.UI)).ServeHTTP(w, r)
				return
			}
		}
	}
	// Client-side routes (/simple/*) fall back to index.html.
	b, err := fs.ReadFile(s.UI, "index.html")
	if err != nil {
		http.Error(w, "index.html tidak ada", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(b)
}

// ---------- helpers ----------

type httpError struct {
	code int
	msg  string
}

func (e httpError) Error() string { return e.msg }

func badRequest(msg string) error { return httpError{http.StatusBadRequest, msg} }

func statusFor(err error) int {
	var he httpError
	switch {
	case errors.As(err, &he):
		return he.code
	case errors.Is(err, pipeline.ErrBusy):
		return http.StatusConflict
	case errors.Is(err, work.ErrSupportBusy), errors.Is(err, work.ErrNoSupportIntel):
		return http.StatusConflict
	case errors.Is(err, pipeline.ErrGate), errors.Is(err, pipeline.ErrWrongStage), errors.Is(err, pipeline.ErrNothingToDo), errors.Is(err, pipeline.ErrIncomplete):
		return http.StatusConflict
	case errors.Is(err, pipeline.ErrNoAI):
		return http.StatusPreconditionFailed
	case errors.Is(err, core.ErrNotFound), errors.Is(err, errNoProject):
		return http.StatusNotFound
	}
	return http.StatusBadRequest
}

func decode(r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return badRequest("body JSON tidak valid")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

// guard blocks cross-site requests: the API can run agents and commands, so
// only same-origin browser calls are accepted (defends against CSRF and DNS
// rebinding from pages open in the same browser).
func guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.Host)
		if err != nil {
			host = r.Host
		}
		if !isLocal(host) && os.Getenv("FOUNDRY_ALLOW_REMOTE") != "1" {
			http.Error(w, "host tidak diizinkan", http.StatusForbidden)
			return
		}
		if o := r.Header.Get("Origin"); o != "" {
			u, err := url.Parse(o)
			// Same host, or a loopback origin (Vite dev server on another port).
			if err != nil || (u.Host != r.Host && !isLocal(u.Hostname())) {
				http.Error(w, "origin tidak diizinkan", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func isLocal(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
