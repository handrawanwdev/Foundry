// Package mcp exposes Foundry to any AI host over the Model Context Protocol
// (JSON-RPC 2.0, newline-delimited, stdio). It is the only bridge protocol:
// every tool calls the same pipeline.Service the UI uses.
package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/handrawanwdev/foundry/internal/config"
	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/pipeline"
	"github.com/handrawanwdev/foundry/internal/modules/progress"
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
				"To implement a finalized blueprint, take one command at a time with next_command and format json (it carries the instructions and what its dependencies left; format json leaves out the Markdown copy of the same command) and read only the artifacts it references with get_blueprint ids; call get_workflow or the whole get_blueprint only when you need the full picture. Screens follow the design-rules and layout-spec artifacts. " +
				"Report every command with report_progress when you start, finish, fail, get blocked, skip (with a reason), or hand work for review. " +
				"A command is not done until Foundry verifies evidence against its acceptance criteria. Call get_progress for the next eligible commands; do not start a command before its dependencies are verified. Do not assume Foundry ran the work. " +
				"After implementing screens, submit measured browser snapshots with record_visual_review; without snapshots and screenshots the UI is untested, and a build or unit test is not visual proof. Call create_visual_fixes to turn findings into a workflow command.",
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
		return toolResult(compactJSON(out), false), nil
	}
	return nil, &rpcError{Code: -32601, Message: "method not found: " + req.Method}
}

// compactJSON is a tool answer: no indentation and no HTML escaping, since
// the reader is a model paying per token and Markdown is full of < > &.
func compactJSON(v any) string {
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	e.Encode(v)
	return strings.TrimSuffix(b.String(), "\n")
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
	if required == nil {
		required = []string{}
	}
	return map[string]any{"type": "object", "properties": props, "required": required}
}

var str = map[string]any{"type": "string"}

