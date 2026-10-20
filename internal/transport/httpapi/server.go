// Package httpapi serves the Foundry REST API (/api/v1) and the embedded UI.
package httpapi

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode"

	"github.com/handrawanw/foundry/internal/config"
	"github.com/handrawanw/foundry/internal/core"
	"github.com/handrawanw/foundry/internal/modules/agent"
	"github.com/handrawanw/foundry/internal/modules/pipeline"
	"github.com/handrawanw/foundry/internal/modules/spec"
	"github.com/handrawanw/foundry/internal/modules/stack"
	"github.com/handrawanw/foundry/internal/modules/structure"
	"github.com/handrawanw/foundry/internal/modules/work"
	"github.com/handrawanw/foundry/internal/safefile"
	"github.com/handrawanw/foundry/internal/transport/httpapi/respond"
	"github.com/handrawanw/foundry/internal/transport/mcp"
)

type Server struct {
	UI     fs.FS // built SPA; may be nil
	Binary string

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
	if err := svc.RecoverStale(); err != nil {
		return nil, err
	}
	p, err = svc.DetectStructure()
	if err != nil {
		return nil, err
	}
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

// Shutdown stops the jobs of the open project and waits up to timeout.
func (s *Server) Shutdown(timeout time.Duration) error {
	svc, err := s.service()
	if err != nil {
		return nil
	}
	return svc.Shutdown(timeout)
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
				fail(w, r, err)
				return
			}
			respond.Success(w, out)
		})
	}

	// AI connection
	api("GET /api/v1/ai", s.getAI)
	api("GET /api/v1/ai/models", s.listModels)
	api("POST /api/v1/ai/connect", s.connectAgent)
	api("POST /api/v1/ai/connect-mcp", s.connectMCP)
	api("POST /api/v1/ai/disconnect", s.disconnectAI)

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
	api("GET /api/v1/project", s.withSvc(func(svc *pipeline.Service, r *http.Request) (any, error) {
		p, err := svc.Store.Load()
		if err != nil {
			return nil, err
		}
		return p, nil
	}))
	api("POST /api/v1/project/story", s.withSvc(func(svc *pipeline.Service, r *http.Request) (any, error) {
		var in struct {
			Story  string
			Survey *core.Survey `json:"survey"`
		}
		if err := decode(r, &in); err != nil {
			return nil, err
		}
		return svc.SubmitStoryWith(in.Story, in.Survey)
	}))
	api("POST /api/v1/project/retry", s.action(func(svc *pipeline.Service) error { return svc.RetryJob() }))
	api("POST /api/v1/project/job/dismiss", s.action(func(svc *pipeline.Service) error { return svc.DismissJob() }))
	api("PUT /api/v1/project/platform", s.withSvc(func(svc *pipeline.Service, r *http.Request) (any, error) {
		var in struct{ Targets []core.PlatformTarget }
		if err := decode(r, &in); err != nil {
			return nil, err
		}
		return svc.SetPlatform(in.Targets)
	}))
	// Structure edits become a proposal; apply/discard decide it. A bare
	// template is a change of the structure type.
	api("PUT /api/v1/project/structure", s.withSvc(func(svc *pipeline.Service, r *http.Request) (any, error) {
		var in struct {
			Ops      []core.StructureOp
			Template string
		}
		if err := decode(r, &in); err != nil {
			return nil, err
		}
		if in.Template != "" {
			in.Ops = append(in.Ops, core.StructureOp{Op: "type", Name: in.Template})
		}
		return svc.ProposeStructure(in.Ops)
	}))
	api("POST /api/v1/project/structure/request", s.withSvc(func(svc *pipeline.Service, r *http.Request) (any, error) {
		var in struct{ Text string }
		if err := decode(r, &in); err != nil {
			return nil, err
		}
		return ok(svc.RequestStructureChange(in.Text))
	}))
	api("POST /api/v1/project/structure/apply", s.withSvc(func(svc *pipeline.Service, r *http.Request) (any, error) { return svc.ApplyStructureProposal() }))
	api("POST /api/v1/project/structure/discard", s.withSvc(func(svc *pipeline.Service, r *http.Request) (any, error) { return svc.DiscardStructureProposal() }))
	api("POST /api/v1/project/structure/approve", s.withSvc(func(svc *pipeline.Service, r *http.Request) (any, error) { return svc.ApproveStructure() }))
	api("POST /api/v1/project/structure/recommend", s.withSvc(func(svc *pipeline.Service, r *http.Request) (any, error) { return svc.RecommendStructure() }))
	// Technology changes become a proposal; apply/discard decide it.
	api("PUT /api/v1/project/stack", s.withSvc(func(svc *pipeline.Service, r *http.Request) (any, error) {
		var in struct{ Changes []stack.Change }
		if err := decode(r, &in); err != nil {
			return nil, err
		}
		return svc.ProposeStack(in.Changes)
	}))
	api("POST /api/v1/project/stack/request", s.withSvc(func(svc *pipeline.Service, r *http.Request) (any, error) {
		var in struct{ Text string }
		if err := decode(r, &in); err != nil {
			return nil, err
		}
		return ok(svc.RequestStackChange(in.Text))
	}))
	api("POST /api/v1/project/stack/apply", s.withSvc(func(svc *pipeline.Service, r *http.Request) (any, error) { return svc.ApplyStackProposal() }))
	api("POST /api/v1/project/stack/discard", s.withSvc(func(svc *pipeline.Service, r *http.Request) (any, error) { return svc.DiscardStackProposal() }))
	api("POST /api/v1/project/stack/approve", s.withSvc(func(svc *pipeline.Service, r *http.Request) (any, error) { return svc.ApproveStack() }))
	api("POST /api/v1/project/stack/review-design", s.withSvc(func(svc *pipeline.Service, r *http.Request) (any, error) { return svc.AckTechReview() }))
	api("POST /api/v1/project/stack/recommend", s.withSvc(func(svc *pipeline.Service, r *http.Request) (any, error) { return svc.RecommendStack() }))
	api("GET /api/v1/stack/catalog", func(r *http.Request) (any, error) {
		return map[string]any{"catalog": stack.Catalog, "profiles": stack.Profiles, "areas": core.TechAreas}, nil
	})
	// UI design: design rules and layout spec (blueprint artifacts).
	api("GET /api/v1/project/design", s.withSvc(func(svc *pipeline.Service, r *http.Request) (any, error) {
		p, err := svc.Store.Load()
		if err != nil {
			return nil, err
		}
		var issues []core.DesignIssue
		if p.UI.Written() {
			issues = p.DesignIssues(nil)
		}
		if issues == nil {
			issues = []core.DesignIssue{}
		}
		vocab := map[string]any{"patterns": core.LayoutPatterns, "states": core.UIStates, "required_states": core.RequiredStates,
			"platforms": core.UIPlatform}
		return map[string]any{"design": p.UI, "issues": issues, "has_ui": p.HasUI(), "platforms": p.UIPlatforms(), "vocabulary": vocab}, nil
	}))
	api("POST /api/v1/project/design/generate", s.action(func(svc *pipeline.Service) error { return svc.GenerateDesign() }))
	api("POST /api/v1/project/design/revise", s.withSvc(func(svc *pipeline.Service, r *http.Request) (any, error) {
		var in struct {
			Text  string
			Focus []string
		}
		if err := decode(r, &in); err != nil {
			return nil, err
		}
		return ok(svc.ReviseDesign(in.Text, in.Focus))
	}))
	api("PUT /api/v1/project/design/rules", s.withSvc(func(svc *pipeline.Service, r *http.Request) (any, error) {
		var in core.DesignRules
		if err := decode(r, &in); err != nil {
			return nil, err
		}
		return svc.EditDesignRules(in)
	}))
	api("PUT /api/v1/project/design/layout", s.withSvc(func(svc *pipeline.Service, r *http.Request) (any, error) {
		var in core.LayoutSpec
		if err := decode(r, &in); err != nil {
			return nil, err
		}
		return svc.EditLayout(in)
	}))
	// Final spec: the same documents Finalize writes into docs/.
	api("GET /api/v1/project/spec", s.withSvc(func(svc *pipeline.Service, r *http.Request) (any, error) {
		p, err := svc.Store.Load()
		if err != nil {
			return nil, err
		}
		return map[string]any{"markdown": spec.Markdown(p), "spec": spec.Export(p)}, nil
	}))

	// Blueprint: the built-in artifacts and the ones written for the project.
	api("GET /api/v1/project/blueprint", s.withSvc(func(svc *pipeline.Service, r *http.Request) (any, error) {
		p, err := svc.Store.Load()
		if err != nil {
			return nil, err
		}
		return map[string]any{"artifacts": spec.Artifacts(p), "has_ui": p.HasUI(), "request": p.ArtifactRequest,
			"blueprint_hash": spec.BlueprintHash(spec.Artifacts(p))}, nil
	}))
	api("POST /api/v1/project/blueprint/revise", s.withSvc(func(svc *pipeline.Service, r *http.Request) (any, error) {
		var in struct {
			Text  string
			Focus []string
		}
		if err := decode(r, &in); err != nil {
			return nil, err
		}
		return ok(svc.ReviseArtifacts(in.Text, in.Focus))
	}))
	api("PUT /api/v1/project/blueprint/artifacts/{id}", s.withSvc(func(svc *pipeline.Service, r *http.Request) (any, error) {
		var in core.Artifact
		if err := decode(r, &in); err != nil {
			return nil, err
		}
		return svc.EditArtifact(r.PathValue("id"), in)
	}))
	api("DELETE /api/v1/project/blueprint/artifacts/{id}", s.withSvc(func(svc *pipeline.Service, r *http.Request) (any, error) {
		return svc.RemoveArtifact(r.PathValue("id"))
	}))
	// Command workflow: instructions for external coding agents; never run here.
	api("GET /api/v1/project/workflow", s.withSvc(func(svc *pipeline.Service, r *http.Request) (any, error) {
		p, err := svc.Store.Load()
		if err != nil {
			return nil, err
		}
		return map[string]any{"workflow": p.Workflow, "report": pipeline.CheckWorkflow(p)}, nil
	}))
	api("POST /api/v1/project/workflow/generate", s.action(func(svc *pipeline.Service) error { return svc.GenerateWorkflow() }))
	api("POST /api/v1/project/workflow/revise", s.withSvc(func(svc *pipeline.Service, r *http.Request) (any, error) {
		var in struct {
			Text  string
			Focus []string
		}
		if err := decode(r, &in); err != nil {
			return nil, err
		}
		return ok(svc.ReviseWorkflow(in.Text, in.Focus))
	}))
	api("PUT /api/v1/project/workflow/commands/{id}", s.withSvc(func(svc *pipeline.Service, r *http.Request) (any, error) {
		var in work.CommandEdit
		if err := decode(r, &in); err != nil {
			return nil, err
		}
		return svc.EditCommand(r.PathValue("id"), in)
	}))
	// needs_review true marks the command for review; false records it as
	// reviewed against the current artifacts.
	api("POST /api/v1/project/workflow/commands/{id}/review", s.withSvc(func(svc *pipeline.Service, r *http.Request) (any, error) {
		var in struct {
			NeedsReview bool `json:"needs_review"`
			Reason      string
		}
		if err := decode(r, &in); err != nil {
			return nil, err
		}
		return svc.ReviewCommand(r.PathValue("id"), in.NeedsReview, in.Reason)
	}))
	// Finalize approves blueprint + workflow and writes the export package.
	api("POST /api/v1/project/finalize", s.action(func(svc *pipeline.Service) error { return svc.Finalize() }))
	api("POST /api/v1/project/reopen", s.action(func(svc *pipeline.Service) error { return svc.Reopen() }))
	api("GET /api/v1/project/export", s.withSvc(func(svc *pipeline.Service, r *http.Request) (any, error) {
		p, err := svc.Store.Load()
		if err != nil {
			return nil, err
		}
		type file struct {
			Path string `json:"path"`
			Size int    `json:"size"`
		}
		files := []file{}
		for path, body := range spec.Files(p) {
			files = append(files, file{path, len(body)})
		}
		slices.SortFunc(files, func(a, b file) int { return strings.Compare(a.Path, b.Path) })
		return map[string]any{"dir": spec.ExportDir, "finalized": p.Finalized(), "manifest": spec.BuildManifest(p),
			"files": files, "blockers": pipeline.Blockers(p)}, nil
	}))

	mux.HandleFunc("GET /api/v1/events", s.events)
	mux.HandleFunc("GET /api/v1/project/export.zip", s.downloadExport)
	mux.HandleFunc("GET /api/v1/project/spec.md", s.downloadSpec)
	mux.HandleFunc("GET /api/v1/project/spec.json", s.downloadSpec)
	mux.HandleFunc("/", s.spa)
	return guard(mux)
}

