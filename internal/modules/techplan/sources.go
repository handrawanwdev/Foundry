package techplan

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/repoplan"
)

// Source areas the plan is written from. The plan records a hash of each;
// when one changes, only the plan sections that depend on it are marked for
// review. The technology decision itself is not a source (it is the plan),
// and neither is anything derived from it (the blueprint's database engine,
// the structure type), so the plan never goes stale from its own result.
var SourceAreaOrder = []string{"requirements", "modules", "platform", "data", "integrations", "security", "folder"}

// Plan sections are the workspace tabs.
var Sections = []string{
	"context", "overview", "stack", "data", "infra", "architecture", "standards", "compatibility", "alternatives", "reasoning", "evaluation", "history",
}

var sectionsOf = map[string][]string{
	"requirements": {"context", "stack", "reasoning"},
	"modules":      {"context", "architecture", "reasoning"},
	"platform":     {"context", "stack", "architecture", "infra"},
	"data":         {"context", "data", "compatibility"},
	"integrations": {"context", "infra", "data"},
	"security":     {"context", "infra"},
	"folder":       {"architecture", "compatibility"},
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
	disc := p.Discovery
	if disc == nil {
		disc = &core.Discovery{}
	}
	var mods []string
	for _, m := range repoplan.Modules(p) {
		mods = append(mods, m.ID)
	}
	var folder any
	if s := p.Structure; s != nil && s.Mode != core.StructureExisting && s.Repo != nil {
		folder = []any{s.Repo.Approach, s.Repo.Topology, FileLanguages(s.Repo)}
	}
	return map[string]string{
		"requirements": digest([]any{p.Story, p.Requirements, p.Features, p.Flows, p.Survey, disc.TechNeeds}),
		"modules":      digest(mods),
		"platform":     digest(p.Platform),
		"data":         digest(disc.Data),
		"integrations": digest(disc.Integrations),
		"security":     digest([]any{disc.Security, disc.Roles}),
		"folder":       digest(folder),
	}
}

// StaleAreas are the source areas that changed since the plan was written,
// in a fixed order.
func StaleAreas(p *core.Project, plan *core.TechPlan) []string {
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
// evaluation is always among them.
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

// MarkCurrent records that the plan was written, or fully reviewed, from the
// project as it is now.
func MarkCurrent(p *core.Project, plan *core.TechPlan) {
	plan.Sources = SourceAreas(p)
	plan.GeneratedAt = time.Now().UTC()
}

// fileLanguage maps source file extensions to the language they belong to.
// Configuration, data and documentation extensions are not listed on
// purpose: they say nothing about the stack.
var fileLanguage = map[string]string{
	".ts": "typescript", ".tsx": "typescript", ".js": "typescript", ".jsx": "typescript", ".mjs": "typescript",
	".go": "go", ".py": "python", ".rs": "rust", ".java": "java", ".kt": "kotlin", ".cs": "c#", ".php": "php",
	".rb": "ruby", ".dart": "dart", ".swift": "swift", ".ex": "elixir", ".exs": "elixir",
}

// FileLanguages are the languages the repository plan's files are written
// in, from their extensions.
func FileLanguages(repo *core.RepoPlan) []string {
	var out []string
	if repo == nil {
		return nil
	}
	for _, n := range repo.Nodes {
		if n.IsDir() {
			continue
		}
		if l := fileLanguage[strings.ToLower(path.Ext(n.Path))]; l != "" && !slices.Contains(out, l) {
			out = append(out, l)
		}
	}
	slices.Sort(out)
	return out
}

// CanonLanguage is the language name two technologies are compared by:
// JavaScript and TypeScript are one family, aliases collapse.
func CanonLanguage(l string) string {
	l = strings.ToLower(strings.TrimSpace(l))
	switch l {
	case "javascript", "js", "ts", "node", "node.js", "nodejs":
		return "typescript"
	case "golang":
		return "go"
	case "csharp", "c sharp", ".net", "dotnet":
		return "c#"
	}
	return l
}
