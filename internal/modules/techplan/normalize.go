// Package techplan validates and manages the AI's technology plan: the
// technology decisions of a product with their reasoning, alternatives,
// compatibility review and evaluation. The AI is free to choose any
// technology; this package only checks that the plan is complete, honest
// and consistent with the rest of the project, and computes what the AI must
// not be trusted with (scores, staleness, cross-artifact conflicts).
package techplan

import (
	"regexp"
	"slices"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/stack"
)

// Limits that keep a plan readable and bounded.
const (
	maxItems   = 60
	maxAlts    = 4
	maxList    = 12
	maxText    = 700
	maxLongTxt = 1400
	maxName    = 120
)

var slug = regexp.MustCompile(`[^a-z0-9]+`)

// AreaID turns a free area name into its id: lower case, words joined with
// "_" ("CI/CD" is ci_cd).
func AreaID(s string) string {
	return strings.Trim(slug.ReplaceAllString(strings.ToLower(strings.TrimSpace(s)), "_"), "_")
}

func clean(s string, limit int) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > limit {
		s = strings.TrimSpace(string(r[:limit])) + "…"
	}
	return s
}

func cleanList(in []string, limit int) []string {
	var out []string
	for _, s := range in {
		if s = clean(s, limit); s != "" && !slices.Contains(out, s) {
			out = append(out, s)
		}
		if len(out) == maxList {
			break
		}
	}
	return out
}

func cleanAreas(in []string) []string {
	var out []string
	for _, s := range in {
		if a := AreaID(s); a != "" && !slices.Contains(out, a) {
			out = append(out, a)
		}
	}
	return out
}

