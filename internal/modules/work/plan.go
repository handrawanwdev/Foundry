package work

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/repoplan"
	"github.com/handrawanwdev/foundry/internal/modules/structure"
	"github.com/handrawanwdev/foundry/internal/modules/techplan"
)

// KindPlan is Unified Blueprint Planning: technology, architecture and
// folder structure as one AI job with one shared context.
const KindPlan = "plan"

// PlanParts are the design sections a unified job may produce.
func PlanParts(p *core.Project) []string {
	var parts []string
	if p.Stack != nil && p.Stack.PlanRequest != nil {
		parts = append(parts, KindTech)
	}
	if p.Structure != nil && p.Structure.PlanRequest != nil {
		parts = append(parts, KindStructure)
	}
	if p.Structure != nil && p.Structure.RepoRequest != nil {
		parts = append(parts, KindRepo)
	}
	return parts
}

// CurrentPlanPart is the section the next AI round designs. Sections run one
// per round (technology, architecture, repository) so every answer stays
// small and parseable; each round sees the landed sections as frozen context.
func CurrentPlanPart(p *core.Project) string {
	if parts := PlanParts(p); len(parts) > 0 {
		return parts[0]
	}
	return ""
}

func hasPlanPart(parts []string, kind string) bool {
	for _, p := range parts {
		if p == kind {
			return true
		}
	}
	return false
}

