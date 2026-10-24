package work

import (
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
- when the blueprint has design-rules and layout-spec, every command that builds or changes screens lists both, and its instructions name the page and shell ids of layout-spec it implements; the design reaches the code only through these commands.
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
func WorkflowPrompt(p *core.Project, docs []core.Artifact) string {
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
	if w == nil || len(w.Commands) == 0 {
		b.WriteString("Turn the blueprint into a command workflow of 3-25 commands. Use ids C1, C2, ...\n\n")
		if w != nil && w.Request != nil && w.Request.Rejected != "" {
			b.WriteString("YOUR LAST ANSWER WAS REFUSED, fix this: " + w.Request.Rejected + "\n\n")
		}
		b.WriteString(commandRules)
		b.WriteString("\nReply with ONLY one JSON object in a ```json block:\n{\"commands\": [" + commandShape + "]}")
		return b.String()
	}
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
- remove lists the ids of commands to delete; commands that depended on them must be returned with fixed depends_on.
- commands not returned stay as they are.

Reply with ONLY one JSON object in a ` + "```json" + ` block:
{"commands": [` + commandShape + `], "remove": ["CMD-003"]}`)
	return b.String()
}

// ApplyWorkflow lands the AI's workflow or revision. The result must pass
// the workflow check (references, no cycle) or nothing changes.
func ApplyWorkflow(p *core.Project, raw string, docs []core.Artifact) error {
	var out workflowOut
	if err := ExtractJSON(raw, &out); err != nil {
		return err
	}
	w := p.Workflow
	fresh := w == nil || len(w.Commands) == 0
	if fresh && len(out.Commands) == 0 {
		return errors.New("workflow kosong: tidak ada command")
	}
	if !fresh && len(out.Commands) == 0 && len(out.Remove) == 0 {
		if w.Request != nil && w.Request.Sync {
			w.Request = nil // no command contradicts the changed decisions
			return nil
		}
		return errors.New("AI tidak menemukan perubahan untuk permintaan ini; tulis lebih spesifik")
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
	hashes := workflow.Hashes(docs)
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
	// A new workflow covers every MUST requirement; an older workflow is
	// only reported (it may predate covers).
	if _, uncovered := workflow.Covers(cmds, p.Requirements); fresh && len(uncovered) > 0 {
		return fmt.Errorf("workflow tidak valid: requirement MUST belum dicakup command mana pun (covers): %s", strings.Join(uncovered, ", "))
	}
	if w == nil {
		w = &core.Workflow{}
		p.Workflow = w
	}
	w.Commands, w.Request, w.Status = cmds, nil, core.CommandDraft
	w.Version++
	return nil
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
// command ids; unknown ones are refused.
func RequestWorkflowChange(p *core.Project, text string, focus []string) error {
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
	p.Workflow.Request = &core.ChangeRequest{Text: text, Focus: focus}
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
