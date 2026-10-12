package work

import (
	"fmt"
	"path"
	"strings"

	"github.com/handrawanw/foundry/internal/core"
	"github.com/handrawanw/foundry/internal/modules/standard"
)

const finishJSON = `{"criteria": [{"index": 1, "met": true, "evidence": "path + what it does"}], "patches": [{"file": "", "change": ""}]}`

// TaskPrompt is the brief an agent receives to implement one task.
func TaskPrompt(p *core.Project, t *core.Task) string {
	var b strings.Builder
	fmt.Fprintf(&b, "You are a coding agent for Foundry on project %q (cwd).\n", p.Name)
	b.WriteString("Implement ONLY this task. Production code + its tests.\n")
	b.WriteString("Follow project structure, FOUNDRY.md, and docs/foundry-profile.md. Do not delete or restructure architecture.\n")
	b.WriteString("Foundry runs build/test after you finish. DONE = they pass AND every acceptance criterion is met.\n")
	b.WriteString(standard.Rules + "\n")
	b.WriteString(workRules + "\n")
	fmt.Fprintf(&b, "TASK %s: %s\nArea: %s  Platform: %s\n\n%s\n\n", t.ID, t.Title, t.Area, t.Platform, t.Description)
	if f := p.Feature(t.FeatureID); f != nil {
		fmt.Fprintf(&b, "FEATURE %s %s: %s\n", f.ID, f.Name, f.Description)
		if len(f.SubFeatures) > 0 {
			names := make([]string, len(f.SubFeatures))
			for i, s := range f.SubFeatures {
				names[i] = s.Name
			}
			b.WriteString("Sub-features: " + strings.Join(names, "; ") + "\n")
		}
		b.WriteByte('\n')
	}
	for _, rid := range t.RequirementIDs {
		for _, r := range p.Requirements {
			if r.ID == rid {
				fmt.Fprintf(&b, "REQUIREMENT %s: %s\n", r.ID, r.Text)
			}
		}
	}
	ui := t.Area == "ui"
	for _, pid := range t.PageIDs {
		if pg := p.Page(pid); pg != nil {
			ui = true
			b.WriteString("\n" + mustJSON(handoff(p, *pg, p.Design.Pages)) + "\n")
		}
	}
	if ui {
		if p.Design != nil && p.Design.System != nil {
			b.WriteString("\n" + systemBrief(p.Design.System))
		}
		b.WriteString("\n" + uiRules)
	}
	if p.PRD != nil && strings.TrimSpace(p.PRD.Markdown) != "" {
		b.WriteString("\nPRODUCT PRD (source of truth): docs/PRD.md. Trace this task through Capability → Business Flow → this TASK-ID. Do not implement beyond that capability.\n")
	}
	b.WriteString("\nACCEPTANCE CRITERIA:\n")
	for i, c := range t.AcceptanceCriteria {
		fmt.Fprintf(&b, "%d. %s\n", i+1, c)
	}
	if len(t.TestRequirements) > 0 {
		b.WriteString("\nTEST REQUIREMENTS:\n- " + strings.Join(t.TestRequirements, "\n- ") + "\n")
	}
	b.WriteString(ownership(p, t))
	if len(t.Notes) > 0 {
		b.WriteString("\nFEEDBACK (fix):\n- " + strings.Join(t.Notes, "\n- ") + "\n")
	}
	b.WriteString("\nWhile working, call update_task ≤3 times, progress ≤80 chars. Example: {\"task_id\":\"" + t.ID + "\",\"progress\":\"Menulis skema user\"}.\n")
	b.WriteString("When you finish, include a ```json block so Foundry can skip a second review pass: " + finishJSON +
		". Every criterion needs evidence. \"patches\" is optional: foreign-file changes only. Do not implement other tasks.\n")
	b.WriteString("\n" + structureBrief(p))
	if brief := ArchitectureBrief(p); brief != "" {
		b.WriteString("\n" + brief)
	}
	return b.String()
}

// systemBrief is the design tokens as compact lines (cheaper than JSON objects).
func systemBrief(s *core.DesignSystem) string {
	if s == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("DESIGN SYSTEM:\n")
	if s.Direction != "" {
		b.WriteString(s.Direction + "\n")
	}
	tok := func(kind string, xs []core.Token) {
		if len(xs) == 0 {
			return
		}
		b.WriteString(kind + ":")
		for i, t := range xs {
			if i > 0 {
				b.WriteByte(';')
			}
			fmt.Fprintf(&b, " %s=%s", t.Name, t.Value)
			if t.Use != "" {
				b.WriteString(" (" + t.Use + ")")
			}
		}
		b.WriteByte('\n')
	}
	tok("colors", s.Colors)
	tok("typography", s.Typography)
	tok("spacing", s.Spacing)
	tok("radius", s.Radius)
	tok("shadow", s.Shadow)
	tok("breakpoints", s.Breakpoints)
	tok("components", s.Components)
	return b.String()
}

