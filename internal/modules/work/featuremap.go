package work

import (
	"fmt"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
)

// KindFeatureMap composes the product feature tree before Blueprint.
const KindFeatureMap = "feature_map"

type featureMapOut struct {
	Summary string              `json:"summary"`
	Modules []core.FeatureModule `json:"modules"`
	Node    *core.FeatureNode    `json:"node"`
}

// FeatureMapPrompt asks the AI to generate, expand, or fill gaps in the
// feature map. The identity line is unique so test fakes do not match
// the discovery analyst.
func FeatureMapPrompt(p *core.Project) string {
	var b strings.Builder
	b.WriteString(`You are the feature mapper of Foundry. Foundry designs; an external coding agent implements.
Do not write application code. Do not modify files.
The user described a product in ordinary language. Compose a flexible feature tree: modules, features, and subfeatures when a feature has distinct parts.
This is not requirements engineering, scoring, or a workflow. Name what the product needs.

Write every human-readable text in the same language as the story, in simple everyday words a non-expert would say out loud.
Each of description, purpose, benefit, and outcome is one short sentence. No formal, legal, academic, or spec phrasing.
If the story is Indonesian: tulis seperti ngomong biasa, bukan bahasa baku. Hindari "memungkinkan", "tersebut", "melakukan", "guna", "serta", "terpenuhi", "diperbarui", "pengguna dapat".

Each feature has: name, description, purpose (tujuan: buat apa), benefit (manfaat: enaknya apa), outcome (hasil: kalau berhasil jadi apa).
Rules:
- universal: any kind of product or system
- group related features into modules
- split a feature into children only when the parts are used on their own
- do not invent a different product than the story
- at most 80 feature nodes, depth at most 4
- ids: lowercase-kebab, unique across modules and features

STORY:
`)
	b.WriteString(strings.TrimSpace(p.Story))
	b.WriteString("\n\n")
	if s := p.Survey; s != nil {
		if s.What != "" && s.What != p.Story {
			fmt.Fprintf(&b, "Survey what: %s\n", s.What)
		}
		if len(s.Kinds) > 0 {
			fmt.Fprintf(&b, "Platforms: %s\n", strings.Join(s.Kinds, ", "))
		}
		if s.Users != "" {
			fmt.Fprintf(&b, "Users: %s\n", s.Users)
		}
		if s.Stage != "" {
			fmt.Fprintf(&b, "Delivery: %s\n", s.Stage)
		}
		if s.Constraints != "" {
			fmt.Fprintf(&b, "Constraints: %s\n", s.Constraints)
		}
		b.WriteString("\n")
	}
	req := featureMapRequest(p)
	action := core.FeatureMapGenerate
	if req != nil && req.Action != "" {
		action = req.Action
	}
	if !p.FeatureMap.Empty() && action != core.FeatureMapGenerate {
		b.WriteString("CURRENT FEATURE MAP:\n")
		b.WriteString(FeatureMapContext(p))
		b.WriteString("\n")
	}
	if req != nil && strings.TrimSpace(req.Text) != "" {
		fmt.Fprintf(&b, "USER NOTE:\n%s\n\n", strings.TrimSpace(req.Text))
	}
	if req != nil && req.Rejected != "" {
		fmt.Fprintf(&b, "YOUR LAST ANSWER WAS REFUSED: %s\nFix the JSON and try again.\n\n", req.Rejected)
	}
	switch action {
	case core.FeatureMapExpand:
		name, kind := "the selected item", "feature"
		if req != nil {
			if mod, node := p.FeatureMap.Locate(req.NodeID); node != nil {
				name, kind = node.Name, "feature"
			} else if mod != nil {
				name, kind = mod.Name, "module"
			}
		}
		fmt.Fprintf(&b, `ACTION: expand
EXPAND this %s: %q
Fill description, purpose, benefit, outcome. Add children only when they are real subfeatures.
Reply with ONLY one JSON object in a `+"`"+`json`+"`"+` block, shape:
{"node": {"name": "", "description": "", "purpose": "", "benefit": "", "outcome": "", "children": [{"name": "", "description": "", "purpose": "", "benefit": "", "outcome": ""}]}}
If expanding a module, put its features in:
{"modules": [{"name": %q, "features": [{"name": "", "description": "", "purpose": "", "benefit": "", "outcome": "", "children": []}]}]}
`, kind, name, name)
	case core.FeatureMapGaps:
		b.WriteString(`ACTION: gaps
Find modules or features the story needs that the CURRENT FEATURE MAP does not cover.
Reply with ONLY the additions (do not repeat existing names) as one JSON object in a ` + "`" + `json` + "`" + ` block, shape:
{"modules": [{"name": "", "features": [{"name": "", "description": "", "purpose": "", "benefit": "", "outcome": "", "children": [{"name": ""}]}]}]}
If nothing is missing, reply {"modules": []}.
`)
	default:
		b.WriteString(`ACTION: generate
Replace the map with a complete tree for THIS story.
Reply with ONLY one JSON object in a ` + "`" + `json` + "`" + ` block, shape:
{"summary": "one-line product summary",
 "modules": [{"name": "Trading", "features": [
   {"name": "Spot Trading", "description": "", "purpose": "", "benefit": "", "outcome": "",
    "children": [{"name": "Market Order", "description": "", "purpose": "", "benefit": "", "outcome": ""}]}
 ]}]}
`)
	}
	return b.String()
}

