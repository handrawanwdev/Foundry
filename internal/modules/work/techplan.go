package work

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/repoplan"
	"github.com/handrawanwdev/foundry/internal/modules/stack"
	"github.com/handrawanwdev/foundry/internal/modules/techplan"
)

// KindTech covers two jobs of the technology plan: a fresh design by the AI
// and a revision of the current plan. Stack.PlanRequest decides which prompt
// runs and which Apply lands the answer.
const KindTech = "tech"

// TechPlanPrompt asks the AI to analyze what the system needs and design its
// whole technology: every area it needs, each with its reasoning, the
// alternatives it weighed, a compatibility review of the stack, an honest
// evaluation and the consistency checks.
func TechPlanPrompt(p *core.Project) string {
	if techRepairing(p) {
		return TechRepairPrompt(p, false)
	}
	var b strings.Builder
	b.WriteString(`You are the technology architect of Foundry. Foundry designs software and writes instructions for an external coding agent; it never implements and you must not write application code or touch files.
Decide the TECHNOLOGY of the system below. First understand what it needs, then choose, area by area, what it needs and nothing more.
You are free. Foundry's catalog and profiles are a REFERENCE of what Foundry happens to know, not a menu: choose any technology that fits, outside the catalog, an unpopular one, a combination, a new approach, or no technology at all for an area that does not need one. Reject what does not meet a need. Different areas may come from different ecosystems as long as they integrate. Do not copy the baseline; keep it only where it is right for THIS system. The project profile is weak context: an explicit requirement always outranks it and a technology is never "mandatory" because of a profile.
Pick the simplest stack that satisfies the requirements; every technology must serve a need that you can name. Do not add a component to be complete. Do not change the requirements, the platform or the domain boundaries.
Do not invent facts. When the requirements do not say something that matters (load, data volume, compliance, team skills, budget), list it in context.unknowns and state the assumption you made; never present an assumption as a requirement. Version numbers: give one only when you know it, and set version_verified true only if you checked it against a source in this answer (you usually cannot: leave it false). Never claim two technologies are compatible without a basis you can state.
Write every human-readable text in the same language as the story.

PROJECT: `)
	b.WriteString(p.Name)
	b.WriteString("\nSTORY:\n")
	b.WriteString(p.Story)
	b.WriteString("\n\n")
	writeBinding(&b, p)
	writeWorkIntent(&b, p)
	writeExistingContext(&b, p)
	writeTechContext(&b, p)
	writeTechAnchors(&b, p)
	writeTechPrevious(&b, p)
	b.WriteString(techPlanRules)
	b.WriteString(techplan.Contract(p.Stack))
	b.WriteString(`
Reply with ONLY one JSON object in a ` + "```json" + ` block:
{
  "approach": "<your own name for this technology strategy>",
  "summary": "<the stack in one short paragraph>",
  "rationale": "<why this stack fits this system>",
  "context": {"summary": "<what the system technically needs>",
    "facts": [{"category": "transaction", "statement": "...", "source": "requirements"}],
    "unknowns": ["<what the requirements do not say yet>"], "assumptions": ["<what you assumed because of it>"]},
  "items": [
    {"area": "backend", "group": "stack", "technology": "Elixir + Phoenix", "version": "Phoenix 1.7", "version_verified": false, "language": "Elixir", "runtime": "BEAM",
     "purpose": "...", "fulfills": ["REQ-002", "realtime"], "driver": "requirement", "reasoning": "...",
     "pros": ["..."], "cons": ["..."], "tradeoffs": ["..."], "depends_on": ["database"], "integration": "...", "operational": "...",
     "alternatives": [{"technology": "Go + Gin", "why": "...", "difference": "...", "pros": ["..."], "cons": ["..."], "architecture_impact": "...", "maintenance_impact": "...", "cost_impact": "..."}],
     "confidence": "medium", "assumptions": ["..."]},
    {"area": "cache", "group": "data", "technology": "Tidak perlu", "purpose": "", "fulfills": [], "driver": "engineering", "reasoning": "<why this area needs no technology>", "confidence": "high"}
  ],
  "constraints": [{"areas": ["orm", "backend"], "rule": "<a compatibility constraint, e.g. a version range>", "reason": "..."}],
  "compatibility": [{"area": "runtime", "status": "compatible", "between": ["backend", "database"], "note": "...", "basis": "<what makes you sure>"}],
  "evaluation": {"summary": "...", "criteria": [{"name": "requirement_coverage", "score": 4, "reason": "<concrete>", "evidence": ["backend", "REQ-002"]}],
    "findings": [{"category": "over-engineering", "severity": "info", "area": "messaging", "message": "...", "resolution": "..."}]},
  "consistency": [{"area": "requirements", "status": "consistent", "note": "..."}]
}`)
	return b.String()
}

