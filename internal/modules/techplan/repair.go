package techplan

import (
	"fmt"
	"slices"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
)

// A targeted repair rewrites only what the validator named. Everything else
// of the refused plan was valid and is kept by Foundry, not by the AI: the
// answer is merged into the draft, never trusted to resend it intact.

// Fix is what a repair answer may carry: complete decisions for the areas to
// repair, and the plan sections the issues named.
type Fix struct {
	Items []core.TechItem
	// Alternatives are complete alternative lists per area, for an area
	// whose only problem is an incomplete alternative.
	Alternatives                 map[string][]core.TechAlternative
	Approach, Summary, Rationale string
	Context                      *core.TechContext
	Compat                       []core.TechCompat
	Evaluation                   *core.TechEvaluation
	Consistency                  []core.TechCheck
}

// Scope is what a set of issues allows a repair to rewrite.
type Scope struct {
	// Areas are the decisions to rewrite or add, in plan order then issue
	// order. Missing are those of them the draft does not have yet.
	Areas   []string
	Missing []string
	// Locked are the areas whose issues are about completeness only: their
	// technology stays what it is.
	Locked map[string]bool
	// AltOnly are the areas whose only problems are incomplete
	// alternatives: a repair sends their alternative list, not the item.
	AltOnly map[string]bool
	// Plan sections named by the issues.
	Meta, Context, Compat, Evaluation, Consistency bool
	// Full: the draft has nothing to keep; the answer is a whole plan.
	Full bool
}

// completeness are the codes about a missing explanation, not the choice.
var completeness = []string{CodeAltIncomplete, CodeRequiredField, CodeUnknownDepend, CodeInvalidGroup}

// RepairScope derives what may change from the draft and its issues.
func RepairScope(draft *core.TechPlan, issues []core.TechIssue) Scope {
	sc := Scope{Locked: map[string]bool{}, AltOnly: map[string]bool{}}
	if draft == nil || len(draft.Items) == 0 {
		sc.Full = true
		return sc
	}
	choice := map[string]bool{}
	other := map[string]bool{}
	for _, is := range issues {
		switch is.Code {
		case CodeEmptyPlan:
			sc.Full = true
		case CodePlanMeta:
			sc.Meta = true
		case CodeContext:
			sc.Context = true
		case CodeCompatibility:
			sc.Compat = true
		case CodeEvaluation:
			sc.Evaluation = true
		case CodeConsistency:
			sc.Consistency = true
		}
		if is.Area == "" {
			continue
		}
		if !slices.Contains(sc.Areas, is.Area) {
			sc.Areas = append(sc.Areas, is.Area)
		}
		if !slices.Contains(completeness, is.Code) {
			choice[is.Area] = true
		}
		if is.Code != CodeAltIncomplete {
			other[is.Area] = true
		}
	}
	for _, a := range sc.Areas {
		it := draft.Item(a)
		if it == nil {
			sc.Missing = append(sc.Missing, a)
			continue
		}
		if !choice[a] && it.Technology != "" {
			sc.Locked[a] = true
		}
		if !other[a] {
			sc.AltOnly[a] = true
		}
	}
	return sc
}

// Merged is what a merge did.
type Merged struct {
	Repaired []string // areas rewritten or added
	Sections []string // plan sections replaced
	Ignored  []string // changes outside the scope, with the reason
}

