package uicheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/handrawanwdev/foundry/internal/core"
)

func sampleDesign() *core.UIDesign {
	return &core.UIDesign{Format: core.UIDesignFormat, Rules: &core.DesignRules{Source: core.RulesNew,
		Principles: []string{"Padat"}, UX: []string{"Filter di atas tabel"}, Accessibility: []string{"AA"}, Responsive: []string{"list di mobile"},
		Tokens: core.DesignTokens{
			Colors:     []core.Token{{Name: "text.primary", Value: "#111111"}, {Name: "surface.page", Value: "#ffffff"}},
			Typography: []core.Token{{Name: "type.h1", Value: "32px"}, {Name: "type.body", Value: "14px"}},
			Spacing:    []core.Token{{Name: "space.md", Value: "16px"}},
		}},
		Layout: &core.LayoutSpec{Pages: []core.LayoutPage{{ID: "antrean", Name: "Antrean", Platform: core.PlatformWeb, Route: "/antrean",
			Purpose: "Cek antrean", Goal: "GOAL-1", Pattern: "table", PrimaryAction: "Kirim",
			Sections: []core.Section{{ID: "tabel", Name: "Tabel", Content: []string{"No, Pasien, Status"}}},
			States:   map[string]string{"loading": "skeleton", "empty": "kosong", "error": "gagal"}}}}}
}

func sampleProject(t *testing.T) (*core.Project, string) {
	t.Helper()
	dir := t.TempDir()
	p := &core.Project{Dir: dir, Platform: &core.PlatformPlan{Targets: []core.PlatformTarget{{Kind: core.PlatformWeb}}}, UI: sampleDesign()}
	return p, dir
}

func shot(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"), 0o644); err != nil {
		t.Fatal(err)
	}
	return filepath.ToSlash(path)
}

func pageNodes() []core.VisualNode {
	return []core.VisualNode{
		{Path: "body > main > h1", Tag: "h1", Text: "Antrean", Bounds: core.VisualBounds{X: 32, Y: 24, Width: 300, Height: 40},
			Style: core.VisualStyle{Color: "#111111", Background: "#ffffff", FontSize: 32, FontWeight: 700}},
		{Path: "body > main > button", Tag: "button", Role: "button", Text: "Kirim", Bounds: core.VisualBounds{X: 32, Y: 80, Width: 120, Height: 48},
			Style: core.VisualStyle{Color: "#111111", Background: "#ffffff", FontSize: 14, Padding: []float64{16, 16, 16, 16}}},
		{Path: "body > main > img", Tag: "img", Bounds: core.VisualBounds{X: 32, Y: 140, Width: 100, Height: 60},
			Attrs: map[string]string{"alt": "denah"}},
		{Path: "body > main > p", Tag: "p", Text: "Tidak ada antrean", Bounds: core.VisualBounds{X: 32, Y: 210, Width: 240, Height: 20},
			Style: core.VisualStyle{Color: "#111111", Background: "#ffffff", FontSize: 14}},
	}
}

