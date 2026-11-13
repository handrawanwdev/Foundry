package repoplan

import (
	"fmt"
	"slices"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
)

// journal is the execution state of one revision. The AI writes all its ops
// against the plan it was shown, but each op runs on the plan the ops before
// it left behind — including what Foundry did on its own (Normalize drops an
// implied folder that lost its last child). The journal remembers where every
// path went so a later op is checked against that state, not the snapshot:
// a path that moved is followed to its new place, a removal of a path that is
// already gone is a recorded no-op, and a path that never existed stops the
// revision with what does exist nearby.
type journal struct {
	// moved maps a path to where an op moved it.
	moved map[string]string
	// gone maps a path to the 1-based op that made it disappear.
	gone map[string]int
	// pending are the moves of the op that is running; settle confirms them.
	pending map[string]string
}

func newJournal() *journal {
	return &journal{moved: map[string]string{}, gone: map[string]int{}, pending: map[string]string{}}
}

func pathSet(plan *core.RepoPlan) map[string]bool {
	out := make(map[string]bool, len(plan.Nodes))
	for _, n := range plan.Nodes {
		out[n.Path] = true
	}
	return out
}

// settle records what op step changed: every path that disappeared either
// moved (the op said where) or is gone. A path that exists again (set_node
// created it anew) is a live node, not a forwarding entry.
func (j *journal) settle(before map[string]bool, plan *core.RepoPlan, step int) {
	after := pathSet(plan)
	for p := range before {
		if after[p] {
			continue
		}
		if to, ok := j.pending[p]; ok {
			j.moved[p] = to
			delete(j.gone, p)
			continue
		}
		j.gone[p] = step
		delete(j.moved, p)
	}
	for p := range after {
		delete(j.gone, p)
		delete(j.moved, p)
	}
	clear(j.pending)
}

// target is what an op's path means in the current state of the revision.
type target struct {
	path string // the node's own spelling now, or the last spelling it had
	from string // the path the op named, when it was followed through a move
	gone int    // the op that made path disappear; 0 when it exists
}

// resolve finds the node raw names: as written, else through the moves of
// earlier ops. A path that is gone resolves with gone set. A path that never
// existed in this revision is an error naming the closest existing folder and
// what it holds, so the retry can pick a real path.
func (j *journal) resolve(plan *core.RepoPlan, raw string) (target, error) {
	if p, err := findPath(plan, raw); err == nil {
		return target{path: p}, nil
	}
	have := pathSet(plan)
	for _, dir := range []bool{true, false} {
		start := CleanPath(raw, dir)
		if start == "" {
			continue
		}
		p, seen := start, map[string]bool{}
		for !seen[p] {
			seen[p] = true
			if have[p] {
				return target{path: p, from: start}, nil
			}
			if to, ok := j.moved[p]; ok {
				p = to
				continue
			}
			if k, ok := j.gone[p]; ok {
				t := target{path: p, gone: k}
				if p != start {
					t.from = start
				}
				return t, nil
			}
			break
		}
	}
	return target{}, missingPath(plan, raw)
}

// follow rewrites a reference (a module location, an interface, a dependency
// endpoint) that names a path an earlier op moved, keeping it as written
// otherwise: a module id, a path that exists, a path never seen. The
// validation after the ops judges what is left.
func (j *journal) follow(have map[string]bool, ref string) string {
	if len(j.moved) == 0 || !strings.ContainsAny(ref, "/\\.") {
		return ref
	}
	for _, dir := range []bool{true, false} {
		p := CleanPath(ref, dir)
		if p == "" || have[p] {
			return ref
		}
		for seen := map[string]bool{}; !seen[p]; {
			seen[p] = true
			to, ok := j.moved[p]
			if !ok {
				break
			}
			if have[to] {
				return to
			}
			p = to
		}
	}
	return ref
}

// followRule applies follow to both ends of a dependency rule.
func (j *journal) followRule(have map[string]bool, r *core.RepoDepRule) {
	r.From, r.To = j.follow(have, r.From), j.follow(have, r.To)
}

// missingPath is the refusal for a path that is not in the plan: it names
// the deepest folder of that path that does exist and lists its content.
func missingPath(plan *core.RepoPlan, raw string) error {
	p := CleanPath(raw, false)
	if p == "" {
		return fmt.Errorf("path %q tidak valid", raw)
	}
	have := pathSet(plan)
	for parent := parentPath(p); parent != ""; parent = parentPath(parent) {
		if !have[parent] {
			continue
		}
		var kids []string
		for _, n := range plan.Nodes {
			if n.Parent() == parent {
				kids = append(kids, strings.TrimPrefix(n.Path, parent))
			}
		}
		slices.Sort(kids)
		const show = 12
		more := ""
		if len(kids) > show {
			more = fmt.Sprintf(", …(+%d)", len(kids)-show)
			kids = kids[:show]
		}
		if len(kids) == 0 {
			return fmt.Errorf("path %q tidak ada di struktur; folder terdekat yang ada %s kosong", raw, parent)
		}
		return fmt.Errorf("path %q tidak ada di struktur; folder terdekat yang ada %s berisi: %s%s", raw, parent, strings.Join(kids, ", "), more)
	}
	return fmt.Errorf("path %q tidak ada di struktur dan tidak ada folder induknya", raw)
}

// followed is the note a change line carries when its path was followed
// through an earlier move.
func (t target) followed() string {
	if t.from == "" || t.from == t.path {
		return ""
	}
	return fmt.Sprintf(" (ditulis sebagai %s; sudah dipindah oleh perubahan sebelumnya)", t.from)
}
