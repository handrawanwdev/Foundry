package core

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// legacyState is what older project files kept for the visual design
// workspace (Design IR pages, design system). Only the fields that carry
// over into the design rules and layout spec are read.
type legacyState struct {
	Design *struct {
		System *struct {
			Direction   string  `json:"direction"`
			Colors      []Token `json:"colors"`
			Typography  []Token `json:"typography"`
			Spacing     []Token `json:"spacing"`
			Radius      []Token `json:"radius"`
			Shadow      []Token `json:"shadow"`
			Breakpoints []Token `json:"breakpoints"`
			Components  []Token `json:"components"`
		} `json:"system"`
		Pages       []legacyPage `json:"pages"`
		TechVersion int          `json:"tech_version"`
	} `json:"design"`
}

type legacyPage struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Platform         string            `json:"platform"`
	Role             string            `json:"role"`
	FeatureID        string            `json:"feature_id"`
	Purpose          string            `json:"purpose"`
	Kind             string            `json:"kind"`
	Route            string            `json:"route"`
	Access           string            `json:"access"`
	Responsive       string            `json:"responsive"`
	SecondaryActions []string          `json:"secondary_actions"`
	States           map[string]string `json:"states"`
	PrimaryAction    *struct {
		Label string `json:"label"`
	} `json:"primary_action"`
	Data *struct {
		Required []string `json:"required"`
		Optional []string `json:"optional"`
		Dynamic  []string `json:"dynamic"`
		API      []string `json:"api"`
	} `json:"data"`
	Components []struct {
		ID      string   `json:"id"`
		Type    string   `json:"type"`
		Label   string   `json:"label"`
		Items   []string `json:"items"`
		Columns []string `json:"columns"`
		Target  string   `json:"target"`
	} `json:"components"`
}

// legacyPattern maps the page kinds of the Design IR onto layout patterns.
var legacyPattern = map[string]string{
	"auth": "auth", "dashboard": "dashboard", "list": "list", "detail": "detail", "form": "form", "wizard": "wizard",
	"search": "search", "settings": "settings", "payment": "form", "profile": "detail", "analytics": "dashboard",
	"admin": "table", "status": "detail", "table": "table", "workspace": "editor", "master-detail": "master-detail",
	"monitoring": "dashboard", "report": "report",
}

// migrateLegacyDesign turns the design of an older project file into the
// design rules and layout spec, once: what the old design did not record
// (principles, accessibility, ...) is left empty for the user or the AI to
// complete, never invented. Commands that referred to the old ui-design
// artifact now refer to both new artifacts and are reported for review.
func migrateLegacyDesign(p *Project, raw []byte) {
	if p.UI != nil {
		return
	}
	var old legacyState
	if json.Unmarshal(raw, &old) != nil || old.Design == nil || old.Design.System == nil && len(old.Design.Pages) == 0 {
		return
	}
	d := &UIDesign{TechVersion: old.Design.TechVersion, Version: 1}
	if s := old.Design.System; s != nil {
		r := &DesignRules{Source: RulesNew, Direction: s.Direction, Tokens: DesignTokens{Colors: s.Colors, Typography: s.Typography,
			Spacing: s.Spacing, Radius: s.Radius, Shadow: s.Shadow, Breakpoints: s.Breakpoints}}
		if p.Structure != nil && p.Structure.Mode == StructureExisting {
			r.Source = RulesExisting
		}
		for _, c := range s.Components {
			r.Components = append(r.Components, ComponentRule{Name: c.Name, Use: strings.TrimSpace(c.Use + " " + c.Value)})
		}
		d.Rules = r
	}
	if len(old.Design.Pages) > 0 {
		ids := map[string]string{}
		for _, pg := range old.Design.Pages {
			ids[pg.ID] = strings.ToLower(pg.ID)
		}
		l := &LayoutSpec{}
		for _, pg := range old.Design.Pages {
			l.Pages = append(l.Pages, legacyLayoutPage(pg, ids))
		}
		d.Layout = l
		for i := range p.Flows {
			for j, id := range p.Flows[i].PageIDs {
				if n, ok := ids[id]; ok {
					p.Flows[i].PageIDs[j] = n
				}
			}
		}
	}
	p.UI = d
	if w := p.Workflow; w != nil {
		for i := range w.Commands {
			c := &w.Commands[i]
			if j := slices.IndexFunc(c.Artifacts, func(r ArtifactRef) bool { return r.ID == "ui-design" }); j >= 0 {
				c.Artifacts = slices.Delete(c.Artifacts, j, j+1)
				c.Artifacts = append(c.Artifacts, ArtifactRef{ID: ArtifactDesignRules}, ArtifactRef{ID: ArtifactLayoutSpec})
			}
		}
	}
	p.Log("system", fmt.Sprintf("Desain visual lama dimigrasi ke aturan desain dan spesifikasi layout (%d halaman); lengkapi yang belum ada", len(old.Design.Pages)))
}

