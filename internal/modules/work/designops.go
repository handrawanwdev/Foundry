package work

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strings"

	"github.com/handrawanw/foundry/internal/core"
)

// A revision is a patch: small operations on existing pages. Foundry applies
// them, so everything the patch does not name stays exactly as it is, and
// then checks the result against the scope the user asked for.

type pagePatch struct {
	Page       string    `json:"page"`
	Operations []patchOp `json:"operations"`
}

// patchOp positions (target, after) are component indexes of the page before
// the patch; earlier operations never shift them.
type patchOp struct {
	Op        string              `json:"op"` // add | remove | move | update
	Target    *int                `json:"target,omitempty"`
	After     *int                `json:"after,omitempty"` // -1 = top; omitted = bottom
	Component *core.PageComponent `json:"component,omitempty"`
	// PageFields: update without target changes these page fields.
	PageFields map[string]any `json:"page_fields,omitempty"`
}

// pageFields the patch may set; ids, feature, platform and components are
// changed by other means.
var pageFields = map[string]bool{
	"name": true, "role": true, "purpose": true, "kind": true, "density": true, "states": true, "route": true,
	"access": true, "primary_action": true, "data": true, "sub_feature_ids": true,
	"hierarchy": true, "secondary_actions": true, "responsive": true, "layout": true,
}

// PatchRejected is a patch that went outside the requested scope. Nothing of
// it is applied.
type PatchRejected struct {
	Requested  []string
	Unexpected []string
}

func (e *PatchRejected) Error() string {
	var b strings.Builder
	b.WriteString("PATCH DITOLAK\n\nDiminta:\n")
	for _, r := range e.Requested {
		b.WriteString("- " + r + "\n")
	}
	if len(e.Requested) == 0 {
		b.WriteString("- (tidak ada operasi yang sah)\n")
	}
	b.WriteString("\nTidak diminta:\n")
	for _, u := range e.Unexpected {
		b.WriteString("- " + u + "\n")
	}
	return strings.TrimSpace(b.String())
}

