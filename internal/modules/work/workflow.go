package work

import (
	"encoding/json"
	"errors"
	"fmt"
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
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Objective    string   `json:"objective"`
	Context      string   `json:"context"`
	Inputs       []string `json:"inputs"`
	Instructions string   `json:"instructions"`
	DependsOn    []string `json:"depends_on"`
	Artifacts    []string `json:"artifacts"`
	Covers       []string `json:"covers"`
	Outputs      []string `json:"expected_outputs"`
	Acceptance   []string `json:"acceptance_criteria"`
	Constraints  []string `json:"constraints"`
}

type workflowOut struct {
	Commands []commandOut `json:"commands"`
	Remove   []string     `json:"remove"`
}

const commandRules = `Command rules:
- one command is one coherent unit of work an agent can finish and verify on its own.
- derive the commands and their order from THIS blueprint and the real dependencies, not from a fixed list of phases.
- depends_on lists only the commands whose result this one needs (a contract, schema, module, interface). Commands without such a need do not depend on each other, so they can be done in parallel. No cycles, no self dependency.
- artifacts lists the ids of the blueprint artifacts the command implements or must follow (from the list above); at least one. system-overview is a plain-language explanation for humans; do not add a command whose only job is to write or implement it.
- when the blueprint has design-rules and layout-spec, every command that builds or changes screens lists both, and its instructions name the page and shell ids of layout-spec it implements; the design reaches the code only through these commands. Every page id of layout-spec must be named by at least one command that lists layout-spec.
- an API, migration or backend command that does not build screens does not list the design artifacts; only commands that name page ids do.
- covers lists the requirement ids (REQ-...) the command implements; every MUST requirement is covered by at least one command. The command's acceptance_criteria include the criterion ids (REQ-001-AC1) of the acceptance artifact it must prove, and add only command-specific checks.
- when the blueprint has threat-model, data-lifecycle, integrations, operations or migration artifacts, the commands that build the affected parts list them too, so their rules reach the code.
- instructions are specific Markdown: what to build or change, behavior, edge cases, which artifact sections to follow. Never generic ("implement this well"). Understandable without this conversation.
- never invent file paths, endpoints or tables of an existing codebase. When the repository structure is unknown the instruction starts with inspecting the repository.
- tests and verification are part of the instructions and acceptance criteria (or own commands when they span several commands). Foundry never runs them; the agent does.
- expected_outputs: what the agent hands back (code, migration, test, document, report). acceptance_criteria: checkable conditions.
- constraints: limits that apply (technology given by the user, compatibility, performance, security).
- decided decisions override every artifact and command. An instruction, constraint or acceptance criterion that contradicts a decided decision is wrong; follow the decision and rewrite the command.
`

const commandShape = `{"id": "C1", "name": "", "objective": "", "context": "", "inputs": [""], "instructions": "",
 "depends_on": ["C0"], "artifacts": ["requirements"], "covers": ["REQ-001"], "expected_outputs": [""], "acceptance_criteria": [""], "constraints": [""]}`

// WorkflowPrompt asks for the workflow (no commands yet) or for a revision
// of it (the pending request). docs is the whole blueprint.
// When the blueprint is large (total body > threshold), uses two-phase batching
// to avoid context-limit / quota errors:
// Phase 1 (Sketch): Generate command skeleton with artifact summaries only
// Phase 2 (Details): Fill in details for commands in batches
// Small blueprints use the original single-pass prompt.
func WorkflowPrompt(p *core.Project, docs []core.Artifact) string {
	mode := detectWorkflowPromptMode(p, docs)
	switch mode {
	case "sketch":
		return workflowSketchPrompt(p, docs)
	case "detail":
		return workflowDetailBatchPrompt(p, docs)
	case "fresh":
		return workflowFreshPrompt(p, docs)
	default:
		return workflowRevisionPrompt(p, docs)
	}
}