// maxOwned bounds the "owned by other tasks" list.
const maxOwned = 30

// ownership tells the agent which files it may edit, which another running
// agent is changing now, and which belong to other pending tasks.
func ownership(p *core.Project, t *core.Task) string {
	var b strings.Builder
	b.WriteString("\nOWNERSHIP:\nYou own:\n")
	mine := map[string]bool{}
	for _, f := range t.Files {
		if k := normPath(f); k != "" {
			mine[k] = true
			b.WriteString("- " + f + "\n")
		}
	}
	if len(mine) == 0 {
		if t.Area == "scaffold" || t.Area == "integration" {
			b.WriteString("- the whole project, but other agents may be running: respect the lists below\n")
		} else {
			b.WriteString("- the feature and pages of this task\n")
		}
	}
	owned := func(f string) bool {
		k := normPath(f)
		for m := range mine {
			if k == m || strings.HasPrefix(k, m+"/") {
				return true
			}
		}
		return false
	}
	seen := map[string]bool{}
	var running []string
	for _, o := range p.ActiveTasks() {
		if o.ID == t.ID {
			continue
		}
		for _, f := range o.Files {
			if k := normPath(f); k != "" && !seen[k] {
				seen[k] = true
				running = append(running, fmt.Sprintf("- %s (%s)", f, o.ID))
			}
		}
		for _, pg := range o.PageIDs {
			if k := "page:" + pg; !seen[k] {
				seen[k] = true
				running = append(running, fmt.Sprintf("- page %s (%s)", pg, o.ID))
			}
		}
	}
	if len(running) > 0 {
		b.WriteString("Running now (do NOT edit, another agent is changing them):\n" + strings.Join(running, "\n") + "\n")
	}
	var other []string
	for i := range p.Tasks {
		o := &p.Tasks[i]
		if o.ID == t.ID || o.Status == core.TaskDone || o.Active() {
			continue
		}
		for _, f := range o.Files {
			if k := normPath(f); k != "" && !seen[k] && !owned(f) {
				seen[k] = true
				other = append(other, fmt.Sprintf("- %s (%s)", f, o.ID))
			}
		}
	}
	if len(other) > 0 {
		b.WriteString("Owned by other tasks (do not edit):\n")
		if len(other) > maxOwned {
			b.WriteString(strings.Join(other[:maxOwned], "\n") + fmt.Sprintf("\n- … and %d more\n", len(other)-maxOwned))
		} else {
			b.WriteString(strings.Join(other, "\n") + "\n")
		}
	}
	b.WriteString("Edit only owned files. New files: next to owned files or this feature's module in PROJECT STRUCTURE. Foreign file needed → do not edit; put {file, change} under \"patches\" in final JSON.\n")
	return b.String()
}

// workRules keep runs cheap and the tree tidy; the stray-file guard in the
// build enforces the last one.
const workRules = `WORK RULES:
- Read only owned files + their imports + shared contracts. No full-repo scan.
- Reuse existing helper/component/type/client in shared folders. Never duplicate.
- Tests next to the code they test, project convention.
- ORM only in repository. Cross-module side effects via events. Function ≤ 40 lines, file ≤ 300.
- No notes, summaries, reports, TODO md, scratch scripts, logs, backups, or new top-level folders. Foundry discards them. Keep FOUNDRY.md and AGENTS.md.
`

