package work

import (
	"fmt"
	"strings"
	"time"

	"github.com/handrawanw/foundry/internal/core"
)

const KindPRD = "prd"

const (
	FindGAP        = "GAP"
	FindMissing    = "MISSING_TASK"
	FindUnassigned = "UNASSIGNED"
	FindDuplicate  = "DUPLICATE"
	FindOverlap    = "OVERLAP"
	FindInvalidDep = "INVALID_DEPENDENCY"
)

var prdFindingKinds = map[string]bool{
	FindGAP: true, FindMissing: true, FindUnassigned: true, FindDuplicate: true, FindOverlap: true, FindInvalidDep: true,
}

// prdSections are the required headings of the product document. Order is
// the document structure: product → capability → flow → contract → task.
var prdSections = []string{
	"1. Product Overview",
	"2. Goals",
	"3. Non-Goals",
	"4. Actors",
	"5. Core Capabilities",
	"6. Business Flows",
	"7. UI",
	"8. API Contract",
	"9. Data Model",
	"10. Authorization",
	"11. Background Workers",
	"12. Security",
	"13. Non-Functional",
	"14. Product Acceptance",
	"15. Technical Constraints",
	"16. Task Traceability",
	"17. Dependency",
	"18. Task Breakdown",
}

type prdLink struct {
	Requirement string   `json:"requirement,omitempty"`
	Capability  string   `json:"capability,omitempty"`
	FeatureID   string   `json:"feature_id,omitempty"`
	Flow        string   `json:"flow,omitempty"`
	FlowID      string   `json:"flow_id,omitempty"`
	Page        string   `json:"page,omitempty"`
	PageID      string   `json:"page_id,omitempty"`
	Route       string   `json:"route,omitempty"`
	API         []string `json:"api,omitempty"`
	TaskID      string   `json:"task_id,omitempty"`
	Area        string   `json:"area,omitempty"`
}

type prdAnalysis struct {
	Links    []prdLink         `json:"links"`
	Findings []core.PRDFinding `json:"findings"`
}

type prdOut struct {
	Markdown string           `json:"markdown"`
	Findings []core.PRDFinding `json:"findings"`
}