// workflowBatchThreshold is the total artifact body size above which the
// workflow prompt switches to sketch mode to avoid context-limit errors.
const workflowBatchThreshold = 50_000

// detectWorkflowPromptMode decides which prompt style to use.
// Returns "sketch" for large fresh workflows, "detail" for workflows with
// commands that still lack instructions, "fresh" for small new workflows,
// and "" for the original revision prompt.
func detectWorkflowPromptMode(p *core.Project, docs []core.Artifact) string {
	w := p.Workflow

	// Existing commands that still need details → detail phase
	if w != nil && len(w.Commands) > 0 {
		for _, cmd := range w.Commands {
			if cmd.Instructions == "" {
				return "detail"
			}
		}
		// All commands have instructions → revision
		return ""
	}

	// Fresh workflow (no commands)
	totalBody := 0
	for _, d := range docs {
		totalBody += len(d.Body)
	}
	if totalBody > workflowBatchThreshold {
		return "sketch"
	}
	return "fresh"
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
	if p.Structure != nil && p.Structure.Mode == core.StructureExisting {
		b.WriteString("EXISTING CODEBASE: yes. Commands change it; the agent inspects it first.\n")
	} else {
		b.WriteString("EXISTING CODEBASE: no (new project).\n")
	}
	b.WriteString("\n" + surveyWorkflowRules(p))
	writeBinding(&b, p)
	
	// Use artifact summaries only (not full bodies) to reduce context size
	b.WriteString("BLUEPRINT ARTIFACTS (summaries only):\n")
	for _, d := range docs {
		fmt.Fprintf(&b, "=== %s (%s, %s%s) %s\n", d.ID, d.Title, d.Category, map[bool]string{true: ", builtin"}[d.Builtin], d.Summary)
	}
	b.WriteString("\n")
	b.WriteString(precedenceRules)
	
	if w := p.Workflow; w != nil && w.Request != nil && w.Request.Rejected != "" {
		b.WriteString("YOUR LAST ANSWER WAS REFUSED, fix this: " + w.Request.Rejected + "\n\n")
	}
	
	b.WriteString(`SKETCH PHASE - Generate command skeleton only:
- Create 3-25 commands that cover all MUST requirements
- For each command, provide ONLY: id, name, objective, depends_on, artifacts (IDs), covers (requirement IDs)
- DO NOT provide instructions, context, inputs, outputs, or acceptance_criteria yet
- Use ids C1, C2, C3, etc.
- depends_on lists only commands whose result this one needs (a contract, schema, module, interface)
- No cycles, no self dependency
- artifacts lists the blueprint artifact IDs the command implements (from the list above)
- covers lists the requirement IDs (REQ-...) the command implements; every MUST requirement must be covered

` + commandRules)
	b.WriteString("\nReply with ONLY one JSON object in a ```json block:\n{\"commands\": [{\"id\": \"C1\", \"name\": \"\", \"objective\": \"\", \"depends_on\": [], \"artifacts\": [\"requirements\"], \"covers\": [\"REQ-001\"]}]}")
	return b.String()
}