const techPlanRules = `Rules:
- items is one decision per area. area is one of Foundry's areas — frontend, ui, mobile, desktop, cli, backend, api, database, cache, messaging, workers, storage, search, architecture, repository, deployment, observability, testing, quality — or any other area the system needs, as a lowercase id (runtime, state_management, orm, data_migration, authentication, authorization, session, background_jobs, scheduler, message_broker, integrations, containerization, hosting, networking, ci_cd, logging, metrics, tracing, monitoring, build_tool, package_manager, static_analysis, formatting, ...). Not every area needs a technology: decide each independently, and for an area the system does not need write technology "Tidak perlu" with the reason in reasoning. Do not list an area just to fill a category.
- group: stack (application stack), data (data architecture), app_infra (authentication, authorization, background processing, broker, scheduler, integrations), architecture (architecture and repository), operations (deployment, hosting, containers, networking, CI/CD, logging, metrics, tracing, monitoring), standards (testing, lint, format, build, packages, static analysis). Foundry's own areas have a fixed group.
- the platform decides which application parts exist: a web target needs frontend, mobile needs mobile, desktop needs desktop, backend needs backend, a CLI needs cli. architecture must be exactly one of "Simple", "Modular Monolith", "Microservices" and repository exactly "Simple" or "Monorepo"; Foundry derives the project structure from them, so they must agree with the structure decision and the folder plan shown above.
- every decision (except "Tidak perlu"): technology (your own name, with its language or framework), language and runtime when they apply, version (only if known) with version_verified, purpose (what it is for), fulfills (the requirement ids, feature ids, module ids or technical needs it serves; at least one), driver (requirement | architecture | compatibility | engineering | operational | profile), reasoning (a technical reason that points at this system), pros, cons, tradeoffs, depends_on (areas it integrates with, only areas that are in items), integration (how it fits those), operational (what it asks from operations), confidence (high | medium | low) and the assumptions it rests on. Facts, assumptions, preferences and your recommendation are different things: say which one each reason is.
- alternatives: only technologies that would genuinely work here. Each says why it is viable, how it differs from your choice, pros, cons, and its impact on architecture, maintenance and operational cost. No alternative when there is no other technically sensible option; never pad the list.
- compatibility: check the whole stack — language, runtime, framework, package, database, api, authentication, storage, build, deployment, testing, operational. status is compatible (only with a basis), unverified (plausible but you could not verify), conflict (an incompatibility you could not fix; it blocks approval), resolved (found and fixed in this answer) or not_applicable. List what is redundant, incompatible or unnecessary in evaluation.findings. Check these conflicts in particular and resolve them with the simplest sufficient fix, not by adding components: transactions without a database or persistence; file upload without a storage strategy; a Modular Monolith or microservices whose deployment has no backend; an ORM that does not run on the backend's runtime; background processing with nothing that executes it; authentication without a session or token strategy; a frontend that does not fit the application.
- evaluation: score EVERY criterion 1-5 with a concrete reason and evidence (a list of the ids it rests on: areas of items, requirement ids or module ids; a section name such as "compatibility" is not evidence): requirement_coverage, compatibility, maintainability, performance, scalability, security, reliability, developer_experience, sustainability (5 = best) and operational_complexity, total_cost (1 = low, 5 = high). A score without evidence is refused. A larger stack is not a better one; be honest.
- consistency: check the plan against each of requirements, project_modules, architecture, folder_structure, data_architecture, security, deployment. status is consistent, resolved (conflict found and fixed in this answer) or conflict (not fixable; blocks approval). Resolve conflicts before answering.
`

