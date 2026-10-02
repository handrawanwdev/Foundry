package work

import (
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/handrawanw/foundry/internal/core"
	"github.com/handrawanw/foundry/internal/modules/structure"
)

type planOut struct {
	Tasks []core.Task `json:"tasks"`
}

var taskAreas = map[string]bool{"scaffold": true, "data": true, "api": true, "ui": true, "test": true, "integration": true}

func PlanPrompt(p *core.Project) string {
	var b strings.Builder
	b.WriteString(`You are the tech lead of Foundry. Turn the APPROVED design into executable engineering tasks for a coding agent.
Tasks are derived from requirements, features, sub-features, flows, pages, platforms and the project structure below.
A task must be small enough for one agent run, concrete (never generic like "Build appointment"), and verifiable.
Each feature needs implementation tasks AND at least one task with area "test". Write text in the same language as the features.
Do not write code. Do not modify files.

`)
	b.WriteString("REQUIREMENTS:\n" + mustJSON(p.Requirements) + "\n\n")
	b.WriteString("FEATURES:\n" + mustJSON(p.Features) + "\n\n")
	b.WriteString("FLOWS:\n" + mustJSON(p.Flows) + "\n\n")
	b.WriteString("PAGES (build handoff per page):\n" + mustJSON(Handoffs(p)) + "\n\n")
	if p.Design != nil && p.Design.System != nil {
		b.WriteString("DESIGN SYSTEM:\n" + mustJSON(p.Design.System) + "\n\n")
	}
	b.WriteString("PLATFORMS:\n" + mustJSON(p.Platform.Targets) + "\n\n")
	b.WriteString(structureBrief(p) + "\n")
	if brief := ArchitectureBrief(p); brief != "" {
		b.WriteString(brief + "\n")
	}
	b.WriteString(`Areas: scaffold, data, api, ui, test, integration. If the project is new, the first task (area "scaffold", feature_id "")
must create the chosen layout with real build/test tooling (e.g. go.mod, package.json with build/test scripts) so Foundry can verify later tasks.
Scaffold, data and api tasks follow the architecture blueprint: module folders, layering (controller → service → repository), auth, workers for slow jobs, object storage for files, pagination and indexes as specified.
Do not plan a different architecture, stack, ORM or folder layout.
UI tasks implement pages from their handoff: use its route, access, layout (Design IR), data/api, hierarchy, primary and secondary actions,
interactions, states, validation and responsive behavior, and copy the handoff tests into test_requirements.
UI acceptance criteria also cover: design system tokens used (no arbitrary values), one primary action, every state shown,
no horizontal overflow on mobile, labeled inputs and keyboard focus. API tasks cover every api call listed in the page handoffs.
depends_on lists task ids that must be DONE first — only true prerequisites (scaffold before the rest; an API before its own tests).
Do not chain independent features, pages, or modules.
files lists the fewest concrete file paths this task will create or change (e.g. apps/web/src/pages/Login.tsx). Never a whole tree
(src, app, packages, services, packages/ui). Two tasks that can run in parallel must not share files or the same page.
Build the dependency graph first. Shared foundations run sequentially before their consumers: database schema/migrations,
shared types/models, shared utilities, API contracts (route/DTO definitions, API client) and app wiring (router, navigation menu, DI/module registration).
Each shared file has exactly ONE owner task. The owner creates every entry later tasks need (e.g. registers every planned route and menu item up front).
Consumer tasks depend_on the owner and list only their own files; they must not edit owner files.
Everything else (independent pages, modules, tests) must be parallel: no depends_on between them.
UI tasks: one page per task (page_ids length 1). Two pages = two tasks.
Only area "scaffold" may touch the whole project. Do not use area "integration" unless the work truly spans every package.

Reply with ONLY one JSON object in a ` + "```json" + ` block, shape:
{"tasks": [{"id": "TASK-001", "title": "", "description": "", "area": "api", "requirement_ids": ["REQ-001"], "feature_id": "FEAT-001",
            "page_ids": [], "platform": "backend", "depends_on": [], "acceptance_criteria": [""], "files": [""], "test_requirements": [""]}]}`)
	return b.String()
}