// workflowDetailBatchPrompt generates a prompt for filling in details for a batch of commands
func workflowDetailBatchPrompt(p *core.Project, docs []core.Artifact) string {
	var b strings.Builder
	b.WriteString(`You are the workflow planner of Foundry. Foundry designs software and writes instructions for an EXTERNAL coding agent (Claude Code, Codex, OpenCode, Qwen Code, ...). Foundry never runs them. Do not write application code. Do not modify files.
Write every human-readable text in the same language as the story.

PROJECT: ` + p.Name + "\nSTORY:\n" + p.Story + "\n\n")
	
	w := p.Workflow
	b.WriteString("CURRENT COMMANDS (skeleton):\n" + mustJSON(w.Commands) + "\n\n")
	
	// Find commands that need details (instructions empty)
	var needsDetails []core.Command
	for _, cmd := range w.Commands {
		if cmd.Instructions == "" {
			needsDetails = append(needsDetails, cmd)
		}
	}
	
	// Process in batches of 5 commands
	batchSize := 5
	if len(needsDetails) < batchSize {
		batchSize = len(needsDetails)
	}
	batch := needsDetails[:batchSize]
	
	b.WriteString(fmt.Sprintf("DETAIL PHASE - Fill in details for these %d commands:\n", len(batch)))
	for _, cmd := range batch {
		b.WriteString(fmt.Sprintf("- %s: %s\n", cmd.ID, cmd.Name))
	}
	b.WriteString("\n")
	
	// Include full artifact bodies only for artifacts referenced by this batch
	neededArtifacts := make(map[string]bool)
	for _, cmd := range batch {
		for _, art := range cmd.Artifacts {
			neededArtifacts[art.ID] = true
		}
	}
	
	b.WriteString("BLUEPRINT ARTIFACTS (full details for this batch only):\n")
	for _, d := range docs {
		if neededArtifacts[d.ID] {
			// Include full body for needed artifacts
			fmt.Fprintf(&b, "=== %s (%s, %s%s) %s\n%s\n", d.ID, d.Title, d.Category, map[bool]string{true: ", builtin"}[d.Builtin], d.Summary, d.Body)
		} else {
			// Include summary only for others
			fmt.Fprintf(&b, "=== %s (%s, %s%s) %s\n", d.ID, d.Title, d.Category, map[bool]string{true: ", builtin"}[d.Builtin], d.Summary)
		}
	}
	b.WriteString("\n")
	b.WriteString(precedenceRules)
	
	b.WriteString(`For each command in the batch, provide:
- context: background and why this command exists
- instructions: specific Markdown instructions (what to build/change, behavior, edge cases, which artifact sections to follow). Never generic. Understandable without this conversation.
- inputs: what the command needs (from dependencies)
- expected_outputs: what the agent hands back (code, migration, test, document, report)
- acceptance_criteria: checkable conditions (include criterion IDs like REQ-001-AC1)
- constraints: limits that apply (technology, compatibility, performance, security)

` + commandRules)
	b.WriteString("\nReply with ONLY one JSON object in a ```json block:\n{\"commands\": [{\"id\": \"C1\", \"context\": \"\", \"instructions\": \"\", \"inputs\": [\"\"], \"expected_outputs\": [\"\"], \"acceptance_criteria\": [\"\"], \"constraints\": [\"\"]}]}")
	return b.String()
}

