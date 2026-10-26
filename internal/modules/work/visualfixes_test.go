package work

import (
	"strings"
	"testing"
	"time"

	"github.com/handrawanwdev/foundry/internal/core"
)

func TestVisualFixCommand(t *testing.T) {
	p := &core.Project{
		Name: "Test Project",
		UI: &core.UIDesign{
			Rules: &core.DesignRules{
				Tokens: core.DesignTokens{
					Spacing: []core.Token{
						{Name: "space.md", Value: "16px"},
					},
				},
			},
			Layout: &core.LayoutSpec{
				Pages: []core.LayoutPage{
					{
						ID:       "dashboard",
						Name:     "Dashboard",
						Platform: "web",
						Route:    "/dashboard",
						Pattern:  "dashboard",
					},
				},
			},
		},
	}

	findings := []core.VisualFinding{
		{
			Page:      "dashboard",
			Criterion: "contrast",
			Severity:  core.SeverityHigh,
			Message:   "Contrast ratio 3.5:1 below 4.5:1",
			Evidence:  "Text on button",
			Fix:       "Increase contrast to meet WCAG AA",
		},
		{
			Page:      "dashboard",
			Criterion: "spacing",
			Severity:  core.SeverityLow,
			Message:   "Spacing 12px not in token set",
			Evidence:  "Card padding",
			Fix:       "Use space.md (16px) instead",
		},
	}

	cmd := VisualFixCommand(p, "dashboard", findings)

	if cmd == nil {
		t.Fatal("Expected command, got nil")
	}

	if cmd.ID != "CMD-FIX-VISUAL-DASHBOARD" {
		t.Errorf("Expected ID CMD-FIX-VISUAL-DASHBOARD, got %s", cmd.ID)
	}

	if cmd.Name != "Perbaiki visual: dashboard" {
		t.Errorf("Expected name 'Perbaiki visual: dashboard', got %s", cmd.Name)
	}

	if len(cmd.Artifacts) != 2 {
		t.Errorf("Expected 2 artifacts, got %d", len(cmd.Artifacts))
	}

	if !contains(cmd.DependsOn, "CMD-SUBMIT-VISUAL-REVIEWS") {
		t.Error("Expected dependency on CMD-SUBMIT-VISUAL-REVIEWS")
	}

	// Check that high severity finding comes first in instructions
	if cmd.Instructions == "" {
		t.Error("Expected instructions to be non-empty")
	}

	// Verify contrast finding (high severity) appears before spacing (low severity)
	contrastIdx := indexOf(cmd.Instructions, "contrast")
	spacingIdx := indexOf(cmd.Instructions, "spacing")
	if contrastIdx == -1 || spacingIdx == -1 {
		t.Error("Expected both findings in instructions")
	}
	if contrastIdx > spacingIdx {
		t.Error("Expected high severity finding to appear before low severity")
	}
}

func TestVisualFixCommandEmptyFindings(t *testing.T) {
	p := &core.Project{}
	cmd := VisualFixCommand(p, "dashboard", []core.VisualFinding{})
	if cmd != nil {
		t.Error("Expected nil command for empty findings")
	}
}

func TestGenerateVisualFixCommands(t *testing.T) {
	p := &core.Project{
		UI: &core.UIDesign{
			Layout: &core.LayoutSpec{
				Pages: []core.LayoutPage{
					{ID: "dashboard", Name: "Dashboard", Platform: "web", Route: "/dashboard"},
					{ID: "profile", Name: "Profile", Platform: "web", Route: "/profile"},
				},
			},
		},
		Visual: &core.VisualAudit{
			Pages: map[string]core.PageAudit{
				"dashboard": {
					Page:   "dashboard",
					Status: core.VisualFail,
					Findings: []core.VisualFinding{
						{Page: "dashboard", Criterion: "contrast", Severity: core.SeverityHigh, Message: "Low contrast"},
					},
				},
				"profile": {
					Page:     "profile",
					Status:   core.VisualPass,
					Findings: []core.VisualFinding{}, // No findings
				},
			},
		},
	}

	cmds := GenerateVisualFixCommands(p)

	if len(cmds) != 1 {
		t.Errorf("Expected 1 command (only for failing page), got %d", len(cmds))
	}

	if len(cmds) > 0 && cmds[0].ID != "CMD-FIX-VISUAL-DASHBOARD" {
		t.Errorf("Expected command for dashboard, got %s", cmds[0].ID)
	}
}