var tools = []tool{
	{Name: "get_project", perm: PermRead, Description: "Project summary: stage, story, feature map, discovery, architecture blueprint, platform, structure, technology stack, job, counts.", InputSchema: obj(nil)},
	{Name: "get_requirements", perm: PermRead, Description: "Requirements with acceptance criteria, features, sub-features, use cases (flows) and the Definition of Done.", InputSchema: obj(nil)},
	{Name: "get_blueprint", perm: PermRead, Description: "Blueprint: every artifact (requirements, architecture, technology, design rules, layout spec, use cases, acceptance criteria, decisions and the project's own documents) as Markdown, the structured architecture, area coverage, artifact states, the decision register and binding_decisions (decided entries that override every other artifact). Pass ids to get only those artifacts with their body, or outline true for the artifact list without bodies.",
		InputSchema: obj(map[string]any{"ids": map[string]any{"type": "array", "items": str}, "outline": map[string]any{"type": "boolean"}})},
	{Name: "get_workflow", perm: PermRead, Description: "Command workflow for the coding agent: commands with objective, instructions, dependencies, feature and subfeature links, blueprint references, expected outputs and acceptance criteria; the execution waves; validation issues, commands that need review, feature-to-command impact, and current execution progress.", InputSchema: obj(nil)},
	{Name: "get_design", perm: PermRead, Description: "UI design as structured data: design rules (principles, UX guidelines, tokens, components, accessibility, responsive, interaction, platform rules) and layout spec (shells, pages, sections, hierarchy, grid, states, links), plus validation issues.", InputSchema: obj(nil)},
	{Name: "get_spec", perm: PermRead, Description: "Whole spec for implementation outside Foundry (JSON: requirements, flows, stack, structure, blueprint, artifacts, design rules, layout spec, workflow) plus the same as Markdown. format json or markdown returns only that one; both is the default.",
		InputSchema: obj(map[string]any{"format": map[string]any{"type": "string", "enum": []string{"json", "markdown", "both"}}})},
	{Name: "next_work", perm: PermModify, Description: "Get the next thing Foundry needs from you (feature_map, discovery, design, stack, tech, structure, repo, plan, blueprint or workflow).", InputSchema: obj(nil)},
	{Name: "submit_work", perm: PermWrite, Description: "Hand back a result (the JSON object requested by next_work).",
		InputSchema: obj(map[string]any{"kind": map[string]any{"type": "string", "enum": []string{"feature_map", "discovery", "design", "stack", "tech", "structure", "repo", "plan", "blueprint", "workflow"}}, "result": str}, "kind", "result")},
	{Name: "get_progress", perm: PermRead, Description: "Execution progress of the command workflow: overall, per-module, per-feature, per-subfeature and per-workflow weighted percentage, command counts (completed, in progress, not started, failed, blocked, unknown), dependency state, next eligible commands, warnings and status history. Foundry does not run the commands.", InputSchema: obj(nil)},
	{Name: "get_visual_review", perm: PermRead, Description: "Measured quality of implemented screens: per-page status (pass/partial/fail/untested), findings with severity/evidence/fix, criteria results, and a summary. Empty until snapshots are submitted; pages without screenshot evidence are untested, never passed.", InputSchema: obj(nil)},
	{Name: "record_visual_review", perm: PermWrite, Description: "Submit measured browser snapshots for one designed page: viewport, screenshot file inside the project, state (default/loading/empty/error/...), and measured DOM nodes (tag, text, bounds, computed style, attrs). Foundry checks contrast, overflow, spacing/typography tokens, labels, headings, targets and the primary action. Use after implementing or fixing a screen; resubmit to verify fixes.", InputSchema: obj(map[string]any{
		"page":   str,
		"url":    str,
		"source": str,
		"snapshots": map[string]any{"type": "array", "items": obj(map[string]any{
			"viewport":   obj(map[string]any{"name": str, "width": map[string]any{"type": "number"}, "height": map[string]any{"type": "number"}}, "width", "height"),
			"state":      str,
			"screenshot": str,
			"background": str,
			"nodes": map[string]any{"type": "array", "items": obj(map[string]any{
				"path": str, "tag": str, "role": str, "text": str,
				"bounds": obj(map[string]any{"x": map[string]any{"type": "number"}, "y": map[string]any{"type": "number"},
					"width": map[string]any{"type": "number"}, "height": map[string]any{"type": "number"}}),
				"style": obj(map[string]any{"color": str, "background": str, "font_size": map[string]any{"type": "number"},
					"font_weight": map[string]any{"type": "integer"}, "line_height": map[string]any{"type": "number"},
					"padding": map[string]any{"type": "array", "items": map[string]any{"type": "number"}},
					"margin":  map[string]any{"type": "array", "items": map[string]any{"type": "number"}},
					"gap":     map[string]any{"type": "number"}, "display": str}),
				"attrs": map[string]any{"type": "object"},
			})},
		})},
	}, "page", "snapshots")},
	{Name: "create_visual_fixes", perm: PermWrite, Description: "Turn the open visual findings into one concrete correction command in the existing workflow and reopen a finalized project for that revision. Use after get_visual_review shows findings; then implement the command wave and resubmit snapshots.", InputSchema: obj(nil)},
	{Name: "next_command", perm: PermWrite, Description: "Get the command to work on next: instructions, dependencies and what they left, affected files, acceptance criteria. Commands with unmet dependencies are never handed out. With claim true the command is marked started for your session, so another agent does not get it; the session gets back the command it already holds. Failed, blocked and review commands are listed under attention and only move on by name.",
		InputSchema: obj(map[string]any{"session": str, "claim": map[string]any{"type": "boolean"}, "format": briefFormatSchema})},
	{Name: "get_command", perm: PermRead, Description: "One command in full with its execution state, or with history true its reports, status changes and earlier revisions.",
		InputSchema: obj(map[string]any{"id": str, "history": map[string]any{"type": "boolean"}, "format": briefFormatSchema}, "id")},
	{Name: "report_progress", perm: PermWrite, Description: "Report execution progress for one command, by its id. Send started when you begin, progress while working (summary, percent, files), then done, failed, blocked, skipped (reason required) or review. Include a stable id for retries and your session. Done without required evidence is stored as review, not verified. Duplicate ids are idempotent. A warning in the answer means the report was stored but something is off, such as a revised command.",
		InputSchema: obj(map[string]any{
			"id":               str,
			"seq":              map[string]any{"type": "integer"},
			"project":          str,
			"workflow_version": map[string]any{"type": "integer"},
			"command":          str,
			"status":           map[string]any{"type": "string", "enum": core.AgentReportStatuses},
			"session":          str,
			"percent":          map[string]any{"type": "integer", "minimum": 0, "maximum": 100},
			"started_at":       str,
			"finished_at":      str,
			"updated_at":       str,
			"summary":          str,
			"reason":           str,
			"source":           str,
			"files":            map[string]any{"type": "array", "items": str},
			"changes":          map[string]any{"type": "array", "items": str},
			"evidence":         map[string]any{"type": "array", "items": obj(map[string]any{"kind": str, "ref": str})},
			"tests":            map[string]any{"type": "array", "items": obj(map[string]any{"name": str, "result": str, "detail": str})},
		}, "command", "status")},
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
		Kind    string          `json:"kind"`
		Result  json.RawMessage `json:"result"`
		ID      string          `json:"id"`
		Session string          `json:"session"`
		Claim   bool            `json:"claim"`
		History bool            `json:"history"`
		IDs     []string        `json:"ids"`
		Outline bool            `json:"outline"`
		Format  string          `json:"format"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, fmt.Errorf("argumen tidak valid: %v", err)
		}
	}
	p, err := s.Svc.Snapshot()
	if err != nil {
		return nil, err
	}
	switch name {
	case "get_project":
		pages := 0
		if p.UI != nil && p.UI.Layout != nil {
			pages = len(p.UI.Layout.Pages)
		}
		return map[string]any{"name": p.Name, "stage": p.Stage, "story": p.Story, "feature_map": p.FeatureMap, "discovery": p.Discovery,
			"blueprint": p.Blueprint,
			"platform":  p.Platform, "structure": p.Structure, "stack": p.Stack, "job": p.Job,
			"counts": map[string]int{"requirements": len(p.Requirements), "features": len(p.Features), "pages": pages}}, nil
	case "get_requirements":
		return map[string]any{"requirements": p.Requirements, "features": p.Features, "flows": p.Flows, "definition_of_done": p.Done,
			"binding_decisions": p.BindingDecisions()}, nil
	case "get_blueprint":
		docs := spec.Artifacts(p)
		if len(args.IDs) > 0 || args.Outline {
			return selectArtifacts(p, docs, args.IDs, args.Outline)
		}
		areas, states := core.BlueprintChecks(p, docs)
		return map[string]any{"artifacts": docs, "blueprint": p.Blueprint, "areas": areas,
			"states": states, "decisions": p.Decisions, "binding_decisions": p.BindingDecisions()}, nil
	case "get_workflow":
		return map[string]any{"workflow": p.Workflow, "report": pipeline.CheckWorkflow(p), "finalized": p.Finalized(),
			"binding_decisions": p.BindingDecisions(), "progress": s.Svc.ProgressOf(p)}, nil
	case "get_design":
		var issues []core.Issue
		if p.UI.Written() {
			issues = p.DesignIssues(nil)
		}
		return map[string]any{"design": p.UI, "issues": issues,
			"vocabulary": map[string]any{"patterns": core.LayoutPatterns, "states": core.UIStates, "required_states": core.RequiredStates,
				"platforms": core.UIPlatform, "field_presentations": core.FieldPresentations, "decision_sources": core.DecisionSources}}, nil
	case "get_spec":
		switch args.Format {
		case "json":
			return map[string]any{"spec": spec.Export(p)}, nil
		case "markdown":
			return map[string]any{"markdown": spec.Markdown(p)}, nil
		case "", "both":
			return map[string]any{"spec": spec.Export(p), "markdown": spec.Markdown(p)}, nil
		}
		return nil, fmt.Errorf("format %q tidak dikenal: json, markdown atau both", args.Format)
	case "next_work":
		return s.Svc.NextWork()
	case "get_progress":
		return s.Svc.ProgressOf(p), nil
	case "next_command":
		if err := checkBriefFormat(args.Format); err != nil {
			return nil, err // before a claim, so a typo never takes a command
		}
		var out any
		var err error
		if args.Claim {
			out, err = s.Svc.ClaimCommand("", args.Session, s.source())
		} else {
			out, err = s.Svc.NextCommand(args.Session)
		}
		if err != nil {
			return nil, err
		}
		return shapeBrief(out, args.Format)
	case "get_command":
		if err := checkBriefFormat(args.Format); err != nil {
			return nil, err
		}
		if args.History {
			return s.Svc.CommandDetail(args.ID)
		}
		out, err := s.Svc.Command(args.ID)
		if err != nil {
			return nil, err
		}
		return shapeBrief(out, args.Format)
	case "get_visual_review":
		summary, audit, err := s.Svc.VisualReport()
		if err != nil {
			return nil, err
		}
		return map[string]any{"summary": summary, "audit": audit}, nil
	case "record_visual_review":
		var in core.VisualReviewIn
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &in); err != nil {
				return nil, fmt.Errorf("argumen tidak valid: %v", err)
			}
		}
		if in.Source == "" {
			in.Source = s.source()
		}
		return s.Svc.RecordVisualReview(in, "mcp")
	case "create_visual_fixes":
		id, err := s.Svc.CreateVisualFixes()
		if err != nil {
			return nil, err
		}
		return map[string]any{"command": id}, nil
	case "report_progress":
		var in progress.ReportIn
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &in); err != nil {
				return nil, fmt.Errorf("argumen tidak valid: %v", err)
			}
		}
		if in.Source == "" {
			in.Source = s.source()
		}
		return s.Svc.ReportProgress(in, "mcp")
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

// selectArtifacts answers a selective get_blueprint: the named artifacts in
// full, or every artifact without its body. An unknown id is an error, so a
// typo never reads as an empty artifact. Binding decisions always come along:
// they override every artifact.
func selectArtifacts(p *core.Project, docs []core.Artifact, ids []string, outline bool) (any, error) {
	want := map[string]bool{}
	for _, id := range ids {
		if !slices.ContainsFunc(docs, func(d core.Artifact) bool { return d.ID == id }) {
			return nil, fmt.Errorf("artefak %q tidak ada", id)
		}
		want[id] = true
	}
	out := []core.Artifact{}
	for _, d := range docs {
		if len(want) > 0 && !want[d.ID] {
			continue
		}
		if outline {
			d.Body = ""
		}
		out = append(out, d)
	}
	return map[string]any{"artifacts": out, "binding_decisions": p.BindingDecisions()}, nil
}

var briefFormatSchema = map[string]any{"type": "string", "enum": []string{"json", "markdown", "both"},
	"description": "json: the structured command only; markdown: the command as Markdown only; both (default): both copies."}

func checkBriefFormat(f string) error {
	switch f {
	case "", "both", "json", "markdown":
		return nil
	}
	return fmt.Errorf("format %q tidak dikenal: json, markdown atau both", f)
}

// shapeBrief keeps one copy of the command in a brief (a get_command answer,
// or the brief inside a next_command answer); the brief carries it twice,
// as structure and as Markdown. json drops the Markdown; markdown drops the
// structure but keeps the command id and review reason. Progress,
// dependency results, binding decisions and the rest stay as they are.
func shapeBrief(v any, format string) (any, error) {
	if format == "" || format == "both" {
		return v, nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if err := d.Decode(&m); err != nil {
		return nil, err
	}
	brief := m
	if b, ok := m["brief"].(map[string]any); ok {
		brief = b
	}
	cmd, ok := brief["command"].(map[string]any)
	if !ok {
		return m, nil // nothing handed out, no brief
	}
	if format == "json" {
		delete(brief, "markdown")
		return m, nil
	}
	ref := map[string]any{"id": cmd["id"]}
	if r, ok := cmd["review"]; ok {
		ref["review"] = r
	}
	brief["command"] = ref
	return m, nil
}

// source names the connected host for reports that do not name themselves.
func (s *Server) source() string {
	if s.client != "" {
		return s.client
	}
	return "mcp"
}

// ClientConfig is the JSON snippet an AI host needs to launch this server.
func ClientConfig(binary, dir string) map[string]any {
	return map[string]any{"mcpServers": map[string]any{"foundry": map[string]any{
		"command": binary,
		"args":    []string{"mcp", "--allow", "READ,WRITE,MODIFY", "--project", dir},
	}}}
}