// FeatureMapContext is the map as a compact outline for other prompts.
func FeatureMapContext(p *core.Project) string {
	if p == nil || p.FeatureMap.Empty() {
		return ""
	}
	var b strings.Builder
	b.WriteString("FEATURE MAP (user-reviewed product features; use as the reference for requirements, features and flows; do not invent a different product):\n")
	if p.FeatureMap.Summary != "" {
		fmt.Fprintf(&b, "Summary: %s\n", p.FeatureMap.Summary)
	}
	for _, mod := range p.FeatureMap.Modules {
		fmt.Fprintf(&b, "- Module: %s\n", mod.Name)
		writeNodes(&b, mod.Features, 2)
	}
	return b.String()
}

func writeNodes(b *strings.Builder, nodes []core.FeatureNode, indent int) {
	pad := strings.Repeat("  ", indent)
	for _, n := range nodes {
		fmt.Fprintf(b, "%s- %s", pad, n.Name)
		if n.Description != "" {
			fmt.Fprintf(b, ": %s", n.Description)
		}
		b.WriteByte('\n')
		if len(n.Children) > 0 {
			writeNodes(b, n.Children, indent+1)
		}
	}
}

func featureMapRequest(p *core.Project) *core.FeatureMapRequest {
	if p == nil || p.FeatureMap == nil {
		return nil
	}
	return p.FeatureMap.Request
}

// ApplyFeatureMap lands a generate, expand, or gaps answer on the project.
func ApplyFeatureMap(p *core.Project, raw string) error {
	var out featureMapOut
	if err := ExtractJSONFunc(raw, &out, func() bool {
		return len(out.Modules) > 0 || out.Node != nil || out.Summary != ""
	}); err != nil {
		// An empty gaps answer is valid JSON {"modules":[]}; ExtractJSONFunc
		// may skip it. Accept a bare object with modules (even empty).
		var loose featureMapOut
		if ExtractJSON(raw, &loose) != nil {
			return err
		}
		out = loose
	}
	incoming := &core.FeatureMap{Summary: out.Summary, Modules: out.Modules}
	incoming.Normalize()
	action := core.FeatureMapGenerate
	if req := featureMapRequest(p); req != nil && req.Action != "" {
		action = req.Action
	}
	switch action {
	case core.FeatureMapExpand:
		if err := applyExpand(p, incoming, out.Node); err != nil {
			return err
		}
	case core.FeatureMapGaps:
		applyGaps(p, incoming)
	default:
		if incoming.Empty() {
			return fmt.Errorf("daftar fitur kosong")
		}
		req := featureMapRequest(p)
		p.FeatureMap = incoming
		p.FeatureMap.Request = req
	}
	p.FeatureMap.Normalize()
	if err := p.FeatureMap.Validate(); err != nil {
		return err
	}
	p.FeatureMap.Request = nil
	return nil
}

