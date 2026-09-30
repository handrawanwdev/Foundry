package work

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/handrawanw/foundry/internal/core"
)

// componentOp is one small edit to an existing page. The AI sends these
// instead of whole pages so everything it does not name stays exactly as it
// is. Index and After are positions in the page before any edit.
type componentOp struct {
	Page      string              `json:"page"`
	Op        string              `json:"op"` // remove | move | update | insert | page
	Index     *int                `json:"index"`
	After     *int                `json:"after"` // -1 = top; omitted = bottom
	Component *core.PageComponent `json:"component"`
	Set       map[string]any      `json:"set"` // op page: page fields that change
}

// pageFields the "page" op may set; ids, feature, platform and components
// are changed by other means.
var pageFields = map[string]bool{
	"name": true, "role": true, "purpose": true, "kind": true, "density": true, "states": true, "route": true,
	"access": true, "primary_action": true, "data": true, "sub_feature_ids": true,
}

type opEntry struct {
	orig int // position before the edits; -1 for inserted components
	c    core.PageComponent
}

// applyOps edits one page; the result still goes through normPage.
func applyOps(pg core.Page, ops []componentOp) (core.Page, error) {
	list := make([]opEntry, len(pg.Components))
	for i, c := range pg.Components {
		list[i] = opEntry{i, c}
	}
	find := func(orig int) int {
		return slices.IndexFunc(list, func(e opEntry) bool { return e.orig == orig })
	}
	fail := func(o componentOp, why string) error {
		return fmt.Errorf("perubahan %q di %s: %s", o.Op, pg.ID, why)
	}
	// insertAt is where a component placed after `after` goes.
	insertAt := func(o componentOp) (int, error) {
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
	target := func(o componentOp) (int, error) {
		if o.Index == nil {
			return 0, fail(o, "index kosong")
		}
		k := find(*o.Index)
		if k < 0 {
			return 0, fail(o, fmt.Sprintf("komponen [%d] tidak ada", *o.Index))
		}
		return k, nil
	}
	for _, o := range ops {
		switch o.Op {
		case "remove":
			k, err := target(o)
			if err != nil {
				return pg, err
			}
			list = slices.Delete(list, k, k+1)
		case "update":
			k, err := target(o)
			if err != nil {
				return pg, err
			}
			if o.Component == nil {
				return pg, fail(o, "component kosong")
			}
			list[k].c = *o.Component
		case "move":
			k, err := target(o)
			if err != nil {
				return pg, err
			}
			e := list[k]
			list = slices.Delete(list, k, k+1)
			at, err := insertAt(o)
			if err != nil {
				return pg, err
			}
			list = slices.Insert(list, at, e)
		case "insert":
			if o.Component == nil {
				return pg, fail(o, "component kosong")
			}
			at, err := insertAt(o)
			if err != nil {
				return pg, err
			}
			list = slices.Insert(list, at, opEntry{-1, *o.Component})
		case "page":
			np, err := setPageFields(pg, o.Set)
			if err != nil {
				return pg, fail(o, err.Error())
			}
			pg = np
		default:
			return pg, fail(o, "jenis perubahan tidak dikenal")
		}
	}
	pg.Components = make([]core.PageComponent, len(list))
	for i, e := range list {
		pg.Components[i] = e.c
	}
	return pg, nil
}

// setPageFields replaces the given top-level page fields; the page is
// rebuilt from JSON so nothing is shared with the previous version.
func setPageFields(pg core.Page, set map[string]any) (core.Page, error) {
	if len(set) == 0 {
		return pg, fmt.Errorf("set kosong")
	}
	raw, _ := json.Marshal(pg)
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	for k, v := range set {
		if !pageFields[k] {
			return pg, fmt.Errorf("field %q tidak boleh diubah", k)
		}
		m[k] = v
	}
	raw, _ = json.Marshal(m)
	var out core.Page
	if err := json.Unmarshal(raw, &out); err != nil {
		return pg, err
	}
	return out, nil
}
