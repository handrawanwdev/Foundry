package work

import (
	"slices"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/uikit"
)

// The UI kit turns the designer's choices into a concrete design: the
// preset becomes every token, sections land in the slots of their page
// template, and kit components the pages reference get their rules.

// kitRules settles the preset of rules the AI wrote: an unknown preset or
// none on a new design falls back to the default preset, an invalid brand
// color is dropped, and the tokens are materialized from the preset. An
// existing project keeps its own design system: the preset only fills the
// mockup, its tokens are not added to the rules.
func kitRules(p *core.Project, r *core.DesignRules) *core.DesignRules {
	if r == nil {
		return nil
	}
	k := uikit.Default()
	if r.Preset != "" {
		if _, ok := k.Preset(r.Preset); !ok {
			r.Preset = ""
		}
	}
	existing := r.Source == core.RulesExisting || p != nil && p.Structure != nil && p.Structure.Mode == core.StructureExisting
	if existing {
		return r
	}
	if r.Preset == "" {
		r.Preset = uikit.DefaultPreset
	}
	if b := r.Brand; b != nil && b.Primary != "" && !hexColor(b.Primary) {
		b.Primary = ""
		if b.Font == "" {
			r.Brand = nil
		}
	}
	k.Materialize(r)
	return r
}

func hexColor(s string) bool {
	if len(s) != 7 || s[0] != '#' {
		return false
	}
	return strings.Trim(strings.ToLower(s[1:]), "0123456789abcdef") == ""
}

// assignSlots places every section in a slot of its page or shell template.
func assignSlots(l *core.LayoutSpec) {
	if l == nil {
		return
	}
	k := uikit.Default()
	for i := range l.Pages {
		l.Pages[i].Sections = k.AssignSlots(l.Pages[i].Pattern, l.Pages[i].Sections)
	}
	for i := range l.Shells {
		l.Shells[i].Regions = k.AssignSlots(l.Shells[i].Pattern, l.Shells[i].Regions)
	}
}

// addCollectionControls gives every table or list of a new page the
// search/filter and pagination it lacks, as entries of that section's
// content, so the rule holds when the designer left them out.
func addCollectionControls(l *core.LayoutSpec) {
	if l == nil {
		return
	}
	for i := range l.Pages {
		pg := &l.Pages[i]
		if pg.Existing {
			continue
		}
		for _, c := range core.Collections(pg) {
			j := slices.IndexFunc(pg.Sections, func(s core.Section) bool { return s.ID == c.Section })
			if j < 0 {
				continue
			}
			s := &pg.Sections[j]
			if c.Filter == "" {
				s.Content = append(s.Content, collectionFilter(pg))
			}
			if c.Pagination == "" {
				s.Content = append(s.Content, "Paginasi: 20 baris per halaman")
			}
		}
	}
}

// collectionFilter is the filter entry of a collection: a keyword search
// plus the page's status field when it shows one.
func collectionFilter(pg *core.LayoutPage) string {
	out := "Filter: cari kata kunci"
	for _, badge := range []bool{false, true} {
		for _, f := range pg.Fields {
			label := f.Label
			if label == "" {
				label = f.Field[strings.LastIndex(f.Field, ".")+1:]
			}
			if strings.Contains(strings.ToLower(label), "status") || badge && f.Presentation == "badge" {
				return out + ", " + label
			}
		}
	}
	return out
}

// addKitComponents adds the kit's rules for every kit component a section
// references that the design rules do not list yet.
func addKitComponents(d *core.UIDesign) {
	if d == nil || d.Rules == nil || d.Layout == nil {
		return
	}
	k := uikit.Default()
	have := func(name string) bool {
		return slices.ContainsFunc(d.Rules.Components, func(c core.ComponentRule) bool { return strings.EqualFold(c.Name, name) })
	}
	add := func(secs []core.Section) {
		for _, s := range secs {
			for _, name := range s.Components {
				if have(name) {
					continue
				}
				if c, ok := k.Component(name); ok {
					d.Rules.Components = append(d.Rules.Components, c)
				}
			}
		}
	}
	for _, sh := range d.Layout.Shells {
		add(sh.Regions)
	}
	for _, pg := range d.Layout.Pages {
		add(pg.Sections)
	}
}
