package work

import (
	"github.com/handrawanw/foundry/internal/core"
	"github.com/handrawanw/foundry/internal/modules/standard"
)

// PromptSize compares the first-run task prompt before and after context
// scoping. Both are exact character counts of text Foundry sends; they are
// not provider tokens.
type PromptSize struct {
	TaskID   string
	Baseline int // blueprint in full, no dependency memory (the original prompt)
	Current  int
	// Memory is the dependency-memory block inside Current, and MemorySource
	// the source bytes its summaries stand for (0 when no memory is used).
	Memory       int
	MemorySource int
}

// MeasurePrompts sizes the fresh-run prompt of every task. Baseline is rebuilt
// from the current prompt by swapping the scoped brief and rules for the full
// blueprint and full rules and dropping the dependency memory block, so both come from the same
// project state.
func MeasurePrompts(p *core.Project) []PromptSize {
	full := len(ArchitectureBrief(p))
	out := make([]PromptSize, 0, len(p.Tasks))
	for i := range p.Tasks {
		t := &p.Tasks[i]
		cur := len(TaskPrompt(p, t))
		mem, src := dependencyMemory(p, t)
		base := cur - len(mem) - len(TaskArchitectureBrief(p, t)) + full - len(standard.RulesFor(ruleScope(t))) + len(standard.Rules)
		out = append(out, PromptSize{TaskID: t.ID, Baseline: base, Current: cur, Memory: len(mem), MemorySource: src})
	}
	return out
}