// workflowFreshPrompt generates the prompt for creating a new workflow (no commands yet)
func workflowFreshPrompt(p *core.Project, docs []core.Artifact) string {
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
	if p.Structure != nil && p.Structure.Mode == core.StructureExisting {
		b.WriteString("EXISTING CODEBASE: yes. Commands change it; the agent inspects it first.\n")
	} else {
		b.WriteString("EXISTING CODEBASE: no (new project).\n")
	}
	b.WriteString("\n" + surveyWorkflowRules(p))
	writeBinding(&b, p)
	cut := promptBody
	if w := p.Workflow; w != nil && w.Request != nil && w.Request.Sync {
		cut = 0
	}
	writeDocs(&b, docs, cut)
	b.WriteString("Turn the blueprint into a command workflow of 3-25 commands. Use ids C1, C2, ...\n\n")
	if w := p.Workflow; w != nil && w.Request != nil && w.Request.Rejected != "" {
		b.WriteString("YOUR LAST ANSWER WAS REFUSED, fix this: " + w.Request.Rejected + "\n\n")
	}
	b.WriteString(commandRules)
	b.WriteString("\nReply with ONLY one JSON object in a ```json block:\n{\"commands\": [" + commandShape + "]}")
	return b.String()
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
	if p.Structure != nil && p.Structure.Mode == core.StructureExisting {
		b.WriteString("EXISTING CODEBASE: yes. Commands change it; the agent inspects it first.\n")
	} else {
		b.WriteString("EXISTING CODEBASE: no (new project).\n")
	}
	b.WriteString("\n" + surveyWorkflowRules(p))
	writeBinding(&b, p)
	cut := promptBody
	if w := p.Workflow; w != nil && w.Request != nil && w.Request.Sync {
		cut = 0
	}
	writeDocs(&b, docs, cut)
	w := p.Workflow
	b.WriteString("CURRENT COMMANDS:\n" + mustJSON(w.Commands) + "\n\n")
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
		if r.Rejected != "" {
			b.WriteString("YOUR LAST ANSWER WAS REFUSED, fix this: " + r.Rejected + "\n\n")
		}
	}
	b.WriteString(commandRules)
	b.WriteString(`Revision rules:
- change only the commands the request affects. Return only commands you add or change, each in full. Keep the id of a changed command; give new commands ids NEW1, NEW2, ...
- when the request asks to follow changed blueprint artifacts, return every command whose artifacts changed even if its text stays valid; that records the new artifact versions.
- remove lists the ids of commands to delete; commands that depended on them must be returned with fixed depends_on.
- commands not returned stay as they are. A long answer may be cut short; Foundry asks again for the commands you did not return yet.

Reply with ONLY one JSON object in a ` + "```json" + ` block:
{"commands": [` + commandShape + `], "remove": ["CMD-003"]}`)
	return b.String()
}

// ApplyWorkflow lands the AI's workflow or revision. The result must pass
// the workflow check (references, no cycle) or nothing changes.
// Routes based on response content:
// - Commands with instructions → complete workflow (original behavior)
// - Commands without instructions → sketch phase (triggers detail batches)
// - Existing skeleton + commands with details → detail batch phase
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
	if fresh && !hasInstructions && len(out.Commands) > 0 {
		// Fresh workflow + no instructions → sketch phase
		return applyWorkflowSketchFromParsed(p, out, docs)
	}

	if !fresh {
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
			return applyWorkflowDetailBatchFromParsed(p, out, docs)
		}
	}

	// Default: original revision behavior
	return applyWorkflowRevisionFromParsed(p, out, docs)
}