func opText(page string, o patchOp) string {
	pos := func(p *int) string {
		switch {
		case p == nil:
			return "paling bawah"
		case *p == -1:
			return "paling atas"
		}
		return fmt.Sprintf("setelah [%d]", *p)
	}
	idx := "?"
	if o.Target != nil {
		idx = fmt.Sprint(*o.Target)
	}
	switch o.Op {
	case "add":
		name := "komponen"
		if o.Component != nil {
			name = componentName(*o.Component)
		}
		return fmt.Sprintf("%s ADD %s → %s", page, name, pos(o.After))
	case "remove":
		return fmt.Sprintf("%s [%s] REMOVE", page, idx)
	case "move":
		return fmt.Sprintf("%s [%s] MOVE → %s", page, idx, pos(o.After))
	case "update":
		if o.Target == nil {
			keys := make([]string, 0, len(o.PageFields))
			for k := range o.PageFields {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			return fmt.Sprintf("%s UPDATE halaman: %s", page, strings.Join(keys, ", "))
		}
		return fmt.Sprintf("%s [%s] UPDATE", page, idx)
	}
	return fmt.Sprintf("%s %s", page, o.Op)
}

type opEntry struct {
	orig int // position before the patch; -1 for added components
	c    core.PageComponent
}

// applyOps edits one page. origins[i] is where result component i was
// before the patch (-1 when added). The layout tree follows the operations:
// an added or moved component goes right after "after" in that component's
// layout node, a removed one leaves the tree.
func applyOps(pg core.Page, ops []patchOp) (core.Page, []int, error) {
	pg.Layout = pg.Layout.Clone()
	list := make([]opEntry, len(pg.Components))
	for i, c := range pg.Components {
		list[i] = opEntry{i, c}
	}
	find := func(orig int) int {
		return slices.IndexFunc(list, func(e opEntry) bool { return e.orig == orig })
	}
	fail := func(o patchOp, why string) error {
		return fmt.Errorf("operasi %q di %s: %s", o.Op, pg.ID, why)
	}
	insertAt := func(o patchOp) (int, error) {
		switch {
		case o.After == nil:
			return len(list), nil
		case *o.After == -1:
			return 0, nil
		}
		k := find(*o.After)
		if k < 0 {
			return 0, fail(o, fmt.Sprintf("komponen [%d] tidak ada", *o.After))
		}
		return k + 1, nil
	}
	// place puts component id in the tree where the operation says.
	place := func(id string, o patchOp) {
		if pg.Layout == nil {
			return
		}
		anchor := ""
		if o.After != nil && *o.After >= 0 {
			if k := find(*o.After); k >= 0 {
				anchor = list[k].c.ID
			}
		}
		pg.Layout.Place(id, anchor, o.After != nil && *o.After == -1)
	}
	detach := func(c core.PageComponent) {
		if pg.Layout != nil && c.ID != "" {
			pg.Layout.Detach(c.ID)
		}
	}
	target := func(o patchOp) (int, error) {
		if o.Target == nil {
			return 0, fail(o, "target kosong")
		}
		k := find(*o.Target)
		if k < 0 {
			return 0, fail(o, fmt.Sprintf("komponen [%d] tidak ada", *o.Target))
		}
		return k, nil
	}
	for _, o := range ops {
		switch o.Op {
		case "remove":
			k, err := target(o)
			if err != nil {
				return pg, nil, err
			}
			detach(list[k].c)
			list = slices.Delete(list, k, k+1)
		case "update":
			if o.Target == nil {
				np, err := setPageFields(pg, o.PageFields)
				if err != nil {
					return pg, nil, fail(o, err.Error())
				}
				pg = np
				continue
			}
			k, err := target(o)
			if err != nil {
				return pg, nil, err
			}
			if o.Component == nil {
				return pg, nil, fail(o, "component kosong")
			}
			old, c := list[k].c, *o.Component
			c.ID = old.ID // ids are stable; the layout refers to them
			switch {
			case old.Type != "nav" && c.Type == "nav":
				detach(old)
			case old.Type == "nav" && c.Type != "nav" && pg.Layout != nil && c.ID != "":
				pg.Layout.Place(c.ID, "", false)
			}
			list[k].c = c
		case "move":
			k, err := target(o)
			if err != nil {
				return pg, nil, err
			}
			e := list[k]
			list = slices.Delete(list, k, k+1)
			at, err := insertAt(o)
			if err != nil {
				return pg, nil, err
			}
			list = slices.Insert(list, at, e)
			if e.c.Type != "nav" && e.c.ID != "" {
				detach(e.c)
				place(e.c.ID, o)
			}
		case "add":
			if o.Component == nil {
				return pg, nil, fail(o, "component kosong")
			}
			at, err := insertAt(o)
			if err != nil {
				return pg, nil, err
			}
			c := *o.Component
			c.ID = newID(list, c)
			if c.Type != "nav" {
				place(c.ID, o)
			}
			list = slices.Insert(list, at, opEntry{-1, c})
		default:
			return pg, nil, fail(o, "operasi tidak dikenal (add, remove, move, update)")
		}
	}
	pg.Components = make([]core.PageComponent, len(list))
	origins := make([]int, len(list))
	for i, e := range list {
		pg.Components[i], origins[i] = e.c, e.orig
	}
	return pg, origins, nil
}

// newID keeps the id an added component brings when it is free, else gives
// it the next free "<type>-<n>".
func newID(list []opEntry, c core.PageComponent) string {
	taken := func(id string) bool {
		return slices.ContainsFunc(list, func(e opEntry) bool { return e.c.ID == id })
	}
	id := strings.TrimSpace(c.ID)
	for n := len(list) + 1; id == "" || taken(id); n++ {
		id = fmt.Sprintf("%s-%d", strings.ToLower(c.Type), n)
	}
	return id
}

// setPageFields replaces the given top-level page fields; the page is
// rebuilt from JSON so nothing is shared with the previous version.
func setPageFields(pg core.Page, set map[string]any) (core.Page, error) {
	if len(set) == 0 {
		return pg, fmt.Errorf("page_fields kosong")
	}
	m := toMap(pg)
	for k, v := range set {
		if !pageFields[k] {
			return pg, fmt.Errorf("field %q tidak boleh diubah", k)
		}
		m[k] = v
	}
	raw, _ := json.Marshal(m)
	var out core.Page
	if err := json.Unmarshal(raw, &out); err != nil {
		return pg, err
	}
	return out, nil
}

func toMap(v any) map[string]any {
	raw, _ := json.Marshal(v)
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	return m
}

// changedKeys lists the JSON fields that differ; null and empty are equal.
func changedKeys(a, b any, skip ...string) []string {
	ma, mb := toMap(a), toMap(b)
	empty := func(v any) bool {
		switch x := v.(type) {
		case nil:
			return true
		case []any:
			return len(x) == 0
		case string:
			return x == ""
		}
		return false
	}
	var out []string
	for _, k := range sortedKeys(ma, mb) {
		if slices.Contains(skip, k) {
			continue
		}
		va, vb := ma[k], mb[k]
		if empty(va) && empty(vb) || reflect.DeepEqual(va, vb) {
			continue
		}
		out = append(out, k)
	}
	return out
}

func sortedKeys(ms ...map[string]any) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range ms {
		for k := range m {
			if !seen[k] {
				seen[k] = true
				out = append(out, k)
			}
		}
	}
	sort.Strings(out)
	return out
}