// AnalyzePRD traces requirement → capability → flow → page/API → task and
// records gaps. It does not invent entities, APIs or flows.
func AnalyzePRD(p *core.Project) prdAnalysis {
	a := prdAnalysis{}
	if p == nil {
		return a
	}
	reqByID := map[string]core.Requirement{}
	for _, r := range p.Requirements {
		reqByID[r.ID] = r
	}
	featByID := map[string]*core.Feature{}
	reqFeatures := map[string][]*core.Feature{}
	for i := range p.Features {
		f := &p.Features[i]
		featByID[f.ID] = f
		for _, rid := range f.RequirementIDs {
			reqFeatures[rid] = append(reqFeatures[rid], f)
		}
	}
	flowsByFeat := map[string][]core.Flow{}
	for _, fl := range p.Flows {
		flowsByFeat[fl.FeatureID] = append(flowsByFeat[fl.FeatureID], fl)
	}
	pagesByFeat := map[string][]core.Page{}
	pageByID := map[string]core.Page{}
	if p.Design != nil {
		for _, pg := range p.Design.Pages {
			pagesByFeat[pg.FeatureID] = append(pagesByFeat[pg.FeatureID], pg)
			pageByID[pg.ID] = pg
		}
	}
	tasksByFeat := map[string][]core.Task{}
	tasksByReq := map[string][]core.Task{}
	taskByID := map[string]*core.Task{}
	pageTasks := map[string][]string{}
	for i := range p.Tasks {
		t := &p.Tasks[i]
		taskByID[t.ID] = t
		if t.FeatureID != "" {
			tasksByFeat[t.FeatureID] = append(tasksByFeat[t.FeatureID], *t)
		}
		for _, rid := range t.RequirementIDs {
			tasksByReq[rid] = append(tasksByReq[rid], *t)
		}
		for _, pid := range t.PageIDs {
			pageTasks[pid] = append(pageTasks[pid], t.ID)
		}
	}

	seenLink := map[string]bool{}
	addLink := func(l prdLink) {
		key := strings.Join([]string{l.Requirement, l.FeatureID, l.FlowID, l.PageID, l.TaskID}, "|")
		if seenLink[key] {
			return
		}
		seenLink[key] = true
		a.Links = append(a.Links, l)
	}

	for _, r := range p.Requirements {
		feats := reqFeatures[r.ID]
		if len(feats) == 0 {
			addLink(prdLink{Requirement: r.ID + " " + r.Text})
			continue
		}
		for _, f := range feats {
			flows := flowsByFeat[f.ID]
			pages := pagesByFeat[f.ID]
			tasks := uniqueTasks(append(tasksByFeat[f.ID], tasksByReq[r.ID]...))
			if len(flows) == 0 && len(pages) == 0 && len(tasks) == 0 {
				addLink(prdLink{Requirement: r.ID + " " + r.Text, Capability: f.Name, FeatureID: f.ID})
				continue
			}
			if len(tasks) == 0 {
				if len(flows) == 0 {
					for _, pg := range pages {
						addLink(prdLink{Requirement: r.ID + " " + r.Text, Capability: f.Name, FeatureID: f.ID,
							Page: pg.Name, PageID: pg.ID, Route: pg.Route, API: pageAPI(pg)})
					}
				}
				for _, fl := range flows {
					addLink(prdLink{Requirement: r.ID + " " + r.Text, Capability: f.Name, FeatureID: f.ID,
						Flow: fl.Name, FlowID: fl.ID})
				}
				continue
			}
			for _, t := range tasks {
				if len(t.PageIDs) > 0 {
					for _, pid := range t.PageIDs {
						pg := pageByID[pid]
						addLink(prdLink{Requirement: r.ID + " " + r.Text, Capability: f.Name, FeatureID: f.ID,
							Page: pg.Name, PageID: pid, Route: pg.Route, API: pageAPI(pg),
							TaskID: t.ID, Area: t.Area})
					}
				}
				matchedFlow := false
				for _, fl := range flows {
					if flowServesTask(fl, t) {
						matchedFlow = true
						addLink(prdLink{Requirement: r.ID + " " + r.Text, Capability: f.Name, FeatureID: f.ID,
							Flow: fl.Name, FlowID: fl.ID, TaskID: t.ID, Area: t.Area})
					}
				}
				if !matchedFlow && len(t.PageIDs) == 0 {
					addLink(prdLink{Requirement: r.ID + " " + r.Text, Capability: f.Name, FeatureID: f.ID,
						TaskID: t.ID, Area: t.Area})
				}
			}
		}
	}

	find := func(kind, ref, msg string) {
		a.Findings = append(a.Findings, core.PRDFinding{Kind: kind, Ref: ref, Message: msg})
	}

	coverFeat := func(f *core.Feature) []core.Task {
		if f == nil {
			return nil
		}
		out := append([]core.Task{}, tasksByFeat[f.ID]...)
		for _, rid := range f.RequirementIDs {
			out = append(out, tasksByReq[rid]...)
		}
		return uniqueTasks(out)
	}

	missingReq := map[string]bool{}
	for _, r := range p.Requirements {
		if len(reqFeatures[r.ID]) == 0 {
			find(FindGAP, r.ID, r.ID+" tidak punya capability/feature")
		}
		impl := uniqueTasks(append(tasksByReq[r.ID], tasksOfFeatures(reqFeatures[r.ID], tasksByFeat)...))
		if !hasImplTask(impl) {
			find(FindMissing, r.ID, r.ID+" tidak punya task implementasi")
			missingReq[r.ID] = true
		}
	}
	allMissing := func(ids []string) bool {
		if len(ids) == 0 {
			return false
		}
		for _, id := range ids {
			if !missingReq[id] {
				return false
			}
		}
		return true
	}
	for _, f := range p.Features {
		if allMissing(f.RequirementIDs) {
			continue
		}
		if !hasImplTask(coverFeat(&f)) {
			find(FindMissing, f.ID, f.ID+" "+f.Name+" tidak punya task implementasi")
		}
	}
	for _, fl := range p.Flows {
		feat := featByID[fl.FeatureID]
		if feat != nil && allMissing(feat.RequirementIDs) {
			continue
		}
		covering := coverFeat(feat)
		if feat == nil {
			covering = tasksByFeat[fl.FeatureID]
		}
		served := false
		for _, t := range covering {
			if flowServesTask(fl, t) {
				served = true
				break
			}
		}
		if !served {
			find(FindMissing, fl.ID, fl.ID+" "+fl.Name+" tidak punya implementasi")
		}
	}
	if p.Design != nil {
		pageInFlow := map[string]bool{}
		for _, fl := range p.Flows {
			for _, pid := range fl.PageIDs {
				pageInFlow[pid] = true
			}
		}
		for _, pg := range p.Design.Pages {
			if len(pageTasks[pg.ID]) > 0 {
				continue
			}
			feat := featByID[pg.FeatureID]
			if feat != nil && allMissing(feat.RequirementIDs) {
				continue
			}
			needed := pageInFlow[pg.ID] || hasImplTask(coverFeat(feat))
			if needed {
				find(FindMissing, pg.ID, pg.ID+" "+pg.Name+" tidak punya task UI")
			} else {
				find(FindUnassigned, pg.ID, pg.ID+" tidak jelas dimiliki task mana pun")
			}
		}
	}

	titleSeen := map[string]string{}
	for i := range p.Tasks {
		t := &p.Tasks[i]
		if t.Area == "scaffold" {
			if len(t.AcceptanceCriteria) == 0 {
				find(FindGAP, t.ID, t.ID+" scaffold tidak punya acceptance criteria")
			}
			continue
		}
		if t.FeatureID == "" {
			find(FindGAP, t.ID, t.ID+" tidak punya business requirement (bukan scaffold)")
		} else if featByID[t.FeatureID] == nil {
			find(FindGAP, t.ID, t.ID+" merujuk feature yang tidak ada")
		} else if len(t.RequirementIDs) == 0 && len(featByID[t.FeatureID].RequirementIDs) == 0 {
			find(FindGAP, t.ID, t.ID+" tidak tertaut ke requirement")
		}
		if len(t.AcceptanceCriteria) == 0 {
			find(FindGAP, t.ID, t.ID+" tidak punya acceptance criteria")
		}
		key := t.FeatureID + "\x00" + strings.ToLower(strings.TrimSpace(t.Title))
		if prev, ok := titleSeen[key]; ok {
			find(FindDuplicate, t.ID, t.ID+" judul sama dengan "+prev)
		} else {
			titleSeen[key] = t.ID
		}
		for _, d := range t.DependsOn {
			if taskByID[d] == nil {
				find(FindInvalidDep, t.ID, t.ID+" depends_on "+d+" yang tidak ada")
			}
		}
	}
	if c := findCycle(p.Tasks); c != "" {
		find(FindInvalidDep, c, "dependency task melingkar di "+c)
	}

	locks := make([][]string, len(p.Tasks))
	for i := range p.Tasks {
		if p.Tasks[i].Area == "scaffold" || p.Tasks[i].Area == "integration" {
			continue
		}
		for _, l := range p.Tasks[i].Locks() {
			if strings.HasPrefix(l, "file:") || strings.HasPrefix(l, "page:") {
				locks[i] = append(locks[i], l)
			}
		}
	}
	reaches := taskReaches(p.Tasks)
	for i := range p.Tasks {
		for j := i + 1; j < len(p.Tasks); j++ {
			if !ownedOverlap(locks[i], locks[j]) {
				continue
			}
			if reaches(i, j) || reaches(j, i) {
				continue
			}
			find(FindOverlap, p.Tasks[j].ID, p.Tasks[i].ID+" dan "+p.Tasks[j].ID+" berbagi file/halaman tanpa dependensi")
		}
	}
	return a
}

func hasImplTask(in []core.Task) bool {
	for _, t := range in {
		if t.Area != "test" {
			return true
		}
	}
	return false
}

func uniqueTasks(in []core.Task) []core.Task {
	seen := map[string]bool{}
	var out []core.Task
	for _, t := range in {
		if seen[t.ID] {
			continue
		}
		seen[t.ID] = true
		out = append(out, t)
	}
	return out
}

func tasksOfFeatures(feats []*core.Feature, byFeat map[string][]core.Task) []core.Task {
	var out []core.Task
	for _, f := range feats {
		out = append(out, byFeat[f.ID]...)
	}
	return out
}

func pageAPI(pg core.Page) []string {
	if pg.Data == nil {
		return nil
	}
	return pg.Data.API
}

func flowServesTask(fl core.Flow, t core.Task) bool {
	if t.FeatureID != "" && t.FeatureID == fl.FeatureID && t.Area != "ui" && len(t.PageIDs) == 0 {
		return true
	}
	for _, pid := range t.PageIDs {
		for _, fp := range fl.PageIDs {
			if pid == fp {
				return true
			}
		}
	}
	return false
}

func taskReaches(tasks []core.Task) func(a, b int) bool {
	idx := map[string]int{}
	for i, t := range tasks {
		idx[t.ID] = i
	}
	return func(a, b int) bool {
		seen := map[int]bool{}
		stack := []int{a}
		for len(stack) > 0 {
			n := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			for _, d := range tasks[n].DependsOn {
				j, ok := idx[d]
				if !ok || seen[j] {
					continue
				}
				if j == b {
					return true
				}
				seen[j] = true
				stack = append(stack, j)
			}
		}
		return false
	}
}

// PRDPrompt asks the product analyst to reconstruct the PRD from existing
// project context. Analysis is computed first and injected so the model
// does not start from TASK-IDs.
func PRDPrompt(p *core.Project) string {
	var b strings.Builder
	b.WriteString(`You are the product analyst of Foundry. Reconstruct the Product Requirements Document from the EXISTING project context.
Do not write application code. Do not modify files.
The PRD is the source of truth for humans and coding agents: every task must be traceable to the requirement and business flow that created it.

Structure: Product → Capability → Business Flow → Technical Contract → Task.
Do not make TASK-ID the document structure. Do not turn the PRD into a backlog.
Do not invent requirements, APIs, entities, flows, pages, workers, or business rules that the context does not support.
Do not create a requirement only because a task exists.
TASK-001 (area scaffold) is technical foundation, not a business capability.

Audit traceability reports only problems that affect requirement completeness or consistency.

MISSING_TASK: only when a product/business requirement needs implementation, no task covers it, and that work must be a separate task.
Do not mark internal technical details (file, function, helper, wire.go, config, sub-component) as MISSING_TASK when they reasonably belong to an existing task.
Do not create a task per file. Example: wire.go that fills moduleRegistrars belongs to the scaffold task already in the list. If there is no evidence it is a separate work unit, treat it as part of that task or GAP/UNASSIGNED — never MISSING_TASK.
Skipped TASK numbers are not MISSING_TASK.

INVALID_DEPENDENCY: only when a task explicitly depends_on a TASK-ID that is absent from the task list.
Empty or null depends_on is not INVALID_DEPENDENCY.

GAP / UNASSIGNED: implementation work whose owning task is unclear, without enough evidence that a new task is required. Do not invent a new TASK-ID.

Priority: (1) missing business requirement (2) missing capability (3) missing business flow implementation (4) missing API/UI/worker required by a flow (5) invalid task dependency (6) overlapping ownership of the same work (7) duplicate traceability.
Do not treat a file or function as a task unit.
One task covering related capabilities in one bounded context is not OVERLAP. OVERLAP is two tasks owning the same files/pages without a dependency.

Analyze the inventory and TRACEABILITY ANALYSIS first, then write the document.

Write human-readable text in the same language as the features. Keep the 18 English section headings exactly.

`)
	fmt.Fprintf(&b, "PRODUCT: %s\nSTORY:\n%s\n\n", p.Name, p.Story)
	if p.Discovery != nil {
		b.WriteString("DISCOVERY:\n" + mustJSON(p.Discovery) + "\n\n")
	}
	if brief := ArchitectureBrief(p); brief != "" {
		b.WriteString(brief + "\n")
	}
	b.WriteString("REQUIREMENTS:\n" + mustJSON(p.Requirements) + "\n\n")
	b.WriteString("FEATURES (capabilities):\n" + mustJSON(p.Features) + "\n\n")
	b.WriteString("FLOWS:\n" + mustJSON(p.Flows) + "\n\n")
	b.WriteString("PAGES:\n" + mustJSON(prdPages(p)) + "\n\n")
	if p.Stack != nil {
		b.WriteString("STACK:\n" + mustJSON(p.Stack.Decisions) + "\n\n")
	}
	if p.Structure != nil {
		b.WriteString(structureBrief(p) + "\n")
	}
	b.WriteString("TASKS:\n" + mustJSON(prdTasks(p)) + "\n\n")
	b.WriteString("TRACEABILITY ANALYSIS (computed; include every finding. Do not add MISSING_TASK on an existing TASK-ID. Do not invent TASK-IDs):\n")
	b.WriteString(mustJSON(AnalyzePRD(p)) + "\n\n")
	b.WriteString(`Required sections, in this order:

# 1. Product Overview
What the product is, the problem, target users, main value. From discovery/story only.

# 2. Goals
Concrete, measurable when the context allows.

# 3. Non-Goals
What is intentionally out of scope. If the context does not say, write GAP.

# 4. Actors & Roles
Each actor and responsibility from discovery.roles.

# 5. Core Capabilities
Group by capability/feature/module, never by TASK-ID.
For each capability: purpose, business requirement, business rules, actors, main input/output, dependency, security.
Capability names come from features (and blueprint modules when they match). Do not invent Authentication/User Management/etc. unless they exist.

# 6. Business Flows
Real business processes from FLOWS, not function or file names.
For each flow:
Actor → Action → Validation → Business Rule → State Change → Event / Side Effect → Audit → Response
Use requirement text as business rules. If a step is unknown, write GAP. Show how the business flow maps to the technical flow (page route, API from page data, task).
Do not add Login/Refresh token/etc. unless those flows exist.

# 7. UI / Pages
Table of existing routes only:

| Page | Actor | Capability | Business Flow | API |

# 8. API Contract
Endpoints required by business flows and page data. For each: Method, Route, Authentication, Permission, Scope, Request, Response, Error, Side effect.
Do not invent endpoints. If page data only has "GET /x", document that; unknown fields are GAP.

# 9. Data Model
Entities from the blueprint (and discovery data). Relationship, lifecycle, constraint. Do not invent entities.

# 10. Authorization Model
From blueprint auth/authz. User → Application → Membership → Role → Permission → Scope → Allow/Deny only when the product has that model. Otherwise describe the actual model and mark missing parts GAP.

# 11. Background Workers
From blueprint workers, by business responsibility. Trigger, input, processing, consistency, retry, failure, idempotency, side effect. Omit the section body with GAP if none.

# 12. Security Requirements
From discovery.security and blueprint auth. Do not invent JWT/JWKS/etc. unless the blueprint says so.

# 13. Non-Functional Requirements
performance, availability, reliability, scalability, observability, security, consistency, maintainability, testing — only from blueprint/stack. GAP when absent.

# 14. Product Acceptance Criteria
Product/business level. Example shape: "Admin dapat … dan user hanya memperoleh …". No function or file names unless they are a product constraint.

# 15. Technical Constraints
The project's chosen stack, structure, languages, tools. Do not replace technology.

# 16. Task Traceability
Requirement → Capability → Business Flow → API/UI/Worker → Task
Use the TRACEABILITY ANALYSIS links. If one task serves several requirements, list them all.

# 17. Dependency & Execution Order
Capability dependency from blueprint modules and task depends_on. Do not force Foundation→Identity→… unless that is the actual graph.
TASK-001 scaffold is the technical foundation when present.

# 18. Task Breakdown
Implementation plan derived from the PRD, not the other way around. For every existing task:
TASK-ID, Type (area), Capability, Objective, Depends On, Business Requirement, Technical Scope, Acceptance Criteria, Tests, Files / Area, Status.
Do not delete a task because it looks unused; explain why it exists (scaffold = foundation).
Mark: GAP, UNASSIGNED, MISSING_TASK, DUPLICATE, OVERLAP, INVALID_DEPENDENCY as specified in the analysis and the audit rules above.

Reply with ONLY one JSON object in a ` + "```json" + ` block, shape:
{"markdown": "# 1. Product Overview\n...", "findings": [{"kind": "GAP", "ref": "TASK-002", "message": ""}]}`)
	return b.String()
}

