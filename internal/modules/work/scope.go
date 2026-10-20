package work

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/handrawanw/foundry/internal/core"
)

// The structured parts of the blueprint besides the UI design: use cases
// (flows), acceptance criteria per requirement with the Definition of
// Done, the decision register and the areas left out on purpose. The AI
// writes them in discovery and revises them with the blueprint; the user
// edits them directly.

// scopeOut is what a blueprint revision may change besides artifacts.
type scopeOut struct {
	UseCases       []core.Flow     `json:"use_cases"`
	RemoveUseCases []string        `json:"remove_use_cases"`
	Acceptance     []AcceptanceIn  `json:"acceptance"`
	Done           []string        `json:"definition_of_done"`
	Decisions      []core.Decision `json:"decisions"`
	Skip           []core.AreaSkip `json:"skip"`
	Unskip         []string        `json:"unskip"`
}

func (o scopeOut) empty() bool {
	return len(o.UseCases) == 0 && len(o.RemoveUseCases) == 0 && len(o.Acceptance) == 0 && o.Done == nil &&
		len(o.Decisions) == 0 && len(o.Skip) == 0 && len(o.Unskip) == 0
}

// AcceptanceIn is the acceptance of one requirement.
type AcceptanceIn struct {
	Requirement string           `json:"requirement"`
	Criteria    []core.Criterion `json:"criteria"`
	Untestable  string           `json:"untestable"`
}

// scopeRules tells the AI how to write the structured blueprint parts.
const scopeRules = `Structured blueprint rules:
- use cases (flows): role is the actor; goal, preconditions, steps (main flow), postconditions (what holds when it succeeds), alternatives and failures (at least one of them), business_rules, permissions (who may do it; use the project's own permission names, do not assume RBAC or JWT), data_effects (entity: what changes), integrations (external systems called or notified). Leave a list empty when it does not apply.
- acceptance criteria per requirement: kind is behavior | negative | edge | security | performance | compatibility | accessibility; text is one checkable condition (Given/When/Then is fine); evidence is the proof the coding agent must hand back (test, command output, screenshot). Every MUST requirement gets a negative or edge case. A requirement that cannot be tested gets "untestable" with the reason instead of criteria. Add performance numbers only when the story or survey gives them.
- definition_of_done: checkable conditions every finished command meets (tests pass, criteria proven with evidence, no secret in code, docs updated, ...), sized to this project.
- decisions: kind is assumption | question | conflict | risk; text says what is assumed, asked, conflicting or at risk and its impact; area is one of ui, use-cases, data, security, integration, operations, migration, acceptance (or empty); critical is true only when the blueprint cannot be finalized safely without the user's decision. Never resolve a question yourself; record what you assumed instead.
- skip: areas this project does not need, each with the concrete reason (only use-cases, data, security, integration, operations, migration). Do not skip to save work.
- never invent APIs, endpoints, limits, SLO numbers or service capabilities that were not given or verified; record them as a question or an assumption.
`

// Blueprint area artifacts the AI writes as Markdown, with what they cover.
func areaGuide(b *strings.Builder, p *core.Project) {
	b.WriteString("Blueprint areas (artifact id: topics). Write the area artifacts that apply to this project, with area set; skip the others with a reason:\n")
	for _, a := range core.Areas {
		if a.Builtin {
			continue
		}
		applies, why := p.AreaApplies(a.ID)
		note := ""
		if !applies {
			note = " (likely not needed: " + why + ")"
		}
		fmt.Fprintf(b, "- %s, area %s%s: %s\n", a.Artifacts[0], a.ID, note, strings.Join(a.Topics, "; "))
	}
	b.WriteString("Do not repeat what another artifact already says (entities and modules are in architecture, permissions in use cases); refer to it.\n\n")
}

// ---------- normalizing ----------

func normFlow(f core.Flow) core.Flow {
	f.ID = strings.ToUpper(strings.TrimSpace(f.ID))
	f.Name, f.FeatureID, f.Role, f.Goal = strings.TrimSpace(f.Name), strings.TrimSpace(f.FeatureID), strings.TrimSpace(f.Role), strings.TrimSpace(f.Goal)
	f.Steps, f.Pre, f.Post = nonEmpty(f.Steps), nonEmpty(f.Pre), nonEmpty(f.Post)
	f.Alternatives, f.Failures, f.Rules = nonEmpty(f.Alternatives), nonEmpty(f.Failures), nonEmpty(f.Rules)
	f.Permissions, f.Data, f.Integrations = nonEmpty(f.Permissions), nonEmpty(f.Data), nonEmpty(f.Integrations)
	return f
}