// patchPages applies the patches to the current pages and returns the
// changed pages by id. Everything outside the scope, and any change the
// patch did not ask for, rejects the whole patch.
func patchPages(p *core.Project, current []core.Page, out designOut, idMap map[string]string, scope *core.ChangeScope) (map[string]core.Page, []string, map[string][]int, error) {
	rej := &PatchRejected{}
	inScope := func(id string) bool { return scope == nil || scope.Open || scope.Page(id) != nil }
	gone := map[string]bool{}
	for _, id := range out.Removed {
		gone[id] = true
		if !inScope(id) {
			rej.Unexpected = append(rej.Unexpected, id+" dihapus")
		}
	}
	for _, pg := range out.Pages {
		if slices.ContainsFunc(current, func(c core.Page) bool { return c.ID == pg.ID }) {
			rej.Unexpected = append(rej.Unexpected, pg.ID+" dikirim utuh; revisi hanya boleh berupa patch")
		}
	}
	changed := map[string]core.Page{}
	moved := map[string][]int{}
	seen := map[string]bool{}
	for _, pp := range out.Patches {
		for _, o := range pp.Operations {
			rej.Requested = append(rej.Requested, opText(pp.Page, o))
		}
		i := slices.IndexFunc(current, func(c core.Page) bool { return c.ID == pp.Page })
		switch {
		case i < 0:
			return nil, nil, nil, fmt.Errorf("patch untuk halaman %s yang tidak ada", pp.Page)
		case gone[pp.Page]:
			return nil, nil, nil, fmt.Errorf("halaman %s dihapus sekaligus di-patch", pp.Page)
		case seen[pp.Page]:
			return nil, nil, nil, fmt.Errorf("halaman %s punya lebih dari satu patch; gabungkan operasinya", pp.Page)
		case len(pp.Operations) == 0:
			return nil, nil, nil, fmt.Errorf("patch %s tanpa operasi", pp.Page)
		}
		seen[pp.Page] = true
		prev := current[i]
		if !inScope(prev.ID) {
			rej.Unexpected = append(rej.Unexpected, prev.ID+" di-patch, padahal tidak diminta")
		}
		pg, origins, err := applyOps(prev, pp.Operations)
		if err != nil {
			return nil, nil, nil, err
		}
		np, ok := normPage(p, pg, idMap)
		if !ok {
			return nil, nil, nil, fmt.Errorf("halaman %s tidak valid setelah di-patch", prev.ID)
		}
		rej.Unexpected = append(rej.Unexpected, checkPatch(prev, np, origins, pp.Operations, scope, gone)...)
		changed[prev.ID] = np
		moved[prev.ID] = origins
	}
	if len(rej.Unexpected) > 0 {
		return nil, nil, nil, rej
	}
	return changed, rej.Requested, moved, nil
}

// checkPatch compares a page before and after its patch: only the targets
// of the operations, and page fields the scope allows, may differ.
func checkPatch(prev, next core.Page, origins []int, ops []patchOp, scope *core.ChangeScope, gone map[string]bool) []string {
	var out []string
	id := prev.ID
	sp := scope.Page(id)
	picked := func(i int) bool {
		return sp == nil || len(sp.Components) == 0 || slices.Contains(sp.Components, i)
	}
	fieldsOK := scope == nil || scope.Fields || sp != nil && sp.Fields
	content, place := map[int]bool{}, map[int]bool{} // targets whose content / position may change
	allowed := map[string]bool{}
	for _, o := range ops {
		if o.Target == nil {
			if o.Op == "update" {
				for k := range o.PageFields {
					allowed[k] = fieldsOK
				}
			}
			continue
		}
		t := *o.Target
		if !picked(t) {
			out = append(out, fmt.Sprintf("%s [%d] %s, padahal yang dipilih %v", id, t, strings.ToUpper(o.Op), sp.Components))
		}
		switch o.Op {
		case "update":
			content[t] = true
		case "move":
			place[t] = true
		case "remove":
			content[t], place[t] = true, true
		}
	}
	// The layout follows the component operations; set as a page field it
	// needs the page-field scope like any other.
	for _, k := range changedKeys(prev, next, "components", "id", "layout") {
		if !allowed[k] {
			out = append(out, fmt.Sprintf("%s %s berubah", id, k))
		}
	}
	if ok, set := allowed["layout"]; set && !ok {
		out = append(out, id+" layout berubah")
	}
	last := -1
	kept := map[int]bool{}
	for pos, o := range origins {
		if o < 0 {
			continue
		}
		kept[o] = true
		if !content[o] {
			pc, nc := prev.Components[o], next.Components[pos]
			if gone[pc.Target] {
				pc.Target = "" // its page is removed in this patch
			}
			for _, k := range changedKeys(pc, nc) {
				out = append(out, fmt.Sprintf("%s [%d] %s berubah", id, o, k))
			}
		}
		if place[o] {
			continue
		}
		if o < last {
			out = append(out, fmt.Sprintf("%s [%d] pindah posisi", id, o))
		}
		last = o
	}
	for i := range prev.Components {
		if !kept[i] && !content[i] {
			out = append(out, fmt.Sprintf("%s [%d] hilang", id, i))
		}
	}
	return out
}

// remapScope follows the picked components to their positions after a
// patch, so a follow-up patch (visual fix) targets the same components.
// Components the patch added belong to the request too.
func remapScope(scope *core.ChangeScope, origins map[string][]int) *core.ChangeScope {
	if scope == nil {
		return nil
	}
	out := *scope
	out.Pages = slices.Clone(scope.Pages)
	for i, sp := range out.Pages {
		o, ok := origins[sp.ID]
		if !ok || len(sp.Components) == 0 {
			continue
		}
		var comps []int
		for pos, from := range o {
			if from < 0 || slices.Contains(sp.Components, from) {
				comps = append(comps, pos)
			}
		}
		out.Pages[i].Components = comps
	}
	return &out
}
