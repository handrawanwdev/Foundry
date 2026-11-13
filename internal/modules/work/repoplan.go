package work

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/repoplan"
)

// KindRepo covers two jobs of the repository plan: a fresh design by the AI
// and a revision of the current plan. Structure.RepoRequest decides which
// prompt runs and which Apply lands the answer.
const KindRepo = "repo"

// MaxRepoRequest bounds a natural-language revision request.
const MaxRepoRequest = 6000

// RepoPlanPrompt asks the AI to design the whole repository structure of
// the system: topology, folders and files with their responsibilities, the
// place of every module, the dependency architecture, an honest quality
// review and the consistency checks.
func RepoPlanPrompt(p *core.Project) string {
	var b strings.Builder
	b.WriteString(`You are the repository architect of Foundry. Foundry designs software and writes instructions for an external coding agent; it never implements and you must not write application code or touch files.
Design how the SOURCE CODE of the system below is organized: repository topology, root folders, application and service boundaries, module folders, shared code, configuration, tests, documentation and infrastructure, down to the files whose responsibility needs to be written down.
You are free to choose the organization approach: feature-based, layered, domain-driven, modular, monorepo, microservices, a mix, or something of your own. Names such as Clean Architecture or Feature-Based are references, not a menu; name your own approach (approach) and justify it by what THIS system needs. Pick the simplest structure that keeps responsibilities clear, dependencies controlled and ownership obvious. Do not create a folder without a reason, or a file only to follow a convention. Not every project needs every category (no infrastructure folder for a script, no frontend for a service).
Do not change the technology, the requirements, the domain boundaries, the platform, or the architecture decision. Follow the approved technology: file names, build files and conventions must belong to the chosen languages and frameworks.
Write every human-readable text in the same language as the story.

PROJECT: `)
	b.WriteString(p.Name)
	b.WriteString("\nSTORY:\n")
	b.WriteString(p.Story)
	b.WriteString("\n\n")
	writeBinding(&b, p)
	writeSystemSnapshot(&b, p, snapshotLean)
	writeRepoAnchors(&b, p)
	writeStructureBrief(&b, p, "ARCHITECTURE STRUCTURE DECISION (the parts the repository must implement; its dependency rules are binding)")
	writeRepoRevisionContext(&b, p)
	b.WriteString(`Rules:
- nodes is a flat list; path is relative to the repository root, forward slashes, a folder ends with "/". Every folder on a path is its own node, listed before what it contains. Describe folders down to the level a developer needs to know where code goes; list files only when their role must be pinned down (entry points, public interfaces, contracts, configuration, migrations, test setup, build files). Never list generated, trivial or obvious files, and never enumerate every endpoint or CRUD file. A typical project needs 40-250 nodes.
- every node: type (dir|file), responsibility (what it does, one sentence), owner, reason (why it exists in THIS system), role (free text such as source, test, config, docs, infra), depends_on (node paths or module ids it may use).
- every folder also: allowed (the kinds of code that belong), forbidden (the kinds that must not be placed there), relations (how it relates to other modules or folders) and boundary (where its ownership ends).
- owner is a project module id (exact id from PROJECT MODULES) when the node holds that module's code, otherwise "shared" (code several modules use; list them in shared_by and give the reason) or "platform" (build, configuration, tooling, documentation, infrastructure, application shells). Never mix a business module with a technical folder: a module's code lives under that module's folders.
- part links a folder (never a file) to a part of the architecture structure decision, by exact ref (app:web, service:api, module:booking, worker:x, package:contracts). Every part listed under STRUCTURE PARTS must be linked to exactly one folder — the folder that implements that architecture part (for a module, its backend/domain folder from STRUCTURE PARTS). A project module that also has a frontend lists that UI folder in modules[].locations with role "ui"; do not put part:"module:…" on the UI folder (the UI belongs to the app: part). You may place a part somewhere other than its default folder when the design needs it; the plan then replaces the default.
- modules has one entry for EVERY project module: its locations (FOLDERS, never files, with their role: ui, api, domain, data, test, ...) and whether it needs a frontend and/or backend implementation. A module with both frontend and backend has two locations (one ui, one api). A module that needs neither (for example only documentation or a vendor service) has no locations, frontend false, backend false and a note that says why. A folder used by several modules is owned by "shared" or lists them in shared_by, with the reason in the node. Every location path is also a folder in nodes, with its owner and responsibility (a UI folder such as apps/mobile/lib/features/booking/ too); a location that is not a node is refused.
- dependency: direction (which way dependencies point); allowed and forbidden rules (from, to, reason; from and to are an exact project module id from PROJECT MODULES, "shared", "platform" or an existing node path — a folder covers everything below it. Never write "*", "all", "all-modules", or any invented group. Never forbid a module folder from depending on its parent: that also forbids the module's own internal layers); communication (module-to-module links with via = the public interface, event or contract used, and mode sync|event); interfaces (the public entry point of each module that others use; interfaces[].module is a project module id, never "shared" or "platform"); shared_rules (what shared code may and may not contain); cycle_guard (how circular dependencies are prevented). The rules must agree with depends_on of the nodes: no node may depend on what a forbidden rule forbids. depends_on of a module lists shared, platform, contracts, or the few modules it actually imports — not every other module. A mesh of all modules is a cycle and will be refused (break it with events or a shared contract).
- rules are the architecture rules of the repository: where code goes, naming, public interfaces, tests, configuration. Short, checkable, no repetition of dependency rules.
- evaluation: score EVERY criterion 1-5 with a concrete reason that points at paths or modules of this plan: cohesion, coupling, ownership, maintainability, discoverability, testability, extensibility, simplicity (5 = best) and complexity (1 = simple, 5 = complex). Be honest; a plan with more folders is not better. findings lists the weaknesses you see yourself, each with a category (unnecessary-directory, unnecessary-abstraction, responsibility-overlap, circular-dependency, excessive-coupling, poor-naming, duplication, over-engineering, other), severity, path, message and resolution; an empty list means you looked and found none.
- consistency: check the plan against each of architecture_identity (the layout expresses the blueprint architecture pattern), project_modules, folder_responsibility, technology, dependency_rules, architecture_rules, structure_decision. status is consistent, resolved (you found a conflict and fixed it in this plan) or conflict (you could not fix it; it blocks approval). Resolve conflicts before answering.
- decisions records the main design decisions: title, choice, reason, alternatives you rejected and the trade-off.

Reply with ONLY one JSON object in a ` + "```json" + ` block:
{
  "approach": "<your own name for this organization of the code>",
  "topology": "<repository topology: how many repositories, root folders and what each holds>",
  "summary": "<how the code is organized, in one short paragraph>",
  "rationale": "<why this approach fits this system>",
  "nodes": [
    {"path": "apps/", "type": "dir", "role": "source", "responsibility": "...", "owner": "platform", "reason": "...", "allowed": ["..."], "forbidden": ["..."], "relations": "...", "boundary": "..."},
    {"path": "apps/api/modules/booking/", "type": "dir", "role": "source", "part": "module:booking", "responsibility": "...", "owner": "booking", "reason": "...", "depends_on": ["shared"], "allowed": ["..."], "forbidden": ["..."]},
    {"path": "apps/api/modules/booking/index.ts", "type": "file", "role": "public interface", "responsibility": "...", "owner": "booking", "reason": "..."},
    {"path": "apps/web/src/features/booking/", "type": "dir", "role": "source", "responsibility": "...", "owner": "booking", "reason": "...", "allowed": ["..."], "forbidden": ["..."]}
  ],
  "modules": [{"module": "booking", "locations": [{"path": "apps/api/modules/booking/", "role": "api"}, {"path": "apps/web/src/features/booking/", "role": "ui"}], "frontend": true, "backend": true, "note": ""}],
  "dependency": {"direction": "...", "allowed": [{"from": "booking", "to": "shared", "reason": "..."}], "forbidden": [{"from": "shared", "to": "booking", "reason": "..."}],
    "communication": [{"from": "booking", "to": "notification", "via": "booking.confirmed event", "mode": "event", "reason": "..."}],
    "interfaces": [{"module": "booking", "path": "apps/api/modules/booking/index.ts", "reason": "..."}], "shared_rules": ["..."], "cycle_guard": "..."},
  "rules": ["..."],
  "evaluation": {"summary": "...", "criteria": [{"name": "cohesion", "score": 4, "reason": "..."}], "findings": [{"category": "over-engineering", "severity": "info", "path": "...", "message": "...", "resolution": "..."}]},
  "consistency": [{"area": "project_modules", "status": "consistent", "note": "..."}],
  "decisions": [{"title": "...", "choice": "...", "reason": "...", "alternatives": ["..."], "tradeoff": "..."}]
}`)
	return b.String()
}

