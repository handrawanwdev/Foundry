package work

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/workflow"
)

// KindWorkflow writes the command workflow, or revises it when the
// workflow has commands and a pending request.
const KindWorkflow = "workflow"

// MaxCommands bounds one workflow.
const MaxCommands = 60

type commandOut struct {
	ID            string      `json:"id"`
	Name          string      `json:"name"`
	Objective     string      `json:"objective"`
	Context       string      `json:"context"`
	Inputs        []string    `json:"inputs"`
	Instructions  string      `json:"instructions"`
	DependsOn     []string    `json:"depends_on"`
	Artifacts     artifactIDs `json:"artifacts"`
	Covers        []string    `json:"covers"`
	FeatureIDs    []string    `json:"feature_ids"`
	SubFeatureIDs []string    `json:"sub_feature_ids"`
	Outputs       []string    `json:"expected_outputs"`
	Acceptance    []string    `json:"acceptance_criteria"`
	Constraints   []string    `json:"constraints"`
	Files         []string    `json:"files"`
	Priority      string      `json:"priority"`
	Complexity    string      `json:"complexity"`
}

// artifactIDs reads the artifacts of an answered command: ids, or the
// {"id", "hash"} objects the prompt shows for the current commands, which a
// model often copies back. Foundry sets the hash itself either way.
type artifactIDs []string

func (a *artifactIDs) UnmarshalJSON(b []byte) error {
	var raw []json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	out := make(artifactIDs, 0, len(raw))
	for _, r := range raw {
		var id string
		if err := json.Unmarshal(r, &id); err != nil {
			var ref struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(r, &ref); err != nil || ref.ID == "" {
				return fmt.Errorf("artefak harus id atau {\"id\": ...}, bukan %s", r)
			}
			id = ref.ID
		}
		out = append(out, id)
	}
	*a = out
	return nil
}

type workflowOut struct {
	Commands []commandOut `json:"commands"`
	Remove   []string     `json:"remove"`
	// Done ends a revision: this answer is the last change it needs.
	Done bool `json:"done"`
}

const commandRules = `Command rules:
- one command is one coherent unit of work an agent can finish and verify on its own.
- derive the commands and their order from THIS blueprint and the real dependencies, not from a fixed list of phases.
- depends_on lists only the commands whose result this one needs (a contract, schema, module, interface). Commands without such a need do not depend on each other, so they can be done in parallel. No cycles, no self dependency.
- a business rule that is a data invariant (stock never negative, a transfer is atomic) becomes an instruction and an acceptance criterion with a test; a transaction must fail without partial change. Never a generic "implement all features".
- access by role is explicit: a command that changes protected data names the permission(s) from authz that gate the action, and its acceptance criteria prove an actor without the permission is refused.
- artifacts lists the ids of the blueprint artifacts the command implements or must follow (from the list above); at least one. system-overview is a plain-language explanation for humans; do not add a command whose only job is to write or implement it.
- when the blueprint has design-rules and layout-spec, every command that builds or changes screens lists both, and its instructions name the page and shell ids of layout-spec it implements; the design reaches the code only through these commands. Every page id of layout-spec must be named by at least one command that lists layout-spec.
- an API, migration or backend command that does not build screens does not list the design artifacts; only commands that name page ids do.
- covers lists the requirement ids (REQ-...) the command implements; every MUST requirement is covered by at least one command. The command's acceptance_criteria include the criterion ids (REQ-001-AC1) of the acceptance artifact it must prove, and add only command-specific checks.
- feature_ids lists the feature ids (FEAT-...) this command implements; sub_feature_ids lists the specific subfeature ids (FEAT-001.1). Link every command to the feature it serves. Every subfeature id belongs to a feature listed in the same command's feature_ids (FEAT-001.1 needs FEAT-001). A command that serves a whole feature may leave sub_feature_ids empty. Do not invent ids.
- when the blueprint has threat-model, data-lifecycle, integrations, operations or migration artifacts, the commands that build the affected parts list them too, so their rules reach the code.
- instructions are specific Markdown: what to build or change, behavior, edge cases, which artifact sections to follow. Never generic ("implement this well"). Understandable without this conversation.
- never invent file paths, endpoints or tables of an existing codebase. When the repository structure is unknown the instruction starts with inspecting the repository.
- tests and verification are part of the instructions and acceptance criteria (or own commands when they span several commands). Foundry never runs them; the agent does.
- expected_outputs: what the agent hands back (code, migration, test, document, report). acceptance_criteria: checkable conditions.
- constraints: limits that apply (technology given by the user, compatibility, performance, security).
- every command has one main goal and does not repeat the instructions of another command; shared work gets its own command that the others depend on. A command with more than about 8 acceptance criteria is split into logical commands; work too small to verify on its own is merged into the command it belongs to.
- acceptance_criteria are measurable: name the observable result, not "works well".
- files lists the files, folders or modules the command touches, taken only from the project structure and repository plan above; leave it empty when they are unknown.
- priority is high (others wait for it, or a core MUST flow), medium or low. complexity is the estimated effort: low, medium or high.
- decided decisions override every artifact and command. An instruction, constraint or acceptance criterion that contradicts a decided decision is wrong; follow the decision and rewrite the command.
- the project structure and its rules are the source of truth for folders, module boundaries and dependencies. Every command creates or changes code only in those folders, follows the dependency rules, and never invents a different layout or a new top-level folder.
`

const commandShape = `{"id": "C1", "name": "", "objective": "", "context": "", "inputs": [""], "instructions": "",
 "depends_on": ["C0"], "artifacts": ["requirements"], "covers": ["REQ-001"], "feature_ids": ["FEAT-001"], "sub_feature_ids": ["FEAT-001.1"], "expected_outputs": [""], "acceptance_criteria": [""], "constraints": [""], "files": [""], "priority": "high", "complexity": "medium"}`