// ContinuePrompt asks an agent to finish a partial attempt already in cwd.
// Used when a task returns to the queue and is claimed again.
func ContinuePrompt(p *core.Project, t *core.Task) string {
	var b strings.Builder
	fmt.Fprintf(&b, "You are a coding agent for Foundry on project %q (cwd).\n", p.Name)
	b.WriteString("CONTINUE FROM PARTIAL work. This directory already has a partial implementation of this task.\n")
	b.WriteString("Do not start over or rewrite working code. Finish remaining work until every acceptance criterion is met.\n")
	b.WriteString("Follow project structure. Do not delete or restructure architecture.\n")
	b.WriteString("Foundry runs build/test after you finish. DONE = they pass AND every acceptance criterion is met.\n")
	b.WriteString(workRules + "\n")
	fmt.Fprintf(&b, "TASK %s: %s\nArea: %s  Platform: %s\n\n%s\n\n", t.ID, t.Title, t.Area, t.Platform, t.Description)
	b.WriteString("ACCEPTANCE CRITERIA:\n")
	for i, c := range t.AcceptanceCriteria {
		fmt.Fprintf(&b, "%d. %s\n", i+1, c)
	}
	if len(t.Files) > 0 {
		b.WriteString("\nFILES YOU OWN:\n- " + strings.Join(t.Files, "\n- ") + "\n")
	}
	if len(t.Notes) > 0 {
		b.WriteString("\nFEEDBACK (fix):\n- " + strings.Join(t.Notes, "\n- ") + "\n")
	}
	b.WriteString("\nWhen you finish, include a ```json block: " + finishJSON + ". Every criterion needs evidence.\n")
	return b.String()
}

// BlockedPrompt briefs a retry of a BLOCKED task. It carries what the earlier
// attempts could not: every failure note, the unmet criteria, and the tail of
// the last failing build, with an order to find the root cause before editing.
// hasDraft says the directory still holds the previous attempt's changes.
func BlockedPrompt(p *core.Project, t *core.Task, hasDraft bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "You are a coding agent for Foundry on project %q (cwd).\n", p.Name)
	fmt.Fprintf(&b, "TASK %s was BLOCKED after repeated failed attempts. This is a targeted repair, not a normal run.\n", t.ID)
	b.WriteString("1. Read the failures below and name the ROOT CAUSE before editing. Do not repeat an approach that already failed.\n")
	b.WriteString("2. Fix the cause with the smallest change. Never delete, skip, or weaken tests to make them pass.\n")
	b.WriteString("3. Touch only files this task owns; a change needed elsewhere goes in \"patches\".\n")
	if hasDraft {
		b.WriteString("This directory still holds the previous attempt. Keep what works, fix what failed.\n")
	} else {
		b.WriteString("The previous attempt is gone: implement the task again, applying what the failures teach.\n")
	}
	b.WriteString("Foundry runs build/test after you finish. DONE = they pass AND every acceptance criterion is met.\n")
	b.WriteString(workRules + "\n")
	fmt.Fprintf(&b, "TASK %s: %s\nArea: %s  Platform: %s\n\n%s\n", t.ID, t.Title, t.Area, t.Platform, t.Description)
	b.WriteString("\nACCEPTANCE CRITERIA:\n")
	for i, c := range t.AcceptanceCriteria {
		mark := ""
		if i < len(t.Criteria) && !t.Criteria[i].Met {
			mark = " [NOT MET: " + clip(t.Criteria[i].Evidence, 200) + "]"
		}
		fmt.Fprintf(&b, "%d. %s%s\n", i+1, c, mark)
	}
	if len(t.TestRequirements) > 0 {
		b.WriteString("\nTEST REQUIREMENTS:\n- " + strings.Join(t.TestRequirements, "\n- ") + "\n")
	}
	b.WriteString(ownership(p, t))
	if len(t.Notes) > 0 {
		b.WriteString("\nFAILED ATTEMPTS (oldest first):\n")
		for i, n := range t.Notes {
			fmt.Fprintf(&b, "%d) %s\n", i+1, clip(n, 1500))
		}
	}
	if bld := p.Build(t.LastBuildID); bld != nil && !bld.Passed {
		b.WriteString("\nLAST FAILING BUILD: " + bld.Reason + "\n")
		for _, s := range bld.Steps {
			if s.ExitCode != 0 {
				fmt.Fprintf(&b, "$ %s (exit %d)\n%s\n", s.Command, s.ExitCode, tail(s.Output, 2000))
			}
		}
	}
	if done := doneDeps(p, t); done != "" {
		b.WriteString("\nDEPENDENCIES ALREADY DONE (use them, do not redo): " + done + "\n")
	}
	b.WriteString("\nWhen you finish, include a ```json block: " + finishJSON + ". Every criterion needs evidence.\n")
	b.WriteString("\n" + structureBrief(p))
	return b.String()
}

func doneDeps(p *core.Project, t *core.Task) string {
	var out []string
	for _, id := range t.DependsOn {
		if d := p.Task(id); d != nil && d.Status == core.TaskDone {
			out = append(out, id+" "+d.Title)
		}
	}
	return strings.Join(out, "; ")
}