type prdPageView struct {
	ID, Name, Platform, Role, FeatureID, Purpose, Route, Access string
	API, FlowIDs                                                []string
}

type prdTaskView struct {
	ID, Title, Description, Area, FeatureID, Platform, Status string
	RequirementIDs, PageIDs, DependsOn, AcceptanceCriteria, Files, TestRequirements []string
}

func prdPages(p *core.Project) []prdPageView {
	if p.Design == nil {
		return nil
	}
	flowPages := map[string][]string{}
	for _, fl := range p.Flows {
		for _, pid := range fl.PageIDs {
			flowPages[pid] = append(flowPages[pid], fl.ID)
		}
	}
	var out []prdPageView
	for _, pg := range p.Design.Pages {
		v := prdPageView{ID: pg.ID, Name: pg.Name, Platform: pg.Platform, Role: pg.Role,
			FeatureID: pg.FeatureID, Purpose: pg.Purpose, Route: pg.Route, Access: pg.Access,
			FlowIDs: flowPages[pg.ID]}
		if pg.Data != nil {
			v.API = pg.Data.API
		}
		out = append(out, v)
	}
	return out
}

func prdTasks(p *core.Project) []prdTaskView {
	var out []prdTaskView
	for _, t := range p.Tasks {
		out = append(out, prdTaskView{
			ID: t.ID, Title: t.Title, Description: t.Description, Area: t.Area, FeatureID: t.FeatureID,
			Platform: t.Platform, Status: string(t.Status), RequirementIDs: t.RequirementIDs, PageIDs: t.PageIDs,
			DependsOn: t.DependsOn, AcceptanceCriteria: t.AcceptanceCriteria, Files: t.Files,
			TestRequirements: t.TestRequirements,
		})
	}
	return out
}

