// Package mcp exposes Foundry to any AI host over the Model Context Protocol
// (JSON-RPC 2.0, newline-delimited, stdio). It is the only bridge protocol:
// every tool calls the same pipeline.Service the UI uses.
package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/handrawanwdev/foundry/internal/config"
	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/pipeline"
	"github.com/handrawanwdev/foundry/internal/modules/spec"
)

type Perm string

const (
	PermRead   Perm = "READ"
	PermWrite  Perm = "WRITE"
	PermModify Perm = "MODIFY"
	// PermExecute is still accepted in --allow for old host configs; no
	// tool needs it since Foundry no longer runs builds.
	PermExecute Perm = "EXECUTE"
)

// ParseAllow parses "READ,WRITE" or "ALL".
func ParseAllow(s string) (map[Perm]bool, error) {
	out := map[Perm]bool{PermRead: true}
	for _, part := range strings.Split(s, ",") {
		switch p := Perm(strings.ToUpper(strings.TrimSpace(part))); p {
		case "":
		case "ALL":
			out[PermWrite], out[PermModify], out[PermExecute] = true, true, true
		case PermRead, PermWrite, PermModify, PermExecute:
			out[p] = true
		default:
			return nil, fmt.Errorf("permission %q tidak dikenal", part)
		}
	}
	return out, nil
}

type Server struct {
	Svc     *pipeline.Service
	Allow   map[Perm]bool
	Version string
	client  string
}

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Serve reads requests from r until EOF.
func (s *Server) Serve(r io.Reader, w io.Writer) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	enc := json.NewEncoder(w)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var req request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			enc.Encode(response{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcError{Code: -32700, Message: "parse error"}})
			continue
		}
		res, rerr := s.handle(req)
		if len(req.ID) == 0 {
			continue // notification
		}
		resp := response{JSONRPC: "2.0", ID: req.ID, Result: res, Error: rerr}
		if err := enc.Encode(resp); err != nil {
			return err
		}
	}
	return sc.Err()
}

func (s *Server) handle(req request) (any, *rpcError) {
	switch req.Method {
	case "initialize":
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
			ClientInfo      struct {
				Name string `json:"name"`
			} `json:"clientInfo"`
		}
		json.Unmarshal(req.Params, &p)
		s.client = p.ClientInfo.Name
		s.heartbeat()
		version := p.ProtocolVersion
		if version == "" {
			version = "2024-11-05"
		}
		return map[string]any{
			"protocolVersion": version,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "foundry", "version": s.Version},
			"instructions": "Foundry is the orchestrator and source of truth. Loop: call next_work, do exactly what its prompt says, " +
				"then hand the result back with the tool named in `submit`. Never mark work done yourself; Foundry validates what you submit. " +
				"To implement a finalized blueprint, read get_blueprint and get_workflow and follow the commands wave by wave; screens follow the design-rules and layout-spec artifacts.",
		}, nil
	case "ping":
		return map[string]any{}, nil
	case "notifications/initialized", "notifications/cancelled":
		return nil, nil
	case "tools/list":
		return map[string]any{"tools": toolList()}, nil
	case "tools/call":
		var p struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return nil, &rpcError{Code: -32602, Message: "invalid params"}
		}
		s.heartbeat()
		out, err := s.call(p.Name, p.Arguments)
		if err != nil {
			return toolResult(err.Error(), true), nil
		}
		b, _ := json.MarshalIndent(out, "", "  ")
		return toolResult(string(b), false), nil
	}
	return nil, &rpcError{Code: -32601, Message: "method not found: " + req.Method}
}

func toolResult(text string, isErr bool) map[string]any {
	return map[string]any{"content": []map[string]any{{"type": "text", "text": text}}, "isError": isErr}
}

func (s *Server) heartbeat() {
	client := s.client
	if client == "" {
		client = "AI Host"
	}
	config.Update(func(c *config.Config) {
		c.MCP = &config.MCPHeartbeat{Client: client, Dir: s.Svc.Store.Dir, At: time.Now().UTC()}
	})
}

type tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
	perm        Perm
}

func obj(props map[string]any, required ...string) map[string]any {
	if props == nil {
		props = map[string]any{}
	}
	return map[string]any{"type": "object", "properties": props, "required": required}
}

var str = map[string]any{"type": "string"}