// RepairPrompt asks an agent to fix its own attempt in place: the worktree
// still holds the changes, so it repairs them instead of starting over.
func RepairPrompt(p *core.Project, t *core.Task, problem string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "You are a coding agent for Foundry on project %q (cwd).\n", p.Name)
	b.WriteString("This directory already has your implementation; it is not accepted yet. Fix ONLY the problem below, in place.\n")
	b.WriteString("Do not start over, rewrite working code, or implement other tasks. Read only files the problem points to.\n\n")
	fmt.Fprintf(&b, "TASK %s: %s\nArea: %s  Platform: %s\n\n%s\n", t.ID, t.Title, t.Area, t.Platform, t.Description)
	b.WriteString("\nACCEPTANCE CRITERIA:\n")
	for i, c := range t.AcceptanceCriteria {
		fmt.Fprintf(&b, "%d. %s\n", i+1, c)
	}
	if len(t.Files) > 0 {
		b.WriteString("\nFILES YOU OWN:\n- " + strings.Join(t.Files, "\n- ") + "\n")
	}
	b.WriteString("\nPROBLEM TO FIX:\n" + problem + "\n")
	b.WriteString("\nWhen you finish, include a ```json block: " + finishJSON + ". Every criterion needs evidence.\n")
	return b.String()
}

// normPath is a file path as a comparable key (same rules as Task.Locks).
func normPath(f string) string {
	f = strings.TrimPrefix(path.Clean(strings.ReplaceAll(strings.TrimSpace(f), "\\", "/")), "./")
	if f == "" || f == "." || f == "/" {
		return ""
	}
	return strings.ToLower(strings.TrimSuffix(f, "/"))
}

// uiRules is how a coding agent builds a designed page.
const uiRules = `UI RULES:
- Map DESIGN SYSTEM tokens once onto project styling (CSS vars, theme, Tailwind). No arbitrary color/size/space/radius/shadow where a token fits.
- Reuse existing components (Button, Input, FormField, Card, Table, Modal, Tabs, Badge, Alert, EmptyState, LoadingState, ErrorState, PageHeader). Create only if none fits; make it reusable.
- Hierarchy follows handoff "hierarchy" (first = most prominent). At most one primary CTA per context; secondary visually lower. Never render the same action twice.
- Every handoff state: loading keeps layout (skeleton, no jump); empty says what/why/next; error says what failed + how to recover.
- Layout = handoff "layout" (Design IR): same frame, nav, width, tree (stack, row, grid, split, sidebar, center, full, overlay), per-device (hide, order, type, columns, swap). Component "#id"s map to the tree. Resolve semantic tokens (space.*, surface.*, action.*, text.*, status.*) from the design system. Do not invent another layout.
- Responsive as handoff "responsive" + "layout.devices": desktop, tablet, mobile. No horizontal page overflow on mobile. Touch targets ≥44px. Do not hide important info.
- A11y: semantic HTML, label every input, visible focus, keyboard nav, WCAG AA, field-tied form errors, accessible names on icon buttons.
- Do not change APIs/business logic for visuals, add routes beyond the handoff, use mock data where real data exists, or remove existing functionality. No extra features, animations or decoration.
`

// Verdict is a reviewer's judgement of one acceptance criterion (1-based).
type Verdict struct {
	Index    int    `json:"index"`
	Met      bool   `json:"met"`
	Evidence string `json:"evidence"`
}

type reviewOut struct {
	Criteria []Verdict `json:"criteria"`
}

// ReviewPrompt asks the AI to judge acceptance criteria against the changes.
func ReviewPrompt(p *core.Project, t *core.Task, changes string, build *core.BuildRun) string {
	var b strings.Builder
	b.WriteString("You are the reviewer of Foundry. Judge strictly: each acceptance criterion is met only with concrete code evidence (file + what it does). Do not modify files.\n\n")
	fmt.Fprintf(&b, "TASK %s: %s\n%s\n\nACCEPTANCE CRITERIA:\n", t.ID, t.Title, t.Description)
	for i, c := range t.AcceptanceCriteria {
		fmt.Fprintf(&b, "%d. %s\n", i+1, c)
	}
	if build != nil {
		b.WriteString("\nVERIFICATION RESULT (run by Foundry):\n")
		for _, s := range build.Steps {
			fmt.Fprintf(&b, "- %s exit %d\n", s.Command, s.ExitCode)
		}
	}
	b.WriteString("\nCHANGED FILES:\n" + changes + "\n")
	b.WriteString("\nReply with ONLY one JSON object in a ```json block: {\"criteria\": [{\"index\": 1, \"met\": true, \"evidence\": \"\"}]}")
	return b.String()
}