// ApplyPRD stores the analyst's markdown after checking required sections.
// Deterministic findings are merged in so gaps cannot be dropped.
func ApplyPRD(p *core.Project, raw string) error {
	var out prdOut
	if err := ExtractJSON(raw, &out); err != nil {
		return err
	}
	md := strings.TrimSpace(out.Markdown)
	if md == "" {
		return fmt.Errorf("PRD kosong")
	}
	if missing := missingPRDSections(md); len(missing) > 0 {
		return fmt.Errorf("PRD kurang bagian: %s", strings.Join(missing, ", "))
	}
	known := prdKnownRefs(p)
	var findings []core.PRDFinding
	seen := map[string]bool{}
	add := func(f core.PRDFinding) {
		f.Kind = strings.ToUpper(strings.TrimSpace(f.Kind))
		f.Ref = strings.TrimSpace(f.Ref)
		f.Message = strings.TrimSpace(f.Message)
		if !prdFindingKinds[f.Kind] || f.Message == "" {
			return
		}
		if f.Kind == FindMissing && strings.HasPrefix(f.Ref, "TASK-") {
			return
		}
		if f.Ref != "" && !known[f.Ref] {
			return
		}
		key := f.Kind + "|" + f.Ref + "|" + f.Message
		if seen[key] {
			return
		}
		seen[key] = true
		findings = append(findings, f)
	}
	for _, f := range AnalyzePRD(p).Findings {
		add(f)
	}
	for _, f := range out.Findings {
		add(f)
	}
	p.PRD = &core.PRD{Markdown: md, Findings: findings, Generated: time.Now().UTC()}
	return nil
}

func missingPRDSections(md string) []string {
	var missing []string
	for _, s := range prdSections {
		if !strings.Contains(md, s) {
			missing = append(missing, s)
		}
	}
	return missing
}

func prdKnownRefs(p *core.Project) map[string]bool {
	known := map[string]bool{}
	for _, r := range p.Requirements {
		known[r.ID] = true
	}
	for _, f := range p.Features {
		known[f.ID] = true
	}
	for _, fl := range p.Flows {
		known[fl.ID] = true
	}
	if p.Design != nil {
		for _, pg := range p.Design.Pages {
			known[pg.ID] = true
		}
	}
	for _, t := range p.Tasks {
		known[t.ID] = true
	}
	if p.Blueprint != nil {
		for _, m := range p.Blueprint.Modules {
			known[m.ID] = true
		}
	}
	return known
}
