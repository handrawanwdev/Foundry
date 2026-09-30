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

	"github.com/handrawanw/foundry/internal/config"
	"github.com/handrawanw/foundry/internal/modules/pipeline"
	"github.com/handrawanw/foundry/internal/modules/work"
)

type Perm string

const (
	PermRead    Perm = "READ"
	PermWrite   Perm = "WRITE"
	PermModify  Perm = "MODIFY"
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
				"then hand the result back with the tool named in `submit`. Never mark work done yourself; Foundry runs build/test.",
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
	{Name: "get_project", perm: PermRead, Description: "Project summary: stage, story, discovery, platform, structure, job, counts.", InputSchema: obj(nil)},
	{Name: "get_requirements", perm: PermRead, Description: "Requirements, features, sub-features and user flows.", InputSchema: obj(nil)},
	{Name: "get_design", perm: PermRead, Description: "Design pages with wireframe components and status.", InputSchema: obj(nil)},
	{Name: "get_tasks", perm: PermRead, Description: "All tasks (Kanban), optionally filtered by status.",
		InputSchema: obj(map[string]any{"status": map[string]any{"type": "string", "enum": []string{"TODO", "IN_PROGRESS", "REVIEW", "DONE", "BLOCKED"}}})},
	{Name: "get_task", perm: PermRead, Description: "One task with its full brief.", InputSchema: obj(map[string]any{"task_id": str}, "task_id")},
	{Name: "get_verification", perm: PermRead, Description: "Latest verification: checks, issues, requirement traceability.", InputSchema: obj(nil)},
	{Name: "next_work", perm: PermModify, Description: "Get the next thing Foundry needs from you (discovery, design, plan or a task). Claims a task when building.", InputSchema: obj(nil)},
	{Name: "submit_work", perm: PermWrite, Description: "Hand back a discovery/design/plan/design_qa result (the JSON object requested by next_work).",
		InputSchema: obj(map[string]any{"kind": map[string]any{"type": "string", "enum": []string{"discovery", "design", "plan", "design_qa"}}, "result": str}, "kind", "result")},
	{Name: "submit_task", perm: PermExecute, Description: "Report a task as implemented. Foundry runs build/test; DONE only if checks pass and every acceptance criterion is met with evidence.",
		InputSchema: obj(map[string]any{"task_id": str, "criteria": map[string]any{"type": "array", "items": obj(map[string]any{
			"index": map[string]any{"type": "integer"}, "met": map[string]any{"type": "boolean"}, "evidence": str}, "index", "met", "evidence")}}, "task_id", "criteria")},
	{Name: "update_task", perm: PermModify, Description: "Add a progress note to a task or mark it BLOCKED with the reason.",
		InputSchema: obj(map[string]any{"task_id": str, "note": str, "blocked": map[string]any{"type": "boolean"}}, "task_id", "note")},
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
	if t.perm == PermExecute && !s.Allow[PermModify] {
		return nil, fmt.Errorf("submit_task butuh izin MODIFY dan EXECUTE")
	}
	var args struct {
		Status   string          `json:"status"`
		TaskID   string          `json:"task_id"`
		Kind     string          `json:"kind"`
		Result   json.RawMessage `json:"result"`
		Criteria []work.Verdict  `json:"criteria"`
		Note     string          `json:"note"`
		Blocked  bool            `json:"blocked"`
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
		return map[string]any{"name": p.Name, "stage": p.Stage, "story": p.Story, "discovery": p.Discovery,
			"platform": p.Platform, "structure": p.Structure, "job": p.Job,
			"counts": map[string]int{"requirements": len(p.Requirements), "features": len(p.Features), "tasks": len(p.Tasks)}}, nil
	case "get_requirements":
		return map[string]any{"requirements": p.Requirements, "features": p.Features, "flows": p.Flows}, nil
	case "get_design":
		return p.Design, nil
	case "get_tasks":
		var out []map[string]any
		for _, t := range p.Tasks {
			if args.Status == "" || string(t.Status) == args.Status {
				out = append(out, map[string]any{"id": t.ID, "title": t.Title, "status": t.Status, "feature_id": t.FeatureID, "depends_on": t.DependsOn})
			}
		}
		return out, nil
	case "get_task":
		t := p.Task(args.TaskID)
		if t == nil {
			return nil, fmt.Errorf("task %s tidak ada", args.TaskID)
		}
		return map[string]any{"task": t, "brief": work.TaskPrompt(p, t)}, nil
	case "get_verification":
		return p.Verification, nil
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
	case "submit_task":
		t, b, err := s.Svc.SubmitTask(args.TaskID, args.Criteria)
		if err != nil {
			return nil, err
		}
		return map[string]any{"task": t, "build": b}, nil
	case "update_task":
		return s.Svc.UpdateTaskNote(args.TaskID, args.Note, args.Blocked)
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