// UnifiedPlanPrompt asks for technology, architecture and repository
// structure in one answer. Shared context is written once. Sections the
// project already decided stay frozen and are not redesigned.
func UnifiedPlanPrompt(p *core.Project) string {
	if raw := lastPlanOutput(p); raw != "" && planRejection(p) != "" {
		return unifiedRepairPrompt(p, raw)
	}
	open := PlanParts(p)
	parts := open
	if len(open) > 1 {
		parts = open[:1]
	}
	var b strings.Builder
	b.WriteString(`You are the unified blueprint architect of Foundry. Foundry designs software and writes instructions for an external coding agent; it never implements and you must not write application code or touch files.
Design TECHNOLOGY, ARCHITECTURE and REPOSITORY STRUCTURE as ONE consistent decision. Analyze the system once. Do not repeat the analysis per section. Every technology choice must fit the architecture; every folder must implement that architecture and those technologies. If a choice in one section forces a change in another, resolve it in this answer.
You are free. Foundry's catalog and profiles are a REFERENCE, not a menu. Pick the simplest stack, architecture and folder tree that meet THIS system's needs. No folder, service, module or technology without a named need.
Do not invent facts. When the requirements do not say something that matters, list it in technology.context.unknowns and state the assumption; never present an assumption as a requirement.
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
	if existingProject(p) {
		b.WriteString(unifiedExistingRules)
	}
	writeSystemSnapshot(&b, p, snapshotLean)
	if hasPlanPart(parts, KindStructure) {
		if domains := structure.Domains(p); len(domains) > 0 {
			fmt.Fprintf(&b, "DOMAINS THAT MUST APPEAR IN EVERY ALTERNATIVE (exact ids): %s\n", strings.Join(domains, ", "))
			b.WriteString("Place every id in exactly one service or module. An alternative that misses one domain is invalid.\n\n")
		}
		writeStructureBrief(&b, p, "CURRENT STRUCTURE (heuristic before this analysis; you may recommend a better fit)")
	}
	if hasPlanPart(parts, KindRepo) {
		writeRepoAnchors(&b, p)
		if p.Structure != nil && p.Structure.IR != nil {
			b.WriteString("If you recommend a different architecture alternative, the repository must implement THAT alternative's parts, not the current heuristic folders. Every part of the recommended alternative maps to exactly one folder.\n\n")
		}
	}
	writeUnifiedFrozen(&b, p, open)
	writeUnifiedRefusal(&b, p)
	if len(open) > 1 {
		fmt.Fprintf(&b, "This blueprint is designed in %d sequential rounds (%s). This round: %s only. Decisions already landed are FROZEN above; later sections are designed next with your answer as input. Do not produce later sections.\n\n", len(open), strings.Join(sectionLabels(open), ", "), sectionLabels(parts)[0])
	}
	fmt.Fprintf(&b, "SECTIONS TO PRODUCE: %s\n\n", strings.Join(sectionLabels(parts), ", "))
	if hasPlanPart(parts, KindTech) {
		b.WriteString("TECHNOLOGY RULES:\n")
		b.WriteString(techPlanRules)
		b.WriteString("\n")
		if existingProject(p) {
			b.WriteString("For an existing project every technology item sets action: reuse | add | modify | refactor | remove. Keep existing technologies as reuse unless the story or the user asked to change them.\n\n")
		}
	}
	if hasPlanPart(parts, KindStructure) {
		b.WriteString(unifiedStructureRules)
	}
	if hasPlanPart(parts, KindRepo) {
		b.WriteString(unifiedRepoRules)
	}
	b.WriteString("Reply with ONLY JSON in ```json fences. No prose before or after. No trailing commas. Omit a top-level key you were not asked to produce. Do not invent JSON keys. One technology item per area the system actually needs; omit unused areas. Keep each string to one sentence, no line breaks inside strings, escape quotes. Keep the answer compact: no filler nodes, no duplicated text.\n")
	b.WriteString("Preferred: one object with those keys. Also accepted: one complete json block per section, in order, each the inner object (not wrapped in the key name).\n")
	b.WriteString("{\n")
	needComma := false
	writeKey := func(line string) {
		if needComma {
			b.WriteString(",\n")
		}
		b.WriteString(line)
		needComma = true
	}
	if hasPlanPart(parts, KindTech) {
		writeKey(`  "technology": { "approach": "...", "summary": "...", "rationale": "...", "context": {"summary":"...","facts":[],"unknowns":[],"assumptions":[]}, "items": [{"area":"backend","group":"stack","technology":"...","purpose":"...","fulfills":["REQ-001"],"driver":"requirement","reasoning":"...","pros":["..."],"cons":["..."],"tradeoffs":["..."],"depends_on":["database"],"integration":"...","operational":"...","alternatives":[{"technology":"...","why":"...","difference":"..."}],"confidence":"medium"` + existingActionHint(p) + `}], "compatibility": [{"area":"runtime","status":"compatible","between":["backend","database"],"note":"...","basis":"..."}], "evaluation": {"summary":"...","criteria":[{"name":"requirement_coverage","score":4,"reason":"...","evidence":["backend"]}], "findings":[]}, "consistency": [{"area":"requirements","status":"consistent","note":"..."}] }`)
	}
	if hasPlanPart(parts, KindStructure) {
		writeKey(`  "architecture": { "summary": "...", "drivers": ["..."], "domains": [{"name":"booking","responsibility":"..."}], "dependency_rules": ["..."], "recommendation": "recommended", "reason": "...", "alternatives": [{"id":"recommended","name":"...","summary":"...","type":"modular","ir":{"type":"modular","apps":[{"name":"web","reason":"..."}],"services":[{"name":"api","reason":"...","modules":["booking"],"module_reasons":{"booking":"..."}}],"workers":[],"packages":[],"docs":"docs/","rules":["..."]},"evaluation":{"maintainability":5,"simplicity":5,"extensibility":3,"testability":4,"separation_of_concerns":4,"complexity":2}}] }`)
	}
	if hasPlanPart(parts, KindRepo) {
		writeKey(`  "repository": { "approach": "...", "topology": "...", "summary": "...", "rationale": "...", "nodes": [{"path":"apps/","type":"dir","role":"source","responsibility":"...","owner":"platform","reason":"...","allowed":["..."],"forbidden":["..."]}], "modules": [{"module":"booking","locations":[{"path":"apps/api/modules/booking/","role":"api"}],"frontend":true,"backend":true}], "dependency": {"direction":"...","allowed":[{"from":"booking","to":"shared","reason":"..."}],"forbidden":[{"from":"shared","to":"booking","reason":"..."}],"communication":[],"interfaces":[],"shared_rules":["..."],"cycle_guard":"..."}, "rules": ["..."], "evaluation": {"summary":"...","criteria":[{"name":"cohesion","score":4,"reason":"..."}],"findings":[]}, "consistency": [{"area":"project_modules","status":"consistent","note":"..."}], "decisions": [{"title":"...","choice":"...","reason":"...","alternatives":["..."],"tradeoff":"..."}] }`)
	}
	if existingProject(p) {
		writeKey(`  "adaptation": [{"action":"reuse","target":"existing module or path","module":"id","reason":"...","files":["path"]}]`)
	}
	b.WriteString("\n}")
	return b.String()
}

func existingActionHint(p *core.Project) string {
	if existingProject(p) {
		return `,"action":"reuse"`
	}
	return ""
}

func sectionLabels(parts []string) []string {
	var out []string
	for _, p := range parts {
		switch p {
		case KindTech:
			out = append(out, "technology")
		case KindStructure:
			out = append(out, "architecture")
		case KindRepo:
			out = append(out, "repository")
		}
	}
	return out
}

func writeUnifiedFrozen(b *strings.Builder, p *core.Project, parts []string) {
	if !hasPlanPart(parts, KindTech) && p.Stack != nil && p.Stack.Plan != nil {
		b.WriteString("FROZEN TECHNOLOGY (do not change; architecture and folders must follow it):\n")
		fmt.Fprintf(b, "- approach: %s\n- %s\n\n", p.Stack.Plan.Approach, p.Stack.Plan.Summary)
		for _, it := range p.Stack.Plan.Items {
			if it.None() {
				continue
			}
			fmt.Fprintf(b, "- %s: %s\n", it.Area, it.Technology)
		}
		b.WriteByte('\n')
	}
	if !hasPlanPart(parts, KindStructure) && p.Structure != nil && p.Structure.Plan != nil {
		writeStructureBrief(b, p, "FROZEN ARCHITECTURE (do not change; the folder plan must implement it)")
	}
	if !hasPlanPart(parts, KindRepo) && p.Structure != nil && p.Structure.Repo != nil {
		fmt.Fprintf(b, "FROZEN FOLDER PLAN (do not change): approach %s; topology %s\n\n", p.Structure.Repo.Approach, p.Structure.Repo.Topology)
	}
}

func writeUnifiedRefusal(b *strings.Builder, p *core.Project) {
	reason := planRejection(p)
	if reason == "" {
		return
	}
	b.WriteString("YOUR LAST ANSWER WAS REFUSED, fix this: ")
	b.WriteString(reason)
	b.WriteString("\n\n")
}

func planRejection(p *core.Project) string {
	switch CurrentPlanPart(p) {
	case KindTech:
		return p.Stack.PlanRequest.Rejected
	case KindStructure:
		return p.Structure.PlanRequest.Rejected
	case KindRepo:
		return p.Structure.RepoRequest.Rejected
	}
	return ""
}

func lastPlanOutput(p *core.Project) string {
	if p == nil || p.Dir == "" {
		return ""
	}
	b, err := os.ReadFile(filepath.Join(p.Dir, core.StateDir, "ai-raw", KindPlan+".txt"))
	if err != nil {
		return ""
	}
	return string(b)
}

func unifiedRepairPrompt(p *core.Project, raw string) string {
	var b strings.Builder
	b.WriteString(`You are the unified blueprint architect of Foundry. The previous JSON failed to parse. Repair JSON syntax only: extra characters, mismatched brackets, trailing commas, extra quotes, truncated closers, unescaped text. Do not change any technology, architecture, or folder decision.
Error: `)
	b.WriteString(planRejection(p))
	b.WriteString("\nSECTIONS TO PRODUCE: ")
	b.WriteString(strings.Join(sectionLabels(PlanParts(p)[:1]), ", "))
	b.WriteString("\nReply with ONLY the complete repaired JSON in one ```json fence. No prose.\n\nPREVIOUS OUTPUT:\n")
	b.WriteString(raw)
	return b.String()
}

const unifiedExistingRules = `EXISTING PROJECT — classify every recommendation:
- action is one of reuse, add, modify, refactor, remove.
- reuse what still holds. add only what the need requires. modify the smallest surface. refactor only when the current shape blocks the need. remove only with a dependency reason.
- do not replace a technology, architecture pattern or top-level layout without a reason the story or the user gave.
- adaptation lists architecture and folder actions; technology items carry their own action.

`

const unifiedStructureRules = `ARCHITECTURE RULES:
- design 2-3 complete alternatives that genuinely differ in a trade-off, not in names.
- type is simple | modular | monorepo | microservices and must agree with the technology architecture and repository decisions in this same answer.
- apps must match the target platform: web, mobile (android/ios), desktop, cli.
- a backend platform needs exactly one API service unless technology architecture is Microservices; then one service per independently deployed domain.
- services and modules are business domain boundaries. Put every domain in exactly one place in EVERY alternative.
- score maintainability, simplicity, extensibility, testability, separation_of_concerns 1-5 (5 best) and complexity 1 (simple) to 5 (complex).
- recommendation is the alternative that best fits THIS system. Prefer fewer moving parts when the difference is small.

`

const unifiedRepoRules = `REPOSITORY RULES:
- nodes is a flat list; path is relative, forward slashes, a folder ends with "/". List folders a developer needs; list files only when their role must be pinned (entry points, public interfaces, contracts, configuration, migrations, test setup, build files).
- every node: type (dir|file), responsibility, owner, reason, role. Folders also: allowed, forbidden, relations, boundary.
- owner is a project module id, "shared", or "platform".
- part links a folder to an architecture part (app:web, service:api, module:booking, ...). Every part of the recommended architecture alternative maps to exactly one folder.
- modules has one entry for EVERY project module.
- dependency rules use exact module ids, "shared", "platform" or a node path. No "*", "all", or invented groups.
- evaluation scores cohesion, coupling, ownership, maintainability, discoverability, testability, extensibility, simplicity (5 best) and complexity (1 simple, 5 complex).
- consistency checks architecture_identity, project_modules, folder_responsibility, technology, dependency_rules, architecture_rules, structure_decision.

`

// UnifiedAnswer is the parsed unified planning JSON.
type UnifiedAnswer struct {
	Technology   json.RawMessage   `json:"technology"`
	Architecture json.RawMessage   `json:"architecture"`
	Repository   json.RawMessage   `json:"repository"`
	Adaptation   []core.ChangeItem `json:"adaptation"`
}

func unifiedMatch(ans *UnifiedAnswer) bool {
	return ans.has(KindTech) || ans.has(KindStructure) || ans.has(KindRepo) || len(ans.Adaptation) > 0
}

// ParseUnified extracts the unified planning object from AI output.
// It accepts one wrapper object, a truncated object that can be closed,
// or separate JSON blocks per section (technology / architecture / repository).
func ParseUnified(raw string) (UnifiedAnswer, error) {
	var ans UnifiedAnswer
	match := func() bool { return unifiedMatch(&ans) }
	if err := ExtractJSONFunc(raw, &ans, match); err == nil {
		return ans, nil
	}
	ans = UnifiedAnswer{}
	if err := extractClosedJSON(raw, &ans, match); err == nil {
		return ans, nil
	}
	return parseUnifiedParts(raw)
}

func parseUnifiedParts(raw string) (UnifiedAnswer, error) {
	var ans UnifiedAnswer
	chunks := fenceBodies(raw)
	if len(chunks) == 0 {
		chunks = topLevelJSONObjects(raw)
	}
	for _, c := range chunks {
		mergeUnifiedChunk(&ans, []byte(c))
	}
	if !unifiedMatch(&ans) {
		return UnifiedAnswer{}, DiagnoseJSON(raw)
	}
	return ans, nil
}

func mergeUnifiedChunk(ans *UnifiedAnswer, raw []byte) {
	closed := closeJSON(string(raw))
	raw = []byte(closed)
	var wrap UnifiedAnswer
	if json.Unmarshal(raw, &wrap) == nil && unifiedMatch(&wrap) {
		if wrap.has(KindTech) {
			ans.Technology = wrap.Technology
		}
		if wrap.has(KindStructure) {
			ans.Architecture = wrap.Architecture
		}
		if wrap.has(KindRepo) {
			ans.Repository = wrap.Repository
		}
		if len(wrap.Adaptation) > 0 {
			ans.Adaptation = wrap.Adaptation
		}
		return
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil {
		return
	}
	switch {
	case len(m["alternatives"]) > 2:
		ans.Architecture = json.RawMessage(append([]byte(nil), raw...))
	case len(m["nodes"]) > 2:
		ans.Repository = json.RawMessage(append([]byte(nil), raw...))
	case len(m["items"]) > 2:
		ans.Technology = json.RawMessage(append([]byte(nil), raw...))
	}
}

func topLevelJSONObjects(raw string) []string {
	b := []byte(raw)
	var out []string
	for i := bytes.IndexByte(b, '{'); i >= 0; {
		var msg json.RawMessage
		dec := json.NewDecoder(bytes.NewReader(b[i:]))
		if err := dec.Decode(&msg); err != nil {
			closed := closeJSON(string(b[i:]))
			if json.Unmarshal([]byte(closed), &msg) == nil {
				out = append(out, closed)
			}
			break
		}
		out = append(out, string(msg))
		n := int(dec.InputOffset())
		if n <= 0 {
			break
		}
		next := bytes.IndexByte(b[i+n:], '{')
		if next < 0 {
			break
		}
		i += n + next
	}
	return out
}

func rawObject(m json.RawMessage) bool {
	s := bytes.TrimSpace(m)
	return len(s) > 2 && s[0] == '{'
}

func (a UnifiedAnswer) has(kind string) bool {
	switch kind {
	case KindTech:
		return rawObject(a.Technology)
	case KindStructure:
		return rawObject(a.Architecture)
	case KindRepo:
		return rawObject(a.Repository)
	}
	return false
}

// Has reports whether the answer includes a complete section for kind.
func (a UnifiedAnswer) Has(kind string) bool { return a.has(kind) }

func (a UnifiedAnswer) raw(kind string) string {
	switch kind {
	case KindTech:
		return string(a.Technology)
	case KindStructure:
		return string(a.Architecture)
	case KindRepo:
		return string(a.Repository)
	}
	return ""
}

// ApplyUnifiedTech lands the technology section. v is the stack version when
// decisions changed at once.
func (a UnifiedAnswer) ApplyUnifiedTech(p *core.Project) (*core.TechVersion, error) {
	if p.Stack == nil || p.Stack.PlanRequest == nil {
		return nil, errors.New("tidak ada permintaan rencana teknologi")
	}
	if !a.has(KindTech) {
		return nil, errors.New("jawaban unified tidak berisi bagian technology")
	}
	return ApplyTechPlan(p, a.raw(KindTech))
}

// ApplyUnifiedStructure lands the architecture section.
func (a UnifiedAnswer) ApplyUnifiedStructure(p *core.Project) error {
	if p.Structure == nil || p.Structure.PlanRequest == nil {
		return errors.New("tidak ada permintaan analisis struktur")
	}
	if !a.has(KindStructure) {
		return errors.New("jawaban unified tidak berisi bagian architecture")
	}
	return ApplyStructurePlan(p, a.raw(KindStructure))
}

// ApplyUnifiedRepo lands the repository section.
func (a UnifiedAnswer) ApplyUnifiedRepo(p *core.Project) error {
	if p.Structure == nil || p.Structure.RepoRequest == nil {
		return errors.New("tidak ada permintaan struktur repository")
	}
	if !a.has(KindRepo) {
		return errors.New("jawaban unified tidak berisi bagian repository")
	}
	apply := ApplyRepoPlan
	if p.Structure.RepoRequest.Action == core.RepoRevise {
		apply = ApplyRepoRevision
	}
	return apply(p, a.raw(KindRepo))
}

// LandAdaptation stores classified existing-project actions.
func (a UnifiedAnswer) LandAdaptation(p *core.Project) {
	if p == nil || p.Structure == nil || len(a.Adaptation) == 0 {
		return
	}
	plan := core.NormalizeChangePlan(&core.ChangePlan{Items: a.Adaptation})
	if plan == nil {
		p.Structure.Adaptation = nil
		return
	}
	p.Structure.Adaptation = plan.Items
}

// SyncUnifiedSources marks co-designed plans current so none go stale from
// the other sections of the same answer.
func SyncUnifiedSources(p *core.Project) {
	if p.Stack != nil && p.Stack.Plan != nil {
		techplan.MarkCurrent(p, p.Stack.Plan)
		p.Stack.Plan.Stale = false
	}
	if p.Structure != nil && p.Structure.Plan != nil {
		p.Structure.Plan.SourceHash = structure.SourceHash(p)
		p.Structure.Plan.Stale = false
	}
	if p.Structure != nil && p.Structure.Repo != nil {
		repoplan.MarkCurrent(p, p.Structure.Repo)
		p.Structure.Repo.Stale = false
	}
}
