package techplan

import (
	"fmt"
	"slices"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
)

// RejectError is a plan the validator refused, with every blocking issue.
// Plan is the cleaned candidate: the decisions in it that passed are the
// base of a targeted repair.
type RejectError struct {
	Plan   *core.TechPlan
	Issues []core.TechIssue
	// Merge is set when the plan came from a targeted repair.
	Merge *Merged
}

// Error keeps the long-standing one-line form: the count, the issues grouped
// by kind, then the first issues in full.
func (e *RejectError) Error() string {
	var msgs []string
	for i, is := range e.Issues {
		if i == 8 {
			msgs = append(msgs, fmt.Sprintf("dan %d masalah lain", len(e.Issues)-8))
			break
		}
		msgs = append(msgs, is.Message)
	}
	return fmt.Sprintf("rencana teknologi ditolak, %d masalah (%s): %s", len(e.Issues), GroupSummary(e.Issues), strings.Join(msgs, "; "))
}

// CodeCount is how many issues have one code, and the areas they touch.
type CodeCount struct {
	Code  string
	Count int
	Areas []string
}

// GroupIssues counts the issues per code, the largest group first.
func GroupIssues(issues []core.TechIssue) []CodeCount {
	idx := map[string]int{}
	var out []CodeCount
	for _, is := range issues {
		code := is.Code
		if code == "" {
			code = CodeConflict
		}
		i, ok := idx[code]
		if !ok {
			i = len(out)
			idx[code] = i
			out = append(out, CodeCount{Code: code})
		}
		out[i].Count++
		if is.Area != "" && !slices.Contains(out[i].Areas, is.Area) {
			out[i].Areas = append(out[i].Areas, is.Area)
		}
	}
	slices.SortStableFunc(out, func(a, b CodeCount) int { return b.Count - a.Count })
	return out
}

// GroupSummary is the grouped issues in one line: "alternatif belum lengkap
// 39 di 35 area; teknologi platform belum ditentukan 2: mobile, desktop".
func GroupSummary(issues []core.TechIssue) string {
	var parts []string
	for _, g := range GroupIssues(issues) {
		part := fmt.Sprintf("%s %d", CodeLabel(g.Code), g.Count)
		switch {
		case len(g.Areas) > 0 && len(g.Areas) <= 4:
			part += ": " + strings.Join(g.Areas, ", ")
		case len(g.Areas) > 4:
			part += fmt.Sprintf(" di %d area", len(g.Areas))
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, "; ")
}

// CodeCounts is the number of issues per code.
func CodeCounts(issues []core.TechIssue) map[string]int {
	out := map[string]int{}
	for _, is := range issues {
		code := is.Code
		if code == "" {
			code = CodeConflict
		}
		out[code]++
	}
	return out
}
