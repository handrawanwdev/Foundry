package work

import (
	"fmt"
	"slices"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
)

// Autonomous correction loop: evaluate → identify issues → prioritize → fix → re-evaluate
// This module generates correction commands from visual findings and manages the correction loop.

// VisualFixCommand generates a correction command from visual findings for a specific page.
// The command includes detailed instructions based on the findings severity and type.
func VisualFixCommand(p *core.Project, pageID string, findings []core.VisualFinding) *core.Command {
	if len(findings) == 0 {
		return nil
	}

	// Sort findings by severity (critical first)
	sortedFindings := make([]core.VisualFinding, len(findings))
	copy(sortedFindings, findings)
	slices.SortFunc(sortedFindings, func(a, b core.VisualFinding) int {
		severityOrder := map[string]int{
			core.SeverityCritical: 0,
			core.SeverityHigh:     1,
			core.SeverityMedium:   2,
			core.SeverityLow:      3,
		}
		return severityOrder[a.Severity] - severityOrder[b.Severity]
	})

	// Build instruction text
	var instructions strings.Builder
	instructions.WriteString(fmt.Sprintf("# Perbaiki masalah visual pada halaman %s\n\n", pageID))
	instructions.WriteString("## Temuan yang harus diperbaiki:\n\n")

	for i, finding := range sortedFindings {
		instructions.WriteString(fmt.Sprintf("### %d. %s (%s)\n", i+1, finding.Criterion, finding.Severity))
		instructions.WriteString(fmt.Sprintf("**Masalah:** %s\n", finding.Message))
		if finding.Evidence != "" {
			instructions.WriteString(fmt.Sprintf("**Bukti:** %s\n", finding.Evidence))
		}
		if finding.Fix != "" {
			instructions.WriteString(fmt.Sprintf("**Solusi:** %s\n", finding.Fix))
		}
		instructions.WriteString("\n")
	}

	instructions.WriteString("## Instruksi:\n\n")
	instructions.WriteString("1. Perbaiki semua masalah di atas sesuai solusi yang disarankan\n")
	instructions.WriteString("2. Pastikan perbaikan mengikuti design-rules dan layout-spec\n")
	instructions.WriteString("3. Test di semua viewport (desktop, tablet, mobile)\n")
	instructions.WriteString("4. Capture screenshot baru untuk verifikasi\n")
	instructions.WriteString("5. Submit visual review baru menggunakan `record_visual_review`\n")

	// Determine depends_on (visual verification commands)
	dependsOn := []string{"CMD-SUBMIT-VISUAL-REVIEWS"}

	return &core.Command{
		ID:           fmt.Sprintf("CMD-FIX-VISUAL-%s", strings.ToUpper(pageID)),
		Name:         fmt.Sprintf("Perbaiki visual: %s", pageID),
		Objective:    fmt.Sprintf("Perbaiki %d masalah visual pada halaman %s", len(findings), pageID),
		Instructions: instructions.String(),
		DependsOn:    dependsOn,
		Artifacts: []core.ArtifactRef{
			{ID: core.ArtifactDesignRules},
			{ID: core.ArtifactLayoutSpec},
		},
		Covers:     []string{},
		Outputs:    []string{"Kode yang diperbaiki", "Screenshot baru", "Visual review submission"},
		Acceptance: []string{"Semua temuan visual diperbaiki", "Screenshot baru di-submit", "Visual review pass"},
		Constraints: []string{
			"Ikuti design-rules dan layout-spec",
			"Jangan ubah struktur halaman yang tidak terkait",
			"Test di semua viewport",
		},
		Status: core.CommandDraft,
	}
}

// GenerateVisualFixCommands generates correction commands for all pages with visual findings.
// Returns a list of commands, one per page that has findings.
func GenerateVisualFixCommands(p *core.Project) []core.Command {
	if p.Visual == nil || len(p.Visual.Pages) == 0 {
		return nil
	}

	var commands []core.Command

	for pageID, audit := range p.Visual.Pages {
		if audit.Status == core.VisualPass {
			continue // Skip pages that already pass
		}

		if len(audit.Findings) == 0 {
			continue // Skip pages with no findings
		}

		cmd := VisualFixCommand(p, pageID, audit.Findings)
		if cmd != nil {
			commands = append(commands, *cmd)
		}
	}

	return commands
}