// writeFeatureCatalog lists the feature tree so every command can name the
// feature and subfeature it implements instead of forcing Foundry to guess.
func writeFeatureCatalog(b *strings.Builder, p *core.Project) {
	if p == nil || len(p.Features) == 0 {
		return
	}
	b.WriteString("FEATURES (link commands with feature_ids and sub_feature_ids):\n")
	for _, f := range p.Features {
		fmt.Fprintf(b, "- %s %s", f.ID, f.Name)
		if len(f.RequirementIDs) > 0 {
			fmt.Fprintf(b, " (requirements %s)", strings.Join(f.RequirementIDs, ", "))
		}
		b.WriteString("\n")
		for _, s := range f.SubFeatures {
			fmt.Fprintf(b, "  - %s %s\n", s.ID, s.Name)
		}
	}
	b.WriteString("\n")
}

// WorkflowPrompt asks for the workflow (no commands yet) or for a revision
// of it (the pending request). docs is the whole blueprint.
// Generation is always sequential, whatever the blueprint size:
// Phase 1 (Sketch): the command skeleton, with artifact summaries only
// Phase 2 (Details): one command per round, in dependency order
func WorkflowPrompt(p *core.Project, docs []core.Artifact) string {
	switch detectWorkflowPromptMode(p) {
	case "sketch":
		return workflowSketchPrompt(p, docs)
	case "detail":
		return workflowDetailBatchPrompt(p, docs)
	default:
		return workflowRevisionPrompt(p, docs)
	}
}

// workflowSketchFullBody is the total artifact body size up to which the
// outline prompt carries the artifact bodies; above it, summaries only.
const workflowSketchFullBody = 50_000

// WorkflowPhase names the round the next workflow prompt asks for: sketch,
// detail or revision.
func WorkflowPhase(p *core.Project) string {
	if m := detectWorkflowPromptMode(p); m != "" {
		return m
	}
	return "revision"
}

// detectWorkflowPromptMode decides which prompt style to use: "sketch" for
// a new workflow, "detail" while commands still lack instructions, and ""
// for a revision of a complete workflow.
func detectWorkflowPromptMode(p *core.Project) string {
	w := p.Workflow
	if w == nil || len(w.Commands) == 0 {
		return "sketch"
	}
	if w.Unfilled() > 0 {
		return "detail"
	}
	return ""
}

// workflowSketchPrompt generates a prompt for command skeleton only
// Uses artifact summaries instead of full bodies to reduce context size
func workflowSketchPrompt(p *core.Project, docs []core.Artifact) string {
	var b strings.Builder
	b.WriteString(`You are the workflow planner of Foundry. Foundry designs software and writes instructions for an EXTERNAL coding agent (Claude Code, Codex, OpenCode, Qwen Code, ...). Foundry never runs them. Do not write application code. Do not modify files.
Write every human-readable text in the same language as the story.

PROJECT: ` + p.Name + "\nSTORY:\n" + p.Story + "\n\n")
	if p.Platform != nil {
		var kinds []string
		for _, t := range p.Platform.Targets {
			kinds = append(kinds, t.Kind)
		}
		b.WriteString("PLATFORM: " + strings.Join(kinds, ", ") + "\n")
	}
	writeCodebaseBrief(&b, p)
	b.WriteString("\n" + surveyWorkflowRules(p))
	writeBinding(&b, p)
	writeStackBrief(&b, p)
	writeStructureBrief(&b, p, "PROJECT STRUCTURE (source of truth for folders, module boundaries and dependencies)")
	writeRepoBrief(&b, p)
	writeFeatureCatalog(&b, p)
	writeRequirementRegistry(&b, p)

	// A small blueprint is read whole (long artifacts cut); a large one by
	// summaries only, so the outline stays within the context limit.
	totalBody := 0
	for _, d := range docs {
		totalBody += len(d.Body)
	}
	if totalBody <= workflowSketchFullBody {
		writeDocs(&b, docs, promptBody)
	} else {
		b.WriteString("BLUEPRINT ARTIFACTS (summaries only):\n")
		for _, d := range docs {
			fmt.Fprintf(&b, "=== %s (%s, %s%s) %s\n", d.ID, d.Title, d.Category, map[bool]string{true: ", builtin"}[d.Builtin], d.Summary)
		}
		b.WriteString("\n")
	}
	b.WriteString(precedenceRules)

	if w := p.Workflow; w != nil && w.Request != nil && w.Request.Rejected != "" {
		b.WriteString("YOUR LAST ANSWER WAS REFUSED, fix this: " + w.Request.Rejected + "\n\n")
		writeCoverageRepair(&b, p, uncoveredRequirements(p, w.Request.Rejected))
	}

	b.WriteString(`SKETCH PHASE - Generate command skeleton only:
- Create 3-25 commands that cover all MUST requirements
- For each command, provide ONLY: id, name, objective, depends_on, artifacts (IDs), covers (requirement IDs), feature_ids (FEAT ids), sub_feature_ids (FEAT-001.1 ids), priority, complexity
- DO NOT provide instructions, context, inputs, outputs, or acceptance_criteria yet
- Use ids C1, C2, C3, etc.
- depends_on lists only commands whose result this one needs (a contract, schema, module, interface)
- No cycles, no self dependency
- artifacts lists the blueprint artifact IDs the command implements (from the list above)
- covers lists the requirement IDs (REQ-...) the command implements; every MUST requirement must be covered
- before replying, check the REQUIREMENTS registry: every MUST id appears in the covers of a command that builds what its acceptance criteria check. A cross-cutting MUST (security, durability, recovery, observability) gets the command that implements it, not a mention
- feature_ids lists the feature IDs (FEAT-...) the command implements; sub_feature_ids lists the specific subfeature IDs. Every subfeature belongs to a feature in the same command's feature_ids (FEAT-007.1 needs FEAT-007)

` + commandRules)
	if HasUI(p) {
		b.WriteString(VisualVerificationRules)
	}
	b.WriteString("\nReply with ONLY one JSON object in a ```json block:\n{\"commands\": [{\"id\": \"C1\", \"name\": \"\", \"objective\": \"\", \"depends_on\": [], \"artifacts\": [\"requirements\"], \"covers\": [\"REQ-001\"], \"feature_ids\": [\"FEAT-001\"], \"sub_feature_ids\": [\"FEAT-001.1\"], \"priority\": \"high\", \"complexity\": \"medium\"}]}")
	return b.String()
}

