package work

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/repoplan"
	"github.com/handrawanwdev/foundry/internal/modules/techplan"
)

// A refused technology plan is repaired, not designed again: the next prompt
// names every problem the validator found (code, path, fix) and asks only for
// the parts it names. Foundry merges the answer into the refused draft, so
// what was valid cannot be lost or changed by the repair.

// techRepairing reports whether the next technology attempt is a targeted
// repair of a refused draft.
func techRepairing(p *core.Project) bool {
	if p.Stack == nil {
		return false
	}
	r := p.Stack.PlanRequest
	return r != nil && r.Action == core.TechPlanGenerate && r.Draft != nil && len(r.Issues) > 0
}

// syntaxRefusal: the last answer could not be read at all, so only its JSON
// needs repair; a refusal of valid JSON needs the content fixed instead.
func syntaxRefusal(msg string) bool {
	return strings.HasPrefix(msg, "output AI") || strings.Contains(msg, "tidak berisi bagian")
}

// PlanStrategy is how the next unified round is asked for: a design, a
// targeted repair of the technology draft, or a JSON syntax repair.
func PlanStrategy(p *core.Project) string {
	if CurrentPlanPart(p) == KindTech && techRepairing(p) {
		return core.TechAttemptRepair
	}
	if syntaxRefusal(planRejection(p)) && lastPlanOutput(p) != "" {
		return core.TechAttemptSyntax
	}
	return core.TechAttemptGenerate
}

// TechStrategy is PlanStrategy for the standalone technology job.
func TechStrategy(p *core.Project) string {
	if techRepairing(p) {
		return core.TechAttemptRepair
	}
	return core.TechAttemptGenerate
}