func legacyLayoutPage(pg legacyPage, ids map[string]string) LayoutPage {
	out := LayoutPage{ID: ids[pg.ID], Name: pg.Name, Platform: pg.Platform, Route: pg.Route, Role: pg.Role,
		Access: strings.ToLower(pg.Access), FeatureID: pg.FeatureID, Purpose: pg.Purpose, Pattern: legacyPattern[strings.ToLower(pg.Kind)]}
	if out.Pattern == "" {
		out.Pattern = "custom"
	}
	if pg.PrimaryAction != nil {
		out.PrimaryAction = pg.PrimaryAction.Label
	}
	if pg.Responsive != "" {
		out.Responsive = []string{pg.Responsive}
	}
	if len(pg.States) > 0 {
		out.States = map[string]string{}
		for k, v := range pg.States {
			if v != "" {
				out.States[k] = v
			}
		}
	}
	if d := pg.Data; d != nil {
		out.Data = slices.Concat(d.Required, d.Optional, d.Dynamic, d.API)
	}
	for i, c := range pg.Components {
		id := strings.ToLower(c.ID)
		if !ValidID(id) {
			id = fmt.Sprintf("section-%d", i+1)
		}
		content := []string{strings.TrimSpace(c.Type + ": " + c.Label)}
		content = append(content, c.Columns...)
		content = append(content, c.Items...)
		out.Sections = append(out.Sections, Section{ID: id, Name: cmpOr(c.Label, c.Type), Content: content})
		if n, ok := ids[c.Target]; ok && n != out.ID && !slices.ContainsFunc(out.Links, func(l PageLink) bool { return l.To == n }) {
			out.Links = append(out.Links, PageLink{To: n, Via: cmpOr(c.Label, c.Type)})
		}
	}
	return out
}

func cmpOr(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

// migrateArtifactIDs renames written artifacts whose id became a built-in
// one (use-cases, acceptance, decisions) to "<id>-doc"; refs of artifacts
// and commands follow. The content and its hash stay the same, so no
// command is reported stale by the rename.
func migrateArtifactIDs(p *Project) {
	rename := map[string]string{}
	for i, a := range p.Artifacts {
		if !BuiltinArtifacts[a.ID] {
			continue
		}
		id := a.ID + "-doc"
		for n := 2; slices.ContainsFunc(p.Artifacts, func(x Artifact) bool { return x.ID == id }); n++ {
			id = fmt.Sprintf("%s-doc-%d", a.ID, n)
		}
		rename[a.ID] = id
		p.Artifacts[i].ID = id
	}
	if len(rename) == 0 {
		return
	}
	for i := range p.Artifacts {
		for j, r := range p.Artifacts[i].Refs {
			if to, ok := rename[r]; ok {
				p.Artifacts[i].Refs[j] = to
			}
		}
	}
	if w := p.Workflow; w != nil {
		for i := range w.Commands {
			for j, r := range w.Commands[i].Artifacts {
				if to, ok := rename[r.ID]; ok {
					w.Commands[i].Artifacts[j].ID = to
				}
			}
		}
	}
	froms := make([]string, 0, len(rename))
	for from := range rename {
		froms = append(froms, from)
	}
	slices.Sort(froms)
	for _, from := range froms {
		to := rename[from]
		p.Log("system", fmt.Sprintf("Artefak %s diganti nama menjadi %s (id %s kini artefak bawaan)", from, to, from))
	}
}