// nextFillCommand is the command the next detail round writes: the first
// unfilled command, in workflow order, whose dependencies are all filled, so
// every command is written after the commands it builds on. Without such a
// command (a dependency cycle) it falls back to the first unfilled one.
func nextFillCommand(cmds []core.Command) *core.Command {
	filled := map[string]bool{}
	for _, c := range cmds {
		if strings.TrimSpace(c.Instructions) != "" {
			filled[c.ID] = true
		}
	}
	var first *core.Command
	for i := range cmds {
		c := &cmds[i]
		if filled[c.ID] {
			continue
		}
		if first == nil {
			first = c
		}
		ready := true
		for _, d := range c.DependsOn {
			if !filled[d] {
				ready = false
				break
			}
		}
		if ready {
			return c
		}
	}
	return first
}

// skeletonCommands is the workflow a detail batch reads: what each command
// is and how they connect. The text of finished commands stays out, or every
// batch would carry all the batches before it; only the outputs of the
// commands the batch depends on stay, since its inputs come from them.
func skeletonCommands(cmds, batch []core.Command) []core.Command {
	deps := map[string]bool{}
	for _, c := range batch {
		for _, d := range c.DependsOn {
			deps[d] = true
		}
	}
	out := slices.Clone(cmds)
	for i := range out {
		c := &out[i]
		c.Context, c.Instructions, c.Inputs, c.Acceptance, c.Constraints = "", "", nil, nil, nil
		if !deps[c.ID] {
			c.Outputs = nil
		}
	}
	return out
}

// workflowRevisionPrompt is the original prompt for revising an existing complete workflow
func workflowRevisionPrompt(p *core.Project, docs []core.Artifact) string {
	var b strings.Builder
	b.WriteString(`You are the workflow planner of Foundry. Foundry designs software and writes instructions for an EXTERNAL coding agent (Claude Code, Codex, OpenCode, Qwen Code, ...). Foundry never runs them. Do not write application code. Do not modify files.
Write every human-readable text in the same language as the story.

PROJECT: ` + p.Name + "\nSTORY:\n" + p.Story + "\n\n")
	if p.Platform != nil {
		var kinds []string
		for _, t := range p.Platform.Targets {
			kinds = append(kinds, t.Kind)
		}
		b.WriteString("PLATFORM: " + strings.Join(kinds, ", ") + "\n")
	}
	writeCodebaseBrief(&b, p)
	b.WriteString("\n" + surveyWorkflowRules(p))
	writeBinding(&b, p)
	writeStackBrief(&b, p)
	writeStructureBrief(&b, p, "PROJECT STRUCTURE (source of truth for folders, module boundaries and dependencies)")
	writeRepoBrief(&b, p)
	writeFeatureCatalog(&b, p)
	cut := promptBody
	if w := p.Workflow; w != nil && w.Request != nil && w.Request.Sync {
		cut = 0
	}
	w := p.Workflow
	scope := revisionScope(w)
	if scope == nil {
		writeDocs(&b, docs, cut)
		b.WriteString(CacheBreak) // the commands below change with every round; the blueprint above does not
		b.WriteString("CURRENT COMMANDS:\n" + mustJSON(w.Commands) + "\n\n")
	} else {
		var full []core.Command
		needed := map[string]bool{}
		for _, c := range w.Commands {
			if scope[c.ID] {
				full = append(full, c)
				for _, a := range c.Artifacts {
					needed[a.ID] = true
				}
			}
		}
		writeScopedDocs(&b, docs, cut, needed)
		b.WriteString(CacheBreak)
		b.WriteString("CURRENT COMMANDS (full text: the commands this request is about, the commands earlier rounds changed, and every command that depends on them):\n" + mustJSON(full) + "\n\n")
		b.WriteString("OTHER COMMANDS (outline only, with the outputs the commands above build on; they stay as they are, never return them):\n" + mustJSON(skeletonCommands(outside(w.Commands, scope), full)) + "\n\n")
	}
	writeExecutionState(&b, p)
	stale := workflow.Stale(w.Commands, workflow.Hashes(docs))
	if len(stale) > 0 {
		b.WriteString("Commands whose artifacts changed since they were written:\n")
		for _, c := range w.Commands {
			if s := stale[c.ID]; len(s) > 0 {
				fmt.Fprintf(&b, "- %s: %s\n", c.ID, strings.Join(s, ", "))
			}
		}
		b.WriteString("\n")
	}
	if r := w.Request; r != nil {
		b.WriteString("USER REQUEST:\n" + r.Text + "\n\n")
		if len(r.Focus) > 0 {
			b.WriteString("Commands the request is about: " + strings.Join(r.Focus, ", ") + ".\n\n")
		}
		if len(r.Handled) > 0 {
			b.WriteString("EARLIER ROUNDS of this request (already saved in CURRENT COMMANDS): " + strings.Join(r.Handled, ", ") + ".\n\n")
		}
		if r.Rejected != "" {
			b.WriteString("YOUR LAST ANSWER WAS REFUSED, fix this: " + r.Rejected + "\n\n")
		}
	}
	b.WriteString(commandRules)
	if HasUI(p) {
		b.WriteString(VisualVerificationRules)
	}
	b.WriteString(`Revision rules (Foundry runs the request in rounds, ONE command change per round):
- each answer makes exactly one change: return one command (changed or new) in commands, or put one id in remove and leave commands empty. Never more than one.
- pick the next change the request still needs, in dependency order: a command before the commands that depend on it. Do not repeat a change from the earlier rounds.
- return the command in full. Keep the id of a changed command; give a new command the id NEW1.
- removing a command hands its dependencies to the commands that depended on it; change those in later rounds only when their text has to change.
- when the request asks to follow changed blueprint artifacts, return every command whose artifacts changed, one per round, even if its text stays valid; that records the new artifact versions.
- set done to true when this answer is the last change the request needs, false when more commands still need a change. When nothing is left to change, reply {"commands": [], "done": true}.
- a changed command has to be done again by the coding agent, so leave commands the agent already finished as they are unless the request or the blueprint really requires a change. Add what is missing as new commands instead.

Reply with ONLY one JSON object in a ` + "```json" + ` block:
{"commands": [` + commandShape + `], "remove": [], "done": false}`)
	return b.String()
}

