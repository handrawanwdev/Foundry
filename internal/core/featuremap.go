package core

import (
	"fmt"
	"strings"
	"unicode"
)

// FeatureMap is the user-editable product feature tree, composed before
// Blueprint. It is a reference for discovery, not Project.Features.
type FeatureMap struct {
	Summary string              `json:"summary,omitempty"`
	Modules []FeatureModule     `json:"modules"`
	Request *FeatureMapRequest  `json:"request,omitempty"`
}

// FeatureModule is a group of related features (Trading, Identity, …).
type FeatureModule struct {
	ID       string        `json:"id"`
	Name     string        `json:"name"`
	Features []FeatureNode `json:"features"`
}

// FeatureNode is a feature or subfeature. Children are subfeatures.
type FeatureNode struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	Purpose     string        `json:"purpose,omitempty"`
	Benefit     string        `json:"benefit,omitempty"`
	Outcome     string        `json:"outcome,omitempty"`
	Children    []FeatureNode `json:"children,omitempty"`
}

// FeatureMapRequest is an AI action waiting to land on the map.
type FeatureMapRequest struct {
	Action   string `json:"action"`             // generate | expand | gaps
	NodeID   string `json:"node_id,omitempty"`  // expand target
	Text     string `json:"text,omitempty"`
	Rejected string `json:"rejected,omitempty"`
}

const (
	FeatureMapGenerate = "generate"
	FeatureMapExpand   = "expand"
	FeatureMapGaps     = "gaps"

	MaxFeatureNodes = 80
	MaxFeatureDepth = 4
)

// Empty reports whether the map has no named feature.
func (m *FeatureMap) Empty() bool {
	return m == nil || m.Count() == 0
}

// Count is the number of feature nodes (modules themselves are not counted).
func (m *FeatureMap) Count() int {
	if m == nil {
		return 0
	}
	n := 0
	m.Walk(func(*FeatureNode, *FeatureModule, int) { n++ })
	return n
}

// Walk visits every feature node, depth 1 at the module root.
func (m *FeatureMap) Walk(fn func(n *FeatureNode, mod *FeatureModule, depth int)) {
	if m == nil || fn == nil {
		return
	}
	for i := range m.Modules {
		walkNodes(m.Modules[i].Features, &m.Modules[i], 1, fn)
	}
}

func walkNodes(nodes []FeatureNode, mod *FeatureModule, depth int, fn func(*FeatureNode, *FeatureModule, int)) {
	for i := range nodes {
		fn(&nodes[i], mod, depth)
		if len(nodes[i].Children) > 0 {
			walkNodes(nodes[i].Children, mod, depth+1, fn)
		}
	}
}

// FindModule returns the module with id, or nil.
func (m *FeatureMap) FindModule(id string) *FeatureModule {
	if m == nil || id == "" {
		return nil
	}
	for i := range m.Modules {
		if m.Modules[i].ID == id {
			return &m.Modules[i]
		}
	}
	return nil
}

// Find returns the feature node with id, or nil.
func (m *FeatureMap) Find(id string) *FeatureNode {
	if m == nil || id == "" {
		return nil
	}
	var found *FeatureNode
	m.Walk(func(n *FeatureNode, _ *FeatureModule, _ int) {
		if found == nil && n.ID == id {
			found = n
		}
	})
	return found
}

// Locate returns the module and, when id is a feature, that node.
func (m *FeatureMap) Locate(id string) (mod *FeatureModule, node *FeatureNode) {
	if m == nil || id == "" {
		return nil, nil
	}
	if mod = m.FindModule(id); mod != nil {
		return mod, nil
	}
	m.Walk(func(n *FeatureNode, parent *FeatureModule, _ int) {
		if node == nil && n.ID == id {
			mod, node = parent, n
		}
	})
	return mod, node
}

// Normalize trims text, drops nameless entries, caps depth and size, and
// assigns unique ids.
func (m *FeatureMap) Normalize() {
	if m == nil {
		return
	}
	m.Summary = strings.TrimSpace(m.Summary)
	used := map[string]bool{}
	var modules []FeatureModule
	nleft := MaxFeatureNodes
	for _, mod := range m.Modules {
		mod.Name = strings.TrimSpace(mod.Name)
		if mod.Name == "" {
			continue
		}
		mod.ID = uniqueSlug(used, mod.ID, mod.Name, "mod")
		mod.Features = normalizeNodes(mod.Features, 1, used, &nleft)
		modules = append(modules, mod)
	}
	m.Modules = modules
}

func normalizeNodes(in []FeatureNode, depth int, used map[string]bool, nleft *int) []FeatureNode {
	if depth > MaxFeatureDepth || *nleft <= 0 {
		return nil
	}
	var out []FeatureNode
	for _, n := range in {
		if *nleft <= 0 {
			break
		}
		n.Name = strings.TrimSpace(n.Name)
		if n.Name == "" {
			continue
		}
		n.Description = strings.TrimSpace(n.Description)
		n.Purpose = strings.TrimSpace(n.Purpose)
		n.Benefit = strings.TrimSpace(n.Benefit)
		n.Outcome = strings.TrimSpace(n.Outcome)
		n.ID = uniqueSlug(used, n.ID, n.Name, "feat")
		*nleft--
		if depth < MaxFeatureDepth {
			n.Children = normalizeNodes(n.Children, depth+1, used, nleft)
		} else {
			n.Children = nil
		}
		out = append(out, n)
	}
	return out
}

// Validate checks ids are unique and names are present. An empty map is valid.
func (m *FeatureMap) Validate() error {
	if m == nil {
		return nil
	}
	seen := map[string]bool{}
	claim := func(kind, id, name string) error {
		if name == "" {
			return fmt.Errorf("%s tanpa nama", kind)
		}
		if id == "" {
			return fmt.Errorf("%s %q tanpa id", kind, name)
		}
		if seen[id] {
			return fmt.Errorf("id fitur duplikat: %s", id)
		}
		seen[id] = true
		return nil
	}
	for i := range m.Modules {
		mod := &m.Modules[i]
		if err := claim("modul", mod.ID, mod.Name); err != nil {
			return err
		}
	}
	var err error
	m.Walk(func(n *FeatureNode, _ *FeatureModule, _ int) {
		if err != nil {
			return
		}
		err = claim("fitur", n.ID, n.Name)
	})
	return err
}

// FeatureMapFromFeatures builds a one-module map from blueprint features,
// used when discovery produced features before a map existed.
func FeatureMapFromFeatures(feats []Feature) *FeatureMap {
	if len(feats) == 0 {
		return nil
	}
	m := &FeatureMap{Modules: []FeatureModule{{Name: "Fitur"}}}
	for _, f := range feats {
		n := FeatureNode{ID: f.ID, Name: f.Name, Description: f.Description}
		for _, s := range f.SubFeatures {
			n.Children = append(n.Children, FeatureNode{ID: s.ID, Name: s.Name})
		}
		m.Modules[0].Features = append(m.Modules[0].Features, n)
	}
	m.Normalize()
	return m
}

func uniqueSlug(used map[string]bool, id, name, prefix string) string {
	base := slugID(id)
	if base == "" {
		base = slugID(name)
	}
	if base == "" {
		base = prefix
	}
	out := base
	for n := 2; used[out]; n++ {
		out = fmt.Sprintf("%s-%d", base, n)
	}
	used[out] = true
	return out
}

func slugID(s string) string {
	s = strings.Map(func(r rune) rune {
		if r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
			return unicode.ToLower(r)
		}
		return '-'
	}, strings.TrimSpace(s))
	s = strings.Trim(s, "-")
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	if len(s) > 32 {
		s = strings.TrimRight(s[:32], "-")
	}
	return s
}
