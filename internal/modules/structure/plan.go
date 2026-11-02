package structure

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
)

// The AI structure recommendation: the whole system is analyzed, 2-3
// complete alternatives are compared on the qualities that decide
// maintainability, and one is recommended. A plan is only a recommendation;
// adopting an alternative goes through the existing proposal and approval.

var (
	ErrNoPlan      = errors.New("analisis struktur AI belum ada")
	ErrPlanStale   = errors.New("analisis struktur AI sudah kedaluwarsa; analisis ulang dulu")
	ErrPlanCurrent = errors.New("analisis struktur AI masih berlaku: artefak sumber belum berubah")
)

const (
	minAlternatives = 2
	maxAlternatives = 3
)

// SourceHash identifies the system the analysis reads: story, discovery,
// blueprint, requirements, features, flows, platform and the technology
// choices. It deliberately excludes the structure and the plan themselves,
// so generating a plan does not make it stale.
func SourceHash(p *core.Project) string {
	type stackSnapshot struct {
		Mode      core.StructureMode  `json:"mode"`
		Needs     []string            `json:"needs,omitempty"`
		Platform  core.TechPlatform   `json:"platform"`
		Decisions []core.TechDecision `json:"decisions,omitempty"`
	}
	snapshot := struct {
		Name         string             `json:"name"`
		Story        string             `json:"story"`
		Survey       *core.Survey       `json:"survey,omitempty"`
		Discovery    *core.Discovery    `json:"discovery,omitempty"`
		Blueprint    *core.Blueprint    `json:"blueprint,omitempty"`
		Requirements []core.Requirement `json:"requirements,omitempty"`
		Features     []core.Feature     `json:"features,omitempty"`
		Flows        []core.Flow        `json:"flows,omitempty"`
		Platform     *core.PlatformPlan `json:"platform,omitempty"`
		Stack        *stackSnapshot     `json:"stack,omitempty"`
	}{Name: p.Name, Story: p.Story, Survey: p.Survey, Discovery: p.Discovery, Blueprint: p.Blueprint,
		Requirements: p.Requirements, Features: p.Features, Flows: p.Flows, Platform: p.Platform}
	if s := p.Stack; s != nil {
		snapshot.Stack = &stackSnapshot{Mode: s.Mode, Needs: s.Needs, Platform: s.Platform, Decisions: s.Decisions}
	}
	b, err := json.Marshal(snapshot)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:8])
}

// PlanCurrent reports whether the plan still matches the system snapshot it
// read.
func PlanCurrent(s *core.Structure, p *core.Project) bool {
	return s != nil && s.Plan != nil && s.Plan.SourceHash != "" && s.Plan.SourceHash == SourceHash(p)
}

// PlanNeeded reports whether an analysis has to run for s: none yet, or the
// source artifacts changed.
func PlanNeeded(s *core.Structure, p *core.Project) bool {
	return s != nil && s.Mode == core.StructureNew && !PlanCurrent(s, p)
}

// RefreshPlan updates only the stale mark of the AI analysis; a read of the
// project can call it without revalidating the whole structure.
func RefreshPlan(s *core.Structure, p *core.Project) {
	if s != nil && s.Plan != nil {
		s.Plan.Stale = !PlanCurrent(s, p)
	}
}

