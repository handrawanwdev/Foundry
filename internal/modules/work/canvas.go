package work

import (
	"fmt"
	"slices"
	"strings"

	"github.com/handrawanw/foundry/internal/core"
)

// Canvas layer: the visual editor edits the Design IR tree of one page
// directly, by component and node ids. Foundry applies the operations, then
// validates the page; an invalid result rejects every operation. No AI is
// involved and nothing outside the page changes.

// CanvasOp is one edit of a page's IR tree.
//
//	add       Component placed in Parent (node id, "" = root) at Index (nil = end)
//	remove    component ID
//	move      component or node ID into Parent at Index
//	update    component ID replaced by Component (the id stays)
//	set_node  node ID: its props (type, columns, ratio, side, placement,
//	          gap, align, surface, elevation) replaced by Node's
//	wrap      sibling keys IDs grouped into a new node Node (id and type)
//	unwrap    node ID replaced by its children
//	set_page  Fields: name, purpose, kind, density, frame, nav, width
type CanvasOp struct {
	Op        string              `json:"op"`
	ID        string              `json:"id,omitempty"`
	Parent    string              `json:"parent,omitempty"`
	Index     *int                `json:"index,omitempty"`
	Component *core.PageComponent `json:"component,omitempty"`
	Node      *core.LayoutNode    `json:"node,omitempty"`
	IDs       []string            `json:"ids,omitempty"`
	Fields    map[string]string   `json:"fields,omitempty"`
}

// MaxCanvasOps caps one edit.
const MaxCanvasOps = 50

// EditPage applies canvas operations to one page of the draft design and
// returns what they did, readable.
func EditPage(p *core.Project, pageID string, ops []CanvasOp) ([]string, error) {
	d := p.Design
	if d == nil || len(d.Pages) == 0 {
		return nil, fmt.Errorf("belum ada rancangan")
	}
	if d.Revision != nil {
		return nil, fmt.Errorf("simpan atau batalkan perubahan rancangan dulu")
	}
	if len(ops) == 0 || len(ops) > MaxCanvasOps {
		return nil, fmt.Errorf("jumlah operasi harus 1-%d", MaxCanvasOps)
	}
	i := slices.IndexFunc(d.Pages, func(pg core.Page) bool { return pg.ID == pageID })
	if i < 0 {
		return nil, fmt.Errorf("halaman %q tidak ada", pageID)
	}
	pages := slices.Clone(d.Pages)
	core.EnsureIR(pages, p.Name)
	pg := clonePages(pages[i : i+1])[0]
	var done []string
	for n, o := range ops {
		msg, err := canvasOp(&pg, o)
		if err != nil {
			return nil, fmt.Errorf("operasi %d (%s): %w", n+1, o.Op, err)
		}
		done = append(done, pageID+" "+msg)
	}
	pages[i] = pg
	if err := core.ValidateDesign([]core.Page{pg}, pages); err != nil {
		return nil, err
	}
	d.Pages = pages
	d.Status, d.Screens, d.Scope, d.Rejected = core.DesignDraft, nil, nil, ""
	return done, nil
}

func canvasOp(pg *core.Page, o CanvasOp) (string, error) {
	l := pg.Layout
	switch o.Op {
	case "add":
		if o.Component == nil || strings.TrimSpace(o.Component.Type) == "" {
			return "", fmt.Errorf("component kosong")
		}
		c := *o.Component
		list := make([]opEntry, len(pg.Components))
		for k, x := range pg.Components {
			list[k] = opEntry{k, x}
		}
		c.ID = newID(list, c)
		if c.Type != "nav" {
			if err := insertKey(l, core.LayoutNode{Component: c.ID}, o.Parent, o.Index); err != nil {
				return "", err
			}
		}
		pg.Components = append(pg.Components, c)
		return "ADD " + componentName(c) + " #" + c.ID, nil
	case "remove":
		k := componentIndex(*pg, o.ID)
		if k < 0 {
			return "", fmt.Errorf("komponen %q tidak ada", o.ID)
		}
		l.Detach(o.ID)
		pg.Components = slices.Delete(slices.Clone(pg.Components), k, k+1)
		return "REMOVE #" + o.ID, nil
	case "update":
		k := componentIndex(*pg, o.ID)
		if k < 0 {
			return "", fmt.Errorf("komponen %q tidak ada", o.ID)
		}
		if o.Component == nil || strings.TrimSpace(o.Component.Type) == "" {
			return "", fmt.Errorf("component kosong")
		}
		old, c := pg.Components[k], *o.Component
		c.ID = old.ID
		switch {
		case old.Type != "nav" && c.Type == "nav":
			l.Detach(c.ID)
		case old.Type == "nav" && c.Type != "nav":
			l.Place(c.ID, "", false)
		}
		pg.Components[k] = c
		return "UPDATE #" + c.ID, nil
	case "move":
		if o.ID == "" || o.ID == l.Root.ID {
			return "", fmt.Errorf("id kosong atau root")
		}
		n, from, ok := takeKey(l, o.ID)
		if !ok {
			return "", fmt.Errorf("%q tidak ada di layout", o.ID)
		}
		dropOrder(l, from, o.ID)
		if err := insertKey(l, n, o.Parent, o.Index); err != nil {
			return "", err
		}
		return fmt.Sprintf("MOVE #%s → %s", o.ID, cmpOr(o.Parent, "root")), nil
	case "set_node":
		n := findNode(&l.Root, o.ID)
		if n == nil || o.Node == nil {
			return "", fmt.Errorf("node %q tidak ada atau props kosong", o.ID)
		}
		p := o.Node
		n.Type, n.Columns, n.Ratio, n.Side, n.Placement = p.Type, p.Columns, p.Ratio, p.Side, p.Placement
		n.Gap, n.Align, n.Surface, n.Elevation = p.Gap, p.Align, p.Surface, p.Elevation
		return "SET node #" + o.ID, nil
	case "wrap":
		return wrapKeys(l, o)
	case "unwrap":
		return unwrapNode(l, o.ID)
	case "set_page":
		return setCanvasPage(pg, o.Fields)
	}
	return "", fmt.Errorf("operasi tidak dikenal (add, remove, move, update, set_node, wrap, unwrap, set_page)")
}

func componentIndex(pg core.Page, id string) int {
	if id == "" {
		return -1
	}
	return slices.IndexFunc(pg.Components, func(c core.PageComponent) bool { return c.ID == id })
}

// findNode returns the layout node with this id ("" = root).
func findNode(n *core.LayoutNode, id string) *core.LayoutNode {
	if n.Component != "" {
		return nil
	}
	if id == "" || n.ID == id {
		return n
	}
	for i := range n.Children {
		if f := findNode(&n.Children[i], id); f != nil {
			return f
		}
	}
	return nil
}

// parentOf returns the node holding key, nil when key is not in the tree.
func parentOf(n *core.LayoutNode, key string) *core.LayoutNode {
	for i := range n.Children {
		if n.Children[i].Key() == key {
			return n
		}
		if n.Children[i].Component == "" {
			if f := parentOf(&n.Children[i], key); f != nil {
				return f
			}
		}
	}
	return nil
}

// takeKey removes a leaf or node (with its subtree) from the tree and
// returns it with the id of the node it came from.
func takeKey(l *core.PageLayout, key string) (core.LayoutNode, string, bool) {
	par := parentOf(&l.Root, key)
	if par == nil {
		return core.LayoutNode{}, "", false
	}
	k := slices.IndexFunc(par.Children, func(c core.LayoutNode) bool { return c.Key() == key })
	n := par.Children[k]
	par.Children = slices.Delete(slices.Clone(par.Children), k, k+1)
	return n, par.ID, true
}

// insertKey puts n into the parent node at index (nil or past the end =
// last). Device child orders of that parent get it at the end.
func insertKey(l *core.PageLayout, n core.LayoutNode, parent string, index *int) error {
	par := findNode(&l.Root, parent)
	if par == nil {
		return fmt.Errorf("node induk %q tidak ada", parent)
	}
	at := len(par.Children)
	if index != nil && *index >= 0 && *index < at {
		at = *index
	}
	par.Children = slices.Insert(slices.Clone(par.Children), at, n)
	for _, d := range l.Devices {
		if ch, ok := d.Nodes[par.ID]; ok && len(ch.Order) > 0 && par.ID != "" {
			ch.Order = append(slices.Clone(ch.Order), n.Key())
			d.Nodes[par.ID] = ch
		}
	}
	return nil
}

// dropOrder removes key from the device child orders of node from.
func dropOrder(l *core.PageLayout, from, key string) {
	for _, d := range l.Devices {
		if ch, ok := d.Nodes[from]; ok && from != "" {
			ch.Order = slices.DeleteFunc(slices.Clone(ch.Order), func(k string) bool { return k == key })
			d.Nodes[from] = ch
		}
	}
}

func wrapKeys(l *core.PageLayout, o CanvasOp) (string, error) {
	if o.Node == nil || o.Node.ID == "" || o.Node.Type == "" || len(o.IDs) == 0 {
		return "", fmt.Errorf("wrap butuh ids dan node baru (id, type)")
	}
	if findNode(&l.Root, o.Node.ID) != nil || parentOf(&l.Root, o.Node.ID) != nil {
		return "", fmt.Errorf("id %q sudah dipakai", o.Node.ID)
	}
	par := parentOf(&l.Root, o.IDs[0])
	if par == nil {
		return "", fmt.Errorf("%q tidak ada di layout", o.IDs[0])
	}
	first := -1
	group := *o.Node
	group.Component, group.Children = "", nil
	var keep []core.LayoutNode
	for _, c := range par.Children {
		if slices.Contains(o.IDs, c.Key()) {
			if first < 0 {
				first = len(keep)
			}
			group.Children = append(group.Children, c)
			continue
		}
		keep = append(keep, c)
	}
	if len(group.Children) != len(o.IDs) {
		return "", fmt.Errorf("ids harus anak dari node yang sama")
	}
	par.Children = slices.Insert(keep, first, group)
	for _, d := range l.Devices {
		if ch, ok := d.Nodes[par.ID]; ok && len(ch.Order) > 0 && par.ID != "" {
			at := len(ch.Order)
			for k, key := range ch.Order {
				if slices.Contains(o.IDs, key) {
					at = min(at, k)
				}
			}
			ch.Order = slices.DeleteFunc(slices.Clone(ch.Order), func(k string) bool { return slices.Contains(o.IDs, k) })
			ch.Order = slices.Insert(ch.Order, min(at, len(ch.Order)), group.ID)
			d.Nodes[par.ID] = ch
		}
	}
	return fmt.Sprintf("WRAP %s → %s #%s", strings.Join(o.IDs, ", "), group.Type, group.ID), nil
}

func unwrapNode(l *core.PageLayout, id string) (string, error) {
	if id == "" || id == l.Root.ID {
		return "", fmt.Errorf("root tidak bisa dilepas")
	}
	par := parentOf(&l.Root, id)
	n := findNode(&l.Root, id)
	if par == nil || n == nil {
		return "", fmt.Errorf("node %q tidak ada", id)
	}
	k := slices.IndexFunc(par.Children, func(c core.LayoutNode) bool { return c.Key() == id })
	kids := n.Children
	par.Children = slices.Insert(slices.Delete(slices.Clone(par.Children), k, k+1), k, kids...)
	var keys []string
	for _, c := range kids {
		keys = append(keys, c.Key())
	}
	for dev, d := range l.Devices {
		delete(d.Nodes, id)
		d.Hide = slices.DeleteFunc(slices.Clone(d.Hide), func(h string) bool { return h == id })
		if ch, ok := d.Nodes[par.ID]; ok && len(ch.Order) > 0 && par.ID != "" {
			at := slices.Index(ch.Order, id)
			ch.Order = slices.Insert(slices.Delete(slices.Clone(ch.Order), at, at+1), at, keys...)
			d.Nodes[par.ID] = ch
		}
		l.Devices[dev] = d
	}
	return "UNWRAP #" + id, nil
}

// canvasPageFields are the page fields the canvas may set; frame, nav and
// width belong to the layout.
var canvasPageFields = []string{"name", "purpose", "kind", "density", "frame", "nav", "width"}

func setCanvasPage(pg *core.Page, fields map[string]string) (string, error) {
	if len(fields) == 0 {
		return "", fmt.Errorf("fields kosong")
	}
	var keys []string
	for k, v := range fields {
		v = strings.TrimSpace(v)
		switch k {
		case "name":
			if v == "" {
				return "", fmt.Errorf("nama halaman kosong")
			}
			pg.Name = v
		case "purpose":
			pg.Purpose = v
		case "kind":
			if v != "" && !pageKinds[v] {
				return "", fmt.Errorf("kind %q tidak dikenal", v)
			}
			pg.Kind = v
		case "density":
			if pg.Density = core.NormDensity(v); pg.Density == "" && v != "" {
				return "", fmt.Errorf("density %q tidak dikenal", v)
			}
		case "frame":
			pg.Layout.Frame = v
		case "nav":
			pg.Layout.Nav = v
		case "width":
			pg.Layout.Width = v
		default:
			return "", fmt.Errorf("field %q tidak boleh diubah (pakai: %s)", k, strings.Join(canvasPageFields, ", "))
		}
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return "SET halaman: " + strings.Join(keys, ", "), nil
}