// revisionScope is the set of commands a focused revision reads in full:
// the focus, the commands earlier rounds of the request changed or added,
// and every command that depends on them, directly or through others, since
// a change can travel down that chain one round at a time. Dependencies stay
// in the outline with their outputs, which is all the scope reads from them.
// nil means every command: an unfocused request (coverage, decision sync), a
// focus that no longer exists, or a scope that covers them all anyway.
func revisionScope(w *core.Workflow) map[string]bool {
	if w == nil || w.Request == nil || len(w.Request.Focus) == 0 {
		return nil
	}
	exists := map[string]bool{}
	for _, c := range w.Commands {
		exists[c.ID] = true
	}
	scope := map[string]bool{}
	for _, id := range w.Request.Focus {
		if exists[id] {
			scope[id] = true
		}
	}
	if len(scope) == 0 {
		return nil
	}
	for _, h := range w.Request.Handled { // "changed CMD-002", "added CMD-007"
		if _, id, ok := strings.Cut(h, " "); ok && exists[id] {
			scope[id] = true
		}
	}
	for grew := true; grew; { // dependents to a fixed point; a cycle stops it too
		grew = false
		for _, c := range w.Commands {
			if scope[c.ID] {
				continue
			}
			if slices.ContainsFunc(c.DependsOn, func(d string) bool { return scope[d] }) {
				scope[c.ID], grew = true, true
			}
		}
	}
	if len(scope) >= len(w.Commands) {
		return nil
	}
	return scope
}

func outside(cmds []core.Command, scope map[string]bool) []core.Command {
	var out []core.Command
	for _, c := range cmds {
		if !scope[c.ID] {
			out = append(out, c)
		}
	}
	return out
}

// writeScopedDocs is writeDocs for a focused revision: the artifacts the
// scoped commands reference and the binding ones keep their body, the rest
// are listed by summary.
func writeScopedDocs(b *strings.Builder, docs []core.Artifact, cut int, needed map[string]bool) {
	b.WriteString("BLUEPRINT ARTIFACTS (body only for the artifacts the commands below reference; the rest by summary):\n")
	for _, d := range docs {
		tag := fmt.Sprintf("=== %s (%s, %s%s) %s\n", d.ID, d.Title, d.Category, map[bool]string{true: ", builtin"}[d.Builtin], d.Summary)
		if !needed[d.ID] && !bindingDocs[d.ID] {
			b.WriteString(tag)
			continue
		}
		body := d.Body
		if cut > 0 && len(body) > cut && !bindingDocs[d.ID] {
			body = strings.ToValidUTF8(body[:cut], "") + "\n…(dipotong)"
		}
		b.WriteString(tag + body + "\n")
	}
	b.WriteString("\n")
	b.WriteString(precedenceRules)
}

// writeExecutionState tells a revision how far the agent got, so it does not
// rewrite finished work without a reason.
func writeExecutionState(b *strings.Builder, p *core.Project) {
	if p.Progress == nil || p.Workflow == nil {
		return
	}
	var lines []string
	for _, c := range p.Workflow.Commands {
		if run, ok := p.Progress.Commands[c.ID]; ok && run.Status != "" && run.Status != core.RunUnconfirmed {
			lines = append(lines, "- "+c.ID+": "+run.Status)
		}
	}
	if len(lines) > 0 {
		b.WriteString("EXECUTION STATE reported by the coding agent:\n" + strings.Join(lines, "\n") + "\n\n")
	}
}

// ApplyWorkflow lands the AI's workflow or revision. The result must pass
// the workflow check (references, no cycle) or nothing changes.
// Routes based on response content:
//   - New workflow → sketch phase; any instructions in the answer are dropped
//     and every command is filled in its own round
//   - Existing skeleton + commands with details → detail phase (one command)
//   - Complete workflow → revision
//
// Visual verification commands can be added explicitly via AddVisualVerificationCommands.
func ApplyWorkflow(p *core.Project, raw string, docs []core.Artifact) error {
	var out workflowOut
	err := ExtractJSONFunc(raw, &out, func() bool { return out.Commands != nil || out.Remove != nil })
	if err != nil {
		if cmds := salvageCommands(raw); len(cmds) > 0 {
			out = workflowOut{Commands: cmds}
		} else {
			return err
		}
	}

	w := p.Workflow
	fresh := w == nil || len(w.Commands) == 0

	// Check if response has instructions
	hasInstructions := false
	for _, o := range out.Commands {
		if strings.TrimSpace(o.Instructions) != "" {
			hasInstructions = true
			break
		}
	}

	// Route based on project state and response content
	var applyErr error
	if fresh && len(out.Commands) > 0 {
		// Fresh workflow → sketch phase, filled one command per round
		applyErr = applyWorkflowSketchFromParsed(p, out, docs)
	} else if !fresh {
		// Check if existing commands need details
		needsDetails := false
		for _, cmd := range w.Commands {
			if cmd.Instructions == "" {
				needsDetails = true
				break
			}
		}
		if needsDetails && hasInstructions {
			// Skeleton exists + response has instructions → detail batch
			applyErr = applyWorkflowDetailBatchFromParsed(p, out, docs)
		} else {
			// Default: original revision behavior
			applyErr = applyWorkflowRevisionFromParsed(p, out, docs)
		}
	} else {
		// Default: original revision behavior
		applyErr = applyWorkflowRevisionFromParsed(p, out, docs)
	}

	return applyErr
}

// maxRevisionRounds ends a revision that never says it is done.
const maxRevisionRounds = MaxCommands