// NormalizePlan validates and cleans the AI's answer. Errors are concrete:
// a plan that cannot be the structure is refused, never silently repaired
// into a different architecture.
func NormalizePlan(p *core.Project, plan *core.StructurePlan) error {
	if plan == nil {
		return ErrNoPlan
	}
	plan.Summary = strings.TrimSpace(plan.Summary)
	plan.Reason = strings.TrimSpace(plan.Reason)
	plan.Drivers = cleanTexts(plan.Drivers, 20, maxRuleLen)
	plan.DependencyRules = cleanRules(plan.DependencyRules)
	plan.Domains = cleanDomains(plan.Domains)
	if plan.Summary == "" {
		return errors.New("analisis tanpa ringkasan sistem")
	}
	if len(plan.DependencyRules) == 0 {
		return errors.New("analisis harus menyertakan aturan dependency antarbagian")
	}
	if len(plan.Alternatives) < minAlternatives {
		return fmt.Errorf("analisis harus membandingkan %d-%d alternatif struktur, bukan %d",
			minAlternatives, maxAlternatives, len(plan.Alternatives))
	}
	if len(plan.Alternatives) > maxAlternatives {
		plan.Alternatives = plan.Alternatives[:maxAlternatives]
	}
	seen := map[string]bool{}
	for i := range plan.Alternatives {
		a := &plan.Alternatives[i]
		a.ID = Slug(a.ID)
		if a.ID == "" {
			a.ID = fmt.Sprintf("alternatif-%d", i+1)
		}
		base := a.ID
		for n := 2; seen[a.ID]; n++ {
			a.ID = fmt.Sprintf("%s-%d", base, n)
		}
		seen[a.ID] = true
		a.Name, a.Summary = strings.TrimSpace(a.Name), strings.TrimSpace(a.Summary)
		if a.Name == "" {
			return fmt.Errorf("alternatif %q tanpa nama", a.ID)
		}
		if a.IR == nil {
			return fmt.Errorf("alternatif %q tanpa struktur folder", a.Name)
		}
		if a.IR.Type == "" {
			a.IR.Type = strings.ToLower(strings.TrimSpace(a.Type))
		}
		a.Type = a.IR.Type
		if !slices.Contains(core.StructureTypes, a.Type) {
			return fmt.Errorf("alternatif %q memakai tipe struktur %q yang tidak dikenal (simple, modular, monorepo, microservices)", a.Name, a.Type)
		}
		Normalize(a.IR)
		fillModuleReasonsFromDomains(a.IR, plan.Domains)
		if err := partsHaveReasons(a.IR); err != nil {
			return fmt.Errorf("alternatif %q: %w", a.Name, err)
		}
		if missing := missingDomains(a.IR, Domains(p)); len(missing) > 0 {
			return fmt.Errorf("alternatif %q tidak menempatkan domain %s; setiap domain harus punya folder dan tanggung jawab",
				a.Name, strings.Join(missing, ", "))
		}
		if errs := errorIssues(Validate(a.IR, p, p.Stack)); len(errs) > 0 {
			return fmt.Errorf("alternatif %q tidak sesuai project: %s", a.Name, errs[0])
		}
		a.Evaluation = normalizeEvaluation(a.Evaluation)
		a.Pros = cleanTexts(a.Pros, 12, maxRuleLen)
		a.Cons = cleanTexts(a.Cons, 12, maxRuleLen)
		a.Tradeoffs = cleanTexts(a.Tradeoffs, 12, maxRuleLen)
	}
	plan.Recommendation = Slug(plan.Recommendation)
	if plan.Alternative(plan.Recommendation) == nil {
		// An invalid id is not a reason to throw the analysis away; the
		// best-scoring alternative becomes the recommendation.
		best := 0
		for i := 1; i < len(plan.Alternatives); i++ {
			if plan.Alternatives[i].Evaluation.Score > plan.Alternatives[best].Evaluation.Score {
				best = i
			}
		}
		plan.Recommendation = plan.Alternatives[best].ID
	}
	for i := range plan.Alternatives {
		plan.Alternatives[i].Recommended = plan.Alternatives[i].ID == plan.Recommendation
	}
	rec := plan.RecommendedAlternative()
	if plan.Reason == "" {
		plan.Reason = rec.Summary
	}
	if plan.Reason == "" {
		plan.Reason = rec.Name
	}
	if plan.Selected != "" && plan.Alternative(plan.Selected) == nil {
		plan.Selected = ""
		plan.Decision = ""
	}
	return nil
}

// AlternativeIR is the structure of one alternative with the plan's
// dependency rules applied.
func AlternativeIR(plan *core.StructurePlan, a *core.StructureAlternative) *core.StructureIR {
	if a == nil || a.IR == nil {
		return nil
	}
	ir := a.IR.Clone()
	if plan != nil {
		ir.Rules = append(ir.Rules, plan.DependencyRules...)
	}
	Normalize(ir)
	return ir
}

