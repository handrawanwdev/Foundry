package work

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/handrawanw/foundry/internal/core"
)

func qualityProject(pages ...core.Page) *core.Project {
	return &core.Project{Features: []core.Feature{{ID: "FEAT-001", Name: "Tiket"}},
		Design: &core.Design{Status: core.DesignDraft, Domain: "ticketing", Pages: pages}}
}

func TestQualitySerialization(t *testing.T) {
	q := core.DesignQuality{Overall: 84, Dimensions: map[string]int{"hierarchy": 90}, Confidence: "medium", Decision: "revision_required",
		Reasons:    []core.QAIssue{{PageID: "PAGE-001", Severity: "major", Category: "responsive", Source: "lint", Rule: "mobile_split"}},
		Compared:   &core.QualityComparison{Round: 1, Before: core.QualityScore{Overall: 72}, After: core.QualityScore{Overall: 86}, Verdict: "improved"},
		MeasuredAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)}
	b, err := json.Marshal(core.Design{Pages: []core.Page{}, Quality: &q})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"quality"`, `"overall":84`, `"compared_to_previous"`, `"decision":"revision_required"`, `"rule":"mobile_split"`} {
		if !strings.Contains(string(b), key) {
			t.Errorf("json misses %s: %s", key, b)
		}
	}
	var back core.Design
	if err := json.Unmarshal(b, &back); err != nil || !reflect.DeepEqual(*back.Quality, q) {
		t.Fatalf("round trip: %v %+v", err, back.Quality)
	}

	// Older designs have no quality and no rule ids.
	var old core.Design
	if err := json.Unmarshal([]byte(`{"pages": [], "status": "draft", "feedback": [], "qa": {"status": "pass", "round": 1, "issues": [{"severity": "minor", "problem": "x"}]}}`), &old); err != nil {
		t.Fatal(err)
	}
	if old.Quality != nil || old.QA.Issues[0].Rule != "" || old.QA.Compared != nil {
		t.Fatalf("old design = %+v", old)
	}
	if MeasureDesign(&core.Project{Design: &old}) != nil {
		t.Fatal("a design without pages is not measured")
	}
	// A page without intent, plan or layout is measured with low confidence.
	q2 := MeasureDesign(qualityProject(core.Page{ID: "PAGE-001", Name: "Lama", Platform: "web"}))
	if q2.Overall != 100 || q2.Confidence != "low" || q2.Decision != "pass" || len(q2.Dimensions) != len(QualityDimensions) {
		t.Fatalf("legacy quality = %+v", q2)
	}
}

func TestQualityGoodPage(t *testing.T) {
	q := MeasureDesign(qualityProject(ticketWorkspace()))
	if q.Overall != 100 || q.Decision != "pass" || q.Confidence != "medium" || len(q.Findings) != 0 || !strings.Contains(q.Recommendation, "tidak ada temuan") {
		t.Fatalf("quality = %+v", q)
	}
	if m := q.Pages[0].Metrics; m["primary_actions"] != 1 || m["nested_surfaces"] != 0 || m["mobile_split_remaining"] != 0 {
		t.Fatalf("metrics = %v", m)
	}
}

func TestQualityMetrics(t *testing.T) {
	for _, tc := range []struct {
		name    string
		edit    func(pg *core.Page)
		metric  string
		rule    string
		dim     string
		blocker bool
	}{
		{"duplicate primary action", func(pg *core.Page) {
			pg.Components = append(pg.Components, core.PageComponent{ID: "send2", Type: "button", Label: "Kirim balasan"})
			pg.Layout.Root.Children = append(pg.Layout.Root.Children, core.LayoutNode{Component: "send2"})
		}, "primary_actions", "duplicate_primary_action", "hierarchy", true},
		{"nested surface", func(pg *core.Page) {
			pg.Layout.Root.Children[1].Surface = "surface.raised"
			pg.Layout.Root.Children[1].Children[1].Surface = "surface.sunken"
		}, "nested_surfaces", "nested_surface", "ai_slop", false},
		{"bad mobile layout: split remains", func(pg *core.Page) {
			pg.Layout.Devices["mobile"] = core.DeviceLayout{Nav: "bottom", Hide: []string{"queue"}}
		}, "mobile_split_remaining", "mobile_split", "responsive", true},
		{"density mismatch", func(pg *core.Page) { pg.Density = "spacious" }, "", "density_mismatch", "density", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pg := ticketWorkspace()
			tc.edit(&pg)
			q := MeasureDesign(qualityProject(pg))
			if tc.metric != "" && q.Pages[0].Metrics[tc.metric] == 0 {
				t.Errorf("metric %s = 0: %v", tc.metric, q.Pages[0].Metrics)
			}
			i := slices.IndexFunc(q.Findings, func(is core.QAIssue) bool { return is.Rule == tc.rule })
			if i < 0 {
				t.Fatalf("missing rule %s in %v", tc.rule, categories(q.Findings))
			}
			if q.Dimensions[tc.dim] >= 100 || q.Overall >= 100 {
				t.Errorf("%s not scored: %v overall %d", tc.dim, q.Dimensions, q.Overall)
			}
			if IsBlocker(q.Findings[i]) != tc.blocker || (q.Decision == "revision_required") != tc.blocker {
				t.Errorf("blocker=%v decision=%s", IsBlocker(q.Findings[i]), q.Decision)
			}
			if tc.blocker && !strings.HasPrefix(q.Recommendation, "Revisi dulu") {
				t.Errorf("recommendation = %q", q.Recommendation)
			}
		})
	}
}

// The quality gate blocks approval on rule blockers, not on minor findings.
func TestQualityGateBlocksApproval(t *testing.T) {
	pg := ticketWorkspace()
	pg.Layout.Devices["mobile"] = core.DeviceLayout{Nav: "bottom", Hide: []string{"queue"}}
	p := qualityProject(pg)
	if !slices.ContainsFunc(Blockers(p), func(b Blocker) bool { return strings.HasPrefix(b.Text, "kualitas: Split") }) {
		t.Fatalf("blockers = %+v", Blockers(p))
	}
	pg = ticketWorkspace()
	pg.Density = "spacious"
	if slices.ContainsFunc(Blockers(qualityProject(pg)), func(b Blocker) bool { return strings.HasPrefix(b.Text, "kualitas:") }) {
		t.Fatal("a density mismatch must not block approval")
	}
}

func TestQualityWithCritic(t *testing.T) {
	p := qualityProject(ticketWorkspace())
	p.Design.QA = &core.DesignQA{Status: "pass", Round: 1, Shots: []core.QAShot{{PageID: "PAGE-001", Device: "mobile"}},
		Issues: []core.QAIssue{{PageID: "PAGE-001", Device: "mobile", Severity: "major", Category: "overflow", Problem: "Kolom SLA terpotong"}}}
	q := MeasureDesign(p)
	if q.Confidence != "high" || q.Decision != "revision_required" || q.Dimensions["responsive"] != 75 || len(q.Reasons) != 1 {
		t.Fatalf("quality = %+v", q)
	}
	// After a change request the check of the new design no longer
	// describes the pages: rules only.
	p.Design.Feedback = []string{"ubah judul"}
	if q := MeasureDesign(p); q.Confidence != "medium" || q.Decision != "pass" {
		t.Fatalf("stale critic used: %+v", q)
	}
	// A running round's findings are about the pages before the patch.
	p.Design.Feedback = nil
	p.Design.QA.Status = "running"
	if q := MeasureDesign(p); q.Decision != "pass" {
		t.Fatalf("running round counted: %+v", q)
	}
}

func TestPatchComparison(t *testing.T) {
	fixed := initialQAProject().Design.Pages[0].Layout.Clone()
	fixed.Devices["mobile"] = core.DeviceLayout{Nav: "bottom", Hide: []string{"queue"}, Swap: map[string]string{"queue": "list"},
		Nodes: map[string]core.NodeChange{"work": {Type: "stack"}, "detail": {Type: "stack"}}}
	for _, tc := range []struct {
		name    string
		edit    func(pg *core.Page)
		verdict string
	}{
		{"improved", func(pg *core.Page) { pg.Layout = fixed.Clone() }, "improved"},
		{"neutral", func(pg *core.Page) { pg.Name = "Tiket masuk" }, "neutral"},
		{"degraded", func(pg *core.Page) {
			pg.Components = append(pg.Components, core.PageComponent{ID: "send2", Type: "button", Label: "Kirim balasan"})
		}, "degraded"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := initialQAProject()
			after := slices.Clone(p.Design.Pages)
			after[0].Layout = after[0].Layout.Clone()
			after[0].Components = slices.Clone(after[0].Components)
			tc.edit(&after[0])
			err := comparePatch(p, after)
			c := p.Design.QA.Compared
			if len(c) != 1 || c[0].Verdict != tc.verdict || c[0].Round != 1 || c[0].Rejected != (tc.verdict == "degraded") || (err != nil) != c[0].Rejected {
				t.Fatalf("compared = %+v err = %v", c, err)
			}
			if tc.verdict == "improved" && (c[0].After.Overall <= c[0].Before.Overall || c[0].After.Blockers >= c[0].Before.Blockers) {
				t.Fatalf("improved scores = %+v", c[0])
			}
		})
	}
}

// A critic patch that lowers the score is rejected: the design stays as it
// was and the check fails.
func TestCriticPatchRegressionRejected(t *testing.T) {
	p := initialQAProject()
	before := mustJSON(p.Design.Pages)
	again, err := ApplyQA(p, `{"status": "fix", "notes": "tambah tombol", "patches": [{"page": "PAGE-001", "operations": [
  {"op": "add", "after": 1, "component": {"id": "send2", "type": "button", "label": "Kirim balasan"}}]}]}`)
	qa := p.Design.QA
	if err != nil || again || qa.Status != "failed" || !strings.Contains(qa.Notes, "memperburuk kualitas") {
		t.Fatalf("again=%v err=%v qa=%+v", again, err, qa)
	}
	if mustJSON(p.Design.Pages) != before {
		t.Fatal("rejected patch changed the design")
	}
	if len(qa.Compared) != 1 || !qa.Compared[0].Rejected || MeasureDesign(p).Compared.Verdict != "degraded" {
		t.Fatalf("compared = %+v", qa.Compared)
	}
}

func TestQualityRecords(t *testing.T) {
	p := initialQAProject()
	p.Design.QA.Status = "pass"
	p.Design.Quality = MeasureDesign(p)
	r := QualityRecords(p)
	if len(r) != 1 || r[0].PageID != "PAGE-001" || r[0].Round != 1 || !slices.Equal(r[0].Devices, []string{"desktop", "mobile"}) ||
		r[0].Decision != "revision_required" || r[0].Overall != p.Design.Quality.Pages[0].Overall || len(r[0].Findings) == 0 {
		t.Fatalf("records = %+v", r)
	}
}
