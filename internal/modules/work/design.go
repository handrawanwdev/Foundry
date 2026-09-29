package work

import (
	"fmt"
	"strings"

	"github.com/handrawanw/foundry/internal/core"
)

type designOut struct {
	Pages []core.Page `json:"pages"`
	Flows []struct {
		ID      string   `json:"id"`
		PageIDs []string `json:"page_ids"`
	} `json:"flows"`
}

var componentTypes = map[string]bool{
	"header": true, "nav": true, "text": true, "list": true, "table": true, "form": true,
	"button": true, "card": true, "stat": true, "input": true,
}

func DesignPrompt(p *core.Project) string {
	var b strings.Builder
	b.WriteString(`You are the product designer of Foundry. Create the UI pages for this product as simple wireframes.
Pages MUST come from the features, sub-features and user flows below. Do not invent features. Backend-only features need no page.
Write labels in the same language as the features. Do not write code. Do not modify files.

`)
	b.WriteString("PLATFORMS:\n" + mustJSON(p.Platform.Targets) + "\n\n")
	b.WriteString("FEATURES:\n" + mustJSON(p.Features) + "\n\n")
	b.WriteString("FLOWS:\n" + mustJSON(p.Flows) + "\n\n")
	if p.Discovery != nil {
		b.WriteString("ROLES:\n" + mustJSON(p.Discovery.Roles) + "\n\n")
	}
	if p.Design != nil && len(p.Design.Feedback) > 0 {
		b.WriteString("PREVIOUS DESIGN:\n" + mustJSON(p.Design.Pages) + "\n\n")
		b.WriteString("USER REQUESTED CHANGES (apply all):\n- " + strings.Join(p.Design.Feedback, "\n- ") + "\n\n")
	}
	b.WriteString(`Component types: header, nav, text, list, table, form, button, card, stat, input.
Use "target" on button and card components to link to another page id so the prototype is clickable. list/table/nav/form use "items".
Every UI feature needs at least one page. Every flow lists the pages it walks through, in order.

Reply with ONLY one JSON object in a ` + "```json" + ` block, shape:
{
  "pages": [{"id": "PAGE-001", "name": "", "platform": "web|android|ios|desktop", "role": "", "feature_id": "FEAT-001",
             "sub_feature_ids": ["FEAT-001.1"], "purpose": "",
             "components": [{"type": "header", "label": ""}, {"type": "button", "label": "", "target": "PAGE-002"}]}],
  "flows": [{"id": "FLOW-001", "page_ids": ["PAGE-001", "PAGE-002"]}]
}`)
	return b.String()
}

// ApplyDesign replaces pages with a new draft; approval is reset.
func ApplyDesign(p *core.Project, raw string) error {
	var out designOut
	if err := ExtractJSON(raw, &out); err != nil {
		return err
	}
	if len(out.Pages) == 0 {
		return fmt.Errorf("rancangan tidak berisi page")
	}
	idMap := map[string]string{}
	for i, pg := range out.Pages {
		idMap[pg.ID] = fmt.Sprintf("PAGE-%03d", i+1)
	}
	var pages []core.Page
	for _, pg := range out.Pages {
		if p.Feature(pg.FeatureID) == nil {
			continue // design must trace to a feature
		}
		np := core.Page{ID: idMap[pg.ID], Name: pg.Name, Platform: strings.ToLower(pg.Platform), Role: pg.Role,
			FeatureID: pg.FeatureID, Purpose: pg.Purpose}
		f := p.Feature(pg.FeatureID)
		for _, sid := range pg.SubFeatureIDs {
			for _, s := range f.SubFeatures {
				if s.ID == sid {
					np.SubFeatureIDs = append(np.SubFeatureIDs, sid)
				}
			}
		}
		for _, c := range pg.Components {
			c.Type = strings.ToLower(c.Type)
			if !componentTypes[c.Type] {
				c.Type = "text"
			}
			if c.Target != "" {
				c.Target = idMap[c.Target] // unknown targets become "" (dead link removed)
			}
			np.Components = append(np.Components, c)
		}
		pages = append(pages, np)
	}
	if len(pages) == 0 {
		return fmt.Errorf("tidak ada page yang terhubung ke feature")
	}
	// Pages that were dropped leave dangling targets; clear them.
	valid := map[string]bool{}
	for _, pg := range pages {
		valid[pg.ID] = true
	}
	for i := range pages {
		for j := range pages[i].Components {
			if !valid[pages[i].Components[j].Target] {
				pages[i].Components[j].Target = ""
			}
		}
	}
	for i := range p.Flows {
		p.Flows[i].PageIDs = nil
		for _, fl := range out.Flows {
			if fl.ID != p.Flows[i].ID {
				continue
			}
			for _, id := range fl.PageIDs {
				if m := idMap[id]; valid[m] {
					p.Flows[i].PageIDs = append(p.Flows[i].PageIDs, m)
				}
			}
		}
	}
	var feedback []string
	if p.Design != nil {
		feedback = p.Design.Feedback
	}
	p.Design = &core.Design{Pages: pages, Status: core.DesignDraft, Feedback: feedback}
	p.Tasks, p.Verification = nil, nil
	return nil
}

// UIFeatures returns features that must have at least one page.
func UIFeatures(p *core.Project) []core.Feature {
	var out []core.Feature
	for _, f := range p.Features {
		for _, pl := range f.Platforms {
			if pl != core.PlatformBackend {
				out = append(out, f)
				break
			}
		}
	}
	return out
}