// writeTechContext lists what the AI must understand before it chooses.
func writeTechContext(b *strings.Builder, p *core.Project) {
	if s := p.Survey; s != nil {
		b.WriteString("SURVEY (the user's technical answers):\n")
		if len(s.Kinds) > 0 {
			fmt.Fprintf(b, "- kind: %s\n", strings.Join(s.Kinds, ", "))
		}
		if l := core.ChoiceLabel(core.SurveyUsers, s.Users); l != "" {
			fmt.Fprintf(b, "- users: %s\n", l)
		}
		if l := core.ChoiceLabel(core.SurveyStages, s.Stage); l != "" {
			fmt.Fprintf(b, "- delivery target: %s — %s\n", l, core.DeliveryCriteria[s.Stage])
		}
		if l := core.ChoiceLabel(core.SurveyArchs, s.Architecture); l != "" {
			fmt.Fprintf(b, "- architecture preference: %s\n", l)
		}
		for _, id := range s.Priorities {
			if l := core.ChoiceLabel(core.SurveyPriorities, id); l != "" {
				fmt.Fprintf(b, "- priority: %s\n", l)
			}
		}
		if s.Constraints != "" {
			fmt.Fprintf(b, "- MANDATORY CONSTRAINTS (the user's words; binding): %s\n", s.Constraints)
		}
		b.WriteString("\n")
	}
	if d := p.Discovery; d != nil {
		fmt.Fprintf(b, "DISCOVERY: %s\nBUSINESS: %s\n", d.Summary, d.Business)
		if len(d.Users) > 0 {
			fmt.Fprintf(b, "USERS: %s\n", strings.Join(d.Users, ", "))
		}
		for _, r := range d.Roles {
			fmt.Fprintf(b, "- role %s (%s): %s\n", r.Name, r.Platform, r.Description)
		}
		for _, x := range []struct {
			label string
			list  []string
		}{{"DATA", d.Data}, {"INTEGRATIONS", d.Integrations}, {"SECURITY", d.Security}, {"WORKFLOWS", d.Workflows}} {
			if len(x.list) > 0 {
				fmt.Fprintf(b, "%s: %s\n", x.label, strings.Join(x.list, "; "))
			}
		}
		b.WriteString("\n")
	}
	if p.Platform != nil {
		b.WriteString("TARGET PLATFORM:\n")
		for _, t := range p.Platform.Targets {
			fmt.Fprintf(b, "- %s: %s\n", t.Kind, t.Rationale)
		}
		b.WriteString("\n")
	}
	if len(p.Requirements) > 0 {
		b.WriteString("REQUIREMENTS:\n")
		for _, r := range p.Requirements {
			fmt.Fprintf(b, "- %s [%s] %s\n", r.ID, r.Priority, r.Text)
		}
		b.WriteString("\n")
	}
	if len(p.Features) > 0 {
		b.WriteString("FEATURES:\n")
		for _, f := range p.Features {
			fmt.Fprintf(b, "- %s %s: %s (requirements %s; platforms %s)\n", f.ID, f.Name, f.Description, strings.Join(f.RequirementIDs, ", "), strings.Join(f.Platforms, ", "))
		}
		b.WriteString("\n")
	}
	if len(p.Flows) > 0 {
		b.WriteString("USE CASES:\n")
		for _, f := range p.Flows {
			fmt.Fprintf(b, "- %s %s (%s): %s\n", f.ID, f.Name, f.Role, strings.Join(f.Steps, " → "))
		}
		b.WriteString("\n")
	}
	if mods := repoplan.Modules(p); len(mods) > 0 {
		b.WriteString("PROJECT MODULES:\n")
		for _, m := range mods {
			fmt.Fprintf(b, "- %s: %s\n", m.ID, m.Responsibility)
		}
		b.WriteString("\n")
	}
	if len(p.Artifacts) > 0 {
		b.WriteString("BLUEPRINT DOCUMENTS (domain, security, API and operations rules the technology must serve):\n")
		for _, a := range p.Artifacts {
			body := a.Body
			if len(body) > 1500 {
				body = strings.ToValidUTF8(body[:1500], "") + "\n…(dipotong)"
			}
			fmt.Fprintf(b, "=== %s (%s) %s\n%s\n", a.ID, a.Title, a.Summary, body)
		}
		b.WriteString("\n")
	}
	if s := p.Structure; s != nil && s.Mode != core.StructureExisting {
		if s.IR != nil {
			fmt.Fprintf(b, "STRUCTURE DECISION: type %s (the technology's architecture and repository must agree with it)\n", s.IR.Type)
		}
		if r := s.Repo; r != nil {
			fmt.Fprintf(b, "FOLDER PLAN (Folder Structure Planning; the technology must be consistent with it): approach %s; topology %s; languages of its files: %s\n",
				r.Approach, r.Topology, strings.Join(techplan.FileLanguages(r), ", "))
		}
		b.WriteString("\n")
	}
}

