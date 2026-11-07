package repoplan

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"time"

	"github.com/handrawanwdev/foundry/internal/core"
)

// Source areas the repository plan is written from. A plan records a hash
// of each; when an area changes, only the plan sections that depend on it
// are marked for review.
var SourceAreaOrder = []string{
	"requirements", "modules", "architecture", "technology", "data", "integrations", "security", "operations", "structure",
}

// sectionsOf are the plan sections an area feeds. Section ids are the
// workspace tabs.
var sectionsOf = map[string][]string{
	"requirements": {"modules", "tree"},
	"modules":      {"modules", "tree", "dependency"},
	"architecture": {"tree", "dependency", "rules", "decisions"},
	"technology":   {"tree", "rules"},
	"data":         {"tree", "modules"},
	"integrations": {"tree", "dependency"},
	"security":     {"tree", "dependency", "rules"},
	"operations":   {"tree"},
	"structure":    {"tree", "modules", "dependency"},
}

func digest(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:6])
}

// SourceAreas hashes every source area of the project.
func SourceAreas(p *core.Project) map[string]string {
	bp := p.Blueprint
	if bp == nil {
		bp = &core.Blueprint{}
	}
	disc := p.Discovery
	if disc == nil {
		disc = &core.Discovery{}
	}
	type tech struct{ Area, Choice, Lang string }
	var stack []tech
	if p.Stack != nil {
		for _, d := range p.Stack.Decisions {
			stack = append(stack, tech{d.Area, d.Choice(), d.Lang()})
		}
		// the plan's other decisions (ORM, auth, CI/CD, ...) shape the code too
		for _, it := range p.Stack.Plan.Extras() {
			stack = append(stack, tech{it.Area, it.Technology, it.Language})
		}
	}
	var ir *core.StructureIR
	if p.Structure != nil {
		ir = p.Structure.IR
	}
	return map[string]string{
		"requirements": digest([]any{p.Story, p.Requirements, p.Features, p.Flows, p.Platform.Content()}),
		"modules":      digest(bp.Modules),
		"architecture": digest([]any{bp.Architecture, bp.Frontend, bp.Backend, bp.Coding, bp.Overview.CoreDomain}),
		"technology":   digest(stack),
		"data":         digest([]any{bp.Database, bp.Cache, bp.Storage, disc.Data}),
		"integrations": digest([]any{disc.Integrations, bp.Events, bp.Realtime, bp.Workers}),
		"security":     digest([]any{disc.Security, bp.Auth, bp.Authz}),
		"operations":   digest([]any{bp.Platform, bp.Performance}),
		"structure":    digest(ir),
	}
}

// StaleAreas are the source areas that changed since the plan was written,
// in a fixed order.
func StaleAreas(p *core.Project, plan *core.RepoPlan) []string {
	if len(plan.Sources) == 0 {
		return nil
	}
	now := SourceAreas(p)
	var out []string
	for _, a := range SourceAreaOrder {
		if was, ok := plan.Sources[a]; ok && was != now[a] {
			out = append(out, a)
		}
	}
	return out
}

// Affected are the plan sections a set of changed areas touches; the
// quality evaluation is always among them.
func Affected(areas []string) []string {
	if len(areas) == 0 {
		return nil
	}
	var out []string
	for _, a := range areas {
		for _, s := range sectionsOf[a] {
			if !slices.Contains(out, s) {
				out = append(out, s)
			}
		}
	}
	return append(out, "evaluation")
}

// MarkCurrent records that the plan was written, or fully reviewed, from
// the project as it is now.
func MarkCurrent(p *core.Project, plan *core.RepoPlan) {
	plan.Sources = SourceAreas(p)
	plan.GeneratedAt = time.Now().UTC()
}
