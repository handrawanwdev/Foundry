package core

import (
	"strings"
)

// TraceLink is one requirement followed through the plan to a command.
// Empty fields mean that hop was not found; they are not invented.
type TraceLink struct {
	RequirementID string   `json:"requirement_id"`
	FeatureID     string   `json:"feature_id,omitempty"`
	Domain        string   `json:"domain,omitempty"`
	Module        string   `json:"module,omitempty"`
	Files         []string `json:"files,omitempty"`
	Acceptance    []string `json:"acceptance,omitempty"`
	Tests         []string `json:"tests,omitempty"`
	Commands      []string `json:"commands,omitempty"`
}

// Traceability follows Requirement → Feature → Domain → Module → File
// delta → Acceptance → Test plan → Command. A hop is filled only from
// recorded project data.
func Traceability(p *Project) []TraceLink {
	if p == nil || len(p.Requirements) == 0 {
		return nil
	}
	featOf := map[string][]Feature{}
	for _, f := range p.Features {
		for _, r := range f.RequirementIDs {
			featOf[r] = append(featOf[r], f)
		}
	}
	domainOf := featureDomains(p)
	moduleOf := featureModules(p)
	filesOf := moduleFiles(p)
	cmdOf := requirementCommands(p)
	var out []TraceLink
	for _, r := range p.Requirements {
		feats := featOf[r.ID]
		if len(feats) == 0 {
			out = append(out, TraceLink{RequirementID: r.ID, Acceptance: criterionIDs(r)})
			continue
		}
		for _, f := range feats {
			link := TraceLink{
				RequirementID: r.ID,
				FeatureID:     f.ID,
				Domain:        domainOf[f.ID],
				Module:        moduleOf[f.ID],
				Acceptance:    criterionIDs(r),
				Commands:      cmdOf[r.ID],
			}
			if link.Module != "" {
				link.Files = filesOf[strings.ToLower(link.Module)]
			}
			link.Tests = commandTests(p, link.Commands)
			out = append(out, link)
		}
	}
	return out
}

func criterionIDs(r Requirement) []string {
	var out []string
	for _, c := range r.Acceptance {
		if c.ID != "" {
			out = append(out, c.ID)
		}
	}
	return out
}

func featureDomains(p *Project) map[string]string {
	out := map[string]string{}
	if p.FeatureMap == nil {
		return out
	}
	p.FeatureMap.Walk(func(n *FeatureNode, mod *FeatureModule, _ int) {
		if n.FeatureID != "" && mod != nil && out[n.FeatureID] == "" {
			out[n.FeatureID] = mod.Name
		}
	})
	return out
}

func featureModules(p *Project) map[string]string {
	out := map[string]string{}
	if p.Change != nil {
		for _, it := range p.Change.Items {
			if it.Module == "" {
				continue
			}
			key := strings.ToLower(strings.TrimSpace(it.Target))
			for _, f := range p.Features {
				if strings.ToLower(f.ID) == key || strings.ToLower(f.Name) == key {
					out[f.ID] = it.Module
				}
			}
			if it.Action == ChangeAdd || it.Action == ChangeModify || it.Action == ChangeReuse {
				for _, f := range p.Features {
					if out[f.ID] == "" && strings.Contains(strings.ToLower(f.Name), strings.ToLower(it.Target)) {
						out[f.ID] = it.Module
					}
				}
			}
		}
	}
	if p.Blueprint == nil {
		return out
	}
	for _, f := range p.Features {
		if out[f.ID] != "" {
			continue
		}
		name := strings.ToLower(f.Name)
		for _, m := range p.Blueprint.Modules {
			id := strings.ToLower(m.ID)
			if id == name || strings.Contains(strings.ToLower(m.Responsibility), name) {
				out[f.ID] = m.ID
				break
			}
			for _, d := range m.Data {
				if strings.Contains(name, strings.ToLower(d)) {
					out[f.ID] = m.ID
					break
				}
			}
		}
	}
	return out
}

func moduleFiles(p *Project) map[string][]string {
	out := map[string][]string{}
	if p.Change == nil {
		return out
	}
	add := func(mod, path string) {
		mod = strings.ToLower(strings.TrimSpace(mod))
		path = strings.TrimSpace(path)
		if mod == "" || path == "" {
			return
		}
		for _, e := range out[mod] {
			if e == path {
				return
			}
		}
		out[mod] = append(out[mod], path)
	}
	for _, it := range p.Change.Items {
		for _, f := range it.Files {
			add(it.Module, f)
		}
	}
	for _, f := range p.Change.Files.Add {
		add(moduleOfPath(p, f.Path), f.Path)
	}
	for _, f := range p.Change.Files.Modify {
		add(moduleOfPath(p, f.Path), f.Path)
	}
	for _, f := range p.Change.Files.Remove {
		add(moduleOfPath(p, f.Path), f.Path)
	}
	return out
}

func moduleOfPath(p *Project, path string) string {
	path = strings.ToLower(strings.TrimSpace(path))
	best := ""
	if p.Structure != nil && p.Structure.Context != nil {
		for _, m := range p.Structure.Context.Modules {
			mp := strings.ToLower(m.Path)
			if mp != "" && strings.HasPrefix(path, mp) && len(mp) >= len(best) {
				best = m.Name
			}
		}
	}
	if best != "" {
		return best
	}
	if p.Blueprint != nil {
		for _, m := range p.Blueprint.Modules {
			if strings.Contains(path, strings.ToLower(m.ID)) {
				return m.ID
			}
		}
	}
	return ""
}

func requirementCommands(p *Project) map[string][]string {
	out := map[string][]string{}
	if p.Workflow == nil {
		return out
	}
	for _, c := range p.Workflow.Commands {
		for _, id := range c.Covers {
			id = strings.ToUpper(strings.TrimSpace(id))
			if id == "" {
				continue
			}
			out[id] = append(out[id], c.ID)
		}
	}
	return out
}

func commandTests(p *Project, cmds []string) []string {
	if p.Workflow == nil || len(cmds) == 0 {
		return nil
	}
	want := map[string]bool{}
	for _, id := range cmds {
		want[id] = true
	}
	var out []string
	seen := map[string]bool{}
	for _, c := range p.Workflow.Commands {
		if !want[c.ID] {
			continue
		}
		for _, s := range append(append([]string{}, c.Outputs...), c.Acceptance...) {
			ls := strings.ToLower(s)
			if !strings.Contains(ls, "test") && !strings.Contains(ls, "tes ") {
				continue
			}
			if seen[s] {
				continue
			}
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