// writeTechAnchors lists the starting points and the binding choices.
func writeTechAnchors(b *strings.Builder, p *core.Project) {
	s := p.Stack
	if s == nil {
		return
	}
	if len(s.Needs) > 0 {
		fmt.Fprintf(b, "TECHNICAL NEED SIGNALS (keywords Foundry found in the requirements; a hint that may be wrong): %s\n\n", strings.Join(s.Needs, ", "))
	}
	if pr := stack.ProfileByID(s.Profile); pr != nil {
		fmt.Fprintf(b, "PROFILE HINT (weak context only, lowest priority): %s — %s\n\n", pr.Name, pr.Description)
	}
	var existing, chosen, base []string
	for _, d := range s.Decisions {
		line := fmt.Sprintf("- %s: %s", d.Area, d.Choice())
		if l := d.Lang(); l != "" {
			line += " (language " + l + ")"
		}
		switch {
		case d.Override != nil:
			chosen = append(chosen, line+" — "+orNo(d.Override.Reason, "chosen by the user"))
		case d.Source == core.SourcePreference:
			chosen = append(chosen, line+" — "+orNo(d.Reason, "chosen by the user"))
		case d.Source == core.SourceExisting || d.Source == "" && s.Mode == core.StructureExisting:
			existing = append(existing, line+" — "+d.Reason)
		case s.Plan == nil:
			base = append(base, line+" — "+d.Reason)
		}
	}
	if len(existing) > 0 {
		b.WriteString("TECHNOLOGY ALREADY USED IN THE CODE (binding: keep each exactly, same technology string; replacing one is a migration only the user can ask for):\n" + strings.Join(existing, "\n") + "\n\n")
	}
	if len(chosen) > 0 {
		b.WriteString("CHOSEN BY THE USER (binding: use exactly this technology in items, and explain and evaluate it; never replace it):\n" + strings.Join(chosen, "\n") + "\n\n")
	}
	if len(base) > 0 {
		b.WriteString("FOUNDRY BASELINE (a deterministic first guess from keywords and a profile; NOT binding; question every line):\n" + strings.Join(base, "\n") + "\n\n")
	}
	b.WriteString("CATALOG REFERENCE (technologies Foundry has compatibility data for; reference only, you may choose others):\n")
	for _, area := range core.TechAreas {
		var vs []string
		for _, o := range stack.Catalog[area] {
			vs = append(vs, o.Value)
		}
		if len(vs) > 0 {
			fmt.Fprintf(b, "- %s: %s\n", area, strings.Join(vs, " | "))
		}
	}
	b.WriteString("\n")
}

