package mcp

import (
	"bufio"
	"encoding/json"
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/config"
	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/agent"
	"github.com/handrawanwdev/foundry/internal/modules/pipeline"
)

type hostAI struct{}

func (hostAI) Runner() (*agent.Runner, bool, error) { return nil, true, nil }

func run(t *testing.T, allow string, lines ...string) []map[string]any {
	t.Helper()
	t.Setenv("FOUNDRY_HOME", t.TempDir())
	st, _ := core.NewStore(t.TempDir())
	st.Init("t")
	perms, err := ParseAllow(allow)
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{Svc: pipeline.New(st, hostAI{}), Allow: perms, Version: "test"}
	var out strings.Builder
	if err := s.Serve(strings.NewReader(strings.Join(lines, "\n")), &out); err != nil {
		t.Fatal(err)
	}
	var res []map[string]any
	sc := bufio.NewScanner(strings.NewReader(out.String()))
	for sc.Scan() {
		var m map[string]any
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			t.Fatalf("bad line %q", sc.Text())
		}
		res = append(res, m)
	}
	return res
}

func text(m map[string]any) (string, bool) {
	r := m["result"].(map[string]any)
	c := r["content"].([]any)[0].(map[string]any)
	return c["text"].(string), r["isError"].(bool)
}

func TestProtocol(t *testing.T) {
	res := run(t, "READ",
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","clientInfo":{"name":"claude-code"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_project","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"next_work","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":5,"method":"nope"}`,
	)
	if len(res) != 5 { // notification gets no reply
		t.Fatalf("got %d replies", len(res))
	}
	if v := res[0]["result"].(map[string]any)["protocolVersion"]; v != "2025-03-26" {
		t.Fatalf("protocol = %v", v)
	}
	listed := res[1]["result"].(map[string]any)["tools"].([]any)
	if n := len(listed); n != len(tools) {
		t.Fatalf("tools = %d", n)
	}
	for _, raw := range listed {
		schema := raw.(map[string]any)["inputSchema"].(map[string]any)
		if schema["required"] == nil {
			t.Fatalf("tool %v has null required; MCP clients require an array", raw.(map[string]any)["name"])
		}
	}
	if txt, isErr := text(res[2]); isErr || !strings.Contains(txt, `"stage": "TELL"`) {
		t.Fatalf("get_project = %s", txt)
	}
	if txt, isErr := text(res[3]); !isErr || !strings.Contains(txt, "MODIFY") {
		t.Fatalf("permission not enforced: %s", txt)
	}
	if res[4]["error"] == nil {
		t.Fatal("unknown method must error")
	}
	c, _ := config.Load()
	if c.MCP == nil || c.MCP.Client != "claude-code" {
		t.Fatalf("heartbeat = %+v", c.MCP)
	}
}

func TestNextWorkWithPermission(t *testing.T) {
	res := run(t, "ALL", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"next_work","arguments":{}}}`)
	if txt, isErr := text(res[0]); isErr || !strings.Contains(txt, `"kind": "none"`) {
		t.Fatalf("next_work = %s", txt)
	}
}

func TestReportProgress(t *testing.T) {
	t.Setenv("FOUNDRY_HOME", t.TempDir())
	st, _ := core.NewStore(t.TempDir())
	st.Init("Klinik")
	st.Update(func(p *core.Project) error {
		p.Name = "Klinik"
		p.Workflow = &core.Workflow{Version: 1, Commands: []core.Command{
			{ID: "CMD-001", Name: "Skema", Objective: "o", Instructions: "i", Acceptance: []string{"ok"}},
		}}
		return nil
	})
	perms, _ := ParseAllow("READ,WRITE")
	s := &Server{Svc: pipeline.New(st, hostAI{}), Allow: perms, Version: "test"}
	lines := []string{
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"report_progress","arguments":{"id":"r1","command":"CMD-001","status":"started","source":"claude-code"}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"report_progress","arguments":{"id":"r1","command":"CMD-001","status":"started"}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"report_progress","arguments":{"id":"r2","command":"CMD-001","status":"done","summary":"siap","evidence":[{"kind":"files","ref":"db/schema.sql"}]}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"get_progress","arguments":{}}}`,
	}
	var out strings.Builder
	if err := s.Serve(strings.NewReader(strings.Join(lines, "\n")), &out); err != nil {
		t.Fatal(err)
	}
	var res []map[string]any
	sc := bufio.NewScanner(strings.NewReader(out.String()))
	for sc.Scan() {
		var m map[string]any
		json.Unmarshal(sc.Bytes(), &m)
		res = append(res, m)
	}
	if txt, isErr := text(res[0]); isErr || !strings.Contains(txt, `"status": "started"`) {
		t.Fatalf("started = %s err=%v", txt, isErr)
	}
	if txt, isErr := text(res[1]); isErr || !strings.Contains(txt, `"duplicate": true`) {
		t.Fatalf("dup = %s", txt)
	}
	if txt, isErr := text(res[2]); isErr || !strings.Contains(txt, `"applied_status": "verified"`) {
		t.Fatalf("done = %s", txt)
	}
	if txt, isErr := text(res[3]); isErr || !strings.Contains(txt, `"verified": 1`) || !strings.Contains(txt, `"complete": true`) {
		t.Fatalf("progress = %s", txt)
	}
}

func TestParseAllow(t *testing.T) {
	if _, err := ParseAllow("READ,DELETE"); err == nil {
		t.Fatal("unknown permission accepted")
	}
	p, _ := ParseAllow("ALL")
	if !p[PermExecute] || !p[PermWrite] {
		t.Fatal("ALL incomplete")
	}
}
