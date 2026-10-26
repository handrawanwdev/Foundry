package mcp

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
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

func TestVisualReviewTool(t *testing.T) {
	t.Setenv("FOUNDRY_HOME", t.TempDir())
	dir := t.TempDir()
	st, _ := core.NewStore(dir)
	st.Init("Klinik")
	if err := os.MkdirAll(filepath.Join(dir, "review"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "review", "detail.png"), []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"), 0o644); err != nil {
		t.Fatal(err)
	}
	st.Update(func(p *core.Project) error {
		p.Dir = dir
		p.Platform = &core.PlatformPlan{Targets: []core.PlatformTarget{{Kind: core.PlatformWeb}}}
		p.UI = &core.UIDesign{Format: core.UIDesignFormat, Rules: &core.DesignRules{Source: core.RulesNew},
			Layout: &core.LayoutSpec{Pages: []core.LayoutPage{{ID: "detail", Name: "Detail", Platform: core.PlatformWeb, Pattern: "detail",
				Purpose: "Lihat", Goal: "GOAL-1", Sections: []core.Section{{ID: "isi", Name: "Isi", Content: []string{"Data"}}}}}}}
		return nil
	})
	perms, _ := ParseAllow("READ,WRITE")
	s := &Server{Svc: pipeline.New(st, hostAI{}), Allow: perms, Version: "test"}
	review := `{"page":"detail","source":"claude-code","snapshots":[{"viewport":{"name":"desktop","width":1440,"height":900},"screenshot":"review/detail.png","nodes":[{"path":"body > h1","tag":"h1","text":"Detail","bounds":{"x":0,"y":0,"width":200,"height":40},"style":{"color":"#111111","background":"#ffffff","font_size":24}}]}]}`
	lines := []string{
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"record_visual_review","arguments":` + review + `}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"get_visual_review","arguments":{}}}`,
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
	if txt, isErr := text(res[0]); isErr || !strings.Contains(txt, `"page": "detail"`) {
		t.Fatalf("record = %s err=%v", txt, isErr)
	}
	if txt, isErr := text(res[1]); isErr || !strings.Contains(txt, `"pages": 1`) || !strings.Contains(txt, `"detail"`) {
		t.Fatalf("report = %s err=%v", txt, isErr)
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