func applyExpand(p *core.Project, incoming *core.FeatureMap, node *core.FeatureNode) error {
	if p.FeatureMap == nil {
		p.FeatureMap = &core.FeatureMap{}
	}
	req := featureMapRequest(p)
	id := ""
	if req != nil {
		id = req.NodeID
	}
	mod, cur := p.FeatureMap.Locate(id)
	if cur != nil {
		src := node
		if src == nil {
			src = firstNode(incoming)
		}
		if src == nil {
			return fmt.Errorf("jawaban expand tidak berisi fitur")
		}
		mergeNode(cur, *src)
		return nil
	}
	if mod != nil {
		for i := range incoming.Modules {
			if eqName(incoming.Modules[i].Name, mod.Name) || incoming.Modules[i].ID == mod.ID {
				mod.Features = mergeFeatureList(mod.Features, incoming.Modules[i].Features)
				return nil
			}
		}
		if node != nil {
			mod.Features = mergeFeatureList(mod.Features, []core.FeatureNode{*node})
			return nil
		}
		if n := firstNode(incoming); n != nil {
			mod.Features = mergeFeatureList(mod.Features, []core.FeatureNode{*n})
			return nil
		}
		return fmt.Errorf("jawaban expand tidak berisi fitur untuk modul %s", mod.Name)
	}
	return fmt.Errorf("fitur tidak ditemukan")
}

func applyGaps(p *core.Project, incoming *core.FeatureMap) {
	if incoming.Empty() {
		if p.FeatureMap == nil {
			p.FeatureMap = &core.FeatureMap{}
		}
		return
	}
	if p.FeatureMap.Empty() {
		p.FeatureMap = incoming
		return
	}
	if incoming.Summary != "" && p.FeatureMap.Summary == "" {
		p.FeatureMap.Summary = incoming.Summary
	}
	for _, in := range incoming.Modules {
		found := false
		for i := range p.FeatureMap.Modules {
			if eqName(p.FeatureMap.Modules[i].Name, in.Name) {
				p.FeatureMap.Modules[i].Features = mergeFeatureList(p.FeatureMap.Modules[i].Features, in.Features)
				found = true
				break
			}
		}
		if !found {
			p.FeatureMap.Modules = append(p.FeatureMap.Modules, in)
		}
	}
}

func firstNode(m *core.FeatureMap) *core.FeatureNode {
	if m == nil {
		return nil
	}
	for i := range m.Modules {
		if len(m.Modules[i].Features) > 0 {
			return &m.Modules[i].Features[0]
		}
	}
	return nil
}

func mergeNode(dst *core.FeatureNode, src core.FeatureNode) {
	if src.Name != "" {
		dst.Name = src.Name
	}
	if src.Description != "" {
		dst.Description = src.Description
	}
	if src.Purpose != "" {
		dst.Purpose = src.Purpose
	}
	if src.Benefit != "" {
		dst.Benefit = src.Benefit
	}
	if src.Outcome != "" {
		dst.Outcome = src.Outcome
	}
	if len(src.Children) > 0 {
		dst.Children = mergeFeatureList(dst.Children, src.Children)
	}
}

func mergeFeatureList(dst, src []core.FeatureNode) []core.FeatureNode {
	for _, in := range src {
		found := false
		for i := range dst {
			if eqName(dst[i].Name, in.Name) {
				mergeNode(&dst[i], in)
				found = true
				break
			}
		}
		if !found {
			dst = append(dst, in)
		}
	}
	return dst
}

func eqName(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}
