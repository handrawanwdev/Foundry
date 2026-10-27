// Package work defines every AI work item Foundry hands out: the prompt
// (what the AI must do) and Apply (how the answer lands in the single project
// context). Agent mode and MCP mode share these functions, so both paths
// produce identical state.
package work

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/standard"
)

const (
	KindDiscovery = "discovery"
	// KindDesign writes the design rules and layout spec, or revises them
	// for the pending request.
	KindDesign = "design"
)

type discoveryOut struct {
	Name         string             `json:"name"`
	Discovery    core.Discovery     `json:"discovery"`
	Blueprint    *core.Blueprint    `json:"blueprint"`
	Requirements []core.Requirement `json:"requirements"`
	Features     []core.Feature     `json:"features"`
	Flows        []core.Flow        `json:"flows"`
	Platform     core.PlatformPlan  `json:"platform"`
	Artifacts    []core.Artifact    `json:"artifacts"`
	Done         []string           `json:"definition_of_done"`
	Decisions    []core.Decision    `json:"decisions"`
	Skip         []core.AreaSkip    `json:"skip"`
}

func DiscoveryPrompt(p *core.Project) string {
	var b strings.Builder
	b.WriteString(`You are the discovery analyst of Foundry, acting as a software architect for any kind of software:
web, mobile, desktop, backend, APIs/SDKs, distributed and real-time systems, data pipelines, infrastructure,
AI/ML systems, embedded/firmware/IoT, systems programming and developer tooling.
Understand the request below BEFORE anything is built. Foundry designs; an external coding agent implements.
Do not write application code. Do not modify files.
Produce a blueprint a developer or AI coding agent can follow, sized to THIS project. Do not design a toy that merely runs,
and do not add parts the project does not need.

` + analysisDomains + `

` + analysisRules + `

` + adaptiveSurveyRules + `

Write every human-readable text in the same language as the story.

STORY:
`)
	b.WriteString(p.Story)
	b.WriteString("\n\n")
	b.WriteString(standard.Rules)
	b.WriteString("\n")
	b.WriteString(surveyPrompt(p))
	if p.Structure != nil && p.Structure.Mode == core.StructureExisting {
		b.WriteString("EXISTING PROJECT COMPONENTS (respect this architecture):\n")
		for _, c := range p.Structure.Components {
			fmt.Fprintf(&b, "- %s at %s (%s) evidence: %s\n", c.Kind, c.Path, c.Tech, c.Evidence)
		}
		b.WriteString("\n")
	}
	b.WriteString(`Platform kinds allowed: web, android, ios, desktop, backend, cli, embedded (firmware, IoT, robotics), other (library, SDK,
data pipeline, ML system, infrastructure, tooling). Choose them from where the software runs. A platform target names no
users: who uses which platform is discovery.roles[].platform, permissions belong to authz and security.
tech_needs lists only the technical needs the product really has, from: realtime, offline, seo, ai, high_concurrency,
flexible_schema, transactions, independent_scaling, enterprise, files, search, webhook. For web, mobile, desktop, cli and backend
parts Foundry picks the technology stack from them; do not choose those frameworks, databases, caches, brokers or API protocols
in the blueprint. For other software
(embedded, cli, other) write the technology constraints (language, toolchain, runtime, hardware) the request names or needs
into an artifact, marked as recommendation when the user did not name them.
When modules exchange domain events, list them (name domain.happened, producer module, consumers) in blueprint.events.events.
Set performance.budget only when the story or the survey gives a reason; otherwise leave it empty.

Blueprint rules:
- modules/ are domain boundaries (identity, the core business, files, audit, …), never folders of file types (controllers/, services/).
- each module owns data, exposes a public interface, and may depend on other modules without cycles.
- do not put large files in the database; describe object storage when the product has uploads, attachments, exports, or media.
- processes that are slow (email, reports, export, image, scan, sync, cron) are workers, not API request work.
- dashboards and reports read summary/read-model, not heavy queries on the transaction tables.
- authentication and authorization sized to the product: who the users are, how they sign in, what each may do (roles, ownership or none). Name permissions resource:action. Do not assume JWT or RBAC when the story does not need them.
- keep the blueprint specific to THIS product; omit unused areas (empty realtime if none, storage.strategy "none" if no files).
- the blueprint object below has web/backend-oriented fields; leave the fields that do not apply empty (no database, auth or
  REST API for firmware, a library or a pipeline) and describe the real design in artifacts instead.

Artifacts: 0-10 extra blueprint documents for what THIS project needs beyond requirements, architecture and screens:
the area artifacts below that apply, plus other documents the project needs (domain model, API or event contracts,
protocol spec, data pipeline design, evaluation plan, test strategy, hardware interfaces). Skip what is not needed.
Do not use the ids requirements, survey-decisions, architecture, technology, design-rules, layout-spec, use-cases, acceptance, decisions, system-overview
(Foundry writes those from the structured data).
` + artifactRules + `
`)
	areaGuide(&b, p)
	b.WriteString(scopeRules)
	b.WriteString(traceabilityRules)
	b.WriteString(consistencyRules)
	b.WriteString(outputValidationRules)
	b.WriteString(`
Reply with ONLY one JSON object in a ` + "```json" + ` block, shape:
{
  "name": "short product name",
  "discovery": {"summary": "", "business": "", "users": [""], "roles": [{"name": "", "description": "", "platform": "web"}],
                "data": ["entity: key fields"], "integrations": [""], "security": [""], "workflows": [""],
                "tech_needs": ["transactions"]},
  "blueprint": {
    "overview": {"goal": "", "users": [""], "capabilities": [""], "core_domain": ""},
    "platform": {"reason": "", "devices": ["web"], "offline": "none|partial|required", "realtime": "none|needed|critical", "deployment": ""},
    "architecture": {"pattern": "", "reason": "", "tradeoff": "", "evolve_when": [""], "scale": ""},
    "modules": [{"id": "identity", "responsibility": "", "data": ["User"], "interface": ["POST /auth/login"], "depends_on": []}],
    "database": {"entities": [{"name": "User", "fields": ["id","email"], "module": "identity", "relates": []}],
                 "transactions": "", "indexing": [""]},
    "storage": {"strategy": "none|object", "types": [""], "security": [""], "processing": [""]},
    "auth": {"method": "", "hashing": "", "tokens": "", "session": "", "refresh": ""},
    "authz": {"model": "RBAC", "convention": "resource:action", "permissions": ["appointment:create"]},
    "realtime": {"needed": false, "events": ["appointment.created"], "retry": ""},
    "workers": [{"name": "email", "trigger": "queue|cron|event", "job": ""}],
    "aggregation": {"needed": false, "approach": "", "reads": [""]},
    "events": {"events": [{"type": "appointment.created", "producer": "janji-temu", "consumers": ["identity"], "durable": true}]},
    "performance": {"budget": "", "health": "GET /health and GET /ready", "pagination": "", "caching": "", "avoid": [""], "indexing": ""},
    "coding": {"backend": [""], "frontend": [""], "api": {"style": "REST", "versioning": "/api/v1", "errors": "", "pagination": ""}}
  },
  "requirements": [{"id": "REQ-001", "text": "", "priority": "MUST|SHOULD|COULD", "acceptance": [` + criterionShape + `], "untestable": ""}],
  "features": [{"id": "FEAT-001", "name": "", "description": "", "requirement_ids": ["REQ-001"], "platforms": ["web","backend"],
                "sub_features": [{"id": "FEAT-001.1", "name": ""}]}],
  "flows": [` + useCaseShape + `],
  "platform": {"targets": [{"kind": "web", "rationale": ""}], "reason": "one short sentence for a non-technical user"},
  "artifacts": [{"id": "threat-model", "title": "", "category": "quality", "area": "security", "summary": "", "body": "Markdown", "refs": []}],
  "definition_of_done": [""],
  "decisions": [` + decisionShape + `],
  "skip": [{"area": "migration", "reason": ""}]
}`)
	return b.String()
}