func cleanTexts(in []string, max int, maxLen int) []string {
	var out []string
	for _, v := range in {
		v = strings.Join(strings.Fields(v), " ")
		if len(v) > maxLen {
			v = strings.TrimSpace(v[:maxLen])
		}
		if v == "" || slices.ContainsFunc(out, func(x string) bool { return strings.EqualFold(x, v) }) {
			continue
		}
		if out = append(out, v); len(out) == max {
			break
		}
	}
	return out
}

func cleanDomains(in []core.StructureDomain) []core.StructureDomain {
	var out []core.StructureDomain
	for _, d := range in {
		d.Name = Slug(d.Name)
		d.Responsibility = strings.Join(strings.Fields(d.Responsibility), " ")
		if d.Name == "" || slices.ContainsFunc(out, func(x core.StructureDomain) bool { return x.Name == d.Name }) {
			continue
		}
		out = append(out, d)
		if len(out) == maxDomains {
			break
		}
	}
	return out
}

func normalizeEvaluation(e core.StructureEvaluation) core.StructureEvaluation {
	clamp := func(v int) int {
		if v < 1 {
			return 3 // a missing score is neutral, not a reason to fail
		}
		if v > 5 {
			return 5
		}
		return v
	}
	e.Maintainability = clamp(e.Maintainability)
	e.Simplicity = clamp(e.Simplicity)
	e.Extensibility = clamp(e.Extensibility)
	e.Testability = clamp(e.Testability)
	e.Separation = clamp(e.Separation)
	e.Complexity = clamp(e.Complexity)
	e.Score = (e.Maintainability + e.Simplicity + e.Extensibility + e.Testability + e.Separation - e.Complexity) * 5
	return e
}

func errorIssues(issues []core.TechIssue) []string {
	var out []string
	for _, i := range issues {
		if i.Level == "error" {
			out = append(out, i.Message)
		}
	}
	return out
}

func fillModuleReasonsFromDomains(ir *core.StructureIR, domains []core.StructureDomain) {
	byName := map[string]string{}
	for _, d := range domains {
		byName[d.Name] = d.Responsibility
	}
	for i := range ir.Services {
		svc := &ir.Services[i]
		if len(svc.ModuleReasons) == 0 {
			svc.ModuleReasons = map[string]string{}
		}
		for _, m := range svc.Modules {
			if strings.TrimSpace(svc.ModuleReasons[m]) == "" && byName[m] != "" {
				svc.ModuleReasons[m] = byName[m]
			}
		}
	}
}

// partsHaveReasons enforces that every folder the AI proposes explains what
// it is responsible for.
func partsHaveReasons(ir *core.StructureIR) error {
	check := func(kind string, ps []core.StructurePart) error {
		for _, p := range ps {
			if strings.TrimSpace(p.Reason) == "" {
				return fmt.Errorf("%s %s tanpa alasan/tanggung jawab", kind, p.Path)
			}
			if kind == core.PartService {
				for _, m := range p.Modules {
					if strings.TrimSpace(p.ModuleReasons[m]) == "" {
						return fmt.Errorf("modul %s%s tanpa alasan/tanggung jawab", p.Path, m)
					}
				}
			}
		}
		return nil
	}
	for _, group := range []struct {
		kind string
		ps   []core.StructurePart
	}{
		{core.PartApp, ir.Apps}, {core.PartService, ir.Services}, {core.PartWorker, ir.Workers}, {core.PartPackage, ir.Packages},
	} {
		if err := check(group.kind, group.ps); err != nil {
			return err
		}
	}
	return nil
}

func missingDomains(ir *core.StructureIR, want []string) []string {
	have := map[string]bool{}
	for _, d := range domainsIn(ir) {
		have[d] = true
	}
	var out []string
	for _, d := range want {
		if !have[d] {
			out = append(out, d)
		}
	}
	return out
}
