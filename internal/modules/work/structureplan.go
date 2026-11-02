package work

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/structure"
)

// KindStructure (structurechange.go) covers two jobs of the structure
// workspace: a natural-language change of the current structure, and an AI
// analysis that compares complete alternatives. The pending request decides
// which prompt and which Apply runs.

// StructurePlanPrompt asks the AI to analyze the whole system, design 2-3
// complete project structures and compare them before recommending one.
func StructurePlanPrompt(p *core.Project) string {
	var b strings.Builder
	b.WriteString(`You are the software architect of Foundry. Foundry designs software and writes instructions for an external coding agent; it never implements.
Analyze the WHOLE system below (business domain, requirements, features and their relationships, use cases, platform, architecture blueprint, data, security, operations and the approved technology), then design 2-3 complete project structure alternatives that genuinely fit this system.
Do not start from a fixed folder template and do not force Clean Architecture, Microservices, Monorepo, Modular Monolith or any named approach. Choose the simplest structure a developer can understand and maintain, with clear responsibilities, controlled dependencies and no folder that has no reason to exist.
Do not change the technology, the business requirements, the domain boundaries or the platform. A structure that needs a different architecture or repository is not a valid alternative here.
Write every human-readable text in the same language as the story.

PROJECT: `)
	b.WriteString(p.Name)
	b.WriteString("\nSTORY:\n")
	b.WriteString(p.Story)
	b.WriteString("\n\n")
	writeBinding(&b, p)
	writeSystemSnapshot(&b, p)
	if domains := structure.Domains(p); len(domains) > 0 {
		fmt.Fprintf(&b, "DOMAINS THAT MUST APPEAR IN EVERY ALTERNATIVE (exact ids): %s\n", strings.Join(domains, ", "))
		b.WriteString("Place every id in exactly one service or module, with its responsibility. An alternative that misses one domain is invalid and will be sent back to you once with the concrete error.\n\n")
	}
	writeStructureBrief(&b, p, "CURRENT STRUCTURE (the decision before this analysis)")
	if plan := p.Structure.Plan; plan != nil {
		b.WriteString("PREVIOUS ANALYSIS (revise it, keep what still holds):\n")
		fmt.Fprintf(&b, "- summary: %s\n- recommendation: %s — %s\n", plan.Summary, plan.Recommendation, plan.Reason)
		for _, a := range plan.Alternatives {
			fmt.Fprintf(&b, "- alternative %s (%s): %s\n", a.ID, a.Name, a.Summary)
		}
		b.WriteString("\n")
	}
	if r := p.Structure.PlanRequest; r != nil && strings.TrimSpace(r.Text) != "" {
		b.WriteString("USER REVISION REQUEST:\n" + strings.TrimSpace(r.Text) + "\n\n")
	}
	if r := p.Structure.PlanRequest; r != nil && r.Rejected != "" {
		b.WriteString("YOUR LAST ANSWER WAS REFUSED, fix this: " + r.Rejected + "\n\n")
	}
	b.WriteString(`Rules for every alternative:
- type must be one of: simple | modular | monorepo | microservices, and must agree with the approved architecture and repository decisions.
- apps must match the target platform exactly: web, mobile (android/ios), desktop, cli. Do not invent a platform the product does not have.
- a backend platform needs exactly one API service unless the technology decision is Microservices; then one service per domain that is deployed and scaled independently.
- services and modules are business domain boundaries. Put every domain from the blueprint in exactly one place (a service or a module), in EVERY alternative. When the platform has no backend, keep the domains as modules of the single service that owns the app's data; never drop a domain, rename it away from its id, or turn it into a package. Never use controllers/, services/ or other file-type folders as module boundaries, and never use CRUD names (create-booking).
- every app, service, module, worker and package needs a concrete reason/responsibility in this system. Module reasons go in the service's module_reasons map.
- add packages/ only when code is really shared by at least two consumers. Add workers only when the blueprint has background jobs and the technology uses a separate worker.
- the rules list contains the internal module layout and conventions; dependency_rules contains only the allowed dependencies between parts (what may import what, and what must go through a public interface or event). Keep both short and checkable.
- alternatives must differ in a real trade-off (fewer folders vs clearer isolation, one backend vs separate services, shared packages vs local code), not in names. Never return the same folder tree twice.
- score each alternative 1-5 (5 best) for maintainability, simplicity, extensibility, testability and separation_of_concerns; score complexity 1 (simple) to 5 (complex). Be honest: the recommendation is not automatically the most layered or the biggest one.
- the recommendation is the alternative that best balances maintainability, simplicity, separation of concerns, testability and controlled dependencies for THIS system. Prefer fewer moving parts when the difference is small.

Reply with ONLY one JSON object in a ` + "```json" + ` block:
{
  "summary": "<what the system is and how its domains relate, in one short paragraph>",
  "drivers": ["<the system characteristics that decide the structure>"],
  "domains": [{"name": "booking", "responsibility": "<what this domain owns>"}],
  "dependency_rules": ["<a rule every alternative must follow>"],
  "recommendation": "<id of the recommended alternative>",
  "reason": "<why this alternative is the best fit, in Indonesian>",
  "alternatives": [
    {"id": "recommended", "name": "<short name>", "summary": "<the idea in one sentence>", "type": "modular",
     "ir": {"type": "modular",
       "apps": [{"name": "web", "reason": "<why this app exists>"}],
       "services": [{"name": "api", "reason": "<why one API>", "modules": ["booking"], "module_reasons": {"booking": "<what the booking module owns>"}}],
       "workers": [], "packages": [{"name": "contracts", "reason": "<what is shared and by whom>"}],
       "docs": "docs/", "rules": ["<internal module layout or convention>"]},
     "evaluation": {"maintainability": 4, "simplicity": 5, "extensibility": 3, "testability": 4, "separation_of_concerns": 4, "complexity": 2},
     "pros": ["<concrete advantage>"], "cons": ["<concrete cost>"], "tradeoffs": ["<what is given up>"]},
    {"id": "alternative-2", "name": "<short name>", "summary": "<the idea in one sentence>", "type": "...", "ir": {}, "evaluation": {}, "pros": [], "cons": [], "tradeoffs": []}
  ]
}
Return 2 or 3 alternatives. Paths are derived by Foundry from names, so do not invent them.`)
	return b.String()
}