// writeRepoAnchors lists what the plan must place: project modules and the
// parts of the structure decision.
func writeRepoAnchors(b *strings.Builder, p *core.Project) {
	if mods := repoplan.Modules(p); len(mods) > 0 {
		b.WriteString("PROJECT MODULES (exact ids; every one must appear in modules and own its code):\n")
		for _, m := range mods {
			fmt.Fprintf(b, "- %s: %s\n", m.ID, m.Responsibility)
		}
		b.WriteString("\n")
	}
	if s := p.Structure; s != nil && s.IR != nil {
		b.WriteString("STRUCTURE PARTS (exact refs; each must be linked to exactly one folder through part; a module's UI folder is a location with role ui, not a second part mapping):\n")
		for _, part := range repoplan.IRParts(s.IR) {
			fmt.Fprintf(b, "- %s (default folder %s)\n", part.Ref, part.Path)
		}
		b.WriteString("\n")
	}
}

// writeRepoRevisionContext carries the previous plan and the refusal of the
// last answer into a design prompt.
func writeRepoRevisionContext(b *strings.Builder, p *core.Project) {
	s := p.Structure
	if s.Repo != nil {
		b.WriteString("PREVIOUS REPOSITORY PLAN (design again; keep what still holds, change what the request or the changed sources require):\n")
		writeRepoPlan(b, s.Repo, 60000, 110)
		b.WriteString("\n")
	}
	if r := s.RepoRequest; r != nil {
		if strings.TrimSpace(r.Text) != "" {
			b.WriteString("USER REQUEST:\n" + strings.TrimSpace(r.Text) + "\n\n")
		}
		if r.Rejected != "" {
			b.WriteString("YOUR LAST ANSWER WAS REFUSED, fix this: " + r.Rejected + "\n\n")
		}
	}
}