func TestGenerateVisualFixCommandsNoVisual(t *testing.T) {
	p := &core.Project{}
	cmds := GenerateVisualFixCommands(p)
	if cmds != nil {
		t.Error("Expected nil commands when no visual audit exists")
	}
}

func TestCorrectionLoopState(t *testing.T) {
	state := NewCorrectionLoopState(5)

	if state.Iteration != 0 {
		t.Errorf("Expected initial iteration 0, got %d", state.Iteration)
	}

	if state.MaxIterations != 5 {
		t.Errorf("Expected max iterations 5, got %d", state.MaxIterations)
	}
}

func TestCorrectionLoopUpdate(t *testing.T) {
	state := NewCorrectionLoopState(5)

	p := &core.Project{
		Visual: &core.VisualAudit{
			Pages: map[string]core.PageAudit{
				"dashboard": {
					Page:   "dashboard",
					Status: core.VisualFail,
					Findings: []core.VisualFinding{
						{Severity: core.SeverityHigh},
						{Severity: core.SeverityLow},
					},
				},
			},
		},
	}

	// First update - establishes baseline, always returns true
	progress := state.Update(p)
	if !progress {
		t.Error("Expected progress=true on first update (baseline establishment)")
	}

	if state.TotalFindings != 2 {
		t.Errorf("Expected 2 total findings, got %d", state.TotalFindings)
	}

	if state.HighSeverity != 1 {
		t.Errorf("Expected 1 high severity finding, got %d", state.HighSeverity)
	}

	if state.Iteration != 1 {
		t.Errorf("Expected iteration 1, got %d", state.Iteration)
	}

	// Simulate no progress (stall)
	progress = state.Update(p)
	if progress {
		t.Error("Expected no progress when findings remain the same")
	}

	if state.StalledCount != 1 {
		t.Errorf("Expected stalled count 1, got %d", state.StalledCount)
	}

	// Simulate progress: reduce findings
	p.Visual.Pages["dashboard"] = core.PageAudit{
		Page:   "dashboard",
		Status: core.VisualFail,
		Findings: []core.VisualFinding{
			{Severity: core.SeverityLow},
		},
	}

	progress = state.Update(p)
	if !progress {
		t.Error("Expected progress after reducing findings")
	}

	if state.TotalFindings != 1 {
		t.Errorf("Expected 1 total finding after progress, got %d", state.TotalFindings)
	}

	if state.LastProgress != 1 {
		t.Errorf("Expected last progress 1, got %d", state.LastProgress)
	}

	if state.StalledCount != 0 {
		t.Errorf("Expected stalled count 0 after progress, got %d", state.StalledCount)
	}
}

func TestCorrectionLoopShouldContinue(t *testing.T) {
	state := NewCorrectionLoopState(3)

	p := &core.Project{
		Visual: &core.VisualAudit{
			Pages: map[string]core.PageAudit{
				"dashboard": {
					Page:   "dashboard",
					Status: core.VisualFail,
					Findings: []core.VisualFinding{
						{Severity: core.SeverityHigh},
					},
				},
			},
		},
	}

	// Should continue initially
	state.Update(p)
	if !state.ShouldContinue() {
		t.Error("Expected to continue on first iteration")
	}

	// Simulate no progress (stall)
	state.Update(p)
	if !state.ShouldContinue() {
		t.Error("Expected to continue after first stall")
	}

	state.Update(p)
	if state.ShouldContinue() {
		t.Error("Expected to stop after 2 consecutive stalls")
	}
}