func TestReviewPassesOnlyWithMeasuredEvidence(t *testing.T) {
	p, dir := sampleProject(t)
	in := core.VisualReviewIn{Page: "antrean", Source: "test"}
	for _, vp := range []core.VisualViewport{{Name: "desktop", Width: 1440, Height: 900}, {Name: "mobile", Width: 375, Height: 812}} {
		in.Snapshots = append(in.Snapshots, core.VisualSnapshot{Viewport: vp, State: "default", Screenshot: shot(t, dir, vp.Name+".png"),
			Background: "#ffffff", Nodes: pageNodes()})
	}
	for _, st := range []string{"loading", "empty", "error"} {
		in.Snapshots = append(in.Snapshots, core.VisualSnapshot{Viewport: core.VisualViewport{Name: "desktop", Width: 1440, Height: 900},
			State: st, Screenshot: shot(t, dir, st+".png"), Background: "#ffffff", Nodes: pageNodes()})
	}
	a, err := Review(p, in, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if a.Status != core.VisualPass {
		t.Fatalf("status = %s findings = %+v criteria = %+v", a.Status, a.Findings, a.Criteria)
	}
	if len(a.Screenshots) != 5 || a.Viewports == nil {
		t.Fatalf("audit = %+v", a)
	}
}

func TestReviewWithoutScreenshotIsUntested(t *testing.T) {
	p, _ := sampleProject(t)
	in := core.VisualReviewIn{Page: "antrean", Snapshots: []core.VisualSnapshot{{Viewport: core.VisualViewport{Name: "desktop", Width: 1440, Height: 900},
		Screenshot: "missing.png", Nodes: pageNodes()}}}
	a, err := Review(p, in, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if a.Status != core.VisualUntested {
		t.Fatalf("status = %s", a.Status)
	}
	if !strings.Contains(a.Findings[0].Message, "screenshot tidak tersedia") {
		t.Fatalf("findings = %+v", a.Findings)
	}
}

func TestReviewFindsContrastOverflowLabelsAndTarget(t *testing.T) {
	p, dir := sampleProject(t)
	nodes := []core.VisualNode{
		{Path: "body > main > h1", Tag: "h1", Text: "Antrean", Bounds: core.VisualBounds{X: 0, Y: 0, Width: 300, Height: 40},
			Style: core.VisualStyle{Color: "#bbbbbb", Background: "#ffffff", FontSize: 32}},
		{Path: "body > main > div", Tag: "div", Text: "lebar", Bounds: core.VisualBounds{X: 1300, Y: 300, Width: 300, Height: 40},
			Style: core.VisualStyle{Color: "#111111", Background: "#ffffff", FontSize: 14}},
		{Path: "body > main > button", Tag: "button", Text: "", Bounds: core.VisualBounds{X: 10, Y: 100, Width: 20, Height: 20},
			Style: core.VisualStyle{Background: "#ffffff"}},
		{Path: "body > main > img", Tag: "img", Bounds: core.VisualBounds{X: 10, Y: 200, Width: 100, Height: 60}},
	}
	in := core.VisualReviewIn{Page: "antrean", Snapshots: []core.VisualSnapshot{
		{Viewport: core.VisualViewport{Name: "desktop", Width: 1440, Height: 900}, Screenshot: shot(t, dir, "d.png"), Background: "#ffffff", Nodes: nodes[:2]},
		{Viewport: core.VisualViewport{Name: "mobile", Width: 375, Height: 812}, Screenshot: shot(t, dir, "m.png"), Background: "#ffffff", Nodes: nodes},
	}}
	a, err := Review(p, in, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if a.Status != core.VisualFail {
		t.Fatalf("status = %s findings = %+v", a.Status, a.Findings)
	}
	crit := map[string]string{}
	for _, c := range a.Criteria {
		crit[c.ID] = c.Status
	}
	for _, id := range []string{"contrast", "overflow", "accessibility"} {
		if crit[id] != core.VisualFail {
			t.Errorf("criterion %s = %s", id, crit[id])
		}
	}
	if !strings.Contains(joined(a.Findings), "target sentuh") {
		t.Errorf("touch target missing: %+v", a.Findings)
	}
}

func TestReviewTokenDriftAndResolved(t *testing.T) {
	p, dir := sampleProject(t)
	// 13px padding is outside the 16px token; the image has no alt yet.
	bad := pageNodes()
	bad[1].Style.Padding = []float64{13, 13, 13, 13}
	bad[2].Attrs = nil
	in := core.VisualReviewIn{Page: "antrean", Snapshots: []core.VisualSnapshot{
		{Viewport: core.VisualViewport{Name: "desktop", Width: 1440, Height: 900}, Screenshot: shot(t, dir, "1.png"), Background: "#ffffff", Nodes: bad},
	}}
	a, err := Review(p, in, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(joined(a.Findings), "spasi di luar token") || !strings.Contains(joined(a.Findings), "tanpa atribut alt") {
		t.Fatalf("findings = %+v", a.Findings)
	}
	if len(a.Resolved) != 0 {
		t.Fatalf("resolved on first review = %v", a.Resolved)
	}
	good := pageNodes()
	in.Snapshots[0].Screenshot = shot(t, dir, "2.png")
	in.Snapshots[0].Nodes = good
	b, err := Review(p, in, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Resolved) == 0 || b.Previous == "" {
		t.Fatalf("second review = %+v", b)
	}
	if strings.Contains(joined(b.Findings), "spasi di luar token") {
		t.Fatalf("spacing finding not resolved: %+v", b.Findings)
	}
}

func TestReviewRejectsUnknownPageAndEmptyProof(t *testing.T) {
	p, _ := sampleProject(t)
	if _, err := Review(p, core.VisualReviewIn{Page: "ghost", Snapshots: []core.VisualSnapshot{{}}}, time.Now()); err == nil {
		t.Fatal("unknown page accepted")
	}
	if _, err := Review(p, core.VisualReviewIn{Page: "antrean"}, time.Now()); err == nil {
		t.Fatal("empty proof accepted")
	}
}

func joined(fs []core.VisualFinding) string {
	var out []string
	for _, f := range fs {
		out = append(out, f.Message)
	}
	return strings.Join(out, "\n")
}