// CorrectionLoopState tracks the state of the autonomous correction loop.
type CorrectionLoopState struct {
	Iteration      int      `json:"iteration"`
	MaxIterations  int      `json:"max_iterations"`
	PagesFixed     []string `json:"pages_fixed"`
	PagesRemaining []string `json:"pages_remaining"`
	TotalFindings  int      `json:"total_findings"`
	HighSeverity   int      `json:"high_severity"`
	LastProgress   int      `json:"last_progress"`
	StalledCount   int      `json:"stalled_count"`
}

// NewCorrectionLoopState creates a new correction loop state.
func NewCorrectionLoopState(maxIterations int) *CorrectionLoopState {
	return &CorrectionLoopState{
		Iteration:     0,
		MaxIterations: maxIterations,
	}
}

// Update updates the correction loop state based on current visual audit.
// Returns true if progress was made, false if stalled.
func (s *CorrectionLoopState) Update(p *core.Project) bool {
	s.Iteration++

	// Count current findings
	var totalFindings, highSeverity int
	var pagesRemaining []string

	if p.Visual != nil {
		for pageID, audit := range p.Visual.Pages {
			if audit.Status != core.VisualPass {
				pagesRemaining = append(pagesRemaining, pageID)
				totalFindings += len(audit.Findings)
				for _, f := range audit.Findings {
					if f.Severity == core.SeverityHigh || f.Severity == core.SeverityCritical {
						highSeverity++
					}
				}
			}
		}
	}

	// First iteration: establish baseline, don't count as stall
	if s.Iteration == 1 {
		s.TotalFindings = totalFindings
		s.HighSeverity = highSeverity
		s.PagesRemaining = pagesRemaining
		return true // First update always returns true (baseline established)
	}

	// Check if progress was made
	progress := s.TotalFindings - totalFindings
	s.LastProgress = progress
	s.TotalFindings = totalFindings
	s.HighSeverity = highSeverity
	s.PagesRemaining = pagesRemaining

	if progress > 0 {
		s.StalledCount = 0
		return true
	}

	s.StalledCount++
	return false
}

// ShouldContinue returns true if the correction loop should continue.
func (s *CorrectionLoopState) ShouldContinue() bool {
	// Stop if max iterations reached
	if s.Iteration >= s.MaxIterations {
		return false
	}

	// Stop if no findings remain
	if s.TotalFindings == 0 {
		return false
	}

	// Stop if stalled for 2 consecutive iterations
	if s.StalledCount >= 2 {
		return false
	}

	return true
}

// Status returns a human-readable status of the correction loop.
func (s *CorrectionLoopState) Status() string {
	if s.TotalFindings == 0 {
		return "Semua halaman lulus visual review"
	}

	if s.StalledCount >= 2 {
		return fmt.Sprintf("Loop stagnan setelah %d iterasi (%d temuan tersisa)", s.Iteration, s.TotalFindings)
	}

	if s.Iteration >= s.MaxIterations {
		return fmt.Sprintf("Maksimum iterasi tercapai (%d iterasi, %d temuan tersisa)", s.Iteration, s.TotalFindings)
	}

	return fmt.Sprintf("Iterasi %d/%d: %d temuan tersisa (%d high severity)",
		s.Iteration, s.MaxIterations, s.TotalFindings, s.HighSeverity)
}

// PrioritizeFindings separates findings into critical/high and medium/low priority.
// Returns two slices: high priority and low priority findings.
func PrioritizeFindings(findings []core.VisualFinding) (highPriority, lowPriority []core.VisualFinding) {
	for _, f := range findings {
		if f.Severity == core.SeverityCritical || f.Severity == core.SeverityHigh {
			highPriority = append(highPriority, f)
		} else {
			lowPriority = append(lowPriority, f)
		}
	}
	return
}

// GroupFindingsByPage groups visual findings by page ID.
func GroupFindingsByPage(p *core.Project) map[string][]core.VisualFinding {
	if p.Visual == nil {
		return nil
	}

	groups := make(map[string][]core.VisualFinding)
	for pageID, audit := range p.Visual.Pages {
		if len(audit.Findings) > 0 {
			groups[pageID] = audit.Findings
		}
	}

	return groups
}