// normCriteria trims the criteria of a requirement; ids that do not belong
// to it get the next free number.
func normCriteria(req string, in []core.Criterion) []core.Criterion {
	var out []core.Criterion
	used := map[string]bool{}
	for _, c := range in {
		c.Text, c.Evidence = strings.TrimSpace(c.Text), strings.TrimSpace(c.Evidence)
		c.Kind = strings.ToLower(strings.TrimSpace(c.Kind))
		if c.Kind == "" {
			c.Kind = "behavior"
		}
		if c.Text == "" {
			continue
		}
		c.ID = strings.ToUpper(strings.TrimSpace(c.ID))
		if !strings.HasPrefix(c.ID, req+"-AC") || used[c.ID] {
			c.ID = ""
		}
		used[c.ID] = true
		out = append(out, c)
	}
	n := 1
	for i := range out {
		for out[i].ID == "" {
			if id := fmt.Sprintf("%s-AC%d", req, n); !used[id] {
				out[i].ID, used[id] = id, true
			}
			n++
		}
	}
	return out
}

// nextDecisionID is the next free DEC number.
func nextDecisionID(ds []core.Decision) string {
	next := 1
	for _, d := range ds {
		var n int
		if _, err := fmt.Sscanf(d.ID, "DEC-%d", &n); err == nil && n >= next {
			next = n + 1
		}
	}
	return fmt.Sprintf("DEC-%03d", next)
}

// mergeDecisions adds or replaces decisions by id; unknown or empty ids
// are new.
func mergeDecisions(list []core.Decision, in []core.Decision) []core.Decision {
	list = slices.Clone(list)
	for _, d := range in {
		d.ID = strings.ToUpper(strings.TrimSpace(d.ID))
		d.Kind, d.Area = strings.ToLower(strings.TrimSpace(d.Kind)), strings.TrimSpace(d.Area)
		d.Text, d.Resolution = strings.TrimSpace(d.Text), strings.TrimSpace(d.Resolution)
		if i := slices.IndexFunc(list, func(x core.Decision) bool { return x.ID == d.ID }); i >= 0 && d.ID != "" {
			list[i] = d
			continue
		}
		d.ID = nextDecisionID(list)
		list = append(list, d)
	}
	return list
}

func skipArea(p *core.Project, area, reason string) error {
	a := core.AreaByID(area)
	reason = strings.TrimSpace(reason)
	switch {
	case a == nil:
		return fmt.Errorf("area %q tidak dikenal", area)
	case !a.Skippable:
		return fmt.Errorf("area %s tidak bisa dilewati", a.Title)
	case reason == "":
		return fmt.Errorf("tuliskan alasan melewati %s", a.Title)
	}
	unskip(p, area)
	p.Skips = append(p.Skips, core.AreaSkip{Area: area, Reason: reason})
	return nil
}

func unskip(p *core.Project, area string) {
	p.Skips = slices.DeleteFunc(p.Skips, func(s core.AreaSkip) bool { return s.Area == area })
	if len(p.Skips) == 0 {
		p.Skips = nil
	}
}

// refuseTouched refuses errors on the items a change touched; errors on
// other items were there before and stay blockers only.
func refuseTouched(subject string, issues []core.Issue, touched map[string]bool) error {
	var errs []core.Issue
	for _, is := range core.Errors(issues) {
		if touched[is.Where] {
			errs = append(errs, is)
		}
	}
	if len(errs) > 0 {
		return &core.Invalid{Subject: subject, Issues: errs}
	}
	return nil
}

// ---------- applying ----------

// applyScope lands a revision's structured changes on p. It returns
// whether anything changed.
func applyScope(p *core.Project, o scopeOut) (bool, error) {
	if o.empty() {
		return false, nil
	}
	touched := map[string]bool{}
	flows := slices.Clone(p.Flows)
	for _, id := range o.RemoveUseCases {
		i := slices.IndexFunc(flows, func(f core.Flow) bool { return f.ID == id })
		if i < 0 {
			return false, fmt.Errorf("use case %s tidak ada", id)
		}
		flows = slices.Delete(flows, i, i+1)
	}
	for _, f := range o.UseCases {
		f = normFlow(f)
		if i := slices.IndexFunc(flows, func(x core.Flow) bool { return x.ID == f.ID }); i >= 0 && f.ID != "" {
			if f.PageIDs == nil {
				f.PageIDs = flows[i].PageIDs
			}
			flows[i] = f
		} else {
			f.ID = nextFlowID(flows)
			flows = append(flows, f)
		}
		touched[f.ID] = true
	}
	if err := refuseTouched("use case", p.UseCaseIssues(flows), touched); err != nil {
		return false, err
	}
	reqs := slices.Clone(p.Requirements)
	for _, a := range o.Acceptance {
		id := strings.ToUpper(strings.TrimSpace(a.Requirement))
		i := slices.IndexFunc(reqs, func(r core.Requirement) bool { return r.ID == id })
		if i < 0 {
			return false, fmt.Errorf("requirement %q tidak ada", a.Requirement)
		}
		reqs[i].Acceptance, reqs[i].Untestable = normCriteria(id, a.Criteria), strings.TrimSpace(a.Untestable)
		touched[id] = true
	}
	done := p.Done
	if o.Done != nil {
		done = nonEmpty(o.Done)
		touched[core.ArtifactAcceptance] = true
	}
	if err := refuseTouched("acceptance criteria", core.AcceptanceIssues(reqs, done), touched); err != nil {
		return false, err
	}
	decisions := mergeDecisions(p.Decisions, o.Decisions)
	if issues := core.DecisionIssues(decisions); len(issues) > 0 {
		return false, &core.Invalid{Subject: "keputusan", Issues: issues}
	}
	next := *p
	next.Skips = slices.Clone(p.Skips)
	for _, a := range o.Unskip {
		unskip(&next, a)
	}
	for _, s := range o.Skip {
		if err := skipArea(&next, s.Area, s.Reason); err != nil {
			return false, err
		}
	}
	p.Flows, p.Requirements, p.Done, p.Decisions, p.Skips = flows, reqs, done, decisions, next.Skips
	return true, nil
}