// Merge applies fix to a copy of draft within the scope of issues. Areas and
// sections the issues do not name stay exactly as they were in the draft.
func Merge(draft *core.TechPlan, issues []core.TechIssue, fix Fix) (*core.TechPlan, Merged) {
	sc := RepairScope(draft, issues)
	var res Merged
	if sc.Full {
		plan := &core.TechPlan{Approach: fix.Approach, Summary: fix.Summary, Rationale: fix.Rationale, Items: fix.Items, Compatibility: fix.Compat, Consistency: fix.Consistency}
		if fix.Context != nil {
			plan.Context = *fix.Context
		}
		if fix.Evaluation != nil {
			plan.Evaluation = *fix.Evaluation
		}
		for _, it := range fix.Items {
			res.Repaired = append(res.Repaired, AreaID(it.Area))
		}
		res.Sections = []string{"plan"}
		return plan, res
	}
	plan := draft.Clone()
	plan.History, plan.Analysis = nil, core.TechAnalysis{}
	seen := map[string]bool{}
	for _, it := range fix.Items {
		it.Area = AreaID(it.Area)
		if it.Area == "" || seen[it.Area] {
			continue
		}
		seen[it.Area] = true
		old := plan.Item(it.Area)
		if !slices.Contains(sc.Areas, it.Area) {
			if old == nil || !strings.EqualFold(clean(old.Technology, maxName), clean(it.Technology, maxName)) {
				res.Ignored = append(res.Ignored, fmt.Sprintf("%s: tidak bermasalah, perubahan diabaikan", it.Area))
			}
			continue
		}
		if sc.Locked[it.Area] && old != nil && !strings.EqualFold(clean(old.Technology, maxName), clean(it.Technology, maxName)) {
			res.Ignored = append(res.Ignored, fmt.Sprintf("%s: teknologi %q diganti %q padahal masalahnya hanya kelengkapan; perubahan diabaikan", it.Area, old.Technology, it.Technology))
			continue
		}
		if old != nil {
			if it.Group == "" {
				it.Group = old.Group
			}
			*old = it
			// One decision per area: a duplicate the issues named goes.
			first := true
			plan.Items = slices.DeleteFunc(plan.Items, func(x core.TechItem) bool {
				if x.Area != it.Area {
					return false
				}
				if first {
					first = false
					return false
				}
				return true
			})
		} else {
			plan.Items = append(plan.Items, it)
		}
		res.Repaired = append(res.Repaired, it.Area)
	}
	for area, alts := range fix.Alternatives {
		area = AreaID(area)
		old := plan.Item(area)
		switch {
		case seen[area]:
			continue // the complete item already carries its alternatives
		case old == nil || !slices.Contains(sc.Areas, area):
			res.Ignored = append(res.Ignored, fmt.Sprintf("%s: alternatif untuk area yang tidak bermasalah diabaikan", area))
			continue
		}
		old.Alternatives = alts
		res.Repaired = append(res.Repaired, area)
	}
	slices.Sort(res.Repaired)
	if sc.Meta {
		if fix.Approach != "" {
			plan.Approach = fix.Approach
		}
		if fix.Summary != "" {
			plan.Summary = fix.Summary
		}
		if fix.Rationale != "" {
			plan.Rationale = fix.Rationale
		}
		res.Sections = append(res.Sections, "meta")
	}
	if sc.Context && fix.Context != nil {
		plan.Context = *fix.Context
		res.Sections = append(res.Sections, "context")
	}
	if sc.Compat && len(fix.Compat) > 0 {
		plan.Compatibility = fix.Compat
		res.Sections = append(res.Sections, "compatibility")
	}
	if sc.Evaluation && fix.Evaluation != nil {
		mergeEvaluation(&plan.Evaluation, fix.Evaluation)
		res.Sections = append(res.Sections, "evaluation")
	}
	if sc.Consistency && len(fix.Consistency) > 0 {
		for _, k := range fix.Consistency {
			k.Area = AreaID(k.Area)
			if i := slices.IndexFunc(plan.Consistency, func(x core.TechCheck) bool { return x.Area == k.Area }); i >= 0 {
				plan.Consistency[i] = k
			} else {
				plan.Consistency = append(plan.Consistency, k)
			}
		}
		res.Sections = append(res.Sections, "consistency")
	}
	return plan, res
}

// mergeEvaluation replaces the criteria the fix scores and keeps the others.
func mergeEvaluation(ev *core.TechEvaluation, fix *core.TechEvaluation) {
	if fix.Summary != "" {
		ev.Summary = fix.Summary
	}
	for _, k := range fix.Criteria {
		k.Name = AreaID(k.Name)
		if i := slices.IndexFunc(ev.Criteria, func(x core.TechCriterion) bool { return x.Name == k.Name }); i >= 0 {
			ev.Criteria[i] = k
		} else {
			ev.Criteria = append(ev.Criteria, k)
		}
	}
	if len(fix.Findings) > 0 {
		ev.Findings = fix.Findings
	}
}