// writeSystemSnapshot writes the whole system the structure must fit. It is
// shared by the analysis and the revision prompts.
func writeSystemSnapshot(b *strings.Builder, p *core.Project) {
	if d := p.Discovery; d != nil {
		fmt.Fprintf(b, "DISCOVERY SUMMARY: %s\nBUSINESS: %s\n", d.Summary, d.Business)
		if len(d.Users) > 0 {
			fmt.Fprintf(b, "USERS: %s\n", strings.Join(d.Users, ", "))
		}
		for _, r := range d.Roles {
			fmt.Fprintf(b, "- role %s (%s): %s\n", r.Name, r.Platform, r.Description)
		}
		if len(d.Data) > 0 {
			fmt.Fprintf(b, "DATA: %s\n", strings.Join(d.Data, "; "))
		}
		if len(d.Integrations) > 0 {
			fmt.Fprintf(b, "INTEGRATIONS: %s\n", strings.Join(d.Integrations, "; "))
		}
		if len(d.Security) > 0 {
			fmt.Fprintf(b, "SECURITY: %s\n", strings.Join(d.Security, "; "))
		}
		if len(d.Workflows) > 0 {
			fmt.Fprintf(b, "WORKFLOWS: %s\n", strings.Join(d.Workflows, "; "))
		}
		if len(d.TechNeeds) > 0 {
			fmt.Fprintf(b, "TECH NEEDS: %s\n", strings.Join(d.TechNeeds, ", "))
		}
		b.WriteString("\n")
	}
	if bp := p.Blueprint; bp != nil {
		if bp.Overview.Goal != "" || bp.Overview.CoreDomain != "" {
			fmt.Fprintf(b, "BLUEPRINT: goal %s; core domain %s\n", bp.Overview.Goal, bp.Overview.CoreDomain)
		}
		if bp.Architecture.Pattern != "" {
			fmt.Fprintf(b, "ARCHITECTURE: %s — %s\n", bp.Architecture.Pattern, bp.Architecture.Reason)
		}
		if len(bp.Modules) > 0 {
			b.WriteString("DOMAIN MODULES (id | responsibility | data | depends on):\n")
			for _, m := range bp.Modules {
				fmt.Fprintf(b, "- %s | %s | %s | %s\n", m.ID, m.Responsibility,
					strings.Join(m.Data, ", "), strings.Join(m.DependsOn, ", "))
			}
		}
		if len(bp.Database.Entities) > 0 {
			b.WriteString("ENTITIES:\n")
			for _, e := range bp.Database.Entities {
				fmt.Fprintf(b, "- %s (module %s): %s\n", e.Name, e.Module, strings.Join(e.Fields, ", "))
			}
		}
		if len(bp.Workers) > 0 {
			b.WriteString("WORKERS:\n")
			for _, w := range bp.Workers {
				fmt.Fprintf(b, "- %s [%s]: %s\n", w.Name, w.Trigger, w.Job)
			}
		}
		if bp.Authz.Model != "" {
			fmt.Fprintf(b, "AUTHORIZATION: %s (%s)\n", bp.Authz.Model, strings.Join(bp.Authz.Permissions, ", "))
		}
		if bp.Platform.Deployment != "" {
			fmt.Fprintf(b, "OPERATIONS: deployment %s\n", bp.Platform.Deployment)
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
	if st := p.Stack; st != nil {
		b.WriteString("TECHNOLOGY DECISION (fixed; the structure must follow it):\n")
		for _, d := range st.Decisions {
			fmt.Fprintf(b, "- %s: %s", d.Area, d.Choice())
			if l := d.Lang(); l != "" {
				fmt.Fprintf(b, " (language %s)", l)
			}
			fmt.Fprintf(b, " — %s\n", d.Reason)
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
		b.WriteString("FEATURES AND RELATIONSHIPS:\n")
		for _, f := range p.Features {
			fmt.Fprintf(b, "- %s %s: %s (requirements %s; platforms %s)\n", f.ID, f.Name, f.Description,
				strings.Join(f.RequirementIDs, ", "), strings.Join(f.Platforms, ", "))
			for _, s := range f.SubFeatures {
				fmt.Fprintf(b, "  - %s %s\n", s.ID, s.Name)
			}
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
	if len(p.Artifacts) > 0 {
		b.WriteString("BLUEPRINT DOCUMENTS (domain, security, API, operations and other rules this structure must fit):\n")
		for _, a := range p.Artifacts {
			body := a.Body
			if len(body) > 2000 {
				body = strings.ToValidUTF8(body[:2000], "") + "\n…(dipotong)"
			}
			fmt.Fprintf(b, "=== %s (%s) %s\n%s\n", a.ID, a.Title, a.Summary, body)
		}
		b.WriteString("\n")
	}
	if len(p.Done) > 0 {
		b.WriteString("DEFINITION OF DONE:\n")
		for _, d := range p.Done {
			fmt.Fprintf(b, "- %s\n", d)
		}
		b.WriteString("\n")
	}
}

// writeStructureBrief writes an approved/current structure as source of
// truth for other prompts: tree, responsibility of every folder and the
// dependency rules.
func writeStructureBrief(b *strings.Builder, p *core.Project, title string) {
	s := p.Structure
	if s == nil || s.Mode == core.StructureExisting || s.IR == nil {
		return
	}
	if title == "" {
		title = "PROJECT STRUCTURE"
	}
	ir := s.IR.Clone()
	structure.Normalize(ir)
	fmt.Fprintf(b, "%s (type %s; source of truth for folders and dependencies; the coding agent must not invent another):\n", title, ir.Type)
	b.WriteString("```text\n")
	b.WriteString(structure.Tree(ir))
	b.WriteString("```\n")
	for _, ps := range [][]core.StructurePart{ir.Apps, ir.Services, ir.Workers, ir.Packages} {
		for _, part := range ps {
			fmt.Fprintf(b, "- %s: %s\n", part.Path, part.Reason)
			for _, m := range part.Modules {
				fmt.Fprintf(b, "  - %s%s: %s\n", part.Path, m, part.ModuleReasons[m])
			}
		}
	}
	if len(ir.Rules) > 0 {
		b.WriteString("Structure and dependency rules:\n")
		for _, r := range ir.Rules {
			fmt.Fprintf(b, "- %s\n", r)
		}
	}
	b.WriteString("\n")
}

// ApplyStructurePlan records the AI analysis. It does not change the
// structure decision: the user adopts one alternative.
func ApplyStructurePlan(p *core.Project, raw string) error {
	s := p.Structure
	if s == nil || s.PlanRequest == nil {
		return errors.New("tidak ada permintaan analisis struktur")
	}
	var plan core.StructurePlan
	if err := ExtractJSONFunc(raw, &plan, func() bool { return len(plan.Alternatives) > 0 }); err != nil {
		return err
	}
	if err := structure.NormalizePlan(p, &plan); err != nil {
		return err
	}
	plan.GeneratedAt = time.Now().UTC()
	plan.SourceHash = s.PlanRequest.SourceHash
	if plan.SourceHash == "" {
		plan.SourceHash = structure.SourceHash(p)
	}
	plan.Stale = false
	plan.Selected, plan.Decision, plan.DecidedAt = "", "", time.Time{}
	s.Plan = &plan
	s.PlanRequest = nil
	s.Recommended = plan.RecommendedAlternative().IR.Type
	s.Reason = plan.Reason
	return nil
}

// PlanAlternativeIR is exported for the pipeline when it adopts an
// alternative; it keeps the merge rule in one place.
func PlanAlternativeIR(plan *core.StructurePlan, a *core.StructureAlternative) *core.StructureIR {
	return structure.AlternativeIR(plan, a)
}
