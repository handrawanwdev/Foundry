package core

import (
	"fmt"
	"strings"
	"unicode"
)

// FeatureMap is the user-editable product feature tree, composed before
// Blueprint. It is a reference for discovery, not Project.Features.
type FeatureMap struct {
	Summary string             `json:"summary,omitempty"`
	Modules []FeatureModule    `json:"modules"`
	Request *FeatureMapRequest `json:"request,omitempty"`
}

// FeatureModule is a group of related features (Trading, Identity, …).
type FeatureModule struct {
	ID       string        `json:"id"`
	Name     string        `json:"name"`
	Features []FeatureNode `json:"features"`
}

// FeatureNode is a feature or subfeature. Children are subfeatures.
type FeatureNode struct {
	ID string `json:"id"`
	// ParentID is the module or parent feature node this item belongs to.
	ParentID string `json:"parent_id,omitempty"`
	Name     string `json:"name"`
	// FeatureID links this node to the blueprint feature it implements;
	// SubFeatureID links it to a specific subfeature. Both are filled by
	// discovery or by the deterministic legacy mapping in MapFeatureMap.
	FeatureID    string        `json:"feature_id,omitempty"`
	SubFeatureID string        `json:"sub_feature_id,omitempty"`
	Description  string        `json:"description,omitempty"`
	Purpose      string        `json:"purpose,omitempty"`
	Benefit      string        `json:"benefit,omitempty"`
	Outcome      string        `json:"outcome,omitempty"`
	Children     []FeatureNode `json:"children,omitempty"`
}

// FeatureMapRequest is an AI action waiting to land on the map.
type FeatureMapRequest struct {
	Action   string `json:"action"`            // generate | expand | gaps
	NodeID   string `json:"node_id,omitempty"` // expand target
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
		mod.Features = normalizeNodes(mod.Features, 1, mod.ID, used, &nleft)
		modules = append(modules, mod)
	}
	m.Modules = modules
}

func normalizeNodes(in []FeatureNode, depth int, parentID string, used map[string]bool, nleft *int) []FeatureNode {
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
		n.ParentID = parentID
		*nleft--
		if depth < MaxFeatureDepth {
			n.Children = normalizeNodes(n.Children, depth+1, n.ID, used, nleft)
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
	LinkFeatureMap(m, feats)
	return m
}

// LinkFeatureMap records the feature-map node each blueprint feature and
// subfeature came from. It matches the recorded source id first, then the
// name inside the same parent. Nothing is invented: an unmatched item keeps
// an empty SourceID.
func LinkFeatureMap(m *FeatureMap, feats []Feature) {
	if m == nil {
		return
	}
	used := map[string]bool{}
	for i := range feats {
		f := &feats[i]
		if f.SourceID != "" && m.Find(f.SourceID) == nil {
			f.SourceID = ""
		}
		if f.SourceID == "" {
			if n := m.findUnused(f.Name, used); n != nil {
				f.SourceID = n.ID
			}
		}
		if f.SourceID != "" {
			used[f.SourceID] = true
		}
		parent := m.Find(f.SourceID)
		for j := range f.SubFeatures {
			s := &f.SubFeatures[j]
			s.FeatureID = f.ID
			if s.SourceID != "" && (parent == nil || findChildID(parent.Children, s.SourceID) == nil) {
				s.SourceID = ""
			}
			if s.SourceID != "" || parent == nil {
				continue
			}
			if n := findChild(parent.Children, s.Name, used); n != nil {
				s.SourceID = n.ID
				used[n.ID] = true
			}
		}
	}
}

func findChildID(nodes []FeatureNode, id string) *FeatureNode {
	for i := range nodes {
		if nodes[i].ID == id {
			return &nodes[i]
		}
	}
	return nil
}

func (m *FeatureMap) findUnused(name string, used map[string]bool) *FeatureNode {
	var found *FeatureNode
	m.Walk(func(n *FeatureNode, _ *FeatureModule, depth int) {
		if found != nil || depth != 1 || used[n.ID] || !equalName(n.Name, name) {
			return
		}
		found = n
	})
	return found
}

func findChild(nodes []FeatureNode, name string, used map[string]bool) *FeatureNode {
	for i := range nodes {
		if !used[nodes[i].ID] && equalName(nodes[i].Name, name) {
			return &nodes[i]
		}
	}
	return nil
}

func equalName(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

// MapFeatureMap links Feature Discovery nodes to the blueprint features they
// implement. Explicit ids win; then exact names; then a conservative
// token match using the node text and its module. A node whose module has one
// confident feature match inherits it, so a group such as "Kelola Tugas"
// points at the same blueprint feature. Nodes with no confident match stay
// empty, never guessed.
func MapFeatureMap(m *FeatureMap, feats []Feature) {
	if m == nil || len(feats) == 0 {
		return
	}
	byID := map[string]*Feature{}
	for i := range feats {
		byID[strings.ToLower(strings.TrimSpace(feats[i].ID))] = &feats[i]
	}
	for mi := range m.Modules {
		mod := &m.Modules[mi]
		matched := make([]*Feature, len(mod.Features))
		for ni := range mod.Features {
			node := &mod.Features[ni]
			if f := byID[strings.ToLower(strings.TrimSpace(node.FeatureID))]; f != nil {
				matched[ni] = f
				continue
			}
			node.FeatureID = ""
			matched[ni] = matchNodeFeature(node, mod.Name, feats)
		}
		// A module is a group of related features. When one node maps with
		// confidence, its siblings inherit the same blueprint feature.
		var moduleFeature *Feature
		for _, f := range matched {
			if f != nil {
				moduleFeature = f
				break
			}
		}
		if moduleFeature == nil {
			moduleFeature = bestFeature("", mod.Name, feats)
		}
		for ni := range mod.Features {
			node := &mod.Features[ni]
			f := matched[ni]
			if f == nil {
				f = moduleFeature
			}
			if f == nil {
				node.FeatureID, node.SubFeatureID = "", ""
			} else {
				node.FeatureID = f.ID
				node.SubFeatureID = ""
				if f.SourceID == "" {
					f.SourceID = node.ID
				}
				mapNodeChildren(node, f, feats)
			}
		}
	}
}

func mapNodeChildren(node *FeatureNode, f *Feature, feats []Feature) {
	for ci := range node.Children {
		child := &node.Children[ci]
		child.FeatureID = f.ID
		if s := matchSubFeature(child.Name+" "+child.Description, feats); s != nil && s.FeatureID == f.ID {
			child.SubFeatureID = s.ID
		} else {
			child.SubFeatureID = ""
		}
		mapNodeChildren(child, f, feats)
	}
}

func matchNodeFeature(node *FeatureNode, module string, feats []Feature) *Feature {
	if node == nil {
		return nil
	}
	for i := range feats {
		if equalName(feats[i].Name, node.Name) {
			return &feats[i]
		}
	}
	text := node.Name + " " + node.Description
	return bestFeature(text, module, feats)
}

func bestFeature(text, module string, feats []Feature) *Feature {
	if strings.TrimSpace(text+" "+module) == "" {
		return nil
	}
	var best *Feature
	candidate := text + " " + module
	bestScore, bestName, bestModule := 0.0, 0.0, 0.0
	for i := range feats {
		f := &feats[i]
		nameScore := tokenScore(candidate, f.Name)
		moduleScore := 0.0
		if strings.TrimSpace(module) != "" {
			moduleScore = tokenScore(module, f.Name)
		}
		score := maxFloat(nameScore, tokenScore(candidate, f.Description))
		if score > bestScore || (score == bestScore && (nameScore > bestName || nameScore == bestName && moduleScore > bestModule)) {
			best, bestScore, bestName, bestModule = f, score, nameScore, moduleScore
		}
	}
	if bestScore < 0.5 {
		return nil
	}
	return best
}

func matchSubFeature(text string, feats []Feature) *SubFeature {
	var best *SubFeature
	bestScore := 0.0
	for i := range feats {
		for j := range feats[i].SubFeatures {
			s := &feats[i].SubFeatures[j]
			if score := tokenScore(text, s.Name); score > bestScore {
				best, bestScore = s, score
			}
		}
	}
	if bestScore < 0.5 {
		return nil
	}
	return best
}

var featureStopwords = map[string]bool{
	"dan": true, "atau": true, "yang": true, "untuk": true, "dengan": true, "di": true, "ke": true,
	"dari": true, "pada": true, "dalam": true, "secara": true, "ini": true, "itu": true, "semua": true,
	"tiap": true, "setiap": true, "saat": true, "sudah": true, "akan": true, "baru": true, "lagi": true,
	"bisa": true, "dapat": true, "milik": true, "oleh": true, "sebagai": true, "agar": true, "supaya": true,
	"jika": true, "bila": true, "telah": true, "masih": true, "dibuat": true, "buat": true,
}

var featureGenericTokens = map[string]bool{
	"tugas": true, "data": true, "halaman": true, "sistem": true, "aplikasi": true, "fitur": true, "layar": true,
	"daftar": true, "catatan": true,
}

func tokenScore(a, b string) float64 {
	ta, tb := featureTokens(a), featureTokens(b)
	if len(ta) == 0 || len(tb) == 0 {
		return 0
	}
	inter := 0
	var shared []string
	for t := range ta {
		if tb[t] {
			inter++
			shared = append(shared, t)
		}
	}
	if inter == 0 {
		return 0
	}
	union := len(ta) + len(tb) - inter
	score := float64(inter) / float64(union)
	if inter >= 2 && inter == minInt(len(ta), len(tb)) {
		score = maxFloat(score, 0.75)
	}
	for _, t := range shared {
		if len(t) >= 6 && !featureGenericTokens[t] {
			score = maxFloat(score, 0.5)
		}
	}
	return score
}

func featureTokens(s string) map[string]bool {
	out := map[string]bool{}
	for _, t := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if len(t) >= 3 && !featureStopwords[t] {
			out[t] = true
		}
	}
	return out
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// BackfillCommandFeatures stores the features a command implements when an
// older workflow has no explicit link yet. It uses the requirements the
// command covers; a command that already has a link is left untouched.
func BackfillCommandFeatures(p *Project) {
	if p == nil || p.Workflow == nil {
		return
	}
	for i := range p.Workflow.Commands {
		c := &p.Workflow.Commands[i]
		if len(c.FeatureIDs) > 0 || len(c.SubFeatureIDs) > 0 {
			continue
		}
		c.FeatureIDs = ResolveCommandFeatureIDs(*c, p.Features)
	}
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
