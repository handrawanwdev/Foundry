package pipeline_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/pipeline"
)

func TestVisualReviewFeedsCorrectionCommand(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	designReady(t, s)
	if err := workflowReady(s); err != nil {
		t.Fatal(err)
	}
	p := load(t, s)
	shotPath := filepath.Join(p.Dir, "review", "jadwal-desktop.png")
	if err := os.MkdirAll(filepath.Dir(shotPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(shotPath, []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"), 0o644); err != nil {
		t.Fatal(err)
	}
	in := core.VisualReviewIn{Page: "jadwal", Source: "test", Snapshots: []core.VisualSnapshot{{
		Viewport:   core.VisualViewport{Name: "desktop", Width: 1440, Height: 900},
		Screenshot: "review/jadwal-desktop.png",
		Background: "#ffffff",
		Nodes: []core.VisualNode{
			{Path: "body > h1", Tag: "h1", Text: "Jadwal", Bounds: core.VisualBounds{Width: 200, Height: 40},
				Style: core.VisualStyle{Color: "#111111", Background: "#ffffff", FontSize: 32}},
			{Path: "body > img", Tag: "img", Bounds: core.VisualBounds{Y: 100, Width: 100, Height: 60}},
		},
	}}}
	a, err := s.RecordVisualReview(in, "test")
	if err != nil {
		t.Fatal(err)
	}
	if a.Status != core.VisualFail || len(a.Findings) == 0 {
		t.Fatalf("audit = %+v", a)
	}
	summary, audit, err := s.VisualReport()
	if err != nil {
		t.Fatal(err)
	}
	if summary.Fail != 1 || audit == nil || audit.Pages["jadwal"].Status != core.VisualFail {
		t.Fatalf("summary = %+v audit = %+v", summary, audit)
	}

	before := len(load(t, s).Workflow.Commands)
	id, err := s.CreateVisualFixes()
	if err != nil {
		t.Fatal(err)
	}
	p = load(t, s)
	if len(p.Workflow.Commands) != before+1 || p.Workflow.Commands[before].ID != id {
		t.Fatalf("commands = %+v", p.Workflow.Commands)
	}
	fix := p.Workflow.Commands[before]
	if fix.DependsOn == nil || !strings.Contains(fix.Instructions, "jadwal") || !strings.Contains(fix.Instructions, "alt") {
		t.Fatalf("fix command = %+v", fix)
	}
	if r := pipeline.CheckWorkflow(p); len(r.Issues) != 0 {
		t.Fatalf("workflow issues after fix = %+v", r.Issues)
	}
}