// downloadSpec serves the final spec as Markdown (spec.md) or JSON
// (spec.json) for an external implementation tool.
func (s *Server) downloadSpec(w http.ResponseWriter, r *http.Request) {
	svc, err := s.service()
	if err != nil {
		fail(w, r, err)
		return
	}
	p, err := svc.Store.Load()
	if err != nil {
		fail(w, r, err)
		return
	}
	if p.Discovery == nil {
		respond.Fail(w, respond.CodeNotFound, "blueprint belum dibuat", http.StatusNotFound)
		return
	}
	if strings.HasSuffix(r.URL.Path, ".json") {
		b, err := json.MarshalIndent(spec.Export(p), "", "  ")
		if err != nil {
			fail(w, r, err)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="`+specDownloadName(p.Name, ".json")+`"`)
		w.Write(b)
		return
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+specDownloadName(p.Name, ".md")+`"`)
	w.Write([]byte(spec.Markdown(p)))
}

// downloadExport sends the export package of a finalized project as a
// zip: the same files finalize wrote into docs/foundry, under that same
// path, so extracting it at a repository root matches the paths the
// package and the agent prompt use.
func (s *Server) downloadExport(w http.ResponseWriter, r *http.Request) {
	svc, err := s.service()
	if err != nil {
		fail(w, r, err)
		return
	}
	p, err := svc.Store.Load()
	if err != nil {
		fail(w, r, err)
		return
	}
	if !p.Finalized() {
		respond.Fail(w, respond.CodeConflict, "finalisasi blueprint dan workflow dulu", http.StatusConflict)
		return
	}
	files := spec.Files(p)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	slices.Sort(names)
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	name := "foundry" + strings.TrimPrefix(specDownloadName(p.Name, ""), "spec")
	folder := spec.ExportDir + "/"
	for _, path := range names {
		f, err := zw.Create(folder + path)
		if err == nil {
			_, err = f.Write([]byte(files[path]))
		}
		if err != nil {
			fail(w, r, err)
			return
		}
	}
	if err := zw.Close(); err != nil {
		fail(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`.zip"`)
	w.Write(buf.Bytes())
}

func specDownloadName(name, ext string) string {
	var b strings.Builder
	b.WriteString("spec")
	dash := true
	for _, r := range strings.TrimSpace(name) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			b.WriteRune(r)
			dash = false
			continue
		}
		dash = true
	}
	b.WriteString(ext)
	return b.String()
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

// disconnectAI drops the active connection. Disconnecting an MCP host also
// forgets its heartbeat, so a host that is still attached no longer counts as
// alive. {"all": true} additionally clears the heartbeat of a host that was
// set up by hand and is not the active connection.
func (s *Server) disconnectAI(r *http.Request) (any, error) {
	var in struct {
		All bool `json:"all"`
	}
	_ = decode(r, &in) // body is optional
	_, err := config.Update(func(c *config.Config) {
		if in.All || (c.Connection != nil && c.Connection.Mode == config.ModeMCP) {
			c.MCP = nil
		}
		c.Connection = nil
	})
	return map[string]bool{"ok": err == nil}, err
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
		return nil, err
	}
	entries, err := readDir(dir)
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
		respond.Fail(w, respond.CodeInternal, "streaming unsupported", http.StatusInternalServerError)
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
			fmt.Fprint(w, "event: ping\ndata: 0\n\n") // a named event: the page can see it, a comment it cannot
			fl.Flush()
		case <-tick.C:
		}
	}
}