func orNo(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

// writeTechPrevious carries the previous plan and the refusal of the last
// answer into a design prompt.
func writeTechPrevious(b *strings.Builder, p *core.Project) {
	s := p.Stack
	if s == nil {
		return
	}
	if s.Plan != nil {
		b.WriteString("PREVIOUS TECHNOLOGY PLAN (design again; keep what still holds, change what the request or the changed sources require):\n")
		writeTechPlan(b, s.Plan, 60000)
		b.WriteString("\n")
	}
	if r := s.PlanRequest; r != nil {
		if t := strings.TrimSpace(r.Text); t != "" {
			b.WriteString("USER REQUEST:\n" + t + "\n\n")
		}
		writeTechRefusal(b, r)
	}
}

// writeTechRefusal tells the AI why its last answer was refused: every
// issue with its path and fix when the validator listed them, else the
// refusal text.
func writeTechRefusal(b *strings.Builder, r *core.TechPlanRequest) {
	if r.Rejected == "" {
		return
	}
	if len(r.Issues) == 0 {
		b.WriteString("YOUR LAST ANSWER WAS REFUSED, fix this: " + r.Rejected + "\n\n")
		return
	}
	fmt.Fprintf(b, "YOUR LAST ANSWER WAS REFUSED, fix this: %d problems (%s)\n", len(r.Issues), techplan.GroupSummary(r.Issues))
	for _, is := range r.Issues {
		fmt.Fprintf(b, "- [%s] %s: %s", is.Code, orNo(is.Path, "plan"), is.Message)
		if is.Hint != "" {
			fmt.Fprintf(b, " → %s", is.Hint)
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
}

// writeTechPlan writes the plan compactly: the exact areas and choices the AI
// must keep referring to.
func writeTechPlan(b *strings.Builder, plan *core.TechPlan, limit int) {
	cut := func(s string, n int) string {
		if r := []rune(s); len(r) > n {
			return string(r[:n]) + "…"
		}
		return s
	}
	fmt.Fprintf(b, "approach: %s\nsummary: %s\n", plan.Approach, plan.Summary)
	if c := plan.Context; c.Summary != "" {
		fmt.Fprintf(b, "context: %s\n", cut(c.Summary, 400))
		for _, u := range c.Unknowns {
			fmt.Fprintf(b, "- unknown: %s\n", cut(u, 160))
		}
	}
	b.WriteString("items (area | technology | version | language | driver | confidence | purpose):\n")
	start := b.Len()
	for i, it := range plan.Items {
		fmt.Fprintf(b, "- %s | %s | %s | %s | %s | %s | %s\n", it.Area, it.Technology, it.Version, it.Language, it.Driver, it.Confidence, cut(it.Purpose, 110))
		if b.Len()-start > limit {
			fmt.Fprintf(b, "…(dipotong, %d keputusan lagi)\n", len(plan.Items)-i-1)
			break
		}
	}
	for _, c := range plan.Constraints {
		fmt.Fprintf(b, "- constraint %s: %s\n", strings.Join(c.Areas, ","), cut(c.Rule, 160))
	}
	for _, x := range plan.Compatibility {
		fmt.Fprintf(b, "- compatibility %s [%s] %s\n", x.Area, x.Status, strings.Join(x.Between, ","))
	}
	for _, k := range plan.Evaluation.Criteria {
		fmt.Fprintf(b, "- score %s %d\n", k.Name, k.Score)
	}
}

// CleanTechRequest trims a revision request. Its length is not limited: the
// request is the user's own words, like a chat message.
func CleanTechRequest(text string) (string, error) {
	return strings.TrimSpace(text), nil
}

// TechRevisePrompt asks the AI to revise the current plan: the decisions that
// change, and the review of the whole stack again.
func TechRevisePrompt(p *core.Project) string {
	s := p.Stack
	plan := s.Plan
	var b strings.Builder
	b.WriteString(`You are the technology architect of Foundry. Foundry designs software and writes instructions for an external coding agent; it never implements and you must not write application code or touch files.
Revise the CURRENT TECHNOLOGY PLAN below for the user's request and for the sources that changed since it was written. Change ONLY what the request or a changed source requires; keep everything else. The user decides the technology: never swap a choice the user asked to keep, or one chosen by the user, for your own preference. Choose freely otherwise (outside the catalog is fine) and keep the stack as simple as the requirements allow. Do not invent facts; list what is unknown and what you assumed. Every decision you set is complete (purpose, fulfills, driver, reasoning, pros, cons, tradeoffs, depends_on, integration, operational, alternatives, confidence, assumptions) and a claim of compatibility needs a basis.
Write every human-readable text in the same language as the story.

PROJECT: `)
	b.WriteString(p.Name)
	b.WriteString("\nSTORY:\n")
	b.WriteString(p.Story)
	b.WriteString("\n\n")
	writeBinding(&b, p)
	writeTechContext(&b, p)
	writeTechAnchors(&b, p)
	b.WriteString("CURRENT TECHNOLOGY PLAN:\n")
	writeTechPlan(&b, plan, 60000)
	b.WriteString("\n")
	if areas := plan.Analysis.StaleAreas; len(areas) > 0 {
		fmt.Fprintf(&b, "SOURCES THAT CHANGED since the plan was written: %s. Plan sections affected: %s. Re-check those against the system above and fix what no longer fits.\n\n",
			strings.Join(areas, ", "), strings.Join(plan.Analysis.Affected, ", "))
	}
	if len(plan.Analysis.Diverged) > 0 {
		fmt.Fprintf(&b, "CHANGED BY THE USER since the plan was written (areas %s): the current choice is binding; update the decision, its reasoning and the evaluation for it.\n\n", strings.Join(plan.Analysis.Diverged, ", "))
	}
	if errs := plan.Errors(); len(errs) > 0 {
		b.WriteString("CURRENT PROBLEMS in the plan (fix them too):\n")
		for i, e := range errs {
			if i == 12 {
				break
			}
			fmt.Fprintf(&b, "- %s\n", e.Message)
		}
		b.WriteString("\n")
	}
	if r := s.PlanRequest; r != nil {
		if t := strings.TrimSpace(r.Text); t != "" {
			b.WriteString("USER REQUEST:\n" + t + "\n")
		} else {
			b.WriteString("USER REQUEST: none. Reconcile the plan with the changed sources and evaluate it again.\n")
		}
		if r.Reason != "" {
			b.WriteString("REASON GIVEN BY THE USER:\n" + r.Reason + "\n")
		}
		b.WriteString("\n")
		writeTechRefusal(&b, r)
	}
	b.WriteString(techPlanRules)
	b.WriteString(techplan.Contract(p.Stack))
	b.WriteString(`
Reply with ONLY one JSON object in a ` + "```json" + ` block:
{"set": [ complete decisions to add or replace, same shape as items of a fresh plan ],
 "remove": ["<area to delete>"],
 "meta": {"approach": "", "summary": "", "rationale": ""},
 "context": { optional: the complete technical context again, only when it changed },
 "constraints": [ optional: the complete list again, only when it changed ],
 "compatibility": [{"area": "runtime", "status": "compatible", "between": ["backend", "database"], "note": "...", "basis": "..."}, ... the review of the stack AS IT IS AFTER your changes, every area again],
 "evaluation": {"summary": "...", "criteria": [{"name": "requirement_coverage", "score": 4, "reason": "<concrete>", "evidence": ["backend", "REQ-002"]}, ... one object per criterion, all ` + strconv.Itoa(len(core.TechCriteria)) + `: ` + strings.Join(core.TechCriteria, ", ") + `], "findings": [{"category": "other", "severity": "info", "area": "...", "message": "...", "resolution": "..."}]},
 "consistency": [{"area": "requirements", "status": "consistent", "note": "..."}, ... one object per area: ` + strings.Join(core.TechConsistencyAreas, ", ") + `],
 "note": "<one sentence on what you changed and why>"}
"set" and "remove" may be empty only when nothing in the plan has to change; compatibility, evaluation and consistency are still required. Areas that depend on a change (for example the UI library on a frontend, or testing tools on a language) are yours to update in the same answer. meta fields are only the ones to change.`)
	return b.String()
}

type techRevision struct {
	Set         []core.TechItem                                `json:"set"`
	Remove      []string                                       `json:"remove"`
	Meta        *struct{ Approach, Summary, Rationale string } `json:"meta"`
	Context     *core.TechContext                              `json:"context"`
	Constraints *[]core.TechConstraint                         `json:"constraints"`
	Compat      []core.TechCompat                              `json:"compatibility"`
	Evaluation  *core.TechEvaluation                           `json:"evaluation"`
	Consistency []core.TechCheck                               `json:"consistency"`
	Note        string                                         `json:"note"`
}

// ApplyTechPlan lands a fresh design. It replaces the plan, keeps its
// history, and either becomes the decision at once (it replaces a
// recommendation nobody approved) or waits as a proposal. v is the stack
// version when the decisions changed at once.
func ApplyTechPlan(p *core.Project, raw string) (v *core.TechVersion, err error) {
	s := p.Stack
	if s == nil || s.PlanRequest == nil || s.PlanRequest.Action != core.TechPlanGenerate {
		return nil, errors.New("tidak ada permintaan rancangan teknologi")
	}
	req := s.PlanRequest
	var plan *core.TechPlan
	if techRepairing(p) {
		fixed, res, err := repairTechPlan(p, raw)
		if err != nil {
			return nil, err
		}
		plan = fixed
		p.Log("system", fmt.Sprintf("Perbaikan terarah rencana teknologi lolos validasi: %d masalah selesai, %d area ditulis ulang, %d keputusan valid dipertahankan",
			len(req.Issues), len(res.Repaired), len(plan.Items)-len(res.Repaired)))
	} else {
		plan = &core.TechPlan{}
		if err := ExtractJSONFunc(raw, plan, func() bool { return len(plan.Items) > 0 }); err != nil {
			if body := jsonObjectBytes(raw); len(body) == 0 || unmarshalLoose(body, plan) != nil || len(plan.Items) == 0 {
				return nil, err
			}
		}
		plan.History, plan.Analysis, plan.Stale = nil, core.TechAnalysis{}, false
		if err := techplan.Accept(p, plan); err != nil {
			return nil, err
		}
	}
	if v, err = techplan.Land(p, plan, "generate", strings.TrimSpace(req.Text), req.Reason, ""); err != nil {
		return nil, err
	}
	s.PlanRequest = nil
	return v, nil
}

// ApplyTechRevision lands a revision: the changed decisions and the review
// of the whole stack again, as one new version.
func ApplyTechRevision(p *core.Project, raw string) (v *core.TechVersion, err error) {
	s := p.Stack
	if s == nil || s.Plan == nil || s.PlanRequest == nil || s.PlanRequest.Action != core.TechPlanRevise {
		return nil, errors.New("tidak ada permintaan revisi teknologi")
	}
	var a techRevision
	// "note" alone is not a match: compatibility and consistency entries
	// carry one too.
	match := func() bool {
		return a.Set != nil || a.Remove != nil || a.Evaluation != nil || a.Compat != nil || a.Consistency != nil
	}
	if err := ExtractJSONFunc(raw, &a, match); err != nil {
		return nil, err
	}
	var missing []string
	if len(a.Compat) == 0 {
		missing = append(missing, "compatibility")
	}
	if a.Evaluation == nil {
		missing = append(missing, "evaluation")
	}
	if len(a.Consistency) == 0 {
		missing = append(missing, "consistency")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("jawaban tidak menyertakan %s untuk rencana setelah perubahan; compatibility, evaluation, dan consistency wajib ada", strings.Join(missing, ", "))
	}
	plan := s.Plan.Clone()
	for _, it := range a.Set {
		it.Area = techplan.AreaID(it.Area)
		if i := slices.IndexFunc(plan.Items, func(x core.TechItem) bool { return x.Area == it.Area }); i >= 0 {
			plan.Items[i] = it
		} else {
			plan.Items = append(plan.Items, it)
		}
	}
	for _, area := range a.Remove {
		area = techplan.AreaID(area)
		plan.Items = slices.DeleteFunc(plan.Items, func(x core.TechItem) bool { return x.Area == area })
	}
	if m := a.Meta; m != nil {
		if m.Approach != "" {
			plan.Approach = m.Approach
		}
		if m.Summary != "" {
			plan.Summary = m.Summary
		}
		if m.Rationale != "" {
			plan.Rationale = m.Rationale
		}
	}
	if a.Context != nil {
		plan.Context = *a.Context
	}
	if a.Constraints != nil {
		plan.Constraints = *a.Constraints
	}
	plan.Compatibility, plan.Evaluation, plan.Consistency = a.Compat, *a.Evaluation, a.Consistency
	if err := techplan.Accept(p, plan); err != nil {
		return nil, err
	}
	req := s.PlanRequest
	if v, err = techplan.Land(p, plan, "ai", strings.TrimSpace(req.Text), req.Reason, strings.TrimSpace(a.Note)); err != nil {
		return nil, err
	}
	s.PlanRequest = nil
	return v, nil
}