func structureBrief(p *core.Project) string {
	s := p.Structure
	if s == nil {
		return p.Stack.Brief() + "PROJECT STRUCTURE: unknown\n"
	}
	var b strings.Builder
	b.WriteString(p.Stack.Brief())
	if s.Mode == core.StructureExisting {
		b.WriteString("PROJECT STRUCTURE: existing project. Follow it, do not restructure.\n")
		for _, c := range s.Components {
			fmt.Fprintf(&b, "- %s: %s %s\n", c.Kind, c.Path, c.Tech)
		}
		b.WriteString(existingUIBrief(p, true))
	} else if ir := s.IR; ir != nil {
		fmt.Fprintf(&b, "PROJECT STRUCTURE (approved by the user, type %s) — the contract: put code only in these folders, "+
			"do not add top-level folders, services or packages; modules are domain boundaries, never split per endpoint or CRUD operation:\n", ir.Type)
		b.WriteString(structure.Tree(ir))
		for _, ps := range [][]core.StructurePart{ir.Apps, ir.Services, ir.Workers, ir.Packages} {
			for _, x := range ps {
				fmt.Fprintf(&b, "- %s", x.Path)
				if len(x.Modules) > 0 {
					fmt.Fprintf(&b, " modules: %s", strings.Join(x.Modules, ", "))
				}
				if x.Reason != "" {
					b.WriteString(" — " + x.Reason)
				}
				b.WriteString("\n")
			}
		}
	} else {
		fmt.Fprintf(&b, "PROJECT STRUCTURE: new project, template %q, layout:\n", s.Template)
		for _, l := range s.Layout {
			b.WriteString("- " + l + "\n")
		}
	}
	if len(s.Commands) > 0 {
		b.WriteString("VERIFICATION COMMANDS Foundry runs after each task:\n")
		for _, c := range s.Commands {
			fmt.Fprintf(&b, "- (%s) %s in %s\n", c.Name, strings.Join(c.Argv, " "), c.Dir)
		}
	}
	return b.String()
}

// ApplyPlan validates tasks, renumbers IDs, rejects dependency cycles and
// fills coverage gaps so every feature has implementation and test work.
func ApplyPlan(p *core.Project, raw string) error {
	var out planOut
	if err := ExtractJSON(raw, &out); err != nil {
		return err
	}
	idMap := map[string]string{}
	for i, t := range out.Tasks {
		idMap[t.ID] = fmt.Sprintf("TASK-%03d", i+1)
	}
	reqValid := map[string]bool{}
	for _, r := range p.Requirements {
		reqValid[r.ID] = true
	}
	var tasks []core.Task
	for _, t := range out.Tasks {
		if strings.TrimSpace(t.Title) == "" || len(t.AcceptanceCriteria) == 0 {
			continue
		}
		nt := core.Task{ID: idMap[t.ID], Title: t.Title, Description: t.Description, Area: strings.ToLower(t.Area),
			Platform: strings.ToLower(t.Platform), AcceptanceCriteria: t.AcceptanceCriteria, Files: normalizeFiles(t.Files),
			TestRequirements: t.TestRequirements, Status: core.TaskTodo}
		if !taskAreas[nt.Area] {
			nt.Area = "api"
		}
		if f := p.Feature(t.FeatureID); f != nil {
			nt.FeatureID = f.ID
		} else if nt.Area != "scaffold" {
			continue // untraceable task
		}
		for _, r := range t.RequirementIDs {
			if reqValid[r] {
				nt.RequirementIDs = append(nt.RequirementIDs, r)
			}
		}
		if len(nt.RequirementIDs) == 0 && nt.FeatureID != "" {
			nt.RequirementIDs = p.Feature(nt.FeatureID).RequirementIDs
		}
		for _, pg := range t.PageIDs {
			if p.Page(pg) != nil {
				nt.PageIDs = append(nt.PageIDs, pg)
			}
		}
		if nt.Area == "ui" && len(nt.PageIDs) > 1 {
			nt.PageIDs = nt.PageIDs[:1]
		}
		for _, d := range t.DependsOn {
			if m, ok := idMap[d]; ok && m != nt.ID {
				nt.DependsOn = append(nt.DependsOn, m)
			}
		}
		tasks = append(tasks, nt)
	}
	if len(tasks) == 0 {
		return fmt.Errorf("plan tidak berisi task yang valid")
	}
	// Drop deps pointing to removed tasks.
	exists := map[string]bool{}
	for _, t := range tasks {
		exists[t.ID] = true
	}
	for i := range tasks {
		var deps []string
		for _, d := range tasks[i].DependsOn {
			if exists[d] {
				deps = append(deps, d)
			}
		}
		tasks[i].DependsOn = deps
	}
	if c := findCycle(tasks); c != "" {
		return fmt.Errorf("dependency task melingkar di %s", c)
	}
	resolveOwnership(tasks)
	p.Tasks = tasks
	FillCoverage(p)
	p.Verification = nil
	return nil
}

