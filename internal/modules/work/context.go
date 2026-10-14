package work

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/handrawanw/foundry/internal/core"
	"github.com/handrawanw/foundry/internal/modules/standard"
)

// briefScope narrows the architecture blueprint to what one task touches.
// A nil scope keeps everything.
type briefScope struct {
	modules map[string]bool
	names   map[string]bool // entity names pulled in through relations
	noFront bool            // backend-only task: frontend rules/libs do not apply
	noBack  bool            // ui-only task: backend rules/shape do not apply
	text    string          // lower-cased task text, to match entities and workers by name
}

func (s *briefScope) frontend() bool { return s == nil || !s.noFront }
func (s *briefScope) backend() bool  { return s == nil || !s.noBack }

func (s *briefScope) module(id string) bool { return s == nil || s.modules[id] }

func (s *briefScope) entity(e core.Entity) bool {
	return s == nil || (e.Module != "" && s.modules[e.Module]) || s.names[strings.ToLower(e.Name)] || s.mentions(e.Name)
}

func (s *briefScope) worker(w core.Worker) bool { return s == nil || s.mentions(w.Name) }

func (s *briefScope) mentions(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	return name != "" && strings.Contains(s.text, name)
}

// taskScope finds the blueprint modules a task works in: modules named by its
// text, files, or feature, and by the tasks it depends on (their contracts are
// not always named in its own text), plus every module they depend on,
// transitively (indirect contracts). Entities related to an included entity
// follow, so foreign keys stay resolvable. It returns nil — the full blueprint — when the task is cross-cutting
// (scaffold/integration) or no module matches, so required context is never
// dropped on a guess.
func taskScope(p *core.Project, t *core.Task) *briefScope {
	b := p.Blueprint
	if b == nil || len(b.Modules) == 0 || t.Area == "scaffold" || t.Area == "integration" {
		return nil
	}
	var text strings.Builder
	text.WriteString(t.Title + " " + t.Description + " " + strings.Join(t.Files, " "))
	if f := p.Feature(t.FeatureID); f != nil {
		text.WriteString(" " + f.Name + " " + f.Description)
	}
	for _, id := range t.DependsOn {
		if d := p.Task(id); d != nil {
			text.WriteString(" " + d.Title + " " + strings.Join(d.Files, " "))
		}
	}
	sc := &briefScope{modules: map[string]bool{}, names: map[string]bool{}, text: strings.ToLower(text.String())}
	byID := map[string]core.Module{}
	for _, m := range b.Modules {
		byID[m.ID] = m
		if sc.mentions(m.ID) {
			sc.modules[m.ID] = true
		}
	}
	if len(sc.modules) == 0 {
		return nil
	}
	switch hasPage := len(t.PageIDs) > 0; {
	case t.Area == "ui" || hasPage:
		sc.noBack = true
	case t.Area == "api" || t.Area == "data":
		sc.noFront = true
	}
	queue := make([]string, 0, len(sc.modules))
	for id := range sc.modules {
		queue = append(queue, id)
	}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		for _, d := range byID[id].DependsOn {
			if !sc.modules[d] {
				sc.modules[d] = true
				queue = append(queue, d)
			}
		}
	}
	for _, e := range b.Database.Entities {
		if sc.modules[e.Module] || sc.mentions(e.Name) {
			for _, r := range e.Relates {
				sc.names[strings.ToLower(r)] = true
			}
		}
	}
	return sc
}

// TaskArchitectureBrief is ArchitectureBrief limited to the task's modules,
// their entities, and workers it mentions. Global rules (stack, auth, API,
// events, coding rules) are always kept.
func TaskArchitectureBrief(p *core.Project, t *core.Task) string {
	return architectureBrief(p, taskScope(p, t))
}

// ruleScope picks the conditional FOUNDRY rules a task needs. UI-only work
// skips the backend rules (Redis, latency, event/outbox details), api/data
// work skips the frontend rule, and everything else — scaffold, integration,
// test, unknown areas — gets the full contract. Global rules (architecture,
// code quality, ORM, event contract, structure, stop-and-ask) are never
// conditional.
func ruleScope(t *core.Task) standard.Scope {
	backendArea := t.Area == "api" || t.Area == "data"
	s := standard.Scope{
		Frontend: t.Area == "ui" || len(t.PageIDs) > 0 || !backendArea,
		Backend:  t.Area != "ui",
	}
	text := strings.ToLower(t.Title + " " + t.Description + " " + strings.Join(t.AcceptanceCriteria, " "))
	s.Webhooks = s.Backend && (!backendArea || strings.Contains(text, "webhook"))
	return s
}

// rulesVersion identifies every rule text a prompt can carry. A change to any
// of them changes the version, which invalidates task memory built under the
// old rules.
func rulesVersion() string {
	sum := sha256.Sum256([]byte(standard.Version() + workRules + uiRules))
	return hex.EncodeToString(sum[:6])
}