func TestCorrectionLoopMaxIterations(t *testing.T) {
	state := NewCorrectionLoopState(2)

	p := &core.Project{
		Visual: &core.VisualAudit{
			Pages: map[string]core.PageAudit{
				"dashboard": {
					Page:   "dashboard",
					Status: core.VisualFail,
					Findings: []core.VisualFinding{
						{Severity: core.SeverityHigh},
					},
				},
			},
		},
	}

	state.Update(p)
	if !state.ShouldContinue() {
		t.Error("Expected to continue on iteration 1")
	}

	state.Update(p)
	if state.ShouldContinue() {
		t.Error("Expected to stop at max iterations")
	}
}

func TestCorrectionLoopNoFindings(t *testing.T) {
	state := NewCorrectionLoopState(5)

	p := &core.Project{
		Visual: &core.VisualAudit{
			Pages: map[string]core.PageAudit{
				"dashboard": {
					Page:     "dashboard",
					Status:   core.VisualPass,
					Findings: []core.VisualFinding{},
				},
			},
		},
	}

	state.Update(p)
	if state.ShouldContinue() {
		t.Error("Expected to stop when no findings remain")
	}
}

func TestCorrectionLoopStatus(t *testing.T) {
	state := NewCorrectionLoopState(5)

	p := &core.Project{
		Visual: &core.VisualAudit{
			Pages: map[string]core.PageAudit{
				"dashboard": {
					Page:   "dashboard",
					Status: core.VisualFail,
					Findings: []core.VisualFinding{
						{Severity: core.SeverityHigh},
						{Severity: core.SeverityLow},
					},
				},
			},
		},
	}

	state.Update(p)
	status := state.Status()

	if status == "" {
		t.Error("Expected non-empty status")
	}

	// Check status contains key information
	if !strings.Contains(status, "Iterasi") {
		t.Error("Expected status to contain 'Iterasi'")
	}
}

func TestPrioritizeFindings(t *testing.T) {
	findings := []core.VisualFinding{
		{Severity: core.SeverityLow},
		{Severity: core.SeverityCritical},
		{Severity: core.SeverityMedium},
		{Severity: core.SeverityHigh},
	}

	high, low := PrioritizeFindings(findings)

	if len(high) != 2 {
		t.Errorf("Expected 2 high priority findings, got %d", len(high))
	}

	if len(low) != 2 {
		t.Errorf("Expected 2 low priority findings, got %d", len(low))
	}

	// Verify high priority contains critical and high
	if high[0].Severity != core.SeverityCritical {
		t.Error("Expected first high priority to be critical")
	}
	if high[1].Severity != core.SeverityHigh {
		t.Error("Expected second high priority to be high")
	}
}

func TestGroupFindingsByPage(t *testing.T) {
	p := &core.Project{
		Visual: &core.VisualAudit{
			Pages: map[string]core.PageAudit{
				"dashboard": {
					Page:     "dashboard",
					Findings: []core.VisualFinding{{Page: "dashboard"}},
				},
				"profile": {
					Page:     "profile",
					Findings: []core.VisualFinding{{Page: "profile"}},
				},
				"settings": {
					Page:     "settings",
					Findings: []core.VisualFinding{}, // No findings
				},
			},
		},
	}

	groups := GroupFindingsByPage(p)

	if len(groups) != 2 {
		t.Errorf("Expected 2 pages with findings, got %d", len(groups))
	}

	if _, ok := groups["dashboard"]; !ok {
		t.Error("Expected dashboard in groups")
	}

	if _, ok := groups["profile"]; !ok {
		t.Error("Expected profile in groups")
	}

	if _, ok := groups["settings"]; ok {
		t.Error("Expected settings to be excluded (no findings)")
	}
}

// Helper functions
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// Unused import prevention
var _ = time.Now