// FillCoverage adds tasks for features lacking implementation or tests, and
// pages no task touches. Generated tasks are derived from the feature data so
// they stay concrete and traceable.
func FillCoverage(p *core.Project) int {
	added := 0
	var scaffoldID string
	for _, t := range p.Tasks {
		if t.Area == "scaffold" {
			scaffoldID = t.ID
		}
	}
	pageCovered := map[string]bool{}
	for _, t := range p.Tasks {
		for _, pg := range t.PageIDs {
			pageCovered[pg] = true
		}
	}
	for _, f := range p.Features {
		var impl, test []string
		for _, t := range p.Tasks {
			if t.FeatureID != f.ID {
				continue
			}
			if t.Area == "test" {
				test = append(test, t.ID)
			} else {
				impl = append(impl, t.ID)
			}
		}
		var subs []string
		for _, s := range f.SubFeatures {
			subs = append(subs, s.Name)
		}
		platform := ""
		if len(f.Platforms) > 0 {
			platform = f.Platforms[0]
		}
		deps := func(base []string) []string {
			if scaffoldID != "" {
				return append([]string{scaffoldID}, base...)
			}
			return base
		}
		if len(impl) == 0 {
			id := NextTaskID(p)
			p.Tasks = append(p.Tasks, core.Task{ID: id, Title: "Implementasi " + f.Name, Area: "api",
				Description:    f.Description + "\nSub-feature: " + strings.Join(subs, ", "),
				RequirementIDs: f.RequirementIDs, FeatureID: f.ID, Platform: platform, DependsOn: deps(nil),
				AcceptanceCriteria: prefixAll("Berfungsi: ", subs), TestRequirements: []string{"Unit test untuk setiap sub-feature " + f.Name},
				Status: core.TaskTodo})
			impl = append(impl, id)
			added++
		}
		if len(test) == 0 {
			p.Tasks = append(p.Tasks, core.Task{ID: NextTaskID(p), Title: "Test " + f.Name, Area: "test",
				Description:    "Tulis test otomatis yang membuktikan feature " + f.Name + " bekerja sesuai sub-feature.",
				RequirementIDs: f.RequirementIDs, FeatureID: f.ID, Platform: platform, DependsOn: impl,
				AcceptanceCriteria: prefixAll("Ada test yang lulus untuk: ", subs), TestRequirements: []string{"Test dijalankan oleh command test project"},
				Status: core.TaskTodo})
			added++
		}
	}
	if p.Design != nil {
		handoffs := Handoffs(p)
		for i, pg := range p.Design.Pages {
			if pageCovered[pg.ID] {
				continue
			}
			h := handoffs[i]
			var dep []string
			for _, t := range p.Tasks {
				if t.FeatureID == pg.FeatureID && t.Area != "ui" && t.Area != "test" {
					dep = append(dep, t.ID)
				}
			}
			p.Tasks = append(p.Tasks, core.Task{ID: NextTaskID(p), Title: "UI " + pg.Name, Area: "ui",
				Description: pg.Purpose, FeatureID: pg.FeatureID, PageIDs: []string{pg.ID}, Platform: pg.Platform,
				RequirementIDs: featureReqs(p, pg.FeatureID), DependsOn: dep,
				AcceptanceCriteria: append([]string{"Halaman " + pg.Name + " tampil sesuai rancangan " + pg.ID}, h.Interactions...),
				TestRequirements:   h.Tests,
				Status:             core.TaskTodo})
			added++
		}
	}
	return added
}

