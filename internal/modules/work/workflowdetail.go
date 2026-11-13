package work

import (
	"fmt"
	"slices"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
)

// A detail round writes one command. Its prompt has two parts:
//
// Static, the same text in every round of the workflow (the provider keeps
// it in its prompt cache): story, binding decisions, technology, structure,
// repository plan, feature catalog and the rules for writing a command.
//
// Dynamic, only what this round needs: the outline, the command, the outputs
// of the commands it builds on, the requirements it covers with every
// acceptance criterion whole, and the blueprint artifacts it implements cut
// to the sections about it (scopeArtifact). The artifacts it does not
// implement stay out: a round cannot change which artifacts a command
// implements.

// The artifacts a command implements share detailArtifactsBudget; each
// gets an equal share within [minArtifactShare, maxArtifactShare], and one
// shorter than its share is read whole.
const (
	detailArtifactsBudget = 14000
	minArtifactShare      = 2500
	maxArtifactShare      = 9000
)

func artifactShare(n int) int {
	if n == 0 {
		return maxArtifactShare
	}
	return min(maxArtifactShare, max(minArtifactShare, detailArtifactsBudget/n))
}

// maxRepoNodes bounds the repository nodes listed with their responsibility.
const maxRepoNodes = 12

func workflowDetailBatchPrompt(p *core.Project, docs []core.Artifact) string {
	var b strings.Builder
	b.WriteString(`You are the workflow planner of Foundry. Foundry designs software and writes instructions for an EXTERNAL coding agent (Claude Code, Codex, OpenCode, Qwen Code, ...). Foundry never runs them. Do not write application code. Do not modify files.
Write every human-readable text in the same language as the story.

PROJECT: ` + p.Name + "\nSTORY:\n" + p.Story + "\n\n")

	writeBinding(&b, p)
	writeStackBrief(&b, p)
	writeStructureBrief(&b, p, "PROJECT STRUCTURE (source of truth for folders, module boundaries and dependencies)")
	// Paths and owners only: the responsibilities of the nodes the command
	// touches follow in the dynamic part.
	writeRepoBriefText(&b, p, 0)
	b.WriteString(precedenceRules)
	b.WriteString(`DETAIL ROUND: each round writes the details of ONE command of the workflow outline. For it, provide:
- context: background and why this command exists
- instructions: specific Markdown instructions (what to build/change, behavior, edge cases, which artifact sections to follow). Never generic. Understandable without this conversation.
- inputs: what the command needs (from dependencies)
- expected_outputs: what the agent hands back (code, migration, test, document, report)
- acceptance_criteria: checkable conditions. One that proves a criterion listed under REQUIREMENTS starts with that id (REQ-001-AC1). Any other criterion has no id: never number new ones (no REQ-001-AC9 that is not listed)
- constraints: limits that apply (technology, compatibility, performance, security)
- files: files, folders or modules the command touches, only from the project structure; empty when unknown
- feature_ids / sub_feature_ids: keep or complete the links from the outline; use only existing ids
The blueprint artifacts the command implements are cut to the sections about it; the headings left out are listed. Stay within the command's own scope: what another command of the outline builds is that command's job.

` + commandRules)
	if HasUI(p) {
		b.WriteString(VisualVerificationRules)
	}

	b.WriteString(CacheBreak) // the rest changes with every round
	w := p.Workflow
	var batch []core.Command
	if c := nextFillCommand(w.Commands); c != nil {
		batch = []core.Command{*c}
	}
	if w.Request != nil && w.Request.Rejected != "" {
		b.WriteString("YOUR LAST ANSWER WAS REFUSED, fix this: " + w.Request.Rejected + "\n\n")
	}
	writeOutline(&b, w.Commands, batch)

	b.WriteString(fmt.Sprintf("DETAIL PHASE - Fill in details for these %d commands:\n", len(batch)))
	for _, cmd := range batch {
		b.WriteString(fmt.Sprintf("- %s: %s\n", cmd.ID, cmd.Name))
	}
	b.WriteString("\n")
	for _, cmd := range batch {
		writeDetailCommand(&b, cmd)
		writeDependencyOutputs(&b, w.Commands, cmd)
		writeLayoutPages(&b, p, w.Commands, cmd)
	}
	if !writeBatchRequirements(&b, p, batch) {
		b.WriteString("REQUIREMENTS: this command covers none; its acceptance criteria carry no requirement or criterion id.\n\n")
	}
	writeBatchFeatures(&b, p, batch)
	q := commandQuery(p, batch)
	needed := map[string]bool{}
	for _, cmd := range batch {
		for _, a := range cmd.Artifacts {
			needed[a.ID] = true
		}
	}
	// The repository-structure artifact, when read, carries the nodes with
	// their responsibility itself.
	if !needed[core.ArtifactRepoStructure] {
		writeRepoNodes(&b, p, q)
	}
	budget := artifactShare(len(needed))
	b.WriteString("BLUEPRINT ARTIFACTS this command implements (cut to the sections about it):\n")
	for _, d := range docs {
		if !needed[d.ID] {
			continue
		}
		fmt.Fprintf(&b, "=== %s (%s, %s%s) %s\n", d.ID, d.Title, d.Category, map[bool]string{true: ", builtin"}[d.Builtin], d.Summary)
		var skip func(string) bool
		if d.ID == core.ArtifactAcceptance {
			// Its sections per requirement: the covered ones are under
			// REQUIREMENTS above, the rest belong to other commands.
			skip = func(h string) bool { return requirementID.MatchString(h) }
		}
		b.WriteString(strings.TrimRight(scopeArtifactSkip(d.Body, q, budget, skip), "\n") + "\n")
	}
	b.WriteString("\n")

	// The example carries the id this round asks for: an example id ("C1",
	// the outline's) is copied back and the round is refused.
	id, pages := "CMD-001", ""
	if len(batch) > 0 {
		id = batch[0].ID
		if len(layoutPages(p)) > 0 && listsLayout(batch[0]) {
			pages = `, "pages": [""]`
		}
	}
	fmt.Fprintf(&b, "Reply with ONLY one JSON object in a ```json block; \"id\" is exactly %s:\n{\"commands\": [{\"id\": %q, \"context\": \"\", \"instructions\": \"\", \"inputs\": [\"\"], \"expected_outputs\": [\"\"], \"acceptance_criteria\": [\"\"], \"constraints\": [\"\"], \"files\": [\"\"]%s}]}", id, id, pages)
	return b.String()
}

