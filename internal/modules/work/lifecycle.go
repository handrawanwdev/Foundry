package work

import (
	"slices"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
)

// The document lifecycles are written with the use cases: the AI returns
// the lifecycles it adds or changes, each in full, and the ones it removes.

// normLifecycle trims a lifecycle and points its transitions at the use
// cases: an id the answer used for a use case that got another id (a new
// or renumbered one) is mapped, and a use case named instead of its id is
// found by name.
func normLifecycle(lc core.Lifecycle, flows []core.Flow, ids map[string]string) core.Lifecycle {
	lc.Entity = strings.TrimSpace(lc.Entity)
	for i := range lc.States {
		s := &lc.States[i]
		s.ID, s.Label, s.Kind = stateID(s.ID), strings.TrimSpace(s.Label), strings.ToLower(strings.TrimSpace(s.Kind))
	}
	for i := range lc.Transitions {
		t := &lc.Transitions[i]
		t.From = slices.DeleteFunc(mapSlice(t.From, stateID), func(s string) bool { return s == "" })
		t.To, t.Kind = stateID(t.To), strings.ToLower(strings.TrimSpace(t.Kind))
		t.Action, t.Actor, t.Guard = strings.TrimSpace(t.Action), strings.TrimSpace(t.Actor), strings.TrimSpace(t.Guard)
		t.Flow = flowRefID(strings.TrimSpace(t.Flow), flows, ids)
	}
	return lc
}

// stateID is a status id as written in the lifecycle: lowercase with
// underscores ("Pending Approval" → "pending_approval").
func stateID(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.ReplaceAll(strings.TrimSpace(s), "-", " "))), "_")
}

// flowRefID resolves a use case reference of an answer to its id.
func flowRefID(ref string, flows []core.Flow, ids map[string]string) string {
	if ref == "" {
		return ""
	}
	if id, ok := ids[strings.ToUpper(ref)]; ok {
		return id
	}
	for _, f := range flows {
		if strings.EqualFold(f.ID, ref) {
			return f.ID
		}
	}
	for _, f := range flows {
		if strings.EqualFold(strings.TrimSpace(f.Name), ref) {
			return f.ID
		}
	}
	return strings.ToUpper(ref)
}

func mapSlice(in []string, f func(string) string) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = f(s)
	}
	return out
}

// mergeLifecycles replaces the lifecycles the answer returned (by entity),
// adds the new ones and drops the removed ones. It returns the entities
// touched.
func mergeLifecycles(list, in []core.Lifecycle, remove []string, flows []core.Flow, ids map[string]string) ([]core.Lifecycle, map[string]bool, error) {
	out := slices.Clone(list)
	touched := map[string]bool{}
	for _, e := range remove {
		i := slices.IndexFunc(out, func(lc core.Lifecycle) bool { return strings.EqualFold(lc.Entity, strings.TrimSpace(e)) })
		if i < 0 {
			return nil, nil, &core.Invalid{Subject: "siklus status", Issues: []core.Issue{{Level: core.IssueError, Where: "lifecycle:" + e, Message: "siklus status ini tidak ada"}}}
		}
		out = slices.Delete(out, i, i+1)
	}
	for _, lc := range in {
		lc = normLifecycle(lc, flows, ids)
		if i := slices.IndexFunc(out, func(x core.Lifecycle) bool { return strings.EqualFold(x.Entity, lc.Entity) }); i >= 0 && lc.Entity != "" {
			out[i] = lc
		} else {
			out = append(out, lc)
		}
		touched["lifecycle:"+lc.Entity] = true
	}
	return out, touched, nil
}