// applyWorkflowRevisionFromParsed applies one round of a revision: exactly
// one command changed or added, or one command removed. The request stays
// open for the next round until the answer says done (or is empty); the UI
// check, which a half-done revision may fail, runs on the last round.
func applyWorkflowRevisionFromParsed(p *core.Project, out workflowOut, docs []core.Artifact) error {
	w := p.Workflow
	if w == nil || len(w.Commands) == 0 {
		return errors.New("workflow kosong: tidak ada command")
	}
	hashes := workflow.Hashes(docs)
	r := w.Request
	if r == nil {
		r = &core.ChangeRequest{}
	}
	if n := len(out.Commands) + len(out.Remove); n > 1 {
		return fmt.Errorf("revisi harus satu command per jawaban, jawaban berisi %d perubahan; kirim satu saja, sisanya di putaran berikutnya", n)
	}
	if len(out.Commands) == 0 && len(out.Remove) == 0 {
		switch {
		case r.Sync:
			acceptStale(w, r.Focus, hashes) // reviewed, text stays valid
		case len(r.Handled) == 0:
			return errors.New("AI tidak menemukan perubahan untuk permintaan ini; tulis lebih spesifik")
		}
		if issues := CheckUI(p, w.Commands); len(issues) > 0 {
			return UIError(issues)
		}
		w.Request = nil
		w.Version++
		return nil
	}
	cmds := slices.Clone(w.Commands)
	var did string
	for _, id := range out.Remove {
		i := slices.IndexFunc(cmds, func(c core.Command) bool { return c.ID == id })
		if i < 0 {
			return fmt.Errorf("command %s yang dihapus tidak ada", id)
		}
		gone := cmds[i]
		cmds = slices.Delete(cmds, i, i+1)
		rewireDependents(cmds, gone)
		did = "removed " + id
	}
	// A new command gets the next free number; its AI id maps to it.
	idMap := map[string]string{}
	for _, c := range cmds {
		idMap[c.ID] = c.ID
	}
	next := nextCommandNumber(p, cmds)
	// A focused revision sent the commands outside its scope as an outline
	// only; returning one would replace its text with a guess.
	scope := revisionScope(w)
	for _, o := range out.Commands {
		id := strings.TrimSpace(o.ID)
		if id == "" {
			return errors.New("id command kosong")
		}
		if _, kept := idMap[id]; kept {
			if scope != nil && !scope[id] {
				return fmt.Errorf("command %s di luar cakupan permintaan ini dan hanya dikirim sebagai outline; jangan kembalikan command itu. Ubah command dalam cakupan, tambahkan command baru, atau set done bila tidak ada yang tersisa", id)
			}
			did = "changed " + id
			continue
		}
		idMap[id] = fmt.Sprintf("CMD-%03d", next)
		did = "added " + idMap[id]
	}
	for _, o := range out.Commands {
		c, err := commandFrom(o, idMap, hashes)
		if err != nil {
			return err
		}
		if i := slices.IndexFunc(cmds, func(x core.Command) bool { return x.ID == c.ID }); i >= 0 {
			cmds[i] = c
		} else {
			cmds = append(cmds, c)
		}
	}
	if len(cmds) > MaxCommands {
		return fmt.Errorf("terlalu banyak command (%d, maks %d)", len(cmds), MaxCommands)
	}
	if err := checkCommands(cmds, hashes, p.Requirements, p.Features); err != nil {
		return err
	}
	// The revision ends when the AI says so, repeats a change (no progress),
	// runs out of rounds, or (sync) has no focused stale command left.
	// Only the AI's done (or nothing stale left) reviews the rest of a sync
	// request; a revision cut off by a repeat or the round limit leaves the
	// remaining stale commands marked for review.
	reviewed := out.Done || r.Sync && !focusStale(cmds, r.Focus, hashes)
	last := reviewed || slices.Contains(r.Handled, did) || len(r.Handled)+1 >= maxRevisionRounds
	if last {
		if issues := CheckUI(p, cmds); len(issues) > 0 {
			return UIError(issues)
		}
	}
	w.Commands, w.Status = cmds, core.CommandDraft
	w.EnsureID()
	w.Version++
	switch {
	case reviewed && r.Sync:
		acceptStale(w, r.Focus, hashes) // the rest was reviewed: text stays valid
		w.Request = nil
	case last:
		w.Request = nil
	case w.Request != nil:
		w.Request.Handled = append(w.Request.Handled, did)
		w.Request.Rejected, w.Request.Retries = "", 0 // the next round gets its own retries
	}
	return nil
}

// rewireDependents hands a removed command's dependencies to the commands
// that depended on it, so the order the removal leaves stays valid.
func rewireDependents(cmds []core.Command, gone core.Command) {
	for i := range cmds {
		c := &cmds[i]
		at := slices.Index(c.DependsOn, gone.ID)
		if at < 0 {
			continue
		}
		deps := slices.Delete(slices.Clone(c.DependsOn), at, at+1)
		for _, d := range gone.DependsOn {
			if d != c.ID && !slices.Contains(deps, d) {
				deps = append(deps, d)
			}
		}
		c.DependsOn = deps
	}
}

// focusStale reports whether a focused command still refers to an artifact
// that changed after it was written.
func focusStale(cmds []core.Command, focus []string, hashes map[string]string) bool {
	stale := workflow.Stale(cmds, hashes)
	for _, c := range cmds {
		if (len(focus) == 0 || slices.Contains(focus, c.ID)) && len(stale[c.ID]) > 0 {
			return true
		}
	}
	return false
}

// nextCommandNumber is the first number never used: not by a command, nor by
// one removed earlier, whose id still names its run and its history.
func nextCommandNumber(p *core.Project, cmds []core.Command) int {
	next := 1
	use := func(id string) {
		var n int
		if _, err := fmt.Sscanf(id, "CMD-%d", &n); err == nil && n >= next {
			next = n + 1
		}
	}
	for _, c := range cmds {
		use(c.ID)
	}
	if p.Progress != nil {
		for id := range p.Progress.Retired {
			use(id)
		}
	}
	if p.Workflow != nil {
		for _, ch := range p.Workflow.Changes {
			for _, r := range ch.Removed {
				use(r.ID)
			}
		}
	}
	return next
}