func featureReqs(p *core.Project, fid string) []string {
	if f := p.Feature(fid); f != nil {
		return f.RequirementIDs
	}
	return nil
}

func prefixAll(prefix string, items []string) []string {
	if len(items) == 0 {
		return []string{prefix + "semua alur utama"}
	}
	out := make([]string, len(items))
	for i, s := range items {
		out[i] = prefix + s
	}
	return out
}

func normalizeFiles(files []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, f := range files {
		f = strings.TrimPrefix(path.Clean(strings.ReplaceAll(strings.TrimSpace(f), "\\", "/")), "./")
		if f == "" || f == "." || f == "/" || strings.HasPrefix(f, "../") || strings.Contains(f, "..") {
			continue
		}
		if tooWideFile(f) {
			continue
		}
		if seen[f] {
			continue
		}
		seen[f] = true
		out = append(out, f)
	}
	return out
}

var wideRoots = map[string]bool{
	"src": true, "app": true, "apps": true, "packages": true, "services": true,
	"internal": true, "cmd": true, "lib": true, "pkg": true, "web": true,
	"frontend": true, "backend": true, "mobile": true, "server": true, "client": true,
}

func tooWideFile(f string) bool {
	segs := strings.Split(f, "/")
	file := strings.Contains(segs[len(segs)-1], ".")
	if len(segs) == 1 && !file {
		return true
	}
	return len(segs) == 2 && !file && wideRoots[segs[0]]
}

func NextTaskID(p *core.Project) string {
	max := 0
	for _, t := range p.Tasks {
		var n int
		fmt.Sscanf(t.ID, "TASK-%d", &n)
		if n > max {
			max = n
		}
	}
	return fmt.Sprintf("TASK-%03d", max+1)
}

// resolveOwnership makes file/page ownership explicit: when two tasks hold an
// overlapping "file:" or "page:" lock and neither depends on the other, the
// earlier task (plan order) owns it and the later one waits for it. The
// Kanban then shows the dependency instead of an unexplained lock wait.
// Project-wide and feature locks are ignored. Edges only point from a later
// task to an earlier one that does not reach it, so no cycle is created.
func resolveOwnership(tasks []core.Task) {
	idx := map[string]int{}
	for i, t := range tasks {
		idx[t.ID] = i
	}
	// reaches reports whether task a transitively depends on task b.
	reaches := func(a, b int) bool {
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
	locks := make([][]string, len(tasks))
	for i := range tasks {
		for _, l := range tasks[i].Locks() {
			if strings.HasPrefix(l, "file:") || strings.HasPrefix(l, "page:") {
				locks[i] = append(locks[i], l)
			}
		}
	}
	for j := range tasks {
		for i := 0; i < j; i++ {
			if !ownedOverlap(locks[i], locks[j]) || reaches(j, i) || reaches(i, j) {
				continue
			}
			if !slices.Contains(tasks[j].DependsOn, tasks[i].ID) {
				tasks[j].DependsOn = append(tasks[j].DependsOn, tasks[i].ID)
			}
		}
	}
}

// ownedOverlap mirrors core lock overlap for file and page locks only.
func ownedOverlap(a, b []string) bool {
	for _, x := range a {
		for _, y := range b {
			if x == y {
				return true
			}
			if strings.HasPrefix(x, "file:") && strings.HasPrefix(y, "file:") &&
				(strings.HasPrefix(x, y+"/") || strings.HasPrefix(y, x+"/")) {
				return true
			}
		}
	}
	return false
}

func findCycle(tasks []core.Task) string {
	deps := map[string][]string{}
	for _, t := range tasks {
		deps[t.ID] = t.DependsOn
	}
	state := map[string]int{} // 0 new, 1 visiting, 2 done
	var visit func(id string) string
	visit = func(id string) string {
		switch state[id] {
		case 1:
			return id
		case 2:
			return ""
		}
		state[id] = 1
		for _, d := range deps[id] {
			if c := visit(d); c != "" {
				return c
			}
		}
		state[id] = 2
		return ""
	}
	for _, t := range tasks {
		if c := visit(t.ID); c != "" {
			return c
		}
	}
	return ""
}
