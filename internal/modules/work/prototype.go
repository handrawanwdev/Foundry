package work

import (
	"fmt"
	"slices"

	"github.com/handrawanw/foundry/internal/core"
)

// Prototype layer: the navigation the prototype runs on, derived from the
// Design IR only (component targets, nav items, the back link and the
// primary action). The same links are the build's navigation contract, so
// prototype and production cannot disagree.

type ProtoLink struct {
	To   string `json:"to"`
	Via  string `json:"via"`  // what the user clicks
	Kind string `json:"kind"` // target | nav | back | primary
}

type ProtoPage struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Role     string      `json:"role,omitempty"`
	Platform string      `json:"platform,omitempty"`
	Route    string      `json:"route,omitempty"`
	Entry    bool        `json:"entry,omitempty"`
	Links    []ProtoLink `json:"links"`
}

// ProtoFlow is a user flow walked on the prototype. Gaps are consecutive
// steps with no link from one page to the next.
type ProtoFlow struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Steps []string `json:"steps"`
	Gaps  []string `json:"gaps,omitempty"`
}

type Prototype struct {
	Pages       []ProtoPage `json:"pages"`
	Flows       []ProtoFlow `json:"flows"`
	Unreachable []string    `json:"unreachable,omitempty"` // no entry page leads here
	DeadEnds    []string    `json:"dead_ends,omitempty"`   // no way out
}

// BuildPrototype derives the prototype navigation of the current design.
func BuildPrototype(p *core.Project) Prototype {
	var out Prototype
	if p.Design == nil {
		return out
	}
	pages := p.Design.Pages
	byID := map[string]bool{}
	for _, pg := range pages {
		byID[pg.ID] = true
	}
	entries := map[string]bool{}
	for _, fl := range p.Flows {
		for _, id := range fl.PageIDs {
			if byID[id] {
				entries[id] = true
				break
			}
		}
	}
	for _, pg := range pages {
		if core.PageKind(pg) == "auth" {
			entries[pg.ID] = true
		}
	}
	links := map[string][]ProtoLink{}
	for _, pg := range pages {
		links[pg.ID] = pageLinks(pg, pages, byID)
	}
	// Without a flow or a sign-in page, every page nothing links to starts.
	if len(entries) == 0 {
		in := map[string]bool{}
		for _, ls := range links {
			for _, l := range ls {
				in[l.To] = true
			}
		}
		for _, pg := range pages {
			if !in[pg.ID] {
				entries[pg.ID] = true
			}
		}
		if len(entries) == 0 && len(pages) > 0 {
			entries[pages[0].ID] = true
		}
	}

	seen := map[string]bool{}
	var queue []string
	for _, pg := range pages {
		if entries[pg.ID] {
			seen[pg.ID] = true
			queue = append(queue, pg.ID)
		}
	}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		for _, l := range links[id] {
			if !seen[l.To] {
				seen[l.To] = true
				queue = append(queue, l.To)
			}
		}
	}
	for _, pg := range pages {
		out.Pages = append(out.Pages, ProtoPage{ID: pg.ID, Name: pg.Name, Role: pg.Role, Platform: pg.Platform, Route: pg.Route,
			Entry: entries[pg.ID], Links: links[pg.ID]})
		if !seen[pg.ID] {
			out.Unreachable = append(out.Unreachable, pg.ID)
		}
		if len(links[pg.ID]) == 0 {
			out.DeadEnds = append(out.DeadEnds, pg.ID)
		}
	}
	for _, fl := range p.Flows {
		f := ProtoFlow{ID: fl.ID, Name: fl.Name}
		for _, id := range fl.PageIDs {
			if byID[id] {
				f.Steps = append(f.Steps, id)
			}
		}
		if len(f.Steps) == 0 {
			continue
		}
		for k := 1; k < len(f.Steps); k++ {
			from, to := f.Steps[k-1], f.Steps[k]
			if !slices.ContainsFunc(links[from], func(l ProtoLink) bool { return l.To == to }) {
				f.Gaps = append(f.Gaps, fmt.Sprintf("%s → %s", from, to))
			}
		}
		out.Flows = append(out.Flows, f)
	}
	return out
}

// pageLinks lists the distinct pages one page opens.
func pageLinks(pg core.Page, pages []core.Page, byID map[string]bool) []ProtoLink {
	out := []ProtoLink{}
	add := func(to, via, kind string) {
		if to == "" || to == pg.ID || !byID[to] {
			return
		}
		if slices.ContainsFunc(out, func(l ProtoLink) bool { return l.To == to }) {
			return
		}
		out = append(out, ProtoLink{To: to, Via: via, Kind: kind})
	}
	if a := pg.PrimaryAction; a != nil {
		add(a.Target, a.Label, "primary")
	}
	for _, c := range pg.Components {
		if c.Type == "nav" {
			for _, it := range c.Items {
				if t := navTarget(it, pg, pages); t != nil {
					add(t.ID, it, "nav")
				}
			}
			continue
		}
		add(c.Target, componentName(c), "target")
	}
	if pg.Layout != nil {
		add(pg.Layout.Back, "Kembali", "back")
	}
	return out
}