// applyWorkflowSketchFromParsed applies a command skeleton (no instructions yet)
// and triggers the detail phase.
func applyWorkflowSketchFromParsed(p *core.Project, out workflowOut, docs []core.Artifact) error {
	if len(out.Commands) == 0 {
		return errors.New("workflow kosong: tidak ada command")
	}
	hashes := workflow.Hashes(docs)
	var cmds []core.Command
	idMap := map[string]string{}
	next := 1
	for _, o := range out.Commands {
		id := strings.TrimSpace(o.ID)
		if _, dup := idMap[id]; dup || id == "" {
			return fmt.Errorf("id command %q kosong atau ganda", id)
		}
		idMap[id] = fmt.Sprintf("CMD-%03d", next)
		next++
	}
	for _, o := range out.Commands {
		c, err := commandFrom(o, idMap, hashes)
		if err != nil {
			return answerIDs(err, idMap)
		}
		// Instructions are empty — will be filled in detail phase
		c.Instructions = ""
		c.Status = core.CommandDraft
		cmds = append(cmds, c)
	}
	if len(cmds) > MaxCommands {
		return fmt.Errorf("terlalu banyak command (%d, maks %d)", len(cmds), MaxCommands)
	}
	// Validate sketch (structure only — no instructions required)
	if err := checkSketchCommands(cmds, hashes, p.Requirements, p.Features); err != nil {
		return answerIDs(err, idMap)
	}
	if _, uncovered := workflow.Covers(cmds, p.Requirements); len(uncovered) > 0 {
		return fmt.Errorf("workflow tidak valid: %s%s", uncoveredMust, strings.Join(uncovered, ", "))
	}
	if p.Workflow == nil {
		p.Workflow = &core.Workflow{}
	}
	p.Workflow.Commands = cmds
	p.Workflow.Status = core.CommandDraft
	p.Workflow.EnsureID()
	p.Workflow.Version++
	// Set Request to trigger detail phase
	p.Workflow.Request = &core.ChangeRequest{
		Text: "Fill in details for all commands (context, instructions, acceptance criteria, etc.)",
	}
	return nil
}

// applyWorkflowDetailBatchFromParsed applies details for a batch of commands.
func applyWorkflowDetailBatchFromParsed(p *core.Project, out workflowOut, docs []core.Artifact) error {
	if len(out.Commands) == 0 {
		return errors.New("tidak ada detail command yang diberikan")
	}
	w := p.Workflow
	hashes := workflow.Hashes(docs)
	// Rounds are sequential: only the command this round asked for is taken;
	// details for any other command are ignored and written in its own round.
	next := nextFillCommand(w.Commands)
	if next == nil {
		return errors.New("tidak ada command yang perlu diisi")
	}
	detailsMap := make(map[string]commandOut)
	for _, o := range out.Commands {
		if id := strings.TrimSpace(o.ID); id == next.ID {
			detailsMap[id] = o
		}
	}
	updated := 0
	for i := range w.Commands {
		cmd := &w.Commands[i]
		if cmd.ID != next.ID {
			continue
		}
		d, ok := detailsMap[cmd.ID]
		if !ok || strings.TrimSpace(d.Instructions) == "" {
			continue
		}
		cmd.Context = strings.TrimSpace(d.Context)
		cmd.Instructions = strings.TrimSpace(d.Instructions)
		cmd.Inputs = nonEmpty(d.Inputs)
		cmd.Outputs = nonEmpty(d.Outputs)
		cmd.Acceptance = nonEmpty(d.Acceptance)
		cmd.Constraints = nonEmpty(d.Constraints)
		cmd.Files = nonEmpty(d.Files)
		if lv := level(d.Priority); lv != "" {
			cmd.Priority = lv
		}
		if lv := level(d.Complexity); lv != "" {
			cmd.Complexity = lv
		}
		if len(d.FeatureIDs) > 0 {
			cmd.FeatureIDs = nonEmpty(d.FeatureIDs)
		}
		if len(d.SubFeatureIDs) > 0 {
			cmd.SubFeatureIDs = nonEmpty(d.SubFeatureIDs)
		}
		updated++
	}
	if updated == 0 {
		return fmt.Errorf("detail untuk %s tidak ada; pastikan id command sesuai", next.ID)
	}
	w.EnsureID()
	w.Version++
	// Check if all commands now have details
	allDone := true
	for _, cmd := range w.Commands {
		if cmd.Instructions == "" {
			allDone = false
			break
		}
	}
	// Validate based on completion status
	if allDone {
		if err := checkCommands(w.Commands, hashes, p.Requirements, p.Features); err != nil {
			return err
		}
		if issues := CheckUI(p, w.Commands); len(issues) > 0 {
			return UIError(issues)
		}
		w.Request = nil
	} else {
		if err := checkSketchCommands(w.Commands, hashes, p.Requirements, p.Features); err != nil {
			return err
		}
		remaining := 0
		for _, cmd := range w.Commands {
			if cmd.Instructions == "" {
				remaining++
			}
		}
		w.Request = &core.ChangeRequest{Text: fmt.Sprintf("Fill in details for remaining %d commands", remaining)}
	}
	return nil
}

// salvageCommands recovers the complete command objects from an answer that
// was cut off at the model's output limit.
func salvageCommands(raw string) []commandOut {
	i := strings.LastIndex(raw, `"commands"`)
	if i < 0 {
		return nil
	}
	rest := raw[i+len(`"commands"`):]
	j := strings.IndexByte(rest, '[')
	if j < 0 {
		return nil
	}
	dec := json.NewDecoder(strings.NewReader(rest[j:]))
	if _, err := dec.Token(); err != nil {
		return nil
	}
	var out []commandOut
	for dec.More() {
		var o commandOut
		if err := dec.Decode(&o); err != nil {
			break
		}
		if strings.TrimSpace(o.ID) != "" {
			out = append(out, o)
		}
	}
	return out
}