// writeRepoPlan writes the plan compactly: the exact paths and ownership
// the AI must keep referring to. text cuts each responsibility; limit cuts
// the whole listing.
func writeRepoPlan(b *strings.Builder, plan *core.RepoPlan, limit, text int) {
	cut := func(s string) string {
		if r := []rune(s); len(r) > text {
			return string(r[:text]) + "…"
		}
		return s
	}
	fmt.Fprintf(b, "approach: %s\ntopology: %s\nsummary: %s\n", plan.Approach, plan.Topology, plan.Summary)
	start := b.Len()
	if text <= 0 {
		// Every path, grouped by owner; no responsibility.
		b.WriteString("nodes by owner (owner: path [part], ...):\n")
		var owners []string
		paths := map[string][]string{}
		for _, n := range plan.Nodes {
			if _, ok := paths[n.Owner]; !ok {
				owners = append(owners, n.Owner)
			}
			p := n.Path
			if n.Part != "" {
				p += " [" + n.Part + "]"
			}
			paths[n.Owner] = append(paths[n.Owner], p)
		}
		for _, o := range owners {
			fmt.Fprintf(b, "- %s: %s\n", o, strings.Join(paths[o], ", "))
		}
	} else {
		b.WriteString("nodes (path | owner | part | responsibility):\n")
	}
	for i, n := range plan.Nodes {
		if text <= 0 {
			break
		}
		fmt.Fprintf(b, "- %s | %s | %s | %s\n", n.Path, n.Owner, n.Part, cut(n.Responsibility))
		if b.Len()-start > limit {
			fmt.Fprintf(b, "…(dipotong, %d node lagi)\n", len(plan.Nodes)-i-1)
			break
		}
	}
	if len(plan.Modules) > 0 {
		b.WriteString("modules (module | locations | frontend | backend):\n")
		for _, m := range plan.Modules {
			var locs []string
			for _, l := range m.Locations {
				locs = append(locs, l.Path)
			}
			fmt.Fprintf(b, "- %s | %s | %t | %t | %s\n", m.Module, strings.Join(locs, ", "), m.Frontend, m.Backend, m.Note)
		}
	}
	d := plan.Dependency
	fmt.Fprintf(b, "dependency direction: %s\ncycle guard: %s\n", d.Direction, d.CycleGuard)
	for _, set := range []struct {
		name  string
		rules []core.RepoDepRule
	}{{"allowed", d.Allowed}, {"forbidden", d.Forbidden}, {"communication", d.Communication}} {
		for _, r := range set.rules {
			fmt.Fprintf(b, "- %s %s -> %s %s\n", set.name, r.From, r.To, r.Via)
		}
	}
	for _, i := range d.Interfaces {
		fmt.Fprintf(b, "- interface %s: %s\n", i.Module, i.Path)
	}
	for _, r := range plan.Rules {
		fmt.Fprintf(b, "- rule: %s\n", r)
	}
}

