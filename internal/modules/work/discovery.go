// Package work defines every AI work item Foundry hands out: the prompt
// (what the AI must do) and Apply (how the answer lands in the single project
// context). Agent mode and MCP mode share these functions, so both paths
// produce identical state.
package work

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/handrawanw/foundry/internal/core"
)

const (
	KindDiscovery = "discovery"
	KindDesign    = "design"
	KindPlan      = "plan"
	KindTask      = "task"
	KindReview    = "review"
)

type discoveryOut struct {
	Name         string             `json:"name"`
	Discovery    core.Discovery     `json:"discovery"`
	Requirements []core.Requirement `json:"requirements"`
	Features     []core.Feature     `json:"features"`
	Flows        []core.Flow        `json:"flows"`
	Platform     core.PlatformPlan  `json:"platform"`
}

func DiscoveryPrompt(p *core.Project) string {
	var b strings.Builder
	b.WriteString(`You are the discovery analyst of Foundry. Understand the product idea below BEFORE anything is built.
Analyze: business, users, roles, features, sub-features, workflows, data, integrations, security, platform and device needs.
Write every human-readable text in the same language as the story. Do not write code. Do not modify files.

STORY:
`)
	b.WriteString(p.Story)
	b.WriteString("\n\n")
	if p.Structure != nil && p.Structure.Mode == core.StructureExisting {
		b.WriteString("EXISTING PROJECT COMPONENTS (respect this architecture):\n")
		for _, c := range p.Structure.Components {
			fmt.Fprintf(&b, "- %s at %s (%s) evidence: %s\n", c.Kind, c.Path, c.Tech, c.Evidence)
		}
		b.WriteString("\n")
	}
	if p.Design != nil && len(p.Design.Feedback) > 0 {
		b.WriteString("USER CORRECTIONS (must be applied):\n- " + strings.Join(p.Design.Feedback, "\n- ") + "\n\n")
	}
	b.WriteString(`Platform kinds allowed: web, android, ios, desktop, backend. Choose them from who uses the product and on which device.
tech_needs lists only the technical needs the product really has, from: realtime, offline, seo, ai, high_concurrency,
flexible_schema, transactions, independent_scaling, enterprise. Foundry picks the technology stack from them.
Every requirement must be covered by at least one feature. Every feature needs 2-6 concrete sub-features.

Reply with ONLY one JSON object in a ` + "```json" + ` block, shape:
{
  "name": "short product name",
  "discovery": {"summary": "", "business": "", "users": [""], "roles": [{"name": "", "description": "", "platform": "web"}],
                "data": ["entity: key fields"], "integrations": [""], "security": [""], "workflows": [""],
                "tech_needs": ["transactions"]},
  "requirements": [{"id": "REQ-001", "text": "", "priority": "MUST|SHOULD|COULD"}],
  "features": [{"id": "FEAT-001", "name": "", "description": "", "requirement_ids": ["REQ-001"], "platforms": ["web","backend"],
                "sub_features": [{"id": "FEAT-001.1", "name": ""}]}],
  "flows": [{"id": "FLOW-001", "name": "", "feature_id": "FEAT-001", "role": "", "steps": [""]}],
  "platform": {"targets": [{"kind": "web", "audience": ["Admin"], "rationale": ""}], "reason": "one short sentence for a non-technical user"}
}`)
	return b.String()
}

var validPlatforms = map[string]bool{
	core.PlatformWeb: true, core.PlatformAndroid: true, core.PlatformIOS: true,
	core.PlatformDesktop: true, core.PlatformBackend: true,
}

// ApplyDiscovery validates the AI answer, renumbers IDs and writes the
// discovery layers. Downstream artifacts (design, tasks) are reset because
// they derive from these features.
func ApplyDiscovery(p *core.Project, raw string) error {
	var out discoveryOut
	if err := ExtractJSON(raw, &out); err != nil {
		return err
	}
	if len(out.Requirements) == 0 || len(out.Features) == 0 {
		return fmt.Errorf("discovery tidak lengkap: requirements/features kosong")
	}
	reqMap := map[string]string{}
	var reqs []core.Requirement
	for i, r := range out.Requirements {
		if strings.TrimSpace(r.Text) == "" {
			continue
		}
		id := fmt.Sprintf("REQ-%03d", i+1)
		reqMap[r.ID] = id
		pr := strings.ToUpper(r.Priority)
		if pr != "MUST" && pr != "SHOULD" && pr != "COULD" {
			pr = "MUST"
		}
		reqs = append(reqs, core.Requirement{ID: id, Text: r.Text, Priority: pr})
	}
	featMap := map[string]string{}
	var feats []core.Feature
	for i, f := range out.Features {
		if strings.TrimSpace(f.Name) == "" {
			continue
		}
		id := fmt.Sprintf("FEAT-%03d", i+1)
		featMap[f.ID] = id
		nf := core.Feature{ID: id, Name: f.Name, Description: f.Description}
		for _, r := range f.RequirementIDs {
			if m, ok := reqMap[r]; ok {
				nf.RequirementIDs = append(nf.RequirementIDs, m)
			}
		}
		for _, pl := range f.Platforms {
			if pl = strings.ToLower(pl); validPlatforms[pl] {
				nf.Platforms = append(nf.Platforms, pl)
			}
		}
		for j, s := range f.SubFeatures {
			nf.SubFeatures = append(nf.SubFeatures, core.SubFeature{ID: fmt.Sprintf("%s.%d", id, j+1), Name: s.Name})
		}
		feats = append(feats, nf)
	}
	// Every requirement must reach a feature, otherwise it can never be built.
	covered := map[string]bool{}
	for _, f := range feats {
		for _, r := range f.RequirementIDs {
			covered[r] = true
		}
	}
	for _, r := range reqs {
		if !covered[r.ID] {
			return fmt.Errorf("%s (%s) tidak terhubung ke feature mana pun", r.ID, r.Text)
		}
	}
	var flows []core.Flow
	for _, fl := range out.Flows {
		fid, ok := featMap[fl.FeatureID]
		if !ok || len(fl.Steps) == 0 {
			continue
		}
		flows = append(flows, core.Flow{ID: fmt.Sprintf("FLOW-%03d", len(flows)+1), Name: fl.Name, FeatureID: fid, Role: fl.Role, Steps: fl.Steps})
	}
	var targets []core.PlatformTarget
	for _, t := range out.Platform.Targets {
		if t.Kind = strings.ToLower(t.Kind); validPlatforms[t.Kind] {
			targets = append(targets, t)
		}
	}
	if len(targets) == 0 {
		return fmt.Errorf("platform tidak ditentukan")
	}

	if out.Name != "" {
		p.Name = out.Name
	}
	d := out.Discovery
	p.Discovery = &d
	p.Requirements, p.Features, p.Flows = reqs, feats, flows
	p.Platform = &core.PlatformPlan{Targets: targets, Reason: out.Platform.Reason}
	p.Design, p.Tasks, p.Verification = nil, nil, nil
	return nil
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v) // compact: prompts are paid per token
	return string(b)
}