// ---------- SPA ----------

func (s *Server) spa(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		respond.Fail(w, respond.CodeNotFound, "not found", http.StatusNotFound)
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/")
	if s.UI != nil && name != "" {
		if f, err := s.UI.Open(name); err == nil {
			st, _ := f.Stat()
			f.Close()
			if st != nil && !st.IsDir() {
				http.FileServer(http.FS(s.UI)).ServeHTTP(w, r)
				return
			}
		}
	}
	// Hashed Vite files must 404. Serving index.html as JS makes the browser
	// report "error loading dynamically imported module".
	if strings.HasPrefix(name, "assets/") {
		http.NotFound(w, r)
		return
	}
	if s.UI == nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><title>Foundry</title><p>UI belum di-build. Jalankan <code>make web</code> lalu build ulang.</p>`)
		return
	}
	// Client-side routes (/simple/*) fall back to index.html.
	b, err := fs.ReadFile(s.UI, "index.html")
	if err != nil {
		respond.Fail(w, respond.CodeInternal, "index.html tidak ada", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
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
	case errors.Is(err, pipeline.ErrBusy), errors.Is(err, pipeline.ErrFinalized):
		return http.StatusConflict
	case errors.Is(err, pipeline.ErrWrongStage), errors.Is(err, pipeline.ErrNothingToDo), errors.Is(err, pipeline.ErrIncomplete):
		return http.StatusConflict
	case errors.Is(err, pipeline.ErrNoAI):
		return http.StatusPreconditionFailed
	case errors.Is(err, core.ErrNotFound), errors.Is(err, errNoProject):
		return http.StatusNotFound
	case internal(err):
		return http.StatusInternalServerError
	}
	return http.StatusBadRequest // a domain rule the request broke
}

// internal reports an error of the system rather than of the request: IO,
// a process, a corrupt or locked state file, a cancelled operation.
// Messages built by Foundry's own rules carry none of these.
func internal(err error) bool {
	var (
		pathErr *fs.PathError
		linkErr *os.LinkError
		sysErr  *os.SyscallError
		errno   syscall.Errno
		execErr *exec.Error
		exitErr *exec.ExitError
		synErr  *json.SyntaxError
		typErr  *json.UnmarshalTypeError
	)
	return errors.As(err, &pathErr) || errors.As(err, &linkErr) || errors.As(err, &sysErr) || errors.As(err, &errno) ||
		errors.As(err, &execErr) || errors.As(err, &exitErr) || errors.As(err, &synErr) || errors.As(err, &typErr) ||
		errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, core.ErrLocked) || errors.Is(err, config.ErrLocked) || errors.Is(err, safefile.ErrTimeout)
}

func decode(r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return badRequest("body JSON tidak valid")
	}
	return nil
}

// internalMessage is all a client learns of an internal error; the error
// itself, which may name paths or carry git or tool output, goes to the log.
const internalMessage = "Terjadi kesalahan internal di Foundry. Detailnya ada di log server."

// fail maps a service error to the standard error envelope.
func fail(w http.ResponseWriter, r *http.Request, err error) {
	status := statusFor(err)
	msg := err.Error()
	if status >= http.StatusInternalServerError {
		log.Printf("api %s %s: %v", r.Method, r.URL.Path, err)
		msg = internalMessage
	}
	// An invalid design lists every problem, so the editor can show them.
	var invalid *core.DesignInvalid
	if errors.As(err, &invalid) {
		details := make([]respond.Detail, len(invalid.Issues))
		for i, is := range invalid.Issues {
			details[i] = respond.Detail{Field: is.Where, Message: is.Message}
		}
		respond.Fail(w, respond.CodeFor(status), msg, status, details...)
		return
	}
	respond.Fail(w, respond.CodeFor(status), msg, status)
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
		// The Host header is the client's word; the peer address is not.
		// Both matter when --addr listens on more than loopback.
		peer, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			peer = r.RemoteAddr
		}
		if (!isLocal(host) || !isLocal(peer)) && os.Getenv("FOUNDRY_ALLOW_REMOTE") != "1" {
			respond.Fail(w, respond.CodeForbidden, "host tidak diizinkan", http.StatusForbidden)
			return
		}
		if o := r.Header.Get("Origin"); o != "" {
			u, err := url.Parse(o)
			// Same host, or a loopback origin (Vite dev server on another port).
			if err != nil || (u.Host != r.Host && !isLocal(u.Hostname())) {
				respond.Fail(w, respond.CodeForbidden, "origin tidak diizinkan", http.StatusForbidden)
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