var validPlatforms = map[string]bool{
	core.PlatformWeb: true, core.PlatformAndroid: true, core.PlatformIOS: true,
	core.PlatformDesktop: true, core.PlatformBackend: true, core.PlatformCLI: true,
	core.PlatformEmbedded: true, core.PlatformOther: true,
}

// ApplyDiscovery validates the AI answer, renumbers IDs and writes the
// discovery layers and the project's artifacts. The UI design is reset
// because it derives from these features; the workflow is kept and its
// commands are reported stale by the workflow check.
func ApplyDiscovery(p *core.Project, raw string) error {
	var out discoveryOut
	if err := ExtractJSON(raw, &out); err != nil {
		return err
	}
	if len(out.Requirements) == 0 || len(out.Features) == 0 {
		return fmt.Errorf("discovery tidak lengkap: requirements/features kosong")
	}
	reqMap := map[string]string{}
	var reqs []core.Requirement
	for i, r := range out.Requirements {
		if strings.TrimSpace(r.Text) == "" {
			continue
		}
		id := fmt.Sprintf("REQ-%03d", i+1)
		reqMap[r.ID] = id
		pr := strings.ToUpper(r.Priority)
		if pr != "MUST" && pr != "SHOULD" && pr != "COULD" {
			pr = "MUST"
		}
		reqs = append(reqs, core.Requirement{ID: id, Text: r.Text, Priority: pr, Acceptance: normCriteria(id, r.Acceptance),
			Untestable: strings.TrimSpace(r.Untestable)})
	}
	featMap := map[string]string{}
	var feats []core.Feature
	for i, f := range out.Features {
		if strings.TrimSpace(f.Name) == "" {
			continue
		}
		id := fmt.Sprintf("FEAT-%03d", i+1)
		featMap[f.ID] = id
		nf := core.Feature{ID: id, Name: f.Name, Description: f.Description}
		for _, r := range f.RequirementIDs {
			if m, ok := reqMap[r]; ok {
				nf.RequirementIDs = append(nf.RequirementIDs, m)
			}
		}
		for _, pl := range f.Platforms {
			if pl = strings.ToLower(pl); validPlatforms[pl] {
				nf.Platforms = append(nf.Platforms, pl)
			}
		}
		for j, s := range f.SubFeatures {
			nf.SubFeatures = append(nf.SubFeatures, core.SubFeature{ID: fmt.Sprintf("%s.%d", id, j+1), Name: s.Name})
		}
		feats = append(feats, nf)
	}
	// Every requirement must reach a feature, otherwise it can never be built.
	// A requirement the AI left out of every feature is kept and linked to
	// the feature that matches it by meaning; only one with no meaningful
	// feature is refused, so the AI retries instead of shipping an orphan.
	covered := map[string]bool{}
	for _, f := range feats {
		for _, r := range f.RequirementIDs {
			covered[r] = true
		}
	}
	for i := range reqs {
		if covered[reqs[i].ID] {
			continue
		}
		if id := core.MapRequirementToFeature(reqs[i], feats); id != "" {
			for j := range feats {
				if feats[j].ID == id {
					feats[j].RequirementIDs = append(feats[j].RequirementIDs, reqs[i].ID)
					covered[reqs[i].ID] = true
					break
				}
			}
		}
	}
	for _, r := range reqs {
		if !covered[r.ID] {
			return fmt.Errorf("%s (%s) tidak terhubung ke feature mana pun", r.ID, r.Text)
		}
	}
	var flows []core.Flow
	for _, fl := range out.Flows {
		fid, ok := featMap[fl.FeatureID]
		if !ok || len(fl.Steps) == 0 {
			continue
		}
		fl = normFlow(fl)
		fl.ID, fl.FeatureID, fl.PageIDs = fmt.Sprintf("FLOW-%03d", len(flows)+1), fid, nil
		flows = append(flows, fl)
	}
	var targets []core.PlatformTarget
	for _, t := range out.Platform.Targets {
		if t.Kind = strings.ToLower(t.Kind); validPlatforms[t.Kind] {
			targets = append(targets, t)
		}
	}
	if len(targets) == 0 {
		return fmt.Errorf("platform tidak ditentukan")
	}
	targets = applySurveyKinds(p.Survey, targets)

	if out.Name != "" {
		p.Name = out.Name
	}
	d := out.Discovery
	p.Discovery = &d
	p.Blueprint = NormalizeBlueprint(out.Blueprint)
	p.Requirements, p.Features, p.Flows = reqs, feats, flows
	p.Platform = &core.PlatformPlan{Targets: targets, Reason: out.Platform.Reason}
	SetArtifacts(p, out.Artifacts)
	p.UI = nil
	p.Done = nonEmpty(out.Done)
	p.Decisions = mergeDecisions(nil, append(cleanDecisions(out.Decisions), SurveyConflicts(p)...))
	p.DecisionSync = nil // the artifacts were written with these decisions
	p.Skips = nil
	for _, sk := range out.Skip {
		skipArea(p, sk.Area, sk.Reason) // an invalid skip is dropped: the area stays expected
	}
	return nil
}