// acceptStale records the current artifact versions on the focused commands:
// their text still matches the blueprint, so nothing needs rewriting.
func acceptStale(w *core.Workflow, focus []string, hashes map[string]string) {
	for i := range w.Commands {
		c := &w.Commands[i]
		if len(focus) > 0 && !slices.Contains(focus, c.ID) {
			continue
		}
		for j, r := range c.Artifacts {
			if h, ok := hashes[r.ID]; ok {
				c.Artifacts[j].Hash = h
			}
		}
		c.Status, c.Review = core.CommandDraft, ""
	}
}

func commandFrom(o commandOut, idMap, hashes map[string]string) (core.Command, error) {
	c := core.Command{ID: idMap[strings.TrimSpace(o.ID)], Name: strings.TrimSpace(o.Name), Objective: strings.TrimSpace(o.Objective),
		Context: strings.TrimSpace(o.Context), Inputs: nonEmpty(o.Inputs), Instructions: strings.TrimSpace(o.Instructions),
		Outputs: nonEmpty(o.Outputs), Acceptance: nonEmpty(o.Acceptance), Constraints: nonEmpty(o.Constraints), Status: core.CommandDraft,
		FeatureIDs: nonEmpty(o.FeatureIDs), SubFeatureIDs: nonEmpty(o.SubFeatureIDs),
		Files: nonEmpty(o.Files), Priority: level(o.Priority), Complexity: level(o.Complexity)}
	for _, d := range o.DependsOn {
		id, ok := idMap[strings.TrimSpace(d)]
		if !ok {
			return c, fmt.Errorf("%s bergantung pada command %q yang tidak ada", c.ID, d)
		}
		if !slices.Contains(c.DependsOn, id) {
			c.DependsOn = append(c.DependsOn, id)
		}
	}
	refs, err := artifactRefs(o.Artifacts, hashes)
	if err != nil {
		return c, fmt.Errorf("%s: %w", c.ID, err)
	}
	c.Artifacts = refs
	for _, id := range o.Covers {
		if id = strings.ToUpper(strings.TrimSpace(id)); id != "" && !slices.Contains(c.Covers, id) {
			c.Covers = append(c.Covers, id)
		}
	}
	return c, nil
}

// level normalizes a priority or complexity; anything else is left unset.
func level(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case core.LevelHigh, "tinggi":
		return core.LevelHigh
	case core.LevelMedium, "sedang":
		return core.LevelMedium
	case core.LevelLow, "rendah":
		return core.LevelLow
	}
	return ""
}

// artifactRefs resolves artifact ids with their current hashes.
func artifactRefs(ids []string, hashes map[string]string) ([]core.ArtifactRef, error) {
	var out []core.ArtifactRef
	for _, id := range ids {
		id = strings.TrimSpace(id)
		h, ok := hashes[id]
		if !ok {
			return nil, fmt.Errorf("artefak blueprint %q tidak ada", id)
		}
		if !slices.ContainsFunc(out, func(r core.ArtifactRef) bool { return r.ID == id }) {
			out = append(out, core.ArtifactRef{ID: id, Hash: h})
		}
	}
	return out, nil
}

// validateSketch checks only structural validity for sketch phase:
// IDs, dependencies, artifacts, and cycles. Does NOT require instructions.
func validateSketch(cmds []core.Command, hashes map[string]string) []workflow.Issue {
	exists := map[string]bool{}
	for id := range hashes {
		exists[id] = true
	}

	var issues []workflow.Issue
	add := func(id, format string, a ...any) {
		issues = append(issues, workflow.Issue{Command: id, Message: fmt.Sprintf(format, a...)})
	}

	// Check for duplicate/missing IDs
	ids := map[string]bool{}
	for _, c := range cmds {
		switch {
		case strings.TrimSpace(c.ID) == "":
			add("", "command tanpa ID (%q)", c.Name)
			continue
		case ids[c.ID]:
			add(c.ID, "ID %s dipakai lebih dari satu command", c.ID)
		}
		ids[c.ID] = true
	}

	// Check dependencies and artifacts (but NOT instructions)
	for _, c := range cmds {
		if c.ID == "" {
			continue
		}
		if strings.TrimSpace(c.Name) == "" {
			add(c.ID, "nama belum diisi")
		}
		if strings.TrimSpace(c.Objective) == "" {
			add(c.ID, "tujuan belum diisi")
		}
		// Skip instruction check for sketch phase
		for _, d := range c.DependsOn {
			switch {
			case d == c.ID:
				add(c.ID, "bergantung pada dirinya sendiri")
			case !ids[d]:
				add(c.ID, "dependency %s tidak ada", d)
			}
		}
		if len(c.Artifacts) == 0 {
			add(c.ID, "belum merujuk artefak blueprint")
		}
		for _, r := range c.Artifacts {
			if !exists[r.ID] {
				add(c.ID, "artefak blueprint %q tidak ada", r.ID)
			}
		}
	}

	// Check for cycles
	if cycle := workflow.Cycle(cmds); len(cycle) > 0 {
		add(cycle[0], "dependency melingkar: %s", strings.Join(cycle, " → "))
	}

	return issues
}

// maxIssuesShown bounds the problems one refusal lists; the retry prompt
// must show the AI every problem it can fix in one answer.
const maxIssuesShown = 40

// invalidWorkflow is the refusal listing the check's problems.
func invalidWorkflow(issues []workflow.Issue) error {
	var msgs []string
	for i, is := range issues {
		if i == maxIssuesShown {
			msgs = append(msgs, fmt.Sprintf("… %d lagi", len(issues)-maxIssuesShown))
			break
		}
		msgs = append(msgs, strings.TrimSpace(is.Command+" "+is.Message))
	}
	return fmt.Errorf("workflow tidak valid: %s", strings.Join(msgs, "; "))
}

var commandID = regexp.MustCompile(`CMD-\d+`)