func lower(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// Normalize cleans a plan the AI wrote: trimmed texts, area ids, groups,
// canonical architecture and repository values, computed catalog flags and
// score. It never invents content; what is missing stays missing for the
// validation to report.
func Normalize(plan *core.TechPlan) {
	plan.Approach, plan.Summary = clean(plan.Approach, maxName), clean(plan.Summary, maxLongTxt)
	plan.Rationale = clean(plan.Rationale, maxLongTxt)

	c := &plan.Context
	c.Summary = clean(c.Summary, maxLongTxt)
	c.Unknowns, c.Assumptions = cleanList(c.Unknowns, maxText), cleanList(c.Assumptions, maxText)
	var facts []core.TechFact
	for _, f := range c.Facts {
		f.Category, f.Statement, f.Source = lower(f.Category), clean(f.Statement, maxText), lower(f.Source)
		if !slices.Contains(core.TechFactCategories, f.Category) {
			f.Category = "non_functional"
		}
		if f.Statement != "" && len(facts) < 40 {
			facts = append(facts, f)
		}
	}
	c.Facts = facts

	items := make([]core.TechItem, 0, len(plan.Items))
	for _, it := range plan.Items {
		normalizeItem(&it)
		if it.Area != "" {
			items = append(items, it)
		}
		if len(items) == maxItems {
			break
		}
	}
	slices.SortStableFunc(items, func(a, b core.TechItem) int { return rank(a) - rank(b) })
	plan.Items = items

	var cons []core.TechConstraint
	for _, k := range plan.Constraints {
		k.Areas, k.Rule, k.Reason = cleanAreas(k.Areas), clean(k.Rule, maxText), clean(k.Reason, maxText)
		if k.Rule != "" {
			cons = append(cons, k)
		}
	}
	plan.Constraints = cons

	var compat []core.TechCompat
	for _, x := range plan.Compatibility {
		x.Area, x.Status, x.Source = AreaID(x.Area), lower(x.Status), core.SourceAI
		x.Between, x.Note, x.Basis = cleanAreas(x.Between), clean(x.Note, maxText), clean(x.Basis, maxText)
		switch x.Status {
		case core.CompatCompatible:
			// A claim of compatibility needs a basis; without one it is only
			// a belief.
			if x.Basis == "" {
				x.Status = core.CompatUnverified
			}
		case core.CompatUnverified, core.CompatConflict, core.CompatResolved, core.CompatNotApplicable:
		default:
			x.Status = core.CompatUnverified
		}
		if x.Area != "" {
			compat = append(compat, x)
		}
	}
	plan.Compatibility = compat

	normalizeEvaluation(&plan.Evaluation)

	var checks []core.TechCheck
	seen := map[string]bool{}
	for _, k := range plan.Consistency {
		k.Area, k.Status, k.Note, k.Source = AreaID(k.Area), lower(k.Status), clean(k.Note, maxText), core.SourceAI
		if k.Status != core.CheckResolved && k.Status != core.CheckConflict {
			k.Status = core.CheckConsistent
		}
		if k.Area != "" && !seen[k.Area] {
			seen[k.Area] = true
			checks = append(checks, k)
		}
	}
	plan.Consistency = checks
}

func normalizeItem(it *core.TechItem) {
	it.Area = AreaID(it.Area)
	it.Technology = clean(it.Technology, maxName)
	if strings.EqualFold(it.Technology, core.TechNone) || it.Technology == "-" || strings.EqualFold(it.Technology, "none") {
		it.Technology = core.TechNone
	}
	// closed values the project structure is derived from
	switch it.Area {
	case core.TechArchitecture:
		it.Technology = canonical(it.Technology, core.ArchSimple, core.ArchModular, core.ArchMicroservices)
	case core.TechRepository:
		it.Technology = canonical(it.Technology, core.RepoSimple, core.RepoMonorepo)
	}
	if g := core.TechAreaGroup(it.Area); g != "" {
		it.Group = g
	} else {
		it.Group = lower(it.Group)
	}
	it.Version, it.Language, it.Runtime = clean(it.Version, 60), clean(it.Language, 40), clean(it.Runtime, 80)
	if it.Version == "" {
		it.VersionVerified = false
	}
	if it.Language == "" && !it.None() {
		it.Language = stack.LanguageOf(it.Area, it.Technology)
	}
	it.Purpose, it.Reasoning = clean(it.Purpose, maxText), clean(it.Reasoning, maxLongTxt)
	it.Integration, it.Operational = clean(it.Integration, maxText), clean(it.Operational, maxText)
	it.Fulfills = cleanList(it.Fulfills, maxName)
	it.Pros, it.Cons, it.Tradeoffs = cleanList(it.Pros, maxText), cleanList(it.Cons, maxText), cleanList(it.Tradeoffs, maxText)
	it.Assumptions = cleanList(it.Assumptions, maxText)
	it.DependsOn = cleanAreas(it.DependsOn)
	it.DependsOn = slices.DeleteFunc(it.DependsOn, func(a string) bool { return a == it.Area })
	it.Driver, it.Confidence = lower(it.Driver), lower(it.Confidence)
	var alts []core.TechAlternative
	for _, a := range it.Alternatives {
		a.Technology, a.Why, a.Difference = clean(a.Technology, maxName), clean(a.Why, maxText), clean(a.Difference, maxText)
		a.Pros, a.Cons = cleanList(a.Pros, maxText), cleanList(a.Cons, maxText)
		a.ArchitectureImpact, a.MaintenanceImpact, a.CostImpact = clean(a.ArchitectureImpact, maxText), clean(a.MaintenanceImpact, maxText), clean(a.CostImpact, maxText)
		if a.Technology != "" && !strings.EqualFold(a.Technology, it.Technology) && len(alts) < maxAlts {
			alts = append(alts, a)
		}
	}
	it.Alternatives = alts
	it.InCatalog = stack.Known(it.Area, it.Technology)
	it.UserChosen = clean(it.UserChosen, maxText)
}

func canonical(v string, allowed ...string) string {
	for _, a := range allowed {
		if strings.EqualFold(v, a) {
			return a
		}
	}
	return v
}

// rank orders the items: Foundry's core areas in their usual order, then the
// other areas by group and name.
func rank(it core.TechItem) int {
	if i := slices.Index(core.TechAreas, it.Area); i >= 0 {
		return i
	}
	g := slices.Index(core.TechPlanGroups, it.Group)
	if g < 0 {
		g = len(core.TechPlanGroups)
	}
	return 1000 + g*100
}

func normalizeEvaluation(ev *core.TechEvaluation) {
	ev.Summary = clean(ev.Summary, maxLongTxt)
	var crit []core.TechCriterion
	seen := map[string]bool{}
	for _, c := range ev.Criteria {
		c.Name = AreaID(c.Name)
		c.Reason = clean(c.Reason, maxText)
		c.Evidence = cleanList(c.Evidence, maxName)
		if c.Name != "" && !seen[c.Name] {
			seen[c.Name] = true
			crit = append(crit, c)
		}
	}
	slices.SortStableFunc(crit, func(a, b core.TechCriterion) int {
		return slices.Index(core.TechCriteria, a.Name) - slices.Index(core.TechCriteria, b.Name)
	})
	ev.Criteria = crit
	var fs []core.TechFinding
	for _, f := range ev.Findings {
		f.Category, f.Severity, f.Source = lower(f.Category), lower(f.Severity), core.SourceAI
		f.Area, f.Message, f.Resolution = AreaID(f.Area), clean(f.Message, maxText), clean(f.Resolution, maxText)
		if !slices.Contains(core.TechFindingCategories, f.Category) {
			f.Category = "other"
		}
		if f.Severity != "error" && f.Severity != "warning" {
			f.Severity = "info"
		}
		if f.Message != "" {
			fs = append(fs, f)
		}
	}
	ev.Findings = fs
	ev.Score = Score(ev.Criteria)
}

// Score is the evaluation as 0-100 from the criteria that were scored 1-5.
// A criterion where low is better counts inverted. Foundry computes it; the
// AI does not set it.
func Score(cs []core.TechCriterion) int {
	sum, n := 0.0, 0
	for _, c := range cs {
		if !slices.Contains(core.TechCriteria, c.Name) || c.Score < 1 || c.Score > 5 {
			continue
		}
		v := float64(c.Score-1) / 4
		if slices.Contains(core.TechLowerIsBetter, c.Name) {
			v = 1 - v
		}
		sum += v
		n++
	}
	if n == 0 {
		return 0
	}
	return int(sum/float64(n)*100 + 0.5)
}