// writeRepoBrief is the approved repository plan as a source of truth for
// other prompts (blueprint, workflow): where code goes and what may depend
// on what.
func writeRepoBrief(b *strings.Builder, p *core.Project) { writeRepoBriefText(b, p, 90) }

// writeRepoBriefText is writeRepoBrief with node responsibilities cut to
// text runes; 0 leaves them out (paths, owners and rules stay).
func writeRepoBriefText(b *strings.Builder, p *core.Project, text int) {
	s := p.Structure
	if s == nil || s.Mode == core.StructureExisting || s.Repo == nil {
		return
	}
	plan := s.Repo
	status := "approved"
	switch {
	case plan.State() != core.TechApproved:
		status = "not approved yet"
	case plan.Stale:
		status = "approved but stale; sources changed"
	}
	fmt.Fprintf(b, "REPOSITORY PLAN (%s; source of truth for exact folders, files, module locations and dependency rules; the coding agent must not invent another layout):\n", status)
	writeRepoPlan(b, plan, 14000, text)
	b.WriteString("\n")
}

// CleanRepoRequest validates a revision request.
func CleanRepoRequest(text string) (string, error) {
	text = strings.TrimSpace(text)
	if len(text) > MaxRepoRequest {
		return "", fmt.Errorf("permintaan terlalu panjang (maks. %d karakter)", MaxRepoRequest)
	}
	return text, nil
}