// cleanDecisions drops decisions without text and fixes unknown kinds and
// areas, so a discovery is not refused for its decision register.
func cleanDecisions(in []core.Decision) []core.Decision {
	var out []core.Decision
	for _, d := range in {
		if strings.TrimSpace(d.Text) == "" {
			continue
		}
		d.ID = ""
		if !slices.Contains(core.DecisionKinds, strings.ToLower(strings.TrimSpace(d.Kind))) {
			d.Kind = core.DecisionAssumption
		}
		if core.AreaByID(strings.TrimSpace(d.Area)) == nil {
			d.Area = ""
		}
		out = append(out, d)
	}
	return out
}

func applySurveyKinds(s *core.Survey, targets []core.PlatformTarget) []core.PlatformTarget {
	if s == nil || len(s.Kinds) == 0 {
		return targets
	}
	have := map[string]bool{}
	for _, t := range targets {
		have[t.Kind] = true
	}
	add := func(kind string) {
		if have[kind] {
			return
		}
		have[kind] = true
		targets = append(targets, core.PlatformTarget{Kind: kind, Rationale: "survey"})
	}
	for _, k := range s.Kinds {
		switch strings.ToLower(strings.TrimSpace(k)) {
		case core.SurveyKindWeb:
			add(core.PlatformWeb)
		case core.SurveyKindMobile:
			add(core.PlatformAndroid)
			add(core.PlatformIOS)
		case core.SurveyKindDesktop:
			add(core.PlatformDesktop)
		case core.SurveyKindCLI:
			add(core.PlatformCLI)
		case core.SurveyKindAPI, core.SurveyKindBot:
			add(core.PlatformBackend)
		case core.SurveyKindEmbedded:
			add(core.PlatformEmbedded)
		} // "other" names no kind: the discovery picks it from the story
	}
	return targets
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v) // compact: prompts are paid per token
	return string(b)
}