// applyWorkflowRevisionFromParsed applies a revision using already-parsed output.
func applyWorkflowRevisionFromParsed(p *core.Project, out workflowOut, docs []core.Artifact) error {
	w := p.Workflow
	fresh := w == nil || len(w.Commands) == 0
	if fresh && len(out.Commands) == 0 {
		return errors.New("workflow kosong: tidak ada command")
	}
	hashes := workflow.Hashes(docs)
	if !fresh && len(out.Commands) == 0 && len(out.Remove) == 0 {
		if w.Request != nil && w.Request.Sync {
			acceptStale(w, w.Request.Focus, hashes) // reviewed, text stays valid
			w.Request = nil
			w.Version++
			return nil
		}
		return errors.New("AI tidak menemukan perubahan untuk permintaan ini; tulis lebih spesifik")
	}
	var staleBefore map[string][]string
	if !fresh {
		staleBefore = workflow.Stale(w.Commands, hashes)
	}
	var cmds []core.Command
	if !fresh {
		cmds = slices.Clone(w.Commands)
		for _, id := range out.Remove {
			i := slices.IndexFunc(cmds, func(c core.Command) bool { return c.ID == id })
			if i < 0 {
				return fmt.Errorf("command %s yang dihapus tidak ada", id)
			}
			cmds = slices.Delete(cmds, i, i+1)
		}
	}
	// New commands get the next free numbers; AI ids map to them.
	idMap := map[string]string{}
	next := 1
	for _, c := range cmds {
		var n int
		if _, err := fmt.Sscanf(c.ID, "CMD-%d", &n); err == nil && n >= next {
			next = n + 1
		}
		idMap[c.ID] = c.ID
	}
	for _, o := range out.Commands {
		id := strings.TrimSpace(o.ID)
		if _, kept := idMap[id]; kept && !fresh {
			continue
		}
		if _, dup := idMap[id]; dup || id == "" {
			return fmt.Errorf("id command %q kosong atau ganda", id)
		}
		idMap[id] = fmt.Sprintf("CMD-%03d", next)
		next++
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
	if err := checkCommands(cmds, hashes, p.Requirements); err != nil {
		return err
	}
	if issues := CheckUI(p, cmds); len(issues) > 0 {
		return UIError(issues)
	}
	// A new workflow covers every MUST requirement; an older workflow is
	// only reported (it may predate covers).
	if _, uncovered := workflow.Covers(cmds, p.Requirements); fresh && len(uncovered) > 0 {
		return fmt.Errorf("workflow tidak valid: requirement MUST belum dicakup command mana pun (covers): %s", strings.Join(uncovered, ", "))
	}
	if w == nil {
		w = &core.Workflow{}
		p.Workflow = w
	}
	w.Commands, w.Status = cmds, core.CommandDraft
	w.Version++
	pendingRequest(w, staleBefore, hashes)
	return nil
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
			return err
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
	if err := checkSketchCommands(cmds, hashes, p.Requirements); err != nil {
		return err
	}
	if _, uncovered := workflow.Covers(cmds, p.Requirements); len(uncovered) > 0 {
		return fmt.Errorf("workflow tidak valid: requirement MUST belum dicakup command mana pun (covers): %s", strings.Join(uncovered, ", "))
	}
	if p.Workflow == nil {
		p.Workflow = &core.Workflow{}
	}
	p.Workflow.Commands = cmds
	p.Workflow.Status = core.CommandDraft
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
	// Build a map of command details from the output
	detailsMap := make(map[string]commandOut)
	for _, o := range out.Commands {
		id := strings.TrimSpace(o.ID)
		if id != "" {
			detailsMap[id] = o
		}
	}
	// Apply details to existing commands that lack instructions
	updated := 0
	for i := range w.Commands {
		cmd := &w.Commands[i]
		if cmd.Instructions != "" {
			continue // Already has details
		}
		d, ok := detailsMap[cmd.ID]
		if !ok {
			continue
		}
		cmd.Context = strings.TrimSpace(d.Context)
		cmd.Instructions = strings.TrimSpace(d.Instructions)
		cmd.Inputs = nonEmpty(d.Inputs)
		cmd.Outputs = nonEmpty(d.Outputs)
		cmd.Acceptance = nonEmpty(d.Acceptance)
		cmd.Constraints = nonEmpty(d.Constraints)
		updated++
	}
	if updated == 0 {
		return errors.New("tidak ada command yang diupdate; pastikan id command sesuai")
	}
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
		if err := checkCommands(w.Commands, hashes, p.Requirements); err != nil {
			return err
		}
		if issues := CheckUI(p, w.Commands); len(issues) > 0 {
			return UIError(issues)
		}
		pendingRequest(w, nil, hashes)
		w.Request = nil
	} else {
		if err := checkSketchCommands(w.Commands, hashes, p.Requirements); err != nil {
			return err
		}
		remaining := 0
		for _, cmd := range w.Commands {
			if cmd.Instructions == "" {
				remaining++
			}
		}
		w.Request.Text = fmt.Sprintf("Fill in details for remaining %d commands", remaining)
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

// pendingRequest keeps a change request alive while the answer changed only
// some of the focused stale commands; the orchestrator runs another round for
// the rest. No progress clears it, so a revision always terminates.
func pendingRequest(w *core.Workflow, staleBefore map[string][]string, hashes map[string]string) {
	if w.Request == nil {
		return
	}
	focus := w.Request.Focus
	staleAfter := workflow.Stale(w.Commands, hashes)
	remaining, progressed := false, false
	for _, c := range w.Commands {
		if len(focus) > 0 && !slices.Contains(focus, c.ID) {
			continue
		}
		before := len(staleBefore[c.ID]) > 0
		after := len(staleAfter[c.ID]) > 0
		remaining = remaining || after
		progressed = progressed || before && !after
	}
	if !remaining || !progressed {
		w.Request = nil
	}
}

func commandFrom(o commandOut, idMap, hashes map[string]string) (core.Command, error) {
	c := core.Command{ID: idMap[strings.TrimSpace(o.ID)], Name: strings.TrimSpace(o.Name), Objective: strings.TrimSpace(o.Objective),
		Context: strings.TrimSpace(o.Context), Inputs: nonEmpty(o.Inputs), Instructions: strings.TrimSpace(o.Instructions),
		Outputs: nonEmpty(o.Outputs), Acceptance: nonEmpty(o.Acceptance), Constraints: nonEmpty(o.Constraints), Status: core.CommandDraft}
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

// checkSketchCommands validates sketch phase commands (no instructions required)
func checkSketchCommands(cmds []core.Command, hashes map[string]string, reqs []core.Requirement) error {
	issues := validateSketch(cmds, hashes)
	unknown, _ := workflow.Covers(cmds, reqs)
	issues = append(issues, unknown...)
	if len(issues) == 0 {
		return nil
	}
	var msgs []string
	for i, is := range issues {
		if i == 5 {
			msgs = append(msgs, fmt.Sprintf("… %d lagi", len(issues)-5))
			break
		}
		msgs = append(msgs, strings.TrimSpace(is.Command+" "+is.Message))
	}
	return fmt.Errorf("workflow tidak valid: %s", strings.Join(msgs, "; "))
}

// checkCommands refuses a workflow that fails the check.
func checkCommands(cmds []core.Command, hashes map[string]string, reqs []core.Requirement) error {
	exists := map[string]bool{}
	for id := range hashes {
		exists[id] = true
	}
	issues := workflow.Validate(cmds, exists)
	unknown, _ := workflow.Covers(cmds, reqs)
	issues = append(issues, unknown...)
	if len(issues) == 0 {
		return nil
	}
	var msgs []string
	for i, is := range issues {
		if i == 5 {
			msgs = append(msgs, fmt.Sprintf("… %d lagi", len(issues)-5))
			break
		}
		msgs = append(msgs, strings.TrimSpace(is.Command+" "+is.Message))
	}
	return fmt.Errorf("workflow tidak valid: %s", strings.Join(msgs, "; "))
}

// CommandEdit is a user's edit of one command.
type CommandEdit struct {
	Name         string   `json:"name"`
	Objective    string   `json:"objective"`
	Context      string   `json:"context"`
	Inputs       []string `json:"inputs"`
	Instructions string   `json:"instructions"`
	DependsOn    []string `json:"depends_on"`
	Artifacts    []string `json:"artifacts"`
	Covers       []string `json:"covers"`
	Outputs      []string `json:"expected_outputs"`
	Acceptance   []string `json:"acceptance_criteria"`
	Constraints  []string `json:"constraints"`
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
		Instructions: in.Instructions, DependsOn: in.DependsOn, Artifacts: in.Artifacts, Covers: in.Covers, Outputs: in.Outputs,
		Acceptance: in.Acceptance, Constraints: in.Constraints}, idMap, hashes)
	if err != nil {
		return err
	}
	cmds := slices.Clone(p.Workflow.Commands)
	cmds[slices.IndexFunc(cmds, func(x core.Command) bool { return x.ID == id })] = next
	if err := checkCommands(cmds, hashes, p.Requirements); err != nil {
		return err
	}
	if issues := CheckUI(p, cmds); len(issues) > 0 {
		return UIError(issues)
	}
	p.Workflow.Commands = cmds
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