// TechRepairPrompt asks for exactly the parts of the refused draft that the
// validator named. unified frames it as a round of Unified Blueprint
// Planning.
func TechRepairPrompt(p *core.Project, unified bool) string {
	r := p.Stack.PlanRequest
	draft := r.Draft
	sc := techplan.RepairScope(draft, r.Issues)
	var b strings.Builder
	if unified {
		b.WriteString("You are the unified blueprint architect of Foundry. ")
	} else {
		b.WriteString("You are the technology architect of Foundry. ")
	}
	b.WriteString(`Foundry designs software and writes instructions for an external coding agent; it never implements and you must not write application code or touch files.
This is a TARGETED REPAIR of the TECHNOLOGY plan. Foundry's validator refused your last answer for the problems listed below. Every other part of that answer passed: Foundry keeps it exactly as it is. Do not resend it and do not redesign it.
Repair rules:
- Fix exactly the listed problems. Never change a decision that is not listed.
- An area listed under ALTERNATIVES TO COMPLETE keeps its technology: send its complete alternatives list. Complete every alternative with why (why it is viable for THIS system), difference (against the chosen technology), pros and cons (non-empty lists). Remove an alternative you cannot honestly explain that way; never pad.
- An item you send in "set" is COMPLETE (every field of the contract, alternatives included) and replaces the old one. Keep its technology unless a problem is about the choice itself.
- Explanations point at this system (its requirements, platform, scale, constraints), never generic text. Do not invent facts: an unknown stays an unknown, with confidence low and the assumption stated.
- Write every human-readable text in the same language as the story.

`)
	b.WriteString(techplan.Contract(p.Stack))
	b.WriteString("\n")
	b.WriteString(CacheBreak)

	fmt.Fprintf(&b, "PROJECT: %s\n", p.Name)
	if d := p.Discovery; d != nil && d.Summary != "" {
		fmt.Fprintf(&b, "SYSTEM: %s\n", cutRunes(d.Summary, 600))
	}
	b.WriteString("\n")
	if p.Platform != nil && len(p.Platform.Targets) > 0 {
		b.WriteString("TARGET PLATFORM:\n")
		for _, t := range p.Platform.Targets {
			fmt.Fprintf(&b, "- %s: %s\n", t.Kind, cutRunes(t.Rationale, 200))
		}
		b.WriteString("\n")
	}
	if s := p.Survey; s != nil && strings.TrimSpace(s.Constraints) != "" {
		fmt.Fprintf(&b, "MANDATORY CONSTRAINTS (the user's words; binding): %s\n\n", s.Constraints)
	}
	writeRepairRefs(&b, p)
	writeRepairBindings(&b, p)

	fmt.Fprintf(&b, "YOUR LAST ANSWER WAS REFUSED, fix this: %d problems (%s)\n", len(r.Issues), techplan.GroupSummary(r.Issues))
	for _, is := range r.Issues {
		fmt.Fprintf(&b, "- [%s] %s: %s", is.Code, orNo(is.Path, "plan"), is.Message)
		if is.Hint != "" {
			fmt.Fprintf(&b, " → %s", is.Hint)
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")

	if !sc.Full {
		b.WriteString("ACCEPTED DECISIONS (kept by Foundry; use them for depends_on and the checks, do not resend):\n")
		for _, it := range draft.Items {
			if !slices.Contains(sc.Areas, it.Area) {
				fmt.Fprintf(&b, "- %s: %s\n", it.Area, it.Technology)
			}
		}
		b.WriteString("\n")
	}
	var altOnly, rewrite []string
	for _, a := range sc.Areas {
		switch {
		case slices.Contains(sc.Missing, a):
		case sc.AltOnly[a]:
			altOnly = append(altOnly, a)
		default:
			rewrite = append(rewrite, a)
		}
	}
	if len(altOnly) > 0 {
		b.WriteString("ALTERNATIVES TO COMPLETE (area | technology | purpose | reasoning, then its current alternatives):\n")
		for _, a := range altOnly {
			it := draft.Item(a)
			fmt.Fprintf(&b, "- %s | %s | %s | %s\n  current: %s\n", it.Area, it.Technology, cutRunes(it.Purpose, 200), cutRunes(it.Reasoning, 300), compactJSON(it.Alternatives))
		}
		b.WriteString("\n")
	}
	if len(rewrite) > 0 {
		b.WriteString("ITEMS TO REWRITE (your previous version):\n")
		for _, a := range rewrite {
			fmt.Fprintf(&b, "- %s\n", compactJSON(draft.Item(a)))
		}
		b.WriteString("\n")
	}
	if len(sc.Missing) > 0 {
		fmt.Fprintf(&b, "ITEMS TO ADD (missing from the plan): %s\n\n", strings.Join(sc.Missing, ", "))
	}
	if !sc.Full {
		if sc.Context {
			fmt.Fprintf(&b, "CURRENT CONTEXT: %s\n\n", compactJSON(draft.Context))
		}
		if sc.Evaluation {
			fmt.Fprintf(&b, "CURRENT EVALUATION: %s\n\n", compactJSON(draft.Evaluation.Criteria))
		}
		if sc.Consistency {
			fmt.Fprintf(&b, "CURRENT CONSISTENCY: %s\n\n", compactJSON(draft.Consistency))
		}
	}
	if unified {
		b.WriteString("SECTIONS TO PRODUCE: technology (targeted repair only)\n\n")
	}

	var keys []string
	if sc.Full {
		keys = append(keys, `"approach": "...", "summary": "...", "context": {...}, "items": [ every decision of the plan ], "compatibility": [...], "evaluation": {...}, "consistency": [...]`)
	} else {
		if len(rewrite)+len(sc.Missing) > 0 {
			keys = append(keys, fmt.Sprintf(`"set": [ one complete item for each of: %s ]`, strings.Join(append(slices.Clone(rewrite), sc.Missing...), ", ")))
		}
		if len(altOnly) > 0 {
			keys = append(keys, fmt.Sprintf(`"alternatives": { "<area>": [{"technology": "...", "why": "...", "difference": "...", "pros": ["..."], "cons": ["..."]}] } for each of: %s`, strings.Join(altOnly, ", ")))
		}
		if sc.Meta {
			keys = append(keys, `"approach": "...", "summary": "..."`)
		}
		if sc.Context {
			keys = append(keys, `"context": {"summary": "...", "facts": [{"category": "...", "statement": "...", "source": "..."}], "unknowns": [], "assumptions": []}`)
		}
		if sc.Compat {
			keys = append(keys, `"compatibility": [ the review of the whole stack ]`)
		}
		if sc.Evaluation {
			keys = append(keys, `"evaluation": {"criteria": [ only the criteria named above, complete ]}`)
		}
		if sc.Consistency {
			keys = append(keys, `"consistency": [ only the areas named above ]`)
		}
	}
	b.WriteString("Reply with ONLY JSON in one ```json fence, no prose. Only these keys")
	if unified {
		b.WriteString(", wrapped as {\"technology\": { ... }}")
	}
	b.WriteString(":\n{" + strings.Join(keys, ",\n ") + "}\n")
	return b.String()
}

// writeRepairRefs lists the ids an item may name in fulfills and evaluation
// evidence.
func writeRepairRefs(b *strings.Builder, p *core.Project) {
	if len(p.Requirements) > 0 {
		b.WriteString("REQUIREMENTS:\n")
		for _, r := range p.Requirements {
			fmt.Fprintf(b, "- %s [%s] %s\n", r.ID, r.Priority, cutRunes(r.Text, 160))
		}
		b.WriteString("\n")
	}
	if len(p.Features) > 0 {
		var ids []string
		for _, f := range p.Features {
			ids = append(ids, f.ID+" "+f.Name)
		}
		fmt.Fprintf(b, "FEATURES: %s\n\n", strings.Join(ids, "; "))
	}
	if mods := repoplan.Modules(p); len(mods) > 0 {
		var ids []string
		for _, m := range mods {
			ids = append(ids, m.ID)
		}
		fmt.Fprintf(b, "PROJECT MODULES: %s\n\n", strings.Join(ids, ", "))
	}
	if s := p.Stack; s != nil && len(s.Needs) > 0 {
		fmt.Fprintf(b, "TECHNICAL NEED SIGNALS: %s\n\n", strings.Join(s.Needs, ", "))
	}
}

// writeRepairBindings lists the choices the plan may not change.
func writeRepairBindings(b *strings.Builder, p *core.Project) {
	s := p.Stack
	if s == nil {
		return
	}
	var lines []string
	for _, d := range s.Decisions {
		switch {
		case d.Override != nil:
			lines = append(lines, fmt.Sprintf("- %s: %s (chosen by the user)", d.Area, d.Override.Value))
		case d.Source == core.SourcePreference:
			lines = append(lines, fmt.Sprintf("- %s: %s (chosen by the user)", d.Area, d.Choice()))
		case d.Source == core.SourceExisting || d.Source == "" && s.Mode == core.StructureExisting:
			lines = append(lines, fmt.Sprintf("- %s: %s (already used in the code)", d.Area, d.Choice()))
		}
	}
	if len(lines) > 0 {
		b.WriteString("BINDING CHOICES (use exactly this technology):\n" + strings.Join(lines, "\n") + "\n\n")
	}
}

func compactJSON(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(raw)
}

func cutRunes(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

// TechSection is the technology part of a unified answer: the "technology"
// object when there is one, else the whole answer (a repair may answer with
// the patch alone).
func TechSection(raw string) string {
	if ans, err := ParseUnified(raw); err == nil && ans.has(KindTech) {
		return ans.raw(KindTech)
	}
	return raw
}

// techFix is the JSON of a repair answer. A whole plan ("items") is read
// the same way: only the parts in scope are merged.
type techFix struct {
	Set          []core.TechItem                   `json:"set"`
	Items        []core.TechItem                   `json:"items"`
	Alternatives map[string][]core.TechAlternative `json:"alternatives"`
	Approach     string                            `json:"approach"`
	Summary      string                            `json:"summary"`
	Rationale    string                            `json:"rationale"`
	Context      *core.TechContext                 `json:"context"`
	Compat       []core.TechCompat                 `json:"compatibility"`
	Evaluation   *core.TechEvaluation              `json:"evaluation"`
	Consistency  []core.TechCheck                  `json:"consistency"`
}

func (f *techFix) any() bool {
	return f.Set != nil || f.Items != nil || f.Alternatives != nil || f.Context != nil || f.Compat != nil ||
		f.Evaluation != nil || f.Consistency != nil || f.Approach != "" || f.Summary != ""
}

// repairTechPlan merges a repair answer into the draft and validates the
// result as a whole. A refusal carries the merged plan: what the repair
// fixed stays fixed for the next attempt.
func repairTechPlan(p *core.Project, raw string) (*core.TechPlan, techplan.Merged, error) {
	r := p.Stack.PlanRequest
	var fix techFix
	if err := ExtractJSONFunc(raw, &fix, fix.any); err != nil {
		return nil, techplan.Merged{}, err
	}
	plan, res := techplan.Merge(r.Draft, r.Issues, techplan.Fix{
		Items: append(fix.Set, fix.Items...), Alternatives: fix.Alternatives,
		Approach: fix.Approach, Summary: fix.Summary, Rationale: fix.Rationale,
		Context: fix.Context, Compat: fix.Compat, Evaluation: fix.Evaluation, Consistency: fix.Consistency,
	})
	plan.History, plan.Analysis, plan.Stale = nil, core.TechAnalysis{}, false
	if err := techplan.Accept(p, plan); err != nil {
		var rej *techplan.RejectError
		if errors.As(err, &rej) {
			rej.Merge = &res
		}
		return nil, res, err
	}
	return plan, res, nil
}
