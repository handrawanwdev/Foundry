package work

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
)

// The workflow outline must cover every MUST requirement. A large blueprint
// reaches the outline by artifact summaries only, so the requirement
// registry is written into the prompt on its own: the planner cannot cover a
// requirement it never reads, and a refusal that names only ids makes it
// guess what they mean.

// uncoveredMust starts the refusal of a workflow that leaves a MUST
// requirement without a command; the ids follow it.
const uncoveredMust = "requirement MUST belum dicakup command mana pun (covers): "

var requirementID = regexp.MustCompile(`REQ-\d+`)

// maxCriterionText cuts a criterion in the registry; the id still names it.
const maxCriterionText = 220

// writeRequirementRegistry lists every requirement: a MUST one with its
// acceptance criteria (what implementing it means), the others by text.
func writeRequirementRegistry(b *strings.Builder, p *core.Project) {
	if p == nil || len(p.Requirements) == 0 {
		return
	}
	b.WriteString("REQUIREMENTS (registry; covers takes these ids; every MUST is covered by a command that really implements it):\n")
	for _, r := range p.Requirements {
		fmt.Fprintf(b, "- %s [%s] %s\n", r.ID, r.Priority, oneLine(r.Text))
		if r.Priority != "MUST" {
			continue
		}
		for _, c := range r.Acceptance {
			fmt.Fprintf(b, "  - %s (%s) %s\n", c.ID, c.Kind, cutRunes(oneLine(c.Text), maxCriterionText))
		}
		if r.Untestable != "" {
			fmt.Fprintf(b, "  - untestable: %s\n", cutRunes(oneLine(r.Untestable), maxCriterionText))
		}
	}
	b.WriteString("\n")
}

// uncoveredRequirements are the requirements a refusal names as MUST without
// a command, in registry order; nil for any other refusal.
func uncoveredRequirements(p *core.Project, rejected string) []core.Requirement {
	i := strings.Index(rejected, uncoveredMust)
	if p == nil || i < 0 {
		return nil
	}
	named := map[string]bool{}
	for _, id := range requirementID.FindAllString(rejected[i+len(uncoveredMust):], -1) {
		named[id] = true
	}
	var out []core.Requirement
	for _, r := range p.Requirements {
		if named[r.ID] {
			out = append(out, r)
		}
	}
	return out
}

// writeCoverageRepair spells out each MUST requirement the last answer left
// uncovered, whole, with the features that carry it, and how to cover it
// without a mapping that only names it.
func writeCoverageRepair(b *strings.Builder, p *core.Project, missing []core.Requirement) {
	if len(missing) == 0 {
		return
	}
	b.WriteString("MUST REQUIREMENTS YOUR LAST ANSWER LEFT UNCOVERED (read each one whole; do not guess from the id):\n")
	for _, r := range missing {
		fmt.Fprintf(b, "- %s [MUST] %s\n", r.ID, oneLine(r.Text))
		var feats []string
		for _, f := range p.Features {
			for _, id := range f.RequirementIDs {
				if id == r.ID {
					feats = append(feats, f.ID+" "+f.Name)
					break
				}
			}
		}
		if len(feats) > 0 {
			fmt.Fprintf(b, "  features: %s\n", strings.Join(feats, "; "))
		}
		for _, c := range r.Acceptance {
			fmt.Fprintf(b, "  - %s (%s) %s", c.ID, c.Kind, oneLine(c.Text))
			if c.Evidence != "" {
				fmt.Fprintf(b, " [evidence: %s]", oneLine(c.Evidence))
			}
			b.WriteString("\n")
		}
	}
	b.WriteString(`For each one: when a command of your outline really builds what its acceptance criteria check, add the id to that command's covers. Otherwise add a command whose objective implements it (it depends_on the commands it builds on, lists the artifacts that rule it: threat-model, operations, data-lifecycle, integrations when present). Never add an id to a command that only mentions it. A requirement that spans several parts may be covered by several commands, each implementing its share. Keep every other command and its coverage.

`)
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }
