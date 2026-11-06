package repoplan

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
)

// MaxOps bounds one revision.
const MaxOps = 200

// Op names.
const (
	OpSetNode          = "set_node"
	OpRemoveNode       = "remove_node"
	OpMoveNode         = "move_node"
	OpSetModule        = "set_module"
	OpRemoveModule     = "remove_module"
	OpAddRule          = "add_rule"
	OpRemoveRule       = "remove_rule"
	OpAddDependency    = "add_dependency"
	OpRemoveDependency = "remove_dependency"
	OpSetInterface     = "set_interface"
	OpRemoveInterface  = "remove_interface"
	OpSetMeta          = "set_meta"
)

var ErrNoOps = errors.New("tidak ada perubahan struktur")

// Apply revises plan in place and returns what changed, one line per op.
// It stops at the first op that cannot apply; the caller works on a clone,
// so a refused revision leaves the real plan untouched.
func Apply(plan *core.RepoPlan, ops []core.RepoOp) ([]string, error) {
	if len(ops) == 0 {
		return nil, ErrNoOps
	}
	if len(ops) > MaxOps {
		return nil, fmt.Errorf("terlalu banyak perubahan sekaligus (maks. %d)", MaxOps)
	}
	var changes []string
	for i, op := range ops {
		msg, err := applyOp(plan, op)
		if err != nil {
			return nil, fmt.Errorf("perubahan %d (%s): %w", i+1, op.Op, err)
		}
		changes = append(changes, msg)
		Normalize(plan)
	}
	return changes, nil
}

func applyOp(plan *core.RepoPlan, op core.RepoOp) (string, error) {
	switch strings.ToLower(strings.TrimSpace(op.Op)) {
	case OpSetNode:
		if op.Node == nil {
			return "", errors.New("node wajib diisi")
		}
		n, ok := normalizeNode(*op.Node)
		if !ok {
			return "", fmt.Errorf("path %q tidak valid", op.Node.Path)
		}
		if i := slices.IndexFunc(plan.Nodes, func(x core.RepoNode) bool { return x.Path == n.Path }); i >= 0 {
			n.Implied = false
			plan.Nodes[i] = n
			return "Mengubah " + Describe(n), nil
		}
		plan.Nodes = append(plan.Nodes, n)
		return "Menambah " + Describe(n), nil

	case OpRemoveNode:
		root, err := findPath(plan, op.Path)
		if err != nil {
			return "", err
		}
		removed := 0
		plan.Nodes = slices.DeleteFunc(plan.Nodes, func(n core.RepoNode) bool {
			if under(n.Path, root) {
				removed++
				return true
			}
			return false
		})
		rewriteRefs(plan, func(e string) string {
			if under(e, root) || under(e+"/", root) {
				return ""
			}
			return e
		})
		return fmt.Sprintf("Menghapus %s (%d node)", root, removed), nil

	case OpMoveNode:
		root, err := findPath(plan, op.Path)
		if err != nil {
			return "", err
		}
		to := CleanPath(op.To, strings.HasSuffix(root, "/"))
		switch {
		case to == "":
			return "", fmt.Errorf("tujuan %q tidak valid", op.To)
		case to == root:
			return "", errors.New("tujuan sama dengan asal")
		case strings.HasSuffix(root, "/") && strings.HasPrefix(to, root):
			return "", errors.New("folder tidak bisa dipindah ke dalam dirinya sendiri")
		case slices.ContainsFunc(plan.Nodes, func(n core.RepoNode) bool { return n.Path == to && !n.Implied }):
			return "", fmt.Errorf("%s sudah ada", to)
		}
		plan.Nodes = slices.DeleteFunc(plan.Nodes, func(n core.RepoNode) bool { return n.Path == to && n.Implied })
		remap := func(e string) string {
			switch {
			case e == root:
				return to
			case e+"/" == root:
				return strings.TrimSuffix(to, "/")
			case strings.HasSuffix(root, "/") && strings.HasPrefix(e, root):
				return to + strings.TrimPrefix(e, root)
			}
			return e
		}
		for i := range plan.Nodes {
			if under(plan.Nodes[i].Path, root) {
				plan.Nodes[i].Path = remap(plan.Nodes[i].Path)
			}
		}
		rewriteRefs(plan, remap)
		// The parent folders of the old place may now be empty implied ones.
		plan.Nodes = dropEmptyImplied(plan.Nodes)
		return fmt.Sprintf("Memindahkan %s ke %s", root, to), nil

	case OpSetModule:
		if op.Module == nil || strings.TrimSpace(op.Module.Module) == "" {
			return "", errors.New("module wajib diisi")
		}
		m := *op.Module
		m.Module = strings.ToLower(strings.TrimSpace(m.Module))
		if i := slices.IndexFunc(plan.Modules, func(x core.RepoModuleMap) bool { return x.Module == m.Module }); i >= 0 {
			plan.Modules[i] = m
			return "Mengubah pemetaan modul " + m.Module, nil
		}
		plan.Modules = append(plan.Modules, m)
		return "Memetakan modul " + m.Module, nil

	case OpRemoveModule:
		if op.Module == nil {
			return "", errors.New("module wajib diisi")
		}
		id := strings.ToLower(strings.TrimSpace(op.Module.Module))
		n := len(plan.Modules)
		plan.Modules = slices.DeleteFunc(plan.Modules, func(x core.RepoModuleMap) bool { return x.Module == id })
		if len(plan.Modules) == n {
			return "", fmt.Errorf("pemetaan modul %q tidak ada", id)
		}
		return "Menghapus pemetaan modul " + id, nil

	case OpAddRule:
		text := clean(op.Text)
		if text == "" {
			return "", errors.New("teks aturan kosong")
		}
		if slices.ContainsFunc(plan.Rules, func(r string) bool { return strings.EqualFold(r, text) }) {
			return "", errors.New("aturan sudah ada")
		}
		plan.Rules = append(plan.Rules, text)
		return "Menambah aturan: " + text, nil

	case OpRemoveRule:
		text := clean(op.Text)
		n := len(plan.Rules)
		plan.Rules = slices.DeleteFunc(plan.Rules, func(r string) bool { return strings.EqualFold(r, text) })
		if len(plan.Rules) == n {
			return "", errors.New("aturan tidak ditemukan; tulis teksnya persis")
		}
		return "Menghapus aturan: " + text, nil

	case OpAddDependency:
		set, err := depSet(plan, op.Kind)
		if err != nil {
			return "", err
		}
		if op.Dep == nil || Endpoint(op.Dep.From) == "" || Endpoint(op.Dep.To) == "" {
			return "", errors.New("dep.from dan dep.to wajib diisi")
		}
		r := *op.Dep
		r.From, r.To = Endpoint(r.From), Endpoint(r.To)
		if i := slices.IndexFunc(*set, func(x core.RepoDepRule) bool { return x.From == r.From && x.To == r.To }); i >= 0 {
			(*set)[i] = r
			return fmt.Sprintf("Mengubah aturan %s %s → %s", op.Kind, r.From, r.To), nil
		}
		*set = append(*set, r)
		return fmt.Sprintf("Menambah aturan %s %s → %s", op.Kind, r.From, r.To), nil

	case OpRemoveDependency:
		set, err := depSet(plan, op.Kind)
		if err != nil {
			return "", err
		}
		if op.Dep == nil {
			return "", errors.New("dep wajib diisi")
		}
		from, to := Endpoint(op.Dep.From), Endpoint(op.Dep.To)
		n := len(*set)
		*set = slices.DeleteFunc(*set, func(x core.RepoDepRule) bool { return x.From == from && x.To == to })
		if len(*set) == n {
			return "", fmt.Errorf("aturan %s %s → %s tidak ditemukan", op.Kind, from, to)
		}
		return fmt.Sprintf("Menghapus aturan %s %s → %s", op.Kind, from, to), nil

	case OpSetInterface:
		if op.Interface == nil {
			return "", errors.New("interface wajib diisi")
		}
		i := *op.Interface
		i.Module, i.Path = strings.ToLower(strings.TrimSpace(i.Module)), Endpoint(i.Path)
		if i.Module == "" || i.Path == "" {
			return "", errors.New("interface.module dan interface.path wajib diisi")
		}
		d := &plan.Dependency
		if j := slices.IndexFunc(d.Interfaces, func(x core.RepoInterface) bool { return x.Module == i.Module && x.Path == i.Path }); j >= 0 {
			d.Interfaces[j] = i
			return "Mengubah public interface " + i.Module + " di " + i.Path, nil
		}
		d.Interfaces = append(d.Interfaces, i)
		return "Menambah public interface " + i.Module + " di " + i.Path, nil

	case OpRemoveInterface:
		if op.Interface == nil {
			return "", errors.New("interface wajib diisi")
		}
		m, p := strings.ToLower(strings.TrimSpace(op.Interface.Module)), Endpoint(op.Interface.Path)
		d := &plan.Dependency
		n := len(d.Interfaces)
		d.Interfaces = slices.DeleteFunc(d.Interfaces, func(x core.RepoInterface) bool { return x.Module == m && x.Path == p })
		if len(d.Interfaces) == n {
			return "", errors.New("public interface tidak ditemukan")
		}
		return "Menghapus public interface " + m + " di " + p, nil

	case OpSetMeta:
		var parts []string
		if v := clean(op.Approach); v != "" {
			plan.Approach = v
			parts = append(parts, "pendekatan")
		}
		if v := clean(op.Topology); v != "" {
			plan.Topology = v
			parts = append(parts, "topologi")
		}
		if v := clean(op.Summary); v != "" {
			plan.Summary = v
			parts = append(parts, "ringkasan")
		}
		if len(parts) == 0 {
			return "", errors.New("approach, topology, atau summary wajib diisi")
		}
		return "Mengubah " + strings.Join(parts, ", "), nil
	}
	return "", fmt.Errorf("op %q tidak dikenal", op.Op)
}

func depSet(plan *core.RepoPlan, kind string) (*[]core.RepoDepRule, error) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "allowed":
		return &plan.Dependency.Allowed, nil
	case "forbidden":
		return &plan.Dependency.Forbidden, nil
	case "communication":
		return &plan.Dependency.Communication, nil
	}
	return nil, errors.New("kind harus allowed, forbidden, atau communication")
}

// findPath resolves a path the user or the AI named to the node's own
// spelling (a folder with its slash).
func findPath(plan *core.RepoPlan, raw string) (string, error) {
	for _, dir := range []bool{true, false} {
		if p := CleanPath(raw, dir); p != "" && slices.ContainsFunc(plan.Nodes, func(n core.RepoNode) bool { return n.Path == p }) {
			return p, nil
		}
	}
	return "", fmt.Errorf("path %q tidak ada di struktur", raw)
}

// under reports whether path is root or lies below the folder root.
func under(path, root string) bool {
	return path == root || strings.HasSuffix(root, "/") && strings.HasPrefix(path, root)
}

// rewriteRefs applies fn to every node path a rule, interface, dependency or
// module location mentions. fn returns "" to drop the reference.
func rewriteRefs(plan *core.RepoPlan, fn func(string) string) {
	for i := range plan.Nodes {
		var deps []string
		for _, d := range plan.Nodes[i].DependsOn {
			if v := fn(d); v != "" {
				deps = append(deps, v)
			}
		}
		plan.Nodes[i].DependsOn = deps
	}
	for i := range plan.Modules {
		var locs []core.RepoLocation
		for _, l := range plan.Modules[i].Locations {
			if v := fn(l.Path); v != "" {
				l.Path = v
				locs = append(locs, l)
			}
		}
		plan.Modules[i].Locations = locs
	}
	d := &plan.Dependency
	for _, set := range []*[]core.RepoDepRule{&d.Allowed, &d.Forbidden, &d.Communication} {
		var out []core.RepoDepRule
		for _, r := range *set {
			if r.From, r.To = fn(r.From), fn(r.To); r.From != "" && r.To != "" {
				out = append(out, r)
			}
		}
		*set = out
	}
	var ifs []core.RepoInterface
	for _, i := range d.Interfaces {
		if i.Path = fn(i.Path); i.Path != "" {
			ifs = append(ifs, i)
		}
	}
	d.Interfaces = ifs
}

// dropEmptyImplied removes implied folders that no node lives in anymore.
func dropEmptyImplied(nodes []core.RepoNode) []core.RepoNode {
	for changed := true; changed; {
		changed = false
		used := map[string]bool{}
		for _, n := range nodes {
			used[n.Parent()] = true
		}
		next := nodes[:0:0]
		for _, n := range nodes {
			if n.Implied && !used[n.Path] {
				changed = true
				continue
			}
			next = append(next, n)
		}
		nodes = next
	}
	return nodes
}