func nextFlowID(flows []core.Flow) string {
	next := 1
	for _, f := range flows {
		var n int
		if _, err := fmt.Sscanf(f.ID, "FLOW-%d", &n); err == nil && n >= next {
			next = n + 1
		}
	}
	return fmt.Sprintf("FLOW-%03d", next)
}

// ---------- user edits ----------

// EditUseCases replaces the use cases with the user's. Changed use cases
// must be valid; page links of a use case are kept when not given.
// Unchanged input changes nothing.
func EditUseCases(p *core.Project, in []core.Flow) error {
	touched := map[string]bool{}
	var flows []core.Flow
	for _, f := range in {
		f = normFlow(f)
		old := slices.IndexFunc(p.Flows, func(x core.Flow) bool { return x.ID == f.ID })
		if old >= 0 && f.PageIDs == nil {
			f.PageIDs = p.Flows[old].PageIDs
		}
		if f.ID == "" {
			f.ID = nextFlowID(append(slices.Clone(p.Flows), flows...))
		}
		if old < 0 || !flowEqual(p.Flows[old], f) {
			touched[f.ID] = true
		}
		flows = append(flows, f)
	}
	if err := refuseTouched("use case", p.UseCaseIssues(flows), touched); err != nil {
		return err
	}
	p.Flows = flows
	return nil
}

func flowEqual(a, b core.Flow) bool { return mustJSON(a) == mustJSON(b) }

// EditAcceptance saves the user's acceptance criteria and Definition of
// Done. Changed requirements must be valid.
func EditAcceptance(p *core.Project, in []AcceptanceIn, done []string) error {
	_, err := applyScope(p, scopeOut{Acceptance: in, Done: nonNil(done)})
	return err
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// ResolveDecision records the user's decision; an empty resolution opens
// it again.
func ResolveDecision(p *core.Project, id, resolution string) error {
	i := slices.IndexFunc(p.Decisions, func(d core.Decision) bool { return d.ID == id })
	if i < 0 {
		return fmt.Errorf("keputusan %s tidak ada", id)
	}
	p.Decisions[i].Resolution = strings.TrimSpace(resolution)
	return nil
}

// SkipArea leaves an area out on purpose; UnskipArea brings it back.
func SkipArea(p *core.Project, area, reason string) error { return skipArea(p, area, reason) }

func UnskipArea(p *core.Project, area string) error {
	if p.Skip(area) == "" {
		return fmt.Errorf("area %q tidak sedang dilewati", area)
	}
	unskip(p, area)
	return nil
}

// ---------- artifact review ----------

// StampRefs records the current hashes of the refs of every written
// artifact that has none recorded yet: from now on a change of a
// referenced artifact marks it for review.
func StampRefs(p *core.Project, hashes map[string]string) {
	for i := range p.Artifacts {
		a := &p.Artifacts[i]
		if a.Seen != nil || len(a.Refs) == 0 {
			continue
		}
		a.Seen = map[string]string{}
		for _, r := range a.Refs {
			if h, ok := hashes[r]; ok {
				a.Seen[r] = h
			}
		}
	}
}

// ReviewArtifact accepts the current state of an artifact's refs.
func ReviewArtifact(p *core.Project, id string, hashes map[string]string) error {
	i := slices.IndexFunc(p.Artifacts, func(a core.Artifact) bool { return a.ID == id })
	if i < 0 {
		return fmt.Errorf("artefak %q tidak ada atau bawaan", id)
	}
	if len(core.StaleRefs(p.Artifacts[i], hashes)) == 0 {
		return errors.New("artefak tidak perlu ditinjau")
	}
	p.Artifacts[i].Seen = nil
	StampRefs(p, hashes)
	return nil
}

// ScopeRequest is the change request that asks the AI to complete the
// blueprint areas that are missing or invalid; "" when nothing is.
func ScopeRequest(p *core.Project, docs []core.Artifact) string {
	var todo []string
	for _, a := range core.Coverage(p, docs) {
		if a.ID == core.AreaUI {
			continue // the UI design has its own job
		}
		switch a.Status {
		case core.AreaMissing:
			todo = append(todo, fmt.Sprintf("tulis area %s (%s), atau lewati dengan alasan jika memang tidak relevan", a.ID, a.Title))
		case core.AreaInvalid:
			todo = append(todo, fmt.Sprintf("perbaiki area %s (%s): %d hal wajib", a.ID, a.Title, a.Errors))
		}
	}
	if len(todo) == 0 {
		return ""
	}
	return "Lengkapi blueprint: " + strings.Join(todo, "; ") + "."
}