// RepoRevisePrompt asks the AI to revise the current plan: a few ops, a
// refreshed evaluation and consistency check.
func RepoRevisePrompt(p *core.Project) string {
	s := p.Structure
	plan := s.Repo
	var b strings.Builder
	b.WriteString(`You are the repository architect of Foundry. Foundry designs software and writes instructions for an external coding agent; it never implements and you must not write application code or touch files.
Revise the CURRENT REPOSITORY PLAN below for the user's request and for the sources that changed since it was written. Change ONLY what the request or a changed source requires; keep everything else. Every folder and file keeps responsibility, owner, reason; every folder keeps allowed and forbidden. Do not change the technology, requirements, domain boundaries, platform or architecture decision. Never leave a project module without a location, a dependency cycle, or a node that depends on what a forbidden rule forbids.
Write every human-readable text in the same language as the story.

PROJECT: `)
	b.WriteString(p.Name)
	b.WriteString("\nSTORY:\n")
	b.WriteString(p.Story)
	b.WriteString("\n\n")
	writeBinding(&b, p)
	writeSystemSnapshot(&b, p, snapshotLean)
	writeRepoAnchors(&b, p)
	writeStructureBrief(&b, p, "ARCHITECTURE STRUCTURE DECISION (the parts the repository must implement; its dependency rules are binding)")
	b.WriteString("CURRENT REPOSITORY PLAN:\n")
	writeRepoPlan(&b, plan, 60000, 110)
	b.WriteString("\n")
	if areas := plan.Analysis.StaleAreas; len(areas) > 0 {
		fmt.Fprintf(&b, "SOURCES THAT CHANGED since the plan was written: %s. Plan sections affected: %s. Re-check those against the system above and fix what no longer fits.\n\n",
			strings.Join(areas, ", "), strings.Join(plan.Analysis.Affected, ", "))
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
	writeRepoReview(&b, plan)
	if r := s.RepoRequest; r != nil {
		if t := strings.TrimSpace(r.Text); t != "" {
			b.WriteString("USER REQUEST:\n" + t + "\n")
		} else {
			b.WriteString("USER REQUEST: none. Reconcile the plan with the changed sources and evaluate it again.\n")
		}
		if r.Reason != "" {
			b.WriteString("REASON GIVEN BY THE USER:\n" + r.Reason + "\n")
		}
		b.WriteString("\n")
		if r.Rejected != "" {
			b.WriteString("YOUR LAST ANSWER WAS REFUSED, fix this: " + r.Rejected + "\n\n")
		}
	}
	b.WriteString(`Reply with ONLY one JSON object in a ` + "```json" + ` block. evaluation and consistency are mandatory JSON keys (never omit them, never leave placeholders). Put them first:
{"evaluation": {"summary": "...", "criteria": [{"name":"cohesion","score":4,"reason":"..."}, {"name":"coupling","score":4,"reason":"..."}, {"name":"ownership","score":4,"reason":"..."}, {"name":"maintainability","score":4,"reason":"..."}, {"name":"discoverability","score":4,"reason":"..."}, {"name":"testability","score":4,"reason":"..."}, {"name":"extensibility","score":4,"reason":"..."}, {"name":"simplicity","score":4,"reason":"..."}, {"name":"complexity","score":2,"reason":"..."}], "findings": []},
 "consistency": [{"area":"architecture_identity","status":"consistent","note":"..."}, {"area":"project_modules","status":"consistent","note":"..."}, {"area":"folder_responsibility","status":"consistent","note":"..."}, {"area":"technology","status":"consistent","note":"..."}, {"area":"dependency_rules","status":"consistent","note":"..."}, {"area":"architecture_rules","status":"consistent","note":"..."}, {"area":"structure_decision","status":"consistent","note":"..."}],
 "ops": [ ... ], "note": "<one sentence on what you changed and why>",
 "decisions": [new design decisions only: {"title", "choice", "reason", "alternatives", "tradeoff"}]}
Ops apply in order; paths are exact paths of the current plan (a folder ends with "/"):
- {"op":"set_node","node":{...a complete node: path, type, role, responsibility, owner, reason, part, depends_on, shared_by, allowed, forbidden, relations, boundary}} adds or replaces a folder or file; its parent folders must exist or are added without description, so describe them too
- {"op":"remove_node","path":"x/"} removes the node and everything below it, and the references to it
- {"op":"move_node","path":"x/","to":"y/"} moves a node (with everything below it) and rewrites the references
- {"op":"set_module","module":{"module":"id","locations":[{"path":"...","role":"..."}],"frontend":true,"backend":true,"note":""}} ; {"op":"remove_module","module":{"module":"id"}}; every location path must be a node of the plan after the ops (add it with set_node in the same answer)
- {"op":"add_rule","text":"..."} ; {"op":"remove_rule","text":"<exact current rule>"}
- {"op":"add_dependency","kind":"allowed|forbidden|communication","dep":{"from":"","to":"","via":"","mode":"sync|event","reason":""}} ; {"op":"remove_dependency","kind":"...","dep":{"from":"","to":""}}
- {"op":"set_interface","interface":{"module":"","path":"","reason":""}} ; {"op":"remove_interface","interface":{"module":"","path":""}}
- {"op":"set_meta","approach":"","topology":"","summary":""} (only the fields to change)
"ops": [] is valid only when nothing in the plan has to change; still return evaluation and consistency for the plan after your ops. To redesign an area, remove its nodes and set the new ones in the same answer. If ops cannot express the change, return a complete plan instead (nodes, modules, dependency, rules, evaluation, consistency — same shape as a fresh design) and Foundry replaces the current plan with it.`)
	return b.String()
}

// writeRepoReview dumps the current quality review so a revision can update
// it instead of inventing the schema from the example.
func writeRepoReview(b *strings.Builder, plan *core.RepoPlan) {
	if plan == nil {
		return
	}
	if len(plan.Evaluation.Criteria) > 0 {
		b.WriteString("CURRENT EVALUATION (return a full updated evaluation; copy scores that still hold):\n")
		if plan.Evaluation.Summary != "" {
			fmt.Fprintf(b, "summary: %s\n", plan.Evaluation.Summary)
		}
		for _, c := range plan.Evaluation.Criteria {
			fmt.Fprintf(b, "- %s: %d — %s\n", c.Name, c.Score, c.Reason)
		}
		b.WriteString("\n")
	}
	if len(plan.Consistency) > 0 {
		b.WriteString("CURRENT CONSISTENCY (return every area again):\n")
		for _, c := range plan.Consistency {
			fmt.Fprintf(b, "- %s: %s %s\n", c.Area, c.Status, c.Note)
		}
		b.WriteString("\n")
	}
}

type repoRevision struct {
	Ops         []core.RepoOp        `json:"ops"`
	Note        string               `json:"note"`
	Evaluation  *core.RepoEvaluation `json:"evaluation"`
	Consistency []core.RepoCheck     `json:"consistency"`
	Decisions   []core.RepoDecision  `json:"decisions"`
}

// ApplyRepoPlan lands a fresh design. It replaces the nodes, modules and
// rules, keeps the revision history, and starts a new review.
func ApplyRepoPlan(p *core.Project, raw string) error {
	s := p.Structure
	if s == nil || s.RepoRequest == nil || s.RepoRequest.Action != core.RepoGenerate {
		return errors.New("tidak ada permintaan rancangan repository")
	}
	var plan core.RepoPlan
	if err := ExtractJSONFunc(raw, &plan, func() bool { return len(plan.Nodes) > 0 }); err != nil {
		if body := jsonObjectBytes(raw); len(body) == 0 || unmarshalLoose(body, &plan) != nil || len(plan.Nodes) == 0 {
			return err
		}
	}
	plan.Status, plan.Analysis, plan.History, plan.Stale = "", core.RepoAnalysis{}, nil, false
	plan.Evaluation.Outdated = false
	if err := repoplan.Accept(p, &plan); err != nil {
		return err
	}
	req := s.RepoRequest
	if old := s.Repo; old != nil {
		plan.History = old.History
	}
	plan.Status = core.TechRecommended
	repoplan.Record(&plan, "generate", strings.TrimSpace(req.Text), req.Reason,
		[]string{fmt.Sprintf("Rancangan oleh AI: %s (%d folder, %d file)", plan.Approach, plan.Analysis.Dirs, plan.Analysis.Files)})
	s.Repo = &plan
	s.RepoRequest = nil
	return nil
}

// ApplyRepoRevision lands a revision: the ops, the refreshed evaluation and
// consistency check, as one new version. A complete plan (nodes) is accepted
// when the model redesigns instead of sending ops. A missing or incomplete
// review is filled from the current plan so a usable tree change still lands.
func ApplyRepoRevision(p *core.Project, raw string) error {
	s := p.Structure
	if s == nil || s.Repo == nil || s.RepoRequest == nil || s.RepoRequest.Action != core.RepoRevise {
		return errors.New("tidak ada permintaan revisi repository")
	}
	a, full, err := parseRepoRevision(raw)
	if err != nil {
		return err
	}
	if full != nil && len(a.Ops) == 0 {
		return applyFullRepoRevision(p, full)
	}
	reused := fillRepoReview(&a, s.Repo)
	if a.Evaluation == nil || !completeRepoReview(a.Evaluation, a.Consistency) {
		return errors.New("jawaban harus menyertakan evaluation dan consistency untuk rencana setelah perubahan")
	}
	req := s.RepoRequest
	plan := s.Repo.Clone()
	var changes []string
	if len(a.Ops) > 0 {
		if changes, err = repoplan.Apply(plan, a.Ops); err != nil {
			return err
		}
	}
	plan.Evaluation, plan.Consistency = *a.Evaluation, a.Consistency
	plan.Decisions = append(plan.Decisions, a.Decisions...)
	if reused && len(changes) > 0 {
		plan.Evaluation.Outdated = true
	} else if !reused {
		plan.Evaluation.Outdated = false
	}
	if err := repoplan.Accept(p, plan); err != nil {
		return err
	}
	if len(changes) == 0 {
		changes = []string{"Evaluasi dan konsistensi diperbarui; struktur tidak berubah"}
	}
	if n := strings.TrimSpace(a.Note); n != "" {
		changes = append(changes, "Catatan AI: "+n)
	}
	repoplan.Changed(plan)
	repoplan.Record(plan, "ai", strings.TrimSpace(req.Text), req.Reason, changes)
	s.Repo = plan
	s.RepoRequest = nil
	return nil
}

func parseRepoRevision(raw string) (repoRevision, *core.RepoPlan, error) {
	var a repoRevision
	matchRev := func() bool { return a.Ops != nil || a.Evaluation != nil || a.Note != "" }
	revErr := ExtractJSONFunc(raw, &a, matchRev)
	if revErr != nil {
		revErr = extractClosedJSON(raw, &a, matchRev)
	}
	var full core.RepoPlan
	matchFull := func() bool { return len(full.Nodes) > 0 }
	fullOK := ExtractJSONFunc(raw, &full, matchFull) == nil
	if !fullOK {
		fullOK = extractClosedJSON(raw, &full, matchFull) == nil
	}
	if fullOK && len(a.Ops) == 0 {
		return a, &full, nil
	}
	if revErr != nil {
		if fullOK {
			return a, &full, nil
		}
		return a, nil, revErr
	}
	return a, nil, nil
}

// extractClosedJSON is ExtractJSONFunc for a truncated fence: missing
// quotes and braces are closed so a cut-off revision can still land.
func extractClosedJSON(out string, v any, match func() bool) error {
	for _, fence := range fenceBodies(out) {
		if err := ExtractJSONFunc(closeJSON(fence), v, match); err == nil {
			return nil
		}
	}
	i := strings.IndexByte(out, '{')
	if i >= 0 {
		if err := ExtractJSONFunc(closeJSON(out[i:]), v, match); err == nil {
			return nil
		}
	}
	return errors.New("output AI tidak berisi JSON yang valid")
}

func applyFullRepoRevision(p *core.Project, plan *core.RepoPlan) error {
	s := p.Structure
	old := s.Repo
	if !completeRepoReview(&plan.Evaluation, plan.Consistency) && completeRepoReview(&old.Evaluation, old.Consistency) {
		plan.Evaluation = old.Evaluation
		plan.Consistency = append([]core.RepoCheck(nil), old.Consistency...)
		plan.Evaluation.Outdated = true
	} else {
		plan.Evaluation.Outdated = false
	}
	plan.Status, plan.Analysis, plan.History, plan.Stale = "", core.RepoAnalysis{}, nil, false
	if err := repoplan.Accept(p, plan); err != nil {
		return err
	}
	req := s.RepoRequest
	plan.History = old.History
	plan.Status = old.Status
	repoplan.Changed(plan)
	repoplan.Record(plan, "ai", strings.TrimSpace(req.Text), req.Reason,
		[]string{fmt.Sprintf("Rancangan ulang oleh AI: %s (%d folder, %d file)", plan.Approach, plan.Analysis.Dirs, plan.Analysis.Files)})
	s.Repo = plan
	s.RepoRequest = nil
	return nil
}

func fillRepoReview(a *repoRevision, cur *core.RepoPlan) bool {
	if completeRepoReview(a.Evaluation, a.Consistency) {
		return false
	}
	if cur == nil || !completeRepoReview(&cur.Evaluation, cur.Consistency) {
		return false
	}
	ev := cur.Evaluation
	a.Evaluation = &ev
	a.Consistency = append([]core.RepoCheck(nil), cur.Consistency...)
	return true
}

func completeRepoReview(e *core.RepoEvaluation, cons []core.RepoCheck) bool {
	if e == nil {
		return false
	}
	for _, name := range core.RepoCriteria {
		i := slices.IndexFunc(e.Criteria, func(c core.RepoCriterion) bool { return c.Name == name })
		if i < 0 || e.Criteria[i].Score < 1 || strings.TrimSpace(e.Criteria[i].Reason) == "" {
			return false
		}
	}
	for _, area := range core.RepoConsistencyAreas {
		i := slices.IndexFunc(cons, func(c core.RepoCheck) bool { return c.Area == area })
		if i < 0 {
			return false
		}
		switch cons[i].Status {
		case core.CheckConsistent, core.CheckResolved, core.CheckConflict:
		default:
			return false
		}
	}
	return true
}