// answerIDs writes an outline refusal with the ids the answer used: the
// outline is renumbered before the check, and an id the AI never wrote
// cannot be fixed.
func answerIDs(err error, idMap map[string]string) error {
	back := make(map[string]string, len(idMap))
	for orig, id := range idMap {
		back[id] = orig
	}
	return errors.New(commandID.ReplaceAllStringFunc(err.Error(), func(id string) string {
		if orig, ok := back[id]; ok {
			return orig
		}
		return id
	}))
}

// checkSketchCommands validates sketch phase commands (no instructions required)
func checkSketchCommands(cmds []core.Command, hashes map[string]string, reqs []core.Requirement, feats []core.Feature) error {
	issues := validateSketch(cmds, hashes)
	unknown, _ := workflow.Covers(cmds, reqs)
	issues = append(issues, unknown...)
	issues = append(issues, workflow.ValidateFeatures(cmds, feats)...)
	if len(issues) == 0 {
		return nil
	}
	return invalidWorkflow(issues)
}

// checkCommands refuses a workflow that fails the check.
func checkCommands(cmds []core.Command, hashes map[string]string, reqs []core.Requirement, feats []core.Feature) error {
	exists := map[string]bool{}
	for id := range hashes {
		exists[id] = true
	}
	issues := workflow.Validate(cmds, exists)
	unknown, _ := workflow.Covers(cmds, reqs)
	issues = append(issues, unknown...)
	issues = append(issues, workflow.ValidateFeatures(cmds, feats)...)
	if len(issues) == 0 {
		return nil
	}
	return invalidWorkflow(issues)
}

// CommandEdit is a user's edit of one command.
type CommandEdit struct {
	Name          string   `json:"name"`
	Objective     string   `json:"objective"`
	Context       string   `json:"context"`
	Inputs        []string `json:"inputs"`
	Instructions  string   `json:"instructions"`
	DependsOn     []string `json:"depends_on"`
	Artifacts     []string `json:"artifacts"`
	Covers        []string `json:"covers"`
	FeatureIDs    []string `json:"feature_ids"`
	SubFeatureIDs []string `json:"sub_feature_ids"`
	Outputs       []string `json:"expected_outputs"`
	Acceptance    []string `json:"acceptance_criteria"`
	Constraints   []string `json:"constraints"`
	Files         []string `json:"files"`
	Priority      string   `json:"priority"`
	Complexity    string   `json:"complexity"`
}

// EditCommand saves a user's edit. The workflow must still pass the check;
// editing counts as reviewing the command against its artifacts.
func EditCommand(p *core.Project, id string, in CommandEdit, docs []core.Artifact) error {
	c := p.Workflow.Command(id)
	if c == nil {
		return fmt.Errorf("command %s tidak ada", id)
	}
	idMap := map[string]string{}
	for _, x := range p.Workflow.Commands {
		idMap[x.ID] = x.ID
	}
	hashes := workflow.Hashes(docs)
	next, err := commandFrom(commandOut{ID: id, Name: in.Name, Objective: in.Objective, Context: in.Context, Inputs: in.Inputs,
		Instructions: in.Instructions, DependsOn: in.DependsOn, Artifacts: in.Artifacts, Covers: in.Covers,
		FeatureIDs: in.FeatureIDs, SubFeatureIDs: in.SubFeatureIDs, Outputs: in.Outputs,
		Acceptance: in.Acceptance, Constraints: in.Constraints, Files: in.Files,
		Priority: in.Priority, Complexity: in.Complexity}, idMap, hashes)
	if err != nil {
		return err
	}
	cmds := slices.Clone(p.Workflow.Commands)
	cmds[slices.IndexFunc(cmds, func(x core.Command) bool { return x.ID == id })] = next
	if err := checkCommands(cmds, hashes, p.Requirements, p.Features); err != nil {
		return err
	}
	if issues := CheckUI(p, cmds); len(issues) > 0 {
		return UIError(issues)
	}
	p.Workflow.Commands = cmds
	p.Workflow.EnsureID()
	p.Workflow.Version++
	return nil
}

// ReviewCommand marks a command for review (needs) or as reviewed: the
// latter records the current artifact hashes, so the change that made it
// stale is accepted.
func ReviewCommand(p *core.Project, id string, needs bool, reason string, docs []core.Artifact) error {
	c := p.Workflow.Command(id)
	if c == nil {
		return fmt.Errorf("command %s tidak ada", id)
	}
	if needs {
		c.Status, c.Review = core.CommandNeedsReview, strings.TrimSpace(reason)
		if c.Review == "" {
			c.Review = "ditandai perlu ditinjau"
		}
		return nil
	}
	hashes := workflow.Hashes(docs)
	for i, r := range c.Artifacts {
		if h, ok := hashes[r.ID]; ok {
			c.Artifacts[i].Hash = h
		}
	}
	c.Status, c.Review = core.CommandDraft, ""
	p.Workflow.Version++
	return nil
}

// RequestWorkflowChange records a change request for the AI. focus are
// command ids; unknown ones are refused. sync marks a request that only
// brings the commands in line with changed artifacts, so an empty answer
// means nothing needs rewriting.
func RequestWorkflowChange(p *core.Project, text string, focus []string, sync bool) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return errors.New("tuliskan perubahan yang diinginkan")
	}
	if p.Workflow == nil || len(p.Workflow.Commands) == 0 {
		return errors.New("workflow belum disusun")
	}
	for _, id := range focus {
		if p.Workflow.Command(id) == nil {
			return fmt.Errorf("command %s tidak ada", id)
		}
	}
	p.Workflow.Request = &core.ChangeRequest{Text: text, Focus: focus, Sync: sync}
	return nil
}

// RequestArtifactChange records a blueprint change request for the AI.
func RequestArtifactChange(p *core.Project, text string, focus []string, docs []core.Artifact) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return errors.New("tuliskan perubahan yang diinginkan")
	}
	for _, id := range focus {
		if !slices.ContainsFunc(docs, func(d core.Artifact) bool { return d.ID == id }) {
			return fmt.Errorf("artefak %q tidak ada", id)
		}
	}
	p.ArtifactRequest = &core.ChangeRequest{Text: text, Focus: focus}
	return nil
}