var tools = []tool{
	{Name: "get_project", perm: PermRead, Description: "Project summary: stage, story, discovery, architecture blueprint, platform, structure, technology stack, job, counts.", InputSchema: obj(nil)},
	{Name: "get_requirements", perm: PermRead, Description: "Requirements with acceptance criteria, features, sub-features, use cases (flows) and the Definition of Done.", InputSchema: obj(nil)},
	{Name: "get_blueprint", perm: PermRead, Description: "Blueprint: every artifact (requirements, architecture, technology, design rules, layout spec, use cases, acceptance criteria, decisions and the project's own documents) as Markdown, the structured architecture, area coverage, artifact states and the decision register.", InputSchema: obj(nil)},
	{Name: "get_workflow", perm: PermRead, Description: "Command workflow for the coding agent: commands with objective, instructions, dependencies, blueprint references, expected outputs and acceptance criteria; the execution waves; validation issues and commands that need review.", InputSchema: obj(nil)},
	{Name: "get_design", perm: PermRead, Description: "UI design as structured data: design rules (principles, UX guidelines, tokens, components, accessibility, responsive, interaction, platform rules) and layout spec (shells, pages, sections, hierarchy, grid, states, links), plus validation issues.", InputSchema: obj(nil)},
	{Name: "get_spec", perm: PermRead, Description: "Whole spec for implementation outside Foundry (JSON: requirements, flows, stack, structure, blueprint, artifacts, design rules, layout spec, workflow) plus the same as Markdown.", InputSchema: obj(nil)},
	{Name: "next_work", perm: PermModify, Description: "Get the next thing Foundry needs from you (discovery, design, stack, structure, blueprint or workflow).", InputSchema: obj(nil)},
	{Name: "submit_work", perm: PermWrite, Description: "Hand back a result (the JSON object requested by next_work).",
		InputSchema: obj(map[string]any{"kind": map[string]any{"type": "string", "enum": []string{"discovery", "design", "stack", "structure", "blueprint", "workflow"}}, "result": str}, "kind", "result")},
}

func toolList() []tool { return tools }

func (s *Server) call(name string, raw json.RawMessage) (any, error) {
	var t *tool
	for i := range tools {
		if tools[i].Name == name {
			t = &tools[i]
		}
	}
	if t == nil {
		return nil, fmt.Errorf("tool %q tidak ada", name)
	}
	if !s.Allow[t.perm] {
		return nil, fmt.Errorf("izin %s belum diberikan. Jalankan `foundry mcp --allow %s`", t.perm, t.perm)
	}
	var args struct {
		Kind   string          `json:"kind"`
		Result json.RawMessage `json:"result"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, fmt.Errorf("argumen tidak valid: %v", err)
		}
	}
	p, err := s.Svc.Store.Load()
	if err != nil {
		return nil, err
	}
	switch name {
	case "get_project":
		pages := 0
		if p.UI != nil && p.UI.Layout != nil {
			pages = len(p.UI.Layout.Pages)
		}
		return map[string]any{"name": p.Name, "stage": p.Stage, "story": p.Story, "discovery": p.Discovery,
			"blueprint": p.Blueprint,
			"platform":  p.Platform, "structure": p.Structure, "stack": p.Stack, "job": p.Job,
			"counts": map[string]int{"requirements": len(p.Requirements), "features": len(p.Features), "pages": pages}}, nil
	case "get_requirements":
		return map[string]any{"requirements": p.Requirements, "features": p.Features, "flows": p.Flows, "definition_of_done": p.Done}, nil
	case "get_blueprint":
		docs := spec.Artifacts(p)
		return map[string]any{"artifacts": docs, "blueprint": p.Blueprint, "areas": core.Coverage(p, docs),
			"states": core.ArtifactStates(p, docs), "decisions": p.Decisions}, nil
	case "get_workflow":
		return map[string]any{"workflow": p.Workflow, "report": pipeline.CheckWorkflow(p), "finalized": p.Finalized()}, nil
	case "get_design":
		var issues []core.Issue
		if p.UI.Written() {
			issues = p.DesignIssues(nil)
		}
		return map[string]any{"design": p.UI, "issues": issues}, nil
	case "get_spec":
		return map[string]any{"spec": spec.Export(p), "markdown": spec.Markdown(p)}, nil
	case "next_work":
		return s.Svc.NextWork()
	case "submit_work":
		// Accept the result as a JSON object or as a string containing JSON.
		result := string(args.Result)
		var asString string
		if json.Unmarshal(args.Result, &asString) == nil {
			result = asString
		}
		if err := s.Svc.SubmitWork(args.Kind, result); err != nil {
			return nil, err
		}
		np, _ := s.Svc.Store.Load()
		return map[string]any{"accepted": true, "stage": np.Stage, "job": np.Job, "next": "Panggil next_work untuk pekerjaan berikutnya."}, nil
	}
	return nil, fmt.Errorf("tool %q tidak ada", name)
}

// ClientConfig is the JSON snippet an AI host needs to launch this server.
func ClientConfig(binary, dir string) map[string]any {
	return map[string]any{"mcpServers": map[string]any{"foundry": map[string]any{
		"command": binary,
		"args":    []string{"mcp", "--allow", "ALL", "--project", dir},
	}}}
}