// writeOutline lists every command of the workflow, one line each: its
// name, what it builds on and covers. The round's command, its dependencies
// and its dependents also show their objective.
func writeOutline(b *strings.Builder, cmds, batch []core.Command) {
	near := map[string]bool{}
	for _, c := range batch {
		near[c.ID] = true
		for _, d := range c.DependsOn {
			near[d] = true
		}
	}
	for _, c := range cmds {
		for _, d := range c.DependsOn {
			for _, t := range batch {
				if d == t.ID {
					near[c.ID] = true
				}
			}
		}
	}
	b.WriteString("WORKFLOW OUTLINE (every command; what each builds is its own job):\n")
	for _, c := range cmds {
		fmt.Fprintf(b, "- %s %s", c.ID, oneLine(c.Name))
		if near[c.ID] {
			fmt.Fprintf(b, ": %s", oneLine(c.Objective))
		}
		if len(c.DependsOn) > 0 {
			fmt.Fprintf(b, " | depends_on %s", strings.Join(c.DependsOn, ","))
		}
		if len(c.Covers) > 0 {
			fmt.Fprintf(b, " | covers %s", strings.Join(c.Covers, ","))
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
}

// writeDetailCommand is the outline entry of the command the round writes,
// every field of it.
func writeDetailCommand(b *strings.Builder, c core.Command) {
	var arts []string
	for _, a := range c.Artifacts {
		arts = append(arts, a.ID)
	}
	fmt.Fprintf(b, "COMMAND %s\nname: %s\nobjective: %s\n", c.ID, oneLine(c.Name), oneLine(c.Objective))
	for _, f := range []struct{ name, val string }{
		{"depends_on", strings.Join(c.DependsOn, ", ")},
		{"artifacts", strings.Join(arts, ", ")},
		{"covers", strings.Join(c.Covers, ", ")},
		{"feature_ids", strings.Join(c.FeatureIDs, ", ")},
		{"sub_feature_ids", strings.Join(c.SubFeatureIDs, ", ")},
		{"priority", c.Priority},
		{"complexity", c.Complexity},
	} {
		if f.val != "" {
			fmt.Fprintf(b, "%s: %s\n", f.name, f.val)
		}
	}
	b.WriteString("\n")
}

// writeDependencyOutputs lists what the commands c builds on hand over:
// their outputs and files are c's inputs.
func writeDependencyOutputs(b *strings.Builder, cmds []core.Command, c core.Command) {
	wrote := false
	for _, d := range cmds {
		if !slices.Contains(c.DependsOn, d.ID) || (len(d.Outputs) == 0 && len(d.Files) == 0) {
			continue
		}
		if !wrote {
			b.WriteString("WHAT THE DEPENDENCIES HAND OVER (inputs come from these):\n")
			wrote = true
		}
		fmt.Fprintf(b, "- %s %s\n", d.ID, oneLine(d.Name))
		for _, o := range d.Outputs {
			fmt.Fprintf(b, "  - output: %s\n", oneLine(o))
		}
		if len(d.Files) > 0 {
			fmt.Fprintf(b, "  - files: %s\n", strings.Join(d.Files, ", "))
		}
	}
	if wrote {
		b.WriteString("\n")
	}
}

// writeBatchRequirements lists the requirements the batch covers with every
// acceptance criterion whole: they are what the command must prove, and the
// only criterion ids it may cite. It reports whether it listed any.
func writeBatchRequirements(b *strings.Builder, p *core.Project, batch []core.Command) bool {
	covered := map[string]bool{}
	for _, c := range batch {
		for _, id := range c.Covers {
			covered[id] = true
		}
	}
	wrote := false
	for _, r := range p.Requirements {
		if !covered[r.ID] {
			continue
		}
		if !wrote {
			b.WriteString("REQUIREMENTS this command covers, with their ACCEPTANCE CRITERIA (cite these ids exactly; an id not listed here does not exist):\n")
			wrote = true
		}
		fmt.Fprintf(b, "- %s [%s] %s\n", r.ID, r.Priority, oneLine(r.Text))
		for _, c := range r.Acceptance {
			fmt.Fprintf(b, "  - %s (%s): %s", c.ID, c.Kind, oneLine(c.Text))
			if c.Evidence != "" {
				fmt.Fprintf(b, " [evidence: %s]", oneLine(c.Evidence))
			}
			b.WriteString("\n")
		}
		if r.Untestable != "" {
			fmt.Fprintf(b, "  - untestable: %s\n", oneLine(r.Untestable))
		}
	}
	if wrote {
		b.WriteString("\n")
	}
	return wrote
}

// commandQuery is what the batch is about, for picking artifact sections:
// its name and objective first, then the requirements and features it
// carries.
func commandQuery(p *core.Project, batch []core.Command) scopeQuery {
	var q scopeQuery
	for _, c := range batch {
		q.add(c.Name, 2)
		q.add(c.Objective, 2)
		q.add(strings.Join(c.Covers, " "), 1)
		q.add(strings.Join(c.FeatureIDs, " ")+" "+strings.Join(c.SubFeatureIDs, " "), 1)
		for _, r := range p.Requirements {
			if !slices.Contains(c.Covers, r.ID) {
				continue
			}
			q.add(r.Text, 1)
			for _, ac := range r.Acceptance {
				q.add(ac.Text, 1)
			}
		}
		for _, f := range p.Features {
			if slices.Contains(c.FeatureIDs, f.ID) {
				q.add(f.Name, 1.5)
			}
			for _, s := range f.SubFeatures {
				if slices.Contains(c.SubFeatureIDs, s.ID) {
					q.add(s.Name, 1.5)
				}
			}
		}
	}
	return q
}

// writeBatchFeatures names the features and subfeatures the batch links,
// and those of the requirements it covers, for keeping or completing its
// feature_ids and sub_feature_ids.
func writeBatchFeatures(b *strings.Builder, p *core.Project, batch []core.Command) {
	wrote := false
	for _, f := range p.Features {
		linked := false
		for _, c := range batch {
			if slices.Contains(c.FeatureIDs, f.ID) {
				linked = true
			}
			for _, r := range f.RequirementIDs {
				if slices.Contains(c.Covers, r) {
					linked = true
				}
			}
		}
		if !linked {
			continue
		}
		if !wrote {
			b.WriteString("FEATURES this command links or whose requirements it covers:\n")
			wrote = true
		}
		fmt.Fprintf(b, "- %s %s", f.ID, f.Name)
		if len(f.RequirementIDs) > 0 {
			fmt.Fprintf(b, " (requirements %s)", strings.Join(f.RequirementIDs, ", "))
		}
		b.WriteString("\n")
		for _, s := range f.SubFeatures {
			fmt.Fprintf(b, "  - %s %s\n", s.ID, s.Name)
		}
	}
	if wrote {
		b.WriteString("\n")
	}
}

// writeRepoNodes lists, with their responsibility, the repository nodes the
// command most likely touches; the static part names every node by path.
func writeRepoNodes(b *strings.Builder, p *core.Project, q scopeQuery) {
	s := p.Structure
	if s == nil || s.Mode == core.StructureExisting || s.Repo == nil || len(s.Repo.Nodes) == 0 {
		return
	}
	nodes := s.Repo.Nodes
	secs := make([]mdSection, len(nodes))
	for i, n := range nodes {
		secs[i] = mdSection{text: n.Path + " " + n.Owner + " " + n.Part + " " + n.Responsibility, parent: -1}
	}
	scores := rank(secs, q)
	order := make([]int, 0, len(nodes))
	for i := range nodes {
		if scores[i] > 0 {
			order = append(order, i)
		}
	}
	slices.SortStableFunc(order, func(a, b int) int {
		switch {
		case scores[a] > scores[b]:
			return -1
		case scores[a] < scores[b]:
			return 1
		}
		return a - b
	})
	if len(order) > maxRepoNodes {
		order = order[:maxRepoNodes]
	}
	if len(order) == 0 {
		return
	}
	slices.Sort(order)
	b.WriteString("REPOSITORY NODES this command most likely touches (path | owner | responsibility):\n")
	for _, i := range order {
		n := nodes[i]
		fmt.Fprintf(b, "- %s | %s | %s\n", n.Path, n.Owner, oneLine(n.Responsibility))
	}
	b.WriteString("\n")
}
