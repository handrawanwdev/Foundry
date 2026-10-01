package work

import (
	"fmt"
	"strings"

	"github.com/handrawanw/foundry/internal/core"
)

// TaskPrompt is the brief an agent receives to implement one task.
func TaskPrompt(p *core.Project, t *core.Task) string {
	var b strings.Builder
	fmt.Fprintf(&b, "You are a coding agent working for Foundry on project %q in the current directory.\n", p.Name)
	b.WriteString("Implement ONLY the task below. Write production-quality code and the tests it requires.\n")
	b.WriteString("Follow the project structure. Never delete or restructure existing architecture.\n")
	b.WriteString("Foundry runs the build/test commands itself after you finish; the task is DONE only when they pass and every acceptance criterion is met.\n\n")
	fmt.Fprintf(&b, "TASK %s: %s\nArea: %s  Platform: %s\n\n%s\n\n", t.ID, t.Title, t.Area, t.Platform, t.Description)
	if f := p.Feature(t.FeatureID); f != nil {
		fmt.Fprintf(&b, "FEATURE %s %s: %s\nSub-features:\n", f.ID, f.Name, f.Description)
		for _, s := range f.SubFeatures {
			b.WriteString("- " + s.Name + "\n")
		}
		b.WriteString("\n")
	}
	for _, rid := range t.RequirementIDs {
		for _, r := range p.Requirements {
			if r.ID == rid {
				fmt.Fprintf(&b, "REQUIREMENT %s: %s\n", r.ID, r.Text)
			}
		}
	}
	ui := t.Area == "ui"
	for _, pid := range t.PageIDs {
		if pg := p.Page(pid); pg != nil {
			ui = true
			fmt.Fprintf(&b, "\nPAGE %s %s (%s): %s\nComponents:\n%s\n", pg.ID, pg.Name, pg.Platform, pg.Purpose, mustJSON(pg.Components))
			b.WriteString("Handoff:\n" + mustJSON(handoff(p, *pg, p.Design.Pages)) + "\n")
		}
	}
	if ui {
		if p.Design != nil && p.Design.System != nil {
			b.WriteString("\nDESIGN SYSTEM:\n" + mustJSON(p.Design.System) + "\n")
		}
		b.WriteString("\n" + uiRules)
	}
	b.WriteString("\nACCEPTANCE CRITERIA:\n")
	for i, c := range t.AcceptanceCriteria {
		fmt.Fprintf(&b, "%d. %s\n", i+1, c)
	}
	if len(t.TestRequirements) > 0 {
		b.WriteString("\nTEST REQUIREMENTS:\n- " + strings.Join(t.TestRequirements, "\n- ") + "\n")
	}
	if len(t.Files) > 0 {
		b.WriteString("\nLIKELY FILES:\n- " + strings.Join(t.Files, "\n- ") + "\n")
	}
	if len(t.Notes) > 0 {
		b.WriteString("\nFEEDBACK FROM PREVIOUS ATTEMPTS (fix these):\n- " + strings.Join(t.Notes, "\n- ") + "\n")
	}
	b.WriteString("\n" + structureBrief(p))
	return b.String()
}

// uiRules is how a coding agent builds a designed page.
const uiRules = `UI RULES:
- Build with the DESIGN SYSTEM tokens: map them once onto the project's styling (CSS variables, theme, Tailwind config) and reuse them. No arbitrary colors, font sizes, spacing, radius or shadows where a token fits.
- Reuse existing components (Button, Input, FormField, Card, Table, Modal, Tabs, Badge, Alert, EmptyState, LoadingState, ErrorState, PageHeader). Create one only when none fits, and make it reusable.
- Visual hierarchy follows the handoff "hierarchy", first item most prominent. At most one primary CTA per context (the primary action); secondary actions are visually lower. Never render the same action twice.
- Implement every state in the handoff: loading keeps the layout (skeleton, no jump), empty says what happened, why and what to do, error says what failed and how to recover.
- Layout as the handoff "layout" (Design IR) says: the same frame, nav placement, content width, component tree (stack, row, grid, split, sidebar, center, full, overlay) and per-device changes (hide, order, type, columns, swap); component "#id"s map to the tree. Resolve its semantic tokens (space.*, surface.*, action.*, text.*, status.*) from the design system. Do not invent another layout.
- Responsive as the handoff "responsive" and "layout.devices" say, on desktop, tablet and mobile: no horizontal page overflow on mobile, touch targets at least 44px, important information not hidden.
- Accessibility: semantic HTML, a label for every input, visible focus state, keyboard navigation, WCAG AA contrast, form errors tied to their field, accessible names on icon buttons.
- Do not change APIs or business logic for visuals, add routes beyond the handoff, use mock data where real data exists, or remove existing functionality. No extra features, animations or decoration.
`

// Verdict is a reviewer's judgement of one acceptance criterion (1-based).
type Verdict struct {
	Index    int    `json:"index"`
	Met      bool   `json:"met"`
	Evidence string `json:"evidence"`
}

type reviewOut struct {
	Criteria []Verdict `json:"criteria"`
}

// ReviewPrompt asks the AI to judge acceptance criteria against the changes.
func ReviewPrompt(p *core.Project, t *core.Task, changes string, build *core.BuildRun) string {
	var b strings.Builder
	b.WriteString("You are the reviewer of Foundry. Judge strictly whether each acceptance criterion is met by the code changes.\n")
	b.WriteString("A criterion is met only with concrete evidence in the code (file + what it does). Do not modify files.\n\n")
	fmt.Fprintf(&b, "TASK %s: %s\n%s\n\nACCEPTANCE CRITERIA:\n", t.ID, t.Title, t.Description)
	for i, c := range t.AcceptanceCriteria {
		fmt.Fprintf(&b, "%d. %s\n", i+1, c)
	}
	if build != nil {
		b.WriteString("\nVERIFICATION RESULT (run by Foundry):\n")
		for _, s := range build.Steps {
			fmt.Fprintf(&b, "- %s exit %d\n", s.Command, s.ExitCode)
		}
	}
	b.WriteString("\nCHANGED FILES:\n" + changes + "\n")
	b.WriteString("\nReply with ONLY one JSON object in a ```json block: {\"criteria\": [{\"index\": 1, \"met\": true, \"evidence\": \"\"}]}")
	return b.String()
}

// ParseReview maps a review answer onto the task's criteria. Criteria the
// reviewer did not mention count as not met.
func ParseReview(t *core.Task, raw string) ([]core.Criterion, error) {
	var out reviewOut
	if err := ExtractJSON(raw, &out); err != nil {
		return nil, err
	}
	return MergeCriteria(t, out.Criteria), nil
}

// MergeCriteria is shared by agent review and MCP submit_task.
func MergeCriteria(t *core.Task, in []Verdict) []core.Criterion {
	res := make([]core.Criterion, len(t.AcceptanceCriteria))
	for i, c := range t.AcceptanceCriteria {
		res[i] = core.Criterion{Text: c}
	}
	for _, c := range in {
		if i := c.Index - 1; i >= 0 && i < len(res) {
			res[i].Met, res[i].Evidence = c.Met, c.Evidence
		}
	}
	return res
}

func AllMet(cs []core.Criterion) bool {
	if len(cs) == 0 {
		return false
	}
	for _, c := range cs {
		if !c.Met {
			return false
		}
	}
	return true
}