// ParseReview maps a review answer onto the task's criteria. Criteria the
// reviewer did not mention count as not met. A claim without evidence is
// not accepted. Progress JSON (update_task) is skipped.
func ParseReview(t *core.Task, raw string) ([]core.Criterion, error) {
	var out reviewOut
	if err := ExtractJSONFunc(raw, &out, func() bool { return len(out.Criteria) > 0 }); err != nil {
		return nil, err
	}
	for i := range out.Criteria {
		if out.Criteria[i].Met && strings.TrimSpace(out.Criteria[i].Evidence) == "" {
			out.Criteria[i].Met = false
		}
	}
	return MergeCriteria(t, out.Criteria), nil
}

// SelfReview is ParseReview when the coding agent already judged criteria.
func SelfReview(t *core.Task, raw string) ([]core.Criterion, bool) {
	crit, err := ParseReview(t, raw)
	if err != nil {
		return nil, false
	}
	return crit, true
}

// MergeCriteria is shared by agent review and MCP submit_task.
func MergeCriteria(t *core.Task, in []Verdict) []core.Criterion {
	res := make([]core.Criterion, len(t.AcceptanceCriteria))
	for i, c := range t.AcceptanceCriteria {
		res[i] = core.Criterion{Text: c}
	}
	for _, c := range in {
		if i := c.Index - 1; i >= 0 && i < len(res) {
			res[i].Met, res[i].Evidence = c.Met, c.Evidence
		}
	}
	return res
}

func AllMet(cs []core.Criterion) bool {
	if len(cs) == 0 {
		return false
	}
	for _, c := range cs {
		if !c.Met {
			return false
		}
	}
	return true
}

// Patch is a change an agent needs in a file it does not own.
type Patch struct {
	File   string `json:"file"`
	Change string `json:"change"`
}

// ParsePatches reads the optional "patches" from an agent's final JSON.
func ParsePatches(raw string) []Patch {
	var out struct {
		Patches []Patch `json:"patches"`
	}
	if ExtractJSONFunc(raw, &out, func() bool { return len(out.Patches) > 0 }) != nil {
		return nil
	}
	return CleanPatches(out.Patches)
}

// CleanPatches drops patches without a file or a change.
func CleanPatches(in []Patch) []Patch {
	var res []Patch
	for _, pt := range in {
		if normPath(pt.File) != "" && strings.TrimSpace(pt.Change) != "" {
			res = append(res, pt)
		}
	}
	return res
}

// PatchOwner is the not-DONE task (other than from) whose files hold file.
func PatchOwner(p *core.Project, from, file string) *core.Task {
	k := normPath(file)
	for i := range p.Tasks {
		o := &p.Tasks[i]
		if o.ID == from || o.Status == core.TaskDone {
			continue
		}
		for _, f := range o.Files {
			if m := normPath(f); m != "" && (k == m || strings.HasPrefix(k, m+"/")) {
				return o
			}
		}
	}
	return nil
}

// ForeignOwner is the pending task owning file when file is outside t's own
// files: an edit there crosses an ownership boundary.
func ForeignOwner(p *core.Project, t *core.Task, file string) *core.Task {
	if DependencyFile(file) {
		return nil
	}
	k := normPath(file)
	for _, f := range t.Files {
		if m := normPath(f); m != "" && (k == m || strings.HasPrefix(k, m+"/")) {
			return nil
		}
	}
	return PatchOwner(p, t.ID, file)
}

// depFiles are dependency manifests and lockfiles: any task adding a
// dependency edits them, so they are never owned; parallel edits meet at
// merge instead.
var depFiles = map[string]bool{
	"package.json": true, "package-lock.json": true, "yarn.lock": true, "pnpm-lock.yaml": true, "bun.lockb": true, "bun.lock": true,
	"go.mod": true, "go.sum": true, "cargo.toml": true, "cargo.lock": true,
	"pyproject.toml": true, "poetry.lock": true, "uv.lock": true, "requirements.txt": true,
	"composer.json": true, "composer.lock": true, "gemfile": true, "gemfile.lock": true,
	"pubspec.yaml": true, "pubspec.lock": true,
}

// DependencyFile reports whether file is a dependency manifest or lockfile.
func DependencyFile(file string) bool {
	return depFiles[path.Base(normPath(file))]
}
